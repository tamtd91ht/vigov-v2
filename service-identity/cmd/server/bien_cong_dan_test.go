package main

// THE PUBLIC EDGE OF THIS BINARY — the wiring, not the middleware (core/httpx proves that).
//
// The file name is historical: until 2026-09-27 this edge was the CITIZEN chain. The owner then made
// both Mini App routes Public, and dungBienCongKhai replaced dungBienCongDan.
//
// What only this file can see: that dungNgoai routes GET /api/v1/communes and GET
// /api/v1/commune-staff to the PUBLIC chain on the reserved API host, and leaves
// /api/v1/communes/current, /api/v1/staff and /api/v1/staff-directory on the STAFF chain; and that
// dungBienCongKhai really installs CORS. A pattern changed from exact to subtree, or a middleware
// deleted, leaves a service that starts and serves — and either 404s the Mini App on the reserved
// host or puts a staff route outside Host resolution. Nothing else in the repository turns red.

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	svchttp "github.com/vihat/vigov/service-identity/internal/http"
)

const (
	hostApiIdentity = "identity.api.vigov.vn" // reserved, maps to no commune (ADR 0046)
	nguonMiniAppThu = "https://h5.zdn.vn"
)

type xaTheoHostThu struct{}

func (xaTheoHostThu) XaTheoHost(context.Context, string) (tenant.Tenant, bool, error) {
	return tenant.Tenant{ID: ulidThu, Name: "Xã Thử", Active: true}, true, nil
}

type danhBaThu struct{}

func (danhBaThu) DanhBaCongKhai(ctx context.Context) ([]domain.CanBoCongKhai, error) {
	tenant.MustFrom(ctx) // the handler must have resolved the commune before reading
	return nil, nil
}

type profileStub struct{}

func (profileStub) TenantProfile(ctx context.Context) (platformclient.TenantProfile, bool, error) {
	tenant.MustFrom(ctx) // the handler must have resolved the commune before reading
	return platformclient.TenantProfile{}, false, nil
}

// signInStub answers every own-app sign-in with 503: what is under test here is only WHICH chain the
// path reaches, and an answer from the public mux proves it.
type signInStub struct{}

func (signInStub) SignIn(context.Context, app.OwnAppSignInRequest) (app.KetQuaMoPhienCau, error) {
	return app.KetQuaMoPhienCau{}, app.ErrOwnAppUnavailable
}

// canBoDanhDau stands in for the whole staff chain: it only records that it was reached.
type canBoDanhDau struct{ goi int }

func (c *canBoDanhDau) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.goi++
	w.WriteHeader(http.StatusTeapot)
}

func dungNgoaiThu(t *testing.T) (http.Handler, *canBoDanhDau) {
	t.Helper()
	nguon, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	mux := http.NewServeMux()
	svchttp.RegisterCongKhai(mux, svchttp.DepsCongKhai{Xa: xaTheoHostThu{}, DanhBa: danhBaThu{}, Profile: profileStub{},
		CitizenSessions: signInStub{}})
	cb := &canBoDanhDau{}
	return dungNgoai(cb, dungBienCongKhai(mux, nguon, slog.New(slog.DiscardHandler))), cb
}

func goiThu(h http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+hostApiIdentity+path, nil)
	r.Host = hostApiIdentity
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

var tuyenCongKhai = []string{svchttp.MauDanhMucXa, svchttp.MauDanhBaCongKhai, svchttp.CommuneProfilesPath}

func TestTuyenCongKhaiDiChuoiCongKhaiKhongCanPhien(t *testing.T) {
	for _, p := range tuyenCongKhai {
		h, cb := dungNgoaiThu(t)
		// No Authorization header at all: Public by owner decision 2026-09-27.
		if w := goiThu(h, http.MethodGet, p+"?host=xa-thu.vigov.vn", ""); w.Code != http.StatusOK {
			t.Fatalf("%s không có phiên: mã = %d, muốn 200 — thân: %s", p, w.Code, w.Body.String())
		}
		if cb.goi != 0 {
			t.Fatalf("%s: chuỗi cán bộ bị gọi %d lần cho tuyến công khai", p, cb.goi)
		}
	}
}

// POST /api/v1/citizen-sessions (ADR 0066) reaches the PUBLIC chain with no session, and the STAFF
// sign-in POST /api/v1/sessions — a different path on the same host — stays on the staff chain.
func TestCitizenSessionsOnPublicChain(t *testing.T) {
	h, cb := dungNgoaiThu(t)
	r := httptest.NewRequest(http.MethodPost, "https://"+hostApiIdentity+svchttp.CitizenSessionsPath,
		strings.NewReader(`{"appId":"1234567890123456789","accessToken":"FAKE-ACCESS","phoneToken":"FAKE-PHONE"}`))
	r.Host = hostApiIdentity
	r.Header.Set("Origin", nguonMiniAppThu)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || cb.goi != 0 {
		t.Fatalf("citizen-sessions: status %d, staff chain hit %d times — want the public mux's 503", w.Code, cb.goi)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != nguonMiniAppThu {
		t.Errorf("citizen-sessions: Access-Control-Allow-Origin = %q", got)
	}

	h, cb = dungNgoaiThu(t)
	goiThu(h, http.MethodPost, "/api/v1/sessions", "")
	if cb.goi != 1 {
		t.Fatalf("staff sign-in reached the staff chain %d times, want 1", cb.goi)
	}
	h, cb = dungNgoaiThu(t)
	goiThu(h, http.MethodPost, svchttp.CitizenSessionsPath+"/x", "")
	if cb.goi != 1 {
		t.Fatalf("a path under citizen-sessions must not reach the public chain (exact path only)")
	}
}

func TestTuyenCanBoVanOChuoiCanBo(t *testing.T) {
	// Sent through the public chain, the staff sign-in read would have no commune from `Host`, and the
	// register and the picker would have no staff session check at all.
	for _, path := range []string{"/api/v1/communes/current", "/api/v1/communes/", "/api/v1/staff",
		"/api/v1/staff-directory", "/api/v1/commune-staff/x", "/api/v1/commune-profiles/x"} {
		h, cb := dungNgoaiThu(t)
		goiThu(h, http.MethodGet, path, "")
		if cb.goi != 1 {
			t.Errorf("%s: chuỗi cán bộ gọi %d lần — muốn 1", path, cb.goi)
		}
	}
}

func TestPreflightTuyenCongKhai204(t *testing.T) {
	for _, p := range tuyenCongKhai {
		h, cb := dungNgoaiThu(t)
		w := goiThu(h, http.MethodOptions, p, "https://mini.zalo.me")
		if w.Code != http.StatusNoContent {
			t.Fatalf("%s preflight: mã = %d, muốn 204", p, w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://mini.zalo.me" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q", p, got)
		}
		if cb.goi != 0 {
			t.Errorf("%s: preflight chạm chuỗi cán bộ", p)
		}
	}
}

func TestTuyenCongKhaiMangCORSChoNguonMiniApp(t *testing.T) {
	for _, p := range tuyenCongKhai {
		h, _ := dungNgoaiThu(t)
		w := goiThu(h, http.MethodGet, p+"?host=xa-thu.vigov.vn", nguonMiniAppThu)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != nguonMiniAppThu {
			t.Errorf("%s: Access-Control-Allow-Origin = %q, muốn %q", p, got, nguonMiniAppThu)
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("%s: tuyến công khai cấp Allow-Credentials", p)
		}
	}
}

func TestTuyenCanBoKhongMangCORS(t *testing.T) {
	h, _ := dungNgoaiThu(t)
	w := goiThu(h, http.MethodGet, "/api/v1/communes/current", nguonMiniAppThu)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("tuyến cán bộ mang header CORS: %v", w.Header())
	}
}

func TestHealthzNgoaiCaHaiChuoi(t *testing.T) {
	h, cb := dungNgoaiThu(t)
	if w := goiThu(h, http.MethodGet, "/healthz", ""); w.Code != http.StatusOK {
		t.Fatalf("/healthz: mã = %d", w.Code)
	}
	if cb.goi != 0 {
		t.Errorf("/healthz đi qua chuỗi cán bộ %d lần", cb.goi)
	}
}
