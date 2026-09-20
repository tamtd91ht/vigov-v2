package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The read behind BatchGetStaff — the inter-service lookup, and the THIRD staff read path in
// this package.
//
// WHY IT IS NOT TheoID AND NOT ChiTiet, and why merging it into either one is a defect rather
// than tidying. The three answer three different questions and therefore filter three different
// predicates; the cost of unifying them is paid by whichever side loses a condition, and neither
// loss is loud:
//
//	TheoID          SIGN-IN / session path. Three conditions — not deleted, has an account, not
//	                locked. Widen it and a directory-only person, or a locked-out former
//	                employee, holds a live principal.
//	ChiTiet         the commune's staff REGISTER, one row. One condition — not deleted. Narrow
//	                it and the 26 people of the directory vanish from the screen that lists them.
//	TheoNhieuID     this one. Same single condition as ChiTiet, plural, and returning ONLY the
//	                two fields `message Staff` declares.
//
// THE PREDICATE HERE IS ChiTiet's, ON PURPOSE, and the contract says so in its own words: an id
// is absent from BatchGetStaffResponse when it does not exist, is soft deleted, or belongs to
// another commune — and NOT when the person is locked out or has no sign-in account. A list
// screen decorating rows with a handler whose account was closed last month still has to name
// that handler; the row in the archival record did not stop existing when the account did.

// truyVanCanBoLo reads the ids the caller asked for, in the commune the context carries.
//
// THE COMMUNE IS $1 AND IT IS NOT A PARAMETER OF ANY FUNCTION HERE. Scoped.QueryJoin binds it
// from the context (rule 1, invariant 4), so an id belonging to another commune matches no row
// and simply comes back absent. That absence is indistinguishable from "does not exist", which
// is exactly what ADR 0012 decision 2 requires: telling the two apart would answer "does this
// record exist in a commune you may not read".
//
// THE JOIN CARRIES THE COMMUNE TOO — `vt.tenant_id = nd.tenant_id`, not `vt.id = nd.vai_tro_id`
// alone. Joining on id alone would pick up another commune's role wherever two ULIDs collide,
// and no test of a single commune can ever show it (Scoped.QueryJoin's own contract says this).
//
// LEFT JOIN, NOT JOIN: `nguoi_dung.vai_tro_id` is nullable and `vai_tro.deleted_at` may be set.
// An inner join would silently DROP a staff member who holds no role — turning "this person has
// no role" into "this id does not exist", which the caller reads as a deleted record and renders
// as a blank. The `coalesce` around `vt.ma` is what makes the roleless case an empty string
// instead of a NULL that fails the scan.
const truyVanCanBoLo = `
SELECT nd.id, coalesce(vt.ma, '')
FROM nguoi_dung nd
LEFT JOIN vai_tro vt
       ON vt.tenant_id = nd.tenant_id
      AND vt.id        = nd.vai_tro_id
      AND vt.deleted_at IS NULL
WHERE nd.tenant_id = $1
  AND nd.id = ANY($2)
  AND nd.deleted_at IS NULL`

// TheoNhieuID reads several staff records in ONE round trip.
//
// PLURAL WITH NO SINGULAR FORM BESIDE IT, mirroring the contract (ADR 0012, decision 2). A
// singular read looped by the caller is one query per row: invisible in a three-row test, and
// the page that stops loading belongs to the commune with the most records — the one that
// matters most.
//
// FEWER ROWS THAN IDS IS NORMAL and is not reported as an error. The caller maps by id.
//
// The caller is responsible for the 200-id ceiling and for dropping empties; this layer does not
// second-guess it, because the ceiling is a contract fact and belongs where the contract is
// enforced (internal/grpc), not in the SQL.
func (s *CanBoStore) TheoNhieuID(ctx context.Context, ids []string) ([]domain.CanBoVaiTro, error) {
	if len(ids) == 0 {
		// No query at all. `= ANY('{}')` is a valid statement that matches nothing, so this is
		// an optimisation rather than a correctness fix — but it also keeps an empty request off
		// the database entirely, which at 200+ communes on one pool is worth the two lines.
		return nil, nil
	}

	// pgx binds a []string to a Postgres text[] directly, so ANY($2) works with no driver
	// helper — the same fact store.Checker.AllowsNhieu already relies on.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanCanBoLo, ids)
	if err != nil {
		return nil, fmt.Errorf("can_bo: tra theo lô id: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CanBoVaiTro, 0, len(ids))
	for rows.Next() {
		var cb domain.CanBoVaiTro
		if err := rows.Scan(&cb.ID, &cb.VaiTroMa); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng lô: %w", err)
		}
		ra = append(ra, cb)
	}
	// rows.Err() IS CHECKED, and dropping it is the classic silent truncation: a connection lost
	// halfway through the result set ends the loop exactly like a complete read, and the caller
	// renders a short list as if it were the whole answer.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt lô: %w", err)
	}
	return ra, nil
}
