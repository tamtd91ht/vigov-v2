package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/phone"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The two use cases behind the directory ↔ petition link (ADR 0102): FindCitizenContactByPhone and
// GetCitizenContactMatchPhone in identity.proto, whose comments are the contract this file implements.
// The link is DERIVED from (tenant_id, national phone) each time; nothing here stores the other side.

// ActionRevealCitizenContactMatchPhone is the audit verb for handing a directory row's FULL number to
// service-petitions so it can list that person's petitions — "mở số để xem phản ánh của người dân" (ADR
// 0102 #2, #8). The VALUE follows the house convention (ADR 0011, Vietnamese snake_case like
// `xem_email_can_bo`); the identifier is English (rule 12).
//
// A DIFFERENT VERB from ActionRevealCitizenDirectoryPhone ("Xem số đầy đủ": the number reaches a PERSON's
// screen) and from ActionRevealCitizenContactPhone (a citizen's session number attached to their own
// petition): here the number reaches ANOTHER SERVICE's memory, for a staff member's lookup. An inspection
// asking "who looked up whose petitions" filters on this string alone.
const ActionRevealCitizenContactMatchPhone = "mo_so_xem_phan_anh_nguoi_dan"

// contactMatchPurpose is the one purpose ADR 0102 grants this disclosure — recorded so an inspection
// reads WHY the number left identity. A second purpose is a new owner decision, not a new value here.
const contactMatchPurpose = "xem_phan_anh_cua_nguoi_dan"

// ErrCitizenContactMatchInvalid marks every request the contract answers INVALID_ARGUMENT. Wrapped with
// the reason; the reason never carries a number, a contact id or a staff code (rule 3).
var ErrCitizenContactMatchInvalid = errors.New("yêu cầu liên kết phản ánh với danh bạ không hợp lệ")

// The reads, declared at the point of use. *idstore.CitizenDirectoryStore and *idstore.CanBoStore.
type (
	CitizenContactMatchReader interface {
		LiveIDByPhone(ctx context.Context, national string) (string, bool, error)
		ReadPhone(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error)
	}
	LiveStaffCodeChecker interface {
		LiveStaffCodeExists(ctx context.Context, tx *store.ScopedTx, code string) (bool, error)
	}
)

// CitizenContactMatch serves both RPCs.
type CitizenContactMatch struct {
	db       *store.DB
	contacts CitizenContactMatchReader
	staff    LiveStaffCodeChecker
}

func NewCitizenContactMatch(db *store.DB, contacts CitizenContactMatchReader, staff LiveStaffCodeChecker) *CitizenContactMatch {
	if db == nil || contacts == nil || staff == nil {
		panic("app.NewCitizenContactMatch: db, contacts and staff are required")
	}
	return &CitizenContactMatch{db: db, contacts: contacts, staff: staff}
}

// FindByPhone answers whether `raw` — the number as the petition holds it — has a LIVE directory row in
// the commune of ctx, and that row's id.
//
// THE FOLD IS core/phone.NormalizeNational, the one service-petitions uses for its own normalised column
// (ADR 0102 #3). It is pinned equal to the fold that WROTE `citizen_contact.phone`
// (domain.NormalizeCitizenPhone) by citizen_contact_match_fold_test.go, so "found" is an equality on one
// spelling of the number on both sides.
//
// A number that does not fold is found = false, NOT an error: the contract makes it indistinguishable
// from "not in the directory" (identity.proto), and the caller offers "Thêm người dân" for both. Only an
// empty number is ErrCitizenContactMatchInvalid — the caller must not call without one.
//
// NOT AUDITED, writes nothing: it returns no personal data (identity.proto §Not audited).
func (uc *CitizenContactMatch) FindByPhone(ctx context.Context, raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return "", false, errors.Join(ErrCitizenContactMatchInvalid, errors.New("thiếu số điện thoại"))
	}
	national, err := phone.NormalizeNational(raw)
	if err != nil {
		return "", false, nil
	}
	id, found, err := uc.contacts.LiveIDByPhone(ctx, national)
	if err != nil {
		// The commune is what an operator needs; never the number (rule 3).
		return "", false, fmt.Errorf("tim_danh_ba_theo_so: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	if !found || id == "" {
		return "", false, nil
	}
	return id, true, nil
}

// MatchPhone returns the FULL national number of the LIVE directory row `contactID`, for service-petitions
// to key its own query on, and records the disclosure.
//
// ORDER INSIDE THE ONE TRANSACTION: actor check, read, entry. The number leaves only after the entry has
// COMMITTED (the ordering of StaffEmailReveal, ADR 0082); a failed entry rolls back and nothing is
// disclosed. found == false — unknown, soft-deleted or another commune's row — writes NOTHING, because
// nothing was disclosed (identity.proto).
//
// THE ACTOR is the staff member viewing, by business code (rule 6, invariant 8) — `actor_code`, which
// petitions took from authz.Principal.Ma of the request it is serving. It must be a LIVE `nguoi_dung.ma`
// of this commune, checked inside the same transaction; anything else is ErrCitizenContactMatchInvalid,
// never a fallback actor. IP is EMPTY: identity observes no browser on this hop.
//
// EVERY CALL THAT DISCLOSES IS ONE ENTRY — a retry is another entry (identity.proto: "safe for the data,
// not free for the trail").
func (uc *CitizenContactMatch) MatchPhone(ctx context.Context, contactID, actorCode string) (string, bool, error) {
	switch {
	case contactID == "":
		return "", false, errors.Join(ErrCitizenContactMatchInvalid, errors.New("thiếu contact_id"))
	case strings.TrimSpace(actorCode) == "":
		// NEVER a fallback actor: an entry naming nobody cannot answer "who looked up whose petitions".
		return "", false, errors.Join(ErrCitizenContactMatchInvalid, errors.New("thiếu actor_code"))
	}

	delta, err := json.Marshal(map[string]any{
		// The FIELD opened and the purpose, never the value: the trail must not become a second store
		// of the number (rule 6, forbidden #4). The shape of the other reveal entries of this service.
		"truong_da_mo": []string{"so_dien_thoai"},
		"muc_dich":     contactMatchPurpose,
	})
	if err != nil {
		return "", false, fmt.Errorf("mo_so_xem_phan_anh_nguoi_dan: delta: %w", err)
	}

	var (
		number string
		found  bool
	)
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		ok, err := uc.staff.LiveStaffCodeExists(ctx, tx, actorCode)
		if err != nil {
			return err
		}
		if !ok {
			return errors.Join(ErrCitizenContactMatchInvalid, errors.New("actor_code không phải mã cán bộ còn hiệu lực của xã này"))
		}
		p, ok, err := uc.contacts.ReadPhone(ctx, tx, contactID)
		if err != nil {
			return err
		}
		if !ok || p == "" {
			return nil // nothing disclosed, nothing written
		}
		// TenantID left unset: audit.Write takes it from the transaction, which took it from the
		// context — one source for the commune (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: actorCode, Kind: "staff", IP: ""},
			Action:  ActionRevealCitizenContactMatchPhone,
			Subject: contactID, // a directory row has no business code; its id is what the screen opens
			Delta:   delta,
		}); err != nil {
			return err
		}
		number, found = p, true
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCitizenContactMatchInvalid) {
			return "", false, err
		}
		// Never the number, the contact id or the code in the message: the commune is what an operator
		// needs (rule 3).
		return "", false, fmt.Errorf("mo_so_xem_phan_anh_nguoi_dan: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return number, found, nil
}
