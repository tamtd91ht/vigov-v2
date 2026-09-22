package main

// What this test defends: THE WIRING, not the interceptors.
//
// core/grpcx proves that UnaryServerCallerAuth refuses a call with no key and that
// UnaryServerInterceptor refuses a call with no commune. internal/grpc proves the handlers branch
// correctly. NEITHER of them can see whether THIS binary installs the interceptors — an
// interceptor deleted from the chain in dungGRPCServer leaves a server that starts, serves,
// answers, and hands a staff member's whole grant set to anything that opened a socket to the
// port. Nothing else in this repository turns red for that.
//
// So this test speaks to the real server over a real connection, with the real chain.
//
// THE TWO AXES ARE CHECKED SEPARATELY AND ON PURPOSE. "Who is calling" and "which commune" are
// orthogonal, and a test that only ever dials with both interceptors would stay green with either
// one missing from the server. TestCongGRPCDoiXaDuDaCoKhoa is the half that is easy to leave out:
// it dials WITH the caller key and WITHOUT the tenant interceptor, so the refusal it asserts can
// only come from the server's own chain.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-identity/internal/grpc"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Fake key material — the text says so in full (rule 8, forbidden #1).
var (
	khoaGoiGia = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")
	khoaKyGia  = secret.Secret("KHOA-KY-PHIEN-GIA-KHONG-PHAI-THAT-32B")
)

const (
	ulidThu = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
	idThu   = "01JD9AAAAAAAAAAAAAAAAAAAAA"
)

// The stores, absent. This test is about the chain in FRONT of the handlers, so the handlers only
// have to answer something.
type (
	phienGia        struct{}
	canBoGia        struct{}
	loGia           struct{}
	tenGia          struct{}
	quyenGia        struct{}
	phienCongDanGia struct{}
	lichGia         struct{}
	nghiLeGia       struct{}
	lamBuGia        struct{}
	slaGia          struct{}
)

func (phienGia) KiemTra(context.Context, string) (idstore.Phien, error) {
	return idstore.Phien{ID: "sid-gia", NguoiDungID: idThu,
		HetHanLuc: time.Now().Add(time.Hour)}, nil
}
func (phienGia) GhiNhanDung(context.Context, string) {}

func (canBoGia) TheoID(context.Context, string) (domain.CanBo, error) {
	return domain.CanBo{ID: idThu, Ma: "CB001", CoTaiKhoan: true, DangHoatDong: true}, nil
}

func (loGia) TheoNhieuID(_ context.Context, ids []string) ([]domain.CanBoVaiTro, error) {
	ra := make([]domain.CanBoVaiTro, 0, len(ids))
	for _, id := range ids {
		ra = append(ra, domain.CanBoVaiTro{ID: id, VaiTroMa: "chu-tich-ubnd"})
	}
	return ra, nil
}

// The name read behind ResolveStaffNames. It answers for EVERY code asked, including codes whose
// record would be soft deleted — the predicate itself is defended in internal/store against a real
// PostgreSQL, and what each standing means in internal/grpc. These wiring tests only need the RPC
// to be reachable through the real interceptor chain.
func (tenGia) TenTheoNhieuMa(_ context.Context, ma []string) ([]domain.TenCanBo, error) {
	ra := make([]domain.TenCanBo, 0, len(ma))
	for _, m := range ma {
		ra = append(ra, domain.TenCanBo{Ma: m, HoTen: "Nguyễn Văn A", ConTrongDanhBa: true})
	}
	return ra, nil
}

func (quyenGia) QuyenCua(context.Context, authz.Principal) ([]authz.Perm, error) {
	return []authz.Perm{"admin.user"}, nil
}

// The citizen session registry, absent. It answers a usable session so the interceptor chain is
// what decides the outcome of a call to ResolveCitizenSession — which today is a refusal, because
// that RPC is not on core/grpcx.methodsWithoutTenant.
func (phienCongDanGia) TraCuuCoLoi(context.Context, string) (httpx.CitizenSession, bool, error) {
	return httpx.CitizenSession{ID: "sid-cong-dan-gia", CitizenID: idThu, TenantID: ulidThu}, true, nil
}

// A minimal working week — Monday 07:30–11:30 — and no closures, no swap days. Enough for
// AdvanceWorkingHours to answer at all, which is all these wiring tests need: what the arithmetic
// computes is defended in internal/domain, and which code each fault gets in internal/grpc.
func (lichGia) DanhSach(context.Context) ([]domain.CaLamViec, error) {
	return []domain.CaLamViec{{
		ID: "01JD9BBBBBBBBBBBBBBBBBBBBB", Thu: 1,
		BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60,
	}}, nil
}

func (nghiLeGia) TheoNam(context.Context, int) ([]domain.NgayNghiLe, error) { return nil, nil }
func (lamBuGia) TheoNam(context.Context, int) ([]domain.CaLamBu, error)     { return nil, nil }

// An EMPTY deadline table — the real state of every commune, since migration 0008 seeds nothing.
// These wiring tests only need ResolveDeadlines to be reachable and to refuse for the commune's own
// reason; what each fault answers is defended in internal/grpc.
func (slaGia) DanhSach(context.Context) ([]domain.DongSLA, error) { return nil, nil }

func noiDayGia(t *testing.T) svcgrpc.Deps {
	t.Helper()
	ky, err := token.NewSigner([]secret.Secret{khoaKyGia})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return svcgrpc.Deps{
		Signer: ky,
		Phien:  phienGia{},
		CanBo:  canBoGia{},
		Lo:     loGia{},
		Ten:    tenGia{},
		Quyen:  quyenGia{},
		// Required, or NewServer refuses to build: every OTHER service's citizen edge is built on
		// this one lookup (svcgrpc.Deps.PhienCongDan).
		PhienCongDan: phienCongDanGia{},
		Lich:         lichGia{},
		NghiLe:       nghiLeGia{},
		LamBu:        lamBuGia{},
		SLA:          slaGia{},
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// moMay starts the REAL server on an in-memory connection and returns a client dialled with the
// given interceptors — so a test can choose to dial like a correctly configured service, or like
// something that just opened a socket to the port.
func moMay(t *testing.T, opts ...grpc.DialOption) identityv1.IdentityServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := dungGRPCServer(khoaGoiGia, noiDayGia(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	opts = append(opts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}))
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return identityv1.NewIdentityServiceClient(conn)
}

// A caller that only opened a socket gets nothing — on EVERY RPC.
//
// There is no exemption from caller authentication anywhere, for any method, and this is the
// assertion that keeps it that way at the binary level. The tenant exemption list must never
// double as a list of RPCs that skip authentication: this port now carries live session
// credentials and whole grant sets, which is a different exposure from the platform's registry
// metadata (ADR 0003 does not cover this service).
func TestCongGRPCTuChoiBenGoiKhongCoKhoa(t *testing.T) {
	cl := moMay(t) // no interceptors at all: the naked caller

	// A commune is attached BY HAND so the call is complete in every respect except the key.
	// Without it, a missing commune could be what produced the refusal, and this test would stay
	// green with the caller-key interceptor deleted from the chain.
	//
	// The metadata key comes from grpcx.MetadataTenantKey rather than a literal, so a rename of
	// the constant reaches this test instead of silently making it assert nothing.
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		grpcx.MetadataTenantKey, ulidThu.String())

	if _, err := cl.BatchGetStaff(ctx,
		&identityv1.BatchGetStaffRequest{Ids: []string{idThu}}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("BatchGetStaff không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}

	if _, err := cl.ResolveStaffPrincipal(ctx,
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: "bat-ky"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ResolveStaffPrincipal không khoá: mã = %v, muốn Unauthenticated (lỗi: %v)",
			status.Code(err), err)
	}
}

// THE OTHER AXIS, and the one a test is most likely to miss.
//
// Dialled WITH the caller key and WITHOUT the client-side tenant interceptor, so nothing on the
// client refuses the call locally: it reaches the server, and the InvalidArgument asserted here
// can only have come from grpcx.UnaryServerInterceptor sitting in the chain this binary builds.
// Delete that interceptor from dungGRPCServer and this is what turns red — the handler would run
// with no commune and answer Internal instead.
func TestCongGRPCDoiXaDuDaCoKhoa(t *testing.T) {
	cl := moMay(t, grpc.WithChainUnaryInterceptor(grpcx.UnaryClientCallerAuth(khoaGoiGia)))

	for ten, goi := range map[string]func(context.Context) error{
		"BatchGetStaff": func(ctx context.Context) error {
			_, err := cl.BatchGetStaff(ctx, &identityv1.BatchGetStaffRequest{Ids: []string{idThu}})
			return err
		},
		"ResolveStaffPrincipal": func(ctx context.Context) error {
			_, err := cl.ResolveStaffPrincipal(ctx,
				&identityv1.ResolveStaffPrincipalRequest{SessionToken: "bat-ky"})
			return err
		},
	} {
		t.Run(ten, func(t *testing.T) {
			if err := goi(context.Background()); status.Code(err) != codes.InvalidArgument {
				t.Errorf("thiếu xã: mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
			}
		})
	}
}

// Both conditions met: the chain lets the call through and the handlers answer. A server that
// refused everything would pass both tests above and be just as broken.
func TestCongGRPCChoQuaKhiDungCaHaiDieuKien(t *testing.T) {
	cl := moMay(t, grpc.WithChainUnaryInterceptor(
		grpcx.UnaryClientCallerAuth(khoaGoiGia),
		grpcx.UnaryClientInterceptor(),
	))

	// The commune comes from the CONTEXT, never from the body — the client interceptor lifts it
	// into metadata, and the server interceptor lifts it back into the handler's context.
	ctx := tenant.Into(context.Background(), ulidThu)

	ra, err := cl.BatchGetStaff(ctx, &identityv1.BatchGetStaffRequest{Ids: []string{idThu}})
	if err != nil {
		t.Fatalf("BatchGetStaff với khoá và xã đầy đủ vẫn bị từ chối: %v", err)
	}
	if len(ra.GetItems()) != 1 || ra.GetItems()[0].GetTenantId() != string(ulidThu) {
		t.Errorf("BatchGetStaff trả %+v", ra.GetItems())
	}

	// ResolveStaffPrincipal with a token this signer cannot read: OK with no principal, NOT an
	// error. That is the contract's single answer for every unusable credential, and asserting it
	// here proves the response travels the whole chain intact.
	pr, err := cl.ResolveStaffPrincipal(ctx,
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: "khong-phai-token"})
	if err != nil {
		t.Fatalf("credential không dùng được phải là OK không principal, nhận lỗi: %v", err)
	}
	if pr.GetPrincipal() != nil {
		t.Errorf("token rác vẫn dựng được principal: %+v", pr.GetPrincipal())
	}

	// UNAUTHENTICATED belongs to the caller-key interceptor and means the DEPLOYMENT is
	// misconfigured. This RPC must never produce it, or an operator cannot tell a fleet-wide
	// outage from one staff member's expired login.
	if status.Code(err) == codes.Unauthenticated {
		t.Error("ResolveStaffPrincipal trả Unauthenticated — mã ấy thuộc về interceptor khoá gọi")
	}
}

// A server that starts without the caller key accepts every call it should refuse, and the first
// person to find out would be nobody. Refusing at construction is the only failure anybody sees.
func TestKhongCoKhoaGoiThiKhongDungDuocMayChu(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("dựng được máy chủ gRPC với GRPC_CALLER_KEY rỗng")
		}
	}()
	_ = dungGRPCServer(nil, noiDayGia(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// Incomplete wiring fails at construction too — the same discipline identity/http.Register
// applies, one boundary over. A server registered with a nil store would panic inside a handler,
// in four other services' hot path, long after the deploy that caused it.
func TestNoiDayThieuThiKhongDungDuocMayChu(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("dựng được máy chủ gRPC với Deps rỗng")
		}
	}()
	_ = dungGRPCServer(khoaGoiGia, svcgrpc.Deps{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
