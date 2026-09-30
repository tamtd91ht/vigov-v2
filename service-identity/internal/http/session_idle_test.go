package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The idle lock at identity's HTTP edge — open question #38, decided 2026-09-30: a staff session
// ends after 30 minutes without a request, a session holding `admin.user` after 15. The revocation
// itself (transaction + audit entry) is defended in internal/app/session_idle_test.go; here the
// question is the EDGE: which sessions it refuses, that it asks for the revocation, and that the
// client cannot tell an idle session from an expired one.

// idleRevokerFake records what XacThuc asked to revoke.
type idleRevokerFake struct {
	calls []idleRevokeCall
	err   error
}

type idleRevokeCall struct {
	sid, staffCode, ip string
	lastActivity       time.Time
	admin              bool
}

func (f *idleRevokerFake) RevokeIdle(_ context.Context, sid, staffCode, ip string, lastActivity time.Time, admin bool) error {
	f.calls = append(f.calls, idleRevokeCall{sid, staffCode, ip, lastActivity, admin})
	return f.err
}

// idleFor sets sidA's last activity to `ago` before now.
func idleFor(m *mayChu, ago time.Duration) time.Time {
	last := time.Now().UTC().Add(-ago)
	ph := m.phien.phien[sidA]
	ph.DungGanNhat = &last
	m.phien.phien[sidA] = ph
	return last
}

// withoutAdmin rebuilds the chain with a checker that grants the account NOTHING — an ordinary
// member of staff, for whom the 30-minute limit applies.
func withoutAdmin(t *testing.T, m *mayChu) {
	t.Helper()
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {}}}
	})
}

func TestIdleStaffSession29MinutesStillWorks(t *testing.T) {
	m := dungMayChu(t)
	withoutAdmin(t, m)
	idleFor(m, 29*time.Minute)

	doiMa(t, m.goi(t, "GET", hostA, mauXemPhienHienTai[len("GET "):], "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if len(m.idleSessions.calls) != 0 {
		t.Fatalf("a staff session idle 29 minutes was revoked: %+v", m.idleSessions.calls)
	}
	if m.phien.soLanDung != 1 {
		t.Errorf("last-activity stamp written %d times, want 1 — a live session must be kept alive", m.phien.soLanDung)
	}
}

func TestIdleStaffSession31MinutesRevokedAndAnswersLikeExpired(t *testing.T) {
	m := dungMayChu(t)
	withoutAdmin(t, m)
	last := idleFor(m, 31*time.Minute)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goi(t, "GET", hostA, mauXemPhienHienTai[len("GET "):], "", tok)
	doiMa(t, w, http.StatusUnauthorized)
	if c := cookiePhien(t, w); c == nil || c.MaxAge >= 0 {
		t.Error("idle session refused but its cookie was left in the browser")
	}
	if len(m.idleSessions.calls) != 1 {
		t.Fatalf("revocations requested = %d, want 1 — an idle session must be REVOKED, not only refused", len(m.idleSessions.calls))
	}
	c := m.idleSessions.calls[0]
	if c.sid != sidA || c.staffCode != maCanBo || c.admin || !c.lastActivity.Equal(last) || c.ip != "10.0.0.7" {
		t.Errorf("revocation asked with %+v — want sid, BUSINESS code, admin=false, the judged last activity, the edge's IP", c)
	}
	if m.phien.soLanDung != 0 {
		t.Error("the request that found the session idle refreshed its last activity")
	}

	// THE SAME ANSWER AS AN EXPIRED SESSION — body and status — so nothing tells the two apart.
	expired := dungMayChu(t)
	expired.phien.phien[sidA] = idstore.Phien{ID: sidA, NguoiDungID: idNoiBo, TaoLuc: time.Now().UTC(),
		HetHanLuc: time.Now().UTC().Add(-time.Minute)}
	we := expired.goi(t, "GET", hostA, mauXemPhienHienTai[len("GET "):], "", expired.tokenCho(t, xaA, sidA))
	if we.Code != w.Code || we.Body.String() != w.Body.String() {
		t.Errorf("idle answers %d %q, expired answers %d %q — they must be identical",
			w.Code, w.Body.String(), we.Code, we.Body.String())
	}
}

func TestIdleAdminSession14MinutesStillWorks(t *testing.T) {
	m := dungMayChu(t) // the default checker grants admin.user in commune A
	idleFor(m, 14*time.Minute)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if len(m.idleSessions.calls) != 0 {
		t.Fatalf("an admin session idle 14 minutes was revoked: %+v", m.idleSessions.calls)
	}
}

func TestIdleAdminSession16MinutesRevoked(t *testing.T) {
	m := dungMayChu(t)
	idleFor(m, 16*time.Minute)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
	if len(m.idleSessions.calls) != 1 || !m.idleSessions.calls[0].admin {
		t.Fatalf("revocations = %+v, want one, judged as an ADMIN session", m.idleSessions.calls)
	}
}

func TestIdleNonAdminSession16MinutesStillWorks(t *testing.T) {
	// The control for the test above: the same 16 minutes, the one difference being admin.user.
	m := dungMayChu(t)
	withoutAdmin(t, m)
	idleFor(m, 16*time.Minute)

	doiMa(t, m.goi(t, "GET", hostA, mauXemPhienHienTai[len("GET "):], "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if len(m.idleSessions.calls) != 0 {
		t.Fatalf("a non-admin session idle 16 minutes was revoked: %+v", m.idleSessions.calls)
	}
}

func TestIdleNeverUsedSessionMeasuredFromCreation(t *testing.T) {
	// dung_gan_nhat NULL must not read as "never idle": a session opened and abandoned at a shared
	// counter is exactly the one this lock exists for.
	m := dungMayChu(t)
	ph := m.phien.phien[sidA]
	ph.TaoLuc = time.Now().UTC().Add(-31 * time.Minute)
	ph.DungGanNhat = nil
	m.phien.phien[sidA] = ph

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
	if len(m.idleSessions.calls) != 1 {
		t.Fatal("a never-used session 31 minutes old was not revoked")
	}
}

func TestIdleRevocationFailureStillRefuses(t *testing.T) {
	// Fail closed: the refusal is decided from the timestamp, not from whether the revocation landed.
	m := dungMayChu(t)
	m.idleSessions.err = context.DeadlineExceeded
	idleFor(m, 31*time.Minute)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
}

func TestAbsoluteExpiryStillAppliesToAnActiveSession(t *testing.T) {
	// The idle lock sits UNDER the 12-hour absolute lifetime, never instead of it: a session used a
	// second ago but past het_han_luc is refused by the registry before idleness is even asked.
	m := dungMayChu(t)
	just := time.Now().UTC()
	m.phien.phien[sidA] = idstore.Phien{ID: sidA, NguoiDungID: idNoiBo, TaoLuc: just.Add(-12 * time.Hour),
		DungGanNhat: &just, HetHanLuc: just.Add(-time.Second)}

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
	if len(m.idleSessions.calls) != 0 {
		t.Error("an expired session reached the idle check — expiry must be decided first")
	}
}
