package app

// OwnAppSignIn — a commune's OWN Zalo Mini App signing a citizen in straight to ViGov (ADR 0066),
// POST /api/v1/citizen-sessions. The shared ViHAT app keeps signing in through vihat-miniapp.
//
// WHY HERE AND NOT THROUGH THE BRIDGE: the exchange of accessToken/phoneToken is signed with THAT
// App ID's secret, and since ADR 0066 identity holds that secret (sealed, per commune, migration
// 0021). Which repo owns a Zalo surface is decided by which secret signs it (ADR 0032).
//
// # THE ORDER
//
//  0. Shape, before any outbound call (internal/http has already rate-limited the client).
//  1. ResolveMiniApp: must be a commune's OWN app (MODE_COMMUNE) bound to an active commune. The
//     shared app, an unknown App ID and an inactive commune give ONE answer (ErrOwnAppNotReady):
//     the shared app's sign-in stays on vihat-miniapp, and nothing here tells a prober which.
//  2. In THAT commune's context: its live mini_app_secret row. Missing = the same one answer.
//  3. Demo (`{appId, demoIdentity:true}`): the row must have demo identity ON, else
//     ErrOwnAppPhoneRequired — the same answer vihat-miniapp gives an own app with no phoneToken.
//     NO Zalo call at all, not even getAccessToken's check (ADR 0066: an app awaiting review is
//     refused by Zalo for every call).
//     Normal: open the sealed secret, then AccountID(accessToken), then Phone(..., secret) —
//     account id FIRST because the phoneToken may be single-use (internal/zalo package comment).
//     A secret that does not open is OUR fault: 503 and an operator alert, never "sign in again".
//  4. CauPhienCongDan.Mo, IN-PROCESS, with RequireOwnAppOf = the commune from step 1: Mo resolves
//     the app again and refuses unless the answer is unchanged, so a binding moved between steps 1
//     and 4 cannot open a session in a commune whose secret was never checked. Mo owns the one
//     transaction and its audit entries (rule 6).
//
// # WHAT `--demo` GIVES AWAY
//
// A session with NO phone verification, for an App ID an operator switched on (default off, ADR
// 0066 — "phải tắt trước khi app được duyệt"). It names the fixed fake identity
// domain.DemoIdentityPhone, so it reads only that identity's own records in that commune (rule 4).
// Every such session is logged at WARN and its audit entry carries danh_tinh_demo=true.
//
// # NOTHING PERSONAL IS LOGGED
//
// Not the tokens, not the Zalo account id, not the number, not the secret (rules 3 and 8). app_id,
// the commune id and the session id are not personal data and are logged.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/internal/zalo"
)

// The error classes the HTTP handler maps onto vihat-miniapp's status table. Messages are generic:
// nothing the caller sent is echoed.
var (
	ErrOwnAppRequestInvalid  = errors.New("own app sign-in: request is not valid")                // 400
	ErrOwnAppPhoneRequired   = errors.New("own app sign-in: a phone token is required")           // 400
	ErrOwnAppTokenInvalid    = errors.New("own app sign-in: Zalo refused the token")              // 401
	ErrOwnAppNotReady        = errors.New("own app sign-in: app not ready for a commune")         // 422
	ErrOwnAppZaloUnreachable = errors.New("own app sign-in: Zalo unreachable")                    // 502
	ErrOwnAppUnavailable     = errors.New("own app sign-in: temporarily unable to open sessions") // 503
)

// OwnAppSettingsReader is the sign-in read of *idstore.MiniAppSecretStore.
type OwnAppSettingsReader interface {
	Live(ctx context.Context, appID string) (idstore.MiniAppSecret, bool, error)
}

// SecretOpener is the opening half of *crypto.Envelope.
type SecretOpener interface {
	Open(ctx context.Context, sealed, aad []byte) (secret.Secret, error)
}

// ZaloExchange is *zalo.Client.
type ZaloExchange interface {
	AccountID(ctx context.Context, accessToken string) (string, error)
	Phone(ctx context.Context, accessToken, phoneToken string, appSecret secret.Secret) (string, error)
}

// CitizenSessionOpener is *CauPhienCongDan.
type CitizenSessionOpener interface {
	Mo(ctx context.Context, yc YeuCauMoPhienCau) (KetQuaMoPhienCau, error)
}

// OwnAppSignInRequest is the HTTP body plus what the edge knows. Tokens are credentials (rule 8).
type OwnAppSignInRequest struct {
	AppID        string
	AccessToken  string
	PhoneToken   string
	DemoIdentity bool
	IP           string
	Device       string
}

// OwnAppSignIn is the use case. See the file comment for the order.
type OwnAppSignIn struct {
	platform MiniAppBinding
	settings OwnAppSettingsReader
	opener   SecretOpener
	zalo     ZaloExchange
	sessions CitizenSessionOpener
	log      *slog.Logger
}

func NewOwnAppSignIn(platform MiniAppBinding, settings OwnAppSettingsReader, opener SecretOpener,
	z ZaloExchange, sessions CitizenSessionOpener, log *slog.Logger) *OwnAppSignIn {
	if platform == nil || settings == nil || opener == nil || z == nil || sessions == nil {
		panic("app.NewOwnAppSignIn: thiếu phụ thuộc — POST /api/v1/citizen-sessions sẽ panic ở lượt đăng nhập đầu tiên")
	}
	if log == nil {
		log = slog.Default()
	}
	return &OwnAppSignIn{platform: platform, settings: settings, opener: opener, zalo: z, sessions: sessions, log: log}
}

// SignIn opens a citizen session for the own app named in req.
func (uc *OwnAppSignIn) SignIn(ctx context.Context, req OwnAppSignInRequest) (KetQuaMoPhienCau, error) {
	appID := strings.TrimSpace(req.AppID)

	// --- 0. shape --------------------------------------------------------------------------------
	switch {
	case !domain.ValidMiniAppID(appID):
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: appId", ErrOwnAppRequestInvalid)
	case req.DemoIdentity && (req.AccessToken != "" || req.PhoneToken != ""):
		// A demo body carrying tokens is a client wired to two modes at once; refusing it keeps the
		// demo path from ever touching a credential.
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: demo body carries tokens", ErrOwnAppRequestInvalid)
	case !req.DemoIdentity && req.AccessToken == "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: accessToken", ErrOwnAppRequestInvalid)
	case !req.DemoIdentity && req.PhoneToken == "":
		// The exchange of the phoneToken with THIS App ID's secret is the only thing that verifies
		// the App ID the client claims (internal/zalo package comment). Refused before any call.
		return KetQuaMoPhienCau{}, ErrOwnAppPhoneRequired
	}

	// --- 1. which commune's own app ----------------------------------------------------------------
	app, found, err := uc.platform.MiniApp(ctx, appID)
	if err != nil {
		uc.log.WarnContext(ctx, "đăng nhập app riêng: không tra được Mini App ở dịch vụ nền tảng — không phát phiên",
			"app_id", appID, "err", err)
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrOwnAppUnavailable, err)
	}
	if !found || app.CheDo != platformclient.CheDoAppRieng || app.XaRieng == "" {
		// Unknown, the shared app (vihat-miniapp's), and a bound commune not active: ONE answer.
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: not a live commune's own app", ErrOwnAppNotReady)
	}
	xa := app.XaRieng
	ctxXa := tenant.Into(ctx, xa)

	// --- 2. that commune's settings for the App ID ---------------------------------------------------
	// Scoped: Live reads through db.For(ctxXa) — the commune from step 1, never from the request.
	cfg, found, err := uc.settings.Live(ctxXa, appID)
	if err != nil {
		uc.log.ErrorContext(ctx, "đăng nhập app riêng: không đọc được cấu hình App ID", "app_id", appID, "xa", string(xa), "err", err)
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrOwnAppUnavailable, err)
	}
	if !found {
		uc.log.WarnContext(ctx, "đăng nhập app riêng: App ID chưa có cấu hình đăng nhập ở xã — operatorctl mini-app-secret set",
			"app_id", appID, "xa", string(xa))
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: no live settings", ErrOwnAppNotReady)
	}

	yc := YeuCauMoPhienCau{AppID: appID, IP: req.IP, ThietBi: req.Device, RequireOwnAppOf: xa}

	// --- 3. who --------------------------------------------------------------------------------------
	if req.DemoIdentity {
		if !cfg.DemoIdentity {
			return KetQuaMoPhienCau{}, ErrOwnAppPhoneRequired
		}
		yc.MaZalo = domain.DemoZaloAccountID(appID)
		yc.SoDaXacThuc = domain.DemoIdentityPhone
		yc.DemoIdentity = true
	} else {
		if cfg.Sealed == nil {
			// Demo-only settings (stage 2 of ADR 0066's lifecycle): there is no secret to verify a
			// real sign-in with. Configuration, not the citizen's token.
			uc.log.WarnContext(ctx, "đăng nhập app riêng: App ID chỉ bật demo, chưa có secret — từ chối lượt đăng nhập thật",
				"app_id", appID, "xa", string(xa))
			return KetQuaMoPhienCau{}, fmt.Errorf("%w: no secret set", ErrOwnAppNotReady)
		}
		appSecret, err := uc.opener.Open(ctxXa, cfg.Sealed, miniAppSecretAAD(ctxXa, appID))
		if err != nil {
			// Opened BEFORE any Zalo call: a broken secret must not spend the citizen's phoneToken.
			level := slog.LevelError
			if errors.Is(err, crypto.ErrNotConfigured) {
				level = slog.LevelWarn
			}
			uc.log.Log(ctx, level, "CẢNH BÁO VẬN HÀNH: không mở được secret của App ID — kiểm SECRET_ENCRYPTION_KEYS và đặt lại secret",
				"app_id", appID, "xa", string(xa), "err", err)
			return KetQuaMoPhienCau{}, fmt.Errorf("%w: open secret: %w", ErrOwnAppUnavailable, err)
		}
		defer clear(appSecret)

		maZalo, err := uc.zalo.AccountID(ctx, req.AccessToken)
		if err != nil {
			return KetQuaMoPhienCau{}, uc.zaloError(ctx, appID, err)
		}
		so, err := uc.zalo.Phone(ctx, req.AccessToken, req.PhoneToken, appSecret)
		if err != nil {
			return KetQuaMoPhienCau{}, uc.zaloError(ctx, appID, err)
		}
		yc.MaZalo, yc.SoDaXacThuc = maZalo, so
	}

	// --- 4. the session --------------------------------------------------------------------------------
	kq, err := uc.sessions.Mo(ctx, yc)
	if err != nil {
		switch {
		case errors.Is(err, ErrCauAppChuaSanSang), errors.Is(err, ErrCauXaKhongHoatDong):
			return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrOwnAppNotReady, err)
		case errors.Is(err, ErrCauYeuCauSai):
			// Built here, so this is a defect of this file, not the citizen's request.
			uc.log.ErrorContext(ctx, "đăng nhập app riêng: cầu phiên từ chối yêu cầu như lỗi nối dây", "app_id", appID, "err", err)
			return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrOwnAppRequestInvalid, err)
		}
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrOwnAppUnavailable, err)
	}
	if kq.Token == "" {
		// An own app always has its commune; a session-less answer here breaks Mo's contract.
		uc.log.ErrorContext(ctx, "đăng nhập app riêng: cầu phiên không phát token cho app riêng", "app_id", appID, "xa", string(xa))
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: no token issued", ErrOwnAppUnavailable)
	}

	if req.DemoIdentity {
		uc.log.WarnContext(ctx, "đăng nhập app riêng bằng DANH TÍNH DEMO — không xác minh số; tắt trước khi Zalo duyệt app (ADR 0066)",
			"app_id", appID, "demo_identity", true, "xa", string(xa), "phien_id", kq.Sid)
	} else {
		uc.log.InfoContext(ctx, "mở phiên công dân qua app riêng", "app_id", appID, "xa", string(xa), "phien_id", kq.Sid)
	}
	return kq, nil
}

// zaloError keeps Zalo's two classes apart (internal/zalo: merging them tells a citizen to sign in
// again while the fault is ours). The wrapped error carries only a status or a numeric code.
func (uc *OwnAppSignIn) zaloError(ctx context.Context, appID string, err error) error {
	if errors.Is(err, zalo.ErrTokenInvalid) {
		return fmt.Errorf("%w: %w", ErrOwnAppTokenInvalid, err)
	}
	uc.log.ErrorContext(ctx, "đăng nhập app riêng: không đổi được token với Zalo", "app_id", appID, "err", err)
	return fmt.Errorf("%w: %w", ErrOwnAppZaloUnreachable, err)
}
