package store

// Integration test for migration 0018 (`du_an.at_risk`, ADR 0080 #2) against a real PostgreSQL.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES, nothing more
// (the same footing as nguon_von_pg_test.go, whose fixtures this file reuses).

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// An existing project reads false (the DEFAULT), the edit path writes the flag back through CapNhat, both
// read paths (the list's cotDuAn and the edit's cotDuAnGhi) see it, a colliding id of another commune is
// untouched, and the column refuses NULL.
func TestPgProjectAtRiskFlag(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 100_000_000)
	themDuAnToiThieu(t, db, otherCommune, "da-1", "DA01", 2026, 100_000_000)

	scoped := pkgstore.New(db)
	reads := NewDuAnStore(scoped)
	writes := NewDuAnGhiStore(scoped)
	ctx := ctxXa(tenant.ID(commune))

	before, err := reads.ChiTiet(ctx, "da-1")
	if err != nil {
		t.Fatalf("ChiTiet: %v", err)
	}
	if before.DuAn.AtRisk {
		t.Fatal("a project that predates the flag must read false (DEFAULT false)")
	}

	err = scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		d, err := writes.TheoIDDeSua(ctx, tx, "da-1")
		if err != nil {
			return err
		}
		if d.AtRisk {
			t.Error("edit read: want false before the write")
		}
		d.AtRisk = true
		return writes.CapNhat(ctx, tx, d)
	})
	if err != nil {
		t.Fatalf("tick the flag: %v", err)
	}

	list, err := reads.DanhSach(ctx, LocDuAn{Nam: 2026})
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(list) != 1 || !list[0].DuAn.AtRisk {
		t.Fatalf("list after tick = %+v, want the flag set", list)
	}
	other, err := reads.ChiTiet(ctxXa(tenant.ID(otherCommune)), "da-1")
	if err != nil {
		t.Fatalf("ChiTiet other commune: %v", err)
	}
	if other.DuAn.AtRisk {
		t.Fatal("another commune's project with the same id was flagged (rule 1)")
	}

	if _, err := db.Exec(`UPDATE du_an SET at_risk = NULL WHERE tenant_id = $1 AND id = 'da-1'`, commune); err == nil {
		t.Fatal("at_risk accepted NULL — 0018 must declare it NOT NULL")
	}
}
