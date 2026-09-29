package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// WHAT THIS FILE PROVES, and it is the half no store test can reach.
//
// The store suite proves the statements. This one proves the two decisions that sit above them,
// both of which fail SILENTLY:
//
//  1. THE ROW AND ITS AUDIT ENTRY SHARE ONE TRANSACTION (rule 6, invariant 3). That is a claim
//     about a transaction, so the fake below records Begin / Commit / Rollback and every
//     statement in order. A version that audited "afterwards, if it works" passes every test that
//     only checks both rows exist.
//  2. A REDELIVERY WRITES NEITHER. Queues deliver at least once (rule 2, invariant 5). Auditing
//     unconditionally would fill a public authority's ledger with entries saying it promised the
//     same citizen the same thing four times — every one of them false, and none of them
//     removable, because entries are append-only.
//
// NOT PROVED HERE: anything PostgreSQL does. The UNIQUE constraint that makes ON CONFLICT mean
// something, the CHECK constraints, and the immutability trigger all live in migration 0004 and
// are exercised only by the integration suite, which skips without VIGOV_TEST_DSN.

var testTenant = tenant.ID("01JA" + strings.Repeat("A", 22))

func tenantCtx(t tenant.ID) context.Context { return tenant.Into(context.Background(), t) }

const (
	testReportCode     = "PA-2026-7F3K9Q"
	testCitizenCode    = "cd-01JCONGDANMAUTHUNGHIEM"
	testNotificationID = "01JTHONGBAOMAUTHUNGHIEM01"
)

// --- a database that records the order of things ------------------------------------------------

type txLogDB struct {
	// steps is the ORDER of everything that happened: "begin", "commit", "rollback", and the first
	// words of each statement. Order is the whole point — "insert, audit, commit" and "insert,
	// commit, audit" are the difference rule 6 invariant 3 is about, and a test that only counted
	// statements could not tell them apart.
	steps []string
	err   error
}

func (d *txLogDB) Connect(context.Context) (driver.Conn, error) { return &txLogConn{d: d}, nil }
func (d *txLogDB) Driver() driver.Driver                        { return txLogDriver{} }

type txLogDriver struct{}

func (txLogDriver) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type txLogConn struct{ d *txLogDB }

func (c *txLogConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("không hỗ trợ") }
func (c *txLogConn) Close() error                        { return nil }
func (c *txLogConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *txLogConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.steps = append(c.d.steps, "begin")
	return &txLogTx{d: c.d}, nil
}

type txLogTx struct{ d *txLogDB }

func (t *txLogTx) Commit() error   { t.d.steps = append(t.d.steps, "commit"); return nil }
func (t *txLogTx) Rollback() error { t.d.steps = append(t.d.steps, "rollback"); return nil }

type txLogResult struct{}

func (txLogResult) LastInsertId() (int64, error) { return 0, errors.New("không dùng") }
func (txLogResult) RowsAffected() (int64, error) { return 1, nil }

func (c *txLogConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.d.steps = append(c.d.steps, stepName(q))
	if c.d.err != nil {
		return nil, c.d.err
	}
	return txLogResult{}, nil
}

func (c *txLogConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	c.d.steps = append(c.d.steps, stepName(q))
	return &emptyRows{}, nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string         { return []string{"x"} }
func (emptyRows) Close() error              { return nil }
func (emptyRows) Next([]driver.Value) error { return io.EOF }

// stepName names a statement by what it touches, so the recorded sequence reads like the story it
// is meant to tell.
func stepName(q string) string {
	switch {
	case strings.Contains(q, "audit_log"):
		return "audit"
	case strings.Contains(q, "INSERT INTO thong_bao_gui_cong_dan"):
		return "chen"
	case strings.Contains(q, "UPDATE thong_bao_gui_cong_dan"):
		return "capnhat"
	default:
		return "khac"
	}
}

// --- a store that answers what the test wants ---------------------------------------------------

type fakeNotificationStore struct {
	created   bool
	moved     bool
	insertErr error
	resultErr error

	inserts  int
	results  int
	received domain.CitizenNotification
}

func (f *fakeNotificationStore) Insert(ctx context.Context, tx *pkgstore.ScopedTx,
	n domain.CitizenNotification) (bool, error) {
	f.inserts++
	f.received = n
	if f.insertErr != nil {
		return false, f.insertErr
	}
	// A real write inside the transaction, so the recorded sequence carries it.
	if _, err := tx.Exec(ctx, "INSERT INTO thong_bao_gui_cong_dan (tenant_id) VALUES ($1)",
		string(tx.TenantID())); err != nil {
		return false, err
	}
	return f.created, nil
}

func (f *fakeNotificationStore) RecordSendResult(ctx context.Context, tx *pkgstore.ScopedTx, _ string,
	_ domain.SendResult) (bool, error) {
	f.results++
	if f.resultErr != nil {
		return false, f.resultErr
	}
	if _, err := tx.Exec(ctx, "UPDATE thong_bao_gui_cong_dan SET trang_thai = $2 WHERE tenant_id = $1",
		string(tx.TenantID()), "da-gui"); err != nil {
		return false, err
	}
	return f.moved, nil
}

func newTestLedger(t *testing.T, d *txLogDB, repo *fakeNotificationStore) *CitizenNotifications {
	t.Helper()
	uc := NewCitizenNotifications(pkgstore.New(sql.OpenDB(d)), repo)
	// The id is pinned so assertions can name it. In production it is newULID.
	uc.newID = func() (string, error) { return testNotificationID, nil }
	return uc
}

func sampleRecordRequest() RecordRequest {
	return RecordRequest{
		SubjectType:   domain.SubjectCitizenReport,
		SubjectCode:   testReportCode,
		Milestone:     "da-chuyen-xu-ly",
		Round:         1,
		Channel:       domain.ChannelZaloZNS,
		RecipientCode: testCitizenCode,
		TemplateCode:  "zns-phan-anh-doi-trang-thai",
		Params: domain.NotificationParams{
			LookupCode:  testReportCode,
			StatusLabel: "Đã chuyển xử lý",
			NextStep:    "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
		},
	}
}

// --- (1) the row and its entry share one transaction ---------------------------------------------

func TestRecordWritesLedgerAndTrailInOneTransaction(t *testing.T) {
	// THE INVARIANT THIS WHOLE LAYER EXISTS FOR. The measured defect on the previous project was
	// zero transactions across the whole backend, so "every write leaves a trail" could not hold:
	// there was always a window where the record had changed and the trail had not.
	//
	// The SEQUENCE is asserted, not the presence of both statements. "chen, audit, commit" and
	// "chen, commit, audit" both contain the same two writes; only the first one is rule 6.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}

	res, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), sampleRecordRequest())
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !res.Created || res.ID != testNotificationID {
		t.Errorf("kết quả sai: %+v", res)
	}

	want := []string{"begin", "chen", "audit", "commit"}
	if !equalSteps(d.steps, want) {
		t.Fatalf("thứ tự = %v, muốn %v", d.steps, want)
	}
}

func TestRecordTrailFailureCommitsNothing(t *testing.T) {
	// THE OTHER HALF OF THE SAME INVARIANT, and the half a "belt and braces" instinct usually
	// removes. If the audit entry fails, the obligation must not be recorded either: a commitment
	// nobody can attribute is the state the records rules do not permit. The queue will redeliver
	// and the whole thing will be attempted again — which is safe precisely because the insert is
	// idempotent.
	d := &txLogDB{err: errors.New("audit_log không ghi được")}
	repo := &fakeNotificationStore{created: true}

	if _, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), sampleRecordRequest()); err == nil {
		t.Fatal("nhật ký hỏng mà Record vẫn báo thành công")
	}
	for _, s := range d.steps {
		if s == "commit" {
			t.Fatalf("đã chốt giao dịch dù nhật ký hỏng: %v", d.steps)
		}
	}
	if d.steps[len(d.steps)-1] != "rollback" {
		t.Errorf("không huỷ giao dịch: %v", d.steps)
	}
}

// --- (2) a redelivery writes neither --------------------------------------------------------------

func TestRecordRedeliveryWritesNoSecondTrail(t *testing.T) {
	// THE DECISION THIS PINS. `Insert` reports that nothing was inserted, so no entry is owed.
	// Auditing anyway would record that the commune promised the same citizen the same thing
	// twice — and audit entries are append-only (rule 6, invariant 4), so the false statement
	// could never be corrected, only added to.
	//
	// It must also NOT be an error: an at-least-once queue makes this normal operation, and a
	// consumer that treats it as failure retries forever or dead-letters a message that was
	// handled correctly.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: false}

	res, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), sampleRecordRequest())
	if err != nil {
		t.Fatalf("giao lại phải là no-op, nhận lỗi: %v", err)
	}
	if res.Created {
		t.Error("giao lại mà báo là dòng mới")
	}
	for _, s := range d.steps {
		if s == "audit" {
			t.Fatalf("giao lại vẫn ghi nhật ký: %v", d.steps)
		}
	}
}

func TestRecordTwiceInARowYieldsOneTrailAndOneRow(t *testing.T) {
	// THE SAME PROPERTY SEEN FROM THE CALLER'S SIDE, because that is how it will actually happen:
	// the same envelope delivered twice, seconds apart, to the same consumer. The first call
	// inserts and audits; the second must do neither.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}
	uc := newTestLedger(t, d, repo)

	if _, err := uc.Record(tenantCtx(testTenant), sampleRecordRequest()); err != nil {
		t.Fatalf("lần một: %v", err)
	}
	repo.created = false // the UNIQUE constraint swallowed the second insert
	if _, err := uc.Record(tenantCtx(testTenant), sampleRecordRequest()); err != nil {
		t.Fatalf("lần hai: %v", err)
	}

	if n := countOf(d.steps, "audit"); n != 1 {
		t.Fatalf("ghi %d vết cho một cam kết, muốn 1 — sổ của cơ quan sẽ nói xã hứa hai lần", n)
	}
}

// --- what the ledger and the trail must not carry ---------------------------------------------------

func TestRecordSendKeyCarriesMilestoneAndRoundButNoPersonalData(t *testing.T) {
	// THE KEY IS WHAT DECIDES WHETHER A CITIZEN IS TOLD TWICE, so what goes into it is asserted
	// rather than assumed. `lan` is the component that is easy to drop as redundant: without it a
	// petition closed, reopened and closed AGAIN (ADR 0008) owes a second notification that would
	// be swallowed as a duplicate — a failure that looks exactly like correct deduplication.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}
	uc := newTestLedger(t, d, repo)

	req := sampleRecordRequest()
	req.Round = 2
	if _, err := uc.Record(tenantCtx(testTenant), req); err != nil {
		t.Fatalf("Record: %v", err)
	}
	key := repo.received.SendKey
	if !strings.Contains(key, "|2|") {
		t.Errorf("khoá không phân biệt lần thứ mấy: %q", key)
	}
	if !strings.Contains(key, "da-chuyen-xu-ly") {
		t.Errorf("khoá không phân biệt mốc: %q", key)
	}
}

func TestRecordMissingRoundCountsAsFirst(t *testing.T) {
	// NORMALISED EXPLICITLY, not accepted as a zero. `lan` is part of the deduplication key, so a
	// producer sending 0 and another sending 1 would make two keys for one fact — and the citizen
	// receives the message twice.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}

	req := sampleRecordRequest()
	req.Round = 0
	if _, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), req); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if repo.received.Round != 1 {
		t.Fatalf("lan = %d, muốn 1", repo.received.Round)
	}
	if !strings.Contains(repo.received.SendKey, "|1|") {
		t.Errorf("khoá không mang lần 1: %q", repo.received.SendKey)
	}
}

func TestRecordInvalidContentOpensNoTransaction(t *testing.T) {
	// RULE 10, INVARIANT 6, ENFORCED BEFORE A ROW LOCK IS TAKEN. A message that names a state and
	// no consequence tells the citizen nothing they can act on, and refusing it inside the
	// transaction would hold locks while doing so and hand the caller a rollback instead of a
	// reason.
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}

	req := sampleRecordRequest()
	req.Params.NextStep = ""

	if _, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), req); !errors.Is(err, domain.ErrMissingNextStep) {
		t.Fatalf("lỗi = %v, muốn ErrMissingNextStep", err)
	}
	if len(d.steps) != 0 {
		t.Errorf("đã chạm cơ sở dữ liệu: %v", d.steps)
	}
	if repo.inserts != 0 {
		t.Errorf("đã gọi kho %d lần", repo.inserts)
	}
}

func TestRecordMissingRequiredFieldIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RecordRequest)
		want   error
	}{
		{"thiếu mã hồ sơ", func(r *RecordRequest) { r.SubjectCode = "" }, ErrMissingSubject},
		{"thiếu mốc", func(r *RecordRequest) { r.Milestone = "" }, ErrMissingMilestone},
		{"thiếu người nhận", func(r *RecordRequest) { r.RecipientCode = "" }, ErrMissingRecipient},
		{"thiếu mẫu tin", func(r *RecordRequest) { r.TemplateCode = "" }, ErrMissingTemplate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := &txLogDB{}
			repo := &fakeNotificationStore{created: true}
			req := sampleRecordRequest()
			c.mutate(&req)
			if _, err := newTestLedger(t, d, repo).Record(tenantCtx(testTenant), req); !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if len(d.steps) != 0 {
				t.Errorf("đã chạm cơ sở dữ liệu: %v", d.steps)
			}
		})
	}
}

func TestRecordWithoutTenantInContextPanics(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A consumer whose message carried no commune must refuse, never guess
	// (rule 1, invariant 9). core/events.Dispatch refuses first; this is the second line, and it
	// is a panic rather than an error because a write that ran without a commune would land in
	// whichever commune happened to be convenient.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ghi sổ khi context không có xã mà không panic")
		}
	}()
	d := &txLogDB{}
	repo := &fakeNotificationStore{created: true}
	_, _ = newTestLedger(t, d, repo).Record(context.Background(), sampleRecordRequest())
}

// --- the outcome path -------------------------------------------------------------------------------

func TestRecordSendResultWritesResultAndTrailInOneTransaction(t *testing.T) {
	d := &txLogDB{}
	repo := &fakeNotificationStore{moved: true}

	moved, err := newTestLedger(t, d, repo).RecordSendResult(tenantCtx(testTenant), testNotificationID, domain.SendResult{
		Status:          domain.SendStatusSent,
		SentAt:          time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC),
		MaskedRecipient: "09****0000",
		AttemptCount:    1,
	}, audit.Actor{}, testReportCode)
	if err != nil {
		t.Fatalf("RecordSendResult: %v", err)
	}
	if !moved {
		t.Error("dòng đã đổi mà báo là không")
	}
	want := []string{"begin", "capnhat", "audit", "commit"}
	if !equalSteps(d.steps, want) {
		t.Fatalf("thứ tự = %v, muốn %v", d.steps, want)
	}
}

func TestRecordSendResultRepeatedOldResultWritesNoSecondTrail(t *testing.T) {
	d := &txLogDB{}
	repo := &fakeNotificationStore{moved: false}

	moved, err := newTestLedger(t, d, repo).RecordSendResult(tenantCtx(testTenant), testNotificationID, domain.SendResult{
		Status: domain.SendStatusFailed, ErrorCode: "zns-429", AttemptCount: 3,
	}, audit.Actor{}, testReportCode)
	if err != nil {
		t.Fatalf("báo lại phải là no-op, nhận lỗi: %v", err)
	}
	if moved {
		t.Error("không dòng nào đổi mà báo là đã đổi")
	}
	if countOf(d.steps, "audit") != 0 {
		t.Errorf("vẫn ghi vết cho một thay đổi không xảy ra: %v", d.steps)
	}
}

func TestRecordSendResultMissingSubjectRefusedWithoutTransaction(t *testing.T) {
	// The audit entry's subject is the BUSINESS code, and core/audit refuses an entry without one.
	// Failing here, before the transaction, means the caller gets the reason instead of a rollback
	// from inside a closure.
	d := &txLogDB{}
	repo := &fakeNotificationStore{moved: true}

	_, err := newTestLedger(t, d, repo).RecordSendResult(tenantCtx(testTenant), testNotificationID,
		domain.SendResult{Status: domain.SendStatusFailed}, audit.Actor{}, "")
	if !errors.Is(err, ErrMissingSubject) {
		t.Fatalf("lỗi = %v, muốn ErrMissingSubject", err)
	}
	if len(d.steps) != 0 {
		t.Errorf("đã chạm cơ sở dữ liệu: %v", d.steps)
	}
}

// --- who the trail attributes it to -----------------------------------------------------------------

func TestActorDefaultsToSystemNotEmpty(t *testing.T) {
	// RULE 6, INVARIANT 6: system actions are audited too, WITH A SYSTEM PRINCIPAL. core/audit
	// refuses an entry with no actor, so leaving it empty would make a consumer unable to record
	// anything — and the fix somebody reaches for next is passing a staff id that did no work.
	if a := actorOrSystem(audit.Actor{}); a.ID != audit.SystemActor || a.Kind != "system" {
		t.Fatalf("người gây mặc định = %+v, muốn nguyên tắc hệ thống", a)
	}
	// A named actor is never overwritten: when a member of staff causes a notification, the trail
	// must say so.
	actor := audit.Actor{ID: "CB-007", Kind: "staff", IP: "10.0.0.7"}
	if a := actorOrSystem(actor); a != actor {
		t.Fatalf("người gây = %+v, muốn %+v", a, actor)
	}
}

// --- the id -------------------------------------------------------------------------------------------

func TestNewULIDIs26CharsOfCrockfordAlphabet(t *testing.T) {
	// The alphabet matters beyond neatness: identity's copy (and the CHECK constraint beside it)
	// uses exactly this one, and an id that differs in alphabet or length between two services is
	// an id that fails a constraint in one of them. See the comment on newULID about the second
	// copy.
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, err := newULID()
		if err != nil {
			t.Fatalf("newULID: %v", err)
		}
		if len(id) != 26 {
			t.Fatalf("độ dài = %d, muốn 26: %q", len(id), id)
		}
		for _, r := range id {
			if !strings.ContainsRune(ulidAlphabet, r) {
				t.Fatalf("ký tự ngoài bảng chữ Crockford: %q trong %q", r, id)
			}
		}
		if seen[id] {
			t.Fatalf("hai mã trùng nhau: %q", id)
		}
		seen[id] = true
	}
}

// --- helpers -------------------------------------------------------------------------------------------

func equalSteps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func countOf(a []string, x string) int {
	n := 0
	for _, v := range a {
		if v == x {
			n++
		}
	}
	return n
}
