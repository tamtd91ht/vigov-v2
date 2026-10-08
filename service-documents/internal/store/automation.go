package store

// The automation jobs' reads of SỔ VĂN BẢN ĐẾN and SỔ ĐƠN THƯ (ADR 0058 §1: each runner counts its own
// books, never another service's). SQL, and nothing else. Same design as
// service-petitions/internal/store/automation.go.
//
// THE SAME THREE GUARANTEES AS THE REST OF THIS PACKAGE: the commune is $1 from the context on every
// table of the statement (rule 1, invariant 5), soft-deleted rows are excluded (rule 7, invariant 2),
// and every value is a bound parameter. There is NO now() here: the run's `now` is identity's
// `claimed_at`, and a second clock inside one run puts a document on neither list or on both
// (identity.proto, ClaimDueAutomationRuns).
//
// WHAT IS READ: id, issued number, stored deadline, holders — never `trich_yeu`, `co_quan_ban_hanh` or
// `so_ky_hieu`, and never a letter's `sender_*`, `summary` or `letter_type` (free text and personal
// data; rule 3, ADR 0078 #4). A notice is composed from exactly these columns.

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

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

// --- citizen letters --------------------------------------------------------------------------------

// letterHoldStartColumn is incomingHoldStartColumn's derivation over the letter log: `holding_unit_id`
// and `assignee_code` are written ONLY by the routing act (CitizenLetterStore.UpdateHolder, and Insert
// when routed at booking), and that act appends a `luan-chuyen` row copying both in the same
// transaction (app.CitizenLetters.Route / Book). So the hold began at the FIRST routing row naming
// (this unit, nobody) after the LAST routing row naming anything else. Fallback `updated_at` — errs
// late, never early. `assignee_code` is NULL, never ”, on both tables (0006's CHECKs). $2 is the
// routing kind, bound from domain.LetterLogRouting so the SQL holds no second copy of the word.
const letterHoldStartColumn = `CASE WHEN c.holding_unit_id IS NOT NULL AND c.assignee_code IS NULL THEN COALESCE(
	(SELECT min(lg.at) FROM citizen_letter_log lg
	  WHERE lg.tenant_id = $1 AND lg.letter_id = c.id AND lg.kind = $2
	    AND lg.to_unit_id = c.holding_unit_id AND lg.assignee_code IS NULL
	    AND lg.at > COALESCE(
	      (SELECT max(l2.at) FROM citizen_letter_log l2
	        WHERE l2.tenant_id = $1 AND l2.letter_id = c.id AND l2.kind = $2
	          AND (l2.to_unit_id IS DISTINCT FROM c.holding_unit_id OR l2.assignee_code IS NOT NULL)),
	      '-infinity'::timestamptz)),
	c.updated_at) END`

// automationLetterQuery reads every live, NOT FINISHED letter carrying at least one deadline. "Not
// finished" is the complement of domain's Finished(), bound from $3 — the set the transition table
// draws, never a second list. Which of the two deadlines applies is decided in Go by ActiveDueAt, so a
// letter whose only deadline belongs to a phase it has left is read and then dropped, not mis-reminded.
func automationLetterQuery() (string, []any) {
	args := []any{string(domain.LetterLogRouting)}
	var open []string
	for _, s := range domain.LetterStatuses {
		if !s.Finished() {
			args = append(args, string(s))
			open = append(open, "$"+strconv.Itoa(len(args)+1))
		}
	}
	return `SELECT c.id, c.year, c.number, c.status, c.processing_due_at, c.resolution_due_at,
	COALESCE(c.holding_unit_id, ''), COALESCE(c.assignee_code, ''), ` + letterHoldStartColumn + `
	FROM citizen_letter c
	WHERE c.tenant_id = $1 AND c.deleted_at IS NULL AND c.status IN (` + strings.Join(open, ", ") + `)
	  AND (c.processing_due_at IS NOT NULL OR c.resolution_due_at IS NOT NULL)
	ORDER BY c.id`, args
}

// OpenLettersForAutomation reads, for the commune in ctx, every open letter whose CURRENT phase has a
// deadline a clerk set (domain.CitizenLetter.ActiveDueAt). A letter left "Không đặt" is not a record
// here at all — no reminder, no escalation, no hold report: nothing is ever sent from a default
// (ADR 0079 lô 5 Q18). Unbounded for the reason OpenIncomingForAutomation gives.
func (s *CitizenLetterStore) OpenLettersForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	stmt, args := automationLetterQuery()
	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("citizen_letter: đọc cho việc nền: %w", err)
	}
	defer rows.Close()
	var out []domain.AutomationRecord
	for rows.Next() {
		var (
			r                     domain.AutomationRecord
			year, number          int
			status                string
			procDue, resDue, hold sql.NullTime
		)
		if err := rows.Scan(&r.ID, &year, &number, &status, &procDue, &resDue, &r.OrgUnitID, &r.AssigneeMa, &hold); err != nil {
			return nil, fmt.Errorf("citizen_letter: đọc cho việc nền: đọc dòng: %w", err)
		}
		l := domain.CitizenLetter{Status: domain.LetterStatus(status),
			ProcessingDueAt: nullTime(procDue), ResolutionDueAt: nullTime(resDue)}
		due := l.ActiveDueAt()
		if due.IsZero() {
			continue
		}
		r.Code = domain.LetterAuditSubject(year, number)
		r.Deadline = due.UTC()
		if hold.Valid {
			r.HoldStartedAt = hold.Time.UTC()
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("citizen_letter: đọc cho việc nền: %w", err)
	}
	return out, nil
}
