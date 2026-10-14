package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The commune's OWN role writes (owner decision 15/10/2026; the rules are on app.RoleCatalogue):
//
//	POST /api/v1/roles                 201 vaiTroMucRa
//	PUT  /api/v1/roles/{id}            200 vaiTroMucRa
//	POST /api/v1/roles/{id}/deletion   201 roleDeletionOut
//
// THE RESPONSE OF CREATE AND EDIT IS ONE ITEM OF GET /api/v1/roles (vaiTroMucRa), so the screen
// inserts or replaces a column without a second read and without a second shape to keep in step.

// roleWriteIn is the body of create and edit. POINTERS SO AN ABSENT FIELD IS REFUSED rather than read
// as "" / false — an edit that forgot `is_leader` would otherwise silently take the flag away (the
// argument luuPhanQuyenVao makes for its list).
type roleWriteIn struct {
	Name     *string `json:"name"`      // tên vai trò, 1–100 ký tự
	IsLeader *bool   `json:"is_leader"` // vai trò lãnh đạo — chỉ chọn màn hình mở đầu, không cấp quyền gì
}

// roleDeletionIn is the body of the deletion: the reason is mandatory (rule 7, invariant 1).
type roleDeletionIn struct {
	Reason string `json:"reason"` // lý do xoá, bắt buộc, tối đa 500 ký tự
}

// roleDeletionOut is what the deletion wrote — the precedent of service-platform's
// POST /api/v1/communes/{id}/deletion: the role is no longer readable through GET /api/v1/roles the
// moment this answers, so the record of the act is the response.
type roleDeletionOut struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy string    `json:"deleted_by"` // mã cán bộ người xoá
}

// readRoleWriteIn decodes and checks the two mandatory fields. false = already answered.
func readRoleWriteIn(w http.ResponseWriter, r *http.Request) (app.RoleInput, bool) {
	var in roleWriteIn
	if !docThanCanBo(w, r, &in) {
		return app.RoleInput{}, false
	}
	if in.IsLeader == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu trường is_leader: phải gửi rõ vai trò có phải vai trò lãnh đạo hay không.", "")
		return app.RoleInput{}, false
	}
	name := ""
	if in.Name != nil {
		name = *in.Name
	}
	return app.RoleInput{Name: name, IsLeader: *in.IsLeader}, true
}

// AddRole serves POST /api/v1/roles.
func (h *Handler) AddRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	in, ok := readRoleWriteIn(w, r)
	if !ok {
		return
	}
	role, err := h.d.Roles.Add(r.Context(), in, actor)
	if err != nil {
		h.writeRoleError(w, r, err)
		return
	}
	vietJSON(w, http.StatusCreated, vaiTroMucRaNgoai(role))
}

// EditRole serves PUT /api/v1/roles/{id}.
func (h *Handler) EditRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	in, ok := readRoleWriteIn(w, r)
	if !ok {
		return
	}
	role, err := h.d.Roles.Edit(r.Context(), r.PathValue("id"), in, actor)
	if err != nil {
		h.writeRoleError(w, r, err)
		return
	}
	vietJSON(w, http.StatusOK, vaiTroMucRaNgoai(role))
}

// RemoveRole serves POST /api/v1/roles/{id}/deletion.
func (h *Handler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in roleDeletionIn
	if !docThanCanBo(w, r, &in) {
		return
	}
	del, err := h.d.Roles.Remove(r.Context(), r.PathValue("id"), in.Reason, actor)
	if err != nil {
		h.writeRoleError(w, r, err)
		return
	}
	vietJSON(w, http.StatusCreated, roleDeletionOut{
		ID: del.Role.ID, Code: del.Role.Ma, Name: del.Role.Ten,
		DeletedAt: del.DeletedAt.UTC(), DeletedBy: del.DeletedBy,
	})
}

// writeRoleError maps one refusal of the three role writes onto a status and a sentence. ONE function
// for the three routes, so the same refusal cannot answer differently on two of them.
func (h *Handler) writeRoleError(w http.ResponseWriter, r *http.Request, err error) {
	var inUse *app.RoleInUseError
	switch {
	case errors.Is(err, idstore.ErrVaiTroKhongTonTaiDeGhi):
		// 404 for an invented id, a soft-deleted role and ANOTHER COMMUNE'S role alike — every read is
		// scoped, so the three are one answer and none can be told apart by trying (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "role_not_found", "Không tìm thấy vai trò.", "")
	case errors.Is(err, app.ErrSystemRole):
		httpx.WriteError(w, http.StatusConflict, "system_role",
			"Vai trò Quản trị hệ thống do hệ thống tạo và giữ mọi quyền quản trị của xã: không đổi tên, không đổi cờ lãnh đạo và không xoá được.", "")
	case errors.As(err, &inUse):
		httpx.WriteError(w, http.StatusConflict, "role_in_use",
			"Còn "+strconv.Itoa(inUse.Holders)+" cán bộ đang giữ vai trò này. "+
				"Hãy chuyển các cán bộ đó sang vai trò khác trên màn hình Người dùng trước, rồi xoá lại.", "")
	case errors.Is(err, domain.ErrRoleNameTaken):
		httpx.WriteError(w, http.StatusConflict, "duplicate_name",
			"Xã đã có một vai trò cùng tên (không phân biệt chữ hoa, chữ thường). Hãy đặt tên khác.", "")
	case errors.Is(err, domain.ErrRoleNameInvalid):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_name",
			"Tên vai trò phải có từ 1 đến "+strconv.Itoa(domain.MaxRoleName)+
				" ký tự, có ít nhất một chữ cái hoặc chữ số và không chứa ký tự điều khiển.", "")
	case errors.Is(err, domain.ErrRoleDeleteReasonInvalid):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_reason",
			"Hãy nhập lý do xoá vai trò (tối đa "+strconv.Itoa(domain.MaxRoleDeleteReason)+" ký tự).", "")
	case errors.Is(err, app.ErrRoleCatalogueFull):
		httpx.WriteError(w, http.StatusConflict, "role_limit_reached",
			"Xã đã có "+strconv.Itoa(idstore.TranDanhMucVaiTro)+" vai trò — mức tối đa. Hãy xoá vai trò không còn dùng trước khi tạo vai trò mới.", "")
	case errors.Is(err, idstore.ErrRoleCodeTaken):
		// A concurrent template seed committed the same code first (the template seed takes no
		// catalogue lock), or every suffix is used. Nothing was written.
		httpx.WriteError(w, http.StatusConflict, "role_code_taken",
			"Danh mục vai trò của xã vừa được người khác thay đổi. Hãy tải lại trang rồi thử lại.", "")
	default:
		// The wrapped error carries the store failure and holds no personal data; it never reaches the
		// client (rule 3, forbidden #3).
		h.d.Log.Error("ghi vai trò: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
