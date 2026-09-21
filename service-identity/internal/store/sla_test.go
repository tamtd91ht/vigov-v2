package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a suite that prints `ok`
// while asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL: the fake driver in kenh_cong_dan_kho_gia_test.go records the
// statement the store built and serves rows BUILT BY COLUMN NAME out of that statement's own
// SELECT list. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the soft-delete predicate is in the statement · the order, its
//	              NULLS FIRST and its tie-break are in the statement · the LIMIT is the ceiling
//	              PLUS ONE · the refusal fires at ceiling+1 and returns NO rows · the positional
//	              Scan lines up with the column list BY NAME · a NULL `linh_vuc` becomes the
//	              default row and nothing else · an unknown `loai_viec` refuses the WHOLE read ·
//	              a driver failure is wrapped rather than swallowed.
//	NOT PROVED    anything PostgreSQL does with that statement: that UNIQUE (tenant_id,
//	              loai_viec, linh_vuc_khoa) really admits one default row per commune, that
//	              sla_linh_vuc_khong_rong really refuses '', that sla_gio_phai_duong refuses 0,
//	              that the generated column frees its slot on soft delete, that the partition
//	              routing works. That needs a real server, and sla_pg_test.go next door — which
//	              SKIPS without VIGOV_TEST_DSN — is where it belongs.
//
// THE DEFECT THIS FILE EXISTS FOR: `sla` carries FIVE ADJACENT INTEGER COLUMNS. Swap any two of
// them between the SELECT list and the Scan and NOTHING FAILS — the rows load, the screen
// renders, and the only difference is that the authority has promised something else. No CHECK
// can catch it either: all five are simply positive numbers. Every fixture below therefore gives
// all five DIFFERENT values, and no two rows share a figure.

// dongSLAMau builds one sample row. `linhVuc` empty means the DEFAULT row, which is SQL NULL —
// that mapping is the second thing this file pins.
func dongSLAMau(id, loaiViec, linhVuc string, tn, xl, sdh, bld, bct int) map[string]driver.Value {
	var lv driver.Value
	if linhVuc != "" {
		lv = linhVuc
	}
	return map[string]driver.Value{
		"id":               id,
		"loai_viec":        loaiViec,
		"linh_vuc":         lv,
		"gio_tiep_nhan":    int64(tn),
		"gio_xu_ly_xong":   int64(xl),
		"gio_sap_den_han":  int64(sdh),
		"gio_bao_lanh_dao": int64(bld),
		"gio_bao_chu_tich": int64(bct),
	}
}

// motBangSLAMau is one commune's table reduced to what the assertions need: the default row for
// petitions and the tight `an-ninh-trat-tu` row beside it. TEN DISTINCT INTEGERS across two rows,
// so a column read off the wrong position shows up as WRONG DATA rather than as a plausible
// duplicate.
func motBangSLAMau() []map[string]driver.Value {
	return []map[string]driver.Value{
		dongSLAMau("sla-pa-mac-dinh", "phan-anh", "", 8, 56, 24, 25, 49),
		dongSLAMau("sla-pa-an-ninh", "phan-anh", "an-ninh-trat-tu", 2, 16, 4, 9, 17),
	}
}

func nhieuDongSLA(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, dongSLAMau(
			fmt.Sprintf("sla-%04d", i), "phan-anh", fmt.Sprintf("linh-vuc-%04d", i),
			1+i, 2+i, 3+i, 4+i, 5+i))
	}
	return ra
}

// --- the statement the store builds ------------------------------------------------------------

func TestSLABuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. It matters more here than on most
	// tables — an SLA row read across the boundary would make one commune's promise to its
	// citizens be computed from another commune's policy.
	k := khoMoi()
	k.hang = motBangSLAMau()

	if _, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != xaMau {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaMau)
	}

	// The same store shape, a different commune in the context, a different $1. A store caching
	// the first commune it saw fails here.
	k2 := khoMoi()
	k2.hang = motBangSLAMau()
	const xaKhac = "01J0000000000000000000000B"
	if _, err := NewSLAStore(dbGia(k2)).DanhSach(ctxXa(xaKhac)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.cuoi().args[0] != xaKhac {
		t.Errorf("$1 = %v, muốn %q", k2.cuoi().args[0], xaKhac)
	}
}

func TestSLALocDongDaXoaMemVaSapXepOnDinh(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always. A
	// deleted SLA row that came back is a deadline the commune removed still being promised.
	//
	// NULLS FIRST IS ASSERTED, NOT ASSUMED. PostgreSQL's ASC default is NULLS LAST, so leaving
	// it off would put the default row at the END for one kind of work and the ordering would
	// stop matching the index the migration declares — two statements' worth of drift that no
	// test of the returned rows could see while every commune has one default row.
	k := khoMoi()
	k.hang = motBangSLAMau()

	if _, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	q := k.cuoi().sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY loai_viec, linh_vuc NULLS FIRST") {
		t.Errorf("thứ tự không ổn định hoặc thiếu NULLS FIRST: %q", q)
	}
}

func TestSLALayDuTranCongMot(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all.
	k := khoMoi()
	k.hang = motBangSLAMau()

	if _, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(TranSLA+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], TranSLA+1)
	}
}

func TestSLADanhSachCotKhopTungDichCuaScan(t *testing.T) {
	// THE CHEAPEST GUARD IN THIS FILE, AND THE ONE THE TABLE MOST NEEDS. Five of the eight
	// columns are integers sitting next to each other; the type system cannot tell them apart
	// and neither can PostgreSQL. This pins the SELECT list the store really built against the
	// order DanhSach scans in.
	k := khoMoi()
	k.hang = motBangSLAMau()

	if _, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	cot, err := cotTrongCauLenh(k.cuoi().sql)
	if err != nil {
		t.Fatal(err)
	}
	muon := []string{
		"id", "loai_viec", "linh_vuc",
		"gio_tiep_nhan", "gio_xu_ly_xong", "gio_sap_den_han",
		"gio_bao_lanh_dao", "gio_bao_chu_tich",
	}
	if len(cot) != len(muon) {
		t.Fatalf("danh sách cột = %v, muốn %v", cot, muon)
	}
	for i := range muon {
		if cot[i] != muon[i] {
			t.Fatalf("cột %d = %q, muốn %q — thứ tự này phải khớp từng đích của Scan",
				i, cot[i], muon[i])
		}
	}
}

func TestSLAKhongCoaLesceLinhVucTrongSQL(t *testing.T) {
	// STRUCTURAL, AND IT GUARDS THE ORDER BY RATHER THAN THE DATA. PostgreSQL resolves a bare
	// name in ORDER BY against the OUTPUT column list first, so an output column named
	// `linh_vuc` that was really `coalesce(linh_vuc, '')` would make NULLS FIRST decorative —
	// there would be no NULL left to sort. The NULL is folded into "" once, in Go.
	k := khoMoi()
	k.hang = motBangSLAMau()

	if _, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if strings.Contains(strings.ToLower(k.cuoi().sql), "coalesce(linh_vuc") {
		t.Errorf("linh_vuc bị COALESCE trong SQL — NULLS FIRST mất tác dụng: %q", k.cuoi().sql)
	}
}

// --- what comes back ---------------------------------------------------------------------------

func TestSLADocDungTungCot(t *testing.T) {
	k := khoMoi()
	k.hang = motBangSLAMau()

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d dòng, muốn 2", len(ra))
	}

	// The tight row, where every one of the five figures differs from every other. A swapped
	// pair reads as a different promise, and this is the only place it can be seen.
	an := ra[1]
	if an.ID != "sla-pa-an-ninh" || an.LoaiViec != domain.LoaiViecPhanAnh {
		t.Fatalf("hai cột TEXT đọc sai chỗ: %+v", an)
	}
	if an.LinhVuc != "an-ninh-trat-tu" {
		t.Errorf("linh_vuc = %q", an.LinhVuc)
	}
	if an.GioTiepNhan != 2 {
		t.Errorf("gio_tiep_nhan = %d, muốn 2 — ĐÂY LÀ CAM KẾT 2 GIỜ LÀM VIỆC CỦA AN NINH TRẬT TỰ",
			an.GioTiepNhan)
	}
	if an.GioXuLyXong != 16 {
		t.Errorf("gio_xu_ly_xong = %d, muốn 16", an.GioXuLyXong)
	}
	if an.GioSapDenHan != 4 {
		t.Errorf("gio_sap_den_han = %d, muốn 4", an.GioSapDenHan)
	}
	if an.GioBaoLanhDao != 9 {
		t.Errorf("gio_bao_lanh_dao = %d, muốn 9", an.GioBaoLanhDao)
	}
	if an.GioBaoChuTich != 17 {
		t.Errorf("gio_bao_chu_tich = %d, muốn 17", an.GioBaoChuTich)
	}
}

func TestSLALinhVucNULLDocRaDongMacDinh(t *testing.T) {
	// NULL IS THE DEFAULT ROW. The schema refuses the empty string outright
	// (sla_linh_vuc_khong_rong), so "" arriving in Go can only have come from a NULL — which is
	// what lets the domain use one plain string instead of a pointer.
	k := khoMoi()
	k.hang = motBangSLAMau()

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if !ra[0].LaDongMacDinh() {
		t.Fatalf("dòng linh_vuc NULL không đọc ra dòng mặc định: %+v", ra[0])
	}
	if ra[0].LinhVuc != "" {
		t.Errorf("linh_vuc = %q, muốn rỗng", ra[0].LinhVuc)
	}
	// And the whole point of having a default row: an unconfigured field falls back to it
	// instead of being refused.
	d, co := domain.DongTheoLinhVuc(ra, domain.LoaiViecPhanAnh, "y-te-giao-duc")
	if !co || d.ID != "sla-pa-mac-dinh" {
		t.Errorf("không rơi về dòng mặc định: %+v (tìm thấy=%v)", d, co)
	}
}

func TestSLAXaChuaCauHinhTraDanhSachRongChuKhongLoi(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE: migration 0008 creates the table and seeds nothing, and
	// the onboarding step that would sow a commune's first rows does not exist.
	//
	// The store returns an empty list — a list, never a nil the caller has to branch on —
	// because the configuration screen has to be able to show "chưa cấu hình". What must never
	// happen is a caller reading it as "no deadline applies".
	k := khoMoi()

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("bảng rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa cấu hình", len(ra))
	}
	if vd := domain.VanDeCuaSLA(ra); len(vd) != 1 || vd[0].Loai != domain.VanDeSLATrong {
		t.Fatalf("bảng rỗng phải được nêu tên là vấn đề, nhận: %+v", vd)
	}
}

// --- the refusals ---------------------------------------------------------------------------------

func TestSLALoaiViecLaThiTuChoiCaLuotDoc(t *testing.T) {
	// The database should have refused the value (sla_loai_viec_hop_le), so reaching this means
	// the CHECK is gone. REFUSING THE WHOLE READ IS THE POINT: skipping the unknown row would
	// hand back a table that looks complete while a kind of work is missing from it, and no
	// screen showing that table could tell.
	k := khoMoi()
	k.hang = []map[string]driver.Value{
		dongSLAMau("sla-pa-mac-dinh", "phan-anh", "", 8, 56, 24, 25, 49),
		dongSLAMau("sla-la-hoac", "giai-ngan", "", 8, 40, 24, 25, 49),
	}

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if !errors.Is(err, ErrLoaiViecLa) {
		t.Fatalf("lỗi = %v, muốn ErrLoaiViecLa", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
	// The unknown value itself must NOT be in the message: it came from the database and an
	// error travels into logs and back to clients (rule 3, forbidden #3).
	if strings.Contains(err.Error(), "giai-ngan") {
		t.Errorf("giá trị từ CSDL lọt vào thông báo lỗi: %v", err)
	}
}

func TestSLADungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose configuration is perfectly valid.
	k := khoMoi()
	k.hang = nhieuDongSLA(TranSLA)

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranSLA {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranSLA)
	}
}

func TestSLAVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// REFUSE, DO NOT TRUNCATE — and here truncation has a second victim beyond the missing
	// rows. DongTheoLinhVuc falls back to the DEFAULT row, so a truncated read that dropped a
	// field's own row does not fail: it quietly answers with the default deadline. The commune
	// promised 16 working hours and the software promises 56, nothing errors, and the
	// difference reaches the citizen.
	k := khoMoi()
	k.hang = nhieuDongSLA(TranSLA + 1)

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if !errors.Is(err, ErrQuaNhieuDongSLA) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuDongSLA", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

func TestSLALoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Wrapped with %w, never swallowed: the caller tells the ceiling from an ordinary failure
	// with errors.Is, and that only works if the chain is intact.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc

	ra, err := NewSLAStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuDongSLA) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestSLAKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either query every commune's
	// deadlines or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover
	// turns that into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc bảng SLA khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	k.hang = motBangSLAMau()
	_, _ = NewSLAStore(dbGia(k)).DanhSach(context.Background())
}
