package store

import (
	"context"
	"testing"
)

// Integration tests for migration 0011 against a real PostgreSQL — skipped without VIGOV_TEST_DSN,
// like every *_pg_test.go here. The CHECKs, the guard trigger and the seed are behaviour the database
// owns.

func TestPgCitizenReportFieldSeedAndRead(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)

	fs, err := NewCitizenReportFieldStore(db).ListCitizenReportFields(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(fs) != 12 {
		t.Fatalf("fields = %d, want the 12 seeded", len(fs))
	}
	if fs[0].Code != "rac-thai" || fs[11].Code != "khac" || !fs[0].IsActive || fs[0].Icon != "Trash2" {
		t.Errorf("order or content wrong: first=%+v last=%+v", fs[0], fs[11])
	}

	var entries int
	if err := db.QueryRow(`SELECT count(*) FROM platform_audit_log
		WHERE action = 'petition_field.seeded' AND actor = 'system'`).Scan(&entries); err != nil {
		t.Fatalf("count trail: %v", err)
	}
	if entries != 12 {
		t.Errorf("seed trail entries = %d, want 12 — one per seeded code, same transaction", entries)
	}
}

// A retired code is still read — old petitions keep their label.
func TestPgCitizenReportFieldRetiredIsStillRead(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	if _, err := db.Exec(`UPDATE petition_field SET active = false, updated_by = 'system'
		WHERE code = 'dien'`); err != nil {
		t.Fatalf("retire: %v", err)
	}
	fs, err := NewCitizenReportFieldStore(db).ListCitizenReportFields(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, f := range fs {
		if f.Code == "dien" {
			found = true
			if f.IsActive {
				t.Error("dien still active after retirement")
			}
		}
	}
	if !found || len(fs) != 12 {
		t.Errorf("retired code missing from the read (found=%v, n=%d)", found, len(fs))
	}
}

func TestPgCitizenReportFieldConstraints(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	// Each statement breaks exactly one rule. An UPDATE that matched no row would return no error and
	// be reported as accepted — a false red, never a false green.
	for name, stmt := range map[string]string{
		"hard delete refused": `DELETE FROM petition_field WHERE code = 'khac'`,
		"rename refused":      `UPDATE petition_field SET code = 'khac-2' WHERE code = 'khac'`,
		"blank label":         `UPDATE petition_field SET default_label = ' ' WHERE code = 'khac'`,
		"sort order zero":     `UPDATE petition_field SET sort_order = 0 WHERE code = 'khac'`,
		"unknown tone":        `UPDATE petition_field SET tone = 'pink' WHERE code = 'khac'`,
		"blank signer":        `UPDATE petition_field SET updated_by = ' ' WHERE code = 'khac'`,
		"code with diacritic": `INSERT INTO petition_field (code, default_label, sort_order, active, created_by, updated_by) VALUES ('điện', 'x', 1, true, 'system', 'system')`,
		"code upper case":     `INSERT INTO petition_field (code, default_label, sort_order, active, created_by, updated_by) VALUES ('Moi', 'x', 1, true, 'system', 'system')`,
		"active unstated":     `INSERT INTO petition_field (code, default_label, sort_order, created_by, updated_by) VALUES ('moi', 'x', 1, 'system', 'system')`,
	} {
		if _, err := db.Exec(stmt); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}
	// Editing a default label IS allowed — only the code is frozen.
	if _, err := db.Exec(`UPDATE petition_field SET default_label = 'Khác (khác)', updated_by = 'system'
		WHERE code = 'khac'`); err != nil {
		t.Errorf("label edit refused: %v", err)
	}
}
