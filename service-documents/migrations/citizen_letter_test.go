package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0006 (the citizen-letter register, ADR 0039 / ADR 0078).
//
// WHAT THIS IS AND IS NOT. It reads the SQL this binary embeds and asserts the keys, the closed lists,
// the bindings and the triggers are WRITTEN. It does NOT prove PostgreSQL applies or enforces them —
// internal/store/citizen_letter_schema_pg_test.go does that, and it SKIPS without VIGOV_TEST_DSN. A pg
// suite that skips on most machines turns green whatever the SQL says, so a widened CHECK or a removed
// trigger has to go red somewhere that always runs: here. This is the weaker half, said as such.

const (
	file0004 = "0004_so_van_ban.sql"
	file0006 = "0006_citizen_letter.sql"
)

// executableSQL returns the executable part of a migration: `--` comments removed, whitespace
// collapsed, lower-cased. Comments go FIRST because 0006's header names the rejected designs (a
// trigram index, an overdue column) in prose — a test matching comments would read the explanation
// as DDL.
func executableSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("read %s from the embedded FS: %v", name, err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))
}

// checkCodes returns the quoted codes of `CONSTRAINT <name> CHECK (<col> IN (...))`, sorted. It FAILS
// when the constraint is not found: a parser that went blind must not read as "both lists are empty,
// therefore equal".
func checkCodes(t *testing.T, sql, name, col string) []string {
	t.Helper()
	m := regexp.MustCompile(`constraint ` + regexp.QuoteMeta(name) + ` check \(` +
		regexp.QuoteMeta(col) + ` in \(([^)]*)\)\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("constraint %s CHECK (%s IN (...)) not found", name, col)
	}
	var codes []string
	for _, x := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(m[1], -1) {
		codes = append(codes, x[1])
	}
	sort.Strings(codes)
	return codes
}

// The C3 set (TT 05/2021), sorted. Written out here so that a change to it is a change to TWO files,
// which is the point: the status list of an archival register is not edited in passing.
const wantStatuses = "chuyen-don,da-giai-quyet,dang-giai-quyet,dang-xu-ly-don,dinh-chi," +
	"huong-dan,khong-thu-ly,luu-don,moi-vao-so,thu-ly"

// TestMigration0006StatusListsAgree — the register's status CHECK and the log's two status CHECKs are
// the SAME ten codes. A log row holding a status the register refuses contradicts the row it hangs
// off; a register status the log refuses makes that act unloggable, and the log row shares the
// business transaction, so the whole act rolls back.
//
// THE MUTATION THAT MUST TURN THIS RED: add, drop or misspell one code in any of the three lists.
func TestMigration0006StatusListsAgree(t *testing.T) {
	sql := executableSQL(t, file0006)
	register := strings.Join(checkCodes(t, sql, "citizen_letter_status_valid", "status"), ",")
	from := strings.Join(checkCodes(t, sql, "citizen_letter_log_from_status_valid", "from_status"), ",")
	to := strings.Join(checkCodes(t, sql, "citizen_letter_log_to_status_valid", "to_status"), ",")

	if register != wantStatuses {
		t.Errorf("citizen_letter statuses = %s, want the C3 set %s", register, wantStatuses)
	}
	if from != register || to != register {
		t.Errorf("log status lists differ from the register:\n  register: %s\n  from:     %s\n  to:       %s",
			register, from, to)
	}
}

// TestMigration0006ClosedLists — four letter types (C4, `phan-anh` is NOT one), five log kinds.
func TestMigration0006ClosedLists(t *testing.T) {
	sql := executableSQL(t, file0006)
	if got := strings.Join(checkCodes(t, sql, "citizen_letter_type_valid", "letter_type"), ","); got !=
		"de-nghi,khieu-nai,kien-nghi-phan-anh,to-cao" {
		t.Errorf("letter_type = %s, want the four types of C4", got)
	}
	if got := strings.Join(checkCodes(t, sql, "citizen_letter_log_kind_valid", "kind"), ","); got !=
		"chuyen-trang-thai,ghi-chu,ket-qua,luan-chuyen,sua-nguoi-gui" {
		t.Errorf("log kind = %s", got)
	}
}

// TestMigration0006SeriesIsThirdKindOnTheSameCounter — the letter number comes from 0004's counter
// with a third `so_sach` value, and the widened list is a SUPERSET of 0004's: dropping 'den' or 'di'
// would make every document booking fail its counter insert.
func TestMigration0006SeriesIsThirdKindOnTheSameCounter(t *testing.T) {
	old := checkCodes(t, executableSQL(t, file0004), "day_so_van_ban_so_sach_hop_le", "so_sach")
	now := checkCodes(t, executableSQL(t, file0006), "day_so_van_ban_so_sach_hop_le", "so_sach")
	if strings.Join(now, ",") != "den,di,don-thu" {
		t.Errorf("0006 series kinds = %v, want [den di don-thu]", now)
	}
	have := map[string]bool{}
	for _, c := range now {
		have[c] = true
	}
	for _, c := range old {
		if !have[c] {
			t.Errorf("0006 drops series kind %q that 0004 allows — existing bookings would fail", c)
		}
	}
}

// TestMigration0006KeysConstraintsTriggers — composite keys, hash partitioning, the bindings and the
// guards.
//
// THE MUTATIONS THAT MUST TURN THIS RED: a partial or single-column number key; dropping the ELSE arm
// of the acceptance binding; dropping the partition loop; dropping any trigger; letting a note row
// carry a status change.
func TestMigration0006KeysConstraintsTriggers(t *testing.T) {
	sql := executableSQL(t, file0006)
	for _, c := range []struct{ want, why string }{
		{"create table if not exists citizen_letter (", "the register table"},
		{"create table if not exists citizen_letter_log (", "the log table"},
		{"primary key (tenant_id, id), unique (tenant_id, year, number), constraint",
			"number key composite with tenant_id and NOT partial (rule 1 inv 6, rule 7 inv 3)"},
		{"partition of citizen_letter ' 'for values with (modulus 32, remainder %s)", "32 partitions of the register"},
		{"partition of citizen_letter_log ' 'for values with (modulus 32, remainder %s)", "32 partitions of the log"},
		{"sender_name text, sender_phone text, sender_address text,", "sender fields all nullable (C7)"},
		{"summary text not null,", "summary required"},
		{"processing_due_at timestamptz, resolution_due_at timestamptz,", "both deadlines nullable this run (C8)"},
		{"then accepted_at is not null else accepted_at is null end", "accepted_at bound to the thu-ly branch both ways"},
		{"then resolved_at is not null and accepted_at is not null and resolved_at >= accepted_at else resolved_at is null end",
			"resolved_at bound to the two resolution ends"},
		{"check (resolution_due_at is null or accepted_at is not null)", "resolution deadline only after thu-ly"},
		{"foreign key (tenant_id, related_letter_id) references citizen_letter (tenant_id, id)",
			"duplicate link is same-commune (C11, rule 1)"},
		{"check ((result_document_no is null) = (result_document_date is null))", "result citation complete"},
		{"status <> 'da-giai-quyet' or (result_document_no is not null", "da-giai-quyet closes with its result (C10)"},
		{"before delete on citizen_letter for each row execute function ho_so_luu_tru_cam_xoa_cung()",
			"hard delete refused (rule 7)"},
		{"before update on citizen_letter for each row execute function citizen_letter_number_immutable()",
			"number/year immutable"},
		{"if new.year is distinct from old.year or new.number is distinct from old.number then",
			"the immutability trigger checks number and year"},
		{"before update or delete on citizen_letter_log for each row execute function citizen_letter_log_append_only()",
			"log append-only on UPDATE/DELETE"},
		{"before truncate on %s ' 'for each statement execute function citizen_letter_log_append_only()",
			"log TRUNCATE guarded per partition"},
		{"when 'ghi-chu' then to_status is null when 'sua-nguoi-gui' then to_status is null",
			"a note or sender correction carries no status change"},
		{"(from_status is null and to_status is null) or (from_status is not null and to_status is not null and from_status <> to_status)",
			"status pair together and a real change"},
		{"else from_unit_id is null and to_unit_id is null and assignee_code is null end",
			"routing columns only on luan-chuyen"},
		{"on citizen_letter_log (tenant_id, letter_id, at desc, id desc)", "log timeline index"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0006 is missing %q — %s", c.want, c.why)
		}
	}
}

// TestMigration0006IndexesStartWithTenantAndKeepPhoneOut — every index starts with tenant_id (rule 1),
// and the phone number and address are in NO index (rule 3: the duplicate check needs the name only).
func TestMigration0006IndexesStartWithTenantAndKeepPhoneOut(t *testing.T) {
	sql := executableSQL(t, file0006)
	idx := regexp.MustCompile(`create index if not exists (\w+) on (\w+) \(([^;]*);`).FindAllStringSubmatch(sql, -1)
	if len(idx) != 6 {
		t.Fatalf("found %d indexes, want 6 — the parser went blind or an index was added/removed", len(idx))
	}
	for _, m := range idx {
		if !strings.HasPrefix(m[3], "tenant_id,") {
			t.Errorf("index %s does not start with tenant_id", m[1])
		}
		if strings.Contains(m[3], "sender_phone") || strings.Contains(m[3], "sender_address") {
			t.Errorf("index %s holds the sender's phone or address (rule 3)", m[1])
		}
		if m[2] == "citizen_letter" && !strings.Contains(m[3], "where deleted_at is null") {
			t.Errorf("register index %s keeps soft-deleted rows (rule 7 inv 2)", m[1])
		}
	}
	if !strings.Contains(sql, "on citizen_letter (tenant_id, lower(sender_name), received_date) "+
		"where deleted_at is null and sender_name is not null;") {
		t.Error("duplicate-check index is not the documented expression — B2's query depends on it byte for byte")
	}
}

// TestMigration0006DestroysNothing — the file only adds. The single change to an existing object is
// the widened CHECK on `day_so_van_ban`; nothing else of 0001–0005 is altered, and the log has no
// soft-delete columns (a row that could be hidden is a log that can be made to say something else).
func TestMigration0006DestroysNothing(t *testing.T) {
	sql := executableSQL(t, file0006)
	for _, banned := range []string{"drop table", "drop column", "alter column", "delete from",
		"truncate table", "insert into", "update day_so_van_ban", "update citizen_letter",
		"create extension", "replace function ho_so_luu_tru_cam_xoa_cung", "replace function so_van_ban_bat_bien",
		"replace function day_so_khong_lui", "trigger day_so_van_ban"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0006 contains %q", banned)
		}
	}
	if n := strings.Count(sql, "alter table day_so_van_ban"); n != 2 {
		t.Errorf("0006 alters day_so_van_ban %d times, want exactly the DROP+ADD of its so_sach CHECK", n)
	}
	logDDL := sql[strings.Index(sql, "create table if not exists citizen_letter_log ("):]
	logDDL = logDDL[:strings.Index(logDDL, ") partition by hash (tenant_id);")]
	if strings.Contains(logDDL, "deleted_at") {
		t.Error("citizen_letter_log has a soft-delete column — the log is append-only")
	}
}

const file0008 = "0008_citizen_letter_source_and_result_by_type.sql"

// TestMigration0008SourceList — the four entry sources of ADR 0084 #7, and existing rows read as
// manual entry (owner, 08/10/2026). NOT NULL with that DEFAULT is what makes "đơn cũ gán Nhập tay"
// true without a backfill.
//
// THE MUTATIONS THAT MUST TURN THIS RED: add, drop or misspell a source; drop NOT NULL or change the
// default.
func TestMigration0008SourceList(t *testing.T) {
	sql := executableSQL(t, file0008)
	if got := strings.Join(checkCodes(t, sql, "citizen_letter_source_valid", "source"), ","); got !=
		"mini-app,nhap-excel,nhap-tay,thu-dien-tu" {
		t.Errorf("source = %s, want the four sources of ADR 0084 #7", got)
	}
	if !strings.Contains(sql, "add column if not exists source text not null default 'nhap-tay';") {
		t.Error("source is not NOT NULL DEFAULT 'nhap-tay' — existing letters would not read as Nhập tay")
	}
}

// TestMigration0008ResultRuleNarrowedToComplaintAndDenunciation — ADR 0084 #2: `kien-nghi-phan-anh`
// and `de-nghi` close without a result; `khieu-nai` and `to-cao` still need the five fields. The
// constraint keeps 0006's name, and is replaced, not left beside the old one (two CHECKs with the same
// intent would make the stricter one win silently).
//
// THE MUTATIONS THAT MUST TURN THIS RED: exempt `khieu-nai` or `to-cao`; drop any of the five fields
// from the required arm; add the new rule without dropping the old one.
func TestMigration0008ResultRuleNarrowedToComplaintAndDenunciation(t *testing.T) {
	sql := executableSQL(t, file0008)
	want := "alter table citizen_letter drop constraint if exists citizen_letter_resolved_has_result; " +
		"alter table citizen_letter add constraint citizen_letter_resolved_has_result check ( " +
		"status <> 'da-giai-quyet' or letter_type in ('kien-nghi-phan-anh', 'de-nghi') " +
		"or (result_document_no is not null and result_document_date is not null " +
		"and result_signer is not null and result_issuer is not null and result_summary is not null));"
	if !strings.Contains(sql, want) {
		t.Errorf("0008 does not replace citizen_letter_resolved_has_result with the KN/TC rule:\n  want %q", want)
	}
	// The exempt list must be a subset of 0006's types and must not hold the two statutory ones.
	m := regexp.MustCompile(`or letter_type in \(([^)]*)\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("exempt letter_type list not found — the parser went blind")
	}
	for _, statutory := range []string{"'khieu-nai'", "'to-cao'"} {
		if strings.Contains(m[1], statutory) {
			t.Errorf("%s is exempted from the result document — ADR 0084 #2 keeps it required", statutory)
		}
	}
}

// TestMigration0008DestroysNothing — the file adds a column and swaps two CHECKs; it rewrites no row
// and touches nothing else of 0006.
func TestMigration0008DestroysNothing(t *testing.T) {
	sql := executableSQL(t, file0008)
	for _, banned := range []string{"drop table", "drop column", "alter column", "delete from",
		"truncate", "insert into", "update ", "not valid", "drop trigger", "replace function"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0008 contains %q", banned)
		}
	}
	dropped := regexp.MustCompile(`drop constraint if exists (\w+);`).FindAllStringSubmatch(sql, -1)
	if len(dropped) != 2 {
		t.Fatalf("0008 drops %d constraints, want exactly 2", len(dropped))
	}
	for _, d := range dropped {
		if !strings.Contains(sql, "add constraint "+d[1]+" check (") {
			t.Errorf("0008 drops %s without adding it back in the same file", d[1])
		}
	}
}
