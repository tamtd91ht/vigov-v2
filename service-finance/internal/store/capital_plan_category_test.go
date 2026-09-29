package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a test suite that prints
// `ok` while asserting nothing is this repository's worst known trap, and the sibling file
// capital_plan_category_pg_test.go is exactly that on every machine anybody has run it on.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back the rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the table read is the one this service owns · the soft-delete
//	              predicate is in the statement and `dang_dung` is NOT · the order and its
//	              tie-break are in the statement · the LIMIT really is the ceiling PLUS ONE · the
//	              refusal fires at ceiling+1 and returns NO rows · the positional Scan lines up
//	              with the column list, by NAME · a driver failure is wrapped, not swallowed ·
//	              no commune in the context panics rather than defaulting.
//
//	NOT PROVED    anything PostgreSQL does with that statement. THE SHARPEST LIMIT IS THIS: the
//	              fake builds its rows FROM THE COLUMN LIST THE STORE ITSELF HANDS IT, so a column
//	              named here that does not exist in the real table passes cleanly. A typo in
//	              categoryColumns, a column renamed by a later migration, the partition routing, the
//	              index, the `danh_muc_ba_tang` trigger — none of that is reachable from here.
//	              That is why the pg suite stays: it is the only thing that will ever check the
//	              statement against a real schema, and it becomes valuable the day a DSN exists.
//
// The column-name check is the one worth explaining: the driver builds each row BY COLUMN NAME out
// of the statement, so reordering categoryColumns without reordering the Scan turns these tests red.
// Read by position, `ma`/`nhan` and `la_mac_dinh`/`dang_dung` are two pairs of adjacent same-typed
// columns — swapping either produces no error at all, only slugs where labels belong, or a form
// pre-selecting a category the commune took out of use.

var (
	testTenant       = tenant.ID("01JA" + strings.Repeat("A", 22))
	secondTestTenant = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// tenantCtx is shared with capital_plan_category_pg_test.go, which converts its own string ids at the
// call site. ONE helper, not two: two ways to put a commune into a context is one of them ending up
// subtly different from what the edge really does.
func tenantCtx(tenantID tenant.ID) context.Context {
	return tenant.Into(context.Background(), tenantID)
}

// --- the fake driver ------------------------------------------------------------------------

type fakeStmt struct {
	sql  string
	args []driver.Value
}

// fakeRow is one row the fake returns. The values are distinct per column and distinct per type,
// so a mis-wired Scan shows up as WRONG DATA rather than as a zero value that looks plausible.
type fakeRow struct {
	id, code, label     string
	isDefault, isActive bool
	sortOrder           int
	source              string
	branched            bool
}

func (h fakeRow) value(column string) driver.Value {
	switch column {
	case "id":
		return h.id
	case "ma":
		return h.code
	case "nhan":
		return h.label
	case "la_mac_dinh":
		return h.isDefault
	case "dang_dung":
		return h.isActive
	case "thu_tu":
		// int64 AND NOT int: database/sql only accepts the driver.Value set, and `int` is not in
		// it. A fake handing back `int` fails every Scan with a message about conversion rather
		// than about the column — the kind of noise that gets a fake deleted.
		return int64(h.sortOrder)
	case "nguon":
		return h.source
	case "ma_nguon_re_nhanh":
		return h.branched
	default:
		// A column was added to categoryColumns and not here. Failing loudly beats scanning a nil that
		// "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + column)
	}
}

type fakeStore struct {
	stmts []fakeStmt
	row   []fakeRow
	err   error
}

func (k *fakeStore) Connect(context.Context) (driver.Conn, error) { return &fakeConn{k: k}, nil }
func (k *fakeStore) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeConn struct{ k *fakeStore }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *fakeConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.k.stmts = append(c.k.stmts, fakeStmt{sql: q, args: vals})
	if c.k.err != nil {
		return nil, c.k.err
	}
	column, err := selectColumns(q)
	if err != nil {
		return nil, err
	}
	row := make([][]driver.Value, 0, len(c.k.row))
	for _, h := range c.k.row {
		one := make([]driver.Value, len(column))
		for i, c := range column {
			one[i] = h.value(c)
		}
		row = append(row, one)
	}
	return &fakeRows{columns: column, rows: row}, nil
}

func selectColumns(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var result []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		result = append(result, strings.TrimSpace(c))
	}
	return result, nil
}

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	i       int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func newStoreOver(k *fakeStore) *CapitalPlanCategoryStore {
	return NewCapitalPlanCategoryStore(pkgstore.New(sql.OpenDB(k)))
}

func oneSampleRow() []fakeRow {
	// Values chosen so every field is distinguishable from every other: a Scan reading `nhan` into
	// Code, or `dang_dung` into IsDefault, cannot produce a passing assertion. The two booleans are
	// set to OPPOSITE values for exactly that reason.
	return []fakeRow{{
		id: "hm-001", code: "xay-dung-moi", label: "Xây dựng mới",
		isDefault: true, isActive: false,
		// `thu_tu` is deliberately NOT 0 and NOT 1: a zero is indistinguishable from an unscanned
		// field, and 1 from a length. `nguon`/`ma_nguon_re_nhanh` describe a TIER 3 row — the one
		// combination whose tier cannot be guessed from either column alone.
		sortOrder: 7, source: "he-thong", branched: true,
	}}
}

func manyRows(n int) []fakeRow {
	result := make([]fakeRow, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, fakeRow{
			id: fmt.Sprintf("hm-%04d", i), code: fmt.Sprintf("hang-muc-%04d", i),
			label: fmt.Sprintf("Hạng mục %d", i), isActive: true,
			sortOrder: i, source: "don-vi",
		})
	}
	return result
}

// --- the statement the store builds ----------------------------------------------------------

func TestListBindsTenantToParamOneFromContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of List and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	k := &fakeStore{row: oneSampleRow()}

	if _, err := newStoreOver(k).ListCategories(tenantCtx(testTenant)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(k.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.stmts))
	}
	l := k.stmts[0]
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(testTenant) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, testTenant)
	}
	// THE TABLE THIS SERVICE OWNS, and nothing else. A read against another service's table is a
	// path around the contract (rule 2, forbidden #2), and a typo in the name would otherwise only
	// surface on a machine that has a PostgreSQL — which is no machine here.
	if !strings.Contains(l.sql, "FROM hang_muc_ke_hoach_von ") {
		t.Errorf("đọc sai bảng: %q", l.sql)
	}

	// The same store, a different commune in the context, a different $1. This is what "scoped
	// repository" means in practice — and a store caching the first commune it saw would fail here.
	k2 := &fakeStore{row: oneSampleRow()}
	if _, err := newStoreOver(k2).ListCategories(tenantCtx(secondTestTenant)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.stmts[0].args[0] != string(secondTestTenant) {
		t.Errorf("$1 = %v, muốn %q", k2.stmts[0].args[0], secondTestTenant)
	}
}

func TestListFiltersSoftDeletedAndOrdersStably(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	// And the order is total: `thu_tu` is the commune's own arrangement, `ma` breaks ties and is
	// unique per commune, so two calls cannot return the same rows in a different sequence.
	k := &fakeStore{row: oneSampleRow()}

	if _, err := newStoreOver(k).ListCategories(tenantCtx(testTenant)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	q := k.stmts[0].sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu_tu, ma") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// `dang_dung` MUST NOT BE IN THE PREDICATE, and this is the assertion most likely to be
	// "fixed" by somebody who thinks a catalogue should only return live rows. A category taken
	// out of use is still read: the catalogue screen shows it with a "Đã tắt" chip, and a capital
	// plan line of an earlier budget year holds its code AS A VALUE and would otherwise render
	// with no category name at all. It is selected as a column — hence the two specific shapes
	// below rather than a bare search for the word.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") {
		t.Errorf("câu lệnh lọc mất hạng mục đã tắt: %q", q)
	}
}

func TestListFetchesCeilingPlusOne(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	k := &fakeStore{row: oneSampleRow()}

	if _, err := newStoreOver(k).ListCategories(tenantCtx(testTenant)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	l := k.stmts[0]
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(MaxCapitalPlanCategories+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], MaxCapitalPlanCategories+1)
	}
}

// --- what comes back -------------------------------------------------------------------------

func TestListReadsEveryColumnCorrectly(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Every column here is read BY POSITION, in lockstep with
	// categoryColumns, and the compiler cannot help: `ma`/`nhan` are both TEXT and `la_mac_dinh`/
	// `dang_dung` are both BOOLEAN. A swap in either pair compiles, runs, and passes any test whose
	// fixture happens to agree with itself.
	k := &fakeStore{row: oneSampleRow()}

	result, err := newStoreOver(k).ListCategories(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(result))
	}
	one := result[0]
	if one.ID != "hm-001" || one.Code != "xay-dung-moi" || one.Label != "Xây dựng mới" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", one)
	}
	// The pair that cannot be caught by type: both are BOOLEAN and adjacent. The fixture sets them
	// to OPPOSITE values on purpose — swapped, this row would report a category the commune took
	// out of use as the one every form pre-selects.
	if !one.IsDefault || one.IsActive {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", one)
	}
}

func TestListCommuneWithNoRowsReturnsEmptyList(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and seeds nothing: a row
	// here carries a tenant_id and the migration runner has no commune in it, so the step that
	// would sow a commune's system rows is commune onboarding — which does not exist yet. An empty
	// list is correct, not a failure, and it is a list rather than a nil the caller must branch on.
	k := &fakeStore{}

	result, err := newStoreOver(k).ListCategories(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("danh mục rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if result == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(result) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa có dòng nào", len(result))
	}
}

// --- the ceiling -------------------------------------------------------------------------------

func TestListAtCeilingStillReturnsAll(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid.
	k := &fakeStore{row: manyRows(MaxCapitalPlanCategories)}

	result, err := newStoreOver(k).ListCategories(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(result) != MaxCapitalPlanCategories {
		t.Errorf("nhận %d dòng, muốn %d", len(result), MaxCapitalPlanCategories)
	}
}

func TestListPastCeilingRefusesAndReturnsNoRows(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	k := &fakeStore{row: manyRows(MaxCapitalPlanCategories + 1)}

	result, err := newStoreOver(k).ListCategories(tenantCtx(testTenant))
	if !errors.Is(err, ErrTooManyCapitalPlanCategories) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuHangMuc", err)
	}
	if result != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(result))
	}
}

// --- failures ----------------------------------------------------------------------------------

func TestListStoreErrorIsWrappedNotSwallowed(t *testing.T) {
	// Errors are wrapped with %w, never swallowed with `_`. The caller distinguishes the ceiling
	// from an ordinary failure with errors.Is, which only works if the chain is intact.
	original := errors.New("cơ sở dữ liệu không phản hồi")
	k := &fakeStore{err: original}

	result, err := newStoreOver(k).ListCategories(tenantCtx(testTenant))
	if !errors.Is(err, original) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, original)
	}
	if errors.Is(err, ErrTooManyCapitalPlanCategories) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if result != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestListWithoutTenantInContextPanics(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either query every commune's
	// rows or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover turns that
	// into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc danh mục khi context không có xã mà không panic")
		}
	}()
	k := &fakeStore{row: oneSampleRow()}
	_, _ = newStoreOver(k).ListCategories(context.Background())
}
