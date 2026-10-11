package store

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Each verb records only its own fields, and an unknown verb is refused rather than written with a
// half-empty object the log screen would show as a real act.
func TestZaloBotActAfter(t *testing.T) {
	got, err := zaloBotActAfter(domain.ZaloBotAct{Action: domain.ZaloBotActionTokenSet, Outcome: "thanh-cong",
		BotName: "Bot nhắc việc", ChatURL: "https://zalo.me/bot-FAKE", EndedLinkCount: 4, WebhookURL: "ignored"})
	if err != nil || got["bot_name"] != "Bot nhắc việc" || got["ended_link_count"] != int32(4) || got["url"] != nil {
		t.Errorf("token_set after = %v %v", got, err)
	}
	got, err = zaloBotActAfter(domain.ZaloBotAct{Action: domain.ZaloBotActionChecked, Outcome: "token-bi-tu-choi", BotName: "x"})
	if err != nil || len(got) != 1 || got["outcome"] != "token-bi-tu-choi" {
		t.Errorf("checked after = %v %v", got, err)
	}
	got, err = zaloBotActAfter(domain.ZaloBotAct{Action: domain.ZaloBotActionWebhookSet, Outcome: "thanh-cong",
		WebhookURL: "https://bot.api.example.invalid/api/v1/zalo-bot-updates"})
	if err != nil || got["url"] == nil {
		t.Errorf("webhook_set after = %v %v", got, err)
	}
	if _, err := zaloBotActAfter(domain.ZaloBotAct{Action: "zalo_bot.other"}); err == nil {
		t.Error("an unknown verb was accepted")
	}
}

// No "who" → refused before any statement (rule 6 invariant 8: no fallback). A nil handle proves no
// statement was attempted.
func TestRecordZaloBotActRefusesNoActor(t *testing.T) {
	l := &ZaloBotDisplayLog{}
	err := l.RecordZaloBotAct(context.Background(), domain.ZaloBotAct{Action: domain.ZaloBotActionChecked}, domain.OperatorActor{IP: "203.0.113.7"})
	if !errors.Is(err, domain.ErrNoActor) {
		t.Fatalf("%v, want ErrNoActor", err)
	}
}
