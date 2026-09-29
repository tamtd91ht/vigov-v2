package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// A second fake database/sql driver, for the disbursement voucher register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// fake_driver_catalogue_test.go makes, and it is heavier here because the properties are about money:
//
//	the business write and its audit entry are in ONE transaction   (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	`trang_thai` is never written from anything a request could reach
//	an unlock writes its reason IN THE SAME STATEMENT as the state  (migration 0005, decision 1)
//	`so_lan_mo_khoa` is incremented IN SQL, never read-modify-written
//	a soft delete writes all three of rule 7 invariant 1's columns
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A SECOND DRIVER AND NOT AN EXTENSION OF THE FIRST: that one answers `count(*)` and one catalogue
// row shape. Teaching it a second table would make every case in either file depend on a branch
// written for the other, and the first surprising interaction would be silent.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the `chung_tu_da_khoa` trigger, not
// `chung_tu_giai_ngan_mo_khoa_du_vet`, not `CHECK (so_tien > 0)`, not the partition routing. That
// half needs a real server (VIGOV_TEST_DSN is unset here), and it is the FLOOR under everything
// this file asserts — the application refusals tested below are the SENTENCE, not the enforcement.

// voucherRow is the row the FOR UPDATE read hands back. A nil *voucherRow means "no such live voucher".
type voucherRow struct {
	id, investmentProjectID string
	paymentDate             time.Time
	amount                  int64
	description             string
	counterparty            string
	voucherNo               string
	fundingSourceID         string
	status                  string
	enteredBy               string
	confirmedBy             string
	lockedBy                string
	unlockedBy              string
	lockedAt                *time.Time
	unlockedAt              *time.Time
	unlockReason            string
	unlockCount             int64
}

type fakeVoucherStore struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begins, commits, rollbacks int

	row *voucherRow

	// investmentProjectCode is what `SELECT ma FROM du_an` answers. EMPTY MEANS NO SUCH LIVE PROJECT, which is the
	// case ErrVoucherInvestmentProjectNotFound exists for — money filed against a project that totals nowhere.
	investmentProjectCode string

	// liveFundingSources is the set of LIVE funding sources of this commune, and it is a SET rather than a
	// boolean on purpose: the case worth separating is "this id names nothing here" from "no source
	// exists at all". The first is what a voucher attached to another commune's source looks like
	// from inside this one — the query binds tenant_id = $1, so such a row is simply not there
	// (rule 1) — and it is the case with no foreign key underneath to catch it (0007:102-113).
	liveFundingSources map[string]bool

	err error

	// failOnSQL fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `err`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business write down with it, and that needs everything before it to
	// have already run.
	failOnSQL string
	failed    bool
}

func (k *fakeVoucherStore) write(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, recordedStmt{sql: q, args: vals})
	k.mu.Unlock()
}

// stmtsContaining returns every recorded statement containing `substr`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *fakeVoucherStore) stmtsContaining(substr string) []recordedStmt {
	k.mu.Lock()
	defer k.mu.Unlock()
	var result []recordedStmt
	for _, l := range k.stmts {
		if strings.Contains(l.sql, substr) {
			result = append(result, l)
		}
	}
	return result
}

func (k *fakeVoucherStore) hasStmt(substr string) bool { return len(k.stmtsContaining(substr)) > 0 }

func (k *fakeVoucherStore) failFor(q string) error {
	if k.err != nil {
		return k.err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failOnSQL != "" && !k.failed && strings.Contains(q, k.failOnSQL) {
		k.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeVoucherStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeVoucherConn{k: k}, nil
}
func (k *fakeVoucherStore) Driver() driver.Driver { return fakeDriver{} }

type fakeVoucherConn struct{ k *fakeVoucherStore }

func (c *fakeVoucherConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeVoucherConn) Close() error { return nil }

func (c *fakeVoucherConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeVoucherConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begins++
	c.k.mu.Unlock()
	return &fakeVoucherTx{k: c.k}, nil
}

func (c *fakeVoucherConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.expectOneVoucherRow turns zero into "not found", and a fake
	// returning zero would make every update look like a missing voucher — hiding the case this
	// actually tests.
	return driver.RowsAffected(1), nil
}

func (c *fakeVoucherConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.write(q, args)
	if err := c.k.failFor(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "FROM du_an"):
		if c.k.investmentProjectCode == "" {
			return &fakeRows{columns: []string{"ma"}}, nil
		}
		return &fakeRows{columns: []string{"ma"}, rows: [][]driver.Value{{c.k.investmentProjectCode}}}, nil
	case strings.Contains(q, "FROM nguon_von"):
		// NO ROW IS THE ANSWER FOR AN ID THIS COMMUNE DOES NOT HOLD, which is exactly what PostgreSQL
		// would return: the statement binds tenant_id = $1, so another commune's source and a source
		// that never existed are one answer here (store.ErrVoucherFundingSourceNotFound).
		if len(args) < 2 || !c.k.liveFundingSources[fmt.Sprint(args[1].Value)] {
			return &fakeRows{columns: []string{"?column?"}}, nil
		}
		return &fakeRows{columns: []string{"?column?"}, rows: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.row == nil {
			return &fakeRows{columns: voucherColumns()}, nil
		}
		h := *c.k.row
		return &fakeRows{columns: voucherColumns(), rows: [][]driver.Value{{
			h.id, h.investmentProjectID, h.paymentDate, h.amount, h.description,
			h.counterparty, h.voucherNo, h.fundingSourceID, h.status,
			h.enteredBy, h.confirmedBy, h.lockedBy, h.unlockedBy,
			timeOrNil(h.lockedAt), timeOrNil(h.unlockedAt),
			h.unlockReason, h.unlockCount,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

func timeOrNil(t *time.Time) driver.Value {
	if t == nil {
		return nil
	}
	return *t
}

// voucherColumns mirrors fistore's voucherColumns ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — and that list has
// four adjacent staff-code columns whose swap would make "the person who locked it" name the person
// who unlocked it, which is the exact comparison the self-unlock rule is made of.
func voucherColumns() []string {
	return []string{
		"id", "du_an_id", "ngay_chi", "so_tien", "noi_dung",
		"doi_tac", "so_chung_tu", "nguon_von_id", "trang_thai",
		"nguoi_nhap_id", "nguoi_xac_nhan_id", "nguoi_khoa_id", "nguoi_mo_khoa_id",
		"thoi_diem_khoa", "thoi_diem_mo_khoa", "ly_do_mo_khoa", "so_lan_mo_khoa",
	}
}

type fakeVoucherTx struct{ k *fakeVoucherStore }

func (t *fakeVoucherTx) Commit() error {
	t.k.mu.Lock()
	t.k.commits++
	t.k.mu.Unlock()
	return nil
}

func (t *fakeVoucherTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rollbacks++
	t.k.mu.Unlock()
	return nil
}

// fixedTime is the instant every lock and unlock below is stamped with. FIXED rather than
// time.Now(), so that an assertion can compare what reached `thoi_diem_khoa` with what the test
// asked for — a wall-clock read would make that assertion about scheduling.
var fixedTime = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)

// newVoucherUseCase builds the REAL use case over the REAL store over the fake driver, with the id
// and the clock pinned so assertions can name them.
func newVoucherUseCase(t *testing.T, k *fakeVoucherStore) (*DisbursementVoucherService, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	repo := store.New(db)
	uc := NewDisbursementVoucherService(repo, fistore.NewDisbursementVoucherStore(repo))
	uc.newID = func() (string, error) { return newVoucherID, nil }
	uc.now = func() time.Time { return fixedTime }
	return uc, tenant.Into(context.Background(), tenantA)
}

const newVoucherID = "01JCHUNGTUMOIVUATAO000000"
