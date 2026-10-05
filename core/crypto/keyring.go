// Package crypto encrypts a commune's own secrets at rest — envelope encryption per ADR 0009.
//
//	KEK  key-encryption key. Platform-wide, from SECRET_ENCRYPTION_KEYS (a k8s Secret), NEVER in a
//	     database or in git. A list, newest first: the first wraps, every entry unwraps.
//	DEK  data-encryption key. ONE PER COMMUNE, random, stored in the owning service's database
//	     ONLY in wrapped form (sealed under a KEK, bound to that commune's tenant_id).
//	data AES-256-GCM under the commune's DEK, bound to the tenant_id and to the caller's AAD.
//
// WHY A DEK PER COMMUNE AND NOT ONE KEY FOR EVERYTHING: a DEK that leaks exposes one commune, not
// 200 (ADR 0009 #1). And it is what makes KEK rotation cheap: rotating re-wraps one 32-byte DEK per
// commune instead of decrypting and re-encrypting every stored secret (ADR 0009 #5).
//
// WHY THE DEK TABLE BELONGS TO THE SERVICE, not to this package or to a shared service: rule 2 —
// a service never opens another service's database, and the secrets live in the owning service's
// schema. So this package defines DEKStore and each service implements it on its own table, scoped
// by the tenant in the context like every other query (rule 1, invariant 5).
//
// WHAT LOSING THE KEK COSTS, which is why the backup procedure is a release gate and not a chore:
// every wrapped DEK becomes unreadable, and with it every stored secret of every commune on the
// deployment. There is no recovery path in this package by design — a recovery path is a second
// way in. The KEK must be backed up BEFORE the first secret is written, and SEPARATELY from the
// database backup: a backup that holds the KEK and the wrapped DEKs together holds the plaintext.
//
// ROTATION: prepend the new KEK to SECRET_ENCRYPTION_KEYS and redeploy (new DEKs are wrapped under
// it, every old DEK still opens), run Envelope.RewrapDEK for every commune, and only when no row
// names the old key id drop the old KEK. Dropping it first makes those communes' secrets
// unreadable (ErrUnknownKey) — the same failure as losing it.
//
// NOTHING HERE LOGS, and every type refuses to render: the material is either a key or a secret.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// Wrapped DEK format, version 1:
//
//	0x01 | KEK id (4 bytes) | nonce (12 bytes) | AES-256-GCM(DEK) + 16-byte tag
//
// The GCM additional data is a label, the 5-byte header and the tenant_id. The header makes the
// version and KEK id tamper-evident; the tenant_id means a wrapped DEK copied onto another
// commune's row does not open there.
//
// WHY THE KEK ID IS DERIVED FROM THE KEY AND NOT ITS POSITION IN THE LIST: rotation is "prepend
// the new key". A positional id would renumber every old key at that moment. HMAC-SHA256(key,
// label)[:4] is stable across list order and, being a PRF output, says nothing about the key —
// the operatorauth sealer precedent.
const (
	formatVersion  byte = 0x01
	kekIDSize           = 4
	nonceSize           = 12
	tagSize             = 16
	wrapHeaderSize      = 1 + kekIDSize
	// KeyLength is the exact length of a KEK and of a DEK: AES-256 takes 32 bytes and nothing else.
	KeyLength = 32

	kekIDLabel = "vigov/core/crypto/kek-id/v1"
	wrapLabel  = "vigov/core/crypto/dek-wrap/v1"
	dataLabel  = "vigov/core/crypto/data/v1"
)

var (
	// ErrNotConfigured is a process without a KEK. Every Seal and Open refuses with it: a service
	// without SECRET_ENCRYPTION_KEYS must refuse to save or read a commune secret, never store it
	// in the clear or skip it (ADR 0009 #3).
	ErrNotConfigured = errors.New("crypto: SECRET_ENCRYPTION_KEYS is not configured — commune secrets cannot be sealed or opened")

	// ErrInvalidKey is a KEK that is not exactly 32 bytes, or two KEKs sharing an id.
	ErrInvalidKey = errors.New("crypto: a KEK must be exactly 32 bytes")

	// ErrEmptyAAD refuses sealing or opening without associated data. The AAD binds a ciphertext
	// to ITS row and field; without it one commune's SMTP password could be copied onto its portal
	// API key column and would still open.
	ErrEmptyAAD = errors.New("crypto: associated data is required")

	// ErrUnknownKey is a DEK wrapped under a KEK no longer configured — usually an old KEK dropped
	// before every DEK was re-wrapped.
	ErrUnknownKey = errors.New("crypto: DEK wrapped under a KEK that is not configured")

	// ErrOpenFailed is a malformed, tampered, wrong-tenant or wrong-AAD ciphertext or wrapped DEK.
	// One error for all of them (core/token precedent): saying which part failed says how far a
	// forgery got.
	ErrOpenFailed = errors.New("crypto: ciphertext cannot be opened")

	// ErrNoDEK is Open for a commune that has no DEK yet: nothing was ever sealed for it, so the
	// ciphertext handed in cannot be its own.
	ErrNoDEK = errors.New("crypto: this commune has no data key")
)

type kek struct {
	id   [kekIDSize]byte
	aead cipher.AEAD
}

// keyring holds the KEKs, newest first.
type keyring struct {
	keks []kek
}

func newKeyring(keys []secret.Secret) (*keyring, error) {
	if len(keys) == 0 {
		return nil, ErrNotConfigured
	}
	r := &keyring{keks: make([]kek, 0, len(keys))}
	for i, k := range keys {
		if len(k) != KeyLength {
			return nil, fmt.Errorf("%w: KEK %d is %d bytes", ErrInvalidKey, i+1, len(k))
		}
		aead, err := newAEAD(k.Lo())
		if err != nil {
			return nil, fmt.Errorf("crypto: KEK %d: %w", i+1, err)
		}
		m := hmac.New(sha256.New, k.Lo())
		m.Write([]byte(kekIDLabel))
		var id [kekIDSize]byte
		copy(id[:], m.Sum(nil))
		for j, prev := range r.keks {
			if prev.id == id {
				return nil, fmt.Errorf("%w: KEK %d has the same id as KEK %d (duplicate key?)", ErrInvalidKey, i+1, j+1)
			}
		}
		r.keks = append(r.keks, kek{id: id, aead: aead})
	}
	return r, nil
}

// wrap seals a DEK under the NEWEST KEK, bound to the commune.
func (r *keyring) wrap(tid tenant.ID, dek []byte) (WrappedDEK, error) {
	return r.wrapBound(wrapLabel, []byte(tid), dek)
}

// unwrap opens a commune's wrapped DEK with whichever KEK it names. The caller wipes the result.
func (r *keyring) unwrap(tid tenant.ID, w WrappedDEK) ([]byte, error) {
	return r.unwrapBound(wrapLabel, []byte(tid), w)
}

// wrapBound seals a DEK under the NEWEST KEK, bound to (label, binding). The commune wrap passes
// (wrapLabel, tenant_id); the platform wrap passes (platformWrapLabel, PlatformScope) — two labels, so
// a wrapped DEK copied from one table to the other opens in neither (platform.go).
func (r *keyring) wrapBound(label string, binding, dek []byte) (WrappedDEK, error) {
	k := r.keks[0]
	out := make([]byte, wrapHeaderSize+nonceSize, wrapHeaderSize+nonceSize+len(dek)+tagSize)
	out[0] = formatVersion
	copy(out[1:wrapHeaderSize], k.id[:])
	if _, err := rand.Read(out[wrapHeaderSize:]); err != nil {
		return WrappedDEK{}, fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := k.aead.Seal(out, out[wrapHeaderSize:], dek, wrapAD(label, out[:wrapHeaderSize], binding))
	return WrappedDEK{KEKID: kekIDString(k.id), Wrapped: sealed}, nil
}

// unwrapBound opens a wrapped DEK bound to (label, binding) with whichever KEK it names. The caller
// wipes the result.
func (r *keyring) unwrapBound(label string, binding []byte, w WrappedDEK) ([]byte, error) {
	b := w.Wrapped
	if len(b) != wrapHeaderSize+nonceSize+KeyLength+tagSize || b[0] != formatVersion {
		return nil, ErrOpenFailed
	}
	var id [kekIDSize]byte
	copy(id[:], b[1:wrapHeaderSize])
	// The column and the header must agree: a row whose KEKID says one key and whose bytes say
	// another is a corrupted or hand-edited row, and a rotation job reading the column would skip it.
	if w.KEKID != kekIDString(id) {
		return nil, ErrOpenFailed
	}
	k, ok := r.find(id)
	if !ok {
		return nil, ErrUnknownKey
	}
	dek, err := k.aead.Open(nil, b[wrapHeaderSize:wrapHeaderSize+nonceSize], b[wrapHeaderSize+nonceSize:], wrapAD(label, b[:wrapHeaderSize], binding))
	if err != nil {
		return nil, ErrOpenFailed
	}
	return dek, nil
}

func (r *keyring) find(id [kekIDSize]byte) (kek, bool) {
	for _, k := range r.keks {
		if hmac.Equal(k.id[:], id[:]) {
			return k, true
		}
	}
	return kek{}, false
}

func (r *keyring) currentID() string { return kekIDString(r.keks[0].id) }

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return aead, nil
}

func kekIDString(id [kekIDSize]byte) string { return hex.EncodeToString(id[:]) }

func wrapAD(label string, header, binding []byte) []byte {
	return appendField(appendField(appendField(nil, []byte(label)), header), binding)
}

// appendField length-prefixes every part of an additional-data string, so "ab"+"c" and "a"+"bc"
// can never produce the same bytes — without it, a caller AAD could be shifted into the tenant id.
func appendField(dst, b []byte) []byte {
	n := len(b)
	dst = append(dst, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	return append(dst, b...)
}

// String, GoString, Format and LogValue refuse to print anything but the key count.
func (r *keyring) String() string {
	if r == nil {
		return "crypto.keyring(0 keys)"
	}
	return fmt.Sprintf("crypto.keyring(%d keys)", len(r.keks))
}

func (r *keyring) GoString() string { return r.String() }

func (r *keyring) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(r.String())) }

func (r *keyring) LogValue() slog.Value { return slog.StringValue(r.String()) }
