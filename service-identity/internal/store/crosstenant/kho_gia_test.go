package crosstenant

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/httpx"
)

// The fake database/sql driver the cross-commune reads share.
//
// WHAT IT PROVES, AND WHAT IT CANNOT. It runs with NO PostgreSQL: it RECORDS the statement the
// store built and hands back rows BUILT BY COLUMN NAME out of that statement's own SELECT list.
// So it decides which columns are read, in which order, with which predicate and which bound —
// and it decides nothing at all about what PostgreSQL would do with the statement. The
// *_pg_test.go files next door are the only place the real schema is ever touched, and they
// SKIP without VIGOV_TEST_DSN while the package still prints `ok`.
//
// THE COLUMN-NAME CHECK IS THE POINT: reordering a SELECT list without reordering the Scan
// turns these tests red. Both statements in this package have adjacent same-typed columns whose
// swap produces no error at all — three TEXT columns in a row for the pairing code, two
// nullable timestamps whose swap turns "somebody cancelled this" into "somebody used this".

type lenhCT struct {
	sql  string
	args []driver.Value
}

type khoCT struct {
	lenh []lenhCT
	hang []map[string]driver.Value
	loi  error
}

func (k *khoCT) cuoi() lenhCT { return k.lenh[len(k.lenh)-1] }

func dbGiaCT(k *khoCT) *sql.DB { return sql.OpenDB(ketNoiCT{k: k}) }

type ketNoiCT struct{ k *khoCT }

func (c ketNoiCT) Connect(context.Context) (driver.Conn, error) { return &connCT{k: c.k}, nil }
func (c ketNoiCT) Driver() driver.Driver                        { return trinhCT{} }

type trinhCT struct{}

func (trinhCT) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connCT struct{ k *khoCT }

func (c *connCT) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connCT) Close() error { return nil }
func (c *connCT) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connCT) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhCT{sql: q, args: gt})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	cot, err := cotTrongCauLenhCT(q)
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
	return &rowsCT{cot: cot, hang: dong}, nil
}

type rowsCT struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsCT) Columns() []string { return r.cot }
func (r *rowsCT) Close() error      { return nil }
func (r *rowsCT) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

// cotTrongCauLenhCT reads the SELECT list out of the statement the STORE built, so these tests
// are pinned to the real column list rather than to a copy of it.
func cotTrongCauLenhCT(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, "FROM ")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var ra []string
	for _, bieu := range tachDauPhayCT(q[i+len("SELECT ") : j]) {
		ra = append(ra, tenCotCT(bieu))
	}
	return ra, nil
}

// tachDauPhayCT splits on commas AT DEPTH ZERO: `coalesce(ly_do_tu_choi,”)` carries a comma of
// its own, and a naive split would turn one column into two.
func tachDauPhayCT(s string) []string {
	var ra []string
	sau, muc := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			muc++
		case ')':
			muc--
		case ',':
			if muc == 0 {
				ra = append(ra, s[sau:i])
				sau = i + 1
			}
		}
	}
	return append(ra, s[sau:])
}

// tenCotCT reduces one SELECT-list expression to the column it reads.
func tenCotCT(bieu string) string {
	s := strings.Join(strings.Fields(bieu), "")
	if rest, co := strings.CutPrefix(s, "coalesce("); co {
		if i := strings.Index(rest, ","); i >= 0 {
			return rest[:i]
		}
		return strings.TrimSuffix(rest, ")")
	}
	return s
}

// phienGiaCT stands in for the citizen session registry so the real edge can resolve a session.
type phienGiaCT struct{ p httpx.CitizenSession }

func (s phienGiaCT) TraCuu(context.Context, string) (httpx.CitizenSession, bool) { return s.p, true }

// ctxKhamPha builds the context a citizen request carries AT THE DISCOVERY LAYER, by running
// the real edge.
//
// WHY NOT context.WithValue: core/httpx keeps the context key unexported on purpose, so nothing
// outside the edge can forge a citizen session — that IS the mechanism behind rule 4, invariant
// 2, and a test that forged one would be testing a path production does not have.
//
// httpx.KhongThuocXa IS THE RIGHT CLASS HERE, not XaTuPhien: a citizen who has signed in but
// not chosen a commune is exactly who asks which communes they are known to (ADR 0005), and
// their session legitimately carries an empty commune.
func ctxKhamPha(t *testing.T, p httpx.CitizenSession) context.Context {
	t.Helper()
	var ra context.Context
	cuoi := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ra = r.Context()
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-gia-khong-phai-token-that")
	rec := httptest.NewRecorder()
	httpx.CitizenEdge(phienGiaCT{p: p})(
		httpx.KhongThuocXa("test: lớp khám phá, công dân chưa chọn xã")(cuoi),
	).ServeHTTP(rec, req)
	if ra == nil {
		t.Fatalf("rìa kênh công dân không gọi tới handler, mã %d", rec.Code)
	}
	return ra
}
