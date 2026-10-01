package crosstenant

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// The lock keys are the only logic here without a database: the scheduler's lock and each commune's
// run lock must never coincide, or a tick would block a manual run (and the reverse) for no reason.
// What the advisory locks DO is PostgreSQL's — no DSN reaches this build, so it is not exercised here.
func TestPortalSyncLockKeysDoNotCollide(t *testing.T) {
	a := tenant.ID("01JA" + strings.Repeat("A", 22))
	b := tenant.ID("01JB" + strings.Repeat("B", 22))
	keys := map[int64]string{SchedulerLockKey(): "scheduler"}
	for name, k := range map[string]int64{"A": CommuneLockKey(a), "B": CommuneLockKey(b)} {
		if prev, ok := keys[k]; ok {
			t.Fatalf("lock key of %s equals %s's", name, prev)
		}
		keys[k] = name
	}
	if CommuneLockKey(a) != CommuneLockKey(a) {
		t.Fatal("a commune's lock key is not stable")
	}
}

// The due query is identifiers only and filters what the scheduler owes: enabled, scheduled, due.
func TestDueCommunesQueryShape(t *testing.T) {
	for _, want := range []string{"SELECT tenant_id FROM portal_sync_settings", "is_enabled", "interval_hours > 0",
		"last_run_at IS NULL", "make_interval(hours => interval_hours) <= now()"} {
		if !strings.Contains(dueCommunesQuery, want) {
			t.Errorf("due query lacks %q", want)
		}
	}
}
