package app

// "Tắt / Bật" of a report sentence (ADR 0079 Q2, migration 0004; every key since migration 0005). The
// same use case as service-petitions' system_message_custom.go SetActive, copied, never imported
// (rule 2). Only the configuration list reads these today; the report export and notifications, when
// built, must read through ResolveMessage and decide per sentence "hidden" (Active false) or "default"
// (CurrentText), or a switched-off wording is printed on a report leadership signs.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// SetActive is "Tắt" (active false) or "Bật" (active true) of a SHIPPED key.
//
// EVERY SHIPPED KEY HAS A SWITCH (user decision 09/10/2026, migration 0005), reworded or not. A
// switched-off sentence is HIDDEN where it is used, and a consumer that must say something falls back
// to the shipped sentence — which kind each consumer is, is decided at the consumer (Active vs
// CurrentText). "Tắt" of an unworded key stores a row with no wording; "Bật" of it soft deletes that row.
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
		before := domain.ResolveMessage(shipped, cur)
		if before.Active == active {
			after = before
			return nil
		}
		at := uc.now().UTC()
		switch {
		case cur == nil:
			// "Tắt" of a sentence the commune never reworded (no row means on, so only "Tắt" gets
			// here): a row with NO wording, switched off. Never a copy of the default — that would pin
			// it against a corrected default in a later release.
			o := domain.MessageOverride{Key: key, Inactive: true, UpdatedAt: at, UpdatedBy: actor.ID}
			if o.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("system_message: sinh mã: %w", err)
			}
			if err := uc.store.AddOverride(ctx, tx, o); err != nil {
				return err
			}
			after = domain.ResolveMessage(shipped, &o)
		case cur.Text == "":
			// "Bật" of an unworded sentence: the row has nothing left to say, so it is soft deleted
			// (rule 7, invariant 1) and the commune is back to "no row" — the CHECK of migration 0005
			// refuses a live unworded row switched on.
			if err := uc.store.SoftDelete(ctx, tx, cur.ID, actor.ID, SwitchOnReason, at); err != nil {
				return err
			}
			after = domain.ResolveMessage(shipped, nil)
		default:
			if err := uc.store.SetActive(ctx, tx, cur.ID, active, actor.ID, at); err != nil {
				return err
			}
			o := *cur
			o.Inactive, o.UpdatedAt, o.UpdatedBy = !active, at, actor.ID
			after = domain.ResolveMessage(shipped, &o)
		}
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
