package platformclient

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

type mapFrameFake struct {
	platformv1.PlatformServiceClient
	res         *platformv1.GetMapFrameDefaultResponse
	err         error
	calls       int
	hadDeadline bool
}

func (f *mapFrameFake) GetMapFrameDefault(ctx context.Context, _ *platformv1.GetMapFrameDefaultRequest,
	_ ...grpc.CallOption) (*platformv1.GetMapFrameDefaultResponse, error) {
	f.calls++
	_, f.hadDeadline = ctx.Deadline()
	return f.res, f.err
}

func frameRes(lat, lng, r float64) *platformv1.GetMapFrameDefaultResponse {
	return &platformv1.GetMapFrameDefaultResponse{Frame: &platformv1.MapFrameDefault{
		CenterLat: lat, CenterLng: lng, RadiusKm: r}}
}

func TestMapFrameDefaultMapsEveryField(t *testing.T) {
	at := time.Date(2026, 10, 4, 1, 2, 3, 0, time.UTC)
	res := frameRes(16.047079, 108.20623, 12.5)
	res.Frame.UpdatedAt = timestamppb.New(at)
	f := &mapFrameFake{res: res}
	got, ok, err := NewDirectory(f, nil).MapFrameDefault(tenant.Into(context.Background(), ulidThu))
	if err != nil || !ok || got.CenterLat != 16.047079 || got.CenterLng != 108.20623 ||
		got.RadiusKm != 12.5 || !got.UpdatedAt.Equal(at) {
		t.Fatalf("khung = %+v ok=%v err=%v", got, ok, err)
	}
	if !f.hadDeadline {
		t.Fatal("lời gọi không mang hạn chờ — một nền tảng treo sẽ treo luôn trang bản đồ")
	}
}

func TestMapFrameDefaultNotConfiguredIsOkFalseNotError(t *testing.T) {
	d := NewDirectory(&mapFrameFake{res: &platformv1.GetMapFrameDefaultResponse{}}, nil)
	_, ok, err := d.MapFrameDefault(tenant.Into(context.Background(), ulidThu))
	if err != nil || ok {
		t.Fatalf("chưa đặt khung: ok=%v err=%v, muốn ok=false err=nil", ok, err)
	}
}

// A frame outside the contract's bounds is "not configured" — never clamped, never an error.
func TestMapFrameDefaultOutOfBoundsIsNotConfigured(t *testing.T) {
	cases := map[string]*platformv1.GetMapFrameDefaultResponse{
		"vĩ độ dưới khung":   frameRes(8.3, 106, 10),
		"vĩ độ trên khung":   frameRes(23.5, 106, 10),
		"kinh độ dưới khung": frameRes(16, 102.0, 10),
		"kinh độ trên khung": frameRes(16, 109.6, 10),
		"bán kính 0":         frameRes(16, 106, 0),
		"bán kính âm":        frameRes(16, 106, -1),
		"bán kính quá 50":    frameRes(16, 106, 50.1),
		"vĩ độ NaN":          frameRes(math.NaN(), 106, 10),
		"bán kính vô cực":    frameRes(16, 106, math.Inf(1)),
		"toạ độ 0,0":         frameRes(0, 0, 10),
	}
	for name, res := range cases {
		got, ok, err := NewDirectory(&mapFrameFake{res: res}, nil).
			MapFrameDefault(tenant.Into(context.Background(), ulidThu))
		if err != nil || ok || got != (MapFrameDefault{}) {
			t.Errorf("%s: khung=%+v ok=%v err=%v, muốn rỗng ok=false err=nil", name, got, ok, err)
		}
	}
}

func TestMapFrameDefaultBoundsAreInclusive(t *testing.T) {
	for _, res := range []*platformv1.GetMapFrameDefaultResponse{
		frameRes(8.4, 102.1, 50), frameRes(23.4, 109.5, 0.1),
	} {
		_, ok, err := NewDirectory(&mapFrameFake{res: res}, nil).
			MapFrameDefault(tenant.Into(context.Background(), ulidThu))
		if err != nil || !ok {
			t.Errorf("khung ở mép %+v: ok=%v err=%v, muốn ok=true", res.Frame, ok, err)
		}
	}
}

func TestMapFrameDefaultOutageIsErrorNotNotConfigured(t *testing.T) {
	d := NewDirectory(&mapFrameFake{err: status.Error(codes.Unavailable, "down")}, nil)
	_, ok, err := d.MapFrameDefault(tenant.Into(context.Background(), ulidThu))
	if err == nil || ok || status.Code(errors.Unwrap(err)) != codes.Unavailable {
		t.Fatalf("mất mạng tới nền tảng: ok=%v err=%v", ok, err)
	}
}

func TestMapFrameDefaultWithoutCommuneRefusesBeforeCalling(t *testing.T) {
	f := &mapFrameFake{res: frameRes(16, 106, 10)}
	_, _, err := NewDirectory(f, nil).MapFrameDefault(context.Background())
	if !errors.Is(err, tenant.ErrNoTenant) || f.calls != 0 {
		t.Fatalf("không có xã: err=%v, số lần gọi=%d — muốn ErrNoTenant, 0 lần", err, f.calls)
	}
}
