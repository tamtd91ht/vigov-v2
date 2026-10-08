package store

// The two reads the Excel import of the budget board needs (ADR 0081 #6), inside its transaction.
// The writes are thu_chi_ngan_sach.go's own statements: an imported sheet is inserted, starred and
// soft deleted by exactly the statements a hand-made one is, so the properties stated at the top of that
// file (`is_headline` set by two statements only, no hard delete) hold for this path unchanged.

import (
	"context"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// LiveSheetsForUpdate reads the LIVE sheets of one year and kind and locks them until the transaction
// ends — the same FOR UPDATE every line write takes first, so an import and an edit of the sheet it
// replaces serialise. Normally zero or one row; the caller refuses more than one rather than choosing.
func (s *NganSachStore) LiveSheetsForUpdate(ctx context.Context, tx *store.ScopedTx,
	year int, kind domain.LoaiBang) ([]domain.BangNganSach, error) {

	const stmt = `SELECT ` + cotBang + ` FROM bang_ngan_sach ` +
		`WHERE tenant_id = $1 AND nam = $2 AND loai = $3 AND deleted_at IS NULL ORDER BY lan DESC FOR UPDATE`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), year, string(kind))
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc bảng còn sống để nạp: %w", err)
	}
	defer rows.Close()
	var out []domain.BangNganSach
	for rows.Next() {
		b, err := quetBang(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng bảng để nạp: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt bảng để nạp: %w", err)
	}
	return out, nil
}

// handEntryLines — the lines of one sheet that carry LIVE batches, with their batch count. Line and
// batch both live; the commune is bound on both sides of the join (rule 1).
const handEntryLines = `SELECT k.tt, k.ten, count(d.id) AS hand_entry_lines
	FROM khoan_muc_ngan_sach k
	JOIN dot_thu_chi d ON d.tenant_id = k.tenant_id AND d.khoan_muc_id = k.id
	WHERE k.tenant_id = $1 AND k.bang_id = $2 AND k.deleted_at IS NULL AND d.deleted_at IS NULL
	GROUP BY k.id, k.tt, k.ten, k.thu_tu
	ORDER BY k.thu_tu, k.id`

// handEntryCells — the cells of one sheet entered BY HAND:
//
//	a sheet created by hand (`nguon_tep` NULL)   every filled cell
//	a sheet loaded from Excel                     every cell written AFTER the load (typed, edited, or
//	                                              cleared — a cleared figure is a hand act too)
//
// "After the load" is `g.cap_nhat_luc > b.tao_luc`: the load inserts the sheet and its cells in ONE
// transaction, where now() is one instant, so a loaded cell is EQUAL to the sheet's creation time and
// any later write is strictly greater. Both sides are the database's clock; the application's clock
// takes no part.
const handEntryCells = `SELECT count(*) AS hand_entry_cells
	FROM gia_tri_khoan_muc g
	JOIN khoan_muc_ngan_sach k ON k.tenant_id = g.tenant_id AND k.id = g.khoan_muc_id
	JOIN bang_ngan_sach b ON b.tenant_id = k.tenant_id AND b.id = k.bang_id
	WHERE g.tenant_id = $1 AND b.id = $2 AND k.deleted_at IS NULL
	  AND (CASE WHEN b.nguon_tep IS NULL THEN g.gia_tri IS NOT NULL ELSE g.cap_nhat_luc > b.tao_luc END)`

// HandEntries reports what the sheet holds that somebody entered by hand: the "TT Tên" of every line
// with live batches, the number of those batches, and the number of hand-entered cells.
func (s *NganSachStore) HandEntries(ctx context.Context, tx *store.ScopedTx,
	sheetID string) ([]string, int, int, error) {

	rows, err := tx.Underlying().QueryContext(ctx, handEntryLines, string(tx.TenantID()), sheetID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("ngan_sach: đọc đợt của bảng: %w", err)
	}
	defer rows.Close()
	var lines []string
	batches := 0
	for rows.Next() {
		var tt, ten string
		var count int
		if err := rows.Scan(&tt, &ten, &count); err != nil {
			return nil, 0, 0, fmt.Errorf("ngan_sach: đọc dòng đợt của bảng: %w", err)
		}
		lines = append(lines, strings.TrimSpace(tt+" "+ten))
		batches += count
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("ngan_sach: duyệt đợt của bảng: %w", err)
	}

	var cells int
	if err := tx.Underlying().QueryRowContext(ctx, handEntryCells, string(tx.TenantID()), sheetID).Scan(&cells); err != nil {
		return nil, 0, 0, fmt.Errorf("ngan_sach: đếm ô nhập tay: %w", err)
	}
	return lines, batches, cells, nil
}
