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
// THE TRACKING TASK IS DELIBERATELY ABSENT, AND IT IS A FOLLOW-UP, NOT AN OVERSIGHT. §13 rule 4 asks
// that recording an issue on a project with an officer in charge raise a tracking task (tasks belong to
// petitions). That crosses a service boundary, and the user decided on 06/10/2026 that it comes later,
// as an event published from THIS transaction through the outbox — so the issue, its audit entry and
// the "please raise a task" message commit together or not at all. Until then
// `project_issues.tracking_task_id` stays NULL.
//
// THE MENTION NOTICE IS A gRPC CALL AFTER THE COMMIT, NOT AN OUTBOX ROW (ADR 0081 #5, 08/10/2026 —
// it replaces ADR 0075 #5 for this one kind). Each mentioned colleague except the author gets a bell
// notice (kind 17, bell only — never Zalo) through comms' DeliverStaffNotifications. THE TRADE-OFF THE
// OWNER ACCEPTED: comms down means the notice is lost and only a log line says so; the comment itself
// is saved either way, because a message somebody typed must never be refused for a bell. An outbox
// would close that gap, and exists nowhere in this repository yet.
//
// MENTIONS ARE STAFF BUSINESS CODES (CB-…), because the only staff list a `budget.*` account can read
// (GET /api/v1/staff-directory) carries codes and no internal id. Comms does NOT re-resolve them
// (comms.proto, "WHO MAY BE ADDRESSED"): a code naming nobody in this commune yields a bell row nobody
// in this commune can read — harmless, which is why the shape check in domain is enough here.
//
// FREE TEXT NEVER ENTERS THE AUDIT DELTA. An issue or a message is typed by staff and may name a
// person (rule 6, forbidden #4). The delta carries ids, lengths and who was mentioned; the row itself
// is immutable (0015's triggers), so the row IS the record of what was written.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
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
	LiveProjectCodeAndName(ctx context.Context, tx *store.ScopedTx, projectID string) (code, name string, err error)
	LiveProjectForIssue(ctx context.Context, tx *store.ScopedTx, projectID string) (code, assignee string, err error)
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

	// notifier is comms' bell inbox; nil = mentions are stored and nobody is told (dev with no
	// COMMS_GRPC_ADDR — staging/prod refuse to start without it, config.CommsClient).
	notifier MentionNotifier
	log      *slog.Logger
}

// MentionNotifier is comms' bell inbox, declared at the point of use. *commsclient.Client satisfies it.
type MentionNotifier interface {
	DeliverStaffNotifications(ctx context.Context, notices []commsclient.Notice) (map[string]commsclient.Delivery, error)
}

func NewProjectDiscussion(db *store.DB, s ProjectDiscussionStore) *ProjectDiscussion {
	return &ProjectDiscussion{db: db, store: s, newID: ulid.Moi, log: slog.Default()}
}

// NotifyMentionsThrough makes AddComment tell each mentioned colleague through n (ADR 0081 #5). A nil
// n leaves mentions unsent. log receives the failure line; nil keeps slog.Default.
func (uc *ProjectDiscussion) NotifyMentionsThrough(n MentionNotifier, log *slog.Logger) *ProjectDiscussion {
	uc.notifier = n
	if log != nil {
		uc.log = log
	}
	return uc
}

// The mention notice as the prototype writes it (vigov-require budget/service.py:1065-1076).
const (
	mentionTitlePrefix = "Bạn được nhắc trong trao đổi giải ngân: "
	// mentionBodyRunes is the prototype's `body[:280]` — characters, not bytes: a cut inside a UTF-8
	// sequence would hand comms an invalid string.
	mentionBodyRunes = 280
	// mentionTitleRunes is comms' bound on `title` (comms.proto, 1..200): a long project name is cut
	// rather than letting comms refuse the whole notice.
	mentionTitleRunes = 200
	// ONE KEY PER COMMENT, ALL RECIPIENTS UNDER IT: comms deduplicates per (commune, key, recipient),
	// so a retry reaches each person once — "comment id + recipient" without the recipient IN the key.
	// Putting it there would break the key's printable-ASCII rule for a staff code with a Vietnamese
	// letter (domain checks shape only), and comms would refuse the whole batch.
	mentionKeyPrefix = "giai-ngan.nhac-ten:"
)

// mentionNotice builds the notice for one saved comment, or ok=false when nobody but the author was
// mentioned. Pure, so the test pins title, body, key and recipients without a database.
func mentionNotice(c domain.ProjectComment, projectName string) (commsclient.Notice, bool) {
	recipients := make([]string, 0, len(c.MentionedStaffCodes))
	for _, code := range c.MentionedStaffCodes { // already trimmed and de-duplicated by domain
		if code != c.AuthorCode {
			recipients = append(recipients, code)
		}
	}
	if len(recipients) == 0 {
		return commsclient.Notice{}, false
	}
	return commsclient.Notice{
		IdempotencyKey: mentionKeyPrefix + c.ID,
		Kind:           commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DISBURSEMENT_MENTION,
		RecipientMa:    recipients,
		Title:          firstRunes(mentionTitlePrefix+projectName, mentionTitleRunes),
		Body:           firstRunes(c.Body, mentionBodyRunes),
		Link:           "/giai-ngan/du-an/" + url.PathEscape(c.ProjectID),
	}, true
}

func firstRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// notifyMentions runs AFTER the commit and never fails the request: the comment is already saved.
//
// context.WithoutCancel keeps the commune (the interceptor reads it) but not the request's
// cancellation — a client hanging up after the save must not drop its colleagues' notices. commsclient
// bounds the call at CallTimeout.
//
// THE LOG LINE CARRIES IDS AND A COUNT, NEVER THE TEXT (rule 3): the comment and the project name are
// free text, and the error is not logged as a string because comms' refusal may quote a field.
func (uc *ProjectDiscussion) notifyMentions(ctx context.Context, c domain.ProjectComment, projectCode, projectName string) {
	if uc.notifier == nil {
		return
	}
	n, ok := mentionNotice(c, projectName)
	if !ok {
		return
	}
	if _, err := uc.notifier.DeliverStaffNotifications(context.WithoutCancel(ctx), []commsclient.Notice{n}); err != nil {
		uc.log.WarnContext(ctx, "CẢNH BÁO: không gửi được thông báo nhắc tên trong trao đổi dự án — ý kiến vẫn đã lưu",
			"service", "finance", "comment_id", c.ID, "project_code", projectCode,
			"recipients", len(n.RecipientMa), "comms_unavailable", errors.Is(err, commsclient.ErrCommsUnavailable))
	}
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

// ProjectIssueRequest is one obstacle as the form sends it (migration 0016 added the last two).
type ProjectIssueRequest struct {
	// Text is exactly as typed — first line the title, the rest the description.
	Text string

	// OwnerCode is who follows the issue, a STAFF BUSINESS CODE. Blank = take the project's officer in
	// charge at this moment (prototype budget/service.py:949), or nobody if it has none.
	OwnerCode string

	// DueOn is by when; zero = no date. No "not in the past" rule — the prototype has none.
	DueOn time.Time
}

// RecordIssue records one obstacle on one live project of this commune.
//
// RECORDING NEVER DEPENDS ON ANYTHING ELSE, deliberately, as the prototype argues
// (budget/service.py:936-940): a clerk who typed "chưa bàn giao mặt bằng" told the commune something
// true, and refusing to store it until somebody is assigned is how obstacles end up in a notebook. So an
// owner is OPTIONAL, and a project officer whose stored value is not code-shaped yields NO owner rather
// than a refusal.
//
// THE OWNER IS A SNAPSHOT: the officer in charge when the issue is recorded, frozen with the row (0016's
// guard). Re-assigning the project later does not move who was asked to follow an issue already raised.
func (uc *ProjectDiscussion) RecordIssue(ctx context.Context, projectID string, req ProjectIssueRequest,
	actor audit.Actor) (domain.ProjectIssue, error) {

	if projectID == "" {
		return domain.ProjectIssue{}, fistore.ErrKhongThayDuAn
	}
	title, description, err := domain.SplitIssueText(req.Text)
	if err != nil {
		return domain.ProjectIssue{}, err
	}
	owner, err := domain.NormaliseIssueOwnerCode(req.OwnerCode)
	if err != nil {
		return domain.ProjectIssue{}, err
	}
	if err := domain.CheckIssueDueOn(req.DueOn); err != nil {
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
		OwnerCode: owner, DueOn: req.DueOn,
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		code, assignee, err := uc.store.LiveProjectForIssue(ctx, tx, projectID)
		if err != nil {
			return err
		}
		ownerFromProject := false
		if issue.OwnerCode == "" {
			// THE PROJECT'S OFFICER, read under the project's share lock in this transaction. Its value is
			// the staff code the web stores in `assignee_id` (web-admin project-people.ts:25), which the
			// server does not enforce — so it passes the same shape check a typed owner does, and a value
			// that fails it gives no owner instead of refusing the record.
			if snap, err := domain.NormaliseIssueOwnerCode(assignee); err == nil && snap != "" {
				issue.OwnerCode, ownerFromProject = snap, true
			}
		}
		if issue.RecordedAt, err = uc.store.InsertIssue(ctx, tx, issue); err != nil {
			return err
		}
		after := map[string]any{
			"issue_id":           issue.ID,
			"project_id":         projectID,
			"title_length":       len([]rune(title)),
			"description_length": len([]rune(description)),
			// A staff business code and a date: neither is free text nor personal data (rule 3). ""
			// means nobody was named and the project had no officer.
			"owner_code": issue.OwnerCode,
			"due_on":     ngayDelta(issue.DueOn),
		}
		if ownerFromProject {
			// WHO CHOSE THE OWNER: the clerk, or the project's officer by default. The two look identical
			// in the row, and years later "why was CB-00123 chasing this" has two different answers.
			after["owner_from_project"] = true
		}
		delta, err := json.Marshal(map[string]any{"sau": after})
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
	var code, name string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		code, name, err = uc.store.LiveProjectCodeAndName(ctx, tx, projectID)
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
	uc.notifyMentions(ctx, comment, code, name)
	return comment, nil
}

// wrapDiscussion wraps a failure with the commune and the operation, and NOTHING ELSE — never the
// text: an error travels into centralised logging across every commune. %w keeps the chain so the
// handler can tell a refusal from a failure. The handler NEVER returns this text to a client.
func wrapDiscussion(ctx context.Context, op string, err error) error {
	return fmt.Errorf("du_an: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
