package identityclient

// What these tests defend: the MAPPING from a gRPC answer to the three outcomes, and the one place
// the third is deliberately thrown away.
//
// THE TEST THAT MATTERS MOST IS THE ONE THAT ASSERTS SOMETHING DOES NOT WORK.
// TestPhienCongDanBiChanVIKhongCoMienXa pins the fact that this whole path is refused today,
// because the RPC is not on core/grpcx.methodsWithoutTenant. Without it, the day somebody adds the
// exemption nothing tells them these tests were passing over a call that never left the process —
// and a green suite for the wrong reason is the defect this repository has paid for most often.

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
)

const tokenCongDanGia = "token-phien-cong-dan-GIA-KHONG-PHAI-THAT"

// mayChuCongDanGia is identity, absent, answering only the citizen RPC. A fake of its own rather
// than fields bolted onto mayChuGia: the two populations are kept apart in the contract, in the
// store and in the server, and merging them in the tests is where that separation starts to blur.
type mayChuCongDanGia struct {
	identityv1.UnimplementedIdentityServiceServer

	tra *identityv1.CitizenSessionPrincipal
	loi error

	goi     int
	thayReq *identityv1.ResolveCitizenSessionRequest
	thayXa  []string
	thayKey []string
}

func (s *mayChuCongDanGia) ResolveCitizenSession(ctx context.Context, in *identityv1.ResolveCitizenSessionRequest) (
	*identityv1.ResolveCitizenSessionResponse, error) {
	s.goi++
	s.thayReq = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
		s.thayKey = md.Get(grpcx.MetadataCallerKey)
	}
	if s.loi != nil {
		return nil, s.loi
	}
	return &identityv1.ResolveCitizenSessionResponse{Session: s.tra}, nil
}

// ⚠ THE TEST THIS FILE EXISTS FOR — and as of 2026-09-21 it asserts a CALL, not a refusal.
//
// It used to assert the opposite. The citizen edge has NO commune in context — that is the entire
// reason ResolveCitizenSession exists (ADR 0022) — and until the user answered ADR 0012's stop
// condition, grpcx.UnaryClientInterceptor refused the call before anything left the process. The
// refusal was pinned here precisely so that whoever took the decision would be told that every
// other test in this file had, until that moment, been exercising a transport that never reached a
// server. The decision was taken, this went red, and this is the replacement it asked for: the same
// call from a commune-less context REACHING the fake server.
//
// TWO ASSERTIONS, AND THE SECOND IS THE LOAD-BEARING ONE. That the call arrives is the easy half.
// That it arrives carrying NO "x-tenant-id" is the half that would rot silently: an exemption means
// the interceptor stops REQUIRING a commune, and if some later edit made it start SENDING one
// picked up from an ambient context, this RPC would be answering "which commune" for a caller that
// had already named one — rule 1, forbidden #2, arriving through the back door.
func TestPhienCongDanGoiDuocTuNgucCanhKhongCoXa(t *testing.T) {
	if !grpcx.ExemptFromTenant("/vigov.identity.v1.IdentityService/ResolveCitizenSession") {
		t.Fatal("ResolveCitizenSession KHÔNG còn được miễn xã — miễn trừ đã bị gỡ ở đâu đó; " +
			"kênh công dân không gọi được nữa, và mọi ca dưới đây chạy trên một transport không tới máy chủ")
	}

	srv := &mayChuCongDanGia{tra: &identityv1.CitizenSessionPrincipal{
		SessionId: "01JE1AAAAAAAAAAAAAAAAAAAAA",
		CitizenId: "01JE1BBBBBBBBBBBBBBBBBBBBB",
		TenantId:  string(xaA),
	}}
	cl := moMay(t, srv)

	// A citizen edge's context: no commune, by construction.
	p, co, err := cl.TraCuuPhienCongDan(context.Background(), tokenCongDanGia)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !co {
		t.Fatal("phiên hợp lệ mà trả về không có")
	}
	if srv.goi != 1 {
		t.Fatalf("máy chủ nhận %d lời gọi, muốn 1 — lời gọi không rời khỏi tiến trình", srv.goi)
	}
	if len(srv.thayXa) != 0 {
		t.Fatalf("lời gọi mang %q trong metadata xã — RPC này PHÂN GIẢI ra xã, không được NHẬN xã "+
			"(luật 1 cấm #2)", srv.thayXa)
	}
	if string(p.TenantID) != string(xaA) {
		t.Errorf("xã = %q, muốn %q — xã phải tới TỪ PHẢN HỒI", p.TenantID, xaA)
	}
}

// ngucCanhCongDan is the context the mapping tests run on: EMPTY, exactly like the real citizen
// edge (ADR 0022). It used to borrow a commune, purely so the commune interceptor would let the
// call through at all; that borrow stopped being necessary on 2026-09-21 and was removed, because
// a mapping test that runs with a commune in context is a mapping test running on a shape that
// never occurs.
func ngucCanhCongDan() context.Context { return context.Background() }

func TestPhienCongDanDungDuocTraVeDuBaDinhDanh(t *testing.T) {
	srv := &mayChuCongDanGia{tra: &identityv1.CitizenSessionPrincipal{
		SessionId: "01JE1AAAAAAAAAAAAAAAAAAAAA",
		CitizenId: "01JE1BBBBBBBBBBBBBBBBBBBBB",
		TenantId:  string(xaA),
	}}
	cl := moMay(t, srv)

	p, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !co {
		t.Fatal("không có phiên dù máy chủ trả về một phiên")
	}
	if p.ID != "01JE1AAAAAAAAAAAAAAAAAAAAA" || p.CitizenID != "01JE1BBBBBBBBBBBBBBBBBBBBB" {
		t.Errorf("định danh sai: %+v", p)
	}
	if p.TenantID != xaA {
		t.Errorf("TenantID = %q, muốn %q", p.TenantID, xaA)
	}
	// The token crossed VERBATIM. This end does not hold the registry and a caller that pre-checks
	// anything builds a second, weaker copy of the decision this RPC exists to make.
	if srv.thayReq.GetSessionToken() != tokenCongDanGia {
		t.Error("token bị sửa trên đường đi")
	}
	// The caller key is on EVERY call, including one that will one day be exempt from the commune:
	// an RPC excused from carrying a COMMUNE is never thereby excused from proving the CALLER
	// (ADR 0025).
	if len(srv.thayKey) != 1 {
		t.Errorf("khoá gọi trên dây = %v", srv.thayKey)
	}
}

// AN OUTAGE IS NOT "no session". Folded into the quiet answer it tells every citizen of every
// commune that their session ended, through the service that is down.
func TestPhienCongDanLoiGoiKhongPhaiKhongCoPhien(t *testing.T) {
	srv := &mayChuCongDanGia{loi: status.Error(codes.Unavailable, "giả lập: identity đang chết")}
	cl := moMay(t, srv)

	_, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err == nil {
		t.Fatal("lỗi hạ tầng bị nuốt")
	}
	if co {
		t.Fatal("có phiên kèm lỗi")
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("mã gRPC không đi kèm lỗi: %v", status.Code(err))
	}
}

// ABSENT MEANS NO SESSION, and it is an ordinary daily event: no error, nothing logged.
func TestPhienCongDanVangMatLaKhongCoPhien(t *testing.T) {
	cl := moMay(t, &mayChuCongDanGia{})

	p, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if co {
		t.Fatal("máy chủ không trả phiên mà client lại bảo có")
	}
	if p != (httpx.CitizenSession{}) {
		t.Errorf("phiên rỗng lại mang dữ liệu: %+v", p)
	}
}

// A SESSION WITH NO COMMUNE IS PASSED STRAIGHT THROUGH (ADR 0005). Rejecting it would break the
// commune-picker screen, which is called WITH exactly this session; filling anything in would be
// rule 1, forbidden #1.
func TestPhienCongDanChuaChonXaDiThangQua(t *testing.T) {
	cl := moMay(t, &mayChuCongDanGia{tra: &identityv1.CitizenSessionPrincipal{
		SessionId: "01JE1AAAAAAAAAAAAAAAAAAAAA",
		CitizenId: "01JE1BBBBBBBBBBBBBBBBBBBBB",
	}})

	p, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err != nil || !co {
		t.Fatalf("phiên chưa chọn xã bị từ chối: co=%v err=%v", co, err)
	}
	if p.TenantID != "" {
		t.Errorf("TenantID = %q — có gì đó đã điền mặc định vào đường cách ly", p.TenantID)
	}
}

// A CONTRACT FAULT IS AN ERROR, NOT A QUIET NEGATIVE. Served quietly it would be invisible for as
// long as the two ends disagree — and a session with no sid cannot be revoked.
func TestPhienCongDanThieuSidLaLoiHopDong(t *testing.T) {
	cl := moMay(t, &mayChuCongDanGia{tra: &identityv1.CitizenSessionPrincipal{
		CitizenId: "01JE1BBBBBBBBBBBBBBBBBBBBB", TenantId: string(xaA)}})

	_, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err == nil {
		t.Fatal("phiên thiếu sid được phục vụ như phiên thật")
	}
	if co {
		t.Fatal("có phiên kèm lỗi hợp đồng")
	}
	if strings.Contains(err.Error(), "01JE1BBBBBBBBBBBBBBBBBBBBB") {
		t.Error("thông điệp lỗi mang định danh công dân — luật 3")
	}
}

// ADR 0045 §Phiên chưa có số: an EMPTY citizen_id is a real answer — the phone is not verified yet.
// Passed straight through, exactly like an empty tenant_id; httpx.XaTuPhien is what refuses it on
// every route that reads the citizen's own records. Refusing it here would make every view-only
// screen of a freshly opened Mini App look like an identity outage.
func TestPhienCongDanChuaCoSoDiThangQua(t *testing.T) {
	cl := moMay(t, &mayChuCongDanGia{tra: &identityv1.CitizenSessionPrincipal{
		SessionId: "01JE1AAAAAAAAAAAAAAAAAAAAA", TenantId: string(xaA)}})

	p, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
	if err != nil || !co {
		t.Fatalf("phiên chưa có số bị từ chối: co=%v err=%v", co, err)
	}
	if p.CitizenID != "" || p.ID != "01JE1AAAAAAAAAAAAAAAAAAAAA" || p.TenantID != xaA {
		t.Errorf("phiên sai: %+v", p)
	}
}

// A request that can only fail has no business on the network, and the message names the CALLER's
// fault rather than the server's answer.
func TestPhienCongDanTokenRongBiChanTaiCho(t *testing.T) {
	srv := &mayChuCongDanGia{}
	cl := moMay(t, srv)

	if _, _, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), ""); err == nil {
		t.Fatal("token rỗng vẫn được gửi đi")
	}
	if srv.goi != 0 {
		t.Fatal("lời gọi rỗng vẫn ra tới máy chủ")
	}
}

// THE COLLAPSE, AND ITS COST, PINNED. httpx.CitizenSessions has two outcomes, so the adapter has to
// throw the third away — and the only trace left is a log line. If that line ever disappears, an
// identity outage becomes indistinguishable from every citizen's session expiring at once, with
// nothing anywhere to say which happened.
func TestSoPhienCongDanGopLoiThanhKhongCoPhienVaGhiCanhBao(t *testing.T) {
	nhatKy := &strings.Builder{}
	cl := moMay(t, &mayChuCongDanGia{loi: status.Error(codes.Unavailable, "giả lập: identity đang chết")})
	cl.log = slog.New(slog.NewTextHandler(nhatKy, nil))
	so := NewSoPhienCongDan(cl)

	p, co := so.TraCuu(ngucCanhCongDan(), tokenCongDanGia)
	if co {
		t.Fatal("lỗi hạ tầng lại thành phiên dùng được")
	}
	if p != (httpx.CitizenSession{}) {
		t.Errorf("phiên rỗng lại mang dữ liệu: %+v", p)
	}
	ghi := nhatKy.String()
	if !strings.Contains(ghi, "CẢNH BÁO HẠ TẦNG") {
		t.Errorf("mất dấu vết duy nhất của việc gộp: %q", ghi)
	}
	if !strings.Contains(ghi, codes.Unavailable.String()) {
		t.Errorf("cảnh báo không nói lỗi gì: %q", ghi)
	}
	// NEVER THE TOKEN (rule 8): it substitutes for a whole citizen session, and a log is a place
	// from which nothing can be recalled.
	if strings.Contains(ghi, tokenCongDanGia) {
		t.Error("token lọt vào log")
	}
}

// The ordinary negative logs NOTHING. This runs on every citizen request; a line per expired
// session would be the highest-volume log in the system and would carry no information.
func TestSoPhienCongDanKhongGhiLogKhiPhienHetHan(t *testing.T) {
	nhatKy := &strings.Builder{}
	cl := moMay(t, &mayChuCongDanGia{})
	cl.log = slog.New(slog.NewTextHandler(nhatKy, nil))

	if _, co := NewSoPhienCongDan(cl).TraCuu(ngucCanhCongDan(), tokenCongDanGia); co {
		t.Fatal("không có phiên mà lại bảo có")
	}
	if nhatKy.Len() != 0 {
		t.Errorf("phiên hết hạn thường ngày lại ghi log: %q", nhatKy.String())
	}
}

// Nil wiring fails where a human is watching, not on the first citizen request of the day.
func TestSoPhienCongDanTuChoiClientRong(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewSoPhienCongDan nhận nil mà không dừng")
		}
	}()
	_ = NewSoPhienCongDan(nil)
}

// The adapter is what the citizen edge consumes. A drift in either signature stops compiling here
// rather than at the first citizen request.
var _ httpx.CitizenSessions = (*SoPhienCongDan)(nil)
