package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ADR 0096 (owner, 15/10/2026): only a table whose rows grow with time or with the number of
// citizens is PARTITION BY HASH (tenant_id), MODULUS 8. Every other table is a plain table. This
// list IS that decision for this service: moving a table across the line changes it here, in review.
var hashPartitionedTables = []string{
	"audit_log",
	"portal_sync_runs",
	"staff_notification",
	"stored_file",
	"thong_bao",
	"thong_bao_gui_cong_dan",
	"thong_bao_nguoi_nhan",
	"zalo_delivery",
	"zalo_group_delivery",
}

// createTableStatement returns `create table if not exists <table> ( ... ;` out of executable SQL.
// It fails rather than returning "", so a renamed table cannot read as "not partitioned".
func createTableStatement(t *testing.T, file, sql, table string) string {
	t.Helper()
	head := "create table if not exists " + table + " ("
	i := strings.Index(sql, head)
	if i < 0 {
		t.Fatalf("%s: no %q — the parser went blind", file, head)
	}
	j := strings.Index(sql[i:], ";")
	if j < 0 {
		t.Fatalf("%s: %s has no end of statement", file, table)
	}
	return sql[i : i+j+1]
}

// requirePlainTable — ADR 0096 #3: a catalogue, configuration, permission, setting or counter table
// is plain. Both halves are checked: no PARTITION BY on its CREATE, and no leaf created for it.
func requirePlainTable(t *testing.T, file, sql, table string) {
	t.Helper()
	if strings.Contains(createTableStatement(t, file, sql, table), "partition by") {
		t.Errorf("%s: %s is declared PARTITION BY — ADR 0096 makes it a plain table", file, table)
	}
	if strings.Contains(sql, "partition of "+table+" ") {
		t.Errorf("%s: a partition of %s is created — ADR 0096 makes it a plain table", file, table)
	}
}

// requireHashPartitionedBy8 — the parent is HASH (tenant_id) and the SAME file creates the eight
// leaves (ADR 0013 #4). The loop bound is pinned with the modulus: 0..31 against MODULUS 8 fails at
// apply time (remainder >= modulus), and a shorter loop leaves remainders whose every INSERT is refused.
func requireHashPartitionedBy8(t *testing.T, file, sql, table string) {
	t.Helper()
	if !strings.HasSuffix(createTableStatement(t, file, sql, table), ") partition by hash (tenant_id);") {
		t.Errorf("%s: %s is not PARTITION BY HASH (tenant_id) — ADR 0096 keeps it partitioned", file, table)
	}
	loop := "for i in 0..7 loop execute format( 'create table if not exists " + table + "_p%s partition of " + table +
		" ' 'for values with (modulus 8, remainder %s)', lpad(i::text, 2, '0'), i); end loop;"
	if !strings.Contains(sql, loop) {
		t.Errorf("%s: %s has no 0..7 / MODULUS 8 leaf loop (ADR 0096)", file, table)
	}
}

var partitionLeafLoop = regexp.MustCompile(`partition of (\w+) '`)

// THE MUTATIONS THAT MUST TURN THIS RED: a catalogue table partitioned again "to be safe" (ADR 0096
// Hệ quả); a growing table made plain; a 32-way loop surviving anywhere; a PARTITION BY parent whose
// file creates no leaves (it refuses every INSERT); one table's leaves created in two files.
func TestOnlyGrowingTablesArePartitioned(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	where := map[string]string{}
	files := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		files++
		sql := executableSQL(t, name)
		if strings.Contains(sql, "modulus 32") || strings.Contains(sql, "0..31") {
			t.Errorf("%s: still carries a 32-way partition loop — ADR 0096 is MODULUS 8", name)
		}
		leaves := partitionLeafLoop.FindAllStringSubmatch(sql, -1)
		if parents := strings.Count(sql, "partition by hash (tenant_id)"); parents != len(leaves) {
			t.Errorf("%s: %d PARTITION BY HASH parents, %d leaf loops", name, parents, len(leaves))
		}
		for _, m := range leaves {
			if prev, dup := where[m[1]]; dup {
				t.Errorf("%s: leaves of %s are created in %s and %s", name, m[1], prev, name)
			}
			where[m[1]] = name
		}
	}
	if files == 0 {
		t.Fatal("no .sql file in the embedded FS — the test went blind")
	}
	got := make([]string, 0, len(where))
	for table := range where {
		got = append(got, table)
	}
	sort.Strings(got)
	want := append([]string(nil), hashPartitionedTables...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("partitioned tables = %v\nADR 0096 says        %v", got, want)
	}
	for _, table := range want {
		if file, ok := where[table]; ok {
			requireHashPartitionedBy8(t, file, executableSQL(t, file), table)
		}
	}
}
