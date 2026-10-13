package http

// The OWNER-ONLY account screens of platform-admin (ADR 0048 §"Sửa đổi 13/10/2026 — owner, …" #2–#6):
// the operator accounts ("Phân quyền": list, create, set ops.* keys, reset password) and a commune's
// sign-in accounts (list, create the commune's administrator, reset any staff password).
//
// THIS SERVICE IS A RELAY. Every act is identity's (OperatorService, operator.proto, the seven
// owner-only RPCs): identity resolves the owner from the forwarded `op1.` token, re-checks the owner
// flag on the account IT resolved, writes the change and its trail entry in one transaction — operator
// acts in operator_audit_log, staff acts in the TARGET commune's audit_log (§Sửa đổi #5). Platform
// writes no row of its own: there is no platform-side record of these acts to keep.
//
// AUTHORISATION HAPPENS TWICE, ON PURPOSE: opauth.SignedIn + opauth.RequireOwner here (a non-owner
// never reaches identity's account tables through this edge), and identity's own owner check on the
// session it resolves — the most powerful acts in the realm never rest on a check made in another
// process alone.
//
// WHERE A TEMPORARY PASSWORD GOES IN THIS PROCESS — the whole list, as on the Mini App secret:
//
//	body bytes      read once by decodeBody (bounded), never logged; no request-logging middleware on
//	                the operator edge (cmd/server/operator_edge.go)
//	decoded value   secretInput → secret.Secret, whose every rendering is "***"
//	idempotency     idem.KhongCan on every route: core/idem stores nothing for KhongCan
//	identity        one gRPC field, taken out with .Lo() inside core/operatorclient only
//	responses       never — identity's answer has no field for it
//
// PERSONAL DATA (rule 3): the responses carry operators' and staff members' names and sign-in emails —
// the owner's screen needs them to find the person. Every response here is Cache-Control: no-store,
// and nothing here logs a body, an email or a name: only the class of a failure.

import (
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// --- shapes ---------------------------------------------------------------------------------------

// operatorAccountView is one operator account. NEVER a password, a hash, a TOTP secret, a recovery
// code or a session; never the internal operator id — the account is named by `code` (VH-…).
type operatorAccountView struct {
	Code               string   `json:"code"`
	Email              string   `json:"email"`
	DisplayName        string   `json:"display_name"`
	PermissionKeys     []string `json:"permission_keys"`
	IsOwner            bool     `json:"is_owner"`
	Disabled           bool     `json:"disabled"`
	MustChangePassword bool     `json:"must_change_password"`
	// LockedUntil: null = not locked. A past instant also means not locked — the console compares it
	// with now; there is no "locked" flag to go stale (migration 0012).
	LockedUntil *time.Time `json:"locked_until"`
	CreatedAt   time.Time  `json:"created_at"`
}

type operatorAccountListView struct {
	Items []operatorAccountView `json:"items"`
}

// staffAccountView is one sign-in account of a commune — the narrowest shape the owner's screen needs
// (no phone, no unit, no position). user_id names it in the password-reset path.
type staffAccountView struct {
	UserID             string `json:"user_id"`
	StaffCode          string `json:"staff_code"`
	Email              string `json:"email"`
	FullName           string `json:"full_name"`
	RoleCode           string `json:"role_code"`
	RoleName           string `json:"role_name"`
	IsAdmin            bool   `json:"is_admin"`
	Disabled           bool   `json:"disabled"`
	MustChangePassword bool   `json:"must_change_password"`
	// SignInLockedUntil: null = not locked; a past instant also means not locked.
	SignInLockedUntil *time.Time `json:"sign_in_locked_until"`
}

type staffAccountListView struct {
	Items []staffAccountView `json:"items"`
}

type operatorAccountBody struct {
	Email             string      `json:"email"`
	DisplayName       string      `json:"display_name"`
	TemporaryPassword secretInput `json:"temporary_password"`
	// May be absent or empty: an account holding no key (ADR 0048 §Sửa đổi #2).
	PermissionKeys []string `json:"permission_keys"`
}

type operatorPermissionsBody struct {
	// A pointer so an ABSENT field is a 400, never a silent "revoke every key": an empty list is
	// meaningful (the complete set is nothing), a missing one is a malformed request.
	PermissionKeys *[]string `json:"permission_keys"`
	Reason         string    `json:"reason"`
}

// temporaryPasswordBody is the reset body of both password routes.
type temporaryPasswordBody struct {
	TemporaryPassword secretInput `json:"temporary_password"`
	Reason            string      `json:"reason"`
}

type communeAdminAccountBody struct {
	Email             string      `json:"email"`
	FullName          string      `json:"full_name"`
	TemporaryPassword secretInput `json:"temporary_password"`
}

const (
	msgAccountEmailTaken    = "Email này đã có tài khoản."
	msgAccountInput         = "Thông tin tài khoản không hợp lệ: kiểm tra email, họ tên và các quyền đã chọn."
	msgOperatorAccountGone  = "Không tìm thấy tài khoản vận hành."
	msgStaffAccountGone     = "Không tìm thấy tài khoản đăng nhập này ở xã."
	msgStaffAccountLocked   = "Tài khoản đang bị xã khoá nên không đặt lại mật khẩu được. Xã mở khoá trước."
	msgAccountsUnavailable  = "Hệ thống tạm thời không quản lý được tài khoản. Vui lòng thử lại sau."
	msgAccountsNotReady     = "Chức năng quản lý tài khoản chưa sẵn sàng trên máy chủ xác thực."
	msgOwnerForbidden       = "Chỉ tài khoản owner của khu vận hành làm được việc này."
	maxPathAccountCodeRunes = 64
)

func toOperatorAccountView(a operatorclient.OperatorAccount) operatorAccountView {
	keys := a.PermissionKeys
	if keys == nil {
		keys = []string{}
	}
	v := operatorAccountView{Code: a.Code, Email: a.Email, DisplayName: a.DisplayName, PermissionKeys: keys,
		IsOwner: a.IsOwner, Disabled: a.Disabled, MustChangePassword: a.MustChangePassword, CreatedAt: a.CreatedAt.UTC()}
	if a.LockedUntil != nil {
		t := a.LockedUntil.UTC()
		v.LockedUntil = &t
	}
	return v
}

func toStaffAccountView(a operatorclient.CommuneStaffAccount) staffAccountView {
	v := staffAccountView{UserID: a.UserID, StaffCode: a.StaffCode, Email: a.Email, FullName: a.FullName,
		RoleCode: a.RoleCode, RoleName: a.RoleName, IsAdmin: a.IsAdmin, Disabled: a.Disabled,
		MustChangePassword: a.MustChangePassword}
	if a.SignInLockedUntil != nil {
		t := a.SignInLockedUntil.UTC()
		v.SignInLockedUntil = &t
	}
	return v
}

// --- answers --------------------------------------------------------------------------------------

// ownerRefusal answers an outcome that is neither ACCEPTED nor NEW_PASSWORD_REJECTED.
func ownerRefusal(w http.ResponseWriter, o operatorclient.Outcome) {
	switch o {
	case operatorclient.OutcomeSessionNotLive:
		// Possible between our Resolve and identity's own: the session ended in between.
		opauth.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgSignInRefused, "")
	case operatorclient.OutcomePermissionDenied:
		// Identity's own owner check refused a principal ours admitted — the flag changed in between.
		httpx.WriteError(w, http.StatusForbidden, "forbidden", msgOwnerForbidden, "")
	default:
		httpx.WriteError(w, http.StatusServiceUnavailable, "account_management_unavailable", msgAccountsUnavailable, "")
	}
}

// ownerCallFailed answers an owner-only RPC that returned an error. notFoundCode is the route's 404
// code. Logs the class of failure only — never the request, never identity's status message.
func (h *operatorHandlers) ownerCallFailed(w http.ResponseWriter, r *http.Request, err error, notFoundCode, notFoundMsg string) {
	switch {
	case errors.Is(err, operatorclient.ErrAccountNotFound):
		httpx.WriteError(w, http.StatusNotFound, notFoundCode, notFoundMsg, "")
	case errors.Is(err, operatorclient.ErrAccountExists):
		httpx.WriteError(w, http.StatusConflict, "account_email_taken", msgAccountEmailTaken, "")
	case errors.Is(err, operatorclient.ErrCommuneNotLive):
		h.writeRegistryError(w, r, store.ErrCommuneInactive)
	case errors.Is(err, operatorclient.ErrStaffAccountLocked):
		httpx.WriteError(w, http.StatusConflict, "staff_account_locked", msgStaffAccountLocked, "")
	case errors.Is(err, operatorclient.ErrArgumentRefused):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_account_input", msgAccountInput, "")
	case errors.Is(err, operatorclient.ErrNotSupported):
		// An identity older than these RPCs, during a rollout in which platform ships first. 503, not
		// 501: it is temporary, and the console shows "not ready" rather than "not supported".
		httpx.WriteError(w, http.StatusServiceUnavailable, "account_management_not_ready", msgAccountsNotReady, "")
	default:
		// Unreachable identity, deadline, realm not configured, contract fault, or a wiring fault of
		// ours (ErrInvalidRequest): the act did not happen. 503 — never "refused", never "signed out".
		h.d.Log.WarnContext(r.Context(), "khu vận hành: thao tác tài khoản owner không thực hiện được",
			"event", "owner.account_call_failed", "err_class", secretStatusErrClass(err))
		httpx.WriteError(w, http.StatusServiceUnavailable, "account_management_unavailable", msgAccountsUnavailable, "")
	}
}

// cleanSecretInput: NUL / U+FFFD in a password are not something a person typed (cleanCredentials).
func cleanSecretInput(s secret.Secret) bool { return cleanCredentials(string(s.Lo())) }

// pathAccountCode is the {code} of an operator-account path. Its shape is identity's to judge (an
// unknown code is NOT_FOUND there, after the owner check); only an absurd length is refused here, as
// the same 404, so a megabyte path segment is never forwarded.
func pathAccountCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := r.PathValue("code")
	if code == "" || utf8.RuneCountInString(code) > maxPathAccountCodeRunes || !cleanCredentials(code) {
		httpx.WriteError(w, http.StatusNotFound, "operator_account_not_found", msgOperatorAccountGone, "")
		return "", false
	}
	return code, true
}

// --- operator accounts ----------------------------------------------------------------------------

func (h *operatorHandlers) listOperatorAccounts(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.ListOperatorAccounts(r.Context(), tok)
	if err != nil {
		h.ownerCallFailed(w, r, err, "operator_account_not_found", msgOperatorAccountGone)
		return
	}
	if res.Outcome != operatorclient.OutcomeAccepted {
		ownerRefusal(w, res.Outcome)
		return
	}
	out := operatorAccountListView{Items: make([]operatorAccountView, 0, len(res.Accounts))}
	for _, a := range res.Accounts {
		out.Items = append(out.Items, toOperatorAccountView(a))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *operatorHandlers) createOperatorAccount(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorAccountBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(append([]string{b.Email, b.DisplayName}, b.PermissionKeys...)...) || !cleanSecretInput(b.TemporaryPassword.Secret) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.CreateOperatorAccount(r.Context(), operatorclient.CreateOperatorAccountRequest{
		Token: tok, Email: b.Email, DisplayName: norm.NFC.String(b.DisplayName),
		TemporaryPassword: b.TemporaryPassword.Secret, PermissionKeys: b.PermissionKeys, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.ownerCallFailed(w, r, err, "operator_account_not_found", msgOperatorAccountGone)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		writeJSON(w, http.StatusCreated, toOperatorAccountView(res.Account))
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	default:
		ownerRefusal(w, res.Outcome)
	}
}

func (h *operatorHandlers) setOperatorPermissions(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	code, ok := pathAccountCode(w, r)
	if !ok {
		return
	}
	var b operatorPermissionsBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.PermissionKeys == nil || !cleanCredentials(*b.PermissionKeys...) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.SetOperatorPermissions(r.Context(), operatorclient.SetOperatorPermissionsRequest{
		Token: tok, TargetCode: code, PermissionKeys: *b.PermissionKeys, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.ownerCallFailed(w, r, err, "operator_account_not_found", msgOperatorAccountGone)
		return
	}
	if res.Outcome != operatorclient.OutcomeAccepted {
		ownerRefusal(w, res.Outcome)
		return
	}
	writeJSON(w, http.StatusOK, toOperatorAccountView(res.Account))
}

func (h *operatorHandlers) resetOperatorPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	code, ok := pathAccountCode(w, r)
	if !ok {
		return
	}
	var b temporaryPasswordBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanSecretInput(b.TemporaryPassword.Secret) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.ResetOperatorPassword(r.Context(), operatorclient.ResetOperatorPasswordRequest{
		Token: tok, TargetCode: code, TemporaryPassword: b.TemporaryPassword.Secret, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.ownerCallFailed(w, r, err, "operator_account_not_found", msgOperatorAccountGone)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		writeJSON(w, http.StatusOK, toOperatorAccountView(res.Account))
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	default:
		ownerRefusal(w, res.Outcome)
	}
}

// --- a commune's sign-in accounts -----------------------------------------------------------------

// ownerTargetCommune is targetCommune plus the registry read: the commune must exist and not be
// soft-deleted (store.OperatorRegistry.Commune filters deleted_at), else 404 commune_not_found —
// before identity is asked anything about it. The commune travels to identity as ctx's x-tenant-id,
// from the PATH only (rule 1, forbidden #2).
func (h *operatorHandlers) ownerTargetCommune(w http.ResponseWriter, r *http.Request) (domain.Commune, *http.Request, bool) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return domain.Commune{}, nil, false
	}
	c, _, err := h.d.Registry.Commune(r.Context(), id)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return domain.Commune{}, nil, false
	}
	return c, r.WithContext(ctx), true
}

func (h *operatorHandlers) listCommuneStaffAccounts(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	_, r, ok := h.ownerTargetCommune(w, r)
	if !ok {
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.ListCommuneStaffAccounts(r.Context(), tok, httpx.ClientIP(r))
	if err != nil {
		h.ownerCallFailed(w, r, err, "staff_account_not_found", msgStaffAccountGone)
		return
	}
	if res.Outcome != operatorclient.OutcomeAccepted {
		ownerRefusal(w, res.Outcome)
		return
	}
	out := staffAccountListView{Items: make([]staffAccountView, 0, len(res.Accounts))}
	for _, a := range res.Accounts {
		out.Items = append(out.Items, toStaffAccountView(a))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *operatorHandlers) createCommuneAdminAccount(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	c, r, ok := h.ownerTargetCommune(w, r)
	if !ok {
		return
	}
	var b communeAdminAccountBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.Email, b.FullName) || !cleanSecretInput(b.TemporaryPassword.Secret) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	// An inactive commune is refused here as identity would refuse it (FAILED_PRECONDITION, re-read
	// live there): an account nobody can reach is not created, and the refusal costs no RPC.
	if !c.Active {
		h.writeRegistryError(w, r, store.ErrCommuneInactive)
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.CreateCommuneAdminAccount(r.Context(), operatorclient.CreateCommuneAdminAccountRequest{
		Token: tok, Email: b.Email, FullName: norm.NFC.String(b.FullName),
		TemporaryPassword: b.TemporaryPassword.Secret, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.ownerCallFailed(w, r, err, "staff_account_not_found", msgStaffAccountGone)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		writeJSON(w, http.StatusCreated, toStaffAccountView(res.Account))
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	default:
		ownerRefusal(w, res.Outcome)
	}
}

func (h *operatorHandlers) resetStaffPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	_, r, ok := h.ownerTargetCommune(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("user_id")
	if utf8.RuneCountInString(userID) > maxPathAccountCodeRunes || !cleanCredentials(userID) {
		httpx.WriteError(w, http.StatusNotFound, "staff_account_not_found", msgStaffAccountGone, "")
		return
	}
	var b temporaryPasswordBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanSecretInput(b.TemporaryPassword.Secret) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.ResetStaffPassword(r.Context(), operatorclient.ResetStaffPasswordRequest{
		Token: tok, UserID: userID, TemporaryPassword: b.TemporaryPassword.Secret, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.ownerCallFailed(w, r, err, "staff_account_not_found", msgStaffAccountGone)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		writeJSON(w, http.StatusOK, toStaffAccountView(res.Account))
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	default:
		ownerRefusal(w, res.Outcome)
	}
}
