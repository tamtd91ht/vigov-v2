package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The harness for the finance service's HTTP routes.
//
// NOTHING HERE IS A PostgreSQL OR A REDIS, and that is the point. The properties under test are
// ordering and isolation — the commune resolved before the guard, the guard before the store, one
// commune's catalogue never reaching another's caller — and a test that needs infrastructure is a
// test that stops being run. There is no PostgreSQL reachable from this repository's build
// environment, so "needs a database" means "never runs".
//
// WHAT THIS HARNESS STANDS IN FOR, said plainly because it is the one place a reader could be
// misled: this service has NO staff-authentication middleware of its own yet. The identity service
// has one (XacThuc: cookie -> session registry -> account -> Principal) and finance may not import
// it — reaching into another service's internal/ breaks the boundary at compile time (rule 2,
// forbidden #1). So fakeAuth below puts the Principal into the context directly, exactly as that
// middleware will when this service gets one.
//
// The GUARD ITSELF IS REAL: authz.AnyAuthenticated is the shipped code, and it performs the commune
// comparison on its own (authz.xacNhanXa). The wrong-commune case below therefore exercises real
// enforcement, not a stand-in for it.

// --- fixtures ---------------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// The internal staff id, the value identity's Checker matches on (`nd.id = $2`). Never a
	// business code: a swap there makes every guarded route answer 403 with nothing to show why.
	internalStaffID = "nd-01JINTERNALIDCUACANBO"
)

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// staffOf builds the principal core/staffauth.Middleware builds for one commune — the same shape,
// injected directly so this package's properties stay testable without a fake identity service.
// The real middleware is driven end to end in cmd/server/main_test.go.
func staffOf(tenantID tenant.ID) *authz.Principal {
	return &authz.Principal{ID: internalStaffID, Kind: "staff", TenantID: tenantID}
}

// --- fakes ------------------------------------------------------------------------------------

type fakeDirectory map[string]tenant.Tenant

func (m fakeDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// sampleDirectory is the platform registry for the two test communes. A NEW MAP PER CALL, so a test that
// rewrites one copy cannot change another's.
func sampleDirectory() fakeDirectory {
	return fakeDirectory{
		hostA: {ID: tenantA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: tenantB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// fakeCategoryCatalogue is the capital plan category catalogue, KEYED BY COMMUNE, reading the commune from
// the context exactly as *store.Scoped does. Keyed any other way, the isolation case in
// capital_plan_category_test.go would pass while proving nothing.
//
// `calls` counts the reads. The count is what proves the commune check happens BEFORE any store
// access, rather than merely producing the right answer afterwards.
type fakeCategoryCatalogue struct {
	byTenant map[tenant.ID][]domain.CapitalPlanCategory
	err      error
	calls    int
}

func (h *fakeCategoryCatalogue) ListCategories(ctx context.Context) ([]domain.CapitalPlanCategory, error) {
	h.calls++
	if h.err != nil {
		return nil, h.err
	}
	return h.byTenant[tenant.MustFrom(ctx)], nil
}

// sampleCategories gives commune A three categories and commune B one with a DIFFERENT name. Two
// communes whose catalogues were named the same could not show a leak.
//
// THREE PROPERTIES ARE BUILT INTO THIS FIXTURE, each for a specific failure:
//
//   - THE ORDER IS THE STORE'S (ORDER BY thu_tu, ma) AND IS DELIBERATELY NOT ALPHABETICAL — neither
//     by code nor by label. Any re-sort in the handler turns the order test red.
//   - hm-002 CARRIES DIFFERENT VALUES IN THE TWO BOOLEANS (is_default false, active true). The two
//     columns are adjacent and a swap between them is invisible on any row where they agree; this
//     row is the one that makes a swap visible.
//   - hm-003 IS OUT OF USE BUT NOT DELETED. It must still be returned: the catalogue screen shows
//     it with a "Đã tắt" chip, and a plan line from an earlier budget year still holds its code.
//
// Real category codes, written the way ADR 0011 requires: Vietnamese without diacritics. No row is
// seeded anywhere but here — the table itself ships empty for every commune on purpose.
func sampleCategories() *fakeCategoryCatalogue {
	return &fakeCategoryCatalogue{byTenant: map[tenant.ID][]domain.CapitalPlanCategory{
		tenantA: {
			// A FOURTH PROPERTY, ADDED WITH THE WRITE ROUTES: the three tiers of ADR 0024 are all
			// represented, because the tier is what the configuration screen reads to decide which
			// buttons it may draw. A fixture where every row was `don-vi` could not tell a mapper
			// that always answers tier 1 from one that reads the columns.
			{ID: "hm-001", Code: "xay-dung-moi", Label: "Xây dựng mới", IsDefault: true, IsActive: true,
				Source: domain.SourceCommune},
			{ID: "hm-002", Code: "cai-tao-nang-cap", Label: "Cải tạo, nâng cấp", IsActive: true,
				SortOrder: 2, Source: domain.SourceSystem},
			{ID: "hm-003", Code: "tra-no", Label: "Trả nợ",
				SortOrder: 3, Source: domain.SourceSystem, BranchedInSource: true},
		},
		tenantB: {
			{ID: "hm-b-001", Code: "giai-phong-mat-bang", Label: "Giải phóng mặt bằng XÃ B",
				IsActive: true, Source: domain.SourceCommune},
		},
	}}
}

// fakeChecker grants nothing to anybody, in any commune.
//
// THAT IS THE HONEST FIXTURE FOR THIS SERVICE TODAY: no route here declares RequirePermission, so
// a checker that granted something would be describing an enforcement path that does not exist. It
// is wired anyway, so that the AnyAuthenticated route is proved to answer 200 for an account
// holding NO permission at all — which is the whole reason that declaration was chosen.
// fakeChecker grants exactly the permissions it was built with, and nothing else, in any commune.
//
// IT USED TO GRANT NOTHING AT ALL, and the comment here said why: no route declared
// RequirePermission, so a checker that granted something would have described an enforcement path
// that did not exist. The disbursement routes declare `budget.read`, so the fixture now has to be
// able to say "this account holds it" and "this one does not" — that difference is what the 403
// case of rule 5, invariant 7 is made of.
//
// THE COMMUNE IS NOT COMPARED HERE, deliberately: authz.RequirePermission does that comparison
// itself, before it ever calls Allows. A checker that also compared would hide a guard that had
// stopped comparing.
type fakeChecker struct{ allow map[authz.Perm]bool }

func (c fakeChecker) Allows(_ context.Context, _ authz.Principal, perm authz.Perm) bool {
	return c.allow[perm]
}

// noPermissions is an account that can sign in and holds nothing. It is the fixture the
// AnyAuthenticated route needs — that declaration exists precisely so such an account gets 200.
func noPermissions() fakeChecker { return fakeChecker{} }

// withPermission grants one real key. `budget.read` is one of the three `budget.*` rows loaded by
// service-identity/migrations/0001_init.sql; a fixture granting an invented key would prove a
// route reachable that no administrator could ever grant access to.
func withPermission(perm authz.Perm) fakeChecker {
	return fakeChecker{allow: map[authz.Perm]bool{perm: true}}
}

// --- harness ----------------------------------------------------------------------------------

type testServer struct {
	d                  Deps
	mux                *http.ServeMux
	directory          fakeDirectory
	categories         *fakeCategoryCatalogue
	investmentProjects *fakeInvestmentProjectReader
}

// newTestServer mounts the REAL routes through Register.
//
// Building from Register rather than from a stand-in route is what makes every case below prove
// something about what ships: a route mounted without a permission declaration, or mounted at a
// different path, fails here rather than in production.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	return newTestServerWith(t, noPermissions())
}

// newTestServerWith mounts the REAL routes with a chosen permission set.
//
// The clock is FIXED at atElapsed7096 — the instant §3 uses in its worked examples, 70,96% of the
// 2026 budget year. A test that let the routes read the wall clock would assert a different delay
// score every day it ran, so it would end up asserting nothing.
func newTestServerWith(t *testing.T, c fakeChecker) *testServer {
	t.Helper()

	categories := sampleCategories()
	investmentProjects := sampleInvestmentProjects()
	d := Deps{
		Checker:               c,
		CapitalPlanCategories: categories,
		// The write use case, so Register accepts the Deps. NOTHING IN THIS FILE CALLS IT: the
		// write routes have their own four-case suite in capital_plan_category_write_test.go, with a
		// fake that records the commune and the acting person. Register refuses a nil dependency at
		// construction, so it has to be present here — and a fake that is never invoked cannot
		// answer anything wrongly.
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        investmentProjects,
		// The voucher write use case and the commune's threshold, so Register accepts the Deps.
		// NOTHING IN THIS FILE CALLS THE FIRST: the write routes have their own four-case suite in
		// disbursement_voucher_test.go, with a fake that records the commune and the acting person.
		// The threshold IS read here, by the two project routes. defaultThresholds() leaves EVERY commune
		// on the software's 10 points — the state of every commune today — so the delay assertions in
		// investment_project_test.go stay about the projects rather than about a threshold fixture. The case where
		// a commune has chosen its own figure lives beside the fake, in disbursement_voucher_test.go.
		// The project write use case, so Register accepts the Deps. NOTHING IN THIS FILE CALLS IT:
		// the three write routes have their own four-case suite in investment_project_write_test.go, with a fake
		// that records the commune and the acting person.
		InvestmentProjectWriter:   &fakeInvestmentProjectWriter{},
		DisbursementVoucherWriter: &fakeVoucherWriter{},
		DelayThresholds:           defaultThresholds(),
		// The budget board, so Register accepts the Deps. NOTHING IN THIS FILE CALLS EITHER: the
		// eight budget routes have their own suite in budget_test.go, with fakes that
		// record the commune and the acting person. Register refuses a nil dependency at
		// construction, so both have to be present here — and a fake that is never invoked cannot
		// answer anything wrongly.
		Budget:         &fakeBudgetReader{},
		BudgetWriter:   &fakeBudgetWriter{},
		AuditLog:       &auditLogFake{},
		SystemMessages: &systemMessagesFake{},
		Now:            func() time.Time { return atElapsed7096 },
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	}

	mux := http.NewServeMux()
	Register(mux, d)

	return &testServer{d: d, mux: mux, directory: sampleDirectory(), categories: categories, investmentProjects: investmentProjects}
}

// fakeAuth stands in for the staff-authentication middleware this service does not have yet.
// See the note at the top of this file.
//
// A nil principal means "no session" — the request reaches the route with nothing in the context,
// which is exactly the state a request with no cookie arrives in.
func fakeAuth(p *authz.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p == nil {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(authz.Into(r.Context(), *p)))
		})
	}
}

// call runs one request through the real edge chain, in the real order:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	TenantMiddleware    resolves Host -> commune; an unknown Host is 404, never a default
//	<authentication>    builds the Principal
//	mux                 the route, with its own guard
//
// The chain is rebuilt per call rather than once, so the principal can differ between calls
// without any test assembling the order by hand — the ORDER is the property most of these tests
// are about, and a copy assembled per test is a copy that drifts out of it.
func (m *testServer) call(t *testing.T, method, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()

	var h http.Handler = m.mux
	h = fakeAuth(p)(h)
	h = httpx.TenantMiddleware(m.directory)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	r := httptest.NewRequest(method, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- wiring -------------------------------------------------------------------------------------

func TestRegisterWithoutStorePanicsAtConstruction(t *testing.T) {
	// AT CONSTRUCTION, NOT AT REQUEST TIME. A route mounted without its store would accept requests
	// it cannot honour, and the first person to find out would be a member of staff in front of a
	// government screen. A process that refuses to start is a deployment that fails visibly.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register chấp nhận Deps thiếu kho — tuyến sẽ panic khi có người gọi")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}

func TestHostOfNoCommuneIs404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve the commune -> 404, never a default commune. 404 also
	// reveals nothing about which communes exist on the platform.
	m := newTestServer(t)

	w := m.call(t, "GET", "khong-ai-biet.example.gov.vn", categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusNotFound)
	if m.categories.calls != 0 {
		t.Error("tên miền không thuộc xã nào mà vẫn đọc danh mục")
	}
}
