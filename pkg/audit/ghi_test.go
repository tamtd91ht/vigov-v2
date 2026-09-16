package audit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/store"
	"github.com/vihat/vigov/pkg/tenant"
)

// audit_test.go covers validate() only — the pure function. Write() itself, the part that
// actually reaches the database, had no test: not the statement it emits, not its refusal to
// file one commune's action inside another commune's transaction.
//
// THE DEFECT CLASS: an entry filed under the WRONG commune, or an entry silently not written.
// Both are invisible until an inspection asks who did something, and by then the answer cannot
// be reconstructed. The second is worse in a shared-infrastructure system: an entry attributed
// to commune B for an act in commune A is a record in another authority's archive.

var (
	xaA = tenant.ID("01J0000000000000000000000A")
	xaB = tenant.ID("01J0000000000000000000000B")
)

// --- fake driver (Exec only: audit writes, it never reads) --------------------------------------

type lenhGhi struct {
	sql  string
	args []driver.Value
}

type ghiChep struct {
	mu   sync.Mutex
	lenh []lenhGhi
}

func (g *ghiChep) all() []lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]lenhGhi(nil), g.lenh...)
}

type ketNoiGia struct{ g *ghiChep }

func (c ketNoiGia) Connect(context.Context) (driver.Conn, error) { return &connGia{g: c.g}, nil }
func (c ketNoiGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connGia struct{ g *ghiChep }

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error              { return nil }
func (c *connGia) Begin() (driver.Tx, error) { return txGia{}, nil }
func (c *connGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return txGia{}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	c.g.mu.Lock()
	c.g.lenh = append(c.g.lenh, lenhGhi{sql: q, args: v})
	c.g.mu.Unlock()
	return driver.RowsAffected(1), nil
}

type txGia struct{}

func (txGia) Commit() error   { return nil }
func (txGia) Rollback() error { return nil }

// trongGiaoDich runs fn inside a real *store.ScopedTx bound to xa, which is the only way to
// obtain one — by design: audit.Write takes a transaction so that auditing outside one cannot
// be expressed at all.
func trongGiaoDich(t *testing.T, xa tenant.ID, fn func(ctx context.Context, tx *store.ScopedTx) error) (*ghiChep, error) {
	t.Helper()
	g := &ghiChep{}
	db := sql.OpenDB(ketNoiGia{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	ctx := tenant.Into(context.Background(), xa)
	err := store.New(db).For(ctx).Tx(ctx, func(tx *store.ScopedTx) error { return fn(ctx, tx) })
	return g, err
}

func entryMau() Entry {
	return Entry{
		Actor:   Actor{ID: "CB-001", Kind: "staff", IP: "10.0.0.7"},
		Action:  "dang_nhap",
		Subject: "CB-001",
	}
}

// --- tests ---------------------------------------------------------------------------------------

func TestWriteGhiDuTamCotVaLayXaTuGiaoDich(t *testing.T) {
	// Rule 6, invariant 2: who · what · on which record · when · from which IP · in which
	// commune. The commune is taken from the transaction, never from the caller's optimism.
	g, err := trongGiaoDich(t, xaA, func(ctx context.Context, tx *store.ScopedTx) error {
		return Write(ctx, tx, entryMau())
	})
	if err != nil {
		t.Fatalf("Write lỗi: %v", err)
	}

	lenh := g.all()
	if len(lenh) != 1 {
		t.Fatalf("có %d câu lệnh, muốn 1", len(lenh))
	}
	if !strings.Contains(lenh[0].sql, "INSERT INTO audit_log") {
		t.Fatalf("câu lệnh không phải ghi vết: %q", lenh[0].sql)
	}
	args := lenh[0].args
	if len(args) != 8 {
		t.Fatalf("có %d tham số, muốn 8: %v", len(args), args)
	}
	muon := []any{string(xaA), "CB-001", "staff", "10.0.0.7", "dang_nhap", "CB-001"}
	ten := []string{"tenant_id", "actor_id", "actor_kind", "actor_ip", "action", "subject"}
	for i, v := range muon {
		if args[i] != v {
			t.Errorf("%s = %v, muốn %v", ten[i], args[i], v)
		}
	}
	if at, ok := args[6].(time.Time); !ok || at.IsZero() {
		t.Errorf("at = %v, phải là thời điểm ghi", args[6])
	}
}

func TestWriteTuChoiGhiVetCuaXaKhacVaoGiaoDichNay(t *testing.T) {
	// Rule 1: filing commune B's action inside commune A's transaction is either a bug or an
	// attack, and the entry would end up in the wrong authority's archive with nothing to show
	// it was misfiled. It must stop before the statement runs.
	e := entryMau()
	e.TenantID = xaB

	g, err := trongGiaoDich(t, xaA, func(ctx context.Context, tx *store.ScopedTx) error {
		return Write(ctx, tx, e)
	})
	if err == nil {
		t.Fatal("ghi vết của xã khác phải bị từ chối")
	}
	if n := len(g.all()); n != 0 {
		t.Fatalf("đã chạy %d câu lệnh dù xã không khớp — vết sai xã đã nằm trong CSDL", n)
	}
}

func TestWriteThieuTruongThiKhongChayCauLenhNao(t *testing.T) {
	// An entry nobody can trace is worse than none: it takes up the place of the real one. The
	// refusal has to happen before the INSERT, not as a nullable column.
	cases := map[string]func(*Entry){
		"thiếu người thực hiện": func(e *Entry) { e.Actor.ID = "" },
		"thiếu hành động":       func(e *Entry) { e.Action = "" },
		"thiếu đối tượng":       func(e *Entry) { e.Subject = "" },
	}
	for ten, sua := range cases {
		t.Run(ten, func(t *testing.T) {
			e := entryMau()
			sua(&e)
			g, err := trongGiaoDich(t, xaA, func(ctx context.Context, tx *store.ScopedTx) error {
				return Write(ctx, tx, e)
			})
			if err == nil {
				t.Fatal("phải bị từ chối")
			}
			if n := len(g.all()); n != 0 {
				t.Fatalf("đã chạy %d câu lệnh cho một vết không hợp lệ", n)
			}
		})
	}
}

func TestWriteHongThiGiaoDichBaoLoi(t *testing.T) {
	// The caller must be able to fail its own transaction when the entry cannot be written —
	// that is the whole mechanism behind rule 6, invariant 3.
	g, err := trongGiaoDich(t, xaA, func(ctx context.Context, tx *store.ScopedTx) error {
		e := entryMau()
		e.TenantID = xaB
		if err := Write(ctx, tx, e); err != nil {
			return err // exactly what a use case does
		}
		return nil
	})
	if err == nil {
		t.Fatal("lỗi ghi vết phải nổi lên tới người gọi để giao dịch bị huỷ")
	}
	if len(g.all()) != 0 {
		t.Fatal("không được có câu lệnh nào")
	}
}
