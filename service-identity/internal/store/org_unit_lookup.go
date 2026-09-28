package store

import (
	"context"
	"fmt"
)

// The two reads behind ResolveLiveOrgUnits and ResolveStaffOrgUnits (identity.proto). Both answer
// with `bo_phan.id` values only — no name, no slug, nothing about a person — so nothing a caller did
// not already hold can leave through them.

// liveOrgUnit is THE predicate of a live unit, shared by both reads below. It is the org chart's own
// (truyVanBoPhanKemSoCanBo: `bp.deleted_at IS NULL`), and the contract says the two must never
// diverge: a unit the assignment box offers and a write refuses is a choice that always fails.
const liveOrgUnit = `deleted_at IS NULL`

// LiveIDs returns the subset of ids that are live `bo_phan` rows of the commune the context carries.
//
// THE COMMUNE IS $1, bound by Scoped.Query from the context (rule 1, invariant 5): another commune's
// id matches no row and is ABSENT — indistinguishable from unknown or soft-deleted, which is the one
// answer the contract wants. Each id at most once: (tenant_id, id) is the primary key.
//
// AN EMPTY LIST READS NOTHING AND ANSWERS NOTHING — never "every live unit". The 50-id ceiling is a
// contract fact and is enforced where the contract is — internal/grpc.
func (s *BoPhanStore) LiveIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `id`, "bo_phan", "AND id = ANY($2) AND "+liveOrgUnit, ids)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: kiểm bộ phận còn hiệu lực: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng bộ phận còn hiệu lực: %w", err)
		}
		out = append(out, id)
	}
	// Checked: a connection lost mid-result ends the loop like a complete read, and a truncated answer
	// here refuses a real unit with nothing reporting it.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt bộ phận còn hiệu lực: %w", err)
	}
	return out, nil
}

// staffOrgUnitsQuery joins ONE staff record to its unit, both in the commune of $1.
//
// THE JOIN REPEATS THE COMMUNE (`bp.tenant_id = nd.tenant_id`), not merely the unit id — joining on
// id alone would match another commune's unit wherever ids collide (rule 1, and the QueryJoin
// contract in core/store). An INNER join on purpose: a person with no unit (`bo_phan_id` NULL) or
// whose unit was removed yields no row, which is the empty answer the contract defines for both.
//
// `nd.deleted_at IS NULL` is the register's predicate. Account and lock state are NOT conditions:
// this answers "which unit is this record in today", not "may this person be handed work".
const staffOrgUnitsQuery = `
SELECT bp.id
FROM nguoi_dung nd
JOIN bo_phan bp
  ON bp.tenant_id = nd.tenant_id
 AND bp.id        = nd.bo_phan_id
 AND bp.` + liveOrgUnit + `
WHERE nd.tenant_id = $1
  AND nd.ma = $2
  AND nd.deleted_at IS NULL`

// UnitsOfStaff returns the live org unit(s) of the staff record with business code ma — at most one
// today, since `nguoi_dung.bo_phan_id` is a single column and (tenant_id, ma) is unique.
//
// AN EMPTY CODE READS NOTHING. The handler already refuses it; the early return makes "no code"
// unable to become a statement independent of that.
func (s *BoPhanStore) UnitsOfStaff(ctx context.Context, ma string) ([]string, error) {
	if ma == "" {
		return nil, nil
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, staffOrgUnitsQuery, ma)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: đọc bộ phận của cán bộ: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng bộ phận của cán bộ: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt bộ phận của cán bộ: %w", err)
	}
	return out, nil
}
