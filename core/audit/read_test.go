package audit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Tests for Log.Read on a fake database/sql driver.
//
// WHAT A FAKE CAN PROVE HERE, AND WHAT IT CANNOT. It proves the statement this package BUILDS — the
// commune bound to $1, each filter's clause and argument, the default exclusion of read entries, the
// order and the limit+1 — and everything done with the rows and the transaction: which fields are
// blanked for a citizen, that the read and its entry share one transaction, and that a failed entry
// returns no page. It cannot prove PostgreSQL evaluates that statement the way the clauses say; that
// half is service-identity/internal/store/audit_log_pg_test.go, which skips without VIGOV_TEST_DSN.

// --- fake driver -------------------------------------------------------------------------------

type rdStmt struct {
	kind string // "query" | "exec" | "begin" | "commit" | "rollback"
	sql  string
	args []driver.Value
	tx   int // the transaction the statement ran in; 0 = none
}

type rdDB struct {
	mu       sync.Mutex
	log      []rdStmt
	txSeq    int
	curTx    int
	rows     [][]driver.Value
	queryErr error
	execErr  error
}

func (d *rdDB) record(s rdStmt) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s.tx = d.curTx
	d.log = append(d.log, s)
}

func (d *rdDB) all() []rdStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]rdStmt(nil), d.log...)
}

func (d *rdDB) of(kind string) []rdStmt {
	var out []rdStmt
	for _, s := range d.all() {
		if s.kind == kind {
			out = append(out, s)
		}
	}
	return out
}

type rdConnector struct{ d *rdDB }

func (c rdConnector) Connect(context.Context) (driver.Conn, error) { return &rdConn{d: c.d}, nil }
func (c rdConnector) Driver() driver.Driver                        { return rdDriver{} }

type rdDriver struct{}

func (rdDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type rdConn struct{ d *rdDB }

func (c *rdConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *rdConn) Close() error                        { return nil }
func (c *rdConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *rdConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.txSeq++
	c.d.curTx = c.d.txSeq
	c.d.mu.Unlock()
	c.d.record(rdStmt{kind: "begin"})
	return rdTx{d: c.d}, nil
}

func vals(args []driver.NamedValue) []driver.Value {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	return v
}

func (c *rdConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(rdStmt{kind: "query", sql: q, args: vals(args)})
	if c.d.queryErr != nil {
		return nil, c.d.queryErr
	}
	return &rdRows{rows: c.d.rows}, nil
}

func (c *rdConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(rdStmt{kind: "exec", sql: q, args: vals(args)})
	if c.d.execErr != nil {
		return nil, c.d.execErr
	}
	return driver.RowsAffected(1), nil
}

type rdTx struct{ d *rdDB }

func (t rdTx) Commit() error   { t.d.record(rdStmt{kind: "commit"}); t.end(); return nil }
func (t rdTx) Rollback() error { t.d.record(rdStmt{kind: "rollback"}); t.end(); return nil }
func (t rdTx) end() {
	t.d.mu.Lock()
	t.d.curTx = 0
	t.d.mu.Unlock()
}

type rdRows struct {
	rows [][]driver.Value
	i    int
}

func (r *rdRows) Columns() []string {
	return []string{"id", "at", "actor_kind", "actor_id", "actor_ip", "action", "subject", "delta"}
}
func (r *rdRows) Close() error { return nil }
func (r *rdRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// --- fixtures ----------------------------------------------------------------------------------

var (
	t0     = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	reader = Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
)

func row(id int64, at time.Time, kind, actor, ip, action, subject string, delta []byte) []driver.Value {
	var d driver.Value
	if delta != nil {
		d = delta
	}
	return []driver.Value{id, at, kind, actor, ip, action, subject, d}
}

func newLog(t *testing.T, d *rdDB, opts ...Option) (*Log, context.Context) {
	t.Helper()
	db := sql.OpenDB(rdConnector{d: d})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewLog(store.New(db), opts...), tenant.Into(context.Background(), xaA)
}

func theRead(t *testing.T, d *rdDB) rdStmt {
	t.Helper()
	q := d.of("query")
	if len(q) != 1 {
		t.Fatalf("%d SELECT, want 1: %+v", len(q), d.all())
	}
	return q[0]
}

func readEntries(t *testing.T, d *rdDB) []readDelta {
	t.Helper()
	var out []readDelta
	for _, s := range d.of("exec") {
		if !strings.Contains(s.sql, "INSERT INTO audit_log") {
			t.Fatalf("unexpected statement %q", s.sql)
		}
		if s.args[4] != ActionReadLog {
			t.Fatalf("entry action = %v, want %s", s.args[4], ActionReadLog)
		}
		if s.args[1] != reader.ID || s.args[5] != reader.ID {
			t.Errorf("entry actor/subject = %v/%v, want the reader's business code %s", s.args[1], s.args[5], reader.ID)
		}
		var rd readDelta
		b, _ := s.args[7].([]byte)
		if err := json.Unmarshal(b, &rd); err != nil {
			t.Fatalf("entry delta is not JSON: %q", b)
		}
		out = append(out, rd)
	}
	return out
}

// --- tests -------------------------------------------------------------------------------------

func TestRead_DefaultHidesReadEntriesOrdersAndScopesToCommune(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	if _, err := l.Read(ctx, reader, Query{}); err != nil {
		t.Fatal(err)
	}
	s := theRead(t, d)
	for _, want := range []string{
		"FROM audit_log WHERE tenant_id = $1 ",
		"AND action <> $2",
		"ORDER BY at DESC, id DESC LIMIT $3",
	} {
		if !strings.Contains(s.sql, want) {
			t.Errorf("SQL lacks %q:\n%s", want, s.sql)
		}
	}
	if s.args[0] != string(xaA) {
		t.Errorf("$1 = %v, want the context's commune — rule 1", s.args[0])
	}
	if s.args[1] != ActionReadLog {
		t.Errorf("$2 = %v, want the read verb excluded by default", s.args[1])
	}
	if s.args[2] != int64(page.DefaultLimit+1) {
		t.Errorf("limit arg = %v, want DefaultLimit+1", s.args[2])
	}
}

func TestRead_ExplicitReadActionShowsReadEntries(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	if _, err := l.Read(ctx, reader, Query{Action: ActionReadLog}); err != nil {
		t.Fatal(err)
	}
	s := theRead(t, d)
	if strings.Contains(s.sql, "action <>") || !strings.Contains(s.sql, "AND action = $2") {
		t.Fatalf("action=%s must be an exact match, not the default exclusion:\n%s", ActionReadLog, s.sql)
	}
	if s.args[1] != ActionReadLog {
		t.Errorf("$2 = %v", s.args[1])
	}
}

func TestRead_FiltersAreExactAndHalfOpen(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	q := Query{
		From: "2026-09-01T00:00:00+07:00", To: "2026-10-01T00:00:00+07:00",
		Actor: "CB-00999", Action: "khoa_tai_khoan_can_bo", Subject: "CB-00555",
	}
	if _, err := l.Read(ctx, reader, q); err != nil {
		t.Fatal(err)
	}
	s := theRead(t, d)
	for _, want := range []string{
		"AND at >= $2", "AND at < $3",
		"AND actor_id = $4 AND actor_kind <> 'citizen'",
		"AND action = $5", "AND subject = $6", "LIMIT $7",
	} {
		if !strings.Contains(s.sql, want) {
			t.Errorf("SQL lacks %q:\n%s", want, s.sql)
		}
	}
	if strings.Contains(s.sql, "at <= ") || strings.Contains(s.sql, "at > ") {
		t.Errorf("range must be half-open [from, to):\n%s", s.sql)
	}
	from, _ := time.Parse(time.RFC3339, q.From)
	to, _ := time.Parse(time.RFC3339, q.To)
	if !s.args[1].(time.Time).Equal(from) || !s.args[2].(time.Time).Equal(to) {
		t.Errorf("from/to args = %v/%v", s.args[1], s.args[2])
	}
	for i, want := range []string{"CB-00999", "khoa_tai_khoan_can_bo", "CB-00555"} {
		if s.args[3+i] != want {
			t.Errorf("arg $%d = %v, want %s", 4+i, s.args[3+i], want)
		}
	}
	// The read's own entry summarises exactly these filters.
	e := readEntries(t, d)
	if len(e) != 1 || e[0].Outcome != "ok" || e[0].Actor != "CB-00999" ||
		e[0].Action != "khoa_tai_khoan_can_bo" || e[0].Subject != "CB-00555" ||
		e[0].From != "2026-08-31T17:00:00Z" || e[0].To != "2026-09-30T17:00:00Z" {
		t.Errorf("read entry summary = %+v", e)
	}
}

func TestRead_RejectedRangeRunsNoSelectButLeavesAnEntry(t *testing.T) {
	for name, q := range map[string]Query{
		"from equals to": {From: "2026-09-01T00:00:00Z", To: "2026-09-01T00:00:00Z"},
		"from after to":  {From: "2026-09-02T00:00:00Z", To: "2026-09-01T00:00:00Z"},
		"not RFC 3339":   {From: "01/09/2026"},
	} {
		t.Run(name, func(t *testing.T) {
			d := &rdDB{}
			l, ctx := newLog(t, d)
			_, err := l.Read(ctx, reader, q)
			if !errors.Is(err, ErrRange) {
				t.Fatalf("err = %v, want ErrRange", err)
			}
			if st, _, _ := HTTPError(err); st != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", st)
			}
			if len(d.of("query")) != 0 {
				t.Error("a rejected filter ran a SELECT")
			}
			e := readEntries(t, d)
			if len(e) != 1 || e[0].Outcome != "rejected" || e[0].Reason != "invalid_range" {
				t.Fatalf("rejected read left %+v, want one 'rejected' entry — ADR 0054 §5 'Ghi vết lỗi'", e)
			}
			if len(d.of("commit")) != 1 {
				t.Error("the rejection's entry was not committed")
			}
		})
	}
}

func TestRead_OverlongFilterIs400(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	_, err := l.Read(ctx, reader, Query{Subject: strings.Repeat("x", maxFilterLen+1)})
	if st, _, _ := HTTPError(err); st != http.StatusBadRequest || !errors.Is(err, ErrFilter) {
		t.Fatalf("err = %v status %d, want ErrFilter 400", err, st)
	}
}

func TestRead_CursorWalksAtThenIdDescending(t *testing.T) {
	d := &rdDB{rows: [][]driver.Value{
		row(30, t0, "staff", "CB-1", "10.0.0.1", "a", "s1", nil),
		row(29, t0, "staff", "CB-1", "10.0.0.1", "b", "s2", nil), // same instant: id breaks the tie
		row(12, t0.Add(-time.Hour), "staff", "CB-1", "10.0.0.1", "c", "s3", nil),
	}}
	l, ctx := newLog(t, d)
	res, err := l.Read(ctx, reader, Query{Limit: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || !res.HasMore || res.NextCursor == "" {
		t.Fatalf("got %d items has_more=%v cursor=%q, want 2/true/non-empty", len(res.Items), res.HasMore, res.NextCursor)
	}
	if res.Items[1].Action != "b" {
		t.Errorf("the limit+1-th row leaked into the page")
	}

	d2 := &rdDB{}
	l2, ctx2 := newLog(t, d2)
	if _, err := l2.Read(ctx2, reader, Query{Limit: "2", Cursor: res.NextCursor}); err != nil {
		t.Fatal(err)
	}
	s := theRead(t, d2)
	if !strings.Contains(s.sql, "AND (at, id) < ($3, $4) ORDER BY at DESC, id DESC LIMIT $5") {
		t.Fatalf("keyset clause wrong:\n%s", s.sql)
	}
	if !s.args[2].(time.Time).Equal(t0) || s.args[3] != int64(29) {
		t.Errorf("anchor = (%v, %v), want (t0, 29) — the LAST RETURNED row, bound as int64", s.args[2], s.args[3])
	}
	if e := readEntries(t, d2); len(e) != 1 || !e[0].Cursor {
		t.Errorf("entry must record that a cursor was used: %+v", e)
	}
}

func TestRead_CursorWithNonIntegerIdIs400(t *testing.T) {
	col := page.Col("at", "at", page.KindTime)
	bad := page.Encode(col, page.Desc, page.Anchor{Key: page.TimeKey(t0), ID: "01JNOTANUMBER"})
	d := &rdDB{}
	l, ctx := newLog(t, d)
	_, err := l.Read(ctx, reader, Query{Cursor: bad})
	if st, code, _ := HTTPError(err); st != http.StatusBadRequest || code != "invalid_cursor" {
		t.Fatalf("status %d code %q, want 400 invalid_cursor (err %v)", st, code, err)
	}
	if len(d.of("query")) != 0 {
		t.Error("a forged cursor reached the database")
	}
}

func TestRead_CitizenEntriesHaveNoCodeAndNoIP(t *testing.T) {
	stored := []byte(`{"truoc":"0900***000"}`)
	d := &rdDB{rows: [][]driver.Value{
		row(3, t0, "citizen", "cd-01JINTERNALCITIZENID", "203.0.113.9", "gui_phan_anh", "PA-X", stored),
		row(2, t0, "staff", "CB-00123", "10.0.0.7", "tiep_nhan", "PA-X", nil),
		row(1, t0, "system", SystemActor, "", "tu_dong", "PA-X", nil),
	}}
	l, ctx := newLog(t, d)
	res, err := l.Read(ctx, reader, Query{})
	if err != nil {
		t.Fatal(err)
	}
	c, s, sys := res.Items[0], res.Items[1], res.Items[2]
	if c.ActorCode != "" || c.ActorIP != "" || c.ActorKind != "citizen" {
		t.Errorf("citizen entry = %+v, want empty actor_code and actor_ip (ADR 0054 §4)", c)
	}
	if raw, ok := c.Delta.(json.RawMessage); !ok || string(raw) != string(stored) {
		t.Errorf("delta = %s, want exactly as stored", c.Delta)
	}
	if s.ActorCode != "CB-00123" || s.ActorIP != "10.0.0.7" {
		t.Errorf("staff entry lost its code or IP: %+v", s)
	}
	if sys.ActorCode != SystemActor {
		t.Errorf("system entry = %+v", sys)
	}
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), `"delta":null`) || strings.Contains(string(b), `"id"`) {
		t.Errorf("wire shape = %s — null delta, no id", b)
	}
	for _, f := range []string{"at", "actor_kind", "actor_code", "actor_ip", "action", "subject", "delta"} {
		if !strings.Contains(string(b), `"`+f+`":`) {
			t.Errorf("wire shape lacks %q: %s", f, b)
		}
	}
}

func TestRead_ReadAndItsEntryShareOneTransaction(t *testing.T) {
	d := &rdDB{rows: [][]driver.Value{row(1, t0, "staff", "CB-1", "", "a", "s", nil)}}
	l, ctx := newLog(t, d)
	if _, err := l.Read(ctx, reader, Query{}); err != nil {
		t.Fatal(err)
	}
	q, x := theRead(t, d), d.of("exec")
	if len(x) != 1 || q.tx == 0 || q.tx != x[0].tx {
		t.Fatalf("SELECT in tx %d, entry in %+v — they must share ONE transaction (ADR 0054 §5)", q.tx, x)
	}
	if len(d.of("commit")) != 1 || len(d.of("rollback")) != 0 {
		t.Errorf("want one commit and no rollback: %+v", d.all())
	}
	if e := readEntries(t, d); e[0].Returned != 1 {
		t.Errorf("returned = %d, want 1", e[0].Returned)
	}
}

func TestRead_EntryFailureReturnsNoPage(t *testing.T) {
	d := &rdDB{
		rows:    [][]driver.Value{row(1, t0, "staff", "CB-1", "", "a", "s", nil)},
		execErr: errors.New("disk full"),
	}
	l, ctx := newLog(t, d)
	res, err := l.Read(ctx, reader, Query{})
	if err == nil {
		t.Fatal("the read's entry failed and the page was returned anyway — stop condition #5")
	}
	if len(res.Items) != 0 {
		t.Errorf("%d items returned with no trail", len(res.Items))
	}
	if st, _, _ := HTTPError(err); st != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", st)
	}
	if len(d.of("commit")) != 0 {
		t.Error("something committed although the entry failed")
	}
}

func TestRead_QueryFailureStillLeavesAnEntry(t *testing.T) {
	d := &rdDB{queryErr: errors.New("connection reset")}
	l, ctx := newLog(t, d)
	if _, err := l.Read(ctx, reader, Query{Subject: "VB-1"}); err == nil {
		t.Fatal("want the database error")
	}
	e := readEntries(t, d)
	if len(e) != 1 || e[0].Outcome != "error" || e[0].Subject != "VB-1" {
		t.Fatalf("entries = %+v, want one 'error' entry with the filter", e)
	}
	if len(d.of("rollback")) != 1 || len(d.of("commit")) != 1 {
		t.Errorf("want the read tx rolled back and the error entry committed: %+v", d.all())
	}
}

func TestRead_NoReaderRunsNothing(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	_, err := l.Read(ctx, Actor{Kind: "staff", IP: "10.0.0.7"}, Query{})
	if !errors.Is(err, ErrNoReader) {
		t.Fatalf("err = %v, want ErrNoReader", err)
	}
	if n := len(d.all()); n != 0 {
		t.Errorf("%d statements ran for a read nobody can be named for", n)
	}
	if st, _, _ := HTTPError(err); st != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 — wiring gone wrong, not a client error", st)
	}
}

func TestRead_NoCommuneRefusesRatherThanReadingOpen(t *testing.T) {
	d := &rdDB{}
	db := sql.OpenDB(rdConnector{d: d})
	t.Cleanup(func() { db.Close() })
	l := NewLog(store.New(db))
	defer func() {
		if recover() == nil {
			t.Fatal("a read with no commune in the context did not refuse")
		}
		if len(d.of("query")) != 0 {
			t.Error("a SELECT ran with no commune")
		}
	}()
	_, _ = l.Read(context.Background(), reader, Query{})
}

func TestRead_HiddenSubjects(t *testing.T) {
	const sub = "SELECT ma_tra_cuu FROM phieu_phan_anh WHERE tenant_id = $1 AND linh_vuc = 'can-bo'"
	d := &rdDB{}
	l, ctx := newLog(t, d, WithHiddenSubjects(sub))
	if _, err := l.Read(ctx, reader, Query{}); err != nil {
		t.Fatal(err)
	}
	if s := theRead(t, d); !strings.Contains(s.sql, "AND subject NOT IN (SELECT h.s FROM ("+sub+") AS h(s) WHERE h.s IS NOT NULL)") {
		t.Fatalf("hidden-subject clause missing:\n%s", s.sql)
	}

	d2 := &rdDB{}
	l2, ctx2 := newLog(t, d2, WithHiddenSubjects(sub))
	if _, err := l2.Read(ctx2, reader, Query{SeeHidden: true}); err != nil {
		t.Fatal(err)
	}
	if s := theRead(t, d2); strings.Contains(s.sql, "NOT IN") {
		t.Errorf("SeeHidden did not lift the exclusion:\n%s", s.sql)
	}

	for name, bad := range map[string]string{
		"second placeholder": "SELECT ma FROM x WHERE tenant_id = $1 AND y = $2",
		"two statements":     "SELECT 1; DELETE FROM audit_log",
		"not a select":       "DELETE FROM audit_log",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("accepted")
				}
			}()
			WithHiddenSubjects(bad)
		})
	}
}

func TestRead_LimitAboveCapIsCapped(t *testing.T) {
	d := &rdDB{}
	l, ctx := newLog(t, d)
	if _, err := l.Read(ctx, reader, Query{Limit: "10000"}); err != nil {
		t.Fatal(err)
	}
	s := theRead(t, d)
	if s.args[len(s.args)-1] != int64(page.MaxLimit+1) {
		t.Errorf("limit arg = %v, want MaxLimit+1", s.args[len(s.args)-1])
	}
}
