package grpc_test

// GetMapFrameDefault over bufconn with the real commune interceptor, fake store.

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
)

// framesFake answers by the commune IN ctx — the only source the real store has — and records which
// commune it was asked for.
type framesFake struct {
	byCommune map[tenant.ID]domain.MapFrameDefault
	broken    bool
	asked     *[]tenant.ID
}

func (f framesFake) MapFrameDefault(ctx context.Context) (domain.MapFrameDefault, bool, error) {
	id := tenant.MustFrom(ctx)
	if f.asked != nil {
		*f.asked = append(*f.asked, id)
	}
	if f.broken {
		return domain.MapFrameDefault{}, false, loiHaTang
	}
	v, ok := f.byCommune[id]
	return v, ok, nil
}

var frameUpdatedAt = time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC)

func sampleFrames() framesFake {
	return framesFake{byCommune: map[tenant.ID]domain.MapFrameDefault{
		xaTanPhu: {CenterLat: 15.730507, CenterLng: 108.37811, RadiusKm: 12.5, UpdatedAt: frameUpdatedAt, UpdatedBy: "VH-00001"},
	}}
}

func framesClient(t *testing.T, f svcgrpc.MapFrameDefaults) (platformv1.PlatformServiceClient, platformv1.PlatformServiceClient) {
	t.Helper()
	return dungVoi(t, svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(), HoSo: hoSoMau(),
		Policies: samplePolicies(), Fields: sampleFields(), MapFrames: f})
}

// The commune comes from "x-tenant-id" only: the store is asked for exactly the commune in metadata,
// and each commune gets its own answer — configured for one, absent for the other.
func TestGetMapFrameDefaultReadsTheCommuneFromMetadata(t *testing.T) {
	t.Parallel()
	var asked []tenant.ID
	f := sampleFrames()
	f.asked = &asked
	cli, _ := framesClient(t, f)

	res, err := cli.GetMapFrameDefault(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetMapFrameDefaultRequest{})
	if err != nil {
		t.Fatal(err)
	}
	fr := res.GetFrame()
	if fr == nil || fr.GetCenterLat() != 15.730507 || fr.GetCenterLng() != 108.37811 || fr.GetRadiusKm() != 12.5 ||
		!fr.GetUpdatedAt().AsTime().Equal(frameUpdatedAt) {
		t.Fatalf("frame = %+v", fr)
	}

	res, err = cli.GetMapFrameDefault(tenant.Into(ctxTest(t), xaDaSapNhap), &platformv1.GetMapFrameDefaultRequest{})
	if err != nil || res.GetFrame() != nil {
		t.Fatalf("other commune: res=%+v err=%v; want OK with NO frame (never another commune's)", res, err)
	}
	if len(asked) != 2 || asked[0] != xaTanPhu || asked[1] != xaDaSapNhap {
		t.Errorf("store asked for %v, want the communes of the two calls' metadata", asked)
	}
}

// No commune in metadata: refused by the interceptor before the store is reached.
func TestGetMapFrameDefaultWithoutCommuneRefused(t *testing.T) {
	t.Parallel()
	var asked []tenant.ID
	f := sampleFrames()
	f.asked = &asked
	_, raw := framesClient(t, f)
	_, err := raw.GetMapFrameDefault(ctxTest(t), &platformv1.GetMapFrameDefaultRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	if len(asked) != 0 {
		t.Error("the store was reached without a commune")
	}
}

// An outage is Internal — never an empty OK, which the caller reads as "not configured" — and its
// cause does not cross the boundary.
func TestGetMapFrameDefaultStoreFailureIsInternal(t *testing.T) {
	t.Parallel()
	cli, _ := framesClient(t, framesFake{broken: true})
	_, err := cli.GetMapFrameDefault(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.GetMapFrameDefaultRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == loiHaTang.Error() {
		t.Error("infrastructure error leaked to the caller")
	}
}

// A server built without the frame store is refused at construction, not at the first call.
func TestNewServerWithoutMapFramesPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("built a server without MapFrames")
		}
	}()
	_ = svcgrpc.NewServer(svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(), HoSo: hoSoMau(),
		Policies: samplePolicies(), Fields: sampleFields()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
