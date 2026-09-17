// Package idem protects a state-changing route against a duplicate request.
//
// WHY THIS IS NOT OPTIONAL: a double-submitted POST creates a second petition with a second
// lookup code ALREADY SHOWN to the citizen, or a second disbursement against real money.
// Rule 7 forbids hard delete, so the duplicate is permanent — it can only be soft-deleted,
// and the code it consumed is never reissued. The cheapest moment to prevent it is the only
// moment: before the handler runs.
//
// THREE DECLARATIONS, AND THERE IS NO FOURTH:
//
//	idem.Required(idem.MoKhiHong)   // Redis down -> let it through + log.Warn
//	idem.Required(idem.DongKhiHong) // Redis down -> 503
//	idem.KhongCan("<lý do>")        // naturally idempotent; reason MANDATORY
//
// There is deliberately no bare `Required()`: the failure mode is a business decision — a
// citizen refused at intake because a cache is down is worse than a rare duplicate, while a
// second disbursement is not. Whoever writes the route CHOOSES; nothing defaults.
//
// `authz` wraps OUTSIDE `idem`, for two reasons: an unauthorised request must not consume an
// idempotency key, and the key is scoped to the principal, which therefore has to be in the
// context before this middleware runs.
//
// WHY `Required` ANSWERS 500 WHEN THERE IS NO PRINCIPAL — read this before "fixing" it:
//
// The key is client-supplied and is scoped to (commune, actor, method, path). With no principal
// the actor component collapses to one shared value, so EVERY anonymous sender of one commune
// shares one key space. A route declared `authz.Public(...)` + `idem.Required(...)` — which is
// a perfectly reasonable-looking pair for citizen intake from the Mini App — then does this:
//
//	citizen A sends Idempotency-Key aaaaaaaa  -> petition created, lookup code PA-2026-0001
//	citizen B sends the SAME key              -> same key, replayed: B is handed A's code
//
// B can look up A's petition (rule 4, invariant 1), and B's own petition was never created
// while B believes it was (rule 10, invariant 1). Two failures at once, both silent. The lower
// bound on key length does not help: a client deriving its key from the form contents or from a
// device id collides deterministically, not by chance.
//
// So this refuses, loudly, at request time, the same way the package refuses a bare `Required()`:
// the failure mode is a decision nobody may forget into a default, and the identity the key is
// scoped to is the same kind of decision. The 500 says the ROUTE is wired wrong, because it is —
// the sender did nothing wrong and is told so.
//
// THERE IS DELIBERATELY NO `RequiredAnDanh` ESCAPE HATCH. No route needs one today, and an exit
// nobody has needed yet is an exit nobody has thought through. When a genuinely anonymous route
// needs duplicate protection, the identity it is scoped to gets designed then — with the person
// who owns the business rule.
//
// → .claude/skills/rest-api-design/SKILL.md §4 owns this convention.
package idem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// Header is the request header carrying the client-generated key.
const Header = "Idempotency-Key"

const (
	// TTLDangChay bounds how long a claim survives while the handler runs.
	//
	// WHY SHORT: a handler that dies mid-request (panic, pod killed, network cut) leaves the
	// claim behind. If it lived as long as a finished result, every retry would be refused for
	// 24 hours although the petition was never created — the citizen is locked out of a channel
	// that holds no record of them.
	TTLDangChay = 60 * time.Second

	// TTLDaXong bounds how long a finished result can be replayed. Long enough to cover a
	// client retrying after an outage, short enough that Redis never becomes a record store.
	TTLDaXong = 24 * time.Hour
)

const (
	dauDangChay = "1" // in flight
	dauDaXong   = "2" // finished: "2:<status>:<code>"
)

// Bounds on the client-supplied key. The key is hashed anyway, but an unbounded string still
// travels through this process and into the Redis command, so it is bounded at the edge.
const (
	KeyToiDa = 200 // upper bound: nobody needs more, and an endless string is an attack surface
	KeyToiIt = 8   // lower bound: "1" would collide between two clerks of the SAME commune
)

// Store is the small surface idem needs. Redis is ONE implementation.
//
// WHY AN INTERFACE: the tests of every route that declares duplicate protection must run
// without a Redis, the same way tenant.Directory lets the edge be tested without the platform
// service. A test that needs infrastructure is a test that stops being run.
//
// Every method takes the already-built key. Building it — including the commune prefix — is
// this package's job and is deliberately not delegated to an implementation.
type Store interface {
	// Claim writes the in-flight marker only if the key is absent (SET NX EX).
	// ok=false means somebody already holds it; that is not an error.
	Claim(ctx context.Context, key string, ttl time.Duration) (ok bool, err error)

	// Get reads the current value, or "" when the key is absent.
	Get(ctx context.Context, key string) (string, error)

	// Complete overwrites the claim with the finished marker and the long TTL.
	Complete(ctx context.Context, key, value string, ttl time.Duration) error

	// Release drops the key so the caller may correct the request and send it again.
	Release(ctx context.Context, key string) error
}

// CheDoHong is what happens when the Store is unreachable.
type CheDoHong int

const (
	// MoKhiHong lets the request through and logs a warning. For intake paths: refusing a
	// citizen because a cache is down is worse than a rare duplicate.
	MoKhiHong CheDoHong = iota + 1

	// DongKhiHong answers 503. For anything with legal consequence — money, document numbers,
	// issuance, closure, privilege changes — where a duplicate cannot be undone.
	DongKhiHong
)

func (m CheDoHong) String() string {
	switch m {
	case MoKhiHong:
		return "MoKhiHong"
	case DongKhiHong:
		return "DongKhiHong"
	default:
		return "khong-hop-le"
	}
}

// --- what the middleware needs, carried on the context ----------------------------------

type ctxKeyRuntime struct{}

type runtime struct {
	store Store
	log   *slog.Logger
}

type ctxKeyKetQua struct{}

// ketQua is the handler's report back to the middleware: the business code a retry must be
// told about. It is a pointer in the context because a handler cannot hand a value upwards.
type ketQua struct{ ma string }

// Middleware installs the Store for every request. Mount it at the edge, once, next to
// httpx.TenantMiddleware — never per route.
//
// A nil store is a valid deployment: local development with no Redis configured. The route
// then behaves exactly as if Redis were unreachable, which is what the declared CheDoHong is
// for. A service must not fail to start because a cache is absent.
func Middleware(s Store, log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), s, log)))
		})
	}
}

// Into puts the Store on a context. Exported for wiring and for tests.
func Into(ctx context.Context, s Store, log *slog.Logger) context.Context {
	if log == nil {
		log = slog.Default()
	}
	return context.WithValue(ctx, ctxKeyRuntime{}, runtime{store: s, log: log})
}

func runtimeFrom(ctx context.Context) runtime {
	rt, ok := ctx.Value(ctxKeyRuntime{}).(runtime)
	if !ok {
		return runtime{log: slog.Default()}
	}
	if rt.log == nil {
		rt.log = slog.Default()
	}
	return rt
}

// RecordCode is how a handler tells a future retry what the first attempt produced — the
// lookup code, the issued document number.
//
// WHY THE CODE AND NOT THE BODY: the response body holds names, phone numbers and the text of
// the petition. Putting it in Redis builds a personal-data store outside PostgreSQL, with no
// audit trail and no soft delete (rule 3). But a bare flag is not enough either: the retry
// would get a 409 and the citizen would NEVER learn their lookup code while their petition
// sits in the system (rule 10, invariant 1). The business code is the one value that is
// neither personal data nor droppable.
//
// A no-op on a route with no claim in flight, so a handler may call it unconditionally.
func RecordCode(ctx context.Context, ma string) {
	kq, ok := ctx.Value(ctxKeyKetQua{}).(*ketQua)
	if !ok {
		return
	}
	kq.ma = sachMa(ma)
}

// sachMa keeps the stored value to characters that survive a round trip and cannot smuggle a
// newline or a colon storm into the value format.
func sachMa(ma string) string {
	if len(ma) > 64 {
		ma = ma[:64]
	}
	var b strings.Builder
	for _, r := range ma {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- declarations -----------------------------------------------------------------------

// KhongCan declares that a route needs no duplicate protection because repeating it produces
// the same outcome. The reason is mandatory and is kept in the binary so it can be audited —
// the same discipline as authz.Public.
//
// Panics at route construction, not at request time: a missing reason must stop a deployment,
// not surprise a citizen.
func KhongCan(lyDo string) func(http.Handler) http.Handler {
	if lyDo == "" {
		panic("idem: KhongCan requires a specific reason")
	}
	return func(next http.Handler) http.Handler { return next }
}

// Required guards a route against a duplicate request.
//
// There is no parameterless form on purpose: `Required()` does not compile, so the failure
// mode can never be forgotten into a default.
func Required(cheDo CheDoHong) func(http.Handler) http.Handler {
	switch cheDo {
	case MoKhiHong, DongKhiHong:
	default:
		panic(fmt.Sprintf("idem: Required needs MoKhiHong or DongKhiHong, got %d", cheDo))
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			phucVu(w, r, next, cheDo)
		})
	}
}

func phucVu(w http.ResponseWriter, r *http.Request, next http.Handler, cheDo CheDoHong) {
	ctx := r.Context()
	rt := runtimeFrom(ctx)

	// Commune AND principal both come from the CONTEXT, never from a parameter or a client
	// header (rule 1, invariant 4).
	//
	// BOTH ARE CHECKED BEFORE THE HEADER, and the order is deliberate: what the route got wrong
	// outranks what the client got wrong. Answering "your header is missing" to a request that
	// this route could not have protected anyway sends the integrator to fix something that is
	// not broken, and hides the defect that is.
	xa := tenant.MustFrom(ctx)

	// NO PRINCIPAL, NO DUPLICATE PROTECTION — see the package comment. Every anonymous sender of
	// this commune would otherwise share one key space, and the second one would be handed the
	// first one's lookup code. Refusing is the only answer that does not quietly hand one
	// citizen another citizen's record.
	chuThe := ChuThe(ctx)
	if chuThe == ChuTheAnDanh {
		rt.log.Error("idem: route khai Required nhưng yêu cầu không có chủ thể — SAI CẤU HÌNH ROUTE",
			"method", r.Method, "path", r.URL.Path, "xa", xa.String(), "che_do", cheDo.String())
		loi(w, http.StatusInternalServerError, "idempotency_misconfigured",
			"Hệ thống chưa xác định được người gửi nên không bảo đảm được việc chống trùng thao tác. "+
				"Đây là lỗi cấu hình của hệ thống, không phải do bạn: yêu cầu CHƯA được xử lý. "+
				"Vui lòng báo cơ quan quản trị hệ thống.")
		return
	}

	khoa := r.Header.Get(Header)
	if khoa == "" {
		loi(w, http.StatusBadRequest, "missing_idempotency_key",
			"Thiếu header "+Header+". Hãy gửi lại kèm một mã ngẫu nhiên duy nhất cho mỗi lần "+
				"thao tác (UUID hoặc ULID), và giữ nguyên mã đó khi thử lại.")
		return
	}
	if err := kiemTraKhoa(khoa); err != nil {
		// The key itself is never echoed back: it is client-supplied and lands in access logs.
		loi(w, http.StatusBadRequest, "invalid_idempotency_key",
			"Header "+Header+" không hợp lệ: "+err.Error())
		return
	}

	key := Key(xa, chuThe, r.Method, r.URL.Path, khoa)

	if rt.store == nil {
		// No Redis configured. Same handling as an unreachable one: the route already declared
		// what to do, and local development must still run.
		hong(w, r, next, cheDo, rt, xa, errors.New("idem: chưa cấu hình Store"))
		return
	}

	chiemDuoc, err := rt.store.Claim(ctx, key, TTLDangChay)
	if err != nil {
		hong(w, r, next, cheDo, rt, xa, err)
		return
	}

	if !chiemDuoc {
		giaTri, err := rt.store.Get(ctx, key)
		if err != nil {
			hong(w, r, next, cheDo, rt, xa, err)
			return
		}
		switch {
		case giaTri == "":
			// The key vanished between Claim and Get: it expired, or the first attempt failed
			// and released it. One more claim is the honest answer — anything else refuses a
			// request that nothing is actually protecting against.
			chiemDuoc, err = rt.store.Claim(ctx, key, TTLDangChay)
			if err != nil {
				hong(w, r, next, cheDo, rt, xa, err)
				return
			}
			if !chiemDuoc {
				dangChay(w)
				return
			}
		case giaTri == dauDangChay:
			dangChay(w)
			return
		case strings.HasPrefix(giaTri, dauDaXong+":"):
			phatLai(w, giaTri, rt, xa)
			return
		default:
			// An unrecognised value is not a reason to run the handler twice.
			rt.log.Warn("idem: giá trị khoá không đọc được",
				"method", r.Method, "path", r.URL.Path, "xa", xa.String())
			dangChay(w)
			return
		}
	}

	kq := &ketQua{}
	rw := &ghiLai{ResponseWriter: w}

	// The deferred release also runs while a panic unwinds, so a handler that dies does not
	// leave its claim behind for the rest of the TTL. The panic keeps propagating to
	// httpx.Recover — nothing is swallowed here.
	defer func() {
		// Detached from the request context: it may already be cancelled once the client has
		// the response, and the key still has to be settled.
		bg, huy := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer huy()

		if rw.status >= 200 && rw.status < 300 {
			val := dauDaXong + ":" + strconv.Itoa(rw.status) + ":" + kq.ma
			if err := rt.store.Complete(bg, key, val, TTLDaXong); err != nil {
				// The response is already on the wire; the request itself succeeded. All that
				// is lost is the ability to replay it, so this is a warning, not a failure.
				rt.log.Warn("idem: không ghi được kết quả", "method", r.Method,
					"path", r.URL.Path, "xa", xa.String(), "err", err)
			}
			return
		}
		// 4xx/5xx or a panic (status 0): the write did not happen, so the client must be able
		// to correct the request and send it again with the same key.
		if err := rt.store.Release(bg, key); err != nil {
			rt.log.Warn("idem: không nhả được khoá", "method", r.Method,
				"path", r.URL.Path, "xa", xa.String(), "err", err)
		}
	}()

	next.ServeHTTP(rw, r.WithContext(context.WithValue(ctx, ctxKeyKetQua{}, kq)))
}

// hong applies the failure mode the route declared.
//
// THE COMMUNE IS ON EVERY LINE, and the MoKhiHong one is the reason: it says duplicate
// protection was skipped. One process serves 200+ communes into one log stream, so a line
// without the commune tells an operator that something was let through unprotected somewhere —
// which is not something they can act on. It is not personal data (rule 3): it is an opaque id.
func hong(w http.ResponseWriter, r *http.Request, next http.Handler, cheDo CheDoHong,
	rt runtime, xa tenant.ID, err error) {
	switch cheDo {
	case MoKhiHong:
		// No key, no personal data: method and path only. The path may not carry personal data
		// either (rule 3, forbidden #4).
		rt.log.Warn("idem: bỏ qua chống trùng vì Store không dùng được",
			"method", r.Method, "path", r.URL.Path, "xa", xa.String(),
			"che_do", cheDo.String(), "err", err)
		next.ServeHTTP(w, r)
	default:
		rt.log.Error("idem: từ chối vì Store không dùng được",
			"method", r.Method, "path", r.URL.Path, "xa", xa.String(),
			"che_do", cheDo.String(), "err", err)
		w.Header().Set("Retry-After", "1")
		loi(w, http.StatusServiceUnavailable, "idempotency_unavailable",
			"Hệ thống tạm thời không bảo đảm được việc chống trùng thao tác. Vui lòng thử lại.")
	}
}

func dangChay(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	loi(w, http.StatusConflict, "request_in_progress",
		"Yêu cầu trước đó với cùng mã này đang được xử lý. Vui lòng thử lại sau giây lát.")
}

// PhatLai is the body of a replayed response. It carries the business code and nothing else —
// the first response's body is never stored, so it cannot be reproduced, and must not be.
type PhatLai struct {
	Code     string `json:"code,omitempty"` // business code: lookup code, document number
	Replayed bool   `json:"replayed"`
}

// HeaderPhatLai marks a replayed response, so a client (and a log) can tell it apart from the
// first one without parsing the body.
const HeaderPhatLai = "Idempotent-Replay"

func phatLai(w http.ResponseWriter, giaTri string, rt runtime, xa tenant.ID) {
	status, ma, ok := TachGiaTri(giaTri)
	if !ok {
		rt.log.Warn("idem: giá trị đã xong không đọc được", "xa", xa.String())
		dangChay(w)
		return
	}
	w.Header().Set(HeaderPhatLai, "true")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(PhatLai{Code: ma, Replayed: true})
}

// loi writes a failure through httpx — the ONE response shape for every failure in the system.
// Copying the shape here would be a second copy to drift from (rule 9, forbidden #2).
//
//	code    machine-readable identifier, ENGLISH snake_case, like a JSON field name
//	message read by a person, Vietnamese with diacritics, never personal data
//
// The trace id is empty on purpose: only the edge knows it, and a middleware must not invent
// one that matches nothing in the logs.
func loi(w http.ResponseWriter, status int, code, msg string) {
	httpx.WriteError(w, status, code, msg, "")
}

// --- key -------------------------------------------------------------------------------

// Key builds the Redis key for one attempt.
//
//	t:<tenant_id>:idem:<sha256(actor + method + path + Idempotency-Key)>
//
// THE COMMUNE PREFIX is rule 1, invariant 7, and is the whole reason this function exists
// instead of a string concatenation at the call site. Without it, two communes whose clients
// generate the same key collide and commune B is served commune A's result — a breach between
// two public authorities, through a cache key.
//
// THE ACTOR closes the same leak INSIDE one commune. The key is supplied by the client, so two
// clerks of the same commune posting to the same route can present the same one. The second
// would then be replayed the first one's lookup code: a business code for something they did
// not do, while believing their own petition exists. It does not. A lower bound on the key
// length makes that unlikely, not impossible — and "unlikely" is not the standard when the
// consequence is one clerk reading another's result.
//
// The actor carries Kind as well as ID (`staff:01J…`): staff ids and citizen ids come from two
// different tables and nothing guarantees the strings never coincide.
//
// METHOD AND PATH are hashed so one client key reused across two routes does not replay the
// wrong result.
func Key(tid tenant.ID, actor, method, path, khoa string) string {
	// Separated by \n: without a separator, ("POST", "/ab") and ("POS", "T/ab") hash alike.
	sum := sha256.Sum256([]byte(actor + "\n" + method + "\n" + path + "\n" + khoa))
	return "t:" + tid.String() + ":idem:" + hex.EncodeToString(sum[:])
}

// ChuTheAnDanh is what ChuThe reports when the request carries no principal.
//
// IT IS NOT A USABLE ACTOR. Required refuses to serve a request that resolves to it (see the
// package comment): one shared actor means one shared key space for every anonymous sender of a
// commune. The value exists so the refusal has something to compare against, and so a caller
// reading ChuThe cannot mistake it for an identity.
const ChuTheAnDanh = "anon"

// ChuThe returns the actor component of the key: "<Kind>:<ID>", or ChuTheAnDanh when the request
// carries no principal.
//
// `authz` wraps OUTSIDE `idem` (rest-api-design §4), so by the time Required runs the principal
// is already in the context. A route declared Public — sign-in — resolves to ChuTheAnDanh, which
// is fine only because such a route declares KhongCan: it has no key to share. A Public route
// that declares Required is refused at request time rather than trusted to that convention.
func ChuThe(ctx context.Context) string {
	p, ok := authz.From(ctx)
	if !ok || p.ID == "" {
		return ChuTheAnDanh
	}
	return p.Kind + ":" + p.ID
}

var (
	ErrKhoaQuaDai  = fmt.Errorf("độ dài tối đa %d ký tự", KeyToiDa)
	ErrKhoaQuaNgan = fmt.Errorf("độ dài tối thiểu %d ký tự", KeyToiIt)
	ErrKhoaKyTuLa  = errors.New("chỉ chấp nhận chữ cái, chữ số, dấu gạch ngang và gạch dưới")
)

// kiemTraKhoa bounds a value the CLIENT controls.
//
// The upper bound keeps an endless string out of the process and out of the Redis command
// even though it is hashed. The lower bound matters more than it looks: the key is scoped to
// the commune, not to the account, so a client sending "1" would replay the result of a
// DIFFERENT clerk in the SAME commune who also sent "1".
func kiemTraKhoa(khoa string) error {
	if len(khoa) > KeyToiDa {
		return ErrKhoaQuaDai
	}
	if len(khoa) < KeyToiIt {
		return ErrKhoaQuaNgan
	}
	for i := 0; i < len(khoa); i++ {
		ch := khoa[i]
		ok := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_'
		if !ok {
			return ErrKhoaKyTuLa
		}
	}
	return nil
}

// TachGiaTri parses a finished value "2:<status>:<code>". Exported so a Store implementation
// or an operator tool can read a key without re-deriving the format.
func TachGiaTri(giaTri string) (status int, ma string, ok bool) {
	phan := strings.SplitN(giaTri, ":", 3)
	if len(phan) != 3 || phan[0] != dauDaXong {
		return 0, "", false
	}
	n, err := strconv.Atoi(phan[1])
	if err != nil || n < 100 || n > 599 {
		return 0, "", false
	}
	return n, phan[2], true
}

// --- response recorder -------------------------------------------------------------------

// ghiLai records the status the handler produced. Only the status: the body streams straight
// through and is never held, never stored.
type ghiLai struct {
	http.ResponseWriter
	status int
}

func (g *ghiLai) WriteHeader(status int) {
	if g.status == 0 {
		g.status = status
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *ghiLai) Write(b []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK // net/http's implicit 200 on first write
	}
	return g.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the real writer (Flush, SetWriteDeadline).
func (g *ghiLai) Unwrap() http.ResponseWriter { return g.ResponseWriter }
