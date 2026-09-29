package store

// The Mini App content register's SQL, over a FAKE DRIVER — the half that runs on every machine.
//
// THE OTHER HALF IS content_item_pg_test.go AND NEITHER REPLACES THE OTHER. This file proves the
// Go side: that the page read asks for the columns the scan reads, in that order; that the commune is
// bound and is never a parameter; that §6's three filters become bound placeholders in the right
// order; that a soft-deleted item is excluded everywhere and a soft-deleted SLUG is deliberately not;
// that the write statements cannot carry the values they must not carry. What it CANNOT prove is
// anything the database decides — that these columns and these tables exist at all, the CHECK
// constraints, the immutability trigger, the foreign key, the unique key. That is what the pg suite
// is for, and it SKIPS unless VIGOV_TEST_DSN is set.

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
	contentTenant = tenant.ID("01JND" + strings.Repeat("C", 21))
	contentTime   = time.Date(2026, 9, 14, 8, 9, 0, 0, time.UTC)
	sampleDate    = time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
)

// fakeContentRow is one `noi_dung_mini_app` row, addressed BY COLUMN NAME.
//
// THIS IS WHAT MAKES THE COLUMN CHECK REAL: the fake assembles each row from the names the store
// actually asked for, so reordering contentItemColumns without reordering scanContentItem turns
// this suite red. A fake that returned a fixed tuple would agree with any order — and three of these
// columns (`nguon`, `nguon_url`, `nguon_id_ngoai`) are adjacent text, where a swap compiles, runs,
// and produces an article claiming to come from somewhere it did not.
type fakeContentRow struct {
	id, typ, title, status, source, authorCode       string
	categoryID, summary, image, sourceURL, sourceRef any // nil where the column is NULL
	body                                             any // `noi_dung`, nil on a row with no body
	publishedOn                                      time.Time
	viewCount                                        int64
	handEdited                                       bool
	createdAt, updatedAt                             time.Time
}

func (r fakeContentRow) value(col string) driver.Value {
	switch col {
	case "id":
		return r.id
	case "loai":
		return r.typ
	case "danh_muc_id":
		return r.categoryID
	case "tieu_de":
		return r.title
	case "tom_tat":
		return r.summary
	case "anh_dai_dien_url":
		return r.image
	case "ngay_dang":
		return r.publishedOn
	case "luot_xem":
		return r.viewCount
	case "trang_thai":
		return r.status
	case "nguon":
		return r.source
	case "nguon_url":
		return r.sourceURL
	case "nguon_id_ngoai":
		return r.sourceRef
	case "da_sua_tay":
		return r.handEdited
	case "nguoi_tao_ma":
		return r.authorCode
	case "tao_luc":
		return r.createdAt
	case "cap_nhat_luc":
		return r.updatedAt
	case "noi_dung":
		return r.body
	default:
		// LOUD, NOT ZERO. A silent zero here would let a column be added to contentItemColumns and
		// never actually be read by anything, while this suite "passed" and proved nothing about it.
		panic("driver giả nội dung Mini App: không có giá trị mẫu cho cột " + col)
	}
}

// fakeCategoryRow is one `danh_muc_mini_app` row, addressed BY COLUMN NAME for the same reason.
type fakeCategoryRow struct {
	id, name, slug string
	parentID       any
	sortOrder      int64
	createdAt      time.Time
}

func (r fakeCategoryRow) value(col string) driver.Value {
	switch col {
	case "id":
		return r.id
	case "ten":
		return r.name
	case "slug":
		return r.slug
	case "cha_id":
		return r.parentID
	case "thu_tu":
		return r.sortOrder
	case "tao_luc":
		return r.createdAt
	default:
		panic("driver giả danh mục Mini App: không có giá trị mẫu cho cột " + col)
	}
}

type fakeContentStore struct {
	stmts []recordedStmt

	rows        []fakeContentRow
	categories  []fakeCategoryRow
	countResult int64 // what a `SELECT count(*)` answers
	rowExists   bool  // what a `SELECT 1 …` existence probe answers

	rowsAffected int64 // what Exec reports as RowsAffected; 1 unless a test says otherwise
	err          error
}

func (f *fakeContentStore) Connect(context.Context) (driver.Conn, error) {
	return &fakeContentConn{f: f}, nil
}
func (f *fakeContentStore) Driver() driver.Driver { return fakeContentDriver{} }

type fakeContentDriver struct{}

func (fakeContentDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả nội dung Mini App: chỉ dùng Connector")
}

type fakeContentConn struct{ f *fakeContentStore }

func (c *fakeContentConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả nội dung Mini App: không hỗ trợ Prepare")
}
func (c *fakeContentConn) Close() error { return nil }
func (c *fakeContentConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeContentConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeContentTx{}, nil
}

type fakeContentTx struct{}

func (fakeContentTx) Commit() error   { return nil }
func (fakeContentTx) Rollback() error { return nil }

func (c *fakeContentConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.err != nil {
		return nil, c.f.err
	}
	n := c.f.rowsAffected
	if n == 0 && !strings.Contains(q, "UPDATE") {
		n = 1
	}
	return fakeResult{n: n}, nil
}

func (c *fakeContentConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.f.stmts = append(c.f.stmts, recordedStmt{sql: q, args: argValues(args)})
	if c.f.err != nil {
		return nil, c.f.err
	}

	// The aggregate and the existence probe are answered BEFORE the column reader, because their
	// SELECT lists are not column names and the row assembler would panic on them. Keeping the two
	// outcomes separable is the whole reason the store distinguishes them.
	if strings.Contains(q, "count(*)") {
		return &fakeWriteRows{cols: []string{"count"}, rows: [][]driver.Value{{c.f.countResult}}}, nil
	}
	if strings.Contains(q, "SELECT 1 FROM") {
		if !c.f.rowExists {
			return &fakeWriteRows{cols: []string{"?column?"}}, nil
		}
		return &fakeWriteRows{cols: []string{"?column?"}, rows: [][]driver.Value{{int64(1)}}}, nil
	}

	cols, err := selectColumnsWrite(q)
	if err != nil {
		return nil, err
	}
	if strings.Contains(q, "FROM danh_muc_mini_app") {
		out := make([][]driver.Value, 0, len(c.f.categories))
		for _, r := range c.f.categories {
			one := make([]driver.Value, len(cols))
			for i, name := range cols {
				one[i] = r.value(name)
			}
			out = append(out, one)
		}
		return &fakeWriteRows{cols: cols, rows: out}, nil
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

func newTestContentItemStore(t *testing.T, f *fakeContentStore) (*ContentItemStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewContentItemStore(pkgstore.New(db)), tenant.Into(context.Background(), contentTenant)
}

func newTestContentCategoryStore(t *testing.T, f *fakeContentStore) (*ContentCategoryStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewContentCategoryStore(pkgstore.New(db)), tenant.Into(context.Background(), contentTenant)
}

func contentFirstPage(t *testing.T) pkgpage.Request {
	t.Helper()
	req, err := pkgpage.New(ContentItemSorts, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return req
}

func sampleContentRow() fakeContentRow {
	return fakeContentRow{
		id: "nd-001", typ: "tin-tuc", title: "Xã Thăng Bình khai giảng năm học mới",
		categoryID: "dm-001", summary: "Sáng nay…", image: "https://x/a.png",
		publishedOn: sampleDate, viewCount: 7, status: "dang-hien",
		source: "dong-bo-cong", sourceURL: "https://cong/a", sourceRef: "cong-42",
		handEdited: true, authorCode: "CB-2026-7K3M9Q",
		createdAt: contentTime, updatedAt: contentTime, body: "<p>Toàn văn</p>",
	}
}

// --- the read path -------------------------------------------------------------------------

func TestListContentReadsRightColumnsInRightOrder(t *testing.T) {
	// THE ASSERTION THIS FILE EXISTS FOR. The row is assembled BY NAME from the columns the store
	// asked for, so a swap between contentItemColumns and scanContentItem lands the wrong value in
	// the wrong field — and `nguon` / `nguon_url` / `nguon_id_ngoai` are three adjacent text columns
	// the compiler cannot tell apart.
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	res, err := s.List(ctx, ContentItemFilter{}, contentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("số dòng = %d, muốn 1", len(res.Items))
	}
	one := res.Items[0]
	if one.ID != "nd-001" || one.Title != "Xã Thăng Bình khai giảng năm học mới" {
		t.Errorf("dòng = %+v", one)
	}
	if one.Type != domain.ContentTypeNews || one.Status != domain.ContentStatusVisible {
		t.Errorf("loại = %q, trạng thái = %q", one.Type, one.Status)
	}
	if one.Source != domain.ContentSourcePortalSync || one.SourceURL != "https://cong/a" || one.SourceRef != "cong-42" {
		t.Errorf("ba cột xuất xứ bị hoán vị: nguon=%q url=%q ma_ngoai=%q",
			one.Source, one.SourceURL, one.SourceRef)
	}
	if !one.HandEdited {
		t.Error("cờ §10.4 `da_sua_tay` không đọc được")
	}
	if one.ViewCount != 7 || !one.PublishedOn.Equal(sampleDate) {
		t.Errorf("lượt xem = %d, ngày đăng = %v", one.ViewCount, one.PublishedOn)
	}
	if one.AuthorCode != "CB-2026-7K3M9Q" {
		t.Errorf("người tạo = %q, muốn mã cán bộ", one.AuthorCode)
	}
}

func TestListContentCarriesNoBody(t *testing.T) {
	// THE PROPERTY THAT MAKES THE DETAIL ROUTE NECESSARY. A hundred articles at
	// domain.ContentBodyMaxLen is twenty million runes in one response, so the page must not select
	// `noi_dung` — and the fake driver PANICS on a column it has no sample for, which is how this
	// assertion also catches the column being added to the list by accident.
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	res, err := s.List(ctx, ContentItemFilter{}, contentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if res.Items[0].Body != "" {
		t.Errorf("trang danh sách mang toàn văn: %q", res.Items[0].Body)
	}
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "FROM noi_dung_mini_app") && strings.Contains(st.sql, "noi_dung,") {
			t.Errorf("câu đọc trang chọn cả `noi_dung`: %s", st.sql)
		}
	}
}

func TestListContentBindsTenantAndFiltersSoftDeleted(t *testing.T) {
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	if _, err := s.List(ctx, ContentItemFilter{}, contentFirstPage(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var stmt string
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "FROM noi_dung_mini_app") {
			stmt = st.sql
			if len(st.args) == 0 || st.args[0] != string(contentTenant) {
				t.Fatalf("tham số $1 = %v, muốn xã %v — xã đến từ ngữ cảnh (luật 1, bất biến 5)",
					st.args, contentTenant)
			}
		}
	}
	if stmt == "" {
		t.Fatal("không thấy câu đọc trang")
	}
	if !strings.Contains(stmt, "tenant_id = $1") {
		t.Errorf("câu đọc trang thiếu `tenant_id = $1`: %s", stmt)
	}
	// RULE 7, INVARIANT 2 — everywhere, always. Both partial indexes of migration 0006 are built on
	// exactly this predicate.
	if !strings.Contains(stmt, "deleted_at IS NULL") {
		t.Errorf("câu đọc trang thiếu `deleted_at IS NULL`: %s", stmt)
	}
	// §6 shows the newest first, and the `id` tie-break is what makes the order TOTAL — without it
	// two items written in the same millisecond let page two repeat one and drop the other.
	if !strings.Contains(stmt, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự sai: %s", stmt)
	}
}

func TestListContentThreeFiltersBecomeParamsInOrder(t *testing.T) {
	// §6's filter bar: the tab, the category select and the title box. THE NUMBERING IS THE
	// ASSERTION: store.QueryPage binds PageSpec.Args from $2 in slice order, so a mismatch between
	// the `$n` written into the predicate and the position in the slice binds the title pattern to
	// the category — which returns an empty list for every request, with no error anywhere.
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	_, err := s.List(ctx, ContentItemFilter{Type: "banner", CategoryID: "dm-9", Search: "khai giảng"},
		contentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	st := f.stmts[0]
	for _, want := range []string{"loai = $2", "danh_muc_id = $3", "tieu_de ILIKE $4"} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("câu thiếu %q: %s", want, st.sql)
		}
	}
	if len(st.args) < 5 {
		t.Fatalf("số tham số = %d, muốn ít nhất 5 (xã, loại, danh mục, tiêu đề, limit)", len(st.args))
	}
	if st.args[0] != string(contentTenant) || st.args[1] != "banner" || st.args[2] != "dm-9" {
		t.Errorf("tham số = %v", st.args[:3])
	}
	if st.args[3] != "%khai giảng%" {
		t.Errorf("mẫu tìm = %v, muốn %q", st.args[3], "%khai giảng%")
	}
}

func TestListContentEscapesWildcardsInSearch(t *testing.T) {
	// WITHOUT THIS, A `%` TYPED IN THE SEARCH BOX MATCHES EVERYTHING and `_` matches any character.
	// The box is the commune's own search, so the failure is not an attack — it is a member of staff
	// who pasted a title containing an underscore and got back rows they did not ask for.
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	if _, err := s.List(ctx, ContentItemFilter{Search: `100%_ke\hoach`}, contentFirstPage(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	pattern, _ := f.stmts[0].args[1].(string)
	if pattern != `%100\%\_ke\\hoach%` {
		t.Errorf("mẫu tìm = %q, muốn ba ký tự đại diện đã được thoát", pattern)
	}
}

func TestListContentNoFilterAddsNoParam(t *testing.T) {
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	if _, err := s.List(ctx, ContentItemFilter{Search: "   "}, contentFirstPage(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	st := f.stmts[0]
	// A BLANK SEARCH BOX IS NOT A FILTER. `ILIKE '%%'` would match every row and cost a scan for
	// nothing, on the request every screen makes when it opens.
	if strings.Contains(st.sql, "ILIKE") {
		t.Errorf("từ khoá toàn khoảng trắng mà vẫn sinh mệnh đề ILIKE: %s", st.sql)
	}
	if len(st.args) != 2 { // the commune and the limit
		t.Errorf("số tham số = %d, muốn 2 (xã, limit)", len(st.args))
	}
}

func TestContentItemSortsAcceptOnlyCreatedAt(t *testing.T) {
	// THE ALLOWLIST IS THE WHOLE SURFACE a client may sort on. `ngay_dang` is deliberately not on it:
	// it is a DATE, so one sync run importing four hundred articles published the same day gives four
	// hundred rows the same sort value and the `id` tie-break silently carries the whole order.
	for _, col := range []string{"published_on", "ngay_dang", "view_count", "title"} {
		if _, err := pkgpage.New(ContentItemSorts, col, "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
			t.Errorf("sắp xếp theo %q: lỗi = %v, muốn ErrSort", col, err)
		}
	}
}

func TestByIDReadsBodyAndFiltersSoftDeleted(t *testing.T) {
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	c, err := s.ByID(ctx, "nd-001")
	if err != nil {
		t.Fatalf("đọc chi tiết lỗi: %v", err)
	}
	if c.Body != "<p>Toàn văn</p>" {
		t.Errorf("toàn văn = %q — tuyến chi tiết tồn tại chính vì trang danh sách không mang nó", c.Body)
	}
	stmt := f.stmts[0].sql
	for _, want := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL", "noi_dung"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("câu đọc chi tiết thiếu %q: %s", want, stmt)
		}
	}
}

func TestByIDNoRowReportsNotFound(t *testing.T) {
	// 404 AND NOT 403 FOR ANOTHER COMMUNE'S ITEM, indistinguishably: the predicate binds the commune
	// to $1, so an id belonging to another authority is simply not there. Telling the two apart would
	// confirm what that authority holds.
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	if _, err := s.ByID(ctx, "nd-cua-xa-khac"); !errors.Is(err, ErrContentItemNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrContentItemNotFound", err)
	}
}

// --- the write path -------------------------------------------------------------------------

func runContentTx(t *testing.T, f *fakeContentStore, ctx context.Context,
	fn func(tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return pkgstore.New(db).For(ctx).Tx(ctx, fn)
}

func TestInsertContentWritesSourceAsLiteralAndTakesNoExternalRef(t *testing.T) {
	// THE SECURITY PROPERTY OF THIS STATEMENT, not a shortcut: there is no `$n` for `nguon`,
	// `nguon_id_ngoai`, `nguon_url` or `da_sua_tay`, so no layer above can pass one and no client can
	// fill one. That is what keeps §10.4's protection — a hand-edited portal article survives the next
	// sync — out of reach of a request body.
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.ContentItem{
			ID: "nd-moi", Type: domain.ContentTypeNews, Title: "Tiêu đề",
			PublishedOn: sampleDate, Status: domain.ContentStatusHidden, AuthorCode: "CB-2026-7K3M9Q",
		})
	})
	if err != nil {
		t.Fatalf("chèn lỗi: %v", err)
	}

	var stmt recordedStmt
	for _, st := range f.stmts {
		if strings.Contains(st.sql, "INSERT INTO noi_dung_mini_app") {
			stmt = st
		}
	}
	if stmt.sql == "" {
		t.Fatal("không thấy câu chèn")
	}
	if !strings.Contains(stmt.sql, "'thu-cong'") {
		t.Errorf("`nguon` phải là hằng trong câu lệnh: %s", stmt.sql)
	}
	for _, banned := range []string{"nguon_id_ngoai", "nguon_url", "da_sua_tay", "luot_xem",
		"deleted_at", "deleted_by", "delete_reason"} {
		if strings.Contains(stmt.sql, banned) {
			t.Errorf("câu chèn không được nhận %q: %s", banned, stmt.sql)
		}
	}
	if stmt.args[0] != string(contentTenant) {
		t.Errorf("xã = %v, muốn %v — xã đến từ giao dịch, không từ tham số", stmt.args[0], contentTenant)
	}
}

func TestInsertContentTurnsEmptyStringsIntoNull(t *testing.T) {
	// `danh_muc_id` HAS A FOREIGN KEY AND '' IS NOT A CATEGORY ID — the constraint would refuse it,
	// and the ordinary row is precisely the one with no category: §7's `— Chưa xếp danh mục —`.
	f := &fakeContentStore{}
	s, ctx := newTestContentItemStore(t, f)

	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.ContentItem{
			ID: "nd-moi", Type: domain.ContentTypeNews, Title: "Tiêu đề",
			PublishedOn: sampleDate, Status: domain.ContentStatusHidden, AuthorCode: "CB-1",
		})
	})
	if err != nil {
		t.Fatalf("chèn lỗi: %v", err)
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 ngày đăng, $10 trạng thái, $11 người tạo.
	args := f.stmts[0].args
	if len(args) != 11 {
		t.Fatalf("số tham số = %d, muốn 11", len(args))
	}
	for _, i := range []int{3, 5, 6, 7} {
		if args[i] != nil {
			t.Errorf("tham số $%d = %v, muốn NULL cho chuỗi rỗng", i+1, args[i])
		}
	}
}

func TestUpdateContentCannotTouchProvenanceOrAuthor(t *testing.T) {
	// SIX COLUMNS ARE REFUSED BY THIS STATEMENT'S SHAPE, and the trigger in migration 0006 refuses
	// them again. Both layers are meant: the trigger is the floor that holds against every writer, and
	// their absence here is what makes the floor unreachable from this service in the first place.
	f := &fakeContentStore{rowsAffected: 1}
	s, ctx := newTestContentItemStore(t, f)

	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Update(ctx, tx, domain.ContentItem{
			ID: "nd-001", Type: domain.ContentTypeNews, Title: "Tiêu đề mới",
			Status: domain.ContentStatusVisible, HandEdited: true,
		})
	})
	if err != nil {
		t.Fatalf("cập nhật lỗi: %v", err)
	}
	stmt := f.stmts[0].sql
	for _, banned := range []string{"nguon =", "nguon_id_ngoai", "nguoi_tao_ma", "tao_luc =",
		"luot_xem", "deleted_at =", "ngay_dang ="} {
		if strings.Contains(stmt, banned) {
			t.Errorf("câu cập nhật không được ghi %q: %s", banned, stmt)
		}
	}
	// `da_sua_tay` IS THE ONE PROVENANCE COLUMN THIS STATEMENT DOES WRITE — §10.4 is recorded here.
	if !strings.Contains(stmt, "da_sua_tay = $10") {
		t.Errorf("câu cập nhật phải ghi `da_sua_tay` (§10.4): %s", stmt)
	}
	// `AND deleted_at IS NULL` IS WHAT MAKES EDITING A DELETED ITEM A 404 rather than a resurrection.
	if !strings.Contains(stmt, "deleted_at IS NULL") {
		t.Errorf("câu cập nhật thiếu `deleted_at IS NULL`: %s", stmt)
	}
}

func TestUpdateNoRowReportsNotFound(t *testing.T) {
	// An UPDATE touching zero rows is not an error to PostgreSQL; it is only an error to us. Without
	// this check a method used without the locked read would report success for a row that is gone.
	f := &fakeContentStore{rowsAffected: 0}
	s, ctx := newTestContentItemStore(t, f)

	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Update(ctx, tx, domain.ContentItem{ID: "nd-da-xoa", Type: domain.ContentTypeNews})
	})
	if !errors.Is(err, ErrContentItemNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrContentItemNotFound", err)
	}
}

func TestCategoryExistsFiltersSoftDeleted(t *testing.T) {
	// THE FOREIGN KEY DOES NOT COVER THIS CASE: a soft-deleted category is still a row, so the
	// constraint accepts it. Filing a new article under a category the commune retired last week is a
	// mistake only this predicate catches.
	f := &fakeContentStore{rowExists: true}
	s, ctx := newTestContentItemStore(t, f)

	_, err := runContentTxResult(t, f, ctx, func(tx *pkgstore.ScopedTx) (bool, error) {
		return s.CategoryExists(ctx, tx, "dm-001")
	})
	if err != nil {
		t.Fatalf("kiểm danh mục lỗi: %v", err)
	}
	stmt := f.stmts[0].sql
	if !strings.Contains(stmt, "deleted_at IS NULL") || !strings.Contains(stmt, "tenant_id = $1") {
		t.Errorf("câu kiểm danh mục = %s", stmt)
	}
}

// runContentTxResult is the same helper with a value coming back out of the closure.
func runContentTxResult[T any](t *testing.T, f *fakeContentStore, ctx context.Context,
	fn func(tx *pkgstore.ScopedTx) (T, error)) (T, error) {
	t.Helper()
	var out T
	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		out, err = fn(tx)
		return err
	})
	return out, err
}

// --- the category tree ---------------------------------------------------------------------------

func TestListCategoriesReadsRightColumnsInRightOrder(t *testing.T) {
	f := &fakeContentStore{categories: []fakeCategoryRow{
		{id: "dm-001", name: "Chuyển đổi số", slug: "chuyen-doi-so", parentID: "dm-goc",
			sortOrder: 2, createdAt: contentTime},
		{id: "dm-goc", name: "Danh mục", slug: "danh-muc", parentID: nil, sortOrder: 1, createdAt: contentTime},
	}}
	s, ctx := newTestContentCategoryStore(t, f)

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("đọc danh mục lỗi: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2", len(list))
	}
	// `ten` and `slug` are adjacent TEXT columns, as are `id` and `cha_id`: swapping either pair
	// compiles, runs, and produces a tree whose every label is a slug or whose every node is its own
	// parent.
	if list[0].Name != "Chuyển đổi số" || list[0].Slug != "chuyen-doi-so" {
		t.Errorf("tên/slug bị hoán vị: %+v", list[0])
	}
	if list[0].ParentID != "dm-goc" {
		t.Errorf("cha = %q, muốn dm-goc", list[0].ParentID)
	}
	// A ROOT CATEGORY HAS A NULL PARENT and it must land as "" rather than crash the scan.
	if list[1].ParentID != "" {
		t.Errorf("danh mục gốc có cha = %q, muốn rỗng", list[1].ParentID)
	}

	stmt := f.stmts[0].sql
	if !strings.Contains(stmt, "ORDER BY thu_tu, slug") {
		t.Errorf("thứ tự sai — `slug` là thứ làm cho thứ tự TOÀN PHẦN: %s", stmt)
	}
	if !strings.Contains(stmt, "deleted_at IS NULL") {
		t.Errorf("câu đọc danh mục thiếu `deleted_at IS NULL`: %s", stmt)
	}
	// LIMIT IS THE CEILING PLUS ONE, which is what makes "there are too many" detectable at all.
	if a := f.stmts[0].args; len(a) != 2 || a[1] != int64(ContentCategoryCeiling+1) {
		t.Errorf("tham số = %v, muốn [xã, %d]", a, ContentCategoryCeiling+1)
	}
}

func TestSlugTakenCountsSoftDeletedRows(t *testing.T) {
	// RULE 7, INVARIANT 3, AS A PREDICATE: `deleted_at` is deliberately ABSENT here. A commune that
	// could soft-delete `chuyen-doi-so` and create a new, unrelated `chuyen-doi-so` would silently
	// refile every article already filed under the old one.
	f := &fakeContentStore{countResult: 1}
	s, ctx := newTestContentCategoryStore(t, f)

	taken, err := runContentTxResult(t, f, ctx, func(tx *pkgstore.ScopedTx) (bool, error) {
		return s.SlugTaken(ctx, tx, "chuyen-doi-so")
	})
	if err != nil {
		t.Fatalf("kiểm slug lỗi: %v", err)
	}
	if !taken {
		t.Error("slug đã dùng mà báo chưa")
	}
	stmt := f.stmts[0].sql
	if strings.Contains(stmt, "deleted_at") {
		t.Errorf("câu kiểm slug KHÔNG được loại dòng đã xoá mềm — mã đã cấp không cấp lại: %s", stmt)
	}
	if !strings.Contains(stmt, "tenant_id = $1") {
		t.Errorf("câu kiểm slug thiếu `tenant_id = $1`: %s", stmt)
	}
}

func TestInsertCategoryBindsTenantAndTurnsEmptyParentIntoNull(t *testing.T) {
	f := &fakeContentStore{}
	s, ctx := newTestContentCategoryStore(t, f)

	err := runContentTx(t, f, ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.ContentCategory{
			ID: "dm-moi", Name: "Chuyển đổi số", Slug: "chuyen-doi-so", ParentID: "", SortOrder: 3,
		})
	})
	if err != nil {
		t.Fatalf("chèn danh mục lỗi: %v", err)
	}
	args := f.stmts[0].args
	if args[0] != string(contentTenant) {
		t.Errorf("xã = %v, muốn %v", args[0], contentTenant)
	}
	// A ROOT CATEGORY HAS NO PARENT, and '' would fail the self-referencing foreign key.
	if args[4] != nil {
		t.Errorf("cha rỗng = %v, muốn NULL", args[4])
	}
}
