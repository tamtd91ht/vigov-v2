package operatorauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/core/secret"
)

// Sealed format, version 1:
//
//	0x01 | key id (4 bytes) | nonce (12 bytes) | AES-256-GCM ciphertext + 16-byte tag
//
// The GCM additional data is the 5-byte header FOLLOWED BY the caller's AAD, so neither the
// version nor the key id can be rewritten without the tag failing.
//
// WHY A KEY ID DERIVED FROM THE KEY AND NOT THE KEY'S POSITION IN THE LIST: rotation is "prepend
// the new key". A positional id would renumber every old key at that moment and make every
// existing ciphertext point at the wrong key. The id is HMAC-SHA256(key, label)[:4] — stable
// across list order, and a PRF output, so it says nothing about the key itself.
const (
	sealVersion   byte = 0x01
	keyIDSize          = 4
	nonceSize          = 12
	sealHeader         = 1 + keyIDSize
	sealKeyLength      = 32 // AES-256
	keyIDLabel         = "vigov/operatorauth/sealer-key-id/v1"
)

var (
	// ErrInvalidKey is a sealer key that is not exactly 32 bytes, or two keys sharing an id.
	ErrInvalidKey = errors.New("operatorauth: sealer key must be exactly 32 bytes")

	// ErrEmptyAAD refuses sealing or opening without associated data. The AAD is what binds a
	// ciphertext to ITS row (the operator account id); without it a sealed TOTP secret could be
	// copied onto another account and would still decrypt there.
	ErrEmptyAAD = errors.New("operatorauth: associated data is required")

	// ErrUnknownKey is a ciphertext sealed under a key no longer configured — usually an old key
	// dropped from OPERATOR_TOTP_ENCRYPTION_KEY before every secret was re-sealed.
	ErrUnknownKey = errors.New("operatorauth: ciphertext sealed under a key that is not configured")

	// ErrOpenFailed is a malformed, tampered or wrong-AAD ciphertext. One error for all of them,
	// the core/token precedent: saying which part failed says how far a forgery got.
	ErrOpenFailed = errors.New("operatorauth: ciphertext cannot be opened")
)

type sealerKey struct {
	id   [keyIDSize]byte
	aead cipher.AEAD
}

// Sealer encrypts with the FIRST key and decrypts with whichever configured key the ciphertext
// names. Build it from config.Config.OperatorTOTPEncryptionKeys (already base64-decoded).
type Sealer struct {
	keys []sealerKey
}

// NewSealer validates the keys once. No keys → ErrNotConfigured: the caller refuses enrolment and
// sign-in rather than storing a TOTP secret in the clear.
func NewSealer(keys []secret.Secret) (*Sealer, error) {
	if len(keys) == 0 {
		return nil, ErrNotConfigured
	}
	s := &Sealer{keys: make([]sealerKey, 0, len(keys))}
	for i, k := range keys {
		if len(k) != sealKeyLength {
			return nil, fmt.Errorf("%w: key %d is %d bytes", ErrInvalidKey, i+1, len(k))
		}
		block, err := aes.NewCipher(k.Lo())
		if err != nil {
			return nil, fmt.Errorf("operatorauth: key %d: %w", i+1, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("operatorauth: key %d: %w", i+1, err)
		}
		m := hmac.New(sha256.New, k.Lo())
		m.Write([]byte(keyIDLabel))
		var id [keyIDSize]byte
		copy(id[:], m.Sum(nil))
		for j, prev := range s.keys {
			if prev.id == id {
				return nil, fmt.Errorf("%w: key %d has the same id as key %d (duplicate key?)", ErrInvalidKey, i+1, j+1)
			}
		}
		s.keys = append(s.keys, sealerKey{id: id, aead: aead})
	}
	return s, nil
}

// Seal encrypts plaintext under the first key, binding it to aad (pass the operator account id).
func (s *Sealer) Seal(plaintext secret.Secret, aad []byte) ([]byte, error) {
	if s == nil || len(s.keys) == 0 {
		return nil, ErrNotConfigured
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	k := s.keys[0]
	out := make([]byte, sealHeader+nonceSize, sealHeader+nonceSize+len(plaintext)+k.aead.Overhead())
	out[0] = sealVersion
	copy(out[1:sealHeader], k.id[:])
	if _, err := rand.Read(out[sealHeader:]); err != nil {
		return nil, fmt.Errorf("operatorauth: nonce: %w", err)
	}
	return k.aead.Seal(out, out[sealHeader:], plaintext.Lo(), additionalData(out[:sealHeader], aad)), nil
}

// Open decrypts a Seal output. The aad must be the one it was sealed with.
func (s *Sealer) Open(sealed, aad []byte) (secret.Secret, error) {
	if s == nil || len(s.keys) == 0 {
		return nil, ErrNotConfigured
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	k, err := s.keyFor(sealed)
	if err != nil {
		return nil, err
	}
	header, nonce, body := sealed[:sealHeader], sealed[sealHeader:sealHeader+nonceSize], sealed[sealHeader+nonceSize:]
	plain, err := k.aead.Open(nil, nonce, body, additionalData(header, aad))
	if err != nil {
		return nil, ErrOpenFailed
	}
	return secret.Secret(plain), nil
}

// IsCurrent reports whether a ciphertext is sealed under the FIRST (current) key. A rotation job
// re-seals every row for which this is false; only then may the old key be dropped.
func (s *Sealer) IsCurrent(sealed []byte) bool {
	if s == nil || len(s.keys) == 0 {
		return false
	}
	k, err := s.keyFor(sealed)
	return err == nil && k.id == s.keys[0].id
}

func (s *Sealer) keyFor(sealed []byte) (sealerKey, error) {
	if len(sealed) < sealHeader+nonceSize+16 || sealed[0] != sealVersion {
		return sealerKey{}, ErrOpenFailed
	}
	for _, k := range s.keys {
		if hmac.Equal(k.id[:], sealed[1:sealHeader]) {
			return k, nil
		}
	}
	return sealerKey{}, ErrUnknownKey
}

func additionalData(header, aad []byte) []byte {
	ad := make([]byte, 0, len(header)+len(aad))
	return append(append(ad, header...), aad...)
}

// String, GoString, Format and LogValue refuse to print the keys (see doc.go).
func (s Sealer) String() string { return fmt.Sprintf("operatorauth.Sealer(%d keys)", len(s.keys)) }

func (s Sealer) GoString() string { return s.String() }

func (s Sealer) Format(f fmt.State, verb rune) { redacted(f, verb, s.String()) }

func (s Sealer) LogValue() slog.Value { return redactedLog(s.String()) }
