package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ZALO_BOT_WEBHOOK_HOST — the host Zalo posts the shared bot's updates to (ADR 0074 #5 and its
// frontmatter, owner 05/10/2026: "host webhook Zalo Bot là biến toàn nền tảng ZALO_BOT_WEBHOOK_HOST,
// chỉ comms nạp, bắt buộc ở prod"). `bot.api.vigov.vn` in production.
//
// WHO READS IT: service-comms only, through group ZaloBotWebhook. Comms builds the webhook URL
// `https://<host>/api/v1/zalo-bot-updates` itself when it calls Zalo's setWebhook — no caller supplies
// a URL (proto/vigov/comms/v1/zalo_bot_operator.proto, SetSharedZaloBotWebhook), so a holder of the
// shared caller key cannot point every staff update at itself.
//
// REQUIRED IN STAGING/PROD for the declaring service (ADR 0057): the operator's "Trỏ webhook" button
// has nothing to point at without it. In DEV, empty means that button is refused by name.
//
// A PLATFORM-WIDE CONSTANT, never a commune's value (rule 1 invariant 10): one shared bot, one
// webhook host for the whole deployment.

// ErrZaloBotWebhookHostInvalid is a malformed or commune-shaped ZALO_BOT_WEBHOOK_HOST. Load refuses.
var ErrZaloBotWebhookHostInvalid = errors.New("config: ZALO_BOT_WEBHOOK_HOST is invalid")

// parseZaloBotWebhookHost validates the value r.read returned (trimmed). "" is returned as "" — the
// requirement (staging/prod) was already applied by r.read.
//
// THE SHAPE OF OPERATOR_HOST (checkBareHost), and the same "refused, never repaired" rule. One extra
// refusal for the same reason OPERATOR_HOST has its own: a host directly under a platform web root
// (`<xa>.vigov.vn`, `<xa>.dev.vigov.vn`) is a commune's web host or a candidate for one (ADR 0046).
// Pointing the shared bot there would hand every commune's staff updates to one commune's surface. The
// root itself is refused too. Hosts deeper than one label (`bot.api.vigov.vn`) and hosts outside the
// platform's domains are not judged further: there is no rule in this repository for them.
func parseZaloBotWebhookHost(h string) (string, error) {
	return parsePlatformZaloHost(h, ErrZaloBotWebhookHostInvalid, "the shared bot's webhook")
}

// parsePlatformZaloHost is the ONE check behind every host Zalo calls the platform on —
// ZALO_BOT_WEBHOOK_HOST and ZALO_OA_CALLBACK_HOST (ADR 0098 #3: "cùng hình dạng và cùng phép kiểm").
// Two variables share it instead of each carrying a copy because the commune-host refusal is the
// security property: a copy that drifts is a host Zalo can be pointed at that one of the two never
// refuses. errInvalid names the variable; what names the thing being pointed, for the message.
func parsePlatformZaloHost(h string, errInvalid error, what string) (string, error) {
	if h == "" {
		return "", nil
	}
	if err := checkBareHost(h, errInvalid); err != nil {
		return "", err
	}
	if slices.Contains(platformWebRoots, h) {
		return "", fmt.Errorf("%w: %q is the platform's root domain", errInvalid, h)
	}
	for _, root := range platformWebRoots {
		if under, ok := strings.CutSuffix(h, "."+root); ok {
			if !strings.Contains(under, ".") {
				return "", fmt.Errorf("%w: %q is shaped like a commune's web host under %s — %s "+
					"must never sit on a commune's host; use bot.api.%s", errInvalid, h, root, what, root)
			}
			break
		}
	}
	return h, nil
}
