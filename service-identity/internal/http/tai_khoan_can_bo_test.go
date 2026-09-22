package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// WHAT THIS FILE IS FOR: the three credential routes of open questions #9 and #17, plus the gate
// that makes #9 mean anything — a session under the forced password change may reach NOTHING but
// the screen that frees it.
//
// Rule 5, invariant 7 asks four cases of every new route:
//
//	401  no token
//	403  a signed-in account holding the WRONG permission
//	403  the RIGHT permission, in the WRONG COMMUNE
//	2xx  both correct
//
// THE THIRD CASE IS THE ONE THAT PROVES ANYTHING, and only because checkerGia is keyed BY COMMUNE
// (routes_test.go): the same account holds `admin.user` in commune A and nothing in commune B.
//
// THE SELF ROUTE IS AnyAuthenticated, so its four cases are 401 · 204 · the commune case (a token
// of commune A presented at commune B's host, which the edge refuses at the token layer before any
// query) · and the refusals of its own body. There is no "wrong permission" for a route every
// account must be able to call — that IS the decision, and it is argued at the route.
//
// WHAT IS DELIBERATELY NOT HERE: the transaction, the audit entry and the #14 refusal. Those are
// decisions of the use case, proven against a real transaction in app/tai_khoan_can_bo_test.go and
// against a real server in app/tai_khoan_can_bo_pg_test.go. Asserting them through a fake use case
// would assert that the fake returns what it was told to.

// --- the fake use case --------------------------------------------------------------------------

// taiKhoanGia stands in for *app.TaiKhoanCanBo.
//
// IT RETURNS A FIXED PLAINTEXT so a test can assert the handler puts it in the body — and, more
// importantly, that nothing else in the response or the chain carries it.
type taiKhoanGia struct {
	mu sync.Mutex

	goi       int
	idCuoi    string
	nguoiCuoi app.NguoiThucHien
	doiCuoi   app.YeuCauDoiMatKhau

	matKhauTam string
	kq         domain.CanBoTomTat
	loi        error
}

const matKhauTamGia = "MAT-KHAU-TAM-GIA-KHONG-PHAI-THAT"

func taiKhoanMau() *taiKhoanGia {
	return &taiKhoanGia{
		matKhauTam: matKhauTamGia,
		kq: domain.CanBoTomTat{
			ID: idNoiBo, Ma: maCanBo, HoTen: "Nguyễn Văn A", Email: emailDung,
			ChucVu: "Công chức Văn phòng", CoTaiKhoan: true, DangHoatDong: true,
		},
	}
}

func (g *taiKhoanGia) ghiNhan(nguoi app.NguoiThucHien, id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goi++
	g.nguoiCuoi = nguoi
	g.idCuoi = id
}

func (g *taiKhoanGia) soLanGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.goi
}

func (g *taiKhoanGia) Cap(_ context.Context, id string, nguoi app.NguoiThucHien) (app.KetQuaCapMatKhau, error) {
	g.ghiNhan(nguoi, id)
	if g.loi != nil {
		return app.KetQuaCapMatKhau{}, g.loi
	}
	return app.KetQuaCapMatKhau{CanBo: g.kq, MatKhauTam: g.matKhauTam}, nil
}

func (g *taiKhoanGia) DatLai(_ context.Context, id string, nguoi app.NguoiThucHien) (app.KetQuaCapMatKhau, error) {
	g.ghiNhan(nguoi, id)
	if g.loi != nil {
		return app.KetQuaCapMatKhau{}, g.loi
	}
	return app.KetQuaCapMatKhau{CanBo: g.kq, MatKhauTam: g.matKhauTam}, nil
}

func (g *taiKhoanGia) DoiCuaChinhMinh(_ context.Context, yc app.YeuCauDoiMatKhau, nguoi app.NguoiThucHien) error {
	g.ghiNhan(nguoi, "")
	g.mu.Lock()
	g.doiCuoi = yc
	g.mu.Unlock()
	return g.loi
}

// --- helpers ------------------------------------------------------------------------------------

const (
	duongCapTaiKhoan  = "/api/v1/staff/nd-muc-tieu-0001/account"
	duongDatLaiMK     = "/api/v1/staff/nd-muc-tieu-0001/password"
	duongTuDoiMatKhau = "/api/v1/staff/current/password"

	thanDoiMatKhau = `{"current_password":"mat-khau-cu-KHONG-PHAI-THAT",` +
		`"new_password":"mat-khau-moi-KHONG-PHAI-THAT"}`
)

// batDoiMatKhau puts the harness's one account under the forced change of #9.
func (m *mayChu) batDoiMatKhau(phai bool) {
	cb := m.canBo.theo[idNoiBo]
	cb.PhaiDoiMatKhau = phai
	m.canBo.theo[idNoiBo] = cb
}

// khongCoQuyenNao is the "wrong permission" fixture: a signed-in account holding nothing at all.
func khongCoQuyenNao() checkerGia {
	return checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{}}
}

// goiIdem issues one request carrying a FRESH Idempotency-Key.
//
// PUT /api/v1/staff/{id}/password declares idem.Required, so it refuses a request without one
// BEFORE the handler runs — a harness that omitted the header would turn every assertion about that
// route into an assertion about the header. The other routes here declare idem.KhongCan and ignore
// it, so one helper serves all of them.
//
// THE KEY IS UNIQUE PER CALL because the harness's store is real (in memory): replaying a key would
// be answered from the first result, which is correct behaviour and not what these cases are about.
var soKhoaIdemTaiKhoan int

func (m *mayChu) goiIdem(t *testing.T, method, host, duong, than, tok string) *httptest.ResponseRecorder {
	t.Helper()
	soKhoaIdemTaiKhoan++
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+duong, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMKEYTAIKHOAN"+
		string(rune('A'+soKhoaIdemTaiKhoan%26))+string(rune('A'+(soKhoaIdemTaiKhoan/26)%26)))
	if tok != "" {
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
	}
	return m.chay(r)
}

func duongThu2(t *testing.T, duong string) *url.URL {
	t.Helper()
	u, err := url.Parse(duong)
	if err != nil {
		t.Fatalf("đường dẫn %q không phân tích được: %v", duong, err)
	}
	return u
}

// --- POST /api/v1/staff/{id}/account -------------------------------------------------------------

func TestCapTaiKhoanKhongCoTokenTra401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPost, hostA, duongCapTaiKhoan, "", "")
	doiMa(t, w, http.StatusUnauthorized)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi dù chưa đăng nhập")
	}
}

func TestCapTaiKhoanSaiQuyenTra403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = khongCoQuyenNao()
	})
	w := m.goi(t, http.MethodPost, hostA, duongCapTaiKhoan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi dù thiếu quyền")
	}
}

// THE CASE THAT PROVES THE ISOLATION: the same account, the same permission key, the other
// commune's host — and the answer is 403, not 201.
func TestCapTaiKhoanDungQuyenSaiXaTra403(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPost, hostB, duongCapTaiKhoan, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi ở xã không cấp quyền")
	}
}

func TestCapTaiKhoanDuQuyenTra201VaMatKhauTamMotLan(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiIdem(t, http.MethodPost, hostA, duongCapTaiKhoan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusCreated)

	var ra struct {
		Staff struct {
			Code string `json:"code"`
		} `json:"staff"`
		TemporaryPassword string `json:"temporary_password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
	if ra.TemporaryPassword != matKhauTamGia {
		t.Fatalf("temporary_password = %q, muốn mật khẩu tạm use case trả về", ra.TemporaryPassword)
	}
	if ra.Staff.Code != maCanBo {
		t.Fatalf("staff.code = %q, muốn %q", ra.Staff.Code, maCanBo)
	}
	// Rule 6, invariant 8: the handler hands the use case the BUSINESS CODE as the audit actor, and
	// the internal id as the thing the guards decide on. A swap makes both look like they work.
	if m.taiKhoan.nguoiCuoi.Vet.ID != maCanBo {
		t.Fatalf("chủ thể vết = %q, muốn mã cán bộ %q", m.taiKhoan.nguoiCuoi.Vet.ID, maCanBo)
	}
	if m.taiKhoan.nguoiCuoi.ID != idNoiBo {
		t.Fatalf("id người thực hiện = %q, muốn id nội bộ %q", m.taiKhoan.nguoiCuoi.ID, idNoiBo)
	}
	if m.taiKhoan.idCuoi != "nd-muc-tieu-0001" {
		t.Fatalf("id đích = %q, không phải đoạn {id} của đường dẫn", m.taiKhoan.idCuoi)
	}
}

func TestCapTaiKhoanChoNguoiDaCoTaiKhoanTra409(t *testing.T) {
	m := dungMayChu(t)
	m.taiKhoan.loi = app.ErrDaCoTaiKhoan

	w := m.goiIdem(t, http.MethodPost, hostA, duongCapTaiKhoan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "account_exists" {
		t.Fatalf("mã lỗi = %q, muốn account_exists", loiTra(t, w).Code)
	}
}

// #14 reaches the client as 403 with a code that says it is NOT about a permission: granting the
// caller another right would not change the answer, so the message must not send them to the Phân
// quyền screen.
func TestCapTaiKhoanTuThaoTacChinhMinhTra403(t *testing.T) {
	m := dungMayChu(t)
	m.taiKhoan.loi = app.ErrTuThaoTacChinhMinh

	w := m.goiIdem(t, http.MethodPost, hostA, duongCapTaiKhoan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if loiTra(t, w).Code != "self_target_forbidden" {
		t.Fatalf("mã lỗi = %q, muốn self_target_forbidden", loiTra(t, w).Code)
	}
}

// --- PUT /api/v1/staff/{id}/password -------------------------------------------------------------

func TestDatLaiMatKhauKhongCoTokenTra401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPut, hostA, duongDatLaiMK, "", "")
	doiMa(t, w, http.StatusUnauthorized)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi dù chưa đăng nhập")
	}
}

func TestDatLaiMatKhauSaiQuyenTra403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = khongCoQuyenNao()
	})
	w := m.goi(t, http.MethodPut, hostA, duongDatLaiMK, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
}

func TestDatLaiMatKhauDungQuyenSaiXaTra403(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPut, hostB, duongDatLaiMK, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi ở xã không cấp quyền")
	}
}

func TestDatLaiMatKhauDuQuyenTra200(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiIdem(t, http.MethodPut, hostA, duongDatLaiMK, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), matKhauTamGia) {
		t.Fatal("phản hồi không mang mật khẩu tạm — quản trị viên không có gì để đọc cho người ta")
	}
}

func TestDatLaiMatKhauChoNguoiChuaCoTaiKhoanTra409(t *testing.T) {
	m := dungMayChu(t)
	m.taiKhoan.loi = app.ErrChuaCoTaiKhoan

	w := m.goiIdem(t, http.MethodPut, hostA, duongDatLaiMK, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "account_missing" {
		t.Fatalf("mã lỗi = %q, muốn account_missing", loiTra(t, w).Code)
	}
}

// --- PUT /api/v1/staff/current/password ----------------------------------------------------------

// THE ROUTING RULE THIS ROUTE DEPENDS ON, PINNED. `current` is a literal segment and `{id}` is a
// wildcard; net/http routes the more specific pattern, so a person changing their own password is
// NOT answered by the administrator route — which requires `admin.user` and would refuse most of
// the commune.
//
// MUTATION THAT MUST TURN THIS RED: give the self route a path under `{id}`, or register it before
// the mux can tell them apart.
func TestTuDoiMatKhauKhongBiTuyenQuanTriNuot(t *testing.T) {
	m := dungMayChu(t)
	// The account holds NO permission in commune B, and yet the self route must serve: if the
	// administrator pattern had swallowed it, this would be 403.
	m.dungLai(t, func(d *Deps) {
		d.Checker = khongCoQuyenNao()
	})
	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, thanDoiMatKhau, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)
	if m.taiKhoan.soLanGoi() != 1 {
		t.Fatalf("use case được gọi %d lần, muốn 1 — tuyến tự đổi có thể đã bị tuyến quản trị nuốt",
			m.taiKhoan.soLanGoi())
	}
}

func TestTuDoiMatKhauKhongCoTokenTra401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, thanDoiMatKhau, "")
	doiMa(t, w, http.StatusUnauthorized)
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi dù chưa đăng nhập")
	}
}

// A token issued for commune A, presented at commune B's host: refused at the token layer, before
// any query. This is the self route's form of the "wrong commune" case.
func TestTuDoiMatKhauTokenXaKhacTra401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPut, hostB, duongTuDoiMatKhau, thanDoiMatKhau, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if loiTra(t, w).Code != "tenant_mismatch" {
		t.Fatalf("mã lỗi = %q, muốn tenant_mismatch", loiTra(t, w).Code)
	}
	if m.taiKhoan.soLanGoi() != 0 {
		t.Fatal("use case bị gọi với token của xã khác")
	}
}

func TestTuDoiMatKhauThanhCongTra204VaXoaCookie(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, thanDoiMatKhau, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)

	if w.Body.Len() != 0 {
		t.Fatalf("204 nhưng có thân trả về: %s", w.Body.String())
	}
	// The use case revoked every session of this account, including this one. Leaving the browser
	// holding the token would show a signed-in interface that 401s on the first click.
	c := cookiePhien(t, w)
	if c == nil || c.Value != "" || c.MaxAge >= 0 {
		t.Fatalf("cookie phiên chưa bị xoá sau khi đổi mật khẩu: %+v", c)
	}
	if m.taiKhoan.doiCuoi.HienTai == "" || m.taiKhoan.doiCuoi.Moi == "" {
		t.Fatalf("hai trường mật khẩu không tới được use case: %+v", m.taiKhoan.doiCuoi)
	}
}

func TestTuDoiMatKhauSaiMatKhauHienTaiTra400(t *testing.T) {
	m := dungMayChu(t)
	m.taiKhoan.loi = app.ErrMatKhauHienTaiSai

	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, thanDoiMatKhau, m.tokenCho(t, xaA, sidA))
	// 400 AND NOT 401: the session is valid, one field is wrong. A 401 would sign the person out
	// and, on the forced-change screen, loop them back to exactly the same place.
	doiMa(t, w, http.StatusBadRequest)
	if loiTra(t, w).Code != "current_password_invalid" {
		t.Fatalf("mã lỗi = %q, muốn current_password_invalid", loiTra(t, w).Code)
	}
	if cookiePhien(t, w) != nil {
		t.Fatal("mật khẩu sai nhưng phiên bị xoá — người dùng bị đăng xuất vì gõ nhầm một ô")
	}
}

func TestTuDoiMatKhauQuaNganTra400VaKhongNhacLaiGiaTri(t *testing.T) {
	m := dungMayChu(t)
	m.taiKhoan.loi = domain.ErrMatKhauQuaNgan

	const than = `{"current_password":"cu-KHONG-PHAI-THAT","new_password":"ngan"}`
	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, than, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if strings.Contains(w.Body.String(), "ngan") || strings.Contains(w.Body.String(), "KHONG-PHAI-THAT") {
		t.Fatalf("thông báo lỗi nhắc lại giá trị đã gửi lên: %s", w.Body.String())
	}
}

// A malformed body must not come back quoting itself: json.Decoder's own message includes the
// offending input, which on this route is a password (rule 3, forbidden #3).
func TestTuDoiMatKhauThanHongKhongNhacLaiThan(t *testing.T) {
	m := dungMayChu(t)
	const than = `{"current_password":"mat-khau-that-KHONG-DUOC-LO"` // truncated on purpose

	w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, than, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if strings.Contains(w.Body.String(), "KHONG-DUOC-LO") {
		t.Fatalf("thân lỗi mang theo nội dung đã gửi lên: %s", w.Body.String())
	}
}

// --- the forced-change gate (#9) ------------------------------------------------------------------

// THE PROPERTY THE WHOLE OF #9 RESTS ON: while `phai_doi_mat_khau` is true, the session may do
// NOTHING but change that password. The customer's decision is that after the first sign-in the
// administrator no longer knows anybody's password; a session that can work normally before that
// change makes the temporary password a full account instead.
//
// MUTATION THAT MUST TURN THIS RED: delete the `cb.PhaiDoiMatKhau && !duocPhepKhiPhaiDoiMatKhau(r)`
// branch from XacThuc. Every other test in this package stays green.
func TestBatDoiMatKhauThiMoiTuyenKHACDeuBiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	m.batDoiMatKhau(true)

	ca := []struct {
		method, duong, than string
	}{
		{http.MethodGet, "/api/v1/staff", ""},
		{http.MethodGet, "/api/v1/staff/" + idNoiBo, ""},
		{http.MethodGet, "/api/v1/org-units", ""},
		{http.MethodGet, "/api/v1/roles", ""},
		{http.MethodPost, duongCapTaiKhoan, ""},
		{http.MethodPut, duongDatLaiMK, ""},
		{http.MethodPatch, "/api/v1/staff/" + idNoiBo, `{"position":"x"}`},
	}
	for _, c := range ca {
		t.Run(c.method+" "+c.duong, func(t *testing.T) {
			w := m.goiIdem(t, c.method, hostA, c.duong, c.than, m.tokenCho(t, xaA, sidA))
			if w.Code != http.StatusForbidden {
				t.Fatalf("mã trạng thái = %d, muốn 403 — phiên đang bị bắt đổi mật khẩu vẫn dùng được tuyến này",
					w.Code)
			}
			if got := loiTra(t, w).Code; got != "password_change_required" {
				t.Fatalf("mã lỗi = %q, muốn password_change_required — client không phân biệt được "+
					"'phải đổi mật khẩu' với 'thiếu quyền'", got)
			}
		})
	}
}

// THE THREE THAT MUST STILL WORK. Blocking any of them leaves a member of staff with a session that
// can do nothing at all — including the one thing they are being asked to do.
func TestBatDoiMatKhauVanDiDuocBaTuyenCanThiet(t *testing.T) {
	m := dungMayChu(t)
	m.batDoiMatKhau(true)
	tok := m.tokenCho(t, xaA, sidA)

	t.Run("đổi mật khẩu của chính mình", func(t *testing.T) {
		w := m.goi(t, http.MethodPut, hostA, duongTuDoiMatKhau, thanDoiMatKhau, tok)
		doiMa(t, w, http.StatusNoContent)
	})
	t.Run("xem phiên hiện tại", func(t *testing.T) {
		w := m.goi(t, http.MethodGet, hostA, "/api/v1/sessions/current", "", tok)
		doiMa(t, w, http.StatusOK)
		var ra struct {
			MustChangePassword bool `json:"must_change_password"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
			t.Fatalf("thân không phải JSON: %v", err)
		}
		if !ra.MustChangePassword {
			t.Fatal("must_change_password = false trong khi tài khoản đang bị bắt đổi — " +
				"giao diện không có cách nào biết vì sao mọi thứ khác trả 403")
		}
	})
	t.Run("đăng xuất", func(t *testing.T) {
		w := m.goi(t, http.MethodDelete, hostA, "/api/v1/sessions/"+sidA, "", tok)
		doiMa(t, w, http.StatusNoContent)
	})
}

// The gate must not fire on an account that is NOT under the forced change — otherwise it would
// refuse the whole commune, which is the failure direction that gets a guard deleted.
func TestKhongBatDoiMatKhauThiKhongChanGi(t *testing.T) {
	m := dungMayChu(t)
	m.batDoiMatKhau(false)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
}

// A PUBLIC route has no principal to check, so the gate never sees it. The sign-in screen has to
// keep working — it is how somebody under the forced change got there in the first place.
func TestBatDoiMatKhauKhongChanTuyenCongKhai(t *testing.T) {
	m := dungMayChu(t)
	m.batDoiMatKhau(true)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/communes/current", "", "")
	doiMa(t, w, http.StatusOK)
}

// The sign-in response says so too, so the client goes straight to the change screen instead of
// discovering the state by being refused.
func TestDangNhapBaoMustChangePassword(t *testing.T) {
	m := dungMayChu(t)
	cb := canBoMau()
	cb.PhaiDoiMatKhau = true
	m.dangNhap.canBo = &cb

	w := m.goi(t, http.MethodPost, hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	doiMa(t, w, http.StatusCreated)

	var ra struct {
		MustChangePassword bool `json:"must_change_password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if !ra.MustChangePassword {
		t.Fatal("đăng nhập không báo must_change_password cho tài khoản mang mật khẩu tạm")
	}
}

// The allow-list is derived from the SAME strings the mux registers with. If somebody renames a
// route and forgets this function, the gate would refuse the very screen that frees the account.
func TestDuongDanChoPhepKhiBatDoiKhopVoiMauRoute(t *testing.T) {
	if !strings.HasSuffix(MauDoiMatKhauChinhMinh, duongTuDoiMatKhau) {
		t.Fatalf("MauDoiMatKhauChinhMinh = %q không kết thúc bằng %q", MauDoiMatKhauChinhMinh, duongTuDoiMatKhau)
	}
	if !strings.HasPrefix(MauDoiMatKhauChinhMinh, http.MethodPut+" ") {
		t.Fatalf("MauDoiMatKhauChinhMinh = %q không bắt đầu bằng phương thức", MauDoiMatKhauChinhMinh)
	}
	if mauXemPhienHienTai != http.MethodGet+" /api/v1/sessions/current" {
		t.Fatalf("mauXemPhienHienTai = %q", mauXemPhienHienTai)
	}
	if !strings.HasSuffix(mauDangXuat, "{sid}") {
		t.Fatalf("mauDangXuat = %q — hàm duocPhepKhiPhaiDoiMatKhau cắt hậu tố này để lấy tiền tố", mauDangXuat)
	}
	// Nothing deeper than one segment under /sessions/ is allowed: a longer path would be a route
	// this list has never considered.
	r := &http.Request{Method: http.MethodDelete}
	r.URL = duongThu2(t, "/api/v1/sessions/"+sidA+"/khac")
	if duocPhepKhiPhaiDoiMatKhau(r) {
		t.Fatal("đường dẫn sâu hơn một đoạn dưới /sessions/ vẫn được cho qua")
	}
	r.URL = duongThu2(t, "/api/v1/sessions/")
	if duocPhepKhiPhaiDoiMatKhau(r) {
		t.Fatal("sid rỗng vẫn được cho qua")
	}
}
