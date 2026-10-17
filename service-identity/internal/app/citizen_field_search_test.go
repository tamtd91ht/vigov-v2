package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/search"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ADR 0101 in the use cases: the attribute configuration (one entry per write, same transaction, ≤ 50 live),
// attribute values and e-mail on the contact writes, ONE outbox row in the write's transaction, the search
// plan (PostgreSQL / index / 503 / 400), the indexer's convergence, and the relay.

// --- fakes -----------------------------------------------------------------------------------------------

type fakeCitizenFields struct {
	mu      sync.Mutex
	rows    []domain.CitizenField
	inserts []domain.CitizenField
	updates []domain.CitizenField
	retired []string
	order   []string
}

func (f *fakeCitizenFields) List(_ context.Context, includeRetired bool) ([]domain.CitizenField, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.CitizenField
	for _, r := range f.rows {
		if includeRetired || r.Live() {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeCitizenFields) Get(_ context.Context, code string) (domain.CitizenField, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.Code == code {
			return r, true, nil
		}
	}
	return domain.CitizenField{}, false, nil
}
func (f *fakeCitizenFields) LockAndListLive(ctx context.Context, tx *store.ScopedTx) ([]domain.CitizenField, error) {
	if _, err := tx.Exec(ctx, "FAKE citizen_field lock"); err != nil {
		return nil, err
	}
	return f.List(ctx, false)
}
func (f *fakeCitizenFields) Insert(ctx context.Context, tx *store.ScopedTx, r domain.CitizenField) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_field insert", r.Code); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.rows {
		if x.Code == r.Code {
			return idstore.ErrCitizenFieldCodeTaken
		}
	}
	f.inserts = append(f.inserts, r)
	f.rows = append(f.rows, r)
	return nil
}
func (f *fakeCitizenFields) Update(ctx context.Context, tx *store.ScopedTx, r domain.CitizenField) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_field update", r.Code); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, r)
	return nil
}
func (f *fakeCitizenFields) Retire(ctx context.Context, tx *store.ScopedTx, code, by, reason string, at time.Time) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_field retire", code); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retired = append(f.retired, code+"|"+by)
	return nil
}
func (f *fakeCitizenFields) SetOrder(ctx context.Context, tx *store.ScopedTx, codes []string, _ time.Time) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_field order"); err != nil {
		return err
	}
	f.order = codes
	return nil
}

type fakeOutbox struct {
	mu  sync.Mutex
	evs []idstore.OutboxEvent
}

func (f *fakeOutbox) Record(ctx context.Context, tx *store.ScopedTx, evs []idstore.OutboxEvent) error {
	if _, err := tx.Exec(ctx, "FAKE event_outbox record"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.evs = append(f.evs, evs...)
	return nil
}

type fakeSearch struct {
	ready   error
	page    search.Page
	err     error
	lastQ   search.Query
	lastAft *search.After
	commune tenant.ID
}

func (f *fakeSearch) Ready() error { return f.ready }
func (f *fakeSearch) Search(ctx context.Context, q search.Query, _ int, after *search.After) (search.Page, error) {
	f.lastQ, f.lastAft, f.commune = q, after, tenant.MustFrom(ctx)
	return f.page, f.err
}
func (f *fakeSearch) Count(ctx context.Context, q search.Query) (int, error) {
	f.lastQ, f.commune = q, tenant.MustFrom(ctx)
	return 7, f.err
}

func sampleFields() []domain.CitizenField {
	return []domain.CitizenField{
		{Code: "hongheo1", Label: "Phân loại hộ", Type: domain.FieldCombobox, ShowInList: true,
			Options: []domain.CitizenFieldOption{{Value: "ho-ngheo", Label: "Hộ nghèo"}, {Value: "can-ngheo", Label: "Cận nghèo"}}},
		{Code: "sonhan01", Label: "Số nhân khẩu", Type: domain.FieldNumber, NumberKind: domain.NumberInteger},
	}
}

// --- contact writes ----------------------------------------------------------------------------------------

// Every write records ONE event in the SAME transaction as the row and its entry; the payload names the row
// id and the operation, nothing personal.
func TestContactWritesRecordTheEventInTheSameTransaction(t *testing.T) {
	h := newDirectoryHarness(t)
	h.fields.rows = sampleFields()
	req := sampleContactRequest()
	email := "Ten.Nguoi@Example.vn"
	req.Email = &email
	req.Attributes = map[string]json.RawMessage{"hongheo1": json.RawMessage(`["ho-ngheo"]`), "sonhan01": json.RawMessage(`4`)}
	id, err := h.uc.Create(h.ctx, nguoiThucHienGia(), req)
	if err != nil {
		t.Fatal(err)
	}
	c := h.contacts.inserted[0]
	if c.Email != "ten.nguoi@example.vn" || string(c.Attributes["sonhan01"]) != "4" {
		t.Errorf("email / attributes not stored normalised: %q %v", c.Email, c.Attributes)
	}
	ev := h.g.tim("FAKE event_outbox record")
	ins := h.g.tim("FAKE citizen_contact insert")
	if ev == nil || ev.tx == 0 || ev.tx != ins.tx || h.g.ketThucCua(ev.tx) != "commit" {
		t.Fatalf("outbox in tx %v, insert in tx %d", ev, ins.tx)
	}
	var payload map[string]string
	if len(h.outbox.evs) != 1 || h.outbox.evs[0].Name != EventCitizenContactChanged ||
		json.Unmarshal(h.outbox.evs[0].Payload, &payload) != nil || len(payload) != 2 ||
		payload["contact_id"] != id || payload["op"] != "upsert" {
		t.Errorf("event = %+v / %s — want exactly {contact_id, op}", h.outbox.evs, h.outbox.evs[0].Payload)
	}
	_, raw := directoryDelta(t, h.auditEntries()[0])
	if strings.Contains(raw, "ten.nguoi") || !strings.Contains(raw, `"co_email":true`) || !strings.Contains(raw, "hongheo1") {
		t.Errorf("create delta = %s", raw)
	}

	// Edit: email absent = kept, attributes not sent = kept; a change records one more event.
	h.contacts.rows[id] = c
	upd := sampleContactRequest()
	upd.Contact.FullName = "Nguyễn Văn B"
	if err := h.uc.Update(h.ctx, nguoiThucHienGia(), id, upd); err != nil {
		t.Fatal(err)
	}
	after := h.contacts.updated[0]
	if after.Email != c.Email || string(after.Attributes["hongheo1"]) != `["ho-ngheo"]` {
		t.Errorf("an edit without email/attributes must keep them: %+v", after)
	}
	if err := h.uc.Remove(h.ctx, nguoiThucHienGia(), id, "Chuyển đi"); err != nil {
		t.Fatal(err)
	}
	if len(h.outbox.evs) != 3 || h.outbox.evs[2].Name != EventCitizenContactChanged ||
		!strings.Contains(string(h.outbox.evs[2].Payload), `"op":"remove"`) {
		t.Errorf("events = %d, last %s", len(h.outbox.evs), h.outbox.evs[len(h.outbox.evs)-1].Payload)
	}
}

func TestContactAttributesValidatedAgainstDefinitions(t *testing.T) {
	h := newDirectoryHarness(t)
	h.fields.rows = sampleFields()
	h.fields.rows[1].Required = true
	req := sampleContactRequest()
	req.Attributes = map[string]json.RawMessage{"hongheo1": json.RawMessage(`["khong-co"]`), "sonhan01": json.RawMessage(`1`)}
	if _, err := h.uc.Create(h.ctx, nguoiThucHienGia(), req); !errors.Is(err, domain.ErrCitizenAttributeInvalid) || !IsCitizenDirectoryInputError(err) {
		t.Errorf("unknown option: %v", err)
	}
	req.Attributes = map[string]json.RawMessage{}
	if _, err := h.uc.Create(h.ctx, nguoiThucHienGia(), req); !errors.Is(err, domain.ErrCitizenAttributeRequired) {
		t.Errorf("required missing: %v", err)
	}
	if h.g.soGiaoDich() != 0 {
		t.Error("a refused form opened a transaction")
	}
}

func TestRevealEmailOneEntryInTheReadTransaction(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()
	c := h.contacts.rows[contactSampleID]
	c.Email = "ten.nguoi@example.vn"
	h.contacts.rows[contactSampleID] = c
	got, err := h.uc.RevealEmail(h.ctx, nguoiThucHienGia(), contactSampleID)
	if err != nil || got != c.Email {
		t.Fatalf("= %q %v", got, err)
	}
	read := h.g.tim("FAKE citizen_contact read email")
	e := h.auditEntries()
	if len(e) != 1 || e[0].tx != read.tx || e[0].args[4] != ActionRevealCitizenDirectoryEmail {
		t.Fatalf("entries %v", e)
	}
	_, raw := directoryDelta(t, e[0])
	if strings.Contains(raw, "ten.nguoi") {
		t.Errorf("the address reached the trail: %s", raw)
	}
	d, _ := h.uc.Get(h.ctx, contactSampleID)
	if d.EmailMasked == "" || strings.Contains(d.EmailMasked, "ten.nguoi") {
		t.Errorf("detail email must be masked: %q", d.EmailMasked)
	}
}

// --- search plan -------------------------------------------------------------------------------------------

func TestSearchPlan(t *testing.T) {
	h := newDirectoryHarness(t)
	h.fields.rows = sampleFields()
	h.contacts.listItems = []idstore.CitizenContactListItem{{ID: contactSampleID, Phone: dienThoaiGia}}
	req, _ := page.New(idstore.CitizenContactSorts, "", "", "20", "")
	from := int64(1)
	attr := CitizenContactSearch{Filter: idstore.CitizenContactFilter{Keyword: "an"},
		Options: []AttributeOptionFilter{{Code: "hongheo1", Values: []string{"ho-ngheo"}}}}

	// Not configured: keyword alone → PostgreSQL; attribute / created filters → 503.
	if r, err := h.uc.Search(h.ctx, CitizenContactSearch{Filter: idstore.CitizenContactFilter{Keyword: "an"}}, req); err != nil || len(r.Items) != 1 {
		t.Fatalf("keyword fallback: %v %v", r, err)
	}
	if _, err := h.uc.Search(h.ctx, attr, req); !errors.Is(err, ErrCitizenSearchUnavailable) {
		t.Errorf("attribute filter without index: %v", err)
	}
	if _, err := h.uc.CountSearch(h.ctx, CitizenContactSearch{CreatedFrom: &from}); !errors.Is(err, ErrCitizenSearchUnavailable) {
		t.Errorf("created filter without index: %v", err)
	}
	// Invalid filters are 400 before anything else.
	bad := CitizenContactSearch{Ranges: []AttributeRangeFilter{{Code: "hongheo1", From: ptrF(1)}}}
	if _, err := h.uc.Search(h.ctx, bad, req); !errors.Is(err, ErrCitizenSearchFilterInvalid) {
		t.Errorf("range on an option field: %v", err)
	}
	zalo := true
	combo := attr
	combo.Filter.HasOwnAppSession = &zalo
	if _, err := h.uc.Search(h.ctx, combo, req); !errors.Is(err, ErrCitizenSearchCombination) {
		t.Errorf("zalo + attribute: %v", err)
	}

	// Configured but off (key mismatch): every search is 503, the keyword one too.
	idx := &fakeSearch{ready: search.ErrKeyMismatch}
	h.uc.WithSearch(idx)
	if _, err := h.uc.Search(h.ctx, CitizenContactSearch{Filter: idstore.CitizenContactFilter{Keyword: "an"}}, req); !errors.Is(err, ErrCitizenSearchUnavailable) {
		t.Errorf("index off: %v", err)
	}

	// On: the index chooses ids, PostgreSQL hydrates (masked), the cursor follows the last hit.
	h.seedContact()
	idx.ready = nil
	idx.page = search.Page{IDs: []string{contactSampleID, "01JGONE0000000000000000000"}, HasMore: true,
		Next: &search.After{CreatedAt: 1700000000000, ContactID: "01JGONE0000000000000000000"}}
	r, err := h.uc.Search(h.ctx, attr, req)
	if err != nil || len(r.Items) != 1 || r.Items[0].PhoneMasked == dienThoaiGia || !r.HasMore || r.NextCursor == "" {
		t.Fatalf("index page: %+v %v", r, err)
	}
	if idx.commune != xaThu || len(idx.lastQ.Options) != 1 || idx.lastQ.Keyword != "an" {
		t.Errorf("index query %+v in commune %q", idx.lastQ, idx.commune)
	}
	next, err := page.New(idstore.CitizenContactSorts, "", "", "20", r.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.Search(h.ctx, attr, next); err != nil || idx.lastAft == nil || idx.lastAft.CreatedAt != 1700000000000 {
		t.Errorf("cursor round trip: %+v %v", idx.lastAft, err)
	}
	if n, err := h.uc.CountSearch(h.ctx, attr); err != nil || n != 7 {
		t.Errorf("count = %d %v", n, err)
	}
	// An index error is 503, never a quiet PostgreSQL answer.
	idx.err = errors.New("es down")
	if _, err := h.uc.Search(h.ctx, attr, req); !errors.Is(err, ErrCitizenSearchUnavailable) {
		t.Errorf("index error: %v", err)
	}
}

func ptrF(v float64) *float64 { return &v }

// --- attribute configuration --------------------------------------------------------------------------------

func newFieldsUC(t *testing.T) (*CitizenFields, *fakeCitizenFields, *ghiChep) {
	db, g := moDB(t)
	f := &fakeCitizenFields{}
	uc := NewCitizenFields(db, f)
	uc.now = func() time.Time { return directoryInstant }
	return uc, f, g
}

func auditOf(g *ghiChep) []lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []lenhGhi
	for _, l := range g.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			out = append(out, l)
		}
	}
	return out
}

func TestCitizenFieldCreateAuditsInTheSameTxAndCapsAt50(t *testing.T) {
	uc, f, g := newFieldsUC(t)
	ctx := ctxXa(xaThu)
	got, err := uc.Create(ctx, nguoiThucHienGia(), domain.CitizenFieldInput{Code: "sonhan01", Label: "Số nhân khẩu", Type: domain.FieldNumber, NumberKind: domain.NumberInteger, LayoutWidth: domain.LayoutHalf})
	if err != nil {
		t.Fatal(err)
	}
	if got.LayoutWidth != domain.LayoutHalf || got.DisplayMode != "" {
		t.Errorf("created field carries width %q mode %q", got.LayoutWidth, got.DisplayMode)
	}
	ins := g.tim("FAKE citizen_field insert")
	e := auditOf(g)
	if len(e) != 1 || e[0].tx != ins.tx || e[0].args[1] != maCanBo || e[0].args[4] != ActionAddCitizenField || e[0].args[5] != got.Code {
		t.Fatalf("entry %v, insert tx %d", e, ins.tx)
	}
	for i := len(f.rows); i < domain.MaxCitizenFieldsLive; i++ {
		f.rows = append(f.rows, domain.CitizenField{Code: "f" + strings.Repeat("x", 4) + string(rune('a'+i%26)) + string(rune('a'+i/26)), SortOrder: i})
	}
	if _, err := uc.Create(ctx, nguoiThucHienGia(), domain.CitizenFieldInput{Code: "thuocmoi", Label: "Mới", Type: domain.FieldSingleText, LayoutWidth: domain.LayoutHalf}); !errors.Is(err, ErrCitizenFieldLimit) {
		t.Errorf("51st live field: %v", err)
	}
	if _, err := uc.Create(ctx, nguoiThucHienGia(), domain.CitizenFieldInput{Code: "x", Label: "Mới", Type: domain.FieldSingleText}); !IsCitizenFieldInputError(err) {
		t.Errorf("bad code must be a 400: %v", err)
	}
}

// display_mode / layout_width (migration 0038) are editable, written with the row, and their before/after
// reach the trail entry of the same transaction.
func TestCitizenFieldDisplayModeAndWidthEditIsAudited(t *testing.T) {
	uc, f, g := newFieldsUC(t)
	ctx := ctxXa(xaThu)
	f.rows = sampleFields()
	f.rows[0].DisplayMode, f.rows[0].LayoutWidth = domain.DisplayFull, domain.LayoutHalf
	dd, full := domain.DisplayDropdown, domain.LayoutFull
	got, err := uc.Update(ctx, nguoiThucHienGia(), "hongheo1", domain.CitizenFieldPatch{DisplayMode: &dd, LayoutWidth: &full})
	if err != nil || got.DisplayMode != dd || got.LayoutWidth != full || len(f.updates) != 1 || f.updates[0].DisplayMode != dd || f.updates[0].LayoutWidth != full {
		t.Fatalf("mode/width edit: %+v %v %+v", got, err, f.updates)
	}
	entries := auditOf(g)
	if len(entries) != 1 || entries[0].tx != g.tim("FAKE citizen_field update").tx {
		t.Fatalf("entries %v", entries)
	}
	var b strings.Builder
	for _, a := range entries[0].args {
		if s, ok := a.(string); ok {
			b.WriteString(s)
		} else if bs, ok := a.([]byte); ok {
			b.Write(bs)
		}
	}
	delta := b.String()
	for _, want := range []string{"display_mode", "layout_width", `"cach_hien_thi":"full"`, `"cach_hien_thi":"dropdown"`, `"do_rong":"half"`, `"do_rong":"full"`} {
		if !strings.Contains(delta, want) {
			t.Errorf("mode/width delta lacks %s: %s", want, delta)
		}
	}
}

func TestCitizenFieldUpdateRetireReorder(t *testing.T) {
	uc, f, g := newFieldsUC(t)
	ctx := ctxXa(xaThu)
	f.rows = sampleFields()
	f.rows[1].SortOrder = 1
	label := "Phân loại hộ gia đình"
	if _, err := uc.Update(ctx, nguoiThucHienGia(), "hongheo1", domain.CitizenFieldPatch{Label: &label}); err != nil {
		t.Fatal(err)
	}
	same := f.rows[0].Label
	if _, err := uc.Update(ctx, nguoiThucHienGia(), "hongheo1", domain.CitizenFieldPatch{Label: &same}); err != nil || len(f.updates) != 1 {
		t.Errorf("a no-op edit wrote: %v / %d", err, len(f.updates))
	}
	typ := domain.FieldSingleBox
	if _, err := uc.Update(ctx, nguoiThucHienGia(), "hongheo1", domain.CitizenFieldPatch{Type: &typ}); !errors.Is(err, domain.ErrCitizenFieldImmutable) {
		t.Errorf("type change: %v", err)
	}
	if _, err := uc.Update(ctx, nguoiThucHienGia(), "zzzzzz", domain.CitizenFieldPatch{Label: &label}); !errors.Is(err, idstore.ErrCitizenFieldNotFound) {
		t.Errorf("unknown code: %v", err)
	}
	if err := uc.Retire(ctx, nguoiThucHienGia(), "hongheo1", ""); !errors.Is(err, domain.ErrCitizenFieldReasonMissing) {
		t.Errorf("retire without reason: %v", err)
	}
	if err := uc.Retire(ctx, nguoiThucHienGia(), "hongheo1", "Không dùng nữa"); err != nil || f.retired[0] != "hongheo1|"+maCanBo {
		t.Errorf("retire: %v %v", err, f.retired)
	}
	if err := uc.Reorder(ctx, nguoiThucHienGia(), []string{"hongheo1"}); !errors.Is(err, domain.ErrCitizenFieldOrderMismatch) {
		t.Errorf("partial order: %v", err)
	}
	if err := uc.Reorder(ctx, nguoiThucHienGia(), []string{"sonhan01", "hongheo1"}); err != nil || strings.Join(f.order, ",") != "sonhan01,hongheo1" {
		t.Errorf("reorder: %v %v", err, f.order)
	}
	n := len(auditOf(g))
	if err := uc.Reorder(ctx, nguoiThucHienGia(), []string{"hongheo1", "sonhan01"}); err != nil || len(auditOf(g)) != n {
		t.Errorf("the stored order again must write nothing: %v", err)
	}
	if n != 3 { // edit, retire, reorder
		t.Errorf("%d entries, want 3", n)
	}
}

// --- indexer ------------------------------------------------------------------------------------------------

type fakeIndexWriter struct {
	ready    error
	upserts  []search.Doc
	unindex  []string
	cleared  int
	adopted  bool
	communes []tenant.ID
}

func (f *fakeIndexWriter) Ready() error { return f.ready }
func (f *fakeIndexWriter) BuildDoc(t tenant.ID, c domain.CitizenContact, g []string, _ []domain.CitizenField) search.Doc {
	return search.Doc{Commune: t, ContactID: c.ID, GroupIDs: g}
}
func (f *fakeIndexWriter) Upsert(ctx context.Context, d search.Doc) error {
	f.communes = append(f.communes, tenant.MustFrom(ctx))
	f.upserts = append(f.upserts, d)
	return nil
}
func (f *fakeIndexWriter) BulkUpsert(ctx context.Context, docs []search.Doc) error {
	f.communes = append(f.communes, tenant.MustFrom(ctx))
	f.upserts = append(f.upserts, docs...)
	return nil
}
func (f *fakeIndexWriter) Unindex(_ context.Context, id string) error {
	f.unindex = append(f.unindex, id)
	return nil
}
func (f *fakeIndexWriter) DeleteCommune(context.Context) error { f.cleared++; return nil }
func (f *fakeIndexWriter) DeleteEverything(context.Context) error {
	f.cleared += 100
	return nil
}
func (f *fakeIndexWriter) AdoptKey(context.Context) error { f.adopted, f.ready = true, nil; return nil }

type indexerContacts struct{ *fakeCitizenContacts }

func (c indexerContacts) LiveIDsAfter(_ context.Context, after string, _ int) ([]string, error) {
	if after != "" {
		return nil, nil
	}
	var out []string
	for id := range c.rows {
		out = append(out, id)
	}
	return out, nil
}

type communesFake []tenant.ID

func (c communesFake) CommunesWithContacts(context.Context) ([]tenant.ID, error) { return c, nil }

func TestIndexerConvergesOnTheRow(t *testing.T) {
	contacts := newFakeCitizenContacts()
	contacts.rows[contactSampleID] = domain.CitizenContact{ID: contactSampleID}
	w := &fakeIndexWriter{}
	ix := NewCitizenSearchIndexer(indexerContacts{contacts}, &fakeCitizenFields{}, w, nil)
	env := func(id string) events.Envelope {
		return events.Envelope{ID: "e", Name: EventCitizenContactChanged, TenantID: xaThu,
			Payload: json.RawMessage(`{"contact_id":"` + id + `","op":"upsert"}`)}
	}
	for _, id := range []string{contactSampleID, contactSampleID, contactSecondID} {
		if err := events.Dispatch(context.Background(), env(id), ix.Handle); err != nil {
			t.Fatal(err)
		}
	}
	// Twice the same event = the same document twice (idempotent); a row not found is unindexed.
	if len(w.upserts) != 2 || w.upserts[0].Commune != xaThu || len(w.unindex) != 1 || w.unindex[0] != contactSecondID {
		t.Errorf("upserts %+v unindex %v", w.upserts, w.unindex)
	}
	bad := env("")
	if err := events.Dispatch(context.Background(), bad, ix.Handle); !errors.Is(err, events.ErrPermanent) {
		t.Errorf("empty id: %v", err)
	}
	w.ready = search.ErrKeyMismatch
	if err := events.Dispatch(context.Background(), env(contactSampleID), ix.Handle); err == nil || errors.Is(err, events.ErrPermanent) {
		t.Errorf("index off must be a TRANSIENT error (retried, never committed past): %v", err)
	}
	// Per-commune reindex refuses while off; ReindexAll adopts the key and rebuilds every commune.
	if _, err := ix.ReindexCommune(ctxXa(xaThu)); err == nil {
		t.Error("reindex of one commune with a mismatching key must refuse")
	}
	n, err := ix.ReindexAll(context.Background(), communesFake{xaThu})
	if err != nil || n != 1 || !w.adopted || w.cleared < 100 {
		t.Errorf("ReindexAll = %d %v adopted=%v cleared=%d", n, err, w.adopted, w.cleared)
	}
}

// --- relay --------------------------------------------------------------------------------------------------

type fakeQueue struct {
	rows      map[tenant.ID][]idstore.OutboxEvent
	published []string
	failed    []string
}

func (q *fakeQueue) Unpublished(ctx context.Context, _ int) ([]idstore.OutboxEvent, error) {
	return q.rows[tenant.MustFrom(ctx)], nil
}
func (q *fakeQueue) MarkPublished(_ context.Context, ids []string, _ time.Time) error {
	q.published = append(q.published, ids...)
	return nil
}
func (q *fakeQueue) RecordFailedAttempt(_ context.Context, id string) error {
	q.failed = append(q.failed, id)
	return nil
}

type pendingFake []tenant.ID

func (p pendingFake) CommunesWithUnpublishedEvents(context.Context) ([]tenant.ID, error) {
	return p, nil
}

type fakePublisher struct {
	sent []events.Envelope
	err  error
}

func (p *fakePublisher) Publish(_ context.Context, e events.Envelope) error {
	if p.err != nil {
		return p.err
	}
	p.sent = append(p.sent, e)
	return nil
}

func TestRelayPublishesWithTheRowsCommuneAndStampsAfterAck(t *testing.T) {
	q := &fakeQueue{rows: map[tenant.ID][]idstore.OutboxEvent{
		xaThu: {{ID: "E1", Name: EventCitizenContactChanged, Payload: []byte(`{"contact_id":"c","op":"upsert"}`)}},
	}}
	pub := &fakePublisher{}
	r := NewEventRelay(pendingFake{xaThu}, q, pub, nil)
	if !r.Tick(context.Background()) {
		t.Fatal("healthy tick reported an outage")
	}
	if len(pub.sent) != 1 || pub.sent[0].TenantID != xaThu || pub.sent[0].ID != "E1" || len(q.published) != 1 {
		t.Errorf("sent %+v published %v", pub.sent, q.published)
	}
	pub.err = errors.New("broker down")
	q.published = nil
	if r.Tick(context.Background()) || len(q.published) != 0 || len(q.failed) != 1 {
		t.Errorf("a refused publish must leave the row owed: published %v failed %v", q.published, q.failed)
	}
}
