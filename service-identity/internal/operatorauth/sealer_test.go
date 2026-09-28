package operatorauth

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

// 32-byte fake keys — the text says so (rule 8, forbidden #1).
var (
	sealKeyNew = secret.Secret("seal-key-NEW-FAKE-NOT-REAL-32byt")
	sealKeyOld = secret.Secret("seal-key-OLD-FAKE-NOT-REAL-32byt")
	accountAAD = []byte("01J0000000000000000000FAKE")
)

func mustSealer(t *testing.T, keys ...secret.Secret) *Sealer {
	t.Helper()
	s, err := NewSealer(keys)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealerRoundTrip(t *testing.T) {
	s := mustSealer(t, sealKeyNew)
	plain := secret.Secret("totp-secret-FAKE-20b")
	a, err := s.Seal(plain, accountAAD)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Seal(plain, accountAAD)
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext must differ (random nonce)")
	}
	if bytes.Contains(a, plain.Lo()) {
		t.Fatal("plaintext visible in ciphertext")
	}
	got, err := s.Open(a, accountAAD)
	if err != nil || !bytes.Equal(got.Lo(), plain.Lo()) {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestSealerTamper(t *testing.T) {
	s := mustSealer(t, sealKeyNew)
	c, _ := s.Seal(secret.Secret("totp-secret-FAKE-20b"), accountAAD)
	for i := range c {
		bad := bytes.Clone(c)
		bad[i] ^= 0x01
		if _, err := s.Open(bad, accountAAD); err == nil {
			t.Fatalf("flipping byte %d still opened", i)
		}
	}
	if _, err := s.Open(c[:len(c)-1], accountAAD); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("truncated: %v", err)
	}
	if _, err := s.Open(nil, accountAAD); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("empty: %v", err)
	}
}

// The AAD binds a sealed secret to its row: copied onto another account, it must not open.
func TestSealerWrongAAD(t *testing.T) {
	s := mustSealer(t, sealKeyNew)
	c, _ := s.Seal(secret.Secret("totp-secret-FAKE-20b"), accountAAD)
	if _, err := s.Open(c, []byte("01J0000000000000000000OTHR")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("wrong AAD opened: %v", err)
	}
	if _, err := s.Seal(secret.Secret("x"), nil); !errors.Is(err, ErrEmptyAAD) {
		t.Fatalf("empty AAD on seal: %v", err)
	}
	if _, err := s.Open(c, nil); !errors.Is(err, ErrEmptyAAD) {
		t.Fatalf("empty AAD on open: %v", err)
	}
}

func TestSealerRotation(t *testing.T) {
	old := mustSealer(t, sealKeyOld)
	sealedOld, _ := old.Seal(secret.Secret("totp-secret-FAKE-20b"), accountAAD)

	// Rotation: the new key is PREPENDED. Old ciphertexts still open; new ones use the new key.
	rotated := mustSealer(t, sealKeyNew, sealKeyOld)
	got, err := rotated.Open(sealedOld, accountAAD)
	if err != nil || string(got.Lo()) != "totp-secret-FAKE-20b" {
		t.Fatalf("old ciphertext must open after rotation: %v", err)
	}
	if rotated.IsCurrent(sealedOld) {
		t.Fatal("a ciphertext under the old key must report as needing re-seal")
	}
	sealedNew, _ := rotated.Seal(got, accountAAD)
	if !rotated.IsCurrent(sealedNew) {
		t.Fatal("a fresh seal must be under the first key")
	}
	if _, err := old.Open(sealedNew, accountAAD); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("the old-only sealer must not know the new key: %v", err)
	}

	// Reordering the list must not break anything: ids are derived from the key, not position.
	reordered := mustSealer(t, sealKeyOld, sealKeyNew)
	if _, err := reordered.Open(sealedNew, accountAAD); err != nil {
		t.Fatalf("key id must not depend on list position: %v", err)
	}

	// Old key dropped too early.
	newOnly := mustSealer(t, sealKeyNew)
	if _, err := newOnly.Open(sealedOld, accountAAD); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("want ErrUnknownKey, got %v", err)
	}
}

func TestSealerConfiguration(t *testing.T) {
	if _, err := NewSealer(nil); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no key: %v", err)
	}
	var none *Sealer
	if _, err := none.Seal(secret.Secret("x"), accountAAD); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil sealer: %v", err)
	}
	for _, k := range []secret.Secret{secret.Secret("short"), secret.Secret(strings.Repeat("k", 33))} {
		if _, err := NewSealer([]secret.Secret{k}); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("len %d: %v", len(k), err)
		}
	}
	if _, err := NewSealer([]secret.Secret{sealKeyNew, sealKeyNew}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("duplicate key: %v", err)
	}
}

func TestSealerNeverRendersKeys(t *testing.T) {
	s := mustSealer(t, sealKeyNew)
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "sealer", s, "value", *s)
	outs := []string{buf.String()}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q"} {
		outs = append(outs, fmt.Sprintf(verb, s), fmt.Sprintf(verb, *s))
	}
	for _, o := range outs {
		if strings.Contains(o, "seal-key") || !strings.Contains(o, "Sealer(1 keys)") {
			t.Fatalf("unexpected rendering: %q", o)
		}
	}
}
