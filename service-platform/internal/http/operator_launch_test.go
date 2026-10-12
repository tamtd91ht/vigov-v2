package http

// The shared Mini App and the commune QR link (owner 04/10/2026). 401 / 401 staff token / 403 / 503 /
// 2xx for every route are in TestGuardedRoutes; this file defends what is specific to each.

import (
	"context"
	"encoding/json"
	"errors"
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
	code        string
	active      bool
	targetErr   error
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
		code:   launchCodeFake,
		active: true,
	}
}

const launchCodeFake = "0a1b2c3d4e5f"

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

func (f *sharedAppFake) CommuneLaunchTarget(_ context.Context, id string) (store.LaunchTarget, error) {
	f.launchReads++
	if id != communeIDFake {
		return store.LaunchTarget{}, store.ErrCommuneNotFound
	}
	if f.targetErr != nil {
		return store.LaunchTarget{}, f.targetErr
	}
	return store.LaunchTarget{Active: f.active, LaunchCode: f.code, Host: f.host}, nil
}

const launchPath = "/api/v1/communes/" + communeIDFake + "/mini-app-launch-link"

// citizenHint is what citizen-app's thamSoXa returns: a launch code (kind "code") or a host ("host").
type citizenHint struct{ kind, value string }

// citizenAppAccepts is citizen-app/src/lib/launch-params.ts thamSoXa, transcribed: `src` must be one
// of NGUON_CHON_SAN; then `x`, when PRESENT, must (lower-cased) pass isLaunchCode — a malformed `x`
// gives nothing and never falls back to `d`; with no `x`, `d` (lower-cased) must pass laTenMien
// (TRANSITIONAL, owner 12/10/2026). If this transcription and the TypeScript drift, the TS file is the
// authority — update both together.
func citizenAppAccepts(query string) (citizenHint, bool) {
	label := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	code := regexp.MustCompile(`^[0-9a-f]{12}$`)
	p, err := url.ParseQuery(query)
	if err != nil {
		return citizenHint{}, false
	}
	if src := p.Get("src"); src != "qr" && src != "zns" {
		return citizenHint{}, false
	}
	if p.Has("x") {
		x := strings.ToLower(p.Get("x"))
		if !code.MatchString(x) {
			return citizenHint{}, false
		}
		return citizenHint{"code", x}, true
	}
	d := strings.ToLower(p.Get("d"))
	if d == "" || len(d) > 253 {
		return citizenHint{}, false
	}
	parts := strings.Split(d, ".")
	if len(parts) < 2 {
		return citizenHint{}, false
	}
	for _, l := range parts {
		if !label.MatchString(l) {
			return citizenHint{}, false
		}
	}
	return citizenHint{"host", d}, true
}

// The transcription itself: `x` wins over `d`, a malformed `x` gives nothing (no fall back to `d`).
func TestCitizenAppAcceptsTranscription(t *testing.T) {
	for q, want := range map[string]citizenHint{
		"src=qr&x=0A1B2C3D4E5F":              {"code", "0a1b2c3d4e5f"},
		"d=a.vigov.vn&src=qr&x=0a1b2c3d4e5f": {"code", "0a1b2c3d4e5f"},
		"d=a.vigov.vn&src=qr":                {"host", "a.vigov.vn"},
	} {
		if got, ok := citizenAppAccepts(q); !ok || got != want {
			t.Errorf("%s: %+v %v, want %+v", q, got, ok, want)
		}
	}
	for _, q := range []string{"d=a.vigov.vn&src=qr&x=bad", "d=a.vigov.vn&src=qr&x=", "src=web&x=0a1b2c3d4e5f", "x=0a1b2c3d4e5f"} {
		if got, ok := citizenAppAccepts(q); ok {
			t.Errorf("%s: accepted as %+v", q, got)
		}
	}
}

// noLiveOwnApp leaves the commune with no running own app: only a row switched off before
// 05/10/2026 — not live, so the shared app's link is the only one.
func noLiveOwnApp(h *harness) {
	h.reg.setApps = true
	h.reg.apps = []domain.CommuneMiniApp{{AppID: "3043188591857102858", Mode: domain.CheDoRieng, Active: false}}
}

func decodeLaunchLinks(t *testing.T, body []byte) launchLinksView {
	t.Helper()
	var got launchLinksView
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Links == nil || got.Unavailable == nil {
		t.Fatalf("both arrays must be present, even empty: %s", body)
	}
	return got
}

const (
	sharedLinkFake       = "https://zalo.me/s/" + sharedAppIDFake + "/?src=qr&x=" + launchCodeFake
	sharedLegacyLinkFake = "https://zalo.me/s/" + sharedAppIDFake + "/?d=thangbinh-danang.vigov.vn&src=qr"
	ownLinkFake          = "https://zalo.me/s/3291993990104489440/?src=qr"
)

// A commune WITHOUT a live own app: the shared app's two links — `x=<launch code>` first, then the
// TRANSITIONAL `d=<primary host>` one (owner 12/10/2026). Both are the published-app form and both
// round-trip through the citizen app's own parameter reader: the first as the commune's code, the
// second as its primary host. No own app is an absence, not a refusal: nothing is unavailable.
func TestLaunchLinkRoundTripsThroughCitizenApp(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	noLiveOwnApp(h)
	rec := h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	all := decodeLaunchLinks(t, rec.Body.Bytes())
	if len(all.Links) != 2 || len(all.Unavailable) != 0 {
		t.Fatalf("got %+v, want the two shared links only", all)
	}
	for i, want := range []struct {
		url, source string
		hint        citizenHint
	}{
		{sharedLinkFake, "chung", citizenHint{"code", launchCodeFake}},
		{sharedLegacyLinkFake, "chung_ten_mien", citizenHint{"host", "thangbinh-danang.vigov.vn"}},
	} {
		got := all.Links[i]
		if got.URL != want.url || got.Domain != "thangbinh-danang.vigov.vn" || got.AppID != sharedAppIDFake || got.Source != want.source {
			t.Fatalf("link %d: %+v, want url %s source %s", i, got, want.url, want.source)
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
		if hint, ok := citizenAppAccepts(u.RawQuery); !ok || hint != want.hint {
			t.Errorf("citizen-app would not read the commune from %q (got %+v, %v)", u.RawQuery, hint, ok)
		}
	}
	// The x link names no domain: the code alone carries the commune.
	if strings.Contains(all.Links[0].URL, "d=") || strings.Contains(all.Links[0].URL, "vigov.vn") {
		t.Errorf("the x link carries the domain: %s", all.Links[0].URL)
	}
}

// A commune WITH a live own app gets EVERY link, shared first (owner 06/10/2026: "QR từ đâu thì mở
// app từ đó"): the shared app is borrowed while the own app waits for Zalo, so it must stay printable.
func TestLaunchLinkGivesBothWhenOwnAppIsLive(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	// registryFake's default: 3291993990104489440 running, 3043188591857102858 switched off.
	rec := h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	all := decodeLaunchLinks(t, rec.Body.Bytes())
	if len(all.Links) != 3 || len(all.Unavailable) != 0 {
		t.Fatalf("got %+v, want three links", all)
	}
	if s := all.Links[0]; s.Source != "chung" || s.URL != sharedLinkFake || s.AppID != sharedAppIDFake {
		t.Errorf("first link %+v, want the shared app's x link", s)
	}
	if s := all.Links[1]; s.Source != "chung_ten_mien" || s.URL != sharedLegacyLinkFake || s.AppID != sharedAppIDFake {
		t.Errorf("second link %+v, want the shared app's transitional d link", s)
	}
	o := all.Links[2]
	if o.Source != "rieng" || o.URL != ownLinkFake || o.AppID != "3291993990104489440" || o.Domain != "thangbinh-danang.vigov.vn" {
		t.Errorf("second link %+v, want the own app's", o)
	}
	u, err := url.Parse(o.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Has("d") {
		t.Error("an own-app link must not carry d=")
	}
}

// The shared link cannot be built, the own app is live: the own link alone, and the shared link's
// reason — the same code the route refused with before — so the card explains the gap.
func TestLaunchLinkOwnOnlyWhenSharedUnavailable(t *testing.T) {
	for _, c := range []struct {
		name   string
		setup  func(f *sharedAppFake)
		code   string
		domain string
	}{
		{"no shared app declared", func(f *sharedAppFake) { f.app = nil }, "shared_mini_app_not_declared", "thangbinh-danang.vigov.vn"},
		{"two shared apps", func(f *sharedAppFake) { f.appErr = store.ErrSharedMiniAppAmbiguous }, "shared_mini_app_ambiguous", "thangbinh-danang.vigov.vn"},
		{"no primary domain either", func(f *sharedAppFake) { f.app, f.host = nil, "" }, "shared_mini_app_not_declared", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.qr.issue"}
			c.setup(h.shared)
			rec := h.do("GET", launchPath, "", opCookie(t))
			if rec.Code != 200 {
				t.Fatalf("status %d (%s)", rec.Code, rec.Body)
			}
			all := decodeLaunchLinks(t, rec.Body.Bytes())
			if len(all.Links) != 1 || all.Links[0].Source != "rieng" || all.Links[0].URL != ownLinkFake || all.Links[0].Domain != c.domain {
				t.Fatalf("links %+v, want the own link only", all.Links)
			}
			// ONE entry: the shared app's reason is the reason for both shared links, listed once.
			if len(all.Unavailable) != 1 || all.Unavailable[0].Source != "chung" || all.Unavailable[0].Code != c.code || all.Unavailable[0].Message == "" {
				t.Fatalf("unavailable %+v, want chung %s", all.Unavailable, c.code)
			}
		})
	}
}

// Since 12/10/2026 the shared app's main link needs no domain: a commune with no usable primary host
// still gets its `x=` link. Only the TRANSITIONAL `d=` link is missing, listed with the reason.
func TestLaunchLinkNoUsablePrimaryHostLosesOnlyTheLegacyLink(t *testing.T) {
	for _, c := range []struct {
		name, host string
	}{
		{"no primary domain", ""},
		{"unparseable stored host", "Bad Host"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.qr.issue"}
			noLiveOwnApp(h)
			h.shared.host = c.host
			rec := h.do("GET", launchPath, "", opCookie(t))
			if rec.Code != 200 {
				t.Fatalf("status %d (%s)", rec.Code, rec.Body)
			}
			all := decodeLaunchLinks(t, rec.Body.Bytes())
			if len(all.Links) != 1 || all.Links[0].Source != "chung" || all.Links[0].URL != sharedLinkFake || all.Links[0].Domain != c.host {
				t.Fatalf("links %+v, want the x link only", all.Links)
			}
			if len(all.Unavailable) != 1 || all.Unavailable[0].Source != "chung_ten_mien" ||
				all.Unavailable[0].Code != "commune_no_primary_domain" || all.Unavailable[0].Message == "" {
				t.Fatalf("unavailable %+v, want chung_ten_mien commune_no_primary_domain", all.Unavailable)
			}
		})
	}
}

// A stored code that is not the 0024 shape (only a hand-edited row can hold one): no `x=` link — a QR
// the app would ignore is worse than none — and the reason is listed; the `d=` link still prints.
func TestLaunchLinkMalformedStoredCode(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	noLiveOwnApp(h)
	h.shared.code = "0A1B2C3D4E5F"
	rec := h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	all := decodeLaunchLinks(t, rec.Body.Bytes())
	if len(all.Links) != 1 || all.Links[0].Source != "chung_ten_mien" || all.Links[0].URL != sharedLegacyLinkFake {
		t.Fatalf("links %+v, want the d link only", all.Links)
	}
	if len(all.Unavailable) != 1 || all.Unavailable[0].Source != "chung" || all.Unavailable[0].Code != "commune_launch_code_invalid" {
		t.Fatalf("unavailable %+v", all.Unavailable)
	}
}

// Two running own apps is a state ADR 0070 #1 forbids but no constraint prevents: never a QR for an
// App ID nobody chose. It no longer hides the shared link; with no shared link either, the route
// refuses with the own-app reason, as it did before 06/10/2026.
func TestLaunchLinkTwoLiveOwnApps(t *testing.T) {
	setup := func(h *harness) {
		h.id.keys = []string{"ops.qr.issue"}
		h.reg.setApps = true
		h.reg.apps = []domain.CommuneMiniApp{
			{AppID: "3291993990104489440", Mode: domain.CheDoRieng, Active: true},
			{AppID: "3043188591857102858", Mode: domain.CheDoRieng, Active: true},
		}
	}
	h := newHarness(t)
	setup(h)
	rec := h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	all := decodeLaunchLinks(t, rec.Body.Bytes())
	if len(all.Links) != 2 || all.Links[0].Source != "chung" || all.Links[1].Source != "chung_ten_mien" {
		t.Fatalf("links %+v, want the two shared links only", all.Links)
	}
	if len(all.Unavailable) != 1 || all.Unavailable[0].Source != "rieng" || all.Unavailable[0].Code != "own_mini_app_ambiguous" {
		t.Fatalf("unavailable %+v", all.Unavailable)
	}

	h = newHarness(t)
	setup(h)
	h.shared.app = nil
	rec = h.do("GET", launchPath, "", opCookie(t))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"own_mini_app_ambiguous"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

// Refusals of the whole commune hold whatever links it has: an inactive commune with a live own app
// gets no QR, and a store failure on one link is an error, not a quietly half-empty card.
func TestLaunchLinkRefusesTheCommuneDespiteOwnApp(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(f *sharedAppFake)
		want  int
		code  string
	}{
		{"inactive commune", func(f *sharedAppFake) { f.active = false }, 409, "commune_inactive"},
		{"shared app read fails", func(f *sharedAppFake) { f.appErr = errors.New("connection reset") }, 500, "internal"},
		{"launch target read fails", func(f *sharedAppFake) { f.targetErr = errors.New("connection reset") }, 500, "internal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.qr.issue"}
			c.setup(h.shared)
			rec := h.do("GET", launchPath, "", opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
		})
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
		// No link at all — bad code AND no primary host — refuses with the x link's reason.
		{"bad code, no primary domain", func(f *sharedAppFake) { f.code, f.host = "", "" }, launchPath, 409, "commune_launch_code_invalid"},
		{"inactive commune", func(f *sharedAppFake) { f.active = false }, launchPath, 409, "commune_inactive"},
		{"inactive with no primary", func(f *sharedAppFake) { f.active, f.host = false, "" }, launchPath, 409, "commune_inactive"},
		{"unknown commune", func(*sharedAppFake) {}, "/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EZ/mini-app-launch-link", 404, "commune_not_found"},
		{"malformed id", func(*sharedAppFake) {}, "/api/v1/communes/not-a-ulid/mini-app-launch-link", 404, "commune_not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.qr.issue"}
			noLiveOwnApp(h)
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
