package http

// THE MAP FRAME — the fixed view of ONE commune's economic map (ADR 0072 §"Sửa đổi 04/10/2026", H3, and
// §"Sửa đổi 04/10/2026 (lần 2)" K2–K6; migrations/0016_map_frame.sql, 0017).
//
//	GET  /api/v1/map-frame         asset.read     the EFFECTIVE frame — the commune's own, else the
//	                               OR             platform default — and its bounds, or {configured:false}
//	                               feedback.read
//	PUT  /api/v1/map-frame         admin.lookup   set or move the commune's own frame (legal notice required)
//	POST /api/v1/map-frame/reset   admin.lookup   "Về mặc định": stop applying the own frame (notice required)
//
// `map-frame` IS A SINGLETON RESOURCE OF THE COMMUNE, like `mail-settings`: one per commune, no id in
// the path. The noun follows the table (`map_frame`, 0016) and its siblings (`map-assets`).
//
// "VỀ MẶC ĐỊNH" IS A POST TO A SUB-RESOURCE, NOT A DELETE. DELETE says the resource goes away; here the
// row is KEPT with its last centre and radius (0017, `is_enabled = false`; 0016's trigger refuses a
// DELETE) and the map keeps showing a frame — the platform's. A verb the semantics contradict is a verb
// some proxy, cache or client library will one day act on as written.
//
// THE KEYS EXIST AND NONE WAS INVENTED (rule 5, invariant 3c): `asset.read` is the key of every read of
// the economic map (map_assets.go), seeded at service-identity/migrations/0001_init.sql:287; `feedback.read`
// is the key of every staff read of petitions, seeded at :299; `admin.lookup` is the key H3 and K4 name
// ("Ai đặt", "Ai đổi"), seeded at :281.
//
// NOT SET IS 200 {"configured": false, …hints}, NOT 404 — the repository's convention for a singleton not
// yet saved (GET /api/v1/mail-settings). "No frame" is a designed state of the page (H3 "Chưa đặt").
//
// THE BOUNDS ARE COMPUTED HERE (domain.MapFrame.Bounds), not in the browser — for the commune frame and
// the platform default alike: web-admin passes `bounds` to MapLibre's `maxBounds` as is.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// MapFrameReader is the READ half — no transaction. *app.MapFrames satisfies it.
type MapFrameReader interface {
	Get(ctx context.Context) (app.MapFrameView, error)
}

// MapFrameWriter is the WRITE half: each method opens a TRANSACTION and writes its audit entry inside it
// (rule 6, invariant 3). *app.MapFrames satisfies it.
type MapFrameWriter interface {
	Save(ctx context.Context, in app.MapFrameInput, actor audit.Actor) (app.MapFrameView, error)
	Reset(ctx context.Context, noticeVersion string, actor audit.Actor) (app.MapFrameView, error)
}

// MapFrameDeps is everything the three routes touch.
type MapFrameDeps struct {
	Checker authz.Checker
	Reader  MapFrameReader
	Writer  MapFrameWriter
	Log     *slog.Logger
}

type mapFrameHandler struct {
	d MapFrameDeps
}

// mapFrameDefaultOut is the platform's default frame for the commune and its bounds.
type mapFrameDefaultOut struct {
	CenterLat float64 `json:"center_lat"`
	CenterLng float64 `json:"center_lng"`
	RadiusKm  float64 `json:"radius_km"`
	// Bounds is [minLng, minLat, maxLng, maxLat], same function and order as the frame's own `bounds`.
	Bounds []float64 `json:"bounds"`
}

// mapFrameOut is the commune's EFFECTIVE frame. When `configured` is false NO FRAME FIELD IS PRESENT —
// only the hints below: the page draws no map, only the guidance sentence and the form for a holder of
// `admin.lookup` (H3), and the form needs the hints.
type mapFrameOut struct {
	Configured bool `json:"configured"`
	// Source is "commune" (the commune's own frame) or "default" (the platform default — the commune has
	// none, or chose "Về mặc định"). Absent when not configured.
	Source    string   `json:"source,omitempty"`
	CenterLat *float64 `json:"center_lat,omitempty"`
	CenterLng *float64 `json:"center_lng,omitempty"`
	RadiusKm  *float64 `json:"radius_km,omitempty"`
	// Bounds is [minLng, minLat, maxLng, maxLat] — [west, south, east, north], MapLibre's
	// LngLatBoundsLike, LONGITUDE FIRST. Rounded outward to 6 decimals; apply as `maxBounds` unchanged.
	Bounds []float64 `json:"bounds,omitempty"`
	// UpdatedAt / UpdatedBy — when and by whom (`CB-…`) the commune last set its own frame. Only with
	// source "commune".
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy string     `json:"updated_by,omitempty"`
	// Default is the platform default — what "Về mặc định" returns to. Always present with source
	// "default". With source "commune" it is BEST-EFFORT: absent when there is none OR the platform could
	// not be read (an outage must not take a commune's own map away, so that path never answers 503).
	Default *mapFrameDefaultOut `json:"default,omitempty"`

	// THE FORM'S HINTS (ADR 0072 K2/K6) — always present. recommended_radius_km and usual_radius_km are
	// proposals the owner may adjust: the server never refuses a radius for being outside them, only
	// above max_radius_km (or not above 0). notice_version is the version the next PUT / reset must echo.
	RecommendedRadiusKm float64    `json:"recommended_radius_km"`
	UsualRadiusKm       [2]float64 `json:"usual_radius_km"`
	MaxRadiusKm         float64    `json:"max_radius_km"`
	NoticeVersion       string     `json:"notice_version"`
}

// mapFrameIn is the body of PUT — the whole frame. Pointers so a missing field is a 400 and never read
// as 0 (a centre in the Gulf of Guinea, a radius of nothing). Centre: 6 decimals kept, radius: 1 —
// a longer value is ROUNDED to that, as the database would, and the rounded value is validated.
//
// NoticeVersion is the version of ADR 0072 K6's legal notice the official acknowledged — it must equal
// the current one (`notice_version` of GET), else 422 notice_not_acknowledged.
type mapFrameIn struct {
	CenterLat     *float64 `json:"center_lat"`
	CenterLng     *float64 `json:"center_lng"`
	RadiusKm      *float64 `json:"radius_km"`
	NoticeVersion *string  `json:"notice_version"`
}

// mapFrameResetIn is the body of POST /reset: only the acknowledgement of the current notice.
type mapFrameResetIn struct {
	NoticeVersion *string `json:"notice_version"`
}

func mapFrameToOut(v app.MapFrameView) mapFrameOut {
	out := mapFrameOut{
		RecommendedRadiusKm: domain.MapFrameRecommendedRadiusKm,
		UsualRadiusKm:       [2]float64{domain.MapFrameUsualMinKm, domain.MapFrameUsualMaxKm},
		MaxRadiusKm:         domain.MapFrameMaxRadiusKm,
		NoticeVersion:       domain.MapFrameNoticeVersion,
	}
	if v.Default != nil {
		d := v.Default
		out.Default = &mapFrameDefaultOut{
			CenterLat: d.Frame.CenterLat, CenterLng: d.Frame.CenterLng, RadiusKm: d.Frame.RadiusKm,
			Bounds: []float64{d.Bounds.MinLng, d.Bounds.MinLat, d.Bounds.MaxLng, d.Bounds.MaxLat},
		}
	}
	if !v.Configured {
		return out
	}
	f, b := v.Frame, v.Bounds
	out.Configured, out.Source = true, v.Source
	out.CenterLat, out.CenterLng, out.RadiusKm = &f.CenterLat, &f.CenterLng, &f.RadiusKm
	out.Bounds = []float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat}
	if v.Source == domain.MapFrameSourceCommune {
		at := f.UpdatedAt
		out.UpdatedAt, out.UpdatedBy = &at, f.UpdatedBy
	}
	return out
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// RegisterMapFrame mounts the three routes. Refuses incomplete wiring at construction, like Register.
func RegisterMapFrame(mux *http.ServeMux, d MapFrameDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến khung bản đồ sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu bộ đọc khung bản đồ — GET /api/v1/map-frame sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi khung bản đồ — PUT/POST /api/v1/map-frame sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &mapFrameHandler{d: d}

	// THE EFFECTIVE FRAME, READ BY THE MAP PAGE ON EVERY OPEN. `asset.read`, the key of every other read
	// of the map: whoever may see the map must learn its frame, or that there is none.
	//
	// OR `feedback.read` (owner decision, ADR 0072 §"Trả lời 09/10/2026" under §"Sửa đổi 09/10/2026"): the petition heat
	// map and the petition mini-map are centred and bounded by this same frame, and a petition officer
	// typically holds `feedback.read` without `asset.read`. Requiring `asset.read` alone answers that
	// officer 403 and the petition maps open on no frame; granting `asset.read` instead would open the
	// whole economic map (every asset) to them, which nobody decided. The reply carries no asset and no
	// personal data — a centre, a radius, a box and who last set it (`CB-…`) — so the second key widens
	// nothing beyond the frame itself. READ ONLY: the PUT and the reset below keep `admin.lookup` alone.
	//
	// NO idem.* DECLARATION: a GET changes no state. No audit entry: no personal data, and the read is
	// inside the commune the request arrived in.
	//
	//	503 map_frame_default_unavailable  the commune has no own frame and the platform could not be
	//	                                   asked for its default — never answered with a guessed frame
	//
	// @summary  Khung bản đồ kinh tế số đang áp cho xã — của xã hoặc mặc định của nền tảng (source), tâm, bán kính, khung giới hạn [kinh độ nhỏ, vĩ độ nhỏ, kinh độ lớn, vĩ độ lớn] để đặt maxBounds, khung mặc định (default) khi có, và các gợi ý cho biểu mẫu; không có khung nào thì configured=false
	// @screen   10-ban-do-kinh-te-so §4
	// @reply    200 mapFrameOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error map_frame_default_unavailable
	mux.Handle("GET /api/v1/map-frame",
		authz.RequireAnyPermission(d.Checker, "asset.read", "feedback.read")(
			http.HandlerFunc(h.GetMapFrame)))

	// SET OR MOVE THE COMMUNE'S OWN FRAME. PUT AND NOT PATCH: the form is saved whole and the values only
	// make sense together. Saving also switches a frame left by "Về mặc định" back on.
	//
	// `admin.lookup` — H3 "Ai đặt" / K4 "Ai đổi". Holding `asset.update` is NOT enough: editing what is on
	// the map is not deciding how much of the country the map may show.
	//
	//	400 invalid_request          not JSON, or a frame field missing
	//	422 notice_not_acknowledged  `notice_version` missing, empty or not the current one (K4/K6)
	//	422 center_outside_mainland  the (rounded) centre is outside the mainland box of 0016's CHECK
	//	422 radius_out_of_range      the (rounded) radius is not > 0 and ≤ 50 km (0017's CHECK, K2)
	//
	// idem.KhongCan: the result is the state the body names, and app.MapFrames.Save writes and audits
	// NOTHING when the rounded values equal the stored, enabled ones — so the same request twice leaves
	// one row and one entry. Were that comparison removed, this declaration would become a lie.
	//
	// @summary  Đặt hoặc đổi khung bản đồ kinh tế số riêng của xã — phải xác nhận lưu ý pháp lý phiên bản hiện hành (notice_version); tâm trong khung đất liền Việt Nam, bán kính lớn hơn 0 và không quá 50 km; có ghi vết
	// @screen   10-ban-do-kinh-te-so §4
	// @request  mapFrameIn
	// @reply    200 mapFrameOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error notice_not_acknowledged center_outside_mainland radius_out_of_range
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/map-frame",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lưu là ghi đè khung bằng đúng giá trị trong thân; giá trị (đã làm tròn) trùng giá trị đang lưu và đang bật thì không ghi và không có vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.PutMapFrame))))

	// "VỀ MẶC ĐỊNH" (K4): the commune stops applying its own frame; the platform default applies, or —
	// with none — "Chưa đặt". The row and its last values are kept (rule 7). Same key as the PUT: it
	// changes the same frame. The reply is the effective frame afterwards, exactly as GET reads it.
	//
	//	400 invalid_request                not JSON
	//	422 notice_not_acknowledged        `notice_version` missing, empty or not the current one
	//	503 map_frame_default_unavailable  the reset COMMITTED but the platform default could not be read
	//	                                   for the reply; retrying is a no-op that returns the frame
	//
	// idem.KhongCan: the result is a state, "own frame off". App.MapFrames.Reset writes and audits
	// NOTHING when there is no row or it is already off, so a second request leaves one entry.
	//
	// @summary  Về khung mặc định của nền tảng — xã thôi dùng khung riêng (giữ lại giá trị cũ, không xoá); phải xác nhận lưu ý pháp lý phiên bản hiện hành; có ghi vết; trả về khung đang áp sau khi đổi
	// @screen   10-ban-do-kinh-te-so §4
	// @request  mapFrameResetIn
	// @reply    200 mapFrameOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error notice_not_acknowledged
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error map_frame_default_unavailable
	mux.Handle("POST /api/v1/map-frame/reset",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("về mặc định là đặt trạng thái 'không dùng khung riêng'; xã chưa có khung riêng hoặc đã tắt thì không ghi và không có vết, nên lần gửi thứ hai để lại đúng một vết")(
				http.HandlerFunc(h.ResetMapFrame))))
}

// GetMapFrame — GET /api/v1/map-frame
func (h *mapFrameHandler) GetMapFrame(w http.ResponseWriter, r *http.Request) {
	// Scoped in the use case: MapFrameStore.Get reads through db.For(ctx).Query.
	v, err := h.d.Reader.Get(r.Context())
	if err != nil {
		h.fail(r.Context(), w, "đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, mapFrameToOut(v))
}

// PutMapFrame — PUT /api/v1/map-frame
func (h *mapFrameHandler) PutMapFrame(w http.ResponseWriter, r *http.Request) {
	var in mapFrameIn
	if !docThan(w, r, &in) {
		return
	}
	if in.CenterLat == nil || in.CenterLng == nil || in.RadiusKm == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần đủ ba giá trị: vĩ độ tâm (`center_lat`), kinh độ tâm (`center_lng`) và bán kính (`radius_km`, đơn vị km).", "")
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	// Scoped in the use case: app.MapFrames opens db.For(ctx).Tx, and writes the audit.Write entry —
	// with the notice version — inside that same transaction (rule 6, invariant 3).
	v, err := h.d.Writer.Save(r.Context(), app.MapFrameInput{
		CenterLat: *in.CenterLat, CenterLng: *in.CenterLng, RadiusKm: *in.RadiusKm,
		NoticeVersion: strOrEmpty(in.NoticeVersion),
	}, actor)
	if err != nil {
		h.fail(r.Context(), w, "lưu", err)
		return
	}
	vietJSON(w, http.StatusOK, mapFrameToOut(v))
}

// ResetMapFrame — POST /api/v1/map-frame/reset
func (h *mapFrameHandler) ResetMapFrame(w http.ResponseWriter, r *http.Request) {
	var in mapFrameResetIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	// Scoped in the use case: app.MapFrames opens db.For(ctx).Tx, and writes the audit.Write entry —
	// with the notice version — inside that same transaction (rule 6, invariant 3).
	v, err := h.d.Writer.Reset(r.Context(), strOrEmpty(in.NoticeVersion), actor)
	if err != nil {
		h.fail(r.Context(), w, "về mặc định", err)
		return
	}
	vietJSON(w, http.StatusOK, mapFrameToOut(v))
}

// actor is the audit actor, or a 500 already written: a write whose trail cannot name who made it is
// not permitted (rule 6, invariant 8 — see nguoiThucHien).
func (h *mapFrameHandler) actor(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi khung bản đồ chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return actor, ok
}

// fail maps a use-case error to its reply. The bounds and the version are read from domain, never
// retyped: a second copy of a number drifts.
func (h *mapFrameHandler) fail(ctx context.Context, w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrMapFrameNoticeNotAcknowledged):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "notice_not_acknowledged",
			fmt.Sprintf("Cần đọc và xác nhận lưu ý pháp lý (phiên bản %s) trước khi thay đổi khung bản đồ của xã.",
				domain.MapFrameNoticeVersion), "")
	case errors.Is(err, domain.ErrMapFrameCenterOutsideMainland):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "center_outside_mainland",
			fmt.Sprintf("Tâm khung bản đồ phải nằm trong khung đất liền Việt Nam: vĩ độ từ %g đến %g, kinh độ từ %g đến %g.",
				domain.MapFrameMinLat, domain.MapFrameMaxLat, domain.MapFrameMinLng, domain.MapFrameMaxLng), "")
	case errors.Is(err, domain.ErrMapFrameRadiusOutOfRange):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "radius_out_of_range",
			fmt.Sprintf("Bán kính khung bản đồ phải lớn hơn %g và không quá %g km.",
				domain.MapFrameMinRadiusKm, domain.MapFrameMaxRadiusKm), "")
	case errors.Is(err, app.ErrMapFrameDefaultUnavailable):
		// Warn, not Error: comms is healthy; the platform it asks is not. The detail stays in the log.
		h.d.Log.Warn("khung bản đồ: "+op+" — không đọc được khung mặc định từ nền tảng",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "map_frame_default_unavailable",
			"Tạm thời không đọc được khung bản đồ mặc định. Vui lòng thử lại sau ít phút.", "")
	default:
		// The wrapped error never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("khung bản đồ: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
