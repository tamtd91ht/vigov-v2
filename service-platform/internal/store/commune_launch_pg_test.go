package store

import (
	"context"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// CommuneLaunchStore against a real PostgreSQL — skipped without VIGOV_TEST_DSN. What only the database
// can show: the read is bound to the commune in ctx (another commune's own app never appears), and a
// switched-off own app is not live.
func TestPgCommuneLaunch(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewCommuneLaunchStore(corestore.New(db))
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Cũ", false)
	themMiniApp(t, db, "2001", "rieng", ulidA, true)
	themMiniApp(t, db, "2002", "rieng", ulidA, false)
	themMiniApp(t, db, "2003", "rieng", ulidB, true)

	ctxA := tenant.Into(context.Background(), tenant.ID(ulidA))
	got, err := s.CommuneLaunch(ctxA)
	if err != nil || !got.Active || len(got.LaunchCode) != 12 ||
		len(got.LiveOwnAppIDs) != 1 || got.LiveOwnAppIDs[0] != "2001" {
		t.Fatalf("commune A: %+v %v, want active, a code, and only its running own app", got, err)
	}

	ctxB := tenant.Into(context.Background(), tenant.ID(ulidB))
	got, err = s.CommuneLaunch(ctxB)
	if err != nil || got.Active || len(got.LiveOwnAppIDs) != 1 || got.LiveOwnAppIDs[0] != "2003" {
		t.Fatalf("commune B: %+v %v, want inactive with its own app only", got, err)
	}

	// A second running own app in A is reported, never chosen between.
	themMiniApp(t, db, "2004", "rieng", ulidA, true)
	if got, err := s.CommuneLaunch(ctxA); err != nil || len(got.LiveOwnAppIDs) != 2 {
		t.Fatalf("two running: %+v %v", got, err)
	}
}
