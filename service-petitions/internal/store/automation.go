package store

// The automation jobs' reads of this service's own registers (ADR 0058 §1: each runner counts its
// own books, never another service's). SQL, and nothing else.
//
// THE SAME THREE GUARANTEES AS THE REST OF THIS PACKAGE: the commune is $1 from the context (rule 1,
// invariant 5 — every table of every join constrained to it), soft-deleted rows are excluded (rule 7,
// invariant 2), and every instant is a bound parameter. There is NO now() here: the run's `now` is
// identity's `claimed_at`, and a second clock inside one run puts a record on neither list or on
// both (identity.proto, ClaimDueAutomationRuns).
//
// WHAT IS READ: codes, statuses, stored deadlines, holders — no content, no reporter, no title
// (rule 3). A notice is composed from exactly these columns.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// taskHoldStartColumn derives WHEN THE UNIT BEGAN HOLDING a task with nobody named — the anchor
// ResolveUnassignedHoldInstants counts from. THERE IS NO COLUMN FOR IT, and none is added: the
// timeline already holds the fact. Every act appends a `nhat_ky_nhiem_vu` row copying the holder AT
// THAT STEP (app.GhiNhiemVu.ghiNhatKy), so the hold began at the FIRST entry naming (this unit,
// nobody) after the LAST entry naming anything else. Creation writes an entry too.
//
// FALLBACK `cap_nhat_luc` when no such entry exists (rows booked before the timeline, or a holder
// written without an entry): the row's last change is the latest instant the hold can have begun, so
// the report errs LATE, never early — never the receipt time, which the contract names as the wrong
// anchor ("would report work that was assigned for most of that time").
//
// NULL unless the task is held by a unit with nobody named.
const taskHoldStartColumn = `CASE WHEN n.bo_phan_id IS NOT NULL AND NULLIF(n.nguoi_thuc_hien_ma, '') IS NULL THEN COALESCE(
	(SELECT min(k.thoi_diem) FROM nhat_ky_nhiem_vu k
	  WHERE k.tenant_id = $1 AND k.nhiem_vu_id = n.id
	    AND k.bo_phan_id = n.bo_phan_id AND NULLIF(k.nguoi_phu_trach_ma, '') IS NULL
	    AND k.thoi_diem > COALESCE(
	      (SELECT max(k2.thoi_diem) FROM nhat_ky_nhiem_vu k2
	        WHERE k2.tenant_id = $1 AND k2.nhiem_vu_id = n.id
	          AND (k2.bo_phan_id IS DISTINCT FROM n.bo_phan_id OR NULLIF(k2.nguoi_phu_trach_ma, '') IS NOT NULL)),
	      '-infinity'::timestamptz)),
	n.cap_nhat_luc) END`

// automationTaskQuery reads every live task still owed — the overview's "đang thực hiện" set
// (taskInProgressCondition: `tam-dung` excluded, `cho-duyet` and legacy `chuyen-tiep` included), so
// a reminder speaks about exactly the tasks the overview counts.
var automationTaskQuery = `SELECT n.id, n.ma, n.trang_thai, n.han_xu_ly,
	COALESCE(n.bo_phan_id, ''), COALESCE(n.nguoi_thuc_hien_ma, ''), ` + taskHoldStartColumn + `
	FROM nhiem_vu n
	WHERE n.tenant_id = $1 AND n.deleted_at IS NULL AND n.` + taskInProgressCondition + `
	ORDER BY n.id`

// OpenTasksForAutomation reads every live, still-owed task of the commune in ctx.
//
// ONE READ PER RUN, NOT PER RECORD: the job compares each row with the instants identity answered, in
// memory (skill load-data-once). Unbounded on purpose — a limit would silently leave late work
// unreported; the set is a commune's OPEN tasks, not its archive.
func (s *NhiemVuStore) OpenTasksForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, automationTaskQuery)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu: đọc cho việc nền: %w", err)
	}
	defer rows.Close()
	var out []domain.AutomationRecord
	for rows.Next() {
		var (
			r        domain.AutomationRecord
			deadline sql.NullTime
			hold     sql.NullTime
		)
		if err := rows.Scan(&r.ID, &r.Code, &r.Status, &deadline, &r.OrgUnitID, &r.AssigneeMa, &hold); err != nil {
			return nil, fmt.Errorf("nhiem_vu: đọc cho việc nền: đọc dòng: %w", err)
		}
		if deadline.Valid {
			r.Deadline = deadline.Time.UTC()
		}
		if hold.Valid {
			r.HoldStartedAt = hold.Time.UTC()
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu: đọc cho việc nền: %w", err)
	}
	return out, nil
}

// automationCitizenReportOpen is the resolve clock still running: the four statuses before
// `da-xu-ly` — the same set citizenReportResolutionOverdue names (citizen_report_summary.go), without
// its now(). Once the work is done the clock has stopped, and a petition awaiting the citizen's
// confirmation is not late work.
var automationCitizenReportOpen = `trang_thai IN ('` + string(domain.DaTiepNhan) + `', '` +
	string(domain.DangPhanLoai) + `', '` + string(domain.DaChuyenXuLy) + `', '` + string(domain.DangXuLy) + `')`

// citizenReportHoldStartColumn: a petition's unit and officer are written ONLY by the assignment act
// (XuLyPhanAnh.PhanCong), and every assignment appends a `phan-cong` timeline row. So the hold began
// at the LATEST `phan-cong` entry. Fallback `cap_nhat_luc`, for the reason taskHoldStartColumn gives.
var citizenReportHoldStartColumn = `CASE WHEN p.bo_phan_id IS NOT NULL AND NULLIF(p.can_bo_xu_ly_id, '') IS NULL THEN COALESCE(
	(SELECT max(k.thoi_diem) FROM nhat_ky_phan_anh k
	  WHERE k.tenant_id = $1 AND k.phieu_phan_anh_id = p.id AND k.hanh_vi = '` + string(domain.NhatKyPhanCong) + `'),
	p.cap_nhat_luc) END`

var automationCitizenReportQuery = `SELECT p.id, p.ma_tra_cuu, COALESCE(p.linh_vuc, ''), p.trang_thai,
	p.han_xu_ly_xong, COALESCE(p.bo_phan_id, ''), COALESCE(p.can_bo_xu_ly_id, ''), ` + citizenReportHoldStartColumn + `
	FROM phieu_phan_anh p
	WHERE p.tenant_id = $1 AND p.deleted_at IS NULL AND p.` + automationCitizenReportOpen + `
	ORDER BY p.id`

// OpenCitizenReportsForAutomation reads every live petition whose resolve clock is still running.
//
// THE RESTRICTED FIELD IS READ, AND FLAGGED: its assignee must still be reminded; who ELSE may be
// told is the job's decision (domain.AutomationRecord.Restricted), made fail-closed there.
//
// `can_bo_xu_ly_id` holds a staff BUSINESS CODE despite its name (migration 0013's note).
func (s *PhieuPhanAnhStore) OpenCitizenReportsForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, automationCitizenReportQuery)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: đọc cho việc nền: %w", err)
	}
	defer rows.Close()
	var out []domain.AutomationRecord
	for rows.Next() {
		var (
			r        domain.AutomationRecord
			deadline sql.NullTime
			hold     sql.NullTime
		)
		if err := rows.Scan(&r.ID, &r.Code, &r.Field, &r.Status, &deadline, &r.OrgUnitID, &r.AssigneeMa, &hold); err != nil {
			return nil, fmt.Errorf("phieu_phan_anh: đọc cho việc nền: đọc dòng: %w", err)
		}
		if deadline.Valid {
			r.Deadline = deadline.Time.UTC()
		}
		if hold.Valid {
			r.HoldStartedAt = hold.Time.UTC()
		}
		r.Restricted = r.Field == domain.LinhVucHanChe
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: đọc cho việc nền: %w", err)
	}
	return out, nil
}

// citizenReportDigestColumns counts the petition half of the weekly digest in ONE statement, so the
// three figures describe one instant of the register. $2 = the run's instant, $3 = the start of the
// rating window.
var citizenReportDigestColumns = func() string {
	late := `(` + automationCitizenReportOpen + ` AND han_xu_ly_xong IS NOT NULL AND han_xu_ly_xong <= $2)`
	rated := `(diem_hai_long IN (1, 2) AND danh_gia_luc IS NOT NULL AND danh_gia_luc >= $3 AND danh_gia_luc <= $2)`
	return `count(*), count(*) FILTER (WHERE ` + late + `), count(*) FILTER (WHERE ` + rated + `),` +
		` count(*) FILTER (WHERE ` + late + ` OR ` + rated + `)`
}()

// CitizenReportDigestCounts counts "phản ánh nóng" for the weekly digest (ADR 0058 §8).
//
// THE RESTRICTED FIELD IS EXCLUDED (restrictedFieldExclusion) — fail closed, exactly as the overview
// counts for a reader without `feedback.restricted`: the leadership receiving the digest is not known
// to hold it, and a count of `can-bo` reports tells a colleague that reports about staff exist.
func (s *PhieuPhanAnhStore) CitizenReportDigestCounts(ctx context.Context, asOf, ratedFrom time.Time) (
	domain.CitizenReportDigestCounts, error) {

	if asOf.IsZero() || ratedFrom.IsZero() || !ratedFrom.Before(asOf) {
		return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: khoảng thời gian không hợp lệ")
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenReportDigestColumns, "phieu_phan_anh",
		`AND deleted_at IS NULL`+restrictedFieldExclusion, asOf, ratedFrom)
	if err != nil {
		return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: %w", err)
		}
		return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: câu đếm không trả dòng nào")
	}
	var c [4]int64
	if err := rows.Scan(&c[0], &c[1], &c[2], &c[3]); err != nil {
		return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenReportDigestCounts{}, fmt.Errorf("phieu_phan_anh: bản tin tuần: %w", err)
	}
	return domain.CitizenReportDigestCounts{Examined: int(c[0]), PastDeadline: int(c[1]), LowRated: int(c[2]),
		Hot: int(c[3])}, nil
}
