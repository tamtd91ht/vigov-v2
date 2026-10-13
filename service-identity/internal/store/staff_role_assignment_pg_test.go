package store

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/migrations"
)

// Several roles per person (migration 0031, owner decision 13/10/2026), against a real PostgreSQL.
// Skipped unless VIGOV_TEST_DSN is set — the union, the soft removal and the backfill live entirely
// in SQL, and a fake would agree with whatever the query says.

// A PERSON WITH TWO ROLES HOLDS THE UNION OF THEIR GRANTS — and nothing a third role of the same
// commune grants, nor what the same role id grants in another commune.
//
// MUTATION THAT MUST TURN THIS RED: read grants off `nd.vai_tro_id` (only the primary role counts).
func TestPgTwoRolesGrantTheUnion(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	dungXa(t, db, xaA, "vt-2", "nd-khac", []string{"budget.confirm"})
	dungXa(t, db, xaA, "vt-3", "nd-thu-ba", []string{"admin.role"})
	dungXa(t, db, xaB, "vt-2", "nd-b", []string{"admin.user"})
	assignRolePg(t, db, xaA, "nd-1", "vt-2")

	c := dungChecker(db)
	p := canBo("nd-1", xaA)
	for perm, want := range map[string]bool{
		"task.read": true, "budget.confirm": true, "admin.role": false, "admin.user": false,
	} {
		if got := c.Allows(ctxXa(xaA), p, authz.Perm(perm)); got != want {
			t.Errorf("%s = %v, muốn %v", perm, got, want)
		}
	}
	ds, err := c.QuyenCua(ctxXa(xaA), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 {
		t.Errorf("danh sách quyền = %v, muốn đúng hai khoá, mỗi khoá một lần", ds)
	}
}

// SetStaffRoles SOFT-REMOVES what leaves the set, ADDS what enters, and keeps the primary column equal
// to the first role — all in the caller's transaction.
func TestPgSetStaffRolesSoftRemovesAndSyncsPrimary(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	dungXa(t, db, xa, "vt-1", "nd-1", []string{"task.read"})
	dungXa(t, db, xa, "vt-2", "nd-2", []string{"budget.confirm"})

	kho := NewCanBoStore(pkgstore.New(db))
	ctx := ctxXa(xa)
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.SetStaffRoles(ctx, tx, "nd-1", []string{"vt-2"}, "CB-2026-QT0001")
	}); err != nil {
		t.Fatalf("SetStaffRoles: %v", err)
	}

	var primary, removedBy string
	if err := db.QueryRow(`SELECT coalesce(vai_tro_id,'') FROM nguoi_dung WHERE tenant_id=$1 AND id='nd-1'`, xa).
		Scan(&primary); err != nil {
		t.Fatal(err)
	}
	if primary != "vt-2" {
		t.Errorf("vai_tro_id = %q, muốn vt-2 — cột chính lệch khỏi tập", primary)
	}
	if err := db.QueryRow(`SELECT deleted_by FROM staff_role_assignment
	                        WHERE tenant_id=$1 AND staff_id='nd-1' AND role_id='vt-1'`, xa).Scan(&removedBy); err != nil {
		t.Fatalf("dòng vt-1 phải còn (xoá mềm): %v", err)
	}
	if removedBy != "CB-2026-QT0001" {
		t.Errorf("deleted_by = %q, muốn mã cán bộ", removedBy)
	}
	c := dungChecker(db)
	if c.Allows(ctx, canBo("nd-1", xa), "task.read") || !c.Allows(ctx, canBo("nd-1", xa), "budget.confirm") {
		t.Error("quyền sau đổi vai trò không khớp tập mới")
	}
}

// THE LOCKED ADMINISTRATOR SET NAMES A TWO-ROLE ADMINISTRATOR ONCE (#13).
func TestPgLastAdminWithTwoAdminRolesCountsOnce(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	dungXa(t, db, xa, "vt-qt", "nd-qt", []string{QuyenQuanTriNguoiDung})
	themVaiTro(t, db, xa, "vt-qt2", "quan-tri-2", "Quản trị 2", false)
	capQuyenCho(t, db, xa, "vt-qt2", QuyenQuanTriNguoiDung)
	assignRolePg(t, db, xa, "nd-qt", "vt-qt2")

	kho := NewCanBoStore(pkgstore.New(db))
	ctx := ctxXa(xa)
	var ids []string
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		ids, err = kho.QuanTriDangHoatDong(ctx, tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "nd-qt" {
		t.Fatalf("quản trị viên = %v, muốn [nd-qt] — một người hai vai trò quản trị phải là MỘT người", ids)
	}
}

// THE BACKFILL OF MIGRATION 0031, run again on a row written the pre-0031 way (role in the column
// only): one assignment appears, created_by 'system', and one system audit entry for the commune.
// The statement is read FROM THE EMBEDDED FILE, so this exercises the SQL that ships.
func TestPgMigration0031BackfillCopiesTheColumn(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	if _, err := db.Exec(`INSERT INTO vai_tro (tenant_id, id, ten, ma) VALUES ($1,'vt-cu','Cũ','vt-cu')`, xa); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, vai_tro_id)
	                      VALUES ($1,'nd-cu','CB-CU','Nguyễn Văn A','cu@xa.danang.gov.vn','vt-cu')`, xa); err != nil {
		t.Fatal(err)
	}

	raw, err := fs.ReadFile(migrations.FS, "0031_staff_role_assignment.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	start := strings.Index(sql, "WITH copied AS (")
	end := strings.Index(sql, "FROM per_commune;")
	if start < 0 || end < start {
		t.Fatal("không tìm thấy câu backfill trong 0031")
	}
	if _, err := db.ExecContext(context.Background(), sql[start:end+len("FROM per_commune;")]); err != nil {
		t.Fatalf("chạy lại backfill: %v", err)
	}

	var by string
	if err := db.QueryRow(`SELECT created_by FROM staff_role_assignment
	                        WHERE tenant_id=$1 AND staff_id='nd-cu' AND role_id='vt-cu' AND deleted_at IS NULL`, xa).
		Scan(&by); err != nil {
		t.Fatalf("backfill không tạo dòng gán: %v", err)
	}
	if by != "system" {
		t.Errorf("created_by = %q, muốn system", by)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE tenant_id=$1 AND action='gan_vai_tro_ban_dau'
	                        AND actor_kind='system' AND (delta->>'so_dong')::int = 1`, xa).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("có %d vết backfill cho xã, muốn 1", n)
	}
}
