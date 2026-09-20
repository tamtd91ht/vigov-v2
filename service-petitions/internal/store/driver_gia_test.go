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
)

// A fake database/sql driver, shared by the two catalogue suites in this package.
//
// WHAT IT PROVES, AND WHAT IT DOES NOT — stated first, because a package that prints `ok` while
// asserting nothing is this repository's worst known trap, and this file exists precisely because
// danh_muc_nhiem_vu_pg_test.go does exactly that on a machine with no PostgreSQL.
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument, and follows the context when the context changes · the soft-delete
//	              predicate is in the statement and `dang_dung` is NOT · the order and its tie-break
//	              are in the statement · the LIMIT really is the ceiling PLUS ONE · exactly at the
//	              ceiling returns every row and ceiling+1 refuses with NO rows · the positional Scan
//	              lines up with the column list, BY NAME · a driver failure is wrapped, not
//	              swallowed · no commune in the context panics rather than defaulting.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES WITH THE STATEMENT. The driver builds each row from the
//	              column list THE STORE ITSELF HANDED IT, so a column that does not exist in the
//	              real table passes here without a murmur; so does a table name that does not
//	              exist. Nothing here touches the hash partition routing, the partial index, or the
//	              three-tier trigger in migration 0003 — the six refusals that guard an issued code
//	              are invisible to this file. That is what danh_muc_nhiem_vu_pg_test.go is for, and
//	              it is kept for the day a DSN exists; this file does not replace it.

var xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))

// ctxXa puts the commune in the context the way the edge does. THE STORES TAKE NO COMMUNE
// PARAMETER, so this is the only way to address one (rule 1, invariant 4).
func ctxXa(xa tenant.ID) context.Context {
	return tenant.Into(context.Background(), xa)
}

// --- the fake driver ------------------------------------------------------------------------

type lenhGia struct {
	sql  string
	args []driver.Value
}

// hangGia is one row the fake returns. The values are distinct per column AND the two booleans are
// set to opposite values by the fixtures, so a mis-wired Scan shows up as WRONG DATA rather than as
// a zero value that looks plausible.
type hangGia struct {
	id, ma, nhan      string
	macDinh, dangDung bool
}

func (h hangGia) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return h.id
	case "ma":
		return h.ma
	case "nhan":
		return h.nhan
	case "la_mac_dinh":
		return h.macDinh
	case "dang_dung":
		return h.dangDung
	default:
		// A column was added to one of the cot… constants and not here. Failing loudly beats
		// scanning a nil that "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + cot)
	}
}

type khoGia struct {
	lenh []lenhGia
	hang []hangGia

	// hangTheoCot is the same idea as `hang` for a table whose shape hangGia does not cover —
	// the petition register has twenty-five columns and three different SQL types, and one
	// struct covering both shapes would be a struct where half the fields are always unused.
	//
	// EXPRESSED AS column -> value, NOT AS A SLICE, and that is what preserves the property this
	// whole driver exists for: the row is assembled BY NAME from the SELECT list the store
	// itself wrote, so reordering a `cot…` constant without reordering the matching Scan comes
	// back as WRONG DATA rather than as a plausible-looking zero.
	hangTheoCot []map[string]driver.Value

	loi error
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
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGia{sql: q, args: gt})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	cot, err := cotTrongCauLenh(q)
	if err != nil {
		return nil, err
	}
	dong := make([][]driver.Value, 0, len(c.k.hang)+len(c.k.hangTheoCot))
	for _, h := range c.k.hang {
		mot := make([]driver.Value, len(cot))
		for i, c := range cot {
			mot[i] = h.giaTri(c)
		}
		dong = append(dong, mot)
	}
	for _, h := range c.k.hangTheoCot {
		mot := make([]driver.Value, len(cot))
		for i, c := range cot {
			v, co := h[c]
			if !co {
				// A column was added to a `cot…` constant and not to the fixture. Failing loudly
				// beats scanning a nil that "passes" while proving nothing — the same discipline
				// as hangGia.giaTri.
				panic("driver giả: không có giá trị mẫu cho cột " + c)
			}
			mot[i] = v
		}
		dong = append(dong, mot)
	}
	return &rowsGia{cot: cot, hang: dong}, nil
}

// cotTrongCauLenh reads the SELECT list out of the statement. THIS IS WHAT MAKES THE COLUMN CHECK
// WORK: the row is assembled by NAME from the columns the store asked for, so reordering a cot…
// constant without reordering the matching Scan turns the suites red.
func cotTrongCauLenh(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var ra []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		ra = append(ra, strings.TrimSpace(c))
	}
	return ra, nil
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

func moKhoGia(k *khoGia) *sql.DB { return sql.OpenDB(k) }

// --- assertions shared by the two suites -------------------------------------------------------
//
// ONE IMPLEMENTATION FOR BOTH CATALOGUES, because these are the properties of the SHAPE — a scoped
// read of one commune's reference list — and a second copy is a copy that gets a check added to it
// and not to the other. The differences between the two catalogues (the table, the ceiling, the
// sentinel error) are parameters here; everything they share is asserted identically.

// doiCauLenhCoXaVaLoc checks the one statement the store ran: the commune bound from the context,
// the soft-delete predicate, the stable total order, and the absence of any `dang_dung` predicate.
func doiCauLenhCoXaVaLoc(t *testing.T, k *khoGia, xa tenant.ID, bang string, tran int) {
	t.Helper()
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]

	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(xa) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xa)
	}
	if !strings.Contains(l.sql, " FROM "+bang+" ") {
		t.Errorf("đọc nhầm bảng: %q, muốn %s", l.sql, bang)
	}

	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
	// `dang_dung` MUST NOT BE IN THE PREDICATE. A row taken out of use is still read: an older task
	// holds its code, and the catalogue screen shows it with a "Đã tắt" chip. Filtering it here
	// would leave that task rendering a raw slug with no label.
	if strings.Contains(l.sql, "dang_dung =") || strings.Contains(l.sql, "dang_dung IS") {
		t.Errorf("câu lệnh lọc mất dòng đã tắt: %q", l.sql)
	}

	// The order is TOTAL: `thu_tu` is the commune's own arrangement and `ma` breaks ties — it
	// carries UNIQUE (tenant_id, ma), so two calls cannot return the same rows in a different
	// sequence. For the priority scale this is the rank itself, not a presentation detail.
	if !strings.Contains(l.sql, "ORDER BY thu_tu, ma") {
		t.Errorf("thứ tự không ổn định: %q", l.sql)
	}

	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation the route refuses to perform, performed by
	// the bound meant to prevent it.
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(tran+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], tran+1)
	}
}

// mauMotDong is the single-row fixture. Every field is distinguishable from every other, and the
// two BOOLEANs are OPPOSITE: read by position, `ma`/`nhan` and `la_mac_dinh`/`dang_dung` are two
// pairs of adjacent same-typed columns, and swapping either compiles, runs and is wrong.
func mauMotDong(id, ma, nhan string) []hangGia {
	return []hangGia{{id: id, ma: ma, nhan: nhan, macDinh: true, dangDung: false}}
}

func nhieuDong(n int) []hangGia {
	ra := make([]hangGia, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, hangGia{
			id:   fmt.Sprintf("dm-%04d", i),
			ma:   fmt.Sprintf("mau-%04d", i),
			nhan: fmt.Sprintf("Mẫu %d", i),
			// dangDung true here and false in mauMotDong: neither value may be the one the code
			// happens to assume.
			dangDung: true,
		})
	}
	return ra
}
