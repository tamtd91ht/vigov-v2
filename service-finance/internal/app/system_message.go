package app

// The use cases behind "Lời hệ thống" for the sentences THIS service raises (14-cau-hinh §7,
// ADR 0024 §`loi_he_thong`, migration 0010).
//
// WHY THIS LAYER: every write here is read-decide-write on the commune's live wording, and the
// audit entry has to share the transaction with it (rule 6, invariant 3). core/audit.Write takes a
// *store.ScopedTx and nothing else, so opening that transaction is this layer's job.
//
// THE PETITIONS HALF COPIES THIS FILE, the store, the domain catalogue, the handler and migration
// 0010 into its own service — it does NOT import them (rule 2, forbidden #1), and the two tables
// live in two schemas (rule 2, invariant 2).

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// MessageOverrideStore is the store, declared at the point of use. Every WRITE method takes the
// transaction, so there is no signature that writes the row in one transaction and the audit entry
// in another. The one read without a transaction, ListLive, goes through db.For(ctx) in the store.
type MessageOverrideStore interface {
	ListLive(ctx context.Context) ([]domain.MessageOverride, error)
	LiveForUpdate(ctx context.Context, tx *store.ScopedTx, key string) (*domain.MessageOverride, error)
	AddOverride(ctx context.Context, tx *store.ScopedTx, o domain.MessageOverride) error
	UpdateText(ctx context.Context, tx *store.ScopedTx, o domain.MessageOverride) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// The business verbs written into the trail. Vietnamese snake_case like every other action this
// system writes: an inspection reads these strings.
const (
	ActionRewordSystemMessage  = "sua_loi_he_thong"
	ActionRestoreSystemMessage = "khoi_phuc_loi_he_thong_mac_dinh"
)

// RestoreReason is what `delete_reason` holds on a reverted wording. A FIXED SENTENCE, not a field
// the administrator fills: the act IS the reason ("go back to the software's sentence"), and the
// wording that was retired is in the audit entry beside it. Asking for free text on a button whose
// whole meaning is "undo my commune's change" would be friction that records nothing new.
const RestoreReason = "khôi phục câu mặc định"

// SystemMessages owns listing, rewording and restoring this service's system sentences.
type SystemMessages struct {
	db    *store.DB
	store MessageOverrideStore

	// Injected so a test can pin them. ulid.Moi and time.Now in production.
	newID func() (string, error)
	now   func() time.Time
}

func NewSystemMessages(db *store.DB, s MessageOverrideStore) *SystemMessages {
	return &SystemMessages{db: db, store: s, newID: ulid.Moi, now: time.Now}
}

// Messages returns every shipped key with the text in force for the request's commune.
//
// THIS IS THE FALLBACK: a key with no live row reads the default from code (domain.ResolveMessage).
// Any route that later EMITS one of these sentences must read it through here, never the constant
// directly, or the commune's wording is ignored (rule 1, invariant 10). A STORE FAILURE IS AN
// ERROR, NOT THE DEFAULT: falling back when the read fails would show a commune words it replaced,
// with nothing saying so.
func (uc *SystemMessages) Messages(ctx context.Context) ([]domain.SystemMessage, error) {
	live, err := uc.store.ListLive(ctx)
	if err != nil {
		return nil, wrapMessage(ctx, "đọc", err)
	}
	byKey := make(map[string]*domain.MessageOverride, len(live))
	for i := range live {
		byKey[live[i].Key] = &live[i]
	}
	shipped := domain.ShippedMessages()
	out := make([]domain.SystemMessage, 0, len(shipped))
	for _, m := range shipped {
		out = append(out, domain.ResolveMessage(m, byKey[m.Key]))
	}
	return out, nil
}

// Reword sets the commune's own wording of one key.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — including sending the DEFAULT while the commune is on
// the default. Pressing `Lưu` on an untouched card must not pin the current default into a row:
// that row would stop the commune receiving a corrected default in a later release, silently.
func (uc *SystemMessages) Reword(ctx context.Context, key, rawText string, actor audit.Actor) (domain.SystemMessage, error) {
	shipped, ok := domain.LookupShippedMessage(key)
	if !ok {
		return domain.SystemMessage{}, domain.ErrUnknownMessageKey
	}
	text, err := domain.NormalizeMessageText(rawText)
	if err != nil {
		return domain.SystemMessage{}, err
	}
	if actor.ID == "" {
		// `updated_by` with nothing in it is a change nobody can be asked about (rule 6, inv 8).
		return domain.SystemMessage{}, domain.ErrMessageActorMissing
	}

	var after domain.SystemMessage
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.store.LiveForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		before := domain.ResolveMessage(shipped, cur)
		if before.CurrentText == text {
			after = before
			return nil
		}

		o := domain.MessageOverride{Key: key, Text: text, UpdatedAt: uc.now().UTC(), UpdatedBy: actor.ID}
		if cur == nil {
			if o.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("system_message: sinh mã: %w", err)
			}
			if err := uc.store.AddOverride(ctx, tx, o); err != nil {
				return err
			}
		} else {
			o.ID = cur.ID
			if err := uc.store.UpdateText(ctx, tx, o); err != nil {
				return err
			}
		}
		after = domain.ResolveMessage(shipped, &o)
		return writeMessageAudit(ctx, tx, actor, ActionRewordSystemMessage, before, after)
	})
	if err != nil {
		return domain.SystemMessage{}, wrapMessage(ctx, "sửa", err)
	}
	return after, nil
}

// Restore takes one key back to the software's sentence for the request's commune.
//
// ALREADY ON THE DEFAULT IS NOT AN ERROR AND WRITES NOTHING: the state the caller asked for holds,
// and an entry saying "restored" when nothing changed would bury the entries that carry weight.
func (uc *SystemMessages) Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error) {
	shipped, ok := domain.LookupShippedMessage(key)
	if !ok {
		return domain.SystemMessage{}, domain.ErrUnknownMessageKey
	}
	if actor.ID == "" {
		return domain.SystemMessage{}, domain.ErrMessageActorMissing
	}

	after := domain.ResolveMessage(shipped, nil)
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.store.LiveForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		if cur == nil {
			return nil
		}
		if err := uc.store.SoftDelete(ctx, tx, cur.ID, actor.ID, RestoreReason, uc.now().UTC()); err != nil {
			return err
		}
		return writeMessageAudit(ctx, tx, actor, ActionRestoreSystemMessage, domain.ResolveMessage(shipped, cur), after)
	})
	if err != nil {
		return domain.SystemMessage{}, wrapMessage(ctx, "khôi phục", err)
	}
	return after, nil
}

// writeMessageAudit files the entry IN THE CALLER'S TRANSACTION (rule 6, invariant 3), with the
// before and after text (invariant 5). The text is a sentence the authority prints for its staff,
// not personal data (rule 3).
//
// SUBJECT IS THE KEY: it is the business identifier an inspection searches by; the row id names
// nothing and changes on every revert.
func writeMessageAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	before, after domain.SystemMessage) error {

	delta, err := json.Marshal(map[string]any{
		"truoc": map[string]any{"text": before.CurrentText, "overridden": before.Overridden},
		"sau":   map[string]any{"text": after.CurrentText, "overridden": after.Overridden},
	})
	if err != nil {
		return fmt.Errorf("system_message: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   actor,
		Action:  action,
		Subject: after.Key,
		Delta:   delta,
	})
}

// wrapMessage adds the commune and the operation, nothing else — no text, no actor: errors travel
// into centralised logging across every commune. %w keeps refusals distinguishable from failures.
func wrapMessage(ctx context.Context, op string, err error) error {
	return fmt.Errorf("system_message: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
