package portal

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// The fixture key. Not a credential anywhere; distinctive so a leak is found by substring.
const testKey = "fixture-portal-key-QX7Z"

const (
	portalHost = "portal.example.gov.vn"
	otherHost  = "other.example.gov.vn"
)

// publicAddr is what the fake resolver answers for the test hosts. A PUBLIC address, so the guard
// passes it exactly as it would in production; the fake dialer then maps it onto the httptest
// listener. The guard itself is not touched.
var publicAddr = netip.MustParseAddr("93.184.216.34")

// testPortal is an httptest TLS server whose certificate names the `.gov.vn` test hosts, and a Client
// wired to reach it through the REAL guard with an injected resolver and socket.
type testPortal struct {
	srv    *httptest.Server
	client *Client

	mu       sync.Mutex
	answers  map[string][]netip.Addr
	dialed   []string
	queries  []url.Values
	resolved int
}

type fakeResolver struct{ p *testPortal }

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.resolved++
	a, ok := r.p.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return a, nil
}

func newTestPortal(t *testing.T, h http.HandlerFunc) *testPortal {
	t.Helper()
	cert, pool := testCertificate(t, portalHost, otherHost)
	p := &testPortal{answers: map[string][]netip.Addr{
		portalHost: {publicAddr},
		otherHost:  {publicAddr},
	}}
	p.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.queries = append(p.queries, r.URL.Query())
		p.mu.Unlock()
		h(w, r)
	}))
	p.srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	p.srv.StartTLS()
	t.Cleanup(p.srv.Close)

	target := p.srv.Listener.Addr().String()
	p.client = New(Options{
		Resolver: fakeResolver{p},
		RootCAs:  pool,
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			p.mu.Lock()
			p.dialed = append(p.dialed, addr)
			p.mu.Unlock()
			var d net.Dialer
			return d.DialContext(ctx, network, target)
		},
	})
	return p
}

func (p *testPortal) endpoint(t *testing.T) Endpoint {
	t.Helper()
	base, err := ParseBase("https://" + portalHost + "/DesktopModules/cttdt/api/apichiase")
	if err != nil {
		t.Fatalf("base: %v", err)
	}
	return Endpoint{Base: base, Key: secret.Secret(testKey)}
}

func testCertificate(t *testing.T, hosts ...string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: hosts[0]},
		DNSNames:              hosts,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// assertNoSecret is the redaction check: neither the message nor any formatting of the error carries
// the key or the query string it travels in.
func assertNoSecret(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(s, testKey) || strings.Contains(s, "secret_code") {
			t.Fatalf("error leaks the key: %q", s)
		}
	}
}

// --- check 1: the URL --------------------------------------------------------------------------------

func TestCheckURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://thangbinh.danang.gov.vn/DesktopModules/cttdt/api/apichiase": true,
		"https://THANGBINH.Danang.Gov.Vn/api":                                true,
		"https://xn--thng-bnh-ixa.danang.gov.vn/api":                         true,
		"https://portal.example.gov.vn:443/api":                              true,
		"http://thangbinh.danang.gov.vn/api":                                 false, // not https
		"https://thangbinh.danang.gov.vn:8443/api":                           false, // only 443
		"https://example.com/api":                                            false,
		"https://evilgov.vn/api":                                             false,
		"https://gov.vn/api":                                                 false,
		"https://x.gov.vn.evil.com/api":                                      false,
		"https://thangbinh.danang.gov.vn./api":                               false, // trailing dot
		"https://10.0.0.5/api":                                               false, // IP literal
		"https://[::1]/api":                                                  false,
		"https://user:pass@thangbinh.danang.gov.vn/api":                      false, // userinfo
		"https://evil.com@thangbinh.danang.gov.vn/api":                       false,
		"https://-bad.gov.vn/api":                                            false,
		"https://a_b.gov.vn/api":                                             false,
		"ftp://thangbinh.danang.gov.vn/api":                                  false,
	} {
		u, err := url.Parse(raw)
		if err != nil {
			if ok {
				t.Fatalf("%s: parse: %v", raw, err)
			}
			continue
		}
		if got := CheckURL(u) == nil; got != ok {
			t.Errorf("CheckURL(%s) = %v, want %v", raw, got, ok)
		}
	}
}

func TestParseBaseRefusesQueryAndFragment(t *testing.T) {
	for _, raw := range []string{
		"https://portal.example.gov.vn/api?secret_code=x",
		"https://portal.example.gov.vn/api#x",
	} {
		if _, err := ParseBase(raw); !errors.Is(err, ErrURLRefused) {
			t.Errorf("ParseBase(%s) = %v, want url-refused", raw, err)
		}
	}
}

func TestAddrAllowed(t *testing.T) {
	for s, ok := range map[string]bool{
		"93.184.216.34":   true,
		"2606:4700::1111": true,
		"127.0.0.1":       false,
		"10.1.2.3":        false,
		"172.16.0.1":      false,
		"192.168.1.1":     false,
		"169.254.169.254": false, // cloud metadata
		"100.100.100.200": false, // carrier NAT (some clouds' metadata)
		"0.0.0.0":         false,
		"224.0.0.1":       false,
		"::1":             false,
		"fe80::1":         false,
		"fd00:ec2::254":   false, // ULA (a v6 metadata endpoint)
		"::ffff:10.0.0.1": false, // mapped private v4
		"64:ff9b::a00:1":  false, // NAT64 of 10.0.0.1
		"2002:a00:1::":    false, // 6to4 of 10.0.0.1
	} {
		if got := AddrAllowed(netip.MustParseAddr(s)); got != ok {
			t.Errorf("AddrAllowed(%s) = %v, want %v", s, got, ok)
		}
	}
}

// --- check 2: the address at connect time --------------------------------------------------------------

func TestDialRefusesPrivateResolution(t *testing.T) {
	for name, answer := range map[string][]netip.Addr{
		"loopback":       {netip.MustParseAddr("127.0.0.1")},
		"private":        {netip.MustParseAddr("10.0.0.5")},
		"metadata":       {netip.MustParseAddr("169.254.169.254")},
		"one bad of two": {publicAddr, netip.MustParseAddr("192.168.0.10")},
	} {
		t.Run(name, func(t *testing.T) {
			p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
			p.answers[portalHost] = answer
			_, err := p.client.Categories(context.Background(), p.endpoint(t))
			if !errors.Is(err, ErrAddressRefused) {
				t.Fatalf("err = %v, want address-refused", err)
			}
			if len(p.dialed) != 0 {
				t.Fatalf("a socket was opened to %v", p.dialed)
			}
			assertNoSecret(t, err)
		})
	}
}

func TestDialConnectsToTheCheckedAddressOnly(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	if _, err := p.client.Categories(context.Background(), p.endpoint(t)); err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(p.dialed) != 1 || p.dialed[0] != "93.184.216.34:443" {
		t.Fatalf("dialed %v, want the resolved address on 443", p.dialed)
	}
}

func TestUnknownHostIsDNSAndNoSecret(t *testing.T) {
	p := newTestPortal(t, nil)
	delete(p.answers, portalHost)
	_, err := p.client.Categories(context.Background(), p.endpoint(t))
	if !errors.Is(err, ErrDNS) {
		t.Fatalf("err = %v, want dns", err)
	}
	assertNoSecret(t, err)
}

// --- redirects (owner, 01/10/2026: ≤ 3 hops, every target checked like api_url) ------------------------

func TestRedirects(t *testing.T) {
	cases := map[string]struct {
		locations []string // the Location of hop i; then 200
		want      error
	}{
		"to http":             {[]string{"http://" + portalHost + "/x"}, ErrRedirectRefused},
		"to a non-gov host":   {[]string{"https://example.com/x"}, ErrRedirectRefused},
		"to an IP":            {[]string{"https://10.0.0.1/x"}, ErrRedirectRefused},
		"to userinfo":         {[]string{"https://u:p@" + portalHost + "/x"}, ErrRedirectRefused},
		"four hops":           {[]string{"/a", "/b", "/c", "/d"}, ErrTooManyRedirects},
		"three hops are fine": {[]string{"/a", "/b", "/c"}, nil},
		"another gov host":    {[]string{"https://" + otherHost + "/x"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			hop := 0
			p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				i := hop
				hop++
				mu.Unlock()
				if i < len(tc.locations) {
					http.Redirect(w, r, tc.locations[i], http.StatusFound)
					return
				}
				_, _ = w.Write([]byte(`[]`))
			})
			_, err := p.client.Categories(context.Background(), p.endpoint(t))
			if tc.want == nil {
				if err != nil {
					t.Fatalf("err = %v, want success", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			assertNoSecret(t, err)
		})
	}
}

// --- redaction ---------------------------------------------------------------------------------------

func TestErrorsNeverCarryTheKey(t *testing.T) {
	t.Run("http status", func(t *testing.T) {
		p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) {
			// An error page quoting the request — the body is never read into the error.
			http.Error(w, "bad request for "+r.URL.String(), http.StatusServiceUnavailable)
		})
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{})
		if code, ok := IsHTTPStatus(err); !ok || code != 503 || ClassOf(err) != "http-503" {
			t.Fatalf("err = %v, want http-503", err)
		}
		if !errors.Is(err, ErrHTTPStatus) {
			t.Error("ErrHTTPStatus does not match an http-<n> error")
		}
		assertNoSecret(t, err)
	})
	t.Run("connection dropped", func(t *testing.T) {
		p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijacker")
			}
			c, _, _ := hj.Hijack()
			_ = c.Close()
		})
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{})
		assertNoSecret(t, err)
	})
	t.Run("connect refused", func(t *testing.T) {
		p := newTestPortal(t, nil)
		p.client = New(Options{Resolver: fakeResolver{p}, Dial: func(ctx context.Context, n, a string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: n, Err: errors.New("connection refused")}
		}})
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{})
		if !errors.Is(err, ErrConnect) {
			t.Fatalf("err = %v, want connect", err)
		}
		assertNoSecret(t, err)
	})
	t.Run("timeout", func(t *testing.T) {
		p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) })
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := p.client.Articles(ctx, p.endpoint(t), "120", time.Time{})
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("err = %v, want timeout", err)
		}
		assertNoSecret(t, err)
	})
	t.Run("certificate for another name", func(t *testing.T) {
		p := newTestPortal(t, nil)
		_, other := testCertificate(t, "unrelated.example.gov.vn")
		p.client = New(Options{Resolver: fakeResolver{p}, RootCAs: other, Dial: func(ctx context.Context, n, a string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, n, p.srv.Listener.Addr().String())
		}})
		_, err := p.client.Categories(context.Background(), p.endpoint(t))
		if !errors.Is(err, ErrTLS) {
			t.Fatalf("err = %v, want tls", err)
		}
		assertNoSecret(t, err)
	})
}

func TestTheKeyTravelsOnlyInTheQuery(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	if _, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}); err != nil {
		t.Fatal(err)
	}
	q := p.queries[0]
	if q.Get("secret_code") != testKey || q.Get("lstChuyenMuc") != "120" {
		t.Fatalf("query = %v", q)
	}
	if s := fmt.Sprintf("%v %+v", p.endpoint(t), p.endpoint(t)); strings.Contains(s, testKey) {
		t.Error("formatting an Endpoint prints the key")
	}
}

func TestEmptyCategoryIsNeverAsked(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { t.Error("the portal was called") })
	for _, id := range []string{"", " ", "0"} {
		if _, err := p.client.Articles(context.Background(), p.endpoint(t), id, time.Time{}); !errors.Is(err, ErrCategoryRequired) {
			t.Errorf("Articles(%q) = %v, want category-required", id, err)
		}
	}
}

func TestResponseSizeCap(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 100))) })
	u, _ := url.Parse("https://" + portalHost + "/x")
	if _, err := p.client.get(context.Background(), u, 10, ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want too-large", err)
	}
}

// --- parsing -----------------------------------------------------------------------------------------

func TestParseCategories(t *testing.T) {
	got, err := parseCategories([]byte(`[
		{"ChuyenMucID":144,"TenChuyenMuc":"Chuyển đổi số","DanhMucID":2,"TenDanhMuc":"Thông tin - Tư liệu"},
		{"ChuyenMucID":"120","TenChuyenMuc":"Tin &amp; sự kiện","DanhMucID":"1","TenDanhMuc":"Tin tức"},
		{"ChuyenMucID":120,"TenChuyenMuc":"duplicate","TenDanhMuc":"Tin tức"},
		{"ChuyenMucID":0,"TenChuyenMuc":"root"},
		{"ChuyenMucID":"","TenChuyenMuc":"no id"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ExternalID != "144" || got[1].Name != "Tin & sự kiện" || got[1].ParentName != "Tin tức" {
		t.Fatalf("got %+v", got)
	}
	if _, err := parseCategories([]byte(`{"error":1}`)); !errors.Is(err, ErrParse) {
		t.Fatalf("object answer = %v, want parse", err)
	}
}

func TestParseArticles(t *testing.T) {
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, portalLocation)
	body := `[
		{"TinTucID":9001,"TieuDe":" Hội nghị  &amp; tổng kết ","GioiThieu":"<b>Tóm</b> tắt",
		 "NoiDung":"&lt;p&gt;Đoạn &lt;strong&gt;một&lt;/strong&gt;&lt;/p&gt;&lt;script&gt;x()&lt;/script&gt;",
		 "AnhLonUrl":"/Portals/0/a.jpg","NgayDang":"2026-09-29T21:27:27.367","NguonTin":"Báo Đà Nẵng","TacGia":"Nguyễn Văn A"},
		{"TinTucID":"9002","TieuDe":"Cũ","NgayDang":"2026-08-01T08:00:00"},
		{"TinTucID":"9003","TieuDe":"Không ngày","NgayDang":"0001-01-01T00:00:00"},
		{"TinTucID":"9004","TieuDe":"Đã xoá","NgayDang":"2026-09-29T08:00:00","isDelete":true},
		{"TinTucID":"","TieuDe":"Không mã","NgayDang":"2026-09-29T08:00:00"},
		{"TinTucID":"9005","TieuDe":"Ảnh nhỏ","AnhNhoUrl":"http://portal.example.gov.vn/b.png","NgayDang":"2026-09-30"}]`
	b, err := parseArticles([]byte(body), since)
	if err != nil {
		t.Fatal(err)
	}
	if b.Read != 4 { // 9001, 9002, 9003, 9005 — the deleted and the id-less rows are not articles
		t.Errorf("Read = %d, want 4", b.Read)
	}
	if len(b.Articles) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(b.Articles), b.Articles)
	}
	a := b.Articles[0]
	if a.ExternalID != "9001" || a.Title != "Hội nghị & tổng kết" || a.Summary != "Tóm tắt" {
		t.Errorf("article = %+v", a)
	}
	// Decoded ONCE: real markup now, which the caller's sanitiser then reduces.
	if a.BodyHTML != "<p>Đoạn <strong>một</strong></p><script>x()</script>" {
		t.Errorf("body = %q", a.BodyHTML)
	}
	if a.SourceLabel != "Báo Đà Nẵng" || a.ImageRef != "/Portals/0/a.jpg" {
		t.Errorf("source/image = %q / %q", a.SourceLabel, a.ImageRef)
	}
	if want := time.Date(2026, 9, 29, 21, 27, 27, 367e6, portalLocation); !a.PublishedAt.Equal(want) {
		t.Errorf("published = %v, want %v (+07:00, not UTC)", a.PublishedAt, want)
	}
	if b.Articles[1].ImageRef != "http://portal.example.gov.vn/b.png" {
		t.Errorf("small image fallback = %q", b.Articles[1].ImageRef)
	}
	if _, err := parseArticles([]byte(`[{"TinTucID":`), since); !errors.Is(err, ErrParse) {
		t.Errorf("truncated JSON = %v, want parse", err)
	}
}

func TestResolveImage(t *testing.T) {
	base, _ := ParseBase("https://" + portalHost + "/DesktopModules/cttdt/api/apichiase")
	for ref, want := range map[string]string{
		"":                                     "",
		"/Portals/0/a.jpg":                     "https://portal.example.gov.vn/Portals/0/a.jpg",
		"//portal.example.gov.vn/a.jpg":        "https://portal.example.gov.vn/a.jpg",
		"http://portal.example.gov.vn/a.jpg":   "https://portal.example.gov.vn/a.jpg",
		"https://PORTAL.example.gov.vn/a":      "https://PORTAL.example.gov.vn/a",
		"https://other.example.gov.vn/a.jpg":   "foreign",
		"http://cdn.example.com/a.jpg":         "foreign",
		"javascript:alert(1)":                  "invalid",
		"a.jpg":                                "invalid",
		"/a b.jpg":                             "invalid",
		"https://portal.example.gov.vn:8443/a": "invalid",
	} {
		u, err := ResolveImage(base, ref)
		switch want {
		case "":
			if u != nil || err != nil {
				t.Errorf("%q: got %v %v, want nothing", ref, u, err)
			}
		case "foreign":
			if !errors.Is(err, ErrImageForeignHost) {
				t.Errorf("%q: err = %v, want foreign host", ref, err)
			}
		case "invalid":
			if !errors.Is(err, ErrImageInvalidURL) {
				t.Errorf("%q: err = %v, want invalid", ref, err)
			}
		default:
			if err != nil || u.String() != want {
				t.Errorf("%q: got %v %v, want %s", ref, u, err, want)
			}
		}
	}
}

func TestImageRedirectMayNotLeaveTheHost(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+otherHost+"/a.jpg", http.StatusFound)
	})
	u, _ := ResolveImage(p.endpoint(t).Base, "/a.jpg")
	if _, err := p.client.Image(context.Background(), p.endpoint(t), u, 1<<20); !errors.Is(err, ErrRedirectRefused) {
		t.Fatalf("err = %v, want redirect-refused", err)
	}
}

func TestImageCap(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(make([]byte, 2048)) })
	u, _ := ResolveImage(p.endpoint(t).Base, "/a.jpg")
	if _, err := p.client.Image(context.Background(), p.endpoint(t), u, 1024); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want too-large", err)
	}
	b, err := p.client.Image(context.Background(), p.endpoint(t), u, 4096)
	if err != nil || len(b) != 2048 {
		t.Fatalf("got %d bytes, %v", len(b), err)
	}
}

func TestPlainText(t *testing.T) {
	for in, want := range map[string]string{
		"  a \n\t b ":          "a b",
		"&lt;b&gt;x&lt;/b&gt;": "x",
		"<p>one</p><p>two</p>": "one two",
		"a\u0000b":             "a b",
	} {
		if got := PlainText(in); got != want {
			t.Errorf("PlainText(%q) = %q, want %q", in, got, want)
		}
	}
}
