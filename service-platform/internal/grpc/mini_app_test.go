package grpc_test

// ResolveMiniApp and GetTenantProfile.
//
// ResolveMiniApp's mapping is tested by calling the handler DIRECTLY, not over bufconn: the RPC is
// not yet on core/grpcx.methodsWithoutTenant (a separate task adds it), so over the wire the
// server interceptor refuses it — cmd/server/main_test.go pins that. What is under test here is
// the status table on the RPC in platform.proto, which is a property of the handler alone.
//
// GetTenantProfile goes over bufconn with the real interceptor, because what matters there is that
// the commune comes from metadata and nowhere else.

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const (
	appMain          = "1000000000000000001"
	appCommuneActive = "1000000000000000002"
	appCommuneMerged = "1000000000000000003"
)

type fakeMiniApps struct {
	apps   map[string]domain.MiniApp
	broken bool
}

func (s fakeMiniApps) MiniApp(_ context.Context, appID string) (domain.MiniApp, error) {
	if s.broken {
		return domain.MiniApp{}, errInfra
	}
	a, ok := s.apps[appID]
	if !ok {
		// The store answers unknown, switched-off and soft-deleted alike; so does this fake.
		return domain.MiniApp{}, store.ErrMiniAppNotFound
	}
	return a, nil
}

func sampleMiniApps() fakeMiniApps {
	return fakeMiniApps{apps: map[string]domain.MiniApp{
		appMain: {AppID: appMain, Mode: domain.MiniAppModeMain},
		appCommuneActive: {AppID: appCommuneActive, Mode: domain.MiniAppModeCommune, Tenant: &domain.Tenant{
			ID: tenantActive.String(), Name: "Phường Tân Phú", Province: "Thành phố Đà Nẵng", IsActive: true,
		}},
		appCommuneMerged: {AppID: appCommuneMerged, Mode: domain.MiniAppModeCommune, Tenant: &domain.Tenant{
			ID: tenantMerged.String(), Name: "Xã Cũ", IsActive: false,
		}},
	}}
}

// fakeProfiles keys on the commune IN CONTEXT — the only input the real store has.
type fakeProfiles struct {
	byTenant map[tenant.ID]domain.CommuneProfile
	broken   bool
}

func (h fakeProfiles) Read(ctx context.Context) (domain.CommuneProfile, error) {
	if h.broken {
		return domain.CommuneProfile{}, errInfra
	}
	profile, ok := h.byTenant[tenant.MustFrom(ctx)]
	if !ok {
		return domain.CommuneProfile{}, store.ErrCommuneProfileNotDeclared
	}
	return profile, nil
}

func sampleProfiles() fakeProfiles {
	return fakeProfiles{byTenant: map[tenant.ID]domain.CommuneProfile{
		tenantActive: {
			OfficeAddress:   "Số 1 đường Thử, Phường Tân Phú",
			LogoURL:         "https://example.gov.vn/logo-tan-phu.png",
			Hotline:         "0900000000",
			OfficeHoursText: "Thứ 2 – Thứ 6",
			Introduction:    "Giới thiệu thử",
		},
	}}
}

func directServer(apps svcgrpc.MiniAppRegistry) *svcgrpc.Server {
	return svcgrpc.NewServer(svcgrpc.Deps{Dir: sampleDirectory(), Apps: apps, Profiles: sampleProfiles(), Policies: samplePolicies(),
		Fields: sampleFields()},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// ---- ResolveMiniApp -------------------------------------------------------------------

func TestResolveMiniAppUnregisteredReturnsOKWithoutApp(t *testing.T) {
	t.Parallel()
	res, err := directServer(sampleMiniApps()).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: "9999999999"})
	if err != nil {
		t.Fatalf("app lạ phải là OK không có app, nhận lỗi: %v", err)
	}
	if res.GetApp() != nil {
		t.Fatalf("app lạ mà có app: %+v", res.GetApp())
	}
}

func TestResolveMiniAppMainAppNeverHasTenant(t *testing.T) {
	t.Parallel()
	res, err := directServer(sampleMiniApps()).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: appMain})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetApp().GetMode() != platformv1.MiniApp_MODE_MAIN {
		t.Errorf("mode = %v, muốn MODE_MAIN", res.GetApp().GetMode())
	}
	if res.GetApp().GetTenant() != nil {
		t.Errorf("app chính mang xã %+v — một xã mặc định trên đường cách ly", res.GetApp().GetTenant())
	}
}

func TestResolveMiniAppCommuneAppActiveTenantReturned(t *testing.T) {
	t.Parallel()
	res, err := directServer(sampleMiniApps()).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: appCommuneActive})
	if err != nil {
		t.Fatal(err)
	}
	a := res.GetApp()
	if a.GetMode() != platformv1.MiniApp_MODE_COMMUNE || a.GetAppId() != appCommuneActive {
		t.Fatalf("app = %+v", a)
	}
	if a.GetTenant().GetId() != tenantActive.String() ||
		a.GetTenant().GetDisplayName() != "Phường Tân Phú" ||
		a.GetTenant().GetProvince() != "Thành phố Đà Nẵng" {
		t.Errorf("xã = %+v", a.GetTenant())
	}
}

// The contract's signal for "bound commune inactive" is MODE_COMMUNE with no tenant — the app is
// still returned so the caller can tell this apart from an unknown app, and no successor is named.
func TestResolveMiniAppCommuneAppInactiveTenantOmitted(t *testing.T) {
	t.Parallel()
	res, err := directServer(sampleMiniApps()).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: appCommuneMerged})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetApp() == nil || res.GetApp().GetMode() != platformv1.MiniApp_MODE_COMMUNE {
		t.Fatalf("app = %+v, muốn MODE_COMMUNE", res.GetApp())
	}
	if res.GetApp().GetTenant() != nil {
		t.Errorf("xã ngừng hoạt động vẫn được trả: %+v", res.GetApp().GetTenant())
	}
}

func TestResolveMiniAppAppIDEmptyOrPadded(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", " ", " " + appMain} {
		_, err := directServer(sampleMiniApps()).ResolveMiniApp(testCtx(t),
			&platformv1.ResolveMiniAppRequest{AppId: id})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("app_id %q: mã = %v, muốn InvalidArgument", id, status.Code(err))
		}
	}
}

// An outage is never "not registered": the caller would refuse the citizen for the wrong reason
// and nobody would page.
func TestResolveMiniAppDatabaseDownReturnsInternal(t *testing.T) {
	t.Parallel()
	_, err := directServer(fakeMiniApps{broken: true}).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: appMain})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == errInfra.Error() {
		t.Error("lỗi hạ tầng lọt ra bên gọi")
	}
}

// A dedicated app without its commune must not read as "commune inactive".
func TestResolveMiniAppCommuneAppWithoutTenantIsInternal(t *testing.T) {
	t.Parallel()
	broken := fakeMiniApps{apps: map[string]domain.MiniApp{
		appCommuneActive: {AppID: appCommuneActive, Mode: domain.MiniAppModeCommune},
	}}
	_, err := directServer(broken).ResolveMiniApp(testCtx(t),
		&platformv1.ResolveMiniAppRequest{AppId: appCommuneActive})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
}

// ---- GetTenantProfile -----------------------------------------------------------------

func TestGetTenantProfileWithoutTenantRefused(t *testing.T) {
	t.Parallel()
	_, raw := start(t, sampleDirectory())
	_, err := raw.GetTenantProfile(testCtx(t), &platformv1.GetTenantProfileRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument", status.Code(err))
	}
}

func TestGetTenantProfileReturnsMetadataTenantsProfile(t *testing.T) {
	t.Parallel()
	cli, _ := start(t, sampleDirectory())
	res, err := cli.GetTenantProfile(tenant.Into(testCtx(t), tenantActive), &platformv1.GetTenantProfileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	p := res.GetProfile()
	if p.GetOfficeAddress() != "Số 1 đường Thử, Phường Tân Phú" ||
		p.GetLogoUrl() != "https://example.gov.vn/logo-tan-phu.png" ||
		p.GetHotline() != "0900000000" ||
		p.GetOfficeHoursText() != "Thứ 2 – Thứ 6" ||
		p.GetIntroduction() != "Giới thiệu thử" {
		t.Errorf("hồ sơ sai trường (ánh xạ từng trường một): %+v", p)
	}
}

// Commune B, whose profile does not exist, must NOT receive commune A's — the only answer is
// "not declared".
func TestGetTenantProfileOtherTenantCannotReadThisProfile(t *testing.T) {
	t.Parallel()
	cli, _ := start(t, sampleDirectory())
	res, err := cli.GetTenantProfile(tenant.Into(testCtx(t), tenantMerged), &platformv1.GetTenantProfileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetProfile() != nil {
		t.Fatalf("xã chưa khai hồ sơ lại nhận hồ sơ: %+v", res.GetProfile())
	}
}

func TestGetTenantProfileDatabaseDownReturnsInternal(t *testing.T) {
	t.Parallel()
	cli, _ := startWith(t, svcgrpc.Deps{Dir: sampleDirectory(), Apps: sampleMiniApps(), Profiles: fakeProfiles{broken: true},
		Policies: samplePolicies(), Fields: sampleFields()})
	_, err := cli.GetTenantProfile(tenant.Into(testCtx(t), tenantActive), &platformv1.GetTenantProfileRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
}

func TestNewServerMissingDependencyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ thiếu Profiles")
		}
	}()
	_ = svcgrpc.NewServer(svcgrpc.Deps{Dir: sampleDirectory(), Apps: sampleMiniApps(), Policies: samplePolicies(),
		Fields: sampleFields()},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}
