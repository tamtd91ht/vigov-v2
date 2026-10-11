package store

import (
	"bytes"
	"context"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Migration 0029 (Zalo GROUP chats, ADR 0092) against a REAL PostgreSQL: the platform-wide group chat key,
// the one-chat-one-kind trigger on both link tables, the group kind CHECKs, the guards, the outbox dedupe,
// and the widened bot-switch check. Raw SQL, because the Go store for these tables is the next card; when
// it lands, its own pg suite replaces the inserts here. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run
// without the DSN means this file COMPILES, nothing more.

// groupTx runs fn in one commune-scoped transaction.
func groupTx(h *pkgstore.DB, commune string, fn func(ctx context.Context, tx *pkgstore.ScopedTx) error) error {
	ctx := ctxXa(tenant.ID(commune))
	return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(ctx, tx) })
}

// insertGroupCode issues one group code. kinds is comma-separated; orgUnit "" = the whole commune.
func insertGroupCode(h *pkgstore.DB, commune, id string, hash byte, orgUnit, kinds string) error {
	return groupTx(h, commune, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, err := tx.Exec(ctx, `INSERT INTO zalo_group_pairing_code
			(tenant_id, id, issued_by, org_unit_id, kinds, code_hash, expires_at, created_at)
			VALUES ($1, $2, 'CB-00123', NULLIF($3, ''), string_to_array($4::text, ','), $5, $6, $7)`,
			commune, id, orgUnit, kinds, bytes.Repeat([]byte{hash}, 32), zaloAt.Add(10*time.Minute), zaloAt)
		return err
	})
}

func insertGroupLink(ctx context.Context, tx *pkgstore.ScopedTx, commune, id, botRef, chat, codeID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO zalo_group_link
		(tenant_id, id, bot_ref, chat_id, org_unit_id, kinds, pairing_code_id, linked_at, linked_by)
		VALUES ($1, $2, $3, $4, NULL, ARRAY['nhiem-vu.giao-moi'], $5, $6, 'CB-00123')`,
		commune, id, botRef, chat, codeID, zaloAt)
	return err
}

func linkGroup(h *pkgstore.DB, commune, id, botRef, chat, codeID string) error {
	return groupTx(h, commune, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return insertGroupLink(ctx, tx, commune, id, botRef, chat, codeID)
	})
}

func TestPgZaloGroupChatIsOneKindAndOneCommune(t *testing.T) {
	h, links, z, c1, c2 := zaloRig(t)
	pairStaff(t, h, links, z, c1, "CB-00001", "chat-g-personal-1", "01JLINKGP00000000000000001")
	for i, id := range []string{"01JGCODE000000000000000001", "01JGCODE000000000000000002", "01JGCODE000000000000000003"} {
		if err := insertGroupCode(h, c1, id, byte(10+i), "", "nhiem-vu.giao-moi,phan-anh.qua-han"); err != nil {
			t.Fatalf("issue code %d: %v", i, err)
		}
	}
	if err := insertGroupCode(h, c2, "01JGCODE000000000000000009", 10, "01JORGUNIT0000000000000001", "van-ban.chuyen-toi"); err != nil {
		t.Fatalf("same hash in ANOTHER commune must be allowed (unique per commune): %v", err)
	}

	// A chat paired to a person is not linked as a group (0029 zalo_chat_one_link_kind).
	if err := linkGroup(h, c1, "01JGLINK000000000000000001", domain.SharedZaloBotRef, "chat-g-personal-1",
		"01JGCODE000000000000000001"); err == nil {
		t.Fatal("a personal chat was linked as a group")
	}
	if err := linkGroup(h, c1, "01JGLINK000000000000000002", domain.SharedZaloBotRef, "chat-g-group-1",
		"01JGCODE000000000000000001"); err != nil {
		t.Fatalf("link a group: %v", err)
	}
	// ...and the reverse: a live group chat is not paired to a person.
	if err := groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return links.InsertLink(ctx, tx, "01JLINKGP00000000000000002", "CB-00002", domain.SharedZaloBotRef, "chat-g-group-1", zaloAt)
	}); err == nil {
		t.Fatal("a live group chat was paired to a person")
	}
	// One code, one link.
	if err := linkGroup(h, c1, "01JGLINK000000000000000003", domain.SharedZaloBotRef, "chat-g-group-2",
		"01JGCODE000000000000000001"); err == nil {
		t.Fatal("one code made two links")
	}
	// ADR 0092 #9: the same group in ANOTHER commune is refused while it is live in the first.
	if err := linkGroup(h, c2, "01JGLINK000000000000000004", domain.SharedZaloBotRef, "chat-g-group-1",
		"01JGCODE000000000000000009"); err == nil {
		t.Fatal("a group live in commune 1 was linked by commune 2")
	}
	// A link must name a code of its OWN commune (composite FK).
	if err := linkGroup(h, c2, "01JGLINK000000000000000005", domain.SharedZaloBotRef, "chat-g-group-3",
		"01JGCODE000000000000000002"); err == nil {
		t.Fatal("commune 2 linked through commune 1's code")
	}

	for _, c := range []struct{ orgUnit, kinds, why string }{
		{"", "nhiem-vu.nhac-ten", "a mention names one person (ADR 0092 #2)"},
		{"", "nhiem-vu.sap-den-han", "a per-person digest"},
		{"", "ban-tin-tuan", "a digest"},
		{"", "qua-han", "a legacy kind no group can ever receive"},
		{"", "", "an empty kind set"},
		{"bp-1", "nhiem-vu.giao-moi", "an org unit that is not a ULID"},
	} {
		if err := insertGroupCode(h, c1, "01JGCODE000000000000000099", 99, c.orgUnit, c.kinds); err == nil {
			t.Errorf("a group code was issued with %s", c.why)
		}
	}
}

func TestPgZaloGroupLinkGuardAndBotSwitch(t *testing.T) {
	h, links, _, c1, _ := zaloRig(t)
	bots := NewZaloCommuneBotStore(h)
	for i, id := range []string{"01JGCODE000000000000000011", "01JGCODE000000000000000012", "01JGCODE000000000000000013",
		"01JGCODE000000000000000014"} {
		if err := insertGroupCode(h, c1, id, byte(20+i), "", "nhiem-vu.giao-moi"); err != nil {
			t.Fatal(err)
		}
	}
	if err := linkGroup(h, c1, "01JGLINK000000000000000011", domain.SharedZaloBotRef, "chat-g-group-11",
		"01JGCODE000000000000000011"); err != nil {
		t.Fatal(err)
	}
	exec := func(stmt string, args ...any) error {
		return groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			_, err := tx.Exec(ctx, stmt, append([]any{c1}, args...)...)
			return err
		})
	}
	if err := exec(`UPDATE zalo_group_link SET chat_id = 'other' WHERE tenant_id = $1 AND id = $2`,
		"01JGLINK000000000000000011"); err == nil {
		t.Fatal("a group link was re-pointed at another chat")
	}
	if err := exec(`UPDATE zalo_group_link SET kinds = ARRAY['nhiem-vu.giao-moi', 'phan-anh.mo-lai'], updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, "01JGLINK000000000000000011"); err != nil {
		t.Fatalf("editing a live group's kinds: %v", err)
	}
	if err := exec(`UPDATE zalo_group_link SET unlinked_at = $3, unlinked_by = 'CB-00123', unlink_reason = 'Gỡ thử'
		WHERE tenant_id = $1 AND id = $2`, "01JGLINK000000000000000011", zaloAt.Add(time.Hour)); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := exec(`UPDATE zalo_group_link SET kinds = ARRAY['nhiem-vu.giao-moi'] WHERE tenant_id = $1 AND id = $2`,
		"01JGLINK000000000000000011"); err == nil {
		t.Fatal("an ended group link was edited")
	}

	// Shared -> own: a live SHARED group link left behind fails the adoption at COMMIT (0029 adopt check).
	if err := linkGroup(h, c1, "01JGLINK000000000000000012", domain.SharedZaloBotRef, "chat-g-group-12",
		"01JGCODE000000000000000012"); err != nil {
		t.Fatal(err)
	}
	if err := adoptBot(t, h, bots, links, c1, "01JBOTGP000000000000000001", "pg-bot-g1", true); err == nil {
		t.Fatal("an own bot was adopted with a live shared GROUP link left")
	}
	if err := groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		if _, err := tx.Exec(ctx, `UPDATE zalo_group_link SET unlinked_at = $2, unlinked_by = 'CB-00123',
			unlink_reason = 'Đổi sang bot riêng' WHERE tenant_id = $1 AND bot_ref = 'shared' AND unlinked_at IS NULL`,
			c1, zaloAt.Add(2*time.Hour)); err != nil {
			return err
		}
		return bots.InsertLive(ctx, tx, domain.CommuneZaloBot{ID: "01JBOTGP000000000000000001", BotAccountID: "pg-bot-g1",
			BotName: "Bot Xã", ChatURL: "https://zalo.me/1", SetAt: zaloAt, SetBy: "CB-00123"}, fakeSealed)
	}); err != nil {
		t.Fatalf("adopt with the group links ended: %v", err)
	}
	// While the commune has its own bot, a shared-bot group link is refused; its own bot's is accepted.
	if err := linkGroup(h, c1, "01JGLINK000000000000000013", domain.SharedZaloBotRef, "chat-g-group-13",
		"01JGCODE000000000000000013"); err == nil {
		t.Fatal("a shared-bot group link was made while the commune uses its own bot")
	}
	if err := linkGroup(h, c1, "01JGLINK000000000000000014", "commune:pg-bot-g1", "chat-g-group-14",
		"01JGCODE000000000000000014"); err != nil {
		t.Fatalf("group link through the own bot: %v", err)
	}
	// Retiring the own bot with its group link live fails at COMMIT (0029 retire check).
	if err := groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return bots.Retire(ctx, tx, "01JBOTGP000000000000000001", "CB-00123", "Thử quay về bot chung", zaloAt.Add(3*time.Hour))
	}); err == nil {
		t.Fatal("a bot was retired with a live group link left")
	}
}

func TestPgZaloGroupDeliveryDedupeAndFinalOutcome(t *testing.T) {
	h, _, _, c1, _ := zaloRig(t)
	if err := insertGroupCode(h, c1, "01JGCODE000000000000000021", 40, "", "nhiem-vu.giao-moi"); err != nil {
		t.Fatal(err)
	}
	if err := linkGroup(h, c1, "01JGLINK000000000000000021", domain.SharedZaloBotRef, "chat-g-group-21",
		"01JGCODE000000000000000021"); err != nil {
		t.Fatal(err)
	}
	enqueue := func(id, kind string) (int64, error) {
		var n int64
		err := groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			r, err := tx.Exec(ctx, `INSERT INTO zalo_group_delivery
				(tenant_id, id, group_link_id, idempotency_key, kind, title, body, link)
				VALUES ($1, $2, '01JGLINK000000000000000021', 'nhiem-vu.giao-moi:01JACT0000000000000000001', $3,
				        'Việc mới NV-00001', '', '/nhiem-vu')
				ON CONFLICT (tenant_id, group_link_id, idempotency_key) DO NOTHING`, c1, id, kind)
			if err != nil {
				return err
			}
			n, err = r.RowsAffected()
			return err
		})
		return n, err
	}
	if n, err := enqueue("01JGDELIV00000000000000001", "nhiem-vu.giao-moi"); err != nil || n != 1 {
		t.Fatalf("first enqueue: %d %v", n, err)
	}
	if n, err := enqueue("01JGDELIV00000000000000002", "nhiem-vu.giao-moi"); err != nil || n != 0 {
		t.Fatalf("the same key to the same group queued twice: %d %v", n, err)
	}
	exec := func(stmt string) error {
		return groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			_, err := tx.Exec(ctx, stmt, c1, "01JGDELIV00000000000000001")
			return err
		})
	}
	if err := exec(`UPDATE zalo_group_delivery SET title = 'khác' WHERE tenant_id = $1 AND id = $2`); err == nil {
		t.Fatal("what was posted was rewritten")
	}
	if err := exec(`UPDATE zalo_group_delivery SET status = 'da-gui', sent_at = now(), next_attempt_at = NULL, attempts = 1
		WHERE tenant_id = $1 AND id = $2`); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if err := exec(`UPDATE zalo_group_delivery SET status = 'cho-gui', sent_at = NULL, next_attempt_at = now()
		WHERE tenant_id = $1 AND id = $2`); err == nil {
		t.Fatal("a sent group message became owed again")
	}
	if _, err := enqueue("01JGDELIV00000000000000003", "ban-tin-tuan"); err == nil {
		t.Fatal("a digest kind was queued for a group")
	}
}
