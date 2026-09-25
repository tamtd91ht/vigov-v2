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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE MEETING MINUTES REGISTER — what actually reaches the WHERE clause, and what actually comes
// back.
//
//	PROVED HERE   the commune is $1 on BOTH statements and comes from the CONTEXT, never from an
//	              argument · the commune is bound on the JOINED side too, so a badge cannot count
//	              another commune's tasks · soft-deleted rows are excluded on the minutes, on the
//	              conclusions AND on the tasks behind the counters · the two enum codes are BOUND
//	              PARAMETERS carrying the domain constants, not literals in the string · the
//	              conclusion ids are placeholders, never concatenated text · an empty page runs the
//	              second statement NOT AT ALL · the positional Scan lines up with cotBienBan BY NAME
//	              · a NULL `so_hieu`/`dia_diem`/`chu_tri_ma` arrives as an empty string.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES WITH THE STATEMENT. The fake builds each row from the
//	              column list THE STORE ITSELF HANDED IT, so a column that does not exist in the real
//	              table passes here without a murmur; so does a table name that does not exist. It
//	              cannot execute a LATERAL join, cannot aggregate, and knows nothing of hash partition
//	              routing, the foreign key, the unique key on (tenant_id, bien_ban_id, thu_tu) or the
//	              hard-delete trigger. Those need a DSN, and bien_ban_hop_pg_test.go is where they
//	              live — SKIPPED in this environment.

var (
	mocTaoBBThu   = time.Date(2026, 8, 5, 3, 30, 0, 0, time.UTC)
	mocNgayHopThu = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	mocTaoKLThu   = time.Date(2026, 8, 6, 4, 0, 0, 0, time.UTC)
	mocKyBBThu    = time.Date(2026, 8, 7, 9, 15, 0, 0, time.UTC)
	mocTbNgayThu  = time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
)

// dongBienBan is ONE fixture serving BOTH statements: the fake assembles every row from the SELECT
// list of whatever query is running, so the map has to carry the columns of the page read AND the
// aliased columns of the joined read. A missing key panics loudly rather than scanning a nil that
// "passes" while proving nothing.
//
// EVERY VALUE IS DISTINCT, AND THE THREE ADJACENT NULLABLE TEXT COLUMNS (`so_hieu`, `dia_diem`,
// `chu_tri_ma`) hold visibly different KINDS of string: a Scan wired one position off puts a room
// name where a staff code belongs, and that must come back as wrong data rather than as a plausible
// blank. The two counters differ (1 of 2) for the same reason.
func dongBienBan(sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		// GET /api/v1/meetings — the page of cards.
		"id":           "bb-001",
		"ten_cuoc_hop": "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
		"ngay_hop":     mocNgayHopThu,
		"so_hieu":      "31/BB-UBND",
		"dia_diem":     "Phòng họp UBND xã",
		"chu_tri_ma":   "CB-00007",
		"nguoi_tao_ma": "CB-00123",
		"tao_luc":      mocTaoBBThu,

		// Migration 0012's columns. The four adjacent nullable TEXT columns hold four visibly
		// different KINDS of value (two staff codes, a notice number, an id), so a Scan shifted by one
		// position puts a notice number where the secretary belongs.
		"trang_thai":     "da-ky",
		"ky_luc":         mocKyBBThu,
		"ky_boi_ma":      "CB-00031",
		"thu_ky_ma":      "CB-00042",
		"tb_so_ky_hieu":  "45/TB-UBND",
		"tb_ngay":        mocTbNgayThu,
		"bo_sung_cho_id": "bb-goc-000",

		// The detail read's two extra columns.
		"noi_dung":   "Toàn văn biên bản.",
		"thanh_phan": []byte(`["CB-00007","Đại diện Mặt trận xã"]`),

		// The conclusions of that page, with their counters — aliased, as the joined statement
		// selects them. The three counters and the mark are all DIFFERENT, so a swap shows.
		"k.id":                  "kl-001",
		"k.bien_ban_id":         "bb-001",
		"k.thu_tu":              int64(2),
		"k.noi_dung":            "Giao Địa chính rà soát tiến độ tuyến đường, báo cáo trước ngày 20/8.",
		"k.tao_luc":             mocTaoKLThu,
		"k.khong_phat_sinh":     false,
		"n.so_nhiem_vu":         int64(3),
		"n.so_nhiem_vu_xong":    int64(1),
		"n.so_nhiem_vu_tre_han": int64(2),
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

func trangDau(t *testing.T) page.Request {
	t.Helper()
	yc, err := page.Parse(url.Values{}, SapXepBienBan)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return yc
}

// --- the page of cards --------------------------------------------------------------------------

func TestDanhSachBienBanBuocXaVaLoaiDongDaXoa(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	if _, err := s.DanhSach(ctxXa(xaThu), trangDau(t)); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2 (một trang thẻ, một lượt đọc kết luận)", len(k.lenh))
	}

	// RULE 1, INVARIANTS 4 AND 5, ON THE PAGE READ. The commune is not a parameter of DanhSach and
	// cannot become one: it arrives in the context and Scoped.Query binds it to $1.
	trang := k.lenh[0]
	if !strings.Contains(trang.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh trang không lọc theo xã: %q", trang.sql)
	}
	if len(trang.args) == 0 || trang.args[0] != string(xaThu) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", trang.args, xaThu)
	}
	if !strings.Contains(trang.sql, " FROM bien_ban_hop ") {
		t.Errorf("đọc nhầm bảng: %q", trang.sql)
	}
	// RULE 7, INVARIANT 2.
	if !strings.Contains(trang.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", trang.sql)
	}
	// The order is TOTAL and is the register's (user decision 5): meeting day newest first, same day
	// by entry time, `id` last — the column order and directions of index bien_ban_hop_theo_ngay_hop.
	if !strings.Contains(trang.sql, "ORDER BY ngay_hop DESC, tao_luc DESC, id ASC") {
		t.Errorf("thứ tự không theo ngày họp / không ổn định: %q", trang.sql)
	}
}

// TestDanhSachBienBanXaTrongContextQuyetDinhChuKhongPhaiThamSo — the same store, two communes, two
// contexts. A store that had captured a commune at construction, or taken one as an argument, would
// send the same $1 twice and no test of a single commune could tell.
func TestDanhSachBienBanXaTrongContextQuyetDinhChuKhongPhaiThamSo(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := s.DanhSach(ctxXa(xaThu), trangDau(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DanhSach(ctxXa(xaKhac), trangDau(t)); err != nil {
		t.Fatal(err)
	}
	// Four statements: page, conclusions, page, conclusions. BOTH of the second commune's must carry
	// its own id — the joined one is the dangerous half, because it is the one that counts tasks.
	if len(k.lenh) != 4 {
		t.Fatalf("chạy %d câu lệnh, muốn 4", len(k.lenh))
	}
	for i, l := range k.lenh {
		muon := string(xaThu)
		if i >= 2 {
			muon = string(xaKhac)
		}
		if l.args[0] != muon {
			t.Errorf("câu lệnh %d: $1 = %v, muốn %q", i, l.args[0], muon)
		}
	}
}

// TestDanhSachBienBanScanKhopVoiCotBienBan catches a Scan wired one position off — the three
// adjacent nullable TEXT columns are where that produces a room name in a staff-code field and no
// error at all.
func TestDanhSachBienBanScanKhopVoiCotBienBan(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(kq.Items) != 1 {
		t.Fatalf("đọc %d biên bản, muốn 1", len(kq.Items))
	}
	b := kq.Items[0]
	if b.ID != "bb-001" || b.TenCuocHop != "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026" {
		t.Errorf("id/tên sai: %+v", b)
	}
	if !b.NgayHop.Equal(mocNgayHopThu) {
		t.Errorf("NgayHop = %v, muốn %v", b.NgayHop, mocNgayHopThu)
	}
	if b.SoHieu != "31/BB-UBND" || b.DiaDiem != "Phòng họp UBND xã" || b.ChuTriMa != "CB-00007" {
		t.Errorf("ba cột TEXT liền nhau bị lệch vị trí: so_hieu=%q dia_diem=%q chu_tri_ma=%q",
			b.SoHieu, b.DiaDiem, b.ChuTriMa)
	}
	if b.NguoiTaoMa != "CB-00123" || !b.TaoLuc.Equal(mocTaoBBThu) {
		t.Errorf("người tạo / thời điểm tạo sai: %+v", b)
	}
}

// TestDanhSachBienBanCotNullThanhChuoiRong — a NULL reference number is ordinary (§4 marks the field
// optional). It must arrive as an empty string, not as a driver error and not as a plausible value.
func TestDanhSachBienBanCotNullThanhChuoiRong(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(map[string]driver.Value{
		"so_hieu":    nil,
		"dia_diem":   nil,
		"chu_tri_ma": nil,
	})}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	b := kq.Items[0]
	if b.SoHieu != "" || b.DiaDiem != "" || b.ChuTriMa != "" {
		t.Errorf("cột NULL không thành chuỗi rỗng: %+v", b)
	}
}

func TestDanhSachBienBanLoiDriverDuocBocChuKhongNuot(t *testing.T) {
	loi := errors.New("kết nối rụng")
	k := &khoGia{loi: loi}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	_, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if !errors.Is(err, loi) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, loi)
	}
}

// --- the conclusions and their counters -----------------------------------------------------------

// TestKetLuanBuocXaTrenCaHaiVeCuaPhepNoi is the cross-commune case this file exists for.
//
// The joined statement reads `ket_luan_hop` AND `nhiem_vu`. Constraining only the first would count
// ANOTHER COMMUNE's tasks into this commune's badge wherever two conclusion ids collide — a leak no
// test of a single commune could ever show (rule 1, and store.Scoped.QueryJoin's contract says so in
// as many words).
func TestKetLuanBuocXaTrenCaHaiVeCuaPhepNoi(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	if _, err := s.DanhSach(ctxXa(xaThu), trangDau(t)); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	noi := k.lenh[1]

	if !strings.Contains(noi.sql, "k.tenant_id = $1") {
		t.Errorf("vế kết luận không buộc xã: %q", noi.sql)
	}
	if !strings.Contains(noi.sql, "nv.tenant_id = $1") {
		t.Errorf("VẾ NHIỆM VỤ KHÔNG BUỘC XÃ — badge của xã này đếm được việc của xã khác: %q", noi.sql)
	}
	if noi.args[0] != string(xaThu) {
		t.Errorf("$1 = %v, muốn %q", noi.args[0], xaThu)
	}
	// RULE 7, INVARIANT 2 ON BOTH SIDES. A removed task must leave BOTH halves of `x/y`, and a
	// removed conclusion must leave the card entirely.
	if !strings.Contains(noi.sql, "k.deleted_at IS NULL") {
		t.Errorf("kết luận đã xoá mềm vẫn được đọc: %q", noi.sql)
	}
	if !strings.Contains(noi.sql, "nv.deleted_at IS NULL") {
		t.Errorf("nhiệm vụ đã xoá mềm vẫn được đếm: %q", noi.sql)
	}
	// The circles are drawn in `thu_tu` order, fixed in the query so no layer above has to sort.
	if !strings.Contains(noi.sql, "ORDER BY k.bien_ban_id, k.thu_tu") {
		t.Errorf("thứ tự kết luận không theo số trong ô tròn: %q", noi.sql)
	}
}

// TestKetLuanHaiMaEnumLaThamSoBuocChuKhongPhaiChuoiTrongCauLenh.
//
// A misspelt `'ket-luan-hop'` or `'hoan-thanh'` written into the SQL produces NO error: every badge
// reads `0/0`, forever, and looks exactly like a commune that has split nothing yet. Binding the
// DOMAIN CONSTANTS makes a rename a compile error instead — and this case is what proves the values
// really travel as arguments rather than as text.
func TestKetLuanHaiMaEnumLaThamSoBuocChuKhongPhaiChuoiTrongCauLenh(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongBienBan(nil)}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	if _, err := s.DanhSach(ctxXa(xaThu), trangDau(t)); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	noi := k.lenh[1]

	for _, ma := range []string{string(domain.NguonKetLuanHop), string(domain.HoanThanh)} {
		if strings.Contains(noi.sql, "'"+ma+"'") {
			t.Errorf("mã %q nằm thẳng trong câu lệnh thay vì là tham số: %q", ma, noi.sql)
		}
	}
	if len(noi.args) < 3 {
		t.Fatalf("tham số = %v, muốn ít nhất [xã, nguồn giao, trạng thái xong, …id]", noi.args)
	}
	if noi.args[1] != string(domain.NguonKetLuanHop) {
		t.Errorf("$2 = %v, muốn %q", noi.args[1], domain.NguonKetLuanHop)
	}
	if noi.args[2] != string(domain.HoanThanh) {
		t.Errorf("$3 = %v, muốn %q", noi.args[2], domain.HoanThanh)
	}
}

// TestKetLuanIdLaThamSoChuKhongNoiChuoi — the ids come from rows the store has just read, and they
// still go in as placeholders. A list built by concatenation is an injection point in a government
// register, and the fact that "the values came from our own table" is exactly the argument that
// makes such a shape survive review.
func TestKetLuanIdLaThamSoChuKhongNoiChuoi(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		dongBienBan(nil),
		dongBienBan(map[string]driver.Value{"id": "bb-002", "k.bien_ban_id": "bb-002", "k.id": "kl-002"}),
	}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(kq.Items) != 2 {
		t.Fatalf("đọc %d biên bản, muốn 2", len(kq.Items))
	}
	noi := k.lenh[1]
	if !strings.Contains(noi.sql, "IN ($4, $5)") {
		t.Errorf("danh sách id không phải chỗ giữ tham số: %q", noi.sql)
	}
	if strings.Contains(noi.sql, "bb-001") {
		t.Errorf("id bị nối thẳng vào câu lệnh: %q", noi.sql)
	}
	if len(noi.args) != 5 || noi.args[3] != "bb-001" || noi.args[4] != "bb-002" {
		t.Errorf("tham số = %v, muốn [xã, nguồn, trạng thái, bb-001, bb-002]", noi.args)
	}
}

// TestKetLuanGanDungVaoBienBanCuaNo — the second statement's rows are keyed back onto the cards by
// `bien_ban_id`. Keyed any other way (by position, by index) a page with one meeting would pass and
// a page with several would put one meeting's conclusions on another's card.
func TestKetLuanGanDungVaoBienBanCuaNo(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		dongBienBan(nil),
		dongBienBan(map[string]driver.Value{
			"id": "bb-002", "k.id": "kl-002", "k.bien_ban_id": "bb-002",
			"k.thu_tu": int64(1), "n.so_nhiem_vu": int64(0), "n.so_nhiem_vu_xong": int64(0),
			"n.so_nhiem_vu_tre_han": int64(0),
		}),
	}}
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	for _, b := range kq.Items {
		if len(b.KetLuan) != 1 {
			t.Fatalf("biên bản %s có %d kết luận, muốn 1", b.ID, len(b.KetLuan))
		}
		if b.KetLuan[0].BienBanID != b.ID {
			t.Errorf("biên bản %s mang kết luận của %s", b.ID, b.KetLuan[0].BienBanID)
		}
	}
	// And the counters survive the trip: 1 of 3 on the first card, nothing split on the second.
	if xong, tong := kq.Items[0].TienDoNhiemVu(); xong != 1 || tong != 3 {
		t.Errorf("thẻ 1: %d/%d, muốn 1/3", xong, tong)
	}
	if !kq.Items[1].KetLuan[0].ChuaTachNhiemVu() {
		t.Error("thẻ 2: kết luận chưa tách lại không báo `chưa tách`")
	}
}

// TestDanhSachBienBanRongThiKhongChayCauThuHai is both a saving and a correctness case: the `IN (…)`
// list is built from the page's ids, and an empty list is not valid SQL. A newly onboarded commune
// takes this branch on every request until its first meeting is recorded.
func TestDanhSachBienBanRongThiKhongChayCauThuHai(t *testing.T) {
	k := &khoGia{} // no rows at all
	s := NewBienBanHopStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), trangDau(t))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(kq.Items) != 0 {
		t.Fatalf("đọc %d biên bản trên một xã rỗng", len(kq.Items))
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh trên trang rỗng, muốn 1", len(k.lenh))
	}
}

// TestSapXepBienBanTuChoiCotNgoaiDanhSachTrang — `held_at` is NOT offered, deliberately (see
// SapXepBienBan). A sort key the allowlist does not know must be refused rather than ignored: a page
// silently sorted by something else is a page the cursor walks in an order nobody asked for.
func TestSapXepBienBanTuChoiCotNgoaiDanhSachTrang(t *testing.T) {
	for _, cot := range []string{"held_at", "ngay_hop", "title", "reference_no"} {
		if _, err := page.Parse(url.Values{"sort": {cot}}, SapXepBienBan); err == nil {
			t.Errorf("nhận sắp xếp theo %q mà cột đó không nằm trong danh sách trắng", cot)
		}
	}
	// `held_on` is the new default; `created_at` stays accepted for clients already sending it.
	for _, cot := range []string{"held_on", "created_at"} {
		if _, err := page.Parse(url.Values{"sort": {cot}}, SapXepBienBan); err != nil {
			t.Errorf("từ chối sắp xếp theo %q: %v", cot, err)
		}
	}
}
