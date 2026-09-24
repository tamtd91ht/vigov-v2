package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// WHAT THIS FILE ADDS to vai_tro_quyen_ghi_test.go, and why it is a separate driver:
//
//  1. THE ROW-COUNT GUARD ON THE HARD DELETE (dungSoDong). ADR 0040 made grant removal a hard DELETE.
//     The string test pins its WHERE clause; this pins the RUNTIME defence behind it — a DELETE that
//     removed more rows than the diff named (a widened predicate, a whole-column wipe) must abort the
//     transaction instead of committing an audit entry that lists fewer removals than happened. The
//     recording driver in the sibling file always answers "exactly len(keys) rows", so it can never
//     make that guard fire.
//  2. THE LOCK READ OF THE TARGET ROLE. `deleted_at IS NULL` on it is what makes a soft-deleted role
//     404 instead of a column that can be re-granted; nothing pinned that statement. (Dropping the
//     commune from it while still binding $1 fails in PostgreSQL on an untyped parameter, so the
//     commune half is self-detecting; the soft-delete half is not.)

type pqbLenh struct {
	sql  string
	args []driver.Value
}

type pqbGhi struct {
	lenh []pqbLenh
	// soDong is what every Exec reports as RowsAffected. The point of this driver.
	soDong int64
	// coVaiTro: the vai_tro lock read finds a row.
	coVaiTro bool
}

type pqbKetNoi struct{ g *pqbGhi }

func (k pqbKetNoi) Connect(context.Context) (driver.Conn, error) { return &pqbConn{g: k.g}, nil }
func (k pqbKetNoi) Driver() driver.Driver                        { return pqTrinh{} }

type pqbConn struct{ g *pqbGhi }

func (c *pqbConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *pqbConn) Close() error                             { return nil }
func (c *pqbConn) Begin() (driver.Tx, error)                { return gtsTx{}, nil }
func (c *pqbConn) CheckNamedValue(*driver.NamedValue) error { return nil }

func (c *pqbConn) ghi(q string, args []driver.NamedValue) {
	l := pqbLenh{sql: q}
	for _, a := range args {
		l.args = append(l.args, a.Value)
	}
	c.g.lenh = append(c.g.lenh, l)
}

func (c *pqbConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.ghi(q, args)
	return driver.RowsAffected(c.g.soDong), nil
}

func (c *pqbConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.ghi(q, args)
	switch {
	case strings.Contains(q, "FROM vai_tro\n") || strings.Contains(q, "FROM vai_tro "):
		if c.g.coVaiTro {
			return &pqbRows{cot: []string{"id"}, hang: [][]driver.Value{{"vt-1"}}}, nil
		}
		return &pqbRows{cot: []string{"id"}}, nil
	default:
		return &pqbRows{cot: []string{"quyen_ma"}, hang: [][]driver.Value{{"task.read"}}}, nil
	}
}

type pqbRows struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *pqbRows) Columns() []string { return r.cot }
func (r *pqbRows) Close() error      { return nil }
func (r *pqbRows) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func pqbChay(t *testing.T, g *pqbGhi, fn func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	db := sql.OpenDB(pqbKetNoi{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	pdb := pkgstore.New(db)
	ctx := ctxXa(xaMau)
	return pdb.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(NewPhanQuyenStore(pdb), tx) })
}

// A DELETE THAT TOUCHED A DIFFERENT NUMBER OF ROWS THAN THE DIFF NAMED ABORTS THE SAVE.
//
// More rows is the dangerous direction: the predicate was wider than (tenant, role, keys) — another
// role's cells, another commune's, or the whole column — and committing would leave an audit entry
// whose `bo` lists fewer keys than were destroyed. Fewer rows means the diff and the table disagree.
//
// MUTATION THAT MUST TURN THIS RED: in BoCap, replace `return dungSoDong(...)` with `return nil`.
func TestBoCapSoDongLechThiTuChoi(t *testing.T) {
	for _, soDong := range []int64{3, 0} {
		g := &pqbGhi{soDong: soDong}
		err := pqbChay(t, g, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
			return s.BoCap(context.Background(), tx, "vt-1", []string{"task.read"})
		})
		if err == nil {
			t.Errorf("DELETE báo %d dòng cho 1 ô mà BoCap vẫn thành công — vết sẽ ghi sai số ô bị xoá cứng", soDong)
		}
		if len(g.lenh) != 1 {
			t.Fatalf("muốn đúng 1 câu DELETE, có %d", len(g.lenh))
		}
		// The keys bound at $3 are EXACTLY the removed set — not the before set, not the after set.
		if ds, _ := g.lenh[0].args[2].([]string); strings.Join(ds, ",") != "task.read" {
			t.Errorf("$3 của câu gỡ = %v, muốn đúng tập bị gỡ", g.lenh[0].args[2])
		}
	}
}

// The same guard on the insert half: a grant that landed a different number of cells than asked.
//
// MUTATION THAT MUST TURN THIS RED: in ThemCap, replace `return dungSoDong(...)` with `return nil`.
func TestThemCapSoDongLechThiTuChoi(t *testing.T) {
	g := &pqbGhi{soDong: 1}
	err := pqbChay(t, g, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
		return s.ThemCap(context.Background(), tx, "vt-1", []string{"task.read", "document.read"}, "CB-001")
	})
	if err == nil {
		t.Error("INSERT báo 1 dòng cho 2 ô mà ThemCap vẫn thành công")
	}
}

// THE TARGET ROLE IS READ LIVE, IN THIS COMMUNE, LOCKED — then its cells, in this commune, locked.
//
// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from stmtVaiTro. A soft-deleted
// role would then be a column this route re-grants, where the contract says 404.
func TestVaiTroDeGhiDocDongSongCuaXaVaKhoa(t *testing.T) {
	g := &pqbGhi{coVaiTro: true}
	var ds []string
	err := pqbChay(t, g, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
		var err error
		ds, err = s.VaiTroDeGhi(context.Background(), tx, "vt-1")
		return err
	})
	if err != nil {
		t.Fatalf("VaiTroDeGhi: %v", err)
	}
	if strings.Join(ds, ",") != "task.read" {
		t.Errorf("các ô = %v", ds)
	}
	if len(g.lenh) != 2 {
		t.Fatalf("muốn 2 câu (dòng vai trò, các ô), có %d", len(g.lenh))
	}
	vt := strings.Join(strings.Fields(g.lenh[0].sql), " ")
	if !strings.Contains(vt, "WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE") {
		t.Errorf("câu khoá vai trò không bó theo xã + dòng sống + FOR UPDATE: %s", vt)
	}
	cap := strings.Join(strings.Fields(g.lenh[1].sql), " ")
	if !strings.Contains(cap, "WHERE tenant_id = $1 AND vai_tro_id = $2") || !strings.HasSuffix(cap, "FOR UPDATE") {
		t.Errorf("câu đọc ô không bó theo xã + vai trò, hoặc không khoá: %s", cap)
	}
	for i, l := range g.lenh {
		if len(l.args) < 2 || l.args[0] != string(xaMau) || l.args[1] != "vt-1" {
			t.Errorf("câu %d: tham số = %v, muốn (xã của ngữ cảnh, vt-1)", i, l.args)
		}
	}
}

// Absent (which is also soft-deleted and another commune's) is ONE sentinel, and no cell is read.
func TestVaiTroDeGhiKhongCoLaMotLoi(t *testing.T) {
	g := &pqbGhi{coVaiTro: false}
	err := pqbChay(t, g, func(s *PhanQuyenStore, tx *pkgstore.ScopedTx) error {
		_, err := s.VaiTroDeGhi(context.Background(), tx, "vt-1")
		return err
	})
	if !errors.Is(err, ErrVaiTroKhongTonTaiDeGhi) {
		t.Fatalf("err = %v, muốn ErrVaiTroKhongTonTaiDeGhi", err)
	}
	if len(g.lenh) != 1 {
		t.Errorf("vai trò không có mà vẫn đọc tiếp %d câu", len(g.lenh)-1)
	}
}
