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
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/migrations"
)

func TestMiniAppSecretStatementsAreScoped(t *testing.T) {
	for name, stmt := range map[string]string{"retire": retireMiniAppSecret, "insert": insertMiniAppSecret} {
		if !strings.Contains(stmt, "tenant_id") || !strings.Contains(stmt, "$1") {
			t.Errorf("%s does not bind the commune as $1: %s", name, stmt)
		}
	}
	// New versions never turn the demo identity back on (owner decision 05/10/2026).
	if !strings.Contains(insertMiniAppSecret, "$4, false, $5") {
		t.Errorf("insert does not write demo_identity_enabled = false: %s", insertMiniAppSecret)
	}
	// The status list carries a boolean, never the sealed bytes (operator.proto MiniAppSecretVersion).
	if !strings.Contains(miniAppSecretStatusColumns, "app_secret_sealed IS NOT NULL") ||
		strings.Contains(strings.ReplaceAll(miniAppSecretStatusColumns, "app_secret_sealed IS NOT NULL", ""), "app_secret_sealed") {
		t.Errorf("status columns read the sealed bytes: %s", miniAppSecretStatusColumns)
	}
	if strings.Contains(miniAppSecretColumns, "demo_identity_enabled") {
		t.Errorf("the settings read still loads the retired demo switch: %s", miniAppSecretColumns)
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

	// Retire + insert the next version in one transaction.
	sealed2, err := env.Seal(ctxA, secret.Secret("FAKE-APP-SECRET-0001"), aad)
	if err != nil {
		t.Fatal(err)
	}
	v2 := MiniAppSecret{ID: strings.Repeat("3", 26), AppID: appID, Sealed: sealed2, SetAt: now, SetBy: "VH-00001"}
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
	if err != nil || !ok || got.ID != v2.ID || !bytes.Equal(got.Sealed, sealed2) {
		t.Fatalf("after replace = %+v %v %v", got, ok, err)
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM mini_app_secret WHERE tenant_id = $1 AND id = $2`,
		a, v1.ID).Scan(&by, &reason); err != nil || by != "system" || reason != "ticket:OPS-1" {
		t.Fatalf("retired row: %q %q %v", by, reason, err)
	}
	var demo bool
	if err := db.QueryRow(`SELECT demo_identity_enabled FROM mini_app_secret WHERE tenant_id = $1 AND id = $2`,
		a, v2.ID).Scan(&demo); err != nil || demo {
		t.Fatalf("new version demo_identity_enabled = %v %v, want false", demo, err)
	}

	// The status list: live rows of THIS commune only, no bytes, ordered by app_id.
	st, err := s.LiveStatuses(ctxA)
	if err != nil || len(st) != 1 || st[0].ID != v2.ID || st[0].AppID != appID || !st[0].SecretSet ||
		st[0].SetBy != "VH-00001" || !st[0].SetAt.Equal(now) {
		t.Fatalf("statuses A = %+v %v", st, err)
	}
	if st, err := s.LiveStatuses(ctxB); err != nil || len(st) != 0 {
		t.Fatalf("commune B lists commune A's settings: %+v %v", st, err)
	}

	// Retiring a retired version, editing a version, deleting: all refused.
	err = kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Retire(ctxA, tx, v1.ID, "system", "ticket:OPS-2") })
	if !errors.Is(err, ErrMiniAppSecretRetired) {
		t.Fatalf("retire twice: %v", err)
	}
	if _, err := db.Exec(`UPDATE mini_app_secret SET demo_identity_enabled = true WHERE tenant_id = $1 AND id = $2`, a, v2.ID); err == nil {
		t.Fatal("a version was edited in place")
	}
	if _, err := db.Exec(`DELETE FROM mini_app_secret WHERE tenant_id = $1 AND id = $2`, a, v2.ID); err == nil {
		t.Fatal("a version was deleted")
	}
	// A version with no secret is refused before the database ...
	bad := MiniAppSecret{ID: strings.Repeat("4", 26), AppID: "99", SetAt: now, SetBy: "system"}
	if err := kho.For(ctxA).Tx(ctxA, func(tx *store.ScopedTx) error { return s.Insert(ctxA, tx, bad) }); !errors.Is(err, ErrMiniAppSecretNoSecret) {
		t.Fatalf("a version with no secret: %v, want ErrMiniAppSecretNoSecret", err)
	}
	// ... and by the schema, which 0024 left unchanged.
	if _, err := db.Exec(`INSERT INTO mini_app_secret (tenant_id, id, app_id, set_at, set_by) VALUES ($1, $2, '99', now(), 'system')`,
		a, strings.Repeat("4", 26)); err == nil {
		t.Fatal("a row with no secret and demo off was accepted")
	}
}

// Migration 0024 against the real guard trigger: a LIVE demo-only row (written the pre-05/10/2026
// way, by raw SQL) is retired by the system with ONE trail entry; a live row WITH a secret is left
// alone even if its demo flag is on; running the file again changes nothing (idempotent).
func TestMigration0024RetiresDemoOnlyRowsPG(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	const demoOnly, withSecret = "1111111111", "2222222222"
	ins := func(tid, id, appID string, sealed []byte) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO mini_app_secret (tenant_id, id, app_id, app_secret_sealed, demo_identity_enabled, set_at, set_by)
			VALUES ($1, $2, $3, $4, true, now(), 'system')`, tid, id, appID, sealed); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	ins(a, strings.Repeat("5", 26), demoOnly, nil)
	ins(a, strings.Repeat("6", 26), withSecret, bytes.Repeat([]byte{1}, 40))
	ins(b, strings.Repeat("7", 26), demoOnly, nil)

	body, err := fs.ReadFile(migrations.FS, "0024_retire_demo_identity_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("run %d of 0024: %v", run, err)
		}
	}

	for _, c := range []struct {
		tid, appID string
		wantLive   bool
	}{{a, demoOnly, false}, {a, withSecret, true}, {b, demoOnly, false}} {
		var live bool
		if err := db.QueryRow(`SELECT deleted_at IS NULL FROM mini_app_secret WHERE tenant_id = $1 AND app_id = $2`,
			c.tid, c.appID).Scan(&live); err != nil || live != c.wantLive {
			t.Errorf("%s/%s live = %v (%v), want %v", c.tid, c.appID, live, err, c.wantLive)
		}
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND subject = $2
			AND action = 'ngung_cau_hinh_app_rieng' AND actor_id = 'system' AND actor_kind = 'system'
			AND delta->'sau' = 'null'::jsonb AND (delta->'truoc'->>'co_secret')::boolean = false`,
			c.tid, c.appID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if want := map[bool]int{true: 0, false: 1}[c.wantLive]; n != want {
			t.Errorf("%s/%s: %d retirement entries after two runs, want %d", c.tid, c.appID, n, want)
		}
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM mini_app_secret WHERE tenant_id = $1 AND app_id = $2`,
		a, demoOnly).Scan(&by, &reason); err != nil || by != "system" || reason != "gỡ danh tính demo theo quyết định 05/10/2026" {
		t.Fatalf("retired by %q for %q (%v)", by, reason, err)
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
