package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The reads the Excel import of the capital-plan-category catalogue needs (ADR 0059 §3), and the
// insert it runs — which IS the form's own statement (InsertCategory), so the import cannot write a row the form
// could not: `nguon` and `ma_nguon_re_nhanh` stay literals.

// ErrCatalogueSnapshotTooLarge — past maxCatalogueSnapshot rows (soft-deleted included) the data is not
// a catalogue; refused rather than planned against a truncated list, which would miss a taken code.
var ErrCatalogueSnapshotTooLarge = errors.New("hang_muc_ke_hoach_von: số dòng (kể cả đã xoá) vượt trần ảnh chụp")

// maxCatalogueSnapshot counts SOFT-DELETED rows too, so it sits well above the 200-row live ceiling.
const maxCatalogueSnapshot = 5000

// LockImport serialises the imports of this catalogue for ONE commune until the transaction ends. The
// unique key is the floor for the CODE; nothing in the schema is a floor for the LABEL, so two files
// carrying "Xây dựng mới" imported at the same instant would both see it absent. Released by COMMIT or
// ROLLBACK — there is no path that forgets it.
func (s *CapitalPlanCategoryStore) LockImport(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended('hang_muc_ke_hoach_von:nhap:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("hang_muc_ke_hoach_von: khoá lượt nhập: %w", err)
	}
	return nil
}

// ImportSnapshot reads EVERY row of this commune's catalogue — soft-deleted rows included and flagged,
// because `UNIQUE (tenant_id, ma)` counts them — inside the caller's transaction.
func (s *CapitalPlanCategoryStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `ma, nhan, deleted_at IS NOT NULL`, "hang_muc_ke_hoach_von", `ORDER BY ma LIMIT $2`, maxCatalogueSnapshot+1)
	if err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc ảnh chụp để nhập: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ExistingCatalogueEntry, 0, 16)
	for rows.Next() {
		var e domain.ExistingCatalogueEntry
		if err := rows.Scan(&e.Code, &e.Label, &e.Deleted); err != nil {
			return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc dòng ảnh chụp: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: duyệt ảnh chụp: %w", err)
	}
	if len(out) > maxCatalogueSnapshot {
		return nil, ErrCatalogueSnapshotTooLarge
	}
	return out, nil
}

// InsertImported runs the form's INSERT (InsertCategory) and turns the unique key refusing a code into
// ErrCodeTaken. That can only happen when a FORM took the code between the snapshot and the insert
// (the form does not take the import lock); the whole file rolls back and the handler answers 409
// `catalogue_changed`. The constraint is UNIQUE (tenant_id, ma) of migration 0003; on the hash
// partitions PostgreSQL names it `hang_muc_ke_hoach_von_pNN_tenant_id_ma_key` — the suffix is the same.
func (s *CapitalPlanCategoryStore) InsertImported(ctx context.Context, tx *store.ScopedTx, category domain.CapitalPlanCategory) error {
	err := s.InsertCategory(ctx, tx, category)
	if err != nil && strings.Contains(err.Error(), "tenant_id_ma_key") {
		return fmt.Errorf("hang_muc_ke_hoach_von: chèn khi nhập: %w", ErrCodeTaken)
	}
	return err
}
