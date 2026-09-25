package httpx

// ClientIPTuProxyTinCay — the address recorded as "from which IP" on the audit trail (rule 6,
// invariant 2). Every failure here is silent in production: the trail looks normal and names the
// wrong machine, or worse, the address a client chose to write.
//
// Addresses are from the documentation ranges (RFC 5737, RFC 3849).

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

var (
	// The cluster: ingress-nginx and web-admin pods live in 10.0.0.0/8.
	tinCayCum = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}
	webAdmin  = "10.1.2.3"
	ingress   = "10.9.9.9"
)

// quaMiddleware runs one request through the middleware and returns what ClientIP saw inside.
func quaMiddleware(t *testing.T, tinCay []netip.Prefix, remote string, xff ...string) string {
	t.Helper()
	var thay string
	h := ClientIPTuProxyTinCay(tinCay)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		thay = ClientIP(r)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return thay
}

func TestClientIPTuProxy(t *testing.T) {
	for _, tc := range []struct {
		ten    string
		tinCay []netip.Prefix
		remote string
		xff    []string
		muon   string
	}{
		{"peer không tin cậy, XFF giả bị bỏ qua", tinCayCum, "203.0.113.5:4000",
			[]string{"198.51.100.1"}, "203.0.113.5"},
		{"peer tin cậy, XFF một mục", tinCayCum, webAdmin + ":4000",
			[]string{"198.51.100.1"}, "198.51.100.1"},
		{"chuỗi: giả, khách thật, web-admin — phần giả bên trái bị bỏ qua", tinCayCum, webAdmin + ":4000",
			[]string{"192.0.2.66, 198.51.100.1, " + ingress}, "198.51.100.1"},
		{"mục rác: dừng ở bước tin cậy cuối cùng", tinCayCum, webAdmin + ":4000",
			[]string{"198.51.100.1, khong-phai-ip, " + ingress}, ingress},
		{"mục rỗng cũng là rác", tinCayCum, webAdmin + ":4000",
			[]string{"198.51.100.1,," + ingress}, ingress},
		{"mục có cổng không phải địa chỉ: rác", tinCayCum, webAdmin + ":4000",
			[]string{"198.51.100.1:5555"}, webAdmin},
		{"mọi mục đều tin cậy: bước tin cậy tận cùng bên trái", tinCayCum, webAdmin + ":4000",
			[]string{"10.5.5.5, " + ingress}, "10.5.5.5"},
		{"peer tin cậy, không có XFF: peer", tinCayCum, webAdmin + ":4000", nil, webAdmin},
		{"IPv6", tinCayCum, "[fd00::10]:4000",
			[]string{"2001:db8::7, fd00::20"}, "2001:db8::7"},
		{"IPv4 ánh xạ trong IPv6 được chuẩn hoá, cả peer lẫn mục", tinCayCum, "[::ffff:10.1.2.3]:4000",
			[]string{"::ffff:198.51.100.1"}, "198.51.100.1"},
		{"peer ánh xạ không tin cậy vẫn trả dạng chuẩn", tinCayCum, "[::ffff:203.0.113.5]:4000",
			[]string{"198.51.100.1"}, "203.0.113.5"},
		{"cấu hình trống: RemoteAddr, bất kể XFF", nil, webAdmin + ":4000",
			[]string{"198.51.100.1"}, webAdmin},
		{"nhiều dòng XFF: nối theo thứ tự", tinCayCum, webAdmin + ":4000",
			[]string{"192.0.2.66", "198.51.100.1", ingress}, "198.51.100.1"},
		{"RemoteAddr không phải host:port: trả nguyên văn", tinCayCum, "@",
			[]string{"198.51.100.1"}, "@"},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			if got := quaMiddleware(t, tc.tinCay, tc.remote, tc.xff...); got != tc.muon {
				t.Errorf("ClientIP = %q, muốn %q", got, tc.muon)
			}
		})
	}
}

func TestClientIPKhongDocXRealIPHayForwarded(t *testing.T) {
	var thay string
	h := ClientIPTuProxyTinCay(tinCayCum)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		thay = ClientIP(r)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = webAdmin + ":4000"
	r.Header.Set("X-Real-IP", "198.51.100.1")
	r.Header.Set("Forwarded", "for=198.51.100.1")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if thay != webAdmin {
		t.Errorf("ClientIP = %q, muốn %q — X-Real-IP/Forwarded không ai trong cụm ghi", thay, webAdmin)
	}
}

func TestClientIPKhongCoMiddlewareVanLaSocket(t *testing.T) {
	// The call sites that run without the middleware (a test server, a handler exercised
	// directly) keep today's answer: the socket peer, header ignored.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = webAdmin + ":4000"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	if got := ClientIP(r); got != webAdmin {
		t.Errorf("ClientIP = %q, muốn %q", got, webAdmin)
	}
}

func TestClientIPTuProxyKhongBiNoiRongSauKhiGan(t *testing.T) {
	ds := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	mw := ClientIPTuProxyTinCay(ds)
	ds[0] = netip.MustParsePrefix("203.0.113.0/24") // caller mutates its slice after wiring
	var thay string
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { thay = ClientIP(r) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.5:4000"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if thay != "203.0.113.5" {
		t.Errorf("ClientIP = %q — ranh giới tin cậy bị nới sau khi gắn", thay)
	}
}
