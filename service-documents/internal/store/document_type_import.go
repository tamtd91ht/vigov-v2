package store

// The one read the document-type Excel import needs beyond the form's own statements (ADR 0059 §3).
// The rows it creates go through LoaiVanBanStore.Chen — the form's INSERT, with `nguon` and
// `ma_nguon_re_nhanh` as literals — so an imported row is a tier-1 row of the commune, exactly like a
// typed one.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// MaxImportSnapshotRows bounds the import snapshot: live rows are capped at TranDanhMucLoaiVanBan, and
// ten times that leaves room for a long history of soft-deleted rows. Past it the import REFUSES — a
// snapshot missing rows would miss a taken code, and the plan would promise a row the unique key then
// refuses.
const MaxImportSnapshotRows = 10 * TranDanhMucLoaiVanBan

// ErrImportSnapshotTooLarge — the commune's catalogue history exceeds MaxImportSnapshotRows.
var ErrImportSnapshotTooLarge = errors.New("loai_van_ban: ảnh chụp danh mục vượt trần")

// ImportSnapshot reads EVERY row of this commune's catalogue — soft-deleted ones INCLUDED and flagged —
// inside the import's transaction. Deleted rows are there because their codes are still taken (rule 7,
// invariant 3); the planner never treats them as live for the label check.
func (s *LoaiVanBanStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingDocumentType, error) {
	const stmt = `SELECT ma, nhan, deleted_at IS NOT NULL FROM loai_van_ban ` +
		`WHERE tenant_id = $1 ORDER BY ma LIMIT $2`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), MaxImportSnapshotRows+1)
	if err != nil {
		return nil, fmt.Errorf("loai_van_ban: đọc ảnh chụp để nhập: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ExistingDocumentType, 0, 16)
	for rows.Next() {
		var e domain.ExistingDocumentType
		if err := rows.Scan(&e.Code, &e.Label, &e.Deleted); err != nil {
			return nil, fmt.Errorf("loai_van_ban: đọc dòng ảnh chụp: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_van_ban: duyệt ảnh chụp: %w", err)
	}
	if len(out) > MaxImportSnapshotRows {
		return nil, ErrImportSnapshotTooLarge
	}
	return out, nil
}
