package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The one READ the org-chart import needs (app.OrgUnitImporter). Its inserts reuse Chen, and its
// parent locks reuse KhoaBoPhan — the same statements POST /api/v1/org-units runs, so the import
// cannot write a row the form could not.

// maxOrgUnitSnapshot bounds ImportSnapshot. It counts SOFT-DELETED rows too, so it sits above
// TranDanhMucBoPhan (live rows only); past it the data is not an org chart and the import refuses
// rather than planning against a truncated list of taken codes.
const maxOrgUnitSnapshot = 5000

// ErrOrgUnitSnapshotTooLarge — the commune has more org-unit rows, deleted included, than
// maxOrgUnitSnapshot. 500: the data is wrong, not the file.
var ErrOrgUnitSnapshotTooLarge = errors.New("bo_phan: số dòng bộ phận (kể cả đã xoá) vượt trần ảnh chụp")

// ImportSnapshot reads EVERY org unit of this commune, soft-deleted rows included and flagged.
//
// DELETED ROWS ARE NEEDED because `UNIQUE (tenant_id, ma)` counts them (0001_init.sql:86): a plan
// that did not know a deleted unit's code would pick it, and the INSERT would refuse the whole file.
// Read INSIDE the import's transaction so the plan and the inserts see one state; a concurrent
// creation of the same code is still caught by the unique key, which rolls the whole file back.
func (s *BoPhanStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingOrgUnit, error) {
	const stmt = `SELECT id, ma, ten, coalesce(cha_id,''), deleted_at IS NOT NULL FROM bo_phan ` +
		`WHERE tenant_id = $1 ORDER BY thu_tu, ten, id LIMIT $2`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), maxOrgUnitSnapshot+1)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: đọc ảnh chụp để nhập: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ExistingOrgUnit, 0, 16)
	for rows.Next() {
		var u domain.ExistingOrgUnit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.ParentID, &u.Deleted); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng ảnh chụp: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt ảnh chụp: %w", err)
	}
	if len(out) > maxOrgUnitSnapshot {
		return nil, ErrOrgUnitSnapshotTooLarge
	}
	return out, nil
}
