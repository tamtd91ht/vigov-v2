package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a suite that prints `ok`
// while asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the statement as $1 and comes from the CONTEXT, never from
//	              an argument · BOTH sides of the join are constrained to that same $1 · the
//	              soft-delete predicate is present on the project AND on the voucher subquery ·
//	              the disbursed total is an AGGREGATE and not a column of du_an · no voucher state
//	              is excluded from the total · the budget year is required rather than defaulted ·
//	              the LIMIT is the ceiling PLUS ONE and the refusal fires at ceiling+1 returning
//	              NO rows · a missing project is ErrInvestmentProjectNotFound and not a zero value · a driver
//	              failure is wrapped, not swallowed · no commune in the context panics rather than
//	              defaulting.
//
//	NOT PROVED    anything PostgreSQL does with that statement. The fake returns whatever rows the
//	              test supplies whatever the column list says, so a typo in investmentProjectColumns passes cleanly
//	              here. Partition routing, the CHECK constraints, the hard-delete trigger and the
//	              locked-voucher trigger are all unreachable from this file — they need a real
//	              server, and none is reachable from this build environment.

// --- the fake driver ------------------------------------------------------------------------

// fakeInvestmentProjectRow is one row as the fake hands it back, in the order of investmentProjectColumns plus the aggregate.
// Values are distinct per field so a mis-wired Scan shows up as WRONG DATA rather than as a zero
// that looks plausible — the two amounts especially, which swap with no error at all.
type fakeInvestmentProjectRow struct {
	id, code                  string
	year                      int64
	categoryID, name          string
	description               string
	plan, approvedAmount      int64
	orgUnit, ownerStaff       string
	startDate, completionDate any // nil or time.Time
	deadline                  time.Time
	disbursed                 int64
}

func (d fakeInvestmentProjectRow) values() []driver.Value {
	return []driver.Value{
		d.id, d.code, d.year, d.categoryID, d.name, d.description,
		d.plan, d.approvedAmount, d.orgUnit, d.ownerStaff,
		d.startDate, d.completionDate, d.deadline, d.disbursed,
	}
}

type fakeInvestmentProjectStore struct {
	stmts []fakeStmt
	row   []fakeInvestmentProjectRow
	err   error
}

func (k *fakeInvestmentProjectStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeInvestmentProjectConn{k: k}, nil
}
func (k *fakeInvestmentProjectStore) Driver() driver.Driver { return fakeInvestmentProjectDriver{} }

type fakeInvestmentProjectDriver struct{}

func (fakeInvestmentProjectDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeInvestmentProjectConn struct{ k *fakeInvestmentProjectStore }

func (c *fakeInvestmentProjectConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeInvestmentProjectConn) Close() error { return nil }
func (c *fakeInvestmentProjectConn) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *fakeInvestmentProjectConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.k.stmts = append(c.k.stmts, fakeStmt{sql: q, args: vals})
	if c.k.err != nil {
		return nil, c.k.err
	}
	row := make([][]driver.Value, 0, len(c.k.row))
	for _, h := range c.k.row {
		row = append(row, h.values())
	}
	// 14 unnamed columns: investmentProjectColumns's thirteen plus the aggregate. The fake does not parse the
	// column list — see the NOT PROVED note at the top.
	column := make([]string, 14)
	return &fakeRows{columns: column, rows: row}, nil
}

func newInvestmentProjectStoreOver(k *fakeInvestmentProjectStore) *InvestmentProjectStore {
	return NewInvestmentProjectStore(pkgstore.New(sql.OpenDB(k)))
}

func sampleInvestmentProjectRows() []fakeInvestmentProjectRow {
	return []fakeInvestmentProjectRow{{
		id: "da-001", code: "DA-2026-be-tong-hoa-duong-ngo-xo-2", year: 2026,
		categoryID: "hm-001", name: "Bê tông hoá đường ngõ xóm tổ 6", description: "mô tả",
		// §8's worked project: plan 100 triệu, disbursed 90 triệu. The two amounts differ from
		// each other and from the total, so a swap between them cannot pass.
		plan: 100_000_000, approvedAmount: 120_000_000,
		orgUnit: "bp-001", ownerStaff: "cb-001",
		deadline: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
		// NOT a column of du_an: the aggregate over the project's live vouchers.
		disbursed: 90_000_000,
	}}
}

// --- the statement the store builds ----------------------------------------------------------

func TestListInvestmentProjectsBindsTenantToParamOneFromContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of List and cannot be: it
	// arrives in the context and QueryJoin binds it to $1.
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	if _, err := newInvestmentProjectStoreOver(k).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	if len(k.stmts) != 1 {
		t.Fatalf("số câu lệnh = %d, muốn 1", len(k.stmts))
	}
	if got := k.stmts[0].args[0]; got != string(testTenant) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", got, testTenant)
	}
	if got := k.stmts[0].args[1]; got != int64(2026) {
		t.Fatalf("$2 = %v, muốn năm ngân sách 2026", got)
	}
}

func TestListInvestmentProjectsRequiresTenantOnBOTHSIDESofJoin(t *testing.T) {
	// THE PROPERTY THIS CASE EXISTS FOR: a join reaches two tables, and constraining only the
	// outer one leaves the other joinable across communes wherever ids collide. No test of a
	// single commune would ever show it, so it is asserted on the TEXT of the statement.
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	if _, err := newInvestmentProjectStoreOver(k).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	q := k.stmts[0].sql

	for _, right := range []string{
		"da.tenant_id = $1",           // the project side
		"ct.tenant_id = $1",           // the voucher subquery
		"ct.tenant_id = da.tenant_id", // and the join itself, not id alone
		"da.deleted_at IS NULL",       // rule 7, invariant 2 — the project
		"ct.deleted_at IS NULL",       // rule 7, invariant 2 — the vouchers
		"SUM(ct.so_tien)",             // the total is DERIVED, never a stored column
		"chung_tu_giai_ngan",          // ... from the vouchers, not from anywhere else
		"ORDER BY da.ma",              // total order: `ma` is unique per commune
	} {
		if !strings.Contains(q, right) {
			t.Fatalf("câu lệnh thiếu %q:\n%s", right, q)
		}
	}
	// No voucher state is excluded: §11 counts from `ke-toan-nhap` upward, which is every state.
	if strings.Contains(q, "trang_thai") {
		t.Fatalf("câu lệnh lọc theo trang_thai — §11 tính TỪ `ke-toan-nhap` trở đi, tức là cả ba:\n%s", q)
	}
	// And `da_giai_ngan` must never be read as a column of du_an.
	if strings.Contains(q, "da.da_giai_ngan") {
		t.Fatalf("đọc `da_giai_ngan` như một CỘT của du_an — nó là tổng suy ra:\n%s", q)
	}
}

func TestListInvestmentProjectsRequiresBudgetYearNotDefault(t *testing.T) {
	// A default here would report a different year's money under this year's heading, and nothing
	// on the screen would say so (§13 rule 8). Fail closed.
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	_, err := newInvestmentProjectStoreOver(k).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{})
	if !errors.Is(err, ErrBudgetYearMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNamNganSach", err)
	}
	if len(k.stmts) != 0 {
		t.Fatal("thiếu năm mà vẫn chạy truy vấn — phải từ chối TRƯỚC khi chạm kho")
	}
}

func TestListInvestmentProjectsReadsDerivedTotal(t *testing.T) {
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	result, err := newInvestmentProjectStoreOver(k).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026})
	if err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("số dòng = %d, muốn 1", len(result))
	}
	one := result[0]
	if one.InvestmentProject.Code != "DA-2026-be-tong-hoa-duong-ngo-xo-2" {
		t.Fatalf("mã = %q", one.InvestmentProject.Code)
	}
	if one.InvestmentProject.PlannedAmount != 100_000_000 {
		t.Fatalf("kế hoạch vốn = %d, muốn 100000000 — hai số tiền đứng cạnh nhau, đảo là im lặng", one.InvestmentProject.PlannedAmount)
	}
	if one.InvestmentProject.ApprovedAmount != 120_000_000 {
		t.Fatalf("tổng mức = %d, muốn 120000000", one.InvestmentProject.ApprovedAmount)
	}
	if one.DisbursedAmount != 90_000_000 {
		t.Fatalf("đã giải ngân = %d, muốn 90000000", one.DisbursedAmount)
	}
	// And the derived figures line up with §8's worked project: 90%.
	ratio, ok := one.DisbursedRatio()
	if !ok || ratio != 9000 {
		t.Fatalf("tỷ lệ = %d phần vạn (ok=%v), muốn 9000", ratio, ok)
	}
}

func TestListInvestmentProjectsLIMITIsCeilingPlusOne(t *testing.T) {
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	if _, err := newInvestmentProjectStoreOver(k).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	end := k.stmts[0].args[len(k.stmts[0].args)-1]
	if end != int64(MaxInvestmentProjectsPerYear+1) {
		t.Fatalf("LIMIT = %v, muốn trần+1 = %d — lấy đúng trần thì một danh sách bị cắt trông y hệt một danh sách đủ",
			end, MaxInvestmentProjectsPerYear+1)
	}
}

func TestListInvestmentProjectsPastCeilingIsREFUSEDNotTruncated(t *testing.T) {
	row := make([]fakeInvestmentProjectRow, 0, MaxInvestmentProjectsPerYear+1)
	for i := 0; i <= MaxInvestmentProjectsPerYear; i++ {
		row = append(row, fakeInvestmentProjectRow{id: "da", code: "DA", year: 2026, plan: 1, disbursed: 0,
			deadline: time.Now()})
	}
	result, err := newInvestmentProjectStoreOver(&fakeInvestmentProjectStore{row: row}).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026})
	if !errors.Is(err, ErrTooManyInvestmentProjects) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuDuAn", err)
	}
	if result != nil {
		// Handing back rows alongside the error is how a refusal turns back into a truncation:
		// these figures are totalled on the screen.
		t.Fatalf("trả về %d dòng CÙNG với lỗi — phải trả nil", len(result))
	}
}

func TestListInvestmentProjectsWrapsErrorNotSwallows(t *testing.T) {
	failure := errors.New("kho hỏng")
	_, err := newInvestmentProjectStoreOver(&fakeInvestmentProjectStore{err: failure}).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026})
	if !errors.Is(err, failure) {
		t.Fatalf("lỗi = %v, phải bọc lỗi gốc bằng %%w", err)
	}
}

func TestListInvestmentProjectsWithoutTenantPANICSNotDefault(t *testing.T) {
	// FAIL CLOSED. A default on the isolation path collapses the whole isolation silently, with
	// tests still green (rule 1, forbidden #1).
	defer func() {
		if recover() == nil {
			t.Fatal("không có xã trong context mà vẫn chạy — phải panic, không được mặc định")
		}
	}()
	_, _ = newInvestmentProjectStoreOver(&fakeInvestmentProjectStore{}).ListInvestmentProjects(context.Background(), InvestmentProjectFilter{Year: 2026})
}

func TestGetInvestmentProjectNotFoundIsNotEMPTYValue(t *testing.T) {
	// A zero value returned with a nil error is a project that renders as an empty screen with
	// 0 đ on it — a figure, and a wrong one. The caller answers 404.
	_, err := newInvestmentProjectStoreOver(&fakeInvestmentProjectStore{}).GetInvestmentProject(tenantCtx(testTenant), "da-khong-ton-tai")
	if !errors.Is(err, ErrInvestmentProjectNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAn", err)
	}
}

func TestGetInvestmentProjectUsesSAMETotalFormulaAsList(t *testing.T) {
	// The detail page and the list must not be able to disagree about one project's disbursed
	// figure. Asserted on the text: both statements carry the same aggregate expression.
	namedK := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	if _, err := newInvestmentProjectStoreOver(namedK).ListInvestmentProjects(tenantCtx(testTenant), InvestmentProjectFilter{Year: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	kOnly := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	one, err := newInvestmentProjectStoreOver(kOnly).GetInvestmentProject(tenantCtx(testTenant), "da-001")
	if err != nil {
		t.Fatalf("đọc chi tiết: %v", err)
	}
	if one.DisbursedAmount != 90_000_000 {
		t.Fatalf("đã giải ngân = %d, muốn 90000000", one.DisbursedAmount)
	}
	if !strings.Contains(namedK.stmts[0].sql, voucherTotals) || !strings.Contains(kOnly.stmts[0].sql, voucherTotals) {
		t.Fatal("hai tuyến đọc không dùng chung một biểu thức tổng — hai màn hình sẽ lệch nhau")
	}
}

func TestGetInvestmentProjectReadsOnlyContextTenant(t *testing.T) {
	// Another commune's project is indistinguishable from one that does not exist, because the
	// query cannot reach it: $1 comes from the context.
	k := &fakeInvestmentProjectStore{row: sampleInvestmentProjectRows()}
	if _, err := newInvestmentProjectStoreOver(k).GetInvestmentProject(tenantCtx(secondTestTenant), "da-001"); err != nil {
		t.Fatalf("đọc chi tiết: %v", err)
	}
	if got := k.stmts[0].args[0]; got != string(secondTestTenant) {
		t.Fatalf("$1 = %v, muốn %q", got, secondTestTenant)
	}
	if got := k.stmts[0].args[1]; got != "da-001" {
		t.Fatalf("$2 = %v, muốn id dự án", got)
	}
}

// domain is imported for the type assertion below; keeping the reference explicit stops a future
// edit from dropping the import and quietly changing what these tests scan into.
var _ = domain.InvestmentProjectProgress{}
