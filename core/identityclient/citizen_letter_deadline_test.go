package identityclient

// What these tests defend: the three outcomes of ResolveCitizenLetterDeadline stay apart. A deadline
// is a non-nil instant; "not configured" is (nil, false, nil); every failure — an outage, an unusable
// rule, a malformed answer — is an error, never (nil, false, nil), because that pair books a letter
// with no deadline and counts it on time.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

var letterCountFrom = time.Date(2026, 9, 20, 17, 0, 0, 0, time.UTC) // 00:00 on 21/09 in Vietnam

const (
	letterComplaint  = identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KHIEU_NAI
	letterResolution = identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_RESOLUTION
)

// fakeLetterDeadlineServer is identity answering ResolveCitizenLetterDeadline, and nothing else.
type fakeLetterDeadlineServer struct {
	identityv1.UnimplementedIdentityServiceServer

	res       *identityv1.ResolveCitizenLetterDeadlineResponse
	err       error
	calls     int
	sawTenant []string
	saw       *identityv1.ResolveCitizenLetterDeadlineRequest
}

func (s *fakeLetterDeadlineServer) ResolveCitizenLetterDeadline(ctx context.Context,
	in *identityv1.ResolveCitizenLetterDeadlineRequest) (*identityv1.ResolveCitizenLetterDeadlineResponse, error) {
	s.calls++
	s.saw = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.res, nil
}

func deadlineAnswer(ts *timestamppb.Timestamp) *identityv1.ResolveCitizenLetterDeadlineResponse {
	return &identityv1.ResolveCitizenLetterDeadlineResponse{
		Outcome: &identityv1.ResolveCitizenLetterDeadlineResponse_Deadline{
			Deadline: &identityv1.CitizenLetterDeadline{DueAt: ts}}}
}

func TestResolveCitizenLetterDeadlineReturnsInstantAndCarriesTenant(t *testing.T) {
	want := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	srv := &fakeLetterDeadlineServer{res: deadlineAnswer(timestamppb.New(want))}
	c := moMay(t, srv)

	due, configured, err := c.ResolveCitizenLetterDeadline(ngucCanh(), letterComplaint, letterResolution, letterCountFrom)
	if err != nil || !configured || due == nil || !due.Equal(want) {
		t.Fatalf("= %v, %v, %v — muốn %s, true, nil", due, configured, err, want)
	}
	assertTenantOnWire(t, srv.sawTenant)
	if srv.saw.GetLetterType() != letterComplaint || srv.saw.GetKind() != letterResolution ||
		!srv.saw.GetCountFrom().AsTime().Equal(letterCountFrom) {
		t.Errorf("yêu cầu trên dây = %v", srv.saw)
	}
}

func TestResolveCitizenLetterDeadlineNotConfiguredIsNotAnError(t *testing.T) {
	srv := &fakeLetterDeadlineServer{res: &identityv1.ResolveCitizenLetterDeadlineResponse{
		Outcome: &identityv1.ResolveCitizenLetterDeadlineResponse_NotConfigured{
			NotConfigured: &identityv1.CitizenLetterDeadlineNotConfigured{}}}}
	due, configured, err := moMay(t, srv).ResolveCitizenLetterDeadline(ngucCanh(), letterComplaint, letterResolution, letterCountFrom)
	if err != nil || configured || due != nil {
		t.Errorf("= %v, %v, %v — muốn nil, false, nil", due, configured, err)
	}
}

// FAILED_PRECONDITION IS THE UNUSABLE-RULE SENTINEL — an error, never "not configured".
func TestResolveCitizenLetterDeadlineFailedPreconditionIsUnusable(t *testing.T) {
	srv := &fakeLetterDeadlineServer{err: status.Error(codes.FailedPrecondition, "khoá đơn vị")}
	due, configured, err := moMay(t, srv).ResolveCitizenLetterDeadline(ngucCanh(), letterComplaint, letterResolution, letterCountFrom)
	if err == nil || configured || due != nil {
		t.Fatalf("= %v, %v, %v — muốn lỗi", due, configured, err)
	}
	if !errors.Is(err, ErrCitizenLetterDeadlineUnusable) || errors.Is(err, ErrIdentityUnavailable) {
		t.Errorf("sentinel sai: %v", err)
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("mã = %v, muốn giữ FailedPrecondition", status.Code(err))
	}
}

func TestResolveCitizenLetterDeadlineUnavailableIsRetryable(t *testing.T) {
	srv := &fakeLetterDeadlineServer{err: status.Error(codes.Unavailable, "identity đang sập")}
	_, configured, err := moMay(t, srv).ResolveCitizenLetterDeadline(ngucCanh(), letterComplaint, letterResolution, letterCountFrom)
	if !errors.Is(err, ErrIdentityUnavailable) || errors.Is(err, ErrCitizenLetterDeadlineUnusable) || configured {
		t.Errorf("= %v, %v", configured, err)
	}
}

// An answer with neither outcome, or a deadline without an instant, is a contract fault — an error.
func TestResolveCitizenLetterDeadlineMalformedAnswersAreErrors(t *testing.T) {
	for name, res := range map[string]*identityv1.ResolveCitizenLetterDeadlineResponse{
		"không có outcome":      {},
		"deadline không due_at": deadlineAnswer(nil),
		"due_at không hợp lệ":   deadlineAnswer(&timestamppb.Timestamp{Seconds: 1, Nanos: -1}),
	} {
		t.Run(name, func(t *testing.T) {
			due, configured, err := moMay(t, &fakeLetterDeadlineServer{res: res}).
				ResolveCitizenLetterDeadline(ngucCanh(), letterComplaint, letterResolution, letterCountFrom)
			if err == nil || configured || due != nil {
				t.Errorf("= %v, %v, %v — muốn lỗi hợp đồng", due, configured, err)
			}
		})
	}
}

func TestResolveCitizenLetterDeadlineRefusedLocally(t *testing.T) {
	cases := []struct {
		name string
		lt   identityv1.CitizenLetterType
		k    identityv1.CitizenLetterDeadlineKind
		from time.Time
	}{
		{"không có loại đơn", identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_UNSPECIFIED, letterResolution, letterCountFrom},
		{"không có loại hạn", letterComplaint, identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_UNSPECIFIED, letterCountFrom},
		{"gốc đếm rỗng", letterComplaint, letterResolution, time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := &fakeLetterDeadlineServer{}
			if _, _, err := moMay(t, srv).ResolveCitizenLetterDeadline(ngucCanh(), c.lt, c.k, c.from); err == nil {
				t.Fatal("không có lỗi")
			}
			if srv.calls != 0 {
				t.Errorf("đã gọi máy chủ %d lần", srv.calls)
			}
		})
	}
}

func TestResolveCitizenLetterDeadlineWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeLetterDeadlineServer{}
	if _, _, err := moMay(t, srv).ResolveCitizenLetterDeadline(context.Background(), letterComplaint, letterResolution, letterCountFrom); err == nil {
		t.Fatal("gọi được mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới máy chủ %d lần dù không mang xã", srv.calls)
	}
}
