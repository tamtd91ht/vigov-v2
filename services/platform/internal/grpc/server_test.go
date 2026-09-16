package grpc_test

// End to end over bufconn: real grpc-go, real interceptor, real generated stubs, fake
// directory. No PostgreSQL — the distinctions under test are about which failure maps to
// which code, and a real database cannot be made to fail on demand in a unit test.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	platformv1 "github.com/vihat/vigov/gen/vigov/platform/v1"
	"github.com/vihat/vigov/pkg/grpcx"
	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/services/platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/services/platform/internal/grpc"
	"github.com/vihat/vigov/services/platform/internal/store"
)

const (
	xaTanPhu    = tenant.ID("01J8Z4K2R7QG5TMN9WXYB3CDEF")
	xaDaSapNhap = tenant.ID("01J8Z4K2R7QG5TMN9WXYB3CDEG")
)

// loiHaTang stands in for "the database is unreachable". It is deliberately NOT one of the
// store's sentinel errors: the mapping under test is precisely that an unknown failure becomes
// Internal instead of being mistaken for a missing commune.
var loiHaTang = errors.New("pgx: connection refused on 10.0.0.7:5432")

type danhBaGia struct {
	theoHost map[string]tenant.Tenant
	theoID   map[tenant.ID]tenant.Tenant
	hong     bool // every lookup fails as infrastructure
}

func (d *danhBaGia) ByHostErr(_ context.Context, host string) (tenant.Tenant, error) {
	if d.hong {
		return tenant.Tenant{}, loiHaTang
	}
	h, err := domain.NormaliseHost(host)
	if err != nil {
		return tenant.Tenant{}, err
	}
	t, ok := d.theoHost[h]
	if !ok {
		return tenant.Tenant{}, store.ErrKhongCoXa
	}
	if !t.Active {
		return tenant.Tenant{}, store.ErrXaNgungHoatDong
	}
	return t, nil
}

func (d *danhBaGia) ByID(_ context.Context, id tenant.ID) (tenant.Tenant, error) {
	if d.hong {
		return tenant.Tenant{}, loiHaTang
	}
	t, ok := d.theoID[id]
	if !ok {
		return tenant.Tenant{}, store.ErrKhongCoXa
	}
	return t, nil // a deactivated commune is still describable — rule 7
}

func danhBaMau() *danhBaGia {
	tanPhu := tenant.Tenant{ID: xaTanPhu, Host: "tanphu.vigov.vn", Name: "Phường Tân Phú", Active: true}
	cu := tenant.Tenant{ID: xaDaSapNhap, Host: "xacu.vigov.vn", Name: "Xã Cũ", Active: false}
	return &danhBaGia{
		theoHost: map[string]tenant.Tenant{tanPhu.Host: tanPhu, cu.Host: cu},
		theoID:   map[tenant.ID]tenant.Tenant{xaTanPhu: tanPhu, xaDaSapNhap: cu},
	}
}

// dung starts the real server behind bufconn and returns two clients: one that goes through
// the grpcx client interceptor (the production path) and one raw, used to send calls the
// interceptor would never allow — that is the only way to test what the SERVER does with bad
// or missing metadata.
func dung(t *testing.T, dir svcgrpc.Directory) (platformv1.PlatformServiceClient, platformv1.PlatformServiceClient) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor()))
	platformv1.RegisterPlatformServiceServer(srv,
		svcgrpc.NewServer(dir, slog.New(slog.NewTextHandler(io.Discard, nil))))

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	quay := func(opts ...grpc.DialOption) platformv1.PlatformServiceClient {
		opts = append(opts,
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		cc, err := grpc.NewClient("passthrough:///bufnet", opts...)
		if err != nil {
			t.Fatalf("không mở được kết nối: %v", err)
		}
		t.Cleanup(func() { _ = cc.Close() })
		return platformv1.NewPlatformServiceClient(cc)
	}

	return quay(grpc.WithUnaryInterceptor(grpcx.UnaryClientInterceptor())), quay()
}

func ctxTest(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// ---- ResolveHost ----------------------------------------------------------------------

// The exemption has to hold end to end: ResolveHost is what ESTABLISHES the commune, so a
// server interceptor demanding one here would make every edge unable to resolve its Host.
func TestResolveHostChayDuocKhiKhongCoXaTrongContext(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	res, err := cli.ResolveHost(ctxTest(t), &platformv1.ResolveHostRequest{Host: "TanPhu.ViGov.vn:443"})
	if err != nil {
		t.Fatalf("ResolveHost lỗi: %v", err)
	}
	if got := res.GetTenant().GetId(); got != xaTanPhu.String() {
		t.Fatalf("id = %q, muốn %q", got, xaTanPhu)
	}
	if res.GetTenant().GetDisplayName() != "Phường Tân Phú" || !res.GetTenant().GetActive() {
		t.Fatalf("siêu dữ liệu sai: %+v", res.GetTenant())
	}
}

func TestResolveHostKhongKhopTraNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	_, err := cli.ResolveHost(ctxTest(t), &platformv1.ResolveHostRequest{Host: "khonghe.vigov.vn"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

// A deactivated commune keeps its data and its address (rule 7) but stops serving, and the
// caller must not be able to tell it apart from a Host nobody has claimed — that difference
// discloses which communes exist on the platform.
func TestResolveHostXaNgungHoatDongTraNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	_, err := cli.ResolveHost(ctxTest(t), &platformv1.ResolveHostRequest{Host: "xacu.vigov.vn"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

// THE ONE THIS FILE EXISTS FOR. A database outage reported as NotFound tells every service
// edge that no commune exists, and sends the operator hunting a DNS problem that is not there.
func TestResolveHostCSDLHongTraInternalChuKhongPhaiNotFound(t *testing.T) {
	t.Parallel()

	dir := danhBaMau()
	dir.hong = true
	cli, _ := dung(t, dir)

	_, err := cli.ResolveHost(ctxTest(t), &platformv1.ResolveHostRequest{Host: "tanphu.vigov.vn"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
	// And the cause stays on this side of the boundary: an internal message can carry a host,
	// a port or a query fragment, and this response crosses into another service.
	if strings.Contains(err.Error(), "10.0.0.7") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("lỗi nội bộ rò ra ngoài: %v", err)
	}
}

func TestResolveHostRong(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	_, err := cli.ResolveHost(ctxTest(t), &platformv1.ResolveHostRequest{Host: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

// ---- GetTenant ------------------------------------------------------------------------

// GetTenant is NOT on the exemption list. A call reaching it without a commune in metadata is
// refused by the interceptor before the handler sees it — no default, no guess.
func TestGetTenantKhongCoMetadataXaBiTuChoi(t *testing.T) {
	t.Parallel()

	_, tho := dung(t, danhBaMau())
	_, err := tho.GetTenant(ctxTest(t), &platformv1.GetTenantRequest{Id: xaTanPhu.String()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantNhieuGiaTriMetadataBiTuChoi(t *testing.T) {
	t.Parallel()

	_, tho := dung(t, danhBaMau())
	ctx := metadata.AppendToOutgoingContext(ctxTest(t),
		grpcx.MetadataTenantKey, xaTanPhu.String(),
		grpcx.MetadataTenantKey, xaDaSapNhap.String())

	_, err := tho.GetTenant(ctx, &platformv1.GetTenantRequest{Id: xaTanPhu.String()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantTraSieuDuLieu(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	ctx := tenant.Into(ctxTest(t), xaTanPhu)

	res, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: xaTanPhu.String()})
	if err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
	got := res.GetTenant()
	if got.GetId() != xaTanPhu.String() || got.GetHost() != "tanphu.vigov.vn" ||
		got.GetDisplayName() != "Phường Tân Phú" || !got.GetActive() {
		t.Fatalf("siêu dữ liệu sai: %+v", got)
	}
}

// A merged commune must stay describable: rule 7 keeps its data and its codes, and an archival
// record referring to it still has to be able to render a name. Active=false is the answer,
// NotFound is not.
func TestGetTenantXaNgungHoatDongVanTraVeVoiActiveFalse(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	ctx := tenant.Into(ctxTest(t), xaTanPhu)

	res, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: xaDaSapNhap.String()})
	if err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
	if res.GetTenant().GetActive() {
		t.Fatal("xã đã ngừng hoạt động lại báo Active=true")
	}
}

func TestGetTenantIdKhongPhaiULID(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	ctx := tenant.Into(ctxTest(t), xaTanPhu)

	// An administrative code instead of a ULID: rule 1, invariant 2 forbids a meaningful
	// identifier, and saying so beats a NotFound the caller reads as "commune gone".
	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: "26734"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantKhongTonTaiTraNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())
	ctx := tenant.Into(ctxTest(t), xaTanPhu)

	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: "01J8Z4K2R7QG5TMN9WXYB3CDEH"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantCSDLHongTraInternal(t *testing.T) {
	t.Parallel()

	dir := danhBaMau()
	dir.hong = true
	cli, _ := dung(t, dir)
	ctx := tenant.Into(ctxTest(t), xaTanPhu)

	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: xaTanPhu.String()})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
}

// The production client path end to end: the commune goes out as metadata and arrives as
// context on the other side, without any business argument naming it.
func TestClientInterceptorMangXaQuaDuongDayThat(t *testing.T) {
	t.Parallel()

	cli, _ := dung(t, danhBaMau())

	// No commune in context: the client interceptor refuses before anything is sent.
	if _, err := cli.GetTenant(ctxTest(t),
		&platformv1.GetTenantRequest{Id: xaTanPhu.String()}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}

	// With one, the same call succeeds.
	if _, err := cli.GetTenant(tenant.Into(ctxTest(t), xaTanPhu),
		&platformv1.GetTenantRequest{Id: xaTanPhu.String()}); err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
}
