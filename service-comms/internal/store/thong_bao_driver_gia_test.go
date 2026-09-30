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
// The one in loai_tai_nguyen_ban_do_test.go serves a READ-ONLY store: its connection refuses
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
//	              to. That is what thong_bao_gui_cong_dan_pg_test.go is for, and on a machine with
//	              no VIGOV_TEST_DSN that file skips in full.

// --- recorded statements ---------------------------------------------------------------------

type lenhGhi struct {
	sql  string
	args []driver.Value
}

// dongThongBao is one ledger row the fake returns. Every value is distinguishable from every
// other, so a mis-wired Scan shows up as WRONG DATA rather than as a zero value that looks
// plausible. In particular `doiTuongMa` and `moc` are adjacent TEXT columns, as are
// `nguoiNhanMa` and `nguoiNhanChe`: swapping either pair compiles, runs, and is wrong.
type dongThongBao struct {
	id, khoa, loai, doiTuongMa, moc string
	lan                             int64
	kenh, nguoiNhanMa               string
	nguoiNhanChe                    any // nil when the send has not been attempted
	mauMa                           string
	thamSo                          []byte
	trangThai                       string
	soLanThu                        int64
	guiLuc                          any // nil until delivered
	loiMa                           any // nil when nothing failed
	taoLuc                          time.Time
}

func (d dongThongBao) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return d.id
	case "khoa_lan_gui":
		return d.khoa
	case "doi_tuong_loai":
		return d.loai
	case "doi_tuong_ma":
		return d.doiTuongMa
	case "moc":
		return d.moc
	case "lan":
		return d.lan
	case "kenh":
		return d.kenh
	case "nguoi_nhan_ma":
		return d.nguoiNhanMa
	case "nguoi_nhan_che":
		return d.nguoiNhanChe
	case "mau_ma":
		return d.mauMa
	case "tham_so":
		return d.thamSo
	case "trang_thai":
		return d.trangThai
	case "so_lan_thu":
		return d.soLanThu
	case "gui_luc":
		return d.guiLuc
	case "loi_ma":
		return d.loiMa
	case "tao_luc":
		return d.taoLuc
	default:
		// A column was added to cotThongBao and not here. Failing loudly beats scanning a nil that
		// "passes" while proving nothing.
		panic("driver ghi giả: không có giá trị mẫu cho cột " + cot)
	}
}

// khoGhiGia is the fake database. Everything a test needs to steer is a field here.
type khoGhiGia struct {
	lenh []lenhGhi
	dong []dongThongBao

	// soDongDoi is what Exec reports as RowsAffected — the ONE number that decides whether a
	// write is treated as new or as a redelivery. 1 by default; a test sets 0 to mean "the
	// UNIQUE constraint swallowed it" or "the row is already delivered".
	soDongDoi int64

	// coDong answers the existence probe behind ErrThongBaoKhongTonTai.
	coDong bool

	loiTruyVan error
	loiGhi     error
	loiMoGD    error

	// daMo / daChot / daHuy record the transaction's life. Rule 6 invariant 3 is a statement
	// about a transaction, so a suite that never looks at one cannot check it.
	daMo, daChot, daHuy int
}

func (k *khoGhiGia) Connect(context.Context) (driver.Conn, error) { return &connGhiGia{k: k}, nil }
func (k *khoGhiGia) Driver() driver.Driver                        { return trinhGhiGia{} }

type trinhGhiGia struct{}

func (trinhGhiGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver ghi giả: chỉ dùng Connector")
}

type connGhiGia struct{ k *khoGhiGia }

func (c *connGhiGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver ghi giả: không hỗ trợ Prepare")
}
func (c *connGhiGia) Close() error { return nil }
func (c *connGhiGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGhiGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.k.loiMoGD != nil {
		return nil, c.k.loiMoGD
	}
	c.k.daMo++
	return &gdGia{k: c.k}, nil
}

type gdGia struct{ k *khoGhiGia }

func (g *gdGia) Commit() error   { g.k.daChot++; return nil }
func (g *gdGia) Rollback() error { g.k.daHuy++; return nil }

func ghiArgs(args []driver.NamedValue) []driver.Value {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	return gt
}

type ketQuaGia struct{ n int64 }

func (r ketQuaGia) LastInsertId() (int64, error) { return 0, errors.New("không dùng") }
func (r ketQuaGia) RowsAffected() (int64, error) { return r.n, nil }

func (c *connGhiGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loiGhi != nil {
		return nil, c.k.loiGhi
	}
	return ketQuaGia{n: c.k.soDongDoi}, nil
}

func (c *connGhiGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loiTruyVan != nil {
		return nil, c.k.loiTruyVan
	}

	// The existence probe selects a literal, not columns. Answering it from `coDong` keeps the two
	// outcomes of GhiKetQua — "already delivered" and "no such row" — separable in a test, which
	// is the whole reason the store distinguishes them.
	if strings.Contains(q, "SELECT 1 FROM") {
		if !c.k.coDong {
			return &rowsGhiGia{cot: []string{"?column?"}}, nil
		}
		return &rowsGhiGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
	}

	cot, err := cotTrongCauLenhGhi(q)
	if err != nil {
		return nil, err
	}
	hang := make([][]driver.Value, 0, len(c.k.dong))
	for _, d := range c.k.dong {
		mot := make([]driver.Value, len(cot))
		for i, ten := range cot {
			mot[i] = d.giaTri(ten)
		}
		hang = append(hang, mot)
	}
	return &rowsGhiGia{cot: cot, hang: hang}, nil
}

// cotTrongCauLenhGhi reads the SELECT list out of the statement. THIS IS WHAT MAKES THE COLUMN
// CHECK WORK: the row is assembled BY NAME from the columns the store asked for, so reordering
// cotThongBao without reordering the Scan in docMotThongBao turns this suite red.
func cotTrongCauLenhGhi(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver ghi giả: không đọc được danh sách cột từ %q", q)
	}
	var ra []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		ra = append(ra, strings.TrimSpace(c))
	}
	return ra, nil
}

type rowsGhiGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsGhiGia) Columns() []string { return r.cot }
func (r *rowsGhiGia) Close() error      { return nil }
func (r *rowsGhiGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func moKhoGhi(k *khoGhiGia) *sql.DB { return sql.OpenDB(k) }
