package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run against the REAL migrations, not a fixture. A fixture would have proved the
// regex works on text shaped the way the test author imagined; what actually broke was the
// distance between the mark and the table in the repository's own house style.

func khoRoot(t *testing.T) string {
	t.Helper()
	_, tep, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("không xác định được đường dẫn tệp test")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(tep)))
}

func quet(t *testing.T) ([]soHuu, []string) {
	t.Helper()
	root := khoRoot(t)
	sv, err := scanServices(root)
	if err != nil {
		t.Fatalf("scanServices: %v", err)
	}
	rows, canhBao, err := quetSoHuu(root, sv)
	if err != nil {
		t.Fatalf("quetSoHuu: %v", err)
	}
	return rows, canhBao
}

func TestChiMucSoHuuKHONGDuocRONG(t *testing.T) {
	// THE CASE THAT WOULD HAVE CAUGHT THE ORIGINAL DEFECT. For months this index said
	// "no schemas defined yet" while the marks were there, and CLAUDE.md step 4 sends every
	// session here FIRST with "do NOT read every schema". An empty index does not look broken,
	// so nobody asked — which is why the floor is asserted rather than trusted.
	rows, _ := quet(t)
	if len(rows) < 15 {
		t.Fatalf("chỉ mục sở hữu có %d thực thể — quá ít, bộ đọc nhiều khả năng đã chết", len(rows))
	}
}

func TestMoiDauEntityDeuTimDuocBang(t *testing.T) {
	// THE EXACT REGRESSION. The first version stopped looking 12 lines after the mark and lost
	// 11 of 20 entities, because this repo puts a long WHY comment between the mark and the
	// table. It produced an index that looked populated and was missing over half — worse than
	// the placeholder, because a populated-looking index gets believed.
	_, canhBao := quet(t)
	for _, c := range canhBao {
		if strings.Contains(c, "không có CREATE TABLE") {
			t.Errorf("dấu @entity không tìm được bảng: %s", c)
		}
	}
}

func TestKhoangCachXaGiuaDauVaBangVanDoc(t *testing.T) {
	// Petition's mark sits 13 lines above its CREATE TABLE — one line past the window that
	// broke. Named explicitly so that shortening the walk fails here with the reason, not
	// somewhere downstream with a smaller number.
	rows, _ := quet(t)
	for _, r := range rows {
		if r.Entity == "Petition" {
			if r.Table != "phieu_phan_anh" || r.Service != "petitions" {
				t.Errorf("Petition: got %s/%s, want petitions/phieu_phan_anh", r.Service, r.Table)
			}
			return
		}
	}
	t.Error("không thấy thực thể Petition — bộ đọc bỏ sót đúng ca có khoảng cách xa nhất")
}

func TestMoiThucTheCoDUNGMOTChuSoHuu(t *testing.T) {
	// Rule 2, invariant 1. Two services declaring one entity is a distributed monolith forming,
	// and this generated index is the only place it is visible at a glance.
	rows, _ := quet(t)
	for _, c := range trung(rows) {
		t.Error(c)
	}
}

// ---- RENAME folding (ADR 0061, layer 0) -----------------------------------------------------
//
// Layer B renames tables with `ALTER TABLE <old> RENAME TO <new>` in a NEW migration, with the
// `@entity` mark above that statement. These tests use a FIXTURE on purpose — unlike the ones
// above — because on the day they were written no migration in the repository carried a rename,
// and the shape layer B will write is exactly what has to be pinned before it exists.

// writeFixture lays out root/service-platform/migrations/<name> for each entry.
func writeFixture(t *testing.T, files map[string]string) (string, []serviceEntry) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "service-platform", "migrations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, []serviceEntry{{Name: "platform", Path: "service-platform"}}
}

const fixtureCreate = `-- @entity: PetitionField
-- @scope:  platform
CREATE TABLE IF NOT EXISTS petition_field (
    code   TEXT PRIMARY KEY,
    active BOOLEAN NOT NULL
);

-- @entity: TenantDisplayProfile
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS ho_so_hien_thi_xa (
    tenant_id TEXT PRIMARY KEY
);

-- @entity: Province
-- @scope:  platform
CREATE TABLE IF NOT EXISTS tinh_thanh (
    code TEXT PRIMARY KEY
);
`

const fixtureRename = `-- The reversal is written as PROSE here and must not be folded:
--   ALTER TABLE citizen_report_field RENAME TO petition_field;

-- @entity: CitizenReportField
-- @scope:  platform
ALTER TABLE petition_field RENAME TO citizen_report_field;
ALTER TABLE citizen_report_field RENAME COLUMN active TO is_active;

-- @entity: CommuneProfile
-- @scope:  tenant
ALTER TABLE IF EXISTS ho_so_hien_thi_xa
    RENAME TO commune_profile;

-- No mark: the entity keeps its name and its declaration, only the table moves.
DO $$
BEGIN
    ALTER TABLE tinh_thanh RENAME TO province;
END $$;
`

func TestRenameFoldsIntoTheCurrentTableName(t *testing.T) {
	root, sv := writeFixture(t, map[string]string{
		"0001_init.sql":                     fixtureCreate,
		"0002_rename_schema_to_english.sql": fixtureRename,
	})
	rows, warnings, err := quetSoHuu(root, sv)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		t.Errorf("unexpected warning: %s", w)
	}

	type want struct{ table, scope, source, former string }
	wants := map[string]want{
		"CitizenReportField": {"citizen_report_field", "platform",
			"service-platform/migrations/0002_rename_schema_to_english.sql:4", "petition_field"},
		"CommuneProfile": {"commune_profile", "tenant",
			"service-platform/migrations/0002_rename_schema_to_english.sql:9", "ho_so_hien_thi_xa"},
		"Province": {"province", "platform",
			"service-platform/migrations/0001_init.sql:14", "tinh_thanh"},
	}
	got := map[string]soHuu{}
	for _, r := range rows {
		got[r.Entity] = r
	}
	// THE OLD DECLARATIONS MUST BE GONE, not listed beside the new ones: two rows for one table
	// is the "both names" failure, and an index row naming a table that no longer exists sends
	// the reader to a schema object PostgreSQL will say is not there.
	for _, stale := range []string{"PetitionField", "TenantDisplayProfile"} {
		if _, ok := got[stale]; ok {
			t.Errorf("%s still listed after its table was renamed and re-declared", stale)
		}
	}
	for e, w := range wants {
		r, ok := got[e]
		if !ok {
			t.Errorf("%s missing", e)
			continue
		}
		if r.Table != w.table || r.Scope != w.scope || r.Source != w.source {
			t.Errorf("%s: got %s/%s/%s, want %s/%s/%s", e, r.Table, r.Scope, r.Source,
				w.table, w.scope, w.source)
		}
		if len(r.FormerTables) != 1 || r.FormerTables[0] != w.former {
			t.Errorf("%s: former_tables %v, want [%s]", e, r.FormerTables, w.former)
		}
	}
	if len(rows) != len(wants) {
		t.Errorf("%d rows, want %d: %+v", len(rows), len(wants), rows)
	}
}

func TestRenameThatChangesTheScopeIsReported(t *testing.T) {
	// A rename moves no data, so it cannot move a table from one commune's ownership to the
	// platform's. A mark that says otherwise is a declaration nobody made on purpose.
	root, sv := writeFixture(t, map[string]string{
		"0001_init.sql": fixtureCreate,
		"0002_rename.sql": "-- @entity: CommuneProfile\n-- @scope:  platform\n" +
			"ALTER TABLE ho_so_hien_thi_xa RENAME TO commune_profile;\n",
	})
	_, warnings, err := quetSoHuu(root, sv)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "CommuneProfile") && strings.Contains(w, "tenant") &&
			strings.Contains(w, "platform") {
			return
		}
	}
	t.Errorf("scope change across a rename not reported; warnings: %v", warnings)
}

func TestMoiThucTheKhaiScope(t *testing.T) {
	// A missing scope is never defaulted (see quetSoHuu). Declaring a commune's table as
	// platform-wide, or the reverse, is rule 1's entire subject matter.
	rows, _ := quet(t)
	for _, r := range rows {
		switch r.Scope {
		case "tenant", "platform", "cross-tenant":
		default:
			t.Errorf("%s (%s): @scope %q không hợp lệ", r.Entity, r.Source, r.Scope)
		}
	}
}
