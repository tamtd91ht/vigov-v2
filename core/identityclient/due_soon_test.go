package identityclient

// What these tests defend: no path out of DueSoonCutoff reads as "nothing is due soon". A commune
// with no configuration is its own sentinel, an outage is the retryable one, and a missing or
// backwards cutoff is a contract fault — never a zero time.Time.

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

var asOfFake = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)

// fakeDueSoonServer is identity answering ResolveDueSoonCutoff, and nothing else.
type fakeDueSoonServer struct {
	identityv1.UnimplementedIdentityServiceServer

	until     *timestamppb.Timestamp
	err       error
	calls     int
	sawTenant []string
	saw       *identityv1.ResolveDueSoonCutoffRequest
}

func (s *fakeDueSoonServer) ResolveDueSoonCutoff(ctx context.Context, in *identityv1.ResolveDueSoonCutoffRequest) (
	*identityv1.ResolveDueSoonCutoffResponse, error) {
	s.calls++
	s.saw = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveDueSoonCutoffResponse{DueSoonUntil: s.until}, nil
}

func TestDueSoonCutoffReturnsInstantAndCarriesTenant(t *testing.T) {
	// A fixed instant from the fake server — this test does no hour arithmetic of its own.
	want := time.Date(2026, 9, 30, 0, 30, 0, 0, time.UTC)
	srv := &fakeDueSoonServer{until: timestamppb.New(want)}
	c := moMay(t, srv)

	got, err := c.DueSoonCutoff(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake)
	if err != nil {
		t.Fatalf("DueSoonCutoff: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("cutoff = %s, muốn %s", got, want)
	}
	assertTenantOnWire(t, srv.sawTenant)
	if !srv.saw.GetAsOf().AsTime().Equal(asOfFake) {
		t.Errorf("as_of trên dây = %s, muốn đúng `now` của bên gọi %s", srv.saw.GetAsOf().AsTime(), asOfFake)
	}
	if srv.saw.GetLinhVuc() != "" {
		t.Errorf("linh_vuc trên dây = %q, muốn rỗng — bên bọc không được tự điền", srv.saw.GetLinhVuc())
	}
}

// Cutoff equal to as_of is legal (the contract says "at or after").
func TestDueSoonCutoffEqualToAsOfIsAccepted(t *testing.T) {
	c := moMay(t, &fakeDueSoonServer{until: timestamppb.New(asOfFake)})
	if _, err := c.DueSoonCutoff(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake); err != nil {
		t.Fatalf("cutoff == as_of bị từ chối: %v", err)
	}
}

// XÃ CHƯA CẤU HÌNH LÀ SENTINEL RIÊNG, KHÔNG PHẢI MỘT CON SỐ VÀ KHÔNG PHẢI "không có gì sắp đến hạn".
func TestDueSoonCutoffFailedPreconditionIsNotConfiguredSentinel(t *testing.T) {
	c := moMay(t, &fakeDueSoonServer{err: status.Error(codes.FailedPrecondition, "xã chưa cấu hình sla")})

	got, err := c.DueSoonCutoff(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake)
	if err == nil {
		t.Fatalf("không có lỗi, trả %s — xã chưa cấu hình phải làm HỎNG bộ lọc", got)
	}
	if !got.IsZero() {
		t.Errorf("trả %s kèm lỗi", got)
	}
	if !errors.Is(err, ErrDueSoonNotConfigured) {
		t.Errorf("không mang ErrDueSoonNotConfigured: %v", err)
	}
	if errors.Is(err, ErrIdentityUnavailable) {
		t.Error("chưa cấu hình bị đánh dấu thử lại được — chờ không sửa được cấu hình")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("mã = %v, muốn giữ nguyên FailedPrecondition", status.Code(err))
	}
}

func TestDueSoonCutoffUnavailableIsRetryableSentinel(t *testing.T) {
	c := moMay(t, &fakeDueSoonServer{err: status.Error(codes.Unavailable, "identity đang sập")})
	_, err := c.DueSoonCutoff(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake)
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("không mang ErrIdentityUnavailable: %v", err)
	}
	if errors.Is(err, ErrDueSoonNotConfigured) {
		t.Error("sự cố hạ tầng bị báo là chưa cấu hình")
	}
}

// OK with no instant, or an instant before as_of, is a contract fault — never a zero cutoff that
// empties the filter into "all clear".
func TestDueSoonCutoffMissingOrBackwardsIsContractFault(t *testing.T) {
	for name, until := range map[string]*timestamppb.Timestamp{
		"thiếu mốc":       nil,
		"mốc trước as_of": timestamppb.New(asOfFake.Add(-time.Minute)),
	} {
		t.Run(name, func(t *testing.T) {
			c := moMay(t, &fakeDueSoonServer{until: until})
			got, err := c.DueSoonCutoff(ngucCanh(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake)
			if err == nil {
				t.Fatalf("không có lỗi, trả %s", got)
			}
			if errors.Is(err, ErrIdentityUnavailable) || errors.Is(err, ErrDueSoonNotConfigured) {
				t.Errorf("lỗi hợp đồng mang nhầm sentinel: %v", err)
			}
		})
	}
}

func TestDueSoonCutoffRefusedLocally(t *testing.T) {
	cases := []struct {
		name string
		kind identityv1.WorkKind
		asOf time.Time
	}{
		{"không có loại việc", identityv1.WorkKind_WORK_KIND_UNSPECIFIED, asOfFake},
		{"as_of rỗng", identityv1.WorkKind_WORK_KIND_NHIEM_VU, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &fakeDueSoonServer{}
			c := moMay(t, srv)
			if _, err := c.DueSoonCutoff(ngucCanh(), tc.kind, "", tc.asOf); err == nil {
				t.Fatal("không có lỗi")
			}
			if srv.calls != 0 {
				t.Errorf("đã gọi máy chủ %d lần cho một yêu cầu chỉ có thể hỏng", srv.calls)
			}
		})
	}
}

func TestDueSoonCutoffWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeDueSoonServer{}
	c := moMay(t, srv)
	_, err := c.DueSoonCutoff(context.Background(), identityv1.WorkKind_WORK_KIND_NHIEM_VU, "", asOfFake)
	if err == nil {
		t.Fatal("gọi được ResolveDueSoonCutoff mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}
