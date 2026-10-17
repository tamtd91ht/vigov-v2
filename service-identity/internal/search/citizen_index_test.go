package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	coresearch "github.com/vihat/vigov/core/search"
	"github.com/vihat/vigov/core/searchhash"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

const (
	communeA = tenant.ID("01J0000000000000000000000A")
	communeB = tenant.ID("01J0000000000000000000000B")
)

type fakeES struct {
	exists   bool
	meta     map[string]any
	lastBody any
	lastRout string
	puts     map[string]any
	hits     []coresearch.Hit
}

func (f *fakeES) EnsureIndex(_ context.Context, _ string, def any) (bool, error) {
	if f.exists {
		return false, nil
	}
	f.exists = true
	f.meta = def.(map[string]any)["mappings"].(map[string]any)["_meta"].(map[string]any)
	return true, nil
}
func (f *fakeES) Meta(context.Context, string) (map[string]any, error) { return f.meta, nil }
func (f *fakeES) PutMeta(_ context.Context, _ string, m map[string]any) error {
	f.meta = m
	return nil
}
func (f *fakeES) PutDoc(_ context.Context, _, id, routing string, doc any) error {
	if f.puts == nil {
		f.puts = map[string]any{}
	}
	f.puts[id], f.lastRout = doc, routing
	return nil
}
func (f *fakeES) DeleteDoc(_ context.Context, _, _, routing string) error {
	f.lastRout = routing
	return nil
}
func (f *fakeES) Search(_ context.Context, _, routing string, body any) (coresearch.SearchResult, error) {
	f.lastBody, f.lastRout = body, routing
	return coresearch.SearchResult{Hits: f.hits}, nil
}
func (f *fakeES) Count(_ context.Context, _, routing string, q any) (int64, error) {
	f.lastBody, f.lastRout = q, routing
	return 3, nil
}
func (f *fakeES) DeleteByQuery(_ context.Context, _, routing string, q any) error {
	f.lastBody, f.lastRout = q, routing
	return nil
}
func (f *fakeES) Bulk(context.Context, string, []coresearch.BulkIndex) error { return nil }

func testKey(t *testing.T, fill byte) searchhash.Key {
	t.Helper()
	k, err := searchhash.NewKey(secret.Secret(bytes.Repeat([]byte{fill}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func ctxOf(t tenant.ID) context.Context { return tenant.Into(context.Background(), t) }

// The owner's guard (17/10/2026): an index built with another SEARCH_HASH_KEY is opened OFF — no search,
// no write — until the reindex path adopts the new key.
func TestOpenWithAnotherKeyLeavesTheIndexOff(t *testing.T) {
	fake := &fakeES{}
	if err := New(fake, testKey(t, 1), "vigov-dev").Open(context.Background()); err != nil {
		t.Fatalf("first open creates the index: %v", err)
	}
	if fake.meta["key_fingerprint"] != testKey(t, 1).Fingerprint() {
		t.Fatalf("the fingerprint is not recorded: %v", fake.meta)
	}
	other := New(fake, testKey(t, 2), "vigov-dev")
	if err := other.Open(context.Background()); !errors.Is(err, ErrKeyMismatch) {
		t.Fatalf("a different key must be refused: %v", err)
	}
	if !strings.Contains(ErrKeyMismatch.Error(), "SEARCH_HASH_KEY") {
		t.Error("the refusal must name the variable")
	}
	ctx := ctxOf(communeA)
	if _, err := other.Search(ctx, Query{Keyword: "0900"}, 10, nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("search with the wrong key: %v", err)
	}
	if err := other.Upsert(ctx, Doc{Commune: communeA, ContactID: "c"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("write with the wrong key: %v", err)
	}
	if fake.puts != nil {
		t.Error("nothing may be written with the wrong key")
	}
	// The reindex path adopts the new key, and only then is the index on.
	if err := other.AdoptKey(context.Background()); err != nil || other.Ready() != nil {
		t.Fatalf("AdoptKey: %v / %v", err, other.Ready())
	}
	if fake.meta["key_fingerprint"] != testKey(t, 2).Fingerprint() {
		t.Error("AdoptKey must rewrite the fingerprint")
	}
	if err := New(fake, testKey(t, 1), "vigov-dev").Open(context.Background()); !errors.Is(err, ErrKeyMismatch) {
		t.Error("the old key is now the mismatching one")
	}
}

func TestNilAndUnconfiguredIndexIsOff(t *testing.T) {
	var x *CitizenIndex
	if x.Ready() == nil {
		t.Error("nil index must be off")
	}
	if err := New(&fakeES{}, searchhash.Key{}, "p").Open(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no key: %v", err)
	}
}

// Every query carries the commune of the CONTEXT as its first filter and its routing.
func TestEveryQueryFiltersTheContextCommune(t *testing.T) {
	fake := &fakeES{}
	x := New(fake, testKey(t, 1), "vigov-dev")
	if err := x.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	from := 5.0
	created := int64(1700000000000)
	q := Query{Keyword: "Nguyễn 0900", GroupID: "G", ActiveOnly: true, CreatedFrom: &created,
		Options: []OptionFilter{{Code: "hongheo1", Values: []string{"ho-ngheo", "can-ngheo"}}},
		Ranges:  []RangeFilter{{Code: "sonhan01", From: &from}}}
	if _, err := x.Search(ctxOf(communeA), q, 20, &After{CreatedAt: 1, ContactID: "x"}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(fake.lastBody)
	s := string(body)
	if fake.lastRout != string(communeA) || !strings.Contains(s, `{"term":{"tenant_id":"`+string(communeA)+`"}}`) {
		t.Errorf("tenant filter or routing missing: routing %q body %s", fake.lastRout, s)
	}
	for _, want := range []string{`"hongheo1=ho-ngheo"`, `"attr_num.code":"sonhan01"`, `"gte":5`, `"search_after":[1,"x"]`, `"size":21`} {
		if !strings.Contains(s, want) {
			t.Errorf("query lacks %s: %s", want, s)
		}
	}
	if strings.Contains(s, "0900") || !strings.Contains(s, `"phone_grams"`) || !strings.Contains(s, `"query":"Nguyễn"`) {
		t.Errorf("the phone fragment must travel hashed, the name as text, both required: %s", s)
	}
	for _, kw := range []string{"ten.nguoi@example.vn", "+84 900 000 000"} {
		if _, err := x.Search(ctxOf(communeA), Query{Keyword: kw}, 20, nil); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(fake.lastBody)
		if strings.Contains(string(b), "example") || strings.Contains(string(b), "900") || strings.Contains(string(b), "full_name") {
			t.Errorf("%q reached the query as text: %s", kw, b)
		}
	}
	if _, err := x.Count(ctxOf(communeB), Query{}); err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(fake.lastBody)
	if fake.lastRout != string(communeB) || !strings.Contains(string(body), string(communeB)) || strings.Contains(string(body), string(communeA)) {
		t.Errorf("count must filter commune B only: %s", body)
	}
}

// A hit outside the commune's id prefix refuses the whole page (rule 1).
func TestSearchRefusesForeignHits(t *testing.T) {
	fake := &fakeES{hits: []coresearch.Hit{{ID: "t:" + string(communeB) + ":c1", Sort: []json.RawMessage{json.RawMessage("1"), json.RawMessage(`"c1"`)}}}}
	x := New(fake, testKey(t, 1), "")
	_ = x.Open(context.Background())
	if _, err := x.Search(ctxOf(communeA), Query{}, 10, nil); err == nil {
		t.Error("a document of commune B returned to commune A must be refused")
	}
	fake.hits = []coresearch.Hit{
		{ID: "t:" + string(communeA) + ":c2", Sort: []json.RawMessage{json.RawMessage("20"), json.RawMessage(`"c2"`)}},
		{ID: "t:" + string(communeA) + ":c1", Sort: []json.RawMessage{json.RawMessage("10"), json.RawMessage(`"c1"`)}},
	}
	p, err := x.Search(ctxOf(communeA), Query{}, 1, nil)
	if err != nil || len(p.IDs) != 1 || p.IDs[0] != "c2" || !p.HasMore || p.Next == nil || p.Next.CreatedAt != 20 {
		t.Errorf("page = %+v, %v", p, err)
	}
}

func TestBuildDocHoldsNoPhoneOrEmail(t *testing.T) {
	x := New(&fakeES{}, testKey(t, 1), "")
	unit := "01JUNIT0000000000000000000"
	c := domain.CitizenContact{ID: "c1", FullName: "Nguyễn Văn A", Phone: "0900000000", Email: "ten.nguoi@example.vn",
		ResidentialUnitID: &unit, Tags: []string{"t"}, IsActive: true, CreatedAt: time.UnixMilli(1700000000000),
		Attributes: map[string]json.RawMessage{"hongheo1": json.RawMessage(`["ho-ngheo"]`), "sonhan01": json.RawMessage(`4`),
			"ngaysinh": json.RawMessage(`-315619200000`), "ghichu01": json.RawMessage(`"ghi chú"`)}}
	defs := []domain.CitizenField{
		{Code: "hongheo1", Type: domain.FieldCombobox}, {Code: "sonhan01", Type: domain.FieldNumber, NumberKind: domain.NumberInteger},
		{Code: "ngaysinh", Type: domain.FieldDateTime}, {Code: "ghichu01", Type: domain.FieldSingleText},
	}
	d := x.BuildDoc(communeA, c, []string{"G"}, defs)
	b, _ := json.Marshal(d.body())
	s := string(b)
	for _, leak := range []string{"0900000000", "ten.nguoi", "example.vn", "ghi chú"} {
		if strings.Contains(s, leak) {
			t.Errorf("the document holds %q: %s", leak, s)
		}
	}
	for _, want := range []string{`"tenant_id":"` + string(communeA) + `"`, `"hongheo1=ho-ngheo"`, `"value":4`, `"value":-315619200000`, `"created_at":1700000000000`} {
		if !strings.Contains(s, want) {
			t.Errorf("document lacks %s: %s", want, s)
		}
	}
	if len(d.PhoneGrams) == 0 || d.EmailExact == "" {
		t.Error("hashes missing")
	}
	// A document of commune A is never written under commune B's context.
	_ = x.AdoptKey(context.Background())
	if err := x.Upsert(ctxOf(communeB), d); err == nil {
		t.Error("a commune-A document written in commune B's context")
	}
}

func TestIndexName(t *testing.T) {
	for in, want := range map[string]string{"vigov-prod": "vigov-prod-citizen-contacts", "vigov-dev-": "vigov-dev-citizen-contacts", "": "citizen-contacts"} {
		if got := IndexName(in); got != want {
			t.Errorf("IndexName(%q) = %q, want %q", in, got, want)
		}
	}
}
