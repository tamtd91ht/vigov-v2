package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveCitizenLetterDeadline — the server side of a citizen letter's deadline (ADR 0085 B,
// ADR 0084 #3, ADR 0064). THIS FILE JOINS; IT DOES NOT COUNT. The unit lock and the day count live in
// domain/citizen_letter_deadline.go; the calendar read is Server.readCalendar, the SAME one
// AdvanceWorkingHours and ResolveDeadlines enter through.

// CitizenLetterDeadlineRuleReader reads the ONE live rule for a (letter type, deadline kind) of the
// commune in context. found == false is "not configured". Declared at the point of use;
// *idstore.CitizenLetterDeadlineRuleStore satisfies it.
type CitizenLetterDeadlineRuleReader interface {
	Live(ctx context.Context, letterType domain.CitizenLetterType, kind domain.CitizenLetterDeadlineKind) (
		domain.CitizenLetterDeadlineRule, bool, error)
}

// ResolveCitizenLetterDeadline answers the instant one deadline of one letter falls due — or that the
// commune has no rule for it.
//
// # The order is the contract's (identity.proto, the RPC comment)
//
// CALLER FAULTS FIRST, BEFORE ANY READ: an UNSPECIFIED or unknown enum and a missing count_from are
// INVALID_ARGUMENT, and a misshapen call never becomes read load.
//
// NO LIVE RULE IS AN OK ANSWER — `not_configured`, the ONE answer with which the caller books a letter
// without a deadline (ADR 0085 B3). Nothing falls back to `sla`'s `don-thu` row.
//
// A RULE THAT EXISTS BUT IS UNUSABLE IS FAILED_PRECONDITION, NEVER not_configured: a unit the lock
// refuses (working hours anywhere, working days on a complaint), an amount not positive or over the
// ceiling, or a calendar the count cannot walk. Answering not_configured there would book the letter
// with no deadline, and ADR 0084 #4 counts such a letter ON TIME — a broken configuration would inflate
// the on-time ratio sent upward, silently.
//
// Nothing is written, nothing is audited; retrying is safe. The audited act is the caller's.
func (s *Server) ResolveCitizenLetterDeadline(ctx context.Context, req *identityv1.ResolveCitizenLetterDeadlineRequest) (
	*identityv1.ResolveCitizenLetterDeadlineResponse, error) {

	letterType, err := citizenLetterTypeFrom(req.GetLetterType())
	if err != nil {
		return nil, err
	}
	kind, err := citizenLetterKindFrom(req.GetKind())
	if err != nil {
		return nil, err
	}
	from := req.GetCountFrom()
	if from == nil {
		// No "from now" default: the deadline would depend on when the call happened to be made.
		return nil, status.Error(codes.InvalidArgument, "thiếu count_from")
	}
	if err := from.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "count_from không phải một mốc thời gian hợp lệ")
	}

	// The commune scopes both reads through store.Scoped; this call guards that the commune
	// interceptor is in the chain and keeps the value for the log lines.
	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	rule, found, err := s.d.CitizenLetterRules.Live(ctx, letterType, kind)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveCitizenLetterDeadline/rule")
	}
	if !found {
		return &identityv1.ResolveCitizenLetterDeadlineResponse{
			Outcome: &identityv1.ResolveCitizenLetterDeadlineResponse_NotConfigured{
				NotConfigured: &identityv1.CitizenLetterDeadlineNotConfigured{},
			},
		}, nil
	}

	// THE SAME CHECK THE CONFIGURATION ROUTES RUN ON A WRITE. A stored row it refuses is never used.
	if err := domain.CheckCitizenLetterDeadlineRule(rule); err != nil {
		s.d.Log.WarnContext(ctx, "ResolveCitizenLetterDeadline: quy tắc hạn đơn thư của xã không dùng được — TỪ CHỐI",
			"xa", string(xa), "quy_tac", rule.ID, "loai_don", string(rule.LetterType),
			"loai_han", string(rule.Kind), "don_vi", string(rule.Unit), "so", rule.Amount)
		return nil, status.Errorf(codes.FailedPrecondition,
			"quy tắc hạn đơn thư của xã cho %s / %s không dùng được: %s — sửa quy tắc trên màn hình cấu hình",
			rule.LetterType, rule.Kind, err.Error())
	}

	week, readYear, err := s.readCalendar(ctx, xa, "ResolveCitizenLetterDeadline")
	if err != nil {
		return nil, err
	}
	due, err := domain.CitizenLetterDueAt(from.AsTime(), rule.Amount, rule.Unit, week, readYear)
	if err != nil {
		return nil, s.loiLich(ctx, err, xa, "ResolveCitizenLetterDeadline/tinh")
	}

	return &identityv1.ResolveCitizenLetterDeadlineResponse{
		Outcome: &identityv1.ResolveCitizenLetterDeadlineResponse_Deadline{
			Deadline: &identityv1.CitizenLetterDeadline{DueAt: timestamppb.New(due)},
		},
	}, nil
}

// citizenLetterTypeFrom maps the wire enum onto the stored value. UNSPECIFIED and any value outside
// the four are the caller's fault — a guessed type applies another statutory procedure's numbers.
func citizenLetterTypeFrom(t identityv1.CitizenLetterType) (domain.CitizenLetterType, error) {
	switch t {
	case identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KIEN_NGHI_PHAN_ANH:
		return domain.CitizenLetterKienNghiPhanAnh, nil
	case identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KHIEU_NAI:
		return domain.CitizenLetterKhieuNai, nil
	case identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_TO_CAO:
		return domain.CitizenLetterToCao, nil
	case identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_DE_NGHI:
		return domain.CitizenLetterDeNghi, nil
	case identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_UNSPECIFIED:
		return "", status.Error(codes.InvalidArgument, "thiếu letter_type — không có loại đơn mặc định")
	}
	return "", status.Errorf(codes.InvalidArgument, "letter_type %d không nằm trong bốn loại đơn", int32(t))
}

// citizenLetterKindFrom maps the wire enum onto the stored value. UNSPECIFIED is the caller's fault.
func citizenLetterKindFrom(k identityv1.CitizenLetterDeadlineKind) (domain.CitizenLetterDeadlineKind, error) {
	switch k {
	case identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_PROCESSING:
		return domain.CitizenLetterProcessing, nil
	case identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_RESOLUTION:
		return domain.CitizenLetterResolution, nil
	case identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_UNSPECIFIED:
		return "", status.Error(codes.InvalidArgument, "thiếu kind — mỗi lời gọi phải nói rõ hạn xử lý đơn hay hạn giải quyết")
	}
	return "", status.Errorf(codes.InvalidArgument, "kind %d không phải một loại hạn hợp lệ", int32(k))
}
