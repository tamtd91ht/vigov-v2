package main

// THE CITIZEN EDGE OF THIS BINARY — the wiring, not the middleware (core/httpx proves that).
//
// What only this file can see: that dungNgoai routes GET /api/v1/communes to the CITIZEN chain and
// leaves /api/v1/communes/current on the STAFF chain, and that dungBienCongDan really installs the
// session layer and CORS. A pattern changed from exact to subtree, or a middleware deleted, leaves
// a service that starts and serves — and either 401s the staff sign-in screen or answers the Mini
// App with no session check. Nothing else in the repository turns red for that.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	svchttp "github.com/vihat/vigov/service-identity/internal/http"
)

const (
	hostApiIdentity = "identity.api.vigov.vn" // reserved, maps to no commune (ADR 0046)
	tokCongDanThu   = "tok-cong-dan-thu"
	nguonMiniAppThu = "https://h5.zdn.vn"
)

type soPhienThu struct{ goi int }

func (s *soPhienThu) TraCuu(_ context.Context, tok string) (httpx.CitizenSession, bool) {
	s.goi++
	if tok == tokCongDanThu {
		return httpx.CitizenSession{ID: "sid-thu", TenantID: ulidThu}, true // no phone: ADR 0045
	}
	return httpx.CitizenSession{}, false
}

type xaTheoHostThu struct{}

func (xaTheoHostThu) XaTheoHost(context.Context, string) (tenant.Tenant, bool, error) {
	return tenant.Tenant{ID: ulidThu, Name: "Xã Thử", Active: true}, true, nil
}

// canBoDanhDau stands in for the whole staff chain: it only records that it was reached.
type canBoDanhDau struct{ goi int }

func (c *canBoDanhDau) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.goi++
	w.WriteHeader(http.StatusTeapot)
}

func dungNgoaiThu(t *testing.T) (http.Handler, *canBoDanhDau, *soPhienThu) {
	t.Helper()
	nguon, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	mux := http.NewServeMux()
	svchttp.RegisterCongDan(mux, svchttp.DepsCongDan{Xa: xaTheoHostThu{}})
	so := &soPhienThu{}
	cb := &canBoDanhDau{}
	return dungNgoai(cb, dungBienCongDan(mux, so, nguon)), cb, so
}

func goiThu(h http.Handler, method, path, tok, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+hostApiIdentity+path, nil)
	r.Host = hostApiIdentity
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
		if method == http.MethodOptions {
			r.Header.Set("Access-Control-Request-Method", "GET")
			r.Header.Set("Access-Control-Request-Headers", "authorization")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDanhMucXaDiChuoiCongDanKhongDiChuoiCanBo(t *testing.T) {
	h, cb, _ := dungNgoaiThu(t)

	if w := goiThu(h, http.MethodGet, svchttp.MauDanhMucXa+"?host=xa-thu.vigov.vn", tokCongDanThu, ""); w.Code != http.StatusOK {
		t.Fatalf("có phiên công dân: mã = %d, muốn 200 — thân: %s", w.Code, w.Body.String())
	}
	// The session layer is really installed: without a token the SAME path is refused.
	if w := goiThu(h, http.MethodGet, svchttp.MauDanhMucXa+"?host=xa-thu.vigov.vn", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("không có phiên: mã = %d, muốn 401", w.Code)
	}
	if cb.goi != 0 {
		t.Fatalf("chuỗi cán bộ bị gọi %d lần cho tuyến công dân", cb.goi)
	}
}

func TestCommunesCurrentVanOChuoiCanBo(t *testing.T) {
	// The staff sign-in screen's read. Sent through CitizenEdge it would have no session and answer
	// 401 — the sign-in page unable to print the commune's name, with nothing red.
	for _, path := range []string{"/api/v1/communes/current", "/api/v1/communes/", "/api/v1/staff"} {
		h, cb, so := dungNgoaiThu(t)
		goiThu(h, http.MethodGet, path, "", "")
		if cb.goi != 1 || so.goi != 0 {
			t.Errorf("%s: chuỗi cán bộ gọi %d lần, sổ phiên công dân %d lần — muốn 1 và 0", path, cb.goi, so.goi)
		}
	}
}

func TestPreflightDanhMucXa204KhongChamSoPhien(t *testing.T) {
	h, _, so := dungNgoaiThu(t)
	w := goiThu(h, http.MethodOptions, svchttp.MauDanhMucXa, "", "https://mini.zalo.me")
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight: mã = %d, muốn 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://mini.zalo.me" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if so.goi != 0 {
		t.Errorf("preflight chạm sổ phiên %d lần", so.goi)
	}
}

func TestTuyenCanBoKhongMangCORS(t *testing.T) {
	h, _, _ := dungNgoaiThu(t)
	w := goiThu(h, http.MethodGet, "/api/v1/communes/current", "", nguonMiniAppThu)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("tuyến cán bộ mang header CORS: %v", w.Header())
	}
}

func TestHealthzNgoaiCaHaiChuoi(t *testing.T) {
	h, cb, so := dungNgoaiThu(t)
	if w := goiThu(h, http.MethodGet, "/healthz", "", ""); w.Code != http.StatusOK {
		t.Fatalf("/healthz: mã = %d", w.Code)
	}
	if cb.goi != 0 || so.goi != 0 {
		t.Errorf("/healthz đi qua một chuỗi: cán bộ %d, sổ phiên %d", cb.goi, so.goi)
	}
}
