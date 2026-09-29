package store

// The count behind CountOrgUnitHoldings (proto/vigov/documents/v1/documents.proto): how many OPEN
// incoming documents one org unit still holds in the commune of the context. SQL, and nothing else.
//
// "Held" is `bo_phan_dang_giu_id` — the ĐANG GIỮ column. "Open" is openPredicate, i.e. NOT
// domain.IncomingDocumentStatus.IsFinished() via domain.FinishedIncomingStatuses — the SAME predicate
// the dashboard's open figure uses, so a status added to IsFinished reaches this answer in the same
// edit. Routing history (`lich_su_chuyen_van_ban`) is NOT read: it says who held it WHEN.
//
// Commune $1 from the context (rule 1, invariant 5); live rows only (rule 7, invariant 2).

import (
	"context"
	"errors"
	"fmt"
)

// ErrOrgUnitIDBlank refuses a blank unit id before any statement runs: comparing the holder column
// with the empty string is not a question anybody means to ask. The gRPC handler refuses it first.
var ErrOrgUnitIDBlank = errors.New("van_ban_den: mã bộ phận rỗng")

// CountOpenHeldByOrgUnit counts the live incoming documents, not finished, whose
// `bo_phan_dang_giu_id` is orgUnitID, in the commune of ctx. An id of another commune counts zero.
//
// AN ERROR IS NEVER ZERO: an aggregate with no GROUP BY yields one row, and none is a driver fault —
// a zero here lets a delete through.
func (s *IncomingDocumentStore) CountOpenHeldByOrgUnit(ctx context.Context, orgUnitID string) (int, error) {
	if orgUnitID == "" {
		return 0, ErrOrgUnitIDBlank
	}
	var args []any
	bind := newBinder(&args)
	tail := "AND deleted_at IS NULL AND bo_phan_dang_giu_id = " + bind(orgUnitID) + " AND " + openPredicate(bind)

	rows, err := s.db.For(ctx).Query(ctx, "count(*)", "van_ban_den", tail, args...)
	if err != nil {
		return 0, fmt.Errorf("van_ban_den: đếm văn bản bộ phận đang giữ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("van_ban_den: đếm văn bản bộ phận đang giữ: %w", err)
		}
		return 0, errors.New("van_ban_den: đếm văn bản bộ phận đang giữ: câu đếm không trả dòng nào")
	}
	var n int64
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("van_ban_den: đếm văn bản bộ phận đang giữ: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("van_ban_den: đếm văn bản bộ phận đang giữ: %w", err)
	}
	return int(n), nil
}
