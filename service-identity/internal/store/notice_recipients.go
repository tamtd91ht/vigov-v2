package store

import (
	"context"
	"fmt"
)

// The three reads behind ResolveOrgUnitPermissionHolders and ResolveLeadershipStaff
// (proto/vigov/identity/v1/identity.proto): WHO IS TOLD about late work. Staff business codes only —
// no name, no contact detail, no internal id — so nothing here is personal data (rule 3).
//
// NONE OF THEM DECIDES ACCESS. They answer "who receives a notice"; a caller that used one as a guard
// would have a second authorisation path outside (tenant_id, role, permission) — rule 5, forbidden #3.

// holdersQuery answers, for a set of units, which staff sitting DIRECTLY in each hold one permission
// key there.
//
// THE PREDICATE IS THE CHECKER'S, BY CONSTANT, NOT A SECOND SPELLING. noiGiuQuyen is the person → role
// → grant join bound to the commune at every table, and dieuKienGiuQuyen is "can exercise a permission
// today" (not deleted, has an account, not locked, role not deleted) — which is exactly
// ResolveAssignableStaff's predicate (locChonNguoi) plus the live role, as the contract requires. A
// copy would drift: looser, and a locked person is "told"; stricter, and a real holder is missing with
// nothing reporting it.
//
// THE UNIT MUST BE LIVE (liveOrgUnit, the org chart's own predicate), checked with EXISTS on the same
// commune rather than a join on id alone — ids of two communes may collide (rule 1).
//
// DISTINCT because a role can reach one key through one grant row only today, but a person counted
// twice would receive a notice twice under two recipient entries.
const holdersQuery = `
SELECT DISTINCT nd.bo_phan_id, nd.ma` + noiGiuQuyen + dieuKienGiuQuyen + `
  AND nd.bo_phan_id = ANY($2)
  AND vq.quyen_ma = $3
  AND EXISTS (SELECT 1 FROM bo_phan bp
              WHERE bp.tenant_id = nd.tenant_id AND bp.id = nd.bo_phan_id AND bp.` + liveOrgUnit + `)
ORDER BY nd.bo_phan_id, nd.ma`

// OrgUnitPermissionHolders returns, per requested unit that has at least one holder, the staff codes
// holding `key` there. An empty unit list reads nothing and answers nothing — never "every unit".
//
// BOUNDED BY ITS INPUT: at most 50 units (enforced at the contract, internal/grpc), each answering its
// own direct members only.
func (s *CanBoStore) OrgUnitPermissionHolders(ctx context.Context, unitIDs []string, key string) (map[string][]string, error) {
	if len(unitIDs) == 0 || key == "" {
		return map[string][]string{}, nil
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, holdersQuery, unitIDs, key)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc người giữ quyền theo bộ phận: %w", err)
	}
	defer rows.Close()

	out := map[string][]string{}
	for rows.Next() {
		var unit, code string
		if err := rows.Scan(&unit, &code); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng người giữ quyền: %w", err)
		}
		out[unit] = append(out[unit], code)
	}
	// Checked: a connection lost mid-result ends the loop like a complete read, and a truncated answer
	// would record late work as "without recipient" when it had one.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt người giữ quyền: %w", err)
	}
	return out, nil
}

// leadershipQuery reads the commune's leadership: live accounts (ResolveAssignableStaff's predicate)
// whose live role carries `la_lanh_dao`. The role join repeats the commune (rule 1).
//
// `la_lanh_dao` DECIDES WHO IS TOLD HERE, NOT WHO MAY ACT — the contract (ResolveLeadershipStaff)
// states why this is the one reading of the flag besides the default screen, and domain.VaiTro
// forbids it from ever becoming an authority axis.
const leadershipQuery = `
SELECT nd.ma
FROM nguoi_dung nd
JOIN vai_tro vt ON vt.tenant_id = nd.tenant_id AND vt.id = nd.vai_tro_id
WHERE nd.tenant_id = $1
  AND nd.deleted_at IS NULL
  AND nd.co_tai_khoan
  AND nd.dang_hoat_dong
  AND vt.deleted_at IS NULL
  AND vt.la_lanh_dao
ORDER BY nd.ma
LIMIT $2`

// LeadershipCodes returns at most `limit` leadership codes. The caller passes its ceiling PLUS ONE so
// "too many" is detectable — the contract refuses above 50 rather than truncating.
func (s *CanBoStore) LeadershipCodes(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, leadershipQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc lãnh đạo xã: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng lãnh đạo: %w", err)
		}
		out = append(out, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt lãnh đạo: %w", err)
	}
	return out, nil
}

// permissionKeyExistsQuery asks the software's permission catalogue whether one key is a row.
//
// `quyen` HAS NO tenant_id AND THAT IS NOT A HOLE IN RULE 1 — the catalogue is identical in every
// commune (truyVanDanhMucQuyen states it in full). The `$1::text <> ”` consumes the commune the scoped
// path binds, exactly as that query does, so this stays on the one sanctioned path.
const permissionKeyExistsQuery = `
SELECT count(*)
FROM quyen q
WHERE $1::text <> ''
  AND q.ma = $2`

// PermissionKeyExists reports whether `key` is a row of `quyen` (rule 5, invariant 3c). A typo'd key
// must be refused by the caller, never answered as "nobody holds it".
func (s *QuyenStore) PermissionKeyExists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, permissionKeyExistsQuery, key)
	if err != nil {
		return false, fmt.Errorf("quyen: kiểm khoá quyền: %w", err)
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return false, fmt.Errorf("quyen: đọc kết quả kiểm khoá: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("quyen: duyệt kết quả kiểm khoá: %w", err)
	}
	return n > 0, nil
}
