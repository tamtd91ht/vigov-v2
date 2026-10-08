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

// ErrCitizenLetterDeadlineUnusable marks FAILED_PRECONDITION from ResolveCitizenLetterDeadline: the
// commune HAS a rule for this letter type and deadline kind but it cannot be used — a unit the
// statute does not allow, an amount out of range, or a working calendar the count cannot walk.
//
// IT IS NEVER "not configured". The caller REFUSES the act (booking, or the move to `thu-ly`) with a
// configuration sentence; storing no deadline here would count the letter ON TIME (ADR 0084 #4) on the
// strength of a broken configuration (identity.proto, the RPC comment).
var ErrCitizenLetterDeadlineUnusable = errors.New("identityclient: quy tắc hạn đơn thư của xã không dùng được — cần sửa cấu hình")

// ResolveCitizenLetterDeadline asks identity when ONE deadline of ONE citizen letter of the commune
// in ctx falls due (ADR 0085 B).
//
// THREE OUTCOMES, AND THE SIGNATURE KEEPS THEM APART so "not configured" cannot be confused with an
// error or with a deadline:
//
//	dueAt != nil, configured == true,  err == nil   store *dueAt AS GIVEN, in the column of `kind`
//	dueAt == nil, configured == false, err == nil   the commune has no rule: store NULL ("Không đặt hạn")
//	err != nil                                       REFUSE THE ACT. Never store NULL for an error.
//	                                                 errors.Is(err, ErrIdentityUnavailable) → 503, retry;
//	                                                 errors.Is(err, ErrCitizenLetterDeadlineUnusable) →
//	                                                 the configuration must be fixed; anything else → 503.
//
// countFrom per kind (identity.proto): PROCESSING — 00:00 Asia/Ho_Chi_Minh of the received DATE;
// RESOLUTION — `accepted_at` as stored. The caller never adds or trims anything; which day is day one
// and what hour the last day ends at are decided in identity.
//
// CALL IT ONCE, AT THE ACT THAT FIXES THE CLOCK (rule 10, invariant 2), never to re-derive a stored
// deadline. Not cached.
func (c *Client) ResolveCitizenLetterDeadline(ctx context.Context, letterType identityv1.CitizenLetterType,
	kind identityv1.CitizenLetterDeadlineKind, countFrom time.Time) (dueAt *time.Time, configured bool, err error) {

	// Refused locally: sent, each comes back as an INVALID_ARGUMENT naming a wire field, and the
	// operator would look at the contract for a fault that is in the caller's wiring.
	if letterType == identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_UNSPECIFIED {
		return nil, false, errors.New("identityclient: ResolveCitizenLetterDeadline không có loại đơn")
	}
	if kind == identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_UNSPECIFIED {
		return nil, false, errors.New("identityclient: ResolveCitizenLetterDeadline không nói rõ hạn xử lý đơn hay hạn giải quyết")
	}
	if countFrom.IsZero() {
		return nil, false, errors.New("identityclient: ResolveCitizenLetterDeadline với gốc đếm rỗng")
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	res, err := c.cl.ResolveCitizenLetterDeadline(ctx, &identityv1.ResolveCitizenLetterDeadlineRequest{
		LetterType: letterType,
		Kind:       kind,
		CountFrom:  timestamppb.New(countFrom),
	})
	if err != nil {
		// No personal data in the request or here: an enum pair and an instant.
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được hạn đơn thư của xã",
			"ma_loi", status.Code(err).String(), "loai_don", letterType.String(), "loai_han", kind.String(), "err", err)
		return nil, false, wrapCallError("ResolveCitizenLetterDeadline", err, ErrCitizenLetterDeadlineUnusable)
	}

	switch {
	case res.GetDeadline() != nil:
		ts := res.GetDeadline().GetDueAt()
		if ts == nil {
			// NEVER the zero time.Time: a deadline in year 1 is a letter overdue the moment it is booked.
			return nil, false, errors.New("identityclient: ResolveCitizenLetterDeadline trả deadline không có due_at")
		}
		if err := ts.CheckValid(); err != nil {
			return nil, false, fmt.Errorf("identityclient: ResolveCitizenLetterDeadline trả due_at không hợp lệ: %w", err)
		}
		t := ts.AsTime()
		return &t, true, nil
	case res.GetNotConfigured() != nil:
		return nil, false, nil
	}
	// Neither set is a contract fault (identity.proto): the caller refuses, as for an error.
	return nil, false, errors.New("identityclient: ResolveCitizenLetterDeadline không trả deadline lẫn not_configured")
}
