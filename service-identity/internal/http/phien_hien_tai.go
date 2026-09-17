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

	// Role is the caller's OWN role, or null.
	//
	// A POINTER, SO null MEANS SOMETHING PRECISE: "this account has no role assigned". That is a
	// real state — `nguoi_dung.vai_tro_id` is nullable, and a person can sit in the commune's
	// directory before anybody has decided what they do. It is NOT the answer for a failure: if the
	// role cannot be read the whole route answers 500, exactly as it does for the permission list,
	// and for the same reason — a screen told "you have no role" when the truth is "we could not
	// find out" is a screen stating something false about a member of staff.
	//
	// AN EMPTY OBJECT WOULD BE THE WRONG SHAPE. `{"code":"","name":"","is_leader":false}` reads as
	// a role that exists and is nameless, and `is_leader:false` in it is an assertion nobody made.
	// null cannot be mistaken for a role.
	Role *vaiTroGon `json:"role"`

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

// vaiTroGon is the caller's role as it leaves the API. Three fields, and no fourth is coming: this
// describes a role, it does not grant anything.
type vaiTroGon struct {
	Code string `json:"code"` // vai_tro.ma — the stable slug a client keys on
	Name string `json:"name"` // vai_tro.ten — what a person reads

	// IsLeader CHOOSES THE DEFAULT SCREEN AND NOTHING ELSE: a leader lands on /nhiem-vu/so-tay
	// rather than the ordinary home screen (docs/ui-ux/15-phu-luc-giao-dien-chung.md §1).
	//
	// IT IS NOT A SECOND AUTHORITY AXIS. Nothing on the server reads it to decide anything, and
	// nothing on the client may either. The day a decision hangs off it, this project has two
	// authorisation systems and the second one does not go through (tenant_id, role, permission):
	// it is invisible on the Phân quyền screen, an administrator cannot grant or withdraw it, and
	// changing it leaves no trail (rule 5, invariant 3 and forbidden #2, #3). The argument is made
	// in full on domain.VaiTro.LaLanhDao — this is the same flag, one hop further out.
	//
	// Hiding a menu entry from it is the same mistake in a softer form: the server would still
	// permit the action, so the button is gone for a person who is allowed to act, and present for
	// one who is not the moment somebody edits a flag. Menus are hidden from `permissions` above,
	// which is the list the server's own checks are made of.
	IsLeader bool `json:"is_leader"`
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
//
// The role added to the reply does not change that reading: a role's name is an attribute of the
// COMMUNE'S ORGANISATION, not of a person, it is the caller's own, and it is read inside the
// caller's own commune. What WOULD change it is this route ever describing somebody else.
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

	// THE ROLE IS READ HERE, IN THE HANDLER, AND DELIBERATELY NOT AT THE EDGE. The middleware
	// already holds this person's row and could have carried the role along; it must not, because
	// it runs on every request of every route and the role needs a second table. One route wants
	// it, one route pays for it. See the note on VaiTroDoc.
	//
	// p.ID, not ph.MaCanBo: the query matches `nd.id`, the same column store.Checker matches. The
	// business code belongs on the audit trail, not in a join predicate.
	vaiTro, coVaiTro, err := h.d.VaiTro.VaiTroCuaCanBo(ctx, p.ID)
	if err != nil {
		// 500, NOT A null ROLE — the same asymmetry as the permission list above. "No role
		// assigned" is a legitimate answer this route gives with null, so answering null on a
		// failure would be indistinguishable from it, and the screen would tell a member of staff
		// they hold no role when the truth is that it could not be read. Nothing is widened by
		// refusing: this value decides no access.
		h.d.Log.Error("phiên hiện tại: không đọc được vai trò",
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
	// Left nil — marshalled as null — when there is no role. The zero value of the pointer IS the
	// answer here, which is why the store returns a bool rather than an empty struct.
	if coVaiTro {
		ra.Role = &vaiTroGon{
			Code:     vaiTro.Ma,
			Name:     vaiTro.Ten,
			IsLeader: vaiTro.LaLanhDao,
		}
	}
	for _, q := range quyen {
		ra.Permissions = append(ra.Permissions, string(q))
	}
	vietJSON(w, http.StatusOK, ra)
}
