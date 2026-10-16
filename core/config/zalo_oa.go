package config

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/vihat/vigov/core/secret"
)

// ZALO_OA_APP_ID · ZALO_OA_APP_SECRET · ZALO_OA_CALLBACK_HOST — the ONE Zalo App of the platform
// (ADR 0098 #2 and #3, owner 16/10/2026: "nó là 1 app dùng chung cho toàn bộ dự án không phải riêng
// cho từng xã").
//
// WHY ONE APP AND NOT ONE PER COMMUNE: a commune does not create a Zalo App; it authorises ITS OWN
// OA to this app through OAuth v4 + PKCE, and comms then calls Zalo's OA/ZBS APIs on the commune's
// behalf with the commune's own token (stored sealed per commune, ADR 0098 #5). 200+ apps would be
// 200+ secrets and 200+ callbacks that no operator can audit. So the app's id, secret and callback
// host are PLATFORM-WIDE CONSTANTS and belong here (rule 1 invariant 10) — the per-commune part
// (the OA, its tokens, its templates) never does.
//
// WHY A CALLBACK HOST OF ITS OWN: Zalo registers exactly ONE callback URL per app, in the app's
// settings. It therefore cannot sit on a commune's domain, and comms finds the commune from the
// request's `state` — never from Host (ADR 0098 #4). comms builds
// `https://<host>/api/v1/zalo-oa-authorizations` itself; no caller supplies a URL.
//
// WHY NOT REUSE ZALO_BOT_WEBHOOK_HOST even though a deployment may give both the same value
// (`bot.api.vigov.vn`): rule 11 invariant 3 — a name says the ROLE. The bot host is where Zalo posts
// the shared bot's updates; this one is where Zalo sends a commune's OA authorisation code. Binding
// the two to one name would make "move the OA callback" a change to the bot's webhook, and vice
// versa. Same SHAPE and same CHECK as the bot host (parsePlatformZaloHost, zalo_bot.go) — never a
// commune's web host — with its own error so a refusal names the variable that is wrong.
//
// WHO READS IT: service-comms only, through group ZaloOA. REQUIRED IN STAGING/PROD once declared
// (ADR 0057). In DEV, all three empty = the group is OFF: the accessors return "" / nil and comms
// refuses "Kết nối OA" by name (ADR 0098 #2).
//
// THE SECRET IS secret.Secret: it is the `secret_key` header of every token exchange and refresh —
// the key that opens the tokens of EVERY commune. It never reaches a log line, a startup message or
// a chat (rule 8); leaked, it is rotated at Zalo and every commune authorises again (ADR 0098 §Rủi ro).

var (
	// ErrZaloOAAppIDInvalid is a ZALO_OA_APP_ID that is not a plain number. Load refuses.
	ErrZaloOAAppIDInvalid = errors.New("config: ZALO_OA_APP_ID is invalid")
	// ErrZaloOACallbackHostInvalid is a malformed or commune-shaped ZALO_OA_CALLBACK_HOST. Load refuses.
	ErrZaloOACallbackHostInvalid = errors.New("config: ZALO_OA_CALLBACK_HOST is invalid")
)

// zaloOAAppIDShape: Zalo's App ID is a `long` printed in decimal (ADR 0098 §Kết quả tra tài liệu, the
// `app_id` field of every OAuth call). 32 digits is far beyond any int64 — the bound exists so a
// pasted token or URL is refused as "not an app id" rather than accepted as a very long one.
var zaloOAAppIDShape = regexp.MustCompile(`^[0-9]{1,32}$`)

// parseZaloOAAppID validates the value r.read returned (trimmed). "" is returned as "" — the
// requirement (staging/prod) was already applied by r.read. The value is quoted in the refusal: an
// app id is not a credential (it travels in the authorisation link every OA admin opens).
func parseZaloOAAppID(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if !zaloOAAppIDShape.MatchString(v) {
		return "", fmt.Errorf("%w: %q must be the app's numeric id (1–32 digits) as shown on developers.zalo.me — "+
			"not its name, not its secret", ErrZaloOAAppIDInvalid, v)
	}
	return v, nil
}

// parseZaloOAAppSecret wraps the trimmed value. No shape is asserted: Zalo documents no format for
// the secret key, and a rule invented here would refuse a real key the day Zalo changes it. "" = nil.
func parseZaloOAAppSecret(v string) secret.Secret {
	if v == "" {
		return nil
	}
	return secret.Secret(v)
}

// parseZaloOACallbackHost is parsePlatformZaloHost with this variable's error: the bot host's shape,
// the bot host's commune-host refusal, nothing copied (ADR 0098 #3).
func parseZaloOACallbackHost(h string) (string, error) {
	return parsePlatformZaloHost(h, ErrZaloOACallbackHostInvalid, "the OA authorisation callback")
}
