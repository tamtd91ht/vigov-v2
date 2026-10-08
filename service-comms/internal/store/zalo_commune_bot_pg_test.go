package store

import (
	"bytes"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The commune bot's SQL (migration 0022), the mail test result (0019) and the catalogue colour (0020)
// against a REAL PostgreSQL: column spellings, the generated bot_ref, the unique keys, the deferred
// checks the use case must satisfy in one transaction. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run
// without the DSN means this file COMPILES, nothing more.

// fakeSealed is 40 bytes: past 0022's "envelope overhead + one byte" floor. Not a real seal — the
// store never opens anything.
var fakeSealed = bytes.Repeat([]byte{7}, 40)

func adoptBot(t *testing.T, h *pkgstore.DB, bots *ZaloCommuneBotStore, links *ZaloLinkStore, commune, id, account string,
	endShared bool) error {
	t.Helper()
	ctx := ctxXa(tenant.ID(commune))
	return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if endShared {
			if _, err := links.EndLiveLinksOfBot(ctx, tx, domain.SharedZaloBotRef, "CB-00123",
				domain.ZaloLinkEndedByBotChange, zaloAt); err != nil {
				return err
			}
		}
		return bots.InsertLive(ctx, tx, domain.CommuneZaloBot{ID: id, BotAccountID: account, BotName: "Bot Xã",
			ChatURL: "https://zalo.me/1", SetAt: zaloAt, SetBy: "CB-00123"}, fakeSealed)
	})
}

func TestPgCommuneBotAdoptEndsSharedLinksOrTheCommitFails(t *testing.T) {
	h, links, z, c1, c2 := zaloRig(t)
	bots := NewZaloCommuneBotStore(h)
	pairStaff(t, h, links, z, c1, "CB-00001", "chat-shared-1", "01JLINKPG00000000000000001")

	// Forgetting to end the shared links fails at COMMIT (0022 zalo_commune_bot_adopt_check).
	if err := adoptBot(t, h, bots, links, c1, "01JBOTPG000000000000000001", "pg-bot-1", false); err == nil {
		t.Fatal("an own bot was adopted with a live shared link left")
	}
	if err := adoptBot(t, h, bots, links, c1, "01JBOTPG000000000000000001", "pg-bot-1", true); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	ctx := ctxXa(tenant.ID(c1))
	b, found, err := bots.Live(ctx)
	if err != nil || !found || b.Ref() != "commune:pg-bot-1" || b.LastCheckResult != domain.ZaloCallOK {
		t.Fatalf("live = %+v %v %v", b, found, err)
	}
	// The same account in ANOTHER commune: one generic error, nothing about where.
	err = adoptBot(t, h, bots, links, c2, "01JBOTPG000000000000000002", "pg-bot-1", true)
	if !errors.Is(err, ErrCommuneZaloBotAccountTaken) {
		t.Fatalf("second commune, same account: %v", err)
	}
	if _, found, _ := bots.Live(ctxXa(tenant.ID(c2))); found {
		t.Fatal("commune 2 sees a bot")
	}
}

func TestPgCommuneBotWebhookSlotsAndRetire(t *testing.T) {
	h, links, _, c1, _ := zaloRig(t)
	bots := NewZaloCommuneBotStore(h)
	if err := adoptBot(t, h, bots, links, c1, "01JBOTPG000000000000000011", "pg-bot-11", true); err != nil {
		t.Fatal(err)
	}
	ctx := ctxXa(tenant.ID(c1))
	id := "01JBOTPG000000000000000011"
	pending := bytes.Repeat([]byte{9}, 40)
	run := func(fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }

	if err := run(func(tx *pkgstore.ScopedTx) error {
		return bots.SetPendingWebhookSecret(ctx, tx, id, pending, zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	// The slot is taken: a second pending secret is refused, never overwritten.
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return bots.SetPendingWebhookSecret(ctx, tx, id, fakeSealed, zaloAt)
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("second pending: %v", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return bots.PromotePendingWebhookSecret(ctx, tx, id, pending, "CB-00123", zaloAt.Add(time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
	s, found, err := bots.Sealed(ctx)
	if err != nil || !found || !bytes.Equal(s.WebhookSecretSealed, pending) || s.PendingSecretSealed != nil ||
		s.Bot.WebhookSetBy != "CB-00123" {
		t.Fatalf("after promote: %+v %v", s.Bot, err)
	}
	// Retire without ending the bot's links fails at COMMIT (0022 retire check) once a link exists.
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return links.InsertLink(ctx, tx, "01JLINKPG00000000000000011", "CB-00002", "commune:pg-bot-11", "chat-own-1", zaloAt)
	}); err != nil {
		t.Fatalf("pair through the own bot: %v", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return bots.Retire(ctx, tx, id, "CB-00123", "Thử quay về bot chung", zaloAt.Add(time.Hour))
	}); err == nil {
		t.Fatal("a bot was retired with a live link left")
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		if err := bots.Retire(ctx, tx, id, "CB-00123", "Thử quay về bot chung", zaloAt.Add(time.Hour)); err != nil {
			return err
		}
		ended, err := links.EndLiveLinksOfBot(ctx, tx, "commune:pg-bot-11", "CB-00123", domain.ZaloLinkEndedByBotChange,
			zaloAt.Add(time.Hour))
		if err == nil && (len(ended) != 1 || ended[0].BotRef != "commune:pg-bot-11") {
			t.Errorf("ended = %+v", ended)
		}
		return err
	}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, found, _ := bots.Live(ctx); found {
		t.Fatal("a retired bot is still live")
	}
	// D2: the webhook authenticates through Sealed — a retired row's secrets (in force: `pending`, promoted
	// above) are never read for comparison again.
	if s, found, err := bots.Sealed(ctx); err != nil || found {
		t.Fatalf("the retired row's sealed secrets are still readable: %+v %v", s.Bot, err)
	}
}

func TestPgMailTestResultAndColourColumns(t *testing.T) {
	db := moKetNoi(t)
	c1, _ := xaRieng(t)
	h := pkgstore.New(db)
	ctx := ctxXa(tenant.ID(c1))
	mail := NewMailSettingsStore(h)
	m := domain.MailSettings{Host: "smtp.example.test", Port: 587, Security: "starttls", Username: "u@example.test",
		FromAddress: "u@example.test"}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return mail.Upsert(ctx, tx, m, fakeSealed, "CB-00123")
	}); err != nil {
		t.Fatal(err)
	}
	r := domain.MailTestResult{At: zaloAt, ToMasked: "c***@example.test", ErrorClass: domain.MailTestErrorAuthRejected}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return mail.RecordTest(ctx, tx, r) }); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, err := mail.Get(ctx)
	if err != nil || got.LastTest == nil || got.LastTest.ErrorClass != domain.MailTestErrorAuthRejected || got.LastTest.OK {
		t.Fatalf("read back: %+v %v", got.LastTest, err)
	}
	// A raw address is refused by 0019's CHECK — the floor under the Go-side mask.
	raw := r
	raw.ToMasked = "canbo@example.test"
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return mail.RecordTest(ctx, tx, raw) }); err == nil {
		t.Fatal("an unmasked recipient was stored")
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return mail.ClearTest(ctx, tx) }); err != nil {
		t.Fatal(err)
	}
	if got, _ := mail.Get(ctx); got.LastTest != nil {
		t.Fatal("the test result survived a clear")
	}

	types := NewLoaiTaiNguyenBanDoStore(h)
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return types.Chen(ctx, tx, domain.LoaiTaiNguyenBanDo{ID: "01JTYPEPG00000000000000001", Ma: "mau-pg",
			Nhan: "Màu", DangDung: true, Color: "#1a2b3c"})
	}); err != nil {
		t.Fatal(err)
	}
	list, err := types.DanhSach(ctx)
	if err != nil || len(list) == 0 || list[0].Color != "#1a2b3c" {
		t.Fatalf("colour read back: %+v %v", list, err)
	}
}
