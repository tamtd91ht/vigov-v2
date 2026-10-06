package store

// Integration tests for a project's allocation reads and the edit path's line writes (user decisions
// 06/10/2026), against a real PostgreSQL.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES, nothing more
// (the same footing as nguon_von_pg_test.go, whose fixtures this file reuses).

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The year read and the detail read stay inside the commune even when project ids collide, skip
// soft-deleted lines, and the detail's disbursed figure counts only vouchers naming that source.
func TestPgProjectAllocationReads(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)

	themNguonVon(t, db, commune, "nv-xa", "Ngân sách xã", 1)
	themNguonVon(t, db, commune, "nv-tinh", "Ngân sách tỉnh", 2)
	themNguonVon(t, db, otherCommune, "nv-xa", "Nguồn của xã khác", 1)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themDuAnToiThieu(t, db, otherCommune, "da-1", "DA01", 2026, 100_000_000)
	themPhanBo(t, db, commune, "pb-tinh", "da-1", "nv-tinh", 30_000_000)
	themPhanBo(t, db, commune, "pb-xa", "da-1", "nv-xa", 40_000_000)
	themPhanBo(t, db, otherCommune, "pb-other", "da-1", "nv-xa", 99_000_000)
	sourceID := "nv-xa"
	themChungTu(t, db, commune, "ct-1", "da-1", 15_000_000, &sourceID)
	themChungTu(t, db, commune, "ct-none", "da-1", 7_000_000, nil)

	s := NewDuAnStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))

	year, err := s.AllocationsOfYear(ctx, LocDuAn{Nam: 2026})
	if err != nil {
		t.Fatalf("AllocationsOfYear: %v", err)
	}
	lines := year["da-1"]
	// The commune's own source order (thu_tu), not insertion order.
	if len(lines) != 2 || lines[0].SourceName != "Ngân sách xã" || lines[1].SourceName != "Ngân sách tỉnh" {
		t.Fatalf("year lines = %+v", lines)
	}

	detail, err := s.AllocationsOfProject(ctx, "da-1")
	if err != nil {
		t.Fatalf("AllocationsOfProject: %v", err)
	}
	if len(detail) != 2 || detail[0].Disbursed != 15_000_000 || detail[1].Disbursed != 0 {
		t.Fatalf("detail = %+v — want 15M on nv-xa, 0 on nv-tinh (the sourceless voucher is on no line)", detail)
	}
}

// Re-adding a removed source REVIVES its row: the full unique key (0013) would refuse an INSERT.
func TestPgReviveAllocationAfterSoftDelete(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themPhanBo(t, db, commune, "pb-1", "da-1", "nv-xa", 40_000_000)

	scoped := pkgstore.New(db)
	w := NewDuAnGhiStore(scoped)
	ctx := ctxXa(tenant.ID(commune))

	err := scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := w.SoftDeleteAllocation(ctx, tx, "pb-1", "CB-TEST", "gỡ thử"); err != nil {
			return err
		}
		stored, err := w.AllocationLinesForEdit(ctx, tx, "da-1")
		if err != nil {
			return err
		}
		if len(stored) != 1 || !stored[0].Removed {
			t.Errorf("edit read = %+v, want the removed row", stored)
		}
		return w.ReviveAllocation(ctx, tx, "pb-1", 10_000_000)
	})
	if err != nil {
		t.Fatalf("soft delete then revive: %v", err)
	}
	got, err := dungNguonVonStore(db).PhanBoTheoDuAn(ctx, "da-1")
	if err != nil {
		t.Fatalf("PhanBoTheoDuAn: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pb-1" || got[0].SoTien != domain.Dong(10_000_000) {
		t.Fatalf("after revive = %+v", got)
	}
}
