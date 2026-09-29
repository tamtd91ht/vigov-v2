package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for migration 0022 and NhanLinhVucStore's write path.
//
// ⚠ EVERY TEST HERE SKIPS unless VIGOV_TEST_DSN is set, and the package still prints `ok`. The
// always-running halves are migrations/petition_field_switches_test.go and
// internal/app/petition_field_catalogue_test.go (fake driver).

func upsertInTx(t *testing.T, s *NhanLinhVucStore, db *pkgstore.DB, commune string, n domain.NhanLinhVuc) error {
	t.Helper()
	ctx := tenant.Into(context.Background(), tenant.ID(commune))
	return db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if _, _, err := s.ForUpdate(ctx, tx, n.Ma); err != nil {
			return err
		}
		return s.Upsert(ctx, tx, n)
	})
}

// TestPgFieldOverrideUpsertAndInherit — insert, overwrite in place, and NULL as the inherit spelling;
// the second commune never sees the first commune's row.
func TestPgFieldOverrideUpsertAndInherit(t *testing.T) {
	raw := moKetNoi(t)
	db := pkgstore.New(raw)
	s := NewNhanLinhVucStore(db)
	communeA, communeB := xaRieng(t)

	if err := upsertInTx(t, s, db, communeA, domain.NhanLinhVuc{Ma: "rac-thai", Nhan: "Rác xã A", SortOrder: 1, Enabled: false}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := upsertInTx(t, s, db, communeA, domain.NhanLinhVuc{Ma: "rac-thai", Nhan: "", SortOrder: 0, Enabled: true}); err != nil {
		t.Fatalf("overwrite back to inherit: %v", err)
	}
	got, err := s.DanhSach(ctxXa(tenant.ID(communeA)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Nhan != "" || got[0].SortOrder != 0 || !got[0].Enabled {
		t.Errorf("after overwrite: %+v — want one row, NULL label, 0, enabled", got)
	}
	other, err := s.DanhSach(ctxXa(tenant.ID(communeB)))
	if err != nil || len(other) != 0 {
		t.Errorf("the second commune sees %d rows (%v)", len(other), err)
	}
}

// TestPgFieldOverrideRefusesASoftDeletedRow — UNIQUE (tenant_id, ma) counts soft-deleted rows; the
// write must refuse, not revive.
func TestPgFieldOverrideRefusesASoftDeletedRow(t *testing.T) {
	raw := moKetNoi(t)
	db := pkgstore.New(raw)
	s := NewNhanLinhVucStore(db)
	commune, _ := xaRieng(t)
	if _, err := raw.Exec(`INSERT INTO nhan_linh_vuc (tenant_id, id, ma, nhan, deleted_at, deleted_by, delete_reason)
		VALUES ($1, 'nlv-del', 'giao-thong', 'Cũ', now(), 'CB-00001', 'test')`, commune); err != nil {
		t.Fatal(err)
	}
	err := upsertInTx(t, s, db, commune, domain.NhanLinhVuc{Ma: "giao-thong", Enabled: false})
	if !errors.Is(err, ErrFieldOverrideSoftDeleted) {
		t.Fatalf("err = %v, want ErrFieldOverrideSoftDeleted", err)
	}
}

// TestPgFieldOverrideChecks — the two CHECKs of 0022, by SQLSTATE 23514.
func TestPgFieldOverrideChecks(t *testing.T) {
	raw := moKetNoi(t)
	commune, _ := xaRieng(t)
	for name, stmt := range map[string]string{
		"blank label":    `INSERT INTO nhan_linh_vuc (tenant_id, id, ma, nhan) VALUES ($1, 'a', 'x-1', '  ')`,
		"label over 100": `INSERT INTO nhan_linh_vuc (tenant_id, id, ma, nhan) VALUES ($1, 'b', 'x-2', '` + strings.Repeat("a", 101) + `')`,
		"negative order": `INSERT INTO nhan_linh_vuc (tenant_id, id, ma, thu_tu) VALUES ($1, 'c', 'x-3', -1)`,
	} {
		_, err := raw.Exec(stmt, commune)
		canMaLoi(t, err, "23514", name)
	}
	// A NULL label is legal — a row that only switches or reorders.
	if _, err := raw.Exec(`INSERT INTO nhan_linh_vuc (tenant_id, id, ma, enabled) VALUES ($1, 'd', 'x-4', false)`, commune); err != nil {
		t.Errorf("a switch-only row was refused: %v", err)
	}
}
