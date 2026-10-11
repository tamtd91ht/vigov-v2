package app

// adrDB is a fake database/sql driver for the use cases added with ADR 0091 (disaster alerts, the
// citizen notification switch): it records every statement with its arguments and the transaction
// boundaries, and answers each query through a function the test supplies. The SQL and the
// transaction are what is worth proving here; what PostgreSQL decides (CHECKs, triggers) is the pg
// suites'.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
)

type adrStmt struct {
	sql  string
	args []driver.Value
}

type adrDB struct {
	mu    sync.Mutex
	stmts []adrStmt

	begun, committed, rolledBack int

	// answer returns the columns and rows for a query; nil means "no rows".
	answer func(q string) ([]string, [][]driver.Value)
	// failOn makes any statement containing it fail.
	failOn string
}

func (d *adrDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, adrStmt{sql: q, args: vals})
	d.mu.Unlock()
}

func (d *adrDB) with(sub string) []adrStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []adrStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *adrDB) indexOf(sub string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

func (d *adrDB) handle(t *testing.T) *store.DB {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

// auditDeltaOf decodes the delta ($8) of the single audit entry.
func (d *adrDB) auditEntry(t *testing.T) (adrStmt, map[string]any) {
	t.Helper()
	entries := d.with("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d audit entries, want 1", len(entries))
	}
	raw, _ := entries[0].args[7].([]byte)
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatalf("delta is not JSON: %v", err)
	}
	return entries[0], delta
}

func (d *adrDB) Connect(context.Context) (driver.Conn, error) { return &adrConn{d: d}, nil }
func (d *adrDB) Driver() driver.Driver                        { return adrDriver{} }

type adrDriver struct{}

func (adrDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type adrConn struct{ d *adrDB }

func (c *adrConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *adrConn) Close() error { return nil }
func (c *adrConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *adrConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &adrTx{d: c.d}, nil
}

func (c *adrConn) fail(q string) error {
	if c.d.failOn != "" && strings.Contains(q, c.d.failOn) {
		return errors.New("fake driver: this statement is built to fail")
	}
	return nil
}

func (c *adrConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if err := c.fail(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *adrConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	if err := c.fail(q); err != nil {
		return nil, err
	}
	if c.d.answer == nil {
		return nil, fmt.Errorf("fake driver: no answer for %q", q)
	}
	cols, rows := c.d.answer(q)
	if cols == nil {
		cols = []string{"x"}
	}
	return &adrRows{cols: cols, rows: rows}, nil
}

type adrTx struct{ d *adrDB }

func (t *adrTx) Commit() error {
	t.d.mu.Lock()
	t.d.committed++
	t.d.mu.Unlock()
	return nil
}

func (t *adrTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rolledBack++
	t.d.mu.Unlock()
	return nil
}

type adrRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *adrRows) Columns() []string { return r.cols }
func (r *adrRows) Close() error      { return nil }
func (r *adrRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}
