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
		// D4 (owner, 02/10/2026): another .gov.vn host is no longer enough — same host as api_url only.
		"another gov host": {[]string{"https://" + otherHost + "/x"}, ErrRedirectRefused},
		"same host, https": {[]string{"https://" + portalHost + "/elsewhere"}, nil},
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
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, 100, noneHeld)
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
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, 100, noneHeld)
		assertNoSecret(t, err)
	})
	t.Run("connect refused", func(t *testing.T) {
		p := newTestPortal(t, nil)
		p.client = New(Options{Resolver: fakeResolver{p}, Dial: func(ctx context.Context, n, a string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: n, Err: errors.New("connection refused")}
		}})
		_, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, 100, noneHeld)
		if !errors.Is(err, ErrConnect) {
			t.Fatalf("err = %v, want connect", err)
		}
		assertNoSecret(t, err)
	})
	t.Run("timeout", func(t *testing.T) {
		p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) })
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := p.client.Articles(ctx, p.endpoint(t), "120", time.Time{}, 100, noneHeld)
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
	if _, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, 100, noneHeld); err != nil {
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
		if _, err := p.client.Articles(context.Background(), p.endpoint(t), id, time.Time{}, 100, noneHeld); !errors.Is(err, ErrCategoryRequired) {
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
	b, err := parseArticles(context.Background(), []byte(body), since, 100, noneHeld)
	if err != nil {
		t.Fatal(err)
	}
	if b.Read != 4 { // 9001, 9002, 9003, 9005 — the deleted and the id-less rows are not articles
		t.Errorf("Read = %d, want 4", b.Read)
	}
	if len(b.Articles) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(b.Articles), b.Articles)
	}
	// Newest first: 9005 (30/09) before 9001 (29/09).
	a := b.Articles[1]
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
	if b.Articles[0].ExternalID != "9005" || b.Articles[0].ImageRef != "http://portal.example.gov.vn/b.png" {
		t.Errorf("small image fallback = %+v", b.Articles[0])
	}
	if _, err := parseArticles(context.Background(), []byte(`[{"TinTucID":`), since, 100, noneHeld); !errors.Is(err, ErrParse) {
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

// R1 (02/10/2026): a category answering many in-window rows keeps only the newest keep, newest first,
// ties in the portal's order — and Read still counts every usable row.
func TestParseArticlesKeepsOnlyTheNewest(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, portalLocation)
	var b strings.Builder
	b.WriteString("[")
	// 50 rows, published on days in a shuffled order; days 7 and 7 tie (ids 107, 207).
	days := []int{3, 17, 9, 25, 7, 1, 30, 12, 7, 22}
	for i := 0; i < 50; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		d := days[i%len(days)]
		id := fmt.Sprintf("%d%02d", i/10+1, d)
		if i%10 == 8 {
			id = fmt.Sprintf("%d%02d", i/10+2, d) // the second day-7 row of each block
		}
		fmt.Fprintf(&b, `{"TinTucID":"%s-%d","TieuDe":"T","NgayDang":"2026-03-%02dT08:00:00"}`, id, i, d)
	}
	b.WriteString("]")
	got, err := parseArticles(context.Background(), []byte(b.String()), since, 3, noneHeld)
	if err != nil {
		t.Fatal(err)
	}
	if got.Read != 50 {
		t.Fatalf("Read = %d, want 50", got.Read)
	}
	if len(got.Articles) != 3 {
		t.Fatalf("kept %d, want 3", len(got.Articles))
	}
	for i, a := range got.Articles {
		if a.PublishedAt.Day() != 30 {
			t.Fatalf("article %d is day %d, want the three day-30 rows", i, a.PublishedAt.Day())
		}
	}
	// The five day-30 rows are at positions 6, 16, 26, 36, 46: a tie keeps the portal's order.
	if got.Articles[0].ExternalID != "130-6" || got.Articles[1].ExternalID != "230-16" || got.Articles[2].ExternalID != "330-26" {
		t.Fatalf("tie order = %s %s %s", got.Articles[0].ExternalID, got.Articles[1].ExternalID, got.Articles[2].ExternalID)
	}
}

func TestArticlesRefusesAKeepOutsideItsBounds(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	for _, keep := range []int{0, -1, MaxKeepPerCategory + 1} {
		if _, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, keep, noneHeld); !errors.Is(err, ErrKeepInvalid) {
			t.Errorf("keep %d: err = %v, want keep-invalid", keep, err)
		}
	}
	if len(p.queries) != 0 {
		t.Fatal("a refused keep still called the portal")
	}
}

// R4 (02/10/2026): the IPv4-compatible and IPv4-translated IPv6 forms of a private v4 are refused.
func TestAddrAllowedRefusesEmbeddedIPv4Forms(t *testing.T) {
	for _, s := range []string{"::10.0.0.1", "::a00:1", "::169.254.169.254", "::ffff:0:10.0.0.1", "::ffff:0:a9fe:a9fe",
		"::ffff:0:93.184.216.34"} {
		if AddrAllowed(netip.MustParseAddr(s)) {
			t.Errorf("AddrAllowed(%s) = true, want refused", s)
		}
	}
}

func TestIsRefusal(t *testing.T) {
	for _, e := range []error{ErrURLRefused, ErrAddressRefused, ErrRedirectRefused, ErrTooManyRedirects,
		ErrImageForeignHost, ErrImageInvalidURL} {
		if !IsRefusal(e) {
			t.Errorf("%v is a refusal", e)
		}
	}
	for _, e := range []error{ErrTimeout, ErrConnect, ErrParse, httpStatusError(503), errors.New("x"), nil} {
		if IsRefusal(e) {
			t.Errorf("%v is not a refusal", e)
		}
	}
}

// noneHeld is a HeldFunc for a commune that holds nothing yet.
func noneHeld(context.Context, []string) (map[string]bool, error) { return nil, nil }

// heldRecorder holds a fixed set and records every chunk it was asked about.
type heldRecorder struct {
	held   map[string]bool
	chunks [][]string
}

func (h *heldRecorder) fn(_ context.Context, ids []string) (map[string]bool, error) {
	h.chunks = append(h.chunks, append([]string(nil), ids...))
	out := map[string]bool{}
	for _, id := range ids {
		if d, ok := h.held[id]; ok {
			out[id] = d
		}
	}
	return out, nil
}

// manyArticles is n in-window rows, ids "1".."n", row i published i hours after the window start —
// so the highest ids are the newest.
func manyArticles(n int) []byte {
	var b strings.Builder
	b.WriteString("[")
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteString(",")
		}
		at := time.Date(2026, 3, 1, 0, 0, 0, 0, portalLocation).Add(time.Duration(i) * time.Hour)
		fmt.Fprintf(&b, `{"TinTucID":"%d","TieuDe":"T","NgayDang":"%s"}`, i, at.Format("2006-01-02T15:04:05"))
	}
	b.WriteString("]")
	return []byte(b.String())
}

// Follow-up 02/10/2026: already-held rows (live OR soft-deleted) never take a heap slot, so the newest
// NOT-held ones are kept — the next run continues below what this one imported.
func TestParseArticlesHeldRowsNeverOccupySlots(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, portalLocation)
	h := &heldRecorder{held: map[string]bool{"10": false, "9": true, "7": false}}
	got, err := parseArticles(context.Background(), manyArticles(10), since, 3, h.fn)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, a := range got.Articles {
		ids = append(ids, a.ExternalID)
	}
	if strings.Join(ids, ",") != "8,6,5" {
		t.Fatalf("kept %v, want the newest three not held: 8,6,5", ids)
	}
	if got.HeldLive != 2 || got.HeldDeleted != 1 || got.Read != 10 {
		t.Fatalf("held live %d deleted %d read %d", got.HeldLive, got.HeldDeleted, got.Read)
	}

	// Run two: what run one kept is now held too → the next three.
	for _, id := range ids {
		h.held[id] = false
	}
	got, err = parseArticles(context.Background(), manyArticles(10), since, 3, h.fn)
	if err != nil {
		t.Fatal(err)
	}
	ids = ids[:0]
	for _, a := range got.Articles {
		ids = append(ids, a.ExternalID)
	}
	if strings.Join(ids, ",") != "4,3,2" {
		t.Fatalf("second run kept %v, want 4,3,2", ids)
	}
}

// The lookup is batched: at most heldChunk ids per call, and a large category is asked in chunks.
func TestParseArticlesAsksHeldInChunks(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, portalLocation)
	h := &heldRecorder{held: map[string]bool{}}
	// Ascending dates: every row is better than the heap's worst, so every row is buffered and asked.
	if _, err := parseArticles(context.Background(), manyArticles(450), since, 100, h.fn); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, c := range h.chunks {
		if len(c) > heldChunk {
			t.Fatalf("a chunk of %d ids, ceiling %d", len(c), heldChunk)
		}
		total += len(c)
	}
	if len(h.chunks) != 3 || total != 450 {
		t.Fatalf("chunks = %d holding %d ids, want 3 holding 450", len(h.chunks), total)
	}
}

func TestParseArticlesHeldFailureIsAClass(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, portalLocation)
	fail := func(context.Context, []string) (map[string]bool, error) {
		return nil, errors.New("pq: secret_code=leak")
	}
	_, err := parseArticles(context.Background(), manyArticles(5), since, 3, fail)
	if !errors.Is(err, ErrHeldLookup) || strings.Contains(err.Error(), "secret_code") {
		t.Fatalf("err = %v, want held-lookup and nothing of the caller's error", err)
	}
}

func TestArticlesRefusesANilHeldFunc(t *testing.T) {
	p := newTestPortal(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	if _, err := p.client.Articles(context.Background(), p.endpoint(t), "120", time.Time{}, 10, nil); !errors.Is(err, ErrHeldLookup) {
		t.Fatalf("err = %v, want held-lookup", err)
	}
	if len(p.queries) != 0 {
		t.Fatal("the portal was called without a held lookup")
	}
}
