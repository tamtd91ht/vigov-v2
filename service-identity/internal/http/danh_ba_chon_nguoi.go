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
// TWO OPTIONAL FILTERS, both ANDed onto the picker predicate — neither can widen the list. The
// commune is never read from the request (rule 1, forbidden #2).
//
//	unit        a department id, the same parameter name and validation as GET /api/v1/staff, so a
//	            client filters both lists the same way.
//	permission  one flat permission key (`task.extend`): only people who HOLD it in this commune.
//	            Owner decision 27/09/2026 — the "Lãnh đạo giao việc" box offers only `task.extend`
//	            holders. English and singular, the word rule 5 uses in (tenant_id, role, permission).
//
// `permission` ACCEPTS ONLY THE KEYS A PICKER USES (quyenLanhDaoGiaoViec, see kiemQuyenLoc) and
// answers 400 for every other value — malformed or merely not allowed. Never an empty list, and
// never the unfiltered list.
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

	quyen, ok := kiemQuyenLoc(w, r.URL.Query()["permission"])
	if !ok {
		return
	}

	ds, err := h.d.ChonNguoi.ChonNguoi(ctx, domain.LocChonNguoi{BoPhanID: boPhan, QuyenMa: quyen})
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

// quyenLanhDaoGiaoViec is THE WHOLE ALLOWLIST of the `permission` filter: the key behind the
// "Lãnh đạo giao việc" box on the task form (owner decision 27/09/2026; narrowed to exactly that by
// the main session the same day).
//
// WHY AN ALLOWLIST AND NOT ANY KEY: this route is AnyAuthenticated. A generic filter lets every
// account of the commune list WHO holds any right — `?permission=admin.user` would name the commune's
// administrators, the accounts most worth attacking — which is the role map `GET
// /api/v1/role-permissions` keeps behind `admin.role`. A key off this list is refused with 400, NOT
// answered with an empty list: an empty answer for some keys and names for others is itself an
// oracle on who holds what.
//
// ADDING A KEY IS A DELIBERATE REVIEW, not a convenience: it publishes that key's holders to every
// account of the commune. Add a named constant and a `case` in kiemQuyenLoc, with the screen that
// needs it and who approved it.
const quyenLanhDaoGiaoViec = "task.extend"

// kiemQuyenLoc validates the `permission` filter, answering 400 itself. gt is the raw
// `r.URL.Query()["permission"]`: nil when absent — no filter — and otherwise exactly one value that
// must be a flat key ON THE ALLOWLIST (quyenLanhDaoGiaoViec).
//
// PRESENT BUT EMPTY (`?permission=`) IS REFUSED, NOT READ AS "NO FILTER": a client that meant to
// narrow to holders and sent nothing must not receive everybody (fail closed). Repeated
// (`?permission=a.b&permission=c.d`) is refused too — one key is the whole contract, and silently
// keeping the first would answer a question the client did not ask.
//
// IT TAKES THE []string, NOT url.Values, ON PURPOSE: tools/apidoc marks a parameter `required` when
// it appears in the condition of a branch that answers 400 inside a function reading url.Values.
// This parameter is optional; its 400 is for a malformed value, not for a missing one.
func kiemQuyenLoc(w http.ResponseWriter, gt []string) (string, bool) {
	if gt == nil {
		return "", true
	}
	if len(gt) != 1 || !domain.LaKhoaQuyenPhang(gt[0]) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Khoá quyền dùng để lọc không hợp lệ.", "")
		return "", false
	}
	switch gt[0] {
	case quyenLanhDaoGiaoViec:
		return gt[0], true
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Bộ lọc quyền chỉ hỗ trợ các khoá quyền mà một ô chọn người đang dùng.", "")
		return "", false
	}
}

func chonNguoiRaNgoai(cb domain.CanBoChonNguoi) canBoChonNguoiRa {
	return canBoChonNguoiRa{Code: cb.Ma, FullName: cb.HoTen, Position: cb.ChucVu, DepartmentID: cb.BoPhanID}
}
