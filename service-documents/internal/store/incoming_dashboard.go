package store

// The dashboard figures of SỔ VĂN BẢN ĐẾN, and the SQL that selects each figure's rows.
//
// ONE PREDICATE PER FIGURE, USED BY EVERY READ THAT SHOWS IT. metricPredicate below is called by
// CountIncomingSummary (the figure), by incomingFilterSQL (the drill-down list behind it) and by
// OverdueIncoming (the "CẦN XỬ LÝ NGAY" queue). docs/ui-ux/13-bao-cao.md §10 forbids /tong-quan and
// the lists behind it from disagreeing; three hand-written WHERE clauses would disagree the first time
// one of them was edited, and every test of each alone would stay green.
//
// EVERY STATEMENT HERE GOES THROUGH Scoped.Query, so `tenant_id = $1` is written by the scoped
// repository and not by this file (rule 1, invariant 5), and every one carries
// `deleted_at IS NULL` (rule 7, invariant 2).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// IncomingMetricFilter selects exactly one figure's row set.
//
// `Window` is read only by MetricArrived and `Now` only by MetricOverdue; the handler fills the one
// its metric needs and leaves the other zero. A zero `Metric` means "no figure filter".
type IncomingMetricFilter struct {
	Metric domain.IncomingMetric
	Window domain.ArrivalWindow
	Now    time.Time
}

// ErrUnknownIncomingMetric — a metric this file has no predicate for. Unreachable through the handler,
// which parses with domain.ParseIncomingMetric; refused here too, so a future caller cannot turn an
// unknown selector into "no filter".
var ErrUnknownIncomingMetric = errors.New("van_ban_den: chỉ số tổng quan không xác định")

// bindFunc appends one value to the statement's arguments and returns its placeholder.
type bindFunc func(v any) string

// newBinder returns a bindFunc numbering from $2 — $1 is the commune, always (Scoped.Query).
func newBinder(args *[]any) bindFunc {
	return func(v any) string {
		*args = append(*args, v)
		return fmt.Sprintf("$%d", len(*args)+1)
	}
}

// arrivedPredicate — `ngay_den` is a DATE and the window is a range of DATES, both ends inclusive
// (domain.ArrivalWindowFor). Bound as 'YYYY-MM-DD' text and cast, so no driver ever decides which
// zone a time.Time carrying a date should be rendered in.
func arrivedPredicate(w domain.ArrivalWindow, bind bindFunc) string {
	return "ngay_den BETWEEN " + bind(w.FirstDate.Format(time.DateOnly)) + "::date AND " +
		bind(w.LastDate.Format(time.DateOnly)) + "::date"
}

// openPredicate is NOT IncomingDocumentStatus.IsFinished(), with the closing codes taken FROM the domain
// (domain.FinishedIncomingStatuses) rather than typed here. `trang_thai` is NOT NULL (migration 0004),
// so NOT IN has no NULL trap.
func openPredicate(bind bindFunc) string {
	finished := domain.FinishedIncomingStatuses()
	ph := make([]string, 0, len(finished))
	for _, s := range finished {
		ph = append(ph, bind(string(s)))
	}
	return "trang_thai NOT IN (" + strings.Join(ph, ", ") + ")"
}

// lateAgainstPredicate is IncomingDocument.IsOverdue(now): not finished AND now is strictly after the deadline,
// i.e. `han_xu_ly_xong < now`. `han_xu_ly_xong` is NOT NULL (migration 0004), so every row has one.
//
// `now` IS BOUND FROM THE CALLER, not SQL now(): the queue compares the same instant against identity's
// answer, and the figure, the list and the queue of one dashboard load should share one clock.
func lateAgainstPredicate(now time.Time, bind bindFunc) string {
	return openPredicate(bind) + " AND han_xu_ly_xong < " + bind(now)
}

// metricPredicate is the ONE place a figure becomes SQL.
func metricPredicate(f IncomingMetricFilter, bind bindFunc) (string, error) {
	switch f.Metric {
	case domain.MetricArrived:
		if f.Window.FirstDate.IsZero() || f.Window.LastDate.IsZero() {
			return "", fmt.Errorf("van_ban_den: chỉ số %q thiếu kỳ", f.Metric)
		}
		return arrivedPredicate(f.Window, bind), nil
	case domain.MetricOpen:
		return openPredicate(bind), nil
	case domain.MetricOverdue:
		if f.Now.IsZero() {
			// A zero instant would make nothing overdue — a silent zero on the red cell.
			return "", fmt.Errorf("van_ban_den: chỉ số %q thiếu thời điểm so sánh", f.Metric)
		}
		return lateAgainstPredicate(f.Now, bind), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownIncomingMetric, f.Metric)
	}
}

// CountIncomingSummary counts the three figures in ONE statement over the commune's live register.
//
// ONE SCAN, THREE FILTERS, rather than three statements: the three numbers describe one register at
// one moment, and three reads could straddle a booking. Each FILTER clause is the same metricPredicate
// the drill-down list uses.
//
// ⚠ COST, STATED: this counts one commune's whole live register on every call. The partitioning is by
// commune and the register grows by a few thousand rows a year per commune, so it is affordable today;
// docs/ui-ux/01-tong-quan-dieu-hanh.md §6 already says the dashboard should be materialised, and that is
// the change to make when it stops being affordable — not a cache keyed without the commune.
func (s *IncomingDocumentStore) CountIncomingSummary(ctx context.Context, window domain.ArrivalWindow,
	now time.Time) (domain.IncomingSummary, error) {

	var args []any
	bind := newBinder(&args)

	var cols []string
	for _, f := range []IncomingMetricFilter{
		{Metric: domain.MetricArrived, Window: window},
		{Metric: domain.MetricOpen},
		{Metric: domain.MetricOverdue, Now: now},
	} {
		pred, err := metricPredicate(f, bind)
		if err != nil {
			return domain.IncomingSummary{}, err
		}
		cols = append(cols, "count(*) FILTER (WHERE "+pred+")")
	}

	rows, err := s.db.For(ctx).Query(ctx, strings.Join(cols, ", "), "van_ban_den",
		"AND deleted_at IS NULL", args...)
	if err != nil {
		return domain.IncomingSummary{}, fmt.Errorf("van_ban_den: đếm tổng quan: %w", err)
	}
	defer rows.Close()

	var sum domain.IncomingSummary
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.IncomingSummary{}, fmt.Errorf("van_ban_den: đếm tổng quan: %w", err)
		}
		// An aggregate without GROUP BY always yields one row; none is a driver fault, not a zero.
		return domain.IncomingSummary{}, errors.New("van_ban_den: đếm tổng quan không trả dòng nào")
	}
	if err := rows.Scan(&sum.Arrived, &sum.Open, &sum.Overdue); err != nil {
		return domain.IncomingSummary{}, fmt.Errorf("van_ban_den: đọc số đếm tổng quan: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.IncomingSummary{}, fmt.Errorf("van_ban_den: đếm tổng quan: %w", err)
	}
	return sum, nil
}

// OverdueIncoming reads at most `limit` overdue documents, the longest-missed deadline first.
//
// ORDER BY han_xu_ly_xong, then `id`: the earliest missed deadline is the most late, which is §5's
// "sắp xếp giảm dần theo mức độ trễ", and the id keeps two equal deadlines from swapping between loads.
//
// ⚠ NO INDEX SERVES THIS ORDER DIRECTLY. `van_ban_den_theo_bo_phan` is (tenant_id, bo_phan_dang_giu_id,
// han_xu_ly_xong); PostgreSQL scans the commune's partition and sorts the overdue rows, which is a
// handful. An index on (tenant_id, han_xu_ly_xong) WHERE deleted_at IS NULL is the migration to add if
// that stops being true — not added here, because this change carries no migration.
func (s *IncomingDocumentStore) OverdueIncoming(ctx context.Context, now time.Time,
	limit int) ([]domain.IncomingDocument, error) {

	if limit < 1 || limit > domain.OverdueQueueMax {
		return nil, fmt.Errorf("van_ban_den: giới hạn hàng đợi quá hạn %d ngoài [1, %d]",
			limit, domain.OverdueQueueMax)
	}
	var args []any
	bind := newBinder(&args)
	pred, err := metricPredicate(IncomingMetricFilter{Metric: domain.MetricOverdue, Now: now}, bind)
	if err != nil {
		return nil, err
	}
	tail := "AND deleted_at IS NULL AND " + pred +
		" ORDER BY han_xu_ly_xong ASC, id ASC LIMIT " + bind(limit)

	rows, err := s.db.For(ctx).Query(ctx, incomingDocumentColumns, "van_ban_den", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("van_ban_den: đọc hàng đợi quá hạn: %w", err)
	}
	defer rows.Close()

	out := make([]domain.IncomingDocument, 0, limit)
	for rows.Next() {
		d, err := scanIncomingDocument(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("van_ban_den: quét dòng hàng đợi quá hạn: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("van_ban_den: duyệt hàng đợi quá hạn: %w", err)
	}
	return out, nil
}
