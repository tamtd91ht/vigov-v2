package documentsclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// DocumentTypeCodes over the real interceptor chain (startClient, documentsclient_test.go).

type typesServer struct {
	documentsv1.UnimplementedDocumentsServiceServer

	items  []*documentsv1.DocumentTypeCode
	err    error
	calls  int
	sawReq []string
	sawTen []string
}

func (s *typesServer) ResolveDocumentTypeCodes(ctx context.Context, in *documentsv1.ResolveDocumentTypeCodesRequest) (
	*documentsv1.ResolveDocumentTypeCodesResponse, error) {
	s.calls++
	s.sawReq = in.GetCodes()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTen = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &documentsv1.ResolveDocumentTypeCodesResponse{Items: s.items}, nil
}

func TestResolveDocumentTypeCodesIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(documentsv1.DocumentsService_ResolveDocumentTypeCodes_FullMethodName) {
		t.Fatal("ResolveDocumentTypeCodes nằm trong danh sách miễn xã")
	}
}

func TestDocumentTypeCodesAnswersAndCarriesTenant(t *testing.T) {
	srv := &typesServer{items: []*documentsv1.DocumentTypeCode{
		{Code: "cong-van", Active: true}, {Code: "to-trinh", Active: false},
	}}
	got, err := startClient(t, srv).DocumentTypeCodes(communeCtx(), []string{"cong-van", "to-trinh", "khong-co", "cong-van"})
	if err != nil {
		t.Fatal(err)
	}
	if active, ok := got["cong-van"]; !ok || !active {
		t.Errorf("cong-van = %v/%v, want present and active", active, ok)
	}
	if active, ok := got["to-trinh"]; !ok || active {
		t.Errorf("to-trinh = %v/%v, want present and inactive", active, ok)
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

func TestDocumentTypeCodesRefusedBeforeWire(t *testing.T) {
	over := make([]string, MaxDocumentTypeCodesPerCall+1)
	for i := range over {
		over[i] = "cong-van"
	}
	cases := map[string][]string{"blank": {"cong-van", "  "}, "empty string": {""}, "over ceiling": over}
	for name, in := range cases {
		srv := &typesServer{}
		if _, err := startClient(t, srv).DocumentTypeCodes(communeCtx(), in); err == nil {
			t.Errorf("%s: no error", name)
		}
		if srv.calls != 0 {
			t.Errorf("%s: sent a request that can only fail", name)
		}
	}
}

func TestDocumentTypeCodesEmptyNoRoundTrip(t *testing.T) {
	srv := &typesServer{}
	got, err := startClient(t, srv).DocumentTypeCodes(communeCtx(), nil)
	if err != nil || len(got) != 0 || srv.calls != 0 {
		t.Errorf("got=%v err=%v calls=%d", got, err, srv.calls)
	}
}

func TestDocumentTypeCodesWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &typesServer{}
	if _, err := startClient(t, srv).DocumentTypeCodes(context.Background(), []string{"cong-van"}); err == nil {
		t.Fatal("called without a commune")
	}
	if srv.calls != 0 {
		t.Errorf("reached the server %d times without a commune", srv.calls)
	}
}

func TestDocumentTypeCodesErrorsNeverAnAnswer(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		got, err := startClient(t, &typesServer{err: status.Error(code, "down")}).DocumentTypeCodes(communeCtx(), []string{"cong-van"})
		if err == nil || got != nil || !errors.Is(err, ErrDocumentsUnavailable) {
			t.Errorf("%v: got=%v err=%v — want ErrDocumentsUnavailable and no map", code, got, err)
		}
	}
	got, err := startClient(t, &typesServer{err: status.Error(codes.InvalidArgument, "x")}).DocumentTypeCodes(communeCtx(), []string{"cong-van"})
	if err == nil || got != nil || errors.Is(err, ErrDocumentsUnavailable) {
		t.Errorf("InvalidArgument: got=%v err=%v — want a non-retryable error", got, err)
	}
}

func TestDocumentTypeCodesRefusesMalformedAnswer(t *testing.T) {
	cases := map[string][]*documentsv1.DocumentTypeCode{
		"code not asked": {{Code: "quyet-dinh", Active: true}},
		"two states":     {{Code: "cong-van", Active: true}, {Code: "cong-van", Active: false}},
	}
	for name, items := range cases {
		got, err := startClient(t, &typesServer{items: items}).DocumentTypeCodes(communeCtx(), []string{"cong-van"})
		if err == nil || got != nil {
			t.Errorf("%s: got=%v err=%v — want refused", name, got, err)
		}
	}
}
