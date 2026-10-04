package opauth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/operatortoken"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
)

// Fake key material — the text says so in full (rule 8, forbidden #1).
var (
	opKeyFake    = secret.Secret("operator-signing-key-FAKE-0123456789abcdef")
	staffKeyFake = secret.Secret("staff-signing-key-FAKE-0123456789abcdefgh")
	now          = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
)

type resolverFake struct {
	calls int
	p     operatorclient.Principal
	live  bool
	err   error
}

func (f *resolverFake) Resolve(context.Context, secret.Secret) (operatorclient.Principal, bool, error) {
	f.calls++
	return f.p, f.live, f.err
}

func newAuth(t *testing.T, r *resolverFake) *Auth {
	t.Helper()
	s, err := operatortoken.NewSigner([]secret.Secret{opKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	return NewAuth(s, r, slog.New(slog.NewTextHandler(io.Discard, nil))).WithClock(func() time.Time { return now })
}

func opToken(t *testing.T) string {
	t.Helper()
	s, _ := operatortoken.NewSigner([]secret.Secret{opKeyFake})
	tok, err := s.Sign(operatortoken.Claims{SessionID: "sid-FAKE", ExpiresAt: now.Add(8 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func staffToken(t *testing.T) string {
	t.Helper()
	s, err := token.NewSigner([]secret.Secret{staffKeyFake})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.Ky(token.Claims{TenantID: tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF"), Sid: "sid", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func serve(h http.Handler, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: CookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	p, ok := From(r.Context())
	if !ok || p.OperatorCode == "" {
		w.WriteHeader(http.StatusTeapot)
		return
	}
	w.WriteHeader(http.StatusOK)
})

func clearedCookie(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func TestRequireKey(t *testing.T) {
	live := operatorclient.Principal{OperatorID: "op-1", OperatorCode: "VH-00001",
		PermissionKeys: []string{string(KeyTenantManage)}}

	cases := []struct {
		name      string
		cookie    func(*testing.T) string
		res       resolverFake
		keys      []Key
		want      int
		wantCalls int
		cleared   bool
	}{
		{"no cookie → 401, no RPC", func(*testing.T) string { return "" }, resolverFake{}, []Key{KeyTenantManage}, 401, 0, false},
		{"forged → 401, no RPC", func(*testing.T) string { return "op1.forged.AAAA" }, resolverFake{}, []Key{KeyTenantManage}, 401, 0, true},
		{"staff v1 token → 401, no RPC", staffToken, resolverFake{}, []Key{KeyTenantManage}, 401, 0, true},
		{"session not live → 401", opToken, resolverFake{}, []Key{KeyTenantManage}, 401, 1, true},
		{"identity down → 503", opToken, resolverFake{err: errors.New("unavailable")}, []Key{KeyTenantManage}, 503, 1, false},
		{"missing key → 403", opToken, resolverFake{p: live, live: true}, []Key{KeyTenantManage, KeyDomainManage}, 403, 1, false},
		{"holds every key → 200", opToken, resolverFake{p: live, live: true}, []Key{KeyTenantManage}, 200, 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.res
			h := RequireKey(newAuth(t, &r), c.keys...)(okHandler)
			rec := serve(h, c.cookie(t))
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, c.want, rec.Body)
			}
			if r.calls != c.wantCalls {
				t.Errorf("identity calls = %d, want %d", r.calls, c.wantCalls)
			}
			if clearedCookie(rec) != c.cleared {
				t.Errorf("cookie cleared = %v, want %v", clearedCookie(rec), c.cleared)
			}
		})
	}
}

// AnyKey (ADR 0073 #1): one decided key of any kind opens a console read; a live session holding no
// decided key — none at all, a commune permission, an unknown ops string, or the literal "ops.*" —
// is 403, never the SignedIn pass-through.
func TestRequireAnyKey(t *testing.T) {
	for name, c := range map[string]struct {
		keys []string
		want int
	}{
		"zero keys":             {nil, 403},
		"commune key only":      {[]string{"feedback.read"}, 403},
		"unknown ops key":       {[]string{"ops.anything.goes"}, 403},
		"literal sentinel":      {[]string{"ops.*"}, 403},
		"only ops.qr.issue":     {[]string{string(KeyQRIssue)}, 200},
		"only mini app":         {[]string{string(KeyMiniAppManage)}, 200},
		"unknown + one decided": {[]string{"ops.anything.goes", string(KeyDomainManage)}, 200},
	} {
		t.Run(name, func(t *testing.T) {
			r := &resolverFake{p: operatorclient.Principal{OperatorID: "op-1", OperatorCode: "VH-00001",
				PermissionKeys: c.keys}, live: true}
			if rec := serve(RequireKey(newAuth(t, r), AnyKey)(okHandler), opToken(t)); rec.Code != c.want {
				t.Fatalf("status = %d, want %d", rec.Code, c.want)
			}
		})
	}
	if rec := serve(RequireKey(newAuth(t, &resolverFake{}), AnyKey)(okHandler), ""); rec.Code != 401 {
		t.Fatalf("no cookie: status = %d, want 401", rec.Code)
	}
}

func TestSignedInNeedsNoKey(t *testing.T) {
	r := &resolverFake{p: operatorclient.Principal{OperatorID: "op-1", OperatorCode: "VH-00001"}, live: true}
	if rec := serve(SignedIn(newAuth(t, r), "test")(okHandler), opToken(t)); rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec := serve(SignedIn(newAuth(t, &resolverFake{}), "test")(okHandler), ""); rec.Code != 401 {
		t.Fatalf("no cookie: status = %d", rec.Code)
	}
}

// No signing keys configured (dev only): 503 for every guarded route, never a pass-through.
func TestNoVerifierIs503(t *testing.T) {
	a := NewAuth(nil, &resolverFake{live: true}, nil)
	if rec := serve(RequireKey(a, KeyTenantManage)(okHandler), opToken(t)); rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestDeclarationsRefuseBadWiring(t *testing.T) {
	a := newAuth(t, &resolverFake{})
	for name, f := range map[string]func(){
		"unknown key":      func() { RequireKey(a, Key("ops.anything.goes")) },
		"commune key":      func() { RequireKey(a, Key("feedback.read")) },
		"no key":           func() { RequireKey(a) },
		"any key mixed":    func() { RequireKey(a, AnyKey, KeyTenantManage) },
		"any key twice":    func() { RequireKey(a, AnyKey, AnyKey) },
		"nil auth":         func() { RequireKey(nil, KeyTenantManage) },
		"signed-in no why": func() { SignedIn(a, "") },
		"public no why":    func() { Public("") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			f()
		}()
	}
}

func TestCookieIsHostOnlyStrictAndBounded(t *testing.T) {
	rec := httptest.NewRecorder()
	SetCookie(rec, secret.Secret("op1.tok"), now.Add(8*time.Hour), now)
	raw := rec.Header().Get("Set-Cookie")
	for _, want := range []string{CookieName + "=", "Path=/", "Max-Age=28800", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(raw, want) {
			t.Errorf("Set-Cookie %q lacks %q", raw, want)
		}
	}
	if strings.Contains(strings.ToLower(raw), "domain=") {
		t.Errorf("Set-Cookie carries a Domain attribute: %q", raw)
	}
	if !strings.HasPrefix(CookieName, "__Host-") {
		t.Error("cookie name must keep the __Host- prefix")
	}
}
