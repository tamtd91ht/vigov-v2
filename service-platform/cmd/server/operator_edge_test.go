package main

// What this test defends: THE HOST SPLIT in this binary — that operator routes exist on
// OPERATOR_HOST only, that they do not exist at all when OPERATOR_HOST is unset, and that the commune
// chain never honours an operator cookie (ADR 0048 §28/09 #2 + #4, stop condition #6). The guards and
// handlers have their own tests in internal/; nothing there can see which chain main mounts.

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

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/operatortoken"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const operatorHostTest = "admin.vigov.vn"

var opKeyTest = secret.Secret("operator-signing-key-FAKE-0123456789abcdef")

// directoryOneCommune resolves hostThu (main_test.go) and nothing else — like the real directory,
// it never resolves a reserved host such as admin.vigov.vn.
type directoryOneCommune struct{}

func (directoryOneCommune) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	if host == hostThu {
		return tenant.Tenant{ID: ulidThu, Host: hostThu, Active: true}, true
	}
	return tenant.Tenant{}, false
}

type resolverCounting struct{ calls int }

func (r *resolverCounting) Resolve(context.Context, secret.Secret) (operatorclient.Principal, bool, error) {
	r.calls++
	return operatorclient.Principal{OperatorID: "op-1", OperatorCode: "VH-00001",
		PermissionKeys: []string{"ops.tenant.manage"}}, true, nil
}

type readerEmpty struct{}

func (readerEmpty) ListCommunes(context.Context, page.Request) (page.Result[domain.Commune], error) {
	return page.NewResult[domain.Commune](), nil
}
func (readerEmpty) Commune(context.Context, string) (domain.Commune, []domain.CommuneMiniApp, error) {
	return domain.Commune{}, nil, store.ErrCommuneNotFound
}
func (readerEmpty) Provinces(context.Context) ([]domain.Province, error) { return nil, nil }

type counterOK struct{}

func (counterOK) Incr(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 1, time.Minute, nil
}

// communeChain is the commune edge exactly as run() builds it, over an empty route table — platform
// has no commune REST routes yet, which is the point: an operator path there is a 404.
func communeChain() http.Handler {
	var h http.Handler = http.NewServeMux()
	h = httpx.TenantMiddleware(directoryOneCommune{})(h)
	h = httpx.Recover(traceID)(h)
	return httpx.StripTenantHeaders(h)
}

func operatorChain(t *testing.T, res *resolverCounting) http.Handler {
	t.Helper()
	signer, _ := operatortoken.NewSigner([]secret.Secret{opKeyTest})
	lim, _ := ratelimit.New(counterOK{}, ratelimit.OperatorSignIn)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return buildOperatorEdge(svchttp.OperatorDeps{
		Auth: opauth.NewAuth(signer, res, log), Identity: nil, Limiter: lim,
		Registry: readerEmpty{}, OperatorHost: operatorHostTest,
		NewID: func() (string, error) { return "", nil }, Log: log,
	})
}

func operatorCookieTest(t *testing.T) *http.Cookie {
	t.Helper()
	signer, _ := operatortoken.NewSigner([]secret.Secret{opKeyTest})
	tok, err := signer.Sign(operatortoken.Claims{SessionID: "sid", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: opauth.CookieName, Value: tok}
}

func get(h http.Handler, host, path string, c *http.Cookie) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestOperatorRoutesOnlyOnOperatorHost(t *testing.T) {
	res := &resolverCounting{}
	outer := buildOuter(communeChain(), operatorChain(t, res), operatorHostTest)
	cookie := operatorCookieTest(t)

	if got := get(outer, operatorHostTest, "/api/v1/communes", cookie); got != http.StatusOK {
		t.Fatalf("operator host: %d, want 200", got)
	}
	if got := get(outer, "ADMIN.vigov.vn:443", "/api/v1/communes", nil); got != http.StatusUnauthorized {
		t.Errorf("operator host with case and port: %d, want 401 from the operator chain", got)
	}
	callsBefore := res.calls
	// A commune host: the commune chain, where no operator route exists — and the operator cookie is
	// never looked at, so identity is never asked about it.
	if got := get(outer, hostThu, "/api/v1/communes", cookie); got != http.StatusNotFound {
		t.Errorf("commune host: %d, want 404", got)
	}
	if res.calls != callsBefore {
		t.Error("the commune chain asked identity about an operator cookie")
	}
	if got := get(outer, "unknown.example.vn", "/api/v1/communes", cookie); got != http.StatusNotFound {
		t.Errorf("unknown host: %d, want 404", got)
	}
	if got := get(outer, "10.0.0.1:8080", "/healthz", nil); got != http.StatusOK {
		t.Errorf("/healthz: %d", got)
	}
}

// OPERATOR_HOST unset: the operator routes are not mounted anywhere. The operator host itself is a
// reserved host on the commune chain and answers like any unknown host.
func TestOperatorAreaOffIs404Everywhere(t *testing.T) {
	outer := buildOuter(communeChain(), nil, "")
	cookie := operatorCookieTest(t)
	for _, host := range []string{operatorHostTest, hostThu} {
		for _, path := range []string{"/api/v1/communes", "/api/v1/operator-sessions/current", "/api/v1/provinces"} {
			if got := get(outer, host, path, cookie); got != http.StatusNotFound {
				t.Errorf("%s%s with the area off: %d, want 404", host, path, got)
			}
		}
	}
	// A handler without a host, or a host without a handler, is OFF too — never "route everything".
	res := &resolverCounting{}
	for _, outer := range []http.Handler{
		buildOuter(communeChain(), operatorChain(t, res), ""),
		buildOuter(communeChain(), nil, operatorHostTest),
	} {
		if got := get(outer, operatorHostTest, "/api/v1/communes", cookie); got != http.StatusNotFound {
			t.Errorf("half-configured edge served an operator route: %d", got)
		}
	}
	if res.calls != 0 {
		t.Error("identity asked while the operator area is off")
	}
}

type holderFake struct {
	held  bool
	err   error
	asked string
}

func (h *holderFake) HostHeld(_ context.Context, host string) (bool, error) {
	h.asked = host
	return h.held, h.err
}

// OPERATOR_HOST held by a commune row refuses startup, naming the variable and never the commune;
// a lookup failure refuses too (fail closed); unset asks nothing.
func TestOperatorHostCollisionRefusesStartup(t *testing.T) {
	ctx := context.Background()

	off := &holderFake{held: true}
	if err := refuseOperatorHostCollision(ctx, off, ""); err != nil || off.asked != "" {
		t.Fatalf("area off: err %v, asked %q — nothing to check", err, off.asked)
	}

	held := &holderFake{held: true}
	err := refuseOperatorHostCollision(ctx, held, operatorHostTest)
	if err == nil || !strings.Contains(err.Error(), "OPERATOR_HOST") {
		t.Fatalf("held host: %v, want a refusal naming OPERATOR_HOST", err)
	}
	if held.asked != operatorHostTest {
		t.Errorf("asked about %q", held.asked)
	}

	down := &holderFake{err: errors.New("db down (fake)")}
	if err := refuseOperatorHostCollision(ctx, down, operatorHostTest); err == nil {
		t.Fatal("a failed lookup let the service start — 'could not check' is not 'no collision'")
	}

	if err := refuseOperatorHostCollision(ctx, &holderFake{}, operatorHostTest); err != nil {
		t.Fatalf("free host: %v", err)
	}
}

// A write from a commune page reaches the operator edge with that commune's Origin: refused by the
// chain main mounts, not just by the handler in isolation.
func TestOperatorEdgeRefusesCrossOriginWrite(t *testing.T) {
	outer := buildOuter(communeChain(), operatorChain(t, &resolverCounting{}), operatorHostTest)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/operator-sessions",
		strings.NewReader(`{"email":"x@example.invalid","password":"p"}`))
	req.Host = operatorHostTest
	req.Header.Set("Origin", "https://"+hostThu)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	outer.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin write on the operator host: %d, want 403", rec.Code)
	}
}
