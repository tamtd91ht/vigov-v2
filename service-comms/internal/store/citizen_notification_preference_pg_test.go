package store

import (
	"context"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// citizen_notification_preferences against a REAL PostgreSQL (migration 0027). SKIPPED WITHOUT
// VIGOV_TEST_DSN (shared harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN means
// this file COMPILES, nothing more.
//
// WHAT ONLY A REAL SERVER CAN SHOW: that `tenant_id = $1` really is in every statement this store sends.
// THE SAME citizen_id in two communes is the case that makes a missing predicate visible — the identity
// service mints one id per person, and a person who moved (or a merged commune) has rows in both. Dropping
// the commune from Get would read the other commune's switch; from ForUpdate, lock and report it; from the
// upsert's conflict target, overwrite it.
func TestPgCitizenNotificationPreferenceIsPerCommune(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	handle := pkgstore.New(db)
	s := NewCitizenNotificationPreferenceStore(handle)
	ctx1, ctx2 := ctxXa(tenant.ID(c1)), ctxXa(tenant.ID(c2))
	// ONE citizen id in both communes; unique per run so reruns never meet an old row.
	citizen := "01JCITIZEN" + c1[len(c1)-16:]

	for i, ctx := range []context.Context{ctx1, ctx2} {
		if p, err := s.Get(ctx, citizen); err != nil || !p.ZaloEnabled || p.Stored {
			t.Fatalf("commune %d before any act: %+v %v — want the default (on, no row)", i+1, p, err)
		}
	}

	// Commune 1: the citizen turns notifications OFF.
	if err := handle.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return s.Set(ctx1, tx, citizen, false)
	}); err != nil {
		t.Fatalf("Set c1: %v", err)
	}
	if p, err := s.Get(ctx1, citizen); err != nil || p.ZaloEnabled || !p.Stored {
		t.Fatalf("c1 after its own write: %+v %v", p, err)
	}
	// Commune 2 still reads its own absence: ON, no row — not commune 1's OFF.
	if p, err := s.Get(ctx2, citizen); err != nil || !p.ZaloEnabled || p.Stored {
		t.Fatalf("c2 read commune 1's row: %+v %v", p, err)
	}
	// ForUpdate in commune 2 neither sees nor locks commune 1's row.
	if err := handle.For(ctx2).Tx(ctx2, func(tx *pkgstore.ScopedTx) error {
		p, err := s.ForUpdate(ctx2, tx, citizen)
		if err != nil {
			return err
		}
		if !p.ZaloEnabled || p.Stored {
			t.Errorf("c2 ForUpdate saw commune 1's row: %+v", p)
		}
		// Commune 2: the same citizen states ON explicitly — a row of its own.
		return s.Set(ctx2, tx, citizen, true)
	}); err != nil {
		t.Fatalf("c2 tx: %v", err)
	}

	// Each commune reads only its own row, and commune 2's upsert did not overwrite commune 1's.
	if p, err := s.Get(ctx1, citizen); err != nil || p.ZaloEnabled || !p.Stored {
		t.Fatalf("c1 after c2's write: %+v %v — commune 2's upsert reached commune 1's row", p, err)
	}
	if p, err := s.Get(ctx2, citizen); err != nil || !p.ZaloEnabled || !p.Stored {
		t.Fatalf("c2 after its own write: %+v %v", p, err)
	}
	// Commune 1 flips back ON; commune 2's row is untouched (two rows, each its own value).
	if err := handle.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		p, err := s.ForUpdate(ctx1, tx, citizen)
		if err != nil {
			return err
		}
		if p.ZaloEnabled || !p.Stored {
			t.Errorf("c1 ForUpdate = %+v, want its own stored OFF", p)
		}
		return s.Set(ctx1, tx, citizen, true)
	}); err != nil {
		t.Fatalf("c1 second tx: %v", err)
	}

	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM citizen_notification_preferences
		WHERE citizen_id = $1 AND tenant_id IN ($2, $3)`, citizen, c1, c2).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 2 {
		t.Fatalf("rows for the citizen across the two communes = %d, want 2 (one each)", rows)
	}

	// The guard: a DELETE would turn an opt-out back on without the citizen's act — refused.
	if _, err := db.Exec(`DELETE FROM citizen_notification_preferences WHERE tenant_id = $1 AND citizen_id = $2`,
		c1, citizen); err == nil {
		t.Fatal("DELETE of a preference row was accepted")
	}
}
