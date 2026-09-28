package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

// The unit check's two refusals, on both routes that write a unit: creation and the assignment act.
// 400 with one sentence that echoes no id; 503 when identity could not be asked.
func TestOrgUnitRefusalsMapOnBothRoutes(t *testing.T) {
	for _, route := range []struct {
		name string
		perm authz.Perm
		path string
		body any
	}{
		{"giao việc mới", "task.create", duongTasks, thanTaoNV()},
		{"giao lại", "task.assign", taskAssignmentPath(maNVThu), taskAssignmentIn{Unit: strPtr("bp-x")}},
	} {
		for _, c := range []struct {
			name string
			err  error
			want int
			code string
		}{
			{"không sống", app.ErrOrgUnitNotLive, http.StatusBadRequest, "invalid_request"},
			{"chưa kiểm", fmt.Errorf("%w: %w", app.ErrOrgUnitUnchecked, errors.New("unavailable")),
				http.StatusServiceUnavailable, "assignee_check_unavailable"},
		} {
			t.Run(route.name+"/"+c.name, func(t *testing.T) {
				m := dungMayChu(t)
				m.capQuyen(t, route.perm)
				m.ghiNhiemVu.loi = bocNhuApp(c.err)

				w := m.goiGhiNV(t, http.MethodPost, hostA, route.path, canBoCuaXa(xaA), route.body)
				doiMa(t, w, c.want)
				e := loiTra(t, w)
				if e.Code != c.code || !strings.Contains(e.Message, "Bộ phận") && !strings.Contains(e.Message, "bộ phận") {
					t.Errorf("lỗi = %q / %q", e.Code, e.Message)
				}
				if strings.Contains(w.Body.String(), "bp-x") || strings.Contains(w.Body.String(), string(xaBocThu)) {
					t.Errorf("thân lộ mã bộ phận hoặc mã xã: %s", w.Body.String())
				}
			})
		}
	}
}
