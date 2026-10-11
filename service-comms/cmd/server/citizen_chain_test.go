package main

// THE CITIZEN EDGE OF THIS BINARY (ADR 0091 §4) — the wiring, not the middleware (core/httpx and
// core/authz prove those). What only this file can see: that dungBien routes
// /api/v1/my-notification-preferences to the CITIZEN chain on the reserved API host — session from the
// bearer token, commune from the session, CORS for the Mini App — and that no staff path falls onto it.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

const citizenTokenChain = "tok-citizen-chain"

type citizenSessionsChain struct{}

func (citizenSessionsChain) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool, error) { // vi-name-ok: implements the existing httpx.CitizenSessions method (rule 12 inv 3)
	if token != citizenTokenChain {
		return httpx.CitizenSession{}, false, nil
	}
	return httpx.CitizenSession{ID: "sid", CitizenID: "01JCITIZENCHAIN00000000001", TenantID: xaA}, true, nil
}

// preferencesChain answers in the commune of the context, asserting there is one.
type preferencesChain struct{}

func (preferencesChain) Get(ctx context.Context, _ audit.Actor) (domain.CitizenNotificationPreference, error) {
	tenant.MustFrom(ctx)
	return domain.DefaultCitizenNotificationPreference(), nil
}

func (preferencesChain) Set(ctx context.Context, on bool, _ audit.Actor) (domain.CitizenNotificationPreference, error) {
	tenant.MustFrom(ctx)
	return domain.CitizenNotificationPreference{ZaloEnabled: on, Stored: true}, nil
}

// newCitizenEdgeForTest is the real citizen chain around the real citizen routes, with a fake registry
// and use case.
func newCitizenEdgeForTest(t *testing.T) http.Handler {
	t.Helper()
	origins, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	mux := http.NewServeMux()
	svchttp.RegisterCitizen(mux, svchttp.CitizenDeps{Preferences: preferencesChain{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return buildCitizenEdge(mux, citizenSessionsChain{}, nil, origins, slog.New(slog.DiscardHandler))
}

func callCitizen(h http.Handler, method, token, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+hostApiComms+svchttp.MyNotificationPreferencesPath, nil)
	r.Host = hostApiComms
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
		if method == http.MethodOptions {
			r.Header.Set("Access-Control-Request-Method", "PUT")
			r.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMyNotificationPreferencesOnCitizenChain(t *testing.T) {
	h, cb := ngoaiThu(t)
	w := callCitizen(h, http.MethodGet, citizenTokenChain, nguonMiniAppThu)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"zalo_enabled":true}` {
		t.Fatalf("on the reserved API host with a session: %d %s", w.Code, w.Body.String())
	}
	if cb.goi != 0 {
		t.Fatal("the staff chain was reached")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != nguonMiniAppThu {
		t.Fatalf("no ACAO for the Mini App: %v", w.Header())
	}
}

func TestMyNotificationPreferencesWithoutSessionIs401Not404(t *testing.T) {
	// 404 here would mean the path fell to the staff chain (TenantMiddleware, reserved host).
	h, _ := ngoaiThu(t)
	if w := callCitizen(h, http.MethodGet, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d, want 401 from the citizen chain — body %s", w.Code, w.Body.String())
	}
}

func TestMyNotificationPreferencesPreflight(t *testing.T) {
	h, _ := ngoaiThu(t)
	w := callCitizen(h, http.MethodOptions, "", "https://mini.zalo.me")
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://mini.zalo.me" {
		t.Fatalf("preflight: %d ACAO=%q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
	// The preflight asks for PUT (callCitizen): a 204 with an ACAO but without PUT in the allowed methods
	// is a preflight the browser REFUSES — the Mini App's toggle would fail as a "network error" while the
	// two lines above stay green. This is the line that was missing when PUT was absent from core/httpx.
	if !allowsMethod(w.Header().Get("Access-Control-Allow-Methods"), http.MethodPut) {
		t.Fatalf("preflight for PUT: Access-Control-Allow-Methods = %q, PUT absent — the browser blocks the save",
			w.Header().Get("Access-Control-Allow-Methods"))
	}
}

// allowsMethod reads Access-Control-Allow-Methods the way a browser does: a comma-separated token list.
func allowsMethod(header, method string) bool {
	for _, m := range strings.Split(header, ",") {
		if strings.TrimSpace(m) == method {
			return true
		}
	}
	return false
}

func TestNoStaffPathFallsOntoTheCitizenChain(t *testing.T) {
	for _, p := range []string{svchttp.MyNotificationPreferencesPath + "/x", "/api/v1/notification-preferences",
		svchttp.MyNotificationPreferencesPath + "x"} {
		h, _ := ngoaiThu(t)
		if w := goiApi(h, http.MethodGet, p, ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404 from the staff chain's TenantMiddleware", p, w.Code)
		}
	}
}
