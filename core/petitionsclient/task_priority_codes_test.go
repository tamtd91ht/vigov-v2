package petitionsclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// TaskPriorityCodes over the real interceptor chain (startClient, petitionsclient_test.go).

type prioritiesServer struct {
	petitionsv1.UnimplementedPetitionsServiceServer

	items  []*petitionsv1.TaskPriorityCode
	err    error
	calls  int
	sawReq []string
	sawTen []string
}

func (s *prioritiesServer) ResolveTaskPriorityCodes(ctx context.Context, in *petitionsv1.ResolveTaskPriorityCodesRequest) (
	*petitionsv1.ResolveTaskPriorityCodesResponse, error) {
	s.calls++
	s.sawReq = in.GetCodes()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTen = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &petitionsv1.ResolveTaskPriorityCodesResponse{Items: s.items}, nil
}

func TestResolveTaskPriorityCodesIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(petitionsv1.PetitionsService_ResolveTaskPriorityCodes_FullMethodName) {
		t.Fatal("ResolveTaskPriorityCodes nằm trong danh sách miễn xã")
	}
}

func TestTaskPriorityCodesAnswersAndCarriesTenant(t *testing.T) {
	srv := &prioritiesServer{items: []*petitionsv1.TaskPriorityCode{
		{Code: "khan", Active: true}, {Code: "cao", Active: false},
	}}
	got, err := startClient(t, srv).TaskPriorityCodes(communeCtx(), []string{"khan", "cao", "khong-co", "khan"})
	if err != nil {
		t.Fatal(err)
	}
	if active, ok := got["khan"]; !ok || !active {
		t.Errorf("khan = %v/%v, want present and active", active, ok)
	}
	if active, ok := got["cao"]; !ok || active {
		t.Errorf("cao = %v/%v, want present and inactive", active, ok)
	}
	if _, ok := got["khong-co"]; ok {
		t.Error("unanswered code reads as present")
	}
	if len(srv.sawReq) != 3 {
		t.Errorf("sent %v — duplicates must be sent once", srv.sawReq)
	}
	if len(srv.sawTen) != 1 || srv.sawTen[0] != string(communeA) {
		t.Errorf("x-tenant-id = %v", srv.sawTen)
	}
}

func TestTaskPriorityCodesRefusedBeforeWire(t *testing.T) {
	over := make([]string, MaxTaskPriorityCodesPerCall+1)
	for i := range over {
		over[i] = "khan"
	}
	cases := map[string][]string{"blank": {"khan", "  "}, "empty string": {""}, "over ceiling": over}
	for name, in := range cases {
		srv := &prioritiesServer{}
		if _, err := startClient(t, srv).TaskPriorityCodes(communeCtx(), in); err == nil {
			t.Errorf("%s: no error", name)
		}
		if srv.calls != 0 {
			t.Errorf("%s: sent a request that can only fail", name)
		}
	}
}

func TestTaskPriorityCodesEmptyNoRoundTrip(t *testing.T) {
	srv := &prioritiesServer{}
	got, err := startClient(t, srv).TaskPriorityCodes(communeCtx(), nil)
	if err != nil || len(got) != 0 || srv.calls != 0 {
		t.Errorf("got=%v err=%v calls=%d", got, err, srv.calls)
	}
}

func TestTaskPriorityCodesWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &prioritiesServer{}
	if _, err := startClient(t, srv).TaskPriorityCodes(context.Background(), []string{"khan"}); err == nil {
		t.Fatal("called without a commune")
	}
	if srv.calls != 0 {
		t.Errorf("reached the server %d times without a commune", srv.calls)
	}
}

func TestTaskPriorityCodesErrorsNeverAnAnswer(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		got, err := startClient(t, &prioritiesServer{err: status.Error(code, "down")}).TaskPriorityCodes(communeCtx(), []string{"khan"})
		if err == nil || got != nil || !errors.Is(err, ErrPetitionsUnavailable) {
			t.Errorf("%v: got=%v err=%v — want ErrPetitionsUnavailable and no map", code, got, err)
		}
	}
	got, err := startClient(t, &prioritiesServer{err: status.Error(codes.InvalidArgument, "x")}).TaskPriorityCodes(communeCtx(), []string{"khan"})
	if err == nil || got != nil || errors.Is(err, ErrPetitionsUnavailable) {
		t.Errorf("InvalidArgument: got=%v err=%v — want a non-retryable error", got, err)
	}
}

func TestTaskPriorityCodesRefusesMalformedAnswer(t *testing.T) {
	cases := map[string][]*petitionsv1.TaskPriorityCode{
		"code not asked": {{Code: "rat-khan", Active: true}},
		"two states":     {{Code: "khan", Active: true}, {Code: "khan", Active: false}},
	}
	for name, items := range cases {
		got, err := startClient(t, &prioritiesServer{items: items}).TaskPriorityCodes(communeCtx(), []string{"khan"})
		if err == nil || got != nil {
			t.Errorf("%s: got=%v err=%v — want refused", name, got, err)
		}
	}
}
