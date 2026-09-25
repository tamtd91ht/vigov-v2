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

type nenTangMiniAppGia struct {
	platformv1.PlatformServiceClient
	app    *platformv1.ResolveMiniAppResponse
	loiApp error
	xa     *platformv1.GetTenantResponse
	loiXa  error
	hoiXa  string
}

func (n *nenTangMiniAppGia) ResolveMiniApp(context.Context, *platformv1.ResolveMiniAppRequest,
	...grpc.CallOption) (*platformv1.ResolveMiniAppResponse, error) {
	return n.app, n.loiApp
}

func (n *nenTangMiniAppGia) GetTenant(_ context.Context, r *platformv1.GetTenantRequest,
	_ ...grpc.CallOption) (*platformv1.GetTenantResponse, error) {
	n.hoiXa = r.GetId()
	return n.xa, n.loiXa
}

func TestMiniAppChuaDangKyLaOkFalseKhongPhaiLoi(t *testing.T) {
	d := NewDirectory(&nenTangMiniAppGia{app: &platformv1.ResolveMiniAppResponse{}}, nil)
	_, ok, err := d.MiniApp(context.Background(), "1234567890")
	if err != nil || ok {
		t.Fatalf("app chưa đăng ký: ok=%v err=%v, muốn ok=false err=nil", ok, err)
	}
}

func TestMiniAppHongMangLaLoiKhongPhaiChuaDangKy(t *testing.T) {
	// The distinction the bridge needs: UNAVAILABLE, never "not registered".
	d := NewDirectory(&nenTangMiniAppGia{loiApp: status.Error(codes.Unavailable, "down")}, nil)
	_, ok, err := d.MiniApp(context.Background(), "1234567890")
	if err == nil || ok || status.Code(errors.Unwrap(err)) != codes.Unavailable {
		t.Fatalf("mất mạng tới nền tảng: ok=%v err=%v", ok, err)
	}
}

func TestMiniAppHaiCheDo(t *testing.T) {
	chinh := NewDirectory(&nenTangMiniAppGia{app: &platformv1.ResolveMiniAppResponse{
		App: &platformv1.MiniApp{AppId: "a1", Mode: platformv1.MiniApp_MODE_MAIN}}}, nil)
	a, ok, err := chinh.MiniApp(context.Background(), "a1")
	if err != nil || !ok || a.CheDo != CheDoAppChinh || a.XaRieng != "" {
		t.Fatalf("app chính: %+v ok=%v err=%v", a, ok, err)
	}

	rieng := NewDirectory(&nenTangMiniAppGia{app: &platformv1.ResolveMiniAppResponse{
		App: &platformv1.MiniApp{AppId: "a2", Mode: platformv1.MiniApp_MODE_COMMUNE,
			Tenant: &platformv1.TenantSummary{Id: ulidThu}}}}, nil)
	a, ok, err = rieng.MiniApp(context.Background(), "a2")
	if err != nil || !ok || a.CheDo != CheDoAppRieng || a.XaRieng != tenant.ID(ulidThu) {
		t.Fatalf("app riêng: %+v ok=%v err=%v", a, ok, err)
	}

	// Dedicated app whose commune is not active: the platform leaves `tenant` absent.
	ngung := NewDirectory(&nenTangMiniAppGia{app: &platformv1.ResolveMiniAppResponse{
		App: &platformv1.MiniApp{AppId: "a3", Mode: platformv1.MiniApp_MODE_COMMUNE}}}, nil)
	a, ok, err = ngung.MiniApp(context.Background(), "a3")
	if err != nil || !ok || a.CheDo != CheDoAppRieng || a.XaRieng != "" {
		t.Fatalf("app riêng xã ngừng: %+v ok=%v err=%v", a, ok, err)
	}
}

func TestMiniAppTraSaiHopDongThiLoi(t *testing.T) {
	for ten, app := range map[string]*platformv1.MiniApp{
		"app chính mang xã":  {AppId: "a", Mode: platformv1.MiniApp_MODE_MAIN, Tenant: &platformv1.TenantSummary{Id: ulidThu}},
		"xã không phải ULID": {AppId: "a", Mode: platformv1.MiniApp_MODE_COMMUNE, Tenant: &platformv1.TenantSummary{Id: "xa-thang-binh"}},
	} {
		t.Run(ten, func(t *testing.T) {
			d := NewDirectory(&nenTangMiniAppGia{app: &platformv1.ResolveMiniAppResponse{App: app}}, nil)
			if _, _, err := d.MiniApp(context.Background(), "a"); !errors.Is(err, ErrNenTangTraSai) {
				t.Fatalf("err = %v, muốn ErrNenTangTraSai", err)
			}
		})
	}
}

func TestXaTrongNguCanhHoiDungXaCuaNguCanh(t *testing.T) {
	gia := &nenTangMiniAppGia{xa: &platformv1.GetTenantResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, DisplayName: "Xã Thăng Bình", Active: true}}}
	d := NewDirectory(gia, nil)

	x, ok, err := d.XaTrongNguCanh(tenant.Into(context.Background(), tenant.ID(ulidThu)))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if gia.hoiXa != ulidThu {
		t.Errorf("GetTenant hỏi xã %q, muốn đúng xã trong ngữ cảnh", gia.hoiXa)
	}
	if x.Name != "Xã Thăng Bình" || !x.Active {
		t.Errorf("xã = %+v", x)
	}
}

func TestXaTrongNguCanhKhongCoXaThiLoiTruocKhiGoi(t *testing.T) {
	gia := &nenTangMiniAppGia{}
	d := NewDirectory(gia, nil)
	if _, _, err := d.XaTrongNguCanh(context.Background()); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("err = %v, muốn ErrNoTenant", err)
	}
	if gia.hoiXa != "" {
		t.Error("đã gọi nền tảng dù ngữ cảnh không có xã")
	}
}

func TestXaTrongNguCanhKhongCoThiOkFalseConHongThiLoi(t *testing.T) {
	ctx := tenant.Into(context.Background(), tenant.ID(ulidThu))

	khongCo := NewDirectory(&nenTangMiniAppGia{loiXa: status.Error(codes.NotFound, "x")}, nil)
	if _, ok, err := khongCo.XaTrongNguCanh(ctx); ok || err != nil {
		t.Fatalf("NotFound: ok=%v err=%v, muốn ok=false err=nil", ok, err)
	}

	hong := NewDirectory(&nenTangMiniAppGia{loiXa: status.Error(codes.Unavailable, "x")}, nil)
	if _, ok, err := hong.XaTrongNguCanh(ctx); ok || err == nil {
		t.Fatalf("Unavailable: ok=%v err=%v, muốn lỗi", ok, err)
	}

	lech := NewDirectory(&nenTangMiniAppGia{xa: &platformv1.GetTenantResponse{Tenant: &platformv1.Tenant{
		Id: "01JD8ZQK9M3NPXR7TVWYB2C4EG", Active: true}}}, nil)
	if _, _, err := lech.XaTrongNguCanh(ctx); !errors.Is(err, ErrNenTangTraSai) {
		t.Fatalf("xã trả về khác xã hỏi: err = %v, muốn ErrNenTangTraSai", err)
	}
}
