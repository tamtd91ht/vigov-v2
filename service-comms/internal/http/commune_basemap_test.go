package http

// THE PUBLIC BASEMAP ROUTE (ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M1) — the handler's own half:
// the closed file list, what crosses to storage and back, the statuses passed through, the one answer
// for "not configured" and "no such commune", the policy and key of the rate limit, and what the log
// never carries. The CORS answer and the outer split live in cmd/server/bien_cong_khai_test.go.
//
// Rule 5 invariant 7 adapted to Public: no principal, so no 401/403; "wrong commune" becomes "a domain
// no active commune holds gets nothing" — the same 503 as an unset BASEMAP_URL.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/ratelimit"
)

const (
	bmPrefix  = "/vigov-test-public/config/map/20261011/"
	bmETag    = `"archive-v1"`
	bmArchive = "0123456789abcdefghijklmnopqrstuvwxyz" // 36 bytes of a fake archive
)

// bmStorage is the fake MinIO: it records each request it received and serves the archive with real
// range / conditional semantics (http.ServeContent), or a scripted status.
type bmStorage struct {
	mu     sync.Mutex
	seen   []*http.Request
	status int           // != 0: answer this status with an XML body instead of a file
	delay  time.Duration // before any header
}

func (s *bmStorage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.seen = append(s.seen, r.Clone(context.Background()))
	status, delay := s.status, s.delay
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if status != 0 {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, "<Error><BucketName>vigov-test-public</BucketName></Error>")
		return
	}
	// Storage's own content type, which the route must NOT copy.
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("ETag", bmETag)
	http.ServeContent(w, r, "x", time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), strings.NewReader(bmArchive))
}

func (s *bmStorage) requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*http.Request(nil), s.seen...)
}

type bmRig struct {
	h       http.Handler
	storage *bmStorage
	counter *memCounter
	log     *bytes.Buffer
}

// bmIdle bounds a silent fake storage. It also covers the TLS handshake of a fresh connection, which on a
// loaded Windows runner (full `make test`) took longer than 200ms and turned every case into a 504 — so
// it is generous, and the "silent" case waits well past it.
const bmIdle = 3 * time.Second

// bmServer mounts the route over a TLS fake storage. configured=false leaves BASEMAP_URL unset.
func bmServer(t *testing.T, configured bool) *bmRig {
	t.Helper()
	st := &bmStorage{}
	srv := httptest.NewTLSServer(st)
	t.Cleanup(srv.Close)
	base := ""
	if configured {
		base = srv.URL + bmPrefix
	}
	c := &memCounter{}
	lim, err := ratelimit.New(c, ratelimit.PublicBasemapRead)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	mux := http.NewServeMux()
	RegisterPublicBasemap(mux, PublicBasemapDeps{Xa: &ckNenTang{}, BaseURL: base, Client: srv.Client(),
		Limiter: lim, IdleTimeout: bmIdle, Log: slog.New(slog.NewJSONHandler(&buf, nil))})
	return &bmRig{h: mux, storage: st, counter: c, log: &buf}
}

// bmReq calls the route with ?host= and the given headers ("Name: value").
func (rg *bmRig) do(method, file, host string, headers ...string) *httptest.ResponseRecorder {
	q := ""
	if host != "" {
		q = "?host=" + url.QueryEscape(host)
	}
	r := httptest.NewRequest(method, "https://comms.api.vigov.vn"+PublicBasemapPath+file+q, nil)
	r.RemoteAddr = "203.0.113.7:5000"
	for _, h := range headers {
		k, v, _ := strings.Cut(h, ": ")
		r.Header.Add(k, v)
	}
	w := httptest.NewRecorder()
	rg.h.ServeHTTP(w, r)
	return w
}

// The closed set = web-admin/src/lib/basemap/assets.ts: the archive, the three font stacks in 256-point
// ranges up to U+FFFF, the four sprite files. Each maps to ITS object key and OUR content type.
func TestPublicBasemapServesExactlyTheClosedSet(t *testing.T) {
	cases := []struct{ file, key, ct, cache string }{
		{"vn-mainland.pmtiles", "vn-mainland.pmtiles", "application/octet-stream", "public, no-cache"},
		{"fonts/Noto%20Sans%20Regular/0-255.pbf", "fonts/Noto%20Sans%20Regular/0-255.pbf", "application/x-protobuf", "public, max-age=86400"},
		{"fonts/Noto%20Sans%20Medium/7680-7935.pbf", "fonts/Noto%20Sans%20Medium/7680-7935.pbf", "application/x-protobuf", "public, max-age=86400"},
		{"fonts/Noto%20Sans%20Italic/65280-65535.pbf", "fonts/Noto%20Sans%20Italic/65280-65535.pbf", "application/x-protobuf", "public, max-age=86400"},
		{"sprites/light.json", "sprites/light.json", "application/json", "public, max-age=86400"},
		{"sprites/light.png", "sprites/light.png", "image/png", "public, max-age=86400"},
		{"sprites/light@2x.json", "sprites/light@2x.json", "application/json", "public, max-age=86400"},
		{"sprites/light@2x.png", "sprites/light@2x.png", "image/png", "public, max-age=86400"},
	}
	for _, c := range cases {
		rg := bmServer(t, true)
		w := rg.do(http.MethodGet, c.file, ckHostA)
		if w.Code != http.StatusOK || w.Body.String() != bmArchive {
			t.Errorf("%s: %d %q", c.file, w.Code, w.Body.String())
			continue
		}
		if got := w.Header().Get("Content-Type"); got != c.ct {
			t.Errorf("%s: Content-Type %q, want %q (never storage's)", c.file, got, c.ct)
		}
		if got := w.Header().Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: Cache-Control %q, want %q", c.file, got, c.cache)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: no nosniff", c.file)
		}
		reqs := rg.storage.requests()
		if len(reqs) != 1 || reqs[0].URL.EscapedPath() != bmPrefix+c.key {
			t.Errorf("%s: storage asked %v, want %s", c.file, reqs, bmPrefix+c.key)
		}
	}
}

// Anything off the list is 404 and storage is never asked — traversal, encoded slashes, other stacks,
// misaligned or out-of-range glyph ranges, other sprites, extra segments.
func TestPublicBasemapRefusesEverythingElse(t *testing.T) {
	for _, file := range []string{
		"", "other.pmtiles", "vn-mainland.pmtiles/x", "VN-MAINLAND.pmtiles",
		"../vn-mainland.pmtiles", "%2e%2e/vn-mainland.pmtiles", "fonts%2FNoto%20Sans%20Regular%2F0-255.pbf",
		"fonts/Noto%20Sans%20Bold/0-255.pbf", "fonts/Noto%20Sans%20Regular/1-256.pbf",
		"fonts/Noto%20Sans%20Regular/0-511.pbf", "fonts/Noto%20Sans%20Regular/65536-65791.pbf",
		"fonts/Noto%20Sans%20Regular/0-255.pbf/x", "fonts/..%2F..%2Fsecret/0-255.pbf",
		"sprites/dark.json", "sprites/light.svg", "sprites/../vn-mainland.pmtiles",
	} {
		rg := bmServer(t, true)
		w := rg.do(http.MethodGet, file, ckHostA)
		if strings.Contains(file, "../") {
			// A literal dot-segment never reaches the handler: ServeMux redirects to the CLEANED path,
			// which is then judged like any other. Nothing is served and storage is never asked.
			if w.Code != http.StatusTemporaryRedirect || len(rg.storage.requests()) != 0 {
				t.Errorf("%q: %d, storage asked %d", file, w.Code, len(rg.storage.requests()))
			}
			continue
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("%q: %d, want 404", file, w.Code)
			continue
		}
		if e := loiTra(t, w); e.Code != "not_found" {
			t.Errorf("%q: code %q", file, e.Code)
		}
		if n := len(rg.storage.requests()); n != 0 {
			t.Errorf("%q: storage asked %d times", file, n)
		}
	}
}

// Only the range and conditional headers cross to storage — never the caller's cookie, token or origin
// — and the 206 comes back with what PMTiles reads.
func TestPublicBasemapForwardsRangeAndPasses206(t *testing.T) {
	rg := bmServer(t, true)
	w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA,
		"Range: bytes=10-19", "If-Range: "+bmETag, "Cookie: s=abc", "Authorization: Bearer x",
		"Origin: https://h5.zdn.vn", "X-Tenant-Id: 01JX")
	if w.Code != http.StatusPartialContent {
		t.Fatalf("range read: %d %s", w.Code, w.Body.String())
	}
	if w.Body.String() != bmArchive[10:20] {
		t.Fatalf("body %q", w.Body.String())
	}
	for k, want := range map[string]string{
		"Content-Range": "bytes 10-19/36", "Content-Length": "10", "Accept-Ranges": "bytes", "ETag": bmETag,
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	up := rg.storage.requests()[0]
	if up.Header.Get("Range") != "bytes=10-19" || up.Header.Get("If-Range") != bmETag {
		t.Errorf("range headers not forwarded: %v", up.Header)
	}
	if up.Header.Get("Accept-Encoding") != "identity" {
		t.Errorf("Accept-Encoding = %q, want identity", up.Header.Get("Accept-Encoding"))
	}
	for _, h := range []string{"Cookie", "Authorization", "Origin", "X-Tenant-Id"} {
		if up.Header.Get(h) != "" {
			t.Errorf("%s reached storage", h)
		}
	}
	if up.URL.RawQuery != "" {
		t.Errorf("the query reached storage: %q", up.URL.RawQuery)
	}
}

// 304 / 412 / 416 pass through WITHOUT storage's body (its XML names the bucket); 416 keeps Content-Range.
func TestPublicBasemapPassesConditionalStatuses(t *testing.T) {
	cases := []struct {
		name   string
		header string
		status int
	}{
		{"not modified", "If-None-Match: " + bmETag, http.StatusNotModified},
		{"precondition failed", `If-Match: "archive-v0"`, http.StatusPreconditionFailed},
		{"range not satisfiable", "Range: bytes=100-200", http.StatusRequestedRangeNotSatisfiable},
	}
	for _, c := range cases {
		rg := bmServer(t, true)
		w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA, c.header)
		if w.Code != c.status {
			t.Errorf("%s: %d, want %d", c.name, w.Code, c.status)
			continue
		}
		if w.Body.Len() != 0 {
			t.Errorf("%s: body passed through: %q", c.name, w.Body.String())
		}
		if c.status == http.StatusRequestedRangeNotSatisfiable && w.Header().Get("Content-Range") != "bytes */36" {
			t.Errorf("416 without Content-Range: %v", w.Header())
		}
	}
}

func TestPublicBasemapHeadHasLengthNoBody(t *testing.T) {
	rg := bmServer(t, true)
	w := rg.do(http.MethodHead, "vn-mainland.pmtiles", ckHostA)
	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "36" {
		t.Fatalf("HEAD: %d len=%q body=%q", w.Code, w.Header().Get("Content-Length"), w.Body.String())
	}
	if m := rg.storage.requests()[0].Method; m != http.MethodHead {
		t.Fatalf("storage asked with %s", m)
	}
}

func TestPublicBasemapOnlyGetAndHead(t *testing.T) {
	rg := bmServer(t, true)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if w := rg.do(m, "vn-mainland.pmtiles", ckHostA); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", m, w.Code)
		}
	}
	if n := len(rg.storage.requests()); n != 0 {
		t.Fatalf("storage asked %d times", n)
	}
}

// BASEMAP_URL unset, a domain no commune holds and an INACTIVE commune: one answer, byte for byte, and
// never a fallback host.
func TestPublicBasemapUnconfiguredAndUnknownHostAreOneAnswer(t *testing.T) {
	unset := bmServer(t, false).do(http.MethodGet, "vn-mainland.pmtiles", ckHostA)
	if unset.Code != http.StatusServiceUnavailable {
		t.Fatalf("unset: %d", unset.Code)
	}
	if e := loiTra(t, unset); e.Code != "basemap_not_configured" || e.Message != "Chưa cấu hình bản đồ nền." {
		t.Fatalf("unset: %+v", e)
	}
	for _, host := range []string{ckHostKhongCo, ckHostNgung, ckHostRieng} {
		rg := bmServer(t, true)
		w := rg.do(http.MethodGet, "vn-mainland.pmtiles", host)
		if w.Code != unset.Code || w.Body.String() != unset.Body.String() {
			t.Errorf("%s: %d %q — differs from the unset answer", host, w.Code, w.Body.String())
		}
		if n := len(rg.storage.requests()); n != 0 {
			t.Errorf("%s: storage asked", host)
		}
	}
	// Unconfigured is ONE answer for every path, junk included (web-admin's order).
	if w := bmServer(t, false).do(http.MethodGet, "etc/passwd", ckHostA); w.Body.String() != unset.Body.String() {
		t.Errorf("unset + junk path: %d %q", w.Code, w.Body.String())
	}
}

func TestPublicBasemapRefusesBadHost(t *testing.T) {
	for _, q := range []string{"", "not a host"} {
		rg := bmServer(t, true)
		if w := rg.do(http.MethodGet, "vn-mainland.pmtiles", q); w.Code != http.StatusBadRequest {
			t.Errorf("host %q: %d, want 400", q, w.Code)
		}
		if len(rg.counter.keys) != 0 {
			t.Errorf("host %q: a 400 was counted", q)
		}
	}
}

// Storage failures: its 404 is ours, anything else unlisted is 502, unreachable 502, silent 504 — and
// the log carries the commune and an outcome, NEVER the path, the Range or the storage address.
func TestPublicBasemapStorageFailures(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(rg *bmRig)
		status int
		code   string
	}{
		{"missing object", func(rg *bmRig) { rg.storage.status = http.StatusNotFound }, http.StatusNotFound, "not_found"},
		{"storage error", func(rg *bmRig) { rg.storage.status = http.StatusInternalServerError }, http.StatusBadGateway, "basemap_unavailable"},
		{"forbidden", func(rg *bmRig) { rg.storage.status = http.StatusForbidden }, http.StatusBadGateway, "basemap_unavailable"},
		{"silent", func(rg *bmRig) { rg.storage.delay = 3 * bmIdle }, http.StatusGatewayTimeout, "basemap_unavailable"},
	}
	for _, c := range cases {
		rg := bmServer(t, true)
		c.setup(rg)
		w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA, "Range: bytes=16384-32767")
		if w.Code != c.status {
			t.Errorf("%s: %d, want %d", c.name, w.Code, c.status)
			continue
		}
		e := loiTra(t, w)
		if e.Code != c.code || strings.Contains(w.Body.String(), "vigov-test-public") {
			t.Errorf("%s: body %s", c.name, w.Body.String())
		}
		log := rg.log.String()
		for _, leak := range []string{"16384", "pmtiles", "127.0.0.1", "vigov-test-public"} {
			if strings.Contains(log, leak) {
				t.Errorf("%s: the log carries %q: %s", c.name, leak, log)
			}
		}
		if !strings.Contains(log, string(xaA)) {
			t.Errorf("%s: the log does not name the commune: %s", c.name, log)
		}
	}

	// Unreachable: a closed server.
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	client, base := srv.Client(), srv.URL+bmPrefix
	srv.Close()
	lim, _ := ratelimit.New(&memCounter{}, ratelimit.PublicBasemapRead)
	mux := http.NewServeMux()
	RegisterPublicBasemap(mux, PublicBasemapDeps{Xa: &ckNenTang{}, BaseURL: base, Client: client, Limiter: lim,
		Log: slog.New(slog.DiscardHandler)})
	w := (&bmRig{h: mux}).do(http.MethodGet, "vn-mainland.pmtiles", ckHostA)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("unreachable: %d, want 502", w.Code)
	}
}

// The policy is PublicBasemapRead (its own name and budget), keyed `t:<tenant_id>` from the platform's
// answer; an unknown host is keyed by host with no tenant, at the same threshold.
func TestPublicBasemapRateLimitKey(t *testing.T) {
	rg := bmServer(t, true)
	rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA)
	rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostKhongCo)
	want := []string{
		"t:" + string(xaA) + ":rl:public-basemap:host:" + ckHostA + ":ip:203.0.113.7",
		"rl:public-basemap:host:" + ckHostKhongCo + ":ip:203.0.113.7",
	}
	if strings.Join(rg.counter.keys, "|") != strings.Join(want, "|") {
		t.Fatalf("keys = %v\nwant   %v", rg.counter.keys, want)
	}
}

// 6000 per minute, then 429 with Retry-After — counted before the file is even looked at.
func TestPublicBasemapIs429PastTheLimit(t *testing.T) {
	rg := bmServer(t, false) // unconfigured: the cheapest answer, still counted
	for i := 0; i < ratelimit.PublicBasemapReadLimit; i++ {
		if w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
	}
	w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("request %d: %d, want 429 + Retry-After", ratelimit.PublicBasemapReadLimit+1, w.Code)
	}
}

// FAILS CLOSED: the ADR decided the number, not the outage behaviour — Redis down is 503, storage untouched.
func TestPublicBasemapFailsClosedWhenTheCounterStoreIsDown(t *testing.T) {
	rg := bmServer(t, true)
	rg.counter.fail = errors.New("dial tcp 10.0.0.9:6379: connection refused")
	w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Redis down: %d, want 503", w.Code)
	}
	if e := loiTra(t, w); e.Code != "rate_limit_unavailable" {
		t.Fatalf("code %q", e.Code)
	}
	if n := len(rg.storage.requests()); n != 0 {
		t.Fatal("storage asked while the limit could not be enforced")
	}
}

// The literal in the route statement and the exported prefix cmd/server mounts must agree.
func TestPublicBasemapPathConstantMatchesRoute(t *testing.T) {
	rg := bmServer(t, true)
	if w := rg.do(http.MethodGet, "vn-mainland.pmtiles", ckHostA); w.Code != http.StatusOK {
		t.Fatalf("%svn-mainland.pmtiles: %d", PublicBasemapPath, w.Code)
	}
}

// Wiring refusals: a public route without its limiter, lookup or client must not start.
func TestRegisterPublicBasemapRefusesMissingDeps(t *testing.T) {
	lim, _ := ratelimit.New(&memCounter{}, ratelimit.PublicBasemapRead)
	for name, d := range map[string]PublicBasemapDeps{
		"no lookup":  {Client: http.DefaultClient, Limiter: lim},
		"no client":  {Xa: &ckNenTang{}, Limiter: lim},
		"no limiter": {Xa: &ckNenTang{}, Client: http.DefaultClient},
		"plain http": {Xa: &ckNenTang{}, Client: http.DefaultClient, Limiter: lim, BaseURL: "http://minio/p/"},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: RegisterPublicBasemap did not refuse", name)
				}
			}()
			RegisterPublicBasemap(http.NewServeMux(), d)
		}()
	}
}
