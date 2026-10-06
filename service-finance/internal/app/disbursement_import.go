package app

// The use case behind "Nhập lần giải ngân từ Excel" (docs/ui-ux/06-giai-ngan.md §10, §13 rule 7):
// ALL OR NOTHING, under `budget.update` — the key POST /api/v1/disbursements declares.
//
//	Preview  plan the file against the commune's projects and sources — WRITES NOTHING, audits nothing
//	Import   the same plan, then every row through the FORM'S OWN create path (newVoucher +
//	         insertNewVoucher), one audit entry per row, all in ONE transaction (rule 6, invariant 3).
//	         Any error anywhere rolls the whole file back.
//
// THE ROW RULES ARE domain.PlanVoucherImport's and are not restated here (rule 9). What this layer adds
// is the guarantee that the plan cannot drift from the form: after the plan passes, each row is written
// by the function POST /api/v1/disbursements runs, which re-checks the shape, re-reads the project
// under its share lock, re-asks the catalogue and re-applies CheckVoucherSource. A rule added to the
// form tomorrow and forgotten in the planner refuses the import at write time — as a row error naming
// the row, never as a half-written file.
//
// ONE ENTRY PER ROW, SAME VERB AS THE FORM (`them_chung_tu_giai_ngan`), subject = the project's code,
// each carrying the batch (`lo_nhap`), the row (`dong`) and the row count (`so_dong`) beside `nguon:
// nhap_excel` — the catalogue import's convention, so "every voucher ever entered" stays one query on
// the trail however it was entered.
//
// NO ADVISORY LOCK, unlike the catalogue import: there is no uniqueness for a concurrent file to break.
// A double submit of one file is stopped by the route's idem.Required(DongKhiHong); uploading the same
// file AGAIN as a new request writes its vouchers again. Whether that should be refused is the
// customer's call (two identical payments are legitimate), so it is not decided here.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// VoucherImportRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *fistore.ChungTuGiaiNganStore satisfies it; the last three are the form's own create statements.
type VoucherImportRepo interface {
	// ProjectsForVoucherImport reads the LIVE projects of this commune among `codes`, each with its
	// allocated sources, holding every project row FOR SHARE until the transaction ends.
	ProjectsForVoucherImport(ctx context.Context, tx *store.ScopedTx, codes []string) ([]domain.VoucherImportProject, error)
	// SourcesForVoucherImport reads the commune's LIVE funding source catalogue.
	SourcesForVoucherImport(ctx context.Context, tx *store.ScopedTx) ([]domain.VoucherImportSource, error)
	voucherInserter
}

// VoucherImportRejected carries every error of a file that was refused. Nothing was written.
type VoucherImportRejected struct {
	Errors []domain.VoucherImportError
}

func (e *VoucherImportRejected) Error() string {
	return fmt.Sprintf("chung_tu_giai_ngan: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// VoucherImportResult is what a run planned (Preview) or wrote (Import).
type VoucherImportResult struct {
	RowCount int
	Total    domain.Dong
	Errors   []domain.VoucherImportError // Preview only; Import returns *VoucherImportRejected
	Batch    string                      // Import only: the `lo_nhap` every entry of this file carries
	Created  []ImportedVoucher           // Import only, in file order
}

// ImportedVoucher is one voucher an import wrote.
type ImportedVoucher struct {
	Row         int
	ID          string
	ProjectCode string
}

// voucherImportSource marks, in the delta, that a voucher came from a file.
const voucherImportSource = "nhap_excel"

// ErrVoucherImportNoActor — the actor carries no staff code. Refused before any transaction opens:
// rule 6, invariant 8 has no fallback.
var ErrVoucherImportNoActor = errors.New("chung_tu_giai_ngan: nhập Excel không có mã cán bộ thực hiện")

var errVoucherPreviewRollback = errors.New("chung_tu_giai_ngan: xem trước — huỷ giao dịch")

// DisbursementImporter owns the Excel import of one commune's disbursement vouchers.
type DisbursementImporter struct {
	db   *store.DB
	repo VoucherImportRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

// NewDisbursementImporter builds the import. Give it the SAME store as the voucher form.
func NewDisbursementImporter(db *store.DB, repo VoucherImportRepo) *DisbursementImporter {
	return &DisbursementImporter{db: db, repo: repo, newID: ulid.Moi}
}

// plan reads what the rows name and plans them, inside tx.
func (uc *DisbursementImporter) plan(ctx context.Context, tx *store.ScopedTx,
	rows []domain.VoucherImportRow) (domain.VoucherImportPlan, []domain.VoucherImportError, error) {

	// Too many rows is a file error the planner states; reading hundreds of projects first would only
	// hold their locks for nothing.
	var projects map[string]domain.VoucherImportProject
	var sources []domain.VoucherImportSource
	if len(rows) > 0 && len(rows) <= domain.MaxVoucherImportRows {
		list, err := uc.repo.ProjectsForVoucherImport(ctx, tx, domain.VoucherImportProjectCodes(rows))
		if err != nil {
			return domain.VoucherImportPlan{}, nil, err
		}
		projects = make(map[string]domain.VoucherImportProject, len(list))
		for _, p := range list {
			projects[p.Code] = p
		}
		if sources, err = uc.repo.SourcesForVoucherImport(ctx, tx); err != nil {
			return domain.VoucherImportPlan{}, nil, err
		}
	}
	plan, errs := domain.PlanVoucherImport(rows, projects, sources)
	return plan, errs, nil
}

// Preview plans the file and reports. The returned error is a system failure only; a file with errors
// is a SUCCESSFUL preview whose Errors is non-empty. The transaction always rolls back.
func (uc *DisbursementImporter) Preview(ctx context.Context, rows []domain.VoucherImportRow) (VoucherImportResult, error) {
	var res VoucherImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		plan, errs, err := uc.plan(ctx, tx, rows)
		if err != nil {
			return err
		}
		res = VoucherImportResult{RowCount: plan.RowCount, Total: plan.Total, Errors: errs}
		return errVoucherPreviewRollback
	})
	if err != nil && !errors.Is(err, errVoucherPreviewRollback) {
		return VoucherImportResult{}, bocChungTu(ctx, "xem trước nhập Excel", err)
	}
	return res, nil
}

// Import writes the file, or nothing.
func (uc *DisbursementImporter) Import(ctx context.Context, rows []domain.VoucherImportRow, actor audit.Actor) (VoucherImportResult, error) {
	if actor.ID == "" {
		return VoucherImportResult{}, ErrVoucherImportNoActor
	}
	batch, err := uc.newID()
	if err != nil {
		return VoucherImportResult{}, fmt.Errorf("chung_tu_giai_ngan: sinh mã lô nhập: %w", err)
	}
	var res VoucherImportResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = VoucherImportResult{}
		plan, errs, err := uc.plan(ctx, tx, rows)
		if err != nil {
			return err
		}
		if len(errs) > 0 {
			// Returned from INSIDE the closure so the transaction rolls back; nothing was written.
			return &VoucherImportRejected{Errors: errs}
		}
		created := make([]ImportedVoucher, 0, len(plan.Vouchers))
		for _, p := range plan.Vouchers {
			moi, err := newVoucher(YeuCauThemChungTu{
				DuAnID: p.ProjectID, NgayChi: p.PaymentDate, SoTien: p.Amount, NoiDung: p.Description,
				DoiTac: p.Counterparty, SoChungTu: p.VoucherNo, NguonVonID: p.SourceID,
			}, actor, uc.newID)
			if err == nil {
				err = insertNewVoucher(ctx, tx, uc.repo, moi, actor, map[string]any{
					"nguon":   voucherImportSource,
					"lo_nhap": batch,
					"dong":    p.Row,
					"so_dong": len(plan.Vouchers),
				})
			}
			if err != nil {
				if msg, ok := voucherWriteRefusal(err); ok {
					// The form's path refused what the planner passed: the two drifted, or the data moved
					// under the plan. Either way it is THIS row, and the whole file rolls back.
					return &VoucherImportRejected{Errors: []domain.VoucherImportError{{Row: p.Row, Message: msg}}}
				}
				return err
			}
			created = append(created, ImportedVoucher{Row: p.Row, ID: moi.ID, ProjectCode: p.ProjectCode})
		}
		res = VoucherImportResult{RowCount: plan.RowCount, Total: plan.Total, Batch: batch, Created: created}
		return nil
	})
	if err != nil {
		var rej *VoucherImportRejected
		if errors.As(err, &rej) {
			return VoucherImportResult{}, rej
		}
		return VoucherImportResult{}, bocChungTu(ctx, "nhập Excel", err)
	}
	return res, nil
}

// voucherWriteRefusal names the refusals of the form's create path that are about the ROW rather than
// the system, with a sentence for the row. Listed explicitly, like laLoiDauVaoChungTu in the handler: a
// default of "the row's fault" would turn a database outage into a row error.
func voucherWriteRefusal(err error) (string, bool) {
	switch {
	case errors.Is(err, fistore.ErrKhongThayDuAnCuaChungTu):
		return "Dự án của dòng này vừa bị xoá hoặc không còn trong xã.", true
	case errors.Is(err, fistore.ErrKhongThayNguonVonCuaChungTu):
		return "Nguồn vốn của dòng này không còn trong danh mục của xã.", true
	case errors.Is(err, domain.ErrSourceRequired):
		return "Dự án đã gắn nguồn vốn — ghi tên nguồn ở cột Nguồn vốn.", true
	case errors.Is(err, domain.ErrSourceNotAllocated):
		return "Nguồn vốn này không được phân bổ cho dự án — ghi một nguồn đã phân bổ cho dự án.", true
	}
	for _, e := range []error{
		domain.ErrThieuDuAn, domain.ErrSoTienKhongDuong, domain.ErrSoTienQuaLon, domain.ErrThieuNgayChi,
		domain.ErrNgayChiNgoaiLich, domain.ErrThieuNoiDung, domain.ErrNoiDungQuaDai, domain.ErrDoiTacQuaDai,
		domain.ErrSoChungTuQuaDai, domain.ErrNguonVonIDSai,
	} {
		if errors.Is(err, e) {
			// The domain's own sentence: fixed text, no cell value (rule 3, forbidden #3).
			return "Dòng này không ghi được: " + e.Error(), true
		}
	}
	return "", false
}
