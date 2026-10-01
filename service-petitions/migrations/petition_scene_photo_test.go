package migrations

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Schema-TEXT checks for migration 0026 (citizen scene photos in `stored_file`).
//
// Same standing as task_log_attachment_test.go: it reads the SQL this binary embeds and asserts the
// constraints and the trigger are WRITTEN, and that the literals agree with internal/domain. It does
// not prove PostgreSQL enforces any of it — tools/schema-smoke applies the file to a real server and
// needs VIGOV_TEST_DSN.

const tep0026 = "0026_petition_scene_photo.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: dropping 'petition' from the subject list; letting the
// citizen marker onto a task or another purpose; a scene photo outside the private bucket or the
// citizen-media class; removing the trigger, the commune/soft-delete filter, the petition lock, the
// citizen-filed check or the limit of 5; a count that stops filtering tenant_id or deleted_at.
func TestMigration0026ConstraintsAndTrigger(t *testing.T) {
	sql := maChay(t, tep0026)
	for _, c := range []struct{ want, why string }{
		{"constraint stored_file_citizen_upload_shape check (uploaded_by <> 'cong-dan' or (subject_type = 'petition' and purpose = 'petition-photo'))",
			"a citizen uploads scene photos and nothing else"},
		{"constraint stored_file_petition_photo_shape check (purpose <> 'petition-photo' or (subject_type = 'petition' and retention_class = 'citizen-media' and bucket = 'private'))",
			"a scene photo is private citizen media on a petition — never public"},
		{"before insert or update of status on stored_file for each row execute function stored_file_petition_check()",
			"the floor runs on insert and on entering the stored set"},
		{"where p.tenant_id = new.tenant_id and p.id = new.subject_id and p.deleted_at is null for update",
			"petition in this commune, live, locked to serialise the count"},
		{"if new.uploaded_by = 'cong-dan' and (petition_citizen is null or btrim(petition_citizen) = '') then raise exception",
			"a citizen upload needs a citizen-filed petition"},
		{"where f.tenant_id = new.tenant_id and f.subject_type = 'petition' and f.subject_id = new.subject_id and f.uploaded_by = 'cong-dan' and f.purpose = 'petition-photo' and f.deleted_at is null and f.status in ('stored', 'processing', 'ready') and f.id <> new.id",
			"the count is per commune, per petition, citizen photos only, live only, excluding the row itself"},
		{"if live_photos >= 5 then raise exception", "at most 5 (owner decision 02/10/2026)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0026 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"update stored_file", "writes rows — this file is schema only"},
		{"insert into", "writes rows — this file is schema only"},
		{"drop trigger if exists stored_file_guard", "removes 0021's guard (no hard delete, frozen identity)"},
		{"create table", "a new table needs its own ownership mark"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0026 contains %q — %s", c.banned, c.why)
		}
	}
}

// The subject list after 0026 is exactly the domain's two subjects. Mutation: a third subject in the
// CHECK, or a domain constant renamed without the migration.
func TestMigration0026SubjectListMatchesDomain(t *testing.T) {
	got := danhSachMaTrongCheck(t, maChay(t, tep0026), "stored_file_subject_type_known", "subject_type")
	want := []string{domain.StoredFileSubjectPetition, domain.StoredFileSubjectTask}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("subject_type CHECK %v, domain %v", got, want)
	}
}

// The marker in SQL is the marker the app writes. Mutation: change domain.CitizenLogActor without
// migrating — every citizen upload would then fail the shape CHECK, or pass as "staff".
func TestMigration0026CitizenMarkerIsDomains(t *testing.T) {
	sql := maChay(t, tep0026)
	marker := "'" + domain.CitizenLogActor + "'"
	if n := strings.Count(sql, marker); n < 4 {
		t.Errorf("0026 names the citizen marker %s %d time(s); want it in the CHECK, the trigger check and the count", marker, n)
	}
	if strings.Count(sql, "'cong-dan'") != strings.Count(sql, marker) {
		t.Errorf("0026 uses a citizen marker other than domain.CitizenLogActor %q", domain.CitizenLogActor)
	}
}

// The stored set the floor counts is the one the store's completion re-check counts
// (store/stored_file.go countStoredForSubjectTail) and the one the domain calls attachable-or-later.
// Mutation: add 'pending' or drop 'processing' on either side.
func TestMigration0026CountedStatusesAreTheStoredSet(t *testing.T) {
	sql := maChay(t, tep0026)
	const set = "('stored', 'processing', 'ready')"
	if strings.Count(sql, set) < 4 {
		t.Errorf("0026 should test the stored set %s on entry and in the count", set)
	}
	for _, s := range []domain.StoredFileStatus{domain.StoredFileStored, domain.StoredFileReady} {
		if !strings.Contains(set, "'"+string(s)+"'") {
			t.Errorf("domain status %s missing from 0026's counted set", s)
		}
	}
}
