package app

// The "Cần xử lý ngay" panel of the leadership overview: the currently-overdue tasks and petitions,
// each marked critical or not.
//
// A USE CASE AND NOT A STORE CALL, for one reason: `critical` needs the commune's working-hours
// calendar, which identity owns (ADR 0007). "Two working days past the deadline" is not 48 wall-clock
// hours — over a weekend or a Tết holiday the two differ by days — so the instant is asked of identity
// and never computed here (rule 10, forbidden #2).
//
// NOTHING IS WRITTEN AND NOTHING IS AUDITED: this is a read of codes, category keys and deadlines, with
// no personal data (domain.OverdueItem), inside one commune (rule 6, invariant 7 audits full personal
// data and cross-commune reads; this is neither).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// TaskOverdueReader is the store read behind the task panel. *store.NhiemVuStore satisfies it.
type TaskOverdueReader interface {
	OverdueTasks(ctx context.Context, limit int) ([]domain.OverdueItem, error)
}

// CitizenReportOverdueReader is the store read behind the petition panel. *store.PhieuPhanAnhStore
// satisfies it. `restricted` is the caller's `feedback.restricted` fact — false excludes `can-bo`.
type CitizenReportOverdueReader interface {
	OverdueCitizenReports(ctx context.Context, limit int, restricted bool) ([]domain.OverdueItem, error)
}

// WorkingHoursCalculator is identity's AdvanceWorkingHours and MeasureWorkingHours, as
// core/identityclient exposes them. *identityclient.Client satisfies it — the SAME client the deadline
// paths already use, so this panel and those paths never disagree about identity's health.
type WorkingHoursCalculator interface {
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	TienGioLamViec(ctx context.Context, tuLuc time.Time, gio []uint32) (map[uint32]time.Time, error)

	// MeasureWorkingSeconds answers the working time inside each span, one calendar read per call
	// (commit 1c28348a). Used for the petition panel's `late_working_seconds` only.
	MeasureWorkingSeconds(ctx context.Context, spans []identityclient.WorkingSpan) (
		map[identityclient.WorkingSpan]uint64, error)
}

// ErrWorkingHoursUnavailable means identity could not say when an item becomes critical.
//
// THE REQUEST FAILS; NO ITEM IS SILENTLY MARKED NOT CRITICAL. A panel that answered `critical: false`
// for everything while identity was down would show a leader a calm list at exactly the moment the
// calendar service is broken — a plausible screen that is wrong, with nothing to say so.
var ErrWorkingHoursUnavailable = errors.New("không tính được mốc nghiêm trọng theo giờ làm việc")

// OverdueQueue builds both panels.
type OverdueQueue struct {
	tasks   TaskOverdueReader
	reports CitizenReportOverdueReader
	hours   WorkingHoursCalculator

	// clock is the seam critical compares identity's instant with. Tests replace it; nothing else does.
	clock func() time.Time

	// log records why `late_working_seconds` is absent from a page — the field degrades silently on
	// the wire by design, so the cause has to be somewhere an operator can read it.
	log *slog.Logger
}

func NewOverdueQueue(tasks TaskOverdueReader, reports CitizenReportOverdueReader,
	hours WorkingHoursCalculator) *OverdueQueue {
	return &OverdueQueue{tasks: tasks, reports: reports, hours: hours, clock: time.Now, log: slog.Default()}
}

// Tasks returns at most `limit` overdue tasks, oldest missed deadline first, each with Critical set.
func (q *OverdueQueue) Tasks(ctx context.Context, limit int) ([]domain.OverdueItem, error) {
	items, err := q.tasks.OverdueTasks(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("hàng đợi quá hạn nhiệm vụ: %w", err)
	}
	return q.markCritical(ctx, items)
}

// CitizenReports returns at most `limit` overdue petitions. `restricted` is the caller's
// `feedback.restricted` fact, the same type every petition act takes (QuyenXemHanChe).
func (q *OverdueQueue) CitizenReports(ctx context.Context, limit int, restricted QuyenXemHanChe) (
	[]domain.OverdueItem, error) {

	items, err := q.reports.OverdueCitizenReports(ctx, limit, bool(restricted))
	if err != nil {
		return nil, fmt.Errorf("hàng đợi quá hạn phản ánh: %w", err)
	}
	items, err = q.markCritical(ctx, items)
	if err != nil {
		return nil, err
	}
	return q.measureLate(ctx, items), nil
}

// measureLate fills LateWorkingSeconds — the working time between each missed deadline and the `now`
// the overdue predicate used (OverdueItem.AsOf) — in ONE MeasureWorkingHours call per
// identityclient.MaxMeasuredSpansPerCall spans. The page is at most OverdueQueueMax (ten) rows, so that
// is one call; the chunking exists so a larger page can never become a refused request.
//
// ALL OR NOTHING, AND NOTHING IS NOT AN ERROR. Any failure — identity unavailable, the commune's
// calendar not configured, a span that cannot be measured, a short answer — leaves the field ABSENT on
// EVERY item and the page is still returned: "Quá hạn" is derivable without it (the proto comment), and
// a page where some rows carry a number and some do not would read as "those rows are not late". NEVER
// 0 and never a default. ADR 0007 decision 10: the screen shows no number yet; this only makes one
// available. Never stored.
func (q *OverdueQueue) measureLate(ctx context.Context, items []domain.OverdueItem) []domain.OverdueItem {
	if len(items) == 0 {
		return items
	}
	spans := make([]identityclient.WorkingSpan, 0, len(items))
	for _, it := range items {
		// The predicate guarantees MissedDeadline < AsOf; checked anyway, because a zero AsOf (a store
		// that did not fill it) would otherwise be sent as a span ending in the year 1.
		if it.AsOf.IsZero() || it.MissedDeadline.IsZero() || it.AsOf.Before(it.MissedDeadline) {
			q.log.WarnContext(ctx, "CẢNH BÁO: không đo được thời gian trễ theo giờ làm việc — dòng thiếu mốc",
				"xa", string(tenant.MustFrom(ctx)))
			return items
		}
		spans = append(spans, identityclient.WorkingSpan{Start: it.MissedDeadline.UTC(), End: it.AsOf.UTC()})
	}

	measured := make(map[identityclient.WorkingSpan]uint64, len(spans))
	for start := 0; start < len(spans); start += identityclient.MaxMeasuredSpansPerCall {
		end := min(start+identityclient.MaxMeasuredSpansPerCall, len(spans))
		got, err := q.hours.MeasureWorkingSeconds(ctx, spans[start:end])
		if err != nil {
			// identityclient already logged the gRPC code; the commune says which calendar to open.
			q.log.WarnContext(ctx, "CẢNH BÁO: không đo được thời gian trễ theo giờ làm việc — trả hàng đợi không kèm số",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			return items
		}
		for k, v := range got {
			measured[k] = v
		}
	}

	out := make([]domain.OverdueItem, len(items))
	for i, it := range items {
		v, ok := measured[spans[i]]
		if !ok {
			// identityclient refuses a short answer itself; this is the floor for a fake or a future
			// client. One row unmeasured -> no row measured.
			q.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: thiếu một khoảng đã hỏi — trả hàng đợi không kèm số",
				"xa", string(tenant.MustFrom(ctx)))
			return items
		}
		late := v
		it.LateWorkingSeconds = &late
		out[i] = it
	}
	return out
}

// markCritical asks identity ONCE PER DISTINCT MISSED DEADLINE, and at most OverdueQueueMax times.
//
// ONE CALL PER DEADLINE AND NOT ONE BATCHED CALL because the contract takes a single `count_from` per
// request (proto AdvanceWorkingHoursRequest); the amounts are what it batches, and here the amount is
// constant while the origin varies. The store bounds the list at ten, so the calls are bounded at ten,
// and identical deadlines — common when a leader assigns a batch with one date — are asked once.
//
// THE FIRST FAILURE ENDS THE REQUEST. There is no partial answer: an item without a critical flag
// would have to be given one, and whichever value it got would be a guess (ErrWorkingHoursUnavailable).
func (q *OverdueQueue) markCritical(ctx context.Context, items []domain.OverdueItem) (
	[]domain.OverdueItem, error) {

	if len(items) == 0 {
		return items, nil
	}
	amounts := []uint32{domain.CriticalWorkingHours}
	criticalFrom := make(map[time.Time]time.Time, len(items))
	for _, it := range items {
		// Keyed on the UTC instant so two representations of one moment are one call.
		key := it.MissedDeadline.UTC()
		if _, seen := criticalFrom[key]; seen {
			continue
		}
		reached, err := q.hours.TienGioLamViec(ctx, key, amounts)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrWorkingHoursUnavailable, err)
		}
		at, ok := reached[domain.CriticalWorkingHours]
		if !ok || at.IsZero() {
			// identityclient already refuses a partial answer; this is the floor if a fake or a future
			// client does not. A zero instant would read as "critical since year 1".
			return nil, fmt.Errorf("%w: thiếu mốc %d giờ làm việc", ErrWorkingHoursUnavailable,
				domain.CriticalWorkingHours)
		}
		criticalFrom[key] = at
	}

	now := q.clock()
	out := make([]domain.OverdueItem, len(items))
	for i, it := range items {
		it.Critical = domain.IsCritical(criticalFrom[it.MissedDeadline.UTC()], now)
		out[i] = it
	}
	return out, nil
}
