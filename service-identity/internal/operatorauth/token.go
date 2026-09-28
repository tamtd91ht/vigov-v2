package operatorauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// TokenVersion prefixes every operator token. It is NOT core/token's `v1`: a verifier of either
// realm rejects the other's tokens at the first byte (ADR 0048 stop condition #6).
const TokenVersion = "op1"

// Realm is the only realm value an operator token may carry.
const Realm = "operator"

// MinSigningKeyLength mirrors core/token.KhoaToiThieu: 32 bytes is SHA-256's output size, and a
// shorter key only makes offline forgery cheaper.
const MinSigningKeyLength = 32

// macKeyLabel derives the operator MAC key from the configured key: HMAC-SHA256(key, label).
//
// DEFENCE IN DEPTH, NOT THE PRIMARY SEPARATION. The version prefix already separates the realms
// and core/config refuses an operator key equal to a staff key. The derivation means that even if
// both of those failed — a key pasted into both lists, a verifier that stopped checking the
// prefix — a MAC computed for one realm is still not a valid MAC in the other.
const macKeyLabel = "vigov/operatorauth/session-token/v1"

var (
	ErrKeyTooShort = fmt.Errorf("operatorauth: signing key shorter than %d bytes", MinSigningKeyLength)

	// ErrInvalidToken covers a malformed token, a wrong version, a wrong realm, an unexpected
	// field and a bad MAC — one error, so a caller cannot learn how far a forgery got.
	ErrInvalidToken = errors.New("operatorauth: invalid token")

	// ErrExpiredToken is separate only so the edge can log an ordinary expiry differently from a
	// forgery. The caller treats both the same: no principal.
	ErrExpiredToken = errors.New("operatorauth: token expired")

	ErrMissingClaim = errors.New("operatorauth: token needs a session id and an expiry")
)

// TokenClaims is the whole content of an operator token. There is NO tenant field — the operator
// realm has no commune — and there are no permissions: those are read from
// operator_permission_grant on every request, which is what makes a revocation take effect now.
type TokenClaims struct {
	// SessionID is checked against the operator session registry on every request (rule 5,
	// invariant 4): it is what makes the token revocable.
	SessionID string
	ExpiresAt time.Time
}

// tokenPayload is the wire form. Decoded with DisallowUnknownFields: a payload carrying `tid` or
// anything else is refused rather than silently ignored.
type tokenPayload struct {
	Realm string `json:"realm"`
	Sid   string `json:"sid"`
	Exp   int64  `json:"exp"` // Unix seconds, UTC
}

// TokenSigner signs with the FIRST key and verifies against EVERY key (rotation: prepend the new
// key, wait one session lifetime, drop the old one — the core/token procedure).
type TokenSigner struct {
	keys [][]byte // derived MAC keys, never the configured material
}

// NewTokenSigner validates config.Config.OperatorSessionSigningKeys once. No keys →
// ErrNotConfigured: operator sign-in is refused, the service keeps serving everything else. A
// short key is a MISCONFIGURATION, not an absence: the wiring should treat ErrKeyTooShort as fatal
// at startup.
func NewTokenSigner(keys []secret.Secret) (*TokenSigner, error) {
	if len(keys) == 0 {
		return nil, ErrNotConfigured
	}
	s := &TokenSigner{keys: make([][]byte, 0, len(keys))}
	for i, k := range keys {
		if len(k) < MinSigningKeyLength {
			return nil, fmt.Errorf("%w: key %d is %d bytes", ErrKeyTooShort, i+1, len(k))
		}
		m := hmac.New(sha256.New, k.Lo())
		m.Write([]byte(macKeyLabel))
		s.keys = append(s.keys, m.Sum(nil))
	}
	return s, nil
}

// Sign returns `op1.<b64url(payload)>.<b64url(HMAC-SHA256)>`. The MAC covers `op1.<payload>`, the
// version included, so a token cannot be relabelled.
func (s *TokenSigner) Sign(c TokenClaims) (string, error) {
	if s == nil || len(s.keys) == 0 {
		return "", ErrNotConfigured
	}
	if c.SessionID == "" || c.ExpiresAt.IsZero() {
		return "", ErrMissingClaim
	}
	raw, err := json.Marshal(tokenPayload{Realm: Realm, Sid: c.SessionID, Exp: c.ExpiresAt.UTC().Unix()})
	if err != nil {
		return "", fmt.Errorf("operatorauth: encode claims: %w", err)
	}
	body := TokenVersion + "." + base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + base64.RawURLEncoding.EncodeToString(mac(s.keys[0], body)), nil
}

// Verify checks prefix, MAC (constant time, against every key), realm and expiry at `now`.
//
// The MAC is checked BEFORE the payload is parsed: parsing unauthenticated attacker bytes is how a
// parser bug becomes a vulnerability.
func (s *TokenSigner) Verify(tok string, now time.Time) (TokenClaims, error) {
	if s == nil || len(s.keys) == 0 {
		return TokenClaims{}, ErrNotConfigured
	}
	if !strings.HasPrefix(tok, TokenVersion+".") {
		return TokenClaims{}, ErrInvalidToken
	}
	i := strings.LastIndexByte(tok, '.')
	if i <= len(TokenVersion) {
		return TokenClaims{}, ErrInvalidToken
	}
	body, sig := tok[:i], tok[i+1:]
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	valid := false
	for _, k := range s.keys {
		if hmac.Equal(got, mac(k, body)) {
			valid = true
		}
	}
	if !valid {
		return TokenClaims{}, ErrInvalidToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(body[len(TokenVersion)+1:])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p tokenPayload
	if err := dec.Decode(&p); err != nil || dec.More() {
		return TokenClaims{}, ErrInvalidToken
	}
	if p.Realm != Realm || p.Sid == "" || p.Exp == 0 {
		return TokenClaims{}, ErrInvalidToken
	}
	c := TokenClaims{SessionID: p.Sid, ExpiresAt: time.Unix(p.Exp, 0).UTC()}
	if !now.UTC().Before(c.ExpiresAt) {
		return TokenClaims{}, ErrExpiredToken
	}
	return c, nil
}

func mac(key []byte, body string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(body))
	return m.Sum(nil)
}

// String, GoString, Format and LogValue refuse to print the keys (see doc.go).
func (s TokenSigner) String() string {
	return fmt.Sprintf("operatorauth.TokenSigner(%d keys)", len(s.keys))
}

func (s TokenSigner) GoString() string { return s.String() }

func (s TokenSigner) Format(f fmt.State, verb rune) { redacted(f, verb, s.String()) }

func (s TokenSigner) LogValue() slog.Value { return redactedLog(s.String()) }
