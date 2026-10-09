package app

// "Tắt / Bật lại" and the commune's own sentences on "Lời hệ thống", group "Giải ngân" (ADR 0079 Q2,
// Q5a, Q5b; migration 0017 §A, §B). A COPY of service-petitions' file of the same name (rule 2). Same transaction rule as system_message.go: every write and its audit entry
// share one transaction (rule 6, invariant 3).
//
// THE COMMUNE'S SENTENCES ARE STORED AND MANAGED ONLY (ADR 0079 Q5b). Nothing in this service resolves
// one: Text reads shipped keys only, and no route, job or message sends a commune sentence anywhere.
// Wiring one into a refusal, a ZNS message or the Mini App is a later card's decision — and a rule 4
// question if a citizen reads it.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// SetActive is "Tắt" (active false) or "Bật" (active true) of a SHIPPED key. While off, the
// `scope_notice` banner the investment-project reads print is NOT SENT (du_an.go scopeNotice).
//
// EVERY SHIPPED KEY HAS A SWITCH (user decision 09/10/2026, migration 0019), reworded or not. A
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
			// (rule 7, invariant 1) and the commune is back to "no row" — the CHECK of migration 0019
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

// NewCustomMessage is one commune sentence as it arrives from the handler. Description nil = none.
type NewCustomMessage struct {
	Group       string
	Key         string
	Text        string
	Description *string
}

// CustomMessageEdit is a PARTIAL edit: nil = leave alone. Description has three states like a
// catalogue colour: nil = leave it; a change whose Value is nil (or blank) = clear it; a value = set it.
type CustomMessageEdit struct {
	Text        *string
	Description *DescriptionChange
	Active      *bool
}

// DescriptionChange is one edit of a commune sentence's description. Value nil clears it.
type DescriptionChange struct {
	Value *string
}

// CreateCustom adds one commune sentence, switched on.
//
// ORDER OF REFUSALS: shape (no transaction), then the ceiling, then the taken key — the same order as
// the catalogues, for the same reason (danh_muc_hang_muc_ke_hoach_von.go Them).
func (uc *SystemMessages) CreateCustom(ctx context.Context, in NewCustomMessage, actor audit.Actor) (domain.SystemMessage, error) {
	group, key, err := domain.NormalizeCustomKey(in.Group, in.Key)
	if err != nil {
		return domain.SystemMessage{}, err
	}
	text, err := domain.NormalizeMessageText(in.Text)
	if err != nil {
		return domain.SystemMessage{}, err
	}
	var desc string
	if in.Description != nil {
		if desc, err = domain.NormalizeDescription(*in.Description); err != nil {
			return domain.SystemMessage{}, err
		}
	}
	if actor.ID == "" {
		return domain.SystemMessage{}, domain.ErrMessageActorMissing
	}
	id, err := uc.newID()
	if err != nil {
		return domain.SystemMessage{}, fmt.Errorf("system_message: sinh mã: %w", err)
	}
	at := uc.now().UTC()
	c := domain.CustomMessage{
		ID: id, Group: group, Key: key, Text: text, Description: desc, Active: true,
		CreatedAt: at, CreatedBy: actor.ID, UpdatedAt: at, UpdatedBy: actor.ID,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.store.CountLiveCustom(ctx, tx)
		if err != nil {
			return err
		}
		if n >= domain.TranCustomMessages {
			return domain.ErrCustomCatalogueFull
		}
		taken, err := uc.store.CustomKeyTaken(ctx, tx, key)
		if err != nil {
			return err
		}
		if taken {
			return domain.ErrMessageCodeTaken
		}
		if err := uc.store.AddCustom(ctx, tx, c); err != nil {
			return err
		}
		return writeCustomAudit(ctx, tx, actor, ActionAddCustomMessage, key, nil, &c)
	})
	if err != nil {
		return domain.SystemMessage{}, wrapMessage(ctx, "thêm câu", err)
	}
	return domain.CustomToMessage(c), nil
}

// EditCustom applies a partial edit to one live commune sentence. A shipped key answers
// ErrShippedMessageNotCustom — its words change through Reword, its switch through SetActive.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING (the route's idem.KhongCan rests on it). When only the
// switch moved, the entry is "tắt"/"bật lại", not "sửa" — the act an inspection searches for.
func (uc *SystemMessages) EditCustom(ctx context.Context, key string, in CustomMessageEdit, actor audit.Actor) (domain.SystemMessage, error) {
	if _, shipped := domain.LookupShippedMessage(key); shipped {
		return domain.SystemMessage{}, domain.ErrShippedMessageNotCustom
	}
	var text, desc string
	var err error
	if in.Text != nil {
		if text, err = domain.NormalizeMessageText(*in.Text); err != nil {
			return domain.SystemMessage{}, err
		}
	}
	if in.Description != nil && in.Description.Value != nil {
		if desc, err = domain.NormalizeDescription(*in.Description.Value); err != nil {
			return domain.SystemMessage{}, err
		}
	}
	if actor.ID == "" {
		return domain.SystemMessage{}, domain.ErrMessageActorMissing
	}

	var after domain.CustomMessage
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.store.CustomForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		if cur == nil {
			return domain.ErrCustomMessageNotFound
		}
		after = *cur
		if in.Text != nil {
			after.Text = text
		}
		if in.Description != nil {
			after.Description = desc // "" when the change clears it
		}
		if in.Active != nil {
			after.Active = *in.Active
		}
		if after.Text == cur.Text && after.Description == cur.Description && after.Active == cur.Active {
			after = *cur
			return nil
		}
		after.UpdatedAt, after.UpdatedBy = uc.now().UTC(), actor.ID
		if err := uc.store.UpdateCustom(ctx, tx, after); err != nil {
			return err
		}
		action := ActionEditCustomMessage
		if after.Text == cur.Text && after.Description == cur.Description {
			action = switchAction(after.Active)
		}
		return writeCustomAudit(ctx, tx, actor, action, key, cur, &after)
	})
	if err != nil {
		return domain.SystemMessage{}, wrapMessage(ctx, "sửa câu", err)
	}
	return domain.CustomToMessage(after), nil
}

// DeleteCustom soft deletes one commune sentence (rule 7, invariant 1). Its key stays taken for ever
// (invariant 3) and the deleted row is frozen by the database (0017 custom_system_message_guard). A
// shipped key answers ErrShippedMessageNotDeletable.
func (uc *SystemMessages) DeleteCustom(ctx context.Context, key, rawReason string, actor audit.Actor) error {
	if _, shipped := domain.LookupShippedMessage(key); shipped {
		return domain.ErrShippedMessageNotDeletable
	}
	reason, err := domain.NormalizeCustomDeleteReason(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return domain.ErrMessageActorMissing
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.store.CustomForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		if cur == nil {
			return domain.ErrCustomMessageNotFound
		}
		if err := uc.store.SoftDeleteCustom(ctx, tx, cur.ID, actor.ID, reason, uc.now().UTC()); err != nil {
			return err
		}
		return writeCustomAuditDelete(ctx, tx, actor, cur, reason)
	})
	if err != nil {
		return wrapMessage(ctx, "xoá câu", err)
	}
	return nil
}

// customAuditView is the trail's view of one commune sentence. The text is a sentence the authority
// wrote, not personal data (rule 3).
func customAuditView(c *domain.CustomMessage) map[string]any {
	if c == nil {
		return nil
	}
	var desc any
	if c.Description != "" {
		desc = c.Description
	}
	return map[string]any{"group_code": c.Group, "text": c.Text, "description": desc, "is_active": c.Active}
}

func writeCustomAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action, key string,
	before, after *domain.CustomMessage) error {

	d := map[string]any{"sau": customAuditView(after)}
	if before != nil {
		d["truoc"] = customAuditView(before)
	}
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("system_message: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: key, Delta: delta})
}

func writeCustomAuditDelete(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, before *domain.CustomMessage, reason string) error {
	delta, err := json.Marshal(map[string]any{"truoc": customAuditView(before), "ly_do": reason, "xoa_mem": true})
	if err != nil {
		return fmt.Errorf("system_message: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: ActionDeleteCustomMessage, Subject: before.Key, Delta: delta})
}
