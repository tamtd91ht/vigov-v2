package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// SetPublicationStatus records a staff moderation decision on the public page (ADR 0050 point 8):
// `publication_status` moves from `from` to `to`, and NOTHING ELSE on the row moves.
//
// `trang_thai` IS NOT IN THE STATEMENT, and its absence is the user's decision of 28/09/2026: a
// moderation never changes the lifecycle status (the requirement's RECEIVED -> SCREENING side effect is
// not copied). Neither is any deadline column — rule 10, invariant 2 fixes those at intake and at
// classification only.
//
// THE WHERE CLAUSE CARRIES THE EXPECTED `from`, the discipline of every write in xu_ly_phan_anh.go:
// two officers moderating one petition at once cannot both succeed; the second matches no row and is
// told ErrPhieuDaChuyenTrang (409, reload) instead of silently overwriting the first decision. The
// caller reads the row FOR UPDATE first, so zero rows can only mean that race.
//
// THE DATABASE IS THE FLOOR: migration 0017's CHECKs refuse a fourth value and refuse `cong-khai` on a
// `can-bo` row, whatever this method is handed. The use case refuses first, in a sentence.
func (s *PhieuPhanAnhStore) SetPublicationStatus(ctx context.Context, tx *store.ScopedTx, id string,
	from, to domain.PublicationStatus) error {

	const stmt = `UPDATE phieu_phan_anh
		SET publication_status = $3, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND publication_status = $4 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, string(to), string(from))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: đặt trạng thái công khai: %w", err)
	}
	return doiMotDongPhieu(kq, "đặt trạng thái công khai")
}
