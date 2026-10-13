package http

// GET /api/v1/commune-launch-link (owner 13/10/2026): the four-case permission suite rule 5, invariant 7
// requires — 401 · 403 wrong permission · 403 right permission WRONG COMMUNE · 200 — through the REAL
// Register behind the real edge order, then the ONE-link rule (chooseLaunchLink) and its refusals.

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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const (
	qlPath     = "/api/v1/commune-launch-link"
	qlOwnApp   = "3291993990104489440"
	qlOwnApp2  = "3043188591857102858"
	qlSharedID = "1234567890123456789"
	qlCodeA    = "0a1b2c3d4e5f"
	qlCodeB    = "b0b1b2b3b4b5"
)

// launchFake answers BY THE COMMUNE IN CTX — never by an argument — so a handler reading the wrong
// commune would show up as the wrong link, not just a wrong counter.
type launchFake struct {
	communes  map[tenant.ID]store.CommuneLaunch
	err       error
	calls     int
	sawTenant tenant.ID
}

func (f *launchFake) CommuneLaunch(ctx context.Context) (store.CommuneLaunch, error) {
	f.calls++
	f.sawTenant = tenant.MustFrom(ctx)
	if f.err != nil {
		return store.CommuneLaunch{}, f.err
	}
	c, ok := f.communes[f.sawTenant]
	if !ok {
		return store.CommuneLaunch{}, store.ErrCommuneNotFound
	}
	return c, nil
}

type qlSharedFake struct {
	app *domain.SharedMiniApp
	err error
}

func (f *qlSharedFake) SharedMiniApp(context.Context) (domain.SharedMiniApp, error) {
	if f.err != nil {
		return domain.SharedMiniApp{}, f.err
	}
	if f.app == nil {
		return domain.SharedMiniApp{}, store.ErrNoSharedMiniApp
	}
	return *f.app, nil
}

type qlServer struct {
	h       http.Handler
	launch  *launchFake
	shared  *qlSharedFake
	checker *brChecker
}

func newQlServer(t *testing.T) *qlServer {
	t.Helper()
	launch := &launchFake{communes: map[tenant.ID]store.CommuneLaunch{
		brCommuneA: {Active: true, LaunchCode: qlCodeA},
		brCommuneB: {Active: true, LaunchCode: qlCodeB},
	}}
	shared := &qlSharedFake{app: &domain.SharedMiniApp{AppID: qlSharedID}}
	checker := &brChecker{grants: map[tenant.ID]map[authz.Perm]bool{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{Checker: checker, Branding: &brActs{}, URLs: brURLs{},
		Uploads: httpx.NewUploadSlots(httpx.UploadSlotsPerPod), Launch: launch, SharedApp: shared, Log: log})

	var h http.Handler = mux
	// In place of core/staffauth, as in branding_test.go: the principal is NOT copied from the Host.
	h = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p, ok := r.Context().Value(brPrincipalKey{}).(authz.Principal); ok {
				r = r.WithContext(authz.Into(r.Context(), p))
			}
			next.ServeHTTP(w, r)
		})
	}(h)
	h = idem.Middleware(&brIdem{m: map[string]string{}}, log)(h)
	h = httpx.TenantMiddleware(brDirectory{
		brHostA: {ID: brCommuneA, Host: brHostA, Name: "Xã A", Active: true},
		brHostB: {ID: brCommuneB, Host: brHostB, Name: "Xã B", Active: true},
	})(h)
	h = httpx.Recover(func(context.Context) string { return "test" })(h)
	h = httpx.StripTenantHeaders(h)
	return &qlServer{h: h, launch: launch, shared: shared, checker: checker}
}

func (s *qlServer) grant(c tenant.ID, perm authz.Perm) {
	if s.checker.grants[c] == nil {
		s.checker.grants[c] = map[authz.Perm]bool{}
	}
	s.checker.grants[c][perm] = true
}

func (s *qlServer) get(host string, p *authz.Principal) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "https://"+host+qlPath, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), brPrincipalKey{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func TestCommuneLaunchLinkNoSessionIs401(t *testing.T) {
	s := newQlServer(t)
	brWant(t, s.get(brHostA, nil), http.StatusUnauthorized)
	if s.launch.calls != 0 {
		t.Error("reached the store without a session")
	}
}

func TestCommuneLaunchLinkWrongPermissionIs403(t *testing.T) {
	s := newQlServer(t)
	// A neighbouring administration key: managing the org chart is not the key the owner chose.
	s.grant(brCommuneA, "admin.org")
	brWant(t, s.get(brHostA, brStaff(brCommuneA)), http.StatusForbidden)
	if s.launch.calls != 0 {
		t.Error("reached the store with the wrong permission")
	}
}

func TestCommuneLaunchLinkRightPermissionWrongCommuneIs403(t *testing.T) {
	s := newQlServer(t)
	s.grant(brCommuneA, "admin.lookup")
	// (1) granted in A; a principal of B at B's host.
	brWant(t, s.get(brHostB, brStaff(brCommuneB)), http.StatusForbidden)
	// (2) A's principal presented at B's host — the token's commune is not the Host's: 401 (rule 1
	// invariant 8, core/authz), stricter than the 403 of case (1).
	brWant(t, s.get(brHostB, brStaff(brCommuneA)), http.StatusUnauthorized)
	if s.launch.calls != 0 {
		t.Error("a permission granted in commune A read commune B")
	}
}

func TestCommuneLaunchLinkRightPermissionRightCommuneIs200InThatCommune(t *testing.T) {
	s := newQlServer(t)
	s.grant(brCommuneB, "admin.lookup")
	w := s.get(brHostB, brStaff(brCommuneB))
	brWant(t, w, http.StatusOK)
	if s.launch.calls != 1 || s.launch.sawTenant != brCommuneB {
		t.Fatalf("calls=%d commune=%q, want one read in commune B (from Host)", s.launch.calls, s.launch.sawTenant)
	}
	got := decodeCommuneLaunch(t, w)
	// B's code, never A's: the link is the Host commune's.
	if got.URL != "https://zalo.me/s/"+qlSharedID+"/?src=qr&x="+qlCodeB {
		t.Errorf("url = %s, want commune B's x link", got.URL)
	}
}

// decodeCommuneLaunch also pins the reply's shape: exactly url and source — no App ID, no domain
// (owner 13/10/2026: staff see nothing about the shared app).
func decodeCommuneLaunch(t *testing.T, w *httptest.ResponseRecorder) communeLaunchLinkOut {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 || raw["url"] == nil || raw["source"] == nil {
		t.Fatalf("reply %s, want exactly url and source", w.Body.String())
	}
	var out communeLaunchLinkOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCommuneLaunchLinkChoosesOneLink(t *testing.T) {
	for _, c := range []struct {
		name   string
		own    []string
		url    string
		source string
	}{
		{"one live own app wins", []string{qlOwnApp}, "https://zalo.me/s/" + qlOwnApp + "/?src=qr", "rieng"},
		{"no own app: shared x link", nil, "https://zalo.me/s/" + qlSharedID + "/?src=qr&x=" + qlCodeA, "chung"},
		// Two running own apps: never a QR for one nobody chose — the shared link is still given.
		{"two own apps: shared x link", []string{qlOwnApp, qlOwnApp2}, "https://zalo.me/s/" + qlSharedID + "/?src=qr&x=" + qlCodeA, "chung"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newQlServer(t)
			s.grant(brCommuneA, "admin.lookup")
			s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: true, LaunchCode: qlCodeA, LiveOwnAppIDs: c.own}
			w := s.get(brHostA, brStaff(brCommuneA))
			brWant(t, w, http.StatusOK)
			got := decodeCommuneLaunch(t, w)
			if got.URL != c.url || got.Source != c.source {
				t.Errorf("got %+v, want %s %s", got, c.url, c.source)
			}
			if strings.Contains(got.URL, "d=") {
				t.Errorf("the d= form is withdrawn (owner 13/10/2026): %s", got.URL)
			}
		})
	}
}

// The own app wins even when the shared app cannot be read: the shared app is not consulted at all.
func TestCommuneLaunchLinkOwnAppNeedsNoSharedApp(t *testing.T) {
	s := newQlServer(t)
	s.grant(brCommuneA, "admin.lookup")
	s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: true, LaunchCode: qlCodeA, LiveOwnAppIDs: []string{qlOwnApp}}
	s.shared.err = errors.New("connection reset")
	w := s.get(brHostA, brStaff(brCommuneA))
	brWant(t, w, http.StatusOK)
	if got := decodeCommuneLaunch(t, w); got.Source != "rieng" {
		t.Errorf("got %+v, want the own link", got)
	}
}

func TestCommuneLaunchLinkRefusals(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(s *qlServer)
		want  int
		code  string
	}{
		{"inactive commune", func(s *qlServer) {
			s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: false, LaunchCode: qlCodeA, LiveOwnAppIDs: []string{qlOwnApp}}
		}, http.StatusConflict, "commune_inactive"},
		{"no shared app, no own app", func(s *qlServer) { s.shared.app = nil }, http.StatusConflict, "shared_mini_app_not_declared"},
		{"two shared apps", func(s *qlServer) { s.shared.err = store.ErrSharedMiniAppAmbiguous }, http.StatusConflict, "shared_mini_app_ambiguous"},
		{"two own apps, no shared app", func(s *qlServer) {
			s.shared.app = nil
			s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: true, LaunchCode: qlCodeA, LiveOwnAppIDs: []string{qlOwnApp, qlOwnApp2}}
		}, http.StatusConflict, "own_mini_app_ambiguous"},
		{"malformed stored code", func(s *qlServer) {
			s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: true, LaunchCode: "0A1B2C3D4E5F"}
		}, http.StatusConflict, "commune_launch_code_invalid"},
		// An outage is not "no link".
		{"store fails", func(s *qlServer) { s.launch.err = errors.New("connection reset") }, http.StatusInternalServerError, "internal"},
		{"shared read fails, two own apps", func(s *qlServer) {
			s.shared.err = errors.New("connection reset")
			s.launch.communes[brCommuneA] = store.CommuneLaunch{Active: true, LaunchCode: qlCodeA, LiveOwnAppIDs: []string{qlOwnApp, qlOwnApp2}}
		}, http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newQlServer(t)
			s.grant(brCommuneA, "admin.lookup")
			c.setup(s)
			w := s.get(brHostA, brStaff(brCommuneA))
			brWant(t, w, c.want)
			if !strings.Contains(w.Body.String(), `"code":"`+c.code+`"`) {
				t.Errorf("body %s, want code %s", w.Body.String(), c.code)
			}
			if strings.Contains(w.Body.String(), qlSharedID) || strings.Contains(w.Body.String(), "zalo.me") {
				t.Errorf("a refusal carried a link or an App ID: %s", w.Body.String())
			}
		})
	}
}

// The commune is never read from the request: a header or query naming another commune changes nothing.
func TestCommuneLaunchLinkIgnoresAClientNamedCommune(t *testing.T) {
	s := newQlServer(t)
	s.grant(brCommuneA, "admin.lookup")
	r := httptest.NewRequest(http.MethodGet, "https://"+brHostA+qlPath+"?tenant_id="+string(brCommuneB), nil)
	r.Host = brHostA
	r.Header.Set("X-Tenant-ID", string(brCommuneB))
	r = r.WithContext(context.WithValue(r.Context(), brPrincipalKey{}, *brStaff(brCommuneA)))
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	brWant(t, w, http.StatusOK)
	if s.launch.sawTenant != brCommuneA || !strings.Contains(w.Body.String(), qlCodeA) {
		t.Errorf("read commune %q, body %s — want A, from Host", s.launch.sawTenant, w.Body.String())
	}
}
