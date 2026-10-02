// Package operatorclient is how service-platform — the operator area's only HTTP edge — talks to
// identity's OperatorService (proto/vigov/identity/v1/operator.proto; ADR 0048 §"Chốt của chủ dự
// án — 01/10/2026" #2).
//
// WHY IT LIVES IN core/ AND NOT INSIDE service-platform: the same reason identityclient does. The
// transport, the deadline and — above all — the reading of what a failure means belong in one place,
// next to the other identity client, so the two cannot drift into two answers to "identity did not
// answer".
//
// THE ONE RULE THIS PACKAGE EXISTS TO HOLD: an error is NEVER folded into an answer. A call that did
// not happen (unreachable, deadline, caller key refused, broken contract, realm not configured) is
// an error, and the caller answers 503. It is never OutcomeRefused — that would count as a wrong
// password in the person's mind — and never "not signed in" — that would sign every operator out
// through the service that is down. Fail closed means "refuse to proceed", not "pretend to know".
//
// NO COMMUNE TRAVELS ON THE SEVEN REALM CALLS. They are on core/grpcx.methodsWithoutTenant (owner's
// decision 2026-10-01), and grpcx.UnaryClientInterceptor sends no x-tenant-id for an exempt method
// even when the context holds a commune: the exemption depends on the list, never on the caller's
// state. THE TWO MINI APP SECRET CALLS (mini_app_secret.go) ARE NOT EXEMPT: they write a commune's
// own data, so the target commune travels in metadata from the context like any inter-service call
// (rule 2, invariant 8), and a context without one is refused here, before anything is sent.
//
// NOTHING HERE LOGS A REQUEST, A RESPONSE, OR ANY CREDENTIAL FIELD — at any level. The generated
// String() prints every field in full (operator.proto, last paragraph of the service comment):
// passwords, TOTP codes, recovery codes, bearer tokens, TOTP secrets, and the operator's email
// (personal data, rule 3). Credentials cross this package's API as secret.Secret, so a caller that
// logs a result struct prints `***`; the raw bytes are taken out with .Lo() only at the line that
// builds the request.
package operatorclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

// CallTimeout bounds one OperatorService call. THE SAME NUMBER AS identityclient.HanGoi, on
// purpose: ResolveOperatorSession runs on every operator request, exactly the hop shape HanGoi was
// sized for, and two deadlines on two hops of one request make the observed timeout depend on
// which dependency was slow. The sign-in RPCs do an argon2id verification plus a timing
// equaliser (service-identity operator_auth.go) — well inside three seconds; a stall shows as 503.
const CallTimeout = 3 * time.Second

var (
	// ErrInvalidRequest is a request this client refuses BEFORE sending: an empty session token
	// (the caller must not call without a cookie), or both second factors at once. Decided from the
	// request alone, so it says nothing about any account. The caller answers 400 (both factors) or
	// treats it as a wiring fault (empty token).
	ErrInvalidRequest = errors.New("operatorclient: invalid request")

	// ErrRealmNotConfigured is FAILED_PRECONDITION: identity has no operator keys. A deployment
	// fault — the caller answers 503 and alerts; it is never "refused".
	ErrRealmNotConfigured = errors.New("operatorclient: operator realm not configured in identity")

	// ErrContract is an answer that breaks the contract (ACCEPTED without its payload, an outcome
	// outside the RPC's subset, a principal without its business code). The caller answers 503: a
	// contract fault served as an ordinary answer is invisible for as long as the two ends disagree.
	ErrContract = errors.New("operatorclient: identity answered outside the contract")
)

// Outcome is what an RPC decided. THE ZERO VALUE IS OutcomeRefused, so an Outcome nobody set grants
// nothing.
type Outcome int

const (
	// OutcomeRefused is the ONE credential answer — never refined (operator.proto).
	OutcomeRefused Outcome = iota
	OutcomeAccepted
	// OutcomeEnrollmentRequired: Open only — password right, no second factor yet. Grants nothing.
	OutcomeEnrollmentRequired
	// OutcomeSessionNotLive: session-bound RPCs only — the caller answers 401 and clears the cookie.
	OutcomeSessionNotLive
	// OutcomeNewPasswordRejected: see the result's Refusal.
	OutcomeNewPasswordRejected
	// OutcomePermissionDenied: the Mini App secret RPCs only — a live session whose operator lacks
	// `ops.mini_app.manage`. The caller answers 403. Nothing was written.
	OutcomePermissionDenied
)

func (o Outcome) String() string {
	switch o {
	case OutcomeAccepted:
		return "accepted"
	case OutcomeEnrollmentRequired:
		return "enrollment_required"
	case OutcomeSessionNotLive:
		return "session_not_live"
	case OutcomeNewPasswordRejected:
		return "new_password_rejected"
	case OutcomePermissionDenied:
		return "permission_denied"
	}
	return "refused"
}

// SessionGrant is a session just opened.
type SessionGrant struct {
	// Token is the `op1.` bearer token — set it as a host-only, HttpOnly, Secure cookie on
	// OPERATOR_HOST and nowhere else (rule 1 forbidden #3, rule 13 invariant 4).
	Token secret.Secret
	// ExpiresAt is the ABSOLUTE expiry, for the cookie's lifetime only — idle expiry and revocation
	// can end the session sooner, and only Resolve says whether it is live.
	ExpiresAt time.Time
	// OperatorCode is `VH-00001`; may be logged.
	OperatorCode string
}

// Principal is the operator behind a live session, valid for the ONE request it authenticated —
// never cached, never written anywhere (rule 5, invariant 4).
type Principal struct {
	// OperatorID authorises; never written into a trail as "who".
	OperatorID string
	// OperatorCode is "who" in every audit entry (rule 6, invariant 8). Never empty here.
	OperatorCode string
	// PermissionKeys are `ops.*` keys, read live. May be empty: signed in, holds nothing.
	PermissionKeys []string
}

// NewPasswordRefusal says which password rule refused the new value — the rule, never the value.
type NewPasswordRefusal struct {
	Problem   identityv1.NewPasswordProblem
	MinLength uint32
	MaxLength uint32
}

// Client calls OperatorService over gRPC.
//
// THERE IS NO CACHE AND THERE MUST NOT BE ONE: hybrid, not cached, is the owner's decision (ADR
// 0048 §01/10 #2) — a cached Resolve would keep a revoked or idle-expired session working for the
// length of the cache.
//
// THE CONNECTION: Dial (dial.go) opens it over the SAME plaintext transport identityclient uses —
// the user's answer of 2026-10-01 to the rule 13 stop condition this channel raised ("chung nợ kênh
// gRPC nội cụm" — the same debt as the existing gRPC channels, and a go-live blocker). It carries
// operator passwords, TOTP codes and bearer tokens, so the NetworkPolicy rule platform → identity
// 9090 (deploy/base/mang/netpol.yaml, rule 11) must be on the real cluster before OPERATOR_HOST is
// set. A caller that has its own connection uses NewFromConn with UnaryInterceptors instead.
type Client struct {
	cl   identityv1.OperatorServiceClient
	log  *slog.Logger
	conn interface{ Close() error } // set by Dial only; nil on an injected client
}

// UnaryInterceptors is the chain the connection to identity MUST carry — pass it to
// grpc.WithChainUnaryInterceptor. Caller key first, as everywhere (identityclient.Dial).
// grpcx.UnaryClientInterceptor is kept although every OperatorService method is exempt: it is the
// one place that decides which RPCs may travel without a commune, and a method added to
// OperatorService but not to that list must be refused locally, not sent bare.
//
// key is the deployment's GRPC_CALLER_KEY; an empty one panics inside grpcx.UnaryClientCallerAuth,
// at wiring time.
func UnaryInterceptors(key secret.Secret) []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{grpcx.UnaryClientCallerAuth(key), grpcx.UnaryClientInterceptor()}
}

// NewFromConn builds a client on a connection the caller opened with UnaryInterceptors.
func NewFromConn(cc grpc.ClientConnInterface, log *slog.Logger) *Client {
	return New(identityv1.NewOperatorServiceClient(cc), log)
}

// New wraps an already-built client — for tests against a fake server.
func New(cl identityv1.OperatorServiceClient, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{cl: cl, log: log}
}

// callFailed logs the gRPC code of a call that did not happen and turns it into this package's
// error. Only the RPC name and the code are logged — the status message is identity's, and the
// request is never anywhere near this line.
func (c *Client) callFailed(ctx context.Context, rpc string, err error) error {
	code := status.Code(err)
	c.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không gọi được OperatorService", "rpc", rpc, "ma_loi", code.String())
	if code == codes.FailedPrecondition {
		return fmt.Errorf("%w: %s: %w", ErrRealmNotConfigured, rpc, err)
	}
	return fmt.Errorf("operatorclient: %s: %w", rpc, err)
}

// outcome maps the wire enum, restricted to the subset the RPC may answer. UNSPECIFIED is
// OutcomeRefused (operator.proto: "treats it as REFUSED"); a value outside the subset is ErrContract.
func outcome(rpc string, o identityv1.OperatorAuthOutcome, allowed ...identityv1.OperatorAuthOutcome) (Outcome, error) {
	if o == identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_UNSPECIFIED {
		return OutcomeRefused, nil
	}
	for _, a := range allowed {
		if o != a {
			continue
		}
		switch o {
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ACCEPTED:
			return OutcomeAccepted, nil
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_REFUSED:
			return OutcomeRefused, nil
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_ENROLLMENT_REQUIRED:
			return OutcomeEnrollmentRequired, nil
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_SESSION_NOT_LIVE:
			return OutcomeSessionNotLive, nil
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_NEW_PASSWORD_REJECTED:
			return OutcomeNewPasswordRejected, nil
		case identityv1.OperatorAuthOutcome_OPERATOR_AUTH_OUTCOME_PERMISSION_DENIED:
			return OutcomePermissionDenied, nil
		}
	}
	return OutcomeRefused, fmt.Errorf("%w: %s answered outcome %d", ErrContract, rpc, int32(o))
}

func grant(rpc string, g *identityv1.OperatorSessionGrant) (SessionGrant, error) {
	if g == nil || g.GetSessionToken() == "" || g.GetOperatorCode() == "" || g.GetExpiresAt() == nil {
		return SessionGrant{}, fmt.Errorf("%w: %s ACCEPTED without a complete session", ErrContract, rpc)
	}
	return SessionGrant{
		Token:        secret.Secret(g.GetSessionToken()),
		ExpiresAt:    g.GetExpiresAt().AsTime(),
		OperatorCode: g.GetOperatorCode(),
	}, nil
}

func refusal(rpc string, r *identityv1.NewPasswordRefusal) (NewPasswordRefusal, error) {
	if r == nil {
		return NewPasswordRefusal{}, fmt.Errorf("%w: %s NEW_PASSWORD_REJECTED without a refusal", ErrContract, rpc)
	}
	return NewPasswordRefusal{Problem: r.GetProblem(), MinLength: r.GetMinLength(), MaxLength: r.GetMaxLength()}, nil
}

func codes2secrets(rpc string, in []string) ([]secret.Secret, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("%w: %s ACCEPTED without recovery codes", ErrContract, rpc)
	}
	out := make([]secret.Secret, 0, len(in))
	for _, s := range in {
		if s == "" {
			return nil, fmt.Errorf("%w: %s returned an empty recovery code", ErrContract, rpc)
		}
		out = append(out, secret.Secret(s))
	}
	return out, nil
}

// raw takes the material out at the one line that builds a request. "" for an empty secret.
func raw(s secret.Secret) string { return string(s.Lo()) }
