package identityclient

// What these tests defend: THE MAPPING from a gRPC answer to the three outcomes staffauth.Resolver
// declares. This is the only place in the system that can see a status code, so it is the only
// place the distinction "the call did not happen" versus "the credential is not usable" can be got
// wrong — and getting it wrong in the quiet direction turns an outage of identity into every
// member of staff being signed out.
//
// It speaks to a REAL gRPC server over a real connection, with the real interceptor chain, because
// half of what is under test lives in that chain: the caller key and the commune both have to be on
// the wire, and a client assembled by hand in a test would prove nothing about what Dial builds.

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
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// khoaGoiGia is fake key material — the text says so in full (rule 8, forbidden #1).
var khoaGoiGia = secret.Secret("khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT")

const phieuGia = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

var xaA = tenant.ID("01JA" + strings.Repeat("A", 22))

// mayChuGia is identity, absent. It records what actually arrived on the wire, which is how the
// metadata assertions below are made without reaching into the interceptor.
type mayChuGia struct {
	identityv1.UnimplementedIdentityServiceServer

	tra *identityv1.StaffPrincipal
	loi error

	goi     int
	thayXa  []string
	thayKey []string
	thayReq *identityv1.ResolveStaffPrincipalRequest

	traLich  []*identityv1.WorkingHoursReached
	loiLich  error
	goiLich  int
	thayLich *identityv1.AdvanceWorkingHoursRequest
}

func (s *mayChuGia) ResolveStaffPrincipal(ctx context.Context, in *identityv1.ResolveStaffPrincipalRequest) (
	*identityv1.ResolveStaffPrincipalResponse, error) {
	s.goi++
	s.thayReq = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
		s.thayKey = md.Get(grpcx.MetadataCallerKey)
	}
	if s.loi != nil {
		return nil, s.loi
	}
	return &identityv1.ResolveStaffPrincipalResponse{Principal: s.tra}, nil
}

// moMay starts the real server on an in-memory connection and returns a Client built exactly the
// way Dial builds one — same interceptors, same order.
//
// IT TAKES THE SERVER INTERFACE, NOT *mayChuGia, so a second fake can be exercised through the same
// real interceptor chain without a second copy of this function — see phien_cong_dan_test.go. Every
// existing call site passes *mayChuGia unchanged.
func moMay(t *testing.T, srv identityv1.IdentityServiceServer) *Client {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcx.UnaryServerCallerAuth(khoaGoiGia, slog.New(slog.NewTextHandler(io.Discard, nil))),
		grpcx.UnaryServerInterceptor(),
	))
	identityv1.RegisterIdentityServiceServer(gs, srv)
	go func() {
		if err := gs.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithChainUnaryInterceptor(
			grpcx.UnaryClientCallerAuth(khoaGoiGia),
			grpcx.UnaryClientInterceptor(),
		))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return New(identityv1.NewIdentityServiceClient(conn), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// ngucCanh is the context the middleware would have: a commune already resolved from Host.
func ngucCanh() context.Context { return tenant.Into(context.Background(), xaA) }

func TestChuTheCoQuyenTraVeDayDu(t *testing.T) {
	srv := &mayChuGia{tra: &identityv1.StaffPrincipal{
		StaffId:        "nd-01JINTERNALIDCUACANBO",
		PermissionKeys: []string{"document.read", "task.extend"},
	}}
	cl := moMay(t, srv)

	p, co, err := cl.ResolveStaff(ngucCanh(), phieuGia, "10.0.0.7")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !co {
		t.Fatal("không có chủ thể dù máy chủ trả về một chủ thể")
	}
	if p.StaffID != "nd-01JINTERNALIDCUACANBO" {
		t.Errorf("StaffID = %q", p.StaffID)
	}
	if len(p.PermissionKeys) != 2 || p.PermissionKeys[0] != authz.Perm("document.read") {
		t.Errorf("PermissionKeys = %v", p.PermissionKeys)
	}

	// The credential crossed VERBATIM — not parsed, not trimmed, not re-encoded. This end does not
	// hold the signing key, and a caller that pre-checks anything builds a second, weaker copy of
	// the decision this RPC exists to make.
	if srv.thayReq.GetSessionToken() != phieuGia {
		t.Error("phiếu phiên bị sửa trên đường đi")
	}
	if srv.thayReq.GetClientIp() != "10.0.0.7" {
		t.Errorf("client_ip = %q", srv.thayReq.GetClientIp())
	}

	// BOTH metadata keys were on the wire. The commune is what the server compares against the
	// commune inside the credential — rule 1, invariant 8, made once for all four services — so an
	// exemption added to grpcx for this RPC would leave that comparison with nothing to compare.
	if len(srv.thayXa) != 1 || srv.thayXa[0] != xaA.String() {
		t.Errorf("metadata %q = %v, muốn đúng xã phân giải từ Host", grpcx.MetadataTenantKey, srv.thayXa)
	}
	if len(srv.thayKey) != 1 {
		t.Errorf("metadata %q = %d giá trị, muốn 1", grpcx.MetadataCallerKey, len(srv.thayKey))
	}
}

func TestChuTheKhongCoQuyenNaoVanLaChuThe(t *testing.T) {
	// PRESENT WITH AN EMPTY KEY SET IS NOT "no principal", and the contract says so in as many
	// words. It is a live session held by somebody whose role was withdrawn: AnyAuthenticated
	// serves them, RequirePermission refuses them. Folding it into the branch below would sign
	// that person out instead of telling them they may not do this one thing.
	srv := &mayChuGia{tra: &identityv1.StaffPrincipal{StaffId: "nd-01JINTERNALIDCUACANBO"}}
	cl := moMay(t, srv)

	p, co, err := cl.ResolveStaff(ngucCanh(), phieuGia, "")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !co {
		t.Fatal("chủ thể không giữ quyền nào bị đọc thành 'không có chủ thể'")
	}
	if len(p.PermissionKeys) != 0 {
		t.Errorf("PermissionKeys = %v, muốn rỗng", p.PermissionKeys)
	}
}

func TestChuTheVangMatLaKhongCoChuThe(t *testing.T) {
	// OK with no principal: unknown, malformed, expired, revoked, locked, deleted, or issued for
	// another commune — the contract deliberately does not tell them apart, so a probe cannot learn
	// how close it is. No error, and nothing logged: a stale cookie is a daily event.
	cl := moMay(t, &mayChuGia{tra: nil})

	_, co, err := cl.ResolveStaff(ngucCanh(), phieuGia, "")
	if err != nil {
		t.Fatalf("chủ thể vắng mặt bị đọc thành lỗi: %v", err)
	}
	if co {
		t.Error("báo có chủ thể khi phản hồi không mang chủ thể nào")
	}
}

func TestGoiHongLaLOI_KhongPhaiKhongCoChuThe(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Every one of these means the call did not happen, and every one of
	// them must reach the middleware as an error so it answers 503. Read as "no principal", each
	// would tell every member of staff to sign in again — through the service that is down.
	for _, tc := range []struct {
		ten string
		loi error
	}{
		{"UNAVAILABLE", status.Error(codes.Unavailable, "connection refused")},
		{"DEADLINE_EXCEEDED", status.Error(codes.DeadlineExceeded, "quá hạn")},
		{"INTERNAL", status.Error(codes.Internal, "hỏng")},
		// A wiring fault, not a user's stale session. It answers 503 like the rest — stated in
		// ResolveStaff as a cost of the uniform answer, with the code in the log line.
		{"INVALID_ARGUMENT", status.Error(codes.InvalidArgument, "session_token rỗng")},
		// The caller key is wrong or missing: the DEPLOYMENT is misconfigured, every call failing.
		// Never a stale login.
		{"UNAUTHENTICATED", status.Error(codes.Unauthenticated, "unauthenticated")},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			cl := moMay(t, &mayChuGia{loi: tc.loi})

			_, co, err := cl.ResolveStaff(ngucCanh(), phieuGia, "")
			if err == nil {
				t.Fatalf("%s bị nuốt thành câu trả lời bình thường — middleware sẽ trả 401 thay vì 503", tc.ten)
			}
			if co {
				t.Error("báo có chủ thể khi lời gọi hỏng")
			}
		})
	}
}

func TestLoiTraVeKhongMangPhieuPhien(t *testing.T) {
	// The error is logged by the middleware and may be logged again by whoever wraps it. A
	// credential inside it is a credential in the log pipeline, from where it cannot be recalled
	// (rule 3, rule 8).
	cl := moMay(t, &mayChuGia{loi: status.Error(codes.Unavailable, "connection refused")})

	_, _, err := cl.ResolveStaff(ngucCanh(), phieuGia, "10.0.0.7")
	if err == nil {
		t.Fatal("muốn lỗi")
	}
	if strings.Contains(err.Error(), phieuGia) {
		t.Errorf("lỗi chứa phiếu phiên: %v", err)
	}
}

func TestChuTheCoMatNhungKhongCoIdLaLoiHopDong(t *testing.T) {
	// A principal with no id would make identity's own permission query match no row: every check
	// false, every guarded route 403, and nothing in the response, the logs or a test to point at
	// the cause. It is a contract fault, so it must NOT be served as an ordinary expired session —
	// that would hide it for as long as the two ends disagree.
	cl := moMay(t, &mayChuGia{tra: &identityv1.StaffPrincipal{PermissionKeys: []string{"document.read"}}})

	_, co, err := cl.ResolveStaff(ngucCanh(), phieuGia, "")
	if err == nil {
		t.Fatal("chủ thể không có staff_id được nhận như bình thường")
	}
	if co {
		t.Error("báo có chủ thể khi staff_id rỗng")
	}
}

func TestPhieuRongThiKhongRaKhoiTienTrinh(t *testing.T) {
	// The middleware never calls without a credential. Reaching here is a wiring fault in some
	// other caller, and a request that can only fail has no business on the network — the count
	// below is the assertion.
	srv := &mayChuGia{tra: &identityv1.StaffPrincipal{StaffId: "nd-01J"}}
	cl := moMay(t, srv)

	if _, _, err := cl.ResolveStaff(ngucCanh(), "", "10.0.0.7"); err == nil {
		t.Fatal("phiếu rỗng được gửi đi thay vì bị từ chối tại chỗ")
	}
	if srv.goi != 0 {
		t.Errorf("máy chủ nhận %d lời gọi với phiếu rỗng", srv.goi)
	}
}

func TestKhongCoXaTrongContextThiKhongGoiDuoc(t *testing.T) {
	// The RPC is NOT on grpcx's tenant-exemption list and must never be added to it. Without a
	// commune the client interceptor refuses before anything leaves the process — which is what
	// mounting the middleware outside httpx.TenantMiddleware would produce, and why that case
	// panics loudly instead.
	srv := &mayChuGia{tra: &identityv1.StaffPrincipal{StaffId: "nd-01J"}}
	cl := moMay(t, srv)

	if _, _, err := cl.ResolveStaff(context.Background(), phieuGia, ""); err == nil {
		t.Fatal("gọi được khi context không có xã")
	}
	if srv.goi != 0 {
		t.Errorf("máy chủ nhận %d lời gọi không mang xã", srv.goi)
	}
}

func TestDialThieuDiaChiThiTuChoiVaGoiTenBien(t *testing.T) {
	// Fail closed and BY NAME. A client with no address resolves no session, so every staff request
	// answers 503 — a failure that reads as "identity is down" and sends somebody to inspect a
	// service that is running perfectly well.
	_, err := Dial("", khoaGoiGia, nil)
	if err == nil {
		t.Fatal("Dial(\"\") thành công")
	}
	if !strings.Contains(err.Error(), "IDENTITY_GRPC_ADDR") {
		t.Errorf("thông báo không gọi tên biến còn thiếu: %v", err)
	}
}

func TestDialThieuKhoaGoiThiPanic(t *testing.T) {
	// At construction, and for the reason grpcx states: this end would otherwise send every call
	// without a key and have every one refused, and "503 on every staff request" is a much slower
	// read than a startup message naming the variable.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Dial dựng được client với GRPC_CALLER_KEY rỗng")
		}
	}()
	_, _ = Dial("identity:9090", secret.Secret(""), nil)
}

// ---- TienGioLamViec ------------------------------------------------------------------
//
// What these tests defend is NOT "does it call the RPC". It is the three ways a caller ends up
// with a WRONG DEADLINE while every happy-path test stays green — and a wrong deadline is a
// commitment a public authority made to a citizen and will miss.

func (s *mayChuGia) AdvanceWorkingHours(ctx context.Context, in *identityv1.AdvanceWorkingHoursRequest) (
	*identityv1.AdvanceWorkingHoursResponse, error) {
	s.goiLich++
	s.thayLich = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
		s.thayKey = md.Get(grpcx.MetadataCallerKey)
	}
	if s.loiLich != nil {
		return nil, s.loiLich
	}
	return &identityv1.AdvanceWorkingHoursResponse{Items: s.traLich}, nil
}

func moc(gio uint32, t time.Time) *identityv1.WorkingHoursReached {
	return &identityv1.WorkingHoursReached{WorkingHours: gio, ReachedAt: timestamppb.New(t)}
}

var nhanLuc = time.Date(2026, 9, 21, 9, 30, 0, 0, time.UTC)

func TestHaiHanTraVeTraCUUTHEOSOGIO_KhongTheoThuTu(t *testing.T) {
	// THE CASE THAT CATCHES AN INDEX-BASED READ. The server answers in the REVERSE order of the
	// request on purpose: nothing in the contract promises order, and a caller that took items[0]
	// as "the first amount I asked for" would here swap the acknowledge deadline with the resolve
	// deadline — a petition due in 2 hours would be recorded as due in 16, on the security field.
	hanTiepNhan := nhanLuc.Add(2 * time.Hour)
	hanXuLy := nhanLuc.Add(48 * time.Hour)
	srv := &mayChuGia{traLich: []*identityv1.WorkingHoursReached{moc(16, hanXuLy), moc(2, hanTiepNhan)}}
	c := moMay(t, srv)

	got, err := c.TienGioLamViec(ngucCanh(), nhanLuc, []uint32{2, 16})
	if err != nil {
		t.Fatalf("TienGioLamViec: %v", err)
	}
	if !got[2].Equal(hanTiepNhan) {
		t.Errorf("2 giờ: got %v, want %v", got[2], hanTiepNhan)
	}
	if !got[16].Equal(hanXuLy) {
		t.Errorf("16 giờ: got %v, want %v", got[16], hanXuLy)
	}
	if srv.goiLich != 1 {
		t.Errorf("phải gọi ĐÚNG MỘT lần cho cả hai hạn, gọi %d lần", srv.goiLich)
	}
}

func TestThieuMotMocLaLOI_KhongPhaiThanhCongMotPhan(t *testing.T) {
	// The server answers only one of the two amounts. Without this check the missing one reaches
	// the caller as the zero time.Time — a deadline in year 1 — and the petition is overdue the
	// instant it is received, on a call that returned no error.
	srv := &mayChuGia{traLich: []*identityv1.WorkingHoursReached{moc(2, nhanLuc.Add(2*time.Hour))}}
	c := moMay(t, srv)

	_, err := c.TienGioLamViec(ngucCanh(), nhanLuc, []uint32{2, 16})
	if err == nil {
		t.Fatal("thiếu một mốc phải là LỖI")
	}
	if !strings.Contains(err.Error(), "16") {
		t.Errorf("lỗi phải nói rõ THIẾU MỐC NÀO, got: %v", err)
	}
}

func TestLichRongLaLOI_KhongBaoGioLaKhongCoHan(t *testing.T) {
	// FAILED_PRECONDITION is today's ordinary answer: no commune has a calendar yet. It must reach
	// the caller as an error so the intake fails — a caller that read it as "no deadline needed"
	// would store a petition with no commitment at all, and nothing on any screen would show it.
	srv := &mayChuGia{loiLich: status.Error(codes.FailedPrecondition, "xã chưa khai lịch làm việc")}
	c := moMay(t, srv)

	if _, err := c.TienGioLamViec(ngucCanh(), nhanLuc, []uint32{8}); err == nil {
		t.Fatal("lịch rỗng phải là LỖI, không phải map rỗng")
	}
}

func TestMocKhoiDongRongThiKhongRaKhoiTienTrinh(t *testing.T) {
	// A zero instant is a wiring fault in the caller. Sent, it comes back as a horizon failure
	// about the year 1, and an operator reads that as a fault in the commune's calendar.
	srv := &mayChuGia{}
	c := moMay(t, srv)

	_, err := c.TienGioLamViec(ngucCanh(), time.Time{}, []uint32{8})
	if err == nil {
		t.Fatal("mốc khởi động rỗng phải bị từ chối")
	}
	if srv.goiLich != 0 {
		t.Errorf("một yêu cầu chỉ có thể hỏng thì không được lên dây: gọi %d lần", srv.goiLich)
	}
}

func TestGoiLichMangXaVaKhoaGoi(t *testing.T) {
	// The calendar belongs to ONE commune (rule 1). If the tenant never reaches the wire, identity
	// answers for whichever commune it guesses — and every deadline in the system is that guess.
	srv := &mayChuGia{traLich: []*identityv1.WorkingHoursReached{moc(8, nhanLuc.Add(8*time.Hour))}}
	c := moMay(t, srv)

	if _, err := c.TienGioLamViec(ngucCanh(), nhanLuc, []uint32{8}); err != nil {
		t.Fatalf("TienGioLamViec: %v", err)
	}
	if len(srv.thayXa) != 1 || srv.thayXa[0] != string(xaA) {
		t.Errorf("xã trên dây: got %v, want %v", srv.thayXa, xaA)
	}
	if len(srv.thayKey) != 1 {
		t.Errorf("khoá gọi nội bộ phải có trên dây, got %v", srv.thayKey)
	}
}
