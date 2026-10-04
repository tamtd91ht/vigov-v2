package http

// THE MAP FRAME — the fixed view of ONE commune's economic map (ADR 0072 §"Sửa đổi 04/10/2026", H3;
// migrations/0016_map_frame.sql).
//
//	GET  /api/v1/map-frame   asset.read     the frame and its bounds, or {configured:false}
//	PUT  /api/v1/map-frame   admin.lookup   set or move it — centre and radius
//
// `map-frame` IS A SINGLETON RESOURCE OF THE COMMUNE, like `mail-settings`: one per commune, no id in
// the path. The noun follows the table (`map_frame`, 0016) and its siblings (`map-assets`).
//
// THE KEYS EXIST AND NONE WAS INVENTED (rule 5, invariant 3c): `asset.read` is the key of every read of
// the economic map (map_assets.go), seeded at service-identity/migrations/0001_init.sql:287; `admin.lookup`
// is the key H3 names ("Ai đặt") and the one every write of the map catalogue declares, seeded at :274.
//
// NOT SET IS 200 {"configured": false}, NOT 404 — the repository's convention for a singleton not yet
// saved (GET /api/v1/mail-settings). The page asks this on every open, and "no frame" is a designed
// state of that page (H3 "Chưa đặt"), not a missing resource: a 404 would read as a broken route in
// every log and every browser console.
//
// THE BOUNDS ARE COMPUTED HERE (domain.MapFrame.Bounds), not in the browser: web-admin passes `bounds`
// to MapLibre's `maxBounds` as is, so the geometry has one copy.

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

// MapFrameWriter is the WRITE half: it opens a TRANSACTION and writes its audit entry inside it (rule 6,
// invariant 3). *app.MapFrames satisfies it.
type MapFrameWriter interface {
	Save(ctx context.Context, in app.MapFrameInput, actor audit.Actor) (app.MapFrameView, error)
}

// MapFrameDeps is everything the two routes touch.
type MapFrameDeps struct {
	Checker authz.Checker
	Reader  MapFrameReader
	Writer  MapFrameWriter
	Log     *slog.Logger
}

type mapFrameHandler struct {
	d MapFrameDeps
}

// mapFrameOut is the commune's frame. When `configured` is false NO OTHER FIELD IS PRESENT: the page
// draws no map, only the guidance sentence and the form for a holder of `admin.lookup` (H3).
type mapFrameOut struct {
	Configured bool     `json:"configured"`
	CenterLat  *float64 `json:"center_lat,omitempty"`
	CenterLng  *float64 `json:"center_lng,omitempty"`
	RadiusKm   *float64 `json:"radius_km,omitempty"`
	// Bounds is [minLng, minLat, maxLng, maxLat] — [west, south, east, north], MapLibre's
	// LngLatBoundsLike, LONGITUDE FIRST. Rounded outward to 6 decimals; apply as `maxBounds` unchanged.
	Bounds    []float64  `json:"bounds,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// UpdatedBy is the business code `CB-…` of who last set the frame.
	UpdatedBy string `json:"updated_by,omitempty"`
}

// mapFrameIn is the body of PUT — the whole frame. Pointers so a missing field is a 400 and never read
// as 0 (a centre in the Gulf of Guinea, a radius of nothing). Centre: 6 decimals kept, radius: 1 —
// a longer value is ROUNDED to that, as the database would, and the rounded value is validated.
type mapFrameIn struct {
	CenterLat *float64 `json:"center_lat"`
	CenterLng *float64 `json:"center_lng"`
	RadiusKm  *float64 `json:"radius_km"`
}

func mapFrameToOut(v app.MapFrameView) mapFrameOut {
	if !v.Configured {
		return mapFrameOut{}
	}
	f, b := v.Frame, v.Bounds
	at := f.UpdatedAt
	return mapFrameOut{
		Configured: true,
		CenterLat:  &f.CenterLat, CenterLng: &f.CenterLng, RadiusKm: &f.RadiusKm,
		Bounds:    []float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat},
		UpdatedAt: &at, UpdatedBy: f.UpdatedBy,
	}
}

// RegisterMapFrame mounts the two routes. Refuses incomplete wiring at construction, like Register.
func RegisterMapFrame(mux *http.ServeMux, d MapFrameDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — hai tuyến khung bản đồ sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu bộ đọc khung bản đồ — GET /api/v1/map-frame sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi khung bản đồ — PUT /api/v1/map-frame sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &mapFrameHandler{d: d}

	// THE FRAME, READ BY THE MAP PAGE ON EVERY OPEN. `asset.read`, the key of every other read of the
	// map: whoever may see the map must learn its frame, or that there is none.
	//
	// NO idem.* DECLARATION: a GET changes no state. No audit entry: no personal data, and the read is
	// inside the commune the request arrived in.
	//
	// @summary  Khung bản đồ kinh tế số của xã — tâm, bán kính và khung giới hạn [kinh độ nhỏ, vĩ độ nhỏ, kinh độ lớn, vĩ độ lớn] để đặt maxBounds; xã chưa đặt thì chỉ có configured=false
	// @screen   10-ban-do-kinh-te-so §4
	// @reply    200 mapFrameOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-frame",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.GetMapFrame)))

	// SET OR MOVE THE FRAME. PUT AND NOT PATCH: the form is saved whole and the three values only make
	// sense together. There is no DELETE: H3 has no "frame off" state, and 0016's trigger refuses one.
	//
	// `admin.lookup` — H3 "Ai đặt": "người giữ admin.lookup". Holding `asset.update` is NOT enough: editing
	// what is on the map is not deciding how much of the country the map may show.
	//
	//	400 invalid_request          not JSON, or a field missing
	//	422 center_outside_mainland  the (rounded) centre is outside the mainland box of 0016's CHECK
	//	422 radius_out_of_range      the (rounded) radius is outside 0016's CHECK
	//
	// idem.KhongCan: the result is the state the body names, and app.MapFrames.Save writes and audits
	// NOTHING when the rounded values equal the stored ones — so the same request twice leaves one row
	// and one entry. Were that comparison removed, this declaration would become a lie.
	//
	// @summary  Đặt hoặc đổi khung bản đồ kinh tế số của xã — tâm phải trong khung đất liền Việt Nam, bán kính trong giới hạn; có ghi vết
	// @screen   10-ban-do-kinh-te-so §4
	// @request  mapFrameIn
	// @reply    200 mapFrameOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error center_outside_mainland radius_out_of_range
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/map-frame",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lưu là ghi đè khung bằng đúng giá trị trong thân; giá trị (đã làm tròn) trùng giá trị đang lưu thì không ghi và không có vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.PutMapFrame))))
}

// GetMapFrame — GET /api/v1/map-frame
func (h *mapFrameHandler) GetMapFrame(w http.ResponseWriter, r *http.Request) {
	// Scoped in the use case: MapFrameStore.Get reads through db.For(ctx).Query.
	v, err := h.d.Reader.Get(r.Context())
	if err != nil {
		h.internal(r.Context(), w, "đọc", err)
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
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ghi khung bản đồ chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	// Scoped in the use case: app.MapFrames opens db.For(ctx).Tx.
	v, err := h.d.Writer.Save(r.Context(), app.MapFrameInput{
		CenterLat: *in.CenterLat, CenterLng: *in.CenterLng, RadiusKm: *in.RadiusKm,
	}, actor)
	switch {
	case err == nil:
		vietJSON(w, http.StatusOK, mapFrameToOut(v))
	case errors.Is(err, domain.ErrMapFrameCenterOutsideMainland):
		// The bounds are read from domain, never retyped: a second copy of an owner-pending number drifts.
		httpx.WriteError(w, http.StatusUnprocessableEntity, "center_outside_mainland",
			fmt.Sprintf("Tâm khung bản đồ phải nằm trong khung đất liền Việt Nam: vĩ độ từ %g đến %g, kinh độ từ %g đến %g.",
				domain.MapFrameMinLat, domain.MapFrameMaxLat, domain.MapFrameMinLng, domain.MapFrameMaxLng), "")
	case errors.Is(err, domain.ErrMapFrameRadiusOutOfRange):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "radius_out_of_range",
			fmt.Sprintf("Bán kính khung bản đồ phải từ %g đến %g km.",
				domain.MapFrameMinRadiusKm, domain.MapFrameMaxRadiusKm), "")
	default:
		h.internal(r.Context(), w, "lưu", err)
	}
}

func (h *mapFrameHandler) internal(ctx context.Context, w http.ResponseWriter, op string, err error) {
	// The wrapped error never reaches the client (rule 3, forbidden #3).
	h.d.Log.Error("khung bản đồ: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
