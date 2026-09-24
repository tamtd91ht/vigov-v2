package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// GET /api/v1/staff-directory — the staff picker shared by Phản ánh, Nhiệm vụ, Thông báo and
// Văn bản & đơn thư. Declared AnyAuthenticated in routes.go, with the user's approval and reason.

// canBoChonNguoiRa is one pickable person on the wire.
//
// THE JSON NAMES ARE canBoTomTat's (`code`, `full_name`, `position`, `department_id`), so the web
// types of the register and of the picker line up field for field. WHAT IS ABSENT IS THE CONTRACT:
// no `id`, no `email`, no `phone`/`mobile`, no `has_account`/`active`. There is no field to put
// them in, here or in domain.CanBoChonNguoi, and the store does not select them.
//
// `code` IS THE IDENTIFIER A CLIENT SENDS BACK when it assigns work: every holder check compares the
// assignee against Principal.Ma, the staff business code — never the internal ULID.
type canBoChonNguoiRa struct {
	Code         string `json:"code"`
	FullName     string `json:"full_name"`
	Position     string `json:"position"`
	DepartmentID string `json:"department_id"` // "" when the person sits in no unit
}

// danhBaChonNguoiRa is an OBJECT, not a bare array — the same shape as danhSachBoPhanRa and for the
// same reason. NO next_cursor, NO has_more: the whole list or a refusal (idstore.TranDanhBaChonNguoi).
type danhBaChonNguoiRa struct {
	Items []canBoChonNguoiRa `json:"items"`
}

// DanhBaChonNguoi serves the picker. GET /api/v1/staff-directory
//
// ONE OPTIONAL FILTER, `unit` — a department id, the same parameter name and the same validation as
// GET /api/v1/staff, so a client filters both lists the same way. The commune is never read from the
// request (rule 1, forbidden #2).
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data or reading ACROSS communes.
// This is neither — names and positions of the commune's own staff, which already appear on every
// record they handle, read inside the commune the request arrived in.
func (h *Handler) DanhBaChonNguoi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	boPhan := thamSoLoc(r.URL.Query(), "unit")
	if !kiemBoPhanLoc(w, boPhan) {
		return
	}

	ds, err := h.d.ChonNguoi.ChonNguoi(ctx, boPhan)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuCanBoChonNguoi) {
			// REFUSED, NOT TRUNCATED: a picker missing a colleague sends work to the wrong person
			// with nothing on screen to show it. The log names the commune — the one thing an
			// operator can act on — and no person.
			h.d.Log.Error("danh bạ chọn người vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhBaChonNguoi)
		} else {
			// The wrapped error carries the store failure, never a name, and never reaches the
			// client (rule 3, forbidden #3).
			h.d.Log.Error("danh bạ chọn người: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...): `items` is [] on a commune with nobody pickable yet, never null.
	ra := danhBaChonNguoiRa{Items: make([]canBoChonNguoiRa, 0, len(ds))}
	for _, cb := range ds {
		ra.Items = append(ra.Items, chonNguoiRaNgoai(cb))
	}
	vietJSON(w, http.StatusOK, ra)
}

func chonNguoiRaNgoai(cb domain.CanBoChonNguoi) canBoChonNguoiRa {
	return canBoChonNguoiRa{Code: cb.Ma, FullName: cb.HoTen, Position: cb.ChucVu, DepartmentID: cb.BoPhanID}
}
