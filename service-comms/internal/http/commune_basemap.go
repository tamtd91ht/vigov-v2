package http

// THE SELF-HOSTED BASEMAP, SERVED PUBLICLY TO THE ZALO MINI APP — ADR 0072 §"Sửa đổi 11/10/2026 (Mini
// App)" M1: the commune's "Gửi phản ánh" map draws on the SAME PMTiles archive, glyphs and sprites as
// web-admin's `/phan-anh` and `/ban-do`, from ViGov's own MinIO, with HTTP Range.
//
//	GET|HEAD /api/v1/commune-basemap/{file...}?host=   Public — one file of the closed set below
//
// THE NAME (ADR 0072 §Còn mở (Mini App) #2 left it to the builder): `commune-basemap`, the public noun
// shape of this surface (`commune-news`, `commune-map-frame`). The bytes are platform-wide, but the
// request is made FOR a commune — `?host=` scopes the rate limit (`t:<tenant_id>:…`) and refuses
// domains no active commune holds.
//
// WHY comms AND NOT web-admin's `/basemap/*`: the Mini App has no commune domain (CLAUDE.md, Stack), so
// web-admin's same-origin route is not same-origin for it; and the Mini App is already allowed to call
// comms' API host on Zalo's console (M1 "Vì sao service-comms").
//
// A MIRROR OF web-admin's PROXY (web-admin/src/lib/may-chu/basemap.ts, lib/basemap/assets.ts) — the
// same closed list of files, the same forwarded headers, the same passed statuses, the same idle
// timeout, the same "no fallback host". Two readers of one archive that disagree on which files exist
// draw two different maps; a change to one list is a change to both.
//
// WHAT THIS ROUTE DELIBERATELY DOES NOT DO:
//   - forward anything the client sent except the range and conditional headers — no Origin, no
//     cookie, no Authorization, no Host reaches storage;
//   - buffer: the upstream body is copied to the client as it arrives (pod limit 384Mi, archive GBs);
//   - log what was asked: a byte range of the archive is a viewport, i.e. roughly WHERE THE CITIZEN IS
//     (ADR 0072 M5, rule 3). Logs carry the commune and a status, never a path, a Range or a host IP;
//   - fall back to any other host when unconfigured (M1 "Thiếu tệp nền", §S1): 503 and one sentence.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// PublicBasemapPath is the public subtree — exported so cmd/server routes the SAME prefix to the public
// chain; the route spells it as a literal for tools/apidoc, and a test asserts the two agree.
const PublicBasemapPath = "/api/v1/commune-basemap/"

// BasemapIdleTimeout is the upstream INACTIVITY bound: no byte for this long and the read is abandoned
// — bytes moving means alive, so a large range is never cut for being slow. web-admin's
// BASEMAP_IDLE_TIMEOUT_MS, the same 30 s.
const BasemapIdleTimeout = 30 * time.Second

// basemapMissingSentence is web-admin's BASEMAP_MISSING_SENTENCE, word for word: one fact, one sentence
// on every surface that draws the basemap.
const basemapMissingSentence = "Chưa cấu hình bản đồ nền."

const (
	basemapNotFoundSentence    = "Không tìm thấy tệp bản đồ nền."
	basemapUnreachableSentence = "Không tải được bản đồ nền. Vui lòng thử lại."
)

// The closed set of files (web-admin/src/lib/basemap/assets.ts — keep the two in step).
const (
	// basemapArchive is the base tile archive: mainland Vietnam + near-shore islands. A FIXED name: a
	// refresh uploads under a NEW dated prefix and changes BASEMAP_URL, so an archive being read is never
	// overwritten in place.
	basemapArchive = "vn-mainland.pmtiles"
	// basemapPlacesArchive is the second archive beside it under the same BASEMAP_URL (owner, 12/10/2026):
	// Overture Maps places (CDLA-Permissive-2.0), vector layer `places`, z13–15, drawn as part of the
	// basemap. Same fixed-name and refresh rule as basemapArchive.
	basemapPlacesArchive = "vn-places.pmtiles"
	basemapSprite        = "light"
)

// basemapArchives is the EXACT-name set of tile archives: looked up by the whole segment, never by
// prefix or suffix — a pattern would serve any `*.pmtiles` someone uploads under the prefix.
var basemapArchives = map[string]bool{basemapArchive: true, basemapPlacesArchive: true}

// basemapFonts are the font stacks the Protomaps `light` style names in `text-font`.
var basemapFonts = map[string]bool{"Noto Sans Regular": true, "Noto Sans Medium": true, "Noto Sans Italic": true}

var basemapSpriteFiles = map[string]bool{
	basemapSprite + ".json": true, basemapSprite + ".png": true,
	basemapSprite + "@2x.json": true, basemapSprite + "@2x.png": true,
}

// A glyph range is 256 code points, `<start>-<start+255>.pbf`, up to U+FFFF.
var basemapGlyphRange = regexp.MustCompile(`^(\d{1,5})-(\d{1,5})\.pbf$`)

// Cache policy. The archive is `no-cache` (revalidate every time; the ETag still saves the body): a
// stale cached range of the PREVIOUS archive mixed with ranges of the new one is a corrupted tile.
// Glyphs and sprites are immutable per refresh and tiny. `public` (web-admin says `private` because its
// route sits behind the staff session): these bytes are derived from public OpenStreetMap and Overture
// Maps data, the same for everyone.
const (
	basemapArchiveCache = "public, no-cache"
	basemapStaticCache  = "public, max-age=86400"
)

// basemapAsset is one allowed file: its object key relative to BASEMAP_URL (each segment escaped) and
// the headers WE set — never storage's: an object uploaded as text/html must not render as a page.
type basemapAsset struct {
	key          string
	contentType  string
	cacheControl string
}

// basemapAssetFor maps the DECODED path segments after the route to an allowed file, or false. Every
// check is an exact comparison against the closed set and nothing is normalised first — a traversal
// attempt (`..`, an encoded `/`, a second dot) is simply not on the list.
func basemapAssetFor(segs []string) (basemapAsset, bool) {
	switch {
	case len(segs) == 1 && basemapArchives[segs[0]]:
		return basemapAsset{key: segs[0], contentType: "application/octet-stream", cacheControl: basemapArchiveCache}, true
	case len(segs) == 3 && segs[0] == "fonts" && basemapFonts[segs[1]]:
		m := basemapGlyphRange.FindStringSubmatch(segs[2])
		if m == nil {
			return basemapAsset{}, false
		}
		start, _ := strconv.Atoi(m[1]) // \d{1,5}: always parses
		end, _ := strconv.Atoi(m[2])
		if start%256 != 0 || end != start+255 || end > 65535 {
			return basemapAsset{}, false
		}
		return basemapAsset{key: "fonts/" + url.PathEscape(segs[1]) + "/" + strconv.Itoa(start) + "-" + strconv.Itoa(end) + ".pbf",
			contentType: "application/x-protobuf", cacheControl: basemapStaticCache}, true
	case len(segs) == 2 && segs[0] == "sprites" && basemapSpriteFiles[segs[1]]:
		ct := "application/json"
		if strings.HasSuffix(segs[1], ".png") {
			ct = "image/png"
		}
		return basemapAsset{key: "sprites/" + url.PathEscape(segs[1]), contentType: ct, cacheControl: basemapStaticCache}, true
	}
	return basemapAsset{}, false
}

// basemapSegments is the DECODED segments after PublicBasemapPath, read from the ESCAPED path so `%2F`
// stays inside one segment (and then matches nothing). false on a malformed escape.
func basemapSegments(r *http.Request) ([]string, bool) {
	rest, ok := strings.CutPrefix(r.URL.EscapedPath(), PublicBasemapPath)
	if !ok {
		return nil, false
	}
	raw := strings.Split(rest, "/")
	out := make([]string, len(raw))
	for i, s := range raw {
		d, err := url.PathUnescape(s)
		if err != nil {
			return nil, false
		}
		out[i] = d
	}
	return out, true
}

// Request headers forwarded upstream — an ALLOW-list; anything else is dropped.
var basemapForwardedRequest = []string{"Range", "If-Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since"}

// Response headers passed back. Content-Type and Cache-Control are ours (basemapAsset).
var basemapPassedResponse = []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified", "Content-Encoding"}

// Statuses passed through as they are. 416 carries `Content-Range: bytes */<size>`, which PMTiles reads.
var basemapPassedStatus = map[int]bool{
	http.StatusOK: true, http.StatusPartialContent: true, http.StatusNotModified: true,
	http.StatusPreconditionFailed: true, http.StatusRequestedRangeNotSatisfiable: true,
}

// NewBasemapClient is the upstream client main() wires: TLS verified with the system roots (rule 13
// invariant 1 — a private MinIO CA is added to the image's trust store, verification is never switched
// off), no proxy from the environment (the bytes go to ViGov's MinIO and nowhere else), NO transparent
// gzip (a byte-exact archive read must see the bytes storage holds — DisableCompression plus
// `Accept-Encoding: identity`), bounded connect and header waits. The body's inactivity bound is
// BasemapIdleTimeout, applied per read by the handler.
func NewBasemapClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: BasemapIdleTimeout,
			DisableCompression:    true,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
		},
		// Storage never redirects a GetObject; following one would be a host this route did not choose.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// PublicBasemapDeps is everything the public basemap route touches — nothing else is reachable from it.
type PublicBasemapDeps struct {
	// Xa is the platform lookup the public news routes use: an outage is a 503, never "no basemap".
	Xa TraXaTheoHost
	// BaseURL is config.BasemapURL(): "" = not configured (503, never a fallback host). Validated by
	// core/config (https, no credentials, no query, ending in "/").
	BaseURL string
	// Client reaches storage. NewBasemapClient in production; a test server's client in tests.
	Client *http.Client
	// Limiter is ratelimit.PublicBasemapRead — its OWN instance and budget (owner, ADR 0072 M1: 6000 per
	// minute per (host, client network)), so map drags never spend the news or alert budget.
	Limiter *ratelimit.Limiter
	// IdleTimeout overrides BasemapIdleTimeout (tests). Zero = BasemapIdleTimeout.
	IdleTimeout time.Duration
	Log         *slog.Logger
}

// RegisterPublicBasemap mounts the public basemap route onto the PUBLIC mux.
func RegisterPublicBasemap(mux *http.ServeMux, d PublicBasemapDeps) {
	switch {
	case d.Xa == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến nền bản đồ công khai sẽ panic khi có người gọi")
	case d.Client == nil:
		panic("comms/http: thiếu client tới kho tệp nền bản đồ")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất tuyến nền bản đồ công khai (ratelimit.PublicBasemapRead)")
	}
	var base *url.URL
	if d.BaseURL != "" {
		u, err := url.Parse(d.BaseURL)
		if err != nil || u.Scheme != "https" {
			// core/config already refused this at startup; reaching here is a wiring bug, not an input.
			panic("comms/http: BASEMAP_URL không hợp lệ tới được tuyến — core/config phải đã từ chối")
		}
		base = u
	}
	idle := d.IdleTimeout
	if idle <= 0 {
		idle = BasemapIdleTimeout
	}
	// THE PUBLIC NEWS HANDLER's host resolution (xaTheoHost): host check, platform lookup, `t:<tenant_id>`
	// limiter key, 429, 503 — the same path every public route takes, with THIS route's limiter.
	p := &publicBasemap{h: newHandlerCongKhai(DepsCongKhai{Xa: d.Xa, Limiter: d.Limiter, Log: d.Log}),
		base: base, client: d.Client, idle: idle}

	// PUBLIC (owner, 11/10/2026 — ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M1 opens this route; rule 13's
	// "new unauthenticated route" stop condition was decided there).
	//
	// RATE LIMIT — ratelimit.PublicBasemapRead, its own instance: 6000 per minute per (host, client
	// network), counted AFTER the commune resolves so the key is `t:<tenant_id>:…`. FAILS CLOSED (the ADR
	// decided the number, not the outage behaviour): Redis down = 503. Over it: 429 + Retry-After.
	//
	// GET ALSO ANSWERS HEAD (Go's ServeMux). NO idem.* DECLARATION: nothing changes state. NO AUDIT ENTRY:
	// nothing is written, and the bytes are public map data, not personal data (rule 6 invariant 7).
	//
	// @summary  Nền bản đồ tự host (PMTiles, glyph, sprite) cho bản đồ màn Gửi phản ánh của Zalo Mini App — đọc theo khoảng byte (Range, If-Range, If-Match, If-None-Match); chỉ tệp trong danh sách đóng: vn-mainland.pmtiles, vn-places.pmtiles, fonts/<Noto Sans Regular|Medium|Italic>/<n>-<n+255>.pbf, sprites/light[@2x].json|png
	// @consumer citizen-app
	//
	// 200/206/304/412/416 are storage's answer, passed through with Content-Range, Accept-Ranges, ETag,
	// Content-Length. 400 is `host` missing, repeated or malformed. 404 a file not on the list or not
	// uploaded. 503 `basemap_not_configured` for BASEMAP_URL unset AND for a domain no active commune
	// holds (identical bytes), `platform_unavailable` the registry unreachable, `rate_limit_unavailable`
	// Redis down (the limiter fails closed). 502/504 storage unreachable / silent. 429 the rate limit.
	// The 2xx/3xx/412/416 bodies are the file's bytes or empty — not JSON, hence `-`.
	//
	// @reply    200 -
	// @reply    206 -
	// @reply    304 -
	// @reply    400 httpx.Error invalid_host
	// @reply    404 httpx.Error not_found
	// @reply    412 -
	// @reply    416 -
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    502 httpx.Error basemap_unavailable
	// @reply    503 httpx.Error basemap_not_configured platform_unavailable rate_limit_unavailable
	// @reply    504 httpx.Error basemap_unavailable
	mux.Handle("GET /api/v1/commune-basemap/{file...}",
		authz.Public("nền bản đồ tự host cho bản đồ màn Gửi phản ánh trên Zalo Mini App (chủ dự án mở tuyến này 11/10/2026, ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M1): người dân không có tài khoản; chỉ phát lại một danh sách đóng tệp nền PMTiles, glyph và sprite dựng từ dữ liệu công khai OpenStreetMap và Overture Maps trong MinIO của ViGov — không dữ liệu của xã, không dữ liệu cá nhân; chỉ cho tên miền của một xã đang hoạt động; giới hạn 6.000 lần/phút theo tên miền và mạng của máy gọi, trần riêng")(
			http.HandlerFunc(p.PublicBasemap)))
}

type publicBasemap struct {
	h      *HandlerCongKhai
	base   *url.URL
	client *http.Client
	idle   time.Duration
}

// PublicBasemap — GET|HEAD /api/v1/commune-basemap/{file...}?host=
func (p *publicBasemap) PublicBasemap(w http.ResponseWriter, r *http.Request) {
	hosts := r.URL.Query()["host"]
	if len(hosts) != 1 || !domain.HopLeTenMienXa(hosts[0]) {
		viet400Host(w)
		return
	}
	xa, found, ok := p.h.xaTheoHost(w, r, hosts[0], "nền bản đồ")
	if !ok {
		return
	}
	// A domain no active commune holds and an unset BASEMAP_URL are ONE answer, byte for byte — neither
	// the status nor the body tells a probe which domains are communes. Checked before the file, like
	// web-admin: unconfigured is one answer for every path, not a scatter of 404s.
	if !found || p.base == nil {
		basemapError(w, http.StatusServiceUnavailable, "basemap_not_configured", basemapMissingSentence)
		return
	}
	ctx := tenant.Into(r.Context(), xa.ID)

	segs, ok := basemapSegments(r)
	var asset basemapAsset
	if ok {
		asset, ok = basemapAssetFor(segs)
	}
	if !ok {
		basemapError(w, http.StatusNotFound, "not_found", basemapNotFoundSentence)
		return
	}
	p.proxy(ctx, w, r, asset)
}

func (p *publicBasemap) proxy(ctx context.Context, w http.ResponseWriter, r *http.Request, asset basemapAsset) {
	commune := string(tenant.MustFrom(ctx))
	target, err := p.base.Parse(asset.key)
	if err != nil {
		p.h.loi500(ctx, w, "nền bản đồ", err)
		return
	}
	// The upstream request lives as long as the client's and no longer — a Mini App that pans away
	// cancels it — and is also cut when no byte moves for p.idle.
	upCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := time.AfterFunc(p.idle, cancel)
	defer idle.Stop()

	method := http.MethodGet
	if r.Method == http.MethodHead {
		method = http.MethodHead
	}
	req, err := http.NewRequestWithContext(upCtx, method, target.String(), nil)
	if err != nil {
		p.h.loi500(ctx, w, "nền bản đồ", err)
		return
	}
	req.Header.Set("Accept-Encoding", "identity")
	for _, name := range basemapForwardedRequest {
		if v := r.Header.Values(name); len(v) > 0 {
			req.Header[name] = append([]string(nil), v...)
		}
	}

	res, err := p.client.Do(req)
	if err != nil {
		if r.Context().Err() != nil {
			return // the client went away; nobody is left to answer
		}
		// The error text carries the internal storage address: logged as an outcome only, never sent.
		outcome := "unreachable"
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			outcome, status = "timeout", http.StatusGatewayTimeout
		}
		p.h.d.Log.WarnContext(ctx, "nền bản đồ: không đọc được kho tệp nền", "xa", commune, "outcome", outcome)
		basemapError(w, status, "basemap_unavailable", basemapUnreachableSentence)
		return
	}
	defer res.Body.Close()
	idle.Reset(p.idle)

	if !basemapPassedStatus[res.StatusCode] {
		// Status only: never the path or the Range (a range names the area being viewed — rule 3).
		p.h.d.Log.WarnContext(ctx, "nền bản đồ: kho tệp nền trả mã ngoài danh sách",
			"xa", commune, "upstream_status", res.StatusCode)
		// 404 from storage = the file was never uploaded under this prefix — the runbook's first check.
		if res.StatusCode == http.StatusNotFound {
			basemapError(w, http.StatusNotFound, "not_found", basemapNotFoundSentence)
		} else {
			basemapError(w, http.StatusBadGateway, "basemap_unavailable", basemapUnreachableSentence)
		}
		return
	}

	// 412/416 carry storage's XML error body, which names the bucket and the key — internal layout the
	// client has no use for. PMTiles reads only the status and Content-Range, so the body is dropped.
	noBody := method == http.MethodHead || res.StatusCode == http.StatusNotModified ||
		res.StatusCode == http.StatusPreconditionFailed || res.StatusCode == http.StatusRequestedRangeNotSatisfiable

	out := w.Header()
	out.Set("Content-Type", asset.contentType)
	out.Set("Cache-Control", asset.cacheControl)
	out.Set("X-Content-Type-Options", "nosniff")
	withLength := !noBody || method == http.MethodHead
	for _, name := range basemapPassedResponse {
		if name == "Content-Length" && !withLength {
			continue
		}
		if v := res.Header.Values(name); len(v) > 0 {
			out.Set(name, v[0]) // Set canonicalises (ETag -> Etag); a raw map write would hide it from Get
		}
	}
	// net/http may move Content-Length out of the header map into res.ContentLength.
	if withLength && res.ContentLength >= 0 && !hasHeader(out, "Content-Length") {
		out.Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	}
	if basemapArchives[asset.key] && !hasHeader(out, "Accept-Ranges") {
		out.Set("Accept-Ranges", "bytes")
	}
	w.WriteHeader(res.StatusCode)
	if noBody {
		return
	}
	// Streamed, never buffered; each chunk read re-arms the inactivity bound. A copy error after the
	// header is out cannot be answered — the client sees a short body and PMTiles retries.
	_, _ = io.Copy(w, &idleReader{r: res.Body, timer: idle, idle: p.idle})
}

func hasHeader(h http.Header, name string) bool { return len(h.Values(name)) > 0 }

// idleReader re-arms the inactivity timer on every read that returns bytes.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	idle  time.Duration
}

func (ir *idleReader) Read(b []byte) (int, error) {
	n, err := ir.r.Read(b)
	if n > 0 {
		ir.timer.Reset(ir.idle)
	}
	return n, err
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// basemapError is the one error shape of this route: the httpx error body, never cached.
func basemapError(w http.ResponseWriter, status int, code, sentence string) {
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteError(w, status, code, sentence, "")
}
