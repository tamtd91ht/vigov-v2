package config

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/vihat/vigov/core/secret"
)

// Operator realm configuration (ADR 0048, owner's decisions of 2026-09-28 #2 and #10, 2026-10-01 #2).
//
// TWO GROUPS, each declared by exactly one service (identity: OperatorRealm; platform, since
// 2026-10-01: OperatorEdge). config.OperatorRealm (both keys) is for the service that
// ISSUES operator sessions — identity. config.OperatorEdge (OPERATOR_HOST + the signing keys) is
// for the operator area's HTTP edge — platform, which checks the `op1.` signature locally before it
// asks identity (§01/10 #2) and has no use for the TOTP key. Once a service declares its group,
// staging and prod refuse to start it without the keys; a deployment with no operator area (the
// on-premise case of ADR 0048 condition #1) leaves OPERATOR_HOST unset, which turns the area off
// without touching the declaration. In dev, absent keys mean operator sign-in is REFUSED
// (operatorauth answers ErrNotConfigured), never a fallback.
//
// ROTATION: both are lists, first entry signs/encrypts, every entry verifies/decrypts. HOW OFTEN
// they rotate is not decided — the same gap SESSION_SIGNING_KEYS carries under ADR 0025 and rule
// 8 invariant 6. Inherited here, deliberately not solved here.

// OperatorKeyLength is the exact decoded length of an OPERATOR_TOTP_ENCRYPTION_KEY entry: AES-256
// takes a 32-byte key and nothing else.
const OperatorKeyLength = 32

var (
	// ErrOperatorSigningKeysInvalid is a malformed OPERATOR_SESSION_SIGNING_KEYS. Load refuses.
	ErrOperatorSigningKeysInvalid = errors.New("config: OPERATOR_SESSION_SIGNING_KEYS is invalid")

	// ErrOperatorTOTPKeyInvalid is a malformed OPERATOR_TOTP_ENCRYPTION_KEY. Load refuses.
	ErrOperatorTOTPKeyInvalid = errors.New("config: OPERATOR_TOTP_ENCRYPTION_KEY is invalid")

	// ErrOperatorHostInvalid is a malformed or commune-shaped OPERATOR_HOST. Load refuses.
	ErrOperatorHostInvalid = errors.New("config: OPERATOR_HOST is invalid")
)

// operatorConsoleLabels are the only first labels OPERATOR_HOST may carry directly under the
// platform's own domains (platformWebRoots).
//
// WHY ANY OTHER LABEL THERE IS REFUSED: under vigov.vn every other first label is a commune's web
// host or a candidate for one (ADR 0046: `<xa>.vigov.vn`, `<xa>.stg.vigov.vn`), or a service API
// host (`<service>.api.vigov.vn`). Pointing the operator area at one of those makes a commune's
// surface and the vendor's cross-commune surface the same surface — ADR 0048 stop condition #6 and
// the boundary ADR 0003 draws. `admin` / `admin-stg` are the labels ADR 0046 reserves for the
// vendor console.
//
// A SUBSET COPY of service-platform/internal/domain/ten_mien_danh_rieng.go (nhanDanhRieng), which
// core cannot import (rule 2, forbidden #1). It only ever needs to be NARROWER than that list: a
// label accepted here but not reserved there would be a host a commune row could also claim.
//
// A host OUTSIDE vigov.vn (a deployment on another domain) is not judged here: there is no rule in
// this repository for what a commune host looks like on a domain it does not know.
var operatorConsoleLabels = map[string]bool{"admin": true, "admin-stg": true}

// platformWebRoots and platformRoot are GENERATED into hosts.gen.go from deploy/hosts.yaml, the one
// place public hostnames are written (owner, 01/10/2026) — the same source service-platform's
// reserved-domain rule and platform-admin's copy of this check are generated from.

// parseOperatorHost validates OPERATOR_HOST. raw is the value as the environment held it, trimmed
// the value r.read returned. "" is the designed OFF state and is returned as such.
//
// REFUSED, NEVER REPAIRED — unlike domain.NormaliseHost, which lower-cases and strips a port
// because its input is a client's Host header. This value is written by an operator, once: a port,
// an upper-case letter or a scheme means they meant something this field cannot express, and
// silently normalising it would make the edge answer on a host nobody wrote down. Every refusal
// quotes the value — a host name is not a credential, and the operator needs to see what they set.
func parseOperatorHost(raw, trimmed string) (string, error) {
	if trimmed == "" {
		if raw != "" {
			// Set, but only whitespace: somebody meant to turn the area on and the value got lost
			// in a paste. Reading it as OFF would hide that until the first operator tries to sign in.
			return "", fmt.Errorf("%w: set but blank — unset it to turn the operator area off", ErrOperatorHostInvalid)
		}
		return "", nil
	}
	h := trimmed
	switch {
	case strings.Contains(h, "://") || strings.Contains(h, "/"):
		return "", fmt.Errorf("%w: %q must be a bare host name, no scheme or path", ErrOperatorHostInvalid, h)
	case strings.Contains(h, ":") || strings.Contains(h, "["):
		return "", fmt.Errorf("%w: %q must not carry a port (or be an IPv6 literal)", ErrOperatorHostInvalid, h)
	case h != strings.ToLower(h):
		return "", fmt.Errorf("%w: %q must be lower-case — the edge compares Host lower-cased", ErrOperatorHostInvalid, h)
	case strings.HasSuffix(h, "."):
		return "", fmt.Errorf("%w: %q must not end with a dot", ErrOperatorHostInvalid, h)
	case net.ParseIP(h) != nil:
		// A host-only cookie on an IP is a cookie for whatever else answers on that address, and
		// TLS for the area needs a name.
		return "", fmt.Errorf("%w: %q is an IP address, the operator area needs a host name", ErrOperatorHostInvalid, h)
	case len(h) > 253:
		return "", fmt.Errorf("%w: longer than 253 characters", ErrOperatorHostInvalid)
	}
	labels := strings.Split(h, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: %q is not a fully qualified host name", ErrOperatorHostInvalid, h)
	}
	for _, l := range labels {
		if !validHostLabel(l) {
			return "", fmt.Errorf("%w: %q has an invalid label %q (letters a-z, digits, '-', 1-63 characters, "+
				"not starting or ending with '-')", ErrOperatorHostInvalid, h, l)
		}
	}
	if slices.Contains(platformWebRoots, h) {
		return "", fmt.Errorf("%w: %q is the platform's root domain, not the operator console", ErrOperatorHostInvalid, h)
	}
	// Most specific root first (the generated order): "admin.stg.vigov.vn" also ends in
	// ".vigov.vn", and its label under vigov.vn ("admin.stg") would miss the table.
	for _, root := range platformWebRoots {
		if under, ok := strings.CutSuffix(h, "."+root); ok {
			if !operatorConsoleLabels[under] {
				return "", fmt.Errorf("%w: %q is shaped like a commune or service host under %s — "+
					"the operator area must never share a host with a commune (ADR 0048 stop condition #6); "+
					"use admin.%s or admin-stg.%s", ErrOperatorHostInvalid, h, root, platformRoot, platformRoot)
			}
			break
		}
	}
	return h, nil
}

func validHostLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for _, c := range l {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// operatorEdgeWarnings is the CanhBao contribution of OperatorEdge. Reachable in dev only: in
// staging/prod Load already refused a declared edge without signing keys.
func (c Config) operatorEdgeWarnings() []string {
	if c.operatorHost != "" && len(c.operatorSessionSigningKeys) == 0 {
		return []string{"OPERATOR_HOST có giá trị nhưng OPERATOR_SESSION_SIGNING_KEYS trống — " +
			"khu vận hành bật mà không kiểm được phiên nào; mọi yêu cầu vận hành sẽ bị từ chối"}
	}
	return nil
}

// parseOperatorSigningKeys splits OPERATOR_SESSION_SIGNING_KEYS and refuses any entry that is
// also a staff SESSION_SIGNING_KEYS entry.
//
// WHY THE OVERLAP IS REFUSED: the owner decided a SEPARATE key list (ADR 0048 stop condition #6 —
// a staff token is never accepted on an operator route, and the reverse). The token formats are
// already distinct (`v1.` vs `op1.`) and operatorauth derives its own MAC key, so a shared key
// would not by itself let one token pass for the other — but a shared key means one leak forges
// both realms, and a rotation of one silently rotates the other. "Separate" has to be true of the
// bytes, not merely of the variable name.
//
// Length is NOT checked here: operatorauth.NewTokenSigner owns the minimum (the khoaKy precedent —
// one owner per rule). Errors name the variable and the position, never a value (rule 8).
func parseOperatorSigningKeys(raw string, staff []Khoa) ([]secret.Secret, error) {
	var out []secret.Secret
	for i, entry := range danhSach(raw) {
		for _, s := range staff {
			if subtle.ConstantTimeCompare([]byte(entry), s.Lo()) == 1 {
				return nil, fmt.Errorf("%w: entry %d is also a SESSION_SIGNING_KEYS entry — "+
					"the operator realm must not share key material with staff sessions",
					ErrOperatorSigningKeysInvalid, i+1)
			}
		}
		out = append(out, secret.Secret(entry))
	}
	return out, nil
}

// parseOperatorTOTPKeys decodes OPERATOR_TOTP_ENCRYPTION_KEY: a comma-separated list, each entry
// standard base64 (with padding, as `openssl rand -base64 32` prints it) decoding to EXACTLY 32
// bytes. The returned material is the decoded key, ready for AES-256.
//
// DECODED HERE, NOT BY THE CONSUMER, because the encoding is a property of how the value travels
// through a ConfigMap/Secret, not of how it is used. A malformed entry is FATAL (the
// ObjectStorage / TRUSTED_PROXY_CIDRS precedent): an operator who wrote a value meant something,
// and a skipped entry would be a key that silently cannot decrypt the secrets it sealed.
//
// Duplicates are refused for the same reason a duplicate endpoint is: they are always a paste
// mistake, and here they would also give two entries the same key id.
func parseOperatorTOTPKeys(raw string) ([]secret.Secret, error) {
	return parseAES256KeyList(raw, ErrOperatorTOTPKeyInvalid)
}

// parseAES256KeyList is the one decoder for every "comma-separated list of base64 AES-256 keys"
// variable (OPERATOR_TOTP_ENCRYPTION_KEY, SECRET_ENCRYPTION_KEYS). ONE FORMAT, ONE PARSER: two
// copies of this loop are two places for the length check or the duplicate check to drift, and an
// operator who learned the format on one variable must be able to rely on it for the other.
// errInvalid names the variable in every error, so the caller's sentinel says which one was wrong.
func parseAES256KeyList(raw string, errInvalid error) ([]secret.Secret, error) {
	entries := danhSach(raw)
	out := make([]secret.Secret, 0, len(entries))
	for i, entry := range entries {
		key, err := base64.StdEncoding.DecodeString(entry)
		if err != nil {
			// The decoder's own error is NOT wrapped: it quotes the offending byte offset and,
			// through it, part of the material.
			return nil, fmt.Errorf("%w: entry %d is not standard base64 (generate with "+
				"`openssl rand -base64 32`)", errInvalid, i+1)
		}
		if len(key) != OperatorKeyLength {
			return nil, fmt.Errorf("%w: entry %d decodes to %d bytes, AES-256 needs exactly %d",
				errInvalid, i+1, len(key), OperatorKeyLength)
		}
		for j, prev := range out {
			if subtle.ConstantTimeCompare(key, prev.Lo()) == 1 {
				return nil, fmt.Errorf("%w: entry %d repeats entry %d", errInvalid, i+1, j+1)
			}
		}
		out = append(out, secret.Secret(key))
	}
	return out, nil
}

// OperatorRealmConfigured reports whether BOTH operator variables are set — the only state in
// which operator sign-in can succeed. One without the other is reported by CanhBao.
func (c Config) OperatorRealmConfigured() bool {
	c.require("OperatorRealmConfigured", OperatorRealm)
	return len(c.operatorSessionSigningKeys) > 0 && len(c.operatorTOTPEncryptionKeys) > 0
}

// operatorHalfConfigured names the missing variable when exactly one of the two is set.
func (c Config) operatorHalfConfigured() string {
	hasSign, hasTOTP := len(c.operatorSessionSigningKeys) > 0, len(c.operatorTOTPEncryptionKeys) > 0
	switch {
	case hasSign && !hasTOTP:
		return "OPERATOR_TOTP_ENCRYPTION_KEY"
	case hasTOTP && !hasSign:
		return "OPERATOR_SESSION_SIGNING_KEYS"
	}
	return ""
}

// operatorWarnings is the CanhBao contribution of this file.
func (c Config) operatorWarnings() []string {
	if missing := c.operatorHalfConfigured(); missing != "" {
		return []string{"OPERATOR_* cấu hình nửa vời — thiếu " + missing +
			"; mọi lần đăng nhập vận hành sẽ bị từ chối"}
	}
	return nil
}
