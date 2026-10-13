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

// MiniAppLaunchLink builds the shared app's QR link for one commune, by its LAUNCH CODE:
//
//	https://zalo.me/s/<shared app id>/?src=qr&x=<commune launch code>
//
// `x` replaced `d=<domain>` on the owner's decision of 12/10/2026 (ADR 0047 §Sửa đổi 12/10/2026): the
// code is opaque, fixed per commune and names no domain. Like `d` before it, `x` and `src` STEER THE
// INTERFACE AND GRANT NOTHING: the app asks the server which commune holds the code (ResolveHost), and
// enters a session only after the citizen confirms (ADR 0047 stop condition #4).
//
// The code is the registry's stored value; it is checked again rather than trusted, because a code the
// app's parser refuses (isLaunchCode) yields a QR that silently opens no commune. url.Values encodes
// with sorted keys, so the query is exactly `src=qr&x=…`.
func MiniAppLaunchLink(appID, launchCode string) (string, error) {
	if err := ValidateMiniAppID(appID); err != nil {
		return "", err
	}
	if !ValidLaunchCode(launchCode) {
		return "", ErrLaunchCodeInvalid
	}
	q := url.Values{}
	q.Set("x", launchCode)
	q.Set("src", LaunchSourceQR)
	return zaloMiniAppLinkBase + appID + "/?" + q.Encode(), nil
}

// OwnMiniAppLaunchLink builds the QR link of a commune's OWN (che_do rieng) Mini App (ADR 0070
// §Sửa đổi 05/10/2026 #4):
//
//	https://zalo.me/s/<the commune's App ID>/?src=qr
//
// NO `d=`: the own app has its commune baked in at build time (ADR 0047 #6), and the server resolves
// the commune from the App ID, not from a parameter. A `d` here would be a second, client-carried
// statement of the commune that could only disagree with the first.
func OwnMiniAppLaunchLink(appID string) (string, error) {
	if err := ValidateMiniAppID(appID); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("src", LaunchSourceQR)
	return zaloMiniAppLinkBase + appID + "/?" + q.Encode(), nil
}
