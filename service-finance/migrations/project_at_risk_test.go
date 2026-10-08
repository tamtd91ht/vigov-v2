package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0018 (`du_an.at_risk`, ADR 0080 #2). Same caveat as 0016's test:
// this proves the SQL is WRITTEN, not that PostgreSQL accepts it —
// internal/store/project_at_risk_pg_test.go does that when VIGOV_TEST_DSN is set.

const file0018 = "0018_project_at_risk_flag.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a nullable column (a third state "unknown" every reader would
// have to guess at), a default of true (every project of every commune flagged at once), a missing
// IF NOT EXISTS (a retry fails), the column landing on another table.
func TestMigration0018Shape(t *testing.T) {
	sql := maChay(t, file0018)
	want := "alter table du_an add column if not exists at_risk boolean not null default false;"
	if !strings.Contains(sql, want) {
		t.Fatalf("0018 lacks %q — got:\n%s", want, sql)
	}
	// One statement, nothing else: the file is exactly the column.
	if got := strings.Count(sql, ";"); got != 1 {
		t.Fatalf("0018 has %d statements, want 1", got)
	}
}

// Only ADDS: no existing row, column, constraint, key or partition is touched.
func TestMigration0018DestroysNothing(t *testing.T) {
	sql := maChay(t, file0018)
	for _, banned := range []string{"drop ", "alter column", "delete from", "truncate", "insert into",
		"update ", "primary key", "partition", "default true", "trigger"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0018 contains %q", banned)
		}
	}
}
