package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
)

// The directory link use cases (ADR 0102) on the fake driver of dang_nhap_giao_dich_test.go: the fakes
// below stand in for the two stores and record WHICH transaction they ran in, so "the actor check, the
// read and the entry share one committed transaction" is asserted, not assumed. The live / commune
// predicate itself is SQL, pinned against PostgreSQL in store/citizen_contact_match_pg_test.go.

const (
	contactSample   = "CT01JCONTACTSAMPLE000000000"
	staffCodeSample = "CB-00123"
)

type matchContactsFake struct {
	g *ghiChep
	// rows: stored national number → live row id, for THIS commune. Anything absent (unknown, removed,
	// another commune's) is simply not here — the store gives all three one answer.
	byPhone map[string]string
	byID    map[string]string
	err     error

	findArg   string
	readCalls int
	readTx    int
}

func (f *matchContactsFake) LiveIDByPhone(_ context.Context, national string) (string, bool, error) {
	f.findArg = national
	if f.err != nil {
		return "", false, f.err
	}
	id, ok := f.byPhone[national]
	return id, ok, nil
}

func (f *matchContactsFake) ReadPhone(_ context.Context, tx *store.ScopedTx, id string) (string, bool, error) {
	f.readCalls++
	if tx == nil {
		return "", false, errors.New("ReadPhone outside a transaction")
	}
	f.readTx = f.g.giaoDichDangMo()
	if f.err != nil {
		return "", false, f.err
	}
	p, ok := f.byID[id]
	return p, ok, nil
}

type liveStaffFake struct {
	g     *ghiChep
	live  map[string]bool
	calls int
	tx    int
}

func (f *liveStaffFake) LiveStaffCodeExists(_ context.Context, tx *store.ScopedTx, code string) (bool, error) {
	f.calls++
	if tx == nil {
		return false, errors.New("LiveStaffCodeExists outside a transaction")
	}
	f.tx = f.g.giaoDichDangMo()
	return f.live[code], nil
}

func contactMatchHarness(t *testing.T) (*CitizenContactMatch, *ghiChep, *matchContactsFake, *liveStaffFake) {
	t.Helper()
	db, g := moDB(t)
	c := &matchContactsFake{g: g,
		byPhone: map[string]string{dienThoaiGia: contactSample},
		byID:    map[string]string{contactSample: dienThoaiGia},
	}
	s := &liveStaffFake{g: g, live: map[string]bool{staffCodeSample: true}}
	return NewCitizenContactMatch(db, c, s), g, c, s
}

// --- FindByPhone ------------------------------------------------------------------------------

func TestFindByPhoneFoldsEverySpellingToTheStoredForm(t *testing.T) {
	uc, g, c, _ := contactMatchHarness(t)
	for _, raw := range []string{dienThoaiGia, "+84 900 000 000", "0900.000.000", "84900000000"} {
		id, found, err := uc.FindByPhone(ctxXa(xaThu), raw)
		if err != nil || !found || id != contactSample {
			t.Errorf("FindByPhone(%q) = %q, %v, %v; want the row", raw, id, found, err)
		}
		if c.findArg != dienThoaiGia {
			t.Errorf("store asked for %q, want the national form", c.findArg)
		}
	}
	if g.tim("audit_log") != nil || g.soGiaoDich() != 0 {
		t.Error("a lookup that discloses nothing opened a transaction or wrote an entry")
	}
}

func TestFindByPhoneAbsentIsNotFoundNotError(t *testing.T) {
	uc, _, c, _ := contactMatchHarness(t)
	// Another commune's row, a soft-deleted row and an unknown number are all absent from this commune's
	// live rows — the fake models the store's one answer.
	c.byPhone = map[string]string{}
	id, found, err := uc.FindByPhone(ctxXa(xaThu), dienThoaiGia)
	if err != nil || found || id != "" {
		t.Fatalf("got %q, %v, %v — want not found, no error", id, found, err)
	}
}

func TestFindByPhoneUnnormalisableIsNotFoundAndNeverQueried(t *testing.T) {
	uc, _, c, _ := contactMatchHarness(t)
	for _, raw := range []string{"abc", "900000000", "0900+000000", strings.Repeat("0 ", 17)} {
		c.findArg = "untouched"
		id, found, err := uc.FindByPhone(ctxXa(xaThu), raw)
		if err != nil || found || id != "" {
			t.Errorf("FindByPhone(%q) = %q, %v, %v — want not found, no error", raw, id, found, err)
		}
		if c.findArg != "untouched" {
			t.Errorf("FindByPhone(%q) reached the store with %q", raw, c.findArg)
		}
	}
}

func TestFindByPhoneEmptyIsInvalid(t *testing.T) {
	uc, _, _, _ := contactMatchHarness(t)
	for _, raw := range []string{"", "   "} {
		if _, _, err := uc.FindByPhone(ctxXa(xaThu), raw); !errors.Is(err, ErrCitizenContactMatchInvalid) {
			t.Errorf("FindByPhone(%q): err = %v, want ErrCitizenContactMatchInvalid", raw, err)
		}
	}
}

func TestFindByPhoneStoreFailureCarriesNoNumber(t *testing.T) {
	uc, _, c, _ := contactMatchHarness(t)
	c.err = errors.New("connection reset")
	_, _, err := uc.FindByPhone(ctxXa(xaThu), dienThoaiGia)
	if err == nil || errors.Is(err, ErrCitizenContactMatchInvalid) {
		t.Fatalf("err = %v, want a plain failure", err)
	}
	if strings.Contains(err.Error(), dienThoaiGia) {
		t.Errorf("the error carries the number: %v", err)
	}
}

// --- MatchPhone -------------------------------------------------------------------------------

func TestMatchPhoneCheckReadAndEntryShareOneCommittedTransaction(t *testing.T) {
	uc, g, c, s := contactMatchHarness(t)

	p, found, err := uc.MatchPhone(ctxXa(xaThu), contactSample, staffCodeSample)
	if err != nil || !found || p != dienThoaiGia {
		t.Fatalf("MatchPhone = %q, %v, %v", p, found, err)
	}
	entry := g.tim("INSERT INTO audit_log")
	if entry == nil {
		t.Fatal("a disclosure with no audit entry")
	}
	if s.tx == 0 || s.tx != c.readTx || c.readTx != entry.tx || g.ketThucCua(entry.tx) != "commit" {
		t.Fatalf("check tx %d, read tx %d, entry tx %d (%q) — must be ONE committed transaction",
			s.tx, c.readTx, entry.tx, g.ketThucCua(entry.tx))
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if entry.args[0] != string(xaThu) || entry.args[1] != staffCodeSample || entry.args[2] != "staff" ||
		entry.args[3] != "" || entry.args[4] != ActionRevealCitizenContactMatchPhone || entry.args[5] != contactSample {
		t.Errorf("entry %v — want commune, the STAFF CODE, kind staff, NO IP, the verb, the contact id", entry.args[:6])
	}
	raw := entry.args[7].([]byte)
	if strings.Contains(string(raw), dienThoaiGia) || strings.Contains(string(raw), "900000000") {
		t.Fatalf("the audit delta stores the number: %s", raw)
	}
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatal(err)
	}
	if f, _ := delta["truong_da_mo"].([]any); len(f) != 1 || f[0] != "so_dien_thoai" {
		t.Errorf("truong_da_mo = %v", delta["truong_da_mo"])
	}
	if delta["muc_dich"] != "xem_phan_anh_cua_nguoi_dan" {
		t.Errorf("muc_dich = %v", delta["muc_dich"])
	}
	if n := countStatements(g, "INSERT INTO audit_log"); n != 1 {
		t.Errorf("%d audit entries, want exactly one", n)
	}
}

func TestMatchPhoneEveryDisclosureIsAnEntry(t *testing.T) {
	uc, g, _, _ := contactMatchHarness(t)
	for i := 0; i < 2; i++ {
		if _, _, err := uc.MatchPhone(ctxXa(xaThu), contactSample, staffCodeSample); err != nil {
			t.Fatal(err)
		}
	}
	if n := countStatements(g, "INSERT INTO audit_log"); n != 2 {
		t.Errorf("%d entries for two calls — a retry is another disclosure", n)
	}
}

func TestMatchPhoneAbsentRowWritesNothing(t *testing.T) {
	uc, g, c, _ := contactMatchHarness(t)
	// Unknown, soft-deleted, another commune's: absent from this commune's live rows.
	c.byID = map[string]string{}
	p, found, err := uc.MatchPhone(ctxXa(xaThu), contactSample, staffCodeSample)
	if err != nil || found || p != "" {
		t.Fatalf("got %q, %v, %v — want not found", p, found, err)
	}
	if g.tim("audit_log") != nil {
		t.Error("nothing was disclosed, yet an entry was written")
	}
}

func TestMatchPhoneActorNotLiveIsInvalidAndReadsNothing(t *testing.T) {
	uc, g, c, s := contactMatchHarness(t)
	// A soft-deleted code, another commune's code and an invented one are all "not live here".
	s.live = map[string]bool{}
	_, _, err := uc.MatchPhone(ctxXa(xaThu), contactSample, staffCodeSample)
	if !errors.Is(err, ErrCitizenContactMatchInvalid) {
		t.Fatalf("err = %v, want ErrCitizenContactMatchInvalid", err)
	}
	if c.readCalls != 0 {
		t.Error("the number was read for an actor who is not a live staff member")
	}
	if g.tim("audit_log") != nil {
		t.Error("an entry was written for a refused actor")
	}
	if strings.Contains(err.Error(), staffCodeSample) || strings.Contains(err.Error(), contactSample) {
		t.Errorf("the refusal quotes a key: %v", err)
	}
}

func TestMatchPhoneEmptyKeysTouchNothing(t *testing.T) {
	uc, g, _, s := contactMatchHarness(t)
	for _, k := range [][2]string{{"", staffCodeSample}, {contactSample, ""}, {contactSample, "  "}} {
		if _, _, err := uc.MatchPhone(ctxXa(xaThu), k[0], k[1]); !errors.Is(err, ErrCitizenContactMatchInvalid) {
			t.Errorf("MatchPhone(%q, %q): err = %v, want ErrCitizenContactMatchInvalid", k[0], k[1], err)
		}
	}
	if n := g.soGiaoDich(); n != 0 || s.calls != 0 {
		t.Errorf("%d transactions, %d actor checks for an empty key", n, s.calls)
	}
}

func TestMatchPhoneEntryFailureDisclosesNothing(t *testing.T) {
	uc, g, _, _ := contactMatchHarness(t)
	g.loiTheo["audit_log"] = errors.New("audit write failed")

	p, found, err := uc.MatchPhone(ctxXa(xaThu), contactSample, staffCodeSample)
	if err == nil || found || p != "" {
		t.Fatalf("audit failed and the number was returned: %q, %v, %v", p, found, err)
	}
	for _, v := range []string{dienThoaiGia, contactSample, staffCodeSample} {
		if strings.Contains(err.Error(), v) {
			t.Errorf("the error carries %q: %v", v, err)
		}
	}
	entry := g.tim("INSERT INTO audit_log")
	if entry == nil || g.ketThucCua(entry.tx) != "rollback" {
		t.Fatal("a disclosure with no trail was committed")
	}
}
