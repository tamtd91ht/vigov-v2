package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The directory use cases over IN-MEMORY store fakes and the shared fake driver (moDB). What is
// worth proving here is the use case's own contract: the mask on every read, ONE audit entry per
// write in the SAME transaction as the business write, nothing personal in any delta, the reveal's
// one-entry-per-call ordering, and that a refusal commits nothing. The real SQL is the store's.
//
// EVERY FAKE METHOD THAT TAKES THE TRANSACTION WRITES A MARKER STATEMENT THROUGH IT, so the tape
// (ghiChep) shows the business write and the audit entry side by side with their transaction ids —
// "same transaction" is read off the tape, not trusted.

const (
	contactSampleID   = "01JCONTACTSAMPLE0000000001"
	contactSecondID   = "01JCONTACTSAMPLE0000000002"
	groupSampleID     = "01JGROUPSAMPLE000000000001"
	groupSecondID     = "01JGROUPSAMPLE000000000002"
	unitSampleID      = "01JRESDNT0000000000000000A" // ULID-shaped: domain.NormalizeCitizenContactUnitRef
	unitSecondID      = "01JRESDNT0000000000000000B"
	directoryIDPrefix = "01JDIRECTORYNEWID00000000"
	phoneSecondFake   = "0900000001"
	phoneThirdFake    = "0900000002"
	nameSample        = "Nguyễn Văn A"
	addressSample     = "Số 1, đường Lê Lợi"
)

var directoryInstant = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

type softDeleteCall struct{ id, by, reason string }
type setGroupsCall struct {
	contactID string
	groupIDs  []string
	by        string
}

type fakeCitizenContacts struct {
	mu           sync.Mutex
	rows         map[string]domain.CitizenContact
	memberships  map[string][]string
	takenPhones  map[string]bool // numbers "already in the directory" besides rows
	listItems    []idstore.CitizenContactListItem
	tagCounts    []idstore.TagCount
	memberCounts map[string]int

	inserted  []domain.CitizenContact
	updated   []domain.CitizenContact
	deleted   []softDeleteCall
	setGroups []setGroupsCall
}

func newFakeCitizenContacts() *fakeCitizenContacts {
	return &fakeCitizenContacts{
		rows: map[string]domain.CitizenContact{}, memberships: map[string][]string{},
		takenPhones: map[string]bool{}, memberCounts: map[string]int{},
	}
}

func (f *fakeCitizenContacts) List(context.Context, idstore.CitizenContactFilter, page.Request) (page.Result[idstore.CitizenContactListItem], error) {
	return page.Result[idstore.CitizenContactListItem]{Items: f.listItems}, nil
}
func (f *fakeCitizenContacts) Count(context.Context, idstore.CitizenContactFilter) (int, error) {
	return len(f.listItems), nil
}
func (f *fakeCitizenContacts) TagCounts(context.Context) ([]idstore.TagCount, error) {
	return f.tagCounts, nil
}
func (f *fakeCitizenContacts) Get(_ context.Context, id string) (domain.CitizenContact, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.rows[id]
	return c, ok, nil
}
func (f *fakeCitizenContacts) GroupIDsOf(_ context.Context, ids []string) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string][]string{}
	for _, id := range ids {
		if g, ok := f.memberships[id]; ok {
			out[id] = g
		}
	}
	return out, nil
}
func (f *fakeCitizenContacts) MemberCounts(context.Context) (map[string]int, error) {
	return f.memberCounts, nil
}
func (f *fakeCitizenContacts) ReadPhone(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error) {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact read phone", id); err != nil {
		return "", false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.rows[id]
	return c.Phone, ok, nil
}
func (f *fakeCitizenContacts) PhoneTaken(ctx context.Context, tx *store.ScopedTx, phone, exceptID string) (bool, error) {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact phone taken", exceptID); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.takenPhones[phone] {
		return true, nil
	}
	for id, c := range f.rows {
		if id != exceptID && c.Phone == phone {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCitizenContacts) Insert(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact insert", c.ID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, c)
	f.rows[c.ID] = c
	return nil
}
func (f *fakeCitizenContacts) Update(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact update", c.ID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, c)
	f.rows[c.ID] = c
	return nil
}
func (f *fakeCitizenContacts) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, _ time.Time) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact soft delete", id); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return idstore.ErrCitizenContactNotFound
	}
	f.deleted = append(f.deleted, softDeleteCall{id: id, by: by, reason: reason})
	delete(f.rows, id)
	return nil
}
func (f *fakeCitizenContacts) SetGroups(ctx context.Context, tx *store.ScopedTx, contactID string, groupIDs []string, by string, _ time.Time) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_contact_group set", contactID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setGroups = append(f.setGroups, setGroupsCall{contactID: contactID, groupIDs: groupIDs, by: by})
	f.memberships[contactID] = groupIDs
	return nil
}

type fakeCitizenGroups struct {
	mu       sync.Mutex
	rows     map[string]domain.CitizenGroup
	inserted []domain.CitizenGroup
	updated  []domain.CitizenGroup
	deleted  []softDeleteCall
}

func newFakeCitizenGroups() *fakeCitizenGroups {
	return &fakeCitizenGroups{rows: map[string]domain.CitizenGroup{}}
}

func (f *fakeCitizenGroups) List(context.Context) ([]domain.CitizenGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.CitizenGroup, 0, len(f.rows))
	for _, g := range f.rows {
		out = append(out, g)
	}
	return out, nil
}
func (f *fakeCitizenGroups) Get(_ context.Context, id string) (domain.CitizenGroup, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.rows[id]
	return g, ok, nil
}
func (f *fakeCitizenGroups) IDsByNames(_ context.Context, names []string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for _, n := range names {
		for id, g := range f.rows {
			if strings.EqualFold(g.Name, n) {
				out[n] = id
			}
		}
	}
	return out, nil
}
func (f *fakeCitizenGroups) NameTaken(ctx context.Context, tx *store.ScopedTx, name, exceptID string) (bool, error) {
	if _, err := tx.Exec(ctx, "FAKE citizen_group name taken", exceptID); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, g := range f.rows {
		if id != exceptID && strings.EqualFold(g.Name, name) {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCitizenGroups) Insert(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_group insert", g.ID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, g)
	f.rows[g.ID] = g
	return nil
}
func (f *fakeCitizenGroups) Update(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_group update", g.ID); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, g)
	f.rows[g.ID] = g
	return nil
}
func (f *fakeCitizenGroups) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, _ time.Time) error {
	if _, err := tx.Exec(ctx, "FAKE citizen_group soft delete", id); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return idstore.ErrCitizenGroupNotFound
	}
	f.deleted = append(f.deleted, softDeleteCall{id: id, by: by, reason: reason})
	delete(f.rows, id)
	return nil
}

// --- harness ---------------------------------------------------------------------------------

type directoryHarness struct {
	uc       *CitizenDirectory
	g        *ghiChep
	contacts *fakeCitizenContacts
	groups   *fakeCitizenGroups
	ctx      context.Context
}

func newDirectoryHarness(t *testing.T) *directoryHarness {
	t.Helper()
	db, g := moDB(t)
	contacts, groups := newFakeCitizenContacts(), newFakeCitizenGroups()
	uc := NewCitizenDirectory(db, contacts, groups)
	n := 0
	uc.newID = func() (string, error) {
		n++
		return directoryIDPrefix + string(rune('A'+n-1)), nil
	}
	uc.now = func() time.Time { return directoryInstant }
	groups.rows[groupSampleID] = domain.CitizenGroup{ID: groupSampleID, Name: "Tổ trưởng dân phố"}
	groups.rows[groupSecondID] = domain.CitizenGroup{ID: groupSecondID, Name: "Hộ kinh doanh"}
	return &directoryHarness{uc: uc, g: g, contacts: contacts, groups: groups, ctx: ctxXa(xaThu)}
}

func (h *directoryHarness) seedContact() {
	unit := unitSampleID
	h.contacts.rows[contactSampleID] = domain.CitizenContact{
		ID: contactSampleID, FullName: nameSample, Phone: dienThoaiGia, ResidentialUnitID: &unit,
		Address: addressSample, Note: "ghi chú", Tags: []string{"tổ trưởng"},
		Source: domain.CitizenContactSourceManual, IsActive: true,
		CreatedAt: directoryInstant.Add(-time.Hour), UpdatedAt: directoryInstant.Add(-time.Hour),
	}
	h.contacts.memberships[contactSampleID] = []string{groupSampleID}
}

func sampleContactRequest() CitizenContactRequest {
	return CitizenContactRequest{
		Contact: domain.CitizenContactInput{
			FullName: nameSample, Phone: dienThoaiGia, ResidentialUnitID: unitSampleID,
			Address: addressSample, Note: "ghi chú", Tags: []string{"tổ trưởng"}, IsActive: true,
		},
		GroupIDs: []string{groupSampleID},
	}
}

// statements returns every statement on the tape containing manh, in order.
func (h *directoryHarness) statements(manh string) []lenhGhi {
	h.g.mu.Lock()
	defer h.g.mu.Unlock()
	var out []lenhGhi
	for _, l := range h.g.lenh {
		if strings.Contains(l.sql, manh) {
			out = append(out, l)
		}
	}
	return out
}

func (h *directoryHarness) auditEntries() []lenhGhi { return h.statements("INSERT INTO audit_log") }

// directoryDelta decodes an audit statement's delta ($8).
func directoryDelta(t *testing.T, e lenhGhi) (map[string]any, string) {
	t.Helper()
	raw := e.args[7].([]byte)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("delta: %v: %s", err, raw)
	}
	return m, string(raw)
}

// assertNoPersonalData fails when the text carries the sample person's name, number, address,
// note or label — the values rule 6 forbidden #4 keeps out of the trail.
func assertNoPersonalData(t *testing.T, what, text string) {
	t.Helper()
	for _, v := range []string{nameSample, dienThoaiGia, phoneSecondFake, addressSample, "ghi chú", "tổ trưởng"} {
		if strings.Contains(text, v) {
			t.Errorf("%s carries personal data: %s", what, text)
		}
	}
}

// --- reads: the mask ---------------------------------------------------------------------------

func TestCitizenDirectoryListAndDetailMaskThePhone(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()
	h.contacts.listItems = []idstore.CitizenContactListItem{{ID: contactSampleID, FullName: nameSample, Phone: dienThoaiGia}}

	list, err := h.uc.List(h.ctx, idstore.CitizenContactFilter{}, page.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].PhoneMasked != "09****0000" {
		t.Fatalf("list = %+v, want the masked number", list.Items)
	}
	detail, err := h.uc.Get(h.ctx, contactSampleID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PhoneMasked != "09****0000" || detail.Address != addressSample || detail.Note != "ghi chú" {
		t.Errorf("detail = %+v, want masked number with address and note", detail)
	}
	if len(detail.GroupIDs) != 1 || detail.GroupIDs[0] != groupSampleID {
		t.Errorf("detail groups = %v", detail.GroupIDs)
	}
	if _, err := h.uc.Get(h.ctx, "unknown"); !errors.Is(err, idstore.ErrCitizenContactNotFound) {
		t.Errorf("unknown id: %v", err)
	}
}

// --- the reveal --------------------------------------------------------------------------------

func TestCitizenDirectoryRevealPhoneOneEntryPerCallInOneCommittedTx(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()

	for i := 1; i <= 2; i++ {
		phone, err := h.uc.RevealPhone(h.ctx, nguoiThucHienGia(), contactSampleID)
		if err != nil || phone != dienThoaiGia {
			t.Fatalf("call %d: phone %q err %v", i, phone, err)
		}
	}
	reads := h.statements("FAKE citizen_contact read phone")
	entries := h.auditEntries()
	if len(reads) != 2 || len(entries) != 2 {
		t.Fatalf("%d reads, %d entries — want one entry PER CALL", len(reads), len(entries))
	}
	for i := range entries {
		r, e := reads[i], entries[i]
		if r.tx == 0 || r.tx != e.tx || h.g.ketThucCua(e.tx) != "commit" {
			t.Fatalf("call %d: read tx %d, entry tx %d, end %q — one committed transaction", i+1, r.tx, e.tx, h.g.ketThucCua(e.tx))
		}
		if e.args[1] != maCanBo || e.args[4] != ActionRevealCitizenDirectoryPhone || e.args[5] != contactSampleID {
			t.Errorf("entry %v — want the STAFF CODE, the verb, the contact id", e.args[:6])
		}
		delta, raw := directoryDelta(t, e)
		if strings.ContainsAny(raw, "0123456789") {
			t.Fatalf("the delta carries digits — a number: %s", raw)
		}
		if delta["quyen"] != revealCitizenContactPhonePermission {
			t.Errorf("delta = %v, want quyen citizen.update", delta)
		}
	}
}

func TestCitizenDirectoryRevealPhoneAuditFailureDisclosesNothing(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()
	h.g.loiTheo["audit_log"] = errors.New("audit write failed")

	phone, err := h.uc.RevealPhone(h.ctx, nguoiThucHienGia(), contactSampleID)
	if err == nil || phone != "" {
		t.Fatalf("audit failed and the number was returned: %q, %v", phone, err)
	}
	if strings.Contains(err.Error(), dienThoaiGia) {
		t.Errorf("the error carries the number: %v", err)
	}
	read := h.g.tim("FAKE citizen_contact read phone")
	if read == nil || h.g.ketThucCua(read.tx) != "rollback" {
		t.Fatal("a disclosure with no trail was committed")
	}
}

func TestCitizenDirectoryRevealPhoneUnknownIsNotFoundAndUnaudited(t *testing.T) {
	h := newDirectoryHarness(t)

	_, err := h.uc.RevealPhone(h.ctx, nguoiThucHienGia(), "unknown")
	if !errors.Is(err, idstore.ErrCitizenContactNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(h.auditEntries()) != 0 {
		t.Error("nothing was disclosed, yet an entry was written")
	}
	actor := nguoiThucHienGia()
	actor.Vet.ID = "" // rule 6, invariant 8: no fallback to the internal id
	if _, err := h.uc.RevealPhone(h.ctx, actor, contactSampleID); err == nil {
		t.Fatal("a disclosure without a staff code to attribute it to was allowed")
	}
}

// --- create ------------------------------------------------------------------------------------

func TestCitizenDirectoryCreateInsertsGroupsAndOneEntryInOneTxWithoutPersonalData(t *testing.T) {
	h := newDirectoryHarness(t)

	id, err := h.uc.Create(h.ctx, nguoiThucHienGia(), sampleContactRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(h.contacts.inserted) != 1 {
		t.Fatalf("%d inserts", len(h.contacts.inserted))
	}
	c := h.contacts.inserted[0]
	if c.ID != id || c.Source != domain.CitizenContactSourceManual || c.Phone != dienThoaiGia ||
		c.ResidentialUnitID == nil || *c.ResidentialUnitID != unitSampleID || c.ImportBatchID != nil ||
		!c.CreatedAt.Equal(directoryInstant) {
		t.Errorf("inserted = %+v", c)
	}
	if len(h.contacts.setGroups) != 1 || h.contacts.setGroups[0].by != maCanBo ||
		len(h.contacts.setGroups[0].groupIDs) != 1 || h.contacts.setGroups[0].groupIDs[0] != groupSampleID {
		t.Errorf("SetGroups = %+v — want the group, by the STAFF CODE", h.contacts.setGroups)
	}
	ins := h.g.tim("FAKE citizen_contact insert")
	entries := h.auditEntries()
	if len(entries) != 1 {
		t.Fatalf("%d entries, want 1", len(entries))
	}
	e := entries[0]
	if ins.tx == 0 || ins.tx != e.tx || h.g.ketThucCua(e.tx) != "commit" {
		t.Fatalf("insert tx %d, entry tx %d, end %q", ins.tx, e.tx, h.g.ketThucCua(e.tx))
	}
	if e.args[1] != maCanBo || e.args[4] != ActionAddCitizenContact || e.args[5] != id {
		t.Errorf("entry %v", e.args[:6])
	}
	delta, raw := directoryDelta(t, e)
	assertNoPersonalData(t, "create delta", raw)
	sau, _ := delta["sau"].(map[string]any)
	if delta["nguon"] != "nhap-tay" || sau["so_nhom"] != float64(1) || sau["so_nhan"] != float64(1) || sau["is_active"] != true {
		t.Errorf("delta = %s", raw)
	}
}

func TestCitizenDirectoryCreateDuplicatePhoneIsAConflictAndWritesNothing(t *testing.T) {
	h := newDirectoryHarness(t)
	h.contacts.takenPhones[dienThoaiGia] = true

	_, err := h.uc.Create(h.ctx, nguoiThucHienGia(), sampleContactRequest())
	if !errors.Is(err, idstore.ErrCitizenPhoneTaken) {
		t.Fatalf("err = %v, want ErrCitizenPhoneTaken", err)
	}
	if len(h.contacts.inserted) != 0 || len(h.auditEntries()) != 0 {
		t.Error("a refused create inserted or audited")
	}
	chk := h.g.tim("FAKE citizen_contact phone taken")
	if chk == nil || h.g.ketThucCua(chk.tx) != "rollback" {
		t.Error("the transaction of a refused create did not roll back")
	}
}

func TestCitizenDirectoryCreateRefusesUnknownGroupAndInvalidInputBeforeAnyTx(t *testing.T) {
	h := newDirectoryHarness(t)

	req := sampleContactRequest()
	req.GroupIDs = []string{"not-a-live-group"}
	if _, err := h.uc.Create(h.ctx, nguoiThucHienGia(), req); !errors.Is(err, idstore.ErrCitizenGroupNotFound) {
		t.Errorf("unknown group: %v", err)
	}
	req = sampleContactRequest()
	req.Contact.Phone = "12345"
	if _, err := h.uc.Create(h.ctx, nguoiThucHienGia(), req); !IsCitizenDirectoryInputError(err) {
		t.Errorf("bad phone: %v", err)
	}
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""
	if _, err := h.uc.Create(h.ctx, actor, sampleContactRequest()); err == nil {
		t.Error("an actor without a staff code was allowed to write")
	}
	if h.g.soGiaoDich() != 0 {
		t.Errorf("%d transactions opened for refused input", h.g.soGiaoDich())
	}
}

// --- update ------------------------------------------------------------------------------------

func TestCitizenDirectoryUpdateAuditsChangedColumnNamesOnly(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()

	req := sampleContactRequest()
	req.Contact.Phone = phoneSecondFake
	req.Contact.Note = "ghi chú mới"
	req.Contact.IsActive = false
	req.GroupIDs = []string{groupSecondID, groupSampleID}
	if err := h.uc.Update(h.ctx, nguoiThucHienGia(), contactSampleID, req); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(h.contacts.updated) != 1 || h.contacts.updated[0].Phone != phoneSecondFake ||
		h.contacts.updated[0].Source != domain.CitizenContactSourceManual || !h.contacts.updated[0].UpdatedAt.Equal(directoryInstant) {
		t.Errorf("updated = %+v", h.contacts.updated)
	}
	if len(h.contacts.setGroups) != 1 || len(h.contacts.setGroups[0].groupIDs) != 2 {
		t.Errorf("SetGroups = %+v", h.contacts.setGroups)
	}
	upd := h.g.tim("FAKE citizen_contact update")
	entries := h.auditEntries()
	if len(entries) != 1 || upd.tx != entries[0].tx || h.g.ketThucCua(upd.tx) != "commit" {
		t.Fatalf("%d entries; update tx %d", len(entries), upd.tx)
	}
	if entries[0].args[4] != ActionEditCitizenContact {
		t.Errorf("verb %v", entries[0].args[4])
	}
	delta, raw := directoryDelta(t, entries[0])
	assertNoPersonalData(t, "update delta", raw)
	if strings.Contains(raw, "ghi chú mới") {
		t.Errorf("the delta carries the new note: %s", raw)
	}
	changed, _ := delta["truong_da_doi"].([]any)
	got := make([]string, 0, len(changed))
	for _, c := range changed {
		got = append(got, c.(string))
	}
	if strings.Join(got, ",") != "phone,note,is_active,groups" {
		t.Errorf("truong_da_doi = %v", got)
	}
	truoc, _ := delta["truoc"].(map[string]any)
	sau, _ := delta["sau"].(map[string]any)
	if truoc["is_active"] != true || sau["is_active"] != false {
		t.Errorf("is_active before/after = %v / %v", truoc, sau)
	}
}

func TestCitizenDirectoryUpdateNoChangeWritesNothing(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()

	if err := h.uc.Update(h.ctx, nguoiThucHienGia(), contactSampleID, sampleContactRequest()); err != nil {
		t.Fatal(err)
	}
	if h.g.soGiaoDich() != 0 || len(h.contacts.updated) != 0 {
		t.Error("a no-op edit opened a transaction or wrote")
	}
	if err := h.uc.Update(h.ctx, nguoiThucHienGia(), "unknown", sampleContactRequest()); !errors.Is(err, idstore.ErrCitizenContactNotFound) {
		t.Errorf("unknown id: %v", err)
	}
}

func TestCitizenDirectoryUpdateToATakenPhoneIsAConflict(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()
	h.contacts.takenPhones[phoneSecondFake] = true

	req := sampleContactRequest()
	req.Contact.Phone = phoneSecondFake
	err := h.uc.Update(h.ctx, nguoiThucHienGia(), contactSampleID, req)
	if !errors.Is(err, idstore.ErrCitizenPhoneTaken) {
		t.Fatalf("err = %v", err)
	}
	if len(h.contacts.updated) != 0 || len(h.auditEntries()) != 0 {
		t.Error("a refused edit wrote or audited")
	}
}

// --- remove ------------------------------------------------------------------------------------

func TestCitizenDirectoryRemoveRequiresAReason(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()

	err := h.uc.Remove(h.ctx, nguoiThucHienGia(), contactSampleID, "   ")
	if !errors.Is(err, domain.ErrCitizenDirectoryReasonMissing) {
		t.Fatalf("err = %v", err)
	}
	if h.g.soGiaoDich() != 0 || len(h.contacts.deleted) != 0 {
		t.Error("a removal without a reason reached the store")
	}
}

func TestCitizenDirectoryRemoveSoftDeletesByStaffCodeAndAudits(t *testing.T) {
	h := newDirectoryHarness(t)
	h.seedContact()

	if err := h.uc.Remove(h.ctx, nguoiThucHienGia(), contactSampleID, " Đã chuyển đi nơi khác "); err != nil {
		t.Fatal(err)
	}
	if len(h.contacts.deleted) != 1 || h.contacts.deleted[0].by != maCanBo || h.contacts.deleted[0].reason != "Đã chuyển đi nơi khác" {
		t.Errorf("deleted = %+v — want deleted_by = STAFF CODE, trimmed reason", h.contacts.deleted)
	}
	del := h.g.tim("FAKE citizen_contact soft delete")
	entries := h.auditEntries()
	if len(entries) != 1 || entries[0].tx != del.tx || h.g.ketThucCua(del.tx) != "commit" {
		t.Fatalf("%d entries; delete tx %d", len(entries), del.tx)
	}
	delta, raw := directoryDelta(t, entries[0])
	if entries[0].args[4] != ActionRemoveCitizenContact || delta["ly_do"] != "Đã chuyển đi nơi khác" || delta["xoa_mem"] != true {
		t.Errorf("entry %v delta %s", entries[0].args[4], raw)
	}
	assertNoPersonalData(t, "remove delta", raw)
	if err := h.uc.Remove(h.ctx, nguoiThucHienGia(), contactSampleID, "lần hai"); !errors.Is(err, idstore.ErrCitizenContactNotFound) {
		t.Errorf("second removal: %v", err)
	}
}

// --- groups ------------------------------------------------------------------------------------

func TestCitizenGroupCreateAuditsAndRefusesADuplicateName(t *testing.T) {
	h := newDirectoryHarness(t)

	id, err := h.uc.CreateGroup(h.ctx, nguoiThucHienGia(), domain.CitizenGroupInput{Name: " Hội  phụ nữ ", SortOrder: 3})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if len(h.groups.inserted) != 1 || h.groups.inserted[0].ID != id || h.groups.inserted[0].Name != "Hội phụ nữ" {
		t.Errorf("inserted = %+v", h.groups.inserted)
	}
	ins := h.g.tim("FAKE citizen_group insert")
	entries := h.auditEntries()
	if len(entries) != 1 || entries[0].tx != ins.tx || h.g.ketThucCua(ins.tx) != "commit" || entries[0].args[4] != ActionAddCitizenGroup {
		t.Fatalf("entries %d, insert tx %d, verb %v", len(entries), ins.tx, entries[0].args[4])
	}
	delta, _ := directoryDelta(t, entries[0])
	sau, _ := delta["sau"].(map[string]any)
	if sau["name"] != "Hội phụ nữ" || sau["sort_order"] != float64(3) {
		t.Errorf("delta = %v", delta)
	}

	_, err = h.uc.CreateGroup(h.ctx, nguoiThucHienGia(), domain.CitizenGroupInput{Name: "hộ kinh doanh"})
	if !errors.Is(err, idstore.ErrCitizenGroupNameTaken) {
		t.Fatalf("duplicate name: %v", err)
	}
	if len(h.groups.inserted) != 1 || len(h.auditEntries()) != 1 {
		t.Error("a refused group was inserted or audited")
	}
	if _, err := h.uc.CreateGroup(h.ctx, nguoiThucHienGia(), domain.CitizenGroupInput{}); !IsCitizenDirectoryInputError(err) {
		t.Errorf("blank name: %v", err)
	}
}

func TestCitizenGroupUpdateAuditsBeforeAndAfterAndSkipsNoOp(t *testing.T) {
	h := newDirectoryHarness(t)

	if err := h.uc.UpdateGroup(h.ctx, nguoiThucHienGia(), groupSampleID, domain.CitizenGroupInput{Name: "Tổ trưởng dân phố"}); err != nil {
		t.Fatal(err)
	}
	if h.g.soGiaoDich() != 0 {
		t.Error("a no-op edit opened a transaction")
	}
	if err := h.uc.UpdateGroup(h.ctx, nguoiThucHienGia(), groupSampleID, domain.CitizenGroupInput{Name: "Tổ trưởng", Description: "mới", SortOrder: 2}); err != nil {
		t.Fatal(err)
	}
	entries := h.auditEntries()
	if len(entries) != 1 || entries[0].args[4] != ActionEditCitizenGroup || h.g.ketThucCua(entries[0].tx) != "commit" {
		t.Fatalf("entries = %d", len(entries))
	}
	delta, _ := directoryDelta(t, entries[0])
	truoc, _ := delta["truoc"].(map[string]any)
	sau, _ := delta["sau"].(map[string]any)
	if truoc["name"] != "Tổ trưởng dân phố" || sau["name"] != "Tổ trưởng" || sau["sort_order"] != float64(2) {
		t.Errorf("delta = %v", delta)
	}
	err := h.uc.UpdateGroup(h.ctx, nguoiThucHienGia(), groupSampleID, domain.CitizenGroupInput{Name: "Hộ kinh doanh"})
	if !errors.Is(err, idstore.ErrCitizenGroupNameTaken) {
		t.Errorf("rename onto another group: %v", err)
	}
}

func TestCitizenGroupRemoveReasonIsOptionalAndListCarriesMemberCounts(t *testing.T) {
	h := newDirectoryHarness(t)
	h.contacts.memberCounts[groupSampleID] = 7

	items, err := h.uc.ListGroups(h.ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListGroups: %d, %v", len(items), err)
	}
	for _, it := range items {
		if (it.ID == groupSampleID && it.MemberCount != 7) || (it.ID == groupSecondID && it.MemberCount != 0) {
			t.Errorf("member count of %s = %d", it.ID, it.MemberCount)
		}
	}

	if err := h.uc.RemoveGroup(h.ctx, nguoiThucHienGia(), groupSecondID, ""); err != nil {
		t.Fatalf("RemoveGroup without a reason: %v", err)
	}
	if len(h.groups.deleted) != 1 || h.groups.deleted[0].by != maCanBo || h.groups.deleted[0].reason != "" {
		t.Errorf("deleted = %+v", h.groups.deleted)
	}
	entries := h.auditEntries()
	if len(entries) != 1 || entries[0].args[4] != ActionRemoveCitizenGroup || entries[0].args[5] != groupSecondID {
		t.Fatalf("entries = %d", len(entries))
	}
	if err := h.uc.RemoveGroup(h.ctx, nguoiThucHienGia(), groupSecondID, ""); !errors.Is(err, idstore.ErrCitizenGroupNotFound) {
		t.Errorf("second removal: %v", err)
	}
}
