package domain

import "regexp"

// The sign-in settings of a commune's OWN Mini App (ADR 0066, migration 0021).

// miniAppIDShape is the shape migration 0021 enforces (mini_app_secret_app_id_shape): a Zalo App ID
// is digits. Checked here too so a malformed value is refused before any platform call — on a route
// anybody on the internet can reach, a junk value must cost nothing.
var miniAppIDShape = regexp.MustCompile(`^[0-9]{1,32}$`)

// ValidMiniAppID reports whether s has the shape of a Zalo App ID.
func ValidMiniAppID(s string) bool { return miniAppIDShape.MatchString(s) }

// THE `--demo` FIXED IDENTITY IS GONE (owner decision 05/10/2026, ADR 0066 §Sửa đổi). Its data is
// not: Zalo account rows keyed "demo:<App ID>" and the sessions and petitions opened under them stay
// as they are (rule 7). A real Zalo account id is digits only (internal/zalo readAccountID), so no new
// sign-in can ever land on one of those rows.
