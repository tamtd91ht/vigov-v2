package grpc

// CitizenEdgeSessions is identity's own citizen edge reading sessions through ResolveCitizenSession
// in-process. What is pinned: the THREE outcomes reach CitizenEdge apart (session · no session · error),
// and the deactivated-commune check — the RPC's — applies here too, because it is the same method.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/vihat/vigov/core/httpx"
)

func edgeSessions(reg PhienCongDanDoc, c *communesFake) *CitizenEdgeSessions {
	return NewCitizenEdgeSessions(reg, c, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCitizenEdgeSessionsUsableSessionPassesStraightThrough(t *testing.T) {
	reg := &phienCongDanGia{p: httpx.CitizenSession{ID: sidCongDan, CitizenID: idCongDan, TenantID: xaA,
		ZaloAccountID: "01JE1CCCCCCCCCCCCCCCCCCCCC"}, ok: true}
	got, ok, err := edgeSessions(reg, &communesFake{}).TraCuu(context.Background(), tokenGia)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got != reg.p {
		t.Fatalf("session = %+v, want %+v", got, reg.p)
	}
	if reg.thayTok != tokenGia {
		t.Error("the registry was not asked with the bearer token")
	}
}

func TestCitizenEdgeSessionsUnusableTokenIsNoSession(t *testing.T) {
	_, ok, err := edgeSessions(&phienCongDanGia{}, &communesFake{}).TraCuu(context.Background(), tokenGia)
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v, want no session and no error", ok, err)
	}
}

// The RPC's check, reached through the edge: a deactivated commune's session is no session.
func TestCitizenEdgeSessionsInactiveCommuneIsNoSession(t *testing.T) {
	_, ok, err := edgeSessions(phienCongDanTot(), &communesFake{inactive: true}).TraCuu(context.Background(), tokenGia)
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v — a deactivated commune's session still resolves on identity's edge", ok, err)
	}
}

// FAIL CLOSED: the registry or the commune check unreachable is an ERROR (CitizenEdge → 503), never
// "no session" (which would sign the citizen out) and never a session.
func TestCitizenEdgeSessionsOutageIsAnError(t *testing.T) {
	for name, s := range map[string]*CitizenEdgeSessions{
		"registry": edgeSessions(&phienCongDanGia{err: errors.New("db down")}, &communesFake{}),
		"commune":  edgeSessions(phienCongDanTot(), &communesFake{err: errors.New("platform down")}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok, err := s.TraCuu(context.Background(), tokenGia); err == nil || ok {
				t.Fatalf("ok=%v err=%v, want an error", ok, err)
			}
		})
	}
}

func TestCitizenEdgeSessionsSessionWithoutSidIsAnError(t *testing.T) {
	reg := &phienCongDanGia{p: httpx.CitizenSession{CitizenID: idCongDan, TenantID: xaA}, ok: true}
	if _, ok, err := edgeSessions(reg, &communesFake{}).TraCuu(context.Background(), tokenGia); err == nil || ok {
		t.Fatalf("ok=%v err=%v — a session nobody can revoke was served", ok, err)
	}
}
