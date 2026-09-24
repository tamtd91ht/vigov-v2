package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChuanHoaTuKhoaTimCanBoRongHoacToanKhoangTrangBiTuChoi(t *testing.T) {
	for _, tho := range []string{"", " ", "\t\n  ", " "} {
		if _, err := ChuanHoaTuKhoaTimCanBo(tho); !errors.Is(err, ErrThieuTuKhoa) {
			t.Errorf("ChuanHoaTuKhoaTimCanBo(%q) = %v, muốn ErrThieuTuKhoa", tho, err)
		}
	}
}

// THE LIMIT IS IN CHARACTERS. 150 letters of Vietnamese with diacritics is well over 200 BYTES, so
// a limit written with len() refuses it while a limit written with utf8.RuneCountInString accepts it.
//
// MUTATION THAT MUST TURN THIS RED: replace utf8.RuneCountInString(tu) with len(tu).
func TestChuanHoaTuKhoaTimCanBoDemKyTuKhongDemByte(t *testing.T) {
	// "Nguyễn Thị Hồng " is 16 characters; 150 = 9×16 + 6, so the cut lands on "Nguyễn" and the
	// sample ends on a letter rather than on a space the trim would remove.
	nam := string([]rune(strings.Repeat("Nguyễn Thị Hồng ", 10))[:150])
	if n := utf8.RuneCountInString(nam); n != 150 {
		t.Fatalf("mẫu dài %d ký tự, muốn 150", n)
	}
	if len(nam) <= TuKhoaTimCanBoToiDa {
		t.Fatalf("mẫu chỉ %d byte — không phân biệt được đếm byte với đếm ký tự", len(nam))
	}
	tu, err := ChuanHoaTuKhoaTimCanBo(nam)
	if err != nil {
		t.Fatalf("150 ký tự tiếng Việt bị từ chối (%d byte): %v — giới hạn đang đếm byte", len(nam), err)
	}
	if tu != nam {
		t.Errorf("từ khoá bị đổi nội dung")
	}

	vuaDu := strings.Repeat("ễ", TuKhoaTimCanBoToiDa)
	if _, err := ChuanHoaTuKhoaTimCanBo(vuaDu); err != nil {
		t.Errorf("đúng %d ký tự bị từ chối: %v", TuKhoaTimCanBoToiDa, err)
	}
	if _, err := ChuanHoaTuKhoaTimCanBo(vuaDu + "ễ"); !errors.Is(err, ErrTuKhoaQuaDai) {
		t.Errorf("%d ký tự được nhận, muốn ErrTuKhoaQuaDai (err = %v)", TuKhoaTimCanBoToiDa+1, err)
	}
}

func TestChuanHoaTuKhoaTimCanBoGopKhoangTrangVaKhongTraLaiDauVao(t *testing.T) {
	tu, err := ChuanHoaTuKhoaTimCanBo("  Nguyễn \t Văn   A ")
	if err != nil || tu != "Nguyễn Văn A" {
		t.Fatalf("= %q, %v; muốn %q", tu, err, "Nguyễn Văn A")
	}
	_, err = ChuanHoaTuKhoaTimCanBo(strings.Repeat("0900000000", 30))
	if err == nil || strings.Contains(err.Error(), "0900000000") {
		t.Errorf("lỗi quá dài phải có và không trích lại đầu vào: %v", err)
	}
}

func TestChuSoTimSoDienThoai(t *testing.T) {
	for tu, muon := range map[string]string{
		"0900 000 001":    "0900000001",
		"0900.000.001":    "0900000001",
		"(0900)-000-001":  "0900000001",
		"0900000001":      "0900000001",
		"Thôn 3":          "", // letters and digits: a position, not a number
		"Nguyễn Văn A":    "",
		"Kế toán 0900":    "",
		"- .":             "", // separators only: no digit, nothing to compare
		"+84 900 000 001": "84900000001",
	} {
		if got := ChuSoTimSoDienThoai(tu); got != muon {
			t.Errorf("ChuSoTimSoDienThoai(%q) = %q, muốn %q", tu, got, muon)
		}
	}
}
