package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// DELETE /api/v1/staff/{id}/account — `admin.user.revoke`, user decision 2026-10-03. Rule 5,
// invariant 7's four cases plus what the handler alone decides: the body is read, the reason and the
// actor reach the use case, and each refusal reaches the client as its own status and code.
//
// The decisions themselves (#13, #14, the transaction, sessions, the trail) are proven against a
// transaction in app/account_revoke_test.go and a real server in app/account_revoke_pg_test.go.

const (
	revokePath       = "/api/v1/staff/nd-muc-tieu-0001/account"
	revokeBody       = `{"reason":"Dòng nhập trùng"}`
	revokePermission = authz.Perm("admin.user.revoke")
)

// grantRevokeOnly rebuilds the chain with a checker granting `admin.user.revoke` — and NOTHING
// else, not even `admin.user` — in commune A only. Commune B has the same account and grants nothing.
func grantRevokeOnly(t *testing.T, m *mayChu) {
	t.Helper()
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {revokePermission: true}},
			xaB: {},
		}}
	})
}

func TestRevokeAccount_401WithoutToken(t *testing.T) {
	m := dungMayChu(t)
	grantRevokeOnly(t, m)

	doiMa(t, m.goi(t, http.MethodDelete, hostA, revokePath, revokeBody, ""), http.StatusUnauthorized)
	if n := m.taiKhoan.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION — `admin.user`, the key the issue/reset routes declare. Revoking
// has its own key; `admin.user` is not it.
//
// MUTATION THAT MUST TURN THIS RED: declare the route with "admin.user".
func TestRevokeAccount_403WithAdminUserButNotRevoke(t *testing.T) {
	m := dungMayChu(t) // the shipped default: admin.user in commune A, nothing else

	doiMa(t, m.goi(t, http.MethodDelete, hostA, revokePath, revokeBody, m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if n := m.taiKhoan.soLanGoi(); n != 0 {
		t.Errorf("chỉ có admin.user mà use case chạy %d lần", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE (rule 5, invariant 3).
func TestRevokeAccount_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	grantRevokeOnly(t, m)

	doiMa(t, m.goi(t, http.MethodDelete, hostB, revokePath, revokeBody, m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if n := m.taiKhoan.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case chạy %d lần", n)
	}
}

// 200 WITH THE RECORD, now directory-only — and the id, the reason and BOTH identifiers of the actor
// reach the use case as they should.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestRevokeAccount_200PassesFieldsAndReturnsDirectoryRow(t *testing.T) {
	m := dungMayChu(t)
	grantRevokeOnly(t, m)

	w := m.goi(t, http.MethodDelete, hostA, revokePath, revokeBody, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var out struct {
		Code       string `json:"code"`
		HasAccount bool   `json:"has_account"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
	if out.HasAccount || out.Code != maCanBo {
		t.Errorf("bản ghi trả về: code=%q has_account=%v, muốn %q/false", out.Code, out.HasAccount, maCanBo)
	}

	g := m.taiKhoan
	if g.idCuoi != "nd-muc-tieu-0001" {
		t.Errorf("id tới use case = %q", g.idCuoi)
	}
	if g.lastReason != "Dòng nhập trùng" {
		t.Errorf("lý do tới use case = %q", g.lastReason)
	}
	if g.nguoiCuoi.Vet.ID != maCanBo || g.nguoiCuoi.ID != idNoiBo {
		t.Errorf("người thực hiện: vết=%q quyết định=%q, muốn %q/%q",
			g.nguoiCuoi.Vet.ID, g.nguoiCuoi.ID, maCanBo, idNoiBo)
	}
}

// NO BODY OR A BODY THAT IS NOT JSON — refused before the use case runs.
func TestRevokeAccountWithoutBodyRefused(t *testing.T) {
	for _, body := range []string{"", "khong-phai-json"} {
		m := dungMayChu(t)
		grantRevokeOnly(t, m)

		doiMa(t, m.goi(t, http.MethodDelete, hostA, revokePath, body, m.tokenCho(t, xaA, sidA)), http.StatusBadRequest)
		if n := m.taiKhoan.soLanGoi(); n != 0 {
			t.Errorf("thân %q: use case vẫn chạy %d lần", body, n)
		}
	}
}

// EACH REFUSAL REACHES THE CLIENT AS ITS OWN STATUS AND CODE.
func TestRevokeAccountMapsEachErrorToItsStatus(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"không có tài khoản", app.ErrChuaCoTaiKhoan, http.StatusConflict, "account_missing"},
		{"quản trị viên cuối cùng", app.ErrQuanTriCuoiCung, http.StatusConflict, "last_admin"},
		{"tự thu hồi", app.ErrTuThaoTacChinhMinh, http.StatusForbidden, "self_target_forbidden"},
		{"không tìm thấy", idstore.ErrCanBoKhongTonTai, http.StatusNotFound, "staff_not_found"},
		{"thiếu lý do", domain.ErrRevokeReasonMissing, http.StatusBadRequest, "invalid_request"},
		{"lý do quá dài", domain.ErrRevokeReasonTooLong, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range cases {
		m := dungMayChu(t)
		grantRevokeOnly(t, m)
		m.taiKhoan.loi = c.err

		w := m.goi(t, http.MethodDelete, hostA, revokePath, revokeBody, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.name, w.Code, c.status, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != c.code {
			t.Errorf("%s: code = %q, muốn %q", c.name, e.Code, c.code)
		}
	}
}

// The route must not swallow, or be swallowed by, POST .../account: same path, different method,
// different key. A checker granting ONLY `admin.user.revoke` must not reach the issue route.
func TestRevokePermissionDoesNotOpenIssueRoute(t *testing.T) {
	m := dungMayChu(t)
	grantRevokeOnly(t, m)

	doiMa(t, m.goi(t, http.MethodPost, hostA, revokePath, "", m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
}
