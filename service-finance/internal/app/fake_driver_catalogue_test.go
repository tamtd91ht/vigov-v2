package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	docstore "github.com/vihat/vigov/service-finance/internal/store"
)

// A fake database/sql driver that RECORDS EVERY STATEMENT AND EVERY TRANSACTION BOUNDARY.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE. The properties this
// package is responsible for are not "the use case called a method": they are
//
//	the business write and its audit entry are in ONE transaction  (rule 6, invariant 3)
//	a refusal leaves the transaction with nothing committed
//	`nguon` is never written from anything a request could reach   (ADR 0024, the tier model)
//	a soft delete writes all three of rule 7 invariant 1's columns
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that. Running docstore.CapitalPlanCategoryStore unchanged over this driver keeps the
// statements real while needing no PostgreSQL — and a test that needs infrastructure is a test that
// stops being run (ADR 0013, last section: VIGOV_TEST_DSN is unset here).
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the `danh_muc_ba_tang` trigger, not
// `UNIQUE (tenant_id, ma)`, not the partition routing. That half needs a real server; the pg suites
// next door are where it lands the day a DSN exists.

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type recordedStmt struct {
	sql  string
	args []driver.Value
}

// categoryRow is the row the FOR UPDATE read hands back. A nil *categoryRow means "no such live row".
type categoryRow struct {
	id, code, label string
	isActive        bool
	isDefault       bool
	sortOrder       int
	source          string
	branched        bool
}

type fakeStore struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begins, commits, rollbacks int

	// count answers the ceiling check, codeTaken the duplicate check. Separate fields because the two
	// statements ask different questions and a single counter would make one of them untestable.
	count     int
	codeTaken int

	row *categoryRow
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

func (k *fakeStore) write(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, recordedStmt{sql: q, args: vals})
	k.mu.Unlock()
}

// stmtsContaining returns every recorded statement containing `substr`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *fakeStore) stmtsContaining(substr string) []recordedStmt {
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

func (k *fakeStore) hasStmt(substr string) bool { return len(k.stmtsContaining(substr)) > 0 }

// failFor decides whether this statement is the one that fails.
func (k *fakeStore) failFor(q string) error {
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

func (k *fakeStore) Connect(context.Context) (driver.Conn, error) { return &fakeConn{k: k}, nil }
func (k *fakeStore) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeConn struct{ k *fakeStore }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begins++
	c.k.mu.Unlock()
	return &fakeTx{k: c.k}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.expectOneRow turns zero into "not found", and a fake returning
	// zero would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

func (c *fakeConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	switch {
	// ORDER MATTERS: the duplicate check is also a count(*), so it has to be recognised first.
	case strings.Contains(q, "count(*)") && strings.Contains(q, "ma = $2"):
		return &fakeRows{columns: []string{"count"}, rows: [][]driver.Value{{int64(c.k.codeTaken)}}}, nil
	case strings.Contains(q, "count(*)"):
		return &fakeRows{columns: []string{"count"}, rows: [][]driver.Value{{int64(c.k.count)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.row == nil {
			return &fakeRows{columns: categoryColumns()}, nil
		}
		h := *c.k.row
		return &fakeRows{columns: categoryColumns(), rows: [][]driver.Value{{
			h.id, h.code, h.label, h.isActive, h.isDefault, int64(h.sortOrder), h.source, h.branched,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// categoryColumns mirrors docstore's column list ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — the store's own
// suite makes the same argument for the same reason.
func categoryColumns() []string {
	return []string{"id", "ma", "nhan", "dang_dung", "la_mac_dinh", "thu_tu", "nguon", "ma_nguon_re_nhanh"}
}

type fakeTx struct{ k *fakeStore }

func (t *fakeTx) Commit() error {
	t.k.mu.Lock()
	t.k.commits++
	t.k.mu.Unlock()
	return nil
}

func (t *fakeTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rollbacks++
	t.k.mu.Unlock()
	return nil
}

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	i       int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// newCategoryUseCase builds the REAL use case over the REAL store over the fake driver, with the id pinned
// so assertions can name it.
func newCategoryUseCase(t *testing.T, k *fakeStore) (*CapitalPlanCategoryService, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	repo := store.New(db)
	uc := NewCapitalPlanCategoryService(repo, docstore.NewCapitalPlanCategoryStore(repo))
	uc.newID = func() (string, error) { return "01JIDMOICUADONGVUATAO0000", nil }
	return uc, tenant.Into(context.Background(), tenantA)
}
