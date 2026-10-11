package crosstenant

// The unscoped statements of the shared bot's GROUP side (ADR 0092; migration 0029). The shared bot's
// webhook has no commune Host, so — as for the personal code (zalo_bot.go) — the commune is found FROM the
// group code a chat typed, and whether a chat is already live as a group is asked platform-wide, because
// 0029's zalo_group_link_one_live_per_chat is platform-wide (ADR 0092 #9).
//
// Both RESOLVE from an unguessable key a chat presented and return identifiers or a yes/no — never a row of
// a second commune. chat_id is a bind parameter here and nothing else, never in an error (ADR 0092 §Hệ quả).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/tenant"
)

// openGroupPairingCodesByHash uses 0029's zalo_group_pairing_code_open_by_hash. LIMIT 2: the caller needs
// only "exactly one" versus "not exactly one".
const openGroupPairingCodesByHash = `SELECT tenant_id, id FROM zalo_group_pairing_code
	WHERE code_hash = $1 AND used_at IS NULL AND cancelled_at IS NULL
	ORDER BY tenant_id, id LIMIT 2`

// OpenGroupPairingCodesByHash returns the open GROUP codes whose hash is hash — at most two.
func (z *ZaloBot) OpenGroupPairingCodesByHash(ctx context.Context, hash []byte) ([]PairingCodeMatch, error) {
	// @cross-tenant: ADR 0092 #3 / 0029 zalo_group_pairing_code_open_by_hash — the shared bot's webhook has
	// no commune Host; the commune is found FROM the group code typed in the group. Identifiers only, at most
	// two rows, and the caller refuses anything but exactly one.
	rows, err := z.db.QueryContext(ctx, openGroupPairingCodesByHash, hash)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: find group pairing code: %w", err)
	}
	defer rows.Close()
	var out []PairingCodeMatch
	for rows.Next() {
		var tid, id string
		if err := rows.Scan(&tid, &id); err != nil {
			return nil, fmt.Errorf("crosstenant: find group pairing code: %w", err)
		}
		if t := tenant.ID(tid); t.Valid() {
			out = append(out, PairingCodeMatch{TenantID: t, CodeID: id})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: find group pairing code: %w", err)
	}
	return out, nil
}

// GroupChatLiveAnywhere reports whether chatID of botRef is live as a group in ANY commune. A yes/no and
// nothing about which commune: the refusal sentence names none (ADR 0092 #9).
func (z *ZaloBot) GroupChatLiveAnywhere(ctx context.Context, botRef, chatID string) (bool, error) {
	var one int
	// @cross-tenant: ADR 0092 #9 — a group live in commune A is refused to commune B's code until A unlinks;
	// 0029's zalo_group_link_one_live_per_chat is platform-wide on (bot_ref, chat_id). One yes/no, no row.
	err := z.db.QueryRowContext(ctx, `SELECT 1 FROM zalo_group_link
		WHERE bot_ref = $1 AND chat_id = $2 AND unlinked_at IS NULL LIMIT 1`, botRef, chatID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crosstenant: is the chat a live group: %w", err)
	}
	return true, nil
}
