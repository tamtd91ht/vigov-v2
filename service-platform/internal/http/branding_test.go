package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403 right
// permission WRONG COMMUNE · 2xx both correct — for ALL FIVE commune-branding routes, through the REAL
// Register behind the real edge order (StripTenantHeaders → Recover → TenantMiddleware → idem →
// principal). Plus the refusal mapping, the upload transport (ADR 0052 §Sửa đổi 09/10/2026: one
// multipart request, a per-pod slot cap) and the actor (CB- code, never the internal id).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
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
	// maxBytes is MaxUploadBytes' answer (0 = 2 MB); maxCalls counts it apart from calls, which counts
	// the acts that DO something.
	maxBytes int64
	maxCalls int
	// the last upload as the use case received it
	upload   app.BrandingUploadRequest
	received []byte
}

func (a *brActs) note(ctx context.Context, img domain.BrandingImage, actor audit.Actor) {
	a.calls++
	a.tenant, a.img, a.actor = tenant.MustFrom(ctx), img, actor
}
func (a *brActs) Settings(ctx context.Context) (app.BrandingSettings, error) {
	a.note(ctx, "", audit.Actor{})
	return app.BrandingSettings{LogoPublicURL: "https://media.example/x.png"}, a.err
}
func (a *brActs) MaxUploadBytes(context.Context, domain.BrandingImage) (int64, error) {
	a.maxCalls++
	if a.maxBytes == 0 {
		return 2 << 20, nil
	}
	return a.maxBytes, nil
}

// Upload reads the stream the way core/storage PutUpload does — exactly Size bytes — then confirms the
// transport, so the handler's wiring of Body / Received is exercised for real.
func (a *brActs) Upload(ctx context.Context, img domain.BrandingImage, req app.BrandingUploadRequest,
	actor audit.Actor) (domain.StoredFile, error) {
	a.note(ctx, img, actor)
	a.upload = req
	if a.err != nil {
		return domain.StoredFile{}, a.err
	}
	b, err := io.ReadAll(io.LimitReader(req.Body, req.Size))
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("storage: read: %w", err)
	}
	a.received = b
	if err := req.Received(); err != nil {
		return domain.StoredFile{}, err
	}
	return domain.StoredFile{ID: "01JFILE0000000000000000000", Status: domain.StoredFileReady,
		MIMEType: storage.MIMEPNG, SizeBytes: int64(len(b)), PublicObjectKey: "public-media/k"}, nil
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
	slots   *httpx.UploadSlots
	logs    *bytes.Buffer
}

type brPrincipalKey struct{}

func newBrServer(t *testing.T) *brServer {
	t.Helper()
	acts := &brActs{}
	checker := &brChecker{grants: map[tenant.ID]map[authz.Perm]bool{}}
	logs := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	slots := httpx.NewUploadSlots(httpx.UploadSlotsPerPod)
	mux := http.NewServeMux()
	Register(mux, Deps{Checker: checker, Branding: acts, URLs: brURLs{}, Uploads: slots, Log: log})

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
	return &brServer{h: h, acts: acts, checker: checker, slots: slots, logs: logs}
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
	ctype := ""
	if method != http.MethodGet {
		ctype = "application/json"
	}
	return s.send(method, host, path, ctype, body, p)
}

// upload POSTs a multipart body to an upload route.
func (s *brServer) upload(host, path string, mp brPart, p *authz.Principal) *httptest.ResponseRecorder {
	body, ctype := mp.encode()
	return s.send(http.MethodPost, host, path, ctype, body, p)
}

func (s *brServer) send(method, host, path, ctype, body string, p *authz.Principal) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if method != http.MethodGet {
		r.Header.Set(idem.Header, "01JBRANDINGKEY000000000000")
	}
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), brPrincipalKey{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

// brPart is one multipart upload body: text fields in order, then the file part.
type brPart struct {
	fields   [][2]string
	filename string
	ctype    string
	content  []byte
}

func (mp brPart) encode() (string, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range mp.fields {
		_ = w.WriteField(f[0], f[1])
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+mp.filename+`"`)
	h.Set("Content-Type", mp.ctype)
	part, _ := w.CreatePart(h)
	_, _ = part.Write(mp.content)
	_ = w.Close()
	return buf.String(), w.FormDataContentType()
}

// brPNG is a small, well-formed upload: size, file_name, then the file.
func brPNG() brPart {
	content := []byte("\x89PNG\r\n\x1a\n-not-really-but-the-fake-acts-never-decode")
	return brPart{fields: [][2]string{{"size", strconv.Itoa(len(content))}, {"file_name", "logo xã.png"}},
		filename: "IMG_0001.png", ctype: "image/png", content: content}
}

type brRoute struct {
	name, method, path string
	upload             bool
	ok                 int
}

func brRoutes() []brRoute {
	return []brRoute{
		{"settings", http.MethodGet, "/api/v1/commune-branding", false, http.StatusOK},
		{"logo upload", http.MethodPost, "/api/v1/commune-branding/logo-uploads", true, http.StatusCreated},
		{"logo removal", http.MethodDelete, "/api/v1/commune-branding/logo", false, http.StatusNoContent},
		{"banner upload", http.MethodPost, "/api/v1/commune-branding/banner-uploads", true, http.StatusCreated},
		{"banner removal", http.MethodDelete, "/api/v1/commune-branding/banner", false, http.StatusNoContent},
	}
}

func (s *brServer) route(rt brRoute, host string, p *authz.Principal) *httptest.ResponseRecorder {
	if rt.upload {
		return s.upload(host, rt.path, brPNG(), p)
	}
	return s.call(rt.method, host, rt.path, "", p)
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
			brWant(t, s.route(rt, brHostA, nil), http.StatusUnauthorized)
			if s.acts.calls != 0 || s.acts.maxCalls != 0 {
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
			brWant(t, s.route(rt, brHostA, brStaff(brCommuneA)), http.StatusForbidden)
			if s.acts.calls != 0 || s.acts.maxCalls != 0 {
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
			brWant(t, s.route(rt, brHostB, brStaff(brCommuneB)), http.StatusForbidden)
			// (2) A's principal presented at B's host — the token's commune is not the Host's: 401, by
			// rule 1 invariant 8 (core/authz), stricter than the 403 of case (1).
			brWant(t, s.route(rt, brHostB, brStaff(brCommuneA)), http.StatusUnauthorized)
			if s.acts.calls != 0 || s.acts.maxCalls != 0 {
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
			brWant(t, s.route(rt, brHostB, brStaff(brCommuneB)), rt.ok)
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
		{"store refuses the declared type", fmt.Errorf("wrapped: %w", storage.ErrTypeNotAllowed), http.StatusBadRequest, "invalid_request"},
		{"over the policy, declared", app.ErrBrandingTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"over the cap, at the store", fmt.Errorf("nhận diện xã: tệp X: %w", storage.ErrTooLarge), http.StatusRequestEntityTooLarge, "file_too_large"},
		{"stream shorter than declared", fmt.Errorf("x: %w: %w", storage.ErrInvalidArgument, storage.ErrSizeMismatch), http.StatusBadRequest, "invalid_upload"},
		{"client too slow, through the store", fmt.Errorf("storage: read: %w", httpx.ErrUploadTimeout), http.StatusRequestTimeout, "upload_timeout"},
		{"malformed body, through the store", fmt.Errorf("storage: read: %w", httpx.ErrUploadMalformed), http.StatusBadRequest, "invalid_upload"},
		{"malware", &app.BrandingRejection{Reason: app.BrandingRejectMalware}, http.StatusUnprocessableEntity, "image_rejected"},
		{"malware, wrapped with the file id", fmt.Errorf("nhận diện xã: tệp X: %w", &app.BrandingRejection{Reason: app.BrandingRejectMalware}), http.StatusUnprocessableEntity, "image_rejected"},
		{"soft-deleted profile", domain.ErrProfileDeleted, http.StatusConflict, "profile_deleted"},
		{"file limit", app.ErrBrandingCountReached, http.StatusConflict, "upload_limit"},
		{"row taken by a concurrent writer", app.ErrBrandingNotPending, http.StatusConflict, "upload_state"},
		{"another commune's / officer's file", app.ErrBrandingFileNotFound, http.StatusNotFound, "not_found"},
		{"scanner down", app.ErrBrandingScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrBrandingUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"publish down", app.ErrBrandingPublishUnavailable, http.StatusServiceUnavailable, "publish_unavailable"},
		{"decode slot busy", app.ErrBrandingDecodeBusy, http.StatusServiceUnavailable, "image_processing_busy"},
		{"anything else", errors.New("storage: put upload: connection refused"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newBrServer(t)
			s.grant(brCommuneA, "admin.org")
			s.acts.err = tc.err
			w := s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", brPNG(), brStaff(brCommuneA))
			brWant(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("body: %s", w.Body.String())
			}
		})
	}
}

func TestBrandingUploadHandsTheDeclarationAndTheStreamToTheUseCase(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	mp := brPNG()
	mp.ctype = "image/PNG; foo=bar" // parameters and case are a client's habit, not a different type
	w := s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", mp, brStaff(brCommuneA))
	brWant(t, w, http.StatusCreated)
	up := s.acts.upload
	if up.FileName != "logo xã.png" || up.ContentType != "image/png" || up.Size != int64(len(mp.content)) {
		t.Errorf("declaration = %q %q %d, want the file_name field, image/png, %d", up.FileName, up.ContentType,
			up.Size, len(mp.content))
	}
	if !bytes.Equal(s.acts.received, mp.content) {
		t.Error("the use case did not receive the file's bytes")
	}
	if up.Deadline.IsZero() || time.Until(up.Deadline) > httpx.UploadReadTimeout {
		t.Errorf("deadline = %v, want ReadUpload's (≤ %v from now)", up.Deadline, httpx.UploadReadTimeout)
	}
	// The reply is the stored file — never an upload form.
	body := w.Body.String()
	for _, want := range []string{`"id":"01JFILE0000000000000000000"`, `"status":"ready"`, `"public_url":"https://media.example/public-media/k"`} {
		if !strings.Contains(body, want) {
			t.Errorf("reply lacks %s: %s", want, body)
		}
	}
	if strings.Contains(body, `"upload"`) || strings.Contains(body, "minio") {
		t.Errorf("the reply still carries an upload form: %s", body)
	}

	// No file_name field: the file part's own filename is kept.
	s2 := newBrServer(t)
	s2.grant(brCommuneA, "admin.org")
	mp = brPNG()
	mp.fields = mp.fields[:1]
	brWant(t, s2.upload(brHostA, "/api/v1/commune-branding/banner-uploads", mp, brStaff(brCommuneA)), http.StatusCreated)
	if s2.acts.upload.FileName != "IMG_0001.png" || s2.acts.img != domain.BrandingBanner {
		t.Errorf("file name = %q image = %q, want the part's filename on the banner", s2.acts.upload.FileName, s2.acts.img)
	}
}

func TestBrandingUploadWhenEverySlotIsTakenIs503AndReadsNothing(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	var releases []func()
	for i := 0; i < httpx.UploadSlotsPerPod; i++ {
		release, ok := s.slots.Acquire(context.Background())
		if !ok {
			t.Fatalf("slot %d not available", i)
		}
		releases = append(releases, release)
	}
	w := s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", brPNG(), brStaff(brCommuneA))
	brWant(t, w, http.StatusServiceUnavailable)
	if !strings.Contains(w.Body.String(), `"code":"upload_busy"`) || w.Header().Get("Retry-After") == "" {
		t.Errorf("busy reply = %s, Retry-After %q", w.Body.String(), w.Header().Get("Retry-After"))
	}
	if s.acts.calls != 0 || s.acts.maxCalls != 0 {
		t.Error("a request without a slot reached the use case")
	}
	if !strings.Contains(s.logs.String(), "ma_loi=upload_busy") {
		t.Errorf("the busy refusal left no log line: %s", s.logs.String())
	}
	// The slot is the PROCESS's: once one frees, the next upload goes through — and returns its slot.
	releases[0]()
	brWant(t, s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", brPNG(), brStaff(brCommuneA)), http.StatusCreated)
	if _, ok := s.slots.Acquire(context.Background()); !ok {
		t.Error("a finished upload did not release its slot")
	}
}

func TestBrandingUploadDeclaredOverThePolicyIs413BeforeTheUseCase(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	s.acts.maxBytes = 16
	w := s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", brPNG(), brStaff(brCommuneA))
	brWant(t, w, http.StatusRequestEntityTooLarge)
	if !strings.Contains(w.Body.String(), `"code":"file_too_large"`) {
		t.Errorf("body: %s", w.Body.String())
	}
	if s.acts.calls != 0 {
		t.Error("an over-cap upload reached the use case")
	}
	if !strings.Contains(s.logs.String(), "level=INFO") || !strings.Contains(s.logs.String(), "ma_loi=file_too_large") {
		t.Errorf("the 413 left no INFO line: %s", s.logs.String())
	}
}

func TestBrandingUploadThatIsNotMultipartIs415(t *testing.T) {
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	w := s.call(http.MethodPost, brHostA, "/api/v1/commune-branding/logo-uploads",
		`{"file_name":"logo.png","content_type":"image/png","size":2048}`, brStaff(brCommuneA))
	brWant(t, w, http.StatusUnsupportedMediaType)
	if s.acts.calls != 0 {
		t.Error("the old JSON request reached the use case")
	}
}

func TestBrandingUploadRefusesAFieldItDoesNotDeclare(t *testing.T) {
	// The old JSON declaration's `content_type` is not a field any more: the declared type is the file
	// part's own header. An undeclared field is a malformed upload, never silently ignored.
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	mp := brPNG()
	mp.fields = append(mp.fields, [2]string{"content_type", "image/png"})
	brWant(t, s.upload(brHostA, "/api/v1/commune-branding/logo-uploads", mp, brStaff(brCommuneA)), http.StatusBadRequest)
	if s.acts.calls != 0 {
		t.Error("a malformed upload reached the use case")
	}
}

func TestBrandingCompletionRoutesAreGone(t *testing.T) {
	// ADR 0052 §Sửa đổi 09/10/2026: "bỏ hẳn các tuyến …/completion cũ". Not 401, not 403 — not there.
	s := newBrServer(t)
	s.grant(brCommuneA, "admin.org")
	for _, path := range []string{
		"/api/v1/commune-branding/logo-uploads/01JFILE0000000000000000000/completion",
		"/api/v1/commune-branding/banner-uploads/01JFILE0000000000000000000/completion",
	} {
		brWant(t, s.call(http.MethodPost, brHostA, path, "", brStaff(brCommuneA)), http.StatusNotFound)
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
