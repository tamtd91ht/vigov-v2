package store

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// A fake driver under the REAL DataEncryptionKeyStore — and the REAL crypto.Envelope on top of it,
// so the DEKStore contract is exercised the way the envelope actually calls it. What this does NOT
// prove: the ON CONFLICT, the compare-and-swap or the trigger inside PostgreSQL. The fake answers
// "rows affected" as the database would, so the Go side of each contract is what is under test.

type dekStmt struct {
	sql  string
	args []driver.Value
}

type dekDB struct {
	mu    sync.Mutex
	stmts []dekStmt

	begun, committed, rolledBack int
	// affected is what the next business INSERT/UPDATE reports (the audit INSERT always reports 1).
	affected int64
	// stored is the row GetDEK returns; nil means none. A successful INSERT/UPDATE sets it, so the
	// envelope's read-after-create sees what was written.
	stored *crypto.WrappedDEK
}

func (d *dekDB) with(sub string) []dekStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []dekStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *dekDB) Connect(context.Context) (driver.Conn, error) { return &dekConn{d: d}, nil }
func (d *dekDB) Driver() driver.Driver                        { return dekOpen{} }

type dekOpen struct{}

func (dekOpen) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type dekConn struct{ d *dekDB }

func (c *dekConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *dekConn) Close() error                        { return nil }
func (c *dekConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *dekConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &dekTx{d: c.d}, nil
}

func (c *dekConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.stmts = append(c.d.stmts, dekStmt{sql: q, args: vals})
	if strings.Contains(q, "audit_log") {
		return driver.RowsAffected(1), nil
	}
	if c.d.affected > 0 {
		switch {
		case strings.HasPrefix(q, "INSERT"):
			c.d.stored = &crypto.WrappedDEK{KEKID: vals[1].(string), Wrapped: vals[2].([]byte)}
		case strings.HasPrefix(q, "UPDATE"):
			c.d.stored = &crypto.WrappedDEK{KEKID: vals[3].(string), Wrapped: vals[4].([]byte)}
		}
	}
	return driver.RowsAffected(c.d.affected), nil
}

func (c *dekConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.stmts = append(c.d.stmts, dekStmt{sql: q, args: vals})
	r := &dekRows{}
	if c.d.stored != nil {
		r.rows = [][]driver.Value{{c.d.stored.KEKID, c.d.stored.Wrapped}}
	}
	return r, nil
}

type dekTx struct{ d *dekDB }

func (t *dekTx) Commit() error   { t.d.mu.Lock(); t.d.committed++; t.d.mu.Unlock(); return nil }
func (t *dekTx) Rollback() error { t.d.mu.Lock(); t.d.rolledBack++; t.d.mu.Unlock(); return nil }

type dekRows struct {
	rows [][]driver.Value
	i    int
}

func (r *dekRows) Columns() []string { return []string{"kek_id", "wrapped"} }
func (r *dekRows) Close() error      { return nil }
func (r *dekRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

var dekCommune = tenant.ID("01JA" + strings.Repeat("A", 22))

func newDEKStore(t *testing.T, d *dekDB) (*DataEncryptionKeyStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewDataEncryptionKeyStore(store.New(db)), tenant.Into(context.Background(), dekCommune)
}

func TestFirstSealCreatesDEKAuditedUnderSystemPrincipal(t *testing.T) {
	d := &dekDB{affected: 1}
	s, ctx := newDEKStore(t, d)
	env, err := crypto.New([]secret.Secret{bytes.Repeat([]byte{9}, crypto.KeyLength)}, s)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := env.Seal(ctx, secret.Secret("fixture"), []byte("t/c/r"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if plain, err := env.Open(ctx, sealed, []byte("t/c/r")); err != nil || string(plain.Lo()) != "fixture" {
		t.Fatalf("round trip through the real store failed: %v", err)
	}
	ins := d.with("INSERT INTO data_encryption_key")
	if len(ins) != 1 || ins[0].args[0] != string(dekCommune) {
		t.Fatalf("inserts = %+v", ins)
	}
	aud := d.with("INSERT INTO audit_log")
	if len(aud) != 1 || aud[0].args[1] != "system" || aud[0].args[4] != ActionCreateDataKey {
		t.Fatalf("audit = %+v", aud)
	}
	delta, _ := aud[0].args[7].([]byte)
	if bytes.Contains(delta, ins[0].args[2].([]byte)) || !strings.Contains(string(delta), "kek_id") {
		t.Errorf("delta = %s — must hold the KEK id and never the wrapped bytes", delta)
	}
	if d.committed != 1 {
		t.Errorf("commits = %d, want the insert and its entry in one", d.committed)
	}
}

func TestCreateDEKConflictReportsExistsAndAuditsNothing(t *testing.T) {
	d := &dekDB{affected: 0}
	s, ctx := newDEKStore(t, d)
	err := s.CreateDEK(ctx, crypto.WrappedDEK{KEKID: "0a0b0c0d", Wrapped: []byte{1}})
	if !errors.Is(err, crypto.ErrDEKExists) {
		t.Fatalf("err = %v, want ErrDEKExists", err)
	}
	if len(d.with("audit_log")) != 0 {
		t.Error("the losing side of a race audited a creation that did not happen")
	}
}

func TestReplaceDEKMissReportsChangedAndRollsBack(t *testing.T) {
	d := &dekDB{affected: 0}
	s, ctx := newDEKStore(t, d)
	err := s.ReplaceDEK(ctx, crypto.WrappedDEK{KEKID: "0a0b0c0d", Wrapped: []byte{1}},
		crypto.WrappedDEK{KEKID: "0e0f1011", Wrapped: []byte{2}})
	if !errors.Is(err, crypto.ErrDEKChanged) {
		t.Fatalf("err = %v, want ErrDEKChanged", err)
	}
	if len(d.with("audit_log")) != 0 || d.rolledBack != 1 {
		t.Errorf("audit rows %d, rollbacks %d", len(d.with("audit_log")), d.rolledBack)
	}
}

func TestReplaceDEKAuditsKEKIDsOnly(t *testing.T) {
	d := &dekDB{affected: 1}
	s, ctx := newDEKStore(t, d)
	if err := s.ReplaceDEK(ctx, crypto.WrappedDEK{KEKID: "0a0b0c0d", Wrapped: []byte("old-bytes")},
		crypto.WrappedDEK{KEKID: "0e0f1011", Wrapped: []byte("new-bytes")}); err != nil {
		t.Fatal(err)
	}
	aud := d.with("INSERT INTO audit_log")
	if len(aud) != 1 || aud[0].args[4] != ActionRewrapDataKey {
		t.Fatalf("audit = %+v", aud)
	}
	delta := string(aud[0].args[7].([]byte))
	if strings.Contains(delta, "bytes") || !strings.Contains(delta, "0a0b0c0d") || !strings.Contains(delta, "0e0f1011") {
		t.Errorf("delta = %s", delta)
	}
}

func TestGetDEKNoRowIsNotFound(t *testing.T) {
	s, ctx := newDEKStore(t, &dekDB{})
	if _, err := s.GetDEK(ctx); !errors.Is(err, crypto.ErrDEKNotFound) {
		t.Fatalf("err = %v", err)
	}
}
