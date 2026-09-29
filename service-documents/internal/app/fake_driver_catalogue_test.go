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
	docstore "github.com/vihat/vigov/service-documents/internal/store"
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
// erases exactly that. Running docstore.DocumentTypeStore unchanged over this driver keeps the
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

// documentTypeRow is the row the FOR UPDATE read hands back. A nil *documentTypeRow means "no such
// live row".
type documentTypeRow struct {
	id, code, label string
	isActive        bool
	isDefault       bool
	sortOrder       int
	source          string
	branched        bool
}

type fakeCatalogueDB struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begun, committed, rolledBack int

	// liveCount answers the ceiling check, codeMatches the duplicate check. Separate fields because
	// the two statements ask different questions and a single counter would make one of them
	// untestable.
	liveCount   int
	codeMatches int

	row *documentTypeRow
	err error

	// snapshot answers the Excel import's snapshot read: rows of (ma, nhan, da_xoa).
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

func (k *fakeCatalogueDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, recordedStmt{sql: q, args: vals})
	k.mu.Unlock()
}

// stmtsContaining returns every recorded statement containing `sub`, so an assertion names the
// statement it cares about rather than an index that shifts when a check is added.
func (k *fakeCatalogueDB) stmtsContaining(sub string) []recordedStmt {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []recordedStmt
	for _, l := range k.stmts {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *fakeCatalogueDB) hasStmt(sub string) bool { return len(k.stmtsContaining(sub)) > 0 }

// maybeFail decides whether this statement is the one that fails.
func (k *fakeCatalogueDB) maybeFail(q string) error {
	if k.err != nil {
		return k.err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeCatalogueDB) Connect(context.Context) (driver.Conn, error) {
	return &fakeCatalogueConn{k: k}, nil
}
func (k *fakeCatalogueDB) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeCatalogueConn struct{ k *fakeCatalogueDB }

func (c *fakeCatalogueConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeCatalogueConn) Close() error { return nil }

func (c *fakeCatalogueConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeCatalogueConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &fakeCatalogueTx{k: c.k}, nil
}

func (c *fakeCatalogueConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.requireOneRow turns zero into "not found", and a fake returning
	// zero would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

func (c *fakeCatalogueConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "deleted_at IS NOT NULL FROM loai_van_ban"):
		return &fakeRows{cols: []string{"ma", "nhan", "da_xoa"}, rows: c.k.snapshot}, nil
	// ORDER MATTERS: the duplicate check is also a count(*), so it has to be recognised first.
	case strings.Contains(q, "count(*)") && strings.Contains(q, "ma = $2"):
		return &fakeRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.k.codeMatches)}}}, nil
	case strings.Contains(q, "count(*)"):
		return &fakeRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.k.liveCount)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.row == nil {
			return &fakeRows{cols: documentTypeCols()}, nil
		}
		r := *c.k.row
		return &fakeRows{cols: documentTypeCols(), rows: [][]driver.Value{{
			r.id, r.code, r.label, r.isActive, r.isDefault, int64(r.sortOrder), r.source, r.branched,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// documentTypeCols mirrors docstore's column list ORDER. Written out here rather than imported so
// that reordering the store's list without reordering its Scan turns this red too — the store's own
// suite makes the same argument for the same reason.
func documentTypeCols() []string {
	return []string{"id", "ma", "nhan", "dang_dung", "la_mac_dinh", "thu_tu", "nguon", "ma_nguon_re_nhanh"}
}

type fakeCatalogueTx struct{ k *fakeCatalogueDB }

func (t *fakeCatalogueTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	return nil
}

func (t *fakeCatalogueTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	return nil
}

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// newCatalogueUseCase builds the REAL use case over the REAL store over the fake driver, with the id
// pinned so assertions can name it.
func newCatalogueUseCase(t *testing.T, k *fakeCatalogueDB) (*DocumentTypeCatalogue, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	storeDB := store.New(db)
	uc := NewDocumentTypeCatalogue(storeDB, docstore.NewDocumentTypeStore(storeDB))
	uc.newID = func() (string, error) { return "01JIDMOICUADONGVUATAO0000", nil }
	return uc, tenant.Into(context.Background(), tenantA)
}
