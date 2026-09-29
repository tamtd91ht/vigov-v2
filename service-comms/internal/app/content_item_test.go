package app

// The use cases behind the Mini App content write routes, run over the REAL store over a FAKE
// DRIVER.
//
// WHY A DRIVER AND NOT A FAKE STORE — the same argument fake_driver_catalogue_test.go and
// announcement_test.go both make, and it holds hardest here because §10.4 is a rule about what
// a WRITE puts in a column. What this package is responsible for is not "the use case called a
// method", it is:
//
//	the row AND the audit entry are in ONE transaction (rule 6, invariant 3)
//	a refusal writes NOTHING and commits nothing
//	the state, the author and the provenance are decided here and never taken from the request
//	§10.4 — editing a SYNCED item sets `da_sua_tay`, and a no-op edit does not
//	the audit actor is the staff BUSINESS CODE and the article's text never enters the ledger
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the CHECK constraints, not the immutability
// trigger of migration 0006, not the foreign key, not the unique key on `nguon_id_ngoai`, not
// partition routing. That half needs a real server; the pg suite in internal/store is where it lands
// the day a DSN exists.

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
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// contentDB records every statement and every transaction boundary, and answers the four reads these
// use cases make: the locked row, the category probe, the parent probe and the two counts.
type contentDB struct {
	stmts []recordedStmt

	begun, committed, rolledBack int

	// existing is what ByIDForUpdate finds. nil means "no such row".
	existing *domain.ContentItem

	// rowExists answers both existence probes (`SELECT 1 …`).
	rowExists bool

	// TWO COUNTERS FOR TWO `count(*)` STATEMENTS, and merging them is exactly the mistake this
	// comment exists to stop. `CountLive` counts the commune's LIVE categories for the ceiling,
	// while `SlugTaken` counts rows carrying one slug INCLUDING soft-deleted ones. A single field
	// makes "the commune has three categories" also mean "this slug is taken three times", so the
	// happy path of ContentCategories.Create can never be reached and the ceiling case passes for the
	// wrong reason.
	liveCount int64 // CountLive
	slugCount int64 // SlugTaken

	// failOn fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET ERROR: failing everything cannot tell "rolled back" from "never started". The
	// invariant worth proving is that a failure on the LAST statement of the transaction — the audit
	// entry — takes the business write down with it, and that needs everything before it to have
	// already run.
	failOn string
	failed bool
}

func (d *contentDB) Connect(context.Context) (driver.Conn, error) { return &contentConn{d: d}, nil }
func (d *contentDB) Driver() driver.Driver                        { return contentDriverOpen{} }

type contentDriverOpen struct{}

func (contentDriverOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả nội dung Mini App: chỉ dùng Connector")
}

type contentConn struct{ d *contentDB }

func (c *contentConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả nội dung Mini App: không hỗ trợ Prepare")
}
func (c *contentConn) Close() error { return nil }

func (c *contentConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *contentConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.begun++
	return &contentTx{d: c.d}, nil
}

func (c *contentConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.stmts = append(c.d.stmts, recordedStmt{sql: q, args: vals})
	if c.d.failOn != "" && !c.d.failed && strings.Contains(q, c.d.failOn) {
		c.d.failed = true
		return nil, errors.New("driver giả nội dung Mini App: câu lệnh này được dựng để hỏng")
	}
	return driver.RowsAffected(1), nil
}

func (c *contentConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.stmts = append(c.d.stmts, recordedStmt{sql: q, args: vals})

	switch {
	case strings.Contains(q, "count(*)") && strings.Contains(q, "slug = $2"):
		return &contentRows{cols: []string{"count"}, rows: [][]driver.Value{{c.d.slugCount}}}, nil
	case strings.Contains(q, "count(*)"):
		return &contentRows{cols: []string{"count"}, rows: [][]driver.Value{{c.d.liveCount}}}, nil
	case strings.Contains(q, "SELECT 1 FROM"):
		if !c.d.rowExists {
			return &contentRows{cols: []string{"?column?"}}, nil
		}
		return &contentRows{cols: []string{"?column?"}, rows: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.d.existing == nil {
			return &contentRows{cols: contentDetailCols()}, nil
		}
		return &contentRows{cols: contentDetailCols(),
			rows: [][]driver.Value{rowFrom(*c.d.existing)}}, nil
	}
	return nil, errors.New("driver giả nội dung Mini App: truy vấn ngoài dự kiến — " + q)
}

// contentDetailCols is the detail column list, IN THE STORE'S OWN ORDER. It is written out rather than
// parsed out of the statement so that a reorder in the store without a matching reorder in the scan
// lands the wrong value in the wrong field HERE, which is where a test can see it.
func contentDetailCols() []string {
	return []string{"id", "loai", "danh_muc_id", "tieu_de", "tom_tat", "anh_dai_dien_url",
		"ngay_dang", "luot_xem", "trang_thai", "nguon", "nguon_url", "nguon_id_ngoai",
		"da_sua_tay", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc", "noi_dung"}
}

func rowFrom(c domain.ContentItem) []driver.Value {
	nullable := func(s string) driver.Value {
		if s == "" {
			return nil
		}
		return s
	}
	return []driver.Value{
		c.ID, string(c.Type), nullable(c.CategoryID), c.Title, nullable(c.Summary), nullable(c.ImageURL),
		c.PublishedOn, int64(c.ViewCount), string(c.Status), string(c.Source),
		nullable(c.SourceURL), nullable(c.SourceRef), c.HandEdited, c.AuthorCode,
		c.CreatedAt, c.UpdatedAt, nullable(c.Body),
	}
}

type contentRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *contentRows) Columns() []string { return r.cols }
func (r *contentRows) Close() error      { return nil }

// Next MUST RETURN io.EOF AND NOTHING THAT MERELY READS LIKE IT. database/sql compares the driver's
// error against io.EOF by identity to decide "no more rows"; an `errors.New("EOF")` is a different
// value, so the read fails with that error instead of yielding sql.ErrNoRows — and every "row not
// found" refusal in this file would then be reported as a database failure. Cost the first run of
// this suite exactly that.
func (r *contentRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

type contentTx struct{ d *contentDB }

func (t *contentTx) Commit() error   { t.d.committed++; return nil }
func (t *contentTx) Rollback() error { t.d.rolledBack++; return nil }

func (d *contentDB) with(sub string) []recordedStmt {
	var out []recordedStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *contentDB) ran(sub string) bool { return len(d.with(sub)) > 0 }

const (
	pinnedContentID   = "01JNOIDUNGMOI00000000000"
	contentAuthorCode = "CB-2026-7K3M9Q"
)

var pinnedContentTime = time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC)

// newTestContent builds the REAL use cases over the REAL stores over the fake driver, with the
// id AND THE CLOCK pinned so assertions can name both.
func newTestContent(t *testing.T, d *contentDB) (*ContentItems, *ContentCategories, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	s := store.New(db)
	items := commsstore.NewContentItemStore(s)
	categories := commsstore.NewContentCategoryStore(s)

	uc := NewContentItems(s, items, categories)
	uc.newID = func() (string, error) { return pinnedContentID, nil }
	uc.now = func() time.Time { return pinnedContentTime }

	ucCat := NewContentCategories(s, categories)
	ucCat.newID = func() (string, error) { return "01JDANHMUCMOI00000000000", nil }

	return uc, ucCat, tenant.Into(context.Background(), tenantA)
}

func contentAuthor() audit.Actor {
	return audit.Actor{ID: contentAuthorCode, Kind: "staff", IP: "10.0.0.7"}
}

func sampleCreateContent() domain.CreateContentItemRequest {
	return domain.CreateContentItemRequest{
		Type:    "tin-tuc",
		Title:   "Xã Thăng Bình khai giảng năm học mới",
		Summary: "Sáng nay, xã tổ chức lễ khai giảng.",
		Body:    "<p>Toàn văn bài viết.</p>",
	}
}

// --- composing ---------------------------------------------------------------------------------

func TestCreateContentWritesTwoStatementsInOneTransaction(t *testing.T) {
	// RULE 6, INVARIANT 3, AS A PROPERTY OF THE TRANSACTION BOUNDARIES RATHER THAN A CLAIM. If the
	// item and the trail could land in two transactions, a commune could end up publishing something
	// nobody can be asked about.
	d := &contentDB{}
	uc, _, ctx := newTestContent(t, d)

	created, err := uc.Create(ctx, sampleCreateContent(), contentAuthor())
	if err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	for _, want := range []string{"INSERT INTO noi_dung_mini_app", "INSERT INTO audit_log"} {
		if !d.ran(want) {
			t.Errorf("thiếu câu lệnh %q", want)
		}
	}
	if created.ID != pinnedContentID {
		t.Errorf("id = %q, muốn %q", created.ID, pinnedContentID)
	}
	// §7: the checkbox was NOT ticked, so the item is invisible to residents. THE DEFAULT DIRECTION
	// IS THE SAFE ONE: a commune that published something nobody decided to publish has an incident,
	// not a bug.
	if created.Status != domain.ContentStatusHidden {
		t.Errorf("trạng thái = %q, muốn an khi chưa bật `Đăng lên Mini App`", created.Status)
	}
}

func TestCreateContentDecidesStatusSourceAndAuthorHere(t *testing.T) {
	// The state, the provenance and the author are NOT in the request (domain.CreateContentItemRequest
	// has no field for any of them). This asserts what actually reaches the INSERT, which is the only
	// place a future "small" change could reintroduce a client-chosen value.
	d := &contentDB{}
	uc, _, ctx := newTestContent(t, d)

	req := sampleCreateContent()
	req.Publish = true
	if _, err := uc.Create(ctx, req, contentAuthor()); err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}

	ins := d.with("INSERT INTO noi_dung_mini_app")
	if len(ins) != 1 {
		t.Fatalf("số câu chèn = %d, muốn 1", len(ins))
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 ngày đăng, $10 trạng thái, $11 người tạo. `nguon` là hằng trong câu lệnh.
	args := ins[0].args
	if len(args) != 11 {
		t.Fatalf("số tham số = %d, muốn 11", len(args))
	}
	if args[0] != string(tenantA) {
		t.Errorf("xã = %v, muốn %v — xã đến từ ngữ cảnh, không từ yêu cầu", args[0], tenantA)
	}
	if args[9] != string(domain.ContentStatusVisible) {
		t.Errorf("trạng thái = %v, muốn dang-hien khi ô `Đăng lên Mini App` được tích", args[9])
	}
	if args[10] != contentAuthorCode {
		t.Errorf("người tạo = %v, muốn mã cán bộ từ phiên (luật 6, bất biến 8)", args[10])
	}
	// `ngay_dang` IS ONE CLOCK READING FOR THE WHOLE ACT, as a DATE. §7 collects no date field.
	if day, _ := args[8].(time.Time); !day.Equal(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày đăng = %v, muốn 2026-09-23 (phần ngày của một lần đọc đồng hồ)", args[8])
	}
	// THE PROVENANCE IS A LITERAL AND NOT A PARAMETER — the one edit that would reopen §10.4.
	if !strings.Contains(ins[0].sql, "'thu-cong'") {
		t.Errorf("`nguon` phải là hằng trong câu chèn: %s", ins[0].sql)
	}
}

func TestCreateContentTrailUsesStaffCodeAndCarriesNoText(t *testing.T) {
	// TWO RULES IN ONE ASSERTION, and both have a measured history in this repository:
	//   rule 6, invariant 8 — `actor_id` is `CB-…`, never a ULID. Six write paths got this wrong on
	//                          2026-09-22 and no test turned red.
	//   rule 3, forbidden #5 — the audit ledger is never deleted, so the title, the summary and the
	//                          body (free text that routinely names residents) must not enter it.
	d := &contentDB{}
	uc, _, ctx := newTestContent(t, d)

	if _, err := uc.Create(ctx, sampleCreateContent(), contentAuthor()); err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	trail := d.with("INSERT INTO audit_log")
	if len(trail) != 1 {
		t.Fatalf("số vết kiểm toán = %d, muốn 1", len(trail))
	}
	args := trail[0].args
	// $1 xã, $2 actor_id, $3 actor_kind, $4 actor_ip, $5 action, $6 subject, $7 at, $8 delta.
	if args[1] != contentAuthorCode {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q", args[1], contentAuthorCode)
	}
	if args[4] != ActionCreateContentItem {
		t.Errorf("action = %v, muốn %q", args[4], ActionCreateContentItem)
	}
	subject, _ := args[5].(string)
	if subject != "noi-dung-mini-app/tin-tuc/2026-09-23" {
		t.Errorf("subject = %q, muốn noi-dung-mini-app/<loại>/<ngày đăng>", subject)
	}
	delta, _ := args[7].([]byte)
	for _, banned := range []string{"Thăng Bình", "khai giảng", "Toàn văn"} {
		if strings.Contains(string(delta), banned) {
			t.Errorf("delta chứa %q — tiêu đề, tóm tắt và toàn văn không được vào sổ kiểm toán", banned)
		}
	}
	// What the delta MUST carry: the handle on the record, and the facts that decide what may later
	// be done to it.
	for _, must := range []string{pinnedContentID, "thu-cong", "tin-tuc"} {
		if !strings.Contains(string(delta), must) {
			t.Errorf("delta thiếu %q", must)
		}
	}
}

func TestCreateContentMissingCategoryRefusedAndRolledBack(t *testing.T) {
	// The foreign key would refuse it too, with a driver's exception and a 500. This turns the
	// ordinary case — a screen somebody left open while a colleague retired the category — into a
	// refusal with a sentence, AND nothing is committed.
	d := &contentDB{rowExists: false}
	uc, _, ctx := newTestContent(t, d)

	req := sampleCreateContent()
	req.CategoryID = "dm-da-xoa"
	_, err := uc.Create(ctx, req, contentAuthor())
	if !errors.Is(err, commsstore.ErrContentCategoryNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrContentCategoryNotFound", err)
	}
	if d.ran("INSERT INTO noi_dung_mini_app") || d.ran("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", d.committed, d.rolledBack)
	}
}

func TestCreateContentTrailFailureRollsBackWrite(t *testing.T) {
	// THE INVARIANT THAT ONLY A TRANSACTION TEST CAN SHOW. If the trail fails, the article must go
	// with it: rule 6, invariant 3 does not permit a business write whose entry failed, and rule 7
	// would make the orphaned article permanent.
	d := &contentDB{failOn: "INSERT INTO audit_log"}
	uc, _, ctx := newTestContent(t, d)

	if _, err := uc.Create(ctx, sampleCreateContent(), contentAuthor()); err == nil {
		t.Fatal("vết kiểm toán hỏng mà thêm vẫn báo thành công")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", d.committed, d.rolledBack)
	}
}

func TestCreateContentMissingStaffCodeRefusedBeforeTransaction(t *testing.T) {
	// `nguoi_tao_ma` with nothing in it is an article nobody can be asked about, on a channel every
	// resident reads. There is NO FALLBACK to an internal id (rule 6, invariant 8).
	d := &contentDB{}
	uc, _, ctx := newTestContent(t, d)

	_, err := uc.Create(ctx, sampleCreateContent(), audit.Actor{Kind: "staff", IP: "10.0.0.7"})
	if !errors.Is(err, ErrMissingContentAuthor) {
		t.Fatalf("lỗi = %v, muốn ErrMissingContentAuthor", err)
	}
	if d.begun != 0 || len(d.stmts) != 0 {
		t.Errorf("từ chối mà vẫn mở %d giao dịch và chạy %d câu lệnh", d.begun, len(d.stmts))
	}
}

func TestCreateContentBadShapeRefusedBeforeTransaction(t *testing.T) {
	d := &contentDB{}
	uc, _, ctx := newTestContent(t, d)

	req := sampleCreateContent()
	req.ImageURL = "javascript:alert(1)"
	if _, err := uc.Create(ctx, req, contentAuthor()); !errors.Is(err, domain.ErrInvalidURL) {
		t.Fatalf("lỗi = %v, muốn ErrInvalidURL", err)
	}
	if d.begun != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng", d.begun)
	}
}

// --- editing, and §10.4 --------------------------------------------------------------------------

func syncedRow() *domain.ContentItem {
	return &domain.ContentItem{
		ID: "nd-001", Type: domain.ContentTypeNews, Title: "Tiêu đề từ Cổng",
		PublishedOn: pinnedContentTime, Status: domain.ContentStatusVisible,
		Source: domain.ContentSourcePortalSync, SourceURL: "https://cong/a", SourceRef: "cong-42",
		HandEdited: false, AuthorCode: "CB-2026-AAAA11",
		CreatedAt: pinnedContentTime, UpdatedAt: pinnedContentTime, Body: "<p>Từ Cổng</p>",
	}
}

func TestUpdateSyncedItemSetsHandEdited(t *testing.T) {
	// §10.4, THE ONE RULE OF THIS FILE WORTH READING TWICE. A member of staff corrected a portal
	// article's title; the next sync runs in six hours and must not undo it. The flag is the whole
	// mechanism, and migration 0006 then refuses to clear it.
	d := &contentDB{existing: syncedRow()}
	uc, _, ctx := newTestContent(t, d)

	title := "Tiêu đề đã sửa tay"
	after, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Title: &title}, contentAuthor())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if !after.HandEdited {
		t.Fatal("sửa tay một bài đồng bộ về mà KHÔNG đặt cờ da_sua_tay — lượt đồng bộ sau sẽ ghi đè (§10.4)")
	}
	upd := d.with("UPDATE noi_dung_mini_app")
	if len(upd) != 1 {
		t.Fatalf("số câu cập nhật = %d, muốn 1", len(upd))
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 trạng thái, $10 da_sua_tay.
	if upd[0].args[9] != true {
		t.Errorf("tham số da_sua_tay = %v, muốn true", upd[0].args[9])
	}
}

func TestUpdateManualItemDoesNotSetHandEdited(t *testing.T) {
	// THE OTHER HALF, AND IT IS NOT DECORATION: the CHECK constraint of migration 0006 refuses
	// `da_sua_tay` on a `thu-cong` row, so setting it here would make every edit of a hand-composed
	// article fail at the database — and the flag would claim a protection that protects nothing.
	manual := syncedRow()
	manual.Source = domain.ContentSourceManual
	manual.SourceURL, manual.SourceRef = "", ""

	d := &contentDB{existing: manual}
	uc, _, ctx := newTestContent(t, d)

	title := "Tiêu đề đã sửa"
	after, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Title: &title}, contentAuthor())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if after.HandEdited {
		t.Error("bài soạn tay không được mang cờ da_sua_tay — ràng buộc CHECK của 0006 từ chối đúng hình dạng ấy")
	}
}

func TestUpdateNoChangeWritesNothingAuditsNothingSetsNoFlag(t *testing.T) {
	// THREE PROPERTIES IN ONE CASE, and the third is the one that would be missed. A no-op must write
	// nothing and audit nothing — otherwise the ledger fills with entries saying nothing changed, and
	// those are the entries that bury the ones carrying legal weight. AND it must not set
	// `da_sua_tay`: opening the modal and pressing save without touching anything would otherwise
	// take a portal article permanently out of the sync's reach.
	d := &contentDB{existing: syncedRow()}
	uc, _, ctx := newTestContent(t, d)

	same := "Tiêu đề từ Cổng"
	after, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Title: &same}, contentAuthor())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if d.ran("UPDATE noi_dung_mini_app") {
		t.Error("không đổi gì mà vẫn chạy câu cập nhật")
	}
	if d.ran("INSERT INTO audit_log") {
		t.Error("không đổi gì mà vẫn ghi vết kiểm toán")
	}
	if after.HandEdited {
		t.Error("lần lưu không đổi gì mà vẫn đặt cờ da_sua_tay — bài sẽ thoát khỏi đồng bộ vì một lần bấm Lưu")
	}
	if d.committed != 1 {
		t.Errorf("giao dịch không đổi gì: chốt=%d, muốn 1", d.committed)
	}
}

func TestUpdateKeepsUnmentionedFields(t *testing.T) {
	// THE WHOLE REASON THE REQUEST IS A STRUCT OF POINTERS. A screen editing only the title must not
	// clear the summary, drop the article out of its category, or unpublish it.
	orig := syncedRow()
	orig.CategoryID = "dm-001"
	orig.Summary = "Tóm tắt cũ"

	d := &contentDB{existing: orig}
	uc, _, ctx := newTestContent(t, d)

	title := "Tiêu đề mới"
	after, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Title: &title}, contentAuthor())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if after.Summary != "Tóm tắt cũ" || after.CategoryID != "dm-001" ||
		after.Status != domain.ContentStatusVisible {
		t.Errorf("trường không được nhắc bị đổi: %+v", after)
	}
}

func TestUpdateTakesItemOffMiniAppViaCheckbox(t *testing.T) {
	// §7's checkbox is also how an item comes OFF the Mini App. There is no DELETE route in chapter
	// 11, and §6's action column offers only `✎`.
	d := &contentDB{existing: syncedRow()}
	uc, _, ctx := newTestContent(t, d)

	off := false
	after, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Publish: &off}, contentAuthor())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if after.Status != domain.ContentStatusHidden {
		t.Errorf("trạng thái = %q, muốn an", after.Status)
	}
	if after.IsVisibleToCitizens() {
		t.Error("bài đã tắt mà vẫn hiện cho dân")
	}
}

func TestUpdateNoRowReportsNotFound(t *testing.T) {
	d := &contentDB{existing: nil}
	uc, _, ctx := newTestContent(t, d)

	title := "Tiêu đề"
	_, err := uc.Update(ctx, "nd-cua-xa-khac", domain.UpdateContentItemRequest{Title: &title}, contentAuthor())
	if !errors.Is(err, commsstore.ErrContentItemNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrContentItemNotFound", err)
	}
	if d.ran("UPDATE noi_dung_mini_app") {
		t.Error("không tìm thấy dòng mà vẫn chạy câu cập nhật")
	}
}

func TestUpdateReadsRowUnderForUpdateLock(t *testing.T) {
	// `FOR UPDATE` IS NOT AN OPTIMISATION. Without it two members of staff editing the same article
	// both read the old state and the second write silently overwrites the first — including the case
	// where one of them was unpublishing it.
	d := &contentDB{existing: syncedRow()}
	uc, _, ctx := newTestContent(t, d)

	title := "Tiêu đề mới"
	if _, err := uc.Update(ctx, "nd-001", domain.UpdateContentItemRequest{Title: &title}, contentAuthor()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	reads := d.with("FOR UPDATE")
	if len(reads) != 1 {
		t.Fatalf("số câu đọc có khoá = %d, muốn 1", len(reads))
	}
	if reads[0].args[0] != string(tenantA) {
		t.Errorf("xã = %v, muốn %v", reads[0].args[0], tenantA)
	}
}

// --- the category tree -----------------------------------------------------------------------------

func TestCreateCategoryTrailSubjectIsSlug(t *testing.T) {
	// A CATEGORY HAS A BUSINESS CODE, so its audit subject is a VALUE rather than something composed
	// — which is the shape rule 6 and audit.Entry's own contract ask for, and the reason the content
	// item's subject has to be composed instead.
	d := &contentDB{liveCount: 3}
	_, ucCat, ctx := newTestContent(t, d)

	created, err := ucCat.Create(ctx, domain.CreateContentCategoryRequest{
		Name: "Chuyển đổi số", Slug: "chuyen-doi-so", SortOrder: 2,
	}, contentAuthor())
	if err != nil {
		t.Fatalf("thêm danh mục lỗi: %v", err)
	}
	if created.Slug != "chuyen-doi-so" {
		t.Errorf("slug = %q", created.Slug)
	}
	trail := d.with("INSERT INTO audit_log")
	if len(trail) != 1 {
		t.Fatalf("số vết = %d, muốn 1", len(trail))
	}
	if trail[0].args[1] != contentAuthorCode {
		t.Errorf("actor_id = %v, muốn mã cán bộ", trail[0].args[1])
	}
	if trail[0].args[5] != "chuyen-doi-so" {
		t.Errorf("subject = %v, muốn slug", trail[0].args[5])
	}
	if d.committed != 1 || d.rolledBack != 0 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 1/0", d.committed, d.rolledBack)
	}
}

func TestCreateCategoryTakenSlugRefused(t *testing.T) {
	// The unique key is the real guard; this is the readable message. `slugCount` = 1 makes SlugTaken
	// say yes — including for a SOFT-DELETED row, which is the case rule 7, invariant 3 is about.
	d := &contentDB{slugCount: 1}
	_, ucCat, ctx := newTestContent(t, d)

	_, err := ucCat.Create(ctx, domain.CreateContentCategoryRequest{Name: "Chuyển đổi số", Slug: "chuyen-doi-so"},
		contentAuthor())
	if !errors.Is(err, commsstore.ErrCategorySlugTaken) {
		t.Fatalf("lỗi = %v, muốn ErrCategorySlugTaken", err)
	}
	if d.ran("INSERT INTO danh_muc_mini_app") {
		t.Error("slug trùng mà vẫn chèn")
	}
}

func TestCreateCategoryOverCeilingRefusedBeforeSlugCheck(t *testing.T) {
	// ORDER OF THE REFUSALS: the ceiling is a condition of the WHOLE list, which the caller can act on
	// without knowing anything about this value. A full tree reported as "slug đã dùng" would send
	// somebody renaming their category forever.
	d := &contentDB{liveCount: int64(commsstore.ContentCategoryCeiling)}
	_, ucCat, ctx := newTestContent(t, d)

	_, err := ucCat.Create(ctx, domain.CreateContentCategoryRequest{Name: "Mục mới", Slug: "muc-moi"}, contentAuthor())
	if !errors.Is(err, commsstore.ErrTooManyContentCategories) {
		t.Fatalf("lỗi = %v, muốn ErrTooManyContentCategories", err)
	}
}

func TestCreateCategoryMissingParentRefused(t *testing.T) {
	// The self-referencing foreign key refuses a parent that does not exist AT ALL; this also catches
	// a SOFT-DELETED parent, which is still a row and which the constraint therefore accepts — and
	// which §7's select, excluding deleted rows, would draw as a child whose parent is nowhere.
	d := &contentDB{liveCount: 0, rowExists: false}
	_, ucCat, ctx := newTestContent(t, d)

	_, err := ucCat.Create(ctx, domain.CreateContentCategoryRequest{
		Name: "Mục con", Slug: "muc-con", ParentID: "dm-da-xoa",
	}, contentAuthor())
	if !errors.Is(err, ErrParentCategoryNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrParentCategoryNotFound", err)
	}
	if d.ran("INSERT INTO danh_muc_mini_app") {
		t.Error("cha không tồn tại mà vẫn chèn")
	}
}
