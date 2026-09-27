package platformclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
)

// XaTheoHost exists for ONE property ByHost does not have: an outage is an error, not "unknown".
// Each case below is one way that property could be lost.

func TestXaTheoHostPhanGiaiDuocThiTraXaKemTinh(t *testing.T) {
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true, Province: "Thành phố Đà Nẵng",
	}}})
	got, ok, err := d.XaTheoHost(context.Background(), hostThu)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, muốn phân giải được", ok, err)
	}
	if got.Name != "Xã Thăng Bình" || got.Province != "Thành phố Đà Nẵng" || !got.Active {
		t.Fatalf("xã trả về sai: %+v", got)
	}
}

func TestXaTheoHostKhongCoXaLaMotCauTraLoiKhongPhaiLoi(t *testing.T) {
	for _, ma := range []codes.Code{codes.NotFound, codes.InvalidArgument} {
		d, _ := dungThu(t, &nenTangGia{loi: status.Error(ma, "x")})
		_, ok, err := d.XaTheoHost(context.Background(), hostThu)
		if ok || err != nil {
			t.Errorf("%s: ok=%v err=%v, muốn ok=false err=nil", ma, ok, err)
		}
	}
}

func TestXaTheoHostNenTangChetLaLoiKhongPhaiKhongCoXa(t *testing.T) {
	// THE CASE THE METHOD EXISTS FOR. Folding this into ok=false would tell a citizen the QR they
	// scanned names no commune, during what is only a platform blip.
	for _, ma := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Unauthenticated, codes.Internal} {
		d, _ := dungThu(t, &nenTangGia{loi: status.Error(ma, "x")})
		_, ok, err := d.XaTheoHost(context.Background(), hostThu)
		if ok || err == nil {
			t.Errorf("%s: ok=%v err=%v, muốn một lỗi", ma, ok, err)
		}
	}
}

func TestXaTheoHostTraSaiHopDongLaLoi(t *testing.T) {
	for ten, ra := range map[string]*platformv1.ResolveHostResponse{
		"không có xã":        {},
		"mã không phải ULID": {Tenant: &platformv1.Tenant{Id: "xa-thang-binh", DisplayName: "Xã Thăng Bình", Active: true}},
	} {
		d, _ := dungThu(t, &nenTangGia{ra: ra})
		_, ok, err := d.XaTheoHost(context.Background(), hostThu)
		if ok || !errors.Is(err, ErrNenTangTraSai) {
			t.Errorf("%s: ok=%v err=%v, muốn ErrNenTangTraSai", ten, ok, err)
		}
	}
}

func TestXaTheoHostXaNgungHoatDongVanTraVeKemCo(t *testing.T) {
	// Passed through with Active=false, as ByHost does: the CALLER decides what inactive means.
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: hostThu, DisplayName: "Xã cũ", Active: false,
	}}})
	got, ok, err := d.XaTheoHost(context.Background(), hostThu)
	if err != nil || !ok || got.Active {
		t.Fatalf("ok=%v err=%v active=%v, muốn xã trả về với Active=false", ok, err, got.Active)
	}
}
