package store

import (
	"database/sql"
	"testing"
)

// Integration tests for migration 0012 — budget period closes and `dot_thu_chi.adjustment_reason` —
// against a real PostgreSQL. What they exist to prove is what the text tests in
// migrations/period_close_test.go CANNOT: that the server accepts the partial NULLS NOT DISTINCT
// unique index on a partitioned table, and that it refuses what it claims to.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET. On a machine without it every test here SKIPS while the
// package prints `ok` — a green run means the code COMPILES, nothing more. TestMain, moKetNoi and
// xaRieng live in hang_muc_ke_hoach_von_pg_test.go.

// closePeriod inserts one close row. month 0 = the whole year (NULL).
func closePeriod(db *sql.DB, tenantID, id, code string, year, month, revision int) error {
	var m any
	if month != 0 {
		m = month
	}
	_, err := db.Exec(
		`INSERT INTO budget_period_closes (tenant_id, id, code, year, month, revision, closed_by)
		 VALUES ($1,$2,$3,$4,$5,$6,'CB-00001')`,
		tenantID, id, code, year, m, revision)
	return err
}

func reopenClose(db *sql.DB, tenantID, id, reason string) error {
	_, err := db.Exec(
		`UPDATE budget_period_closes
		 SET reopened_at = now(), reopened_by = 'CB-00002', reopen_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, tenantID, id, reason)
	return err
}

func mustOK(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func mustRefuse(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: accepted, want refused", what)
	}
}

// TestMigration0012OneActiveClose — one active close per (commune, year, month-or-year); a reopen
// frees the period; the year close is unique too (the NULLS NOT DISTINCT half); another commune is
// unaffected; an issued code or revision is never reissued.
func TestMigration0012OneActiveClose(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)

	mustOK(t, "close 09/2026", closePeriod(db, a, "k1", "CK-2026-09-01", 2026, 9, 1))
	mustRefuse(t, "second active close of 09/2026", closePeriod(db, a, "k2", "CK-2026-09-02", 2026, 9, 2))

	mustOK(t, "close year 2026 beside the month close", closePeriod(db, a, "y1", "CK-2026-CN-01", 2026, 0, 1))
	mustRefuse(t, "second active YEAR close of 2026", closePeriod(db, a, "y2", "CK-2026-CN-02", 2026, 0, 2))

	// Commune B: same period, same code, same ids — its own business.
	mustOK(t, "commune B closes 09/2026", closePeriod(db, b, "k1", "CK-2026-09-01", 2026, 9, 1))
	mustOK(t, "commune B closes 2026", closePeriod(db, b, "y1", "CK-2026-CN-01", 2026, 0, 1))

	mustOK(t, "reopen 09/2026", reopenClose(db, a, "k1", "Ghi sót đợt thu phí chợ"))
	mustRefuse(t, "re-close reusing an issued code", closePeriod(db, a, "k3", "CK-2026-09-01", 2026, 9, 2))
	mustRefuse(t, "re-close reusing an issued revision", closePeriod(db, a, "k3", "CK-2026-09-02", 2026, 9, 1))
	mustOK(t, "re-close 09/2026 as a new row", closePeriod(db, a, "k3", "CK-2026-09-02", 2026, 9, 2))

	var active int
	mustOK(t, "count", db.QueryRow(
		`SELECT count(*) FROM budget_period_closes
		 WHERE tenant_id = $1 AND year = 2026 AND (month = 9 OR month IS NULL) AND reopened_at IS NULL`,
		a).Scan(&active))
	if active != 2 {
		t.Errorf("active closes covering 09/2026 in commune A = %d, want 2 (month + year)", active)
	}
}

// TestMigration0012History — no hard delete, no edit, one reopen fill, no un-reopen.
func TestMigration0012History(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	mustOK(t, "close", closePeriod(db, a, "k1", "CK-2026-08-01", 2026, 8, 1))

	_, err := db.Exec(`DELETE FROM budget_period_closes WHERE tenant_id = $1 AND id = 'k1'`, a)
	mustRefuse(t, "hard delete", err)
	_, err = db.Exec(`UPDATE budget_period_closes SET closed_by = 'CB-00009' WHERE tenant_id = $1 AND id = 'k1'`, a)
	mustRefuse(t, "rewrite closed_by", err)
	_, err = db.Exec(`UPDATE budget_period_closes SET month = 7 WHERE tenant_id = $1 AND id = 'k1'`, a)
	mustRefuse(t, "move the close to another month", err)
	_, err = db.Exec(`UPDATE budget_period_closes SET reopened_at = now() WHERE tenant_id = $1 AND id = 'k1'`, a)
	mustRefuse(t, "half a reopen (no actor, no reason)", err)
	mustRefuse(t, "reopen with a blank reason", reopenClose(db, a, "k1", "   "))

	mustOK(t, "reopen", reopenClose(db, a, "k1", "Sai ngày đợt chi"))
	mustRefuse(t, "rewrite the reopen reason", reopenClose(db, a, "k1", "Lý do khác"))
	_, err = db.Exec(`UPDATE budget_period_closes SET reopened_at = NULL, reopened_by = NULL, reopen_reason = NULL
		WHERE tenant_id = $1 AND id = 'k1'`, a)
	mustRefuse(t, "un-reopen", err)
}

// TestMigration0012Checks — row-level CHECKs, and the adjustment reason on a batch.
func TestMigration0012Checks(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)

	mustRefuse(t, "month 13", closePeriod(db, a, "x1", "CK-X1", 2026, 13, 1))
	mustRefuse(t, "year 1999", closePeriod(db, a, "x2", "CK-X2", 1999, 1, 1))
	mustRefuse(t, "revision 0", closePeriod(db, a, "x3", "CK-X3", 2026, 1, 0))
	mustRefuse(t, "blank code", closePeriod(db, a, "x4", " ", 2026, 1, 1))

	insertBatch := func(id string, reason any) error {
		_, err := db.Exec(
			`INSERT INTO dot_thu_chi (tenant_id, id, khoan_muc_id, ngay, noi_dung, nguoi_ghi_ma, adjustment_reason)
			 VALUES ($1,$2,'km1',DATE '2026-10-02','Điều chỉnh đợt thu','CB-00001',$3)`, a, id, reason)
		return err
	}
	mustOK(t, "ordinary batch (reason NULL)", insertBatch("d1", nil))
	mustOK(t, "adjustment batch with a reason", insertBatch("d2", "Bù đợt ghi sót tháng 9"))
	mustRefuse(t, "adjustment batch with a blank reason", insertBatch("d3", " "))

	_, err := db.Exec(`UPDATE dot_thu_chi SET adjustment_reason = 'sửa' WHERE tenant_id = $1 AND id = 'd1'`, a)
	mustRefuse(t, "adding a reason to a recorded batch (0008's freeze covers the new column)", err)
}
