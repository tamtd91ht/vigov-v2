package store

import (
	"database/sql/driver"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// What this file defends: GET /api/v1/my-citizen-reports at the layer where the isolation is
// decided — the statement and its arguments.
//
// THE SAME WARNING AS phieu_phan_anh_cong_dan_test.go, AND IT MATTERS MORE FOR A LIST: the fake
// driver returns every fixture row whatever the WHERE clause says, so "citizen B's rows are not in
// the page" cannot be proved here by counting rows. It is proved by what reaches the database:
//
//	PROVED HERE   commune $1 from the CONTEXT · citizen $2 from the argument · soft-deleted excluded ·
//	              the citizen predicate is an EQUALITY, so a staff-booked row (NULL cong_dan_id) can
//	              never match · the restricted-field exclusion of the staff list is NOT applied (the
//	              by-code route does not apply it either) · status bound as a parameter · order is
//	              goc_dem_han DESC, id DESC · the cursor of page 1 becomes the bound anchor of page 2 ·
//	              an empty identity runs no statement · errors are wrapped and carry no identity.
//
//	NOT PROVED    what PostgreSQL does with it — phieu_cua_toi_danh_sach_pg_test.go, skipped without
//	              a DSN.

func trangCuaToi(t *testing.T, q url.Values) page.Request {
	t.Helper()
	yc, err := page.Parse(q, SapXepPhieuCuaToi)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return yc
}

func chayDanhSachCuaToi(t *testing.T, trangThai string) lenhGia {
	t.Helper()
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))
	if _, err := s.DanhSachCuaCongDan(ctxXa(xaThu), congDanThu, trangThai, trangCuaToi(t, url.Values{})); err != nil {
		t.Fatalf("DanhSachCuaCongDan: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	return k.lenh[0]
}

func TestDanhSachCuaCongDanLocCaHaiTrucVaLoaiDongDaXoa(t *testing.T) {
	l := chayDanhSachCuaToi(t, "")

	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	// ĐỘT BIẾN: bỏ `cong_dan_id = $2` khỏi DanhSachCuaCongDan và ca này ĐỎ — không thế thì một công
	// dân mở "Phản ánh của tôi" sẽ thấy cả sổ phản ánh của xã.
	if !strings.Contains(l.sql, "cong_dan_id = $2") {
		t.Errorf("câu lệnh KHÔNG lọc theo danh tính công dân: %q", l.sql)
	}
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
	if len(l.args) < 3 {
		t.Fatalf("tham số = %v, muốn ít nhất [xã, định danh công dân, limit]", l.args)
	}
	if l.args[0] != string(xaThu) {
		t.Errorf("$1 = %v, muốn xã trong context %q", l.args[0], xaThu)
	}
	if l.args[1] != congDanThu {
		t.Errorf("$2 = %v, muốn định danh công dân %q", l.args[1], congDanThu)
	}
	// Limit+1: DefaultLimit (20) rows asked for, one more fetched to know has_more.
	if l.args[len(l.args)-1] != int64(page.DefaultLimit+1) {
		t.Errorf("LIMIT = %v, muốn %d", l.args[len(l.args)-1], page.DefaultLimit+1)
	}
}

// TestDanhSachCuaCongDanPhieuNhapHoKhongBaoGioKhop — a staff-booked petition carries NULL in
// `cong_dan_id`, and `NULL = $2` is never TRUE. The property holds only while the predicate is a
// plain equality; this pins that shape against an edit that "helpfully" widens it.
func TestDanhSachCuaCongDanPhieuNhapHoKhongBaoGioKhop(t *testing.T) {
	l := chayDanhSachCuaToi(t, "")
	for _, cam := range []string{"cong_dan_id IS NULL", "COALESCE(cong_dan_id", "OR cong_dan_id"} {
		if strings.Contains(l.sql, cam) {
			t.Errorf("vị từ công dân bị nới (%q) — phiếu nhập hộ không có công dân sẽ lọt vào: %q", cam, l.sql)
		}
	}
}

// TestDanhSachCuaCongDanKhongLoaiLinhVucHanChe — the by-code route lets the filer read their own
// report ABOUT an officer; the list must agree, or the citizen could open a petition they cannot find.
func TestDanhSachCuaCongDanKhongLoaiLinhVucHanChe(t *testing.T) {
	if l := chayDanhSachCuaToi(t, ""); strings.Contains(l.sql, dieuKienHanChe) {
		t.Errorf("danh sách của công dân loại lĩnh vực hạn chế, trong khi tuyến theo mã không loại: %q", l.sql)
	}
}

func TestDanhSachCuaCongDanThuTuMoiNhatTruocCoPhaHoa(t *testing.T) {
	if l := chayDanhSachCuaToi(t, ""); !strings.Contains(l.sql, "ORDER BY goc_dem_han DESC, id DESC") {
		t.Errorf("thứ tự không phải mới nhất trước với khoá phá hoà: %q", l.sql)
	}
}

func TestDanhSachCuaCongDanTrangThaiLaThamSoRangBuoc(t *testing.T) {
	l := chayDanhSachCuaToi(t, "dang-xu-ly")
	if !strings.Contains(l.sql, "trang_thai = $3") {
		t.Fatalf("thiếu vị từ trạng thái: %q", l.sql)
	}
	if strings.Contains(l.sql, "dang-xu-ly") {
		t.Errorf("trạng thái nằm TRONG câu lệnh thay vì là tham số: %q", l.sql)
	}
	if len(l.args) < 3 || l.args[2] != "dang-xu-ly" {
		t.Errorf("$3 = %v, muốn trạng thái", l.args)
	}

	if l := chayDanhSachCuaToi(t, ""); strings.Contains(l.sql, "trang_thai = $") {
		t.Errorf("không yêu cầu mà vẫn lọc trạng thái: %q", l.sql)
	}
}

// TestDanhSachCuaCongDanXaVanDenTuContext — the same citizen id in two communes' contexts binds two
// different $1. A store that captured a commune at construction would bind the same one twice.
func TestDanhSachCuaCongDanXaVanDenTuContext(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))

	for _, xa := range []tenant.ID{xaThu, xaKhac} {
		if _, err := s.DanhSachCuaCongDan(ctxXa(xa), congDanThu, "", trangCuaToi(t, url.Values{})); err != nil {
			t.Fatal(err)
		}
	}
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}
	for i, l := range k.lenh {
		if len(l.args) < 2 {
			t.Fatalf("lời gọi %d có %d tham số: %v", i+1, len(l.args), l.args)
		}
	}
	if k.lenh[0].args[0] != string(xaThu) || k.lenh[1].args[0] != string(xaKhac) {
		t.Errorf("$1 = %v và %v — xã phải đi theo context", k.lenh[0].args[0], k.lenh[1].args[0])
	}
	if k.lenh[1].args[1] != congDanThu {
		t.Errorf("$2 = %v, muốn %q", k.lenh[1].args[1], congDanThu)
	}
}

// TestDanhSachCuaCongDanConTroQuaHaiTrang — page 1's cursor, fed back, becomes page 2's BOUND anchor
// (goc_dem_han, id) after the commune and the citizen, never text in the statement.
func TestDanhSachCuaCongDanConTroQuaHaiTrang(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		dongPhieu(nil),
		dongPhieu(map[string]driver.Value{"id": "pa-002", "ma_tra_cuu": "PA-8QLT-61VB-35ZN"}),
	}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSachCuaCongDan(ctxXa(xaThu), congDanThu, "", trangCuaToi(t, url.Values{"limit": {"1"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 1 || !kq.HasMore || kq.NextCursor == "" {
		t.Fatalf("trang 1: %d mục, has_more=%v, cursor=%q", len(kq.Items), kq.HasMore, kq.NextCursor)
	}

	if _, err := s.DanhSachCuaCongDan(ctxXa(xaThu), congDanThu, "",
		trangCuaToi(t, url.Values{"limit": {"1"}, "cursor": {kq.NextCursor}})); err != nil {
		t.Fatal(err)
	}
	l := k.lenh[1]
	if !strings.Contains(l.sql, "(goc_dem_han, id) < ($3, $4)") {
		t.Fatalf("trang 2 không neo theo mốc của trang 1: %q", l.sql)
	}
	if len(l.args) != 5 {
		t.Fatalf("tham số trang 2 = %v, muốn [xã, công dân, mốc, id, limit]", l.args)
	}
	if l.args[0] != string(xaThu) || l.args[1] != congDanThu {
		t.Errorf("trang 2 mất một trong hai trục: %v", l.args[:2])
	}
	if l.args[3] != "pa-001" {
		t.Errorf("id phá hoà = %v, muốn id dòng cuối trang 1", l.args[3])
	}
	if l.args[4] != int64(2) {
		t.Errorf("LIMIT = %v, muốn 2", l.args[4])
	}
}

func TestDanhSachCuaCongDanDinhDanhRongBiTuChoiTruocKhiChayLenh(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSachCuaCongDan(ctxXa(xaThu), "", "", trangCuaToi(t, url.Values{}))
	if !errors.Is(err, ErrThieuDinhDanhCongDan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuDinhDanhCongDan", err)
	}
	if len(k.lenh) != 0 {
		t.Fatalf("đã chạy %d câu lệnh với định danh rỗng", len(k.lenh))
	}
	if len(kq.Items) != 0 {
		t.Errorf("trả %d mục dù bị từ chối", len(kq.Items))
	}
}

func TestDanhSachCuaCongDanLoiDuocBocVaKhongMangDinhDanh(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: connection refused")}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.DanhSachCuaCongDan(ctxXa(xaThu), congDanThu, "", trangCuaToi(t, url.Values{}))
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("lỗi driver bị nuốt hoặc không bọc %%w: %v", err)
	}
	if strings.Contains(err.Error(), congDanThu) {
		t.Errorf("định danh công dân lọt vào thông điệp lỗi: %v", err)
	}
}
