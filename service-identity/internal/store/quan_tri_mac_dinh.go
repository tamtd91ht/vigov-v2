package store

// The four statements that create a commune's DEFAULT ADMINISTRATOR at its first `admin` sign-in
// (owner's decision, 2026-09-26; ledger item `xa-moi-khong-co-vai-tro-va-quyen`).
//
// WHY IT EXISTS: no production path created the first role, the first grant or the first
// administrator of a commune, so every RequirePermission route of a new commune answered 403 to
// every account, and nothing inside the commune could undo that. The caller is app.DangNhap; it
// opens the ONE transaction all four share with the audit entry (rule 6, invariant 3).
//
// EVERY INSERT IS `ON CONFLICT … DO NOTHING` FOLLOWED BY A READ, never read-then-insert: two
// first sign-ins of the same commune arriving together must end with exactly one role and one
// account, and only the unique keys of migration 0001 can decide that race — a check in Go has a
// gap between the read and the write.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// The fixed identity of the default administrator. Constants, not parameters: nothing a client
// sends can choose which role is created or which email the account carries.
const (
	// EmailQuanTriMacDinh is the literal login the owner chose. Sign-in matches `email = $2`
	// exactly (TheoEmail), so this is the only spelling that ever seeds.
	EmailQuanTriMacDinh = "admin"

	// MaVaiTroQuanTriMacDinh / TenVaiTroQuanTriMacDinh — the role holding every key.
	MaVaiTroQuanTriMacDinh  = "quan-tri-he-thong"
	TenVaiTroQuanTriMacDinh = "Quản trị hệ thống"

	// HoTenQuanTriMacDinh is the display name. Deliberately not a person's name: nobody is this
	// account until somebody changes its password and the commune records who holds it.
	HoTenQuanTriMacDinh = "Quản trị hệ thống"
)

// ErrVaiTroQuanTriDaXoa — the commune has a `quan-tri-he-thong` role that was soft-deleted.
//
// REFUSED, NOT REUSED AND NOT RECREATED. `UNIQUE (tenant_id, ma)` is not partial, so a second
// role with that code cannot be created; reviving the deleted one would undo somebody's decision
// without a trail of who reversed it (rule 7). Whoever deleted it has to be asked.
var ErrVaiTroQuanTriDaXoa = errors.New("quan_tri_mac_dinh: vai trò quản trị hệ thống đã bị xoá mềm")

// CoEmailMoiTrangThai reports whether ANY row of this commune carries the email — soft-deleted,
// locked, directory-only included.
//
// THE ONE READ IN THIS STORE THAT DELIBERATELY DOES NOT EXCLUDE SOFT-DELETED ROWS (rule 7,
// invariant 2 is about showing and counting records; this is the opposite question). A commune
// whose `admin` was deleted or locked by somebody made that choice; seeding a fresh `admin` next
// to it would silently reverse it. Any row at all means "never seed here".
func (s *CanBoStore) CoEmailMoiTrangThai(ctx context.Context, tx *store.ScopedTx, email string) (bool, error) {
	var mot int
	err := tx.Underlying().QueryRowContext(ctx,
		`SELECT 1 FROM nguoi_dung WHERE tenant_id = $1 AND email = $2 LIMIT 1`,
		string(tx.TenantID()), email).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("quan_tri_mac_dinh: tra email mọi trạng thái: %w", err)
	}
	return true, nil
}

// VaiTroQuanTriMacDinh returns the id of this commune's `quan-tri-he-thong` role, creating it
// with idMoi when the commune has none. taoMoi says which of the two happened, for the audit
// delta.
func (s *CanBoStore) VaiTroQuanTriMacDinh(ctx context.Context, tx *store.ScopedTx, idMoi string) (id string, taoMoi bool, err error) {
	kq, err := tx.Exec(ctx,
		`INSERT INTO vai_tro (tenant_id, id, ten, ma)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, ma) DO NOTHING`,
		string(tx.TenantID()), idMoi, TenVaiTroQuanTriMacDinh, MaVaiTroQuanTriMacDinh)
	if err != nil {
		return "", false, fmt.Errorf("quan_tri_mac_dinh: tạo vai trò: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return "", false, fmt.Errorf("quan_tri_mac_dinh: đếm dòng vai trò: %w", err)
	}

	var daXoa bool
	err = tx.Underlying().QueryRowContext(ctx,
		`SELECT id, deleted_at IS NOT NULL FROM vai_tro WHERE tenant_id = $1 AND ma = $2`,
		string(tx.TenantID()), MaVaiTroQuanTriMacDinh).Scan(&id, &daXoa)
	if err != nil {
		return "", false, fmt.Errorf("quan_tri_mac_dinh: đọc lại vai trò: %w", err)
	}
	if daXoa {
		return "", false, ErrVaiTroQuanTriDaXoa
	}
	return id, n == 1, nil
}

// CapMoiQuyen grants the role EVERY key in the `quyen` catalogue and returns how many cells were
// added. Keys the role already holds are left as they are.
//
// FROM THE TABLE, NEVER FROM A LIST IN GO — rule 5, invariant 3c: a key the catalogue lacks is a
// key nobody can hold, and a Go list is a second copy of the catalogue that drifts the day a
// migration adds a key. `quyen` is the platform-wide catalogue (migration 0001 §quyen, `@scope:
// platform`); the commune is $1 on the row written.
//
// `cap_boi` is audit.SystemActor ("system"): no person granted these — the system did, on the
// owner's standing decision, and the audit entry beside it says so (rule 6, invariant 6).
func (s *CanBoStore) CapMoiQuyen(ctx context.Context, tx *store.ScopedTx, vaiTroID, capBoi string) (int64, error) {
	kq, err := tx.Exec(ctx,
		`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_boi)
		 SELECT $1, $2, q.ma, $3 FROM quyen q
		 ON CONFLICT (tenant_id, vai_tro_id, quyen_ma) DO NOTHING`,
		string(tx.TenantID()), vaiTroID, capBoi)
	if err != nil {
		return 0, fmt.Errorf("quan_tri_mac_dinh: cấp mọi quyền: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("quan_tri_mac_dinh: đếm ô quyền: %w", err)
	}
	return n, nil
}

// ChenQuanTriMacDinh inserts the `admin` account and reports whether THIS call created it.
//
// false, nil means a concurrent first sign-in committed an `admin` first: the caller rolls its own
// transaction back and signs in against the row that won. `ON CONFLICT (tenant_id, email)` — the
// email key only, on purpose: a collision on the staff code is not "somebody else seeded", it is
// ErrMaCanBoDaDung and the caller mints another code.
//
// WHAT THE ROW CARRIES, column by column:
//
//	co_tai_khoan = true, dang_hoat_dong = true   an account that can sign in
//	phai_doi_mat_khau = true                     the password came from the platform, not the
//	                                             person (migration 0009 §1): the first request
//	                                             after sign-in is the forced change
//	mat_khau_hash                                argon2id of the configured value — never the value
//	vai_tro_id                                   the role CapMoiQuyen just filled
//	no phone, no personal mobile                 nobody's personal data (rule 3)
func (s *CanBoStore) ChenQuanTriMacDinh(ctx context.Context, tx *store.ScopedTx, id, ma, bam, vaiTroID string) (bool, error) {
	kq, err := tx.Exec(ctx,
		`INSERT INTO nguoi_dung
		   (tenant_id, id, ma, ho_ten, email, vai_tro_id,
		    co_tai_khoan, dang_hoat_dong, phai_doi_mat_khau, mat_khau_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, true, true, true, $7)
		 ON CONFLICT (tenant_id, email) DO NOTHING`,
		string(tx.TenantID()), id, ma, HoTenQuanTriMacDinh, EmailQuanTriMacDinh, vaiTroID, bam)
	if err != nil {
		return false, dichLoiGhiCanBo("tạo quản trị mặc định", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("quan_tri_mac_dinh: đếm dòng tài khoản: %w", err)
	}
	return n == 1, nil
}
