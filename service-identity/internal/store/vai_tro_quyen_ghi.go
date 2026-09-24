package store

// The WRITE path of the Phân quyền matrix: saving ONE role's column.
// PUT /api/v1/roles/{id}/permissions — the use case is app/phan_quyen_vai_tro.go.
//
// THE SAME FIVE PROPERTIES AS can_bo_ghi.go, for the same reasons:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context.
//     No method here takes a commune, so no caller can name another commune's role.
//  2. NOTHING HERE OPENS A TRANSACTION. Every method takes the *store.ScopedTx the use case opened,
//     so the grant change and its audit entry cannot land in two transactions (rule 6, inv 3).
//  3. EVERY READ A GUARD DEPENDS ON RUNS INSIDE THAT TRANSACTION, and the ones whose answer could
//     change under the guard take `FOR UPDATE`.
//  4. A SOFT-DELETED ROLE IS NOT A ROLE. It is 404, never a column that can be re-granted.
//  5. `quyen` IS PLATFORM-SCOPE AND IS READ AS SUCH — see KhoaKhongTonTai.
//
// THE LOCK ORDER IS PART OF THE CONTRACT, and the use case takes the locks in exactly this order:
//
//	1. holders of `admin.user`   truyVanQuanTriDeGhi — THE SAME TEXT the staff routes lock with
//	                             (DatKhoa, DoiVaiTro, Xoa), so a role save and a staff lock queue on
//	                             the same first rows instead of deadlocking in opposite orders.
//	2. holders of `admin.role`   truyVanPhanQuyenDeGhi — same shape, same predicate.
//	3. the target role row       VaiTroDeGhi, then that role's grant rows.
//
// Every one of these is ordered, so two saves of two different columns take shared rows in the same
// sequence.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// QuyenPhanQuyen is the key that guards the Phân quyền screen and its write route, and the second
// key open question #13 is counted on for THIS route (user decision 2026-09-24): a commune with
// nobody left holding `admin.role` can no longer change any role's permissions.
//
// A CONSTANT SPLICED INTO THE SQL, never bound — the reason is on QuyenQuanTriNguoiDung.
//
// THE KEY EXISTS IN `quyen` (migration 0001, "Phân quyền"); the GET route already declares it.
const QuyenPhanQuyen = "admin.role"

// truyVanPhanQuyenDeGhi lists — and LOCKS — everybody who can exercise `admin.role` today. Same
// halves as truyVanQuanTriDeGhi, so the two cannot disagree about what "holds a key" means.
const truyVanPhanQuyenDeGhi = truyVanGiuQuyenDeGhiDau + QuyenPhanQuyen + truyVanGiuQuyenDeGhiDuoi

// ErrVaiTroKhongTonTaiDeGhi — the role is absent, soft-deleted, or another commune's (which is the
// same thing, because every statement carries `tenant_id = $1`). One answer for all three, so none
// of them can be told apart by trying.
var ErrVaiTroKhongTonTaiDeGhi = errors.New("phan_quyen: vai trò không tồn tại trong xã")

// PhanQuyenStore writes `vai_tro_quyen`.
type PhanQuyenStore struct{ db *store.DB }

func NewPhanQuyenStore(db *store.DB) *PhanQuyenStore { return &PhanQuyenStore{db: db} }

// QuanTriDeGhi — step 1 of the lock order: every active holder of `admin.user`, locked.
func (s *PhanQuyenStore) QuanTriDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.NguoiGiuQuyen, error) {
	ds, err := docNguoiGiuDeGhi(ctx, tx, truyVanQuanTriDeGhi)
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: đọc người giữ admin.user: %w", err)
	}
	return ds, nil
}

// PhanQuyenDeGhi — step 2 of the lock order: every active holder of `admin.role`, locked.
//
// THE ACTOR IS IN THIS SET — the route requires `admin.role` — so the actor's own `nguoi_dung` row is
// locked from here on. That is what makes VaiTroCuaNguoi and QuyenDangGiu, read afterwards, a
// stable answer rather than a snapshot somebody else can move.
func (s *PhanQuyenStore) PhanQuyenDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.NguoiGiuQuyen, error) {
	ds, err := docNguoiGiuDeGhi(ctx, tx, truyVanPhanQuyenDeGhi)
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: đọc người giữ admin.role: %w", err)
	}
	return ds, nil
}

// VaiTroDeGhi — step 3: locks the target role row, then its grant rows, and returns the grants.
//
// THE ROLE ROW LOCK IS WHAT SERIALISES TWO SAVES OF THE SAME COLUMN. Without it both read the same
// "before", both compute a diff against it, and the second overwrites the first — or collides on the
// primary key inserting a cell the first one just inserted.
func (s *PhanQuyenStore) VaiTroDeGhi(ctx context.Context, tx *store.ScopedTx, vaiTroID string) ([]string, error) {
	if vaiTroID == "" {
		return nil, ErrVaiTroKhongTonTaiDeGhi
	}
	const stmtVaiTro = `SELECT id FROM vai_tro
	                    WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`
	var id string
	err := tx.Underlying().QueryRowContext(ctx, stmtVaiTro, string(tx.TenantID()), vaiTroID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrVaiTroKhongTonTaiDeGhi
	}
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: khoá dòng vai trò: %w", err)
	}

	const stmtCap = `SELECT quyen_ma FROM vai_tro_quyen
	                 WHERE tenant_id = $1 AND vai_tro_id = $2 ORDER BY quyen_ma FOR UPDATE`
	ds, err := docCotChuoi(ctx, tx, stmtCap, vaiTroID)
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: đọc quyền của vai trò: %w", err)
	}
	return ds, nil
}

// VaiTroCuaNguoi returns the role a person currently holds, "" for none — #14's first constraint
// (the actor may not save the column of their own role).
//
// NOT FILTERED BY dieuKienGiuQuyen: the question is "is this THEIR role", not "can they exercise it".
// A soft-deleted person cannot reach here (they have no session), and refusing on the row as it
// stands is the conservative reading.
func (s *PhanQuyenStore) VaiTroCuaNguoi(ctx context.Context, tx *store.ScopedTx, canBoID string) (string, error) {
	const stmt = `SELECT coalesce(vai_tro_id, '') FROM nguoi_dung
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	var vt string
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), canBoID).Scan(&vt)
	if errors.Is(err, sql.ErrNoRows) {
		// The actor's own row is gone. Nothing they hold can be proven, so the use case refuses.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("phan_quyen: đọc vai trò của người thực hiện: %w", err)
	}
	return vt, nil
}

// QuyenDangGiu lists the keys a person can exercise right now — the same text as checker.go
// (truyVanQuyen1Nguoi), so #14's subset rule and the route guard cannot disagree.
func (s *PhanQuyenStore) QuyenDangGiu(ctx context.Context, tx *store.ScopedTx, canBoID string) ([]string, error) {
	if canBoID == "" {
		return nil, nil
	}
	ds, err := docCotChuoi(ctx, tx, truyVanQuyen1Nguoi, canBoID)
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: đọc quyền đang giữ: %w", err)
	}
	return ds, nil
}

// truyVanKhoaTonTai returns which of the submitted keys exist in the platform catalogue.
//
// `quyen` HAS NO tenant_id, and the "$1 is not empty" predicate is the same assertion
// truyVanDanhMucQuyen makes (quyen.go): $1 is the commune bound by docCotChuoi, consumed so the read
// stays on the scoped path rather than asking core/store for an unscoped escape hatch.
const truyVanKhoaTonTai = `
SELECT q.ma
FROM quyen q
WHERE $1::text <> ''
  AND q.ma = ANY($2)
ORDER BY q.ma`

// KhoaKhongTonTai returns the keys of `ds` that are NOT in `quyen`, in the order given.
//
// RULE 5, INVARIANT 3c: a key no migration seeds is a right no route checks. The foreign key on
// `vai_tro_quyen.quyen_ma` would refuse it too, but only as a constraint error AFTER the delete
// half of the save had run — the rollback is correct and the message names nothing. This names the
// keys, before anything is written.
func (s *PhanQuyenStore) KhoaKhongTonTai(ctx context.Context, tx *store.ScopedTx, ds []string) ([]string, error) {
	if len(ds) == 0 {
		return nil, nil
	}
	co, err := docCotChuoi(ctx, tx, truyVanKhoaTonTai, ds)
	if err != nil {
		return nil, fmt.Errorf("phan_quyen: kiểm khoá quyền tồn tại: %w", err)
	}
	daCo := make(map[string]bool, len(co))
	for _, k := range co {
		daCo[k] = true
	}
	var thieu []string
	for _, k := range ds {
		if !daCo[k] {
			thieu = append(thieu, k)
		}
	}
	return thieu, nil
}

// ThemCap inserts the added cells. `cap_boi` is the actor's STAFF CODE (rule 6, invariant 8) —
// never the internal id; the use case refuses an empty one before reaching here.
func (s *PhanQuyenStore) ThemCap(ctx context.Context, tx *store.ScopedTx, vaiTroID string, them []string, capBoi string) error {
	if len(them) == 0 {
		return nil
	}
	const stmt = `INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_boi)
	              SELECT $1, $2, k, $4 FROM unnest($3::text[]) AS k`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), vaiTroID, them, capBoi)
	if err != nil {
		return fmt.Errorf("phan_quyen: thêm ô quyền: %w", err)
	}
	return dungSoDong(kq, len(them), "thêm ô quyền")
}

// BoCap removes the unticked cells.
//
// A HARD DELETE, AND THAT IS A RECORDED USER DECISION, NOT AN EXCEPTION TAKEN QUIETLY (2026-09-24,
// option A): a grant cell is authorisation CONFIGURATION, not an archival record. The table was
// built without soft-delete columns and with `PRIMARY KEY (tenant_id, vai_tro_id, quyen_ma)`, so a
// soft-deleted cell would block its own re-grant. The history rule 7 protects is kept where it is
// append-only: the audit entry written IN THIS SAME TRANSACTION carries the full before and after
// sets and names every removed key.
//
// BOUNDED ON ALL THREE COLUMNS OF THE KEY: this commune, this role, these keys. Never a whole column.
func (s *PhanQuyenStore) BoCap(ctx context.Context, tx *store.ScopedTx, vaiTroID string, bo []string) error {
	if len(bo) == 0 {
		return nil
	}
	const stmt = `DELETE FROM vai_tro_quyen
	              WHERE tenant_id = $1 AND vai_tro_id = $2 AND quyen_ma = ANY($3)`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), vaiTroID, bo)
	if err != nil {
		return fmt.Errorf("phan_quyen: gỡ ô quyền: %w", err)
	}
	return dungSoDong(kq, len(bo), "gỡ ô quyền")
}

// dungSoDong refuses a write that touched a different number of rows than the diff said. The rows
// were read FOR UPDATE in this transaction, so a mismatch means the diff and the table disagree —
// committing it would audit a change that did not happen as recorded.
func dungSoDong(kq sql.Result, muon int, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("phan_quyen: %s: đếm dòng: %w", viec, err)
	}
	if n != int64(muon) {
		return fmt.Errorf("phan_quyen: %s: ghi %d dòng, muốn %d", viec, n, muon)
	}
	return nil
}
