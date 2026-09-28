package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// POST /api/v1/roles/defaults — seed the eight template roles of 14-cau-hinh.md §4.1 into the
// request's commune (user decision 2026-09-28; the rules it runs on are on app.RoleTemplateSeeder,
// the list itself on domain.RoleTemplates — not restated here, rule 9).
//
// 200 AND NOT 201, INCLUDING ON THE FIRST RUN, for the reason GieoSLAMacDinh gives: the request
// brings the commune's role catalogue to a known state rather than creating one addressable
// resource, so there is no single Location a 201 would owe.

// roleTemplateRefOut names one role: code and name only — nothing about who holds it.
type roleTemplateRefOut struct {
	Code string `json:"code"` // vai_tro.ma, e.g. "chu-tich-ubnd"
	Name string `json:"name"`
}

// seedRoleTemplatesOut is what one run did. THREE LISTS, always present, `[]` when empty.
type seedRoleTemplatesOut struct {
	Created         []roleTemplateRefOut `json:"created"`          // vai trò vừa tạo, kèm đủ quyền của mẫu
	SkippedExisting []roleTemplateRefOut `json:"skipped_existing"` // xã đã có vai trò mang mã này — giữ NGUYÊN, kể cả quyền
	SkippedDeleted  []roleTemplateRefOut `json:"skipped_deleted"`  // xã đã XOÁ vai trò mang mã này — không tạo lại, không khôi phục
}

func roleTemplateRefsOut(in []app.RoleTemplateRef) []roleTemplateRefOut {
	out := make([]roleTemplateRefOut, 0, len(in))
	for _, r := range in {
		out = append(out, roleTemplateRefOut{Code: r.Code, Name: r.Name})
	}
	return out
}

// SeedRoleTemplates serves POST /api/v1/roles/defaults.
func (h *Handler) SeedRoleTemplates(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}

	res, err := h.d.RoleTemplates.SeedDefaults(r.Context(), actor)
	if err != nil {
		var missing *app.LoiTraoQuyenKhongCam
		switch {
		case errors.As(err, &missing):
			// #14: the caller must hold every key the templates grant. The list is the point — it
			// tells the administrator exactly what to ask for. Permission keys are not personal data.
			httpx.WriteError(w, http.StatusForbidden, "permission_escalation",
				"Chỉ người đang giữ đủ mọi quyền mà bộ vai trò mẫu cấp mới gieo được bộ này. "+
					"Tài khoản của bạn còn thiếu: "+strings.Join(missing.Thieu, ", ")+
					". Chưa có vai trò nào được tạo. Hãy nhờ người có đủ quyền thực hiện.", "")
		case errors.Is(err, idstore.ErrRoleCodeTaken):
			// A concurrent run committed first. Nothing was half-written; retrying reports the roles
			// as already present.
			httpx.WriteError(w, http.StatusConflict, "role_exists",
				"Danh mục vai trò của xã vừa được người khác thay đổi. Hãy tải lại trang rồi thử lại.", "")
		default:
			// The wrapped error carries the store failure and never reaches the client (rule 3,
			// forbidden #3).
			h.d.Log.Error("gieo vai trò mẫu: lỗi hệ thống",
				"xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		}
		return
	}

	vietJSON(w, http.StatusOK, seedRoleTemplatesOut{
		Created:         roleTemplateRefsOut(res.Created),
		SkippedExisting: roleTemplateRefsOut(res.SkippedExisting),
		SkippedDeleted:  roleTemplateRefsOut(res.SkippedDeleted),
	})
}
