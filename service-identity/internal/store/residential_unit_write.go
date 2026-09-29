package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The WRITE paths of the residential units (user decision 2026-09-29, ADR 0059 §2): create, edit, take
// out of use, and the Excel import — under `admin.org`. There is NO merge and NO split (rule 1, stop
// condition #3), and NO statement here writes `deleted_at`: taking a unit out of use is `dang_dung =
// false`, so the unit keeps labelling the records that point at it.
//
// THE SAME THREE PROPERTIES AS bo_phan_ghi.go, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID() (rule 1, invariants 4 and 5), and every
//     joined table is constrained to it in its ON clause.
//  2. NOTHING HERE OPENS A TRANSACTION. Every write takes the *store.ScopedTx the use case opened and
//     writes its audit entry in (rule 6, invariant 3).
//  3. `ma` APPEARS IN NO UPDATE — and migration 0005's trigger refuses one anyway (rule 7, invariant 3).

var (
	// ErrResidentialUnitNotFound — the unit NAMED IN THE PATH matches no live row of this commune. One
	// answer for an invented id, a soft-deleted row and another commune's unit (rule 4, forbidden #2
	// applied to configuration). 404.
	ErrResidentialUnitNotFound = errors.New("thon_to_dan_pho: không tìm thấy thôn / tổ dân phố")

	// ErrResidentialUnitCodeTaken — `UNIQUE (tenant_id, ma)` refused the code. Not partial: a code once
	// issued stays taken for good. 409.
	ErrResidentialUnitCodeTaken = errors.New("thon_to_dan_pho: mã đã được cấp trong xã")

	// ErrResidentialUnitNameTaken — another live unit of the commune (in use or out of use) has the same
	// name. Decided by the use case against the snapshot; 409.
	ErrResidentialUnitNameTaken = errors.New("thon_to_dan_pho: xã đã có thôn / tổ dân phố cùng tên")

	// ErrResidentialUnitTypeNotFound — the type is not a live, in-use type of THIS commune (another
	// commune's code included). 400: a value the client sent is wrong.
	ErrResidentialUnitTypeNotFound = errors.New("thon_to_dan_pho: loại đơn vị dân cư không có hoặc đã ngưng dùng")

	// ErrHeadStaffNotFound — the staff code is not a live, unlocked member of staff of THIS commune. 400.
	ErrHeadStaffNotFound = errors.New("thon_to_dan_pho: không tìm thấy cán bộ đang làm việc có mã này")

	// ErrResidentialUnitSnapshotTooLarge — past maxResidentialUnitSnapshot rows (deleted included) the
	// data is not a list of hamlets; refused rather than planned against a truncated list. 500.
	ErrResidentialUnitSnapshotTooLarge = errors.New("thon_to_dan_pho: số dòng (kể cả đã xoá) vượt trần ảnh chụp")

	// ErrTooManyHeadCandidates — the commune has more live staff than maxHeadCandidates. 500.
	ErrTooManyHeadCandidates = errors.New("thon_to_dan_pho: số cán bộ vượt trần danh sách chọn trưởng thôn")
)

const (
	// maxResidentialUnitSnapshot counts SOFT-DELETED rows too, so it sits well above
	// TranDanhSachThonToDanPho (live rows only) — the argument of maxOrgUnitSnapshot.
	maxResidentialUnitSnapshot = 5000

	// maxHeadCandidates bounds the staff read for the template dropdown and the import. A commune has a
	// few dozen staff; the bound is where the data has stopped being a commune's staff.
	maxHeadCandidates = 2000
)

// LockRegister serialises every write to ONE commune's residential units until the transaction ends.
//
// WHY: two rules here are "no other live unit has this name" and "no row holds this code", decided by
// reading the snapshot and then inserting. The unique key is the floor for the code; NOTHING in the
// schema is a floor for the name (a unique index on a case-folded name would need a collation this
// repository cannot verify), so two administrators creating "Thôn Bình An" at the same instant would
// both see it absent. A transaction-scoped advisory lock keyed on the commune makes the second wait
// for the first commit and see its row. It is per commune, so no commune waits for another, and it is
// released by COMMIT or ROLLBACK — there is no path that forgets it.
func (s *ThonToDanPhoStore) LockRegister(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('thon_to_dan_pho:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("thon_to_dan_pho: khoá sổ đơn vị dân cư: %w", err)
	}
	return nil
}

// UnitSnapshot reads EVERY residential unit of this commune — soft-deleted rows included and flagged,
// units out of use included as live — inside the caller's transaction. Deleted rows are needed because
// the unique key on `ma` counts them.
func (s *ThonToDanPhoStore) UnitSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingResidentialUnit, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `id, ma, ten, deleted_at IS NOT NULL`, `thon_to_dan_pho`,
		`ORDER BY ma LIMIT $2`, maxResidentialUnitSnapshot+1)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: đọc ảnh chụp: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ExistingResidentialUnit, 0, 32)
	for rows.Next() {
		var u domain.ExistingResidentialUnit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.Deleted); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng ảnh chụp: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt ảnh chụp: %w", err)
	}
	if len(out) > maxResidentialUnitSnapshot {
		return nil, ErrResidentialUnitSnapshotTooLarge
	}
	return out, nil
}

// queryOne is the shape Scoped.Query and ScopedTx.Query share: ONE table, `WHERE tenant_id = $1`
// added by core/store, the caller's placeholders from $2.
type queryOne func(ctx context.Context, cot, bang, tail string, args ...any) (*sql.Rows, error)

// The two pick lists. `dang_dung` / `dang_hoat_dong` AND `deleted_at IS NULL`: a NEW unit may not be
// filed under a type the commune has taken out of use, nor headed by somebody locked or removed.
// Existing units keep whatever they already carry — nothing here rewrites them.
//
// STAFF WITHOUT A SIGN-IN ACCOUNT ARE INCLUDED, unlike the work-assignment picker
// (can_bo_chon_nguoi.go, `co_tai_khoan`): a head of hamlet is a fact about the territory, not somebody
// work is routed to, and many heads are recorded in the register without an account.
const (
	usableTypeCols = `ma, nhan`
	usableTypeTail = `AND deleted_at IS NULL AND dang_dung ORDER BY thu_tu, ma LIMIT $2`

	eligibleHeadCols = `id, ma, ho_ten`
	eligibleHeadTail = `AND deleted_at IS NULL AND dang_hoat_dong ORDER BY ho_ten, ma LIMIT $2`
)

func usableTypes(ctx context.Context, q queryOne) ([]domain.ResidentialUnitTypeChoice, error) {
	rows, err := q(ctx, usableTypeCols, `loai_don_vi_dan_cu`, usableTypeTail, TranDanhMucLoaiDonViDanCu+1)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: đọc loại đang dùng: %w", err)
	}
	defer rows.Close()
	var out []domain.ResidentialUnitTypeChoice
	for rows.Next() {
		var t domain.ResidentialUnitTypeChoice
		if err := rows.Scan(&t.Code, &t.Label); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng loại: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt loại: %w", err)
	}
	if len(out) > TranDanhMucLoaiDonViDanCu {
		return nil, ErrQuaNhieuLoaiDonViDanCu
	}
	return out, nil
}

func eligibleHeads(ctx context.Context, q queryOne) ([]domain.HeadStaffChoice, error) {
	rows, err := q(ctx, eligibleHeadCols, `nguoi_dung`, eligibleHeadTail, maxHeadCandidates+1)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: đọc cán bộ: %w", err)
	}
	defer rows.Close()
	var out []domain.HeadStaffChoice
	for rows.Next() {
		var c domain.HeadStaffChoice
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng cán bộ: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt cán bộ: %w", err)
	}
	if len(out) > maxHeadCandidates {
		return nil, ErrTooManyHeadCandidates
	}
	return out, nil
}

// ImportSnapshot reads everything the import plans against, inside the import's transaction.
func (s *ThonToDanPhoStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) (domain.ResidentialUnitImportSnapshot, error) {
	units, err := s.UnitSnapshot(ctx, tx)
	if err != nil {
		return domain.ResidentialUnitImportSnapshot{}, err
	}
	types, err := usableTypes(ctx, tx.Query)
	if err != nil {
		return domain.ResidentialUnitImportSnapshot{}, err
	}
	staff, err := eligibleHeads(ctx, tx.Query)
	if err != nil {
		return domain.ResidentialUnitImportSnapshot{}, err
	}
	return domain.ResidentialUnitImportSnapshot{Units: units, Types: types, Staff: staff}, nil
}

// TemplateChoices reads the two dropdowns of the import template — outside any transaction, because
// the template writes nothing. The same predicates as ImportSnapshot, so the file never offers a value
// the import then refuses.
func (s *ThonToDanPhoStore) TemplateChoices(ctx context.Context) ([]domain.ResidentialUnitTypeChoice, []domain.HeadStaffChoice, error) {
	sc := s.db.For(ctx)
	types, err := usableTypes(ctx, sc.Query)
	if err != nil {
		return nil, nil, err
	}
	staff, err := eligibleHeads(ctx, sc.Query)
	if err != nil {
		return nil, nil, err
	}
	return types, staff, nil
}

// UsableType reads one type a unit may be filed under — live and in use in THIS commune — and returns
// its label. FOR SHARE, so it cannot be taken out of use between this read and the commit.
// ErrResidentialUnitTypeNotFound otherwise.
func (s *ThonToDanPhoStore) UsableType(ctx context.Context, tx *store.ScopedTx, code string) (string, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `nhan`, `loai_don_vi_dan_cu`,
		`AND ma = $2 AND deleted_at IS NULL AND dang_dung FOR SHARE`, code)
	if err != nil {
		return "", fmt.Errorf("thon_to_dan_pho: đọc loại: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("thon_to_dan_pho: đọc loại: %w", err)
		}
		return "", ErrResidentialUnitTypeNotFound
	}
	var label string
	if err := rows.Scan(&label); err != nil {
		return "", fmt.Errorf("thon_to_dan_pho: đọc nhãn loại: %w", err)
	}
	return label, nil
}

// EligibleHeadByCode resolves a staff CODE to the member of staff who may head a unit: live and not
// locked, in THIS commune. FOR SHARE, so they cannot be locked or removed between this read and the
// commit. ErrHeadStaffNotFound otherwise — one answer for an invented code, a locked person and
// another commune's code.
func (s *ThonToDanPhoStore) EligibleHeadByCode(ctx context.Context, tx *store.ScopedTx, code string) (domain.HeadStaffChoice, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, eligibleHeadCols, `nguoi_dung`,
		`AND ma = $2 AND deleted_at IS NULL AND dang_hoat_dong FOR SHARE`, code)
	if err != nil {
		return domain.HeadStaffChoice{}, fmt.Errorf("thon_to_dan_pho: đọc cán bộ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.HeadStaffChoice{}, fmt.Errorf("thon_to_dan_pho: đọc cán bộ: %w", err)
		}
		return domain.HeadStaffChoice{}, ErrHeadStaffNotFound
	}
	var c domain.HeadStaffChoice
	if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
		return domain.HeadStaffChoice{}, fmt.Errorf("thon_to_dan_pho: đọc dòng cán bộ: %w", err)
	}
	return c, nil
}

// lockResidentialUnitStmt reads one LIVE unit of this commune with its type label and its head, and
// holds its row until the transaction ends. `FOR UPDATE OF tt`: only the unit is locked — PostgreSQL
// refuses a lock on the nullable side of an outer join, and neither the type nor the person is being
// written. POSITIONAL, in lockstep with the Scan below and with truyVanThonToDanPho.
const lockResidentialUnitStmt = `
SELECT tt.id, tt.ma, tt.ten, coalesce(tt.loai, ''), coalesce(l.nhan, ''),
       tt.so_ho, tt.nhan_khau, tt.dang_dung,
       coalesce(tt.head_staff_id, ''), coalesce(nd.ma, ''), coalesce(nd.ho_ten, ''), tt.sort_order
FROM thon_to_dan_pho tt
LEFT JOIN loai_don_vi_dan_cu l
       ON l.tenant_id  = tt.tenant_id
      AND l.ma         = tt.loai
      AND l.deleted_at IS NULL
LEFT JOIN nguoi_dung nd
       ON nd.tenant_id = tt.tenant_id
      AND nd.id        = tt.head_staff_id
WHERE tt.tenant_id = $1 AND tt.id = $2 AND tt.deleted_at IS NULL
FOR UPDATE OF tt`

// LockUnit reads one live unit of this commune under a row lock. ErrResidentialUnitNotFound otherwise.
func (s *ThonToDanPhoStore) LockUnit(ctx context.Context, tx *store.ScopedTx, id string) (domain.ThonToDanPho, error) {
	if id == "" {
		return domain.ThonToDanPho{}, ErrResidentialUnitNotFound
	}
	var u domain.ThonToDanPho
	var households, population sql.NullInt64
	err := tx.Underlying().QueryRowContext(ctx, lockResidentialUnitStmt, string(tx.TenantID()), id).
		Scan(&u.ID, &u.Ma, &u.Ten, &u.LoaiMa, &u.LoaiNhan, &households, &population, &u.DangDung,
			&u.HeadStaffID, &u.HeadStaffCode, &u.HeadStaffName, &u.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ThonToDanPho{}, ErrResidentialUnitNotFound
	}
	if err != nil {
		return domain.ThonToDanPho{}, fmt.Errorf("thon_to_dan_pho: khoá dòng: %w", err)
	}
	if households.Valid {
		n := int(households.Int64)
		u.SoHo = &n
	}
	if population.Valid {
		n := int(population.Int64)
		u.NhanKhau = &n
	}
	return u, nil
}

// nullCount carries "not entered" to SQL as NULL, never 0.
func nullCount(n *int) any {
	if n == nil {
		return nil
	}
	return int64(*n)
}

// InsertUnit writes one new unit. The id is minted by the caller. `loai` and `head_staff_id` go in as
// NULL when empty (nullif): an empty string would fail the composite foreign keys.
func (s *ThonToDanPhoStore) InsertUnit(ctx context.Context, tx *store.ScopedTx, u domain.ThonToDanPho) error {
	const stmt = `INSERT INTO thon_to_dan_pho
		(tenant_id, id, ten, ma, loai, so_ho, nhan_khau, dang_dung, head_staff_id, sort_order)
		VALUES ($1,$2,$3,$4,nullif($5,''),$6,$7,$8,nullif($9,''),$10)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), u.ID, u.Ten, u.Ma, u.LoaiMa,
		nullCount(u.SoHo), nullCount(u.NhanKhau), u.DangDung, u.HeadStaffID, u.SortOrder)
	if err != nil {
		return translateResidentialUnitWrite("chèn", err)
	}
	return nil
}

// UpdateUnit writes the editable columns of one live unit. `ma`, `id` AND EVERY SOFT-DELETE COLUMN ARE
// ABSENT FROM THE SET CLAUSE, so this statement cannot renumber an issued code, delete or resurrect a
// row, or move it to another commune whatever it is passed.
func (s *ThonToDanPhoStore) UpdateUnit(ctx context.Context, tx *store.ScopedTx, u domain.ThonToDanPho) error {
	if u.ID == "" {
		return ErrResidentialUnitNotFound
	}
	const stmt = `UPDATE thon_to_dan_pho SET ten = $3, loai = nullif($4,''), so_ho = $5, nhan_khau = $6,
		dang_dung = $7, head_staff_id = nullif($8,''), sort_order = $9, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), u.ID, u.Ten, u.LoaiMa,
		nullCount(u.SoHo), nullCount(u.NhanKhau), u.DangDung, u.HeadStaffID, u.SortOrder)
	if err != nil {
		return translateResidentialUnitWrite("cập nhật", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("thon_to_dan_pho: đếm dòng đã ghi: %w", err)
	}
	if n == 0 {
		// The caller holds the row FOR UPDATE, so this cannot normally happen. Checked because a write
		// path reporting success having changed nothing is the one failure nobody notices.
		return ErrResidentialUnitNotFound
	}
	return nil
}

// translateResidentialUnitWrite turns a constraint violation into a sentinel. SUBSTRING match on the
// constraint name, because the table is PARTITION BY HASH and PostgreSQL may name the partition's copy
// (dichLoiGhiBoPhan). Anything unrecognised is wrapped and passed on, never flattened into a refusal.
func translateResidentialUnitWrite(op string, err error) error {
	switch raw := err.Error(); {
	case strings.Contains(raw, "tenant_id_ma_key"):
		return ErrResidentialUnitCodeTaken
	case strings.Contains(raw, "loai_fkey"):
		return ErrResidentialUnitTypeNotFound
	case strings.Contains(raw, "head_staff_fkey"):
		return ErrHeadStaffNotFound
	default:
		return fmt.Errorf("thon_to_dan_pho: %s: %w", op, err)
	}
}
