package main

// What this test defends: THE WIRING, not the middleware — the same argument as finance's
// cmd/server/main_test.go. core/staffauth and core/httpx prove what their middleware does; neither
// can see whether THIS binary installs them. So this drives the chain edgeChain actually builds,
// with the REAL routes and the REAL use case mounted behind it.

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
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-reporting/internal/app"
	"github.com/vihat/vigov/service-reporting/internal/domain"
	svchttp "github.com/vihat/vigov/service-reporting/internal/http"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	staffInternalID = "nd-01JINTERNALIDCUACANBO"
	fakeCookie      = "session-FAKE-NOT-A-REAL-TOKEN"

	route = "/api/v1/reporting-system-messages"
)

var (
	communeA = tenant.ID("01JA" + strings.Repeat("A", 22))
	communeB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type directoryFake map[string]tenant.Tenant

func (m directoryFake) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// resolverFake stands in for identity's ResolveStaffPrincipal and records the commune carried on
// the outgoing call — the one thing this service can assert about the cross-commune refusal, which
// identity itself decides (service-identity/internal/grpc/server.go).
type resolverFake struct {
	calls int
	out   staffauth.StaffPrincipal
	found bool
	err   error

	sentCommune tenant.ID
}

func (p *resolverFake) ResolveStaff(ctx context.Context, _, _ string) (staffauth.StaffPrincipal, bool, error) {
	p.calls++
	p.sentCommune, _ = tenant.From(ctx)
	return p.out, p.found, p.err
}

// overrideStoreCounting is the table of a commune that reworded nothing, counting reads so a refused
// request can be shown to stop before the store.
type overrideStoreCounting struct{ reads int }

var errNoWritePath = errors.New("overrideStoreCounting: no write path in this file")

func (s *overrideStoreCounting) ListLive(context.Context) ([]domain.MessageOverride, error) {
	s.reads++
	return nil, nil
}
func (s *overrideStoreCounting) LiveForUpdate(context.Context, *pkgstore.ScopedTx, string) (*domain.MessageOverride, error) {
	return nil, errNoWritePath
}
func (s *overrideStoreCounting) AddOverride(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}
func (s *overrideStoreCounting) UpdateText(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}
func (s *overrideStoreCounting) SoftDelete(context.Context, *pkgstore.ScopedTx, string, string, string, time.Time) error {
	return errNoWritePath
}

type server struct {
	h     http.Handler
	store *overrideStoreCounting
	res   *resolverFake
}

func newServer(t *testing.T, res *resolverFake) *server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := &overrideStoreCounting{}
	mux := http.NewServeMux()
	// THE REAL ROUTES, REGISTERED THE WAY main() REGISTERS THEM. A nil *store.DB: no case here
	// writes, so no transaction is ever opened.
	svchttp.Register(mux, svchttp.Deps{
		Checker:        staffauth.Checker{},
		SystemMessages: app.NewSystemMessages(pkgstore.New(nil), st),
		Log:            log,
	})
	dir := directoryFake{
		hostA: {ID: communeA, Host: hostA, Active: true},
		hostB: {ID: communeB, Host: hostB, Active: true},
	}
	return &server{h: edgeChain(mux, dir, res, log), store: st, res: res}
}

func (s *server) get(host, path, cookie string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d — body: %s", w.Code, want, w.Body.String())
	}
}

// adminOfA holds `admin.lookup`, the key the three routes declare — a row of `quyen`
// (service-identity/migrations/0001_init.sql:281).
func adminOfA() *resolverFake {
	return &resolverFake{found: true, out: staffauth.StaffPrincipal{
		StaffID: staffInternalID, PermissionKeys: []authz.Perm{"admin.lookup"},
	}}
}

func TestAdminAtOwnHostListsThe38Messages(t *testing.T) {
	s := newServer(t, adminOfA())
	w := s.get(hostA, route, fakeCookie)
	wantStatus(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 38 {
		t.Fatalf("items = %d, err = %v", len(out.Items), err)
	}
	if s.store.reads != 1 || s.res.calls != 1 || s.res.sentCommune != communeA {
		t.Errorf("reads=%d resolver calls=%d sent commune=%q", s.store.reads, s.res.calls, s.res.sentCommune)
	}
}

func TestSignedInWithoutAdminLookupIs403(t *testing.T) {
	s := newServer(t, &resolverFake{found: true, out: staffauth.StaffPrincipal{
		StaffID: staffInternalID, PermissionKeys: []authz.Perm{"report.read"},
	}})
	wantStatus(t, s.get(hostA, route, fakeCookie), http.StatusForbidden)
	if s.store.reads != 0 {
		t.Error("store read without admin.lookup")
	}
}

func TestNoCookieIs401AndCallsNobody(t *testing.T) {
	s := newServer(t, adminOfA())
	wantStatus(t, s.get(hostA, route, ""), http.StatusUnauthorized)
	if s.res.calls != 0 || s.store.reads != 0 {
		t.Errorf("resolver calls=%d reads=%d", s.res.calls, s.store.reads)
	}
}

func TestCredentialOfAnotherCommuneIs401AndSendsTheHostsCommune(t *testing.T) {
	// Identity answers "no principal" when the commune in x-tenant-id does not match the credential.
	// What this service proves is that it SENT the Host's commune to be compared.
	s := newServer(t, &resolverFake{found: false})
	wantStatus(t, s.get(hostB, route, fakeCookie), http.StatusUnauthorized)
	if s.res.sentCommune != communeB || s.store.reads != 0 {
		t.Errorf("sent commune=%q reads=%d", s.res.sentCommune, s.store.reads)
	}
}

func TestIdentityDownIs503NotA401(t *testing.T) {
	s := newServer(t, &resolverFake{err: errors.New("rpc error: code = Unavailable")})
	wantStatus(t, s.get(hostA, route, fakeCookie), http.StatusServiceUnavailable)
}

func TestUnknownHostIs404AndCallsNobody(t *testing.T) {
	s := newServer(t, adminOfA())
	wantStatus(t, s.get("khong-co-xa.example.gov.vn", route, fakeCookie), http.StatusNotFound)
	if s.res.calls != 0 || s.store.reads != 0 {
		t.Errorf("resolver calls=%d reads=%d", s.res.calls, s.store.reads)
	}
}

func TestHealthzIsOutsideTheCommuneChain(t *testing.T) {
	s := newServer(t, adminOfA())
	wantStatus(t, s.get("khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
	if s.res.calls != 0 {
		t.Error("/healthz called identity")
	}
}
