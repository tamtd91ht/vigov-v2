package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// ErrNoContactPhone is the ONE answer for every session that cannot disclose a phone: unknown sid,
// revoked, expired, another commune's, no verified phone (cong_dan_id NULL, ADR 0080), or a citizen
// other than the one named. Deliberately one sentinel — the gRPC contract
// (ResolveCitizenContactPhone) answers all six identically, and a store that told them apart would
// be the first place a caller learned which one it hit (rule 4, forbidden #2).
var ErrNoContactPhone = errors.New("phiên công dân: không có số điện thoại đã xác thực để gắn")

// contactPhoneJoin reads the verified number of the citizen behind ONE live session.
//
// THE COMMUNE IS BOUND BY ScopedTx.Query ($1), on the session row: `tenant_id` is unambiguous here
// because dinh_danh_cong_dan has no such column (migration 0004, `@scope: cross-tenant`). If that
// table ever gains one, PostgreSQL refuses this statement as ambiguous — it fails closed, it does not
// start matching another commune.
//
// THE PREDICATE IS THE WHOLE SECURITY PROPERTY of the RPC, every clause on the session row:
//   - p.id = $2               the session the citizen edge resolved — a sid, never a citizen id alone
//     (the contract explains why `citizen_id -> phone` would be an oracle);
//   - p.cong_dan_id = $3      the caller's statement of whose petition it is must MATCH the session;
//     a NULL cong_dan_id (phone not verified) matches nothing, and the inner join drops it as well;
//   - thu_hoi_luc IS NULL     a revoked session discloses nothing;
//   - het_han_luc > now()     a comparison, never a stored flag (rule 10, invariant 3), exactly as
//     truyVanPhienTheoToken does it.
//
// NO deleted_at: neither table has soft-delete columns (migration 0004 explains both).
const (
	contactPhoneTables = `phien_cong_dan p JOIN dinh_danh_cong_dan d ON d.id = p.cong_dan_id`
	contactPhoneFilter = `AND p.id = $2 AND p.cong_dan_id = $3 AND p.thu_hoi_luc IS NULL AND p.het_han_luc > now()`
)

// ContactPhoneForReveal returns the FULL verified phone number of the citizen behind the live
// session `sessionID` of the transaction's commune, provided that session belongs to `citizenID`.
// ErrNoContactPhone otherwise.
//
// INSIDE THE CALLER'S TRANSACTION, because the caller writes the disclosure's audit entry in that
// same transaction and returns the number only after it commits (rule 6, invariants 3 and 7 — the
// ordering of CanBoStore.EmailForReveal, ADR 0082). Its one caller is app.CitizenContactPhoneReveal.
//
// THE NUMBER NEVER ENTERS AN ERROR OR A LOG LINE (rule 3). Neither do the sid or the citizen id:
// the caller logs the commune.
func (s *PhienCongDanStore) ContactPhoneForReveal(ctx context.Context, tx *store.ScopedTx, sessionID, citizenID string) (string, error) {
	if sessionID == "" || citizenID == "" {
		// Fail closed before touching the database: "" matches no session, and an empty key must
		// never be the spelling of anything wider.
		return "", ErrNoContactPhone
	}
	if tx == nil {
		return "", ErrThieuGiaoDich
	}

	// @cross-tenant: dinh_danh_cong_dan không có tenant_id (ADR 0002 — một công dân cho toàn nền
	// tảng). Dòng phiên được phạm vi hoá theo xã ($1) và khoá theo (id phiên, id công dân); bảng định
	// danh chỉ được nối qua cong_dan_id của CHÍNH phiên ấy — nhiều nhất một dòng, không bao giờ danh
	// sách. Chủ dự án quyết định 08/10/2026 (ADR 0050 §Sửa đổi 08/10/2026, điểm 2).
	rows, err := tx.Query(ctx, "d.so_dien_thoai", contactPhoneTables, contactPhoneFilter, sessionID, citizenID)
	if err != nil {
		return "", fmt.Errorf("phiên công dân: đọc số đã xác thực: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("phiên công dân: đọc số đã xác thực: %w", err)
		}
		return "", ErrNoContactPhone
	}
	var phone string
	// NEVER LOG phone (rule 3, forbidden #1).
	if err := rows.Scan(&phone); err != nil {
		return "", fmt.Errorf("phiên công dân: đọc dòng số đã xác thực: %w", err)
	}
	if phone == "" {
		// The column is NOT NULL, but "" is not a number either: answering it as a disclosure would
		// write an audit entry for nothing and attach an empty contact to a petition.
		return "", ErrNoContactPhone
	}
	return phone, nil
}
