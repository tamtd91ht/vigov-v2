package identityclient

// What these tests defend: no path out of MeasureWorkingSeconds reads as "0 seconds late". An
// unconfigured calendar is its own sentinel, an outage is the retryable one, and a missing or unasked
// span is a contract fault.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// fakeMeasureServer is identity answering MeasureWorkingHours with a fixed figure per span (it does
// no arithmetic of its own), or an error.
type fakeMeasureServer struct {
	identityv1.UnimplementedIdentityServiceServer

	answer    func(*identityv1.MeasureWorkingHoursRequest) *identityv1.MeasureWorkingHoursResponse
	err       error
	calls     int
	sawTenant []string
	saw       *identityv1.MeasureWorkingHoursRequest
}

func (s *fakeMeasureServer) MeasureWorkingHours(ctx context.Context, in *identityv1.MeasureWorkingHoursRequest) (
	*identityv1.MeasureWorkingHoursResponse, error) {
	s.calls++
	s.saw = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.answer(in), nil
}

// echo answers every span asked with 3600 seconds.
func echo(in *identityv1.MeasureWorkingHoursRequest) *identityv1.MeasureWorkingHoursResponse {
	out := &identityv1.MeasureWorkingHoursResponse{}
	for _, sp := range in.GetSpans() {
		out.Items = append(out.Items, &identityv1.WorkingTimeMeasured{Span: sp, WorkingSeconds: 3600})
	}
	return out
}

var (
	spanA = WorkingSpan{Start: time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 22, 1, 0, 0, 0, time.UTC)}
	spanB = WorkingSpan{Start: time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 23, 1, 0, 0, 0, time.UTC)}
)

func TestMeasureWorkingSecondsMapsBySpanCollapsesAndCarriesTenant(t *testing.T) {
	srv := &fakeMeasureServer{answer: echo}
	c := moMay(t, srv)

	// The same instant in another zone is the same span.
	ict := time.FixedZone("ICT", 7*3600)
	dupA := WorkingSpan{Start: spanA.Start.In(ict), End: spanA.End.In(ict)}
	got, err := c.MeasureWorkingSeconds(ngucCanh(), []WorkingSpan{spanA, spanB, dupA})
	if err != nil {
		t.Fatalf("MeasureWorkingSeconds: %v", err)
	}
	if len(srv.saw.GetSpans()) != 2 {
		t.Errorf("gửi %d khoảng, muốn 2 — trùng phải gộp", len(srv.saw.GetSpans()))
	}
	if got[spanA] != 3600 || got[spanB] != 3600 || len(got) != 2 {
		t.Errorf("kết quả = %v", got)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestMeasureWorkingSecondsErrorsAreNeverZero(t *testing.T) {
	short := func(in *identityv1.MeasureWorkingHoursRequest) *identityv1.MeasureWorkingHoursResponse {
		r := echo(in)
		r.Items = r.Items[:len(r.Items)-1]
		return r
	}
	unasked := func(in *identityv1.MeasureWorkingHoursRequest) *identityv1.MeasureWorkingHoursResponse {
		r := echo(in)
		r.Items[0].Span = &identityv1.WorkingTimeSpan{Start: in.GetSpans()[0].GetEnd(), End: in.GetSpans()[0].GetEnd()}
		return r
	}
	unset := func(in *identityv1.MeasureWorkingHoursRequest) *identityv1.MeasureWorkingHoursResponse {
		r := echo(in)
		r.Items[0].Span = nil
		return r
	}
	cases := []struct {
		name     string
		srv      *fakeMeasureServer
		sentinel error
	}{
		{"chưa cấu hình lịch", &fakeMeasureServer{err: status.Error(codes.FailedPrecondition, "lịch trống")}, ErrWorkingCalendarNotConfigured},
		{"identity sập", &fakeMeasureServer{err: status.Error(codes.Unavailable, "sập")}, ErrIdentityUnavailable},
		{"trả thiếu khoảng", &fakeMeasureServer{answer: short}, nil},
		{"trả khoảng không được hỏi", &fakeMeasureServer{answer: unasked}, nil},
		{"trả khoảng rỗng", &fakeMeasureServer{answer: unset}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := moMay(t, tc.srv).MeasureWorkingSeconds(ngucCanh(), []WorkingSpan{spanA, spanB})
			if err == nil {
				t.Fatalf("không có lỗi, trả %v", got)
			}
			if got != nil {
				t.Errorf("trả %v kèm lỗi", got)
			}
			for _, s := range []error{ErrWorkingCalendarNotConfigured, ErrIdentityUnavailable} {
				if errors.Is(err, s) != (s == tc.sentinel) {
					t.Errorf("errors.Is(err, %v) = %v — sai sentinel: %v", s, errors.Is(err, s), err)
				}
			}
		})
	}
}

func TestMeasureWorkingSecondsRefusedLocally(t *testing.T) {
	tooMany := make([]WorkingSpan, MaxMeasuredSpansPerCall+1)
	for i := range tooMany {
		tooMany[i] = spanA
	}
	cases := map[string][]WorkingSpan{
		"không có khoảng": nil,
		"quá 500 khoảng":  tooMany,
		"mốc rỗng":        {{Start: time.Time{}, End: spanA.End}},
		"end trước start": {{Start: spanA.End, End: spanA.Start}},
	}
	for name, spans := range cases {
		t.Run(name, func(t *testing.T) {
			srv := &fakeMeasureServer{answer: echo}
			if _, err := moMay(t, srv).MeasureWorkingSeconds(ngucCanh(), spans); err == nil {
				t.Fatal("không có lỗi")
			}
			if srv.calls != 0 {
				t.Errorf("đã gọi máy chủ %d lần cho một yêu cầu chỉ có thể hỏng", srv.calls)
			}
		})
	}
}

func TestMeasureWorkingSecondsWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeMeasureServer{answer: echo}
	if _, err := moMay(t, srv).MeasureWorkingSeconds(context.Background(), []WorkingSpan{spanA}); err == nil {
		t.Fatal("gọi được MeasureWorkingHours mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}
