package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// The FIFTH cross-tenant read shape (ADR 0097 §3; crosstenant/doc.go shape 5): the "Zalo" column of
// the citizen contact directory.
//
// THE QUESTION IT ANSWERS: "this number, in the directory of commune X — is it signed in to commune
// X's Mini App right now?" One boolean per asked number, and nothing else.
//
// WHY IT IS CROSS-TENANT AT ALL: `dinh_danh_cong_dan` has no tenant_id (ADR 0002 — one phone number is
// one identity for the whole platform), so the join crosses from a commune-scoped table to a
// platform-wide one. What keeps it on the right side of the boundary is that the SESSION row is bound
// to THIS commune (`p.tenant_id = $1`, bound by the scoped transaction from the context — rule 1,
// invariant 3), exactly as citizen_contact_phone.go:19-23 does it. A session that number holds in
// another commune does not satisfy the predicate, so commune A learns nothing about commune B.
//
// WHAT IT NEVER RETURNS (ADR 0097 §3, invariants 3 and 4): no cong_dan_id, no session id, no Zalo
// account, no sign-in time, and no "this number exists in dinh_danh_cong_dan" — a number with an
// identity row and no live session HERE is indistinguishable from a number nobody ever used.
//
// WHY THERE IS NO STORED FLAG (§3 invariant 5): `het_han_luc > now()` is a comparison at read time;
// a column written when a session opens is wrong the moment it expires (rule 10, invariant 3).
//
// WHY `tx` AND NOT THE POOL: the ADR fixes the shape on ScopedTx (§3 invariant 2), the same way the
// one other read that joins the identity table does (ContactPhoneForReveal). A caller that only has
// the context reads the flag as a COLUMN of the directory page instead (citizen_directory.go), built
// from the same predicate text below, so the two can never disagree about what "signed in" means.

// ownAppSessionPredicate is the ONE spelling of "a live session of THIS commune holds this number".
//
// %s is the identity-form phone expression ('84…', domain.CitizenPhoneIdentityForm): a bound array
// element in OwnAppSessionByPhone, `'84' || substr(citizen_contact.phone, 2)` on the directory page.
// `$1` is the commune, always — both callers bind it from the scoped handle.
//
//   - d.id = p.cong_dan_id    an INNER join, so a session with no verified number (cong_dan_id NULL,
//     ADR 0080) matches nothing;
//   - p.thu_hoi_luc IS NULL   a revoked session is "—";
//   - p.het_han_luc > now()   an expired one too — the comparison truyVanPhienTheoToken makes.
//
// NO deleted_at: neither table has soft-delete columns (migration 0004).
const ownAppSessionPredicate = `EXISTS (SELECT 1 FROM phien_cong_dan p` +
	` JOIN dinh_danh_cong_dan d ON d.id = p.cong_dan_id` +
	` WHERE p.tenant_id = $1 AND d.so_dien_thoai = %s` +
	` AND p.thu_hoi_luc IS NULL AND p.het_han_luc > now())`

// MaxOwnAppSessionBatch bounds one call: a directory page is at most page.MaxLimit rows, and the
// Excel import is at most domain.MaxCitizenContactImportRows; 500 is past both. Refused, not
// truncated — a short answer reads as "—" for real sessions.
const MaxOwnAppSessionBatch = 500

// ErrOwnAppSessionBatchTooLarge says the ceiling above was crossed.
var ErrOwnAppSessionBatchTooLarge = fmt.Errorf("danh bạ người dân: quá %d số trong một lần hỏi cột Zalo", MaxOwnAppSessionBatch)

// OwnAppSessionByPhone answers, for each identity-form phone asked (`84…`), whether a LIVE citizen
// session of the transaction's commune holds that number.
//
// THE MAP HOLDS EXACTLY THE PHONES ASKED (empty strings dropped, repeats once), each false unless the
// predicate holds — so a caller can never read "absent" as anything, and a phone that is unknown to
// the platform looks the same as one known elsewhere (ADR 0097 §3 invariant 4). An empty ask returns
// an empty map without a round trip.
//
// THE NUMBERS NEVER ENTER AN ERROR OR A LOG LINE (rule 3).
func (s *CitizenDirectoryStore) OwnAppSessionByPhone(ctx context.Context, tx *store.ScopedTx, identityPhones []string) (map[string]bool, error) {
	if tx == nil {
		return nil, ErrThieuGiaoDich
	}
	ask := make([]string, 0, len(identityPhones))
	out := make(map[string]bool, len(identityPhones))
	for _, p := range identityPhones {
		if p == "" {
			// domain.CitizenPhoneIdentityForm answers "" for a malformed number: fail closed, and never
			// let "" reach a comparison where it could only ever mean nothing.
			continue
		}
		if _, seen := out[p]; seen {
			continue
		}
		out[p] = false
		ask = append(ask, p)
	}
	if len(ask) == 0 {
		return out, nil
	}
	if len(ask) > MaxOwnAppSessionBatch {
		return nil, ErrOwnAppSessionBatchTooLarge
	}

	stmt := `SELECT u.phone, ` + fmt.Sprintf(ownAppSessionPredicate, "u.phone") +
		` FROM unnest($2::text[]) AS u(phone)`
	// @cross-tenant: ADR 0097 §3 — boolean per (tenant, phone); never returns an id, never reveals
	// another commune. dinh_danh_cong_dan has no tenant_id (ADR 0002); the session row is bound to
	// THIS commune by $1 from the scoped transaction, so a session elsewhere answers false.
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), ask)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đọc cột Zalo theo lô: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var phone string
		var live bool
		if err := rows.Scan(&phone, &live); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng cột Zalo: %w", err)
		}
		if _, asked := out[phone]; asked {
			out[phone] = live
		}
	}
	// Checked: a connection lost mid-result would leave real sessions reading "—".
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt cột Zalo: %w", err)
	}
	return out, nil
}
