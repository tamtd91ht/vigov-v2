package store

// The automation jobs' read of SỔ VĂN BẢN ĐẾN (ADR 0058 §1: each runner counts its own books, never
// another service's). SQL, and nothing else. Same design as service-petitions/internal/store/automation.go.
//
// THE SAME THREE GUARANTEES AS THE REST OF THIS PACKAGE: the commune is $1 from the context on every
// table of the statement (rule 1, invariant 5), soft-deleted rows are excluded (rule 7, invariant 2),
// and every value is a bound parameter. There is NO now() here: the run's `now` is identity's
// `claimed_at`, and a second clock inside one run puts a document on neither list or on both
// (identity.proto, ClaimDueAutomationRuns).
//
// WHAT IS READ: id, issued number, stored deadline, holders — never `trich_yeu`, `co_quan_ban_hanh` or
// `so_ky_hieu` (free text; rule 3). A notice is composed from exactly these columns.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// incomingHoldStartColumn derives WHEN THE UNIT BEGAN HOLDING a document with nobody named — the
// anchor ResolveUnassignedHoldInstants counts from. THERE IS NO COLUMN FOR IT, and none is added: the
// routing timeline already holds the fact. `bo_phan_dang_giu_id` and `can_bo_xu_ly_ma` are written ONLY
// by the routing act (VanBanDenStore.ChuyenBoPhan), and that act appends a `lich_su_chuyen_van_ban`
// row copying both, in the same transaction (app.VanBanDen.Chuyen). So the hold began at the FIRST
// entry naming (this unit, nobody) after the LAST entry naming anything else — a re-routing to the same
// unit with nobody named does not restart the episode.
//
// FALLBACK `cap_nhat_luc` when no such entry exists (a holder written without an entry — not possible
// through this service today): the row's last change is the latest instant the hold can have begun, so
// the report errs LATE, never early, and never from the booking time, which the contract names as the
// wrong anchor.
//
// NULL unless the document is held by a unit with nobody named.
const incomingHoldStartColumn = `CASE WHEN v.bo_phan_dang_giu_id IS NOT NULL AND NULLIF(v.can_bo_xu_ly_ma, '') IS NULL THEN COALESCE(
	(SELECT min(l.thoi_diem) FROM lich_su_chuyen_van_ban l
	  WHERE l.tenant_id = $1 AND l.van_ban_den_id = v.id
	    AND l.den_bo_phan_id = v.bo_phan_dang_giu_id AND NULLIF(l.can_bo_xu_ly_ma, '') IS NULL
	    AND l.thoi_diem > COALESCE(
	      (SELECT max(l2.thoi_diem) FROM lich_su_chuyen_van_ban l2
	        WHERE l2.tenant_id = $1 AND l2.van_ban_den_id = v.id
	          AND (l2.den_bo_phan_id IS DISTINCT FROM v.bo_phan_dang_giu_id OR NULLIF(l2.can_bo_xu_ly_ma, '') IS NOT NULL)),
	      '-infinity'::timestamptz)),
	v.cap_nhat_luc) END`

// automationIncomingQuery reads every live, OPEN incoming document. "Open" is openPredicate — the SAME
// predicate the dashboard's "Chưa xử lý xong" figure and its drill-down use — bound from $2, so a
// reminder speaks about exactly the documents /tong-quan counts.
func automationIncomingQuery() (string, []any) {
	var args []any
	open := openPredicate(newBinder(&args))
	return `SELECT v.id, v.nam, v.so_vao_so, v.han_xu_ly_xong,
	COALESCE(v.bo_phan_dang_giu_id, ''), COALESCE(v.can_bo_xu_ly_ma, ''), ` + incomingHoldStartColumn + `
	FROM van_ban_den v
	WHERE v.tenant_id = $1 AND v.deleted_at IS NULL AND v.` + open + `
	ORDER BY v.id`, args
}

// OpenIncomingForAutomation reads every live, open incoming document of the commune in ctx.
//
// ONE READ PER RUN, NOT PER DOCUMENT: the job compares each row with the instants identity answered, in
// memory. Unbounded on purpose — a limit would silently leave late documents unreported; the set is a
// commune's OPEN documents, not its archive.
func (s *VanBanDenStore) OpenIncomingForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	stmt, args := automationIncomingQuery()
	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("van_ban_den: đọc cho việc nền: %w", err)
	}
	defer rows.Close()
	var out []domain.AutomationRecord
	for rows.Next() {
		var (
			r        domain.AutomationRecord
			nam, so  int
			deadline sql.NullTime
			hold     sql.NullTime
		)
		if err := rows.Scan(&r.ID, &nam, &so, &deadline, &r.OrgUnitID, &r.AssigneeMa, &hold); err != nil {
			return nil, fmt.Errorf("van_ban_den: đọc cho việc nền: đọc dòng: %w", err)
		}
		r.Code = domain.MaVanBanDen(nam, so)
		if deadline.Valid {
			r.Deadline = deadline.Time.UTC()
		}
		if hold.Valid {
			r.HoldStartedAt = hold.Time.UTC()
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("van_ban_den: đọc cho việc nền: %w", err)
	}
	return out, nil
}
