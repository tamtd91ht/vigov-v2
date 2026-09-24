package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the five WRITE routes of the staff register. Rule 5, invariant 7 asks
// four cases of every one of them —
//
//	401  no token
//	403  a signed-in account holding the WRONG permission
//	403  the RIGHT permission, in the WRONG COMMUNE
//	2xx  both correct
//
// THE THIRD CASE IS THE ONE THAT PROVES ANYTHING, and it only proves it because checkerGia is
// keyed BY COMMUNE (routes_test.go): the same account holds `admin.user` in commune A and nothing
// in commune B. A flat set of permissions in the fixture would make that case pass while asserting
// nothing at all — the request would be refused for being unauthenticated, or allowed for holding
// the key, and neither answer would be about the commune.
//
// WHAT IS DELIBERATELY NOT HERE: the refusals of open questions #13 and #14. They are decisions of
// the use case, they are proven against a transaction in app/danh_ba_can_bo_test.go, and asserting
// them again through a fake use case here would assert that the fake returns what it was told to.
// What IS asserted here is that each refusal reaches the client as the right status and the right
// code — which is the handler's own job and nobody else's.

// --- the fake use case ---------------------------------------------------------------------

// ghiDanhBaGia stands in for *app.DanhBaCanBo, recording what the handler passed and returning
// whatever the test set.
//
// IT RECORDS THE ACTOR, and that is the point of the `nguoiCuoi` field: the handler is the one
// layer that decides which of a principal's two identifiers reaches the audit trail, and a fake
// that dropped it would let `p.ID` be written there with nothing turning red.
type ghiDanhBaGia struct {
	mu sync.Mutex

	goi        int
	nguoiCuoi  app.NguoiThucHien
	xaCuoi     tenant.ID
	idCuoi     string
	khoaCuoi   bool
	vaiTroCuoi string
	themCuoi   app.YeuCauThemCanBo
	suaCuoi    app.YeuCauSuaCanBo
	congKhai   app.YeuCauCongKhai
	lyDoXoa    string

	kq  domain.CanBoTomTat
	loi error
}

func ghiDanhBaMau() *ghiDanhBaGia {
	return &ghiDanhBaGia{kq: domain.CanBoTomTat{
		ID: idNoiBo, Ma: maCanBo, HoTen: "Nguyễn Văn A", Email: emailDung,
		ChucVu: "Công chức Văn phòng", BoPhanID: "bp-001", VaiTroID: "vt-001",
		DienThoaiCoQuan: "0900000000", DiDongCaNhan: "0900000009",
		CoTaiKhoan: true, DangHoatDong: true,
		TaoLuc: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}
}

func (g *ghiDanhBaGia) ghiNhan(ctx context.Context, nguoi app.NguoiThucHien) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goi++
	g.nguoiCuoi = nguoi
	g.xaCuoi = tenant.MustFrom(ctx)
}

func (g *ghiDanhBaGia) soLanGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.goi
}

func (g *ghiDanhBaGia) Them(ctx context.Context, yc app.YeuCauThemCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error) {
	g.ghiNhan(ctx, nguoi)
	g.themCuoi = yc
	return g.kq, g.loi
}

func (g *ghiDanhBaGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaCanBo, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.suaCuoi = id, yc
	return g.kq, g.loi
}

func (g *ghiDanhBaGia) DatKhoa(ctx context.Context, id string, khoa bool, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.khoaCuoi = id, khoa
	return g.kq, g.loi
}

func (g *ghiDanhBaGia) DoiVaiTro(ctx context.Context, id, vaiTroID string, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.vaiTroCuoi = id, vaiTroID
	return g.kq, g.loi
}

func (g *ghiDanhBaGia) DatCongKhai(ctx context.Context, id string, yc app.YeuCauCongKhai, nguoi app.NguoiThucHien) (domain.CanBoTomTat, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.congKhai = id, yc
	return g.kq, g.loi
}

func (g *ghiDanhBaGia) Xoa(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.lyDoXoa = id, lyDo
	return g.loi
}

// --- the in-memory idempotency store ----------------------------------------------------------

// khoIdemGia is idem.Store in a map.
//
// WHY A REAL STORE RATHER THAN nil, WHICH IS WHAT THE OTHER SERVICES' HARNESSES USE: POST
// /api/v1/staff declares idem.Required(idem.DongKhiHong), and a nil store is an UNREACHABLE store
// — which that declaration answers with 503, by design. With nil here, every assertion about that
// route would be an assertion about a missing cache.
//
// TTLs are ignored: nothing in these tests waits.
type khoIdemGia struct {
	mu  sync.Mutex
	gia map[string]string
}

func khoIdemMau() *khoIdemGia { return &khoIdemGia{gia: map[string]string{}} }

func (k *khoIdemGia) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, co := k.gia[key]; co {
		return false, nil
	}
	k.gia[key] = "1"
	return true, nil
}

func (k *khoIdemGia) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.gia[key], nil
}

func (k *khoIdemGia) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.gia[key] = value
	return nil
}

func (k *khoIdemGia) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.gia, key)
	return nil
}

// --- the routes under test --------------------------------------------------------------------

// tuyenGhi is every write route with a body that satisfies it. ONE TABLE, so a case written once
// cannot be added to one route and forgotten on another — which is exactly how a list route gets a
// permission check and its detail route does not.
type tuyenGhi struct {
	ten    string
	method string
	duong  string
	than   string
	ok     int // the status of the "both correct" case
}

func moiTuyenGhi() []tuyenGhi {
	return []tuyenGhi{
		{"thêm cán bộ", "POST", "/api/v1/staff",
			`{"full_name":"Trần Thị B","email":"canbo.b@example.gov.vn","position":"Công chức","mobile":"0900000002"}`,
			http.StatusCreated},
		{"sửa hồ sơ", "PATCH", "/api/v1/staff/" + idNoiBo,
			`{"position":"Chuyên viên"}`, http.StatusOK},
		{"khoá tài khoản", "POST", "/api/v1/staff/" + idNoiBo + "/lockout", "", http.StatusOK},
		{"mở khoá", "DELETE", "/api/v1/staff/" + idNoiBo + "/lockout", "", http.StatusOK},
		{"đổi vai trò", "PUT", "/api/v1/staff/" + idNoiBo + "/role",
			`{"role_id":"vt-002"}`, http.StatusOK},
	}
}

// goiGhi issues one write request. It always sends an Idempotency-Key: POST /api/v1/staff refuses
// a request without one BEFORE the handler runs, so a harness that omitted it would turn that
// route's assertions into assertions about the header.
//
// THE KEY IS UNIQUE PER CALL, because the store is real: a second request replaying the same key
// would be answered from the first one's result, which is the correct behaviour and not what most
// of these cases are about. The one case that IS about it builds its own key.
var soKhoaIdem int

func (m *mayChu) goiGhi(t *testing.T, tg tuyenGhi, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	soKhoaIdem++
	return m.goiGhiKhoa(t, tg, host, tok, "01JIDEMKEYCUATEST"+string(rune('A'+soKhoaIdem%26))+
		string(rune('A'+(soKhoaIdem/26)%26)))
}

func (m *mayChu) goiGhiKhoa(t *testing.T, tg tuyenGhi, host, tok, khoa string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if tg.than != "" {
		body = strings.NewReader(tg.than)
	}
	r := httptest.NewRequest(tg.method, "https://"+host+tg.duong, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, khoa)
	if tok != "" {
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
	}
	return m.chay(r)
}

// --- rule 5, invariant 7: four cases, every route -----------------------------------------------

func TestGhiCanBo_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	for _, tg := range moiTuyenGhi() {
		w := m.goiGhi(t, tg, hostA, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case ghi đã chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION. The chain is rebuilt with a checker that grants this account a
// different key in the same commune — a real role that has nothing to do with the staff register.
func TestGhiCanBo_403SaiQuyen(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("task.read"): true}},
			xaB: {},
		}}
	})

	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range moiTuyenGhi() {
		w := m.goiGhi(t, tg, hostA, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("sai quyền mà use case ghi đã chạy %d lần — phép kiểm quyền phải chặn TRƯỚC", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE — the case that catches a permission crossing
// a commune boundary (rule 5, invariant 3). The same person, holding `admin.user` in commune A,
// properly signed in at commune B: nothing about the request is malformed, the grant simply does
// not exist there.
func TestGhiCanBo_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChu(t)

	tok := m.tokenCho(t, xaB, sidB)
	for _, tg := range moiTuyenGhi() {
		w := m.goiGhi(t, tg, hostB, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case ghi đã chạy %d lần", n)
	}
}

func TestGhiCanBo_2xxDuCaHai(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenGhi() {
		w := m.goiGhi(t, tg, hostA, tok)
		if w.Code != tg.ok {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", tg.ten, w.Code, tg.ok, w.Body.String())
		}
	}
	if n := m.ghiDanhBa.soLanGoi(); n != len(moiTuyenGhi()) {
		t.Errorf("use case ghi chạy %d lần, muốn %d", n, len(moiTuyenGhi()))
	}
	if m.ghiDanhBa.xaCuoi != xaA {
		t.Errorf("use case được gọi với xã %q, muốn %q", m.ghiDanhBa.xaCuoi, xaA)
	}
}

// --- rule 6, invariant 8: which identifier reaches the trail --------------------------------------

// THE HANDLER PASSES THE STAFF CODE AS THE AUDIT ACTOR AND THE INTERNAL ID AS THE DECIDER. This is
// the one layer that chooses between a principal's two identifiers, and the wrong choice is
// invisible: a ULID is a perfectly valid string in `audit_log.actor_id`.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestGhiCanBoChuTheVetLaMaCanBoVaQuyetDinhTheoIDNoiBo(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[2], hostA, tok) // khoá tài khoản
	doiMa(t, w, http.StatusOK)

	nguoi := m.ghiDanhBa.nguoiCuoi
	if nguoi.Vet.ID != maCanBo {
		t.Errorf("chủ thể vết = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", nguoi.Vet.ID, maCanBo)
	}
	if nguoi.Vet.ID == idNoiBo {
		t.Error("chủ thể vết đang mang ĐỊNH DANH NỘI BỘ")
	}
	if nguoi.ID != idNoiBo {
		t.Errorf("định danh quyết định = %q, muốn id nội bộ %q — câu #14 so sánh trên id này",
			nguoi.ID, idNoiBo)
	}
	if nguoi.Vet.Kind != "staff" {
		t.Errorf("actor kind = %q, muốn staff", nguoi.Vet.Kind)
	}
	if nguoi.Vet.IP != "10.0.0.7" {
		t.Errorf("actor IP = %q, muốn địa chỉ thật của yêu cầu", nguoi.Vet.IP)
	}
}

// --- what the handler passes through --------------------------------------------------------------

func TestThemCanBoChuyenDungCacTruong(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[0], hostA, tok)
	doiMa(t, w, http.StatusCreated)

	yc := m.ghiDanhBa.themCuoi
	if yc.HoTen != "Trần Thị B" || yc.Email != "canbo.b@example.gov.vn" {
		t.Errorf("trường bị lẫn: %+v", yc)
	}
	// `mobile` IS `di_dong_ca_nhan` AND `office_phone` IS `dien_thoai_co_quan`. They are adjacent
	// strings of the same type, so a swap produces no error anywhere — and the two are different
	// kinds of data in law (#16), so a swap applies every masking and export rule to the wrong one.
	if yc.DiDongCaNhan != "0900000002" {
		t.Errorf("mobile không vào di_dong_ca_nhan: %+v", yc)
	}
	if yc.DienThoaiCoQuan != "" {
		t.Errorf("office_phone không được gửi mà vẫn có giá trị: %q", yc.DienThoaiCoQuan)
	}
}

// AN ABSENT FIELD IS A nil POINTER, NOT AN EMPTY STRING. Sending `{"position":"..."}` must leave
// both telephone numbers alone; if the handler turned absent into empty, a screen that edits only
// the position would wipe them off a government directory with nothing reporting it.
func TestSuaCanBoTruongVangMatLaNilChuKhongPhaiRong(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[1], hostA, tok)
	doiMa(t, w, http.StatusOK)

	yc := m.ghiDanhBa.suaCuoi
	if yc.ChucVu == nil || *yc.ChucVu != "Chuyên viên" {
		t.Fatalf("position không tới được use case: %+v", yc)
	}
	if yc.DiDongCaNhan != nil || yc.DienThoaiCoQuan != nil || yc.HoTen != nil || yc.Email != nil {
		t.Error("trường KHÔNG gửi lên lại tới use case dưới dạng rỗng — sẽ xoá trắng dữ liệu")
	}
	if m.ghiDanhBa.idCuoi != idNoiBo {
		t.Errorf("id trên đường dẫn không tới use case: %q", m.ghiDanhBa.idCuoi)
	}
}

// THE TWO DIRECTIONS OF THE LOCKOUT REACH THE USE CASE AS TWO DIFFERENT VALUES. One boolean, and
// the two routes differ in nothing else — which is exactly why a test has to read it.
func TestKhoaVaMoKhoaGuiDungHuong(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goiGhi(t, moiTuyenGhi()[2], hostA, tok), http.StatusOK)
	if !m.ghiDanhBa.khoaCuoi {
		t.Error("POST .../lockout phải yêu cầu KHOÁ")
	}
	doiMa(t, m.goiGhi(t, moiTuyenGhi()[3], hostA, tok), http.StatusOK)
	if m.ghiDanhBa.khoaCuoi {
		t.Error("DELETE .../lockout phải yêu cầu MỞ KHOÁ")
	}
}

// AN EMPTY role_id IS A LEGITIMATE DESTINATION — "no role" — and must not be turned into an error
// or dropped. `nguoi_dung.vai_tro_id` is nullable, and a person can sit in the org chart holding
// nothing.
func TestDoiVaiTroChuoiRongLaGoVaiTro(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	tg := moiTuyenGhi()[4]
	tg.than = `{"role_id":""}`
	doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusOK)

	if m.ghiDanhBa.vaiTroCuoi != "" {
		t.Errorf("vai trò gửi xuống = %q, muốn chuỗi rỗng", m.ghiDanhBa.vaiTroCuoi)
	}
	if m.ghiDanhBa.soLanGoi() != 1 {
		t.Error("use case không được gọi")
	}
}

// --- how each refusal reaches the client ------------------------------------------------------------

// THE MAPPING FROM REFUSAL TO STATUS IS THE HANDLER'S OWN JOB, and each row is a decision:
//
//	403 self_target_forbidden   NOT about a permission — granting the caller another one would not
//	                            change the answer, so the client must not send them to Phân quyền.
//	403 permission_escalation   same shape, and the missing keys are named so the administrator
//	                            knows what to ask for.
//	409 last_admin              NOT 403: the caller holds `admin.user`. What is refused is the
//	                            operation against the STATE of the commune, and the same caller may
//	                            do it once a second administrator exists.
//	404 staff_not_found         another commune's id answers exactly like an invented one.
func TestGhiCanBoAnhXaTungLoiVeDungMaTrangThai(t *testing.T) {
	cases := []struct {
		ten    string
		loi    error
		status int
		ma     string
	}{
		{"tự thao tác lên chính mình", app.ErrTuThaoTacChinhMinh, http.StatusForbidden, "self_target_forbidden"},
		{"trao quyền không cầm", &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm"}},
			http.StatusForbidden, "permission_escalation"},
		{"quản trị viên cuối cùng", app.ErrQuanTriCuoiCung, http.StatusConflict, "last_admin"},
		{"không tìm thấy cán bộ", idstore.ErrCanBoKhongTonTai, http.StatusNotFound, "staff_not_found"},
		{"vai trò không còn", app.ErrVaiTroKhongTonTai, http.StatusBadRequest, "role_not_found"},
		{"bộ phận không còn", idstore.ErrBoPhanKhongTonTai, http.StatusBadRequest, "org_unit_not_found"},
		{"thư điện tử đã dùng", idstore.ErrEmailDaDung, http.StatusConflict, "email_taken"},
		{"thiếu họ tên", domain.ErrThieuHoTen, http.StatusBadRequest, "invalid_request"},
		{"kho hỏng", errors.New("kho hỏng"), http.StatusInternalServerError, "internal"},
	}

	for _, c := range cases {
		m := dungMayChu(t)
		m.ghiDanhBa.loi = c.loi
		tok := m.tokenCho(t, xaA, sidA)

		w := m.goiGhi(t, moiTuyenGhi()[4], hostA, tok) // đổi vai trò — tuyến gặp đủ các lỗi này
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.ten, w.Code, c.status, w.Body.String())
			continue
		}
		e := loiTra(t, w)
		if e.Code != c.ma {
			t.Errorf("%s: code = %q, muốn %q", c.ten, e.Code, c.ma)
		}
	}
}

// THE KEYS THE CALLER LACKS ARE NAMED IN THE MESSAGE. "You may not do this" sends an administrator
// to guess; naming `budget.confirm` tells them what to ask their own superior for. Permission keys
// are not personal data — they are the strings the Phân quyền screen prints.
func TestLoiTraoQuyenNeuTenCacKhoaConThieu(t *testing.T) {
	m := dungMayChu(t)
	m.ghiDanhBa.loi = &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm", "document.route"}}
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[4], hostA, tok)
	doiMa(t, w, http.StatusForbidden)

	than := w.Body.String()
	for _, khoa := range []string{"budget.confirm", "document.route"} {
		if !strings.Contains(than, khoa) {
			t.Errorf("thông báo không nêu khoá %q: %s", khoa, than)
		}
	}
}

// NO ERROR BODY EVER CARRIES PERSONAL DATA (rule 3, forbidden #3). The one that is most likely to
// is the 500, whose wrapped error holds whatever the store said.
func TestLoiGhiCanBoKhongLoDuLieuCaNhan(t *testing.T) {
	m := dungMayChu(t)
	m.ghiDanhBa.loi = errors.New("pq: duplicate key value \"0900000002\" for Trần Thị B")
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[0], hostA, tok)
	doiMa(t, w, http.StatusInternalServerError)

	than := w.Body.String()
	if strings.Contains(than, "0900000002") || strings.Contains(than, "Trần Thị B") {
		t.Errorf("thân lỗi mang dữ liệu cá nhân: %s", than)
	}
}

// --- the reply shape ------------------------------------------------------------------------------

// EVERY WRITE ROUTE ANSWERS WITH THE SAME SHAPE THE READ ROUTES DO, through raNgoai. A write route
// building its own shape is how a screen ends up rendering two versions of one row.
//
// IT CARRIES `mobile`, UNMASKED, AND THAT IS OPEN QUESTION #11's ANSWER (see soRaManHinhNoiBo).
// It also carries NO password field of any kind — there is no field on the type to put one in.
func TestGhiCanBoTraVeCungHinhDangVoiTuyenDoc(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiGhi(t, moiTuyenGhi()[1], hostA, tok)
	doiMa(t, w, http.StatusOK)

	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	for _, truong := range []string{"id", "code", "full_name", "email", "phone", "mobile", "has_account", "active"} {
		if _, co := ra[truong]; !co {
			t.Errorf("thiếu trường %q trong phản hồi: %s", truong, w.Body.String())
		}
	}
	if ra["mobile"] != "0900000009" {
		t.Errorf("mobile = %v, muốn số đầy đủ (câu #11: không che trong nội bộ xã)", ra["mobile"])
	}
	for _, cam := range []string{"password", "mat_khau", "password_hash", "mat_khau_hash"} {
		if _, co := ra[cam]; co {
			t.Errorf("phản hồi mang trường %q", cam)
		}
	}
}

// --- duplicate protection -------------------------------------------------------------------------

// POST /api/v1/staff REFUSES A REQUEST WITH NO Idempotency-Key. There is no natural unique key
// underneath — the staff code is random by design (#15) and a name is not a key — so this header is
// the ONLY thing standing between a double-submitted form and two permanent directory rows — #10's
// soft delete can hide the second one, but only as an audited act under its own key, and the row
// and its code stay in the table for ever.
func TestThemCanBoThieuIdempotencyKeyBiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	tg := moiTuyenGhi()[0]
	r := httptest.NewRequest(tg.method, "https://"+hostA+tg.duong, strings.NewReader(tg.than))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})

	w := m.chay(r)
	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); e.Code != "missing_idempotency_key" {
		t.Errorf("code = %q, muốn missing_idempotency_key", e.Code)
	}
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("thiếu khoá chống trùng mà use case vẫn chạy %d lần", n)
	}
}

// THE SAME KEY SENT TWICE RUNS THE USE CASE ONCE. The second request is answered from the first
// one's result — which is what stops a double-submitted form from creating two staff rows.
func TestThemCanBoGuiLaiCungKhoaChayDungMotLan(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	tg := moiTuyenGhi()[0]

	doiMa(t, m.goiGhiKhoa(t, tg, hostA, tok, "01JIDEMKEYTRUNGLAP01"), http.StatusCreated)
	w := m.goiGhiKhoa(t, tg, hostA, tok, "01JIDEMKEYTRUNGLAP01")
	if w.Code != http.StatusCreated {
		t.Fatalf("lần gửi thứ hai mã = %d, muốn 201 phát lại — thân: %s", w.Code, w.Body.String())
	}
	if n := m.ghiDanhBa.soLanGoi(); n != 1 {
		t.Errorf("use case chạy %d lần cho cùng một khoá chống trùng, muốn 1", n)
	}
}
