package store

import (
	"context"
	"fmt"
)

// The read behind ResolveAssignableStaff — "which of these codes may be handed NEW work in this
// commune, today". The SIXTH staff read in this package, and deliberately NOT a sixth predicate.
//
// IT REUSES locChonNguoi, THE PICKER'S PREDICATE, VERBATIM. The contract (identity.proto,
// ResolveAssignableStaff) requires the two to be identical: a code the picker offers and this read
// refuses is a dropdown whose choice always fails, and a code this read accepts and the picker hides
// is a body-crafted assignment the screen could never produce. Sharing the constant is what makes
// "whoever changes one changes both" true by construction instead of by memory — a copy here would
// be a copy that drifts.
//
// THE ONLY COLUMN IS `ma`. The answer is a subset of the codes asked for, so nothing else is needed,
// and a value never read cannot leak through a handler that forgot to drop it.
//
// THE COMMUNE IS $1, bound by Scoped.Query from the context (rule 1, invariant 5). A code of another
// commune matches no row and is ABSENT — indistinguishable from unknown, deleted, account-less or
// locked, which is exactly the one answer the contract wants.
const cotGiaoViec = `ma`

// GiaoViecDuoc returns the subset of ma that satisfies the picker's predicate in the commune the
// context carries. Each code at most once: `UNIQUE (tenant_id, ma)` (migration 0001) is a plain key
// covering soft-deleted rows too, so one code matches at most one row.
//
// AN EMPTY LIST READS NOTHING AND ANSWERS NOTHING — never "everybody assignable". `= ANY('{}')`
// would already match no row; the early return makes the property independent of that.
//
// The 50-code ceiling is a CONTRACT fact and is enforced where the contract is — internal/grpc.
func (s *CanBoStore) GiaoViecDuoc(ctx context.Context, ma []string) ([]string, error) {
	if len(ma) == 0 {
		return nil, nil
	}

	// pgx binds a []string to a Postgres text[] directly — the same fact TenTheoNhieuMa relies on.
	rows, err := s.db.For(ctx).Query(ctx, cotGiaoViec, "nguoi_dung", "AND ma = ANY($2) "+locChonNguoi, ma)
	if err != nil {
		return nil, fmt.Errorf("can_bo: kiểm mã giao việc được: %w", err)
	}
	defer rows.Close()

	ra := make([]string, 0, len(ma))
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng mã giao việc được: %w", err)
		}
		ra = append(ra, m)
	}
	// rows.Err() IS CHECKED: a connection lost mid-result ends the loop like a complete read, and a
	// truncated answer here would refuse real, assignable people with nothing reporting it.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt mã giao việc được: %w", err)
	}
	return ra, nil
}
