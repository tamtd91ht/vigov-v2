package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The statements behind removing one org unit (app.SoDoToChuc.Delete; menu Cấu hình §12.4).
//
// THE COMMUNE IS $1 IN EVERY STATEMENT and no method takes one (rule 1, invariants 4 and 5). The
// write takes the caller's transaction, so the soft delete and its audit entry share it (rule 6).

// staffCountedInUnit IS THE ONE DEFINITION OF "a member of staff sitting in the unit", shared by the
// org chart's staff count (truyVanBoPhanKemSoCanBo) and the delete refusal below. Two copies would
// drift, and the day they did the screen would print "0 cán bộ" against a unit the delete refuses —
// or, worse, the other way round. Who is counted and why: domain.BoPhan.SoCanBo.
const staffCountedInUnit = `nd.deleted_at IS NULL AND nd.dang_hoat_dong`

// localHoldingsCols are the two counts identity can see itself, for the row aliased `bp`.
//
// EACH SUBQUERY REPEATS THE COMMUNE (`… .tenant_id = bp.tenant_id`), not merely the unit id — the
// reason truyVanBoPhanKemSoCanBo gives: ids joined alone would count another commune's rows wherever
// they collide. A CHILD is a LIVE row whose `cha_id` is this unit; a soft-deleted child holds
// nothing and does not block.
const localHoldingsCols = `(SELECT count(*) FROM nguoi_dung nd
         WHERE nd.tenant_id = bp.tenant_id AND nd.bo_phan_id = bp.id AND ` + staffCountedInUnit + `),
       (SELECT count(*) FROM bo_phan con
         WHERE con.tenant_id = bp.tenant_id AND con.cha_id = bp.id AND con.deleted_at IS NULL)`

// LiveForDelete reads one LIVE unit of this commune with its staff and child counts, OUTSIDE any
// transaction — the read that decides 404 and whose counts go into a 409, before the use case asks
// petitions and documents. Nothing is locked: the use case re-counts under the row lock.
//
// ErrKhongTimThayBoPhan for an invented id, a soft-deleted unit and another commune's unit — one
// answer, so none can be told apart by trying (rule 4, forbidden #2).
//
// COLUMNS ARE READ BY POSITION; `ma`/`ten` and the two counts are adjacent pairs of one type.
func (s *BoPhanStore) LiveForDelete(ctx context.Context, id string) (domain.BoPhan, domain.OrgUnitHoldings, error) {
	if id == "" {
		return domain.BoPhan{}, domain.OrgUnitHoldings{}, ErrKhongTimThayBoPhan
	}
	const stmt = `SELECT bp.id, bp.ma, bp.ten, coalesce(bp.cha_id,''), bp.thu_tu,
       ` + localHoldingsCols + `
FROM bo_phan bp
WHERE bp.tenant_id = $1 AND bp.id = $2 AND bp.deleted_at IS NULL`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, id)
	if err != nil {
		return domain.BoPhan{}, domain.OrgUnitHoldings{}, fmt.Errorf("bo_phan: đọc bộ phận để xoá: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.BoPhan{}, domain.OrgUnitHoldings{}, fmt.Errorf("bo_phan: đọc bộ phận để xoá: %w", err)
		}
		return domain.BoPhan{}, domain.OrgUnitHoldings{}, ErrKhongTimThayBoPhan
	}
	var bp domain.BoPhan
	var h domain.OrgUnitHoldings
	if err := rows.Scan(&bp.ID, &bp.Ma, &bp.Ten, &bp.ChaID, &bp.ThuTu, &h.Staff, &h.ChildUnits); err != nil {
		return domain.BoPhan{}, domain.OrgUnitHoldings{}, fmt.Errorf("bo_phan: đọc dòng bộ phận để xoá: %w", err)
	}
	return bp, h, nil
}

// LocalHoldings re-counts staff and child units INSIDE the delete's transaction, after the caller
// has locked the unit (KhoaBoPhan). Only the two local fields of the result are filled.
//
// WHY THE LOCK MAKES THIS COUNT HOLD, for the two paths that can add to it:
//
//   - a CHILD: SoDoToChuc.Them and the move in Sua lock the would-be parent FOR UPDATE and refuse a
//     deleted one, so they either commit before this count (and are counted) or wait and then see
//     the unit deleted.
//   - a MEMBER OF STAFF: the staff write path checks the unit only through the foreign key, whose
//     KEY SHARE lock waits on our FOR UPDATE — so a staff write that commits first is counted. One
//     that waits and runs AFTER our commit is NOT refused by the key, which does not read
//     `deleted_at`. That gap is in the staff write path, not here, and is reported with this card.
func (s *BoPhanStore) LocalHoldings(ctx context.Context, tx *store.ScopedTx, id string) (domain.OrgUnitHoldings, error) {
	if id == "" {
		return domain.OrgUnitHoldings{}, ErrKhongTimThayBoPhan
	}
	const stmt = `SELECT ` + localHoldingsCols + `
FROM bo_phan bp
WHERE bp.tenant_id = $1 AND bp.id = $2`

	var h domain.OrgUnitHoldings
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan(&h.Staff, &h.ChildUnits)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OrgUnitHoldings{}, ErrKhongTimThayBoPhan
	}
	if err != nil {
		return domain.OrgUnitHoldings{}, fmt.Errorf("bo_phan: đếm cán bộ và bộ phận con: %w", err)
	}
	return h, nil
}

// SoftDelete marks one live unit deleted: `deleted_at`, `deleted_by` (the remover's STAFF CODE,
// rule 6 invariant 8) and `delete_reason` (rule 7, invariant 1). Nothing else changes.
//
// `ma` IS UNTOUCHED AND STAYS TAKEN FOR EVER: `UNIQUE (tenant_id, ma)` is not partial, and
// MaCungGoc counts deleted rows, so a code once issued is never issued again (rule 7, invariant 3).
//
// `AND deleted_at IS NULL` is what makes a second DELETE unable to overwrite who removed the unit or
// why; zero rows is ErrKhongTimThayBoPhan.
func (s *BoPhanStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, deletedBy, reason string) error {
	if id == "" {
		return ErrKhongTimThayBoPhan
	}
	if deletedBy == "" {
		// No fallback (rule 6, invariant 8): a removal nobody can be named for is refused.
		return fmt.Errorf("bo_phan: xoá mềm không có mã người xoá")
	}
	const stmt = `UPDATE bo_phan SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, deletedBy, reason)
	if err != nil {
		return fmt.Errorf("bo_phan: xoá mềm bộ phận: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("bo_phan: đếm dòng đã xoá: %w", err)
	}
	if n == 0 {
		return ErrKhongTimThayBoPhan
	}
	return nil
}
