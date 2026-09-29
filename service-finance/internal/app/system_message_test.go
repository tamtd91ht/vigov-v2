package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The use case runs over a REAL core/store transaction on the recording driver (fakeStore, in
// fake_driver_catalogue_test.go), so "the audit entry is in the same transaction" and "a failure
// rolls the write back" are properties of real Begin/Commit/Rollback calls. The override store is
// an in-memory fake that records WHICH TRANSACTION each write arrived in and WHICH COMMUNE — the
// store's own SQL is exercised by nothing here, and says so.

type overrideFake struct {
	live map[tenant.ID]*domain.MessageOverride // at most one live row per key; one key today

	listErr error
	addErr  error

	adds, updates, deletes int
	lastTx                 *store.ScopedTx
	txsSeen                map[*store.ScopedTx]bool
	lastBy, lastReason     string
	lastAt                 time.Time
}

func newOverrideFake() *overrideFake {
	return &overrideFake{live: map[tenant.ID]*domain.MessageOverride{}, txsSeen: map[*store.ScopedTx]bool{}}
}

func (f *overrideFake) seen(tx *store.ScopedTx) { f.lastTx = tx; f.txsSeen[tx] = true }

func (f *overrideFake) ListLive(ctx context.Context) ([]domain.MessageOverride, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if o := f.live[tenant.MustFrom(ctx)]; o != nil {
		return []domain.MessageOverride{*o}, nil
	}
	return nil, nil
}

func (f *overrideFake) LiveForUpdate(_ context.Context, tx *store.ScopedTx, key string) (*domain.MessageOverride, error) {
	f.seen(tx)
	if o := f.live[tx.TenantID()]; o != nil && o.Key == key {
		c := *o
		return &c, nil
	}
	return nil, nil
}

func (f *overrideFake) AddOverride(_ context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	f.seen(tx)
	f.adds++
	if f.addErr != nil {
		return f.addErr
	}
	f.live[tx.TenantID()] = &o
	return nil
}

func (f *overrideFake) UpdateText(_ context.Context, tx *store.ScopedTx, o domain.MessageOverride) error {
	f.seen(tx)
	f.updates++
	f.live[tx.TenantID()] = &o
	return nil
}

func (f *overrideFake) SoftDelete(_ context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	f.seen(tx)
	f.deletes++
	f.lastBy, f.lastReason, f.lastAt = by, reason, at
	delete(f.live, tx.TenantID())
	return nil
}

var fixedNow = time.Date(2026, 9, 29, 2, 30, 0, 0, time.UTC)

func newMessagesUseCase(t *testing.T, k *fakeStore, f *overrideFake) *SystemMessages {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	uc := NewSystemMessages(store.New(db), f)
	uc.newID = func() (string, error) { return "01JOVERRIDEID0000000000000", nil }
	uc.now = func() time.Time { return fixedNow }
	return uc
}

var staff = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func auditDelta(t *testing.T, k *fakeStore) map[string]map[string]any {
	t.Helper()
	ins := k.stmtsContaining("INSERT INTO audit_log")
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

func TestMessagesFallsBackToDefaultWithoutOverride(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := tenant.Into(context.Background(), tenantA)

	got, err := uc.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	if len(got) != 1 || got[0].CurrentText != m.DefaultText || got[0].Overridden {
		t.Fatalf("got %+v, want the shipped default", got)
	}

	// Commune B's wording never reaches commune A.
	f.live[tenantB] = &domain.MessageOverride{ID: "b", Key: m.Key, Text: "Câu xã B.", UpdatedAt: fixedNow, UpdatedBy: "CB-9"}
	got, _ = uc.Messages(ctx)
	if got[0].CurrentText != m.DefaultText {
		t.Errorf("commune A reads %q — another commune's wording", got[0].CurrentText)
	}
	got, _ = uc.Messages(tenant.Into(context.Background(), tenantB))
	if got[0].CurrentText != "Câu xã B." || !got[0].Overridden || got[0].UpdatedBy != "CB-9" {
		t.Errorf("commune B: %+v", got[0])
	}
}

func TestMessagesStoreFailureIsAnErrorNotTheDefault(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	f.listErr = errors.New("db down")
	uc := newMessagesUseCase(t, k, f)
	if _, err := uc.Messages(tenant.Into(context.Background(), tenantA)); err == nil {
		t.Fatal("a failed read returned the default — the commune would see words it replaced")
	}
}

func TestRewordWritesRowAndAuditInOneTransaction(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := tenant.Into(context.Background(), tenantA)

	got, err := uc.Reword(ctx, domain.KeyBudgetScopeNotice, "  Câu của xã A.  ", staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != "Câu của xã A." || !got.Overridden || got.UpdatedBy != "CB-00123" ||
		got.UpdatedAt == nil || !got.UpdatedAt.Equal(fixedNow) {
		t.Errorf("result %+v", got)
	}
	if f.adds != 1 || f.updates != 0 {
		t.Errorf("adds=%d updates=%d, want one insert", f.adds, f.updates)
	}
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 || len(f.txsSeen) != 1 {
		t.Errorf("begin=%d commit=%d rollback=%d txs=%d — want ONE committed transaction",
			k.begins, k.commits, k.rollbacks, len(f.txsSeen))
	}
	if f.lastTx.TenantID() != tenantA {
		t.Errorf("written in commune %q", f.lastTx.TenantID())
	}
	d := auditDelta(t, k)
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	if d["truoc"]["text"] != m.DefaultText || d["truoc"]["overridden"] != false ||
		d["sau"]["text"] != "Câu của xã A." || d["sau"]["overridden"] != true {
		t.Errorf("delta = %v, want before=default after=new text", d)
	}
	if subj := k.stmtsContaining("INSERT INTO audit_log")[0].args[5]; subj != domain.KeyBudgetScopeNotice {
		t.Errorf("subject = %v, want the message key", subj)
	}

	// Second wording: UPDATE of the same row, before = first wording.
	k2 := &fakeStore{}
	uc2 := newMessagesUseCase(t, k2, f)
	if _, err := uc2.Reword(ctx, domain.KeyBudgetScopeNotice, "Câu thứ hai.", staff); err != nil {
		t.Fatal(err)
	}
	if f.adds != 1 || f.updates != 1 {
		t.Errorf("adds=%d updates=%d, want the live row updated in place", f.adds, f.updates)
	}
	if d := auditDelta(t, k2); d["truoc"]["text"] != "Câu của xã A." || d["sau"]["text"] != "Câu thứ hai." {
		t.Errorf("delta = %v", d)
	}
}

func TestRewordAuditFailureRollsBackTheWrite(t *testing.T) {
	k, f := &fakeStore{failOnSQL: "INSERT INTO audit_log"}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	if _, err := uc.Reword(tenant.Into(context.Background(), tenantA), domain.KeyBudgetScopeNotice, "Câu.", staff); err == nil {
		t.Fatal("audit failure reported success")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Errorf("commit=%d rollback=%d — the wording must not commit without its entry", k.commits, k.rollbacks)
	}
}

func TestRewordNoOpWritesNothing(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	ctx := tenant.Into(context.Background(), tenantA)

	// Sending the default while on the default: no row pinned, no entry.
	k, f := &fakeStore{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, m.DefaultText, staff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.adds+f.updates != 0 || k.hasStmt("audit_log") {
		t.Error("the default was pinned into a row, or an entry saying nothing changed was filed")
	}

	// Sending the current override again.
	f.live[tenantA] = &domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"}
	k = &fakeStore{}
	if _, err := newMessagesUseCase(t, k, f).Reword(ctx, m.Key, "Câu.", staff); err != nil {
		t.Fatal(err)
	}
	if f.adds+f.updates != 0 || k.hasStmt("audit_log") {
		t.Error("same text re-sent wrote something")
	}
}

func TestRewordRefusesBeforeAnyTransaction(t *testing.T) {
	ctx := tenant.Into(context.Background(), tenantA)
	for name, c := range map[string]struct {
		key, text string
		actor     audit.Actor
		want      error
	}{
		"unknown key": {"feedback.reason_required", "Câu.", staff, domain.ErrUnknownMessageKey},
		"empty text":  {domain.KeyBudgetScopeNotice, "  ", staff, domain.ErrMessageTextEmpty},
		"markup":      {domain.KeyBudgetScopeNotice, "<b>x</b>", staff, domain.ErrMessageTextMarkup},
		"no actor":    {domain.KeyBudgetScopeNotice, "Câu.", audit.Actor{Kind: "staff"}, domain.ErrMessageActorMissing},
	} {
		t.Run(name, func(t *testing.T) {
			k, f := &fakeStore{}, newOverrideFake()
			_, err := newMessagesUseCase(t, k, f).Reword(ctx, c.key, c.text, c.actor)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if k.begins != 0 {
				t.Error("a refusal opened a transaction")
			}
		})
	}
}

func TestRestoreSoftDeletesAndAudits(t *testing.T) {
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	ctx := tenant.Into(context.Background(), tenantA)
	k, f := &fakeStore{}, newOverrideFake()
	f.live[tenantA] = &domain.MessageOverride{ID: "o", Key: m.Key, Text: "Câu của xã.", UpdatedAt: fixedNow, UpdatedBy: "CB-1"}

	got, err := newMessagesUseCase(t, k, f).Restore(ctx, m.Key, staff)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentText != m.DefaultText || got.Overridden {
		t.Errorf("after restore %+v, want the default", got)
	}
	if f.deletes != 1 || f.lastBy != "CB-00123" || f.lastReason != RestoreReason || !f.lastAt.Equal(fixedNow) {
		t.Errorf("soft delete by=%q reason=%q at=%v", f.lastBy, f.lastReason, f.lastAt)
	}
	if k.begins != 1 || k.commits != 1 || len(f.txsSeen) != 1 {
		t.Errorf("begin=%d commit=%d txs=%d", k.begins, k.commits, len(f.txsSeen))
	}
	d := auditDelta(t, k)
	if d["truoc"]["text"] != "Câu của xã." || d["sau"]["text"] != m.DefaultText || d["sau"]["overridden"] != false {
		t.Errorf("delta = %v", d)
	}
	if a := k.stmtsContaining("INSERT INTO audit_log")[0].args[4]; a != ActionRestoreSystemMessage {
		t.Errorf("action = %v", a)
	}
}

func TestRestoreOnDefaultWritesNothing(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	got, err := newMessagesUseCase(t, k, f).Restore(tenant.Into(context.Background(), tenantA), domain.KeyBudgetScopeNotice, staff)
	if err != nil || got.Overridden {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.deletes != 0 || k.hasStmt("audit_log") {
		t.Error("restoring a key already on the default wrote something")
	}
}

func TestRestoreUnknownKey(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	_, err := newMessagesUseCase(t, k, f).Restore(tenant.Into(context.Background(), tenantA), "report.title", staff)
	if !errors.Is(err, domain.ErrUnknownMessageKey) || k.begins != 0 {
		t.Errorf("err = %v, begin = %d", err, k.begins)
	}
}

func TestTextResolvesOneKeyForTheRequestsCommune(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	m, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)

	// A commune that reworded nothing reads the shipped sentence.
	got, err := uc.Text(tenant.Into(context.Background(), tenantA), domain.KeyBudgetScopeNotice)
	if err != nil || got != m.DefaultText {
		t.Fatalf("no override: got %q, %v — want the default", got, err)
	}

	// Commune A's own wording reaches A; commune B, which reworded nothing, still reads the default.
	f.live[tenantA] = &domain.MessageOverride{ID: "o1", Key: domain.KeyBudgetScopeNotice, Text: "Câu riêng của xã A."}
	if got, _ := uc.Text(tenant.Into(context.Background(), tenantA), domain.KeyBudgetScopeNotice); got != "Câu riêng của xã A." {
		t.Errorf("commune A reads %q, want its own wording", got)
	}
	if got, _ := uc.Text(tenant.Into(context.Background(), tenantB), domain.KeyBudgetScopeNotice); got != m.DefaultText {
		t.Errorf("commune B reads %q, want the default — another commune's wording leaked", got)
	}
}

func TestTextRefusesUnknownKeyAndStoreFailure(t *testing.T) {
	k, f := &fakeStore{}, newOverrideFake()
	uc := newMessagesUseCase(t, k, f)
	ctx := tenant.Into(context.Background(), tenantA)

	if _, err := uc.Text(ctx, "feedback.reason_required"); !errors.Is(err, domain.ErrUnknownMessageKey) {
		t.Errorf("another service's key: err = %v, want ErrUnknownMessageKey", err)
	}
	// A STORE FAILURE IS AN ERROR, NOT THE DEFAULT.
	f.listErr = errors.New("cơ sở dữ liệu không phản hồi")
	if got, err := uc.Text(ctx, domain.KeyBudgetScopeNotice); err == nil || got != "" {
		t.Errorf("store failure: got %q, %v — want an error and no sentence", got, err)
	}
}
