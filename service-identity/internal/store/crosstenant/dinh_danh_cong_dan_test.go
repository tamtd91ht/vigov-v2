package crosstenant

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Tests that need no database. They run everywhere, including on a machine with no
// VIGOV_TEST_DSN — which matters here, because the integration file next to this one SKIPS
// silently on such a machine and the package still prints `ok`.

// soGia is the agreed fake number (rule 3, invariant 5). No real number appears in this
// repository, not even in a test.
const soGia = "0900000000"

func TestTimHoacTaoTuChoiKhiKhongCoGiaoDich(t *testing.T) {
	// The identity row and the audit entry for the sign-in must succeed or fail together
	// (rule 6, invariant 3). Without a transaction from the caller there is nothing to join, so
	// this refuses rather than quietly opening one of its own.
	_, _, err := NewDinhDanhStore().TimHoacTao(context.Background(), nil, soGia)
	if !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("gọi không kèm giao dịch mà không bị từ chối, err = %v", err)
	}
}

func TestTimHoacTaoTuChoiSoRong(t *testing.T) {
	// nil transaction is safe here, and the assertion is still exact: the number is validated
	// BEFORE the transaction is looked at, so a blank number must produce ErrSoRong and not the
	// missing-transaction error. Accepting either would make this test green for the wrong
	// reason — it would keep passing if the validation were deleted outright.
	for ten, so := range map[string]string{
		"rỗng":         "",
		"chỉ dấu cách": "   ",
		"tab":          "\t",
	} {
		t.Run(ten, func(t *testing.T) {
			_, _, err := NewDinhDanhStore().TimHoacTao(context.Background(), nil, so)
			if !errors.Is(err, ErrSoRong) {
				t.Errorf("số %q không bị từ chối đúng lý do, err = %v", so, err)
			}
		})
	}
}

func TestTimHoacTaoTuChoiSoQuaDai(t *testing.T) {
	// An unbounded client-supplied string reaching a government database is a liability, and
	// this is the one table with no commune boundary to limit the damage.
	qua := strings.Repeat("9", doDaiSoToiDa+1)
	_, _, err := NewDinhDanhStore().TimHoacTao(context.Background(), nil, qua)
	if !errors.Is(err, ErrSoQuaDai) {
		t.Errorf("số dài %d ký tự không bị từ chối đúng lý do, err = %v", len(qua), err)
	}
}

func TestLoiKhongBaoGioChuaSoDienThoai(t *testing.T) {
	// An error travels into log lines, into monitoring, sometimes onto a screen. A phone number
	// riding along is a Decree 13/2023 breach written by whoever was debugging (rule 3).
	for _, so := range []string{soGia, strings.Repeat("9", doDaiSoToiDa+5)} {
		_, _, err := NewDinhDanhStore().TimHoacTao(context.Background(), nil, so)
		if err == nil {
			continue
		}
		if strings.Contains(err.Error(), so) {
			t.Errorf("thông điệp lỗi chứa số điện thoại: %v", err)
		}
	}
	for _, err := range []error{ErrSoRong, ErrSoQuaDai, ErrThieuGiaoDich} {
		if strings.ContainsAny(err.Error(), "0123456789") {
			t.Errorf("lỗi dựng sẵn chứa chữ số, dễ bị đọc nhầm là một số thật: %v", err)
		}
	}
}

func TestMaULIDDungHinhDang(t *testing.T) {
	// 26 characters is a CHECK constraint in the schema (dinh_danh_cong_dan_id_la_ulid), so a
	// generator that drifts off it fails every insert — and only against a real database, which
	// is the one place this suite cannot reach. Checking the shape here is what makes that
	// failure impossible rather than merely unlikely.
	id, err := maULID()
	if err != nil {
		t.Fatalf("maULID lỗi: %v", err)
	}
	if len(id) != 26 {
		t.Fatalf("ULID dài %d ký tự, phải là 26: %q", len(id), id)
	}
	for i, r := range id {
		if !strings.ContainsRune(chuCaiULID, r) {
			t.Errorf("ký tự %d (%q) không thuộc bảng chữ cái Crockford", i, r)
		}
	}
}

func TestMaULIDKhongLap(t *testing.T) {
	// 80 bits of randomness per value. A repeat inside one run means the random source is not
	// being read at all — and two citizens sharing one identity row is the failure ADR 0002
	// exists to prevent, seen from the other side.
	thay := make(map[string]bool, 2000)
	for range 2000 {
		id, err := maULID()
		if err != nil {
			t.Fatalf("maULID lỗi: %v", err)
		}
		if thay[id] {
			t.Fatalf("ULID lặp lại sau %d lần sinh", len(thay))
		}
		thay[id] = true
	}
}

func TestMaULIDMangDauThoiGian(t *testing.T) {
	// The first ten characters encode the 48-bit millisecond timestamp. Two values minted in
	// the same run share that prefix unless a millisecond ticked over, and neither may be the
	// all-zero prefix — which is what a generator that forgot the clock would produce.
	a, err := maULID()
	if err != nil {
		t.Fatal(err)
	}
	if a[:10] == "0000000000" {
		t.Error("mười ký tự đầu toàn 0 — dấu thời gian không được ghi vào")
	}
}

func TestChuCaiULIDKhongCoKyTuDeNhamLan(t *testing.T) {
	// Crockford's alphabet drops I, L, O and U so a transcribed identifier cannot become a
	// different valid one. An identifier read aloud at a counter is a real path in this system.
	if len(chuCaiULID) != 32 {
		t.Fatalf("bảng chữ cái có %d ký tự, phải là 32", len(chuCaiULID))
	}
	for _, r := range "ILOU" {
		if strings.ContainsRune(chuCaiULID, r) {
			t.Errorf("bảng chữ cái chứa %q — dễ đọc nhầm", r)
		}
	}
}
