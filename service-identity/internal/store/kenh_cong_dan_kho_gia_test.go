package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The fake database/sql driver the citizen-channel store tests share.
//
// WHAT IT PROVES, AND WHAT IT CANNOT — stated here because a suite that prints `ok` while
// asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL. It RECORDS the statement the store built and hands back rows the
// test supplied, BUILT BY COLUMN NAME out of the statement's own SELECT list. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT · the
//	              soft-delete predicate is in the statement · the ORDER BY and its tie-break are
//	              in the statement · the bound is ceiling+1 · the positional Scan lines up with
//	              the column list BY NAME · a driver failure is wrapped rather than swallowed ·
//	              every write runs inside the caller's transaction.
//	NOT PROVED    anything PostgreSQL does with that statement: that the CHECK constraints
//	              refuse what they say they refuse, that the partition routing works, that
//	              `make_interval` and `now()` mean what this code assumes, that UPDATE ... WHERE
//	              dung_luc IS NULL really serialises two racing redeems. That needs a real
//	              server, and the *_pg_test.go files next door are where it is asserted — they
//	              SKIP without VIGOV_TEST_DSN, and the package still prints `ok`.
//
// THE COLUMN-NAME CHECK IS THE ONE WORTH EXPLAINING: rows are assembled by looking each column
// of the statement up in a sample map, so reordering a SELECT list without reordering the Scan
// turns these tests red. Read by position, `xet_duyet_boi`/`ly_do_tu_choi` are two adjacent
// TEXT columns whose swap produces no error at all — an officer's account id would be shown to
// a citizen as the reason their declaration was refused.

// lenhKC is one statement the driver saw, with the transaction it ran in (0 = none).
type lenhKC struct {
	sql  string
	args []driver.Value
	tx   int
}

type khoKC struct {
	lenh []lenhKC

	// hang is the rows the next read returns, keyed by column name.
	hang []map[string]driver.Value

	// loi, when set, fails every statement — the "database did not answer" case.
	loi error

	// soDong is what RowsAffected reports for a write. 1 is the happy path; 0 is how a
	// predicate refusing looks from Go, which is the whole point of the single-use test.
	soDong int64

	soTx    int
	ketThuc map[int]string
}

func khoMoi() *khoKC {
	return &khoKC{soDong: 1, ketThuc: map[int]string{}}
}

// dbGia wires the fake behind a real *sql.DB, so the code under test uses the ordinary
// database/sql path and nothing is stubbed out inside the store.
func dbGia(k *khoKC) *pkgstore.DB { return pkgstore.New(sql.OpenDB(ketNoiKC{k: k})) }

func (k *khoKC) chay(l lenhKC) error {
	k.lenh = append(k.lenh, l)
	return k.loi
}

// cuoi returns the last statement the driver saw.
func (k *khoKC) cuoi() lenhKC { return k.lenh[len(k.lenh)-1] }

type ketNoiKC struct{ k *khoKC }

func (c ketNoiKC) Connect(context.Context) (driver.Conn, error) { return &connKC{k: c.k}, nil }
func (c ketNoiKC) Driver() driver.Driver                        { return trinhKC{} }

type trinhKC struct{}

func (trinhKC) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connKC struct {
	k  *khoKC
	tx int
}

func (c *connKC) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connKC) Close() error { return nil }

func (c *connKC) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connKC) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.soTx++
	c.tx = c.k.soTx
	return &txKC{c: c, id: c.tx}, nil
}

func (c *connKC) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.chay(lenhKC{sql: q, args: giaTriKC(args), tx: c.tx}); err != nil {
		return nil, err
	}
	return driver.RowsAffected(c.k.soDong), nil
}

func (c *connKC) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.chay(lenhKC{sql: q, args: giaTriKC(args), tx: c.tx}); err != nil {
		return nil, err
	}
	cot, err := cotTraVe(q)
	if err != nil {
		return nil, err
	}
	dong := make([][]driver.Value, 0, len(c.k.hang))
	for _, h := range c.k.hang {
		mot := make([]driver.Value, len(cot))
		for i, ten := range cot {
			v, co := h[ten]
			if !co {
				// A column was added to the SELECT list and not to the sample row. Failing
				// loudly beats scanning a nil that "passes" while proving nothing.
				return nil, fmt.Errorf("driver giả: không có giá trị mẫu cho cột %q", ten)
			}
			mot[i] = v
		}
		dong = append(dong, mot)
	}
	return &rowsKC{cot: cot, hang: dong}, nil
}

// cotTraVe reads the columns a statement hands back — out of its SELECT list, or out of its
// RETURNING clause for an INSERT. Both are read from the statement the STORE built, so these
// tests are pinned to the real column lists rather than to a copy of them.
func cotTraVe(q string) ([]string, error) {
	if strings.Contains(q, "SELECT ") {
		return cotTrongCauLenh(q)
	}
	i := strings.Index(q, "RETURNING ")
	if i < 0 {
		return nil, fmt.Errorf("driver giả: câu lệnh không trả cột nào: %q", q)
	}
	var ra []string
	for _, bieu := range tachDauPhay(q[i+len("RETURNING "):]) {
		ra = append(ra, tenCot(bieu))
	}
	return ra, nil
}

type txKC struct {
	c  *connKC
	id int
}

func (t *txKC) Commit() error   { return t.dong("commit") }
func (t *txKC) Rollback() error { return t.dong("rollback") }

func (t *txKC) dong(sao string) error {
	t.c.k.ketThuc[t.id] = sao
	t.c.tx = 0
	return nil
}

type rowsKC struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsKC) Columns() []string { return r.cot }
func (r *rowsKC) Close() error      { return nil }
func (r *rowsKC) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func giaTriKC(args []driver.NamedValue) []driver.Value {
	ra := make([]driver.Value, 0, len(args))
	for _, a := range args {
		ra = append(ra, a.Value)
	}
	return ra
}
