package main

// buildCommuneEdge is the chain THIS binary installs in front of the commune-host routes. core/staffauth
// proves its middleware; only a test of this function sees that platform actually mounts it — without
// it every staff request to the branding routes answers 401 and nothing else turns red.

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/app"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
)

type staffResolverFake struct {
	keys  []authz.Perm
	calls int
}

func (r *staffResolverFake) ResolveStaff(ctx context.Context, token, _ string) (staffauth.StaffPrincipal, bool, error) {
	r.calls++
	if token != "valid-session" {
		return staffauth.StaffPrincipal{}, false, nil
	}
	return staffauth.StaffPrincipal{StaffID: "01JINTERNAL0000000000000000", Ma: "CB-00123", PermissionKeys: r.keys}, true, nil
}

type brandingActsFake struct {
	commune tenant.ID
	// uploaded: the bytes Upload read off the stream, through the real edge.
	uploaded []byte
}

func (b *brandingActsFake) Settings(ctx context.Context) (app.BrandingSettings, error) {
	b.commune = tenant.MustFrom(ctx)
	return app.BrandingSettings{}, nil
}
func (b *brandingActsFake) MaxUploadBytes(context.Context, domain.BrandingImage) (int64, error) {
	return 2 << 20, nil
}
func (b *brandingActsFake) Upload(ctx context.Context, _ domain.BrandingImage, req app.BrandingUploadRequest, _ audit.Actor) (domain.StoredFile, error) {
	b.commune = tenant.MustFrom(ctx)
	raw, err := io.ReadAll(io.LimitReader(req.Body, req.Size))
	if err != nil {
		return domain.StoredFile{}, err
	}
	b.uploaded = raw
	return domain.StoredFile{ID: "01JFILE0000000000000000000"}, req.Received()
}
func (b *brandingActsFake) Remove(context.Context, domain.BrandingImage, audit.Actor) (bool, error) {
	return false, nil
}

type noURLs struct{}

func (noURLs) PublicURL(string) string { return "" }

func communeEdgeForTest(res staffauth.Resolver, acts svchttp.BrandingActs) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{Checker: staffauth.Checker{}, Branding: acts, URLs: noURLs{},
		Uploads: httpx.NewUploadSlots(httpx.UploadSlotsPerPod), Log: log})
	return buildCommuneEdge(mux, directoryOneCommune{}, res, nil, log)
}

func getBranding(h http.Handler, cookie string) int {
	r := httptest.NewRequest(http.MethodGet, "https://"+hostThu+"/api/v1/commune-branding", nil)
	r.Host = hostThu
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestCommuneEdgeResolvesTheStaffSessionAndChecksThePermission(t *testing.T) {
	acts := &brandingActsFake{}
	res := &staffResolverFake{keys: []authz.Perm{"admin.org"}}
	h := communeEdgeForTest(res, acts)

	if got := getBranding(h, ""); got != http.StatusUnauthorized {
		t.Errorf("no cookie: %d, want 401", got)
	}
	if got := getBranding(h, "stale"); got != http.StatusUnauthorized {
		t.Errorf("unusable session: %d, want 401", got)
	}
	if got := getBranding(h, "valid-session"); got != http.StatusOK {
		t.Fatalf("admin.org holder: %d, want 200 — staffauth is not mounted, or mounted outside the tenant edge", got)
	}
	if acts.commune == "" || res.calls != 2 {
		t.Errorf("commune=%q resolver calls=%d", acts.commune, res.calls)
	}

	res.keys = []authz.Perm{"admin.lookup"}
	if got := getBranding(h, "valid-session"); got != http.StatusForbidden {
		t.Errorf("without admin.org: %d, want 403", got)
	}
}

// The upload route through the REAL edge: a multipart body passes every layer of the chain untouched
// (nothing on it reads or wraps the body), and the stream reaches the use case whole.
func TestCommuneEdgeCarriesAMultipartUploadToTheUseCase(t *testing.T) {
	acts := &brandingActsFake{}
	h := communeEdgeForTest(&staffResolverFake{keys: []authz.Perm{"admin.org"}}, acts)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("size", "4")
	part, _ := mw.CreateFormFile("file", "logo.png")
	_, _ = part.Write([]byte("abcd"))
	_ = mw.Close()

	post := func(cookie string) int {
		r := httptest.NewRequest(http.MethodPost, "https://"+hostThu+"/api/v1/commune-branding/logo-uploads",
			bytes.NewReader(buf.Bytes()))
		r.Host = hostThu
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("Idempotency-Key", "01JEDGEUPLOADKEY0000000000")
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if got := post(""); got != http.StatusUnauthorized {
		t.Errorf("no cookie: %d, want 401", got)
	}
	if got := post("valid-session"); got != http.StatusCreated {
		t.Fatalf("admin.org holder: %d, want 201", got)
	}
	if string(acts.uploaded) != "abcd" || acts.commune == "" {
		t.Errorf("uploaded=%q commune=%q — the stream did not reach the use case whole, in the Host commune",
			acts.uploaded, acts.commune)
	}
}
