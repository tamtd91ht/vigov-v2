package store

// The only path to `stored_file` and `task_log_attachment` (migration 0021, ADR 0052 §5). SQL, and
// nothing else: which purpose, which limit, who may upload and what is audited are internal/app's.
//
// ⚠ `original_name` IS PERSONAL DATA (rule 3). No error built here quotes it, nor an object key.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// StoredFileStore holds *store.DB, never a *sql.DB: the commune is $1 in every statement, taken
// from the context (rule 1, invariants 4 and 5), and no method takes it as a parameter.
type StoredFileStore struct {
	db *store.DB
}

func NewStoredFileStore(db *store.DB) *StoredFileStore { return &StoredFileStore{db: db} }

var (
	// ErrStoredFileMoved — the row is not in the status the caller expected (or is soft-deleted, or
	// is not this commune's). Two completions racing on one upload: the second gets this, never a
	// second copy.
	ErrStoredFileMoved = errors.New("stored_file: tệp không còn ở trạng thái mong đợi")
	// ErrStoredFileEdge — the caller asked for a transition that is not an edge, or one that has
	// its own method (stored, purged). Refused before any SQL runs.
	ErrStoredFileEdge = errors.New("stored_file: chuyển trạng thái không hợp lệ")
	// ErrAttachmentList — an empty id, or one id twice, in a link or batch-read request.
	ErrAttachmentList = errors.New("task_log_attachment: danh sách tệp không hợp lệ")
)

// MaxLogEntriesPerAttachmentRead bounds one batched read — one timeline page, with room. A caller
// asking for more is reading something that is not a page.
const MaxLogEntriesPerAttachmentRead = 200

// Read by position, in lockstep with scanStoredFile.
const storedFileCols = `id, bucket, object_key, retention_class, purpose, subject_type, subject_id,
	original_name, mime_type, size_bytes, sha256, status, uploaded_by, retain_until, legal_hold,
	created_at, updated_at`

// InsertPending records an issued upload (ADR 0052 §1a) INSIDE the caller's transaction, with its
// audit entry. `status` IS A LITERAL: no layer above can create a row that is already stored.
// The measured facts, retain_until and the purge/delete columns are NULL by construction.
func (s *StoredFileStore) InsertPending(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile) error {
	const stmt = `INSERT INTO stored_file (
		tenant_id, id, bucket, object_key, retention_class, purpose, subject_type, subject_id,
		original_name, status, uploaded_by, legal_hold, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending',$10,false,$11,$11)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), f.ID, f.Bucket, f.ObjectKey,
		f.RetentionClass, f.Purpose, f.SubjectType, f.SubjectID, f.OriginalName, f.UploadedBy,
		f.CreatedAt); err != nil {
		return fmt.Errorf("stored_file: chèn tệp chờ tải: %w", err)
	}
	return nil
}

const storedFileForUpdateStmt = `SELECT ` + storedFileCols + ` FROM stored_file ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

// ForUpdate reads one live (not soft-deleted) row inside the transaction and LOCKS it: the complete
// step reads, checks subject and uploader, then transitions. nil means no such live row in this
// commune — the caller answers the same way it answers "not yours" (rule 4, forbidden #2).
func (s *StoredFileStore) ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error) {
	row := tx.Underlying().QueryRowContext(ctx, storedFileForUpdateStmt, string(tx.TenantID()), id)
	f, err := scanStoredFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func scanStoredFile(row interface{ Scan(...any) error }) (domain.StoredFile, error) {
	var (
		f           domain.StoredFile
		status      string
		mime, sha   sql.NullString
		size        sql.NullInt64
		retainUntil sql.NullTime
	)
	if err := row.Scan(&f.ID, &f.Bucket, &f.ObjectKey, &f.RetentionClass, &f.Purpose,
		&f.SubjectType, &f.SubjectID, &f.OriginalName, &mime, &size, &sha, &status, &f.UploadedBy,
		&retainUntil, &f.LegalHold, &f.CreatedAt, &f.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StoredFile{}, err
		}
		return domain.StoredFile{}, fmt.Errorf("stored_file: quét dòng: %w", err)
	}
	f.Status = domain.StoredFileStatus(status)
	f.MIMEType, f.SizeBytes, f.SHA256 = mime.String, size.Int64, sha.String
	if retainUntil.Valid {
		f.RetainUntil = retainUntil.Time
	}
	return f, nil
}

// Transition moves a live row from `from` to `to` — a status-only edge. `from` IS IN THE WHERE, so
// a row that moved since the caller read it is not moved again (ErrStoredFileMoved).
//
// `stored` and `purged` are refused here: the first carries the measured facts (MarkStored), the
// second carries who, when and why and is the purge worker's (ADR 0052 §6), not built in this pass.
func (s *StoredFileStore) Transition(ctx context.Context, tx *store.ScopedTx, id string,
	from, to domain.StoredFileStatus, at time.Time) error {

	if !from.CanMoveTo(to) || to == domain.StoredFileStored || to == domain.StoredFilePurged {
		return ErrStoredFileEdge
	}
	const stmt = `UPDATE stored_file SET status = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2 AND status = $3 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, string(from), string(to), at)
	if err != nil {
		return fmt.Errorf("stored_file: chuyển trạng thái: %w", err)
	}
	return oneStoredFileRow(res, "chuyển trạng thái")
}

// MarkStored is `scanning → stored`, writing what the complete step measured. The schema makes the
// three facts write-once (stored_file_guard), so a second completion cannot rewrite the hash.
func (s *StoredFileStore) MarkStored(ctx context.Context, tx *store.ScopedTx, id string,
	facts domain.StoredFileFacts, at time.Time) error {

	const stmt = `UPDATE stored_file SET status = 'stored', mime_type = $3, size_bytes = $4,
		sha256 = $5, updated_at = $6
		WHERE tenant_id = $1 AND id = $2 AND status = 'scanning' AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, facts.MIMEType, facts.SizeBytes,
		facts.SHA256, at)
	if err != nil {
		return fmt.Errorf("stored_file: ghi tệp đã lưu: %w", err)
	}
	return oneStoredFileRow(res, "ghi tệp đã lưu")
}

func oneStoredFileRow(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("stored_file: %s, đếm dòng: %w", what, err)
	}
	if n == 0 {
		return ErrStoredFileMoved
	}
	return nil
}

// The two link tables a log entry's files live in — a CLOSED list, so the table name concatenated into
// the statements below is never a value from a request. task_log_attachment is migration 0021's,
// petition_log_attachment migration 0027's; their columns and triggers have the same shape.
const (
	taskLinkTable     = "task_log_attachment"
	petitionLinkTable = "petition_log_attachment"
)

// linkInsertHead: $1 is the commune and $2 the entry, in every VALUES tuple; the file ids start at $3.
func linkInsertHead(table string) string {
	return `INSERT INTO ` + table + ` (tenant_id, log_entry_id, stored_file_id) VALUES `
}

// LinkToLogEntry attaches files to a log entry, in ONE statement, INSIDE THE TRANSACTION THAT
// WROTE THE ENTRY (GhiNhatKy) — the schema refuses a link to an entry written by another
// transaction, to another task's file, to another author's file, or to a file not yet stored
// (task_log_attachment_check). The link is append-only: there is no method that removes one.
//
// An empty list writes nothing. An empty id or a duplicate is refused before any SQL.
func (s *StoredFileStore) LinkToLogEntry(ctx context.Context, tx *store.ScopedTx, logEntryID string,
	fileIDs []string) error {
	return s.linkToLogEntry(ctx, tx, taskLinkTable, logEntryID, fileIDs)
}

// LinkToPetitionLogEntry is LinkToLogEntry for a PETITION log entry (migration 0027): the schema
// refuses a link to an entry another transaction wrote, to another petition's file, to another
// author's file, to a file that is not a `petition-log-attachment`, or to one not yet stored
// (petition_log_attachment_check).
func (s *StoredFileStore) LinkToPetitionLogEntry(ctx context.Context, tx *store.ScopedTx, logEntryID string,
	fileIDs []string) error {
	return s.linkToLogEntry(ctx, tx, petitionLinkTable, logEntryID, fileIDs)
}

func (s *StoredFileStore) linkToLogEntry(ctx context.Context, tx *store.ScopedTx, table, logEntryID string,
	fileIDs []string) error {

	if len(fileIDs) == 0 {
		return nil
	}
	if logEntryID == "" || !distinctNonEmpty(fileIDs) {
		return ErrAttachmentList
	}
	tuples := make([]string, 0, len(fileIDs))
	args := make([]any, 0, len(fileIDs)+2)
	args = append(args, string(tx.TenantID()), logEntryID)
	for i, id := range fileIDs {
		tuples = append(tuples, "($1, $2, $"+strconv.Itoa(i+3)+")")
		args = append(args, id)
	}
	stmt := linkInsertHead(table) + strings.Join(tuples, ", ")
	if _, err := tx.Exec(ctx, stmt, args...); err != nil {
		return fmt.Errorf("%s: gắn tệp vào nhật ký: %w", table, err)
	}
	return nil
}

func distinctNonEmpty(ids []string) bool {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// Both tables are constrained to $1 (core/store QueryJoin's contract): joining on id alone would
// match another commune's row wherever ids collide.
func attachmentsByEntriesStmt(table string) string {
	return `SELECT a.log_entry_id, f.id, f.original_name, f.mime_type,
	f.size_bytes, f.status FROM ` + table + ` a
	JOIN stored_file f ON f.tenant_id = $1 AND f.id = a.stored_file_id
	WHERE a.tenant_id = $1 AND a.log_entry_id = ANY($2) AND f.deleted_at IS NULL
	ORDER BY a.log_entry_id, a.attached_at, f.id`
}

// AttachmentsByLogEntries reads the attachments of a page of log entries in ONE statement, keyed by
// entry id; an entry with none is absent from the map. SOFT-DELETED FILES ARE EXCLUDED (rule 7,
// invariant 2). A `purged` file IS returned: the entry did carry it, and the reader says it was
// removed under the retention schedule rather than pretending it never existed.
//
// The caller passes entry ids it has just read for one task of this commune; the commune bound
// here is the context's, so another commune's id matches nothing.
func (s *StoredFileStore) AttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
	map[string][]domain.TaskLogAttachment, error) {
	return s.attachmentsByLogEntries(ctx, taskLinkTable, logEntryIDs)
}

// PetitionAttachmentsByLogEntries is AttachmentsByLogEntries for one page of a PETITION timeline
// (petition_log_attachment, migration 0027). STAFF-ONLY: no citizen route may call it (rule 4,
// forbidden #5).
func (s *StoredFileStore) PetitionAttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
	map[string][]domain.PetitionLogAttachment, error) {
	return s.attachmentsByLogEntries(ctx, petitionLinkTable, logEntryIDs)
}

func (s *StoredFileStore) attachmentsByLogEntries(ctx context.Context, table string, logEntryIDs []string) (
	map[string][]domain.TaskLogAttachment, error) {

	out := make(map[string][]domain.TaskLogAttachment)
	if len(logEntryIDs) == 0 {
		return out, nil
	}
	if len(logEntryIDs) > MaxLogEntriesPerAttachmentRead || !distinctNonEmpty(logEntryIDs) {
		return nil, ErrAttachmentList
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, attachmentsByEntriesStmt(table), logEntryIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: đọc tệp đính kèm: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			a      domain.TaskLogAttachment
			mime   sql.NullString
			size   sql.NullInt64
			status string
		)
		if err := rows.Scan(&a.LogEntryID, &a.FileID, &a.OriginalName, &mime, &size, &status); err != nil {
			return nil, fmt.Errorf("task_log_attachment: quét dòng: %w", err)
		}
		a.MIMEType, a.SizeBytes, a.Status = mime.String, size.Int64, domain.StoredFileStatus(status)
		out[a.LogEntryID] = append(out[a.LogEntryID], a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task_log_attachment: duyệt kết quả: %w", err)
	}
	return out, nil
}

// ByID reads one live (not soft-deleted) row of this commune, WITHOUT a lock — the completion
// step's pre-read (before any object-store I/O) and the download. nil means no such live row here;
// the caller answers it exactly as it answers "not yours" (rule 4, forbidden #2).
func (s *StoredFileStore) ByID(ctx context.Context, id string) (*domain.StoredFile, error) {
	rows, err := s.db.For(ctx).Query(ctx, storedFileCols, "stored_file",
		"AND id = $2 AND deleted_at IS NULL", id)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc tệp: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("stored_file: đọc tệp: %w", err)
		}
		return nil, nil
	}
	f, err := scanStoredFile(rows)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// LinkedLogEntry returns the log entry a file is attached to, "" when it is on none. A file is on at
// most one entry (primary key (tenant_id, stored_file_id)), so there is no list to choose from.
func (s *StoredFileStore) LinkedLogEntry(ctx context.Context, fileID string) (string, error) {
	return s.linkedLogEntry(ctx, taskLinkTable, fileID)
}

// LinkedPetitionLogEntry is LinkedLogEntry for a petition log attachment (migration 0027).
func (s *StoredFileStore) LinkedPetitionLogEntry(ctx context.Context, fileID string) (string, error) {
	return s.linkedLogEntry(ctx, petitionLinkTable, fileID)
}

func (s *StoredFileStore) linkedLogEntry(ctx context.Context, table, fileID string) (string, error) {
	// Scoped.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := s.db.For(ctx).Query(ctx, "log_entry_id", table, "AND stored_file_id = $2", fileID)
	if err != nil {
		return "", fmt.Errorf("%s: đọc dòng gắn tệp: %w", table, err)
	}
	defer rows.Close()
	var entry string
	if rows.Next() {
		if err := rows.Scan(&entry); err != nil {
			return "", fmt.Errorf("task_log_attachment: quét dòng gắn tệp: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("task_log_attachment: duyệt dòng gắn tệp: %w", err)
	}
	return entry, nil
}

// countForSubjectTail is platform's `max_files_per_subject` count (proto UploadPolicy): the files of
// one purpose on one subject that are NOT deleted, failed, rejected — and not purged either, because
// a purged row is one that came from failed/rejected (records are never purged, stored_file_guard),
// and counting it would bring a refused file back into the limit.
//
// A `pending` / `scanning` row counts ONLY WHILE IT CAN STILL RECEIVE BYTES — created at or after
// `pendingSince`, which the caller sets to now minus the presigned POST's lifetime. A row whose form
// expired unused would otherwise hold one slot of the limit for ever: nothing moves an abandoned
// upload out of `pending` (no worker; a completion call that finds nothing marks it `failed`, but only
// if somebody makes that call).
const countForSubjectTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4
	AND deleted_at IS NULL
	AND (status IN ('stored', 'processing', 'ready')
	     OR (status IN ('pending', 'scanning') AND created_at >= $5))`

// countStoredForSubjectTail is the same count with no `pending` row in it — the completion step's
// re-check, where the row being completed is itself pending and must not count against itself.
const countStoredForSubjectTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4
	AND deleted_at IS NULL AND status IN ('stored', 'processing', 'ready')`

// CountForSubjectTx counts inside the caller's transaction — the upload request, which holds the
// task row FOR UPDATE, so two requests for one task are serialised and cannot both pass the limit.
// A zero pendingSince counts no pending row at all.
func (s *StoredFileStore) CountForSubjectTx(ctx context.Context, tx *store.ScopedTx,
	subjectType, subjectID, purpose string, pendingSince time.Time) (int, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	if pendingSince.IsZero() {
		return scanCount(tx.Query(ctx, "count(*)", "stored_file", countStoredForSubjectTail,
			subjectType, subjectID, purpose))
	}
	// Same: `tenant_id = $1` is ScopedTx.Query's, never this call's.
	return scanCount(tx.Query(ctx, "count(*)", "stored_file", countForSubjectTail,
		subjectType, subjectID, purpose, pendingSince))
}

// CountForSubject is CountForSubjectTx outside a transaction — the completion step's pre-check,
// which runs BEFORE the object leaves the temp bucket (a file promoted to the private bucket is a
// `records` object and can never be purged again, so the refusal has to come first).
func (s *StoredFileStore) CountForSubject(ctx context.Context,
	subjectType, subjectID, purpose string, pendingSince time.Time) (int, error) {

	if pendingSince.IsZero() {
		return scanCount(s.db.For(ctx).Query(ctx, "count(*)", "stored_file", countStoredForSubjectTail,
			subjectType, subjectID, purpose))
	}
	return scanCount(s.db.For(ctx).Query(ctx, "count(*)", "stored_file", countForSubjectTail,
		subjectType, subjectID, purpose, pendingSince))
}

func scanCount(rows *sql.Rows, err error) (int, error) {
	if err != nil {
		return 0, fmt.Errorf("stored_file: đếm tệp: %w", err)
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("stored_file: quét số đếm: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("stored_file: duyệt số đếm: %w", err)
	}
	return n, nil
}

// MaxAttachCandidates bounds one link request's id list. A log entry can only carry files the same
// officer uploaded for the same task, so this bounds the STATEMENT, not the business: the business
// limit is platform's per-task count, enforced when the uploads were issued.
const MaxAttachCandidates = 100

// attachCandidatesHead is read by position in lockstep with AttachCandidates' Scan: storedFileCols,
// then the link. `FOR UPDATE OF f`: the outer join's nullable side cannot be locked, and the file row
// is the one a concurrent soft delete or a second link would touch.
func attachCandidatesHead(table string) string {
	return `SELECT ` + storedFileCols + `, a.log_entry_id FROM stored_file f
	LEFT JOIN ` + table + ` a ON a.tenant_id = $1 AND a.stored_file_id = f.id
	WHERE f.tenant_id = $1 AND f.deleted_at IS NULL AND f.id IN (`
}

// AttachCandidates reads the files a log entry asks to carry, INSIDE the entry's transaction, each
// with the entry it is already on (if any), and LOCKS the file rows. Ids that match no live row of
// this commune are simply absent from the map. Both tables are constrained to $1.
func (s *StoredFileStore) AttachCandidates(ctx context.Context, tx *store.ScopedTx, ids []string) (
	map[string]domain.AttachCandidate, error) {
	return s.attachCandidates(ctx, tx, taskLinkTable, ids)
}

// PetitionAttachCandidates is AttachCandidates for a PETITION log entry: each file with the petition
// log entry it is already on (petition_log_attachment), locked FOR UPDATE.
func (s *StoredFileStore) PetitionAttachCandidates(ctx context.Context, tx *store.ScopedTx, ids []string) (
	map[string]domain.AttachCandidate, error) {
	return s.attachCandidates(ctx, tx, petitionLinkTable, ids)
}

func (s *StoredFileStore) attachCandidates(ctx context.Context, tx *store.ScopedTx, table string, ids []string) (
	map[string]domain.AttachCandidate, error) {

	out := make(map[string]domain.AttachCandidate, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > MaxAttachCandidates || !distinctNonEmpty(ids) {
		return nil, ErrAttachmentList
	}
	marks := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, string(tx.TenantID()))
	for i, id := range ids {
		marks = append(marks, "$"+strconv.Itoa(i+2))
		args = append(args, id)
	}
	stmt := attachCandidatesHead(table) + strings.Join(marks, ", ") + `) FOR UPDATE OF f`
	// $1 is tx.TenantID() and BOTH tables carry `tenant_id = $1` (attachCandidatesHead).
	rows, err := tx.Underlying().QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc tệp để gắn: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			c           domain.AttachCandidate
			status      string
			mime, sha   sql.NullString
			size        sql.NullInt64
			retainUntil sql.NullTime
			linked      sql.NullString
		)
		f := &c.File
		if err := rows.Scan(&f.ID, &f.Bucket, &f.ObjectKey, &f.RetentionClass, &f.Purpose,
			&f.SubjectType, &f.SubjectID, &f.OriginalName, &mime, &size, &sha, &status, &f.UploadedBy,
			&retainUntil, &f.LegalHold, &f.CreatedAt, &f.UpdatedAt, &linked); err != nil {
			return nil, fmt.Errorf("stored_file: quét tệp để gắn: %w", err)
		}
		f.Status = domain.StoredFileStatus(status)
		f.MIMEType, f.SizeBytes, f.SHA256 = mime.String, size.Int64, sha.String
		if retainUntil.Valid {
			f.RetainUntil = retainUntil.Time
		}
		c.LinkedTo = linked.String
		out[f.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt tệp để gắn: %w", err)
	}
	return out, nil
}
