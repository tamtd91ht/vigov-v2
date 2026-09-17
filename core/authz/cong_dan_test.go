package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// Hai trục vuông góc trên CÙNG một chuỗi: authz.* trả lời AI, lớp xã của httpx trả lời XÃ NÀO
// (ADR 0022). Tệp này kiểm chỗ hai trục gặp nhau — nếu CitizenPrincipal không đọc được phiên
// mà rìa đã tra, thì CitizenOnly từ chối mọi công dân hợp lệ và không có gì nói vì sao.

type soPhienCongDanGia map[string]httpx.CitizenSession

func (m soPhienCongDanGia) TraCuu(_ context.Context, tok string) (httpx.CitizenSession, bool) {
	p, ok := m[tok]
	return p, ok
}

const (
	tokenCongDan = "token-cong-dan"
	xaCongDan    = tenant.ID("01J0000000000000000000000A")
)

func chuoiCongDan(trong http.Handler) http.Handler {
	so := soPhienCongDanGia{
		tokenCongDan: {ID: "phien-1", CitizenID: "cd-1", TenantID: xaCongDan},
	}
	return httpx.CitizenEdge(so)(CitizenPrincipal()(CitizenOnly()(trong)))
}

func goiCongDan(h http.Handler, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://api.vigov.vn/api/v1/bat-ky", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestKhongCoPhienThiCitizenOnlyTuChoi(t *testing.T) {
	var chay bool
	h := chuoiCongDan(httpx.XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		chay = true
	})))

	w := goiCongDan(h, "")

	if w.Code != http.StatusUnauthorized || chay {
		t.Fatalf("mã = %d, handler chạy = %v — muốn 401 và handler không chạy", w.Code, chay)
	}
}

// Principal của công dân mang ĐỊNH DANH MỜ, không mang dữ liệu cá nhân: số điện thoại là thứ
// định danh công dân trong nghiệp vụ (ADR 0020) và là dữ liệu cá nhân theo luật 3. Nó không
// được có đường nào đi vào core, nhất là qua một kiểu chạy trên mọi yêu cầu.
func TestPhienHopLeThiCoPrincipalCongDan(t *testing.T) {
	var p Principal
	h := chuoiCongDan(httpx.XaTuPhien()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ = From(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})))

	w := goiCongDan(h, tokenCongDan)

	if w.Code != http.StatusNoContent {
		t.Fatalf("mã = %d, muốn 204", w.Code)
	}
	if p.Kind != "citizen" || p.ID != "cd-1" || p.TenantID != xaCongDan {
		t.Fatalf("principal = %+v, muốn công dân cd-1 của xã %q", p, xaCongDan)
	}
	if len(p.Roles) != 0 {
		t.Fatalf("công dân có vai trò: %v — công dân không dùng RBAC (luật 5 bất biến 6)", p.Roles)
	}
}
