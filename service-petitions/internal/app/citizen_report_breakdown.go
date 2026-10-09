package app

// GET /api/v1/citizen-report-breakdown — the /phan-anh statistics for one period (owner decisions of
// 09/10/2026, batch A): the counts the store reads in one statement, plus the per-unit handling time,
// which needs identity's working-hours calendar.
//
// A USE CASE AND NOT A STORE CALL for overdue_queue.go's reason: "how long did unit X take" is WORKING
// time — nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` excluded — and that is identity's answer
// (ADR 0007; MeasureWorkingHours, identity.proto). A subtraction of two instants here is rule 10,
// forbidden #2, and its answer over a Tết week is off by days.
//
// FAIL CLOSED: identity unavailable, the calendar missing, a short answer — the whole read is REFUSED
// (ErrWorkingHoursUnavailable / ErrWorkingCalendarMissing), never answered with a wall-clock fallback and
// never with the unit section quietly emptied. A table of "average handling time" that silently turned
// into clock hours, or into nothing, is a plausible figure about an authority that is wrong.
//
// NOTHING IS WRITTEN AND NOTHING IS AUDITED: counts, unit ids and seconds of one commune, no personal
// data (rule 6, invariant 7 asks for neither case).

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CitizenReportBreakdownReader is the store half. *petstore.PhieuPhanAnhStore satisfies it.
// `restricted` is the reader's `feedback.restricted` fact — false excludes `can-bo` from both reads.
type CitizenReportBreakdownReader interface {
	CitizenReportBreakdown(ctx context.Context, p domain.Period, restricted bool) (domain.CitizenReportBreakdown, error)
	CitizenReportHandling(ctx context.Context, p domain.Period, restricted bool) ([]domain.CitizenReportHandling, error)
}

// WorkingTimeMeasurer is identity's MeasureWorkingHours as core/identityclient exposes it.
// *identityclient.Client satisfies it — the client every deadline path already uses.
type WorkingTimeMeasurer interface {
	MeasureWorkingSeconds(ctx context.Context, spans []identityclient.WorkingSpan) (
		map[identityclient.WorkingSpan]uint64, error)
}

// ErrWorkingCalendarMissing means the commune's working calendar is not configured, so working time
// cannot be measured. A configuration gap the commune can fix, not an outage — the route says which.
var ErrWorkingCalendarMissing = errors.New("xã chưa cấu hình lịch làm việc nên chưa đo được thời gian xử lý")

// CitizenReportBreakdown builds the statistics read.
type CitizenReportBreakdown struct {
	reports CitizenReportBreakdownReader
	hours   WorkingTimeMeasurer
}

// NewCitizenReportBreakdown builds the use case. A nil `hours` is refused at the first read
// (ErrWorkingHoursUnavailable), never answered without the unit section.
func NewCitizenReportBreakdown(reports CitizenReportBreakdownReader, hours WorkingTimeMeasurer) *CitizenReportBreakdown {
	return &CitizenReportBreakdown{reports: reports, hours: hours}
}

// Read returns the breakdown of period `p`. `restricted` is QuyenXemHanChe, the type every petition
// read and act takes.
func (uc *CitizenReportBreakdown) Read(ctx context.Context, p domain.Period, restricted QuyenXemHanChe) (
	domain.CitizenReportBreakdown, error) {

	out, err := uc.reports.CitizenReportBreakdown(ctx, p, bool(restricted))
	if err != nil {
		return domain.CitizenReportBreakdown{}, fmt.Errorf("thống kê phản ánh theo kỳ: %w", err)
	}
	handled, err := uc.reports.CitizenReportHandling(ctx, p, bool(restricted))
	if err != nil {
		return domain.CitizenReportBreakdown{}, fmt.Errorf("thống kê phản ánh theo kỳ: thời gian xử lý: %w", err)
	}
	units, err := uc.unitFigures(ctx, handled)
	if err != nil {
		return domain.CitizenReportBreakdown{}, err
	}
	out.Units = units
	return out, nil
}

// unitFigures groups the finished petitions by the unit holding each, and sums the working time of
// those whose hand-over is recorded — ONE MeasureWorkingHours call per identityclient.MaxMeasuredSpansPerCall
// spans, never one per petition (skills/load-data-once).
//
// THE ORDER IS THE STORE'S FIRST APPEARANCE OF EACH UNIT and means nothing; the client sorts and adds
// names and zero rows from GET /api/v1/org-units, as for task-unit-summary.
func (uc *CitizenReportBreakdown) unitFigures(ctx context.Context, handled []domain.CitizenReportHandling) (
	[]domain.CitizenReportUnitFigures, error) {

	out := []domain.CitizenReportUnitFigures{}
	if len(handled) == 0 {
		return out, nil
	}

	spans := make([]identityclient.WorkingSpan, 0, len(handled))
	for _, h := range handled {
		if h.AssignedAt.IsZero() {
			continue
		}
		if h.FinishedAt.Before(h.AssignedAt) {
			// The statement only reads hand-overs at or before the finishing instant; a row that says
			// otherwise is a broken statement, and measuring it would send identity a refused span.
			return nil, errors.New("thống kê phản ánh theo kỳ: mốc giao việc sau mốc xử lý xong — câu đọc sai")
		}
		spans = append(spans, identityclient.WorkingSpan{Start: h.AssignedAt.UTC(), End: h.FinishedAt.UTC()})
	}
	measured, err := uc.measure(ctx, spans)
	if err != nil {
		return nil, err
	}

	index := map[string]int{}
	for _, h := range handled {
		i, ok := index[h.OrgUnitID]
		if !ok {
			i = len(out)
			index[h.OrgUnitID] = i
			out = append(out, domain.CitizenReportUnitFigures{OrgUnitID: h.OrgUnitID})
		}
		out[i].Finished++
		if h.AssignedAt.IsZero() {
			continue
		}
		v, ok := measured[identityclient.WorkingSpan{Start: h.AssignedAt.UTC(), End: h.FinishedAt.UTC()}]
		if !ok {
			// identityclient refuses a short answer itself; this is the floor for a fake or a future
			// client. One span unmeasured -> no figure, never a 0 that drags an average down.
			return nil, fmt.Errorf("%w: thiếu một khoảng đã hỏi", ErrWorkingHoursUnavailable)
		}
		out[i].HandlingSample++
		out[i].HandlingWorkingSeconds += v
	}
	return out, nil
}

// measure asks identity in chunks of at most identityclient.MaxMeasuredSpansPerCall. Duplicate spans
// collapse in the map, which is correct: two petitions with the same two instants took the same time.
func (uc *CitizenReportBreakdown) measure(ctx context.Context, spans []identityclient.WorkingSpan) (
	map[identityclient.WorkingSpan]uint64, error) {

	measured := make(map[identityclient.WorkingSpan]uint64, len(spans))
	if len(spans) == 0 {
		return measured, nil
	}
	if uc.hours == nil {
		return nil, fmt.Errorf("%w: chưa nối dây đo giờ làm việc", ErrWorkingHoursUnavailable)
	}
	for start := 0; start < len(spans); start += identityclient.MaxMeasuredSpansPerCall {
		end := min(start+identityclient.MaxMeasuredSpansPerCall, len(spans))
		got, err := uc.hours.MeasureWorkingSeconds(ctx, spans[start:end])
		if errors.Is(err, identityclient.ErrWorkingCalendarNotConfigured) {
			return nil, fmt.Errorf("%w: %w", ErrWorkingCalendarMissing, err)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrWorkingHoursUnavailable, err)
		}
		for k, v := range got {
			measured[k] = v
		}
	}
	return measured, nil
}

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ CitizenReportBreakdownReader = (*petstore.PhieuPhanAnhStore)(nil)
	_ WorkingTimeMeasurer          = (*identityclient.Client)(nil)
)
