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

// WHAT THIS FILE PROVES: the same list as lich_lam_viec_test.go — commune from the context, soft
// delete, stable order, ceiling plus one, Scan lined up by column NAME, errors wrapped — plus the
// two things this read carries that no catalogue does:
//
//  1. THE YEAR WINDOW. This table grows with TIME, so the read is bounded by a year rather than by
//     the commune's shape. The window has to reach the statement as a RANGE (the index can use it)
//     and the year has to be the caller's, not a default.
//  2. THE REFUSAL. A date recorded BOTH as a holiday and as a swap working day is a configuration
//     error, and this store refuses rather than picking a winner (migration 0006:254). Nothing in
//     PostgreSQL can enforce that — it spans two tables — so if it is not asserted here it is not
//     asserted anywhere.
//
// NOT PROVED: that PostgreSQL's `to_char`, `make_date` and the LEFT JOIN behave as assumed. That
// needs a real server; the *_pg_test.go files next door are where it belongs, and they SKIP
// without VIGOV_TEST_DSN.

const namMau = 2026

func dongNghiLe(id, ngay, ten string, trungLamBu bool) map[string]driver.Value {
	return map[string]driver.Value{
		"id":           id,
		"ngay":         ngay,
		"ten":          ten,
		"trung_lam_bu": trungLamBu,
	}
}

// mauNghiLe is one commune's holidays reduced to what the assertions need. Every value differs
// from every other: `ngay` and `ten` are two adjacent TEXT columns, and a swap between them shows
// up as a date where the holiday's name belongs.
func mauNghiLe() []map[string]driver.Value {
	return []map[string]driver.Value{
		dongNghiLe("nnl-001", "2026-01-01", "Tết Dương lịch", false),
		dongNghiLe("nnl-002", "2026-09-02", "Quốc khánh", false),
	}
}

func nhieuNghiLe(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, dongNghiLe(fmt.Sprintf("nnl-%04d", i),
			fmt.Sprintf("2026-%02d-%02d", 1+i%12, 1+i%28), fmt.Sprintf("Ngày nghỉ %d", i), false))
	}
	return ra
}

// --- the statement ------------------------------------------------------------------------------

func TestNgayNghiLeBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. `nam` is the only thing the caller chooses; the commune arrives
	// in the context and Scoped.QueryJoin binds it to $1.
	k := khoMoi()
	k.hang = mauNghiLe()

	if _, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau); err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	l := k.cuoi()
	if l.args[0] != xaMau {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaMau)
	}
	if !strings.Contains(l.sql, "WHERE nl.tenant_id = $1") {
		t.Errorf("bảng chính không lọc theo xã: %q", l.sql)
	}
	// THE JOINED TABLE IS CONSTRAINED TWICE, AND BOTH HALVES MATTER. The derived table filters on
	// $1 so no other commune's partition is scanned; the ON clause ties the two rows to the SAME
	// commune, because 02/09 is a holiday in every commune in the country and joining on the date
	// alone would let another authority's swap day flag this one's holiday. No single-commune test
	// would ever show that.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("bảng ngày làm bù trong join không lọc theo xã: %q", l.sql)
	}
	if !strings.Contains(l.sql, "lb.tenant_id = nl.tenant_id") {
		t.Errorf("join không buộc cùng một xã: %q", l.sql)
	}
}

func TestNgayNghiLeLocDongDaXoaMemOCaHaiBangVaSapXepOnDinh(t *testing.T) {
	// RULE 7, INVARIANT 2, IN BOTH TABLES. Missing on the main table, a deleted holiday comes back
	// and a deadline is counted through a day the office was open. Missing inside the join, a
	// DELETED swap day keeps flagging a conflict — and the commune could never make its calendar
	// readable again, because removing the offending row would change nothing.
	k := khoMoi()
	k.hang = mauNghiLe()

	if _, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau); err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	q := k.cuoi().sql
	if n := strings.Count(q, "deleted_at IS NULL"); n < 2 {
		t.Errorf("chỉ có %d điều kiện xoá mềm, cần ở cả hai bảng: %q", n, q)
	}
	if !strings.Contains(q, "ORDER BY nl.ngay") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// DISTINCT IS LOAD-BEARING AND NOT A HABIT: `ngay_lam_bu` holds one row per SESSION, so a swap
	// day with a lunch break is two rows on one date. Without it the join returns the holiday
	// twice and the commune's own list grows a row nobody entered.
	if !strings.Contains(q, "SELECT DISTINCT") {
		t.Errorf("thiếu DISTINCT — một ngày làm bù hai ca sẽ nhân đôi dòng nghỉ lễ: %q", q)
	}
}

func TestNgayNghiLeDocDungMotNamTheoKhoangChuKhongPhaiHamTrenCot(t *testing.T) {
	// THE WINDOW IS THE CALLER'S YEAR, BOUND AS $2 — never a default this code picks, and never
	// the whole table: `ngay_nghi_le` grows with time, so "all of it" is a list with no upper
	// bound.
	//
	// A RANGE AND NOT `EXTRACT(YEAR FROM ngay) = $2`: a function on the column cannot use the index
	// migration 0006:231 creates, and the two forms read the same rows.
	k := khoMoi()
	k.hang = mauNghiLe()

	if _, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau); err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	l := k.cuoi()
	if len(l.args) < 2 || l.args[1] != int64(namMau) {
		t.Fatalf("$2 = %v, muốn năm %d", l.args[1:], namMau)
	}
	for _, can := range []string{"nl.ngay >= make_date($2, 1, 1)", "nl.ngay < make_date($2 + 1, 1, 1)"} {
		if !strings.Contains(l.sql, can) {
			t.Errorf("thiếu %q trong câu lệnh: %q", can, l.sql)
		}
	}
	if strings.Contains(l.sql, "EXTRACT(YEAR") || strings.Contains(l.sql, "date_part('year'") {
		t.Errorf("lọc năm bằng hàm trên cột — chỉ mục theo ngày sẽ không dùng được: %q", l.sql)
	}
	if len(l.args) < 3 || l.args[2] != int64(TranNgayNghiLeMotNam+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[2:], TranNgayNghiLeMotNam+1)
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestNgayNghiLeDocDungTungCot(t *testing.T) {
	k := khoMoi()
	k.hang = mauNghiLe()

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d ngày nghỉ, muốn 2", len(ra))
	}
	mot := ra[0]
	// `ngay` AND `ten` ARE TWO ADJACENT TEXT COLUMNS: swapping them produces no error at all, only
	// a date printed where a holiday's name belongs on the commune's own screen.
	if mot.ID != "nnl-001" || mot.Ngay != "2026-01-01" || mot.Ten != "Tết Dương lịch" {
		t.Fatalf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	if ra[1].Ngay != "2026-09-02" || ra[1].Ten != "Quốc khánh" {
		t.Errorf("dòng thứ hai sai: %+v", ra[1])
	}
}

func TestNgayNghiLeNamChuaNhapTraDanhSachRong(t *testing.T) {
	// An empty year is a legitimate answer — and today it is the only one, because migration 0006
	// seeds nothing. It is a LIST, never a nil the caller has to branch on, and it is NOT the same
	// statement as an empty weekly calendar: a commune with no holidays still has working hours.
	k := khoMoi()

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("năm rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil || len(ra) != 0 {
		t.Fatalf("muốn lát rỗng, nhận %v", ra)
	}
}

// --- the refusal ------------------------------------------------------------------------------------

func TestNgayNghiLeTrungNgayLamBuThiTuChoiChuKhongChonBen(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one the schema cannot hold: a date recorded as BOTH a
	// closure and a working day is a configuration error, not a puzzle to resolve with a
	// precedence rule. Either winner is defensible and both are wrong — one of two visible
	// configuration rows would quietly do nothing, and nobody would ever see which
	// (migration 0006:254).
	k := khoMoi()
	k.hang = []map[string]driver.Value{
		dongNghiLe("nnl-001", "2026-01-01", "Tết Dương lịch", false),
		dongNghiLe("nnl-002", "2026-02-14", "Nghỉ bù Tết", true),
		dongNghiLe("nnl-003", "2026-04-30", "Ngày Giải phóng", true),
	}

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if !errors.As(err, &xungDot) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiNgayVuaNghiVuaLamBu", err)
	}
	// EVERY conflicting date, so whoever fixes the configuration is not sent back for a second
	// round after correcting the first one.
	muon := []string{"2026-02-14", "2026-04-30"}
	if len(xungDot.Ngay) != len(muon) {
		t.Fatalf("nêu %v, muốn %v", xungDot.Ngay, muon)
	}
	for i := range muon {
		if xungDot.Ngay[i] != muon[i] {
			t.Errorf("ngày xung đột %d = %q, muốn %q", i, xungDot.Ngay[i], muon[i])
		}
	}
	// NO ROWS COME BACK WITH THE REFUSAL. A list handed over alongside the error is a caller
	// picking a winner after all, one careless `if err != nil { log }` later.
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
	// The sentence names the day somebody has to go and correct. A date is not personal data
	// (rule 3) and a refusal nobody can act on is a refusal that gets worked around.
	if !strings.Contains(xungDot.Error(), "2026-02-14") {
		t.Errorf("thông báo không nêu ngày: %q", xungDot.Error())
	}
}

// --- the ceiling --------------------------------------------------------------------------------------

func TestNgayNghiLeDungTranThiVanTraDu(t *testing.T) {
	k := khoMoi()
	k.hang = nhieuNghiLe(TranNgayNghiLeMotNam)

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranNgayNghiLeMotNam {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranNgayNghiLeMotNam)
	}
}

func TestNgayNghiLeVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// The ceiling is a leap year's worth of dates and UNIQUE (tenant_id, ngay) makes it
	// unreachable while the window works — which is exactly why tripping it must REFUSE: it can
	// only mean the year filter itself is broken.
	k := khoMoi()
	k.hang = nhieuNghiLe(TranNgayNghiLeMotNam + 1)

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if !errors.Is(err, ErrQuaNhieuNgayNghiLe) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuNgayNghiLe", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures -------------------------------------------------------------------------------------------

func TestNgayNghiLeLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc

	ra, err := NewNgayNghiLeStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if errors.As(err, &xungDot) || errors.Is(err, ErrQuaNhieuNgayNghiLe) {
		t.Error("lỗi kho bị nhận nhầm là xung đột lịch hoặc vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestNgayNghiLeKhongCoXaTrongContextThiPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc ngày nghỉ lễ khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	k.hang = mauNghiLe()
	_, _ = NewNgayNghiLeStore(dbGia(k)).TheoNam(context.Background(), namMau)
}
