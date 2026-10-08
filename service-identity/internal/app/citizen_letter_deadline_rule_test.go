package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The citizen-letter deadline rules' use case over the REAL store over a fake database/sql driver —
// the driver_gia_sla_test.go argument: what is worth proving here is the SQL and the transaction
// boundary (rule rows and audit entry in ONE transaction; a refusal commits nothing; the UPDATE
// names `amount` and `unit` only; `deleted_by` is the staff code). What it does not prove is anything
// PostgreSQL enforces — the live-unique key, the CHECKs, FOR UPDATE serialising two administrators.

type ruleRowFake struct {
	id, letterType, kind string
	amount               int
	unit                 string
}

// ruleDBFake is the fake table plus the recording tape.
type ruleDBFake struct {
	mu   sync.Mutex
	log  []lenhGhi
	rows []ruleRowFake

	begun, committed, rolledBack int

	// failOn fails the FIRST statement containing this substring, with failWith when set.
	failOn   string
	failWith error
	failed   bool
}

func (f *ruleDBFake) record(q string, args []driver.NamedValue) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, lenhGhi{sql: q, args: giaTri(args)})
	if f.failOn != "" && !f.failed && strings.Contains(q, f.failOn) {
		f.failed = true
		if f.failWith != nil {
			return f.failWith
		}
		return errors.New("fake driver: this statement was built to fail")
	}
	return nil
}

func (f *ruleDBFake) statements(substr string) []lenhGhi {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []lenhGhi
	for _, l := range f.log {
		if strings.Contains(l.sql, substr) {
			out = append(out, l)
		}
	}
	return out
}

func (f *ruleDBFake) Connect(context.Context) (driver.Conn, error) { return &ruleConnFake{f: f}, nil }
func (f *ruleDBFake) Driver() driver.Driver                        { return trinhGia{} }

type ruleConnFake struct{ f *ruleDBFake }

func (c *ruleConnFake) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *ruleConnFake) Close() error { return nil }
func (c *ruleConnFake) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *ruleConnFake) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.f.mu.Lock()
	c.f.begun++
	c.f.mu.Unlock()
	return &ruleTxFake{f: c.f}, nil
}

func (c *ruleConnFake) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.f.record(q, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *ruleConnFake) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.f.record(q, args); err != nil {
		return nil, err
	}
	cols := []string{"id", "letter_type", "deadline_kind", "amount", "unit"}
	// FOR UPDATE reads one row by id — $2, taken from the bound parameter, never from the fixture.
	var id string
	if len(args) >= 2 {
		id, _ = args[1].Value.(string)
	}
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	for _, r := range c.f.rows {
		if r.id == id {
			return &rowsGia{cot: cols, hang: [][]driver.Value{{r.id, r.letterType, r.kind, int64(r.amount), r.unit}}}, nil
		}
	}
	return &rowsGia{cot: cols}, nil
}

type ruleTxFake struct{ f *ruleDBFake }

func (t *ruleTxFake) Commit() error {
	t.f.mu.Lock()
	t.f.committed++
	t.f.mu.Unlock()
	return nil
}

func (t *ruleTxFake) Rollback() error {
	t.f.mu.Lock()
	t.f.rolledBack++
	t.f.mu.Unlock()
	return nil
}

const ruleIDFixture = "01JRULEKHIEUNAIGIAIQUYET00"

func newRuleUseCase(t *testing.T, f *ruleDBFake) (*CitizenLetterDeadlineRules, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	scoped := store.New(db)
	uc := NewCitizenLetterDeadlineRules(scoped, idstore.NewCitizenLetterDeadlineRuleStore(scoped))
	uc.newID = func() (string, error) { return ruleIDFixture, nil }
	return uc, tenant.Into(context.Background(), xaSLA)
}

func complaintResolution(amount int) CreateCitizenLetterDeadlineRuleRequest {
	return CreateCitizenLetterDeadlineRuleRequest{LetterType: domain.CitizenLetterKhieuNai,
		Kind: domain.CitizenLetterResolution, Amount: amount, Unit: domain.UnitCalendarDays}
}

// THE ROW AND ITS AUDIT ENTRY IN ONE TRANSACTION; the entry names the STAFF CODE and the commitment.
func TestCreateCitizenLetterRuleInsertsAndAuditsInOneTx(t *testing.T) {
	f := &ruleDBFake{}
	uc, ctx := newRuleUseCase(t, f)

	r, err := uc.Create(ctx, complaintResolution(30), nguoiSLA())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID != ruleIDFixture {
		t.Errorf("id = %q", r.ID)
	}
	ins := f.statements("INSERT INTO citizen_letter_deadline_rule")
	if len(ins) != 1 {
		t.Fatalf("%d INSERT, muốn 1", len(ins))
	}
	a := ins[0].args
	if a[0] != string(xaSLA) || a[2] != "khieu-nai" || a[3] != "giai-quyet" || a[4] != int64(30) || a[5] != "ngay-lich" {
		t.Errorf("INSERT args = %v", a)
	}
	if f.begun != 1 || f.committed != 1 || f.rolledBack != 0 {
		t.Errorf("tx: %d/%d/%d, muốn 1/1/0", f.begun, f.committed, f.rolledBack)
	}
	entries := f.statements("audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d vết, muốn 1", len(entries))
	}
	v := entries[0].args
	if v[1] != maCanBoSLA || v[4] != ActionAddCitizenLetterDeadlineRule || v[5] != "khieu-nai/giai-quyet" {
		t.Errorf("vết = actor %v action %v subject %v", v[1], v[4], v[5])
	}
	var delta map[string]map[string]any
	if err := json.Unmarshal(v[7].([]byte), &delta); err != nil || delta["sau"]["amount"] != float64(30) ||
		delta["sau"]["unit"] != "ngay-lich" {
		t.Errorf("delta = %s (%v)", v[7], err)
	}
}

// THE LOCK REFUSES BEFORE ANY STATEMENT — working hours on every cell, working days on a complaint.
func TestCreateCitizenLetterRuleLockRefusesBeforeWriting(t *testing.T) {
	for _, req := range []CreateCitizenLetterDeadlineRuleRequest{
		{LetterType: domain.CitizenLetterKhieuNai, Kind: domain.CitizenLetterProcessing, Amount: 10, Unit: domain.UnitWorkingDays},
		{LetterType: domain.CitizenLetterDeNghi, Kind: domain.CitizenLetterProcessing, Amount: 10, Unit: domain.UnitWorkingHours},
		{LetterType: domain.CitizenLetterDeNghi, Kind: domain.CitizenLetterResolution, Amount: 10, Unit: domain.UnitWorkingDays},
		{LetterType: domain.CitizenLetterToCao, Kind: domain.CitizenLetterResolution, Amount: 0, Unit: domain.UnitCalendarDays},
	} {
		f := &ruleDBFake{}
		uc, ctx := newRuleUseCase(t, f)
		_, err := uc.Create(ctx, req, nguoiSLA())
		if !IsCitizenLetterDeadlineRuleInputError(err) {
			t.Errorf("%+v: lỗi %v — muốn lỗi đầu vào", req, err)
		}
		if len(f.log) != 0 || f.begun != 0 {
			t.Errorf("%+v: %d câu lệnh, %d giao dịch — muốn không có gì", req, len(f.log), f.begun)
		}
	}
}

// THE AUDIT ENTRY FAILING TAKES THE INSERT DOWN WITH IT (rule 6, forbidden #2).
func TestCreateCitizenLetterRuleAuditFailureRollsBack(t *testing.T) {
	f := &ruleDBFake{failOn: "audit_log"}
	uc, ctx := newRuleUseCase(t, f)
	if _, err := uc.Create(ctx, complaintResolution(30), nguoiSLA()); err == nil {
		t.Fatal("vết hỏng mà vẫn thành công")
	}
	if f.committed != 0 || f.rolledBack != 1 {
		t.Errorf("commit %d rollback %d — muốn 0/1", f.committed, f.rolledBack)
	}
}

// The partition's unique-index name, as PostgreSQL reports it, is the 409 sentinel.
func TestCreateCitizenLetterRuleDuplicateIsExists(t *testing.T) {
	f := &ruleDBFake{failOn: "INSERT INTO citizen_letter_deadline_rule",
		failWith: errors.New(`ERROR: duplicate key value violates unique constraint "citizen_letter_deadline_rule_p07_tenant_id_letter_type_deadline_kind_live_k" (SQLSTATE 23505)`)}
	uc, ctx := newRuleUseCase(t, f)
	if _, err := uc.Create(ctx, complaintResolution(30), nguoiSLA()); !errors.Is(err, ErrCitizenLetterDeadlineRuleExists) {
		t.Errorf("lỗi %v, muốn ErrCitizenLetterDeadlineRuleExists", err)
	}
}

// An actor without a staff code is refused before anything is written (rule 6, invariant 8).
func TestCitizenLetterRuleRefusesActorWithoutStaffCode(t *testing.T) {
	f := &ruleDBFake{}
	uc, ctx := newRuleUseCase(t, f)
	actor := nguoiSLA()
	actor.Vet.ID = ""
	if _, err := uc.Create(ctx, complaintResolution(30), actor); err == nil {
		t.Fatal("không có mã cán bộ mà vẫn ghi")
	}
	if len(f.log) != 0 {
		t.Errorf("%d câu lệnh đã chạy", len(f.log))
	}
}

func storedComplaintRule(unit string) []ruleRowFake {
	return []ruleRowFake{{id: ruleIDFixture, letterType: "khieu-nai", kind: "giai-quyet", amount: 30, unit: unit}}
}

// THE UPDATE NAMES amount AND unit ONLY, before/after in the entry, one transaction.
func TestUpdateCitizenLetterRuleWritesAmountAndAudits(t *testing.T) {
	f := &ruleDBFake{rows: storedComplaintRule("ngay-lich")}
	uc, ctx := newRuleUseCase(t, f)
	amount := 45
	r, err := uc.Update(ctx, ruleIDFixture, UpdateCitizenLetterDeadlineRuleRequest{Amount: &amount}, nguoiSLA())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if r.Amount != 45 || r.Unit != domain.UnitCalendarDays || r.LetterType != domain.CitizenLetterKhieuNai {
		t.Errorf("quy tắc = %+v", r)
	}
	upd := f.statements("UPDATE citizen_letter_deadline_rule SET amount")
	if len(upd) != 1 || upd[0].args[2] != int64(45) || upd[0].args[3] != "ngay-lich" {
		t.Fatalf("UPDATE = %+v", upd)
	}
	if strings.Contains(upd[0].sql, "letter_type =") || strings.Contains(upd[0].sql, "deadline_kind =") {
		t.Errorf("UPDATE đổi được loại đơn/loại hạn: %s", upd[0].sql)
	}
	entries := f.statements("audit_log")
	if len(entries) != 1 || entries[0].args[4] != ActionEditCitizenLetterDeadlineRule {
		t.Fatalf("vết = %+v", entries)
	}
	if !strings.Contains(string(entries[0].args[7].([]byte)), `"truoc":{"amount":30`) {
		t.Errorf("delta thiếu giá trị trước: %s", entries[0].args[7])
	}
	if f.committed != 1 {
		t.Errorf("commit %d", f.committed)
	}
}

// A NO-OP WRITES NOTHING AND AUDITS NOTHING.
func TestUpdateCitizenLetterRuleNoOpWritesNothing(t *testing.T) {
	f := &ruleDBFake{rows: storedComplaintRule("ngay-lich")}
	uc, ctx := newRuleUseCase(t, f)
	amount := 30
	if _, err := uc.Update(ctx, ruleIDFixture, UpdateCitizenLetterDeadlineRuleRequest{Amount: &amount}, nguoiSLA()); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(f.statements("UPDATE citizen_letter_deadline_rule")) != 0 || len(f.statements("audit_log")) != 0 {
		t.Errorf("không đổi gì mà vẫn ghi: %+v", f.log)
	}
}

// A STORED ROW THAT VIOLATES THE LOCK CANNOT BE COMMITTED AGAIN by an edit of its number — the result
// is validated, not the request.
func TestUpdateCitizenLetterRuleValidatesTheResult(t *testing.T) {
	f := &ruleDBFake{rows: storedComplaintRule("ngay-lam-viec")}
	uc, ctx := newRuleUseCase(t, f)
	amount := 45
	_, err := uc.Update(ctx, ruleIDFixture, UpdateCitizenLetterDeadlineRuleRequest{Amount: &amount}, nguoiSLA())
	if !errors.Is(err, domain.ErrCitizenLetterDeadlineUnitLocked) {
		t.Fatalf("lỗi %v, muốn khoá đơn vị", err)
	}
	if len(f.statements("UPDATE citizen_letter_deadline_rule")) != 0 || f.committed != 0 {
		t.Errorf("đã ghi một dòng vi phạm khoá đơn vị")
	}
	// Fixing the unit in the same edit is accepted.
	f2 := &ruleDBFake{rows: storedComplaintRule("ngay-lam-viec")}
	uc2, ctx2 := newRuleUseCase(t, f2)
	unit := domain.UnitCalendarDays
	if _, err := uc2.Update(ctx2, ruleIDFixture, UpdateCitizenLetterDeadlineRuleRequest{Amount: &amount, Unit: &unit}, nguoiSLA()); err != nil {
		t.Errorf("sửa cả đơn vị bị từ chối: %v", err)
	}
}

func TestUpdateCitizenLetterRuleUnknownIDIsNotFound(t *testing.T) {
	f := &ruleDBFake{}
	uc, ctx := newRuleUseCase(t, f)
	amount := 10
	_, err := uc.Update(ctx, "01JKHONGCO000000000000000X", UpdateCitizenLetterDeadlineRuleRequest{Amount: &amount}, nguoiSLA())
	if !errors.Is(err, idstore.ErrCitizenLetterDeadlineRuleNotFound) {
		t.Errorf("lỗi %v", err)
	}
}

// REMOVE: three soft-delete columns, deleted_by = STAFF CODE, reason in the entry, one transaction.
func TestRemoveCitizenLetterRuleSoftDeletesWithStaffCode(t *testing.T) {
	f := &ruleDBFake{rows: storedComplaintRule("ngay-lich")}
	uc, ctx := newRuleUseCase(t, f)
	if err := uc.Remove(ctx, ruleIDFixture, "  Chờ pháp chế đối chiếu  ", nguoiSLA()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	del := f.statements("SET deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("%d câu xoá mềm", len(del))
	}
	if del[0].args[2] != maCanBoSLA || del[0].args[3] != "Chờ pháp chế đối chiếu" {
		t.Errorf("deleted_by %v, reason %v — muốn mã cán bộ và lý do đã cắt khoảng trắng", del[0].args[2], del[0].args[3])
	}
	if len(f.statements("DELETE")) != 0 {
		t.Error("có câu DELETE — luật 7")
	}
	entries := f.statements("audit_log")
	if len(entries) != 1 || entries[0].args[4] != ActionRemoveCitizenLetterDeadlineRule ||
		!strings.Contains(string(entries[0].args[7].([]byte)), "Chờ pháp chế đối chiếu") {
		t.Errorf("vết = %+v", entries)
	}
	if f.committed != 1 {
		t.Errorf("commit %d", f.committed)
	}
}

func TestRemoveCitizenLetterRuleNeedsReason(t *testing.T) {
	f := &ruleDBFake{rows: storedComplaintRule("ngay-lich")}
	uc, ctx := newRuleUseCase(t, f)
	if err := uc.Remove(ctx, ruleIDFixture, "  ", nguoiSLA()); !errors.Is(err, domain.ErrCitizenLetterDeleteReasonMissing) {
		t.Errorf("lỗi %v", err)
	}
	if len(f.log) != 0 {
		t.Errorf("%d câu lệnh đã chạy", len(f.log))
	}
}
