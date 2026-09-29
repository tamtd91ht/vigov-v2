package app

// The use cases behind the `⇄ Các đợt thu, chi` dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5,
// migration 0008): record one batch against a leaf line, and remove one with a reason. Same shape as
// every write in budget.go — lock the sheet, read the tree under the lock, ask
// internal/domain, write, and write the audit entry in the SAME transaction (rule 6, invariant 3).
//
// WRITING A BATCH DOES NOT SWITCH THE LINE'S MODE (user decision 25/09/2026, §4.2). A batch on a
// `manual` leaf is stored and counts nowhere until somebody switches that leaf to `entries` through
// PATCH /api/v1/budget-lines/{id}. The audit entry records the mode the line had, so "why did this
// batch not move the figure" has an answer.
//
// ⚠ migration 0008's header says the line "must be a LEAF in `entries` mode when the batch is
// written". The user's later decision (no auto-switch, the user chooses) is what is implemented: the
// LEAF half is enforced, the `entries` half is not.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// RecordBudgetEntryRequest is one batch as the dialog's form sends it.
//
// THERE IS NO `ID`, NO `EnteredBy` AND NO `CreatedAt`: the id is issued here, the author is the session's
// staff code, and the timestamp is the database's.
type RecordBudgetEntryRequest struct {
	LineID       string
	Date         time.Time
	Content      string
	Counterparty string // optional; PERSONAL DATA when it names a person
	VoucherNo    string // optional

	// Amounts is columnID -> amount. A nil value is "left empty" and writes no row — 0008: "a column with
	// NO row is empty too". At least one amount must be non-nil (domain.ErrEntryHasNoAmount).
	Amounts map[string]*domain.Dong
}

// RecordEntry records one batch and its amounts against one leaf line.
//
// REFUSED, ALL BEFORE ANY WRITE: a line that is not a live line of a live sheet of this commune
// (404); a line WITH children (409, domain.ErrEntryLeafOnly); an amount keyed by a column of another
// sheet or a removed column (400, ErrColumnInOtherSheet); an amount in a `phan_tram` column (400,
// ErrColumnNotNumber). None of the three column checks exists in the database (0008 says why), so
// this function is the only thing standing between a batch and a figure filed on no screen.
func (uc *BudgetService) RecordEntry(ctx context.Context, req RecordBudgetEntryRequest,
	actor audit.Actor) (domain.BudgetEntry, error) {

	if req.LineID == "" {
		return domain.BudgetEntry{}, domain.ErrLineNotFound
	}
	if err := domain.ValidateEntryDate(req.Date); err != nil {
		return domain.BudgetEntry{}, err
	}
	description, err := domain.NormalizeEntryContent(req.Content)
	if err != nil {
		return domain.BudgetEntry{}, err
	}
	counterparty, err := domain.NormalizeEntryCounterparty(req.Counterparty)
	if err != nil {
		return domain.BudgetEntry{}, err
	}
	voucherNo, err := domain.NormalizeEntryVoucherNo(req.VoucherNo)
	if err != nil {
		return domain.BudgetEntry{}, err
	}
	hasValue := false
	for _, g := range req.Amounts {
		if g == nil {
			continue
		}
		hasValue = true
		if err := domain.ValidateValue(*g); err != nil {
			return domain.BudgetEntry{}, err
		}
	}
	if !hasValue {
		return domain.BudgetEntry{}, domain.ErrEntryHasNoAmount
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetEntry{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.BudgetEntry{}, fmt.Errorf("ngan_sach: sinh mã đợt: %w", err)
	}
	next := domain.BudgetEntry{
		ID: id, LineID: req.LineID, Date: req.Date, Content: description,
		Counterparty: counterparty, VoucherNo: voucherNo,
		// THE STAFF BUSINESS CODE — audit.Actor.ID is Principal.Ma (http.actorFrom). 0008 named the
		// column `nguoi_ghi_ma` so it cannot be read as licence to store the internal id.
		EnteredBy: actor.ID,
		Amounts:   map[string]domain.Dong{},
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sheetID, err := uc.repo.SheetIDOfLine(ctx, tx, req.LineID)
		if err != nil {
			return err
		}
		sheet, full, err := uc.lockAndReadTree(ctx, tx, sheetID)
		if err != nil {
			return err
		}
		k, found := full.ByID(req.LineID)
		if !found {
			return domain.ErrLineNotFound
		}
		if full.HasChildren(k.ID) {
			return domain.ErrEntryLeafOnly
		}
		// THE LIST'S CEILING, AT THE WRITE. The `⇄` read refuses past MaxEntriesPerLine rather than
		// truncate; a batch accepted beyond it would count in the figure and never be listable, so it
		// could never be reconciled or removed. Counted under the sheet lock lockAndReadTree just took.
		liveCount, err := uc.repo.CountLiveEntries(ctx, tx, k.ID)
		if err != nil {
			return err
		}
		if liveCount >= domain.MaxEntriesPerLine {
			return fmt.Errorf("%w (tối đa %d đợt)", domain.ErrLineEntriesFull, domain.MaxEntriesPerLine)
		}

		// EVERY MENTIONED COLUMN MUST BE A LIVE NUMBER COLUMN OF THIS SHEET — checked for all of
		// them before the first insert, in the sheet's column order so the delta reads the same way
		// twice.
		amounts := make([]map[string]any, 0, len(req.Amounts))
		mentionedCount := 0
		for _, column := range full.Columns {
			g, mentioned := req.Amounts[column.ID]
			if !mentioned {
				continue
			}
			mentionedCount++
			if column.Format != domain.ColumnFormatNumber {
				return domain.ErrColumnNotNumber
			}
			if g != nil {
				next.Amounts[column.ID] = *g
				amounts = append(amounts, map[string]any{"cot_id": column.ID, "cot": column.Name, "gia_tri": int64(*g)})
			}
		}
		if mentionedCount != len(req.Amounts) {
			return domain.ErrColumnInOtherSheet
		}

		if err := uc.repo.InsertEntry(ctx, tx, next); err != nil {
			return err
		}
		for _, column := range full.Columns {
			g, ok := next.Amounts[column.ID]
			if !ok {
				continue
			}
			if err := uc.repo.InsertEntryAmount(ctx, tx, next.ID, column.ID, g); err != nil {
				return err
			}
		}

		delta, err := json.Marshal(map[string]any{
			"dot_id":       next.ID,
			"khoan_muc_id": k.ID,
			// Whether this batch moves the displayed figure right now (§4.2): only in `entries` mode.
			"cach_tinh_khoan_muc": string(k.Method),
			"sau":                 entrySummary(next, amounts),
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionRecordBudgetEntry, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetEntry{}, wrapBudgetErr(ctx, "ghi đợt", err)
	}
	return next, nil
}

// RemoveEntry soft deletes one batch, once, with a mandatory reason. Its amounts drop out of every sum with
// it: every sum joins `dot_thu_chi.deleted_at IS NULL` (store.entryTotalsOfSheet).
//
// A SECOND REMOVAL IS A 404, never a rewrite: the UPDATE carries `AND deleted_at IS NULL`, and 0008's
// trigger would refuse a change of who removed it or why anyway.
func (uc *BudgetService) RemoveEntry(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return domain.ErrEntryNotFound
	}
	reason, err := domain.NormalizeBudgetRemoveReason(rawReason)
	if err != nil {
		return err
	}
	if err := requireActor(actor); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.EntryByIDInTx(ctx, tx, id)
		if err != nil {
			return err
		}
		sheetID, err := uc.repo.SheetIDOfLine(ctx, tx, before.LineID)
		if errors.Is(err, domain.ErrLineNotFound) {
			// A batch of a removed line is off every screen — the same answer as no batch at all.
			return domain.ErrEntryNotFound
		}
		if err != nil {
			return err
		}
		sheet, full, err := uc.lockAndReadTree(ctx, tx, sheetID)
		if err != nil {
			return err
		}
		if _, found := full.ByID(before.LineID); !found {
			return domain.ErrEntryNotFound
		}

		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds.
		if err := uc.repo.SoftDeleteEntry(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		var amounts []map[string]any
		for _, column := range full.Columns {
			if g, ok := before.Amounts[column.ID]; ok {
				amounts = append(amounts, map[string]any{"cot_id": column.ID, "cot": column.Name, "gia_tri": int64(g)})
			}
		}
		delta, err := json.Marshal(map[string]any{
			"dot_id":       before.ID,
			"khoan_muc_id": before.LineID,
			"truoc":        entrySummary(before, amounts),
			"ly_do":        reason,
			"xoa_mem":      true,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionRemoveBudgetEntry, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return wrapBudgetErr(ctx, "gỡ đợt", err)
	}
	return nil
}

// entrySummary is the audit delta's view of one batch.
//
// `don_vi_ca_nhan` IS NOT HERE — ONLY WHETHER IT WAS STATED AND HOW LONG IT IS. The column may hold a
// citizen's name (migration 0008), and an audit entry is append-only and kept for years: copying the
// text would make the ledger a personal-data store nobody can erase (rule 6 forbidden #4, rule 3).
// The batch row itself is immutable and carries the text; `dot_id` points at it.
func entrySummary(d domain.BudgetEntry, amounts []map[string]any) map[string]any {
	return map[string]any{
		"ngay":                  d.Date.Format(time.DateOnly),
		"noi_dung":              d.Content,
		"so_chung_tu":           d.VoucherNo,
		"co_don_vi_ca_nhan":     d.Counterparty != "",
		"do_dai_don_vi_ca_nhan": utf8.RuneCountInString(d.Counterparty),
		"so_tien":               amounts,
	}
}
