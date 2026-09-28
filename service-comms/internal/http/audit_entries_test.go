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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/comms-audit-entries — the rule 5 invariant 7 set, plus what the handler itself owns:
// which query parameters reach core/audit, who the reader is, and how a failure is answered.
// The reader's own behaviour (filters, order, the read's entry) is core/audit/read_test.go.
//
// Harness pieces are the catalogue write suite's (checkerDanhMucGia, chuTheGhi, canBoGhi) because
// "right permission, wrong commune" needs a checker keyed by commune.

const auditEntriesPath = "/api/v1/comms-audit-entries"

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
	h       http.Handler
	fake    *auditLogFake
	checker *checkerDanhMucGia
}

func auditDeps(checker authz.Checker, fake AuditLogReader, log *slog.Logger) Deps {
	return Deps{
		Checker:              checker,
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     &ghiDanhMucGia{},
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		MapFieldSchemas:      &fakeMapFieldSchemas{},
		WriteMapFieldSchemas: &fakeMapFieldSchemas{},
		MailSettings:         &fakeMailSettings{},
		WriteMailSettings:    &fakeMailSettings{},
		AuditLog:             fake,
		Log:                  log,
	}
}

func newAuditServer(t *testing.T) *auditServer {
	t.Helper()
	fake := &auditLogFake{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, auditDeps(checker, fake, log))

	// The real edge chain in the real order, minus authentication (chuTheGhi injects the principal).
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &auditServer{h: h, fake: fake, checker: checker}
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
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func TestAuditEntries_401WithoutSession(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	doiMa(t, s.call(t, hostA, auditEntriesPath, nil), http.StatusUnauthorized)
	if s.fake.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	// The wrong keys are real and adjacent: the same Cấu hình tab's catalogue and mail-server key.
	s := newAuditServer(t)
	s.grant(xaA, "admin.lookup", "content.read")
	doiMa(t, s.call(t, hostA, auditEntriesPath, canBoGhi(xaA)), http.StatusForbidden)
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
	s.grant(xaA, "admin.audit")
	doiMa(t, s.call(t, hostB, auditEntriesPath, canBoGhi(xaB)), http.StatusForbidden)
	if s.fake.calls != 0 {
		t.Error("commune B read its audit log with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	s := newAuditServer(t)
	s.grant(xaA, "admin.audit")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	s.fake.res = page.Result[audit.EntryView]{
		Items:   []audit.EntryView{{At: at, ActorKind: "staff", ActorCode: "CB-00777", Action: "phat_hanh_thong_bao", Subject: "TB-8"}},
		HasMore: true, NextCursor: "c2",
	}
	w := s.call(t, hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=phat_hanh_thong_bao"+
		"&subject=TB-8&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE", canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)

	f := s.fake
	if f.calls != 1 {
		t.Fatalf("reader ran %d times, want once", f.calls)
	}
	if f.commune != xaA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, xaA)
	}
	if f.reader.ID != maCanBoGhi || f.reader.Kind != "staff" || f.reader.IP != "10.0.0.7" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8), kind and socket IP", f.reader, maCanBoGhi)
	}
	want := audit.Query{From: "2026-09-01T00:00:00Z", To: "2026-10-01T00:00:00Z", Actor: "CB-00777",
		Action: "phat_hanh_thong_bao", Subject: "TB-8", Limit: "5", Cursor: "c1"}
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
			w := s.call(t, hostA, auditEntriesPath, canBoGhi(xaA))
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
	p := canBoGhi(xaA)
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
	Register(http.NewServeMux(), auditDeps(&checkerDanhMucGia{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
}
