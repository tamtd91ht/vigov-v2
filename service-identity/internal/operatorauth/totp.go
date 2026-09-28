package operatorauth

import (
	"crypto/hmac"
	"crypto/rand"
	// @security-exception: HMAC-SHA1 is the RFC 6238 TOTP default the owner fixed on 2026-09-28 (ADR 0048 #10) for authenticator-app compatibility; used only inside HMAC, whose security does not rest on SHA-1 collision resistance. Never used as a bare hash.
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// TOTP parameters, fixed by the owner on 2026-09-28 (ADR 0048 #10): RFC 6238 defaults, which is
// what every mainstream authenticator app assumes when the URI names nothing else.
//
// WHY HMAC-SHA1 IS ACCEPTABLE: the attacks on SHA-1 are collision attacks; HMAC's security rests
// on the compression function as a PRF, which collisions do not break. SHA-256 would buy nothing
// here and would silently fail in authenticators that ignore the `algorithm` parameter.
const (
	TOTPPeriod     = 30 * time.Second
	TOTPDigits     = 6
	TOTPSkewSteps  = 1  // ±1 step: tolerates 30 s of clock drift either way, no more
	TOTPSecretSize = 20 // bytes; RFC 4226 §4 recommends 160 bits for HMAC-SHA1
)

// base32NoPad is the encoding authenticator apps expect in the `secret` parameter.
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

var (
	// ErrInvalidLabel is an account label that is empty, contains a colon, or looks like an
	// e-mail address. The label is the operator CODE: an e-mail in a provisioning URI is personal
	// data in a URL and in a QR image (rule 3, forbidden #4).
	ErrInvalidLabel = errors.New("operatorauth: TOTP account label must be the operator code")

	// ErrInvalidIssuer is an empty issuer or one containing a colon (the label separator).
	ErrInvalidIssuer = errors.New("operatorauth: TOTP issuer must be non-empty and contain no colon")

	// ErrInvalidSecret is TOTP key material of the wrong size.
	ErrInvalidSecret = fmt.Errorf("operatorauth: TOTP secret must be %d bytes", TOTPSecretSize)
)

// GenerateSecret returns a fresh 160-bit TOTP secret from crypto/rand.
func GenerateSecret() (secret.Secret, error) {
	b := make([]byte, TOTPSecretSize)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("operatorauth: generate TOTP secret: %w", err)
	}
	return secret.Secret(b), nil
}

// EncodeSecret returns the base32 (no padding) form an operator types into an authenticator app
// by hand. It is still secret material, so it stays secret.Secret.
func EncodeSecret(s secret.Secret) secret.Secret {
	return secret.Secret(base32NoPad.EncodeToString(s.Lo()))
}

// ProvisioningURI builds the otpauth:// URI shown once, as a QR code, at enrolment.
//
// The result CONTAINS THE SECRET, so it is returned as secret.Secret: a string here would be one
// debugging line away from putting a working second factor into centralised logging.
//
// accountLabel is the operator's business code (e.g. `VH-00001`), never an e-mail address — see
// ErrInvalidLabel. The issuer is set both as the label prefix and as the `issuer` parameter, the
// form Google Authenticator documents. Callers pass "ViGov".
func ProvisioningURI(issuer, accountLabel string, s secret.Secret) (secret.Secret, error) {
	accountLabel = strings.TrimSpace(accountLabel)
	if accountLabel == "" || strings.ContainsAny(accountLabel, "@:") {
		return nil, ErrInvalidLabel
	}
	issuer = strings.TrimSpace(issuer)
	if issuer == "" || strings.Contains(issuer, ":") {
		return nil, ErrInvalidIssuer
	}
	if len(s) != TOTPSecretSize {
		return nil, ErrInvalidSecret
	}
	q := url.Values{}
	q.Set("secret", base32NoPad.EncodeToString(s.Lo()))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(TOTPDigits))
	q.Set("period", fmt.Sprint(int(TOTPPeriod/time.Second)))
	u := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + issuer + ":" + accountLabel,
		RawQuery: q.Encode(),
	}
	return secret.Secret(u.String()), nil
}

// TimeStep is the RFC 6238 counter for an instant: floor(unix seconds / 30).
func TimeStep(now time.Time) int64 {
	return now.Unix() / int64(TOTPPeriod/time.Second)
}

// Verify checks a 6-digit code against the secret at `now`, accepting the steps now-1, now and
// now+1. It returns the MATCHED STEP so the caller can refuse replay: a code whose step is <= the
// last step accepted for that account must be refused, or one code observed over a shoulder works
// for up to 90 seconds.
//
// Anything that is not exactly six ASCII digits is refused before any HMAC is computed. The three
// candidate codes are ALL computed and compared in constant time — no early exit — so the response
// time does not say which window matched.
func Verify(s secret.Secret, code string, now time.Time) (step int64, ok bool) {
	if len(s) != TOTPSecretSize || !sixDigits(code) {
		return 0, false
	}
	current := TimeStep(now)
	matched := 0
	var matchedStep int64
	for delta := int64(-TOTPSkewSteps); delta <= TOTPSkewSteps; delta++ {
		candidate := current + delta
		eq := subtle.ConstantTimeCompare([]byte(hotp(s.Lo(), uint64(candidate), TOTPDigits)), []byte(code))
		// Keep the LATEST matching step: if two windows ever produced the same code, recording
		// the later one is the conservative choice for the replay check.
		if eq == 1 {
			matchedStep = candidate
		}
		matched |= eq
	}
	if matched != 1 {
		return 0, false
	}
	return matchedStep, true
}

func sixDigits(code string) bool {
	if len(code) != TOTPDigits {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

// hotp is RFC 4226 §5.3: HMAC-SHA1 over the big-endian counter, dynamic truncation, modulo
// 10^digits, zero-padded.
func hotp(key []byte, counter uint64, digits int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, bin%mod)
}
