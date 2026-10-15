package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0029 (Zalo GROUP chats, ADR 0092). They assert the keys, CHECKs,
// guards and the widened bot-switch checks are WRITTEN; they do not prove PostgreSQL enforces them —
// internal/store/zalo_group_link_pg_test.go does, and it SKIPS without VIGOV_TEST_DSN.

const file0029 = "0029_zalo_group_link.sql"

// groupKinds is the ONE list a group may receive (0029 header): 0025's per-domain kinds minus the
// per-person / digest ones, the legacy domainless ones and the undecided announcement.
var groupKinds = []string{
	"'nhiem-vu.qua-han'", "'nhiem-vu.chua-cu-nguoi'", "'nhiem-vu.leo-thang'",
	"'van-ban.qua-han'", "'van-ban.chua-cu-nguoi'", "'van-ban.leo-thang'",
	"'phan-anh.qua-han'", "'phan-anh.chua-cu-nguoi'", "'phan-anh.leo-thang'",
	"'nhiem-vu.giao-moi'", "'nhiem-vu.cho-duyet'", "'van-ban.chuyen-toi'",
	"'phan-anh.phan-cong'", "'phan-anh.mo-lai'",
}

// neverInAGroup are the kinds ADR 0092 #2 and comms.proto GroupPosting keep out of every group, plus the
// legacy and undecided values the header refuses. A mutation admitting any of them must turn this red.
var neverInAGroup = []string{
	"'sap-den-han'", "'nhiem-vu.sap-den-han'", "'van-ban.sap-den-han'", "'phan-anh.sap-den-han'",
	"'ban-tin-tuan'", "'bao-cao.san-sang'", "'nhiem-vu.nhac-ten'", "'nhiem-vu.de-nghi-lui-han'",
	"'giai-ngan.nhac-ten'", "'qua-han'", "'leo-thang'", "'thong-bao.moi'", "'thu-nghiem'",
}

var quoted = regexp.MustCompile(`'[a-z.-]+'`)

// kindList returns the sorted quoted values of the CHECK named `name`, up to its closing `)),` / `])`.
func kindList(t *testing.T, sql, name string) []string {
	t.Helper()
	i := strings.Index(sql, "constraint "+name+" ")
	if i < 0 {
		t.Fatalf("0029 lacks constraint %s", name)
	}
	rest := sql[i:]
	if j := strings.Index(rest, ", constraint "); j > 0 {
		rest = rest[:j]
	}
	got := quoted.FindAllString(rest, -1)
	sort.Strings(got)
	return got
}

// THE MUTATIONS THAT MUST TURN THIS RED: the outbox without its MODULUS 8 loop or the code table
// partitioned again (ADR 0096); zalo_group_link
// partitioned (its platform-wide chat key cannot exist then) or its chat key losing the live-only predicate
// or gaining tenant_id; the dedupe key without tenant_id or made partial; a guard no longer refusing DELETE,
// no longer freezing a closed code / ended link / finished delivery, or no longer fixing the scope of a
// code; the cross-table chat trigger detached from either table; a seed row.
func TestMigration0029ZaloGroupKeysAndGuards(t *testing.T) {
	sql := executableSQL(t, file0029)
	requirePlainTable(t, file0029, sql, "zalo_group_pairing_code")
	requireHashPartitionedBy8(t, file0029, sql, "zalo_group_delivery")
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, id), unique (tenant_id, code_hash),", "a hash never reissued within a commune"},
		{"check (octet_length(code_hash) = 32)", "a hash, never the code"},
		{"on zalo_group_pairing_code (code_hash) where used_at is null and cancelled_at is null", "the webhook's open-code lookup"},
		{"bot_ref text not null, chat_id text not null,", "no default bot (0022:614-617)"},
		{"check (bot_ref = 'shared' or bot_ref ~ '^commune:[!-~]{1,128}$')", "zalo_link's bot_ref shape"},
		{"check (org_unit_id is null or org_unit_id ~ '^[0-9a-hjkmnp-tv-z]{26}$')", "a ULID or the whole commune"},
		{"check (cardinality(kinds) > 0)", "a group that receives nothing is not linked"},
		{"primary key (tenant_id, id), unique (tenant_id, pairing_code_id), foreign key (tenant_id, pairing_code_id) references zalo_group_pairing_code (tenant_id, id),", "one code, one link, and the link names its code"},
		{"check ((unlinked_at is null) = (unlinked_by is null) and (unlinked_at is null) = (unlink_reason is null))", "unlink trio all or none (rule 7)"},
		{"on zalo_group_link (bot_ref, chat_id) where unlinked_at is null", "one live group link per chat, platform-wide (ADR 0092 #9)"},
		{"on zalo_group_link (tenant_id, org_unit_id) where unlinked_at is null", "the enqueue's read, by commune"},
		{"primary key (tenant_id, id), unique (tenant_id, group_link_id, idempotency_key), foreign key (tenant_id, group_link_id) references zalo_group_link (tenant_id, id),", "one message per (key, group) — ADR 0092 #5"},
		{"check (status in ('cho-gui', 'da-gui', 'bo-qua', 'that-bai'))", "0018's states"},
		{"check (skip_reason is null or skip_reason in ('chua-lien-ket', 'kenh-tat', 'loai-tat', 'bot-chua-cau-hinh'))", "0018's reasons; never the due-soon lead"},
		{"'token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang'", "0018's error classes"},
		{"check ((status = 'bo-qua') = (skip_reason is not null))", "every skip has a reason"},
		{"check ((status = 'da-gui') = (sent_at is not null))", "sent has a time"},
		{"check ((status = 'cho-gui') = (next_attempt_at is not null))", "only owed rows are scheduled"},
		{"check (btrim(title) <> '' and char_length(title) <= 200)", "0010's title bound"},
		{"check (link = '' or (char_length(link) <= 300 and left(link, 1) = '/' and left(link, 2) <> '//' and position('\\' in link) = 0))", "0010's relative-link floor"},
		{"check ((deleted_at is null) = (deleted_by is null) and (deleted_at is null) = (delete_reason is null))", "soft-delete trio (rule 7)"},
		{"on zalo_group_delivery (tenant_id, next_attempt_at) where status = 'cho-gui' and deleted_at is null", "the sender's queue, per commune"},
		{"before insert on zalo_link for each row execute function zalo_chat_one_link_kind()", "a group chat is not paired personally"},
		{"before insert on zalo_group_link for each row execute function zalo_chat_one_link_kind()", "a personal chat is not linked as a group"},
		{"before insert on zalo_group_link for each row execute function zalo_link_commune_bot_live()", "0022's bot-liveness rule on group links"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0029 lacks %q — %s", c.want, c.why)
		}
	}

	// zalo_group_link is deliberately NOT partitioned: its platform-wide chat key cannot contain tenant_id.
	link := sql[strings.Index(sql, "create table if not exists zalo_group_link ("):]
	link = link[:strings.Index(link, ";")]
	if strings.Contains(link, "partition by") || strings.Contains(sql, "partition of zalo_group_link ") {
		t.Error("zalo_group_link is partitioned — its platform-wide chat key cannot exist on a partitioned table")
	}
	if strings.Contains(sql, "on zalo_group_link (tenant_id, bot_ref, chat_id)") {
		t.Error("the group chat key gained tenant_id — two communes could then hold one group live (ADR 0092 #9)")
	}

	for _, c := range []struct{ fn, table string }{
		{"zalo_group_pairing_code_guard", "zalo_group_pairing_code"},
		{"zalo_group_link_guard", "zalo_group_link"},
		{"zalo_group_delivery_guard", "zalo_group_delivery"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), "if tg_op = 'delete' then raise exception") {
			t.Errorf("%s no longer refuses DELETE", c.fn)
		}
		if !strings.Contains(sql, "before update or delete on "+c.table+" for each row execute function "+c.fn+"()") {
			t.Errorf("%s is not attached to %s", c.fn, c.table)
		}
	}
	for _, c := range []struct{ want, fn, why string }{
		{"if old.used_at is not null or old.cancelled_at is not null then raise exception", "zalo_group_pairing_code_guard", "a spent code is frozen"},
		{"or new.org_unit_id is distinct from old.org_unit_id or new.kinds is distinct from old.kinds", "zalo_group_pairing_code_guard", "an issued code's scope is fixed"},
		{"if new.failed_attempts < old.failed_attempts then raise exception", "zalo_group_pairing_code_guard", "no fresh guesses"},
		{"if old.unlinked_at is not null then raise exception", "zalo_group_link_guard", "an ended link is history"},
		{"or new.chat_id is distinct from old.chat_id", "zalo_group_link_guard", "a link is never re-pointed"},
		{"or new.linked_by is distinct from old.linked_by", "zalo_group_link_guard", "who linked is fixed (rule 6 inv 8)"},
		{"if old.deleted_at is not null then raise exception", "zalo_group_delivery_guard", "a deleted row is history"},
		{"or new.title is distinct from old.title or new.body is distinct from old.body or new.link is distinct from old.link", "zalo_group_delivery_guard", "what was posted is fixed"},
		{"if old.status <> 'cho-gui'", "zalo_group_delivery_guard", "a finished delivery keeps its outcome"},
		{"if new.attempts < old.attempts then raise exception", "zalo_group_delivery_guard", "attempts never decrease"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), c.want) {
			t.Errorf("%s lacks %q — %s", c.fn, c.want, c.why)
		}
	}
	// The group link guard must NOT freeze scope and kinds on a live link (decided editable, audited by Go).
	if g := functionBody(t, sql, "zalo_group_link_guard"); strings.Contains(g, "new.kinds is distinct") || strings.Contains(g, "new.org_unit_id is distinct") {
		t.Error("zalo_group_link_guard freezes scope or kinds — a live group could then never be narrowed")
	}

	// The cross-table chat rule: one lock key for both tables, taken before the EXISTS.
	one := functionBody(t, sql, "zalo_chat_one_link_kind")
	lock := "perform pg_advisory_xact_lock(hashtextextended('zalo-chat:' || new.bot_ref || ':' || new.chat_id, 0));"
	if !strings.HasPrefix(strings.SplitN(one, "begin ", 2)[1], lock) {
		t.Error("zalo_chat_one_link_kind does not take the per-chat lock first")
	}
	for _, want := range []string{
		"select 1 from zalo_group_link where bot_ref = new.bot_ref and chat_id = new.chat_id and unlinked_at is null",
		"select 1 from zalo_link where bot_ref = new.bot_ref and chat_id = new.chat_id and unlinked_at is null",
	} {
		if !strings.Contains(one, want) {
			t.Errorf("zalo_chat_one_link_kind lacks %q", want)
		}
	}
	noDestruction(t, file0029, sql)
}

// THE MUTATIONS THAT MUST TURN THIS RED: a group CHECK admitting a per-person, digest, legacy or undecided
// kind; the three group lists drifting apart; a group kind the commune settings CHECK (0025) does not know.
func TestMigration0029GroupKinds(t *testing.T) {
	sql := executableSQL(t, file0029)
	want := append([]string{}, groupKinds...)
	sort.Strings(want)
	for _, name := range []string{"zalo_group_pairing_code_kinds_known", "zalo_group_link_kinds_known", "zalo_group_delivery_kind_known"} {
		got := kindList(t, sql, name)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s = %v, want exactly %v", name, got, want)
		}
		for _, bad := range neverInAGroup {
			for _, g := range got {
				if g == bad {
					t.Errorf("%s admits %s — ADR 0092 #2 / comms.proto GroupPosting keep it out of groups", name, bad)
				}
			}
		}
	}
	settings := executableSQL(t, file0025)
	i := strings.Index(settings, "add constraint zalo_channel_setting_kinds_with_act_notices ")
	if i < 0 {
		t.Fatal("0025's settings CHECK not found")
	}
	settings = settings[i:]
	settings = settings[:strings.Index(settings, "end if;")]
	for _, k := range groupKinds {
		if !strings.Contains(settings, k) {
			t.Errorf("group kind %s is not a commune Zalo kind (0025) — a group may only hold a subset (ADR 0092 #2)", k)
		}
	}
}

// THE MUTATIONS THAT MUST TURN THIS RED: a replaced bot-switch check losing its original personal-link
// test (the replacement must be a superset), or not gaining the group-link test; the adopt/retire checks
// losing the per-commune lock.
func TestMigration0029BotSwitchChecksCoverGroups(t *testing.T) {
	sql := executableSQL(t, file0029)
	for _, c := range []struct{ fn, origin, group string }{
		{"zalo_bot_shared_account_change_check", file0018,
			"exists (select 1 from zalo_group_link where bot_ref = new.bot_ref and unlinked_at is null and linked_at < new.set_at)"},
		{"zalo_commune_bot_adopt_check", file0022,
			"exists (select 1 from zalo_group_link where tenant_id = new.tenant_id and bot_ref = 'shared' and unlinked_at is null)"},
		{"zalo_commune_bot_retire_check", file0022,
			"exists (select 1 from zalo_group_link where tenant_id = new.tenant_id and bot_ref = new.bot_ref and unlinked_at is null)"},
	} {
		orig := functionBody(t, executableSQL(t, c.origin), c.fn)
		// The original's first block, through its first `end if;`, must survive verbatim.
		first := orig[:strings.Index(orig, "end if;")+len("end if;")]
		now := functionBody(t, sql, c.fn)
		if !strings.Contains(now, first) {
			t.Errorf("%s no longer contains its %s personal-link check verbatim", c.fn, c.origin)
		}
		if !strings.Contains(now, c.group) {
			t.Errorf("%s lacks the group-link check %q", c.fn, c.group)
		}
		if !strings.Contains(now, "return null;") {
			t.Errorf("%s is no longer an AFTER (constraint) trigger function", c.fn)
		}
	}
	// 0029 replaces functions only; the deferred triggers of 0018/0022 stay as they are.
	for _, banned := range []string{"drop trigger if exists zalo_commune_bot_adopt_check", "drop trigger if exists zalo_commune_bot_retire_check",
		"drop trigger if exists zalo_bot_shared_account_change_check", "drop trigger if exists zalo_link_commune_bot_live"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0029 touches an existing trigger: %q", banned)
		}
	}
}

// The ownership marks tools/kb reads: three entities, each `@scope: tenant`, each above its own table.
func TestMigration0029OwnershipMarks(t *testing.T) {
	b, err := fs.ReadFile(FS, file0029)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	last := -1
	for _, c := range []struct{ entity, table string }{
		{"ZaloGroupPairingCode", "zalo_group_pairing_code"},
		{"ZaloGroupLink", "zalo_group_link"},
		{"ZaloGroupDelivery", "zalo_group_delivery"},
	} {
		mark := strings.Index(src, "-- @entity: "+c.entity+"\n-- @scope:  tenant\n")
		create := strings.Index(src, "CREATE TABLE IF NOT EXISTS "+c.table+" (")
		if mark < 0 || create < 0 || mark > create || mark < last {
			t.Errorf("@entity: %s / @scope: tenant must precede CREATE TABLE %s, in order", c.entity, c.table)
		}
		if next := strings.Index(src[mark+1:], "-- @entity:"); next >= 0 && mark+1+next < create {
			t.Errorf("another @entity mark sits between %s and its table", c.entity)
		}
		last = create
	}
	if n := strings.Count(src, "-- @entity:"); n != 3 {
		t.Errorf("%d @entity marks, want 3", n)
	}
}
