package store

import (
	"context"
	"database/sql"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// markPublished sets migration 0025's mark on one own app, as SetMiniAppPublication would.
func markPublished(t *testing.T, db *sql.DB, appIDs ...string) {
	t.Helper()
	for _, id := range appIDs {
		if _, err := db.Exec(`UPDATE mini_app SET published_at = now(), published_by = 'VH-00001' WHERE app_id = $1`, id); err != nil {
			t.Fatalf("mark %s published: %v", id, err)
		}
	}
}

// CommuneLaunchStore against a real PostgreSQL — skipped without VIGOV_TEST_DSN. What only the database
// can show: the read is bound to the commune in ctx (another commune's own app never appears), a
// switched-off own app is not live, and an own app NOT marked published on Zalo (migration 0025, ADR
// 0070 bổ sung 13/10/2026) is never a QR candidate.
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
	// Nothing marked yet — the state 0025 leaves every row in: no own app, the QR falls back to shared.
	got, err := s.CommuneLaunch(ctxA)
	if err != nil || !got.Active || len(got.LaunchCode) != 12 || len(got.LiveOwnAppIDs) != 0 {
		t.Fatalf("commune A unmarked: %+v %v, want active, a code, and NO own app", got, err)
	}

	// Marked: the running one is chosen; the switched-off one stays out even when marked.
	markPublished(t, db, "2001", "2002", "2003")
	got, err = s.CommuneLaunch(ctxA)
	if err != nil || len(got.LiveOwnAppIDs) != 1 || got.LiveOwnAppIDs[0] != "2001" {
		t.Fatalf("commune A: %+v %v, want only its running, published own app", got, err)
	}

	ctxB := tenant.Into(context.Background(), tenant.ID(ulidB))
	got, err = s.CommuneLaunch(ctxB)
	if err != nil || got.Active || len(got.LiveOwnAppIDs) != 1 || got.LiveOwnAppIDs[0] != "2003" {
		t.Fatalf("commune B: %+v %v, want inactive with its own app only", got, err)
	}

	// A second running own app in A, unmarked, is not a candidate; marked, it is reported, never chosen.
	themMiniApp(t, db, "2004", "rieng", ulidA, true)
	if got, err := s.CommuneLaunch(ctxA); err != nil || len(got.LiveOwnAppIDs) != 1 {
		t.Fatalf("second running, unmarked: %+v %v", got, err)
	}
	markPublished(t, db, "2004")
	if got, err := s.CommuneLaunch(ctxA); err != nil || len(got.LiveOwnAppIDs) != 2 {
		t.Fatalf("two running, both marked: %+v %v", got, err)
	}
}
