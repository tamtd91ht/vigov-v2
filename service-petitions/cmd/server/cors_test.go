package main

// CORS ON THE CITIZEN EDGE — the wiring, not the middleware (core/httpx/cors_test.go proves that).
//
// What only this file can see: that dungBien mounts httpx.CORSCongDan on the CITIZEN chain, outside
// the session layer, and NOT on the staff chain; and that the Mini App's real API host — a reserved
// host that never resolves to a commune (ADR 0046) — reaches the citizen chain rather than the
// staff chain's Host-resolution 404.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/staffauth"
)

// hostApiPetitions is the host the Mini App calls in production (ADR 0046). It is NOT in thuMucGia,
// exactly as it is in no commune's registry entry.
const hostApiPetitions = "petitions.api.vigov.vn"

const nguonMiniApp = "https://h5.zdn.vn"

func nguonDeXuat(t *testing.T) config.NguonCORS {
	t.Helper()
	n, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	return n
}

func (m *mayChu) goiCORS(t *testing.T, method, host, path, origin, acrm, token, phieu string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.9:51000"
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if acrm != "" {
		r.Header.Set("Access-Control-Request-Method", acrm)
		r.Header.Set("Access-Control-Request-Headers", "authorization,content-type,idempotency-key")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if phieu != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: phieu})
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func coHeaderCORS(w *httptest.ResponseRecorder) bool {
	for k := range w.Header() {
		if len(k) >= 12 && k[:12] == "Access-Contr" {
			return true
		}
	}
	for _, v := range w.Header().Values("Vary") {
		if v == "Origin" {
			return true
		}
	}
	return false
}

// THE HOST QUESTION THE TASK TURNS ON: a citizen request to the reserved API host reaches the
// citizen chain (401 from the session layer), never TenantMiddleware (404).
func TestHostApiPetitionsToiChuoiCongDan_401KhongPhai404(t *testing.T) {
	for _, path := range []string{tapCongDan, tienToCongDan + maPhieuCuaToi} {
		m := dungMayChuCORS(t, canBoXaA(), nguonDeXuat(t))
		doiMa(t, m.goiCORS(t, http.MethodGet, hostApiPetitions, path, nguonMiniApp, "", "", ""),
			http.StatusUnauthorized)
		if m.pg.goi != 0 {
			t.Errorf("%s: chuỗi cán bộ đã chạy (gọi phân giải cán bộ %d lần)", path, m.pg.goi)
		}
		// And with a usable token it is served — so the 401 above is the session layer's.
		doiMa(t, m.goiCORS(t, http.MethodGet, hostApiPetitions, path, nguonMiniApp, "", tokenCongDan, ""),
			http.StatusOK)
	}
}

func TestPreflightCongDan_204CoHeader_KhongToiSoPhien(t *testing.T) {
	m := dungMayChuCORS(t, canBoXaA(), nguonDeXuat(t))

	for _, path := range []string{tapCongDan, tienToCongDan + maPhieuCuaToi} {
		w := m.goiCORS(t, http.MethodOptions, hostApiPetitions, path, "https://mini.zalo.me", "POST", "", "")
		doiMa(t, w, http.StatusNoContent)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://mini.zalo.me" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q", path, got)
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("%s: Access-Control-Allow-Credentials không bao giờ được gửi", path)
		}
	}
	if m.so.goi != 0 || m.pg.goi != 0 {
		t.Errorf("preflight chạm sổ phiên %d lần, phân giải cán bộ %d lần — muốn 0 và 0", m.so.goi, m.pg.goi)
	}
}

func TestPreflightCongDanNguonLa_204KhongHeader(t *testing.T) {
	m := dungMayChuCORS(t, canBoXaA(), nguonDeXuat(t))

	w := m.goiCORS(t, http.MethodOptions, hostApiPetitions, tapCongDan,
		"https://evil.zdn.vn.attacker.com", "POST", "", "")
	doiMa(t, w, http.StatusNoContent)
	if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Errorf("origin lạ nhận header CORS: %v", w.Header())
	}
	if m.so.goi != 0 {
		t.Errorf("preflight chạm sổ phiên %d lần", m.so.goi)
	}
}

func TestGetCongDanNguonDuocPhep_CoHeaderVaVary(t *testing.T) {
	m := dungMayChuCORS(t, canBoXaA(), nguonDeXuat(t))

	w := m.goiCORS(t, http.MethodGet, hostApiPetitions, tienToCongDan+maPhieuCuaToi, nguonMiniApp, "", tokenCongDan, "")
	doiMa(t, w, http.StatusOK)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != nguonMiniApp {
		t.Errorf("Access-Control-Allow-Origin = %q, muốn %q", got, nguonMiniApp)
	}
	vary := false
	for _, v := range w.Header().Values("Vary") {
		vary = vary || v == "Origin"
	}
	if !vary {
		t.Error("thiếu Vary: Origin")
	}
}

// THE STAFF CHAIN NEVER CARRIES CORS — not on a served request, not on a preflight, not for an
// origin the citizen edge would accept.
func TestTuyenCanBoKhongBaoGioCoCORS(t *testing.T) {
	m := dungMayChuCORS(t, canBoXaA(), nguonDeXuat(t))

	w := m.goiCORS(t, http.MethodGet, hostA, tuyen, nguonMiniApp, "", "", phieuGia)
	doiMa(t, w, http.StatusOK)
	if coHeaderCORS(w) {
		t.Errorf("tuyến cán bộ mang header CORS: %v", w.Header())
	}

	p := m.goiCORS(t, http.MethodOptions, hostA, tuyen, nguonMiniApp, "GET", "", "")
	if p.Code == http.StatusNoContent || coHeaderCORS(p) {
		t.Errorf("preflight tới tuyến cán bộ được trả lời như CORS: mã %d, %v", p.Code, p.Header())
	}
}

func TestKhongCauHinhCORSThiKhongCoHeaderNaoCa(t *testing.T) {
	m := dungMayChuCORS(t, canBoXaA(), config.NguonCORS(nil))

	for _, w := range []*httptest.ResponseRecorder{
		m.goiCORS(t, http.MethodGet, hostApiPetitions, tienToCongDan+maPhieuCuaToi, nguonMiniApp, "", tokenCongDan, ""),
		m.goiCORS(t, http.MethodOptions, hostApiPetitions, tapCongDan, nguonMiniApp, "POST", "", ""),
		m.goiCORS(t, http.MethodGet, hostA, tuyen, nguonMiniApp, "", "", phieuGia),
	} {
		if coHeaderCORS(w) {
			t.Errorf("không cấu hình CORS mà vẫn có header: mã %d, %v", w.Code, w.Header())
		}
	}
}
