package app

// The use cases behind §8.1 "Vướng mắc" and §8.4 "Trao đổi" (docs/ui-ux/06-giai-ngan.md; migration
// 0015; user decision 06/10/2026):
//
//	RecordIssue   a member of staff records an obstacle on a project
//	ResolveIssue  the obstacle is marked "Đã gỡ" — once, never reopened
//	AddComment    a message in the project's discussion, optionally mentioning colleagues
//
// WHY THIS LAYER: rule 6, invariant 3 — the row and its audit entry share ONE transaction, and
// core/audit.Write takes a *store.ScopedTx with no overload that writes outside one.
//
// WHAT HAPPENS ACROSS SERVICES IS DELIBERATELY ABSENT, AND IT IS A FOLLOW-UP, NOT AN OVERSIGHT.
// §13 rule 4 asks that recording an issue on a project with an officer in charge raise a tracking task
// (tasks belong to petitions), and the prototype notifies every mentioned colleague (messages belong to
// comms). Both cross a service boundary, which leaves exactly two legal paths (rule 2, invariant 3):
// a gRPC call or an event. The user decided on 06/10/2026 that these come later, as events published
// from THIS transaction through the outbox — so the issue, its audit entry and the "please raise a
// task" message commit together or not at all. Until then nothing here calls, publishes or enqueues
// anything, `project_issues.tracking_task_id` stays NULL, and a mention is stored without being sent.
//
// MENTIONS ARE STAFF BUSINESS CODES (CB-…), because the only staff list a `budget.*` account can read
// (GET /api/v1/staff-directory) carries codes and no internal id. So the notification follow-up has a
// step this service cannot do: comms must resolve each code to an ACTIVE account of THIS commune
// (through identity, which owns staff) and drop any code that resolves to nobody before sending.
//
// FREE TEXT NEVER ENTERS THE AUDIT DELTA. An issue or a message is typed by staff and may name a
// person (rule 6, forbidden #4). The delta carries ids, lengths and who was mentioned; the row itself
// is immutable (0015's triggers), so the row IS the record of what was written.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// ProjectDiscussionStore is the write store, declared at the point of use. EVERY METHOD TAKES THE
// TRANSACTION, so a row and its audit entry cannot be written in two.
type ProjectDiscussionStore interface {
	LiveProjectCode(ctx context.Context, tx *store.ScopedTx, projectID string) (string, error)
	InsertIssue(ctx context.Context, tx *store.ScopedTx, i domain.ProjectIssue) (time.Time, error)
	IssueForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.ProjectIssue, error)
	ResolveIssue(ctx context.Context, tx *store.ScopedTx, id, by string) (time.Time, error)
	InsertComment(ctx context.Context, tx *store.ScopedTx, c domain.ProjectComment) (time.Time, error)
}

// ProjectDiscussion owns recording and resolving a project's issues and posting to its discussion.
type ProjectDiscussion struct {
	db    *store.DB
	store ProjectDiscussionStore

	// newID is injected so a test can pin the ids. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewProjectDiscussion(db *store.DB, s ProjectDiscussionStore) *ProjectDiscussion {
	return &ProjectDiscussion{db: db, store: s, newID: ulid.Moi}
}

// The business verbs written into the trail — Vietnamese snake_case, like every other action this
// service writes (`them_du_an`, `them_nguon_von`): an inspection reads these strings.
const (
	ActionProjectIssueRecord  = "ghi_vuong_mac"
	ActionProjectIssueResolve = "go_vuong_mac"
	ActionProjectCommentAdd   = "gui_trao_doi"
)

// ProjectCommentRequest is one message as the form sends it.
type ProjectCommentRequest struct {
	Body                string
	MentionedStaffCodes []string
}

// RecordIssue records one obstacle on one live project of this commune.
//
// RECORDING NEVER DEPENDS ON ANYTHING ELSE, deliberately, as the prototype argues
// (budget/service.py:936-940): a clerk who typed "chưa bàn giao mặt bằng" told the commune something
// true, and refusing to store it until somebody is assigned is how obstacles end up in a notebook.
func (uc *ProjectDiscussion) RecordIssue(ctx context.Context, projectID, text string,
	actor audit.Actor) (domain.ProjectIssue, error) {

	if projectID == "" {
		return domain.ProjectIssue{}, fistore.ErrKhongThayDuAn
	}
	title, description, err := domain.SplitIssueText(text)
	if err != nil {
		return domain.ProjectIssue{}, err
	}
	if err := coNguoiThucHien(actor); err != nil {
		return domain.ProjectIssue{}, err
	}
	id, err := uc.newID()
	if err != nil {
		return domain.ProjectIssue{}, fmt.Errorf("vuong_mac: sinh mã: %w", err)
	}

	issue := domain.ProjectIssue{
		ID: id, ProjectID: projectID, Title: title, Description: description, RecordedBy: actor.ID,
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		code, err := uc.store.LiveProjectCode(ctx, tx, projectID)
		if err != nil {
			return err
		}
		if issue.RecordedAt, err = uc.store.InsertIssue(ctx, tx, issue); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": map[string]any{
			"issue_id":           issue.ID,
			"project_id":         projectID,
			"title_length":       len([]rune(title)),
			"description_length": len([]rune(description)),
		}})
		if err != nil {
			return fmt.Errorf("vuong_mac: mã hoá delta: %w", err)
		}
		// Subject = the PROJECT's business code (rule 6, invariant 8): an issue has no code of its own.
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionProjectIssueRecord, Subject: code, Delta: delta,
		})
	})
	if err != nil {
		return domain.ProjectIssue{}, wrapDiscussion(ctx, "ghi nhận vướng mắc", err)
	}
	return issue, nil
}

// ResolveIssue marks one open issue "Đã gỡ". ONE-WAY: an issue already resolved is
// domain.ErrIssueAlreadyResolved, and there is no reopen (the prototype has none — a returning
// obstacle is a new issue, which keeps "when was it first cleared" answerable).
//
// AN ISSUE OF A REMOVED PROJECT IS NOT FOUND: every read path has already dropped it with its project
// (rule 7, invariant 2), so resolving it would change a record no screen can show.
func (uc *ProjectDiscussion) ResolveIssue(ctx context.Context, issueID string,
	actor audit.Actor) (domain.ProjectIssue, error) {

	if issueID == "" {
		return domain.ProjectIssue{}, fistore.ErrIssueNotFound
	}
	if err := coNguoiThucHien(actor); err != nil {
		return domain.ProjectIssue{}, err
	}
	var issue domain.ProjectIssue
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		if issue, err = uc.store.IssueForUpdate(ctx, tx, issueID); err != nil {
			return err
		}
		code, err := uc.store.LiveProjectCode(ctx, tx, issue.ProjectID)
		if errors.Is(err, fistore.ErrKhongThayDuAn) {
			return fistore.ErrIssueNotFound
		}
		if err != nil {
			return err
		}
		if issue.Resolved() {
			return domain.ErrIssueAlreadyResolved
		}
		if issue.ResolvedAt, err = uc.store.ResolveIssue(ctx, tx, issueID, actor.ID); err != nil {
			return err
		}
		issue.ResolvedBy = actor.ID
		delta, err := json.Marshal(map[string]any{
			"issue_id":   issue.ID,
			"project_id": issue.ProjectID,
			"truoc":      map[string]any{"resolved": false},
			"sau":        map[string]any{"resolved": true},
		})
		if err != nil {
			return fmt.Errorf("vuong_mac: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionProjectIssueResolve, Subject: code, Delta: delta,
		})
	})
	if err != nil {
		return domain.ProjectIssue{}, wrapDiscussion(ctx, "ghi đã gỡ vướng mắc", err)
	}
	return issue, nil
}

// AddComment posts one message to the discussion of one live project of this commune.
func (uc *ProjectDiscussion) AddComment(ctx context.Context, projectID string, req ProjectCommentRequest,
	actor audit.Actor) (domain.ProjectComment, error) {

	if projectID == "" {
		return domain.ProjectComment{}, fistore.ErrKhongThayDuAn
	}
	body, err := domain.NormaliseCommentBody(req.Body)
	if err != nil {
		return domain.ProjectComment{}, err
	}
	mentions, err := domain.NormaliseMentions(req.MentionedStaffCodes)
	if err != nil {
		return domain.ProjectComment{}, err
	}
	if err := coNguoiThucHien(actor); err != nil {
		return domain.ProjectComment{}, err
	}
	id, err := uc.newID()
	if err != nil {
		return domain.ProjectComment{}, fmt.Errorf("trao_doi: sinh mã: %w", err)
	}

	comment := domain.ProjectComment{
		ID: id, ProjectID: projectID, Body: body, AuthorCode: actor.ID, MentionedStaffCodes: mentions,
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		code, err := uc.store.LiveProjectCode(ctx, tx, projectID)
		if err != nil {
			return err
		}
		if comment.CreatedAt, err = uc.store.InsertComment(ctx, tx, comment); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": map[string]any{
			"comment_id":            comment.ID,
			"project_id":            projectID,
			"body_length":           len([]rune(body)),
			"mentioned_staff_codes": mentions,
		}})
		if err != nil {
			return fmt.Errorf("trao_doi: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionProjectCommentAdd, Subject: code, Delta: delta,
		})
	})
	if err != nil {
		return domain.ProjectComment{}, wrapDiscussion(ctx, "gửi ý kiến trao đổi", err)
	}
	return comment, nil
}

// wrapDiscussion wraps a failure with the commune and the operation, and NOTHING ELSE — never the
// text: an error travels into centralised logging across every commune. %w keeps the chain so the
// handler can tell a refusal from a failure. The handler NEVER returns this text to a client.
func wrapDiscussion(ctx context.Context, op string, err error) error {
	return fmt.Errorf("du_an: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
