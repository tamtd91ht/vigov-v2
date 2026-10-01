package portal

// THE CLIENT — two documented endpoints and the image fetch (ADR 0067 §2 §Phạm vi), and what the
// prototype (`../vigov-require/apps/api/app/integrations/cttdt/client.py`) measured about them against
// the real `thangbinh.danang.gov.vn`, which this file takes as the API's behaviour:
//
//	GET {base}/chuyenmuc                                         the category list
//	GET {base}/tintheochuyenmuc?lstChuyenMuc=<id>&secret_code=<key>
//	                                                             ONE category's articles — no paging, no
//	                                                             date filter; an EMPTY id returns the whole
//	                                                             portal (7 001 rows), so it is refused here
//
// `NoiDung` is HTML ENCODED TWICE (`&lt;p&gt;`): it is unescaped ONCE here and sanitised by the caller
// (internal/richtext). Image references come as https, http, relative or empty — ResolveImage decides.
// Dates carry no zone and are the portal's local time (+07:00).
//
// BOUNDS, every one a vendor guard against a portal (or something pretending to be one) holding this
// process hostage: a connect / TLS / header / whole-call timeout, a byte cap per response read through
// a LimitReader (gzip is decompressed by the transport BEFORE the cap, so the cap is on decoded bytes),
// and a streamed JSON decode that keeps only the articles inside the window — a category answering
// thousands of rows is read once and mostly dropped.
//
// The Go field names below are English; the JSON tags are the portal's wire names, which this file
// does not choose.

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	xhtml "golang.org/x/net/html"

	"github.com/vihat/vigov/core/secret"
)

// MaxRedirects — the owner's answer of 01/10/2026 (ADR 0067 Còn mở #5): at most three redirects, and
// every target must pass CheckURL like the api_url itself; otherwise the call fails.
//
// NARROWED 02/10/2026 (owner, D4): every target must also stay on the SAME HOST as the api_url — for
// the API calls exactly as for the images. "Another .gov.vn host" is no longer enough: the key travels
// in the query string, and a redirect is the portal choosing where the next request goes. A hop to any
// other host refuses the call; the run records it per category.
const MaxRedirects = 3

// MaxKeepPerCategory bounds Articles' keep — the run's MaxItemsPerRun ceiling (domain, owner 02/10).
const MaxKeepPerCategory = 100

// The bounds. Vendor numbers, chosen and stated, not customer figures.
const (
	connectTimeout        = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 30 * time.Second
	// categoriesTimeout / articlesTimeout / imageTimeout bound one whole call. The prototype measured
	// the two largest Thăng Bình categories at "nearly half a minute" between them.
	categoriesTimeout = 30 * time.Second
	articlesTimeout   = 90 * time.Second
	imageTimeout      = 20 * time.Second

	// maxCategoriesBytes / maxArticlesBytes cap one decoded response. The prototype measured the whole
	// portal at ~64 MB and one large category at >10 MB; 32 MiB fits any one category it saw.
	maxCategoriesBytes = 4 << 20
	maxArticlesBytes   = 32 << 20
)

// portalLocation is the zone of the portal's zone-less timestamps. A FIXED offset, not a tz database
// lookup: Việt Nam has no daylight saving, and the image this runs in may carry no zoneinfo.
var portalLocation = time.FixedZone("ICT", 7*60*60)

// PortalLocation exposes the zone for callers that turn a portal instant into the portal's own date.
func PortalLocation() *time.Location { return portalLocation }

// Options configures a Client. Every field has a production default; the injectable ones exist for
// tests, which can NOT weaken the checks this way (they replace the resolver and the socket, never
// CheckURL or AddrAllowed).
type Options struct {
	Resolver Resolver       // default net.DefaultResolver
	Dial     DialFunc       // default a net.Dialer with connectTimeout; receives a CHECKED ip:443
	RootCAs  *x509.CertPool // default the system pool
}

// Client calls a portal. Safe for concurrent use; one per process.
type Client struct {
	transport http.RoundTripper
}

// New builds the client: a transport whose ONLY way to a socket is the guarded dialer, no proxy (an
// HTTPS_PROXY in the environment would move the connect check onto the proxy's address), TLS 1.2+
// with the certificate verified against the host name (rule 13, invariant 1).
func New(o Options) *Client {
	if o.Resolver == nil {
		o.Resolver = net.DefaultResolver
	}
	if o.Dial == nil {
		d := &net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
		o.Dial = d.DialContext
	}
	gd := &guardedDialer{resolver: o.Resolver, dial: o.Dial}
	return &Client{transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           gd.DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: o.RootCAs},
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
		ForceAttemptHTTP2:     true,
	}}
}

// Endpoint is one commune's portal API: the saved api_url and the opened key. Key is a secret.Secret
// so formatting an Endpoint cannot print it.
type Endpoint struct {
	Base *url.URL
	Key  secret.Secret
}

// ParseBase parses and checks a saved api_url. A query or fragment is refused: the key is the only
// query parameter this client adds, and a base carrying its own would be a second, unchecked place a
// value travels.
func ParseBase(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if strings.ContainsAny(raw, "?#") {
		return nil, ErrURLRefused
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, ErrURLRefused
	}
	if err := CheckURL(u); err != nil {
		return nil, err
	}
	return u, nil
}

// endpoint joins base and a path element and adds the query.
func (e Endpoint) endpoint(elem string, q url.Values) *url.URL {
	u := *e.Base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + elem
	u.RawPath = ""
	u.RawQuery = q.Encode()
	return &u
}

// redirectPolicy returns the CheckRedirect for one call: at most MaxRedirects hops, each passing
// CheckURL AND staying on sameHost — the api_url's host, for every call (D4, 02/10/2026; images since
// ADR 0067 §2 decision 2). sameHost is required: an empty one refuses every hop rather than meaning
// "any host", so a caller that forgets it fails closed.
func redirectPolicy(sameHost string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > MaxRedirects {
			return ErrTooManyRedirects
		}
		if CheckURL(req.URL) != nil {
			return ErrRedirectRefused
		}
		if sameHost == "" || !strings.EqualFold(req.URL.Hostname(), sameHost) {
			return ErrRedirectRefused
		}
		return nil
	}
}

// get performs one GET and returns the body bounded to max bytes. The URL is checked FIRST; no error
// returned quotes it.
func (c *Client) get(ctx context.Context, u *url.URL, max int64, sameHost string) ([]byte, error) {
	if err := CheckURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrURLRefused
	}
	req.Header.Set("Accept", "application/json, image/*;q=0.9")
	req.Header.Set("User-Agent", "ViGov-comms/1")

	// A client per call shares the transport (and so the connection pool) and carries this call's
	// redirect rule.
	hc := &http.Client{Transport: c.transport, CheckRedirect: redirectPolicy(sameHost)}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, classify(err) // the *url.Error — which quotes the URL, key included — ends here
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// The body is never read into anything: a portal error page can quote the request.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil, httpStatusError(resp.StatusCode)
	}
	if resp.ContentLength > max {
		return nil, ErrTooLarge
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, classify(err)
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

// --- categories ---------------------------------------------------------------------------------------

// Category is one portal category. The portal's child level (`ChuyenMuc`) is what the article endpoint
// takes; ParentID/ParentName are its parent (`DanhMuc`). Names are NOT unique (Thăng Bình has two
// "Chuyển đổi số" under different parents), so the parent travels with it — the screen cannot tell
// the two apart otherwise.
type Category struct {
	ExternalID string
	Name       string
	ParentID   string
	ParentName string
}

type wireCategory struct {
	ID         json.RawMessage `json:"ChuyenMucID"`
	Name       string          `json:"TenChuyenMuc"`
	ParentID   json.RawMessage `json:"DanhMucID"`
	ParentName string          `json:"TenDanhMuc"`
}

// Categories reads the portal's category list, deduplicated by id, ordered by parent then name.
func (c *Client) Categories(ctx context.Context, e Endpoint) ([]Category, error) {
	if len(e.Key) == 0 || e.Base == nil {
		return nil, ErrMissingCredential
	}
	ctx, cancel := context.WithTimeout(ctx, categoriesTimeout)
	defer cancel()
	body, err := c.get(ctx, e.endpoint("chuyenmuc", url.Values{"secret_code": {string(e.Key.Lo())}}),
		maxCategoriesBytes, e.Base.Hostname())
	if err != nil {
		return nil, err
	}
	return parseCategories(body)
}

func parseCategories(body []byte) ([]Category, error) {
	var rows []wireCategory
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, ErrParse
	}
	seen := map[string]bool{}
	out := make([]Category, 0, len(rows))
	for _, r := range rows {
		id := flexString(r.ID)
		name := PlainText(r.Name)
		if name == "" {
			name = PlainText(r.ParentName)
		}
		if id == "" || id == "0" || name == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Category{ExternalID: id, Name: name, ParentID: flexString(r.ParentID),
			ParentName: PlainText(r.ParentName)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ParentName != b.ParentName {
			return a.ParentName < b.ParentName
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ExternalID < b.ExternalID
	})
	return out, nil
}

// --- articles -----------------------------------------------------------------------------------------

// Article is one portal article, decoded and trimmed — NOT yet sanitised (BodyHTML) and NOT yet
// resolved (ImageRef). Title, Summary and SourceLabel are plain text.
//
// ⚠ ARTICLE TEXT NAMES PEOPLE (residents in the news, officials). Nothing here is ever logged; callers
// log the category id and counts only (rule 3).
type Article struct {
	ExternalID  string
	Title       string
	Summary     string
	BodyHTML    string
	ImageRef    string
	PublishedAt time.Time // never zero in a returned batch: undated rows are dropped
	// SourceLabel is the portal's `NguonTin` — the outlet it re-published from. The author field
	// (`TacGia`) is deliberately not read: it names a person.
	SourceLabel string
}

type wireArticle struct {
	ID         json.RawMessage `json:"TinTucID"`
	Title      string          `json:"TieuDe"`
	Summary    string          `json:"GioiThieu"`
	Body       string          `json:"NoiDung"`
	LargeImage string          `json:"AnhLonUrl"`
	SmallImage string          `json:"AnhNhoUrl"`
	Published  string          `json:"NgayDang"`
	Source     string          `json:"NguonTin"`
	Deleted1   json.RawMessage `json:"isDelete"`
	Deleted2   json.RawMessage `json:"isDeleted"`
}

// ArticleBatch is one category's answer: the articles kept (dated, published at or after `since`, NOT
// already held by the commune, the newest `keep` of those, newest first), how many usable rows the
// portal returned in total — this category's share of `fetched_count` — and how many in-window rows
// were already held, live or soft-deleted.
type ArticleBatch struct {
	Articles []Article
	Read     int
	// HeldLive / HeldDeleted count the in-window rows HeldFunc reported, per category (an article filed
	// under two categories is counted in both; the caller deduplicates if it needs to).
	HeldLive    int
	HeldDeleted int
}

// HeldFunc reports which of articleIDs — the PORTAL'S OWN ids, one chunk of one category — the commune
// already holds: id → true when the holder is soft-deleted, false when live, absent when not held. The
// caller owns the namespacing and the store (this package knows no SQL); the commune is in ctx.
type HeldFunc func(ctx context.Context, articleIDs []string) (map[string]bool, error)

// heldChunk is how many in-window candidates are buffered before one HeldFunc call: one query per 200
// rows, and the most decoded articles held at once besides the heap.
const heldChunk = 200

// Articles reads ONE category and keeps the NEWEST keep articles published at or after since THAT THE
// COMMUNE DOES NOT ALREADY HOLD (held, soft-deleted included, is asked BEFORE an article may take a heap
// slot — follow-up of 02/10/2026). keep is 1..MaxKeepPerCategory; the run passes its MaxItemsPerRun.
//
// WHY THE LOOKUP SITS BEFORE THE HEAP: kept after it, the newest keep rows of a category would be the
// same already-imported rows run after run, and an older in-window article would never be reached — a
// backlog larger than one run's ceiling would never drain. Asked first, already-held rows never occupy
// a slot, so each run takes the newest keep NOT YET imported and the next run continues below them.
//
// Memory stays bounded: the heap (keep) plus one chunk (heldChunk) of decoded articles. Undated rows are
// dropped with the old ones: a row the window cannot place is not imported under today's date.
func (c *Client) Articles(ctx context.Context, e Endpoint, categoryID string, since time.Time, keep int,
	held HeldFunc) (ArticleBatch, error) {

	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" || categoryID == "0" {
		return ArticleBatch{}, ErrCategoryRequired
	}
	if keep < 1 || keep > MaxKeepPerCategory {
		return ArticleBatch{}, ErrKeepInvalid
	}
	if held == nil {
		// Without it every held article would compete for a slot — the drain this exists for is lost,
		// silently. Refused, never defaulted to "nothing is held".
		return ArticleBatch{}, ErrHeldLookup
	}
	if len(e.Key) == 0 || e.Base == nil {
		return ArticleBatch{}, ErrMissingCredential
	}
	ctx, cancel := context.WithTimeout(ctx, articlesTimeout)
	defer cancel()
	q := url.Values{"lstChuyenMuc": {categoryID}, "secret_code": {string(e.Key.Lo())}}
	body, err := c.get(ctx, e.endpoint("tintheochuyenmuc", q), maxArticlesBytes, e.Base.Hostname())
	if err != nil {
		return ArticleBatch{}, err
	}
	return parseArticles(ctx, body, since, keep, held)
}

// newestArticles is a bounded min-heap of the newest articles seen. Its root is the WORST kept one (the
// oldest; on a tie, the later in the portal's answer) and is evicted when a better one arrives, so a
// category answering thousands of in-window rows holds keep decoded articles, never all of them.
type newestArticles []rankedArticle

type rankedArticle struct {
	a   Article
	pos int // position in the portal's answer: ties keep the portal's order
}

// worse reports whether x ranks below y: older, or as old and later in the answer.
func worse(x, y rankedArticle) bool {
	if !x.a.PublishedAt.Equal(y.a.PublishedAt) {
		return x.a.PublishedAt.Before(y.a.PublishedAt)
	}
	return x.pos > y.pos
}

func (h newestArticles) Len() int           { return len(h) }
func (h newestArticles) Less(i, j int) bool { return worse(h[i], h[j]) }
func (h newestArticles) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *newestArticles) Push(x any)        { *h = append(*h, x.(rankedArticle)) }
func (h *newestArticles) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// parseArticles streams the array: one element decoded at a time, the ones outside the window dropped at
// once, the rest buffered heldChunk at a time, the commune's holdings asked once per chunk, and only the
// newest keep NOT-held ones kept (newestArticles). The result is newest first, ties in the portal's order.
func parseArticles(ctx context.Context, body []byte, since time.Time, keep int, held HeldFunc) (ArticleBatch, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return ArticleBatch{}, ErrParse
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return ArticleBatch{}, ErrParse
	}
	var out ArticleBatch
	kept := make(newestArticles, 0, keep)
	chunk := make([]rankedArticle, 0, heldChunk)
	offer := func(cand rankedArticle) {
		switch {
		case len(kept) < keep:
			heap.Push(&kept, cand)
		case worse(kept[0], cand):
			kept[0] = cand
			heap.Fix(&kept, 0)
		}
	}
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		ids := make([]string, len(chunk))
		for i, c := range chunk {
			ids[i] = c.a.ExternalID
		}
		// The caller's error is DROPPED, like every error here: this package returns classes only.
		have, err := held(ctx, ids)
		if err != nil {
			return ErrHeldLookup
		}
		for _, c := range chunk {
			if deleted, ok := have[c.a.ExternalID]; ok {
				if deleted {
					out.HeldDeleted++
				} else {
					out.HeldLive++
				}
				continue // never occupies a slot
			}
			offer(c)
		}
		chunk = chunk[:0]
		return nil
	}
	pos := 0
	for dec.More() {
		var r wireArticle
		if err := dec.Decode(&r); err != nil {
			return ArticleBatch{}, ErrParse
		}
		if flexBool(r.Deleted1) || flexBool(r.Deleted2) {
			continue
		}
		id := flexString(r.ID)
		title := PlainText(r.Title)
		if id == "" || id == "0" || title == "" {
			continue
		}
		out.Read++
		at := ParseMoment(r.Published)
		if at.IsZero() || at.Before(since) {
			continue
		}
		cand := rankedArticle{pos: pos, a: Article{ExternalID: id, PublishedAt: at}}
		pos++
		// The heap's worst only ever improves, so a row no better than it now can never enter: it is
		// not buffered, not looked up and its body never unescaped. (It is not counted as held either —
		// the held counts cover the rows that were asked about.)
		if len(kept) == keep && !worse(kept[0], cand) {
			continue
		}
		img := strings.TrimSpace(r.LargeImage)
		if img == "" {
			img = strings.TrimSpace(r.SmallImage)
		}
		cand.a.Title = title
		cand.a.Summary = PlainText(r.Summary)
		cand.a.BodyHTML = strings.TrimSpace(html.UnescapeString(r.Body)) // ONCE: the portal encodes twice
		cand.a.ImageRef = img
		cand.a.SourceLabel = PlainText(r.Source)
		chunk = append(chunk, cand)
		if len(chunk) == heldChunk {
			if err := flush(); err != nil {
				return ArticleBatch{}, err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // the closing ']'
		return ArticleBatch{}, ErrParse
	}
	if err := flush(); err != nil {
		return ArticleBatch{}, err
	}
	sort.Slice(kept, func(i, j int) bool { return worse(kept[j], kept[i]) })
	out.Articles = make([]Article, len(kept))
	for i, k := range kept {
		out.Articles[i] = k.a
	}
	return out, nil
}

// ParseMoment reads `2026-08-29T21:27:27.367` (no zone → +07:00) or an RFC 3339 instant. The portal's
// "no date" is year 0001; anything before 1900 is no date.
func ParseMoment(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return validMoment(t)
	}
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, portalLocation); err == nil {
			return validMoment(t)
		}
	}
	return time.Time{}
}

func validMoment(t time.Time) time.Time {
	if t.Year() < 1900 {
		return time.Time{}
	}
	return t
}

// --- images -------------------------------------------------------------------------------------------

// ResolveImage turns an article's image reference into the URL to fetch, under ADR 0067 §2 decision 2:
// the image must be on the SAME host as the api_url, over https.
//
//	""                         → nil, nil (no image; not a failure)
//	"/Portals/0/x.jpg"         → https://<api host>/Portals/0/x.jpg
//	"//<api host>/x.jpg"       → https://<api host>/x.jpg
//	"http://<api host>/x.jpg"  → upgraded to https (the API itself answers https on that host)
//	another host, any scheme   → ErrImageForeignHost (dropped, counted, never fetched)
//
// A relative reference without a leading slash is refused: the portal's are root-relative, and a path
// relative to the API directory would be a guess.
func ResolveImage(base *url.URL, ref string) (*url.URL, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	if base == nil || strings.ContainsAny(ref, " \t\r\n\\") {
		return nil, ErrImageInvalidURL
	}
	switch {
	case strings.HasPrefix(ref, "//"):
		ref = "https:" + ref
	case strings.HasPrefix(ref, "/"):
		ref = "https://" + base.Host + ref
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" || u.Opaque != "" {
		return nil, ErrImageInvalidURL
	}
	if !strings.EqualFold(u.Hostname(), base.Hostname()) {
		return nil, ErrImageForeignHost
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		u.Scheme = "https"
		if u.Port() == "80" {
			u.Host = u.Hostname()
		}
	default:
		return nil, ErrImageInvalidURL
	}
	u.Fragment = ""
	if CheckURL(u) != nil {
		return nil, ErrImageInvalidURL
	}
	return u, nil
}

// Image fetches one image already passed through ResolveImage, at most max bytes, redirects confined to
// the same host. The bytes are NOT trusted: the caller sniffs, scans and re-encodes them.
func (c *Client) Image(ctx context.Context, e Endpoint, u *url.URL, max int64) ([]byte, error) {
	if u == nil || e.Base == nil || !strings.EqualFold(u.Hostname(), e.Base.Hostname()) {
		return nil, ErrImageForeignHost
	}
	ctx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()
	return c.get(ctx, u, max, e.Base.Hostname())
}

// --- text helpers -------------------------------------------------------------------------------------

// PlainText turns a portal text field into one line of plain text: entities decoded, any markup dropped
// (its text kept), control characters removed, whitespace collapsed. For titles, summaries and labels
// — never for the body, which keeps its allowed markup through richtext.Sanitize.
func PlainText(s string) string {
	if s == "" {
		return ""
	}
	s = html.UnescapeString(s)
	if strings.ContainsAny(s, "<>") {
		var b strings.Builder
		z := xhtml.NewTokenizer(strings.NewReader(s))
		for {
			tt := z.Next()
			if tt == xhtml.ErrorToken {
				break
			}
			if tt == xhtml.TextToken {
				b.Write(z.Text())
				b.WriteByte(' ')
			}
		}
		s = b.String()
	}
	var b strings.Builder
	pending := false
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			pending = b.Len() > 0
			continue
		}
		if pending {
			b.WriteByte(' ')
			pending = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// flexString reads an id the portal sends as a number or a string.
func flexString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if _, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
			return n.String()
		}
	}
	return ""
}

// flexBool reads a flag sent as true/false, 0/1 or "true"/"1".
func flexBool(raw json.RawMessage) bool {
	switch strings.Trim(strings.ToLower(string(raw)), `"`) {
	case "true", "1":
		return true
	}
	return false
}
