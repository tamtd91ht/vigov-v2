package app

// The use case behind POST /api/v1/announcements, run over the REAL store over a FAKE DRIVER.
//
// WHY A DRIVER AND NOT A FAKE STORE — the same argument fake_driver_catalogue_test.go makes, and it
// holds harder here because this act has TWO writes plus a trail. What this package is responsible
// for is not "the use case called a method", it is:
//
//	the announcement, its recipients AND the audit entry are in ONE transaction (rule 6, invariant 3)
//	a refusal writes NOTHING and commits nothing
//	the state and the author are decided here and never taken from the request
//	`trang_thai_thu` is `chua-gui` whatever the request asked for
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the CHECK constraints, not the immutability
// triggers of migration 0005, not the foreign keys, not partition routing. That half needs a real
// server; the pg suite in internal/store is where it lands the day a DSN exists.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// announcementDB records every statement and every transaction boundary. It answers no query, because
// this use case runs none — a fake that invented an answer to a query nobody makes would be a fake
// that keeps working after the code starts making one.
type announcementDB struct {
	stmts []recordedStmt

	begun, committed, rolledBack int

	// failOn fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET ERROR: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of the transaction — the
	// audit entry — takes both business writes down with it, and that needs everything before it to
	// have already run.
	failOn string
	failed bool
}

func (d *announcementDB) Connect(context.Context) (driver.Conn, error) {
	return &announcementConn{d: d}, nil
}
func (d *announcementDB) Driver() driver.Driver { return announcementDriverOpen{} }

type announcementDriverOpen struct{}

func (announcementDriverOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả thông báo: chỉ dùng Connector")
}

type announcementConn struct{ d *announcementDB }

func (c *announcementConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả thông báo: không hỗ trợ Prepare")
}
func (c *announcementConn) Close() error { return nil }

func (c *announcementConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *announcementConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.begun++
	return &announcementTx{d: c.d}, nil
}

func (c *announcementConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.stmts = append(c.d.stmts, recordedStmt{sql: q, args: vals})
	if c.d.failOn != "" && !c.d.failed && strings.Contains(q, c.d.failOn) {
		c.d.failed = true
		return nil, errors.New("driver giả thông báo: câu lệnh này được dựng để hỏng")
	}
	return driver.RowsAffected(1), nil
}

func (c *announcementConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("driver giả thông báo: use case này không truy vấn gì — " + q)
}

type announcementTx struct{ d *announcementDB }

func (t *announcementTx) Commit() error   { t.d.committed++; return nil }
func (t *announcementTx) Rollback() error { t.d.rolledBack++; return nil }

// with returns every recorded statement containing `sub`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (d *announcementDB) with(sub string) []recordedStmt {
	var out []recordedStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *announcementDB) ran(sub string) bool { return len(d.with(sub)) > 0 }

const (
	pinnedAnnouncementID = "01JTHONGBAOCODINH00000000"
	authorStaffCode      = "CB-2026-7K3M9Q"
)

var pinnedTime = time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC)

// newTestAnnouncements builds the REAL use case over the REAL store over the fake driver, with the
// id AND THE CLOCK pinned so assertions can name both.
func newTestAnnouncements(t *testing.T, d *announcementDB) (*Announcements, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	s := store.New(db)
	uc := NewAnnouncements(s, commsstore.NewAnnouncementStore(s))
	uc.newID = func() (string, error) { return pinnedAnnouncementID, nil }
	uc.now = func() time.Time { return pinnedTime }
	return uc, tenant.Into(context.Background(), tenantA)
}

func sampleAuthor() audit.Actor {
	return audit.Actor{ID: authorStaffCode, Kind: "staff", IP: "10.0.0.7"}
}

func sampleAnnouncementRequest() domain.CreateAnnouncementRequest {
	return domain.CreateAnnouncementRequest{
		Title:          "Mời họp giao ban tháng 9",
		Body:           "Kính mời các đồng chí dự họp.",
		RecipientCodes: []string{"CB-2026-AAAA11", "CB-2026-BBBB22"},
	}
}

// --- the happy path ---------------------------------------------------------------------------

func TestPublishWritesThreeStatementsInOneTransaction(t *testing.T) {
	// RULE 6, INVARIANT 3, AS A PROPERTY OF THE TRANSACTION BOUNDARIES RATHER THAN A CLAIM. The
	// announcement, the recipients and the trail are three statements; if any of them could land in
	// a different transaction, a commune could end up with a notice nobody can be asked about.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	a, err := uc.Publish(ctx, sampleAnnouncementRequest(), sampleAuthor())
	if err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}

	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	for _, want := range []string{"INSERT INTO thong_bao\n", "INSERT INTO thong_bao_nguoi_nhan", "INSERT INTO audit_log"} {
		if !d.ran(want) {
			t.Errorf("thiếu câu lệnh %q", want)
		}
	}
	if a.ID != pinnedAnnouncementID {
		t.Errorf("id = %q, muốn %q", a.ID, pinnedAnnouncementID)
	}
}

func TestPublishDecidesStatusAndAuthorHere(t *testing.T) {
	// The state and the author are NOT in the request (domain.CreateAnnouncementRequest has no field for
	// either). This asserts what actually reaches the INSERT, which is the only place a future
	// "small" change could reintroduce a client-chosen state.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	req := sampleAnnouncementRequest()
	req.EmailRequested = true // the checkbox was ticked
	if _, err := uc.Publish(ctx, req, sampleAuthor()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}

	ins := d.with("INSERT INTO thong_bao\n")
	if len(ins) != 1 {
		t.Fatalf("số câu chèn thông báo = %d, muốn 1", len(ins))
	}
	// $1 tenant, $2 id, $3 tiêu đề, $4 nội dung, $5 trạng thái, $6 ghim, $7 bắt buộc xác nhận,
	// $8 gửi thư, $9 người soạn, $10 phát hành lúc.
	args := ins[0].args
	if len(args) != 10 {
		t.Fatalf("số tham số = %d, muốn 10", len(args))
	}
	if args[0] != string(tenantA) {
		t.Errorf("xã = %v, muốn %v — xã đến từ ngữ cảnh, không từ yêu cầu", args[0], tenantA)
	}
	if args[4] != string(domain.AnnouncementPublished) {
		t.Errorf("trạng thái = %v, muốn da-phat-hanh", args[4])
	}
	if args[8] != authorStaffCode {
		t.Errorf("người soạn = %v, muốn mã cán bộ từ phiên (luật 6, bất biến 8)", args[8])
	}
	if args[9] != pinnedTime {
		t.Errorf("phát hành lúc = %v, muốn %v — một lần đọc đồng hồ cho cả hành vi", args[9], pinnedTime)
	}
	// `trang_thai_thu` IS NOT A PARAMETER OF THIS STATEMENT. The ticked checkbox is recorded in
	// `gui_thu_dien_tu`; the mail STATE stays at its default because nothing has sent anything.
	if strings.Contains(ins[0].sql, "trang_thai_thu") {
		t.Error("câu chèn không được nhận trang_thai_thu: đó là kết quả của một lần gửi chưa xảy ra")
	}
}

func TestPublishMarksEveryRecipientNamed(t *testing.T) {
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	if _, err := uc.Publish(ctx, sampleAnnouncementRequest(), sampleAuthor()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}
	ins := d.with("INSERT INTO thong_bao_nguoi_nhan")
	if len(ins) != 1 {
		t.Fatalf("số câu chèn người nhận = %d, muốn 1 — một câu cho cả danh sách", len(ins))
	}
	// $1 xã, $2 id thông báo, rồi từng cặp (mã, đích danh).
	args := ins[0].args
	if len(args) != 2+2*2 {
		t.Fatalf("số tham số = %d, muốn 6", len(args))
	}
	if args[2] != "CB-2026-AAAA11" || args[4] != "CB-2026-BBBB22" {
		t.Errorf("mã người nhận = %v, %v", args[2], args[4])
	}
	if args[3] != true || args[5] != true {
		t.Error("mọi người nhận trong lượt này phải là đích danh: không có đường nào khác vào danh sách")
	}
}

func TestPublishTrailUsesStaffCodeAndCarriesNoContent(t *testing.T) {
	// TWO RULES IN ONE ASSERTION, and both have a measured history in this repository:
	//   rule 6, invariant 8 — `actor_id` is `CB-…`, never a ULID. Six write paths got this wrong on
	//                          2026-09-22 and no test turned red.
	//   rule 3, forbidden #5 — the audit ledger is never deleted, so the title and the body (free
	//                          text that can name a person) must not enter it.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	if _, err := uc.Publish(ctx, sampleAnnouncementRequest(), sampleAuthor()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}
	trail := d.with("INSERT INTO audit_log")
	if len(trail) != 1 {
		t.Fatalf("số vết kiểm toán = %d, muốn 1", len(trail))
	}
	args := trail[0].args
	// $1 xã, $2 actor_id, $3 actor_kind, $4 actor_ip, $5 action, $6 subject, $7 at, $8 delta.
	if args[1] != authorStaffCode {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q", args[1], authorStaffCode)
	}
	if args[4] != ActionPublishAnnouncement {
		t.Errorf("action = %v, muốn %q", args[4], ActionPublishAnnouncement)
	}
	subject, _ := args[5].(string)
	if !strings.HasPrefix(subject, "thong-bao/2026-09-23/") || !strings.HasSuffix(subject, authorStaffCode) {
		t.Errorf("subject = %q, muốn thong-bao/<ngày>/<mã cán bộ>", subject)
	}
	if strings.Contains(subject, "Mời họp") {
		t.Error("tiêu đề lọt vào chủ đề vết kiểm toán — sổ này không bao giờ bị xoá (luật 3)")
	}
	delta, _ := args[7].([]byte)
	for _, banned := range []string{"Mời họp", "Kính mời"} {
		if strings.Contains(string(delta), banned) {
			t.Errorf("delta chứa %q — tiêu đề và nội dung không được vào sổ kiểm toán", banned)
		}
	}
	// What the delta MUST carry: who was told. Without it the entry cannot answer the question an
	// inspection opens with.
	if !strings.Contains(string(delta), "CB-2026-AAAA11") {
		t.Error("delta thiếu mã người nhận — vết không trả lời được ai đã được báo")
	}
}

// --- the refusals ------------------------------------------------------------------------------

func TestPublishToOrgUnitsWritesNothingAndOpensNoTransaction(t *testing.T) {
	// THE REFUSAL THIS PASS EXISTS TO MAKE LOUD, asserted as "nothing happened" rather than as an
	// error value: a version that recorded the departments and delivered to nobody would return the
	// same error here if the check were moved, and only the statement count catches that.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	req := sampleAnnouncementRequest()
	req.OrgUnitIDs = []string{"01JBOPHAN0000000000000000"}
	_, err := uc.Publish(ctx, req, sampleAuthor())
	if !errors.Is(err, ErrOrgUnitDeliveryUnavailable) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitDeliveryUnavailable", err)
	}
	if len(d.stmts) != 0 || d.begun != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh và mở %d giao dịch", len(d.stmts), d.begun)
	}
}

func TestPublishWithNoRecipientsIsRefused(t *testing.T) {
	// An announcement that reaches nobody is the one failure of this module that produces no error
	// anywhere: the card appears in the book and the commune is simply never told.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	req := sampleAnnouncementRequest()
	req.RecipientCodes = nil
	if _, err := uc.Publish(ctx, req, sampleAuthor()); !errors.Is(err, domain.ErrNoRecipients) {
		t.Fatalf("lỗi = %v, muốn ErrNoRecipients", err)
	}
	if len(d.stmts) != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh", len(d.stmts))
	}
}

func TestPublishOrgUnitRefusalComesBeforeNoRecipients(t *testing.T) {
	// ORDER OF THE TWO REFUSALS. An author who ticked only departments must be told the capability
	// is missing — not "you addressed this to nobody", which is true, useless, and sends them
	// looking for a mistake in their own form.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	req := sampleAnnouncementRequest()
	req.RecipientCodes = nil
	req.OrgUnitIDs = []string{"01JBOPHAN0000000000000000"}
	if _, err := uc.Publish(ctx, req, sampleAuthor()); !errors.Is(err, ErrOrgUnitDeliveryUnavailable) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitDeliveryUnavailable", err)
	}
}

func TestPublishWithoutAuthorCodeIsRefused(t *testing.T) {
	// `nguoi_soan_ma` with nothing in it is an announcement nobody can be asked about, and there is
	// NO FALLBACK to an internal id (rule 6, invariant 8).
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	if _, err := uc.Publish(ctx, sampleAnnouncementRequest(), audit.Actor{Kind: "staff", IP: "10.0.0.7"}); !errors.Is(err, ErrMissingAuthor) {
		t.Fatalf("lỗi = %v, muốn ErrMissingAuthor", err)
	}
	if len(d.stmts) != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh", len(d.stmts))
	}
}

func TestPublishTrailFailureRollsBackBothWrites(t *testing.T) {
	// THE INVARIANT THAT ONLY A TRANSACTION TEST CAN SHOW. If the trail fails, the announcement and
	// its recipients must go with it: rule 6, invariant 3 does not permit a business write whose
	// entry failed, and rule 7 would make the orphaned announcement permanent.
	d := &announcementDB{failOn: "INSERT INTO audit_log"}
	uc, ctx := newTestAnnouncements(t, d)

	if _, err := uc.Publish(ctx, sampleAnnouncementRequest(), sampleAuthor()); err == nil {
		t.Fatal("vết kiểm toán hỏng mà phát hành vẫn báo thành công")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", d.committed, d.rolledBack)
	}
}

func TestPublishEmptyTitleRefusedBeforeTransaction(t *testing.T) {
	// Shape first, outside the transaction. A request that fails its shape must never hold a
	// transaction open while doing so.
	d := &announcementDB{}
	uc, ctx := newTestAnnouncements(t, d)

	req := sampleAnnouncementRequest()
	req.Title = "   "
	if _, err := uc.Publish(ctx, req, sampleAuthor()); !errors.Is(err, domain.ErrAnnouncementTitleEmpty) {
		t.Fatalf("lỗi = %v, muốn ErrAnnouncementTitleEmpty", err)
	}
	if d.begun != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng", d.begun)
	}
}
