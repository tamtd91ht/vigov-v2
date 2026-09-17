package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/identity/internal/domain"
	idstore "github.com/vihat/vigov/identity/internal/store"
)

// The two read routes of the staff register. GET /api/v1/staff and GET /api/v1/staff/{id}.
//
// THE WRITE ROUTES ARE DELIBERATELY ABSENT, and no scaffolding for them is left here either. All
// of them are blocked on questions the customer has not answered — kb/00-foundation/
// open-questions.json #9 (how a new member of staff gets their first password), #10 (retiring
// somebody: lock or delete), #13 (whether a commune may remove its last administrator) and #14
// (whether admin.user may act on its own account). A half-written write path is worse than none:
// it looks like a decision somebody made.

// canBoTomTat is one staff record as it leaves the API. JSON field names are English
// (rest-api-design §1); the values are whatever the commune typed, in Vietnamese.
//
// WHAT IS NOT HERE IS PART OF THE CONTRACT: there is no password field of any kind, and there is
// no route by which one could appear — the store does not even SELECT mat_khau_hash, and
// domain.CanBoTomTat has nowhere to put it. That is three independent layers, which is what it
// takes for "a credential never leaves this service" to be a property rather than a habit.
type canBoTomTat struct {
	ID       string `json:"id"`   // ULID — what GET /api/v1/staff/{id} takes
	Code     string `json:"code"` // cb.Ma — the code the audit trail quotes
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Position string `json:"position"`

	// Ids, not labels. The screen already holds the department and role lists for its own filter
	// boxes; resolving the label here would mean a join, and the paged read walks one table.
	DepartmentID string `json:"department_id"`
	RoleID       string `json:"role_id"`

	// MASKED — 09****5678, never the number itself. See maskDienThoai.
	Phone string `json:"phone"`

	// HasAccount separates "this person can sign in" from "this account is not locked". They are
	// two questions (migration 0003), and a client that reads one for the other reports a
	// directory entry as an active account — a figure a commune sends upward.
	HasAccount bool `json:"has_account"`
	Active     bool `json:"active"`

	// Null means never signed in. The screen shows "Chưa đăng nhập" for it; a zero timestamp
	// would render as some date in year 1.
	LastLoginAt *time.Time `json:"last_login_at"`

	CreatedAt time.Time `json:"created_at"`
}

// trangCanBo is one page of the register.
//
// WHY THE SHAPE IS RESTATED HERE INSTEAD OF RETURNING page.Result[canBoTomTat] DIRECTLY: it is
// the same three fields, and it is a copy, which rule 9 would normally forbid. tools/apidoc
// resolves reply types through the AST and has no case for a generic instantiation
// (tools/apidoc/schema.go:303 — "kiểu %T chưa hỗ trợ"), so a generic reply type would drop this
// route out of kb/20-contracts/openapi.json in silence, and the admin web would then type the
// screen by guessing. The copy is pinned to its original by TestTrangCanBoTrungHinhDangVoiPage,
// which compares the two field by field with reflection — so the two cannot drift apart, which is
// the only thing rule 9 actually cares about.
type trangCanBo struct {
	Items      []canBoTomTat `json:"items"`
	NextCursor string        `json:"next_cursor"` // empty when has_more is false
	HasMore    bool          `json:"has_more"`
}

// maskDienThoai decides what a staff phone number looks like on the way out.
//
// IT IS MASKED, ALWAYS, FOR EVERY CALLER — and this is a FAIL-CLOSED DEFAULT, not a business
// rule somebody chose. Rule 3, invariant 3 lets full personal data out only to a caller holding
// an EXPLICIT full-view permission. Of the 33 permission keys seeded into `quyen` there is no
// key that means "see a staff member's full details": `admin.user` is "Quản lý người dùng" and
// `feedback.restricted` scopes the CONTENT of petitions, not personal data. With no key there is
// no explicit path, so there are only two reachable states — mask everyone, or show everyone.
//
// → THIS IS WAITING ON open-questions.json #11, which asks the customer exactly this and is
// still OPEN. Do not remove the mask because a screen looks incomplete. Masking first and opening
// later is one line; showing first and closing later does not take the number back out of the
// access logs, the Excel exports and the screens people have already seen.
//
// When #11 is answered with "some role may see it", the change is: a new permission key, a
// migration into `quyen`, a Checker call here, and an audit entry for the full read (rule 6,
// invariant 7 — reading FULL personal data is itself an audited event). Not a deleted line.
func maskDienThoai(so string) string { return privacy.MaskPhone(so) }

// raNgoai converts one record for the wire. IT IS THE ONLY EXIT, which is what makes the masking
// decision above enforceable: there is no second place that builds this shape.
//
// ho_ten, chuc_vu and the department are NOT masked. `admin.user` — "Quản lý người dùng" — is an
// explicit permission that guards exactly this screen (14-cau-hinh.md §12.8), and a register of
// staff whose names are masked is not a register of staff. `email` is not masked either: it is a
// work address issued by the authority, the same reading identity/internal/app/
// dang_nhap.go:81 takes when it explains that a staff work address is not citizen personal data
// (what it guards there is an UNAUTHENTICATED, client-supplied string, which this is not).
func raNgoai(cb domain.CanBoTomTat) canBoTomTat {
	return canBoTomTat{
		ID:           cb.ID,
		Code:         cb.Ma,
		FullName:     cb.HoTen,
		Email:        cb.Email,
		Position:     cb.ChucVu,
		DepartmentID: cb.BoPhanID,
		RoleID:       cb.VaiTroID,
		Phone:        maskDienThoai(cb.DienThoai),
		HasAccount:   cb.CoTaiKhoan,
		Active:       cb.DangHoatDong,
		LastLoginAt:  cb.DangNhapGanNhat,
		CreatedAt:    cb.TaoLuc,
	}
}

// DanhSachCanBo serves one page of the register. GET /api/v1/staff
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data; the phone number leaves here masked, so nothing full is read. An
// entry for every page of every list would bury the entries that carry legal weight — who
// changed a role, who locked an account — under thousands that carry none, and a trail nobody can
// read is a trail that answers no inspection.
func (h *Handler) DanhSachCanBo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The commune is fixed by httpx.TenantMiddleware from Host; page.Parse reads only
	// limit/cursor/sort/order, and a cursor carries no commune by construction. A client naming
	// its own commune is a client granting itself access — nothing here reads tenant_id from the
	// query string (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	yc, err := page.Parse(thamSo, idstore.SapXepCanBo)
	if err != nil {
		// page.HTTPError owns the mapping so all eight services answer a bad cursor the same way.
		// It never echoes what the client sent: a cursor is opaque, and a rejected sort key is
		// often a probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	kq, err := h.d.DanhBa.DanhSach(ctx, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry a name, an email or a
		// phone number, and it never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("danh sách cán bộ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on an empty commune, never
	// as null. A newly onboarded commune has an empty register, and a client that has to handle
	// both shapes handles one of them wrong.
	ra := trangCanBo{
		Items:      make([]canBoTomTat, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, cb := range kq.Items {
		ra.Items = append(ra.Items, raNgoai(cb))
	}
	vietJSON(w, http.StatusOK, ra)
}

// ChiTietCanBo serves one record. GET /api/v1/staff/{id}
//
// 404 FOR ANOTHER COMMUNE'S ID, NOT 403 — and it costs nothing to get right here, because the
// store cannot see the other commune's row at all: Scoped.Query binds the commune from the
// context to $1, so the id matches nothing and comes back as ErrCanBoKhongTonTai. An invented id,
// a soft-deleted person and another authority's staff member are one single answer, so none of
// them can be told apart by trying (rule 4, forbidden #2).
func (h *Handler) ChiTietCanBo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cb, err := h.d.DanhBa.ChiTiet(ctx, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			httpx.WriteError(w, http.StatusNotFound, "staff_not_found",
				"Không tìm thấy cán bộ.", "")
			return
		}
		h.d.Log.Error("chi tiết cán bộ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, raNgoai(cb))
}
