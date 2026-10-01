package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

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
