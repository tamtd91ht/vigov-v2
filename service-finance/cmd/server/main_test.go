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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	svchttp "github.com/vihat/vigov/service-finance/internal/http"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	idCanBo  = "nd-01JINTERNALIDCUACANBO"
	phieuGia = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	tuyen = "/api/v1/capital-plan-categories"

	nhanXaA = "Xây dựng mới xã A"
	nhanXaB = "Xây dựng mới xã B"
)

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
	theo map[tenant.ID][]domain.HangMucKeHoachVon
	doc  int
}

func (k *khoGia) DanhSach(ctx context.Context) ([]domain.HangMucKeHoachVon, error) {
	k.doc++
	return k.theo[tenant.MustFrom(ctx)], nil
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

type mayChu struct {
	h   http.Handler
	kho *khoGia
	pg  *phanGiaiGia
}

func dungMayChu(t *testing.T, pg *phanGiaiGia) *mayChu {
	t.Helper()

	kho := &khoGia{theo: map[tenant.ID][]domain.HangMucKeHoachVon{
		xaA: {{ID: "hm-001", Ma: "xay-dung-moi", Nhan: nhanXaA, DangDung: true, LaMacDinh: true}},
		xaB: {{ID: "hm-b-001", Ma: "xay-dung-moi", Nhan: nhanXaB, DangDung: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY main() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker: staffauth.Checker{},
		HangMuc: kho,
		// The disbursement routes are wired so that Register mounts THE WHOLE surface this service
		// ships. This file is about the edge chain — Host -> commune -> principal — and it asserts
		// on the capital plan catalogue route; a Deps missing a dependency would make Register
		// refuse to start, which is exactly what it is supposed to do.
		// Built on a nil *store.DB. NOTHING IN THIS FILE CALLS IT: this test is about the edge
		// chain — Host -> commune -> principal — and it asserts on the catalogue READ route.
		// Register refuses a nil dependency at construction, so it has to be present.
		GhiHangMuc: app.NewDanhMucHangMuc(nil, nil),
		DuAn:       khoDuAnTrong{},
		// Same reasoning as GhiHangMuc above: built on a nil *store.DB, never called from this file,
		// present because Register refuses a nil dependency at construction.
		GhiChungTu: app.NewChungTuGiaiNgan(nil, nil),
		Nguong:     nguongTrong{},
		// The budget board, same reasoning again: built on a nil *store.DB and never called from this
		// file, present because Register refuses a nil dependency at construction.
		NganSach:    khoNganSachTrong{},
		GhiNganSach: app.NewNganSach(nil, nil),
		Log:         log,
	})

	danhBa := thuMucGia{
		hostA: {ID: xaA, Host: hostA, Active: true},
		hostB: {ID: xaB, Host: hostB, Active: true},
	}
	return &mayChu{h: dungBien(mux, danhBa, pg, nil, log), kho: kho, pg: pg}
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

// canBoXaA is a member of staff of commune A holding `budget.read`.
//
// THE KEY IS A ROW OF `quyen` AND IT IS THE ONE THIS SERVICE'S ROUTES DECLARE. Until today this
// read `finance.read` — a string that appears in NO migration and in NO route in this repository.
// It made every case below green while proving nothing about permissions: the catalogue route is
// AnyAuthenticated, which never consults the key set at all, so any string whatsoever passed. A
// key absent from `quyen` is a key no administrator can grant on the Phân quyền screen, so a
// route guarded by it could never be reached by anybody (rule 5, invariant 3b).
//
// `budget.read` is loaded by service-identity/migrations/0001_init.sql:283 and declared by
// service-finance/internal/http/routes.go:211 and :229. TestQuyenBudgetReadThatSuDuocKiem below
// is what keeps this line honest: without it, this key would still be decoration.
func canBoXaA() *phanGiaiGia {
	return &phanGiaiGia{co: true, tra: staffauth.StaffPrincipal{
		StaffID:        idCanBo,
		PermissionKeys: []authz.Perm{"budget.read"},
	}}
}

// canBoKhongCoQuyen is a member of staff of commune A who is signed in and holds NOTHING.
//
// This is the account the specification's smallest role produces, and it is the one a 403 has to
// be provable against.
func canBoKhongCoQuyen() *phanGiaiGia {
	return &phanGiaiGia{co: true, tra: staffauth.StaffPrincipal{StaffID: idCanBo}}
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

// --- the permission declaration, exercised through the chain this binary builds ---------------

// TUYẾN DUY NHẤT CỦA DỊCH VỤ NÀY CÓ KHAI `RequirePermission`, ĐI QUA ĐÚNG CHUỖI dungBien DỰNG.
//
// Vì sao ca này phải có: mọi ca khác trong tệp bắn vào tuyến danh mục, và tuyến ấy khai
// AnyAuthenticated — guard đó KHÔNG BAO GIỜ đọc bộ khoá (core/authz/authz.go:187-208). Nên trước
// ca này, `PermissionKeys` trong tệp này là trang trí: đổi nó thành chuỗi rác, mọi ca vẫn xanh.
// Đó chính là cách `finance.read` — một khoá không migration nào gieo — sống sót ở đây.
//
// Ca này buộc bộ khoá phải có nghĩa: staffauth.Checker đọc đúng bộ khoá mà Middleware đặt vào
// context, và authz.RequirePermission so nó với "budget.read".
func TestQuyenBudgetReadThatSuDuocKiem(t *testing.T) {
	const tuyenDuAn = "/api/v1/investment-projects?year=2026"

	// 200 — đúng xã, đúng khoá.
	m := dungMayChu(t, canBoXaA())
	doiMa(t, m.goi(t, hostA, tuyenDuAn, phieuGia), http.StatusOK)

	// 403 — đã đăng nhập, đúng xã, KHÔNG có khoá. Đây là nửa chứng minh khoá được đem ra so thật,
	// chứ không phải guard cho qua mọi tài khoản.
	k := dungMayChu(t, canBoKhongCoQuyen())
	doiMa(t, k.goi(t, hostA, tuyenDuAn, phieuGia), http.StatusForbidden)

	// 401 — không phiếu.
	n := dungMayChu(t, canBoXaA())
	doiMa(t, n.goi(t, hostA, tuyenDuAn, ""), http.StatusUnauthorized)

	// 401 — ĐÚNG KHOÁ, SAI XÃ. Hình dạng một vụ rò chéo xã có thật: quyền hợp lệ, nhưng phiếu cấp
	// cho xã khác. Bên phân giải trả "không có chủ thể" đúng như identity trả khi x-tenant-id
	// không khớp xã nằm trong chứng thực (service-identity/internal/grpc/server.go:257).
	x := dungMayChu(t, &phanGiaiGia{co: false})
	doiMa(t, x.goi(t, hostB, tuyenDuAn, phieuGia), http.StatusUnauthorized)
}

// khoDuAnTrong is a project store holding nothing. No case in this file reads a project: the
// properties under test here are about the edge, not about disbursement figures.
type khoDuAnTrong struct{}

func (khoDuAnTrong) DanhSach(context.Context, fistore.LocDuAn) ([]domain.TienDoDuAn, error) {
	return nil, nil
}

func (khoDuAnTrong) ChiTiet(context.Context, string) (domain.TienDoDuAn, error) {
	return domain.TienDoDuAn{}, fistore.ErrKhongThayDuAn
}

// nguongTrong answers the software's default threshold, which is what every commune is on today —
// `cau_hinh_giai_ngan` has no write path yet. No case in this file reads it; it is here because
// Register refuses to start without one, deliberately (see routes.go).
type nguongTrong struct{}

func (nguongTrong) NguongCanhBaoCham(context.Context, int) (domain.NguongCanhBaoCham, error) {
	return domain.MacDinhCuaPhanMem(), nil
}

// khoNganSachTrong answers "this commune has no such sheet" — the state of every commune today,
// since nothing seeds `bang_ngan_sach`. No case in this file reads it; it is here because Register
// refuses to start without one, deliberately (see routes.go).
type khoNganSachTrong struct{}

func (khoNganSachTrong) BangDayDu(context.Context, int, domain.LoaiBang) (domain.BangDayDu, error) {
	return domain.BangDayDu{}, fistore.ErrKhongThayBangNganSach
}
