package domain

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeMapFrameEdges(t *testing.T) {
	for name, tc := range map[string]struct {
		lat, lng, r float64
		want        error
	}{
		"south edge 8.4":             {8.4, 105, 10, nil},
		"north edge 23.4":            {23.4, 105, 10, nil},
		"west edge 102.1":            {16, 102.1, 10, nil},
		"east edge 109.5":            {16, 109.5, 10, nil},
		"east past 109.51":           {16, 109.51, 10, ErrMapFrameCenterOutsideMainland},
		"Hoàng Sa 112":               {16.5, 112, 10, ErrMapFrameCenterOutsideMainland},
		"south past 8.39":            {8.39, 105, 10, ErrMapFrameCenterOutsideMainland},
		"north past 23.41":           {23.41, 105, 10, ErrMapFrameCenterOutsideMainland},
		"west past 102.09":           {16, 102.09, 10, ErrMapFrameCenterOutsideMainland},
		"radius 0.1 (smallest > 0)":  {16, 108, 0.1, nil},
		"radius 0.9":                 {16, 108, 0.9, nil},
		"radius 30.1 (old ceiling)":  {16, 108, 30.1, nil},
		"radius 50 (K2 ceiling)":     {16, 108, 50, nil},
		"radius 50.1":                {16, 108, 50.1, ErrMapFrameRadiusOutOfRange},
		"radius 0.04 rounds to 0":    {16, 108, 0.04, ErrMapFrameRadiusOutOfRange},
		"radius 0":                   {16, 108, 0, ErrMapFrameRadiusOutOfRange},
		"radius negative":            {16, 108, -5, ErrMapFrameRadiusOutOfRange},
		"NaN centre":                 {math.NaN(), 108, 10, ErrMapFrameCenterOutsideMainland},
		"Inf radius":                 {16, 108, math.Inf(1), ErrMapFrameRadiusOutOfRange},
		"centre checked before size": {0, 0, 0, ErrMapFrameCenterOutsideMainland},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeMapFrame(tc.lat, tc.lng, tc.r)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeMapFrameRoundsToStoredScalesFirst(t *testing.T) {
	// numeric(10,6) / numeric(4,1) ROUND on insert; the value validated is the value stored.
	f, err := NormalizeMapFrame(15.7305074, 108.3781096, 12.34)
	if err != nil {
		t.Fatal(err)
	}
	if f.CenterLat != 15.730507 || f.CenterLng != 108.37811 || f.RadiusKm != 12.3 {
		t.Errorf("= %+v", f)
	}
	// 109.5000004 rounds to 109.5 — accepted, as the CHECK would accept the stored value.
	if _, err := NormalizeMapFrame(16, 109.5000004, 10); err != nil {
		t.Errorf("109.5000004: %v", err)
	}
	// 50.04 rounds to 50.0 (accepted); 50.06 to 50.1 (refused); 0.06 to 0.1 (accepted).
	if _, err := NormalizeMapFrame(16, 108, 50.04); err != nil {
		t.Errorf("50.04: %v", err)
	}
	if _, err := NormalizeMapFrame(16, 108, 50.06); !errors.Is(err, ErrMapFrameRadiusOutOfRange) {
		t.Errorf("50.06: %v", err)
	}
	if _, err := NormalizeMapFrame(16, 108, 0.06); err != nil {
		t.Errorf("0.06: %v", err)
	}
}

func TestMapFrameBoundsAreOutwardSixDecimals(t *testing.T) {
	f := MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 10}
	b := f.Bounds()
	// Latitude has no cosine: 10/111.32 = 0.0898311…, so 15.9101688… floors and 16.0898311… ceils.
	if b.MinLat != 15.910168 || b.MaxLat != 16.089832 {
		t.Errorf("lat bounds = %v, %v", b.MinLat, b.MaxLat)
	}

	for _, f := range []MapFrame{
		{CenterLat: 16, CenterLng: 108, RadiusKm: 10},
		{CenterLat: 8.4, CenterLng: 104.9, RadiusKm: 1},
		{CenterLat: 23.4, CenterLng: 105.3, RadiusKm: 50},
		{CenterLat: 15.730507, CenterLng: 108.37811, RadiusKm: 12.3},
	} {
		b := f.Bounds()
		dLat := f.RadiusKm / KmPerDegree
		dLng := f.RadiusKm / (KmPerDegree * math.Cos(f.CenterLat*math.Pi/180))
		exact := [4]float64{f.CenterLng - dLng, f.CenterLat - dLat, f.CenterLng + dLng, f.CenterLat + dLat}
		got := [4]float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat}
		for i := range got {
			if math.Abs(got[i]*1e6-math.Round(got[i]*1e6)) > 1e-6 {
				t.Errorf("%+v bound %d = %v — not 6 decimals", f, i, got[i])
			}
			// Outward: min ≤ exact, max ≥ exact, and by less than one step of the 6th decimal.
			outward := got[i] <= exact[i]
			if i >= 2 {
				outward = got[i] >= exact[i]
			}
			if !outward || math.Abs(got[i]-exact[i]) > 1e-6+1e-12 {
				t.Errorf("%+v bound %d = %v, exact %v — not rounded outward by < 1e-6", f, i, got[i], exact[i])
			}
		}
		// Longitude half-width is wider than latitude's away from the equator (cos < 1).
		if b.MaxLng-b.MinLng <= b.MaxLat-b.MinLat {
			t.Errorf("%+v lng span %v not wider than lat span %v", f, b.MaxLng-b.MinLng, b.MaxLat-b.MinLat)
		}
	}
}

func TestEastEdgeFrameStaysWestOfHoangSa(t *testing.T) {
	// ADR 0072 K2's table: at the 50 km ceiling a centre on 109.5°E reaches ≈ 109.95°E (8.4°N) to
	// ≈ 109.99°E (23.4°N) — still ≥ 1.2° short of Hoàng Sa (≈ 111.2°E).
	for _, lat := range []float64{MapFrameMinLat, MapFrameMaxLat} {
		f, err := NormalizeMapFrame(lat, MapFrameMaxLng, MapFrameMaxRadiusKm)
		if err != nil {
			t.Fatal(err)
		}
		if b := f.Bounds(); b.MaxLng >= 110 {
			t.Errorf("lat %v: east bound %v — past K2's ≈ 109.99°E", lat, b.MaxLng)
		}
	}
}

func TestMapFrameNoticeOnlyTheCurrentVersion(t *testing.T) {
	if MapFrameNoticeVersion != "2026-10-04.1" {
		// ADR 0072 K6 owns the version; a change here without the ADR is its stop condition 3.
		t.Fatalf("notice version %q — not ADR 0072 K6's", MapFrameNoticeVersion)
	}
	if err := CheckMapFrameNotice(MapFrameNoticeVersion); err != nil {
		t.Errorf("current version refused: %v", err)
	}
	for _, v := range []string{"", " 2026-10-04.1", "2026-10-04", "2026-10-04.0"} {
		if err := CheckMapFrameNotice(v); !errors.Is(err, ErrMapFrameNoticeNotAcknowledged) {
			t.Errorf("%q: %v, want refusal", v, err)
		}
	}
}

func TestMapFrameHintsSitInsideTheHardBounds(t *testing.T) {
	// The hints are proposals; they must at least be values the save would accept.
	for _, r := range []float64{MapFrameRecommendedRadiusKm, MapFrameUsualMinKm, MapFrameUsualMaxKm} {
		if _, err := NormalizeMapFrame(16, 108, r); err != nil {
			t.Errorf("hint %v refused: %v", r, err)
		}
	}
	if !(MapFrameUsualMinKm <= MapFrameRecommendedRadiusKm && MapFrameRecommendedRadiusKm <= MapFrameUsualMaxKm) {
		t.Error("recommended radius outside the usual band")
	}
}
