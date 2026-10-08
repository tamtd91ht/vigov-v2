package documentsclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// ResolveCitizenLetterForTask over the real interceptor chain (startClient, documentsclient_test.go),
// and the contract checks on the answer.

const letterThu = "01JDONTHU0000000000000000A"

type letterServer struct {
	documentsv1.UnimplementedDocumentsServiceServer

	resp   *documentsv1.ResolveCitizenLetterForTaskResponse
	err    error
	calls  int
	sawID  string
	sawTen []string
}

func (s *letterServer) ResolveCitizenLetterForTask(ctx context.Context,
	in *documentsv1.ResolveCitizenLetterForTaskRequest) (*documentsv1.ResolveCitizenLetterForTaskResponse, error) {
	s.calls++
	s.sawID = in.GetLetterId()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTen = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func eligible(src *documentsv1.CitizenLetterTaskSource) *documentsv1.ResolveCitizenLetterForTaskResponse {
	return &documentsv1.ResolveCitizenLetterForTaskResponse{
		Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_ELIGIBLE,
		Letter:      src,
	}
}

func TestResolveCitizenLetterForTaskIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(documentsv1.DocumentsService_ResolveCitizenLetterForTask_FullMethodName) {
		t.Fatal("ResolveCitizenLetterForTask nằm trong danh sách miễn xã")
	}
}

func TestResolveCitizenLetterForTaskEligibleCarriesFactsAndTenant(t *testing.T) {
	due := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	srv := &letterServer{resp: eligible(&documentsv1.CitizenLetterTaskSource{
		LetterId: letterThu, Number: 12, Year: 2026, HoldingUnitId: "bp-dia-chinh", AssigneeCode: "CB-00012",
		CurrentDueAt: timestamppb.New(due),
	})}
	got, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), letterThu)
	if err != nil {
		t.Fatal(err)
	}
	want := CitizenLetterForTask{Eligibility: CitizenLetterEligible, LetterID: letterThu, Number: 12, Year: 2026,
		HoldingUnitID: "bp-dia-chinh", AssigneeCode: "CB-00012", CurrentDueAt: due}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if srv.sawID != letterThu {
		t.Errorf("letter_id on the wire = %q", srv.sawID)
	}
	if len(srv.sawTen) != 1 || srv.sawTen[0] != string(communeA) {
		t.Errorf("x-tenant-id = %v", srv.sawTen)
	}
}

func TestResolveCitizenLetterForTaskNoDeadlineIsZero(t *testing.T) {
	srv := &letterServer{resp: eligible(&documentsv1.CitizenLetterTaskSource{LetterId: letterThu, Number: 1, Year: 2026})}
	got, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), letterThu)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CurrentDueAt.IsZero() {
		t.Errorf("unset current_due_at read as %v — must stay zero (no deadline), never a default", got.CurrentDueAt)
	}
}

func TestResolveCitizenLetterForTaskBareAnswers(t *testing.T) {
	for name, c := range map[string]struct {
		e    documentsv1.CitizenLetterTaskEligibility
		want CitizenLetterEligibility
	}{
		"not found":    {documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_NOT_FOUND, CitizenLetterNotFound},
		"denunciation": {documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_DENUNCIATION, CitizenLetterDenunciation},
	} {
		t.Run(name, func(t *testing.T) {
			srv := &letterServer{resp: &documentsv1.ResolveCitizenLetterForTaskResponse{Eligibility: c.e}}
			got, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), letterThu)
			if err != nil {
				t.Fatal(err)
			}
			if got != (CitizenLetterForTask{Eligibility: c.want}) {
				t.Errorf("got %+v", got)
			}
		})
	}
}

// Every shape the contract rules out is an error — never one of the three answers.
func TestResolveCitizenLetterForTaskContractFaultsRefused(t *testing.T) {
	src := &documentsv1.CitizenLetterTaskSource{LetterId: letterThu, Number: 3, Year: 2026}
	for name, resp := range map[string]*documentsv1.ResolveCitizenLetterForTaskResponse{
		"unspecified":                 {},
		"unknown value":               {Eligibility: documentsv1.CitizenLetterTaskEligibility(99)},
		"eligible without facts":      eligible(nil),
		"eligible for another letter": eligible(&documentsv1.CitizenLetterTaskSource{LetterId: "01JKHAC"}),
		"denunciation with facts": {Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_DENUNCIATION,
			Letter: src},
		"not found with facts": {Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_NOT_FOUND,
			Letter: src},
		"invalid deadline": eligible(&documentsv1.CitizenLetterTaskSource{LetterId: letterThu,
			CurrentDueAt: &timestamppb.Timestamp{Seconds: 1, Nanos: -5}}),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := startClient(t, &letterServer{resp: resp}).ResolveCitizenLetterForTask(communeCtx(), letterThu)
			if !errors.Is(err, ErrCitizenLetterAnswerInvalid) {
				t.Fatalf("err = %v, want ErrCitizenLetterAnswerInvalid", err)
			}
			if got != (CitizenLetterForTask{}) {
				t.Errorf("a refused answer still carried facts: %+v", got)
			}
		})
	}
}

func TestResolveCitizenLetterForTaskOutageIsNeverAnAnswer(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		srv := &letterServer{err: status.Error(code, "down")}
		_, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), letterThu)
		if !errors.Is(err, ErrDocumentsUnavailable) {
			t.Errorf("%v: err = %v, want ErrDocumentsUnavailable", code, err)
		}
	}
	// UNIMPLEMENTED (documents older than the RPC) is an error too, but not "retry soon".
	srv := &letterServer{err: status.Error(codes.Unimplemented, "old")}
	_, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), letterThu)
	if err == nil || errors.Is(err, ErrDocumentsUnavailable) {
		t.Errorf("UNIMPLEMENTED: err = %v, want a non-retryable error", err)
	}
}

func TestResolveCitizenLetterForTaskRefusedBeforeWire(t *testing.T) {
	for name, id := range map[string]string{"blank": "  ", "empty": "", "too long": strings.Repeat("A", MaxCitizenLetterIDLen+1)} {
		srv := &letterServer{}
		if _, err := startClient(t, srv).ResolveCitizenLetterForTask(communeCtx(), id); err == nil {
			t.Errorf("%s: no error", name)
		}
		if srv.calls != 0 {
			t.Errorf("%s: reached the wire", name)
		}
	}
}

func TestResolveCitizenLetterForTaskWithoutCommuneNeverLeaves(t *testing.T) {
	srv := &letterServer{}
	if _, err := startClient(t, srv).ResolveCitizenLetterForTask(context.Background(), letterThu); err == nil {
		t.Fatal("call without a commune succeeded")
	}
	if srv.calls != 0 {
		t.Error("a call without a commune reached documents")
	}
}
