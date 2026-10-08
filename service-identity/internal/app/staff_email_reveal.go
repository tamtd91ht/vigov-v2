package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ActionRevealStaffEmail is the audit verb for reading ONE staff member's FULL email — the "Xem"
// button beside the masked address in the task drawer and the staff picker (owner decision
// 08/10/2026, ADR 0082 §3). The VALUE follows the house convention (ADR 0011, Vietnamese snake_case
// like `xem_day_du_nguoi_gui` in petitions); the identifier is English (rule 12).
const ActionRevealStaffEmail = "xem_email_can_bo"

// revealStaffEmailPermission is the key the route declares, recorded in the entry so an inspection
// reads under which right the address was opened. `task.read` is seeded by migration 0001 — NO KEY
// WAS ADDED for this (rule 5, invariant 3c; the owner chose this scope on 08/10/2026).
const revealStaffEmailPermission = "task.read"

// StaffEmailReveal discloses one staff member's full email and records the disclosure.
//
// WHY AN AUDIT ENTRY ON A READ: rule 6, invariant 7 — reading FULL personal data is itself audited.
// The masked address on the picker is not; this is. Every click is one row: two reads are two
// disclosures (the same reading as petitions' xem_nguoi_gui.go), so there is no idempotency key.
//
// WHY THE READ AND THE ENTRY SHARE ONE TRANSACTION: the address leaves this use case only after the
// entry has COMMITTED. A failed entry rolls back and the caller answers 500 with nothing disclosed —
// the one ordering rule 6 permits.
type StaffEmailReveal struct {
	db    *store.DB
	staff *idstore.CanBoStore
}

func NewStaffEmailReveal(db *store.DB, staff *idstore.CanBoStore) *StaffEmailReveal {
	if db == nil || staff == nil {
		panic("app.NewStaffEmailReveal: db and staff are required")
	}
	return &StaffEmailReveal{db: db, staff: staff}
}

// Reveal returns the full email of the staff member with business code `code` in the commune of ctx.
//
// idstore.ErrCanBoKhongTonTai for an unknown code, another commune's code, a soft-deleted row and a
// person with no address — one answer for all four (rule 4, forbidden #2); nothing is audited then,
// because nothing was disclosed.
//
// The actor's `Vet.ID` is the reader's STAFF CODE (rule 6, invariant 8); actor.hopLe refuses an empty
// one before the transaction opens, so a disclosure can never be attributed to nobody.
func (uc *StaffEmailReveal) Reveal(ctx context.Context, code string, actor NguoiThucHien) (string, error) {
	if err := actor.hopLe(); err != nil {
		return "", err
	}
	if code == "" {
		return "", idstore.ErrCanBoKhongTonTai
	}

	delta, err := json.Marshal(map[string]any{
		// The FIELD opened, never its value: the trail must not become a second store of the
		// address (rule 6, forbidden #4).
		"truong_da_mo": []string{"email"},
		"quyen":        revealStaffEmailPermission,
	})
	if err != nil {
		return "", fmt.Errorf("xem_email_can_bo: delta: %w", err)
	}

	var email string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		e, err := uc.staff.EmailForReveal(ctx, tx, code)
		if err != nil {
			return err
		}
		// TenantID left unset: audit.Write takes it from the transaction, which took it from the
		// context — one source for the commune (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRevealStaffEmail,
			Subject: code, // the staff BUSINESS code whose address was read
			Delta:   delta,
		}); err != nil {
			return err
		}
		email = e
		return nil
	})
	if err != nil {
		// Never the address, never the code in the message: the commune is what an operator needs.
		return "", fmt.Errorf("xem_email_can_bo: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return email, nil
}
