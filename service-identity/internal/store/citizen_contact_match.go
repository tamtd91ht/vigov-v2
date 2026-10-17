package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
)

// The two reads behind the directory ↔ petition link (ADR 0102; identity.proto FindCitizenContactByPhone
// and GetCitizenContactMatchPhone). The full-number read of the second RPC is ReadPhone
// (citizen_directory.go) — the same live predicate the directory's own "Xem số đầy đủ" uses, so the link
// can never see a row the directory screen does not.

// LiveIDByPhone returns the id of the LIVE contact of this commune whose stored number is `national`.
// found == false for no such row — including a row that exists but is soft-deleted, and a row of another
// commune (rule 4, forbidden #2: one answer).
//
// `national` MUST ALREADY BE IN THE STORED FORM (core/phone.NormalizeNational): this is an equality on
// `phone`, so a typed "0900 000 000" matches nothing. Folding is the caller's, once, so the fold that
// decides the match is visible at the use case and not buried here.
//
// At most one row: `UNIQUE (tenant_id, phone, live_key)` (migration 0033) admits one live row per number
// per commune, and that index is also what serves this lookup. `is_active` is NOT filtered — a contact
// switched to "ngừng liên hệ" is still that person's row (identity.proto).
//
// NOT IN A TRANSACTION: it reads one id and writes nothing, and the contract does not audit it. The
// number never enters the error (rule 3).
func (s *CitizenDirectoryStore) LiveIDByPhone(ctx context.Context, national string) (string, bool, error) {
	if national == "" {
		// "" is stored by nobody (CHECK phone ~ '^[0-9]{9,15}$'); never let an empty key reach SQL.
		return "", false, nil
	}
	// Scoped.Query binds tenant_id = $1 from the context's commune (rule 1, invariant 5).
	rows, err := s.db.For(ctx).Query(ctx, `id`, "citizen_contact",
		citizenContactLiveFilter+` AND phone = $2 LIMIT 1`, national)
	if err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: tìm dòng theo số: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", false, fmt.Errorf("danh bạ người dân: tìm dòng theo số: %w", err)
		}
		return "", false, nil
	}
	var id string
	if err := rows.Scan(&id); err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: đọc mã dòng theo số: %w", err)
	}
	return id, true, rows.Err()
}

// LiveStaffCodeExists reports whether `code` is the `nguoi_dung.ma` of a LIVE (not soft-deleted) staff
// member of this commune — the actor check of GetCitizenContactMatchPhone.
//
// DELIBERATELY NOT StaffCodeExists (notice_audience.go), which counts soft-deleted rows: a notice job may
// run after its sender left, so the sender stays the actor. Here the staff member is viewing a screen
// RIGHT NOW — the caller resolved their principal for this very request — so a removed code is a wiring
// fault, and the contract answers it INVALID_ARGUMENT rather than recording a disclosure to nobody
// current (identity.proto). Locked accounts are NOT excluded: the contract says "live", and locking is a
// sign-in state, not removal from the register.
func (s *CanBoStore) LiveStaffCodeExists(ctx context.Context, tx *store.ScopedTx, code string) (bool, error) {
	if tx == nil {
		return false, ErrThieuGiaoDich
	}
	if strings.TrimSpace(code) == "" {
		return false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `1`, "nguoi_dung", `AND deleted_at IS NULL AND ma = $2 LIMIT 1`, code)
	if err != nil {
		return false, fmt.Errorf("liên kết danh bạ: kiểm mã cán bộ xem: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("liên kết danh bạ: kiểm mã cán bộ xem: %w", err)
	}
	return found, nil
}
