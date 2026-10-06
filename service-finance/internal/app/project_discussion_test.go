package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// What these cases hold, each a property that fails silently if it stops holding:
//
//  1. the row and its audit entry commit in ONE transaction, and a refusal or a failed entry commits
//     NOTHING (rule 6, invariant 3);
//  2. the trail names the STAFF BUSINESS CODE and the PROJECT's code, never an internal id (rule 6,
//     invariant 8), and the free text never enters the delta (rule 6, forbidden #4);
//  3. the commune is $1 of every statement (rule 1);
//  4. the first line is the title, the rest the description — split by the server;
//  5. resolution is one-way; a removed project's issue is not found;
//  6. nothing here reaches another service — no outbox row, no event (user decision 06/10/2026).
//
// The REAL ProjectDiscussionWriteStore runs over a fake database/sql driver, for the reason
// driver_gia_funding_source_test.go gives: transaction boundaries are properties of the SQL.

const (
	idIssueNew   = "01JVUONGMACMOI000000000000"
	idCommentNew = "01JTRAODOIMOI0000000000000"
	recordedAt   = "2026-09-07T08:30:00Z"
)

type fakeDiscussionDB struct {
	mu         sync.Mutex
	statements []lenhGhi

	begun, committed, rolledBack int

	projectCode string // "" = no live project with that id in this commune

	// issue is what IssueForUpdate finds; nil = none in this commune.
	issue *domain.ProjectIssue

	failOn string
	failed bool
}

func (k *fakeDiscussionDB) record(q string, args []driver.NamedValue) error {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.statements = append(k.statements, lenhGhi{sql: q, args: vals})
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return fmt.Errorf("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *fakeDiscussionDB) stmts(sub string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []lenhGhi
	for _, l := range k.statements {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *fakeDiscussionDB) Connect(context.Context) (driver.Conn, error) {
	return &fakeDiscussionConn{k: k}, nil
}
func (k *fakeDiscussionDB) Driver() driver.Driver { return trinhGia{} }

type fakeDiscussionConn struct{ k *fakeDiscussionDB }

func (c *fakeDiscussionConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("driver giả: không hỗ trợ Prepare")
}
func (c *fakeDiscussionConn) Close() error { return nil }
func (c *fakeDiscussionConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeDiscussionConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &fakeDiscussionTx{k: c.k}, nil
}

func (c *fakeDiscussionConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *fakeDiscussionConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	at, _ := time.Parse(time.RFC3339, recordedAt)
	one := func(col string, v driver.Value) driver.Rows {
		return &rowsGia{cot: []string{col}, hang: [][]driver.Value{{v}}}
	}
	issueCols := []string{"id", "project_id", "title", "description", "recorded_by",
		"recorded_at", "resolved_at", "resolved_by", "tracking_task_id"}
	switch {
	case strings.Contains(q, "FROM du_an") && strings.Contains(q, "FOR SHARE"):
		if c.k.projectCode == "" {
			return &rowsGia{cot: []string{"ma"}}, nil
		}
		return one("ma", c.k.projectCode), nil
	case strings.Contains(q, "INSERT INTO project_issues"):
		return one("recorded_at", at), nil
	case strings.Contains(q, "FROM project_issues") && strings.Contains(q, "FOR UPDATE"):
		if c.k.issue == nil {
			return &rowsGia{cot: issueCols}, nil
		}
		i := c.k.issue
		var resolved driver.Value
		if i.Resolved() {
			resolved = i.ResolvedAt
		}
		return &rowsGia{cot: issueCols, hang: [][]driver.Value{{i.ID, i.ProjectID, i.Title, i.Description,
			i.RecordedBy, i.RecordedAt, resolved, i.ResolvedBy, i.TrackingTaskID}}}, nil
	case strings.Contains(q, "UPDATE project_issues"):
		return one("resolved_at", at.Add(time.Hour)), nil
	case strings.Contains(q, "INSERT INTO project_comments"):
		return one("created_at", at), nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

type fakeDiscussionTx struct{ k *fakeDiscussionDB }

func (t *fakeDiscussionTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	return nil
}
func (t *fakeDiscussionTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	return nil
}

func buildDiscussion(t *testing.T, k *fakeDiscussionDB) (*ProjectDiscussion, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	scoped := store.New(db)
	uc := NewProjectDiscussion(scoped, fistore.NewProjectDiscussionWriteStore(scoped))
	uc.newID = func() (string, error) { return idIssueNew, nil }
	return uc, tenant.Into(context.Background(), xaA)
}

// discussionEntry decodes the ONE audit entry the case expects: (tenant_id, actor_id, actor_kind,
// actor_ip, action, subject, at, delta).
func discussionEntry(t *testing.T, k *fakeDiscussionDB) ([]driver.Value, map[string]any) {
	t.Helper()
	entries := k.stmts("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d vết, muốn đúng 1", len(entries))
	}
	var delta map[string]any
	if err := json.Unmarshal(entries[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	return entries[0].args, delta
}

func assertOneCommitAndScoped(t *testing.T, k *fakeDiscussionDB) {
	t.Helper()
	if k.begun != 1 || k.committed != 1 || k.rolledBack != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0", k.begun, k.committed, k.rolledBack)
	}
	for _, s := range k.statements {
		if len(s.args) == 0 || s.args[0] != string(xaA) {
			t.Fatalf("câu không buộc xã ở $1: %q %v", s.sql, s.args)
		}
		for _, banned := range []string{"outbox", "event"} {
			if strings.Contains(strings.ToLower(s.sql), banned) {
				t.Fatalf("lượt này không được phát sự kiện nào (quyết định 06/10/2026): %q", s.sql)
			}
		}
	}
}

func TestRecordIssueWritesRowAndEntryInOneTransaction(t *testing.T) {
	k := &fakeDiscussionDB{projectCode: "DA07"}
	uc, ctx := buildDiscussion(t, k)

	got, err := uc.RecordIssue(ctx, "da-001", "Chưa bàn giao mặt bằng\nHộ ông A chưa đồng ý.", clerk())
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}
	assertOneCommitAndScoped(t, k)
	if got.ID != idIssueNew || got.Title != "Chưa bàn giao mặt bằng" || got.Description != "Hộ ông A chưa đồng ý." ||
		got.RecordedBy != "CB-00123" || got.RecordedAt.IsZero() || got.Resolved() {
		t.Fatalf("trả về %+v", got)
	}
	ins := k.stmts("INSERT INTO project_issues")
	if len(ins) != 1 || ins[0].args[2] != "da-001" || ins[0].args[3] != "Chưa bàn giao mặt bằng" ||
		ins[0].args[5] != "CB-00123" {
		t.Fatalf("INSERT = %v", ins)
	}

	args, delta := discussionEntry(t, k)
	if args[1] != "CB-00123" || args[4] != ActionProjectIssueRecord || args[5] != "DA07" {
		t.Fatalf("vết: người %v, động từ %v, đối tượng %v", args[1], args[4], args[5])
	}
	raw, _ := json.Marshal(delta)
	if strings.Contains(string(raw), "mặt bằng") || strings.Contains(string(raw), "ông A") {
		t.Fatalf("nội dung tự do lọt vào delta: %s", raw)
	}
	if delta["sau"].(map[string]any)["issue_id"] != idIssueNew {
		t.Fatalf("delta = %s", raw)
	}
}

func TestRecordIssueRefusalsCommitNothing(t *testing.T) {
	for name, c := range map[string]struct {
		k       *fakeDiscussionDB
		project string
		text    string
		actor   audit.Actor
		want    error
	}{
		"dự án không có trong xã": {&fakeDiscussionDB{}, "da-x", "Vướng", clerk(), fistore.ErrKhongThayDuAn},
		"nội dung rỗng":           {&fakeDiscussionDB{projectCode: "DA01"}, "da-001", "  \n ", clerk(), domain.ErrIssueTextMissing},
		"thiếu mã cán bộ":         {&fakeDiscussionDB{projectCode: "DA01"}, "da-001", "Vướng", audit.Actor{Kind: "staff"}, nil},
		"vết hỏng":                {&fakeDiscussionDB{projectCode: "DA01", failOn: "INSERT INTO audit_log"}, "da-001", "Vướng", clerk(), nil},
	} {
		t.Run(name, func(t *testing.T) {
			uc, ctx := buildDiscussion(t, c.k)
			_, err := uc.RecordIssue(ctx, c.project, c.text, c.actor)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if c.k.committed != 0 {
				t.Fatal("bị từ chối mà vẫn commit")
			}
		})
	}
}

func TestResolveIssueOnceWithEntry(t *testing.T) {
	at, _ := time.Parse(time.RFC3339, recordedAt)
	k := &fakeDiscussionDB{projectCode: "DA07", issue: &domain.ProjectIssue{
		ID: "vm-1", ProjectID: "da-001", Title: "Vướng", RecordedBy: "CB-00001", RecordedAt: at}}
	uc, ctx := buildDiscussion(t, k)

	got, err := uc.ResolveIssue(ctx, "vm-1", clerk())
	if err != nil {
		t.Fatalf("ResolveIssue: %v", err)
	}
	assertOneCommitAndScoped(t, k)
	if !got.Resolved() || got.ResolvedBy != "CB-00123" {
		t.Fatalf("trả về %+v", got)
	}
	if u := k.stmts("UPDATE project_issues"); len(u) != 1 || u[0].args[2] != "CB-00123" {
		t.Fatalf("UPDATE = %v", u)
	}
	args, delta := discussionEntry(t, k)
	if args[4] != ActionProjectIssueResolve || args[5] != "DA07" ||
		delta["sau"].(map[string]any)["resolved"] != true || delta["truoc"].(map[string]any)["resolved"] != false {
		t.Fatalf("vết %v %v delta %v", args[4], args[5], delta)
	}
}

func TestResolveIssueRefusals(t *testing.T) {
	at, _ := time.Parse(time.RFC3339, recordedAt)
	resolved := &domain.ProjectIssue{ID: "vm-1", ProjectID: "da-001", Title: "x", RecordedBy: "CB-1",
		RecordedAt: at, ResolvedAt: at, ResolvedBy: "CB-2"}
	open := &domain.ProjectIssue{ID: "vm-1", ProjectID: "da-001", Title: "x", RecordedBy: "CB-1", RecordedAt: at}
	for name, c := range map[string]struct {
		k    *fakeDiscussionDB
		want error
	}{
		"đã gỡ rồi — không gỡ lại, không mở lại": {&fakeDiscussionDB{projectCode: "DA01", issue: resolved}, domain.ErrIssueAlreadyResolved},
		"không có trong xã":           {&fakeDiscussionDB{projectCode: "DA01"}, fistore.ErrIssueNotFound},
		"dự án đã rút khỏi danh sách": {&fakeDiscussionDB{issue: open}, fistore.ErrIssueNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			uc, ctx := buildDiscussion(t, c.k)
			if _, err := uc.ResolveIssue(ctx, "vm-1", clerk()); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if c.k.committed != 0 || len(c.k.stmts("UPDATE project_issues")) != 0 || len(c.k.stmts("audit_log")) != 0 {
				t.Fatal("bị từ chối mà vẫn ghi")
			}
		})
	}
}

func TestAddCommentStoresMentionsAndAuditsWithoutText(t *testing.T) {
	k := &fakeDiscussionDB{projectCode: "DA07"}
	uc, ctx := buildDiscussion(t, k)
	uc.newID = func() (string, error) { return idCommentNew, nil }

	got, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{
		Body: "  Đề nghị kế toán kiểm lại đợt 2 ", MentionedStaffCodes: []string{"CB-00011", "CB-00011", "CB-00012"},
	}, clerk())
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	assertOneCommitAndScoped(t, k)
	if got.ID != idCommentNew || got.Body != "Đề nghị kế toán kiểm lại đợt 2" || got.AuthorCode != "CB-00123" ||
		len(got.MentionedStaffCodes) != 2 || got.CreatedAt.IsZero() {
		t.Fatalf("trả về %+v", got)
	}
	ins := k.stmts("INSERT INTO project_comments")
	if len(ins) != 1 || ins[0].args[5] != `["CB-00011","CB-00012"]` {
		t.Fatalf("INSERT = %v", ins)
	}
	args, delta := discussionEntry(t, k)
	raw, _ := json.Marshal(delta)
	if args[4] != ActionProjectCommentAdd || args[5] != "DA07" || strings.Contains(string(raw), "kế toán") {
		t.Fatalf("vết %v %v delta %s", args[4], args[5], raw)
	}
}

func TestAddCommentWithNoMentionStoresEmptyArray(t *testing.T) {
	k := &fakeDiscussionDB{projectCode: "DA07"}
	uc, ctx := buildDiscussion(t, k)
	if _, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{Body: "Ok"}, clerk()); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if ins := k.stmts("INSERT INTO project_comments"); len(ins) != 1 || ins[0].args[5] != `[]` {
		t.Fatalf("INSERT = %v — muốn `[]`, không phải null", ins)
	}
}

func TestAddCommentRefusalsCommitNothing(t *testing.T) {
	for name, c := range map[string]struct {
		k    *fakeDiscussionDB
		req  ProjectCommentRequest
		want error
	}{
		"dự án không có trong xã": {&fakeDiscussionDB{}, ProjectCommentRequest{Body: "x"}, fistore.ErrKhongThayDuAn},
		"nội dung rỗng":           {&fakeDiscussionDB{projectCode: "DA1"}, ProjectCommentRequest{Body: " "}, domain.ErrCommentBodyMissing},
		"nhắc sai":                {&fakeDiscussionDB{projectCode: "DA1"}, ProjectCommentRequest{Body: "x", MentionedStaffCodes: []string{""}}, domain.ErrMentionInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			uc, ctx := buildDiscussion(t, c.k)
			if _, err := uc.AddComment(ctx, "da-001", c.req, clerk()); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if c.k.committed != 0 {
				t.Fatal("bị từ chối mà vẫn commit")
			}
		})
	}
}
