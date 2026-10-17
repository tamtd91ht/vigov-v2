// Package search is identity's Elasticsearch index of the citizen directory (ADR 0101 #7–#11): the
// mapping, the document built from a PostgreSQL row, and the queries. PostgreSQL stays the source of truth;
// this index answers "which ids, in which order, how many" and nothing else — the use case hydrates the
// page from the scoped store, masked.
//
// RULE 1 IS ENFORCED HERE, ON EVERY CALL: the commune comes from the context (tenant.MustFrom) and is a
// `term` filter of every query, the routing of every request, and the prefix of every document id
// (`t:<tenant_id>:<contact_id>`, rule 1 invariant 7). No method takes a commune. With ONE system-wide hash
// key (owner, 17/10/2026) the same phone in two communes has identical hashes, so this filter is the ONLY
// thing separating them — which is why no query can be built without it.
//
// WHAT THE INDEX HOLDS (ADR 0101 #8, #9, #10): the full name verbatim (accepted by the owner, for
// accent-insensitive full-text search), keyed HMAC grams of the phone and of the e-mail's local part, the
// HMAC of the whole e-mail, active flag, group and residential-unit ids, tags, created_at in epoch millis,
// and the attribute values — option fields as "code=value" keywords, number and dateTime as nested
// {code, value}. NEVER the phone number, the e-mail, the address or the note.
//
// THE KEY GUARD (owner, 17/10/2026): the index records the fingerprint of SEARCH_HASH_KEY in its mapping
// `_meta` when it is created. Open compares it with the configured key; on a mismatch the index is opened
// DISABLED — every call answers ErrUnavailable — so hashes made with two different keys are never mixed and
// a search never silently misses everyone. AdoptKey (the reindex path) is the only way to accept a new key.
//
// NOTHING HERE LOGS a document or a query: both carry citizens' names.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	coresearch "github.com/vihat/vigov/core/search"
	"github.com/vihat/vigov/core/searchhash"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// IndexSuffix is the index name after ELASTICSEARCH_INDEX_PREFIX (task card E-1).
const IndexSuffix = "citizen-contacts"

// schemaVersion is bumped with a mapping change that needs a new index; recorded in `_meta`.
const schemaVersion = 1

var (
	// ErrUnavailable is a search or an indexing call while the index is off: not configured (dev), or the
	// configured SEARCH_HASH_KEY does not match the index. The route answers 503 with one sentence.
	ErrUnavailable = errors.New("tìm kiếm danh bạ: chỉ mục Elasticsearch đang tắt")
	// ErrKeyMismatch is Open finding an index built with another SEARCH_HASH_KEY.
	ErrKeyMismatch = errors.New("tìm kiếm danh bạ: SEARCH_HASH_KEY khác khoá đã dựng chỉ mục — tìm kiếm và ghi chỉ mục bị tắt; đổi khoá phải dựng lại chỉ mục mọi xã")
)

// es is the part of *coresearch.Client the index uses — an interface for tests.
type es interface {
	EnsureIndex(ctx context.Context, index string, definition any) (bool, error)
	Meta(ctx context.Context, index string) (map[string]any, error)
	PutMeta(ctx context.Context, index string, meta map[string]any) error
	PutDoc(ctx context.Context, index, id, routing string, doc any) error
	DeleteDoc(ctx context.Context, index, id, routing string) error
	Search(ctx context.Context, index, routing string, body any) (coresearch.SearchResult, error)
	Count(ctx context.Context, index, routing string, query any) (int64, error)
	DeleteByQuery(ctx context.Context, index, routing string, query any) error
	Bulk(ctx context.Context, index string, items []coresearch.BulkIndex) error
}

// CitizenIndex is the directory's index. A nil pointer is OFF (every call ErrUnavailable), which is what
// dev without Elasticsearch gets.
type CitizenIndex struct {
	es   es
	key  searchhash.Key
	name string

	mu    sync.RWMutex
	ready bool
}

// IndexName is the full index name for a prefix: "<prefix>-citizen-contacts", or the suffix alone.
func IndexName(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		return IndexSuffix
	}
	if !strings.HasSuffix(p, "-") && !strings.HasSuffix(p, "_") {
		p += "-"
	}
	return p + IndexSuffix
}

// New builds the index handle without touching the cluster. Open must be called before use.
func New(client es, key searchhash.Key, prefix string) *CitizenIndex {
	return &CitizenIndex{es: client, key: key, name: IndexName(prefix)}
}

// Name is the index name (for the startup log line).
func (x *CitizenIndex) Name() string { return x.name }

// Open creates the index with its strict mapping when missing (recording the key fingerprint), or checks
// the fingerprint of an existing one. ErrKeyMismatch leaves the index OFF; any other error too.
func (x *CitizenIndex) Open(ctx context.Context) error {
	if x == nil || x.es == nil || !x.key.Valid() {
		return ErrUnavailable
	}
	created, err := x.es.EnsureIndex(ctx, x.name, x.definition())
	if err != nil {
		return fmt.Errorf("tìm kiếm danh bạ: tạo chỉ mục: %w", err)
	}
	if !created {
		meta, err := x.es.Meta(ctx, x.name)
		if err != nil {
			return fmt.Errorf("tìm kiếm danh bạ: đọc _meta chỉ mục: %w", err)
		}
		if fp, _ := meta["key_fingerprint"].(string); fp != x.key.Fingerprint() {
			return ErrKeyMismatch
		}
	}
	x.setReady(true)
	return nil
}

// AdoptKey writes the configured key's fingerprint into the index `_meta` and switches the index on. ONLY
// the reindex-every-commune path calls it, after deleting every document hashed with the old key.
func (x *CitizenIndex) AdoptKey(ctx context.Context) error {
	if x == nil || x.es == nil || !x.key.Valid() {
		return ErrUnavailable
	}
	if _, err := x.es.EnsureIndex(ctx, x.name, x.definition()); err != nil {
		return fmt.Errorf("tìm kiếm danh bạ: tạo chỉ mục: %w", err)
	}
	if err := x.es.PutMeta(ctx, x.name, x.meta()); err != nil {
		return fmt.Errorf("tìm kiếm danh bạ: ghi _meta chỉ mục: %w", err)
	}
	x.setReady(true)
	return nil
}

// DeleteEverything removes every document of every commune — the first step of adopting a new key, so no
// hash made with the old key survives. Called only by the reindex-every-commune path, which then rebuilds
// each commune from its own rows.
func (x *CitizenIndex) DeleteEverything(ctx context.Context) error {
	if x == nil || x.es == nil || !x.key.Valid() {
		return ErrUnavailable
	}
	// @cross-tenant: adopting a new SEARCH_HASH_KEY (owner 17/10/2026) — every document of every commune
	// was hashed with the old key and must go; each commune is rebuilt from its own PostgreSQL rows next.
	return x.es.DeleteByQuery(ctx, x.name, "", map[string]any{"match_all": map[string]any{}})
}

func (x *CitizenIndex) setReady(v bool) {
	x.mu.Lock()
	x.ready = v
	x.mu.Unlock()
}

// Ready reports whether searches and writes are served. nil = on.
func (x *CitizenIndex) Ready() error {
	if x == nil {
		return ErrUnavailable
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	if !x.ready {
		return ErrUnavailable
	}
	return nil
}

func (x *CitizenIndex) meta() map[string]any {
	return map[string]any{"key_fingerprint": x.key.Fingerprint(), "schema": schemaVersion}
}

// definition is the index settings and the STRICT mapping: a document with a field not declared here is
// refused, so nothing can reach the index by accident (a typo, a future column).
func (x *CitizenIndex) definition() map[string]any {
	kw := map[string]any{"type": "keyword"}
	return map[string]any{
		"settings": map[string]any{
			"analysis": map[string]any{
				"filter": map[string]any{
					"name_prefix": map[string]any{"type": "edge_ngram", "min_gram": 1, "max_gram": 20},
				},
				"analyzer": map[string]any{
					// asciifolding maps Vietnamese diacritics and đ to ASCII (ADR 0101 #9: "có bỏ dấu").
					"folded":        map[string]any{"type": "custom", "tokenizer": "standard", "filter": []string{"lowercase", "asciifolding"}},
					"folded_prefix": map[string]any{"type": "custom", "tokenizer": "standard", "filter": []string{"lowercase", "asciifolding", "name_prefix"}},
				},
			},
		},
		"mappings": map[string]any{
			"dynamic": "strict",
			"_meta":   x.meta(),
			"properties": map[string]any{
				fieldTenant:  kw,
				"contact_id": kw,
				"full_name": map[string]any{"type": "text", "analyzer": "folded", "fields": map[string]any{
					"prefix": map[string]any{"type": "text", "analyzer": "folded_prefix", "search_analyzer": "folded"},
				}},
				"phone_grams":         kw,
				"email_grams":         kw,
				"email_exact":         kw,
				"is_active":           map[string]any{"type": "boolean"},
				"group_ids":           kw,
				"residential_unit_id": kw,
				"tags":                kw,
				"created_at":          map[string]any{"type": "long"},
				"attr_kw":             kw,
				"attr_num": map[string]any{"type": "nested", "properties": map[string]any{
					"code":  kw,
					"value": map[string]any{"type": "double"},
				}},
			},
		},
	}
}

// fieldTenant is the document field holding the commune — set from the context, filtered on every query.
const fieldTenant = "tenant_id"

// docID is the document id of one contact: the commune prefix makes the id meaningless in any other
// commune (rule 1 invariant 7).
func docID(t tenant.ID, contactID string) string { return "t:" + string(t) + ":" + contactID }

// Doc is one document. Its JSON is built by body(), field by field, so the strict mapping and the
// document cannot drift apart silently.
type Doc struct {
	Commune           tenant.ID
	ContactID         string
	FullName          string
	PhoneGrams        []string
	EmailGrams        []string
	EmailExact        string
	IsActive          bool
	GroupIDs          []string
	ResidentialUnitID string
	Tags              []string
	CreatedAt         int64
	AttrKW            []string
	AttrNum           []AttrNum
}

// AttrNum is one numeric or date attribute value.
type AttrNum struct {
	Code  string  `json:"code"`
	Value float64 `json:"value"`
}

func (d Doc) body() map[string]any {
	b := map[string]any{
		fieldTenant: string(d.Commune), "contact_id": d.ContactID, "full_name": d.FullName,
		"phone_grams": nonNil(d.PhoneGrams), "email_grams": nonNil(d.EmailGrams), "is_active": d.IsActive,
		"group_ids": nonNil(d.GroupIDs), "tags": nonNil(d.Tags), "created_at": d.CreatedAt,
		"attr_kw": nonNil(d.AttrKW), "attr_num": d.AttrNum,
	}
	if d.AttrNum == nil {
		b["attr_num"] = []AttrNum{}
	}
	if d.EmailExact != "" {
		b["email_exact"] = d.EmailExact
	}
	if d.ResidentialUnitID != "" {
		b["residential_unit_id"] = d.ResidentialUnitID
	}
	return b
}

// BuildDoc turns one live PostgreSQL row of the commune t into its document. Attribute values are typed by
// the commune's definitions (retired ones included: their values stay consistent, and the filters only
// offer live fields).
func (x *CitizenIndex) BuildDoc(t tenant.ID, c domain.CitizenContact, groupIDs []string, defs []domain.CitizenField) Doc {
	d := Doc{
		Commune: t, ContactID: c.ID, FullName: c.FullName,
		PhoneGrams: x.key.PhoneGrams(c.Phone), IsActive: c.IsActive,
		GroupIDs: groupIDs, Tags: c.Tags, CreatedAt: c.CreatedAt.UnixMilli(),
	}
	if c.ResidentialUnitID != nil {
		d.ResidentialUnitID = *c.ResidentialUnitID
	}
	if c.Email != "" {
		d.EmailGrams, d.EmailExact = x.key.EmailGrams(c.Email)
	}
	for _, f := range defs {
		raw, ok := c.Attributes[f.Code]
		if !ok {
			continue
		}
		switch f.Type {
		case domain.FieldSingleBox:
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				d.AttrKW = append(d.AttrKW, f.Code+"="+s)
			}
		case domain.FieldCombobox:
			var list []string
			if json.Unmarshal(raw, &list) == nil {
				for _, s := range list {
					d.AttrKW = append(d.AttrKW, f.Code+"="+s)
				}
			}
		case domain.FieldNumber, domain.FieldDateTime:
			if v, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64); err == nil {
				d.AttrNum = append(d.AttrNum, AttrNum{Code: f.Code, Value: v})
			}
		}
	}
	return d
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// errOtherCommune is a document built for another commune than the context's. Never sent (rule 1).
var errOtherCommune = errors.New("tìm kiếm danh bạ: tài liệu không thuộc xã đang xử lý")

// Upsert writes one contact's document in the commune of ctx.
func (x *CitizenIndex) Upsert(ctx context.Context, d Doc) error {
	if err := x.Ready(); err != nil {
		return err
	}
	t := tenant.MustFrom(ctx)
	if d.Commune != t {
		return errOtherCommune
	}
	return x.es.PutDoc(ctx, x.name, docID(t, d.ContactID), string(t), d.body())
}

// BulkUpsert writes many documents of the commune of ctx in one request (the reindex).
func (x *CitizenIndex) BulkUpsert(ctx context.Context, docs []Doc) error {
	if err := x.Ready(); err != nil {
		return err
	}
	t := tenant.MustFrom(ctx)
	items := make([]coresearch.BulkIndex, 0, len(docs))
	for _, d := range docs {
		if d.Commune != t {
			return errOtherCommune
		}
		items = append(items, coresearch.BulkIndex{ID: docID(t, d.ContactID), Routing: string(t), Doc: d.body()})
	}
	return x.es.Bulk(ctx, x.name, items)
}

// Unindex removes one contact's SEARCH DOCUMENT in the commune of ctx — never the row, which stays in
// PostgreSQL soft-deleted (rule 7). Absent already is success.
func (x *CitizenIndex) Unindex(ctx context.Context, contactID string) error {
	if err := x.Ready(); err != nil {
		return err
	}
	t := tenant.MustFrom(ctx)
	return x.es.DeleteDoc(ctx, x.name, docID(t, contactID), string(t))
}

// DeleteCommune removes every document of the commune of ctx — the first step of its reindex.
func (x *CitizenIndex) DeleteCommune(ctx context.Context) error {
	if err := x.Ready(); err != nil {
		return err
	}
	t := tenant.MustFrom(ctx)
	// tenant_id: the context's commune is the only term of this query.
	return x.es.DeleteByQuery(ctx, x.name, string(t), x.communeOnly(t, nil))
}

// --- queries ----------------------------------------------------------------------------------------------

// OptionFilter keeps contacts holding ANY of the values of one combobox / singleBox field.
type OptionFilter struct {
	Code   string
	Values []string
}

// RangeFilter keeps contacts whose number / dateTime value lies in [From, To]; one bound is enough.
type RangeFilter struct {
	Code     string
	From, To *float64
}

// Query is one directory search. The commune is NOT here — it comes from the context.
type Query struct {
	Keyword           string
	GroupID           string
	ResidentialUnitID string
	ActiveOnly        bool
	CreatedFrom       *int64 // epoch millis, inclusive
	CreatedTo         *int64 // epoch millis, inclusive
	Options           []OptionFilter
	Ranges            []RangeFilter
}

// After is the position of the last hit of the previous page: its created_at millis and contact id.
type After struct {
	CreatedAt int64
	ContactID string
}

// Page is one page of ids, newest first.
type Page struct {
	IDs     []string
	Next    *After // nil when there is no further page
	HasMore bool
}

// communeOnly is the bool query every request starts from: the commune filter (tenant_id), always first.
func (x *CitizenIndex) communeOnly(t tenant.ID, extra []any) map[string]any {
	filter := append([]any{map[string]any{"term": map[string]any{fieldTenant: string(t)}}}, extra...)
	return map[string]any{"bool": map[string]any{"filter": filter}}
}

// query builds the bool query of q in the commune t.
func (x *CitizenIndex) query(t tenant.ID, q Query) map[string]any {
	var filter []any
	if q.ActiveOnly {
		filter = append(filter, map[string]any{"term": map[string]any{"is_active": true}})
	}
	if q.GroupID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"group_ids": q.GroupID}})
	}
	if q.ResidentialUnitID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"residential_unit_id": q.ResidentialUnitID}})
	}
	if q.CreatedFrom != nil || q.CreatedTo != nil {
		r := map[string]any{}
		if q.CreatedFrom != nil {
			r["gte"] = *q.CreatedFrom
		}
		if q.CreatedTo != nil {
			r["lte"] = *q.CreatedTo
		}
		filter = append(filter, map[string]any{"range": map[string]any{"created_at": r}})
	}
	for _, o := range q.Options {
		vals := make([]string, 0, len(o.Values))
		for _, v := range o.Values {
			vals = append(vals, o.Code+"="+v)
		}
		filter = append(filter, map[string]any{"terms": map[string]any{"attr_kw": vals}})
	}
	for _, r := range q.Ranges {
		rng := map[string]any{}
		if r.From != nil {
			rng["gte"] = *r.From
		}
		if r.To != nil {
			rng["lte"] = *r.To
		}
		filter = append(filter, map[string]any{"nested": map[string]any{"path": "attr_num", "query": map[string]any{
			"bool": map[string]any{"filter": []any{
				map[string]any{"term": map[string]any{"attr_num.code": r.Code}},
				map[string]any{"range": map[string]any{"attr_num.value": rng}},
			}},
		}}})
	}
	b := x.communeOnly(t, filter)
	if must := x.keywordClauses(q.Keyword); len(must) > 0 {
		b["bool"].(map[string]any)["must"] = must
	}
	return b
}

func anyOf(clauses []any) map[string]any {
	return map[string]any{"bool": map[string]any{"should": clauses, "minimum_should_match": 1}}
}

// keywordClauses splits the keyword into the NAME part (words with no digit and no '@') and the CONTACT
// part (the rest), and requires both when both are present (ADR 0101 #7, #8):
//
//	name part      the name accent-insensitively — whole words, or word prefixes; a single bare word is
//	               also tried as an e-mail fragment (hashed)
//	contact part   the phone through ONE hashed gram, the e-mail through a hashed gram or the hashed whole
//	               address. It is NEVER sent as text: a digit or an address fragment in a query body would
//	               be personal data in Elasticsearch's slow log (rule 3). A contact part that hashes to
//	               nothing (two digits, an over-long number) matches nobody rather than being dropped.
func (x *CitizenIndex) keywordClauses(keyword string) []any {
	var nameWords, contactWords []string
	for _, w := range strings.Fields(keyword) {
		if strings.ContainsAny(w, "0123456789@") {
			contactWords = append(contactWords, w)
		} else {
			nameWords = append(nameWords, w)
		}
	}
	var must []any
	if len(nameWords) > 0 {
		name := strings.Join(nameWords, " ")
		should := []any{
			map[string]any{"match": map[string]any{"full_name": map[string]any{"query": name, "operator": "and"}}},
			map[string]any{"match": map[string]any{"full_name.prefix": map[string]any{"query": name, "operator": "and"}}},
		}
		if len(nameWords) == 1 && len(contactWords) == 0 {
			if gram, exact := x.key.EmailQuery(name); gram != "" || exact != "" {
				should = append(should, x.emailClauses(gram, exact)...)
			}
		}
		must = append(must, anyOf(should))
	}
	if len(contactWords) > 0 {
		contact := strings.Join(contactWords, " ")
		var should []any
		if h, ok := x.key.PhoneQuery(phoneDigits(contact)); ok {
			should = append(should, map[string]any{"term": map[string]any{"phone_grams": h}})
		}
		if len(contactWords) == 1 {
			should = append(should, x.emailClauses(x.key.EmailQuery(contact))...)
		}
		if len(should) == 0 {
			should = []any{map[string]any{"match_none": map[string]any{}}}
		}
		must = append(must, anyOf(should))
	}
	return must
}

func (x *CitizenIndex) emailClauses(gram, exact string) []any {
	var out []any
	if gram != "" {
		out = append(out, map[string]any{"term": map[string]any{"email_grams": gram}})
	}
	if exact != "" {
		out = append(out, map[string]any{"term": map[string]any{"email_exact": exact}})
	}
	return out
}

// phoneDigits turns a typed phone fragment into the digits the grams were cut from: separators removed;
// a whole number typed in international form ("+84 900…") is first brought to the stored national form
// (domain.NormalizeCitizenPhone), so it matches the 10-digit gram. "" when the keyword is not a phone.
func phoneDigits(k string) string {
	if national, err := domain.NormalizeCitizenPhone(k); err == nil {
		return national
	}
	compact := strings.NewReplacer(" ", "", ".", "", "-", "", "(", "", ")", "").Replace(k)
	compact = strings.TrimPrefix(compact, "+")
	for i := 0; i < len(compact); i++ {
		if compact[i] < '0' || compact[i] > '9' {
			return ""
		}
	}
	return compact
}

// Search returns one page of contact ids of the commune of ctx, newest first (created_at, then contact id,
// both descending — the PostgreSQL list's order).
func (x *CitizenIndex) Search(ctx context.Context, q Query, limit int, after *After) (Page, error) {
	if err := x.Ready(); err != nil {
		return Page{}, err
	}
	t := tenant.MustFrom(ctx)
	body := map[string]any{
		"query":            x.query(t, q),
		"size":             limit + 1,
		"_source":          false,
		"track_total_hits": false,
		"sort":             []any{map[string]any{"created_at": "desc"}, map[string]any{"contact_id": "desc"}},
	}
	if after != nil {
		body["search_after"] = []any{after.CreatedAt, after.ContactID}
	}
	// tenant_id: the context's commune routes the request and filters the query (x.query).
	res, err := x.es.Search(ctx, x.name, string(t), body)
	if err != nil {
		return Page{}, err
	}
	prefix := "t:" + string(t) + ":"
	out := Page{IDs: make([]string, 0, len(res.Hits))}
	for i, h := range res.Hits {
		if i == limit {
			out.HasMore = true
			break
		}
		if !strings.HasPrefix(h.ID, prefix) {
			// A document of another commune can only appear if the tenant filter were broken; refuse the
			// whole page rather than hand one id across (rule 1).
			return Page{}, errors.New("tìm kiếm danh bạ: chỉ mục trả về tài liệu ngoài xã")
		}
		id := strings.TrimPrefix(h.ID, prefix)
		out.IDs = append(out.IDs, id)
		if len(h.Sort) == 2 {
			var ms int64
			if json.Unmarshal(h.Sort[0], &ms) == nil {
				out.Next = &After{CreatedAt: ms, ContactID: id}
			}
		}
	}
	if !out.HasMore {
		out.Next = nil
	}
	return out, nil
}

// Count counts the contacts of the commune of ctx matching q.
func (x *CitizenIndex) Count(ctx context.Context, q Query) (int, error) {
	if err := x.Ready(); err != nil {
		return 0, err
	}
	t := tenant.MustFrom(ctx)
	// tenant_id: the context's commune routes the request and filters the query (x.query).
	n, err := x.es.Count(ctx, x.name, string(t), x.query(t, q))
	return int(n), err
}
