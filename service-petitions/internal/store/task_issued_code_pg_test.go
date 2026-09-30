package store

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for the issued-code ledger `task_issued_code` (migration 0015) under migration
// 0024, which made `nhiem_vu.ma` immutable again (ADR 0065 NV3, user decision 30/09/2026).
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. Every property here is enforced by the DATABASE alone.
//
// WHAT CHANGED ON 30/09: the rename path (PATCH `code`, store.ChangeCode) is gone, so no test here
// renames through the store. A code "renamed away from" while that path was live (28/09 → 30/09) is
// simulated the way it would sit on disk: a ledger row issued to the task, which the task no longer
// carries.

// TestPgInsertRegistersCode — creating a task issues its code into the ledger (the AFTER INSERT
// trigger), and the minting high-water mark reads it.
func TestPgInsertRegistersCode(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-ic-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}

	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	ctx := ctxXa(tenant.ID(xa))
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		taken, err := s.MaDaDung(ctx, tx, "NV01")
		if err != nil {
			return err
		}
		if !taken {
			t.Error("trigger chèn không cấp NV01 vào sổ mã đã cấp")
		}
		max, err := s.SoLonNhatDaCap(ctx, tx)
		if err != nil {
			return err
		}
		if max != 1 {
			t.Errorf("số lớn nhất đã cấp = %d, muốn 1", max)
		}
		return nil
	}); err != nil {
		t.Fatalf("đọc sổ mã: %v", err)
	}
}

// TestPgFormerCodeCannotBeReissued — a code a task was renamed away from while the edit path was live
// sits on no row of the register, only in the ledger. It must still be refused to a NEW task (the
// insert trigger hits the ledger's key) and still count as taken for the readable 409.
func TestPgFormerCodeCannotBeReissued(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-rt-1", "NV05", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("thêm NV05: %v", err)
	}
	// NV01: issued to nv-rt-1 before a 28/09-era rename moved it to NV05.
	if _, err := db.Exec(`INSERT INTO task_issued_code (tenant_id, code, task_id) VALUES ($1, 'NV01', 'nv-rt-1')`,
		xa); err != nil {
		t.Fatalf("ghi mã cũ vào sổ: %v", err)
	}

	canMaLoi(t, themNhiemVu(db, xa, "nv-rt-2", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil),
		"23505", "một nhiệm vụ MỚI nhận lại NV01 đã đổi đi — biên bản ghi NV01 sẽ chỉ sang việc khác")

	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	ctx := ctxXa(tenant.ID(xa))
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		taken, err := s.MaDaDung(ctx, tx, "NV01")
		if err != nil {
			return err
		}
		if !taken {
			t.Error("mã cũ NV01 không còn tính là đã cấp — sẽ bị cấp lại")
		}
		return nil
	}); err != nil {
		t.Fatalf("đọc sổ mã: %v", err)
	}
}

// TestPgIssuedCodeLedgerIsAppendOnly — a ledger row that could be edited or removed is a code that
// could be issued again.
func TestPgIssuedCodeLedgerIsAppendOnly(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-ao-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if _, err := db.Exec(`UPDATE task_issued_code SET code = 'NV99' WHERE tenant_id = $1 AND code = 'NV01'`,
		xa); err == nil {
		t.Error("sửa được một dòng sổ mã đã cấp")
	}
	if _, err := db.Exec(`DELETE FROM task_issued_code WHERE tenant_id = $1 AND code = 'NV01'`,
		xa); err == nil {
		t.Error("xoá được một dòng sổ mã đã cấp — mã ấy sẽ được cấp lại")
	}
}

// TestPgRenameRefusedEvenWithLedgerRowOfThisTask — migration 0024: `ma` changes for NOBODY. 0015's
// door ("the new code was issued to THIS task") is closed; a ledger row issued to the task itself no
// longer lets a statement rename it. THE MUTATION THAT MUST TURN THIS RED: restoring 0015's condition.
func TestPgRenameRefusedEvenWithLedgerRowOfThisTask(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-lr-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_issued_code (tenant_id, code, task_id) VALUES ($1, 'NV07', 'nv-lr-1')`,
		xa); err != nil {
		t.Fatalf("cấp NV07 cho chính nhiệm vụ này: %v", err)
	}
	canMaLoi(t, func() error {
		_, err := db.Exec(`UPDATE nhiem_vu SET ma = 'NV07' WHERE tenant_id = $1 AND id = 'nv-lr-1'`, xa)
		return err
	}(), "P0001", "đổi được mã nhiệm vụ đã cấp — mã phải bất biến (ADR 0065 NV3)")
}

// TestPgTwoCommunesSameIssuedCode — the ledger key is COMPOSITE with tenant_id: every commune's register
// starts at NV01, and one commune's issued codes reserve nothing in another.
func TestPgTwoCommunesSameIssuedCode(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	danhMucChoXa(t, db, xaA)
	danhMucChoXa(t, db, xaB)
	if err := themNhiemVu(db, xaA, "nv-2x-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("xã A: %v", err)
	}
	if err := themNhiemVu(db, xaB, "nv-2x-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("xã B không cấp được NV01 vì xã A đã dùng: %v — khoá phải GHÉP với tenant_id", err)
	}
}
