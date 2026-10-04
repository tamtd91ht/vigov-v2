package grpc_test

import (
	"io"
	"log/slog"
	"testing"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
)

// GetTenantProfile's two image URLs (ADR 0069): base URL + the public key the store returned, "" when the
// store returned none (unpublished, deleted or unset — the store's join decides), never logo_url.

type urlsFake struct{}

func (urlsFake) PublicURL(key string) string { return "https://media.example/" + key }

func profileServer(hs domain.HoSoHienThi, urls svcgrpc.PublicURLs) *svcgrpc.Server {
	return svcgrpc.NewServer(svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(),
		HoSo:     hoSoGia{theoXa: map[tenant.ID]domain.HoSoHienThi{xaTanPhu: hs}},
		Policies: samplePolicies(), Fields: sampleFields(), MapFrames: sampleFrames(), URLs: urls},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGetTenantProfileBuildsImageURLsFromPublicKeys(t *testing.T) {
	s := profileServer(domain.HoSoHienThi{
		LogoPublicKey:           "public-media/t_x/2026/10/platform/tenant-logo/01j/thumb-512.png",
		WebAdminBannerPublicKey: "public-media/t_x/2026/10/platform/tenant-banner/01k/thumb-1600.jpg",
	}, urlsFake{})
	res, err := s.GetTenantProfile(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetTenantProfileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	p := res.GetProfile()
	if p == nil {
		t.Fatal("a profile holding only branding must still be returned")
	}
	if p.GetLogoPublicUrl() != "https://media.example/public-media/t_x/2026/10/platform/tenant-logo/01j/thumb-512.png" ||
		p.GetWebAdminBannerPublicUrl() != "https://media.example/public-media/t_x/2026/10/platform/tenant-banner/01k/thumb-1600.jpg" {
		t.Errorf("urls = %q / %q", p.GetLogoPublicUrl(), p.GetWebAdminBannerPublicUrl())
	}
}

func TestGetTenantProfileNeverFallsBackToTheTypedLogoURL(t *testing.T) {
	for name, urls := range map[string]svcgrpc.PublicURLs{"storage configured": urlsFake{}, "no URL builder": nil} {
		t.Run(name, func(t *testing.T) {
			s := profileServer(domain.HoSoHienThi{LogoURL: "https://somewhere.example/typed.png"}, urls)
			res, err := s.GetTenantProfile(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetTenantProfileRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if got := res.GetProfile().GetLogoPublicUrl(); got != "" {
				t.Errorf("logo_public_url = %q, want empty — no published file", got)
			}
			if got := res.GetProfile().GetWebAdminBannerPublicUrl(); got != "" {
				t.Errorf("web_admin_banner_public_url = %q, want empty", got)
			}
		})
	}
}
