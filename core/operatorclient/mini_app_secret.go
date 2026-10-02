package operatorclient

// The two RPCs of OperatorService that act ON a commune: SetMiniAppSecret and RetireMiniAppSecret
// (proto/vigov/identity/v1/operator.proto; ADR 0070 #4, §"Bổ sung 02/10/2026" #6).
//
// THE COMMUNE IS THE CONTEXT'S. Both are absent from core/grpcx.methodsWithoutTenant, so
// grpcx.UnaryClientInterceptor writes x-tenant-id from ctx. A ctx without a commune is refused HERE
// with ErrInvalidRequest — a wiring fault — rather than by the interceptor's INVALID_ARGUMENT, which
// would read to the caller as "identity refused the request shape" and reach the operator as a 422.
//
// THE SECRET crosses this API as secret.Secret and is taken out with raw() only at the line that
// builds the request. Nothing here logs a request, a response, or a status message.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

var (
	// ErrMiniAppNotBound is FAILED_PRECONDITION on SetMiniAppSecret: the platform does not bind this
	// App ID, live, to the target commune as its own app (operator.proto: one answer for five causes,
	// so it says nothing about another commune). The caller answers 409. Nothing was written.
	//
	// Not ErrRealmNotConfigured, although the code is the same: on this RPC the realm meaning is
	// excluded by the caller having resolved the session on the same request (operator.proto,
	// SetMiniAppSecret).
	ErrMiniAppNotBound = errors.New("operatorclient: the App ID is not bound, live, to the target commune")

	// ErrNoLiveSecret is NOT_FOUND on RetireMiniAppSecret: nothing live to retire in the target
	// commune. For the automatic revoke after a change or removal of App ID it means DONE.
	ErrNoLiveSecret = errors.New("operatorclient: no live Mini App settings for this App ID in the target commune")

	// ErrArgumentRefused is INVALID_ARGUMENT from identity on the two secret RPCs: a shape identity
	// refuses (operator.proto lists them). The caller validated App ID and reason first, so for a
	// conforming caller what remains is the secret's shape. The status message is never surfaced.
	ErrArgumentRefused = errors.New("operatorclient: identity refused the request shape")
)

// MiniAppSecretVersion is what may be shown about a secret just set: that it exists, since when, by
// whom. NEVER the value or anything derived from it.
type MiniAppSecretVersion struct {
	AppID   string
	Version string
	SetAt   time.Time
	// SetBy is the operator's BUSINESS CODE, `VH-00001` (rule 6, invariant 8).
	SetBy string
}

// MiniAppSecretRetirement is what was retired.
type MiniAppSecretRetirement struct {
	AppID          string
	RetiredVersion string
	RetiredAt      time.Time
	RetiredBy      string
}

// SetMiniAppSecretRequest — the target commune is NOT a field: it is the context's.
type SetMiniAppSecretRequest struct {
	// Token is the `op1.` cookie verbatim; identity resolves the operator from it.
	Token secret.Secret
	AppID string
	// AppSecret is the commune's Zalo app secret — write-only, verbatim (not trimmed).
	AppSecret secret.Secret
	// Reason as typed in the confirmation dialog — free text, never logged.
	Reason   string
	ClientIP string
}

// SetMiniAppSecretResult: Version is set exactly when Outcome is OutcomeAccepted.
type SetMiniAppSecretResult struct {
	Outcome Outcome
	Version MiniAppSecretVersion
}

// RetireMiniAppSecretRequest — the target commune is the context's.
type RetireMiniAppSecretRequest struct {
	Token    secret.Secret
	AppID    string
	Reason   string
	ClientIP string
}

// RetireMiniAppSecretResult: Retirement is set exactly when Outcome is OutcomeAccepted.
type RetireMiniAppSecretResult struct {
	Outcome    Outcome
	Retirement MiniAppSecretRetirement
}

// requireCommune refuses a call whose context names no target commune.
func requireCommune(ctx context.Context, rpc string) error {
	if _, ok := tenant.From(ctx); !ok {
		return fmt.Errorf("%w: %s without a target commune in the context", ErrInvalidRequest, rpc)
	}
	return nil
}

// secretCallFailed reads the status of a secret RPC. NOT_FOUND on Retire is an ordinary answer and
// logs nothing; every other failure logs the RPC name and the gRPC code only.
func (c *Client) secretCallFailed(ctx context.Context, rpc string, err error) error {
	code := status.Code(err)
	switch {
	case code == codes.NotFound && rpc == "RetireMiniAppSecret":
		return fmt.Errorf("%w: %s", ErrNoLiveSecret, rpc)
	case code == codes.FailedPrecondition && rpc == "SetMiniAppSecret":
		return fmt.Errorf("%w: %s", ErrMiniAppNotBound, rpc)
	case code == codes.InvalidArgument:
		// A shape refusal, not an outage — but logged, because a conforming caller pre-checks all
		// but the secret: a burst of these is a disagreement between the two ends worth seeing.
		c.log.WarnContext(ctx, "OperatorService từ chối dạng yêu cầu", "rpc", rpc, "ma_loi", code.String())
		return fmt.Errorf("%w: %s", ErrArgumentRefused, rpc)
	}
	// Wrapped with %w for errors.Is on the status; the status message is identity's and by contract
	// never quotes a value (operator.proto, service table).
	return c.callFailed(ctx, rpc, err)
}

// SetMiniAppSecret is OperatorService.SetMiniAppSecret, for the commune in ctx.
//
//	err == nil        Outcome ACCEPTED (Version set), SESSION_NOT_LIVE (401) or PERMISSION_DENIED (403)
//	ErrMiniAppNotBound   409 — bind or turn on the App ID first
//	ErrArgumentRefused   422
//	ErrInvalidRequest    a wiring fault in the caller (empty token, no commune) — nothing sent
//	any other error      the call did not happen → 503
func (c *Client) SetMiniAppSecret(ctx context.Context, req SetMiniAppSecretRequest) (SetMiniAppSecretResult, error) {
	const rpc = "SetMiniAppSecret"
	if req.Token.Rong() {
		return SetMiniAppSecretResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	if err := requireCommune(ctx, rpc); err != nil {
		return SetMiniAppSecretResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.SetMiniAppSecret(ctx, &identityv1.SetMiniAppSecretRequest{
		SessionToken: raw(req.Token),
		AppId:        req.AppID,
		AppSecret:    raw(req.AppSecret),
		Reason:       req.Reason,
		ClientIp:     req.ClientIP,
	})
	if err != nil {
		return SetMiniAppSecretResult{}, c.secretCallFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wNotLive, wDenied)
	if err != nil {
		return SetMiniAppSecretResult{}, err
	}
	if o == OutcomeRefused {
		// UNSPECIFIED: not in this RPC's subset. Read as a contract fault, never as "refused" —
		// there is no credential here for "refused" to be about.
		return SetMiniAppSecretResult{}, fmt.Errorf("%w: %s answered no outcome", ErrContract, rpc)
	}
	res := SetMiniAppSecretResult{Outcome: o}
	if o == OutcomeAccepted {
		v := ra.GetVersion()
		if v == nil || v.GetVersion() == "" || v.GetSetBy() == "" || v.GetSetAt() == nil {
			return SetMiniAppSecretResult{}, fmt.Errorf("%w: %s ACCEPTED without a complete version", ErrContract, rpc)
		}
		res.Version = MiniAppSecretVersion{AppID: v.GetAppId(), Version: v.GetVersion(),
			SetAt: v.GetSetAt().AsTime(), SetBy: v.GetSetBy()}
	}
	return res, nil
}

// RetireMiniAppSecret is OperatorService.RetireMiniAppSecret, for the commune in ctx.
//
//	err == nil        Outcome ACCEPTED (Retirement set), SESSION_NOT_LIVE or PERMISSION_DENIED
//	ErrNoLiveSecret      nothing live to retire — for the automatic revoke, DONE
//	ErrArgumentRefused   422
//	ErrInvalidRequest    a wiring fault in the caller — nothing sent
//	any other error      the call did not happen → 503
func (c *Client) RetireMiniAppSecret(ctx context.Context, req RetireMiniAppSecretRequest) (RetireMiniAppSecretResult, error) {
	const rpc = "RetireMiniAppSecret"
	if req.Token.Rong() {
		return RetireMiniAppSecretResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	if err := requireCommune(ctx, rpc); err != nil {
		return RetireMiniAppSecretResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.RetireMiniAppSecret(ctx, &identityv1.RetireMiniAppSecretRequest{
		SessionToken: raw(req.Token),
		AppId:        req.AppID,
		Reason:       req.Reason,
		ClientIp:     req.ClientIP,
	})
	if err != nil {
		return RetireMiniAppSecretResult{}, c.secretCallFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wNotLive, wDenied)
	if err != nil {
		return RetireMiniAppSecretResult{}, err
	}
	if o == OutcomeRefused {
		return RetireMiniAppSecretResult{}, fmt.Errorf("%w: %s answered no outcome", ErrContract, rpc)
	}
	res := RetireMiniAppSecretResult{Outcome: o}
	if o == OutcomeAccepted {
		v := ra.GetRetirement()
		if v == nil || v.GetRetiredVersion() == "" || v.GetRetiredBy() == "" || v.GetRetiredAt() == nil {
			return RetireMiniAppSecretResult{}, fmt.Errorf("%w: %s ACCEPTED without a complete retirement", ErrContract, rpc)
		}
		res.Retirement = MiniAppSecretRetirement{AppID: v.GetAppId(), RetiredVersion: v.GetRetiredVersion(),
			RetiredAt: v.GetRetiredAt().AsTime(), RetiredBy: v.GetRetiredBy()}
	}
	return res, nil
}
