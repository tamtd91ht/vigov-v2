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
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// The use case runs over a REAL core/store transaction on recordingDriver, so "the audit entry is in
// the same transaction" and "a failure rolls the write back" are properties of real
// Begin/Commit/Rollback calls. The override store is an in-memory fake that records WHICH
// TRANSACTION each write arrived in and WHICH COMMUNE — the store's own SQL is exercised by nothing
// here, and says so.

var (
	communeA = tenant.ID("01JA" + strings.Repeat("A", 22))
	communeB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

const titleKey = "report.title"

// overrideFake holds live rows keyed by commune, then by message key.
type overrideFake struct {
	live map[tenant.ID]map[string]*domain.MessageOverride

	listErr error

	adds, updates, deletes int
	lastTx                 *store.ScopedTx
	txsSeen                map[*store.ScopedTx]bool
	lastBy, lastReason     string
	lastAt                 time.Time
}

func newOverrideFake() *overrideFake {
	return &overrideFake{live: map[tenant.ID]map[string]*domain.MessageOverride{}, txsSeen: map[*store.ScopedTx]bool{}}
}

func (f *overrideFake) put(t tenant.ID, o domain.MessageOverride) {
	if f.live[t] == nil {
		f.live[t] = map[string]*domain.MessageOverride{}
	}
	f.live[t][o.Key] = &o
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
	f.put(tx.TenantID(), o)
	return nil
}

func (f *overrideFake) UpdateText(_ context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	f.seen(tx)
	f.updates++
	f.put(tx.TenantID(), o)
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

var fixedNow = time.Date(2026, 9, 29, 2, 30, 0, 0, time.UTC)

func newMessagesUseCase(t *testing.T, d *recordingDriver, f *overrideFake) *SystemMessages {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	uc := NewSystemMessages(store.New(db), f)
	uc.newID = func() (string, error) { return "01JOVERRIDEID0000000000000", nil }
	uc.now = func() time.Time { return fixedNow }
	return uc
}

var staff = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

// auditEntry returns the one audit INSERT and its decoded delta. Argument positions are
// core/audit.Write's: tenant, actor_id, actor_kind, actor_ip, action, subject, at, delta.
func auditEntry(t *testing.T, d *recordingDriver) (recordedStmt, map[string]map[string]any) {
	t.Helper()
	ins := d.matching("INSERT INTO audit_log")
	if len(ins) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(ins))
	}
	args := ins[0].args
	if args[1] != "CB-00123" {
		t.Errorf("actor_id = %v, want the staff business code", args[1])
	}
	var delta map[string]map[string]any
	if err := json.Unmarshal(args[7].([]byte), &delta); err != nil {
		t.Fatal(err)
	}
	return ins[0], delta
}

func TestMessagesListsAll38WithDefaultsAndIsolatesCommunes(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	uc := newMessagesUseCase(t, d, f)
	ctxA := tenant.Into(context.Background(), communeA)

	got, err := uc.Messages(ctxA)
	if err != nil {
		t.Fatal(err)
	}
	shipped := domain.ShippedMessages()
	if len(got) != 38 || len(shipped) != 38 {
		t.Fatalf("listed %d, shipped %d — want 38", len(got), len(shipped))
	}
	for i, m := range got {
		if m.Key != shipped[i].Key || m.CurrentText != shipped[i].DefaultText || m.DefaultText != shipped[i].DefaultText || m.Overridden {
			t.Errorf("row %d = %+v, want the shipped default of %s", i, m, shipped[i].Key)
		}
	}

	// Commune B's wording never reaches commune A.
	f.put(communeB, domain.MessageOverride{ID: "b", Key: titleKey, Text: "Báo cáo xã B.", UpdatedAt: fixedNow, UpdatedBy: "CB-9"})
	got, _ = uc.Messages(ctxA)
	if got[0].CurrentText != "Báo cáo điều hành" || got[0].Overridden {
		t.Errorf("commune A reads %q — another commune's wording", got[0].CurrentText)
	}
	got, _ = uc.Messages(tenant.Into(context.Background(), communeB))
	if got[0].CurrentText != "Báo cáo xã B." || !got[0].Overridden || got[0].UpdatedBy != "CB-9" {
		t.Errorf("commune B: %+v", got[0])
	}
	if got[1].Overridden {
		t.Errorf("commune B's override of %s spread to %s", titleKey, got[1].Key)
	}
}

func TestMessagesStoreFailureIsAnErrorNotTheDefault(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	f.listErr = errors.New("db down")
	uc := newMessagesUseCase(t, d, f)
	if _, err := uc.Messages(tenant.Into(context.Background(), communeA)); err == nil {
		t.Fatal("a failed read returned the default — the commune would see words it replaced")
	}
}

func TestRewordWritesRowAndAuditInOneTransaction(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	uc := newMessagesUseCase(t, d, f)
	ctx := tenant.Into(context.Background(), communeA)

	got, err := uc.Reword(ctx, titleKey, "  Báo cáo của xã A.  ", staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != "Báo cáo của xã A." || !got.Overridden || got.UpdatedBy != "CB-00123" ||
		got.UpdatedAt == nil || !got.UpdatedAt.Equal(fixedNow) {
		t.Errorf("result %+v", got)
	}
	if f.adds != 1 || f.updates != 0 {
		t.Errorf("adds=%d updates=%d, want one insert", f.adds, f.updates)
	}
	if d.begins != 1 || d.commits != 1 || d.rollbacks != 0 || len(f.txsSeen) != 1 {
		t.Errorf("begin=%d commit=%d rollback=%d txs=%d — want ONE committed transaction",
			d.begins, d.commits, d.rollbacks, len(f.txsSeen))
	}
	if f.lastTx.TenantID() != communeA {
		t.Errorf("written in commune %q", f.lastTx.TenantID())
	}
	ins, delta := auditEntry(t, d)
	if delta["truoc"]["text"] != "Báo cáo điều hành" || delta["truoc"]["overridden"] != false ||
		delta["sau"]["text"] != "Báo cáo của xã A." || delta["sau"]["overridden"] != true {
		t.Errorf("delta = %v, want before=default after=new text", delta)
	}
	if ins.args[0] != string(communeA) || ins.args[4] != ActionRewordSystemMessage || ins.args[5] != titleKey {
		t.Errorf("entry tenant=%v action=%v subject=%v", ins.args[0], ins.args[4], ins.args[5])
	}

	// Second wording: UPDATE of the same row, before = first wording.
	d2 := &recordingDriver{}
	if _, err := newMessagesUseCase(t, d2, f).Reword(ctx, titleKey, "Báo cáo thứ hai.", staff); err != nil {
		t.Fatal(err)
	}
	if f.adds != 1 || f.updates != 1 {
		t.Errorf("adds=%d updates=%d, want the live row updated in place", f.adds, f.updates)
	}
	if _, delta := auditEntry(t, d2); delta["truoc"]["text"] != "Báo cáo của xã A." || delta["sau"]["text"] != "Báo cáo thứ hai." {
		t.Errorf("delta = %v", delta)
	}
}

func TestRewordAuditFailureRollsBackTheWrite(t *testing.T) {
	d, f := &recordingDriver{failOn: "INSERT INTO audit_log"}, newOverrideFake()
	uc := newMessagesUseCase(t, d, f)
	if _, err := uc.Reword(tenant.Into(context.Background(), communeA), titleKey, "Báo cáo.", staff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if d.commits != 0 || d.rollbacks != 1 {
		t.Errorf("commit=%d rollback=%d — the wording must not commit without its entry", d.commits, d.rollbacks)
	}
}

func TestRewordNoOpWritesNothing(t *testing.T) {
	m, _ := domain.LookupShippedMessage(titleKey)
	ctx := tenant.Into(context.Background(), communeA)

	// Sending the default while on the default: no row pinned, no entry.
	d, f := &recordingDriver{}, newOverrideFake()
	got, err := newMessagesUseCase(t, d, f).Reword(ctx, m.Key, m.DefaultText, staff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.adds+f.updates != 0 || d.has("audit_log") {
		t.Error("the default was pinned into a row, or an entry saying nothing changed was filed")
	}

	// Sending the current override again.
	f.put(communeA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Báo cáo.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"})
	d = &recordingDriver{}
	if _, err := newMessagesUseCase(t, d, f).Reword(ctx, m.Key, "Báo cáo.", staff); err != nil {
		t.Fatal(err)
	}
	if f.adds+f.updates != 0 || d.has("audit_log") {
		t.Error("same text re-sent wrote something")
	}
}

func TestRewordRefusesBeforeAnyTransaction(t *testing.T) {
	ctx := tenant.Into(context.Background(), communeA)
	for name, c := range map[string]struct {
		key, text string
		actor     audit.Actor
		want      error
	}{
		"unknown key":         {"feedback.reason_required", "Câu.", staff, domain.ErrUnknownMessageKey},
		"another service key": {"budget.scope_notice", "Câu.", staff, domain.ErrUnknownMessageKey},
		"empty text":          {titleKey, "  ", staff, domain.ErrMessageTextEmpty},
		"markup":              {titleKey, "<b>x</b>", staff, domain.ErrMessageTextMarkup},
		"no actor":            {titleKey, "Câu.", audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			d, f := &recordingDriver{}, newOverrideFake()
			_, err := newMessagesUseCase(t, d, f).Reword(ctx, c.key, c.text, c.actor)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if d.begins != 0 {
				t.Error("a refusal opened a transaction")
			}
		})
	}
}

func TestRestoreSoftDeletesAndAuditsInOneTransaction(t *testing.T) {
	m, _ := domain.LookupShippedMessage(titleKey)
	ctx := tenant.Into(context.Background(), communeA)
	d, f := &recordingDriver{}, newOverrideFake()
	f.put(communeA, domain.MessageOverride{ID: "o", Key: m.Key, Text: "Báo cáo của xã.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"})
	// Commune B's wording of the same key must survive commune A's restore.
	f.put(communeB, domain.MessageOverride{ID: "b", Key: m.Key, Text: "Báo cáo xã B.", UpdatedAt: fixedNow, UpdatedBy: "CB-9"})

	got, err := newMessagesUseCase(t, d, f).Restore(ctx, m.Key, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != m.DefaultText || got.Overridden {
		t.Errorf("after restore %+v, want the default", got)
	}
	if f.deletes != 1 || f.lastBy != "CB-00123" || f.lastReason != RestoreReason || !f.lastAt.Equal(fixedNow) {
		t.Errorf("soft delete by=%q reason=%q at=%v", f.lastBy, f.lastReason, f.lastAt)
	}
	if d.begins != 1 || d.commits != 1 || len(f.txsSeen) != 1 || f.lastTx.TenantID() != communeA {
		t.Errorf("begin=%d commit=%d txs=%d", d.begins, d.commits, len(f.txsSeen))
	}
	ins, delta := auditEntry(t, d)
	if delta["truoc"]["text"] != "Báo cáo của xã." || delta["sau"]["text"] != m.DefaultText || delta["sau"]["overridden"] != false {
		t.Errorf("delta = %v", delta)
	}
	if ins.args[4] != ActionRestoreSystemMessage {
		t.Errorf("action = %v", ins.args[4])
	}
	if f.live[communeB][m.Key] == nil {
		t.Error("commune A's restore removed commune B's wording")
	}

	// After restore the list reads the default again for A.
	list, _ := newMessagesUseCase(t, &recordingDriver{}, f).Messages(ctx)
	if list[0].Overridden || list[0].CurrentText != m.DefaultText {
		t.Errorf("list after restore: %+v", list[0])
	}
}

func TestRestoreAuditFailureRollsBack(t *testing.T) {
	d, f := &recordingDriver{failOn: "INSERT INTO audit_log"}, newOverrideFake()
	f.put(communeA, domain.MessageOverride{ID: "o", Key: titleKey, Text: "Báo cáo.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"})
	if _, err := newMessagesUseCase(t, d, f).Restore(tenant.Into(context.Background(), communeA), titleKey, staff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if d.commits != 0 || d.rollbacks != 1 {
		t.Errorf("commit=%d rollback=%d", d.commits, d.rollbacks)
	}
}

func TestRestoreOnDefaultWritesNothing(t *testing.T) {
	d, f := &recordingDriver{}, newOverrideFake()
	got, err := newMessagesUseCase(t, d, f).Restore(tenant.Into(context.Background(), communeA), titleKey, staff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.deletes != 0 || d.has("audit_log") {
		t.Error("restoring a key already on the default wrote something")
	}
}

func TestRestoreRefusesBeforeAnyTransaction(t *testing.T) {
	ctx := tenant.Into(context.Background(), communeA)
	for name, c := range map[string]struct {
		key   string
		actor audit.Actor
		want  error
	}{
		"unknown key": {"budget.scope_notice", staff, domain.ErrUnknownMessageKey},
		"no actor":    {titleKey, audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			d, f := &recordingDriver{}, newOverrideFake()
			_, err := newMessagesUseCase(t, d, f).Restore(ctx, c.key, c.actor)
			if !errors.Is(err, c.want) || d.begins != 0 {
				t.Errorf("err = %v, begin = %d", err, d.begins)
			}
		})
	}
}
