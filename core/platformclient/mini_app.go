package platformclient

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// The two platform reads the citizen-session bridge makes before it writes anything (ADR 0045
// §Luồng, steps 5-6; kb/30-indexes/transaction-boundaries.json, phat_hanh_phien_cong_dan_qua_cau_mini_app).
//
// THEY RETURN AN ERROR FOR THE THIRD OUTCOME, UNLIKE ByHost. ByHost has to fit tenant.Directory's
// bool and collapses an outage into "unknown Host". Here the caller must tell them apart: "the
// platform could not be reached" is UNAVAILABLE and nothing is issued, while "this app is not
// registered" is FAILED_PRECONDITION. Collapsing the two would either refuse every citizen during a
// platform blip with a message telling them the app is misconfigured, or — the direction that
// matters — tempt a caller to "use the commune it remembered last time", which the transaction
// boundary forbids by name.

// CheDoMiniApp is the mode of a registered Mini App (ADR 0044).
type CheDoMiniApp int

const (
	// CheDoKhongRo is what an unknown enum value maps to. The caller refuses it: a third mode is a
	// decision (ADR 0044), not something to guess the meaning of.
	CheDoKhongRo CheDoMiniApp = iota
	CheDoAppChinh
	CheDoAppRieng
)

// MiniApp is one registered Zalo Mini App, as the bridge needs it.
type MiniApp struct {
	AppID string
	CheDo CheDoMiniApp

	// XaRieng is the bound commune of a dedicated app — present ONLY when the platform answered
	// that the binding exists AND the commune is active. Empty under CheDoAppRieng means "bound
	// commune not active": the caller refuses and does not follow tenant_succession on its own.
	XaRieng tenant.ID
}

// ErrNenTangTraSai is a response that breaks the contract (an id that is not a ULID, a tenant
// on a main app). Distinct from an outage so the caller can log it as a contract alert.
var ErrNenTangTraSai = errors.New("platformclient: dịch vụ nền tảng trả lời sai hợp đồng")

// MiniApp resolves one app id. ok=false means NOT REGISTERED (the caller refuses; there is no
// default app). err != nil means the call did not happen or the answer broke the contract — never
// "not registered".
//
// Runs with NO commune in context: ResolveMiniApp is on core/grpcx.methodsWithoutTenant.
func (d *Directory) MiniApp(ctx context.Context, appID string) (MiniApp, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	ra, err := d.cl.ResolveMiniApp(ctx, &platformv1.ResolveMiniAppRequest{AppId: appID})
	if err != nil {
		// app_id is not personal data (platform.proto) and may be logged by the caller; it is not
		// put into the error text anyway — the caller already knows it.
		return MiniApp{}, false, fmt.Errorf("platformclient: ResolveMiniApp: %w", err)
	}
	app := ra.GetApp()
	if app == nil {
		return MiniApp{}, false, nil
	}

	kq := MiniApp{AppID: app.GetAppId()}
	switch app.GetMode() {
	case platformv1.MiniApp_MODE_MAIN:
		kq.CheDo = CheDoAppChinh
		if app.GetTenant() != nil {
			// A main app bound to a commune is the one shape that must never be served: the main
			// app enters a commune only through a confirmed QR (ADR 0044).
			return MiniApp{}, false, fmt.Errorf("%w: app chính mang xã", ErrNenTangTraSai)
		}
	case platformv1.MiniApp_MODE_COMMUNE:
		kq.CheDo = CheDoAppRieng
		if t := app.GetTenant(); t != nil {
			id := tenant.ID(t.GetId())
			if !id.Valid() {
				return MiniApp{}, false, fmt.Errorf("%w: mã xã của app riêng không phải ULID", ErrNenTangTraSai)
			}
			kq.XaRieng = id
		}
	default:
		kq.CheDo = CheDoKhongRo
	}
	return kq, true, nil
}

// XaTrongNguCanh reads THE COMMUNE ALREADY IN ctx from the registry — the bridge puts the commune
// it is about to open a session in there first (ADR 0045 §Miễn xã: GetTenant needs no exemption).
//
// The commune is not a parameter (rule 1, invariant 4): the GetTenant request's id is filled from
// the same context value the commune interceptor writes into metadata, so the two cannot differ.
//
// ok=false means the registry does not know the commune. An INACTIVE commune is returned with
// Active=false — the caller decides what inactive means for its case.
func (d *Directory) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) {
	id, ok := tenant.From(ctx)
	if !ok {
		return tenant.Tenant{}, false, fmt.Errorf("platformclient: XaTrongNguCanh: %w", tenant.ErrNoTenant)
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	ra, err := d.cl.GetTenant(ctx, &platformv1.GetTenantRequest{Id: id.String()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tenant.Tenant{}, false, nil
		}
		return tenant.Tenant{}, false, fmt.Errorf("platformclient: GetTenant: %w", err)
	}
	t := ra.GetTenant()
	if t == nil || tenant.ID(t.GetId()) != id {
		// Asked about one commune, answered about another (or none): never serve it.
		return tenant.Tenant{}, false, fmt.Errorf("%w: GetTenant trả về xã khác xã được hỏi", ErrNenTangTraSai)
	}
	return tenant.Tenant{
		ID:       id,
		Host:     t.GetHost(),
		Name:     t.GetDisplayName(),
		Active:   t.GetActive(),
		Province: t.GetProvince(),
	}, true, nil
}
