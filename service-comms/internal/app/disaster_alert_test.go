package app

// Disaster alerts through the REAL DisasterAlertStore over the fake driver (adrDB): one transaction
// for each write and its entry, the commune is $1 from the context, created_by / ended_by / deleted_by
// are the BUSINESS code, phones and contact names are MASKED in the trail, an ended alert is not edited,
// a no-op writes nothing, a refusal opens nothing. What PostgreSQL decides (the CHECKs, the guard
// trigger) is internal/store's pg suite.

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const pinnedAlertID = "01JDISASTERALERTPINNED0000"

var (
	alertStaff = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
	alertT0    = time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)
)

var alertCols = []string{"id", "kind", "title", "level", "headline", "detail", "effective_from", "effective_to",
	"ended_at", "ended_by", "danger_zone", "evacuation_points", "emergency_contacts",
	"created_at", "created_by", "updated_at", "updated_by"}

// alertRowValues is one stored row; ended != zero makes it an ended alert.
func alertRowValues(ended time.Time) []driver.Value {
	var endedAt, endedBy driver.Value
	if !ended.IsZero() {
		endedAt, endedBy = ended, "CB-00999"
	}
	return []driver.Value{pinnedAlertID, "lu", "Lũ", nil, nil, "Sơ tán", alertT0, nil, endedAt, endedBy,
		[]byte(`[{"x":1,"y":1},{"x":50,"y":1},{"x":20,"y":40}]`),
		[]byte(`[{"name":"Trường","phone":"0900000000"}]`),
		[]byte(`[{"role":"Trực ban","name":"Nguyễn Văn A","phone":"0900000000"}]`),
		alertT0, "CB-00001", alertT0, "CB-00001"}
}

func alertDB(stored []driver.Value) *adrDB {
	return &adrDB{answer: func(q string) ([]string, [][]driver.Value) {
		if stored == nil || !strings.Contains(q, "FROM disaster_alerts") {
			return alertCols, nil
		}
		return alertCols, [][]driver.Value{stored}
	}}
}

func newAlertUseCase(t *testing.T, d *adrDB) (*DisasterAlerts, context.Context) {
	t.Helper()
	h := d.handle(t)
	uc := NewDisasterAlerts(h, commsstore.NewDisasterAlertStore(h))
	uc.newID = func() (string, error) { return pinnedAlertID, nil }
	uc.now = func() time.Time { return alertT0.Add(time.Hour) }
	return uc, tenant.Into(context.Background(), xaA)
}

func validAlertInput() DisasterAlertInput {
	return DisasterAlertInput{
		Kind: "lu", Title: "Lũ", Detail: "Sơ tán", EffectiveFrom: alertT0,
		DangerZone:        []domain.DangerZonePoint{{X: 1, Y: 1}, {X: 50, Y: 1}, {X: 20, Y: 40}},
		EvacuationPoints:  []domain.EvacuationPoint{{Name: "Trường", Phone: "0900000000"}},
		EmergencyContacts: []domain.EmergencyContact{{Role: "Trực ban", Name: "Nguyễn Văn A", Phone: "0900000000"}},
	}
}

// assertNoRawAlertPII: the full number and the full contact name never enter the trail.
func assertNoRawAlertPII(t *testing.T, e adrStmt) {
	t.Helper()
	raw, _ := e.args[7].([]byte)
	for _, banned := range []string{"0900000000", "Nguyễn Văn A", "Văn A"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("audit delta holds %q raw: %s", banned, raw)
		}
	}
}

func TestCreateDisasterAlertWritesRowAndEntryInOneTransaction(t *testing.T) {
	d := alertDB(nil)
	uc, ctx := newAlertUseCase(t, d)
	got, err := uc.Create(ctx, validAlertInput(), alertStaff)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx = %d/%d/%d", d.begun, d.committed, d.rolledBack)
	}
	ins := d.with("INSERT INTO disaster_alerts")
	if len(ins) != 1 || ins[0].args[0] != string(xaA) || ins[0].args[1] != pinnedAlertID || ins[0].args[2] != "lu" ||
		ins[0].args[12] != "CB-00123" {
		t.Fatalf("INSERT args = %v", ins)
	}
	if ins[0].args[8] != nil {
		t.Errorf("an open-ended alert binds effective_to NULL, got %v", ins[0].args[8])
	}
	if d.indexOf("INSERT INTO audit_log") < d.indexOf("INSERT INTO disaster_alerts") {
		t.Fatal("the entry must follow the write inside its transaction")
	}
	e, delta := d.auditEntry(t)
	if e.args[1] != "CB-00123" || e.args[4] != ActionCreateDisasterAlert || e.args[5] != "canh-bao-thien-tai/lu/2026-10-11" {
		t.Errorf("audit actor/action/subject = %v/%v/%v", e.args[1], e.args[4], e.args[5])
	}
	after, _ := delta["sau"].(map[string]any)
	contacts, _ := after["emergency_contacts"].([]any)
	if len(contacts) != 1 || contacts[0].(map[string]any)["phone"] != "09****0000" ||
		contacts[0].(map[string]any)["name"] != "Nguyễn V. A." {
		t.Errorf("contacts in the trail = %v", after["emergency_contacts"])
	}
	assertNoRawAlertPII(t, e)
	if got.ID != pinnedAlertID || got.CreatedBy != "CB-00123" {
		t.Errorf("returned = %+v", got)
	}
}

func TestCreateDisasterAlertRefusalOpensNothing(t *testing.T) {
	d := alertDB(nil)
	uc, ctx := newAlertUseCase(t, d)
	in := validAlertInput()
	in.EffectiveTo = in.EffectiveFrom
	if _, err := uc.Create(ctx, in, alertStaff); !errors.Is(err, domain.ErrDisasterWindowInverted) {
		t.Fatalf("err = %v", err)
	}
	if _, err := uc.Create(ctx, validAlertInput(), audit.Actor{Kind: "staff"}); !errors.Is(err, ErrDisasterAlertMissingActor) {
		t.Fatalf("no business code: err = %v", err)
	}
	if d.begun != 0 {
		t.Error("a refusal opened a transaction")
	}
}

func TestCreateDisasterAlertAuditFailureRollsBack(t *testing.T) {
	d := alertDB(nil)
	d.failOn = "INSERT INTO audit_log"
	uc, ctx := newAlertUseCase(t, d)
	if _, err := uc.Create(ctx, validAlertInput(), alertStaff); err == nil {
		t.Fatal("an audit failure must fail the write")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("tx committed/rolled back = %d/%d", d.committed, d.rolledBack)
	}
}

func TestUpdateDisasterAlertAuditsWhatMovedMasked(t *testing.T) {
	d := alertDB(alertRowValues(time.Time{}))
	uc, ctx := newAlertUseCase(t, d)
	newContacts := []domain.EmergencyContact{{Role: "Công an", Name: "Trần Thị B", Phone: "0900000001"}}
	level := "Cấp 2"
	got, err := uc.Update(ctx, pinnedAlertID, DisasterAlertPatch{Level: &level, EmergencyContacts: &newContacts}, alertStaff)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Level != "Cấp 2" || got.EmergencyContacts[0].Role != "Công an" {
		t.Errorf("after = %+v", got)
	}
	up := d.with("UPDATE disaster_alerts")
	if len(up) != 1 || up[0].args[0] != string(xaA) || up[0].args[12] != "CB-00123" ||
		!strings.Contains(up[0].sql, "ended_at IS NULL") {
		t.Fatalf("UPDATE = %v", up)
	}
	e, delta := d.auditEntry(t)
	before, _ := delta["truoc"].(map[string]any)
	after, _ := delta["sau"].(map[string]any)
	if before["level"] != "" || after["level"] != "Cấp 2" {
		t.Errorf("level diff = %v → %v", before["level"], after["level"])
	}
	if _, ok := after["title_changed"]; ok {
		t.Error("an unchanged title reported as changed")
	}
	raw, _ := e.args[7].([]byte)
	for _, banned := range []string{"0900000001", "Trần Thị B", "0900000000"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("trail holds %q raw", banned)
		}
	}
}

func TestUpdateDisasterAlertNoOpWritesNothing(t *testing.T) {
	d := alertDB(alertRowValues(time.Time{}))
	uc, ctx := newAlertUseCase(t, d)
	same := "Lũ"
	if _, err := uc.Update(ctx, pinnedAlertID, DisasterAlertPatch{Title: &same}, alertStaff); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(d.with("UPDATE disaster_alerts")) != 0 || len(d.with("INSERT INTO audit_log")) != 0 {
		t.Error("a no-op wrote or audited")
	}
}

func TestEndedAlertIsNotEditedNorEndedTwice(t *testing.T) {
	d := alertDB(alertRowValues(alertT0.Add(30 * time.Minute)))
	uc, ctx := newAlertUseCase(t, d)
	title := "Sửa"
	_, err := uc.Update(ctx, pinnedAlertID, DisasterAlertPatch{Title: &title}, alertStaff)
	if !errors.Is(err, domain.ErrDisasterEnded) || strings.Contains(err.Error(), string(xaA)) {
		t.Fatalf("edit of an ended alert: %v (the refusal must not carry the commune)", err)
	}
	if _, err := uc.End(ctx, pinnedAlertID, alertStaff); !errors.Is(err, domain.ErrDisasterEnded) {
		t.Fatalf("second end: %v", err)
	}
	if len(d.with("UPDATE disaster_alerts")) != 0 || len(d.with("INSERT INTO audit_log")) != 0 {
		t.Error("a refusal wrote")
	}
}

func TestEndDisasterAlertSignsWithTheBusinessCode(t *testing.T) {
	d := alertDB(alertRowValues(time.Time{}))
	uc, ctx := newAlertUseCase(t, d)
	got, err := uc.End(ctx, pinnedAlertID, alertStaff)
	if err != nil {
		t.Fatalf("End: %v", err)
	}
	up := d.with("UPDATE disaster_alerts SET ended_at = now()")
	if len(up) != 1 || up[0].args[2] != "CB-00123" {
		t.Fatalf("end UPDATE = %v", up)
	}
	e, delta := d.auditEntry(t)
	if e.args[4] != ActionEndDisasterAlert || delta["was_in_force"] != true {
		t.Errorf("entry = %v %v", e.args[4], delta)
	}
	if !got.Ended() || got.EndedBy != "CB-00123" {
		t.Errorf("returned = %+v", got)
	}
}

func TestDeleteDisasterAlertNeedsReasonAndIsSoft(t *testing.T) {
	d := alertDB(alertRowValues(alertT0.Add(time.Minute))) // ended alerts may still be retired
	uc, ctx := newAlertUseCase(t, d)
	if err := uc.Delete(ctx, pinnedAlertID, " ", alertStaff); !errors.Is(err, domain.ErrThieuLyDoXoa) {
		t.Fatalf("no reason: %v", err)
	}
	if err := uc.Delete(ctx, pinnedAlertID, "đăng nhầm xã", alertStaff); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	up := d.with("UPDATE disaster_alerts SET deleted_at = now()")
	if len(up) != 1 || up[0].args[2] != "CB-00123" || up[0].args[3] != "đăng nhầm xã" {
		t.Fatalf("soft delete = %v", up)
	}
	if len(d.with("DELETE")) != 0 {
		t.Error("a hard DELETE was issued")
	}
	e, _ := d.auditEntry(t)
	assertNoRawAlertPII(t, e)
}
