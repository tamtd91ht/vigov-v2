package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// "Tắt / Bật lại" of a reworded report sentence (ADR 0079 Q2, migration 0004), over the same real
// transaction as system_message_test.go.

func (f *overrideFake) SetActive(_ context.Context, tx *store.ScopedTx, id string, active bool, by string, at time.Time) error {
	f.seen(tx)
	f.switches++
	f.lastBy, f.lastAt = by, at
	for _, o := range f.live[tx.TenantID()] {
		if o.ID == id {
			o.Inactive = !active
			o.UpdatedAt, o.UpdatedBy = at, by
		}
	}
	return nil
}

func TestSwitchedOffReportWordingResolvesToDefault(t *testing.T) {
	m, _ := domain.LookupShippedMessage(titleKey)
	d, f := &recordingDriver{}, newOverrideFake()
	f.put(communeA, domain.MessageOverride{ID: "o", Key: titleKey, Text: "Báo cáo của xã.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"})
	ctx := tenant.Into(context.Background(), communeA)

	got, err := newMessagesUseCase(t, d, f).SetActive(ctx, titleKey, false, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.CurrentText != m.DefaultText || got.OverrideText != "Báo cáo của xã." || got.Group != domain.GroupReport {
		t.Errorf("after Tắt: %+v", got)
	}
	if f.switches != 1 || d.begins != 1 || d.commits != 1 || len(f.txsSeen) != 1 {
		t.Errorf("switches=%d begin=%d commit=%d", f.switches, d.begins, d.commits)
	}
	stmt, delta := auditEntry(t, d)
	if stmt.args[4] != ActionSwitchOffSystemMessage || stmt.args[5] != titleKey {
		t.Errorf("action/subject = %v/%v", stmt.args[4], stmt.args[5])
	}
	if delta["truoc"]["is_active"] != true || delta["sau"]["is_active"] != false {
		t.Errorf("delta = %v", delta)
	}

	// Every reader goes through ResolveMessage: the list now prints the software's title.
	list, _ := newMessagesUseCase(t, &recordingDriver{}, f).Messages(ctx)
	if list[0].Key != titleKey || list[0].CurrentText != m.DefaultText || list[0].Active {
		t.Errorf("list while off: %+v", list[0])
	}

	// Bật lại: same words back.
	d2 := &recordingDriver{}
	got, err = newMessagesUseCase(t, d2, f).SetActive(ctx, titleKey, true, staff)
	if err != nil || !got.Active || got.CurrentText != "Báo cáo của xã." {
		t.Fatalf("Bật lại: %+v, %v", got, err)
	}
	if stmt, _ := auditEntry(t, d2); stmt.args[4] != ActionSwitchOnSystemMessage {
		t.Errorf("action = %v", stmt.args[4])
	}
}

func TestSwitchRefusals(t *testing.T) {
	ctx := tenant.Into(context.Background(), communeA)
	for name, c := range map[string]struct {
		key   string
		actor audit.Actor
		want  error
		begin int
	}{
		"unknown key": {"budget.scope_notice", staff, domain.ErrUnknownMessageKey, 0},
		"no actor":    {titleKey, audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing, 0},
	} {
		t.Run(name, func(t *testing.T) {
			d, f := &recordingDriver{}, newOverrideFake()
			if _, err := newMessagesUseCase(t, d, f).SetActive(ctx, c.key, false, c.actor); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if d.begins != c.begin || d.commits != 0 || f.switches != 0 {
				t.Errorf("begin=%d commit=%d switches=%d", d.begins, d.commits, f.switches)
			}
		})
	}
}

// EVERY SHIPPED KEY HAS A SWITCH (user decision 09/10/2026, migration 0005). "Tắt" of a key the commune
// never reworded stores a row with NO wording, switched off, audited in the same transaction; the list
// shows the default with Active false; "Bật" soft deletes that row.
func TestSwitchOffUnwordedReportSentence(t *testing.T) {
	m, _ := domain.LookupShippedMessage(titleKey)
	d, f := &recordingDriver{}, newOverrideFake()
	ctx := tenant.Into(context.Background(), communeA)

	got, err := newMessagesUseCase(t, d, f).SetActive(ctx, titleKey, false, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.Overridden || got.CurrentText != m.DefaultText || got.UpdatedBy != staff.ID {
		t.Errorf("after Tắt: %+v", got)
	}
	row := f.live[communeA][titleKey]
	if f.adds != 1 || row == nil || row.Text != "" || !row.Inactive || d.begins != 1 || d.commits != 1 || len(f.txsSeen) != 1 {
		t.Fatalf("adds=%d row=%+v begin=%d commit=%d — want one unworded off row in one transaction", f.adds, row, d.begins, d.commits)
	}
	stmt, delta := auditEntry(t, d)
	if stmt.args[4] != ActionSwitchOffSystemMessage || delta["sau"]["is_active"] != false || delta["sau"]["overridden"] != false {
		t.Errorf("audit %v %v", stmt.args[4], delta)
	}
	list, _ := newMessagesUseCase(t, &recordingDriver{}, f).Messages(ctx)
	if list[0].Key != titleKey || list[0].Active || list[0].Overridden || list[0].CurrentText != m.DefaultText {
		t.Errorf("list while off: %+v", list[0])
	}

	d2 := &recordingDriver{}
	got, err = newMessagesUseCase(t, d2, f).SetActive(ctx, titleKey, true, staff)
	if err != nil || !got.Active || got.Overridden {
		t.Fatalf("Bật: %+v, %v", got, err)
	}
	if f.deletes != 1 || f.lastReason != SwitchOnReason || f.switches != 0 || f.live[communeA][titleKey] != nil {
		t.Errorf("deletes=%d reason=%q switches=%d", f.deletes, f.lastReason, f.switches)
	}
	if stmt, _ := auditEntry(t, d2); stmt.args[4] != ActionSwitchOnSystemMessage {
		t.Errorf("action = %v", stmt.args[4])
	}
}

func TestSwitchSameStateWritesNothing(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	f.put(communeA, domain.MessageOverride{ID: "o", Key: titleKey, Text: "X.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"})
	if _, err := newMessagesUseCase(t, d, f).SetActive(tenant.Into(context.Background(), communeA), titleKey, true, staff); err != nil {
		t.Fatal(err)
	}
	if f.switches != 0 || len(d.matching("audit_log")) != 0 {
		t.Error("switching on an override already on wrote something")
	}
}

func TestRewordKeepsTheSwitchOff(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	f.put(communeA, domain.MessageOverride{ID: "o", Key: titleKey, Text: "Cũ.", UpdatedAt: fixedNow, UpdatedBy: "CB-1", Inactive: true})
	got, err := newMessagesUseCase(t, d, f).Reword(tenant.Into(context.Background(), communeA), titleKey, "Mới.", staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.OverrideText != "Mới." || !f.live[communeA][titleKey].Inactive {
		t.Errorf("reworded while off: %+v", got)
	}
}
