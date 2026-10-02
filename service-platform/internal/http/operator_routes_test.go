package http

// What these tests defend: every operator route, through the REAL handler OperatorHandler builds
// (request guard included), with the REAL guards (internal/opauth), the REAL rate-limit middleware
// and the REAL `op1.` verifier —
// only identity, Redis and the database are fakes.
//
// RULE 5 INVARIANT 7, READ FOR THE OPERATOR REALM. Its four cases are 401 · 403 wrong permission ·
// 403 right permission WRONG COMMUNE · 200. The third has no operator analogue — an operator belongs
// to no commune (ADR 0048 condition #3), so there is no commune to be wrong about. What takes its
// place is the realm boundary (ADR 0048 stop condition #6): a well-formed STAFF token is refused here
// with 401 and never reaches identity. Plus 503 when identity is down, which must never read as 401.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/operatortoken"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// Fake material — the text says so in full (rule 8, forbidden #1).
var (
	opKeyFake    = secret.Secret("operator-signing-key-FAKE-0123456789abcdef")
	staffKeyFake = secret.Secret("staff-signing-key-FAKE-0123456789abcdefgh")
	nowFake      = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
)

const (
	operatorHostFake = "admin.vigov.vn"
	communeIDFake    = "01JD8ZQK9M3NPXR7TVWYB2C4EF"
	newIDFake        = "01JD8ZQK9M3NPXR7TVWYB2C4EG"
	opCodeFake       = "VH-00001"
)

// --- fakes -------------------------------------------------------------------------------------

type identityFake struct {
	mu        sync.Mutex
	calls     int
	down      bool
	live      bool
	keys      []string
	open      operatorclient.OpenResult
	change    operatorclient.ChangePasswordResult
	regen     operatorclient.RecoveryCodesResult
	begin     operatorclient.BeginEnrollmentResult
	complete  operatorclient.CompleteEnrollmentResult
	sawOpen   operatorclient.OpenRequest
	revoked   int
	errAnswer error

	// The two Mini App secret RPCs (operator_mini_app_secrets_test.go). A nil result answers
	// ACCEPTED with complete metadata.
	setRes          *operatorclient.SetMiniAppSecretResult
	setErr          error
	retireRes       *operatorclient.RetireMiniAppSecretResult
	retireErr       error
	sawSet          []operatorclient.SetMiniAppSecretRequest
	sawRetire       []operatorclient.RetireMiniAppSecretRequest
	sawSetTenant    []tenant.ID
	sawRetireTenant []tenant.ID
}

func (f *identityFake) hit() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.down {
		return errors.New("identity: unavailable (fake)")
	}
	return f.errAnswer
}

func (f *identityFake) Resolve(context.Context, secret.Secret) (operatorclient.Principal, bool, error) {
	if err := f.hit(); err != nil {
		return operatorclient.Principal{}, false, err
	}
	if !f.live {
		return operatorclient.Principal{}, false, nil
	}
	return operatorclient.Principal{OperatorID: "op-internal-1", OperatorCode: opCodeFake, PermissionKeys: f.keys}, true, nil
}

func (f *identityFake) Open(_ context.Context, req operatorclient.OpenRequest) (operatorclient.OpenResult, error) {
	f.sawOpen = req
	return f.open, f.hit()
}

func (f *identityFake) Revoke(context.Context, secret.Secret, string) (operatorclient.Outcome, error) {
	f.revoked++
	return operatorclient.OutcomeAccepted, f.hit()
}

func (f *identityFake) ChangePassword(context.Context, operatorclient.ChangePasswordRequest) (operatorclient.ChangePasswordResult, error) {
	return f.change, f.hit()
}

func (f *identityFake) RegenerateRecoveryCodes(context.Context, secret.Secret, secret.Secret, string) (operatorclient.RecoveryCodesResult, error) {
	return f.regen, f.hit()
}

func (f *identityFake) BeginEnrollment(context.Context, operatorclient.BeginEnrollmentRequest) (operatorclient.BeginEnrollmentResult, error) {
	return f.begin, f.hit()
}

func (f *identityFake) CompleteEnrollment(context.Context, operatorclient.CompleteEnrollmentRequest) (operatorclient.CompleteEnrollmentResult, error) {
	return f.complete, f.hit()
}

type registryFake struct{}

func (registryFake) ListCommunes(context.Context, page.Request) (page.Result[domain.Commune], error) {
	r := page.NewResult[domain.Commune]()
	r.Items = append(r.Items, domain.Commune{ID: communeIDFake, Name: "Xã Thăng Bình", Province: "Đà Nẵng",
		Active: true, Domains: []string{"thangbinh-danang.vigov.vn"}})
	return r, nil
}

func (registryFake) Commune(_ context.Context, id string) (domain.Commune, []domain.CommuneMiniApp, error) {
	if id != communeIDFake && id != newIDFake {
		return domain.Commune{}, nil, store.ErrCommuneNotFound
	}
	// Two dedicated apps, one on and one off — the secret routes answer 404 for any other App ID.
	return domain.Commune{ID: id, Name: "Xã Thăng Bình", Province: "Đà Nẵng", Active: true,
			Domains: []string{"thangbinh-danang.vigov.vn"}}, []domain.CommuneMiniApp{
			{AppID: "3291993990104489440", Mode: domain.CheDoRieng, Active: true, CreatedBy: opCodeFake},
			{AppID: "3043188591857102858", Mode: domain.CheDoRieng, Active: false, CreatedBy: opCodeFake},
		}, nil
}

func (registryFake) Provinces(context.Context) ([]domain.Province, error) {
	return []domain.Province{{ID: "66QW36RCJ7GVW79W8GJRYH3ZNR", Name: "Đà Nẵng"}}, nil
}

// writerFake records the commune each write targeted (from ctx — rule 1 invariant 4) and the actor.
type writerFake struct {
	err       error
	target    tenant.ID
	actor     domain.OperatorActor
	lastName  string
	lastHost  string
	lastState *bool
	// The mini-app routes: the App ID of the path, the new one of a replacement, the reason.
	lastApp, lastNewApp, lastReason string
}

func (w *writerFake) record(ctx context.Context, by domain.OperatorActor) {
	w.target, _ = tenant.From(ctx)
	w.actor = by
}

func (w *writerFake) CreateCommune(ctx context.Context, in store.NewCommune, by domain.OperatorActor) (domain.Commune, error) {
	w.record(ctx, by)
	w.lastName, w.lastHost = in.Name, in.Host
	return domain.Commune{ID: string(w.target)}, w.err
}

func (w *writerFake) AddDomain(ctx context.Context, host string, by domain.OperatorActor) error {
	w.record(ctx, by)
	w.lastHost = host
	return w.err
}

func (w *writerFake) SetPrimaryDomain(ctx context.Context, host string, by domain.OperatorActor) (bool, error) {
	w.record(ctx, by)
	w.lastHost = host
	return true, w.err
}

func (w *writerFake) CorrectName(ctx context.Context, name, _ string, by domain.OperatorActor) (bool, error) {
	w.record(ctx, by)
	w.lastName = name
	return true, w.err
}

func (w *writerFake) SetActivation(ctx context.Context, active bool, _ string, by domain.OperatorActor) (bool, []string, error) {
	w.record(ctx, by)
	w.lastState = &active
	return true, []string{"thangbinh-danang.vigov.vn"}, w.err
}

func (w *writerFake) AttachMiniApp(ctx context.Context, appID, _ string, by domain.OperatorActor) (domain.CommuneMiniApp, error) {
	w.record(ctx, by)
	return domain.CommuneMiniApp{AppID: appID, Mode: domain.CheDoRieng, Active: true, CreatedBy: by.Code}, w.err
}

func (w *writerFake) ReplaceMiniApp(ctx context.Context, oldAppID, newAppID, reason string, by domain.OperatorActor) (domain.CommuneMiniApp, error) {
	w.record(ctx, by)
	w.lastApp, w.lastNewApp, w.lastReason = oldAppID, newAppID, reason
	return domain.CommuneMiniApp{AppID: newAppID, Mode: domain.CheDoRieng, Active: true, CreatedBy: by.Code}, w.err
}

func (w *writerFake) SetMiniAppActivation(ctx context.Context, appID string, active bool, reason string, by domain.OperatorActor) (bool, error) {
	w.record(ctx, by)
	w.lastApp, w.lastReason, w.lastState = appID, reason, &active
	return true, w.err
}

// memCounter is Redis, absent: one fixed window per key.
type memCounter struct {
	mu   sync.Mutex
	n    map[string]int64
	down bool
}

func (c *memCounter) Incr(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.down {
		return 0, 0, errors.New("redis: down (fake)")
	}
	if c.n == nil {
		c.n = map[string]int64{}
	}
	c.n[key]++
	return c.n[key], window, nil
}

// --- harness ------------------------------------------------------------------------------------

type harness struct {
	mux     http.Handler
	id      *identityFake
	w       *writerFake
	counter *memCounter
	forgot  []string
	logs    *bytes.Buffer // JSON lines the handlers logged
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{id: &identityFake{live: true}, w: &writerFake{}, counter: &memCounter{}, logs: &bytes.Buffer{}}
	signer, err := operatortoken.NewSigner([]secret.Secret{opKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	lim, err := ratelimit.New(h.counter, ratelimit.OperatorSignIn)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(h.logs, nil))
	h.mux = OperatorHandler(OperatorDeps{
		Auth:         opauth.NewAuth(signer, h.id, log).WithClock(func() time.Time { return nowFake }),
		Identity:     h.id,
		Limiter:      lim,
		Registry:     registryFake{},
		Writer:       h.w,
		OperatorHost: operatorHostFake,
		NewID:        func() (string, error) { return newIDFake, nil },
		Forget:       func(host string) { h.forgot = append(h.forgot, host) },
		Now:          func() time.Time { return nowFake },
		Log:          log,
	})
	return h
}

func opCookie(t *testing.T) string {
	t.Helper()
	s, _ := operatortoken.NewSigner([]secret.Secret{opKeyFake})
	tok, err := s.Sign(operatortoken.Claims{SessionID: "sid-FAKE", ExpiresAt: nowFake.Add(8 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func staffCookie(t *testing.T) string {
	t.Helper()
	s, _ := token.NewSigner([]secret.Secret{staffKeyFake})
	tok, err := s.Ky(token.Claims{TenantID: communeIDFake, Sid: "sid", ExpiresAt: nowFake.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// do sends what the console sends: for a write, the console's own Origin and a JSON body type.
// The request guard's refusals are tested with doRaw.
func (h *harness) do(method, path, body, cookie string) *httptest.ResponseRecorder {
	hdr := http.Header{}
	if method != http.MethodGet && method != http.MethodHead {
		hdr.Set("Origin", "https://"+operatorHostFake)
		if body != "" {
			hdr.Set("Content-Type", "application/json")
		}
	}
	return h.doRaw(method, path, body, cookie, hdr)
}

func (h *harness) doRaw(method, path, body, cookie string, hdr http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = operatorHostFake
	for k, v := range hdr {
		req.Header[k] = v
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: opauth.CookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

// --- the guarded routes: 401 · 401 staff token · 403 · 503 · 2xx ----------------------------------

type guardedRoute struct {
	method, path, body string
	keys               []string // the keys that make it succeed
	ok                 int
}

var guarded = []guardedRoute{
	{"GET", "/api/v1/communes", "", []string{"ops.tenant.manage"}, 200},
	{"GET", "/api/v1/communes/" + communeIDFake, "", []string{"ops.tenant.manage"}, 200},
	{"GET", "/api/v1/provinces", "", []string{"ops.tenant.manage"}, 200},
	{"POST", "/api/v1/communes", `{"name":"Xã Mới","province_id":"66QW36RCJ7GVW79W8GJRYH3ZNR","primary_domain":"xamoi.vigov.vn"}`,
		[]string{"ops.tenant.manage", "ops.domain.manage"}, 201},
	{"POST", "/api/v1/communes/" + communeIDFake + "/domains", `{"domain":"thangbinh.example.gov.vn"}`,
		[]string{"ops.domain.manage"}, 201},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/primary-domain", `{"domain":"thangbinh-danang.vigov.vn"}`,
		[]string{"ops.domain.manage"}, 200},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/name", `{"name":"Xã Thăng Bình","reason":"Sửa lỗi gõ"}`,
		[]string{"ops.tenant.manage"}, 200},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/activation", `{"active":false,"reason":"Sáp nhập theo NQ"}`,
		[]string{"ops.tenant.manage"}, 200},
	{"POST", "/api/v1/communes/" + communeIDFake + "/mini-apps", `{"app_id":"3291993990104489440"}`,
		[]string{"ops.mini_app.manage"}, 201},
	{"POST", "/api/v1/communes/" + communeIDFake + "/mini-apps/3291993990104489440/replacement",
		`{"new_app_id":"3043188591857102858","reason":"Xã đổi App ID"}`, []string{"ops.mini_app.manage"}, 201},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/mini-apps/3291993990104489440/activation",
		`{"active":false,"reason":"Xã ngừng dùng app riêng"}`, []string{"ops.mini_app.manage"}, 200},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/mini-apps/3291993990104489440/secret",
		`{"secret":"zalo-app-secret-FAKE-NOT-REAL","reason":"Đặt khoá cho app riêng"}`, []string{"ops.mini_app.manage"}, 200},
	{"DELETE", "/api/v1/communes/" + communeIDFake + "/mini-apps/3291993990104489440/secret",
		`{"reason":"Thu hồi khoá cũ"}`, []string{"ops.mini_app.manage"}, 200},
	{"GET", "/api/v1/operator-sessions/current", "", nil, 200},
	{"DELETE", "/api/v1/operator-sessions/current", "", nil, 204},
}

func TestGuardedRoutes(t *testing.T) {
	for _, rt := range guarded {
		name := rt.method + " " + rt.path
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			if rec := h.do(rt.method, rt.path, rt.body, ""); rec.Code != 401 {
				t.Errorf("no cookie: %d, want 401", rec.Code)
			}
			if rec := h.do(rt.method, rt.path, rt.body, "op1.forged.AAAA"); rec.Code != 401 {
				t.Errorf("forged cookie: %d, want 401", rec.Code)
			}
			if rec := h.do(rt.method, rt.path, rt.body, staffCookie(t)); rec.Code != 401 {
				t.Errorf("staff token: %d, want 401", rec.Code)
			}
			if h.id.calls != 0 {
				t.Errorf("identity called %d times for cookies the signature already refused", h.id.calls)
			}

			h.id.down = true
			if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 503 {
				t.Errorf("identity down: %d, want 503", rec.Code)
			}
			h.id.down = false

			if len(rt.keys) > 0 {
				// Every key but the last — the wrong permission set.
				h.id.keys = append([]string{"ops.qr.issue"}, rt.keys[:len(rt.keys)-1]...)
				if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 403 {
					t.Errorf("missing a key: %d, want 403", rec.Code)
				}
				if h.w.target != "" {
					t.Errorf("a write ran without its key (target %s)", h.w.target)
				}
			}

			h.id.keys = rt.keys
			if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != rt.ok {
				t.Errorf("authorised: %d, want %d (%s)", rec.Code, rt.ok, rec.Body)
			}
		})
	}
}

// Every write records its actor as the operator's business code — never the internal id — and
// targets the commune of the PATH, never one from the body.
func TestWritesAuditActorAndPathTarget(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.domain.manage"}
	body := `{"domain":"thangbinh.example.gov.vn","tenant_id":"01JD8ZQK9M3NPXR7TVWYB2C4EH"}`
	if rec := h.do("POST", "/api/v1/communes/"+communeIDFake+"/domains", body, opCookie(t)); rec.Code != 400 {
		t.Fatalf("a body naming a commune must be refused as an unknown field: %d", rec.Code)
	}
	rec := h.do("POST", "/api/v1/communes/"+communeIDFake+"/domains", `{"domain":"thangbinh.example.gov.vn"}`, opCookie(t))
	if rec.Code != 201 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	if h.w.target != communeIDFake || h.w.actor.Code != opCodeFake || h.w.actor.IP == "" {
		t.Errorf("target %q actor %+v", h.w.target, h.w.actor)
	}
	if len(h.forgot) != 1 || h.forgot[0] != "thangbinh.example.gov.vn" {
		t.Errorf("cache not told about the new host: %v", h.forgot)
	}
}

func TestCreateCommuneRefusals(t *testing.T) {
	body := func(name, domain string) string {
		return `{"name":"` + name + `","province_id":"66QW36RCJ7GVW79W8GJRYH3ZNR","primary_domain":"` + domain + `"}`
	}
	cases := []struct {
		name, body string
		storeErr   error
		want       int
		code       string
	}{
		{"reserved domain", body("Xã A", "api.vigov.vn"), nil, 422, "reserved_domain"},
		{"OPERATOR_HOST as domain", body("Xã A", operatorHostFake), nil, 422, "reserved_domain"},
		{"domain with port", body("Xã A", "xa.vigov.vn:443"), nil, 422, "invalid_domain"},
		{"blank name", body("  ", "xa.vigov.vn"), nil, 422, "invalid_name"},
		{"duplicate domain", body("Xã A", "xa.vigov.vn"), store.ErrDomainTaken, 409, "domain_taken"},
		{"duplicate name in province", body("Xã A", "xa.vigov.vn"), store.ErrDuplicateName, 409, "duplicate_name"},
		{"unknown province", body("Xã A", "xa.vigov.vn"), store.ErrProvinceNotFound, 422, "unknown_province"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage"}
			h.w.err = c.storeErr
			rec := h.do("POST", "/api/v1/communes", c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
		})
	}

	// The happy path targets the NEW id generated by the server, in ctx.
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage"}
	if rec := h.do("POST", "/api/v1/communes", body("  Xã   Mới ", "XaMoi.vigov.vn"), opCookie(t)); rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if h.w.target != newIDFake || h.w.lastHost != "xamoi.vigov.vn" || h.w.lastName != "Xã   Mới" {
		t.Errorf("target %q host %q name %q", h.w.target, h.w.lastHost, h.w.lastName)
	}
}

func TestActivationNeedsExplicitStateAndReason(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	path := "/api/v1/communes/" + communeIDFake + "/activation"
	if rec := h.do("PUT", path, `{"reason":"x"}`, opCookie(t)); rec.Code != 400 {
		t.Errorf("absent active: %d, want 400 (never a silent false)", rec.Code)
	}
	if rec := h.do("PUT", path, `{"active":false,"reason":"  "}`, opCookie(t)); rec.Code != 422 {
		t.Errorf("blank reason: %d, want 422", rec.Code)
	}
	if h.w.lastState != nil {
		t.Error("a refused request reached the writer")
	}
	if rec := h.do("PUT", path, `{"active":false,"reason":"Sáp nhập"}`, opCookie(t)); rec.Code != 200 {
		t.Fatalf("deactivate: %d", rec.Code)
	}
	if len(h.forgot) == 0 {
		t.Error("deactivation must drop the commune's hosts from this process's cache")
	}
}

func TestUnknownCommuneIs404(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	for _, id := range []string{"not-a-ulid", "01JD8ZQK9M3NPXR7TVWYB2C4EZ"} {
		if rec := h.do("GET", "/api/v1/communes/"+id, "", opCookie(t)); rec.Code != 404 {
			t.Errorf("GET %s: %d, want 404", id, rec.Code)
		}
	}
}

// No route removes a domain or moves it (ADR 0048 §01/10 #5): those methods must not exist.
func TestNoDomainRemovalOrRepoint(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.domain.manage", "ops.tenant.manage"}
	for _, m := range []struct{ method, path string }{
		{"DELETE", "/api/v1/communes/" + communeIDFake + "/domains/thangbinh-danang.vigov.vn"},
		{"DELETE", "/api/v1/communes/" + communeIDFake},
		{"PUT", "/api/v1/communes/" + communeIDFake + "/domains"},
	} {
		if rec := h.do(m.method, m.path, "{}", opCookie(t)); rec.Code != 404 && rec.Code != 405 {
			t.Errorf("%s %s answered %d — the route must not exist", m.method, m.path, rec.Code)
		}
	}
}

// --- the sign-in routes ---------------------------------------------------------------------------

func TestSignInAccepted(t *testing.T) {
	h := newHarness(t)
	h.id.open = operatorclient.OpenResult{Outcome: operatorclient.OutcomeAccepted, Session: operatorclient.SessionGrant{
		Token: secret.Secret("op1.issued.FAKE"), ExpiresAt: nowFake.Add(8 * time.Hour), OperatorCode: opCodeFake}}
	rec := h.do("POST", "/api/v1/operator-sessions",
		`{"email":"operator@example.invalid","password":"pw FAKE","totp_code":"000000"}`, "")
	if rec.Code != 201 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	sc := rec.Header().Get("Set-Cookie")
	for _, want := range []string{opauth.CookieName + "=op1.issued.FAKE", "HttpOnly", "Secure", "SameSite=Strict", "Path=/", "Max-Age=28800"} {
		if !strings.Contains(sc, want) {
			t.Errorf("Set-Cookie %q lacks %q", sc, want)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("credential response must be no-store")
	}
	if strings.Contains(rec.Body.String(), "op1.issued") {
		t.Error("the token must travel only in the cookie, never in the body")
	}
	if h.id.sawOpen.ClientIP == "" {
		t.Error("identity was not given the client address")
	}
}

func TestSignInOutcomes(t *testing.T) {
	for _, c := range []struct {
		name    string
		outcome operatorclient.Outcome
		want    int
		code    string
	}{
		{"refused", operatorclient.OutcomeRefused, 401, "sign_in_refused"},
		{"enrolment required", operatorclient.OutcomeEnrollmentRequired, 403, "enrollment_required"},
	} {
		h := newHarness(t)
		h.id.open = operatorclient.OpenResult{Outcome: c.outcome}
		rec := h.do("POST", "/api/v1/operator-sessions", `{"email":"x@example.invalid","password":"p"}`, "")
		if rec.Code != c.want || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
		}
	}
	h := newHarness(t)
	h.id.down = true
	if rec := h.do("POST", "/api/v1/operator-sessions", `{"email":"x@example.invalid","password":"p"}`, ""); rec.Code != 503 {
		t.Errorf("identity down on sign-in: %d, want 503 (never 401)", rec.Code)
	}
}

// NUL, invalid UTF-8 and both factors are refused BEFORE identity — none of them is a guess to count.
func TestSignInRefusesMalformedCredentialsLocally(t *testing.T) {
	bodies := map[string]string{
		"NUL in password":   `{"email":"x@example.invalid","password":"a\u0000b"}`,
		"NUL in email":      `{"email":"x\u0000@example.invalid","password":"p"}`,
		"lone surrogate":    `{"email":"x@example.invalid","password":"\ud800"}`,
		"both factors":      `{"email":"x@example.invalid","password":"p","totp_code":"1","recovery_code":"2"}`,
		"unknown field":     `{"email":"x@example.invalid","password":"p","tenant_id":"x"}`,
		"invalid UTF-8 raw": "{\"email\":\"x@example.invalid\",\"password\":\"\xff\xfe\"}",
	}
	for name, b := range bodies {
		h := newHarness(t)
		if rec := h.do("POST", "/api/v1/operator-sessions", b, ""); rec.Code != 400 {
			t.Errorf("%s: %d, want 400", name, rec.Code)
		}
		if h.id.calls != 0 {
			t.Errorf("%s: identity was called", name)
		}
	}
}

func TestSignInRateLimit(t *testing.T) {
	h := newHarness(t)
	h.id.open = operatorclient.OpenResult{Outcome: operatorclient.OutcomeRefused}
	b := `{"email":"x@example.invalid","password":"p"}`
	for i := 0; i < ratelimit.OperatorSignInLimit; i++ {
		if rec := h.do("POST", "/api/v1/operator-sessions", b, ""); rec.Code != 401 {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	callsBefore := h.id.calls
	rec := h.do("POST", "/api/v1/operator-sessions", b, "")
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("attempt 21: %d, want 429 with Retry-After", rec.Code)
	}
	if h.id.calls != callsBefore {
		t.Error("a throttled attempt reached identity — it would count toward the 12-hour lockout")
	}
	// The same budget covers the enrolment steps (one counter per IP).
	if rec := h.do("POST", "/api/v1/operator-enrollments", `{"email":"x@example.invalid","temporary_password":"t"}`, ""); rec.Code != 429 {
		t.Errorf("enrolment after the limit: %d, want 429", rec.Code)
	}

	down := newHarness(t)
	down.counter.down = true
	if rec := down.do("POST", "/api/v1/operator-sessions", b, ""); rec.Code != 503 {
		t.Errorf("limiter store down: %d, want 503 (fail closed)", rec.Code)
	}
	if down.id.calls != 0 {
		t.Error("identity called while the limiter could not count")
	}
}

func TestEnrollmentFlow(t *testing.T) {
	h := newHarness(t)
	h.id.begin = operatorclient.BeginEnrollmentResult{Outcome: operatorclient.OutcomeAccepted, OperatorCode: opCodeFake,
		ProvisioningURI: secret.Secret("otpauth://totp/ViGov:VH-00001?secret=FAKE"), ManualEntryKey: secret.Secret("FAKEKEY")}
	rec := h.do("POST", "/api/v1/operator-enrollments", `{"email":"x@example.invalid","temporary_password":"t"}`, "")
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), "otpauth://") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("begin: %d %s", rec.Code, rec.Body)
	}

	h.id.complete = operatorclient.CompleteEnrollmentResult{Outcome: operatorclient.OutcomeNewPasswordRejected,
		Refusal: operatorclient.NewPasswordRefusal{Problem: 3, MinLength: 12, MaxLength: 128}}
	rec = h.do("POST", "/api/v1/operator-enrollments/completion",
		`{"email":"x@example.invalid","temporary_password":"t","new_password":"short","totp_code":"000000"}`, "")
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"problem":"too_short"`) ||
		!strings.Contains(rec.Body.String(), `"min_length":12`) {
		t.Fatalf("rejected: %d %s", rec.Code, rec.Body)
	}

	h.id.complete = operatorclient.CompleteEnrollmentResult{Outcome: operatorclient.OutcomeAccepted,
		Session:       operatorclient.SessionGrant{Token: secret.Secret("op1.new.FAKE"), ExpiresAt: nowFake.Add(8 * time.Hour), OperatorCode: opCodeFake},
		RecoveryCodes: []secret.Secret{secret.Secret("rc-FAKE-1")}}
	rec = h.do("POST", "/api/v1/operator-enrollments/completion",
		`{"email":"x@example.invalid","temporary_password":"t","new_password":"long enough FAKE","totp_code":"000000"}`, "")
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), "rc-FAKE-1") ||
		!strings.Contains(rec.Header().Get("Set-Cookie"), "op1.new.FAKE") {
		t.Fatalf("completed: %d %s", rec.Code, rec.Body)
	}
}

func TestPasswordChangeAndRecoveryCodes(t *testing.T) {
	h := newHarness(t)
	pw := `{"current_password":"a","new_password":"b","totp_code":"000000"}`
	h.id.change = operatorclient.ChangePasswordResult{Outcome: operatorclient.OutcomeAccepted}
	rec := h.do("PUT", "/api/v1/operators/current/password", pw, opCookie(t))
	if rec.Code != 204 || !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("accepted: %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	h.id.change = operatorclient.ChangePasswordResult{Outcome: operatorclient.OutcomeRefused}
	if rec := h.do("PUT", "/api/v1/operators/current/password", pw, opCookie(t)); rec.Code != 403 {
		t.Errorf("wrong re-proof: %d, want 403 (the session is live)", rec.Code)
	}
	if rec := h.do("PUT", "/api/v1/operators/current/password", pw, ""); rec.Code != 401 {
		t.Errorf("no cookie: %d", rec.Code)
	}

	h.id.regen = operatorclient.RecoveryCodesResult{Outcome: operatorclient.OutcomeAccepted, Codes: []secret.Secret{secret.Secret("rc-FAKE-2")}}
	rec = h.do("POST", "/api/v1/operators/current/recovery-codes", `{"totp_code":"000000"}`, opCookie(t))
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), "rc-FAKE-2") {
		t.Fatalf("regenerate: %d %s", rec.Code, rec.Body)
	}
}

func TestWhoAmIAndSignOut(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	rec := h.do("GET", "/api/v1/operator-sessions/current", "", opCookie(t))
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"operator_code":"VH-00001"`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"ops.tenant.manage"`)) {
		t.Fatalf("who am I: %d %s", rec.Code, rec.Body)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("op-internal-1")) {
		t.Error("the internal operator id must not leave the service")
	}
	rec = h.do("DELETE", "/api/v1/operator-sessions/current", "", opCookie(t))
	if rec.Code != 204 || h.id.revoked != 1 || !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("sign out: %d revoked %d cookie %q", rec.Code, h.id.revoked, rec.Header().Get("Set-Cookie"))
	}
}
