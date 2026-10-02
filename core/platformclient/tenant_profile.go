package platformclient

import (
	"context"
	"fmt"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// TenantProfile is a commune's display profile (ADR 0045, decision 5; platform.proto TenantProfile).
// "" in any field means "not declared" — the consumer renders nothing and never substitutes a default.
//
// THE WHOLE CONTRACT IS CARRIED, AND EACH CONSUMER DECIDES WHAT IT PUBLISHES. In particular LogoURL is a
// link to an image served ELSEWHERE, not an object in ViGov's public bucket (ADR 0052 §2): a public
// route that forwards it makes every reader fetch whatever host a commune typed. It is deprecated by
// ADR 0069; a consumer that wants the logo reads LogoPublicURL and never falls back to LogoURL.
type TenantProfile struct {
	OfficeAddress   string
	LogoURL         string // DEPRECATED (ADR 0069): a typed URL — never publish it, never fall back to it
	Hotline         string // the commune's official line — public-service information (#16)
	OfficeHoursText string // DISPLAY TEXT ONLY; never parsed into hours (ADR 0007)
	Introduction    string

	// LogoPublicURL and WebAdminBannerPublicURL are ViGov-issued public-bucket URLs (ADR 0069 #4, #5;
	// ADR 0052 §2), "" when not set. Each new upload is a new URL, so a consumer must not cache them
	// past the read. The banner is web-admin's only — it is NOT the Mini App banner (ADR 0069 #6).
	LogoPublicURL           string
	WebAdminBannerPublicURL string
}

// TenantProfile reads the profile of THE COMMUNE ALREADY IN ctx. The commune is not a parameter
// (rule 1, invariant 4): grpcx.UnaryClientInterceptor writes it into "x-tenant-id" from the same
// context value, and the RPC has no field that could name another one.
//
// ok=false with err=nil: the commune has not declared a profile (or it was soft-deleted).
// err != nil: no commune in ctx, or the call did not happen — never "not declared", so the caller can
// answer 503 rather than an empty profile that reads as "this commune published nothing".
func (d *Directory) TenantProfile(ctx context.Context) (TenantProfile, bool, error) {
	if _, ok := tenant.From(ctx); !ok {
		return TenantProfile{}, false, fmt.Errorf("platformclient: TenantProfile: %w", tenant.ErrNoTenant)
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	res, err := d.cl.GetTenantProfile(ctx, &platformv1.GetTenantProfileRequest{})
	if err != nil {
		return TenantProfile{}, false, fmt.Errorf("platformclient: GetTenantProfile: %w", err)
	}
	p := res.GetProfile()
	if p == nil {
		return TenantProfile{}, false, nil
	}
	return TenantProfile{
		OfficeAddress:   p.GetOfficeAddress(),
		LogoURL:         p.GetLogoUrl(),
		Hotline:         p.GetHotline(),
		OfficeHoursText: p.GetOfficeHoursText(),
		Introduction:    p.GetIntroduction(),

		LogoPublicURL:           p.GetLogoPublicUrl(),
		WebAdminBannerPublicURL: p.GetWebAdminBannerPublicUrl(),
	}, true, nil
}
