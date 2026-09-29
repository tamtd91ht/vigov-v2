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

// GET /api/v1/finance-audit-entries — the rule 5 invariant 7 set, plus what the handler itself
// owns: which query parameters reach core/audit, who the reader is, and how a failure is answered.
// The reader's own behaviour (filters, order, the read's entry) is core/audit/read_test.go.
//
// ITS OWN HARNESS, for the reason budget_test.go gives: the "right permission, wrong
// commune" case needs fakeCatalogueChecker, whose grants are keyed by commune.

const auditEntriesPath = "/api/v1/finance-audit-entries"

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

type auditHarness struct {
	h       http.Handler
	log     *auditLogFake
	checker *fakeCatalogueChecker
}

func newAuditHarness(t *testing.T) *auditHarness {
	t.Helper()
	log := &auditLogFake{}
	checker := &fakeCatalogueChecker{}
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                   checker,
		CapitalPlanCategories:     sampleCategories(),
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        sampleInvestmentProjects(),
		InvestmentProjectWriter:   &fakeInvestmentProjectWriter{},
		DisbursementVoucherWriter: &fakeVoucherWriter{},
		DelayThresholds:           defaultThresholds(),
		Budget:                    sampleBudget(),
		BudgetWriter:              &fakeBudgetWriter{},
		AuditLog:                  log,
		SystemMessages:            &systemMessagesFake{},
		Now:                       func() time.Time { return atElapsed7096 },
		Log:                       slog.New(slog.NewTextHandler(io.Discard, nil)),

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	})
	var h http.Handler = mux
	h = injectPrincipal(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &auditHarness{h: h, log: log, checker: checker}
}

func (a *auditHarness) grant(tenantID tenant.ID, perm ...authz.Perm) {
	if a.checker.grants == nil {
		a.checker.grants = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if a.checker.grants[tenantID] == nil {
		a.checker.grants[tenantID] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		a.checker.grants[tenantID][p] = struct{}{}
	}
}

func (a *auditHarness) get(host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *p))
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

func TestAuditEntries_401WithoutSession(t *testing.T) {
	a := newAuditHarness(t)
	a.grant(tenantA, "admin.audit")
	wantStatus(t, a.get(hostA, auditEntriesPath, nil), http.StatusUnauthorized)
	if a.log.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	a := newAuditHarness(t)
	a.grant(tenantA, "budget.read", "admin.lookup")
	wantStatus(t, a.get(hostA, auditEntriesPath, writerPrincipal(tenantA)), http.StatusForbidden)
	if a.log.calls != 0 {
		t.Error("audit log read without admin.audit")
	}
	if got := a.checker.lastAskedKey(); got != "admin.audit" {
		t.Errorf("checker asked %q, want admin.audit", got)
	}
}

func TestAuditEntries_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	a := newAuditHarness(t)
	a.grant(tenantA, "admin.audit")
	wantStatus(t, a.get(hostB, auditEntriesPath, writerPrincipal(tenantB)), http.StatusForbidden)
	if a.log.calls != 0 {
		t.Error("commune B read its audit log with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	a := newAuditHarness(t)
	a.grant(tenantA, "admin.audit")
	a.log.res = page.Result[audit.EntryView]{
		Items: []audit.EntryView{{At: time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC),
			ActorKind: "staff", ActorCode: "CB-00777", Action: "them_du_an", Subject: "DA-1"}},
	}
	w := a.get(hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=them_du_an"+
		"&subject=DA-1&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE", writerPrincipal(tenantA))
	wantStatus(t, w, http.StatusOK)

	f := a.log
	if f.commune != tenantA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, tenantA)
	}
	if f.reader.ID != writerStaffCode || f.reader.ID == writerInternalID || f.reader.IP == "" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8) and socket IP", f.reader, writerStaffCode)
	}
	want := audit.Query{From: "2026-09-01T00:00:00Z", To: "2026-10-01T00:00:00Z", Actor: "CB-00777",
		Action: "them_du_an", Subject: "DA-1", Limit: "5", Cursor: "c1"}
	if f.query != want {
		t.Errorf("query = %+v, want %+v", f.query, want)
	}
	var body page.Result[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0]["actor_code"] != "CB-00777" {
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
			a := newAuditHarness(t)
			a.grant(tenantA, "admin.audit")
			a.log.err = c.err
			w := a.get(hostA, auditEntriesPath, writerPrincipal(tenantA))
			wantStatus(t, w, c.want)
			if e := errorBody(t, w); e.Message == "" || e.Message == c.err.Error() {
				t.Errorf("error body = %+v — a sentence for a person, never the raw error", e)
			}
		})
	}
}

func TestAuditEntries_NoBusinessCodeIs500AndReadsNothing(t *testing.T) {
	a := newAuditHarness(t)
	a.grant(tenantA, "admin.audit")
	wantStatus(t, a.get(hostA, auditEntriesPath, staffOf(tenantA)), http.StatusInternalServerError) // staffOf has no Ma
	if a.log.calls != 0 {
		t.Error("read ran for a reader with no business code — its entry could not name who read")
	}
}
