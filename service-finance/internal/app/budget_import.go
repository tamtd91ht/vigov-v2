package app

// "Nạp từ Excel" on the revenue / expenditure board (docs/ui-ux/07-thu-chi-ngan-sach.md §6; ADR 0081
// #6, decided 08/10/2026). The file is parsed in internal/domain (budget_import.go); this layer decides,
// inside ONE transaction, what the parsed sheets do to the commune's board, and writes it with its trail.
//
//	Preview  every check below, then ROLLBACK — writes nothing, audits nothing
//	Import   every check, then every sheet of the file, all or nothing
//
// THE CHECKS, in the order they run (each refuses the WHOLE file — a file that loads half is a year
// whose thu and chi come from two different reports):
//
//	1. the year, or ANY month of it, is closed       domain.ImportLockingClose (stricter than the
//	                                                 sheet-edit guard: an import replaces a year's figures)
//	2. the live sheet of that year+kind holds figures domain.HandEntriesError — the user's decision of
//	   entered by hand (live batches, or cells typed  30/09/2026 and ADR 0081 #6: `Gỡ` first, a
//	   or edited after the sheet was created)         deliberate act with a reason, never an overwrite
//
// A LIVE SHEET WITHOUT HAND ENTRIES IS REPLACED: soft deleted (rule 7) with its own `go_bang_ngan_sach`
// entry naming the new code, and the file becomes the next revision (`lan`, migration 0006) — so
// `NS-2026-CHI-01` stays in the table, readable, and the new sheet is `NS-2026-CHI-02`.
//
// THE LOCK ORDER IS THE ONE budget_period_close.go STATES FOR EVERY BUDGET WRITE: the sheets' FOR UPDATE
// first, then the (tenant, year) advisory lock. The live sheets are read AGAIN under the advisory lock
// and that second read is the one acted on: every creator of a sheet (TaoBang, another import) holds the
// same advisory lock while it inserts, so after taking it no live sheet can appear or vanish unseen —
// without the re-read, two imports racing on an empty year would both create a live sheet.
//
// PERMISSION: `budget.update`, the key POST /api/v1/budget-sheets takes (the caller's card). ⚠ Replacing
// a live sheet soft deletes it, which DELETE /api/v1/budget-sheets/{id} guards with `budget.confirm`;
// here it happens under `budget.update` ONLY when the sheet holds nothing entered by hand. Stated in the
// hand-back as a decision the owner may tighten.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// ActionBudgetSheetImport is the trail's verb for one sheet loaded from Excel. The VALUE is Vietnamese
// snake_case like every verb of this board (an inspection reads it); the identifier is English (rule 12).
const ActionBudgetSheetImport = "nap_excel_bang_ngan_sach"

// importReplaceReason is the `delete_reason` of a sheet replaced by a newer load of the same year+kind.
const importReplaceReason = "Thay bằng bản nạp lại từ tệp Excel"

// BudgetImportRequest is one parsed file for one budget year.
type BudgetImportRequest struct {
	Year     int
	FileName string
	Sheets   []domain.ImportedSheet
}

// BudgetImportOutcome is what one sheet of the file does (Preview) or did (Import).
type BudgetImportOutcome struct {
	Sheet domain.ImportedSheet

	// Replaces is the live sheet this one replaces; zero ID when the year+kind had none.
	Replaces domain.BangNganSach

	// Refusal is the sentence that refuses this sheet (Preview only — Import returns it as the error).
	Refusal string

	// Created and Columns are what Import wrote. Zero on a preview.
	Created domain.BangNganSach
	Columns []domain.CotNganSach
}

// BudgetImportResult is the whole file.
type BudgetImportResult struct {
	Year     int
	FileName string

	// Refusal is the period-close sentence when the year or a month of it is closed (Preview only).
	Refusal string

	Sheets []BudgetImportOutcome
}

// Valid reports whether the import would go through.
func (r BudgetImportResult) Valid() bool {
	if r.Refusal != "" {
		return false
	}
	for _, s := range r.Sheets {
		if s.Refusal != "" {
			return false
		}
	}
	return true
}

var (
	errBudgetImportPreviewRollback = errors.New("ngan_sach: xem trước nạp Excel — huỷ giao dịch")

	// ErrBudgetImportEmpty — a request with no sheet. internal/http answers a file without a table
	// before it gets here; this is the use case not trusting that.
	ErrBudgetImportEmpty = errors.New("ngan_sach: tệp không có bảng thu, chi nào để nạp")

	// ErrBudgetImportDuplicateKind — two sheets of one kind in one request.
	ErrBudgetImportDuplicateKind = errors.New("ngan_sach: tệp có hai bảng cùng loại")
)

func validateBudgetImport(req BudgetImportRequest) error {
	if err := domain.KiemTraNamNganSach(req.Year); err != nil {
		return err
	}
	if len(req.Sheets) == 0 {
		return ErrBudgetImportEmpty
	}
	seen := map[domain.LoaiBang]bool{}
	for _, s := range req.Sheets {
		if err := domain.KiemTraLoaiBang(s.Kind); err != nil {
			return err
		}
		if seen[s.Kind] {
			return ErrBudgetImportDuplicateKind
		}
		seen[s.Kind] = true
	}
	return nil
}

// PreviewBudgetImport runs every check and reports — the transaction always rolls back. The returned
// error is a system failure or a malformed request; a refused file is a SUCCESSFUL preview whose
// Refusal fields say why.
func (uc *NganSach) PreviewBudgetImport(ctx context.Context, req BudgetImportRequest) (BudgetImportResult, error) {
	if err := validateBudgetImport(req); err != nil {
		return BudgetImportResult{}, err
	}
	res := BudgetImportResult{Year: req.Year, FileName: domain.NormaliseSourceFileName(req.FileName)}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.runBudgetImport(ctx, tx, req, nil, &res); err != nil {
			return err
		}
		return errBudgetImportPreviewRollback
	})
	if err != nil && !errors.Is(err, errBudgetImportPreviewRollback) {
		return BudgetImportResult{}, bocNganSach(ctx, "xem trước nạp Excel", err)
	}
	return res, nil
}

// ImportBudgetWorkbook loads the file, or nothing. A period close or a sheet with hand entries is
// returned as the error (domain.ErrPeriodClosed / domain.ErrBudgetSheetHasHandEntries).
func (uc *NganSach) ImportBudgetWorkbook(ctx context.Context, req BudgetImportRequest,
	actor audit.Actor) (BudgetImportResult, error) {

	if err := validateBudgetImport(req); err != nil {
		return BudgetImportResult{}, err
	}
	if err := coNguoiThucHien(actor); err != nil {
		return BudgetImportResult{}, err
	}
	var res BudgetImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = BudgetImportResult{Year: req.Year, FileName: domain.NormaliseSourceFileName(req.FileName)}
		return uc.runBudgetImport(ctx, tx, req, &actor, &res)
	})
	if err != nil {
		return BudgetImportResult{}, bocNganSach(ctx, "nạp Excel", err)
	}
	return res, nil
}

// runBudgetImport is the shared body. actor == nil is the preview: refusals go into res and nothing is
// written. Otherwise the first refusal is returned and the caller's transaction rolls back.
func (uc *NganSach) runBudgetImport(ctx context.Context, tx *store.ScopedTx, req BudgetImportRequest,
	actor *audit.Actor, res *BudgetImportResult) error {

	sheets := domain.SortedKinds(req.Sheets)

	// (1) the sheets' row locks, in the standard order; (2) the year's advisory lock; (3) the re-read.
	for _, s := range sheets {
		if _, err := uc.kho.LiveSheetsForUpdate(ctx, tx, req.Year, s.Kind); err != nil {
			return err
		}
	}
	if err := uc.kho.LockBudgetYears(ctx, tx, req.Year); err != nil {
		return err
	}
	live := make([][]domain.BangNganSach, len(sheets))
	for i, s := range sheets {
		l, err := uc.kho.LiveSheetsForUpdate(ctx, tx, req.Year, s.Kind)
		if err != nil {
			return err
		}
		if len(l) > 1 {
			// A state no route of this service creates. Refused rather than resolved: replacing only
			// the newest would leave the other live beside the new one.
			return fmt.Errorf("ngan_sach: có %d bảng %s năm %d cùng còn hiệu lực", len(l), s.Kind, req.Year)
		}
		live[i] = l
	}

	closes, err := uc.kho.ActiveBudgetPeriodCloses(ctx, tx, req.Year, req.Year)
	if err != nil {
		return err
	}
	if c, locked := domain.ImportLockingClose(closes, req.Year); locked {
		if actor != nil {
			return domain.ImportPeriodClosedError(c)
		}
		res.Refusal = domain.ImportPeriodClosedError(c).Error()
	}

	res.Sheets = make([]BudgetImportOutcome, len(sheets))
	for i, s := range sheets {
		out := BudgetImportOutcome{Sheet: s}
		if len(live[i]) == 1 {
			old := live[i][0]
			out.Replaces = old
			lines, batches, cells, err := uc.kho.HandEntries(ctx, tx, old.ID)
			if err != nil {
				return err
			}
			if batches > 0 || cells > 0 {
				refusal := domain.HandEntriesError(old, lines, batches, cells)
				if actor != nil {
					return refusal
				}
				out.Refusal = refusal.Error()
			}
		}
		res.Sheets[i] = out
	}
	if actor == nil {
		return nil
	}

	loadedAt := time.Now().UTC()
	for i := range res.Sheets {
		if err := uc.writeImportedSheet(ctx, tx, req.Year, res.FileName, loadedAt, *actor, &res.Sheets[i]); err != nil {
			return err
		}
	}
	return nil
}

// writeImportedSheet replaces (when there is one) the live sheet and writes the parsed one with its
// columns, lines, leaf figures and star, then the entry. Every statement is thu_chi_ngan_sach.go's own.
func (uc *NganSach) writeImportedSheet(ctx context.Context, tx *store.ScopedTx, year int, fileName string,
	loadedAt time.Time, actor audit.Actor, out *BudgetImportOutcome) error {

	s := out.Sheet
	sheetID, err := uc.sinhID()
	if err != nil {
		return fmt.Errorf("ngan_sach: sinh mã bảng: %w", err)
	}
	colIDs := make([]string, len(s.Columns))
	for i := range colIDs {
		if colIDs[i], err = uc.sinhID(); err != nil {
			return fmt.Errorf("ngan_sach: sinh mã cột: %w", err)
		}
	}
	cols, err := domain.ImportedColumnSet(s, sheetID, colIDs)
	if err != nil {
		return err
	}
	revision, err := uc.kho.LanKeTiep(ctx, tx, year, s.Kind)
	if err != nil {
		return err
	}
	created := domain.BangNganSach{
		ID: sheetID, Ma: maBang(year, s.Kind, revision), Nam: year, Loai: s.Kind, Lan: revision,
		TieuDe: s.Title, DonViTinh: s.Unit.Nhan(), NguonTep: fileName, NapLuc: loadedAt,
	}

	if old := out.Replaces; old.ID != "" {
		// `deleted_by` holds the staff business code, as GoBang writes it (rule 6, invariant 8).
		if err := uc.kho.XoaMemBang(ctx, tx, old.ID, actor.ID, importReplaceReason); err != nil {
			return err
		}
		// ITS OWN ENTRY, with the verb GoBang writes: "every removal of a year's sheet" stays one query
		// on the trail whichever route removed it.
		delta, err := json.Marshal(map[string]any{
			"bang_id": old.ID, "truoc": tomTatBang(old, nil), "ly_do": importReplaceReason,
			"xoa_mem": true, "thay_bang_nap_excel": created.Ma,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: HanhViGoBangNganSach, Subject: old.Ma, Delta: delta,
		}); err != nil {
			return err
		}
	}

	if err := uc.kho.ChenBang(ctx, tx, created); err != nil {
		return err
	}
	for _, c := range cols {
		if err := uc.kho.ChenCot(ctx, tx, c); err != nil {
			return err
		}
	}

	lineIDs := make([]string, len(s.Lines))
	for i := range lineIDs {
		if lineIDs[i], err = uc.sinhID(); err != nil {
			return fmt.Errorf("ngan_sach: sinh mã khoản mục: %w", err)
		}
	}
	cells := 0
	for i, l := range s.Lines {
		line := domain.KhoanMucNganSach{
			ID: lineIDs[i], BangID: sheetID, TT: l.TT, Ten: l.Name, ThuTu: i + 1, Cap: l.Depth,
			// DERIVED FROM THE TREE, as for every line (domain.CachTinhTheoCay).
			CachTinh: domain.CachTinhTheoCay(s.HasChildren(i)),
		}
		if l.Parent >= 0 {
			line.ChaID = lineIDs[l.Parent]
		}
		if err := uc.kho.ChenKhoanMuc(ctx, tx, line); err != nil {
			return err
		}
		// A PARENT'S FIGURE IS NOT STORED: it always sums its children (the customer's rule of
		// 06/09/2026, domain.ChoGhiGiaTri). Leaves only, in column order so two runs write alike.
		if line.CachTinh == domain.TinhTheoCon {
			continue
		}
		for ci := range cols {
			g, ok := l.Values[ci]
			if !ok {
				continue
			}
			v := g
			if err := uc.kho.GhiGiaTri(ctx, tx, line.ID, cols[ci].ID, &v); err != nil {
				return err
			}
			cells++
		}
	}

	var headline any
	if s.Headline >= 0 {
		// The star goes through the star's own statement pair — the only writer of `is_headline`.
		if err := uc.kho.DatDongTong(ctx, tx, sheetID, lineIDs[s.Headline]); err != nil {
			return err
		}
		l := s.Lines[s.Headline]
		headline = map[string]any{"khoan_muc_id": lineIDs[s.Headline], "tt": l.TT, "ten": l.Name}
	}

	var replaced any
	if out.Replaces.ID != "" {
		replaced = out.Replaces.Ma
	}
	// THE ENTRY RECORDS THE ACT AND THE SHAPE, NOT EVERY FIGURE: the figures are the rows just
	// written, and the replaced sheet keeps its own in the table. What the entry adds is what only it
	// can say: which file, which guesses the system made (the star, the column roles), what it replaced.
	delta, err := json.Marshal(map[string]any{
		"sau":                           tomTatBang(created, cols),
		"nguon_tep":                     fileName,
		"sheet":                         s.SheetName,
		"so_khoan_muc":                  len(s.Lines),
		"so_o_so":                       cells,
		"dong_tong_tu_danh_dau":         headline,
		"vai_tro_cot_doan_tu_tieu_de":   true,
		"cot_phan_tram_khong_tinh_duoc": s.UnidentifiedPercentColumns(),
		"thay_bang":                     replaced,
	})
	if err != nil {
		return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		Actor: actor, Action: ActionBudgetSheetImport, Subject: created.Ma, Delta: delta,
	}); err != nil {
		return err
	}
	out.Created, out.Columns = created, cols
	return nil
}
