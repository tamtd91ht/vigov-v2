package domain

import (
	"errors"
	"strings"
	"testing"
)

// THE SLUG IS PERMANENT (rule 7, invariant 3): whatever SinhMaBoPhan answers the first time is the
// code the unit carries for ever, so every case here is a code somebody would otherwise be stuck with.
func TestSinhMaBoPhanTiengViet(t *testing.T) {
	for ten, muon := range map[string]string{
		"VĂN PHÒNG ĐẢNG ỦY":             "van-phong-dang-uy",
		"LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ":   "lanh-dao-uy-ban-nhan-dan-xa",
		"THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN": "thuong-truc-hoi-dong-nhan-dan",
		"THƯỜNG TRỰC UBMTTQ VIỆT NAM":   "thuong-truc-ubmttq-viet-nam",
		"Tổ Một cửa":                    "to-mot-cua",
		"  Phòng   Văn hoá – Xã hội  ":  "phong-van-hoa-xa-hoi",
		"Đội 3/Khối 2":                  "doi-3-khoi-2",
		"ẮẰẲẴẶ ẤẦẨẪẬ ẾỀỂỄỆ ỐỒỔỖỘ ỚỜỞỠỢ ỨỪỬỮỰ": "aaaaa-aaaaa-eeeee-ooooo-ooooo-uuuuu",
		"ÝỲỶỸỴ ÍÌỈĨỊ Đđ": "yyyyy-iiiii-dd",
	} {
		got, err := SinhMaBoPhan(ten)
		if err != nil {
			t.Errorf("%q: lỗi %v", ten, err)
			continue
		}
		if got != muon {
			t.Errorf("SinhMaBoPhan(%q) = %q, muốn %q", ten, got, muon)
		}
		if _, err := ChuanHoaMaBoPhan(got); err != nil {
			t.Errorf("mã sinh ra %q không qua được chính phép kiểm mã: %v", got, err)
		}
	}
}

// A NAME WITH NOTHING TO DERIVE FROM IS REFUSED, never given an empty or random code.
func TestSinhMaBoPhanTenKhongCoChuThiTuChoi(t *testing.T) {
	for _, ten := range []string{"", "—", "!!! ???", "   "} {
		if _, err := SinhMaBoPhan(ten); !errors.Is(err, ErrTenKhongSinhDuocMa) {
			t.Errorf("%q: lỗi = %v, muốn ErrTenKhongSinhDuocMa", ten, err)
		}
	}
}

// A LONG NAME IS CUT AT A WORD BOUNDARY INSIDE THE BOUND — never mid-word, never ending in '-'.
func TestSinhMaBoPhanDaiThiCatTheoTu(t *testing.T) {
	ten := strings.Repeat("THƯỜNG TRỰC ", 20)
	got, err := SinhMaBoPhan(ten)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > TranMaBoPhan || strings.HasSuffix(got, "-") {
		t.Fatalf("mã %q (%d byte) vượt trần hoặc kết thúc bằng '-'", got, len(got))
	}
	if !strings.HasSuffix(got, "thuong") && !strings.HasSuffix(got, "truc") {
		t.Errorf("mã bị cắt giữa một từ: %q", got)
	}
}

func TestMaBoPhanThuNLuonHopLeVaTrongTran(t *testing.T) {
	if got := MaBoPhanThuN("van-phong", 1); got != "van-phong" {
		t.Errorf("n=1 = %q", got)
	}
	if got := MaBoPhanThuN("van-phong", 3); got != "van-phong-3" {
		t.Errorf("n=3 = %q", got)
	}
	dai := strings.Repeat("a", TranMaBoPhan)
	got := MaBoPhanThuN(dai, 12)
	if len(got) > TranMaBoPhan || !strings.HasSuffix(got, "-12") {
		t.Errorf("hậu tố trên mã dài: %q (%d byte)", got, len(got))
	}
	if _, err := ChuanHoaMaBoPhan(got); err != nil {
		t.Errorf("ứng viên %q không hợp lệ: %v", got, err)
	}
}

// A CODE THE CLIENT TYPES IS VALIDATED, NEVER REPAIRED.
func TestChuanHoaMaBoPhanTuChoiKhongSua(t *testing.T) {
	for _, sai := range []string{"", "Van-Phong", "van phong", "-van", "van-", "van--phong", "văn-phòng", "van_phong"} {
		if _, err := ChuanHoaMaBoPhan(sai); err == nil {
			t.Errorf("%q được nhận — phải bị từ chối, không được sửa hộ", sai)
		}
	}
	if got, err := ChuanHoaMaBoPhan("  van-phong-2  "); err != nil || got != "van-phong-2" {
		t.Errorf("mã hợp lệ bị từ chối: %q, %v", got, err)
	}
	if _, err := ChuanHoaMaBoPhan(strings.Repeat("a", TranMaBoPhan+1)); !errors.Is(err, ErrMaBoPhanQuaDai) {
		t.Errorf("mã quá dài: %v", err)
	}
}

func TestChuanHoaTenBoPhan(t *testing.T) {
	if got, err := ChuanHoaTenBoPhan("  VĂN PHÒNG ĐẢNG ỦY "); err != nil || got != "VĂN PHÒNG ĐẢNG ỦY" {
		t.Errorf("= %q, %v", got, err)
	}
	// Case kept as typed — the upper-case convention is the commune's, not this service's.
	if got, _ := ChuanHoaTenBoPhan("Tổ một cửa"); got != "Tổ một cửa" {
		t.Errorf("tên bị đổi chữ hoa/thường: %q", got)
	}
	if _, err := ChuanHoaTenBoPhan("   "); !errors.Is(err, ErrThieuTenBoPhan) {
		t.Errorf("tên trống: %v", err)
	}
	if _, err := ChuanHoaTenBoPhan("TỔ\nMỘT CỬA"); !errors.Is(err, ErrTenBoPhanKyTuLa) {
		t.Errorf("tên có xuống dòng: %v", err)
	}
	// Counted in runes: 200 accented letters (≈ 600 bytes) are still within the bound.
	if _, err := ChuanHoaTenBoPhan(strings.Repeat("Ử", TranTenBoPhan)); err != nil {
		t.Errorf("200 ký tự có dấu bị từ chối: %v", err)
	}
	if _, err := ChuanHoaTenBoPhan(strings.Repeat("Ử", TranTenBoPhan+1)); !errors.Is(err, ErrTenBoPhanQuaDai) {
		t.Errorf("201 ký tự: %v", err)
	}
}

func TestKiemTraThuTuBoPhan(t *testing.T) {
	if KiemTraThuTuBoPhan(0) != nil || KiemTraThuTuBoPhan(TranThuTuBoPhan) != nil {
		t.Error("biên hợp lệ bị từ chối")
	}
	if !errors.Is(KiemTraThuTuBoPhan(-1), ErrThuTuBoPhanAm) {
		t.Error("thứ tự âm được nhận")
	}
	if !errors.Is(KiemTraThuTuBoPhan(TranThuTuBoPhan+1), ErrThuTuBoPhanQuaLon) {
		t.Error("thứ tự quá lớn được nhận")
	}
}
