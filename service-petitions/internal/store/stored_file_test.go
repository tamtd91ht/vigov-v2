package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// StoredFileStore on a fake driver that records every statement and answers reads BY COLUMN NAME.
//
// PROVED HERE   the commune is $1 and comes from the CONTEXT · the expected current status is in
//               every transition's WHERE · zero rows affected is ErrStoredFileMoved · a non-edge,
//               `stored` and `purged` are refused before any SQL · `pending` is a literal · the link
//               is ONE statement with $1/$2 in every tuple · an empty/duplicate list writes nothing ·
//               the batched read binds the ids as ONE array, constrains BOTH tables to $1 and
//               excludes soft-deleted files · the positional Scans line up with the column lists.
// NOT PROVED    anything PostgreSQL does: the CHECKs, the guard trigger, the link trigger, the
//               append-only trigger, ANY($2) on a real array. stored_file_pg_test.go is for that,
//               and it SKIPS without VIGOV_TEST_DSN.

type sfStmt struct {
	sql  string
	args []driver.Value
}

type sfDB struct {
	stmts    []sfStmt
	affected int64                     // RowsAffected of every Exec
	rows     []map[string]driver.Value // rows of every Query, assembled by column NAME
}

type sfConnector struct{ d *sfDB }

func (c sfConnector) Connect(context.Context) (driver.Conn, error) { return &sfConn{d: c.d}, nil }
func (c sfConnector) Driver() driver.Driver                        { return sfDriver{} }

type sfDriver struct{}

func (sfDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type sfConn struct{ d *sfDB }

func (c *sfConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *sfConn) Close() error                        { return nil }
func (c *sfConn) Begin() (driver.Tx, error)           { return sfTx{}, nil }
func (c *sfConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return sfTx{}, nil
}

// CheckNamedValue accepts every argument as is — a []string included, as pgx's stdlib does. Without
// it database/sql refuses the slice before the statement is recorded, and the batched read would be
// untestable here.
func (c *sfConn) CheckNamedValue(*driver.NamedValue) error { return nil }

func sfVals(args []driver.NamedValue) []driver.Value {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	return v
}

func (c *sfConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.stmts = append(c.d.stmts, sfStmt{sql: q, args: sfVals(args)})
	return driver.RowsAffected(c.d.affected), nil
}

func (c *sfConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.stmts = append(c.d.stmts, sfStmt{sql: q, args: sfVals(args)})
	cols, err := cotTrongCauLenh(q)
	if err != nil {
		return nil, err
	}
	out := make([][]driver.Value, 0, len(c.d.rows))
	for _, r := range c.d.rows {
		one := make([]driver.Value, len(cols))
		for i, col := range cols {
			v, ok := r[col]
			if !ok {
				panic("fake driver: no fixture value for column " + col)
			}
			one[i] = v
		}
		out = append(out, one)
	}
	return &sfRows{cols: cols, rows: out}, nil
}

type sfTx struct{}

func (sfTx) Commit() error   { return nil }
func (sfTx) Rollback() error { return nil }

type sfRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *sfRows) Columns() []string { return r.cols }
func (r *sfRows) Close() error      { return nil }
func (r *sfRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func sfStore(d *sfDB) (*StoredFileStore, *pkgstore.DB) {
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	return NewStoredFileStore(h), h
}

// sfInTx runs fn in a scoped transaction of `commune`.
func sfInTx(t *testing.T, h *pkgstore.DB, commune tenant.ID, fn func(context.Context, *pkgstore.ScopedTx) error) error {
	t.Helper()
	ctx := ctxXa(commune)
	return h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(ctx, tx) })
}

var sfAt = time.Date(2026, 9, 29, 3, 4, 5, 0, time.UTC)

func sfPending() domain.StoredFile {
	return domain.StoredFile{
		ID: "01JBFILE0000000000000000AA", Bucket: domain.StoredFileBucketPrivate,
		ObjectKey:      "records/t_x/2026/09/petitions/task-attachment/01jbfile0000000000000000aa/original.pdf",
		RetentionClass: "records", Purpose: "task-attachment",
		SubjectType: domain.StoredFileSubjectTask, SubjectID: "nv-7",
		OriginalName: "bien-ban.pdf", UploadedBy: "CB-00311", CreatedAt: sfAt,
	}
}

func TestInsertPendingBindsCommuneAndLiteralPending(t *testing.T) {
	d := &sfDB{affected: 1}
	s, h := sfStore(d)
	f := sfPending()
	// A status on the input must be ignored: the literal decides.
	f.Status = domain.StoredFileStored
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.InsertPending(ctx, tx, f)
	}); err != nil {
		t.Fatalf("InsertPending: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(d.stmts))
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "'pending'") || strings.Contains(st.sql, "'stored'") {
		t.Errorf("status is not the literal 'pending': %s", st.sql)
	}
	want := []driver.Value{string(xaThu), f.ID, f.Bucket, f.ObjectKey, f.RetentionClass, f.Purpose,
		f.SubjectType, f.SubjectID, f.OriginalName, f.UploadedBy, f.CreatedAt}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v\nwant   %v", st.args, want)
	}
}

func TestTransitionCarriesExpectedStatusInWhere(t *testing.T) {
	d := &sfDB{affected: 1}
	s, h := sfStore(d)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Transition(ctx, tx, "f1", domain.StoredFilePending, domain.StoredFileScanning, sfAt)
	}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	st := d.stmts[0]
	for _, frag := range []string{"WHERE tenant_id = $1 AND id = $2 AND status = $3", "deleted_at IS NULL", "SET status = $4"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	want := []driver.Value{string(xaThu), "f1", "pending", "scanning", sfAt}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}
}

func TestTransitionZeroRowsIsMoved(t *testing.T) {
	d := &sfDB{affected: 0}
	s, h := sfStore(d)
	err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Transition(ctx, tx, "f1", domain.StoredFileScanning, domain.StoredFileRejected, sfAt)
	})
	if !errors.Is(err, ErrStoredFileMoved) {
		t.Fatalf("err = %v, want ErrStoredFileMoved", err)
	}
}

// A NON-EDGE, AND THE TWO EDGES THAT HAVE THEIR OWN METHOD, RUN NO SQL AT ALL.
func TestTransitionRefusesBeforeSQL(t *testing.T) {
	for _, c := range []struct{ from, to domain.StoredFileStatus }{
		{domain.StoredFilePending, domain.StoredFileStored},  // skips the scan
		{domain.StoredFileScanning, domain.StoredFileStored}, // an edge, but MarkStored's
		{domain.StoredFileStored, domain.StoredFilePurged},   // an edge, but the purge worker's
		{domain.StoredFilePurged, domain.StoredFilePending},  // nothing leaves purged
		{domain.StoredFileRejected, domain.StoredFileStored}, // a rejected file is never stored
		{domain.StoredFilePending, domain.StoredFilePending}, // not a move
	} {
		d := &sfDB{affected: 1}
		s, h := sfStore(d)
		err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			return s.Transition(ctx, tx, "f1", c.from, c.to, sfAt)
		})
		if !errors.Is(err, ErrStoredFileEdge) {
			t.Errorf("%s → %s: err = %v, want ErrStoredFileEdge", c.from, c.to, err)
		}
		if len(d.stmts) != 0 {
			t.Errorf("%s → %s: ran %d statements, want 0", c.from, c.to, len(d.stmts))
		}
	}
}

func TestMarkStoredOnlyFromScanning(t *testing.T) {
	d := &sfDB{affected: 1}
	s, h := sfStore(d)
	facts := domain.StoredFileFacts{MIMEType: "application/pdf", SizeBytes: 12345, SHA256: strings.Repeat("ab", 32)}
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.MarkStored(ctx, tx, "f1", facts, sfAt)
	}); err != nil {
		t.Fatalf("MarkStored: %v", err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "status = 'scanning' AND deleted_at IS NULL") ||
		!strings.Contains(st.sql, "WHERE tenant_id = $1 AND id = $2") {
		t.Errorf("MarkStored WHERE does not pin commune, row and 'scanning': %s", st.sql)
	}
	want := []driver.Value{string(xaThu), "f1", facts.MIMEType, facts.SizeBytes, facts.SHA256, sfAt}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}

	d.affected = 0
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.MarkStored(ctx, tx, "f1", facts, sfAt)
	}); !errors.Is(err, ErrStoredFileMoved) {
		t.Errorf("second completion: err = %v, want ErrStoredFileMoved", err)
	}
}

func TestLinkToLogEntryOneStatement(t *testing.T) {
	d := &sfDB{affected: 3}
	s, h := sfStore(d)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.LinkToLogEntry(ctx, tx, "nk-1", []string{"f1", "f2", "f3"})
	}); err != nil {
		t.Fatalf("LinkToLogEntry: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(d.stmts))
	}
	st := d.stmts[0]
	if !strings.HasSuffix(st.sql, "VALUES ($1, $2, $3), ($1, $2, $4), ($1, $2, $5)") {
		t.Errorf("tuples do not all carry $1 (commune) and $2 (entry): %s", st.sql)
	}
	want := []driver.Value{string(xaThu), "nk-1", "f1", "f2", "f3"}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}
}

func TestLinkToLogEntryRefusesBadListsWithoutSQL(t *testing.T) {
	for _, c := range []struct {
		entry string
		ids   []string
		want  error
	}{
		{"nk-1", nil, nil}, // nothing to attach: no-op
		{"nk-1", []string{"f1", "f1"}, ErrAttachmentList}, // one file twice
		{"nk-1", []string{"f1", ""}, ErrAttachmentList},
		{"", []string{"f1"}, ErrAttachmentList},
	} {
		d := &sfDB{affected: 1}
		s, h := sfStore(d)
		err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			return s.LinkToLogEntry(ctx, tx, c.entry, c.ids)
		})
		if !errors.Is(err, c.want) {
			t.Errorf("%q %v: err = %v, want %v", c.entry, c.ids, err, c.want)
		}
		if len(d.stmts) != 0 {
			t.Errorf("%q %v: ran %d statements, want 0", c.entry, c.ids, len(d.stmts))
		}
	}
}

func TestAttachmentsByLogEntriesBatchedAndScoped(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{
		{"a.log_entry_id": "nk-1", "f.id": "f1", "f.original_name": "a.pdf", "f.mime_type": "application/pdf", "f.size_bytes": int64(10), "f.status": "stored"},
		{"a.log_entry_id": "nk-1", "f.id": "f2", "f.original_name": "b.png", "f.mime_type": "image/png", "f.size_bytes": int64(20), "f.status": "purged"},
		{"a.log_entry_id": "nk-3", "f.id": "f3", "f.original_name": "c.jpg", "f.mime_type": "image/jpeg", "f.size_bytes": int64(30), "f.status": "ready"},
	}}
	s, _ := sfStore(d)
	got, err := s.AttachmentsByLogEntries(ctxXa(xaThu), []string{"nk-1", "nk-2", "nk-3"})
	if err != nil {
		t.Fatalf("AttachmentsByLogEntries: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements, want ONE for the whole page", len(d.stmts))
	}
	st := d.stmts[0]
	for _, frag := range []string{"WHERE a.tenant_id = $1", "ON f.tenant_id = $1 AND f.id = a.stored_file_id",
		"a.log_entry_id = ANY($2)", "f.deleted_at IS NULL"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	if len(st.args) != 2 || st.args[0] != string(xaThu) || !reflect.DeepEqual(st.args[1], []string{"nk-1", "nk-2", "nk-3"}) {
		t.Errorf("args = %v, want [commune, ids as one array]", st.args)
	}
	want := map[string][]domain.TaskLogAttachment{
		"nk-1": {
			{LogEntryID: "nk-1", FileID: "f1", OriginalName: "a.pdf", MIMEType: "application/pdf", SizeBytes: 10, Status: domain.StoredFileStored},
			{LogEntryID: "nk-1", FileID: "f2", OriginalName: "b.png", MIMEType: "image/png", SizeBytes: 20, Status: domain.StoredFilePurged},
		},
		"nk-3": {{LogEntryID: "nk-3", FileID: "f3", OriginalName: "c.jpg", MIMEType: "image/jpeg", SizeBytes: 30, Status: domain.StoredFileReady}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestAttachmentsByLogEntriesBounds(t *testing.T) {
	d := &sfDB{}
	s, _ := sfStore(d)
	got, err := s.AttachmentsByLogEntries(ctxXa(xaThu), nil)
	if err != nil || got == nil || len(got) != 0 || len(d.stmts) != 0 {
		t.Errorf("empty page: got %v, err %v, %d statements — want an empty map and no query", got, err, len(d.stmts))
	}
	many := make([]string, MaxLogEntriesPerAttachmentRead+1)
	for i := range many {
		many[i] = "nk-" + strconv.Itoa(i)
	}
	if _, err := s.AttachmentsByLogEntries(ctxXa(xaThu), many); !errors.Is(err, ErrAttachmentList) {
		t.Errorf("over the ceiling: err = %v, want ErrAttachmentList", err)
	}
	if _, err := s.AttachmentsByLogEntries(ctxXa(xaThu), []string{"nk-1", "nk-1"}); !errors.Is(err, ErrAttachmentList) {
		t.Errorf("duplicate id: err = %v, want ErrAttachmentList", err)
	}
	if len(d.stmts) != 0 {
		t.Errorf("a refused read ran %d statements", len(d.stmts))
	}
}

// NO COMMUNE IN THE CONTEXT PANICS (core/tenant.MustFrom) rather than reading with a default.
func TestAttachmentsByLogEntriesWithoutCommunePanics(t *testing.T) {
	s, _ := sfStore(&sfDB{})
	defer func() {
		if recover() == nil {
			t.Error("no commune in the context did not panic")
		}
	}()
	_, _ = s.AttachmentsByLogEntries(context.Background(), []string{"nk-1"})
}

func TestForUpdateScansByNameAndLocks(t *testing.T) {
	retain := sfAt.Add(24 * time.Hour)
	d := &sfDB{rows: []map[string]driver.Value{{
		"id": "f1", "bucket": "private", "object_key": "k", "retention_class": "records",
		"purpose": "task-attachment", "subject_type": "task", "subject_id": "nv-7",
		"original_name": "x.pdf", "mime_type": nil, "size_bytes": nil, "sha256": nil,
		"status": "pending", "uploaded_by": "CB-00311", "retain_until": retain, "legal_hold": true,
		"created_at": sfAt, "updated_at": sfAt.Add(time.Minute),
	}}}
	s, h := sfStore(d)
	var got *domain.StoredFile
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.ForUpdate(ctx, tx, "f1")
		return err
	}); err != nil {
		t.Fatalf("ForUpdate: %v", err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE") {
		t.Errorf("not a scoped, live, locking read: %s", st.sql)
	}
	if st.args[0] != string(xaThu) || st.args[1] != "f1" {
		t.Errorf("args = %v", st.args)
	}
	want := &domain.StoredFile{ID: "f1", Bucket: "private", ObjectKey: "k", RetentionClass: "records",
		Purpose: "task-attachment", SubjectType: "task", SubjectID: "nv-7", OriginalName: "x.pdf",
		Status: domain.StoredFilePending, UploadedBy: "CB-00311", RetainUntil: retain, LegalHold: true,
		CreatedAt: sfAt, UpdatedAt: sfAt.Add(time.Minute)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	d.rows = nil
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.ForUpdate(ctx, tx, "f-missing")
		return err
	}); err != nil || got != nil {
		t.Errorf("missing row: got %v, err %v — want nil, nil", got, err)
	}
}
