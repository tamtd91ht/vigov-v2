package store

import (
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The filters (`unit`, `published`) and the free-text search of the staff register, against the
// SAME fake engine as can_bo_danh_sach_test.go — which executes the predicates rather than
// recording them, so a filter that is built wrong returns wrong ROWS here.
//
// dsDuLieu, as these cases use it:
//
//	nd-01  bp-001  not published  "Nguyễn Văn A"  Chủ tịch UBND xã   0900000001 / 0300000001
//	nd-02  bp-002  PUBLISHED      "Trần Thị B"    Trưởng thôn         0900000002 / 0300000002
//	nd-03  bp-001  not published  "Lê Văn C"      Kế toán             0900000003 / 0300000003
//	nd-04  —       soft-deleted   "Phạm Thị D"
//	nd-05  none    not published  "Đỗ Văn E"                          0900000005 / 0300.000.005
//	ndb-*  commune B

// timHet walks every page of one filtered read and returns the ids, sorted.
func (b *banThuDS) timHet(t *testing.T, xa string, loc domain.LocCanBo, truyVan string) []string {
	t.Helper()
	var ids []string
	conTro := ""
	for i := 0; ; i++ {
		if i > 50 {
			t.Fatal("con trỏ không tiến — vòng lặp vô hạn")
		}
		q, err := url.ParseQuery(truyVan)
		if err != nil {
			t.Fatal(err)
		}
		if conTro != "" {
			q.Set("cursor", conTro)
		}
		yc, err := page.Parse(q, SapXepCanBo)
		if err != nil {
			t.Fatalf("page.Parse: %v", err)
		}
		kq, err := b.kho.DanhSach(ctxXa(xa), loc, yc)
		if err != nil {
			t.Fatalf("DanhSach(%+v): %v", loc, err)
		}
		for _, cb := range kq.Items {
			ids = append(ids, cb.ID)
		}
		if !kq.HasMore {
			sort.Strings(ids)
			return ids
		}
		conTro = kq.NextCursor
	}
}

func boolP(v bool) *bool { return &v }

func kiemIDs(t *testing.T, ten string, got []string, muon ...string) {
	t.Helper()
	sort.Strings(muon)
	if strings.Join(got, ",") != strings.Join(muon, ",") {
		t.Errorf("%s: được %v, muốn %v", ten, got, muon)
	}
}

// lenhCuoi is the last statement that reached the driver.
func (b *banThuDS) lenhCuoi(t *testing.T) lenhDS {
	t.Helper()
	ds := b.ghi.tatCa()
	if len(ds) == 0 {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	return ds[len(ds)-1]
}

// --- the two URL filters -------------------------------------------------------------------------

func TestLocCanBoTheoBoPhan(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "unit=bp-001", b.timHet(t, xaMau, domain.LocCanBo{BoPhanID: "bp-001"}, "limit=100"),
		"nd-01", "nd-03")

	l := b.lenhCuoi(t)
	if !strings.Contains(l.sql, "AND bo_phan_id = $2") {
		t.Errorf("thiếu bộ lọc bộ phận ràng buộc ở $2: %q", l.sql)
	}
	if l.args[0] != xaMau || l.args[1] != "bp-001" {
		t.Errorf("tham số = %v — $1 phải là xã, $2 là bộ phận", l.args)
	}
}

func TestLocCanBoTheoCongKhai(t *testing.T) {
	// false IS A REAL FILTER ("who is not on the Mini App yet"), not "no filter" — a mutation that
	// dropped the pointer would return all four rows for it.
	b := moBanThuDS(t)
	kiemIDs(t, "published=true", b.timHet(t, xaMau, domain.LocCanBo{CongKhai: boolP(true)}, "limit=100"),
		"nd-02")
	kiemIDs(t, "published=false", b.timHet(t, xaMau, domain.LocCanBo{CongKhai: boolP(false)}, "limit=100"),
		"nd-01", "nd-03", "nd-05")

	if l := b.lenhCuoi(t); l.args[1] != false {
		t.Errorf("$2 = %#v, muốn bool false ràng buộc — không nối chuỗi vào câu lệnh", l.args[1])
	}
}

func TestLocCanBoHaiBoLocLaAND(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "bp-001 & published", b.timHet(t, xaMau,
		domain.LocCanBo{BoPhanID: "bp-001", CongKhai: boolP(true)}, "limit=100"))
	kiemIDs(t, "bp-002 & published", b.timHet(t, xaMau,
		domain.LocCanBo{BoPhanID: "bp-002", CongKhai: boolP(true)}, "limit=100"), "nd-02")
}

// The keyset walk is UNCHANGED by a filter: limit=1 through a filtered set visits each matching row
// exactly once. The filter's placeholders come BEFORE the anchor's — a numbering slip there binds
// the anchor to the filter and this walk loops or loses a row.
func TestLocCanBoPhanTrangKhongLapKhongSot(t *testing.T) {
	b := moBanThuDS(t)
	for _, truyVan := range []string{"limit=1", "limit=1&sort=created_at", "limit=1&order=desc"} {
		kiemIDs(t, "unit=bp-001 "+truyVan,
			b.timHet(t, xaMau, domain.LocCanBo{BoPhanID: "bp-001"}, truyVan), "nd-01", "nd-03")
		kiemIDs(t, "search 0900 "+truyVan,
			b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "0900", CongKhai: boolP(false)}, truyVan),
			"nd-01", "nd-03", "nd-05")
	}
}

// --- the text search -----------------------------------------------------------------------------

func TestTimCanBoTheoTenVaChucVuKhongPhanBietHoaThuong(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "trần thị", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "trần thị"}, "limit=100"), "nd-02")
	kiemIDs(t, "KẾ TOÁN", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "KẾ TOÁN"}, "limit=100"), "nd-03")
	kiemIDs(t, "Văn", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Văn"}, "limit=100"),
		"nd-01", "nd-03", "nd-05")
}

// `%` AND `_` ARE MATCHED LITERALLY. Unescaped, "Nguy_n" matches "Nguyễn" (`_` = any one
// character) and "Trần%B" matches "Trần Thị B" (`%` = any run) — the administrator's text would be
// a pattern language.
//
// MUTATION THAT MUST TURN THIS RED: drop thoatLike.Replace in menhDeLocCanBo.
func TestTimCanBoThoatPhanTramVaGachDuoi(t *testing.T) {
	b := moBanThuDS(t)
	for tu, muonMau := range map[string]string{
		"Nguy_n":  `%Nguy\_n%`,
		"Trần%B":  `%Trần\%B%`,
		`Văn\`:    `%Văn\\%`, // a trailing backslash must not escape the closing %
		"100% đủ": `%100\% đủ%`,
	} {
		kiemIDs(t, tu, b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: tu}, "limit=100"))
		if l := b.lenhCuoi(t); l.args[1] != muonMau {
			t.Errorf("%q: mẫu ràng buộc = %#v, muốn %#v", tu, l.args[1], muonMau)
		}
	}
}

// A TELEPHONE NUMBER IS COMPARED BY ITS DIGITS, on both sides: "0900 000 001" finds the number
// stored as "0900000001", and "0300000005" finds the one stored as "0300.000.005".
func TestTimCanBoSoDienThoaiSoChuSo(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "gõ có dấu cách", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "0900 000 001"}, "limit=100"), "nd-01")
	kiemIDs(t, "cột có dấu chấm", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "0300000005"}, "limit=100"), "nd-05")
	kiemIDs(t, "gõ nguyên dạng lưu", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "0300.000.005"}, "limit=100"), "nd-05")

	// Letters and digits together is not a number: no digit clause, so "Thôn 3" does not match
	// every number containing a 3.
	kiemIDs(t, "Thôn 3", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Thôn 3"}, "limit=100"))
	if l := b.lenhCuoi(t); strings.Contains(l.sql, "regexp_replace") {
		t.Errorf("chữ lẫn số mà vẫn so theo chữ số: %q", l.sql)
	}
}

// --- isolation and preservation on the search path ----------------------------------------------

// MUTATION THAT MUST TURN THIS RED: drop `WHERE tenant_id = $1` from Scoped.Query (core/store),
// or start menhDeLocCanBo's placeholders at $1.
func TestTimCanBoXaLaThamSoMotTuContext(t *testing.T) {
	b := moBanThuDS(t)

	// "Thị" is in a commune-B name (Vũ Thị F) as well as in commune A's.
	for _, id := range b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Thị"}, "limit=100") {
		if strings.HasPrefix(id, "ndb-") {
			t.Fatalf("tìm ở xã mẫu ra bản ghi %s của xã khác — RÒ RỈ GIỮA HAI XÃ", id)
		}
	}
	kiemIDs(t, "Vũ Thị F từ xã mẫu", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Vũ Thị F"}, "limit=100"))

	l := b.lenhCuoi(t)
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Fatalf("câu lệnh tìm kiếm thiếu ràng buộc xã: %q", l.sql)
	}
	if l.args[0] != xaMau {
		t.Errorf("$1 = %v, muốn xã %q từ context", l.args[0], xaMau)
	}
	if !strings.Contains(l.sql, "ho_ten ILIKE $2") {
		t.Errorf("mẫu tìm kiếm không ở $2 — tham số của bộ lọc phải bắt đầu sau xã: %q", l.sql)
	}
}

// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from locTomTat.
func TestTimCanBoKhongTraDongDaXoaMem(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "Phạm Thị D (đã xoá mềm)", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Phạm Thị D"}, "limit=100"))
	kiemIDs(t, "0900000004 (đã xoá mềm)", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "0900000004"}, "limit=100"))
	if l := b.lenhCuoi(t); !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("câu lệnh tìm kiếm thiếu điều kiện xoá mềm: %q", l.sql)
	}
}

func TestTimCanBoKetHopBoLoc(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "0900 + bp-001", b.timHet(t, xaMau,
		domain.LocCanBo{TuKhoa: "0900", BoPhanID: "bp-001"}, "limit=100"), "nd-01", "nd-03")
	kiemIDs(t, "Văn + published", b.timHet(t, xaMau,
		domain.LocCanBo{TuKhoa: "Văn", CongKhai: boolP(true)}, "limit=100"))
}
