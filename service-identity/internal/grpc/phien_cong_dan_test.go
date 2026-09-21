package grpc

// What these tests defend: the four branches of ResolveCitizenSession, and one property that has
// no branch — that this handler NEVER reads the commune from context.
//
// THE PROPERTY WORTH THE MOST HERE IS THE ONE THAT LOOKS LIKE A DETAIL: a registry OUTAGE must not
// come back as "no session". It is the failure whose symptom is invisible — every citizen of every
// commune told their session ended, while nothing in this service is red — and it is a property of
// this Go code, not of any SQL, so it is checkable on every `go test`.

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The real store satisfies the narrow interface as it is — nothing was shaped to accommodate this
// server. A drift in either signature stops compiling here rather than at the first citizen
// request of the day.
var _ PhienCongDanDoc = (*idstore.PhienCongDanStore)(nil)

const (
	sidCongDan = "01JE1AAAAAAAAAAAAAAAAAAAAA"
	idCongDan  = "01JE1BBBBBBBBBBBBBBBBBBBBB"
	tokenGia   = "token-phien-cong-dan-GIA-KHONG-PHAI-THAT"
)

type phienCongDanGia struct {
	p        httpx.CitizenSession
	ok       bool
	err      error
	soLanGoi int
	thayTok  string
}

func (f *phienCongDanGia) TraCuuCoLoi(_ context.Context, token string) (httpx.CitizenSession, bool, error) {
	f.soLanGoi++
	f.thayTok = token
	return f.p, f.ok, f.err
}

// phienCongDanTot is the default collaborator: one usable session, in a commune.
func phienCongDanTot() *phienCongDanGia {
	return &phienCongDanGia{
		p:  httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan, TenantID: xaA},
		ok: true,
	}
}

// An empty token is a WIRING fault of the caller, answered loudly. Answered quietly it would cost a
// round trip on every anonymous request of the citizen channel, forever, with nothing to report it.
func TestPhienCongDanTokenRongLaInvalidArgument(t *testing.T) {
	s, _ := may(t, nil)

	_, err := s.ResolveCitizenSession(context.Background(), &identityv1.ResolveCitizenSessionRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã lỗi = %v, muốn InvalidArgument", status.Code(err))
	}
}

// THE HANDLER MUST NOT NEED A COMMUNE. It is the call that establishes one (ADR 0022), so it runs
// with none in context — and a background context has none. Calling Server.xa from here would turn
// every success into Internal, which is exactly the edit this test catches.
func TestPhienCongDanChayDuocKhiKhongCoXaTrongContext(t *testing.T) {
	kho := phienCongDanTot()
	s, _ := may(t, func(d *Deps) { d.PhienCongDan = kho })

	if _, co := tenant.From(context.Background()); co {
		t.Fatal("tiền đề hỏng: context nền lại có xã")
	}

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if ra.GetSession().GetTenantId() != string(xaA) {
		t.Errorf("tenant_id = %q, muốn %q", ra.GetSession().GetTenantId(), xaA)
	}
	if ra.GetSession().GetSessionId() != sidCongDan || ra.GetSession().GetCitizenId() != idCongDan {
		t.Errorf("phiên trả về sai: %+v", ra.GetSession())
	}
	// Verbatim. This end does not parse the token, and a handler that trimmed or re-encoded it
	// would build a second, weaker copy of the decision the registry exists to make.
	if kho.thayTok != tokenGia {
		t.Error("token bị sửa trên đường xuống sổ phiên")
	}
}

// THE TEST THIS FILE EXISTS FOR. A registry outage is NOT "no session": answered as one, it tells
// every citizen of every commune that their session ended, through the one channel a commune is
// judged on (rule 10), while nothing in this service turns red.
func TestPhienCongDanLoiKhoLaInternalChuKhongPhaiKhongCoPhien(t *testing.T) {
	s, nhatKy := may(t, func(d *Deps) {
		d.PhienCongDan = &phienCongDanGia{err: loiKho}
	})

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã lỗi = %v, muốn Internal", status.Code(err))
	}
	if ra != nil {
		t.Error("trả về cả phản hồi lẫn lỗi — bên gọi sẽ đọc phản hồi rỗng thành 'không có phiên'")
	}
	// The cause is logged HERE, where the operator is, and never crosses the boundary (rule 3).
	if !strings.Contains(nhatKy.String(), loiKho.Error()) {
		t.Error("nguyên nhân không được ghi ở phía này")
	}
	if strings.Contains(status.Convert(err).Message(), loiKho.Error()) {
		t.Error("nguyên nhân nội bộ rò ra thông điệp gửi qua ranh giới service")
	}
}

// ONE NEGATIVE ANSWER, AND NOTHING LOGGED. Unknown, malformed, expired and revoked are
// indistinguishable, and a log line per failed probe is a log line per probe.
func TestPhienCongDanKhongDungDuocTraVeRongVaKhongGhiLog(t *testing.T) {
	s, nhatKy := may(t, func(d *Deps) { d.PhienCongDan = &phienCongDanGia{} })

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if ra.GetSession() != nil {
		t.Error("token không dùng được mà vẫn trả về phiên")
	}
	if nhatKy.Len() != 0 {
		t.Errorf("phiên hỏng thường ngày lại ghi log: %q", nhatKy.String())
	}
}

// A SESSION WITH NO COMMUNE IS A REAL ANSWER, NOT A FAILURE (ADR 0005): the citizen is signed in
// and has not chosen a commune yet — the state the commune-picker screen is called in. Refusing it
// here would break that screen; filling in a default would be rule 1, forbidden #1.
func TestPhienCongDanChuaChonXaVanLaPhienDungDuoc(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.PhienCongDan = &phienCongDanGia{
			p:  httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan},
			ok: true,
		}
	})

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if ra.GetSession() == nil {
		t.Fatal("phiên chưa chọn xã bị coi là không có phiên")
	}
	if ra.GetSession().GetTenantId() != "" {
		t.Errorf("tenant_id = %q — có gì đó đã điền mặc định vào đường cách ly",
			ra.GetSession().GetTenantId())
	}
}

// A session with no sid cannot be revoked; a session with no citizen id cannot filter a
// citizen-path query (rule 4, invariant 3). Serving either would hand the edge something that looks
// valid and isolates nothing, so it is an error and not a quiet negative.
func TestPhienCongDanThieuDinhDanhLaLoiChuKhongPhaiKhongCoPhien(t *testing.T) {
	for ten, p := range map[string]httpx.CitizenSession{
		"thiếu sid":      {CitizenID: idCongDan, TenantID: xaA},
		"thiếu công dân": {ID: sidCongDan, TenantID: xaA},
	} {
		t.Run(ten, func(t *testing.T) {
			s, nhatKy := may(t, func(d *Deps) {
				d.PhienCongDan = &phienCongDanGia{p: p, ok: true}
			})

			_, err := s.ResolveCitizenSession(context.Background(),
				&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
			if status.Code(err) != codes.Internal {
				t.Fatalf("mã lỗi = %v, muốn Internal", status.Code(err))
			}
			// The alert says WHICH field is missing and never a value: one of them is a citizen
			// identifier (rule 3).
			ghi := nhatKy.String()
			if !strings.Contains(ghi, "CẢNH BÁO HỢP ĐỒNG") {
				t.Errorf("không có cảnh báo hợp đồng: %q", ghi)
			}
			if strings.Contains(ghi, idCongDan) || strings.Contains(ghi, sidCongDan) {
				t.Error("cảnh báo mang định danh — luật 3")
			}
		})
	}
}

// Wiring is refused AT CONSTRUCTION, where a human is watching a process fail to start. The case
// lives in server_test.go's TestNewServerTuChoiNoiDayKhongDu beside the other seven, so the list
// of required collaborators is read in one place rather than two.
