// Package search is the Elasticsearch client of the platform (ADR 0010: Elasticsearch serves SEARCH only,
// never a reported figure), on the config group ELASTICSEARCH_ADDRS / _USERNAME / _PASSWORD / _INDEX_PREFIX.
//
// PLAIN net/http, NOT github.com/elastic/go-elasticsearch. The service needs eight calls (create an index,
// read and write its metadata, put / delete one document, search, count, delete-by-query, bulk), each one
// JSON body in and one JSON body out. The official client would add a module tree to every image that
// imports core and to the vulnerability gate (rule 13 #6, `make vuln`) for what is a URL and a header here;
// it also hides which node a request went to. Kept small on purpose: a call it lacks is twenty lines.
//
// WHAT IT GUARANTEES:
//
//   - TLS VERIFIED against the system roots, TLS 1.2 minimum — never InsecureSkipVerify (rule 13 #1). An
//     http:// address is refused unless the caller allows it, which only dev does.
//   - Every node of ELASTICSEARCH_ADDRS is kept (rule 11 invariant 5); requests go round-robin and fail over
//     to the next node on a network error. An HTTP answer, even a 5xx, is ES's answer and is returned.
//   - Username + password travel as HTTP Basic auth in the Authorization header and nowhere else (owner
//     17/10/2026: user/pass, not an API key); secret.Secret keeps both out of logs and errors.
//   - NO DOCUMENT, NO QUERY AND NO RESPONSE BODY IS EVER LOGGED OR PUT IN AN ERROR: documents carry citizens'
//     names, and an ES error `reason` can quote a field value. An error names the HTTP status and ES's
//     error TYPE only.
package search

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vihat/vigov/core/secret"
)

const (
	// RequestTimeout bounds one call: a search behind a user request must answer or fail, never hang.
	RequestTimeout = 10 * time.Second
	// maxResponse bounds what is read back (a search page of ids is a few KB).
	maxResponse = 16 << 20
)

var (
	// ErrNotConfigured is a client built without any address.
	ErrNotConfigured = errors.New("search: ELASTICSEARCH_ADDRS is not set")
	// ErrInsecureAddress is an http:// node outside dev, or an address that is not a URL.
	ErrInsecureAddress = errors.New("search: ELASTICSEARCH_ADDRS entry must be an https URL")
	// ErrNoCredentials is a client outside dev without ELASTICSEARCH_USERNAME / ELASTICSEARCH_PASSWORD.
	ErrNoCredentials = errors.New("search: ELASTICSEARCH_USERNAME and ELASTICSEARCH_PASSWORD are not set")
	// ErrHalfCredentials is exactly one of the two set: never a silent fall back to no authentication.
	ErrHalfCredentials = errors.New("search: set both ELASTICSEARCH_USERNAME and ELASTICSEARCH_PASSWORD, or neither (dev only)")
)

// Error is a non-2xx answer: the status and ES's error type ("index_not_found_exception"), never the reason.
type Error struct {
	Status int
	Type   string
}

func (e *Error) Error() string {
	if e.Type == "" {
		return fmt.Sprintf("search: elasticsearch answered %d", e.Status)
	}
	return fmt.Sprintf("search: elasticsearch answered %d (%s)", e.Status, e.Type)
}

// IsStatus reports whether err is an *Error with that status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// Client talks to one Elasticsearch cluster. Safe for concurrent use.
type Client struct {
	addrs    []string
	username secret.Secret
	password secret.Secret
	hc       *http.Client
	next     atomic.Uint32
}

// New builds the client. allowPlainHTTP is true in dev ONLY: an http:// node sends the credentials and
// every document in the clear. Username and password are required whenever plain http is not allowed;
// exactly one of them is refused in every environment.
func New(addrs []string, username, password secret.Secret, allowPlainHTTP bool) (*Client, error) {
	if len(addrs) == 0 {
		return nil, ErrNotConfigured
	}
	clean := make([]string, 0, len(addrs))
	for i, a := range addrs {
		u, err := url.Parse(strings.TrimSpace(a))
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			// The entry is not quoted: a malformed URL is where credentials get pasted (rule 8).
			return nil, fmt.Errorf("%w (entry %d)", ErrInsecureAddress, i+1)
		}
		if u.Scheme == "http" && !allowPlainHTTP {
			return nil, fmt.Errorf("%w (entry %d is http://)", ErrInsecureAddress, i+1)
		}
		clean = append(clean, strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"))
	}
	if username.Rong() != password.Rong() {
		return nil, ErrHalfCredentials
	}
	if username.Rong() && !allowPlainHTTP {
		return nil, ErrNoCredentials
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	// Verified against the system roots — never InsecureSkipVerify (rule 13 #1).
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &Client{addrs: clean, username: username, password: password,
		hc: &http.Client{Transport: tr, Timeout: RequestTimeout}}, nil
}

// do sends one request, trying each node once from the round-robin position. ndjson selects the bulk
// content type.
func (c *Client) do(ctx context.Context, method, path string, body []byte, ndjson bool) (int, []byte, error) {
	start := int(c.next.Add(1))
	var lastErr error
	for i := range c.addrs {
		base := c.addrs[(start+i)%len(c.addrs)]
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
		if err != nil {
			return 0, nil, fmt.Errorf("search: build request: %w", err)
		}
		if body != nil {
			if ndjson {
				req.Header.Set("Content-Type", "application/x-ndjson")
			} else {
				req.Header.Set("Content-Type", "application/json")
			}
		}
		if !c.username.Rong() {
			req.SetBasicAuth(string(c.username.Lo()), string(c.password.Lo()))
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, nil, fmt.Errorf("search: %w", ctx.Err())
			}
			// A node that does not answer: the next one may. The error names the node, not the body.
			lastErr = err
			continue
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
		resp.Body.Close()
		if err != nil {
			return 0, nil, fmt.Errorf("search: read response: %w", err)
		}
		return resp.StatusCode, b, nil
	}
	return 0, nil, fmt.Errorf("search: no elasticsearch node answered: %w", lastErr)
}

// call sends a JSON body and returns the response body of a 2xx, or an *Error.
func (c *Client) call(ctx context.Context, method, path string, body any) ([]byte, error) {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("search: encode body: %w", err)
		}
	}
	status, resp, err := c.do(ctx, method, path, raw, false)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, esError(status, resp)
	}
	return resp, nil
}

// esError keeps the status and `error.type` — never `error.reason`, which may quote a value.
func esError(status int, body []byte) error {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	var typed struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(body, &e) == nil && len(e.Error) > 0 {
		_ = json.Unmarshal(e.Error, &typed)
	}
	return &Error{Status: status, Type: typed.Type}
}

func seg(s string) string { return url.PathEscape(s) }

func withRouting(path, routing string, extra ...string) string {
	q := url.Values{}
	if routing != "" {
		q.Set("routing", routing)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// EnsureIndex creates `index` with `definition` (settings + mappings) unless it exists. created reports
// whether THIS call created it. A concurrent creator is not an error.
func (c *Client) EnsureIndex(ctx context.Context, index string, definition any) (created bool, err error) {
	status, _, err := c.do(ctx, http.MethodHead, "/"+seg(index), nil, false)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return false, nil
	case http.StatusNotFound:
	default:
		return false, &Error{Status: status}
	}
	_, err = c.call(ctx, http.MethodPut, "/"+seg(index), definition)
	var e *Error
	if errors.As(err, &e) && e.Type == "resource_already_exists_exception" {
		return false, nil
	}
	return err == nil, err
}

// Meta reads the `_meta` object of the index mapping. An index without one answers an empty map.
func (c *Client) Meta(ctx context.Context, index string) (map[string]any, error) {
	b, err := c.call(ctx, http.MethodGet, "/"+seg(index)+"/_mapping", nil)
	if err != nil {
		return nil, err
	}
	var byIndex map[string]struct {
		Mappings struct {
			Meta map[string]any `json:"_meta"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(b, &byIndex); err != nil {
		return nil, errors.New("search: mapping response is not readable")
	}
	for _, v := range byIndex {
		if v.Mappings.Meta == nil {
			return map[string]any{}, nil
		}
		return v.Mappings.Meta, nil
	}
	return map[string]any{}, nil
}

// PutMeta replaces the `_meta` object of the index mapping.
func (c *Client) PutMeta(ctx context.Context, index string, meta map[string]any) error {
	_, err := c.call(ctx, http.MethodPut, "/"+seg(index)+"/_mapping", map[string]any{"_meta": meta})
	return err
}

// PutDoc writes one document under `id`, routed by `routing` (a commune keeps its documents on one shard).
func (c *Client) PutDoc(ctx context.Context, index, id, routing string, doc any) error {
	_, err := c.call(ctx, http.MethodPut, withRouting("/"+seg(index)+"/_doc/"+seg(id), routing), doc)
	return err
}

// DeleteDoc removes one document. Absent already is success: a delete is idempotent.
func (c *Client) DeleteDoc(ctx context.Context, index, id, routing string) error {
	_, err := c.call(ctx, http.MethodDelete, withRouting("/"+seg(index)+"/_doc/"+seg(id), routing), nil)
	if IsStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

// Hit is one search result: the document id and its sort values (for search_after).
type Hit struct {
	ID   string            `json:"_id"`
	Sort []json.RawMessage `json:"sort"`
}

// SearchResult is the ids of one page and, when tracked, the total.
type SearchResult struct {
	Total int64
	Hits  []Hit
}

// Search runs one query body (the caller builds it, `_source: false`).
func (c *Client) Search(ctx context.Context, index, routing string, body any) (SearchResult, error) {
	b, err := c.call(ctx, http.MethodPost, withRouting("/"+seg(index)+"/_search", routing), body)
	if err != nil {
		return SearchResult{}, err
	}
	var r struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []Hit `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return SearchResult{}, errors.New("search: search response is not readable")
	}
	return SearchResult{Total: r.Hits.Total.Value, Hits: r.Hits.Hits}, nil
}

// Count counts the documents a query matches.
func (c *Client) Count(ctx context.Context, index, routing string, query any) (int64, error) {
	b, err := c.call(ctx, http.MethodPost, withRouting("/"+seg(index)+"/_count", routing), map[string]any{"query": query})
	if err != nil {
		return 0, err
	}
	var r struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return 0, errors.New("search: count response is not readable")
	}
	return r.Count, nil
}

// DeleteByQuery removes every document a query matches, and waits for it (refresh=true).
func (c *Client) DeleteByQuery(ctx context.Context, index, routing string, query any) error {
	_, err := c.call(ctx, http.MethodPost,
		withRouting("/"+seg(index)+"/_delete_by_query", routing, "conflicts", "proceed", "refresh", "true"),
		map[string]any{"query": query})
	return err
}

// BulkIndex is one document of a bulk write.
type BulkIndex struct {
	ID      string
	Routing string
	Doc     any
}

// Bulk writes documents in one request. A partial failure is an error naming how many failed and the
// first error TYPE — never which document or what it held.
func (c *Client) Bulk(ctx context.Context, index string, items []BulkIndex) error {
	if len(items) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, it := range items {
		action := map[string]any{"index": map[string]any{"_index": index, "_id": it.ID, "routing": it.Routing}}
		if err := enc.Encode(action); err != nil {
			return fmt.Errorf("search: encode bulk action: %w", err)
		}
		if err := enc.Encode(it.Doc); err != nil {
			return fmt.Errorf("search: encode bulk document: %w", err)
		}
	}
	status, resp, err := c.do(ctx, http.MethodPost, "/_bulk", buf.Bytes(), true)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return esError(status, resp)
	}
	var r struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  struct {
				Type string `json:"type"`
			} `json:"error"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return errors.New("search: bulk response is not readable")
	}
	if !r.Errors {
		return nil
	}
	failed, firstType := 0, ""
	for _, it := range r.Items {
		for _, v := range it {
			if v.Status > 299 {
				failed++
				if firstType == "" {
					firstType = v.Error.Type
				}
			}
		}
	}
	return fmt.Errorf("search: bulk: %d of %d documents refused (%s)", failed, len(items), firstType)
}
