package domain

import (
	"errors"
	"strings"
	"testing"
)

// Migration 0012 / ADR 0067 §3 and §5 in the domain: the banner tap target, its order, the cover rule,
// the category edit request, the cycle walk and the hidden-chip rule.

func TestNormalizeLinkToAcceptsTheMigrationsTwoShapes(t *testing.T) {
	for in, want := range map[string]string{
		"":                                  "",
		"  ":                                "",
		"/":                                 "/",
		"/tin-tuc":                          "/tin-tuc",
		"/tin-tuc?type=su-kien#top":         "/tin-tuc?type=su-kien#top",
		"https://dichvucong.gov.vn":         "https://dichvucong.gov.vn",
		"HTTPS://Thangbinh.danang.gov.vn/a": "https://Thangbinh.danang.gov.vn/a",
		" https://x.gov.vn/a?b=c ":          "https://x.gov.vn/a?b=c",
	} {
		got, err := NormalizeLinkTo(in)
		if err != nil || got != want {
			t.Errorf("NormalizeLinkTo(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestNormalizeLinkToRefusesEverythingElse(t *testing.T) {
	for _, in := range []string{
		"//evil.example/x", "/\\evil.example", "http://x.gov.vn", "javascript:alert(1)",
		"data:text/html,x", "intent://x", "zalo://x", "https://", "https:///x", "https://a b.vn",
		"https://gov.vn@evil.example/", "tin-tuc", "/a\tb", "/a\x00b", "ftp://x",
		"/" + strings.Repeat("a", LinkToMaxRunes),
	} {
		if _, err := NormalizeLinkTo(in); !errors.Is(err, ErrLinkToInvalid) {
			t.Errorf("NormalizeLinkTo(%q) err = %v, want ErrLinkToInvalid", in, err)
		}
	}
	// Exactly the bound is accepted (characters, as char_length counts — diacritics are one each).
	if _, err := NormalizeLinkTo("/" + strings.Repeat("ă", LinkToMaxRunes-1)); err != nil {
		t.Errorf("500 ký tự phải được nhận: %v", err)
	}
}

func TestCheckDisplayOrder(t *testing.T) {
	ok := []int{0, 1, 9999, displayOrderMax}
	for _, n := range ok {
		n := n
		if err := CheckDisplayOrder(&n); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
	for _, n := range []int{-1, displayOrderMax + 1} {
		n := n
		if err := CheckDisplayOrder(&n); !errors.Is(err, ErrDisplayOrderInvalid) {
			t.Errorf("%d: err = %v", n, err)
		}
	}
	if err := CheckDisplayOrder(nil); err != nil {
		t.Errorf("nil: %v", err)
	}
}

func TestCreateRefusesBannerFieldsOnAnotherType(t *testing.T) {
	order := 1
	for _, y := range []YeuCauThemNoiDung{
		{Loai: "tin-tuc", TieuDe: "T", LinkTo: "/tin-tuc"},
		{Loai: "su-kien", TieuDe: "T", DisplayOrder: &order},
	} {
		if _, err := y.KiemTra(); !errors.Is(err, ErrBannerFieldsOnlyForBanner) {
			t.Errorf("%s: err = %v", y.Loai, err)
		}
	}
	got, err := YeuCauThemNoiDung{Loai: "banner", TieuDe: "T", LinkTo: "HTTPS://x.gov.vn", DisplayOrder: &order}.KiemTra()
	if err != nil || got.LinkTo != "https://x.gov.vn" || got.DisplayOrder == nil || *got.DisplayOrder != 1 {
		t.Fatalf("banner hợp lệ: %+v, %v", got, err)
	}
	order = 5 // the cleaned request must not alias the caller's variable
	if *got.DisplayOrder != 1 {
		t.Error("DisplayOrder trỏ chung biến của người gọi")
	}
	if _, err := (YeuCauThemNoiDung{Loai: "banner", TieuDe: "T", LinkTo: "//evil"}).KiemTra(); !errors.Is(err, ErrLinkToInvalid) {
		t.Errorf("link_to sai: %v", err)
	}
}

func TestTypeChangeAwayFromBannerClearsItsFields(t *testing.T) {
	order := 3
	n := NoiDungMiniApp{Loai: LoaiTinTuc, LinkTo: "/a", DisplayOrder: &order}
	n = n.WithoutOtherTypeFields()
	if n.LinkTo != "" || n.DisplayOrder != nil {
		t.Errorf("đổi khỏi banner phải xoá link_to/display_order: %+v", n)
	}
	b := NoiDungMiniApp{Loai: LoaiBanner, LinkTo: "/a", DisplayOrder: &order}.WithoutOtherTypeFields()
	if b.LinkTo != "/a" || b.DisplayOrder == nil {
		t.Errorf("banner giữ trường của nó: %+v", b)
	}
	if err := (NoiDungMiniApp{Loai: LoaiVideo, LinkTo: "/a"}).CheckTypeFields(); !errors.Is(err, ErrBannerFieldsOnlyForBanner) {
		t.Errorf("CheckTypeFields: %v", err)
	}
}

func TestCheckBannerCoverMirrorsTheTrigger(t *testing.T) {
	with := NoiDungMiniApp{Loai: LoaiBanner, CoverImageFileID: "f1"}
	without := NoiDungMiniApp{Loai: LoaiBanner}
	news := NoiDungMiniApp{Loai: LoaiTinTuc}
	cases := []struct {
		name   string
		before *NoiDungMiniApp
		after  NoiDungMiniApp
		want   error
	}{
		{"create with cover", nil, with, nil},
		{"create without cover", nil, without, ErrBannerCoverRequired},
		{"news into banner without cover", &news, without, ErrBannerCoverRequired},
		{"cover removed from banner", &with, without, ErrBannerCoverRequired},
		{"legacy coverless banner edited", &without, without, nil},
		{"not a banner", nil, news, nil},
	}
	for _, c := range cases {
		if err := CheckBannerCover(c.before, c.after); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestPatchSetsBannerField(t *testing.T) {
	empty, value, order := "", "/a", 0
	if (YeuCauSuaNoiDung{LinkTo: &empty}).SetsBannerField() {
		t.Error("xoá link_to không phải là đặt giá trị")
	}
	if !(YeuCauSuaNoiDung{LinkTo: &value}).SetsBannerField() || !(YeuCauSuaNoiDung{DisplayOrder: &order}).SetsBannerField() {
		t.Error("đặt link_to / display_order phải tính là đặt giá trị")
	}
	bad := "javascript:x"
	if _, err := (YeuCauSuaNoiDung{LinkTo: &bad}).KiemTra(); !errors.Is(err, ErrLinkToInvalid) {
		t.Errorf("PATCH link_to sai: %v", err)
	}
	neg := -1
	if _, err := (YeuCauSuaNoiDung{DisplayOrder: &neg}).KiemTra(); !errors.Is(err, ErrDisplayOrderInvalid) {
		t.Errorf("PATCH display_order âm: %v", err)
	}
}

func TestCategoryEditRequestValidatesWhatItMentions(t *testing.T) {
	blank, long, order, tooBig := "  ", strings.Repeat("x", MaCanBoToiDa+1), 3, ThuTuDanhMucToiDa+1
	if _, err := (ContentCategoryUpdate{Ten: &blank}).KiemTra(); !errors.Is(err, ErrTenDanhMucTrong) {
		t.Errorf("tên trống: %v", err)
	}
	if _, err := (ContentCategoryUpdate{ChaID: &long}).KiemTra(); !errors.Is(err, ErrMaCanBoQuaDai) {
		t.Errorf("cha quá dài: %v", err)
	}
	if _, err := (ContentCategoryUpdate{ThuTu: &tooBig}).KiemTra(); !errors.Is(err, ErrThuTuDanhMucNgoaiKhoang) {
		t.Errorf("thứ tự ngoài khoảng: %v", err)
	}
	name, root, hidden := "  Y tế ", " ", true
	got, err := ContentCategoryUpdate{Ten: &name, ChaID: &root, ThuTu: &order, Hidden: &hidden}.KiemTra()
	if err != nil || *got.Ten != "Y tế" || *got.ChaID != "" || *got.ThuTu != 3 || !*got.Hidden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got, _ := (ContentCategoryUpdate{}).KiemTra(); got.Ten != nil || got.ChaID != nil || got.ThuTu != nil || got.Hidden != nil {
		t.Errorf("không nhắc thì phải nil: %+v", got)
	}
}

func TestReparentCreatesCycle(t *testing.T) {
	// Tree: a → b → c (c's parent is b, b's parent is a). Moving a under c: c's chain is [c, b, a].
	if !ReparentCreatesCycle("a", []string{"c", "b", "a"}) {
		t.Error("a dưới con cháu của nó phải là vòng")
	}
	if !ReparentCreatesCycle("a", []string{"a"}) {
		t.Error("a làm cha chính nó phải là vòng")
	}
	if ReparentCreatesCycle("c", []string{"b", "a"}) {
		t.Error("c dưới b không phải vòng")
	}
}

func TestHiddenCategoryIsNoChipButItsItemsStillCountUpward(t *testing.T) {
	live := []DanhMucMiniApp{
		{ID: "root", ThuTu: 1},
		{ID: "hidden-child", ChaID: "root", Hidden: true},
		{ID: "under-hidden", ChaID: "hidden-child"},
		{ID: "hidden-root", Hidden: true},
		{ID: "child-of-hidden-root", ChaID: "hidden-root"},
		{ID: "plain", ThuTu: 2},
	}
	got := CategoriesWithPublishedItems(live, []string{"under-hidden", "child-of-hidden-root", "plain"})
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	// `root` stays a chip: choosing it lists the hidden child's items. Nothing hidden, nothing under a
	// hidden node, is a chip.
	if strings.Join(ids, ",") != "root,plain" {
		t.Errorf("chips = %v, muốn [root plain]", ids)
	}
}
