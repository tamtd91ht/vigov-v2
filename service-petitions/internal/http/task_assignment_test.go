package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for POST /api/v1/tasks/{ma}/assignment (owner decision 28/09/2026).
//
//	PROVED HERE   the four cases of rule 5 invariant 7 — in the shared table caCacTuyenGhiNhiemVu
//	              (401 · 403 with `task.update` · 401 right key wrong commune · 200) · the body reaches
//	              the use case with "absent" and "" kept apart · every refusal maps to its status and
//	              code with no commune id or wrapped chain on the wire · the status route answers 400
//	              for `chuyen-tiep`, naming the assignment act.
//
//	NOT PROVED    the transaction, the status reset and the identity check — internal/app.

func strPtr(s string) *string { return &s }

func taskAssignmentPath(ma string) string { return duongNV(ma) + "/assignment" }

func TestReassignTask_BodyReachesUseCaseKeepingAbsentApartFromEmpty(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.assign"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, taskAssignmentPath(maNVThu), canBoCuaXa(xaA),
		taskAssignmentIn{Assignee: strPtr(""), Note: "Theo giao ban."})

	doiMa(t, w, http.StatusOK)
	got := m.ghiNhiemVu.ycAssignment
	if m.ghiNhiemVu.maDa != maNVThu {
		t.Errorf("mã nhiệm vụ tới use case = %q", m.ghiNhiemVu.maDa)
	}
	if got.Change.Unit != nil {
		t.Error("trường không gửi lại tới use case như một giá trị — sẽ xoá bộ phận không ai yêu cầu")
	}
	if got.Change.Assignee == nil || *got.Change.Assignee != "" {
		t.Error("`assignee: \"\"` (để bộ phận phân công) không tới use case như một lần XOÁ")
	}
	if got.Note != "Theo giao ban." {
		t.Errorf("use case nhận %+v", got)
	}
}

func TestReassignTask_ErrorsMapToStatusAndCode(t *testing.T) {
	const commune = "01JXBOCTHUAAAAAAAAAAAAAAAA"
	for _, c := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"cán bộ không nhận được việc", app.ErrAssignmentStaffInvalid, http.StatusBadRequest, "invalid_request"},
		{"chưa kiểm được cán bộ", fmt.Errorf("%w: %w", app.ErrAssignmentStaffUnchecked,
			errors.New("rpc error: code = Unavailable")), http.StatusServiceUnavailable, "assignee_check_unavailable"},
		{"không có gì đổi", domain.ErrAssignmentNoChange, http.StatusConflict, "no_change"},
		{"nhiệm vụ đã kết thúc", domain.ErrTaskClosedForAssignment, http.StatusConflict, "task_state"},
		{"thân rỗng", domain.ErrAssignmentEmpty, http.StatusBadRequest, "invalid_request"},
		{"unit trống", domain.ErrAssignmentUnitEmpty, http.StatusBadRequest, "invalid_request"},
		{"không có nhiệm vụ", petstore.ErrNhiemVuKhongTonTai, http.StatusNotFound, ""},
		{"đua ghi", petstore.ErrNhiemVuDaChuyenTrang, http.StatusConflict, "task_state"},
		{"lỗi hệ thống", errors.New("kết nối cơ sở dữ liệu hỏng"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.assign"))
			// Wrapped the way app.bocNhiemVu wraps it: operation + commune id, which must NOT reach the wire.
			m.ghiNhiemVu.loi = fmt.Errorf("nhiem_vu: giao lại nhiệm vụ cho xã %s: %w", commune, c.err)

			w := m.goiGhiNV(t, http.MethodPost, hostA, taskAssignmentPath(maNVThu), canBoCuaXa(xaA),
				taskAssignmentIn{Unit: strPtr("bp-dia-chinh")})

			doiMa(t, w, c.status)
			if c.code != "" {
				if e := loiTra(t, w); e.Code != c.code {
					t.Errorf("mã lỗi = %q, muốn %q", e.Code, c.code)
				}
			}
			body := w.Body.String()
			for _, leak := range []string{commune, "cho xã", "rpc error", "nhiem_vu:"} {
				if strings.Contains(body, leak) {
					t.Errorf("thân lỗi lộ %q: %s", leak, body)
				}
			}
		})
	}
}

// ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: `chuyen-tiep` on the status route was a legal move; it is now a 400
// whose sentence names the assignment act, so the web knows where to send the user.
func TestStatusRouteForwardingAnswers400NamingAssignment(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"), authz.Perm("task.update"))
	m.ghiNhiemVu.loi = fmt.Errorf("nhiem_vu: chuyển trạng thái cho xã x: %w", domain.ErrForwardingIsAssignment)

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
		doiTrangThaiVao{Status: string(domain.ChuyenTiep)})

	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); !strings.Contains(e.Message, "/assignment") {
		t.Errorf("câu từ chối không chỉ tới thao tác giao lại: %q", e.Message)
	}
}
