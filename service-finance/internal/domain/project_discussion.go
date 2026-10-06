package domain

// §8.1 "Vướng mắc" and §8.4 "Trao đổi" of the project detail screen (docs/ui-ux/06-giai-ngan.md;
// migration 0015; user decision 06/10/2026).
//
// WHAT IS DELIBERATELY NOT HERE, AND WHERE IT WILL GO: §13 rule 4's tracking task (petitions owns
// tasks) and the notification of a mentioned member of staff (comms owns messages). Both cross a
// service boundary, so both arrive later as events this service publishes — the user decided that on
// 06/10/2026, and nothing in this file or its callers calls another service in the meantime.
//
// PERSONAL DATA (rule 3): the texts here are free text typed by staff. They are about a public works
// project, but nothing stops a clerk typing a household's name, so no error in this file quotes the
// text it refused — every sentence names the field and the bound, never the value.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	// IssueTitleMax is the prototype's bound on the first line (IssueForm.tsx:40-42, 500 characters),
	// and 0015's CHECK. REFUSED, NOT TRUNCATED: the prototype client cut the line silently, which loses
	// the end of the sentence the clerk wrote with nothing saying so.
	IssueTitleMax = 500

	// IssueDescriptionMax and CommentBodyMax bound the free text at 4000 characters — 0015's CHECKs.
	// A paragraph or two is what these boxes hold; past this a document is being pasted, and it
	// belongs in an attachment the commune can classify, not in a timeline.
	IssueDescriptionMax = 4000
	CommentBodyMax      = 4000

	// MentionsMax bounds one comment's mentions — 0015's CHECK. The prototype's picker shows eight
	// people; twenty is past any real discussion and short of "mention the whole commune".
	MentionsMax = 20

	// StaffCodeMax bounds one mentioned staff business code (`CB-00123`). The bound only refuses
	// something that is plainly not a code; it does not fix a format the commune may not use.
	StaffCodeMax = 64
)

var (
	ErrIssueTextMissing = errors.New("vuong_mac: thiếu `text` — hãy ghi vướng mắc đang gặp")

	// The bound is in the sentinel's own text: the handler answers with this text and never with the
	// wrapped chain, which carries the commune id.
	ErrIssueTitleTooLong = fmt.Errorf("vuong_mac: dòng đầu của `text` quá dài (tối đa %d ký tự) — "+
		"xuống dòng để viết phần diễn giải", IssueTitleMax)
	ErrIssueDescriptionTooLong = fmt.Errorf("vuong_mac: phần diễn giải của `text` quá dài (tối đa %d ký tự)",
		IssueDescriptionMax)
	ErrIssueTextInvalid = errors.New("vuong_mac: `text` chứa ký tự điều khiển không hợp lệ")

	// ErrIssueAlreadyResolved — resolution is one-way (the prototype's one "Đã gỡ xong" button, no
	// reopen; 0015's trigger refuses the same thing underneath). A 409: the caller may resolve issues;
	// this one is already resolved, and resolving it again would overwrite who resolved it and when.
	ErrIssueAlreadyResolved = errors.New("vuong_mac: vướng mắc này đã được ghi là đã gỡ — " +
		"nếu vướng mắc quay lại, hãy ghi nhận một vướng mắc mới")

	ErrCommentBodyMissing = errors.New("trao_doi: thiếu `body` — hãy nhập ý kiến trao đổi")
	ErrCommentBodyTooLong = fmt.Errorf("trao_doi: `body` quá dài (tối đa %d ký tự)", CommentBodyMax)
	ErrCommentBodyInvalid = errors.New("trao_doi: `body` chứa ký tự điều khiển không hợp lệ")

	ErrMentionsTooMany = fmt.Errorf("trao_doi: `mentioned_staff_codes` nhắc quá nhiều người (tối đa %d)", MentionsMax)
	ErrMentionInvalid  = errors.New("trao_doi: `mentioned_staff_codes` có phần tử rỗng hoặc không phải mã cán bộ")
)

// ProjectIssue is one obstacle recorded against one project (§8.1, §11 `vuong_mac`).
//
// OPEN OR RESOLVED IS ResolvedAt, and nothing else. There is no status column: a status beside the
// timestamp would be two homes for one fact (rule 10, invariant 3's reasoning).
type ProjectIssue struct {
	ID        string
	ProjectID string

	Title       string
	Description string // "" = the clerk wrote one line

	// RecordedBy / ResolvedBy are STAFF BUSINESS CODES (CB-00123), never internal ids (rule 6,
	// invariant 8 — the same reader, years later, with no lookup still alive).
	RecordedBy string
	RecordedAt time.Time

	ResolvedAt time.Time // zero = still open
	ResolvedBy string

	// TrackingTaskID is §13 rule 4's link to the task chasing this issue. ALWAYS "" UNTIL THE TASK
	// EVENT EXISTS (user decision 06/10/2026); the column is in 0015 so that day needs no migration.
	TrackingTaskID string
}

// Resolved reports whether the issue has been marked "Đã gỡ".
func (i ProjectIssue) Resolved() bool { return !i.ResolvedAt.IsZero() }

// ProjectComment is one message of §8.4's discussion about one project.
type ProjectComment struct {
	ID        string
	ProjectID string
	Body      string

	// AuthorCode is the STAFF BUSINESS CODE (rule 6, invariant 8).
	AuthorCode string

	// MentionedStaffCodes are STAFF BUSINESS CODES (CB-…) — what GET /api/v1/staff-directory gives the
	// picker; that list carries no internal id. Never empty strings, never duplicated, at most
	// MentionsMax (NormaliseMentions). Not personal data.
	MentionedStaffCodes []string

	CreatedAt time.Time
}

// SplitIssueText turns what the clerk typed into (title, description): the first line is the title,
// the rest the description (prototype IssueForm.tsx:23-44 — "một câu tóm tắt, xuống dòng, rồi giải
// thích"). DONE BY THE SERVER so every client splits the same way.
//
// CRLF is folded to LF first, so a Windows clipboard does not leave a '\r' at the end of every title.
// Leading blank lines are dropped by the outer trim, so the title is the first line with text on it.
func SplitIssueText(text string) (title, description string, err error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return "", "", ErrIssueTextMissing
	}
	title, description, _ = strings.Cut(text, "\n")
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	switch {
	case len([]rune(title)) > IssueTitleMax:
		return "", "", ErrIssueTitleTooLong
	case len([]rune(description)) > IssueDescriptionMax:
		return "", "", ErrIssueDescriptionTooLong
	case hasControlExcept(title, "\t"), hasControlExcept(description, "\n\t"):
		return "", "", ErrIssueTextInvalid
	}
	return title, description, nil
}

// NormaliseCommentBody trims and validates one discussion message. Line breaks are kept — a message
// is a paragraph, and the screen renders it with `whitespace-pre-line`.
func NormaliseCommentBody(body string) (string, error) {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	switch {
	case body == "":
		return "", ErrCommentBodyMissing
	case len([]rune(body)) > CommentBodyMax:
		return "", ErrCommentBodyTooLong
	case hasControlExcept(body, "\n\t"):
		return "", ErrCommentBodyInvalid
	}
	return body, nil
}

// NormaliseMentions trims, validates and de-duplicates the mentioned staff codes, keeping the order the
// clerk picked them in. nil and [] both become an empty, non-nil slice — stored as `[]`, never NULL.
//
// SHAPE ONLY: non-empty, at most StaffCodeMax characters, no whitespace or control character. THE
// CODES ARE NOT CHECKED AGAINST identity HERE — that would be a synchronous call to another service,
// which this round does not make (user decision 06/10/2026). A code that names nobody, or somebody of
// another commune, notifies nobody: the notification follow-up must resolve each code to an active
// account of THIS commune (comms, through identity) before it sends anything.
func NormaliseMentions(codes []string) ([]string, error) {
	out := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" || len([]rune(code)) > StaffCodeMax || strings.ContainsFunc(code, unicode.IsSpace) ||
			hasControlExcept(code, "") {
			return nil, ErrMentionInvalid
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) > MentionsMax {
		return nil, ErrMentionsTooMany
	}
	return out, nil
}

// hasControlExcept reports a control character other than those listed in allowed.
func hasControlExcept(s, allowed string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && !strings.ContainsRune(allowed, r) {
			return true
		}
	}
	return false
}
