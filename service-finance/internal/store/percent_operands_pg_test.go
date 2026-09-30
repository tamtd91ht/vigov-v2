package store

import (
	"database/sql"
	"io/fs"
	"testing"

	"github.com/vihat/vigov/service-finance/migrations"
)

// Integration tests for migration 0011 — the operand columns of a budget % column, their CHECKs, and
// the backfill from `cong_thuc` — against a real PostgreSQL.
//
// HOW THE BACKFILL IS EXERCISED: TestMain has already applied every migration to an empty schema, so
// 0011 found nothing to fill. These tests insert legacy-shaped rows (operands NULL) and then execute
// the file's body AGAIN. That is legitimate, not a trick: the file is written to be re-runnable (every
// DDL is guarded, the fill only matches empty pairs), and re-running it by hand after a rolling deploy
// is exactly what its header tells an operator to do.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET. On a machine without it every test here SKIPS while the
// package prints `ok` — a green run means the code COMPILES, nothing more. TestMain, moKetNoi and
// xaRieng live in hang_muc_ke_hoach_von_pg_test.go.

const file0011 = "0011_budget_percent_operands.sql"

func rerun0011(t *testing.T, db *sql.DB) {
	t.Helper()
	body, err := fs.ReadFile(migrations.FS, file0011)
	if err != nil {
		t.Fatalf("read %s: %v", file0011, err)
	}
	if _, err := db.Exec(string(body)); err != nil {
		t.Fatalf("re-run %s: %v", file0011, err)
	}
}

func addSheet(t *testing.T, db *sql.DB, tenantID, id, code string, year int, kind string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO bang_ngan_sach (tenant_id, id, ma, nam, loai, lan, tieu_de)
		 VALUES ($1,$2,$3,$4,$5,1,'Bảng thử')`,
		tenantID, id, code, year, kind)
	if err != nil {
		t.Fatalf("add sheet %q: %v", id, err)
	}
}

// addColumn inserts a column the way an OLD writer does — operands NULL. formula "" = NULL.
func addColumn(t *testing.T, db *sql.DB, tenantID, sheetID, id string, order int, kind, formula string) {
	t.Helper()
	var f any
	if formula != "" {
		f = formula
	}
	_, err := db.Exec(
		`INSERT INTO cot_ngan_sach (tenant_id, id, bang_id, ten, thu_tu, kieu, cong_thuc)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, id, sheetID, "Cột "+id, order, kind, f)
	if err != nil {
		t.Fatalf("add column %q: %v", id, err)
	}
}

func operandsOf(t *testing.T, db *sql.DB, tenantID, id string) (sql.NullString, sql.NullString) {
	t.Helper()
	var n, d sql.NullString
	err := db.QueryRow(
		`SELECT numerator_column_id, denominator_column_id FROM cot_ngan_sach
		 WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&n, &d)
	if err != nil {
		t.Fatalf("read operands of %q: %v", id, err)
	}
	return n, d
}

func wantOperands(t *testing.T, db *sql.DB, tenantID, id, num, den string) {
	t.Helper()
	n, d := operandsOf(t, db, tenantID, id)
	if num == "" {
		if n.Valid || d.Valid {
			t.Errorf("%s: operands (%v, %v), want both NULL", id, n, d)
		}
		return
	}
	if n.String != num || d.String != den {
		t.Errorf("%s: operands (%v, %v), want (%s, %s)", id, n, d, num, den)
	}
}

// TestMigration0011Backfill — resolves the starter-set shape, leaves every ambiguous or foreign
// shape NULL, never overwrites a set pair, stays inside each commune, and audits once.
func TestMigration0011Backfill(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)

	// Sheet 1 — the thu starter set (nhan-thu-chi.ts:815-819): four number columns, then %.
	addSheet(t, db, a, "s1", "NS-2026-THU-01", 2026, "thu")
	for i, id := range []string{"c1", "c2", "c3", "c4"} {
		addColumn(t, db, a, "s1", id, i+1, "so", "")
	}
	addColumn(t, db, a, "s1", "p-ok", 5, "phan_tram", "col_3 / col_1 * 100")
	addColumn(t, db, a, "s1", "p-same", 6, "phan_tram", "col_2 / col_2 * 100")
	addColumn(t, db, a, "s1", "p-shape", 7, "phan_tram", "col_2 / col_1 * 100.0")
	addColumn(t, db, a, "s1", "p-range", 8, "phan_tram", "col_9 / col_1 * 100")
	// A pair a writer already set must survive, even though the formula says otherwise.
	addColumn(t, db, a, "s1", "p-set", 9, "phan_tram", "col_3 / col_1 * 100")
	if _, err := db.Exec(`UPDATE cot_ngan_sach SET numerator_column_id = 'c2', denominator_column_id = 'c1'
		WHERE tenant_id = $1 AND id = 'p-set'`, a); err != nil {
		t.Fatalf("pre-set operands: %v", err)
	}

	// Sheet 2 — two number columns share thu_tu 1: position 1 and 2 are ambiguous.
	addSheet(t, db, a, "s2", "NS-2026-CHI-01", 2026, "chi")
	addColumn(t, db, a, "s2", "t1", 1, "so", "")
	addColumn(t, db, a, "s2", "t2", 1, "so", "")
	addColumn(t, db, a, "s2", "t3", 2, "so", "")
	addColumn(t, db, a, "s2", "p-tie", 3, "phan_tram", "col_2 / col_1 * 100")

	// Sheet 3 — a % column BEFORE an operand: "2nd number column" and "2nd column" disagree.
	addSheet(t, db, a, "s3", "NS-2027-THU-01", 2027, "thu")
	addColumn(t, db, a, "s3", "y1", 1, "so", "")
	addColumn(t, db, a, "s3", "p-mixed", 2, "phan_tram", "col_2 / col_1 * 100")
	addColumn(t, db, a, "s3", "y2", 3, "so", "")

	// Commune B — SAME ids, reversed order. A join that forgot tenant_id resolves these wrongly.
	addSheet(t, db, b, "s1", "NS-2026-THU-01", 2026, "thu")
	for i, id := range []string{"c4", "c3", "c2", "c1"} {
		addColumn(t, db, b, "s1", id, i+1, "so", "")
	}
	addColumn(t, db, b, "s1", "p-ok", 5, "phan_tram", "col_3 / col_1 * 100")

	rerun0011(t, db)

	wantOperands(t, db, a, "p-ok", "c3", "c1")
	wantOperands(t, db, a, "p-same", "", "")
	wantOperands(t, db, a, "p-shape", "", "")
	wantOperands(t, db, a, "p-range", "", "")
	wantOperands(t, db, a, "p-set", "c2", "c1")
	wantOperands(t, db, a, "p-tie", "", "")
	wantOperands(t, db, a, "p-mixed", "", "")
	wantOperands(t, db, b, "p-ok", "c2", "c4")
	wantOperands(t, db, a, "c1", "", "")

	var formula string
	if err := db.QueryRow(`SELECT cong_thuc FROM cot_ngan_sach WHERE tenant_id = $1 AND id = 'p-ok'`,
		a).Scan(&formula); err != nil || formula != "col_3 / col_1 * 100" {
		t.Errorf("cong_thuc = %q (%v), must be unchanged", formula, err)
	}

	countAudit := func(tenantID string) (n int, subject string) {
		t.Helper()
		if err := db.QueryRow(
			`SELECT count(*), COALESCE(min(subject), '') FROM audit_log
			 WHERE tenant_id = $1 AND action = 'dien_toan_hang_cot_phan_tram'
			   AND actor_id = 'system' AND actor_kind = 'system'`, tenantID).Scan(&n, &subject); err != nil {
			t.Fatalf("count audit: %v", err)
		}
		return n, subject
	}
	if n, s := countAudit(a); n != 1 || s != "NS-2026-THU-01" {
		t.Errorf("commune A: %d audit entries with subject %q, want 1 with the sheet code", n, s)
	}
	if n, _ := countAudit(b); n != 1 {
		t.Errorf("commune B: %d audit entries, want 1", n)
	}

	// Idempotent: a second run fills nothing and audits nothing.
	rerun0011(t, db)
	if n, _ := countAudit(a); n != 1 {
		t.Errorf("after a re-run commune A has %d audit entries, want still 1", n)
	}
}

// TestMigration0011Checks — each CHECK refuses what it exists to refuse.
func TestMigration0011Checks(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	addSheet(t, db, a, "s1", "NS-2026-THU-01", 2026, "thu")
	addColumn(t, db, a, "s1", "c1", 1, "so", "")
	addColumn(t, db, a, "s1", "c2", 2, "so", "")
	addColumn(t, db, a, "s1", "p", 3, "phan_tram", "col_2 / col_1 * 100")

	for _, c := range []struct {
		name, id, num, den string
	}{
		{"number column with operands", "c1", "c2", "c2x"},
		{"numerator only", "p", "c2", ""},
		{"numerator = denominator", "p", "c1", "c1"},
		{"points at itself", "p", "p", "c1"},
		{"blank operand", "p", " ", "c1"},
	} {
		var num, den any
		if c.num != "" {
			num = c.num
		}
		if c.den != "" {
			den = c.den
		}
		_, err := db.Exec(`UPDATE cot_ngan_sach SET numerator_column_id = $3, denominator_column_id = $4
			WHERE tenant_id = $1 AND id = $2`, a, c.id, num, den)
		if err == nil {
			t.Errorf("%s: accepted, want a CHECK violation", c.name)
		}
	}
}
