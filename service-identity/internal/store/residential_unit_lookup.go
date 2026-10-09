package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The two reads behind ResolveActiveResidentialUnits and ResolveResidentialUnitNames (identity.proto,
// ADR 0088). Both are keyed lookups by ids the caller already holds, scoped to the commune the context
// carries: Scoped.Query binds `tenant_id = $1` from the context (rule 1, invariant 5), so another
// commune's id matches no row and is ABSENT.
//
// TWO PREDICATES, AND THEY MUST NOT BE MERGED:
//
//	ActiveUnitsByID   a DECISION read — the caller writes the id onto a petition. `deleted_at IS NULL
//	                  AND dang_dung`: a unit taken out of use is a choice no picker offers (ADR 0059 §2),
//	                  so it is refused here too.
//	UnitNamesByID     a DISPLAY read for records already stored. Filters NOTHING but the commune: an
//	                  out-of-use unit is live, a soft-deleted one comes back flagged. An archival record
//	                  must print the hamlet it holds.
//
// Both answer an empty id list with nothing and without a round trip — never "every unit". The
// per-call ceiling is a contract fact and is enforced in internal/grpc.

// activeResidentialUnit is the "may be written" predicate. ANY PICKER OFFERING UNITS FOR A PETITION
// MUST USE THE SAME ONE (identity.proto, ResolveActiveResidentialUnits) — change both in one commit.
const activeResidentialUnit = "deleted_at IS NULL AND dang_dung"

// ActiveUnitsByID returns the requested units that are live and in use in this commune, with their
// names. `(tenant_id, id)` is the primary key, so each id matches at most one row.
func (s *ThonToDanPhoStore) ActiveUnitsByID(ctx context.Context, ids []string) ([]domain.ActiveResidentialUnit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `id, ten`, "thon_to_dan_pho",
		"AND id = ANY($2) AND "+activeResidentialUnit, ids)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: kiểm đơn vị đang dùng theo lô: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ActiveResidentialUnit, 0, len(ids))
	for rows.Next() {
		var u domain.ActiveResidentialUnit
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng đơn vị đang dùng: %w", err)
		}
		out = append(out, u)
	}
	// Checked: a connection lost mid-result ends the loop like a complete read, and a truncated answer
	// refuses a real hamlet as "no longer receiving petitions".
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt đơn vị đang dùng: %w", err)
	}
	return out, nil
}

// UnitNamesByID reads the names behind several unit ids, OUT-OF-USE AND SOFT-DELETED UNITS INCLUDED,
// in one round trip. Live is `deleted_at IS NULL` and nothing else.
func (s *ThonToDanPhoStore) UnitNamesByID(ctx context.Context, ids []string) ([]domain.ResidentialUnitName, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `id, ten, deleted_at IS NULL`, "thon_to_dan_pho", "AND id = ANY($2)", ids)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: tra tên theo lô: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ResidentialUnitName, 0, len(ids))
	for rows.Next() {
		var n domain.ResidentialUnitName
		if err := rows.Scan(&n.ID, &n.Name, &n.Live); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng tên: %w", err)
		}
		out = append(out, n)
	}
	// Checked: a truncated answer prints real hamlets as blanks on an archival record.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt lô tên: %w", err)
	}
	return out, nil
}
