package app

// The use cases behind "Lời hệ thống" for the sentences THIS service raises (14-cau-hinh §7,
// ADR 0024 §`loi_he_thong`, migration 0020).
//
// A COPY OF service-finance/internal/app/system_message.go (rule 2, forbidden #1 — never imported),
// plus the one thing finance did not need yet: Text, which the refusal branches read their sentence
// through.
//
// WHY THIS LAYER: every write here is read-decide-write on the commune's live wording, and the
// audit entry has to share the transaction with it (rule 6, invariant 3). core/audit.Write takes a
// *store.ScopedTx and nothing else, so opening that transaction is this layer's job.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
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
	// SetActive is "Tắt / Bật lại" (migration 0033 §A). It touches `is_active` and the who/when only.
	SetActive(ctx context.Context, tx *store.ScopedTx, id string, active bool, by string, at time.Time) error
}

// CustomMessageStore is the commune-sentence half (migration 0033 §B). Same shape of promise: every
// write takes the transaction the audit entry is written in.
type CustomMessageStore interface {
	// ListCustom reads the commune's LIVE sentences, bounded by domain.TranCustomMessages (an error
	// past it, never a truncated list).
	ListCustom(ctx context.Context) ([]domain.CustomMessage, error)
	CustomForUpdate(ctx context.Context, tx *store.ScopedTx, key string) (*domain.CustomMessage, error)
	// CustomKeyTaken counts SOFT-DELETED rows too: a deleted key is never reused (rule 7, inv 3).
	CustomKeyTaken(ctx context.Context, tx *store.ScopedTx, key string) (bool, error)
	CountLiveCustom(ctx context.Context, tx *store.ScopedTx) (int, error)
	AddCustom(ctx context.Context, tx *store.ScopedTx, c domain.CustomMessage) error
	UpdateCustom(ctx context.Context, tx *store.ScopedTx, c domain.CustomMessage) error
	SoftDeleteCustom(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// SystemMessageStore is everything "Lời hệ thống" stores in this service. ONE value satisfies both
// halves (store.SystemMessageOverrideStore), so wiring cannot give the screen its overrides and forget
// the commune sentences — the list merges both, and half a list would look complete.
type SystemMessageStore interface {
	MessageOverrideStore
	CustomMessageStore
}

// The business verbs written into the trail — the SAME strings finance writes, so an inspection
// searching "who reworded a system sentence" finds both services' entries with one query.
const (
	ActionRewordSystemMessage  = "sua_loi_he_thong"
	ActionRestoreSystemMessage = "khoi_phuc_loi_he_thong_mac_dinh"

	// "Tắt / Bật lại" (ADR 0079 Q2) — of a commune's wording of a shipped key, or of a commune sentence.
	ActionSwitchOffSystemMessage = "tat_loi_he_thong"
	ActionSwitchOnSystemMessage  = "bat_lai_loi_he_thong"

	// A commune sentence ("Xã tự thêm", ADR 0079 Q2, Q5a).
	ActionAddCustomMessage    = "them_cau_he_thong_cua_xa"
	ActionEditCustomMessage   = "sua_cau_he_thong_cua_xa"
	ActionDeleteCustomMessage = "xoa_cau_he_thong_cua_xa"
)

// RestoreReason is what `delete_reason` holds on a reverted wording. A FIXED SENTENCE, not a field
// the administrator fills: the act IS the reason, and the retired wording is in the audit entry.
const RestoreReason = "khôi phục câu mặc định"

// SwitchOnReason is `delete_reason` on the row "Bật" retires when the commune never reworded the
// sentence (migration 0035): that row held only the switch, so switching on ends it.
const SwitchOnReason = "bật lại câu mặc định"

// SystemMessages owns listing, rewording and restoring this service's system sentences, and
// resolving the one sentence a refusal branch is about to send.
type SystemMessages struct {
	db    *store.DB
	store SystemMessageStore

	// Injected so a test can pin them. ulid.Moi and time.Now in production.
	newID func() (string, error)
	now   func() time.Time
}

func NewSystemMessages(db *store.DB, s SystemMessageStore) *SystemMessages {
	return &SystemMessages{db: db, store: s, newID: ulid.Moi, now: time.Now}
}

// Messages returns every shipped key with the text in force for the request's commune — the
// configuration screen's read.
//
// A STORE FAILURE IS AN ERROR HERE, NOT THE DEFAULT: this is the screen where an administrator
// checks what the commune says, and showing the default when the read failed would tell them their
// own wording is gone. Text below makes the opposite call, for the opposite reader.
//
// THE COMMUNE'S OWN SENTENCES FOLLOW THE SHIPPED ONES (migration 0033 §B), in the store's order (group,
// then key). A failure of EITHER read fails the list: half a list looks complete.
func (uc *SystemMessages) Messages(ctx context.Context) ([]domain.SystemMessage, error) {
	live, err := uc.store.ListLive(ctx)
	if err != nil {
		return nil, wrapMessage(ctx, "đọc", err)
	}
	custom, err := uc.store.ListCustom(ctx)
	if err != nil {
		return nil, wrapMessage(ctx, "đọc câu xã tự thêm", err)
	}
	byKey := make(map[string]*domain.MessageOverride, len(live))
	for i := range live {
		byKey[live[i].Key] = &live[i]
	}
	shipped := domain.ShippedMessages()
	out := make([]domain.SystemMessage, 0, len(shipped)+len(custom))
	for _, m := range shipped {
		out = append(out, domain.ResolveMessage(m, byKey[m.Key]))
	}
	for _, c := range custom {
		out = append(out, domain.CustomToMessage(c))
	}
	return out, nil
}

// Text resolves ONE key to the sentence in force for the request's commune — what a refusal branch
// sends instead of a hardcoded string.
//
// # IT NEVER FAILS TO RETURN A SENTENCE FOR A SHIPPED KEY
//
// When the override cannot be read, the DEFAULT comes back WITH the error. The caller is in the
// middle of refusing a business request — a missing reason, a transition the lifecycle does not have
// — and the refusal is correct whatever the wording is. Turning it into a 500 because a WORDING could
// not be looked up would tell an officer the server is broken when what is wrong is their form, and
// the code the client branches on would change with it. The error is returned so the caller can log
// it; the sentence is returned so the caller never has to invent one.
//
// AN UNKNOWN KEY is a programming error — the keys are constants in domain — and answers "" with
// ErrUnknownMessageKey; the handler's test pins every key it reads to the catalogue.
func (uc *SystemMessages) Text(ctx context.Context, key string) (string, error) {
	shipped, ok := domain.LookupShippedMessage(key)
	if !ok {
		return "", domain.ErrUnknownMessageKey
	}
	live, err := uc.store.ListLive(ctx)
	if err != nil {
		return shipped.DefaultText, wrapMessage(ctx, "đọc câu "+key, err)
	}
	for i := range live {
		if live[i].Key == key {
			return domain.ResolveMessage(shipped, &live[i]).CurrentText, nil
		}
	}
	return shipped.DefaultText, nil
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
		// THE COMPARISON IS AGAINST THE COMMUNE'S WORDING when it has one, switched on or off — not
		// against the text in force. Re-sending the stored words of a switched-off wording changes
		// nothing; comparing to CurrentText (the default, while off) would rewrite it as "new".
		// A row switched off WITHOUT a wording (migration 0035) has no words: compare with the default,
		// or sending the default would pin it as the commune's "wording".
		stored := shipped.DefaultText
		if cur != nil && cur.Text != "" {
			stored = cur.Text
		}
		if stored == text {
			after = before
			return nil
		}

		// REWORDING DOES NOT FLIP THE SWITCH: a switched-off wording that is edited stays off until
		// "Bật lại" — the switch and the words are two acts, each with its own entry. An assumption of
		// this card, stated in its report.
		o := domain.MessageOverride{Key: key, Text: text, UpdatedAt: uc.now().UTC(), UpdatedBy: actor.ID}
		if cur != nil {
			o.Inactive = cur.Inactive
		}
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
// before and after text (invariant 5). The text is a sentence the authority prints, not personal
// data (rule 3).
//
// SUBJECT IS THE KEY: it is the business identifier an inspection searches by; the row id names
// nothing and changes on every revert.
func writeMessageAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	before, after domain.SystemMessage) error {

	delta, err := json.Marshal(map[string]any{
		"truoc": map[string]any{"text": before.CurrentText, "overridden": before.Overridden, "is_active": before.Active, "override_text": before.OverrideText},
		"sau":   map[string]any{"text": after.CurrentText, "overridden": after.Overridden, "is_active": after.Active, "override_text": after.OverrideText},
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
