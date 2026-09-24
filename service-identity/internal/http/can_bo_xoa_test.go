package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// DELETE /api/v1/staff/{id} — the soft delete of a DUPLICATED directory row (#10). Rule 5,
// invariant 7's four cases, plus what the handler alone decides: the body is read, the actor
// carries the staff code, and each refusal reaches the client as its own status and code.
//
// The refusals themselves (account on the row, #13, #14, the reason's shape) are proven against a
// transaction in app/danh_ba_can_bo_xoa_test.go; re-asserting them through a fake use case here
// would assert that the fake returns what it was told to.

const idDongTrung = "nd-01JDONGTRUNG00000000000"

func tuyenXoa(than string) tuyenGhi {
	return tuyenGhi{"xoá dòng nhập trùng", "DELETE", "/api/v1/staff/" + idDongTrung, than, http.StatusNoContent}
}

const thanXoaDung = `{"reason":"Nhập trùng với CB-2026-7K3M9Q"}`

// coAdminUserDelete rebuilds the chain with a checker granting `admin.user.delete` — and NOTHING
// ELSE, not even `admin.user` — in commune A only. Commune B has the same account and grants
// nothing, which is what makes the wrong-commune case a case about the commune.
func coAdminUserDelete(t *testing.T, m *mayChu) {
	t.Helper()
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.user.delete"): true}},
			xaB: {},
		}}
	})
}

func TestXoaCanBo_401KhongToken(t *testing.T) {
	m := dungMayChu(t)
	coAdminUserDelete(t, m)

	doiMa(t, m.goiGhi(t, tuyenXoa(thanXoaDung), hostA, ""), http.StatusUnauthorized)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case xoá đã chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION — and the wrong permission chosen is `admin.user`, the key every
// other register write declares. #10 gave the delete its own key; `admin.user` is not it.
//
// MUTATION THAT MUST TURN THIS RED: declare the route with "admin.user".
func TestXoaCanBo_403CoAdminUserMaKhongCoAdminUserDelete(t *testing.T) {
	m := dungMayChu(t) // the shipped default: admin.user in commune A, nothing else

	doiMa(t, m.goiGhi(t, tuyenXoa(thanXoaDung), hostA, m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("chỉ có admin.user mà use case xoá đã chạy %d lần", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE (rule 5, invariant 3).
func TestXoaCanBo_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChu(t)
	coAdminUserDelete(t, m)

	doiMa(t, m.goiGhi(t, tuyenXoa(thanXoaDung), hostB, m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case xoá đã chạy %d lần", n)
	}
}

// 204 WITH NO BODY — and the id, the reason and the actor reach the use case as they should. The
// checker grants `admin.user.delete` ALONE, so this also proves the route asks for nothing more:
// keys are flat (rule 5, invariant 3b), not a hierarchy under `admin.user`.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestXoaCanBo_204VaChuyenDungTruong(t *testing.T) {
	m := dungMayChu(t)
	coAdminUserDelete(t, m)

	w := m.goiGhi(t, tuyenXoa(thanXoaDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Errorf("204 mà có thân: %s", w.Body.String())
	}

	g := m.ghiDanhBa
	if g.idCuoi != idDongTrung || g.xaCuoi != xaA {
		t.Errorf("id/xã tới use case = %q/%q", g.idCuoi, g.xaCuoi)
	}
	if g.lyDoXoa != "Nhập trùng với CB-2026-7K3M9Q" {
		t.Errorf("lý do tới use case = %q", g.lyDoXoa)
	}
	if g.nguoiCuoi.Vet.ID != maCanBo || g.nguoiCuoi.ID != idNoiBo {
		t.Errorf("người thực hiện: vết=%q quyết định=%q, muốn %q/%q",
			g.nguoiCuoi.Vet.ID, g.nguoiCuoi.ID, maCanBo, idNoiBo)
	}
}

// NO BODY OR A BODY THAT IS NOT JSON — refused before the use case runs.
func TestXoaCanBoThieuThanBiTuChoi(t *testing.T) {
	for _, than := range []string{"", "khong-phai-json"} {
		m := dungMayChu(t)
		coAdminUserDelete(t, m)

		doiMa(t, m.goiGhi(t, tuyenXoa(than), hostA, m.tokenCho(t, xaA, sidA)), http.StatusBadRequest)
		if n := m.ghiDanhBa.soLanGoi(); n != 0 {
			t.Errorf("thân %q: use case vẫn chạy %d lần", than, n)
		}
	}
}

// EACH REFUSAL REACHES THE CLIENT AS ITS OWN STATUS AND CODE.
//
//	409 staff_has_account   NOT 403: the caller holds the key; what is refused is the row's state.
//	400 invalid_request     a blank or over-long reason, with the domain's own sentence.
//	404 staff_not_found     already deleted, another commune's, or invented — one answer.
func TestXoaCanBoAnhXaTungLoiVeDungMaTrangThai(t *testing.T) {
	cases := []struct {
		ten    string
		loi    error
		status int
		ma     string
	}{
		{"dòng có tài khoản", app.ErrCanBoCoTaiKhoan, http.StatusConflict, "staff_has_account"},
		{"thiếu lý do", domain.ErrThieuLyDoXoa, http.StatusBadRequest, "invalid_request"},
		{"lý do quá dài", domain.ErrLyDoXoaQuaDai, http.StatusBadRequest, "invalid_request"},
		{"không tìm thấy", idstore.ErrCanBoKhongTonTai, http.StatusNotFound, "staff_not_found"},
		{"quản trị viên cuối cùng", app.ErrQuanTriCuoiCung, http.StatusConflict, "last_admin"},
		{"tự xoá mình", app.ErrTuThaoTacChinhMinh, http.StatusForbidden, "self_target_forbidden"},
	}
	for _, c := range cases {
		m := dungMayChu(t)
		coAdminUserDelete(t, m)
		m.ghiDanhBa.loi = c.loi

		w := m.goiGhi(t, tuyenXoa(thanXoaDung), hostA, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.ten, w.Code, c.status, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != c.ma {
			t.Errorf("%s: code = %q, muốn %q", c.ten, e.Code, c.ma)
		}
	}
}

// THE 409 FOR AN ACCOUNT NAMES BOTH WAYS FORWARD: lock for a retirement, revoke for a genuine
// duplicate — and says plainly that revocation does not exist yet.
func TestXoaCanBoCoTaiKhoanNoiRoHaiLoiRa(t *testing.T) {
	m := dungMayChu(t)
	coAdminUserDelete(t, m)
	m.ghiDanhBa.loi = app.ErrCanBoCoTaiKhoan

	w := m.goiGhi(t, tuyenXoa(thanXoaDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusConflict)
	than := w.Body.String()
	for _, can := range []string{"khoá tài khoản", "thu hồi tài khoản"} {
		if !strings.Contains(than, can) {
			t.Errorf("thông báo 409 không nêu %q: %s", can, than)
		}
	}
}
