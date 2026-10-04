package domain

// THE DEFAULT MAP FRAME of one commune — a centre and a radius Vihat sets in the operations console
// (migrations/0020_commune_map_frame_default.sql; ADR 0072 §"Sửa đổi 04/10/2026 (lần 2)", K1–K2).
// service-comms applies it only when the commune has set no frame of its own (K3).
//
// THE NUMBERS BELOW ARE THE SAME AS 0020's TWO NAMED CHECKs, and the same as service-comms'
// internal/domain/map_frame.go — COPIED, NOT IMPORTED: rule 2 forbids importing another service's
// internal package, and the shared contract (platform.proto MapFrameDefault) states the bounds in
// prose only. Three copies of one rule is the cost; the K2 table in ADR 0072 is the owner of the
// numbers, and a change there is a stop condition of the second revision ("Điểm dừng — thêm ở lần 2"),
// so all three move together or not at all. Changing one here without a migration changing the CHECK
// makes the service accept what the database then refuses (a 500), or refuse what it would accept.
//
// ROUNDING BEFORE VALIDATING, as comms does: numeric(10,6) / numeric(4,1) ROUND a longer value on
// insert rather than refusing it, so the value checked here is exactly the value stored.

import (
	"errors"
	"math"
	"time"
)

// The mainland box the CENTRE must lie in — 0020's commune_map_frame_default_center_in_mainland_box
// (ADR 0072 H3, unchanged by K2). The eastern edge 109.5°E keeps Hoàng Sa and Trường Sa out of any frame.
const (
	MapFrameMinLat = 8.4
	MapFrameMaxLat = 23.4
	MapFrameMinLng = 102.1
	MapFrameMaxLng = 109.5
)

// MapFrameMaxRadiusKm is K2's HARD ceiling — 0020's commune_map_frame_default_radius_ceiling
// (`radius_km > 0 AND radius_km <= 50`). The floor is the exclusive "> 0"; with numeric(4,1) the
// smallest storable radius is 0.1 km.
const MapFrameMaxRadiusKm = 50.0

// The RECOMMENDED radius and the USUAL band in km — ADR 0072 K2 "Mức khuyến nghị", a proposal of the
// main session the owner may adjust ("Việc còn mở sau lần 2" #2). NOT a ceiling: a radius outside the
// band is saved when the operator CONFIRMS it (K2 "phải xác nhận mới lưu"; the owner: whoever sets it
// answers for it). The write path enforces the confirmation server-side, because a console can be
// modified (rule 5, forbidden #1). Comms returns the same three numbers as hints and enforces nothing
// on them for its own frame — K4 puts a legal-notice acknowledgement there instead.
const (
	MapFrameRecommendedRadiusKm = 10.0
	MapFrameUsualMinKm          = 3.0
	MapFrameUsualMaxKm          = 20.0
)

// KmPerDegree — kilometres per degree of latitude, and of longitude at the equator. The same spherical
// approximation comms uses for its own frame, so a default and a commune frame of equal centre and
// radius produce byte-identical bounds.
const KmPerDegree = 111.32

// The stored scales: numeric(10,6) for the centre, numeric(4,1) for the radius.
const (
	mapFrameCoordScale  = 1e6
	mapFrameRadiusScale = 1e1
)

// MapFrameDefaultSubject is the audit subject: one row per commune, so the business address of the row
// is the configuration itself. A VALUE (ADR 0011), the counterpart of comms' "khung_ban_do".
const MapFrameDefaultSubject = "khung_ban_do_mac_dinh"

var (
	ErrMapFrameCenterOutsideMainland = errors.New("map_frame_default: tâm khung nằm ngoài khung đất liền Việt Nam")
	ErrMapFrameRadiusOutOfRange      = errors.New("map_frame_default: bán kính ngoài khoảng cho phép (> 0 và ≤ 50 km)")
	// ErrMapFrameRadiusUnusualUnconfirmed — the radius is outside the usual band and the request did not
	// carry the operator's confirmation (K2).
	ErrMapFrameRadiusUnusualUnconfirmed = errors.New("map_frame_default: bán kính lệch xa mức khuyến nghị mà chưa xác nhận")
)

// MapFrameDefault is one commune's default frame. CreatedBy / UpdatedBy are operator BUSINESS codes
// (`VH-…`, rule 6 invariant 8).
type MapFrameDefault struct {
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

// NormalizeMapFrameDefault rounds to the stored scales and validates the rounded values against the
// HARD bounds. NaN and ±Inf are refused as outside: every comparison with NaN is false, so without the
// explicit check a NaN centre would pass both range tests.
func NormalizeMapFrameDefault(lat, lng, radiusKm float64) (MapFrameDefault, error) {
	lat = math.Round(lat*mapFrameCoordScale) / mapFrameCoordScale
	lng = math.Round(lng*mapFrameCoordScale) / mapFrameCoordScale
	radiusKm = math.Round(radiusKm*mapFrameRadiusScale) / mapFrameRadiusScale

	if !finite(lat) || !finite(lng) ||
		lat < MapFrameMinLat || lat > MapFrameMaxLat || lng < MapFrameMinLng || lng > MapFrameMaxLng {
		return MapFrameDefault{}, ErrMapFrameCenterOutsideMainland
	}
	if !finite(radiusKm) || radiusKm <= 0 || radiusKm > MapFrameMaxRadiusKm {
		return MapFrameDefault{}, ErrMapFrameRadiusOutOfRange
	}
	return MapFrameDefault{CenterLat: lat, CenterLng: lng, RadiusKm: radiusKm}, nil
}

// IsUnusualRadius reports whether a (rounded, in-range) radius lies outside the usual band — strictly
// below 3 km or strictly above 20 km. 3 and 20 themselves are usual.
func IsUnusualRadius(radiusKm float64) bool {
	return radiusKm < MapFrameUsualMinKm || radiusKm > MapFrameUsualMaxKm
}

// CheckUnusualRadius refuses an unusual radius the operator did not confirm.
func CheckUnusualRadius(radiusKm float64, acknowledged bool) error {
	if IsUnusualRadius(radiusKm) && !acknowledged {
		return ErrMapFrameRadiusUnusualUnconfirmed
	}
	return nil
}

// SameMapFrameDefault compares the three editable values.
func SameMapFrameDefault(a, b MapFrameDefault) bool {
	return a.CenterLat == b.CenterLat && a.CenterLng == b.CenterLng && a.RadiusKm == b.RadiusKm
}

// Bounds is the box around the frame: lat ± r/111.32, lng ± r/(111.32·cos lat), ROUNDED OUTWARD to 6
// decimals — the minimum down, the maximum up — so the box never cuts into the circle it encloses.
// COPIED FROM service-comms domain.MapFrame.Bounds (rule 2 forbids the import; see the file header):
// the same formula, so the console shows the box comms will later hand to web-admin.
func (f MapFrameDefault) Bounds() MapFrameBounds {
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
