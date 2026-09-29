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
// `ok` while asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back the rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the soft-delete predicate is in the statement · the order and its
//	              tie-break are in the statement · the LIMIT really is the ceiling PLUS ONE · the
//	              refusal fires at ceiling+1 and returns NO rows · the positional Scan lines up
//	              with the column list, by NAME · a driver failure is wrapped, not swallowed.
//	NOT PROVED    anything PostgreSQL does with that statement — that the index is used, that the
//	              partition routing works, that the trigger refuses what it says it refuses. That
//	              needs a real server, and there is none on this machine.
//
// The column-name check is the one worth explaining: the driver builds each row BY COLUMN NAME out
// of the statement, so reordering mapAssetTypeColumns without reordering the Scan below turns these
// tests red. Read by position, `ma`/`nhan` and `la_mac_dinh`/`dang_dung` are two pairs of adjacent
// same-typed columns — swapping either produces no error at all, only slugs where labels belong,
// or a map opening on a group that was taken out of use.

var testTenant = tenant.ID("01JA" + strings.Repeat("A", 22))

func tenantCtx(t tenant.ID) context.Context {
	return tenant.Into(context.Background(), t)
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
	sortOrder           int64
	isDefault, isActive bool
	source              string
	branched            bool
}

func (r fakeRow) value(col string) driver.Value {
	switch col {
	case "id":
		return r.id
	case "ma":
		return r.code
	case "nhan":
		return r.label
	case "thu_tu":
		return r.sortOrder
	case "la_mac_dinh":
		return r.isDefault
	case "dang_dung":
		return r.isActive
	case "nguon":
		return r.source
	case "ma_nguon_re_nhanh":
		return r.branched
	default:
		// A column was added to mapAssetTypeColumns and not here. Failing loudly beats scanning a nil
		// that "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + col)
	}
}

type fakeStore struct {
	stmts []fakeStmt
	rows  []fakeRow
	err   error
}

func (f *fakeStore) Connect(context.Context) (driver.Conn, error) { return &fakeConn{f: f}, nil }
func (f *fakeStore) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeConn struct{ f *fakeStore }

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
	c.f.stmts = append(c.f.stmts, fakeStmt{sql: q, args: vals})
	if c.f.err != nil {
		return nil, c.f.err
	}
	cols, err := selectColumns(q)
	if err != nil {
		return nil, err
	}
	out := make([][]driver.Value, 0, len(c.f.rows))
	for _, r := range c.f.rows {
		one := make([]driver.Value, len(cols))
		for i, c := range cols {
			one[i] = r.value(c)
		}
		out = append(out, one)
	}
	return &fakeRows{cols: cols, rows: out}, nil
}

func selectColumns(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var out []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		out = append(out, strings.TrimSpace(c))
	}
	return out, nil
}

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func newTestMapAssetTypeStore(f *fakeStore) *MapAssetTypeStore {
	return NewMapAssetTypeStore(pkgstore.New(sql.OpenDB(f)))
}

func oneRowFixture() []fakeRow {
	// Values chosen so every field is distinguishable from every other: a Scan reading `nhan` into
	// Code, or `dang_dung` into IsDefault, cannot produce a passing assertion.
	return []fakeRow{{
		id: "ltn-001", code: "mau-mot", label: "Nhóm mẫu một",
		sortOrder: 7, isDefault: true, isActive: false,
		// A TIER 3 row — the one combination whose tier cannot be guessed from either column alone.
		source: "he-thong", branched: true,
	}}
}

// --- the statement the store builds ----------------------------------------------------------

func TestListBindsTenantAsFirstParamFromContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of List and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	f := &fakeStore{rows: oneRowFixture()}

	if _, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant)); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if len(f.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(f.stmts))
	}
	s := f.stmts[0]
	if !strings.Contains(s.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", s.sql)
	}
	if len(s.args) == 0 || s.args[0] != string(testTenant) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", s.args, testTenant)
	}
	// The same store, a different commune in the context, a different $1. This is what "scoped
	// repository" means in practice — and a store caching the first commune it saw would fail here.
	f2 := &fakeStore{rows: oneRowFixture()}
	otherTenant := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := newTestMapAssetTypeStore(f2).List(tenantCtx(otherTenant)); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if f2.stmts[0].args[0] != string(otherTenant) {
		t.Errorf("$1 = %v, muốn %q", f2.stmts[0].args[0], otherTenant)
	}
}

func TestListExcludesSoftDeletedAndOrdersStably(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	// And the order is total: `thu_tu` is the commune's own arrangement, `ma` breaks ties and is
	// unique per commune, so two calls cannot return the same rows in a different sequence.
	f := &fakeStore{rows: oneRowFixture()}

	if _, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant)); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	q := f.stmts[0].sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu_tu, ma") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// `dang_dung` must NOT be in the predicate: a group taken out of use is still read — the
	// catalogue screen shows it with a "Đã tắt" chip, and assets already filed under it need its
	// name.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") {
		t.Errorf("câu lệnh lọc mất nhóm đã tắt: %q", q)
	}
}

func TestListFetchesCeilingPlusOne(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	f := &fakeStore{rows: oneRowFixture()}

	if _, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant)); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	s := f.stmts[0]
	if !strings.Contains(s.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", s.sql)
	}
	if len(s.args) < 2 || s.args[1] != int64(MapAssetTypeCeiling+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", s.args[1:], MapAssetTypeCeiling+1)
	}
}

// --- what comes back -------------------------------------------------------------------------

func TestListReadsEachColumnIntoItsField(t *testing.T) {
	f := &fakeStore{rows: oneRowFixture()}

	out, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(out))
	}
	one := out[0]
	if one.ID != "ltn-001" || one.Code != "mau-mot" || one.Label != "Nhóm mẫu một" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", one)
	}
	if one.SortOrder != 7 {
		t.Errorf("thu_tu = %d, muốn 7", one.SortOrder)
	}
	// The pair that cannot be caught by type: both are BOOLEAN and adjacent. The fixture sets them
	// to OPPOSITE values on purpose.
	if !one.IsDefault || one.IsActive {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", one)
	}
}

func TestListTenantWithNoRowsReturnsEmptyList(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and seeds nothing: the
	// specification contradicts itself about the code list, and the step that would sow a commune's
	// system rows does not exist. An empty list is correct, not a failure — and it is a list, never
	// a nil the caller has to branch on.
	f := &fakeStore{}

	out, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("danh mục rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if out == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(out) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa có dòng nào", len(out))
	}
}

// --- the ceiling -------------------------------------------------------------------------------

func TestListAtCeilingStillReturnsEverything(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid.
	f := &fakeStore{rows: manyRows(MapAssetTypeCeiling)}

	out, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(out) != MapAssetTypeCeiling {
		t.Errorf("nhận %d dòng, muốn %d", len(out), MapAssetTypeCeiling)
	}
}

func TestListOverCeilingRefusesAndReturnsNoRows(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	f := &fakeStore{rows: manyRows(MapAssetTypeCeiling + 1)}

	out, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant))
	if !errors.Is(err, ErrTooManyMapAssetTypes) {
		t.Fatalf("lỗi = %v, muốn ErrTooManyMapAssetTypes", err)
	}
	if out != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(out))
	}
}

func manyRows(n int) []fakeRow {
	out := make([]fakeRow, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fakeRow{
			id: fmt.Sprintf("ltn-%04d", i), code: fmt.Sprintf("mau-%04d", i),
			label: fmt.Sprintf("Nhóm mẫu %d", i), sortOrder: int64(i), isActive: true,
			source: "don-vi",
		})
	}
	return out
}

// --- failures ----------------------------------------------------------------------------------

func TestListStoreErrorIsWrappedNotSwallowed(t *testing.T) {
	// Rule 6 of the service pattern: wrapped with %w, never swallowed with `_`. The caller
	// distinguishes the ceiling from an ordinary failure with errors.Is, which only works if the
	// chain is intact.
	cause := errors.New("cơ sở dữ liệu không phản hồi")
	f := &fakeStore{err: cause}

	out, err := newTestMapAssetTypeStore(f).List(tenantCtx(testTenant))
	if !errors.Is(err, cause) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, cause)
	}
	if errors.Is(err, ErrTooManyMapAssetTypes) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if out != nil {
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
	f := &fakeStore{rows: oneRowFixture()}
	_, _ = newTestMapAssetTypeStore(f).List(context.Background())
}
