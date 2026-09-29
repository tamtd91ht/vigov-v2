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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	staffID    = "nd-01JINTERNALIDCUACANBO"
	fakeTicket = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	route = "/api/v1/map-asset-types"

	labelTenantA = "Doanh nghiệp xã A"
	labelTenantB = "Doanh nghiệp xã B"
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

// fakeCatalogue is the catalogue, KEYED BY COMMUNE and COUNTING ITS READS.
//
// Both halves are assertions. Keyed by commune, because a store keyed by nothing would let the
// wrong-commune test pass while proving nothing. Counting, because "the store was never touched" is
// the only way to show a refused request stopped at the edge rather than at the handler.
type fakeCatalogue struct {
	byTenant map[tenant.ID][]domain.MapAssetType
	reads    int
}

func (k *fakeCatalogue) List(ctx context.Context) ([]domain.MapAssetType, error) {
	k.reads++
	return k.byTenant[tenant.MustFrom(ctx)], nil
}

type fakeResolver struct {
	calls int
	out   staffauth.StaffPrincipal
	found bool
	err   error

	// The commune carried on the outgoing call — see ResolveStaff below.
	sentTenant tenant.ID
	hadTenant  bool
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
	p.sentTenant, p.hadTenant = tenant.From(ctx)
	return p.out, p.found, p.err
}

type server struct {
	h         http.Handler
	catalogue *fakeCatalogue
	resolver  *fakeResolver
}

func newServer(t *testing.T, resolver *fakeResolver) *server {
	t.Helper()

	catalogue := &fakeCatalogue{byTenant: map[tenant.ID][]domain.MapAssetType{
		tenantA: {{ID: "ltn-001", Code: "doanh-nghiep", Label: labelTenantA, IsActive: true, IsDefault: true}},
		tenantB: {{ID: "ltn-b-001", Code: "doanh-nghiep", Label: labelTenantB, IsActive: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY main() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		MapAssetTypes: catalogue,
		// Built on a nil *store.DB. NOTHING IN THIS FILE CALLS IT: this test is about the edge
		// chain — Host -> commune -> principal — and it asserts on the catalogue READ route.
		// Register refuses a nil dependency at construction, so it has to be present.
		WriteMapAssetTypes: commsapp.NewMapAssetTypeCatalogue(nil, nil),
		// The announcement book, built on a nil *store.DB for the same reason and with the same
		// consequence: NOTHING IN THIS FILE CALLS EITHER. Register refuses a nil dependency at
		// construction, so both have to be present, and a dependency that is never invoked cannot
		// answer anything wrongly. The announcement routes have their own four-case permission suite
		// in internal/http/announcement_test.go.
		Announcements:      commsstore.NewAnnouncementStore(nil),
		WriteAnnouncements: commsapp.NewAnnouncements(nil, nil),
		// The Mini App content register, on a nil *store.DB for the same reason and with the same
		// consequence: NOTHING IN THIS FILE CALLS ANY OF THE FOUR. Its own four-case permission suite
		// — six routes, twenty-four cases — is in internal/http/content_item_test.go.
		ContentItems:           commsstore.NewContentItemStore(nil),
		WriteContentItems:      commsapp.NewContentItems(nil, nil, nil),
		ContentCategories:      commsstore.NewContentCategoryStore(nil),
		WriteContentCategories: commsapp.NewContentCategories(nil, nil),
		// The map field schema, on a nil *store.DB for the same reason: nothing here calls it; its
		// four-case suite is internal/http/map_field_schema_test.go.
		MapFieldSchemas:      commsstore.NewMapFieldSchemaStore(nil),
		WriteMapFieldSchemas: commsapp.NewMapFieldSchemas(nil, nil),
		// The mail server, on a nil *store.DB and a nil envelope for the same reason: nothing here
		// calls it; its four-case suite is internal/http/mail_settings_test.go.
		MailSettings:      commsapp.NewMailSettingsAdmin(nil, nil, nil, nil),
		WriteMailSettings: commsapp.NewMailSettingsAdmin(nil, nil, nil, nil),
		AuditLog:          audit.NewLog(pkgstore.New(nil)),
		// The header bell, on a nil *store.DB for the same reason: nothing here calls it; its suite is
		// internal/http/staff_notification_test.go.
		StaffInbox:      commsstore.NewStaffNotificationStore(nil),
		WriteStaffInbox: commsapp.NewStaffNotifications(nil, nil),
		Log:             log,
	})

	directory := fakeDirectory{
		hostA: {ID: tenantA, Host: hostA, Active: true},
		hostB: {ID: tenantB, Host: hostB, Active: true},
	}
	// The public chain is the real one too (public_chain_test.go drives it); here it only has to be
	// present so the staff tests run through the same outer mux main() builds.
	return &server{h: buildEdge(mux, newTestPublicChain(t), directory, resolver, nil, log), catalogue: catalogue, resolver: resolver}
}

func (m *server) call(t *testing.T, host, path, ticket string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if ticket != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: ticket})
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

// staffOfTenantA is a member of staff of commune A, signed in and HOLDING NO PERMISSION KEY AT ALL —
// and the empty key set is the assertion, not an omission.
//
// WHAT STOOD HERE UNTIL TODAY: `PermissionKeys: []authz.Perm{"map.read"}`. `map.read` is a string
// that appears in no migration and in no route in this repository — the seeded catalogue calls
// that subsystem `asset.*` (service-identity/migrations/0001_init.sql:280-281). A key absent from
// `quyen` is a key no administrator can grant on the Phân quyền screen, so a route guarded by it
// could never be reached by anybody (rule 5, invariant 3b).
//
// It survived because it was never read. The one route this service mounts —
// GET /api/v1/map-asset-types — declares authz.AnyAuthenticated, and that guard never consults the
// key set (core/authz/authz.go:187-208). Any string whatsoever made these cases green, which is
// precisely why an invented one could sit here unnoticed.
//
// So the honest fixture is an account that holds nothing, and it carries a real assertion a
// plausible-looking key never could: THIS ROUTE MUST SERVE AN ACCOUNT WITH NO RIGHTS. That is the
// trade-off routes.go:146 states in writing. The day somebody swaps that declaration for
// RequirePermission, every case in this file turns red — which is the whole point, because the
// screens that fill their category pickers from it would go blank for every non-administrator.
//
// NO REAL KEY IS WRITTEN HERE INSTEAD: `asset.read` would read as "this route needs asset.read",
// which is false, and the next person would build a role around it.
func staffOfTenantA() *fakeResolver {
	return &fakeResolver{found: true, out: staffauth.StaffPrincipal{StaffID: staffID}}
}

func TestStaffOfTenantAAtHostAIsServed(t *testing.T) {
	// FIRST TIME A ROUTE IN THIS SERVICE SERVES A REQUEST. Until this chain existed, the route
	// answered 401 to everybody — including a member of staff holding a perfectly valid session —
	// because nothing in the binary built an authz.Principal.
	m := newServer(t, staffOfTenantA())

	w := m.call(t, hostA, route, fakeTicket)
	expectStatus(t, w, http.StatusOK)

	// The commune's OWN catalogue, read with the commune from the context — never commune B's.
	if !strings.Contains(w.Body.String(), labelTenantA) || strings.Contains(w.Body.String(), labelTenantB) {
		t.Errorf("thân trả về không phải danh mục của xã A: %s", w.Body.String())
	}
	if m.catalogue.reads != 1 || m.resolver.calls != 1 {
		t.Errorf("đọc kho %d lần, gọi định danh %d lần — muốn 1 và 1", m.catalogue.reads, m.resolver.calls)
	}
}

func TestSameCredentialAtHostBHasNoPrincipalAndStoreUntouched(t *testing.T) {
	// The same credential at commune B's host. The RPC makes the comparison — the commune in
	// "x-tenant-id" against the commune inside the credential — and answers OK with NO PRINCIPAL,
	// which is what this fake reproduces. The route's own guard then refuses.
	//
	// THE STORE MUST NOT BE TOUCHED: a query is where a commune boundary is actually crossed.
	m := newServer(t, &fakeResolver{found: false})

	expectStatus(t, m.call(t, hostB, route, fakeTicket), http.StatusUnauthorized)
	if m.catalogue.reads != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.catalogue.reads)
	}
}

func TestNoCookieDoesNotCallIdentity(t *testing.T) {
	// The count is the assertion. A chain that called anyway would answer this request correctly
	// and pay a LAN round trip on every anonymous hit, forever, with nothing reporting it.
	m := newServer(t, staffOfTenantA())

	expectStatus(t, m.call(t, hostA, route, ""), http.StatusUnauthorized)
	if m.resolver.calls != 0 || m.catalogue.reads != 0 {
		t.Errorf("không có cookie mà vẫn gọi định danh %d lần, đọc kho %d lần", m.resolver.calls, m.catalogue.reads)
	}
}

func TestIdentityDownIs503Not401(t *testing.T) {
	// If identity is down and this chain answered "no principal", every member of staff would be
	// told to sign in again — through the service that is down.
	m := newServer(t, &fakeResolver{err: errors.New("rpc error: code = Unavailable desc = refused")})

	w := m.call(t, hostA, route, fakeTicket)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("trả 401 khi dịch vụ định danh chết")
	}
	expectStatus(t, w, http.StatusServiceUnavailable)
	if m.catalogue.reads != 0 {
		t.Errorf("kho bị đọc %d lần trong lúc không xác thực được", m.catalogue.reads)
	}
}

func TestUnresolvableHostIs404AndCallsNobody(t *testing.T) {
	m := newServer(t, staffOfTenantA())

	expectStatus(t, m.call(t, "khong-co-xa.example.gov.vn", route, fakeTicket), http.StatusNotFound)
	if m.resolver.calls != 0 || m.catalogue.reads != 0 {
		t.Errorf("Host không phân giải được vẫn gọi định danh %d lần và đọc kho %d lần", m.resolver.calls, m.catalogue.reads)
	}
}

func TestHealthzIsOutsideTheCommuneChain(t *testing.T) {
	// It answers whether this process is alive, which is true or false regardless of which commune
	// is asking. Behind Host resolution it would fail whenever the platform service does, and an
	// orchestrator would restart a healthy process during somebody else's outage.
	m := newServer(t, staffOfTenantA())

	expectStatus(t, m.call(t, "khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
	if m.resolver.calls != 0 {
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
func TestResolveCallCarriesHostCommune(t *testing.T) {
	m := newServer(t, staffOfTenantA())

	// Xã A ở host A: lời gọi phải mang xã A.
	expectStatus(t, m.call(t, hostA, route, fakeTicket), http.StatusOK)
	if !m.resolver.hadTenant {
		t.Fatal("lời gọi phân giải KHÔNG mang xã nào — bên kia sẽ so với một giá trị rỗng")
	}
	if m.resolver.sentTenant != tenantA {
		t.Errorf("xã đã gửi = %q, muốn %q (xã suy từ Host)", m.resolver.sentTenant, tenantA)
	}

	// Cùng phiếu ấy ở host B: xã gửi đi phải ĐỔI THEO HOST, không theo phiếu. Đây là nửa bắt được
	// một bản cài đặt lấy xã từ phản hồi RPC thay vì từ Host.
	m2 := newServer(t, staffOfTenantA())
	m2.call(t, hostB, route, fakeTicket)
	if m2.resolver.sentTenant != tenantB {
		t.Errorf("ở host B, xã đã gửi = %q, muốn %q", m2.resolver.sentTenant, tenantB)
	}
}
