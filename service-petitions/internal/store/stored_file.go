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

// $1 is the commune and $2 the entry, in every VALUES tuple; the file ids start at $3.
const linkInsertHead = `INSERT INTO task_log_attachment (tenant_id, log_entry_id, stored_file_id) VALUES `

// LinkToLogEntry attaches files to a log entry, in ONE statement, INSIDE THE TRANSACTION THAT
// WROTE THE ENTRY (GhiNhatKy) — the schema refuses a link to an entry written by another
// transaction, to another task's file, to another author's file, or to a file not yet stored
// (task_log_attachment_check). The link is append-only: there is no method that removes one.
//
// An empty list writes nothing. An empty id or a duplicate is refused before any SQL.
func (s *StoredFileStore) LinkToLogEntry(ctx context.Context, tx *store.ScopedTx, logEntryID string,
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
	stmt := linkInsertHead + strings.Join(tuples, ", ")
	if _, err := tx.Exec(ctx, stmt, args...); err != nil {
		return fmt.Errorf("task_log_attachment: gắn tệp vào nhật ký: %w", err)
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
const attachmentsByEntriesStmt = `SELECT a.log_entry_id, f.id, f.original_name, f.mime_type,
	f.size_bytes, f.status FROM task_log_attachment a
	JOIN stored_file f ON f.tenant_id = $1 AND f.id = a.stored_file_id
	WHERE a.tenant_id = $1 AND a.log_entry_id = ANY($2) AND f.deleted_at IS NULL
	ORDER BY a.log_entry_id, a.attached_at, f.id`

// AttachmentsByLogEntries reads the attachments of a page of log entries in ONE statement, keyed by
// entry id; an entry with none is absent from the map. SOFT-DELETED FILES ARE EXCLUDED (rule 7,
// invariant 2). A `purged` file IS returned: the entry did carry it, and the reader says it was
// removed under the retention schedule rather than pretending it never existed.
//
// The caller passes entry ids it has just read for one task of this commune; the commune bound
// here is the context's, so another commune's id matches nothing.
func (s *StoredFileStore) AttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
	map[string][]domain.TaskLogAttachment, error) {

	out := make(map[string][]domain.TaskLogAttachment)
	if len(logEntryIDs) == 0 {
		return out, nil
	}
	if len(logEntryIDs) > MaxLogEntriesPerAttachmentRead || !distinctNonEmpty(logEntryIDs) {
		return nil, ErrAttachmentList
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, attachmentsByEntriesStmt, logEntryIDs)
	if err != nil {
		return nil, fmt.Errorf("task_log_attachment: đọc tệp đính kèm: %w", err)
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
