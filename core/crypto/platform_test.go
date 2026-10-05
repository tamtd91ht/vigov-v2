package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// memPlatformStore is a PlatformDEKStore over one slot — the shape a service implements on its own
// single-row table. It deliberately IGNORES the context: there is no commune to read.
type memPlatformStore struct {
	mu  sync.Mutex
	row *WrappedDEK
}

func (m *memPlatformStore) GetPlatformDEK(context.Context) (WrappedDEK, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		return WrappedDEK{}, ErrPlatformDEKNotFound
	}
	return WrappedDEK{KEKID: m.row.KEKID, Wrapped: bytes.Clone(m.row.Wrapped)}, nil
}

func (m *memPlatformStore) CreatePlatformDEK(_ context.Context, w WrappedDEK) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row != nil {
		return ErrPlatformDEKExists
	}
	m.row = &w
	return nil
}

func (m *memPlatformStore) ReplacePlatformDEK(_ context.Context, oldDEK, newDEK WrappedDEK) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil || m.row.KEKID != oldDEK.KEKID || !bytes.Equal(m.row.Wrapped, oldDEK.Wrapped) {
		return ErrPlatformDEKChanged
	}
	m.row = &newDEK
	return nil
}

func mustPlatform(t *testing.T, store PlatformDEKStore, keys ...secret.Secret) *PlatformEnvelope {
	t.Helper()
	e, err := NewPlatform(keys, store)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

var platformAAD = []byte("zalo_bot_shared/token_sealed/shared")

const platformPlain = "123456789:FAKE-zalo-bot-token-NOT-REAL"

func TestPlatformRoundTripNeedsNoCommune(t *testing.T) {
	store := &memPlatformStore{}
	e := mustPlatform(t, store, kekA)
	// context.Background: no commune at all, and none is needed.
	sealed, err := e.SealPlatform(context.Background(), secret.Secret(platformPlain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(platformPlain)) {
		t.Fatal("ciphertext contains the plaintext")
	}
	if len(store.row.Wrapped) != 65 {
		t.Fatalf("wrapped platform DEK is %d bytes, 0018 CHECKs 65", len(store.row.Wrapped))
	}
	got, err := e.OpenPlatform(context.Background(), sealed, platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Lo()) != platformPlain {
		t.Fatal("round trip changed the value")
	}
	// A commune in the context changes nothing: it is never read.
	got, err = e.OpenPlatform(ctxFor(communeA), sealed, platformAAD)
	if err != nil || string(got.Lo()) != platformPlain {
		t.Fatalf("a commune in the context must be ignored: %v", err)
	}
	if _, err := e.OpenPlatform(context.Background(), sealed, []byte("zalo_bot_shared/webhook_secret_sealed/shared")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("another field's aad opened it: %v", err)
	}
}

// THE PROPERTY ADR 0074 #4 ASKS FOR: a platform secret cannot be opened as any commune's, and a
// commune's cannot be opened as the platform's — even when the attacker has moved the WRAPPED DEK
// from one table to the other. (A commune whose id is the string "platform" cannot exist: tenant.Into
// refuses an id that is not ULID-shaped, so the scope string can never reach a commune context.)
func TestPlatformAndCommuneKeySpacesDoNotMeet(t *testing.T) {
	pstore := &memPlatformStore{}
	pe := mustPlatform(t, pstore, kekA)
	psealed, err := pe.SealPlatform(context.Background(), secret.Secret(platformPlain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}

	cstore := newMemStore()
	ce := mustNew(t, cstore, kekA)
	csealed, err := ce.Seal(ctxFor(communeA), secret.Secret(plain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}

	// 1. The platform's wrapped DEK copied onto commune rows — including one keyed "platform".
	for _, tid := range []tenant.ID{communeA, communeB} {
		cstore.rows[tid] = *pstore.row
		if _, err := ce.Open(ctxFor(tid), psealed, platformAAD); !errors.Is(err, ErrOpenFailed) {
			t.Errorf("platform secret opened as commune %q: %v", tid, err)
		}
	}

	// 2. A commune's wrapped DEK copied into the platform table.
	pstore2 := &memPlatformStore{}
	cw := newMemStore()
	ce2 := mustNew(t, cw, kekA)
	csealed2, err := ce2.Seal(ctxFor(communeA), secret.Secret(plain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	w := cw.rows[communeA]
	pstore2.row = &w
	pe2 := mustPlatform(t, pstore2, kekA)
	if _, err := pe2.OpenPlatform(context.Background(), csealed2, platformAAD); !errors.Is(err, ErrOpenFailed) {
		t.Errorf("commune secret opened as the platform's: %v", err)
	}
	_ = csealed
}

// Even with the SAME raw DEK on both sides, the data label keeps a ciphertext in its own space. This
// reaches below the wrap on purpose: it proves the second wall stands on its own.
func TestSameRawDEKStillSeparatesData(t *testing.T) {
	ring, err := newKeyring([]secret.Secret{kekA})
	if err != nil {
		t.Fatal(err)
	}
	dek := bytes.Repeat([]byte{7}, KeyLength)
	pw, err := ring.wrapBound(platformWrapLabel, []byte(PlatformScope), dek)
	if err != nil {
		t.Fatal(err)
	}
	cw, err := ring.wrap(communeA, dek)
	if err != nil {
		t.Fatal(err)
	}
	pe := mustPlatform(t, &memPlatformStore{row: &pw}, kekA)
	cs := newMemStore()
	cs.rows[communeA] = cw
	ce := mustNew(t, cs, kekA)

	psealed, err := pe.SealPlatform(context.Background(), secret.Secret(platformPlain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ce.Open(ctxFor(communeA), psealed, platformAAD); !errors.Is(err, ErrOpenFailed) {
		t.Errorf("platform ciphertext opened under the commune data label: %v", err)
	}
	csealed, err := ce.Seal(ctxFor(communeA), secret.Secret(plain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pe.OpenPlatform(context.Background(), csealed, platformAAD); !errors.Is(err, ErrOpenFailed) {
		t.Errorf("commune ciphertext opened under the platform data label: %v", err)
	}
	// Sanity: each opens its own.
	if _, err := pe.OpenPlatform(context.Background(), psealed, platformAAD); err != nil {
		t.Fatal(err)
	}
}

// The scope string can never travel as a commune: tenant.Into refuses it.
func TestPlatformScopeIsNotACommune(t *testing.T) {
	if tenant.ID(PlatformScope).Valid() {
		t.Fatal("PlatformScope is ULID-shaped — it could be mistaken for a commune")
	}
}

func TestPlatformNotConfiguredAndNoDEK(t *testing.T) {
	var nilEnv *PlatformEnvelope
	if _, err := nilEnv.SealPlatform(context.Background(), secret.Secret("x"), platformAAD); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil envelope: %v", err)
	}
	if _, err := NewPlatform(nil, &memPlatformStore{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no keys: %v", err)
	}
	e := mustPlatform(t, &memPlatformStore{}, kekA)
	if _, err := e.OpenPlatform(context.Background(), make([]byte, sealedMin+1), platformAAD); err == nil {
		t.Fatal("opened garbage")
	}
	sealed := append([]byte{formatVersion}, make([]byte, sealedMin)...)
	if _, err := e.OpenPlatform(context.Background(), sealed, platformAAD); !errors.Is(err, ErrNoPlatformDEK) {
		t.Fatalf("no DEK: %v", err)
	}
	if _, err := e.SealPlatform(context.Background(), secret.Secret("x"), nil); !errors.Is(err, ErrEmptyAAD) {
		t.Fatalf("empty aad: %v", err)
	}
}

func TestPlatformRewrapKeepsSecretsReadable(t *testing.T) {
	store := &memPlatformStore{}
	old := mustPlatform(t, store, kekA)
	sealed, err := old.SealPlatform(context.Background(), secret.Secret(platformPlain), platformAAD)
	if err != nil {
		t.Fatal(err)
	}
	rotated := mustPlatform(t, store, kekB, kekA)
	moved, err := rotated.RewrapPlatformDEK(context.Background())
	if err != nil || !moved {
		t.Fatalf("rewrap: moved=%v err=%v", moved, err)
	}
	if store.row.KEKID != rotated.CurrentKEKID() {
		t.Fatal("row not under the newest KEK")
	}
	onlyNew := mustPlatform(t, store, kekB)
	got, err := onlyNew.OpenPlatform(context.Background(), sealed, platformAAD)
	if err != nil || string(got.Lo()) != platformPlain {
		t.Fatalf("secret unreadable after rewrap: %v", err)
	}
	again, err := onlyNew.RewrapPlatformDEK(context.Background())
	if err != nil || again {
		t.Fatalf("second rewrap must be a no-op: %v %v", again, err)
	}
}

func TestPlatformConcurrentFirstSealsShareOneDEK(t *testing.T) {
	store := &memPlatformStore{}
	e := mustPlatform(t, store, kekA)
	var wg sync.WaitGroup
	out := make([][]byte, 8)
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := e.SealPlatform(context.Background(), secret.Secret(platformPlain), platformAAD)
			if err != nil {
				t.Error(err)
			}
			out[i] = s
		}(i)
	}
	wg.Wait()
	for _, s := range out {
		if _, err := e.OpenPlatform(context.Background(), s, platformAAD); err != nil {
			t.Fatalf("a seal used a DEK that was not stored: %v", err)
		}
	}
}

func TestPlatformEnvelopeNeverRenders(t *testing.T) {
	e := mustPlatform(t, &memPlatformStore{}, kekA)
	for _, s := range []string{fmt.Sprint(e), fmt.Sprintf("%+v", e), fmt.Sprintf("%#v", e)} {
		if bytes.Contains([]byte(s), kekA.Lo()) || s != "crypto.PlatformEnvelope(crypto.keyring(1 keys))" {
			t.Errorf("rendered %q", s)
		}
	}
}
