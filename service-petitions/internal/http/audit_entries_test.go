package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/petitions-audit-entries — the rule 5 invariant 7 set, the restricted-field decision
// (ADR 0054 §4: SeeHidden comes from the Checker's `feedback.restricted`, never from the client), and
// what the handler itself owns: which query parameters reach core/audit, who the reader is, and how a
// failure is answered. The reader's own behaviour is core/audit/read_test.go; that the wired subquery
// really reaches the SQL is internal/store/audit_hidden_subjects_test.go.
//
// Harness: dungMayChu's real routes, with the Checker swapped for the commune-keyed checkerDanhMucGia
// (loai_nhiem_vu_ghi_test.go) because "right permission, wrong commune" needs grants per commune.

const auditEntriesPath = "/api/v1/petitions-audit-entries"

// auditLogFake records the commune it was called in, read from the context the way *store.Scoped
// reads it — a fake that ignored the commune would let a wrong-commune test pass proving nothing.
type auditLogFake struct {
	calls   int
	commune tenant.ID
	reader  audit.Actor
	query   audit.Query
	res     page.Result[audit.EntryView]
	err     error
}

func (f *auditLogFake) Read(ctx context.Context, reader audit.Actor, q audit.Query) (page.Result[audit.EntryView], error) {
	f.calls++
	f.commune, f.reader, f.query = tenant.MustFrom(ctx), reader, q
	if f.err != nil {
		return page.NewResult[audit.EntryView](), f.err
	}
	return f.res, nil
}

type auditServer struct {
	m       *mayChu
	fake    *auditLogFake
	checker *checkerDanhMucGia
}

func newAuditServer(t *testing.T) *auditServer {
	t.Helper()
	fake := &auditLogFake{}
	checker := &checkerDanhMucGia{}
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checker
		d.AuditLog = fake
	})
	return &auditServer{m: m, fake: fake, checker: checker}
}

func (s *auditServer) grant(commune tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[commune] == nil {
		s.checker.co[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[commune][p] = struct{}{}
	}
}

func (s *auditServer) call(t *testing.T, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	return s.m.goi(t, http.MethodGet, host, path, p)
}

func TestAuditEntries_401WithoutSession(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit", "feedback.restricted")
	doiMa(t, s.call(t, hostA, auditEntriesPath, nil), http.StatusUnauthorized)
	if s.fake.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	// The wrong keys are real and adjacent: this service's own read key, and the restricted-field key
	// itself — `feedback.restricted` widens what admin.audit shows, it does not stand in for it.
	s := newAuditServer(t)
	s.grant(xaA, "feedback.read", "feedback.restricted")
	doiMa(t, s.call(t, hostA, auditEntriesPath, canBoCuaXa(xaA)), http.StatusForbidden)
	if s.fake.calls != 0 {
		t.Error("audit log read without admin.audit")
	}
	// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
	if got := s.checker.hoiKhoaCuoi(); got != "admin.audit" {
		t.Errorf("checker asked %q, want admin.audit", got)
	}
}

func TestAuditEntries_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit", "feedback.restricted")
	doiMa(t, s.call(t, hostB, auditEntriesPath, canBoCuaXa(xaB)), http.StatusForbidden)
	if s.fake.calls != 0 {
		t.Error("commune B read its audit log with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	s.fake.res = page.Result[audit.EntryView]{
		Items:   []audit.EntryView{{At: at, ActorKind: "staff", ActorCode: "CB-00777", Action: "phan_loai_phieu", Subject: "PA-AAAA-0000-0001"}},
		HasMore: true, NextCursor: "c2",
	}
	// `see_hidden` and `tenant_id` are CLIENT ATTEMPTS: neither may reach the reader.
	w := s.call(t, hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=phan_loai_phieu"+
		"&subject=PA-AAAA-0000-0001&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE&see_hidden=true", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	f := s.fake
	if f.calls != 1 {
		t.Fatalf("reader ran %d times, want once", f.calls)
	}
	if f.commune != xaA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, xaA)
	}
	if f.reader.ID != maCanBo || f.reader.Kind != "staff" || f.reader.IP != "10.0.0.7" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8), kind and socket IP", f.reader, maCanBo)
	}
	want := audit.Query{From: "2026-09-01T00:00:00Z", To: "2026-10-01T00:00:00Z", Actor: "CB-00777",
		Action: "phan_loai_phieu", Subject: "PA-AAAA-0000-0001", Limit: "5", Cursor: "c1", SeeHidden: false}
	if f.query != want {
		t.Errorf("query = %+v, want %+v", f.query, want)
	}

	var body struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
		HasMore    bool             `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0]["actor_code"] != "CB-00777" || !body.HasMore || body.NextCursor != "c2" {
		t.Errorf("body = %s", w.Body.String())
	}
}

// THE RESTRICTED FIELD (ADR 0054 §4, ADR 0030). Without `feedback.restricted` the reader is told to
// withhold `can-bo` petitions' entries — even when the client asks otherwise; with it, they are shown.
func TestAuditEntries_RestrictedFieldHiddenWithoutFeedbackRestricted(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	doiMa(t, s.call(t, hostA, auditEntriesPath+"?see_hidden=true&SeeHidden=true", canBoCuaXa(xaA)), http.StatusOK)
	if s.fake.calls != 1 || s.fake.query.SeeHidden {
		t.Errorf("SeeHidden = %v without feedback.restricted — a `can-bo` petition's trail would show", s.fake.query.SeeHidden)
	}
}

func TestAuditEntries_RestrictedFieldShownWithFeedbackRestricted(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit", "feedback.restricted")
	doiMa(t, s.call(t, hostA, auditEntriesPath, canBoCuaXa(xaA)), http.StatusOK)
	if s.fake.calls != 1 || !s.fake.query.SeeHidden {
		t.Errorf("SeeHidden = %v with feedback.restricted", s.fake.query.SeeHidden)
	}
	// The key asked is the LITERAL ADR 0030 names — a fake checker grants any string.
	if got := s.checker.hoiKhoaCuoi(); got != "feedback.restricted" {
		t.Errorf("checker last asked %q, want feedback.restricted", got)
	}
}

func TestAuditEntries_RestrictedGrantInAnotherCommuneDoesNotOpen(t *testing.T) {
	// admin.audit in A, feedback.restricted only in B: the widening key must be asked of THIS commune.
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	s.grant(xaB, "feedback.restricted")
	doiMa(t, s.call(t, hostA, auditEntriesPath, canBoCuaXa(xaA)), http.StatusOK)
	if s.fake.query.SeeHidden {
		t.Error("feedback.restricted granted in another commune opened this commune's restricted trail")
	}
}

func TestAuditEntries_FailuresMapped(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want int
	}{
		"bad range":  {audit.ErrRange, http.StatusBadRequest},
		"bad cursor": {page.ErrCursor, http.StatusBadRequest},
		"store down": {errors.New("connection reset"), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			s := newAuditServer(t)
			s.grant(xaA, "admin.audit")
			s.fake.err = c.err
			w := s.call(t, hostA, auditEntriesPath, canBoCuaXa(xaA))
			doiMa(t, w, c.want)
			if e := loiTra(t, w); e.Message == "" || e.Message == c.err.Error() {
				t.Errorf("error body = %+v — a sentence for a person, never the raw error", e)
			}
		})
	}
}

func TestAuditEntries_NoBusinessCodeIs500AndReadsNothing(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	p := canBoCuaXa(xaA)
	p.Ma = ""
	doiMa(t, s.call(t, hostA, auditEntriesPath, p), http.StatusInternalServerError)
	if s.fake.calls != 0 {
		t.Error("read ran for a reader with no business code — its entry could not name who read")
	}
}

func TestAuditEntries_RegisterRefusesMissingReader(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register did not panic without an audit-log reader")
		}
	}()
	d := depsDay()
	d.AuditLog = nil
	d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	Register(http.NewServeMux(), d)
}
