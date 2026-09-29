package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// A SECOND fake driver in this package, and the duplication is deliberate rather than laziness.
//
// The one in map_asset_type_test.go serves a READ-ONLY store: its connection refuses
// Begin outright, and its row type has one fixed shape. The notification ledger writes, inside a
// transaction, and the properties worth proving are about that transaction — that the insert and
// the audit entry cannot end up in two of them, that a redelivery affects no row. Widening the
// first fake to cover both would give one type two jobs and, worse, would let a change made for
// this suite silently weaken the catalogue suite's assertions.
//
// WHAT THIS FAKE PROVES, AND WHAT IT DOES NOT — stated first, because a package that prints `ok`
// while asserting nothing is this repository's worst known trap:
//
//	PROVED HERE   the commune reaches every statement bound from the CONTEXT, never from an
//	              argument · the insert really says ON CONFLICT DO NOTHING · a conflict (zero rows
//	              affected) writes no audit entry and is not an error · the update's predicate
//	              excludes rows already delivered · the read filters soft-deleted rows, matches the
//	              business code exactly, orders totally, and takes ceiling+1 · the positional scan
//	              lines up with the column list BY NAME · errors are wrapped, not swallowed.
//
//	NOT PROVED    anything PostgreSQL does with those statements: the UNIQUE constraint that makes
//	              ON CONFLICT mean anything, the CHECK constraints, the immutability trigger, the
//	              hash partition routing. This fake reports whatever row count the test tells it
//	              to. That is what citizen_notification_pg_test.go is for, and on a machine with
//	              no VIGOV_TEST_DSN that file skips in full.

// --- recorded statements ---------------------------------------------------------------------

type recordedStmt struct {
	sql  string
	args []driver.Value
}

// fakeNotificationRow is one ledger row the fake returns. Every value is distinguishable from every
// other, so a mis-wired Scan shows up as WRONG DATA rather than as a zero value that looks
// plausible. In particular `subjectCode` and `milestone` are adjacent TEXT columns, as are
// `recipientCode` and `maskedRecipient`: swapping either pair compiles, runs, and is wrong.
type fakeNotificationRow struct {
	id, key, subjectType, subjectCode, milestone string
	round                                        int64
	channel, recipientCode                       string
	maskedRecipient                              any // nil when the send has not been attempted
	templateCode                                 string
	params                                       []byte
	status                                       string
	attemptCount                                 int64
	sentAt                                       any // nil until delivered
	errorCode                                    any // nil when nothing failed
	createdAt                                    time.Time
}

func (r fakeNotificationRow) value(col string) driver.Value {
	switch col {
	case "id":
		return r.id
	case "khoa_lan_gui":
		return r.key
	case "doi_tuong_loai":
		return r.subjectType
	case "doi_tuong_ma":
		return r.subjectCode
	case "moc":
		return r.milestone
	case "lan":
		return r.round
	case "kenh":
		return r.channel
	case "nguoi_nhan_ma":
		return r.recipientCode
	case "nguoi_nhan_che":
		return r.maskedRecipient
	case "mau_ma":
		return r.templateCode
	case "tham_so":
		return r.params
	case "trang_thai":
		return r.status
	case "so_lan_thu":
		return r.attemptCount
	case "gui_luc":
		return r.sentAt
	case "loi_ma":
		return r.errorCode
	case "tao_luc":
		return r.createdAt
	default:
		// A column was added to citizenNotificationColumns and not here. Failing loudly beats scanning a nil that
		// "passes" while proving nothing.
		panic("driver ghi giả: không có giá trị mẫu cho cột " + col)
	}
}

// fakeWriteStore is the fake database. Everything a test needs to steer is a field here.
type fakeWriteStore struct {
	stmts []recordedStmt
	rows  []fakeNotificationRow

	// rowsAffected is what Exec reports as RowsAffected — the ONE number that decides whether a
	// write is treated as new or as a redelivery. 1 by default; a test sets 0 to mean "the
	// UNIQUE constraint swallowed it" or "the row is already delivered".
	rowsAffected int64

	// rowExists answers the existence probe behind ErrCitizenNotificationNotFound.
	rowExists bool

	queryErr error
	execErr  error
	beginErr error

	// begun / committed / rolledBack record the transaction's life. Rule 6 invariant 3 is a statement
	// about a transaction, so a suite that never looks at one cannot check it.
	begun, committed, rolledBack int
}

func (f *fakeWriteStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeWriteConn{f: f}, nil
}
func (f *fakeWriteStore) Driver() driver.Driver { return fakeWriteDriver{} }

type fakeWriteDriver struct{}

func (fakeWriteDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver ghi giả: chỉ dùng Connector")
}

type fakeWriteConn struct{ f *fakeWriteStore }

func (c *fakeWriteConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver ghi giả: không hỗ trợ Prepare")
}
func (c *fakeWriteConn) Close() error { return nil }
func (c *fakeWriteConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeWriteConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.f.beginErr != nil {
		return nil, c.f.beginErr
	}
	c.f.begun++
	return &fakeTx{f: c.f}, nil
}

type fakeTx struct{ f *fakeWriteStore }

func (t *fakeTx) Commit() error   { t.f.committed++; return nil }
func (t *fakeTx) Rollback() error { t.f.rolledBack++; return nil }

func argValues(args []driver.NamedValue) []driver.Value {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	return vals
}

type fakeResult struct{ n int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, errors.New("không dùng") }
func (r fakeResult) RowsAffected() (int64, error) { return r.n, nil }

func (c *fakeWriteConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.execErr != nil {
		return nil, c.f.execErr
	}
	return fakeResult{n: c.f.rowsAffected}, nil
}

func (c *fakeWriteConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.queryErr != nil {
		return nil, c.f.queryErr
	}

	// The existence probe selects a literal, not columns. Answering it from `rowExists` keeps the two
	// outcomes of RecordSendResult — "already delivered" and "no such row" — separable in a test, which
	// is the whole reason the store distinguishes them.
	if strings.Contains(q, "SELECT 1 FROM") {
		if !c.f.rowExists {
			return &fakeWriteRows{cols: []string{"?column?"}}, nil
		}
		return &fakeWriteRows{cols: []string{"?column?"}, rows: [][]driver.Value{{int64(1)}}}, nil
	}

	cols, err := selectColumnsWrite(q)
	if err != nil {
		return nil, err
	}
	out := make([][]driver.Value, 0, len(c.f.rows))
	for _, r := range c.f.rows {
		one := make([]driver.Value, len(cols))
		for i, name := range cols {
			one[i] = r.value(name)
		}
		out = append(out, one)
	}
	return &fakeWriteRows{cols: cols, rows: out}, nil
}

// selectColumnsWrite reads the SELECT list out of the statement. THIS IS WHAT MAKES THE COLUMN
// CHECK WORK: the row is assembled BY NAME from the columns the store asked for, so reordering
// citizenNotificationColumns without reordering the Scan in scanCitizenNotification turns this suite red.
func selectColumnsWrite(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver ghi giả: không đọc được danh sách cột từ %q", q)
	}
	var out []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		out = append(out, strings.TrimSpace(c))
	}
	return out, nil
}

type fakeWriteRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fakeWriteRows) Columns() []string { return r.cols }
func (r *fakeWriteRows) Close() error      { return nil }
func (r *fakeWriteRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func openFakeWriteDB(f *fakeWriteStore) *sql.DB { return sql.OpenDB(f) }
