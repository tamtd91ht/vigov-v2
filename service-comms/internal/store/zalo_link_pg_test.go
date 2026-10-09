package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
)

// The Zalo channel's SQL against a REAL PostgreSQL (migration 0018): the column spellings, the text[]
// round trip, the CASE of the enqueue, the partial unique indexes, the guards, and the cross-commune
// statements of crosstenant/zalo_bot.go. SKIPPED WITHOUT VIGOV_TEST_DSN (shared harness:
// loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN means this file COMPILES, nothing more.

func zaloRig(t *testing.T) (*pkgstore.DB, *ZaloLinkStore, *crosstenant.ZaloBot, string, string) {
	t.Helper()
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	h := pkgstore.New(db)
	return h, NewZaloLinkStore(h), crosstenant.NewZaloBot(db), c1, c2
}

var zaloAt = time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)

func TestPgZaloSettingsDefaultsAgreeWithTheColumnDefaults(t *testing.T) {
	h, s, _, c1, _ := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	got, err := s.ChannelSetting(ctx)
	if err != nil || got.Saved || got.IsEnabled {
		t.Fatalf("no row: %+v, %v", got, err)
	}
	// The Go defaults shown before the first save must equal 0018's column DEFAULTs.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := tx.Exec(ctx, `INSERT INTO zalo_channel_setting (tenant_id, updated_by) VALUES ($1, 'CB-1')`, c1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := s.ChannelSetting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.QuietStartMinute != got.QuietStartMinute || stored.QuietEndMinute != got.QuietEndMinute ||
		len(stored.Kinds) != 0 || stored.OverdueStartAfterDays != nil {
		t.Fatalf("DB defaults %+v ≠ Go defaults %+v", stored, got)
	}
}

func TestPgZaloSettingsRoundTripAndIsolation(t *testing.T) {
	h, s, _, c1, c2 := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	five, two := 5, 2
	in := domain.ZaloChannelSetting{IsEnabled: true, Kinds: []string{"nhiem-vu.sap-den-han", "van-ban.qua-han"},
		QuietStartMinute: 22*60 + 30, QuietEndMinute: 5 * 60, OverdueStartAfterDays: &five, OverdueRepeatEveryDays: &two}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if _, err := s.LockChannelSetting(ctx, tx); err != nil {
			return err
		}
		return s.SaveChannelSetting(ctx, tx, in, "CB-00123", zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ChannelSetting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Saved || !got.Same(in) || got.UpdatedBy != "CB-00123" {
		t.Fatalf("round trip: %+v", got)
	}
	if other, _ := s.ChannelSetting(ctxXa(tenant.ID(c2))); other.Saved {
		t.Error("commune 2 reads commune 1's settings")
	}
	// 0021: a row still holding an OLD value is read as the per-domain kinds it means.
	legacy := in
	legacy.Kinds = []string{"qua-han"}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SaveChannelSetting(ctx, tx, legacy, "CB-00123", zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	if got, err = s.ChannelSetting(ctx); err != nil || !got.LegacyKindsStored || len(got.Kinds) != 6 ||
		!got.KindEnabled("phan-anh.chua-cu-nguoi") {
		t.Fatalf("legacy read: %+v %v", got, err)
	}
	// The CHECK backs the domain: enabled with no kind is refused by the database too.
	err = h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SaveChannelSetting(ctx, tx, domain.ZaloChannelSetting{IsEnabled: true, Kinds: []string{},
			QuietStartMinute: 1, QuietEndMinute: 2}, "CB-1", zaloAt)
	})
	if err == nil {
		t.Error("an enabled channel with no kind was stored")
	}
}

// pairStaff runs the pairing steps the webhook runs, inside one commune's transaction.
func pairStaff(t *testing.T, h *pkgstore.DB, s *ZaloLinkStore, z *crosstenant.ZaloBot, commune, staff, chat, linkID string) int {
	t.Helper()
	ctx := ctxXa(tenant.ID(commune))
	ended := 0
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		n, err := z.EndChatLinksInOtherCommunes(ctx, tx, chat, zaloAt)
		if err != nil {
			return err
		}
		ended = n
		if l, found, err := s.LockLiveLinkOfChat(ctx, tx, domain.SharedZaloBotRef, chat); err != nil {
			return err
		} else if found {
			if err := s.EndLink(ctx, tx, l.ID, staff, domain.ZaloLinkEndedByChatTaken, zaloAt); err != nil {
				return err
			}
		}
		if l, found, err := s.LockLiveLinkOfStaff(ctx, tx, staff); err != nil {
			return err
		} else if found {
			if err := s.EndLink(ctx, tx, l.ID, staff, domain.ZaloLinkEndedByRepair, zaloAt); err != nil {
				return err
			}
		}
		return s.InsertLink(ctx, tx, linkID, staff, domain.SharedZaloBotRef, chat, zaloAt)
	}); err != nil {
		t.Fatalf("pair %s/%s: %v", commune, staff, err)
	}
	return ended
}

func TestPgZaloPairingCodeLifecycleAndCrossCommuneLookup(t *testing.T) {
	h, s, z, c1, c2 := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	hash := domain.PairingCodeHash("ABCD2345")
	insert := func(commune, id, staff string) error {
		cctx := ctxXa(tenant.ID(commune))
		return h.For(cctx).Tx(cctx, func(tx *pkgstore.ScopedTx) error {
			if _, err := s.CancelOpenPairingCodes(cctx, tx, staff, zaloAt); err != nil {
				return err
			}
			return s.InsertPairingCode(cctx, tx, id, staff, hash, zaloAt.Add(domain.PairingCodeTTL), zaloAt)
		})
	}
	if err := insert(c1, "code-1", "CB-00001"); err != nil {
		t.Fatal(err)
	}
	m, err := z.OpenPairingCodesByHash(context.Background(), hash)
	if err != nil || len(m) != 1 || string(m[0].TenantID) != c1 || m[0].CodeID != "code-1" {
		t.Fatalf("lookup = %+v, %v", m, err)
	}
	// The same hash again in the same commune: refused, and the savepoint kept the transaction usable.
	if err := insert(c1, "code-2", "CB-00002"); !errors.Is(err, ErrPairingCodeTaken) {
		t.Fatalf("second issue of one hash in one commune: %v", err)
	}
	// The same live hash in ANOTHER commune: two matches — the caller must treat it as none.
	if err := insert(c2, "code-3", "CB-00003"); err != nil {
		t.Fatal(err)
	}
	if m, _ := z.OpenPairingCodesByHash(context.Background(), hash); len(m) != 2 {
		t.Fatalf("two communes, one live hash: %d matches", len(m))
	}
	// Use: once.
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		c, err := s.LockOpenPairingCode(ctx, tx, "code-1")
		if err != nil || c.StaffCode != "CB-00001" {
			return fmt.Errorf("lock: %+v %w", c, err)
		}
		return s.MarkPairingCodeUsed(ctx, tx, "code-1", zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	err = h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := s.LockOpenPairingCode(ctx, tx, "code-1")
		return err
	})
	if !errors.Is(err, ErrPairingCodeNotOpen) {
		t.Fatalf("a used code is still open: %v", err)
	}
	// Issuing a new code cancels the open one (one open code per member of staff).
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := s.InsertPairingCode(ctx, tx, "code-4", "CB-00009", domain.PairingCodeHash("ZZZZ2345"),
			zaloAt.Add(time.Minute), zaloAt); err != nil {
			return err
		}
		n, err := s.CancelOpenPairingCodes(ctx, tx, "CB-00009", zaloAt)
		if n != 1 {
			return fmt.Errorf("cancelled %d", n)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPgZaloOneChatOneAccountAcrossCommunes(t *testing.T) {
	h, s, z, c1, c2 := zaloRig(t)
	chat := "chat-" + c1
	if n := pairStaff(t, h, s, z, c1, "CB-00001", chat, "link-1"); n != 0 {
		t.Fatalf("ended %d on a first pairing", n)
	}
	if got, found, err := z.CommuneOfLiveChat(context.Background(), chat); err != nil || !found || string(got) != c1 {
		t.Fatalf("commune of chat = %v %v %v", got, found, err)
	}
	// The same chat paired in commune 2: commune 1's link ends, audited IN commune 1 by the system.
	if n := pairStaff(t, h, s, z, c2, "CB-00002", chat, "link-2"); n != 1 {
		t.Fatalf("ended %d links elsewhere, want 1", n)
	}
	if _, found, _ := s.LiveLink(ctxXa(tenant.ID(c1)), "CB-00001"); found {
		t.Error("commune 1's link is still live")
	}
	if got, _, _ := z.CommuneOfLiveChat(context.Background(), chat); string(got) != c2 {
		t.Errorf("chat now resolves to %q, want %q", got, c2)
	}
	db := moKetNoi(t)
	var actor, kind, action string
	if err := db.QueryRow(`SELECT actor_id, actor_kind, action FROM audit_log WHERE tenant_id = $1`, c1).
		Scan(&actor, &kind, &action); err != nil {
		t.Fatalf("no entry in commune 1: %v", err)
	}
	if actor != "system" || kind != "system" || action != domain.ActionEndZaloLink {
		t.Errorf("entry = %s/%s/%s", actor, kind, action)
	}
	var by string
	if err := db.QueryRow(`SELECT unlinked_by FROM zalo_link WHERE tenant_id = $1 AND id = 'link-1'`, c1).Scan(&by); err != nil || by != "system" {
		t.Errorf("unlinked_by = %q, %v — another commune's staff code must never be written here", by, err)
	}
	// Re-pairing the same staff in commune 2 with a new chat ends the old link (one account, one chat).
	pairStaff(t, h, s, z, c2, "CB-00002", "chat-new-"+c2, "link-3")
	links, err := s.LiveLinks(ctxXa(tenant.ID(c2)))
	if err != nil || len(links) != 1 || links[0].ID != "link-3" {
		t.Fatalf("live links in commune 2 = %+v, %v", links, err)
	}
}

func TestPgZaloEnqueueDecidesTheThreeLocalSkips(t *testing.T) {
	h, s, z, c1, _ := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	notices := NewStaffNotificationStore(h)
	deliver := func(key, kind string, staff ...string) int {
		n := 0
		at := zaloAt.Add(time.Duration(len(key)) * time.Millisecond)
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			ids := make([]string, len(staff))
			for i := range ids {
				ids[i] = fmt.Sprintf("%s-%s-%d", c1[:10], key, i)
			}
			if _, err := notices.AddDelivery(ctx, tx, domain.NotificationDelivery{IdempotencyKey: key, Kind: kind,
				RecipientCodes: staff, Title: "T"}, ids, at); err != nil {
				return err
			}
			var err error
			n, err = s.EnqueueZaloDeliveries(ctx, tx, []string{key}, nil, at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	status := func(key string) (string, string) {
		var st, reason string
		if err := moKetNoi(t).QueryRow(`SELECT d.status, COALESCE(d.skip_reason, '') FROM zalo_delivery d
			JOIN staff_notification n ON n.tenant_id = d.tenant_id AND n.id = d.notification_id
			WHERE d.tenant_id = $1 AND n.idempotency_key = $2`, c1, key).Scan(&st, &reason); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		return st, reason
	}
	// No settings row: kenh-tat.
	deliver("k1", domain.StaffNotificationDueSoon, "CB-00001")
	if st, r := status("k1"); st != "bo-qua" || r != "kenh-tat" {
		t.Errorf("no settings: %s/%s", st, r)
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SaveChannelSetting(ctx, tx, domain.ZaloChannelSetting{IsEnabled: true, Kinds: []string{"sap-den-han"},
			QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60}, "CB-9", zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	deliver("k2", domain.StaffNotificationEscalation, "CB-00001")
	if st, r := status("k2"); st != "bo-qua" || r != "loai-tat" {
		t.Errorf("kind off: %s/%s", st, r)
	}
	deliver("k3", domain.StaffNotificationDueSoon, "CB-00001")
	if st, r := status("k3"); st != "bo-qua" || r != "chua-lien-ket" {
		t.Errorf("not linked: %s/%s", st, r)
	}
	pairStaff(t, h, s, z, c1, "CB-00001", "chat-q-"+c1, "link-q")
	if n := deliver("k4", domain.StaffNotificationDueSoon, "CB-00001"); n != 1 {
		t.Fatalf("queued %d", n)
	}
	if st, _ := status("k4"); st != "cho-gui" {
		t.Errorf("linked, enabled, selected: %s", st)
	}
	// BELL ONLY (ADR 0081 #5): linked and enabled, yet the mention gets no zalo_delivery row — not even a
	// skip — and the enqueue does not fail on 0021's zalo_delivery CHECK, which does not admit it.
	if n := deliver("k5", domain.StaffNotificationDisbursementMention, "CB-00001"); n != 0 {
		t.Fatalf("bell-only notice queued %d Zalo rows", n)
	}
	var zaloRows int
	if err := moKetNoi(t).QueryRow(`SELECT count(*) FROM zalo_delivery d
		JOIN staff_notification n ON n.tenant_id = d.tenant_id AND n.id = d.notification_id
		WHERE d.tenant_id = $1 AND n.idempotency_key = 'k5'`, c1).Scan(&zaloRows); err != nil || zaloRows != 0 {
		t.Errorf("bell-only notice has %d zalo_delivery rows, %v", zaloRows, err)
	}

	// The sender's half: claim, lease, sent — and the commune is listed while it is owed.
	if ids, err := z.CommunesWithDueZaloDeliveries(context.Background(), zaloAt.Add(time.Hour)); err != nil || !contains(ids, c1) {
		t.Fatalf("due communes = %v, %v", ids, err)
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		due, err := s.ClaimDueDeliveries(ctx, tx, zaloAt.Add(time.Hour), 10)
		if err != nil || len(due) != 1 || due[0].Title != "T" || due[0].StaffCode != "CB-00001" {
			return fmt.Errorf("claim = %+v %w", due, err)
		}
		chats, err := s.LiveChatsOf(ctx, tx, domain.SharedZaloBotRef, []string{"CB-00001"})
		if err != nil || chats["CB-00001"].LinkID != "link-q" {
			return fmt.Errorf("chats = %+v %w", chats, err)
		}
		if err := s.StartAttempt(ctx, tx, due[0].ID, "link-q", zaloAt.Add(2*time.Hour), zaloAt); err != nil {
			return err
		}
		return s.MarkDeliverySent(ctx, tx, due[0].ID, zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	if st, _ := status("k4"); st != "da-gui" {
		t.Errorf("after send: %s", st)
	}
	// A finished row keeps its outcome (the 0018 guard, and the store's `status = 'cho-gui'` filter).
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var id string
		if err := tx.Underlying().QueryRowContext(ctx, `SELECT id FROM zalo_delivery WHERE tenant_id = $1 AND status = 'da-gui'`, c1).Scan(&id); err != nil {
			return err
		}
		return s.FailDelivery(ctx, tx, id, domain.ZaloCallRejected, zaloAt)
	})
	if !errors.Is(err, ErrZaloRowChanged) {
		t.Errorf("a sent row was moved: %v", err)
	}
}

// 0025 (ADR 0086): an act notice is admitted on all three tables and gets a QUEUED Zalo row only when the
// commune ticked its own kind — never through an old value, which no producer of an act notice ever sent.
func TestPgZaloEnqueueActNoticeOnlyWhenTicked(t *testing.T) {
	h, s, z, c1, _ := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	notices := NewStaffNotificationStore(h)
	pairStaff(t, h, s, z, c1, "CB-00001", "chat-act-"+c1, "link-act")
	save := func(kinds ...string) {
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.SaveChannelSetting(ctx, tx, domain.ZaloChannelSetting{IsEnabled: true, Kinds: kinds,
				QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60}, "CB-9", zaloAt)
		}); err != nil {
			t.Fatal(err)
		}
	}
	deliver := func(key, kind string) (string, string) {
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			if _, err := notices.AddDelivery(ctx, tx, domain.NotificationDelivery{IdempotencyKey: key, Kind: kind,
				RecipientCodes: []string{"CB-00001"}, Title: "T"}, []string{c1[:10] + "-" + key}, zaloAt); err != nil {
				return err
			}
			_, err := s.EnqueueZaloDeliveries(ctx, tx, []string{key}, nil, zaloAt)
			return err
		}); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		var st, reason string
		if err := moKetNoi(t).QueryRow(`SELECT d.status, COALESCE(d.skip_reason, '') FROM zalo_delivery d
			JOIN staff_notification n ON n.tenant_id = d.tenant_id AND n.id = d.notification_id
			WHERE d.tenant_id = $1 AND n.idempotency_key = $2`, c1, key).Scan(&st, &reason); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		return st, reason
	}
	// Every old value ticked, and a different act notice: still off for this one.
	save("sap-den-han", "qua-han", "leo-thang", "ban-tin-tuan", domain.ZaloKindPetitionReopened)
	if st, r := deliver("act-1", domain.ZaloKindTaskAssigned); st != "bo-qua" || r != "loai-tat" {
		t.Errorf("not ticked: %s/%s", st, r)
	}
	save(domain.ZaloKindTaskAssigned, domain.ZaloKindAnnouncementPublished)
	if st, r := deliver("act-2", domain.ZaloKindTaskAssigned); st != "cho-gui" || r != "" {
		t.Errorf("ticked: %s/%s", st, r)
	}
	if st, _ := deliver("thong-bao.moi:act-3", domain.ZaloKindAnnouncementPublished); st != "cho-gui" {
		t.Errorf("announcement ticked: %s", st)
	}
}

func contains(ids []tenant.ID, want string) bool {
	for _, id := range ids {
		if string(id) == want {
			return true
		}
	}
	return false
}

// 0024 (ADR 0079 lô 5 Q13): the lead round-trips and is bounded by the database; the due-soon items land on
// the QUEUED Zalo row only, come back from the claim as sent, are frozen by the guard; and the new skip
// reason is admitted.
func TestPgZaloDueSoonLeadAndItems(t *testing.T) {
	h, s, z, c1, _ := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	three, fifteen := 3, 15
	save := func(days *int) error {
		return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.SaveChannelSetting(ctx, tx, domain.ZaloChannelSetting{IsEnabled: true,
				Kinds: []string{domain.ZaloKindTaskDueSoon}, QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60,
				DueSoonDays: days}, "CB-9", zaloAt)
		})
	}
	if err := save(&three); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ChannelSetting(ctx); err != nil || got.DueSoonDays == nil || *got.DueSoonDays != 3 {
		t.Fatalf("lead round trip: %+v %v", got, err)
	}
	if err := save(&fifteen); err == nil {
		t.Error("a lead of 15 days was stored — 0024's CHECK must refuse it")
	}
	if err := save(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ChannelSetting(ctx); got.DueSoonDays != nil {
		t.Errorf("NULL lead read back as %v", *got.DueSoonDays)
	}

	notices := NewStaffNotificationStore(h)
	// Fixed instants, as a producer would copy them from storage.
	items := []domain.DueSoonItem{{Code: "NV-2026-0001", Deadline: time.Date(2026, 10, 6, 5, 0, 0, 0, time.UTC)},
		{Code: "NV-2026-0002", Deadline: time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)}}
	deliver := func(key string, staff ...string) {
		at := zaloAt.Add(time.Duration(len(key)) * time.Millisecond)
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			ids := make([]string, len(staff))
			for i := range ids {
				ids[i] = fmt.Sprintf("%s-%s-%d", c1[:10], key, i)
			}
			if _, err := notices.AddDelivery(ctx, tx, domain.NotificationDelivery{IdempotencyKey: key,
				Kind: domain.ZaloKindTaskDueSoon, RecipientCodes: staff, Title: "T"}, ids, at); err != nil {
				return err
			}
			_, err := s.EnqueueZaloDeliveries(ctx, tx, []string{key}, map[string][]domain.DueSoonItem{key: items}, at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	itemsOf := func(staff string) (string, bool) {
		var raw *string
		if err := moKetNoi(t).QueryRow(`SELECT due_soon_items::text FROM zalo_delivery
			WHERE tenant_id = $1 AND staff_code = $2`, c1, staff).Scan(&raw); err != nil {
			t.Fatalf("%s: %v", staff, err)
		}
		if raw == nil {
			return "", false
		}
		return *raw, true
	}
	pairStaff(t, h, s, z, c1, "CB-00001", "chat-ds-"+c1, "link-ds")
	deliver("ds1", "CB-00001", "CB-00002") // CB-00002 is not linked: skipped at enqueue, carries none
	if _, ok := itemsOf("CB-00001"); !ok {
		t.Error("the queued row has no due-soon items")
	}
	if raw, ok := itemsOf("CB-00002"); ok {
		t.Errorf("a skipped row carries items: %s", raw)
	}

	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		due, err := s.ClaimDueDeliveries(ctx, tx, zaloAt.Add(time.Hour), 10)
		if err != nil || len(due) != 1 || len(due[0].DueSoonItems) != 2 {
			return fmt.Errorf("claim = %+v %w", due, err)
		}
		for i, it := range due[0].DueSoonItems {
			if it.Code != items[i].Code || !it.Deadline.Equal(items[i].Deadline) {
				return fmt.Errorf("item %d = %+v, want %+v", i, it, items[i])
			}
		}
		return s.SkipDelivery(ctx, tx, due[0].ID, domain.ZaloSkipOutsideDueSoonLead, zaloAt)
	}); err != nil {
		t.Fatal(err)
	}
	// The guard freezes what the row was about.
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := tx.Exec(ctx, `UPDATE zalo_delivery SET due_soon_items = '[]'::jsonb
			WHERE tenant_id = $1 AND staff_code = 'CB-00001'`, c1)
		return err
	})
	if err == nil {
		t.Error("due_soon_items was rewritten after the row was queued")
	}
}
