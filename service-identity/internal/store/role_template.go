package store

// The statements behind POST /api/v1/roles/defaults — seeding the template roles of
// domain.RoleTemplates into ONE commune (user decision 2026-09-28). The use case is
// app/role_template.go; it opens the ONE transaction every method here writes in, together with the
// audit entries (rule 6, invariant 3).
//
// THE SAME PROPERTIES AS vai_tro_quyen_ghi.go:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context. No
//     method takes a commune (rule 1, invariants 4 and 5).
//  2. NOTHING HERE OPENS A TRANSACTION.
//  3. THE LOCK ORDER IS THE ONE vai_tro_quyen_ghi.go DOCUMENTS — holders of `admin.user`, then
//     holders of `admin.role` — taken by LockGrantorSets before anything is read. The actor holds
//     `admin.role` (the route requires it), so their row, their role and its grant rows are held from
//     that point: a concurrent PUT /api/v1/roles/{id}/permissions withdrawing one of their keys waits
//     for this run instead of racing the "caller holds every key" check. Taking the two sets in the
//     SAME order as that route is what keeps the two from deadlocking against each other.
//  4. NO STATEMENT HERE UPDATES OR DELETES ANYTHING. An existing role — live or soft-deleted — is
//     reported and left alone, INCLUDING ITS GRANTS.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ErrRoleCodeTaken is `UNIQUE (tenant_id, ma)` refusing a template role this commune already has.
//
// THE SEEDING PATH READS THE CODES INSIDE ITS TRANSACTION FIRST, so this fires only when a
// concurrent run (or a concurrent first-admin seed) committed the same code in between: a lost race,
// not a bug. The loser's whole transaction rolls back — no half-seeded commune — and retrying
// reports the role as already present.
var ErrRoleCodeTaken = errors.New("vai_tro_mau: xã vừa có vai trò mang mã này")

// RoleTemplateStore writes the template roles into `vai_tro` and `vai_tro_quyen`.
type RoleTemplateStore struct{ db *store.DB }

func NewRoleTemplateStore(db *store.DB) *RoleTemplateStore { return &RoleTemplateStore{db: db} }

// LockGrantorSets takes lock steps 1 and 2 of vai_tro_quyen_ghi.go, in that order, and discards the
// answer: this use case needs the LOCKS, not the sets. See property 3 above.
func (s *RoleTemplateStore) LockGrantorSets(ctx context.Context, tx *store.ScopedTx) error {
	if _, err := docNguoiGiuDeGhi(ctx, tx, truyVanQuanTriDeGhi); err != nil {
		return fmt.Errorf("vai_tro_mau: khoá người giữ admin.user: %w", err)
	}
	if _, err := docNguoiGiuDeGhi(ctx, tx, truyVanPhanQuyenDeGhi); err != nil {
		return fmt.Errorf("vai_tro_mau: khoá người giữ admin.role: %w", err)
	}
	return nil
}

// HeldPermissions lists the keys a person can exercise right now — THE SAME TEXT as checker.go
// (truyVanQuyen1Nguoi), so "holds every key the templates grant" and the route guard cannot
// disagree about what holding means.
func (s *RoleTemplateStore) HeldPermissions(ctx context.Context, tx *store.ScopedTx, staffID string) ([]string, error) {
	if staffID == "" {
		return nil, nil
	}
	ds, err := docCotChuoi(ctx, tx, truyVanQuyen1Nguoi, staffID)
	if err != nil {
		return nil, fmt.Errorf("vai_tro_mau: đọc quyền đang giữ: %w", err)
	}
	return ds, nil
}

// RoleCodeStates reports, for each of `codes` this commune already has a role for, whether that
// role is SOFT-DELETED (true) or live (false). A code absent from the map has no row at all.
//
// THE ONE READ HERE THAT DELIBERATELY DOES NOT EXCLUDE SOFT-DELETED ROWS, for the reason
// CoEmailMoiTrangThai gives: rule 7, invariant 2 is about SHOWING and COUNTING records, and this is
// the opposite question. `UNIQUE (tenant_id, ma)` is not partial (migration 0001:120), so a deleted
// role still owns its code: inserting beside it would fail, and reviving it would reverse somebody's
// decision without a trail of who reversed it. The use case reports it and moves on — the stance
// ErrVaiTroQuanTriDaXoa takes for the administrator role.
func (s *RoleTemplateStore) RoleCodeStates(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]bool, error) {
	out := make(map[string]bool, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	const stmt = `SELECT ma, deleted_at IS NOT NULL FROM vai_tro
	              WHERE tenant_id = $1 AND ma = ANY($2)`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), codes)
	if err != nil {
		return nil, fmt.Errorf("vai_tro_mau: đọc mã vai trò đã có: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var deleted bool
		if err := rows.Scan(&code, &deleted); err != nil {
			return nil, fmt.Errorf("vai_tro_mau: đọc dòng vai trò: %w", err)
		}
		out[code] = deleted
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vai_tro_mau: duyệt vai trò đã có: %w", err)
	}
	return out, nil
}

// InsertRole writes one template role with the id the caller minted.
//
// NO `ON CONFLICT`, for the reason sla_ghi.go Chen gives: DO NOTHING would make "already there"
// indistinguishable from "written" at the call site, and the use case audits only what it wrote.
// A collision arrives as ErrRoleCodeTaken.
func (s *RoleTemplateStore) InsertRole(ctx context.Context, tx *store.ScopedTx, id string, t domain.RoleTemplate) error {
	const stmt = `INSERT INTO vai_tro (tenant_id, id, ten, ma, la_lanh_dao, thu_tu)
	              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, t.Name, t.Code, t.IsLeader, t.Order)
	if err != nil {
		return translateRoleInsertError(err)
	}
	return nil
}

// GrantPermissions inserts the role's grant cells. `cap_boi` is the actor's STAFF CODE (rule 6,
// invariant 8) — never the internal id; the use case refuses an empty one before reaching here.
//
// Only ever called for a role THIS transaction just inserted, so no existing grant can collide and
// no existing role's grants can be touched.
func (s *RoleTemplateStore) GrantPermissions(ctx context.Context, tx *store.ScopedTx, roleID string, keys []string, grantedBy string) error {
	if len(keys) == 0 {
		return nil
	}
	const stmt = `INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_boi)
	              SELECT $1, $2, k, $4 FROM unnest($3::text[]) AS k`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), roleID, keys, grantedBy)
	if err != nil {
		return fmt.Errorf("vai_tro_mau: cấp quyền cho vai trò mẫu: %w", err)
	}
	return dungSoDong(kq, len(keys), "cấp quyền cho vai trò mẫu")
}

// translateRoleInsertError turns the `(tenant_id, ma)` unique violation into ErrRoleCodeTaken.
//
// A SUBSTRING MATCH ON BOTH HALVES, following dichLoiGhiSLA: `vai_tro` is PARTITION BY HASH, so
// PostgreSQL names the PARTITION's constraint (`vai_tro_p07_tenant_id_ma_key`). `tenant_id_ma_key`
// alone would also match `nguoi_dung`'s staff-code key, hence the table prefix. Anything else is
// returned wrapped and unchanged — a connection failure must never read as "this role exists".
func translateRoleInsertError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "vai_tro_") && strings.Contains(msg, "tenant_id_ma_key") {
		return ErrRoleCodeTaken
	}
	return fmt.Errorf("vai_tro_mau: chèn vai trò mẫu: %w", err)
}
