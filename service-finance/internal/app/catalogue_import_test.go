package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The catalogue import OVER THE REAL STORES AND A REAL TRANSACTION BOUNDARY, on a fake database/sql
// driver that records which statement ran in which transaction and how each transaction ended. The row
// rules are the planner's (domain/catalogue_import_test.go); what is proved here is what the
// transaction owns: lock → snapshot → inserts + one entry per row, all in ONE transaction, and a
// refusal or a late failure that leaves nothing committed.

var importCommune = tenant.ID("01JI" + strings.Repeat("I", 22))

const importStaffCode = "CB-00777"

type importStmt struct {
	sql  string
	args []driver.Value
	tx   int
}

type importRecorder struct {
	mu       sync.Mutex
	stmts    []importStmt
	txCount  int
	ends     map[int]string // tx id -> "commit" | "rollback"
	failOn   map[string]error
	snapshot [][]driver.Value // rows for the snapshot SELECT: ma, nhan, deleted
}

func openImportDB(t *testing.T) (*store.DB, *importRecorder) {
	t.Helper()
	g := &importRecorder{ends: map[int]string{}, failOn: map[string]error{}}
	db := sql.OpenDB(importConnector{g: g})
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return store.New(db), g
}

func (g *importRecorder) record(s importStmt) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stmts = append(g.stmts, s)
	for frag, err := range g.failOn {
		if strings.Contains(s.sql, frag) {
			return err
		}
	}
	return nil
}

func (g *importRecorder) all(frag string) []importStmt {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []importStmt
	for _, s := range g.stmts {
		if strings.Contains(s.sql, frag) {
			out = append(out, s)
		}
	}
	return out
}

func (g *importRecorder) end(tx int) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ends[tx]
}

type importConnector struct{ g *importRecorder }

func (c importConnector) Connect(context.Context) (driver.Conn, error) {
	return &importConn{g: c.g}, nil
}
func (c importConnector) Driver() driver.Driver { return importDriver{} }

type importDriver struct{}

func (importDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: connector only")
}

type importConn struct {
	g  *importRecorder
	tx int
}

func (c *importConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *importConn) Close() error { return nil }
func (c *importConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *importConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.g.mu.Lock()
	c.g.txCount++
	c.tx = c.g.txCount
	c.g.mu.Unlock()
	return &importTx{c: c, id: c.tx}, nil
}

func namedValues(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, 0, len(args))
	for _, a := range args {
		out = append(out, a.Value)
	}
	return out
}

func (c *importConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.g.record(importStmt{sql: q, args: namedValues(args), tx: c.tx}); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *importConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.g.record(importStmt{sql: q, args: namedValues(args), tx: c.tx}); err != nil {
		return nil, err
	}
	if strings.Contains(q, "deleted_at IS NOT NULL") {
		c.g.mu.Lock()
		rows := append([][]driver.Value(nil), c.g.snapshot...)
		c.g.mu.Unlock()
		return &importRows{rows: rows}, nil
	}
	return &importRows{}, nil
}

type importTx struct {
	c  *importConn
	id int
}

func (t *importTx) finish(how string) error {
	t.c.g.mu.Lock()
	t.c.g.ends[t.id] = how
	t.c.g.mu.Unlock()
	t.c.tx = 0
	return nil
}
func (t *importTx) Commit() error   { return t.finish("commit") }
func (t *importTx) Rollback() error { return t.finish("rollback") }

type importRows struct {
	rows [][]driver.Value
	i    int
}

func (r *importRows) Columns() []string { return []string{"ma", "nhan", "deleted"} }
func (r *importRows) Close() error      { return nil }
func (r *importRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func importActor() audit.Actor {
	return audit.Actor{ID: importStaffCode, Kind: "staff", IP: "10.0.0.7"}
}

func importCtx() context.Context { return tenant.Into(context.Background(), importCommune) }

func pinImportIDs(uc *CapitalPlanCategoryImporter) {
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("01JIMPORT%017d", n), nil
	}
}

func categoryImportRows() []domain.CatalogueImportRow {
	return []domain.CatalogueImportRow{{Row: 2, Label: "Xây dựng mới", Order: "3"}, {Row: 3, Label: "Sửa chữa", Code: "sua-chua"}}
}

func newRealImporter(db *store.DB) *CapitalPlanCategoryImporter {
	uc := NewCapitalPlanCategoryImporter(db, fistore.NewHangMucKeHoachVonStore(db))
	pinImportIDs(uc)
	return uc
}

// auditEntries decodes every audit INSERT: args are (tenant, actor, kind, ip, action, subject, at, delta).
func auditEntries(t *testing.T, g *importRecorder) ([]importStmt, []map[string]any) {
	t.Helper()
	stmts := g.all("INSERT INTO audit_log")
	deltas := make([]map[string]any, 0, len(stmts))
	for _, s := range stmts {
		var d map[string]any
		b, _ := s.args[7].([]byte)
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("delta %s: %v", b, err)
		}
		deltas = append(deltas, d)
	}
	return stmts, deltas
}

func TestCatalogueImport_CommitsRowsAndOneEntryPerRowInOneTransaction(t *testing.T) {
	db, g := openImportDB(t)
	res, err := newRealImporter(db).Import(importCtx(), categoryImportRows(), importActor())
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{"xay-dung-moi", "sua-chua"}
	if len(res.Entries) != 2 || res.Entries[0].Code != codes[0] || res.Entries[0].ID == "" || res.Batch == "" {
		t.Fatalf("kết quả %+v", res)
	}
	if g.txCount != 1 || g.end(1) != "commit" {
		t.Fatalf("%d giao dịch, kết thúc %q — muốn MỘT giao dịch commit", g.txCount, g.end(1))
	}
	if lock := g.all("pg_advisory_xact_lock"); len(lock) != 1 || lock[0].tx != 1 {
		t.Errorf("phải khoá lượt nhập trong giao dịch: %+v", lock)
	}
	inserts := g.all("INSERT INTO hang_muc_ke_hoach_von")
	if len(inserts) != 2 {
		t.Fatalf("%d câu chèn, muốn 2", len(inserts))
	}
	for i, ins := range inserts {
		if ins.tx != 1 || !strings.Contains(ins.sql, "'don-vi', false") {
			t.Errorf("câu chèn phải là câu của biểu mẫu, nguon là hằng: %s", ins.sql)
		}
		// (tenant, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung): never the default, always in use.
		if ins.args[2] != codes[i] || ins.args[5] != false || ins.args[6] != true {
			t.Errorf("dòng nhập %d: %v", i, ins.args)
		}
	}
	if inserts[0].args[4] != int64(3) {
		t.Errorf("thu_tu của dòng 2 = %v, muốn 3", inserts[0].args[4])
	}
	stmts, deltas := auditEntries(t, g)
	if len(stmts) != 2 {
		t.Fatalf("%d vết, muốn một vết cho mỗi dòng", len(stmts))
	}
	for i, s := range stmts {
		if s.tx != 1 || s.args[1] != importStaffCode || s.args[4] != HanhViThemHangMuc || s.args[5] != codes[i] {
			t.Errorf("vết %d: tx=%d actor=%v action=%v subject=%v", i, s.tx, s.args[1], s.args[4], s.args[5])
		}
		d := deltas[i]
		if d["nguon"] != catalogueImportSource || d["lo_nhap"] != res.Batch || d["dong"] != float64(i+2) || d["so_dong"] != float64(2) {
			t.Errorf("delta %v", d)
		}
		if sau, _ := d["sau"].(map[string]any); sau["ma"] != codes[i] || sau["nguon"] != domain.NguonDonVi {
			t.Errorf("delta.sau %v", d["sau"])
		}
	}
	for _, s := range g.stmts {
		if len(s.args) == 0 || s.args[0] != string(importCommune) {
			t.Errorf("câu lệnh không mang xã của ngữ cảnh ở $1: %q %v", s.sql, s.args)
		}
	}
}

func TestCatalogueImport_PreviewWritesNothing(t *testing.T) {
	db, g := openImportDB(t)
	res, err := newRealImporter(db).Preview(importCtx(), categoryImportRows())
	if err != nil || len(res.Entries) != 2 || len(res.Errors) != 0 {
		t.Fatalf("xem trước: %+v %v", res, err)
	}
	if len(g.all("INSERT")) != 0 || len(g.all("pg_advisory_xact_lock")) != 0 || g.end(1) != "rollback" {
		t.Errorf("xem trước phải không khoá, không ghi, và luôn rollback: %q", g.end(1))
	}
}

// A DUPLICATE AGAINST THE CATALOGUE — the derived code of a soft-deleted row — refuses the WHOLE file.
func TestCatalogueImport_OneDuplicateRowWritesNothing(t *testing.T) {
	db, g := openImportDB(t)
	g.snapshot = [][]driver.Value{{"xay-dung-moi", "Xây dựng mới (cũ)", true}}
	_, err := newRealImporter(db).Import(importCtx(), categoryImportRows(), importActor())
	var rej *CatalogueImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 2 {
		t.Fatalf("lỗi = %v, muốn đúng một lỗi dòng 2 (mã của dòng đã xoá)", err)
	}
	if len(g.all("INSERT")) != 0 || g.end(1) != "rollback" {
		t.Errorf("tệp có lỗi mà vẫn ghi, kết thúc %q", g.end(1))
	}
}

func TestCatalogueImport_LateAuditFailureRollsBackEverything(t *testing.T) {
	db, g := openImportDB(t)
	g.failOn["INSERT INTO audit_log"] = errors.New("ổ đĩa đầy")
	if _, err := newRealImporter(db).Import(importCtx(), categoryImportRows(), importActor()); err == nil {
		t.Fatal("vết lỗi mà nhập vẫn thành công")
	}
	if g.end(1) != "rollback" {
		t.Errorf("vết lỗi phải huỷ cả tệp: %q", g.end(1))
	}
}

// A code a concurrent FORM took after the snapshot: the unique key refuses it and the file rolls back.
func TestCatalogueImport_UniqueKeyRaceIsCodeTakenAndRollsBack(t *testing.T) {
	db, g := openImportDB(t)
	g.failOn["INSERT INTO hang_muc_ke_hoach_von"] = errors.New(`ERROR: duplicate key value violates unique constraint "hang_muc_ke_hoach_von_p03_tenant_id_ma_key" (SQLSTATE 23505)`)
	_, err := newRealImporter(db).Import(importCtx(), categoryImportRows(), importActor())
	if !errors.Is(err, fistore.ErrMaDaTonTai) || g.end(1) != "rollback" {
		t.Errorf("lỗi = %v, kết thúc %q", err, g.end(1))
	}
}

func TestCatalogueImport_RefusesActorWithoutStaffCode(t *testing.T) {
	db, g := openImportDB(t)
	if _, err := newRealImporter(db).Import(importCtx(), categoryImportRows(), audit.Actor{Kind: "staff"}); !errors.Is(err, ErrCatalogueImportNoActor) {
		t.Fatalf("thiếu mã cán bộ mà vẫn nhập: %v", err)
	}
	if g.txCount != 0 {
		t.Error("từ chối người thực hiện phải xảy ra trước khi mở giao dịch")
	}
}
