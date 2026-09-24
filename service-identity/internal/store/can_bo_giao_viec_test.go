package store

import (
	"sort"
	"strings"
	"testing"
)

// The assignable-code read (can_bo_giao_viec.go), run against the SAME fake engine and the SAME
// dataset as the picker (can_bo_chon_nguoi_test.go). The engine executes the predicate, so each
// exclusion below is proven by a row that really is absent.
//
// Of commune A's five rows, only CB-001 and CB-005 are assignable — EXACTLY the picker's set:
//
//	CB-002  co_tai_khoan = false    directory-only
//	CB-003  dang_hoat_dong = false  locked
//	CB-004  deleted_at set          soft deleted
//
// Commune B holds CB-900 and CB-901, both assignable THERE.

func maGiaoViec(t *testing.T, b *banThuDS, xa string, hoi []string) []string {
	t.Helper()
	ma, err := b.kho.GiaoViecDuoc(ctxXa(xa), hoi)
	if err != nil {
		t.Fatalf("GiaoViecDuoc lỗi: %v", err)
	}
	sort.Strings(ma)
	return ma
}

func TestGiaoViecChiNguoiCoTaiKhoanDangHoatDongChuaXoa(t *testing.T) {
	// MUTATIONS THAT MUST TURN THIS RED, one per clause of locChonNguoi — the same three the picker
	// test names, because it is the same constant.
	b := moBanThuDS(t)

	got := strings.Join(maGiaoViec(t, b, xaMau, []string{"CB-001", "CB-002", "CB-003", "CB-004", "CB-005"}), ",")
	if got != "CB-001,CB-005" {
		t.Fatalf("mã giao việc được = %s, muốn CB-001,CB-005 (khoá / không tài khoản / đã xoá phải vắng)", got)
	}
}

// THE CONTRACT SAYS THE TWO PREDICATES MUST NEVER DIVERGE. Asked for every code of the commune, this
// read must answer exactly the set the picker lists — pinned against the picker itself, not against
// a copy of its expected output.
func TestGiaoViecTrungKhopDanhBaChonNguoi(t *testing.T) {
	b := moBanThuDS(t)

	chon := maChonNguoi(t, b, xaMau, "")
	sort.Strings(chon)
	giao := maGiaoViec(t, b, xaMau, []string{"CB-001", "CB-002", "CB-003", "CB-004", "CB-005"})
	if strings.Join(chon, ",") != strings.Join(giao, ",") {
		t.Fatalf("ô chọn người = %v nhưng kiểm giao việc = %v — HAI VỊ TỪ ĐÃ LỆCH NHAU", chon, giao)
	}
}

// ĐÂY LÀ CA CỦA ĐỘT BIẾN: bỏ `tenant_id = $1` thì mã của xã B lọt sang xã A.
func TestGiaoViecKhongVuotSangXaKhac(t *testing.T) {
	b := moBanThuDS(t)

	got := maGiaoViec(t, b, xaMau, []string{"CB-001", "CB-900", "CB-901"})
	if strings.Join(got, ",") != "CB-001" {
		t.Fatalf("xã mẫu nhận %v, muốn chỉ CB-001 — mã của xã khác phải VẮNG MẶT (luật 1)", got)
	}

	l := b.ghi.chua("FROM nguoi_dung")
	if l == nil {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") || l.args[0] != xaMau {
		t.Fatalf("câu lệnh không buộc tenant_id = $1 theo xã của ngữ cảnh: %q %v", l.sql, l.args)
	}
}

// The codes are a BOUND text[], never spliced into the statement.
func TestGiaoViecMaLaThamSoRangBuoc(t *testing.T) {
	b := moBanThuDS(t)
	maGiaoViec(t, b, xaMau, []string{"CB-001"})

	l := b.ghi.chua("FROM nguoi_dung")
	if strings.Contains(l.sql, "CB-001") {
		t.Fatalf("mã bị nối vào câu lệnh thay vì ràng buộc: %q", l.sql)
	}
	if !strings.Contains(l.sql, "ma = ANY($2)") {
		t.Fatalf("thiếu `ma = ANY($2)`: %q", l.sql)
	}
	cot := l.sql[:strings.Index(l.sql, " FROM ")]
	if strings.TrimSpace(strings.TrimPrefix(cot, "SELECT")) != "ma" {
		t.Errorf("chọn thêm cột ngoài `ma`: %s", cot)
	}
}

// Danh sách rỗng KHÔNG chạm driver và trả rỗng — không bao giờ là "mọi người giao việc được".
func TestGiaoViecDanhSachRongKhongDocGi(t *testing.T) {
	b := moBanThuDS(t)

	if got := maGiaoViec(t, b, xaMau, nil); len(got) != 0 {
		t.Fatalf("danh sách rỗng trả %v", got)
	}
	if n := len(b.ghi.tatCa()); n != 0 {
		t.Fatalf("danh sách rỗng vẫn chạy %d câu lệnh", n)
	}
}
