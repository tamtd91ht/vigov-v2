package http

// The shared Mini App and the commune QR link (owner 04/10/2026). 401 / 401 staff token / 403 / 503 /
// 2xx for every route are in TestGuardedRoutes; this file defends what is specific to each.

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const sharedAppIDFake = "1234567890123456789"

type sharedAppFake struct {
	app         *domain.SharedMiniApp
	appErr      error
	host        string
	active      bool
	hostErr     error
	declareErr  error
	declared    int
	launchReads int
	sawApp      string
	sawReason   string
	sawActor    domain.OperatorActor
}

func newSharedAppFake() *sharedAppFake {
	return &sharedAppFake{
		app:    &domain.SharedMiniApp{AppID: sharedAppIDFake, CreatedAt: nowFake, CreatedBy: opCodeFake},
		host:   "thangbinh-danang.vigov.vn",
		active: true,
	}
}

func (f *sharedAppFake) SharedMiniApp(context.Context) (domain.SharedMiniApp, error) {
	if f.appErr != nil {
		return domain.SharedMiniApp{}, f.appErr
	}
	if f.app == nil {
		return domain.SharedMiniApp{}, store.ErrNoSharedMiniApp
	}
	return *f.app, nil
}

func (f *sharedAppFake) DeclareSharedMiniApp(_ context.Context, appID, reason string, by domain.OperatorActor) (domain.SharedMiniApp, bool, error) {
	f.declared++
	f.sawApp, f.sawReason, f.sawActor = appID, reason, by
	if f.declareErr != nil {
		return domain.SharedMiniApp{}, false, f.declareErr
	}
	return domain.SharedMiniApp{AppID: appID, CreatedAt: nowFake, CreatedBy: by.Code}, true, nil
}

func (f *sharedAppFake) CommuneLaunchHost(_ context.Context, id string) (string, bool, error) {
	f.launchReads++
	if id != communeIDFake {
		return "", false, store.ErrCommuneNotFound
	}
	return f.host, f.active, f.hostErr
}

const launchPath = "/api/v1/communes/" + communeIDFake + "/mini-app-launch-link"

// citizenAppAccepts is citizen-app/src/lib/launch-params.ts thamSoXa, transcribed: `src` must be one
// of NGUON_CHON_SAN, `d` (lower-cased) must pass laTenMien. If this transcription and the TypeScript
// drift, the TS file is the authority — update both together.
func citizenAppAccepts(query string) (host string, ok bool) {
	label := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	p, err := url.ParseQuery(query)
	if err != nil {
		return "", false
	}
	if src := p.Get("src"); src != "qr" && src != "zns" {
		return "", false
	}
	d := strings.ToLower(p.Get("d"))
	if d == "" || len(d) > 253 {
		return "", false
	}
	parts := strings.Split(d, ".")
	if len(parts) < 2 {
		return "", false
	}
	for _, l := range parts {
		if !label.MatchString(l) {
			return "", false
		}
	}
	return d, true
}

// The link is the published-app form, names the commune's PRIMARY host, and round-trips through the
// citizen app's own parameter reader (ADR 0047: `d` + `src=qr`).
func TestLaunchLinkRoundTripsThroughCitizenApp(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	rec := h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	var got launchLinkView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := "https://zalo.me/s/" + sharedAppIDFake + "/?d=thangbinh-danang.vigov.vn&src=qr"
	if got.URL != want || got.Domain != "thangbinh-danang.vigov.vn" || got.AppID != sharedAppIDFake {
		t.Fatalf("got %+v, want url %s", got, want)
	}
	u, err := url.Parse(got.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "zalo.me" || u.Path != "/s/"+sharedAppIDFake+"/" {
		t.Errorf("not the published Zalo Mini App link: %s", got.URL)
	}
	if strings.Contains(u.RawQuery, "env=") || strings.Contains(u.RawQuery, "version=") {
		t.Error("a QR must use the published-app link, never a TESTING build (owner 04/10/2026)")
	}
	if host, ok := citizenAppAccepts(u.RawQuery); !ok || host != got.Domain {
		t.Errorf("citizen-app would not read the commune from %q (got %q, %v)", u.RawQuery, host, ok)
	}
}

func TestLaunchLinkRefusals(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(f *sharedAppFake)
		path  string
		want  int
		code  string
	}{
		{"no shared app", func(f *sharedAppFake) { f.app = nil }, launchPath, 409, "shared_mini_app_not_declared"},
		{"two shared apps", func(f *sharedAppFake) { f.appErr = store.ErrSharedMiniAppAmbiguous }, launchPath, 409, "shared_mini_app_ambiguous"},
		{"no primary domain", func(f *sharedAppFake) { f.host, f.hostErr = "", store.ErrCommuneNoPrimaryHost }, launchPath, 409, "commune_no_primary_domain"},
		{"inactive commune", func(f *sharedAppFake) { f.active = false }, launchPath, 409, "commune_inactive"},
		{"inactive with no primary", func(f *sharedAppFake) { f.active, f.host, f.hostErr = false, "", store.ErrCommuneNoPrimaryHost },
			launchPath, 409, "commune_inactive"},
		{"unparseable stored host", func(f *sharedAppFake) { f.host = "Bad Host" }, launchPath, 409, "commune_no_primary_domain"},
		{"unknown commune", func(*sharedAppFake) {}, "/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EZ/mini-app-launch-link", 404, "commune_not_found"},
		{"malformed id", func(*sharedAppFake) {}, "/api/v1/communes/not-a-ulid/mini-app-launch-link", 404, "commune_not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.qr.issue"}
			c.setup(h.shared)
			rec := h.do("GET", c.path, "", opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
		})
	}
}

// Issuing a QR writes nothing (ADR 0048 §30/09 #9): no declaration, no registry write.
func TestLaunchLinkWritesNothing(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	if rec := h.do("GET", launchPath, "", opCookie(t)); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if h.shared.declared != 0 || h.w.target != "" || h.oplog.reads != 0 {
		t.Errorf("issuing a QR wrote something: declared %d target %q", h.shared.declared, h.w.target)
	}
}

func TestGetSharedMiniApp(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"} // any one key reads (ADR 0073 #1)
	rec := h.do("GET", "/api/v1/shared-mini-app", "", opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"app_id":"`+sharedAppIDFake+`"`) {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	h.shared.app = nil
	rec = h.do("GET", "/api/v1/shared-mini-app", "", opCookie(t))
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "shared_mini_app_not_declared") {
		t.Fatalf("undeclared: %d (%s), want 404", rec.Code, rec.Body)
	}
}

func TestDeclareSharedMiniApp(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.mini_app.manage"}
	path := "/api/v1/shared-mini-app"
	for _, c := range []struct {
		name, body string
		want       int
		code       string
	}{
		{"absent reason", `{"app_id":"1"}`, 422, "invalid_reason"},
		{"not digits", `{"app_id":"https://zalo.me/s/1","reason":"x"}`, 422, "invalid_app_id"},
		{"commune in body", `{"app_id":"1","reason":"x","tenant_id":"` + communeIDFake + `"}`, 400, "invalid_body"},
		{"mode in body", `{"app_id":"1","reason":"x","che_do":"rieng"}`, 400, "invalid_body"},
	} {
		if rec := h.do("PUT", path, c.body, opCookie(t)); rec.Code != c.want || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s: %d %s; want %d %s", c.name, rec.Code, rec.Body, c.want, c.code)
		}
	}
	if h.shared.declared != 0 {
		t.Fatal("a refused request reached the store")
	}
	rec := h.do("PUT", path, `{"app_id":"`+sharedAppIDFake+`","reason":"  Đổi sang app đã phát hành  "}`, opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	if h.shared.sawApp != sharedAppIDFake || h.shared.sawReason != "Đổi sang app đã phát hành" ||
		h.shared.sawActor.Code != opCodeFake || h.shared.sawActor.IP == "" {
		t.Errorf("store saw app %q reason %q actor %+v", h.shared.sawApp, h.shared.sawReason, h.shared.sawActor)
	}

	h.shared.declareErr = store.ErrMiniAppTaken
	if rec := h.do("PUT", path, `{"app_id":"3291993990104489440","reason":"x"}`, opCookie(t)); rec.Code != 409 ||
		!strings.Contains(rec.Body.String(), "mini_app_taken") {
		t.Errorf("an App ID with a row: %d %s, want 409 mini_app_taken", rec.Code, rec.Body)
	}
}
