package http

// The Zalo app secret of a commune's dedicated Mini App, set and retired from the commune detail
// screen of platform-admin (ADR 0070 #4, §"Bổ sung 02/10/2026" #6). This service is a RELAY: the
// secret is stored, sealed, by service-identity (ADR 0066), reached over OperatorService on the
// operator gRPC port. Nothing here keeps it.
//
// WHERE THE SECRET GOES IN THIS PROCESS, AND WHERE IT DOES NOT — the whole list, so a reviewer can
// check it against the code rather than trust it:
//
//	body bytes      read once by decodeBody (bounded by maxOperatorBody), never logged, discarded
//	                when the handler returns. No request-logging middleware is mounted on the
//	                operator edge (cmd/server/operator_edge.go: Recover + StripTenantHeaders only,
//	                and httpx.Recover logs nothing of the request)
//	decoded value   secretInput → secret.Secret, whose every rendering is "***" (%v, %+v, JSON,
//	                slog). The body struct therefore never holds it as a plain string
//	idempotency     idem.KhongCan on both routes: core/idem stores NOTHING for KhongCan, and even
//	                idem.Required stores only "status:code", never a body (core/idem/idem.go,
//	                RecordCode). Declared KhongCan because a retry is safe by contract (below)
//	identity        one gRPC field, taken out with .Lo() inside core/operatorclient only
//	responses       never — the answer is version metadata (version, set_at, set_by)
//	errors / trail  never — identity writes the trail and names only the App ID (operator.proto);
//	                this service writes no trail entry for these two acts, so no second copy
//
// AUTHORISATION HAPPENS TWICE, ON PURPOSE: opauth.RequireKey(KeyMiniAppManage) here, and identity
// re-checks the same key on the principal IT resolves from the forwarded `op1.` token — the write
// into a commune's data never rests on a check made in another process alone (operator.proto).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// secretInput decodes a JSON string straight into a secret.Secret. secret.Secret is a []byte, which
// encoding/json would read as BASE64 — and a plain string field would put the value in a struct that
// a careless %+v prints. The embedded Secret's String/Format/MarshalJSON/LogValue are promoted, so
// the body struct renders "***" too.
type secretInput struct{ secret.Secret }

func (s *secretInput) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		// Our own error, never json's: decodeBody discards it, but nothing here may quote input.
		return errors.New("secret: not a JSON string")
	}
	s.Secret = secret.Secret(v)
	return nil
}

type miniAppSecretBody struct {
	Secret secretInput `json:"secret"`
	Reason string      `json:"reason"`
}

type miniAppSecretRetireBody struct {
	Reason string `json:"reason"`
}

// miniAppSecretView is what the screen may show about a secret just set — never the value, a
// prefix, a length or a fingerprint of it (operator.proto, MiniAppSecretVersion).
type miniAppSecretView struct {
	AppID   string    `json:"app_id"`
	Version string    `json:"version"`
	SetAt   time.Time `json:"set_at"`
	SetBy   string    `json:"set_by"`
}

// miniAppSecretRetirementView: Retired is false when there was nothing live to retire — the same end
// state as a retirement ("this App ID signs nobody in"), with nothing written in identity.
type miniAppSecretRetirementView struct {
	AppID          string     `json:"app_id"`
	Retired        bool       `json:"retired"`
	RetiredVersion string     `json:"retired_version,omitempty"`
	RetiredAt      *time.Time `json:"retired_at,omitempty"`
	RetiredBy      string     `json:"retired_by,omitempty"`
}

// miniAppChangeView answers a replacement or an activation change: the commune as it now stands,
// plus — whenever an App ID was turned off — whether its secret was retired in identity (ADR 0070
// #4). Absent on a reactivation, where nothing is retired.
//
// SecretRetired false does NOT undo the binding change: that committed first, in platform's own
// transaction, and is correct on its own (the login path reads the binding before the secret, so a
// secret left live under a turned-off App ID signs nobody in). The console shows "khoá App ID cũ
// chưa thu hồi" and offers DELETE …/{app_id}/secret as the retry.
type miniAppChangeView struct {
	communeDetailView
	SecretRetired *bool `json:"secret_retired,omitempty"`
	// Set exactly when SecretRetired is false: identity_unavailable | session_not_live | forbidden |
	// refused.
	SecretRetirementError string `json:"secret_retirement_error,omitempty"`
}

const (
	msgSecretInvalid     = "Khoá bí mật không hợp lệ: không được để trống, không chứa khoảng trắng hay ký tự điều khiển."
	msgMiniAppNotBound   = "App ID chưa gắn hoặc đang tắt ở xã này. Gắn hoặc bật lại App ID trước khi đặt khoá bí mật."
	msgSecretUnavailable = "Hệ thống tạm thời không lưu được khoá bí mật. Vui lòng thử lại sau."
)

// validSecretInput refuses, before identity is called, what cannot be anybody's app secret: empty,
// or holding whitespace / a control character / U+FFFD (a lone surrogate escape decodes to it). The
// FULL shape rule — the byte bound, "printable" — is identity's (app.validAppSecret), answered as
// INVALID_ARGUMENT → 422 below; it is not copied here so the two cannot disagree on the boundary.
func validSecretInput(s secret.Secret) bool {
	if s.Rong() {
		return false
	}
	for _, r := range string(s.Lo()) {
		if r == utf8.RuneError || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// communeHoldsDedicatedApp answers 404 unless appID is one of THIS commune's dedicated apps, on or
// off. The same 404 for another commune's App ID, the shared app and an unknown one (ADR 0070: "App
// ID của xã khác/app chung/không biết cùng một 404"). Liveness is NOT checked here: for Set it is
// identity's to decide (it reads the binding itself, live), and Retire is allowed on a turned-off
// App ID by design.
func (h *operatorHandlers) communeHoldsDedicatedApp(w http.ResponseWriter, r *http.Request, id, appID string) bool {
	_, apps, err := h.d.Registry.Commune(r.Context(), id)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return false
	}
	for _, a := range apps {
		if a.AppID == appID && a.Mode == domain.CheDoRieng {
			return true
		}
	}
	h.writeRegistryError(w, r, store.ErrMiniAppNotInCommune)
	return false
}

// secretCallFailed answers a secret RPC that returned an error. Logs the class only — never the
// request, never the status message.
func (h *operatorHandlers) secretCallFailed(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, operatorclient.ErrMiniAppNotBound):
		httpx.WriteError(w, http.StatusConflict, "mini_app_not_bound", msgMiniAppNotBound, "")
	case errors.Is(err, operatorclient.ErrArgumentRefused):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_secret", msgSecretInvalid, "")
	default:
		// Unreachable identity, deadline, realm not configured, contract fault, or a wiring fault
		// of ours: the act did not happen. 503 — never "refused", never "signed out".
		httpx.WriteError(w, http.StatusServiceUnavailable, "mini_app_secret_unavailable", msgSecretUnavailable, "")
	}
}

// secretOutcome answers the two non-ACCEPTED outcomes; false means it wrote nothing.
func secretOutcome(w http.ResponseWriter, o operatorclient.Outcome) bool {
	switch o {
	case operatorclient.OutcomeSessionNotLive:
		// Possible between our Resolve and identity's own: the session ended in between.
		opauth.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgSignInRefused, "")
		return true
	case operatorclient.OutcomePermissionDenied:
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Tài khoản vận hành của bạn không có quyền thực hiện thao tác này.", "")
		return true
	case operatorclient.OutcomeAccepted:
		return false
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, "mini_app_secret_unavailable", msgSecretUnavailable, "")
	return true
}

// setMiniAppSecret forwards a new Zalo app secret for one of the commune's dedicated App IDs.
func (h *operatorHandlers) setMiniAppSecret(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	appID, ok := h.pathMiniApp(w, r)
	if !ok {
		return
	}
	var b miniAppSecretBody
	if !decodeBody(w, r, &b) {
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if !validSecretInput(b.Secret.Secret) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_secret", msgSecretInvalid, "")
		return
	}
	if !h.communeHoldsDedicatedApp(w, r, id, appID) {
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.SetMiniAppSecret(ctx, operatorclient.SetMiniAppSecretRequest{
		Token: tok, AppID: appID, AppSecret: b.Secret.Secret, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		h.secretCallFailed(w, r, err)
		return
	}
	if secretOutcome(w, res.Outcome) {
		return
	}
	v := res.Version
	writeJSON(w, http.StatusOK, miniAppSecretView{AppID: appID, Version: v.Version, SetAt: v.SetAt.UTC(), SetBy: v.SetBy})
}

// retireMiniAppSecret ends the live secret of one of the commune's dedicated App IDs — the manual
// retry when the automatic retirement after a change/detach did not happen.
func (h *operatorHandlers) retireMiniAppSecret(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	appID, ok := h.pathMiniApp(w, r)
	if !ok {
		return
	}
	var b miniAppSecretRetireBody
	if !decodeBody(w, r, &b) {
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if !h.communeHoldsDedicatedApp(w, r, id, appID) {
		return
	}
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.RetireMiniAppSecret(ctx, operatorclient.RetireMiniAppSecretRequest{
		Token: tok, AppID: appID, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	if errors.Is(err, operatorclient.ErrNoLiveSecret) {
		writeJSON(w, http.StatusOK, miniAppSecretRetirementView{AppID: appID, Retired: false})
		return
	}
	if err != nil {
		h.secretCallFailed(w, r, err)
		return
	}
	if secretOutcome(w, res.Outcome) {
		return
	}
	at := res.Retirement.RetiredAt.UTC()
	writeJSON(w, http.StatusOK, miniAppSecretRetirementView{AppID: appID, Retired: true,
		RetiredVersion: res.Retirement.RetiredVersion, RetiredAt: &at, RetiredBy: res.Retirement.RetiredBy})
}

// retireAfterUnbind retires the secret of an App ID the platform has JUST turned off (ADR 0070 #4:
// "đổi/gỡ thì khoá của App ID cũ tự thu hồi"). Called only AFTER platform's transaction committed;
// its failure never touches the binding — it is reported on the response, and logged with the
// commune, the App ID (a public Zalo identifier) and the class of failure only.
//
// The reason passed to identity is the operator's reason for the change or removal: the same act,
// the same justification (operator.proto, RetireMiniAppSecretRequest.reason).
//
// context.WithoutCancel: the binding is already changed, so a client that hangs up now must not
// also cancel the follow-up. The call keeps its own deadline (operatorclient.CallTimeout).
func (h *operatorHandlers) retireAfterUnbind(ctx context.Context, r *http.Request, appID, reason string) (bool, string) {
	tok, _ := opauth.TokenFrom(r)
	res, err := h.d.Identity.RetireMiniAppSecret(context.WithoutCancel(ctx), operatorclient.RetireMiniAppSecretRequest{
		Token: tok, AppID: appID, Reason: reason, ClientIP: httpx.ClientIP(r),
	})
	problem := ""
	switch {
	case errors.Is(err, operatorclient.ErrNoLiveSecret):
		return true, "" // nothing left to retire: done (operator.proto)
	case errors.Is(err, operatorclient.ErrArgumentRefused):
		problem = "refused"
	case err != nil:
		problem = "identity_unavailable"
	case res.Outcome == operatorclient.OutcomeAccepted:
		return true, ""
	case res.Outcome == operatorclient.OutcomeSessionNotLive:
		problem = "session_not_live"
	case res.Outcome == operatorclient.OutcomePermissionDenied:
		problem = "forbidden"
	default:
		problem = "identity_unavailable"
	}
	t, _ := tenant.From(ctx)
	h.d.Log.WarnContext(ctx, "khu vận hành: App ID đã tắt nhưng khoá bí mật của nó chưa thu hồi — cần bấm thu hồi lại",
		"event", "mini_app.secret_retire_pending", "tenant", t.String(), "app_id", appID, "problem", problem)
	return false, problem
}

// respondCommuneAfterChange is respondCommune plus the retirement report. retired == nil means no
// App ID was turned off by this request.
func (h *operatorHandlers) respondCommuneAfterChange(w http.ResponseWriter, r *http.Request, id string, status int, retired *bool, problem string) {
	c, apps, err := h.d.Registry.Commune(r.Context(), id)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	writeJSON(w, status, miniAppChangeView{communeDetailView: toDetailView(c, apps),
		SecretRetired: retired, SecretRetirementError: problem})
}
