package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The idle lock (#38) on the gRPC edge. Every staff request to the four other services
// authenticates through ResolveStaffPrincipal, so an idle lock applied only at identity's HTTP edge
// would be one they walk straight past. These tests pin that the same rule holds here.

type idleRevokerFake struct {
	calls []idleRevokeCall
}

type idleRevokeCall struct {
	sid, staffCode, ip string
	admin              bool
}

func (f *idleRevokerFake) RevokeIdle(_ context.Context, sid, staffCode, ip string, _ time.Time, admin bool) error {
	f.calls = append(f.calls, idleRevokeCall{sid, staffCode, ip, admin})
	return nil
}

func resolveIdle(t *testing.T, idle time.Duration, grants []authz.Perm) (*identityv1.ResolveStaffPrincipalResponse, *idleRevokerFake) {
	t.Helper()
	fake := &idleRevokerFake{}
	last := time.Now().UTC().Add(-idle)
	s, _ := may(t, func(d *Deps) {
		d.Phien = &phienGia{p: idstore.Phien{ID: "sid-gia", NguoiDungID: idCanBo,
			HetHanLuc: time.Now().Add(time.Hour), TaoLuc: last.Add(-time.Minute), DungGanNhat: &last}}
		d.Quyen = quyenGia{quyen: grants}
		d.IdleSessions = fake
	})
	res, err := s.ResolveStaffPrincipal(ctxXa(xaA),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: tokenCua(t, xaA, "sid-gia"), ClientIp: "203.0.113.9"})
	if err != nil {
		t.Fatalf("ResolveStaffPrincipal: %v", err)
	}
	return res, fake
}

// #38: "Phiên công dân Mini App KHÔNG áp". A citizen session is resolved from `phien_cong_dan` with
// its own lifetime; the idle lock is never consulted on that path, even while the staff registry
// holds a session idle for hours.
func TestCitizenSessionNotSubjectToIdleLock(t *testing.T) {
	fake := &idleRevokerFake{}
	old := time.Now().UTC().Add(-10 * time.Hour)
	s, _ := may(t, func(d *Deps) {
		d.Phien = &phienGia{p: idstore.Phien{ID: "sid-gia", NguoiDungID: idCanBo,
			HetHanLuc: time.Now().Add(time.Hour), TaoLuc: old, DungGanNhat: &old}}
		d.IdleSessions = fake
	})
	res, err := s.ResolveCitizenSession(context.Background(),
		&identityv1.ResolveCitizenSessionRequest{SessionToken: tokenGia})
	if err != nil || res.GetSession() == nil {
		t.Fatalf("citizen session refused: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatal("the idle lock was applied to a citizen session")
	}
}

func TestGRPCIdleLock(t *testing.T) {
	staff := []authz.Perm{"task.extend"}
	admin := []authz.Perm{"admin.user", "task.extend"}
	for _, c := range []struct {
		name    string
		idle    time.Duration
		grants  []authz.Perm
		revoked bool
	}{
		{"staff 59m ok", 59 * time.Minute, staff, false},
		{"staff 61m revoked", 61 * time.Minute, staff, true},
		{"admin 59m ok", 59 * time.Minute, admin, false},
		{"admin 61m revoked", 61 * time.Minute, admin, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			res, fake := resolveIdle(t, c.idle, c.grants)
			if got := res.GetPrincipal() == nil; got != c.revoked {
				t.Fatalf("no principal = %v, want %v", got, c.revoked)
			}
			if c.revoked != (len(fake.calls) == 1) {
				t.Fatalf("revocations = %+v", fake.calls)
			}
			if c.revoked {
				call := fake.calls[0]
				if call.staffCode != "CB001" || call.sid != "sid-gia" {
					t.Errorf("revoked %+v — want the sid and the BUSINESS code", call)
				}
				// The browser address on this edge is the CALLER'S CLAIM; it must never become the
				// audit trail's "from which IP".
				if call.ip != "" {
					t.Errorf("claimed client_ip %q reached the audit trail", call.ip)
				}
			}
		})
	}
}
