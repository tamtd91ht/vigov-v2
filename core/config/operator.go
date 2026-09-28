package config

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/secret"
)

// Operator realm configuration (ADR 0048, owner's decisions of 2026-09-28 #2 and #10).
//
// BOTH VARIABLES ARE OPTIONAL, and "optional" is decided per variable with the reason here (rule
// 11 invariant 8): without them every staff and citizen request is still served; only OPERATOR
// sign-in stops, and it stops by REFUSING (operatorauth answers ErrNotConfigured), never by
// falling back to anything. Making them required would stop all eight services on every machine
// that has no operator area — which is the on-premise deployment of ADR 0048 condition #1 — to
// protect one flow in one service (rule 11 stop condition #2).
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
)

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
	return len(c.OperatorSessionSigningKeys) > 0 && len(c.OperatorTOTPEncryptionKeys) > 0
}

// operatorHalfConfigured names the missing variable when exactly one of the two is set.
func (c Config) operatorHalfConfigured() string {
	hasSign, hasTOTP := len(c.OperatorSessionSigningKeys) > 0, len(c.OperatorTOTPEncryptionKeys) > 0
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
