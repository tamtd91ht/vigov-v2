package domain

import (
	"strings"
	"testing"
	"time"
)

// What these assert is the GENERATOR's half of "a code, once issued, is never issued again"
// (rule 7, invariant 3): that a code owes nothing to the clock, to the previous call, or to
// anything countable. The DATABASE's half — a soft-deleted code staying occupied — cannot be
// shown here at all, because it is a property of `UNIQUE (tenant_id, ma)` and not of this
// package; it is asserted against a real server in internal/store/ma_can_bo_pg_test.go.

const mocMauRFC3339 = "2026-09-22T10:00:00+07:00"

func mocMau(t *testing.T) time.Time {
	t.Helper()
	x, err := time.Parse(time.RFC3339, mocMauRFC3339)
	if err != nil {
		t.Fatalf("mốc thời gian mẫu hỏng: %v", err)
	}
	return x
}

// THE DEFECT THIS CATCHES: a generator that derives the code from the moment it runs. Two staff
// members created in the same request — an Excel import does exactly that (docs/ui-ux/
// 12-danh-ba-can-bo.md §6) — would then receive the same code, and the second INSERT would be
// refused by a unique key nobody expected to hit. Same instant in, two different codes out.
func TestHaiLanSinhCungMotMocKhongRaCungMotMa(t *testing.T) {
	x := mocMau(t)

	a, err := SinhMaCanBo(x)
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	b, err := SinhMaCanBo(x)
	if err != nil {
		t.Fatalf("lần 2: %v", err)
	}

	if a == b {
		t.Fatalf("hai lần sinh cùng một mốc ra CÙNG MỘT MÃ %q — mã đang suy ra từ đồng hồ "+
			"chứ không phải từ crypto/rand; nhập Excel nhiều người một lượt sẽ đụng khoá duy nhất", a)
	}
}

// The wider version of the same property, and the one that would catch a generator quietly stuck
// on a constant — a `rand.Read` whose error was swallowed, say. 2000 draws over 2^30 values:
// the birthday probability of even one repeat is under 0.2%, so a duplicate here is a defect,
// not luck.
func TestSinhNhieuLanKhongTrungNhau(t *testing.T) {
	x := mocMau(t)
	const lan = 2000

	thay := make(map[string]int, lan)
	for i := 0; i < lan; i++ {
		ma, err := SinhMaCanBo(x)
		if err != nil {
			t.Fatalf("lần %d: %v", i, err)
		}
		if truoc, co := thay[ma]; co {
			t.Fatalf("mã %q sinh lại ở lần %d (lần đầu %d) — bộ sinh không còn ngẫu nhiên",
				ma, i, truoc)
		}
		thay[ma] = i
	}
}

// The year is the caller's, not the machine's. Pinned because the argument is easy to ignore: a
// later edit reaching for time.Now() inside would compile, pass every other test in this file,
// and start putting the server's UTC year on codes minted in the first seven hours of a Vietnamese
// 1 January.
func TestNamLayTuThamSoChuKhongPhaiDongHoMay(t *testing.T) {
	ma, err := SinhMaCanBo(time.Date(2031, time.March, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sinh mã: %v", err)
	}
	if !strings.HasPrefix(ma, "CB-2031-") {
		t.Fatalf("mã = %q, muốn tiền tố CB-2031- — năm đang lấy từ đồng hồ máy", ma)
	}
}

// Shape, and the alphabet in particular. The four excluded letters are the whole reason for
// choosing Crockford base32 here: `ma` is read aloud in support conversations about a real
// administrative record, and an alphabet containing both I and 1, or both O and 0, lets a
// correctly transcribed code turn into a DIFFERENT VALID code.
func TestHinhDangVaBoChuCuaMa(t *testing.T) {
	x := mocMau(t)

	for i := 0; i < 500; i++ {
		ma, err := SinhMaCanBo(x)
		if err != nil {
			t.Fatalf("sinh mã: %v", err)
		}

		phan := strings.Split(ma, "-")
		if len(phan) != 3 {
			t.Fatalf("mã %q có %d phần, muốn 3 (CB-YYYY-XXXXXX)", ma, len(phan))
		}
		if phan[0] != "CB" {
			t.Fatalf("mã %q mở đầu bằng %q, muốn CB", ma, phan[0])
		}
		if phan[1] != "2026" {
			t.Fatalf("mã %q mang năm %q, muốn 2026", ma, phan[1])
		}
		if len(phan[2]) != soKyTuNgauNhien {
			t.Fatalf("phần ngẫu nhiên của %q dài %d, muốn %d", ma, len(phan[2]), soKyTuNgauNhien)
		}
		for _, c := range phan[2] {
			if !strings.ContainsRune(chuCaiMaCanBo, c) {
				t.Fatalf("mã %q chứa ký tự %q ngoài bộ chữ Crockford", ma, c)
			}
		}
	}
}

func TestBoChuKhongChuaBonChuDeDocNham(t *testing.T) {
	// Structural, and the cheapest of the lot: whatever else is edited, I, L, O and U stay out.
	// Stated here so a future edit that "restores the full base32 alphabet" is told why it is red.
	for _, c := range "ILOU" {
		if strings.ContainsRune(chuCaiMaCanBo, c) {
			t.Errorf("bộ chữ chứa %q — mã đọc qua điện thoại sẽ biến thành một mã HỢP LỆ KHÁC", c)
		}
	}
	if len(chuCaiMaCanBo) != 32 {
		t.Errorf("bộ chữ dài %d, muốn 32 — phép chia 5 bit mỗi ký tự không còn đúng",
			len(chuCaiMaCanBo))
	}
}
