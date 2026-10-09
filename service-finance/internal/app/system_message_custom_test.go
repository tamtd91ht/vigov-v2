package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// "Tắt / Bật lại" and the commune's own sentences (ADR 0079 Q2, Q5a, Q5b; migration 0017), over the
// same real transaction on the recording driver as system_message_test.go. The fake below extends
// overrideFake with the switch and the commune-sentence half.

// --- the fake's other half ------------------------------------------------------------------------

func (f *overrideFake) SetActive(_ context.Context, tx *store.ScopedTx, id string, active bool, by string, at time.Time) error {
	f.seen(tx)
	f.switches++
	f.lastBy, f.lastAt = by, at
	if o := f.live[tx.TenantID()]; o != nil && o.ID == id {
		o.Inactive = !active
		o.UpdatedAt, o.UpdatedBy = at, by
	}
	return nil
}

func (f *overrideFake) customOf(xa tenant.ID) map[string]*domain.CustomMessage {
	if f.custom == nil {
		f.custom = map[tenant.ID]map[string]*domain.CustomMessage{}
	}
	if f.custom[xa] == nil {
		f.custom[xa] = map[string]*domain.CustomMessage{}
	}
	return f.custom[xa]
}

func (f *overrideFake) ListCustom(ctx context.Context) ([]domain.CustomMessage, error) {
	if f.customListErr != nil {
		return nil, f.customListErr
	}
	var out []domain.CustomMessage
	for _, c := range f.customOf(tenant.MustFrom(ctx)) {
		if !f.deleted[c.ID] {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *overrideFake) CustomForUpdate(_ context.Context, tx *store.ScopedTx, key string) (*domain.CustomMessage, error) {
	f.seen(tx)
	if c := f.customOf(tx.TenantID())[key]; c != nil && !f.deleted[c.ID] {
		cp := *c
		return &cp, nil
	}
	return nil, nil
}

func (f *overrideFake) CustomKeyTaken(_ context.Context, tx *store.ScopedTx, key string) (bool, error) {
	f.seen(tx)
	// Deleted rows count: the map keeps them.
	return f.customOf(tx.TenantID())[key] != nil, nil
}

func (f *overrideFake) CountLiveCustom(_ context.Context, tx *store.ScopedTx) (int, error) {
	f.seen(tx)
	if f.liveCount != 0 {
		return f.liveCount, nil
	}
	n := 0
	for _, c := range f.customOf(tx.TenantID()) {
		if !f.deleted[c.ID] {
			n++
		}
	}
	return n, nil
}

func (f *overrideFake) AddCustom(_ context.Context, tx *store.ScopedTx, c domain.CustomMessage) error {
	f.seen(tx)
	f.customAdds++
	if f.addCustomErr != nil {
		return f.addCustomErr
	}
	f.customOf(tx.TenantID())[c.Key] = &c
	return nil
}

func (f *overrideFake) UpdateCustom(_ context.Context, tx *store.ScopedTx, c domain.CustomMessage) error {
	f.seen(tx)
	f.customUpdates++
	f.customOf(tx.TenantID())[c.Key] = &c
	return nil
}

func (f *overrideFake) SoftDeleteCustom(_ context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	f.seen(tx)
	f.customDeletes++
	f.lastBy, f.lastReason, f.lastAt = by, reason, at
	if f.deleted == nil {
		f.deleted = map[string]bool{}
	}
	f.deleted[id] = true
	return nil
}

func auditAction(t *testing.T, k *khoGia) any {
	t.Helper()
	ins := k.cau("INSERT INTO audit_log")
	if len(ins) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(ins))
	}
	return ins[0].args[4]
}

// --- the switch ---------------------------------------------------------------------------------

// OFF RESOLVES TO THE DEFAULT EVERYWHERE — the list and Text, which every refusal branch reads.
func TestSwitchedOffOverrideResolvesToDefault(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	k, f := &khoGia{}, newOverrideFake()
	f.live[xaA] = &domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"}
	uc := newMessagesUseCase(t, k, f)
	ctx := tenant.Into(context.Background(), xaA)

	got, err := uc.SetActive(ctx, m.Key, false, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.CurrentText != m.DefaultText || got.OverrideText != "Câu của xã." || !got.Overridden {
		t.Errorf("after Tắt: %+v", got)
	}
	if f.switches != 1 || k.batDau != 1 || k.daCommit != 1 || len(f.txsSeen) != 1 {
		t.Errorf("switches=%d begin=%d commit=%d txs=%d — want one write in one transaction", f.switches, k.batDau, k.daCommit, len(f.txsSeen))
	}
	if a := auditAction(t, k); a != ActionSwitchOffSystemMessage {
		t.Errorf("action = %v", a)
	}
	d := auditDelta(t, k)
	if d["truoc"]["is_active"] != true || d["sau"]["is_active"] != false || d["sau"]["text"] != m.DefaultText {
		t.Errorf("delta = %v", d)
	}

	if r, err := uc.Resolve(ctx, m.Key); err != nil || r.CurrentText != m.DefaultText || r.Active {
		t.Errorf("Resolve while off = %+v, %v — the banner would print the switched-off wording", r, err)
	}
	list, _ := uc.Messages(ctx)
	for _, x := range list {
		if x.Key == m.Key && (x.CurrentText != m.DefaultText || x.Active || x.OverrideText != "Câu của xã.") {
			t.Errorf("list while off: %+v", x)
		}
	}

	// Bật lại brings the SAME words back.
	k2 := &khoGia{}
	got, err = newMessagesUseCase(t, k2, f).SetActive(ctx, m.Key, true, staff)
	if err != nil || !got.Active || got.CurrentText != "Câu của xã." {
		t.Fatalf("Bật lại: %+v, %v", got, err)
	}
	if a := auditAction(t, k2); a != ActionSwitchOnSystemMessage {
		t.Errorf("action = %v", a)
	}
}

func TestSwitchSameStateWritesNothing(t *testing.T) {
	k, f := &khoGia{}, newOverrideFake()
	f.live[xaA] = &domain.MessageOverride{ID: "o", Key: domain.KeyBudgetScopeNotice, Text: "Câu.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"}
	if _, err := newMessagesUseCase(t, k, f).SetActive(tenant.Into(context.Background(), xaA), domain.KeyBudgetScopeNotice, true, staff); err != nil {
		t.Fatal(err)
	}
	if f.switches != 0 || k.coCau("audit_log") {
		t.Error("switching on an override already on wrote something")
	}
}

// EVERY SHIPPED domain.KeyBudgetScopeNotice HAS A SWITCH (user decision 09/10/2026, migration 0019). "Tắt" of a key the commune
// never reworded stores a row with NO wording, switched off, and audits it in the same transaction;
// readers resolve the default (CurrentText) with Active false; "Bật" soft deletes that row.
func TestSwitchOffUnwordedSentence(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	k, f := &khoGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := tenant.Into(context.Background(), xaA)

	got, err := uc.SetActive(ctx, m.Key, false, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.Overridden || got.OverrideText != "" || got.CurrentText != m.DefaultText || got.UpdatedBy != "CB-00123" {
		t.Errorf("after Tắt: %+v", got)
	}
	if f.adds != 1 || f.switches != 0 || k.batDau != 1 || k.daCommit != 1 || len(f.txsSeen) != 1 {
		t.Errorf("adds=%d switches=%d begin=%d commit=%d txs=%d — want one insert in one transaction",
			f.adds, f.switches, k.batDau, k.daCommit, len(f.txsSeen))
	}
	row, _ := f.LiveForUpdate(ctx, f.lastTx, m.Key)
	if row == nil || row.Text != "" || !row.Inactive || row.UpdatedBy != "CB-00123" {
		t.Fatalf("stored row = %+v — want no wording (NULL), switched off, who = business code", row)
	}
	if a := auditAction(t, k); a != ActionSwitchOffSystemMessage {
		t.Errorf("action = %v", a)
	}
	d := auditDelta(t, k)
	if d["truoc"]["is_active"] != true || d["sau"]["is_active"] != false || d["sau"]["overridden"] != false {
		t.Errorf("delta = %v", d)
	}
	list, err := uc.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list {
		if x.Key == m.Key && (x.Active || x.Overridden || x.CurrentText != m.DefaultText) {
			t.Errorf("list while off: %+v", x)
		}
	}

	// Sending the default to a switched-off unworded key pins nothing.
	k2 := &khoGia{}
	if _, err := newMessagesUseCase(t, k2, f).Reword(ctx, m.Key, m.DefaultText, staff); err != nil {
		t.Fatal(err)
	}
	if f.updates != 0 || k2.coCau("audit_log") {
		t.Error("rewording a switched-off unworded key with the default wrote something")
	}

	// "Bật": the row held only the switch, so it is soft deleted, with its reason, and audited.
	k3 := &khoGia{}
	got, err = newMessagesUseCase(t, k3, f).SetActive(ctx, m.Key, true, staff)
	if err != nil || !got.Active || got.Overridden || got.CurrentText != m.DefaultText {
		t.Fatalf("Bật: %+v, %v", got, err)
	}
	if f.deletes != 1 || f.lastReason != SwitchOnReason || f.lastBy != "CB-00123" || f.switches != 0 {
		t.Errorf("deletes=%d reason=%q by=%q switches=%d", f.deletes, f.lastReason, f.lastBy, f.switches)
	}
	if a := auditAction(t, k3); a != ActionSwitchOnSystemMessage {
		t.Errorf("action = %v", a)
	}
	if row, _ := f.LiveForUpdate(ctx, f.lastTx, m.Key); row != nil {
		t.Errorf("a live row survived Bật: %+v", row)
	}
}

// Switching ON a key with no row is the state already held: nothing written, nothing audited.
func TestSwitchOnWithoutRowWritesNothing(t *testing.T) {
	k, f := &khoGia{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).SetActive(tenant.Into(context.Background(), xaA), domain.KeyBudgetScopeNotice, true, staff)
	if err != nil || !got.Active {
		t.Fatalf("%+v %v", got, err)
	}
	if f.adds+f.deletes+f.switches != 0 || k.coCau("audit_log") {
		t.Error("switching on a key already on wrote something")
	}
}

func TestSwitchRefusesBeforeAnyTransaction(t *testing.T) {
	for name, c := range map[string]struct {
		key   string
		actor audit.Actor
		want  error
	}{
		"unknown key": {"feedback.never_public", staff, domain.ErrUnknownMessageKey},
		"no actor":    {domain.KeyBudgetScopeNotice, audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &khoGia{}, newOverrideFake()
			if _, err := newMessagesUseCase(t, k, f).SetActive(tenant.Into(context.Background(), xaA), c.key, false, c.actor); !errors.Is(err, c.want) {
				t.Errorf("err = %v", err)
			}
			if k.batDau != 0 {
				t.Error("opened a transaction")
			}
		})
	}
}

func TestSwitchAuditFailureRollsBack(t *testing.T) {
	k, f := &khoGia{loiSau: "INSERT INTO audit_log"}, newOverrideFake()
	f.live[xaA] = &domain.MessageOverride{ID: "o", Key: domain.KeyBudgetScopeNotice, Text: "Câu.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"}
	if _, err := newMessagesUseCase(t, k, f).SetActive(tenant.Into(context.Background(), xaA), domain.KeyBudgetScopeNotice, false, staff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}
}

// REWORDING DOES NOT FLIP THE SWITCH, and re-sending the stored words of a switched-off wording is a
// no-op (compared against the stored words, not the default in force).
func TestRewordKeepsTheSwitch(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	ctx := tenant.Into(context.Background(), xaA)
	f := newOverrideFake()
	f.live[xaA] = &domain.MessageOverride{ID: "o", Key: m.Key, Text: "Cũ.", UpdatedAt: fixedNow, UpdatedBy: "CB-1", Inactive: true}

	k := &khoGia{}
	if _, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Cũ.", staff); err != nil {
		t.Fatal(err)
	}
	if f.updates != 0 || k.coCau("audit_log") {
		t.Error("re-sending the stored words of a switched-off wording wrote something")
	}

	k = &khoGia{}
	got, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Mới.", staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.CurrentText != m.DefaultText || got.OverrideText != "Mới." {
		t.Errorf("reworded while off: %+v — want still off, new words kept", got)
	}
	if !f.live[xaA].Inactive {
		t.Error("the stored row was switched back on by a reword")
	}
}

// --- commune sentences ---------------------------------------------------------------------------

func strp(s string) *string { return &s }

func TestCreateCustomWritesRowAndAuditInOneTransaction(t *testing.T) {
	k, f := &khoGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	got, err := uc.CreateCustom(tenant.Into(context.Background(), xaA), NewCustomMessage{
		Group: "giai-ngan", Key: " giai-ngan.loi-nhac ", Text: " Xin chào. ", Description: strp("  Lời chào  "),
	}, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "giai-ngan.loi-nhac" || got.Group != "giai-ngan" || got.Origin != domain.OriginCommune ||
		got.CurrentText != "Xin chào." || got.Description != "Lời chào" || !got.Active || got.UpdatedBy != "CB-00123" {
		t.Errorf("result %+v", got)
	}
	if f.customAdds != 1 || k.batDau != 1 || k.daCommit != 1 || len(f.txsSeen) != 1 {
		t.Errorf("adds=%d begin=%d commit=%d txs=%d", f.customAdds, k.batDau, k.daCommit, len(f.txsSeen))
	}
	if f.lastTx.TenantID() != xaA {
		t.Errorf("written in %q", f.lastTx.TenantID())
	}
	stored := f.custom[xaA]["giai-ngan.loi-nhac"]
	if stored.CreatedBy != "CB-00123" || stored.UpdatedBy != "CB-00123" {
		t.Errorf("who = %q/%q, want the staff business code", stored.CreatedBy, stored.UpdatedBy)
	}
	if a := auditAction(t, k); a != ActionAddCustomMessage {
		t.Errorf("action = %v", a)
	}
	if subj := k.cau("INSERT INTO audit_log")[0].args[5]; subj != "giai-ngan.loi-nhac" {
		t.Errorf("subject = %v", subj)
	}
	d := auditDelta(t, k)
	if d["sau"]["text"] != "Xin chào." || d["sau"]["group_code"] != "giai-ngan" || d["sau"]["is_active"] != true {
		t.Errorf("delta = %v", d)
	}

	// Listed after the shipped sentences.
	list, err := uc.Messages(tenant.Into(context.Background(), xaA))
	if err != nil {
		t.Fatal(err)
	}
	last := list[len(list)-1]
	if len(list) != len(domain.ShippedMessages())+1 || last.Key != "giai-ngan.loi-nhac" || last.Origin != domain.OriginCommune {
		t.Errorf("list = %+v", list)
	}
	if other, _ := uc.Messages(tenant.Into(context.Background(), xaB)); len(other) != len(domain.ShippedMessages()) {
		t.Error("commune B lists commune A's sentence")
	}
}

func TestCreateCustomRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		in   NewCustomMessage
		want error
	}{
		"group of another service": {NewCustomMessage{Group: "chung", Key: "chung.x", Text: "C."}, domain.ErrCustomGroupUnknown},
		"report group":             {NewCustomMessage{Group: "bao-cao", Key: "bao-cao.x", Text: "C."}, domain.ErrCustomGroupUnknown},
		"key without prefix":       {NewCustomMessage{Group: "giai-ngan", Key: "loi-chao", Text: "C."}, domain.ErrCustomKeyPrefix},
		"key of another group":     {NewCustomMessage{Group: "giai-ngan", Key: "phan-anh.x", Text: "C."}, domain.ErrCustomKeyPrefix},
		"prefix only":              {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.", Text: "C."}, domain.ErrCustomKeyShape},
		"uppercase key":            {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.Loi", Text: "C."}, domain.ErrCustomKeyShape},
		"underscore key":           {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.loi_chao", Text: "C."}, domain.ErrCustomKeyShape},
		"empty text":               {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.x", Text: " "}, domain.ErrMessageTextEmpty},
		"markup text":              {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.x", Text: "<b>"}, domain.ErrMessageTextMarkup},
		"markup description":       {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.x", Text: "C.", Description: strp("<i>")}, domain.ErrDescriptionInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &khoGia{}, newOverrideFake()
			if _, err := newMessagesUseCase(t, k, f).CreateCustom(tenant.Into(context.Background(), xaA), c.in, staff); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if k.batDau != 0 {
				t.Error("a shape refusal opened a transaction")
			}
		})
	}
}

// A DELETED KEY IS NEVER REUSED (rule 7, invariant 3), and the ceiling refuses before the key check.
func TestCreateCustomKeyTakenAndCeiling(t *testing.T) {
	ctx := tenant.Into(context.Background(), xaA)
	f := newOverrideFake()
	f.customOf(xaA)["giai-ngan.cam-on"] = &domain.CustomMessage{ID: "c1", Key: "giai-ngan.cam-on"}
	f.deleted = map[string]bool{"c1": true}

	k := &khoGia{}
	_, err := newMessagesUseCase(t, k, f).CreateCustom(ctx, NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.cam-on", Text: "C."}, staff)
	if !errors.Is(err, domain.ErrMessageCodeTaken) || f.customAdds != 0 || k.daCommit != 0 {
		t.Errorf("deleted key reused: err=%v adds=%d", err, f.customAdds)
	}

	f2 := newOverrideFake()
	f2.liveCount = domain.TranCustomMessages
	k = &khoGia{}
	_, err = newMessagesUseCase(t, k, f2).CreateCustom(ctx, NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.x", Text: "C."}, staff)
	if !errors.Is(err, domain.ErrCustomCatalogueFull) || f2.customAdds != 0 {
		t.Errorf("ceiling: err=%v", err)
	}
}

func TestEditCustom(t *testing.T) {
	ctx := tenant.Into(context.Background(), xaA)
	seed := func() *overrideFake {
		f := newOverrideFake()
		f.customOf(xaA)["giai-ngan.x"] = &domain.CustomMessage{ID: "c1", Group: "giai-ngan", Key: "giai-ngan.x",
			Text: "Cũ.", Description: "Mô tả", Active: true, CreatedBy: "CB-1", UpdatedBy: "CB-1", CreatedAt: fixedNow, UpdatedAt: fixedNow}
		return f
	}

	// Words + clear description.
	k, f := &khoGia{}, seed()
	got, err := newMessagesUseCase(t, k, f).EditCustom(ctx, "giai-ngan.x", CustomMessageEdit{
		Text: strp("Mới."), Description: &DescriptionChange{},
	}, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != "Mới." || got.Description != "" || got.UpdatedBy != "CB-00123" {
		t.Errorf("result %+v", got)
	}
	if a := auditAction(t, k); a != ActionEditCustomMessage {
		t.Errorf("action = %v", a)
	}
	if f.custom[xaA]["giai-ngan.x"].CreatedBy != "CB-1" {
		t.Error("created_by rewritten by an edit")
	}

	// Only the switch moved: the entry says tắt.
	k, f = &khoGia{}, seed()
	off := false
	if got, err = newMessagesUseCase(t, k, f).EditCustom(ctx, "giai-ngan.x", CustomMessageEdit{Active: &off}, staff); err != nil || got.Active {
		t.Fatalf("%+v, %v", got, err)
	}
	if a := auditAction(t, k); a != ActionSwitchOffSystemMessage {
		t.Errorf("action = %v, want the switch verb", a)
	}

	// Nothing moved: nothing written.
	k, f = &khoGia{}, seed()
	if _, err := newMessagesUseCase(t, k, f).EditCustom(ctx, "giai-ngan.x", CustomMessageEdit{Text: strp(" Cũ. ")}, staff); err != nil {
		t.Fatal(err)
	}
	if f.customUpdates != 0 || k.coCau("audit_log") {
		t.Error("a no-op edit wrote something")
	}

	// A shipped key is not a commune sentence; an unknown key is not found; another commune's is not found.
	k, f = &khoGia{}, seed()
	if _, err := newMessagesUseCase(t, k, f).EditCustom(ctx, domain.KeyBudgetScopeNotice, CustomMessageEdit{Text: strp("x")}, staff); !errors.Is(err, domain.ErrShippedMessageNotCustom) || k.batDau != 0 {
		t.Errorf("shipped: %v", err)
	}
	if _, err := newMessagesUseCase(t, k, f).EditCustom(tenant.Into(context.Background(), xaB), "giai-ngan.x", CustomMessageEdit{Text: strp("x")}, staff); !errors.Is(err, domain.ErrCustomMessageNotFound) {
		t.Errorf("other commune: %v", err)
	}
}

func TestDeleteCustom(t *testing.T) {
	ctx := tenant.Into(context.Background(), xaA)
	f := newOverrideFake()
	f.customOf(xaA)["giai-ngan.x"] = &domain.CustomMessage{ID: "c1", Group: "giai-ngan", Key: "giai-ngan.x", Text: "C.", Active: true}

	k := &khoGia{}
	if err := newMessagesUseCase(t, k, f).DeleteCustom(ctx, "giai-ngan.x", "  Không dùng nữa  ", staff); err != nil {
		t.Fatal(err)
	}
	if f.customDeletes != 1 || f.lastBy != "CB-00123" || f.lastReason != "Không dùng nữa" || !f.lastAt.Equal(fixedNow) {
		t.Errorf("soft delete by=%q reason=%q", f.lastBy, f.lastReason)
	}
	if a := auditAction(t, k); a != ActionDeleteCustomMessage || k.daCommit != 1 {
		t.Errorf("action = %v commit=%d", a, k.daCommit)
	}
	if list, _ := newMessagesUseCase(t, &khoGia{}, f).Messages(ctx); len(list) != len(domain.ShippedMessages()) {
		t.Error("a deleted sentence is still listed")
	}
	// Second delete: not found, nothing rewritten.
	if err := newMessagesUseCase(t, &khoGia{}, f).DeleteCustom(ctx, "giai-ngan.x", "x", staff); !errors.Is(err, domain.ErrCustomMessageNotFound) || f.customDeletes != 1 {
		t.Errorf("second delete: %v", err)
	}

	for name, c := range map[string]struct {
		key, reason string
		want        error
	}{
		"shipped key": {domain.KeyBudgetScopeNotice, "x", domain.ErrShippedMessageNotDeletable},
		"no reason":   {"giai-ngan.y", "  ", domain.ErrCustomDeleteReasonEmpty},
	} {
		k := &khoGia{}
		if err := newMessagesUseCase(t, k, newOverrideFake()).DeleteCustom(ctx, c.key, c.reason, staff); !errors.Is(err, c.want) || k.batDau != 0 {
			t.Errorf("%s: err=%v begin=%d", name, err, k.batDau)
		}
	}
}

func TestMessagesCustomReadFailureFailsTheList(t *testing.T) {
	f := newOverrideFake()
	f.customListErr = errors.New("db down")
	if _, err := newMessagesUseCase(t, &khoGia{}, f).Messages(tenant.Into(context.Background(), xaA)); err == nil {
		t.Fatal("half a list returned as the whole")
	}
}

// THE RACE THE COUNT CANNOT SEE: two creates of one key both pass CustomKeyTaken, and the second INSERT
// meets UNIQUE (tenant_id, message_key). The store reports that as ErrMessageCodeTaken (proved in
// store/system_message_store_test.go); it must reach the caller as that refusal — 409 — and roll back.
func TestCreateCustomUniqueRaceIsCodeTaken(t *testing.T) {
	k, f := &khoGia{}, newOverrideFake()
	f.addCustomErr = fmt.Errorf("custom_system_message: chèn: %w", domain.ErrMessageCodeTaken)
	_, err := newMessagesUseCase(t, k, f).CreateCustom(tenant.Into(context.Background(), xaA), NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.dua", Text: "C."}, staff)
	if !errors.Is(err, domain.ErrMessageCodeTaken) {
		t.Errorf("err = %v, want ErrMessageCodeTaken", err)
	}
	if k.daCommit != 0 || k.coCau("audit_log") {
		t.Error("a refused insert committed or left an entry")
	}
}
