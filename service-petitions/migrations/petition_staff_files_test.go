package migrations

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/core/storage"
)

// Schema-TEXT checks for migration 0027 (staff verification photos and petition log attachments).
//
// Same standing as petition_scene_photo_test.go: it reads the SQL this binary embeds and asserts the
// constraints and triggers are WRITTEN. It does not prove PostgreSQL enforces any of it — that needs
// VIGOV_TEST_DSN (tools/schema-smoke).

const file0027 = "0027_petition_staff_files.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: either purpose allowed off a petition, outside the private
// bucket or outside the records class; the count trigger removed, its commune / soft-delete / stored-set
// filter dropped, the petition lock dropped, or the limit of 5 changed; the link table losing tenant_id
// from its key, its entry-in-this-transaction check, its purpose / subject / author check, or its
// append-only guard.
func TestMigration0027ConstraintsAndTriggers(t *testing.T) {
	sql := maChay(t, file0027)
	for _, c := range []struct{ want, why string }{
		{"constraint stored_file_petition_verification_photo_shape check (purpose <> 'petition-verification-photo' or (subject_type = 'petition' and retention_class = 'records' and bucket = 'private'))",
			"a verification photo is a private record on a petition — never public"},
		{"constraint stored_file_petition_log_attachment_shape check (purpose <> 'petition-log-attachment' or (subject_type = 'petition' and retention_class = 'records' and bucket = 'private'))",
			"a log attachment is a private record on a petition"},
		{"before insert or update of status on stored_file for each row execute function stored_file_verification_photo_check()",
			"the count runs on insert and on entering the stored set"},
		{"where p.tenant_id = new.tenant_id and p.id = new.subject_id and p.deleted_at is null for update",
			"petition in this commune, live, locked to serialise the count"},
		{"where f.tenant_id = new.tenant_id and f.subject_type = 'petition' and f.subject_id = new.subject_id and f.purpose = 'petition-verification-photo' and f.deleted_at is null and f.status in ('stored', 'processing', 'ready') and f.id <> new.id",
			"the count is per commune, per petition, verification photos only, live only, excluding the row itself"},
		{"if live_photos >= 5 then raise exception", "at most 5 (set by precedent 02/10/2026)"},
		{"primary key (tenant_id, stored_file_id)", "a file belongs to one entry, per commune (rule 1 invariant 6)"},
		{"on petition_log_attachment (tenant_id, log_entry_id, attached_at)", "index leads with tenant_id"},
		{"from nhat_ky_phan_anh l where l.tenant_id = new.tenant_id and l.id = new.log_entry_id",
			"the entry is looked up in this commune"},
		{"if entry_created_at <> now() then raise exception", "attached with its entry, never later"},
		{"where f.tenant_id = new.tenant_id and f.id = new.stored_file_id for share", "file in this commune, locked"},
		{"or file_row.subject_type <> 'petition' or file_row.subject_id <> entry_petition_id or file_row.uploaded_by <> entry_author or file_row.purpose <> 'petition-log-attachment' then raise exception",
			"same petition, same author, log-attachment purpose only"},
		{"if file_row.status not in ('stored', 'ready') then raise exception", "only a file that reached the destination"},
		{"before insert on petition_log_attachment for each row execute function petition_log_attachment_check()", "the link floor"},
		{"before update or delete on petition_log_attachment for each row execute function petition_log_attachment_append_only()", "append-only"},
		{"before truncate on %s ' 'for each statement execute function petition_log_attachment_append_only()", "truncate refused leaf by leaf"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0027 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"update stored_file", "writes rows — this file is schema only"},
		{"insert into", "writes rows — this file is schema only"},
		{"drop trigger if exists stored_file_guard", "removes 0021's guard"},
		{"drop trigger if exists stored_file_petition_check", "removes 0026's citizen floor"},
		{"stored_file_citizen_upload_shape", "0026's citizen shape is not this file's to change"},
		{"create or replace function stored_file_petition_check", "0026's function is not this file's to change"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0027 contains %q — %s", c.banned, c.why)
		}
	}
	raw := strings.ReplaceAll(string(mustRead(t, file0027)), "\r\n", "\n")
	if !strings.Contains(raw, "-- @entity: PetitionLogAttachment\n-- @scope:  tenant") {
		t.Error("0027 must declare the new table's ownership mark (tools/kb/ownership.go)")
	}
}

// The purpose literals in SQL are core/storage's. Mutation: rename a constant without migrating — every
// upload of that purpose would then pass the shape CHECKs vacuously and escape the count.
func TestMigration0027PurposesAreStorages(t *testing.T) {
	sql := maChay(t, file0027)
	for _, p := range []storage.Purpose{storage.PurposePetitionVerificationPhoto, storage.PurposePetitionLogAttachment} {
		if !strings.Contains(sql, "'"+string(p)+"'") {
			t.Errorf("0027 does not name core/storage purpose %q", p)
		}
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}
