package ratelimit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// fakeCounter is an in-memory fixed-window counter with a controllable clock — the module has no
// in-memory Redis, and adding one for a single test is a dependency nobody reviewed.
type fakeCounter struct {
	mu    sync.Mutex
	now   time.Time
	n     map[string]int64
	until map[string]time.Time
	keys  []string
	fail  error
}

func newFake() *fakeCounter {
	return &fakeCounter{now: time.Unix(1_800_000_000, 0), n: map[string]int64{}, until: map[string]time.Time{}}
}

func (f *fakeCounter) Incr(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, key)
	if f.fail != nil {
		return 0, 0, f.fail
	}
	if u, ok := f.until[key]; !ok || !f.now.Before(u) {
		f.n[key] = 0
		f.until[key] = f.now.Add(window)
	}
	f.n[key]++
	return f.n[key], f.until[key].Sub(f.now), nil
}

func mustNew(t *testing.T, c Counter) *Limiter {
	t.Helper()
	l, err := New(c, OperatorSignIn)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// The decided threshold, pinned: changing it is a rule 13 stop condition, so a change must turn
// something red rather than slip through as a refactor.
func TestOperatorSignInIsTwentyPerFifteenMinutes(t *testing.T) {
	if OperatorSignInLimit != 20 || OperatorSignInWindow != 15*time.Minute {
		t.Fatalf("ADR 0048 §01/10 #3 decided 20 / 15 min / IP, got %d / %v", OperatorSignInLimit, OperatorSignInWindow)
	}
	if OperatorSignIn.limit != OperatorSignInLimit || OperatorSignIn.window != OperatorSignInWindow {
		t.Fatal("OperatorSignIn does not use the named constants")
	}
}

func TestTwentyAllowedThenRefusedUntilTheWindowEnds(t *testing.T) {
	f := newFake()
	l := mustNew(t, f)
	ctx := context.Background()
	k := OperatorIPKey("203.0.113.7")

	for i := 1; i <= 20; i++ {
		ok, _, err := l.Allow(ctx, k)
		if err != nil || !ok {
			t.Fatalf("attempt %d refused (err %v)", i, err)
		}
	}
	f.now = f.now.Add(5 * time.Minute)
	ok, retry, err := l.Allow(ctx, k)
	if err != nil || ok {
		t.Fatalf("attempt 21 allowed (err %v)", err)
	}
	if retry != 10*time.Minute {
		t.Fatalf("retryAfter = %v, want the rest of the window (10m)", retry)
	}

	// Another address has its own budget.
	if ok, _, _ := l.Allow(ctx, OperatorIPKey("203.0.113.8")); !ok {
		t.Fatal("a second address was refused on the first's budget")
	}

	// The window is fixed: it ends on schedule even though refused attempts kept counting.
	f.now = f.now.Add(10 * time.Minute)
	if ok, _, _ := l.Allow(ctx, k); !ok {
		t.Fatal("a new window did not start a new budget")
	}
}

// FAIL CLOSED: a store that cannot be asked never lets the attempt through.
func TestStoreFailureIsNotAllowed(t *testing.T) {
	f := newFake()
	f.fail = errors.New("dial tcp: connection refused")
	l := mustNew(t, f)
	ok, _, err := l.Allow(context.Background(), OperatorIPKey("203.0.113.7"))
	if ok {
		t.Fatal("an unreachable store ALLOWED the attempt — the limiter failed open")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

// The operator key has no tenant prefix (the realm has no commune) and is namespaced by policy.
func TestOperatorKeyShape(t *testing.T) {
	f := newFake()
	l := mustNew(t, f)
	_, _, _ = l.Allow(context.Background(), OperatorIPKey("203.0.113.7"))
	if len(f.keys) != 1 || f.keys[0] != "rl:op-signin:ip:203.0.113.7" {
		t.Fatalf("key = %v", f.keys)
	}
	if strings.HasPrefix(f.keys[0], "t:") {
		t.Fatal("an operator key carries a tenant prefix — the operator area has no commune")
	}
}

// A store answering a TTL outside the window never becomes Retry-After 0 or hours.
type badTTL struct{ ttl time.Duration }

func (b badTTL) Incr(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 999, b.ttl, nil
}

func TestOutOfRangeTTLIsClampedToTheWindow(t *testing.T) {
	for _, ttl := range []time.Duration{-1, 0, 48 * time.Hour} {
		l := mustNew(t, badTTL{ttl})
		ok, retry, err := l.Allow(context.Background(), OperatorIPKey(""))
		if ok || err != nil || retry != OperatorSignInWindow {
			t.Errorf("ttl %v: ok=%v retry=%v err=%v", ttl, ok, retry, err)
		}
	}
}

func TestNewRefusesWiringFaults(t *testing.T) {
	if _, err := New(nil, OperatorSignIn); err == nil {
		t.Error("nil counter accepted")
	}
	if _, err := New(newFake(), Policy{}); err == nil {
		t.Error("empty policy accepted — it would allow everything")
	}
}

func TestParseIncrResult(t *testing.T) {
	n, ttl, err := parseIncrResult([]any{int64(3), int64(1500)})
	if err != nil || n != 3 || ttl != 1500*time.Millisecond {
		t.Fatalf("got %d %v %v", n, ttl, err)
	}
	for _, bad := range []any{nil, "x", []any{int64(1)}, []any{"1", int64(1)}, []any{int64(0), int64(1)}} {
		if _, _, err := parseIncrResult(bad); err == nil {
			t.Errorf("%#v accepted", bad)
		}
	}
}

func TestNewRedisCounterRefusesEmptyDSN(t *testing.T) {
	if _, err := NewRedisCounter(""); err == nil {
		t.Fatal("empty REDIS_DSN accepted — the security limiter would never be built")
	}
}

// ---- middleware -----------------------------------------------------------------------------

func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/operator-sessions", nil)
	req.RemoteAddr = "203.0.113.7:51000"
	h.ServeHTTP(rec, req)
	return rec
}

func ipKey(r *http.Request) Key { return OperatorIPKey(strings.Split(r.RemoteAddr, ":")[0]) }

func TestMiddleware429WithRetryAfter(t *testing.T) {
	f := newFake()
	l := mustNew(t, f)
	ran := 0
	h := Middleware(l, ipKey, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran++ }))

	for i := 0; i < OperatorSignInLimit; i++ {
		if rec := serve(h); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	f.now = f.now.Add(14*time.Minute + 500*time.Millisecond)
	rec := serve(h)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60 (59.5 s rounded up)", got)
	}
	if ran != OperatorSignInLimit {
		t.Fatalf("handler ran %d times, want %d — a throttled attempt reached the handler", ran, OperatorSignInLimit)
	}
}

func TestMiddleware503WhenStoreDown(t *testing.T) {
	f := newFake()
	f.fail = errors.New("i/o timeout")
	ran := false
	h := Middleware(mustNew(t, f), ipKey, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }))
	rec := serve(h)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("store down: %d, want 503", rec.Code)
	}
	if ran {
		t.Fatal("store down, yet the handler ran — failed open")
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Error("a 503 for an outage must not claim a window")
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0: "1", time.Millisecond: "1", time.Second: "1", 1001 * time.Millisecond: "2", 15 * time.Minute: "900",
	} {
		if got := retryAfterSeconds(d); got != want {
			t.Errorf("%v → %q, want %q", d, got, want)
		}
	}
}

// IPv4 per address; IPv6 per /64 (owner's decision, 01/10/2026); IPv4-mapped IPv6 is the IPv4
// address; anything unparseable shares ONE counter — never a fresh budget per spelling.
func TestOperatorIPKeyNormalises(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.7":                   "ip:203.0.113.7",
		"::ffff:203.0.113.7":            "ip:203.0.113.7",
		"2001:db8:1:2::1":               "ip:2001:db8:1:2::/64",
		"2001:db8:1:2:ffff:ffff:ffff:1": "ip:2001:db8:1:2::/64",
		"2001:DB8:1:2::abcd":            "ip:2001:db8:1:2::/64",
		"fe80::1%eth0":                  "ip:fe80::/64",
		"2001:db8:1:3::1":               "ip:2001:db8:1:3::/64",
		"":                              "ip:",
		"not-an-ip":                     "ip:",
		"203.0.113.7:51000":             "ip:",
	} {
		if got := OperatorIPKey(in).subject; got != want {
			t.Errorf("OperatorIPKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// One /64 is one budget: rotating the interface identifier does not buy a 21st attempt.
func TestIPv6SlashSixtyFourSharesOneBudget(t *testing.T) {
	l := mustNew(t, newFake())
	ctx := context.Background()
	for i := 0; i < OperatorSignInLimit; i++ {
		if ok, _, _ := l.Allow(ctx, OperatorIPKey(fmt.Sprintf("2001:db8:1:2::%x", i+1))); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if ok, _, _ := l.Allow(ctx, OperatorIPKey("2001:db8:1:2::ffff")); ok {
		t.Fatal("a new address in the same /64 got a fresh budget")
	}
	if ok, _, _ := l.Allow(ctx, OperatorIPKey("2001:db8:1:3::1")); !ok {
		t.Fatal("the neighbouring /64 was refused on the first one's budget")
	}
}

// A 429 is a security event: Warn, the policy's event name, the policy and the client address —
// and nothing else (not the counter key, not the body).
func TestMiddleware429LogsSecurityEvent(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	f := newFake()
	h := Middleware(mustNew(t, f), ipKey, log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 0; i < OperatorSignInLimit; i++ {
		serve(h)
	}
	if buf.Len() != 0 {
		t.Fatalf("allowed attempts logged: %s", buf.String())
	}
	if rec := serve(h); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: %d", rec.Code)
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("one JSON line expected: %v (%s)", err, buf.String())
	}
	if line["level"] != "WARN" || line["event"] != "operator.rate_limited" ||
		line["chinh_sach"] != "op-signin" || line["ip"] != "203.0.113.7" || line["outcome"] != "refused" {
		t.Fatalf("event shape: %v", line)
	}
	if strings.Contains(buf.String(), "rl:") {
		t.Fatalf("the counter key was logged: %s", buf.String())
	}
}

// ---- the public news read (owner, 02/10/2026) ------------------------------------------------

const testCommune = "01JTESTCOMMUNE000000000000"

// The decided threshold, pinned like the operator one.
func TestPublicNewsReadIs120PerMinuteAndFailsOpen(t *testing.T) {
	if PublicNewsReadLimit != 120 || PublicNewsReadWindow != time.Minute {
		t.Fatalf("owner decided 120 / minute, got %d / %v", PublicNewsReadLimit, PublicNewsReadWindow)
	}
	if PublicNewsRead.limit != PublicNewsReadLimit || PublicNewsRead.window != PublicNewsReadWindow {
		t.Fatal("PublicNewsRead does not use the named constants")
	}
	if !PublicNewsRead.FailsOpen() {
		t.Fatal("the owner's exception: the public news read fails OPEN")
	}
	if OperatorSignIn.FailsOpen() {
		t.Fatal("the operator sign-in limit must stay fail-CLOSED")
	}
}

// Rule 1 invariant 7: a resolved commune's counter is prefixed t:<tenant_id>, the tenant taken from
// ctx; an unresolved host's counter belongs to no commune; no commune in ctx is a refusal, never an
// unscoped key.
func TestPublicHostIPKeyShape(t *testing.T) {
	f := newFake()
	l, err := New(f, PublicNewsRead)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenant.Into(context.Background(), tenant.ID(testCommune))

	k, err := PublicHostIPKey(ctx, true, "Xa-A.Vigov.VN", "2001:db8:1:2::9")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = l.Allow(ctx, k)
	k2, _ := PublicHostIPKey(ctx, false, "unknown.vigov.vn", "203.0.113.7")
	_, _, _ = l.Allow(ctx, k2)
	want := []string{
		"t:" + testCommune + ":rl:public-news:host:xa-a.vigov.vn:ip:2001:db8:1:2::/64",
		"rl:public-news:host:unknown.vigov.vn:ip:203.0.113.7",
	}
	if len(f.keys) != 2 || f.keys[0] != want[0] || f.keys[1] != want[1] {
		t.Fatalf("keys = %v, want %v", f.keys, want)
	}

	if _, err := PublicHostIPKey(context.Background(), true, "xa-a.vigov.vn", "203.0.113.7"); !errors.Is(err, ErrNoCommune) {
		t.Fatalf("no commune in ctx: err = %v, want ErrNoCommune", err)
	}
}

// Two communes never share a budget: one commune's flood does not throttle the other's residents.
func TestPublicNewsBudgetIsPerCommune(t *testing.T) {
	l, _ := New(newFake(), PublicNewsRead)
	a := tenant.Into(context.Background(), tenant.ID(testCommune))
	b := tenant.Into(context.Background(), tenant.ID("01JTESTCOMMUNEB00000000000"))
	ka, _ := PublicHostIPKey(a, true, "xa.vigov.vn", "203.0.113.7")
	kb, _ := PublicHostIPKey(b, true, "xa.vigov.vn", "203.0.113.7")
	for i := 0; i < PublicNewsReadLimit; i++ {
		if ok, _, _ := l.Allow(a, ka); !ok {
			t.Fatalf("request %d refused", i+1)
		}
	}
	if ok, _, _ := l.Allow(a, ka); ok {
		t.Fatal("request 121 allowed")
	}
	if ok, _, _ := l.Allow(b, kb); !ok {
		t.Fatal("commune B refused on commune A's budget")
	}
}

func gateReq() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/commune-news?host=xa.vigov.vn", nil)
	r.RemoteAddr = "203.0.113.7:51000"
	return r
}

// FAIL OPEN for the public policy only: served, and ONE warning per window however many requests.
func TestGateFailOpenServesAndWarnsOncePerWindow(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	f := newFake()
	f.fail = errors.New("dial tcp: connection refused")
	l, _ := New(f, PublicNewsRead)
	clock := time.Unix(1_800_000_000, 0)
	l.now = func() time.Time { return clock }
	k, _ := PublicHostIPKey(context.Background(), false, "xa.vigov.vn", "203.0.113.7")

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		if !Gate(rec, gateReq(), l, k, log) || rec.Code != http.StatusOK {
			t.Fatalf("request %d: not served while the store is down (code %d)", i+1, rec.Code)
		}
	}
	if n := strings.Count(buf.String(), "rate_limit.store_unavailable"); n != 1 {
		t.Fatalf("warnings in one window = %d, want 1: %s", n, buf.String())
	}
	clock = clock.Add(PublicNewsReadWindow)
	Gate(httptest.NewRecorder(), gateReq(), l, k, log)
	if n := strings.Count(buf.String(), "rate_limit.store_unavailable"); n != 2 {
		t.Fatalf("a new window did not warn again (%d)", n)
	}
	if strings.Contains(buf.String(), "203.0.113.7") || strings.Contains(buf.String(), "rl:") {
		t.Fatalf("the outage warning names a client or a key: %s", buf.String())
	}
}

// GateOutcome keeps the three outcomes apart; Gate's bool is exactly "not Refused". The case that
// matters is the fail-open one: SERVED, yet NotEnforced — a caller whose side effect needs the bound
// (the public news view count) must be able to see that the bound was not there.
func TestGateOutcomeSeparatesEnforcedFromFailOpen(t *testing.T) {
	ctx := tenant.Into(context.Background(), tenant.ID(testCommune))
	k, _ := PublicHostIPKey(ctx, true, "xa.vigov.vn", "203.0.113.7")

	up, _ := New(newFake(), PublicNewsRead)
	rec := httptest.NewRecorder()
	if got := GateOutcome(rec, gateReq(), up, k, nil); got != Enforced || rec.Code != http.StatusOK {
		t.Fatalf("store up, within limit: outcome = %v, code = %d — want Enforced, nothing written", got, rec.Code)
	}
	for i := 1; i < PublicNewsReadLimit; i++ {
		GateOutcome(httptest.NewRecorder(), gateReq(), up, k, nil)
	}
	rec = httptest.NewRecorder()
	if got := GateOutcome(rec, gateReq(), up, k, nil); got != Refused || rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: outcome = %v, code = %d — want Refused, 429", got, rec.Code)
	}

	down := newFake()
	down.fail = errors.New("dial tcp: connection refused")
	open, _ := New(down, PublicNewsRead)
	rec = httptest.NewRecorder()
	if got := GateOutcome(rec, gateReq(), open, k, nil); got != NotEnforced || rec.Code != http.StatusOK {
		t.Fatalf("fail-open, store down: outcome = %v, code = %d — want NotEnforced, nothing written", got, rec.Code)
	}
	if !Gate(httptest.NewRecorder(), gateReq(), open, k, nil) {
		t.Fatal("Gate stopped serving on the fail-open outage — its behaviour must not change")
	}

	closedDown := newFake()
	closedDown.fail = errors.New("i/o timeout")
	rec = httptest.NewRecorder()
	if got := GateOutcome(rec, gateReq(), mustNew(t, closedDown), OperatorIPKey("203.0.113.7"), nil); got != Refused ||
		rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed policy, store down: outcome = %v, code = %d — want Refused, 503", got, rec.Code)
	}
}

// The closed policy through Gate is still closed.
func TestGateClosedPolicyAnswers503(t *testing.T) {
	f := newFake()
	f.fail = errors.New("i/o timeout")
	rec := httptest.NewRecorder()
	if Gate(rec, gateReq(), mustNew(t, f), OperatorIPKey("203.0.113.7"), nil) {
		t.Fatal("closed policy served while the store is down")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

// Over the limit through Gate: 429, Retry-After, the extra attrs on the security event.
func TestGate429CarriesAttrs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	l, _ := New(newFake(), PublicNewsRead)
	ctx := tenant.Into(context.Background(), tenant.ID(testCommune))
	k, _ := PublicHostIPKey(ctx, true, "xa.vigov.vn", "203.0.113.7")
	for i := 0; i < PublicNewsReadLimit; i++ {
		Gate(httptest.NewRecorder(), gateReq(), l, k, log, "xa", testCommune)
	}
	rec := httptest.NewRecorder()
	if Gate(rec, gateReq(), l, k, log, "xa", testCommune) {
		t.Fatal("request 121 served")
	}
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("code = %d, Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(buf.String(), `"event":"public_news.rate_limited"`) || !strings.Contains(buf.String(), `"xa":"`+testCommune+`"`) {
		t.Fatalf("event shape: %s", buf.String())
	}
}

// The citizen photo policy: its numbers pinned (a change must turn something red — they are a rule 13
// threshold, and today a PROPOSAL awaiting the owner), and it fails CLOSED like every policy but one.
func TestCitizenPhotoUploadPolicyIsPinnedAndFailsClosed(t *testing.T) {
	if CitizenPhotoUploadLimit != 30 || CitizenPhotoUploadWindow != 15*time.Minute {
		t.Fatalf("citizen photo threshold = %d per %s", CitizenPhotoUploadLimit, CitizenPhotoUploadWindow)
	}
	if CitizenPhotoUpload.FailsOpen() {
		t.Fatal("the citizen photo policy must fail closed — nobody decided a Redis outage lifts it")
	}
	if _, err := New(newFake(), CitizenPhotoUpload); err != nil {
		t.Fatal(err)
	}
}

// The staff image-fetch policy (owner, 03/10/2026, ADR 0067 K10): pinned, fails CLOSED, answers its own
// 429 code and sentence.
func TestStaffImageFetchIsThirtyPerHourClosedWithItsOwnRefusal(t *testing.T) {
	if StaffImageFetchLimit != 30 || StaffImageFetchWindow != time.Hour {
		t.Fatalf("K10 decided 30 / hour / officer, got %d / %v", StaffImageFetchLimit, StaffImageFetchWindow)
	}
	if StaffImageFetch.limit != StaffImageFetchLimit || StaffImageFetch.window != StaffImageFetchWindow {
		t.Fatal("StaffImageFetch does not use the named constants")
	}
	if StaffImageFetch.FailsOpen() {
		t.Fatal("the image-fetch limit must fail CLOSED — only the public news read has the owner's exception")
	}
	l, err := New(newFake(), StaffImageFetch)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenant.Into(context.Background(), tenant.ID(testCommune))
	k, err := ActorKey(ctx, "CB-00123")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < StaffImageFetchLimit; i++ {
		if !Gate(httptest.NewRecorder(), gateReq(), l, k, nil) {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	rec := httptest.NewRecorder()
	if Gate(rec, gateReq(), l, k, nil) {
		t.Fatal("attempt 31 served")
	}
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" ||
		!strings.Contains(rec.Body.String(), `"image_fetch_rate_limited"`) ||
		!strings.Contains(rec.Body.String(), "lấy ảnh từ liên kết quá nhiều lần") {
		t.Fatalf("code = %d, Retry-After = %q, body = %s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
	// The generic policies keep the generic answer.
	op := mustNew(t, newFake())
	for i := 0; i < OperatorSignInLimit; i++ {
		Gate(httptest.NewRecorder(), gateReq(), op, OperatorIPKey("203.0.113.7"), nil)
	}
	rec = httptest.NewRecorder()
	Gate(rec, gateReq(), op, OperatorIPKey("203.0.113.7"), nil)
	if !strings.Contains(rec.Body.String(), `"rate_limited"`) {
		t.Fatalf("operator 429 lost its generic code: %s", rec.Body.String())
	}
	if _, err := New(newFake(), Policy{name: "x", limit: 1, window: time.Second, event: "x", refusedCode: "c"}); err == nil {
		t.Fatal("a refusal code without its sentence accepted")
	}
}

// One counter per (commune, officer code); no commune or no code is a refusal, never a shared key.
func TestActorKeyShape(t *testing.T) {
	f := newFake()
	l, _ := New(f, StaffImageFetch)
	a := tenant.Into(context.Background(), tenant.ID(testCommune))
	b := tenant.Into(context.Background(), tenant.ID("01JTESTCOMMUNEB00000000000"))
	ka, _ := ActorKey(a, "CB-00123")
	kb, _ := ActorKey(b, "CB-00123")
	_, _, _ = l.Allow(a, ka)
	_, _, _ = l.Allow(b, kb)
	want := []string{"t:" + testCommune + ":rl:body-image-fetch:actor:CB-00123",
		"t:01JTESTCOMMUNEB00000000000:rl:body-image-fetch:actor:CB-00123"}
	if len(f.keys) != 2 || f.keys[0] != want[0] || f.keys[1] != want[1] {
		t.Fatalf("keys = %v, want %v", f.keys, want)
	}
	if _, err := ActorKey(context.Background(), "CB-1"); !errors.Is(err, ErrNoCommune) {
		t.Errorf("no commune: %v", err)
	}
	if _, err := ActorKey(a, " "); !errors.Is(err, ErrNoActor) {
		t.Errorf("no code: %v", err)
	}
}

// One counter per (commune, citizen); the citizen id never appears in the key as written; no commune
// or no citizen is a refusal, never a shared key.
func TestCitizenKeyShape(t *testing.T) {
	f := newFake()
	l, _ := New(f, CitizenPhotoUpload)
	a := tenant.Into(context.Background(), tenant.ID(testCommune))
	b := tenant.Into(context.Background(), tenant.ID("01JTESTCOMMUNEB00000000000"))

	ka, err := CitizenKey(a, "cd-01JCONGDANTHU")
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := CitizenKey(b, "cd-01JCONGDANTHU")
	kOther, _ := CitizenKey(a, "cd-01JNGUOIKHAC")
	for _, k := range []Key{ka, kb, kOther} {
		_, _, _ = l.Allow(a, k)
	}
	if len(f.keys) != 3 || f.keys[0] == f.keys[1] || f.keys[0] == f.keys[2] {
		t.Fatalf("keys not separated per commune and citizen: %v", f.keys)
	}
	if !strings.HasPrefix(f.keys[0], "t:"+testCommune+":rl:citizen-photo:citizen:") {
		t.Errorf("key = %q", f.keys[0])
	}
	for _, k := range f.keys {
		if strings.Contains(k, "CONGDAN") || strings.Contains(k, "NGUOIKHAC") {
			t.Errorf("citizen id written into the key: %q", k)
		}
	}
	if _, err := CitizenKey(context.Background(), "cd-1"); !errors.Is(err, ErrNoCommune) {
		t.Errorf("no commune: %v", err)
	}
	if _, err := CitizenKey(a, " "); !errors.Is(err, ErrNoCitizen) {
		t.Errorf("no citizen: %v", err)
	}
}

// The Zalo Bot pairing policy (ADR 0074, owner 05/10/2026): 5 per hour per chat, fails CLOSED. The
// webhook policy's number is provisional (rule 13 stop condition, open) — pinned so a change turns red.
func TestZaloBotPoliciesArePinnedAndFailClosed(t *testing.T) {
	if ZaloBotPairingLimit != 5 || ZaloBotPairingWindow != time.Hour {
		t.Fatalf("ADR 0074 decided 5 tries per hour per chat, got %d / %v", ZaloBotPairingLimit, ZaloBotPairingWindow)
	}
	if ZaloBotWebhookLimit != 600 || ZaloBotWebhookWindow != time.Minute {
		t.Fatalf("webhook threshold = %d per %v", ZaloBotWebhookLimit, ZaloBotWebhookWindow)
	}
	for _, p := range []Policy{ZaloBotPairing, ZaloBotWebhook} {
		if p.FailsOpen() {
			t.Fatalf("%s must fail closed", p.name)
		}
		if _, err := New(newFake(), p); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := New(newFake(), ZaloBotPairing)
	k, _ := ZaloChatKey("chat-1")
	for i := 0; i < ZaloBotPairingLimit; i++ {
		if ok, _, _ := l.Allow(context.Background(), k); !ok {
			t.Fatalf("try %d refused", i+1)
		}
	}
	if ok, _, _ := l.Allow(context.Background(), k); ok {
		t.Fatal("the sixth try in the hour was allowed")
	}
}

// Platform-realm keys: no commune prefix, chat id hashed, never written as is; an empty chat id is
// refused rather than counted under a shared key.
func TestZaloKeysShape(t *testing.T) {
	f := newFake()
	pairing, _ := New(f, ZaloBotPairing)
	webhook, _ := New(f, ZaloBotWebhook)
	k1, err := ZaloChatKey("1234567890123")
	if err != nil {
		t.Fatal(err)
	}
	k2, _ := ZaloChatKey("9999999999999")
	_, _, _ = pairing.Allow(context.Background(), k1)
	_, _, _ = pairing.Allow(context.Background(), k2)
	_, _, _ = webhook.Allow(context.Background(), WebhookIPKey("203.0.113.7"))
	if len(f.keys) != 3 || f.keys[0] == f.keys[1] {
		t.Fatalf("keys = %v", f.keys)
	}
	if !strings.HasPrefix(f.keys[0], "rl:zalo-bot-pairing:zalo-chat:") || strings.Contains(f.keys[0], "1234567890123") {
		t.Errorf("chat key = %q", f.keys[0])
	}
	if f.keys[2] != "rl:zalo-bot-webhook:ip:203.0.113.7" {
		t.Errorf("webhook key = %q", f.keys[2])
	}
	if _, err := ZaloChatKey(" "); !errors.Is(err, ErrNoChat) {
		t.Errorf("empty chat: %v", err)
	}
}

// The public Mini App ID lookup (service-platform, owner option A 06/10/2026): a provisional 30 per
// minute per client network, fails CLOSED, keyed with no commune prefix. Pinned so a change turns red.
func TestMiniAppIDLookupPolicyIsPinnedFailClosedAndPerNetwork(t *testing.T) {
	if MiniAppIDLookupLimit != 30 || MiniAppIDLookupWindow != time.Minute {
		t.Fatalf("mini-app-ids threshold = %d per %v", MiniAppIDLookupLimit, MiniAppIDLookupWindow)
	}
	if MiniAppIDLookup.FailsOpen() {
		t.Fatal("mini-app-ids must fail closed")
	}
	f := newFake()
	l, err := New(f, MiniAppIDLookup)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MiniAppIDLookupLimit; i++ {
		if ok, _, _ := l.Allow(context.Background(), PlatformIPKey("203.0.113.7")); !ok {
			t.Fatalf("request %d refused", i+1)
		}
	}
	if ok, _, _ := l.Allow(context.Background(), PlatformIPKey("203.0.113.7")); ok {
		t.Fatal("request 31 in the minute was allowed")
	}
	_, _, _ = l.Allow(context.Background(), PlatformIPKey("2001:db8:1:2::9"))
	if got := f.keys[0]; got != "rl:mini-app-ids:ip:203.0.113.7" {
		t.Errorf("key = %q", got)
	}
	if got := f.keys[len(f.keys)-1]; got != "rl:mini-app-ids:ip:2001:db8:1:2::/64" {
		t.Errorf("IPv6 key = %q", got)
	}
}
