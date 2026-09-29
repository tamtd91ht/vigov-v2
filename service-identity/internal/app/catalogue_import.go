package app

// The use case behind "⬆ Nhập từ Excel" on the residential-unit-type and task-bloc catalogues
// (14-cau-hinh.md §5; user decision 2026-09-29, ADR 0059 §3): create only, ALL OR NOTHING, under
// `admin.lookup` — the key the catalogues' create routes already declare.
//
//	Preview  plan the file against the catalogue and report — WRITES NOTHING, audits nothing
//	Import   the same plan under the import lock; any error = nothing written; else every row, one
//	         audit entry per row, all in ONE transaction (rule 6, invariant 3)
//
// THE ROW RULES ARE domain.PlanCatalogueImport's and are not restated here (rule 9). This layer owns
// the transaction: the lock and the snapshot the plan is made against are taken inside the transaction
// the inserts run in, and a failure anywhere — an insert, an audit entry, the unique key refusing a code
// a concurrent form just took — rolls the WHOLE file back.
//
// ONE ENTRY PER ROW, SAME VERB AS THE FORM (`them_loai_don_vi_dan_cu` / `them_khoi_nhiem_vu`), each
// carrying the batch — the convention this service's residential-unit and org-chart imports set, so
// "every type ever added" stays one query on the trail whichever way it was added. Nothing in a
// catalogue row is personal data (rule 3).
//
// ONE GENERIC USE CASE, TWO INSTANTIATIONS, for the reason danh_muc_ghi.go gives: CatalogueImporter
// [domain.LoaiDonViDanCu] cannot be handed the task-bloc store.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// CatalogueImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *idstore.LoaiDonViDanCuStore satisfies CatalogueImportRepo[domain.LoaiDonViDanCu];
// *idstore.KhoiNhiemVuStore satisfies CatalogueImportRepo[domain.KhoiNhiemVu]. Chen is the statement
// the create form runs.
type CatalogueImportRepo[T any] interface {
	LockImport(ctx context.Context, tx *store.ScopedTx) error
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error)
	Chen(ctx context.Context, tx *store.ScopedTx, row T) error
}

// CatalogueImportRejected carries every error of a file that was refused. Nothing was written.
type CatalogueImportRejected struct {
	Errors []domain.CatalogueImportError
}

func (e *CatalogueImportRejected) Error() string {
	return fmt.Sprintf("danh_muc: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// CatalogueImportResult is what a run planned (Preview) or created (Import), in file order.
type CatalogueImportResult struct {
	Entries []domain.PlannedCatalogueEntry
	Errors  []domain.CatalogueImportError // Preview only; Import returns *CatalogueImportRejected
}

// catalogueImportSource marks, in the delta, that a row came from a file.
const catalogueImportSource = "nhap_excel"

var errCataloguePreviewRollback = errors.New("danh_muc: xem trước — huỷ giao dịch")

// CatalogueImporter owns the import of ONE catalogue.
type CatalogueImporter[T any] struct {
	db   *store.DB
	repo CatalogueImportRepo[T]

	table   string // for wrapping errors — never a request value
	action  string // the form's create verb
	ceiling int
	build   func(domain.PlannedCatalogueEntry) T

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

// NewResidentialUnitTypeImporter builds the import of the residential-unit-type catalogue.
func NewResidentialUnitTypeImporter(db *store.DB, repo CatalogueImportRepo[domain.LoaiDonViDanCu]) *CatalogueImporter[domain.LoaiDonViDanCu] {
	return &CatalogueImporter[domain.LoaiDonViDanCu]{
		db: db, repo: repo, newID: ulid.Moi,
		table: "loai_don_vi_dan_cu", action: HanhViThemLoaiDonViDanCu, ceiling: idstore.TranDanhMucLoaiDonViDanCu,
		build: func(p domain.PlannedCatalogueEntry) domain.LoaiDonViDanCu {
			return domain.LoaiDonViDanCu{ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, Nguon: domain.NguonDonVi}
		},
	}
}

// NewTaskBlocImporter builds the import of the task-bloc catalogue.
func NewTaskBlocImporter(db *store.DB, repo CatalogueImportRepo[domain.KhoiNhiemVu]) *CatalogueImporter[domain.KhoiNhiemVu] {
	return &CatalogueImporter[domain.KhoiNhiemVu]{
		db: db, repo: repo, newID: ulid.Moi,
		table: "khoi_nhiem_vu", action: HanhViThemKhoiNhiemVu, ceiling: idstore.TranDanhMucKhoiNhiemVu,
		build: func(p domain.PlannedCatalogueEntry) domain.KhoiNhiemVu {
			return domain.KhoiNhiemVu{ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, Nguon: domain.NguonDonVi}
		},
	}
}

// Preview plans the file and reports. The returned error is a system failure only; a file with errors
// is a SUCCESSFUL preview whose Errors is non-empty. The transaction always rolls back.
func (uc *CatalogueImporter[T]) Preview(ctx context.Context, rows []domain.CatalogueImportRow) (CatalogueImportResult, error) {
	var res CatalogueImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Entries, res.Errors = domain.PlanCatalogueImport(rows, existing, uc.ceiling)
		return errCataloguePreviewRollback
	})
	if err != nil && !errors.Is(err, errCataloguePreviewRollback) {
		return CatalogueImportResult{}, bocDanhMuc(ctx, uc.table, "xem trước nhập Excel", err)
	}
	return res, nil
}

// Import writes the file, or nothing. Every entry names the row's CODE as its subject and carries
// `lo_nhap` (one ULID per file), `dong` and `so_dong`.
func (uc *CatalogueImporter[T]) Import(ctx context.Context, rows []domain.CatalogueImportRow, actor NguoiThucHien) (CatalogueImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return CatalogueImportResult{}, err
	}
	batch, err := uc.newID()
	if err != nil {
		return CatalogueImportResult{}, fmt.Errorf("%s: sinh mã lô nhập: %w", uc.table, err)
	}
	var res CatalogueImportResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = CatalogueImportResult{}
		if err := uc.repo.LockImport(ctx, tx); err != nil {
			return err
		}
		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		plan, errs := domain.PlanCatalogueImport(rows, existing, uc.ceiling)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &CatalogueImportRejected{Errors: errs}
		}
		for i := range plan {
			p := &plan[i]
			if p.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("%s: sinh id: %w", uc.table, err)
			}
			if err := uc.repo.Chen(ctx, tx, uc.build(*p)); err != nil {
				return err
			}
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   actor.Vet,
				Action:  uc.action,
				Subject: p.Code,
				Delta: deltaDanhMuc(map[string]any{
					"sau": vetDanhMuc(mucDanhMuc{Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true,
						Nguon: domain.NguonDonVi}),
					// Top-level `nguon` is the PROVENANCE of the act (the residential-unit import's key);
					// `sau.nguon` is the row's tier. Two levels, two meanings.
					"nguon":   catalogueImportSource,
					"lo_nhap": batch,
					"dong":    p.Row,
					"so_dong": len(plan),
				}),
			}); err != nil {
				return err
			}
		}
		res.Entries = plan
		return nil
	})
	if err != nil {
		var rej *CatalogueImportRejected
		if errors.As(err, &rej) {
			return CatalogueImportResult{}, rej
		}
		return CatalogueImportResult{}, bocDanhMuc(ctx, uc.table, "nhập Excel", err)
	}
	return res, nil
}
