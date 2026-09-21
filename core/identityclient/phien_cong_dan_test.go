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
	"github.com/vihat/vigov/core/tenant"
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

// ⚠ THE TEST THIS FILE EXISTS FOR, AND IT ASSERTS A REFUSAL.
//
// The citizen edge has NO commune in context — that is the entire reason ResolveCitizenSession
// exists (ADR 0022) — so grpcx.UnaryClientInterceptor refuses the call before anything leaves the
// process, because the method is not on grpcx.methodsWithoutTenant. Adding it there is a STOP
// CONDITION for the user (ADR 0012, decision 1).
//
// WHEN THAT DECISION IS TAKEN, THIS TEST GOES RED. That is the point: it is the thing that tells
// whoever adds the exemption that the rest of this file was, until that moment, exercising a
// transport that never reached a server. Replace it then with the real shape — the same call from a
// commune-less context REACHING the fake server — and not before.
func TestPhienCongDanBiChanViKhongCoMienXa(t *testing.T) {
	if grpcx.ExemptFromTenant("/vigov.identity.v1.IdentityService/ResolveCitizenSession") {
		t.Fatal("ResolveCitizenSession đã được miễn xã — ĐIỀU KIỆN DỪNG này đã được trả lời ở đâu đó; " +
			"hãy viết lại ca kiểm này thành lời gọi THẬT từ context không có xã")
	}

	srv := &mayChuCongDanGia{}
	cl := moMay(t, srv)

	// A citizen edge's context: no commune, by construction.
	_, _, err := cl.TraCuuPhienCongDan(context.Background(), tokenCongDanGia)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã lỗi = %v, muốn InvalidArgument từ interceptor xã", status.Code(err))
	}
	if srv.goi != 0 {
		t.Fatal("lời gọi vẫn ra tới máy chủ — interceptor xã không nằm trong chuỗi")
	}
}

// ngucCanhCongDan is the context these mapping tests have to borrow: a commune, purely so the
// commune interceptor lets the call through and the MAPPING can be exercised at all. The real
// citizen edge has none — see the test above.
func ngucCanhCongDan() context.Context { return tenant.Into(context.Background(), xaA) }

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
// long as the two ends disagree — and a session with no citizen id isolates nothing.
func TestPhienCongDanThieuDinhDanhLaLoiHopDong(t *testing.T) {
	for ten, p := range map[string]*identityv1.CitizenSessionPrincipal{
		"thiếu sid":      {CitizenId: "01JE1BBBBBBBBBBBBBBBBBBBBB", TenantId: string(xaA)},
		"thiếu công dân": {SessionId: "01JE1AAAAAAAAAAAAAAAAAAAAA", TenantId: string(xaA)},
	} {
		t.Run(ten, func(t *testing.T) {
			cl := moMay(t, &mayChuCongDanGia{tra: p})

			_, co, err := cl.TraCuuPhienCongDan(ngucCanhCongDan(), tokenCongDanGia)
			if err == nil {
				t.Fatal("phiên thiếu định danh được phục vụ như phiên thật")
			}
			if co {
				t.Fatal("có phiên kèm lỗi hợp đồng")
			}
			// The message says WHICH field is missing and never its value: one of them is a citizen
			// identifier (rule 3).
			if strings.Contains(err.Error(), "01JE1BBBBBBBBBBBBBBBBBBBBB") {
				t.Error("thông điệp lỗi mang định danh công dân — luật 3")
			}
		})
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
