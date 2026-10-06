package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The Excel import of disbursement vouchers, through the REAL store over a THIRD fake driver — the
// argument driver_gia_chung_tu_test.go makes for its own: what is asserted is a property of the SQL and
// the transaction (all or nothing, the trail inside the transaction, the commune bound on every
// statement), and a fake store erases exactly that. A separate driver because the import asks two
// statements (`ma = ANY($2)`, `SELECT id, ten FROM nguon_von`) the voucher driver answers in another
// shape. Like that one, it proves nothing PostgreSQL does.

type voucherImportProject struct {
	id        string
	allocated []string
}

type voucherImportDB struct {
	mu    sync.Mutex
	stmts []lenhGhi

	begun, committed, rolledBack int

	// projects by CODE, sources by ID → NAME. Anything absent is what another commune's row looks like
	// from inside this one: the statement binds tenant_id = $1, so it is not there.
	projects map[string]voucherImportProject
	sources  map[string]string

	failOn  string // fail the failNth statement containing this
	failNth int    // 1-based; 0 = first
	seen    int
}

func (k *voucherImportDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, lenhGhi{sql: q, args: vals})
	k.mu.Unlock()
}

func (k *voucherImportDB) matching(sub string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []lenhGhi
	for _, l := range k.stmts {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *voucherImportDB) maybeFail(q string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failOn != "" && strings.Contains(q, k.failOn) {
		k.seen++
		want := max(k.failNth, 1)
		if k.seen == want {
			return errors.New("fake driver: this statement is built to fail")
		}
	}
	return nil
}

func (k *voucherImportDB) Connect(context.Context) (driver.Conn, error) {
	return &voucherImportConn{k: k}, nil
}
func (k *voucherImportDB) Driver() driver.Driver { return trinhGia{} }

type voucherImportConn struct{ k *voucherImportDB }

func (c *voucherImportConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: Prepare unsupported")
}
func (c *voucherImportConn) Close() error { return nil }
func (c *voucherImportConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *voucherImportConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &voucherImportTx{k: c.k}, nil
}

// CheckNamedValue lets a []string through untouched, as pgx's stdlib driver does, so `ANY($2)` can be
// modelled; everything else takes database/sql's default conversion.
func (c *voucherImportConn) CheckNamedValue(nv *driver.NamedValue) error {
	if _, ok := nv.Value.([]string); ok {
		return nil
	}
	return driver.ErrSkip
}

func (c *voucherImportConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *voucherImportConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}
	arg := func(i int) any {
		if i < len(args) {
			return args[i].Value
		}
		return nil
	}
	switch {
	case strings.Contains(q, "FROM du_an") && strings.Contains(q, "ANY($2)"):
		out := &rowsGia{cot: []string{"id", "ma"}}
		codes, _ := arg(1).([]string)
		for _, code := range codes {
			if p, ok := c.k.projects[code]; ok {
				out.hang = append(out.hang, []driver.Value{p.id, code})
			}
		}
		return out, nil
	case strings.Contains(q, "FROM phan_bo_nguon_von") && strings.Contains(q, "ANY($2)"):
		out := &rowsGia{cot: []string{"du_an_id", "nguon_von_id"}}
		ids, _ := arg(1).([]string)
		for _, id := range ids {
			for _, p := range c.k.projects {
				if p.id == id {
					for _, s := range p.allocated {
						out.hang = append(out.hang, []driver.Value{id, s})
					}
				}
			}
		}
		return out, nil
	case strings.Contains(q, "FROM phan_bo_nguon_von"):
		out := &rowsGia{cot: []string{"nguon_von_id"}}
		for _, p := range c.k.projects {
			if p.id == fmt.Sprint(arg(1)) {
				for _, s := range p.allocated {
					out.hang = append(out.hang, []driver.Value{s})
				}
			}
		}
		return out, nil
	case strings.Contains(q, "FROM du_an"):
		for code, p := range c.k.projects {
			if p.id == fmt.Sprint(arg(1)) {
				return &rowsGia{cot: []string{"ma"}, hang: [][]driver.Value{{code}}}, nil
			}
		}
		return &rowsGia{cot: []string{"ma"}}, nil
	case strings.Contains(q, "SELECT id, ten FROM nguon_von"):
		out := &rowsGia{cot: []string{"id", "ten"}}
		for id, name := range c.k.sources {
			out.hang = append(out.hang, []driver.Value{id, name})
		}
		return out, nil
	case strings.Contains(q, "FROM nguon_von"):
		if _, ok := c.k.sources[fmt.Sprint(arg(1))]; ok {
			return &rowsGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
		}
		return &rowsGia{cot: []string{"?column?"}}, nil
	}
	return nil, fmt.Errorf("fake driver: no answer for %q", q)
}

type voucherImportTx struct{ k *voucherImportDB }

func (t *voucherImportTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	return nil
}

func (t *voucherImportTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	return nil
}

const (
	projectWithSourceID = "01JPROJECTWITHSOURCE00000A"
	projectNoSourceID   = "01JPROJECTNOSOURCE0000000B"
	provinceSourceID    = "01JSOURCEPROVINCE00000000C"
	communeSourceID     = "01JSOURCECOMMUNE000000000D"
)

// sampleImportDB: DA01 draws on "Ngân sách tỉnh"; DA02 declared no source; "Ngân sách xã" exists in
// the catalogue but is allocated to nothing.
func sampleImportDB() *voucherImportDB {
	return &voucherImportDB{
		projects: map[string]voucherImportProject{
			"DA01": {id: projectWithSourceID, allocated: []string{provinceSourceID}},
			"DA02": {id: projectNoSourceID},
		},
		sources: map[string]string{provinceSourceID: "Ngân sách tỉnh", communeSourceID: "Ngân sách xã"},
	}
}

func newImporterOver(t *testing.T, k *voucherImportDB) (*DisbursementImporter, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	s := store.New(db)
	uc := NewDisbursementImporter(s, fistore.NewChungTuGiaiNganStore(s))
	n := 0
	uc.newID = func() (string, error) { n++; return fmt.Sprintf("01JID%021d", n), nil }
	return uc, tenant.Into(context.Background(), xaA)
}

func importRow(row int, code, source string) domain.VoucherImportRow {
	return domain.VoucherImportRow{Row: row, ProjectCode: code, PaymentDate: "15/03/2026",
		Amount: "1.000.000", Description: "Thanh toán đợt 1", Counterparty: "Công ty TNHH Mẫu",
		VoucherNo: "CT-0001", Source: source}
}

func validImportRows() []domain.VoucherImportRow {
	return []domain.VoucherImportRow{importRow(2, "DA01", "Ngân sách tỉnh"), importRow(3, "DA02", "")}
}

// --- commit: every row, every trail, one transaction ------------------------------------------------

func TestVoucherImport_WritesEveryRowAndTrailInOneTransaction(t *testing.T) {
	k := sampleImportDB()
	uc, ctx := newImporterOver(t, k)
	res, err := uc.Import(ctx, validImportRows(), canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if k.begun != 1 || k.committed != 1 || k.rolledBack != 0 {
		t.Fatalf("transactions begun %d, committed %d, rolled back %d — want 1/1/0", k.begun, k.committed, k.rolledBack)
	}
	ins, trail := k.matching("INSERT INTO chung_tu_giai_ngan"), k.matching("INSERT INTO audit_log")
	if len(ins) != 2 || len(trail) != 2 {
		t.Fatalf("%d vouchers, %d trail entries — want 2/2", len(ins), len(trail))
	}
	if res.RowCount != 2 || res.Total != 2_000_000 || len(res.Created) != 2 || res.Batch == "" {
		t.Errorf("result %+v", res)
	}
	// The source column became the source id on the project with allocations, NULL on the other.
	if got := ins[0].args[8]; got != provinceSourceID {
		t.Errorf("nguon_von_id of row 2 = %v, want %q", got, provinceSourceID)
	}
	if got := ins[1].args[8]; got != nil {
		t.Errorf("nguon_von_id of row 3 = %v, want NULL", got)
	}
	for i, v := range trail {
		// audit.Write binds (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta).
		if v.args[1] != maKeToan || v.args[4] != HanhViThemChungTu {
			t.Errorf("trail %d: actor %v action %v", i, v.args[1], v.args[4])
		}
		d := thanVet(t, v)
		for _, part := range []string{`"nguon":"nhap_excel"`, `"lo_nhap":"` + res.Batch + `"`, `"so_dong":2`} {
			if !strings.Contains(d, part) {
				t.Errorf("trail %d lacks %s: %s", i, part, d)
			}
		}
	}
	if trail[0].args[5] != "DA01" || trail[1].args[5] != "DA02" {
		t.Errorf("subject = %v, %v — want the project codes", trail[0].args[5], trail[1].args[5])
	}
}

// --- all or nothing -------------------------------------------------------------------------------

func TestVoucherImport_OneBadRowWritesNothing(t *testing.T) {
	k := sampleImportDB()
	uc, ctx := newImporterOver(t, k)
	rows := append(validImportRows(), importRow(4, "DA99", ""))
	_, err := uc.Import(ctx, rows, canBoCT(maKeToan))
	var rej *VoucherImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 4 || rej.Errors[0].Column != domain.VoucherImportColProject {
		t.Fatalf("err = %v", err)
	}
	if n := len(k.matching("INSERT")); n != 0 || k.committed != 0 || k.rolledBack != 1 {
		t.Errorf("a file with a bad row still wrote: %d inserts, committed %d, rolled back %d", n, k.committed, k.rolledBack)
	}
}

// THE LAST STATEMENT FAILING takes every row before it down: row 3's trail fails after row 2's voucher
// AND trail are already in.
func TestVoucherImport_LastTrailFailingRollsBackTheWholeFile(t *testing.T) {
	k := sampleImportDB()
	k.failOn, k.failNth = "INSERT INTO audit_log", 2
	uc, ctx := newImporterOver(t, k)
	if _, err := uc.Import(ctx, validImportRows(), canBoCT(maKeToan)); err == nil {
		t.Fatal("a trail failed and Import still reported success")
	}
	if len(k.matching("INSERT INTO chung_tu_giai_ngan")) != 2 || k.committed != 0 || k.rolledBack != 1 {
		t.Errorf("committed %d rolled back %d", k.committed, k.rolledBack)
	}
}

// --- dry run --------------------------------------------------------------------------------------

func TestVoucherImport_PreviewWritesNothing(t *testing.T) {
	for name, rows := range map[string][]domain.VoucherImportRow{
		"valid file":      validImportRows(),
		"file with error": append(validImportRows(), importRow(4, "DA01", "")),
	} {
		k := sampleImportDB()
		uc, ctx := newImporterOver(t, k)
		res, err := uc.Preview(ctx, rows)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n := len(k.matching("INSERT")); n != 0 || k.committed != 0 || k.rolledBack != 1 {
			t.Errorf("%s: the preview wrote: %d inserts, committed %d", name, n, k.committed)
		}
		if res.RowCount != len(rows) {
			t.Errorf("%s: row_count %d", name, res.RowCount)
		}
	}
}

// --- the source rule, end to end --------------------------------------------------------------------

func TestVoucherImport_SourceRule(t *testing.T) {
	for _, c := range []struct {
		name, code, source string
		ok                 bool
	}{
		{"allocated, names its source", "DA01", "Ngân sách tỉnh", true},
		{"allocated, other case", "DA01", "ngân sách TỈNH", true},
		{"allocated, blank", "DA01", "", false},
		{"allocated, source not allocated to it", "DA01", "Ngân sách xã", false},
		{"allocated, source not in the commune", "DA01", "Ngân sách huyện", false},
		{"no allocation, blank", "DA02", "", true},
		{"no allocation, names a source", "DA02", "Ngân sách tỉnh", false},
	} {
		k := sampleImportDB()
		uc, ctx := newImporterOver(t, k)
		res, err := uc.Preview(ctx, []domain.VoucherImportRow{importRow(2, c.code, c.source)})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := len(res.Errors) == 0; got != c.ok {
			t.Errorf("%s: valid = %v, want %v (%+v)", c.name, got, c.ok, res.Errors)
		}
		if !c.ok && (len(res.Errors) != 1 || res.Errors[0].Column != domain.VoucherImportColSource) {
			t.Errorf("%s: the error must sit on the source column: %+v", c.name, res.Errors)
		}
	}
}

// --- commune isolation ----------------------------------------------------------------------------

// EVERY statement binds the commune of the CONTEXT as $1 (rule 1, invariants 4 and 5) — the project
// lookup by code included, so a code of another commune can only ever come back absent.
func TestVoucherImport_EveryStatementBindsTheContextCommune(t *testing.T) {
	k := sampleImportDB()
	uc, ctx := newImporterOver(t, k)
	if _, err := uc.Import(ctx, validImportRows(), canBoCT(maKeToan)); err != nil {
		t.Fatal(err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.stmts) == 0 {
		t.Fatal("no statement ran")
	}
	for _, l := range k.stmts {
		if len(l.args) == 0 || l.args[0] != string(xaA) {
			t.Errorf("statement does not bind commune A as $1: %q %v", l.sql, l.args)
		}
	}
}

func TestVoucherImport_NoStaffCodeIsRefusedBeforeAnyTransaction(t *testing.T) {
	k := sampleImportDB()
	uc, ctx := newImporterOver(t, k)
	if _, err := uc.Import(ctx, validImportRows(), canBoCT("")); !errors.Is(err, ErrVoucherImportNoActor) {
		t.Fatalf("err = %v", err)
	}
	if k.begun != 0 {
		t.Error("no actor, yet a transaction was opened")
	}
}
