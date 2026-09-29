package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// A fake database/sql driver that RECORDS EVERY STATEMENT AND EVERY TRANSACTION BOUNDARY — the
// shape of service-finance's driver_gia_danh_muc_test.go, copied (rule 2, forbidden #1).
//
// It exists so that "the audit entry is in the same transaction as the override" and "a failed
// audit entry rolls the override back" are properties of real core/store Begin/Commit/Rollback
// calls, not of a fake that says so. The override store in these tests is an in-memory fake; the
// only statement that reaches this driver is core/audit's INSERT.
//
// WHAT IT DOES NOT PROVE: anything PostgreSQL does — not the key CHECK, not UNIQUE (tenant_id,
// live_key), not the partition routing. No PostgreSQL is reachable from this build environment.

type recordedStmt struct {
	sql  string
	args []driver.Value
}

type recordingDriver struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begins, commits, rollbacks int

	// failOn fails the FIRST statement containing this substring, and only that one — so a failure
	// on the LAST statement of a transaction (the audit entry) can be told apart from "never started".
	failOn string
	failed bool
}

func (d *recordingDriver) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, recordedStmt{sql: q, args: vals})
	d.mu.Unlock()
}

// matching returns every recorded statement containing s.
func (d *recordingDriver) matching(s string) []recordedStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []recordedStmt
	for _, st := range d.stmts {
		if strings.Contains(st.sql, s) {
			out = append(out, st)
		}
	}
	return out
}

func (d *recordingDriver) has(s string) bool { return len(d.matching(s)) > 0 }

func (d *recordingDriver) shouldFail(q string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failOn != "" && !d.failed && strings.Contains(q, d.failOn) {
		d.failed = true
		return errors.New("recording driver: this statement is built to fail")
	}
	return nil
}

func (d *recordingDriver) Connect(context.Context) (driver.Conn, error) { return &recordingConn{d: d}, nil }
func (d *recordingDriver) Driver() driver.Driver                        { return recordingOpener{} }

type recordingOpener struct{}

func (recordingOpener) Open(string) (driver.Conn, error) {
	return nil, errors.New("recording driver: use the Connector")
}

type recordingConn struct{ d *recordingDriver }

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("recording driver: Prepare is not supported")
}
func (c *recordingConn) Close() error { return nil }

func (c *recordingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *recordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begins++
	c.d.mu.Unlock()
	return &recordingTx{d: c.d}, nil
}

func (c *recordingConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if err := c.d.shouldFail(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *recordingConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	return nil, fmt.Errorf("recording driver: no query is expected here, got %q", q)
}

type recordingTx struct{ d *recordingDriver }

func (t *recordingTx) Commit() error {
	t.d.mu.Lock()
	t.d.commits++
	t.d.mu.Unlock()
	return nil
}

func (t *recordingTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rollbacks++
	t.d.mu.Unlock()
	return nil
}
