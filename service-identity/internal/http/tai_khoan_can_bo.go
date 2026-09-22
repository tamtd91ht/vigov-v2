package http

// The three routes that issue, reset and change a staff credential (open questions #9, #17, #18,
// all decided 2026-09-22).
//
//	POST /api/v1/staff/{id}/account            an administrator gives somebody an account (#9)
//	PUT  /api/v1/staff/{id}/password           an administrator resets somebody's password (#17)
//	PUT  /api/v1/staff/current/password        the person changes their own — the ONLY route that
//	                                           clears `phai_doi_mat_khau`
//
// THE TEMPORARY PASSWORD IS IN THE RESPONSE BODY OF THE FIRST TWO AND NOWHERE ELSE IN THIS SYSTEM.
// It is not stored (the database holds its argon2id hash), not audited, not logged, and not
// readable a second time — there is no GET that returns it and no code path that could rebuild it.
// An administrator who loses it resets again, which mints a DIFFERENT value and files a second
// audit entry saying so. That is #9's guarantee made operational: after the person changes it,
// nobody but them knows their password, which is what makes "who did this" (rule 6, invariant 2)
// mean anything.
//
// WHICH MEANS THE ONE THING NOT TO DO IN THIS FILE IS ADD A LOG LINE. Not at Error level while
// debugging a failing reset, not `"than", than` on a decode failure, not `%+v` of the reply struct.
// The log pipeline is centralised across every commune and a credential that reaches it cannot be
// recalled (rule 3, forbidden #1; rule 8, invariant 1). Every log call below names the commune and
// the staff CODE, and nothing else.

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// capTaiKhoanRa is what the administrator's browser gets back.
//
// TWO FIELDS, AND THE SECOND ONE IS THE REASON THE ROUTE RETURNS A BODY AT ALL. `staff` is the same
// shape every other staff route returns (raNgoai), so the screen can redraw the row it just
// changed; `temporary_password` is the value the administrator reads out, once.
//
// THE FIELD NAME SAYS `temporary`, DELIBERATELY. A client that stores this, caches it, or puts it
// in a toast that stays on screen has misunderstood what it is holding, and the name is the last
// chance to say so before it is out of this process's hands.
type capTaiKhoanRa struct {
	Staff canBoTomTat `json:"staff"`

	// TemporaryPassword is plaintext, returned exactly once, and the server cannot produce it
	// again. See the file header.
	//
	// THE `apidoc` TAG IS A DECLARED EXEMPTION, NOT A WORKAROUND, and the difference is that it is
	// written down here rather than argued away in the generator. `tools/apidoc` refuses any
	// credential-shaped field in a response shape (rule 3, rule 8) and it is right to: a published
	// shape naming a password teaches every integrator to expect one there. That is the cost, and
	// the customer accepted it on 2026-09-22 rather than the alternative, which was the server
	// sending the value to the person's own mobile by ZNS — cleaner under rule 3, and needing
	// `service-comms`, a per-commune ZNS key (ADR 0018) and a template nobody has decided.
	//
	// THE REASON IS PRINTED INTO `openapi.json`, so the exemption makes the contract LOUDER about
	// the credential, never quieter. Removing the tag does not remove the problem — it puts the
	// build back to red, which is the correct behaviour if this field ever stops being a decision
	// somebody made on purpose.
	TemporaryPassword string `json:"temporary_password" apidoc:"bi-mat-co-chu-y:Mật khẩu tạm dùng MỘT LẦN, khách chốt 22/09/2026 (câu mở #9): quản trị viên đọc lại cho cán bộ, máy chủ không giữ bản trần và không trả lại lần thứ hai. Đổi ở lần đăng nhập đầu là bắt buộc."`
}

// doiMatKhauVao is the body of PUT /api/v1/staff/current/password.
//
// THE CURRENT PASSWORD IS REQUIRED EVEN ON THE FORCED-CHANGE SCREEN, and dropping it there is the
// change somebody will propose because it saves a field. It must not be dropped: open question #18
// settled that the machine at the one-stop-shop counter is SHARED, so "this browser holds a
// session" and "this person knows the password" are routinely two different people. Requiring the
// current value is what stops an unattended browser becoming a permanent account takeover.
//
// NO `staff_id` FIELD AND THERE MUST NEVER BE ONE. The account changed is the one the session
// belongs to, taken from the principal the edge built. A field here would be a person naming whose
// password they are setting (rule 4, invariant 2, applied to staff).
type doiMatKhauVao struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// CapTaiKhoanCanBo gives one directory row an account. POST /api/v1/staff/{id}/account
func (h *Handler) CapTaiKhoanCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	kq, err := h.d.TaiKhoan.Cap(r.Context(), r.PathValue("id"), nguoi)
	if err != nil {
		h.traLoiLoiTaiKhoan(w, r, "cấp tài khoản cán bộ", false, err)
		return
	}
	vietJSON(w, http.StatusCreated, capTaiKhoanRa{
		Staff:             raNgoai(kq.CanBo),
		TemporaryPassword: kq.MatKhauTam,
	})
}

// DatLaiMatKhauCanBo resets somebody's password. PUT /api/v1/staff/{id}/password
func (h *Handler) DatLaiMatKhauCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	kq, err := h.d.TaiKhoan.DatLai(r.Context(), r.PathValue("id"), nguoi)
	if err != nil {
		h.traLoiLoiTaiKhoan(w, r, "đặt lại mật khẩu cán bộ", false, err)
		return
	}
	// 200 AND NOT 201: the account already existed, only its credential moved. The administrator
	// screen redraws the same row it was already showing.
	vietJSON(w, http.StatusOK, capTaiKhoanRa{
		Staff:             raNgoai(kq.CanBo),
		TemporaryPassword: kq.MatKhauTam,
	})
}

// DoiMatKhauChinhMinh is the person changing their own password.
// PUT /api/v1/staff/current/password
//
// IT ENDS THE SESSION IT ARRIVED ON, and the cookie is cleared here to match. The use case revokes
// every open session of that account (skills/session-and-token, required #7); leaving the browser
// holding a token whose session no longer exists would show them a signed-in interface that answers
// 401 to the first thing they click.
func (h *Handler) DoiMatKhauChinhMinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}

	// THE BODY IS DECODED WITHOUT ANY ERROR THAT COULD QUOTE IT. json.Decoder's own message
	// includes the offending input, which on this route is a password (rule 3, forbidden #3 —
	// and worse than personal data, because it still works). docThanCanBo already answers with a
	// sentence of its own; it is reused here for exactly that property.
	var than doiMatKhauVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	err := h.d.TaiKhoan.DoiCuaChinhMinh(r.Context(), app.YeuCauDoiMatKhau{
		HienTai: than.CurrentPassword,
		Moi:     than.NewPassword,
	}, nguoi)
	if err != nil {
		h.traLoiLoiTaiKhoan(w, r, "đổi mật khẩu", true, err)
		return
	}

	xoaCookiePhien(w)
	// 204 AND NO BODY. There is nothing to return: the record did not change in any way a screen
	// shows, and the one thing that DID change is a credential, which must not be echoed. The
	// client's next move is the sign-in screen.
	w.WriteHeader(http.StatusNoContent)
}

// traLoiLoiTaiKhoan maps the refusals of the three routes above onto a status and a sentence.
//
// IT DELEGATES THE SHARED ONES to traLoiLoiGhiCanBo rather than restating them: a 404 for another
// commune's id, the #14 self-target refusal and the missing-actor case are the same decisions the
// five register routes already made, and a second copy would drift — the copy that drifts answers
// 500 where it meant 404, which reads as a broken server rather than as a rule doing its job.
//
// `tuChinhMinh` SAYS WHICH SURFACE THIS IS, and it is a parameter rather than something inferred
// from the request, because the one refusal it changes must not depend on a path segment being
// absent. See the ErrCanBoKhongTonTai branch: on an administrator route that error is "no such
// person in this commune", on the self route it is "your own account is gone" — two different
// answers, and reading the wrong one tells a member of staff to go and find an id.
//
// NO MESSAGE HERE EVER QUOTES WHAT WAS SENT. Every branch names the rule.
func (h *Handler) traLoiLoiTaiKhoan(w http.ResponseWriter, r *http.Request, viec string,
	tuChinhMinh bool, err error) {
	switch {
	case errors.Is(err, app.ErrDaCoTaiKhoan):
		// 409 AND NOT 403: the caller holds `admin.user` and is allowed to manage accounts. What is
		// refused is this operation against the STATE of the row — and the sentence points at the
		// route that does what they meant, because "already has an account" and "wants a new
		// password" are the same intent arriving at the wrong door.
		httpx.WriteError(w, http.StatusConflict, "account_exists",
			"Cán bộ này đã có tài khoản. Nếu cần cấp lại mật khẩu, hãy dùng chức năng đặt lại mật khẩu.", "")

	case errors.Is(err, app.ErrChuaCoTaiKhoan):
		httpx.WriteError(w, http.StatusConflict, "account_missing",
			"Cán bộ này chưa có tài khoản đăng nhập. Hãy cấp tài khoản trước.", "")

	case errors.Is(err, app.ErrTaiKhoanDangKhoa):
		// 409, and the sentence says what to do first. Without it the administrator would read a
		// password down the telephone to somebody it cannot work for, and neither of them would be
		// told why: the sign-in read filters `dang_hoat_dong` and answers exactly as it does for a
		// wrong password.
		httpx.WriteError(w, http.StatusConflict, "account_locked",
			"Tài khoản của cán bộ này đang bị khoá. Hãy mở khoá trước khi cấp hoặc đặt lại mật khẩu.", "")

	case errors.Is(err, app.ErrMatKhauHienTaiSai):
		// 400 AND DELIBERATELY NOT 401. The session is perfectly valid; what failed is one field of
		// this request. A 401 makes every browser clear its session and send the person to sign in
		// again — which, on the forced-change screen, is an endless loop for anybody who mistypes.
		//
		// The code is generic on purpose: it says the field was wrong, never whether the account,
		// the hash or the value was the problem.
		httpx.WriteError(w, http.StatusBadRequest, "current_password_invalid",
			"Mật khẩu hiện tại không đúng.", "")

	case errors.Is(err, domain.ErrMatKhauMoiTrungCu):
		httpx.WriteError(w, http.StatusBadRequest, "password_unchanged",
			"Mật khẩu mới phải khác mật khẩu hiện tại.", "")

	case laLoiMatKhauMoi(err):
		// The domain's own sentence is returned: it names the rule, holds no part of the value it
		// refused, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_password", err.Error(), "")

	case errors.Is(err, idstore.ErrCanBoKhongTonTai):
		// Named here as well as in the shared mapper, because on the SELF route it means something
		// different and must not reach the client as "staff not found": the caller's own account was
		// withdrawn or locked between the edge building their principal and this write. Their
		// session is finished, so 401 is the honest answer and the cookie goes with it.
		if tuChinhMinh {
			xoaCookiePhien(w)
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
				"Phiên đăng nhập không còn hiệu lực. Vui lòng đăng nhập lại.", "")
			return
		}
		h.traLoiLoiGhiCanBo(w, r, viec, err)

	default:
		// Everything else — including the store failures — goes through the one mapping the five
		// register routes already use. Its `default` branch logs the commune and the wrapped error,
		// neither of which can carry a password: nothing in app/ or store/ puts a plaintext in an
		// error, and the two places that could were written not to.
		h.traLoiLoiGhiCanBo(w, r, viec, err)
	}
}

// laLoiMatKhauMoi reports whether this is a refusal of the password the client chose.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, the same discipline as laLoiDauVaoCanBo: a
// default of "anything unrecognised is the client's fault" turns a database outage into a 400, and
// a person who believes their password is wrong retries forever while nobody is told the server is
// broken.
func laLoiMatKhauMoi(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuMatKhau, domain.ErrMatKhauQuaNgan,
		domain.ErrMatKhauQuaDai, domain.ErrMatKhauKhongDoc,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
