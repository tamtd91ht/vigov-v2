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

// The Go store of migration 0029 (ZaloGroupStore + crosstenant's group half) against a REAL PostgreSQL: the
// column spellings, the text[] round trip, the error mapping of the three insert refusals, the enqueue's
// dedupe, the claim's join, and the dispatcher's commune list. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run
// without the DSN means this file COMPILES, nothing more.

const (
	pgUnitA = "01JBPHAN00000000000000000A"
	pgUnitB = "01JBPHAN00000000000000000B"
)

func groupHash(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestPgZaloGroupStoreLinkingAndTheThreeRefusals(t *testing.T) {
	h, links, z, c1, c2 := zaloRig(t)
	g := NewZaloGroupStore(h)
	ctx1 := ctxXa(tenant.ID(c1))
	issue := func(ctx context.Context, id string, hash byte, unit string) error {
		return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return g.InsertGroupPairingCode(ctx, tx, id, "CB-00123", unit, []string{"nhiem-vu.giao-moi", "phan-anh.mo-lai"},
				groupHash(hash), zaloAt.Add(10*time.Minute), zaloAt)
		})
	}
	for i, id := range []string{"01JGPGCODE0000000000000001", "01JGPGCODE0000000000000002", "01JGPGCODE0000000000000003"} {
		if err := issue(ctx1, id, byte(60+i), pgUnitA); err != nil {
			t.Fatal(err)
		}
	}
	if err := issue(ctx1, "01JGPGCODE0000000000000009", 60, ""); !errors.Is(err, ErrPairingCodeTaken) {
		t.Fatalf("same hash twice in a commune → %v", err)
	}
	// Scoped lookup and the shared bot's cross-commune lookup.
	if id, found, err := g.OpenGroupPairingCodeByHash(ctx1, groupHash(60)); err != nil || !found || id != "01JGPGCODE0000000000000001" {
		t.Fatalf("scoped lookup = %q %v %v", id, found, err)
	}
	if _, found, _ := g.OpenGroupPairingCodeByHash(ctxXa(tenant.ID(c2)), groupHash(60)); found {
		t.Fatal("commune 2 found commune 1's group code")
	}
	if m, err := z.OpenGroupPairingCodesByHash(context.Background(), groupHash(60)); err != nil || len(m) != 1 ||
		string(m[0].TenantID) != c1 {
		t.Fatalf("cross-commune lookup = %v %v", m, err)
	}

	link := func(ctx context.Context, codeID, linkID, botRef, chat string) error {
		return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			c, err := g.LockOpenGroupPairingCode(ctx, tx, codeID)
			if err != nil {
				return err
			}
			if err := g.MarkGroupPairingCodeUsed(ctx, tx, c.ID, zaloAt); err != nil {
				return err
			}
			return g.InsertGroupLink(ctx, tx, linkID, botRef, chat, c, zaloAt)
		})
	}
	if err := link(ctx1, "01JGPGCODE0000000000000001", "01JGPGLINK0000000000000001", domain.SharedZaloBotRef, "pg-group-1"); err != nil {
		t.Fatal(err)
	}
	if err := link(ctx1, "01JGPGCODE0000000000000001", "01JGPGLINK0000000000000009", domain.SharedZaloBotRef, "pg-group-9"); !errors.Is(err,
		ErrPairingCodeNotOpen) {
		t.Fatalf("a used code → %v", err)
	}
	if live, err := z.GroupChatLiveAnywhere(context.Background(), domain.SharedZaloBotRef, "pg-group-1"); err != nil || !live {
		t.Fatalf("live anywhere = %v %v", live, err)
	}
	if live, _ := g.LiveGroupLinkOfChat(ctx1, domain.SharedZaloBotRef, "pg-group-1"); !live {
		t.Fatal("scoped chat lookup missed the live group")
	}
	// The same chat again: the platform-wide key.
	if err := link(ctx1, "01JGPGCODE0000000000000002", "01JGPGLINK0000000000000002", domain.SharedZaloBotRef, "pg-group-1"); !errors.Is(err,
		ErrZaloGroupChatLinked) {
		t.Fatalf("second live link of a chat → %v", err)
	}
	// A chat paired to a person.
	pairStaff(t, h, links, z, c1, "CB-00001", "pg-personal-1", "01JLINKPG00000000000000001")
	if err := link(ctx1, "01JGPGCODE0000000000000002", "01JGPGLINK0000000000000003", domain.SharedZaloBotRef, "pg-personal-1"); !errors.Is(err,
		ErrZaloGroupChatPaired) {
		t.Fatalf("personal chat → %v", err)
	}
	// A bot that does not serve the commune.
	if err := link(ctx1, "01JGPGCODE0000000000000003", "01JGPGLINK0000000000000004", "commune:nobody", "pg-group-4"); !errors.Is(err,
		ErrZaloGroupBotNotLive) {
		t.Fatalf("bot not live → %v", err)
	}

	// Read back, edit, end.
	list, err := g.LiveGroupLinks(ctx1)
	if err != nil || len(list) != 1 || list[0].OrgUnitID != pgUnitA || len(list[0].Kinds) != 2 || list[0].LinkedBy != "CB-00123" {
		t.Fatalf("list = %+v %v", list, err)
	}
	if other, _ := g.LiveGroupLinks(ctxXa(tenant.ID(c2))); len(other) != 0 {
		t.Fatal("commune 2 lists commune 1's group")
	}
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		if _, err := g.LockLiveGroupLink(ctx1, tx, list[0].ID); err != nil {
			return err
		}
		return g.UpdateGroupLinkScope(ctx1, tx, list[0].ID, "", []string{"phan-anh.mo-lai"}, zaloAt.Add(time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := g.CountLiveGroupLinksOfBot(ctx1, domain.SharedZaloBotRef); err != nil || n != 1 {
		t.Fatalf("count = %d %v", n, err)
	}
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		ended, err := g.EndLiveGroupLinksOfBot(ctx1, tx, domain.SharedZaloBotRef, "CB-00123", domain.ZaloGroupEndedByBotChange,
			zaloAt.Add(2*time.Minute))
		if err != nil || len(ended) != 1 || ended[0].OrgUnitID != "" {
			return errors.Join(err, errors.New("ended != one commune-wide group"))
		}
		return g.EndGroupLink(ctx1, tx, ended[0].ID, "CB-00123", "x", zaloAt)
	}); !errors.Is(err, ErrZaloGroupNotFound) {
		t.Fatalf("ending an ended link → %v", err)
	}
}

func TestPgZaloGroupOutboxDedupeClaimAndCommuneList(t *testing.T) {
	h, _, z, c1, _ := zaloRig(t)
	g := NewZaloGroupStore(h)
	ctx := ctxXa(tenant.ID(c1))
	if err := insertGroupCode(h, c1, "01JGPGCODE0000000000000011", 71, "", "nhiem-vu.giao-moi"); err != nil {
		t.Fatal(err)
	}
	if err := linkGroup(h, c1, "01JGPGLINK0000000000000011", domain.SharedZaloBotRef, "pg-group-11",
		"01JGPGCODE0000000000000011"); err != nil {
		t.Fatal(err)
	}
	enqueue := func(id string) (int, error) {
		var n int
		err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			var err error
			n, err = g.EnqueueGroupDeliveries(ctx, tx, func(groups []domain.ZaloGroupTarget) ([]GroupDeliveryRow, error) {
				if len(groups) != 1 || groups[0].OrgUnitID != "" {
					return nil, errors.New("targets wrong")
				}
				return []GroupDeliveryRow{{ID: id, GroupLinkID: groups[0].ID, IdempotencyKey: "task_assigned:01JPG",
					Kind: "nhiem-vu.giao-moi", Title: "Việc mới NV-1", Link: "/nhiem-vu/1"}}, nil
			}, zaloAt)
			return err
		})
		return n, err
	}
	if n, err := enqueue("01JGPGDELIV000000000000001"); err != nil || n != 1 {
		t.Fatalf("first = %d %v", n, err)
	}
	if n, err := enqueue("01JGPGDELIV000000000000002"); err != nil || n != 0 {
		t.Fatalf("the same key to the same group = %d %v", n, err)
	}
	ids, err := z.CommunesWithDueZaloDeliveries(context.Background(), zaloAt.Add(time.Minute))
	found := false
	for _, id := range ids {
		found = found || string(id) == c1
	}
	if err != nil || !found {
		t.Fatalf("the dispatcher's list misses the commune with a group row: %v %v", ids, err)
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		due, err := g.ClaimDueGroupDeliveries(ctx, tx, zaloAt.Add(time.Minute), 10)
		if err != nil {
			return err
		}
		if len(due) != 1 || !due[0].LinkLive || due[0].ChatID != "pg-group-11" || due[0].BotRef != domain.SharedZaloBotRef ||
			len(due[0].GroupKinds) != 1 {
			return errors.New("claim read wrong")
		}
		if err := g.StartGroupAttempt(ctx, tx, due[0].ID, zaloAt.Add(10*time.Minute), zaloAt); err != nil {
			return err
		}
		return g.MarkGroupDeliverySent(ctx, tx, due[0].ID, zaloAt.Add(time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return g.SkipGroupDelivery(ctx, tx, "01JGPGDELIV000000000000001", domain.ZaloSkipKindOff, zaloAt)
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("a sent row re-decided → %v", err)
	}
}
