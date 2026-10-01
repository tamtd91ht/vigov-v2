package http

// POST /api/v1/citizen-sessions — a commune's OWN Zalo Mini App exchanging its Zalo tokens for a
// ViGov citizen session (ADR 0066). The use case is app.OwnAppSignIn; this file only translates.
//
// THE WIRE IS vihat-miniapp's POST /api/v1/sessions (internal/httpapi/sessions_vigov.go), byte for
// byte where the Mini App reads it: the 201 body `{vigovSession:{...}}`, the error key `message`,
// the status table 201 / 400 / 401 / 422 / 429 / 502 / 503 (ADR 0066 decision row 1). citizen-app
// then reads one shape whichever repo signed the citizen in. Errors here also carry `code` and
// `trace_id` (httpx.Error) — extra keys the Mini App's reader ignores.
//
// NOT /api/v1/sessions: that path is the STAFF sign-in on this same service.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-identity/internal/app"
)

// CitizenSessionsPath is registered on the outer mux by cmd/server so the path reaches the PUBLIC
// chain on the reserved API host. routes_cong_dan.go spells it as a literal for tools/apidoc;
// citizen_sessions_test.go asserts the two agree.
const CitizenSessionsPath = "/api/v1/citizen-sessions"

// citizenSessionBodyLimit — the body is three short strings. 8 KB is vihat-miniapp's figure.
const citizenSessionBodyLimit = 8 << 10

// OwnAppSignInner is *app.OwnAppSignIn.
type OwnAppSignInner interface {
	SignIn(ctx context.Context, req app.OwnAppSignInRequest) (app.KetQuaMoPhienCau, error)
}

// citizenSessionIn is the request body. Exactly one of two shapes:
//
//	{"appId": "...", "accessToken": "...", "phoneToken": "..."}   normal
//	{"appId": "...", "demoIdentity": true}                        `--demo`, no tokens, no Zalo call
//
// Tokens are credentials: never logged, never echoed (rules 3, 8).
type citizenSessionIn struct {
	AppID        string `json:"appId"`
	AccessToken  string `json:"accessToken"`
	PhoneToken   string `json:"phoneToken"`
	DemoIdentity bool   `json:"demoIdentity"`
}

// citizenSessionsOut is the 201 body — vihat-miniapp's phanHoiPhienViGov, key for key.
type citizenSessionsOut struct {
	VigovSession citizenSessionOut `json:"vigovSession"`
}

// citizenSessionOut — no tenant_id: the commune of a citizen comes FROM THE SESSION on the server,
// and a client holding a commune id is a client that will one day send it back (rule 1, forbidden
// #2). communePrimaryHost is a lookup key for `?host=`, not a commune reference.
type citizenSessionOut struct {
	Token              string `json:"token,omitempty" apidoc:"bi-mat-co-chu-y:Bearer token phiên công dân ViGov — Mini App không có cookie nên phải nhận token trong thân; người dùng chốt 01/10/2026 thân 201 giữ y như vihat-miniapp {vigovSession:{token,...}} (ADR 0066 quyết định 1). Không lưu, không log, phản hồi no-store; thu hồi được qua sổ phiên công dân."`
	ExpiresAt          string `json:"expiresAt,omitempty"`
	TenantDisplayName  string `json:"tenantDisplayName"`
	PhoneVerified      bool   `json:"phoneVerified"`
	CommunePrimaryHost string `json:"communePrimaryHost"`
}

// The citizen reads these. vihat-miniapp's sentences, so the two sign-in paths speak alike; never a
// technical code, a column name or Zalo's own message.
const (
	msgCitizenSessionBadRequest  = "Yêu cầu không hợp lệ. Vui lòng mở lại ứng dụng và thử đăng nhập lại."
	msgCitizenSessionNeedsPhone  = "Vui lòng cho phép ứng dụng dùng số điện thoại Zalo của bạn để đăng nhập, rồi thử lại."
	msgCitizenSessionTokenExpiry = "Phiên đăng nhập Zalo đã hết hạn. Vui lòng đóng và mở lại ứng dụng để đăng nhập lại."
	msgCitizenSessionNotReady    = "Ứng dụng chưa sẵn sàng cho địa phương này. Vui lòng quét lại mã QR do địa phương cung cấp hoặc thử lại sau."
	msgCitizenSessionTooMany     = "Bạn đã thử đăng nhập quá nhiều lần. Vui lòng chờ vài phút rồi thử lại."
	msgCitizenSessionZaloDown    = "Hiện chưa kết nối được tới Zalo. Vui lòng thử lại sau ít phút."
	msgCitizenSessionUnavailable = "Chức năng đăng nhập đang tạm ngưng. Vui lòng thử lại sau ít phút."
)

// CitizenSessions opens a citizen session for a commune's own app. POST /api/v1/citizen-sessions
//
// ORDER: the rate limit FIRST — before the body is read, before any outbound call — so a scanner
// costs this pod a map lookup per knock and the platform and Zalo nothing.
func (h *HandlerCongKhai) CitizenSessions(w http.ResponseWriter, r *http.Request) {
	// Every answer of this route is no-store: the 201 carries a bearer token, and the refusals must
	// not be cached by a webview into a 429 that outlives the window.
	w.Header().Set("Cache-Control", "no-store")

	if ok, retry := h.limiter.allow(httpx.ClientIP(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		httpx.WriteError(w, http.StatusTooManyRequests, "too_many_attempts", msgCitizenSessionTooMany, "")
		return
	}

	var in citizenSessionIn
	if !decodeStrict(w, r, &in) {
		// The decoder's message is NOT returned: it can quote what the client sent.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgCitizenSessionBadRequest, "")
		return
	}

	kq, err := h.d.CitizenSessions.SignIn(r.Context(), app.OwnAppSignInRequest{
		AppID:        in.AppID,
		AccessToken:  in.AccessToken,
		PhoneToken:   in.PhoneToken,
		DemoIdentity: in.DemoIdentity,
		IP:           httpx.ClientIP(r),
		Device:       r.UserAgent(),
	})
	if err != nil {
		h.citizenSessionError(w, r, err)
		return
	}

	out := citizenSessionOut{
		Token:              kq.Token,
		TenantDisplayName:  kq.TenXa,
		PhoneVerified:      kq.DaCoSo,
		CommunePrimaryHost: kq.TenMienChinh,
	}
	if !kq.HetHan.IsZero() {
		out.ExpiresAt = kq.HetHan.UTC().Format(time.RFC3339)
	}
	vietJSON(w, http.StatusCreated, citizenSessionsOut{VigovSession: out})
}

func (h *HandlerCongKhai) citizenSessionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrOwnAppPhoneRequired):
		httpx.WriteError(w, http.StatusBadRequest, "phone_required", msgCitizenSessionNeedsPhone, "")
	case errors.Is(err, app.ErrOwnAppRequestInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgCitizenSessionBadRequest, "")
	case errors.Is(err, app.ErrOwnAppTokenInvalid):
		httpx.WriteError(w, http.StatusUnauthorized, "zalo_token_invalid", msgCitizenSessionTokenExpiry, "")
	case errors.Is(err, app.ErrOwnAppNotReady):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "app_not_ready", msgCitizenSessionNotReady, "")
	case errors.Is(err, app.ErrOwnAppZaloUnreachable):
		httpx.WriteError(w, http.StatusBadGateway, "zalo_unreachable", msgCitizenSessionZaloDown, "")
	default:
		if !errors.Is(err, app.ErrOwnAppUnavailable) {
			// Not a class the use case names: logged, then the contract's "nothing was issued".
			h.d.Log.ErrorContext(r.Context(), "đăng nhập app riêng: lỗi không phân loại", "err", err)
		}
		httpx.WriteError(w, http.StatusServiceUnavailable, "sign_in_unavailable", msgCitizenSessionUnavailable, "")
	}
}

// decodeStrict reads exactly one JSON object with known fields only, within the body cap.
func decodeStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, citizenSessionBodyLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return false
	}
	// A second value after the object — `{...}{...}` — is a malformed request, not a bonus.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return false
	}
	return true
}
