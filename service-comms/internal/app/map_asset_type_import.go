package app

// The use case behind `Nhập từ Excel` on the map-asset-type catalogue (ADR 0059): ALL OR NOTHING,
// create only, under `admin.lookup` — the org-unit import's shape
// (service-identity/internal/app/org_unit_import.go).
//
//	PreviewMapAssetTypeImport  plan the file against the catalogue — WRITES NOTHING, audits nothing
//	ImportMapAssetTypes        the same plan inside the write transaction; any error = nothing
//	                           written; else every row and ONE audit entry for the batch
//
// THE RULES ARE domain.PlanMapAssetTypeImport's (rule 9). This layer owns the transaction: the
// snapshot is read inside the transaction the inserts run in, and a failure anywhere — an insert, the
// entry, the unique key refusing a code a concurrent request just took — rolls the WHOLE file back.
//
// ONE ENTRY FOR THE BATCH, not one per row: one person performed one act ("imported this file"), and
// the delta lists every code, label and order created, so "when was `doanh-nghiep` created" is still
// answerable from the trail. Nothing in a catalogue row is personal data (rule 3).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ActionImportMapAssetTypes is the trail's verb for one imported file.
const ActionImportMapAssetTypes = "nhap_loai_tai_nguyen_ban_do"

// MapAssetTypeImportRejected carries every error of a file that was refused. Nothing was written.
type MapAssetTypeImportRejected struct {
	Errors []domain.MapAssetTypeImportError
}

func (e *MapAssetTypeImportRejected) Error() string {
	return fmt.Sprintf("loai_tai_nguyen_ban_do: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// MapAssetTypeImportResult is what a run planned (preview) or created (import), in file order.
type MapAssetTypeImportResult struct {
	Types  []domain.PlannedMapAssetType
	Errors []domain.MapAssetTypeImportError // preview only; import returns *MapAssetTypeImportRejected
}

// errPreviewRollback ends the preview's transaction: a preview must not COMMIT anything, even by
// accident.
var errPreviewRollback = errors.New("loai_tai_nguyen_ban_do: xem trước — huỷ giao dịch")

// PreviewMapAssetTypeImport plans the file and reports. The returned error is a system failure only;
// a file with errors is a SUCCESSFUL preview whose Errors is non-empty.
func (uc *DanhMucLoaiTaiNguyen) PreviewMapAssetTypeImport(ctx context.Context, rows []domain.MapAssetTypeImportRow) (
	MapAssetTypeImportResult, error) {

	var res MapAssetTypeImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		existing, err := uc.kho.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		res.Types, res.Errors = domain.PlanMapAssetTypeImport(rows, existing, docstore.TranDanhMucLoaiTaiNguyen)
		return errPreviewRollback
	})
	if err != nil && !errors.Is(err, errPreviewRollback) {
		return MapAssetTypeImportResult{}, boc(ctx, "xem trước nhập Excel", err)
	}
	return res, nil
}

// ImportMapAssetTypes writes the file, or nothing.
func (uc *DanhMucLoaiTaiNguyen) ImportMapAssetTypes(ctx context.Context, rows []domain.MapAssetTypeImportRow,
	actor audit.Actor) (MapAssetTypeImportResult, error) {

	if actor.ID == "" {
		return MapAssetTypeImportResult{}, ErrNoActor
	}
	var res MapAssetTypeImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = MapAssetTypeImportResult{}
		existing, err := uc.kho.ImportSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		plan, errs := domain.PlanMapAssetTypeImport(rows, existing, docstore.TranDanhMucLoaiTaiNguyen)
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &MapAssetTypeImportRejected{Errors: errs}
		}

		created := make([]map[string]any, 0, len(plan))
		for i := range plan {
			p := &plan[i]
			if p.ID, err = uc.sinhID(); err != nil {
				return fmt.Errorf("loai_tai_nguyen_ban_do: sinh id: %w", err)
			}
			// The form's INSERT: `nguon` and `ma_nguon_re_nhanh` are literals there, so an imported row
			// is a tier-1 row of the commune, exactly like a typed one.
			if err := uc.kho.Chen(ctx, tx, domain.LoaiTaiNguyenBanDo{
				ID: p.ID, Ma: p.Code, Nhan: p.Label, ThuTu: p.Order, DangDung: true,
			}); err != nil {
				return err
			}
			created = append(created, map[string]any{"dong": p.Row, "ma": p.Code, "nhan": p.Label, "thu_tu": p.Order})
		}

		delta, err := json.Marshal(map[string]any{"nguon": "nhap_excel", "so_dong": len(plan), "sau": created})
		if err != nil {
			return fmt.Errorf("loai_tai_nguyen_ban_do: mã hoá delta: %w", err)
		}
		at := time.Now().UTC()
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionImportMapAssetTypes,
			Subject: "loai-tai-nguyen-ban-do/nhap-excel/" + at.Format("2006-01-02"),
			At:      at,
			Delta:   delta,
		}); err != nil {
			return err
		}
		res.Types = plan
		return nil
	})
	if err != nil {
		var rej *MapAssetTypeImportRejected
		if errors.As(err, &rej) {
			return MapAssetTypeImportResult{}, rej
		}
		return MapAssetTypeImportResult{}, boc(ctx, "nhập Excel", err)
	}
	return res, nil
}
