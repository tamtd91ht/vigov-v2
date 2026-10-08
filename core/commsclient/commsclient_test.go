package commsclient

// What these tests defend: an outage is its own retryable sentinel and never "delivered"; a page the
// contract refuses is refused before the wire; commune and caller key travel in metadata; a reply
// that cannot be counted is refused. Real server, real interceptor chain, over bufconn.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// fakeCallerKey is fake key material — the text says so (rule 8, forbidden #1).
var fakeCallerKey = secret.Secret("caller-key-FAKE-NOT-A-REAL-KEY")

var communeA = tenant.ID("01JA" + strings.Repeat("A", 22))

type fakeServer struct {
	commsv1.UnimplementedCommsServiceServer

	err       error
	dropOne   bool
	calls     int
	sawTenant []string
	sawKey    []string
	saw       *commsv1.DeliverStaffNotificationsRequest
}

func (s *fakeServer) DeliverStaffNotifications(ctx context.Context, in *commsv1.DeliverStaffNotificationsRequest) (
	*commsv1.DeliverStaffNotificationsResponse, error) {
	s.calls++
	s.saw = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
		s.sawKey = md.Get(grpcx.MetadataCallerKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	out := &commsv1.DeliverStaffNotificationsResponse{}
	for i, n := range in.GetNotifications() {
		if s.dropOne && i == 0 {
			continue
		}
		out.Items = append(out.Items, &commsv1.StaffNotificationDelivery{
			IdempotencyKey: n.GetIdempotencyKey(), Created: uint32(len(n.GetRecipientMa())),
		})
	}
	return out, nil
}

func startClient(t *testing.T, srv commsv1.CommsServiceServer) *Client {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(fakeCallerKey, quiet),
		grpcx.UnaryServerInterceptor(),
	))
	commsv1.RegisterCommsServiceServer(gs, srv)
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
	return New(commsv1.NewCommsServiceClient(conn), quiet)
}

func communeCtx() context.Context { return tenant.Into(context.Background(), communeA) }

func notice(key string, recipients ...string) Notice {
	return Notice{IdempotencyKey: key, Kind: commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_OVERDUE,
		RecipientMa: recipients, Title: "Nhiệm vụ NV01 đã quá hạn xử lý", Link: "/nhiem-vu?q=NV01"}
}

func TestDeliverIsNotTenantExempt(t *testing.T) {
	if grpcx.ExemptFromTenant(commsv1.CommsService_DeliverStaffNotifications_FullMethodName) {
		t.Fatal("DeliverStaffNotifications nằm trong danh sách miễn xã")
	}
}

func TestDeliverCarriesTenantKeyAndMapsByKey(t *testing.T) {
	srv := &fakeServer{}
	c := startClient(t, srv)
	got, err := c.DeliverStaffNotifications(communeCtx(), []Notice{notice("k1", "CB-1", "CB-2"), notice("k2", "CB-3")})
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if got["k1"].Created != 2 || got["k2"].Created != 1 {
		t.Errorf("kết quả = %+v", got)
	}
	if len(srv.sawTenant) != 1 || srv.sawTenant[0] != string(communeA) {
		t.Errorf("x-tenant-id trên dây = %v", srv.sawTenant)
	}
	if len(srv.sawKey) != 1 {
		t.Errorf("khoá bên gọi trên dây = %d giá trị, muốn 1", len(srv.sawKey))
	}
	if n := srv.saw.GetNotifications()[0]; n.GetLink() != "/nhiem-vu?q=NV01" || n.GetKind() !=
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_OVERDUE {
		t.Errorf("thông báo trên dây bị đổi: %v", n.GetLink())
	}
}

func TestDeliverRefusesBeforeTheWire(t *testing.T) {
	many := make([]Notice, MaxNoticesPerCall+1)
	for i := range many {
		many[i] = notice(strings.Repeat("k", i+1), "CB-1")
	}
	wide := notice("k", make([]string, MaxRecipientsPerCall+1)...)
	for name, page := range map[string][]Notice{
		"rỗng":           nil,
		"quá 100":        many,
		"quá 1000 người": {wide},
		"trùng khoá":     {notice("k", "CB-1"), notice("k", "CB-2")},
	} {
		srv := &fakeServer{}
		c := startClient(t, srv)
		if _, err := c.DeliverStaffNotifications(communeCtx(), page); err == nil {
			t.Errorf("%s: không bị từ chối", name)
		}
		if srv.calls != 0 {
			t.Errorf("%s: đã gửi %d yêu cầu chỉ có thể thất bại", name, srv.calls)
		}
	}
}

func TestDeliverUnavailableIsRetryable(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		c := startClient(t, &fakeServer{err: status.Error(code, "comms đang xuống")})
		_, err := c.DeliverStaffNotifications(communeCtx(), []Notice{notice("k", "CB-1")})
		if !errors.Is(err, ErrCommsUnavailable) || status.Code(err) != code {
			t.Errorf("%v: err = %v", code, err)
		}
	}
	c := startClient(t, &fakeServer{err: status.Error(codes.InvalidArgument, "sai")})
	if _, err := c.DeliverStaffNotifications(communeCtx(), []Notice{notice("k", "CB-1")}); err == nil ||
		errors.Is(err, ErrCommsUnavailable) {
		t.Errorf("INVALID_ARGUMENT phải là lỗi không thử lại: %v", err)
	}
}

// A reply missing an item cannot be counted; reading it as "0 created" would under-report the run.
func TestDeliverShortReplyIsContractFault(t *testing.T) {
	c := startClient(t, &fakeServer{dropOne: true})
	if _, err := c.DeliverStaffNotifications(communeCtx(), []Notice{notice("k1", "CB-1"), notice("k2", "CB-2")}); err == nil {
		t.Fatal("trả thiếu mục mà không báo lỗi")
	}
}

func TestDeliverWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeServer{}
	c := startClient(t, srv)
	if _, err := c.DeliverStaffNotifications(context.Background(), []Notice{notice("k", "CB-1")}); err == nil {
		t.Fatal("gửi được mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}

func TestDialEmptyAddressRefusedByName(t *testing.T) {
	c, err := Dial("", fakeCallerKey, nil)
	if err == nil || c != nil || !strings.Contains(err.Error(), "COMMS_GRPC_ADDR") {
		t.Fatalf("Dial(\"\") = %v, %v — muốn từ chối, nêu tên COMMS_GRPC_ADDR", c, err)
	}
	c, err = Dial("comms.invalid:9090", fakeCallerKey, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := New(nil, nil).Close(); err != nil {
		t.Errorf("Close trên client dựng bằng New: %v", err)
	}
}

// Items reach the wire one to one, in order, with the stored instant unchanged; a notice without
// items sends none — the request an older producer sent.
func TestDeliverMapsDueSoonItems(t *testing.T) {
	srv := &fakeServer{}
	c := startClient(t, srv)
	d1 := time.Date(2026, 10, 8, 3, 30, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 9, 10, 0, 0, 123000000, time.UTC)
	withItems := notice("k1", "CB-1")
	withItems.Kind = commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON
	withItems.DueSoonItems = []DueSoonItem{{Code: "NV-FAKE-02", Deadline: d1}, {Code: "NV-FAKE-01", Deadline: d2}}
	if _, err := c.DeliverStaffNotifications(communeCtx(), []Notice{withItems, notice("k2", "CB-2")}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	ns := srv.saw.GetNotifications()
	got := ns[0].GetDueSoonItems()
	if len(got) != 2 || got[0].GetCode() != "NV-FAKE-02" || !got[0].GetDeadline().AsTime().Equal(d1) ||
		got[1].GetCode() != "NV-FAKE-01" || !got[1].GetDeadline().AsTime().Equal(d2) {
		t.Errorf("due_soon_items trên dây = %v", got)
	}
	if n := len(ns[1].GetDueSoonItems()); n != 0 {
		t.Errorf("thông báo không có mục lại mang %d mục", n)
	}
	if dueSoonItemsToWire(nil) != nil || dueSoonItemsToWire([]DueSoonItem{}) != nil {
		t.Error("không có mục phải ra nil")
	}
}
