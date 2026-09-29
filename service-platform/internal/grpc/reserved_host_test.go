package grpc_test

import (
	"context"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// countingDirectory wraps the fake directory and counts host lookups, so a test can prove the directory
// was never asked — not merely that the answer came out right.
type countingDirectory struct {
	*fakeDirectory
	lookups atomic.Int32
}

func (d *countingDirectory) ByHostErr(ctx context.Context, host string) (tenant.Tenant, error) {
	d.lookups.Add(1)
	return d.fakeDirectory.ByHostErr(ctx, host)
}

// THE CASE THIS CHANGE EXISTS FOR: a reserved host that IS in the directory, mapped to an active
// commune — exactly the admin.vigov.vn row the deploy pipeline wrote. It must answer what an
// unclaimed Host answers, code AND message, and the directory must not be consulted at all.
func TestResolveHostReservedAnswersLikeUnknownWithoutLookup(t *testing.T) {
	t.Parallel()

	sample := sampleDirectory()
	active := sample.byHost["tanphu.vigov.vn"]
	reserved := []string{
		"admin.vigov.vn", "ADMIN.vigov.vn.", "admin-stg.vigov.vn", "api.vigov.vn",
		"stg.vigov.vn", "www.vigov.vn", "vigov.vn", "identity.api.vigov.vn",
		"petitions.api-stg.vigov.vn", "admin.stg.vigov.vn", "api.stg.vigov.vn",
	}
	for _, h := range reserved {
		// Stored in the fake's normalised key shape, so a lookup WOULD succeed if it happened.
		sample.byHost[h] = active
	}
	sample.byHost["admin.vigov.vn."] = active
	dir := &countingDirectory{fakeDirectory: sample}
	cli, _ := start(t, dir)
	ctx := testCtx(t)

	_, unknownErr := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: "khonghe.vigov.vn"})
	want := status.Convert(unknownErr)
	dir.lookups.Store(0)

	for _, h := range reserved {
		res, err := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: h})
		if err == nil {
			t.Errorf("%q phân giải ra xã %q — tên miền của nền tảng không bao giờ là của một xã",
				h, res.GetTenant().GetId())
			continue
		}
		got := status.Convert(err)
		if got.Code() != want.Code() || got.Message() != want.Message() {
			t.Errorf("%q: (%v, %q), muốn giống hệt host lạ (%v, %q) — khác đi là lộ tên miền "+
				"nào thuộc nền tảng", h, got.Code(), got.Message(), want.Code(), want.Message())
		}
	}
	if n := dir.lookups.Load(); n != 0 {
		t.Fatalf("danh bạ bị hỏi %d lần cho tên miền danh riêng — phải từ chối TRƯỚC khi tra bảng", n)
	}

	// And a commune host still resolves through the same server: the guard refuses a shape, not
	// everything.
	if _, err := cli.ResolveHost(ctx, &platformv1.ResolveHostRequest{Host: "tanphu.vigov.vn"}); err != nil {
		t.Fatalf("host của xã bị chặn nhầm: %v", err)
	}
}
