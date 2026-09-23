package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403
// right permission WRONG COMMUNE · 2xx both correct — for ALL SIX Mini App content routes, plus the
// refusals and the contract details that are specific to this module.
//
// THE ROUTES ARE MOUNTED THROUGH THE REAL Register, BEHIND THE REAL EDGE CHAIN in the real order. A
// test mux would prove that a handler works and nothing about the declaration this service actually
// ships — and the declaration is where rule 5's defects live: an endpoint with no permission is
// callable by every staff role, and nothing turns red.
//
// THE "WRONG PERMISSION" CASE GRANTS THE OTHER KEY OF THIS SAME MODULE. `content.read` against a
// write route and `content.update` against a read route is a sharper test than an unrelated key:
// §10.5 divides this screen into exactly those two rights, and a route that accepted either would
// let anybody who may LOOK at the commune's news also PUBLISH it.

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
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	duongNoiDung   = "/api/v1/content-items"
	duongDanhMucND = "/api/v1/content-categories"
)

// --- fakes ---------------------------------------------------------------------------------

// soNoiDungGia is the content READ half. It records the commune it was asked in, which is the one
// thing a handler test can assert about isolation: the store binds `tenant_id` from the context, so
// what is provable here is that the context reaching it carries the commune of the Host.
type soNoiDungGia struct {
	goiDanhSach, goiTheoID int
	xa                     tenant.ID
	locCuoi                commsstore.LocNoiDung
	ycCuoi                 page.Request
	ra                     page.Result[domain.NoiDungMiniApp]
	mot                    domain.NoiDungMiniApp
	loi                    error
}

func (s *soNoiDungGia) DanhSach(ctx context.Context, loc commsstore.LocNoiDung, yc page.Request) (
	page.Result[domain.NoiDungMiniApp], error) {
	s.goiDanhSach++
	s.xa = tenant.MustFrom(ctx)
	s.locCuoi = loc
	s.ycCuoi = yc
	if s.loi != nil {
		return page.NewResult[domain.NoiDungMiniApp](), s.loi
	}
	return s.ra, nil
}

func (s *soNoiDungGia) TheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error) {
	s.goiTheoID++
	s.xa = tenant.MustFrom(ctx)
	if s.loi != nil {
		return domain.NoiDungMiniApp{}, s.loi
	}
	return s.mot, nil
}

func (s *soNoiDungGia) daGoi() bool { return s.goiDanhSach+s.goiTheoID > 0 }

// ghiNoiDungGia is the content WRITE half. It records the request AND THE ACTOR, because the actor
// is what rule 6, invariant 8 is about: the trail must carry `CB-…` and never an internal id, and
// the only place a handler can get that wrong is here.
type ghiNoiDungGia struct {
	themGoi, suaGoi int
	xa              tenant.ID
	nguoi           audit.Actor
	themCuoi        domain.YeuCauThemNoiDung
	suaCuoi         domain.YeuCauSuaNoiDung
	idCuoi          string
	ra              domain.NoiDungMiniApp
	loi             error
}

func (g *ghiNoiDungGia) Them(ctx context.Context, yc domain.YeuCauThemNoiDung, nguoi audit.Actor) (
	domain.NoiDungMiniApp, error) {
	g.themGoi++
	g.xa = tenant.MustFrom(ctx)
	g.themCuoi = yc
	g.nguoi = nguoi
	if g.loi != nil {
		return domain.NoiDungMiniApp{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiNoiDungGia) Sua(ctx context.Context, id string, yc domain.YeuCauSuaNoiDung,
	nguoi audit.Actor) (domain.NoiDungMiniApp, error) {
	g.suaGoi++
	g.xa = tenant.MustFrom(ctx)
	g.idCuoi = id
	g.suaCuoi = yc
	g.nguoi = nguoi
	if g.loi != nil {
		return domain.NoiDungMiniApp{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiNoiDungGia) tongGoi() int { return g.themGoi + g.suaGoi }

// soDanhMucNDGia is the category READ half.
type soDanhMucNDGia struct {
	goi int
	xa  tenant.ID
	ra  []domain.DanhMucMiniApp
	loi error
}

func (s *soDanhMucNDGia) DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error) {
	s.goi++
	s.xa = tenant.MustFrom(ctx)
	if s.loi != nil {
		return nil, s.loi
	}
	return s.ra, nil
}

// ghiDanhMucNDGia is the category WRITE half.
type ghiDanhMucNDGia struct {
	goi    int
	xa     tenant.ID
	nguoi  audit.Actor
	ycCuoi domain.YeuCauThemDanhMuc
	ra     domain.DanhMucMiniApp
	loi    error
}

func (g *ghiDanhMucNDGia) Them(ctx context.Context, yc domain.YeuCauThemDanhMuc, nguoi audit.Actor) (
	domain.DanhMucMiniApp, error) {
	g.goi++
	g.xa = tenant.MustFrom(ctx)
	g.ycCuoi = yc
	g.nguoi = nguoi
	if g.loi != nil {
		return domain.DanhMucMiniApp{}, g.loi
	}
	return g.ra, nil
}

// --- harness -------------------------------------------------------------------------------

type mayChuND struct {
	h       http.Handler
	so      *soNoiDungGia
	ghi     *ghiNoiDungGia
	soDM    *soDanhMucNDGia
	ghiDM   *ghiDanhMucNDGia
	checker *checkerDanhMucGia
}

func dungMayChuND(t *testing.T) *mayChuND {
	t.Helper()

	so := &soNoiDungGia{ra: page.NewResult[domain.NoiDungMiniApp]()}
	ghi := &ghiNoiDungGia{ra: noiDungMau()}
	soDM := &soDanhMucNDGia{}
	ghiDM := &ghiDanhMucNDGia{ra: domain.DanhMucMiniApp{
		ID: "dm-moi", Ten: "Chuyển đổi số", Slug: "chuyen-doi-so", ThuTu: 2, TaoLuc: lucMauND,
	}}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:           checker,
		LoaiTaiNguyen:     danhMucMau(),
		GhiLoaiTaiNguyen:  &ghiDanhMucGia{},
		ThongBao:          &soThongBaoGia{},
		GhiThongBao:       &ghiThongBaoGia{},
		NoiDung:           so,
		GhiNoiDung:        ghi,
		DanhMucNoiDung:    soDM,
		GhiDanhMucNoiDung: ghiDM,
		Log:               im,
	})

	// The real edge chain in the real order. idem.Middleware sits INSIDE TenantMiddleware because the
	// idempotency key is prefixed with the commune (rule 1, invariant 7) — outside it, two communes
	// sending the same key would share one key space.
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuND{h: h, so: so, ghi: ghi, soDM: soDM, ghiDM: ghiDM, checker: checker}
}

func (m *mayChuND) capQuyen(xa tenant.ID, perm ...authz.Perm) {
	if m.checker.co == nil {
		m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.co[xa] == nil {
		m.checker.co[xa] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.co[xa][p] = struct{}{}
	}
}

// goi sends one request. It ALWAYS attaches an Idempotency-Key on a POST: both POST routes declare
// idem.Required, which refuses a request without the header BEFORE it reaches the handler, so a
// harness that omitted it would turn every POST assertion into an assertion about the header.
func (m *mayChuND) goi(t *testing.T, method, host, duong, than string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+duong, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if method == http.MethodPost || method == http.MethodPatch {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JNOIDUNGKEY0000000000000")
	}
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

var lucMauND = time.Date(2026, 9, 14, 8, 9, 0, 0, time.UTC)

func noiDungMau() domain.NoiDungMiniApp {
	return domain.NoiDungMiniApp{
		ID: "01JNOIDUNGMOI00000000000", Loai: domain.LoaiTinTuc,
		DanhMucID: "dm-001", TieuDe: "Xã Thăng Bình khai giảng năm học mới",
		TomTat: "Sáng nay…", NoiDung: "<p>Toàn văn</p>",
		AnhDaiDienURL: "https://x/a.png",
		NgayDang:      time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
		LuotXem:       0, TrangThai: domain.TrangThaiDangHien,
		Nguon: domain.NguonDongBoCong, NguonURL: "https://cong/a", NguonIDNgoai: "cong-42",
		DaSuaTay: true, NguoiTaoMa: "CB-2026-7K3M9Q",
		TaoLuc: lucMauND, CapNhatLuc: lucMauND,
	}
}

const (
	thanThemNoiDungHopLe = `{"type":"tin-tuc","title":"Xã Thăng Bình khai giảng năm học mới","summary":"Sáng nay…","publish":true}`
	thanSuaNoiDungHopLe  = `{"title":"Tiêu đề đã sửa"}`
	thanThemDanhMucHopLe = `{"name":"Chuyển đổi số","slug":"chuyen-doi-so","order":2}`
)

// --- rule 5, invariant 7: the four cases, on all six routes ----------------------------------

// tuyenND is one route under the permission suite.
//
// TABLE-DRIVEN, AND THE TABLE IS THE POINT: six routes times four cases is twenty-four assertions,
// and twenty-four hand-written functions is where one of them quietly stops asserting the commune
// axis. `daGoi` is what turns "the response was 403" into "and the handler was never reached" —
// without it, a route that answered 403 AFTER doing the work would pass.
type tuyenND struct {
	ten    string
	method string
	duong  string
	khoa   authz.Perm // the key this route declares
	khac   authz.Perm // the OTHER key of this module — the "wrong permission" case
	than   string
	maDung int
	daGoi  func(*mayChuND) bool
}

func cacTuyenND() []tuyenND {
	doc, sua := QuyenDocNoiDung, QuyenSuaNoiDung
	return []tuyenND{
		{"đọc sổ nội dung", http.MethodGet, duongNoiDung, doc, sua, "", http.StatusOK,
			func(m *mayChuND) bool { return m.so.goiDanhSach > 0 }},
		{"đọc một mục", http.MethodGet, duongNoiDung + "/nd-001", doc, sua, "", http.StatusOK,
			func(m *mayChuND) bool { return m.so.goiTheoID > 0 }},
		{"thêm nội dung", http.MethodPost, duongNoiDung, sua, doc, thanThemNoiDungHopLe, http.StatusCreated,
			func(m *mayChuND) bool { return m.ghi.themGoi > 0 }},
		{"sửa nội dung", http.MethodPatch, duongNoiDung + "/nd-001", sua, doc, thanSuaNoiDungHopLe, http.StatusOK,
			func(m *mayChuND) bool { return m.ghi.suaGoi > 0 }},
		{"đọc danh mục", http.MethodGet, duongDanhMucND, doc, sua, "", http.StatusOK,
			func(m *mayChuND) bool { return m.soDM.goi > 0 }},
		{"thêm danh mục", http.MethodPost, duongDanhMucND, sua, doc, thanThemDanhMucHopLe, http.StatusCreated,
			func(m *mayChuND) bool { return m.ghiDM.goi > 0 }},
	}
}

func TestNoiDungKhongCoPhienTra401(t *testing.T) {
	for _, tc := range cacTuyenND() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuND(t)
			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, tc.than, nil), http.StatusUnauthorized)
			if tc.daGoi(m) {
				t.Error("không có phiên mà vẫn chạm tới nghiệp vụ")
			}
		})
	}
}

func TestNoiDungSaiQuyenTra403(t *testing.T) {
	for _, tc := range cacTuyenND() {
		t.Run(tc.ten, func(t *testing.T) {
			// THE ACCOUNT HOLDS THE OTHER KEY OF THIS SAME MODULE. §10.5 divides this screen into
			// `content.read` and `content.update`, and a route that accepted either would let anybody
			// who may look at the commune's news also publish it.
			m := dungMayChuND(t)
			m.capQuyen(xaA, tc.khac)
			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, tc.than, canBo(xaA)), http.StatusForbidden)
			if tc.daGoi(m) {
				t.Error("thiếu quyền mà vẫn chạm tới nghiệp vụ")
			}
		})
	}
}

func TestNoiDungDungQuyenNhungXaKhacTra403(t *testing.T) {
	for _, tc := range cacTuyenND() {
		t.Run(tc.ten, func(t *testing.T) {
			// THE CASE THAT IS ONLY EXPRESSIBLE WITH TWO COMMUNES. The permission is granted in
			// commune A; the request arrives at commune B's domain with a principal of commune B.
			// Granting per commune is what stops one commune's administrator from opening another
			// commune's register.
			m := dungMayChuND(t)
			m.capQuyen(xaA, tc.khoa)
			doiMa(t, m.goi(t, tc.method, hostB, tc.duong, tc.than, canBo(xaB)), http.StatusForbidden)
			if tc.daGoi(m) {
				t.Error("quyền cấp ở xã A mà làm được ở xã B")
			}
		})
	}
}

func TestNoiDungDuQuyenTra2xxVaDungXa(t *testing.T) {
	for _, tc := range cacTuyenND() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaB, tc.khoa)
			w := m.goi(t, tc.method, hostB, tc.duong, tc.than, canBo(xaB))
			doiMa(t, w, tc.maDung)
			if !tc.daGoi(m) {
				t.Fatal("đủ quyền mà nghiệp vụ không được gọi")
			}
			// THE COMMUNE COMES FROM Host AND FROM NOWHERE ELSE (rule 1, invariant 3). The store binds
			// it to $1 from the context, so what is provable here is that the context reaching the
			// business layer carries the commune the request arrived at.
			for _, xa := range []tenant.ID{m.so.xa, m.ghi.xa, m.soDM.xa, m.ghiDM.xa} {
				if xa != "" && xa != xaB {
					t.Errorf("nghiệp vụ được gọi với xã %q, muốn %q", xa, xaB)
				}
			}
		})
	}
}

// --- the contract details of this module ------------------------------------------------------

func TestDocSoNoiDungTraMangRongChuKhongPhaiNull(t *testing.T) {
	// `items` MUST BE [] AND NEVER null on a commune that has published nothing — which is every
	// commune today. A client that has to handle both shapes handles one of them wrong.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("danh sách rỗng phải là [] chứ không phải null — thân: %s", w.Body.String())
	}
}

func TestDocSoNoiDungKhongMangToanVanConTuyenChiTietThiCo(t *testing.T) {
	// THE CONTRACT DIFFERENCE BETWEEN THE TWO READS, asserted on the wire rather than assumed. `body`
	// is ABSENT from a list item and PRESENT on the detail: "" would otherwise mean two different
	// things — "this article has no body" and "you asked for a page, which does not carry bodies" —
	// and a client rendering the second as the first shows an empty article with nothing saying so.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	m.so.ra = page.Result[domain.NoiDungMiniApp]{Items: []domain.NoiDungMiniApp{noiDungMau()}}
	m.so.mot = noiDungMau()

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), `"body"`) {
		t.Errorf("trang danh sách không được mang `body`: %s", w.Body.String())
	}

	w = m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	var chiTiet noiDungRa
	if err := json.Unmarshal(w.Body.Bytes(), &chiTiet); err != nil {
		t.Fatalf("thân chi tiết không phải JSON: %v", err)
	}
	if chiTiet.Body == nil || *chiTiet.Body != "<p>Toàn văn</p>" {
		t.Errorf("tuyến chi tiết phải mang toàn văn: %#v", chiTiet.Body)
	}
	// §6's `Tệp đính kèm` column, derived rather than stored.
	if !chiTiet.HasImage {
		t.Error("has_image phải suy ra từ image_url")
	}
	// §10.4's flag is on the wire because it is the only place the screen learns that a correction is
	// now protected from the next synchronisation.
	if !chiTiet.HandEdited || chiTiet.Source != "dong-bo-cong" {
		t.Errorf("xuất xứ không ra tới hợp đồng: source=%q hand_edited=%v",
			chiTiet.Source, chiTiet.HandEdited)
	}
	// A DATE, not a timestamp: an article carried over from the portal was published on a day.
	if chiTiet.PublishedOn != "2026-09-14" {
		t.Errorf("published_on = %q, muốn 2026-09-14", chiTiet.PublishedOn)
	}
}

func TestDocSoNoiDungChuyenBaBoLocXuongKho(t *testing.T) {
	// §6's filter bar reaching the store. A handler that dropped one of the three would show the
	// wrong tab's rows under the right tab's header, with no error anywhere.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)

	w := m.goi(t, http.MethodGet, hostA,
		duongNoiDung+"?type=banner&category=dm-9&q=khai+gi%E1%BA%A3ng", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)

	if m.so.locCuoi.Loai != "banner" || m.so.locCuoi.DanhMucID != "dm-9" ||
		m.so.locCuoi.Tu != "khai giảng" {
		t.Errorf("bộ lọc tới kho = %+v", m.so.locCuoi)
	}
}

func TestDocSoNoiDungLoaiNgoaiSauMaTra400(t *testing.T) {
	// AN UNKNOWN `type` IS REFUSED RATHER THAN PASSED THROUGH. Passed through it matches nothing, and
	// the screen shows an empty tab with no way to tell "this commune has no banners" from "the
	// client sent a code that does not exist".
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"?type=podcast", "", canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.so.goiDanhSach != 0 {
		t.Error("loại lạ mà vẫn chạy truy vấn")
	}
	if loiTra(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", loiTra(t, w).Code)
	}
}

func TestDocSoNoiDungConTroHongTra400(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"?cursor=khong-phai-con-tro", "", canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.so.goiDanhSach != 0 {
		t.Error("con trỏ hỏng mà vẫn chạy truy vấn — page.Parse phải chặn TRƯỚC khi chạm CSDL")
	}
}

func TestDocSoNoiDungSapXepMacDinhLaMoiNhatTruoc(t *testing.T) {
	// The default sort is part of the contract: a client that sends no `sort` must get the newest
	// first, and a silent change of default reorders every screen.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA)), http.StatusOK)

	if m.so.ycCuoi.Column().SQL != "tao_luc" {
		t.Errorf("cột sắp xếp mặc định = %q, muốn tao_luc", m.so.ycCuoi.Column().SQL)
	}
	if m.so.ycCuoi.Dir() != page.Desc {
		t.Errorf("chiều sắp xếp mặc định = %q, muốn desc", m.so.ycCuoi.Dir())
	}
}

func TestThemNoiDungChuTheLaMaCanBoVaThanToiDuocUseCase(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)

	w := m.goi(t, http.MethodPost, hostA, duongNoiDung, thanThemNoiDungHopLe, canBo(xaA))
	doiMa(t, w, http.StatusCreated)

	// RULE 6, INVARIANT 8, AND THIS IS THE ASSERTION THAT WOULD HAVE CAUGHT THE SIX DEFECTS OF
	// 2026-09-22: the actor is the BUSINESS CODE, never `Principal.ID`. A ULID in `actor_id` names
	// nobody to the person reading the trail years later, and the two are indistinguishable on sight.
	if m.ghi.nguoi.ID != "CB-2026-7K3M9Q" {
		t.Errorf("chủ thể vết kiểm toán = %q, muốn mã cán bộ CB-2026-7K3M9Q", m.ghi.nguoi.ID)
	}
	if m.ghi.nguoi.IP == "" {
		t.Error("vết kiểm toán thiếu địa chỉ IP (luật 6, bất biến 2)")
	}
	// The body is DECODED INTO THE REQUEST, not passed through: a handler that dropped `publish`
	// would compose an article nobody can see and nothing else here would notice.
	if m.ghi.themCuoi.Loai != "tin-tuc" || !m.ghi.themCuoi.DangLenMiniApp {
		t.Errorf("thân không tới được use case: %+v", m.ghi.themCuoi)
	}
}

func TestSuaNoiDungPhanBietKhongNhacVoiXoaTrangTrenDayDien(t *testing.T) {
	// THE WHOLE REASON THE PATCH BODY IS A STRUCT OF POINTERS, asserted where a client actually hits
	// it: a screen editing only the title must not clear the summary and unpublish the article.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)

	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001",
		`{"title":"Tiêu đề đã sửa"}`, canBo(xaA)), http.StatusOK)
	if m.ghi.suaCuoi.TieuDe == nil || *m.ghi.suaCuoi.TieuDe != "Tiêu đề đã sửa" {
		t.Errorf("tiêu đề không tới được use case: %#v", m.ghi.suaCuoi.TieuDe)
	}
	for ten, con := range map[string]any{
		"summary":     m.ghi.suaCuoi.TomTat,
		"body":        m.ghi.suaCuoi.NoiDung,
		"category_id": m.ghi.suaCuoi.DanhMucID,
		"image_url":   m.ghi.suaCuoi.AnhDaiDienURL,
		"type":        m.ghi.suaCuoi.Loai,
	} {
		if v, ok := con.(*string); ok && v != nil {
			t.Errorf("trường %q không được nhắc mà vẫn thành con trỏ khác nil: %q", ten, *v)
		}
	}
	if m.ghi.suaCuoi.DangLenMiniApp != nil {
		t.Error("`publish` không được nhắc mà vẫn thành con trỏ khác nil — sẽ gỡ bài khỏi Mini App")
	}
	if m.ghi.idCuoi != "nd-001" {
		t.Errorf("id trên đường dẫn không tới được use case: %q", m.ghi.idCuoi)
	}

	// MENTIONED AND EMPTY IS A REAL REQUEST — `— Chưa xếp danh mục —`.
	m2 := dungMayChuND(t)
	m2.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m2.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001",
		`{"category_id":"","publish":false}`, canBo(xaA)), http.StatusOK)
	if m2.ghi.suaCuoi.DanhMucID == nil || *m2.ghi.suaCuoi.DanhMucID != "" {
		t.Errorf("`category_id: \"\"` mất nghĩa 'bỏ khỏi danh mục': %#v", m2.ghi.suaCuoi.DanhMucID)
	}
	if m2.ghi.suaCuoi.DangLenMiniApp == nil || *m2.ghi.suaCuoi.DangLenMiniApp {
		t.Errorf("`publish: false` mất nghĩa 'gỡ khỏi Mini App': %#v", m2.ghi.suaCuoi.DangLenMiniApp)
	}
}

func TestNoiDungKhongTonTaiTra404(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung, QuyenSuaNoiDung)
	m.so.loi = commsstore.ErrNoiDungKhongTonTai
	m.ghi.loi = commsstore.ErrNoiDungKhongTonTai

	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-cua-xa-khac", "", canBo(xaA)),
		http.StatusNotFound)
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-cua-xa-khac",
		thanSuaNoiDungHopLe, canBo(xaA)), http.StatusNotFound)
}

func TestThemNoiDungDanhMucKhongConTra409(t *testing.T) {
	// 409 AND NOT 400: the body is well-formed and the caller holds the permission. What is refused
	// is this value against the state of the data — usually a screen left open while a colleague
	// retired the category.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghi.loi = commsstore.ErrDanhMucKhongTonTaiMiniApp

	w := m.goi(t, http.MethodPost, hostA, duongNoiDung, thanThemNoiDungHopLe, canBo(xaA))
	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "category_missing" {
		t.Errorf("mã lỗi = %q, muốn category_missing", loiTra(t, w).Code)
	}
}

func TestThemDanhMucSlugTrungTra409VaNoiRoViSao(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghiDM.loi = commsstore.ErrSlugDanhMucDaTonTai

	w := m.goi(t, http.MethodPost, hostA, duongDanhMucND, thanThemDanhMucHopLe, canBo(xaA))
	doiMa(t, w, http.StatusConflict)
	e := loiTra(t, w)
	if e.Code != "code_taken" {
		t.Errorf("mã lỗi = %q, muốn code_taken", e.Code)
	}
	// THE SENTENCE HAS TO SAY WHY a slug that is nowhere on the screen is nonetheless taken: a
	// soft-deleted row keeps its code forever. Without that, this reads as a bug.
	if !strings.Contains(e.Message, "xoá") {
		t.Errorf("thông điệp không giải thích vì sao slug bị chiếm: %q", e.Message)
	}
}

func TestThemDanhMucChaKhongConTra409(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghiDM.loi = app.ErrDanhMucChaKhongTonTai

	w := m.goi(t, http.MethodPost, hostA, duongDanhMucND,
		`{"name":"Mục con","slug":"muc-con","parent_id":"dm-da-xoa"}`, canBo(xaA))
	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "parent_missing" {
		t.Errorf("mã lỗi = %q, muốn parent_missing", loiTra(t, w).Code)
	}
}

func TestNoiDungSaiHinhDangTra400(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghi.loi = domain.ErrURLKhongHopLe

	w := m.goi(t, http.MethodPost, hostA, duongNoiDung,
		`{"type":"tin-tuc","title":"T","image_url":"javascript:alert(1)"}`, canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if loiTra(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", loiTra(t, w).Code)
	}
}

func TestNoiDungLoiKhoKhongLoTieuDeVaToanVan(t *testing.T) {
	// Rule 3, forbidden #3: an article's title and body are free text that a commune's news routinely
	// spends on residents. A store failure must not carry any of it back to the client.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghi.loi = errors.New("pq: duplicate key on tieu_de 'Trao quà cho gia đình ông Nguyễn Văn A'")

	w := m.goi(t, http.MethodPost, hostA, duongNoiDung, thanThemNoiDungHopLe, canBo(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	for _, cam := range []string{"Nguyễn Văn A", "pq:", "tieu_de"} {
		if strings.Contains(w.Body.String(), cam) {
			t.Errorf("lỗi kho lọt ra client (%q): %s", cam, w.Body.String())
		}
	}
}

func TestDocDanhMucNoiDungTraCayPhangVaTruongLaName(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	m.soDM.ra = []domain.DanhMucMiniApp{
		{ID: "dm-goc", Ten: "Danh mục", Slug: "danh-muc", ThuTu: 1, TaoLuc: lucMauND},
		{ID: "dm-001", Ten: "Chuyển đổi số", Slug: "chuyen-doi-so", ChaID: "dm-goc", ThuTu: 2, TaoLuc: lucMauND},
	}

	w := m.goi(t, http.MethodGet, hostA, duongDanhMucND, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)

	var ra danhSachDanhMucRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if len(ra.Items) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2", len(ra.Items))
	}
	// THE FIELD IS `name` AND NOT `label`, and the line is drawn at the SCHEMA: the column is `ten`,
	// a NAME the commune gave a category of its own, which is the `bo_phan.ten` side of that line.
	if !strings.Contains(w.Body.String(), `"name"`) || strings.Contains(w.Body.String(), `"label"`) {
		t.Errorf("hợp đồng danh mục phải trả `name`, không phải `label`: %s", w.Body.String())
	}
	// THE TREE IS FLAT, with `parent_id` on each row: §7 draws a select and §3 draws an indented
	// list, and the two want different shapes of the same rows.
	if ra.Items[1].ParentID != "dm-goc" || ra.Items[0].ParentID != "" {
		t.Errorf("cây phải trả phẳng kèm parent_id: %+v", ra.Items)
	}
}

func TestDocDanhMucVuotTranTra500ChuKhongCatBot(t *testing.T) {
	// REFUSED RATHER THAN TRUNCATED. A silently short tree is a category that has disappeared from
	// §7's select, so articles get filed under the wrong one and the screen looks entirely normal.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	m.soDM.loi = commsstore.ErrQuaNhieuDanhMucMiniApp

	doiMa(t, m.goi(t, http.MethodGet, hostA, duongDanhMucND, "", canBo(xaA)),
		http.StatusInternalServerError)
}

func TestThemNoiDungThieuIdempotencyKeyBiTuChoi(t *testing.T) {
	// BOTH POST ROUTES DECLARE idem.Required, so the header is part of the contract rather than a
	// suggestion. This is the only case in this file that sends a POST WITHOUT it.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)

	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+duongNoiDung,
		strings.NewReader(thanThemNoiDungHopLe))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(authz.Into(r.Context(), *canBo(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	if w.Code == http.StatusCreated {
		t.Fatalf("thiếu Idempotency-Key mà vẫn tạo được: %d", w.Code)
	}
	if m.ghi.tongGoi() != 0 {
		t.Error("thiếu Idempotency-Key mà vẫn chạm tới use case")
	}
}
