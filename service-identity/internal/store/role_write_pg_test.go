package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Integration tests for RoleWriteStore (owner decision 15/10/2026) against a real PostgreSQL, sharing
// the harness in checker_pg_test.go (moKetNoi, xaRieng, ctxXa) and vai_tro_pg_test.go (themVaiTro,
// themNguoi). Every property here belongs to the SQL — the constraint name, the commune on
// every statement, `deleted_at` in the right places, the row locks — so a fake could only agree with
// whatever the statements already do.
//
// Skipped unless VIGOV_TEST_DSN is set.

func inRoleTx(t *testing.T, db *sql.DB, xa string, fn func(ctx context.Context, tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	ctx := ctxXa(xa)
	return pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(ctx, tx) })
}

func TestPgRoleWriteInsertCodesAndCollision(t *testing.T) {
	db := moKetNoi(t)
	xa, xaB := xaRieng(t)
	s := NewRoleWriteStore(pkgstore.New(db))

	themVaiTro(t, db, xa, "vt-bt-"+xa, "bi-thu", "Bí thư", false)
	if _, err := db.Exec(`UPDATE vai_tro SET deleted_at = now(), deleted_by = 'CB-X', delete_reason = 'r'
		WHERE tenant_id = $1 AND id = $2`, xa, "vt-bt-"+xa); err != nil {
		t.Fatal(err)
	}
	themVaiTro(t, db, xaB, "vt-bt2-"+xaB, "bi-thu-2", "Bí thư", false) // another commune: not counted

	err := inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		codes, err := s.CodesWithBase(ctx, tx, "bi-thu")
		if err != nil {
			return err
		}
		if len(codes) != 1 || codes[0] != "bi-thu" {
			t.Errorf("codes = %v — the deleted row's code must count, the other commune's must not", codes)
		}
		live, err := s.LiveRoles(ctx, tx)
		if err != nil {
			return err
		}
		if len(live) != 0 {
			t.Errorf("live = %+v — a deleted role is not live", live)
		}
		return s.InsertRole(ctx, tx, domain.ExistingRole{ID: "vt-new-" + xa, Code: "bi-thu-2", Name: "Bí thư", Order: 1})
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	// The deleted role's code is still taken: the unique key refuses it, named.
	err = inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.InsertRole(ctx, tx, domain.ExistingRole{ID: "vt-again-" + xa, Code: "bi-thu", Name: "Bí thư X"})
	})
	if !errors.Is(err, ErrRoleCodeTaken) {
		t.Fatalf("reissuing a deleted role's code: err = %v, want ErrRoleCodeTaken", err)
	}
}

func TestPgRoleWriteLockUpdateAndScope(t *testing.T) {
	db := moKetNoi(t)
	xa, xaB := xaRieng(t)
	s := NewRoleWriteStore(pkgstore.New(db))
	themVaiTro(t, db, xa, "vt-kt-"+xa, "ke-toan", "Kế toán", false)

	// Another commune's context cannot name it.
	err := inRoleTx(t, db, xaB, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, err := s.LockRole(ctx, tx, "vt-kt-"+xa)
		return err
	})
	if !errors.Is(err, ErrVaiTroKhongTonTaiDeGhi) {
		t.Fatalf("another commune's role: err = %v", err)
	}

	err = inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		r, err := s.LockRole(ctx, tx, "vt-kt-"+xa)
		if err != nil {
			return err
		}
		if r.Code != "ke-toan" || r.Name != "Kế toán" || r.IsLeader {
			t.Errorf("locked = %+v", r)
		}
		return s.UpdateRole(ctx, tx, r.ID, "Kế toán ngân sách", true)
	})
	if err != nil {
		t.Fatal(err)
	}
	var ma, ten string
	var leader bool
	if err := db.QueryRow(`SELECT ma, ten, la_lanh_dao FROM vai_tro WHERE tenant_id = $1 AND id = $2`,
		xa, "vt-kt-"+xa).Scan(&ma, &ten, &leader); err != nil {
		t.Fatal(err)
	}
	if ma != "ke-toan" || ten != "Kế toán ngân sách" || !leader {
		t.Fatalf("row = %q %q %v — the code must never change", ma, ten, leader)
	}
}

func TestPgRoleWriteCountHoldersAndSoftDelete(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	s := NewRoleWriteStore(pkgstore.New(db))
	roleID := "vt-cv-" + xa
	themVaiTro(t, db, xa, roleID, "chuyen-vien", "Chuyên viên", false)
	if _, err := db.Exec(`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_boi) VALUES ($1, $2, 'task.read', 'CB-X')`,
		xa, roleID); err != nil {
		t.Fatal(err)
	}
	themNguoi(t, db, xa, "nd-live-"+xa, roleID)
	themNguoi(t, db, xa, "nd-gone-"+xa, roleID)
	if _, err := db.Exec(`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`, xa, "nd-gone-"+xa); err != nil {
		t.Fatal(err)
	}

	count := func() int {
		var n int
		if err := inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			var err error
			n, err = s.CountHolders(ctx, tx, roleID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(); n != 1 {
		t.Fatalf("holders = %d, want 1 (the soft-deleted person does not count)", n)
	}

	// Remove the live person's assignment AND primary role: nobody holds it now.
	if _, err := db.Exec(`UPDATE staff_role_assignment SET deleted_at = now(), deleted_by = 'CB-X', delete_reason = 'r'
		WHERE tenant_id = $1 AND staff_id = $2`, xa, "nd-live-"+xa); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nguoi_dung SET vai_tro_id = NULL WHERE tenant_id = $1 AND id = $2`, xa, "nd-live-"+xa); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 0 {
		t.Fatalf("holders = %d after removal, want 0", n)
	}

	var at time.Time
	if err := inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		at, err = s.SoftDeleteRole(ctx, tx, roleID, "CB-2026-ADMIN1", "Không còn dùng")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var by, reason string
	var deletedAt time.Time
	if err := db.QueryRow(`SELECT deleted_by, delete_reason, deleted_at FROM vai_tro WHERE tenant_id = $1 AND id = $2`,
		xa, roleID).Scan(&by, &reason, &deletedAt); err != nil {
		t.Fatal(err)
	}
	if by != "CB-2026-ADMIN1" || reason != "Không còn dùng" || !deletedAt.Equal(at) {
		t.Fatalf("soft-delete columns = %q %q %v (returned %v)", by, reason, deletedAt, at)
	}
	// Rule 7: the grant row is kept.
	var grants int
	if err := db.QueryRow(`SELECT count(*) FROM vai_tro_quyen WHERE tenant_id = $1 AND vai_tro_id = $2`, xa, roleID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants != 1 {
		t.Fatalf("grant rows = %d after the deletion, want 1 kept", grants)
	}
	// A second deletion cannot overwrite who deleted it or why.
	err := inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, err := s.SoftDeleteRole(ctx, tx, roleID, "CB-OTHER", "khác")
		return err
	})
	if !errors.Is(err, ErrVaiTroKhongTonTaiDeGhi) {
		t.Fatalf("second deletion: err = %v", err)
	}
	// And the assignment path now refuses the role.
	if err := inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, exists, err := NewCanBoStore(pkgstore.New(db)).QuyenCuaVaiTro(ctx, tx, roleID)
		if exists {
			t.Error("a deleted role is still assignable")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// THE RACE THE FOR SHARE CLOSES: an assignment reading the role while a deletion holds it FOR UPDATE
// waits for the deletion's commit and then finds the role gone — it is never handed a role deleted a
// millisecond earlier.
func TestPgRoleDeletionBlocksAConcurrentAssignmentRead(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	s := NewRoleWriteStore(pkgstore.New(db))
	roleID := "vt-race-" + xa
	themVaiTro(t, db, xa, roleID, "vai-tro-race", "Vai trò thử", false)

	locked := make(chan struct{})
	release := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			if _, err := s.LockRole(ctx, tx, roleID); err != nil {
				return err
			}
			close(locked)
			<-release
			_, err := s.SoftDeleteRole(ctx, tx, roleID, "CB-2026-ADMIN1", "thử tranh chấp")
			return err
		})
	}()
	<-locked

	readDone := make(chan bool, 1)
	go func() {
		var exists bool
		_ = inRoleTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			var err error
			_, exists, err = NewCanBoStore(pkgstore.New(db)).QuyenCuaVaiTro(ctx, tx, roleID)
			return err
		})
		readDone <- exists
	}()

	select {
	case <-readDone:
		t.Fatal("the assignment read did not wait for the deletion's row lock")
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if err := <-deleteDone; err != nil {
		t.Fatalf("deletion: %v", err)
	}
	if exists := <-readDone; exists {
		t.Fatal("the assignment read saw the role as live after the deletion committed")
	}
}
