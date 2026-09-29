package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The reads the Excel import of the two task catalogues needs (ADR 0059 §3), and the insert it runs —
// which IS the form's own statement (each store's Chen), so the import cannot write a row the form
// could not: `nguon` and `ma_nguon_re_nhanh` stay literals.
//
// `table` IS INTERPOLATED AND MUST STAY A COMPILE-TIME CONSTANT — tableTaskType or tableTaskPriority,
// never a request value (the reason MucUuTienNhiemVuStore's comment gives for keeping the two stores
// apart). The helpers are unexported; only the per-store methods below reach them.

const (
	tableTaskType     = "loai_nhiem_vu"
	tableTaskPriority = "muc_uu_tien_nhiem_vu"
)

// ErrCatalogueSnapshotTooLarge — past maxCatalogueSnapshot rows (soft-deleted included) the data is not
// a catalogue; refused rather than planned against a truncated list, which would miss a taken code.
var ErrCatalogueSnapshotTooLarge = errors.New("danh_muc: số dòng (kể cả đã xoá) vượt trần ảnh chụp")

// maxCatalogueSnapshot counts SOFT-DELETED rows too, so it sits well above the 100 / 50 live ceilings.
const maxCatalogueSnapshot = 5000

// lockCatalogueImport serialises the imports of ONE catalogue of ONE commune until the transaction
// ends. The unique key is the floor for the CODE; nothing in the schema is a floor for the LABEL, so
// two files carrying "Theo văn bản" imported at the same instant would both see it absent. Per commune
// and per table, released by COMMIT or ROLLBACK — there is no path that forgets it.
func lockCatalogueImport(ctx context.Context, tx *store.ScopedTx, table string) error {
	stmt := `SELECT pg_advisory_xact_lock(hashtextextended('` + table + `:nhap:' || $1, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("%s: khoá lượt nhập: %w", table, err)
	}
	return nil
}

// catalogueImportSnapshot reads EVERY row of one catalogue of this commune — soft-deleted rows included
// and flagged, because `UNIQUE (tenant_id, ma)` counts them — inside the caller's transaction.
// `thu_tu` is read because the priority scale's import ranks new levels after the last live one.
func catalogueImportSnapshot(ctx context.Context, tx *store.ScopedTx, table string) ([]domain.ExistingCatalogueEntry, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, `ma, nhan, thu_tu, deleted_at IS NOT NULL`, table, `ORDER BY ma LIMIT $2`, maxCatalogueSnapshot+1)
	if err != nil {
		return nil, fmt.Errorf("%s: đọc ảnh chụp để nhập: %w", table, err)
	}
	defer rows.Close()
	out := make([]domain.ExistingCatalogueEntry, 0, 16)
	for rows.Next() {
		var e domain.ExistingCatalogueEntry
		if err := rows.Scan(&e.Code, &e.Label, &e.Order, &e.Deleted); err != nil {
			return nil, fmt.Errorf("%s: đọc dòng ảnh chụp: %w", table, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: duyệt ảnh chụp: %w", table, err)
	}
	if len(out) > maxCatalogueSnapshot {
		return nil, ErrCatalogueSnapshotTooLarge
	}
	return out, nil
}

// translateImportInsert turns the unique key refusing a code into ErrMaDaTonTai. It can only happen
// when a FORM took the code between the snapshot and the insert (the forms do not take the import
// lock); the whole file rolls back and the handler answers 409 `catalogue_changed`. The constraint is
// UNIQUE (tenant_id, ma) of migration 0003; on the hash partitions PostgreSQL names it
// `<table>_pNN_tenant_id_ma_key`, on the parent `<table>_tenant_id_ma_key` — both end the same way.
func translateImportInsert(table string, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "tenant_id_ma_key") {
		return fmt.Errorf("%s: chèn khi nhập: %w", table, ErrMaDaTonTai)
	}
	return err
}

// LockImport — see lockCatalogueImport.
func (s *LoaiNhiemVuStore) LockImport(ctx context.Context, tx *store.ScopedTx) error {
	return lockCatalogueImport(ctx, tx, tableTaskType)
}

// ImportSnapshot — see catalogueImportSnapshot.
func (s *LoaiNhiemVuStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	return catalogueImportSnapshot(ctx, tx, tableTaskType)
}

// InsertImported runs the form's INSERT (Chen) and translates a taken code — see translateImportInsert.
func (s *LoaiNhiemVuStore) InsertImported(ctx context.Context, tx *store.ScopedTx, row domain.LoaiNhiemVu) error {
	return translateImportInsert(tableTaskType, s.Chen(ctx, tx, row))
}

// LockImport — see lockCatalogueImport.
func (s *MucUuTienNhiemVuStore) LockImport(ctx context.Context, tx *store.ScopedTx) error {
	return lockCatalogueImport(ctx, tx, tableTaskPriority)
}

// ImportSnapshot — see catalogueImportSnapshot.
func (s *MucUuTienNhiemVuStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	return catalogueImportSnapshot(ctx, tx, tableTaskPriority)
}

// InsertImported runs the form's INSERT (Chen) and translates a taken code — see translateImportInsert.
func (s *MucUuTienNhiemVuStore) InsertImported(ctx context.Context, tx *store.ScopedTx, row domain.MucUuTienNhiemVu) error {
	return translateImportInsert(tableTaskPriority, s.Chen(ctx, tx, row))
}
