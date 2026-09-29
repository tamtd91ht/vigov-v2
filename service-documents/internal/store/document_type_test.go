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

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a test suite that prints
// `ok` while asserting nothing is this repository's worst known trap, and the sibling file
// document_type_pg_test.go is exactly that on a machine with no PostgreSQL.
//
// It runs with NO database. The fake driver below RECORDS the statement the store builds and hands
// back the rows the test supplied; it never executes SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the soft-delete predicate is in the statement and `dang_dung` is NOT ·
//	              the order and its tie-break are in the statement · the LIMIT really is the ceiling
//	              PLUS ONE · the refusal fires at ceiling+1 and returns NO rows · the positional
//	              Scan lines up with the column list, BY NAME · a driver failure is wrapped, not
//	              swallowed · no commune in the context is a panic, never a default.
//
//	NOT PROVED    anything PostgreSQL does with that statement. The columns come from the store's
//	              OWN SELECT list, so a column that does not exist in `loai_van_ban` — a typo, a
//	              name the migration never created — passes here without a murmur. Nor does this
//	              say anything about the partial index being used, the hash partitioning routing a
//	              row, or the danh_muc_ba_tang trigger refusing what it claims to refuse. That half
//	              needs a real server, which is why document_type_pg_test.go stays and becomes
//	              valuable the day VIGOV_TEST_DSN exists.
//
// The column-name check is the one worth explaining: the fake builds each row BY COLUMN NAME out of
// the statement, so reordering documentTypeColumns without reordering the Scan turns these tests red.
// Read by position, `ma`/`nhan` and `dang_dung`/`la_mac_dinh` are two pairs of adjacent same-typed
// columns — swapping either produces no error at all, only slugs where labels belong, or a form
// pre-selecting a type the commune has taken out of use.

var testTenant = tenant.ID("01JA" + strings.Repeat("A", 22))

// tenantCtx and newDocumentTypeStore are declared in document_type_pg_test.go and reused here on
// purpose: two helpers building the same context, or the same store, are two that drift.

// --- the fake driver --------------------------------------------------------------------------

type fakeStmt struct {
	sql  string
	args []driver.Value
}

// fakeRow is one row the fake returns. The values are distinct per column and, within each type,
// distinct from each other, so a mis-wired Scan shows up as WRONG DATA rather than as a zero value
// that looks plausible.
type fakeRow struct {
	id, code, label     string
	isActive, isDefault bool
	sortOrder           int
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
	case "dang_dung":
		return r.isActive
	case "la_mac_dinh":
		return r.isDefault
	case "thu_tu":
		// int64 AND NOT int: database/sql only accepts the driver.Value set, and `int` is not in
		// it. A fake that handed back `int` would fail every Scan with a message about conversion
		// rather than about the column, which is the kind of noise that gets a fake deleted.
		return int64(r.sortOrder)
	case "nguon":
		return r.source
	case "ma_nguon_re_nhanh":
		return r.branched
	default:
		// A column was added to documentTypeColumns and not here. Failing loudly beats scanning a nil
		// that "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + col)
	}
}

type fakeConnector struct {
	stmts []fakeStmt
	rows  []fakeRow
	err   error
}

func (k *fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeConn{k: k}, nil }
func (k *fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type fakeConn struct{ k *fakeConnector }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

// QueryContext is what makes this a driver.QueryerContext, so database/sql hands the statement
// over whole instead of preparing it — which is the only reason the statement can be recorded and
// asserted on at all.
func (c *fakeConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.k.stmts = append(c.k.stmts, fakeStmt{sql: q, args: vals})
	if c.k.err != nil {
		return nil, c.k.err
	}
	cols, err := columnsOfStatement(q)
	if err != nil {
		return nil, err
	}
	out := make([][]driver.Value, 0, len(c.k.rows))
	for _, r := range c.k.rows {
		one := make([]driver.Value, len(cols))
		for i, name := range cols {
			one[i] = r.value(name)
		}
		out = append(out, one)
	}
	return &fakeRows{cols: cols, rows: out}, nil
}

func columnsOfStatement(q string) ([]string, error) {
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

// newFakeStore builds the real store on top of the fake driver, through the SAME constructor
// production uses — sql.OpenDB turns a driver.Connector into a *sql.DB, and nothing in the store
// was widened to accommodate this.
func newFakeStore(k *fakeConnector) *DocumentTypeStore {
	return newDocumentTypeStore(sql.OpenDB(k))
}

// oneSampleRow is one row whose every field is distinguishable from every other: a Scan reading
// `nhan` into Code, or `dang_dung` into IsDefault, cannot produce a passing assertion.
func oneSampleRow() []fakeRow {
	return []fakeRow{{
		id: "lvb-001", code: "quyet-dinh", label: "Quyết định",
		// OPPOSITE VALUES ON PURPOSE. Both are BOOLEAN and adjacent in the column list, so equal
		// values would make a swap invisible. This row is the awkward-but-legal combination the
		// schema allows: a default that has been taken out of use.
		isActive: false, isDefault: true,
		// `thu_tu` is deliberately NOT 0 and NOT 1: a zero would be indistinguishable from an
		// unscanned field, and 1 from a length. `nguon`/`ma_nguon_re_nhanh` describe a TIER 3 row,
		// the one combination whose tier cannot be guessed from either column alone.
		sortOrder: 7, source: "he-thong", branched: true,
	}}
}

func manyRows(n int) []fakeRow {
	out := make([]fakeRow, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fakeRow{
			id:    fmt.Sprintf("lvb-%04d", i),
			code:  fmt.Sprintf("loai-%04d", i),
			label: fmt.Sprintf("Loại %04d", i),
			// dang_dung true: rows in ordinary use, which is what a commune at its ceiling would
			// actually hold. `nguon` is the commune's own, which is what a catalogue grows into.
			isActive:  true,
			sortOrder: i,
			source:    "don-vi",
		})
	}
	return out
}

// --- the statement the store builds --------------------------------------------------------------

func TestListBindsTheTenantAsParamOneFromContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of List and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	k := &fakeConnector{rows: oneSampleRow()}

	if _, err := newFakeStore(k).List(tenantCtx(string(testTenant))); err != nil {
		t.Fatalf("List lỗi: %v", err)
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
	// The same store type, a different commune in the context, a different $1. This is what
	// "scoped repository" means in practice — and a store that cached the first commune it saw,
	// or that read one from its own construction, would fail here.
	k2 := &fakeConnector{rows: oneSampleRow()}
	otherTenant := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := newFakeStore(k2).List(tenantCtx(string(otherTenant))); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if k2.stmts[0].args[0] != string(otherTenant) {
		t.Errorf("$1 = %v, muốn %q", k2.stmts[0].args[0], otherTenant)
	}
}

func TestListFiltersOnlySoftDeletedRowsAndOrdersStably(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always. And
	// the order is total: `thu_tu` is the commune's own arrangement and `nhan` breaks ties, so two
	// calls cannot return the same rows in a different sequence.
	k := &fakeConnector{rows: oneSampleRow()}

	if _, err := newFakeStore(k).List(tenantCtx(string(testTenant))); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	q := k.stmts[0].sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu_tu, nhan") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// `dang_dung` must NOT be in the predicate, and this assertion is the one most likely to be
	// "fixed" by somebody being helpful: a type taken out of use is still read — the catalogue
	// screen shows it with a "Đã tắt" chip, and a document already registered under it needs its
	// label. Filtering here would silently rewrite what old documents display.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") ||
		strings.Contains(q, "AND dang_dung") {
		t.Errorf("câu lệnh lọc mất loại đã tắt: %q", q)
	}
}

func TestListFetchesTheCeilingPlusOne(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	k := &fakeConnector{rows: oneSampleRow()}

	if _, err := newFakeStore(k).List(tenantCtx(string(testTenant))); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	l := k.stmts[0]
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(MaxDocumentTypes+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], MaxDocumentTypes+1)
	}
}

func TestListReadsTheRightTable(t *testing.T) {
	// The table name is built by the store, not by Scoped: a typo here would reach PostgreSQL as a
	// relation that does not exist, and the pg suite is the only place that catches THAT. What this
	// asserts is the cheaper half — that the read goes to the catalogue table and to no other.
	k := &fakeConnector{rows: oneSampleRow()}

	if _, err := newFakeStore(k).List(tenantCtx(string(testTenant))); err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if !strings.Contains(k.stmts[0].sql, " FROM loai_van_ban ") {
		t.Errorf("đọc nhầm bảng: %q", k.stmts[0].sql)
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestListReadsEachColumnIntoItsField(t *testing.T) {
	// THE TEST THAT MATTERS MOST HERE, and the reason the fake builds rows by column NAME: the Scan
	// is positional, and two pairs of adjacent same-typed columns can be swapped without the
	// compiler, `go vet` or an ordinary test noticing.
	k := &fakeConnector{rows: oneSampleRow()}

	out, err := newFakeStore(k).List(tenantCtx(string(testTenant)))
	if err != nil {
		t.Fatalf("List lỗi: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(out))
	}
	one := out[0]
	if one.ID != "lvb-001" || one.Code != "quyet-dinh" || one.Label != "Quyết định" {
		t.Errorf("ba cột TEXT đọc sai chỗ — ma/nhan có thể đã hoán vị: %+v", one)
	}
	// The pair that no type can catch: both BOOLEAN, adjacent, and set to OPPOSITE values by the
	// fixture. Swapped, the screen pre-selects a type the commune has taken out of use.
	if one.IsActive || !one.IsDefault {
		t.Errorf("dang_dung / la_mac_dinh đọc ngược: %+v", one)
	}
	// The three columns the WRITE surface added. `thu_tu` is what the configuration screen edits;
	// `nguon` and `ma_nguon_re_nhanh` are what decide which buttons that screen may even draw, so a
	// Scan reading them into the wrong field offers `Xoá` on a row the database will refuse to
	// delete — a button that always fails, on the one screen an administrator uses to fix things.
	if one.SortOrder != 7 {
		t.Errorf("thu_tu đọc sai: %+v", one)
	}
	if one.Source != "he-thong" || !one.BranchedInSource {
		t.Errorf("nguon / ma_nguon_re_nhanh đọc sai: %+v", one)
	}
	if one.Tier() != domain.TierBranched {
		t.Errorf("tầng suy ra = %d, muốn %d (tầng 3)", one.Tier(), domain.TierBranched)
	}
}

func TestListOfACommuneWithNoRowsIsAnEmptySlice(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and sows nothing, and the
	// onboarding step that would sow a commune's system rows does not exist. An empty catalogue is
	// correct, not a failure — and it is a slice, never a nil the caller has to branch on.
	k := &fakeConnector{}

	out, err := newFakeStore(k).List(tenantCtx(string(testTenant)))
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

// --- the ceiling ------------------------------------------------------------------------------------

func TestListAtTheCeilingStillReturnsEverything(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid — and the screen it breaks is the one a document is
	// registered on.
	k := &fakeConnector{rows: manyRows(MaxDocumentTypes)}

	out, err := newFakeStore(k).List(tenantCtx(string(testTenant)))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(out) != MaxDocumentTypes {
		t.Errorf("nhận %d dòng, muốn %d", len(out), MaxDocumentTypes)
	}
}

func TestListOverTheCeilingRefusesAndReturnsNoRows(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	k := &fakeConnector{rows: manyRows(MaxDocumentTypes + 1)}

	out, err := newFakeStore(k).List(tenantCtx(string(testTenant)))
	if !errors.Is(err, ErrTooManyDocumentTypes) {
		t.Fatalf("lỗi = %v, muốn ErrTooManyDocumentTypes", err)
	}
	if out != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(out))
	}
}

// --- failures ---------------------------------------------------------------------------------------

func TestListStoreErrorIsWrappedNotSwallowed(t *testing.T) {
	// Rule 6 of the service pattern: wrapped with %w, never swallowed with `_`. The handler
	// distinguishes the ceiling from an ordinary failure with errors.Is, and that only works if the
	// chain is intact — a fmt.Errorf with %v here would make the two indistinguishable and the
	// route would log the wrong sentence about a commune's data.
	cause := errors.New("cơ sở dữ liệu không phản hồi")
	k := &fakeConnector{err: cause}

	out, err := newFakeStore(k).List(tenantCtx(string(testTenant)))
	if !errors.Is(err, cause) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, cause)
	}
	if errors.Is(err, ErrTooManyDocumentTypes) {
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
	k := &fakeConnector{rows: oneSampleRow()}
	_, _ = newFakeStore(k).List(context.Background())
}
