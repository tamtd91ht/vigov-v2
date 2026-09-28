package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The three reads behind ResolveOrgUnitNames, ResolveTaskBlocLabels and ResolveLiveOrgUnitCodes
// (identity.proto). All three are keyed lookups by values the caller already holds, scoped to the
// commune the context carries: Scoped.Query binds `tenant_id = $1` from the context (rule 1,
// invariant 5), so another commune's key matches no row and is ABSENT.
//
// TWO PREDICATES, AND THEY MUST NOT BE MERGED:
//
//	NamesByID, LabelsByCode   DISPLAY reads for an archival printout. They do NOT filter
//	                          `deleted_at`, on purpose — a task still pointing at a removed unit or
//	                          bloc must print what it holds. Same discipline as can_bo_ten.go.
//	LiveIDsByCode             a DECISION read: the caller writes the returned id. It filters with
//	                          liveOrgUnit, the org chart's predicate, shared with LiveIDs so the
//	                          import and an ordinary assignment accept exactly the same units.
//
// Every method answers an empty key list with nothing and without a round trip — never "every
// row". The per-call ceilings are contract facts and are enforced in internal/grpc.

// NamesByID reads the names behind several `bo_phan` ids, REMOVED UNITS INCLUDED, in one round
// trip. `(tenant_id, id)` is the primary key, so each id matches at most one row.
func (s *BoPhanStore) NamesByID(ctx context.Context, ids []string) ([]domain.OrgUnitName, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `id, ten, deleted_at IS NULL`, "bo_phan", "AND id = ANY($2)", ids)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: tra tên theo lô: %w", err)
	}
	defer rows.Close()

	out := make([]domain.OrgUnitName, 0, len(ids))
	for rows.Next() {
		var n domain.OrgUnitName
		if err := rows.Scan(&n.ID, &n.Name, &n.Live); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng tên: %w", err)
		}
		out = append(out, n)
	}
	// Checked: a connection lost mid-result ends the loop like a complete read, and a truncated
	// answer prints real units as blanks on an archival register.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt lô tên: %w", err)
	}
	return out, nil
}

// LiveIDsByCode translates `bo_phan.ma` codes into the ids of the LIVE units carrying them.
//
// liveOrgUnit IS THE PREDICATE, the same constant LiveIDs uses — the contract says the two must never
// diverge. `(tenant_id, ma)` is UNIQUE across removed rows too (migration 0001:86), so a code maps to
// at most one row and a removed unit's code is simply absent, never re-pointed.
func (s *BoPhanStore) LiveIDsByCode(ctx context.Context, codes []string) ([]domain.OrgUnitCodeMatch, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `ma, id`, "bo_phan", "AND ma = ANY($2) AND "+liveOrgUnit, codes)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: tra bộ phận theo mã: %w", err)
	}
	defer rows.Close()

	out := make([]domain.OrgUnitCodeMatch, 0, len(codes))
	for rows.Next() {
		var m domain.OrgUnitCodeMatch
		if err := rows.Scan(&m.Ma, &m.ID); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng mã bộ phận: %w", err)
		}
		out = append(out, m)
	}
	// Checked: a truncated answer would reject real import rows as "unknown unit".
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt mã bộ phận: %w", err)
	}
	return out, nil
}

// LabelsByCode reads the labels behind several task-bloc codes, REMOVED ROWS INCLUDED, in one round
// trip. `(tenant_id, ma)` is UNIQUE across removed rows (migration 0005:335), so each code matches at
// most one row. `dang_dung` is deliberately not read: a switched-off bloc is still live.
func (s *KhoiNhiemVuStore) LabelsByCode(ctx context.Context, codes []string) ([]domain.TaskBlocLabel, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `ma, nhan, deleted_at IS NULL`, bangKhoiNhiemVu, "AND ma = ANY($2)", codes)
	if err != nil {
		return nil, fmt.Errorf("khoi_nhiem_vu: tra nhãn theo lô: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TaskBlocLabel, 0, len(codes))
	for rows.Next() {
		var l domain.TaskBlocLabel
		if err := rows.Scan(&l.Ma, &l.Label, &l.Live); err != nil {
			return nil, fmt.Errorf("khoi_nhiem_vu: đọc dòng nhãn: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("khoi_nhiem_vu: duyệt lô nhãn: %w", err)
	}
	return out, nil
}
