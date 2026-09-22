package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// --- SinhMatKhauTam ---------------------------------------------------------------------------

// THE ENTROPY CLAIM IS 80 BITS, and it rests on two numbers that live in two different files: 16
// characters here, an alphabet of 32 there. Nothing in the compiler holds them together, so a
// well-meant edit to `chuCaiMaCanBo` — dropping the digits to make codes "clearer", say — would
// silently cut the temporary password from 80 bits to 68 and nothing anywhere would say so.
func TestChuCaiSinhMatKhauDu32KyTu(t *testing.T) {
	if len(chuCaiMaCanBo) != 32 {
		t.Fatalf("bảng chữ cái có %d ký tự, không phải 32 — số bit của mật khẩu tạm KHÔNG còn là %d nữa",
			len(chuCaiMaCanBo), soKyTuMatKhauTam*5)
	}
	// Crockford's point: no character that turns into another when read aloud or transcribed.
	for _, c := range "ILOU" {
		if strings.ContainsRune(chuCaiMaCanBo, c) {
			t.Fatalf("bảng chữ cái chứa %q — đọc qua điện thoại sẽ ra một giá trị HỢP LỆ KHÁC", c)
		}
	}
}

func TestSinhMatKhauTamDungHinhDang(t *testing.T) {
	m, err := SinhMatKhauTam()
	if err != nil {
		t.Fatalf("SinhMatKhauTam lỗi: %v", err)
	}

	nhom := strings.Split(m, "-")
	if len(nhom) != soKyTuMatKhauTam/nhomMatKhauTam {
		t.Fatalf("có %d nhóm, muốn %d — %q", len(nhom), soKyTuMatKhauTam/nhomMatKhauTam, m)
	}
	for _, n := range nhom {
		if len(n) != nhomMatKhauTam {
			t.Fatalf("nhóm %q dài %d, muốn %d", n, len(n), nhomMatKhauTam)
		}
		for _, c := range n {
			if !strings.ContainsRune(chuCaiMaCanBo, c) {
				t.Fatalf("ký tự %q không thuộc bảng chữ cái đã chọn", c)
			}
		}
	}
}

// The value the administrator reads out must pass the same rule the person's own choice has to
// pass. If it did not, the system would issue a password it would refuse to accept.
func TestSinhMatKhauTamQuaDuocPhepKiemCuaChinhHeThong(t *testing.T) {
	m, err := SinhMatKhauTam()
	if err != nil {
		t.Fatalf("SinhMatKhauTam lỗi: %v", err)
	}
	if err := KiemTraMatKhauMoi(m); err != nil {
		t.Fatalf("mật khẩu tạm bị chính KiemTraMatKhauMoi từ chối: %v", err)
	}
}

// A GENERATOR THAT REPEATS IS THE FAILURE NOBODY SEES: every screen works, every reset succeeds,
// and two members of staff hold the same credential. 300 draws over 2^80 values collide with
// probability far below anything a flaky test would explain.
//
// MUTATION THAT MUST TURN THIS RED: seed the value from the clock, from a counter, or from the
// staff row.
func TestSinhMatKhauTamKhongLapLai(t *testing.T) {
	const soLan = 300
	thay := make(map[string]bool, soLan)
	for i := 0; i < soLan; i++ {
		m, err := SinhMatKhauTam()
		if err != nil {
			t.Fatalf("lần %d lỗi: %v", i, err)
		}
		if thay[m] {
			t.Fatal("hai lần sinh cho CÙNG một mật khẩu tạm")
		}
		thay[m] = true
	}
}

// Every position must actually vary. A loop that wrote the same byte into several slots — the
// classic off-by-one in a bit-extraction routine — would still pass the two shape tests above.
func TestSinhMatKhauTamMoiViTriDeuDoi(t *testing.T) {
	const soLan = 200
	khac := make([]map[rune]bool, 0)
	for i := 0; i < soLan; i++ {
		m, err := SinhMatKhauTam()
		if err != nil {
			t.Fatalf("lần %d lỗi: %v", i, err)
		}
		ky := []rune(strings.ReplaceAll(m, "-", ""))
		if len(khac) == 0 {
			khac = make([]map[rune]bool, len(ky))
			for j := range khac {
				khac[j] = map[rune]bool{}
			}
		}
		for j, c := range ky {
			khac[j][c] = true
		}
	}
	for j, tap := range khac {
		if len(tap) < 8 {
			t.Fatalf("vị trí %d chỉ nhận %d giá trị khác nhau trong %d lần sinh — vị trí này không ngẫu nhiên",
				j, len(tap), soLan)
		}
	}
}

// --- KiemTraMatKhauMoi ------------------------------------------------------------------------

func TestKiemTraMatKhauMoi(t *testing.T) {
	ca := []struct {
		ten  string
		vao  string
		muon error
	}{
		{"rỗng", "", ErrThieuMatKhau},
		{"ngắn hơn 12 ký tự", "ngan-qua", ErrMatKhauQuaNgan},
		{"đúng 11 ký tự", strings.Repeat("a", DaiMatKhauToiThieu-1), ErrMatKhauQuaNgan},
		{"đúng 12 ký tự", strings.Repeat("a", DaiMatKhauToiThieu), nil},
		{"quá dài", strings.Repeat("a", DaiMatKhauToiDa+1), ErrMatKhauQuaDai},
		{"UTF-8 hỏng", "mat-khau-" + string([]byte{0xff, 0xfe, 0xfd}), ErrMatKhauKhongDoc},
		// THE CASE THE BYTE COUNT GETS WRONG. Twelve Vietnamese characters are far more than twelve
		// bytes, and a byte count would accept this; the failure that matters is the mirror of it —
		// a shorter Vietnamese passphrase being ACCEPTED because its bytes add up.
		{"tiếng Việt đủ 12 ký tự", "mậtkhẩucủatô", nil},
		{"tiếng Việt thiếu 1 ký tự", "mậtkhẩucủat", ErrMatKhauQuaNgan},
	}
	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			err := KiemTraMatKhauMoi(c.vao)
			if !errors.Is(err, c.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.muon)
			}
		})
	}
}

// Eleven Vietnamese characters is eleven characters however many bytes it occupies — the fixture
// above only means something if the counts really are what this asserts.
func TestKiemTraMatKhauMoiDemTheoKyTuChuKhongTheoByte(t *testing.T) {
	const vao = "mậtkhẩucủat"
	if utf8.RuneCountInString(vao) >= DaiMatKhauToiThieu {
		t.Fatalf("mẫu thử có %d ký tự — không còn kiểm được điều cần kiểm", utf8.RuneCountInString(vao))
	}
	if len(vao) <= DaiMatKhauToiThieu {
		t.Fatalf("mẫu thử chỉ có %d byte — phép đếm theo byte sẽ cho cùng kết quả, ca này vô nghĩa", len(vao))
	}
}

// IT MUST NOT NORMALISE. A password with a leading or trailing space is a legitimate password and
// is stored exactly as typed; trimming it would silently change the credential, and the person's
// very next sign-in would fail with nothing to explain it.
//
// THE SIGNATURE IS THE REAL GUARANTEE — this function returns only an error, so there is no
// normalised value for a caller to accidentally prefer. This case is here so a future "ChuanHoa"
// rewrite has something to break.
func TestKiemTraMatKhauMoiKhongCatKhoangTrang(t *testing.T) {
	vao := "  " + strings.Repeat("a", DaiMatKhauToiThieu) + "  "
	if err := KiemTraMatKhauMoi(vao); err != nil {
		t.Fatalf("mật khẩu có khoảng trắng hai đầu bị từ chối: %v", err)
	}
	// And the short-with-spaces case must still be refused on its real length, not on a trimmed one.
	if err := KiemTraMatKhauMoi("  abc  "); !errors.Is(err, ErrMatKhauQuaNgan) {
		t.Fatalf("lỗi = %v, muốn ErrMatKhauQuaNgan", err)
	}
}

// No refusal may quote what it refused: these sentences are returned to a client and written to a
// log, and the value is a credential (rule 3, forbidden #3; rule 8).
func TestLoiMatKhauKhongChuaGiaTriBiTuChoi(t *testing.T) {
	const biMat = "mat-khau-that-KHONG-DUOC-LO"
	for _, vao := range []string{biMat[:5], biMat, biMat + strings.Repeat("x", DaiMatKhauToiDa)} {
		if err := KiemTraMatKhauMoi(vao); err != nil && strings.Contains(err.Error(), vao[:4]) {
			t.Fatalf("thông báo lỗi nhắc lại giá trị đã gửi lên: %v", err)
		}
	}
}
