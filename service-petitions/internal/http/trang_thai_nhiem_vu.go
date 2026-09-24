package http

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The two routes of a commune's task-status wording (open question #21, ADR 0035 §C):
//
//	GET   /api/v1/task-statuses          all seven, the commune's override merged over the default
//	PATCH /api/v1/task-statuses/{code}   re-word or re-order one
//
// NO POST, NO DELETE, NO `Tắt`. #21 closed the code list and this group has no disable. "Back to the
// default" is a PATCH carrying the default wording and order, which the response hands the client
// in `default_label` / `default_order`.

// trangThaiNhiemVuRa is one status as it leaves the API.
//
// `order` AND NOT `position`: the five other catalogue routes of this system answer `order` for the
// same `thu_tu` column, and the domain's validation message already names `order`. One shared
// concept, one field name across routes.
//
// THIS RESPONSE DOES CARRY `order` BESIDE THE ORDER OF `items`, unlike task-types, because here the
// configuration screen needs the number AND the default number to offer "về mặc định" — and the
// order of `items` is still the contract a Kanban board renders.
type trangThaiNhiemVuRa struct {
	// Code is the status code — `cho-duyet`. What a task record holds; immutable (#21).
	Code string `json:"code"`
	// Label is the commune's wording when it has set one, else the default.
	Label string `json:"label"`
	// Order is the effective position, 1 first.
	Order int `json:"order"`
	// Role is `chinh` (a Kanban column) or `re-nhanh` (`tam-dung` / `chuyen-tiep`, no column — §4.1).
	// Vietnamese values without diacritics (ADR 0011).
	Role string `json:"role"`
	// DefaultLabel / DefaultOrder are what the software ships (docs/ui-ux/02-nhiem-vu.md §6).
	DefaultLabel string `json:"default_label"`
	DefaultOrder int    `json:"default_order"`
	// Customised is DERIVED: label or order differs from the default. Not "a row exists" — see
	// domain.TrangThaiHienThi.DaTuyChinh.
	Customised bool `json:"customised"`
}

// danhSachTrangThaiNhiemVuRa — ALWAYS SEVEN ITEMS, in effective order (ties broken by default
// order). No cursor, no has_more: a closed set of seven is never paged.
type danhSachTrangThaiNhiemVuRa struct {
	Items []trangThaiNhiemVuRa `json:"items"`
}

func trangThaiNhiemVuRaNgoai(tt domain.TrangThaiHienThi) trangThaiNhiemVuRa {
	return trangThaiNhiemVuRa{
		Code: string(tt.Ma), Label: tt.Nhan, Order: tt.ThuTu, Role: string(tt.VaiTro),
		DefaultLabel: tt.NhanMacDinh, DefaultOrder: tt.ThuTuMacDinh, Customised: tt.DaTuyChinh,
	}
}

// DanhSachTrangThaiNhiemVu serves all seven statuses. GET /api/v1/task-statuses
//
// NO AUDIT ENTRY: not full personal data, not a cross-commune read (rule 6, invariant 7).
func (h *Handler) DanhSachTrangThaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ghiDe, err := h.d.TrangThaiNhiemVu.DanhSach(ctx)
	if err == nil {
		var gop []domain.TrangThaiHienThi
		if gop, err = domain.GopNhanTrangThai(ghiDe); err == nil {
			ra := danhSachTrangThaiNhiemVuRa{Items: make([]trangThaiNhiemVuRa, 0, len(gop))}
			for _, tt := range gop {
				ra.Items = append(ra.Items, trangThaiNhiemVuRaNgoai(tt))
			}
			vietJSON(w, http.StatusOK, ra)
			return
		}
	}
	// REFUSED rather than falling back to the defaults. A commune that re-worded its statuses and is
	// shown the shipped wording, silently, reads a board that does not match its own practice — and
	// nothing on the screen says so. The wrapped error never reaches the client (rule 3, forbidden #3).
	h.d.Log.Error("trạng thái nhiệm vụ: lỗi đọc nhãn của xã — TỪ CHỐI thay vì trả mặc định",
		"xa", string(tenant.MustFrom(ctx)), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// suaTrangThaiNhiemVuVao is the body of PATCH /api/v1/task-statuses/{code}.
//
// POINTERS: absent (or null) means unchanged. `omitempty` so tools/apidoc marks them optional.
//
// `code` AND `active` ARE HERE ONLY TO BE REFUSED. A client sending `code` believes it could
// rename a status (#21: it cannot); one sending `active` believes it could switch one off (ADR 0035
// §C: this group has no `Tắt`). A field silently dropped is a client that thinks it set something.
type suaTrangThaiNhiemVuVao struct {
	Label *string `json:"label,omitempty"`
	Order *int    `json:"order,omitempty"`

	Code   *string `json:"code,omitempty"`
	Active *bool   `json:"active,omitempty"`
}

// SuaTrangThaiNhiemVu re-words or re-orders one status. PATCH /api/v1/task-statuses/{code}
func (h *Handler) SuaTrangThaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao suaTrangThaiNhiemVuVao
	if !docThan(w, r, &vao) {
		return
	}
	if vao.Code != nil {
		h.traLoiLoiGhi(w, r, "sửa nhãn trạng thái", domain.ErrMaBatBien)
		return
	}
	if vao.Active != nil {
		h.traLoiLoiGhi(w, r, "sửa nhãn trạng thái", domain.ErrKhongTatDuocTrangThai)
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi nhãn trạng thái chạy mà không có chủ thể hoặc mã cán bộ — SAI CẤU HÌNH",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	sau, err := h.d.GhiTrangThaiNhiemVu.Sua(r.Context(), r.PathValue("code"), app.YeuCauSuaNhanTrangThai{
		Nhan:  vao.Label,
		ThuTu: vao.Order,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhi(w, r, "sửa nhãn trạng thái", err)
		return
	}
	vietJSON(w, http.StatusOK, trangThaiNhiemVuRaNgoai(sau))
}
