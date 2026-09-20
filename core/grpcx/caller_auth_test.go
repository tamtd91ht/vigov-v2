package grpcx_test

// What these tests are actually defending.
//
// The interceptors in caller_auth.go are the only thing between the inter-service gRPC port and
// anything that can open a TCP connection to it. Every failure mode here is silent in the same
// way: a server wired without a key, a comparison that leaks the key one byte at a time, or an
// authentication check quietly made conditional on the tenant exemption list. None of the three
// produces a stack trace, an error, or a moved metric. They produce a port that answers.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

// khoaGoiGia is fake key material. The text says so in full: a scanner and a reviewer must both
// be able to tell at a glance that this is not a real key (rule 8, forbidden #1).
// Not a const: secret.Secret is a []byte, and Go has no byte-slice constants.
var khoaGoiGia = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT-cho-test")

const rpcThuong = "/vigov.identity.v1.IdentityService/BatchGetStaff"

// nghe captures the server's own log so a test can assert what it does and does NOT say.
func nghe() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func mdVao(cap ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(cap...))
}

func TestKhoaGoiDungThiChayTiep(t *testing.T) {
	t.Parallel()

	log, _ := nghe()
	var chay bool
	_, err := grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(
		mdVao(grpcx.MetadataCallerKey, string(khoaGoiGia)), nil,
		&grpc.UnaryServerInfo{FullMethod: rpcThuong},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })

	if err != nil {
		t.Fatalf("lời gọi có khoá đúng bị từ chối: %v", err)
	}
	if !chay {
		t.Fatal("handler không chạy dù khoá đúng")
	}
}

// Missing, empty, wrong, ambiguous. All four refuse with the SAME code and the SAME message:
// a response that varies with the reason tells whoever is guessing how close they are.
func TestKhoaGoiSaiThiTuChoi(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ten string
		ctx context.Context
	}{
		{"không có metadata nào", context.Background()},
		{"có metadata nhưng thiếu khoá", mdVao("x-request-id", "abc")},
		{"khoá rỗng", mdVao(grpcx.MetadataCallerKey, "")},
		{"khoá sai", mdVao(grpcx.MetadataCallerKey, "khoa-khac-KHONG-PHAI-KHOA-THAT")},
		{"khoá đúng tiền tố nhưng thiếu đuôi", mdVao(grpcx.MetadataCallerKey,
			string(khoaGoiGia)[:len(khoaGoiGia)-1])},
		{"khoá thừa một ký tự", mdVao(grpcx.MetadataCallerKey, string(khoaGoiGia)+"x")},
		// Two values, one of them correct. Refused rather than searched: accepting a call
		// because ONE of several offered keys matched is an invitation to offer several.
		{"hai giá trị, một đúng", mdVao(
			grpcx.MetadataCallerKey, string(khoaGoiGia),
			grpcx.MetadataCallerKey, "khoa-khac-KHONG-PHAI-KHOA-THAT")},
	}

	var thongDiep string
	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			log, _ := nghe()
			var chay bool
			_, err := grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(tc.ctx, nil,
				&grpc.UnaryServerInfo{FullMethod: rpcThuong},
				func(context.Context, any) (any, error) { chay = true; return nil, nil })

			if chay {
				t.Fatal("handler đã chạy với một lời gọi không xác thực được")
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("mã lỗi = %v, muốn Unauthenticated (lỗi: %v)", status.Code(err), err)
			}
			// Indistinguishable from every other refusal.
			if thongDiep == "" {
				thongDiep = status.Convert(err).Message()
			} else if got := status.Convert(err).Message(); got != thongDiep {
				t.Fatalf("thông điệp khác nhau giữa các ca từ chối: %q vs %q", got, thongDiep)
			}
		})
	}
}

// The value must never reach the caller. An error message naming the expected or the offered
// key hands the deployment's key to whoever asked for it (rule 8, invariant 1).
func TestThongDiepTuChoiKhongMangGiaTriKhoa(t *testing.T) {
	t.Parallel()

	log, _ := nghe()
	saiGia := "khoa-gui-len-KHONG-PHAI-KHOA-THAT"
	_, err := grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(
		mdVao(grpcx.MetadataCallerKey, saiGia), nil,
		&grpc.UnaryServerInfo{FullMethod: rpcThuong},
		func(context.Context, any) (any, error) { return nil, nil })

	for _, cam := range []string{string(khoaGoiGia), saiGia, grpcx.MetadataCallerKey} {
		if strings.Contains(err.Error(), cam) {
			t.Errorf("lỗi trả cho bên gọi để lộ %q: %v", cam, err)
		}
	}
}

// The cause belongs in THIS service's log — and the value does not. A rejected value is usually
// a real key from a stale deployment, and a log line reaches centralised logging, backups and a
// third-party monitoring vendor at once.
func TestLogNoiRoNguyenNhanNhungKhongInKhoa(t *testing.T) {
	t.Parallel()

	log, buf := nghe()
	saiGia := "khoa-gui-len-KHONG-PHAI-KHOA-THAT"
	_, _ = grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(
		mdVao(grpcx.MetadataCallerKey, saiGia), nil,
		&grpc.UnaryServerInfo{FullMethod: rpcThuong},
		func(context.Context, any) (any, error) { return nil, nil })

	ra := buf.String()
	if ra == "" {
		t.Fatal("từ chối một lời gọi mà không ghi gì — sự cố cấu hình sẽ vô hình")
	}
	if !strings.Contains(ra, rpcThuong) {
		t.Errorf("log không nói bị từ chối ở RPC nào: %s", ra)
	}
	for _, cam := range []string{string(khoaGoiGia), saiGia} {
		if strings.Contains(ra, cam) {
			t.Fatalf("KHOÁ LỌT VÀO LOG: %s", ra)
		}
	}
}

// ---- the two axes are orthogonal ---------------------------------------------------------
//
// THE TEST THIS WHOLE FILE EXISTS FOR. ResolveHost is exempt from carrying a COMMUNE because it
// is the call that establishes one. That exemption says nothing about the CALLER, and the day
// methodsWithoutTenant doubles as "RPCs that skip authentication", the one RPC an
// unauthenticated process can reach is the registry lookup.
//
// caller_auth_exempt_test.go iterates the exemption list itself, so a member added later is
// covered without anybody remembering to come back here. This one pins the pair by name.
func TestRpcDuocMienXaVanPhaiCoKhoaGoi(t *testing.T) {
	t.Parallel()

	if !grpcx.ExemptFromTenant(grpcx.MethodResolveHost) {
		t.Fatal("tiền đề của ca kiểm này đã đổi: ResolveHost không còn được miễn xã")
	}

	log, _ := nghe()
	var chay bool
	_, err := grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: grpcx.MethodResolveHost},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })

	if chay {
		t.Fatal("ResolveHost chạy mà không có khoá gọi — miễn XÃ đã bị hiểu thành miễn XÁC THỰC")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("mã lỗi = %v, muốn Unauthenticated (lỗi: %v)", status.Code(err), err)
	}

	// ...and with the key it runs, exactly like any other RPC. Two axes, independent.
	_, err = grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(
		mdVao(grpcx.MetadataCallerKey, string(khoaGoiGia)), nil,
		&grpc.UnaryServerInfo{FullMethod: grpcx.MethodResolveHost},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })
	if err != nil || !chay {
		t.Fatalf("ResolveHost có khoá đúng vẫn bị chặn: %v", err)
	}
}

// ---- client ------------------------------------------------------------------------------

func TestClientGanKhoaVaoMoiLoiGoi(t *testing.T) {
	t.Parallel()

	// Including the RPC exempt from carrying a commune: the client must mirror the server, or
	// one RPC fails in production while the rest work and it reads like a network fault.
	for _, method := range []string{rpcThuong, grpcx.MethodResolveHost} {
		t.Run(method, func(t *testing.T) {
			var md metadata.MD
			invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn,
				_ ...grpc.CallOption) error {
				md, _ = metadata.FromOutgoingContext(ctx)
				return nil
			}

			err := grpcx.UnaryClientCallerAuth(khoaGoiGia)(context.Background(), method,
				nil, nil, nil, invoker)
			if err != nil {
				t.Fatalf("interceptor trả lỗi: %v", err)
			}
			got := md.Get(grpcx.MetadataCallerKey)
			if len(got) != 1 || got[0] != string(khoaGoiGia) {
				t.Fatalf("metadata %q = %v, muốn đúng một giá trị là khoá",
					grpcx.MetadataCallerKey, got)
			}
		})
	}
}

// The two ends have to agree without a running server in between: what the client appends is
// what the server accepts. A test per end would pass with two different key names.
func TestHaiDauKhopNhau(t *testing.T) {
	t.Parallel()

	var raMD metadata.MD
	_ = grpcx.UnaryClientCallerAuth(khoaGoiGia)(context.Background(), rpcThuong, nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			raMD, _ = metadata.FromOutgoingContext(ctx)
			return nil
		})

	log, _ := nghe()
	var chay bool
	_, err := grpcx.UnaryServerCallerAuth(khoaGoiGia, log)(
		metadata.NewIncomingContext(context.Background(), raMD), nil,
		&grpc.UnaryServerInfo{FullMethod: rpcThuong},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })

	if err != nil || !chay {
		t.Fatalf("đầu nhận từ chối đúng thứ đầu gửi gửi đi: %v", err)
	}
}

// ---- refusal at construction ---------------------------------------------------------------

// A server that starts without the key accepts every call it is supposed to refuse, and nothing
// looks wrong — no error, no failed request, no metric moving. The first person to find out
// would be nobody. Same discipline as identity/http.Register.
func TestThieuKhoaThiTuChoiLucDung(t *testing.T) {
	t.Parallel()

	for _, rong := range []secret.Secret{nil, secret.Secret(""), secret.Secret([]byte{})} {
		t.Run("server", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("dựng được interceptor máy chủ với khoá rỗng — cổng gRPC sẽ nhận mọi lời gọi")
				}
			}()
			grpcx.UnaryServerCallerAuth(rong, nil)
		})
		t.Run("client", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("dựng được interceptor máy khách với khoá rỗng")
				}
			}()
			grpcx.UnaryClientCallerAuth(rong)
		})
	}
}

// gRPC lower-cases every metadata key. A constant in any other case compares unequal, and every
// call is refused — loud, but only once it is deployed.
func TestKhoaMetadataVietThuong(t *testing.T) {
	t.Parallel()

	if grpcx.MetadataCallerKey != strings.ToLower(grpcx.MetadataCallerKey) {
		t.Fatalf("khoá %q không viết thường", grpcx.MetadataCallerKey)
	}
	// And it is NOT the commune key: two questions, two keys, and they must be able to change
	// independently of one another.
	if grpcx.MetadataCallerKey == grpcx.MetadataTenantKey {
		t.Fatal("khoá gọi nội bộ trùng khoá xã")
	}
}
