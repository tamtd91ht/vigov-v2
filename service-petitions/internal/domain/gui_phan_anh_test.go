package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// The bounds a citizen's own submission is held to.
//
//	PROVED HERE   content is the ONE mandatory field · whitespace-only content is refused as empty
//	              rather than stored · every bound counts RUNES, so a Vietnamese report is not
//	              refused at a third of the length an English one is · the three optional fields
//	              accept empty · no error message echoes what was sent (rule 3, forbidden #3).
//
//	NOT PROVED    that the numbers are the right numbers. They are an assumption — no ADR and no
//	              screen specifies one — and the file says so.

func TestChuanHoaNoiDungBatBuocVaCatKhoangTrang(t *testing.T) {
	for ten, vao := range map[string]string{
		"rỗng hoàn toàn":   "",
		"chỉ khoảng trắng": "   \n\t  ",
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := ChuanHoaNoiDung(vao); !errors.Is(err, ErrNoiDungTrong) {
				t.Errorf("ChuanHoaNoiDung(%q) = %v, muốn ErrNoiDungTrong — "+
					"một phiếu rỗng vẫn được cấp mã tra cứu và một cam kết", vao, err)
			}
		})
	}

	got, err := ChuanHoaNoiDung("  Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.  ")
	if err != nil {
		t.Fatalf("ChuanHoaNoiDung: %v", err)
	}
	if got != "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn." {
		t.Errorf("không cắt khoảng trắng hai đầu: %q", got)
	}
}

// TestChuanHoaDemTheoKyTuChuKhongTheoBYTE is the assertion the bound exists for.
//
// A string of NoiDungToiDa Vietnamese characters is ~3× that many bytes. Written with len() the
// bound would refuse it, and the person who hits the refusal is a citizen writing in Vietnamese —
// the only kind this channel has.
//
// ĐỘT BIẾN: đổi utf8.RuneCountInString thành len ở ChuanHoaNoiDung và ca này ĐỎ ngay.
func TestChuanHoaDemTheoKyTuChuKhongTheoBYTE(t *testing.T) {
	dung := strings.Repeat("ế", NoiDungToiDa) // 3 bytes per rune
	if len(dung) <= NoiDungToiDa {
		t.Fatalf("mẫu thử không phục vụ được mục đích: %d byte cho %d ký tự", len(dung), NoiDungToiDa)
	}
	if _, err := ChuanHoaNoiDung(dung); err != nil {
		t.Errorf("từ chối một nội dung ĐÚNG %d ký tự vì đếm theo byte: %v", NoiDungToiDa, err)
	}

	qua := strings.Repeat("ế", NoiDungToiDa+1)
	if _, err := ChuanHoaNoiDung(qua); !errors.Is(err, ErrNoiDungQuaDai) {
		t.Errorf("ChuanHoaNoiDung(%d ký tự) = %v, muốn ErrNoiDungQuaDai", NoiDungToiDa+1, err)
	}
}

// TestChuanHoaBaTruongTuyChonNhanRong — content is the only mandatory field on this channel. The
// field code is not (nobody knows it yet, ADR 0028 decision E) and neither is the reporter
// (ADR 0008 admits an anonymous filing).
func TestChuanHoaBaTruongTuyChonNhanRong(t *testing.T) {
	for ten, f := range map[string]func(string) (string, error){
		"địa chỉ":       ChuanHoaDiaChi,
		"họ tên":        ChuanHoaHoTen,
		"số điện thoại": ChuanHoaDienThoai,
	} {
		t.Run(ten, func(t *testing.T) {
			got, err := f("  ")
			if err != nil {
				t.Errorf("từ chối giá trị rỗng: %v", err)
			}
			if got != "" {
				t.Errorf("giá trị rỗng không được cắt về \"\": %q", got)
			}
		})
	}
}

func TestChuanHoaBaTruongTuyChonCoTranDoDai(t *testing.T) {
	cases := map[string]struct {
		f    func(string) (string, error)
		tran int
		muon error
	}{
		"địa chỉ":       {ChuanHoaDiaChi, DiaChiToiDa, ErrDiaChiQuaDai},
		"họ tên":        {ChuanHoaHoTen, HoTenToiDa, ErrHoTenQuaDai},
		"số điện thoại": {ChuanHoaDienThoai, DienThoaiToiDa, ErrDienThoaiQuaDai},
	}
	for ten, c := range cases {
		t.Run(ten, func(t *testing.T) {
			if _, err := c.f(strings.Repeat("a", c.tran)); err != nil {
				t.Errorf("từ chối giá trị đúng trần %d: %v", c.tran, err)
			}
			if _, err := c.f(strings.Repeat("a", c.tran+1)); !errors.Is(err, c.muon) {
				t.Errorf("giá trị %d ký tự = %v, muốn %v", c.tran+1, err, c.muon)
			}
		})
	}
}

// TestLoiGuiPhanAnhKhongMangLaiThuDaGui is rule 3, forbidden #3.
//
// The refusal message reaches a citizen's screen AND a log line. A message quoting the report, the
// address or the number would put citizen personal data in both at once — and the values below are
// exactly the ones an implementer reaches for when "helpfully" naming what was wrong.
func TestLoiGuiPhanAnhKhongMangLaiThuDaGui(t *testing.T) {
	const (
		noiDung   = "Đống rác ở đầu ngõ nhà bà Nguyễn Thị B"
		hoTen     = "Nguyễn Văn An"
		dienThoai = "0900000000" // the agreed fake number (rule 3, invariant 5)
		diaChi    = "Đầu ngõ thôn Hà Lam"
	)
	// REPEATED BY RUNE COUNT, NOT BY len(): every one of these fixtures is Vietnamese, so len() is
	// two to three times the number of characters and the "too long" fixture would come out SHORT
	// of the bound — a test that passes because it never triggered the branch it names.
	dai := func(s string, n int) string { return strings.Repeat(s, n/utf8.RuneCountInString(s)+2) }

	for ten, err := range map[string]error{
		"nội dung quá dài": loiCua(ChuanHoaNoiDung(dai(noiDung, NoiDungToiDa))),
		"họ tên quá dài":   loiCua(ChuanHoaHoTen(dai(hoTen, HoTenToiDa))),
		"số quá dài":       loiCua(ChuanHoaDienThoai(dai(dienThoai, DienThoaiToiDa))),
		"địa chỉ quá dài":  loiCua(ChuanHoaDiaChi(dai(diaChi, DiaChiToiDa))),
	} {
		t.Run(ten, func(t *testing.T) {
			if err == nil {
				t.Fatal("không từ chối giá trị quá dài")
			}
			for _, bi := range []string{noiDung, hoTen, dienThoai, diaChi, "Nguyễn"} {
				if strings.Contains(err.Error(), bi) {
					t.Errorf("thông điệp lỗi nhắc lại thứ công dân đã gửi (%q): %v", bi, err)
				}
			}
			if !LaLoiGuiPhanAnh(err) {
				t.Errorf("LaLoiGuiPhanAnh(%v) = false — sẽ bị trả 500 thay vì 400", err)
			}
		})
	}
}

// TestLaLoiGuiPhanAnhKhongNhanLoiHeThong is the half that makes the function worth having: a
// failure that is NOT the sender's fault must not be answered 400.
func TestLaLoiGuiPhanAnhKhongNhanLoiHeThong(t *testing.T) {
	if LaLoiGuiPhanAnh(errors.New("pg: connection refused")) {
		t.Error("một lỗi hạ tầng bị coi là lỗi đầu vào — công dân sẽ được bảo đi sửa phiếu " +
			"trong khi máy chủ hỏng và không ai được báo")
	}
}

func loiCua(_ string, err error) error { return err }
