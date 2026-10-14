package domain

import (
	"net/url"
	"time"
)

// SharedMiniApp is the platform's ONE running shared Mini App (ADR 0044 "app chung", ADR 0047): the
// single active, not-deleted `mini_app` row with che_do = 'chinh' (migration 0006). The operator
// declares it (owner 04/10/2026); the commune QR links are built from it.
//
// No commune: a main app carrying a commune would be a default commune on the isolation path
// (migration 0006's CHECK, rule 1 forbidden #1).
type SharedMiniApp struct {
	AppID     string
	CreatedAt time.Time
	CreatedBy string // the operator's VH- code, or whoever entered the row
}

// zaloMiniAppLinkBase is Zalo's public link to a PUBLISHED Mini App. The form
// `https://zalo.me/s/<APP_ID>/?k=v` is the one ADR 0018 #2 confirmed and ADR 0005 / 0048 cite; the
// TESTING-build form (`?env=TESTING&version=<n>`, citizen-app/README.md) is deliberately NOT built
// here — owner 04/10/2026: a printed QR uses the published-app link only, because a test build
// number changes with every upload and a QR lives for years (ADR 0019).
const zaloMiniAppLinkBase = "https://zalo.me/s/"

// LaunchSourceQR is the `src` value that marks a link as printed on a QR. citizen-app reads it
// against NGUON_CHON_SAN (citizen-app/src/lib/launch-params.ts); any other value makes the app open
// as if it had no parameter.
const LaunchSourceQR = "qr"

// LaunchProfileParam is the query parameter carrying the runtime profile code (ADR 0046 §Sửa đổi
// 14/10/2026 #9). citizen-app reads it as PROFILE_PARAM (citizen-app/src/lib/runtime-profile.ts): a code
// whose SHA-256 matches the bundle's digest opens that launch against the DEV hosts; no `pf` = prod. The
// name never says "dev", and it is not Zalo's own `env` (which picks a TESTING build).
const LaunchProfileParam = "pf"

// MiniAppLaunchLink builds the shared app's QR link for one commune, by its LAUNCH CODE:
//
//	https://zalo.me/s/<shared app id>/?src=qr&x=<commune launch code>                  profileCode ""
//	https://zalo.me/s/<shared app id>/?pf=<profile code>&src=qr&x=<commune launch code> otherwise
//
// `x` replaced `d=<domain>` on the owner's decision of 12/10/2026 (ADR 0047 §Sửa đổi 12/10/2026): the
// code is opaque, fixed per commune and names no domain. Like `d` before it, `x` and `src` STEER THE
// INTERFACE AND GRANT NOTHING: the app asks the server which commune holds the code (ResolveHost), and
// enters a session only after the citizen confirms (ADR 0047 stop condition #4). `pf` only picks which
// backend that launch talks to; it grants nothing either.
//
// profileCode is MINI_APP_PROFILE_CODE as core/config validated it — set only in the dev cluster (config
// refuses it in staging/prod). "" adds no `pf`, which is the prod link.
//
// The launch code is the registry's stored value; it is checked again rather than trusted, because a code
// the app's parser refuses (isLaunchCode) yields a QR that silently opens no commune. url.Values encodes
// with SORTED keys, so the query is exactly `src=qr&x=…`, or `pf=…&src=qr&x=…` — not the CLI's order
// (`x`, `src`, `pf`, citizen-app/scripts/commune-qr.mjs launchCodeLink); the app reads parameters by
// name, so the order changes nothing.
func MiniAppLaunchLink(appID, launchCode, profileCode string) (string, error) {
	if err := ValidateMiniAppID(appID); err != nil {
		return "", err
	}
	if !ValidLaunchCode(launchCode) {
		return "", ErrLaunchCodeInvalid
	}
	q := url.Values{}
	q.Set("x", launchCode)
	q.Set("src", LaunchSourceQR)
	setLaunchProfile(q, profileCode)
	return zaloMiniAppLinkBase + appID + "/?" + q.Encode(), nil
}

// OwnMiniAppLaunchLink builds the QR link of a commune's OWN (che_do rieng) Mini App (ADR 0070
// §Sửa đổi 05/10/2026 #4):
//
//	https://zalo.me/s/<the commune's App ID>/?src=qr                    profileCode ""
//	https://zalo.me/s/<the commune's App ID>/?pf=<profile code>&src=qr  otherwise
//
// NO `d=`: the own app has its commune baked in at build time (ADR 0047 #6), and the server resolves
// the commune from the App ID, not from a parameter. A `d` here would be a second, client-carried
// statement of the commune that could only disagree with the first. `pf` as in MiniAppLaunchLink: the
// own app is the same citizen-app bundle and reads it the same way.
func OwnMiniAppLaunchLink(appID, profileCode string) (string, error) {
	if err := ValidateMiniAppID(appID); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("src", LaunchSourceQR)
	setLaunchProfile(q, profileCode)
	return zaloMiniAppLinkBase + appID + "/?" + q.Encode(), nil
}

// setLaunchProfile adds `pf` when a profile code is configured. The shape is core/config's to check (one
// owner per rule); url.Values escapes whatever it is given, so a value can never break out of its parameter.
func setLaunchProfile(q url.Values, profileCode string) {
	if profileCode != "" {
		q.Set(LaunchProfileParam, profileCode)
	}
}
