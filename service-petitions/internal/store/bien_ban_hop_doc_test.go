package store

import (
	"database/sql/driver"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The meeting register's READS added with migration 0012: the register order and its DATE-bound
// cursor, the lifecycle columns, the derived-status inputs, the detail read, one conclusion's tasks,
// and the task back-link.
//
//	PROVED HERE   the default order is (ngay_hop DESC, tao_luc DESC, id ASC) and the cursor walks it
//	              with the DAY BOUND AS A 'YYYY-MM-DD' STRING cast `::date`, never as an instant ·
//	              `order=asc` flips every comparison · a malformed composite cursor is page.ErrCursor ·
//	              `created_at` still pages through QueryPage · the lifecycle columns scan BY NAME · the
//	              late count uses THE SAME SQL TEXT as the task register's "late" filter · the detail
//	              read binds the commune and excludes soft-deleted rows on all three statements · one
//	              conclusion's tasks are resolved through a join bound to the commune on BOTH tables,
//	              then read by the blurred pair with the source code bound, soft-deleted tasks out, and
//	              the ceiling refused past +1 · the back-link is ONE statement per page, bound on both
//	              tables, absent for other sources and for an unresolved conclusion.
//
//	NOT PROVED    anything PostgreSQL does with these statements (the fake builds rows from the
//	              SELECT list the store handed it and ignores WHERE, ORDER BY and LIMIT). The skipped
//	              bien_ban_hop_pg_test.go carries the server-side cases.

// trangBienBan parses a page request for the meeting register.
func trangBienBan(t *testing.T, q url.Values) page.Request {
	t.Helper()
	yc, err := page.Parse(q, SapXepBienBan)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return yc
}

// --- the register order and its cursor ---------------------------------------------------------------

func TestDanhSachBienBanConTroNgayHopQuaHaiTrang(t *testing.T) {
	hai := []map[string]driver.Value{
		dongBienBan(map[string]driver.Value{"id": "bb-002", "ngay_hop": time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)}),
		dongBienBan(nil), // bb-001, 2026-08-05
	}
	k := &khoGia{hangTheoCot: hai}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	// Page 1: limit 1 of 2 rows -> one item, a cursor.
	kq, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"limit": {"1"}}))
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	if len(kq.Items) != 1 || !kq.HasMore || kq.NextCursor == "" {
		t.Fatalf("trang 1: %d mục, has_more=%v, cursor=%q", len(kq.Items), kq.HasMore, kq.NextCursor)
	}
	trang1 := k.lenh[0]
	if strings.Contains(trang1.sql, "::date") {
		t.Errorf("trang đầu không được có mốc: %q", trang1.sql)
	}
	if trang1.args[len(trang1.args)-1] != int64(2) {
		t.Errorf("LIMIT = %v, muốn limit+1 = 2", trang1.args[len(trang1.args)-1])
	}

	// Page 2 with that cursor.
	k.lenh = nil
	if _, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"limit": {"1"}, "cursor": {kq.NextCursor}})); err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	l := k.lenh[0]
	muonDK := "AND (ngay_hop < $2::date OR (ngay_hop = $2::date AND (tao_luc < $3 OR (tao_luc = $3 AND id > $4))))"
	if !strings.Contains(l.sql, muonDK) {
		t.Errorf("điều kiện mốc sai:\n có  %q\n muốn %q", l.sql, muonDK)
	}
	if !strings.Contains(l.sql, "ORDER BY ngay_hop DESC, tao_luc DESC, id ASC LIMIT $5") {
		t.Errorf("thứ tự trang 2 sai: %q", l.sql)
	}
	// ⚠ THE ASSERTION MIGRATION 0012'S HEADER ASKS FOR: the day travels as TEXT, not as a time.Time
	// the driver would bind as an instant and PostgreSQL would cast through the session time zone.
	if s, ok := l.args[1].(string); !ok || s != "2026-09-06" {
		t.Errorf("$2 = %#v, muốn chuỗi ngày \"2026-09-06\" (không phải mốc thời gian)", l.args[1])
	}
	if tt, ok := l.args[2].(time.Time); !ok || !tt.Equal(mocTaoBBThu) {
		t.Errorf("$3 = %#v, muốn giờ nhập %v", l.args[2], mocTaoBBThu)
	}
	if l.args[3] != "bb-002" {
		t.Errorf("$4 = %v, muốn id của dòng cuối trang trước", l.args[3])
	}
}

func TestDanhSachBienBanChieuTangDaoMoiPhepSo(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil), dongBienBan(map[string]driver.Value{"id": "bb-002"})}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"limit": {"1"}, "order": {"asc"}}))
	if err != nil {
		t.Fatal(err)
	}
	k.lenh = nil
	if _, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"limit": {"1"}, "order": {"asc"}, "cursor": {kq.NextCursor}})); err != nil {
		t.Fatal(err)
	}
	l := k.lenh[0].sql
	if !strings.Contains(l, "ngay_hop > $2::date") || !strings.Contains(l, "tao_luc > $3") ||
		!strings.Contains(l, "id < $4") || !strings.Contains(l, "ORDER BY ngay_hop ASC, tao_luc ASC, id DESC") {
		t.Errorf("chiều tăng không đảo đủ phép so: %q", l)
	}
}

func TestDanhSachBienBanConTroNgayHopHongLaErrCursor(t *testing.T) {
	cot := SapXepBienBan.Columns()[0] // held_on
	for _, khoa := range []string{"khong-co-gach", "2026-13-40|2026-08-05T03:30:00Z", "2026-08-05|hom-qua"} {
		conTro := page.Encode(cot, page.Desc, page.Anchor{Key: page.TextKey(khoa), ID: "bb-001"})
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
		s := NewBienBanHopStore(store.New(moKhoGia(k)))

		_, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"cursor": {conTro}}))
		if !errors.Is(err, page.ErrCursor) {
			t.Errorf("mốc %q: lỗi = %v, muốn page.ErrCursor", khoa, err)
		}
		if len(k.lenh) != 0 {
			t.Errorf("mốc %q: đã chạy truy vấn với con trỏ hỏng", khoa)
		}
	}
}

// TestDanhSachBienBanCreatedAtVanQuaQueryPage — the contract's old sort still works, unchanged.
func TestDanhSachBienBanCreatedAtVanQuaQueryPage(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	if _, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, url.Values{"sort": {"created_at"}})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(k.lenh[0].sql, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("sort=created_at không còn theo tao_luc: %q", k.lenh[0].sql)
	}
}

// --- the lifecycle columns and the derived-status inputs ----------------------------------------------

func TestDanhSachBienBanDocCotVongDoiTheoTen(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	b := kq.Items[0]
	if b.TrangThai != "da-ky" || !b.KyLuc.Equal(mocKyBBThu) || b.KyBoiMa != "CB-00031" ||
		b.ThuKyMa != "CB-00042" || b.TbSoKyHieu != "45/TB-UBND" || !b.TbNgay.Equal(mocTbNgayThu) ||
		b.BoSungChoID != "bb-goc-000" {
		t.Errorf("cột vòng đời đọc lệch: %+v", b)
	}
	// The LIST does not read the body, the attendees or the supplements.
	if b.NoiDung != "" || b.ThanhPhan != nil || b.DuocBoSungBoi != nil {
		t.Errorf("danh sách đọc cả trường chỉ-chi-tiết: %+v", b)
	}

	kl := b.KetLuan[0]
	if kl.SoNhiemVu != 3 || kl.SoNhiemVuXong != 1 || kl.SoNhiemVuTreHan != 2 || kl.KhongPhatSinh {
		t.Errorf("bộ đếm kết luận đọc lệch: %+v", kl)
	}
	if kl.TrangThai() != domain.KetLuanQuaHan {
		t.Errorf("trạng thái suy ra = %q, muốn qua-han", kl.TrangThai())
	}
}

func TestDanhSachBienBanCotVongDoiNullThanhRong(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(map[string]driver.Value{
		"trang_thai": "du-thao", "ky_luc": nil, "ky_boi_ma": nil, "thu_ky_ma": nil,
		"tb_so_ky_hieu": nil, "tb_ngay": nil, "bo_sung_cho_id": nil,
	})}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	kq, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	b := kq.Items[0]
	if !b.KyLuc.IsZero() || b.KyBoiMa != "" || b.ThuKyMa != "" || b.TbSoKyHieu != "" ||
		!b.TbNgay.IsZero() || b.BoSungChoID != "" {
		t.Errorf("NULL không thành giá trị rỗng: %+v", b)
	}
}

// TestKetLuanDemTreHanDungChungMotCauVoiBoLocSoNhiemVu — the late count and the register's "Chỉ việc
// quá hạn" filter are ONE SQL text (dieuKienTreHan), and the count excludes finished tasks by the
// BOUND status code, not a literal.
func TestKetLuanDemTreHanDungChungMotCauVoiBoLocSoNhiemVu(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	if _, err := s.DanhSach(ctxXa(xaThu), trangBienBan(t, nil)); err != nil {
		t.Fatal(err)
	}
	noi := k.lenh[1].sql
	if !strings.Contains(noi, "nv.trang_thai <> $3 AND "+dieuKienTreHan) {
		t.Errorf("bộ đếm trễ hạn không dùng dieuKienTreHan hoặc không loại việc đã xong: %q", noi)
	}
	loc, _ := locNhiemVuThanhSQL(LocNhiemVu{ChiTreHan: true})
	if !strings.Contains(loc, dieuKienTreHan) {
		t.Errorf("bộ lọc `late` của sổ nhiệm vụ không còn dùng dieuKienTreHan: %q", loc)
	}
	for _, cam := range []string{"is_overdue", "qua_han =", "tre_han ="} {
		if strings.Contains(noi, cam) {
			t.Errorf("câu đếm đọc một cột cờ %q — quá hạn là SUY RA", cam)
		}
	}
}

// --- GET /api/v1/meetings/{id} ------------------------------------------------------------------------

func TestTheoIDBaCauLenhDeuBuocXaVaLoaiDongDaXoa(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	b, err := s.TheoID(ctxXa(xaThu), "bb-001")
	if err != nil {
		t.Fatalf("TheoID: %v", err)
	}
	if len(k.lenh) != 3 {
		t.Fatalf("chạy %d câu lệnh, muốn 3 (biên bản, kết luận, bổ sung)", len(k.lenh))
	}
	for i, l := range k.lenh {
		if l.args[0] != string(xaThu) {
			t.Errorf("câu %d: $1 = %v, muốn xã trong context", i, l.args[0])
		}
		if !strings.Contains(l.sql, "deleted_at IS NULL") {
			t.Errorf("câu %d thiếu loại dòng đã xoá mềm: %q", i, l.sql)
		}
	}
	if !strings.Contains(k.lenh[0].sql, "AND id = $2 AND deleted_at IS NULL") || k.lenh[0].args[1] != "bb-001" {
		t.Errorf("câu đọc biên bản sai: %q %v", k.lenh[0].sql, k.lenh[0].args)
	}
	bs := k.lenh[2]
	if !strings.Contains(bs.sql, "bo_sung_cho_id = $2") || bs.args[1] != "bb-001" ||
		bs.args[2] != int64(TranBoSungMotBienBan+1) {
		t.Errorf("câu đọc bổ sung sai: %q %v", bs.sql, bs.args)
	}

	if b.NoiDung != "Toàn văn biên bản." || len(b.ThanhPhan) != 2 || b.ThanhPhan[1] != "Đại diện Mặt trận xã" {
		t.Errorf("nội dung / thành phần đọc sai: %q %v", b.NoiDung, b.ThanhPhan)
	}
	if len(b.KetLuan) != 1 || b.KetLuan[0].ID != "kl-001" {
		t.Errorf("kết luận không gắn vào biên bản: %+v", b.KetLuan)
	}
	// The fake answers the supplements query from the same row; what matters is that the slice is
	// filled (non-nil) on the detail read.
	if b.DuocBoSungBoi == nil {
		t.Error("tuyến chi tiết trả DuocBoSungBoi nil — `chưa đọc` lẫn với `không có`")
	}
}

func TestTheoIDKhongCoDongThiErrBienBanKhongTonTai(t *testing.T) {
	k := &khoGia{}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	if _, err := s.TheoID(ctxXa(xaThu), "bb-khong-co"); !errors.Is(err, ErrBienBanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanKhongTonTai", err)
	}
	if len(k.lenh) != 1 {
		t.Errorf("chạy %d câu lệnh cho một biên bản không tồn tại, muốn 1", len(k.lenh))
	}
}

func TestTheoIDThanhPhanRongLaMangRong(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(map[string]driver.Value{
		"thanh_phan": []byte(`[]`), "noi_dung": nil,
	})}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	b, err := s.TheoID(ctxXa(xaThu), "bb-001")
	if err != nil {
		t.Fatal(err)
	}
	if b.ThanhPhan == nil || len(b.ThanhPhan) != 0 || b.NoiDung != "" {
		t.Errorf("thành phần rỗng / nội dung NULL đọc sai: %#v %q", b.ThanhPhan, b.NoiDung)
	}
}

// --- GET /api/v1/meetings/{id}/conclusions/{stt}/tasks -------------------------------------------------

func TestNhiemVuCuaKetLuanBuocXaCapNguonVaLoaiDongDaXoa(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	ds, err := s.NhiemVuCuaKetLuan(ctxXa(xaThu), "bb-042", 3)
	if err != nil {
		t.Fatalf("NhiemVuCuaKetLuan: %v", err)
	}
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}

	// 1. The conclusion, through a join bound to the commune on BOTH tables, both live.
	kl := k.lenh[0]
	for _, can := range []string{"k.tenant_id = $1", "b.tenant_id = $1", "b.deleted_at IS NULL",
		"k.deleted_at IS NULL", "k.bien_ban_id = $2", "k.thu_tu = $3"} {
		if !strings.Contains(kl.sql, can) {
			t.Errorf("câu tìm kết luận thiếu %q: %q", can, kl.sql)
		}
	}
	if kl.args[0] != string(xaThu) || kl.args[1] != "bb-042" || kl.args[2] != int64(3) {
		t.Errorf("tham số tìm kết luận = %v", kl.args)
	}

	// 2. The tasks by the blurred pair: commune $1, source code BOUND, THIS conclusion's id, live only.
	nv := k.lenh[1]
	for _, can := range []string{"WHERE tenant_id = $1", " FROM nhiem_vu ", "nguon_giao = $2",
		"nguon_id = $3", "deleted_at IS NULL", "LIMIT $4"} {
		if !strings.Contains(nv.sql, can) {
			t.Errorf("câu đọc nhiệm vụ thiếu %q: %q", can, nv.sql)
		}
	}
	if strings.Contains(nv.sql, "'ket-luan-hop'") {
		t.Errorf("mã nguồn nằm thẳng trong câu lệnh: %q", nv.sql)
	}
	if nv.args[0] != string(xaThu) || nv.args[1] != string(domain.NguonKetLuanHop) ||
		nv.args[2] != "klh-007" || nv.args[3] != int64(TranNhiemVuMotKetLuan+1) {
		t.Errorf("tham số đọc nhiệm vụ = %v — muốn [xã, ket-luan-hop, id kết luận VỪA TÌM, trần+1]", nv.args)
	}
	if len(ds) != 1 || ds[0].Ma != maNhiemVuThu || ds[0].NguonID != "klh-007" {
		t.Errorf("hàng nhiệm vụ đọc sai: %+v", ds)
	}
}

func TestNhiemVuCuaKetLuanKhongCoKetLuanThiKhongDocNhiemVu(t *testing.T) {
	k := &khoGia{}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	if _, err := s.NhiemVuCuaKetLuan(ctxXa(xaThu), "bb-001", 9); !errors.Is(err, ErrKetLuanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongTonTai", err)
	}
	if len(k.lenh) != 1 {
		t.Errorf("chạy %d câu lệnh, muốn 1 — không đọc nhiệm vụ của kết luận không tồn tại", len(k.lenh))
	}
}

func TestNhiemVuCuaKetLuanTranTuChoiQuaMotDong(t *testing.T) {
	nhieu := func(n int) []map[string]driver.Value {
		ra := make([]map[string]driver.Value, n)
		for i := range ra {
			ra[i] = dongNhiemVu(nil)
		}
		return ra
	}
	k := &khoGia{hangTheoCot: nhieu(TranNhiemVuMotKetLuan)}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))
	ds, err := s.NhiemVuCuaKetLuan(ctxXa(xaThu), "bb-042", 3)
	if err != nil || len(ds) != TranNhiemVuMotKetLuan {
		t.Errorf("đúng bằng trần: %d dòng, lỗi %v — muốn đủ %d", len(ds), err, TranNhiemVuMotKetLuan)
	}

	k = &khoGia{hangTheoCot: nhieu(TranNhiemVuMotKetLuan + 1)}
	s = NewBienBanHopStore(store.New(moKhoGia(k)))
	ds, err = s.NhiemVuCuaKetLuan(ctxXa(xaThu), "bb-042", 3)
	if !errors.Is(err, ErrQuaNhieuNhiemVuKetLuan) || ds != nil {
		t.Errorf("vượt trần: lỗi = %v, %d dòng — muốn TỪ CHỐI, không cắt bớt", err, len(ds))
	}
}

// --- the task back-link --------------------------------------------------------------------------------

func TestNguonHopMotCauChoCaTrangVaBuocXaHaiBang(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		dongNhiemVu(nil),
		dongNhiemVu(map[string]driver.Value{"id": "nv-002", "ma": "NV20", "nguon_id": "klh-008"}),
		dongNhiemVu(map[string]driver.Value{"id": "nv-003", "ma": "NV21"}), // same conclusion as nv-001
	}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	kq, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{}, func() page.Request {
		yc, _ := page.Parse(url.Values{}, SapXepNhiemVu)
		return yc
	}())
	if err != nil {
		t.Fatal(err)
	}
	// NO N+1: the page, then ONE back-link statement, whatever the number of tasks.
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh cho 3 nhiệm vụ, muốn 2 (không N+1)", len(k.lenh))
	}
	l := k.lenh[1]
	for _, can := range []string{"k.tenant_id = $1", "b.tenant_id = $1", "b.deleted_at IS NULL",
		"k.deleted_at IS NULL", "IN ($2, $3)"} {
		if !strings.Contains(l.sql, can) {
			t.Errorf("câu liên kết ngược thiếu %q: %q", can, l.sql)
		}
	}
	// Distinct ids, bound — nv-001 and nv-003 share one conclusion.
	if len(l.args) != 3 || l.args[1] != "klh-007" || l.args[2] != "klh-008" {
		t.Errorf("tham số = %v, muốn [xã, klh-007, klh-008]", l.args)
	}
	// The fake's back-link rows all carry k.id = klh-007, so nv-001 and nv-003 resolve and nv-002
	// (klh-008 — no live row) carries NO link: a removed meeting/conclusion omits the fields.
	for _, n := range kq.Items {
		switch n.NguonID {
		case "klh-007":
			if n.NguonHop == nil || n.NguonHop.BienBanID != "bb-042" ||
				n.NguonHop.TenCuocHop != "Giao ban UBND xã tháng 8" || n.NguonHop.ThuTu != 3 {
				t.Errorf("%s: liên kết ngược sai: %+v", n.Ma, n.NguonHop)
			}
		case "klh-008":
			if n.NguonHop != nil {
				t.Errorf("%s: mang liên kết tới một kết luận không còn sống: %+v", n.Ma, n.NguonHop)
			}
		}
	}
}

func TestNguonHopKhongChayKhiKhongCoNhiemVuTuKetLuan(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(map[string]driver.Value{
		"nguon_giao": "truc-tiep", "nguon_id": nil,
	})}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	n, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) != 1 || n.NguonHop != nil {
		t.Errorf("nhiệm vụ giao trực tiếp: %d câu lệnh, liên kết %+v — muốn 1 câu, không liên kết", len(k.lenh), n.NguonHop)
	}
}
