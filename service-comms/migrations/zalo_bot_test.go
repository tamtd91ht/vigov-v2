package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0018 (Zalo Bot, ADR 0074). They assert the keys, CHECKs and guards
// are WRITTEN; they do not prove PostgreSQL enforces them — tools/schema-smoke and the pg suites need
// VIGOV_TEST_DSN for that.

const file0018 = "0018_zalo_bot.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a tenant table without its MODULUS 32 loop; a unique key on a
// tenant table without tenant_id (other than the declared cross-tenant chat key); the platform DEK
// becoming a row under a fake tenant_id; a guard that stops refusing DELETE; a seed row; an enum value
// translated to English (ADR 0011); the chat key losing its live-only predicate or gaining tenant_id.
func TestMigration0018ZaloBotKeysAndPartitions(t *testing.T) {
	sql := executableSQL(t, file0018)
	for _, c := range []struct{ want, why string }{
		{"partition of zalo_channel_setting ' 'for values with (modulus 32, remainder %s)", "settings partition loop"},
		{"partition of zalo_pairing_code ' 'for values with (modulus 32, remainder %s)", "pairing code partition loop"},
		{"partition of zalo_delivery ' 'for values with (modulus 32, remainder %s)", "delivery partition loop"},
		{"primary key (tenant_id), constraint zalo_channel_setting_kinds_known", "one settings row per commune"},
		{"primary key (tenant_id, id), unique (tenant_id, code_hash),", "a hash never reissued within a commune"},
		{"on zalo_pairing_code (tenant_id, staff_code) where used_at is null and cancelled_at is null", "one open code per staff"},
		{"on zalo_link (tenant_id, staff_code) where unlinked_at is null", "one live link per staff, per commune"},
		{"on zalo_link (bot_ref, chat_id) where unlinked_at is null", "one live link per chat, platform-wide (ADR 0074)"},
		{"primary key (tenant_id, id), unique (tenant_id, notification_id),", "one Zalo message per bell notice"},
		{"foreign key (tenant_id, notification_id) references staff_notification (tenant_id, id)", "repeats a real notice"},
		{"foreign key (tenant_id, link_id) references zalo_link (tenant_id, id)", "sent through a real link"},
		{"check (kinds <@ array['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan']::text[])", "staff_notification's kind values (ADR 0011)"},
		{"check (status in ('cho-gui', 'da-gui', 'bo-qua', 'that-bai'))", "delivery states, 0004's words"},
		{"check ((status = 'bo-qua') = (skip_reason is not null))", "every skip has a reason (ADR 0074)"},
		{"check ((status = 'da-gui') = (sent_at is not null))", "sent has a time"},
		{"check (not ('qua-han' = any (kinds)) or overdue_start_after_days is not null)", "overdue cadence set by the commune"},
		{"scope text primary key,", "platform DEK keyed by scope, not tenant_id"},
		{"check (scope in ('platform'))", "platform DEK single row"},
		{"check (octet_length(wrapped) = 65)", "version-1 wrapped DEK"},
		{"bot_ref text primary key, token_sealed bytea not null,", "shared bot: sealed token"},
		{"check (bot_ref in ('shared'))", "one shared bot row"},
		{"bot_account_id text not null,", "getMe's id, stored at save (owner 05/10/2026)"},
		{"webhook_secret_pending_sealed bytea, webhook_secret_pending_at timestamptz,", "pending-then-promote webhook secret"},
		{"'thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang'", "ZaloBotCallOutcome, ADR 0011 spelling"},
		{"after update on zalo_bot_shared deferrable initially deferred for each row execute function zalo_bot_shared_account_change_check()", "a new bot account must end every live link by commit"},
		{"check (actor_code ~ '^vh-[0-9]{5,}$')", "operator trail names an operator"},
		{"before update or delete on platform_operator_audit_log for each row execute function platform_operator_audit_log_append_only()", "operator trail append-only"},
		{"before truncate on platform_operator_audit_log for each statement execute function platform_operator_audit_log_append_only()", "operator trail not truncated"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0018 lacks %q — %s", c.want, c.why)
		}
	}

	// zalo_link is deliberately NOT partitioned: its platform-wide chat key cannot contain tenant_id.
	if strings.Contains(sql, "partition of zalo_link ") {
		t.Error("zalo_link is partitioned — the cross-tenant chat uniqueness cannot exist on a partitioned table")
	}
	// The platform DEK must never be a tenant row.
	if strings.Contains(sql, "insert into") {
		t.Error("0018 writes a row — no seed, no fake commune, no default bot")
	}

	for _, fn := range []string{"zalo_channel_setting_guard", "zalo_pairing_code_guard", "zalo_link_guard",
		"zalo_delivery_guard", "platform_data_encryption_key_guard", "zalo_bot_shared_guard"} {
		body := functionBody(t, sql, fn)
		if !strings.Contains(body, "if tg_op = 'delete' then raise exception") {
			t.Errorf("%s no longer refuses DELETE", fn)
		}
		table := strings.TrimSuffix(fn, "_guard")
		if !strings.Contains(sql, "before update or delete on "+table+" for each row execute function "+fn+"()") {
			t.Errorf("%s is not attached to %s", fn, table)
		}
	}

	for _, c := range []struct{ want, fn, why string }{
		{"if old.used_at is not null or old.cancelled_at is not null then raise exception", "zalo_pairing_code_guard", "a spent code is frozen"},
		{"if new.failed_attempts < old.failed_attempts then raise exception", "zalo_pairing_code_guard", "no fresh guesses"},
		{"if old.unlinked_at is not null then raise exception", "zalo_link_guard", "an ended link is history"},
		{"or new.chat_id is distinct from old.chat_id", "zalo_link_guard", "a link is never re-pointed"},
		{"if old.status <> 'cho-gui'", "zalo_delivery_guard", "a finished delivery keeps its outcome"},
		{"if (new.token_sealed is distinct from old.token_sealed or new.bot_account_id is distinct from old.bot_account_id) and new.set_at is not distinct from old.set_at", "zalo_bot_shared_guard", "a new token or bot account has a new who-and-when"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), c.want) {
			t.Errorf("%s lacks %q — %s", c.fn, c.want, c.why)
		}
	}

	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"'due_soon'", "enum values stay Vietnamese (ADR 0011)"},
		{"'pending'", "enum values stay Vietnamese (ADR 0011)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0018 contains %q — %s", c.banned, c.why)
		}
	}
}
