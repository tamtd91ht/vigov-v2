package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Unit tests for the citizen notification ledger store. The fake driver and the list of what it
// can and cannot prove are in fake_write_driver_test.go.
//
// `testTenant` and `tenantCtx` come from map_asset_type_test.go. ONE way to put a commune into a
// context in this package, not two: a second copy is the one that ends up subtly different from
// what the edge really does.

const (
	testReportCode     = "PA-2026-7F3K9Q"
	testCitizenCode    = "cd-01JCONGDANMAUTHUNGHIEM"
	testNotificationID = "01JTHONGBAOMAUTHUNGHIEM01"
)

func testParams() domain.NotificationParams {
	return domain.NotificationParams{
		LookupCode:  testReportCode,
		StatusLabel: "Đã chuyển xử lý",
		NextStep:    "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
	}
}

func testNotification() domain.CitizenNotification {
	return domain.CitizenNotification{
		ID:            testNotificationID,
		SendKey:       "phieu-phan-anh|" + testReportCode + "|da-chuyen-xu-ly|1|" + testCitizenCode + "|zalo-zns",
		SubjectType:   domain.SubjectCitizenReport,
		SubjectCode:   testReportCode,
		Milestone:     "da-chuyen-xu-ly",
		Round:         1,
		Channel:       domain.ChannelZaloZNS,
		RecipientCode: testCitizenCode,
		TemplateCode:  "zns-phan-anh-doi-trang-thai",
		Params:        testParams(),
	}
}

// runInTx runs fn inside a real *store.ScopedTx built over the fake driver. THE TRANSACTION IS
// REAL EVEN THOUGH THE DATABASE IS NOT: rule 6, invariant 3 is a statement about a transaction, so
// a suite that never opens one cannot check anything about it.
func runInTx(t *testing.T, f *fakeWriteStore, tn tenant.ID,
	fn func(tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	ctx := tenantCtx(tn)
	return pkgstore.New(openFakeWriteDB(f)).For(ctx).Tx(ctx, fn)
}

func newTestCitizenNotificationStore(f *fakeWriteStore) *CitizenNotificationStore {
	return NewCitizenNotificationStore(pkgstore.New(openFakeWriteDB(f)))
}

// --- Insert: the obligation is recorded, once ----------------------------------------------------

func TestInsertBindsTenantFromContextNotFromArgument(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not a parameter of Insert and cannot become one:
	// it is read off the transaction, which took it from the context. If a caller could pass one,
	// a consumer handling a message could write into whichever commune the payload named — which
	// is exactly the door rule 1 forbidden #2 closes.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	var created bool
	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		var err error
		created, err = s.Insert(context.Background(), tx, testNotification())
		return err
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !created {
		t.Error("dòng mới mà báo là đã có")
	}
	if len(f.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(f.stmts))
	}
	st := f.stmts[0]
	if !strings.Contains(st.sql, "INSERT INTO thong_bao_gui_cong_dan") {
		t.Errorf("ghi nhầm bảng: %q", st.sql)
	}
	if len(st.args) == 0 || st.args[0] != string(testTenant) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", st.args, testTenant)
	}
}

func TestInsertLeavesDuplicateDecisionToDatabase(t *testing.T) {
	// THE IDEMPOTENCY IS STRUCTURAL, NOT CHECKED. A read-then-write would pass every
	// single-threaded test and send a citizen two messages under concurrency — the only condition
	// it ever fails under. What makes that impossible is this clause, so it is asserted literally.
	//
	// `DO NOTHING` AND NOT `DO UPDATE`: the conflicting row is the same fact, possibly already
	// delivered, and the trigger in migration 0004 refuses to move a delivered row — so `DO
	// UPDATE` would turn a harmless duplicate into a failed transaction that rolls back a
	// business write.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	if err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(context.Background(), tx, testNotification())
		return err
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	q := f.stmts[0].sql
	if !strings.Contains(q, "ON CONFLICT (tenant_id, khoa_lan_gui) DO NOTHING") {
		t.Fatalf("thiếu chống trùng ở tầng CSDL: %q", q)
	}
	if strings.Contains(q, "DO UPDATE") {
		t.Errorf("DO UPDATE sẽ khiến một lần giao lại ghi đè lên bản ghi đã gửi: %q", q)
	}
}

func TestInsertConflictReportsExistingNotError(t *testing.T) {
	// A REDELIVERY IS NORMAL OPERATION, NOT A FAILURE. Queues deliver at least once (rule 2,
	// invariant 5). Returning an error here would make a consumer retry forever, or give up and
	// dead-letter a message that was already handled correctly.
	//
	// The boolean is load-bearing: app.CitizenNotifications uses it to decide whether an audit entry is
	// owed, and auditing a redelivery would record a promise the commune never made twice.
	f := &fakeWriteStore{rowsAffected: 0}
	s := newTestCitizenNotificationStore(f)

	var created bool
	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		var err error
		created, err = s.Insert(context.Background(), tx, testNotification())
		return err
	})
	if err != nil {
		t.Fatalf("giao lại cùng một sự kiện phải là no-op, nhận lỗi: %v", err)
	}
	if created {
		t.Error("dòng đã có mà báo là mới — một bút toán nhật ký thứ hai sẽ được ghi")
	}
}

func TestInsertAlwaysBornPending(t *testing.T) {
	// `trang_thai` IS NOT A PARAMETER OF Insert, and this pins it. Letting a producer name the state
	// would let it insert a row already marked `da-gui` — a claim about the citizen's phone that
	// nothing in this service witnessed. A row is born owing a message; the send decides the rest.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	n := testNotification()
	n.Status = domain.SendStatusSent // a caller trying to skip ahead
	if err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(context.Background(), tx, n)
		return err
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	args := f.stmts[0].args
	last := args[len(args)-1]
	if last != string(domain.SendStatusPending) {
		t.Fatalf("trạng thái ghi vào = %v, muốn %q", last, domain.SendStatusPending)
	}
}

func TestInsertInvalidContentRunsNoStatement(t *testing.T) {
	// RULE 10, INVARIANT 6 ENFORCED BEFORE THE ROW EXISTS. A notification that names a state and
	// no consequence tells the citizen nothing they can act on, and once the row is in the ledger
	// the adapter will happily send it. The CHECK constraint on `tham_so` would catch the missing
	// key, but it arrives as a database error at the bottom of a transaction with no explanation
	// of which rule was broken.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	n := testNotification()
	n.Params.NextStep = ""

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(context.Background(), tx, n)
		return err
	})
	if !errors.Is(err, domain.ErrMissingNextStep) {
		t.Fatalf("lỗi = %v, muốn ErrMissingNextStep", err)
	}
	if len(f.stmts) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù nội dung không đạt", len(f.stmts))
	}
}

func TestInsertStoreErrorIsWrappedNotSwallowed(t *testing.T) {
	cause := errors.New("cơ sở dữ liệu không phản hồi")
	f := &fakeWriteStore{rowsAffected: 1, execErr: cause}
	s := newTestCitizenNotificationStore(f)

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.Insert(context.Background(), tx, testNotification())
		return err
	})
	if !errors.Is(err, cause) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, cause)
	}
	// The transaction must not have been committed: the row and its audit entry live or die
	// together (rule 6, invariant 3).
	if f.committed != 0 {
		t.Errorf("giao dịch đã chốt %d lần dù lệnh ghi hỏng", f.committed)
	}
	if f.rolledBack == 0 {
		t.Error("giao dịch không được huỷ sau lỗi ghi")
	}
}

// --- RecordSendResult: the outcome of a send ----------------------------------------------------------

func sentResult() domain.SendResult {
	return domain.SendResult{
		Status: domain.SendStatusSent,
		SentAt: time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC),
		// The agreed fake number, masked the way core/privacy.MaskPhone masks it.
		MaskedRecipient: "09****0000",
		AttemptCount:    1,
	}
}

func TestRecordSendResultExcludesSentAndSoftDeletedRows(t *testing.T) {
	// TWO PREDICATES, TWO DIFFERENT JOBS, both easy to drop:
	//
	//	trang_thai <> 'da-gui'  the idempotency. A second report of the same success matches no
	//	                        row and changes nothing, instead of hitting the trigger that
	//	                        refuses to move a delivered notification and failing the caller's
	//	                        whole transaction.
	//	deleted_at IS NULL      rule 7, invariant 2 — everywhere, always, including writes.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	if err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.RecordSendResult(context.Background(), tx, testNotificationID, sentResult())
		return err
	}); err != nil {
		t.Fatalf("RecordSendResult: %v", err)
	}
	q := f.stmts[0].sql
	if !strings.Contains(q, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không buộc xã: %q", q)
	}
	if !strings.Contains(q, "trang_thai <> 'da-gui'") {
		t.Errorf("thiếu điều kiện chống gửi lại: %q", q)
	}
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
}

func TestRecordSendResultAlreadySentChangesNothingAndIsNoError(t *testing.T) {
	// "Already delivered" is the expected outcome of an at-least-once queue, not an error. The
	// `false` is what stops app.CitizenNotifications writing a second audit entry saying the message went
	// out twice.
	f := &fakeWriteStore{rowsAffected: 0, rowExists: true}
	s := newTestCitizenNotificationStore(f)

	var moved bool
	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		var err error
		moved, err = s.RecordSendResult(context.Background(), tx, testNotificationID, sentResult())
		return err
	})
	if err != nil {
		t.Fatalf("báo lại cùng một kết quả phải là no-op, nhận lỗi: %v", err)
	}
	if moved {
		t.Error("không dòng nào đổi mà vẫn báo là đã đổi")
	}
}

func TestRecordSendResultNoRowReportsNotFound(t *testing.T) {
	// THE TWO ZERO-ROW CASES ARE SEPARATED BY A SECOND READ, NOT BY GUESSING, because they lead an
	// operator to opposite conclusions: "already sent" is nothing to do, "no such notification" is
	// a consumer addressing a record that is not there.
	f := &fakeWriteStore{rowsAffected: 0, rowExists: false}
	s := newTestCitizenNotificationStore(f)

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.RecordSendResult(context.Background(), tx, testNotificationID, sentResult())
		return err
	})
	if !errors.Is(err, ErrCitizenNotificationNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrCitizenNotificationNotFound", err)
	}
	// The probe is scoped to the commune as well: a row belonging to another commune must read as
	// absent, not as forbidden.
	probe := f.stmts[len(f.stmts)-1]
	if !strings.Contains(probe.sql, "tenant_id = $1") || probe.args[0] != string(testTenant) {
		t.Errorf("câu tra sự tồn tại không buộc xã: %q %v", probe.sql, probe.args)
	}
}

func TestRecordSendResultRefusesNonTerminalResult(t *testing.T) {
	// `cho-gui` is where a row STARTS. Writing it back as a RESULT would erase a real outcome and
	// put a delivered notification back into the retry set.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.RecordSendResult(context.Background(), tx, testNotificationID,
			domain.SendResult{Status: domain.SendStatusPending})
		return err
	})
	if !errors.Is(err, domain.ErrResultStatusNotTerminal) {
		t.Fatalf("lỗi = %v, muốn ErrResultStatusNotTerminal", err)
	}
	if len(f.stmts) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù kết quả không hợp lệ", len(f.stmts))
	}
}

func TestRecordSendResultRefusesUnmaskedRecipient(t *testing.T) {
	// THE ONE THAT MATTERS MOST IN THIS FILE. A notification ledger is the classic place a whole
	// commune's phone numbers end up in one table, from where they reach every backup and every
	// log aggregator. The rule is also a CHECK constraint in migration 0004 — both layers, on
	// purpose: the constraint holds against a statement typed at a psql prompt, this one holds
	// against a caller and names the rule before a transaction is even open.
	//
	// The fake number is the agreed one (rule 3, invariant 5); what makes it fail here is that it
	// carries no mask.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	res := sentResult()
	res.MaskedRecipient = "0900000000"

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.RecordSendResult(context.Background(), tx, testNotificationID, res)
		return err
	})
	if !errors.Is(err, domain.ErrRecipientNotMasked) {
		t.Fatalf("lỗi = %v, muốn ErrRecipientNotMasked", err)
	}
	if len(f.stmts) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù số chưa được che", len(f.stmts))
	}
	// AND THE VALUE ITSELF MUST NOT BE IN THE ERROR. The error travels into logs, and the value is
	// the very thing suspected of being an unmasked number (rule 3, forbidden #3).
	if strings.Contains(err.Error(), "0900000000") {
		t.Errorf("số chưa che lọt vào thông điệp lỗi: %q", err.Error())
	}
}

func TestRecordSendResultSentWithoutTimeIsRefused(t *testing.T) {
	// "Sent" with no date answers nothing when an inspection asks when the citizen was told.
	f := &fakeWriteStore{rowsAffected: 1}
	s := newTestCitizenNotificationStore(f)

	res := sentResult()
	res.SentAt = time.Time{}

	err := runInTx(t, f, testTenant, func(tx *pkgstore.ScopedTx) error {
		_, err := s.RecordSendResult(context.Background(), tx, testNotificationID, res)
		return err
	})
	if !errors.Is(err, domain.ErrResultMissingSentAt) {
		t.Fatalf("lỗi = %v, muốn ErrResultMissingSentAt", err)
	}
}

// --- BySubject: what did we tell this citizen -----------------------------------------------

func sampleRow(id, createdAt string) fakeNotificationRow {
	t, _ := time.Parse(time.RFC3339, createdAt)
	return fakeNotificationRow{
		id: id, key: "phieu-phan-anh|" + testReportCode + "|da-dong|1|" + testCitizenCode + "|zalo-zns",
		subjectType: "phieu-phan-anh", subjectCode: testReportCode, milestone: "da-dong", round: 1,
		channel: "zalo-zns", recipientCode: testCitizenCode, maskedRecipient: nil,
		templateCode: "zns-phan-anh-doi-trang-thai",
		params:       []byte(`{"ma_tra_cuu":"` + testReportCode + `","moc_nhan":"Đã đóng","viec_tiep_theo":"Ông/bà có thể đánh giá kết quả trong ứng dụng."}`),
		status:       "cho-gui", attemptCount: 0, sentAt: nil, errorCode: nil, createdAt: t,
	}
}

func TestBySubjectStatementBindsTenantFiltersDeletedAndOrdersStably(t *testing.T) {
	f := &fakeWriteStore{rows: []fakeNotificationRow{sampleRow("tb-1", "2026-09-20T08:00:00Z")}}
	s := newTestCitizenNotificationStore(f)

	if _, err := s.BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode); err != nil {
		t.Fatalf("BySubject: %v", err)
	}
	if len(f.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(f.stmts))
	}
	st := f.stmts[0]

	// RULE 1: the commune is bound from the context by Scoped.Query, never passed in.
	if !strings.Contains(st.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", st.sql)
	}
	if st.args[0] != string(testTenant) {
		t.Fatalf("$1 = %v, muốn %q", st.args[0], testTenant)
	}
	// RULE 7, INVARIANT 2.
	if !strings.Contains(st.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", st.sql)
	}
	// THE BUSINESS CODE IS MATCHED EXACTLY. A prefix or a pattern match would turn one lookup code
	// into a way to read the history of the codes around it — rule 4, invariant 4.
	if !strings.Contains(st.sql, "doi_tuong_ma = $3") {
		t.Errorf("mã hồ sơ không được so khớp chính xác: %q", st.sql)
	}
	if strings.Contains(st.sql, "LIKE") || strings.Contains(st.sql, "ILIKE") {
		t.Errorf("so khớp mờ trên mã hồ sơ: %q", st.sql)
	}
	// NEWEST FIRST, WITH A TOTAL ORDER. `id` carries UNIQUE (tenant_id, id), so two calls cannot
	// return the same rows in a different sequence.
	if !strings.Contains(st.sql, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự không ổn định: %q", st.sql)
	}
	// The LIMIT is the ceiling PLUS ONE — that single character is what makes "there are too many"
	// detectable rather than indistinguishable from a complete list.
	if len(st.args) < 4 || st.args[3] != int64(MaxCitizenNotificationsPerRecord+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", st.args[3:], MaxCitizenNotificationsPerRecord+1)
	}
}

func TestBySubjectReadsEachColumnIntoItsField(t *testing.T) {
	// The fake builds each row BY COLUMN NAME out of the statement, so reordering citizenNotificationColumns
	// without reordering the Scan in scanCitizenNotification turns this red. Read by position,
	// `doi_tuong_ma`/`moc` and `nguoi_nhan_ma`/`nguoi_nhan_che` are adjacent same-typed columns and
	// a swap produces no error at all — only a ledger claiming a message about record A carried
	// record B's code.
	r := sampleRow("tb-1", "2026-09-20T08:00:00Z")
	r.maskedRecipient = "09****0000"
	r.status = "da-gui"
	r.attemptCount = 2
	sent, _ := time.Parse(time.RFC3339, "2026-09-20T08:05:00Z")
	r.sentAt = sent
	f := &fakeWriteStore{rows: []fakeNotificationRow{r}}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if err != nil {
		t.Fatalf("BySubject: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(out))
	}
	one := out[0]
	if one.ID != "tb-1" || one.SubjectCode != testReportCode || one.Milestone != "da-dong" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", one)
	}
	if one.RecipientCode != testCitizenCode || one.MaskedRecipient != "09****0000" {
		t.Errorf("người nhận đọc sai chỗ: %+v", one)
	}
	if one.Round != 1 || one.AttemptCount != 2 {
		t.Errorf("hai cột số đọc ngược: lan=%d so_lan_thu=%d", one.Round, one.AttemptCount)
	}
	if one.Status != domain.SendStatusSent || !one.SentAt.Equal(sent) {
		t.Errorf("trạng thái / mốc gửi sai: %+v", one)
	}
	// The template parameters survive the round trip — the ledger has to be able to say what was
	// actually said, and it says it as parameters rather than as rendered text (rule 3).
	if one.Params.LookupCode != testReportCode || one.Params.StatusLabel != "Đã đóng" {
		t.Errorf("tham số đọc sai: %+v", one.Params)
	}
}

func TestBySubjectUnsentHasNoTimeAndNoMaskedNumber(t *testing.T) {
	// THREE NULLABLE COLUMNS, THREE DIFFERENT MEANINGS, and they land as zero values here. An
	// empty masked number means "not known yet", never "the empty number" — which is why the read
	// does not substitute anything for it.
	f := &fakeWriteStore{rows: []fakeNotificationRow{sampleRow("tb-1", "2026-09-20T08:00:00Z")}}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if err != nil {
		t.Fatalf("BySubject: %v", err)
	}
	if out[0].MaskedRecipient != "" || out[0].ErrorCode != "" || !out[0].SentAt.IsZero() {
		t.Errorf("dòng chưa gửi mà mang kết quả: %+v", out[0])
	}
}

func TestBySubjectRecordNeverNotifiedReturnsEmptyList(t *testing.T) {
	f := &fakeWriteStore{}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if err != nil {
		t.Fatalf("sổ rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if out == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(out) != 0 {
		t.Fatalf("nhận %d dòng từ một hồ sơ chưa được báo gì", len(out))
	}
}

func TestBySubjectAtCeilingStillReturnsEverything(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE history, not a refusal. An off-by-one here refuses a
	// record whose data is perfectly valid.
	f := &fakeWriteStore{rows: manyNotificationRows(MaxCitizenNotificationsPerRecord)}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(out) != MaxCitizenNotificationsPerRecord {
		t.Errorf("nhận %d dòng, muốn %d", len(out), MaxCitizenNotificationsPerRecord)
	}
}

func TestBySubjectOverCeilingRefusesAndReturnsNoRows(t *testing.T) {
	// REFUSE, DO NOT TRUNCATE. This list answers "what did we tell this citizen, and when" — the
	// question a complaint opens with. A silently short answer is a commune stating it never sent
	// something it did send. The rows already read are DROPPED: a list handed back alongside an
	// error is a list that gets rendered.
	f := &fakeWriteStore{rows: manyNotificationRows(MaxCitizenNotificationsPerRecord + 1)}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if !errors.Is(err, ErrTooManyCitizenNotifications) {
		t.Fatalf("lỗi = %v, muốn ErrTooManyCitizenNotifications", err)
	}
	if out != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(out))
	}
}

func TestBySubjectStoreErrorIsWrappedNotSwallowed(t *testing.T) {
	cause := errors.New("cơ sở dữ liệu không phản hồi")
	f := &fakeWriteStore{queryErr: cause}

	out, err := newTestCitizenNotificationStore(f).BySubject(tenantCtx(testTenant), domain.SubjectCitizenReport, testReportCode)
	if !errors.Is(err, cause) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, cause)
	}
	if errors.Is(err, ErrTooManyCitizenNotifications) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if out != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestBySubjectWithoutTenantInContextPanics(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either return every commune's
	// notifications or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover
	// turns that into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc sổ khi context không có xã mà không panic")
		}
	}()
	f := &fakeWriteStore{rows: []fakeNotificationRow{sampleRow("tb-1", "2026-09-20T08:00:00Z")}}
	_, _ = newTestCitizenNotificationStore(f).BySubject(context.Background(), domain.SubjectCitizenReport, testReportCode)
}

func manyNotificationRows(n int) []fakeNotificationRow {
	out := make([]fakeNotificationRow, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, sampleRow(fmt.Sprintf("tb-%04d", i), "2026-09-20T08:00:00Z"))
	}
	return out
}
