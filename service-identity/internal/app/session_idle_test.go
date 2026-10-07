package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The revocation half of the idle lock (#38): the edges decide "idle", this use case ends the
// session and records it. On the fake driver of dang_nhap_giao_dich_test.go.

func idleHarness(t *testing.T) (*SessionIdleExpiry, *ghiChep, *bytes.Buffer) {
	t.Helper()
	db, g := moDB(t)
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	return NewSessionIdleExpiry(db, idstore.NewPhienStore(db), log), g, &buf
}

func TestIdleRevocationAndItsEntryShareOneTransaction(t *testing.T) {
	uc, g, logBuf := idleHarness(t)
	last := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)

	if err := uc.RevokeIdle(ctxXa(xaThu), "sid-0001", maCanBo, ipGia, last, true); err != nil {
		t.Fatalf("RevokeIdle: %v", err)
	}
	revoke := g.tim("UPDATE phien SET thu_hoi_luc")
	entry := g.tim("INSERT INTO audit_log")
	if revoke == nil || entry == nil {
		t.Fatal("no revocation or no audit entry")
	}
	if revoke.tx == 0 || revoke.tx != entry.tx || g.ketThucCua(revoke.tx) != "commit" {
		t.Fatalf("revocation tx %d, entry tx %d, end %q — must be ONE committed transaction",
			revoke.tx, entry.tx, g.ketThucCua(revoke.tx))
	}
	// Scoped to the commune, the sid, the reason, and the judged last activity.
	if revoke.args[0] != string(xaThu) || revoke.args[1] != "sid-0001" ||
		revoke.args[2] != idstore.IdleRevokeReason || !revoke.args[3].(time.Time).Equal(last) {
		t.Errorf("revocation args %v", revoke.args)
	}
	if !strings.Contains(revoke.sql, "coalesce(dung_gan_nhat, tao_luc) <= $4") {
		t.Error("the revocation is not conditional on the last activity the edge judged")
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if entry.args[1] != audit.SystemActor || entry.args[2] != "system" || entry.args[4] != ActionSessionIdleExpired ||
		entry.args[5] != maCanBo || entry.args[3] != ipGia {
		t.Errorf("entry %v — want SYSTEM actor, the idle verb, the staff CODE as subject", entry.args[:6])
	}
	var delta map[string]any
	if err := json.Unmarshal(entry.args[7].([]byte), &delta); err != nil {
		t.Fatal(err)
	}
	if delta["idle_timeout_minutes"] != float64(60) {
		t.Errorf("an admin session's entry records limit %v, want 60 (#38, changed 2026-10-07)", delta["idle_timeout_minutes"])
	}
	if !strings.Contains(logBuf.String(), "event=session.idle_expired") || strings.Contains(logBuf.String(), "sid-0001") {
		t.Errorf("security log must carry the event and never the sid:\n%s", logBuf.String())
	}
}

func TestIdleStaffLimitRecordedAsSixty(t *testing.T) {
	uc, g, _ := idleHarness(t)
	if err := uc.RevokeIdle(ctxXa(xaThu), "sid-0001", maCanBo, "", time.Now().UTC(), false); err != nil {
		t.Fatal(err)
	}
	var delta map[string]any
	_ = json.Unmarshal(g.tim("INSERT INTO audit_log").args[7].([]byte), &delta)
	if delta["idle_timeout_minutes"] != float64(60) {
		t.Errorf("staff limit recorded %v, want 60 (#38, changed 2026-10-07)", delta["idle_timeout_minutes"])
	}
}

func TestIdleEntryFailureRollsTheRevocationBack(t *testing.T) {
	uc, g, _ := idleHarness(t)
	g.loiTheo["audit_log"] = errors.New("audit write failed")

	if err := uc.RevokeIdle(ctxXa(xaThu), "sid-0001", maCanBo, ipGia, time.Now().UTC(), false); err == nil {
		t.Fatal("audit failed and the revocation reported success")
	}
	revoke := g.tim("UPDATE phien SET thu_hoi_luc")
	if revoke == nil || g.ketThucCua(revoke.tx) != "rollback" {
		t.Fatal("a revocation with no trail was committed")
	}
}
