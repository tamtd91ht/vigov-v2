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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	svchttp "github.com/vihat/vigov/service-documents/internal/http"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	staffID    = "nd-01JINTERNALIDCUACANBO"
	fakeTicket = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	routePath = "/api/v1/document-types"
)

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// --- fakes ----------------------------------------------------------------------------------

type fakeDirectory map[string]tenant.Tenant

func (m fakeDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// fakeDocumentTypes is the document-type catalogue, KEYED BY COMMUNE and COUNTING ITS READS.
//
// Both halves are assertions. Keyed by commune, because a store keyed by nothing would let the
// wrong-commune test pass while proving nothing. Counting, because "the store was never touched" is
// the only way to show a refused request stopped at the edge rather than at the handler.
type fakeDocumentTypes struct {
	byTenant map[tenant.ID][]domain.DocumentType
	reads    int
}

func (k *fakeDocumentTypes) List(ctx context.Context) ([]domain.DocumentType, error) {
	k.reads++
	return k.byTenant[tenant.MustFrom(ctx)], nil
}

// fakeResolver is identity, absent, counting its calls.
type fakeResolver struct {
	calls int
	out   staffauth.StaffPrincipal
	found bool
	err   error

	// The commune carried on the outgoing call — see ResolveStaff below.
	sentTenant    tenant.ID
	sentHasTenant bool
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
	p.sentTenant, p.sentHasTenant = tenant.From(ctx)
	return p.out, p.found, p.err
}

// --- the binary's own chain -------------------------------------------------------------------

type testServer struct {
	h        http.Handler
	types    *fakeDocumentTypes
	resolver *fakeResolver
}

func newTestServer(t *testing.T, resolver *fakeResolver) *testServer {
	t.Helper()

	types := &fakeDocumentTypes{byTenant: map[tenant.ID][]domain.DocumentType{
		tenantA: {{ID: "lvb-001", Code: "quyet-dinh", Label: "Quyết định", IsActive: true, IsDefault: true}},
		tenantB: {{ID: "lvb-b-001", Code: "to-trinh", Label: "Tờ trình xã B", IsActive: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY run() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		DocumentTypes: types,
		// The write use case, built on a *store.DB that is nil. NOTHING IN THIS FILE CALLS IT: what
		// this test drives is the edge chain in front of the READ route. Register refuses a nil
		// dependency at construction, so it has to be present — and a use case that is never invoked
		// cannot dereference the nil handle. The write routes have their own four-case suite in
		// internal/http, where the store behind them is a fake that records what was written.
		DocumentTypeWriter: app.NewDocumentTypeCatalogue(nil, nil),

		// THE TWO REGISTERS, PRESENT FOR THE SAME REASON AND JUST AS UNUSED. Register refuses a nil
		// dependency at construction — a route mounted without the thing behind it would accept
		// requests it cannot honour, and the first person to find out would be a clerk registering a
		// document. Their stores and use cases are built on a nil *store.DB and a nil identity
		// client; nothing in this file calls them, and the four-case permission suite for all eleven
		// routes lives in internal/http, over fakes that record what was written.
		IncomingDocuments:      docstore.NewIncomingDocumentStore(nil),
		IncomingDocumentWriter: app.NewIncomingDocuments(nil, nil, nil, nil),
		IncomingDocumentReader: app.NewIncomingDocuments(nil, nil, nil, nil),
		OutgoingDocuments:      docstore.NewOutgoingDocumentStore(nil),
		OutgoingDocumentWriter: app.NewOutgoingDocuments(nil, nil, nil),
		IncomingSummary:        docstore.NewIncomingDocumentStore(nil),
		OverdueQueue:           app.NewIncomingDashboard(docstore.NewIncomingDocumentStore(nil), nil),
		AuditLog:               audit.NewLog(store.New(nil)),

		Log: log,
	})

	return &testServer{
		// A NIL idem.Store IS A VALID DEPLOYMENT and is what runs here: local development with no
		// Redis. Every route this file exercises is a GET, which declares no duplicate protection
		// at all, so the store is never consulted.
		h:        buildEdge(mux, fakeDirectory{hostA: {ID: tenantA, Host: hostA, Active: true}, hostB: {ID: tenantB, Host: hostB, Active: true}}, resolver, nil, log),
		types:    types,
		resolver: resolver,
	}
}

func (m *testServer) call(t *testing.T, host, path, ticket string) *httptest.ResponseRecorder {
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

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

func staffOfTenantA() *fakeResolver {
	return &fakeResolver{
		found: true,
		out: staffauth.StaffPrincipal{
			StaffID:        staffID,
			PermissionKeys: []authz.Perm{"document.read"},
		},
	}
}

// --- the first request this service has ever served ----------------------------------------------

func TestStaffOfTenantAAtHostAIsServed(t *testing.T) {
	// FIRST TIME A ROUTE IN THIS SERVICE SERVES A REQUEST. Until this chain existed, the route
	// answered 401 to everybody — including a member of staff holding a perfectly valid session —
	// because nothing in the binary built an authz.Principal.
	m := newTestServer(t, staffOfTenantA())

	w := m.call(t, hostA, routePath, fakeTicket)
	wantStatus(t, w, http.StatusOK)

	var body struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	// The commune's OWN catalogue, read with the commune from the context — never commune B's.
	if len(body.Items) != 1 || body.Items[0].Code != "quyet-dinh" {
		t.Errorf("danh mục trả về = %+v, muốn danh mục của xã A", body.Items)
	}
	if m.types.reads != 1 {
		t.Errorf("kho được đọc %d lần, muốn 1", m.types.reads)
	}
	if m.resolver.calls != 1 {
		t.Errorf("gọi dịch vụ định danh %d lần cho 1 yêu cầu, muốn 1", m.resolver.calls)
	}
}

func TestSameCredentialAtHostBHasNoPrincipalAndStoreUntouched(t *testing.T) {
	// The same credential at commune B's host. The RPC makes the comparison — the commune in
	// "x-tenant-id" against the commune inside the credential — and answers OK with NO PRINCIPAL,
	// which is what this fake reproduces. The route's own guard then refuses.
	//
	// THE STORE MUST NOT BE TOUCHED. A refusal that still ran the query would mean the isolation
	// rested on the handler rather than on the edge, and a query is where a commune boundary is
	// actually crossed.
	m := newTestServer(t, &fakeResolver{found: false})

	wantStatus(t, m.call(t, hostB, routePath, fakeTicket), http.StatusUnauthorized)
	if m.types.reads != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.types.reads)
	}
}

func TestNoCookieCallsNoIdentity(t *testing.T) {
	// The count is the assertion. A chain that called anyway would answer this request correctly
	// and pay a LAN round trip on every anonymous hit, forever, with nothing reporting it.
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, hostA, routePath, ""), http.StatusUnauthorized)
	if m.resolver.calls != 0 {
		t.Errorf("gọi dịch vụ định danh %d lần khi không có cookie — phải là 0", m.resolver.calls)
	}
	if m.types.reads != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu không có phiên", m.types.reads)
	}
}

func TestIdentityDownIs503Not401(t *testing.T) {
	// If identity is down and this chain answered "no principal", every member of staff would be
	// told to sign in again — through the service that is down.
	m := newTestServer(t, &fakeResolver{err: errors.New("rpc error: code = Unavailable desc = refused")})

	w := m.call(t, hostA, routePath, fakeTicket)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("trả 401 khi dịch vụ định danh chết")
	}
	wantStatus(t, w, http.StatusServiceUnavailable)
	if m.types.reads != 0 {
		t.Errorf("kho bị đọc %d lần trong lúc không xác thực được", m.types.reads)
	}
}

func TestUnresolvableHostIs404AndCallsNobody(t *testing.T) {
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, "khong-co-xa.example.gov.vn", routePath, fakeTicket), http.StatusNotFound)
	if m.resolver.calls != 0 || m.types.reads != 0 {
		t.Errorf("Host không phân giải được vẫn gọi định danh %d lần và đọc kho %d lần", m.resolver.calls, m.types.reads)
	}
}

func TestHealthzSitsOutsideTheTenantChain(t *testing.T) {
	// It answers whether this process is alive, which is true or false regardless of which commune
	// is asking. Behind Host resolution it would fail whenever the platform service does, and an
	// orchestrator would restart a healthy process during somebody else's outage.
	m := newTestServer(t, staffOfTenantA())

	wantStatus(t, m.call(t, "khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
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
func TestResolveCallCarriesTheHostsTenant(t *testing.T) {
	m := newTestServer(t, staffOfTenantA())

	// Xã A ở host A: lời gọi phải mang xã A.
	wantStatus(t, m.call(t, hostA, routePath, fakeTicket), http.StatusOK)
	if !m.resolver.sentHasTenant {
		t.Fatal("lời gọi phân giải KHÔNG mang xã nào — bên kia sẽ so với một giá trị rỗng")
	}
	if m.resolver.sentTenant != tenantA {
		t.Errorf("xã đã gửi = %q, muốn %q (xã suy từ Host)", m.resolver.sentTenant, tenantA)
	}

	// Cùng phiếu ấy ở host B: xã gửi đi phải ĐỔI THEO HOST, không theo phiếu. Đây là nửa bắt được
	// một bản cài đặt lấy xã từ phản hồi RPC thay vì từ Host.
	m2 := newTestServer(t, staffOfTenantA())
	m2.call(t, hostB, routePath, fakeTicket)
	if m2.resolver.sentTenant != tenantB {
		t.Errorf("ở host B, xã đã gửi = %q, muốn %q", m2.resolver.sentTenant, tenantB)
	}
}
