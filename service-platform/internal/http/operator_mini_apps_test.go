package http

// The ADR 0070 mini-app routes: replacement and removal ("Gỡ khỏi xã", a soft delete since
// 05/10/2026 — reactivation withdrawn) of a commune's dedicated App ID, plus the attach refusals. 401 / 401 staff token / 403 / 503 /
// 2xx for both routes are in TestGuardedRoutes; this file defends what is specific to them — the
// commune AND the old App ID come from the PATH, the reason is mandatory, every refusal of the store
// keeps its own status, and an App ID another commune holds is the same 404 as an unknown one.

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/store"
)

const (
	oldAppFake = "3291993990104489440"
	newAppFake = "3043188591857102858"
)

func replacementPath(commune, app string) string {
	return "/api/v1/communes/" + commune + "/mini-apps/" + app + "/replacement"
}

func miniAppActivationPath(commune, app string) string {
	return "/api/v1/communes/" + commune + "/mini-apps/" + app + "/activation"
}

func miniAppPublicationPath(commune, app string) string {
	return "/api/v1/communes/" + commune + "/mini-apps/" + app + "/publication"
}

func miniAppHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.id.keys = []string{"ops.mini_app.manage"}
	return h
}

func TestReplacementTakesCommuneAndOldAppFromPath(t *testing.T) {
	h := miniAppHarness(t)
	body := `{"new_app_id":"` + newAppFake + `","reason":"Xã đổi App ID","tenant_id":"01JD8ZQK9M3NPXR7TVWYB2C4EH"}`
	if rec := h.do("POST", replacementPath(communeIDFake, oldAppFake), body, opCookie(t)); rec.Code != 400 {
		t.Fatalf("a body naming a commune must be refused as an unknown field: %d", rec.Code)
	}
	body = `{"new_app_id":"` + newAppFake + `","reason":"  Xã đổi App ID  "}`
	rec := h.do("POST", replacementPath(communeIDFake, oldAppFake), body, opCookie(t))
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"mini_apps"`) {
		t.Fatalf("status %d (%s), want 201 with the commune detail", rec.Code, rec.Body)
	}
	if h.w.target != communeIDFake || h.w.lastApp != oldAppFake || h.w.lastNewApp != newAppFake ||
		h.w.actor.Code != opCodeFake || h.w.actor.IP == "" {
		t.Errorf("target %q old %q new %q actor %+v", h.w.target, h.w.lastApp, h.w.lastNewApp, h.w.actor)
	}
	if h.w.lastReason != "Xã đổi App ID" {
		t.Errorf("reason reached the store as %q, want it validated and trimmed", h.w.lastReason)
	}
}

func TestReplacementRefusals(t *testing.T) {
	good := `{"new_app_id":"` + newAppFake + `","reason":"Xã đổi App ID"}`
	for _, c := range []struct {
		name, path, body string
		storeErr         error
		want             int
		code             string
	}{
		{"commune id malformed", replacementPath("not-a-ulid", oldAppFake), good, nil, 404, "commune_not_found"},
		{"old App ID malformed", replacementPath(communeIDFake, "abc"), good, nil, 404, "mini_app_not_found"},
		{"new App ID malformed", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"12ab","reason":"x"}`, nil, 422, "invalid_app_id"},
		{"reason missing", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"` + newAppFake + `"}`, nil, 422, "invalid_reason"},
		{"reason blank", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"` + newAppFake + `","reason":"   "}`, nil, 422, "invalid_reason"},
		{"unknown commune", replacementPath(communeIDFake, oldAppFake), good, store.ErrCommuneNotFound, 404, "commune_not_found"},
		{"old App ID not this commune's", replacementPath(communeIDFake, oldAppFake), good, store.ErrMiniAppNotInCommune, 404, "mini_app_not_found"},
		{"old App ID already off", replacementPath(communeIDFake, oldAppFake), good, store.ErrMiniAppInactive, 409, "mini_app_inactive"},
		{"new App ID has a row", replacementPath(communeIDFake, oldAppFake), good, store.ErrMiniAppTaken, 409, "mini_app_taken"},
		{"commune inactive", replacementPath(communeIDFake, oldAppFake), good, store.ErrCommuneInactive, 409, "commune_inactive"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			h.w.err = c.storeErr
			rec := h.do("POST", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if c.storeErr == nil && h.w.target != "" {
				t.Error("a request refused at the edge reached the writer")
			}
		})
	}
}

func TestMiniAppRemovalTakesCommuneAndAppFromPath(t *testing.T) {
	h := miniAppHarness(t)
	rec := h.do("PUT", miniAppActivationPath(communeIDFake, oldAppFake), `{"active":false,"reason":"  Xã ngừng dùng app riêng  "}`, opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"mini_apps"`) {
		t.Fatalf("%d (%s)", rec.Code, rec.Body)
	}
	if h.w.target != communeIDFake || h.w.lastApp != oldAppFake || h.w.lastReason != "Xã ngừng dùng app riêng" ||
		h.w.actor.Code != opCodeFake || h.w.actor.IP == "" {
		t.Errorf("target %q app %q reason %q actor %+v", h.w.target, h.w.lastApp, h.w.lastReason, h.w.actor)
	}
}

func TestMiniAppRemovalRefusals(t *testing.T) {
	path := miniAppActivationPath(communeIDFake, oldAppFake)
	off := `{"active":false,"reason":"Gỡ"}`
	for _, c := range []struct {
		name, path, body string
		storeErr         error
		want             int
		code             string
	}{
		{"active absent", path, `{"reason":"x"}`, nil, 400, "invalid_body"},
		// Reactivation was withdrawn (ADR 0070 §Sửa đổi 05/10/2026 #2) — refused whatever the reason.
		{"reactivation", path, `{"active":true,"reason":"Bật lại"}`, nil, 422, "mini_app_reactivation_removed"},
		{"reactivation without reason", path, `{"active":true}`, nil, 422, "mini_app_reactivation_removed"},
		{"reason blank", path, `{"active":false,"reason":"  "}`, nil, 422, "invalid_reason"},
		{"App ID malformed", miniAppActivationPath(communeIDFake, "x"), off, nil, 404, "mini_app_not_found"},
		{"commune id malformed", miniAppActivationPath("nope", oldAppFake), off, nil, 404, "commune_not_found"},
		// Another commune's App ID: the store answers ErrMiniAppNotInCommune — the SAME 404 as unknown.
		{"App ID of another commune", path, off, store.ErrMiniAppNotInCommune, 404, "mini_app_not_found"},
		{"commune inactive", path, off, store.ErrCommuneInactive, 409, "commune_inactive"},
		{"unknown commune", path, off, store.ErrCommuneNotFound, 404, "commune_not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			h.w.err = c.storeErr
			rec := h.do("PUT", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if c.storeErr == nil && h.w.target != "" {
				t.Error("a request refused at the edge reached the writer")
			}
		})
	}
}

// An App ID removed earlier can never be bound again — the operator is told why, in words.
func TestAttachOrReplaceWithRemovedAppID(t *testing.T) {
	h := miniAppHarness(t)
	h.w.err = store.ErrMiniAppRemoved
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/api/v1/communes/" + communeIDFake + "/mini-apps", `{"app_id":"` + newAppFake + `"}`},
		{"POST", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"` + newAppFake + `","reason":"Đổi"}`},
	} {
		rec := h.do(c.method, c.path, c.body, opCookie(t))
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"mini_app_removed"`) ||
			!strings.Contains(rec.Body.String(), "không gắn lại được") {
			t.Errorf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}
}

// ADR 0070 #1: attaching beside a running dedicated app is refused — the console must replace.
func TestAttachRefusedWhileAnotherAppRuns(t *testing.T) {
	h := miniAppHarness(t)
	h.w.err = store.ErrMiniAppAlreadyRunning
	rec := h.do("POST", "/api/v1/communes/"+communeIDFake+"/mini-apps", `{"app_id":"`+newAppFake+`"}`, opCookie(t))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"mini_app_already_running"`) {
		t.Fatalf("status %d body %s; want 409 mini_app_already_running", rec.Code, rec.Body)
	}
}

// The mini-app routes are guarded by ops.mini_app.manage ALONE: ops.tenant.manage, which reads and
// edits the commune, does not reach them.
func TestMiniAppRoutesNeedTheMiniAppKey(t *testing.T) {
	for _, rt := range []struct{ method, path, body string }{
		{"POST", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"` + newAppFake + `","reason":"x"}`},
		{"PUT", miniAppActivationPath(communeIDFake, oldAppFake), `{"active":false,"reason":"x"}`},
		{"PUT", miniAppPublicationPath(communeIDFake, oldAppFake), `{"published":true,"reason":"x"}`},
	} {
		h := newHarness(t)
		h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage"}
		if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 403 {
			t.Errorf("%s %s with the other keys: %d, want 403", rt.method, rt.path, rec.Code)
		}
		if h.w.target != "" {
			t.Errorf("%s %s reached the writer without its key", rt.method, rt.path)
		}
	}
}

// The "published on Zalo" mark (ADR 0070 bổ sung 13/10/2026): commune AND App ID from the PATH, the
// reason validated and trimmed, the actor the operator's business code, the answer the commune detail
// carrying the mark. Both directions reach the store.
func TestMiniAppPublicationTakesCommuneAndAppFromPath(t *testing.T) {
	for _, published := range []bool{true, false} {
		h := miniAppHarness(t)
		body := `{"published":` + map[bool]string{true: "true", false: "false"}[published] + `,"reason":"  Zalo đã phát hành app  "}`
		rec := h.do("PUT", miniAppPublicationPath(communeIDFake, oldAppFake), body, opCookie(t))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"mini_apps"`) ||
			!strings.Contains(rec.Body.String(), `"published_at":"2026-10-01T08:00:00Z"`) ||
			!strings.Contains(rec.Body.String(), `"published_by":"`+opCodeFake+`"`) {
			t.Fatalf("published=%v: %d (%s)", published, rec.Code, rec.Body)
		}
		if h.w.target != communeIDFake || h.w.lastApp != oldAppFake || h.w.lastReason != "Zalo đã phát hành app" ||
			h.w.lastPublished == nil || *h.w.lastPublished != published ||
			h.w.actor.Code != opCodeFake || h.w.actor.IP == "" {
			t.Errorf("published=%v: target %q app %q reason %q state %v actor %+v", published,
				h.w.target, h.w.lastApp, h.w.lastReason, h.w.lastPublished, h.w.actor)
		}
	}
}

// An unmarked row reads as null/null in the detail — never "", never a zero time.
func TestDetailUnpublishedMiniAppIsNull(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	rec := h.do("GET", "/api/v1/communes/"+communeIDFake, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// registryFake: 3043188591857102858 is unmarked.
	if !strings.Contains(rec.Body.String(), `"app_id":"3043188591857102858","mode":"rieng","active":false,"created_at":"0001-01-01T00:00:00Z","created_by":"`+opCodeFake+`","published_at":null,"published_by":null`) {
		t.Errorf("unmarked app must carry published_at/by null: %s", rec.Body)
	}
}

func TestMiniAppPublicationRefusals(t *testing.T) {
	path := miniAppPublicationPath(communeIDFake, oldAppFake)
	on := `{"published":true,"reason":"Phát hành"}`
	for _, c := range []struct {
		name, path, body string
		storeErr         error
		want             int
		code             string
	}{
		{"published absent", path, `{"reason":"x"}`, nil, 400, "invalid_body"},
		{"published not a bool", path, `{"published":"yes","reason":"x"}`, nil, 400, "invalid_body"},
		{"commune in body", path, `{"published":true,"reason":"x","tenant_id":"` + communeIDFake + `"}`, nil, 400, "invalid_body"},
		{"reason missing", path, `{"published":true}`, nil, 422, "invalid_reason"},
		{"reason blank", path, `{"published":false,"reason":"   "}`, nil, 422, "invalid_reason"},
		{"App ID malformed", miniAppPublicationPath(communeIDFake, "x"), on, nil, 404, "mini_app_not_found"},
		{"commune id malformed", miniAppPublicationPath("nope", oldAppFake), on, nil, 404, "commune_not_found"},
		// Unknown, removed, another commune's and the shared `chinh` row: ONE answer (store comment).
		{"App ID not this commune's own app", path, on, store.ErrMiniAppNotInCommune, 404, "mini_app_not_found"},
		{"switched-off row", path, on, store.ErrMiniAppInactive, 409, "mini_app_inactive"},
		{"commune inactive", path, on, store.ErrCommuneInactive, 409, "commune_inactive"},
		{"unknown commune", path, on, store.ErrCommuneNotFound, 404, "commune_not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := miniAppHarness(t)
			h.w.err = c.storeErr
			rec := h.do("PUT", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if c.storeErr == nil && h.w.publications != 0 {
				t.Error("a request refused at the edge reached the writer")
			}
		})
	}
}
