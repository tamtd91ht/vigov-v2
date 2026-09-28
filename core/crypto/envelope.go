package crypto

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// WrappedDEK is one commune's data key as it is stored: sealed under a KEK, never in the clear.
// It is ciphertext, not a secret — without the KEK it is random bytes (ADR 0009 §Vì sao đây KHÔNG
// vi phạm luật 8).
type WrappedDEK struct {
	// KEKID names the KEK that wrapped it (8 hex characters). Stored as its own column so a
	// rotation job can find the rows still wrapped under an old KEK without unwrapping anything.
	KEKID string
	// Wrapped is the sealed DEK (keyring.go, "Wrapped DEK format").
	Wrapped []byte
}

var (
	// ErrDEKNotFound is what a DEKStore returns when the commune in the context has no DEK.
	ErrDEKNotFound = errors.New("crypto: no DEK stored for this commune")
	// ErrDEKExists is what a DEKStore returns from CreateDEK when the commune already has one —
	// the losing side of two concurrent first seals. Envelope then reads the winner's DEK.
	ErrDEKExists = errors.New("crypto: a DEK already exists for this commune")
	// ErrDEKChanged is what a DEKStore returns from ReplaceDEK when the stored row is no longer
	// the one the caller read — another rotation run got there first.
	ErrDEKChanged = errors.New("crypto: the stored DEK changed underneath the re-wrap")
)

// DEKStore persists wrapped DEKs, ONE ROW PER COMMUNE, in the owning service's own database.
//
// Every method takes the commune from ctx (core/tenant) — never as an argument (rule 1, invariant
// 4) — and reaches the table through the service's scoped store. The table carries tenant_id as
// its unique key, is never deleted from (rule 7: a deleted DEK is every secret of that commune
// destroyed), and its rows are written only through these three methods.
type DEKStore interface {
	// GetDEK returns the commune's wrapped DEK, or ErrDEKNotFound.
	GetDEK(ctx context.Context) (WrappedDEK, error)
	// CreateDEK inserts the commune's first DEK, or returns ErrDEKExists if a row already exists
	// (an INSERT … ON CONFLICT (tenant_id) DO NOTHING that reports zero rows). Never overwrites:
	// overwriting a DEK makes every secret sealed under the old one unreadable.
	CreateDEK(ctx context.Context, dek WrappedDEK) error
	// ReplaceDEK swaps oldDEK for newDEK only if the stored row still equals oldDEK (compare the
	// kek_id AND the wrapped bytes), or returns ErrDEKChanged. Used by RewrapDEK only.
	ReplaceDEK(ctx context.Context, oldDEK, newDEK WrappedDEK) error
}

// Envelope seals and opens one commune's secrets. Safe for concurrent use.
//
// A nil *Envelope is valid and refuses every call with ErrNotConfigured, so a service can hold one
// built from an empty SECRET_ENCRYPTION_KEYS, keep serving everything else, and refuse by name
// exactly the operations that need a KEK.
type Envelope struct {
	ring  *keyring
	store DEKStore
}

// New builds an Envelope from config.Config.SecretEncryptionKeys (already base64-decoded) and the
// service's DEKStore. No keys → (nil, ErrNotConfigured).
func New(keys []secret.Secret, store DEKStore) (*Envelope, error) {
	ring, err := newKeyring(keys)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("crypto: a DEKStore is required")
	}
	return &Envelope{ring: ring, store: store}, nil
}

// Sealed data format, version 1:
//
//	0x01 | nonce (12 bytes) | AES-256-GCM ciphertext + 16-byte tag
//
// Additional data: label | version | tenant_id | caller AAD, each length-prefixed. The tenant_id is
// in it even though the DEK is already per commune: it keeps the binding true should two communes
// ever share a DEK through a store bug, and it costs nothing.
//
// NO DEK ID IN THE FORMAT: there is one DEK per commune and it is never replaced, only re-wrapped.
// Rotating the DEK itself (re-encrypting data) is not supported by version 1; a version 2 would add
// the id behind a new version byte.
const sealedMin = 1 + nonceSize + tagSize

// Seal encrypts plaintext under the commune's DEK, creating the DEK on the commune's first seal.
// aad must identify the row and field the value belongs to (e.g. "smtp_config/password/<row id>").
func (e *Envelope) Seal(ctx context.Context, plaintext secret.Secret, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, ErrNotConfigured
	}
	tid, ok := tenant.From(ctx)
	if !ok {
		return nil, fmt.Errorf("crypto: seal: %w", tenant.ErrNoTenant)
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	dek, err := e.dekForSeal(ctx, tid)
	if err != nil {
		return nil, err
	}
	defer clear(dek)
	aead, err := newAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("crypto: seal: %w", err)
	}
	out := make([]byte, 1+nonceSize, sealedMin+len(plaintext))
	out[0] = formatVersion
	if _, err := rand.Read(out[1:]); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	return aead.Seal(out, out[1:], plaintext.Lo(), dataAD(tid, aad)), nil
}

// Open decrypts a Seal output for the commune in ctx. aad must be the one it was sealed with.
func (e *Envelope) Open(ctx context.Context, sealed, aad []byte) (secret.Secret, error) {
	if e == nil {
		return nil, ErrNotConfigured
	}
	tid, ok := tenant.From(ctx)
	if !ok {
		return nil, fmt.Errorf("crypto: open: %w", tenant.ErrNoTenant)
	}
	if len(aad) == 0 {
		return nil, ErrEmptyAAD
	}
	if len(sealed) < sealedMin || sealed[0] != formatVersion {
		return nil, ErrOpenFailed
	}
	w, err := e.store.GetDEK(ctx)
	if errors.Is(err, ErrDEKNotFound) {
		return nil, ErrNoDEK
	}
	if err != nil {
		return nil, fmt.Errorf("crypto: read DEK: %w", err)
	}
	dek, err := e.ring.unwrap(tid, w)
	if err != nil {
		return nil, err
	}
	defer clear(dek)
	aead, err := newAEAD(dek)
	if err != nil {
		return nil, fmt.Errorf("crypto: open: %w", err)
	}
	plain, err := aead.Open(nil, sealed[1:1+nonceSize], sealed[1+nonceSize:], dataAD(tid, aad))
	if err != nil {
		return nil, ErrOpenFailed
	}
	return secret.Secret(plain), nil
}

// RewrapDEK re-wraps the commune's DEK under the newest KEK. It reports false when the DEK is
// already current or the commune has none. The data sealed under the DEK is untouched — that is
// what makes KEK rotation one row per commune (ADR 0009 #5).
//
// The caller runs it once per commune after prepending a new KEK; the old KEK may be dropped only
// when no row names its id. Whether this write is audited, and under which system principal, is
// the owning service's call (rule 6, invariant 6) — this package writes no trail.
func (e *Envelope) RewrapDEK(ctx context.Context) (bool, error) {
	if e == nil {
		return false, ErrNotConfigured
	}
	tid, ok := tenant.From(ctx)
	if !ok {
		return false, fmt.Errorf("crypto: rewrap: %w", tenant.ErrNoTenant)
	}
	old, err := e.store.GetDEK(ctx)
	if errors.Is(err, ErrDEKNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crypto: read DEK: %w", err)
	}
	if old.KEKID == e.ring.currentID() {
		return false, nil
	}
	dek, err := e.ring.unwrap(tid, old)
	if err != nil {
		return false, err
	}
	defer clear(dek)
	fresh, err := e.ring.wrap(tid, dek)
	if err != nil {
		return false, err
	}
	if err := e.store.ReplaceDEK(ctx, old, fresh); err != nil {
		return false, fmt.Errorf("crypto: store re-wrapped DEK: %w", err)
	}
	return true, nil
}

// CurrentKEKID is the id new DEKs are wrapped under. A rotation job compares a row's KEKID with it.
func (e *Envelope) CurrentKEKID() string {
	if e == nil {
		return ""
	}
	return e.ring.currentID()
}

// dekForSeal returns the commune's unwrapped DEK, creating one if it has none. The caller wipes it.
func (e *Envelope) dekForSeal(ctx context.Context, tid tenant.ID) ([]byte, error) {
	w, err := e.store.GetDEK(ctx)
	if err == nil {
		return e.ring.unwrap(tid, w)
	}
	if !errors.Is(err, ErrDEKNotFound) {
		return nil, fmt.Errorf("crypto: read DEK: %w", err)
	}
	dek := make([]byte, KeyLength)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("crypto: generate DEK: %w", err)
	}
	fresh, err := e.ring.wrap(tid, dek)
	if err != nil {
		clear(dek)
		return nil, err
	}
	err = e.store.CreateDEK(ctx, fresh)
	if err == nil {
		return dek, nil
	}
	clear(dek)
	if !errors.Is(err, ErrDEKExists) {
		return nil, fmt.Errorf("crypto: store DEK: %w", err)
	}
	// Lost the race to a concurrent first seal: use the DEK that was stored, never ours — two DEKs
	// for one commune would leave half its secrets unreadable.
	w, err = e.store.GetDEK(ctx)
	if err != nil {
		return nil, fmt.Errorf("crypto: read DEK after concurrent create: %w", err)
	}
	return e.ring.unwrap(tid, w)
}

func dataAD(tid tenant.ID, aad []byte) []byte {
	ad := appendField(nil, []byte(dataLabel))
	ad = appendField(ad, []byte{formatVersion})
	ad = appendField(ad, []byte(tid))
	return appendField(ad, aad)
}

// String, GoString, Format and LogValue refuse to print anything but the key count.
func (e *Envelope) String() string {
	if e == nil {
		return "crypto.Envelope(not configured)"
	}
	return "crypto.Envelope(" + e.ring.String() + ")"
}

func (e *Envelope) GoString() string { return e.String() }

func (e *Envelope) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(e.String())) }

func (e *Envelope) LogValue() slog.Value { return slog.StringValue(e.String()) }
