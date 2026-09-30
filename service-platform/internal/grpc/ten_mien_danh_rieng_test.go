package grpc_test

import (
	"context"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// danhBaDem wraps the fake directory and counts host lookups, so a test can prove the directory
// was never asked — not merely that the answer came out right.
type danhBaDem struct {
	*danhBaGia
	soLanHoi atomic.Int32
}

func (d *danhBaDem) ByHostErr(ctx context.Context, host string) (tenant.Tenant, error) {
	d.soLanHoi.Add(1)
	return d.danhBaGia.ByHostErr(ctx, host)
}

// THE CASE THIS CHANGE EXISTS FOR: a reserved host that IS in the directory, mapped to an active
// commune — exactly the admin.vigov.vn row the deploy pipeline wrote. It must answer what an
// unclaimed Host answers, code AND message, and the directory must not be consulted at all.
func TestResolveHostTenMienDanhRiengTraNhuHostLaVaKhongHoiDanhBa(t *testing.T) {
	t.Parallel()

	mau := danhBaMau()
	tanPhu := mau.theoHost["tanphu.vigov.vn"]
	danhRieng := []string{
		"admin.vigov.vn", "ADMIN.vigov.vn.", "admin-stg.vigov.vn", "api.vigov.vn",
		"stg.vigov.vn", "www.vigov.vn", "vigov.vn", "identity.api.vigov.vn",
		"petitions.api-stg.vigov.vn", "admin.stg.vigov.vn", "api.stg.vigov.vn",
	}
	for _, h := range danhRieng {
		// Stored in the fake's normalised key shape, so a lookup WOULD succeed if it happened.
		mau.theoHost[h] = tanPhu
	}
	mau.theoHost["admin.vigov.vn."] = tanPhu
	dir := &danhBaDem{danhBaGia: mau}
	cli, _ := dung(t, dir)
	ctx := ctxTest(t)

	_, loiLa := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: "khonghe.vigov.vn"})
	muon := status.Convert(loiLa)
	dir.soLanHoi.Store(0)

	for _, h := range danhRieng {
		res, err := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: h})
		if err == nil {
			t.Errorf("%q phân giải ra xã %q — tên miền của nền tảng không bao giờ là của một xã",
				h, res.GetTenant().GetId())
			continue
		}
		got := status.Convert(err)
		if got.Code() != muon.Code() || got.Message() != muon.Message() {
			t.Errorf("%q: (%v, %q), muốn giống hệt host lạ (%v, %q) — khác đi là lộ tên miền "+
				"nào thuộc nền tảng", h, got.Code(), got.Message(), muon.Code(), muon.Message())
		}
	}
	if n := dir.soLanHoi.Load(); n != 0 {
		t.Fatalf("danh bạ bị hỏi %d lần cho tên miền danh riêng — phải từ chối TRƯỚC khi tra bảng", n)
	}

	// And a commune host still resolves through the same server: the guard refuses a shape, not
	// everything.
	if _, err := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: "tanphu.vigov.vn"}); err != nil {
		t.Fatalf("host của xã bị chặn nhầm: %v", err)
	}
}
