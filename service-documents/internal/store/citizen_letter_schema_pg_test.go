package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for migration 0006 (the citizen-letter register) against a real PostgreSQL.
//
// WHY A REAL DATABASE: every property below is a CHECK, a key or a trigger. A fake agrees with
// whatever it was told; only PostgreSQL can refuse the statement.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — and the package still prints `ok`. A green run without the
// DSN says nothing about the SQL; the always-on half is migrations/citizen_letter_test.go. The schema
// comes from TestMain in loai_van_ban_pg_test.go, which applies every migration through the real
// runner.
//
// Test data uses the agreed fake phone number (rule 3, invariant 5).

// seriesCitizenLetter is the third `so_sach` value 0006 adds. Spelled as a literal here on purpose:
// the Go constant belongs to the store card (B2); this test proves the DATABASE accepts the value.
const seriesCitizenLetter SoSach = "don-thu"

// insertLetter books one letter by raw SQL. `extra` is a column suffix from extraValues ("" for none).
func insertLetter(ctx context.Context, db *sql.DB, tenantID, id string, number int, extra string) error {
	vals := ""
	if extra != "" {
		vals = ", " + extraValues(extra)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO citizen_letter
		(tenant_id, id, number, year, received_date, letter_type, summary, created_by_code`+extra+`)
		VALUES ($1, $2, $3, 2026, DATE '2026-10-01', 'kien-nghi-phan-anh', 'Đề nghị sửa đường', 'CB-TEST01'`+
		vals+`)`, tenantID, id, number)
	return err
}

// extraValues maps the optional column suffix used by the tests to its literal values.
func extraValues(extra string) string {
	switch extra {
	case ", status":
		return "'thu-ly'"
	case ", status, accepted_at":
		return "'thu-ly', now()"
	case ", status, accepted_at, resolved_at":
		return "'da-giai-quyet', now(), now()"
	case ", sender_name":
		return "''"
	case ", sender_name, sender_phone":
		return "'Nguyễn Văn A', '0900000000'"
	}
	panic("unknown extra: " + extra)
}

func mustRefuse(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Errorf("PostgreSQL accepted %s — the schema should refuse it", what)
	}
}

func mustAccept(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// The letter series is a third, independent series on 0004's counter: booking letters does not move
// the incoming-document series, and the counter accepts 'don-thu' at all (the widened CHECK).
func TestPgCitizenLetterSeriesIsIndependent(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	ctx := ctxXa(a)
	series := NewDaySoStore(pkgstore.New(db))

	issue := func(s SoSach) int {
		t.Helper()
		var n int
		err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			var err error
			n, err = series.CapSo(ctx, tx, s, 2026)
			return err
		})
		mustAccept(t, err, fmt.Sprintf("issue %s", s))
		return n
	}
	if got := issue(seriesCitizenLetter); got != 1 {
		t.Errorf("first letter number = %d, want 1", got)
	}
	if got := issue(SoSachDen); got != 1 {
		t.Errorf("first incoming number after a letter = %d, want 1 — the series are not independent", got)
	}
	if got := issue(seriesCitizenLetter); got != 2 {
		t.Errorf("second letter number = %d, want 2", got)
	}
}

// An issued number is never rewritten and never reissued, even after a soft delete; a row is never
// hard-deleted.
func TestPgCitizenLetterNumberImmutableAndNotReissued(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	ctx := context.Background()

	mustAccept(t, insertLetter(ctx, db, a, "L1", 1, ""), "book letter 1")

	_, err := db.ExecContext(ctx, `UPDATE citizen_letter SET number = 99 WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustRefuse(t, err, "renumbering an issued letter")
	_, err = db.ExecContext(ctx, `UPDATE citizen_letter SET year = 2027 WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustRefuse(t, err, "moving a letter to another year")
	_, err = db.ExecContext(ctx, `UPDATE citizen_letter SET created_by_code = 'CB-OTHER' WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustRefuse(t, err, "rewriting who booked the letter")

	_, err = db.ExecContext(ctx, `UPDATE citizen_letter SET status = 'dang-xu-ly-don', holding_unit_id = 'BP-1'
		WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustAccept(t, err, "ordinary register work (status, holding unit) must stay possible")

	_, err = db.ExecContext(ctx, `UPDATE citizen_letter SET deleted_at = now(), deleted_by = 'CB-TEST01',
		delete_reason = 'vào sổ nhầm' WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustAccept(t, err, "soft delete")
	mustRefuse(t, insertLetter(ctx, db, a, "L2", 1, ""), "reissuing number 1 after a soft delete")

	_, err = db.ExecContext(ctx, `DELETE FROM citizen_letter WHERE tenant_id = $1 AND id = 'L1'`, a)
	mustRefuse(t, err, "a hard delete of an archival letter")
}

// The statuses bind their timestamps and the closing result; optional sender fields are NULL, never ”.
func TestPgCitizenLetterBindings(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	ctx := context.Background()

	mustRefuse(t, insertLetter(ctx, db, a, "B1", 1, ", status"), "thu-ly without accepted_at")
	mustAccept(t, insertLetter(ctx, db, a, "B2", 2, ", status, accepted_at"), "thu-ly with accepted_at")
	mustRefuse(t, insertLetter(ctx, db, a, "B3", 3, ", status, accepted_at, resolved_at"),
		"da-giai-quyet without its result document and summary (C10)")
	mustRefuse(t, insertLetter(ctx, db, a, "B4", 4, ", sender_name"), "a blank sender name instead of NULL")
	mustAccept(t, insertLetter(ctx, db, a, "B5", 5, ""), "an anonymous letter (C7: all sender fields NULL)")
	mustAccept(t, insertLetter(ctx, db, a, "B6", 6, ", sender_name, sender_phone"), "a named letter")

	_, err := db.ExecContext(ctx, `UPDATE citizen_letter SET related_letter_id = 'B6'
		WHERE tenant_id = $1 AND id = 'B6'`, a)
	mustRefuse(t, err, "a letter marked as a duplicate of itself")
}

// A confirmed-duplicate link cannot cross communes (rule 1).
func TestPgCitizenLetterRelatedLinkStaysInCommune(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	ctx := context.Background()

	mustAccept(t, insertLetter(ctx, db, a, "X1", 1, ""), "commune A letter")
	mustAccept(t, insertLetter(ctx, db, b, "X2", 1, ""), "commune B letter")

	_, err := db.ExecContext(ctx, `UPDATE citizen_letter SET related_letter_id = 'X1'
		WHERE tenant_id = $1 AND id = 'X2'`, b)
	mustRefuse(t, err, "commune B linking to commune A's letter")

	mustAccept(t, insertLetter(ctx, db, b, "X3", 2, ""), "second commune B letter")
	_, err = db.ExecContext(ctx, `UPDATE citizen_letter SET related_letter_id = 'X2'
		WHERE tenant_id = $1 AND id = 'X3'`, b)
	mustAccept(t, err, "a same-commune duplicate link")
}

// The log is append-only, and its rows are bound to their kind.
func TestPgCitizenLetterLogAppendOnly(t *testing.T) {
	db := moKetNoi(t)
	a, _ := xaRieng(t)
	ctx := context.Background()

	ins := func(id, kind, fromStatus, toStatus, toUnit, content string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO citizen_letter_log
			(tenant_id, id, letter_id, at, actor_code, kind, from_status, to_status, to_unit_id, content)
			VALUES ($1, $2, 'L1', now(), 'CB-TEST01', $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''))`,
			a, id, kind, fromStatus, toStatus, toUnit, content)
		return err
	}

	mustAccept(t, ins("G1", "ghi-chu", "", "", "", "Đã gọi điện hẹn"), "a note with no status change")
	mustRefuse(t, ins("G2", "ghi-chu", "", "", "", ""), "an empty note")
	mustRefuse(t, ins("G3", "ghi-chu", "moi-vao-so", "dang-xu-ly-don", "", "x"), "a note carrying a status change")
	mustRefuse(t, ins("G4", "chuyen-trang-thai", "", "", "", ""), "a status change with no status")
	mustRefuse(t, ins("G5", "chuyen-trang-thai", "thu-ly", "thu-ly", "", ""), "a status change to the same status")
	mustRefuse(t, ins("G6", "luan-chuyen", "", "", "", "chuyển"), "a routing with no destination unit")
	mustRefuse(t, ins("G7", "ghi-chu", "", "", "BP-1", "x"), "a stray unit on a note row")
	mustAccept(t, ins("G8", "luan-chuyen", "moi-vao-so", "dang-xu-ly-don", "BP-1", "Chuyển địa chính"),
		"a routing that also moves the status")

	_, err := db.ExecContext(ctx, `UPDATE citizen_letter_log SET content = 'sửa' WHERE tenant_id = $1 AND id = 'G1'`, a)
	mustRefuse(t, err, "editing a log entry")
	_, err = db.ExecContext(ctx, `DELETE FROM citizen_letter_log WHERE tenant_id = $1 AND id = 'G1'`, a)
	mustRefuse(t, err, "removing a log entry")
	_, err = db.ExecContext(ctx, `TRUNCATE citizen_letter_log_p00`)
	mustRefuse(t, err, "truncating a log partition")
}
