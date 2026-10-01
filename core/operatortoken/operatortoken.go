// Package operatortoken signs and verifies the ViHAT OPERATOR realm token, `op1.` (ADR 0048 §"Chốt
// bước 1", row "Token và bí mật").
//
// WHY IT EXISTS IN core/: the format had one implementation, service-identity/internal/operatorauth,
// and that path is `internal/` — service-platform cannot import it (rule 2, forbidden #1). The owner's
// decision of 2026-10-01 (#2) makes platform check the signature LOCALLY before it asks identity, so
// a second reader of the format was unavoidable. It lives here so there is ONE place a second reader
// can come from; a copy inside service-platform would be a second format that drifts the day one
// side rotates its MAC derivation.
//
// THE TWO COPIES ARE STILL TWO TODAY, and that is stated rather than hidden: identity issues tokens
// with its own operatorauth.TokenSigner. TestGoldenToken pins the exact bytes this package produces
// for a fixed key and claim set; the same vector was checked against identity's signer when this
// package was written. The follow-up that removes the duplicate is identity delegating to this
// package — a change inside service-identity, owned there.
//
// WHAT IS DELIBERATELY NOT HERE: anything that decides whether a session is LIVE. A valid signature
// says the token was issued by a holder of the key and has not passed its absolute expiry; it says
// nothing about revocation or the 5-minute idle limit. Only identity's ResolveOperatorSession decides
// those (operator.proto). A caller that treats Verify as "signed in" has rebuilt the hole the hybrid
// check exists to close.
//
// STANDARD LIBRARY ONLY: HMAC-SHA256, base64url, encoding/json.
package operatortoken

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Version prefixes every operator token. It is NOT core/token's `v1`: a verifier of either realm
// rejects the other's tokens at the first byte (ADR 0048 stop condition #6).
const Version = "op1"

// Realm is the only realm value an operator token may carry.
const Realm = "operator"

// MinKeyLength mirrors core/token.KhoaToiThieu: 32 bytes is SHA-256's output size, and a shorter key
// only makes offline forgery cheaper.
const MinKeyLength = 32

// macKeyLabel derives the MAC key from the configured key: HMAC-SHA256(key, label). It MUST equal
// service-identity/internal/operatorauth's label byte for byte, or every token identity issues fails
// here — TestGoldenToken is what turns that red.
const macKeyLabel = "vigov/operatorauth/session-token/v1"

var (
	// ErrNotConfigured: no key material. The caller refuses (fail closed) and never falls back.
	ErrNotConfigured = errors.New("operatortoken: operator realm is not configured")

	ErrKeyTooShort = fmt.Errorf("operatortoken: signing key shorter than %d bytes", MinKeyLength)

	// ErrInvalid covers a malformed token, a wrong version, a wrong realm, an unexpected field and a
	// bad MAC — one error, so a caller cannot learn how far a forgery got.
	ErrInvalid = errors.New("operatortoken: invalid token")

	// ErrExpired is separate only so an edge can log an ordinary expiry differently from a forgery.
	// The caller treats both the same: no principal.
	ErrExpired = errors.New("operatortoken: token expired")

	ErrMissingClaim = errors.New("operatortoken: token needs a session id and an expiry")
)

// Claims is the whole content of an operator token. There is NO tenant field — the operator realm has
// no commune — and no permissions: those are read live on every request.
type Claims struct {
	SessionID string
	ExpiresAt time.Time
}

// payload is the wire form. Decoded with DisallowUnknownFields: a payload carrying `tid` or anything
// else is refused rather than silently ignored.
type payload struct {
	Realm string `json:"realm"`
	Sid   string `json:"sid"`
	Exp   int64  `json:"exp"` // Unix seconds, UTC
}

// Signer signs with the FIRST key and verifies against EVERY key (rotation: prepend the new key, wait
// one session lifetime, drop the old one — the core/token procedure).
//
// service-platform only VERIFIES. Sign exists because a verifier whose tests cannot produce a valid
// token is a verifier tested only on garbage, and because the identity side is meant to move here.
type Signer struct {
	keys [][]byte // derived MAC keys, never the configured material
}

// NewSigner validates config.Config.OperatorSessionSigningKeys once. No keys → ErrNotConfigured. A
// short key is a MISCONFIGURATION, not an absence: wiring treats ErrKeyTooShort as fatal at startup.
func NewSigner(keys []secret.Secret) (*Signer, error) {
	if len(keys) == 0 {
		return nil, ErrNotConfigured
	}
	s := &Signer{keys: make([][]byte, 0, len(keys))}
	for i, k := range keys {
		if len(k) < MinKeyLength {
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
func (s *Signer) Sign(c Claims) (string, error) {
	if s == nil || len(s.keys) == 0 {
		return "", ErrNotConfigured
	}
	if c.SessionID == "" || c.ExpiresAt.IsZero() {
		return "", ErrMissingClaim
	}
	raw, err := json.Marshal(payload{Realm: Realm, Sid: c.SessionID, Exp: c.ExpiresAt.UTC().Unix()})
	if err != nil {
		return "", fmt.Errorf("operatortoken: encode claims: %w", err)
	}
	body := Version + "." + base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + base64.RawURLEncoding.EncodeToString(mac(s.keys[0], body)), nil
}

// Verify checks prefix, MAC (constant time, against every key), realm and expiry at `now`.
//
// The MAC is checked BEFORE the payload is parsed: parsing unauthenticated attacker bytes is how a
// parser bug becomes a vulnerability.
func (s *Signer) Verify(tok string, now time.Time) (Claims, error) {
	if s == nil || len(s.keys) == 0 {
		return Claims{}, ErrNotConfigured
	}
	if !strings.HasPrefix(tok, Version+".") {
		return Claims{}, ErrInvalid
	}
	i := strings.LastIndexByte(tok, '.')
	if i <= len(Version) {
		return Claims{}, ErrInvalid
	}
	body, sig := tok[:i], tok[i+1:]
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return Claims{}, ErrInvalid
	}
	valid := false
	for _, k := range s.keys {
		if hmac.Equal(got, mac(k, body)) {
			valid = true
		}
	}
	if !valid {
		return Claims{}, ErrInvalid
	}

	raw, err := base64.RawURLEncoding.DecodeString(body[len(Version)+1:])
	if err != nil {
		return Claims{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p payload
	if err := dec.Decode(&p); err != nil || dec.More() {
		return Claims{}, ErrInvalid
	}
	if p.Realm != Realm || p.Sid == "" || p.Exp == 0 {
		return Claims{}, ErrInvalid
	}
	c := Claims{SessionID: p.Sid, ExpiresAt: time.Unix(p.Exp, 0).UTC()}
	if !now.UTC().Before(c.ExpiresAt) {
		return Claims{}, ErrExpired
	}
	return c, nil
}

func mac(key []byte, body string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(body))
	return m.Sum(nil)
}

// String, GoString, Format and LogValue refuse to print the keys. Format is load-bearing: fmt consults
// String only for the string-shaped verbs, so without it %d prints the unexported [][]byte field one
// byte at a time — every derived MAC key of the deployment in a log line.
func (s Signer) String() string { return fmt.Sprintf("operatortoken.Signer(%d keys)", len(s.keys)) }

func (s Signer) GoString() string { return s.String() }

func (s Signer) Format(f fmt.State, verb rune) {
	if verb == 'q' {
		_, _ = io.WriteString(f, strconv.Quote(s.String()))
		return
	}
	_, _ = io.WriteString(f, s.String())
}

func (s Signer) LogValue() slog.Value { return slog.StringValue(s.String()) }
