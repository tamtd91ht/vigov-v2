package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0027 (`citizen_notification_preferences` and the fifth status of
// `thong_bao_gui_cong_dan`). The pg suites SKIP without VIGOV_TEST_DSN.

const file0027 = "0027_citizen_notification_preferences.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the key without tenant_id; a DEFAULT on zalo_enabled (a write
// that forgets the value would silently decide for the citizen); the guard no longer refusing DELETE
// (absence means "enabled", so a delete re-enables an opt-out); one of 0004's four statuses dropped from
// the widened CHECK (ledger rows are immutable); the old CHECK dropped before the new one exists; NOT
// VALID on the partitioned ledger; a row written.
func TestMigration0027CitizenNotificationPreferences(t *testing.T) {
	sql := executableSQL(t, file0027)
	requirePlainTable(t, file0027, sql, "citizen_notification_preferences") // ADR 0096: one row per citizen per commune, plain
	for _, c := range []struct{ want, why string }{
		{"create table if not exists citizen_notification_preferences ( tenant_id text not null, citizen_id text not null, zalo_enabled boolean not null,", "the agreed columns, no default on the switch"},
		{"primary key (tenant_id, citizen_id),", "one preference per citizen per commune, composite with tenant_id (rule 1 inv 6)"},
		{"before update or delete on citizen_notification_preferences for each row execute function citizen_notification_preferences_guard()", "guard on update and delete"},
		{"check (trang_thai in ('cho-gui', 'da-gui', 'that-bai', 'chua-cau-hinh-kenh', 'dan-da-tat-nhan'))", "0004's four values kept, the opt-out added"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0027 lacks %q — %s", c.want, c.why)
		}
	}

	guard := functionBody(t, sql, "citizen_notification_preferences_guard")
	for _, want := range []string{
		"if tg_op = 'delete' then raise exception",
		"or new.citizen_id is distinct from old.citizen_id",
	} {
		if !strings.Contains(guard, want) {
			t.Errorf("citizen_notification_preferences_guard lacks %q", want)
		}
	}

	add := strings.Index(sql, "add constraint thong_bao_gui_cong_dan_trang_thai_with_opt_out ")
	drop := strings.Index(sql, "drop constraint if exists thong_bao_gui_cong_dan_trang_thai_hop_le;")
	if add < 0 || drop < 0 || drop < add {
		t.Errorf("0027 must add the widened status CHECK before dropping 0004's (add=%d drop=%d)", add, drop)
	}
	if n := strings.Count(sql, "add constraint"); n != 1 {
		t.Errorf("0027 adds %d constraints, want 1", n)
	}
	for _, c := range []struct{ banned, why string }{
		{"zalo_enabled boolean not null default", "the switch has no default — the citizen's act states it"},
		{"not valid", "NOT VALID on a partitioned table (0011:332-341)"},
		{"phone", "no phone number beside the opaque citizen id (rule 3)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0027 contains %q — %s", c.banned, c.why)
		}
	}
	noDestruction(t, file0027, sql)
}

func TestMigration0027OwnershipMarks(t *testing.T) {
	ownershipMarks(t, file0027, "CitizenNotificationPreference", "citizen_notification_preferences")
}

// Schema-TEXT checks for migration 0028 (`noi_dung_mini_app.video_duration_seconds`).
//
// THE MUTATIONS THAT MUST TURN THIS RED: the duration allowed on a non-video item; the range removed;
// the column made NOT NULL or given a default (every existing video would be rewritten or invalid); the
// file tied to video_url (pre-existing videos have a link and no duration).
func TestMigration0028VideoDuration(t *testing.T) {
	const file0028 = "0028_content_item_video_duration.sql"
	sql := executableSQL(t, file0028)
	for _, c := range []struct{ want, why string }{
		{"alter table noi_dung_mini_app add column if not exists video_duration_seconds int;", "nullable, no default"},
		{"check (loai = 'video' or video_duration_seconds is null)", "video only (0012's audio mirror)"},
		{"check (video_duration_seconds is null or video_duration_seconds between 1 and 21600)", "0012's vendor bound"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0028 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"video_url", "not valid", "create or replace function"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0028 contains %q — the file only adds one column and its two CHECKs", banned)
		}
	}
	noDestruction(t, file0028, sql)
}
