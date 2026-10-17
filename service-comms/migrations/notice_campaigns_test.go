package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0032 (notice campaigns over ZNS / SMS, several OAs and eSMS accounts
// per commune — ADR 0099, block D). They assert the keys, CHECKs and guards are WRITTEN; they do not
// prove PostgreSQL enforces them — tools/schema-smoke and the pg suites need VIGOV_TEST_DSN for that.

const file0032 = "0032_notice_campaigns_sms_multi_oa.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the one-live-OA-per-commune key kept (ADR 0099 #1 undone) or the
// OA floor / one-template-per-milestone dropped with it; a configuration table partitioned or a growing
// one made plain (ADR 0096); a full phone number admitted (the mask CHECK loosened); a sealed column
// without its envelope bound; a guard no longer refusing DELETE, no longer freezing what was sent, or
// letting 'da-gui' move back; the source no longer tied to the channel; a key without tenant_id; a
// partial key on deleted_at; a seed row.
func TestMigration0032NoticeCampaignsKeysAndGuards(t *testing.T) {
	sql := executableSQL(t, file0032)
	requirePlainTable(t, file0032, sql, "sms_account")
	requirePlainTable(t, file0032, sql, "sms_template")
	requireHashPartitionedBy8(t, file0032, sql, "notice_campaign")
	requireHashPartitionedBy8(t, file0032, sql, "notice_recipient")

	for _, c := range []struct{ want, why string }{
		// (A) several OAs per commune
		{"drop index if exists zalo_commune_oa_one_live_per_commune;", "ADR 0099 #1: a commune may hold several live OAs"},
		{"on zalo_commune_oa (tenant_id, created_at) where retired_at is null", "the commune's live OAs are still read per commune"},
		// sms_account
		{"primary key (tenant_id, id), constraint sms_account_provider_known check (provider in ('esms')),", "composite key; one provider today"},
		{"api_key_sealed bytea not null, secret_key_sealed bytea not null,", "both keys sealed, never nullable"},
		{"check (octet_length(api_key_sealed) > 29)", "a sealed envelope, never a bare key"},
		{"check (octet_length(secret_key_sealed) > 29)", "a sealed envelope, never a bare key"},
		{"check (brandname ~ '^[a-za-z0-9][a-za-z0-9 ._-]{0,31}$' and brandname = btrim(brandname))", "a sender name, bounded"},
		{"check (btrim(label) <> '' and char_length(label) <= 100)", "a name the commune reads"},
		{"check ((last_check_at is null) = (last_check_result is null))", "check pair"},
		{"'thanh-cong', 'sai-khoa', 'brandname-chua-kich-hoat', 'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang'", "the card's six classes, ADR 0011 spelling"},
		{"check (last_balance is null or last_check_result = 'thanh-cong')", "a balance only from a successful check"},
		{"check (btrim(set_by) <> '' and char_length(set_by) <= 64)", "who set the keys is a staff code (rule 6 inv 8)"},
		{"check ((retired_at is null) = (retired_by is null) and (retired_at is null) = (retire_reason is null))", "soft-delete trio all or none (rule 7)"},
		{"check (not (is_default and retired_at is not null))", "a retired account is never the default"},
		{"on sms_account (tenant_id) where is_default;", "one default account per commune"},
		{"on sms_account (tenant_id, lower(label)) where retired_at is null;", "one live account per name per commune"},
		// sms_template
		{"unique (tenant_id, account_id, provider_template_id, network_id), foreign key (tenant_id, account_id) references sms_account (tenant_id, id),", "one row per eSMS TempId per network per account"},
		{"check (provider_template_id ~ '^[0-9a-za-z_-]{1,64}$')", "eSMS's TempId shape"},
		{"check (jsonb_typeof(params) = 'array')", "params is the placeholder array"},
		{"vanished_at timestamptz,", "a vanished template is marked, never removed"},
		// notice_campaign
		{"foreign key (tenant_id, oa_row_id) references zalo_commune_oa (tenant_id, id), foreign key (tenant_id, sms_account_id) references sms_account (tenant_id, id),", "the source is a real OA / account of the same commune"},
		{"check (channel in ('zalo-zns', 'sms'))", "0004's ZNS spelling, and SMS"},
		{"check ((channel = 'zalo-zns' and oa_row_id is not null and sms_account_id is null) or (channel = 'sms' and sms_account_id is not null and oa_row_id is null))", "exactly one source, matching the channel"},
		{"check ((channel = 'sms') = (template_content is not null))", "the SMS text is kept as sent"},
		{"check (jsonb_typeof(params) = 'object')", "params is the staff-typed object"},
		{"check (audience_kind in ('nguoi-dan-tat-ca', 'nguoi-dan-theo-nhom', 'nguoi-dan-theo-nhan', 'can-bo-tat-ca', 'can-bo-theo-bo-phan'))", "ADR 0099 #5's five audiences"},
		{"check (sent_count + failed_count <= recipient_count)", "counts within the rows written"},
		{"check ((unit_price is null) = (estimated_cost is null))", "an estimate is both figures or none"},
		{"check (status in ('dang-gui', 'da-gui', 'gui-mot-phan', 'that-bai'))", "ADR 0099 #8's campaign statuses"},
		{"check ((status = 'dang-gui') = (finished_at is null))", "finished has a time"},
		{"check (btrim(created_by) <> '' and char_length(created_by) <= 64)", "who sent is a staff code (rule 6 inv 8)"},
		{"on notice_campaign (tenant_id, created_at desc, id desc);", "the history tab, per commune"},
		// notice_recipient
		{"unique (tenant_id, campaign_id, recipient_ref), foreign key (tenant_id, campaign_id) references notice_campaign (tenant_id, id),", "one message per recipient per campaign; a real campaign of the same commune"},
		{"check (recipient_kind in ('nguoi-dan', 'can-bo'))", "citizen or staff"},
		{"check (phone_masked like '%*%' and char_length(regexp_replace(phone_masked, '[^0-9]', '', 'g')) <= 6 and char_length(phone_masked) <= 20)", "NEVER a full number (rule 3; 0004's CHECK, stronger)"},
		{"check (status in ('cho-gui', 'da-gui', 'that-bai', 'bo-qua'))", "ADR 0099 #8's recipient outcomes"},
		{"check ((status = 'da-gui') = (sent_at is not null))", "sent has a time"},
		{"check (status <> 'da-gui' or provider_message_id is not null)", "sent has the provider's id"},
		{"check (failure_class is null or (failure_class ~ '^[a-z]+(-[a-z]+)*$' and char_length(failure_class) <= 40))", "a class, never a message or a number"},
		{"check (status = 'cho-gui' or lease_until is null)", "only an owed row is leased"},
		{"on notice_recipient (tenant_id, status, lease_until) where status = 'cho-gui';", "the sender's queue, per commune"},
		{"and not exists (select 1 from pg_inherits where inhparent = c.oid)", "the BACKSTOP is present"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0032 lacks %q — %s", c.want, c.why)
		}
	}

	// ADR 0099 #1 keeps both floors: dropping either is a different decision.
	for _, kept := range []string{"zalo_commune_oa_one_live_per_oa", "zalo_oa_template_one_per_milestone"} {
		if strings.Contains(sql, kept) {
			t.Errorf("0032 touches %s — ADR 0099 #1 keeps it", kept)
		}
	}
	if n := strings.Count(sql, "drop index"); n != 1 {
		t.Errorf("0032 drops %d indexes, want exactly 1 (the one-live-per-commune key)", n)
	}
	// Every index here leads with tenant_id (rule 1); none is partial on deleted_at (rule 7 inv 3).
	for _, banned := range []string{"where deleted_at is null", "on zalo_commune_oa (tenant_id) where", "@scope: cross-tenant"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0032 contains %q", banned)
		}
	}
	// The full number and a rendered per-person message have no column (ADR 0099 #5, rule 3).
	for _, banned := range []string{" phone text", "phone_number", "full_name", "rendered", "phone_full"} {
		if strings.Contains(sql, banned) {
			t.Errorf("0032 contains %q — the ledger holds the masked number only", banned)
		}
	}

	for _, c := range []struct{ fn, table string }{
		{"sms_account_guard", "sms_account"},
		{"sms_template_guard", "sms_template"},
		{"notice_campaign_guard", "notice_campaign"},
		{"notice_recipient_guard", "notice_recipient"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), "if tg_op = 'delete' then raise exception") {
			t.Errorf("%s no longer refuses DELETE", c.fn)
		}
		if !strings.Contains(sql, "before update or delete on "+c.table+" for each row execute function "+c.fn+"()") {
			t.Errorf("%s is not attached to %s", c.fn, c.table)
		}
	}
	for _, c := range []struct{ want, fn, why string }{
		{"if old.retired_at is not null then raise exception", "sms_account_guard", "a retired account is history"},
		{"or new.provider is distinct from old.provider", "sms_account_guard", "another provider is another row"},
		{"if (new.api_key_sealed is distinct from old.api_key_sealed or new.secret_key_sealed is distinct from old.secret_key_sealed or new.brandname is distinct from old.brandname) and new.set_at is not distinct from old.set_at then raise exception", "sms_account_guard", "new credentials leave who and when (0022's rule)"},
		{"or new.account_id is distinct from old.account_id or new.provider_template_id is distinct from old.provider_template_id or new.network_id is distinct from old.network_id", "sms_template_guard", "eSMS's identity of a template is fixed"},
		{"or new.title is distinct from old.title or new.channel is distinct from old.channel or new.oa_row_id is distinct from old.oa_row_id or new.sms_account_id is distinct from old.sms_account_id or new.template_ref is distinct from old.template_ref", "notice_campaign_guard", "what was sent and through what is fixed"},
		{"or new.template_content is distinct from old.template_content or new.params is distinct from old.params or new.audience_kind is distinct from old.audience_kind or new.audience_ids is distinct from old.audience_ids", "notice_campaign_guard", "the text, params and audience are fixed"},
		{"or new.created_at is distinct from old.created_at or new.created_by is distinct from old.created_by then raise exception", "notice_campaign_guard", "who sent it and when is fixed"},
		{"if old.status <> 'dang-gui' and (new.status is distinct from old.status", "notice_campaign_guard", "a finished campaign keeps its outcome (status only forward)"},
		{"if new.sent_count < old.sent_count or new.failed_count < old.failed_count then raise exception", "notice_campaign_guard", "counts never decrease"},
		{"or new.campaign_id is distinct from old.campaign_id or new.recipient_ref is distinct from old.recipient_ref or new.recipient_kind is distinct from old.recipient_kind or new.phone_masked is distinct from old.phone_masked", "notice_recipient_guard", "who and which campaign are fixed"},
		{"if old.status <> 'cho-gui' and (new.status is distinct from old.status", "notice_recipient_guard", "'da-gui' never moves back; a finished row keeps its outcome"},
		{"if new.attempts < old.attempts then raise exception", "notice_recipient_guard", "attempts never decrease"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), c.want) {
			t.Errorf("%s lacks %q — %s", c.fn, c.want, c.why)
		}
	}
	// The template guard must NOT freeze eSMS's columns: every sync rewrites them.
	if g := functionBody(t, sql, "sms_template_guard"); strings.Contains(g, "new.content is distinct") || strings.Contains(g, "new.params is distinct") {
		t.Error("sms_template_guard freezes eSMS's columns — a sync could then never record eSMS's text")
	}
	// No soft-delete trio on the ledger or the templates: they are never removed at all.
	for _, table := range []string{"sms_template", "notice_campaign", "notice_recipient"} {
		if strings.Contains(createTableStatement(t, file0032, sql, table), "deleted_at") {
			t.Errorf("%s carries deleted_at — the 0032 header keeps these rows for ever", table)
		}
	}

	noDestruction(t, file0032, sql)
}

// The ownership marks tools/kb reads: four entities, each `@scope: tenant`, each directly above its own
// table, in order.
func TestMigration0032OwnershipMarks(t *testing.T) {
	b, err := fs.ReadFile(FS, file0032)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	last := -1
	for _, c := range []struct{ entity, table string }{
		{"SmsAccount", "sms_account"},
		{"SmsTemplate", "sms_template"},
		{"NoticeCampaign", "notice_campaign"},
		{"NoticeRecipient", "notice_recipient"},
	} {
		mark := strings.Index(src, "-- @entity: "+c.entity+"\n-- @scope:  tenant\n")
		create := strings.Index(src, "CREATE TABLE IF NOT EXISTS "+c.table+" (")
		if mark < 0 || create < 0 || mark > create || mark < last {
			t.Errorf("@entity: %s / @scope: tenant must precede CREATE TABLE %s, in order", c.entity, c.table)
		}
		if mark >= 0 {
			if next := strings.Index(src[mark+1:], "-- @entity:"); next >= 0 && mark+1+next < create {
				t.Errorf("another @entity mark sits between %s and its table", c.entity)
			}
		}
		last = create
	}
	if n := strings.Count(src, "-- @entity:"); n != 4 {
		t.Errorf("%d @entity marks, want 4", n)
	}
}
