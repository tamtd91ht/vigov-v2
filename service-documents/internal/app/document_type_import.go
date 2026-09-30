package app

// The use case behind `Nhập từ Excel` on the document-type catalogue (ADR 0059 §3): ALL OR NOTHING,
// create only, under `admin.lookup` — the shape of service-comms' map-asset-type import
// (service-comms/internal/app/map_asset_type_import.go).
//
//	PreviewDocumentTypeImport  plan the file against the catalogue — WRITES NOTHING, audits nothing
//	ImportDocumentTypes        the same plan inside the write transaction; any error = nothing
//	                           written; else every row and ONE audit entry for the batch
//
// THE RULES ARE domain.PlanDocumentTypeImport's (rule 9). This layer owns the transaction: the snapshot
// is read inside the transaction the inserts run in, and a failure anywhere — an insert, the entry, the
// unique key refusing a code a concurrent request just took — rolls the WHOLE file back.
//
// ONE ENTRY FOR THE BATCH, not one per row: one person performed one act ("imported this file"), and the
// delta lists every code, label and order created, so "when was `to-trinh` created" is still answerable
// from the trail. Nothing in a catalogue row is personal data (rule 3).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// ActionImportDocumentTypes is the trail's verb for one imported file — Vietnamese snake_case like the
// catalogue's other verbs (HanhViThemLoaiVanBan), because an inspection reads it.
const ActionImportDocumentTypes = "nhap_loai_van_ban"

// ErrNoImportActor — an import with no staff code. The trail must name who imported (rule 6, inv. 8).
var ErrNoImportActor = errors.New("danh_muc_loai_van_ban: thiếu người nhập")

// DocumentTypeImportRejected carries every error of a file that was refused. Nothing was written.
type DocumentTypeImportRejected struct {
	Errors []domain.DocumentTypeImportError
}

func (e *DocumentTypeImportRejected) Error() string {
	return fmt.Sprintf("loai_van_ban: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// DocumentTypeImportResult is what a run planned (preview) or created (import), in file order.
type DocumentTypeImportResult struct {
	Types  []domain.PlannedDocumentType
	Errors []domain.DocumentTypeImportError // preview only; import returns *DocumentTypeImportRejected
}

// errPreviewRollback ends the preview's transaction: a preview must not COMMIT anything, even by
// accident.
var errPreviewRollback = errors.New("loai_van_ban: xem trước — huỷ giao dịch")

// PreviewDocumentTypeImport plans the file and reports. The returned error is a system failure only; a
// file with errors is a SUCCESSFUL preview whose Errors is non-empty.
func (uc *DanhMucLoaiVanBan) PreviewDocumentTypeImport(ctx context.Context, rows []domain.DocumentTypeImportRow) (
	DocumentTypeImportResult, error) {

	var res DocumentTypeImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.kho.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Types, res.Errors = domain.PlanDocumentTypeImport(rows, existing, docstore.TranDanhMucLoaiVanBan)
		return errPreviewRollback
	})
	if err != nil && !errors.Is(err, errPreviewRollback) {
		return DocumentTypeImportResult{}, boc(ctx, "xem trước nhập Excel", err)
	}
	return res, nil
}

// ImportDocumentTypes writes the file, or nothing.
func (uc *DanhMucLoaiVanBan) ImportDocumentTypes(ctx context.Context, rows []domain.DocumentTypeImportRow,
	actor audit.Actor) (DocumentTypeImportResult, error) {

	if actor.ID == "" {
		return DocumentTypeImportResult{}, ErrNoImportActor
	}
	var res DocumentTypeImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = DocumentTypeImportResult{}
		existing, err := uc.kho.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		plan, errs := domain.PlanDocumentTypeImport(rows, existing, docstore.TranDanhMucLoaiVanBan)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &DocumentTypeImportRejected{Errors: errs}
		}

		created := make([]map[string]any, 0, len(plan))
		for i := range plan {
			p := &plan[i]
			if p.ID, err = uc.sinhID(); err != nil {
				return fmt.Errorf("loai_van_ban: sinh id: %w", err)
			}
			// The form's INSERT: `nguon` and `ma_nguon_re_nhanh` are literals there. LaMacDinh false:
			// an import creates, it never moves the commune's default (domain/document_type_import.go).
			if err := uc.kho.Chen(ctx, tx, domain.LoaiVanBan{
				ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true, LaMacDinh: false,
			}); err != nil {
				return err
			}
			created = append(created, map[string]any{"dong": p.Row, "ma": p.Code, "nhan": p.Label, "thu_tu": p.Order})
		}

		delta, err := json.Marshal(map[string]any{"nguon": "nhap_excel", "so_dong": len(plan), "sau": created})
		if err != nil {
			return fmt.Errorf("loai_van_ban: mã hoá delta: %w", err)
		}
		at := time.Now().UTC()
		// SAME TRANSACTION AS EVERY INSERT (rule 6, invariant 3); TenantID filled from the transaction.
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionImportDocumentTypes,
			Subject: "loai-van-ban/nhap-excel/" + at.Format("2006-01-02"),
			At:      at,
			Delta:   delta,
		}); err != nil {
			return err
		}
		res.Types = plan
		return nil
	})
	if err != nil {
		var rej *DocumentTypeImportRejected
		if errors.As(err, &rej) {
			return DocumentTypeImportResult{}, rej
		}
		return DocumentTypeImportResult{}, boc(ctx, "nhập Excel", err)
	}
	return res, nil
}
