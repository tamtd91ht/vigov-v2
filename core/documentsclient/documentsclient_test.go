package documentsclient

// Same defences as core/petitionsclient's tests: an outage is never "holds nothing", a blank id and
// a missing commune are refused before the wire, commune and caller key travel in metadata, and the
// RPC is not tenant-exempt. Real server, real interceptor chain, over bufconn.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// fakeCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var fakeCallerKey = secret.Secret("caller-key-FAKE-NOT-A-REAL-KEY")

var communeA = tenant.ID("01JA" + strings.Repeat("A", 22))

type fakeServer struct {
	documentsv1.UnimplementedDocumentsServiceServer

	resp     *documentsv1.CountOrgUnitHoldingsResponse
	err      error
	calls    int
	sawTen   []string
	sawKey   []string
	sawReqID string
}

func (s *fakeServer) CountOrgUnitHoldings(ctx context.Context, in *documentsv1.CountOrgUnitHoldingsRequest) (
	*documentsv1.CountOrgUnitHoldingsResponse, error) {
	s.calls++
	s.sawReqID = in.GetOrgUnitId()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTen = md.Get(grpcx.MetadataTenantKey)
		s.sawKey = md.Get(grpcx.MetadataCallerKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func startClient(t *testing.T, srv documentsv1.DocumentsServiceServer) *Client {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(fakeCallerKey, quiet),
		grpcx.UnaryServerInterceptor(),
	))
	documentsv1.RegisterDocumentsServiceServer(gs, srv)
	go func() {
		if err := gs.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithChainUnaryInterceptor(
			grpcx.UnaryClientCallerAuth(fakeCallerKey),
			grpcx.UnaryClientInterceptor(),
		))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return New(documentsv1.NewDocumentsServiceClient(conn), quiet)
}

func communeCtx() context.Context { return tenant.Into(context.Background(), communeA) }

func TestCountOrgUnitHoldingsIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(documentsv1.DocumentsService_CountOrgUnitHoldings_FullMethodName) {
		t.Fatal("CountOrgUnitHoldings nằm trong danh sách miễn xã")
	}
}

func TestOrgUnitHoldingsReturnsCountAndCarriesTenantAndKey(t *testing.T) {
	srv := &fakeServer{resp: &documentsv1.CountOrgUnitHoldingsResponse{OpenIncomingDocuments: 4}}
	c := startClient(t, srv)

	h, err := c.OrgUnitHoldings(communeCtx(), "bp-van-phong")
	if err != nil {
		t.Fatalf("OrgUnitHoldings: %v", err)
	}
	if h.OpenIncomingDocuments != 4 || !h.Any() {
		t.Errorf("holdings = %+v, Any = %v; muốn 4 văn bản đến, Any true", h, h.Any())
	}
	if srv.sawReqID != "bp-van-phong" {
		t.Errorf("org_unit_id trên dây = %q", srv.sawReqID)
	}
	if len(srv.sawTen) != 1 || srv.sawTen[0] != string(communeA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng %q", srv.sawTen, string(communeA))
	}
	if len(srv.sawKey) != 1 {
		t.Errorf("khoá bên gọi trên dây = %d giá trị, muốn 1", len(srv.sawKey))
	}
}

func TestOrgUnitHoldingsZeroIsNotAny(t *testing.T) {
	c := startClient(t, &fakeServer{resp: &documentsv1.CountOrgUnitHoldingsResponse{}})
	h, err := c.OrgUnitHoldings(communeCtx(), "bp-trong")
	if err != nil {
		t.Fatalf("OrgUnitHoldings: %v", err)
	}
	if h.Any() {
		t.Errorf("không giữ gì mà Any = true: %+v", h)
	}
}

func TestOrgUnitHoldingsUnavailableIsRetryableErrorNeverZero(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		c := startClient(t, &fakeServer{err: status.Error(code, "documents đang xuống")})
		_, err := c.OrgUnitHoldings(communeCtx(), "bp-a")
		if err == nil {
			t.Fatalf("%v mà không báo lỗi — việc xoá sẽ đi qua khi không ai kiểm", code)
		}
		if !errors.Is(err, ErrDocumentsUnavailable) {
			t.Errorf("%v không mang ErrDocumentsUnavailable: %v", code, err)
		}
		if status.Code(err) != code {
			t.Errorf("mã = %v, muốn giữ %v", status.Code(err), code)
		}
	}
}

func TestOrgUnitHoldingsOtherErrorsAreNotRetryable(t *testing.T) {
	for _, e := range []error{status.Error(codes.InvalidArgument, "rỗng"), errors.New("giả lập: hỏng")} {
		c := startClient(t, &fakeServer{err: e})
		_, err := c.OrgUnitHoldings(communeCtx(), "bp-a")
		if err == nil {
			t.Fatalf("lỗi %v mà không báo lỗi", e)
		}
		if errors.Is(err, ErrDocumentsUnavailable) {
			t.Errorf("lỗi không tạm thời %v bị đánh dấu thử lại được", e)
		}
	}
}

func TestOrgUnitHoldingsBlankIDRefusedLocally(t *testing.T) {
	srv := &fakeServer{resp: &documentsv1.CountOrgUnitHoldingsResponse{}}
	c := startClient(t, srv)
	if _, err := c.OrgUnitHoldings(communeCtx(), ""); err == nil {
		t.Fatal("mã bộ phận rỗng mà không báo lỗi")
	}
	if srv.calls != 0 {
		t.Errorf("đã gửi %d yêu cầu chỉ có thể thất bại", srv.calls)
	}
}

func TestOrgUnitHoldingsWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeServer{resp: &documentsv1.CountOrgUnitHoldingsResponse{}}
	c := startClient(t, srv)
	if _, err := c.OrgUnitHoldings(context.Background(), "bp-a"); err == nil {
		t.Fatal("gọi được CountOrgUnitHoldings mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}
