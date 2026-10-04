package domain

// THE MAP FRAME — the fixed view ONE commune's economic map may show: a centre and a radius
// (migrations/0016_map_frame.sql; ADR 0072 §"Sửa đổi 04/10/2026", H3). web-admin sets MapLibre's
// `maxBounds` from the bounds computed here, so the view can neither be dragged nor zoomed out to the
// national scale where the world-wide tiles draw Hoàng Sa and Trường Sa under OSM's names.
//
// THE NUMBERS BELOW ARE THE SAME AS 0016's TWO NAMED CHECKs, and both are OWNER-PENDING (ADR 0072
// "Mở" #4: "Cận bán kính 1–30 km (đề xuất) và giá trị khung đất liền — chủ dự án duyệt khi dựng").
// Changing one here without a migration changing the CHECK makes the service accept what the database
// then refuses (a 500), or refuse what it would accept. Widening either is H3's stop condition 2.
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

// The radius bounds in km — map_frame_radius_range. The upper bound is what keeps a frame away from
// the far sea (ADR 0072 H3 point 3).
const (
	MapFrameMinRadiusKm = 1.0
	MapFrameMaxRadiusKm = 30.0
)

// KmPerDegree — kilometres per degree of latitude, and of longitude at the equator. A spherical
// approximation; at a ≤ 30 km frame its error is metres, and the bounds are rounded OUTWARD anyway.
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
)

// MapFrame is one commune's frame. CreatedBy / UpdatedBy are BUSINESS codes (`CB-…`, rule 6 inv. 8).
type MapFrame struct {
	CenterLat float64
	CenterLng float64
	RadiusKm  float64
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
	if !finite(radiusKm) || radiusKm < MapFrameMinRadiusKm || radiusKm > MapFrameMaxRadiusKm {
		return MapFrame{}, ErrMapFrameRadiusOutOfRange
	}
	return MapFrame{CenterLat: lat, CenterLng: lng, RadiusKm: radiusKm}, nil
}

// SameMapFrame compares the three editable values.
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
