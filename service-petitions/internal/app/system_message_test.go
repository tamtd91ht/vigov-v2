package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The use case runs over a REAL core/store transaction on the recording driver
// (khoLoaiNhiemVuGia, driver_gia_loai_nhiem_vu_test.go), so "the audit entry is in the same
// transaction" and "a failure rolls the write back" are properties of real Begin/Commit/Rollback
// calls. The override store is an in-memory fake that records WHICH TRANSACTION each write arrived
// in and WHICH COMMUNE — the only statement reaching the driver is the audit INSERT. The store's own
// SQL is exercised by nothing here, and says so.

var (
	messageCommuneA = tenant.ID("01JA" + strings.Repeat("M", 22))
	messageCommuneB = tenant.ID("01JB" + strings.Repeat("N", 22))
)

type overrideFake struct {
	live map[tenant.ID]map[string]*domain.MessageOverride

	listErr error

	adds, updates, deletes int
	lastTx                 *store.ScopedTx
	txsSeen                map[*store.ScopedTx]bool
	lastBy, lastReason     string
	lastAt                 time.Time

	// The switch and the commune-sentence half (system_message_custom_test.go).
	switches                                 int
	custom                                   map[tenant.ID]map[string]*domain.CustomMessage
	deleted                                  map[string]bool
	customListErr                            error
	liveCount                                int
	customAdds, customUpdates, customDeletes int
	addCustomErr                             error
}

func newOverrideFake() *overrideFake {
	return &overrideFake{live: map[tenant.ID]map[string]*domain.MessageOverride{}, txsSeen: map[*store.ScopedTx]bool{}}
}

func (f *overrideFake) set(xa tenant.ID, o domain.MessageOverride) {
	if f.live[xa] == nil {
		f.live[xa] = map[string]*domain.MessageOverride{}
	}
	f.live[xa][o.Key] = &o
}

func (f *overrideFake) seen(tx *store.ScopedTx) { f.lastTx = tx; f.txsSeen[tx] = true }

func (f *overrideFake) ListLive(ctx context.Context) ([]domain.MessageOverride, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.MessageOverride
	for _, o := range f.live[tenant.MustFrom(ctx)] {
		out = append(out, *o)
	}
	return out, nil
}

func (f *overrideFake) LiveForUpdate(_ context.Context, tx *store.ScopedTx, key string) (*domain.MessageOverride, error) {
	f.seen(tx)
	if o := f.live[tx.TenantID()][key]; o != nil {
		c := *o
		return &c, nil
	}
	return nil, nil
}

func (f *overrideFake) AddOverride(_ context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	f.seen(tx)
	f.adds++
	f.set(tx.TenantID(), o)
	return nil
}

func (f *overrideFake) UpdateText(_ context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	f.seen(tx)
	f.updates++
	f.set(tx.TenantID(), o)
	return nil
}

func (f *overrideFake) SoftDelete(_ context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	f.seen(tx)
	f.deletes++
	f.lastBy, f.lastReason, f.lastAt = by, reason, at
	for k, o := range f.live[tx.TenantID()] {
		if o.ID == id {
			delete(f.live[tx.TenantID()], k)
		}
	}
	return nil
}

var messageNow = time.Date(2026, 9, 29, 2, 30, 0, 0, time.UTC)

func newMessagesUseCase(t *testing.T, k *khoLoaiNhiemVuGia, f *overrideFake) *SystemMessages {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	uc := NewSystemMessages(store.New(db), f)
	uc.newID = func() (string, error) { return "01JOVERRIDEID0000000000000", nil }
	uc.now = func() time.Time { return messageNow }
	return uc
}

var messageStaff = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func messageAuditDelta(t *testing.T, k *khoLoaiNhiemVuGia) map[string]map[string]any {
	t.Helper()
	ins := k.cau("INSERT INTO audit_log")
	if len(ins) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(ins))
	}
	args := ins[0].args
	if args[1] != "CB-00123" {
		t.Errorf("actor_id = %v, want the staff business code", args[1])
	}
	var d map[string]map[string]any
	if err := json.Unmarshal(args[7].([]byte), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func inCommune(xa tenant.ID) context.Context { return tenant.Into(context.Background(), xa) }

func TestMessagesFallsBackToDefaultWithoutOverride(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)

	got, err := uc.Messages(inCommune(messageCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	shipped := domain.ShippedMessages()
	if len(got) != len(shipped) {
		t.Fatalf("got %d messages, want %d", len(got), len(shipped))
	}
	for i := range got {
		if got[i].Key != shipped[i].Key || got[i].CurrentText != shipped[i].DefaultText || got[i].Overridden {
			t.Errorf("row %d = %+v, want the shipped default", i, got[i])
		}
	}

	// Commune B's wording never reaches commune A.
	f.set(messageCommuneB, domain.MessageOverride{ID: "b", Key: domain.KeyFeedbackReasonRequired, Text: "Câu xã B.", UpdatedAt: messageNow, UpdatedBy: "CB-9"})
	got, _ = uc.Messages(inCommune(messageCommuneA))
	for _, m := range got {
		if m.Overridden {
			t.Errorf("commune A reads %q — another commune's wording", m.CurrentText)
		}
	}
	got, _ = uc.Messages(inCommune(messageCommuneB))
	for _, m := range got {
		if m.Key == domain.KeyFeedbackReasonRequired && (m.CurrentText != "Câu xã B." || !m.Overridden || m.UpdatedBy != "CB-9") {
			t.Errorf("commune B: %+v", m)
		}
	}
}

func TestMessagesStoreFailureIsAnErrorNotTheDefault(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.listErr = errors.New("db down")
	if _, err := newMessagesUseCase(t, k, f).Messages(inCommune(messageCommuneA)); err == nil {
		t.Fatal("a failed read returned the default — the administrator would see words they replaced")
	}
}

// --- Text: the refusal branches' read -------------------------------------------------------------

func TestTextReturnsOverrideOfThisCommuneElseDefault(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackReasonRequired)

	got, err := uc.Text(inCommune(messageCommuneA), m.Key)
	if err != nil || got != m.DefaultText {
		t.Fatalf("no override: %q, %v — want the default", got, err)
	}

	f.set(messageCommuneA, domain.MessageOverride{ID: "a", Key: m.Key, Text: "Câu của xã A.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	// An override of ANOTHER key in the same commune leaves this one on the default.
	f.set(messageCommuneA, domain.MessageOverride{ID: "a2", Key: domain.KeyFeedbackNeverPublic, Text: "Khác.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	if got, err := uc.Text(inCommune(messageCommuneA), m.Key); err != nil || got != "Câu của xã A." {
		t.Errorf("override: %q, %v", got, err)
	}
	if got, _ := uc.Text(inCommune(messageCommuneB), m.Key); got != m.DefaultText {
		t.Errorf("commune B reads %q — commune A's wording", got)
	}
	if k.batDau != 0 {
		t.Error("a read of a wording opened a transaction")
	}
}

func TestTextStoreFailureReturnsDefaultAndTheError(t *testing.T) {
	// The refusal must go out whatever happened to the wording lookup: the default comes back, and
	// the error comes back beside it for the caller's log.
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.listErr = errors.New("db down")
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackInvalidTransition)
	got, err := newMessagesUseCase(t, k, f).Text(inCommune(messageCommuneA), m.Key)
	if got != m.DefaultText {
		t.Errorf("text = %q, want the default on a failed read", got)
	}
	if err == nil {
		t.Error("failure swallowed — the caller could not log it")
	}
}

func TestTextUnknownKey(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).Text(inCommune(messageCommuneA), "budget.scope_notice")
	if got != "" || !errors.Is(err, domain.ErrUnknownMessageKey) {
		t.Errorf("got %q, %v", got, err)
	}
}

// --- writes -------------------------------------------------------------------------------------

func TestRewordWritesRowAndAuditInOneTransaction(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := inCommune(messageCommuneA)
	key := domain.KeyFeedbackReasonRequired

	got, err := uc.Reword(ctx, key, "  Câu của xã A.  ", messageStaff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != "Câu của xã A." || !got.Overridden || got.UpdatedBy != "CB-00123" ||
		got.UpdatedAt == nil || !got.UpdatedAt.Equal(messageNow) {
		t.Errorf("result %+v", got)
	}
	if f.adds != 1 || f.updates != 0 {
		t.Errorf("adds=%d updates=%d, want one insert", f.adds, f.updates)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 || len(f.txsSeen) != 1 {
		t.Errorf("begin=%d commit=%d rollback=%d txs=%d — want ONE committed transaction",
			k.batDau, k.daCommit, k.daRollback, len(f.txsSeen))
	}
	if f.lastTx.TenantID() != messageCommuneA {
		t.Errorf("written in commune %q", f.lastTx.TenantID())
	}
	d := messageAuditDelta(t, k)
	m, _ := domain.LookupShippedMessage(key)
	if d["truoc"]["text"] != m.DefaultText || d["truoc"]["overridden"] != false ||
		d["sau"]["text"] != "Câu của xã A." || d["sau"]["overridden"] != true {
		t.Errorf("delta = %v, want before=default after=new text", d)
	}
	if subj := k.cau("INSERT INTO audit_log")[0].args[5]; subj != key {
		t.Errorf("subject = %v, want the message key", subj)
	}

	// Second wording: UPDATE of the same row, before = first wording.
	k2 := &khoLoaiNhiemVuGia{}
	if _, err := newMessagesUseCase(t, k2, f).Reword(ctx, key, "Câu thứ hai.", messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.adds != 1 || f.updates != 1 {
		t.Errorf("adds=%d updates=%d, want the live row updated in place", f.adds, f.updates)
	}
	if d := messageAuditDelta(t, k2); d["truoc"]["text"] != "Câu của xã A." || d["sau"]["text"] != "Câu thứ hai." {
		t.Errorf("delta = %v", d)
	}
}

func TestRewordAuditFailureRollsBackTheWrite(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{loiSau: "INSERT INTO audit_log"}, newOverrideFake()
	if _, err := newMessagesUseCase(t, k, f).Reword(inCommune(messageCommuneA), domain.KeyFeedbackReasonRequired, "Câu.", messageStaff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d — the wording must not commit without its entry", k.daCommit, k.daRollback)
	}
}

func TestRewordNoOpWritesNothing(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackNeverPublic)
	ctx := inCommune(messageCommuneA)

	// Sending the default while on the default: no row pinned, no entry.
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, m.DefaultText, messageStaff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.adds+f.updates != 0 || k.coCau("audit_log") {
		t.Error("the default was pinned into a row, or an entry saying nothing changed was filed")
	}

	// Sending the current override again.
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})
	k = &khoLoaiNhiemVuGia{}
	if _, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Câu.", messageStaff); err != nil {
		t.Fatal(err)
	}
	if f.adds+f.updates != 0 || k.coCau("audit_log") {
		t.Error("same text re-sent wrote something")
	}
}

func TestRewordRefusesBeforeAnyTransaction(t *testing.T) {
	ctx := inCommune(messageCommuneA)
	key := domain.KeyFeedbackReasonRequired
	for name, c := range map[string]struct {
		key, text string
		actor     audit.Actor
		want      error
	}{
		"unknown key": {"budget.scope_notice", "Câu.", messageStaff, domain.ErrUnknownMessageKey},
		"empty text":  {key, "  ", messageStaff, domain.ErrMessageTextEmpty},
		"markup":      {key, "<b>x</b>", messageStaff, domain.ErrMessageTextMarkup},
		"no actor":    {key, "Câu.", audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
			_, err := newMessagesUseCase(t, k, f).Reword(ctx, c.key, c.text, c.actor)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if k.batDau != 0 {
				t.Error("a refusal opened a transaction")
			}
		})
	}
}

func TestRestoreSoftDeletesAndAudits(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyFeedbackAssignmentRequired)
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	f.set(messageCommuneA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: messageNow, UpdatedBy: "CB-1"})

	got, err := newMessagesUseCase(t, k, f).Restore(inCommune(messageCommuneA), m.Key, messageStaff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != m.DefaultText || got.Overridden {
		t.Errorf("after restore %+v, want the default", got)
	}
	if f.deletes != 1 || f.lastBy != "CB-00123" || f.lastReason != RestoreReason || !f.lastAt.Equal(messageNow) {
		t.Errorf("soft delete by=%q reason=%q at=%v", f.lastBy, f.lastReason, f.lastAt)
	}
	if k.batDau != 1 || k.daCommit != 1 || len(f.txsSeen) != 1 {
		t.Errorf("begin=%d commit=%d txs=%d", k.batDau, k.daCommit, len(f.txsSeen))
	}
	d := messageAuditDelta(t, k)
	if d["truoc"]["text"] != "Câu của xã." || d["sau"]["text"] != m.DefaultText || d["sau"]["overridden"] != false {
		t.Errorf("delta = %v", d)
	}
	if a := k.cau("INSERT INTO audit_log")[0].args[4]; a != ActionRestoreSystemMessage {
		t.Errorf("action = %v", a)
	}
}

func TestRestoreOnDefaultWritesNothing(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).Restore(inCommune(messageCommuneA), domain.KeyFeedbackReasonRequired, messageStaff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.deletes != 0 || k.coCau("audit_log") {
		t.Error("restoring a key already on the default wrote something")
	}
}

func TestRestoreUnknownKey(t *testing.T) {
	k, f := &khoLoaiNhiemVuGia{}, newOverrideFake()
	_, err := newMessagesUseCase(t, k, f).Restore(inCommune(messageCommuneA), "report.title", messageStaff)
	if !errors.Is(err, domain.ErrUnknownMessageKey) || k.batDau != 0 {
		t.Errorf("err = %v, begin = %d", err, k.batDau)
	}
}
