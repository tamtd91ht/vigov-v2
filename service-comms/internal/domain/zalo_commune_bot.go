package domain

// A COMMUNE'S OWN ZALO BOT (migration 0022; ADR 0079 #4 and §Q1 #1–#5). The commune configures it in
// web-admin (Cấu hình › Kênh Zalo); no live row = the commune uses the shared bot (zalo_bot.go). Standard
// library only.
//
// THE TOKEN AND THE WEBHOOK SECRET ARE NOT FIELDS HERE. They are sealed under the commune's data key and
// travel only as sealed bytes (store) or as a secret.Secret on their way to Zalo (app) — never in a value
// that could be returned, logged or compared.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CommuneZaloBotRefPrefix is the bot_ref prefix of a commune bot's links: 0022 generates
// `bot_ref = 'commune:' || bot_account_id` on the bot row, and zalo_link.bot_ref is compared to it.
const CommuneZaloBotRefPrefix = "commune:"

// CommuneZaloBot is the LIVE zalo_commune_bot row as metadata.
type CommuneZaloBot struct {
	ID           string // ULID, internal
	BotAccountID string // Zalo getMe's id — never typed by a person (ADR 0079 Q1 #1)
	BotName      string
	ChatURL      string
	// SetAt / SetBy: when and by whom (staff CODE, rule 6 invariant 8) the token was set.
	SetAt time.Time
	SetBy string
	// LastCheckAt / LastCheckResult: both zero, or both set (0022 zalo_commune_bot_check_pair);
	// the result is a ZaloCall* value.
	LastCheckAt     time.Time
	LastCheckResult string
	// WebhookSetAt / WebhookSetBy: the secret IN FORCE; zero when none was ever made current.
	WebhookSetAt time.Time
	WebhookSetBy string
	// HasPendingWebhook: a secret is stored as pending (requested, not yet confirmed by Zalo).
	HasPendingWebhook bool
}

// ValidCommuneZaloBotAccountID mirrors 0022's zalo_commune_bot_account_id_shape (`^[!-~]{1,128}$`):
// printable ASCII, no space, so the bot_ref built from it is one unambiguous token. Checked on what getMe
// answered BEFORE anything is sealed or written.
func ValidCommuneZaloBotAccountID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

// Ref is the bot_ref this bot's links carry.
func (b CommuneZaloBot) Ref() string { return CommuneZaloBotRefPrefix + b.BotAccountID }

// Subject is the audit subject of an act on this bot: its Zalo account id, the identity that outlives
// the row id (a new token for the same bot keeps it; a different bot is a different subject).
func (b CommuneZaloBot) Subject() string { return "zalo-bot/" + b.BotAccountID }

// The commune audit_log verbs of the commune bot (values, ADR 0011).
const (
	ActionSetCommuneZaloBot             = "luu_bot_zalo_rieng"
	ActionCheckCommuneZaloBot           = "kiem_tra_bot_zalo_rieng"
	ActionRequestCommuneZaloWebhook     = "yeu_cau_webhook_bot_zalo_rieng"
	ActionSetCommuneZaloWebhook         = "dang_ky_webhook_bot_zalo_rieng"
	ActionRefusedCommuneZaloWebhook     = "webhook_bot_zalo_rieng_bi_tu_choi"
	ActionUnconfirmedCommuneZaloWebhook = "webhook_bot_zalo_rieng_chua_xac_nhan"
	ActionRetireCommuneZaloBot          = "quay_ve_bot_zalo_chung"
)

// CommuneZaloBotReplacedReason is retire_reason when the commune saves a DIFFERENT bot over its live one
// — a fixed sentence (0022 CHECK ≤ 200). "Quay về bot chung" carries the administrator's own reason.
const CommuneZaloBotReplacedReason = "Xã đổi sang một bot Zalo riêng khác"

// CommuneZaloBotRevokeNotice is said in the reply of every act that retires a bot (0022 "OWED BY GO"):
// retiring revokes nothing at Zalo, and an unrevoked token is a live credential for a bot that still
// carries the commune's name. Only the commune can revoke it.
const CommuneZaloBotRevokeNotice = "Mã của con bot cũ vẫn còn hiệu lực ở phía Zalo. Hãy vào Mini App “Zalo Bot Creator” " +
	"để thu hồi hoặc tạo lại mã của con bot đó."

// ErrCommuneZaloBotInvalid is the family of request faults of the commune bot routes. Each error below
// wraps it; its text is the sentence the screen shows.
var ErrCommuneZaloBotInvalid = errors.New("zalo_bot_rieng: invalid")

var (
	ErrCommuneZaloBotToken = fmt.Errorf("%w: Mã bot không hợp lệ. Hãy chép nguyên mã lấy trong Mini App “Zalo Bot Creator”.",
		ErrCommuneZaloBotInvalid)
	ErrCommuneZaloBotTokenRequired = fmt.Errorf("%w: Xã chưa có bot riêng nên phải nhập mã bot.", ErrCommuneZaloBotInvalid)
	ErrCommuneZaloBotName          = fmt.Errorf("%w: Tên bot phải bắt đầu bằng “Bot”, tối đa %d ký tự, không có ký tự điều khiển.",
		ErrCommuneZaloBotInvalid, MaxZaloBotNameRunes)
	ErrCommuneZaloBotChatURL = fmt.Errorf("%w: Đường mở khung chat phải là một liên kết https:// đầy đủ, tối đa %d ký tự.",
		ErrCommuneZaloBotInvalid, MaxZaloChatURLLen)
	ErrCommuneZaloBotRetireReason = fmt.Errorf("%w: Cần nhập lý do quay về bot chung, tối đa %d ký tự.",
		ErrCommuneZaloBotInvalid, maxCommuneBotRetireReasonRunes)
)

// maxCommuneBotRetireReasonRunes is 0022's zalo_commune_bot_retire_reason_length.
const maxCommuneBotRetireReasonRunes = 200

// NormalizeCommuneZaloBotName is the shared bot's name rule plus spec Cấu hình 11 §3's "bắt đầu bằng
// “Bot”" — Zalo Bot Creator's own naming rule, which 0022 leaves to Go.
func NormalizeCommuneZaloBotName(s string) (string, error) {
	name, err := NormalizeZaloBotName(s)
	if err != nil || !strings.HasPrefix(name, "Bot") {
		return "", ErrCommuneZaloBotName
	}
	return name, nil
}

// ValidateCommuneZaloChatURL is the shared bot's chat-link rule, refused with the commune screen's
// sentence.
func ValidateCommuneZaloChatURL(s string) error {
	if ValidateZaloChatURL(s) != nil {
		return ErrCommuneZaloBotChatURL
	}
	return nil
}

// NormalizeCommuneZaloBotRetireReason — required, trimmed, ≤ 200 characters, no control characters
// (rule 7: a soft delete carries its reason).
func NormalizeCommuneZaloBotRetireReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxCommuneBotRetireReasonRunes {
		return "", ErrCommuneZaloBotRetireReason
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrCommuneZaloBotRetireReason
		}
	}
	return s, nil
}
