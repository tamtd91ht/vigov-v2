package domain

import (
	"errors"
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

// ErrLaunchHostInvalid — the commune host is not a bare LDH host name, so the citizen app's own
// check (laTenMien) would drop it and the QR would open the app with no commune suggestion.
var ErrLaunchHostInvalid = errors.New("mini_app: tên miền xã không dùng được trong liên kết mở app")

// MiniAppLaunchLink builds the shared app's QR link for one commune host:
//
//	https://zalo.me/s/<shared app id>/?d=<commune primary host>&src=qr
//
// `d` and `src` are the two launch parameters ADR 0047 decided (27/09/2026). They STEER THE
// INTERFACE AND GRANT NOTHING: the app asks the server whether the host is a commune's, and enters
// a session only after the citizen confirms (ADR 0047 stop condition #4).
//
// The host is the registry's stored value (lower-case LDH, ParseCommuneHost); it is checked again
// rather than trusted, because a host the app's parser refuses yields a QR that silently does
// nothing. url.Values encodes with sorted keys, so the query is exactly `d=…&src=qr`.
func MiniAppLaunchLink(appID, host string) (string, error) {
	if err := ValidateMiniAppID(appID); err != nil {
		return "", err
	}
	if h, err := ParseCommuneHost(host, ""); err != nil || h != host {
		return "", ErrLaunchHostInvalid
	}
	q := url.Values{}
	q.Set("d", host)
	q.Set("src", LaunchSourceQR)
	return zaloMiniAppLinkBase + appID + "/?" + q.Encode(), nil
}
