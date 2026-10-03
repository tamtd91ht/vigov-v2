package app

// FetchBodyImage (content_body_image_url.go) over the cover rig (content_cover_test.go): the REAL content
// store on the fake driver, in-memory rows and objects — and, for the network, the REAL imagefetch client
// against an httptest TLS server, reached through an injected resolver, socket and trust pool. The guard
// (URL shape, AddrAllowed at dial, redirect re-checks) is never replaced.

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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/imagefetch"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	fetchHost   = "img.news.example.com"
	fetchedFile = "01JKKKKKKKKKKKKKKKKKKKKKKK"
	// The path and query carry markers a leak into the trail would be found by.
	fetchURL = "https://" + fetchHost + "/2026/PATHMARK.jpg?token=QUERYMARK"
)

// fetchRig is the cover rig plus a TLS image server and the real client wired to reach it.
type fetchRig struct {
	*coverRig
	srv     *httptest.Server
	answers map[string][]netip.Addr
	hits    int
}

type mapResolver map[string][]netip.Addr

func (m mapResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	a, ok := m[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return a, nil
}

func newFetchRig(t *testing.T, h http.HandlerFunc) *fetchRig {
	t.Helper()
	r := &fetchRig{coverRig: newCoverRig(t, nil),
		answers: map[string][]netip.Addr{fetchHost: {netip.MustParseAddr("93.184.216.34")}}}
	r.uc.policies = bodyPolicy(20)
	ids := []string{coverItemID, fetchedFile, coverOtherID} // the third: a later upload in the same test
	r.uc.newID = func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil }

	cert, pool := fetchTestCertificate(t, fetchHost)
	r.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits++
		h(w, req)
	}))
	r.srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	r.srv.StartTLS()
	t.Cleanup(r.srv.Close)
	target := r.srv.Listener.Addr().String()
	r.uc.WithImageFetcher(imagefetch.New(imagefetch.Options{Resolver: mapResolver(r.answers), RootCAs: pool,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, target)
		}}))
	return r
}

func fetchTestCertificate(t *testing.T, hosts ...string) (tls.Certificate, *x509.CertPool) {
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

// auditTexts is every string / []byte argument of the audit INSERTs, the delta JSON included.
func auditTexts(k *khoNDGia) []string {
	var out []string
	for _, l := range k.cau("INSERT INTO audit_log") {
		for _, a := range l.args {
			switch v := a.(type) {
			case string:
				out = append(out, v)
			case []byte:
				out = append(out, string(v))
			}
		}
	}
	return out
}

func TestFetchBodyImageStoresAReadyImageWithPreviewAndTrailsTheHostOnly(t *testing.T) {
	jpg := testJPEG(t, 1600, 900, 0)
	r := newFetchRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html") // a lie: the declared type is never read
		_, _ = w.Write(jpg)
	})
	got, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND())
	if err != nil {
		t.Fatalf("FetchBodyImage: %v", err)
	}
	if got.ID != fetchedFile || got.Status != domain.StoredFileReady || got.SubjectID != coverItemID ||
		got.Purpose != string(storage.PurposeContentBodyImage) || got.UploadedBy != maCanBoSoanND {
		t.Fatalf("row = %+v", got)
	}
	if !strings.Contains(got.ObjectKey, "/comms/content-body-image/") || !strings.HasSuffix(got.ObjectKey, "/thumb-1280.jpg") {
		t.Errorf("object key = %q, want the body-image derivative", got.ObjectKey)
	}
	if _, ok := r.objects.produced[got.ObjectKey]; !ok || r.scanner.scanned != 1 || r.hits != 1 {
		t.Errorf("derivative stored=%v scanned=%d hits=%d", ok, r.scanner.scanned, r.hits)
	}
	if strings.Contains(got.OriginalName, "PATHMARK") {
		t.Errorf("the remote file name became original_name: %q", got.OriginalName)
	}
	// Walked to ready through every guarded edge, in the write transaction.
	if strings.Join(r.files.transitions, ",") != "pending>scanning,scanning>stored,stored>processing,processing>ready" {
		t.Errorf("transitions = %v", r.files.transitions)
	}
	// Two transactions: the pre-check (no write), then row + trail together.
	if r.k.batDau != 2 || r.k.daCommit != 2 || len(r.k.cau("INSERT INTO audit_log")) != 1 {
		t.Errorf("tx begin=%d commit=%d audit=%d", r.k.batDau, r.k.daCommit, len(r.k.cau("INSERT INTO audit_log")))
	}
	texts := strings.Join(auditTexts(r.k), "\n")
	for _, want := range []string{ActionBodyImageFetched, maCanBoSoanND, `"may_chu_nguon":"` + fetchHost + `"`,
		"noi-dung-mini-app/anh-than-bai/2026-10-01"} {
		if !strings.Contains(texts, want) {
			t.Errorf("trail lacks %q: %s", want, texts)
		}
	}
	for _, leak := range []string{"PATHMARK", "QUERYMARK", "token"} {
		if strings.Contains(texts, leak) {
			t.Errorf("the trail carries the URL (%q): %s", leak, texts)
		}
	}

	// The editor's preview: the same signed derivative link as the upload's completion.
	views, err := r.uc.BodyImageViews(r.ctx, coverItemID, []string{got.ID})
	if err != nil || len(views) != 1 || views[0].PreviewURL == "" || views[0].Status != domain.StoredFileReady {
		t.Fatalf("views = %+v, %v", views, err)
	}

	// The reservation it made lets the SAME officer's next upload join the article.
	_, err = r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{ContentItemID: coverItemID, FileName: "b.jpg",
		ContentType: storage.MIMEJPEG, Size: 10}, nguoiSoanND())
	if errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Errorf("the fetched image did not reserve its article: %v", err)
	}
}

func TestFetchBodyImageRefusesTheURLShapeBeforeAnyNetwork(t *testing.T) {
	for _, raw := range []string{
		"http://img.news.example.com/a.jpg", "https://img.news.example.com:8443/a.jpg",
		"https://u:p@img.news.example.com/a.jpg", "https://127.0.0.1/a.jpg", "https://[::1]/a.jpg",
		"https://169.254.169.254/latest/meta-data", "https://localhost/a.jpg", "",
		"https://img.news.example.com/" + strings.Repeat("a", 2048),
	} {
		r := newFetchRig(t, func(w http.ResponseWriter, req *http.Request) {})
		_, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: raw}, nguoiSoanND())
		if !errors.Is(err, ErrImageURLInvalid) {
			t.Errorf("%q: err = %v, want ErrImageURLInvalid", raw, err)
		}
		if r.hits != 0 || r.k.batDau != 0 || strings.Contains(err.Error(), "img.news") {
			t.Errorf("%q: hits=%d tx=%d err=%v", raw, r.hits, r.k.batDau, err)
		}
	}
}

func TestFetchBodyImageNetworkRefusalsAreOneFailureAndWriteNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		handler http.HandlerFunc
		answer  string // what fetchHost resolves to
		refused bool
	}{
		"redirect to a private IP literal": {func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://10.0.0.1/x", http.StatusFound)
		}, "93.184.216.34", true},
		"redirect to the metadata endpoint": {func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://169.254.169.254/latest/meta-data", http.StatusFound)
		}, "93.184.216.34", true},
		"more than three redirects": {func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/again"+req.URL.Path, http.StatusFound)
		}, "93.184.216.34", true},
		"resolves to loopback": {nil, "127.0.0.1", true},
		"resolves to 10.x":     {nil, "10.20.30.40", true},
		"resolves to metadata": {nil, "169.254.169.254", true},
		"resolves to ::1":      {nil, "::1", true},
		"non-200": {func(w http.ResponseWriter, req *http.Request) {
			http.Error(w, "gone", http.StatusNotFound)
		}, "93.184.216.34", false},
	} {
		t.Run(name, func(t *testing.T) {
			h := tc.handler
			if h == nil {
				h = func(w http.ResponseWriter, req *http.Request) { _, _ = w.Write([]byte("never")) }
			}
			r := newFetchRig(t, h)
			r.answers[fetchHost] = []netip.Addr{netip.MustParseAddr(tc.answer)}
			_, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND())
			var fe *ImageFetchError
			if !errors.Is(err, ErrImageFetchFailed) || !errors.As(err, &fe) || fe.Refused != tc.refused {
				t.Fatalf("err = %v (%+v), want ErrImageFetchFailed refused=%v", err, fe, tc.refused)
			}
			if tc.answer != "93.184.216.34" && r.hits != 0 {
				t.Errorf("a non-public resolution reached the server (%d hits)", r.hits)
			}
			if len(r.files.inserted) != 0 || len(r.objects.produced) != 0 || r.scanner.scanned != 0 ||
				r.k.coCau("INSERT INTO audit_log") {
				t.Errorf("a failed fetch wrote something: rows=%d objects=%d scanned=%d",
					len(r.files.inserted), len(r.objects.produced), r.scanner.scanned)
			}
			for _, leak := range []string{"PATHMARK", "QUERYMARK", fetchHost} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("the error carries %q: %v", leak, err)
				}
			}
		})
	}
}

func TestFetchBodyImageStopsAtThePolicyCapAndTheMemoryBound(t *testing.T) {
	big := make([]byte, 64<<10)
	copy(big, testJPEG(t, 10, 10, 0))
	r := newFetchRig(t, func(w http.ResponseWriter, req *http.Request) { _, _ = w.Write(big) })
	pol := bodyPolicy(20)
	pol.p.MaxBytes = 16 << 10 // the POLICY is smaller than the body: it decides
	r.uc.policies = pol
	_, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND())
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectTooLarge {
		t.Fatalf("err = %v, want too-large rejection", err)
	}
	if r.scanner.scanned != 0 || len(r.files.inserted) != 0 {
		t.Error("an oversize download went on to the scanner or a row")
	}

	// The policy's 50 MB never reaches the client: the memory bound is the cap asked for.
	rec := &capRecorder{}
	r2 := newCoverRig(t, nil)
	r2.uc.policies = bodyPolicy(20)
	r2.uc.WithImageFetcher(rec)
	_, _ = r2.uc.FetchBodyImage(r2.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND())
	if rec.max != bodyImageFetchMaxBytes {
		t.Errorf("cap asked = %d, want the memory bound %d under a 50 MB policy", rec.max, bodyImageFetchMaxBytes)
	}
}

type capRecorder struct{ max int64 }

func (c *capRecorder) Fetch(_ context.Context, _ *url.URL, max int64) ([]byte, error) {
	c.max = max
	return nil, imagefetch.ErrTooLarge
}

func TestFetchBodyImageRefusesANonImage(t *testing.T) {
	r := newFetchRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg") // a lie: the bytes decide
		_, _ = w.Write([]byte("<html><body>not an image</body></html>"))
	})
	_, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND())
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectTypeNotAllowed {
		t.Fatalf("err = %v, want type rejection", err)
	}
	if len(r.files.inserted) != 0 || len(r.objects.produced) != 0 || r.k.coCau("INSERT INTO audit_log") {
		t.Error("a non-image left a row, an object or an entry")
	}
}

func TestFetchBodyImageChecksTheArticleAndTheCountBeforeFetching(t *testing.T) {
	// The 21st image: refused before the network.
	r := newFetchRig(t, func(w http.ResponseWriter, req *http.Request) {})
	r.uc.policies = bodyPolicy(1)
	readyBodyImage(r.files, bodyFileA, coverItemID)
	_, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL, ContentItemID: coverItemID}, nguoiSoanND())
	if !errors.Is(err, ErrCoverCountReached) || r.hits != 0 {
		t.Fatalf("21st: err = %v hits = %d", err, r.hits)
	}

	// Another officer's reservation: the 404 of an unknown article, no fetch.
	r = newFetchRig(t, func(w http.ResponseWriter, req *http.Request) {})
	f := readyBodyImage(r.files, bodyFileA, coverItemID)
	f.UploadedBy = "CB-2026-KHAC00"
	_, err = r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL, ContentItemID: coverItemID}, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) || r.hits != 0 {
		t.Fatalf("foreign reservation: err = %v hits = %d", err, r.hits)
	}
}

func TestFetchBodyImageFailsClosedWithoutTheFetcherOrScanner(t *testing.T) {
	r := newCoverRig(t, nil)
	r.uc.policies = bodyPolicy(20)
	if _, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no fetcher: err = %v", err)
	}
	r.uc.WithImageFetcher(&capRecorder{})
	r.uc.scanner = nil
	if _, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL}, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no scanner: err = %v", err)
	}
}
