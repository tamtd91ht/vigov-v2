package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The reads the attachment use case adds (app/task_attachment.go), on the stored_file_test.go fake.
//
// PROVED HERE   the commune is $1 in every statement and comes from the context · the live-row filter
//               is in every read · the per-subject count names subject, purpose and the live statuses,
//               and counts pending rows only when asked · the candidate read is ONE statement over an
//               IN list, constrains BOTH tables to $1, locks the file rows only, and scans the link.
// NOT PROVED    the SQL on PostgreSQL (the LEFT JOIN, FOR UPDATE OF on a partitioned table) —
//               stored_file_pg_test.go SKIPS without VIGOV_TEST_DSN.

func sfRow(id, status string) map[string]driver.Value {
	return map[string]driver.Value{
		"id": id, "bucket": "private", "object_key": "k-" + id, "retention_class": "records",
		"purpose": "task-attachment", "subject_type": "task", "subject_id": "nv-7",
		"original_name": "bien-ban.pdf", "mime_type": "application/pdf", "size_bytes": int64(42),
		"sha256": strings.Repeat("a", 64), "status": status, "uploaded_by": "CB-00311",
		"retain_until": nil, "legal_hold": false, "created_at": sfAt, "updated_at": sfAt,
	}
}

func TestByIDScopedLiveUnlocked(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{sfRow("f1", "stored")}}
	s, _ := sfStore(d)
	got, err := s.ByID(ctxXa(xaThu), "f1")
	if err != nil || got == nil {
		t.Fatalf("ByID: %v, %v", got, err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "FROM stored_file WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL") {
		t.Errorf("not a scoped live read: %s", st.sql)
	}
	if strings.Contains(st.sql, "FOR UPDATE") {
		t.Errorf("the pre-read locks: %s", st.sql)
	}
	if st.args[0] != string(xaThu) || st.args[1] != "f1" {
		t.Errorf("args = %v", st.args)
	}
	if got.Status != domain.StoredFileStored || got.SizeBytes != 42 || got.UploadedBy != "CB-00311" {
		t.Errorf("row = %+v", got)
	}

	d.rows = nil
	if got, err := s.ByID(ctxXa(xaThu), "f-missing"); err != nil || got != nil {
		t.Errorf("missing row: %v, %v — want nil, nil", got, err)
	}
}

func TestLinkedLogEntry(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"log_entry_id": "nk-9"}}}
	s, _ := sfStore(d)
	got, err := s.LinkedLogEntry(ctxXa(xaThu), "f1")
	if err != nil || got != "nk-9" {
		t.Fatalf("LinkedLogEntry = %q, %v", got, err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "FROM task_log_attachment WHERE tenant_id = $1 AND stored_file_id = $2") ||
		st.args[0] != string(xaThu) || st.args[1] != "f1" {
		t.Errorf("statement %s args %v", st.sql, st.args)
	}
	d.rows = nil
	if got, err := s.LinkedLogEntry(ctxXa(xaThu), "f1"); err != nil || got != "" {
		t.Errorf("unlinked: %q, %v", got, err)
	}
}

func TestCountForSubject(t *testing.T) {
	since := sfAt.Add(-15 * time.Minute)
	for _, c := range []struct {
		name        string
		since       time.Time
		inTx        bool
		wantPending bool
	}{
		{"request, in the task's transaction", since, true, true},
		{"completion pre-check, no pending", time.Time{}, false, false},
		{"no pending, in a transaction", time.Time{}, true, false},
		{"pending, outside a transaction", since, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := &sfDB{rows: []map[string]driver.Value{{"count(*)": int64(3)}}}
			s, h := sfStore(d)
			var (
				n   int
				err error
			)
			if c.inTx {
				err = sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
					var e error
					n, e = s.CountForSubjectTx(ctx, tx, "task", "nv-7", "task-attachment", c.since)
					return e
				})
			} else {
				n, err = s.CountForSubject(ctxXa(xaThu), "task", "nv-7", "task-attachment", c.since)
			}
			if err != nil || n != 3 {
				t.Fatalf("count = %d, %v", n, err)
			}
			st := d.stmts[0]
			for _, frag := range []string{"FROM stored_file WHERE tenant_id = $1", "subject_type = $2",
				"subject_id = $3", "purpose = $4", "deleted_at IS NULL", "'stored', 'processing', 'ready'"} {
				if !strings.Contains(st.sql, frag) {
					t.Errorf("statement lacks %q: %s", frag, st.sql)
				}
			}
			for _, bad := range []string{"'failed'", "'rejected'", "'purged'"} {
				if strings.Contains(st.sql, bad) {
					t.Errorf("a refused status %s counts against the limit: %s", bad, st.sql)
				}
			}
			want := []driver.Value{string(xaThu), "task", "nv-7", "task-attachment"}
			if c.wantPending {
				if !strings.Contains(st.sql, "created_at >= $5") {
					t.Errorf("pending rows counted without the freshness bound: %s", st.sql)
				}
				want = append(want, c.since)
			} else if strings.Contains(st.sql, "'pending'") {
				t.Errorf("pending rows counted where none must be: %s", st.sql)
			}
			if !reflect.DeepEqual(st.args, want) {
				t.Errorf("args = %v\nwant   %v", st.args, want)
			}
		})
	}
}

func TestAttachCandidatesOneLockedScopedStatement(t *testing.T) {
	linked := sfRow("f2", "stored")
	linked["a.log_entry_id"] = "nk-1"
	free := sfRow("f1", "stored")
	free["a.log_entry_id"] = nil
	d := &sfDB{rows: []map[string]driver.Value{free, linked}}
	s, h := sfStore(d)

	var got map[string]domain.AttachCandidate
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.AttachCandidates(ctx, tx, []string{"f1", "f2", "f-other-commune"})
		return err
	}); err != nil {
		t.Fatalf("AttachCandidates: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements, want ONE", len(d.stmts))
	}
	st := d.stmts[0]
	for _, frag := range []string{"WHERE f.tenant_id = $1", "ON a.tenant_id = $1 AND a.stored_file_id = f.id",
		"f.deleted_at IS NULL", "f.id IN ($2, $3, $4)", "FOR UPDATE OF f"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	if want := []driver.Value{string(xaThu), "f1", "f2", "f-other-commune"}; !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}
	if len(got) != 2 || got["f1"].LinkedTo != "" || got["f2"].LinkedTo != "nk-1" {
		t.Errorf("candidates = %+v", got)
	}
	if f := got["f1"].File; f.SubjectID != "nv-7" || f.UploadedBy != "CB-00311" || f.Status != domain.StoredFileStored {
		t.Errorf("scan by position drifted: %+v", f)
	}
}

func TestAttachCandidatesRefusesBadListsWithoutSQL(t *testing.T) {
	d := &sfDB{}
	s, h := sfStore(d)
	many := make([]string, MaxAttachCandidates+1)
	for i := range many {
		many[i] = "f" + strconv.Itoa(i)
	}
	for name, ids := range map[string][]string{"duplicate": {"f1", "f1"}, "empty id": {""}, "over": many} {
		err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			_, e := s.AttachCandidates(ctx, tx, ids)
			return e
		})
		if !errors.Is(err, ErrAttachmentList) {
			t.Errorf("%s: err = %v, want ErrAttachmentList", name, err)
		}
	}
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		got, e := s.AttachCandidates(ctx, tx, nil)
		if len(got) != 0 {
			t.Errorf("empty list returned %v", got)
		}
		return e
	}); err != nil {
		t.Errorf("empty list: %v", err)
	}
	if len(d.stmts) != 0 {
		t.Errorf("refused lists ran %d statements", len(d.stmts))
	}
}
