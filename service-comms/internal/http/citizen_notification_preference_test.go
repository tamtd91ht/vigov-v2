package http

// The citizen switch over the REAL citizen edge (httpx.CitizenEdge + authz.CitizenPrincipal, the order
// cmd/server mounts them): identity and commune come from the SESSION only. Rule 4 replaces RBAC on this
// surface (rule 5, invariant 6), so the four cases are: no session 401 · a session with no commune 401 ·
// a session with no verified phone 403 `chua_xac_thuc_so` · the caller's own switch 200 — read and
// written in the SESSION's commune for the SESSION's citizen, whatever the body or query name.

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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const (
	citizenTokenOK        = "tok-citizen"
	citizenTokenNoCommune = "tok-no-commune"
	citizenTokenNoPhone   = "tok-no-phone"
	citizenIDTest         = "01JCITIZENTEST000000000001"
)

// fakeCitizenSessions is the registry: three tokens, three kinds of session.
type fakeCitizenSessions struct{ commune tenant.ID }

func (f fakeCitizenSessions) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool, error) { // vi-name-ok: implements the existing httpx.CitizenSessions method (rule 12 inv 3)
	switch token {
	case citizenTokenOK:
		return httpx.CitizenSession{ID: "sid-1", CitizenID: citizenIDTest, TenantID: f.commune}, true, nil
	case citizenTokenNoCommune:
		return httpx.CitizenSession{ID: "sid-2", CitizenID: citizenIDTest}, true, nil
	case citizenTokenNoPhone:
		return httpx.CitizenSession{ID: "sid-3", TenantID: f.commune, ZaloAccountID: "01JZALOACCOUNT000000000001"}, true, nil
	}
	return httpx.CitizenSession{}, false, nil
}

// fakePreferences records who asked, in which commune, and what was set.
type fakePreferences struct {
	stored  map[string]bool
	actor   audit.Actor
	commune tenant.ID
	sets    int
	lastSet bool
	err     error
}

func (f *fakePreferences) Get(ctx context.Context, actor audit.Actor) (domain.CitizenNotificationPreference, error) {
	f.actor, f.commune = actor, tenant.MustFrom(ctx)
	if f.err != nil {
		return domain.CitizenNotificationPreference{}, f.err
	}
	if v, ok := f.stored[actor.ID]; ok {
		return domain.CitizenNotificationPreference{ZaloEnabled: v, Stored: true}, nil
	}
	return domain.DefaultCitizenNotificationPreference(), nil
}

func (f *fakePreferences) Set(ctx context.Context, on bool, actor audit.Actor) (domain.CitizenNotificationPreference, error) {
	f.actor, f.commune = actor, tenant.MustFrom(ctx)
	f.sets++
	f.lastSet = on
	if f.err != nil {
		return domain.CitizenNotificationPreference{}, f.err
	}
	return domain.CitizenNotificationPreference{ZaloEnabled: on, Stored: true}, nil
}

func citizenServer(t *testing.T, prefs *fakePreferences) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCitizen(mux, CitizenDeps{Preferences: prefs, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var h http.Handler = mux
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(fakeCitizenSessions{commune: xaA})(h)
	return h
}

func citizenCall(h http.Handler, method, token, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, MyNotificationPreferencesPath, rd)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMyNotificationPreferenceRefusals(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, c := range []struct {
			name, token string
			want        int
			code        string
		}{
			{"no session", "", http.StatusUnauthorized, "unauthorized"},
			{"unknown token", "tok-unknown", http.StatusUnauthorized, "unauthorized"},
			{"session without a commune", citizenTokenNoCommune, http.StatusUnauthorized, ""},
			{"session without a verified phone", citizenTokenNoPhone, http.StatusForbidden, "chua_xac_thuc_so"},
		} {
			prefs := &fakePreferences{}
			w := citizenCall(citizenServer(t, prefs), method, c.token, `{"zalo_enabled":false}`)
			if w.Code != c.want {
				t.Errorf("%s %s: status %d, want %d (%s)", method, c.name, w.Code, c.want, w.Body.String())
			}
			if c.code != "" && !strings.Contains(w.Body.String(), `"code":"`+c.code+`"`) {
				t.Errorf("%s %s: body %s", method, c.name, w.Body.String())
			}
			if prefs.sets != 0 || prefs.actor.ID != "" {
				t.Errorf("%s %s: the use case was reached", method, c.name)
			}
		}
	}
}

func TestMyNotificationPreferenceAbsentReadsEnabled(t *testing.T) {
	prefs := &fakePreferences{}
	w := citizenCall(citizenServer(t, prefs), http.MethodGet, citizenTokenOK, "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"zalo_enabled":true}` {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if prefs.actor.ID != citizenIDTest || prefs.actor.Kind != "citizen" || prefs.commune != xaA {
		t.Errorf("read for %+v in %q — must be the session's citizen and commune", prefs.actor, prefs.commune)
	}
}

func TestMyNotificationPreferencePutUsesTheSessionOnly(t *testing.T) {
	prefs := &fakePreferences{}
	// A body naming another citizen and commune, a query and a commune header: all ignored (rule 1
	// forbidden #2, rule 4 invariant 2). The decoder drops unknown keys; the header is stripped by the edge.
	r := httptest.NewRequest(http.MethodPut, MyNotificationPreferencesPath+"?tenant_id=other",
		strings.NewReader(`{"zalo_enabled":false,"citizen_id":"someone-else","tenant_id":"01JOTHER"}`))
	r.Header.Set("Authorization", "Bearer "+citizenTokenOK)
	r.Header.Set("X-Tenant-ID", "01JOTHER")
	w := httptest.NewRecorder()
	citizenServer(t, prefs).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["zalo_enabled"] != false || prefs.lastSet != false || prefs.sets != 1 {
		t.Errorf("set %v (%d calls), reply %v", prefs.lastSet, prefs.sets, out)
	}
	if prefs.actor.ID != citizenIDTest || prefs.actor.Kind != "citizen" || prefs.actor.IP == "" || prefs.commune != xaA {
		t.Errorf("written as %+v in %q — must be the session's citizen, with an IP, in the session's commune",
			prefs.actor, prefs.commune)
	}
}

func TestMyNotificationPreferencePutNeedsTheBoolean(t *testing.T) {
	for _, body := range []string{`{}`, `{"zalo_enabled":null}`, `not json`, `{"zalo_enabled":"false"}`} {
		prefs := &fakePreferences{}
		w := citizenCall(citizenServer(t, prefs), http.MethodPut, citizenTokenOK, body)
		if w.Code != http.StatusBadRequest || prefs.sets != 0 {
			t.Errorf("%s: status %d, sets %d — an absent value must never become a silent false", body, w.Code, prefs.sets)
		}
	}
}

func TestMyNotificationPreferenceStoreFailureIs500WithoutTheCitizen(t *testing.T) {
	prefs := &fakePreferences{err: errors.New("db down")}
	w := citizenCall(citizenServer(t, prefs), http.MethodGet, citizenTokenOK, "")
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), citizenIDTest) {
		t.Errorf("status %d body %s", w.Code, w.Body.String())
	}
}
