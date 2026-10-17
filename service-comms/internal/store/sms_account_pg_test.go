package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The SMS accounts' SQL (migration 0032) against a REAL PostgreSQL: column spellings, the two unique keys
// (live label, one default), the guard's demands (new keys need a new set_at; a retired row is frozen
// and never the default), the template upsert and vanish. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run
// without the DSN means this file COMPILES, nothing more.

var smsAt = time.Date(2026, 10, 17, 3, 0, 0, 0, time.UTC)

func smsRig(t *testing.T) (*pkgstore.DB, *SmsAccountStore, *SmsTemplateStore, context.Context, context.Context) {
	t.Helper()
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	h := pkgstore.New(db)
	return h, NewSmsAccountStore(h), NewSmsTemplateStore(h), ctxXa(tenant.ID(c1)), ctxXa(tenant.ID(c2))
}

func smsAccountRow(id, label string, isDefault bool) domain.SmsAccount {
	return domain.SmsAccount{ID: id, Provider: domain.SmsProviderEsms, Label: label, Brandname: "XaMau", IsDefault: isDefault,
		LastCheckAt: smsAt, LastCheckResult: domain.SmsCheckOK, LastBalance: "36693", SetAt: smsAt, SetBy: "CB-00123"}
}

func TestPgSmsAccountKeysGuardAndIsolation(t *testing.T) {
	h, accounts, _, ctx1, ctx2 := smsRig(t)
	run := func(ctx context.Context, fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }
	const id1, id2, id3 = "01JSMSPG000000000000000001", "01JSMSPG000000000000000002", "01JSMSPG000000000000000003"

	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(id1, "eSMS chính", true), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, found, err := accounts.Live(ctx1, id1)
	if err != nil || !found || got.Label != "eSMS chính" || !got.IsDefault || got.LastBalance != "36693.00" ||
		got.LastCheckResult != domain.SmsCheckOK || got.SetBy != "CB-00123" {
		t.Fatalf("live = %+v %v %v", got, found, err)
	}
	// Another commune sees nothing of it (rule 1).
	if _, found, _ := accounts.Live(ctx2, id1); found {
		t.Fatal("commune 2 reads commune 1's account")
	}
	if l, _ := accounts.ListLive(ctx2); len(l) != 0 {
		t.Fatal("commune 2 lists commune 1's account")
	}
	// The same label, other case, live: refused. In ANOTHER commune: fine.
	err = run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(id2, "ESMS CHÍNH", false), fakeSealed, fakeSealed)
	})
	if !errors.Is(err, ErrSmsLabelTaken) {
		t.Fatalf("same label: %v", err)
	}
	if err := run(ctx2, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx2, tx, smsAccountRow(id1, "eSMS chính", true), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("other commune, same id and label: %v", err)
	}
	// A second default without clearing the first: the key refuses it.
	err = run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(id2, "eSMS dự phòng", true), fakeSealed, fakeSealed)
	})
	if !errors.Is(err, ErrSmsAccountChanged) {
		t.Fatalf("second default: %v", err)
	}
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(id2, "eSMS dự phòng", false), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("insert second: %v", err)
	}
	// SetDefault moves the mark in one transaction.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error { return accounts.SetDefault(ctx1, tx, id2, smsAt.Add(time.Minute)) }); err != nil {
		t.Fatalf("set default: %v", err)
	}
	list, err := accounts.ListLive(ctx1)
	if err != nil || len(list) != 2 || list[0].ID != id2 || !list[0].IsDefault || list[1].IsDefault {
		t.Fatalf("list = %+v %v", list, err)
	}
	// New keys without a new set_at: the guard refuses (the store always passes `at` as set_at).
	err = run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.UpdateCredentials(ctx1, tx, id2, "eSMS dự phòng", "XaMau", bytes.Repeat([]byte{8}, 40), fakeSealed,
			"CB-00123", domain.SmsCheckOK, "1", smsAt)
	})
	if err == nil {
		t.Fatal("new keys with the old set_at accepted")
	}
	later := smsAt.Add(time.Hour)
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.UpdateCredentials(ctx1, tx, id2, "eSMS dự phòng", "XaMau2", bytes.Repeat([]byte{8}, 40), fakeSealed,
			"CB-00124", domain.SmsCheckOK, "120000.5", later)
	}); err != nil {
		t.Fatalf("update credentials: %v", err)
	}
	s, found, err := accounts.Sealed(ctx1, id2)
	if err != nil || !found || !bytes.Equal(s.APIKeySealed, bytes.Repeat([]byte{8}, 40)) || s.Account.Brandname != "XaMau2" ||
		!s.Account.SetAt.Equal(later) || s.Account.SetBy != "CB-00124" || s.Account.LastBalance != "120000.50" {
		t.Fatalf("after update: %+v %v", s.Account, err)
	}
	// A failed check clears the balance.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.RecordCheck(ctx1, tx, id2, domain.SmsCheckKeyRejected, "", later.Add(time.Minute))
	}); err != nil {
		t.Fatalf("record check: %v", err)
	}
	if a, _, _ := accounts.Live(ctx1, id2); a.LastBalance != "" || a.LastCheckResult != domain.SmsCheckKeyRejected {
		t.Fatalf("after failed check: %+v", a)
	}
	// Retire the default: it stops being live AND the default; no other account becomes it.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.Retire(ctx1, tx, id2, "CB-00123", "Hết hợp đồng", later.Add(2*time.Minute))
	}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	list, _ = accounts.ListLive(ctx1)
	if len(list) != 1 || list[0].ID != id1 || list[0].IsDefault {
		t.Fatalf("after retire: %+v", list)
	}
	// A retired row is frozen; its label is free again.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.UpdateLabel(ctx1, tx, id2, "x", later.Add(3*time.Minute))
	}); !errors.Is(err, ErrSmsAccountChanged) {
		t.Fatalf("label on a retired row: %v", err)
	}
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(id3, "eSMS dự phòng", false), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("reuse a retired label: %v", err)
	}
}

func TestPgSmsTemplateUpsertAndVanish(t *testing.T) {
	h, accounts, templates, ctx1, _ := smsRig(t)
	run := func(fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx1).Tx(ctx1, fn) }
	const acc = "01JSMSPGACC000000000000001"
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(acc, "eSMS", false), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatal(err)
	}
	tpl := domain.SmsTemplate{ID: "01JSMSPGTPL000000000000001", AccountID: acc, ProviderTemplateID: "458", NetworkID: 6,
		Content: "{P1,20} la ma", Params: []domain.SmsTemplateParam{{Name: "P1", MaxLen: 20}}}
	if err := run(func(tx *pkgstore.ScopedTx) error { return templates.UpsertFromSync(ctx1, tx, tpl, smsAt) }); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Same (TempId, network) again with a new id: the existing row is rewritten, no second row.
	tpl2 := tpl
	tpl2.ID, tpl2.Content = "01JSMSPGTPL000000000000002", "{P1,10} la ma moi"
	tpl2.Params = []domain.SmsTemplateParam{{Name: "P1", MaxLen: 10}}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return templates.UpsertFromSync(ctx1, tx, tpl2, smsAt.Add(time.Hour))
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := templates.ListCurrentByAccount(ctx1, acc)
	if err != nil || len(got) != 1 || got[0].ID != tpl.ID || got[0].Content != "{P1,10} la ma moi" || got[0].Params[0].MaxLen != 10 {
		t.Fatalf("list = %+v %v", got, err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return templates.MarkVanished(ctx1, tx, tpl.ID, smsAt.Add(2*time.Hour))
	}); err != nil {
		t.Fatalf("vanish: %v", err)
	}
	if got, _ := templates.ListCurrentByAccount(ctx1, acc); len(got) != 0 {
		t.Fatalf("a vanished template is still offered: %+v", got)
	}
	var locked []domain.SmsTemplate
	if err := run(func(tx *pkgstore.ScopedTx) error {
		var err error
		locked, err = templates.LockByAccount(ctx1, tx, acc)
		return err
	}); err != nil || len(locked) != 1 || locked[0].VanishedAt.IsZero() {
		t.Fatalf("lock = %+v %v", locked, err)
	}
	// It reappears: vanished_at cleared.
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return templates.UpsertFromSync(ctx1, tx, tpl2, smsAt.Add(3*time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := templates.ListCurrentByAccount(ctx1, acc); len(got) != 1 {
		t.Fatalf("a reappeared template is not offered: %+v", got)
	}
}
