package store

// Checks on migration 0012 (ADR 0061 layer B) and its reverse script, WITHOUT a database.
// rename_schema_pg_test.go runs forward → reverse → forward for real; these pin what that test can
// only see when a DSN is set, so a reverse script that drifted from its migration turns red on every
// machine.

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/migrations"
)

const (
	renameMigration = "0012_rename_schema_to_english.sql"
	// Relative to this package directory; the file is deliberately NOT in the embedded FS.
	renameReversePath = "../../migrations/reverse/" + renameMigration
)

func readRenameFiles(t *testing.T) (forward, reverse string) {
	t.Helper()
	f, err := fs.ReadFile(migrations.FS, renameMigration)
	if err != nil {
		t.Fatalf("read %s: %v", renameMigration, err)
	}
	r, err := os.ReadFile(renameReversePath)
	if err != nil {
		t.Fatalf("read %s: %v", renameReversePath, err)
	}
	return string(f), string(r)
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(migrations.FS, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// renameStmt matches every rename spelling the two files use. The DO blocks' `%I` renames do not
// match (no \w), which is right: they are rule-based and identical in both files.
var renameStmt = regexp.MustCompile(`(?i)ALTER\s+(TABLE|INDEX|FUNCTION|TRIGGER)\s+(\w+)(?:\(\))?` +
	`(?:\s+ON\s+(\w+))?\s+RENAME\s+(?:(COLUMN|CONSTRAINT)\s+(\w+)\s+)?TO\s+(\w+)`)

// renames returns every rename as "kind table from to", in file order.
//
// Column, constraint and trigger renames name their table by its NEW name in both files (0012
// renames a table before its columns; the reverse renames the columns before the table), so the
// table part of a key is the same on both sides.
func renames(sql string) []string {
	var out []string
	for _, m := range renameStmt.FindAllStringSubmatch(sqlLineComment.ReplaceAllString(sql, ""), -1) {
		kind, obj, on, sub, from, to := strings.ToLower(m[1]), m[2], m[3], strings.ToLower(m[4]), m[5], m[6]
		switch {
		case sub != "":
			out = append(out, sub+" "+obj+" "+from+" "+to)
		case kind == "trigger":
			out = append(out, "trigger "+on+" "+obj+" "+to)
		default:
			out = append(out, kind+" - "+obj+" "+to)
		}
	}
	return out
}

func invert(key string) string {
	f := strings.Fields(key)
	return f[0] + " " + f[1] + " " + f[3] + " " + f[2]
}

// THE REVERSE IS THE EXACT INVERSE: every rename 0012 makes is undone once, and the reverse
// renames nothing 0012 did not. A rename missing from the reverse leaves one English name behind
// after a rollback; an extra one renames an object the previous image never had.
func TestRenameReverseUndoesEveryRename(t *testing.T) {
	forward, reverse := readRenameFiles(t)
	fw, rv := renames(forward), renames(reverse)
	if len(fw) < 60 {
		t.Fatalf("0012 has %d renames — the pattern no longer reads the file", len(fw))
	}
	want := map[string]int{}
	for _, k := range fw {
		want[invert(k)]++
	}
	got := map[string]int{}
	for _, k := range rv {
		got[k]++
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("reverse has %d × %q, want %d", got[k], k, n)
		}
	}
	for k, n := range got {
		if want[k] == 0 {
			t.Errorf("reverse renames %q (%d×), which 0012 never did", k, n)
		}
	}
}

// The five technical names the dictionary does not list (named by the coordinator 2026-09-29), and
// the one column whose dictionary name was corrected: each must be renamed forward AND back. The
// general pairing test above cannot tell a name that was never renamed from one renamed correctly.
func TestRenameTechnicalNamesArePaired(t *testing.T) {
	forward, reverse := readRenameFiles(t)
	fw, rv := map[string]bool{}, map[string]bool{}
	for _, k := range renames(forward) {
		fw[k] = true
	}
	for _, k := range renames(reverse) {
		rv[k] = true
	}
	for _, k := range []string{
		"constraint tenant_domain tenant_domain_host_thuong tenant_domain_host_lowercase",
		"index - tenant_domain_mot_chinh tenant_domain_one_primary",
		"constraint tenant_succession tenant_succession_khong_tro_chinh_no tenant_succession_not_self",
		"function - tenant_succession_chan_vong_lap tenant_succession_no_cycle",
		"trigger tenant_succession tenant_succession_chan_vong_lap tenant_succession_no_cycle",
		"constraint mini_app mini_app_xa_khi_va_chi_khi_rieng mini_app_tenant_iff_dedicated",
		"column tenant tinh_thanh province_name",
	} {
		if !fw[k] {
			t.Errorf("0012 lacks %q", k)
		}
		if !rv[invert(k)] {
			t.Errorf("reverse lacks %q", invert(k))
		}
	}
	if strings.Contains(sqlLineComment.ReplaceAllString(forward, ""), "province_code") {
		t.Error("0012 still names province_code — the column holds a display name, not a code")
	}
}

// dollarBody returns the $$…$$ body of `CREATE OR REPLACE FUNCTION name()` in sql, verbatim.
func dollarBody(t *testing.T, sql, file, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)CREATE OR REPLACE FUNCTION ` + regexp.QuoteMeta(name) +
		`\(\) RETURNS trigger\s+LANGUAGE plpgsql AS \$\$(.*?)\$\$;`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("%s: no body for %s()", file, name)
	}
	return m[1]
}

// Bodies: the reverse restores 0003's and 0011's text EXACTLY, and 0012's is that same text with
// only the renamed identifiers changed — the cycle walk and the two refusals keep their logic.
func TestRenameFunctionBodies(t *testing.T) {
	forward, reverse := readRenameFiles(t)
	succ0003 := dollarBody(t, readMigration(t, "0003_tenant_succession.sql"), "0003", "tenant_succession_chan_vong_lap")
	guard0011 := dollarBody(t, readMigration(t, "0011_petition_field.sql"), "0011", "petition_field_guard")

	// The reverse restores the body under the CURRENT name, then renames the function back.
	if got := dollarBody(t, reverse, "reverse", "tenant_succession_no_cycle"); got != succ0003 {
		t.Errorf("reverse does not restore 0003's cycle guard verbatim:\n%s", got)
	}
	if got := dollarBody(t, reverse, "reverse", "citizen_report_field_guard"); got != guard0011 {
		t.Errorf("reverse does not restore 0011's guard verbatim:\n%s", got)
	}

	wantSucc := strings.NewReplacer("den_id", "to_tenant_id", "tu_id", "from_tenant_id").Replace(succ0003)
	succ0012 := dollarBody(t, forward, "0012", "tenant_succession_chan_vong_lap")
	if succ0012 != wantSucc {
		t.Errorf("0012's cycle guard is not 0003's with the new column names:\n%s", succ0012)
	}
	if strings.Contains(succ0012, "tu_id") || strings.Contains(succ0012, "den_id") {
		t.Error("0012's cycle guard still reads an old column — the first INSERT would fail")
	}
	wantGuard := strings.NewReplacer("'petition_field: ", "'citizen_report_field: ",
		"active = false", "is_active = false").Replace(guard0011)
	if got := dollarBody(t, forward, "0012", "petition_field_guard"); got != wantGuard {
		t.Errorf("0012's field guard is not 0011's with the new names:\n%s", got)
	}
	// Both refusals must survive: TG_OP DELETE and a changed code.
	for _, clause := range []string{"IF TG_OP = 'DELETE' THEN", "IF NEW.code IS DISTINCT FROM OLD.code THEN"} {
		if !strings.Contains(wantGuard, clause) {
			t.Errorf("0012's field guard lost %q", clause)
		}
	}
}

// commentText returns the literal(s) of `COMMENT ON <what> IS '…';`, verbatim. It ends at `';`, not
// at the first `;` — 0006's comment on mini_app.tenant_id has one inside its text.
func commentText(t *testing.T, sql, file, what string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)COMMENT ON ` + regexp.QuoteMeta(what) + ` IS\s+(.*?');`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("%s: no COMMENT ON %s", file, what)
	}
	return m[1]
}

func TestRenameReverseRestoresComments(t *testing.T) {
	_, reverse := readRenameFiles(t)
	for _, c := range []struct{ file, before, after string }{
		{"0004_tinh_thanh.sql", "TABLE tinh_thanh", "TABLE tinh_thanh"},
		{"0006_mini_app_va_ho_so_hien_thi.sql", "COLUMN mini_app.tenant_id", "COLUMN mini_app.tenant_id"},
		{"0011_petition_field.sql", "TABLE petition_field", "TABLE petition_field"},
	} {
		want := commentText(t, readMigration(t, c.file), c.file, c.before)
		if got := commentText(t, reverse, "reverse", c.after); got != want {
			t.Errorf("reverse COMMENT ON %s = %s, want %s's verbatim %s", c.after, got, c.file, want)
		}
	}
}

// The ownership index reads `@entity` above `ALTER TABLE … RENAME TO` (tools/kb, ADR 0061 layer 0).
// A mark one line off declares nothing, and the index keeps the old table name.
func TestRenameEntityMarks(t *testing.T) {
	forward, _ := readRenameFiles(t)
	for _, want := range []string{
		"-- @entity: Province\n-- @scope:  platform\nALTER TABLE tinh_thanh RENAME TO province;",
		"-- @entity: CommuneProfile\n-- @scope:  tenant\nALTER TABLE ho_so_hien_thi_xa RENAME TO commune_profile;",
		"-- @entity: CitizenReportField\n-- @scope:  platform\nALTER TABLE petition_field RENAME TO citizen_report_field;",
	} {
		if !strings.Contains(forward, want) {
			t.Errorf("0012 lacks the mark block:\n%s", want)
		}
	}
}

// Rename only — nothing that copies, removes or rewrites rows, nothing that breaks core/migrate's
// one-transaction guarantee, and no new index (a new index would be judged against the nearest
// @scope mark by tools/check_khoa_duy_nhat.py, which here belongs to a rename).
func TestRenameMigrationIsRenameOnly(t *testing.T) {
	forward, _ := readRenameFiles(t)
	code := sqlLineComment.ReplaceAllString(forward, "")
	for name, re := range map[string]*regexp.Regexp{
		"transaction control":    regexp.MustCompile(`(?im)^\s*(BEGIN|COMMIT|ROLLBACK|END)\s*;`),
		"CONCURRENTLY":           regexp.MustCompile(`(?i)\bCONCURRENTLY\b`),
		"new index":              regexp.MustCompile(`(?i)\bCREATE\s+(UNIQUE\s+)?INDEX\b`),
		"new table":              regexp.MustCompile(`(?i)\bCREATE\s+TABLE\b`),
		"drop":                   regexp.MustCompile(`(?i)\bDROP\s+(TABLE|COLUMN|INDEX|VIEW|FUNCTION|TRIGGER|CONSTRAINT)\b`),
		"truncate":               regexp.MustCompile(`(?i)\bTRUNCATE\b`),
		"row write":              regexp.MustCompile(`(?i)\b(INSERT\s+INTO|UPDATE\s+\w+\s+SET|DELETE\s+FROM)\b`),
		"type change":            regexp.MustCompile(`(?i)\bALTER\s+COLUMN\b`),
		"validating the CHECK":   regexp.MustCompile(`(?i)\bVALIDATE\s+CONSTRAINT\b`),
		"alias that is writable": regexp.MustCompile(`(?i)\bADD\s+COLUMN\s+\w+\s+\w+\s+(NOT\s+NULL|DEFAULT)\b`),
	} {
		if loc := re.FindStringIndex(code); loc != nil {
			t.Errorf("0012 contains %s: %q", name, code[loc[0]:loc[1]])
		}
	}
}

// THE REVERSE MUST NOT SHIP. embed.go embeds `*.sql` of the migrations directory only; a reverse
// script inside the embedded FS would be applied by core/migrate at the next startup.
func TestRenameReverseIsNotEmbedded(t *testing.T) {
	if _, err := fs.Stat(migrations.FS, "reverse"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("migrations.FS contains reverse/ (err = %v) — the runner could apply it", err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name()), "reverse") {
			t.Errorf("embedded migration %s looks like a reverse script", e.Name())
		}
	}
}

// The reverse is one transaction and removes 0012's progress row inside it, keyed on the exact
// file name — otherwise the runner believes the renamed schema is still in place.
func TestRenameReverseTransactionAndProgressRow(t *testing.T) {
	_, reverse := readRenameFiles(t)
	code := strings.TrimSpace(sqlLineComment.ReplaceAllString(reverse, ""))
	if !strings.HasPrefix(code, "BEGIN;") || !strings.HasSuffix(code, "COMMIT;") {
		t.Error("reverse is not wrapped in BEGIN; … COMMIT;")
	}
	row := "DELETE FROM schema_migration WHERE ten = '" + renameMigration + "';"
	if strings.Count(code, row) != 1 {
		t.Errorf("reverse must remove its progress row exactly once with %q", row)
	}
}
