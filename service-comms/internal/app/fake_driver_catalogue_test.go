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
	docstore "github.com/vihat/vigov/service-comms/internal/store"
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
// erases exactly that. Running docstore.MapAssetTypeStore unchanged over this driver keeps the
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

// fakeMapAssetTypeRow is the row the FOR UPDATE read hands back. A nil *fakeMapAssetTypeRow means
// "no such live row".
type fakeMapAssetTypeRow struct {
	id, code, label string
	isActive        bool
	isDefault       bool
	sortOrder       int
	source          string
	branched        bool
}

type catalogueDB struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begun, committed, rolledBack int

	// count answers the ceiling check, codeTaken the duplicate check. Separate fields because the two
	// statements ask different questions and a single counter would make one of them untestable.
	count     int
	codeTaken int

	row *fakeMapAssetTypeRow
	err error

	// snapshot answers the import's snapshot read: rows of (ma, nhan, da_xoa).
	snapshot [][]driver.Value

	// failOn fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `err`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business write down with it, and that needs everything before it to
	// have already run.
	failOn string
	failed bool
}

func (d *catalogueDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, recordedStmt{sql: q, args: vals})
	d.mu.Unlock()
}

// with returns every recorded statement containing `sub`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (d *catalogueDB) with(sub string) []recordedStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []recordedStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *catalogueDB) ran(sub string) bool { return len(d.with(sub)) > 0 }

// maybeFail decides whether this statement is the one that fails.
func (d *catalogueDB) maybeFail(q string) error {
	if d.err != nil {
		return d.err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failOn != "" && !d.failed && strings.Contains(q, d.failOn) {
		d.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (d *catalogueDB) Connect(context.Context) (driver.Conn, error) { return &catalogueConn{d: d}, nil }
func (d *catalogueDB) Driver() driver.Driver                        { return catalogueDriverOpen{} }

type catalogueDriverOpen struct{}

func (catalogueDriverOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type catalogueConn struct{ d *catalogueDB }

func (c *catalogueConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *catalogueConn) Close() error { return nil }

func (c *catalogueConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *catalogueConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &catalogueTx{d: c.d}, nil
}

func (c *catalogueConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if err := c.d.maybeFail(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.expectOneRow turns zero into "not found", and a fake returning
	// zero would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

func (c *catalogueConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	if err := c.d.maybeFail(q); err != nil {
		return nil, err
	}
	switch {
	// ORDER MATTERS: the duplicate check is also a count(*), so it has to be recognised first.
	case strings.Contains(q, "count(*)") && strings.Contains(q, "ma = $2"):
		return &catalogueRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.d.codeTaken)}}}, nil
	case strings.Contains(q, "count(*)"):
		return &catalogueRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.d.count)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.d.row == nil {
			return &catalogueRows{cols: mapAssetTypeCols()}, nil
		}
		r := *c.d.row
		return &catalogueRows{cols: mapAssetTypeCols(), rows: [][]driver.Value{{
			r.id, r.code, r.label, int64(r.sortOrder), r.isDefault, r.isActive, r.source, r.branched,
		}}}, nil
	case strings.Contains(q, "deleted_at IS NOT NULL FROM loai_tai_nguyen_ban_do"):
		return &catalogueRows{cols: []string{"ma", "nhan", "da_xoa"}, rows: c.d.snapshot}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// mapAssetTypeCols mirrors docstore's column list ORDER (mapAssetTypeColumns). Written out here
// rather than imported, AND CHECKED AGAINST THE STATEMENT: TestFakeDriverMirrorsTheSelectedColumns
// reads the SELECT list of the FOR UPDATE statement the store actually sent. Until 2026-09-29 this
// list mirrored the store's (wrong) Scan order instead of its SELECT, so a Scan that could never
// succeed on PostgreSQL passed here.
func mapAssetTypeCols() []string {
	return []string{"id", "ma", "nhan", "thu_tu", "la_mac_dinh", "dang_dung", "nguon", "ma_nguon_re_nhanh"}
}

type catalogueTx struct{ d *catalogueDB }

func (t *catalogueTx) Commit() error {
	t.d.mu.Lock()
	t.d.committed++
	t.d.mu.Unlock()
	return nil
}

func (t *catalogueTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rolledBack++
	t.d.mu.Unlock()
	return nil
}

type catalogueRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *catalogueRows) Columns() []string { return r.cols }
func (r *catalogueRows) Close() error      { return nil }
func (r *catalogueRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// newTestCatalogue builds the REAL use case over the REAL store over the fake driver, with the id
// pinned so assertions can name it.
func newTestCatalogue(t *testing.T, d *catalogueDB) (*MapAssetTypeCatalogue, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	s := store.New(db)
	uc := NewMapAssetTypeCatalogue(s, docstore.NewMapAssetTypeStore(s))
	uc.newID = func() (string, error) { return "01JIDMOICUADONGVUATAO0000", nil }
	return uc, tenant.Into(context.Background(), tenantA)
}
