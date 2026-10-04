package domain

// THE MAP FRAME — the fixed view ONE commune's economic map may show: a centre and a radius
// (migrations/0016_map_frame.sql; ADR 0072 §"Sửa đổi 04/10/2026", H3). web-admin sets MapLibre's
// `maxBounds` from the bounds computed here, so the view can neither be dragged nor zoomed out to the
// national scale where the world-wide tiles draw Hoàng Sa and Trường Sa under OSM's names.
//
// THE NUMBERS BELOW ARE THE SAME AS THE TWO NAMED CHECKs: the centre box is 0016's
// map_frame_center_in_mainland_box; the radius bound is 0017's map_frame_radius_ceiling (ADR 0072
// §"Sửa đổi 04/10/2026 (lần 2)", K2 — owner-decided, a HARD ceiling). Changing one here without a
// migration changing the CHECK makes the service accept what the database then refuses (a 500), or
// refuse what it would accept. Raising the radius past 50 km, or widening the centre box, is a stop
// condition of the second revision ("Điểm dừng — thêm ở lần 2").
//
// ROUNDING BEFORE VALIDATING. The columns are numeric(10,6) and numeric(4,1): PostgreSQL ROUNDS a
// longer value on insert rather than refusing it. So the service rounds first, to the same scales,
// and validates the rounded value — the value checked is exactly the value stored, and a centre typed
// as 109.5000004 is the 109.500000 the database would keep, not a refusal the CHECK would never make.

import (
	"errors"
	"math"
	"time"
)

// The mainland box the CENTRE must lie in — map_frame_center_in_mainland_box. A rectangle, not a
// border: the eastern edge 109.5°E is what keeps Hoàng Sa (~111°E) and Trường Sa out of any frame.
const (
	MapFrameMinLat = 8.4
	MapFrameMaxLat = 23.4
	MapFrameMinLng = 102.1
	MapFrameMaxLng = 109.5
)

// The radius bounds in km — 0017's map_frame_radius_ceiling, `radius_km > 0 AND radius_km <= 50`
// (ADR 0072 K2: the owner chose no other policy bound; whoever sets a value answers for it).
// MapFrameMinRadiusKm IS THE EXCLUSIVE "> 0" FLOOR, not a smallest allowed value: with numeric(4,1)
// the smallest storable radius above it is 0.1 km. The ceiling is what keeps a frame away from the far
// sea — K2's table: ≈ 109.99°E at most, still ≥ 1.2° short of Hoàng Sa / Trường Sa.
const (
	MapFrameMinRadiusKm = 0.0
	MapFrameMaxRadiusKm = 50.0
)

// The RECOMMENDED radius and the "usual" band in km — ADR 0072 K2 "Mức khuyến nghị", a PROPOSAL of the
// main session the owner may adjust ("Việc còn mở sau lần 2" #2). Comms ONLY RETURNS them, as hints
// for the form: it never refuses a radius for being outside the band. Refusing on them would turn an
// owner-adjustable proposal into a second ceiling nobody decided.
const (
	MapFrameRecommendedRadiusKm = 10.0
	MapFrameUsualMinKm          = 3.0
	MapFrameUsualMaxKm          = 20.0
)

// MapFrameNoticeVersion is the version of the legal notice a holder of `admin.lookup` must acknowledge
// on EVERY change of the frame (ADR 0072 K4/K6). ADR 0072 K6 OWNS THE TEXT; this is only its version.
// One word of the text changed = a NEW version, written in the ADR first, then here — and from that
// moment every save carrying the old version is refused, which is the point: an acknowledgement of a
// text the official was not shown is no acknowledgement.
const MapFrameNoticeVersion = "2026-10-04.1"

// Which frame a commune is applying (ADR 0072 K3) — wire VALUES web-admin reads.
const (
	MapFrameSourceCommune = "commune" // the commune's own row, is_enabled = true
	MapFrameSourceDefault = "default" // no own frame (no row, or is_enabled = false): the platform default
)

// KmPerDegree — kilometres per degree of latitude, and of longitude at the equator. A spherical
// approximation; at a ≤ 50 km frame its error is metres, and the bounds are rounded OUTWARD anyway.
const KmPerDegree = 111.32

// The stored scales: numeric(10,6) for the centre, numeric(4,1) for the radius.
const (
	mapFrameCoordScale  = 1e6
	mapFrameRadiusScale = 1e1
)

// MapFrameSubject is the audit subject: one row per commune, so the business address of the row is the
// configuration itself. A VALUE (ADR 0011), like MailSettingsSubject.
const MapFrameSubject = "khung_ban_do"

var (
	ErrMapFrameCenterOutsideMainland = errors.New("map_frame: tâm khung nằm ngoài khung đất liền Việt Nam")
	ErrMapFrameRadiusOutOfRange      = errors.New("map_frame: bán kính ngoài khoảng cho phép")
	// ErrMapFrameNoticeNotAcknowledged — the change does not carry the CURRENT notice version (K4: the
	// server refuses, not only the form — rule 5, forbidden #1).
	ErrMapFrameNoticeNotAcknowledged = errors.New("map_frame: chưa xác nhận lưu ý pháp lý phiên bản hiện hành")
)

// MapFrame is one commune's frame. CreatedBy / UpdatedBy are BUSINESS codes (`CB-…`, rule 6 inv. 8).
// Enabled false is "Về mặc định" (0017): the row and its last values are kept, the commune applies the
// platform default instead.
type MapFrame struct {
	CenterLat float64
	CenterLng float64
	RadiusKm  float64
	Enabled   bool
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt time.Time
	UpdatedBy string
}

// MapFrameBounds is the box MapLibre is held inside, in [west, south, east, north] order.
type MapFrameBounds struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}

// NormalizeMapFrame rounds to the stored scales and validates the rounded values (see the header).
// NaN and ±Inf are refused as outside: every comparison with NaN is false, so without the explicit
// check a NaN centre would pass both range tests.
func NormalizeMapFrame(lat, lng, radiusKm float64) (MapFrame, error) {
	lat = math.Round(lat*mapFrameCoordScale) / mapFrameCoordScale
	lng = math.Round(lng*mapFrameCoordScale) / mapFrameCoordScale
	radiusKm = math.Round(radiusKm*mapFrameRadiusScale) / mapFrameRadiusScale

	if !finite(lat) || !finite(lng) ||
		lat < MapFrameMinLat || lat > MapFrameMaxLat || lng < MapFrameMinLng || lng > MapFrameMaxLng {
		return MapFrame{}, ErrMapFrameCenterOutsideMainland
	}
	if !finite(radiusKm) || radiusKm <= MapFrameMinRadiusKm || radiusKm > MapFrameMaxRadiusKm {
		return MapFrame{}, ErrMapFrameRadiusOutOfRange
	}
	return MapFrame{CenterLat: lat, CenterLng: lng, RadiusKm: radiusKm}, nil
}

// CheckMapFrameNotice refuses a change that does not acknowledge the CURRENT notice version — missing,
// empty and stale all alike (ADR 0072 K4).
func CheckMapFrameNotice(version string) error {
	if version != MapFrameNoticeVersion {
		return ErrMapFrameNoticeNotAcknowledged
	}
	return nil
}

// SameMapFrame compares the three editable values. Enabled is NOT compared: the caller decides what an
// identical frame on a disabled row means (a save re-enables it).
func SameMapFrame(a, b MapFrame) bool {
	return a.CenterLat == b.CenterLat && a.CenterLng == b.CenterLng && a.RadiusKm == b.RadiusKm
}

// Bounds is the box around the frame: lat ± r/111.32, lng ± r/(111.32·cos lat), ROUNDED OUTWARD to 6
// decimals — the minimum down, the maximum up — so the box never cuts into the circle it encloses.
// Computed here and not in the browser so web-admin applies it as is, with no second copy of the
// geometry to drift.
func (f MapFrame) Bounds() MapFrameBounds {
	dLat := f.RadiusKm / KmPerDegree
	dLng := f.RadiusKm / (KmPerDegree * math.Cos(f.CenterLat*math.Pi/180))
	return MapFrameBounds{
		MinLng: math.Floor((f.CenterLng-dLng)*mapFrameCoordScale) / mapFrameCoordScale,
		MinLat: math.Floor((f.CenterLat-dLat)*mapFrameCoordScale) / mapFrameCoordScale,
		MaxLng: math.Ceil((f.CenterLng+dLng)*mapFrameCoordScale) / mapFrameCoordScale,
		MaxLat: math.Ceil((f.CenterLat+dLat)*mapFrameCoordScale) / mapFrameCoordScale,
	}
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
