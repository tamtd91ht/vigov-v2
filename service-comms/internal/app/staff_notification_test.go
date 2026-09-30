package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// THE BELL INBOX, the REAL use case over the REAL store over a recording fake driver — the same
// argument driver_gia_danh_muc_test.go makes: the properties worth proving (one transaction, an entry
// inside it, nothing written on a refusal, the recipient filter on every statement) are properties of
// the SQL and of the transaction boundaries, which a fake store would erase.
//
// NOT PROVED HERE: anything PostgreSQL does — the UNIQUE key that makes ON CONFLICT mean something,
// the CHECKs, the trigger. The fake reports whatever row count a test tells it to.

// --- a small recording driver ---------------------------------------------------------------------

type sqlStmt struct {
	sql  string
	args []driver.Value
}

type sqlFake struct {
	mu                        sync.Mutex
	stmts                     []sqlStmt
	begun, commits, rollbacks int

	// exec answers RowsAffected for a write; nil = 1.
	exec func(q string, args []driver.Value) (int64, error)
	// query answers a read; nil = error.
	query func(q string, args []driver.Value) ([]string, [][]driver.Value, error)
}

func (f *sqlFake) record(q string, args []driver.NamedValue) []driver.Value {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	f.mu.Lock()
	f.stmts = append(f.stmts, sqlStmt{sql: q, args: vals})
	f.mu.Unlock()
	return vals
}

func (f *sqlFake) with(sub string) []sqlStmt {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sqlStmt
	for _, s := range f.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (f *sqlFake) Connect(context.Context) (driver.Conn, error) { return &sqlFakeConn{f: f}, nil }
func (f *sqlFake) Driver() driver.Driver                        { return sqlFakeDriver{} }

type sqlFakeDriver struct{}

func (sqlFakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake: connector only")
}

type sqlFakeConn struct{ f *sqlFake }

func (c *sqlFakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake: no Prepare")
}
func (c *sqlFakeConn) Close() error { return nil }
func (c *sqlFakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *sqlFakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.f.mu.Lock()
	c.f.begun++
	c.f.mu.Unlock()
	return &sqlFakeTx{f: c.f}, nil
}

func (c *sqlFakeConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := c.f.record(q, args)
	if c.f.exec == nil {
		return driver.RowsAffected(1), nil
	}
	n, err := c.f.exec(q, vals)
	if err != nil {
		return nil, err
	}
	return driver.RowsAffected(n), nil
}

func (c *sqlFakeConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := c.f.record(q, args)
	if c.f.query == nil {
		return nil, fmt.Errorf("fake: no answer for %q", q)
	}
	cols, rows, err := c.f.query(q, vals)
	if err != nil {
		return nil, err
	}
	return &sqlFakeRows{cols: cols, rows: rows}, nil
}

type sqlFakeTx struct{ f *sqlFake }

func (t *sqlFakeTx) Commit() error {
	t.f.mu.Lock()
	t.f.commits++
	t.f.mu.Unlock()
	return nil
}

func (t *sqlFakeTx) Rollback() error {
	t.f.mu.Lock()
	t.f.rollbacks++
	t.f.mu.Unlock()
	return nil
}

type sqlFakeRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *sqlFakeRows) Columns() []string { return r.cols }
func (r *sqlFakeRows) Close() error      { return nil }
func (r *sqlFakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// --- fixtures -------------------------------------------------------------------------------------

var inboxClock = time.Date(2026, 9, 29, 1, 30, 0, 0, time.UTC)

const readerCode = "CB-00123"

func newInboxUseCase(t *testing.T, f *sqlFake) (*StaffNotifications, *docstore.StaffNotificationStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	st := docstore.NewStaffNotificationStore(kho)
	uc := NewStaffNotifications(kho, st)
	n := 0
	uc.newID = func() (string, error) { n++; return fmt.Sprintf("01JNOTICE%017d", n), nil }
	uc.now = func() time.Time { return inboxClock }
	return uc, st, tenant.Into(context.Background(), xaA)
}

var jobActor = audit.Actor{ID: audit.SystemActor, Kind: "system", IP: "10.1.2.3"}

// secretTitle and secretBody are text that must never reach the trail.
const (
	secretTitle = "TIEU-DE-KHONG-DUOC-VAO-VET"
	secretBody  = "THAN-KHONG-DUOC-VAO-VET"
)

func twoNotices() []domain.NotificationDelivery {
	return []domain.NotificationDelivery{
		{IdempotencyKey: "sla_reminders:due_soon:NHIEM_VU:2026-09-29", Kind: domain.StaffNotificationDueSoon,
			RecipientCodes: []string{"CB-001", "CB-002", "CB-001"}, Title: secretTitle, Body: secretBody,
			Link: "/nhiem-vu?soon=true"},
		{IdempotencyKey: "escalation:NHIEM_VU:01JREC:chairman", Kind: domain.StaffNotificationEscalation,
			RecipientCodes: []string{"CB-009"}, Title: secretTitle},
	}
}

// insertTuples counts the (id, code) tuples of one generated INSERT — "all created" by default.
func insertTuples(q string) int64 { return int64(strings.Count(q, "($1, $2, $3")) }

// --- Deliver --------------------------------------------------------------------------------------

func TestDeliver_WritesEveryNoticeAndOneAuditEntryInOneTransaction(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return insertTuples(q), nil
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)

	out, err := uc.Deliver(ctx, twoNotices(), jobActor)
	if err != nil {
		t.Fatal(err)
	}
	if f.begun != 1 || f.commits != 1 || f.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn đúng một giao dịch commit", f.begun, f.commits, f.rollbacks)
	}
	if len(out) != 2 || out[0].Created != 2 || out[0].AlreadyDelivered != 0 || out[1].Created != 1 {
		t.Fatalf("kết quả = %+v (người nhận trùng phải gộp: CB-001 hai lần là một)", out)
	}

	ins := f.with("INSERT INTO staff_notification")
	if len(ins) != 2 {
		t.Fatalf("%d câu chèn, muốn 2", len(ins))
	}
	for _, s := range ins {
		if !strings.Contains(s.sql, "ON CONFLICT (tenant_id, idempotency_key, recipient_code) DO NOTHING") {
			t.Error("câu chèn không bỏ qua cặp (khoá, người nhận) đã có — giao lại sẽ trùng")
		}
		if s.args[0] != string(xaA) {
			t.Errorf("$1 = %v, muốn xã của context", s.args[0])
		}
		if strings.Contains(s.sql, "read_at") || strings.Contains(s.sql, "deleted_at") {
			t.Error("câu chèn ghi read_at hoặc cột xoá mềm")
		}
	}

	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("%d dòng vết, muốn đúng 1 cho cả lời gọi", len(aud))
	}
	a := aud[0].args // tenant, actor_id, actor_kind, actor_ip, action, subject, at, delta
	if a[0] != string(xaA) || a[1] != audit.SystemActor || a[2] != "system" || a[3] != "10.1.2.3" {
		t.Errorf("vết mang xã/chủ thể sai: %v", a[:4])
	}
	if a[4] != ActionDeliverStaffNotifications || a[5] != "hop-thu-thong-bao/2026-09-29" {
		t.Errorf("hành vi/đối tượng = %v / %v", a[4], a[5])
	}
	delta := string(a[7].([]byte))
	for _, must := range []string{"sla_reminders:due_soon:NHIEM_VU:2026-09-29", "sap-den-han", "leo-thang",
		"CB-001", "CB-002", "CB-009", `"tao_moi":3`} {
		if !strings.Contains(delta, must) {
			t.Errorf("delta thiếu %q: %s", must, delta)
		}
	}
	for _, never := range []string{secretTitle, secretBody, "/nhiem-vu"} {
		if strings.Contains(delta, never) {
			t.Errorf("delta chứa %q — tiêu đề/thân/liên kết không bao giờ vào vết", never)
		}
	}
}

func TestDeliver_RedeliveryCreatesNothingAndFilesNoEntry(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return 0, nil // the unique key swallowed every pair
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)

	out, err := uc.Deliver(ctx, twoNotices(), jobActor)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Created != 0 || out[0].AlreadyDelivered != 2 || out[1].AlreadyDelivered != 1 {
		t.Fatalf("giao lại = %+v, muốn toàn bộ already_delivered", out)
	}
	if len(f.with("INSERT INTO audit_log")) != 0 {
		t.Error("giao lại không tạo gì mà vẫn ghi vết")
	}
}

func TestDeliver_PartialRedeliveryCountsBoth(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return insertTuples(q) - 1, nil
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	out, err := uc.Deliver(ctx, twoNotices()[:1], jobActor)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Created != 1 || out[0].AlreadyDelivered != 1 {
		t.Fatalf("= %+v", out)
	}
	if len(f.with("INSERT INTO audit_log")) != 1 {
		t.Error("có dòng mới mà không có vết")
	}
}

func TestDeliver_InvalidBatchWritesNothing(t *testing.T) {
	bad := twoNotices()
	bad[1].Link = "https://evil.example/x"
	f := &sqlFake{}
	uc, _, ctx := newInboxUseCase(t, f)
	_, err := uc.Deliver(ctx, bad, jobActor)
	if !errors.Is(err, domain.ErrInvalidDelivery) {
		t.Fatalf("lỗi = %v, muốn ErrInvalidDelivery", err)
	}
	if f.begun != 0 || len(f.stmts) != 0 {
		t.Errorf("lô sai mà đã mở giao dịch (%d) / chạy %d câu lệnh — tất cả hoặc không gì cả", f.begun, len(f.stmts))
	}
}

func TestDeliver_AuditFailureRollsBackTheInserts(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO audit_log") {
			return 0, errors.New("fake: audit write fails")
		}
		return insertTuples(q), nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	if _, err := uc.Deliver(ctx, twoNotices(), jobActor); err == nil {
		t.Fatal("vết hỏng mà lời gọi vẫn thành công")
	}
	if len(f.with("INSERT INTO staff_notification")) != 2 || f.rollbacks != 1 || f.commits != 0 {
		t.Errorf("chèn %d, rollback %d, commit %d — muốn các câu chèn đã chạy rồi bị cuộn lại cùng vết",
			len(f.with("INSERT INTO staff_notification")), f.rollbacks, f.commits)
	}
}

func TestDeliver_RefusesEmptyActor(t *testing.T) {
	f := &sqlFake{}
	uc, _, ctx := newInboxUseCase(t, f)
	if _, err := uc.Deliver(ctx, twoNotices(), audit.Actor{}); !errors.Is(err, ErrNoActor) {
		t.Fatalf("lỗi = %v, muốn ErrNoActor", err)
	}
	if f.begun != 0 {
		t.Error("không có chủ thể mà đã mở giao dịch")
	}
}

// --- the staff member's own inbox ---------------------------------------------------------------

func noticeColumns() []string {
	return []string{"id", "recipient_code", "idempotency_key", "kind", "title", "body", "link", "read_at", "created_at"}
}

func noticeRow(id string, readAt any) []driver.Value {
	return []driver.Value{id, readerCode, "sla_reminders:overdue:NHIEM_VU:01JREC:2026-09-29",
		domain.StaffNotificationOverdue, secretTitle, "", "/nhiem-vu/01JREC", readAt, inboxClock}
}

func inboxReader() audit.Actor { return audit.Actor{ID: readerCode, Kind: "staff", IP: "10.0.0.7"} }

func TestMarkRead_UnreadOwnNoticeIsMarkedAndAudited(t *testing.T) {
	f := &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FOR UPDATE") {
			return noticeColumns(), [][]driver.Value{noticeRow("tb-1", nil)}, nil
		}
		return nil, nil, errors.New("unexpected")
	}}
	uc, _, ctx := newInboxUseCase(t, f)

	n, err := uc.MarkRead(ctx, "tb-1", inboxReader())
	if err != nil {
		t.Fatal(err)
	}
	if !n.Read() || !n.ReadAt.Equal(inboxClock) {
		t.Errorf("read_at = %v", n.ReadAt)
	}
	lock := f.with("FOR UPDATE")[0]
	if !strings.Contains(lock.sql, "recipient_code = $3") || lock.args[2] != readerCode || lock.args[0] != string(xaA) {
		t.Errorf("câu khoá không lọc theo mã cán bộ của phiên: %q %v", lock.sql, lock.args)
	}
	upd := f.with("UPDATE staff_notification")
	if len(upd) != 1 || !strings.Contains(upd[0].sql, "read_at IS NULL") || upd[0].args[2] != readerCode {
		t.Fatalf("câu cập nhật = %+v", upd)
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || aud[0].args[1] != readerCode || aud[0].args[4] != ActionReadStaffNotification {
		t.Fatalf("vết = %+v", aud)
	}
	if strings.Contains(string(aud[0].args[7].([]byte)), secretTitle) {
		t.Error("tiêu đề vào vết")
	}
	if f.commits != 1 {
		t.Error("không commit")
	}
}

func TestMarkRead_AlreadyReadWritesNothing(t *testing.T) {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		return noticeColumns(), [][]driver.Value{noticeRow("tb-1", inboxClock.Add(-time.Minute))}, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	n, err := uc.MarkRead(ctx, "tb-1", inboxReader())
	if err != nil {
		t.Fatal(err)
	}
	if !n.ReadAt.Equal(inboxClock.Add(-time.Minute)) {
		t.Error("lần đọc đầu bị ghi đè")
	}
	if len(f.with("UPDATE staff_notification")) != 0 || len(f.with("INSERT INTO audit_log")) != 0 {
		t.Error("đánh dấu lại một thông báo đã đọc mà vẫn ghi")
	}
}

// Another person's notice: the lock carries the session's code, so PostgreSQL finds no row — the
// fake answers exactly that — and the use case says "not found", writing nothing.
func TestMarkRead_NoticeOfAnotherPersonIsNotFound(t *testing.T) {
	f := &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		if args[2] != "CB-00999" {
			return noticeColumns(), nil, nil
		}
		return noticeColumns(), [][]driver.Value{noticeRow("tb-colleague", nil)}, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	_, err := uc.MarkRead(ctx, "tb-colleague", inboxReader())
	if !errors.Is(err, docstore.ErrStaffNotificationNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrStaffNotificationNotFound", err)
	}
	if len(f.with("UPDATE staff_notification")) != 0 || len(f.with("INSERT INTO audit_log")) != 0 {
		t.Error("thông báo của người khác bị ghi")
	}
}

func TestMarkAllRead_AuditsOnlyWhenSomethingMoved(t *testing.T) {
	moved := int64(0)
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "UPDATE staff_notification") {
			return moved, nil
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)

	n, err := uc.MarkAllRead(ctx, inboxReader())
	if err != nil || n != 0 || len(f.with("audit_log")) != 0 {
		t.Fatalf("không có gì chưa đọc: n=%d err=%v, vết=%d", n, err, len(f.with("audit_log")))
	}

	moved = 3
	n, err = uc.MarkAllRead(ctx, inboxReader())
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	upd := f.with("UPDATE staff_notification")
	if !strings.Contains(upd[len(upd)-1].sql, "recipient_code = $2") || upd[len(upd)-1].args[1] != readerCode {
		t.Error("Đọc hết không lọc theo mã cán bộ của phiên")
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || aud[0].args[4] != ActionReadAllStaffNotifications ||
		!strings.Contains(string(aud[0].args[7].([]byte)), `"so_da_doc":3`) {
		t.Fatalf("vết = %+v", aud)
	}
}

// --- the store's reads, over the same driver -------------------------------------------------------

func TestStoreReadsFilterByRecipientAndLiveRows(t *testing.T) {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "count(*)") {
			return []string{"count"}, [][]driver.Value{{int64(4)}}, nil
		}
		return noticeColumns(), [][]driver.Value{noticeRow("tb-1", nil), noticeRow("tb-2", inboxClock)}, nil
	}}
	_, st, ctx := newInboxUseCase(t, f)

	req, err := page.Parse(nil, docstore.SortStaffNotifications)
	if err != nil {
		t.Fatal(err)
	}
	res, err := st.ListOwn(ctx, readerCode, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || res.Items[0].Read() || !res.Items[1].Read() || res.Items[0].Link != "/nhiem-vu/01JREC" {
		t.Fatalf("= %+v", res.Items)
	}
	n, err := st.CountUnread(ctx, readerCode)
	if err != nil || n != 4 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	for _, s := range f.stmts {
		if !strings.Contains(s.sql, "tenant_id = $1") || !strings.Contains(s.sql, "recipient_code = $2") ||
			!strings.Contains(s.sql, "deleted_at IS NULL") {
			t.Errorf("câu đọc thiếu bộ lọc xã / người nhận / xoá mềm: %q", s.sql)
		}
		if s.args[0] != string(xaA) || s.args[1] != readerCode {
			t.Errorf("tham số = %v", s.args)
		}
	}
	if !strings.Contains(f.with("count(*)")[0].sql, "read_at IS NULL") {
		t.Error("đếm chưa đọc không lọc read_at")
	}
	if !strings.Contains(f.with("ORDER BY")[0].sql, "ORDER BY created_at DESC, id DESC") {
		t.Errorf("thứ tự không toàn phần hoặc không mới nhất trước: %q", f.with("ORDER BY")[0].sql)
	}
}
