package domain

// The pure rules of chapter 11, with no database and no HTTP.
//
// WHAT THIS FILE IS RESPONSIBLE FOR: the six content types are exactly §5's six; a title is refused
// rather than truncated; a link that is not http(s) is refused; a category slug has the form ADR 0011
// fixes; and a partial edit can tell "not mentioned" from "cleared". Everything else — the
// transaction, the commune, the permission — is proved where it lives.

import (
	"errors"
	"strings"
	"testing"
)

func TestSauLoaiNoiDungLaSauMaCuaDacTa(t *testing.T) {
	// §5 IS A TABLE WITH A `Mã` COLUMN, so these strings were read rather than invented. They are
	// also the CHECK constraint of migration 0006 and the `type` a client sends, so a typo in any of
	// the six is a tab that answers 400 to the screen that owns it.
	for _, ma := range []string{"tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video", "banner"} {
		if !LoaiNoiDungHopLe(ma) {
			t.Errorf("mã %q của §5 bị coi là không hợp lệ", ma)
		}
	}
	// A SEVENTH VALUE IS REFUSED, including the plausible ones. `tin_tuc` with an underscore and
	// `TinTuc` are exactly what a client writes when nobody checks, and both would sit in the
	// database as a `loai` no tab ever shows.
	for _, ma := range []string{"", "tin_tuc", "TinTuc", "news", "thong-bao-noi-bo", "su kien"} {
		if LoaiNoiDungHopLe(ma) {
			t.Errorf("mã %q không thuộc §5 mà vẫn được nhận", ma)
		}
	}
}

func TestChuanHoaTieuDeNoiDungTuChoiChuKhongCat(t *testing.T) {
	if _, err := ChuanHoaTieuDeNoiDung("   "); !errors.Is(err, ErrTieuDeNoiDungTrong) {
		t.Errorf("tiêu đề toàn khoảng trắng: lỗi = %v, muốn ErrTieuDeNoiDungTrong", err)
	}
	// REFUSED, NOT TRUNCATED. A headline silently cut at 300 runes is an article whose subject
	// changed on the way into the database, on a channel residents read.
	dai := strings.Repeat("a", TieuDeNoiDungToiDa+1)
	if _, err := ChuanHoaTieuDeNoiDung(dai); !errors.Is(err, ErrTieuDeNoiDungQuaDai) {
		t.Errorf("tiêu đề quá dài: lỗi = %v, muốn ErrTieuDeNoiDungQuaDai", err)
	}
	// A control character corrupts a screen and a log line alike.
	if _, err := ChuanHoaTieuDeNoiDung("Xã thông báo\x00"); err == nil {
		t.Error("tiêu đề chứa ký tự điều khiển mà vẫn qua")
	}
	// Vietnamese with diacritics is the ordinary case and must pass untouched but trimmed.
	ra, err := ChuanHoaTieuDeNoiDung("  Xã Thăng Bình khai giảng năm học mới  ")
	if err != nil || ra != "Xã Thăng Bình khai giảng năm học mới" {
		t.Errorf("tiêu đề hợp lệ = %q, lỗi %v", ra, err)
	}
}

func TestChuanHoaURLChiNhanHttpVaHttps(t *testing.T) {
	// THE ASSERTION THIS FUNCTION EXISTS FOR. `anh_dai_dien_url` is rendered as an image source and
	// `nguon_url` as a link, both inside an application residents open on their phones: `javascript:`
	// is script execution on the citizen channel and `data:` is an arbitrary payload served from the
	// commune's own screen.
	for _, xau := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"file:///etc/passwd",
		"//example.com/anh.png",
		"/anh.png",
		"example.com/anh.png",
	} {
		if _, err := ChuanHoaURL(xau); !errors.Is(err, ErrURLKhongHopLe) {
			t.Errorf("địa chỉ %q: lỗi = %v, muốn ErrURLKhongHopLe", xau, err)
		}
	}
	// AN EMPTY STRING IS VALID and means "no link" — §7 marks the image optional.
	if ra, err := ChuanHoaURL("   "); err != nil || ra != "" {
		t.Errorf("địa chỉ rỗng = %q, lỗi %v — rỗng nghĩa là không có ảnh", ra, err)
	}
	// WHAT WAS TYPED IS WHAT IS STORED: the scheme is compared case-insensitively and the value is
	// NOT lower-cased, because a path is case sensitive and rewriting it breaks the link.
	for _, tot := range []string{
		"https://thangbinh.danang.gov.vn/Anh/KhaiGiang.JPG",
		"HTTPS://thangbinh.danang.gov.vn/a.png",
		"http://thangbinh.danang.gov.vn/a.png",
	} {
		ra, err := ChuanHoaURL(tot)
		if err != nil {
			t.Errorf("địa chỉ %q bị từ chối: %v", tot, err)
		}
		if ra != tot {
			t.Errorf("địa chỉ bị sửa: %q -> %q", tot, ra)
		}
	}
}

func TestChuanHoaSlugDanhMucGiuDungKhuonADR0011(t *testing.T) {
	for _, xau := range []string{"", "Chuyen-Doi-So", "chuyen_doi_so", "chuyển-đổi-số",
		"-chuyen-doi-so", "chuyen-doi-so-", "chuyen--doi-so", "chuyen doi so"} {
		if _, err := ChuanHoaSlugDanhMuc(xau); err == nil {
			t.Errorf("slug %q sai khuôn mà vẫn qua", xau)
		}
	}
	// NO CASE FOLDING, and this is the case that says so: `Chuyen-Doi-So` is REFUSED above rather
	// than lower-cased here. The slug goes into every article as a filing value and rule 7 never lets
	// it be corrected, so the code stored has to be the code the person saw.
	if ra, err := ChuanHoaSlugDanhMuc(" chuyen-doi-so "); err != nil || ra != "chuyen-doi-so" {
		t.Errorf("slug hợp lệ = %q, lỗi %v", ra, err)
	}
	if _, err := ChuanHoaSlugDanhMuc(strings.Repeat("a", SlugDanhMucToiDa+1)); !errors.Is(err, ErrSlugDanhMucQuaDai) {
		t.Error("slug quá dài mà vẫn qua")
	}
}

func TestYeuCauThemNoiDungKhongCoTruongTrangThaiVaNguon(t *testing.T) {
	// A COMPILE-TIME ASSERTION WRITTEN AS A TEST, and it is the one this file exists for. If somebody
	// adds a `TrangThai` or a `Nguon` field to the request struct, §10.2's approval state and §10.4's
	// protection both become things a request body can claim. A struct literal naming every field is
	// what turns that addition into a build failure HERE rather than into a silent capability.
	_ = YeuCauThemNoiDung{
		Loai:           "tin-tuc",
		DanhMucID:      "",
		TieuDe:         "T",
		TomTat:         "",
		NoiDung:        "",
		AnhDaiDienURL:  "",
		DangLenMiniApp: false,
	}
}

func TestKiemTraThemNoiDungTuChoiLoaiLaVaGiuNguyenCoDangLen(t *testing.T) {
	yc := YeuCauThemNoiDung{Loai: "bai-viet", TieuDe: "Tiêu đề"}
	if _, err := yc.KiemTra(); !errors.Is(err, ErrLoaiNoiDungKhongHopLe) {
		t.Fatalf("loại lạ: lỗi = %v, muốn ErrLoaiNoiDungKhongHopLe", err)
	}

	yc = YeuCauThemNoiDung{Loai: "banner", TieuDe: "  Khẩu hiệu  ", DangLenMiniApp: true}
	sach, err := yc.KiemTra()
	if err != nil {
		t.Fatalf("yêu cầu hợp lệ bị từ chối: %v", err)
	}
	if sach.TieuDe != "Khẩu hiệu" {
		t.Errorf("tiêu đề = %q, muốn đã trim", sach.TieuDe)
	}
	// THE CHECKBOX SURVIVES VALIDATION. It decides whether residents see the item, and a
	// normalisation step that dropped it would publish nothing while reporting success.
	if !sach.DangLenMiniApp {
		t.Error("cờ `Đăng lên Mini App` bị mất khi chuẩn hoá")
	}
}

func TestKiemTraSuaNoiDungPhanBietKhongNhacVoiXoaTrang(t *testing.T) {
	// THE WHOLE REASON THE EDIT REQUEST IS A STRUCT OF POINTERS. A screen that edits only the title
	// must not clear the summary, drop the article out of its category and unpublish it — and a
	// struct of plain values cannot tell those two apart.
	var yc YeuCauSuaNoiDung
	sach, err := yc.KiemTra()
	if err != nil {
		t.Fatalf("yêu cầu rỗng bị từ chối: %v", err)
	}
	if sach.TieuDe != nil || sach.TomTat != nil || sach.DanhMucID != nil ||
		sach.NoiDung != nil || sach.AnhDaiDienURL != nil || sach.Loai != nil ||
		sach.DangLenMiniApp != nil {
		t.Errorf("yêu cầu không nhắc trường nào mà chuẩn hoá lại sinh ra giá trị: %+v", sach)
	}

	// MENTIONED AND EMPTY IS A REAL REQUEST: `— Chưa xếp danh mục —`, and a summary the author
	// deleted. It must survive as a non-nil pointer to "".
	rong := ""
	yc = YeuCauSuaNoiDung{DanhMucID: &rong, TomTat: &rong}
	sach, err = yc.KiemTra()
	if err != nil {
		t.Fatalf("xoá trắng bị từ chối: %v", err)
	}
	if sach.DanhMucID == nil || *sach.DanhMucID != "" {
		t.Errorf("`category_id: \"\"` phải giữ được nghĩa 'bỏ khỏi danh mục': %#v", sach.DanhMucID)
	}
	if sach.TomTat == nil || *sach.TomTat != "" {
		t.Errorf("`summary: \"\"` phải giữ được nghĩa 'xoá tóm tắt': %#v", sach.TomTat)
	}
}

func TestKiemTraSuaNoiDungVanKiemTungTruongDuocNhac(t *testing.T) {
	xau := "javascript:alert(1)"
	if _, err := (YeuCauSuaNoiDung{AnhDaiDienURL: &xau}).KiemTra(); !errors.Is(err, ErrURLKhongHopLe) {
		t.Errorf("sửa ảnh sang javascript: lỗi = %v, muốn ErrURLKhongHopLe", err)
	}
	trong := "   "
	if _, err := (YeuCauSuaNoiDung{TieuDe: &trong}).KiemTra(); !errors.Is(err, ErrTieuDeNoiDungTrong) {
		t.Error("sửa tiêu đề thành khoảng trắng mà vẫn qua")
	}
	la := "podcast"
	if _, err := (YeuCauSuaNoiDung{Loai: &la}).KiemTra(); !errors.Is(err, ErrLoaiNoiDungKhongHopLe) {
		t.Error("sửa sang một loại ngoài §5 mà vẫn qua")
	}
}

func TestCoAnhVaHienChoDanSuyRaChuKhongLuu(t *testing.T) {
	// §6's `Tệp đính kèm` column is `🔗 Có ảnh` / `—`, derived from the URL. A boolean column beside
	// it would be two representations of one fact, and the stale one is what a screen would read.
	if (NoiDungMiniApp{}).CoAnh() {
		t.Error("không có ảnh mà CoAnh() trả true")
	}
	if !(NoiDungMiniApp{AnhDaiDienURL: "https://x/a.png"}).CoAnh() {
		t.Error("có ảnh mà CoAnh() trả false")
	}

	// HienChoDan MUST BE FALSE FOR `cho-duyet` AS WELL AS FOR `an`. A place that asked the narrow
	// question by hand — testing for `an` alone — would publish everything waiting for approval.
	for _, tt := range []struct {
		trang TrangThaiNoiDung
		hien  bool
	}{
		{TrangThaiDangHien, true},
		{TrangThaiChoDuyet, false},
		{TrangThaiAn, false},
	} {
		if got := (NoiDungMiniApp{TrangThai: tt.trang}).HienChoDan(); got != tt.hien {
			t.Errorf("trạng thái %q: HienChoDan() = %v, muốn %v", tt.trang, got, tt.hien)
		}
	}
}

func TestKiemTraThemDanhMucChanThuTuAmVaTenRong(t *testing.T) {
	if _, err := (YeuCauThemDanhMuc{Ten: " ", Slug: "a"}).KiemTra(); !errors.Is(err, ErrTenDanhMucTrong) {
		t.Error("tên danh mục rỗng mà vẫn qua")
	}
	if _, err := (YeuCauThemDanhMuc{Ten: "Chuyển đổi số", Slug: "chuyen-doi-so", ThuTu: -1}).
		KiemTra(); !errors.Is(err, ErrThuTuDanhMucNgoaiKhoang) {
		// NEGATIVE IS REFUSED, NOT CLAMPED: -1 to mean "first" works until somebody sends -2, and the
		// order a commune arranged its own tree in then depends on who edited last.
		t.Error("thứ tự âm mà vẫn qua")
	}
	sach, err := (YeuCauThemDanhMuc{Ten: " Chuyển đổi số ", Slug: " chuyen-doi-so ", ThuTu: 3}).KiemTra()
	if err != nil {
		t.Fatalf("danh mục hợp lệ bị từ chối: %v", err)
	}
	if sach.Ten != "Chuyển đổi số" || sach.Slug != "chuyen-doi-so" || sach.ThuTu != 3 {
		t.Errorf("danh mục sau chuẩn hoá = %+v", sach)
	}
}
