package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The read behind ResolveStaffNames (ADR 0034) — the FOURTH staff read path in this package, and
// the only one that deliberately returns rows the other three hide.
//
// IT SITS BESIDE truyVanCanBoLo AND MUST NEVER BE MERGED INTO IT. The two differ in the one
// predicate that makes each of them correct, and whichever side lost its condition would lose it
// silently:
//
//	truyVanCanBoLo    BatchGetStaff — "is this a staff record of this commune TODAY". Keeps
//	                  `nd.deleted_at IS NULL`. Widen it and the assignee dropdown starts offering
//	                  people who were removed from the directory, with nothing turning red.
//	this one          ResolveStaffNames — "what name goes beside this line of an archival record".
//	                  Drops that condition ON PURPOSE: a record removed from the directory is the
//	                  case this query exists to answer.
//
// THE COMMUNE PREDICATE IS NOT RELAXED WITH IT. `nd.tenant_id = $1` stays, and it is the whole of
// what keeps this from becoming a cross-commune read: a `ma` belonging to another commune matches
// no row and comes back ABSENT, indistinguishable from "does not exist" (ADR 0012, decision 2).
// Rule 1 does not bend for an archival read — a leak between two communes is a leak between two
// public authorities whatever the reason for the query.
//
// THE KEY IS `nd.ma`, NOT `nd.id`. `audit_log.actor_id` stores the business code (rule 6,
// invariant 8), so keying this on the internal id would resolve nothing that has ever been
// written — a lookup that compiles, passes its tests against fabricated ids, and renders a blank
// where a person's name belongs. `UNIQUE (tenant_id, ma)` (migration 0001) is what makes one code
// match at most one row.
//
// NO JOIN, and that is the point of the field list. The role, the department and the position are
// absent because `StaffName` declares none of them and having nothing in hand is a stronger
// guarantee than remembering not to send it — the same discipline domain.CanBoVaiTro states.
const truyVanTenCanBoTheoMa = `
SELECT nd.ma, nd.ho_ten, nd.deleted_at IS NULL
FROM nguoi_dung nd
WHERE nd.tenant_id = $1
  AND nd.ma = ANY($2)`

// TenTheoNhieuMa reads the names behind several staff codes in ONE round trip, in the commune the
// context carries.
//
// THE COMMUNE IS $1 AND IT IS NOT A PARAMETER. Scoped.QueryJoin binds it from the context (rule 1,
// invariant 4), so there is no call site that could pass the wrong one and no signature a later
// caller could talk into a cross-commune read.
//
// AN EMPTY LIST READS NOTHING AND ANSWERS NOTHING — never "everybody". This is the single line a
// later edit could turn a lookup into a roster with, which is why the contract states it rather
// than leaving it to an implementation (ADR 0034). `= ANY('{}')` would already match no row; the
// early return makes the property independent of that.
//
// FEWER ROWS THAN CODES IS NORMAL and is not an error: a code of another commune, or of nobody at
// all, is simply absent. The caller maps by `ma`.
//
// The 200-code ceiling is a CONTRACT fact and is enforced where the contract is — internal/grpc —
// not here.
func (s *CanBoStore) TenTheoNhieuMa(ctx context.Context, ma []string) ([]domain.TenCanBo, error) {
	if len(ma) == 0 {
		return nil, nil
	}

	// pgx binds a []string to a Postgres text[] directly, so ANY($2) needs no driver helper — the
	// same fact TheoNhieuID and Checker.AllowsNhieu already rely on.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanTenCanBoTheoMa, ma)
	if err != nil {
		return nil, fmt.Errorf("can_bo: tra tên theo lô mã: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.TenCanBo, 0, len(ma))
	for rows.Next() {
		var t domain.TenCanBo
		// NEVER LOG A SCANNED ROW. `HoTen` is personal data and this loop holds a commune's staff
		// names; a `%+v` here puts them into the log pipeline, from where they cannot be recalled
		// (rule 3, forbidden #1).
		if err := rows.Scan(&t.Ma, &t.HoTen, &t.ConTrongDanhBa); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng tên: %w", err)
		}
		ra = append(ra, t)
	}
	// rows.Err() IS CHECKED, and dropping it is the classic silent truncation: a connection lost
	// halfway through the result set ends the loop exactly like a complete read, and the caller
	// renders the missing names as blanks on an archival record.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt lô tên: %w", err)
	}
	return ra, nil
}
