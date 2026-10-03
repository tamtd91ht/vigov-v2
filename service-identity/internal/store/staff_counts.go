package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The read behind GET /api/v1/staff-counts — the KPI cards and the per-unit `{đang hiện}/{tổng}`
// counter of the Danh bạ cán bộ screen (12-danh-ba-can-bo.md §2, §3).
//
// NO PREDICATE IS WRITTEN HERE. Each number is defined by a read that already exists, and borrows
// that read's predicate rather than retyping it:
//
//	total      locTomTat          — the register, as GET /api/v1/staff and POST /staff/searches list it
//	published  locDanhBaCongKhai  — the public directory, as GET /api/v1/commune-staff serves it
//
// A KPI card that said "18 công khai" over a Mini App showing 17 would be a figure the commune passes
// upward and a citizen can contradict. Sharing the constant is what keeps the two from drifting: edit
// the publication predicate and this count moves with it.
//
// `locTomTat` IS UNQUALIFIED (`deleted_at`) and `locDanhBaCongKhai` IS QUALIFIED (`nd.`). Both resolve
// against the one table in FROM, which is why this statement joins nothing: a second table in scope
// would make the unqualified clause ambiguous and PostgreSQL would refuse it — loudly, not wrongly.
//
// `published` IS A SUBSET OF `total` BY CONSTRUCTION: the FILTER runs over rows the WHERE already
// admitted, and the publication predicate itself requires `deleted_at IS NULL`.
//
// GROUPED BY `bo_phan_id` EXACTLY AS THE LIST'S `unit` FILTER MATCHES IT (menhDeLocCanBo:
// `bo_phan_id = $n`), so the per-unit total is the number of rows that filter returns. A row whose
// department was since soft-deleted keeps counting under that id, because the list's filter still
// returns it; the screen shows counts only for the units in its dropdown. NULL and the empty string are
// one bucket — the register reads both as "no department" (cotTomTat coalesces bo_phan_id to empty).
//
// UNBOUNDED ON PURPOSE: one row per distinct department id of ONE commune, which is the size of its
// organisation chart, not of its register.
//
// `nd.tenant_id = $1` is the Scoped.QueryJoin contract (rule 1, invariant 5). QueryJoin and not Query
// only because the borrowed publication predicate names the alias `nd`.
const staffCountsQuery = `
SELECT coalesce(nd.bo_phan_id, ''),
       count(*),
       count(*) FILTER (WHERE true
` + locDanhBaCongKhai + `)
FROM nguoi_dung nd
WHERE nd.tenant_id = $1
` + locTomTat + `
GROUP BY 1
ORDER BY 1`

// StaffCounts counts the register of the commune in the context, whole and per department.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: QueryJoin binds it to $1 from the context.
// Counts only — no name, number or id of a person is read, so nothing here can reach a log (rule 3).
func (s *CanBoStore) StaffCounts(ctx context.Context) (domain.StaffCounts, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, staffCountsQuery)
	if err != nil {
		return domain.StaffCounts{}, fmt.Errorf("can_bo: đếm danh bạ: %w", err)
	}
	defer rows.Close()

	// make(..., 0, ...) and not nil: `departments` marshals as [] for a commune with nobody in a unit.
	out := domain.StaffCounts{Departments: make([]domain.DepartmentStaffTally, 0, 16)}
	for rows.Next() {
		var (
			department       string
			total, published int64
		)
		if err := rows.Scan(&department, &total, &published); err != nil {
			return domain.StaffCounts{}, fmt.Errorf("can_bo: đếm danh bạ: đọc dòng: %w", err)
		}
		tally := domain.StaffTally{Total: int(total), Published: int(published)}
		out.Total += tally.Total
		out.Published += tally.Published
		if department == "" {
			out.NoDepartment = tally
			continue
		}
		out.Departments = append(out.Departments, domain.DepartmentStaffTally{DepartmentID: department, StaffTally: tally})
	}
	if err := rows.Err(); err != nil {
		return domain.StaffCounts{}, fmt.Errorf("can_bo: đếm danh bạ: %w", err)
	}
	return out, nil
}
