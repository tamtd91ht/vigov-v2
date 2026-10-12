package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Only a violation of 0024's unique index is cured by drawing another code; any other unique
// violation (the tenant id, a host) must surface, not loop.
func TestIsLaunchCodeCollision(t *testing.T) {
	wrap := func(pg *pgconn.PgError) error { return fmt.Errorf("operator registry: insert tenant: %w", pg) }
	if !isLaunchCodeCollision(wrap(&pgconn.PgError{Code: "23505", ConstraintName: launchCodeConstraint})) {
		t.Error("0024's unique violation not recognised")
	}
	for name, err := range map[string]error{
		"tenant pkey":    wrap(&pgconn.PgError{Code: "23505", ConstraintName: "tenant_pkey"}),
		"shape CHECK":    wrap(&pgconn.PgError{Code: "23514", ConstraintName: "tenant_launch_code_shape"}),
		"not a pg error": errors.New("conn reset"),
		"nil":            nil,
	} {
		if isLaunchCodeCollision(err) {
			t.Errorf("%s: treated as a launch code collision", name)
		}
	}
}

// Runs without a database: the mapping of a driver error to the registry's sentinels. 0007's CHECK
// is a refusal working as designed (422 reserved_domain), never a 500 — but only THAT check; any
// other CHECK stays the unexpected error it is.
func TestDomainWriteErrorMapping(t *testing.T) {
	wrap := func(pg *pgconn.PgError) error { return fmt.Errorf("scoped exec: %w", pg) }
	for name, c := range map[string]struct {
		err  error
		want error
	}{
		"0007 CHECK":   {wrap(&pgconn.PgError{Code: "23514", ConstraintName: reservedHostConstraint}), domain.ErrCommuneHostReserved},
		"unique":       {wrap(&pgconn.PgError{Code: "23505", ConstraintName: "tenant_domain_pkey"}), ErrDomainTaken},
		"other CHECK":  {wrap(&pgconn.PgError{Code: "23514", ConstraintName: "tenant_domain_host_thuong"}), nil},
		"not pg error": {errors.New("conn reset"), nil},
	} {
		got := domainWriteError(c.err, "insert domain")
		if c.want != nil {
			if !errors.Is(got, c.want) {
				t.Errorf("%s: %v, want %v", name, got, c.want)
			}
			continue
		}
		if errors.Is(got, domain.ErrCommuneHostReserved) || errors.Is(got, ErrDomainTaken) || !errors.Is(got, c.err) {
			t.Errorf("%s: %v — must stay the original error, wrapped", name, got)
		}
	}
}
