package grpcx_test

// What these tests are actually defending.
//
// The interceptors here are the only place tenant_id is allowed to leave context.Context, so
// every failure mode is a silent one: a call that travels without a commune, a receiver that
// invents one, or a key renamed out of the header sweep that strips caller-supplied communes.
// None of those produces a stack trace. They produce one commune reading another's data.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// A ULID-shaped identifier. 26 characters, opaque, means nothing — which is the point.
const xaTest = tenant.ID("01J8Z4K2R7QG5TMN9WXYB3CDEF")

// The metadata key must stay inside the prefix pkg/httpx already strips from inbound requests.
// This is the regression test for the comment on MetadataTenantKey: somebody renaming it to
// "tenant-id" for tidiness moves it out of that sweep, and nothing else in the suite notices.
func TestKhoaMetadataNamTrongTamQuetCuaHttpx(t *testing.T) {
	t.Parallel()

	if !strings.HasPrefix(grpcx.MetadataTenantKey, "x-tenant") {
		t.Fatalf("khoá %q nằm ngoài tiền tố httpx.StripTenantHeaders quét",
			grpcx.MetadataTenantKey)
	}

	var thay string
	h := httpx.StripTenantHeaders(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		thay = r.Header.Get(grpcx.MetadataTenantKey)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(grpcx.MetadataTenantKey, xaTest.String())
	h.ServeHTTP(httptest.NewRecorder(), req)

	if thay != "" {
		t.Fatalf("header do client đặt vẫn tới được handler: %q", thay)
	}
}

func TestDanhSachMienLaTuongMinh(t *testing.T) {
	t.Parallel()

	if !grpcx.ExemptFromTenant(grpcx.MethodResolveHost) {
		t.Fatal("ResolveHost phải được miễn: nó chạy trước khi biết xã")
	}
	// Cùng một hình dạng, cho kênh KHÔNG CÓ TÊN MIỀN: Mini App không có Host, nên xã được suy ra
	// TỪ PHIÊN (ADR 0022) — đòi "x-tenant-id" ở đây là vòng lặp "biết xã để tìm ra xã".
	// Người dùng chốt 21/09/2026.
	if !grpcx.ExemptFromTenant(grpcx.MethodResolveCitizenSession) {
		t.Fatal("ResolveCitizenSession phải được miễn: nó CHÍNH LÀ thứ trả lời xã nào cho kênh công dân")
	}
	// Chủ dự án chốt 25/09/2026 (ADR 0045, trả lời CÒN MỞ #1). Tên viết bằng chuỗi ở đây, KHÔNG
	// dùng hằng: đổi chính tả của hằng thì miễn trừ âm thầm rơi khỏi RPC thật và ca này phải đỏ.
	for _, m := range []string{
		"/vigov.identity.v1.CitizenSessionBridgeService/OpenCitizenSession",
		"/vigov.platform.v1.PlatformService/ResolveMiniApp",
	} {
		if !grpcx.ExemptFromTenant(m) {
			t.Fatalf("%s phải được miễn: nó QUYẾT ĐỊNH / TRẢ LỜI xã nào, không thể mang xã lúc gọi", m)
		}
	}
	// Anything not named on the list is not exempt. This is the half of the rule that decays
	// first: an exemption that applies by default applies to every RPC written afterwards.
	//
	// ListTenants and ResolveTenantSuccession are named here ON PURPOSE — in the NOT-exempt
	// list — and the reason is worth the lines. Both answer a question asked BEFORE any commune
	// is known, so both will eventually need the exemption, and the person who implements them
	// will meet InvalidArgument and reach for the one-line fix of naming them above.
	//
	// CALLER AUTHENTICATION HAS NOW LANDED, AND IT IS NOT THE PERMISSION SLIP (ADR 0025;
	// core/grpcx/caller_auth.go). The text here used to read "caller authentication first,
	// exemption second", which invites exactly one misreading: the first half arriving means
	// the second half is due. It does not. One shared key says the caller is inside the
	// cluster and never which service it is, so ListTenants would still hand the whole
	// registry to any holder of that key — and the ULIDs in it prefix every cache key, queue,
	// realtime room and file path in the system (rule 1, invariant 7).
	//
	// Whether these two names belong on the exemption list is a separate question for the user
	// (ADR 0012, decision 1). Until it is asked and answered, this loop is what stops the
	// answer being given by accident.
	//
	// ResolveCitizenSession HAS MOVED OUT OF THIS LOOP — asked and answered 2026-09-21, the user
	// said yes — and the move is recorded rather than quietly made, because the loop's whole
	// value is that leaving it costs a visible diff. It sits with ResolveHost above.
	//
	// THE TWO THAT STAYED, STAYED. One question being answered does not answer the others:
	// ListTenants hands back the whole registry, and the ULIDs in it prefix every cache key,
	// queue, realtime room and file path in the system — a different exposure from a call whose
	// reply is one session's own commune. Their question has not been put to the user.
	for _, m := range []string{
		"/vigov.platform.v1.PlatformService/GetTenant",
		"/vigov.platform.v1.PlatformService/ListTenants",
		"/vigov.platform.v1.PlatformService/ResolveTenantSuccession",
		// GetTenantProfile reads THE COMMUNE IN CONTEXT; exempting it would make it answer for no
		// commune — or for whichever one a later edit reads from the request.
		"/vigov.platform.v1.PlatformService/GetTenantProfile",
		"/vigov.identity.v1.IdentityService/BatchGetStaff",
		"",
		"/vigov.platform.v1.PlatformService/ResolveHostSomethingElse",
	} {
		if grpcx.ExemptFromTenant(m) {
			t.Fatalf("%q được miễn ngoài danh sách trắng", m)
		}
	}
}

// ---- client ---------------------------------------------------------------------------

func TestClientDatDungKhoa(t *testing.T) {
	t.Parallel()

	var goi bool
	var md metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn,
		_ ...grpc.CallOption) error {
		goi = true
		md, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}

	ctx := tenant.Into(context.Background(), xaTest)
	err := grpcx.UnaryClientInterceptor()(ctx, "/vigov.identity.v1.IdentityService/BatchGetStaff",
		nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("interceptor trả lỗi: %v", err)
	}
	if !goi {
		t.Fatal("interceptor không gọi tiếp")
	}
	if got := md.Get(grpcx.MetadataTenantKey); len(got) != 1 || got[0] != xaTest.String() {
		t.Fatalf("metadata %q = %v, muốn đúng một giá trị %q",
			grpcx.MetadataTenantKey, got, xaTest)
	}
}

// No commune in context means the call never leaves the process. Letting it go and relying on
// the receiver to refuse would put a request that can only fail on the network, and point the
// investigation at a remote service that is behaving correctly.
func TestClientKhongCoXaThiLoiTruocKhiGui(t *testing.T) {
	t.Parallel()

	var goi bool
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		goi = true
		return nil
	}

	err := grpcx.UnaryClientInterceptor()(context.Background(),
		"/vigov.identity.v1.IdentityService/BatchGetStaff", nil, nil, nil, invoker)

	if goi {
		t.Fatal("đã gửi một lời gọi thiếu xã ra mạng")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã lỗi = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
	if !strings.Contains(err.Error(), tenant.ErrNoTenant.Error()) {
		t.Fatalf("thông điệp không nói rõ nguyên nhân: %v", err)
	}
}

// The exemption has to be symmetric: if the client refused ResolveHost for want of a commune,
// no edge could ever resolve its Host, and every commune would be unreachable.
func TestClientMienThiGuiDuocKhiKhongCoXa(t *testing.T) {
	t.Parallel()

	var md metadata.MD
	var goi bool
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn,
		_ ...grpc.CallOption) error {
		goi = true
		md, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}

	err := grpcx.UnaryClientInterceptor()(context.Background(), grpcx.MethodResolveHost,
		nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("interceptor trả lỗi trên RPC được miễn: %v", err)
	}
	if !goi {
		t.Fatal("RPC được miễn không được gọi tiếp")
	}
	if got := md.Get(grpcx.MetadataTenantKey); len(got) != 0 {
		t.Fatalf("RPC được miễn vẫn mang khoá xã: %v", got)
	}
}

// ---- server ---------------------------------------------------------------------------

func serverInfo(method string) *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: method}
}

const rpcCanXa = "/vigov.platform.v1.PlatformService/GetTenant"

func TestServerDuaXaVaoContext(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(grpcx.MetadataTenantKey, xaTest.String()))

	var thay tenant.ID
	var co bool
	_, err := grpcx.UnaryServerInterceptor()(ctx, nil, serverInfo(rpcCanXa),
		func(ctx context.Context, _ any) (any, error) {
			thay, co = tenant.From(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor từ chối một lời gọi hợp lệ: %v", err)
	}
	if !co || thay != xaTest {
		t.Fatalf("xã trong context = %q (có=%v), muốn %q", thay, co, xaTest)
	}
}

// Missing, empty, malformed, ambiguous. All four refuse, none is repaired, and none falls back
// to a default — a default here serves one commune's data under another commune's name.
func TestServerTuChoiKhiMetadataKhongDung(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ten string
		ctx context.Context
	}{
		{"không có metadata nào", context.Background()},
		{"có metadata nhưng thiếu khoá", metadata.NewIncomingContext(context.Background(),
			metadata.Pairs("x-request-id", "abc"))},
		{"khoá rỗng", metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(grpcx.MetadataTenantKey, ""))},
		{"khoá chỉ có khoảng trắng", metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(grpcx.MetadataTenantKey, "   "))},
		{"không phải ULID", metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(grpcx.MetadataTenantKey, "tan-phu"))},
		{"nhiều giá trị", metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(
				grpcx.MetadataTenantKey, xaTest.String(),
				grpcx.MetadataTenantKey, "01J8Z4K2R7QG5TMN9WXYB3CDEG"))},
	}

	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			var chay bool
			_, err := grpcx.UnaryServerInterceptor()(tc.ctx, nil, serverInfo(rpcCanXa),
				func(context.Context, any) (any, error) {
					chay = true
					return nil, nil
				})
			if chay {
				t.Fatal("handler đã chạy dù không xác định được xã")
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã lỗi = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
			}
		})
	}
}

func TestServerMienThiChayKhongCanXa(t *testing.T) {
	t.Parallel()

	var chay bool
	var co bool
	_, err := grpcx.UnaryServerInterceptor()(context.Background(), nil,
		serverInfo(grpcx.MethodResolveHost),
		func(ctx context.Context, _ any) (any, error) {
			chay = true
			_, co = tenant.From(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("RPC được miễn bị từ chối: %v", err)
	}
	if !chay {
		t.Fatal("handler của RPC được miễn không chạy")
	}
	// And it runs with NO commune: an exempt handler that silently received one would be a
	// handler that could start depending on it.
	if co {
		t.Fatal("RPC được miễn lại có xã trong context")
	}
}
