package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0029 (the verification-photo cap counted per processing round, owner
// decision 02/10/2026, ADR 0047 (e)).
//
// Same standing as petition_staff_files_test.go: it reads the SQL this binary embeds and asserts the
// function is WRITTEN this way. It does not prove PostgreSQL enforces it — that needs VIGOV_TEST_DSN
// (tools/schema-smoke).

const file0029 = "0029_verification_photo_cap_per_round.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the round start read from another act, from another commune or
// petition, or as min() instead of max(); the photo bound turned into `>=` (a photo issued in the same
// instant as the reopen would then count in the new round, while the close gate's `created_at > $5`
// would not accept it as closing evidence — floor and gate disagreeing); `updated_at` instead of
// `created_at`; the "never reopened → count all" arm dropped (a petition never reopened would then count
// nothing and hold unlimited photos); the petition lock dropped or moved after the reads; any of 0027's
// commune / purpose / soft-delete / stored-set filters dropped; the limit of 5 changed.
func TestMigration0029CountsPerRound(t *testing.T) {
	sql := maChay(t, file0029)
	for _, c := range []struct{ want, why string }{
		{"create or replace function stored_file_verification_photo_check() returns trigger",
			"replaces 0027's function by name; 0027's trigger keeps calling it"},
		{"select max(l.thoi_diem) into round_start from nhat_ky_phan_anh l where l.tenant_id = new.tenant_id and l.phieu_phan_anh_id = new.subject_id and l.hanh_vi = 'mo-lai-theo-danh-gia';",
			"round start = the latest reopen of this petition in this commune (store LatestReopenAtTx)"},
		{"where f.tenant_id = new.tenant_id and f.subject_type = 'petition' and f.subject_id = new.subject_id and f.purpose = 'petition-verification-photo' and f.deleted_at is null and f.status in ('stored', 'processing', 'ready') and f.id <> new.id and (round_start is null or f.created_at > round_start);",
			"0027's count narrowed to created_at STRICTLY after the round start (store countStoredCreatedAfterTail); never reopened = all"},
		{"if live_photos >= 5 then raise exception", "at most 5 per round"},
		{"if new.purpose <> 'petition-verification-photo' or new.deleted_at is not null or new.status not in ('stored', 'processing', 'ready') then return new;",
			"0027's early return, unchanged"},
		{"if tg_op = 'update' and old.status in ('stored', 'processing', 'ready') then return new;",
			"0027's re-run filter, unchanged"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0029 lacks %q — %s", c.want, c.why)
		}
	}

	// The petition row is locked BEFORE the timeline and the photos are read, so a completion and a
	// reopen on one petition are serialised and the count is taken against a settled round.
	lock := strings.Index(sql, "where p.tenant_id = new.tenant_id and p.id = new.subject_id and p.deleted_at is null for update")
	reopen := strings.Index(sql, "from nhat_ky_phan_anh l")
	count := strings.Index(sql, "select count(*) into live_photos")
	if lock < 0 || reopen < 0 || count < 0 || !(lock < reopen && reopen < count) {
		t.Errorf("0029: the petition FOR UPDATE lock must come first, then the reopen read, then the count "+
			"(lock=%d reopen=%d count=%d)", lock, reopen, count)
	}

	for _, c := range []struct{ banned, why string }{
		{"updated_at", "a photo's round is dated by created_at; updated_at moves on every transition"},
		{"danh_gia_luc", "overwritten by a later 3–5 star rating (store LatestReopenAtTx)"},
		{"drop trigger", "0027's trigger is not this file's to touch"},
		{"create trigger", "0027's trigger is not this file's to touch"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"alter table", "alters a table — this file only replaces a function body"},
		{"delete from", "deletes rows"},
		{"insert into", "writes rows — schema only"},
		{"update stored_file", "writes rows — schema only"},
		{"create or replace function stored_file_petition_check", "0026's function is not this file's"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0029 contains %q — %s", c.banned, c.why)
		}
	}
}
