package http

// The WRITE routes of this service's two reference catalogues — `residential-unit-types` and
// `task-blocs` — POST · PATCH · DELETE, all under `admin.lookup` (user decision 2026-09-24).
//
// THE CONTRACT IS THE SIBLINGS', FIELD FOR FIELD AND STATUS FOR STATUS
// (service-petitions/internal/http/loai_nhiem_vu.go + danh_muc_ghi.go, and the same shape in
// documents, finance and comms), so the admin web drives all seven catalogues with one piece of
// generic code:
//
//	POST   body  code · label · order? · is_default?          201 row   (idem.Required MoKhiHong)
//	PATCH  body  label? · order? · active? · is_default?      200 row   (idem.KhongCan)
//	DELETE body  reason                                       204       (idem.KhongCan)
//	any    body  naming `source` or `tier`                    400 invalid_request
//	PATCH  body  naming `code`                                400 invalid_request
//	404 not_found · 409 code_taken · catalogue_full · system_row · code_branch_row · 500 internal
//
// ONE HANDLER BODY PER VERB FOR BOTH CATALOGUES (the generic functions below), for the reason the
// sibling gives for its single error mapping: two copies drift, and the copy that drifts answers 500
// where it meant 409.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// thanDanhMucToiDa bounds a write body: four short fields.
const thanDanhMucToiDa = 64 << 10

// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC: tools/apidoc marks a field REQUIRED
// in the generated contract unless it carries `omitempty`, so without it the contract would tell
// every client that `source` and `tier` MUST be sent — on routes that answer 400 to exactly that.

// themDanhMucVao is the body of POST /api/v1/residential-unit-types and POST /api/v1/task-blocs.
//
// `Source` AND `Tier` ARE HERE ONLY SO THEY CAN BE REFUSED. They never reach the store, which writes
// `nguon` and `ma_nguon_re_nhanh` as literals. Unknown fields are IGNORED (a screen posting back a
// row carries `id`); the fields that must not be silently dropped are the ones named here.
type themDanhMucVao struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Order     int    `json:"order,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`

	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// suaDanhMucVao is the body of PATCH …/{id}. EVERY EDITABLE FIELD IS A POINTER: order 0, active
// false and is_default false are real values a plain struct could not tell from "not mentioned".
//
// `Code` IS REFUSED, NOT IGNORED — an issued code is never renumbered (rule 7, invariant 3).
type suaDanhMucVao struct {
	Label     *string `json:"label,omitempty"`
	Order     *int    `json:"order,omitempty"`
	Active    *bool   `json:"active,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`

	Code   *string `json:"code,omitempty"`
	Source *string `json:"source,omitempty"`
	Tier   *int    `json:"tier,omitempty"`
}

// xoaDanhMucVao is the body of DELETE …/{id}. A DELETE WITH A BODY: the reason is mandatory (rule 7,
// invariant 1), and the query string would put free text about a government record into every access
// log. Same shape as every sibling catalogue and as this service's calendar deletes.
type xoaDanhMucVao struct {
	Reason string `json:"reason"`
}

func docThanDanhMuc(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanDanhMucToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil {
		// The decoder's own message quotes the input and never reaches the client (rule 3, forbidden #3).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// --- the six handlers --------------------------------------------------------------------------------

// ThemLoaiDonViDanCu — POST /api/v1/residential-unit-types
func (h *Handler) ThemLoaiDonViDanCu(w http.ResponseWriter, r *http.Request) {
	themDanhMuc(h, w, r, "loại đơn vị dân cư", h.d.GhiLoaiDonViDanCu.Them,
		func(l domain.LoaiDonViDanCu) (any, string) { return loaiDonViDanCuRaNgoai(l), l.Ma })
}

// SuaLoaiDonViDanCu — PATCH /api/v1/residential-unit-types/{id}
func (h *Handler) SuaLoaiDonViDanCu(w http.ResponseWriter, r *http.Request) {
	suaDanhMuc(h, w, r, "loại đơn vị dân cư", h.d.GhiLoaiDonViDanCu.Sua,
		func(l domain.LoaiDonViDanCu) any { return loaiDonViDanCuRaNgoai(l) })
}

// XoaLoaiDonViDanCu — DELETE /api/v1/residential-unit-types/{id}
func (h *Handler) XoaLoaiDonViDanCu(w http.ResponseWriter, r *http.Request) {
	xoaDanhMuc(h, w, r, "loại đơn vị dân cư", h.d.GhiLoaiDonViDanCu.Xoa)
}

// ThemKhoiNhiemVu — POST /api/v1/task-blocs
func (h *Handler) ThemKhoiNhiemVu(w http.ResponseWriter, r *http.Request) {
	themDanhMuc(h, w, r, "khối nhiệm vụ", h.d.GhiKhoiNhiemVu.Them,
		func(k domain.KhoiNhiemVu) (any, string) { return khoiNhiemVuRaNgoai(k), k.Ma })
}

// SuaKhoiNhiemVu — PATCH /api/v1/task-blocs/{id}
func (h *Handler) SuaKhoiNhiemVu(w http.ResponseWriter, r *http.Request) {
	suaDanhMuc(h, w, r, "khối nhiệm vụ", h.d.GhiKhoiNhiemVu.Sua,
		func(k domain.KhoiNhiemVu) any { return khoiNhiemVuRaNgoai(k) })
}

// XoaKhoiNhiemVu — DELETE /api/v1/task-blocs/{id}
func (h *Handler) XoaKhoiNhiemVu(w http.ResponseWriter, r *http.Request) {
	xoaDanhMuc(h, w, r, "khối nhiệm vụ", h.d.GhiKhoiNhiemVu.Xoa)
}

// --- the three shared bodies -------------------------------------------------------------------------

func themDanhMuc[T any](h *Handler, w http.ResponseWriter, r *http.Request, ten string,
	them func(context.Context, app.YeuCauThemDanhMuc, app.NguoiThucHien) (T, error),
	ra func(T) (any, string)) {

	var vao themDanhMucVao
	if !docThanDanhMuc(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE: provenance is not the caller's to set, whatever value it names.
	if vao.Source != nil || vao.Tier != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "thêm", domain.ErrNguonDoTuClient)
		return
	}
	// The trail carries the STAFF CODE, never the internal id (rule 6, invariant 8) — nguoiThucHienCanBo
	// refuses a principal without one rather than falling back.
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	moi, err := them(r.Context(), app.YeuCauThemDanhMuc{
		Ma: vao.Code, Nhan: vao.Label, ThuTu: vao.Order, LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "thêm", err)
		return
	}
	than, ma := ra(moi)
	// What a retry with the same Idempotency-Key is told about: THE CODE, not the body — Redis is a
	// cache, not a record store.
	idem.RecordCode(r.Context(), ma)
	vietJSON(w, http.StatusCreated, than)
}

func suaDanhMuc[T any](h *Handler, w http.ResponseWriter, r *http.Request, ten string,
	sua func(context.Context, string, app.YeuCauSuaDanhMuc, app.NguoiThucHien) (T, error),
	ra func(T) any) {

	var vao suaDanhMucVao
	if !docThanDanhMuc(w, r, &vao) {
		return
	}
	if vao.Source != nil || vao.Tier != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "sửa", domain.ErrNguonDoTuClient)
		return
	}
	if vao.Code != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "sửa", domain.ErrMaBatBien)
		return
	}
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	sau, err := sua(r.Context(), r.PathValue("id"), app.YeuCauSuaDanhMuc{
		Nhan: vao.Label, ThuTu: vao.Order, DangDung: vao.Active, LaMacDinh: vao.IsDefault,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, ra(sau))
}

// xoaDanhMuc — 204 AND NO BODY. The row is still there with its three soft-delete columns, but
// returning it would invite a client to render a row it has just taken off the screen.
func xoaDanhMuc(h *Handler, w http.ResponseWriter, r *http.Request, ten string,
	xoa func(context.Context, string, string, app.NguoiThucHien) error) {

	var vao xoaDanhMucVao
	if !docThanDanhMuc(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	if err := xoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhiDanhMuc(w, r, ten, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// traLoiLoiGhiDanhMuc maps one use-case failure onto a status and a sentence — the SAME codes the
// sibling catalogues answer (service-petitions/internal/http/danh_muc_ghi.go traLoiLoiGhi).
//
// 409 AND NOT 403 FOR A TIER REFUSAL: the caller holds `admin.lookup`; what is refused is this
// operation on THIS row, and a 403 would send an administrator to the Phân quyền screen for nothing.
func (h *Handler) traLoiLoiGhiDanhMuc(w http.ResponseWriter, r *http.Request, ten, viec string, err error) {
	switch {
	case errors.Is(err, idstore.ErrDanhMucKhongTonTai):
		// One answer for another commune's row, a soft-deleted one and an invented id (rule 4,
		// forbidden #2 applied to configuration).
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy mục danh mục này.", "")
	case errors.Is(err, idstore.ErrMaDaTonTai):
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"Mã này đã được dùng trong xã — kể cả khi dòng mang mã đó đã bị xoá. "+
				"Mã đã cấp thì không cấp lại. Hãy chọn một mã khác.", "")
	case errors.Is(err, idstore.ErrDanhMucDayTran):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			"Danh mục "+ten+" của xã đã đạt số mục tối đa. Hãy tắt hoặc xoá bớt mục không dùng.", "")
	case errors.Is(err, domain.ErrKhongXoaDuocMucHeThong):
		httpx.WriteError(w, http.StatusConflict, "system_row",
			"Mục do hệ thống cấp thì không xoá được. Hãy tắt mục đó thay vì xoá.", "")
	case errors.Is(err, domain.ErrKhongTatDuocMucReNhanh):
		httpx.WriteError(w, http.StatusConflict, "code_branch_row",
			"Phần mềm có xử lý riêng theo mã của mục này nên không tắt được. Có thể đổi nhãn hiển thị.", "")
	case app.LaLoiDauVaoDanhMuc(err):
		// The domain's own sentence names the field and the rule and holds no personal data. Every
		// input refusal is returned by the use case BEFORE its transaction opens, so it arrives
		// unwrapped by the table/commune context the use case adds to failures.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("danh mục "+ten+": "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
