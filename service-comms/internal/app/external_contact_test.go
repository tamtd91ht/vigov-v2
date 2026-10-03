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

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// A fake database/sql driver under the REAL ExternalContactStore — the argument of
// map_field_schema_test.go: what is worth proving (one transaction for the write and its entry, a
// refusal commits nothing, the commune is $1 from the context, created_by / deleted_by is the business
// code, the phone is MASKED and the address ABSENT in the audit delta, the read excludes deleted rows)
// lives in the SQL and the transaction boundaries, which a fake store would erase.
//
// WHAT IT DOES NOT PROVE: anything PostgreSQL does — the external_contacts_guard trigger, the CHECKs,
// NULLS LAST. That half is internal/store/external_contact_pg_test.go, which SKIPS without
// VIGOV_TEST_DSN.

type ecStmt struct {
	sql  string
	args []driver.Value
}

// ecRow is what the FOR UPDATE read and the list hand back.
type ecRow struct {
	id, name, category, phone string
	address                   any // string or nil
	order                     any // int64 or nil
}

type ecDB struct {
	mu    sync.Mutex
	stmts []ecStmt

	begun, committed, rolledBack int

	liveCount     int
	row           *ecRow
	list          []ecRow
	failOnContain string
}

func (d *ecDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, ecStmt{sql: q, args: vals})
	d.mu.Unlock()
}

func (d *ecDB) with(sub string) []ecStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []ecStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *ecDB) indexOf(sub string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

func (d *ecDB) Connect(context.Context) (driver.Conn, error) { return &ecConn{d: d}, nil }
func (d *ecDB) Driver() driver.Driver                        { return ecDriverOpen{} }

type ecDriverOpen struct{}

func (ecDriverOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type ecConn struct{ d *ecDB }

func (c *ecConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *ecConn) Close() error { return nil }
func (c *ecConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *ecConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &ecTx{d: c.d}, nil
}

func (c *ecConn) fail(q string) error {
	if c.d.failOnContain != "" && strings.Contains(q, c.d.failOnContain) {
		return errors.New("fake driver: this statement is built to fail")
	}
	return nil
}

func (c *ecConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if err := c.fail(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

var ecCols = []string{"id", "name", "category", "phone", "address", "display_order"}

func (r ecRow) values() []driver.Value {
	return []driver.Value{r.id, r.name, r.category, r.phone, r.address, r.order}
}

func (c *ecConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	if err := c.fail(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "count(*)"):
		return &ecRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.d.liveCount)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.d.row == nil {
			return &ecRows{cols: ecCols}, nil
		}
		return &ecRows{cols: ecCols, rows: [][]driver.Value{c.d.row.values()}}, nil
	case strings.Contains(q, "ORDER BY"):
		rows := make([][]driver.Value, 0, len(c.d.list))
		for _, r := range c.d.list {
			rows = append(rows, r.values())
		}
		return &ecRows{cols: ecCols, rows: rows}, nil
	}
	return nil, fmt.Errorf("fake driver: no answer for %q", q)
}

type ecTx struct{ d *ecDB }

func (t *ecTx) Commit() error {
	t.d.mu.Lock()
	t.d.committed++
	t.d.mu.Unlock()
	return nil
}

func (t *ecTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rolledBack++
	t.d.mu.Unlock()
	return nil
}

type ecRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *ecRows) Columns() []string { return r.cols }
func (r *ecRows) Close() error      { return nil }
func (r *ecRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

const pinnedContactID = "01JEXTERNALCONTACTPINNED00"

// fakePhone is the agreed fake number (rule 3, invariant 5); its mask is what the trail must hold.
const fakePhone = "0900000000"

func newContactUseCase(t *testing.T, d *ecDB) (*ExternalContacts, *commsstore.ExternalContactStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	handle := store.New(db)
	repo := commsstore.NewExternalContactStore(handle)
	uc := NewExternalContacts(handle, repo)
	uc.newID = func() (string, error) { return pinnedContactID, nil }
	return uc, repo, tenant.Into(context.Background(), xaA)
}

func validContact() ExternalContactInput {
	order := 2
	return ExternalContactInput{
		Name: "Trạm Y tế xã", Category: "Y tế", Phone: fakePhone, Address: "Thôn 1, xã A", DisplayOrder: &order,
	}
}

func stationRow() *ecRow {
	return &ecRow{id: "ec-001", name: "Trạm Y tế xã", category: "Y tế", phone: fakePhone,
		address: "Thôn 1, xã A", order: int64(2)}
}

// contactAuditDelta decodes the delta of the single audit entry ($8).
func contactAuditDelta(t *testing.T, d *ecDB) (ecStmt, map[string]any) {
	t.Helper()
	entries := d.with("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d audit entries, want 1", len(entries))
	}
	raw, _ := entries[0].args[7].([]byte)
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatalf("delta is not JSON: %v", err)
	}
	return entries[0], delta
}

// assertNoRawPersonalData: the full number and the address never enter the trail (rule 6 forbidden #4).
func assertNoRawPersonalData(t *testing.T, a ecStmt) {
	t.Helper()
	raw, _ := a.args[7].([]byte)
	for _, banned := range []string{fakePhone, "Thôn 1", "Thôn 2"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("audit delta holds %q raw: %s", banned, raw)
		}
	}
}

// --- create -----------------------------------------------------------------------------------

func TestCreateExternalContactWritesRowAndEntryInOneTransaction(t *testing.T) {
	d := &ecDB{}
	uc, _, ctx := newContactUseCase(t, d)

	got, err := uc.Create(ctx, validContact(), staffActor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx begun/committed/rolled back = %d/%d/%d, want 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	ins := d.with("INSERT INTO external_contacts")
	if len(ins) != 1 {
		t.Fatalf("%d INSERTs, want 1", len(ins))
	}
	// $1 is the commune from the context (rule 1, invariant 4); $8 is created_by = the BUSINESS code.
	if ins[0].args[0] != string(xaA) || ins[0].args[1] != pinnedContactID || ins[0].args[7] != "CB-00123" {
		t.Errorf("INSERT args = %v", ins[0].args)
	}
	if ins[0].args[6] != int64(2) {
		t.Errorf("display_order written = %v", ins[0].args[6])
	}
	if d.indexOf("INSERT INTO audit_log") < d.indexOf("INSERT INTO external_contacts") {
		t.Fatal("the entry must follow the write, inside its transaction")
	}
	a, delta := contactAuditDelta(t, d)
	if a.args[1] != "CB-00123" || a.args[4] != ActionCreateExternalContact || a.args[5] != "lien-he-ngoai-bo-may/Trạm Y tế xã" {
		t.Errorf("audit actor/action/subject = %v/%v/%v", a.args[1], a.args[4], a.args[5])
	}
	after, _ := delta["sau"].(map[string]any)
	if after["phone"] != "09****0000" || after["has_address"] != true || after["id"] != pinnedContactID {
		t.Errorf("delta.sau = %v", after)
	}
	if _, ok := after["address"]; ok {
		t.Error("the address entered the trail")
	}
	assertNoRawPersonalData(t, a)
	if got.ID != pinnedContactID || got.Phone != fakePhone {
		t.Errorf("returned row = %+v", got)
	}
}

func TestCreateExternalContactBlankAddressIsNull(t *testing.T) {
	d := &ecDB{}
	uc, _, ctx := newContactUseCase(t, d)
	in := validContact()
	in.Address, in.DisplayOrder = "   ", nil
	if _, err := uc.Create(ctx, in, staffActor); err != nil {
		t.Fatalf("Create: %v", err)
	}
	ins := d.with("INSERT INTO external_contacts")[0]
	if ins.args[5] != nil || ins.args[6] != nil {
		t.Errorf("address/display_order = %v/%v, want NULL/NULL", ins.args[5], ins.args[6])
	}
}

func TestCreateExternalContactRefusals(t *testing.T) {
	t.Run("ceiling reached commits nothing", func(t *testing.T) {
		d := &ecDB{liveCount: commsstore.ExternalContactCeiling}
		uc, _, ctx := newContactUseCase(t, d)
		if _, err := uc.Create(ctx, validContact(), staffActor); !errors.Is(err, commsstore.ErrExternalContactsFull) {
			t.Fatalf("err = %v", err)
		}
		if len(d.with("INSERT")) != 0 || d.committed != 0 || d.rolledBack != 1 {
			t.Errorf("inserts=%d committed=%d rolledBack=%d", len(d.with("INSERT")), d.committed, d.rolledBack)
		}
	})
	for name, mutate := range map[string]func(*ExternalContactInput){
		"phone with letters": func(in *ExternalContactInput) { in.Phone = "0900000000 máy lẻ 12" },
		"blank name":         func(in *ExternalContactInput) { in.Name = " " },
		"blank category":     func(in *ExternalContactInput) { in.Category = "" },
		"negative order":     func(in *ExternalContactInput) { n := -1; in.DisplayOrder = &n },
	} {
		t.Run(name+" refused before any transaction", func(t *testing.T) {
			d := &ecDB{}
			uc, _, ctx := newContactUseCase(t, d)
			in := validContact()
			mutate(&in)
			if _, err := uc.Create(ctx, in, staffActor); err == nil {
				t.Fatal("accepted")
			}
			if d.begun != 0 {
				t.Error("a shape refusal opened a transaction")
			}
		})
	}
	t.Run("no business code refuses, never falls back", func(t *testing.T) {
		d := &ecDB{}
		uc, _, ctx := newContactUseCase(t, d)
		actor := staffActor
		actor.ID = ""
		if _, err := uc.Create(ctx, validContact(), actor); !errors.Is(err, ErrMissingActor) {
			t.Fatalf("err = %v", err)
		}
		if d.begun != 0 {
			t.Error("transaction opened without an actor")
		}
	})
}

func TestCreateExternalContactAuditFailureRollsBackTheRow(t *testing.T) {
	d := &ecDB{failOnContain: "INSERT INTO audit_log"}
	uc, _, ctx := newContactUseCase(t, d)
	if _, err := uc.Create(ctx, validContact(), staffActor); err == nil {
		t.Fatal("audit write failed and Create still succeeded")
	}
	if len(d.with("INSERT INTO external_contacts")) != 1 || d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("insert ran=%d committed=%d rolledBack=%d, want 1/0/1",
			len(d.with("INSERT INTO external_contacts")), d.committed, d.rolledBack)
	}
}

// --- update -----------------------------------------------------------------------------------

func TestUpdateExternalContactAuditsOnlyWhatMovedMasked(t *testing.T) {
	d := &ecDB{row: stationRow()}
	uc, _, ctx := newContactUseCase(t, d)
	phone, address := "0911111111", "Thôn 2, xã A"
	got, err := uc.Update(ctx, "ec-001", ExternalContactPatch{Phone: &phone, Address: &address}, staffActor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Phone != phone || got.Name != "Trạm Y tế xã" || got.DisplayOrder == nil || *got.DisplayOrder != 2 {
		t.Errorf("merged row = %+v", got)
	}
	upd := d.with("UPDATE external_contacts")
	if len(upd) != 1 || upd[0].args[0] != string(xaA) || upd[0].args[7] != "CB-00123" {
		t.Fatalf("UPDATE = %v", upd)
	}
	if !strings.Contains(d.with("FOR UPDATE")[0].sql, "deleted_at IS NULL") {
		t.Error("the locked read does not exclude deleted rows")
	}
	a, delta := contactAuditDelta(t, d)
	if a.args[4] != ActionUpdateExternalContact {
		t.Errorf("action = %v", a.args[4])
	}
	before, _ := delta["truoc"].(map[string]any)
	after, _ := delta["sau"].(map[string]any)
	if before["phone"] != "09****0000" || after["phone"] != "09****1111" {
		t.Errorf("phone before/after = %v/%v, want masked", before["phone"], after["phone"])
	}
	if after["address_changed"] != true {
		t.Errorf("address change not recorded: %v", after)
	}
	if _, moved := after["name"]; moved {
		t.Error("an unchanged field entered the diff")
	}
	assertNoRawPersonalData(t, a)
}

func TestUpdateExternalContactNoOpWritesNothing(t *testing.T) {
	d := &ecDB{row: stationRow()}
	uc, _, ctx := newContactUseCase(t, d)
	same := "  Trạm Y tế xã " // trims to the stored value
	if _, err := uc.Update(ctx, "ec-001", ExternalContactPatch{Name: &same}, staffActor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(d.with("UPDATE external_contacts")) != 0 || len(d.with("INSERT INTO audit_log")) != 0 {
		t.Error("a no-op edit wrote a row or an entry — idem.KhongCan on PATCH would become a lie")
	}
}

func TestUpdateExternalContactRefusals(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		d := &ecDB{}
		uc, _, ctx := newContactUseCase(t, d)
		name := "X"
		if _, err := uc.Update(ctx, "ec-404", ExternalContactPatch{Name: &name}, staffActor); !errors.Is(err, commsstore.ErrExternalContactNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad phone after merge commits nothing", func(t *testing.T) {
		d := &ecDB{row: stationRow()}
		uc, _, ctx := newContactUseCase(t, d)
		bad := "abc"
		if _, err := uc.Update(ctx, "ec-001", ExternalContactPatch{Phone: &bad}, staffActor); !errors.Is(err, domain.ErrExternalContactPhoneShape) {
			t.Fatalf("err = %v", err)
		}
		if len(d.with("UPDATE external_contacts")) != 0 || d.committed != 0 {
			t.Error("a refused edit wrote")
		}
	})
	t.Run("address cleared with empty string", func(t *testing.T) {
		d := &ecDB{row: stationRow()}
		uc, _, ctx := newContactUseCase(t, d)
		empty := ""
		if _, err := uc.Update(ctx, "ec-001", ExternalContactPatch{Address: &empty}, staffActor); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if upd := d.with("UPDATE external_contacts"); len(upd) != 1 || upd[0].args[5] != nil {
			t.Fatalf("address not written as NULL: %v", upd)
		}
	})
}

// --- delete -----------------------------------------------------------------------------------

func TestDeleteExternalContactSoftDeletesWithReasonAndEntry(t *testing.T) {
	d := &ecDB{row: stationRow()}
	uc, _, ctx := newContactUseCase(t, d)
	if err := uc.Delete(ctx, "ec-001", "  Trạm đã sáp nhập  ", staffActor); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	del := d.with("SET deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("%d soft deletes, want 1", len(del))
	}
	if !strings.Contains(del[0].sql, "AND deleted_at IS NULL") {
		t.Error("a second delete could overwrite who deleted it")
	}
	if del[0].args[0] != string(xaA) || del[0].args[2] != "CB-00123" || del[0].args[3] != "Trạm đã sáp nhập" {
		t.Errorf("soft delete args = %v", del[0].args)
	}
	if len(d.with("DELETE")) != 0 {
		t.Fatal("a hard DELETE was issued")
	}
	a, delta := contactAuditDelta(t, d)
	if a.args[4] != ActionDeleteExternalContact || delta["ly_do"] != "Trạm đã sáp nhập" || delta["xoa_mem"] != true {
		t.Errorf("entry = %v / %v", a.args[4], delta)
	}
	assertNoRawPersonalData(t, a)
	if d.committed != 1 {
		t.Errorf("committed = %d", d.committed)
	}
}

func TestDeleteExternalContactNeedsReason(t *testing.T) {
	d := &ecDB{row: stationRow()}
	uc, _, ctx := newContactUseCase(t, d)
	if err := uc.Delete(ctx, "ec-001", "   ", staffActor); !errors.Is(err, domain.ErrThieuLyDoXoa) {
		t.Fatalf("err = %v", err)
	}
	if d.begun != 0 {
		t.Error("a missing reason opened a transaction")
	}
}

// --- the list (store.List through the same driver) ---------------------------------------------

func TestExternalContactListIsScopedLiveAndOrdered(t *testing.T) {
	d := &ecDB{list: []ecRow{
		{id: "a", name: "Công an xã", category: "Công an", phone: "113", address: nil, order: int64(1)},
		{id: "b", name: "Điện lực", category: "Điện", phone: "19001006", address: "Thôn 3", order: nil},
	}}
	_, repo, ctx := newContactUseCase(t, d)
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	q := d.with("FROM external_contacts")[0]
	for _, want := range []string{"WHERE tenant_id = $1", "deleted_at IS NULL", "ORDER BY display_order, id", "LIMIT $2"} {
		if !strings.Contains(q.sql, want) {
			t.Errorf("list SQL lacks %q: %s", want, q.sql)
		}
	}
	if q.args[0] != string(xaA) || q.args[1] != int64(commsstore.ExternalContactCeiling+1) {
		t.Errorf("list args = %v", q.args)
	}
	if len(got) != 2 || got[0].Address != "" || got[0].DisplayOrder == nil || *got[0].DisplayOrder != 1 ||
		got[1].Address != "Thôn 3" || got[1].DisplayOrder != nil || got[1].Phone != "19001006" {
		t.Errorf("scanned = %+v", got)
	}
}

func TestExternalContactListRefusesPastCeiling(t *testing.T) {
	rows := make([]ecRow, commsstore.ExternalContactCeiling+1)
	for i := range rows {
		rows[i] = ecRow{id: fmt.Sprintf("r%03d", i), name: "N", category: "C", phone: "113"}
	}
	d := &ecDB{list: rows}
	_, repo, ctx := newContactUseCase(t, d)
	if got, err := repo.List(ctx); !errors.Is(err, commsstore.ErrTooManyExternalContacts) || got != nil {
		t.Fatalf("List past ceiling = %d rows, %v — want a refusal, never a truncated list", len(got), err)
	}
}
