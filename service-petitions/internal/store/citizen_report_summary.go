package store

// The petition register's overview figures, its overdue queue, and the drill-down filter that lists
// the rows behind each figure. SQL, and nothing else.
//
// ONE PREDICATE PER FIGURE, USED BY BOTH THE COUNT AND THE LIST — citizenReportMetricCondition, for
// the reason task_summary.go gives. The restricted field `can-bo` is excluded from the figures by the
// SAME constant the register list uses (restrictedFieldExclusion), decided by the SAME permission
// fact, so a reader without `feedback.restricted` is never shown a number their list cannot reach.
//
// Commune $1 from the context (rule 1, invariant 5); soft-deleted rows excluded from every figure
// (rule 7, invariant 2); period bounds bound, never concatenated.

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ErrCitizenReportMetricInvalid — see ErrTaskMetricInvalid; same floor, other register.
var ErrCitizenReportMetricInvalid = errors.New("phieu_phan_anh: chỉ số tổng quan không hợp lệ")

// citizenReportInProgressCondition is "đang xử lý" in the overview's sense: every status in which the
// commune still owes the citizen something.
//
// THE THREE EXCLUSIONS ARE THE THREE ENDINGS: `da-dong` (closed with a result), `khong-tiep-nhan` and
// `chuyen-cap-tren` (the two terminal branches). A REOPENED petition is back in `dang-xu-ly` and is
// therefore counted — it is work owed again, not a closed file. `da-xu-ly` and `cho-dan-xac-nhan` are
// counted too: the work is done but the file is not closed, so it is still on the commune's desk.
var citizenReportInProgressCondition = `trang_thai NOT IN ('` + string(domain.DaDong) + `', '` +
	string(domain.KhongTiepNhan) + `', '` + string(domain.ChuyenCapTren) + `')`

// citizenReportMetricCondition is THE ONE SQL SPELLING of each petition figure. `from`/`to` are the
// placeholders chosen for the period bounds; the stock figure ignores them.
//
// # THE ON-TIME SAMPLE IS A ∪ B, AND B IS OPEN QUESTION #26's ANSWER
//
//	A  settled in the period: `xu_ly_xong_luc` in [from, to) against a stored `han_xu_ly_xong`.
//	   On time iff settled at or before that deadline — the resolve clock stops when the WORK is
//	   done, not at closing (domain.PhieuPhanAnh.QuaHan).
//	B  the classification ceiling fell in the period and the petition was still unclassified at
//	   it — classified after `han_phan_loai`, or not classified yet and the ceiling already past.
//	   ALWAYS LATE. Without B a commune that classifies slowly has a better ratio than one that
//	   classifies promptly, because its slow petitions never reach A (ADR 0035 §C).
//	   `khong-tiep-nhan` and `chuyen-cap-tren` are excluded from B: the commune declined or referred
//	   the petition, and there is no resolve commitment for it to have missed.
//
// A AND B CAN HOLD FOR ONE PETITION IN DIFFERENT PERIODS — B in the period its ceiling fell in, A in
// the period it was settled. INSIDE ONE PERIOD a petition is one row, so it is counted at most once
// in the sample, and a row satisfying both is LATE (it missed the ceiling), never on time.
//
// # WHY EVERY BRANCH TESTS ITS COLUMNS FOR NULL EXPLICITLY
//
// `on_time` is `A-on-time AND NOT B`. In SQL `NOT NULL` is NULL, so a B that evaluated to NULL for an
// unclassified row would silently drop an on-time petition from the numerator while it stayed in the
// denominator — a lower ratio with no error anywhere. Each comparison below is guarded so A and B are
// always TRUE or FALSE, never NULL.
func citizenReportMetricCondition(m domain.CitizenReportMetric, from, to string) string {
	settled := `han_xu_ly_xong IS NOT NULL AND xu_ly_xong_luc IS NOT NULL` +
		` AND xu_ly_xong_luc >= ` + from + ` AND xu_ly_xong_luc < ` + to
	ceilingMissed := `han_phan_loai IS NOT NULL` +
		` AND han_phan_loai >= ` + from + ` AND han_phan_loai < ` + to +
		` AND trang_thai NOT IN ('` + string(domain.KhongTiepNhan) + `', '` + string(domain.ChuyenCapTren) + `')` +
		` AND ((phan_loai_luc IS NOT NULL AND phan_loai_luc > han_phan_loai)` +
		` OR (phan_loai_luc IS NULL AND han_phan_loai < now()))`

	switch m {
	case domain.CitizenReportInProgress:
		return "(" + citizenReportInProgressCondition + ")"
	case domain.CitizenReportReceived:
		// `vao_so_luc` — when the commune took the report into its register — and NOT `goc_dem_han`,
		// which on the staff-booked channel can be a week earlier. Includes `khong-tiep-nhan`: a report
		// the commune declined was still received.
		return "(vao_so_luc >= " + from + " AND vao_so_luc < " + to + ")"
	case domain.CitizenReportOnTimeSample:
		return "((" + settled + ") OR (" + ceilingMissed + "))"
	case domain.CitizenReportOnTime:
		return "((" + settled + " AND xu_ly_xong_luc <= han_xu_ly_xong) AND NOT (" + ceilingMissed + "))"
	case domain.CitizenReportLate:
		return "((" + settled + " AND xu_ly_xong_luc > han_xu_ly_xong) OR (" + ceilingMissed + "))"
	}
	return "(FALSE)"
}

// citizenReportMetricFilter appends one metric's predicate to the list's WHERE clause — see
// taskMetricFilter. Every period-bound petition figure uses BOTH bounds more than once; each is bound
// ONCE and its placeholder repeated, which PostgreSQL allows.
func citizenReportMetricFilter(m domain.CitizenReportMetric, p domain.Period, args []any) (string, []any) {
	var from, to string
	if m.PeriodBound() {
		args = append(args, p.From)
		from = "$" + strconv.Itoa(len(args)+1)
		args = append(args, p.To)
		to = "$" + strconv.Itoa(len(args)+1)
	}
	return " AND " + citizenReportMetricCondition(m, from, to), args
}

// validateCitizenReportMetric is the store-side floor under the handler's validation.
func validateCitizenReportMetric(loc LocPhieu) error {
	if loc.Metric == "" {
		return nil
	}
	if !loc.Metric.Valid() {
		return ErrCitizenReportMetricInvalid
	}
	if loc.Metric.PeriodBound() {
		if _, err := domain.NewPeriod(loc.Period.From, loc.Period.To); err != nil {
			return ErrCitizenReportMetricInvalid
		}
	}
	return nil
}

// citizenReportSummaryColumns is the SELECT list of CitizenReportSummary, in the order of
// domain.CitizenReportMetrics, with the period bound once as $2/$3.
func citizenReportSummaryColumns() string {
	var cols string
	for i, m := range domain.CitizenReportMetrics {
		if i > 0 {
			cols += ", "
		}
		cols += "count(*) FILTER (WHERE " + citizenReportMetricCondition(m, "$2", "$3") + ")"
	}
	return cols
}

// scopeFilter is the WHERE tail every overview read of the register starts from: live rows only, and
// the restricted field excluded unless the reader holds `feedback.restricted`.
func scopeFilter(restricted bool) string {
	filter := `AND deleted_at IS NULL`
	if !restricted {
		filter += restrictedFieldExclusion
	}
	return filter
}

// CitizenReportSummary counts every petition figure of the commune in ONE statement — one instant of
// the register for all five, as TaskSummary argues.
//
// `restricted` IS THE CALLER'S ANSWER TO "does this reader hold `feedback.restricted`". False — the
// zero value — excludes the `can-bo` field from every figure, exactly as LocPhieu.ChoPhepHanChe does
// for the list. Fail closed: a caller that forgets it gets the narrow count.
func (s *PhieuPhanAnhStore) CitizenReportSummary(ctx context.Context, p domain.Period, restricted bool) (
	domain.CitizenReportSummary, error) {

	if _, err := domain.NewPeriod(p.From, p.To); err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: %w", err)
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenReportSummaryColumns(), "phieu_phan_anh",
		scopeFilter(restricted), p.From, p.To)
	if err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: %w", err)
		}
		return domain.CitizenReportSummary{}, errors.New("phieu_phan_anh: tổng quan: câu đếm không trả dòng nào")
	}
	var counts [5]int64
	if err := rows.Scan(&counts[0], &counts[1], &counts[2], &counts[3], &counts[4]); err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: %w", err)
	}
	// POSITIONAL, in the order of domain.CitizenReportMetrics.
	return domain.CitizenReportSummary{
		Received:     int(counts[0]),
		InProgress:   int(counts[1]),
		OnTimeSample: int(counts[2]),
		OnTime:       int(counts[3]),
		Late:         int(counts[4]),
	}, nil
}

// citizenReportClassificationOverdue is "past the classification ceiling and still unclassified, as of
// now" — the live half of B above, with no period.
var citizenReportClassificationOverdue = `phan_loai_luc IS NULL AND han_phan_loai IS NOT NULL` +
	` AND han_phan_loai < now() AND ` + citizenReportInProgressCondition

// citizenReportResolutionOverdue is "past the resolve deadline with the work not done, as of now".
//
// BY STATUS AND NOT BY `xu_ly_xong_luc IS NULL`, and the difference is a reopened petition:
// DoiTrangThai keeps the first finishing instant (COALESCE) when a petition goes back to `dang-xu-ly`,
// so a test on the column would hide every reopened petition past its deadline. The four statuses are
// the ones before `da-xu-ly` — once the work is done the resolve clock has stopped, and a petition
// awaiting the citizen's confirmation is not late work.
var citizenReportResolutionOverdue = `trang_thai IN ('` + string(domain.DaTiepNhan) + `', '` +
	string(domain.DangPhanLoai) + `', '` + string(domain.DaChuyenXuLy) + `', '` +
	string(domain.DangXuLy) + `') AND han_xu_ly_xong IS NOT NULL AND han_xu_ly_xong < now()`

// overdueCitizenReportColumns is the SELECT list of OverdueCitizenReports. THE CASE IS WRITTEN ONCE
// PER COLUMN FROM ONE CONDITION, and the query orders on column 4 by position, so the kind and the
// instant cannot come from two different branches.
var overdueCitizenReportColumns = `ma_tra_cuu, COALESCE(linh_vuc, '') AS linh_vuc,` +
	` CASE WHEN ` + citizenReportClassificationOverdue + ` THEN '` + string(domain.DeadlineClassification) +
	`' ELSE '` + string(domain.DeadlineResolution) + `' END AS kind,` +
	` CASE WHEN ` + citizenReportClassificationOverdue + ` THEN han_phan_loai ELSE han_xu_ly_xong END AS missed`

// OverdueCitizenReports reads at most `limit` petitions currently past one of their two deadlines,
// oldest missed deadline first. `restricted` as in CitizenReportSummary.
//
// WHICH DEADLINE: an unclassified petition past its ceiling reports the CEILING; otherwise the resolve
// deadline. The two cannot both hold on one row today (an unclassified citizen petition has no
// resolve deadline; a staff-booked one has no ceiling), and the CASE puts the ceiling first if they
// ever do, because it is the earlier commitment.
//
// NO CONTENT, NO REPORTER, NO LOCATION — code, field and deadline only (rule 3; domain.OverdueItem).
func (s *PhieuPhanAnhStore) OverdueCitizenReports(ctx context.Context, limit int, restricted bool) (
	[]domain.OverdueItem, error) {

	if limit < 1 || limit > OverdueQueueMax {
		return nil, ErrOverdueQueueLimit
	}
	filter := scopeFilter(restricted) +
		` AND ((` + citizenReportClassificationOverdue + `) OR (` + citizenReportResolutionOverdue + `))` +
		` ORDER BY 4 ASC, id ASC LIMIT $2`

	rows, err := s.db.For(ctx).Query(ctx, overdueCitizenReportColumns, "phieu_phan_anh", filter, limit)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: hàng đợi quá hạn: %w", err)
	}
	defer rows.Close()

	items := make([]domain.OverdueItem, 0, limit)
	for rows.Next() {
		var (
			it   domain.OverdueItem
			kind string
		)
		if err := rows.Scan(&it.Code, &it.CategoryCode, &kind, &it.MissedDeadline); err != nil {
			return nil, fmt.Errorf("phieu_phan_anh: hàng đợi quá hạn: đọc dòng: %w", err)
		}
		it.Kind = domain.DeadlineKind(kind)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: hàng đợi quá hạn: %w", err)
	}
	return items, nil
}
