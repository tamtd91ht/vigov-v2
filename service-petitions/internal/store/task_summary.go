package store

// The task register's overview figures, its overdue queue, and the drill-down filter that lists the
// rows behind each figure. SQL, and nothing else.
//
// ONE PREDICATE PER FIGURE, USED BY BOTH THE COUNT AND THE LIST. taskMetricCondition is the only
// place a figure's row set is spelled: TaskSummary puts it inside `count(*) FILTER (WHERE …)`, and
// locNhiemVuThanhSQL puts the same text into the WHERE clause of GET /api/v1/tasks?metric=…. A leader
// who clicks "12 quá hạn" must land on a list of exactly twelve rows; two spellings of "quá hạn" would
// agree until the day one of them is edited.
//
// THE SAME THREE GUARANTEES AS THE REST OF THIS PACKAGE: the commune is $1 from the context (rule 1,
// invariant 5), soft-deleted rows are excluded from every figure (rule 7, invariant 2), and every
// period bound is a bound parameter.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ErrTaskMetricInvalid is a drill-down request the store refuses: an unknown metric, or a
// period-bound metric without a valid period. The handler validates first; this is the floor under
// it, so a caller that skipped the handler gets a refusal rather than an unfiltered register.
var ErrTaskMetricInvalid = errors.New("nhiem_vu: chỉ số tổng quan không hợp lệ")

// taskInProgressCondition is "đang thực hiện" in the overview's sense: the five statuses in which the
// work is still owed.
//
// `tam-dung` IS EXCLUDED ON PURPOSE — a suspended task is its own figure, and counting it here too
// would put one task in two tiles that a leader reads as disjoint. `hoan-thanh` is done.
//
// A LEGACY `chuyen-tiep` ROW IS COUNTED since 28/09/2026 (P13, user decision). It is no longer
// terminal — require 52ec9b5 lets it move on to `da-tiep-nhan` / `dang-thuc-hien`, and forwarding is
// now the assignment act, so no new row enters it — which makes it work still owed by somebody.
// Leaving it out would hide open work from the tile whose job is to show it. Because the overdue
// figure is built on this condition, a `chuyen-tiep` row past its deadline is now ALSO overdue — the
// same "late and still waiting on somebody" reading.
//
// Sub-tasks are counted as their own rows: `nhiem_vu_cha_id` is not consulted, because each sub-task
// carries its own assignee and its own deadline and the register lists it as a row.
var taskInProgressCondition = `trang_thai IN ('` + string(domain.MoiGiao) + `', '` +
	string(domain.DaTiepNhanNV) + `', '` + string(domain.DangThucHien) + `', '` +
	string(domain.ChoDuyet) + `', '` + string(domain.ChuyenTiep) + `')`

// taskOverdueCondition is "quá hạn" for the overview: work still owed whose CURRENT deadline has
// passed.
//
// ⚠ IT IS DELIBERATELY NOT dieuKienTreHan (nhiem_vu.go), and the difference is the reason it exists.
// dieuKienTreHan answers "was this task late" — it stays true for a task FINISHED late, forever. The
// overview's tile answers "what is
// late and still waiting on somebody", which is a stock of open work: a task finished late last month
// is not something a leader can act on today. The register's `late=true` box keeps its own meaning,
// untouched.
//
// `han_xu_ly` AND NOT `han_ban_dau`: the commitment as it stands today, after any granted extension
// (see domain.NhiemVu.HoanThanhDungHanBanDau for why the two must not be swapped). `now()` is the
// database's clock, the one the deadline was stored against.
var taskOverdueCondition = taskInProgressCondition + ` AND han_xu_ly IS NOT NULL AND han_xu_ly < now()`

// taskMetricCondition is THE ONE SQL SPELLING of each task figure. `from` and `to` are the
// placeholders already chosen for the period bounds ("$2", "$3"…); they are ignored by the stock
// figures, which must then be given no period arguments at all.
//
// THE RESULT IS PARENTHESISED so it can be ANDed into a longer WHERE clause or dropped into a FILTER
// without its OR/AND binding to a neighbour.
//
// AN UNKNOWN METRIC YIELDS `FALSE`, never an empty string: an empty predicate would be "every row",
// which is the whole register under a heading that names one figure. The callers refuse an unknown
// metric before reaching here; this is the floor.
func taskMetricCondition(m domain.TaskMetric, from, to string) string {
	// Completed inside the period, measured on the recorded completion instant. Half-open [from, to):
	// two adjacent periods never count one task twice.
	completed := `trang_thai = '` + string(domain.HoanThanh) + `' AND ngay_hoan_thanh >= ` + from +
		` AND ngay_hoan_thanh < ` + to
	// §11.3's sample: completed tasks that HAD an original deadline. A task with no deadline was
	// promised nothing, so it can be neither on time nor late — counting it in the denominator would
	// make a commune that sets no deadlines look punctual.
	sample := completed + ` AND han_ban_dau IS NOT NULL`

	switch m {
	case domain.TaskInProgress:
		return "(" + taskInProgressCondition + ")"
	case domain.TaskOverdue:
		return "(" + taskOverdueCondition + ")"
	case domain.TaskSuspended:
		return "(trang_thai = '" + string(domain.TamDung) + "')"
	case domain.TaskCompleted:
		return "(" + completed + ")"
	case domain.TaskOnTimeSample:
		return "(" + sample + ")"
	case domain.TaskOnTime:
		// ON TIME AGAINST THE ORIGINAL DEADLINE (`han_ban_dau`), exactly as
		// domain.NhiemVu.HoanThanhDungHanBanDau: an extension granted afterwards must not turn a late
		// task into an on-time one in the ratio reported upward.
		return "(" + sample + ` AND ngay_hoan_thanh <= han_ban_dau)`
	}
	return "(FALSE)"
}

// taskMetricFilter appends one metric's predicate to a WHERE clause under construction, binding the
// period bounds as the next placeholders. `args` is what has been bound so far AFTER the commune, so
// the next placeholder is len(args)+2.
//
// ONE HELPER FOR THE LIST AND NOTHING ELSE: the summary binds its period once for all six figures and
// calls taskMetricCondition directly with "$2"/"$3".
func taskMetricFilter(m domain.TaskMetric, p domain.Period, args []any) (string, []any) {
	var from, to string
	if m.PeriodBound() {
		args = append(args, p.From)
		from = "$" + strconv.Itoa(len(args)+1)
		args = append(args, p.To)
		to = "$" + strconv.Itoa(len(args)+1)
	}
	return " AND " + taskMetricCondition(m, from, to), args
}

// validateTaskMetric is the store-side floor under the handler's validation of LocNhiemVu.Metric.
func validateTaskMetric(loc LocNhiemVu) error {
	if loc.Metric == "" {
		return nil
	}
	if !loc.Metric.Valid() {
		return ErrTaskMetricInvalid
	}
	if loc.Metric.PeriodBound() {
		if _, err := domain.NewPeriod(loc.Period.From, loc.Period.To); err != nil {
			return ErrTaskMetricInvalid
		}
	}
	return nil
}

// taskSummaryColumns is the SELECT list of TaskSummary: one `count(*) FILTER` per figure, in the order
// of domain.TaskMetrics, with the period bound once as $2/$3.
func taskSummaryColumns() string {
	var cols string
	for i, m := range domain.TaskMetrics {
		if i > 0 {
			cols += ", "
		}
		cols += "count(*) FILTER (WHERE " + taskMetricCondition(m, "$2", "$3") + ")"
	}
	return cols
}

// TaskSummary counts every task figure of the commune in ONE statement.
//
// ONE STATEMENT AND NOT SIX, so the six figures describe the same instant of the register: six
// separate counts would let a task that moves between two of them be counted in both, or in neither,
// on a screen that presents them side by side.
//
// THE PERIOD IS BOUND EVEN THOUGH THE STOCK FIGURES IGNORE IT — they are in the same statement, and a
// statement is either given its placeholders or it is not. The period has been validated by the
// caller (domain.NewPeriod); a zero one here is refused, never read as "all time".
func (s *NhiemVuStore) TaskSummary(ctx context.Context, p domain.Period) (domain.TaskSummary, error) {
	if _, err := domain.NewPeriod(p.From, p.To); err != nil {
		return domain.TaskSummary{}, fmt.Errorf("nhiem_vu: tổng quan: %w", err)
	}
	rows, err := s.db.For(ctx).Query(ctx, taskSummaryColumns(), "nhiem_vu",
		`AND deleted_at IS NULL`, p.From, p.To)
	if err != nil {
		return domain.TaskSummary{}, fmt.Errorf("nhiem_vu: tổng quan: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// An aggregate without GROUP BY always yields one row; none means the driver failed.
		if err := rows.Err(); err != nil {
			return domain.TaskSummary{}, fmt.Errorf("nhiem_vu: tổng quan: %w", err)
		}
		return domain.TaskSummary{}, errors.New("nhiem_vu: tổng quan: câu đếm không trả dòng nào")
	}
	var counts [6]int64
	if err := rows.Scan(&counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5]); err != nil {
		return domain.TaskSummary{}, fmt.Errorf("nhiem_vu: tổng quan: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.TaskSummary{}, fmt.Errorf("nhiem_vu: tổng quan: %w", err)
	}
	// POSITIONAL, IN THE ORDER OF domain.TaskMetrics — which taskSummaryColumns walks. The test pins
	// both orders together.
	return domain.TaskSummary{
		InProgress:   int(counts[0]),
		Overdue:      int(counts[1]),
		Suspended:    int(counts[2]),
		Completed:    int(counts[3]),
		OnTimeSample: int(counts[4]),
		OnTime:       int(counts[5]),
	}, nil
}

// taskInHandCondition is the per-department table's `total`: tasks IN HAND at some point of [from, to)
// — created before `to`, and either still not completed or completed at/after `from`.
//
// WHY THIS AND NOT "CREATED IN THE PERIOD": the table puts `completed` and `overdue` beside `total`, and
// a leader reads them as parts of one workload. Counting only tasks created inside the period would
// leave out the work a department carried in from before and finished inside it — `completed` would
// then exceed `total` for any department that clears a backlog.
//
// NOT COMPLETED IS `trang_thai <> 'hoan-thanh'`, so a suspended (`tam-dung`) task is in hand: it is
// still owed, only paused. A completed task always carries `ngay_hoan_thanh` (CHECK, migration 0006),
// so the second branch cannot drop one through a NULL.
//
// NOT A domain.TaskMetric: it has no drill-down list yet, and adding it to TaskMetrics would change the
// /tong-quan summary's columns. Soft-deleted rows are excluded by the caller's WHERE, like every other
// figure here.
func taskInHandCondition(from, to string) string {
	return `(tao_luc < ` + to + ` AND (trang_thai <> '` + string(domain.HoanThanh) +
		`' OR ngay_hoan_thanh >= ` + from + `))`
}

// taskUnitKey is the department a task row is grouped under. COALESCE SO "NO DEPARTMENT" IS ONE ROW:
// the column is nullable TEXT with no CHECK against the empty string, and grouping the raw column would
// split NULL and the empty string into two rows that the client renders under the same "— Chưa xác định —" label.
const taskUnitKey = `COALESCE(bo_phan_id, '')`

// taskUnitSummaryColumns is the SELECT list of TaskUnitSummary. Positional; the scan follows it.
//
// THE FIGURES ARE taskMetricCondition'S TEXT, NOT A RESPELLING — so the table's `completed`, `on_time`
// and `overdue` summed over departments equal the /tong-quan tiles for the same period.
func taskUnitSummaryColumns() string {
	cols := "GROUPING(" + taskUnitKey + "), " + taskUnitKey +
		", count(*) FILTER (WHERE " + taskInHandCondition("$2", "$3") + ")"
	for _, m := range []domain.TaskMetric{domain.TaskCompleted, domain.TaskOnTimeSample, domain.TaskOnTime,
		domain.TaskOverdue} {
		cols += ", count(*) FILTER (WHERE " + taskMetricCondition(m, "$2", "$3") + ")"
	}
	return cols + ", now()"
}

// taskUnitSummaryTail restricts the rows to those counted by at least one figure, then groups.
//
// THE WHERE IS THE UNION OF THE FIGURES (on_time_sample and on_time are subsets of completed), so a
// department appears only when it has something to show — the client fills the zero rows from the
// unit list.
//
// GROUPING SETS WITH AN EMPTY SET: the `()` set yields one grand row EVEN WHEN NO ROW MATCHES, and that
// row carries the statement's now(). Without it, a commune with no task in scope would get no `as_of`
// at all, or one read from a second clock.
func taskUnitSummaryTail() string {
	return `AND deleted_at IS NULL AND (` + taskInHandCondition("$2", "$3") + ` OR ` +
		taskMetricCondition(domain.TaskCompleted, "$2", "$3") + ` OR ` +
		taskMetricCondition(domain.TaskOverdue, "", "") + `)` +
		` GROUP BY GROUPING SETS ((` + taskUnitKey + `), ()) ORDER BY 1, 2`
}

// TaskUnitSummary counts the per-department figures of the commune in ONE statement, grouped by
// `bo_phan_id` (skills/load-data-once) — never one count per department. Each task row counts once,
// sub-tasks included as their own rows (ADR 0053 §4).
//
// The period has been validated by the caller; a zero one is refused here too, never read as "all
// time".
func (s *NhiemVuStore) TaskUnitSummary(ctx context.Context, p domain.Period) (domain.TaskUnitSummary, error) {
	if _, err := domain.NewPeriod(p.From, p.To); err != nil {
		return domain.TaskUnitSummary{}, fmt.Errorf("nhiem_vu: theo bộ phận: %w", err)
	}
	rows, err := s.db.For(ctx).Query(ctx, taskUnitSummaryColumns(), "nhiem_vu", taskUnitSummaryTail(),
		p.From, p.To)
	if err != nil {
		return domain.TaskUnitSummary{}, fmt.Errorf("nhiem_vu: theo bộ phận: %w", err)
	}
	defer rows.Close()

	out := domain.TaskUnitSummary{Units: []domain.TaskUnitFigures{}}
	var sawGrand bool
	for rows.Next() {
		var (
			grand  int64
			unit   sql.NullString
			counts [5]int64
			asOf   time.Time
		)
		if err := rows.Scan(&grand, &unit, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4],
			&asOf); err != nil {
			return domain.TaskUnitSummary{}, fmt.Errorf("nhiem_vu: theo bộ phận: đọc dòng: %w", err)
		}
		if grand != 0 {
			// The `()` grouping set: totals over every department, read for its now() only.
			sawGrand = true
			out.AsOf = asOf
			continue
		}
		out.Units = append(out.Units, domain.TaskUnitFigures{
			OrgUnitID:    unit.String,
			Total:        int(counts[0]),
			Completed:    int(counts[1]),
			OnTimeSample: int(counts[2]),
			OnTime:       int(counts[3]),
			Overdue:      int(counts[4]),
		})
	}
	if err := rows.Err(); err != nil {
		return domain.TaskUnitSummary{}, fmt.Errorf("nhiem_vu: theo bộ phận: %w", err)
	}
	if !sawGrand {
		// The empty grouping set always yields a row; none means the statement is not what was written.
		return domain.TaskUnitSummary{}, errors.New("nhiem_vu: theo bộ phận: thiếu dòng tổng của câu đếm")
	}
	return out, nil
}

// OverdueQueueMax bounds the "Cần xử lý ngay" panel. The route clamps to it; the store refuses more,
// so no caller can turn the panel into an unbounded list.
const OverdueQueueMax = 10

// ErrOverdueQueueLimit is a limit outside [1, OverdueQueueMax].
var ErrOverdueQueueLimit = errors.New("hàng đợi quá hạn: giới hạn số dòng không hợp lệ")

// OverdueTasks reads at most `limit` currently-overdue tasks — the overview's overdue figure's rows,
// oldest missed deadline first.
//
// THE SAME PREDICATE AS THE `overdue` FIGURE (taskMetricCondition), so every task the panel shows is
// one the tile counted. THREE COLUMNS AND NO TITLE: see domain.OverdueItem for why a task title does
// not travel on this surface. Critical is NOT decided here — it needs identity's working-hours
// calendar, which the use case asks.
func (s *NhiemVuStore) OverdueTasks(ctx context.Context, limit int) ([]domain.OverdueItem, error) {
	if limit < 1 || limit > OverdueQueueMax {
		return nil, ErrOverdueQueueLimit
	}
	rows, err := s.db.For(ctx).Query(ctx, "ma, loai, han_xu_ly", "nhiem_vu",
		`AND deleted_at IS NULL AND `+taskMetricCondition(domain.TaskOverdue, "", "")+
			` ORDER BY han_xu_ly ASC, ma ASC LIMIT $2`, limit)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu: hàng đợi quá hạn: %w", err)
	}
	defer rows.Close()

	items := make([]domain.OverdueItem, 0, limit)
	for rows.Next() {
		it := domain.OverdueItem{Kind: domain.DeadlineTask}
		if err := rows.Scan(&it.Code, &it.CategoryCode, &it.MissedDeadline); err != nil {
			return nil, fmt.Errorf("nhiem_vu: hàng đợi quá hạn: đọc dòng: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu: hàng đợi quá hạn: %w", err)
	}
	return items, nil
}
