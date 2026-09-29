package store

// The batches of revenue/expenditure against one budget line — reads and writes (migration 0008,
// docs/ui-ux/07-thu-chi-ngan-sach.md §5). The five properties at the top of budget.go hold
// here unchanged; three more are specific to these two tables:
//
//  1. EVERY SUM AND EVERY LIST JOINS `dot_thu_chi.deleted_at IS NULL`. `gia_tri_dot` has no soft-delete
//     columns of its own (0008 says why), so a read that forgets the join counts removed batches.
//  2. `gia_tri_dot` IS ONLY EVER INSERTED INTO. Its trigger refuses UPDATE and DELETE; no statement
//     here tries either.
//  3. `don_vi_ca_nhan` MAY BE A PERSON'S NAME. It is read and written as a bound parameter and never
//     appears in an error built here (rule 3, forbidden #3).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// ErrTooManyEntries — one line has more live batches than MaxEntriesPerLine. REFUSED, NEVER TRUNCATED:
// the list is what an accountant reconciles against receipts, and a silently short list is a batch
// that looks missing while it is still counted in the line's figure.
var ErrTooManyEntries = errors.New("ngan_sach: khoản mục vượt trần số đợt thu chi")

// MaxEntriesPerLine — the domain's ceiling (domain.MaxEntriesPerLine), named here too because the
// list read below enforces it. One value: the write refuses at the same number the read does.
const MaxEntriesPerLine = domain.MaxEntriesPerLine

// entryTotalsOfSheet sums the LIVE batches of every live LEAF line in `entries` mode of one sheet, per
// line and column.
//
// ONLY `entries` LEAVES ($3 = 'entries', no live child): those are the only lines whose displayed
// figure is the batch sum (FullSheet.Value). Summing every line's batches cost a scan for figures
// nobody reads — and, before the sum stopped failing the read, let one oversized sum on a `manual`
// line lock the whole sheet over a number that counts nowhere.
//
// `d.deleted_at IS NULL` IS THE CLAUSE THAT CAUSES INCIDENTS WHEN MISSING (0008, question 4a).
// `g.gia_tri IS NOT NULL` + GROUP BY means a (line, column) with no stated amount produces NO ROW, so
// the domain reads it as empty (`—`), never 0 (§9 rule 4).
//
// `::text` BECAUSE SUM(BIGINT) IS NUMERIC in PostgreSQL. The text is parsed with strconv.ParseInt, so a
// sum that does not fit int64 is detected (strconv.ErrRange) rather than whatever a driver does with an
// oversize numeric — and it becomes THAT cell's reason, not the sheet's failure (scanEntryTotals).
//
// tenant_id IS BOUND ON EVERY TABLE OF THE JOIN (rule 1): joining on ids alone would match another
// commune's row wherever ids collide.
const entryTotalsOfSheet = `SELECT d.khoan_muc_id, g.cot_id, SUM(g.gia_tri)::text
	FROM gia_tri_dot g
	JOIN dot_thu_chi d
	  ON d.tenant_id = g.tenant_id AND d.id = g.dot_id
	JOIN khoan_muc_ngan_sach k
	  ON k.tenant_id = d.tenant_id AND k.id = d.khoan_muc_id
	WHERE g.tenant_id = $1 AND k.bang_id = $2
	  AND k.deleted_at IS NULL AND d.deleted_at IS NULL AND g.gia_tri IS NOT NULL
	  AND k.cach_tinh = $3
	  AND NOT EXISTS (SELECT 1 FROM khoan_muc_ngan_sach c
	                   WHERE c.tenant_id = k.tenant_id AND c.cha_id = k.id AND c.deleted_at IS NULL)
	GROUP BY d.khoan_muc_id, g.cot_id`

// scanEntryTotals returns the sums that fit int64, and SEPARATELY the (line, column) pairs whose sum does
// not. An oversized sum is NOT an error of the read: failing here failed the whole sheet, which locked
// every write — RemoveEntry included, the one act that removes the offending batch. The domain turns the
// flag into that one cell's reason (domain.ErrEntryTotalOverflow). Any OTHER parse failure is not a size
// and still fails: a sum column that is not a number is a defect, not a figure.
func scanEntryTotals(rows *sql.Rows) (map[string]map[string]domain.Dong, map[string]map[string]bool, error) {
	defer rows.Close()
	result := map[string]map[string]domain.Dong{}
	overflow := map[string]map[string]bool{}
	for rows.Next() {
		var lineID, columnID, total string
		if err := rows.Scan(&lineID, &columnID, &total); err != nil {
			return nil, nil, fmt.Errorf("ngan_sach: đọc dòng tổng đợt: %w", err)
		}
		g, err := strconv.ParseInt(total, 10, 64)
		if errors.Is(err, strconv.ErrRange) {
			if overflow[lineID] == nil {
				overflow[lineID] = map[string]bool{}
			}
			overflow[lineID][columnID] = true
			continue
		}
		if err != nil {
			// The text is not quoted: it is a figure from the commune's budget.
			return nil, nil, fmt.Errorf("ngan_sach: tổng đợt không phải số nguyên: %w", strconv.ErrSyntax)
		}
		if result[lineID] == nil {
			result[lineID] = map[string]domain.Dong{}
		}
		result[lineID][columnID] = domain.Dong(g)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("ngan_sach: duyệt tổng đợt: %w", err)
	}
	return result, overflow, nil
}

// --- reads outside a transaction (GET /api/v1/budget-lines/{id}/entries) ---------------------------

// liveLineInLiveSheet reads one live line whose SHEET is live too. A line of a removed sheet
// is off every screen (budget.go, softDeleteSheet), so its batches must be too.
const liveLineInLiveSheet = `SELECT k.id, k.bang_id, COALESCE(k.cha_id, ''), k.tt, k.ten,
	k.thu_tu, k.cach_tinh, k.cap, k.is_headline
	FROM khoan_muc_ngan_sach k
	JOIN bang_ngan_sach b
	  ON b.tenant_id = k.tenant_id AND b.id = k.bang_id
	WHERE k.tenant_id = $1 AND k.id = $2 AND k.deleted_at IS NULL AND b.deleted_at IS NULL`

const entryColumns = `id, khoan_muc_id, ngay, noi_dung, COALESCE(don_vi_ca_nhan, ''), ` +
	`COALESCE(so_chung_tu, ''), nguoi_ghi_ma, tao_luc`

// entryAmountsOfLine — every stated amount of one line's LIVE batches.
const entryAmountsOfLine = `SELECT g.dot_id, g.cot_id, g.gia_tri
	FROM gia_tri_dot g
	JOIN dot_thu_chi d
	  ON d.tenant_id = g.tenant_id AND d.id = g.dot_id
	WHERE g.tenant_id = $1 AND d.khoan_muc_id = $2
	  AND d.deleted_at IS NULL AND g.gia_tri IS NOT NULL`

// LineEntries reads what the `⇄` dialog shows: the line, its sheet's live columns, and its live
// batches NEWEST FIRST (`ngay DESC, tao_luc DESC`, then `id` so the order is total).
//
// A line that does not exist, was removed, sits in a removed sheet, or belongs to another commune is
// ONE answer — domain.ErrLineNotFound — because the query reaches none of them.
func (s *BudgetStore) LineEntries(ctx context.Context,
	lineID string) (domain.LineEntries, error) {

	rows, err := s.db.For(ctx).QueryJoin(ctx, liveLineInLiveSheet, lineID)
	if err != nil {
		return domain.LineEntries{}, fmt.Errorf("ngan_sach: đọc khoản mục của đợt: %w", err)
	}
	k, found, err := scanOneLine(rows)
	if err != nil {
		return domain.LineEntries{}, err
	}
	if !found {
		return domain.LineEntries{}, domain.ErrLineNotFound
	}

	column, err := s.ColumnsOfSheet(ctx, k.SheetID)
	if err != nil {
		return domain.LineEntries{}, err
	}

	// LIMIT IS THE CEILING PLUS ONE — the same reason LinesOfSheet gives: selecting exactly the
	// ceiling cannot tell a full list from a truncated one.
	rows, err = s.db.For(ctx).Query(ctx, entryColumns, "dot_thu_chi",
		`AND deleted_at IS NULL AND khoan_muc_id = $2 ORDER BY ngay DESC, tao_luc DESC, id DESC LIMIT $3`,
		lineID, MaxEntriesPerLine+1)
	if err != nil {
		return domain.LineEntries{}, fmt.Errorf("ngan_sach: đọc đợt: %w", err)
	}
	entry, err := scanEntries(rows)
	if err != nil {
		return domain.LineEntries{}, err
	}
	if len(entry) > MaxEntriesPerLine {
		return domain.LineEntries{}, ErrTooManyEntries
	}

	rows, err = s.db.For(ctx).QueryJoin(ctx, entryAmountsOfLine, lineID)
	if err != nil {
		return domain.LineEntries{}, fmt.Errorf("ngan_sach: đọc số tiền đợt: %w", err)
	}
	if err := attachAmounts(rows, entry); err != nil {
		return domain.LineEntries{}, err
	}
	return domain.LineEntries{Line: k, Columns: column, Entries: entry}, nil
}

func scanOneLine(rows *sql.Rows) (domain.BudgetLine, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.BudgetLine{}, false, fmt.Errorf("ngan_sach: đọc khoản mục: %w", err)
		}
		return domain.BudgetLine{}, false, nil
	}
	var (
		k      domain.BudgetLine
		method string
	)
	err := rows.Scan(&k.ID, &k.SheetID, &k.ParentID, &k.OrdinalLabel, &k.Name, &k.SortOrder, &method, &k.Level, &k.IsHeadline)
	if err != nil {
		return domain.BudgetLine{}, false, fmt.Errorf("ngan_sach: đọc dòng khoản mục: %w", err)
	}
	k.Method = domain.LineMethod(method)
	return k, true, nil
}

func scanEntries(rows *sql.Rows) ([]domain.BudgetEntry, error) {
	defer rows.Close()
	result := make([]domain.BudgetEntry, 0, 16)
	for rows.Next() {
		var d domain.BudgetEntry
		err := rows.Scan(&d.ID, &d.LineID, &d.Date, &d.Content, &d.Counterparty,
			&d.VoucherNo, &d.EnteredBy, &d.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng đợt: %w", err)
		}
		d.Amounts = map[string]domain.Dong{}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt đợt: %w", err)
	}
	return result, nil
}

// attachAmounts attaches amounts to the batches already read. An amount whose batch is not in the list
// cannot occur (both reads bind the same line and `deleted_at IS NULL`) and is skipped rather than
// invented into a batch.
func attachAmounts(rows *sql.Rows, entry []domain.BudgetEntry) error {
	defer rows.Close()
	byID := make(map[string]int, len(entry))
	for i := range entry {
		byID[entry[i].ID] = i
	}
	for rows.Next() {
		var entryID, columnID string
		var value int64
		if err := rows.Scan(&entryID, &columnID, &value); err != nil {
			return fmt.Errorf("ngan_sach: đọc dòng số tiền đợt: %w", err)
		}
		if i, ok := byID[entryID]; ok {
			entry[i].Amounts[columnID] = domain.Dong(value)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ngan_sach: duyệt số tiền đợt: %w", err)
	}
	return nil
}

// --- reads INSIDE a transaction ----------------------------------------------------------------------

const liveEntryByID = `SELECT ` + entryColumns + ` FROM dot_thu_chi ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

const amountsOfOneEntry = `SELECT g.dot_id, g.cot_id, g.gia_tri
	FROM gia_tri_dot g
	WHERE g.tenant_id = $1 AND g.dot_id = $2 AND g.gia_tri IS NOT NULL`

// EntryByIDInTx reads one LIVE batch with its amounts, inside the transaction that is about
// to remove it. The caller then locks the SHEET (the serialisation point for every write on this
// board) and the soft delete's own `AND deleted_at IS NULL` catches a concurrent removal.
func (s *BudgetStore) EntryByIDInTx(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.BudgetEntry, error) {

	rows, err := tx.Underlying().QueryContext(ctx, liveEntryByID, string(tx.TenantID()), id)
	if err != nil {
		return domain.BudgetEntry{}, fmt.Errorf("ngan_sach: đọc đợt để gỡ: %w", err)
	}
	entry, err := scanEntries(rows)
	if err != nil {
		return domain.BudgetEntry{}, err
	}
	if len(entry) == 0 {
		return domain.BudgetEntry{}, domain.ErrEntryNotFound
	}

	rows, err = tx.Underlying().QueryContext(ctx, amountsOfOneEntry, string(tx.TenantID()), id)
	if err != nil {
		return domain.BudgetEntry{}, fmt.Errorf("ngan_sach: đọc số tiền đợt để gỡ: %w", err)
	}
	if err := attachAmounts(rows, entry[:1]); err != nil {
		return domain.BudgetEntry{}, err
	}
	return entry[0], nil
}

const hasLiveEntriesSQL = `SELECT EXISTS (SELECT 1 FROM dot_thu_chi
	WHERE tenant_id = $1 AND khoan_muc_id = $2 AND deleted_at IS NULL)`

// HasLiveEntries reports whether one line still has a LIVE batch — the question the child-add refusal
// turns on (domain.ErrEntriesLineHasEntries).
func (s *BudgetStore) HasLiveEntries(ctx context.Context, tx *store.ScopedTx,
	lineID string) (bool, error) {

	var ok bool
	err := tx.Underlying().QueryRowContext(ctx, hasLiveEntriesSQL, string(tx.TenantID()), lineID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("ngan_sach: kiểm đợt còn sống: %w", err)
	}
	return ok, nil
}

const countLiveEntriesSQL = `SELECT count(*) FROM dot_thu_chi
	WHERE tenant_id = $1 AND khoan_muc_id = $2 AND deleted_at IS NULL`

// CountLiveEntries counts one line's LIVE batches, inside the write transaction — the number the batch
// ceiling (domain.MaxEntriesPerLine) is checked against before an insert. It runs AFTER the sheet's
// FOR UPDATE lock (every batch write and removal takes that lock first), so two concurrent writes
// cannot both see 1999 and both insert.
func (s *BudgetStore) CountLiveEntries(ctx context.Context, tx *store.ScopedTx, lineID string) (int, error) {
	var n int
	err := tx.Underlying().QueryRowContext(ctx, countLiveEntriesSQL, string(tx.TenantID()), lineID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ngan_sach: đếm đợt còn sống: %w", err)
	}
	return n, nil
}

// --- writes --------------------------------------------------------------------------------------------

// insertEntry — `tao_luc` takes its default and `deleted_*` are absent: a batch is born live. Empty
// optional text goes in as NULL, never ” (0008 refuses ” so "not stated" has one spelling).
const insertEntry = `INSERT INTO dot_thu_chi
	(tenant_id, id, khoan_muc_id, ngay, noi_dung, don_vi_ca_nhan, so_chung_tu, nguoi_ghi_ma)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// InsertEntry records one batch row. Its amounts follow through InsertEntryAmount in the same transaction.
func (s *BudgetStore) InsertEntry(ctx context.Context, tx *store.ScopedTx, d domain.BudgetEntry) error {
	_, err := tx.Exec(ctx, insertEntry, string(tx.TenantID()),
		d.ID, d.LineID, d.Date, d.Content,
		emptyToNil(d.Counterparty), emptyToNil(d.VoucherNo), d.EnteredBy)
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn đợt: %w", err)
	}
	return nil
}

const insertEntryAmount = `INSERT INTO gia_tri_dot (tenant_id, dot_id, cot_id, gia_tri) VALUES ($1, $2, $3, $4)`

// InsertEntryAmount records one amount of one batch. Plain INSERT, no upsert: an amount is written once
// and the primary key (tenant_id, dot_id, cot_id) refuses a second one for the same column.
func (s *BudgetStore) InsertEntryAmount(ctx context.Context, tx *store.ScopedTx,
	entryID, columnID string, value domain.Dong) error {

	if _, err := tx.Exec(ctx, insertEntryAmount, string(tx.TenantID()), entryID, columnID, int64(value)); err != nil {
		return fmt.Errorf("ngan_sach: chèn số tiền đợt: %w", err)
	}
	return nil
}

// softDeleteEntry writes rule 7 invariant 1's three columns in one statement — the ONE update 0008's
// `dot_thu_chi_bat_bien` admits. `AND deleted_at IS NULL` makes a second removal a not-found (404)
// rather than a statement the trigger would refuse with a 500.
const softDeleteEntry = `UPDATE dot_thu_chi
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *BudgetStore) SoftDeleteEntry(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteEntry, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("ngan_sach: xoá mềm đợt: %w", err)
	}
	return expectOneBudgetRow(res, "xoá mềm đợt", domain.ErrEntryNotFound)
}
