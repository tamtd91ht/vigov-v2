package httpx

// The third outcome of CitizenSessions (01/10/2026): the registry could not be asked. It must be 503
// on every route class — never 401 (the Mini App would discard a session that is still valid) and
// never "no session, carry on" (a KhongThuocXa route would serve a caller whose commune could not be
// checked). Fail closed, never allow.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type registryDown struct{}

func (registryDown) TraCuu(context.Context, string) (CitizenSession, bool, error) { // vi-name-ok: implements the existing CitizenSessions method
	return CitizenSession{}, false, errors.New("identity: unavailable")
}

func TestCitizenEdgeRegistryDownIs503OnEveryRouteClass(t *testing.T) {
	for name, class := range map[string]func(http.Handler) http.Handler{
		"XaTuPhien":                       XaTuPhien(),
		"XaTuPhienChiXem":                 XaTuPhienChiXem("test: view-only"),
		"KhongThuocXa":                    KhongThuocXa("test: commune picker"),
		"CommuneFromSessionOrZaloAccount": CommuneFromSessionOrZaloAccount("test: unverified petition"),
	} {
		t.Run(name, func(t *testing.T) {
			ran := false
			h := CitizenEdge(registryDown{})(class(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				ran = true
				w.WriteHeader(http.StatusOK)
			})))
			w := goiCongDan(h, tokenA)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("code = %d, want 503", w.Code)
			}
			if ran {
				t.Fatal("the handler ran although the session could not be checked")
			}
			if strings.Contains(w.Body.String(), "identity") || strings.Contains(w.Body.String(), tokenA) {
				t.Errorf("the 503 body leaks the cause or the token: %s", w.Body.String())
			}
		})
	}
}

// No bearer, no lookup: an anonymous request is not affected by the registry being down — a
// KhongThuocXa route still serves it, a business route still answers 401.
func TestCitizenEdgeRegistryDownDoesNotTouchAnonymousRequests(t *testing.T) {
	h := CitizenEdge(registryDown{})(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	if w := goiCongDan(h, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request with the registry down: %d, want 401", w.Code)
	}
}
