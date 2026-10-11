package domain

import (
	"errors"
	"testing"
)

// Migration 0028's two CHECKs, answered in Go before any transaction.

func TestCreateVideoDuration(t *testing.T) {
	base := YeuCauThemNoiDung{Loai: string(LoaiVideo), TieuDe: "Video"}
	for _, c := range []struct {
		name    string
		loai    LoaiNoiDung
		seconds int
		want    error
	}{
		{"absent", LoaiVideo, 0, nil},
		{"lower bound", LoaiVideo, 1, nil},
		{"upper bound", LoaiVideo, VideoDurationMaxSeconds, nil},
		{"over the bound", LoaiVideo, VideoDurationMaxSeconds + 1, ErrVideoDurationInvalid},
		{"negative", LoaiVideo, -5, ErrVideoDurationInvalid},
		{"on a news article", LoaiTinTuc, 60, ErrVideoDurationOnlyForVideo},
	} {
		y := base
		y.Loai, y.VideoDurationSeconds = string(c.loai), c.seconds
		got, err := y.KiemTra()
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
			continue
		}
		if err == nil && got.VideoDurationSeconds != c.seconds {
			t.Errorf("%s: kept %d, want %d", c.name, got.VideoDurationSeconds, c.seconds)
		}
	}
}

func TestEditVideoDuration(t *testing.T) {
	zero, ok, bad := 0, 95, VideoDurationMaxSeconds+1
	if _, err := (YeuCauSuaNoiDung{VideoDurationSeconds: &bad}).KiemTra(); !errors.Is(err, ErrVideoDurationInvalid) {
		t.Errorf("out of range: %v", err)
	}
	got, err := (YeuCauSuaNoiDung{VideoDurationSeconds: &zero}).KiemTra()
	if err != nil || got.VideoDurationSeconds == nil || *got.VideoDurationSeconds != 0 {
		t.Errorf("0 must survive as CLEAR: %v %v", got.VideoDurationSeconds, err)
	}
	if (YeuCauSuaNoiDung{VideoDurationSeconds: &zero}).SetsVideoDuration() {
		t.Error("clearing is not setting")
	}
	if !(YeuCauSuaNoiDung{VideoDurationSeconds: &ok}).SetsVideoDuration() {
		t.Error("95 is setting")
	}
}

func TestVideoDurationGoesWithTheType(t *testing.T) {
	n := NoiDungMiniApp{Loai: LoaiTinTuc, VideoDurationSeconds: 95}
	if !errors.Is(n.CheckTypeFields(), ErrVideoDurationOnlyForVideo) {
		t.Error("a news article holding a video length must be refused")
	}
	if got := n.WithoutOtherTypeFields(); got.VideoDurationSeconds != 0 {
		t.Errorf("a type change away from video must clear the length, kept %d", got.VideoDurationSeconds)
	}
	v := NoiDungMiniApp{Loai: LoaiVideo, VideoDurationSeconds: 95}
	if v.WithoutOtherTypeFields().VideoDurationSeconds != 95 || v.CheckTypeFields() != nil {
		t.Error("a video keeps its length")
	}
}
