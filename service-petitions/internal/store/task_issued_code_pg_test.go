package store

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for migration 0015 — `task_issued_code` and the relaxed `nhiem_vu_bat_bien`.
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. They were written in an environment with no PostgreSQL reachable, so none of
// the assertions below has been executed yet. Every property here is enforced by the DATABASE alone;
// the fake-driver suites (internal/app/task_code_change_test.go) prove the write path's statements
// and order, not that the database honours them.
//
// The pre-existing TestPgKhongXoaCungDuocNhiemVuVaKhongDoiDuocMa keeps proving the other half: a raw
// rename of `ma` with no ledger row is still refused.

// TestPgInsertRegistersCodeAndRenameKeepsOldReserved — the whole card in one sequence, through the
// real store methods: create → the insert trigger issues NV01 → ChangeCode NV01 → NV05 → NV01 is still
// issued, NV05 is issued, the register carries NV05, and the minting high-water mark is 5.
func TestPgInsertRegistersCodeAndRenameKeepsOldReserved(t *testing.T) {
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
		return s.ChangeCode(ctx, tx, "nv-ic-1", "NV01", "NV05")
	}); err != nil {
		t.Fatalf("đổi mã: %v", err)
	}

	if _, err := s.TheoMa(ctx, "NV05"); err != nil {
		t.Errorf("không đọc được nhiệm vụ theo mã mới: %v", err)
	}
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		for _, code := range []string{"NV01", "NV05"} {
			taken, err := s.MaDaDung(ctx, tx, code)
			if err != nil {
				return err
			}
			if !taken {
				t.Errorf("%s không còn trong sổ mã đã cấp — sẽ bị cấp lại", code)
			}
		}
		max, err := s.SoLonNhatDaCap(ctx, tx)
		if err != nil {
			return err
		}
		if max != 5 {
			t.Errorf("số lớn nhất đã cấp = %d, muốn 5", max)
		}
		return nil
	}); err != nil {
		t.Fatalf("đọc sổ mã: %v", err)
	}
}

// TestPgRetiredCodeCannotBeReissued — after NV01 → NV05, NV01 is on no row of the register, and it must
// still be refused by a raw INSERT (the insert trigger hits the ledger's key) and by a second rename.
// This is the defect of vigov-require's `code_ever_used`, pinned at the floor.
func TestPgRetiredCodeCannotBeReissued(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	for _, r := range []struct{ id, ma string }{{"nv-rt-1", "NV01"}, {"nv-rt-2", "NV02"}} {
		if err := themNhiemVu(db, xa, r.id, r.ma, "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.ma, err)
		}
	}
	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	ctx := ctxXa(tenant.ID(xa))
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.ChangeCode(ctx, tx, "nv-rt-1", "NV01", "NV05")
	}); err != nil {
		t.Fatalf("đổi mã: %v", err)
	}

	canMaLoi(t, themNhiemVu(db, xa, "nv-rt-3", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil),
		"23505", "một nhiệm vụ MỚI nhận lại NV01 đã đổi đi — biên bản ghi NV01 sẽ chỉ sang việc khác")

	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.ChangeCode(ctx, tx, "nv-rt-2", "NV02", "NV01")
	})
	canMaLoi(t, err, "23505", "đổi một nhiệm vụ khác sang NV01 đã đổi đi")
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

// TestPgRenameNeedsLedgerRowOfThisTask — the relaxed trigger admits a change of `ma` only when the new
// code was issued to THIS task. A ledger row issued to another task does not open the door.
func TestPgRenameNeedsLedgerRowOfThisTask(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-lr-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_issued_code (tenant_id, code, task_id) VALUES ($1, 'NV07', 'nv-khac')`,
		xa); err != nil {
		t.Fatalf("cấp NV07 cho nhiệm vụ khác: %v", err)
	}
	canMaLoi(t, func() error {
		_, err := db.Exec(`UPDATE nhiem_vu SET ma = 'NV07' WHERE tenant_id = $1 AND id = 'nv-lr-1'`, xa)
		return err
	}(), "P0001", "đổi được mã sang một mã cấp cho nhiệm vụ khác")
}

// TestPgTwoCommunesSameIssuedCode — the ledger key is COMPOSITE with tenant_id: every commune's register
// starts at NV01, and a commune renaming NV01 away reserves it for itself only.
func TestPgTwoCommunesSameIssuedCode(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	danhMucChoXa(t, db, xaA)
	danhMucChoXa(t, db, xaB)
	if err := themNhiemVu(db, xaA, "nv-2x-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("xã A: %v", err)
	}
	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	ctxA := ctxXa(tenant.ID(xaA))
	if err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		return s.ChangeCode(ctxA, tx, "nv-2x-1", "NV01", "NV05")
	}); err != nil {
		t.Fatalf("xã A đổi mã: %v", err)
	}
	if err := themNhiemVu(db, xaB, "nv-2x-1", "NV01", "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
		t.Fatalf("xã B không cấp được NV01 vì xã A đã dùng: %v — khoá phải GHÉP với tenant_id", err)
	}
}
