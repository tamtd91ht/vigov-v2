package domain

// ZaloBotAct is what platform's DISPLAY row records about an act on the shared Zalo Bot that comms
// already answered (ADR 0074, owner 05/10/2026: "comms ghi vết đầy đủ … platform ghi thêm dòng siêu
// dữ liệu cho màn nhật ký vận hành"). Comms' own trail entry is THE record; this row only lets the
// operator log screen show who did what, when, and the outcome class.
//
// DELIBERATELY NO FIELD THAT COULD HOLD THE TOKEN, the webhook secret_token, or anything derived from
// either: the bot token is write-only (rule 8; zalo_bot_operator.proto), and a type with nowhere to
// put it is the cheapest guarantee that the display row cannot carry it. No reason either — the owner
// decided on 11/10/2026 that no Zalo write asks for one.
type ZaloBotAct struct {
	Action string // one of the ZaloBotAction* verbs
	// Outcome is the Zalo call class as the wire shows it ("thanh-cong", "token-bi-tu-choi", …).
	Outcome string
	// Set on ZaloBotActionTokenSet only: the display fields now live, and how many staff links the
	// change of bot account ended (0 for the same account).
	BotName        string
	ChatURL        string
	EndedLinkCount int32
	// Set on ZaloBotActionWebhookSet only, when Zalo was called: the URL comms asked Zalo to call.
	// Comms builds it; it is not a secret.
	WebhookURL string
}

// The platform_audit_log verbs of the three writes of the Zalo Bot screen.
const (
	ZaloBotActionTokenSet   = "zalo_bot.token_set"
	ZaloBotActionChecked    = "zalo_bot.checked"
	ZaloBotActionWebhookSet = "zalo_bot.webhook_set"
)

// ZaloBotSubject is the platform_audit_log subject of every row above: the ONE shared bot. Not a
// commune — the bot belongs to none (ADR 0074 #4).
const ZaloBotSubject = "zalo-bot-shared"
