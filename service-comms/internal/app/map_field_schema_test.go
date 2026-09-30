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
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// A fake database/sql driver under the REAL MapFieldSchemaStore — same argument as
// driver_gia_danh_muc_test.go: the properties worth proving (one transaction for the write and its
// entry, a refusal commits nothing, the commune is $1, deleted_by is the business code) live in the
// SQL and the transaction boundaries, which a fake store would erase.
//
// WHAT IT DOES NOT PROVE: anything PostgreSQL does — the map_field_schema_guard trigger, the CHECK
// constraints, the composite unique key, the foreign key to loai_tai_nguyen_ban_do. That half needs
// a real server (VIGOV_TEST_DSN is unset here).

type fieldStmt struct {
	sql  string
	args []driver.Value
}

// fieldRow is what the FOR UPDATE read hands back; nil means "no such live row".
type fieldRow struct {
	id, typeCode, key, label, valueType, options string
	required, active                             bool
	sortOrder                                    int
}

type fieldDB struct {
	mu    sync.Mutex
	stmts []fieldStmt

	begun, committed, rolledBack int

	typeLive      bool
	liveCount     int
	keyLive       int
	keyRetired    int
	row           *fieldRow
	failOnContain string
	failed        bool
}

func (d *fieldDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, fieldStmt{sql: q, args: vals})
	d.mu.Unlock()
}

func (d *fieldDB) with(sub string) []fieldStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []fieldStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

// indexOf is the position of the first statement containing sub, or -1.
func (d *fieldDB) indexOf(sub string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

func (d *fieldDB) maybeFail(q string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failOnContain != "" && !d.failed && strings.Contains(q, d.failOnContain) {
		d.failed = true
		return errors.New("fake driver: this statement is built to fail")
	}
	return nil
}

func (d *fieldDB) Connect(context.Context) (driver.Conn, error) { return &fieldConn{d: d}, nil }
func (d *fieldDB) Driver() driver.Driver                        { return fieldDriverOpen{} }

type fieldDriverOpen struct{}

func (fieldDriverOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type fieldConn struct{ d *fieldDB }

func (c *fieldConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *fieldConn) Close() error { return nil }
func (c *fieldConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fieldConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &fieldTx{d: c.d}, nil
}

func (c *fieldConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if err := c.d.maybeFail(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *fieldConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	if err := c.d.maybeFail(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "FROM loai_tai_nguyen_ban_do"):
		if !c.d.typeLive {
			return &fieldRows{cols: []string{"one"}}, nil
		}
		return &fieldRows{cols: []string{"one"}, rows: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FILTER"):
		return &fieldRows{cols: []string{"live", "retired"},
			rows: [][]driver.Value{{int64(c.d.keyLive), int64(c.d.keyRetired)}}}, nil
	case strings.Contains(q, "count(*)"):
		return &fieldRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(c.d.liveCount)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		cols := []string{"id", "asset_type_code", "field_code", "label", "value_type", "options",
			"is_required", "sort_order", "is_active"}
		if c.d.row == nil {
			return &fieldRows{cols: cols}, nil
		}
		r := *c.d.row
		return &fieldRows{cols: cols, rows: [][]driver.Value{{
			r.id, r.typeCode, r.key, r.label, r.valueType, []byte(r.options),
			r.required, int64(r.sortOrder), r.active,
		}}}, nil
	}
	return nil, fmt.Errorf("fake driver: no answer for %q", q)
}

type fieldTx struct{ d *fieldDB }

func (t *fieldTx) Commit() error {
	t.d.mu.Lock()
	t.d.committed++
	t.d.mu.Unlock()
	return nil
}

func (t *fieldTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rolledBack++
	t.d.mu.Unlock()
	return nil
}

type fieldRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fieldRows) Columns() []string { return r.cols }
func (r *fieldRows) Close() error      { return nil }
func (r *fieldRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

const pinnedFieldID = "01JMAPFIELDSCHEMAPINNED00"

// staffActor is what the handler builds from a principal: the BUSINESS CODE, never the internal id.
var staffActor = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func newFieldUseCase(t *testing.T, d *fieldDB) (*MapFieldSchemas, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	handle := store.New(db)
	uc := NewMapFieldSchemas(handle, commsstore.NewMapFieldSchemaStore(handle))
	uc.newID = func() (string, error) { return pinnedFieldID, nil }
	return uc, tenant.Into(context.Background(), xaA)
}

func validCreate() CreateMapFieldRequest {
	return CreateMapFieldRequest{
		AssetTypeCode: "nhom-mau", FieldCode: "legal_form", Label: "Loại hình doanh nghiệp",
		ValueType: domain.ValueTypeChoice,
		Options:   []domain.FieldOption{{Value: "tnhh", Label: "Công ty TNHH"}},
		SortOrder: 1,
	}
}

func choiceRow() *fieldRow {
	return &fieldRow{
		id: "mf-001", typeCode: "nhom-mau", key: "legal_form", label: "Loại hình",
		valueType: domain.ValueTypeChoice,
		options:   `[{"value":"tnhh","label":"Công ty TNHH"},{"value":"cp","label":"Cổ phần"}]`,
		active:    true, sortOrder: 1,
	}
}

// --- create -----------------------------------------------------------------------------------

func TestCreateMapFieldWritesRowAndEntryInOneTransaction(t *testing.T) {
	d := &fieldDB{typeLive: true}
	uc, ctx := newFieldUseCase(t, d)

	got, err := uc.Create(ctx, validCreate(), staffActor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx begun/committed/rolled back = %d/%d/%d, want 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	ins := d.with("INSERT INTO map_field_schema")
	if len(ins) != 1 {
		t.Fatalf("%d INSERTs, want 1", len(ins))
	}
	// $1 is the commune from the context (rule 1, invariant 4).
	if ins[0].args[0] != string(xaA) {
		t.Errorf("INSERT $1 = %v, want the context commune %q", ins[0].args[0], xaA)
	}
	if opts, _ := ins[0].args[6].(string); !strings.Contains(opts, `"value":"tnhh"`) {
		t.Errorf("options written = %v", ins[0].args[6])
	}
	// The entry comes AFTER the insert, inside the same transaction (one connection, one tx).
	iIns, iAudit := d.indexOf("INSERT INTO map_field_schema"), d.indexOf("INSERT INTO audit_log")
	if iAudit < 0 || iAudit < iIns {
		t.Fatalf("audit entry at %d, insert at %d — the entry must follow the write in its tx", iAudit, iIns)
	}
	a := d.with("INSERT INTO audit_log")[0]
	if a.args[1] != "CB-00123" || a.args[4] != ActionCreateMapField || a.args[5] != "nhom-mau/legal_form" {
		t.Errorf("audit actor/action/subject = %v/%v/%v", a.args[1], a.args[4], a.args[5])
	}
	if got.ID != pinnedFieldID || !got.IsActive {
		t.Errorf("returned row = %+v", got)
	}
}

func TestCreateMapFieldRefusalsCommitNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		d   func() *fieldDB // a builder: fieldDB holds a mutex and must not be copied
		err error
	}{
		// The type must be a LIVE row of THIS commune's catalogue — every commune's is empty today.
		"type not in catalogue": {func() *fieldDB { return &fieldDB{typeLive: false} }, commsstore.ErrAssetTypeMissing},
		"ceiling reached": {func() *fieldDB {
			return &fieldDB{typeLive: true, liveCount: commsstore.MapFieldSchemaCeiling}
		}, commsstore.ErrMapFieldSchemaFull},
		"key live": {func() *fieldDB { return &fieldDB{typeLive: true, keyLive: 1} }, commsstore.ErrFieldCodeTaken},
		// A retired key is never reissued, and says so in its own error rather than a 500.
		"key retired": {func() *fieldDB { return &fieldDB{typeLive: true, keyRetired: 1} }, commsstore.ErrFieldCodeRetired},
	} {
		t.Run(name, func(t *testing.T) {
			d := tc.d()
			uc, ctx := newFieldUseCase(t, d)
			_, err := uc.Create(ctx, validCreate(), staffActor)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if len(d.with("INSERT")) != 0 {
				t.Error("a refusal still wrote a row or an entry")
			}
			if d.committed != 0 || d.rolledBack != 1 {
				t.Errorf("committed/rolled back = %d/%d, want 0/1", d.committed, d.rolledBack)
			}
		})
	}
}

func TestCreateMapFieldShapeRefusedBeforeAnyTransaction(t *testing.T) {
	for name, mutate := range map[string]func(*CreateMapFieldRequest){
		"chon without options": func(r *CreateMapFieldRequest) { r.Options = nil },
		"options on text":      func(r *CreateMapFieldRequest) { r.ValueType = domain.ValueTypeText },
		"unknown value type":   func(r *CreateMapFieldRequest) { r.ValueType = "text" },
		"bad field key":        func(r *CreateMapFieldRequest) { r.FieldCode = "Legal-Form" },
		"bad type code":        func(r *CreateMapFieldRequest) { r.AssetTypeCode = "Nhom Mau" },
		"empty label":          func(r *CreateMapFieldRequest) { r.Label = " " },
		"negative sort order":  func(r *CreateMapFieldRequest) { r.SortOrder = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			d := &fieldDB{typeLive: true}
			uc, ctx := newFieldUseCase(t, d)
			req := validCreate()
			mutate(&req)
			if _, err := uc.Create(ctx, req, staffActor); err == nil {
				t.Fatal("accepted")
			}
			if d.begun != 0 {
				t.Error("a request refused on its shape still opened a transaction")
			}
		})
	}
}

func TestCreateMapFieldAuditFailureRollsBackTheRow(t *testing.T) {
	// Rule 6, invariant 3: the entry fails, so the row must not exist either.
	d := &fieldDB{typeLive: true, failOnContain: "INSERT INTO audit_log"}
	uc, ctx := newFieldUseCase(t, d)
	if _, err := uc.Create(ctx, validCreate(), staffActor); err == nil {
		t.Fatal("audit write failed and Create still succeeded")
	}
	if len(d.with("INSERT INTO map_field_schema")) != 1 {
		t.Fatal("the row insert should have run before the failing entry")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("committed/rolled back = %d/%d, want 0/1", d.committed, d.rolledBack)
	}
}

// --- update -----------------------------------------------------------------------------------

func TestUpdateMapFieldRefusesOptionRemoval(t *testing.T) {
	d := &fieldDB{row: choiceRow()}
	uc, ctx := newFieldUseCase(t, d)
	opts := []domain.FieldOption{{Value: "tnhh", Label: "TNHH"}}

	_, err := uc.Update(ctx, "mf-001", UpdateMapFieldRequest{Options: &opts}, staffActor)
	if !errors.Is(err, domain.ErrOptionRemoved) {
		t.Fatalf("err = %v, want ErrOptionRemoved", err)
	}
	// "UPDATE map_field_schema", not "UPDATE": the locking read carries FOR UPDATE.
	if len(d.with("UPDATE map_field_schema")) != 0 || len(d.with("audit_log")) != 0 {
		t.Error("refused edit still wrote")
	}
}

func TestUpdateMapFieldRefusesOptionsOnNonChoiceRow(t *testing.T) {
	r := choiceRow()
	r.valueType, r.options = domain.ValueTypeDecimal, `[]`
	d := &fieldDB{row: r}
	uc, ctx := newFieldUseCase(t, d)
	opts := []domain.FieldOption{{Value: "a", Label: "A"}}

	_, err := uc.Update(ctx, "mf-001", UpdateMapFieldRequest{Options: &opts}, staffActor)
	if !errors.Is(err, domain.ErrOptionsNotAllowed) {
		t.Fatalf("err = %v, want ErrOptionsNotAllowed", err)
	}
}

func TestUpdateMapFieldRelabelAndAppendIsWrittenAndAudited(t *testing.T) {
	d := &fieldDB{row: choiceRow()}
	uc, ctx := newFieldUseCase(t, d)
	opts := []domain.FieldOption{
		{Value: "tnhh", Label: "TNHH"}, {Value: "cp", Label: "Cổ phần"}, {Value: "hkd", Label: "Hộ kinh doanh"},
	}
	req := true
	got, err := uc.Update(ctx, "mf-001", UpdateMapFieldRequest{Options: &opts, IsRequired: &req}, staffActor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	up := d.with("UPDATE map_field_schema")
	if len(up) != 1 || up[0].args[0] != string(xaA) {
		t.Fatalf("UPDATE statements = %+v", up)
	}
	if s, _ := up[0].args[3].(string); !strings.Contains(s, `"hkd"`) {
		t.Errorf("options written = %v", up[0].args[3])
	}
	// No UPDATE may mention the immutable columns.
	for _, col := range []string{"asset_type_code", "field_code", "value_type"} {
		if strings.Contains(up[0].sql, col) {
			t.Errorf("UPDATE mentions immutable column %s: %s", col, up[0].sql)
		}
	}
	entries := d.with("INSERT INTO audit_log")
	if len(entries) != 1 || entries[0].args[4] != ActionUpdateMapField {
		t.Fatalf("entries = %+v", entries)
	}
	var delta map[string]map[string]any
	if err := json.Unmarshal(entries[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	// Only what moved (rule 6, invariant 5): label, sort_order, is_active untouched.
	if _, ok := delta["sau"]["label"]; ok {
		t.Error("delta carries an unchanged field")
	}
	if delta["truoc"]["is_required"] != false || delta["sau"]["is_required"] != true {
		t.Errorf("is_required delta = %v -> %v", delta["truoc"]["is_required"], delta["sau"]["is_required"])
	}
	if !got.IsRequired || len(got.Options) != 3 {
		t.Errorf("returned = %+v", got)
	}
}

func TestUpdateMapFieldNoOpWritesAndAuditsNothing(t *testing.T) {
	d := &fieldDB{row: choiceRow()}
	uc, ctx := newFieldUseCase(t, d)
	label, active := "Loại hình", true
	if _, err := uc.Update(ctx, "mf-001", UpdateMapFieldRequest{Label: &label, IsActive: &active}, staffActor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(d.with("UPDATE map_field_schema")) != 0 || len(d.with("audit_log")) != 0 {
		t.Error("a no-op edit wrote a row or an entry — the route's idem.KhongCan claim would be false")
	}
}

func TestUpdateMapFieldDisableKeepsRowListed(t *testing.T) {
	// `Tắt` is is_active = false through the ordinary UPDATE — not a soft delete.
	d := &fieldDB{row: choiceRow()}
	uc, ctx := newFieldUseCase(t, d)
	off := false
	if _, err := uc.Update(ctx, "mf-001", UpdateMapFieldRequest{IsActive: &off}, staffActor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	up := d.with("UPDATE map_field_schema")
	if len(up) != 1 || strings.Contains(up[0].sql, "deleted_at = now()") {
		t.Fatalf("disable must be a plain UPDATE, got %+v", up)
	}
	if up[0].args[6] != false {
		t.Errorf("is_active written = %v, want false", up[0].args[6])
	}
}

func TestUpdateMapFieldNotFound(t *testing.T) {
	d := &fieldDB{}
	uc, ctx := newFieldUseCase(t, d)
	label := "X"
	_, err := uc.Update(ctx, "mf-404", UpdateMapFieldRequest{Label: &label}, staffActor)
	if !errors.Is(err, commsstore.ErrMapFieldSchemaNotFound) {
		t.Fatalf("err = %v, want ErrMapFieldSchemaNotFound", err)
	}
}

// --- delete -----------------------------------------------------------------------------------

func TestDeleteMapFieldSoftDeletesWithBusinessCode(t *testing.T) {
	d := &fieldDB{row: choiceRow()}
	uc, ctx := newFieldUseCase(t, d)
	if err := uc.Delete(ctx, "mf-001", " không dùng nữa ", staffActor); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	del := d.with("SET deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("%d soft deletes, want 1", len(del))
	}
	// deleted_by is the BUSINESS CODE (rule 6, invariant 8), the reason is trimmed.
	if del[0].args[0] != string(xaA) || del[0].args[2] != "CB-00123" || del[0].args[3] != "không dùng nữa" {
		t.Errorf("soft delete args = %v", del[0].args)
	}
	if len(d.with("DELETE")) != 0 {
		t.Error("a hard DELETE was issued")
	}
	entries := d.with("INSERT INTO audit_log")
	if len(entries) != 1 || entries[0].args[4] != ActionDeleteMapField || entries[0].args[5] != "nhom-mau/legal_form" {
		t.Fatalf("entries = %+v", entries)
	}
	if d.committed != 1 {
		t.Errorf("committed = %d, want 1", d.committed)
	}
}

func TestDeleteMapFieldRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		row    *fieldRow
		reason string
		actor  audit.Actor
		err    error
	}{
		"no reason": {choiceRow(), "  ", staffActor, domain.ErrThieuLyDoXoa},
		"no actor":  {choiceRow(), "lý do", audit.Actor{}, ErrMissingDeleter},
		"not found": {nil, "lý do", staffActor, commsstore.ErrMapFieldSchemaNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			d := &fieldDB{row: tc.row}
			uc, ctx := newFieldUseCase(t, d)
			if err := uc.Delete(ctx, "mf-001", tc.reason, tc.actor); !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if len(d.with("deleted_at = now()")) != 0 || len(d.with("audit_log")) != 0 {
				t.Error("refused delete still wrote")
			}
		})
	}
}
