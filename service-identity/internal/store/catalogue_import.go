package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The two reads the Excel import of the residential-unit-type and task-bloc catalogues needs (ADR 0059
// §3), beside the write statements they share (danh_muc_ghi.go). The insert the import runs is the
// form's own (chenDanhMuc, via each store's Chen), so the import cannot write a row the form could not.
//
// `bang` IS INTERPOLATED AND MUST STAY A COMPILE-TIME CONSTANT — bangLoaiDonViDanCu or bangKhoiNhiemVu,
// never a request value. Same contract as docDanhMuc.

// ErrCatalogueSnapshotTooLarge — past maxCatalogueSnapshot rows (soft-deleted included) the data is not
// a catalogue; refused rather than planned against a truncated list, which would miss a taken code.
var ErrCatalogueSnapshotTooLarge = errors.New("danh_muc: số dòng (kể cả đã xoá) vượt trần ảnh chụp")

// maxCatalogueSnapshot counts SOFT-DELETED rows too, so it sits well above the 100-row live ceilings.
const maxCatalogueSnapshot = 5000

// lockCatalogueImport serialises the imports of ONE catalogue of ONE commune until the transaction
// ends. The unique key is the floor for the CODE; nothing in the schema is a floor for the LABEL, so
// two files carrying "Khu phố" imported at the same instant would both see it absent. Per commune and
// per table, released by COMMIT or ROLLBACK — there is no path that forgets it.
func lockCatalogueImport(ctx context.Context, tx *store.ScopedTx, bang string) error {
	stmt := `SELECT pg_advisory_xact_lock(hashtextextended('` + bang + `:nhap:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("%s: khoá lượt nhập: %w", bang, err)
	}
	return nil
}

// catalogueImportSnapshot reads EVERY row of one catalogue of this commune — soft-deleted rows included
// and flagged, because `UNIQUE (tenant_id, ma)` counts them — inside the caller's transaction.
func catalogueImportSnapshot(ctx context.Context, tx *store.ScopedTx, bang string) ([]domain.ExistingCatalogueEntry, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `ma, nhan, deleted_at IS NOT NULL`, bang, `ORDER BY ma LIMIT $2`, maxCatalogueSnapshot+1)
	if err != nil {
		return nil, fmt.Errorf("%s: đọc ảnh chụp để nhập: %w", bang, err)
	}
	defer rows.Close()
	out := make([]domain.ExistingCatalogueEntry, 0, 16)
	for rows.Next() {
		var e domain.ExistingCatalogueEntry
		if err := rows.Scan(&e.Code, &e.Label, &e.Deleted); err != nil {
			return nil, fmt.Errorf("%s: đọc dòng ảnh chụp: %w", bang, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: duyệt ảnh chụp: %w", bang, err)
	}
	if len(out) > maxCatalogueSnapshot {
		return nil, ErrCatalogueSnapshotTooLarge
	}
	return out, nil
}

// LockImport — see lockCatalogueImport.
func (s *LoaiDonViDanCuStore) LockImport(ctx context.Context, tx *store.ScopedTx) error {
	return lockCatalogueImport(ctx, tx, bangLoaiDonViDanCu)
}

// ImportSnapshot — see catalogueImportSnapshot.
func (s *LoaiDonViDanCuStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	return catalogueImportSnapshot(ctx, tx, bangLoaiDonViDanCu)
}

// LockImport — see lockCatalogueImport.
func (s *KhoiNhiemVuStore) LockImport(ctx context.Context, tx *store.ScopedTx) error {
	return lockCatalogueImport(ctx, tx, bangKhoiNhiemVu)
}

// ImportSnapshot — see catalogueImportSnapshot.
func (s *KhoiNhiemVuStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	return catalogueImportSnapshot(ctx, tx, bangKhoiNhiemVu)
}
