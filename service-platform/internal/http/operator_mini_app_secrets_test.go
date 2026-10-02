package http

// The two secret routes (PUT/DELETE …/mini-apps/{app_id}/secret) and the automatic retirement after a
// change or a detach (ADR 0070 #4, §"Bổ sung 02/10/2026" #6). 401 / 401 staff token / 403 / 503 / 200
// for both routes are in TestGuardedRoutes; this file defends what is specific to them:
//
//   - the secret reaches identity verbatim and NOWHERE ELSE — not a response, not a log line, not
//     the idempotency store (wired here although the operator edge mounts none, so a future
//     idem.Required on these routes would be caught);
//   - the commune and the App ID come from the PATH; another commune's App ID is the same 404 as an
//     unknown one, decided before identity is called;
//   - every identity answer maps to its status;
//   - a retirement that fails AFTER the binding committed leaves the binding as it is and says so.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/tenant"
)

// Fake material — says so in full (rule 8, forbidden #1).
const appSecretFake = "zalo-app-secret-FAKE-NOT-REAL"

func (f *identityFake) SetMiniAppSecret(ctx context.Context, req operatorclient.SetMiniAppSecretRequest) (operatorclient.SetMiniAppSecretResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, _ := tenant.From(ctx)
	f.sawSet = append(f.sawSet, req)
	f.sawSetTenant = append(f.sawSetTenant, t)
	if f.setErr != nil {
		return operatorclient.SetMiniAppSecretResult{}, f.setErr
	}
	if f.setRes != nil {
		return *f.setRes, nil
	}
	return operatorclient.SetMiniAppSecretResult{Outcome: operatorclient.OutcomeAccepted, Version: operatorclient.MiniAppSecretVersion{
		AppID: req.AppID, Version: "01JDVERSIONFAKE00000000000", SetAt: nowFake, SetBy: opCodeFake}}, nil
}

func (f *identityFake) RetireMiniAppSecret(ctx context.Context, req operatorclient.RetireMiniAppSecretRequest) (operatorclient.RetireMiniAppSecretResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, _ := tenant.From(ctx)
	f.sawRetire = append(f.sawRetire, req)
	f.sawRetireTenant = append(f.sawRetireTenant, t)
	if f.retireErr != nil {
		return operatorclient.RetireMiniAppSecretResult{}, f.retireErr
	}
	if f.retireRes != nil {
		return *f.retireRes, nil
	}
	return operatorclient.RetireMiniAppSecretResult{Outcome: operatorclient.OutcomeAccepted, Retirement: operatorclient.MiniAppSecretRetirement{
		AppID: req.AppID, RetiredVersion: "01JDVERSIONFAKE00000000000", RetiredAt: nowFake, RetiredBy: opCodeFake}}, nil
}

func secretPath(commune, app string) string {
	return "/api/v1/communes/" + commune + "/mini-apps/" + app + "/secret"
}

const secretBodyFake = `{"secret":"` + appSecretFake + `","reason":"  Đặt khoá cho app riêng  "}`

// recordingIdemStore is an idempotency store that remembers every key and value it was handed.
type recordingIdemStore struct {
	mu   sync.Mutex
	seen []string
}

func (s *recordingIdemStore) note(v ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = append(s.seen, v...)
}
func (s *recordingIdemStore) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.note(key)
	return true, nil
}
func (s *recordingIdemStore) Get(_ context.Context, key string) (string, error) {
	s.note(key)
	return "", nil
}
func (s *recordingIdemStore) Complete(_ context.Context, key, value string, _ time.Duration) error {
	s.note(key, value)
	return nil
}
func (s *recordingIdemStore) Release(_ context.Context, key string) error { s.note(key); return nil }

func TestSetSecretForwardsAndAnswersMetadataOnly(t *testing.T) {
	h := miniAppHarness(t)
	st := &recordingIdemStore{}
	h.mux = idem.Middleware(st, nil)(h.mux)
	rec := h.do("PUT", secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, want := range []string{`"app_id":"` + oldAppFake + `"`, `"version":"01JDVERSIONFAKE00000000000"`, `"set_by":"VH-00001"`, `"set_at":"2026-10-01T08:00:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("response lacks %s: %s", want, body)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the secret route's answer must be no-store")
	}
	if len(h.id.sawSet) != 1 {
		t.Fatalf("identity saw %d Set calls, want 1", len(h.id.sawSet))
	}
	got := h.id.sawSet[0]
	if string(got.AppSecret.Lo()) != appSecretFake || got.AppID != oldAppFake || got.Reason != "Đặt khoá cho app riêng" ||
		string(got.Token.Lo()) != opCookie(t) || got.ClientIP == "" {
		t.Errorf("forwarded request wrong: app %q reason %q ip %q", got.AppID, got.Reason, got.ClientIP)
	}
	if h.id.sawSetTenant[0] != communeIDFake {
		t.Errorf("target commune %q, want the path's %q", h.id.sawSetTenant[0], communeIDFake)
	}
	assertNoSecret(t, h, body, st)
}

// assertNoSecret: the value is in no response, no log line, no idempotency key or value.
func assertNoSecret(t *testing.T, h *harness, body string, st *recordingIdemStore) {
	t.Helper()
	if strings.Contains(body, appSecretFake) {
		t.Errorf("the response echoes the secret: %s", body)
	}
	if strings.Contains(h.logs.String(), appSecretFake) {
		t.Errorf("the log carries the secret: %s", h.logs.String())
	}
	if st != nil {
		for _, v := range st.seen {
			if strings.Contains(v, appSecretFake) {
				t.Errorf("the idempotency store was handed the secret")
			}
		}
	}
}

func TestSetSecretRefusedAtTheEdge(t *testing.T) {
	path := secretPath(communeIDFake, oldAppFake)
	for _, c := range []struct {
		name, path, body string
		want             int
		code             string
	}{
		{"commune id malformed", secretPath("nope", oldAppFake), secretBodyFake, 404, "commune_not_found"},
		{"unknown commune", secretPath("01JD8ZQK9M3NPXR7TVWYB2C4EZ", oldAppFake), secretBodyFake, 404, "commune_not_found"},
		{"App ID malformed", secretPath(communeIDFake, "12ab"), secretBodyFake, 404, "mini_app_not_found"},
		// Another commune's, the shared app's, or an unknown App ID: one 404, identity never asked.
		{"App ID not this commune's", secretPath(communeIDFake, "1111111111"), secretBodyFake, 404, "mini_app_not_found"},
		{"body names a commune", path, `{"secret":"` + appSecretFake + `","reason":"x","tenant_id":"01JD8ZQK9M3NPXR7TVWYB2C4EH"}`, 400, "invalid_body"},
		{"secret not a string", path, `{"secret":12345,"reason":"x"}`, 400, "invalid_body"},
		{"secret empty", path, `{"secret":"","reason":"x"}`, 422, "invalid_secret"},
		{"secret absent", path, `{"reason":"x"}`, 422, "invalid_secret"},
		{"secret with a space", path, `{"secret":"FAKE secret","reason":"x"}`, 422, "invalid_secret"},
		{"secret with NUL", path, `{"secret":"FAKE\u0000x","reason":"x"}`, 422, "invalid_secret"},
		{"reason blank", path, `{"secret":"` + appSecretFake + `","reason":"   "}`, 422, "invalid_reason"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			rec := h.do("PUT", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if len(h.id.sawSet) != 0 {
				t.Error("a request refused at the edge reached identity")
			}
			assertNoSecret(t, h, rec.Body.String(), nil)
		})
	}
}

func TestSecretRoutesIdentityAnswerMapping(t *testing.T) {
	for _, c := range []struct {
		name    string
		res     operatorclient.Outcome
		err     error
		wantSet int
		setCode string
		wantRet int
		retCode string
	}{
		{"session ended in between", operatorclient.OutcomeSessionNotLive, nil, 401, "unauthorized", 401, "unauthorized"},
		{"key not granted in identity", operatorclient.OutcomePermissionDenied, nil, 403, "forbidden", 403, "forbidden"},
		// (Retire never gets this one from the real client; the row checks the mapping only.)
		{"binding not live", 0, operatorclient.ErrMiniAppNotBound, 409, "mini_app_not_bound", 409, "mini_app_not_bound"},
		{"shape refused", 0, operatorclient.ErrArgumentRefused, 422, "invalid_secret", 422, "invalid_secret"},
		{"identity down", 0, errors.New("operatorclient: SetMiniAppSecret: unavailable (fake)"), 503, "mini_app_secret_unavailable", 503, "mini_app_secret_unavailable"},
		{"realm not configured", 0, operatorclient.ErrRealmNotConfigured, 503, "mini_app_secret_unavailable", 503, "mini_app_secret_unavailable"},
		{"contract fault", 0, operatorclient.ErrContract, 503, "mini_app_secret_unavailable", 503, "mini_app_secret_unavailable"},
		{"nothing live to retire", 0, operatorclient.ErrNoLiveSecret, 503, "mini_app_secret_unavailable", 200, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			h.id.setErr, h.id.retireErr = c.err, c.err
			if c.err == nil {
				h.id.setRes = &operatorclient.SetMiniAppSecretResult{Outcome: c.res}
				h.id.retireRes = &operatorclient.RetireMiniAppSecretResult{Outcome: c.res}
			}
			rec := h.do("PUT", secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t))
			if rec.Code != c.wantSet || !strings.Contains(rec.Body.String(), `"code":"`+c.setCode+`"`) {
				t.Errorf("PUT: %d %s; want %d %s", rec.Code, rec.Body, c.wantSet, c.setCode)
			}
			assertNoSecret(t, h, rec.Body.String(), nil)
			rec = h.do("DELETE", secretPath(communeIDFake, oldAppFake), `{"reason":"Thu hồi"}`, opCookie(t))
			if rec.Code != c.wantRet || (c.retCode != "" && !strings.Contains(rec.Body.String(), `"code":"`+c.retCode+`"`)) {
				t.Errorf("DELETE: %d %s; want %d %s", rec.Code, rec.Body, c.wantRet, c.retCode)
			}
			if c.res == operatorclient.OutcomeSessionNotLive && !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
				t.Error("a session identity calls dead must clear the cookie")
			}
		})
	}
}

func TestRetireSecretAnswers(t *testing.T) {
	h := miniAppHarness(t)
	rec := h.do("DELETE", secretPath(communeIDFake, newAppFake), `{"reason":"  Thu hồi khoá cũ  "}`, opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"retired":true`) ||
		!strings.Contains(rec.Body.String(), `"retired_by":"VH-00001"`) {
		t.Fatalf("retired: %d %s", rec.Code, rec.Body)
	}
	// A turned-off App ID of this commune is retirable — that is when it is needed.
	if r := h.id.sawRetire[0]; r.AppID != newAppFake || r.Reason != "Thu hồi khoá cũ" || h.id.sawRetireTenant[0] != communeIDFake {
		t.Errorf("forwarded %+v to %q", r, h.id.sawRetireTenant[0])
	}

	h = miniAppHarness(t)
	h.id.retireErr = operatorclient.ErrNoLiveSecret
	rec = h.do("DELETE", secretPath(communeIDFake, oldAppFake), `{"reason":"x"}`, opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"retired":false`) || strings.Contains(rec.Body.String(), "retired_at") {
		t.Fatalf("nothing live: %d %s — want 200 retired=false", rec.Code, rec.Body)
	}

	for _, c := range []struct {
		name, path, body string
		want             int
		code             string
	}{
		{"reason absent", secretPath(communeIDFake, oldAppFake), `{}`, 422, "invalid_reason"},
		{"no body", secretPath(communeIDFake, oldAppFake), ``, 400, "invalid_body"},
		{"App ID not this commune's", secretPath(communeIDFake, "1111111111"), `{"reason":"x"}`, 404, "mini_app_not_found"},
	} {
		h := miniAppHarness(t)
		if rec := h.do("DELETE", c.path, c.body, opCookie(t)); rec.Code != c.want || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s: %d %s; want %d %s", c.name, rec.Code, rec.Body, c.want, c.code)
		}
		if len(h.id.sawRetire) != 0 {
			t.Errorf("%s: reached identity", c.name)
		}
	}
}

// ops.tenant.manage, which edits the commune, does not reach the secret routes.
func TestSecretRoutesNeedTheMiniAppKey(t *testing.T) {
	for _, m := range []string{"PUT", "DELETE"} {
		h := newHarness(t)
		h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage"}
		if rec := h.do(m, secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t)); rec.Code != 403 {
			t.Errorf("%s with the other keys: %d, want 403", m, rec.Code)
		}
		if len(h.id.sawSet)+len(h.id.sawRetire) != 0 {
			t.Errorf("%s reached identity without its key", m)
		}
	}
}

// --- the automatic retirement after a change or a detach ----------------------------------------

func TestReplacementRetiresTheOldSecretAfterTheBinding(t *testing.T) {
	body := `{"new_app_id":"` + newAppFake + `","reason":"Xã đổi App ID"}`
	for _, c := range []struct {
		name    string
		err     error
		res     *operatorclient.RetireMiniAppSecretResult
		retired bool
		problem string
	}{
		{"retired", nil, nil, true, ""},
		{"nothing was live", operatorclient.ErrNoLiveSecret, nil, true, ""},
		{"identity down", errors.New("unavailable (fake)"), nil, false, "identity_unavailable"},
		{"session ended", nil, &operatorclient.RetireMiniAppSecretResult{Outcome: operatorclient.OutcomeSessionNotLive}, false, "session_not_live"},
		{"key not granted in identity", nil, &operatorclient.RetireMiniAppSecretResult{Outcome: operatorclient.OutcomePermissionDenied}, false, "forbidden"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			h.id.retireErr, h.id.retireRes = c.err, c.res
			rec := h.do("POST", replacementPath(communeIDFake, oldAppFake), body, opCookie(t))
			// The binding change stands whatever the retirement did: 201, and the writer ran.
			if rec.Code != 201 || h.w.lastNewApp != newAppFake {
				t.Fatalf("status %d (%s); the binding must be kept", rec.Code, rec.Body)
			}
			if len(h.id.sawRetire) != 1 || h.id.sawRetire[0].AppID != oldAppFake ||
				h.id.sawRetire[0].Reason != "Xã đổi App ID" || h.id.sawRetireTenant[0] != communeIDFake {
				t.Fatalf("retirement call %+v tenants %v", h.id.sawRetire, h.id.sawRetireTenant)
			}
			got := rec.Body.String()
			if c.retired && (!strings.Contains(got, `"secret_retired":true`) || strings.Contains(got, "secret_retirement_error")) {
				t.Errorf("want secret_retired true: %s", got)
			}
			if !c.retired && (!strings.Contains(got, `"secret_retired":false`) ||
				!strings.Contains(got, `"secret_retirement_error":"`+c.problem+`"`)) {
				t.Errorf("want secret_retired false / %s: %s", c.problem, got)
			}
			if !c.retired && !strings.Contains(h.logs.String(), "mini_app.secret_retire_pending") {
				t.Error("a pending retirement must leave a warning naming it")
			}
		})
	}
}

func TestReplacementRefusedRetiresNothing(t *testing.T) {
	h := miniAppHarness(t)
	h.w.err = errors.New("store down (fake)")
	h.do("POST", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"`+newAppFake+`","reason":"x"}`, opCookie(t))
	if len(h.id.sawRetire) != 0 {
		t.Fatal("a binding change that did not commit must not retire anything")
	}
}

func TestDetachRetiresReactivateDoesNot(t *testing.T) {
	h := miniAppHarness(t)
	h.id.retireErr = errors.New("unavailable (fake)")
	rec := h.do("PUT", miniAppActivationPath(communeIDFake, oldAppFake), `{"active":false,"reason":"Xã ngừng dùng app riêng"}`, opCookie(t))
	if rec.Code != 200 || h.w.lastState == nil || *h.w.lastState {
		t.Fatalf("detach: %d %s", rec.Code, rec.Body)
	}
	if len(h.id.sawRetire) != 1 || h.id.sawRetire[0].AppID != oldAppFake ||
		!strings.Contains(rec.Body.String(), `"secret_retired":false`) {
		t.Fatalf("detach must try to retire and report the failure: %+v %s", h.id.sawRetire, rec.Body)
	}

	h = miniAppHarness(t)
	rec = h.do("PUT", miniAppActivationPath(communeIDFake, newAppFake), `{"active":true,"reason":"Gỡ nhầm, bật lại"}`, opCookie(t))
	if rec.Code != 200 || len(h.id.sawRetire) != 0 || strings.Contains(rec.Body.String(), "secret_retired") {
		t.Fatalf("reactivate: %d %s, retire calls %d — nothing to retire", rec.Code, rec.Body, len(h.id.sawRetire))
	}
}
