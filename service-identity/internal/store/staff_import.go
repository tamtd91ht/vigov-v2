package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The reads the STAFF IMPORT (ADR 0059 §1) plans against, and its lock. The WRITES are the single-
// create statements of can_bo_ghi.go and mat_khau.go — Chen, DatVaiTro, CapTaiKhoan — so an imported
// person is inserted, assigned and given an account by the very statements the three forms run, and
// every literal those statements carry (`co_tai_khoan = false` on insert, `phai_doi_mat_khau = true`
// on the account) holds for the import by construction.
//
// THE COMMUNE IS $1 EVERYWHERE, from tx.TenantID() / Scoped (rule 1). NOTHING HERE OPENS A TRANSACTION.

// StaffImportDirectoryCeiling bounds the live directory an import may grow a commune to. It is the
// bound the residential-unit head picker already REFUSES past (maxHeadCandidates): a directory over it
// would break that template and import for the whole commune.
const StaffImportDirectoryCeiling = maxHeadCandidates

// maxStaffImportEmails bounds the address read (soft-deleted rows included) — where the data has stopped
// being one commune's directory.
const maxStaffImportEmails = 10000

// ErrStaffImportSnapshotTooLarge — one of the snapshot reads hit its bound. Refused rather than planned
// against a truncated list, which would miss a taken address. 500.
var ErrStaffImportSnapshotTooLarge = errors.New("can_bo: ảnh chụp để nhập vượt trần")

// LockStaffImport serialises every staff import of ONE commune until the transaction ends. The unique
// key is the floor for an ADDRESS; nothing is a floor for a person WITHOUT one, so two imports of the
// same file (two Idempotency-Keys, two tabs) would both see the address-less rows absent and create
// them twice — with two permanent codes each (rule 7, invariant 3). Released by COMMIT or ROLLBACK.
func (s *CanBoStore) LockStaffImport(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('nguoi_dung:nhap:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("can_bo: khoá lượt nhập cán bộ: %w", err)
	}
	return nil
}

type queryFn func(ctx context.Context, cot, bang, tail string, args ...any) (*sql.Rows, error)

// staffImportOrgUnits — live units only: a NEW person is not filed under a removed unit. `q` is
// Scoped.Query or ScopedTx.Query, which add `WHERE tenant_id = $1` and bind the commune.
func staffImportOrgUnits(ctx context.Context, q queryFn) ([]domain.StaffImportOrgUnitChoice, error) {
	// tenant_id = $1 is added by q (core/store Scoped/ScopedTx.Query).
	rows, err := q(ctx, `id, ma, ten`, `bo_phan`, `AND deleted_at IS NULL ORDER BY thu_tu, ten, ma LIMIT $2`, TranDanhMucBoPhan+1)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc bộ phận để nhập: %w", err)
	}
	defer rows.Close()
	var out []domain.StaffImportOrgUnitChoice
	for rows.Next() {
		var u domain.StaffImportOrgUnitChoice
		if err := rows.Scan(&u.ID, &u.Code, &u.Name); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng bộ phận: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt bộ phận: %w", err)
	}
	if len(out) > TranDanhMucBoPhan {
		return nil, ErrStaffImportSnapshotTooLarge
	}
	return out, nil
}

// staffImportRoles — live roles only: the foreign key would accept a soft-deleted one, and a person on
// it would hold nothing while every screen shows a role (the argument of QuyenCuaVaiTro).
func staffImportRoles(ctx context.Context, q queryFn) ([]domain.StaffImportRoleChoice, error) {
	// tenant_id = $1 is added by q (core/store Scoped/ScopedTx.Query).
	rows, err := q(ctx, `id, ma, ten`, `vai_tro`, `AND deleted_at IS NULL ORDER BY thu_tu, ten, ma LIMIT $2`, TranDanhMucVaiTro+1)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc vai trò để nhập: %w", err)
	}
	defer rows.Close()
	var out []domain.StaffImportRoleChoice
	for rows.Next() {
		var r domain.StaffImportRoleChoice
		if err := rows.Scan(&r.ID, &r.Code, &r.Name); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng vai trò: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt vai trò: %w", err)
	}
	if len(out) > TranDanhMucVaiTro {
		return nil, ErrStaffImportSnapshotTooLarge
	}
	return out, nil
}

// StaffImportSnapshot reads everything the plan checks against, inside the import's transaction.
//
// THE ADDRESSES INCLUDE SOFT-DELETED ROWS: `UNIQUE (tenant_id, email)` is not partial. They are read to
// be COMPARED and are never returned to a client or logged (rule 3).
func (s *CanBoStore) StaffImportSnapshot(ctx context.Context, tx *store.ScopedTx) (domain.StaffImportSnapshot, error) {
	var snap domain.StaffImportSnapshot

	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `email`, `nguoi_dung`, `AND email IS NOT NULL LIMIT $2`, maxStaffImportEmails+1)
	if err != nil {
		return snap, fmt.Errorf("can_bo: đọc thư điện tử để nhập: %w", err)
	}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			rows.Close()
			return snap, fmt.Errorf("can_bo: đọc dòng thư điện tử: %w", err)
		}
		snap.TakenEmails = append(snap.TakenEmails, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snap, fmt.Errorf("can_bo: duyệt thư điện tử: %w", err)
	}
	if len(snap.TakenEmails) > maxStaffImportEmails {
		return snap, ErrStaffImportSnapshotTooLarge
	}

	// The two counts the ceilings are checked on. The second is the staff picker's predicate
	// (locChonNguoi), shared by constant so "an account the picker counts" means one thing.
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	cnt, err := tx.Query(ctx, `count(*) FILTER (WHERE deleted_at IS NULL), count(*) FILTER (WHERE TRUE `+locChonNguoi+`)`, `nguoi_dung`, ``)
	if err != nil {
		return snap, fmt.Errorf("can_bo: đếm danh bạ để nhập: %w", err)
	}
	if cnt.Next() {
		if err := cnt.Scan(&snap.LiveStaff, &snap.LiveAccounts); err != nil {
			cnt.Close()
			return snap, fmt.Errorf("can_bo: đọc số đếm danh bạ: %w", err)
		}
	}
	err = cnt.Err()
	cnt.Close()
	if err != nil {
		return snap, fmt.Errorf("can_bo: duyệt số đếm danh bạ: %w", err)
	}

	if snap.OrgUnits, err = staffImportOrgUnits(ctx, tx.Query); err != nil {
		return snap, err
	}
	if snap.Roles, err = staffImportRoles(ctx, tx.Query); err != nil {
		return snap, err
	}
	return snap, nil
}

// StaffTemplateChoices reads the two dropdowns of the import template — outside any transaction, with
// the same predicates as StaffImportSnapshot, so the file never offers a value the import refuses.
func (s *CanBoStore) StaffTemplateChoices(ctx context.Context) ([]domain.StaffImportOrgUnitChoice, []domain.StaffImportRoleChoice, error) {
	// store.DB.For(ctx) binds tenant_id from the context.
	sc := s.db.For(ctx)
	units, err := staffImportOrgUnits(ctx, sc.Query)
	if err != nil {
		return nil, nil, err
	}
	roles, err := staffImportRoles(ctx, sc.Query)
	if err != nil {
		return nil, nil, err
	}
	return units, roles, nil
}
