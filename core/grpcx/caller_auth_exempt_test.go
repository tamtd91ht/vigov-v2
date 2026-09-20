package grpcx

// IN-PACKAGE ON PURPOSE, and it is the only test here that is.
//
// It iterates methodsWithoutTenant itself rather than a list copied into a test. The defect it
// guards against is not today's code — it is the day somebody adds the second member to that
// map and reasons "this RPC runs before anything is known, so it runs before authentication
// too". A test naming ResolveHost by hand would still pass on that day. This one would not.
//
// The two axes:
//
//	miễn XÃ        an RPC that CANNOT know the commune, because it is what establishes one
//	miễn XÁC THỰC  does not exist, and must never be derived from the line above

import (
	"context"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/secret"
)

// A TEST THAT READS THE SOURCE, and it is here because behaviour cannot answer this question.
//
// Replacing subtle.ConstantTimeCompare with `==` was measured against this whole suite: every
// test stayed green. It cannot be otherwise — a functional test sees the same accept and the
// same refusal either way, and a test that TIMES the comparison is a flaky test that gets
// deleted the first week it fails on a loaded CI machine.
//
// What changes is only observable from outside: `==` on a []byte returns as soon as two bytes
// differ, so the response time answers "how many leading bytes were right". That recovers the
// key one byte at a time instead of by brute force, and this comparison is reachable by
// anything that can open a TCP connection to the port.
//
// So the invariant is pinned structurally. It is a blunt instrument, and it is the only one
// that is not already dead on arrival.
func TestSoSanhKhoaPhaiThoiGianKhongDoi(t *testing.T) {
	t.Parallel()

	ma, err := os.ReadFile("caller_auth.go")
	if err != nil {
		t.Fatalf("không đọc được caller_auth.go: %v", err)
	}
	if !strings.Contains(string(ma), "subtle.ConstantTimeCompare") {
		t.Fatal("caller_auth.go không còn dùng subtle.ConstantTimeCompare — " +
			"một phép so sánh dừng sớm trên khoá là một kênh đo thời gian, và không ca kiểm hành vi nào thấy được")
	}
}

func TestMoiRpcDuocMienXaVanPhaiXacThuc(t *testing.T) {
	t.Parallel()

	khoa := secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT-trong-goi")

	if len(methodsWithoutTenant) == 0 {
		t.Fatal("danh sách miễn xã rỗng — ca kiểm này không còn kiểm gì")
	}

	for method := range methodsWithoutTenant {
		t.Run(method, func(t *testing.T) {
			var chay bool
			_, err := UnaryServerCallerAuth(khoa, nil)(context.Background(), nil,
				&grpc.UnaryServerInfo{FullMethod: method},
				func(context.Context, any) (any, error) { chay = true; return nil, nil })

			if chay {
				t.Fatalf("%s chạy mà không có khoá gọi — miễn XÃ đang bị hiểu thành miễn XÁC THỰC", method)
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("%s: mã lỗi = %v, muốn Unauthenticated", method, status.Code(err))
			}
		})
	}
}
