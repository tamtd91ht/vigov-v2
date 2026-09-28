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
	"time"

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

// WorkingHoursCalculator is identity's AdvanceWorkingHours, as core/identityclient exposes it.
// *identityclient.Client satisfies it — the SAME client the deadline paths already use, so this panel
// and those paths never disagree about identity's health.
type WorkingHoursCalculator interface {
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	TienGioLamViec(ctx context.Context, tuLuc time.Time, gio []uint32) (map[uint32]time.Time, error)
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
}

func NewOverdueQueue(tasks TaskOverdueReader, reports CitizenReportOverdueReader,
	hours WorkingHoursCalculator) *OverdueQueue {
	return &OverdueQueue{tasks: tasks, reports: reports, hours: hours, clock: time.Now}
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
	return q.markCritical(ctx, items)
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
