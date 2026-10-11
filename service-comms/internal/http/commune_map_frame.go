package http

// THE COMMUNE'S MAP FRAME, READ PUBLICLY BY HOST — for the Zalo Mini App's "Gửi phản ánh" map (ADR 0072
// §"Sửa đổi 11/10/2026 (Mini App)" M2).
//
//	GET /api/v1/commune-map-frame?host=   Public — the commune's EFFECTIVE frame: its bounds, centre and
//	                                      radius, or {"configured":false}
//
// `commune-map-frame` follows the public nouns of this surface (`commune-news`, `commune-disaster-alerts`)
// over the staff singleton `map-frame` — the same resource, a different audience and a different reply.
//
// WHY PUBLIC (owner, 11/10/2026, M2): a frame is four coordinates around the commune's territory —
// metadata of the commune (K1), no personal data. The resident opening the Mini App has no account.
//
// WHAT IS LEFT OUT, ON PURPOSE (M2 "Không đổi"): `source` (own frame vs platform default),
// `default`, `updated_at` / `updated_by` (a staff business code) and the form's hints. Those serve the
// staff page and its form; a resident's map needs only where it may go.
//
// THE STAFF ROUTE IS UNCHANGED: GET /api/v1/map-frame keeps `asset.read` OR `feedback.read`
// (map_frame.go). Both read the same use case, app.MapFrames.Get — one effective-frame rule.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// PublicMapFramePath is the public resource — exported so cmd/server routes the SAME path to the public
// chain; the route spells it as a literal for tools/apidoc, and a test asserts the two agree.
const PublicMapFramePath = "/api/v1/commune-map-frame"

// PublicMapFrameDeps is everything the public frame route touches — nothing else is reachable from it.
type PublicMapFrameDeps struct {
	// Xa is the platform lookup the public news routes use: an outage is a 503, never "no frame".
	Xa TraXaTheoHost
	// Frames is the effective-frame read. *app.MapFrames satisfies it — the staff route's own use case.
	Frames MapFrameReader
	// Limiter is ratelimit.PublicNewsRead — the SAME instance as the news routes (owner, ADR 0072 M2:
	// "dùng chung trần tin công khai"): one read per screen open, not a byte stream like the basemap.
	Limiter *ratelimit.Limiter
	Log     *slog.Logger
}

// publicMapFrameOut is the frame on the public wire. Configured false carries NO other field — for a
// commune with neither its own frame nor a platform default (K3 "Không có cả hai") AND for a domain no
// active commune holds, byte-identical: the Mini App draws no map and still sends the petition with the
// coordinates it took (M2 "Xã không có khung").
type publicMapFrameOut struct {
	Configured bool     `json:"configured"`
	CenterLat  *float64 `json:"center_lat,omitempty"`
	CenterLng  *float64 `json:"center_lng,omitempty"`
	RadiusKm   *float64 `json:"radius_km,omitempty"`
	// Bounds is [minLng, minLat, maxLng, maxLat] — [west, south, east, north], MapLibre's
	// LngLatBoundsLike, LONGITUDE FIRST, the same values the staff route sends. Apply as `maxBounds`.
	Bounds []float64 `json:"bounds,omitempty"`
}

func publicMapFrameToOut(v app.MapFrameView) publicMapFrameOut {
	if !v.Configured {
		return publicMapFrameOut{}
	}
	f, b := v.Frame, v.Bounds
	return publicMapFrameOut{Configured: true, CenterLat: &f.CenterLat, CenterLng: &f.CenterLng, RadiusKm: &f.RadiusKm,
		Bounds: []float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat}}
}

// RegisterPublicMapFrame mounts the public frame route onto the PUBLIC mux.
func RegisterPublicMapFrame(mux *http.ServeMux, d PublicMapFrameDeps) {
	switch {
	case d.Xa == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến khung bản đồ công khai sẽ panic khi có người gọi")
	case d.Frames == nil:
		panic("comms/http: thiếu bộ đọc khung bản đồ — tuyến khung bản đồ công khai sẽ panic khi có người gọi")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất tuyến khung bản đồ công khai (ratelimit.PublicNewsRead)")
	}
	// THE PUBLIC NEWS HANDLER's host resolution (xaTheoHost): host check, platform lookup, `t:<tenant_id>`
	// limiter key, 429, 503 — the same path the news, contact and alert routes take, not a copy.
	p := &publicMapFrame{h: newHandlerCongKhai(DepsCongKhai{Xa: d.Xa, Limiter: d.Limiter, Log: d.Log}), frames: d.Frames}

	// PUBLIC (owner, 11/10/2026 — ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M2 opens this route).
	//
	// RATE LIMIT — ratelimit.PublicNewsRead, the news routes' instance and budget (owner, M2): 120 per
	// minute per (host, client network), fails open. Over it: 429 + Retry-After.
	//
	// NO idem.* DECLARATION: a GET changes no state. NO AUDIT ENTRY: nothing is written, and the frame is
	// commune metadata, not personal data (rule 6, invariant 7 asks for neither).
	//
	// @summary  Khung bản đồ đang áp của xã theo tên miền — khung giới hạn [kinh độ nhỏ, vĩ độ nhỏ, kinh độ lớn, vĩ độ lớn] để đặt maxBounds, tâm và bán kính — cho bản đồ màn Gửi phản ánh của Zalo Mini App; xã không có khung thì configured=false
	// @consumer citizen-app
	//
	// 200 {"configured":false} for a commune with no frame AND for a domain no active commune holds —
	// identical bytes. 400 is `host` missing, repeated or malformed (the platform is not asked). 500 a
	// store failure. 503 `platform_unavailable` the registry unreachable, or `map_frame_default_unavailable`
	// the commune has no own frame and the platform default could not be read. 429 the rate limit.
	//
	// @reply    200 publicMapFrameOut
	// @reply    400 httpx.Error invalid_host
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error platform_unavailable map_frame_default_unavailable
	mux.Handle("GET /api/v1/commune-map-frame",
		authz.Public("khung bản đồ đang áp của xã cho bản đồ màn Gửi phản ánh trên Zalo Mini App (chủ dự án mở tuyến này 11/10/2026, ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M2): người dân không có tài khoản; chỉ trả bốn toạ độ biên, tâm và bán kính của đúng xã mà nền tảng phân giải từ tên miền — siêu dữ liệu xã, không dữ liệu cá nhân, không phần chỉ cán bộ cần; giới hạn 120 lần/phút theo tên miền và mạng của máy gọi, chung trần tin công khai")(
			http.HandlerFunc(p.PublicMapFrame)))
}

type publicMapFrame struct {
	h      *HandlerCongKhai
	frames MapFrameReader
}

// PublicMapFrame — GET /api/v1/commune-map-frame?host=
func (p *publicMapFrame) PublicMapFrame(w http.ResponseWriter, r *http.Request) {
	hosts := r.URL.Query()["host"]
	if len(hosts) != 1 || !domain.HopLeTenMienXa(hosts[0]) {
		viet400Host(w)
		return
	}
	xa, found, ok := p.h.xaTheoHost(w, r, hosts[0], "khung bản đồ của xã")
	if !ok {
		return
	}
	if !found {
		vietJSON(w, http.StatusOK, publicMapFrameOut{})
		return
	}
	ctx := tenant.Into(r.Context(), xa.ID)
	// Scoped in the use case: MapFrameStore.Get reads through db.For(ctx).Query.
	v, err := p.frames.Get(ctx)
	if err != nil {
		p.fail(ctx, w, err)
		return
	}
	vietJSON(w, http.StatusOK, publicMapFrameToOut(v))
}

func (p *publicMapFrame) fail(ctx context.Context, w http.ResponseWriter, err error) {
	if errors.Is(err, app.ErrMapFrameDefaultUnavailable) {
		// Warn, not Error: comms is healthy; the platform it asks is not. Never a guessed frame.
		p.h.d.Log.WarnContext(ctx, "khung bản đồ công khai: không đọc được khung mặc định từ nền tảng",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "map_frame_default_unavailable",
			"Tạm thời không đọc được khung bản đồ mặc định. Vui lòng thử lại sau ít phút.", "")
		return
	}
	p.h.loi500(ctx, w, "khung bản đồ của xã", err)
}
