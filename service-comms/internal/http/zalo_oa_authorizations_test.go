package http

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// THE OA CALLBACK. What these tests defend (ADR 0098 #4, #7):
//
//  1. the rate limit runs BEFORE the query is read, and fails closed;
//  2. an unknown / expired / used state is ONE 404 sentence naming no commune;
//  3. a spent request sends the browser back to THE COMMUNE's host with the outcome web-admin reads;
//  4. the use case runs with NO commune in the context — the commune is the state's, never Host's;
//  5. no log line carries the code, the state or the query.

// Fixtures, not credentials.
const (
	cbState = "QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI"
	cbCode  = "FAKE-oa-authorization-code-NOT-REAL"
	cbOAID  = "579745863508352884"
	cbHost  = "oa-callback.example.vn"
)

type fakeOACallback struct {
	calls      int
	in         app.CompleteAuthorizationInput
	hadCommune bool
	res        app.CompleteAuthorizationResult
	err        error
}

func (f *fakeOACallback) CompleteAuthorization(ctx context.Context, in app.CompleteAuthorizationInput) (app.CompleteAuthorizationResult, error) {
	f.calls++
	f.in = in
	_, f.hadCommune = tenant.From(ctx)
	return f.res, f.err
}

type oaCallbackServer struct {
	h       http.Handler
	fake    *fakeOACallback
	counter *memCounter
	logs    *bytes.Buffer
}

func newOACallbackServer(t *testing.T) *oaCallbackServer {
	t.Helper()
	fake := &fakeOACallback{res: app.CompleteAuthorizationResult{Tenant: xaA, Host: hostA, Outcome: domain.AuthorizationSucceeded}}
	counter := &memCounter{}
	lim, err := ratelimit.New(counter, ratelimit.ZaloOACallback)
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	mux := http.NewServeMux()
	RegisterZaloOAAuthorizations(mux, ZaloOAAuthorizationDeps{Authorizations: fake, Limiter: lim,
		Log: slog.New(slog.NewTextHandler(logs, nil))})
	return &oaCallbackServer{h: mux, fake: fake, counter: counter, logs: logs}
}

func (s *oaCallbackServer) get(t *testing.T, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+cbHost+ZaloOAAuthorizationsPath+"?"+rawQuery, nil)
	r.Host = cbHost
	r.RemoteAddr = "203.0.113.9:443"
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func goodCallbackQuery() string {
	return url.Values{"state": {cbState}, "code": {cbCode}, "oa_id": {cbOAID}}.Encode()
}

// noSecretInLogs: whatever happened, the security log holds neither the code, the state nor the query.
func (s *oaCallbackServer) noSecretInLogs(t *testing.T) {
	t.Helper()
	for _, needle := range []string{cbCode, cbState, "code=", "state=", ZaloOAAuthorizationsPath + "?"} {
		if strings.Contains(s.logs.String(), needle) {
			t.Fatalf("the log carries %q: %s", needle, s.logs.String())
		}
	}
}

func assertNoStoreNoReferrer(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers = %v — the URL holds a code and a state", w.Header())
	}
}

func TestOACallbackPathAgreesWithUseCase(t *testing.T) {
	// The literal is the one RegisterZaloOAAuthorizations registers (tools/apidoc reads it there).
	for _, p := range []string{app.ZaloOACallbackPath, "/api/v1/zalo-oa-authorizations"} {
		if p != ZaloOAAuthorizationsPath {
			t.Fatalf("%q vs %q — the link Zalo is given and the route served would differ", p, ZaloOAAuthorizationsPath)
		}
	}
}

func TestOACallbackSuccessRedirectsToCommuneZaloChannelTab(t *testing.T) {
	s := newOACallbackServer(t)
	w := s.get(t, goodCallbackQuery())
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", w.Code, w.Body.String())
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil || loc.Scheme != "https" || loc.Host != hostA || loc.Path != "/cau-hinh" ||
		loc.Query().Get("tab") != "kenh-zalo" || loc.Query().Get("oa") != "da-noi" || len(loc.Query()) != 2 {
		t.Fatalf("Location = %q", w.Header().Get("Location"))
	}
	assertNoStoreNoReferrer(t, w)
	if s.fake.calls != 1 || s.fake.in.State != cbState || s.fake.in.Code != cbCode || s.fake.in.OAID != cbOAID ||
		s.fake.in.ClientIP != "203.0.113.9" || s.fake.in.Denied {
		t.Fatalf("use case received %+v", s.fake.in)
	}
	if s.fake.hadCommune {
		t.Fatal("the use case ran with a commune in the context — the commune must come from the state only")
	}
	if strings.Contains(w.Header().Get("Location"), cbCode) || strings.Contains(w.Header().Get("Location"), cbState) {
		t.Fatal("the redirect carries the code or the state onward")
	}
	s.noSecretInLogs(t)
}

func TestOACallbackFailuresRedirectWithOutcomeScreenReads(t *testing.T) {
	// The six failure words of web-admin's CALLBACK_OUTCOME_TEXT the use case can produce, plus het-han —
	// all passed through as stored (ADR 0011); only success is renamed da-noi.
	cases := map[string]error{
		domain.AuthorizationZaloRefused:       app.ErrAuthorizationDenied,
		domain.AuthorizationZaloUnavailable:   &app.ZaloOACallError{Step: "exchange", Class: domain.ZaloCallUnavailable},
		domain.AuthorizationMalformedResponse: &app.ZaloOACallError{Step: "getoa", Class: domain.ZaloCallMalformedResponse},
		domain.AuthorizationOALiveElsewhere:   commsstore.ErrZaloOATaken,
		domain.AuthorizationMalformedCallback: app.ErrAuthorizationMalformed,
		domain.AuthorizationExpired:           errors.New("closed meanwhile"),
	}
	for outcome, err := range cases {
		t.Run(outcome, func(t *testing.T) {
			s := newOACallbackServer(t)
			s.fake.res = app.CompleteAuthorizationResult{Tenant: xaA, Host: hostA, Outcome: outcome}
			s.fake.err = err
			w := s.get(t, goodCallbackQuery())
			if w.Code != http.StatusFound {
				t.Fatalf("status = %d", w.Code)
			}
			loc, _ := url.Parse(w.Header().Get("Location"))
			if loc.Host != hostA || loc.Query().Get("oa") != outcome || loc.Query().Get("tab") != "kenh-zalo" {
				t.Fatalf("Location = %q", w.Header().Get("Location"))
			}
			assertNoStoreNoReferrer(t, w)
			s.noSecretInLogs(t)
		})
	}
}

func TestOACallbackUnknownStateIsOne404NamingNoCommune(t *testing.T) {
	for name, q := range map[string]string{
		"unknown":  goodCallbackQuery(),
		"no state": url.Values{"code": {cbCode}, "oa_id": {cbOAID}}.Encode(),
		"too long": "state=" + strings.Repeat("A", maxZaloOACallbackQuery),
	} {
		t.Run(name, func(t *testing.T) {
			s := newOACallbackServer(t)
			s.fake.res, s.fake.err = app.CompleteAuthorizationResult{}, app.ErrAuthorizationNotFound
			w := s.get(t, q)
			if w.Code != http.StatusNotFound || w.Header().Get("Location") != "" {
				t.Fatalf("status = %d Location %q", w.Code, w.Header().Get("Location"))
			}
			if !strings.Contains(w.Body.String(), zaloOACallbackNotFound) {
				t.Fatalf("body = %s", w.Body.String())
			}
			for _, bad := range []string{string(xaA), hostA, cbCode, cbState} {
				if strings.Contains(w.Body.String(), bad) {
					t.Fatalf("404 names %q", bad)
				}
			}
			if name != "unknown" && s.fake.calls != 0 {
				t.Fatal("a callback with no usable state reached the use case")
			}
			assertNoStoreNoReferrer(t, w)
			s.noSecretInLogs(t)
		})
	}
}

func TestOACallbackDeniedAtZaloIsPassedOn(t *testing.T) {
	s := newOACallbackServer(t)
	s.fake.res = app.CompleteAuthorizationResult{Tenant: xaA, Host: hostA, Outcome: domain.AuthorizationZaloRefused}
	s.fake.err = app.ErrAuthorizationDenied
	w := s.get(t, url.Values{"state": {cbState}, "error": {"access_denied"}, "error_description": {"user cancelled"}}.Encode())
	if !s.fake.in.Denied || s.fake.in.Code != "" {
		t.Fatalf("use case received %+v", s.fake.in)
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || loc.Query().Get("oa") != "zalo-tu-choi" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
	if strings.Contains(s.logs.String(), "access_denied") || strings.Contains(s.logs.String(), "user cancelled") {
		t.Fatal("Zalo's error words reached the log")
	}
}

// The limit is counted BEFORE anything is read: over it, 429 and the use case never runs; with the
// counter store down, 503 (fails closed).
func TestOACallbackRateLimitComesFirstAndFailsClosed(t *testing.T) {
	s := newOACallbackServer(t)
	for i := 0; i < ratelimit.ZaloOACallbackLimit; i++ {
		s.get(t, "state=x")
	}
	calls := s.fake.calls
	w := s.get(t, goodCallbackQuery())
	if w.Code != http.StatusTooManyRequests || s.fake.calls != calls {
		t.Fatalf("61st = %d, use case ran %d more", w.Code, s.fake.calls-calls)
	}
	if len(s.counter.keys) == 0 || !strings.HasPrefix(s.counter.keys[0], "rl:zalo-oa-callback:ip:") {
		t.Fatalf("keys = %v", s.counter.keys)
	}

	s = newOACallbackServer(t)
	s.counter.fail = errors.New("dial tcp: connection refused")
	w = s.get(t, goodCallbackQuery())
	if w.Code != http.StatusServiceUnavailable || s.fake.calls != 0 {
		t.Fatalf("Redis down = %d, use case ran %d", w.Code, s.fake.calls)
	}
	s.noSecretInLogs(t)
}

func TestOACallbackWithoutHostShowsPlainPage(t *testing.T) {
	for name, host := range map[string]string{"unknown": "", "not a bare host": "evil.example/x?y=", "user-info": "a@b.example"} {
		t.Run(name, func(t *testing.T) {
			s := newOACallbackServer(t)
			s.fake.res.Host = host
			w := s.get(t, goodCallbackQuery())
			if w.Code != http.StatusOK || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), zaloOACallbackConnectedNoHost) {
				t.Fatalf("%d %q %s", w.Code, w.Header().Get("Location"), w.Body.String())
			}
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") ||
				!strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Fatalf("headers = %v", w.Header())
			}
			s.fake.res.Outcome = domain.AuthorizationZaloUnavailable
			if w := s.get(t, goodCallbackQuery()); !strings.Contains(w.Body.String(), zaloOACallbackFailedNoHost) {
				t.Fatalf("failure without host = %s", w.Body.String())
			}
		})
	}
}

func TestOACallbackFaultsNameNothing(t *testing.T) {
	cases := []struct {
		name   string
		res    app.CompleteAuthorizationResult
		err    error
		status int
	}{
		{"app not configured", app.CompleteAuthorizationResult{}, app.ErrZaloOANotConfigured, http.StatusServiceUnavailable},
		{"resolver down", app.CompleteAuthorizationResult{}, errors.New("crosstenant: find: dial"), http.StatusServiceUnavailable},
		{"no KEK", app.CompleteAuthorizationResult{Tenant: xaA, Host: hostA}, crypto.ErrNotConfigured, http.StatusServiceUnavailable},
		{"db down", app.CompleteAuthorizationResult{Tenant: xaA, Host: hostA}, errors.New("boom " + string(xaB)), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newOACallbackServer(t)
			s.fake.res, s.fake.err = c.res, c.err
			w := s.get(t, goodCallbackQuery())
			if w.Code != c.status || w.Header().Get("Location") != "" {
				t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
			}
			for _, bad := range []string{string(xaA), string(xaB), "boom", "crosstenant", hostA} {
				if strings.Contains(w.Body.String(), bad) {
					t.Fatalf("page leaks %q", bad)
				}
			}
			assertNoStoreNoReferrer(t, w)
			s.noSecretInLogs(t)
		})
	}
}

// Mounted behind a commune Host by mistake: refused rather than letting Host and state disagree.
func TestOACallbackRefusesCommuneInContext(t *testing.T) {
	s := newOACallbackServer(t)
	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+ZaloOAAuthorizationsPath+"?"+goodCallbackQuery(), nil)
	r = r.WithContext(tenant.Into(r.Context(), xaA))
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound || s.fake.calls != 0 {
		t.Fatalf("%d, use case ran %d", w.Code, s.fake.calls)
	}
}

func TestRegisterZaloOAAuthorizationsRefusesIncompleteWiring(t *testing.T) {
	lim, _ := ratelimit.New(&memCounter{}, ratelimit.ZaloOACallback)
	for name, d := range map[string]ZaloOAAuthorizationDeps{
		"use case": {Limiter: lim},
		"limiter":  {Authorizations: &fakeOACallback{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloOAAuthorizations(http.NewServeMux(), d)
		})
	}
}
