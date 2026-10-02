package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403 right
// permission WRONG COMMUNE · 2xx both correct — for ALL SEVEN commune-branding routes, through the REAL
// Register behind the real edge order (StripTenantHeaders → Recover → TenantMiddleware → idem →
// principal). Plus the refusal mapping and the actor (CB- code, never the internal id).

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/app"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

const (
	brHostA = "xa-a.example.gov.vn"
	brHostB = "xa-b.example.gov.vn"
)

var (
	brCommuneA = tenant.ID("01JA" + strings.Repeat("A", 22))
	brCommuneB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type brDirectory map[string]tenant.Tenant

func (d brDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := d[host]
	return t, ok
}

// brChecker grants BY COMMUNE, read from the CONTEXT — never from the principal, which is the claim.
type brChecker struct {
	grants map[tenant.ID]map[authz.Perm]bool
}

func (c *brChecker) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	if p.TenantID != tenant.MustFrom(ctx) {
		return false
	}
	return c.grants[tenant.MustFrom(ctx)][perm]
}

type brIdem struct {
	mu sync.Mutex
	m  map[string]string
}

func (k *brIdem) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.m[key]; ok {
		return false, nil
	}
	k.m[key] = ""
	return true, nil
}
func (k *brIdem) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[key], nil
}
func (k *brIdem) Complete(_ context.Context, key, v string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = v
	return nil
}
func (k *brIdem) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

// brActs records the commune and the actor of every call — the two facts a handler can get wrong.
type brActs struct {
	calls  int
	tenant tenant.ID
	actor  audit.Actor
	img    domain.BrandingImage
	err    error
}

func (a *brActs) note(ctx context.Context, img domain.BrandingImage, actor audit.Actor) {
	a.calls++
	a.tenant, a.img, a.actor = tenant.MustFrom(ctx), img, actor
}
func (a *brActs) Settings(ctx context.Context) (app.BrandingSettings, error) {
	a.note(ctx, "", audit.Actor{})
	return app.BrandingSettings{LogoPublicURL: "https://media.example/x.png"}, a.err
}
func (a *brActs) RequestUpload(ctx context.Context, img domain.BrandingImage, _ app.BrandingUploadRequest,
	actor audit.Actor) (app.BrandingUpload, error) {
	a.note(ctx, img, actor)
	return app.BrandingUpload{File: domain.StoredFile{ID: "01JFILE0000000000000000000", Status: domain.StoredFilePending},
		Post: storage.PresignedPost{URL: "https://minio.example/temp", Fields: map[string]string{"key": "upload/x"}}}, a.err
}
func (a *brActs) Complete(ctx context.Context, img domain.BrandingImage, id string, actor audit.Actor) (domain.StoredFile, error) {
	a.note(ctx, img, actor)
	return domain.StoredFile{ID: id, Status: domain.StoredFileReady, PublicObjectKey: "public-media/k"}, a.err
}
func (a *brActs) Remove(ctx context.Context, img domain.BrandingImage, actor audit.Actor) (bool, error) {
	a.note(ctx, img, actor)
	return true, a.err
}

type brURLs struct{}

func (brURLs) PublicURL(key string) string {
	if key == "" {
		return ""
	}
	return "https://media.example/" + key
}

type brServer struct {
	h       http.Handler
	acts    *brActs
	checker *brChecker
}

type brPrincipalKey struct{}

func newBrServer(t *testing.T) *brServer {
	t.Helper()
	acts := &brActs{}
	checker := &brChecker{grants: map[tenant.ID]map[authz.Perm]bool{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{Checker: checker, Branding: acts, URLs: brURLs{}, Log: log})

	var h http.Handler = mux
	// In place of core/staffauth: inject the principal WITHOUT copying the Host commune onto it, so the
	// wrong-commune cases stay expressible.
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
	return &brServer{h: h, acts: acts, checker: checker}
}

func (s *brServer) grant(c tenant.ID, perm authz.Perm) {
	if s.checker.grants[c] == nil {
		s.checker.grants[c] = map[authz.Perm]bool{}
	}
	s.checker.grants[c][perm] = true
}

func brStaff(c tenant.ID) *authz.Principal {
	return &authz.Principal{ID: "01JINTERNALSTAFFID00000000", Ma: "CB-00123", Kind: "staff", TenantID: c}
}

func (s *brServer) call(method, host, path, body string, p *authz.Principal) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if method != http.MethodGet {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JBRANDINGKEY000000000000")
	}
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), brPrincipalKey{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const brUploadBody = `{"file_name":"logo.png","content_type":"image/png","size":2048}`

type brRoute struct {
	name, method, path, body string
	ok                       int
}

func brRoutes() []brRoute {
	return []brRoute{
		{"settings", http.MethodGet, "/api/v1/commune-branding", "", http.StatusOK},
		{"logo request", http.MethodPost, "/api/v1/commune-branding/logo-uploads", brUploadBody, http.StatusCreated},
		{"logo completion", http.MethodPost, "/api/v1/commune-branding/logo-uploads/01JFILE0000000000000000000/completion", "", http.StatusOK},
		{"logo removal", http.MethodDelete, "/api/v1/commune-branding/logo", "", http.StatusNoContent},
		{"banner request", http.MethodPost, "/api/v1/commune-branding/banner-uploads", brUploadBody, http.StatusCreated},
		{"banner completion", http.MethodPost, "/api/v1/commune-branding/banner-uploads/01JFILE0000000000000000000/completion", "", http.StatusOK},
		{"banner removal", http.MethodDelete, "/api/v1/commune-branding/banner", "", http.StatusNoContent},
	}
}

func brWant(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("status = %d, want %d — body: %s", w.Code, code, w.Body.String())
	}
}

func TestBrandingNoSessionIs401(t *testing.T) {
	for _, rt := range brRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newBrServer(t)
			brWant(t, s.call(rt.method, brHostA, rt.path, rt.body, nil), http.StatusUnauthorized)
			if s.acts.calls != 0 {
				t.Error("reached the use case without a session")
			}
		})
	}
}

func TestBrandingWrongPermissionIs403(t *testing.T) {
	for _, rt := range brRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			// A neighbouring administration key: the right to manage the catalogue is not the right to
			// change the commune's official identity.
			s := newBrServer(t)
			s.grant(brCommuneA, "admin.lookup")
			brWant(t, s.call(rt.method, brHostA, rt.path, rt.body, brStaff(brCommuneA)), http.StatusForbidden)
			if s.acts.calls != 0 {
				t.Error("reached the use case with the wrong permission")
			}
		})
	}
}

func TestBrandingRightPermissionWrongCommuneIs403(t *testing.T) {
	for _, rt := range brRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newBrServer(t)
			s.grant(brCommuneA, "admin.org")
			// (1) granted in A; a principal of B at B's host.
			brWant(t, s.call(rt.method, brHostB, rt.path, rt.body, brStaff(brCommuneB)), http.StatusForbidden)
			// (2) A's principal presented at B's host — the token's commune is not the Host's: 401, by
			// rule 1 invariant 8 (core/authz), stricter than the 403 of case (1).
			brWant(t, s.call(rt.method, brHostB, rt.path, rt.body, brStaff(brCommuneA)), http.StatusUnauthorized)
			if s.acts.calls != 0 {
				t.Error("a permission granted in commune A acted in commune B")
			}
		})
	}
}

func TestBrandingRightPermissionRightCommuneSucceedsInThatCommune(t *testing.T) {
	for _, rt := range brRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newBrServer(t)
			s.grant(brCommuneB, "admin.org")
			brWant(t, s.call(rt.method, brHostB, rt.path, rt.body, brStaff(brCommuneB)), rt.ok)
			if s.acts.calls != 1 || s.acts.tenant != brCommuneB {
				t.Fatalf("calls=%d commune=%q, want one call in commune B (from Host)", s.acts.calls, s.acts.tenant)
			}
			// RULE 6, INVARIANT 8: the actor is the BUSINESS CODE, never Principal.ID.
			if rt.method != http.MethodGet && s.acts.actor.ID != "CB-00123" {
				t.Errorf("actor = %q, want CB-00123", s.acts.actor.ID)
			}
			if strings.Contains(rt.path, "banner") && s.acts.img != domain.BrandingBanner ||
				strings.Contains(rt.path, "logo") && s.acts.img != domain.BrandingLogo {
				t.Errorf("route %s acted on image %q", rt.path, s.acts.img)
			}
		})
	}
}

func TestBrandingEmptyBusinessCodeRefusesTheWrite(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	p := brStaff(brCommuneA)
	p.Ma = ""
	brWant(t, s.call(http.MethodDelete, brHostA, "/api/v1/commune-branding/logo", "", p), http.StatusInternalServerError)
	if s.acts.calls != 0 {
		t.Error("wrote with no business code — would have fallen back to the internal id")
	}
}

func TestBrandingRefusalsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"wrong mime declared", app.ErrBrandingTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"over 2 MB declared", app.ErrBrandingTooLarge, http.StatusBadRequest, "invalid_request"},
		{"malware", &app.BrandingRejection{Reason: app.BrandingRejectMalware}, http.StatusUnprocessableEntity, "image_rejected"},
		{"soft-deleted profile", domain.ErrProfileDeleted, http.StatusConflict, "profile_deleted"},
		{"another commune's / officer's file", app.ErrBrandingFileNotFound, http.StatusNotFound, "not_found"},
		{"scanner down", app.ErrBrandingScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrBrandingUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"publish down", app.ErrBrandingPublishUnavailable, http.StatusServiceUnavailable, "publish_unavailable"},
		{"decode slot busy", app.ErrBrandingDecodeBusy, http.StatusServiceUnavailable, "image_processing_busy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newBrServer(t)
			s.grant(brCommuneA, "admin.org")
			s.acts.err = tc.err
			w := s.call(http.MethodPost, brHostA, "/api/v1/commune-branding/logo-uploads/01JFILE0000000000000000000/completion",
				"", brStaff(brCommuneA))
			brWant(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("body: %s", w.Body.String())
			}
		})
	}
}

func TestBrandingUploadReplyIsNotCached(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	w := s.call(http.MethodPost, brHostA, "/api/v1/commune-branding/logo-uploads", brUploadBody, brStaff(brCommuneA))
	brWant(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("a presigned form must not be cached: %q", w.Header().Get("Cache-Control"))
	}
}

func TestBrandingRegisterRefusesMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Register with no Checker did not panic")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}
