package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The SQL of "Lời hệ thống" since migration 0033 (store/system_message_override.go, scanOverride and
// SetActive; store/system_message_custom.go) on a fake driver that records every statement and answers
// reads BY COLUMN NAME — so a column list and its positional Scan that drift apart come back as wrong
// data, not as a plausible zero.
//
// PROVED HERE   the commune is $1 and comes from the CONTEXT on every statement · `is_active` is read
//               INVERTED into Inactive and written back as !Inactive · every read but the key check
//               excludes soft-deleted rows, and the key check does NOT · the editable columns are the
//               only ones an UPDATE names · a unique violation on INSERT is ErrMessageCodeTaken.
// NOT PROVED    anything PostgreSQL does — the CHECKs, the guard trigger, the partitions.

type smStmt struct {
	sql  string
	args []driver.Value
}

type smDB struct {
	stmts    []smStmt
	affected int64
	execErr  error
	rows     []map[string]driver.Value
}

type smConnector struct{ d *smDB }

func (c smConnector) Connect(context.Context) (driver.Conn, error) { return &smConn{d: c.d}, nil }
func (c smConnector) Driver() driver.Driver                        { return smDriver{} }

type smDriver struct{}

func (smDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type smConn struct{ d *smDB }

func (c *smConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *smConn) Close() error                        { return nil }
func (c *smConn) Begin() (driver.Tx, error)           { return smTx{}, nil }
func (c *smConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return smTx{}, nil
}

func smVals(args []driver.NamedValue) []driver.Value {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	return v
}

func (c *smConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.stmts = append(c.d.stmts, smStmt{sql: q, args: smVals(args)})
	if c.d.execErr != nil {
		return nil, c.d.execErr
	}
	return driver.RowsAffected(c.d.affected), nil
}

// smCols is the SELECT list of a statement, split on commas — enough for these statements.
func smCols(q string) []string {
	from := strings.Index(q, " FROM ")
	sel := strings.TrimSpace(q[len("SELECT "):from])
	var out []string
	for _, c := range strings.Split(sel, ",") {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

func (c *smConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.stmts = append(c.d.stmts, smStmt{sql: q, args: smVals(args)})
	cols := smCols(q)
	out := make([][]driver.Value, 0, len(c.d.rows))
	for _, r := range c.d.rows {
		one := make([]driver.Value, len(cols))
		for i, col := range cols {
			v, ok := r[col]
			if !ok {
				panic("fake driver: no fixture value for column " + col)
			}
			one[i] = v
		}
		out = append(out, one)
	}
	return &smRows{cols: cols, rows: out}, nil
}

type smTx struct{}

func (smTx) Commit() error   { return nil }
func (smTx) Rollback() error { return nil }

type smRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *smRows) Columns() []string { return r.cols }
func (r *smRows) Close() error      { return nil }
func (r *smRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

var (
	smCommune = tenant.ID("01JA" + strings.Repeat("S", 22))
	smAt      = time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
)

func smStore(d *smDB) (*SystemMessageOverrideStore, *pkgstore.DB, context.Context) {
	h := pkgstore.New(sql.OpenDB(smConnector{d: d}))
	return NewSystemMessageOverrideStore(h), h, tenant.Into(context.Background(), smCommune)
}

func smInTx(t *testing.T, h *pkgstore.DB, ctx context.Context, fn func(*pkgstore.ScopedTx) error) error {
	t.Helper()
	return h.For(ctx).Tx(ctx, fn)
}

// smOne asserts exactly one statement containing `frag`, scoped to the context's commune as $1.
func smOne(t *testing.T, d *smDB, frag string) smStmt {
	t.Helper()
	var hit []smStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, frag) {
			hit = append(hit, s)
		}
	}
	if len(hit) != 1 {
		t.Fatalf("statements containing %q = %d, want 1 (%v)", frag, len(hit), d.stmts)
	}
	// A read or an UPDATE filters `tenant_id = $1`; an INSERT names tenant_id first and binds it to $1.
	scoped := strings.Contains(hit[0].sql, "tenant_id = $1") ||
		(strings.Contains(hit[0].sql, "(tenant_id, ") && strings.Contains(hit[0].sql, "VALUES ($1,"))
	if !scoped || hit[0].args[0] != string(smCommune) {
		t.Errorf("not scoped to the context's commune as $1: %q %v", hit[0].sql, hit[0].args)
	}
	return hit[0]
}

func overrideRow(active bool) map[string]driver.Value {
	return map[string]driver.Value{"id": "o1", "message_key": domain.KeyFeedbackNeverPublic,
		"message_text": "Câu của xã.", "updated_at": smAt, "updated_by": "CB-1", "is_active": active}
}

// --- system_message_override: the inverted switch -------------------------------------------------

func TestOverrideScanReadsIsActiveInverted(t *testing.T) {
	for _, active := range []bool{true, false} {
		d := &smDB{rows: []map[string]driver.Value{overrideRow(active)}}
		s, h, ctx := smStore(d)
		live, err := s.ListLive(ctx)
		if err != nil || len(live) != 1 {
			t.Fatalf("ListLive: %v %v", live, err)
		}
		if live[0].Inactive == active || live[0].Text != "Câu của xã." || live[0].UpdatedBy != "CB-1" {
			t.Errorf("is_active=%v read as %+v", active, live[0])
		}
		if st := smOne(t, d, "FROM system_message_override"); !strings.Contains(st.sql, "deleted_at IS NULL") {
			t.Errorf("list reads reverted wordings: %q", st.sql)
		}

		d.stmts = nil
		var got *domain.MessageOverride
		if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error {
			var e error
			got, e = s.LiveForUpdate(ctx, tx, domain.KeyFeedbackNeverPublic)
			return e
		}); err != nil || got == nil || got.Inactive == active {
			t.Errorf("LiveForUpdate is_active=%v: %+v %v", active, got, err)
		}
		st := smOne(t, d, "FOR UPDATE")
		if !strings.Contains(st.sql, "deleted_at IS NULL") || st.args[1] != domain.KeyFeedbackNeverPublic {
			t.Errorf("LiveForUpdate: %q %v", st.sql, st.args)
		}
	}
}

func TestAddOverrideBindsIsActive(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		d := &smDB{affected: 1}
		s, h, ctx := smStore(d)
		o := domain.MessageOverride{ID: "o1", Key: domain.KeyFeedbackNeverPublic, Text: "C.", UpdatedAt: smAt, UpdatedBy: "CB-1", Inactive: inactive}
		if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.AddOverride(ctx, tx, o) }); err != nil {
			t.Fatal(err)
		}
		st := smOne(t, d, "INSERT INTO system_message_override")
		if !strings.Contains(st.sql, "is_active") || st.args[6] != !inactive {
			t.Errorf("Inactive=%v bound is_active=%v: %q", inactive, st.args[6], st.sql)
		}
	}
}

func TestSetActiveWritesOnlyTheSwitch(t *testing.T) {
	d := &smDB{affected: 1}
	s, h, ctx := smStore(d)
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.SetActive(ctx, tx, "o1", false, "CB-1", smAt) }); err != nil {
		t.Fatal(err)
	}
	st := smOne(t, d, "UPDATE system_message_override")
	if !strings.Contains(st.sql, "SET is_active = $3, updated_at = $4, updated_by = $5") ||
		strings.Contains(st.sql, "message_text") || !strings.Contains(st.sql, "deleted_at IS NULL") {
		t.Errorf("SetActive SQL = %q", st.sql)
	}
	if st.args[1] != "o1" || st.args[2] != false || st.args[4] != "CB-1" {
		t.Errorf("args = %v", st.args)
	}
	d.affected = 0
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.SetActive(ctx, tx, "gone", true, "CB-1", smAt) }); err == nil {
		t.Error("zero rows reported as success")
	}
}

// --- custom_system_message ------------------------------------------------------------------------

func customRow(desc driver.Value, active bool) map[string]driver.Value {
	return map[string]driver.Value{"id": "c1", "group_code": "chung", "message_key": "chung.loi-chao",
		"message_text": "Xin chào.", "description": desc, "is_active": active,
		"created_at": smAt, "created_by": "CB-1", "updated_at": smAt, "updated_by": "CB-2"}
}

func TestListCustomReadsByColumnAndExcludesDeleted(t *testing.T) {
	d := &smDB{rows: []map[string]driver.Value{customRow(nil, false), customRow("Lời chào", true)}}
	s, _, ctx := smStore(d)
	got, err := s.ListCustom(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	c := got[0]
	if c.Group != "chung" || c.Key != "chung.loi-chao" || c.Text != "Xin chào." || c.Description != "" ||
		c.Active || c.CreatedBy != "CB-1" || c.UpdatedBy != "CB-2" || got[1].Description != "Lời chào" || !got[1].Active {
		t.Errorf("read = %+v", got)
	}
	st := smOne(t, d, "FROM custom_system_message")
	if !strings.Contains(st.sql, "deleted_at IS NULL") || st.args[1] != int64(domain.TranCustomMessages+1) {
		t.Errorf("ListCustom: %q %v", st.sql, st.args)
	}
}

func TestCustomForUpdateLocksLiveRowOfThisCommune(t *testing.T) {
	d := &smDB{rows: []map[string]driver.Value{customRow(nil, true)}}
	s, h, ctx := smStore(d)
	var got *domain.CustomMessage
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error {
		var e error
		got, e = s.CustomForUpdate(ctx, tx, "chung.loi-chao")
		return e
	}); err != nil || got == nil || got.ID != "c1" {
		t.Fatalf("%+v %v", got, err)
	}
	st := smOne(t, d, "FOR UPDATE")
	if !strings.Contains(st.sql, "deleted_at IS NULL") || st.args[1] != "chung.loi-chao" {
		t.Errorf("%q %v", st.sql, st.args)
	}

	d.rows, d.stmts = nil, nil
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error {
		var e error
		got, e = s.CustomForUpdate(ctx, tx, "chung.x")
		return e
	}); err != nil || got != nil {
		t.Errorf("no row: %+v %v", got, err)
	}
}

// THE KEY CHECK COUNTS SOFT-DELETED ROWS (rule 7, invariant 3); the ceiling count does not.
func TestCustomCountsDeletedForKeyOnly(t *testing.T) {
	d := &smDB{rows: []map[string]driver.Value{{"count(*)": int64(1)}}}
	s, h, ctx := smStore(d)
	var taken bool
	var n int
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error {
		var e error
		if taken, e = s.CustomKeyTaken(ctx, tx, "chung.x"); e != nil {
			return e
		}
		n, e = s.CountLiveCustom(ctx, tx)
		return e
	}); err != nil || !taken || n != 1 {
		t.Fatalf("taken=%v n=%d err=%v", taken, n, err)
	}
	key := smOne(t, d, "message_key = $2")
	if strings.Contains(key.sql, "deleted_at") || key.args[1] != "chung.x" {
		t.Errorf("key check must see deleted rows: %q", key.sql)
	}
	var live smStmt
	for _, s := range d.stmts {
		if !strings.Contains(s.sql, "message_key") {
			live = s
		}
	}
	if !strings.Contains(live.sql, "deleted_at IS NULL") || live.args[0] != string(smCommune) {
		t.Errorf("ceiling count: %q %v", live.sql, live.args)
	}
}

func TestAddCustomBindsAndMapsUniqueViolation(t *testing.T) {
	c := domain.CustomMessage{ID: "c1", Group: "chung", Key: "chung.x", Text: "C.", Active: true,
		CreatedAt: smAt, CreatedBy: "CB-1", UpdatedAt: smAt, UpdatedBy: "CB-1"}
	d := &smDB{affected: 1}
	s, h, ctx := smStore(d)
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.AddCustom(ctx, tx, c) }); err != nil {
		t.Fatal(err)
	}
	st := smOne(t, d, "INSERT INTO custom_system_message")
	if len(st.args) != 11 || st.args[2] != "chung" || st.args[3] != "chung.x" || st.args[5] != nil || st.args[6] != true {
		t.Errorf("args = %v (description \"\" must bind NULL)", st.args)
	}

	// Two concurrent creates both pass the count; the second meets the unique key: 409, not 500.
	d.stmts, d.execErr = nil, &pgconn.PgError{Code: "23505", ConstraintName: "custom_system_message_p07_tenant_id_message_key_key"}
	err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.AddCustom(ctx, tx, c) })
	if !errors.Is(err, domain.ErrMessageCodeTaken) {
		t.Errorf("unique violation = %v, want ErrMessageCodeTaken", err)
	}
	d.execErr = &pgconn.PgError{Code: "23514"}
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.AddCustom(ctx, tx, c) }); errors.Is(err, domain.ErrMessageCodeTaken) || err == nil {
		t.Errorf("a CHECK violation read as a taken code: %v", err)
	}
}

func TestUpdateCustomNamesOnlyEditableColumns(t *testing.T) {
	d := &smDB{affected: 1}
	s, h, ctx := smStore(d)
	c := domain.CustomMessage{ID: "c1", Key: "chung.x", Text: "Mới.", Description: "", Active: false, UpdatedAt: smAt, UpdatedBy: "CB-2"}
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.UpdateCustom(ctx, tx, c) }); err != nil {
		t.Fatal(err)
	}
	st := smOne(t, d, "UPDATE custom_system_message")
	for _, banned := range []string{"message_key", "group_code", "created_at", "created_by", "deleted_"} {
		if strings.Contains(strings.SplitN(st.sql, "WHERE", 2)[0], banned) {
			t.Errorf("UPDATE writes %s: %q", banned, st.sql)
		}
	}
	if !strings.Contains(st.sql, "deleted_at IS NULL") || st.args[1] != "c1" || st.args[3] != nil || st.args[4] != false {
		t.Errorf("%q %v", st.sql, st.args)
	}
	d.affected = 0
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.UpdateCustom(ctx, tx, c) }); err == nil {
		t.Error("zero rows reported as success")
	}
}

func TestSoftDeleteCustomWritesAllThreeColumns(t *testing.T) {
	d := &smDB{affected: 1}
	s, h, ctx := smStore(d)
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDeleteCustom(ctx, tx, "c1", "CB-1", "Không dùng", smAt)
	}); err != nil {
		t.Fatal(err)
	}
	st := smOne(t, d, "custom_system_message")
	if strings.Contains(st.sql, "DELETE FROM") || !strings.Contains(st.sql, "deleted_at = $3, deleted_by = $4, delete_reason = $5") ||
		!strings.Contains(st.sql, "AND deleted_at IS NULL") {
		t.Errorf("soft delete SQL = %q", st.sql)
	}
	if st.args[3] != "CB-1" || st.args[4] != "Không dùng" {
		t.Errorf("args = %v", st.args)
	}
}

// A sentence switched off WITHOUT a commune wording (migration 0035): NULL is read as "" and "" is
// written as NULL — never as an empty string, which the text-shape CHECK refuses, and never as the
// default, which would pin it.
func TestOverrideWithoutWordingIsNull(t *testing.T) {
	row := overrideRow(false)
	row["message_text"] = nil
	d := &smDB{rows: []map[string]driver.Value{row}}
	s, h, ctx := smStore(d)
	live, err := s.ListLive(ctx)
	if err != nil || len(live) != 1 || live[0].Text != "" || !live[0].Inactive {
		t.Fatalf("ListLive of a NULL wording: %+v %v", live, err)
	}

	d = &smDB{affected: 1}
	s, h, ctx = smStore(d)
	o := domain.MessageOverride{ID: "o1", Key: row["message_key"].(string), UpdatedAt: smAt, UpdatedBy: "CB-1", Inactive: true}
	if err := smInTx(t, h, ctx, func(tx *pkgstore.ScopedTx) error { return s.AddOverride(ctx, tx, o) }); err != nil {
		t.Fatal(err)
	}
	st := smOne(t, d, "INSERT INTO system_message_override")
	if st.args[3] != nil || st.args[6] != false {
		t.Errorf("unworded switched-off row bound message_text=%#v is_active=%v, want NULL/false", st.args[3], st.args[6])
	}
}
