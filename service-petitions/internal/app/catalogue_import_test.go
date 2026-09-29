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
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
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
	snapshot [][]driver.Value // rows for the snapshot SELECT: ma, nhan, thu_tu, deleted
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

func (r *importRows) Columns() []string { return []string{"ma", "nhan", "thu_tu", "deleted"} }
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

func pinImportIDs[T any](uc *CatalogueImporter[T]) {
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("01JIMPORT%017d", n), nil
	}
}

func typeImportRows() []domain.CatalogueImportRow {
	return []domain.CatalogueImportRow{{Row: 2, Label: "Theo văn bản", Order: "3"}, {Row: 3, Label: "Cơ bản", Code: "co-ban"}}
}

func priorityImportRows() []domain.CatalogueImportRow {
	return []domain.CatalogueImportRow{{Row: 2, Label: "Rất khẩn"}, {Row: 3, Label: "Theo dõi", Code: "theo-doi"}}
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

func TestCatalogueImport_BothCataloguesCommitRowsAndOneEntryPerRowInOneTransaction(t *testing.T) {
	for _, c := range []struct {
		name, table, action string
		codes               []string
		run                 func(db *store.DB) (CatalogueImportResult, error)
	}{
		{"loại nhiệm vụ", "INSERT INTO loai_nhiem_vu", HanhViThemLoaiNhiemVu, []string{"theo-van-ban", "co-ban"},
			func(db *store.DB) (CatalogueImportResult, error) {
				uc := NewTaskTypeImporter(db, docstore.NewLoaiNhiemVuStore(db))
				pinImportIDs(uc)
				return uc.Import(importCtx(), typeImportRows(), importActor())
			}},
		{"mức ưu tiên", "INSERT INTO muc_uu_tien_nhiem_vu", HanhViThemMucUuTien, []string{"rat-khan", "theo-doi"},
			func(db *store.DB) (CatalogueImportResult, error) {
				uc := NewTaskPriorityImporter(db, docstore.NewMucUuTienNhiemVuStore(db))
				pinImportIDs(uc)
				return uc.Import(importCtx(), priorityImportRows(), importActor())
			}},
	} {
		db, g := openImportDB(t)
		res, err := c.run(db)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(res.Entries) != 2 || res.Entries[0].Code != c.codes[0] || res.Entries[0].ID == "" || res.Batch == "" {
			t.Fatalf("%s: kết quả %+v", c.name, res)
		}
		if g.txCount != 1 || g.end(1) != "commit" {
			t.Fatalf("%s: %d giao dịch, kết thúc %q — muốn MỘT giao dịch commit", c.name, g.txCount, g.end(1))
		}
		lock := g.all("pg_advisory_xact_lock")
		if len(lock) != 1 || lock[0].tx != 1 {
			t.Errorf("%s: phải khoá lượt nhập trong giao dịch: %+v", c.name, lock)
		}
		inserts := g.all(c.table)
		if len(inserts) != 2 {
			t.Fatalf("%s: %d câu chèn, muốn 2", c.name, len(inserts))
		}
		for _, ins := range inserts {
			if ins.tx != 1 || !strings.Contains(ins.sql, "'don-vi', false") {
				t.Errorf("%s: câu chèn phải là câu của biểu mẫu, nguon là hằng: %s", c.name, ins.sql)
			}
			// (tenant, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung): never the default, always in use.
			if ins.args[5] != false || ins.args[6] != true {
				t.Errorf("%s: dòng nhập phải đang dùng và không mặc định: %v", c.name, ins.args)
			}
		}
		stmts, deltas := auditEntries(t, g)
		if len(stmts) != 2 {
			t.Fatalf("%s: %d vết, muốn một vết cho mỗi dòng", c.name, len(stmts))
		}
		for i, s := range stmts {
			if s.tx != 1 || s.args[1] != importStaffCode || s.args[4] != c.action || s.args[5] != c.codes[i] {
				t.Errorf("%s: vết %d: tx=%d actor=%v action=%v subject=%v", c.name, i, s.tx, s.args[1], s.args[4], s.args[5])
			}
			d := deltas[i]
			if d["nguon"] != catalogueImportSource || d["lo_nhap"] != res.Batch || d["dong"] != float64(i+2) || d["so_dong"] != float64(2) {
				t.Errorf("%s: delta %v", c.name, d)
			}
			if sau, _ := d["sau"].(map[string]any); sau["ma"] != c.codes[i] || sau["nguon"] != domain.NguonDonVi {
				t.Errorf("%s: delta.sau %v", c.name, d["sau"])
			}
		}
		for _, s := range g.stmts {
			if len(s.args) == 0 || s.args[0] != string(importCommune) {
				t.Errorf("%s: câu lệnh không mang xã của ngữ cảnh ở $1: %q %v", c.name, s.sql, s.args)
			}
		}
	}
}

// THE SCALE, over the real store: new levels are inserted AFTER the last live level, in file order.
func TestCatalogueImport_PrioritiesInsertedAfterTheLastLiveLevel(t *testing.T) {
	db, g := openImportDB(t)
	g.snapshot = [][]driver.Value{
		{"cu", "Cũ", int64(900), true},
		{"khan", "Khẩn", int64(0), false},
		{"thuong", "Thường", int64(7), false},
	}
	uc := NewTaskPriorityImporter(db, docstore.NewMucUuTienNhiemVuStore(db))
	if _, err := uc.Import(importCtx(), priorityImportRows(), importActor()); err != nil {
		t.Fatal(err)
	}
	ins := g.all("INSERT INTO muc_uu_tien_nhiem_vu")
	if len(ins) != 2 || ins[0].args[4] != int64(8) || ins[1].args[4] != int64(9) {
		t.Errorf("thu_tu chèn = %v / %v, muốn 8 rồi 9", ins[0].args, ins[1].args)
	}
}

func TestCatalogueImport_PreviewWritesNothing(t *testing.T) {
	db, g := openImportDB(t)
	uc := NewTaskTypeImporter(db, docstore.NewLoaiNhiemVuStore(db))
	res, err := uc.Preview(importCtx(), typeImportRows())
	if err != nil || len(res.Entries) != 2 || len(res.Errors) != 0 {
		t.Fatalf("xem trước: %+v %v", res, err)
	}
	if len(g.all("INSERT")) != 0 || len(g.all("pg_advisory_xact_lock")) != 0 || g.end(1) != "rollback" {
		t.Errorf("xem trước phải không khoá, không ghi, và luôn rollback: %q", g.end(1))
	}
}

// A DUPLICATE AGAINST THE CATALOGUE — here the derived code of a soft-deleted row — refuses the WHOLE
// file: nothing inserted, nothing audited.
func TestCatalogueImport_OneDuplicateRowWritesNothing(t *testing.T) {
	db, g := openImportDB(t)
	g.snapshot = [][]driver.Value{{"theo-van-ban", "Theo văn bản (cũ)", int64(1), true}}
	uc := NewTaskTypeImporter(db, docstore.NewLoaiNhiemVuStore(db))
	_, err := uc.Import(importCtx(), typeImportRows(), importActor())
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
	uc := NewTaskPriorityImporter(db, docstore.NewMucUuTienNhiemVuStore(db))
	if _, err := uc.Import(importCtx(), priorityImportRows(), importActor()); err == nil {
		t.Fatal("vết lỗi mà nhập vẫn thành công")
	}
	if g.end(1) != "rollback" || len(g.all("INSERT INTO muc_uu_tien_nhiem_vu")) != 1 {
		t.Errorf("vết lỗi phải huỷ cả tệp: %q", g.end(1))
	}
}

// A code a concurrent FORM took after the snapshot: the unique key refuses it, the file rolls back,
// and the caller can tell it apart (409 catalogue_changed).
func TestCatalogueImport_UniqueKeyRaceIsCodeTakenAndRollsBack(t *testing.T) {
	db, g := openImportDB(t)
	g.failOn["INSERT INTO loai_nhiem_vu"] = errors.New(`ERROR: duplicate key value violates unique constraint "loai_nhiem_vu_p07_tenant_id_ma_key" (SQLSTATE 23505)`)
	uc := NewTaskTypeImporter(db, docstore.NewLoaiNhiemVuStore(db))
	_, err := uc.Import(importCtx(), typeImportRows(), importActor())
	if !errors.Is(err, docstore.ErrMaDaTonTai) || g.end(1) != "rollback" {
		t.Errorf("lỗi = %v, kết thúc %q", err, g.end(1))
	}
	if strings.Contains(err.Error(), "theo-van-ban") {
		t.Errorf("lỗi bọc không được mang mã hay nhãn: %v", err)
	}
}

func TestCatalogueImport_RefusesActorWithoutStaffCode(t *testing.T) {
	db, g := openImportDB(t)
	uc := NewTaskTypeImporter(db, docstore.NewLoaiNhiemVuStore(db))
	if _, err := uc.Import(importCtx(), typeImportRows(), audit.Actor{Kind: "staff"}); !errors.Is(err, ErrCatalogueImportNoActor) {
		t.Fatalf("thiếu mã cán bộ mà vẫn nhập: %v", err)
	}
	if g.txCount != 0 {
		t.Error("từ chối người thực hiện phải xảy ra trước khi mở giao dịch")
	}
}
