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
// erases exactly that. Running docstore.LoaiTaiNguyenBanDoStore unchanged over this driver keeps the
// statements real while needing no PostgreSQL — and a test that needs infrastructure is a test that
// stops being run (ADR 0013, last section: VIGOV_TEST_DSN is unset here).
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the `danh_muc_ba_tang` trigger, not
// `UNIQUE (tenant_id, ma)`, not the partition routing. That half needs a real server; the pg suites
// next door are where it lands the day a DSN exists.

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type lenhGhi struct {
	sql  string
	args []driver.Value
}

// hangLVB is the row the FOR UPDATE read hands back. A nil *hangLVB means "no such live row".
type hangLVB struct {
	id, ma, nhan string
	dangDung     bool
	macDinh      bool
	thuTu        int
	nguon        string
	reNhanh      bool
}

type khoGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	// dem answers the ceiling check, maTrung the duplicate check. Separate fields because the two
	// statements ask different questions and a single counter would make one of them untestable.
	dem     int
	maTrung int

	hang *hangLVB
	loi  error

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `loi`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business write down with it, and that needs everything before it to
	// have already run.
	loiSau string
	daNo   bool
}

func (k *khoGia) ghi(q string, args []driver.NamedValue) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: gt})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoGia) cau(tu string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ra []lenhGhi
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

// kiemLoi decides whether this statement is the one that fails.
func (k *khoGia) kiemLoi(q string) error {
	if k.loi != nil {
		return k.loi
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.loiSau != "" && !k.daNo && strings.Contains(q, k.loiSau) {
		k.daNo = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *khoGia) Connect(context.Context) (driver.Conn, error) { return &connGia{k: k}, nil }
func (k *khoGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connGia struct{ k *khoGia }

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error { return nil }

func (c *connGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txGia{k: c.k}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.doiMotDong turns zero into "not found", and a fake returning
	// zero would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	switch {
	// ORDER MATTERS: the duplicate check is also a count(*), so it has to be recognised first.
	case strings.Contains(q, "count(*)") && strings.Contains(q, "ma = $2"):
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{int64(c.k.maTrung)}}}, nil
	case strings.Contains(q, "count(*)"):
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{int64(c.k.dem)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.hang == nil {
			return &rowsGia{cot: cotLVB()}, nil
		}
		h := *c.k.hang
		return &rowsGia{cot: cotLVB(), hang: [][]driver.Value{{
			h.id, h.ma, h.nhan, h.dangDung, h.macDinh, int64(h.thuTu), h.nguon, h.reNhanh,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// cotLVB mirrors docstore's column list ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — the store's own
// suite makes the same argument for the same reason.
func cotLVB() []string {
	return []string{"id", "ma", "nhan", "dang_dung", "la_mac_dinh", "thu_tu", "nguon", "ma_nguon_re_nhanh"}
}

type txGia struct{ k *khoGia }

func (t *txGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

type rowsGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsGia) Columns() []string { return r.cot }
func (r *rowsGia) Close() error      { return nil }
func (r *rowsGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

// dungUseCase builds the REAL use case over the REAL store over the fake driver, with the id pinned
// so assertions can name it.
func dungUseCase(t *testing.T, k *khoGia) (*DanhMucLoaiTaiNguyen, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	kho := store.New(db)
	uc := NewDanhMucLoaiTaiNguyen(kho, docstore.NewLoaiTaiNguyenBanDoStore(kho))
	uc.sinhID = func() (string, error) { return "01JIDMOICUADONGVUATAO0000", nil }
	return uc, tenant.Into(context.Background(), xaA)
}
