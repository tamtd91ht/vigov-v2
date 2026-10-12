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

// --- CommuneByLaunchCode: the same three outcomes, keyed by the QR's launch code -----------------

const launchCodeTest = "a1b2c3d4e5f6"

func TestCommuneByLaunchCodeSendsTheCodeNotAHost(t *testing.T) {
	gia := &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true, Province: "Thành phố Đà Nẵng",
	}}}
	d, _ := dungThu(t, gia)
	got, ok, err := d.CommuneByLaunchCode(context.Background(), launchCodeTest)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want resolved", ok, err)
	}
	// EXACTLY ONE of the two request fields: the platform answers INVALID_ARGUMENT to both or neither.
	if gia.lastReq.GetLaunchCode() != launchCodeTest || gia.lastReq.GetHost() != "" {
		t.Fatalf("request = %v, want launch_code only", gia.lastReq)
	}
	if got.Host != hostThu || got.Name != "Xã Thăng Bình" || got.Province != "Thành phố Đà Nẵng" || !got.Active {
		t.Fatalf("commune = %+v", got)
	}
}

func TestCommuneByLaunchCodeUnknownIsAnAnswerNotAnError(t *testing.T) {
	for _, ma := range []codes.Code{codes.NotFound, codes.InvalidArgument} {
		d, _ := dungThu(t, &nenTangGia{loi: status.Error(ma, "x")})
		_, ok, err := d.CommuneByLaunchCode(context.Background(), launchCodeTest)
		if ok || err != nil {
			t.Errorf("%s: ok=%v err=%v, want ok=false err=nil", ma, ok, err)
		}
	}
}

func TestCommuneByLaunchCodePlatformDownIsAnError(t *testing.T) {
	for _, ma := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Unauthenticated, codes.Internal} {
		d, _ := dungThu(t, &nenTangGia{loi: status.Error(ma, "x")})
		_, ok, err := d.CommuneByLaunchCode(context.Background(), launchCodeTest)
		if ok || err == nil {
			t.Errorf("%s: ok=%v err=%v, want an error", ma, ok, err)
		}
	}
}

func TestCommuneByLaunchCodeBrokenContractIsAnError(t *testing.T) {
	for name, ra := range map[string]*platformv1.ResolveHostResponse{
		"no tenant":   {},
		"id not ULID": {Tenant: &platformv1.Tenant{Id: "xa-thang-binh", Host: hostThu, Active: true}},
	} {
		d, _ := dungThu(t, &nenTangGia{ra: ra})
		_, ok, err := d.CommuneByLaunchCode(context.Background(), launchCodeTest)
		if ok || !errors.Is(err, ErrNenTangTraSai) {
			t.Errorf("%s: ok=%v err=%v, want ErrNenTangTraSai", name, ok, err)
		}
	}
}

func TestCommuneByLaunchCodePassesBlankHostAndInactiveThrough(t *testing.T) {
	// The CALLER decides: "" host (reserved primary) and Active=false are both "no commune" there.
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: "", DisplayName: "Xã cũ", Active: false,
	}}})
	got, ok, err := d.CommuneByLaunchCode(context.Background(), launchCodeTest)
	if err != nil || !ok || got.Active || got.Host != "" {
		t.Fatalf("ok=%v err=%v got=%+v, want passed through unchanged", ok, err, got)
	}
}

// --- CommuneByHost: XaTheoHost with Tenant.launch_code kept --------------------------------------

func TestCommuneByHostCarriesTheLaunchCode(t *testing.T) {
	gia := &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true, LaunchCode: launchCodeTest,
	}}}
	d, _ := dungThu(t, gia)
	got, ok, err := d.CommuneByHost(context.Background(), hostThu)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want resolved", ok, err)
	}
	if gia.lastReq.GetHost() != hostThu || gia.lastReq.GetLaunchCode() != "" {
		t.Fatalf("request = %v, want host only", gia.lastReq)
	}
	if got.LaunchCode != launchCodeTest || got.Name != "Xã Thăng Bình" || !got.Active {
		t.Fatalf("commune = %+v", got)
	}
}

// An older platform service sends no field 7: "" passes through, never a code made up here.
func TestCommuneByHostMissingLaunchCodeIsEmpty(t *testing.T) {
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{Tenant: &platformv1.Tenant{
		Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true,
	}}})
	got, ok, err := d.CommuneByHost(context.Background(), hostThu)
	if err != nil || !ok || got.LaunchCode != "" {
		t.Fatalf("ok=%v err=%v got=%+v, want LaunchCode \"\"", ok, err, got)
	}
}

// Outage and broken contract: the same errors XaTheoHost gives — one body, one mapping.
func TestCommuneByHostPlatformDownIsAnError(t *testing.T) {
	d, _ := dungThu(t, &nenTangGia{loi: status.Error(codes.Unavailable, "x")})
	if _, ok, err := d.CommuneByHost(context.Background(), hostThu); ok || err == nil {
		t.Fatalf("ok=%v err=%v, want an error", ok, err)
	}
	d, _ = dungThu(t, &nenTangGia{loi: status.Error(codes.NotFound, "x")})
	if _, ok, err := d.CommuneByHost(context.Background(), hostThu); ok || err != nil {
		t.Fatalf("ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}
