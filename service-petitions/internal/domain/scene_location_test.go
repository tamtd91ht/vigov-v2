package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// The optional scene location a citizen sends with a petition (ADR 0050).
//
//	PROVED HERE   neither is valid and stays nil · one alone is refused · the world's range is the
//	              bound (not Vietnam's) and its edges are accepted · NaN/Inf are refused · the value
//	              is rounded to the column's 6 decimals and -0 folds to 0 ·
//	              both refusals are intake errors (400) · no refusal echoes the value sent.
//	NOT PROVED    that PostgreSQL NUMERIC(9,6) stores the rounded value unchanged — the pg tests own
//	              that, and they skip without VIGOV_TEST_DSN.

func coord(v float64) *float64 { return &v }

func TestSceneLocationNeitherIsValid(t *testing.T) {
	lat, lng, err := NormaliseSceneLocation(nil, nil)
	if err != nil || lat != nil || lng != nil {
		t.Errorf("= %v, %v, %v; muốn nil, nil, nil — vị trí vẫn là tuỳ chọn (ADR 0050 điểm 9)", lat, lng, err)
	}
}

func TestSceneLocationOneAloneIsRefused(t *testing.T) {
	for ten, c := range map[string][2]*float64{
		"chỉ lat": {coord(16.0544), nil},
		"chỉ lng": {nil, coord(108.2022)},
	} {
		t.Run(ten, func(t *testing.T) {
			_, _, err := NormaliseSceneLocation(c[0], c[1])
			if !errors.Is(err, ErrSceneLocationIncomplete) {
				t.Fatalf("lỗi = %v, muốn ErrSceneLocationIncomplete", err)
			}
			if !LaLoiGuiPhanAnh(err) {
				t.Error("không được xếp là lỗi đầu vào — sẽ thành 500 thay vì 400")
			}
		})
	}
}

func TestSceneLocationRange(t *testing.T) {
	for ten, c := range map[string][2]float64{
		"lat > 90":   {90.000001, 0},
		"lat < -90":  {-90.5, 0},
		"lng > 180":  {0, 180.1},
		"lng < -180": {0, -181},
		"lat NaN":    {math.NaN(), 0},
		"lng +Inf":   {0, math.Inf(1)},
		"lng -Inf":   {0, math.Inf(-1)},
	} {
		t.Run(ten, func(t *testing.T) {
			_, _, err := NormaliseSceneLocation(coord(c[0]), coord(c[1]))
			if !errors.Is(err, ErrSceneLocationOutOfRange) || !LaLoiGuiPhanAnh(err) {
				t.Errorf("lỗi = %v, muốn ErrSceneLocationOutOfRange (400)", err)
			}
		})
	}
	// THE EDGES AND A POINT FAR OUTSIDE VIETNAM ARE ACCEPTED: vigov-require bounds the world, not
	// the country (schemas.py:28-29), and so does this.
	for ten, c := range map[string][2]float64{
		"bốn góc +":  {90, 180},
		"bốn góc -":  {-90, -180},
		"ngoài VN":   {48.8566, 2.3522},
		"gốc toạ độ": {0, 0},
	} {
		t.Run(ten, func(t *testing.T) {
			lat, lng, err := NormaliseSceneLocation(coord(c[0]), coord(c[1]))
			if err != nil || *lat != c[0] || *lng != c[1] {
				t.Errorf("= %v, %v, %v; muốn nhận nguyên %v", lat, lng, err, c)
			}
		})
	}
}

func TestSceneLocationRoundsToSixDecimals(t *testing.T) {
	for _, c := range []struct{ vao, muon float64 }{
		{16.05440049, 16.0544},
		{16.05440051, 16.054401},
		{-108.20220051, -108.202201},
		{108.123456, 108.123456},
		{-0.0000001, 0},
	} {
		// The lng slot, whose range admits every fixture; lat goes through the same helper.
		_, lng, err := NormaliseSceneLocation(coord(0), coord(c.vao))
		if err != nil {
			t.Fatalf("%v: %v", c.vao, err)
		}
		if *lng != c.muon {
			t.Errorf("làm tròn %v = %v, muốn %v", c.vao, *lng, c.muon)
		}
		if math.Signbit(*lng) && *lng == 0 {
			t.Errorf("làm tròn %v ra -0 — dây sẽ mang \"-0\"", c.vao)
		}
	}
}

func TestSceneLocationRefusalEchoesNothing(t *testing.T) {
	for _, err := range []error{
		func() error { _, _, e := NormaliseSceneLocation(coord(16.0544123), nil); return e }(),
		func() error { _, _, e := NormaliseSceneLocation(coord(96.0544123), coord(108.2022789)); return e }(),
	} {
		for _, bi := range []string{"16.05", "96.05", "108.20"} {
			if strings.Contains(err.Error(), bi) {
				t.Errorf("thông báo lỗi mang toạ độ đã gửi (%q): %v", bi, err)
			}
		}
	}
}
