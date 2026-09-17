package store

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/migrate"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/migrations"
)

// Integration tests for the permission check, against a real PostgreSQL.
//
// WHY A REAL DATABASE: what is being verified is that commune A's grants do not apply in
// commune B. That property lives entirely in the SQL — in the tenant predicate and in each
// JOIN's ON clause — and a mock would simply agree with whatever the query says.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

// The schema is built ONCE for the whole package, not per test.
//
// WHY: the migration creates five partitioned tables with 32 partitions each — 160 CREATE
// TABLE statements. Running that per test cost 17 seconds each, and a suite slow enough to
// skip is a suite that stops being run. Each test isolates itself by using its own commune
// ids instead, which is also closer to how the data really looks.
var (
	dbChung     *sql.DB
	schemaChung string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // mỗi test tự bỏ qua
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schemaChung = fmt.Sprintf("vigov_id_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a
	// pool it applies to whichever connection happened to serve that statement and to no other —
	// the next statement can land on a fresh connection in the public schema. It happened to work
	// while everything ran on one lazily-opened connection; pkg/migrate deliberately pins its own
	// connection (the advisory lock is session-scoped), which would have been a second one.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}

	// THE REAL RUNNER, not a hand-picked file. This used to read `0001_init.sql` by name, which
	// is why `0002_audit_log_append_only.sql` had never executed anywhere: the only two things in
	// the repository that touched migrations both named one file. Going through pkg/migrate means
	// every migration this service ships is exercised here, and a new one is exercised the day it
	// is added rather than the day somebody remembers to extend this list.
	kq, err := migrate.Chay(ctx, db, migrations.FS, "identity")
	if err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "migration đã áp:", kq.DaAp)

	dbChung = db
	ma := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data;
	// this holds none.
	if _, err := db.ExecContext(context.Background(),
		"DROP SCHEMA IF EXISTS "+schemaChung+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "dọn schema:", err)
	}
	db.Close()
	os.Exit(ma)
}

func moKetNoi(t *testing.T) *sql.DB {
	t.Helper()
	if dbChung == nil {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	return dbChung
}

// xaRieng returns a commune id unique to this test, so tests sharing the schema cannot see
// each other's rows. Two ids are returned for the tests that need to prove isolation.
func xaRieng(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	a := fmt.Sprintf("%026d", n)
	b := fmt.Sprintf("%026d", n+1)
	return a[:26], b[:26]
}

// dungXa creates one commune's org: a role, its grants, and one staff member holding it.
func dungXa(t *testing.T, db *sql.DB, tenantID, vaiTroID, nguoiID string, quyen []string) {
	t.Helper()

	_, err := db.Exec(
		`INSERT INTO vai_tro (tenant_id, id, ten, ma) VALUES ($1,$2,$3,$4)`,
		tenantID, vaiTroID, "Chuyên viên", "chuyen-vien-"+vaiTroID)
	if err != nil {
		t.Fatalf("thêm vai trò: %v", err)
	}

	for _, q := range quyen {
		_, err := db.Exec(
			`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma) VALUES ($1,$2,$3)`,
			tenantID, vaiTroID, q)
		if err != nil {
			t.Fatalf("cấp quyền %s: %v", q, err)
		}
	}

	// co_tai_khoan IS SET EXPLICITLY, and the reason is worth keeping. Migration 0003 defaults it
	// to FALSE — authority is granted by a named person, never by a schema default — so a staff
	// row inserted without it is a directory entry that can hold no permission. Leaving it out
	// here turned every test in this file red at once, which is the default doing exactly what it
	// was designed to do. These tests are about the permission check, so their people are
	// accounts.
	_, err = db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, vai_tro_id, co_tai_khoan)
		 VALUES ($1,$2,$3,$4,$5,$6,true)`,
		tenantID, nguoiID, "CB-"+nguoiID, "Nguyễn Văn A",
		nguoiID+"@xa.danang.gov.vn", vaiTroID)
	if err != nil {
		t.Fatalf("thêm cán bộ: %v", err)
	}
}

func dungChecker(db *sql.DB) *Checker {
	return NewChecker(pkgstore.New(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func ctxXa(id string) context.Context {
	return tenant.Into(context.Background(), tenant.ID(id))
}

func canBo(id, tenantID string) authz.Principal {
	return authz.Principal{ID: id, Kind: "staff", TenantID: tenant.ID(tenantID)}
}

func TestChoPhepKhiCoQuyen(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read", "task.create"})

	c := dungChecker(db)
	if !c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.read") {
		t.Error("cán bộ có task.read mà bị từ chối")
	}
}

func TestTuChoiKhiKhongCoQuyen(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.approve") {
		t.Error("cán bộ không có task.approve mà vẫn qua")
	}
}

func TestQuyenGanNhauKhongSuyRaNhau(t *testing.T) {
	// task.approve (duyệt hoàn thành) closes a commitment made to a citizen; task.extend
	// (duyệt gia hạn) moves its deadline. Holding one must never imply the other — this is the
	// case the old (subsystem, action) model collapsed.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.approve"})

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.extend") {
		t.Error("có task.approve mà suy ra được task.extend")
	}
}

func TestQuyenCuaXaNayKhongApDungOXaKhac(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Two communes, the same role id and the same staff id — the
	// shape produced by seeding communes from one template. If any JOIN matched on id alone
	// instead of (tenant_id, id), commune B's staff would inherit commune A's grants.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	dungXa(t, db, xaA, "vt-chung", "nd-chung", []string{"task.approve", "admin.role"})
	dungXa(t, db, xaB, "vt-chung", "nd-chung", []string{"task.read"})

	c := dungChecker(db)

	// Commune B's staff holds only task.read, even though the identically-named role in
	// commune A holds far more.
	if !c.Allows(ctxXa(xaB), canBo("nd-chung", xaB), "task.read") {
		t.Error("xã B mất quyền của chính mình")
	}
	if c.Allows(ctxXa(xaB), canBo("nd-chung", xaB), "task.approve") {
		t.Error("RÒ RỈ: xã B thừa hưởng quyền của xã A")
	}
	if c.Allows(ctxXa(xaB), canBo("nd-chung", xaB), "admin.role") {
		t.Error("RÒ RỈ: xã B thừa hưởng quyền quản trị của xã A")
	}
}

func TestCanBoNgungHoatDongBiTuChoi(t *testing.T) {
	// An account is locked the moment a person leaves. The grant row is still there, so only
	// the dang_hoat_dong predicate stands between a former employee and the subsystem.
	//
	// THE TWIN OF THIS TEST IS THE ONE BELOW. Two columns, two tests, on purpose: they were one
	// column carrying both meanings until migration 0003, and a single test would let them be
	// merged back without anything turning red.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET dang_hoat_dong = false WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-1"); err != nil {
		t.Fatal(err)
	}

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.read") {
		t.Error("tài khoản đã khoá mà vẫn qua được")
	}
}

func TestNguoiChiCoTrongDanhBaKhongCoQuyenNao(t *testing.T) {
	// The other half of the pair: NOT LOCKED (dang_hoat_dong stays true) but NO SIGN-IN ACCOUNT.
	// This is the 26-minus-12 of the specification's seed — people who exist in the public staff
	// directory and in the org chart, which is why the row can perfectly well carry a vai_tro_id,
	// and therefore reach the grants through the join. Only nd.co_tai_khoan stands between a
	// directory entry and the subsystem, and dang_hoat_dong cannot stand in for it: it is true
	// here, as it is on every directory row.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET co_tai_khoan = false WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-1"); err != nil {
		t.Fatal(err)
	}

	// Stated rather than assumed: the person is NOT locked. Were this false, the test would pass
	// on the wrong predicate and prove nothing about co_tai_khoan.
	var hoatDong bool
	if err := db.QueryRow(
		`SELECT dang_hoat_dong FROM nguoi_dung WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-1").Scan(&hoatDong); err != nil {
		t.Fatal(err)
	}
	if !hoatDong {
		t.Fatal("dựng sai cảnh: người này phải CHƯA bị khoá, chỉ là không có tài khoản")
	}

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.read") {
		t.Error("người chỉ có trong danh bạ, không có tài khoản đăng nhập, mà vẫn có quyền")
	}
}

func TestCanBoXoaMemBiTuChoi(t *testing.T) {
	// Soft delete keeps the row for the audit trail (rule 7). Every read path must exclude it,
	// and a permission check is a read path.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-1"); err != nil {
		t.Fatal(err)
	}

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.read") {
		t.Error("cán bộ đã xoá mềm mà vẫn qua được")
	}
}

func TestVaiTroXoaMemThiMatQuyen(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	if _, err := db.Exec(
		`UPDATE vai_tro SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xaA, "vt-1"); err != nil {
		t.Fatal(err)
	}

	c := dungChecker(db)
	if c.Allows(ctxXa(xaA), canBo("nd-1", xaA), "task.read") {
		t.Error("vai trò đã xoá mềm mà quyền vẫn còn hiệu lực")
	}
}

func TestCongDanKhongDungRbac(t *testing.T) {
	// Citizens are isolated by identity (rule 4), never by RBAC. A citizen reaching an
	// RBAC-guarded route is a routing mistake; answering false is the safe reading.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})

	c := dungChecker(db)
	congDan := authz.Principal{ID: "nd-1", Kind: "citizen", TenantID: tenant.ID(xaA)}
	if c.Allows(ctxXa(xaA), congDan, "task.read") {
		t.Error("công dân đi qua được kiểm tra RBAC")
	}
}

func TestAllowsNhieuMotLuotDiMang(t *testing.T) {
	// A screen asks "which of these may I do" to decide what to render. One query per
	// permission is one round trip per button (skills/load-data-once).
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"feedback.read", "feedback.assign"})

	c := dungChecker(db)
	hoi := []authz.Perm{"feedback.read", "feedback.assign", "feedback.resolve", "feedback.restricted"}
	ra, err := c.AllowsNhieu(ctxXa(xaA), canBo("nd-1", xaA), hoi)
	if err != nil {
		t.Fatalf("AllowsNhieu lỗi: %v", err)
	}

	// Every requested key must be present, so a denied permission reads as false rather than
	// as a missing entry that might be mistaken for "unknown".
	if len(ra) != len(hoi) {
		t.Errorf("trả về %d khoá, muốn %d", len(ra), len(hoi))
	}
	for perm, muon := range map[authz.Perm]bool{
		"feedback.read":       true,
		"feedback.assign":     true,
		"feedback.resolve":    false,
		"feedback.restricted": false,
	} {
		if ra[perm] != muon {
			t.Errorf("%s = %v, muốn %v", perm, ra[perm], muon)
		}
	}
}

func TestDanhMucQuyenDuocNap(t *testing.T) {
	// The catalogue ships with the code: a key no route checks grants nothing, and a route
	// checking a key that does not exist can never be reached.
	db := moKetNoi(t)

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM quyen`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 33 {
		t.Errorf("có %d quyền trong danh mục, muốn 33 theo đặc tả", n)
	}
}

func TestKhongCapDuocQuyenNgoaiDanhMuc(t *testing.T) {
	// A tick box on the Phân quyền screen that no code checks grants nothing and only misleads
	// the administrator ticking it.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", nil)

	_, err := db.Exec(
		`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma) VALUES ($1,$2,$3)`,
		xaA, "vt-1", "quyen.bia.dat")
	if err == nil {
		t.Error("cấp được một quyền không có trong danh mục")
	}
}
