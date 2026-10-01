// Package ratelimit counts attempts per key in fixed windows, for routes that must refuse a flood
// before it reaches the logic behind them.
//
// THE FIRST AND ONLY POLICY TODAY is the operator sign-in limit (ADR 0048 §"Chốt của chủ dự án —
// 01/10/2026" #3): 20 attempts per 15 minutes per IP, across the password, TOTP and recovery-code
// steps. Its reason is specific and is the reason the limiter fails CLOSED: an operator account is
// locked for 12 hours after 5 wrong attempts (§"Chốt bước 1", security amendment), so anybody who
// can guess an operator's email can lock them out for 12 hours unless the route is also bounded per
// address. A limiter that opens when Redis is down removes that bound exactly when nothing else
// notices.
//
// WHY A FIXED WINDOW, NOT A SLIDING ONE: the owner chose it (§01/10 #3), and it is one atomic
// INCR per attempt with no per-attempt memory. Its known cost — up to 2× the limit across a window
// boundary — is inside what the decision accepts; a different algorithm is a different threshold,
// which is a rule 13 stop condition, not a tuning knob.
//
// REDIS, AND ONLY FOR COUNTING. ADR 0010 allows Redis for cache and rate limiting and forbids it as
// durable storage. A counter here is a 15-minute guard holding one integer; losing it loses nothing
// but the count.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The operator sign-in threshold — a SECURITY THRESHOLD CHOSEN BY THE OWNER (ADR 0048 §"Chốt của
// chủ dự án — 01/10/2026" #3, rule 13). Constants, deliberately not environment variables: an env
// var would let a deployment loosen a decided security number without a review, which is rule 13
// forbidden #4. Changing either number is a rule 13 STOP CONDITION — ask the owner, then edit here.
const (
	OperatorSignInLimit  = 20
	OperatorSignInWindow = 15 * time.Minute
)

// Policy is one limit: at most Limit attempts per key per Window.
type Policy struct {
	// name goes into every key; it separates the counters of two policies keyed by the same subject.
	name   string
	limit  int64
	window time.Duration
}

// OperatorSignIn is the policy for the operator area's sign-in steps (password, TOTP, recovery
// code): ADR 0048 §01/10 #3.
var OperatorSignIn = Policy{name: "op-signin", limit: OperatorSignInLimit, window: OperatorSignInWindow}

// Key is one counter's identity. OPAQUE, built only by the constructors below, so that "which scope
// does this counter belong to" is decided once, by name, and never by a caller concatenating a
// string.
//
// THE SCOPE RULE (rule 1, invariant 7): a counter for something that belongs to a commune is
// prefixed `t:<tenant_id>`, or two communes share one budget and one commune's flood locks out the
// other's staff. The operator area belongs to NO commune (ADR 0048 condition #3), so its keys carry
// no tenant prefix — which is why its constructor names the realm. A commune-scoped constructor,
// when a caller first needs one, takes the tenant FROM ctx (tenant.From; rule 1, invariant 4 — never
// as an argument), refuses when there is none, and prefixes `t:<tenant_id>:`. It is not written
// yet: no route needs it, and an unused constructor is a shape nobody has checked.
type Key struct {
	subject string
}

// OperatorIPKey is the key of the operator sign-in limit: one counter per client address.
//
// ip is the address the edge observed (httpx.ClientIP — never a raw X-Forwarded-For). "" is
// allowed and shares one counter: every request the edge could not place is held to one budget
// together, the fail-closed direction.
func OperatorIPKey(ip string) Key { return Key{subject: "ip:" + ip} }

// Counter is the store: one atomic increment that also starts the window on the first hit.
//
// An interface so the limiter can be tested without Redis; RedisCounter is the production one.
type Counter interface {
	// Incr adds one to key and returns the new count and the time until the key's window ends.
	// A new key starts a window of length window; an existing key's window is never extended.
	Incr(ctx context.Context, key string, window time.Duration) (count int64, ttl time.Duration, err error)
}

// Limiter applies one Policy over one Counter.
type Limiter struct {
	c Counter
	p Policy
}

// ErrUnavailable wraps every store failure. The caller answers 503: the attempt was NOT allowed
// (see Allow), and it was not refused for being over the limit either.
var ErrUnavailable = errors.New("ratelimit: counter store unavailable")

// New builds a limiter. A nil counter or an empty policy is a wiring fault, refused here rather
// than turned into a limiter that allows everything.
func New(c Counter, p Policy) (*Limiter, error) {
	if c == nil {
		return nil, errors.New("ratelimit: nil counter")
	}
	if p.name == "" || p.limit <= 0 || p.window <= 0 {
		return nil, errors.New("ratelimit: empty policy — use a named policy such as ratelimit.OperatorSignIn")
	}
	return &Limiter{c: c, p: p}, nil
}

// Allow counts one attempt for key and says whether it may proceed.
//
//	allowed=true                     within the limit.
//	allowed=false, err=nil           over the limit; retryAfter is when the window ends (> 0).
//	allowed=false, err=ErrUnavailable  the store could not be asked. FAIL CLOSED: this is a
//	                                 security limiter (package doc) — the caller answers 503, never
//	                                 lets the attempt through.
//
// Every attempt counts, the refused ones included: a client hammering past the limit does not get
// a fresh budget by being refused, and the fixed window still ends on schedule.
func (l *Limiter) Allow(ctx context.Context, key Key) (allowed bool, retryAfter time.Duration, err error) {
	n, ttl, err := l.c.Incr(ctx, l.redisKey(key), l.p.window)
	if err != nil {
		return false, 0, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if n <= l.p.limit {
		return true, 0, nil
	}
	if ttl <= 0 || ttl > l.p.window {
		// A store answer outside the window it was given: never trust it into a Retry-After of 0
		// (an immediate retry loop) or of hours. The whole window is the honest upper bound.
		ttl = l.p.window
	}
	return false, ttl, nil
}

// redisKey is "rl:<policy>:<subject>". No tenant prefix: see Key for why that is only correct for
// a realm with no commune, and what a commune-scoped key must look like instead.
func (l *Limiter) redisKey(k Key) string { return "rl:" + l.p.name + ":" + k.subject }
