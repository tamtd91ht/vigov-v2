package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ActionSessionIdleExpired is the audit verb for a staff session ended by the idle lock (open
// question #38). Vietnamese snake_case like every other verb in this service's trail (ADR 0011: a
// value, not an identifier). A different verb from `dang_xuat`: the person did not sign out, the
// system ended the session — an inspection asking "who was signed in at the counter at 14:00" has to
// be able to tell the two apart.
const ActionSessionIdleExpired = "het_han_phien_do_khong_dung"

// SessionIdleExpiry revokes a staff session that has gone idle, with its audit entry in the same
// transaction (rule 6, invariant 3).
//
// WHY A USE CASE AND NOT A STORE METHOD: the store does not write audit entries (PhienStore's type
// comment), and the two edges that detect idleness — internal/http.XacThuc and
// internal/grpc.ResolveStaffPrincipal — must end the session the same way. One use case is one way.
type SessionIdleExpiry struct {
	db    *store.DB
	phien *idstore.PhienStore
	log   *slog.Logger
}

func NewSessionIdleExpiry(db *store.DB, phien *idstore.PhienStore, log *slog.Logger) *SessionIdleExpiry {
	if db == nil || phien == nil || log == nil {
		panic("app.NewSessionIdleExpiry: db, phien and log are required")
	}
	return &SessionIdleExpiry{db: db, phien: phien, log: log}
}

// RevokeIdle ends the session sid of the staff member staffCode, judged idle from lastActivity.
//
// THE ACTOR IS THE SYSTEM, not the staff member: nobody chose to end this session, the idle rule did
// (rule 6, invariant 6). The staff code is the SUBJECT — the business code, never the internal id
// (invariant 8). ip is the address the detecting edge observed itself; "" when it observed none (the
// gRPC edge only has a caller's claim, which must never become the trail's "from which IP").
//
// NOTHING IS WRITTEN WHEN THE SESSION WAS ALREADY ENDED by a concurrent request: one revocation, one
// entry. The caller refuses the request either way — the decision is derived from lastActivity, so a
// failed revocation still does not let an idle session through (fail closed).
func (uc *SessionIdleExpiry) RevokeIdle(ctx context.Context, sid, staffCode, ip string,
	lastActivity time.Time, admin bool) error {

	xa := tenant.MustFrom(ctx)
	limit := domain.StaffSessionIdleTimeout
	if admin {
		limit = domain.AdminSessionIdleTimeout
	}
	var revoked bool
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		ok, err := uc.phien.RevokeIdle(ctx, tx, sid, lastActivity)
		if err != nil || !ok {
			return err
		}
		revoked = true
		delta, err := json.Marshal(map[string]any{
			"idle_timeout_minutes": int(limit / time.Minute),
			"last_activity":        lastActivity.UTC().Format(time.RFC3339),
		})
		if err != nil {
			return fmt.Errorf("session idle: delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system", IP: ip},
			Action:  ActionSessionIdleExpired,
			Subject: staffCode,
			Delta:   delta,
		})
	})
	if err != nil {
		return fmt.Errorf("session idle: revoke: %w", err)
	}
	if revoked {
		// The security log (skills/security-logging): event shape, business code. Never the sid — it
		// was a working credential until the commit above.
		uc.log.Info("phiên hết hạn do không dùng",
			"event", "session.idle_expired", "outcome", "revoked",
			"actor", audit.SystemActor, "subject", staffCode, "xa", string(xa), "ip", ip,
			"idle_timeout_minutes", int(limit/time.Minute))
	}
	return nil
}
