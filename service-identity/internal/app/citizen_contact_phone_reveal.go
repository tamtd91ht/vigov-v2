package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ActionRevealCitizenContactPhone is the audit verb for handing a citizen's FULL verified phone to
// the service filing a petition for that citizen's live session (ResolveCitizenContactPhone, owner
// decision 08/10/2026, ADR 0050 §Sửa đổi 08/10/2026 point 2). The VALUE follows the house convention
// (ADR 0011, Vietnamese snake_case like `xem_email_can_bo`); the identifier is English (rule 12).
const ActionRevealCitizenContactPhone = "mo_so_dien_thoai_cong_dan"

// contactPhonePurpose is the one purpose the owner decided — recorded in the entry so an inspection
// reads WHY the number left this service. A second purpose is a contract change (identity.proto).
const contactPhonePurpose = "gan_vao_phan_anh"

// CitizenContactPhoneReveal discloses the verified phone of the citizen behind one live session and
// records the disclosure.
//
// THE FIRST PATH IN THIS SERVICE THAT HANDS AN UNMASKED PHONE ACROSS A SERVICE BOUNDARY. Rule 6,
// invariant 7 audits it although it is a read; every call that discloses is one entry (two intakes,
// two disclosures — no idempotency key), the reading xem_email_can_bo already uses.
//
// THE READ AND THE ENTRY SHARE ONE TRANSACTION, and the number leaves only after the entry has
// committed (the ordering of StaffEmailReveal, ADR 0082). A failed entry rolls back and the caller
// answers Internal with nothing disclosed.
type CitizenContactPhoneReveal struct {
	db       *store.DB
	sessions *idstore.PhienCongDanStore
}

func NewCitizenContactPhoneReveal(db *store.DB, sessions *idstore.PhienCongDanStore) *CitizenContactPhoneReveal {
	if db == nil || sessions == nil {
		panic("app.NewCitizenContactPhoneReveal: db and sessions are required")
	}
	return &CitizenContactPhoneReveal{db: db, sessions: sessions}
}

// Reveal returns the full verified phone of `citizenID`, read through the live session `sessionID`
// of the commune in ctx.
//
// idstore.ErrNoContactPhone for every session that cannot disclose (unknown, revoked, expired,
// another commune's, unverified, another citizen's) — one answer, and NOTHING IS AUDITED then,
// because nothing was disclosed.
//
// THE ACTOR IS THE CITIZEN, by the opaque citizen id (identity.proto, ResolveCitizenContactPhone
// §audit): their live session is the authority and they pressed send. Rule 6 invariant 8's business
// code binds STAFF; a citizen has none, and an opaque id is what every citizen entry in this service
// carries (cau_phien_cong_dan.go, chuTheVet). NOT the calling service — ADR 0025's shared caller key
// cannot name it. The IP is EMPTY on purpose: identity never observes the citizen's socket, and a
// caller-relayed address would be a claim recorded with a government record's authority.
func (uc *CitizenContactPhoneReveal) Reveal(ctx context.Context, sessionID, citizenID string) (string, error) {
	if sessionID == "" || citizenID == "" {
		return "", idstore.ErrNoContactPhone
	}

	delta, err := json.Marshal(map[string]any{
		// The FIELD opened, never its value: the trail must not become a second store of the
		// number (rule 6, forbidden #4).
		"truong_da_mo": []string{"so_dien_thoai"},
		"muc_dich":     contactPhonePurpose,
		// The sid, so this entry can be matched to service-petitions' intake entry of the same act.
		// A sid is not a credential — it opens nothing at the edge (identity.proto).
		"phien": sessionID,
	})
	if err != nil {
		return "", fmt.Errorf("mo_so_dien_thoai_cong_dan: delta: %w", err)
	}

	var phone string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.sessions.ContactPhoneForReveal(ctx, tx, sessionID, citizenID)
		if err != nil {
			return err
		}
		// TenantID left unset: audit.Write takes it from the transaction, which took it from the
		// context — one source for the commune (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: citizenID, Kind: authz.KindCitizen, IP: ""},
			Action:  ActionRevealCitizenContactPhone,
			Subject: citizenID, // a citizen has no business code; the opaque id is what citizen entries carry
			Delta:   delta,
		}); err != nil {
			return err
		}
		phone = p
		return nil
	})
	if err != nil {
		// Never the number, the sid or the citizen id in the message: the commune is what an
		// operator needs (rule 3).
		return "", fmt.Errorf("mo_so_dien_thoai_cong_dan: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return phone, nil
}
