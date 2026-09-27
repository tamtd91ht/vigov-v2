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
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	ra, err := d.cl.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: host})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound, codes.InvalidArgument:
			return tenant.Tenant{}, false, nil
		}
		return tenant.Tenant{}, false, fmt.Errorf("platformclient: ResolveHost: %w", err)
	}

	t := ra.GetTenant()
	if t == nil {
		return tenant.Tenant{}, false, fmt.Errorf("%w: ResolveHost thành công mà không có xã", ErrNenTangTraSai)
	}
	id := tenant.ID(t.GetId())
	if !id.Valid() {
		// Same check as ByHost: an id that is not a ULID means some end is using a meaningful
		// identifier (rule 1, invariant 2). Refused, never passed through.
		return tenant.Tenant{}, false, fmt.Errorf("%w: ResolveHost trả về mã xã không phải ULID", ErrNenTangTraSai)
	}
	return tenant.Tenant{
		ID:       id,
		Host:     t.GetHost(),
		Name:     t.GetDisplayName(),
		Active:   t.GetActive(),
		Province: t.GetProvince(),
	}, true, nil
}
