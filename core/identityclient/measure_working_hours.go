package identityclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// ErrWorkingCalendarNotConfigured marks FAILED_PRECONDITION from MeasureWorkingHours: this commune's
// working calendar is missing or wrong. The caller shows "chưa cấu hình lịch làm việc" and NEVER a
// number — an empty calendar is not "0 hours late", and a 0 reads as "not late" on every row.
var ErrWorkingCalendarNotConfigured = errors.New("identityclient: xã chưa cấu hình lịch làm việc")

// MaxMeasuredSpansPerCall is the contract's 1–500, checked on what the caller sends.
const MaxMeasuredSpansPerCall = 500

// WorkingSpan is [Start, End) — two instants the caller already holds. For lateness: (stored
// deadline, the same `now` the `late` filter compares against). Remaining time is (now, deadline).
type WorkingSpan struct {
	Start time.Time
	End   time.Time
}

// MeasureWorkingSeconds asks identity how much of the commune's WORKING time lies inside each span,
// in whole seconds, against ONE calendar read for the whole batch. The full contract is on
// MeasureWorkingHours in identity.proto.
//
// THE MAP IS KEYED BY THE SPAN WITH BOTH INSTANTS IN UTC — look it up with the same normalisation
// (`WorkingSpan{a.UTC(), b.UTC()}`). Duplicates collapse; a span asked for and not answered is a
// contract fault, never a 0.
//
// AN INTEGER COUNT OF SECONDS AND NOT A time.Duration, on purpose: `deadline.Add(d)` must not
// type-check. NEVER ADD IT TO AN INSTANT — that walks straight through the nights, weekends and
// holidays this RPC exists to exclude (rule 10, forbidden #2) — and never store it: it moves with
// `now` and with the calendar (rule 10, invariant 3).
//
// # ERRORS — NONE OF THEM IS "0 seconds late"
//
//	errors.Is(err, ErrWorkingCalendarNotConfigured)  the commune's calendar is missing or wrong.
//	errors.Is(err, ErrIdentityUnavailable)           identity did not answer; retryable. Show no
//	                                                 figure — "Quá hạn" is still derivable without it.
//	anything else                                    the call did not happen or the contract broke.
func (c *Client) MeasureWorkingSeconds(ctx context.Context, spans []WorkingSpan) (map[WorkingSpan]uint64, error) {
	if len(spans) == 0 || len(spans) > MaxMeasuredSpansPerCall {
		return nil, fmt.Errorf("identityclient: MeasureWorkingSeconds với %d khoảng, cần 1–%d — bên gọi phải chia trang",
			len(spans), MaxMeasuredSpansPerCall)
	}
	seen := make(map[WorkingSpan]bool, len(spans))
	distinct := make([]WorkingSpan, 0, len(spans))
	req := make([]*identityv1.WorkingTimeSpan, 0, len(spans))
	for _, sp := range spans {
		// REFUSED LOCALLY, naming the cause: a zero instant is an unset deadline column, which the
		// server would answer as a ten-year INVALID_ARGUMENT about the year 1.
		if sp.Start.IsZero() || sp.End.IsZero() {
			return nil, errors.New("identityclient: MeasureWorkingSeconds với một mốc rỗng — bên gọi chưa đặt hạn hoặc `now`")
		}
		if sp.End.Before(sp.Start) {
			return nil, errors.New("identityclient: MeasureWorkingSeconds với end trước start — thời gian còn lại gửi (now, deadline)")
		}
		k := WorkingSpan{Start: sp.Start.UTC(), End: sp.End.UTC()}
		if seen[k] {
			continue
		}
		seen[k] = true
		distinct = append(distinct, k)
		req = append(req, &identityv1.WorkingTimeSpan{Start: timestamppb.New(k.Start), End: timestamppb.New(k.End)})
	}

	// THE SAME DEADLINE AS every other method of this package — see HanGoi. One call per page.
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.MeasureWorkingHours(ctx, &identityv1.MeasureWorkingHoursRequest{Spans: req})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không đo được giờ làm việc", "ma_loi", status.Code(err).String())
		return nil, wrapCallError("MeasureWorkingHours", err, ErrWorkingCalendarNotConfigured)
	}

	out := make(map[WorkingSpan]uint64, len(distinct))
	for _, it := range resp.GetItems() {
		start, err := instant(it.GetSpan().GetStart(), "span.start")
		if err != nil {
			return nil, err
		}
		end, err := instant(it.GetSpan().GetEnd(), "span.end")
		if err != nil {
			return nil, err
		}
		k := WorkingSpan{Start: start, End: end}
		if !seen[k] {
			return nil, errors.New("identityclient: MeasureWorkingHours trả khoảng không được hỏi — lỗi hợp đồng")
		}
		out[k] = it.GetWorkingSeconds()
	}
	for _, k := range distinct {
		if _, ok := out[k]; !ok {
			c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: MeasureWorkingHours thiếu một khoảng đã hỏi")
			return nil, errors.New("identityclient: MeasureWorkingHours trả thiếu khoảng — lỗi hợp đồng, không phải 0")
		}
	}
	return out, nil
}
