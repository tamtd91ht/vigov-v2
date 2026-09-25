package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// A session WITH a commune but WITHOUT a verified phone — what the Mini App bridge issues on a
// silent open (ADR 0045 §Phiên chưa có số). It may browse; it must not read or write the citizen's
// own records. The default is refusal: XaTuPhien refuses it, only XaTuPhienChiXem accepts it.

const tokenChuaCoSo = "token-co-xa-chua-co-so"

func riaCoPhienChuaCoSo(trong http.Handler) http.Handler {
	so := soPhienMau()
	so[tokenChuaCoSo] = CitizenSession{ID: "phien-3", CitizenID: "", TenantID: xaA}
	return CitizenEdge(so)(trong)
}

func TestXaTuPhienTuChoiPhienChuaCoSo(t *testing.T) {
	var chay bool
	h := riaCoPhienChuaCoSo(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		chay = true
	})))

	w := goiCongDan(h, tokenChuaCoSo)

	if chay {
		t.Fatal("handler nghiệp vụ ĐÃ CHẠY với phiên chưa có số — truy vấn đường công dân không có " +
			"danh tính nào để lọc (ADR 0045 ĐIỀU KIỆN DỪNG #6)")
	}
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "chua_xac_thuc_so") {
		t.Fatalf("mã = %d, thân = %s — muốn 403 chua_xac_thuc_so để Mini App biết cần xin số",
			w.Code, w.Body.String())
	}
}

func TestXaTuPhienVanNhanPhienDaCoSo(t *testing.T) {
	// The control: the refusal above is about the missing phone, not about the route.
	var chay bool
	h := riaCoPhienChuaCoSo(XaTuPhien()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chay = true
		w.WriteHeader(http.StatusNoContent)
	})))
	if w := goiCongDan(h, tokenA); w.Code != http.StatusNoContent || !chay {
		t.Fatalf("phiên đã có số bị từ chối: mã = %d", w.Code)
	}
}

func TestXaTuPhienChiXemNhanPhienChuaCoSoVaCoXa(t *testing.T) {
	var thay tenant.ID
	h := riaCoPhienChuaCoSo(XaTuPhienChiXem("hồ sơ hiển thị của xã: ai mở app của xã cũng xem được, không phải hồ sơ của riêng ai")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			thay = tenant.MustFrom(r.Context())
			w.WriteHeader(http.StatusOK)
		})))

	w := goiCongDan(h, tokenChuaCoSo)

	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200 — tuyến chỉ xem đã khai mà vẫn từ chối phiên chưa có số", w.Code)
	}
	if thay != xaA {
		t.Fatalf("xã trong context = %q, muốn %q", thay, xaA)
	}
}

func TestXaTuPhienChiXemVanTuChoiKhiKhongCoXa(t *testing.T) {
	// View-only waives the PHONE, never the COMMUNE. The three commune-less states still answer
	// exactly as XaTuPhien does.
	h := riaCoPhienChuaCoSo(XaTuPhienChiXem("danh mục của xã, xem không cần số")(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	chuan := goiCongDan(riaCoPhienChuaCoSo(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))), "")

	for ten, tok := range map[string]string{"không có phiên": "", "token sai": "khong-co-that", "chưa chọn xã": tokenChua} {
		w := goiCongDan(h, tok)
		if w.Code != http.StatusUnauthorized || w.Body.String() != chuan.Body.String() {
			t.Errorf("%s: mã = %d thân = %s — muốn đúng câu trả lời 401 của XaTuPhien", ten, w.Code, w.Body.String())
		}
	}
}

func TestXaTuPhienChiXemKhongCoLyDoThiDungLucDung(t *testing.T) {
	for _, lyDo := range []string{"", "  "} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("XaTuPhienChiXem(%q) không dừng — một lớp nới lỏng không lý do đã lọt vào chuỗi", lyDo)
				}
			}()
			XaTuPhienChiXem(lyDo)
		}()
	}
}

func TestXaTuPhienChiXemTinhLaDaKhaiLop(t *testing.T) {
	// The undeclared-class wall must see this class as declared, or every view-only route answers 500.
	h := riaCoPhienChuaCoSo(XaTuPhienChiXem("danh mục của xã, xem không cần số")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))
	r := httptest.NewRequest("GET", "https://"+apiHost+"/api/v1/x", nil)
	r.Header.Set("Authorization", "Bearer "+tokenA)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200", w.Code)
	}
}
