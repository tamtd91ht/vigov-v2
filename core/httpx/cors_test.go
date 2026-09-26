package httpx_test

// CORSCongDan — the citizen edge's CORS answer. The matching rule itself is proved in
// core/config/cors_test.go; this file proves what the middleware does with the answer.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
)

// dichVuSau counts the requests that got PAST the middleware — the only way to show a preflight
// was answered at the edge rather than by whatever sits behind it.
type dichVuSau struct{ goi int }

func (d *dichVuSau) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	d.goi++
	// Standing in for the citizen chain with no session: authentication decides, not CORS.
	httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "x", "")
}

func dungCORS(t *testing.T, raw string) (http.Handler, *dichVuSau) {
	t.Helper()
	n, err := config.PhanTichNguonCORS(raw)
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	sau := &dichVuSau{}
	return httpx.CORSCongDan(n)(sau), sau
}

func goiCORS(h http.Handler, method, origin, acrm string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://petitions.api.example.vn/api/v1/my-citizen-reports", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if acrm != "" {
		r.Header.Set("Access-Control-Request-Method", acrm)
		r.Header.Set("Access-Control-Request-Headers", "authorization,content-type,idempotency-key")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func coVaryOrigin(w *httptest.ResponseRecorder) bool {
	for _, v := range w.Header().Values("Vary") {
		if v == "Origin" {
			return true
		}
	}
	return false
}

func TestCORSPreflightNguonDuocPhep_204CoHeader_KhongToiLopSau(t *testing.T) {
	h, sau := dungCORS(t, "https://h5.zdn.vn,https://*.zdn.vn")

	w := goiCORS(h, http.MethodOptions, "https://stc.zdn.vn", "POST")
	if w.Code != http.StatusNoContent {
		t.Fatalf("mã = %d, muốn 204", w.Code)
	}
	for k, muon := range map[string]string{
		"Access-Control-Allow-Origin":  "https://stc.zdn.vn",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Authorization, Content-Type, Idempotency-Key",
		"Access-Control-Max-Age":       "600",
	} {
		if got := w.Header().Get(k); got != muon {
			t.Errorf("%s = %q, muốn %q", k, got, muon)
		}
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("Access-Control-Allow-Credentials không bao giờ được gửi — rìa công dân dùng bearer")
	}
	if !coVaryOrigin(w) {
		t.Error("thiếu Vary: Origin")
	}
	if sau.goi != 0 {
		t.Errorf("preflight đi tới lớp xác thực %d lần", sau.goi)
	}
}

func TestCORSPreflightNguonLa_204KhongHeader_KhongToiLopSau(t *testing.T) {
	h, sau := dungCORS(t, "https://h5.zdn.vn")

	for _, o := range []string{"https://evil.zdn.vn.attacker.com", "http://h5.zdn.vn", "https://example.com"} {
		w := goiCORS(h, http.MethodOptions, o, "GET")
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: mã = %d, muốn 204", o, w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q, muốn không có", o, got)
		}
		if w.Header().Get("Access-Control-Allow-Methods") != "" {
			t.Errorf("%s: lộ danh sách phương thức cho origin lạ", o)
		}
	}
	if sau.goi != 0 {
		t.Errorf("preflight đi tới lớp xác thực %d lần", sau.goi)
	}
}

func TestCORSYeuCauThuongNguonDuocPhep_CoHeaderVaVary_XacThucVanQuyetDinh(t *testing.T) {
	h, sau := dungCORS(t, "https://h5.zdn.vn")

	w := goiCORS(h, http.MethodGet, "https://h5.zdn.vn", "")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://h5.zdn.vn" {
		t.Errorf("Access-Control-Allow-Origin = %q, muốn origin được khớp", got)
	}
	if !coVaryOrigin(w) {
		t.Error("thiếu Vary: Origin")
	}
	// CORS says who may READ; authentication still says who may ASK.
	if w.Code != http.StatusUnauthorized || sau.goi != 1 {
		t.Errorf("mã = %d, lớp sau chạy %d lần — muốn 401 và 1", w.Code, sau.goi)
	}
}

func TestCORSYeuCauThuongNguonLa_KhongHeaderNhungVanDiTiep(t *testing.T) {
	h, sau := dungCORS(t, "https://h5.zdn.vn")

	for _, o := range []string{"https://example.com", ""} {
		w := goiCORS(h, http.MethodGet, o, "")
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%q: Access-Control-Allow-Origin = %q", o, got)
		}
		if !coVaryOrigin(w) {
			t.Errorf("%q: thiếu Vary: Origin — bộ đệm dùng chung có thể trao phản hồi cho origin khác", o)
		}
	}
	if sau.goi != 2 {
		t.Errorf("lớp sau chạy %d lần, muốn 2", sau.goi)
	}
}

func TestCORSOptionsKhongPhaiPreflightThiDiTiep(t *testing.T) {
	// OPTIONS with no Access-Control-Request-Method is not a preflight; it is not this layer's.
	h, sau := dungCORS(t, "https://h5.zdn.vn")
	goiCORS(h, http.MethodOptions, "https://h5.zdn.vn", "")
	if sau.goi != 1 {
		t.Errorf("lớp sau chạy %d lần, muốn 1", sau.goi)
	}
}

func TestCORSKhongCauHinhThiKhongCoHeaderNaoCa(t *testing.T) {
	for _, n := range []httpx.NguonCORS{nil, config.NguonCORS(nil)} {
		sau := &dichVuSau{}
		h := httpx.CORSCongDan(n)(sau)

		for _, method := range []string{http.MethodOptions, http.MethodGet} {
			acrm := ""
			if method == http.MethodOptions {
				acrm = "GET"
			}
			w := goiCORS(h, method, "https://h5.zdn.vn", acrm)
			for k := range w.Header() {
				if k == "Vary" || len(k) > 12 && k[:12] == "Access-Contr" {
					t.Errorf("%s: header %s có mặt dù không cấu hình CORS", method, k)
				}
			}
		}
		if sau.goi != 2 {
			t.Errorf("không cấu hình thì mọi yêu cầu đi tiếp nguyên trạng; lớp sau chạy %d lần, muốn 2", sau.goi)
		}
	}
}
