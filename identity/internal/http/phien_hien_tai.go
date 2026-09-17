package http

import (
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// The read route behind "who am I". GET /api/v1/sessions/current

// phienHienTaiRa is the caller's own session as it leaves the API.
//
// THE FIELDS THAT ARE ABSENT ARE THE DESIGN — the same discipline as phanHoiDangNhap
// (handler.go:37), for the same reasons. No session token and no refresh token: the token lives
// in an httpOnly cookie precisely so JavaScript cannot read it, and handing it back in a body
// would undo that in one line. No password hash: it is a credential and never leaves this
// service at all. No phone number and no email: a phone number is personal data under Decree
// 13/2023 (rule 3) and nothing on any screen this feeds needs either.
//
// Staff reuses canBoGon rather than declaring a second shape for the same three fields: the
// sign-in response and this one describe the same person on the same screens, and two shapes are
// two shapes that drift (rule 9).
type phienHienTaiRa struct {
	// Sid is this session, the one the client may end: DELETE /api/v1/sessions/{sid}. Safe to
	// hand to JavaScript — see the note on phanHoiDangNhap.Sid.
	Sid string `json:"sid"`

	// ExpiresAt comes from the session REGISTRY, not from the token's own claim. See
	// PhienHienTai.HetHanLuc.
	ExpiresAt time.Time `json:"expires_at"`

	Staff canBoGon `json:"staff"`

	// Permissions is the caller's own permission keys.
	//
	// WHY IT IS RETURNED AT ALL: docs/ui-ux/15-phu-luc-giao-dien-chung.md §1 requires the check in
	// TWO LAYERS — hide the menu entries and buttons a person cannot use (§2: "Ẩn mục menu mà
	// người dùng không có quyền đọc tương ứng") AND refuse at the server. This list feeds the
	// first layer, and asking 33 yes/no questions on every page load is how that layer otherwise
	// gets built.
	//
	// THE CLIENT LAYER IS A CONVENIENCE, NEVER A CONTROL (rule 5, forbidden #1). A client can be
	// modified, so this list is what the browser was TOLD, not what the caller is ALLOWED. Every
	// route still declares its own permission and store.Checker still reads the grants from the
	// database on every single request — the worst a tampered list achieves is a button that
	// answers 403.
	//
	// EACH ENTRY IS ONE FLAT KEY, "<nhóm>.<việc>" — `task.extend`, the same string the Phân quyền
	// screen shows and the `quyen` table stores (rule 5, invariant 3b). Never a (subsystem,
	// action) pair: these rights are not a Cartesian product, and `task.approve` — closing a
	// commitment made to a citizen — is deliberately not `task.extend`, which moves its deadline.
	//
	// Sorted, and never null: an account whose roles were all withdrawn gets [], which renders as
	// an app with no menu rather than as a client crash on `null.includes`.
	Permissions []string `json:"permissions"`
}

// XemPhienHienTai serves the caller's own session. GET /api/v1/sessions/current
//
// IT DESCRIBES THE CALLER AND NOBODY ELSE. There is no id in the path and no parameter to name a
// person: the identity comes from the session the request arrived on, which is what stops this
// being a route for reading other people's sessions (rule 4, invariant 2, applied to staff —
// the same reading as DangXuat).
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this reads neither. The only person
// described is the caller, with the three fields the sign-in response already returned — no
// phone number, no national id — and the commune is the one the request arrived in. An entry per
// page load would bury the entries that carry legal weight under thousands that carry none.
func (h *Handler) XemPhienHienTai(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ph, coPhien := PhienTu(ctx)
	p, coPrincipal := authz.From(ctx)
	if !coPhien || !coPrincipal {
		// authz.AnyAuthenticated has already refused requests with no principal; reaching here
		// means the middleware was mounted wrong, and answering 401 is the safe reading — the
		// same defensive branch as DangXuat.
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
			"Bạn chưa đăng nhập.", "")
		return
	}

	// The commune is NOT passed: it rides in the context down to the scoped query, so this can
	// only ever read grants made inside the commune the request arrived in (rule 1, invariant 4).
	quyen, err := h.d.Quyen.QuyenCua(ctx, p)
	if err != nil {
		// 500 AND NOT AN EMPTY LIST. Empty is a legitimate answer — an account whose roles were
		// withdrawn holds nothing — so answering [] on failure would be indistinguishable from
		// that person, and the screen would state "you may do nothing" when the truth is "we could
		// not find out". Nothing is widened by refusing: the server-side check is unaffected.
		h.d.Log.Error("phiên hiện tại: không đọc được danh sách quyền",
			"can_bo", ph.MaCanBo, "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `permissions` must marshal as [] and never as null.
	ra := phienHienTaiRa{
		Sid:       ph.Sid,
		ExpiresAt: ph.HetHanLuc,
		Staff: canBoGon{
			Code:     ph.MaCanBo,
			FullName: ph.HoTen,
			Position: ph.ChucVu,
		},
		Permissions: make([]string, 0, len(quyen)),
	}
	for _, q := range quyen {
		ra.Permissions = append(ra.Permissions, string(q))
	}
	vietJSON(w, http.StatusOK, ra)
}
