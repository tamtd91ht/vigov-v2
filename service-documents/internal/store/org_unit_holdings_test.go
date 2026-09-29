package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// CountOpenHeldByOrgUnit — the statement shape, without a database. What it COUNTS is proved in
// org_unit_holdings_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

// countFake answers one row of `row` (nil = no row at all) whatever the SELECT list says — the
// shared fake in document_type_test.go builds rows by column NAME and cannot answer `count(*)`.
type countFake struct {
	stmts []fakeStmt
	row   []driver.Value
	err   error
}

func (f *countFake) Connect(context.Context) (driver.Conn, error) { return &countFakeConn{f: f}, nil }
func (f *countFake) Driver() driver.Driver                        { return fakeDriver{} }

type countFakeConn struct{ f *countFake }

func (c *countFakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *countFakeConn) Close() error                        { return nil }
func (c *countFakeConn) Begin() (driver.Tx, error)           { return nil, errors.New("no tx") }
func (c *countFakeConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.f.stmts = append(c.f.stmts, fakeStmt{sql: q, args: vals})
	if c.f.err != nil {
		return nil, c.f.err
	}
	return &countFakeRows{row: c.f.row}, nil
}

type countFakeRows struct {
	row  []driver.Value
	done bool
}

func (r *countFakeRows) Columns() []string { return []string{"count"} }
func (r *countFakeRows) Close() error      { return nil }
func (r *countFakeRows) Next(dest []driver.Value) error {
	if r.done || r.row == nil {
		return io.EOF
	}
	copy(dest, r.row)
	r.done = true
	return nil
}

const (
	testTenantID = "01JTESTHOLDXAXAXAXAXAXAXAX"
	testUnitID   = "01JBOPHAN0000000000000000A"
)

func TestIncomingHeldOpenStatement(t *testing.T) {
	f := &countFake{row: []driver.Value{int64(4)}}
	n, err := NewIncomingDocumentStore(pkgstore.New(sql.OpenDB(f))).CountOpenHeldByOrgUnit(tenantCtx(testTenantID), testUnitID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("n = %d, want 4", n)
	}
	if len(f.stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(f.stmts))
	}
	l := f.stmts[0]
	want := "SELECT count(*) FROM van_ban_den WHERE tenant_id = $1 AND deleted_at IS NULL AND " +
		"bo_phan_dang_giu_id = $2 AND trang_thai NOT IN ($3, $4, $5)"
	if l.sql != want {
		t.Errorf("sql\n  %q\nwant\n  %q", l.sql, want)
	}
	// The closing codes come from domain.FinishedIncomingStatuses, bound — the same three the
	// dashboard's open figure excludes (incoming_dashboard_test.go).
	wantArgs := []driver.Value{testTenantID, testUnitID, "da-giai-quyet", "chuyen-cap-tren", "luu-khong-thu-ly"}
	if len(l.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", l.args, wantArgs)
	}
	for i := range wantArgs {
		if l.args[i] != wantArgs[i] {
			t.Errorf("arg %d = %v, want %v", i, l.args[i], wantArgs[i])
		}
	}
	if strings.Contains(l.sql, "lich_su_chuyen") || strings.Contains(l.sql, "den_bo_phan_id") {
		t.Error("reads routing history — history is not the holder")
	}
}

func TestIncomingHeldOpenRefusesBlankIDBeforeQuery(t *testing.T) {
	f := &countFake{row: []driver.Value{int64(0)}}
	if _, err := NewIncomingDocumentStore(pkgstore.New(sql.OpenDB(f))).CountOpenHeldByOrgUnit(tenantCtx(testTenantID), ""); !errors.Is(err, ErrOrgUnitIDBlank) {
		t.Errorf("err = %v, want ErrOrgUnitIDBlank", err)
	}
	if len(f.stmts) != 0 {
		t.Error("blank id still ran a statement")
	}
}

// An error is never a zero: a zero lets the delete through.
func TestIncomingHeldOpenErrorIsNotZero(t *testing.T) {
	cause := errors.New("connection lost")
	if _, err := NewIncomingDocumentStore(pkgstore.New(sql.OpenDB(&countFake{err: cause}))).
		CountOpenHeldByOrgUnit(tenantCtx(testTenantID), testUnitID); !errors.Is(err, cause) {
		t.Errorf("not wrapped with %%w: %v", err)
	}
	if _, err := NewIncomingDocumentStore(pkgstore.New(sql.OpenDB(&countFake{}))).
		CountOpenHeldByOrgUnit(tenantCtx(testTenantID), testUnitID); err == nil {
		t.Error("no row read as zero")
	}
}

func TestIncomingHeldOpenWithoutCommunePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("counted with no commune in the context")
		}
	}()
	_, _ = NewIncomingDocumentStore(pkgstore.New(sql.OpenDB(&countFake{row: []driver.Value{int64(0)}}))).
		CountOpenHeldByOrgUnit(context.Background(), testUnitID)
}
