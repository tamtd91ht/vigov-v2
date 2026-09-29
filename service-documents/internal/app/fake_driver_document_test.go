package app

// A SECOND fake database/sql driver, for the two document registers.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// fake_driver_catalogue_test.go makes, and it is heavier here because the property being protected
// is an ISSUED NUMBER:
//
//	the business write, the number and the audit entry are in ONE transaction  (rule 6, inv. 3)
//	a refusal leaves the transaction with NOTHING committed — and NO NUMBER CONSUMED
//	the number is read `FOR UPDATE`, so two clerks cannot take the same one  (rule 7, inv. 3)
//	a soft delete never returns the number to the series
//	a new year starts a new series
//	the routing UPDATE and its timeline row are one transaction              (rule 7, forbidden #5)
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A SECOND DRIVER AND NOT AN EXTENSION OF THE FIRST: that one answers `count(*)` and one catalogue
// row shape. Teaching it three more tables would make every case in either file depend on a branch
// written for the other, and the first surprising interaction would be silent.
//
// # THIS ONE MODELS THE ROW LOCK, AND THAT IS THE POINT OF THE FILE
//
// The other fake drivers in this repository record statements. This one also SERIALISES: a
// `SELECT … FOR UPDATE` on `day_so_van_ban` takes a token that is held until the transaction ends,
// exactly as PostgreSQL holds a row lock under READ COMMITTED. That is what makes
// TestIssueNumber_TwoConcurrentBookingsGetTwoNumbers a real test rather than a description — remove
// `FOR UPDATE` from the store and it goes red, which is the mutation this file exists to survive.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `UNIQUE (tenant_id, nam, so_vao_so)`, not the
// `so_van_ban_bat_bien` trigger, not `day_so_khong_lui`, not `lich_su_chuyen_chi_them`, not the
// partition routing. That half needs a real server (VIGOV_TEST_DSN is unset here), and it is the
// FLOOR under everything this file asserts — the application refusals tested below are the SENTENCE,
// not the enforcement.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// incomingRow is the row the incoming register's FOR UPDATE read hands back. nil means "no such live
// entry".
type incomingRow struct {
	id              string
	arrivalNo, year int
	receivedDate    time.Time
	referenceNo     string
	documentDate    *time.Time
	issuingBody     string
	documentType    string
	summary         string
	urgency         string
	orgUnit         string
	assignee        string
	due             time.Time
	status          string
	createdBy       string
	createdAt       time.Time
	updatedAt       time.Time
}

// outgoingRow is the same for the outgoing register.
type outgoingRow struct {
	id             string
	issuedNo, year int
	documentDate   time.Time
	documentType   string
	summary        string
	recipient      string
	signer         string
	createdBy      string
	createdAt      time.Time
	updatedAt      time.Time
}

type fakeRegisterDB struct {
	mu    sync.Mutex
	stmts []recordedStmt

	begun, committed, rolledBack int

	incoming *incomingRow
	outgoing *outgoingRow

	// routingRows answers `SELECT … FROM lich_su_chuyen_van_ban`, IN THE ORDER GIVEN — the fake does
	// not sort, so the ORDER BY is asserted on the statement text, not on the result.
	routingRows [][]driver.Value

	// typeInCatalogue answers `SELECT 1 FROM loai_van_ban …`. FALSE IS A REAL CASE: a type the
	// commune retired between the form loading and the clerk pressing save.
	typeInCatalogue bool

	// lastNumber is the number series, keyed "<sổ>|<năm>" — the same key the primary key of
	// `day_so_van_ban` uses, minus the commune, which this fake serves only one of.
	lastNumber map[string]int

	// seriesLock MODELS THE ROW LOCK. Capacity one: whoever holds it is the transaction that ran
	// `SELECT … FOR UPDATE` on the series, and it is returned at Commit or Rollback.
	seriesLock chan struct{}

	// seriesReadDelay is slept at the END of a series read, while the lock (if any) is still held.
	// It makes the race window DETERMINISTIC rather than a matter of scheduling: with `FOR UPDATE` the
	// second reader is still blocked and the delay costs nothing, and without it both readers get
	// the same value every single run. A test of a race that only sometimes races is a test that
	// goes green on the day it matters.
	seriesReadDelay time.Duration

	err    error
	failOn string
	failed bool
}

func newFakeRegisterDB() *fakeRegisterDB {
	return &fakeRegisterDB{
		typeInCatalogue: true,
		lastNumber:      map[string]int{},
		seriesLock:      make(chan struct{}, 1),
	}
}

func (k *fakeRegisterDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	k.stmts = append(k.stmts, recordedStmt{sql: q, args: vals})
	k.mu.Unlock()
}

// stmtsContaining returns every recorded statement containing `sub`, so an assertion names the
// statement it cares about rather than an index that shifts when a check is added.
func (k *fakeRegisterDB) stmtsContaining(sub string) []recordedStmt {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []recordedStmt
	for _, l := range k.stmts {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *fakeRegisterDB) hasStmt(sub string) bool { return len(k.stmtsContaining(sub)) > 0 }

func (k *fakeRegisterDB) maybeFail(q string) error {
	if k.err != nil {
		return k.err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeRegisterDB) Connect(context.Context) (driver.Conn, error) {
	return &fakeRegisterConn{k: k}, nil
}
func (k *fakeRegisterDB) Driver() driver.Driver { return fakeDriver{} }

type fakeRegisterConn struct {
	k  *fakeRegisterDB
	tx *fakeRegisterTx // the transaction currently open on THIS connection, if any
}

func (c *fakeRegisterConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *fakeRegisterConn) Close() error { return nil }

func (c *fakeRegisterConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeRegisterConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	t := &fakeRegisterTx{k: c.k, c: c}
	c.tx = t
	return t, nil
}

func (c *fakeRegisterConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}
	// THE GUARD BELOW IS NOT DEFENSIVE PADDING. A statement against the series with a DIFFERENT
	// parameter list is exactly what the mutation "trả số về dãy" looks like, and without this the
	// fake panics on an index — which turns a test that CAUGHT a defect into a test that crashed.
	// A crash and a failure read the same in CI and do not read the same to a person.
	if len(args) < 4 && strings.Contains(q, "day_so_van_ban") {
		return driver.RowsAffected(1), nil
	}
	switch {
	case strings.Contains(q, "INSERT INTO day_so_van_ban"):
		// ON CONFLICT DO NOTHING — the row is created once and never overwritten.
		key := seriesKey(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		if _, ok := c.k.lastNumber[key]; !ok {
			c.k.lastNumber[key] = 0
		}
		c.k.mu.Unlock()
	case strings.Contains(q, "UPDATE day_so_van_ban"):
		key := seriesKey(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		c.k.lastNumber[key] = int(args[3].Value.(int64))
		c.k.mu.Unlock()
	}
	// ONE ROW AFFECTED, ALWAYS. store.requireOneIncomingRow turns zero into "not found", and a fake
	// returning zero would make every update look like a missing document — hiding the case this
	// actually tests.
	return driver.RowsAffected(1), nil
}

// seriesKey builds the series key from the two bound parameters, exactly as the primary key does.
func seriesKey(register, year driver.Value) string {
	return fmt.Sprintf("%v|%v", register, year)
}

func (c *fakeRegisterConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.record(q, args)
	if err := c.k.maybeFail(q); err != nil {
		return nil, err
	}

	switch {
	case strings.Contains(q, "FROM day_so_van_ban"):
		// THE LOCK IS TAKEN ONLY IF THE STATEMENT ASKS FOR IT. That is the whole mechanism: remove
		// `FOR UPDATE` from the store and this branch stops serialising, which is what the
		// concurrency test observes.
		if strings.Contains(q, "FOR UPDATE") && c.tx != nil && !c.tx.holdsLock {
			c.k.seriesLock <- struct{}{}
			c.tx.holdsLock = true
		}
		key := seriesKey(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		last := c.k.lastNumber[key]
		c.k.mu.Unlock()
		if c.k.seriesReadDelay > 0 {
			time.Sleep(c.k.seriesReadDelay)
		}
		return &fakeRows{cols: []string{"so_cuoi"}, rows: [][]driver.Value{{int64(last)}}}, nil

	case strings.Contains(q, "FROM loai_van_ban"):
		if !c.k.typeInCatalogue {
			return &fakeRows{cols: []string{"?column?"}}, nil
		}
		return &fakeRows{cols: []string{"?column?"}, rows: [][]driver.Value{{int64(1)}}}, nil

	case strings.Contains(q, "FROM lich_su_chuyen_van_ban"):
		return &fakeRows{cols: routingCols(), rows: c.k.routingRows}, nil

	case strings.Contains(q, "FROM van_ban_den"):
		if c.k.incoming == nil {
			return &fakeRows{cols: incomingCols()}, nil
		}
		r := *c.k.incoming
		var documentDate driver.Value
		if r.documentDate != nil {
			documentDate = *r.documentDate
		}
		return &fakeRows{cols: incomingCols(), rows: [][]driver.Value{{
			r.id, int64(r.arrivalNo), int64(r.year), r.receivedDate, r.referenceNo, documentDate,
			r.issuingBody, r.documentType, r.summary, r.urgency, r.orgUnit, r.assignee,
			r.due, r.status, r.createdBy, r.createdAt, r.updatedAt,
		}}}, nil

	case strings.Contains(q, "FROM van_ban_di"):
		if c.k.outgoing == nil {
			return &fakeRows{cols: outgoingCols()}, nil
		}
		r := *c.k.outgoing
		return &fakeRows{cols: outgoingCols(), rows: [][]driver.Value{{
			r.id, int64(r.issuedNo), int64(r.year), r.documentDate, r.documentType, r.summary,
			r.recipient, r.signer, r.createdBy, r.createdAt, r.updatedAt,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// incomingCols mirrors docstore's incomingDocumentColumns ORDER. Written out here rather than
// imported so that reordering the store's list without reordering its Scan turns this red too — and
// that list holds `ngay_den` and `ngay_van_ban` side by side, a swap that produces no error and reads
// as a document received before it was signed.
func incomingCols() []string {
	return []string{
		"id", "so_vao_so", "nam", "ngay_den", "so_ky_hieu", "ngay_van_ban",
		"co_quan_ban_hanh", "loai_van_ban", "trich_yeu", "do_khan",
		"bo_phan_dang_giu_id", "can_bo_xu_ly_ma",
		"han_xu_ly_xong", "trang_thai", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc",
	}
}

// routingCols mirrors docstore's routingColumns ORDER — `tu_bo_phan_id` and `den_bo_phan_id` are
// adjacent TEXT columns, and a swap reads as the file travelling backwards.
func routingCols() []string {
	return []string{
		"id", "van_ban_den_id", "thoi_diem", "nguoi_ma", "trang_thai_tai_thoi_diem",
		"tu_bo_phan_id", "den_bo_phan_id", "can_bo_xu_ly_ma", "noi_dung", "tao_luc",
	}
}

// outgoingCols mirrors docstore's outgoingDocumentColumns ORDER — `trich_yeu`, `noi_nhan` and
// `nguoi_ky` are three adjacent TEXT columns, and a swap there makes a document read as having been
// sent to the person who signed it.
func outgoingCols() []string {
	return []string{
		"id", "so_di", "nam", "ngay_van_ban", "loai_van_ban", "trich_yeu",
		"noi_nhan", "nguoi_ky", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc",
	}
}

type fakeRegisterTx struct {
	k         *fakeRegisterDB
	c         *fakeRegisterConn
	holdsLock bool
}

func (t *fakeRegisterTx) end() {
	if t.holdsLock {
		<-t.k.seriesLock
		t.holdsLock = false
	}
	t.c.tx = nil
}

func (t *fakeRegisterTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	t.end()
	return nil
}

func (t *fakeRegisterTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	t.end()
	return nil
}

// --- the identity stand-in ------------------------------------------------------------------------

// fakeDeadlines answers ResolveDeadlines. IT RECORDS WHAT WAS ASKED FOR, because the three arguments
// are decisions ADR 0028 makes about this act: which work kind, which field row, which clock.
type fakeDeadlines struct {
	// mu GIỮ BỐN TRƯỜNG GHI DƯỚI ĐÂY, và nó có mặt vì một ca kiểm thật chứ không phòng xa.
	//
	// `TestIssueNumber_TwoConcurrentBookingsGetTwoNumbers` cố ý chạy HAI goroutine qua cùng một use
	// case để chứng minh khoá dòng của sổ số hoạt động. Cả hai đi qua `HanXuLy`, nên bốn phép gán
	// dưới là ghi-ghi đồng thời trên một bộ đếm không khoá — `go test -race` bắt đúng chỗ ấy.
	//
	// ĐÂY LÀ LỖI CỦA BỘ ĐỒ THỬ, KHÔNG PHẢI CỦA MÃ NGHIỆP VỤ. Nhưng nó làm `make check` ĐỎ, và
	// một cổng đỏ vì lý do không ai sửa là cổng người ta học cách bỏ qua — rồi lần đỏ thật
	// tiếp theo cũng trôi qua cùng cách. Khoá ở đây rẻ hơn hẳn cái giá ấy.
	//
	// CHỈ KHOÁ ĐƯỜNG GHI. Các ca khác đọc thẳng `deadlines.calls`, `deadlines.workKind`, … sau khi
	// goroutine đã kết thúc, nên chúng happens-after và không cần khoá; bọc thêm accessor chỉ để "cho
	// đủ đối xứng" sẽ bắt sửa mười chỗ đang đúng.
	mu        sync.Mutex
	answer    time.Time
	err       error
	calls     int
	workKind  identityv1.WorkKind
	fieldCode string
	from      time.Time
	kinds     []identityv1.DeadlineKind
}

func (h *fakeDeadlines) HanXuLy(_ context.Context, workKind identityv1.WorkKind, fieldCode string,
	from time.Time, kinds []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error) {

	h.mu.Lock()
	h.calls++
	h.workKind, h.fieldCode, h.from, h.kinds = workKind, fieldCode, from, kinds
	h.mu.Unlock()
	if h.err != nil {
		return nil, h.err
	}
	out := map[identityv1.DeadlineKind]time.Time{}
	for _, k := range kinds {
		out[k] = h.answer
	}
	return out, nil
}

// --- wiring ---------------------------------------------------------------------------------------

// registeredAt is the instant every booking below is stamped with. FIXED rather than time.Now(),
// because it becomes the ORIGIN the commitment is counted from and the YEAR of the series — an
// assertion about either would otherwise be an assertion about when the suite ran.
var registeredAt = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)

// identityDue is what identity answers with: 40 working hours after registeredAt, as that commune's
// calendar would place them. The VALUE is arbitrary; what matters is that it comes from identity and
// is stored unchanged.
var identityDue = time.Date(2026, 9, 29, 3, 30, 0, 0, time.UTC)

const newDocumentID = "01JVANBANMOIVUATAO0000000"

// newIncomingUseCase builds the REAL use case over the REAL store over the fake driver.
//
// `conns` IS A PARAMETER because the concurrency test needs two connections while every other test
// needs exactly one: with one connection every statement of one transaction lands on the same fake in
// order, and a pool would make the ordering assertions about the scheduler instead.
func newIncomingUseCase(t *testing.T, k *fakeRegisterDB, conns int) (*IncomingDocuments, *fakeDeadlines, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(conns)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind all of them, exactly as cmd/server wires it: the transaction the use case
	// opens is the transaction the stores write in, and two handles would be two pools.
	storeDB := store.New(db)
	deadlines := &fakeDeadlines{answer: identityDue}
	uc := NewIncomingDocuments(storeDB, docstore.NewIncomingDocumentStore(storeDB),
		docstore.NewNumberSeriesStore(storeDB), deadlines)
	uc.newID = func() (string, error) { return newDocumentID, nil }
	uc.now = func() time.Time { return registeredAt }
	return uc, deadlines, tenant.Into(context.Background(), tenantA)
}

func newOutgoingUseCase(t *testing.T, k *fakeRegisterDB, conns int) (*OutgoingDocuments, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(conns)
	t.Cleanup(func() { db.Close() })

	storeDB := store.New(db)
	uc := NewOutgoingDocuments(storeDB, docstore.NewOutgoingDocumentStore(storeDB), docstore.NewNumberSeriesStore(storeDB))
	uc.newID = func() (string, error) { return newDocumentID, nil }
	uc.now = func() time.Time { return registeredAt }
	return uc, tenant.Into(context.Background(), tenantA)
}
