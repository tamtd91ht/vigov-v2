package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// MeasureWorkingHours — how much of this commune's WORKING time lies inside each span the caller
// holds; the inverse of AdvanceWorkingHours. The full contract (why it exists, the ten-year bound, why
// an empty calendar is refused rather than 0, the status table) is on the RPC in identity.proto.
//
// THIS FILE VALIDATES AND JOINS; IT DOES NOT COUNT. The calendar read is Server.readCalendar, the fault
// mapping is Server.loiLich, and the measuring is domain.MeasureWorkingTime over the same workingWalk
// as every deadline (ADR 0007: no second implementation of the working-hours count).
//
// ORDER: every caller fault first, before any read — a malformed call must not become read load on a
// process serving 200+ communes → commune present → calendar → measure. A read of configuration: it
// writes nothing, publishes nothing, and leaves no audit entry.

// MaxMeasuredSpans — 1 to 500 spans, checked on what the caller SENT, before duplicates collapse. The
// bound ResolveEscalationInstants uses, for the same reason: never a silent truncation.
const MaxMeasuredSpans = 500

// MeasureCallYears is how wide ONE call may be: earliest start to latest end, in calendar years in the
// commune's zone. It refuses a GARBAGE instant (an unset column read as year 1, the Unix epoch) that
// would walk decades of calendar for one row — exceeding it is the caller's fault by construction, so
// it is INVALID_ARGUMENT, never the commune's FAILED_PRECONDITION. It is NOT AdvanceWorkingHours'
// 366-day horizon; the contract explains why.
const MeasureCallYears = 10

// spanKey compares a span AS TWO INSTANTS (seconds and nanos), which is how the contract tells the
// caller to map the response.
type spanKey struct{ start, end instantKey }

func (s *Server) MeasureWorkingHours(ctx context.Context, req *identityv1.MeasureWorkingHoursRequest) (
	*identityv1.MeasureWorkingHoursResponse, error) {

	sent := req.GetSpans()
	if len(sent) == 0 {
		return nil, status.Error(codes.InvalidArgument, "thiếu spans")
	}
	if len(sent) > MaxMeasuredSpans {
		return nil, status.Errorf(codes.InvalidArgument,
			"spans có %d mục, vượt trần %d — bên gọi phải chia lô", len(sent), MaxMeasuredSpans)
	}

	distinct := make([]*identityv1.WorkingTimeSpan, 0, len(sent))
	spans := make([]domain.WorkingSpan, 0, len(sent))
	seen := make(map[spanKey]struct{}, len(sent))
	var earliest, latest time.Time
	for _, sp := range sent {
		if sp == nil {
			// Never skipped: skipping returns fewer items than asked for, which the contract calls a fault.
			return nil, status.Error(codes.InvalidArgument, "spans có mục trống")
		}
		start, end := sp.GetStart(), sp.GetEnd()
		if start == nil || end == nil {
			// NO DEFAULT for either end: no "from now", and no server-side `now` — two clocks one request
			// apart make a record late by a negative amount.
			return nil, status.Error(codes.InvalidArgument, "spans có mục thiếu start hoặc end")
		}
		if start.CheckValid() != nil || end.CheckValid() != nil {
			return nil, status.Error(codes.InvalidArgument, "spans có mục không phải mốc thời gian hợp lệ")
		}
		a, b := start.AsTime(), end.AsTime()
		if b.Before(a) {
			// Never a negative number and never a silent 0: a 0 reads as "not late" and hides a caller
			// that swapped its two instants.
			return nil, status.Error(codes.InvalidArgument,
				"spans có mục end trước start — thời gian còn lại cũng là một khoảng: gửi (now, deadline)")
		}
		if len(spans) == 0 || a.Before(earliest) {
			earliest = a
		}
		if len(spans) == 0 || b.After(latest) {
			latest = b
		}
		k := spanKey{instantKey{start.GetSeconds(), start.GetNanos()}, instantKey{end.GetSeconds(), end.GetNanos()}}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		distinct = append(distinct, sp)
		spans = append(spans, domain.WorkingSpan{Start: a, End: b})
	}

	// CALENDAR YEARS IN THE COMMUNE'S ZONE, not 3650 days: the contract states the bound in the zone,
	// and identity is the one place calendar arithmetic is allowed (ADR 0007).
	zone, err := domain.MuiGio()
	if err != nil {
		return nil, s.loi(ctx, err, "MeasureWorkingHours/mui_gio")
	}
	if latest.After(earliest.In(zone).AddDate(MeasureCallYears, 0, 0)) {
		return nil, status.Errorf(codes.InvalidArgument,
			"spans trải quá %d năm từ start sớm nhất tới end muộn nhất — có mốc chưa đặt hoặc sai ở bên gọi",
			MeasureCallYears)
	}

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}
	week, readYear, err := s.readCalendar(ctx, xa, "MeasureWorkingHours")
	if err != nil {
		return nil, err
	}
	measured, err := domain.MeasureWorkingTime(spans, week, readYear)
	if err != nil {
		return nil, s.loiLich(ctx, err, xa, "MeasureWorkingHours/do")
	}

	// ONE ITEM PER DISTINCT SPAN, each echoing the very span sent — the key the caller maps by.
	items := make([]*identityv1.WorkingTimeMeasured, 0, len(distinct))
	for i, sp := range distinct {
		items = append(items, &identityv1.WorkingTimeMeasured{
			Span: &identityv1.WorkingTimeSpan{Start: sp.GetStart(), End: sp.GetEnd()},
			// WHOLE SECONDS, truncated toward zero; never negative (end >= start was checked above).
			WorkingSeconds: uint64(measured[i] / time.Second),
		})
	}
	return &identityv1.MeasureWorkingHoursResponse{Items: items}, nil
}
