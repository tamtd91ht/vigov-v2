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
	appChinh       = "1000000000000000001"
	appRiengTanPhu = "1000000000000000002"
	appRiengXaCu   = "1000000000000000003"
)

type soMiniAppGia struct {
	apps map[string]domain.MiniApp
	hong bool
}

func (s soMiniAppGia) MiniApp(_ context.Context, appID string) (domain.MiniApp, error) {
	if s.hong {
		return domain.MiniApp{}, loiHaTang
	}
	a, ok := s.apps[appID]
	if !ok {
		// The store answers unknown, switched-off and soft-deleted alike; so does this fake.
		return domain.MiniApp{}, store.ErrKhongCoMiniApp
	}
	return a, nil
}

func soMiniAppMau() soMiniAppGia {
	return soMiniAppGia{apps: map[string]domain.MiniApp{
		appChinh: {AppID: appChinh, CheDo: domain.CheDoChinh},
		appRiengTanPhu: {AppID: appRiengTanPhu, CheDo: domain.CheDoRieng, Xa: &domain.Tenant{
			ID: xaTanPhu.String(), Ten: "Phường Tân Phú", TinhThanh: "Thành phố Đà Nẵng", DangHoatDong: true,
		}},
		appRiengXaCu: {AppID: appRiengXaCu, CheDo: domain.CheDoRieng, Xa: &domain.Tenant{
			ID: xaDaSapNhap.String(), Ten: "Xã Cũ", DangHoatDong: false,
		}},
	}}
}

// hoSoGia keys on the commune IN CONTEXT — the only input the real store has.
type hoSoGia struct {
	theoXa map[tenant.ID]domain.HoSoHienThi
	hong   bool
}

func (h hoSoGia) Doc(ctx context.Context) (domain.HoSoHienThi, error) {
	if h.hong {
		return domain.HoSoHienThi{}, loiHaTang
	}
	hs, ok := h.theoXa[tenant.MustFrom(ctx)]
	if !ok {
		return domain.HoSoHienThi{}, store.ErrChuaCoHoSoHienThi
	}
	return hs, nil
}

func hoSoMau() hoSoGia {
	return hoSoGia{theoXa: map[tenant.ID]domain.HoSoHienThi{
		xaTanPhu: {
			DiaChiTruSo:       "Số 1 đường Thử, Phường Tân Phú",
			LogoURL:           "https://example.gov.vn/logo-tan-phu.png",
			DuongDayNong:      "0900000000",
			GioLamViecHienThi: "Thứ 2 – Thứ 6",
			GioiThieu:         "Giới thiệu thử",
		},
	}}
}

func mayTrucTiep(apps svcgrpc.SoMiniApp) *svcgrpc.Server {
	return svcgrpc.NewServer(svcgrpc.Deps{Dir: danhBaMau(), Apps: apps, HoSo: hoSoMau()},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// ---- ResolveMiniApp -------------------------------------------------------------------

func TestResolveMiniAppKhongDangKyTraOKKhongCoApp(t *testing.T) {
	t.Parallel()
	res, err := mayTrucTiep(soMiniAppMau()).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: "9999999999"})
	if err != nil {
		t.Fatalf("app lạ phải là OK không có app, nhận lỗi: %v", err)
	}
	if res.GetApp() != nil {
		t.Fatalf("app lạ mà có app: %+v", res.GetApp())
	}
}

func TestResolveMiniAppAppChinhKhongBaoGioCoXa(t *testing.T) {
	t.Parallel()
	res, err := mayTrucTiep(soMiniAppMau()).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: appChinh})
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

func TestResolveMiniAppAppRiengXaHoatDongTraXa(t *testing.T) {
	t.Parallel()
	res, err := mayTrucTiep(soMiniAppMau()).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: appRiengTanPhu})
	if err != nil {
		t.Fatal(err)
	}
	a := res.GetApp()
	if a.GetMode() != platformv1.MiniApp_MODE_COMMUNE || a.GetAppId() != appRiengTanPhu {
		t.Fatalf("app = %+v", a)
	}
	if a.GetTenant().GetId() != xaTanPhu.String() ||
		a.GetTenant().GetDisplayName() != "Phường Tân Phú" ||
		a.GetTenant().GetProvince() != "Thành phố Đà Nẵng" {
		t.Errorf("xã = %+v", a.GetTenant())
	}
}

// The contract's signal for "bound commune inactive" is MODE_COMMUNE with no tenant — the app is
// still returned so the caller can tell this apart from an unknown app, and no successor is named.
func TestResolveMiniAppAppRiengXaNgungHoatDongKhongCoXa(t *testing.T) {
	t.Parallel()
	res, err := mayTrucTiep(soMiniAppMau()).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: appRiengXaCu})
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

func TestResolveMiniAppAppIDRongHoacCoKhoangTrang(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", " ", " " + appChinh} {
		_, err := mayTrucTiep(soMiniAppMau()).ResolveMiniApp(ctxTest(t),
			&platformv1.ResolveMiniAppRequest{AppId: id})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("app_id %q: mã = %v, muốn InvalidArgument", id, status.Code(err))
		}
	}
}

// An outage is never "not registered": the caller would refuse the citizen for the wrong reason
// and nobody would page.
func TestResolveMiniAppCSDLHongTraInternal(t *testing.T) {
	t.Parallel()
	_, err := mayTrucTiep(soMiniAppGia{hong: true}).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: appChinh})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == loiHaTang.Error() {
		t.Error("lỗi hạ tầng lọt ra bên gọi")
	}
}

// A dedicated app without its commune must not read as "commune inactive".
func TestResolveMiniAppAppRiengThieuXaLaLoiNoiBo(t *testing.T) {
	t.Parallel()
	hong := soMiniAppGia{apps: map[string]domain.MiniApp{
		appRiengTanPhu: {AppID: appRiengTanPhu, CheDo: domain.CheDoRieng},
	}}
	_, err := mayTrucTiep(hong).ResolveMiniApp(ctxTest(t),
		&platformv1.ResolveMiniAppRequest{AppId: appRiengTanPhu})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
}

// ---- GetTenantProfile -----------------------------------------------------------------

func TestGetTenantProfileKhongMangXaBiTuChoi(t *testing.T) {
	t.Parallel()
	_, tho := dung(t, danhBaMau())
	_, err := tho.GetTenantProfile(ctxTest(t), &platformv1.GetTenantProfileRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument", status.Code(err))
	}
}

func TestGetTenantProfileTraHoSoCuaDungXaTrongMetadata(t *testing.T) {
	t.Parallel()
	cli, _ := dung(t, danhBaMau())
	res, err := cli.GetTenantProfile(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetTenantProfileRequest{})
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
func TestGetTenantProfileXaKhacKhongDocDuocHoSoXaNay(t *testing.T) {
	t.Parallel()
	cli, _ := dung(t, danhBaMau())
	res, err := cli.GetTenantProfile(tenant.Into(ctxTest(t), xaDaSapNhap), &platformv1.GetTenantProfileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetProfile() != nil {
		t.Fatalf("xã chưa khai hồ sơ lại nhận hồ sơ: %+v", res.GetProfile())
	}
}

func TestGetTenantProfileCSDLHongTraInternal(t *testing.T) {
	t.Parallel()
	cli, _ := dungVoi(t, svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(), HoSo: hoSoGia{hong: true}})
	_, err := cli.GetTenantProfile(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetTenantProfileRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
}

func TestNewServerThieuPhuThuocThiPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ thiếu HoSo")
		}
	}()
	_ = svcgrpc.NewServer(svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau()},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}
