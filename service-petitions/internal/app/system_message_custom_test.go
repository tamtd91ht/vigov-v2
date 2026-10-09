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
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// "Tắt / Bật lại" and the commune's own sentences (ADR 0079 Q2, Q5a, Q5b; migration 0033), over the
// same real transaction on the recording driver as system_message_test.go. The fake below extends
// overrideFake with the switch and the commune-sentence half.

// --- the fake's other half ------------------------------------------------------------------------

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

func auditAction(t *testing.T, k *khoLoaiNhiemVuGia) any {
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
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackReasonRequired)
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	uc := newMessagesUseCase(t, k, f)
	ctx := inCommune(messageCommuneA)

	got, err := uc.SetActive(ctx, m.Key, false, messageStaff)
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
	d := messageAuditDelta(t, k)
	if d["truoc"]["is_active"] != true || d["sau"]["is_active"] != false || d["sau"]["text"] != m.DefaultText {
		t.Errorf("delta = %v", d)
	}

	if text, err := uc.Text(ctx, m.Key); err != nil || text != m.DefaultText {
		t.Errorf("Text while off = %q, %v — a refusal would send the switched-off wording", text, err)
	}
	list, _ := uc.Messages(ctx)
	for _, x := range list {
		if x.Key == m.Key && (x.CurrentText != m.DefaultText || x.Active || x.OverrideText != "Câu của xã.") {
			t.Errorf("list while off: %+v", x)
		}
	}

	// Bật lại brings the SAME words back.
	k2 := &khoLoaiNhiemVuGia{}
	got, err = newMessagesUseCase(t, k2, f).SetActive(ctx, m.Key, true, messageStaff)
	if err != nil || !got.Active || got.CurrentText != "Câu của xã." {
		t.Fatalf("Bật lại: %+v, %v", got, err)
	}
	if a := auditAction(t, k2); a != ActionSwitchOnSystemMessage {
		t.Errorf("action = %v", a)
	}
}

func TestSwitchSameStateWritesNothing(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: domain.KeyFeedbackNeverPublic, Text: "Câu.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	if _, err := newMessagesUseCase(t, k, f).SetActive(inCommune(messageCommuneA), domain.KeyFeedbackNeverPublic, true, messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.switches != 0 || k.coCau("audit_log") {
		t.Error("switching on an override already on wrote something")
	}
}

// EVERY SHIPPED domain.KeyFeedbackReasonRequired HAS A SWITCH (user decision 09/10/2026, migration 0035). "Tắt" of a key the commune
// never reworded stores a row with NO wording, switched off, and audits it in the same transaction;
// readers resolve the default (CurrentText) with Active false; "Bật" soft deletes that row.
func TestSwitchOffUnwordedSentence(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackReasonRequired)
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := inCommune(messageCommuneA)

	got, err := uc.SetActive(ctx, m.Key, false, messageStaff)
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
	d := messageAuditDelta(t, k)
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
	k2 := &khoLoaiNhiemVuGia{}
	if _, err := newMessagesUseCase(t, k2, f).Reword(ctx, m.Key, m.DefaultText, messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.updates != 0 || k2.coCau("audit_log") {
		t.Error("rewording a switched-off unworded key with the default wrote something")
	}

	// "Bật": the row held only the switch, so it is soft deleted, with its reason, and audited.
	k3 := &khoLoaiNhiemVuGia{}
	got, err = newMessagesUseCase(t, k3, f).SetActive(ctx, m.Key, true, messageStaff)
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
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).SetActive(inCommune(messageCommuneA), domain.KeyFeedbackReasonRequired, true, messageStaff)
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
		"unknown key": {"budget.scope_notice", messageStaff, domain.ErrUnknownMessageKey},
		"no actor":    {domain.KeyFeedbackReasonRequired, audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
			if _, err := newMessagesUseCase(t, k, f).SetActive(inCommune(messageCommuneA), c.key, false, c.actor); !errors.Is(err, c.want) {
				t.Errorf("err = %v", err)
			}
			if k.batDau != 0 {
				t.Error("opened a transaction")
			}
		})
	}
}

func TestSwitchAuditFailureRollsBack(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{loiSau: "INSERT INTO audit_log"}, newOverrideFake()
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: domain.KeyFeedbackNeverPublic, Text: "Câu.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	if _, err := newMessagesUseCase(t, k, f).SetActive(inCommune(messageCommuneA), domain.KeyFeedbackNeverPublic, false, messageStaff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}
}

// REWORDING DOES NOT FLIP THE SWITCH, and re-sending the stored words of a switched-off wording is a
// no-op (compared against the stored words, not the default in force).
func TestRewordKeepsTheSwitch(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackReasonRequired)
	ctx := inCommune(messageCommuneA)
	f := newOverrideFake()
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Cũ.", UpdatedAt: messageNow, UpdatedBy: "CB-1", Inactive: true})

	k := &khoLoaiNhiemVuGia{}
	if _, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Cũ.", messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.updates != 0 || k.coCau("audit_log") {
		t.Error("re-sending the stored words of a switched-off wording wrote something")
	}

	k = &khoLoaiNhiemVuGia{}
	got, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Mới.", messageStaff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || got.CurrentText != m.DefaultText || got.OverrideText != "Mới." {
		t.Errorf("reworded while off: %+v — want still off, new words kept", got)
	}
	if !f.live[messageCommuneA][m.Key].Inactive {
		t.Error("the stored row was switched back on by a reword")
	}
}

// --- commune sentences ---------------------------------------------------------------------------

func strp(s string) *string { return &s }

func TestCreateCustomWritesRowAndAuditInOneTransaction(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	got, err := uc.CreateCustom(inCommune(messageCommuneA), NewCustomMessage{
		Group: "chung", Key: " chung.loi-chao ", Text: " Xin chào. ", Description: strp("  Lời chào  "),
	}, messageStaff)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "chung.loi-chao" || got.Group != "chung" || got.Origin != domain.OriginCommune ||
		got.CurrentText != "Xin chào." || got.Description != "Lời chào" || !got.Active || got.UpdatedBy != "CB-00123" {
		t.Errorf("result %+v", got)
	}
	if f.customAdds != 1 || k.batDau != 1 || k.daCommit != 1 || len(f.txsSeen) != 1 {
		t.Errorf("adds=%d begin=%d commit=%d txs=%d", f.customAdds, k.batDau, k.daCommit, len(f.txsSeen))
	}
	if f.lastTx.TenantID() != messageCommuneA {
		t.Errorf("written in %q", f.lastTx.TenantID())
	}
	stored := f.custom[messageCommuneA]["chung.loi-chao"]
	if stored.CreatedBy != "CB-00123" || stored.UpdatedBy != "CB-00123" {
		t.Errorf("who = %q/%q, want the staff business code", stored.CreatedBy, stored.UpdatedBy)
	}
	if a := auditAction(t, k); a != ActionAddCustomMessage {
		t.Errorf("action = %v", a)
	}
	if subj := k.cau("INSERT INTO audit_log")[0].args[5]; subj != "chung.loi-chao" {
		t.Errorf("subject = %v", subj)
	}
	d := messageAuditDelta(t, k)
	if d["sau"]["text"] != "Xin chào." || d["sau"]["group_code"] != "chung" || d["sau"]["is_active"] != true {
		t.Errorf("delta = %v", d)
	}

	// Listed after the shipped sentences.
	list, err := uc.Messages(inCommune(messageCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	last := list[len(list)-1]
	if len(list) != len(domain.ShippedMessages())+1 || last.Key != "chung.loi-chao" || last.Origin != domain.OriginCommune {
		t.Errorf("list = %+v", list)
	}
	if other, _ := uc.Messages(inCommune(messageCommuneB)); len(other) != len(domain.ShippedMessages()) {
		t.Error("commune B lists commune A's sentence")
	}
}

func TestCreateCustomRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		in   NewCustomMessage
		want error
	}{
		"group of another service": {NewCustomMessage{Group: "giai-ngan", Key: "giai-ngan.x", Text: "C."}, domain.ErrCustomGroupUnknown},
		"report group":             {NewCustomMessage{Group: "bao-cao", Key: "bao-cao.x", Text: "C."}, domain.ErrCustomGroupUnknown},
		"key without prefix":       {NewCustomMessage{Group: "chung", Key: "loi-chao", Text: "C."}, domain.ErrCustomKeyPrefix},
		"key of another group":     {NewCustomMessage{Group: "chung", Key: "phan-anh.x", Text: "C."}, domain.ErrCustomKeyPrefix},
		"prefix only":              {NewCustomMessage{Group: "chung", Key: "chung.", Text: "C."}, domain.ErrCustomKeyShape},
		"uppercase key":            {NewCustomMessage{Group: "chung", Key: "chung.Loi", Text: "C."}, domain.ErrCustomKeyShape},
		"underscore key":           {NewCustomMessage{Group: "chung", Key: "chung.loi_chao", Text: "C."}, domain.ErrCustomKeyShape},
		"empty text":               {NewCustomMessage{Group: "chung", Key: "chung.x", Text: " "}, domain.ErrMessageTextEmpty},
		"markup text":              {NewCustomMessage{Group: "chung", Key: "chung.x", Text: "<b>"}, domain.ErrMessageTextMarkup},
		"markup description":       {NewCustomMessage{Group: "chung", Key: "chung.x", Text: "C.", Description: strp("<i>")}, domain.ErrDescriptionInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
			if _, err := newMessagesUseCase(t, k, f).CreateCustom(inCommune(messageCommuneA), c.in, messageStaff); !errors.Is(err, c.want) {
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
	ctx := inCommune(messageCommuneA)
	f := newOverrideFake()
	f.customOf(messageCommuneA)["phan-anh.cam-on"] = &domain.CustomMessage{ID: "c1", Key: "phan-anh.cam-on"}
	f.deleted = map[string]bool{"c1": true}

	k := &khoLoaiNhiemVuGia{}
	_, err := newMessagesUseCase(t, k, f).CreateCustom(ctx, NewCustomMessage{Group: "phan-anh", Key: "phan-anh.cam-on", Text: "C."}, messageStaff)
	if !errors.Is(err, domain.ErrMessageCodeTaken) || f.customAdds != 0 || k.daCommit != 0 {
		t.Errorf("deleted key reused: err=%v adds=%d", err, f.customAdds)
	}

	f2 := newOverrideFake()
	f2.liveCount = domain.TranCustomMessages
	k = &khoLoaiNhiemVuGia{}
	_, err = newMessagesUseCase(t, k, f2).CreateCustom(ctx, NewCustomMessage{Group: "chung", Key: "chung.x", Text: "C."}, messageStaff)
	if !errors.Is(err, domain.ErrCustomCatalogueFull) || f2.customAdds != 0 {
		t.Errorf("ceiling: err=%v", err)
	}
}

func TestEditCustom(t *testing.T) {
	ctx := inCommune(messageCommuneA)
	seed := func() *overrideFake {
		f := newOverrideFake()
		f.customOf(messageCommuneA)["chung.x"] = &domain.CustomMessage{ID: "c1", Group: "chung", Key: "chung.x",
			Text: "Cũ.", Description: "Mô tả", Active: true, CreatedBy: "CB-1", UpdatedBy: "CB-1", CreatedAt: messageNow, UpdatedAt: messageNow}
		return f
	}

	// Words + clear description.
	k, f := &khoLoaiNhiemVuGia{}, seed()
	got, err := newMessagesUseCase(t, k, f).EditCustom(ctx, "chung.x", CustomMessageEdit{
		Text: strp("Mới."), Description: &DescriptionChange{},
	}, messageStaff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != "Mới." || got.Description != "" || got.UpdatedBy != "CB-00123" {
		t.Errorf("result %+v", got)
	}
	if a := auditAction(t, k); a != ActionEditCustomMessage {
		t.Errorf("action = %v", a)
	}
	if f.custom[messageCommuneA]["chung.x"].CreatedBy != "CB-1" {
		t.Error("created_by rewritten by an edit")
	}

	// Only the switch moved: the entry says tắt.
	k, f = &khoLoaiNhiemVuGia{}, seed()
	off := false
	if got, err = newMessagesUseCase(t, k, f).EditCustom(ctx, "chung.x", CustomMessageEdit{Active: &off}, messageStaff); err != nil || got.Active {
		t.Fatalf("%+v, %v", got, err)
	}
	if a := auditAction(t, k); a != ActionSwitchOffSystemMessage {
		t.Errorf("action = %v, want the switch verb", a)
	}

	// Nothing moved: nothing written.
	k, f = &khoLoaiNhiemVuGia{}, seed()
	if _, err := newMessagesUseCase(t, k, f).EditCustom(ctx, "chung.x", CustomMessageEdit{Text: strp(" Cũ. ")}, messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.customUpdates != 0 || k.coCau("audit_log") {
		t.Error("a no-op edit wrote something")
	}

	// A shipped key is not a commune sentence; an unknown key is not found; another commune's is not found.
	k, f = &khoLoaiNhiemVuGia{}, seed()
	if _, err := newMessagesUseCase(t, k, f).EditCustom(ctx, domain.KeyFeedbackNeverPublic, CustomMessageEdit{Text: strp("x")}, messageStaff); !errors.Is(err, domain.ErrShippedMessageNotCustom) || k.batDau != 0 {
		t.Errorf("shipped: %v", err)
	}
	if _, err := newMessagesUseCase(t, k, f).EditCustom(inCommune(messageCommuneB), "chung.x", CustomMessageEdit{Text: strp("x")}, messageStaff); !errors.Is(err, domain.ErrCustomMessageNotFound) {
		t.Errorf("other commune: %v", err)
	}
}

func TestDeleteCustom(t *testing.T) {
	ctx := inCommune(messageCommuneA)
	f := newOverrideFake()
	f.customOf(messageCommuneA)["chung.x"] = &domain.CustomMessage{ID: "c1", Group: "chung", Key: "chung.x", Text: "C.", Active: true}

	k := &khoLoaiNhiemVuGia{}
	if err := newMessagesUseCase(t, k, f).DeleteCustom(ctx, "chung.x", "  Không dùng nữa  ", messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.customDeletes != 1 || f.lastBy != "CB-00123" || f.lastReason != "Không dùng nữa" || !f.lastAt.Equal(messageNow) {
		t.Errorf("soft delete by=%q reason=%q", f.lastBy, f.lastReason)
	}
	if a := auditAction(t, k); a != ActionDeleteCustomMessage || k.daCommit != 1 {
		t.Errorf("action = %v commit=%d", a, k.daCommit)
	}
	if list, _ := newMessagesUseCase(t, &khoLoaiNhiemVuGia{}, f).Messages(ctx); len(list) != len(domain.ShippedMessages()) {
		t.Error("a deleted sentence is still listed")
	}
	// Second delete: not found, nothing rewritten.
	if err := newMessagesUseCase(t, &khoLoaiNhiemVuGia{}, f).DeleteCustom(ctx, "chung.x", "x", messageStaff); !errors.Is(err, domain.ErrCustomMessageNotFound) || f.customDeletes != 1 {
		t.Errorf("second delete: %v", err)
	}

	for name, c := range map[string]struct {
		key, reason string
		want        error
	}{
		"shipped key": {domain.KeyFeedbackReasonRequired, "x", domain.ErrShippedMessageNotDeletable},
		"no reason":   {"chung.y", "  ", domain.ErrCustomDeleteReasonEmpty},
	} {
		k := &khoLoaiNhiemVuGia{}
		if err := newMessagesUseCase(t, k, newOverrideFake()).DeleteCustom(ctx, c.key, c.reason, messageStaff); !errors.Is(err, c.want) || k.batDau != 0 {
			t.Errorf("%s: err=%v begin=%d", name, err, k.batDau)
		}
	}
}

func TestMessagesCustomReadFailureFailsTheList(t *testing.T) {
	f := newOverrideFake()
	f.customListErr = errors.New("db down")
	if _, err := newMessagesUseCase(t, &khoLoaiNhiemVuGia{}, f).Messages(inCommune(messageCommuneA)); err == nil {
		t.Fatal("half a list returned as the whole")
	}
}

// THE RACE THE COUNT CANNOT SEE: two creates of one key both pass CustomKeyTaken, and the second INSERT
// meets UNIQUE (tenant_id, message_key). The store reports that as ErrMessageCodeTaken (proved in
// store/system_message_store_test.go); it must reach the caller as that refusal — 409 — and roll back.
func TestCreateCustomUniqueRaceIsCodeTaken(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.addCustomErr = fmt.Errorf("custom_system_message: chèn: %w", domain.ErrMessageCodeTaken)
	_, err := newMessagesUseCase(t, k, f).CreateCustom(inCommune(messageCommuneA), NewCustomMessage{Group: "chung", Key: "chung.dua", Text: "C."}, messageStaff)
	if !errors.Is(err, domain.ErrMessageCodeTaken) {
		t.Errorf("err = %v, want ErrMessageCodeTaken", err)
	}
	if k.daCommit != 0 || k.coCau("audit_log") {
		t.Error("a refused insert committed or left an entry")
	}
}
