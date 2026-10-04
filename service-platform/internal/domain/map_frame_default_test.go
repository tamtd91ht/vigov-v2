package domain

import (
	"errors"
	"math"
	"testing"
)

// The hard bounds of ADR 0072 K2 at their edges, after rounding to the stored scales.
func TestNormalizeMapFrameDefaultEdges(t *testing.T) {
	const lat, lng = 15.73, 108.37
	for _, c := range []struct {
		name        string
		lat, lng, r float64
		want        error
		wantR       float64
	}{
		{"radius 0", lat, lng, 0, ErrMapFrameRadiusOutOfRange, 0},
		{"radius rounds to 0", lat, lng, 0.04, ErrMapFrameRadiusOutOfRange, 0},
		{"radius 0.1", lat, lng, 0.1, nil, 0.1},
		{"radius 50", lat, lng, 50, nil, 50},
		{"radius 50.1", lat, lng, 50.1, ErrMapFrameRadiusOutOfRange, 0},
		{"radius 50.04 rounds to 50", lat, lng, 50.04, nil, 50},
		{"negative radius", lat, lng, -1, ErrMapFrameRadiusOutOfRange, 0},
		{"NaN radius", lat, lng, math.NaN(), ErrMapFrameRadiusOutOfRange, 0},
		{"Inf radius", lat, lng, math.Inf(1), ErrMapFrameRadiusOutOfRange, 0},
		{"box corner SW", 8.4, 102.1, 10, nil, 10},
		{"box corner NE", 23.4, 109.5, 10, nil, 10},
		{"east of the box (Hoàng Sa)", 16.5, 111.6, 10, ErrMapFrameCenterOutsideMainland, 0},
		{"just east", lat, 109.500001, 10, ErrMapFrameCenterOutsideMainland, 0},
		{"rounds onto the edge", lat, 109.5000004, 10, nil, 10},
		{"south of the box", 8.3, lng, 10, ErrMapFrameCenterOutsideMainland, 0},
		{"NaN centre", math.NaN(), lng, 10, ErrMapFrameCenterOutsideMainland, 0},
	} {
		f, err := NormalizeMapFrameDefault(c.lat, c.lng, c.r)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
			continue
		}
		if err == nil && f.RadiusKm != c.wantR {
			t.Errorf("%s: radius %v, want %v", c.name, f.RadiusKm, c.wantR)
		}
	}
}

// 3 and 20 are usual; anything strictly outside needs the operator's confirmation (K2).
func TestCheckUnusualRadius(t *testing.T) {
	for r, unusual := range map[float64]bool{0.1: true, 2.9: true, 3: false, 10: false, 20: false, 20.1: true, 50: true} {
		if got := IsUnusualRadius(r); got != unusual {
			t.Errorf("IsUnusualRadius(%v) = %v, want %v", r, got, unusual)
		}
		err := CheckUnusualRadius(r, false)
		if unusual != errors.Is(err, ErrMapFrameRadiusUnusualUnconfirmed) {
			t.Errorf("CheckUnusualRadius(%v, false) = %v", r, err)
		}
		if err := CheckUnusualRadius(r, true); err != nil {
			t.Errorf("CheckUnusualRadius(%v, true) = %v, want nil", r, err)
		}
	}
}

// The bounds enclose the circle and are rounded outward; at the 50 km ceiling on the eastern edge they
// stay short of Hoàng Sa (K2's table: ≈ 109.99°E).
func TestMapFrameDefaultBounds(t *testing.T) {
	b := MapFrameDefault{CenterLat: 16, CenterLng: 108, RadiusKm: 11.132}.Bounds()
	// Outward rounding may add one micro-degree on top of float error, never take one away.
	if b.MinLat > 15.9 || b.MinLat < 15.899999 || b.MaxLat < 16.1 || b.MaxLat > 16.100001 {
		t.Errorf("lat bounds %v..%v, want 15.9..16.1 rounded outward", b.MinLat, b.MaxLat)
	}
	if !(b.MinLng < 108-0.1 && b.MaxLng > 108+0.1) {
		t.Errorf("lng bounds %v..%v must widen with cos(lat)", b.MinLng, b.MaxLng)
	}
	for _, lat := range []float64{8.4, 23.4} {
		e := MapFrameDefault{CenterLat: lat, CenterLng: MapFrameMaxLng, RadiusKm: MapFrameMaxRadiusKm}.Bounds()
		if e.MaxLng >= 110 || e.MaxLng <= 109.9 {
			t.Errorf("at lat %v the 50 km frame reaches %v°E, want ≈ 109.95–109.99", lat, e.MaxLng)
		}
	}
}
