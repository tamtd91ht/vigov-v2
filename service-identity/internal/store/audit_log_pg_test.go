package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// core/audit.Log against a real PostgreSQL, on identity's migrated `audit_log` (ADR 0054).
//
// WHY HERE: core has no PostgreSQL driver, and what core/audit/read_test.go cannot prove is that
// PostgreSQL evaluates the statement the way its clauses read — the half-open range, the default
// exclusion, the keyset walk across a tie, the commune bound to $1. Skipped without VIGOV_TEST_DSN.

func seedAuditEntry(t *testing.T, db *sql.DB, commune, actor, kind, ip, action, subject string, at time.Time) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO audit_log
		(tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'{"k":"v"}')`,
		commune, actor, kind, ip, action, subject, at); err != nil {
		t.Fatalf("seed audit_log: %v", err)
	}
}

func TestAuditLogReadPG(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	t0 := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

	seedAuditEntry(t, db, a, "CB-1", "staff", "10.0.0.1", "them_bo_phan", "BP-1", t0.Add(-2*time.Hour))
	seedAuditEntry(t, db, a, "CB-1", "staff", "10.0.0.1", "sua_bo_phan", "BP-1", t0) // tie on `at`
	seedAuditEntry(t, db, a, "CB-2", "staff", "10.0.0.2", "sua_bo_phan", "BP-2", t0)
	seedAuditEntry(t, db, a, "cd-internal", "citizen", "203.0.113.9", "dang_ky", "CD-1", t0.Add(-time.Hour))
	seedAuditEntry(t, db, a, "CB-9", "staff", "10.0.0.9", audit.ActionReadLog, "CB-9", t0.Add(-time.Minute))
	seedAuditEntry(t, db, b, "CB-B", "staff", "10.0.0.3", "sua_bo_phan", "BP-1", t0) // another commune

	log := audit.NewLog(pkgstore.New(db))
	reader := audit.Actor{ID: "CB-READER", Kind: "staff", IP: "10.0.0.7"}
	ctx := tenant.Into(context.Background(), tenant.ID(a))

	// Default: commune A only, read entries hidden, citizen code/IP blanked, at DESC then id DESC.
	res, err := log.Read(ctx, reader, audit.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 4 {
		t.Fatalf("%d entries, want 4 (commune B and the read entry excluded): %+v", len(res.Items), res.Items)
	}
	if res.Items[0].ActorCode != "CB-2" || res.Items[1].ActorCode != "CB-1" {
		t.Errorf("tie on `at` not broken by id DESC: %+v", res.Items[:2])
	}
	for _, e := range res.Items {
		if e.ActorKind == "citizen" && (e.ActorCode != "" || e.ActorIP != "") {
			t.Errorf("citizen entry exposes %q/%q", e.ActorCode, e.ActorIP)
		}
		if e.Delta == nil {
			t.Error("delta lost")
		}
	}

	// Half-open: `to` equal to t0 excludes the two entries AT t0.
	res, err = log.Read(ctx, reader, audit.Query{
		From: t0.Add(-3 * time.Hour).Format(time.RFC3339), To: t0.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Errorf("[from, to) returned %d, want 2 — `to` must be exclusive", len(res.Items))
	}

	// Explicit action shows the read entries — including the ones the reads above just wrote.
	res, err = log.Read(ctx, reader, audit.Query{Action: audit.ActionReadLog})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 3 { // the seeded one + the two reads above
		t.Errorf("%d read entries, want 3 (every read leaves one)", len(res.Items))
	}

	// Keyset walk, one row per page, across the tie: every entry once, none repeated.
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 10; i++ {
		res, err = log.Read(ctx, reader, audit.Query{Limit: "1", Cursor: cursor, Actor: "CB-1"})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Items {
			if seen[e.Action] {
				t.Fatalf("entry %s repeated across pages", e.Action)
			}
			seen[e.Action] = true
		}
		if !res.HasMore {
			break
		}
		cursor = res.NextCursor
	}
	if len(seen) != 2 {
		t.Errorf("walk saw %d of CB-1's 2 entries", len(seen))
	}

	// An actor filter never matches a citizen's internal id.
	res, err = log.Read(ctx, reader, audit.Query{Actor: "cd-internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 0 {
		t.Error("actor filter matched a citizen's internal id")
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE tablename = 'audit_log' AND indexname = 'audit_log_by_time'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("migration 0015's index audit_log_by_time is missing")
	}
}
