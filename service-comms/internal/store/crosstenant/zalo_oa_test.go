package crosstenant

import (
	"strings"
	"testing"
)

// The OA webhook's lookup is identifiers only and LIVE rows only: a retired OA (Ngắt OA, or replaced)
// must never route an event to the commune that used to hold it. No DSN reaches this build, so the
// statement's shape is what is checked here; PostgreSQL itself is not exercised.
func TestTenantByLiveOAIDQueryShape(t *testing.T) {
	for _, want := range []string{"SELECT tenant_id FROM zalo_commune_oa", "oa_id = $1", "retired_at IS NULL"} {
		if !strings.Contains(tenantByLiveOAID, want) {
			t.Errorf("query lacks %q: %s", want, tenantByLiveOAID)
		}
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "oa_name", "*"} {
		if strings.Contains(tenantByLiveOAID, forbidden) {
			t.Errorf("the cross-commune lookup must return the commune id only, found %q", forbidden)
		}
	}
}
