package app

// Editing a project's funding allocation set through PATCH (user decisions 06/10/2026, following the
// prototype — vigov-require budget/service.py:531-610).
//
// THE REAL USE CASE OVER THE REAL STORE OVER THE FAKE DRIVER (driver_gia_du_an_test.go), so what is
// proved is the statements and the transaction boundary. What is NOT proved here: that PostgreSQL's
// `UNIQUE (tenant_id, du_an_id, nguon_von_id)` really refuses a reinsert — the reason the revive path
// exists. That half is the floor and needs a real server.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

func allocationSet(lines ...domain.DongPhanBoMoi) *[]domain.DongPhanBoMoi { return &lines }

// editCase is a stored project (plan 100.000.000) with two live lines and one removed line.
func editCase() *khoDAGia {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.nguonVonCo = map[string]bool{"nv-a": true, "nv-b": true, "nv-c": true, "nv-old": true}
	k.lines = []storedLineFake{
		{id: "pb-a", nguon: "nv-a", soTien: 40_000_000},
		{id: "pb-b", nguon: "nv-b", soTien: 30_000_000},
		{id: "pb-old", nguon: "nv-old", soTien: 5_000_000, removed: true},
	}
	return k
}

func projectEditDelta(t *testing.T, k *khoDAGia) map[string]map[string]any {
	t.Helper()
	entries := k.cau("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d audit entries, want 1", len(entries))
	}
	raw, ok := entries[0].args[7].([]byte)
	if !ok {
		t.Fatalf("delta is %T, want []byte", entries[0].args[7])
	}
	var d map[string]map[string]any
	_ = json.Unmarshal(raw, &d) // du_an_id is a string, not a map; the two sides decode
	return d
}

// Replacement semantics: a kept source is UPDATED IN PLACE, a dropped source is SOFT DELETED, a new
// source is INSERTED — and no kept source is ever reinserted (the full unique key would refuse it).
func TestEditAllocationsReplacesInPlace(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	res, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 50_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-c", SoTien: 20_000_000},
	)}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("transactions=%d commits=%d, want 1/1 — lines and audit entry in ONE transaction", k.batDau, k.daCommit)
	}

	updates := k.cau("SET so_tien_phan_bo = $3, cap_nhat_luc")
	if len(updates) != 1 || updates[0].args[1] != "pb-a" || updates[0].args[2] != int64(50_000_000) {
		t.Errorf("in-place updates = %+v, want one on pb-a to 50000000", updates)
	}
	removes := k.cau("SET deleted_at = now(), deleted_by = $3")
	var lineRemoves []lenhGhi
	for _, r := range removes {
		if strings.Contains(r.sql, "phan_bo_nguon_von") {
			lineRemoves = append(lineRemoves, r)
		}
	}
	if len(lineRemoves) != 1 || lineRemoves[0].args[1] != "pb-b" {
		t.Fatalf("soft deletes = %+v, want one on pb-b", lineRemoves)
	}
	// RULE 6, INVARIANT 8 on `deleted_by` too: the staff business code, never an internal id.
	if lineRemoves[0].args[2] != canBo.ID {
		t.Errorf("deleted_by = %v, want %q", lineRemoves[0].args[2], canBo.ID)
	}
	if lineRemoves[0].args[3] == "" {
		t.Error("delete_reason empty — rule 7 invariant 1 names three columns")
	}
	inserts := k.cau("INSERT INTO phan_bo_nguon_von")
	if len(inserts) != 1 || inserts[0].args[3] != "nv-c" {
		t.Fatalf("inserts = %+v, want exactly one, for nv-c", inserts)
	}
	if k.coCau("deleted_at = NULL") {
		t.Error("revived a row when no removed source was re-added")
	}
	// The project row itself did not move, so it is not rewritten.
	if k.coCau("UPDATE du_an") {
		t.Error("project row rewritten although only its lines moved")
	}

	if len(res.Allocations) != 2 || res.Allocations[0].ID != "pb-a" || res.Allocations[1].NguonVonID != "nv-c" {
		t.Errorf("lines after = %+v, want pb-a kept and nv-c new", res.Allocations)
	}

	// THE AUDIT ENTRY CARRIES THE LINE SETS BEFORE AND AFTER, with totals (rule 6, invariant 5).
	d := projectEditDelta(t, k)
	if d["truoc"]["tong_phan_bo"] != float64(70_000_000) || d["sau"]["tong_phan_bo"] != float64(70_000_000) {
		t.Errorf("totals before/after = %v/%v, want 70000000/70000000", d["truoc"]["tong_phan_bo"], d["sau"]["tong_phan_bo"])
	}
	before, _ := d["truoc"]["phan_bo_nguon_von"].([]any)
	after, _ := d["sau"]["phan_bo_nguon_von"].([]any)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("delta lines before=%d after=%d, want 2/2", len(before), len(after))
	}
	if !strings.Contains(string(mustJSON(t, before)), "nv-b") || strings.Contains(string(mustJSON(t, after)), "nv-b") {
		t.Errorf("delta should list nv-b before and not after: before=%v after=%v", before, after)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Re-adding a source the project once dropped REVIVES its row; an INSERT would hit
// `UNIQUE (tenant_id, du_an_id, nguon_von_id)`, which counts soft-deleted rows.
func TestEditAllocationsRevivesRemovedSource(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 40_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-b", SoTien: 30_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-old", SoTien: 10_000_000},
	)}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	revives := k.cau("deleted_at = NULL")
	if len(revives) != 1 || revives[0].args[1] != "pb-old" || revives[0].args[2] != int64(10_000_000) {
		t.Fatalf("revives = %+v, want one on pb-old with 10000000", revives)
	}
	if k.coCau("INSERT INTO phan_bo_nguon_von") {
		t.Error("re-added source was INSERTED — the full unique key refuses that row")
	}
	// The re-added source is checked against the catalogue like any new one.
	if !k.coCau("FROM nguon_von") {
		t.Error("revived source not checked against the commune's catalogue")
	}
}

func TestEditAllocationsAboveMergedPlanIsRefused(t *testing.T) {
	t.Run("new set above the stored plan", func(t *testing.T) {
		k := editCase()
		uc, ctx := dungUseCaseDuAn(t, k)
		_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
			domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 80_000_000},
			domain.DongPhanBoMoi{NguonVonID: "nv-b", SoTien: 40_000_000},
		)}, canBo)
		var over *domain.AllocationExceedsPlanError
		if !errors.As(err, &over) || over.Overrun() != 20_000_000 {
			t.Fatalf("err = %v, want overrun of 20000000", err)
		}
		if !strings.Contains(over.Sentence(), "20.000.000 đ") {
			t.Errorf("sentence %q does not name the overrun", over.Sentence())
		}
		assertNothingWritten(t, k)
	})
	t.Run("plan revised below what is allocated", func(t *testing.T) {
		k := editCase() // live lines total 70.000.000
		uc, ctx := dungUseCaseDuAn(t, k)
		plan := domain.Dong(50_000_000)
		_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{KeHoachVonNam: &plan}, canBo)
		if !errors.Is(err, domain.ErrAllocationExceedsPlan) {
			t.Fatalf("err = %v, want ErrAllocationExceedsPlan", err)
		}
		assertNothingWritten(t, k)
	})
	t.Run("plan revised down to exactly what is allocated", func(t *testing.T) {
		k := editCase()
		uc, ctx := dungUseCaseDuAn(t, k)
		plan := domain.Dong(70_000_000)
		if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{KeHoachVonNam: &plan}, canBo); err != nil {
			t.Fatalf("Sua: %v", err)
		}
	})
	t.Run("plan and set revised together are judged together", func(t *testing.T) {
		k := editCase()
		uc, ctx := dungUseCaseDuAn(t, k)
		plan := domain.Dong(150_000_000)
		_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{KeHoachVonNam: &plan, PhanBo: allocationSet(
			domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 150_000_000},
		)}, canBo)
		if err != nil {
			t.Fatalf("Sua: %v", err)
		}
	})
}

func assertNothingWritten(t *testing.T, k *khoDAGia) {
	t.Helper()
	if k.daCommit != 0 {
		t.Errorf("commit=%d, want 0", k.daCommit)
	}
	for _, s := range []string{"UPDATE du_an", "UPDATE phan_bo_nguon_von", "INSERT INTO phan_bo_nguon_von", "INSERT INTO audit_log"} {
		if k.coCau(s) {
			t.Errorf("refused edit still ran %q", s)
		}
	}
}

// A dropped source that already carries vouchers of this project is refused, and nothing is written.
func TestEditAllocationsRefusesDroppingSourceWithVouchers(t *testing.T) {
	k := editCase()
	k.vouchersBySource = map[string]int64{"nv-b": 2}
	uc, ctx := dungUseCaseDuAn(t, k)

	_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 40_000_000},
	)}, canBo)
	if !errors.Is(err, domain.ErrSourceHasDisbursements) {
		t.Fatalf("err = %v, want ErrSourceHasDisbursements", err)
	}
	assertNothingWritten(t, k)
}

// Vouchers on a source the edit KEEPS block nothing — only removal is guarded, and an edit that removes
// nothing never reads the voucher table.
func TestEditAllocationsKeptSourceWithVouchersIsFine(t *testing.T) {
	k := editCase()
	k.vouchersBySource = map[string]int64{"nv-a": 5}
	uc, ctx := dungUseCaseDuAn(t, k)

	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 45_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-b", SoTien: 30_000_000},
	)}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("FROM chung_tu_giai_ngan") {
		t.Error("voucher table read although no source was dropped")
	}
}

func TestEditAllocationsDuplicateSourceRefusedBeforeTransaction(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 10_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 20_000_000},
	)}, canBo)
	if !errors.Is(err, domain.ErrPhanBoTrungNguon) {
		t.Fatalf("err = %v, want ErrPhanBoTrungNguon", err)
	}
	if k.batDau != 0 {
		t.Errorf("opened %d transactions, want 0", k.batDau)
	}
}

func TestEditAllocationsNewSourceMustBeInCatalogue(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-of-another-commune", SoTien: 10_000_000},
	)}, canBo)
	if !errors.Is(err, fistore.ErrKhongThayNguonVonPhanBo) {
		t.Fatalf("err = %v, want ErrKhongThayNguonVonPhanBo", err)
	}
	assertNothingWritten(t, k)
}

// The same set as stored is a no-op: no write and no audit entry, which is what keeps the PATCH
// route's idem.KhongCan declaration true.
func TestEditAllocationsSameSetIsNoOp(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	res, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet(
		domain.DongPhanBoMoi{NguonVonID: "nv-b", SoTien: 30_000_000},
		domain.DongPhanBoMoi{NguonVonID: "nv-a", SoTien: 40_000_000},
	)}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	// "UPDATE du_an" / "UPDATE phan_bo…" and not bare "UPDATE", which matches the `FOR UPDATE` lock read.
	for _, s := range []string{"UPDATE du_an", "UPDATE phan_bo_nguon_von", "INSERT INTO"} {
		if k.coCau(s) {
			t.Errorf("no-op edit ran %q", s)
		}
	}
	if len(res.Allocations) != 2 {
		t.Errorf("reply lines = %d, want the 2 live lines", len(res.Allocations))
	}
}

// Absent `funding_allocations` leaves the lines alone, and the reply still carries them.
func TestEditWithoutAllocationsLeavesLinesAlone(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	name := "Tên mới"
	res, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &name}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("phan_bo_nguon_von SET") || k.coCau("UPDATE phan_bo_nguon_von") || k.coCau("INSERT INTO phan_bo_nguon_von") {
		t.Error("lines written although the edit did not name them")
	}
	if len(res.Allocations) != 2 {
		t.Errorf("reply lines = %d, want the 2 live lines", len(res.Allocations))
	}
	d := projectEditDelta(t, k)
	if _, ok := d["truoc"]["phan_bo_nguon_von"]; ok {
		t.Error("delta lists lines although they did not move")
	}
}

// `[]` is "remove every line", not "leave alone".
func TestEditAllocationsEmptySetRemovesAll(t *testing.T) {
	k := editCase()
	uc, ctx := dungUseCaseDuAn(t, k)

	res, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{PhanBo: allocationSet()}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	n := 0
	for _, r := range k.cau("SET deleted_at = now(), deleted_by = $3") {
		if strings.Contains(r.sql, "phan_bo_nguon_von") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d lines soft deleted, want 2", n)
	}
	if len(res.Allocations) != 0 {
		t.Errorf("reply lines = %d, want 0", len(res.Allocations))
	}
}
