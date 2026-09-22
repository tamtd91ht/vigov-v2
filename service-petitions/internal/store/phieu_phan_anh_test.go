package store

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// What this file proves, and what it does not — the same split driver_gia_test.go states, kept
// here because the petition register is where a wrong answer costs the most.
//
//	PROVED   the commune reaches the statement as $1 and comes from the CONTEXT · the
//	         soft-delete predicate is in the statement · the positional Scan lines up with
//	         cotPhieu BY NAME, including the two adjacent TIMESTAMPTZ columns whose NULLs mean
//	         opposite things · a NULL deadline arrives as a zero time.Time and NOT as anything
//	         else · no row is ErrPhieuKhongTonTai and not a zero-valued petition · a driver
//	         failure is wrapped and the lookup code is NOT in the message.
//
//	NOT PROVED   anything PostgreSQL does. The fake builds each row from the column list the
//	             store itself handed it, so a column that does not exist in the real table passes
//	             here without a murmur. Nothing here touches hash partition routing, the CHECK
//	             constraints of migration 0004, or the archival trigger — including the two that
//	             matter most, `phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan` and the seven-day
//	             window. Those need a DSN.

const maThu = "PA-4K7M-92XR-BTVD"

var (
	mocGui   = time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)
	mocVaoSo = time.Date(2026, 9, 9, 7, 20, 3, 0, time.UTC)
	mocNhan  = time.Date(2026, 9, 9, 15, 20, 0, 0, time.UTC)
	mocXong  = time.Date(2026, 9, 16, 9, 20, 0, 0, time.UTC)
	mocTran  = time.Date(2026, 9, 10, 11, 5, 0, 0, time.UTC)
)

// dongPhieu is one row of `phieu_phan_anh` as the driver hands it back.
//
// EVERY VALUE IS DISTINCT AND OF THE RIGHT TYPE, so a Scan wired to the wrong position comes back
// as visibly wrong data rather than as a zero that looks plausible. The two deadline columns
// carry two DIFFERENT instants for exactly that reason: equal ones would make a swap invisible.
func dongPhieu(sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":                   "pa-001",
		"ma_tra_cuu":           maThu,
		"kenh_tiep_nhan":       "zalo-mini-app",
		"cong_dan_id":          "cd-001",
		"noi_dung":             "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		"linh_vuc":             "rac-thai",
		"dia_chi":              "Đầu ngõ thôn Hà Lam",
		"thon_id":              "thon-001",
		"lat":                  15.7654,
		"lng":                  108.3211,
		"nguoi_gui_ho_ten":     "Nguyễn Văn An",
		"nguoi_gui_dien_thoai": "0900000000", // số giả đã thống nhất (luật 3 bất biến 5)
		"an_danh":              false,
		"trang_thai":           "dang-phan-loai",
		"bo_phan_id":           "bp-001",
		"can_bo_xu_ly_id":      "nd-001",
		"goc_dem_han":          mocGui,
		"vao_so_luc":           mocVaoSo,
		"han_tiep_nhan":        mocNhan,
		"han_xu_ly_xong":       mocXong,
		// A THIRD INSTANT, DISTINCT FROM THE OTHER TWO on purpose: `han_phan_loai` sits between them
		// in cotPhieu and its NULL means the same as `han_tiep_nhan`'s and the OPPOSITE of
		// `han_xu_ly_xong`'s. Equal fixtures would make a positional swap of any pair invisible.
		"han_phan_loai":  mocTran,
		"phan_loai_luc":  time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC),
		"xu_ly_xong_luc": nil,
		"dong_luc":       nil,
		// Empty rather than a sentence: nothing in this file asserts on the result text, and a
		// fixture holding free text about a case is a fixture somebody copies into a log line.
		"ket_qua_xu_ly":  nil,
		"hien_cong_khai": false,
		"so_lan_mo_lai":  int64(0),
		// Only cotPhieuCoTaoLuc asks for it; the map is shared by both shapes, and a value the SELECT
		// did not ask for is simply not read.
		"tao_luc": mocVaoSo,
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

func TestTheoMaTraCuuBuocXaVaLoaiDongDaXoa(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	if _, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu); err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}

	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]

	// RULE 1, INVARIANTS 4 AND 5. The commune is not a parameter of TheoMaTraCuu and cannot
	// become one: it arrives in the context and Scoped.Query binds it to $1.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[0] != string(xaThu) || l.args[1] != maThu {
		t.Errorf("tham số = %v, muốn [xã, mã tra cứu]", l.args)
	}
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always. A
	// petition removed from the register must not come back through a lookup code a citizen
	// still holds.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
}

func TestTheoMaTraCuuXaTrongContextQuyetDinhChuKhongPhaiThamSo(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	// THE SAME STORE, TWO COMMUNES, TWO CONTEXTS. A store that had captured a commune at
	// construction — or taken one as an argument — would send the same $1 twice, and no test of a
	// single commune could tell.
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TheoMaTraCuu(ctxXa(xaKhac), maThu); err != nil {
		t.Fatal(err)
	}
	if k.lenh[0].args[0] == k.lenh[1].args[0] {
		t.Fatalf("hai ngữ cảnh khác xã lại gửi cùng $1 = %v", k.lenh[0].args[0])
	}
}

// TestTheoMaTraCuuScanKhopVoiCotPhieu is the one that catches a swap between the two deadline
// columns — the single most expensive off-by-one in this service, because it silently turns a
// staff-booked petition into an unclassified one and vice versa, and every report then excludes
// the wrong rows.
func TestTheoMaTraCuuScanKhopVoiCotPhieu(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}

	doiBang(t, "ID", p.ID, "pa-001")
	doiBang(t, "MaTraCuu", p.MaTraCuu, maThu)
	doiBang(t, "Kenh", string(p.Kenh), "zalo-mini-app")
	doiBang(t, "CongDanID", p.CongDanID, "cd-001")
	doiBang(t, "LinhVuc", p.LinhVuc, "rac-thai")
	doiBang(t, "DiaChi", p.DiaChi, "Đầu ngõ thôn Hà Lam")
	doiBang(t, "ThonID", p.ThonID, "thon-001")
	doiBang(t, "TrangThai", string(p.TrangThai), "dang-phan-loai")
	doiBang(t, "BoPhanID", p.BoPhanID, "bp-001")
	doiBang(t, "CanBoXuLyID", p.CanBoXuLyID, "nd-001")

	if p.Lat == nil || *p.Lat != 15.7654 || p.Lng == nil || *p.Lng != 108.3211 {
		t.Errorf("toạ độ = %v/%v — lat và lng là hai cột NUMERIC cạnh nhau, hoán đổi không báo lỗi", p.Lat, p.Lng)
	}

	// THE FOUR ADJACENT TIMESTAMPTZ COLUMNS, each a different instant on purpose.
	if !p.GocDemHan.Equal(mocGui) {
		t.Errorf("GocDemHan = %v, muốn %v", p.GocDemHan, mocGui)
	}
	if !p.VaoSoLuc.Equal(mocVaoSo) {
		t.Errorf("VaoSoLuc = %v, muốn %v", p.VaoSoLuc, mocVaoSo)
	}
	if !p.HanTiepNhan.Equal(mocNhan) {
		t.Errorf("HanTiepNhan = %v, muốn %v — hai cột hạn nằm cạnh nhau và NULL của chúng NGƯỢC NGHĨA", p.HanTiepNhan, mocNhan)
	}
	if !p.HanXuLyXong.Equal(mocXong) {
		t.Errorf("HanXuLyXong = %v, muốn %v", p.HanXuLyXong, mocXong)
	}
}

// TestTheoMaTraCuuHaiLoaiNULLDocRaMocRong asserts that a NULL deadline arrives as the zero
// time.Time — not as `now`, not as an error, not as a value from the neighbouring column.
func TestTheoMaTraCuuHaiLoaiNULLDocRaMocRong(t *testing.T) {
	t.Run("nhập hộ: han_tiep_nhan NULL là KHÔNG ÁP DỤNG", func(t *testing.T) {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
			"kenh_tiep_nhan": "can-bo-nhap-ho",
			"han_tiep_nhan":  nil,
			"phan_loai_luc":  nil,
		})}}
		s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

		p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
		if err != nil {
			t.Fatal(err)
		}
		if !p.HanTiepNhanKhongApDung() {
			t.Errorf("HanTiepNhan = %v, muốn mốc rỗng", p.HanTiepNhan)
		}
		if p.ChuaChotHanXuLy() {
			t.Error("HanXuLyXong bị đọc thành rỗng — phiếu nhập hộ chốt cả hai hạn lúc vào sổ")
		}
	})

	t.Run("chưa phân loại: han_xu_ly_xong NULL là CHƯA CÓ", func(t *testing.T) {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
			"trang_thai":     "da-tiep-nhan",
			"linh_vuc":       nil,
			"han_xu_ly_xong": nil,
			"phan_loai_luc":  nil,
		})}}
		s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

		p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
		if err != nil {
			t.Fatal(err)
		}
		if !p.ChuaChotHanXuLy() {
			t.Errorf("HanXuLyXong = %v, muốn mốc rỗng", p.HanXuLyXong)
		}
		if p.HanTiepNhanKhongApDung() {
			t.Error("HanTiepNhan bị đọc thành rỗng — đồng hồ tiếp nhận PHẢI chạy trong khoảng chờ phân loại")
		}
		if p.LinhVuc != "" {
			t.Errorf("LinhVuc = %q, muốn rỗng khi cột là NULL", p.LinhVuc)
		}
	})
}

func TestTheoMaTraCuuKhongCoDongThiTraLoiRieng(t *testing.T) {
	k := &khoGia{} // no rows at all
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if !errors.Is(err, ErrPhieuKhongTonTai) {
		// A zero-valued petition returned with a nil error would reach the handler as a real
		// record with an empty status and a deadline in year 1.
		t.Fatalf("lỗi = %v, muốn ErrPhieuKhongTonTai", err)
	}
}

func TestTheoMaTraCuuLoiDuocBocVaKhongMangMaTraCuu(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: connection refused")}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err == nil {
		t.Fatal("lỗi driver bị nuốt")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("lỗi gốc không được bọc bằng %%w: %v", err)
	}
	// THE LOOKUP CODE IS THE ONE STRING THAT OPENS A CITIZEN'S PETITION. An error travels into
	// centralised logging, into backups and into a monitoring vendor (rule 3).
	if strings.Contains(err.Error(), maThu) {
		t.Errorf("mã tra cứu lọt vào thông điệp lỗi: %v", err)
	}
}

// --- helpers -----------------------------------------------------------------------------------

func doiBang(t *testing.T, ten, duoc, muon string) {
	t.Helper()
	if duoc != muon {
		t.Errorf("%s = %q, muốn %q", ten, duoc, muon)
	}
}
