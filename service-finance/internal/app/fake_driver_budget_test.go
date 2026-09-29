package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// A third fake `database/sql` driver, for the budget board.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument the
// other two drivers make, and it is heaviest here because the properties are about the figures that
// go into a document sent to a higher authority:
//
//	the business write and its audit entry are in ONE transaction   (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	`is_headline` is written by EXACTLY TWO statements, and marking one row CLEARS the others
//	a soft delete writes all three of rule 7 invariant 1's columns AND releases the star
//	a cell is cleared with NULL, never with a DELETE
//	`cach_tinh` is never written from anything a request could reach
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `ho_so_luu_tru_cam_xoa_cung`, not the CHECK
// constraints on `loai` / `kieu` / `vai_tro` / `cach_tinh`, not `UNIQUE (tenant_id, nam, loai, lan)`,
// not `ON CONFLICT` actually conflicting, and not the partition routing. That half needs a real
// server (VIGOV_TEST_DSN is unset here and Docker is not running), and it is the FLOOR under
// everything this file asserts — the application refusals tested below are the SENTENCE, not the
// enforcement.

// sheetRow is the sheet the FOR UPDATE read hands back. nil means "no such live sheet".
type sheetRow struct {
	id, code    string
	year        int
	kind        string
	revision    int
	title, unit string
}

type fakeBudgetStore struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begins, commits, rollbacks int

	sheet  *sheetRow
	column []domain.BudgetColumn
	lines  []domain.BudgetLine
	value  map[string]map[string]domain.Dong

	// sheetIDOfLine is what `SELECT bang_id FROM khoan_muc_ngan_sach` answers. EMPTY MEANS NO
	// SUCH LIVE LINE, which is the case domain.ErrLineNotFound exists for.
	sheetIDOfLine string

	// hasLiveSheet is what HasLiveSheet answers, and nextRevision what NextRevision answers.
	hasLiveSheet bool
	nextRevision int

	// The batches (migration 0008). entryTotals is what the per-line SUM answers — the fake does no
	// arithmetic and no filtering, so "removed batches are excluded" is asserted on the SQL text.
	// hasLiveEntries is what HasLiveEntries answers; entry is the live batch EntryByIDInTx finds
	// (nil = none).
	entryTotals map[string]map[string]domain.Dong
	// rawEntryTotals OVERRIDES entryTotals with the raw text the SUM(...)::text column hands back — the only
	// way to present a sum that does not fit int64, which is the case scanEntryTotals must refuse.
	rawEntryTotals map[string]map[string]string
	hasLiveEntries bool
	entry          *domain.BudgetEntry
	// liveEntryCount is what CountLiveEntries answers — the live batch count the write ceiling is checked against.
	liveEntryCount int

	err error

	// failOnSQL fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `err`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business write down with it, and that needs everything before it to
	// have already run.
	failOnSQL string
	failed    bool
}

func (k *fakeBudgetStore) write(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, recordedStmt{sql: q, args: vals})
	k.mu.Unlock()
}

// stmtsContaining returns every recorded statement containing `substr`, so an assertion names the statement it cares
// about rather than an index that shifts when a check is added.
func (k *fakeBudgetStore) stmtsContaining(substr string) []recordedStmt {
	k.mu.Lock()
	defer k.mu.Unlock()
	var result []recordedStmt
	for _, l := range k.stmts {
		if strings.Contains(l.sql, substr) {
			result = append(result, l)
		}
	}
	return result
}

func (k *fakeBudgetStore) hasStmt(substr string) bool { return len(k.stmtsContaining(substr)) > 0 }

// hasLiveChild mirrors the NOT EXISTS of store.entryTotalsOfSheet: whether any line of the fake names id as
// its parent.
func (k *fakeBudgetStore) hasLiveChild(id string) bool {
	for _, x := range k.lines {
		if x.ParentID == id && x.ID != id {
			return true
		}
	}
	return false
}

func (k *fakeBudgetStore) failFor(q string) error {
	if k.err != nil {
		return k.err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failOnSQL != "" && !k.failed && strings.Contains(q, k.failOnSQL) {
		k.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeBudgetStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeBudgetConn{k: k}, nil
}
func (k *fakeBudgetStore) Driver() driver.Driver { return fakeDriver{} }

type fakeBudgetConn struct{ k *fakeBudgetStore }

func (c *fakeBudgetConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeBudgetConn) Close() error { return nil }

func (c *fakeBudgetConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeBudgetConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begins++
	c.k.mu.Unlock()
	return &fakeBudgetTx{k: c.k}, nil
}

func (c *fakeBudgetConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. The store turns zero into "not found", and a fake returning zero
	// would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

// QueryContext dispatches on the statement, and THE ORDER OF THE CASES IS LOAD-BEARING: three of
// these statements name `khoan_muc_ngan_sach`, and the value join names it as well as
// `gia_tri_khoan_muc`. Matched in the wrong order, the tree read would answer the value query and
// every figure on the sheet would silently be empty.
func (c *fakeBudgetConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	switch {
	// THE BATCH STATEMENTS COME FIRST: the sum names `khoan_muc_ngan_sach` and the EXISTS names
	// `SELECT EXISTS`, both of which a later case would otherwise answer.
	case strings.Contains(q, "SUM(g.gia_tri)"):
		// THE FAKE APPLIES THE MODE AND LEAF FILTER the statement binds ($3 and the NOT EXISTS), so a
		// test can prove that a `manual` line's oversized sum is never even read. With no $3 bound
		// (an older statement) nothing would be filtered and that test would turn red.
		mode := ""
		if len(args) >= 3 {
			mode, _ = args[2].Value.(string)
		}
		var row [][]driver.Value
		for _, k := range c.k.lines {
			if string(k.Method) != mode || c.k.hasLiveChild(k.ID) {
				continue
			}
			for _, column := range c.k.column {
				if raw, ok := c.k.rawEntryTotals[k.ID][column.ID]; ok {
					row = append(row, []driver.Value{k.ID, column.ID, raw})
					continue
				}
				if g, ok := c.k.entryTotals[k.ID][column.ID]; ok {
					row = append(row, []driver.Value{k.ID, column.ID, fmt.Sprint(int64(g))})
				}
			}
		}
		return &fakeRows{columns: []string{"khoan_muc_id", "cot_id", "sum"}, rows: row}, nil

	case strings.Contains(q, "SELECT EXISTS (SELECT 1 FROM dot_thu_chi"):
		return &fakeRows{columns: []string{"exists"}, rows: [][]driver.Value{{c.k.hasLiveEntries}}}, nil

	case strings.Contains(q, "count(*) FROM dot_thu_chi"):
		return &fakeRows{columns: []string{"count"}, rows: [][]driver.Value{{int64(c.k.liveEntryCount)}}}, nil

	case strings.Contains(q, "g.dot_id = $2"):
		var row [][]driver.Value
		if d := c.k.entry; d != nil {
			for _, column := range c.k.column {
				if g, ok := d.Amounts[column.ID]; ok {
					row = append(row, []driver.Value{d.ID, column.ID, int64(g)})
				}
			}
		}
		return &fakeRows{columns: []string{"dot_id", "cot_id", "gia_tri"}, rows: row}, nil

	// The `⇄` list's amounts (store.entryAmountsOfLine) — every amount of the one live batch.
	case strings.Contains(q, "d.khoan_muc_id = $2"):
		var row [][]driver.Value
		if d := c.k.entry; d != nil {
			for _, column := range c.k.column {
				if g, ok := d.Amounts[column.ID]; ok {
					row = append(row, []driver.Value{d.ID, column.ID, int64(g)})
				}
			}
		}
		return &fakeRows{columns: []string{"dot_id", "cot_id", "gia_tri"}, rows: row}, nil

	case strings.Contains(q, "FROM dot_thu_chi WHERE"):
		if c.k.entry == nil {
			return &fakeRows{columns: entryColumns()}, nil
		}
		d := c.k.entry
		return &fakeRows{columns: entryColumns(), rows: [][]driver.Value{{
			d.ID, d.LineID, d.Date, d.Content, d.Counterparty, d.VoucherNo, d.EnteredBy, d.CreatedAt,
		}}}, nil

	case strings.Contains(q, "FROM gia_tri_khoan_muc"):
		var row [][]driver.Value
		for _, k := range c.k.lines {
			for _, column := range c.k.column {
				if g, ok := c.k.value[k.ID][column.ID]; ok {
					row = append(row, []driver.Value{k.ID, column.ID, int64(g)})
				}
			}
		}
		return &fakeRows{columns: []string{"khoan_muc_id", "cot_id", "gia_tri"}, rows: row}, nil

	case strings.Contains(q, "SELECT bang_id FROM khoan_muc_ngan_sach"):
		if c.k.sheetIDOfLine == "" {
			return &fakeRows{columns: []string{"bang_id"}}, nil
		}
		return &fakeRows{columns: []string{"bang_id"},
			rows: [][]driver.Value{{c.k.sheetIDOfLine}}}, nil

	case strings.Contains(q, "FROM khoan_muc_ngan_sach"):
		var row [][]driver.Value
		for _, k := range c.k.lines {
			row = append(row, []driver.Value{
				k.ID, k.SheetID, k.ParentID, k.OrdinalLabel, k.Name, int64(k.SortOrder),
				string(k.Method), int64(k.Level), k.IsHeadline,
			})
		}
		return &fakeRows{columns: lineColumns(), rows: row}, nil

	case strings.Contains(q, "FROM cot_ngan_sach"):
		var row [][]driver.Value
		for _, column := range c.k.column {
			row = append(row, []driver.Value{
				column.ID, column.SheetID, column.Name, int64(column.SortOrder), string(column.Format),
				column.Formula, string(column.Indicator),
			})
		}
		return &fakeRows{columns: columnColumns(), rows: row}, nil

	case strings.Contains(q, "SELECT EXISTS"):
		return &fakeRows{columns: []string{"exists"},
			rows: [][]driver.Value{{c.k.hasLiveSheet}}}, nil

	case strings.Contains(q, "MAX(lan)"):
		return &fakeRows{columns: []string{"lan"},
			rows: [][]driver.Value{{int64(c.k.nextRevision)}}}, nil

	case strings.Contains(q, "FROM bang_ngan_sach"):
		if c.k.sheet == nil {
			return &fakeRows{columns: sheetColumns()}, nil
		}
		b := c.k.sheet
		return &fakeRows{columns: sheetColumns(), rows: [][]driver.Value{{
			b.id, b.code, int64(b.year), b.kind, int64(b.revision), b.title, b.unit,
			nil, "", nil,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// sheetColumns / columnColumns / lineColumns mirror the store's column lists IN ORDER. Written out here rather
// than imported so that reordering one of the store's lists without reordering its Scan turns this
// red too — and `khoan_muc_ngan_sach` has `tt`, `ten` and `cach_tinh` all TEXT and adjacent, a swap
// among which is invisible on any row where they happen to look alike.
func sheetColumns() []string {
	return []string{"id", "ma", "nam", "loai", "lan", "tieu_de", "don_vi_tinh",
		"luy_ke_den", "nguon_tep", "nap_luc"}
}

func columnColumns() []string {
	return []string{"id", "bang_id", "ten", "thu_tu", "kieu", "cong_thuc", "vai_tro"}
}

func entryColumns() []string {
	return []string{"id", "khoan_muc_id", "ngay", "noi_dung", "don_vi_ca_nhan",
		"so_chung_tu", "nguoi_ghi_ma", "tao_luc"}
}

func lineColumns() []string {
	return []string{"id", "bang_id", "cha_id", "tt", "ten", "thu_tu",
		"cach_tinh", "cap", "is_headline"}
}

type fakeBudgetTx struct{ k *fakeBudgetStore }

func (t *fakeBudgetTx) Commit() error {
	t.k.mu.Lock()
	t.k.commits++
	t.k.mu.Unlock()
	return nil
}

func (t *fakeBudgetTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rollbacks++
	t.k.mu.Unlock()
	return nil
}

// newBudgetUseCase builds the REAL use case over the REAL store over the fake driver, with the id
// pinned so assertions can name it.
func newBudgetUseCase(t *testing.T, k *fakeBudgetStore) (*BudgetService, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	repo := store.New(db)
	uc := NewBudgetService(repo, fistore.NewBudgetStore(repo))
	n := 0
	uc.newID = func() (string, error) {
		n++
		if n == 1 {
			return newBudgetID, nil
		}
		return fmt.Sprintf("%s-%d", newBudgetID, n), nil
	}
	return uc, tenant.Into(context.Background(), tenantA)
}

const newBudgetID = "01JNGANSACHMOIVUATAO000000"
