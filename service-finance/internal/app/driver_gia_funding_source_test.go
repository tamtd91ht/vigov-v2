package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// A fake database/sql driver for the funding source catalogue — the REAL FundingSourceWriteStore runs
// on top of it, for the reason the other three drivers give: "the source, its year amount and the
// audit entry are in ONE transaction", "a refusal commits nothing", "the commune is $1 of every
// statement" are properties of the SQL and the transaction boundaries, and a fake store erases them.
//
// WHAT IT DOES NOT PROVE: anything PostgreSQL does — `nguon_von_name_unique`, the trimmed-name CHECK,
// the partition routing, FOR UPDATE actually blocking. That half is in store/funding_source_pg_test.go
// and runs only with VIGOV_TEST_DSN.

type fakeFundingSourceDB struct {
	mu         sync.Mutex
	statements []lenhGhi

	begun, committed, rolledBack int

	nameCount int64 // what the name check counts (soft-deleted rows included)
	liveCount int64 // what the ceiling check counts
	nextOrder int64 // what INSERT … RETURNING thu_tu answers

	// sourceName is the live source the FOR UPDATE read finds; "" = no such source in this commune.
	sourceName string

	// amount is the existing (source, year) row; nil = nothing entered for the year.
	amount *int64

	// insertSourceErr fails the INSERT INTO nguon_von (to simulate a concurrent create of the name).
	insertSourceErr error

	// failOn fails the FIRST statement containing this substring, and only that one.
	failOn string
	failed bool
}

func (k *fakeFundingSourceDB) record(q string, args []driver.NamedValue) error {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.statements = append(k.statements, lenhGhi{sql: q, args: vals})
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return fmt.Errorf("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeFundingSourceDB) stmts(sub string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []lenhGhi
	for _, l := range k.statements {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *fakeFundingSourceDB) Connect(context.Context) (driver.Conn, error) {
	return &fakeFundingSourceConn{k: k}, nil
}
func (k *fakeFundingSourceDB) Driver() driver.Driver { return trinhGia{} }

type fakeFundingSourceConn struct{ k *fakeFundingSourceDB }

func (c *fakeFundingSourceConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("driver giả: không hỗ trợ Prepare")
}
func (c *fakeFundingSourceConn) Close() error { return nil }
func (c *fakeFundingSourceConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeFundingSourceConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &fakeFundingSourceTx{k: c.k}, nil
}

func (c *fakeFundingSourceConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *fakeFundingSourceConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	one := func(col string, v driver.Value) driver.Rows {
		return &rowsGia{cot: []string{col}, hang: [][]driver.Value{{v}}}
	}
	switch {
	// ORDER MATTERS: both checks are count(*) on nguon_von.
	case strings.Contains(q, "count(*) FROM nguon_von") && strings.Contains(q, "ten = $2"):
		return one("count", c.k.nameCount), nil
	case strings.Contains(q, "count(*) FROM nguon_von"):
		return one("count", c.k.liveCount), nil
	case strings.Contains(q, "INSERT INTO nguon_von"):
		if c.k.insertSourceErr != nil {
			return nil, c.k.insertSourceErr
		}
		return one("thu_tu", c.k.nextOrder), nil
	case strings.Contains(q, "FROM nguon_von") && strings.Contains(q, "FOR UPDATE"):
		if c.k.sourceName == "" {
			return &rowsGia{cot: []string{"id", "ten", "thu_tu"}}, nil
		}
		return &rowsGia{cot: []string{"id", "ten", "thu_tu"},
			hang: [][]driver.Value{{fmt.Sprint(args[1].Value), c.k.sourceName, int64(3)}}}, nil
	case strings.Contains(q, "FROM funding_source_annual_amounts"):
		if c.k.amount == nil {
			return &rowsGia{cot: []string{"id", "granted_amount"}}, nil
		}
		return &rowsGia{cot: []string{"id", "granted_amount"},
			hang: [][]driver.Value{{idAmountExisting, *c.k.amount}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

type fakeFundingSourceTx struct{ k *fakeFundingSourceDB }

func (t *fakeFundingSourceTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	return nil
}

func (t *fakeFundingSourceTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	return nil
}

const (
	idSourceNew      = "01JNGUONVONMOI00000000000"
	idAmountNew      = "01JVONDUOCGIAOMOI00000000"
	idAmountExisting = "01JVONDUOCGIAOCU000000000"
)

// uniqueViolation is what pgx hands back when `nguon_von_name_unique` fires.
var uniqueViolation = &pgconn.PgError{Code: "23505"}

// buildFundingSources wires the REAL use case over the REAL store over the fake driver, one
// connection so every statement of a transaction lands in order. The id generator hands out the
// source id first and the amount id second, so assertions can tell the two apart.
func buildFundingSources(t *testing.T, k *fakeFundingSourceDB) (*FundingSources, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	scoped := store.New(db)
	uc := NewFundingSources(scoped, fistore.NewFundingSourceWriteStore(scoped))
	ids := []string{idSourceNew, idAmountNew}
	n := 0
	uc.newID = func() (string, error) {
		id := ids[n%len(ids)]
		n++
		return id, nil
	}
	return uc, tenant.Into(context.Background(), xaA)
}
