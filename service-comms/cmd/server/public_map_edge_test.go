package main

// THE MINI APP MAP ROUTES ON THE OUTER SPLIT (ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M1, M2): the
// public map frame rides the public chain; the basemap rides its OWN public chain, whose CORS answer lets
// a range reader through — and only that chain does.

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

// fakeMapFrame is the effective-frame read, asserting it is asked inside a commune.
type fakeMapFrame struct{}

func (fakeMapFrame) Get(ctx context.Context) (commsapp.MapFrameView, error) {
	tenant.MustFrom(ctx)
	return commsapp.MapFrameView{Configured: true, Source: "commune",
		Frame:  domain.MapFrame{CenterLat: 16.05, CenterLng: 108.2, RadiusKm: 5},
		Bounds: domain.MapFrameBounds{MinLng: 108.1, MinLat: 16.0, MaxLng: 108.3, MaxLat: 16.1}}, nil
}

// memCounterForTest counts in memory, one never-ending window. The real basemap policy FAILS CLOSED, so
// the dev wiring's unavailableCounter would answer every request 503.
type memCounterForTest struct {
	mu sync.Mutex
	n  map[string]int64
}

func (m *memCounterForTest) Incr(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		m.n = map[string]int64{}
	}
	m.n[key]++
	return m.n[key], window, nil
}

// newBasemapEdgeForTest is the real basemap chain (range CORS) around the real basemap route.
// storageURL "" leaves BASEMAP_URL unset.
func newBasemapEdgeForTest(t *testing.T, storageURL string, client ...*http.Client) http.Handler {
	t.Helper()
	origins, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	lim, err := ratelimit.New(&memCounterForTest{}, ratelimit.PublicBasemapRead)
	if err != nil {
		t.Fatal(err)
	}
	c := http.DefaultClient
	if len(client) > 0 {
		c = client[0]
	}
	mux := http.NewServeMux()
	svchttp.RegisterPublicBasemap(mux, svchttp.PublicBasemapDeps{
		Xa: xaTheoHostThu{}, BaseURL: storageURL, Client: c, Limiter: lim, Log: slog.New(slog.DiscardHandler),
	})
	return dungBienCongKhai(mux, origins, slog.New(slog.DiscardHandler), httpx.CORSRangeReads())
}

// The public map frame rides the public chain on the reserved API host; the STAFF singleton `map-frame`
// does not.
func TestPublicMapFrameOnPublicChain(t *testing.T) {
	h, cb := ngoaiThu(t)
	w := goiApi(h, http.MethodGet, svchttp.PublicMapFramePath+"?host=xa-a.vigov.vn", nguonMiniAppThu)
	if w.Code != http.StatusOK {
		t.Fatalf("public map frame: %d — body: %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got !=
		`{"configured":true,"center_lat":16.05,"center_lng":108.2,"radius_km":5,"bounds":[108.1,16,108.3,16.1]}` {
		t.Fatalf("body = %s", got)
	}
	if cb.goi != 0 {
		t.Fatal("the staff chain was reached")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != nguonMiniAppThu {
		t.Fatalf("no ACAO: %v", w.Header())
	}
	for _, p := range []string{"/api/v1/map-frame", svchttp.PublicMapFramePath + "/x", svchttp.PublicMapFramePath + "x"} {
		h, _ := ngoaiThu(t)
		if w := goiApi(h, http.MethodGet, p+"?host=xa-a.vigov.vn", ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404 from the staff chain's TenantMiddleware", p, w.Code)
		}
	}
}

// The basemap rides its OWN public chain: no session, a 206 passed through, and the range CORS answer —
// Range/If-* allowed at the preflight, Content-Range/ETag exposed — which the news routes do NOT get.
func TestPublicBasemapOnItsOwnPublicChain(t *testing.T) {
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		http.ServeContent(w, r, "x", time.Time{}, strings.NewReader("0123456789"))
	}))
	defer storage.Close()
	cb := &canBoDanhDauComms{}
	h := dungBien(cb, dungCongKhaiThu(t), newBasemapEdgeForTest(t, storage.URL+"/b/map/", storage.Client()),
		newCitizenEdgeForTest(t), thuMucGia{}, canBoXaA(), nil, slog.New(slog.DiscardHandler))

	r := httptest.NewRequest(http.MethodGet,
		"https://"+hostApiComms+svchttp.PublicBasemapPath+"vn-mainland.pmtiles?host=xa-a.vigov.vn", nil)
	r.Host = hostApiComms
	r.Header.Set("Origin", nguonMiniAppThu)
	r.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("basemap range on the reserved host: %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	if cb.goi != 0 {
		t.Fatal("the staff chain was reached")
	}
	expose := w.Header().Get("Access-Control-Expose-Headers")
	if w.Header().Get("Access-Control-Allow-Origin") != nguonMiniAppThu ||
		!strings.Contains(expose, "Content-Range") || !strings.Contains(expose, "ETag") {
		t.Fatalf("range CORS answer missing: %v", w.Header())
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("Allow-Credentials on the basemap")
	}

	// Preflight with the range headers: granted to a Mini App origin, nothing to another.
	preflight := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodOptions, "https://"+hostApiComms+svchttp.PublicBasemapPath+"vn-mainland.pmtiles", nil)
		r.Host = hostApiComms
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", "GET")
		r.Header.Set("Access-Control-Request-Headers", "range,if-match")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	ok := preflight("https://mini.zalo.me")
	allow := ok.Header().Get("Access-Control-Allow-Headers")
	if ok.Code != http.StatusNoContent || ok.Header().Get("Access-Control-Allow-Origin") != "https://mini.zalo.me" ||
		!strings.Contains(allow, "Range") || !strings.Contains(allow, "If-Match") {
		t.Fatalf("allowed preflight: %d %v", ok.Code, ok.Header())
	}
	if bad := preflight("https://evil.example"); bad.Header().Get("Access-Control-Allow-Origin") != "" ||
		bad.Header().Get("Access-Control-Allow-Headers") != "" {
		t.Fatalf("a foreign origin got a CORS grant: %v", bad.Header())
	}

	// The news chain does NOT get the range grant.
	news := goiApi(h, http.MethodGet, svchttp.MauTinXa+"?host=xa-a.vigov.vn", nguonMiniAppThu)
	if news.Header().Get("Access-Control-Expose-Headers") != "" {
		t.Fatalf("the news route exposes headers: %v", news.Header())
	}
	newsPre := goiApi(h, http.MethodOptions, svchttp.MauTinXa, "https://mini.zalo.me")
	if strings.Contains(newsPre.Header().Get("Access-Control-Allow-Headers"), "Range") {
		t.Fatalf("the news preflight grants Range: %v", newsPre.Header())
	}
}

// Unset BASEMAP_URL through the real chain: 503 and the one sentence — never a fallback host. The bare
// collection is no file.
func TestPublicBasemapUnsetThroughTheEdge(t *testing.T) {
	cb := &canBoDanhDauComms{}
	h := dungBien(cb, dungCongKhaiThu(t), newBasemapEdgeForTest(t, ""), newCitizenEdgeForTest(t), thuMucGia{},
		canBoXaA(), nil, slog.New(slog.DiscardHandler))
	w := goiApi(h, http.MethodGet, svchttp.PublicBasemapPath+"vn-mainland.pmtiles?host=xa-a.vigov.vn", nguonMiniAppThu)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Chưa cấu hình bản đồ nền.") {
		t.Fatalf("unset: %d %s", w.Code, w.Body.String())
	}
	// ServeMux redirects the bare collection to the slash form, which names no file.
	if w := goiApi(h, http.MethodGet, "/api/v1/commune-basemap?host=xa-a.vigov.vn", ""); w.Code != http.StatusTemporaryRedirect &&
		w.Code != http.StatusMovedPermanently {
		t.Fatalf("bare /api/v1/commune-basemap: %d, want a redirect to the subtree", w.Code)
	}
	if cb.goi != 0 {
		t.Fatal("the staff marker was reached")
	}
}
