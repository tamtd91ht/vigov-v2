package petitionsclient

// What these tests defend on THIS side of the wire: an outage is never "holds nothing", a blank id
// and a missing commune are refused before the wire, the commune and the caller key travel in
// metadata, and the RPC is not on the tenant-exemption list.
//
// A REAL gRPC server over bufconn with the REAL interceptor chain, because half of what is under
// test — caller key, commune — lives in that chain.

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

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// fakeCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var fakeCallerKey = secret.Secret("caller-key-FAKE-NOT-A-REAL-KEY")

var communeA = tenant.ID("01JA" + strings.Repeat("A", 22))

type fakeServer struct {
	petitionsv1.UnimplementedPetitionsServiceServer

	resp     *petitionsv1.CountOrgUnitHoldingsResponse
	err      error
	calls    int
	sawTen   []string
	sawKey   []string
	sawReqID string
}

func (s *fakeServer) CountOrgUnitHoldings(ctx context.Context, in *petitionsv1.CountOrgUnitHoldingsRequest) (
	*petitionsv1.CountOrgUnitHoldingsResponse, error) {
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

func startClient(t *testing.T, srv petitionsv1.PetitionsServiceServer) *Client {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(fakeCallerKey, quiet),
		grpcx.UnaryServerInterceptor(),
	))
	petitionsv1.RegisterPetitionsServiceServer(gs, srv)
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
	return New(petitionsv1.NewPetitionsServiceClient(conn), quiet)
}

func communeCtx() context.Context { return tenant.Into(context.Background(), communeA) }

// Exempt, the count would be "rows of this unit in some commune" — and an id of another commune
// would stop counting zero.
func TestCountOrgUnitHoldingsIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(petitionsv1.PetitionsService_CountOrgUnitHoldings_FullMethodName) {
		t.Fatal("CountOrgUnitHoldings nằm trong danh sách miễn xã")
	}
}

func TestOrgUnitHoldingsReturnsCountsAndCarriesTenantAndKey(t *testing.T) {
	srv := &fakeServer{resp: &petitionsv1.CountOrgUnitHoldingsResponse{OpenPetitions: 3, OpenTasks: 2}}
	c := startClient(t, srv)

	h, err := c.OrgUnitHoldings(communeCtx(), "bp-dia-chinh")
	if err != nil {
		t.Fatalf("OrgUnitHoldings: %v", err)
	}
	if h.OpenPetitions != 3 || h.OpenTasks != 2 || !h.Any() {
		t.Errorf("holdings = %+v, Any = %v; muốn 3 phản ánh, 2 nhiệm vụ, Any true", h, h.Any())
	}
	if srv.sawReqID != "bp-dia-chinh" {
		t.Errorf("org_unit_id trên dây = %q", srv.sawReqID)
	}
	if len(srv.sawTen) != 1 || srv.sawTen[0] != string(communeA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng %q", srv.sawTen, string(communeA))
	}
	if len(srv.sawKey) != 1 {
		t.Errorf("khoá bên gọi trên dây = %d giá trị, muốn 1", len(srv.sawKey))
	}
}

// Zero and zero is the ordinary "may delete" answer.
func TestOrgUnitHoldingsZeroIsNotAny(t *testing.T) {
	c := startClient(t, &fakeServer{resp: &petitionsv1.CountOrgUnitHoldingsResponse{}})
	h, err := c.OrgUnitHoldings(communeCtx(), "bp-trong")
	if err != nil {
		t.Fatalf("OrgUnitHoldings: %v", err)
	}
	if h.Any() {
		t.Errorf("không giữ gì mà Any = true: %+v", h)
	}
}

// Each kind alone must block: a sum that forgot one kind lets a delete through with records left.
func TestAnyCountsEveryKind(t *testing.T) {
	for _, h := range []OrgUnitHoldings{{OpenPetitions: 1}, {OpenTasks: 1}} {
		if !h.Any() {
			t.Errorf("%+v: Any = false — một loại hồ sơ không chặn được việc xoá", h)
		}
	}
}

func TestOrgUnitHoldingsUnavailableIsRetryableErrorNeverZero(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		c := startClient(t, &fakeServer{err: status.Error(code, "petitions đang xuống")})
		_, err := c.OrgUnitHoldings(communeCtx(), "bp-a")
		if err == nil {
			t.Fatalf("%v mà không báo lỗi — việc xoá sẽ đi qua khi không ai kiểm", code)
		}
		if !errors.Is(err, ErrPetitionsUnavailable) {
			t.Errorf("%v không mang ErrPetitionsUnavailable: %v", code, err)
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
		if errors.Is(err, ErrPetitionsUnavailable) {
			t.Errorf("lỗi không tạm thời %v bị đánh dấu thử lại được", e)
		}
	}
}

func TestOrgUnitHoldingsBlankIDRefusedLocally(t *testing.T) {
	srv := &fakeServer{resp: &petitionsv1.CountOrgUnitHoldingsResponse{}}
	c := startClient(t, srv)
	if _, err := c.OrgUnitHoldings(communeCtx(), ""); err == nil {
		t.Fatal("mã bộ phận rỗng mà không báo lỗi")
	}
	if srv.calls != 0 {
		t.Errorf("đã gửi %d yêu cầu chỉ có thể thất bại", srv.calls)
	}
}

func TestOrgUnitHoldingsWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeServer{resp: &petitionsv1.CountOrgUnitHoldingsResponse{}}
	c := startClient(t, srv)
	if _, err := c.OrgUnitHoldings(context.Background(), "bp-a"); err == nil {
		t.Fatal("gọi được CountOrgUnitHoldings mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}
