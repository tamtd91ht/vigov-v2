package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// The public App ID lookup (owner, option A, 06/10/2026). Rule 5 invariant 7's four cases adapted to
// a route with no account, as identity's citizen-sessions tests adapt them: no credential is needed
// (the "401" case is a 200), a credential or a forged commune changes nothing (the "403" cases are the
// same answer), and the answer itself is checked. Plus: the rate limit, no-store on every answer, and
// one indistinguishable 404 for every absence.

type sharedReaderFake struct {
	app domain.SharedMiniApp
	err error
}

func (f *sharedReaderFake) SharedMiniApp(context.Context) (domain.SharedMiniApp, error) {
	return f.app, f.err
}

// ownResolverFake answers per host; an unknown host is ErrKhongCoMiniApp, as the store answers it.
type ownResolverFake struct {
	byHost map[string]string
	err    error
	asked  []string
}

func (f *ownResolverFake) LiveOwnMiniAppID(_ context.Context, host string) (string, error) {
	f.asked = append(f.asked, host)
	if f.err != nil {
		return "", f.err
	}
	if id, ok := f.byHost[host]; ok {
		return id, nil
	}
	return "", store.ErrKhongCoMiniApp
}

type miniAppIDHarness struct {
	mux     *http.ServeMux
	shared  *sharedReaderFake
	own     *ownResolverFake
	counter *memCounter
}

func newMiniAppIDHarness(t *testing.T) *miniAppIDHarness {
	t.Helper()
	h := &miniAppIDHarness{
		shared:  &sharedReaderFake{app: domain.SharedMiniApp{AppID: "3749147383835819800"}},
		own:     &ownResolverFake{byHost: map[string]string{"thangbinh.vigov.vn": "1234567890123456789"}},
		counter: &memCounter{},
	}
	lim, err := ratelimit.New(h.counter, ratelimit.MiniAppIDLookup)
	if err != nil {
		t.Fatal(err)
	}
	h.mux = http.NewServeMux()
	RegisterMiniAppIDs(h.mux, MiniAppIDDeps{Shared: h.shared, Own: h.own, Limiter: lim,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return h
}

func (h *miniAppIDHarness) get(query string, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app-ids"+query, nil)
	r.Host = "platform.api.vigov.vn"
	for _, m := range mutate {
		m(r)
	}
	w := httptest.NewRecorder()
	h.mux.ServeHTTP(w, r)
	return w
}

func wantMiniAppID(t *testing.T, w *httptest.ResponseRecorder, appID, source string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	// Exactly the two fields: no commune id, no name, nothing else a prober could collect.
	if len(out) != 2 || out["app_id"] != appID || out["source"] != source {
		t.Fatalf("body = %s, want exactly {app_id:%s, source:%s}", w.Body, appID, source)
	}
}

func wantCode(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, want %d; body %s", w.Code, status, w.Body)
	}
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Code != code {
		t.Fatalf("code = %q (%v), want %q; body %s", e.Code, err, code, w.Body)
	}
}

func TestMiniAppIDsSharedAndOwn200WithNoCredential(t *testing.T) {
	h := newMiniAppIDHarness(t)
	wantMiniAppID(t, h.get("?app=vihat"), "3749147383835819800", "chung")
	wantMiniAppID(t, h.get("?host=thangbinh.vigov.vn"), "1234567890123456789", "rieng")
	// The host is normalised as the registry stores it (case, surrounding space).
	wantMiniAppID(t, h.get("?host=ThangBinh.VIGOV.vn"), "1234567890123456789", "rieng")
}

// A staff cookie, a bearer token or a client-named commune grants nothing and changes nothing: the
// answer depends on the query alone. (Rule 5 #7's 403 cases, for a route with no account.)
func TestMiniAppIDsCredentialsAndForgedCommuneChangeNothing(t *testing.T) {
	h := newMiniAppIDHarness(t)
	plain := h.get("?host=khongco.vigov.vn")
	forged := h.get("?host=khongco.vigov.vn", func(r *http.Request) {
		r.Header.Set("Cookie", "vigov_session=x")
		r.Header.Set("Authorization", "Bearer x")
		r.Header.Set("X-Tenant-ID", "01JD8ZQK9M3NPXR7TVWYB2C4EF")
		r.Host = "thangbinh.vigov.vn"
	})
	if plain.Code != forged.Code || plain.Body.String() != forged.Body.String() {
		t.Fatalf("credentials changed the answer: %d %s vs %d %s", plain.Code, plain.Body, forged.Code, forged.Body)
	}
	wantCode(t, forged, http.StatusNotFound, "mini_app_id_not_found")
}

// Every absence is ONE body: unknown domain, reserved platform host, a commune with no own app or an
// inactive one (the store collapses those to ErrKhongCoMiniApp), and the shared app not declared.
func TestMiniAppIDs404IsIndistinguishable(t *testing.T) {
	h := newMiniAppIDHarness(t)
	var bodies []string
	for _, q := range []string{"?host=khongco.vigov.vn", "?host=admin.vigov.vn", "?host=identity.api.vigov.vn"} {
		w := h.get(q)
		wantCode(t, w, http.StatusNotFound, "mini_app_id_not_found")
		bodies = append(bodies, w.Body.String())
	}
	h.shared.err = store.ErrNoSharedMiniApp
	w := h.get("?app=vihat")
	wantCode(t, w, http.StatusNotFound, "mini_app_id_not_found")
	bodies = append(bodies, w.Body.String())
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Fatalf("404 bodies differ:\n%s\n%s", bodies[0], b)
		}
	}
	// A reserved host is answered without asking the registry.
	for _, asked := range h.own.asked {
		if asked == "admin.vigov.vn" {
			t.Error("reserved host reached the registry")
		}
	}
}

func TestMiniAppIDsInvalidQuery400(t *testing.T) {
	h := newMiniAppIDHarness(t)
	for _, q := range []string{
		"", "?app=", "?app=khac", "?app=VIHAT", "?app=vihat&host=thangbinh.vigov.vn",
		"?app=vihat&app=vihat", "?host=thangbinh.vigov.vn&host=x.vigov.vn",
		"?host=", "?host=https://thangbinh.vigov.vn", "?host=thangbinh.vigov.vn:443",
		"?host=thangbinh.vigov.vn/x", "?host=localhost", "?host=192.0.2.1", "?host=th%C3%A0nh.vigov.vn",
	} {
		w := h.get(q)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", q, w.Code)
			continue
		}
		wantCode(t, w, http.StatusBadRequest, "invalid_query")
	}
	if len(h.own.asked) != 0 {
		t.Fatalf("a malformed query reached the registry: %v", h.own.asked)
	}
}

func TestMiniAppIDsAmbiguous409NeverPicks(t *testing.T) {
	h := newMiniAppIDHarness(t)
	h.shared.err = store.ErrSharedMiniAppAmbiguous
	wantCode(t, h.get("?app=vihat"), http.StatusConflict, "shared_mini_app_ambiguous")
	h.own.err = store.ErrOwnMiniAppAmbiguous
	wantCode(t, h.get("?host=thangbinh.vigov.vn"), http.StatusConflict, "own_mini_app_ambiguous")
}

func TestMiniAppIDsStoreFailure500LeaksNothing(t *testing.T) {
	h := newMiniAppIDHarness(t)
	h.own.err = errors.New("directory: own mini app: connection reset")
	w := h.get("?host=thangbinh.vigov.vn")
	wantCode(t, w, http.StatusInternalServerError, "internal")
	if strings.Contains(w.Body.String(), "connection reset") {
		t.Fatal("500 body leaks the cause")
	}
}

// no-store on EVERY answer: an App ID replaced in the console is effective at once (ADR 0070).
func TestMiniAppIDsNoStoreOnEveryAnswer(t *testing.T) {
	h := newMiniAppIDHarness(t)
	for _, q := range []string{"?app=vihat", "?host=thangbinh.vigov.vn", "?host=khongco.vigov.vn", "?app=khac"} {
		if cc := h.get(q).Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%q: Cache-Control = %q", q, cc)
		}
	}
	h.counter.down = true
	w := h.get("?app=vihat")
	wantCode(t, w, http.StatusServiceUnavailable, "rate_limit_unavailable")
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("503: Cache-Control = %q", cc)
	}
}

// 30 per minute per client network, malformed probes included; another network is not limited; a
// Redis outage refuses (fail closed).
func TestMiniAppIDsRateLimitPerNetwork(t *testing.T) {
	h := newMiniAppIDHarness(t)
	from := func(ip string) func(*http.Request) { return func(r *http.Request) { r.RemoteAddr = ip + ":40000" } }
	for i := 0; i < ratelimit.MiniAppIDLookupLimit; i++ {
		q := "?app=vihat"
		if i%2 == 1 {
			q = "?app=khac" // malformed requests spend the budget too
		}
		if w := h.get(q, from("203.0.113.9")); w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited", i+1)
		}
	}
	w := h.get("?app=vihat", from("203.0.113.9"))
	wantCode(t, w, http.StatusTooManyRequests, "rate_limited")
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After = %q", ra)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("429: Cache-Control = %q", cc)
	}
	wantMiniAppID(t, h.get("?app=vihat", from("203.0.113.10")), "3749147383835819800", "chung")

	h.counter.down = true
	wantCode(t, h.get("?app=vihat", from("203.0.113.11")), http.StatusServiceUnavailable, "rate_limit_unavailable")
}

func TestRegisterMiniAppIDsPanicsOnMissingDependency(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic with a nil limiter")
		}
	}()
	RegisterMiniAppIDs(http.NewServeMux(), MiniAppIDDeps{Shared: &sharedReaderFake{}, Own: &ownResolverFake{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}
