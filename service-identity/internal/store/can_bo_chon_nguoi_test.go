package store

import (
	"errors"
	"strings"
	"testing"
)

// The picker read (can_bo_chon_nguoi.go), run against the SAME fake engine and the SAME dataset as
// the register (can_bo_danh_sach_test.go). The engine executes the predicate rather than recording
// it, so each exclusion below is proven by a row that really is absent — not by a string that
// happens to be in the statement.
//
// Of commune A's five rows, only CB-001 and CB-005 may be picked:
//
//	CB-002  co_tai_khoan = false   directory-only — could never act on the work
//	CB-003  dang_hoat_dong = false locked — retired or moved (#10)
//	CB-004  deleted_at set         a duplicate kept for the trail (rule 7)

func maChonNguoi(t *testing.T, b *banThuDS, xa, boPhan string) []string {
	t.Helper()
	ds, err := b.kho.ChonNguoi(ctxXa(xa), boPhan)
	if err != nil {
		t.Fatalf("ChonNguoi lỗi: %v", err)
	}
	var ma []string
	for _, cb := range ds {
		ma = append(ma, cb.Ma)
	}
	return ma
}

func TestChonNguoiChiNguoiCoTaiKhoanDangHoatDongChuaXoa(t *testing.T) {
	// MUTATIONS THAT MUST TURN THIS RED, one per clause of locChonNguoi: drop `AND co_tai_khoan`
	// and CB-002 appears; drop `AND dang_hoat_dong` and CB-003 appears; drop `deleted_at IS NULL`
	// and CB-004 appears.
	b := moBanThuDS(t)

	got := strings.Join(maChonNguoi(t, b, xaMau, ""), ",")
	// Ordered by ho_ten: "Nguyễn Văn A" before "Đỗ Văn E" in byte order, which is what the fake
	// compares. PostgreSQL's collation may order Vietnamese differently; the property here is the
	// SET and that the order is total, not the collation.
	if got != "CB-001,CB-005" {
		t.Fatalf("danh bạ chọn người = %s, muốn CB-001,CB-005", got)
	}
}

func TestChonNguoiChiThayXaCuaMinh(t *testing.T) {
	b := moBanThuDS(t)

	for _, ma := range maChonNguoi(t, b, xaMau, "") {
		if strings.HasPrefix(ma, "CB-9") {
			t.Fatalf("xã mẫu nhận %s của xã khác — RÒ RỈ GIỮA HAI XÃ", ma)
		}
	}
	// By name: "Bùi Văn G" (CB-901) before "Vũ Thị F" (CB-900) — the opposite of code order, which
	// is also what proves the ORDER BY is on ho_ten and not ma.
	if got := strings.Join(maChonNguoi(t, b, dsXaB, ""), ","); got != "CB-901,CB-900" {
		t.Fatalf("xã B nhận %s, muốn CB-901,CB-900", got)
	}

	// The commune is $1 and comes from the CONTEXT — the statement binds it, nothing else does.
	l := b.ghi.chua("FROM nguoi_dung")
	if l == nil {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") || l.args[0] != xaMau {
		t.Fatalf("câu lệnh không buộc tenant_id = $1 theo xã của ngữ cảnh: %q %v", l.sql, l.args)
	}
}

func TestChonNguoiLocTheoBoPhanLaThamSoRangBuoc(t *testing.T) {
	b := moBanThuDS(t)

	if got := strings.Join(maChonNguoi(t, b, xaMau, "bp-001"), ","); got != "CB-001" {
		// bp-001 holds CB-001 and the LOCKED CB-003; only the first may be picked.
		t.Fatalf("lọc bp-001 = %s, muốn CB-001", got)
	}
	l := b.ghi.tatCa()
	cuoi := l[len(l)-1]
	if strings.Contains(cuoi.sql, "bp-001") {
		t.Fatalf("mã bộ phận bị nối vào câu lệnh thay vì ràng buộc: %q", cuoi.sql)
	}
	if cuoi.args[1] != "bp-001" {
		t.Fatalf("$2 = %v, muốn bp-001", cuoi.args[1])
	}
}

func TestChonNguoiKhongChonCotNhayCam(t *testing.T) {
	// The column list IS the privacy guarantee of an AnyAuthenticated route: what is never selected
	// cannot be returned. Checked on the statement that reached the driver.
	b := moBanThuDS(t)
	maChonNguoi(t, b, xaMau, "")

	l := b.ghi.chua("FROM nguoi_dung")
	cot := l.sql[:strings.Index(l.sql, " FROM ")]
	for _, cam := range []string{"email", "dien_thoai_co_quan", "di_dong_ca_nhan", "mat_khau_hash", " id,", "vai_tro_id"} {
		if strings.Contains(cot, cam) {
			t.Errorf("danh bạ chọn người chọn cột %q: %s", cam, cot)
		}
	}
}

func TestChonNguoiVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// Commune A has two pickable people; a ceiling of one must REFUSE, and hand back no rows.
	b := moBanThuDS(t)

	ds, err := b.kho.chonNguoi(ctxXa(xaMau), "", 1)
	if !errors.Is(err, ErrQuaNhieuCanBoChonNguoi) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuCanBoChonNguoi", err)
	}
	if ds != nil {
		t.Fatalf("từ chối mà vẫn trả %d dòng — cắt bớt trá hình", len(ds))
	}
	// Exactly at the ceiling is NOT over it.
	if _, err := b.kho.chonNguoi(ctxXa(xaMau), "", 2); err != nil {
		t.Fatalf("đúng bằng trần mà bị từ chối: %v", err)
	}
}
