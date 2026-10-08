package app

// "Tắt / Bật lại" of the commune's wording of a report sentence (ADR 0079 Q2, migration 0004). The
// same use case as service-petitions' system_message_custom.go SetActive, copied, never imported
// (rule 2). While off, every reader resolves the shipped sentence through domain.ResolveMessage — the
// configuration list today, and the report export and notifications when they are built (they must
// read through ResolveMessage too, or a switched-off wording is printed on a report leadership signs).

import (
	"context"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// SetActive is "Tắt" (active false) or "Bật lại" (active true) of the commune's wording of a SHIPPED
// key. While off, every reader resolves the shipped sentence (domain.ResolveMessage).
//
// ONLY A KEY THE COMMUNE REWORDED HAS A SWITCH: with no live wording there is nothing to switch, and a
// commune may not silence a shipped sentence — domain.ErrNoOverrideToSwitch (409).
//
// THE SAME STATE AGAIN WRITES NOTHING AND AUDITS NOTHING, which is what makes the route idempotent.
func (uc *SystemMessages) SetActive(ctx context.Context, key string, active bool, actor audit.Actor) (domain.SystemMessage, error) {
	shipped, ok := domain.LookupShippedMessage(key)
	if !ok {
		return domain.SystemMessage{}, domain.ErrUnknownMessageKey
	}
	if actor.ID == "" {
		return domain.SystemMessage{}, domain.ErrMessageActorMissing
	}

	var after domain.SystemMessage
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.store.LiveForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		if cur == nil {
			return domain.ErrNoOverrideToSwitch
		}
		before := domain.ResolveMessage(shipped, cur)
		if before.Active == active {
			after = before
			return nil
		}
		at := uc.now().UTC()
		if err := uc.store.SetActive(ctx, tx, cur.ID, active, actor.ID, at); err != nil {
			return err
		}
		o := *cur
		o.Inactive, o.UpdatedAt, o.UpdatedBy = !active, at, actor.ID
		after = domain.ResolveMessage(shipped, &o)
		return writeMessageAudit(ctx, tx, actor, switchAction(active), before, after)
	})
	if err != nil {
		return domain.SystemMessage{}, wrapMessage(ctx, "tắt/bật", err)
	}
	return after, nil
}

func switchAction(active bool) string {
	if active {
		return ActionSwitchOnSystemMessage
	}
	return ActionSwitchOffSystemMessage
}
