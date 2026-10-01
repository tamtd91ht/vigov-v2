package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-platform/internal/opauth"
)

// OperatorIdentity is identity's OperatorService as these handlers use it. *operatorclient.Client
// satisfies it; tests use a fake.
type OperatorIdentity interface {
	Open(ctx context.Context, req operatorclient.OpenRequest) (operatorclient.OpenResult, error)
	Revoke(ctx context.Context, token secret.Secret, clientIP string) (operatorclient.Outcome, error)
	ChangePassword(ctx context.Context, req operatorclient.ChangePasswordRequest) (operatorclient.ChangePasswordResult, error)
	RegenerateRecoveryCodes(ctx context.Context, token, totpCode secret.Secret, clientIP string) (operatorclient.RecoveryCodesResult, error)
	BeginEnrollment(ctx context.Context, req operatorclient.BeginEnrollmentRequest) (operatorclient.BeginEnrollmentResult, error)
	CompleteEnrollment(ctx context.Context, req operatorclient.CompleteEnrollmentRequest) (operatorclient.CompleteEnrollmentResult, error)
}

type operatorHandlers struct{ d OperatorDeps }

// maxOperatorBody bounds every operator request body. The largest legitimate one is a password
// change — three short strings.
const maxOperatorBody = 16 << 10

const (
	msgInvalidBody     = "Dữ liệu gửi lên không hợp lệ."
	msgSignInRefused   = "Thông tin đăng nhập không đúng, hoặc tài khoản tạm thời không đăng nhập được."
	msgEnrollment      = "Tài khoản cần đăng ký ứng dụng xác thực trước khi đăng nhập."
	msgCredsRefused    = "Mật khẩu hoặc mã xác thực không đúng."
	msgPasswordRule    = "Mật khẩu mới không đạt yêu cầu."
	msgIdentityDown    = "Hệ thống tạm thời không xác thực được. Vui lòng thử lại sau."
	msgTwoSecondFactor = "Chỉ gửi mã TOTP hoặc mã khôi phục, không gửi cả hai."
)

// decodeBody reads one JSON object, refusing unknown fields, a second value, a body over the bound,
// and ANY invalid UTF-8 — checked on the raw bytes, because encoding/json silently replaces invalid
// sequences with U+FFFD and a credential would then reach identity as a different string than the
// one typed. false means the 400 is already written.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOperatorBody))
	if err != nil || !utf8.Valid(raw) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil || dec.More() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return false
	}
	return true
}

// cleanCredentials refuses NUL and U+FFFD in any credential field, BEFORE identity is called. A JSON
// `\u0000` decodes to a real NUL that a password store may truncate at; a lone surrogate escape
// decodes to U+FFFD. Neither is something a person typed, and neither may be counted as a guess.
func cleanCredentials(fields ...string) bool {
	for _, f := range fields {
		if strings.ContainsRune(f, 0) || strings.ContainsRune(f, utf8.RuneError) {
			return false
		}
	}
	return true
}

// noStore marks a response that carries a credential or a TOTP secret: never cached by a browser or
// a proxy.
func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// identityFailed answers a call that did not happen. ErrInvalidRequest is decided from the request
// alone (both factors), so it is the client's 400; everything else is 503 — never "refused".
func identityFailed(w http.ResponseWriter, err error) {
	if errors.Is(err, operatorclient.ErrInvalidRequest) {
		httpx.WriteError(w, http.StatusBadRequest, "second_factor_ambiguous", msgTwoSecondFactor, "")
		return
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, "operator_auth_unavailable", msgIdentityDown, "")
}

func signInRefused(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnauthorized, "sign_in_refused", msgSignInRefused, "")
}

// --- request and response shapes -------------------------------------------------------------------

type operatorSignInBody struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	TOTPCode     string `json:"totp_code"`
	RecoveryCode string `json:"recovery_code"`
}

type operatorSessionView struct {
	OperatorCode string    `json:"operator_code"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type operatorEnrollmentBody struct {
	Email             string `json:"email"`
	TemporaryPassword string `json:"temporary_password"`
}

type operatorEnrollmentView struct {
	OperatorCode    string `json:"operator_code"`
	ProvisioningURI string `json:"provisioning_uri"`
	ManualEntryKey  string `json:"manual_entry_key"`
}

type operatorEnrollmentCompletionBody struct {
	Email             string `json:"email"`
	TemporaryPassword string `json:"temporary_password"`
	NewPassword       string `json:"new_password"`
	TOTPCode          string `json:"totp_code"`
}

type operatorEnrollmentCompletedView struct {
	OperatorCode  string    `json:"operator_code"`
	ExpiresAt     time.Time `json:"expires_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
}

// passwordRejectionView is httpx.Error plus the rule that refused the new password — the RULE and its
// bounds, never the value. The bounds come from identity so the console holds no copy of "12".
type passwordRejectionView struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
	Problem   string `json:"problem"` // empty | not_utf8 | too_short | too_long | same_as_current
	MinLength uint32 `json:"min_length"`
	MaxLength uint32 `json:"max_length"`
}

type operatorWhoAmIView struct {
	OperatorCode   string   `json:"operator_code"`
	PermissionKeys []string `json:"permission_keys"`
}

type operatorPasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	TOTPCode        string `json:"totp_code"`
}

type operatorRecoveryCodesBody struct {
	TOTPCode string `json:"totp_code"`
}

type operatorRecoveryCodesView struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

var passwordProblems = map[identityv1.NewPasswordProblem]string{
	identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_EMPTY:           "empty",
	identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_NOT_UTF8:        "not_utf8",
	identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT:       "too_short",
	identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_LONG:        "too_long",
	identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_SAME_AS_CURRENT: "same_as_current",
}

func passwordRejected(w http.ResponseWriter, ref operatorclient.NewPasswordRefusal) {
	writeJSON(w, http.StatusUnprocessableEntity, passwordRejectionView{
		Code: "new_password_rejected", Message: msgPasswordRule,
		Problem: passwordProblems[ref.Problem], MinLength: ref.MinLength, MaxLength: ref.MaxLength,
	})
}

func reveal(codes []secret.Secret) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c.Lo())
	}
	return out
}

// --- handlers -------------------------------------------------------------------------------------

// createSession signs an operator in. Password + AT MOST one second factor: both is 400 here, decided
// from the request; neither is SENT, because identity answers ENROLLMENT_REQUIRED for a right
// password on an account with no factor yet (operator_auth.go) — refusing it locally would leave a
// fresh account no way to learn it must enrol.
func (h *operatorHandlers) createSession(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorSignInBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.Email, b.Password, b.TOTPCode, b.RecoveryCode) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	if b.TOTPCode != "" && b.RecoveryCode != "" {
		httpx.WriteError(w, http.StatusBadRequest, "second_factor_ambiguous", msgTwoSecondFactor, "")
		return
	}
	res, err := h.d.Identity.Open(r.Context(), operatorclient.OpenRequest{
		Email:        b.Email,
		Password:     secret.Secret(b.Password),
		TOTPCode:     secret.Secret(b.TOTPCode),
		RecoveryCode: secret.Secret(b.RecoveryCode),
		ClientIP:     httpx.ClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		identityFailed(w, err)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		opauth.SetCookie(w, res.Session.Token, res.Session.ExpiresAt, h.d.Now())
		writeJSON(w, http.StatusCreated, operatorSessionView{
			OperatorCode: res.Session.OperatorCode, ExpiresAt: res.Session.ExpiresAt.UTC()})
	case operatorclient.OutcomeEnrollmentRequired:
		// Distinct on purpose and only ever after a RIGHT password (operator.proto): the person
		// goes to the enrolment screen with their temporary password.
		httpx.WriteError(w, http.StatusForbidden, "enrollment_required", msgEnrollment, "")
	default:
		signInRefused(w)
	}
}

func (h *operatorHandlers) beginEnrollment(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorEnrollmentBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.Email, b.TemporaryPassword) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	res, err := h.d.Identity.BeginEnrollment(r.Context(), operatorclient.BeginEnrollmentRequest{
		Email: b.Email, TemporaryPassword: secret.Secret(b.TemporaryPassword), ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		identityFailed(w, err)
		return
	}
	if res.Outcome != operatorclient.OutcomeAccepted {
		signInRefused(w)
		return
	}
	writeJSON(w, http.StatusCreated, operatorEnrollmentView{
		OperatorCode:    res.OperatorCode,
		ProvisioningURI: string(res.ProvisioningURI.Lo()),
		ManualEntryKey:  string(res.ManualEntryKey.Lo()),
	})
}

func (h *operatorHandlers) completeEnrollment(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorEnrollmentCompletionBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.Email, b.TemporaryPassword, b.NewPassword, b.TOTPCode) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	res, err := h.d.Identity.CompleteEnrollment(r.Context(), operatorclient.CompleteEnrollmentRequest{
		Email:             b.Email,
		TemporaryPassword: secret.Secret(b.TemporaryPassword),
		NewPassword:       secret.Secret(b.NewPassword),
		TOTPCode:          secret.Secret(b.TOTPCode),
		ClientIP:          httpx.ClientIP(r),
		UserAgent:         r.UserAgent(),
	})
	if err != nil {
		identityFailed(w, err)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		opauth.SetCookie(w, res.Session.Token, res.Session.ExpiresAt, h.d.Now())
		writeJSON(w, http.StatusCreated, operatorEnrollmentCompletedView{
			OperatorCode:  res.Session.OperatorCode,
			ExpiresAt:     res.Session.ExpiresAt.UTC(),
			RecoveryCodes: reveal(res.RecoveryCodes),
		})
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	default:
		signInRefused(w)
	}
}

func (h *operatorHandlers) whoAmI(w http.ResponseWriter, r *http.Request) {
	p, _ := opauth.From(r.Context())
	keys := p.Keys
	if keys == nil {
		keys = []string{}
	}
	writeJSON(w, http.StatusOK, operatorWhoAmIView{OperatorCode: p.OperatorCode, PermissionKeys: keys})
}

// signOut revokes the session NAMED BY THE COOKIE — never a session id from a parameter. Either
// outcome clears the cookie: signing out of a dead session is not an error to the person.
func (h *operatorHandlers) signOut(w http.ResponseWriter, r *http.Request) {
	tok, _ := opauth.TokenFrom(r)
	if _, err := h.d.Identity.Revoke(r.Context(), tok, httpx.ClientIP(r)); err != nil {
		identityFailed(w, err)
		return
	}
	opauth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// changePassword: on success identity revokes EVERY session of the account, this one included, so
// the cookie is cleared and the console sends the person to sign in again.
func (h *operatorHandlers) changePassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorPasswordBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.CurrentPassword, b.NewPassword, b.TOTPCode) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.ChangePassword(r.Context(), operatorclient.ChangePasswordRequest{
		Token:           tok,
		CurrentPassword: secret.Secret(b.CurrentPassword),
		NewPassword:     secret.Secret(b.NewPassword),
		TOTPCode:        secret.Secret(b.TOTPCode),
		ClientIP:        httpx.ClientIP(r),
	})
	if err != nil {
		identityFailed(w, err)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		opauth.ClearCookie(w)
		w.WriteHeader(http.StatusNoContent)
	case operatorclient.OutcomeNewPasswordRejected:
		passwordRejected(w, res.Refusal)
	case operatorclient.OutcomeSessionNotLive:
		opauth.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgSignInRefused, "")
	default:
		// 403, not 401: the session is live, the re-proof failed. A 401 would read to the console as
		// "signed out" and drop the person at the sign-in screen for a typo.
		httpx.WriteError(w, http.StatusForbidden, "credentials_refused", msgCredsRefused, "")
	}
}

func (h *operatorHandlers) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var b operatorRecoveryCodesBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !cleanCredentials(b.TOTPCode) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.RegenerateRecoveryCodes(r.Context(), tok, secret.Secret(b.TOTPCode), httpx.ClientIP(r))
	if err != nil {
		identityFailed(w, err)
		return
	}
	switch res.Outcome {
	case operatorclient.OutcomeAccepted:
		writeJSON(w, http.StatusCreated, operatorRecoveryCodesView{RecoveryCodes: reveal(res.Codes)})
	case operatorclient.OutcomeSessionNotLive:
		opauth.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgSignInRefused, "")
	default:
		httpx.WriteError(w, http.StatusForbidden, "credentials_refused", msgCredsRefused, "")
	}
}
