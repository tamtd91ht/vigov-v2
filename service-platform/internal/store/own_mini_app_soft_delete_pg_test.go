package store

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/migrations"
)

// Integration test for migration 0021 (switched-off own Mini App rows → soft-deleted, ADR 0070
// §Sửa đổi 05/10/2026 #2) against a real PostgreSQL — skipped without VIGOV_TEST_DSN.
//
// HOW THE FILE IS EXERCISED: chayMigration applies every file to an EMPTY schema, where 0021 finds
// nothing. The rows are seeded afterwards and the file's own SQL — the embedded bytes, not a copy —
// is executed again, exactly as a person re-running it by hand would. That second execution is the
// one under test; a third proves the re-run touches nothing. The REVERSAL is read out of the file's
// comment block and run too, so the prose cannot drift from something that works.

const file0021 = "0021_own_mini_app_switched_off_to_soft_delete.sql"

const reason0021 = "Chuyển sang xoá mềm theo quyết định chủ dự án 05/10/2026 (ADR 0070 §Sửa đổi)"

func sql0021(t *testing.T) string {
	t.Helper()
	b, err := fs.ReadFile(migrations.FS, file0021)
	if err != nil {
		t.Fatalf("read %s: %v", file0021, err)
	}
	return string(b)
}

// reversal0021 extracts the commented REVERSAL statement: the lines from "WITH undone AS (" to
// "FROM undone;", each stripped of its "--   " prefix.
func reversal0021(t *testing.T) string {
	t.Helper()
	var out []string
	in := false
	for _, l := range strings.Split(sql0021(t), "\n") {
		body := strings.TrimPrefix(strings.TrimRight(l, "\r"), "--   ")
		if strings.HasPrefix(body, "WITH undone AS (") {
			in = true
		}
		if in {
			out = append(out, body)
		}
		if in && strings.HasPrefix(body, "FROM undone;") {
			return strings.Join(out, "\n")
		}
	}
	t.Fatalf("%s: REVERSAL statement not found in its comment block", file0021)
	return ""
}

type miniAppState struct {
	active                    bool
	deletedAt, deletedBy, why sql.NullString
	updatedBy                 string
}

func readMiniApp(t *testing.T, db *sql.DB, appID string) miniAppState {
	t.Helper()
	var s miniAppState
	err := db.QueryRow(`SELECT dang_hoat_dong, deleted_at::text, deleted_by, delete_reason, cap_nhat_boi
		FROM mini_app WHERE app_id = $1`, appID).
		Scan(&s.active, &s.deletedAt, &s.deletedBy, &s.why, &s.updatedBy)
	if err != nil {
		t.Fatalf("read mini app %s: %v", appID, err)
	}
	return s
}

func TestPgMigration0021SwitchedOffOwnAppsBecomeSoftDeleted(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	// A — active commune: one switched-off own app (the legacy state), one running own app, one
	//     already removed by an operator (must keep ITS who/why).
	// B — inactive (merged) commune with a switched-off own app: left unchanged.
	// ulidNew — active, but recorded as a PREDECESSOR in tenant_succession: left unchanged.
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Cũ", false)
	themXa(t, db, ulidNew, "Xã Tiền Thân", true)
	if _, err := db.Exec(`INSERT INTO tenant_succession (tu_id, den_id, can_cu, hieu_luc_tu)
		VALUES ($1, $2, 'NQ-TEST', '2026-07-01')`, ulidNew, ulidA); err != nil {
		t.Fatal(err)
	}
	themMiniApp(t, db, "3291993990104489440", "rieng", ulidA, false) // switched off 01/10
	themMiniApp(t, db, "3043188591857102858", "rieng", ulidA, true)  // running
	themMiniApp(t, db, "1003", "rieng", ulidA, false)
	if _, err := db.Exec(`UPDATE mini_app SET deleted_at = now(), deleted_by = 'VH-00001', delete_reason = 'gỡ tay'
		WHERE app_id = '1003'`); err != nil {
		t.Fatal(err)
	}
	themMiniApp(t, db, "1004", "chinh", nil, false) // the shared app, switched off: never touched
	themMiniApp(t, db, "2001", "rieng", ulidB, false)
	themMiniApp(t, db, "2002", "rieng", ulidNew, false)

	if _, err := db.Exec(sql0021(t)); err != nil {
		t.Fatalf("run %s: %v", file0021, err)
	}

	// Converted: exactly the legacy row of the active commune.
	got := readMiniApp(t, db, "3291993990104489440")
	if !got.deletedAt.Valid || got.deletedBy.String != "system" || got.why.String != reason0021 || got.active {
		t.Errorf("legacy row not soft-deleted as specified: %+v", got)
	}
	if got.updatedBy != nguoiGhiThu {
		t.Errorf("cap_nhat_boi rewritten to %q — the last person to change the row is lost", got.updatedBy)
	}
	// Untouched.
	for _, id := range []string{"3043188591857102858", "1004", "2001", "2002"} {
		if s := readMiniApp(t, db, id); s.deletedAt.Valid {
			t.Errorf("App ID %s soft-deleted by 0021: %+v", id, s)
		}
	}
	if s := readMiniApp(t, db, "1003"); s.deletedBy.String != "VH-00001" || s.why.String != "gỡ tay" {
		t.Errorf("an operator's removal was rewritten: %+v", s)
	}

	// Trail: one entry, the row's commune, system principal, the Go path's shape.
	a := auditRows(t, db, ulidA)
	if len(a) != 1 {
		t.Fatalf("commune A audit rows = %+v, want exactly one", a)
	}
	e := a[0]
	if e.action != ActionDeactivateMiniApp || e.actorID != "system" || e.actorKind != "system" ||
		e.subject != "MiniApp 3291993990104489440" || e.delta["app_id"] != "3291993990104489440" ||
		e.delta["ly_do"] != reason0021 || e.delta["xa"] != "Xã Thăng Bình" {
		t.Errorf("audit entry = %+v", e)
	}
	before, _ := e.delta["truoc"].(map[string]any)
	after, _ := e.delta["sau"].(map[string]any)
	if before["dang_hoat_dong"] != false || before["da_xoa_mem"] != false ||
		after["dang_hoat_dong"] != false || after["da_xoa_mem"] != true {
		t.Errorf("delta truoc/sau = %v / %v", before, after)
	}
	for _, other := range []string{ulidB, ulidNew} {
		if n := count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, other); n != 0 {
			t.Errorf("commune %s got %d audit entries from 0021", other, n)
		}
	}

	// Sign-in still refuses the App ID; the operator path sees it as already removed.
	if _, err := NewDirectory(db, "").MiniApp(context.Background(), "3291993990104489440"); !errors.Is(err, ErrKhongCoMiniApp) {
		t.Errorf("converted App ID resolves: %v", err)
	}
	w := NewRegistryWriter(corestore.New(db))
	if changed, err := w.RemoveMiniApp(inCommune(ulidA), "3291993990104489440", "lặp", operatorFake); err != nil || changed {
		t.Errorf("RemoveMiniApp on a converted row: changed=%v err=%v, want no-op", changed, err)
	}

	// Re-run: nothing changes, no second entry.
	stamp := readMiniApp(t, db, "3291993990104489440").deletedAt
	if _, err := db.Exec(sql0021(t)); err != nil {
		t.Fatalf("re-run %s: %v", file0021, err)
	}
	if s := readMiniApp(t, db, "3291993990104489440"); s.deletedAt != stamp {
		t.Errorf("re-run moved deleted_at: %v → %v", stamp, s.deletedAt)
	}
	if n := count(t, db, `SELECT count(*) FROM audit_log WHERE action = 'tat_mini_app'`); n != 1 {
		t.Errorf("after re-run tat_mini_app entries = %d, want 1", n)
	}

	// Reversal from the file's comment block: exactly the converted row comes back, with its own entry.
	if _, err := db.Exec(reversal0021(t)); err != nil {
		t.Fatalf("reversal: %v", err)
	}
	if s := readMiniApp(t, db, "3291993990104489440"); s.deletedAt.Valid || s.deletedBy.Valid || s.why.Valid || s.active {
		t.Errorf("reversal did not restore the legacy row: %+v", s)
	}
	if s := readMiniApp(t, db, "1003"); s.deletedBy.String != "VH-00001" {
		t.Errorf("reversal touched an operator's removal: %+v", s)
	}
	if n := count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = 'hoan_xoa_mem_mini_app'
		AND subject = 'MiniApp 3291993990104489440'`, ulidA); n != 1 {
		t.Errorf("reversal entries = %d, want 1", n)
	}
}
