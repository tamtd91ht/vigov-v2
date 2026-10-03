package crosstenant

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// The sweep's scheduler lock never coincides with the portal sync's locks.
func TestBodyImageSweepLockKeyIsItsOwn(t *testing.T) {
	a := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := BodyImageSweepLockKey()
	if k == SchedulerLockKey() || k == CommuneLockKey(a) {
		t.Fatal("the sweep's lock key equals a portal sync lock key")
	}
}

// The commune list is identifiers only and carries every clause of the per-commune candidate read:
// drop one and a commune with no work is listed — or, for NOT EXISTS, one whose files belong to a saved
// (even soft-deleted) article.
func TestAbandonedBodyImageCommunesQueryShape(t *testing.T) {
	q := abandonedBodyImageCommunesQuery
	for _, want := range []string{"SELECT DISTINCT f.tenant_id FROM stored_file f", "f.purpose = $1",
		"f.subject_type = $2", "f.status = 'ready'", "f.deleted_at IS NULL", "f.public_object_key IS NULL",
		"f.updated_at < $3",
		"NOT EXISTS (SELECT 1 FROM noi_dung_mini_app n WHERE n.tenant_id = f.tenant_id AND n.id = f.subject_id)"} {
		if !strings.Contains(q, want) {
			t.Errorf("commune query lacks %q", want)
		}
	}
	if strings.Contains(q, "n.deleted_at") {
		t.Error("the article probe must count SOFT-DELETED articles too")
	}
}
