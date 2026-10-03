// Package ratelimit counts attempts per key in fixed windows, for routes that must refuse a flood
// before it reaches the logic behind them.
//
// THE FIRST POLICY is the operator sign-in limit (ADR 0048 §"Chốt của chủ dự án — 01/10/2026" #3):
// 20 attempts per 15 minutes per IP, across the password, TOTP and recovery-code steps. Its reason is
// specific and is the reason the limiter fails CLOSED by default: an operator account is locked for
// 12 hours after 5 wrong attempts (§"Chốt bước 1", security amendment), so anybody who can guess an
// operator's email can lock them out for 12 hours unless the route is also bounded per address. A
// limiter that opens when Redis is down removes that bound exactly when nothing else notices.
//
// THE SECOND POLICY is the public Mini App news read (PublicNewsRead, owner decision 02/10/2026):
// 120 requests per minute per client network per commune host, and it is the ONE policy that fails
// OPEN — see Policy.failOpen for why that is a per-policy field and never the package default.
//
// The others — CitizenPhotoUpload, StaffImageFetch (ADR 0067 K10) — fail closed, the default.
//
// WHY A FIXED WINDOW, NOT A SLIDING ONE: the owner chose it (§01/10 #3), and it is one atomic
// INCR per attempt with no per-attempt memory. Its known cost — up to 2× the limit across a window
// boundary — is inside what the decision accepts; a different algorithm is a different threshold,
// which is a rule 13 stop condition, not a tuning knob.
//
// REDIS, AND ONLY FOR COUNTING. ADR 0010 allows Redis for cache and rate limiting and forbids it as
// durable storage. A counter here is a short-window guard holding one integer; losing it loses
// nothing but the count.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// The operator sign-in threshold — a SECURITY THRESHOLD CHOSEN BY THE OWNER (ADR 0048 §"Chốt của
// chủ dự án — 01/10/2026" #3, rule 13). Constants, deliberately not environment variables: an env
// var would let a deployment loosen a decided security number without a review, which is rule 13
// forbidden #4. Changing either number is a rule 13 STOP CONDITION — ask the owner, then edit here.
const (
	OperatorSignInLimit  = 20
	OperatorSignInWindow = 15 * time.Minute
)

// The public Mini App news read threshold — chosen by the OWNER on 02/10/2026 (task F1, decision D2)
// for GET /api/v1/commune-news, …/categories and …/{id}. Constants for the operator pair's reason:
// changing either is a rule 13 stop condition.
const (
	PublicNewsReadLimit  = 120
	PublicNewsReadWindow = time.Minute
)

// The citizen scene-photo upload threshold — ADR 0052 §12 requires "giới hạn tần suất theo công dân"
// and names NO number. CHOSEN BY THE OWNER on 02/10/2026 (asked as a rule 13 stop condition; ADR 0047,
// row "Ảnh hiện trường khi gửi phản ánh"); changing either is a rule 13 stop condition again. The
// reasoning they were proposed with: one photo costs TWO
// counted requests (the upload slot and its completion), a petition holds at most 5 photos, so 30
// per 15 minutes lets a citizen attach a full set to three petitions — and retry each photo once — in
// one sitting, while bounding what one session can make the server decode and scan.
const (
	CitizenPhotoUploadLimit  = 30
	CitizenPhotoUploadWindow = 15 * time.Minute
)

// The staff "image from a pasted link" threshold — CHOSEN BY THE OWNER on 03/10/2026 (ADR 0067 table K,
// K10, which replaces K8's "no limit of its own" for this one route): 30 attempts per hour per OFFICER,
// counting EVERY attempt, failures and refusals included. The reason it is not "successes only": a
// stolen session could otherwise fire unbounded outbound requests from a government address, and a
// deliberately slow host could hold every download slot of the pod. Changing either number is a rule 13
// stop condition (forbidden #4: never loosened silently).
const (
	StaffImageFetchLimit  = 30
	StaffImageFetchWindow = time.Hour
)

// Policy is one limit: at most Limit attempts per key per Window.
type Policy struct {
	// name goes into every key; it separates the counters of two policies keyed by the same subject.
	name   string
	limit  int64
	window time.Duration
	// event is the security-log event a refusal emits (skills/security-logging: "rate limit hit",
	// TCVN 14423 5.12.2.4e). Owned by the policy, not the middleware, because the realm a refusal
	// belongs to is a property of the policy — a commune-scoped policy must not log as `operator.*`.
	event string
	// failOpen: when the counter store cannot be asked, the request PROCEEDS — and a security warning
	// is emitted once per window — instead of being refused with 503.
	//
	// FALSE FOR EVERY POLICY BUT ONE, which is why it is a field and not a mode of the limiter. A
	// security limiter that opens when Redis is down removes its bound exactly when nothing else
	// notices: right for the operator sign-in, whose bound is what stands between an email address
	// and a 12-hour lockout. The public news read is the OWNER'S EXPLICIT EXCEPTION (02/10/2026, D2):
	// what it bounds is load on a notice board the commune published to every resident, and a Redis
	// outage turning every commune's board into a 503 is the worse failure. No other policy may copy
	// it without the same decision.
	failOpen bool
	// refusedCode / refusedMessage are the 429's machine code and its ONE Vietnamese sentence. Empty =
	// the generic `rate_limited` answer. A field of the policy because a staff screen must be able to tell
	// "you pasted too many links this hour" from a generic throttle — and the sentence must name the act.
	refusedCode, refusedMessage string
}

// FailsOpen reports whether a store outage lets requests through under this policy.
func (p Policy) FailsOpen() bool { return p.failOpen }

// OperatorSignIn is the policy for the operator area's sign-in steps (password, TOTP, recovery
// code): ADR 0048 §01/10 #3.
var OperatorSignIn = Policy{name: "op-signin", limit: OperatorSignInLimit, window: OperatorSignInWindow,
	event: "operator.rate_limited"}

// PublicNewsRead is the policy of the three public Mini App news reads (owner, 02/10/2026). Its keys
// are built by PublicHostIPKey. FAILS OPEN — see Policy.failOpen.
var PublicNewsRead = Policy{name: "public-news", limit: PublicNewsReadLimit, window: PublicNewsReadWindow,
	event: "public_news.rate_limited", failOpen: true}

// CitizenPhotoUpload is the policy of the citizen scene-photo upload and completion routes
// (service-petitions, ADR 0052 §12). Keys from CitizenKey. FAILS CLOSED, the package default: what it
// bounds is server-side decoding, malware scanning and object writes driven by one weak identity
// (rule 4), and nobody has decided that a Redis outage may lift that bound. The petition itself is
// never behind this limit — a photo failure never fails the petition send (owner, 02/10/2026).
var CitizenPhotoUpload = Policy{name: "citizen-photo", limit: CitizenPhotoUploadLimit,
	window: CitizenPhotoUploadWindow, event: "citizen_photo.rate_limited"}

// StaffImageFetch is the policy of service-comms' POST /api/v1/content-items/body-images/from-url (ADR
// 0067 K10). Keys from ActorKey. FAILS CLOSED (503 when Redis cannot be asked): the route makes the
// SERVER call an arbitrary host on an authenticated officer's behalf, and the bound exists precisely for
// the case where that officer's session is not the officer — nobody decided that a Redis outage may lift
// it. PublicNewsRead's fail-open exception (D2) is about a public read and does not extend here.
var StaffImageFetch = Policy{name: "body-image-fetch", limit: StaffImageFetchLimit,
	window: StaffImageFetchWindow, event: "body_image_fetch.rate_limited",
	refusedCode:    "image_fetch_rate_limited",
	refusedMessage: "Đã lấy ảnh từ liên kết quá nhiều lần trong một giờ. Vui lòng thử lại sau."}

// Key is one counter's identity. OPAQUE, built only by the constructors below, so that "which scope
// does this counter belong to" is decided once, by name, and never by a caller concatenating a
// string.
//
// THE SCOPE RULE (rule 1, invariant 7): a counter for something that belongs to a commune is
// prefixed `t:<tenant_id>`, or two communes share one budget and one commune's flood locks out the
// other's residents. The tenant comes FROM ctx (tenant.From; rule 1, invariant 4 — never as an
// argument), and a constructor that needs one refuses when there is none (ErrNoCommune). The
// operator area belongs to NO commune (ADR 0048 condition #3), so its keys carry no tenant prefix —
// which is why its constructor names the realm.
type Key struct {
	// tenant is "" only for a realm that belongs to no commune (the operator area, a host no commune
	// holds). Never a default: a commune-scoped constructor refuses rather than leave it empty.
	tenant  string
	subject string
}

// ErrNoCommune — a commune-scoped key was asked for with no commune in ctx. A wiring fault on the
// isolation path; the caller must refuse, never fall back to an unscoped key (rule 1, forbidden #1).
var ErrNoCommune = errors.New("ratelimit: no commune in context for a commune-scoped key")

// PublicHostIPKey is the key of a PUBLIC route that names its commune by host (the Mini App news
// reads, `?host=`): one counter per (host, client network), prefixed `t:<tenant_id>` when the host
// resolved to a commune — the commune ctx carries (rule 1, invariant 7).
//
//	resolved=true   tenant from ctx; ErrNoCommune when ctx carries none.
//	                → "t:<tenant>:rl:<policy>:host:<host>:ip:<net>"
//	resolved=false  the host resolved to NO commune (unknown, reserved or inactive). The counter
//	                belongs to no commune, so it has no tenant prefix, and ctx is not consulted.
//	                → "rl:<policy>:host:<host>:ip:<net>"
//
// WHY THE HOST IS IN THE KEY EVEN WHEN THE COMMUNE IS: the public routes answer an unknown domain
// with the same bytes as a commune that published nothing, so they do not disclose which domains are
// communes. Keyed per commune for known hosts and per host for unknown ones, the limiter would undo
// that — 120 requests to one host and one more to a second would tell whether the two name the same
// commune. Per (host, network) in both cases, a 429 says nothing about which hosts are communes. The
// cost, accepted: a commune reachable under two domains has two budgets per client.
//
// host must ALREADY be validated by the caller (the public handlers' domain check); it is
// lower-cased here so two spellings of one name share a counter. ip is httpx.ClientIP — never a raw
// X-Forwarded-For — reduced exactly as OperatorIPKey reduces it (IPv4, mapped IPv4, IPv6 /64).
func PublicHostIPKey(ctx context.Context, resolved bool, host, ip string) (Key, error) {
	subject := "host:" + strings.ToLower(host) + ":" + ipSubject(ip)
	if !resolved {
		return Key{subject: subject}, nil
	}
	id, ok := tenant.From(ctx)
	if !ok || !id.Valid() {
		return Key{}, ErrNoCommune
	}
	return Key{tenant: string(id), subject: subject}, nil
}

// CitizenKey is the key of a per-CITIZEN limit: one counter per (commune, citizen).
//
//	→ "t:<tenant>:rl:<policy>:citizen:<sha256(citizenID) hex>"
//
// The commune comes from ctx (rule 1, invariant 4) — on the citizen chain that is the session's,
// set by httpx.XaTuPhien — and ErrNoCommune when there is none. citizenID is the SESSION's opaque
// citizen id (authz.Principal.ID of a citizen principal), never a request value (rule 4, invariant 2);
// an empty one is refused rather than turned into one counter every anonymous caller would share.
//
// HASHED, NOT WRITTEN AS IS: the id is opaque, but it names one person across every key this
// process writes, and a cache key is somewhere rule 3 forbidden #4 keeps person-linked values out
// of. A digest still separates citizens and cannot be read back (sha256 — rule 13 forbids md5/sha1).
func CitizenKey(ctx context.Context, citizenID string) (Key, error) {
	if strings.TrimSpace(citizenID) == "" {
		return Key{}, ErrNoCitizen
	}
	id, ok := tenant.From(ctx)
	if !ok || !id.Valid() {
		return Key{}, ErrNoCommune
	}
	sum := sha256.Sum256([]byte(citizenID))
	return Key{tenant: string(id), subject: "citizen:" + hex.EncodeToString(sum[:])}, nil
}

// ErrNoCitizen — a per-citizen key was asked for with no citizen id. A wiring fault (the route lost
// the session identity); the caller refuses, never counts under a shared key.
var ErrNoCitizen = errors.New("ratelimit: no citizen id for a per-citizen key")

// ActorKey is the key of a per-OFFICER limit: one counter per (commune, staff business code).
//
//	→ "t:<tenant>:rl:<policy>:actor:<code>"
//
// code is the BUSINESS CODE of the session's principal (authz.Principal.Ma, `CB-00123`) — never a
// request value. Written as is, unlike CitizenKey's digest: a business code is what the audit trail
// already names an officer by (rule 6, invariant 8) and is not personal data in itself, and an operator
// reading Redis during an incident needs to see WHOSE budget ran out. The commune comes from ctx (rule 1,
// invariant 4) — the officer's code is unique within a commune only, so without the prefix two communes'
// `CB-00001` would share a budget. No commune → ErrNoCommune; a blank code → ErrNoActor.
func ActorKey(ctx context.Context, code string) (Key, error) {
	if strings.TrimSpace(code) == "" {
		return Key{}, ErrNoActor
	}
	id, ok := tenant.From(ctx)
	if !ok || !id.Valid() {
		return Key{}, ErrNoCommune
	}
	return Key{tenant: string(id), subject: "actor:" + code}, nil
}

// ErrNoActor — a per-officer key was asked for with no business code. A wiring fault (the route lost
// the principal, or the principal has no code); the caller refuses, never counts under a shared key.
var ErrNoActor = errors.New("ratelimit: no business code for a per-officer key")

// OperatorIPKey is the key of the operator sign-in limit: one counter per client NETWORK.
//
//	IPv4                one counter per address ("ip:203.0.113.7").
//	IPv4-mapped IPv6    the same counter as the IPv4 address it maps (::ffff:203.0.113.7 is
//	                    203.0.113.7). Otherwise one client gets two budgets by switching notation.
//	IPv6                one counter per /64 ("ip:2001:db8:1:2::/64"), the owner's decision of
//	                    01/10/2026. A /64 is what ONE subscriber line is handed (RFC 6177 / RIPE-690
//	                    practice) and SLAAC lets every host on it pick any of 2^64 addresses — per
//	                    address, the limit would be 20 × 2^64 attempts for anyone with a home line.
//	                    The zone (`%eth0`) is dropped: it names a local interface, not a client.
//	"" or unparseable   ONE shared counter ("ip:"): every request the edge could not place is held to
//	                    one budget together — the fail-closed direction. Never a per-string counter,
//	                    which would let a client mint a fresh budget per spelling.
//
// ip is the address the edge observed (httpx.ClientIP — never a raw X-Forwarded-For).
func OperatorIPKey(ip string) Key { return Key{subject: ipSubject(ip)} }

// ipSubject is the client-network part every IP-keyed counter shares — the table on OperatorIPKey.
func ipSubject(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return "ip:"
	}
	a = a.WithZone("").Unmap()
	if a.Is4() {
		return "ip:" + a.String()
	}
	// PrefixFrom cannot fail for 64 on a 128-bit address; Masked zeroes the interface identifier.
	return "ip:" + netip.PrefixFrom(a, 64).Masked().String()
}

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
	// warnedWindow is the index (unix nanoseconds / window) of the last window in which a fail-open
	// policy reported a store outage: one security warning per window, never one per request.
	warnedWindow atomic.Int64
	now          func() time.Time
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
	if p.name == "" || p.event == "" || p.limit <= 0 || p.window <= 0 {
		return nil, errors.New("ratelimit: empty policy — use a named policy such as ratelimit.OperatorSignIn")
	}
	if (p.refusedCode == "") != (p.refusedMessage == "") {
		return nil, errors.New("ratelimit: a policy's refusal code and sentence come together or not at all")
	}
	return &Limiter{c: c, p: p, now: time.Now}, nil
}

// Policy returns the policy this limiter applies.
func (l *Limiter) Policy() Policy { return l.p }

// Allow counts one attempt for key and says whether it may proceed.
//
//	allowed=true                       within the limit.
//	allowed=false, err=nil             over the limit; retryAfter is when the window ends (> 0).
//	allowed=false, err=ErrUnavailable  the store could not be asked. CLOSED HERE FOR EVERY POLICY:
//	                                   a fail-open policy is honoured by Gate / Middleware, which
//	                                   read Policy.FailsOpen — never by this method answering true
//	                                   with an error, so a caller that checks only `err` stays closed.
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

// redisKey is "t:<tenant>:rl:<policy>:<subject>" for a commune-scoped key (rule 1, invariant 7;
// skills/security-baseline §7), and "rl:<policy>:<subject>" for a realm with no commune (see Key).
func (l *Limiter) redisKey(k Key) string {
	base := "rl:" + l.p.name + ":" + k.subject
	if k.tenant != "" {
		return "t:" + k.tenant + ":" + base
	}
	return base
}

// firstOutageInWindow is true once per policy window — the throttle of the fail-open warning.
func (l *Limiter) firstOutageInWindow() bool {
	w := l.now().UnixNano() / int64(l.p.window)
	for {
		prev := l.warnedWindow.Load()
		if prev == w {
			return false
		}
		if l.warnedWindow.CompareAndSwap(prev, w) {
			return true
		}
	}
}
