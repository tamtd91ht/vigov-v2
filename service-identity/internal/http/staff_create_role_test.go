package http

import (
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// POST /api/v1/staff with `role_ids` (owner decision 10/10/2026). Rule 5 invariant 7's four cases for
// this body run in can_bo_ghi_test.go over moiTuyenGhi; the guards themselves (#14, the role being
// this commune's, one transaction) are proven in app/staff_create_role_test.go. What is asserted here
// is what only the handler owns: the field reaches the use case, and each new refusal has its status.

func createStaffRoute(body string) tuyenGhi {
	return tuyenGhi{"thêm cán bộ", "POST", "/api/v1/staff", body, http.StatusCreated}
}

func TestCreateStaffForwardsRoleIDs(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhi(t, createStaffRoute(`{"full_name":"Trần Thị B","mobile":"0900000000","role_ids":["vt-002"]}`),
		hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusCreated)
	if got := m.ghiDanhBa.themCuoi.RoleIDs; len(got) != 1 || got[0] != "vt-002" {
		t.Errorf("role_ids tới use case = %v", got)
	}

	// ABSENT = no role, and the old body keeps working unchanged.
	m = dungMayChu(t)
	doiMa(t, m.goiGhi(t, createStaffRoute(`{"full_name":"Trần Thị B"}`), hostA, m.tokenCho(t, xaA, sidA)),
		http.StatusCreated)
	if got := m.ghiDanhBa.themCuoi.RoleIDs; len(got) != 0 {
		t.Errorf("không gửi role_ids mà use case nhận %v", got)
	}
}

// EACH NEW REFUSAL REACHES THE CLIENT AS ITS OWN STATUS AND CODE.
//
//	400 invalid_request        two roles; a mobile past ten digits — the domain's own sentence
//	400 role_not_found         a role that is not this commune's (another's, invented, deleted)
//	403 permission_escalation  #14 second — the role carries a key the caller does not hold
func TestCreateStaffRoleAndMobileRefusalsMapToStatus(t *testing.T) {
	for _, c := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"two roles", domain.ErrOneRolePerStaff, http.StatusBadRequest, "invalid_request"},
		{"mobile over ten digits", domain.ErrMobileTooManyDigits, http.StatusBadRequest, "invalid_request"},
		{"role not in commune", app.ErrVaiTroKhongTonTai, http.StatusBadRequest, "role_not_found"},
		{"stronger role", &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm"}}, http.StatusForbidden, "permission_escalation"},
	} {
		m := dungMayChu(t)
		m.ghiDanhBa.loi = c.err
		w := m.goiGhi(t, createStaffRoute(`{"full_name":"Trần Thị B","role_ids":["vt-002"]}`), hostA, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — %s", c.name, w.Code, c.status, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != c.code {
			t.Errorf("%s: code = %q, muốn %q", c.name, e.Code, c.code)
		}
	}
}
