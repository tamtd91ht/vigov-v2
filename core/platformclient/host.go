package platformclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// XaTheoHost is ByHost WITH THE THIRD OUTCOME ByHost cannot express: the platform could not be
// asked.
//
// WHY A SECOND METHOD AND NOT A CHANGE TO ByHost: ByHost implements tenant.Directory, whose bool is
// right for the Host EDGE — there, an outage and an unknown Host both mean "serve nothing", and the
// edge answers 404 either way. The Mini App's commune confirmation ("Làm việc với xã X?",
// service-identity GET /api/v1/communes?host=) is not an edge: it is a question a citizen asks
// about a domain printed on a QR, and "the platform is down" read as "no such commune" tells the
// citizen the QR is wrong. The caller must answer 503 there, never an empty result that looks
// like a real negative, and never a remembered commune (rule 1, forbidden #1).
//
// ok=false with err=nil is ONE answer for three cases: no commune holds this Host, the Host is
// reserved for the platform (service-platform ResolveHost answers those with the same NotFound, on
// purpose), or the Host is not a host at all. err != nil means the call did not happen, or the
// answer broke the contract — never "unknown".
//
// An INACTIVE commune is returned with Active=false, exactly as ByHost does; the caller decides
// what inactive means for its case.
//
// It does not log: the caller logs once, in its own voice, and the Host is caller input.
func (d *Directory) XaTheoHost(ctx context.Context, host string) (tenant.Tenant, bool, error) {
	c, ok, err := d.CommuneByHost(ctx, host)
	return c.Tenant, ok, err
}

// Commune is a ResolveHost answer: the shared tenant.Tenant plus the commune's launch code
// (platform.proto Tenant.launch_code).
//
// A WRAPPER, NOT A FIELD ON tenant.Tenant: that type rides every service's request context and the
// tenant.HostResolver cache; the launch code has one reader, the public `?host=` lookup the QR tool asks
// (service-identity GET /api/v1/communes). LaunchCode is public and grants nothing — never route,
// key or authorise on it. "" = the platform did not send one (an older platform service).
type Commune struct {
	tenant.Tenant
	LaunchCode string
}

// CommuneByHost is XaTheoHost with the launch code kept. Same three outcomes, same error mapping —
// XaTheoHost is this minus the code, so the two can never answer the same Host differently.
func (d *Directory) CommuneByHost(ctx context.Context, host string) (Commune, bool, error) {
	return d.resolveWithOutage(ctx, &platformv1.ResolveHostRequest{Host: host})
}

// CommuneByLaunchCode is XaTheoHost keyed by a commune's launch code: the opaque public code printed
// into its QR link (`?x=`, owner decision 2026-10-12). Same three outcomes, same error mapping — the
// platform answers an unknown code with the same NotFound as an unknown Host, and a malformed one with
// InvalidArgument, both folded into ok=false here.
//
// tenant.Tenant.Host is the commune's PRIMARY host, or "" when that primary is a reserved platform host
// (service-platform blanks it, see the ResolveHost comment in proto/vigov/platform/v1/platform.proto).
// The caller must treat "" as "no usable commune" and never pick another of the commune's hosts.
//
// THE CODE IS A LOOKUP KEY, NOT A COMMUNE REFERENCE: it is printed publicly and grants nothing (rule 1,
// forbidden #2). What it yields is the same registry answer a Host yields.
func (d *Directory) CommuneByLaunchCode(ctx context.Context, code string) (tenant.Tenant, bool, error) {
	c, ok, err := d.resolveWithOutage(ctx, &platformv1.ResolveHostRequest{LaunchCode: code})
	return c.Tenant, ok, err
}

// resolveWithOutage is the body both lookups share, so the two can never map an outage differently.
func (d *Directory) resolveWithOutage(ctx context.Context, req *platformv1.ResolveHostRequest) (Commune, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	ra, err := d.cl.ResolveHost(ctx, req)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound, codes.InvalidArgument:
			return Commune{}, false, nil
		}
		return Commune{}, false, fmt.Errorf("platformclient: ResolveHost: %w", err)
	}

	t := ra.GetTenant()
	if t == nil {
		return Commune{}, false, fmt.Errorf("%w: ResolveHost thành công mà không có xã", ErrNenTangTraSai)
	}
	id := tenant.ID(t.GetId())
	if !id.Valid() {
		// Same check as ByHost: an id that is not a ULID means some end is using a meaningful
		// identifier (rule 1, invariant 2). Refused, never passed through.
		return Commune{}, false, fmt.Errorf("%w: ResolveHost trả về mã xã không phải ULID", ErrNenTangTraSai)
	}
	return Commune{
		Tenant: tenant.Tenant{
			ID:       id,
			Host:     t.GetHost(),
			Name:     t.GetDisplayName(),
			Active:   t.GetActive(),
			Province: t.GetProvince(),
		},
		LaunchCode: t.GetLaunchCode(),
	}, true, nil
}
