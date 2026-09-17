// Package token signs and verifies the session token carried in the browser cookie.
//
// WHY HMAC-SHA256 AND THE STANDARD LIBRARY, NOT A JWT LIBRARY: the claim set is three fields
// and there is exactly one issuer and one verifier — this deployment. A JWT library brings an
// algorithm negotiation field (`alg`), which is the single most exploited part of JWT, plus a
// dependency to keep patched, in exchange for interoperability nobody here needs. The token
// format below has no algorithm field to confuse: a token is verified with the only algorithm
// this package knows, or it is rejected.
//
// WHAT IS DELIBERATELY NOT IN THE CLAIMS: roles and permissions. Embedding them means a
// privilege change takes effect only when the token expires — a staff member removed from a
// role keeps that role for up to 12 hours (skills/session-and-token, FORBIDDEN). Permissions
// are read from the database on every request instead, which is also what makes revocation
// work at all.
//
// KNOWN GAP — skills/session-and-token required #4 is NOT met yet: there is no rotating
// refresh token here. The access token simply lives for the session's own lifetime
// (idstore.ThoiHanPhien, 12h) and is revocable through the session registry. Building half of
// a rotation scheme — issuing a refresh token without the reuse-detection chain that revokes
// on replay — would be worse than not building it, because it looks like protection and is
// not. This is a deliberate, stated gap, not an oversight.
package token

import (
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
	"github.com/vihat/vigov/core/tenant"
)

// Version prefixes every token. It exists so a future format can be told apart from this one
// instead of failing as "corrupt", and so two formats can be accepted side by side during a
// migration.
const Version = "v1"

// KhoaToiThieu is the minimum length of a signing key.
//
// 32 bytes is the output size of SHA-256: a shorter key adds no security and a short key is
// what makes an offline forgery attempt feasible. Refusing it at startup is the only moment
// anybody would notice.
const KhoaToiThieu = 32

var (
	// ErrKhongCoKhoa means the signer was built with no key at all. A service with no signing
	// key cannot tell a real token from a forged one, so it must not start.
	ErrKhongCoKhoa = errors.New("token: chưa cấu hình khoá ký")

	ErrKhoaQuaNgan = fmt.Errorf("token: khoá ký ngắn hơn %d byte", KhoaToiThieu)

	// ErrKhongHopLe covers a malformed token, a bad signature and an unknown version. They are
	// ONE error on purpose: telling a caller which part failed tells an attacker how far their
	// forgery got.
	ErrKhongHopLe = errors.New("token: không hợp lệ")

	// ErrHetHan is separate from ErrKhongHopLe only so the edge can log the difference — an
	// expired token is an ordinary daily event, a bad signature is not. The caller treats both
	// the same way: no principal.
	ErrHetHan = errors.New("token: đã hết hạn")

	ErrThieuClaim = errors.New("token: thiếu tenant_id hoặc sid")
)

// Claims is the entire content of a token. Three fields, and there is no fourth.
type Claims struct {
	// TenantID is compared against the commune resolved from Host on EVERY request. A token
	// without it could be replayed against any commune on the platform.
	TenantID tenant.ID

	// Sid is the session id, checked against the session registry on every request. It is what
	// makes a token revocable (skills/session-and-token, required #3).
	Sid string

	// ExpiresAt bounds the damage of a stolen token. A token with no expiry is forbidden.
	ExpiresAt time.Time
}

// payload is the wire form. Short keys because this string travels in a cookie on every
// request; the names are internal to this package and never appear in an API.
type payload struct {
	Tid string `json:"tid"`
	Sid string `json:"sid"`
	Exp int64  `json:"exp"` // Unix seconds, UTC
}

// Signer signs with the FIRST key and verifies against EVERY key.
//
// WHY A LIST AND NOT ONE KEY: rule 8, invariant 6 requires signing keys to have a documented
// lifetime and a rotation procedure. With a single key, rotating it invalidates every token
// issued under the old one — that is every signed-in member of staff across 200+ communes
// logged out simultaneously, which in practice means the key never gets rotated. With a list,
// rotation is: prepend the new key, wait out one session lifetime, drop the old one. Nobody
// is logged out and the old key stops being accepted on a schedule somebody chose.
type Signer struct {
	khoa [][]byte
}

// NewSigner validates the key material once, at startup.
//
// It fails rather than defaulting or generating a key: a process that silently invents its own
// signing key issues tokens no other replica can verify, and every restart logs everybody out
// for reasons nobody can see from the outside.
//
// IT TAKES []secret.Secret AND NOT [][]byte, and that is the whole point of the parameter type.
// With bare byte slices the material travelled unprotected from pkg/config to here, so
// `log.Info("khoá", "k", cfg.KhoaKyBytes())` printed every signing key on the deployment as a
// list of numbers — a leak on a path that never touched this package at all. The bytes are
// unwrapped ONE line below, at the only place that needs them.
//
// pkg/token imports pkg/secret and NEVER pkg/config: the protection belongs to the material,
// not to whoever happened to read it from the environment.
func NewSigner(khoa []secret.Secret) (*Signer, error) {
	if len(khoa) == 0 {
		return nil, ErrKhongCoKhoa
	}
	sao := make([][]byte, 0, len(khoa))
	for i, k := range khoa {
		if len(k) < KhoaToiThieu {
			return nil, fmt.Errorf("%w: khoá thứ %d dài %d byte", ErrKhoaQuaNgan, i+1, len(k))
		}
		// Copied so a caller mutating its slice later cannot change what this signer trusts.
		b := make([]byte, len(k))
		copy(b, k.Lo())
		sao = append(sao, b)
	}
	return &Signer{khoa: sao}, nil
}

// SoKhoa reports how many keys are accepted, for a startup log line. The keys themselves are
// never exposed and never logged (rule 8).
func (s *Signer) SoKhoa() int { return len(s.khoa) }

// String, GoString and LogValue REFUSE TO PRINT THE KEYS. Together they close every route by
// which fmt or slog reaches an unexported field through reflection: the verbs v, +v and s go
// through String, the verb #v goes through GoString, and slog attributes go through LogValue.
//
// WHY THIS IS NOT PARANOIA: without them, one debugging line prints the WHOLE PLATFORM'S session
// signing keys as raw bytes into centralised logging, into backups and into a third-party
// monitoring vendor — from where a secret cannot be recalled (rule 8, invariant 1). One leaked
// signing key forges a session in EVERY commune, not one.
//
// pkg/config guards the key on its way in and KhoaKyBytes() now hands over []secret.Secret, so
// the material arrives protected — but it stops being protected the moment NewSigner copies it
// into this struct's unexported field, and a struct field is exactly what fmt and slog reach by
// reflection. The protection has to be re-established here or it ends at this type.
//
// THE RECEIVER IS A VALUE, NOT A POINTER, ON PURPOSE: a value receiver puts these methods in the
// method set of BOTH Signer and *Signer, so a dereferenced copy is covered too.
func (s Signer) String() string { return fmt.Sprintf("token.Signer(%d khoá)", len(s.khoa)) }

func (s Signer) GoString() string { return s.String() }

func (s Signer) LogValue() slog.Value { return slog.StringValue(s.String()) }

// Ky signs the claims and returns the cookie value.
//
// Shape: "v1.<base64url(payload)>.<base64url(hmac)>", where the MAC covers "v1.<payload>" —
// the version included, so a token cannot be re-labelled as another version and replayed.
func (s *Signer) Ky(c Claims) (string, error) {
	if c.TenantID == "" || c.Sid == "" {
		return "", ErrThieuClaim
	}
	if c.ExpiresAt.IsZero() {
		// A token with no expiry is forbidden (skills/session-and-token, FORBIDDEN). Refusing
		// here is cheaper than discovering a never-expiring token in a browser six months on.
		return "", fmt.Errorf("token: thiếu hạn dùng: %w", ErrThieuClaim)
	}

	tho, err := json.Marshal(payload{
		Tid: string(c.TenantID),
		Sid: c.Sid,
		Exp: c.ExpiresAt.UTC().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("token: đóng gói claim: %w", err)
	}

	than := Version + "." + base64.RawURLEncoding.EncodeToString(tho)
	return than + "." + base64.RawURLEncoding.EncodeToString(s.mac(s.khoa[0], than)), nil
}

// Giai verifies the signature and the expiry, then returns the claims.
//
// The signature is checked BEFORE the payload is parsed: parsing attacker-controlled bytes
// that have not been authenticated is how a parser bug becomes a vulnerability.
func (s *Signer) Giai(tok string) (Claims, error) {
	i := strings.LastIndexByte(tok, '.')
	if i <= 0 {
		return Claims{}, ErrKhongHopLe
	}
	than, chuKy := tok[:i], tok[i+1:]

	if !strings.HasPrefix(than, Version+".") {
		return Claims{}, ErrKhongHopLe
	}

	nhan, err := base64.RawURLEncoding.DecodeString(chuKy)
	if err != nil {
		return Claims{}, ErrKhongHopLe
	}

	// hmac.Equal, never ==. String comparison returns as soon as two bytes differ, and that
	// timing difference is enough to reconstruct a valid signature byte by byte.
	hopLe := false
	for _, k := range s.khoa {
		if hmac.Equal(nhan, s.mac(k, than)) {
			hopLe = true
			break
		}
	}
	if !hopLe {
		return Claims{}, ErrKhongHopLe
	}

	tho, err := base64.RawURLEncoding.DecodeString(than[len(Version)+1:])
	if err != nil {
		return Claims{}, ErrKhongHopLe
	}
	var p payload
	if err := json.Unmarshal(tho, &p); err != nil {
		return Claims{}, ErrKhongHopLe
	}
	if p.Tid == "" || p.Sid == "" || p.Exp == 0 {
		return Claims{}, ErrKhongHopLe
	}

	c := Claims{
		TenantID:  tenant.ID(p.Tid),
		Sid:       p.Sid,
		ExpiresAt: time.Unix(p.Exp, 0).UTC(),
	}
	if !time.Now().UTC().Before(c.ExpiresAt) {
		return Claims{}, ErrHetHan
	}
	return c, nil
}

func (s *Signer) mac(khoa []byte, than string) []byte {
	m := hmac.New(sha256.New, khoa)
	m.Write([]byte(than))
	return m.Sum(nil)
}
