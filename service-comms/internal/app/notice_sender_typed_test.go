package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	"github.com/vihat/vigov/service-comms/internal/zalooa"
)

// The sender's typed / imported path (0033; ADR 0099 S6). What is proved: identity is never asked; the
// row's sealed number and message are opened and used for exactly one provider call; a refusal records the
// provider's text with the number masked; a seal that does not open keeps the row owed; no number reaches
// a log, a statement or the store.

// sealTyped builds a typed due row the way sendTyped writes it.
func sealTyped(t *testing.T, r *senderRig, id, phone string, p domain.NoticePayload) domain.DueNoticeRecipient {
	t.Helper()
	ctx := tenant.Into(context.Background(), xaA)
	raw, _ := json.Marshal(p)
	sealed, err := r.env.SealMany(ctx, []secret.Secret{secret.Secret(phone), secret.Secret(raw)},
		[][]byte{noticePhoneAAD(ctx, id), noticePayloadAAD(ctx, id)})
	if err != nil {
		t.Fatal(err)
	}
	return domain.DueNoticeRecipient{ID: id, CampaignID: senderCampaign, Ref: id, Kind: domain.NoticeRecipientTyped, Attempts: 1,
		PhoneSealed: sealed[0], PayloadSealed: sealed[1]}
}

func TestNoticeSenderTypedSms(t *testing.T) {
	r := newSenderRig(t, domain.NoticeChannelSMS)
	r.repo.campaign.TemplateRef, r.repo.campaign.TemplateContent, r.repo.campaign.Params = "", "", nil
	r.repo.campaign.SmsContentMode = domain.SmsContentPerRow
	r.repo.campaign.Audience = domain.NoticeAudience{Kind: domain.NoticeAudienceTypedList}
	broken := sealTyped(t, r, "R3", senderPhone1, domain.NoticePayload{Text: "x"})
	broken.ID = "R9" // bytes of R3 presented as R9: the row-bound AAD refuses them
	r.repo.due = []domain.DueNoticeRecipient{
		sealTyped(t, r, "R1", senderPhone1, domain.NoticePayload{Text: "Nội dung riêng một"}),
		sealTyped(t, r, "R2", senderPhone2, domain.NoticePayload{Text: "Nội dung riêng hai"}),
		broken,
	}
	r.sms.outcomes = map[string]esms.Outcome{senderPhone2: esms.OutcomeTemplateMismatch}
	r.sms.texts = map[string]string{senderPhone2: "Content not match template for 0900000001"}
	r.tick(t)

	if len(r.phones.calls) != 0 {
		t.Fatalf("identity was asked for typed rows: %v", r.phones.calls)
	}
	if len(r.sms.messages) != 2 || r.sms.messages[0].Phone != senderPhone1 || r.sms.messages[0].Content != "Nội dung riêng một" ||
		r.sms.messages[0].RequestID != "R1" || r.sms.messages[1].Content != "Nội dung riêng hai" {
		t.Fatalf("messages = %+v", r.sms.messages)
	}
	want := []string{"sent:R1:SMS-R1", "fail:R2:" + domain.NoticeFailTemplateRejected, "retry:R9:1m0s"}
	if strings.Join(r.repo.calls, " ") != strings.Join(want, " ") {
		t.Fatalf("outcomes = %v, want %v", r.repo.calls, want)
	}
	if len(r.repo.texts) != 1 || r.repo.texts[0] != "R2:Content not match template for ***" {
		t.Fatalf("provider texts = %v", r.repo.texts)
	}
	r.assertNoPhoneEscaped(t)
	if strings.Contains(r.logs.String(), "Nội dung") {
		t.Fatalf("a row's text reached the log: %s", r.logs.String())
	}
}

func TestNoticeSenderTypedZnsUsesTheRowsParams(t *testing.T) {
	r := newSenderRig(t, domain.NoticeChannelZNS)
	r.repo.campaign.Audience = domain.NoticeAudience{Kind: domain.NoticeAudienceImportedFile}
	r.repo.due = []domain.DueNoticeRecipient{
		sealTyped(t, r, "R1", senderPhone1, domain.NoticePayload{Params: map[string]string{"su_kien": "Họp tổ 3"}}),
	}
	r.zns.outcomes = map[string]zalooa.Outcome{}
	r.tick(t)
	if len(r.zns.sent) != 1 || r.zns.sent[0] != senderPhone1+"|1001|R1|Họp tổ 3" {
		t.Fatalf("sent = %v", r.zns.sent)
	}
	if strings.Join(r.repo.calls, " ") != "sent:R1:zmsg-R1" || len(r.phones.calls) != 0 {
		t.Fatalf("outcomes = %v, reveals %v", r.repo.calls, r.phones.calls)
	}
	r.assertNoPhoneEscaped(t)
}

func TestNoticeSenderTypedWithoutKeyKeepsRowsOwedAndOpensNothingWhenTheSourceIsGone(t *testing.T) {
	r := newSenderRig(t, domain.NoticeChannelSMS)
	r.repo.due = []domain.DueNoticeRecipient{sealTyped(t, r, "R1", senderPhone1, domain.NoticePayload{Text: "x"})}
	r.env = nil // the deployment has no SECRET_ENCRYPTION_KEYS
	r.tick(t)
	if strings.Join(r.repo.calls, " ") != "retry:R1:1m0s" || len(r.sms.messages) != 0 {
		t.Fatalf("no key: %v", r.repo.calls)
	}
	r = newSenderRig(t, domain.NoticeChannelSMS)
	r.repo.due = []domain.DueNoticeRecipient{sealTyped(t, r, "R1", senderPhone1, domain.NoticePayload{Text: "x"})}
	r.accounts.err = ErrSmsAccountNotFound
	r.tick(t)
	if strings.Join(r.repo.calls, " ") != "fail:R1:"+domain.NoticeFailSourceRetired || len(r.sms.messages) != 0 {
		t.Fatalf("retired account: %v", r.repo.calls)
	}
}
