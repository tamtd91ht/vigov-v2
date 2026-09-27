package store

import (
	"errors"
	"strings"
	"testing"
)

// The public reads (noi_dung_cong_khai.go), over the SAME fake driver as the staff register. It proves
// the statement: commune bound to $1 from the context, soft-deleted rows excluded, the state bound to
// `dang-hien`, no body on the page. That the database actually returns only published rows of one
// commune is the pg suite's (noi_dung_cong_khai_pg_test.go).

func TestDanhSachCongKhaiChiDangHienBuocXa(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	kq, err := kho.DanhSachCongKhai(ctx, trangDauND(t))
	if err != nil {
		t.Fatalf("đọc trang công khai lỗi: %v", err)
	}
	if len(kq.Items) != 1 || kq.Items[0].NoiDung != "" {
		t.Fatalf("trang công khai = %+v — một dòng, không mang toàn văn", kq.Items)
	}
	l := k.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: drop the state clause (every draft is published), drop the
	// soft-delete clause, or bind the state to anything but `dang-hien`.
	for _, muon := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2",
		"ORDER BY tao_luc DESC, id DESC"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu đọc trang công khai thiếu %q: %s", muon, l.sql)
		}
	}
	if l.args[0] != string(xaMotND) || l.args[1] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã của ngữ cảnh, dang-hien, …]", l.args)
	}
	if cot := l.sql[:strings.Index(l.sql, " FROM ")]; strings.Contains(cot, "noi_dung") {
		t.Errorf("trang công khai chọn cả `noi_dung`: %s", cot)
	}
}

func TestCongKhaiTheoIDChiDangHienBuocXa(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	n, err := kho.CongKhaiTheoID(ctx, "nd-001")
	if err != nil {
		t.Fatalf("đọc chi tiết công khai lỗi: %v", err)
	}
	if n.NoiDung != "<p>Toàn văn</p>" {
		t.Fatalf("toàn văn = %q", n.NoiDung)
	}
	l := k.lenh[0]
	for _, muon := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL", "trang_thai = $3"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu đọc chi tiết công khai thiếu %q: %s", muon, l.sql)
		}
	}
	if l.args[0] != string(xaMotND) || l.args[1] != "nd-001" || l.args[2] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã, nd-001, dang-hien]", l.args)
	}
}

func TestCongKhaiTheoIDKhongCoDongThiBaoKhongTonTai(t *testing.T) {
	// A draft, another commune's id and no id at all are ONE answer: the predicate returns no row.
	kho, ctx := khoNoiDung(t, &khoNDGia{})
	if _, err := kho.CongKhaiTheoID(ctx, "nd-ban-nhap"); !errors.Is(err, ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
}
