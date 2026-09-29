package app

// The use case behind "⬆ Nhập từ Excel" on the task-type and task-priority catalogues
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
// ONE ENTRY PER ROW, SAME VERB AS THE FORM (`them_loai_nhiem_vu` / `them_muc_uu_tien_nhiem_vu`), each
// carrying the batch — service-identity's catalogue-import convention (fbbae7b), so "every type ever
// added" stays one query on the trail whichever way it was added. Nothing in a catalogue row is
// personal data (rule 3).
//
// ONE GENERIC USE CASE, TWO INSTANTIATIONS: the two stores are separate types on purpose
// (store.MucUuTienNhiemVuStore), and CatalogueImporter[domain.LoaiNhiemVu] cannot be handed the
// priority store.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CatalogueImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *docstore.LoaiNhiemVuStore satisfies CatalogueImportRepo[domain.LoaiNhiemVu];
// *docstore.MucUuTienNhiemVuStore satisfies CatalogueImportRepo[domain.MucUuTienNhiemVu].
// InsertImported is the statement the create form runs.
type CatalogueImportRepo[T any] interface {
	LockImport(ctx context.Context, tx *store.ScopedTx) error
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error)
	InsertImported(ctx context.Context, tx *store.ScopedTx, row T) error
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
	Batch   string                        // Import only: the `lo_nhap` every entry of this file carries
}

// catalogueImportSource marks, in the delta, that a row came from a file.
const catalogueImportSource = "nhap_excel"

// ErrCatalogueImportNoActor — the actor carries no staff code. Refused before any transaction opens:
// rule 6, invariant 8 has no fallback.
var ErrCatalogueImportNoActor = errors.New("danh_muc: nhập Excel không có mã cán bộ thực hiện")

var errCataloguePreviewRollback = errors.New("danh_muc: xem trước — huỷ giao dịch")

// CatalogueImporter owns the import of ONE catalogue.
type CatalogueImporter[T any] struct {
	db   *store.DB
	repo CatalogueImportRepo[T]

	table   string // for wrapping errors — never a request value
	action  string // the form's create verb
	ceiling int
	layout  domain.CatalogueImportLayout
	build   func(domain.PlannedCatalogueEntry) T
	summary func(T) map[string]any // the form's audit view of the row (tomTat…)

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

// NewTaskTypeImporter builds the import of the task-type catalogue: Tên hiển thị · Mã · Thứ tự.
func NewTaskTypeImporter(db *store.DB, repo CatalogueImportRepo[domain.LoaiNhiemVu]) *CatalogueImporter[domain.LoaiNhiemVu] {
	return &CatalogueImporter[domain.LoaiNhiemVu]{
		db: db, repo: repo, newID: ulid.Moi,
		table: "loai_nhiem_vu", action: HanhViThemLoaiNhiemVu, ceiling: docstore.TranDanhMucLoaiNhiemVu,
		layout: domain.CatalogueOrderFromColumn,
		build: func(p domain.PlannedCatalogueEntry) domain.LoaiNhiemVu {
			return domain.LoaiNhiemVu{ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, Nguon: domain.NguonDonVi}
		},
		summary: tomTatLoaiNhiemVu,
	}
}

// NewTaskPriorityImporter builds the import of the priority scale: Tên hiển thị · Mã, ranked by
// position AFTER every existing level (domain.CatalogueOrderFromPosition).
func NewTaskPriorityImporter(db *store.DB, repo CatalogueImportRepo[domain.MucUuTienNhiemVu]) *CatalogueImporter[domain.MucUuTienNhiemVu] {
	return &CatalogueImporter[domain.MucUuTienNhiemVu]{
		db: db, repo: repo, newID: ulid.Moi,
		table: "muc_uu_tien_nhiem_vu", action: HanhViThemMucUuTien, ceiling: docstore.TranDanhMucMucUuTien,
		layout: domain.CatalogueOrderFromPosition,
		build: func(p domain.PlannedCatalogueEntry) domain.MucUuTienNhiemVu {
			return domain.MucUuTienNhiemVu{ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, Nguon: domain.NguonDonVi}
		},
		summary: tomTatMucUuTien,
	}
}

// Layout is the file layout of this catalogue — what the HTTP layer reads the sheet with.
func (uc *CatalogueImporter[T]) Layout() domain.CatalogueImportLayout { return uc.layout }

// Preview plans the file and reports. The returned error is a system failure only; a file with errors
// is a SUCCESSFUL preview whose Errors is non-empty. The transaction always rolls back.
func (uc *CatalogueImporter[T]) Preview(ctx context.Context, rows []domain.CatalogueImportRow) (CatalogueImportResult, error) {
	var res CatalogueImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.repo.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Entries, res.Errors = domain.PlanCatalogueImport(rows, existing, uc.ceiling, uc.layout)
		return errCataloguePreviewRollback
	})
	if err != nil && !errors.Is(err, errCataloguePreviewRollback) {
		return CatalogueImportResult{}, boc(ctx, uc.table+" xem trước nhập Excel", err)
	}
	return res, nil
}

// Import writes the file, or nothing. Every entry names the row's CODE as its subject and carries
// `lo_nhap` (one ULID per file), `dong` and `so_dong`.
func (uc *CatalogueImporter[T]) Import(ctx context.Context, rows []domain.CatalogueImportRow, actor audit.Actor) (CatalogueImportResult, error) {
	if actor.ID == "" {
		return CatalogueImportResult{}, ErrCatalogueImportNoActor
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
		plan, errs := domain.PlanCatalogueImport(rows, existing, uc.ceiling, uc.layout)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &CatalogueImportRejected{Errors: errs}
		}
		for i := range plan {
			p := &plan[i]
			if p.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("%s: sinh id: %w", uc.table, err)
			}
			row := uc.build(*p)
			if err := uc.repo.InsertImported(ctx, tx, row); err != nil {
				return err
			}
			delta, err := json.Marshal(map[string]any{
				"sau": uc.summary(row),
				// Top-level `nguon` is the PROVENANCE of the act; `sau.nguon` is the row's tier. Two
				// levels, two meanings — identity's convention.
				"nguon":   catalogueImportSource,
				"lo_nhap": batch,
				"dong":    p.Row,
				"so_dong": len(plan),
			})
			if err != nil {
				return fmt.Errorf("%s: mã hoá delta: %w", uc.table, err)
			}
			// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3); TenantID filled from the transaction.
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   actor,
				Action:  uc.action,
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
		return CatalogueImportResult{}, boc(ctx, uc.table+" nhập Excel", err)
	}
	return res, nil
}
