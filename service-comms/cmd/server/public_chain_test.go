package main

// THE PUBLIC EDGE OF THIS BINARY — the wiring, not the middleware (core/httpx proves that).
//
// What only this file can see: that buildEdge routes /api/v1/commune-news and its subtree to the PUBLIC
// chain on the reserved API host — with no session and no Host resolution — and leaves every staff
// route on the staff chain; and that buildPublicChain really installs CORS. A pattern dropped or
// widened here leaves a service that starts and serves, and either 404s every resident or puts a staff
// route outside TenantMiddleware.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

const (
	commsAPIHost      = "comms.api.vigov.vn" // reserved, maps to no commune (ADR 0046)
	testMiniAppOrigin = "https://h5.zdn.vn"
)

type fakeTenantByHost struct{}

func (fakeTenantByHost) XaTheoHost(context.Context, string) (tenant.Tenant, bool, error) {
	return tenant.Tenant{ID: tenantA, Name: "Xã A", Active: true}, true, nil
}

type fakePublicContent struct{}

func (fakePublicContent) ListPublic(ctx context.Context, _ domain.ContentType, _ page.Request) (page.Result[domain.ContentItem], error) {
	tenant.MustFrom(ctx)
	return page.NewResult[domain.ContentItem](), nil
}

func (fakePublicContent) PublicByID(ctx context.Context, id string) (domain.ContentItem, error) {
	tenant.MustFrom(ctx)
	return domain.ContentItem{ID: id, Title: "Tin", Status: domain.ContentStatusVisible}, nil
}

type fakeCategories struct{}

func (fakeCategories) List(context.Context) ([]domain.ContentCategory, error) { return nil, nil }

// newTestPublicChain is the real public chain around the real public routes, with fake reads.
func newTestPublicChain(t *testing.T) http.Handler {
	t.Helper()
	origins, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	mux := http.NewServeMux()
	svchttp.RegisterPublic(mux, svchttp.PublicDeps{
		Tenants: fakeTenantByHost{}, ContentItems: fakePublicContent{}, Categories: fakeCategories{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return buildPublicChain(mux, origins)
}

// staffChainMarker stands in for the staff chain: it only records that it was reached.
type staffChainMarker struct{ calls int }

func (c *staffChainMarker) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.calls++
	w.WriteHeader(http.StatusTeapot)
}

// newTestOuter builds the outer split with the staff mux replaced by a marker. buildEdge wraps the
// marker in the real staff chain, so a staff path reaching it on the reserved host is answered 404 by
// TenantMiddleware — which is itself the proof that it went down the staff chain.
func newTestOuter(t *testing.T) (http.Handler, *staffChainMarker) {
	t.Helper()
	marker := &staffChainMarker{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return buildEdge(marker, newTestPublicChain(t), fakeDirectory{}, staffOfTenantA(), nil, log), marker
}

func callAPIHost(h http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+commsAPIHost+path, nil)
	r.Host = commsAPIHost
	if origin != "" {
		r.Header.Set("Origin", origin)
		if method == http.MethodOptions {
			r.Header.Set("Access-Control-Request-Method", "GET")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCommuneNewsGoesDownPublicChainWithoutSession(t *testing.T) {
	for _, p := range []string{svchttp.CommuneNewsPath, svchttp.CommuneNewsPath + "/01JTIN0000000000000000000A"} {
		h, marker := newTestOuter(t)
		if w := callAPIHost(h, http.MethodGet, p+"?host=xa-a.vigov.vn", ""); w.Code != http.StatusOK {
			t.Fatalf("%s trên host API dành riêng, không phiên: mã = %d, muốn 200 — thân: %s", p, w.Code, w.Body.String())
		}
		if marker.calls != 0 {
			t.Fatalf("%s: chuỗi cán bộ bị gọi", p)
		}
	}
}

func TestStaffRoutesDoNotSlipOntoPublicChain(t *testing.T) {
	// On the reserved host the staff chain answers 404 at TenantMiddleware — the marker behind it is
	// never reached. A staff path answered 200 here would mean it went down the public chain.
	for _, p := range []string{"/api/v1/content-items", "/api/v1/content-items/x", "/api/v1/content-categories",
		"/api/v1/commune-newsx", "/api/v1/map-asset-types"} {
		h, _ := newTestOuter(t)
		if w := callAPIHost(h, http.MethodGet, p+"?host=xa-a.vigov.vn", ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: mã = %d, muốn 404 từ TenantMiddleware của chuỗi cán bộ", p, w.Code)
		}
	}
}

func TestCommuneNewsPreflightIs204AndCORSOnlyOnPublicChain(t *testing.T) {
	h, _ := newTestOuter(t)
	w := callAPIHost(h, http.MethodOptions, svchttp.CommuneNewsPath, "https://mini.zalo.me")
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://mini.zalo.me" {
		t.Fatalf("preflight: mã = %d, ACAO = %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
	w = callAPIHost(h, http.MethodGet, svchttp.CommuneNewsPath+"?host=xa-a.vigov.vn", testMiniAppOrigin)
	if w.Header().Get("Access-Control-Allow-Origin") != testMiniAppOrigin {
		t.Fatalf("GET công khai thiếu ACAO: %v", w.Header())
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("tuyến công khai cấp Allow-Credentials")
	}
	w = callAPIHost(h, http.MethodGet, "/api/v1/content-items", testMiniAppOrigin)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("tuyến cán bộ mang header CORS: %v", w.Header())
	}
}
