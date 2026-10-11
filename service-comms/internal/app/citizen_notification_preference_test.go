package app

// The citizen switch over the REAL CitizenNotificationPreferenceStore and a fake driver: the row and
// its audit entry share one transaction, the commune is $1 from the context, the citizen is the
// SESSION's (the actor), no row reads as enabled, a no-op writes nothing, and a non-citizen actor is
// refused before anything is opened.

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const citizenUnderTest = "01JCITIZENUNDERTEST0000001"

var citizenActorUnderTest = audit.Actor{ID: citizenUnderTest, Kind: "citizen", IP: "203.0.113.7"}

// prefAnswer answers every preference read with the stored value, or no row when stored is nil.
func prefAnswer(stored *bool) func(string) ([]string, [][]driver.Value) {
	return func(q string) ([]string, [][]driver.Value) {
		if !strings.Contains(q, "citizen_notification_preferences") || stored == nil {
			return []string{"zalo_enabled"}, nil
		}
		return []string{"zalo_enabled"}, [][]driver.Value{{*stored}}
	}
}

func newPreferences(t *testing.T, d *adrDB) (*CitizenNotificationPreferences, context.Context) {
	t.Helper()
	h := d.handle(t)
	return NewCitizenNotificationPreferences(h, commsstore.NewCitizenNotificationPreferenceStore(h)),
		tenant.Into(context.Background(), xaA)
}

func TestPreferenceNoRowReadsEnabled(t *testing.T) {
	d := &adrDB{answer: prefAnswer(nil)}
	uc, ctx := newPreferences(t, d)
	p, err := uc.Get(ctx, citizenActorUnderTest)
	if err != nil || !p.ZaloEnabled || p.Stored {
		t.Fatalf("no row must read as enabled, not stored: %+v %v", p, err)
	}
	q := d.with("citizen_notification_preferences")[0]
	if q.args[0] != string(xaA) || q.args[1] != citizenUnderTest {
		t.Errorf("read args = %v — $1 the commune from ctx, $2 the session's citizen", q.args)
	}
}

func TestPreferenceTurnOffWritesRowAndEntryInOneTransaction(t *testing.T) {
	d := &adrDB{answer: prefAnswer(nil)}
	uc, ctx := newPreferences(t, d)
	p, err := uc.Set(ctx, false, citizenActorUnderTest)
	if err != nil || p.ZaloEnabled {
		t.Fatalf("Set: %+v %v", p, err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx = %d/%d/%d, want 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	up := d.with("INSERT INTO citizen_notification_preferences")
	if len(up) != 1 || up[0].args[0] != string(xaA) || up[0].args[1] != citizenUnderTest || up[0].args[2] != false {
		t.Fatalf("upsert = %v", up)
	}
	if d.indexOf("INSERT INTO audit_log") < d.indexOf("INSERT INTO citizen_notification_preferences") {
		t.Fatal("the entry must follow the write inside its transaction")
	}
	a, delta := d.auditEntry(t)
	if a.args[1] != citizenUnderTest || a.args[2] != "citizen" || a.args[3] != "203.0.113.7" ||
		a.args[4] != ActionSetCitizenNotificationPreference {
		t.Errorf("audit actor/kind/ip/action = %v", a.args[1:5])
	}
	// No row before: `truoc` null, effective value "on".
	if delta["truoc"] != nil || delta["truoc_hieu_luc"] != true || delta["sau"] != false {
		t.Errorf("delta = %v", delta)
	}
}

func TestPreferenceNoOpWritesNothing(t *testing.T) {
	off := false
	d := &adrDB{answer: prefAnswer(&off)}
	uc, ctx := newPreferences(t, d)
	if _, err := uc.Set(ctx, false, citizenActorUnderTest); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(d.with("INSERT INTO")) != 0 {
		t.Error("the same value twice must write and audit nothing")
	}
}

func TestPreferenceAuditFailureRollsBack(t *testing.T) {
	d := &adrDB{answer: prefAnswer(nil), failOn: "INSERT INTO audit_log"}
	uc, ctx := newPreferences(t, d)
	if _, err := uc.Set(ctx, false, citizenActorUnderTest); err == nil {
		t.Fatal("an audit failure must fail the write")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("tx committed/rolled back = %d/%d, want 0/1", d.committed, d.rolledBack)
	}
}

func TestPreferenceRefusesANonCitizenActor(t *testing.T) {
	for _, a := range []audit.Actor{
		{ID: "CB-00123", Kind: "staff"},
		{ID: "01JZALOACCOUNT000000000001", Kind: audit.KindZaloAccount},
		{ID: "", Kind: "citizen"},
		{ID: "has space", Kind: "citizen"},
	} {
		d := &adrDB{answer: prefAnswer(nil)}
		uc, ctx := newPreferences(t, d)
		if _, err := uc.Set(ctx, false, a); !errors.Is(err, ErrNotACitizen) {
			t.Errorf("%+v: err = %v", a, err)
		}
		if _, err := uc.Get(ctx, a); !errors.Is(err, ErrNotACitizen) {
			t.Errorf("%+v: get err = %v", a, err)
		}
		if d.begun != 0 || len(d.stmts) != 0 {
			t.Errorf("%+v: something was opened", a)
		}
	}
}
