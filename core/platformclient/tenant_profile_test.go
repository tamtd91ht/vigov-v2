package platformclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

type profileFake struct {
	platformv1.PlatformServiceClient
	res   *platformv1.GetTenantProfileResponse
	err   error
	calls int
}

func (f *profileFake) GetTenantProfile(context.Context, *platformv1.GetTenantProfileRequest,
	...grpc.CallOption) (*platformv1.GetTenantProfileResponse, error) {
	f.calls++
	return f.res, f.err
}

func TestTenantProfileMapsEveryField(t *testing.T) {
	d := NewDirectory(&profileFake{res: &platformv1.GetTenantProfileResponse{Profile: &platformv1.TenantProfile{
		OfficeAddress: "Số 1", LogoUrl: "https://x/logo.png", Hotline: "0900000000",
		OfficeHoursText: "Sáng 7h30", Introduction: "Giới thiệu",
		LogoPublicUrl:           "https://public.example/t_x/logo.png",
		WebAdminBannerPublicUrl: "https://public.example/t_x/banner.jpg"}}}, nil)
	p, ok, err := d.TenantProfile(tenant.Into(context.Background(), ulidThu))
	if err != nil || !ok || p.OfficeAddress != "Số 1" || p.LogoURL != "https://x/logo.png" ||
		p.Hotline != "0900000000" || p.OfficeHoursText != "Sáng 7h30" || p.Introduction != "Giới thiệu" ||
		p.LogoPublicURL != "https://public.example/t_x/logo.png" ||
		p.WebAdminBannerPublicURL != "https://public.example/t_x/banner.jpg" {
		t.Fatalf("hồ sơ = %+v ok=%v err=%v", p, ok, err)
	}
}

func TestTenantProfileNotDeclaredIsOkFalseNotError(t *testing.T) {
	d := NewDirectory(&profileFake{res: &platformv1.GetTenantProfileResponse{}}, nil)
	_, ok, err := d.TenantProfile(tenant.Into(context.Background(), ulidThu))
	if err != nil || ok {
		t.Fatalf("chưa khai hồ sơ: ok=%v err=%v, muốn ok=false err=nil", ok, err)
	}
}

func TestTenantProfileOutageIsErrorNotNotDeclared(t *testing.T) {
	d := NewDirectory(&profileFake{err: status.Error(codes.Unavailable, "down")}, nil)
	_, ok, err := d.TenantProfile(tenant.Into(context.Background(), ulidThu))
	if err == nil || ok || status.Code(errors.Unwrap(err)) != codes.Unavailable {
		t.Fatalf("mất mạng tới nền tảng: ok=%v err=%v", ok, err)
	}
}

func TestTenantProfileWithoutCommuneRefusesBeforeCalling(t *testing.T) {
	f := &profileFake{res: &platformv1.GetTenantProfileResponse{}}
	_, _, err := NewDirectory(f, nil).TenantProfile(context.Background())
	if !errors.Is(err, tenant.ErrNoTenant) || f.calls != 0 {
		t.Fatalf("không có xã: err=%v, số lần gọi=%d — muốn ErrNoTenant, 0 lần", err, f.calls)
	}
}
