package platformclient

import (
	"context"
	"fmt"
	"math"
	"time"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// MapFrameDefault is a commune's DEFAULT map frame, set by Vihat in the operations console (ADR 0072,
// amendment 2; platform.proto MapFrameDefault). service-comms falls back to it only when the commune
// has set no frame of its own.
type MapFrameDefault struct {
	CenterLat float64
	CenterLng float64
	RadiusKm  float64
	UpdatedAt time.Time // diagnostics only; zero when the platform sent none
}

// The bounds platform.proto MapFrameDefault states. The server refuses to store anything outside
// them; they are checked again here so that a server bug reads as "not configured" (no map drawn)
// rather than as a frame centred somewhere no commune is.
const (
	mapFrameMinLat    = 8.4
	mapFrameMaxLat    = 23.4
	mapFrameMinLng    = 102.1
	mapFrameMaxLng    = 109.5
	mapFrameMaxRadius = 50.0
)

// MapFrameDefault reads the default frame of THE COMMUNE ALREADY IN ctx. The commune is not a
// parameter (rule 1, invariant 4): grpcx.UnaryClientInterceptor writes it into "x-tenant-id", and the
// RPC has no field that could name another one.
//
// ok=false with err=nil: no default frame configured — or the platform sent one outside the contract's
// bounds, which is treated identically (never clamped). The caller draws no map.
// err != nil: no commune in ctx, or the call did not happen — never "not configured", so the caller can
// answer 503 rather than tell staff the commune has no frame.
//
// NOT CACHED, like TenantProfile and unlike uploadpolicy / PetitionFields. Its one caller reaches it
// only on the fallback path of one page-load read, so the rate is that of staff opening a map; a cache
// would add a per-commune map, a TTL and an invalidation lag to a value Vihat expects to see applied as
// soon as it is saved.
func (d *Directory) MapFrameDefault(ctx context.Context) (MapFrameDefault, bool, error) {
	if _, ok := tenant.From(ctx); !ok {
		return MapFrameDefault{}, false, fmt.Errorf("platformclient: MapFrameDefault: %w", tenant.ErrNoTenant)
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	res, err := d.cl.GetMapFrameDefault(ctx, &platformv1.GetMapFrameDefaultRequest{})
	if err != nil {
		return MapFrameDefault{}, false, fmt.Errorf("platformclient: GetMapFrameDefault: %w", err)
	}
	f := res.GetFrame()
	if f == nil {
		return MapFrameDefault{}, false, nil
	}
	lat, lng, r := f.GetCenterLat(), f.GetCenterLng(), f.GetRadiusKm()
	// The negated comparisons also reject NaN, which compares false to everything.
	if !(lat >= mapFrameMinLat && lat <= mapFrameMaxLat) ||
		!(lng >= mapFrameMinLng && lng <= mapFrameMaxLng) ||
		!(r > 0 && r <= mapFrameMaxRadius) || math.IsInf(r, 0) {
		d.log.WarnContext(ctx, "platformclient: default map frame outside contract bounds, treated as not configured")
		return MapFrameDefault{}, false, nil
	}
	out := MapFrameDefault{CenterLat: lat, CenterLng: lng, RadiusKm: r}
	if ts := f.GetUpdatedAt(); ts != nil {
		out.UpdatedAt = ts.AsTime()
	}
	return out, true, nil
}
