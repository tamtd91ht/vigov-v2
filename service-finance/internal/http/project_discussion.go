package http

// The routes of §8.1 "Vướng mắc" and §8.4 "Trao đổi" (docs/ui-ux/06-giai-ngan.md; migration 0015;
// user decision 06/10/2026). Declared in routes.go with their permissions:
//
//	GET  /api/v1/investment-projects/{id}/issues      §8.1's timeline, newest first
//	POST /api/v1/investment-projects/{id}/issues      record an obstacle (first line = title)
//	POST /api/v1/project-issues/{id}/resolution       mark it "Đã gỡ" — once, never reopened
//	GET  /api/v1/investment-projects/{id}/comments    §8.4's thread, oldest first
//	POST /api/v1/investment-projects/{id}/comments    post a message, optionally mentioning colleagues
//
// PEOPLE ARE BUSINESS CODES, NOT NAMES. `recorded_by`, `resolved_by` and `author_code` are staff codes
// (CB-00123): staff belong to identity, and this round makes no cross-service call (user decision
// 06/10/2026). The screen resolves a code to a name from the staff directory it already reads.
//
// FREE TEXT (rule 3): the issue text and the message body are returned to the commune's own staff who
// hold `budget.read` — the people the discussion is for — and NEVER logged, here or below.

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// ProjectDiscussionReading is the read half, *fistore.ProjectDiscussionStore in production. An
// interface at the point of use for the reason DuAnTienDo gives: the permission declaration, the
// commune check ahead of any read and the refusal instead of a truncated list must be testable
// without a PostgreSQL.
type ProjectDiscussionReading interface {
	IssuesOfProject(ctx context.Context, projectID string) ([]domain.ProjectIssue, error)
	CommentsOfProject(ctx context.Context, projectID string) ([]domain.ProjectComment, error)

	// LatestIssuesOfYear feeds §7.2's "Vướng mắc mới nhất" column — ONE read for the whole page, keyed
	// by project id, under the same filter as DuAnTienDo.DanhSach.
	LatestIssuesOfYear(ctx context.Context, loc fistore.LocDuAn) (map[string]domain.ProjectIssue, error)
	// OpenIssueCount feeds §3's "N vướng mắc đang theo dõi".
	OpenIssueCount(ctx context.Context, year int) (int, error)
}

// ProjectDiscussionWriting is the write half, *app.ProjectDiscussion in production — separate from the
// read half for the reason GhiDuAn gives: each method opens a transaction and writes an audit entry
// inside it.
type ProjectDiscussionWriting interface {
	RecordIssue(ctx context.Context, projectID, text string, actor audit.Actor) (domain.ProjectIssue, error)
	ResolveIssue(ctx context.Context, issueID string, actor audit.Actor) (domain.ProjectIssue, error)
	AddComment(ctx context.Context, projectID string, req app.ProjectCommentRequest,
		actor audit.Actor) (domain.ProjectComment, error)
}

// projectIssueOut is one issue as §8.1's timeline draws it.
type projectIssueOut struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	// Description is the text after the first line; absent when the clerk wrote one line.
	Description string `json:"description,omitempty"`

	RecordedBy string `json:"recorded_by"` // staff business code
	RecordedAt string `json:"recorded_at"` // RFC 3339, UTC

	// Resolved is "Đã gỡ" — derived from resolved_at, never stored on its own.
	Resolved   bool   `json:"resolved"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	ResolvedBy string `json:"resolved_by,omitempty"` // staff business code

	// TrackingTaskID is §13 rule 4's tracking task. ALWAYS ABSENT TODAY: the task is raised by a later
	// event (user decision 06/10/2026). Published now, optional, so the screen can draw the link the
	// day it appears without a contract change.
	TrackingTaskID string `json:"tracking_task_id,omitempty"`
}

func projectIssueOutOf(i domain.ProjectIssue) projectIssueOut {
	return projectIssueOut{
		ID: i.ID, ProjectID: i.ProjectID, Title: i.Title, Description: i.Description,
		RecordedBy: i.RecordedBy, RecordedAt: lucRa(i.RecordedAt),
		Resolved: i.Resolved(), ResolvedAt: lucRa(i.ResolvedAt), ResolvedBy: i.ResolvedBy,
		TrackingTaskID: i.TrackingTaskID,
	}
}

// projectIssuesOut is GET /api/v1/investment-projects/{id}/issues. `items` is never null; `count` is
// len(items) (never paged, never truncated — the store refuses past its ceiling); `open_count` is the
// unresolved ones, for the tab title "Vướng mắc (N)".
type projectIssuesOut struct {
	ProjectID string            `json:"project_id"`
	Items     []projectIssueOut `json:"items"`
	Count     int               `json:"count"`
	OpenCount int               `json:"open_count"`
}

// projectIssueIn is the body of POST .../issues: the text exactly as typed. The server splits it —
// first line the title, the rest the description (domain.SplitIssueText).
type projectIssueIn struct {
	Text string `json:"text"`
}

// projectCommentOut is one message of §8.4's thread.
type projectCommentOut struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	Body       string `json:"body"`
	AuthorCode string `json:"author_code"` // staff business code
	// MentionedStaffCodes are staff business codes (CB-…, the `code` of GET /api/v1/staff-directory),
	// never null ([] when nobody was mentioned).
	MentionedStaffCodes []string `json:"mentioned_staff_codes"`
	CreatedAt           string   `json:"created_at"` // RFC 3339, UTC
}

func projectCommentOutOf(c domain.ProjectComment) projectCommentOut {
	mentions := c.MentionedStaffCodes
	if mentions == nil {
		mentions = []string{}
	}
	return projectCommentOut{
		ID: c.ID, ProjectID: c.ProjectID, Body: c.Body, AuthorCode: c.AuthorCode,
		MentionedStaffCodes: mentions, CreatedAt: lucRa(c.CreatedAt),
	}
}

type projectCommentsOut struct {
	ProjectID string              `json:"project_id"`
	Items     []projectCommentOut `json:"items"`
	Count     int                 `json:"count"`
}

// projectCommentIn is the body of POST .../comments.
//
// mentioned_staff_codes ARE STAFF BUSINESS CODES (the `code` of GET /api/v1/staff-directory — the only
// staff list a budget account can read, and it carries no internal id), checked for shape only. They
// are STORED, NOT SENT: the notification is comms's job and arrives with a later event (user decision
// 06/10/2026), which must resolve each code to an active account of this commune first.
type projectCommentIn struct {
	Body                string   `json:"body"`
	MentionedStaffCodes []string `json:"mentioned_staff_codes,omitempty"`
}

// latestIssueOut is §7.2's "Vướng mắc mới nhất" cell: the latest issue's title, when it was recorded
// and whether it has been resolved ("27/8/2026 · đã gỡ").
type latestIssueOut struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	RecordedAt string `json:"recorded_at"`
	Resolved   bool   `json:"resolved"`
}

func latestIssueOutOf(i domain.ProjectIssue) *latestIssueOut {
	return &latestIssueOut{ID: i.ID, Text: i.Title, RecordedAt: lucRa(i.RecordedAt), Resolved: i.Resolved()}
}

// ListProjectIssues — GET /api/v1/investment-projects/{id}/issues
//
// A PROJECT OF ANOTHER COMMUNE ANSWERS 404, the same as one that does not exist — the store cannot
// reach it. NO AUDIT ENTRY: the commune's own records about its own public works, under budget.read.
func (h *Handler) ListProjectIssues(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument", "Thiếu mã dự án.", "")
		return
	}
	issues, err := h.d.ProjectDiscussion.IssuesOfProject(r.Context(), id)
	if err != nil {
		h.writeDiscussionError(w, r, "đọc vướng mắc", err)
		return
	}
	out := projectIssuesOut{ProjectID: id, Items: make([]projectIssueOut, 0, len(issues)), Count: len(issues)}
	for _, i := range issues {
		if !i.Resolved() {
			out.OpenCount++
		}
		out.Items = append(out.Items, projectIssueOutOf(i))
	}
	vietJSON(w, http.StatusOK, out)
}

// RecordProjectIssue — POST /api/v1/investment-projects/{id}/issues
func (h *Handler) RecordProjectIssue(w http.ResponseWriter, r *http.Request) {
	var in projectIssueIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	got, err := h.d.ProjectDiscussionWrites.RecordIssue(r.Context(), r.PathValue("id"), in.Text, actor)
	if err != nil {
		h.writeDiscussionError(w, r, "ghi nhận vướng mắc", err)
		return
	}
	idem.RecordCode(r.Context(), got.ID)
	vietJSON(w, http.StatusCreated, projectIssueOutOf(got))
}

// ResolveProjectIssue — POST /api/v1/project-issues/{id}/resolution. No body.
func (h *Handler) ResolveProjectIssue(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	got, err := h.d.ProjectDiscussionWrites.ResolveIssue(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.writeDiscussionError(w, r, "ghi đã gỡ vướng mắc", err)
		return
	}
	vietJSON(w, http.StatusOK, projectIssueOutOf(got))
}

// ListProjectComments — GET /api/v1/investment-projects/{id}/comments
func (h *Handler) ListProjectComments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument", "Thiếu mã dự án.", "")
		return
	}
	comments, err := h.d.ProjectDiscussion.CommentsOfProject(r.Context(), id)
	if err != nil {
		h.writeDiscussionError(w, r, "đọc trao đổi", err)
		return
	}
	out := projectCommentsOut{ProjectID: id, Items: make([]projectCommentOut, 0, len(comments)), Count: len(comments)}
	for _, c := range comments {
		out.Items = append(out.Items, projectCommentOutOf(c))
	}
	vietJSON(w, http.StatusOK, out)
}

// AddProjectComment — POST /api/v1/investment-projects/{id}/comments
func (h *Handler) AddProjectComment(w http.ResponseWriter, r *http.Request) {
	var in projectCommentIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	got, err := h.d.ProjectDiscussionWrites.AddComment(r.Context(), r.PathValue("id"),
		app.ProjectCommentRequest{Body: in.Body, MentionedStaffCodes: in.MentionedStaffCodes}, actor)
	if err != nil {
		h.writeDiscussionError(w, r, "gửi ý kiến trao đổi", err)
		return
	}
	idem.RecordCode(r.Context(), got.ID)
	vietJSON(w, http.StatusCreated, projectCommentOutOf(got))
}

// discussionInputErrors are refusals of what the client sent. LISTED, never "anything unknown is a
// 400", for the reason laLoiDauVaoDuAn gives. Each sentinel's text names the field and the bound and
// never quotes the value.
var discussionInputErrors = []error{
	domain.ErrIssueTextMissing, domain.ErrIssueTitleTooLong, domain.ErrIssueDescriptionTooLong,
	domain.ErrIssueTextInvalid,
	domain.ErrCommentBodyMissing, domain.ErrCommentBodyTooLong, domain.ErrCommentBodyInvalid,
	domain.ErrMentionsTooMany, domain.ErrMentionInvalid,
}

// writeDiscussionError maps one failure onto a status and a sentence — ONE mapping for the five
// routes. THE SENTENCE IS A FIXED STRING OR A SENTINEL'S OWN TEXT, NEVER err.Error() OF THE CHAIN: a
// failure inside the transaction is wrapped with the commune id, and a PostgreSQL exception must never
// reach a client (rule 3, forbidden #3). The log line carries the commune and the error chain — which
// by construction (app.wrapDiscussion, the store's wraps) holds no free text.
func (h *Handler) writeDiscussionError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, fistore.ErrKhongThayDuAn):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy dự án.", "")
		return
	case errors.Is(err, fistore.ErrIssueNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy vướng mắc này.", "")
		return
	case errors.Is(err, domain.ErrIssueAlreadyResolved):
		// 409, NOT 403: the caller may resolve issues; this one already is.
		httpx.WriteError(w, http.StatusConflict, "issue_already_resolved", domain.ErrIssueAlreadyResolved.Error(), "")
		return
	}
	for _, e := range discussionInputErrors {
		if errors.Is(err, e) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", e.Error(), "")
			return
		}
	}
	if errors.Is(err, fistore.ErrTooManyIssues) || errors.Is(err, fistore.ErrTooManyComments) {
		// REFUSED, NOT TRUNCATED: the tab count is the length of this list.
		h.d.Log.Error("vướng mắc/trao đổi: "+op+" vượt trần — TỪ CHỐI thay vì cắt bớt",
			"xa", string(tenant.MustFrom(r.Context())))
	} else {
		h.d.Log.Error("vướng mắc/trao đổi: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
