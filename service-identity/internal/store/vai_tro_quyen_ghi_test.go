package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// WHAT THIS FILE PINS WITHOUT A POSTGRESQL: that every statement of the Phân quyền write carries the
// commune as $1 from the transaction (rule 1, invariants 4 and 5), that the reads a guard depends on
// take `FOR UPDATE`, that the removal is bounded on all three key columns, and that the two writes
// bind role / keys / cap_boi to the right placeholders. Whether the locks SERIALISE is a pg-only
// question; no harness here runs one.

// --- a recording driver that accepts []string (the pg driver binds it as text[]) -----------------

type pqLenh struct {
	sql  string
	args []driver.Value
}

type pqGhi struct{ lenh []pqLenh }

type pqKetNoi struct{ g *pqGhi }

func (k pqKetNoi) Connect(context.Context) (driver.Conn, error) { return &pqConn{g: k.g}, nil }
func (k pqKetNoi) Driver() driver.Driver                        { return pqTrinh{} }

type pqTrinh struct{}

func (pqTrinh) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type pqConn struct{ g *pqGhi }

func (c *pqConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *pqConn) Close() error              { return nil }
func (c *pqConn) Begin() (driver.Tx, error) { return gtsTx{}, nil }

// CheckNamedValue lets a []string through untouched, as pgx's stdlib driver does.
func (c *pqConn) CheckNamedValue(*driver.NamedValue) error { return nil }

func (c *pqConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	l := pqLenh{sql: q}
	n := int64(1)
	for _, a := range args {
		l.args = append(l.args, a.Value)
		if ds, ok := a.Value.([]string); ok {
			n = int64(len(ds))
		}
	}
	c.g.lenh = append(c.g.lenh, l)
	return driver.RowsAffected(n), nil
}

func pqChay(t *testing.T, fn func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error) pqLenh {
	t.Helper()
	g := &pqGhi{}
	db := sql.OpenDB(pqKetNoi{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	pdb := pkgstore.New(db)
	ctx := ctxXa(xaMau)
	if err := pdb.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(NewPhanQuyenStore(pdb), tx) }); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	if len(g.lenh) != 1 {
		t.Fatalf("muốn đúng 1 câu lệnh, có %d", len(g.lenh))
	}
	return g.lenh[0]
}

func TestThemCapGanDungThamSo(t *testing.T) {
	l := pqChay(t, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
		return s.ThemCap(context.Background(), tx, "vt-1", []string{"task.read", "document.read"}, "CB-001")
	})
	if len(l.args) != 4 || l.args[0] != string(xaMau) || l.args[1] != "vt-1" || l.args[3] != "CB-001" {
		t.Fatalf("tham số = %v, muốn (xã, vai trò, khoá, mã cán bộ)", l.args)
	}
	if ds, _ := l.args[2].([]string); strings.Join(ds, ",") != "task.read,document.read" {
		t.Errorf("khoá = %v", l.args[2])
	}
	if !strings.Contains(l.sql, "tenant_id, vai_tro_id, quyen_ma, cap_boi") {
		t.Errorf("thứ tự cột không như mong đợi: %s", l.sql)
	}
}

func TestBoCapBoTrenCaBaCotKhoa(t *testing.T) {
	l := pqChay(t, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
		return s.BoCap(context.Background(), tx, "vt-1", []string{"task.read"})
	})
	gon := strings.Join(strings.Fields(l.sql), " ")
	if !strings.Contains(gon, "WHERE tenant_id = $1 AND vai_tro_id = $2 AND quyen_ma = ANY($3)") {
		t.Errorf("câu gỡ không bó trên cả ba cột khoá: %s", gon)
	}
	if len(l.args) != 3 || l.args[0] != string(xaMau) || l.args[1] != "vt-1" {
		t.Errorf("tham số = %v", l.args)
	}
}

// Nothing to add or remove sends nothing.
func TestThemBoRongKhongGuiCauNao(t *testing.T) {
	g := &pqGhi{}
	db := sql.OpenDB(pqKetNoi{g: g})
	t.Cleanup(func() { db.Close() })
	pdb := pkgstore.New(db)
	ctx := ctxXa(xaMau)
	_ = pdb.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		s := NewPhanQuyenStore(pdb)
		if err := s.ThemCap(ctx, tx, "vt-1", nil, "CB-001"); err != nil {
			return err
		}
		return s.BoCap(ctx, tx, "vt-1", nil)
	})
	if len(g.lenh) != 0 {
		t.Errorf("tập rỗng mà vẫn gửi %d câu", len(g.lenh))
	}
}

// Every locked read and every tenant-owned statement binds the commune as $1, and the guard reads
// take FOR UPDATE.
func TestCauPhanQuyenMangXaVaKhoaDong(t *testing.T) {
	for ten, q := range map[string]string{
		"admin.user": truyVanQuanTriDeGhi,
		"admin.role": truyVanPhanQuyenDeGhi,
	} {
		gon := strings.Join(strings.Fields(q), " ")
		for _, can := range []string{
			"nd.tenant_id = $1", "vq.quyen_ma = '" + ten + "'", "ORDER BY nd.id FOR UPDATE",
			"nd.co_tai_khoan", "nd.dang_hoat_dong", "vt.deleted_at IS NULL",
		} {
			if !strings.Contains(gon, can) {
				t.Errorf("%s: thiếu %q trong %s", ten, can, gon)
			}
		}
	}
	if !strings.Contains(truyVanKhoaTonTai, "$1::text <> ''") {
		t.Error("truy vấn danh mục quyền phải tiêu thụ $1 như quyen.go")
	}
}
