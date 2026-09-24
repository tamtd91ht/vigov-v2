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

// The shared WRITE statements of this service's two reference catalogues (`loai_don_vi_dan_cu`,
// `khoi_nhiem_vu`, migration 0005) — user decision 2026-09-24: both are full catalogues.
//
// ONE SET OF STATEMENTS FOR BOTH TABLES, for the reason danh_muc.go gives for the read: the
// migration declares the two tables identically and attaches ONE trigger to both, "because the
// tiers are a property of the SHAPE" (0005:124). The typed wrappers live beside each store, so a
// handler still cannot be handed the wrong catalogue.
//
// FOUR THINGS HOLD ACROSS EVERY FUNCTION BELOW, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context
//     (rule 1, invariants 4 and 5). No function takes a commune.
//  2. `nguon` AND `ma_nguon_re_nhanh` ARE LITERALS IN THE INSERT AND APPEAR IN NO UPDATE. They decide
//     the tier; a bound parameter for either is a value that could come from a client.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS except maDanhMucDaDung, which exists precisely to see
//     them (an issued code is never reissued — rule 7, invariant 3).
//  4. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside it.
//
// `bang` IS INTERPOLATED AND MUST STAY A COMPILE-TIME CONSTANT — bangLoaiDonViDanCu or
// bangKhoiNhiemVu, never a request value. Same contract as docDanhMuc.

var (
	// ErrDanhMucKhongTonTai — no live row with that id in THIS commune. 404, and deliberately the
	// same answer for another commune's row, a soft-deleted one and an invented id.
	ErrDanhMucKhongTonTai = errors.New("danh_muc: không tồn tại trong xã này")

	// ErrMaDaTonTai — the code is already taken in this commune, SOFT-DELETED ROWS INCLUDED.
	// `UNIQUE (tenant_id, ma)` counts them too (migration 0005:69-74). 409.
	ErrMaDaTonTai = errors.New("danh_muc: mã đã được dùng trong xã này")

	// ErrDanhMucDayTran — the commune is at the catalogue's ceiling (TranDanhMuc… beside each store).
	// The read route REFUSES past that number, so the row that crossed it would turn every picker in
	// the commune into a 500. Refusing the write breaks one button instead. 409.
	ErrDanhMucDayTran = errors.New("danh_muc: danh mục đã đầy")
)

// khoaDongDanhMuc reads one LIVE row and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT: every write is read-decide-write (read the row, derive its tier, refuse
// or apply). Without the lock two administrators both decide against the old state and the second
// write silently overwrites the first — including one clearing `la_mac_dinh` while the other sets it.
func khoaDongDanhMuc(ctx context.Context, tx *store.ScopedTx, bang, id string) (dongDanhMuc, error) {
	if id == "" {
		return dongDanhMuc{}, ErrDanhMucKhongTonTai
	}
	stmt := `SELECT ` + cotDanhMuc + ` FROM ` + bang +
		` WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	d, err := quetDongDanhMuc(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return dongDanhMuc{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return dongDanhMuc{}, fmt.Errorf("%s: đọc dòng để sửa: %w", bang, err)
	}
	return d, nil
}

// maDanhMucDaDung reports whether the code is taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates can both
// pass this and the second then hits the key — dichLoiGhiDanhMuc turns that into the same 409.
func maDanhMucDaDung(ctx context.Context, tx *store.ScopedTx, bang, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT from the predicate — see ErrMaDaTonTai.
	stmt := `SELECT count(*) FROM ` + bang + ` WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("%s: kiểm tra mã trùng: %w", bang, err)
	}
	return n > 0, nil
}

// demDanhMucDangSong counts live rows for the ceiling check — the same predicate docDanhMuc reads
// with, so the two numbers mean the same thing.
func demDanhMucDangSong(ctx context.Context, tx *store.ScopedTx, bang string) (int, error) {
	stmt := `SELECT count(*) FROM ` + bang + ` WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("%s: đếm dòng đang sống: %w", bang, err)
	}
	return n, nil
}

// chenDanhMuc adds one row the COMMUNE owns. `nguon` AND `ma_nguon_re_nhanh` ARE LITERALS, NOT
// PARAMETERS: there is no $n for either, so no layer above can pass a value. Turning either into a
// parameter is the one edit that reopens the whole tier model, and it would look like tidying up.
// There is no function here that writes a `he-thong` row — that is commune onboarding (0005:38).
func chenDanhMuc(ctx context.Context, tx *store.ScopedTx, bang string, d dongDanhMuc) error {
	stmt := `INSERT INTO ` + bang +
		` (tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)` +
		` VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()),
		d.ID, d.Ma, d.Nhan, d.ThuTu, d.LaMacDinh, d.DangDung); err != nil {
		return dichLoiGhiDanhMuc(bang, "chèn", err)
	}
	return nil
}

// boMacDinhDanhMucKhac clears the default flag on every OTHER live row of this commune.
//
// SAME TRANSACTION AS THE WRITE THAT SETS THE NEW DEFAULT, and BEFORE it: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits one live default (0005:244-266), so setting the second one first is the
// statement that fails. In a separate transaction the window between them is a commune with none.
func boMacDinhDanhMucKhac(ctx context.Context, tx *store.ScopedTx, bang, trongID string) error {
	stmt := `UPDATE ` + bang + ` SET la_mac_dinh = false, cap_nhat_luc = now()` +
		` WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("%s: bỏ mặc định cũ: %w", bang, err)
	}
	return nil
}

// capNhatDanhMuc writes the four fields a commune may change. `ma`, `nguon` AND `ma_nguon_re_nhanh`
// APPEAR NOWHERE in the statement; the trigger refuses all three as well (0005:168-188), and both
// layers are meant — the floor, and the absence that makes the floor unreachable from here.
func capNhatDanhMuc(ctx context.Context, tx *store.ScopedTx, bang string, d dongDanhMuc) error {
	stmt := `UPDATE ` + bang +
		` SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, cap_nhat_luc = now()` +
		` WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), d.ID, d.Nhan, d.ThuTu, d.DangDung, d.LaMacDinh)
	if err != nil {
		return dichLoiGhiDanhMuc(bang, "cập nhật", err)
	}
	return doiMotDongDanhMuc(kq, bang, "cập nhật")
}

// xoaMemDanhMuc writes all THREE columns rule 7, invariant 1 names in one statement. `boi` is the
// remover's STAFF CODE (rule 6, invariant 8), never the internal id.
//
// `AND deleted_at IS NULL` MAKES A SECOND DELETE A 404 rather than a rewrite of who deleted the row
// and why (rule 7, forbidden #5).
func xoaMemDanhMuc(ctx context.Context, tx *store.ScopedTx, bang, id, boi, lyDo string) error {
	stmt := `UPDATE ` + bang +
		` SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()` +
		` WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return dichLoiGhiDanhMuc(bang, "xoá mềm", err)
	}
	return doiMotDongDanhMuc(kq, bang, "xoá mềm")
}

// doiMotDongDanhMuc turns "the UPDATE matched nothing" into ErrDanhMucKhongTonTai. The caller holds
// the row FOR UPDATE, so this should not happen; checked because a write reporting success having
// changed nothing is the failure nobody notices.
func doiMotDongDanhMuc(kq sql.Result, bang, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %s: đọc số dòng: %w", bang, viec, err)
	}
	if n == 0 {
		return ErrDanhMucKhongTonTai
	}
	return nil
}

// dichLoiGhiDanhMuc turns what the DATABASE refused into the same sentinels the use case returns
// when it refuses first — so a race past the application check still reaches the caller as 409 and
// not as 500.
//
// SUBSTRING MATCHES, for the reason dichLoiGhiBoPhan gives: both tables are PARTITION BY HASH, so
// PostgreSQL names the partition's copy of the key (`khoi_nhiem_vu_p07_tenant_id_ma_key`) and the
// trigger reports the leaf partition in TG_TABLE_NAME. The trigger phrases are the migration's own
// RAISE texts (0005:190-203). Anything unrecognised is wrapped and passed on, never flattened into a
// business refusal.
func dichLoiGhiDanhMuc(bang, viec string, err error) error {
	switch tho := err.Error(); {
	case strings.Contains(tho, "tenant_id_ma_key"):
		return ErrMaDaTonTai
	case strings.Contains(tho, "a system row is disabled, never deleted"):
		return domain.ErrKhongXoaDuocMucHeThong
	case strings.Contains(tho, "a tier-3 row cannot be taken out of use"):
		return domain.ErrKhongTatDuocMucReNhanh
	default:
		return fmt.Errorf("%s: %s: %w", bang, viec, err)
	}
}
