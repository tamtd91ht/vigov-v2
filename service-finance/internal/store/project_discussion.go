package store

// §8.1 issues and §8.4 discussion of one investment project (migration 0015, user decision
// 06/10/2026): the reads behind the two tabs, §7.2's "Vướng mắc mới nhất" column and §3's open-issue
// count, and the write path the use case drives inside its transaction.
//
// THE PROPERTIES chung_tu_giai_ngan.go lists hold here unchanged: the commune is $1 in every
// statement, taken from the context or the transaction and never from a parameter (rule 1,
// invariants 4 and 5), and constrained on EVERY joined table; nothing here opens a transaction
// (internal/app does, and writes the audit entry inside it — rule 6, invariant 3); every read excludes
// soft-deleted rows — of the issue or comment AND of its project (rule 7, invariant 2); there is no
// hard delete and no edit of a recorded text (0015's triggers refuse both underneath).
//
// FREE TEXT NEVER LEAVES THIS FILE IN AN ERROR: every wrap names the operation, never the value.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

var (
	// ErrIssueNotFound — no LIVE issue with this id on a LIVE project of THIS commune. Another
	// commune's issue is indistinguishable from one that does not exist; the caller answers 404.
	ErrIssueNotFound = errors.New("vuong_mac: không có vướng mắc này trong xã")

	// ErrTooManyIssues / ErrTooManyComments — one project past its ceiling. REFUSED, NOT TRUNCATED:
	// the tab count beside the tab title is the length of this list, and a silently short timeline is
	// a record that looks complete and is not.
	ErrTooManyIssues   = errors.New("vuong_mac: dự án vượt trần số vướng mắc")
	ErrTooManyComments = errors.New("trao_doi: dự án vượt trần số ý kiến trao đổi")
)

// MaxIssuesPerProject / MaxCommentsPerProject bound one project's timeline. A real project carries a
// handful of issues and a few dozen messages over its life; these are the point where the data is no
// longer one project's discussion (an import run twice, a fixture on a live database).
const (
	MaxIssuesPerProject   = 2000
	MaxCommentsPerProject = 5000
)

// ProjectDiscussionStore reads one commune's project issues and comments. It holds *store.DB and
// takes the commune from the context on every call (rule 1, invariant 5).
type ProjectDiscussionStore struct {
	db *store.DB
}

func NewProjectDiscussionStore(db *store.DB) *ProjectDiscussionStore {
	return &ProjectDiscussionStore{db: db}
}

// cotIssue IS READ BY POSITION in scanIssue. Nullable columns are COALESCEd to "" except resolved_at,
// whose NULL is the meaning "still open", and due_on (0016), whose NULL is "no date". The two 0016
// columns are LAST, so no existing position moved.
const cotIssue = `pi.id, pi.project_id, pi.title, COALESCE(pi.description, ''), pi.recorded_by, ` +
	`pi.recorded_at, pi.resolved_at, COALESCE(pi.resolved_by, ''), COALESCE(pi.tracking_task_id, ''), ` +
	`COALESCE(pi.owner_code, ''), pi.due_on`

func scanIssue(scan func(dest ...any) error) (domain.ProjectIssue, error) {
	var (
		i        domain.ProjectIssue
		resolved sql.NullTime
		due      sql.NullTime
	)
	if err := scan(&i.ID, &i.ProjectID, &i.Title, &i.Description, &i.RecordedBy,
		&i.RecordedAt, &resolved, &i.ResolvedBy, &i.TrackingTaskID, &i.OwnerCode, &due); err != nil {
		return domain.ProjectIssue{}, err
	}
	if resolved.Valid {
		i.ResolvedAt = resolved.Time
	}
	if due.Valid {
		i.DueOn = due.Time
	}
	return i, nil
}

// requireLiveProject answers ErrKhongThayDuAn unless the project is a live project of this commune —
// so "no such project" (404) and "a project with nothing recorded yet" ([]) are two answers.
func (s *ProjectDiscussionStore) requireLiveProject(ctx context.Context, projectID string) error {
	rows, err := s.db.For(ctx).Query(ctx, "1", "du_an", "AND id = $2 AND deleted_at IS NULL", projectID)
	if err != nil {
		return fmt.Errorf("du_an: kiểm dự án của vướng mắc/trao đổi: %w", err)
	}
	found := rows.Next()
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("du_an: kiểm dự án của vướng mắc/trao đổi: %w", err)
	}
	if !found {
		return ErrKhongThayDuAn
	}
	return nil
}

// issuesOfProject — newest first; the id (a ULID) breaks a tie, so the order is TOTAL.
const issuesOfProject = `SELECT ` + cotIssue + `
	FROM project_issues pi
	WHERE pi.tenant_id = $1 AND pi.project_id = $2 AND pi.deleted_at IS NULL
	ORDER BY pi.recorded_at DESC, pi.id DESC
	LIMIT $3`

// IssuesOfProject reads §8.1's timeline of one live project of this commune, resolved issues
// included (the prototype greys them out, it does not hide them — BudgetItemDetail.tsx:537-541).
func (s *ProjectDiscussionStore) IssuesOfProject(ctx context.Context, projectID string) ([]domain.ProjectIssue, error) {
	if err := s.requireLiveProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, issuesOfProject, projectID, MaxIssuesPerProject+1)
	if err != nil {
		return nil, fmt.Errorf("vuong_mac: đọc theo dự án: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ProjectIssue, 0, 8)
	for rows.Next() {
		i, err := scanIssue(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("vuong_mac: đọc dòng: %w", err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vuong_mac: duyệt theo dự án: %w", err)
	}
	if len(out) > MaxIssuesPerProject {
		return nil, ErrTooManyIssues
	}
	return out, nil
}

// latestIssuesOfYear — §7.2's "Vướng mắc mới nhất": for every live project of the year (and of the
// optional category, the list's own filter), its most recent live issue, RESOLVED OR NOT (§7.2 prints
// "27/8/2026 · đã gỡ"). ONE STATEMENT FOR THE WHOLE PAGE — never one read per project.
//
// DISTINCT ON (pi.project_id) with the timeline's own order, so "latest" here and the top of §8.1's
// timeline are the same row. tenant_id = $1 on both tables (rule 1).
const latestIssuesOfYear = `SELECT DISTINCT ON (pi.project_id) ` + cotIssue + `
	FROM project_issues pi
	JOIN du_an da
	  ON da.tenant_id = pi.tenant_id AND da.tenant_id = $1 AND da.id = pi.project_id
	WHERE pi.tenant_id = $1 AND pi.deleted_at IS NULL
	  AND da.deleted_at IS NULL AND da.nam = $2
	  AND ($3 = '' OR da.hang_muc_id = $3)
	ORDER BY pi.project_id, pi.recorded_at DESC, pi.id DESC
	LIMIT $4`

// LatestIssuesOfYear reads the latest issue of each project of the list, keyed by project id. A
// project with no issue is absent from the map — the column shows "—".
func (s *ProjectDiscussionStore) LatestIssuesOfYear(ctx context.Context,
	loc LocDuAn) (map[string]domain.ProjectIssue, error) {

	if loc.Nam == 0 {
		return nil, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, latestIssuesOfYear, loc.Nam, loc.HangMucID, TranDuAnMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("vuong_mac: đọc mới nhất theo năm: %w", err)
	}
	defer rows.Close()
	out := make(map[string]domain.ProjectIssue, 32)
	for rows.Next() {
		i, err := scanIssue(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("vuong_mac: đọc dòng mới nhất: %w", err)
		}
		out[i.ProjectID] = i
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vuong_mac: duyệt mới nhất theo năm: %w", err)
	}
	if len(out) > TranDuAnMotNam {
		// One row per project, so this is the project list's own ceiling — refused as it refuses.
		return nil, ErrQuaNhieuDuAn
	}
	return out, nil
}

// openIssueCount — §3's "N vướng mắc đang theo dõi": live, UNRESOLVED issues of the year's live
// projects. Scoped to the year, like every other figure on the card (§13 rule 8); the prototype
// counted every year at once (budget/service.py:1115), which would put last year's forgotten issues
// on this year's card.
const openIssueCount = `SELECT count(*)
	FROM project_issues pi
	JOIN du_an da
	  ON da.tenant_id = pi.tenant_id AND da.tenant_id = $1 AND da.id = pi.project_id
	WHERE pi.tenant_id = $1 AND pi.deleted_at IS NULL AND pi.resolved_at IS NULL
	  AND da.deleted_at IS NULL AND da.nam = $2`

// OpenIssueCount counts the unresolved issues of one budget year's live projects.
func (s *ProjectDiscussionStore) OpenIssueCount(ctx context.Context, year int) (int, error) {
	if year == 0 {
		return 0, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, openIssueCount, year)
	if err != nil {
		return 0, fmt.Errorf("vuong_mac: đếm chưa gỡ: %w", err)
	}
	n, err := scanCount(rows)
	if err != nil {
		return 0, fmt.Errorf("vuong_mac: đếm chưa gỡ: %w", err)
	}
	return int(n), nil
}

// commentsOfProject — oldest first, as a conversation reads; the id breaks a tie.
const commentsOfProject = `SELECT pc.id, pc.project_id, pc.body, pc.author_code,
	       pc.mentioned_staff_codes::text, pc.created_at
	FROM project_comments pc
	WHERE pc.tenant_id = $1 AND pc.project_id = $2 AND pc.deleted_at IS NULL
	ORDER BY pc.created_at, pc.id
	LIMIT $3`

// CommentsOfProject reads §8.4's thread of one live project of this commune.
func (s *ProjectDiscussionStore) CommentsOfProject(ctx context.Context, projectID string) ([]domain.ProjectComment, error) {
	if err := s.requireLiveProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, commentsOfProject, projectID, MaxCommentsPerProject+1)
	if err != nil {
		return nil, fmt.Errorf("trao_doi: đọc theo dự án: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ProjectComment, 0, 16)
	for rows.Next() {
		var (
			c        domain.ProjectComment
			mentions string
		)
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Body, &c.AuthorCode, &mentions, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("trao_doi: đọc dòng: %w", err)
		}
		if err := json.Unmarshal([]byte(mentions), &c.MentionedStaffCodes); err != nil {
			return nil, fmt.Errorf("trao_doi: đọc danh sách người được nhắc: %w", err)
		}
		if c.MentionedStaffCodes == nil {
			c.MentionedStaffCodes = []string{}
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trao_doi: duyệt theo dự án: %w", err)
	}
	if len(out) > MaxCommentsPerProject {
		return nil, ErrTooManyComments
	}
	return out, nil
}

// --- writes --------------------------------------------------------------------------------------

// ProjectDiscussionWriteStore writes issues and comments. EVERY METHOD TAKES THE CALLER'S
// TRANSACTION — there is no signature here that could write a row in one transaction and its audit
// entry in another. Every statement binds the commune as $1 from tx.TenantID().
type ProjectDiscussionWriteStore struct {
	db *store.DB
}

func NewProjectDiscussionWriteStore(db *store.DB) *ProjectDiscussionWriteStore {
	return &ProjectDiscussionWriteStore{db: db}
}

// projectForDiscussion reads the project's BUSINESS CODE and holds the row FOR SHARE.
//
// THE CODE IS THE AUDIT SUBJECT (rule 6, invariant 8): an issue and a comment have no code of their
// own, so the project's is the identifier an inspection can look up; the row's id is in the delta.
//
// FOR SHARE, so a project removal (which takes the row FOR UPDATE — DuAnGhiStore.TheoIDDeSua) and a
// new issue or message on it are serialised: whichever comes second sees the other's result, and an
// issue cannot land on a project in the instant it is withdrawn. Two writers of discussion do not block
// each other — share locks are compatible.
const projectForDiscussion = `SELECT ma FROM du_an
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR SHARE`

// LiveProjectCode returns the code of one live project of this commune; ErrKhongThayDuAn otherwise.
func (s *ProjectDiscussionWriteStore) LiveProjectCode(ctx context.Context, tx *store.ScopedTx,
	projectID string) (string, error) {

	var code string
	err := tx.Underlying().QueryRowContext(ctx, projectForDiscussion, string(tx.TenantID()), projectID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrKhongThayDuAn
	}
	if err != nil {
		return "", fmt.Errorf("du_an: đọc mã dự án cho vướng mắc/trao đổi: %w", err)
	}
	return code, nil
}

// projectForIssue is projectForDiscussion plus the project's officer in charge — the owner an issue
// takes when the request names none (prototype budget/service.py:949). Same FOR SHARE, same reason.
const projectForIssue = `SELECT ma, COALESCE(can_bo_phu_trach_id, '') FROM du_an
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR SHARE`

// LiveProjectForIssue returns the code and the officer in charge ("" = nobody) of one live project of
// this commune; ErrKhongThayDuAn otherwise. Read inside the issue's transaction, so the owner snapshot
// is the officer at the moment the issue is recorded.
func (s *ProjectDiscussionWriteStore) LiveProjectForIssue(ctx context.Context, tx *store.ScopedTx,
	projectID string) (code, assignee string, err error) {

	err = tx.Underlying().QueryRowContext(ctx, projectForIssue, string(tx.TenantID()), projectID).
		Scan(&code, &assignee)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrKhongThayDuAn
	}
	if err != nil {
		return "", "", fmt.Errorf("du_an: đọc dự án cho vướng mắc: %w", err)
	}
	return code, assignee, nil
}

// insertIssue — recorded_at is the DATABASE's clock, returned, so the reply and the row agree to the
// microsecond and two replicas with skewed clocks cannot order one project's timeline wrongly.
//
// owner_code / due_on (0016) are written HERE AND ONLY HERE: 0016's guard freezes them after insert.
// "" and the zero date go in as NULL — 0016's CHECK refuses a blank owner_code.
const insertIssue = `INSERT INTO project_issues
	(tenant_id, id, project_id, title, description, recorded_by, owner_code, due_on)
	VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8)
	RETURNING recorded_at`

// InsertIssue records one issue and returns the moment it was recorded.
func (s *ProjectDiscussionWriteStore) InsertIssue(ctx context.Context, tx *store.ScopedTx,
	i domain.ProjectIssue) (time.Time, error) {

	var at time.Time
	err := tx.Underlying().QueryRowContext(ctx, insertIssue, string(tx.TenantID()),
		i.ID, i.ProjectID, i.Title, i.Description, i.RecordedBy,
		rongThanhNil(i.OwnerCode), ngayThanhNil(i.DueOn)).Scan(&at)
	if err != nil {
		return time.Time{}, fmt.Errorf("vuong_mac: ghi nhận: %w", err)
	}
	return at, nil
}

// issueForUpdate locks one live issue of this commune until the transaction ends — what makes "check
// it is open, then resolve it" one decision rather than a race two clerks can both win.
const issueForUpdate = `SELECT ` + cotIssue + `
	FROM project_issues pi
	WHERE pi.tenant_id = $1 AND pi.id = $2 AND pi.deleted_at IS NULL
	FOR UPDATE`

// IssueForUpdate reads and locks one live issue; ErrIssueNotFound otherwise.
func (s *ProjectDiscussionWriteStore) IssueForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.ProjectIssue, error) {

	rows, err := tx.Underlying().QueryContext(ctx, issueForUpdate, string(tx.TenantID()), id)
	if err != nil {
		return domain.ProjectIssue{}, fmt.Errorf("vuong_mac: khoá để gỡ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.ProjectIssue{}, fmt.Errorf("vuong_mac: khoá để gỡ: %w", err)
		}
		return domain.ProjectIssue{}, ErrIssueNotFound
	}
	i, err := scanIssue(rows.Scan)
	if err != nil {
		return domain.ProjectIssue{}, fmt.Errorf("vuong_mac: đọc dòng để gỡ: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.ProjectIssue{}, fmt.Errorf("vuong_mac: duyệt để gỡ: %w", err)
	}
	return i, nil
}

// resolveIssue — the only UPDATE of this table's business columns. `resolved_at IS NULL` in the
// filter is the second fence behind the lock: zero rows means it was already resolved.
const resolveIssue = `UPDATE project_issues
	SET resolved_at = now(), resolved_by = $3
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND resolved_at IS NULL
	RETURNING resolved_at`

// ResolveIssue marks one open issue resolved by `by` (a staff business code) and returns when.
func (s *ProjectDiscussionWriteStore) ResolveIssue(ctx context.Context, tx *store.ScopedTx,
	id, by string) (time.Time, error) {

	var at time.Time
	err := tx.Underlying().QueryRowContext(ctx, resolveIssue, string(tx.TenantID()), id, by).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, domain.ErrIssueAlreadyResolved
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("vuong_mac: ghi đã gỡ: %w", err)
	}
	return at, nil
}

const insertComment = `INSERT INTO project_comments
	(tenant_id, id, project_id, body, author_code, mentioned_staff_codes)
	VALUES ($1, $2, $3, $4, $5, $6::jsonb)
	RETURNING created_at`

// InsertComment posts one message and returns when it was posted (the database's clock, as
// InsertIssue).
func (s *ProjectDiscussionWriteStore) InsertComment(ctx context.Context, tx *store.ScopedTx,
	c domain.ProjectComment) (time.Time, error) {

	mentions := c.MentionedStaffCodes
	if mentions == nil {
		mentions = []string{} // `[]`, never `null` — 0015's CHECK wants an array
	}
	raw, err := json.Marshal(mentions)
	if err != nil {
		return time.Time{}, fmt.Errorf("trao_doi: mã hoá người được nhắc: %w", err)
	}
	var at time.Time
	err = tx.Underlying().QueryRowContext(ctx, insertComment, string(tx.TenantID()),
		c.ID, c.ProjectID, c.Body, c.AuthorCode, string(raw)).Scan(&at)
	if err != nil {
		return time.Time{}, fmt.Errorf("trao_doi: ghi: %w", err)
	}
	return at, nil
}
