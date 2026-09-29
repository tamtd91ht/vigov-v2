package store

// The announcement book's SQL, over a FAKE DRIVER — the half that runs on every machine.
//
// THE OTHER HALF IS announcement_pg_test.go AND NEITHER REPLACES THE OTHER. This file proves
// the Go side: that the page read asks for the columns the scan reads, in that order; that the
// commune is bound and is never a parameter; that the counter read is ONE statement for a whole
// page; that a soft-deleted announcement is excluded. What it CANNOT prove is anything the
// database decides — that these columns and this table exist at all, the CHECK constraints, the
// immutability triggers, the foreign keys. That is what the pg suite is for, and it SKIPS unless
// VIGOV_TEST_DSN is set.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgpage "github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var (
	announcementTenant = tenant.ID("01JTB" + strings.Repeat("A", 21))
	sampleTime         = time.Date(2026, 9, 7, 16, 35, 0, 0, time.UTC)
)

// fakeAnnouncementRow is one `thong_bao` row, addressed BY COLUMN NAME.
//
// THIS IS WHAT MAKES THE COLUMN CHECK REAL: the fake assembles each row from the names the store
// actually asked for, so reordering announcementColumns without reordering scanAnnouncement turns
// this suite red. A fake that returned a fixed tuple would agree with any order.
type fakeAnnouncementRow struct {
	id, title, body, status, emailStatus, authorCode string
	pinned, ackRequired, emailRequested              bool
	issuedAt                                         any // nil for a draft
	createdAt                                        time.Time
}

func (r fakeAnnouncementRow) value(col string) driver.Value {
	switch col {
	case "id":
		return r.id
	case "tieu_de":
		return r.title
	case "noi_dung":
		return r.body
	case "trang_thai":
		return r.status
	case "ghim":
		return r.pinned
	case "bat_buoc_xac_nhan":
		return r.ackRequired
	case "gui_thu_dien_tu":
		return r.emailRequested
	case "trang_thai_thu":
		return r.emailStatus
	case "nguoi_soan_ma":
		return r.authorCode
	case "phat_hanh_luc":
		return r.issuedAt
	case "tao_luc":
		return r.createdAt
	default:
		// LOUD, NOT ZERO. A silent zero here would let a column be added to announcementColumns and
		// never actually be read by anything, while this suite "passed" and proved nothing about it.
		panic("driver giả thông báo nội bộ: không có giá trị mẫu cho cột " + col)
	}
}

// fakeCountRow is one row of the counter query.
type fakeCountRow struct {
	id           string
	total, acked int64
}

type fakeAnnouncementStore struct {
	stmts  []recordedStmt
	rows   []fakeAnnouncementRow
	counts []fakeCountRow
	err    error
}

func (f *fakeAnnouncementStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeAnnouncementConn{f: f}, nil
}
func (f *fakeAnnouncementStore) Driver() driver.Driver { return fakeAnnouncementDriver{} }

type fakeAnnouncementDriver struct{}

func (fakeAnnouncementDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả thông báo nội bộ: chỉ dùng Connector")
}

type fakeAnnouncementConn struct{ f *fakeAnnouncementStore }

func (c *fakeAnnouncementConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả thông báo nội bộ: không hỗ trợ Prepare")
}
func (c *fakeAnnouncementConn) Close() error { return nil }
func (c *fakeAnnouncementConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeAnnouncementConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeAnnouncementTx{}, nil
}

type fakeAnnouncementTx struct{}

func (fakeAnnouncementTx) Commit() error   { return nil }
func (fakeAnnouncementTx) Rollback() error { return nil }

func (c *fakeAnnouncementConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.err != nil {
		return nil, c.f.err
	}
	return fakeResult{n: 1}, nil
}

func (c *fakeAnnouncementConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.err != nil {
		return nil, c.f.err
	}

	if strings.Contains(q, "GROUP BY thong_bao_id") {
		out := make([][]driver.Value, 0, len(c.f.counts))
		for _, d := range c.f.counts {
			out = append(out, []driver.Value{d.id, d.total, d.acked})
		}
		return &fakeWriteRows{cols: []string{"thong_bao_id", "tong", "xac_nhan"}, rows: out}, nil
	}

	cols, err := selectColumnsWrite(q)
	if err != nil {
		return nil, err
	}
	out := make([][]driver.Value, 0, len(c.f.rows))
	for _, r := range c.f.rows {
		one := make([]driver.Value, len(cols))
		for i, name := range cols {
			one[i] = r.value(name)
		}
		out = append(out, one)
	}
	return &fakeWriteRows{cols: cols, rows: out}, nil
}

func newTestAnnouncementStore(t *testing.T, f *fakeAnnouncementStore) (*AnnouncementStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewAnnouncementStore(pkgstore.New(db)), tenant.Into(context.Background(), announcementTenant)
}

func firstPage(t *testing.T) pkgpage.Request {
	t.Helper()
	req, err := pkgpage.New(AnnouncementSorts, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return req
}

// --- the read path -------------------------------------------------------------------------

func TestListAnnouncementsReadsRightColumnsInRightOrder(t *testing.T) {
	// THE ASSERTION THIS FILE EXISTS FOR. The row is assembled BY NAME from the columns the store
	// asked for, so a swap between announcementColumns and scanAnnouncement lands the wrong value in
	// the wrong field — three adjacent booleans make that swap invisible to the compiler.
	f := &fakeAnnouncementStore{rows: []fakeAnnouncementRow{{
		id: "tb-001", title: "Thông báo về việc triển khai hệ thống an ninh",
		body: "Toàn văn nội dung.", status: "da-phat-hanh",
		pinned: true, ackRequired: true, emailRequested: false,
		emailStatus: "chua-gui", authorCode: "CB-2026-7K3M9Q",
		issuedAt: sampleTime, createdAt: sampleTime,
	}}}
	s, ctx := newTestAnnouncementStore(t, f)

	res, err := s.List(ctx, firstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("số thẻ = %d, muốn 1", len(res.Items))
	}
	one := res.Items[0]
	if one.ID != "tb-001" || one.Title != "Thông báo về việc triển khai hệ thống an ninh" {
		t.Errorf("thẻ = %+v", one)
	}
	if one.Body != "Toàn văn nội dung." {
		t.Errorf("nội dung = %q — cột này PHẢI có trong danh sách: §2 vẽ panel chi tiết từ chính nó", one.Body)
	}
	if !one.Pinned || !one.AckRequired || one.EmailRequested {
		t.Errorf("ba cờ bị hoán vị: ghim=%v bắt buộc=%v gửi thư=%v",
			one.Pinned, one.AckRequired, one.EmailRequested)
	}
	if one.Status != domain.AnnouncementPublished || one.EmailStatus != domain.EmailNotSent {
		t.Errorf("trạng thái = %q / %q", one.Status, one.EmailStatus)
	}
	if one.AuthorCode != "CB-2026-7K3M9Q" {
		t.Errorf("người soạn = %q, muốn mã cán bộ", one.AuthorCode)
	}
}

func TestListAnnouncementsDraftHasEmptyIssuedAt(t *testing.T) {
	// `phat_hanh_luc` IS NULL FOR A DRAFT, and scanning a NULL straight into a time.Time is a
	// runtime error in some drivers and a zero value in others. The store reads it through
	// sql.NullTime; this is the case that proves it.
	f := &fakeAnnouncementStore{rows: []fakeAnnouncementRow{{
		id: "tb-nhap", title: "Nháp", body: "Nội dung", status: "nhap",
		emailStatus: "chua-gui", authorCode: "CB-1", issuedAt: nil, createdAt: sampleTime,
	}}}
	s, ctx := newTestAnnouncementStore(t, f)

	res, err := s.List(ctx, firstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if !res.Items[0].IssuedAt.IsZero() {
		t.Errorf("bản nháp có phát hành lúc = %v, muốn rỗng", res.Items[0].IssuedAt)
	}
	if res.Items[0].IsPublished() {
		t.Error("bản nháp không được coi là đã phát hành")
	}
}

func TestListAnnouncementsFiltersSoftDeletedAndBindsTenant(t *testing.T) {
	f := &fakeAnnouncementStore{rows: []fakeAnnouncementRow{{
		id: "tb-001", title: "T", body: "N", status: "da-phat-hanh",
		emailStatus: "chua-gui", authorCode: "CB-1", issuedAt: sampleTime, createdAt: sampleTime,
	}}}
	s, ctx := newTestAnnouncementStore(t, f)

	if _, err := s.List(ctx, firstPage(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var pageStmt string
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "FROM thong_bao ") {
			pageStmt = st.sql
			if len(st.args) == 0 || st.args[0] != string(announcementTenant) {
				t.Fatalf("tham số $1 = %v, muốn xã %v — xã đến từ ngữ cảnh (luật 1, bất biến 5)",
					st.args, announcementTenant)
			}
		}
	}
	if pageStmt == "" {
		t.Fatal("không thấy câu đọc trang")
	}
	// RULE 7, INVARIANT 2 — everywhere, always. A soft-deleted announcement must not appear in a
	// list, and the partial index `thong_bao_so` is built on exactly this predicate.
	if !strings.Contains(pageStmt, "deleted_at IS NULL") {
		t.Errorf("câu đọc trang thiếu `deleted_at IS NULL`: %s", pageStmt)
	}
	if !strings.Contains(pageStmt, "tenant_id = $1") {
		t.Errorf("câu đọc trang thiếu `tenant_id = $1`: %s", pageStmt)
	}
	// §2: "mới nhất ở trên", and the `id` tie-break is what makes the order TOTAL — without it two
	// announcements written in the same millisecond let page two repeat one and drop the other.
	if !strings.Contains(pageStmt, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự sai: %s", pageStmt)
	}
}

func TestListAnnouncementsCountsRecipientsInOneStatementPerPage(t *testing.T) {
	// NEVER 1+N. A query per card is affordable on three sample notices and unaffordable on a
	// commune's third year, and the shape that behaves that way is the shape nobody notices.
	f := &fakeAnnouncementStore{
		rows: []fakeAnnouncementRow{
			{id: "tb-1", title: "T1", body: "N", status: "da-phat-hanh",
				emailStatus: "chua-gui", authorCode: "CB-1", issuedAt: sampleTime, createdAt: sampleTime},
			{id: "tb-2", title: "T2", body: "N", status: "da-phat-hanh",
				emailStatus: "chua-gui", authorCode: "CB-1", issuedAt: sampleTime, createdAt: sampleTime},
		},
		counts: []fakeCountRow{{id: "tb-1", total: 12, acked: 2}},
	}
	s, ctx := newTestAnnouncementStore(t, f)

	res, err := s.List(ctx, firstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var countStmts int
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "GROUP BY thong_bao_id") {
			countStmts++
			if !strings.Contains(st.sql, "tenant_id = $1") {
				t.Errorf("câu đếm thiếu `tenant_id = $1` — sẽ đếm người nhận của xã khác: %s", st.sql)
			}
			if len(st.args) != 3 || st.args[0] != string(announcementTenant) {
				t.Errorf("tham số câu đếm = %v, muốn [xã, tb-1, tb-2]", st.args)
			}
		}
	}
	if countStmts != 1 {
		t.Fatalf("số câu đếm = %d, muốn 1 cho cả trang", countStmts)
	}

	// §3's `2/12 đã xác nhận` on the first card, and `0/0` on the second — a card with no recipient
	// rows produces NO row in a GROUP BY, so the zero value is the correct answer rather than a
	// missing key to crash on.
	if res.Items[0].RecipientCount != 12 || res.Items[0].AckCount != 2 {
		t.Errorf("thẻ 1 = %d/%d, muốn 2/12", res.Items[0].AckCount, res.Items[0].RecipientCount)
	}
	if res.Items[1].RecipientCount != 0 || res.Items[1].AckCount != 0 {
		t.Errorf("thẻ 2 = %d/%d, muốn 0/0", res.Items[1].AckCount, res.Items[1].RecipientCount)
	}
}

func TestListAnnouncementsEmptyPageRunsNoCount(t *testing.T) {
	// Not only a saving: the `IN (…)` list is built from the ids, and an empty list is not valid
	// SQL. A newly onboarded commune takes this branch on every request.
	f := &fakeAnnouncementStore{}
	s, ctx := newTestAnnouncementStore(t, f)

	res, err := s.List(ctx, firstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("số thẻ = %d, muốn 0", len(res.Items))
	}
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "GROUP BY thong_bao_id") {
			t.Fatalf("trang rỗng mà vẫn chạy câu đếm: %s", st.sql)
		}
	}
}

func TestAnnouncementSortsAcceptOnlyCreatedAt(t *testing.T) {
	// THE ALLOWLIST IS THE WHOLE SURFACE a client may sort on, and `phat_hanh_luc` is deliberately
	// not on it: it is NULL for a draft, and `(col, id) > (…)` is NULL for a NULL col — every draft
	// would vanish from every page after the first, silently.
	if _, err := pkgpage.New(AnnouncementSorts, "issued_at", "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
		t.Errorf("sắp xếp theo issued_at: lỗi = %v, muốn ErrSort", err)
	}
	if _, err := pkgpage.New(AnnouncementSorts, "pinned", "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
		t.Errorf("sắp xếp theo pinned: lỗi = %v, muốn ErrSort", err)
	}
}

// --- the write path -------------------------------------------------------------------------

func TestInsertRecipientsOneStatementForWholeList(t *testing.T) {
	f := &fakeAnnouncementStore{}
	s, ctx := newTestAnnouncementStore(t, f)

	err := pkgstore.New(sql.OpenDB(f)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.InsertRecipients(ctx, tx, "tb-1", []domain.AnnouncementRecipient{
			{RecipientCode: "CB-A", IsNamed: true},
			{RecipientCode: "CB-B", IsNamed: false},
		})
	})
	if err != nil {
		t.Fatalf("chèn người nhận lỗi: %v", err)
	}

	var stmt string
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "INSERT INTO thong_bao_nguoi_nhan") {
			if stmt != "" {
				t.Fatal("chèn người nhận phải là MỘT câu cho cả danh sách")
			}
			stmt = st.sql
			if st.args[0] != string(announcementTenant) {
				t.Errorf("xã = %v, muốn %v", st.args[0], announcementTenant)
			}
		}
	}
	if stmt == "" {
		t.Fatal("không thấy câu chèn người nhận")
	}
	// The overlap between "named explicitly" and "member of a chosen department" is legitimate, so
	// one person is ONE row — which is what makes §3's `{y}` count people rather than reasons.
	if !strings.Contains(stmt, "ON CONFLICT DO NOTHING") {
		t.Errorf("thiếu ON CONFLICT DO NOTHING: %s", stmt)
	}
	// The recipient is born having neither opened nor acknowledged anything: binding those columns
	// here would let a caller create a recipient already marked as having read something.
	for _, banned := range []string{"da_mo_luc", "da_xac_nhan_luc"} {
		if strings.Contains(stmt, banned) {
			t.Errorf("câu chèn không được nhận %q: %s", banned, stmt)
		}
	}
}

func TestInsertRecipientsEmptyListRunsNoStatement(t *testing.T) {
	f := &fakeAnnouncementStore{}
	s, ctx := newTestAnnouncementStore(t, f)

	err := pkgstore.New(sql.OpenDB(f)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.InsertRecipients(ctx, tx, "tb-1", nil)
	})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "INSERT INTO thong_bao_nguoi_nhan") {
			t.Fatalf("danh sách rỗng mà vẫn chạy câu chèn: %s", st.sql)
		}
	}
}
