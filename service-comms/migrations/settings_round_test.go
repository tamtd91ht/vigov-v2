package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migrations 0019–0022 (ADR 0079, owner answers 08/10/2026). They assert the
// columns, CHECKs, keys and triggers are WRITTEN; they do not prove PostgreSQL enforces them — the pg
// suites need VIGOV_TEST_DSN for that.

const (
	file0019 = "0019_mail_settings_last_test.sql"
	file0020 = "0020_map_asset_type_color.sql"
	file0021 = "0021_zalo_kinds_per_domain.sql"
	file0022 = "0022_zalo_commune_bot.sql"
)

// noDestruction fails when an additive migration drops a table or a column, deletes or seeds rows.
func noDestruction(t *testing.T, name, sql string) {
	t.Helper()
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"insert into", "writes a row — these files seed nothing"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("%s contains %q — %s", name, c.banned, c.why)
		}
	}
	// `update` alone also names trigger events (`before update or delete on …`); a row rewrite is
	// `update <table> set`.
	if rewrite.MatchString(sql) {
		t.Errorf("%s rewrites rows (%q) — these files change no data", name, rewrite.FindString(sql))
	}
}

var rewrite = regexp.MustCompile(`\bupdate [a-z_]+( [a-z_]+)? set\b`)

// THE MUTATIONS THAT MUST TURN THIS RED: a raw-address column or a free-text error; the class list
// drifting from internal/mail's ten sentinels; the CASE replaced by a NULL-passing comparison.
func TestMigration0019MailLastTest(t *testing.T) {
	sql := executableSQL(t, file0019)
	for _, c := range []struct{ want, why string }{
		{"add column if not exists last_test_at timestamptz;", "when"},
		{"add column if not exists last_test_to_masked text;", "to whom, masked (rule 3)"},
		{"add column if not exists last_test_ok boolean;", "whether"},
		{"add column if not exists last_test_error_class text;", "how it failed, as a class"},
		{"check (case when last_test_ok is null then last_test_error_class is null else last_test_ok = (last_test_error_class is null) end)", "a class exactly on failure"},
		{"'khong-ket-noi', 'het-thoi-gian', 'chung-chi-khong-hop-le', 'loi-tls', 'khong-co-starttls', 'tu-choi-khong-ma-hoa', 'khong-ho-tro-dang-nhap', 'sai-tai-khoan', 'tu-choi-dia-chi', 'sai-giao-thuc', 'khac'", "one class per internal/mail sentinel + khac (ADR 0011 spelling)"},
		{"last_test_to_masked ~ '^[^@[:space:]][*][*][*]@[^@[:space:]]+$'", "MaskEmail's shape; a raw address is refused"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0019 lacks %q — %s", c.want, c.why)
		}
	}
	for _, banned := range []string{"last_test_error text", "last_test_to text", "default"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0019 contains %q — no free-text error, no raw address, no vendor default", banned)
		}
	}
	noDestruction(t, file0019, sql)
}

func TestMigration0020MapAssetTypeColor(t *testing.T) {
	sql := executableSQL(t, file0020)
	for _, c := range []struct{ want, why string }{
		{"alter table loai_tai_nguyen_ban_do add column if not exists color text;", "nullable, no vendor colour"},
		{"check (color is null or color ~ '^#[0-9a-fa-f]{6}$')", "#RRGGBB only, either case (executableSQL lower-cases the text)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0020 lacks %q — %s", c.want, c.why)
		}
	}
	noDestruction(t, file0020, sql)
}

// THE MUTATIONS THAT MUST TURN THIS RED: an old kind dropped from a CHECK (delivered notices are
// immutable — their kind must stay valid); a per-domain kind missing from one of the three lists; the
// old constraint dropped before the new one exists; a settings row rewritten.
func TestMigration0021ZaloKindsPerDomain(t *testing.T) {
	sql := executableSQL(t, file0021)
	perDomain := []string{
		"'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang'",
		"'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang'",
		"'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang'",
	}
	for _, c := range []struct{ name, head string }{
		{"staff_notification_kind_per_domain", "check (kind in ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',"},
		{"zalo_channel_setting_kinds_per_domain", "check (kinds <@ array['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',"},
		{"zalo_delivery_kind_per_domain", "check (kind in ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan', 'thu-nghiem',"},
	} {
		i := strings.Index(sql, "add constraint "+c.name+" ")
		if i < 0 {
			t.Errorf("0021 does not add %s", c.name)
			continue
		}
		rest := sql[i:]
		if end := strings.Index(rest, "end if;"); end > 0 {
			rest = rest[:end]
		}
		if !strings.Contains(rest, c.head) {
			t.Errorf("%s does not keep the old four values valid", c.name)
		}
		for _, d := range perDomain {
			if !strings.Contains(rest, d) {
				t.Errorf("%s lacks %s", c.name, d)
			}
		}
	}
	if !strings.Contains(sql, "check (not (kinds && array['qua-han', 'nhiem-vu.qua-han', 'van-ban.qua-han', 'phan-anh.qua-han']::text[]) or overdue_start_after_days is not null)") {
		t.Error("0021: an overdue kind, old or per-domain, must still need the commune's cadence")
	}
	for _, old := range []string{"staff_notification_kind_known", "zalo_channel_setting_kinds_known",
		"zalo_channel_setting_overdue_needs_cadence", "zalo_delivery_kind_known"} {
		drop := strings.Index(sql, "drop constraint if exists "+old+";")
		if drop < 0 {
			t.Errorf("0021 does not drop %s", old)
			continue
		}
		if strings.LastIndex(sql, "add constraint") > drop {
			t.Errorf("0021 drops %s before every new constraint is added", old)
		}
	}
	noDestruction(t, file0021, sql)
}

// THE MUTATIONS THAT MUST TURN THIS RED: the table partitioned (its platform-wide account key cannot
// exist then); the live-per-commune key losing tenant_id; the guard no longer refusing DELETE or no
// longer freezing a retired row; the link CHECK left at 'shared' only; the retirement check made
// immediate; a seed row.
func TestMigration0022ZaloCommuneBot(t *testing.T) {
	sql := executableSQL(t, file0022)
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, id),", "composite with tenant_id (rule 1 inv 6)"},
		{"bot_ref text generated always as ('commune:' || bot_account_id) stored,", "bot_ref derived, one source"},
		{"token_sealed bytea not null,", "the token is sealed"},
		{"webhook_secret_pending_sealed bytea, webhook_secret_pending_at timestamptz,", "pending-then-promote"},
		{"retired_at timestamptz, retired_by text, retire_reason text,", "retire trio, no DELETE"},
		{"'thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang'", "0018's ZaloBotCallOutcome vocabulary"},
		{"on zalo_commune_bot (tenant_id) where retired_at is null", "one live bot per commune"},
		{"on zalo_commune_bot (bot_account_id) where retired_at is null", "one live place per bot account, platform-wide"},
		{"before update or delete on zalo_commune_bot for each row execute function zalo_commune_bot_guard()", "guard attached"},
		{"before insert on zalo_commune_bot for each row execute function zalo_bot_account_not_shared()", "not the shared bot's account"},
		{"before insert or update on zalo_bot_shared for each row execute function zalo_bot_shared_account_not_commune()", "the shared bot is not a commune's bot"},
		{"check (bot_ref = 'shared' or bot_ref ~ '^commune:[!-~]{1,128}$')", "a link may name a commune bot"},
		{"drop constraint if exists zalo_link_bot_ref_known;", "0018's 'shared'-only CHECK replaced"},
		{"before insert on zalo_link for each row execute function zalo_link_commune_bot_live()", "a link only through a live bot of the same commune"},
		{"after update on zalo_commune_bot deferrable initially deferred for each row execute function zalo_commune_bot_retire_check()", "own -> shared / own -> own ends the old bot's links by commit (ADR 0079 Q1 #4)"},
		{"after insert on zalo_commune_bot deferrable initially deferred for each row execute function zalo_commune_bot_adopt_check()", "shared -> own ends the shared links by commit (ADR 0079 Q1 #4)"},
		{"alter table zalo_link alter column bot_ref drop default;", "no writer pairs to the shared bot by omission"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0022 lacks %q — %s", c.want, c.why)
		}
	}
	if strings.Contains(sql, "partition by") || strings.Contains(sql, "partition of zalo_commune_bot") {
		t.Error("zalo_commune_bot is partitioned — its platform-wide account key cannot exist on a partitioned table")
	}
	if strings.Index(sql, "add constraint zalo_link_bot_ref_shape") > strings.Index(sql, "drop constraint if exists zalo_link_bot_ref_known") {
		t.Error("0022 drops the old bot_ref CHECK before adding the new one")
	}

	guard := functionBody(t, sql, "zalo_commune_bot_guard")
	for _, c := range []struct{ want, why string }{
		{"if tg_op = 'delete' then raise exception", "DELETE refused"},
		{"if old.retired_at is not null then raise exception", "a retired bot is frozen"},
		{"or new.bot_account_id is distinct from old.bot_account_id", "a different bot is a different row"},
		{"if new.token_sealed is distinct from old.token_sealed and new.set_at is not distinct from old.set_at", "a new token has a new who-and-when"},
	} {
		if !strings.Contains(guard, c.want) {
			t.Errorf("zalo_commune_bot_guard lacks %q — %s", c.want, c.why)
		}
	}
	const tenantLock = "perform pg_advisory_xact_lock(hashtextextended('zalo-commune-bot-tenant:' || new.tenant_id, 0));"
	live := functionBody(t, sql, "zalo_link_commune_bot_live")
	for _, c := range []struct{ want, why string }{
		{"where tenant_id = new.tenant_id and bot_ref = new.bot_ref and retired_at is null", "a commune link only through a LIVE bot of the SAME commune"},
		{"if new.bot_ref = 'shared' then if exists (select 1 from zalo_commune_bot where tenant_id = new.tenant_id and retired_at is null) then raise exception", "no shared link while the commune has its own bot"},
		{tenantLock, "serialised with the switch checks"},
	} {
		if !strings.Contains(live, c.want) {
			t.Errorf("zalo_link_commune_bot_live lacks %q — %s", c.want, c.why)
		}
	}
	retire := functionBody(t, sql, "zalo_commune_bot_retire_check")
	for _, want := range []string{"where tenant_id = new.tenant_id and bot_ref = new.bot_ref and unlinked_at is null", tenantLock} {
		if !strings.Contains(retire, want) {
			t.Errorf("zalo_commune_bot_retire_check lacks %q", want)
		}
	}
	adopt := functionBody(t, sql, "zalo_commune_bot_adopt_check")
	for _, want := range []string{"where tenant_id = new.tenant_id and bot_ref = 'shared' and unlinked_at is null",
		"where tenant_id = new.tenant_id and id = new.id and retired_at is null", tenantLock} {
		if !strings.Contains(adopt, want) {
			t.Errorf("zalo_commune_bot_adopt_check lacks %q", want)
		}
	}
	// ONE lock key per commune for every switch/pair path — a second key would let a pairing and a
	// switch interleave.
	if strings.Contains(sql, "'zalo-bot-ref:'") {
		t.Error("0022 still takes a per-bot_ref lock; the switch and pairing paths must share the per-commune lock")
	}
	noDestruction(t, file0022, sql)
}
