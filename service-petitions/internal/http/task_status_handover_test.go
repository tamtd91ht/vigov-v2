package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// POST /api/v1/tasks/{ma}/status with the OPTIONAL `handover` and `attachments` (user decision
// 07/10/2026). The rules themselves are proved over the real store in
// internal/app/task_status_handover_test.go; what this layer can get wrong is the gate, which facts are
// handed down, the body's shape, and how the new refusals map.

func extendedStatusBody() doiTrangThaiVao {
	return doiTrangThaiVao{
		Status: string(domain.ChoDuyet), Note: "Đã xong phần hồ sơ.",
		Handover:    &taskAssignmentIn{Unit: strPtr("bp-dia-chinh"), Assignee: strPtr("CB-00999"), Note: "Theo giao ban."},
		Attachments: []string{"01JTEPA0000000000000000001"},
	}
}

// Rule 5, invariant 7, for the EXTENDED body. The gate is still `task.read`: `task.assign` alone opens
// nothing — it is a fact consulted inside, never a second door.
func TestStatusHandoverRoute_PermissionMatrix(t *testing.T) {
	t.Run("401 không phiên", func(t *testing.T) {
		m := dungMayChu(t)
		w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), nil, extendedStatusBody())
		doiMa(t, w, http.StatusUnauthorized)
		if m.ghiNhiemVu.goi != 0 {
			t.Errorf("đã chạm dữ liệu (%d lần)", m.ghiNhiemVu.goi)
		}
	})
	for _, wrong := range []authz.Perm{"task.assign", "task.approve"} {
		t.Run("403 chỉ có "+string(wrong), func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, wrong)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA), extendedStatusBody())
			doiMa(t, w, http.StatusForbidden)
			if m.ghiNhiemVu.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù sai quyền (%d lần)", m.ghiNhiemVu.goi)
			}
		})
	}
	// Right keys, WRONG COMMUNE: 401, as every route of this register answers it (authz.xacNhanXa
	// compares the communes before any permission — TestTuyenGhiNhiemVuDungQuyenSaiXaThi401).
	t.Run("đúng quyền sai xã", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, "task.read", "task.assign")
		w := m.goiGhiNV(t, http.MethodPost, hostB, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA), extendedStatusBody())
		doiMa(t, w, http.StatusUnauthorized)
		if m.ghiNhiemVu.goi != 0 {
			t.Errorf("đã ghi vào xã B bằng phiên xã A (%d lần)", m.ghiNhiemVu.goi)
		}
	})
	t.Run("200 đúng quyền đúng xã", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, "task.read")
		w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA), extendedStatusBody())
		doiMa(t, w, http.StatusOK)
		g := m.ghiNhiemVu
		if g.goi != 1 || g.xa != xaA {
			t.Fatalf("gọi %d lần trong xã %q", g.goi, g.xa)
		}
		if g.nguoi.ID != maCanBo || g.nguoi.Kind != "staff" {
			t.Errorf("chủ thể = %+v, muốn mã cán bộ %q", g.nguoi, maCanBo)
		}
	})
}

// The body reaches the use case intact, and the `task.assign` fact is read from THAT key.
func TestStatusHandoverRoute_HandsDownBodyAndAssignFact(t *testing.T) {
	for _, c := range []struct {
		name  string
		perms []authz.Perm
		want  bool
	}{
		{"task.read", []authz.Perm{"task.read"}, false},
		{"task.read + task.update", []authz.Perm{"task.read", "task.update"}, false},
		{"task.read + task.assign", []authz.Perm{"task.read", "task.assign"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, c.perms...)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA), extendedStatusBody())
			doiMa(t, w, http.StatusOK)
			yc := m.ghiNhiemVu.ycTrangT
			if bool(yc.AssignRight) != c.want {
				t.Errorf("sự thật task.assign = %v, muốn %v", yc.AssignRight, c.want)
			}
			h := yc.Handover
			if h == nil || h.Change.Unit == nil || *h.Change.Unit != "bp-dia-chinh" ||
				h.Change.Assignee == nil || *h.Change.Assignee != "CB-00999" || h.Note != "Theo giao ban." {
				t.Fatalf("giao việc tới use case = %+v", h)
			}
			if len(yc.Attachments) != 1 || yc.Attachments[0] != "01JTEPA0000000000000000001" {
				t.Errorf("tệp tới use case = %v", yc.Attachments)
			}
		})
	}
}

// Neither field: the request the use case sees is the one it saw before — no handover, no files, and
// the `task.assign` fact not even asked.
func TestStatusHandoverRoute_PlainBodyUnchanged(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.read", "task.assign")
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
		doiTrangThaiVao{Status: string(domain.ChoDuyet)})
	doiMa(t, w, http.StatusOK)
	yc := m.ghiNhiemVu.ycTrangT
	if yc.Handover != nil || yc.Attachments != nil || yc.AssignRight {
		t.Errorf("yêu cầu thường mang thêm %+v", yc)
	}
}

// `lead_unit` / `monitor` inside `handover` are refused as POST …/assignment refuses them.
func TestStatusHandoverRoute_RetiredKeysInsideHandoverRefused(t *testing.T) {
	for _, body := range []string{
		`{"status":"cho-duyet","handover":{"unit":"bp-dia-chinh","lead_unit":"bp-x"}}`,
		`{"status":"cho-duyet","handover":{"Monitor":"CB-00412"}}`,
		`{"status":"cho-duyet","handover":"bp-dia-chinh"}`,
	} {
		m := dungMayChu(t)
		m.capQuyen(t, "task.read")
		r := httptest.NewRequest(http.MethodPost, "https://"+hostA+duongTrangThaiNV(maNVThu), bytes.NewBufferString(body))
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
		r = r.WithContext(authz.Into(r.Context(), *canBoCuaXa(xaA)))
		w := httptest.NewRecorder()
		m.h.ServeHTTP(w, r)
		doiMa(t, w, http.StatusBadRequest)
		if m.ghiNhiemVu.goi != 0 {
			t.Errorf("%s: use case vẫn được gọi", body)
		}
	}
}

func TestStatusHandoverRoute_RefusalsMap(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		code int
		key  string
	}{
		{"không có quyền giao", domain.ErrHandoverNotAllowed, http.StatusForbidden, "forbidden"},
		{"cán bộ không nhận việc", app.ErrAssignmentStaffInvalid, http.StatusBadRequest, "invalid_request"},
		{"identity không trả lời", app.ErrAssignmentStaffUnchecked, http.StatusServiceUnavailable, "assignee_check_unavailable"},
		{"nhiệm vụ đã hoàn thành", domain.ErrTaskClosedForAssignment, http.StatusConflict, "task_state"},
		{"tệp không dùng được", domain.ErrAttachmentNotUsable, http.StatusBadRequest, "invalid_request"},
		{"bộ phận không nhận việc", app.ErrOrgUnitNotLive, http.StatusBadRequest, "invalid_request"},
		// A refusal the move already had still answers as before.
		{"không giữ việc", domain.ErrStatusNeedsHolder, http.StatusForbidden, "forbidden"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.read")
			m.ghiNhiemVu.loi = bocNhuApp(c.err)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA), extendedStatusBody())
			doiMa(t, w, c.code)
			e := loiTra(t, w)
			if e.Code != c.key {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, c.key)
			}
			if strings.Contains(w.Body.String(), xaBocThu) {
				t.Errorf("mã xã lọt ra thân: %s", w.Body.String())
			}
		})
	}
}
