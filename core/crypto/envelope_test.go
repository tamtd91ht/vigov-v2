package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// Fake KEKs — the text says so (rule 8, forbidden #1). Each is exactly 32 bytes.
var (
	kekA = secret.Secret("kek-A-FAKE-NOT-A-REAL-KEY-32byt!")
	kekB = secret.Secret("kek-B-FAKE-NOT-A-REAL-KEY-32byt!")
	kekC = secret.Secret("kek-C-FAKE-NOT-A-REAL-KEY-32byt!")
)

// Fake commune ids: 26 characters, the ULID length tenant.ID.Valid checks.
const (
	communeA tenant.ID = "01JTESTCOMMUNEAAAAAAAAAAAA"
	communeB tenant.ID = "01JTESTCOMMUNEBBBBBBBBBBBB"
)

var aad = []byte("smtp_config/password/01JTESTROW0000000000000000")

// memStore is a DEKStore keyed by the tenant in the context — the shape a service implements on
// its own table.
type memStore struct {
	mu   sync.Mutex
	rows map[tenant.ID]WrappedDEK
}

func newMemStore() *memStore { return &memStore{rows: map[tenant.ID]WrappedDEK{}} }

func (m *memStore) GetDEK(ctx context.Context) (WrappedDEK, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.rows[tenant.MustFrom(ctx)]
	if !ok {
		return WrappedDEK{}, ErrDEKNotFound
	}
	return WrappedDEK{KEKID: w.KEKID, Wrapped: bytes.Clone(w.Wrapped)}, nil
}

func (m *memStore) CreateDEK(ctx context.Context, w WrappedDEK) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := tenant.MustFrom(ctx)
	if _, ok := m.rows[tid]; ok {
		return ErrDEKExists
	}
	m.rows[tid] = w
	return nil
}

func (m *memStore) ReplaceDEK(ctx context.Context, oldDEK, newDEK WrappedDEK) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := tenant.MustFrom(ctx)
	cur, ok := m.rows[tid]
	if !ok || cur.KEKID != oldDEK.KEKID || !bytes.Equal(cur.Wrapped, oldDEK.Wrapped) {
		return ErrDEKChanged
	}
	m.rows[tid] = newDEK
	return nil
}

func ctxFor(id tenant.ID) context.Context { return tenant.Into(context.Background(), id) }

func mustNew(t *testing.T, store DEKStore, keys ...secret.Secret) *Envelope {
	t.Helper()
	e, err := New(keys, store)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

const plain = "smtp-password-FAKE-NOT-REAL"

func TestRoundTrip(t *testing.T) {
	e := mustNew(t, newMemStore(), kekA)
	ctx := ctxFor(communeA)
	sealed, err := e.Seal(ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(plain)) {
		t.Fatal("ciphertext contains the plaintext")
	}
	got, err := e.Open(ctx, sealed, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Lo()) != plain {
		t.Fatal("round trip changed the value")
	}
	again, err := e.Seal(ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of one value are identical — the nonce is not random")
	}
}

func TestOneDEKPerCommune(t *testing.T) {
	store := newMemStore()
	e := mustNew(t, store, kekA)
	for i := 0; i < 3; i++ {
		if _, err := e.Seal(ctxFor(communeA), secret.Secret(plain), aad); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Seal(ctxFor(communeB), secret.Secret(plain), aad); err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 2 {
		t.Fatalf("want one DEK per commune (2), got %d", len(store.rows))
	}
	if bytes.Equal(store.rows[communeA].Wrapped, store.rows[communeB].Wrapped) {
		t.Fatal("two communes share a wrapped DEK")
	}
}

func TestWrongTenantFails(t *testing.T) {
	e := mustNew(t, newMemStore(), kekA)
	sealed, err := e.Seal(ctxFor(communeA), secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	// B has no DEK yet.
	if _, err := e.Open(ctxFor(communeB), sealed, aad); !errors.Is(err, ErrNoDEK) {
		t.Fatalf("commune B opened commune A's secret, or failed wrongly: %v", err)
	}
	// B has its own DEK: A's ciphertext still does not open.
	if _, err := e.Seal(ctxFor(communeB), secret.Secret("other"), aad); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Open(ctxFor(communeB), sealed, aad); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("want ErrOpenFailed opening A's secret as B, got %v", err)
	}
}

func TestWrappedDEKMovedToAnotherCommuneFails(t *testing.T) {
	store := newMemStore()
	e := mustNew(t, store, kekA)
	sealed, err := e.Seal(ctxFor(communeA), secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	store.rows[communeB] = store.rows[communeA] // a copied row, or a store that lost its tenant filter
	if _, err := e.Open(ctxFor(communeB), sealed, aad); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("A's wrapped DEK opened under B: %v", err)
	}
}

func TestWrongAADFails(t *testing.T) {
	e := mustNew(t, newMemStore(), kekA)
	ctx := ctxFor(communeA)
	sealed, err := e.Seal(ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Open(ctx, sealed, []byte("portal_config/api_key/01JTESTROW0000000000000000")); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("want ErrOpenFailed for another row's AAD, got %v", err)
	}
}

func TestWrongKEKFails(t *testing.T) {
	store := newMemStore()
	sealed, err := mustNew(t, store, kekA).Seal(ctxFor(communeA), secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mustNew(t, store, kekB).Open(ctxFor(communeA), sealed, aad); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("want ErrUnknownKey with a different KEK, got %v", err)
	}
}

func TestRotation(t *testing.T) {
	store := newMemStore()
	ctx := ctxFor(communeA)
	oldEnv := mustNew(t, store, kekA)
	sealedOld, err := oldEnv.Seal(ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	oldID := store.rows[communeA].KEKID

	// Deploy 1: new KEK prepended. The old DEK still opens; a NEW commune's DEK uses the new KEK.
	rotated := mustNew(t, store, kekB, kekA)
	if _, err := rotated.Open(ctx, sealedOld, aad); err != nil {
		t.Fatalf("the old KEK must still open after rotation: %v", err)
	}
	if _, err := rotated.Seal(ctxFor(communeB), secret.Secret(plain), aad); err != nil {
		t.Fatal(err)
	}
	if store.rows[communeB].KEKID != rotated.CurrentKEKID() || rotated.CurrentKEKID() == oldID {
		t.Fatal("a new DEK must be wrapped under the NEWEST KEK")
	}

	// Re-wrap: the DEK moves to the new KEK; the data is not re-encrypted.
	done, err := rotated.RewrapDEK(ctx)
	if err != nil || !done {
		t.Fatalf("RewrapDEK = %v, %v; want true, nil", done, err)
	}
	if store.rows[communeA].KEKID != rotated.CurrentKEKID() {
		t.Fatal("RewrapDEK did not move the DEK to the newest KEK")
	}
	if done, err := rotated.RewrapDEK(ctx); err != nil || done {
		t.Fatalf("second RewrapDEK = %v, %v; want false, nil", done, err)
	}

	// Deploy 2: old KEK dropped. Data sealed before the rotation still opens.
	newOnly := mustNew(t, store, kekB)
	got, err := newOnly.Open(ctx, sealedOld, aad)
	if err != nil || string(got.Lo()) != plain {
		t.Fatalf("data sealed before the rotation must open with the new KEK alone: %v", err)
	}
	if _, err := mustNew(t, store, kekA).Open(ctx, sealedOld, aad); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("the dropped KEK must no longer open the re-wrapped DEK, got %v", err)
	}
}

func TestRewrapLosesRaceIsReported(t *testing.T) {
	store := newMemStore()
	ctx := ctxFor(communeA)
	if _, err := mustNew(t, store, kekA).Seal(ctx, secret.Secret(plain), aad); err != nil {
		t.Fatal(err)
	}
	racing := &racingStore{memStore: store, other: mustNew(t, store, kekC, kekA)}
	e := mustNew(t, racing, kekB, kekA)
	if _, err := e.RewrapDEK(ctx); !errors.Is(err, ErrDEKChanged) {
		t.Fatalf("want ErrDEKChanged when another run re-wrapped first, got %v", err)
	}
}

// racingStore re-wraps the DEK through another Envelope between GetDEK and ReplaceDEK.
type racingStore struct {
	*memStore
	other *Envelope
}

func (r *racingStore) ReplaceDEK(ctx context.Context, oldDEK, newDEK WrappedDEK) error {
	if _, err := r.other.RewrapDEK(ctx); err != nil {
		return err
	}
	return r.memStore.ReplaceDEK(ctx, oldDEK, newDEK)
}

func TestMissingKEKRefuses(t *testing.T) {
	e, err := New(nil, newMemStore())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("New with no KEK: want ErrNotConfigured, got %v", err)
	}
	ctx := ctxFor(communeA)
	if _, err := e.Seal(ctx, secret.Secret(plain), aad); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil Envelope Seal: want ErrNotConfigured, got %v", err)
	}
	if _, err := e.Open(ctx, []byte("x"), aad); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil Envelope Open: want ErrNotConfigured, got %v", err)
	}
	if _, err := e.RewrapDEK(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil Envelope RewrapDEK: want ErrNotConfigured, got %v", err)
	}
	if !strings.Contains(ErrNotConfigured.Error(), "SECRET_ENCRYPTION_KEYS") {
		t.Fatal("the refusal must name the variable")
	}
}

func TestInvalidKEKRefused(t *testing.T) {
	if _, err := New([]secret.Secret{secret.Secret("short-FAKE")}, newMemStore()); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("short KEK: want ErrInvalidKey, got %v", err)
	}
	if _, err := New([]secret.Secret{kekA, kekA}, newMemStore()); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("duplicate KEK: want ErrInvalidKey, got %v", err)
	}
	if _, err := New([]secret.Secret{kekA}, nil); err == nil {
		t.Fatal("nil DEKStore must be refused")
	}
}

func TestTamperedFails(t *testing.T) {
	store := newMemStore()
	e := mustNew(t, store, kekA)
	ctx := ctxFor(communeA)
	sealed, err := e.Seal(ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	for i := range sealed {
		bad := bytes.Clone(sealed)
		bad[i] ^= 0x01
		if _, err := e.Open(ctx, bad, aad); !errors.Is(err, ErrOpenFailed) {
			t.Fatalf("byte %d flipped: want ErrOpenFailed, got %v", i, err)
		}
	}
	if _, err := e.Open(ctx, sealed[:len(sealed)-1], aad); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("truncated: want ErrOpenFailed, got %v", err)
	}

	// Tampered wrapped DEK: last byte (tag), and a KEKID column that disagrees with the header.
	good := store.rows[communeA]
	bad := WrappedDEK{KEKID: good.KEKID, Wrapped: bytes.Clone(good.Wrapped)}
	bad.Wrapped[len(bad.Wrapped)-1] ^= 0x01
	store.rows[communeA] = bad
	if _, err := e.Open(ctx, sealed, aad); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("tampered wrapped DEK: want ErrOpenFailed, got %v", err)
	}
	store.rows[communeA] = WrappedDEK{KEKID: "00000000", Wrapped: good.Wrapped}
	if _, err := e.Open(ctx, sealed, aad); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("KEKID column disagreeing with the header: want ErrOpenFailed, got %v", err)
	}
}

func TestMissingTenantRefuses(t *testing.T) {
	store := newMemStore()
	e := mustNew(t, store, kekA)
	ctx := context.Background()
	if _, err := e.Seal(ctx, secret.Secret(plain), aad); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("Seal without a commune: want tenant.ErrNoTenant, got %v", err)
	}
	if _, err := e.Open(ctx, []byte("x"), aad); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("Open without a commune: want tenant.ErrNoTenant, got %v", err)
	}
	if _, err := e.RewrapDEK(ctx); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("RewrapDEK without a commune: want tenant.ErrNoTenant, got %v", err)
	}
	if len(store.rows) != 0 {
		t.Fatal("a DEK was created with no commune in the context")
	}
}

func TestEmptyAADRefuses(t *testing.T) {
	e := mustNew(t, newMemStore(), kekA)
	if _, err := e.Seal(ctxFor(communeA), secret.Secret(plain), nil); !errors.Is(err, ErrEmptyAAD) {
		t.Fatalf("want ErrEmptyAAD, got %v", err)
	}
	if _, err := e.Open(ctxFor(communeA), []byte("x"), nil); !errors.Is(err, ErrEmptyAAD) {
		t.Fatalf("want ErrEmptyAAD, got %v", err)
	}
}

func TestConcurrentFirstSealConverges(t *testing.T) {
	store := newMemStore()
	e := mustNew(t, store, kekA)
	ctx := ctxFor(communeA)
	var wg sync.WaitGroup
	sealed := make([][]byte, 16)
	for i := range sealed {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := e.Seal(ctx, secret.Secret(plain), aad)
			if err != nil {
				t.Error(err)
			}
			sealed[i] = s
		}()
	}
	wg.Wait()
	for i, s := range sealed {
		if _, err := e.Open(ctx, s, aad); err != nil {
			t.Fatalf("seal %d does not open — two DEKs were used for one commune: %v", i, err)
		}
	}
}

func TestNeverRenders(t *testing.T) {
	e := mustNew(t, newMemStore(), kekA)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q"} {
		out := fmt.Sprintf(verb, e)
		if strings.Contains(out, "kek-A") || strings.Contains(out, fmt.Sprintf("%x", []byte(kekA))) {
			t.Fatalf("%s printed KEK material: %s", verb, out)
		}
	}
}
