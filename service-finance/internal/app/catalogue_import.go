package app

// The use case behind "⬆ Nhập từ Excel" on the capital-plan-category catalogue (14-cau-hinh.md §5;
// user decision 2026-09-29, ADR 0059 §3): create only, ALL OR NOTHING, under `admin.lookup` — the key
// the catalogue's create route already declares.
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
// ONE ENTRY PER ROW, SAME VERB AS THE FORM (`them_hang_muc_ke_hoach_von`), each carrying the batch —
// service-identity's catalogue-import convention (fbbae7b), so "every category ever added" stays one
// query on the trail whichever way it was added. Nothing in a catalogue row is personal data (rule 3).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// CatalogueImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *fistore.HangMucKeHoachVonStore satisfies it; InsertImported is the statement the create form runs.
type CatalogueImportRepo interface {
	LockImport(ctx context.Context, tx *store.ScopedTx) error
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error)
	InsertImported(ctx context.Context, tx *store.ScopedTx, hm domain.HangMucKeHoachVon) error
}

// CatalogueImportRejected carries every error of a file that was refused. Nothing was written.
type CatalogueImportRejected struct {
	Errors []domain.CatalogueImportError
}

func (e *CatalogueImportRejected) Error() string {
	return fmt.Sprintf("hang_muc_ke_hoach_von: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// CatalogueImportResult is what a run planned (Preview) or created (Import), in file order.
type CatalogueImportResult struct {
	Entries []domain.PlannedCatalogueEntry
	Errors  []domain.CatalogueImportError // Preview only; Import returns *CatalogueImportRejected
	Batch   string                        // Import only: the `lo_nhap` every entry of this file carries
}

// catalogueImportSource marks, in the delta, that a row came from a file.
const catalogueImportSource = "nhap_excel"

// ErrCatalogueImportNoActor — the actor carries no staff code. Refused before any transaction opens:
// rule 6, invariant 8 has no fallback.
var ErrCatalogueImportNoActor = errors.New("hang_muc_ke_hoach_von: nhập Excel không có mã cán bộ thực hiện")

var errCataloguePreviewRollback = errors.New("hang_muc_ke_hoach_von: xem trước — huỷ giao dịch")

// CapitalPlanCategoryImporter owns the import of the capital-plan-category catalogue.
type CapitalPlanCategoryImporter struct {
	db   *store.DB
	repo CatalogueImportRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

// NewCapitalPlanCategoryImporter builds the import. Give it the SAME store as the create form.
func NewCapitalPlanCategoryImporter(db *store.DB, repo CatalogueImportRepo) *CapitalPlanCategoryImporter {
	return &CapitalPlanCategoryImporter{db: db, repo: repo, newID: ulid.Moi}
}

// Preview plans the file and reports. The returned error is a system failure only; a file with errors
// is a SUCCESSFUL preview whose Errors is non-empty. The transaction always rolls back.
func (uc *CapitalPlanCategoryImporter) Preview(ctx context.Context, rows []domain.CatalogueImportRow) (CatalogueImportResult, error) {
	var res CatalogueImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Entries, res.Errors = domain.PlanCatalogueImport(rows, existing, fistore.TranDanhMucHangMuc)
		return errCataloguePreviewRollback
	})
	if err != nil && !errors.Is(err, errCataloguePreviewRollback) {
		return CatalogueImportResult{}, boc(ctx, "xem trước nhập Excel", err)
	}
	return res, nil
}

// Import writes the file, or nothing. Every entry names the row's CODE as its subject and carries
// `lo_nhap` (one ULID per file), `dong` and `so_dong`.
func (uc *CapitalPlanCategoryImporter) Import(ctx context.Context, rows []domain.CatalogueImportRow, actor audit.Actor) (CatalogueImportResult, error) {
	if actor.ID == "" {
		return CatalogueImportResult{}, ErrCatalogueImportNoActor
	}
	batch, err := uc.newID()
	if err != nil {
		return CatalogueImportResult{}, fmt.Errorf("hang_muc_ke_hoach_von: sinh mã lô nhập: %w", err)
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
		plan, errs := domain.PlanCatalogueImport(rows, existing, fistore.TranDanhMucHangMuc)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &CatalogueImportRejected{Errors: errs}
		}
		for i := range plan {
			p := &plan[i]
			if p.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("hang_muc_ke_hoach_von: sinh id: %w", err)
			}
			hm := domain.HangMucKeHoachVon{ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, Nguon: domain.NguonDonVi}
			if err := uc.repo.InsertImported(ctx, tx, hm); err != nil {
				return err
			}
			delta, err := json.Marshal(map[string]any{
				"sau": tomTatHangMuc(hm),
				// Top-level `nguon` is the PROVENANCE of the act; `sau.nguon` is the row's tier. Two
				// levels, two meanings — identity's convention.
				"nguon":   catalogueImportSource,
				"lo_nhap": batch,
				"dong":    p.Row,
				"so_dong": len(plan),
			})
			if err != nil {
				return fmt.Errorf("hang_muc_ke_hoach_von: mã hoá delta: %w", err)
			}
			// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3); TenantID filled from the transaction.
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   actor,
				Action:  HanhViThemHangMuc,
				Subject: p.Code, // the business code, never the internal id
				Delta:   delta,
			}); err != nil {
				return err
			}
		}
		res.Entries, res.Batch = plan, batch
		return nil
	})
	if err != nil {
		var rej *CatalogueImportRejected
		if errors.As(err, &rej) {
			return CatalogueImportResult{}, rej
		}
		return CatalogueImportResult{}, boc(ctx, "nhập Excel", err)
	}
	return res, nil
}
