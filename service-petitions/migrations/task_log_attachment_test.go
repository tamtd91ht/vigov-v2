package migrations

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Schema-TEXT checks for migration 0021 (`stored_file`, `task_log_attachment`).
//
// Same standing as task_issued_code_test.go: it reads the SQL this binary embeds and asserts the
// keys, the CHECKs and the triggers are WRITTEN, and that the status list and the transition table
// agree with internal/domain. It does not prove PostgreSQL enforces any of it —
// internal/store/stored_file_pg_test.go does, and SKIPS without VIGOV_TEST_DSN.

const tep0021 = "0021_task_log_attachment.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a key or index without tenant_id first; a missing partition
// loop; removing the guard, the link check, the append-only trigger or the per-leaf TRUNCATE guard;
// a unique key on object_key alone.
func TestMigration0021KeysPartitionsTriggers(t *testing.T) {
	sql := maChay(t, tep0021)
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, id), unique (tenant_id, object_key),", "stored_file keys must be composite with tenant_id (rule 1 inv 6)"},
		{"primary key (tenant_id, stored_file_id) ) partition by hash (tenant_id);", "one file, one entry, per commune"},
		{"partition of stored_file ' 'for values with (modulus 32, remainder %s)", "stored_file partition loop"},
		{"partition of task_log_attachment ' 'for values with (modulus 32, remainder %s)", "link partition loop"},
		{"on stored_file (tenant_id, subject_type, subject_id, uploaded_by, created_at desc) where deleted_at is null",
			"the 'my files for this task' index must start with tenant_id and skip soft-deleted rows"},
		{"on task_log_attachment (tenant_id, log_entry_id, attached_at)", "batched read index must start with tenant_id"},
		{"before update or delete on stored_file for each row execute function stored_file_guard()", "metadata guard"},
		{"before insert on task_log_attachment for each row execute function task_log_attachment_check()", "link floor"},
		{"before update or delete on task_log_attachment for each row execute function task_log_attachment_append_only()", "link is append-only"},
		{"before truncate on %s ' 'for each statement execute function task_log_attachment_append_only()", "per-leaf TRUNCATE guard"},
		{"if entry_created_at <> now() then raise exception", "a link is written in the entry's transaction"},
		{"or file_row.subject_id <> entry_task_id or file_row.uploaded_by <> entry_author", "file bound to the entry's task and author"},
		{"starts_with(object_key, retention_class || '/t_' || lower(tenant_id) || '/')", "the key names this commune"},
		{"if new.legal_hold then raise exception", "no purge under legal hold"},
		{"if new.retention_class = 'records' and old.status in ('stored', 'processing', 'ready') then raise exception", "a records file is never purged"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0021 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"alter table", "alters an existing table — this file only adds"},
		{"delete from", "deletes rows"},
		{"unique (object_key)", "single-column unique key"},
		{"dinh_kem", "writes the 0006 JSONB column — the link table is the one source"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0021 contains %q — %s", c.banned, c.why)
		}
	}
}

func TestMigration0021StatusListMatchesDomain(t *testing.T) {
	got := danhSachMaTrongCheck(t, maChay(t, tep0021), "stored_file_status_known", "status")
	var want []string
	for _, s := range domain.StoredFileStatuses() {
		want = append(want, string(s))
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("status CHECK %v, domain %v", got, want)
	}
}

// THE TRANSITION TABLE HAS TWO COPIES — the trigger (floor for every writer) and
// domain.StoredFileStatus.CanMoveTo (the sentence the app answers with) — and they must be the SAME
// SET, in both directions. Mutation: add or drop one pair on either side.
func TestMigration0021EdgesMatchDomain(t *testing.T) {
	sql := maChay(t, tep0021)
	const head = "(old.status, new.status) not in ("
	i := strings.Index(sql, head)
	if i < 0 {
		t.Fatal("transition list not found in stored_file_guard")
	}
	j := strings.Index(sql[i:], ") then raise exception")
	if j < 0 {
		t.Fatal("end of transition list not found")
	}
	var got []string
	for _, m := range regexp.MustCompile(`\('([a-z]+)', '([a-z]+)'\)`).FindAllStringSubmatch(sql[i+len(head):i+j], -1) {
		got = append(got, m[1]+"->"+m[2])
	}
	var want []string
	for from, tos := range domain.StoredFileEdges() {
		for _, to := range tos {
			want = append(want, string(from)+"->"+string(to))
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) == 0 || !reflect.DeepEqual(got, want) {
		t.Errorf("trigger edges %v\ndomain edges  %v", got, want)
	}
}

func TestMigration0021AttachableMatchesDomain(t *testing.T) {
	sql := maChay(t, tep0021)
	if !strings.Contains(sql, "if file_row.status not in ('stored', 'ready') then") {
		t.Fatal("link trigger's attachable list changed")
	}
	for _, s := range domain.StoredFileStatuses() {
		want := s == domain.StoredFileStored || s == domain.StoredFileReady
		if s.Attachable() != want {
			t.Errorf("domain Attachable(%s) = %v, trigger says %v", s, s.Attachable(), want)
		}
	}
}

// core/storage's closed Class list, written out because this module does not import core/storage
// (it would pull the MinIO client into go.sum for a test). A class added there without widening the
// CHECK is refused at the first insert, loudly — the direction that fails safe.
func TestMigration0021RetentionClassesAreCoreStorages(t *testing.T) {
	got := danhSachMaTrongCheck(t, maChay(t, tep0021), "stored_file_retention_class_known", "retention_class")
	want := []string{"citizen-media", "content-source", "public-media", "records"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("retention_class CHECK %v, core/storage Class %v", got, want)
	}
}
