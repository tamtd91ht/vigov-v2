package domain

import (
	"errors"
	"testing"
)

// Migration 0030 (owner, 14/10/2026): one category tree per content type, and a banner has no category.

func TestValidCategoryTypeIsTheFiveTypesWithoutBanner(t *testing.T) {
	for _, ok := range []string{"tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video"} {
		if !ValidCategoryType(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	// banner is a content type but carries no tree; the rest are not types at all.
	for _, bad := range []string{"banner", "", " tin-tuc", "TIN-TUC", "tin_tuc", "news"} {
		if ValidCategoryType(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	// Every category type is a content type: the two lists cannot drift apart in that direction.
	for _, l := range []LoaiNoiDung{LoaiTinTuc, LoaiSuKien, LoaiThongBao, LoaiTruyenThanh, LoaiVideo} {
		if !LoaiNoiDungHopLe(string(l)) {
			t.Errorf("%q is a category type but not a content type", l)
		}
	}
}

func TestCreateCategoryRequestRequiresAType(t *testing.T) {
	for _, typ := range []string{"", "banner", "khong-co"} {
		if _, err := (YeuCauThemDanhMuc{Ten: "Lễ hội", Type: typ}).KiemTra(); !errors.Is(err, ErrCategoryTypeInvalid) {
			t.Errorf("type %q: err = %v, want ErrCategoryTypeInvalid", typ, err)
		}
	}
	got, err := (YeuCauThemDanhMuc{Ten: "Lễ hội", Type: "su-kien"}).KiemTra()
	if err != nil || got.Type != "su-kien" {
		t.Fatalf("valid type: %+v, %v", got, err)
	}
}

func TestBannerCreateWithCategoryIsRefused(t *testing.T) {
	_, err := (YeuCauThemNoiDung{Loai: "banner", TieuDe: "Ngày hội", DanhMucID: "dm-1"}).KiemTra()
	if !errors.Is(err, ErrBannerHasNoCategory) {
		t.Fatalf("err = %v, want ErrBannerHasNoCategory", err)
	}
	// Without a category the banner passes this rule (its cover is the use case's question).
	if _, err := (YeuCauThemNoiDung{Loai: "banner", TieuDe: "Ngày hội"}).KiemTra(); err != nil {
		t.Fatalf("banner without category: %v", err)
	}
	// Any other type may name a category; whether it is of the same type is the use case's question.
	if _, err := (YeuCauThemNoiDung{Loai: "video", TieuDe: "Clip", DanhMucID: "dm-1"}).KiemTra(); err != nil {
		t.Fatalf("video with category: %v", err)
	}
}
