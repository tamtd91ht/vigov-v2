package main

// What this test defends: THE WIRING, not the middleware.
//
// core/staffauth proves the four behaviours of the authentication middleware and core/httpx proves
// what TenantMiddleware does. Neither of them can see whether THIS binary installs them. A
// middleware deleted from dungBien leaves a service that starts, serves and answers — and answers
// 401 to every member of staff holding a perfectly valid session, which is the exact state this
// service shipped in until today. Nothing else in this repository turns red for that.
//
// So this test drives the chain dungBien actually builds, with the REAL route mounted behind it.

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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	svchttp "github.com/vihat/vigov/service-petitions/internal/http"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	idCanBo  = "nd-01JINTERNALIDCUACANBO"
	phieuGia = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	tuyen = "/api/v1/task-types"

	nhanXaA = "Theo văn bản xã A"
	nhanXaB = "Theo văn bản xã B"

	// --- the citizen surface -------------------------------------------------------------------
	//
	// hostMiniApp IS NOT IN thuMucGia, AND THAT IS THE PRODUCTION SHAPE, not a convenience. The
	// Mini App has no domain: it calls ONE API host that maps to no commune (ADR 0005), so this
	// host is 404 on the staff chain by construction. Every citizen assertion below turns on that.
	hostMiniApp = "api.example.gov.vn"

	tokenCongDan  = "token-phien-cong-dan-GIA-KHONG-PHAI-THAT"
	idCongDan     = "cd-01JOPAQUECUACONGDAN"
	maPhieuCuaToi = "PA-4K7M-92XR-BTVD"
)

// mocGui is a FIXED instant. A deadline expressed relative to time.Now() makes an assertion depend
// on when the suite runs.
var mocGui = time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// khoGia is the catalogue, KEYED BY COMMUNE and COUNTING ITS READS.
//
// Both halves are assertions. Keyed by commune, because a store keyed by nothing would let the
// wrong-commune test pass while proving nothing. Counting, because "the store was never touched" is
// the only way to show a refused request stopped at the edge rather than at the handler.
type khoGia struct {
	theo map[tenant.ID][]domain.LoaiNhiemVu
	doc  int
}

func (k *khoGia) DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error) {
	k.doc++
	return k.theo[tenant.MustFrom(ctx)], nil
}

// khoUuTien is the task-priority catalogue, present only so Register accepts the wiring.
type khoUuTien struct{}

func (khoUuTien) DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error) {
	// tenant.MustFrom even here: a store that read without a commune would read every commune, and
	// a stand-in that skips the precondition teaches the wrong shape to whoever copies it.
	_ = tenant.MustFrom(ctx)
	return nil, nil
}

// khoPhieu and khoNhanLinhVuc are the petition register and its label overrides, present for the
// same reason as khoUuTien: Register refuses incomplete Deps at construction, and the assertions
// in this file are about the EDGE CHAIN rather than about any one route. What that route does is
// proved in internal/http, with a harness that can inject a principal per request.
type khoPhieu struct{}

func (khoPhieu) TheoMaTraCuu(ctx context.Context, _ string) (domain.PhieuPhanAnh, error) {
	_ = tenant.MustFrom(ctx)
	return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
}

// DanhSach lets the same stand-in serve the register list. IT STILL ASSERTS THE COMMUNE IS IN THE
// CONTEXT, which is the only thing this file's tests can prove about a store: a route reachable
// without one would panic inside store.Scoped in production, on a member of staff's screen.
func (khoPhieu) DanhSach(ctx context.Context, _ petstore.LocPhieu, _ page.Request) (
	page.Result[domain.PhieuPhanAnh], error) {

	_ = tenant.MustFrom(ctx)
	return page.NewResult[domain.PhieuPhanAnh](), nil
}

// khoNhiemVu stands in for the TASK register, and it serves both read routes from one type for the
// same reason khoPhieu does: this file is about the EDGE CHAIN — Host -> commune -> principal — and
// the only thing it can prove about a store is that the commune reached it. A route reachable without
// one would panic inside store.Scoped in production, on a member of staff's screen.
type khoNhiemVu struct{}

func (khoNhiemVu) TheoMa(ctx context.Context, _ string) (domain.NhiemVu, error) {
	_ = tenant.MustFrom(ctx)
	return domain.NhiemVu{}, petstore.ErrNhiemVuKhongTonTai
}

func (khoNhiemVu) DanhSach(ctx context.Context, _ petstore.LocNhiemVu, _ page.Request) (
	page.Result[domain.NhiemVu], error) {

	_ = tenant.MustFrom(ctx)
	return page.NewResult[domain.NhiemVu](), nil
}

// VanBanCuaNhiemVu — §5.4's document block, asserting the same one thing: the commune reached the
// store. It is unreachable in practice from these tests, because TheoMa above always refuses first.
func (khoNhiemVu) VanBanCuaNhiemVu(ctx context.Context, _ string) ([]domain.NhiemVuVanBan, error) {
	_ = tenant.MustFrom(ctx)
	return nil, nil
}

// khoBienBan stands in for the MEETING MINUTES register, present for the same reason as the stores
// above: Register refuses incomplete Deps at construction. It asserts the one thing this file can
// assert about a store — that the commune reached it.
type khoBienBan struct{}

func (khoBienBan) DanhSach(ctx context.Context, _ page.Request) (
	page.Result[domain.BienBanHop], error) {

	_ = tenant.MustFrom(ctx)
	return page.NewResult[domain.BienBanHop](), nil
}

type khoNhanLinhVuc struct{}

func (khoNhanLinhVuc) DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error) {
	_ = tenant.MustFrom(ctx)
	return nil, nil
}

// vetGia stands in for the full-view audit trail, present for the same reason as the two above.
//
// IT RETURNS AN ERROR RATHER THAN nil, and that is the safe stand-in rather than a lazy one: no
// assertion in this file reaches the unmask branch, and if one ever does, "the trail failed" makes
// the route refuse to disclose. A stand-in that answered nil would hand back a citizen's real
// number with nothing written anywhere.
type vetGia struct{}

func (vetGia) GhiVet(ctx context.Context, _ string, _ audit.Actor) error {
	_ = tenant.MustFrom(ctx)
	return errors.New("vết giả: phép kiểm này không đi qua nhánh xem đầy đủ")
}

type phanGiaiGia struct {
	goi int
	tra staffauth.StaffPrincipal
	co  bool
	loi error

	// The commune carried on the outgoing call — see ResolveStaff below.
	xaDaGui tenant.ID
	coXa    bool
}

func (p *phanGiaiGia) ResolveStaff(ctx context.Context, _, _ string) (staffauth.StaffPrincipal, bool, error) {
	p.goi++
	// THE COMMUNE THAT LEAVES THIS PROCESS IS RECORDED, AND IT IS THE ONLY THING THIS SERVICE CAN
	// ASSERT ABOUT THE CROSS-COMMUNE REFUSAL.
	//
	// authz.AnyAuthenticated's own commune check cannot fail here — core/staffauth stamps the Host
	// commune onto the principal, so it compares a value with itself. The comparison that decides
	// lives in identity (service-identity/internal/grpc/server.go:257), where the commune sent in
	// `x-tenant-id` meets the commune INSIDE the credential. This service cannot reach that line;
	// what it CAN prove is that it sent the right commune to be compared against. Drop that and
	// the far side compares the wrong pair, and nothing here would have noticed.
	p.xaDaGui, p.coXa = tenant.From(ctx)
	return p.tra, p.co, p.loi
}

// khoPhieuCongDan is the identity-filtered read behind the citizen route.
//
// IT ASSERTS ITS OWN PRECONDITIONS rather than returning a fixture blindly: the commune must be in
// the context (which only httpx.XaTuPhien can put there on this chain) and the citizen identifier
// must be the one the session carried. A stand-in that ignored both would let the citizen chain be
// replaced by the staff chain with every test still green.
type khoPhieuCongDan struct{}

func (khoPhieuCongDan) CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (
	domain.PhieuPhanAnh, error) {

	xa := tenant.MustFrom(ctx)
	if congDanID != idCongDan || xa != xaA || ma != maPhieuCuaToi {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return domain.PhieuPhanAnh{
		MaTraCuu: maPhieuCuaToi, Kenh: domain.KenhZaloMiniApp,
		CongDanID: idCongDan, NoiDung: "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		TrangThai: domain.DaTiepNhan, GocDemHan: mocGui, VaoSoLuc: mocGui,
	}, nil
}

// guiPhieuGia is the citizen INTAKE use case, and like khoPhieuCongDan it asserts its own
// preconditions rather than returning a fixture blindly.
//
// THE COMMUNE AND THE ACTOR ARE BOTH CHECKED, because both are things only the citizen chain can
// have put in place: the commune comes from httpx.XaTuPhien and the actor from
// authz.CitizenPrincipal. A stand-in that ignored them would let this route be moved onto the staff
// chain — where it would answer 404 to every citizen — with every test in this file still green.
type guiPhieuGia struct{ goi int }

func (g *guiPhieuGia) Gui(ctx context.Context, yc app.YeuCauGuiPhanAnh, congDan audit.Actor) (
	domain.PhieuPhanAnh, error) {

	g.goi++
	xa := tenant.MustFrom(ctx)
	if xa != xaA || congDan.ID != idCongDan || congDan.Kind != "citizen" {
		return domain.PhieuPhanAnh{}, errors.New("gửi phiếu giả: xã hoặc chủ thể không phải của phiên công dân")
	}
	return domain.PhieuPhanAnh{
		MaTraCuu: maPhieuCuaToi, Kenh: domain.KenhZaloMiniApp,
		CongDanID: congDan.ID, NoiDung: yc.NoiDung,
		TrangThai: domain.DaTiepNhan, GocDemHan: mocGui, VaoSoLuc: mocGui,
		HanTiepNhan: mocGui.Add(0),
	}, nil
}

// soPhienGia is the citizen session registry the edge asks on every citizen request.
//
// ONE USABLE TOKEN AND NOTHING ELSE. Every other string — unknown, expired, revoked — comes back
// ok=false, which is the real contract: httpx.CitizenSessions has ONE negative answer on purpose,
// so a probe cannot learn how close it got (core/httpx/citizen.go).
type soPhienGia struct{ goi int }

func (s *soPhienGia) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool) {
	s.goi++
	if token != tokenCongDan {
		return httpx.CitizenSession{}, false
	}
	return httpx.CitizenSession{ID: "sid-cong-dan", CitizenID: idCongDan, TenantID: xaA}, true
}

type mayChu struct {
	h   http.Handler
	kho *khoGia
	pg  *phanGiaiGia
	so  *soPhienGia
	gui *guiPhieuGia
}

func dungMayChu(t *testing.T, pg *phanGiaiGia) *mayChu {
	t.Helper()

	kho := &khoGia{theo: map[tenant.ID][]domain.LoaiNhiemVu{
		xaA: {{ID: "lnv-001", Ma: "theo-van-ban", Nhan: nhanXaA, DangDung: true, LaMacDinh: true}},
		xaB: {{ID: "lnv-b-001", Ma: "theo-van-ban", Nhan: nhanXaB, DangDung: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY main() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:     staffauth.Checker{},
		LoaiNhiemVu: kho,
		// The second catalogue this service mounts. It is wired because Register refuses
		// incomplete Deps at construction, and it is deliberately NOT read by any assertion below:
		// one route is enough to prove the chain, and two would only be two copies of one test.
		MucUuTien: khoUuTien{},
		// The two catalogue write use cases, built on a nil *store.DB. NOTHING IN THIS FILE CALLS
		// THEM: this test is about the edge chain — Host -> commune -> principal — and it asserts
		// on a READ route. Register refuses a nil dependency at construction, so both have to be
		// present, and a use case that is never invoked cannot dereference the nil handle.
		GhiLoaiNhiemVu: app.NewDanhMucLoaiNhiemVu(nil, nil),
		GhiMucUuTien:   app.NewDanhMucMucUuTien(nil, nil),
		// Task-status wording: never invoked here (Register refuses nil). Own suite:
		// internal/http/trang_thai_nhiem_vu_test.go.
		TrangThaiNhiemVu:    petstore.NewNhanTrangThaiNhiemVuStore(nil),
		GhiTrangThaiNhiemVu: app.NewNhanTrangThaiNhiemVu(nil, nil),
		Phieu:               khoPhieu{},
		NhanLinhVuc:         khoNhanLinhVuc{},
		Vet:                 vetGia{},
		// The register list and the four staff acts, built on a nil *store.DB for the same reason as
		// the two catalogue writers above: this file is about the EDGE CHAIN, it asserts on a read
		// route, and Register refuses a nil dependency at construction. A use case that is never
		// invoked cannot dereference the nil handle. Their own four-case suites live in
		// internal/http/xu_ly_phan_anh_test.go.
		DanhSachPhieu:   khoPhieu{},
		XuLyPhieu:       app.NewXuLyPhanAnh(nil, nil, nil, nil),
		NhiemVu:         khoNhiemVu{},
		DanhSachNhiemVu: khoNhiemVu{},
		// The six task WRITE acts, built on a nil *store.DB for the same reason as every use case
		// above: this file is about the EDGE CHAIN, it asserts on a read route, and Register refuses
		// a nil dependency at construction. A use case that is never invoked cannot dereference the
		// nil handle. Its own four-case suite lives in internal/http/nhiem_vu_ghi_test.go.
		GhiNhiemVu:      app.NewGhiNhiemVu(nil, nil, nil),
		DanhSachBienBan: khoBienBan{},
		// The three meeting-register WRITE acts, on a nil *store.DB for the same reason: never
		// invoked here, and Register refuses a nil dependency at construction. Its own four-case
		// suite lives in internal/http/bien_ban_hop_ghi_test.go.
		GhiBienBan: app.NewGhiBienBanHop(nil, nil, nil),
		Log:        log,
	})

	// THE CITIZEN SURFACE, REGISTERED THE WAY main() REGISTERS IT — its own mux, its own Deps.
	gui := &guiPhieuGia{}
	muxCongDan := http.NewServeMux()
	svchttp.RegisterCongDan(muxCongDan, svchttp.DepsCongDan{
		Phieu:       khoPhieuCongDan{},
		GuiPhieu:    gui,
		NhanLinhVuc: khoNhanLinhVuc{},
		Log:         log,
	})

	danhBa := thuMucGia{
		hostA: {ID: xaA, Host: hostA, Active: true},
		hostB: {ID: xaB, Host: hostB, Active: true},
	}
	so := &soPhienGia{}
	return &mayChu{
		h:   dungBien(mux, muxCongDan, so, danhBa, pg, nil, log),
		kho: kho, pg: pg, so: so, gui: gui,
	}
}

// goiCongDan issues a request to the CITIZEN surface, the way the Mini App does.
//
// THE HOST IS DELIBERATELY ONE THE DIRECTORY DOES NOT KNOW. That is not a shortcut — it is the
// production shape: the Mini App calls ONE API host that corresponds to no commune (ADR 0005), so
// a citizen request reaching httpx.TenantMiddleware is answered 404. Using a commune host here
// would let the split be deleted and every assertion still pass.
//
// THE TOKEN GOES IN `Authorization`, NEVER A COOKIE — one host serving every commune means a
// cookie there is sent with every commune's traffic (rule 1, forbidden #3).
func (m *mayChu) goiCongDan(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+path, nil)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func (m *mayChu) goi(t *testing.T, host, path, phieu string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if phieu != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: phieu})
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

func canBoXaA() *phanGiaiGia {
	return &phanGiaiGia{co: true, tra: staffauth.StaffPrincipal{
		StaffID:        idCanBo,
		PermissionKeys: []authz.Perm{"task.read"},
	}}
}

func TestCanBoXaAO_HostXaA_DuocPhucVu(t *testing.T) {
	// FIRST TIME A ROUTE IN THIS SERVICE SERVES A REQUEST. Until this chain existed, the route
	// answered 401 to everybody — including a member of staff holding a perfectly valid session —
	// because nothing in the binary built an authz.Principal.
	m := dungMayChu(t, canBoXaA())

	w := m.goi(t, hostA, tuyen, phieuGia)
	doiMa(t, w, http.StatusOK)

	// The commune's OWN catalogue, read with the commune from the context — never commune B's.
	if !strings.Contains(w.Body.String(), nhanXaA) || strings.Contains(w.Body.String(), nhanXaB) {
		t.Errorf("thân trả về không phải danh mục của xã A: %s", w.Body.String())
	}
	if m.kho.doc != 1 || m.pg.goi != 1 {
		t.Errorf("đọc kho %d lần, gọi định danh %d lần — muốn 1 và 1", m.kho.doc, m.pg.goi)
	}
}

func TestCungGiayToOHostXaB_KhongCoChuThe_VaKhoKhongBiChamToi(t *testing.T) {
	// The same credential at commune B's host. The RPC makes the comparison — the commune in
	// "x-tenant-id" against the commune inside the credential — and answers OK with NO PRINCIPAL,
	// which is what this fake reproduces. The route's own guard then refuses.
	//
	// THE STORE MUST NOT BE TOUCHED: a query is where a commune boundary is actually crossed.
	m := dungMayChu(t, &phanGiaiGia{co: false})

	doiMa(t, m.goi(t, hostB, tuyen, phieuGia), http.StatusUnauthorized)
	if m.kho.doc != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.kho.doc)
	}
}

func TestKhongCoCookieThiKhongGoiDinhDanh(t *testing.T) {
	// The count is the assertion. A chain that called anyway would answer this request correctly
	// and pay a LAN round trip on every anonymous hit, forever, with nothing reporting it.
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, hostA, tuyen, ""), http.StatusUnauthorized)
	if m.pg.goi != 0 || m.kho.doc != 0 {
		t.Errorf("không có cookie mà vẫn gọi định danh %d lần, đọc kho %d lần", m.pg.goi, m.kho.doc)
	}
}

func TestDinhDanhChetThiTra503ChuKhongPhai401(t *testing.T) {
	// If identity is down and this chain answered "no principal", every member of staff would be
	// told to sign in again — through the service that is down.
	m := dungMayChu(t, &phanGiaiGia{loi: errors.New("rpc error: code = Unavailable desc = refused")})

	w := m.goi(t, hostA, tuyen, phieuGia)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("trả 401 khi dịch vụ định danh chết")
	}
	doiMa(t, w, http.StatusServiceUnavailable)
	if m.kho.doc != 0 {
		t.Errorf("kho bị đọc %d lần trong lúc không xác thực được", m.kho.doc)
	}
}

func TestHostKhongPhanGiaiDuocTra404VaKhongGoiAi(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, "khong-co-xa.example.gov.vn", tuyen, phieuGia), http.StatusNotFound)
	if m.pg.goi != 0 || m.kho.doc != 0 {
		t.Errorf("Host không phân giải được vẫn gọi định danh %d lần và đọc kho %d lần", m.pg.goi, m.kho.doc)
	}
}

func TestHealthzNamNgoaiChuoiXa(t *testing.T) {
	// It answers whether this process is alive, which is true or false regardless of which commune
	// is asking. Behind Host resolution it would fail whenever the platform service does, and an
	// orchestrator would restart a healthy process during somebody else's outage.
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, "khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
	if m.pg.goi != 0 {
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
func TestLoiGoiPhanGiaiMangXaCuaHost(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	// Xã A ở host A: lời gọi phải mang xã A.
	doiMa(t, m.goi(t, hostA, tuyen, phieuGia), http.StatusOK)
	if !m.pg.coXa {
		t.Fatal("lời gọi phân giải KHÔNG mang xã nào — bên kia sẽ so với một giá trị rỗng")
	}
	if m.pg.xaDaGui != xaA {
		t.Errorf("xã đã gửi = %q, muốn %q (xã suy từ Host)", m.pg.xaDaGui, xaA)
	}

	// Cùng phiếu ấy ở host B: xã gửi đi phải ĐỔI THEO HOST, không theo phiếu. Đây là nửa bắt được
	// một bản cài đặt lấy xã từ phản hồi RPC thay vì từ Host.
	m2 := dungMayChu(t, canBoXaA())
	m2.goi(t, hostB, tuyen, phieuGia)
	if m2.pg.xaDaGui != xaB {
		t.Errorf("ở host B, xã đã gửi = %q, muốn %q", m2.pg.xaDaGui, xaB)
	}
}

// --- RÌA CÔNG DÂN — chuỗi thứ hai, tách ở mux NGOÀI theo tiền tố đường dẫn (chốt 22/09/2026) ----
//
// Điều bốn ca dưới đây bảo vệ, mà KHÔNG phép kiểm nào trong internal/http thấy được: tuyến công
// dân nằm sau ĐÚNG chuỗi rìa. internal/http gắn principal thẳng vào context, nên nó chứng minh
// handler làm gì với một danh tính — không chứng minh được danh tính ấy từ đâu ra, và cũng không
// thấy được rằng một tuyến công dân mắc nhầm vào mux cán bộ sẽ trả 404 cho mọi công dân.

// TestTuyenCongDanKhongDiQuaPhanGiaiHost is THE test the split exists for.
//
// The host is one the directory does not know — the real Mini App API host maps to no commune
// (ADR 0005). On the staff chain that is a 404 from httpx.TenantMiddleware. Answering 200 here
// proves the request never reached it, which is the whole point of the second chain.
//
// DELETE `ngoai.Handle(tienToCongDan, c)` FROM dungBien AND THIS TURNS RED WITH 404 — the exact
// failure a citizen would see, and the one that reads as "no such petition".
func TestTuyenCongDanKhongDiQuaPhanGiaiHost(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	w := m.goiCongDan(t, tienToCongDan+maPhieuCuaToi, tokenCongDan)
	doiMa(t, w, http.StatusOK)

	// The staff identity service is NEVER consulted on a citizen request: citizens have no roles
	// and no permissions (rule 5, invariant 6). A call here would mean the staff chain ran.
	if m.pg.goi != 0 {
		t.Errorf("tuyến công dân gọi tới phân giải CÁN BỘ %d lần — chuỗi cán bộ đã chạy", m.pg.goi)
	}
	// The citizen session registry IS consulted, exactly once: the edge looks it up once and both
	// axes read that one answer (core/authz.CitizenPrincipal).
	if m.so.goi != 1 {
		t.Errorf("tra sổ phiên công dân %d lần, muốn đúng 1", m.so.goi)
	}
}

// TestCungHostAyThiTuyenCANBOVAN404 is the other half, and without it the test above proves only
// that something answered — not that the SPLIT is what answered. If the citizen chain had simply
// replaced the staff one, this would come back 200 and nobody would notice until a commune's
// staff screen went blank.
func TestCungHostAyThiTuyenCANBOVAN404(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, hostMiniApp, tuyen, phieuGia), http.StatusNotFound)
	if m.kho.doc != 0 {
		t.Error("kho danh mục bị chạm tới dù Host không phân giải được thành xã nào")
	}
}

// TestTuyenCongDanKhongCoPhienLa401 — the 401 case AT THE CHAIN LEVEL.
//
// No bearer token at all, a token the registry does not know, and a session with no commune chosen
// yet are ONE answer on purpose (ADR 0022): telling them apart tells somebody probing how far they
// got. The third is the one that looks like an oversight and is not — a citizen signed in but with
// no commune is an ordinary state of the Mini App (ADR 0005), and every BUSINESS route refuses it.
func TestTuyenCongDanKhongCoPhienLa401(t *testing.T) {
	for ten, token := range map[string]string{
		"không có Bearer":           "",
		"token sổ phiên không nhận": "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t, canBoXaA())
			doiMa(t, m.goiCongDan(t, tienToCongDan+maPhieuCuaToi, token), http.StatusUnauthorized)
		})
	}
}

// TestTienToCongDanKhopVoiTuyenDaDangKy pins the two spellings of one path together.
//
// dungBien splits on `tienToCongDan`; RegisterCongDan mounts a pattern inside it. Two independent
// strings that must agree is precisely the shape that drifts — and the failure is silent: the
// prefix keeps routing, the pattern inside stops matching, and every citizen gets 404 from a mux
// that is working exactly as told.
func TestTienToCongDanKhopVoiTuyenDaDangKy(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	// Inside the prefix and matching the route -> served.
	doiMa(t, m.goiCongDan(t, tienToCongDan+maPhieuCuaToi, tokenCongDan), http.StatusOK)

	// Inside the prefix and matching NO route -> 404 from the citizen mux, NOT from Host
	// resolution. Either way it is 404, so this case cannot distinguish them on its own; what it
	// does prove is that the prefix does not swallow requests into a handler that answers anyway.
	doiMa(t, m.goiCongDan(t, tienToCongDan, tokenCongDan), http.StatusNotFound)
}

// --- the CITIZEN INTAKE reaches the citizen chain, and never the staff one ---------------------

// goiGuiCongDan posts one petition the way the Mini App does. The host is deliberately one the
// directory does not know — see goiCongDan.
func (m *mayChu) goiGuiCongDan(t *testing.T, path, token, khoa string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+hostMiniApp+path,
		strings.NewReader(`{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn."}`))
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if khoa != "" {
		r.Header.Set(idem.Header, khoa)
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// TestTuyenGuiPhanAnhKhongBiCHUYENHUONG is the trap this whole pair of patterns exists for, and it
// is invisible in any 404.
//
// `tienToCongDan` ends in `/`, so it matches the SUBTREE and not the collection. Registering it
// alone leaves Go's ServeMux to answer a bare `/api/v1/my-citizen-reports` with a redirect to the
// slash form — which matches no route at all.
//
// ĐỘT BIẾN ĐÃ CHẠY, và số đo ở đây là số đo thật chứ không phải phỏng đoán: xoá dòng
// `ngoai.Handle(tapCongDan, c)` khỏi dungBien thì lượt gửi trả **307** kèm
// `Location: /api/v1/my-citizen-reports/`, và đi theo Location ấy ra `404 page not found` dạng văn
// bản thuần. 307 GIỮ NGUYÊN method và thân, nên client đúng chuẩn thật sự gửi lại phản ánh — chỉ là
// gửi vào một cánh cửa không tồn tại. Bốn ca trong tệp này ĐỎ cùng lúc.
func TestTuyenGuiPhanAnhKhongBiChuyenHuong(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	w := m.goiGuiCongDan(t, tapCongDan, tokenCongDan, "01JKHOACHONGTRUNGCUAKHACH")

	// EVERY REDIRECT CODE, not just the one measured. The point is that a submission must be SERVED
	// here; which flavour of redirect a future Go version picks is not something this assertion
	// should depend on.
	if w.Code >= 300 && w.Code < 400 {
		t.Fatalf("POST tới tập bị CHUYỂN HƯỚNG %d tới %q — đích ấy không khớp tuyến nào và trả "+
			"404, nên phản ánh của người dân biến mất", w.Code, w.Header().Get("Location"))
	}
	doiMa(t, w, http.StatusCreated)
	if m.gui.goi != 1 {
		t.Errorf("use case tiếp nhận chạy %d lần, muốn 1", m.gui.goi)
	}
}

// TestTuyenGuiPhanAnhKhongDiQuaPhanGiaiHost is the same assertion the read route makes, for the
// write: the Mini App host resolves to NO commune, so a request that reached the staff chain would
// be answered 404 by httpx.TenantMiddleware — and the directory would have been asked.
//
// ĐỘT BIẾN: đổi `ngoai.Handle(tapCongDan, c)` thành `ngoai.Handle(tapCongDan, h)` và ca này ĐỎ.
func TestTuyenGuiPhanAnhKhongDiQuaPhanGiaiHost(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goiGuiCongDan(t, tapCongDan, tokenCongDan, "01JKHOACHONGTRUNGCUAKHACH"),
		http.StatusCreated)

	// THE DIRECTORY WAS NEVER ASKED. A 201 alone cannot prove the request avoided Host resolution;
	// this can.
	if m.pg.goi != 0 {
		t.Errorf("tuyến gửi phản ánh gọi phân giải cán bộ %d lần — nó đang chạy trên chuỗi cán bộ",
			m.pg.goi)
	}
}

// TestTuyenGuiPhanAnhKhongCoPhienLa401 — the write surface refuses the same three situations the
// read surface does, and the use case is not reached.
func TestTuyenGuiPhanAnhKhongCoPhienLa401(t *testing.T) {
	for ten, token := range map[string]string{
		"không có Bearer":           "",
		"token sổ phiên không nhận": "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t, canBoXaA())
			doiMa(t, m.goiGuiCongDan(t, tapCongDan, token, "01JKHOACHONGTRUNGCUAKHACH"),
				http.StatusUnauthorized)
			if m.gui.goi != 0 {
				t.Errorf("use case tiếp nhận chạy %d lần dù không có phiên dùng được", m.gui.goi)
			}
		})
	}
}

// TestTuyenCANBOKhongPostDuocVaoTapCongDan is the mirror image, and it is what makes the split a
// SPLIT rather than a second door. A staff request arriving at a commune host must not reach the
// citizen intake: `my-citizen-reports` is a resource of its own, and Go's ServeMux matches path
// ELEMENTS, so `citizen-reports` cannot be captured by it either.
func TestTuyenCanBoKhongPostDuocVaoTapCongDan(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	// A staff host, a staff cookie, and the CITIZEN collection path. It lands on the citizen chain
	// by path — where there is no citizen session — and is refused 401. What must NOT happen is a
	// staff principal reaching the intake.
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+tapCongDan,
		strings.NewReader(`{"content":"x"}`))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: "phien-cua-can-bo"})
	r.Header.Set(idem.Header, "01JKHOACHONGTRUNGCUAKHACH")
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	doiMa(t, w, http.StatusUnauthorized)
	if m.gui.goi != 0 {
		t.Errorf("use case tiếp nhận của công dân chạy %d lần cho một yêu cầu của CÁN BỘ", m.gui.goi)
	}
}
