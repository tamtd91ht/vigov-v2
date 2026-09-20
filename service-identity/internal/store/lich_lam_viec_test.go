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
//	              argument · the soft-delete predicate is in the statement · the order and its
//	              tie-break are in the statement · the LIMIT is the ceiling PLUS ONE · the refusal
//	              fires at ceiling+1 and returns NO rows · the positional Scan lines up with the
//	              column list BY NAME · a driver failure is wrapped rather than swallowed.
//	NOT PROVED    anything PostgreSQL does with that statement: that `date_part('epoch', …)` really
//	              yields seconds since midnight, that the CHECK constraints refuse what they say,
//	              that the partition routing works. That needs a real server, and the *_pg_test.go
//	              files next door — which SKIP without VIGOV_TEST_DSN — are where it belongs.
//
// THE DEFECT THIS FILE EXISTS FOR: `bat_dau` and `ket_thuc` are two adjacent columns of ONE type.
// Swap them between the SELECT list and the Scan and nothing fails — the week simply runs from
// closing time to opening time, the CHECK that would have caught it lives in the database and is
// never consulted on a read, and every deadline computed afterwards is wrong in a direction
// nobody can see. Every fixture below is therefore ASYMMETRIC.

const (
	// 07:30 and 11:30 as seconds since midnight. Written as arithmetic rather than as 27000 so a
	// reader can check them, and deliberately different from each other.
	gio0730 = 7*3600 + 30*60
	gio1130 = 11*3600 + 30*60
	gio1330 = 13*3600 + 30*60
	gio1700 = 17 * 3600
)

func dongCa(id string, thu, batDau, ketThuc int, ghiChu string) map[string]driver.Value {
	return map[string]driver.Value{
		"id":       id,
		"thu":      int64(thu),
		"bat_dau":  int64(batDau),
		"ket_thuc": int64(ketThuc),
		"ghi_chu":  ghiChu,
	}
}

// motTuanMau is one commune's week reduced to what the assertions need: a Monday morning and a
// Monday afternoon. Every value differs from every other, so a column read off the wrong position
// shows up as WRONG DATA rather than as a plausible zero.
func motTuanMau() []map[string]driver.Value {
	return []map[string]driver.Value{
		dongCa("llv-001", 1, gio0730, gio1130, "Buổi sáng"),
		dongCa("llv-002", 1, gio1330, gio1700, "Buổi chiều"),
	}
}

func nhieuCa(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, dongCa(fmt.Sprintf("llv-%04d", i), 1+i%7, gio0730+i, gio1700, ""))
	}
	return ra
}

// --- the statement the store builds ------------------------------------------------------------

func TestLichLamViecBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	k := khoMoi()
	k.hang = motTuanMau()

	if _, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
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

	// The same store shape, a different commune in the context, a different $1. This is what
	// "scoped repository" means in practice — a store caching the first commune it saw fails here.
	k2 := khoMoi()
	k2.hang = motTuanMau()
	const xaKhac = "01J0000000000000000000000B"
	if _, err := NewLichLamViecStore(dbGia(k2)).DanhSach(ctxXa(xaKhac)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.cuoi().args[0] != xaKhac {
		t.Errorf("$1 = %v, muốn %q", k2.cuoi().args[0], xaKhac)
	}
}

func TestLichLamViecLocDongDaXoaMemVaSapXepOnDinh(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always. A
	// deleted session that came back would be working hours the commune had removed, and a
	// deadline counted through them is a commitment nobody made.
	//
	// The order is TOTAL: UNIQUE (tenant_id, thu, bat_dau) means two calls cannot return the same
	// rows in a different sequence.
	k := khoMoi()
	k.hang = motTuanMau()

	if _, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	q := k.cuoi().sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu, bat_dau") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
}

func TestLichLamViecLayDuTranCongMot(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	k := khoMoi()
	k.hang = motTuanMau()

	if _, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(TranLichLamViec+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], TranLichLamViec+1)
	}
}

func TestLichLamViecDatTenChoHaiCotGio(t *testing.T) {
	// STRUCTURAL, AND THE CHEAPEST GUARD IN THIS FILE. `date_part('epoch', bat_dau)::int` with no
	// alias is an output column PostgreSQL calls `date_part` — twice — and a reader comparing the
	// column list against the Scan order would have nothing to compare. It would also make every
	// assertion below pass against a statement whose two times had traded places.
	k := khoMoi()
	k.hang = motTuanMau()

	if _, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	cot, err := cotTrongCauLenh(k.cuoi().sql)
	if err != nil {
		t.Fatal(err)
	}
	muon := []string{"id", "thu", "bat_dau", "ket_thuc", "ghi_chu"}
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

// --- what comes back ---------------------------------------------------------------------------

func TestLichLamViecDocDungTungCot(t *testing.T) {
	k := khoMoi()
	k.hang = motTuanMau()

	ra, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d ca, muốn 2", len(ra))
	}
	mot := ra[0]
	if mot.ID != "llv-001" || mot.GhiChu != "Buổi sáng" {
		t.Errorf("hai cột TEXT đọc sai chỗ: %+v", mot)
	}
	if mot.Thu != 1 {
		t.Errorf("thu = %d, muốn 1", mot.Thu)
	}
	// THE PAIR THAT CANNOT BE CAUGHT BY TYPE: both are integers and adjacent. The fixture gives
	// them values that are not only different but ORDERED, so a swap reads as a session ending
	// before it starts.
	if mot.BatDau != gio0730 || mot.KetThuc != gio1130 {
		t.Fatalf("bat_dau / ket_thuc đọc ngược: bat_dau=%d ket_thuc=%d, muốn %d và %d",
			mot.BatDau, mot.KetThuc, gio0730, gio1130)
	}
	if mot.KetThuc <= mot.BatDau {
		t.Fatal("ca làm việc kết thúc trước khi bắt đầu — hai cột giờ đã hoán đổi")
	}
	// The rendering is what the route publishes, and it is where seconds-since-midnight stops
	// being an implementation detail.
	if mot.BatDau.Chuoi() != "07:30:00" || mot.KetThuc.Chuoi() != "11:30:00" {
		t.Errorf("giờ hiển thị sai: %q–%q", mot.BatDau.Chuoi(), mot.KetThuc.Chuoi())
	}
}

func TestLichLamViecXaChuaCauHinhTraDanhSachRongChuKhongLoi(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE: migration 0006 creates the table and seeds nothing, and
	// the onboarding step that would sow a commune's first sessions does not exist yet.
	//
	// AN EMPTY CALENDAR IS NOT AN ERROR HERE AND IT IS NOT A DEFAULT EITHER. The store returns an
	// empty list — a list, never a nil the caller has to branch on — because the configuration
	// screen has to be able to show "chưa cấu hình". What must never happen is a caller reading it
	// as ordinary working hours; domain.VanDeCuaLich is what makes that impossible to do silently.
	k := khoMoi()

	ra, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("lịch rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d ca từ một xã chưa cấu hình", len(ra))
	}
	if vd := domain.VanDeCuaLich(ra); len(vd) != 1 || vd[0].Loai != domain.VanDeLichTrong {
		t.Fatalf("lịch rỗng phải được nêu tên là vấn đề, nhận: %+v", vd)
	}
}

// --- the ceiling ---------------------------------------------------------------------------------

func TestLichLamViecDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid.
	k := khoMoi()
	k.hang = nhieuCa(TranLichLamViec)

	ra, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranLichLamViec {
		t.Errorf("nhận %d ca, muốn %d", len(ra), TranLichLamViec)
	}
}

func TestLichLamViecVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. A silently short week is a session missing
	// from the calendar, and every deadline computed afterwards is longer than the commitment the
	// commune actually made. The rows already read are DROPPED — handing back a list the caller
	// might render anyway is how a refusal turns into a truncation one careless
	// `if err != nil { log }` later.
	k := khoMoi()
	k.hang = nhieuCa(TranLichLamViec + 1)

	ra, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if !errors.Is(err, ErrQuaNhieuCaLamViec) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuCaLamViec", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d ca", len(ra))
	}
}

// --- failures -------------------------------------------------------------------------------------

func TestLichLamViecLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Wrapped with %w, never swallowed: the caller tells the ceiling from an ordinary failure with
	// errors.Is, and that only works if the chain is intact.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc

	ra, err := NewLichLamViecStore(dbGia(k)).DanhSach(ctxXa(xaMau))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuCaLamViec) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestLichLamViecKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either query every commune's
	// rows or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover turns that
	// into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc lịch làm việc khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	k.hang = motTuanMau()
	_, _ = NewLichLamViecStore(dbGia(k)).DanhSach(context.Background())
}
