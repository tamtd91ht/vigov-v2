package crypto

// PLATFORM-SCOPE SEALING — secrets that belong to NO commune (ADR 0074 #4, owner 05/10/2026: "một hàm
// niêm riêng trong core/crypto, không giả làm một xã"). The first is the shared Zalo Bot token and its
// webhook secret_token, held by service-comms.
//
// WHY NOT Envelope WITH SOME tenant_id: Envelope takes the commune from the context, and the only
// way to give it one for a platform secret is to invent one — a "platform" ULID, the first commune, a
// sentinel. That is a default on the isolation path (rule 1, forbidden #1), and a sentinel would be
// accepted by every scoped store and tenant-keyed cache as if it were a commune.
//
// WHAT IS THE SAME AS Envelope: the KEKs (SECRET_ENCRYPTION_KEYS, one keyring), the version-1 wrapped
// DEK format (65 bytes — service-comms 0018 `platform_data_encryption_key` CHECKs that length), the
// sealed data format, AES-256-GCM, one DEK per scope created on the first seal, and rotation by
// re-wrap.
//
// WHAT KEEPS THE TWO KEY SPACES APART, BY CONSTRUCTION AND NOT BY CONVENTION:
//
//   - a DIFFERENT LABEL in the DEK wrap's additional data (platformWrapLabel, not wrapLabel), bound to
//     the fixed scope string PlatformScope instead of a tenant_id. A wrapped DEK copied between
//     `data_encryption_key` and `platform_data_encryption_key` opens in neither table — not even under
//     a commune whose id happened to be the string "platform".
//   - a DIFFERENT LABEL in the data's additional data (platformDataLabel). A ciphertext sealed here
//     does not open as any commune's, and a commune's does not open here, EVEN IF both sides were
//     somehow handed the same raw DEK.
//   - DIFFERENT METHOD NAMES (SealPlatform / OpenPlatform), so no interface written for one envelope
//     is satisfied by the other: a caller holding the wrong one does not compile.
//   - NO CONTEXT IS READ for the scope. tenant.From is never called here; a commune in the context is
//     ignored, never mixed in.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/core/secret"
)

// PlatformScope is the one platform scope that exists (service-comms 0018:
// `platform_data_encryption_key_scope_known CHECK (scope IN ('platform'))`). It is bound into every
// platform wrap and every platform seal. It is NOT a tenant id and never travels as one: it is not
// ULID-shaped, so tenant.ID.Valid refuses it.
const PlatformScope = "platform"

const (
	platformWrapLabel = "vigov/core/crypto/platform-dek-wrap/v1"
	platformDataLabel = "vigov/core/crypto/platform-data/v1"
)

var (
	// ErrPlatformDEKNotFound is what a PlatformDEKStore returns when no platform DEK is stored.
	ErrPlatformDEKNotFound = errors.New("crypto: no platform DEK stored")
	// ErrPlatformDEKExists is what a PlatformDEKStore returns from CreatePlatformDEK when a row already
	// exists — the losing side of two concurrent first seals.
	ErrPlatformDEKExists = errors.New("crypto: a platform DEK already exists")
	// ErrPlatformDEKChanged is what a PlatformDEKStore returns from ReplacePlatformDEK when the stored
	// row is no longer the one the caller read.
	ErrPlatformDEKChanged = errors.New("crypto: the stored platform DEK changed underneath the re-wrap")
	// ErrNoPlatformDEK is OpenPlatform with no platform DEK stored: nothing was ever sealed, so the
	// ciphertext handed in cannot be a platform secret of this deployment.
	ErrNoPlatformDEK = errors.New("crypto: the platform has no data key")
)

// PlatformDEKStore persists THE platform DEK — one row, keyed by PlatformScope, in the owning
// service's own database (rule 2). Never deleted (rule 7: a deleted DEK is every platform secret
// destroyed). Unlike DEKStore it reads NO commune from the context: there is none.
type PlatformDEKStore interface {
	// GetPlatformDEK returns the wrapped platform DEK, or ErrPlatformDEKNotFound.
	GetPlatformDEK(ctx context.Context) (WrappedDEK, error)
	// CreatePlatformDEK inserts the first platform DEK, or returns ErrPlatformDEKExists. Never
	// overwrites.
	CreatePlatformDEK(ctx context.Context, dek WrappedDEK) error
	// ReplacePlatformDEK swaps oldDEK for newDEK only if the stored row still equals oldDEK, or returns
	// ErrPlatformDEKChanged. Used by RewrapPlatformDEK only.
	ReplacePlatformDEK(ctx context.Context, oldDEK, newDEK WrappedDEK) error
}

// PlatformEnvelope seals and opens platform-scope secrets. Safe for concurrent use. A nil
// *PlatformEnvelope refuses every call with ErrNotConfigured, the Envelope convention.
type PlatformEnvelope struct {
	ring  *keyring
	store PlatformDEKStore
}

// NewPlatform builds a PlatformEnvelope from config.Config.SecretEncryptionKeys (already decoded) and
// the service's PlatformDEKStore. No keys → (nil, ErrNotConfigured).
func NewPlatform(keys []secret.Secret, store PlatformDEKStore) (*PlatformEnvelope, error) {
	ring, err := newKeyring(keys)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("crypto: a PlatformDEKStore is required")
	}
	return &PlatformEnvelope{ring: ring, store: store}, nil
}

// SealPlatform encrypts plaintext under the platform DEK, creating the DEK on the first seal. aad must
// identify the row and field (e.g. "zalo_bot_shared/token_sealed/shared"). Output: the version-1 sealed
// format of Envelope.Seal.
func (e *PlatformEnvelope) SealPlatform(ctx context.Context, plaintext secret.Secret, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, ErrNotConfigured
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	dek, err := e.dekForSeal(ctx)
	if err != nil {
		return nil, err
	}
	defer clear(dek)
	aead, err := newAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("crypto: seal platform: %w", err)
	}
	out := make([]byte, 1+nonceSize, sealedMin+len(plaintext))
	out[0] = formatVersion
	if _, err := rand.Read(out[1:]); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	return aead.Seal(out, out[1:], plaintext.Lo(), platformDataAD(aad)), nil
}

// OpenPlatform decrypts a SealPlatform output. aad must be the one it was sealed with. A commune's
// ciphertext, a tampered one, or one under another aad is ErrOpenFailed — one error for all of them.
func (e *PlatformEnvelope) OpenPlatform(ctx context.Context, sealed, aad []byte) (secret.Secret, error) {
	if e == nil {
		return nil, ErrNotConfigured
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	if len(sealed) < sealedMin || sealed[0] != formatVersion {
		return nil, ErrOpenFailed
	}
	w, err := e.store.GetPlatformDEK(ctx)
	if errors.Is(err, ErrPlatformDEKNotFound) {
		return nil, ErrNoPlatformDEK
	}
	if err != nil {
		return nil, fmt.Errorf("crypto: read platform DEK: %w", err)
	}
	dek, err := e.ring.unwrapBound(platformWrapLabel, []byte(PlatformScope), w)
	if err != nil {
		return nil, err
	}
	defer clear(dek)
	aead, err := newAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("crypto: open platform: %w", err)
	}
	plain, err := aead.Open(nil, sealed[1:1+nonceSize], sealed[1+nonceSize:], platformDataAD(aad))
	if err != nil {
		return nil, ErrOpenFailed
	}
	return secret.Secret(plain), nil
}

// RewrapPlatformDEK re-wraps the platform DEK under the newest KEK — the platform half of the KEK
// rotation Envelope.RewrapDEK does per commune. False when already current or when none exists.
// Whether the write is audited, and under whom, is the owning store's decision.
func (e *PlatformEnvelope) RewrapPlatformDEK(ctx context.Context) (bool, error) {
	if e == nil {
		return false, ErrNotConfigured
	}
	old, err := e.store.GetPlatformDEK(ctx)
	if errors.Is(err, ErrPlatformDEKNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crypto: read platform DEK: %w", err)
	}
	if old.KEKID == e.ring.currentID() {
		return false, nil
	}
	dek, err := e.ring.unwrapBound(platformWrapLabel, []byte(PlatformScope), old)
	if err != nil {
		return false, err
	}
	defer clear(dek)
	fresh, err := e.ring.wrapBound(platformWrapLabel, []byte(PlatformScope), dek)
	if err != nil {
		return false, err
	}
	if err := e.store.ReplacePlatformDEK(ctx, old, fresh); err != nil {
		return false, fmt.Errorf("crypto: store re-wrapped platform DEK: %w", err)
	}
	return true, nil
}

// CurrentKEKID is the id new platform DEKs are wrapped under.
func (e *PlatformEnvelope) CurrentKEKID() string {
	if e == nil {
		return ""
	}
	return e.ring.currentID()
}

// dekForSeal returns the unwrapped platform DEK, creating it on first use. The caller wipes it.
func (e *PlatformEnvelope) dekForSeal(ctx context.Context) ([]byte, error) {
	w, err := e.store.GetPlatformDEK(ctx)
	if err == nil {
		return e.ring.unwrapBound(platformWrapLabel, []byte(PlatformScope), w)
	}
	if !errors.Is(err, ErrPlatformDEKNotFound) {
		return nil, fmt.Errorf("crypto: read platform DEK: %w", err)
	}
	dek := make([]byte, KeyLength)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("crypto: generate platform DEK: %w", err)
	}
	fresh, err := e.ring.wrapBound(platformWrapLabel, []byte(PlatformScope), dek)
	if err != nil {
		clear(dek)
		return nil, err
	}
	err = e.store.CreatePlatformDEK(ctx, fresh)
	if err == nil {
		return dek, nil
	}
	clear(dek)
	if !errors.Is(err, ErrPlatformDEKExists) {
		return nil, fmt.Errorf("crypto: store platform DEK: %w", err)
	}
	// Lost the race to a concurrent first seal: use the stored DEK, never ours.
	w, err = e.store.GetPlatformDEK(ctx)
	if err != nil {
		return nil, fmt.Errorf("crypto: read platform DEK after concurrent create: %w", err)
	}
	return e.ring.unwrapBound(platformWrapLabel, []byte(PlatformScope), w)
}

func platformDataAD(aad []byte) []byte {
	ad := appendField(nil, []byte(platformDataLabel))
	ad = appendField(ad, []byte{formatVersion})
	ad = appendField(ad, []byte(PlatformScope))
	return appendField(ad, aad)
}

// String, GoString, Format and LogValue refuse to print anything but the key count.
func (e *PlatformEnvelope) String() string {
	if e == nil {
		return "crypto.PlatformEnvelope(not configured)"
	}
	return "crypto.PlatformEnvelope(" + e.ring.String() + ")"
}

func (e *PlatformEnvelope) GoString() string { return e.String() }

func (e *PlatformEnvelope) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(e.String())) }

func (e *PlatformEnvelope) LogValue() slog.Value { return slog.StringValue(e.String()) }
