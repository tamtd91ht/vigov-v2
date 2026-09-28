package identityclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// DueSoonCutoff asks identity for the LATEST deadline that counts as "sắp đến hạn" at asOf, for ONE
// kind of work and ONE field, in the commune the context carries.
//
// THE ONE COMPARISON THE CALLER MAKES WITH IT:
//
//	due soon   ⟺   asOf < deadline  AND  deadline <= cutoff
//
// asOf MUST BE THE SAME `now` THE CALLER USES FOR ITS `late` FILTER. Two clocks one request apart put
// a record on neither list or on both. That is why there is no server-side "now" and no local one
// here.
//
// # AN INSTANT FOR A COMPARISON, NEVER FOR STORAGE
//
// The answer moves with asOf. Writing it onto a record builds a stored "is due soon" state, the class
// rule 10, invariant 3 forbids. It is not cached for the same reason.
//
// # WHY IT RETURNS AN INSTANT AND NOT HOURS
//
// ADR 0029 §Bổ sung: with the hours in hand the only arithmetic within reach is `now + N*time.Hour`,
// which counts nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` (rule 10, forbidden #2). The
// forward walk that makes this correct happens in identity.
//
// # ERRORS — NONE OF THEM IS "nothing is due soon"
//
//	errors.Is(err, ErrDueSoonNotConfigured)  the commune has no usable threshold or calendar. Refuse
//	                                         the filter with a clear sentence; never substitute 72.
//	errors.Is(err, ErrIdentityUnavailable)   identity did not answer. 503, retryable.
//	anything else                            the call did not happen or the contract broke. 503.
func (c *Client) DueSoonCutoff(ctx context.Context, kind identityv1.WorkKind, linhVuc string,
	asOf time.Time) (time.Time, error) {

	// REFUSED LOCALLY, each message naming the cause rather than a wire field.
	if kind == identityv1.WorkKind_WORK_KIND_UNSPECIFIED {
		// There is no default kind of work: guessing one reads another domain's threshold.
		return time.Time{}, fmt.Errorf(
			"identityclient: DueSoonCutoff không có loại việc — bên gọi phải nói rõ van-ban-den, phan-anh hay nhiem-vu")
	}
	if asOf.IsZero() {
		return time.Time{}, fmt.Errorf(
			"identityclient: DueSoonCutoff với mốc so sánh rỗng — bên gọi phải truyền đúng `now` của trang đang dựng")
	}

	// THE SAME DEADLINE AS every other method of this package — see HanGoi. One call per list
	// request, never one per row.
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveDueSoonCutoff(ctx, &identityv1.ResolveDueSoonCutoffRequest{
		WorkKind: kind,
		// "" IS A REAL REQUEST — the default row, and what a task caller sends today (tasks carry no
		// field). Passed straight through, never substituted.
		LinhVuc: linhVuc,
		AsOf:    timestamppb.New(asOf),
	})
	if err != nil {
		// Logged with the code: FAILED_PRECONDITION is fixed on a configuration screen, UNAVAILABLE is
		// not. Nothing personal is in the request.
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được ngưỡng sắp đến hạn của xã",
			"ma_loi", status.Code(err).String(), "loai_viec", kind.String(), "err", err)
		return time.Time{}, wrapCallError("ResolveDueSoonCutoff", err, ErrDueSoonNotConfigured)
	}

	ts := resp.GetDueSoonUntil()
	if ts == nil {
		// NEVER mapped to the zero time.Time: a cutoff in year 1 filters every record OUT of "due
		// soon", which reads as "all clear". The contract says OK always carries it.
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveDueSoonCutoff trả OK mà không có due_soon_until")
		return time.Time{}, fmt.Errorf("identityclient: ResolveDueSoonCutoff trả OK mà không có due_soon_until")
	}
	if err := ts.CheckValid(); err != nil {
		return time.Time{}, fmt.Errorf("identityclient: ResolveDueSoonCutoff trả mốc không hợp lệ: %w", err)
	}
	cutoff := ts.AsTime()
	if cutoff.Before(asOf) {
		// The contract says the cutoff is at or after as_of. Before it, the filter `asOf < D <= cutoff`
		// is empty by construction — the same silent "all clear".
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveDueSoonCutoff trả mốc trước as_of")
		return time.Time{}, fmt.Errorf("identityclient: ResolveDueSoonCutoff trả mốc trước as_of — lỗi hợp đồng")
	}
	return cutoff, nil
}
