package store

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The READ of task-status overrides, over the fake driver in driver_gia_test.go — NO PostgreSQL.
// What PostgreSQL does with the statements (the CHECK, the PK, the trigger) is
// nhan_trang_thai_nhiem_vu_pg_test.go; the upsert and the FOR UPDATE read run inside a transaction
// and are exercised through the use case in internal/app.

func dungKhoNhanTrangThai(k *khoGia) *NhanTrangThaiNhiemVuStore {
	return NewNhanTrangThaiNhiemVuStore(pkgstore.New(moKhoGia(k)))
}

func dongNhanTrangThai(ma, nhan string, thuTu int, boi string) map[string]driver.Value {
	return map[string]driver.Value{"ma": ma, "nhan": nhan, "thu_tu": int64(thuTu), "cap_nhat_boi": boi}
}

func TestNhanTrangThaiCauLenhBuocXa(t *testing.T) {
	k := &khoGia{}
	if _, err := dungKhoNhanTrangThai(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	// RULE 1, INVARIANTS 4 AND 5: the commune is $1, from the context.
	if !strings.Contains(l.sql, " FROM nhan_trang_thai_nhiem_vu WHERE tenant_id = $1 ") {
		t.Errorf("câu lệnh không lọc theo xã hoặc đọc nhầm bảng: %q", l.sql)
	}
	if l.args[0] != string(xaThu) {
		t.Fatalf("$1 = %v, muốn %q", l.args[0], xaThu)
	}
	// Ceiling PLUS ONE, so "more rows than codes" is detectable.
	if !strings.Contains(l.sql, "LIMIT $2") || l.args[1] != int64(TranNhanTrangThai+1) {
		t.Errorf("LIMIT sai: %q %v", l.sql, l.args)
	}
}

func TestNhanTrangThaiThamSoMotDiTheoContext(t *testing.T) {
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	k := &khoGia{}
	if _, err := dungKhoNhanTrangThai(k).DanhSach(ctxXa(xaKhac)); err != nil {
		t.Fatal(err)
	}
	if k.lenh[0].args[0] != string(xaKhac) {
		t.Errorf("$1 = %v, muốn %q", k.lenh[0].args[0], xaKhac)
	}
}

func TestNhanTrangThaiDocDungTungCot(t *testing.T) {
	// Three adjacent TEXT columns: the fake builds rows BY NAME from the SELECT list, so a Scan out
	// of step with cotNhanTrangThai comes back as wrong data.
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhanTrangThai("tam-dung", "Đang treo", 3, "CB-00123")}}
	ra, err := dungKhoNhanTrangThai(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatal(err)
	}
	if len(ra) != 1 || ra[0].Ma != "tam-dung" || ra[0].Nhan != "Đang treo" || ra[0].ThuTu != 3 ||
		ra[0].CapNhatBoi != "CB-00123" {
		t.Errorf("đọc sai cột: %+v", ra)
	}
}

func TestNhanTrangThaiRongLaLatRong(t *testing.T) {
	ra, err := dungKhoNhanTrangThai(&khoGia{}).DanhSach(ctxXa(xaThu))
	if err != nil || ra == nil || len(ra) != 0 {
		t.Fatalf("xã chưa có dòng: %v %v", ra, err)
	}
}

func TestNhanTrangThaiVuotTranThiTuChoi(t *testing.T) {
	var hang []map[string]driver.Value
	for i := 0; i < TranNhanTrangThai+1; i++ {
		hang = append(hang, dongNhanTrangThai("moi-giao", "X", 1, "CB-00123"))
	}
	ra, err := dungKhoNhanTrangThai(&khoGia{hangTheoCot: hang}).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuNhanTrangThai) || ra != nil {
		t.Fatalf("vượt trần: %v %v", ra, err)
	}
}

func TestNhanTrangThaiLoiDriverDuocBoc(t *testing.T) {
	goc := errors.New("driver hỏng")
	_, err := dungKhoNhanTrangThai(&khoGia{loi: goc}).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi không được bọc bằng %%w: %v", err)
	}
}

func TestNhanTrangThaiKhongCoXaThiPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("không có xã trong context mà vẫn đọc — một mặc định trên đường cách ly")
		}
	}()
	ra, err := dungKhoNhanTrangThai(&khoGia{}).DanhSach(t.Context())
	t.Fatalf("không panic: %v %v", ra, err)
}
