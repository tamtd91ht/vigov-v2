package store

// The statements behind the operator OWNER's three commune-scoped RPCs (operator.proto
// ListCommuneStaffAccounts / CreateCommuneAdminAccount / ResetStaffPassword; ADR 0048 §"Sửa đổi
// 13/10/2026"). Every one runs on a *store.ScopedTx of the TARGET commune — the "x-tenant-id"
// metadata, never a request field — and binds tx.TenantID() as $1.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// selectSignInAccounts — a JOINED read, so every table is constrained to $1 in its own clause (the
// core/store.QueryJoin contract): joining vai_tro on id alone would match another commune's role
// wherever ids collide.
const selectSignInAccounts = `SELECT nd.id, nd.ma, coalesce(nd.email, ''), nd.ho_ten,
	coalesce(vt.ma, ''), coalesce(vt.ten, ''), NOT nd.dang_hoat_dong, nd.phai_doi_mat_khau,
	nd.sign_in_locked_until
	FROM nguoi_dung nd
	LEFT JOIN vai_tro vt ON vt.tenant_id = $1 AND vt.id = nd.vai_tro_id AND vt.deleted_at IS NULL
	WHERE nd.tenant_id = $1 AND nd.deleted_at IS NULL AND nd.co_tai_khoan`

func scanSignInAccount(scan func(...any) error) (domain.StaffSignInAccount, error) {
	var a domain.StaffSignInAccount
	var locked sql.NullTime
	if err := scan(&a.ID, &a.Code, &a.Email, &a.FullName, &a.RoleCode, &a.RoleName, &a.Disabled,
		&a.MustChangePassword, &locked); err != nil {
		return domain.StaffSignInAccount{}, err
	}
	if locked.Valid {
		t := locked.Time
		a.SignInLockedUntil = &t
	}
	return a, nil
}

// ListSignInAccounts lists the commune's sign-in accounts — locked ones included (a reset can be
// aimed at them), directory-only and soft-deleted rows excluded (rule 7, invariant 2) — ordered by
// email, then id so two rows can never swap places between reads.
func (s *CanBoStore) ListSignInAccounts(ctx context.Context, tx *store.ScopedTx) ([]domain.StaffSignInAccount, error) {
	rows, err := tx.Underlying().QueryContext(ctx, selectSignInAccounts+` ORDER BY nd.email, nd.id`,
		string(tx.TenantID()))
	if err != nil {
		return nil, fmt.Errorf("can_bo: list sign-in accounts: %w", err)
	}
	defer rows.Close()
	var out []domain.StaffSignInAccount
	for rows.Next() {
		a, err := scanSignInAccount(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("can_bo: list sign-in accounts: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: list sign-in accounts: %w", err)
	}
	return out, nil
}

// SignInAccountByID reads one sign-in account of the commune, for the response of a create or a
// reset. ErrCanBoKhongTonTai when the row is not a live sign-in account HERE.
func (s *CanBoStore) SignInAccountByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.StaffSignInAccount, error) {
	if id == "" {
		return domain.StaffSignInAccount{}, ErrCanBoKhongTonTai
	}
	a, err := scanSignInAccount(tx.Underlying().QueryRowContext(ctx, selectSignInAccounts+` AND nd.id = $2`,
		string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StaffSignInAccount{}, ErrCanBoKhongTonTai
	}
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("can_bo: read sign-in account: %w", err)
	}
	return a, nil
}

// NewCommuneAdmin is one administrator account the owner creates. Every field is decided by the use
// case (app.CommuneAccountsByOwner) — the code minted, the hash computed, the role resolved.
type NewCommuneAdmin struct {
	ID           string
	Code         string // `CB-…`, minted by domain.SinhMaCanBo
	FullName     string
	Email        string // already normalised by domain.ChuanHoaEmail
	PasswordHash string // argon2id of the owner's temporary password — never the value
	RoleID       string
	// AssignedBy is `created_by` of the role assignment (migration 0031) — the operator's code, the
	// same value CapMoiQuyen records as `cap_boi`. Required.
	AssignedBy string
}

// InsertCommuneAdmin inserts the account with the SAME column shape as the seeded default
// administrator (ChenQuanTriMacDinh): an account that can sign in, phai_doi_mat_khau = true, the
// administrator role, no phone. A PLAIN INSERT, not ON CONFLICT: a taken email must reach the owner
// as ErrEmailDaDung (ALREADY_EXISTS) under `UNIQUE (tenant_id, email)` — the directory's own rule —
// and a taken code as ErrMaCanBoDaDung, which the caller retries with a new code.
func (s *CanBoStore) InsertCommuneAdmin(ctx context.Context, tx *store.ScopedTx, in NewCommuneAdmin) error {
	if in.ID == "" || in.Code == "" || in.FullName == "" || in.Email == "" || in.PasswordHash == "" ||
		in.RoleID == "" || in.AssignedBy == "" {
		return errors.New("can_bo: insert commune admin: required field missing")
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO nguoi_dung
		   (tenant_id, id, ma, ho_ten, email, vai_tro_id,
		    co_tai_khoan, dang_hoat_dong, phai_doi_mat_khau, mat_khau_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, true, true, true, $7)`,
		string(tx.TenantID()), in.ID, in.Code, in.FullName, in.Email, in.RoleID, in.PasswordHash)
	if err != nil {
		return dichLoiGhiCanBo("tạo quản trị xã bởi vận hành", err)
	}
	// The role is HELD through staff_role_assignment (migration 0031); without this row the new
	// administrator would sign in holding no permission at all.
	return insertPrimaryRoleAssignment(ctx, tx, in.ID, in.RoleID, in.AssignedBy)
}
