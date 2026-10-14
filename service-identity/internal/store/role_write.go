package store

// The statements behind the commune's OWN role writes (owner decision 15/10/2026):
// POST /api/v1/roles, PUT /api/v1/roles/{id}, POST /api/v1/roles/{id}/deletion. The use case is
// app/role_write.go; it opens the ONE transaction every method here writes in, together with the
// audit entry (rule 6, invariant 3).
//
// THE SAME PROPERTIES AS vai_tro_quyen_ghi.go:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context. No
//     method takes a commune (rule 1, invariants 4 and 5).
//  2. NOTHING HERE OPENS A TRANSACTION.
//  3. `ma` APPEARS IN NO UPDATE: a role's code is issued once by InsertRole and never rewritten or
//     reissued (rule 7, invariant 3). Grants (`vai_tro_quyen`) are not touched here at all — a new role
//     holds nothing until the matrix ticks it, and a deleted role's rows stay as they were (rule 7).
//
// LOCK ORDER: LockCatalogue (the commune's advisory lock) first, then LockRole (the row). Every write
// of this file takes them in that order, so two of them cannot deadlock against each other.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// RoleWriteStore writes `vai_tro` for the commune's own role routes.
type RoleWriteStore struct{ db *store.DB }

func NewRoleWriteStore(db *store.DB) *RoleWriteStore { return &RoleWriteStore{db: db} }

// LockCatalogue serialises every create, rename and deletion of ONE commune's roles until the
// transaction ends.
//
// WHY: "no other live role has this name" is decided by reading and then writing, and NOTHING in the
// schema is a floor for the name (a unique index on a case-folded name would need a collation this
// repository cannot verify) — the reason ThonToDanPhoStore.LockRegister gives. Without it two
// administrators creating "Bí thư" at the same instant both see it absent. Per commune, released by
// COMMIT or ROLLBACK.
func (s *RoleWriteStore) LockCatalogue(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('vai_tro:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("vai_tro: khoá danh mục vai trò: %w", err)
	}
	return nil
}

// LiveRoles reads the commune's LIVE roles, at most TranDanhMucVaiTro+1 of them — one past the
// ceiling, so the caller can tell "exactly full" from "over".
func (s *RoleWriteStore) LiveRoles(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingRole, error) {
	const stmt = `SELECT id, ma, ten, la_lanh_dao, thu_tu FROM vai_tro
	              WHERE tenant_id = $1 AND deleted_at IS NULL
	              ORDER BY thu_tu, ten, id LIMIT $2`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), TranDanhMucVaiTro+1)
	if err != nil {
		return nil, fmt.Errorf("vai_tro: đọc vai trò đang dùng: %w", err)
	}
	defer rows.Close()
	var out []domain.ExistingRole
	for rows.Next() {
		var r domain.ExistingRole
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.IsLeader, &r.Order); err != nil {
			return nil, fmt.Errorf("vai_tro: đọc dòng vai trò: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vai_tro: duyệt vai trò đang dùng: %w", err)
	}
	return out, nil
}

// maxRoleCodesWithBase bounds CodesWithBase — domain.PickRoleCode tries a hundred suffixes.
const maxRoleCodesWithBase = 1000

// CodesWithBase lists this commune's role codes equal to `base` or of the form `base-…`, SOFT-DELETED
// ROWS INCLUDED — the unique key counts them, so "free" must count them too (BoPhanStore.MaCungGoc).
//
// `base` IS A VALIDATED SLUG ([a-z0-9-] only, domain.RoleCodeBase), so it carries neither `%` nor `_`
// and needs no escaping inside LIKE.
func (s *RoleWriteStore) CodesWithBase(ctx context.Context, tx *store.ScopedTx, base string) ([]string, error) {
	const stmt = `SELECT ma FROM vai_tro WHERE tenant_id = $1 AND (ma = $2 OR ma LIKE $3) LIMIT $4`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), base, base+"-%", maxRoleCodesWithBase)
	if err != nil {
		return nil, fmt.Errorf("vai_tro: đọc mã cùng gốc: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("vai_tro: đọc mã: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vai_tro: duyệt mã cùng gốc: %w", err)
	}
	return out, nil
}

// InsertRole writes one new role with the id and code the caller chose. No grant row: a new role
// holds nothing until somebody ticks it on the matrix (PUT /api/v1/roles/{id}/permissions, #14).
//
// NO `ON CONFLICT`, for the reason RoleTemplateStore.InsertRole gives; a collision arrives as
// ErrRoleCodeTaken.
func (s *RoleWriteStore) InsertRole(ctx context.Context, tx *store.ScopedTx, r domain.ExistingRole) error {
	const stmt = `INSERT INTO vai_tro (tenant_id, id, ten, ma, la_lanh_dao, thu_tu)
	              VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.Name, r.Code, r.IsLeader, r.Order); err != nil {
		return translateRoleInsertError(err)
	}
	return nil
}

// LockRole reads one LIVE role of this commune and holds its row until the transaction ends.
//
// ErrVaiTroKhongTonTaiDeGhi for an invented id, a soft-deleted role and another commune's role — one
// answer, so none can be told apart by trying (rule 4, forbidden #2).
//
// `FOR UPDATE` IS WHAT MAKES THE DELETION'S "nobody holds it" COUNT HOLD: every path that gives a
// person a role reads it through CanBoStore.QuyenCuaVaiTro, which takes `FOR SHARE` on this same row.
// An assignment that read the role first is committed (and counted) before this lock is granted; one
// that reads after waits for the deletion's commit and then finds the role deleted.
func (s *RoleWriteStore) LockRole(ctx context.Context, tx *store.ScopedTx, id string) (domain.ExistingRole, error) {
	if id == "" {
		return domain.ExistingRole{}, ErrVaiTroKhongTonTaiDeGhi
	}
	const stmt = `SELECT id, ma, ten, la_lanh_dao, thu_tu FROM vai_tro
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`
	var r domain.ExistingRole
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).
		Scan(&r.ID, &r.Code, &r.Name, &r.IsLeader, &r.Order)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ExistingRole{}, ErrVaiTroKhongTonTaiDeGhi
	}
	if err != nil {
		return domain.ExistingRole{}, fmt.Errorf("vai_tro: khoá dòng vai trò: %w", err)
	}
	return r, nil
}

// UpdateRole writes the two editable columns of one live role: name and leader flag.
//
// `ma`, `id`, `thu_tu` AND EVERY SOFT-DELETE COLUMN ARE ABSENT FROM THE SET CLAUSE, so this statement
// cannot reissue a code, resurrect a deleted role or move a row to another commune whatever it is
// passed.
func (s *RoleWriteStore) UpdateRole(ctx context.Context, tx *store.ScopedTx, id, name string, isLeader bool) error {
	if id == "" {
		return ErrVaiTroKhongTonTaiDeGhi
	}
	const stmt = `UPDATE vai_tro SET ten = $3, la_lanh_dao = $4, cap_nhat_luc = now()
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, name, isLeader)
	if err != nil {
		return fmt.Errorf("vai_tro: cập nhật vai trò: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("vai_tro: đếm dòng đã cập nhật: %w", err)
	}
	if n == 0 {
		// Cannot normally happen — the caller holds the row FOR UPDATE. Checked because a write
		// reporting success having changed nothing is the one failure nobody notices.
		return ErrVaiTroKhongTonTaiDeGhi
	}
	return nil
}

// CountHolders counts the LIVE staff (`nguoi_dung.deleted_at IS NULL`) who still hold the role —
// through a live staff_role_assignment row, or through `nguoi_dung.vai_tro_id` (the primary-role
// column every write keeps in sync, counted too so a row the sync missed still blocks). Each person
// once. Locked accounts and directory-only entries COUNT: they are still the commune's staff, and a
// role deleted under them would leave them holding a role no screen shows.
//
// A soft-deleted person does not count: nobody can sign in as them, and the checker drops them.
func (s *RoleWriteStore) CountHolders(ctx context.Context, tx *store.ScopedTx, roleID string) (int, error) {
	const stmt = `SELECT count(*) FROM nguoi_dung nd
	              WHERE nd.tenant_id = $1 AND nd.deleted_at IS NULL
	                AND (nd.vai_tro_id = $2
	                     OR EXISTS (SELECT 1 FROM staff_role_assignment sra
	                                WHERE sra.tenant_id = nd.tenant_id AND sra.staff_id = nd.id
	                                  AND sra.role_id = $2 AND sra.deleted_at IS NULL))`
	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), roleID).Scan(&n); err != nil {
		return 0, fmt.Errorf("vai_tro: đếm cán bộ đang giữ vai trò: %w", err)
	}
	return n, nil
}

// SoftDeleteRole marks one live role deleted: `deleted_at`, `deleted_by` (the remover's STAFF CODE,
// rule 6 invariant 8) and `delete_reason` (rule 7, invariant 1). Returns the moment written.
//
// `ma` STAYS TAKEN FOR EVER (CodesWithBase counts deleted rows). The role's `vai_tro_quyen` rows are
// left as they are: every read of grants joins `vai_tro.deleted_at IS NULL` (checker.go, quyen.go), so
// they grant nothing and show nowhere, and they record what the role could do when it was retired.
//
// `AND deleted_at IS NULL` is what makes a second deletion unable to overwrite who removed the role or
// why; zero rows is ErrVaiTroKhongTonTaiDeGhi.
func (s *RoleWriteStore) SoftDeleteRole(ctx context.Context, tx *store.ScopedTx, id, deletedBy, reason string) (time.Time, error) {
	if id == "" {
		return time.Time{}, ErrVaiTroKhongTonTaiDeGhi
	}
	if deletedBy == "" {
		// No fallback (rule 6, invariant 8): a removal nobody can be named for is refused.
		return time.Time{}, errors.New("vai_tro: xoá mềm không có mã người xoá")
	}
	const stmt = `UPDATE vai_tro SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	              RETURNING deleted_at`
	var at time.Time
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id, deletedBy, reason).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrVaiTroKhongTonTaiDeGhi
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("vai_tro: xoá mềm vai trò: %w", err)
	}
	return at, nil
}
