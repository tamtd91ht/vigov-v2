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
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

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

// communesFake is the registry behind the session's commune check. Zero value: every commune
// active. inactive / unknown / err override that; asked records which commune was looked up.
type communesFake struct {
	inactive bool
	unknown  bool
	err      error
	calls    int
	asked    tenant.ID
}

func (c *communesFake) Current(ctx context.Context) (tenant.Tenant, bool, error) {
	c.calls++
	id, ok := tenant.From(ctx)
	if !ok {
		return tenant.Tenant{}, false, tenant.ErrNoTenant
	}
	c.asked = id
	if c.err != nil {
		return tenant.Tenant{}, false, c.err
	}
	if c.unknown {
		return tenant.Tenant{}, false, nil
	}
	return tenant.Tenant{ID: id, Active: !c.inactive}, true, nil
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

// A session with no sid cannot be revoked (rule 5, invariant 4). Serving it would hand the edge a
// session nobody can end, so it is an error and not a quiet negative.
func TestPhienCongDanThieuSidLaLoiChuKhongPhaiKhongCoPhien(t *testing.T) {
	s, nhatKy := may(t, func(d *Deps) {
		d.PhienCongDan = &phienCongDanGia{p: httpx.CitizenSession{CitizenID: idCongDan, TenantID: xaA}, ok: true}
	})

	_, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã lỗi = %v, muốn Internal", status.Code(err))
	}
	ghi := nhatKy.String()
	if !strings.Contains(ghi, "CẢNH BÁO HỢP ĐỒNG") {
		t.Errorf("không có cảnh báo hợp đồng: %q", ghi)
	}
	if strings.Contains(ghi, idCongDan) {
		t.Error("cảnh báo mang định danh công dân — luật 3")
	}
}

// ADR 0045 §Phiên chưa có số: an EMPTY citizen id is a real answer now — a bridge session whose
// phone is not verified yet. It used to be refused here as a contract fault; it must pass through
// unchanged, because core/httpx.XaTuPhien is the ONE wall that refuses it on every route reading
// the citizen's own records. Refusing it here too would make every view-only screen of a freshly
// opened Mini App answer 503.
func TestPhienCongDanChuaCoSoDiQuaNguyenVen(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.PhienCongDan = &phienCongDanGia{p: httpx.CitizenSession{ID: sidCongDan, TenantID: xaA}, ok: true}
	})

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil {
		t.Fatalf("phiên chưa có số bị coi là lỗi: %v", err)
	}
	p := ra.GetSession()
	if p == nil || p.GetSessionId() != sidCongDan || p.GetCitizenId() != "" || p.GetTenantId() != string(xaA) {
		t.Fatalf("phiên trả về sai: %+v", p)
	}
}

// ADR 0080: the Zalo account travels to the edge, because for a session without a verified phone it
// is the ONLY owner a petition route can name. Set → straight through; unset (a paired screen) → ""
// and nothing substituted for it — not the sid, not the citizen id.
func TestCitizenSessionCarriesZaloAccount(t *testing.T) {
	const account = "01JE1CCCCCCCCCCCCCCCCCCCCC"
	for name, c := range map[string]struct {
		session httpx.CitizenSession
		want    string
	}{
		"phone-less, account set":   {httpx.CitizenSession{ID: sidCongDan, TenantID: xaA, ZaloAccountID: account}, account},
		"verified, account set":     {httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan, TenantID: xaA, ZaloAccountID: account}, account},
		"paired screen, no account": {httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan, TenantID: xaA}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := may(t, func(d *Deps) { d.PhienCongDan = &phienCongDanGia{p: c.session, ok: true} })
			ra, err := s.ResolveCitizenSession(context.Background(),
				&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
			if err != nil || ra.GetSession() == nil {
				t.Fatalf("session=%v err=%v", ra.GetSession(), err)
			}
			if got := ra.GetSession().GetZaloAccountId(); got != c.want {
				t.Errorf("zalo_account_id = %q, want %q", got, c.want)
			}
			if got := ra.GetSession().GetCitizenId(); got != c.session.CitizenID {
				t.Errorf("citizen_id = %q, want %q", got, c.session.CitizenID)
			}
		})
	}
}

// Wiring is refused AT CONSTRUCTION, where a human is watching a process fail to start. The case
// lives in server_test.go's TestNewServerTuChoiNoiDayKhongDu beside the other seven, so the list
// of required collaborators is read in one place rather than two.

// ACTIVE ⇒ the session resolves, and the commune asked about is the SESSION's commune — read from
// the registry row this service holds, never from anything the caller sent.
func TestCitizenSessionActiveCommuneResolves(t *testing.T) {
	communes := &communesFake{}
	s, _ := may(t, func(d *Deps) { d.Communes = communes })

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil || ra.GetSession() == nil {
		t.Fatalf("active commune: session=%v err=%v", ra.GetSession(), err)
	}
	if communes.asked != xaA {
		t.Fatalf("asked the registry about %q, want the session's commune %q", communes.asked, xaA)
	}
}

// DEACTIVATED (or no longer known) ⇒ the SAME answer as an expired session: OK, no session, nothing
// logged — byte for byte, so the response never says which commune is inactive (rule 4, forbidden
// #2). This is the finding of 01/10/2026: before it, a deactivated commune's open sessions kept
// working for up to CITIZEN_SESSION_TTL.
func TestCitizenSessionInactiveCommuneIsNoSession(t *testing.T) {
	for name, c := range map[string]*communesFake{
		"deactivated": {inactive: true},
		"unknown":     {unknown: true},
	} {
		t.Run(name, func(t *testing.T) {
			s, nhatKy := may(t, func(d *Deps) { d.Communes = c })
			ra, err := s.ResolveCitizenSession(context.Background(),
				&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
			if err != nil {
				t.Fatalf("an inactive commune must answer like an expired session, not an error: %v", err)
			}
			if ra.GetSession() != nil {
				t.Fatalf("a session of an inactive commune still resolved: %+v", ra.GetSession())
			}
			if !proto.Equal(ra, khongCoPhienCongDan()) {
				t.Fatal("the answer differs from the one every unusable token gets")
			}
			if nhatKy.Len() != 0 {
				t.Errorf("an ordinary negative logged: %q", nhatKy.String())
			}
		})
	}
}

// THE DEACTIVATION TAKES EFFECT WITHIN ONE TTL, through the REAL cache main wires
// (tenant.CachedCommune over the registry). The TTL here is zero — every call asks — which is the
// upper bound's limit case; the TTL window itself is pinned in core/tenant (fake clock).
func TestCitizenSessionDeactivatedMidSessionIsRefused(t *testing.T) {
	registry := &communesFake{}
	s, _ := may(t, func(d *Deps) { d.Communes = tenant.NewCachedCommune(registry, 0) })
	req := &identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia}

	if ra, err := s.ResolveCitizenSession(context.Background(), req); err != nil || ra.GetSession() == nil {
		t.Fatalf("before deactivation: %v, %v", ra, err)
	}
	registry.inactive = true // the operator deactivates the commune; the session row is untouched
	if ra, err := s.ResolveCitizenSession(context.Background(), req); err != nil || ra.GetSession() != nil {
		t.Fatalf("after deactivation: session=%v err=%v — the open session still works", ra.GetSession(), err)
	}
}

// PLATFORM UNREACHABLE ⇒ UNAVAILABLE, never "allow" and never "no session" (fail closed). The caller
// (core/identityclient → core/httpx.CitizenEdge) turns it into 503 and leaves the session alone.
func TestCitizenSessionRegistryDownIsUnavailable(t *testing.T) {
	s, nhatKy := may(t, func(d *Deps) { d.Communes = &communesFake{err: errors.New("platform: connection refused")} })

	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %v, want Unavailable", status.Code(err))
	}
	if ra != nil {
		t.Error("answered a response AND an error — the caller could read it as 'no session'")
	}
	if !strings.Contains(nhatKy.String(), "CẢNH BÁO HẠ TẦNG") {
		t.Errorf("the outage is not said out loud: %q", nhatKy.String())
	}
	for _, leak := range []string{idCongDan, string(xaA), tokenGia} {
		if strings.Contains(nhatKy.String(), leak) {
			t.Errorf("the warning carries an identifier (%q) — rule 3", leak)
		}
	}
}

// The picker state (no commune yet, ADR 0005) is never checked: there is nothing to be inactive, and
// asking the registry about "" would be a lookup with no commune.
func TestCitizenSessionWithoutCommuneSkipsTheCheck(t *testing.T) {
	communes := &communesFake{err: errors.New("must not be asked")}
	s, _ := may(t, func(d *Deps) {
		d.Communes = communes
		d.PhienCongDan = &phienCongDanGia{p: httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan}, ok: true}
	})
	ra, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil || ra.GetSession() == nil {
		t.Fatalf("picker-state session: %v, %v", ra, err)
	}
	if communes.calls != 0 {
		t.Fatal("the registry was asked about a session that has no commune")
	}
}
