package staffauth

// What these tests defend: the four behaviours of the middleware, and the ONE obligation the
// contract puts on the key set — that it does not outlive the request it authenticated.
//
// Nothing here is a gRPC connection or a database. The Resolver is an interface precisely so the
// refusal behaviour can be exercised without infrastructure; a test that needs a running identity
// service is a test that stops being run, and this is the layer where "identity is down" has to be
// provable.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// --- fixtures -------------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// An INTERNAL id, not a business code — the distinction StaffPrincipal.StaffID carries.
	idCanBo = "nd-01JINTERNALIDCUACANBO"

	// The BUSINESS CODE of the same person. Deliberately a different string from idCanBo: the two
	// fields answer two different questions (authz.Principal says which), and a fixture that used
	// one value for both could not tell a correct implementation from one that sends the id twice.
	maCanBo = "CB-00123"

	// Fake credential material, and the text says so in full (rule 8, forbidden #1). It is never
	// logged and never asserted on beyond "the resolver received exactly this".
	phieuGia = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))

	// BOTH KEYS ARE ROWS OF `quyen`, and that is a property of the test, not decoration.
	// service-identity/migrations/0001_init.sql:288 and :289 load them. A test that grants a key
	// no migration seeds proves the Checker compares two strings and nothing about the permission
	// system this repository actually ships: the key would be one no administrator can grant on
	// the Phân quyền screen, so no route could ever be reached with it (rule 5, invariant 3b).
	// `document.approve` stood here until today and was exactly that — a key present in no
	// migration and in no route.
	quyenDoc  = authz.Perm("document.read")  // the key /can-quyen declares
	quyenKhac = authz.Perm("document.route") // a REAL key that route does NOT declare
)

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã A", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã B", Active: true},
	}
}

// phanGiaiGia is identity, absent. It COUNTS ITS CALLS, and that count is the assertion in two
// different tests: "no cookie means no call", and "the key set does not outlive one request".
type phanGiaiGia struct {
	goi   int
	phieu []string // every credential it was handed, in order
	ip    []string
	tra   StaffPrincipal
	co    bool
	loi   error
}

func (p *phanGiaiGia) ResolveStaff(_ context.Context, phieu, ip string) (StaffPrincipal, bool, error) {
	p.goi++
	p.phieu = append(p.phieu, phieu)
	p.ip = append(p.ip, ip)
	return p.tra, p.co, p.loi
}

func canBoCo(khoa ...authz.Perm) *phanGiaiGia {
	return &phanGiaiGia{tra: StaffPrincipal{StaffID: idCanBo, Ma: maCanBo, PermissionKeys: khoa}, co: true}
}

// --- the chain ------------------------------------------------------------------------------

type mayChu struct {
	h  http.Handler
	pg *phanGiaiGia

	// chamTay records whether the handler behind the guard ran at all, and with which principal.
	// "The store was never touched" is asserted through this: a refused request must not reach
	// anything that reads data.
	chamTay int
	thayChu *authz.Principal
}

// dungMayChu builds the real edge chain in the real order, with one route per declaration so each
// guard's own answer is visible.
func dungMayChu(t *testing.T, pg *phanGiaiGia) *mayChu {
	t.Helper()

	m := &mayChu{pg: pg}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ghiNhan := func(w http.ResponseWriter, r *http.Request) {
		m.chamTay++
		if p, ok := authz.From(r.Context()); ok {
			cp := p
			m.thayChu = &cp
		} else {
			m.thayChu = nil
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}

	c := Checker{}
	mux := http.NewServeMux()
	mux.Handle("GET /cong-khai", authz.Public("tuyến công khai trong bài kiểm — dùng để chứng minh 'không có chủ thể' vẫn phục vụ")(
		http.HandlerFunc(ghiNhan)))
	mux.Handle("GET /da-dang-nhap", authz.AnyAuthenticated("tuyến trong bài kiểm — mọi tài khoản đã đăng nhập của xã")(
		http.HandlerFunc(ghiNhan)))
	mux.Handle("GET /can-quyen", authz.RequirePermission(c, quyenDoc)(
		http.HandlerFunc(ghiNhan)))

	var h http.Handler = mux
	h = Middleware(pg, log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
	return m
}

// goi issues one request. phieu == "" is a request carrying no session cookie at all.
func (m *mayChu) goi(t *testing.T, host, path, phieu string) *httptest.ResponseRecorder {
	t.Helper()
	m.chamTay = 0
	m.thayChu = nil

	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if phieu != "" {
		r.AddCookie(&http.Cookie{Name: CookieName, Value: phieu})
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func doiMa(t *testing.T, w *httptest.ResponseRecorder, muon int) {
	t.Helper()
	if w.Code != muon {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, muon, w.Body.String())
	}
}

// --- 1. no cookie means no call ---------------------------------------------------------------

func TestKhongCoCookieThiKhongGoiDinhDanh(t *testing.T) {
	// THE COUNT IS THE ASSERTION. A middleware that called anyway would still answer correctly on
	// every one of these requests — and would pay a LAN round trip on every anonymous hit and every
	// Public route, forever, with nothing anywhere reporting it. Only a call count can see that.
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/cong-khai", ""), http.StatusOK)
	doiMa(t, m.goi(t, hostA, "/da-dang-nhap", ""), http.StatusUnauthorized)
	doiMa(t, m.goi(t, hostA, "/can-quyen", ""), http.StatusUnauthorized)

	if pg.goi != 0 {
		t.Errorf("gọi dịch vụ định danh %d lần khi yêu cầu không mang cookie — phải là 0", pg.goi)
	}
}

func TestCookieRongCungKhongGoi(t *testing.T) {
	// A cookie present but empty is the same thing as no cookie: there is no credential to resolve,
	// and the contract answers INVALID_ARGUMENT for an empty session_token rather than "no
	// principal", precisely so this case never reaches the wire.
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/cong-khai", nil)
	r.Host = hostA
	r.AddCookie(&http.Cookie{Name: CookieName, Value: ""})
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	doiMa(t, w, http.StatusOK)
	if pg.goi != 0 {
		t.Errorf("gọi %d lần với cookie rỗng — phải là 0", pg.goi)
	}
}

// --- 2. a principal reaches the handler --------------------------------------------------------

func TestChuTheDenDuocHandler(t *testing.T) {
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/da-dang-nhap", phieuGia), http.StatusOK)
	if m.chamTay != 1 {
		t.Fatalf("handler chạy %d lần, muốn 1", m.chamTay)
	}
	if m.thayChu == nil {
		t.Fatal("handler không thấy chủ thể nào trong context")
	}
	if m.thayChu.ID != idCanBo {
		t.Errorf("Principal.ID = %q, muốn id NỘI BỘ %q — mã nghiệp vụ ở đây khớp 0 dòng và mọi quyền trả false",
			m.thayChu.ID, idCanBo)
	}
	if m.thayChu.Kind != "staff" {
		t.Errorf("Principal.Kind = %q, muốn \"staff\"", m.thayChu.Kind)
	}
	if m.thayChu.TenantID != xaA {
		t.Errorf("Principal.TenantID = %q, muốn xã phân giải từ Host %q", m.thayChu.TenantID, xaA)
	}
	if len(m.thayChu.Roles) != 0 {
		t.Errorf("Principal.Roles = %v, phải rỗng — vai trò mang trên chủ thể là vai trò nhúng trong phiếu", m.thayChu.Roles)
	}

	// The credential was passed through verbatim — not parsed, not trimmed, not re-encoded. The
	// caller does not hold the signing key and must not build a second, weaker opinion about it.
	if pg.goi != 1 || pg.phieu[0] != phieuGia {
		t.Errorf("phiếu gửi đi = %v (gọi %d lần), muốn đúng một lần với giá trị nguyên văn", pg.phieu, pg.goi)
	}
	// The address comes from this process's own socket, never from X-Forwarded-For.
	if pg.ip[0] != "10.0.0.7" {
		t.Errorf("client_ip = %q, muốn địa chỉ quan sát trên socket", pg.ip[0])
	}
}

// --- 3. OK with no principal: the route's own guard answers ------------------------------------

func TestKhongCoChuTheThiGuardTraLoi(t *testing.T) {
	// A stale cookie, a revoked session, an account locked, a credential of another commune — the
	// contract answers all of them the same way, and NONE of them may become a 401 in the
	// middleware. The Public route is the proof: turn "no principal" into 401 here and a commune's
	// public surface goes dark the moment somebody's cookie goes stale.
	pg := &phanGiaiGia{co: false}
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/cong-khai", phieuGia), http.StatusOK)
	if m.chamTay != 1 {
		t.Errorf("tuyến Public không phục vụ khi không có chủ thể — handler chạy %d lần", m.chamTay)
	}
	if m.thayChu != nil {
		t.Errorf("có chủ thể trong context dù dịch vụ định danh trả về không có: %+v", m.thayChu)
	}

	doiMa(t, m.goi(t, hostA, "/da-dang-nhap", phieuGia), http.StatusUnauthorized)
	if m.chamTay != 0 {
		t.Error("tuyến AnyAuthenticated chạm tới handler dù không có chủ thể")
	}
}

// --- 4. the call failed: 503, and specifically NOT 401 -----------------------------------------

func TestDinhDanhChetThiTra503ChuKhongPhai401(t *testing.T) {
	// THIS IS THE TEST THE WHOLE CHAIN EXISTS FOR. Answering 401 here tells every member of staff
	// of every commune to sign in again, and the sign-in runs on the service that is down. A
	// browser that sees 401 also clears its session, so a few minutes of outage ends with everybody
	// genuinely signed out.
	pg := &phanGiaiGia{loi: errors.New("rpc error: code = Unavailable desc = connection refused")}
	m := dungMayChu(t, pg)

	for _, tuyen := range []string{"/cong-khai", "/da-dang-nhap", "/can-quyen"} {
		w := m.goi(t, hostA, tuyen, phieuGia)
		if w.Code == http.StatusUnauthorized {
			t.Fatalf("%s: trả 401 khi dịch vụ định danh chết — đúng cái điều cấm", tuyen)
		}
		doiMa(t, w, http.StatusServiceUnavailable)
		if m.chamTay != 0 {
			t.Errorf("%s: chạm tới handler dù không xác thực được", tuyen)
		}
	}
}

func TestTra503KhongLoRaPhieuPhien(t *testing.T) {
	// The body a client reads must never carry the credential, the internal error, or the name of
	// the service that failed (rule 3, rule 8). A 503 that echoes the token is a token in every
	// proxy log between here and the browser.
	pg := &phanGiaiGia{loi: errors.New("dial tcp: " + phieuGia)}
	m := dungMayChu(t, pg)

	w := m.goi(t, hostA, "/da-dang-nhap", phieuGia)
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), phieuGia) {
		t.Errorf("thân lỗi chứa phiếu phiên: %s", w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "dial tcp") {
		t.Errorf("thân lỗi chứa chi tiết nội bộ: %s", w.Body.String())
	}
}

// --- 5. present-with-no-keys is NOT no-principal -----------------------------------------------

func TestChuTheKhongCoQuyenNaoVanLaChuThe(t *testing.T) {
	// A live session held by somebody whose role was withdrawn. AnyAuthenticated must serve them;
	// RequirePermission must refuse them with 403.
	//
	// MERGING THIS INTO "no principal" SIGNS THAT PERSON OUT instead of telling them they may not
	// do this one thing — and they would then sign in again, successfully, and meet the same
	// screen. The distinction is 401 versus 403, and only one of them is true.
	pg := canBoCo() // present, empty key set
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/da-dang-nhap", phieuGia), http.StatusOK)
	if m.chamTay != 1 {
		t.Error("AnyAuthenticated từ chối một phiên hợp lệ chỉ vì người đó không giữ quyền nào")
	}

	w := m.goi(t, hostA, "/can-quyen", phieuGia)
	doiMa(t, w, http.StatusForbidden)
	if m.chamTay != 0 {
		t.Error("RequirePermission cho qua một người không giữ quyền")
	}
}

// --- 6. RequirePermission reads the key set ----------------------------------------------------

func TestRequirePermissionDocDungBoKhoa(t *testing.T) {
	// Holding a key: served. Holding a DIFFERENT key: refused. The second half matters as much as
	// the first — a Checker that answered true for anything would pass a test that only tried the
	// key the route declares.
	co := dungMayChu(t, canBoCo(quyenDoc))
	doiMa(t, co.goi(t, hostA, "/can-quyen", phieuGia), http.StatusOK)
	if co.chamTay != 1 {
		t.Error("người giữ đúng quyền vẫn không tới được handler")
	}

	khong := dungMayChu(t, canBoCo(quyenKhac))
	w := khong.goi(t, hostA, "/can-quyen", phieuGia)
	doiMa(t, w, http.StatusForbidden)
	if khong.chamTay != 0 {
		t.Error("người giữ quyền KHÁC vẫn tới được handler")
	}
}

// --- 7. the key set does not outlive one request -----------------------------------------------

func TestBoQuyenKhongSongQuaMotYeuCau(t *testing.T) {
	// THE MUTATION THIS CATCHES: a cache — package-level, per-client, keyed by token, "just a short
	// TTL" — in front of the resolver. It is the optimisation the next person will reach for,
	// because the call is on the hot path of every staff request in four services.
	//
	// TWO ASSERTIONS, and they catch different shapes of the same mistake:
	//
	//	the call count      catches ANY cache, however it is keyed — a second request must produce
	//	                    a second call, no matter what the answer was.
	//	the changed answer  catches a cache that happens to be refreshed for other reasons: a role
	//	                    withdrawn between two requests must take effect on the SECOND one, not
	//	                    when the session ends (rule 5, invariant 4).
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusOK)

	// The role is withdrawn. Same person, same cookie, same everything else.
	pg.tra = StaffPrincipal{StaffID: idCanBo}

	w := m.goi(t, hostA, "/can-quyen", phieuGia)
	doiMa(t, w, http.StatusForbidden)
	if pg.goi != 2 {
		t.Errorf("gọi dịch vụ định danh %d lần cho 2 yêu cầu — bộ khoá đang sống quá một yêu cầu", pg.goi)
	}
}

func TestBoQuyenKhongRoSangYeuCauKhac(t *testing.T) {
	// The other direction of the same property, and the one a call count alone would miss: request
	// A holds the key, request B (same server, same middleware instance) holds none. If anything
	// about the set were stored outside the context — a field on the middleware, a package
	// variable — B would inherit A's grants.
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusOK)

	pg.co = false // request B carries a credential that resolves to nothing at all
	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusUnauthorized)

	pg.co = true
	pg.tra = StaffPrincipal{StaffID: "nd-01JMOTNGUOIKHACHOANTOAN"}
	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusForbidden)
}

// --- 8. the commune -----------------------------------------------------------------------------

func TestChuTheMangXaPhanGiaiTuHost(t *testing.T) {
	// The same credential presented at two hosts. The commune on the principal follows HOST, which
	// is what authz.xacNhanXa compares against — and the reason that is not a tautology is that the
	// RPC already compared the Host commune (sent in "x-tenant-id") against the commune inside the
	// credential, and answered no principal at all on a mismatch.
	//
	// This test pins the FIRST half: whatever commune the edge resolved is the commune stamped on
	// the principal, never a commune the credential or the client named.
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, hostA, "/da-dang-nhap", phieuGia), http.StatusOK)
	if m.thayChu == nil || m.thayChu.TenantID != xaA {
		t.Fatalf("ở host A, chủ thể mang xã %v", m.thayChu)
	}

	doiMa(t, m.goi(t, hostB, "/da-dang-nhap", phieuGia), http.StatusOK)
	if m.thayChu == nil || m.thayChu.TenantID != xaB {
		t.Fatalf("ở host B, chủ thể mang xã %v", m.thayChu)
	}
}

func TestGiayToXaKhacKhongCoChuThe(t *testing.T) {
	// A credential of commune A presented at commune B's host. The RPC makes that comparison and
	// answers OK with NO PRINCIPAL — which is what this fake reproduces — and the route's own guard
	// refuses. The handler must not run: nothing that reads data may be reached.
	pg := &phanGiaiGia{co: false}
	m := dungMayChu(t, pg)

	w := m.goi(t, hostB, "/da-dang-nhap", phieuGia)
	doiMa(t, w, http.StatusUnauthorized)
	if m.chamTay != 0 {
		t.Error("yêu cầu bị từ chối vẫn chạm tới handler")
	}
}

func TestHostKhongPhanGiaiDuocThiKhongGoiDinhDanh(t *testing.T) {
	// 404 from TenantMiddleware, and the middleware behind it never runs — it is mounted INSIDE, so
	// a Host that resolves to nothing costs no round trip to identity either.
	pg := canBoCo(quyenDoc)
	m := dungMayChu(t, pg)

	doiMa(t, m.goi(t, "khong-co-xa.example.gov.vn", "/da-dang-nhap", phieuGia), http.StatusNotFound)
	if pg.goi != 0 {
		t.Errorf("gọi dịch vụ định danh %d lần cho một Host không phân giải được", pg.goi)
	}
}

// --- 9. wiring refusals --------------------------------------------------------------------------

func TestMiddlewareTuChoiResolverNil(t *testing.T) {
	// At construction, where a human is watching a process fail to start — not on the first request
	// of the first member of staff.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Middleware(nil) dựng được — sẽ chết giữa chuỗi biên lúc có người gọi")
		}
	}()
	_ = Middleware(nil, nil)
}

func TestMiddlewareNgoaiTenantMiddlewareThiPanic(t *testing.T) {
	// Mounted outside the commune edge, the outgoing call would carry no commune, core/grpcx would
	// refuse to send it, and every staff request would answer 503 with a message about identity
	// being unreachable while identity was perfectly healthy. Panicking names the wiring instead.
	pg := canBoCo(quyenDoc)
	h := Middleware(pg, slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("chạy ngoài httpx.TenantMiddleware mà không panic")
		}
	}()
	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/da-dang-nhap", nil)
	h.ServeHTTP(httptest.NewRecorder(), r)
}

// --- 10. Checker fails closed on every branch ----------------------------------------------------

func TestCheckerTuChoiKhiThieuBoiCanh(t *testing.T) {
	c := Checker{}
	chuThe := authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaA}

	// No key set in the context: the middleware never ran for this request.
	if c.Allows(tenant.Into(context.Background(), xaA), chuThe, quyenDoc) {
		t.Error("cho qua khi context không có bộ khoá nào")
	}

	daCo := vaoBoQuyen(tenant.Into(context.Background(), xaA),
		boQuyen{canBo: idCanBo, xa: xaA, khoa: map[authz.Perm]struct{}{quyenDoc: {}}})

	if !c.Allows(daCo, chuThe, quyenDoc) {
		t.Fatal("từ chối đúng người, đúng xã, đúng khoá")
	}

	// A handler that builds its own principal and asks about it. The set belongs to ONE person.
	if c.Allows(daCo, authz.Principal{ID: "nd-nguoi-khac", Kind: "staff", TenantID: xaA}, quyenDoc) {
		t.Error("bộ khoá của người này trả lời cho id của người khác")
	}

	// ...and to ONE commune.
	if c.Allows(tenant.Into(context.Background(), xaB), chuThe, quyenDoc) {
		t.Error("bộ khoá của xã A trả lời trong ngữ cảnh xã B")
	}
	if c.Allows(daCo, authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaB}, quyenDoc) {
		t.Error("chủ thể mang xã B được trả lời bằng bộ khoá của xã A")
	}

	// No commune at all: false, and NOT a panic. A Checker can be reached from a background
	// goroutine holding a derived context, and taking the process down there turns one mis-wired
	// call into an outage.
	if c.Allows(vaoBoQuyen(context.Background(), boQuyen{canBo: idCanBo, xa: xaA, khoa: map[authz.Perm]struct{}{quyenDoc: {}}}),
		chuThe, quyenDoc) {
		t.Error("cho qua khi context không có xã")
	}

	// A citizen never holds a staff permission: citizens are isolated by identity (rule 4), not
	// by RBAC.
	if c.Allows(daCo, authz.Principal{ID: idCanBo, Kind: "citizen", TenantID: xaA}, quyenDoc) {
		t.Error("chủ thể công dân được trả lời bằng quyền cán bộ")
	}

	// An empty permission key matches nothing.
	if c.Allows(daCo, chuThe, "") {
		t.Error("cho qua với khoá quyền rỗng")
	}
}

// --- the business code on the principal -----------------------------------------------------

// TestMaSangChuThe asserts the one thing that carries `ma` from the RPC to the audit trail.
//
// IT IS AN ASSERTION ABOUT A VALUE, NOT ABOUT A FIELD NAME. `audit_log.actor_id` is what a person
// handling a complaint or an inspection reads years later, and rule 6, invariant 2 wants a "who"
// that still means something then. On 2026-09-22 four services wrote authz.Principal.ID — a ULID
// — into that column, every one of their tests stayed green, and the column ended up holding two
// kinds of identifier at once: one nobody can query.
func TestMaSangChuThe(t *testing.T) {
	m := dungMayChu(t, canBoCo(quyenDoc))
	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusOK)

	if m.thayChu == nil {
		t.Fatal("không có chủ thể — chuỗi biên hỏng, ca này không nói được gì")
	}
	if m.thayChu.Ma != maCanBo {
		t.Errorf("Principal.Ma = %q, muốn mã nghiệp vụ %q — vết kiểm toán không có gì để ghi",
			m.thayChu.Ma, maCanBo)
	}
	// AND ID IS STILL THE INTERNAL ONE. The two must not collapse: ID is what every permission
	// check joins on (see authz.Principal), so making it carry the code answers 403 everywhere.
	if m.thayChu.ID != idCanBo {
		t.Errorf("Principal.ID = %q, muốn id nội bộ %q", m.thayChu.ID, idCanBo)
	}
}

// TestMaRongKhongLayIDThayThe is the half a green suite cannot otherwise prove.
//
// `ma` is OPTIONAL on the wire (rule 2, forbidden #4), so an identity deployed before the field
// existed answers with it empty. The tempting repair — fall back to StaffID — is exactly the
// defect being fixed, and it would be invisible: the write succeeds, the trail looks populated,
// and the value in it is a ULID again. Empty stays empty here, and the WRITE refuses instead.
func TestMaRongKhongLayIDThayThe(t *testing.T) {
	pg := &phanGiaiGia{tra: StaffPrincipal{StaffID: idCanBo, PermissionKeys: []authz.Perm{quyenDoc}}, co: true}
	m := dungMayChu(t, pg)
	doiMa(t, m.goi(t, hostA, "/can-quyen", phieuGia), http.StatusOK)

	if m.thayChu == nil {
		t.Fatal("không có chủ thể — chuỗi biên hỏng, ca này không nói được gì")
	}
	if m.thayChu.ID != idCanBo {
		t.Fatalf("Principal.ID = %q, muốn id nội bộ %q", m.thayChu.ID, idCanBo)
	}
	if m.thayChu.Ma != "" {
		t.Errorf("Principal.Ma = %q khi định danh không gửi `ma` — một giá trị thay thế ở đây đưa "+
			"id nội bộ trở lại cột actor_id, âm thầm, với mọi phép kiểm vẫn xanh", m.thayChu.Ma)
	}
}
