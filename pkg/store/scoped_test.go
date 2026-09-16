package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/pkg/tenant"
)

// pkg/store had no test at all, and it is the package that decides whether rule 1 holds: every
// query in every service goes through it.
//
// THE DEFECT CLASS THESE COVER: a statement that reaches PostgreSQL WITHOUT the commune bound
// to $1, or with the commune bound to the wrong placeholder. Nothing turns red when that
// happens — every test of a single commune still passes, because one commune's data is all
// there is in a test database. It shows up the day a second commune is onboarded, as one
// commune reading another's records: a breach between two public authorities.
//
// Everything below runs on a fake database/sql driver. No PostgreSQL, so it keeps being run.
// The real SQL is covered separately by services/identity/internal/store/*_pg_test.go, which
// skip themselves when VIGOV_TEST_DSN is empty.

var (
	xaA = tenant.ID("01J0000000000000000000000A")
	xaB = tenant.ID("01J0000000000000000000000B")
)

// --- fake driver -------------------------------------------------------------------------

type lenhGhi struct {
	sql  string
	args []driver.Value
	tx   int // 0 = outside any transaction
}

// ghiChep records what actually reached the driver, and how each transaction ended.
type ghiChep struct {
	mu      sync.Mutex
	lenh    []lenhGhi
	soTx    int
	ketThuc map[int]string // tx id -> "commit" | "rollback"
	loiTheo map[string]error
	loiTx   error // returned by BeginTx
}

func moGhiChep() (*sql.DB, *ghiChep) {
	g := &ghiChep{ketThuc: map[int]string{}, loiTheo: map[string]error{}}
	db := sql.OpenDB(ketNoiGia{g: g})
	db.SetMaxOpenConns(1) // one connection keeps the recorded order deterministic
	return db, g
}

func (g *ghiChep) ghi(l lenhGhi) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lenh = append(g.lenh, l)
	for manh, err := range g.loiTheo {
		if strings.Contains(l.sql, manh) {
			return err
		}
	}
	return nil
}

func (g *ghiChep) chua(manh string) *lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.lenh {
		if strings.Contains(g.lenh[i].sql, manh) {
			return &g.lenh[i]
		}
	}
	return nil
}

func (g *ghiChep) ketThucCua(tx int) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ketThuc[tx]
}

type ketNoiGia struct{ g *ghiChep }

func (c ketNoiGia) Connect(context.Context) (driver.Conn, error) { return &connGia{g: c.g}, nil }
func (c ketNoiGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connGia struct {
	g  *ghiChep
	tx int
}

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error { return nil }
func (c *connGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.g.loiTx != nil {
		return nil, c.g.loiTx
	}
	c.g.mu.Lock()
	c.g.soTx++
	c.tx = c.g.soTx
	id := c.tx
	c.g.mu.Unlock()
	return &txGia{c: c, id: id}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.g.ghi(lenhGhi{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.g.ghi(lenhGhi{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	return &rowsGia{cot: []string{"id"}, hang: [][]driver.Value{{"nd-001"}}}, nil
}

func giaTri(args []driver.NamedValue) []driver.Value {
	ra := make([]driver.Value, 0, len(args))
	for _, a := range args {
		ra = append(ra, a.Value)
	}
	return ra
}

type txGia struct {
	c  *connGia
	id int
}

func (t *txGia) Commit() error   { return t.dong("commit") }
func (t *txGia) Rollback() error { return t.dong("rollback") }

func (t *txGia) dong(sao string) error {
	t.c.g.mu.Lock()
	t.c.g.ketThuc[t.id] = sao
	t.c.g.mu.Unlock()
	t.c.tx = 0
	return nil
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

// --- the commune on every statement --------------------------------------------------------

func TestForKhongCoXaThiPanic(t *testing.T) {
	// Fail closed. A repository built with no commune would read every commune's rows; panicking
	// is the loud failure the alternative does not give.
	db, _ := moGhiChep()
	defer db.Close()

	defer func() {
		if recover() == nil {
			t.Fatal("For không có xã trong context phải panic, không được trả về kho không giới hạn")
		}
	}()
	New(db).For(context.Background())
}

func TestQueryLuonRangBuocXaVaoThamSoDau(t *testing.T) {
	// The whole contract of the package in one assertion: the statement carries
	// `WHERE tenant_id = $1`, $1 is the commune from the CONTEXT, and the caller's own
	// placeholders start at $2. Swap the order and every query silently reads another commune.
	db, g := moGhiChep()
	defer db.Close()

	ctx := tenant.Into(context.Background(), xaA)
	rows, err := New(db).For(ctx).Query(ctx, "id, ma", "nguoi_dung",
		"AND email = $2 AND deleted_at IS NULL", "canbo@example.gov.vn")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	l := g.chua("FROM nguoi_dung")
	if l == nil {
		t.Fatal("không có câu lệnh nào chạm tới driver")
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Fatalf("câu lệnh thiếu ràng buộc xã: %q", l.sql)
	}
	if len(l.args) != 2 {
		t.Fatalf("có %d tham số, muốn 2: %v", len(l.args), l.args)
	}
	if l.args[0] != string(xaA) {
		t.Errorf("$1 = %v, muốn mã xã %q — $1 phải LUÔN là xã", l.args[0], xaA)
	}
	if l.args[1] != "canbo@example.gov.vn" {
		t.Errorf("$2 = %v, muốn tham số của người gọi", l.args[1])
	}
}

func TestQueryJoinVanRangBuocXaVaoThamSoDau(t *testing.T) {
	// QueryJoin hands the statement to the caller, so the only thing the package can still
	// guarantee is that $1 is the commune and that the caller cannot occupy it.
	db, g := moGhiChep()
	defer db.Close()

	ctx := tenant.Into(context.Background(), xaB)
	rows, err := New(db).For(ctx).QueryJoin(ctx,
		`SELECT nd.id FROM nguoi_dung nd
		   JOIN vai_tro vt ON vt.id = nd.vai_tro_id AND vt.tenant_id = $1
		  WHERE nd.tenant_id = $1 AND nd.id = $2`, "nd-001")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	l := g.chua("JOIN vai_tro")
	if l == nil {
		t.Fatal("câu lệnh join không chạm tới driver")
	}
	if l.args[0] != string(xaB) {
		t.Fatalf("$1 = %v, muốn mã xã %q", l.args[0], xaB)
	}
	if l.args[1] != "nd-001" {
		t.Fatalf("$2 = %v, muốn tham số của người gọi", l.args[1])
	}
}

func TestHaiXaKhongDungChungPhamVi(t *testing.T) {
	// Two requests, two communes, one process. The commune travels on the context and nowhere
	// else, so a Scoped built for commune A must never carry commune B's id.
	db, g := moGhiChep()
	defer db.Close()
	d := New(db)

	for _, xa := range []tenant.ID{xaA, xaB, xaA} {
		ctx := tenant.Into(context.Background(), xa)
		sc := d.For(ctx)
		if sc.TenantID() != xa {
			t.Fatalf("Scoped.TenantID = %q, muốn %q", sc.TenantID(), xa)
		}
		rows, err := sc.Query(ctx, "id", "phien", "AND id = $2", "sid-"+string(xa))
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.lenh) != 3 {
		t.Fatalf("ghi nhận %d câu lệnh, muốn 3", len(g.lenh))
	}
	muon := []tenant.ID{xaA, xaB, xaA}
	for i, l := range g.lenh {
		if l.args[0] != string(muon[i]) {
			t.Errorf("câu lệnh %d chạy với xã %v, muốn %q", i, l.args[0], muon[i])
		}
	}
}

// --- transaction boundary -------------------------------------------------------------------

func TestTxThanhCongThiCommitMotLan(t *testing.T) {
	db, g := moGhiChep()
	defer db.Close()

	ctx := tenant.Into(context.Background(), xaA)
	err := New(db).For(ctx).Tx(ctx, func(tx *ScopedTx) error {
		if tx.TenantID() != xaA {
			t.Errorf("ScopedTx.TenantID = %q, muốn %q", tx.TenantID(), xaA)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO phien (tenant_id, id) VALUES ($1,$2)",
			string(tx.TenantID()), "sid-001"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "INSERT INTO audit_log (tenant_id) VALUES ($1)", string(tx.TenantID()))
		return err
	})
	if err != nil {
		t.Fatalf("Tx lỗi: %v", err)
	}

	phien, vet := g.chua("INSERT INTO phien"), g.chua("INSERT INTO audit_log")
	if phien == nil || vet == nil {
		t.Fatal("thiếu câu lệnh trong giao dịch")
	}
	if phien.tx == 0 || phien.tx != vet.tx {
		t.Fatalf("hai câu lệnh nằm ở giao dịch %d và %d — phải cùng một giao dịch", phien.tx, vet.tx)
	}
	if got := g.ketThucCua(phien.tx); got != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", got)
	}
}

func TestTxLoiThiRollbackVaKhongGiuLaiGiCa(t *testing.T) {
	// Rule 6, invariant 3 stands on this: if the audit entry fails, the business write must go
	// with it. A Tx that swallowed the error and committed anyway would leave a change nobody
	// can account for — and it is the audit entry, the cheap-looking one, that gets dropped.
	db, g := moGhiChep()
	defer db.Close()
	g.loiTheo["audit_log"] = errors.New("ghi vết hỏng")

	ctx := tenant.Into(context.Background(), xaA)
	err := New(db).For(ctx).Tx(ctx, func(tx *ScopedTx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO phien (tenant_id) VALUES ($1)", string(xaA)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "INSERT INTO audit_log (tenant_id) VALUES ($1)", string(xaA))
		return err
	})
	if err == nil {
		t.Fatal("Tx phải trả lỗi khi câu lệnh bên trong hỏng")
	}

	phien := g.chua("INSERT INTO phien")
	if phien == nil {
		t.Fatal("câu lệnh nghiệp vụ không chạy")
	}
	if got := g.ketThucCua(phien.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback — thay đổi nghiệp vụ phải bị huỷ cùng ghi vết", got)
	}
}

func TestTxKhongMoDuocThiBaoLoiChuKhongChayTiep(t *testing.T) {
	db, g := moGhiChep()
	defer db.Close()
	g.loiTx = errors.New("hết kết nối")

	ctx := tenant.Into(context.Background(), xaA)
	chay := false
	err := New(db).For(ctx).Tx(ctx, func(*ScopedTx) error {
		chay = true
		return nil
	})
	if err == nil {
		t.Fatal("mở giao dịch hỏng mà Tx vẫn báo thành công")
	}
	if chay {
		t.Fatal("thân giao dịch vẫn chạy dù không mở được giao dịch — sẽ ghi ngoài giao dịch")
	}
}
