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

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const (
	tenantActive = tenant.ID("01J8Z4K2R7QG5TMN9WXYB3CDEF")
	tenantMerged = tenant.ID("01J8Z4K2R7QG5TMN9WXYB3CDEG")
)

// errInfra stands in for "the database is unreachable". It is deliberately NOT one of the
// store's sentinel errors: the mapping under test is precisely that an unknown failure becomes
// Internal instead of being mistaken for a missing commune.
var errInfra = errors.New("pgx: connection refused on 10.0.0.7:5432")

type fakeDirectory struct {
	byHost map[string]tenant.Tenant
	byID   map[tenant.ID]tenant.Tenant
	broken bool // every lookup fails as infrastructure
}

func (d *fakeDirectory) ByHostErr(_ context.Context, host string) (tenant.Tenant, error) {
	if d.broken {
		return tenant.Tenant{}, errInfra
	}
	h, err := domain.NormaliseHost(host)
	if err != nil {
		return tenant.Tenant{}, err
	}
	t, ok := d.byHost[h]
	if !ok {
		return tenant.Tenant{}, store.ErrTenantNotFound
	}
	if !t.Active {
		return tenant.Tenant{}, store.ErrTenantInactive
	}
	return t, nil
}

func (d *fakeDirectory) ByID(_ context.Context, id tenant.ID) (tenant.Tenant, error) {
	if d.broken {
		return tenant.Tenant{}, errInfra
	}
	t, ok := d.byID[id]
	if !ok {
		return tenant.Tenant{}, store.ErrTenantNotFound
	}
	return t, nil // a deactivated commune is still describable — rule 7
}

func sampleDirectory() *fakeDirectory {
	// active HAS a province and merged HAS NONE, deliberately. A fixture where every commune declares
	// one cannot tell "the field is carried" from "the field happens to be filled in everywhere",
	// and "" is the answer the contract says every consumer must handle.
	active := tenant.Tenant{
		ID: tenantActive, Host: "tanphu.vigov.vn", Name: "Phường Tân Phú",
		Province: "Thành phố Đà Nẵng", Active: true,
	}
	merged := tenant.Tenant{ID: tenantMerged, Host: "xacu.vigov.vn", Name: "Xã Cũ", Active: false}
	return &fakeDirectory{
		byHost: map[string]tenant.Tenant{active.Host: active, merged.Host: merged},
		byID:   map[tenant.ID]tenant.Tenant{tenantActive: active, tenantMerged: merged},
	}
}

// start starts the real server behind bufconn and returns two clients: one that goes through
// the grpcx client interceptor (the production path) and one raw, used to send calls the
// interceptor would never allow — that is the only way to test what the SERVER does with bad
// or missing metadata.
func start(t *testing.T, dir svcgrpc.Directory) (platformv1.PlatformServiceClient, platformv1.PlatformServiceClient) {
	t.Helper()
	return startWith(t, svcgrpc.Deps{Dir: dir, Apps: sampleMiniApps(), Profiles: sampleProfiles(), Policies: samplePolicies(),
		Fields: sampleFields()})
}

// startWith is start with every dependency chosen by the test.
func startWith(t *testing.T, d svcgrpc.Deps) (platformv1.PlatformServiceClient, platformv1.PlatformServiceClient) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor()))
	platformv1.RegisterPlatformServiceServer(srv,
		svcgrpc.NewServer(d, slog.New(slog.NewTextHandler(io.Discard, nil))))

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	dial := func(opts ...grpc.DialOption) platformv1.PlatformServiceClient {
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

	return dial(grpc.WithUnaryInterceptor(grpcx.UnaryClientInterceptor())), dial()
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// ---- ResolveHost ----------------------------------------------------------------------

// The exemption has to hold end to end: ResolveHost is what ESTABLISHES the commune, so a
// server interceptor demanding one here would make every edge unable to resolve its Host.
func TestResolveHostWorksWithoutTenantInContext(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	res, err := cli.ResolveHost(testCtx(t), &platformv1.ResolveHostRequest{Host: "TanPhu.ViGov.vn:443"})
	if err != nil {
		t.Fatalf("ResolveHost lỗi: %v", err)
	}
	if got := res.GetTenant().GetId(); got != tenantActive.String() {
		t.Fatalf("id = %q, muốn %q", got, tenantActive)
	}
	if res.GetTenant().GetDisplayName() != "Phường Tân Phú" || !res.GetTenant().GetActive() {
		t.Fatalf("siêu dữ liệu sai: %+v", res.GetTenant())
	}
	// toProto maps FIELD BY FIELD on purpose (ADR 0003) — no reflection, no generic mapper — so
	// every field is one hand-written line and a forgotten line is silent: the commune resolves,
	// nothing errors, and the province is simply "" for every commune in the country.
	if got := res.GetTenant().GetProvince(); got != "Thành phố Đà Nẵng" {
		t.Fatalf("province = %q, muốn %q — toProto đánh rơi một trường",
			got, "Thành phố Đà Nẵng")
	}
}

func TestResolveHostNoMatchReturnsNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	_, err := cli.ResolveHost(testCtx(t), &platformv1.ResolveHostRequest{Host: "khonghe.vigov.vn"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

// A deactivated commune keeps its data and its address (rule 7) but stops serving, and the
// caller must not be able to tell it apart from a Host nobody has claimed — that difference
// discloses which communes exist on the platform.
func TestResolveHostInactiveTenantReturnsNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	_, err := cli.ResolveHost(testCtx(t), &platformv1.ResolveHostRequest{Host: "xacu.vigov.vn"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

// THE ONE THIS FILE EXISTS FOR. A database outage reported as NotFound tells every service
// edge that no commune exists, and sends the operator hunting a DNS problem that is not there.
func TestResolveHostDatabaseDownReturnsInternalNotNotFound(t *testing.T) {
	t.Parallel()

	dir := sampleDirectory()
	dir.broken = true
	cli, _ := start(t, dir)

	_, err := cli.ResolveHost(testCtx(t), &platformv1.ResolveHostRequest{Host: "tanphu.vigov.vn"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
	// And the cause stays on this side of the boundary: an internal message can carry a host,
	// a port or a query fragment, and this response crosses into another service.
	if strings.Contains(err.Error(), "10.0.0.7") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("lỗi nội bộ rò ra ngoài: %v", err)
	}
}

func TestResolveHostEmpty(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	_, err := cli.ResolveHost(testCtx(t), &platformv1.ResolveHostRequest{Host: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

// ---- GetTenant ------------------------------------------------------------------------

// GetTenant is NOT on the exemption list. A call reaching it without a commune in metadata is
// refused by the interceptor before the handler sees it — no default, no guess.
func TestGetTenantWithoutTenantMetadataRefused(t *testing.T) {
	t.Parallel()

	_, raw := start(t, sampleDirectory())
	_, err := raw.GetTenant(testCtx(t), &platformv1.GetTenantRequest{Id: tenantActive.String()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantMultipleMetadataValuesRefused(t *testing.T) {
	t.Parallel()

	_, raw := start(t, sampleDirectory())
	ctx := metadata.AppendToOutgoingContext(testCtx(t),
		grpcx.MetadataTenantKey, tenantActive.String(),
		grpcx.MetadataTenantKey, tenantMerged.String())

	_, err := raw.GetTenant(ctx, &platformv1.GetTenantRequest{Id: tenantActive.String()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantReturnsMetadata(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	ctx := tenant.Into(testCtx(t), tenantActive)

	res, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: tenantActive.String()})
	if err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
	got := res.GetTenant()
	if got.GetId() != tenantActive.String() || got.GetHost() != "tanphu.vigov.vn" ||
		got.GetDisplayName() != "Phường Tân Phú" || !got.GetActive() {
		t.Fatalf("siêu dữ liệu sai: %+v", got)
	}
	// BOTH RPCs go through toProto, and both are asserted: they are two call sites of one
	// mapper, and a mapper that maps field by field (ADR 0003) has one line per field to forget.
	if got.GetProvince() != "Thành phố Đà Nẵng" {
		t.Fatalf("province = %q, muốn %q", got.GetProvince(), "Thành phố Đà Nẵng")
	}
}

// THE admin.vigov.vn ROW: a real commune whose is_primary domain is a platform address. GetTenant
// still describes the commune, but its host is "" — never the reserved address, and never another
// of the commune's hosts picked in its place.
func TestGetTenantReservedPrimaryHostBlanked(t *testing.T) {
	t.Parallel()

	for _, h := range []string{"admin.vigov.vn", "admin-stg.vigov.vn", "identity.api.vigov.vn", "vigov.vn"} {
		dir := sampleDirectory()
		tp := dir.byID[tenantActive]
		tp.Host = h
		dir.byID[tenantActive] = tp
		cli, _ := start(t, dir)

		res, err := cli.GetTenant(tenant.Into(testCtx(t), tenantActive),
			&platformv1.GetTenantRequest{Id: tenantActive.String()})
		if err != nil {
			t.Fatalf("%q: GetTenant lỗi: %v", h, err)
		}
		got := res.GetTenant()
		if got.GetHost() != "" {
			t.Errorf("host = %q, muốn rỗng — tên miền của nền tảng không bao giờ là tên miền của xã", got.GetHost())
		}
		if got.GetId() != tenantActive.String() || got.GetDisplayName() != "Phường Tân Phú" || !got.GetActive() {
			t.Errorf("%q: xã vẫn phải được mô tả đủ, nhận %+v", h, got)
		}
	}
}

// A merged commune must stay describable: rule 7 keeps its data and its codes, and an archival
// record referring to it still has to be able to render a name. Active=false is the answer,
// NotFound is not.
func TestGetTenantInactiveTenantReturnedWithActiveFalse(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	ctx := tenant.Into(testCtx(t), tenantActive)

	res, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: tenantMerged.String()})
	if err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
	if res.GetTenant().GetActive() {
		t.Fatal("xã đã ngừng hoạt động lại báo Active=true")
	}
	// A commune that never declared a province travels as "", which the contract calls a valid
	// answer — not an error, and not something for the server to fill in. This fixture is the one
	// with no province precisely so that case is asserted somewhere.
	if got := res.GetTenant().GetProvince(); got != "" {
		t.Fatalf("province = %q, muốn chuỗi rỗng cho xã chưa khai tỉnh/thành", got)
	}
}

func TestGetTenantIDNotULID(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	ctx := tenant.Into(testCtx(t), tenantActive)

	// An administrative code instead of a ULID: rule 1, invariant 2 forbids a meaningful
	// identifier, and saying so beats a NotFound the caller reads as "commune gone".
	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: "26734"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())
	ctx := tenant.Into(testCtx(t), tenantActive)

	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: "01J8Z4K2R7QG5TMN9WXYB3CDEH"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("mã = %v, muốn NotFound (lỗi: %v)", status.Code(err), err)
	}
}

func TestGetTenantDatabaseDownReturnsInternal(t *testing.T) {
	t.Parallel()

	dir := sampleDirectory()
	dir.broken = true
	cli, _ := start(t, dir)
	ctx := tenant.Into(testCtx(t), tenantActive)

	_, err := cli.GetTenant(ctx, &platformv1.GetTenantRequest{Id: tenantActive.String()})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
}

// The production client path end to end: the commune goes out as metadata and arrives as
// context on the other side, without any business argument naming it.
func TestClientInterceptorCarriesTenantOverRealWire(t *testing.T) {
	t.Parallel()

	cli, _ := start(t, sampleDirectory())

	// No commune in context: the client interceptor refuses before anything is sent.
	if _, err := cli.GetTenant(testCtx(t),
		&platformv1.GetTenantRequest{Id: tenantActive.String()}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}

	// With one, the same call succeeds.
	if _, err := cli.GetTenant(tenant.Into(testCtx(t), tenantActive),
		&platformv1.GetTenantRequest{Id: tenantActive.String()}); err != nil {
		t.Fatalf("GetTenant lỗi: %v", err)
	}
}
