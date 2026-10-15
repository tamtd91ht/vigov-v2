package store

import (
	"context"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The ORDER of GET /api/v1/zalo-links and GET /api/v1/zalo-groups: newest link first, id DESC on a tie
// (owner decision 15/10/2026, tester sheet web-admin-141026). Against a REAL PostgreSQL; SKIPPED WITHOUT
// VIGOV_TEST_DSN — a green run without the DSN means this file COMPILES, nothing more.
//
// The ids are chosen so that id order alone (ascending) is NOT the expected order, and two rows share
// one instant, so both the DESC on the time and the DESC tie-break are pinned.

func TestPgZaloLinksNewestFirst(t *testing.T) {
	h, s, _, c1, _ := zaloRig(t)
	ctx := ctxXa(tenant.ID(c1))
	rows := []struct {
		id, staff, chat string
		at              time.Time
	}{
		{"01JLINKORD0000000000000003", "CB-00001", "chat-ord-1-" + c1, zaloAt},                    // oldest
		{"01JLINKORD0000000000000001", "CB-00002", "chat-ord-2-" + c1, zaloAt.Add(time.Hour)},     // tie
		{"01JLINKORD0000000000000002", "CB-00003", "chat-ord-3-" + c1, zaloAt.Add(time.Hour)},     // tie
		{"01JLINKORD0000000000000000", "CB-00004", "chat-ord-4-" + c1, zaloAt.Add(2 * time.Hour)}, // newest
	}
	for _, r := range rows {
		if err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.InsertLink(ctx, tx, r.id, r.staff, domain.SharedZaloBotRef, r.chat, r.at)
		}); err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}
	got, err := s.LiveLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"01JLINKORD0000000000000000", "01JLINKORD0000000000000002", "01JLINKORD0000000000000001",
		"01JLINKORD0000000000000003"}
	if len(got) != len(want) {
		t.Fatalf("got %d links, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("row %d = %s, want %s (linked_at DESC, id DESC)", i, got[i].ID, want[i])
		}
	}
}

func TestPgZaloGroupsNewestFirst(t *testing.T) {
	h, _, _, c1, _ := zaloRig(t)
	g := NewZaloGroupStore(h)
	rows := []struct {
		link, code string
		hash       byte
		at         time.Time
	}{
		{"01JGLINKORD000000000000003", "01JGCODEORD000000000000001", 101, zaloAt},
		{"01JGLINKORD000000000000001", "01JGCODEORD000000000000002", 102, zaloAt.Add(time.Hour)},
		{"01JGLINKORD000000000000002", "01JGCODEORD000000000000003", 103, zaloAt.Add(time.Hour)},
		{"01JGLINKORD000000000000000", "01JGCODEORD000000000000004", 104, zaloAt.Add(2 * time.Hour)},
	}
	for i, r := range rows {
		if err := insertGroupCode(h, c1, r.code, r.hash, "", "nhiem-vu.giao-moi"); err != nil {
			t.Fatalf("issue %s: %v", r.code, err)
		}
		if err := groupTx(h, c1, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			_, err := tx.Exec(ctx, `INSERT INTO zalo_group_link
				(tenant_id, id, bot_ref, chat_id, org_unit_id, kinds, pairing_code_id, linked_at, linked_by)
				VALUES ($1, $2, $3, $4, NULL, ARRAY['nhiem-vu.giao-moi'], $5, $6, 'CB-00123')`,
				c1, r.link, domain.SharedZaloBotRef, "chat-gord-"+string(rune('a'+i))+"-"+c1, r.code, r.at)
			return err
		}); err != nil {
			t.Fatalf("link %s: %v", r.link, err)
		}
	}
	got, err := g.LiveGroupLinks(ctxXa(tenant.ID(c1)))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"01JGLINKORD000000000000000", "01JGLINKORD000000000000002", "01JGLINKORD000000000000001",
		"01JGLINKORD000000000000003"}
	if len(got) != len(want) {
		t.Fatalf("got %d groups, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("row %d = %s, want %s (linked_at DESC, id DESC)", i, got[i].ID, want[i])
		}
	}
}
