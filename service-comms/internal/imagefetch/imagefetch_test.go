package imagefetch

// The guard runs exactly as in production: the tests replace the resolver, the socket and the trust pool
// (Options), never CheckURL or the AddrAllowed predicate the dialer asks — the portal tests' discipline.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	newsHost  = "img.news.example.com"
	otherHost = "cdn.other.example.net"
	evilHost  = "rebind.example.org" // a public-looking name that resolves inside the cluster
)

// publicAddr is what the fake resolver answers for the good hosts — PUBLIC, so the guard passes it as it
// would in production; the fake dialer then maps it onto the httptest listener.
var publicAddr = netip.MustParseAddr("93.184.216.34")

type rig struct {
	srv    *httptest.Server
	client *Client

	mu      sync.Mutex
	answers map[string][]netip.Addr
	dialed  []string
	paths   []string
	headers []http.Header
}

type fakeResolver struct{ r *rig }

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	f.r.mu.Lock()
	defer f.r.mu.Unlock()
	a, ok := f.r.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return a, nil
}

func newRig(t *testing.T, h http.HandlerFunc) *rig {
	t.Helper()
	cert, pool := testCertificate(t, newsHost, otherHost, evilHost)
	r := &rig{answers: map[string][]netip.Addr{newsHost: {publicAddr}, otherHost: {publicAddr}}}
	r.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.paths = append(r.paths, req.Host+req.URL.RequestURI())
		r.headers = append(r.headers, req.Header.Clone())
		r.mu.Unlock()
		h(w, req)
	}))
	r.srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	r.srv.StartTLS()
	t.Cleanup(r.srv.Close)
	target := r.srv.Listener.Addr().String()
	r.client = New(Options{Resolver: fakeResolver{r}, RootCAs: pool,
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			r.mu.Lock()
			r.dialed = append(r.dialed, addr)
			r.mu.Unlock()
			var d net.Dialer
			return d.DialContext(ctx, network, target)
		}})
	return r
}

func testCertificate(t *testing.T, hosts ...string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: hosts[0]}, DNSNames: hosts,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true,
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

func fetch(t *testing.T, r *rig, raw string, max int64) ([]byte, error) {
	t.Helper()
	u, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL(%q): %v", raw, err)
	}
	return r.client.Fetch(context.Background(), u, max)
}

// assertNoURL: no formatting of the error carries the path or the query (rule 3 — a pasted link can
// carry a token or a person's data).
func assertNoURL(t *testing.T, err error, fragments ...string) {
	t.Helper()
	for _, s := range []string{err.Error()} {
		for _, f := range fragments {
			if strings.Contains(s, f) {
				t.Errorf("the error quotes the URL (%q): %s", f, s)
			}
		}
	}
}

func TestParseURLShape(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://img.news.example.com/a/b.jpg?w=800#x":                   true,
		"https://IMG.News.Example.com:443/a.jpg":                         true,
		"https://xn--bo-tui-6va.vn/anh.png":                              true,
		"http://img.news.example.com/a.jpg":                              false, // not https
		"ftp://img.news.example.com/a.jpg":                               false,
		"https://img.news.example.com:8443/a.jpg":                        false, // port
		"https://user:pw@img.news.example.com/a.jpg":                     false, // userinfo
		"https://127.0.0.1/a.jpg":                                        false, // IP literal
		"https://[::1]/a.jpg":                                            false,
		"https://169.254.169.254/latest/meta-data":                       false,
		"https://127.1/a.jpg":                                            false, // digit TLD
		"https://localhost/a.jpg":                                        false, // single label
		"https://minio/a.jpg":                                            false,
		"https://img.news.example.com./a.jpg":                            false, // trailing dot
		"https://báo.vn/a.jpg":                                           false, // non-LDH; punycode is accepted
		"https:///a.jpg":                                                 false,
		"https:img.news.example.com/a.jpg":                               false, // opaque
		"":                                                               false,
		"https://img.news.example.com/" + strings.Repeat("a", MaxURLLen): false,
		"https://img.news.example.com/a b.jpg":                           false,
	} {
		u, err := ParseURL(raw)
		if ok != (err == nil) {
			t.Errorf("ParseURL(%q) err = %v, want ok=%v", raw, err, ok)
			continue
		}
		if err != nil {
			if !errors.Is(err, ErrURLRefused) {
				t.Errorf("ParseURL(%q) class = %s", raw, ClassOf(err))
			}
			continue
		}
		if u.Fragment != "" {
			t.Errorf("ParseURL(%q) kept the fragment", raw)
		}
	}
}

func TestFetchReturnsTheBodyThroughTheGuard(t *testing.T) {
	r := newRig(t, func(w http.ResponseWriter, req *http.Request) { _, _ = w.Write([]byte("PIXELS")) })
	b, err := fetch(t, r, "https://"+newsHost+"/a.jpg?token=abc", 1<<20)
	if err != nil || string(b) != "PIXELS" {
		t.Fatalf("Fetch = %q, %v", b, err)
	}
	// The socket went to the CHECKED address, never the name.
	if len(r.dialed) != 1 || r.dialed[0] != publicAddr.String()+":443" {
		t.Errorf("dialed %v", r.dialed)
	}
	h := r.headers[0]
	if h.Get("User-Agent") != UserAgent || h.Get("Cookie") != "" {
		t.Errorf("headers: UA=%q Cookie=%q", h.Get("User-Agent"), h.Get("Cookie"))
	}
}

func TestDialRefusesNonPublicResolution(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "::1", "192.168.0.10", "100.100.100.200",
		"::ffff:10.0.0.1", "fd00::1"} {
		t.Run(ip, func(t *testing.T) {
			r := newRig(t, func(w http.ResponseWriter, req *http.Request) { _, _ = w.Write([]byte("x")) })
			// One answer public, one not: ANY non-public answer refuses the whole name.
			r.answers[evilHost] = []netip.Addr{publicAddr, netip.MustParseAddr(ip)}
			_, err := fetch(t, r, "https://"+evilHost+"/secret?q=1", 1<<20)
			if !errors.Is(err, ErrAddressRefused) || !IsRefusal(err) {
				t.Fatalf("err = %v, want address-refused", err)
			}
			if len(r.dialed) != 0 {
				t.Errorf("a socket was opened: %v", r.dialed)
			}
			assertNoURL(t, err, "secret", "q=1", evilHost)
		})
	}
}

func TestRedirects(t *testing.T) {
	for name, tc := range map[string]struct {
		chain []string // Location of each hop; the last hop answers 200
		want  error
	}{
		"to another public host is followed": {[]string{"https://" + otherHost + "/b.jpg"}, nil},
		"three hops":                         {[]string{"/1", "/2", "/3"}, nil},
		"four hops":                          {[]string{"/1", "/2", "/3", "/4"}, ErrTooManyRedirects},
		"to a private IP literal":            {[]string{"https://10.0.0.1/x"}, ErrRedirectRefused},
		"to the metadata endpoint":           {[]string{"https://169.254.169.254/latest/meta-data"}, ErrRedirectRefused},
		"to http":                            {[]string{"http://" + otherHost + "/b.jpg"}, ErrRedirectRefused},
		"to a name resolving inside":         {[]string{"https://" + evilHost + "/x"}, ErrAddressRefused},
	} {
		t.Run(name, func(t *testing.T) {
			hop := 0
			var r *rig
			r = newRig(t, func(w http.ResponseWriter, req *http.Request) {
				if hop < len(tc.chain) {
					hop++
					http.Redirect(w, req, tc.chain[hop-1], http.StatusFound)
					return
				}
				_, _ = w.Write([]byte("OK"))
			})
			r.answers[evilHost] = []netip.Addr{netip.MustParseAddr("10.9.9.9")}
			b, err := fetch(t, r, "https://"+newsHost+"/start?token=zz", 1<<20)
			if tc.want == nil {
				if err != nil || string(b) != "OK" {
					t.Fatalf("Fetch = %q, %v", b, err)
				}
				for _, h := range r.headers {
					if h.Get("Referer") != "" {
						t.Errorf("a redirect carried the Referer: %q", h.Get("Referer"))
					}
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			assertNoURL(t, err, "token", "zz", "start")
		})
	}
}

func TestFetchStopsAtTheCap(t *testing.T) {
	var written int
	r := newRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		// No Content-Length: the cap must hold while streaming, not only on a declared length.
		fl := w.(http.Flusher)
		chunk := make([]byte, 32<<10)
		for i := 0; i < 64; i++ { // 2 MiB offered
			n, err := w.Write(chunk)
			written += n
			if err != nil {
				return
			}
			fl.Flush()
		}
	})
	_, err := fetch(t, r, "https://"+newsHost+"/big.jpg", 100<<10)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want too-large", err)
	}

	// A declared Content-Length over the cap is refused before the body is read.
	r2 := newRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Length", "5000")
		_, _ = w.Write(make([]byte, 5000))
	})
	if _, err := fetch(t, r2, "https://"+newsHost+"/big.jpg", 4999); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("declared: err = %v", err)
	}
}

func TestNon200IsAClassAndTheBodyIsNotKept(t *testing.T) {
	r := newRig(t, func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "nope "+req.URL.RawQuery, http.StatusNotFound)
	})
	_, err := fetch(t, r, "https://"+newsHost+"/a.jpg?sig=SECRETSIG", 1<<20)
	if err == nil || ClassOf(err) != "http-404" || IsRefusal(err) {
		t.Fatalf("err = %v", err)
	}
	assertNoURL(t, err, "SECRETSIG")
}

func TestTLSIsVerified(t *testing.T) {
	r := newRig(t, func(w http.ResponseWriter, req *http.Request) { _, _ = w.Write([]byte("x")) })
	_, other := testCertificate(t, "unrelated.example.com")
	target := r.srv.Listener.Addr().String()
	r.client = New(Options{Resolver: fakeResolver{r}, RootCAs: other,
		Dial: func(ctx context.Context, n, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, n, target)
		}})
	if _, err := fetch(t, r, "https://"+newsHost+"/a.jpg", 1<<20); !errors.Is(err, ErrTLS) {
		t.Fatalf("err = %v, want tls", err)
	}
}

func TestUnknownHostIsDNS(t *testing.T) {
	r := newRig(t, func(w http.ResponseWriter, req *http.Request) {})
	if _, err := fetch(t, r, "https://nowhere.example.com/a.jpg", 1<<20); !errors.Is(err, ErrDNS) {
		t.Fatalf("err = %v, want dns", err)
	}
}
