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
)

// The TASK REGISTER — what actually reaches the WHERE clause, and what actually comes back.
//
//	PROVED HERE   the commune is $1 and comes from the CONTEXT, never from an argument · soft-deleted
//	              rows are excluded on BOTH read paths · every filter becomes a BOUND PARAMETER and
//	              none is concatenated as text · the "late" filter is DERIVED from the deadline
//	              columns and reads no flag column · the "late" filter uses `han_xu_ly` and NOT
//	              `han_ban_dau` · the positional Scan lines up with cotNhiemVu BY NAME, including the
//	              three adjacent TIMESTAMPTZ columns · a NULL deadline arrives as a zero time.Time ·
//	              no row is ErrNhiemVuKhongTonTai and not a zero-valued task.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES WITH THE STATEMENT. The fake builds each row from the
//	              column list THE STORE ITSELF HANDED IT, so a column that does not exist in the real
//	              table passes here without a murmur; so does a table name that does not exist.
//	              Nothing here touches hash partition routing, the two FOREIGN KEYS into the
//	              catalogues, the CHECK constraints of migration 0006 or the immutability triggers —
//	              including the one that matters most, `han_ban_dau` being unchangeable. Those need a
//	              DSN, and nhiem_vu_pg_test.go is where they live.

const maNhiemVuThu = "NV19"

// The fixture instants are FIXED and ALL THREE ARE DIFFERENT. Equal ones would make a positional
// swap between `han_xu_ly`, `han_ban_dau` and `ngay_hoan_thanh` invisible — and that swap produces
// no error at all, only a task that looks extended when it was not.
var (
	mocHanNVThu    = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocHanGocNVThu = time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	mocTaoNVThu    = time.Date(2026, 6, 1, 3, 30, 0, 0, time.UTC)
)

// dongNhiemVu is one row of `nhiem_vu` as the driver hands it back.
//
// EVERY VALUE IS DISTINCT AND OF THE RIGHT TYPE, so a Scan wired to the wrong position comes back as
// visibly wrong data rather than as a zero that looks plausible. The two BOOLEANs are OPPOSITE for
// the same reason: they are adjacent columns of one type, and swapping them compiles and runs.
func dongNhiemVu(sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":                            "nv-001",
		"ma":                            maNhiemVuThu,
		"loai":                          "theo-van-ban",
		"khoi":                          "khoi-dang",
		"tieu_de":                       "Báo cáo tổng kết việc thực hiện chủ trương về công tác cán bộ",
		"mo_ta":                         "Tổng hợp số liệu từ các chi bộ trực thuộc.",
		"trang_thai":                    "dang-thuc-hien",
		"muc_uu_tien":                   "cao",
		"nguon_giao":                    "ket-luan-hop",
		"nguon_id":                      "klh-007",
		"bo_phan_id":                    "bp-vpdu",
		"nguoi_thuc_hien_ma":            "CB-00311",
		"lanh_dao_giao_viec_ma":         "CB-00007",
		"co_quan_chu_tri_id":            "bp-vpdu",
		"chuyen_vien_theo_doi_ma":       "CB-00412",
		"han_xu_ly":                     mocHanNVThu,
		"han_ban_dau":                   mocHanGocNVThu,
		"ngay_hoan_thanh":               nil,
		"tien_do":                       int64(40),
		"tom_tat_ket_qua":               nil,
		"ghi_chu":                       nil,
		"lanh_dao_phe_duyet_hoan_thanh": true,
		"cap_tren_cong_nhan_hoan_thanh": false,
		"nguoi_tao_ma":                  "CB-00123",
		"tao_luc":                       mocTaoNVThu,
		// `nhiem_vu_cha_id` — migration 0008. NON-NULL IN THE FIXTURE ON PURPOSE: NULL is the
		// ordinary case, so a Scan that dropped this column would still pass every assertion if the
		// sample were nil. A root task is covered by the case that overrides it.
		"nhiem_vu_cha_id": "nv-cha-001",

		// The back-link statement's columns (cauNguonHop). The fixture task is `ket-luan-hop`, so the
		// register's reads run that second statement and the fake answers it from this same map. The
		// conclusion id matches `nguon_id` above, so the link resolves.
		"k.id":           "klh-007",
		"b.id":           "bb-042",
		"b.ten_cuoc_hop": "Giao ban UBND xã tháng 8",
		"k.thu_tu":       int64(3),
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

// --- the single read -------------------------------------------------------------------------------

func TestTheoMaBuocXaVaLoaiDongDaXoa(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	if _, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu); err != nil {
		t.Fatalf("đọc nhiệm vụ: %v", err)
	}
	// TWO statements: the task, then its meeting back-link (the fixture task is `ket-luan-hop`).
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}
	l := k.lenh[0]

	// RULE 1, INVARIANTS 4 AND 5. The commune is not a parameter of TheoMa and cannot become one:
	// it arrives in the context and Scoped.Query binds it to $1.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[0] != string(xaThu) || l.args[1] != maNhiemVuThu {
		t.Errorf("tham số = %v, muốn [xã, mã nhiệm vụ]", l.args)
	}
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
}

func TestTheoMaXaTrongContextQuyetDinhChuKhongPhaiThamSo(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	// THE SAME STORE, TWO COMMUNES, TWO CONTEXTS. A store that had captured a commune at
	// construction — or taken one as an argument — would send the same $1 twice, and no test of a
	// single commune could tell.
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TheoMa(ctxXa(xaKhac), maNhiemVuThu); err != nil {
		t.Fatal(err)
	}
	// Two statements per call (task + back-link): [0],[1] are commune A's, [2],[3] commune B's. EVERY
	// one of B's must carry B — the back-link is a join, the half that could name A's meeting.
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

// TestTheoMaScanKhopVoiCotNhiemVu is the one that catches a swap among the three adjacent
// TIMESTAMPTZ columns — the most expensive off-by-one in this table, because `han_xu_ly` and
// `han_ban_dau` read as each other produce a perfectly plausible on-time ratio that is wrong.
func TestTheoMaScanKhopVoiCotNhiemVu(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	n, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu)
	if err != nil {
		t.Fatalf("đọc nhiệm vụ: %v", err)
	}

	doiBang(t, "ID", n.ID, "nv-001")
	doiBang(t, "Ma", n.Ma, maNhiemVuThu)
	doiBang(t, "Loai", n.Loai, "theo-van-ban")
	doiBang(t, "Khoi", n.Khoi, "khoi-dang")
	doiBang(t, "MucUuTien", n.MucUuTien, "cao")
	doiBang(t, "TrangThai", string(n.TrangThai), "dang-thuc-hien")
	doiBang(t, "NguonGiao", string(n.NguonGiao), "ket-luan-hop")
	doiBang(t, "NguonID", n.NguonID, "klh-007")
	doiBang(t, "BoPhanID", n.BoPhanID, "bp-vpdu")
	// THE FOUR STAFF COLUMNS ARE FOUR DIFFERENT BUSINESS CODES, and three of them are adjacent
	// TEXT columns. A swap between `nguoi_thuc_hien_ma` and `lanh_dao_giao_viec_ma` would send every
	// extension request to the person who has to ask for it.
	doiBang(t, "NguoiThucHienMa", n.NguoiThucHienMa, "CB-00311")
	doiBang(t, "LanhDaoGiaoViecMa", n.LanhDaoGiaoViecMa, "CB-00007")
	doiBang(t, "ChuyenVienTheoDoiMa", n.ChuyenVienTheoDoiMa, "CB-00412")
	doiBang(t, "NguoiTaoMa", n.NguoiTaoMa, "CB-00123")

	if n.TienDo != 40 {
		t.Errorf("TienDo = %d, muốn 40", n.TienDo)
	}
	// OPPOSITE VALUES ON PURPOSE — see dongNhiemVu.
	if !n.LanhDaoPheDuyetHoanThanh || n.CapTrenCongNhanHoanThanh {
		t.Errorf("hai ô tích đọc sai: lãnh đạo=%v, cấp trên=%v",
			n.LanhDaoPheDuyetHoanThanh, n.CapTrenCongNhanHoanThanh)
	}

	if !n.HanXuLy.Equal(mocHanNVThu) {
		t.Errorf("HanXuLy = %v, muốn %v — `han_xu_ly` và `han_ban_dau` nằm cạnh nhau và đổi chỗ "+
			"nhau thì tỷ lệ đúng hạn sai mà không có lỗi nào", n.HanXuLy, mocHanNVThu)
	}
	if !n.HanBanDau.Equal(mocHanGocNVThu) {
		t.Errorf("HanBanDau = %v, muốn %v", n.HanBanDau, mocHanGocNVThu)
	}
	if !n.TaoLuc.Equal(mocTaoNVThu) {
		t.Errorf("TaoLuc = %v, muốn %v — con trỏ phân trang đi theo cột này", n.TaoLuc, mocTaoNVThu)
	}
	if !n.NgayHoanThanh.IsZero() {
		t.Errorf("NgayHoanThanh = %v, muốn mốc rỗng khi cột là NULL", n.NgayHoanThanh)
	}
}

func TestTheoMaNULLThanhMocRongChuKhongPhaiNamMot(t *testing.T) {
	// A NULL deadline scanned into a bare time.Time is the zero value, and the zero value marshalled
	// straight out is `0001-01-01` — a date a screen renders and a comparison calls overdue.
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(map[string]driver.Value{
		"han_xu_ly":   nil,
		"han_ban_dau": nil,
		"khoi":        nil,
		"muc_uu_tien": nil,
		"nguon_id":    nil,
	})}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	n, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu)
	if err != nil {
		t.Fatal(err)
	}
	if !n.HanXuLy.IsZero() || !n.HanBanDau.IsZero() {
		t.Errorf("hạn NULL không đọc ra mốc rỗng: %v / %v", n.HanXuLy, n.HanBanDau)
	}
	// AND THE TASK IS THEN NOT LATE, whatever the clock says. Nothing was promised to anybody.
	if n.TreHan(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("nhiệm vụ không có hạn bị tính là trễ")
	}
	for ten, gt := range map[string]string{"Khoi": n.Khoi, "MucUuTien": n.MucUuTien, "NguonID": n.NguonID} {
		if gt != "" {
			t.Errorf("%s = %q, muốn rỗng khi cột là NULL", ten, gt)
		}
	}
}

func TestTheoMaKhongCoDongThiTraLoiRieng(t *testing.T) {
	k := &khoGia{} // no rows at all
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	if _, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu); !errors.Is(err, ErrNhiemVuKhongTonTai) {
		// A zero-valued task returned with a nil error would reach the handler as a real record with
		// an empty status and a deadline in year 1.
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuKhongTonTai", err)
	}
}

func TestTheoMaLoiDuocBoc(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: connection refused")}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	_, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu)
	if err == nil {
		t.Fatal("lỗi driver bị nuốt")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("lỗi gốc không được bọc bằng %%w: %v", err)
	}
}

// --- the register list -----------------------------------------------------------------------------

// chayDanhSachNhiemVu runs one page through the real store on the fake driver and returns the
// statement.
func chayDanhSachNhiemVu(t *testing.T, loc LocNhiemVu) lenhGia {
	t.Helper()
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	yc, err := page.Parse(url.Values{}, SapXepNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	if _, err := s.DanhSach(ctxXa(xaThu), loc, yc); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	// The page, then ONE back-link statement for the whole page (the fixture task is
	// `ket-luan-hop`). The page statement is the one these cases inspect.
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}
	return k.lenh[0]
}

func TestDanhSachNhiemVuBuocXaVaLoaiDongDaXoa(t *testing.T) {
	l := chayDanhSachNhiemVu(t, LocNhiemVu{})

	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(xaThu) {
		t.Errorf("tham số đầu = %v, muốn xã từ context", l.args)
	}
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
	if !strings.Contains(l.sql, " FROM nhiem_vu ") {
		t.Errorf("đọc nhầm bảng: %q", l.sql)
	}
}

// TestDanhSachNhiemVuMoiBoLocLaThamSoRangBuoc — a filter assembled as TEXT is an injection point in
// a government register. Every value below must appear in `args` and NOWHERE in the statement.
func TestDanhSachNhiemVuMoiBoLocLaThamSoRangBuoc(t *testing.T) {
	loc := LocNhiemVu{
		TrangThai:       "dang-thuc-hien",
		Loai:            "theo-van-ban",
		Khoi:            "khoi-dang",
		MucUuTien:       "khan",
		BoPhanID:        "bp-vpdu",
		NguoiThucHienMa: "CB-00311",
		NguonGiao:       "ket-luan-hop",
		Tim:             "tuyến đường Hà Lam",
	}
	l := chayDanhSachNhiemVu(t, loc)

	for _, gt := range []string{
		loc.TrangThai, loc.Loai, loc.Khoi, loc.MucUuTien,
		loc.BoPhanID, loc.NguoiThucHienMa, loc.NguonGiao,
	} {
		if strings.Contains(l.sql, gt) {
			t.Errorf("giá trị %q nằm TRONG câu lệnh thay vì là tham số: %q", gt, l.sql)
		}
		if !coThamSoChuoi(l.args, gt) {
			t.Errorf("giá trị %q không có trong tham số: %v", gt, l.args)
		}
	}
	if !coThamSoChuoi(l.args, "%"+loc.Tim+"%") {
		t.Errorf("chuỗi tìm không được truyền làm tham số: %v", l.args)
	}
	if strings.Contains(l.sql, loc.Tim) {
		t.Errorf("chuỗi tìm nằm TRONG câu lệnh: %q", l.sql)
	}
	// §3: the search box looks "trong mã + tiêu đề" — those two columns and no others. A search that
	// also reached `mo_ta` would return rows the officer cannot see why they matched.
	if !strings.Contains(l.sql, "ma ILIKE") || !strings.Contains(l.sql, "tieu_de ILIKE") {
		t.Errorf("ô tìm kiếm không quét mã + tiêu đề: %q", l.sql)
	}
	if strings.Contains(l.sql, "mo_ta ILIKE") {
		t.Errorf("ô tìm kiếm quét cả mô tả — §3 nói `tìm trong mã + tiêu đề`: %q", l.sql)
	}
}

// TestDanhSachNhiemVuKhongLocThiKhongCoDieuKienThua guards the other direction: an empty filter must
// add NOTHING. A predicate that survived a zero-valued struct would silently narrow every page.
func TestDanhSachNhiemVuKhongLocThiKhongCoDieuKienThua(t *testing.T) {
	l := chayDanhSachNhiemVu(t, LocNhiemVu{})
	for _, cot := range []string{
		"trang_thai =", "loai =", "khoi =", "muc_uu_tien =",
		"bo_phan_id =", "nguoi_thuc_hien_ma =", "nguon_giao =", "ILIKE", "han_xu_ly IS NOT NULL",
	} {
		if strings.Contains(l.sql, cot) {
			t.Errorf("bộ lọc rỗng vẫn sinh điều kiện %q: %q", cot, l.sql)
		}
	}
}

// TestDanhSachNhiemVuLocTreHanLaSUYRA — rule 10, invariant 3. The predicate compares the stored
// deadline with the database's own clock and with the recorded finishing instant; it reads no flag
// column, because there is none and there must never be one.
func TestDanhSachNhiemVuLocTreHanLaSUYRA(t *testing.T) {
	l := chayDanhSachNhiemVu(t, LocNhiemVu{ChiTreHan: true})

	if !strings.Contains(l.sql, "han_xu_ly") || !strings.Contains(l.sql, "now()") {
		t.Fatalf("bộ lọc trễ hạn không so hạn đã lưu với đồng hồ: %q", l.sql)
	}
	if !strings.Contains(l.sql, "ngay_hoan_thanh") {
		t.Error("bộ lọc trễ hạn không xét thời điểm hoàn thành — việc làm TRỄ rồi xong sẽ tự biến " +
			"mất khỏi số liệu, và báo cáo quý trước đổi mỗi lần có người mở màn hình")
	}
	for _, cam := range []string{"is_overdue", "tre_han", "qua_han"} {
		if strings.Contains(l.sql, cam) {
			t.Errorf("câu lệnh đọc cột cờ %q — quá hạn là SUY RA, không phải một cột", cam)
		}
	}
	// A task with no deadline is NOT late: it has had no date promised to anybody.
	if !strings.Contains(l.sql, "han_xu_ly IS NOT NULL") {
		t.Error("nhiệm vụ không có hạn lọt vào bộ lọc trễ hạn")
	}
	// ⚠ THE ASSERTION THIS TEST EXISTS FOR. The register's "Chỉ việc quá hạn" box asks whether the
	// work is late AS THINGS STAND, i.e. against `han_xu_ly`. Measuring it against `han_ban_dau`
	// would list every task whose extension the leader granted — precisely the complaint the
	// extension was granted to answer — and the list would look entirely reasonable.
	//
	// ASSERTED ON THE PREDICATE AND NOT ON THE WHOLE STATEMENT, because `han_ban_dau` is legitimately
	// in the SELECT list — the drawer renders both deadlines side by side (§5.6). A test that scanned
	// the whole statement would be red for a reason that is correct.
	dieuKien, _ := locNhiemVuThanhSQL(LocNhiemVu{ChiTreHan: true})
	if strings.Contains(dieuKien, "han_ban_dau") {
		t.Error("bộ lọc trễ hạn đo theo `han_ban_dau` — đó là mốc của TỶ LỆ ĐÚNG HẠN (§11.3), " +
			"không phải của ô `Chỉ việc quá hạn`")
	}
}

// TestSapXepNhiemVuKhongNhanCotCoTheNULL — page.QueryPage compares `(sort, id) > (…)`, which is NULL
// for a NULL sort column, so a nullable sort column makes every row carrying NULL vanish from page
// two onward. The allowlist is the only thing standing in the way.
func TestSapXepNhiemVuKhongNhanCotCoTheNULL(t *testing.T) {
	for _, cot := range []string{"due_at", "han_xu_ly", "completed_at", "priority", "title"} {
		if _, err := page.Parse(url.Values{"sort": {cot}}, SapXepNhiemVu); err == nil {
			t.Errorf("nhận sắp xếp theo %q — cột đó NULL được, và dòng NULL sẽ biến mất từ trang 2", cot)
		}
	}
	for _, cot := range []string{"created_at", "code"} {
		if _, err := page.Parse(url.Values{"sort": {cot}}, SapXepNhiemVu); err != nil {
			t.Errorf("từ chối sắp xếp theo %q: %v", cot, err)
		}
	}
}

// TestSapXepNhiemVuDoiCotThiDoiORDERBY.
//
// store.NewMoc compares the allowlist against the anchor readers AT CONSTRUCTION and panics on a
// mismatch, so a sort offered with no reader cannot ship. What that does NOT prove is that the
// requested sort reaches the statement — an allowlist accepted and then ignored would page every
// request by `tao_luc` while the screen's column header showed something else, and the cursor would
// still advance.
//
// THE TIE-BREAK IS ASSERTED TOO, AND IT IS THE LOAD-BEARING CHARACTER: without `, id` two rows
// sharing a sort key come back in an order the database is free to change between two queries, so
// the next page repeats one row and loses another — with no error anywhere.
func TestSapXepNhiemVuDoiCotThiDoiORDERBY(t *testing.T) {
	for tham, cot := range map[string]string{"created_at": "tao_luc", "code": "ma"} {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
		s := NewNhiemVuStore(store.New(moKhoGia(k)))

		yc, err := page.Parse(url.Values{"sort": {tham}}, SapXepNhiemVu)
		if err != nil {
			t.Fatalf("page.Parse(%q): %v", tham, err)
		}
		if _, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{}, yc); err != nil {
			t.Fatalf("DanhSach: %v", err)
		}
		if muon := "ORDER BY " + cot; !strings.Contains(k.lenh[0].sql, muon) {
			t.Errorf("sort=%q không tới được câu lệnh (muốn %q): %q", tham, muon, k.lenh[0].sql)
		}
		if !strings.Contains(k.lenh[0].sql, ", id ") {
			t.Errorf("sort=%q thiếu tie-break `, id` — trang sau vừa lặp vừa sót: %q", tham, k.lenh[0].sql)
		}
	}
}
