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

// WHAT THIS FILE PROVES, BEYOND THE LIST ngay_nghi_le_test.go ALREADY COVERS: this read carries
// TWO pairs of adjacent same-typed columns — `ngay`/`ten` (text) and `bat_dau`/`ket_thuc` (int) —
// and the refusal has to name each conflicting date ONCE even though a swap day with a lunch break
// is two rows on that date.

func dongLamBu(id, ngay string, batDau, ketThuc int, ten string, trungNghiLe bool) map[string]driver.Value {
	return map[string]driver.Value{
		"id":            id,
		"ngay":          ngay,
		"bat_dau":       int64(batDau),
		"ket_thuc":      int64(ketThuc),
		"ten":           ten,
		"trung_nghi_le": trungNghiLe,
	}
}

// mauLamBu is one swap day with a lunch break — TWO rows on one date, which is the shape that
// makes the DISTINCT on the other side of this join necessary and the per-date de-duplication
// necessary here.
func mauLamBu() []map[string]driver.Value {
	return []map[string]driver.Value{
		dongLamBu("nlb-001", "2026-02-21", gio0730, gio1130, "Làm bù nghỉ Tết", false),
		dongLamBu("nlb-002", "2026-02-21", gio1330, gio1700, "Làm bù nghỉ Tết", false),
	}
}

func nhieuLamBu(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, dongLamBu(fmt.Sprintf("nlb-%04d", i),
			fmt.Sprintf("2026-%02d-%02d", 1+i%12, 1+i%28), gio0730+i, gio1700,
			fmt.Sprintf("Làm bù %d", i), false))
	}
	return ra
}

// --- the statement --------------------------------------------------------------------------------

func TestNgayLamBuBuocXaVaoThamSoMotTuContextVaKhoaJoinTheoXa(t *testing.T) {
	k := khoMoi()
	k.hang = mauLamBu()

	if _, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau); err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	l := k.cuoi()
	if l.args[0] != xaMau {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaMau)
	}
	if !strings.Contains(l.sql, "WHERE lb.tenant_id = $1") {
		t.Errorf("bảng chính không lọc theo xã: %q", l.sql)
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("bảng nghỉ lễ trong join không lọc theo xã: %q", l.sql)
	}
	if !strings.Contains(l.sql, "nl.tenant_id = lb.tenant_id") {
		t.Errorf("join không buộc cùng một xã: %q", l.sql)
	}
	if n := strings.Count(l.sql, "deleted_at IS NULL"); n < 2 {
		t.Errorf("chỉ có %d điều kiện xoá mềm, cần ở cả hai bảng: %q", n, l.sql)
	}
	if !strings.Contains(l.sql, "ORDER BY lb.ngay, lb.bat_dau") {
		t.Errorf("thứ tự không ổn định: %q", l.sql)
	}
	if len(l.args) < 3 || l.args[1] != int64(namMau) ||
		l.args[2] != int64(TranNgayLamBuMotNam+1) {
		t.Errorf("tham số = %v, muốn năm %d rồi trần+1 = %d",
			l.args, namMau, TranNgayLamBuMotNam+1)
	}
}

func TestNgayLamBuDatTenChoHaiCotGio(t *testing.T) {
	// STRUCTURAL: without the aliases both computed columns are called `date_part`, and the Scan
	// order has nothing to line up against. The order asserted here is the order the Scan reads.
	k := khoMoi()
	k.hang = mauLamBu()

	if _, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau); err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	cot, err := cotTrongCauLenh(k.cuoi().sql)
	if err != nil {
		t.Fatal(err)
	}
	muon := []string{"id", "ngay", "bat_dau", "ket_thuc", "ten", "trung_nghi_le"}
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

// --- what comes back --------------------------------------------------------------------------------

func TestNgayLamBuDocDungTungCot(t *testing.T) {
	k := khoMoi()
	k.hang = mauLamBu()

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("TheoNam lỗi: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d ca làm bù, muốn 2", len(ra))
	}
	mot := ra[0]
	// PAIR ONE — two adjacent TEXT values. A swap prints a date where the announcement's title
	// belongs, which is the string an inspection reads.
	if mot.ID != "nlb-001" || mot.Ngay != "2026-02-21" || mot.Ten != "Làm bù nghỉ Tết" {
		t.Fatalf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	// PAIR TWO — two adjacent integers. A swap makes the session run from closing time to opening
	// time, and nothing on a read ever consults the CHECK that would have caught it.
	if mot.BatDau != gio0730 || mot.KetThuc != gio1130 {
		t.Fatalf("bat_dau / ket_thuc đọc ngược: %d–%d, muốn %d–%d",
			mot.BatDau, mot.KetThuc, gio0730, gio1130)
	}
	if ra[1].BatDau != gio1330 || ra[1].KetThuc != gio1700 {
		t.Errorf("ca chiều sai: %+v", ra[1])
	}
	// The two sessions of one swap day do NOT overlap — the gap between them is the lunch break,
	// and reporting touching-or-separate sessions as a clash would bury the real ones.
	if vd := domain.CaLamBuChongNhau(ra); len(vd) != 0 {
		t.Errorf("ca sáng và ca chiều bị báo chồng nhau: %+v", vd)
	}
}

func TestNgayLamBuNamKhongCoNgayNaoLaBinhThuong(t *testing.T) {
	// AN EMPTY YEAR IS ORDINARY HERE, unlike an empty weekly calendar: most years, most communes
	// work no swap day at all. Nothing downstream may read this as "calendar not configured".
	k := khoMoi()

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("năm không có ngày làm bù phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil || len(ra) != 0 {
		t.Fatalf("muốn lát rỗng, nhận %v", ra)
	}
	if vd := domain.CaLamBuChongNhau(ra); len(vd) != 0 {
		t.Errorf("năm rỗng mà báo vấn đề: %+v", vd)
	}
}

// --- the refusal ---------------------------------------------------------------------------------------

func TestNgayLamBuTrungNgayNghiLeThiTuChoiVaNeuMoiNgayMotLan(t *testing.T) {
	// THE MIRROR OF THE HOLIDAY ROUTE'S REFUSAL, and it exists because refusing on one side only
	// would leave this screen looking healthy while the other one complained — the commune would
	// fix nothing.
	//
	// THE DE-DUPLICATION IS THE SECOND HALF: a swap day with a lunch break is TWO rows on one
	// date, so a naive collector names 2026-02-21 twice in a message a person has to read.
	k := khoMoi()
	k.hang = []map[string]driver.Value{
		dongLamBu("nlb-001", "2026-02-21", gio0730, gio1130, "Làm bù nghỉ Tết", true),
		dongLamBu("nlb-002", "2026-02-21", gio1330, gio1700, "Làm bù nghỉ Tết", true),
		dongLamBu("nlb-003", "2026-03-07", gio0730, gio1130, "Làm bù Giỗ Tổ", false),
	}

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if !errors.As(err, &xungDot) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiNgayVuaNghiVuaLamBu", err)
	}
	if len(xungDot.Ngay) != 1 || xungDot.Ngay[0] != "2026-02-21" {
		t.Fatalf("nêu %v, muốn đúng một lần [2026-02-21]", xungDot.Ngay)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- the ceiling ------------------------------------------------------------------------------------------

func TestNgayLamBuDungTranThiVanTraDu(t *testing.T) {
	k := khoMoi()
	k.hang = nhieuLamBu(TranNgayLamBuMotNam)

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranNgayLamBuMotNam {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranNgayLamBuMotNam)
	}
}

func TestNgayLamBuVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	k := khoMoi()
	k.hang = nhieuLamBu(TranNgayLamBuMotNam + 1)

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if !errors.Is(err, ErrQuaNhieuNgayLamBu) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuNgayLamBu", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures -----------------------------------------------------------------------------------------------

func TestNgayLamBuLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc

	ra, err := NewNgayLamBuStore(dbGia(k)).TheoNam(ctxXa(xaMau), namMau)
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestNgayLamBuKhongCoXaTrongContextThiPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc ngày làm bù khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	k.hang = mauLamBu()
	_, _ = NewNgayLamBuStore(dbGia(k)).TheoNam(context.Background(), namMau)
}
