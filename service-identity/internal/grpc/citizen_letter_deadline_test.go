package grpc

// What these tests defend: the status codes of ResolveCitizenLetterDeadline, because each one tells
// documents something different — INVALID_ARGUMENT (fix the caller), OK not_configured (book with no
// deadline), FAILED_PRECONDITION (a person must fix the commune's configuration; do not book), Internal
// (an outage; do not book). The one confusion that does silent harm is a broken rule answered as
// not_configured: the letter is then booked with no deadline and counted ON TIME (ADR 0084 #4).
// The arithmetic itself is defended in internal/domain.

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type letterRulesFake struct {
	rule  domain.CitizenLetterDeadlineRule
	found bool
	err   error

	calls   int
	sawType domain.CitizenLetterType
	sawKind domain.CitizenLetterDeadlineKind
}

func (f *letterRulesFake) Live(_ context.Context, t domain.CitizenLetterType, k domain.CitizenLetterDeadlineKind) (
	domain.CitizenLetterDeadlineRule, bool, error) {
	f.calls++
	f.sawType, f.sawKind = t, k
	return f.rule, f.found, f.err
}

func letterRule(t domain.CitizenLetterType, k domain.CitizenLetterDeadlineKind, amount int,
	u domain.CitizenLetterDeadlineUnit) *letterRulesFake {
	return &letterRulesFake{found: true, rule: domain.CitizenLetterDeadlineRule{
		ID: "01JRULE0000000000000000000", LetterType: t, Kind: k, Amount: amount, Unit: u}}
}

func letterReq(t *testing.T, lt identityv1.CitizenLetterType, k identityv1.CitizenLetterDeadlineKind,
	from string) *identityv1.ResolveCitizenLetterDeadlineRequest {
	t.Helper()
	return &identityv1.ResolveCitizenLetterDeadlineRequest{
		LetterType: lt, Kind: k, CountFrom: timestamppb.New(mocVN(t, from))}
}

const (
	complaint  = identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KHIEU_NAI
	denounce   = identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_TO_CAO
	processing = identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_PROCESSING
	resolution = identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_RESOLUTION
)

// CALLER FAULTS ARE INVALID_ARGUMENT AND READ NOTHING.
func TestCitizenLetterDeadlineCallerFaultsAreInvalidArgument(t *testing.T) {
	cases := map[string]*identityv1.ResolveCitizenLetterDeadlineRequest{
		"letter_type unspecified": {Kind: processing, CountFrom: timestamppb.Now()},
		"letter_type unknown":     {LetterType: 99, Kind: processing, CountFrom: timestamppb.Now()},
		"kind unspecified":        {LetterType: complaint, CountFrom: timestamppb.Now()},
		"kind unknown":            {LetterType: complaint, Kind: 7, CountFrom: timestamppb.Now()},
		"count_from missing":      {LetterType: complaint, Kind: processing},
		"count_from invalid": {LetterType: complaint, Kind: processing,
			CountFrom: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			rules := letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, domain.UnitCalendarDays)
			week := &lichGia{cas: tuanGia()}
			s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = rules; d.Lich = week })
			_, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã = %v (%v), muốn InvalidArgument", status.Code(err), err)
			}
			if rules.calls != 0 || week.soLanGoi != 0 {
				t.Errorf("đã đọc kho (quy tắc %d, lịch %d) cho một lời gọi sai hình dạng", rules.calls, week.soLanGoi)
			}
		})
	}
}

// NO RULE IS OK + not_configured — and the calendar is not even read.
func TestCitizenLetterDeadlineNoRuleIsNotConfigured(t *testing.T) {
	week := &lichGia{cas: tuanGia()}
	s, _ := may(t, func(d *Deps) { d.Lich = week }) // default: no rule
	res, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, complaint, processing, "2026-09-21 00:00"))
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if res.GetNotConfigured() == nil || res.GetDeadline() != nil {
		t.Fatalf("trả lời = %v, muốn not_configured", res)
	}
	if week.soLanGoi != 0 {
		t.Errorf("đọc lịch %d lần cho một loại đơn chưa cấu hình", week.soLanGoi)
	}
}

// A USABLE RULE IS A DEADLINE, the request's type and kind reaching the store as stored values.
func TestCitizenLetterDeadlineComputesFromTheRule(t *testing.T) {
	cases := []struct {
		name  string
		rules *letterRulesFake
		lt    identityv1.CitizenLetterType
		k     identityv1.CitizenLetterDeadlineKind
		from  string
		want  string
	}{
		// 30 calendar days from Monday 21/09 is Wednesday 21/10, due at the commune's 17:00.
		{"khiếu nại giải quyết 30 ngày lịch", letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterResolution, 30, domain.UnitCalendarDays),
			complaint, resolution, "2026-09-21 14:10", "2026-10-21 17:00"},
		// 7 working days from Monday 21/09: Tue…Fri (4), Mon…Wed (7) = Wednesday 30/09.
		{"tố cáo xử lý đơn 7 ngày làm việc", letterRule(domain.CitizenLetterToCao, domain.CitizenLetterProcessing, 7, domain.UnitWorkingDays),
			denounce, processing, "2026-09-21 00:00", "2026-09-30 17:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = c.rules })
			res, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, c.lt, c.k, c.from))
			if err != nil {
				t.Fatalf("lỗi: %v", err)
			}
			got := res.GetDeadline().GetDueAt()
			if got == nil || !got.AsTime().Equal(mocVN(t, c.want)) {
				t.Errorf("due_at = %v, muốn %s", got, c.want)
			}
			if c.rules.sawType != c.rules.rule.LetterType || c.rules.sawKind != c.rules.rule.Kind {
				t.Errorf("kho được hỏi %s/%s", c.rules.sawType, c.rules.sawKind)
			}
		})
	}
}

// A RULE THAT EXISTS BUT IS UNUSABLE IS FAILED_PRECONDITION — NEVER not_configured.
func TestCitizenLetterDeadlineUnusableRuleIsFailedPrecondition(t *testing.T) {
	cases := map[string]*letterRulesFake{
		"khiếu nại đếm ngày làm việc (khoá đơn vị)": letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, domain.UnitWorkingDays),
		"giờ làm việc":   letterRule(domain.CitizenLetterToCao, domain.CitizenLetterProcessing, 16, domain.UnitWorkingHours),
		"số không dương": letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 0, domain.UnitCalendarDays),
		"vượt trần":      letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 400, domain.UnitCalendarDays),
		"đơn vị lạ":      letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, "ngay"),
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = rules })
			res, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, complaint, processing, "2026-09-21 00:00"))
			if status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("mã = %v, trả lời %v — muốn FailedPrecondition", status.Code(err), res)
			}
		})
	}
}

// A CALENDAR THE COUNT CANNOT WALK IS FAILED_PRECONDITION, for calendar days too.
func TestCitizenLetterDeadlineBrokenCalendarIsFailedPrecondition(t *testing.T) {
	rules := letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, domain.UnitCalendarDays)
	s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = rules; d.Lich = &lichGia{} }) // empty week
	_, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, complaint, processing, "2026-09-21 00:00"))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v (%v), muốn FailedPrecondition", status.Code(err), err)
	}
}

// AN OUTAGE IS INTERNAL — never an answer.
func TestCitizenLetterDeadlineStoreFailureIsInternal(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = &letterRulesFake{err: errors.New("db down")} })
	_, err := s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, complaint, processing, "2026-09-21 00:00"))
	if status.Code(err) != codes.Internal {
		t.Errorf("quy tắc: mã = %v, muốn Internal", status.Code(err))
	}

	rules := letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, domain.UnitCalendarDays)
	s, _ = may(t, func(d *Deps) { d.CitizenLetterRules = rules; d.NghiLe = &nghiLeGia{err: errors.New("db down")} })
	_, err = s.ResolveCitizenLetterDeadline(ctxXa(xaA), letterReq(t, complaint, processing, "2026-09-21 00:00"))
	if status.Code(err) != codes.Internal {
		t.Errorf("lịch: mã = %v, muốn Internal", status.Code(err))
	}
}

// No commune in context is this deployment's wiring fault — Internal, not a panic.
func TestCitizenLetterDeadlineWithoutCommuneIsInternal(t *testing.T) {
	rules := letterRule(domain.CitizenLetterKhieuNai, domain.CitizenLetterProcessing, 10, domain.UnitCalendarDays)
	s, _ := may(t, func(d *Deps) { d.CitizenLetterRules = rules })
	_, err := s.ResolveCitizenLetterDeadline(context.Background(), letterReq(t, complaint, processing, "2026-09-21 00:00"))
	if status.Code(err) != codes.Internal || rules.calls != 0 {
		t.Errorf("mã = %v, đọc quy tắc %d lần", status.Code(err), rules.calls)
	}
}
