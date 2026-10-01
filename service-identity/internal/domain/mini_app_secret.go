package domain

import "regexp"

// The sign-in settings of a commune's OWN Mini App (ADR 0066, migration 0021).

// miniAppIDShape is the shape migration 0021 enforces (mini_app_secret_app_id_shape): a Zalo App ID
// is digits. Checked here too so a malformed value is refused before any platform call — on a route
// anybody on the internet can reach, a junk value must cost nothing.
var miniAppIDShape = regexp.MustCompile(`^[0-9]{1,32}$`)

// ValidMiniAppID reports whether s has the shape of a Zalo App ID.
func ValidMiniAppID(s string) bool { return miniAppIDShape.MatchString(s) }

// DemoIdentityPhone is the fixed identity a dedicated app running `--demo` signs in as (ADR 0066:
// "danh tính cố định"). It is the repository's agreed fake number 0900000000 (rule 3, invariant 5)
// in the stored 84… form Zalo's exchange produces (internal/zalo normalizePhone), so a demo session
// and a test fixture name the same identity and never a real person.
const DemoIdentityPhone = "84900000000"

// demoAccountPrefix cannot collide with a real Zalo account id: those are digits only
// (internal/zalo readAccountID).
const demoAccountPrefix = "demo:"

// DemoZaloAccountID is the stand-in Zalo account id of an App ID's demo identity. The session
// bridge (app.CauPhienCongDan.Mo) keys every session on a Zalo account; `--demo` calls no Zalo API
// at all, so there is no real one. Deterministic PER APP ID: every demo sign-in of one app reuses
// the one account row, instead of creating a row per tap.
func DemoZaloAccountID(appID string) string { return demoAccountPrefix + appID }
