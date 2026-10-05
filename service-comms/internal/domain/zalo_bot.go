package domain

// The shared Zalo Bot's platform-scope vocabulary (ADR 0074; proto/vigov/comms/v1/zalo_bot_operator.proto;
// migrations/0018_zalo_bot.sql). Standard library only.

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The stored class of one Zalo call — `zalo_bot_shared.last_check_result` and
// `zalo_delivery.error_class` (0018). VALUES, Vietnamese without diacritics (ADR 0011); the English side
// of the mapping is the contract enum ZaloBotCallOutcome.
const (
	ZaloCallOK                = "thanh-cong"
	ZaloCallNotConfigured     = "chua-cau-hinh"
	ZaloCallTokenRejected     = "token-bi-tu-choi"
	ZaloCallRateLimited       = "gioi-han-tan-suat"
	ZaloCallUnavailable       = "khong-kha-dung"
	ZaloCallRejected          = "bi-tu-choi"
	ZaloCallMalformedResponse = "phan-hoi-sai-dang"
)

// ZaloWebhookPath is the owner-decided resource Zalo posts updates to (ADR 0074 §"Tài nguyên URL").
const ZaloWebhookPath = "/api/v1/zalo-bot-updates"

// ZaloWebhookURL is the URL comms asks Zalo to call: https, the platform-wide host, the fixed path.
// "" when the host is not configured (dev only).
func ZaloWebhookURL(host string) string {
	if host == "" {
		return ""
	}
	return "https://" + host + ZaloWebhookPath
}

// SharedZaloBotRef is the one row's key in zalo_bot_shared and the bot_ref of every shared-bot link.
const SharedZaloBotRef = "shared"

// The platform operator trail (platform_operator_audit_log, 0018). The verbs follow that table's own
// examples ('zalo_bot.token_set', 'zalo_bot.communes_read'); the target is the record's business key.
const (
	ZaloBotTarget                   = "zalo_bot_shared/shared"
	ActionZaloBotTokenSet           = "zalo_bot.token_set"
	ActionZaloBotChecked            = "zalo_bot.checked"
	ActionZaloBotWebhookRequested   = "zalo_bot.webhook_requested"
	ActionZaloBotWebhookSet         = "zalo_bot.webhook_set"
	ActionZaloBotWebhookRefused     = "zalo_bot.webhook_refused"
	ActionZaloBotWebhookUnconfirmed = "zalo_bot.webhook_unconfirmed"
	ActionZaloBotCommunesRead       = "zalo_bot.communes_read"
	PlatformDataKeyTarget           = "platform_data_encryption_key/platform"
	ActionPlatformDataKeyCreated    = "platform_data_key.create"
	ActionPlatformDataKeyRewrapped  = "platform_data_key.rewrap"
)

// ActionEndZaloLink is the COMMUNE audit_log verb (a value, ADR 0011) for a link of one member of staff
// that was ended. A change of bot account writes one per link, in that link's commune, attributed to the
// operator (actor_kind operator, so the commune's own screen withholds it — core/audit.KindOperator).
const ActionEndZaloLink = "ket_thuc_lien_ket_zalo"

// ZaloLinkEndedByBotChange is `zalo_link.unlink_reason` when a change of bot account ended the link — a
// fixed sentence, as 0018 asks ("unlink_reason a fixed sentence"). ≤ 200 characters by its CHECK.
const ZaloLinkEndedByBotChange = "Đổi sang tài khoản Zalo Bot khác — chat cũ không còn nhận được tin, cán bộ cần ghép lại"

// UnknownActorIP is what an empty actor_ip is stored as (zalo_bot_operator.proto: "Empty is allowed
// (stored as unknown), never invented"). platform_operator_audit_log.ip is NOT NULL and non-blank.
const UnknownActorIP = "unknown"

// The field bounds of SetSharedZaloBotRequest.
const (
	MaxZaloBotNameRunes = 100
	MaxZaloChatURLLen   = 300
	maxOperatorCodeLen  = 64
)

// ErrZaloBotInvalidArgument is the family of caller faults decided from the request alone
// (INVALID_ARGUMENT). Each error below wraps it; its text names the RULE, never the value.
var ErrZaloBotInvalidArgument = errors.New("zalo bot: invalid argument")

var (
	ErrZaloBotActorCode   = fmt.Errorf("%w: actor_code must be an operator business code VH-<at least five digits> — no fallback", ErrZaloBotInvalidArgument)
	ErrZaloBotActorIP     = fmt.Errorf("%w: actor_ip must be empty or an IP address", ErrZaloBotInvalidArgument)
	ErrZaloBotToken       = fmt.Errorf("%w: token must be 1-256 bytes of printable ASCII with no whitespace and none of / ? # %%", ErrZaloBotInvalidArgument)
	ErrZaloBotName        = fmt.Errorf("%w: bot_name must be 1-100 characters of valid UTF-8 with no control characters", ErrZaloBotInvalidArgument)
	ErrZaloBotChatURL     = fmt.Errorf("%w: chat_url must be an absolute https:// URL of at most 300 characters, with no user info", ErrZaloBotInvalidArgument)
	ErrZaloBotRelinkCount = fmt.Errorf("%w: expected_relink_count must not be negative", ErrZaloBotInvalidArgument)
)

// ValidOperatorCode is `^VH-[0-9]{5,}$` — the shape of identity's domain.ValidOperatorCode (which comms
// may not import, rule 2) and of platform_operator_audit_log's CHECK.
func ValidOperatorCode(s string) bool {
	rest, ok := strings.CutPrefix(s, "VH-")
	if !ok || len(rest) < 5 || len(s) > maxOperatorCodeLen {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// NormalizeActorIP returns the IP to store: UnknownActorIP for "", the canonical form of a valid
// address, or ErrZaloBotActorIP. It decides nothing (proto) — it is only refused when it is not an IP.
func NormalizeActorIP(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return UnknownActorIP, nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return "", ErrZaloBotActorIP
	}
	return a.Unmap().String(), nil
}

// NormalizeZaloBotName trims and bounds the operator-supplied display name.
func NormalizeZaloBotName(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", ErrZaloBotName
	}
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n == 0 || n > MaxZaloBotNameRunes {
		return "", ErrZaloBotName
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrZaloBotName
		}
	}
	return s, nil
}

// ValidateZaloChatURL checks the link staff open to reach the bot: absolute https, a host, no user info,
// at most 300 characters. Comms stores and shows it, never fetches it.
func ValidateZaloChatURL(s string) error {
	if s == "" || len(s) > MaxZaloChatURLLen || strings.TrimSpace(s) != s {
		return ErrZaloBotChatURL
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return ErrZaloBotChatURL
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == ' ' {
			return ErrZaloBotChatURL
		}
	}
	return nil
}

// SharedZaloBot is the zalo_bot_shared row as metadata — NEVER a sealed column (they are returned
// separately by the store, and only where a Zalo call needs the token).
type SharedZaloBot struct {
	BotAccountID string
	BotName      string
	ChatURL      string
	SetAt        time.Time
	SetBy        string
	// LastCheckAt / LastCheckResult: both zero, or both set (0018 CHECK zalo_bot_shared_check_pair).
	LastCheckAt     time.Time
	LastCheckResult string
	// WebhookSetAt / WebhookSetBy: the secret now IN FORCE; zero when none was ever made current.
	WebhookSetAt time.Time
	WebhookSetBy string
	// HasPendingWebhook: a secret is stored as pending (requested, not yet confirmed by Zalo).
	HasPendingWebhook bool
}

// ZaloBotCommuneStat is one commune's uptake of the shared bot — counts and a switch only.
type ZaloBotCommuneStat struct {
	TenantID         string
	ChannelEnabled   bool
	LinkedStaffCount int
}
