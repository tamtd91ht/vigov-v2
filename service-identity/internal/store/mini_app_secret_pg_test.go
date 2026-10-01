package store

// mini_app_secret and identity's data_encryption_key (migration 0021, ADR 0066).
//
// TWO HALVES. The statement-shape test always runs and proves the SQL binds the commune and
// excludes retired rows. The rest needs a real PostgreSQL (VIGOV_TEST_DSN; harness in
// checker_pg_test.go) and SKIPS without it — the package still prints `ok`, so read -v.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

func TestMiniAppSecretStatementsAreScoped(t *testing.T) {
	for name, stmt := range map[string]string{"retire": retireMiniAppSecret, "insert": insertMiniAppSecret} {
		if !strings.Contains(stmt, "tenant_id") || !strings.Contains(stmt, "$1") {
			t.Errorf("%s does not bind the commune as $1: %s", name, stmt)
		}
	}
	if !strings.Contains(retireMiniAppSecret, "deleted_at IS NULL") {
		t.Error("retire may touch an already-retired version (the guard trigger would refuse it)")
	}
	for _, bad := range []string{"DELETE", "app_secret_sealed ="} {
		if strings.Contains(retireMiniAppSecret, bad) {
			t.Errorf("retire contains %q — a version row is immutable except for retirement", bad)
		}
	}
}

func pgEnvelope(t *testing.T, kho *store.DB) *crypto.Envelope {
	t.Helper()
	env, err := crypto.New([]secret.Secret{secret.Secret(bytes.Repeat([]byte{5}, crypto.KeyLength))},
		NewDataEncryptionKeyStore(kho))
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestMiniAppSecretVersioningPG(t *testing.T) {
	db := moKetNoi(t)
	kho := store.New(db)
	s := NewMiniAppSecretStore(kho)
	a, b := xaRieng(t)
	ctxA := tenant.Into(context.Background(), tenant.ID(a))
	ctxB := tenant.Into(context.Background(), tenant.ID(b))
	const appID = "1234567890123456789"
	aad := []byte("mini_app_secret/app_secret_sealed/" + a + "/" + appID)

	env := pgEnvelope(t, kho)
	sealed, err := env.Seal(ctxA, secret.Secret("FAKE-APP-SECRET-0000"), aad)
	if err != nil {
		t.Fatalf("seal (creates the commune's DEK in data_encryption_key): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	v1 := MiniAppSecret{ID: strings.Repeat("1", 26), AppID: appID, Sealed: sealed, SetAt: now, SetBy: "system"}
	if err := kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Insert(ctxA, tx, v1) }); err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	got, ok, err := s.Live(ctxA, appID)
	if err != nil || !ok || got.ID != v1.ID || !bytes.Equal(got.Sealed, sealed) {
		t.Fatalf("live = %+v %v %v", got, ok, err)
	}
	if plain, err := env.Open(ctxA, got.Sealed, aad); err != nil || string(plain.Lo()) != "FAKE-APP-SECRET-0000" {
		t.Fatalf("open the stored secret: %v", err)
	}
	if _, ok, _ := s.Live(ctxB, appID); ok {
		t.Fatal("commune B reads commune A's settings")
	}

	// A second live row for the same App ID is refused by (tenant_id, live_app_id).
	dup := v1
	dup.ID = strings.Repeat("2", 26)
	if err := kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Insert(ctxA, tx, dup) }); err == nil {
		t.Fatal("two live versions for one App ID")
	}

	// Retire + insert a demo-only version in one transaction.
	v2 := MiniAppSecret{ID: strings.Repeat("3", 26), AppID: appID, DemoIdentity: true, SetAt: now, SetBy: "system"}
	err = kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error {
		cur, ok, err := s.LiveForUpdate(ctxA, tx, appID)
		if err != nil || !ok {
			return errors.Join(err, errors.New("no live row to lock"))
		}
		if err := s.Retire(ctxA, tx, cur.ID, "system", "ticket:OPS-1"); err != nil {
			return err
		}
		return s.Insert(ctxA, tx, v2)
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, ok, err = s.Live(ctxA, appID)
	if err != nil || !ok || got.ID != v2.ID || got.Sealed != nil || !got.DemoIdentity {
		t.Fatalf("after replace = %+v %v %v", got, ok, err)
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM mini_app_secret WHERE tenant_id = $1 AND id = $2`,
		a, v1.ID).Scan(&by, &reason); err != nil || by != "system" || reason != "ticket:OPS-1" {
		t.Fatalf("retired row: %q %q %v", by, reason, err)
	}

	// Retiring a retired version, editing a version, deleting: all refused.
	err = kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Retire(ctxA, tx, v1.ID, "system", "ticket:OPS-2") })
	if !errors.Is(err, ErrMiniAppSecretRetired) {
		t.Fatalf("retire twice: %v", err)
	}
	if _, err := db.Exec(`UPDATE mini_app_secret SET demo_identity_enabled = false WHERE tenant_id = $1 AND id = $2`, a, v2.ID); err == nil {
		t.Fatal("a version was edited in place")
	}
	if _, err := db.Exec(`DELETE FROM mini_app_secret WHERE tenant_id = $1 AND id = $2`, a, v2.ID); err == nil {
		t.Fatal("a version was deleted")
	}
	// A row with neither secret nor demo is refused by the schema.
	bad := MiniAppSecret{ID: strings.Repeat("4", 26), AppID: "99", SetAt: now, SetBy: "system"}
	if err := kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Insert(ctxA, tx, bad) }); err == nil {
		t.Fatal("a row with no secret and demo off was accepted")
	}
}

func TestIdentityDataEncryptionKeyPG(t *testing.T) {
	db := moKetNoi(t)
	kho := store.New(db)
	d := NewDataEncryptionKeyStore(kho)
	a, _ := xaRieng(t)
	ctx := tenant.Into(context.Background(), tenant.ID(a))

	if _, err := d.GetDEK(ctx); !errors.Is(err, crypto.ErrDEKNotFound) {
		t.Fatalf("empty: %v", err)
	}
	env := pgEnvelope(t, kho)
	if _, err := env.Seal(ctx, secret.Secret("x"), []byte("aad")); err != nil {
		t.Fatal(err)
	}
	w, err := d.GetDEK(ctx)
	if err != nil || w.KEKID != env.CurrentKEKID() {
		t.Fatalf("stored DEK: %+v %v", w, err)
	}
	if err := d.CreateDEK(ctx, w); !errors.Is(err, crypto.ErrDEKExists) {
		t.Fatalf("second create: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = $2 AND actor_id = 'system'`,
		a, ActionCreateDataKey).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit entries for the DEK = %d %v", n, err)
	}
	if _, err := db.Exec(`DELETE FROM data_encryption_key WHERE tenant_id = $1`, a); err == nil {
		t.Fatal("a DEK was deleted")
	}
}
