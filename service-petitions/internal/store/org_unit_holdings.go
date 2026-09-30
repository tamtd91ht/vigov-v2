package store

// The two counts behind CountOrgUnitHoldings (proto/vigov/petitions/v1/petitions.proto): how many
// OPEN petitions and tasks one org unit still holds in the commune of the context. SQL, and nothing
// else. The predicate is the contract's, stated there once; the constants below are its only
// spelling in this service.
//
// Commune $1 from the context (rule 1, invariant 5). Live rows only (rule 7, invariant 2). The unit
// id is bound, never concatenated. Log and timeline tables are NOT read: they record who held
// something WHEN, never who holds it now.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ErrOrgUnitIDBlank refuses a blank unit id before any statement runs. Comparing `bo_phan_id` with
// the empty string would count the rows whose column holds one — a question nobody means to ask. The gRPC
// handler refuses it first with INVALID_ARGUMENT; this is the floor under it.
var ErrOrgUnitIDBlank = errors.New("org_unit_holdings: mã bộ phận rỗng")

// petitionHeldOpenCondition is "held by $2 and not finished". The three exclusions are the three
// endings — the SAME constant the overview's in-progress figure uses, so "open" cannot mean one thing
// on the dashboard and another on the delete. `da-xu-ly` and `cho-dan-xac-nhan` stay open: the
// citizen can still send the petition back to this unit.
var petitionHeldOpenCondition = `bo_phan_id = $2 AND ` + citizenReportInProgressCondition

// taskHeldOpenCondition is "held by $2 and not finished". `bo_phan_id` ONLY: since ADR 0065 NV5
// (migration 0025) the lead unit IS the assigned unit, and the retired `co_quan_chu_tri_id` is never
// read — migration 0025 filled `bo_phan_id` from it wherever `bo_phan_id` was empty, so a unit that
// was only the lead unit of a task with a different assigned unit no longer holds that task.
// `tam-dung` and `chuyen-tiep` stay open: both can return to active work.
var taskHeldOpenCondition = `bo_phan_id = $2 AND trang_thai <> '` + string(domain.HoanThanh) + `'`

// CountOpenHeldByOrgUnit counts the live, unfinished petitions whose `bo_phan_id` is orgUnitID, in
// the commune of ctx. An id of another commune counts zero: $1 is the context's commune.
func (s *PhieuPhanAnhStore) CountOpenHeldByOrgUnit(ctx context.Context, orgUnitID string) (int, error) {
	return countHeldOpen(ctx, s.db, "phieu_phan_anh", petitionHeldOpenCondition, orgUnitID)
}

// CountOpenHeldByOrgUnit counts the live, unfinished tasks whose `bo_phan_id` is orgUnitID, in the
// commune of ctx.
func (s *NhiemVuStore) CountOpenHeldByOrgUnit(ctx context.Context, orgUnitID string) (int, error) {
	return countHeldOpen(ctx, s.db, "nhiem_vu", taskHeldOpenCondition, orgUnitID)
}

// countHeldOpen runs `SELECT count(*) FROM <table> WHERE tenant_id = $1 AND deleted_at IS NULL AND
// <cond>` and reads its one row. An aggregate with no GROUP BY always yields one row; none is a
// driver fault and is an error, never a zero — a zero here lets a delete through.
func countHeldOpen(ctx context.Context, db *store.DB, table, cond, orgUnitID string) (int, error) {
	if orgUnitID == "" {
		return 0, ErrOrgUnitIDBlank
	}
	rows, err := db.For(ctx).Query(ctx, "count(*)", table, "AND deleted_at IS NULL AND "+cond, orgUnitID)
	if err != nil {
		return 0, fmt.Errorf("%s: đếm hồ sơ bộ phận đang giữ: %w", table, err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("%s: đếm hồ sơ bộ phận đang giữ: %w", table, err)
		}
		return 0, fmt.Errorf("%s: đếm hồ sơ bộ phận đang giữ: câu đếm không trả dòng nào", table)
	}
	var n int64
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("%s: đếm hồ sơ bộ phận đang giữ: đọc dòng: %w", table, err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("%s: đếm hồ sơ bộ phận đang giữ: %w", table, err)
	}
	return int(n), nil
}
