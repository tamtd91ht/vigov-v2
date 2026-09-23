package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403
// right permission WRONG COMMUNE · 2xx both correct — for both announcement routes, plus the
// refusals that are specific to this module.
//
// THE ROUTES ARE MOUNTED THROUGH THE REAL Register, BEHIND THE REAL EDGE CHAIN in the real order.
// A test mux would prove that a handler works and nothing about the declaration this service
// actually ships — and the declaration is where rule 5's defects live: an endpoint with no
// permission is callable by every staff role, and nothing turns red.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
)

const duongThongBao = "/api/v1/announcements"

// --- fakes ---------------------------------------------------------------------------------

// soThongBaoGia is the READ half. It records the commune it was asked in, which is the one thing a
// handler test can assert about isolation: the store binds `tenant_id` from the context, so what is
// provable here is that the context reaching it carries the commune of the Host.
type soThongBaoGia struct {
	goi   int
	xa    tenant.ID
	yc    page.Request
	ra    page.Result[domain.ThongBaoNoiBo]
	loi   error
	daGoi bool
}

func (s *soThongBaoGia) DanhSach(ctx context.Context, yc page.Request) (
	page.Result[domain.ThongBaoNoiBo], error) {
	s.goi++
	s.daGoi = true
	s.xa = tenant.MustFrom(ctx)
	s.yc = yc
	if s.loi != nil {
		return page.NewResult[domain.ThongBaoNoiBo](), s.loi
	}
	return s.ra, nil
}

// ghiThongBaoGia is the WRITE half. It records the request AND THE ACTOR, because the actor is what
// rule 6, invariant 8 is about: the trail must carry `CB-…` and never an internal id, and the only
// place a handler can get that wrong is here.
type ghiThongBaoGia struct {
	goi    int
	xa     tenant.ID
	ycCuoi domain.YeuCauSoanThongBao
	nguoi  audit.Actor
	ra     domain.ThongBaoNoiBo
	loi    error
}

func (g *ghiThongBaoGia) PhatHanh(ctx context.Context, yc domain.YeuCauSoanThongBao,
	nguoi audit.Actor) (domain.ThongBaoNoiBo, error) {
	g.goi++
	g.xa = tenant.MustFrom(ctx)
	g.ycCuoi = yc
	g.nguoi = nguoi
	if g.loi != nil {
		return domain.ThongBaoNoiBo{}, g.loi
	}
	return g.ra, nil
}

// khoIdemGia is an in-memory idempotency store.
//
// WHY THE TEST NEEDS ONE AT ALL, when the catalogue's write suite runs with a nil store: the POST
// route below declares idem.Required(DongKhiHong), which answers 503 when no store is configured —
// correctly, and that is the whole point of the declaration. A nil store here would turn every
// assertion in this file into an assertion about Redis being absent.
//
// It is not a Redis emulator: Claim is compare-and-set, Get reads, Complete overwrites, Release
// removes. That is the entire contract idem.Store states, and nothing in these tests exercises a
// TTL.
type khoIdemGia struct {
	mu sync.Mutex
	m  map[string]string
}

func moKhoIdemGia() *khoIdemGia { return &khoIdemGia{m: map[string]string{}} }

func (k *khoIdemGia) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, co := k.m[key]; co {
		return false, nil
	}
	k.m[key] = ""
	return true, nil
}

func (k *khoIdemGia) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[key], nil
}

func (k *khoIdemGia) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = value
	return nil
}

func (k *khoIdemGia) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

// --- harness -------------------------------------------------------------------------------

type mayChuThongBao struct {
	h       http.Handler
	so      *soThongBaoGia
	ghi     *ghiThongBaoGia
	checker *checkerDanhMucGia
}

func dungMayChuThongBao(t *testing.T) *mayChuThongBao {
	t.Helper()

	so := &soThongBaoGia{ra: page.NewResult[domain.ThongBaoNoiBo]()}
	ghi := &ghiThongBaoGia{}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:          checker,
		LoaiTaiNguyen:    danhMucMau(),
		GhiLoaiTaiNguyen: &ghiDanhMucGia{},
		ThongBao:         so,
		GhiThongBao:      ghi,
		// Same again for the four Mini App content dependencies — see noi_dung_mini_app_test.go.
		NoiDung:           &soNoiDungGia{},
		GhiNoiDung:        &ghiNoiDungGia{},
		DanhMucNoiDung:    &soDanhMucNDGia{},
		GhiDanhMucNoiDung: &ghiDanhMucNDGia{},
		Log:               im,
	})

	// The real edge chain in the real order. idem.Middleware sits INSIDE TenantMiddleware because
	// the idempotency key is prefixed with the commune (rule 1, invariant 7) — outside it, two
	// communes sending the same key would share one key space.
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuThongBao{h: h, so: so, ghi: ghi, checker: checker}
}

func (m *mayChuThongBao) capQuyen(xa tenant.ID, perm ...authz.Perm) {
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

// goi sends one request. It ALWAYS attaches an Idempotency-Key on a POST: the route declares
// idem.Required, which refuses a request without the header BEFORE it reaches the handler, so a
// harness that omitted it would turn every POST assertion into an assertion about the header.
func (m *mayChuThongBao) goi(t *testing.T, method, host, than string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+duongThongBao, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JTHONGBAOKEY00000000000")
	}
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// canBo builds a staff principal of one commune. `Ma` IS SET AND IS NOT THE ID: the handler copies
// it into audit.Actor.ID, which is the column an inspection reads years later (rule 6, invariant 8).
func canBo(xa tenant.ID) *authz.Principal {
	return &authz.Principal{ID: "01JCANBONOIBO000000000000", Ma: "CB-2026-7K3M9Q", Kind: "staff", TenantID: xa}
}

// --- rule 5, invariant 7: the four cases, on the READ route ---------------------------------

func TestDocThongBaoKhongCoPhienTra401(t *testing.T) {
	m := dungMayChuThongBao(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "", nil), http.StatusUnauthorized)
	if m.so.daGoi {
		t.Error("không có phiên mà vẫn đọc sổ thông báo")
	}
}

func TestDocThongBaoSaiQuyenTra403(t *testing.T) {
	m := dungMayChuThongBao(t)
	// The account is signed in and holds a DIFFERENT permission of the same service. This is the
	// case a missing declaration would let through, and nothing would report it.
	m.capQuyen(xaA, "admin.lookup")
	doiMa(t, m.goi(t, http.MethodGet, hostA, "", canBo(xaA)), http.StatusForbidden)
	if m.so.daGoi {
		t.Error("thiếu quyền mà vẫn đọc sổ thông báo")
	}
}

func TestDocThongBaoDungQuyenNhungXaKhacTra403(t *testing.T) {
	// THE CASE THAT IS ONLY EXPRESSIBLE WITH TWO COMMUNES. The permission is granted in commune A;
	// the request arrives at commune B's domain with a principal of commune B. Granting per commune
	// is what stops one commune's administrator from opening another commune's book.
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	doiMa(t, m.goi(t, http.MethodGet, hostB, "", canBo(xaB)), http.StatusForbidden)
	if m.so.daGoi {
		t.Error("quyền cấp ở xã A mà đọc được sổ của xã B")
	}
}

func TestDocThongBaoDuQuyenTra200VaDungXa(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaB, QuyenThongBao)
	w := m.goi(t, http.MethodGet, hostB, "", canBo(xaB))
	doiMa(t, w, http.StatusOK)

	if m.so.xa != xaB {
		t.Fatalf("kho được gọi với xã %q, muốn %q — xã phải suy từ Host, không từ yêu cầu", m.so.xa, xaB)
	}
	// `items` MUST BE [] AND NEVER null on a commune that has issued nothing — which is every
	// commune today. A client that has to handle both shapes handles one of them wrong.
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("danh sách rỗng phải là [] chứ không phải null — thân: %s", w.Body.String())
	}
}

// --- rule 5, invariant 7: the four cases, on the WRITE route ---------------------------------

const thanPhatHanhHopLe = `{"title":"Mời họp giao ban tháng 9","body":"Kính mời các đồng chí dự họp.","recipient_codes":["CB-2026-AAAA11"]}`

func TestPhatHanhThongBaoKhongCoPhienTra401(t *testing.T) {
	m := dungMayChuThongBao(t)
	doiMa(t, m.goi(t, http.MethodPost, hostA, thanPhatHanhHopLe, nil), http.StatusUnauthorized)
	if m.ghi.goi != 0 {
		t.Error("không có phiên mà vẫn phát hành thông báo")
	}
}

func TestPhatHanhThongBaoSaiQuyenTra403(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, "admin.lookup")
	doiMa(t, m.goi(t, http.MethodPost, hostA, thanPhatHanhHopLe, canBo(xaA)), http.StatusForbidden)
	if m.ghi.goi != 0 {
		t.Error("thiếu quyền mà vẫn phát hành thông báo")
	}
}

func TestPhatHanhThongBaoDungQuyenNhungXaKhacTra403(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	doiMa(t, m.goi(t, http.MethodPost, hostB, thanPhatHanhHopLe, canBo(xaB)), http.StatusForbidden)
	if m.ghi.goi != 0 {
		t.Error("quyền cấp ở xã A mà phát hành được ở xã B")
	}
}

func TestPhatHanhThongBaoDuQuyenTra201VaChuTheLaMaCanBo(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	m.ghi.ra = domain.ThongBaoNoiBo{
		ID: "01JTHONGBAOMOI0000000000", TieuDe: "Mời họp giao ban tháng 9",
		NoiDung: "Kính mời các đồng chí dự họp.", TrangThai: domain.ThongBaoDaPhatHanh,
		TrangThaiThu: domain.ThuChuaGui, NguoiSoanMa: "CB-2026-7K3M9Q",
		PhatHanhLuc: time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC),
		TaoLuc:      time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC),
		SoNguoiNhan: 1,
	}

	w := m.goi(t, http.MethodPost, hostA, thanPhatHanhHopLe, canBo(xaA))
	doiMa(t, w, http.StatusCreated)

	if m.ghi.xa != xaA {
		t.Fatalf("use case được gọi với xã %q, muốn %q", m.ghi.xa, xaA)
	}
	// RULE 6, INVARIANT 8, AND THIS IS THE ASSERTION THAT WOULD HAVE CAUGHT THE SIX DEFECTS OF
	// 2026-09-22: the actor is the BUSINESS CODE, never `Principal.ID`. A ULID in `actor_id` names
	// nobody to the person reading the trail years later, and the two are indistinguishable on sight.
	if m.ghi.nguoi.ID != "CB-2026-7K3M9Q" {
		t.Errorf("chủ thể vết kiểm toán = %q, muốn mã cán bộ CB-2026-7K3M9Q", m.ghi.nguoi.ID)
	}
	if m.ghi.nguoi.IP == "" {
		t.Error("vết kiểm toán thiếu địa chỉ IP (luật 6, bất biến 2)")
	}
	// The body is DECODED INTO THE REQUEST, not passed through: a handler that dropped
	// `recipient_codes` would issue an announcement to nobody and nothing else here would notice.
	if len(m.ghi.ycCuoi.NguoiNhanMa) != 1 || m.ghi.ycCuoi.NguoiNhanMa[0] != "CB-2026-AAAA11" {
		t.Errorf("người nhận không tới được use case: %#v", m.ghi.ycCuoi.NguoiNhanMa)
	}

	var ra thongBaoRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân trả về không phải JSON: %v", err)
	}
	if ra.Status != "da-phat-hanh" || ra.RecipientCount != 1 {
		t.Errorf("phản hồi = %+v, muốn trạng thái da-phat-hanh và 1 người nhận", ra)
	}
	if ra.IssuedAt == nil {
		t.Error("thông báo đã phát hành phải có issued_at")
	}
	if ra.EmailStatus != "chua-gui" {
		t.Errorf("trạng thái thư = %q, muốn chua-gui — kho này chưa có bộ gửi thư", ra.EmailStatus)
	}
}

// --- the refusals that belong to this module -------------------------------------------------

func TestPhatHanhTheoBoPhanTra501VaKhongGuiChoAi(t *testing.T) {
	// THE REFUSAL THIS PASS EXISTS TO MAKE LOUD. Expanding a department into its staff needs an RPC
	// service-identity does not publish, and the alternative — record the departments, deliver to
	// nobody — produces no error anywhere while the commune is never told.
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	m.ghi.loi = app.ErrGuiTheoBoPhanChuaCo

	w := m.goi(t, http.MethodPost, hostA,
		`{"title":"Mời họp","body":"Nội dung","org_unit_ids":["01JBOPHAN0000000000000000"]}`,
		canBo(xaA))
	doiMa(t, w, http.StatusNotImplemented)

	e := loiTra(t, w)
	if e.Code != "not_implemented" {
		t.Errorf("mã lỗi = %q, muốn not_implemented", e.Code)
	}
	// THE SENTENCE HAS TO NAME THE WORKAROUND. An error a person cannot act on is an error they
	// report to somebody else, and this one has a real answer: name the recipients.
	if !strings.Contains(e.Message, "đích danh") {
		t.Errorf("thông điệp không chỉ ra cách làm được: %q", e.Message)
	}
}

func TestPhatHanhKhongCoNguoiNhanTra400(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	m.ghi.loi = domain.ErrKhongCoNguoiNhan

	w := m.goi(t, http.MethodPost, hostA, `{"title":"Mời họp","body":"Nội dung"}`, canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if loiTra(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", loiTra(t, w).Code)
	}
}

func TestPhatHanhLoiKhoKhongLoNoiDung(t *testing.T) {
	// Rule 3, forbidden #3: an announcement body is free text a colleague typed and can quote a
	// case. A store failure must not carry any of it back to the client.
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	m.ghi.loi = errors.New("pq: duplicate key value violates unique constraint on noi_dung bí mật")

	w := m.goi(t, http.MethodPost, hostA, thanPhatHanhHopLe, canBo(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "bí mật") || strings.Contains(w.Body.String(), "pq:") {
		t.Errorf("lỗi kho lọt ra client: %s", w.Body.String())
	}
}

func TestDocThongBaoConTroHongTra400(t *testing.T) {
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)

	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+duongThongBao+"?cursor=khong-phai-con-tro", nil)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(authz.Into(r.Context(), *canBo(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	doiMa(t, w, http.StatusBadRequest)
	if m.so.daGoi {
		t.Error("con trỏ hỏng mà vẫn chạy truy vấn — page.Parse phải chặn TRƯỚC khi chạm CSDL")
	}
}

func TestDocThongBaoSapXepMacDinhLaMoiNhatTruoc(t *testing.T) {
	// §2: "mới nhất ở trên". The default sort is part of the contract: a client that sends no
	// `sort` must get the newest first, and a silent change of default reorders every screen.
	m := dungMayChuThongBao(t)
	m.capQuyen(xaA, QuyenThongBao)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "", canBo(xaA)), http.StatusOK)

	if m.so.yc.Column().SQL != "tao_luc" {
		t.Errorf("cột sắp xếp mặc định = %q, muốn tao_luc", m.so.yc.Column().SQL)
	}
	if m.so.yc.Dir() != page.Desc {
		t.Errorf("chiều sắp xếp mặc định = %q, muốn desc", m.so.yc.Dir())
	}
}
