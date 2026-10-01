package zalo

import (
	"errors"
	"regexp"
	"strings"
)

// ===========================================================================
//  THE WIRE SHAPE OF THE CALLS TO ZALO — the only file in this repo that describes it.
//  Ported from vihat-miniapp internal/zalo/wire.go (ADR 0066); a protocol change goes into
//  BOTH copies.
//
//  EVIDENCE (as recorded in vihat-miniapp on 2026-09-20):
//  - Token exchange: MEDIUM — several consistent SECONDARY sources; the official docs page
//    renders in JS and could not be read verbatim.
//  - /me/info envelope: DIRECT — one run of vihat-miniapp's cmd/thu-zalo with a FAKE token
//    and FAKE secret got HTTP 200 and a JSON body of exactly the infoEnvelope shape, with
//    error=452 ("session key invalid"): access_token is checked FIRST, and 452 is a user-side
//    fault, so mapping it to 401 is right.
//  - NOT yet proven (that run never got past authentication): the header names "code" and
//    "secret_key", the field data.number, and whether appsecret_proof is required.
//  - The httptest tests in this package prove the code matches THIS FILE, not that this file
//    matches Zalo.
//
//  UNKNOWNS — to close before release:
//   1. appsecret_proof: since 2024-01-01 Zalo requires it when reading user info from a
//      server. UNKNOWN whether it applies to this Mini App flow, whether it goes as a header
//      or a query parameter, and which string is signed. Kept OFF by default — see
//      WithAppSecretProof.
//   2. Error codes: no trustworthy table. Every error != 0 is treated as "token bad/expired"
//      (-> 401). If some code actually means "this app's secret is wrong", that is OUR fault
//      and must become 502 + an operator alert. Observed so far: 452 on /me/info.
//   3. Number format: sources say "84xxxxxxxxx". "0xxxxxxxxx" and a leading "+" are accepted
//      by normalizePhone; anything else is refused rather than guessed.
//   4. Rate limits on these endpoints: unknown.
//   7. Account id: JSON string or JSON number — both accepted, never through float64.
//   8. Error codes of /v2.0/me: only -216 (from the reference implementation) is known.
// ===========================================================================

// DefaultBaseURL is the real server. Injectable through New for tests.
const DefaultBaseURL = "https://graph.zalo.me"

// InfoPath — GET, no body. Exchanges a token ("code") for user info, with the app's secret.
const InfoPath = "/v2.0/me/info"

// AccountIDPath — GET, no body, only ?fields=id on the URL (neither secret nor personal data).
//
//	{"id": "1234567890123456789", "error": 0, "message": "Success"}
//
// FLAT — no "data" envelope. On failure: HTTP 200 with "error" != 0.
// Source: the reference implementation in vigov-require (commit 0053854,
// apps/api/app/integrations/zalo/graph.py:113-134), chosen by the project owner 2026-09-29.
// EVIDENCE: LOW — never called against real Zalo from either repo.
const AccountIDPath = "/v2.0/me"

// Only "id" is requested. Asking for name/picture as well makes Zalo refuse the WHOLE call
// when the caller's IP is outside Vietnam (reference implementation), and this server makes no
// promise about where it runs.
const (
	fieldsParam    = "fields"
	accountIDField = "id"
)

// Header names are part of the protocol. HTTP header names are case-insensitive, so case does
// not matter; spelling does.
const (
	HeaderAccessToken    = "access_token"    // getAccessToken() on the Mini App
	HeaderCode           = "code"            // the phoneToken from getPhoneNumber()
	HeaderSecretKey      = "secret_key"      // the App ID's secret — SECRET
	HeaderAppSecretProof = "appsecret_proof" // UNKNOWN #1; not sent by default
)

// infoEnvelope is the observed /me/info envelope:
//
//	{"data": {"number": "84900000000"}, "error": 0, "message": "Success"}
//
// "message" is deliberately NOT decoded: it is free text this system does not control, and a
// field that is never read cannot leak into an error.
type infoEnvelope struct {
	Data  *phoneData `json:"data"`
	Error int        `json:"error"`
}

type phoneData struct {
	Number string `json:"number"`
}

// errPhoneMalformed — Zalo answered 200 with error=0 but the number is unusable. Phone wraps it
// in ErrZaloUnreachable; it is not a third class for callers.
var errPhoneMalformed = errors.New("phone number in response not in the expected format")

var digitsOnly = regexp.MustCompile(`^[0-9]{9,15}$`)

// normalizePhone brings the number Zalo returns to one stored form: digits only, starting with
// country code 84, no plus, no spaces. Same behaviour as vihat-miniapp's ChuanHoaSo.
//
// One form in one place is what makes a unique key on the phone meaningful: "0900000000" and
// "84900000000" stored as two rows are one person with two identities, and they cannot be
// merged later without losing data.
//
// Neither the argument nor the result is ever logged, and the error never carries the value.
func normalizePhone(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.TrimPrefix(s, "+")

	switch {
	case s == "":
		return "", errPhoneMalformed
	case strings.HasPrefix(s, "0"):
		s = "84" + strings.TrimPrefix(s, "0")
	}
	if !digitsOnly.MatchString(s) {
		return "", errPhoneMalformed
	}
	return s, nil
}
