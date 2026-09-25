package grpcx

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/secret"
)

// Fake bridge keys; the text says so (rule 8, forbidden #1).
var (
	khoaCauMoi = secret.Secret("khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-moi")
	khoaCauCu  = secret.Secret("khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-cu-")
)

func goiCau(t *testing.T, khoa []secret.Secret, gui ...string) (bool, string, error) {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	ctx := context.Background()
	if len(gui) > 0 {
		cap := make([]string, 0, 2*len(gui))
		for _, g := range gui {
			cap = append(cap, MetadataBridgeKey, g)
		}
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(cap...))
	}
	var chay bool
	_, err := UnaryServerBridgeKey(khoa, log)(ctx, nil,
		&grpc.UnaryServerInfo{FullMethod: MethodOpenCitizenSession},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })
	return chay, buf.String(), err
}

func TestKhoaCauKhongCoThiTuChoi(t *testing.T) {
	chay, _, err := goiCau(t, []secret.Secret{khoaCauMoi})
	if chay || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("không khoá: chạy=%v mã=%v", chay, status.Code(err))
	}
}

func TestKhoaCauSaiThiTuChoiVaKhongLoGiaTri(t *testing.T) {
	sai := "khoa-cau-phien-GIA-SAI-SAI-SAI-SAI-SAI-SAI-SAI"
	chay, log, err := goiCau(t, []secret.Secret{khoaCauMoi}, sai)
	if chay || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("khoá sai: chạy=%v mã=%v", chay, status.Code(err))
	}
	if strings.Contains(log, sai) || strings.Contains(log, string(khoaCauMoi.Lo())) ||
		strings.Contains(err.Error(), sai) {
		t.Fatal("log hoặc lỗi mang giá trị khoá (luật 8)")
	}
	if !strings.Contains(log, "CẢNH BÁO AN NINH") {
		t.Fatal("từ chối mà không có dòng cảnh báo")
	}
}

func TestKhoaCauXoayKhoaNhanCaHai(t *testing.T) {
	// Mid-rotation: both keys configured, vihat-miniapp may be on either.
	ds := []secret.Secret{khoaCauMoi, khoaCauCu}
	for _, k := range ds {
		if chay, _, err := goiCau(t, ds, string(k.Lo())); !chay || err != nil {
			t.Fatalf("khoá trong danh sách bị từ chối: chạy=%v err=%v", chay, err)
		}
	}
	// After rotation: the old key dropped from the list is refused.
	if chay, _, _ := goiCau(t, []secret.Secret{khoaCauMoi}, string(khoaCauCu.Lo())); chay {
		t.Fatal("khoá đã bỏ khỏi danh sách vẫn được nhận")
	}
}

func TestKhoaCauHaiGiaTriThiTuChoiDuMotCaiDung(t *testing.T) {
	if chay, _, _ := goiCau(t, []secret.Secret{khoaCauMoi}, "sai-sai", string(khoaCauMoi.Lo())); chay {
		t.Fatal("gửi hai khoá, một đúng, mà được nhận — mời gửi nhiều để dò")
	}
}

func TestKhoaGoiNoiBoKhongMoCongCau(t *testing.T) {
	// GRPC_CALLER_KEY sent under ITS name, or its value under the bridge name: both refused.
	noiBo := secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT-trong-goi")
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(MetadataCallerKey, string(noiBo.Lo())))
	var chay bool
	_, err := UnaryServerBridgeKey([]secret.Secret{khoaCauMoi}, nil)(ctx, nil,
		&grpc.UnaryServerInfo{FullMethod: MethodOpenCitizenSession},
		func(context.Context, any) (any, error) { chay = true; return nil, nil })
	if chay || status.Code(err) != codes.Unauthenticated {
		t.Fatal("khoá gọi nội bộ mở được cổng cầu")
	}
}

func TestKhoaCauRongHoacNganThiDungLucDung(t *testing.T) {
	for ten, ds := range map[string][]secret.Secret{
		"rỗng":  nil,
		"ngắn":  {khoaCauMoi, secret.Secret("ngan")},
		"trống": {secret.Secret("")},
	} {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("dựng được interceptor cổng cầu với khoá không dùng được")
				}
			}()
			UnaryServerBridgeKey(ds, nil)
		})
	}
}

func TestSoSanhKhoaCauPhaiThoiGianKhongDoi(t *testing.T) {
	// Same structural pin as TestSoSanhKhoaPhaiThoiGianKhongDoi, for the same reason.
	ma, err := os.ReadFile("bridge_auth.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ma), "subtle.ConstantTimeCompare") {
		t.Fatal("bridge_auth.go không còn so khoá bằng subtle.ConstantTimeCompare")
	}
}
