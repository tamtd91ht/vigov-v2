package ulid

import (
	"strings"
	"testing"
)

// WHAT IS WORTH TESTING HERE, and what is not. The encoding is arithmetic and a test of it would
// restate the code. What is worth pinning is the SHAPE the rest of the system depends on — the
// three properties a drifting copy would break silently:
//
//  1. exactly 26 characters. tenant.ID.Valid() already checks a length of 26 for the commune id,
//     and callers assume the same of a row id;
//  2. every character inside Crockford's alphabet. I, L, O and U must never appear: their whole
//     purpose is that a transcribed id cannot become a different VALID id;
//  3. two calls differ. A generator that returned a constant would pass 1 and 2 and produce a
//     unique-key violation on the second row of every table.

func TestMoiDungDoVaDungBangChuCai(t *testing.T) {
	ma, err := Moi()
	if err != nil {
		t.Fatalf("Moi: %v", err)
	}
	if len(ma) != Do {
		t.Fatalf("độ dài = %d, muốn %d — mã %q", len(ma), Do, ma)
	}
	for i, r := range ma {
		if !strings.ContainsRune(chuCai, r) {
			t.Errorf("ký tự %q ở vị trí %d nằm ngoài bảng chữ cái Crockford", r, i)
		}
	}
	// Stated as its own assertion rather than left to the alphabet check: these four letters are
	// the entire reason Crockford's variant was chosen over plain base32, and somebody "simplifying"
	// the constant to the standard alphabet would still pass the loop above.
	if strings.ContainsAny(ma, "ILOU") {
		t.Errorf("mã %q chứa I/L/O/U — chép tay một mã như thế ra được một mã KHÁC vẫn hợp lệ", ma)
	}
}

func TestMoiKhongLapLai(t *testing.T) {
	// 1000 in a tight loop lands many calls inside ONE millisecond, so the timestamp prefix is
	// identical and only the 80 random bits separate them. That is the case that actually matters:
	// a generator using a weak or missing random source passes a slow loop and collides here.
	const n = 1000
	thay := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		ma, err := Moi()
		if err != nil {
			t.Fatalf("Moi lần %d: %v", i, err)
		}
		if _, trung := thay[ma]; trung {
			t.Fatalf("mã %q lặp lại ở lần %d — hai dòng sẽ đụng khoá chính", ma, i)
		}
		thay[ma] = struct{}{}
	}
}

func TestMoiTangDanTheoThoiGian(t *testing.T) {
	// The timestamp prefix is what makes a ULID sort by creation order, which is what an index on
	// the id is worth anything for. A generator that dropped the prefix would still be unique and
	// still be 26 characters — and every range scan built on id order would silently return rows
	// in random order.
	truoc, err := Moi()
	if err != nil {
		t.Fatalf("Moi: %v", err)
	}
	// The prefix has millisecond resolution, so two calls in the same millisecond are allowed to
	// come back in either order. Comparing the TIMESTAMP HALF only — the first 10 characters — is
	// what makes this assertion about the property rather than about luck.
	sau, err := Moi()
	if err != nil {
		t.Fatalf("Moi: %v", err)
	}
	if sau[:10] < truoc[:10] {
		t.Errorf("phần thời gian đi lùi: %q rồi %q", truoc[:10], sau[:10])
	}
}
