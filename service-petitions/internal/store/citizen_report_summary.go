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
	"database/sql"
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

// mainPetitionCondition is "a MAIN petition — an incident" (ADR 0087 §7, migration 0037): one that is not
// merged into another. THE ONE SPELLING of "only main petitions", used by every WORK figure:
//
//	on time / late / their sample      here, inside citizenReportMetricCondition — so the /tong-quan tile,
//	                                   the breakdown and the `metric=` drill-down are one predicate
//	the overdue stock and its queue    citizenReportOverdueNow, OverdueCitizenReports (owner, 09/10/2026,
//	                                   ADR 0087 open item #4 answered by the main session)
//	by field, by unit, by hamlet       citizen_report_figures.go (ADR 0087 §7; ADR 0053 §C5)
//
// AND NOT BY "Nhận vào" (`received`), which counts EVERY petition — each one is a citizen reporting, the
// load on the channel (ADR 0087 §7). Counting a merged petition in on-time would count one incident twice
// — what ADR 0008 #8 avoids. Every petition nobody merged has `merged_into` NULL, so before the first merge
// every figure is unchanged.
const mainPetitionCondition = `merged_into IS NULL`

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
	// THE THREE ON-TIME FIGURES COUNT MAIN PETITIONS ONLY — mainPetitionCondition says why.
	case domain.CitizenReportOnTimeSample:
		return "(" + mainPetitionCondition + " AND ((" + settled + ") OR (" + ceilingMissed + ")))"
	case domain.CitizenReportOnTime:
		return "(" + mainPetitionCondition + " AND ((" + settled + " AND xu_ly_xong_luc <= han_xu_ly_xong) AND NOT (" +
			ceilingMissed + ")))"
	case domain.CitizenReportLate:
		return "(" + mainPetitionCondition + " AND ((" + settled + " AND xu_ly_xong_luc > han_xu_ly_xong) OR (" +
			ceilingMissed + ")))"
	case domain.CitizenReportRatingSample:
		return "(" + citizenReportRatedCondition + ")"
	case domain.CitizenReportLowRating:
		// THE REGISTER FILTER'S OWN TEXT (ratingAtMostCondition), with the bound written from the domain
		// constant instead of a placeholder — so this figure and `?rating_max=2` are one predicate.
		return "(" + ratingAtMostCondition(strconv.Itoa(domain.LowRatingMaxStars)) + ")"
	case domain.CitizenReportPublicationPending:
		// `cho-duyet` OUTSIDE `can-bo`, even for a reader holding `feedback.restricted`. A staff-conduct
		// petition can NEVER be published (ADR 0050 point 8; migration 0017's CHECK
		// `phieu_phan_anh_staff_conduct_never_public`), so it is not awaiting a moderation decision —
		// counting it would put on the card a queue nobody can empty. Such a row should not exist (it is
		// born `an`, and classifying into `can-bo` hides it), so the clause is a floor, not a filter that
		// changes today's number.
		//
		// SPELLED `IS DISTINCT FROM` AND NOT WITH restrictedFieldExclusion ON PURPOSE: that constant is the
		// READER's scope (absent exactly when the reader holds `feedback.restricted`), and this is a rule
		// about the record that holds for every reader. One text for two rules would make "does this
		// statement apply the reader's scope" unanswerable by reading it. NULL `linh_vuc` is counted.
		return "(publication_status = '" + string(domain.PublicationPending) + "'" +
			" AND linh_vuc IS DISTINCT FROM '" + domain.LinhVucHanChe + "')"
	}
	return "(FALSE)"
}

// citizenReportRatedCondition is "the citizen has rated this petition". `diem_hai_long` has ONE writer,
// the citizen route (ADR 0062), so every rated row is a citizen's own verdict.
const citizenReportRatedCondition = "diem_hai_long IS NOT NULL"

// ratingAtMostCondition is "rated, and at most `bound` stars" — THE ONE SPELLING shared by the register's
// `rating_max` filter (bound = a placeholder) and the `low_rating` figure (bound = a constant). An
// unrated row is never in it: "no verdict" is not a poor verdict.
func ratingAtMostCondition(bound string) string {
	return citizenReportRatedCondition + " AND diem_hai_long <= " + bound
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
// domain.CitizenReportMetrics, with the period bound once as $2/$3 — then ONE aggregate that is not a
// count: the sum of the stars over exactly the `rating_sample` rows, the numerator of the average.
func citizenReportSummaryColumns() string {
	var cols string
	for i, m := range domain.CitizenReportMetrics {
		if i > 0 {
			cols += ", "
		}
		cols += "count(*) FILTER (WHERE " + citizenReportMetricCondition(m, "$2", "$3") + ")"
	}
	return cols + ", " + citizenReportRatingSumColumn
}

// citizenReportRatingSumColumn is NULL when no row is rated (SQL's sum of nothing); the scan reads that
// as 0 beside a 0 sample, which the client renders as a dash — never as an average of 0.
var citizenReportRatingSumColumn = "sum(diem_hai_long) FILTER (WHERE " +
	citizenReportMetricCondition(domain.CitizenReportRatingSample, "$2", "$3") + ")"

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
// the register for every figure, as TaskSummary argues.
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
	var (
		counts    [8]int64
		ratingSum sql.NullInt64
	)
	if err := rows.Scan(&counts[0], &counts[1], &counts[2], &counts[3], &counts[4],
		&counts[5], &counts[6], &counts[7], &ratingSum); err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenReportSummary{}, fmt.Errorf("phieu_phan_anh: tổng quan: %w", err)
	}
	// POSITIONAL, in the order of domain.CitizenReportMetrics, then the sum.
	return domain.CitizenReportSummary{
		Received:           int(counts[0]),
		InProgress:         int(counts[1]),
		OnTimeSample:       int(counts[2]),
		OnTime:             int(counts[3]),
		Late:               int(counts[4]),
		RatingSample:       int(counts[5]),
		LowRating:          int(counts[6]),
		PublicationPending: int(counts[7]),
		RatingSum:          int(ratingSum.Int64), // NULL (nothing rated) -> 0, beside a 0 sample
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
//
// COLUMN 5 IS `now()` ITSELF — the instant the two predicates compared against, constant for the whole
// statement — so the use case measures the working time late up to exactly the `now` that made the row
// late (domain.OverdueItem.AsOf). It is appended, so the ORDER BY position of column 4 does not move.
var overdueCitizenReportColumns = `ma_tra_cuu, COALESCE(linh_vuc, '') AS linh_vuc,` +
	` CASE WHEN ` + citizenReportClassificationOverdue + ` THEN '` + string(domain.DeadlineClassification) +
	`' ELSE '` + string(domain.DeadlineResolution) + `' END AS kind,` +
	` CASE WHEN ` + citizenReportClassificationOverdue + ` THEN han_phan_loai ELSE han_xu_ly_xong END AS missed,` +
	` now() AS as_of`

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
	// MAIN PETITIONS ONLY (mainPetitionCondition): a merged petition follows its main, whose deadline is
	// already the earlier of the two — listing both is one incident twice, and an unclassified merged
	// petition would sit past its ceiling for ever. The SAME predicate as the breakdown's stock figure.
	filter := scopeFilter(restricted) + ` AND ` + citizenReportOverdueNow +
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
		if err := rows.Scan(&it.Code, &it.CategoryCode, &kind, &it.MissedDeadline, &it.AsOf); err != nil {
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
