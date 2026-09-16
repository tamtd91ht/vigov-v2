package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/httpx"
	"github.com/vihat/vigov/pkg/tenant"
)

// End-to-end test of the edge chain against a real database.
//
// WHY THIS EXISTS SEPARATELY from the directory tests: those check that a Host resolves. This
// checks what a REQUEST does — which is the thing that actually protects a commune. The two
// can diverge: a directory that resolves correctly behind middleware wired in the wrong order
// still leaks, and no unit test of either half would show it.

func dungEdge(t *testing.T, ttl time.Duration) (http.Handler, *tenant.ID) {
	t.Helper()

	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã đã sáp nhập", false)
	themHost(t, db, "xacu.vigov.vn", ulidB, true)

	// The handler records which commune the edge decided on, so the test asserts on what
	// business code would actually see rather than on a status code alone.
	var thay tenant.ID
	cuoi := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thay = tenant.MustFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	dir := NewCachedDirectory(NewDirectory(db), ttl)

	var h http.Handler = cuoi
	h = httpx.TenantMiddleware(dir)(h)
	h = httpx.Recover(func(context.Context) string { return "test" })(h)
	h = httpx.StripTenantHeaders(h)

	return h, &thay
}

func TestEdgeHostHopLeThiVaoDuocVaMangTheoXa(t *testing.T) {
	h, thay := dungEdge(t, 0)

	req := httptest.NewRequest(http.MethodGet, "http://thangbinh.vigov.vn/bat-ky", nil)
	req.Host = "thangbinh.vigov.vn"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("mã trạng thái = %d, muốn 200", w.Code)
	}
	if *thay != tenant.ID(ulidA) {
		t.Errorf("handler thấy xã %q, muốn %q", *thay, ulidA)
	}
}

func TestEdgeHostLaTraVe404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve = 404. Not 400, not a fallback commune, and 404 also
	// reveals nothing about which communes exist on the platform.
	h, thay := dungEdge(t, 0)

	req := httptest.NewRequest(http.MethodGet, "http://khong-ton-tai.vigov.vn/bat-ky", nil)
	req.Host = "khong-ton-tai.vigov.vn"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("mã trạng thái = %d, muốn 404", w.Code)
	}
	if *thay != "" {
		t.Errorf("handler chạy với xã %q — request lẽ ra phải bị chặn ở biên", *thay)
	}
}

func TestEdgeXaNgungHoatDongTraVe404(t *testing.T) {
	h, _ := dungEdge(t, 0)

	req := httptest.NewRequest(http.MethodGet, "http://xacu.vigov.vn/bat-ky", nil)
	req.Host = "xacu.vigov.vn"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("xã ngừng hoạt động: mã = %d, muốn 404", w.Code)
	}
}

func TestEdgeXoaHeaderTenantTuClient(t *testing.T) {
	// THE ONE THAT MATTERS MOST. A client naming its own commune is a client granting itself
	// access to another commune's data (rule 1, forbidden #2). The header must be stripped
	// BEFORE anything reads it, and the commune must come from Host regardless of what the
	// client claimed.
	h, thay := dungEdge(t, 0)

	req := httptest.NewRequest(http.MethodGet, "http://thangbinh.vigov.vn/bat-ky", nil)
	req.Host = "thangbinh.vigov.vn"
	req.Header.Set("X-Tenant-ID", ulidB)       // cố gán sang xã khác
	req.Header.Set("x-tenant-override", ulidB) // và thử cả dạng chữ thường
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200", w.Code)
	}
	if *thay != tenant.ID(ulidA) {
		t.Errorf("client tự đặt header và giành được xã %q — đây là lỗ hổng cách ly", *thay)
	}
	if req.Header.Get("X-Tenant-ID") != "" {
		t.Error("header tenant từ client chưa bị xoá")
	}
}

func TestEdgeHostCoCongVaChuHoa(t *testing.T) {
	// Host arrives from the client. Two spellings of one host that do not compare equal mean a
	// whole commune returns 404 for a reason invisible in the logs.
	h, thay := dungEdge(t, 0)

	for _, host := range []string{
		"ThangBinh.ViGov.VN",
		"thangbinh.vigov.vn:443",
		"ThangBinh.vigov.vn:8080",
	} {
		*thay = ""
		req := httptest.NewRequest(http.MethodGet, "http://thangbinh.vigov.vn/bat-ky", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Host %q: mã = %d, muốn 200", host, w.Code)
			continue
		}
		if *thay != tenant.ID(ulidA) {
			t.Errorf("Host %q: thấy xã %q, muốn %q", host, *thay, ulidA)
		}
	}
}

func TestEdgeCacheKhongLamLoDuLieuGiuaCacXa(t *testing.T) {
	// The cache is keyed by Host, so it cannot serve one commune's rows under another
	// commune's address. This asserts that directly, because a cache keyed on the wrong thing
	// is the classic way multi-tenant isolation dies silently.
	h, thay := dungEdge(t, time.Minute)

	// Warm the cache with commune A.
	req := httptest.NewRequest(http.MethodGet, "http://thangbinh.vigov.vn/x", nil)
	req.Host = "thangbinh.vigov.vn"
	h.ServeHTTP(httptest.NewRecorder(), req)
	if *thay != tenant.ID(ulidA) {
		t.Fatalf("khởi động cache: thấy %q", *thay)
	}

	// A different Host must not be answered from A's entry.
	*thay = ""
	req2 := httptest.NewRequest(http.MethodGet, "http://khong-ton-tai.vigov.vn/x", nil)
	req2.Host = "khong-ton-tai.vigov.vn"
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("host lạ sau khi cache nóng: mã = %d, muốn 404", w2.Code)
	}
	if *thay != "" {
		t.Errorf("host lạ nhận được xã %q từ cache — cache đang khoá sai thứ", *thay)
	}
}
