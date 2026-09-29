package main

// What this test defends: THE WIRING, not the middleware.
//
// core/staffauth proves the four behaviours of the authentication middleware and core/httpx proves
// what TenantMiddleware does. Neither of them can see whether THIS binary installs them. A
// middleware deleted from buildEdge leaves a service that starts, serves and answers — and answers
// 401 to every member of staff holding a perfectly valid session, which is the exact state this
// service shipped in until today. Nothing else in this repository turns red for that.
//
// So this test drives the chain buildEdge actually builds, with the REAL route mounted behind it.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	svchttp "github.com/vihat/vigov/service-finance/internal/http"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	staffInternalID  = "nd-01JINTERNALIDCUACANBO"
	fakeSessionToken = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	categoryRoute = "/api/v1/capital-plan-categories"

	labelTenantA = "Xây dựng mới xã A"
	labelTenantB = "Xây dựng mới xã B"
)

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type fakeDirectory map[string]tenant.Tenant

func (m fakeDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// fakeStore is the catalogue, KEYED BY COMMUNE and COUNTING ITS READS.
//
// Both halves are assertions. Keyed by commune, because a store keyed by nothing would let the
// wrong-commune test pass while proving nothing. Counting, because "the store was never touched" is
// the only way to show a refused request stopped at the edge rather than at the handler.
type fakeStore struct {
	byTenant map[tenant.ID][]domain.CapitalPlanCategory
	reads    int
}

func (k *fakeStore) ListCategories(ctx context.Context) ([]domain.CapitalPlanCategory, error) {
	k.reads++
	return k.byTenant[tenant.MustFrom(ctx)], nil
}

type fakeResolver struct {
	calls   int
	returns staffauth.StaffPrincipal
	ok      bool
	err     error

	// The commune carried on the outgoing call — see ResolveStaff below.
	sentTenant tenant.ID
	hasTenant  bool
}

func (p *fakeResolver) ResolveStaff(ctx context.Context, _, _ string) (staffauth.StaffPrincipal, bool, error) {
	p.calls++
	// THE COMMUNE THAT LEAVES THIS PROCESS IS RECORDED, AND IT IS THE ONLY THING THIS SERVICE CAN
	// ASSERT ABOUT THE CROSS-COMMUNE REFUSAL.
	//
	// authz.AnyAuthenticated's own commune check cannot fail here — core/staffauth stamps the Host
	// commune onto the principal, so it compares a value with itself. The comparison that decides
	// lives in identity (service-identity/internal/grpc/server.go:257), where the commune sent in
	// `x-tenant-id` meets the commune INSIDE the credential. This service cannot reach that line;
	// what it CAN prove is that it sent the right commune to be compared against. Drop that and
	// the far side compares the wrong pair, and nothing here would have noticed.
	p.sentTenant, p.hasTenant = tenant.From(ctx)
	return p.returns, p.ok, p.err
}

type testServer struct {
	h    http.Handler
	repo *fakeStore
	pg   *fakeResolver
}

func newTestServer(t *testing.T, pg *fakeResolver) *testServer {
	t.Helper()

	repo := &fakeStore{byTenant: map[tenant.ID][]domain.CapitalPlanCategory{
		tenantA: {{ID: "hm-001", Code: "xay-dung-moi", Label: labelTenantA, IsActive: true, IsDefault: true}},
		tenantB: {{ID: "hm-b-001", Code: "xay-dung-moi", Label: labelTenantB, IsActive: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY main() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:               staffauth.Checker{},
		CapitalPlanCategories: repo,
		// The disbursement routes are wired so that Register mounts THE WHOLE surface this service
		// ships. This file is about the edge chain — Host -> commune -> principal — and it asserts
		// on the capital plan catalogue route; a Deps missing a dependency would make Register
		// refuse to start, which is exactly what it is supposed to do.
		// Built on a nil *store.DB. NOTHING IN THIS FILE CALLS IT: this test is about the edge
		// chain — Host -> commune -> principal — and it asserts on the catalogue READ route.
		// Register refuses a nil dependency at construction, so it has to be present.
		CapitalPlanCategoryWriter: app.NewCapitalPlanCategoryService(nil, nil),
		InvestmentProjects:        emptyInvestmentProjectStore{},
		// Same reasoning as CapitalPlanCategoryWriter above: built on a nil *store.DB, never called from this file,
		// present because Register refuses a nil dependency at construction.
		InvestmentProjectWriter:   app.NewInvestmentProjectService(nil, nil),
		DisbursementVoucherWriter: app.NewDisbursementVoucherService(nil, nil),
		DelayThresholds:           emptyThresholdStore{},
		// The budget board, same reasoning again: built on a nil *store.DB and never called from this
		// file, present because Register refuses a nil dependency at construction.
		Budget:         emptyBudgetStore{},
		BudgetWriter:   app.NewBudgetService(nil, nil),
		AuditLog:       audit.NewLog(pkgstore.New(nil)),
		SystemMessages: app.NewSystemMessages(nil, emptyOverrideStore{}),
		Log:            log,

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: app.NewCapitalPlanCategoryImporter(nil, nil),
	})

	directory := fakeDirectory{
		hostA: {ID: tenantA, Host: hostA, Active: true},
		hostB: {ID: tenantB, Host: hostB, Active: true},
	}
	return &testServer{h: buildEdge(mux, directory, pg, nil, log), repo: repo, pg: pg}
}

func (m *testServer) call(t *testing.T, host, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if token != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: token})
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

// staffOfTenantA is a member of staff of commune A holding `budget.read`.
//
// THE KEY IS A ROW OF `quyen` AND IT IS THE ONE THIS SERVICE'S ROUTES DECLARE. Until today this
// read `finance.read` — a string that appears in NO migration and in NO route in this repository.
// It made every case below green while proving nothing about permissions: the catalogue route is
// AnyAuthenticated, which never consults the key set at all, so any string whatsoever passed. A
// key absent from `quyen` is a key no administrator can grant on the Phân quyền screen, so a
// route guarded by it could never be reached by anybody (rule 5, invariant 3b).
//
// `budget.read` is loaded by service-identity/migrations/0001_init.sql:283 and declared by
// service-finance/internal/http/routes.go:211 and :229. TestBudgetReadPermissionIsReallyChecked below
// is what keeps this line honest: without it, this key would still be decoration.
func staffOfTenantA() *fakeResolver {
	return &fakeResolver{ok: true, returns: staffauth.StaffPrincipal{
		StaffID:        staffInternalID,
		PermissionKeys: []authz.Perm{"budget.read"},
	}}
}

// staffWithoutPermissions is a member of staff of commune A who is signed in and holds NOTHING.
//
// This is the account the specification's smallest role produces, and it is the one a 403 has to
// be provable against.
func staffWithoutPermissions() *fakeResolver {
	return &fakeResolver{ok: true, returns: staffauth.StaffPrincipal{StaffID: staffInternalID}}
}

func TestStaffOfTenantA_OnHostA_IsServed(t *testing.T) {
	// FIRST TIME A ROUTE IN THIS SERVICE SERVES A REQUEST. Until this chain existed, the route
	// answered 401 to everybody — including a member of staff holding a perfectly valid session —
	// because nothing in the binary built an authz.Principal.
	m := newTestServer(t, staffOfTenantA())

	w := m.call(t, hostA, categoryRoute, fakeSessionToken)
	wantStatus(t, w, http.StatusOK)

	// The commune's OWN catalogue, read with the commune from the context — never commune B's.
	if !strings.Contains(w.Body.String(), labelTenantA) || strings.Contains(w.Body.String(), labelTenantB) {
		t.Errorf("thân trả về không phải danh mục của xã A: %s", w.Body.String())
	}
	if m.repo.reads != 1 || m.pg.calls != 1 {
		t.Errorf("đọc kho %d lần, gọi định danh %d lần — muốn 1 và 1", m.repo.reads, m.pg.calls)
	}
}

func TestSameCredentialsOnHostB_NoPrincipal_AndStoreUntouched(t *testing.T) {
	// The same credential at commune B's host. The RPC makes the comparison — the commune in
	// "x-tenant-id" against the commune inside the credential — and answers OK with NO PRINCIPAL,
	// which is what this fake reproduces. The route's own guard then refuses.
	//
	// THE STORE MUST NOT BE TOUCHED: a query is where a commune boundary is actually crossed.
	m := newTestServer(t, &fakeResolver{ok: false})

	wantStatus(t, m.call(t, hostB, categoryRoute, fakeSessionToken), http.StatusUnauthorized)
	if m.repo.reads != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.repo.reads)
	}
}

func TestNoCookieMeansNoIdentityCall(t *testing.T) {
	// The count is the assertion. A chain that called anyway would answer this request correctly
	// and pay a LAN round trip on every anonymous hit, forever, with nothing reporting it.
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, hostA, categoryRoute, ""), http.StatusUnauthorized)
	if m.pg.calls != 0 || m.repo.reads != 0 {
		t.Errorf("không có cookie mà vẫn gọi định danh %d lần, đọc kho %d lần", m.pg.calls, m.repo.reads)
	}
}

func TestIdentityDownIs503Not401(t *testing.T) {
	// If identity is down and this chain answered "no principal", every member of staff would be
	// told to sign in again — through the service that is down.
	m := newTestServer(t, &fakeResolver{err: errors.New("rpc error: code = Unavailable desc = refused")})

	w := m.call(t, hostA, categoryRoute, fakeSessionToken)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("trả 401 khi dịch vụ định danh chết")
	}
	wantStatus(t, w, http.StatusServiceUnavailable)
	if m.repo.reads != 0 {
		t.Errorf("kho bị đọc %d lần trong lúc không xác thực được", m.repo.reads)
	}
}

func TestUnresolvableHostIs404AndCallsNobody(t *testing.T) {
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, "khong-co-xa.example.gov.vn", categoryRoute, fakeSessionToken), http.StatusNotFound)
	if m.pg.calls != 0 || m.repo.reads != 0 {
		t.Errorf("Host không phân giải được vẫn gọi định danh %d lần và đọc kho %d lần", m.pg.calls, m.repo.reads)
	}
}

func TestHealthzSitsOutsideTenantChain(t *testing.T) {
	// It answers whether this process is alive, which is true or false regardless of which commune
	// is asking. Behind Host resolution it would fail whenever the platform service does, and an
	// orchestrator would restart a healthy process during somebody else's outage.
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, "khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
	if m.pg.calls != 0 {
		t.Error("/healthz gọi tới dịch vụ định danh")
	}
}

// LỜI GỌI ĐI RA PHẢI MANG XÃ SUY TỪ `Host` — phép khẳng định duy nhất service này làm được về
// việc từ chối chéo xã.
//
// Phép so của authz.AnyAuthenticated ở đây KHÔNG THỂ SAI: core/staffauth đóng dấu xã của `Host`
// lên chính principal nó dựng (staffauth.go:157 và :241), nên xacNhanXa so một giá trị với chính
// nó. Phép so quyết định nằm trong identity — service-identity/internal/grpc/server.go:257 — nơi
// xã trong `x-tenant-id` gặp xã NẰM TRONG chứng thực. Service này với không tới dòng đó; thứ nó
// chứng minh được là nó đã GỬI ĐÚNG XÃ để bên kia đem ra so.
//
// Bỏ mất phần gửi ấy thì bên kia so nhầm cặp, và trước ca test này thì không gì ở đây thấy được.
func TestResolveCallCarriesHostsTenant(t *testing.T) {
	m := newTestServer(t, staffOfTenantA())

	// Xã A ở host A: lời gọi phải mang xã A.
	wantStatus(t, m.call(t, hostA, categoryRoute, fakeSessionToken), http.StatusOK)
	if !m.pg.hasTenant {
		t.Fatal("lời gọi phân giải KHÔNG mang xã nào — bên kia sẽ so với một giá trị rỗng")
	}
	if m.pg.sentTenant != tenantA {
		t.Errorf("xã đã gửi = %q, muốn %q (xã suy từ Host)", m.pg.sentTenant, tenantA)
	}

	// Cùng phiếu ấy ở host B: xã gửi đi phải ĐỔI THEO HOST, không theo phiếu. Đây là nửa bắt được
	// một bản cài đặt lấy xã từ phản hồi RPC thay vì từ Host.
	m2 := newTestServer(t, staffOfTenantA())
	m2.call(t, hostB, categoryRoute, fakeSessionToken)
	if m2.pg.sentTenant != tenantB {
		t.Errorf("ở host B, xã đã gửi = %q, muốn %q", m2.pg.sentTenant, tenantB)
	}
}

// --- the permission declaration, exercised through the chain this binary builds ---------------

// TUYẾN DUY NHẤT CỦA DỊCH VỤ NÀY CÓ KHAI `RequirePermission`, ĐI QUA ĐÚNG CHUỖI buildEdge DỰNG.
//
// Vì sao ca này phải có: mọi ca khác trong tệp bắn vào tuyến danh mục, và tuyến ấy khai
// AnyAuthenticated — guard đó KHÔNG BAO GIỜ đọc bộ khoá (core/authz/authz.go:187-208). Nên trước
// ca này, `PermissionKeys` trong tệp này là trang trí: đổi nó thành chuỗi rác, mọi ca vẫn xanh.
// Đó chính là cách `finance.read` — một khoá không migration nào gieo — sống sót ở đây.
//
// Ca này buộc bộ khoá phải có nghĩa: staffauth.Checker đọc đúng bộ khoá mà Middleware đặt vào
// context, và authz.RequirePermission so nó với "budget.read".
func TestBudgetReadPermissionIsReallyChecked(t *testing.T) {
	const investmentProjectRoute = "/api/v1/investment-projects?year=2026"

	// 200 — đúng xã, đúng khoá.
	m := newTestServer(t, staffOfTenantA())
	wantStatus(t, m.call(t, hostA, investmentProjectRoute, fakeSessionToken), http.StatusOK)

	// 403 — đã đăng nhập, đúng xã, KHÔNG có khoá. Đây là nửa chứng minh khoá được đem ra so thật,
	// chứ không phải guard cho qua mọi tài khoản.
	k := newTestServer(t, staffWithoutPermissions())
	wantStatus(t, k.call(t, hostA, investmentProjectRoute, fakeSessionToken), http.StatusForbidden)

	// 401 — không phiếu.
	n := newTestServer(t, staffOfTenantA())
	wantStatus(t, n.call(t, hostA, investmentProjectRoute, ""), http.StatusUnauthorized)

	// 401 — ĐÚNG KHOÁ, SAI XÃ. Hình dạng một vụ rò chéo xã có thật: quyền hợp lệ, nhưng phiếu cấp
	// cho xã khác. Bên phân giải trả "không có chủ thể" đúng như identity trả khi x-tenant-id
	// không khớp xã nằm trong chứng thực (service-identity/internal/grpc/server.go:257).
	x := newTestServer(t, &fakeResolver{ok: false})
	wantStatus(t, x.call(t, hostB, investmentProjectRoute, fakeSessionToken), http.StatusUnauthorized)
}

// emptyInvestmentProjectStore is a project store holding nothing. No case in this file reads a project: the
// properties under test here are about the edge, not about disbursement figures.
type emptyInvestmentProjectStore struct{}

func (emptyInvestmentProjectStore) ListInvestmentProjects(context.Context, fistore.InvestmentProjectFilter) ([]domain.InvestmentProjectProgress, error) {
	return nil, nil
}

func (emptyInvestmentProjectStore) GetInvestmentProject(context.Context, string) (domain.InvestmentProjectProgress, error) {
	return domain.InvestmentProjectProgress{}, fistore.ErrInvestmentProjectNotFound
}

// emptyThresholdStore answers the software's default threshold, which is what every commune is on today —
// `cau_hinh_giai_ngan` has no write path yet. No case in this file reads it; it is here because
// Register refuses to start without one, deliberately (see routes.go).
type emptyThresholdStore struct{}

func (emptyThresholdStore) DelayThreshold(context.Context, int) (domain.DelayThreshold, error) {
	return domain.SoftwareDefaultThreshold(), nil
}

// emptyBudgetStore answers "this commune has no such sheet" — the state of every commune today,
// since nothing seeds `bang_ngan_sach`. No case in this file reads it; it is here because Register
// refuses to start without one, deliberately (see routes.go).
type emptyBudgetStore struct{}

func (emptyBudgetStore) FullSheet(context.Context, int, domain.SheetKind) (domain.FullSheet, error) {
	return domain.FullSheet{}, fistore.ErrBudgetSheetNotFound
}

func (emptyBudgetStore) LineEntries(context.Context, string) (domain.LineEntries, error) {
	return domain.LineEntries{}, domain.ErrLineNotFound
}

// emptyOverrideStore is the override table of a commune that reworded nothing — every commune until
// somebody opens Cấu hình → Lời hệ thống. The project list READS it now (`scope_notice`, investment_project.go),
// so the real use case needs a store that answers; the write methods are never reached from here.
type emptyOverrideStore struct{}

var errNoWritePath = errors.New("emptyOverrideStore: no write path in this file")

func (emptyOverrideStore) ListLive(context.Context) ([]domain.MessageOverride, error) {
	return nil, nil
}

func (emptyOverrideStore) LiveForUpdate(context.Context, *pkgstore.ScopedTx, string) (*domain.MessageOverride, error) {
	return nil, errNoWritePath
}

func (emptyOverrideStore) AddOverride(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}

func (emptyOverrideStore) UpdateText(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}

func (emptyOverrideStore) SoftDelete(context.Context, *pkgstore.ScopedTx, string, string, string, time.Time) error {
	return errNoWritePath
}
