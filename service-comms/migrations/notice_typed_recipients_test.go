package migrations

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Schema-TEXT checks for migration 0033 (typed / imported recipients — ADR 0099 §Sửa đổi S1–S7, TASK D-7).
// They assert the CHECKs and guards are WRITTEN; PostgreSQL enforcing them is store's pg suite
// (TestPgNoticeTypedRecipients), which needs VIGOV_TEST_DSN.

const file0033 = "0033_notice_campaign_typed_recipients.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a sealed column allowed on a finished row (S6 undone); the guard
// letting a seal be rewritten instead of only emptied; error_message admitting a phone-length digit run or
// staying editable once final; template_ref NULL beyond SMS-to-typed; the mapping editable after the send;
// the failure-class list drifting from internal/domain; a narrower 0032 constraint left in place beside
// its replacement; a row rewrite or a dropped column.
func TestMigration0033TypedRecipients(t *testing.T) {
	sql := executableSQL(t, file0033)
	for _, c := range []struct{ want, why string }{
		{"alter table notice_campaign add column if not exists sms_content_mode text;", "S3 mode"},
		{"alter table notice_campaign add column if not exists invalid_count integer not null default 0;", "S5 invalid count"},
		{"alter table notice_campaign add column if not exists recipient_mapping jsonb;", "the imported file's mapping"},
		{"alter table notice_campaign alter column template_ref drop not null;", "no template for typed SMS"},
		{"alter table notice_recipient add column if not exists phone_sealed bytea;", "S6 sealed number"},
		{"alter table notice_recipient add column if not exists payload_sealed bytea;", "S6 sealed message"},
		{"alter table notice_recipient add column if not exists error_message text;", "S5 provider message"},
		{"'can-bo-theo-bo-phan', 'danh-sach-so', 'tep-nhap')", "two new audiences"},
		{"when audience_kind in ('nguoi-dan-tat-ca', 'can-bo-tat-ca', 'danh-sach-so', 'tep-nhap') then jsonb_array_length(audience_ids) = 0", "typed audiences carry no ids"},
		{"case when channel = 'sms' and audience_kind in ('danh-sach-so', 'tep-nhap') then template_ref is null else coalesce(btrim(template_ref) <> '' and char_length(template_ref) <= 64, false) end", "template_ref NULL exactly for typed SMS"},
		{"then coalesce(sms_content_mode in ('cot-trong-tep', 'noi-dung-chung'), false) else sms_content_mode is null end", "mode exactly for typed SMS"},
		{"then (coalesce(sms_content_mode, '') = 'noi-dung-chung') = (template_content is not null)", "shared text kept, per-row none"},
		{"check (invalid_count >= 0 and (invalid_count = 0 or audience_kind in ('danh-sach-so', 'tep-nhap')))", "invalid only for typed"},
		{"case when audience_kind = 'tep-nhap' then coalesce(jsonb_typeof(recipient_mapping) = 'object' and jsonb_typeof(recipient_mapping -> 'phone_column') = 'string', false) else recipient_mapping is null end", "mapping exactly for tep-nhap"},
		{"check (recipient_kind in ('nguoi-dan', 'can-bo', 'so-nhap'))", "typed rows"},
		{"check (status in ('cho-gui', 'da-gui', 'that-bai', 'bo-qua', 'khong-hop-le'))", "S5 invalid status"},
		{"case when recipient_kind = 'so-nhap' and status = 'cho-gui' then phone_sealed is not null and payload_sealed is not null else phone_sealed is null and payload_sealed is null end", "S6: sealed exactly while owed"},
		{"(phone_sealed is null or octet_length(phone_sealed) > 29) and (payload_sealed is null or octet_length(payload_sealed) > 29)", "a sealed envelope, never a bare number"},
		{"status <> 'khong-hop-le' or (recipient_kind = 'so-nhap' and attempts = 0 and provider_message_id is null and failure_class is null)", "an invalid number never reached a provider"},
		{"status = 'that-bai' and btrim(error_message) <> '' and char_length(error_message) <= 500 and error_message !~ '[0-9]{7}'", "provider text on failure only, no phone-length run"},
		{"on notice_recipient (tenant_id, campaign_id, id);", "the per-number history, per commune"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0033 lacks %q — %s", c.want, c.why)
		}
	}
	// The narrower 0032 constraints are replaced, not left beside the wider ones.
	for _, old := range []string{"notice_campaign_audience_kind_known", "notice_campaign_audience_ids_shape",
		"notice_campaign_template_ref_shape", "notice_campaign_template_content_for_sms", "notice_recipient_kind_known",
		"notice_recipient_status_known"} {
		if !strings.Contains(sql, "drop constraint if exists "+old+";") {
			t.Errorf("0033 does not drop %s", old)
		}
	}
	// The failure-class list IS internal/domain's, in its order.
	quoted := make([]string, 0, len(domain.NoticeFailureClasses()))
	for _, c := range domain.NoticeFailureClasses() {
		quoted = append(quoted, "'"+c+"'")
	}
	if !strings.Contains(sql, "failure_class in ("+strings.Join(quoted, ", ")+")") {
		t.Error("0033's failure_class list differs from domain.NoticeFailureClasses()")
	}
	for _, c := range []struct{ want, fn, why string }{
		{"or new.sms_content_mode is distinct from old.sms_content_mode", "notice_campaign_guard", "the mode is fixed at the send"},
		{"or new.invalid_count is distinct from old.invalid_count", "notice_campaign_guard", "the invalid count is fixed at the send"},
		{"or new.recipient_mapping is distinct from old.recipient_mapping", "notice_campaign_guard", "the mapping is fixed at the send"},
		{"if tg_op = 'delete' then raise exception", "notice_campaign_guard", "DELETE refused"},
		{"if tg_op = 'delete' then raise exception", "notice_recipient_guard", "DELETE refused"},
		{"if (new.phone_sealed is distinct from old.phone_sealed and new.phone_sealed is not null) or (new.payload_sealed is distinct from old.payload_sealed and new.payload_sealed is not null) then raise exception", "notice_recipient_guard", "a seal may only be emptied"},
		{"or new.error_message is distinct from old.error_message", "notice_recipient_guard", "the provider text is final with the row"},
		{"or new.phone_masked is distinct from old.phone_masked", "notice_recipient_guard", "the masked number is fixed"},
	} {
		if !strings.Contains(functionBody(t, sql, c.fn), c.want) {
			t.Errorf("%s lacks %q — %s", c.fn, c.want, c.why)
		}
	}
	noDestruction(t, file0033, sql)
}

// 0033 declares no new entity: tools/kb keeps reading the ownership marks of 0032.
func TestMigration0033DeclaresNoEntity(t *testing.T) {
	b, err := fs.ReadFile(FS, file0033)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "-- @entity:") {
		t.Error("0033 adds an @entity mark — it only widens 0032's two tables")
	}
}
