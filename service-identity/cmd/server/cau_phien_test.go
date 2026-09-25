package main

// THE BRIDGE PORT'S WIRING (ADR 0045 §Tin cậy), over a real connection with the real chain — for the
// reason the note at the top of main_test.go gives. core/grpcx proves UnaryServerBridgeKey refuses;
// only this file can see whether dungCongCau installs it, and whether each port serves exactly
// what it should: the bridge key opens ONE RPC because ONE service is registered on its listener.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-identity/internal/grpc"
)

// Fake bridge keys (rule 8, forbidden #1).
var (
	khoaCauGia   = secret.Secret("khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-moi")
	khoaCauCuGia = secret.Secret("khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-cu-")
)

type moPhienGia struct{ goi int }

func (m *moPhienGia) Mo(context.Context, app.YeuCauMoPhienCau) (app.KetQuaMoPhienCau, error) {
	m.goi++
	return app.KetQuaMoPhienCau{CheDo: domain.CheDoAppChinh}, nil
}

func noiBufconn(t *testing.T, srv *grpc.Server) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func moCongCau(t *testing.T, khoa ...secret.Secret) (*grpc.ClientConn, *moPhienGia) {
	t.Helper()
	uc := &moPhienGia{}
	srv := dungCongCau(khoa, svcgrpc.NewCauServer(uc, nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return noiBufconn(t, srv), uc
}

func voiKhoaCau(k secret.Secret) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcx.MetadataBridgeKey, string(k.Lo()))
}

var yeuCauCauThu = &identityv1.OpenCitizenSessionRequest{AppId: "a", ZaloUserId: "z"}

func TestCongCauKhongKhoaThiTuChoi(t *testing.T) {
	conn, uc := moCongCau(t, khoaCauGia)
	cl := identityv1.NewCitizenSessionBridgeServiceClient(conn)
	if _, err := cl.OpenCitizenSession(context.Background(), yeuCauCauThu); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("không khoá: mã = %v, muốn Unauthenticated", status.Code(err))
	}
	if uc.goi != 0 {
		t.Fatal("use case chạy khi không có khoá cầu")
	}
}

func TestCongCauKhoaSaiVaKhoaGoiNoiBoDeuBiTuChoi(t *testing.T) {
	conn, uc := moCongCau(t, khoaCauGia)
	cl := identityv1.NewCitizenSessionBridgeServiceClient(conn)

	if _, err := cl.OpenCitizenSession(voiKhoaCau(secret.Secret("khoa-cau-phien-GIA-SAI-SAI-SAI-SAI-SAI-SAI")),
		yeuCauCauThu); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("khoá sai: mã = %v", status.Code(err))
	}
	// GRPC_CALLER_KEY, the way a ViGov service would send it: not a key for this port.
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcx.MetadataCallerKey, string(khoaGoiGia.Lo()))
	if _, err := cl.OpenCitizenSession(ctx, yeuCauCauThu); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("khoá gọi nội bộ trên cổng cầu: mã = %v", status.Code(err))
	}
	if uc.goi != 0 {
		t.Fatal("use case chạy với khoá không hợp lệ")
	}
}

func TestCongCauXoayKhoaNhanCaKhoaMoiLanKhoaCu(t *testing.T) {
	conn, uc := moCongCau(t, khoaCauGia, khoaCauCuGia)
	cl := identityv1.NewCitizenSessionBridgeServiceClient(conn)
	for _, k := range []secret.Secret{khoaCauGia, khoaCauCuGia} {
		// No commune attached, on purpose: OpenCitizenSession is exempt — success proves it.
		if _, err := cl.OpenCitizenSession(voiKhoaCau(k), yeuCauCauThu); err != nil {
			t.Fatalf("khoá trong danh sách bị từ chối: %v", err)
		}
	}
	if uc.goi != 2 {
		t.Fatalf("use case chạy %d lần, muốn 2", uc.goi)
	}
}

func TestCongCauChiPhucVuMotDichVu(t *testing.T) {
	// The bridge key must open NOTHING of IdentityService — not ResolveStaffPrincipal, not the
	// citizen session lookup. Unimplemented, because the service is simply not on this listener.
	conn, _ := moCongCau(t, khoaCauGia)
	cl := identityv1.NewIdentityServiceClient(conn)
	if _, err := cl.ResolveStaffPrincipal(voiKhoaCau(khoaCauGia),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: "x"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("ResolveStaffPrincipal trên cổng cầu: mã = %v, muốn Unimplemented", status.Code(err))
	}
	if _, err := cl.ResolveCitizenSession(voiKhoaCau(khoaCauGia),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: "x"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("ResolveCitizenSession trên cổng cầu: mã = %v, muốn Unimplemented", status.Code(err))
	}
}

func TestCongNoiBoKhongPhucVuCauPhien(t *testing.T) {
	// The other half: holding GRPC_CALLER_KEY does not open the bridge RPC on the inter-service port.
	lis := bufconn.Listen(1 << 20)
	srv := dungGRPCServer(khoaGoiGia, noiDayGia(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_, err = identityv1.NewCitizenSessionBridgeServiceClient(conn).OpenCitizenSession(context.Background(), yeuCauCauThu)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("OpenCitizenSession trên cổng nội bộ: mã = %v, muốn Unimplemented", status.Code(err))
	}
}

func TestCongCauKhongKhoaThiKhongDungDuoc(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được cổng cầu không có khoá cầu")
		}
	}()
	_ = dungCongCau(nil, svcgrpc.NewCauServer(&moPhienGia{}, nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
}
