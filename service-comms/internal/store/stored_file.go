package store

// The only path to this service's `stored_file` (migration 0011, entity CommsStoredFile, ADR 0052 §5).
// SQL, and nothing else: which purpose, which limit, who may upload, when a derivative is published
// and what is audited are internal/app's (content_cover.go).
//
// THE SHAPE IS service-petitions/internal/store/stored_file.go's — the table is 0021's — minus the
// task attachment link, plus `public_object_key`.
//
// ⚠ `original_name` IS PERSONAL DATA when it names a person (rule 3). No error built here quotes it,
// nor an object key.
//
// NO audit.Write IN THIS FILE — audit_guard warns on this shape, so the answer is written down. Every
// write method takes the caller's *store.ScopedTx, and internal/app writes the entry inside that same
// transaction (rule 6, invariant 3); the actor and the business verb do not exist at the SQL layer.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// StoredFileStore holds *store.DB, never a *sql.DB: the commune is $1 in every statement, taken
// from the context (rule 1, invariants 4 and 5), and no method takes it as a parameter.
type StoredFileStore struct {
	db *store.DB
}

func NewStoredFileStore(db *store.DB) *StoredFileStore { return &StoredFileStore{db: db} }

var (
	// ErrStoredFileMoved — the row is not in the status the caller expected (or is soft-deleted, or
	// is not this commune's). Two completions racing on one upload: the second gets this.
	ErrStoredFileMoved = errors.New("stored_file: tệp không còn ở trạng thái mong đợi")
	// ErrStoredFileEdge — a transition that is not an edge, or one with its own method (stored,
	// purged). Refused before any SQL runs.
	ErrStoredFileEdge = errors.New("stored_file: chuyển trạng thái không hợp lệ")
	// ErrStoredFileList — an empty id, a duplicate, or too many ids in a batch read.
	ErrStoredFileList = errors.New("stored_file: danh sách tệp không hợp lệ")
)

// MaxStoredFileBatch bounds one batch read — a page of the public list (page.MaxLimit is 100), with
// room. A caller asking for more is reading something that is not a page.
const MaxStoredFileBatch = 200

// Read by position, in lockstep with scanStoredFile.
const storedFileCols = `id, bucket, object_key, retention_class, purpose, subject_type, subject_id,
	original_name, mime_type, size_bytes, sha256, status, public_object_key, uploaded_by,
	created_at, updated_at`

// InsertPending records an issued upload (ADR 0052 §1a) INSIDE the caller's transaction, with its
// audit entry. `status` IS A LITERAL: no layer above can create a row that is already stored. The
// measured facts, the public key, retain_until and the purge/delete columns are NULL by construction.
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

// ForUpdate reads one live row inside the transaction and LOCKS it. nil means no such live row in
// this commune — the caller answers it like "not yours" (rule 4, forbidden #2).
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

// ByID reads one live row of this commune WITHOUT a lock — the completion's pre-read (before any
// object-store I/O), the publish pre-check and the staff preview. nil means no such live row here.
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

func scanStoredFile(row interface{ Scan(...any) error }) (domain.StoredFile, error) {
	var (
		f              domain.StoredFile
		status         string
		mime, sha, pub sql.NullString
		size           sql.NullInt64
	)
	if err := row.Scan(&f.ID, &f.Bucket, &f.ObjectKey, &f.RetentionClass, &f.Purpose,
		&f.SubjectType, &f.SubjectID, &f.OriginalName, &mime, &size, &sha, &status, &pub,
		&f.UploadedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StoredFile{}, err
		}
		return domain.StoredFile{}, fmt.Errorf("stored_file: quét dòng: %w", err)
	}
	f.Status = domain.StoredFileStatus(status)
	f.MIMEType, f.SizeBytes, f.SHA256, f.PublicObjectKey = mime.String, size.Int64, sha.String, pub.String
	return f, nil
}

// Transition moves a live row from `from` to `to` — a status-only edge. `from` IS IN THE WHERE, so a
// row that moved since the caller read it is not moved again (ErrStoredFileMoved).
//
// `stored` and `purged` are refused here: the first carries the measured facts (MarkStored), the
// second carries who, when and why and belongs to the purge worker (ADR 0052 §6), not built.
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

// MarkStored is `scanning → stored`, writing what the completion measured. The schema makes the
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

// SetPublicObjectKey records (key != "") or clears (key == "") the published derivative's public key.
// `status = 'ready'` IS IN THE WHERE: 0011's `stored_file_public_only_when_ready` refuses a key on any
// other row, and a zero-row answer here is a sentence instead of a constraint exception.
//
// The caller records a key only AFTER PublishDerivative succeeded, and clears it only AFTER
// UnpublishDerivative did — so a set key means "a public copy may exist", the marker a retry reads.
func (s *StoredFileStore) SetPublicObjectKey(ctx context.Context, tx *store.ScopedTx, id, key string,
	at time.Time) error {

	const stmt = `UPDATE stored_file SET public_object_key = $3, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'ready' AND deleted_at IS NULL`
	var bind any
	if key != "" {
		bind = key
	}
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, bind, at)
	if err != nil {
		return fmt.Errorf("stored_file: ghi khoá bản công khai: %w", err)
	}
	return oneStoredFileRow(res, "ghi khoá bản công khai")
}

const publicForSubjectStmt = `SELECT ` + storedFileCols + ` FROM stored_file
	WHERE tenant_id = $1 AND subject_type = $2 AND subject_id = $3
	AND public_object_key IS NOT NULL AND deleted_at IS NULL
	ORDER BY id FOR UPDATE`

// PublicForSubject reads, inside the transaction and LOCKED, every file of one article that still
// carries a public key, OF EVERY PURPOSE — the current cover and body images when published, and any
// earlier one whose withdrawal has not been confirmed yet. Soft-deleted rows cannot carry one (0011's
// CHECK). Deciding which of them stay public is the caller's (app coverPublisher.settle, by purpose):
// SQL here would have to know what an article's body references, and only the app layer does.
func (s *StoredFileStore) PublicForSubject(ctx context.Context, tx *store.ScopedTx, subjectID string) (
	[]domain.StoredFile, error) {

	rows, err := tx.Underlying().QueryContext(ctx, publicForSubjectStmt, string(tx.TenantID()),
		domain.StoredFileSubjectContentItem, subjectID)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc bản công khai của mục: %w", err)
	}
	defer rows.Close()
	var out []domain.StoredFile
	for rows.Next() {
		f, err := scanStoredFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt bản công khai của mục: %w", err)
	}
	return out, nil
}

// PublicObjectKeys reads the public key of each of a page's cover files in ONE statement, keyed by
// file id; a file with no public key (or no live row) is absent. For the public news routes.
func (s *StoredFileStore) PublicObjectKeys(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > MaxStoredFileBatch || !distinctNonEmpty(ids) {
		return nil, ErrStoredFileList
	}
	marks := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		marks = append(marks, "$"+strconv.Itoa(i+2)) // $1 is the commune (Scoped.Query)
		args = append(args, id)
	}
	rows, err := s.db.For(ctx).Query(ctx, "id, public_object_key", "stored_file",
		"AND deleted_at IS NULL AND public_object_key IS NOT NULL AND id IN ("+
			strings.Join(marks, ", ")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc khoá bản công khai: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, fmt.Errorf("stored_file: quét khoá bản công khai: %w", err)
		}
		out[id] = key
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt khoá bản công khai: %w", err)
	}
	return out, nil
}

// SoftDelete retires one live row (rule 7: `deleted_at` / `deleted_by` / `delete_reason`, never a
// DELETE). The object stays in the bucket — the purge worker, not built, owns that (ADR 0052 §6). Used
// when an item stops pointing at its broadcast audio, so platform's one-file-per-item count
// (`max_files_per_subject` of content-audio) frees the slot for a replacement. `by` is a STAFF BUSINESS
// CODE (rule 6, invariant 8); 0011's CHECK refuses an empty reason. A row with a public key is not
// touched (zero rows → ErrStoredFileMoved): a published copy is withdrawn first, never orphaned.
func (s *StoredFileStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string,
	at time.Time) error {

	const stmt = `UPDATE stored_file SET deleted_at = $3, deleted_by = $4, delete_reason = $5,
		updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND public_object_key IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, at, by, reason)
	if err != nil {
		return fmt.Errorf("stored_file: xoá mềm tệp: %w", err)
	}
	return oneStoredFileRow(res, "xoá mềm tệp")
}

// ReadyObjectKeys reads the PRIVATE object key of each `ready`, live file of ONE purpose among ids, in
// one statement, keyed by file id; anything else is absent. For the public news routes' audio link
// (ADR 0067 §4.2): the key is presigned, never published, and never leaves this service.
func (s *StoredFileStore) ReadyObjectKeys(ctx context.Context, purpose string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > MaxStoredFileBatch || !distinctNonEmpty(ids) || purpose == "" {
		return nil, ErrStoredFileList
	}
	marks := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, purpose) // $2; $1 is the commune (Scoped.Query)
	for i, id := range ids {
		marks = append(marks, "$"+strconv.Itoa(i+3))
		args = append(args, id)
	}
	rows, err := s.db.For(ctx).Query(ctx, "id, object_key", "stored_file",
		"AND deleted_at IS NULL AND status = 'ready' AND purpose = $2 AND id IN ("+
			strings.Join(marks, ", ")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc khoá tệp sẵn sàng: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, fmt.Errorf("stored_file: quét khoá tệp sẵn sàng: %w", err)
		}
		out[id] = key
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt khoá tệp sẵn sàng: %w", err)
	}
	return out, nil
}

// LIMIT is MaxStoredFileBatch + 1: the one extra row is how "too many" is told apart from "exactly enough".
var liveForSubjectTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4
	AND deleted_at IS NULL ORDER BY id LIMIT ` + strconv.Itoa(MaxStoredFileBatch+1)

// LiveForSubject reads every live file of ONE purpose on ONE article in one statement — the body images
// of an article for its staff preview and its public read (ADR 0067 §Sửa đổi 03/10/2026, K2/K7), so a
// detail with twenty images is one query, never twenty. More than MaxStoredFileBatch rows is refused
// (ErrStoredFileList) rather than silently cut: a cut list would drop images from a published article.
func (s *StoredFileStore) LiveForSubject(ctx context.Context, subjectID, purpose string) ([]domain.StoredFile, error) {
	if subjectID == "" || purpose == "" {
		return nil, ErrStoredFileList
	}
	// Scoped.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := s.db.For(ctx).Query(ctx, storedFileCols, "stored_file", liveForSubjectTail,
		domain.StoredFileSubjectContentItem, subjectID, purpose)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc tệp của mục: %w", err)
	}
	defer rows.Close()
	var out []domain.StoredFile
	for rows.Next() {
		f, err := scanStoredFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt tệp của mục: %w", err)
	}
	if len(out) > MaxStoredFileBatch {
		return nil, ErrStoredFileList
	}
	return out, nil
}

// SubjectReservedBy reports, inside the transaction, whether THIS OFFICER holds a live upload on a
// subject id — i.e. the id was minted for them by an upload request (ADR 0052 §1a) for an article not
// saved yet. It is what lets a second upload (another body image, or the cover after a body image) name
// the same unsaved article; anybody else's minted id answers false, like an unknown one.
func (s *StoredFileStore) SubjectReservedBy(ctx context.Context, tx *store.ScopedTx, subjectID,
	uploadedBy string) (bool, error) {

	if subjectID == "" || uploadedBy == "" {
		return false, nil
	}
	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	n, err := scanStoredFileCount(tx.Query(ctx, "count(*)", "stored_file",
		`AND subject_type = $2 AND subject_id = $3 AND uploaded_by = $4 AND deleted_at IS NULL`,
		domain.StoredFileSubjectContentItem, subjectID, uploadedBy))
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// countForSubjectTail is platform's `max_files_per_subject` count: the files of one purpose on one
// subject that are not deleted, failed, rejected or purged. A `pending` / `scanning` row counts only
// while its form can still receive bytes — created at or after `pendingSince` — so an abandoned upload
// does not hold a slot for ever (petitions' count, same reasoning).
const countForSubjectTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4
	AND deleted_at IS NULL
	AND (status IN ('stored', 'processing', 'ready')
	     OR (status IN ('pending', 'scanning') AND created_at >= $5))`

// countStoredForSubjectTail has no pending row in it — the completion's re-check, where the row
// being completed is itself pending and must not count against itself.
const countStoredForSubjectTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4
	AND deleted_at IS NULL AND status IN ('stored', 'processing', 'ready')`

// LockSubjectCount serialises, until the caller's transaction ends, every admission of a file of one
// purpose onto one subject in this commune. Call it BEFORE CountForSubjectTx, in the transaction that
// then inserts the row.
//
// WHY AN ADVISORY LOCK AND NOT THE ARTICLE ROW: `max_files_per_subject` is decided by counting and then
// inserting. When the article exists its row FOR UPDATE serialises that, but an article not saved yet
// (a reserved id, ADR 0052 §1a) has no row to lock, and count(*) under READ COMMITTED does not see the
// other request's uncommitted insert — two requests at 19 both pass and the article holds 21 body images
// (H7 broken silently). The key is tenant-prefixed (rule 1, invariant 7), so no commune waits on another;
// a hash collision only over-serialises. Released by COMMIT or ROLLBACK — no path forgets it.
func (s *StoredFileStore) LockSubjectCount(ctx context.Context, tx *store.ScopedTx, subjectID, purpose string) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtextextended(` +
		`'t:' || $1 || ':' || $2 || ':' || $3 || ':' || $4, 0))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), domain.StoredFileSubjectContentItem,
		subjectID, purpose); err != nil {
		return fmt.Errorf("stored_file: khoá số đếm tệp của mục: %w", err)
	}
	return nil
}

// CountForSubjectTx counts inside the caller's transaction (the upload request), live pending rows
// included from pendingSince on. The caller holds LockSubjectCount for the same subject and purpose.
func (s *StoredFileStore) CountForSubjectTx(ctx context.Context, tx *store.ScopedTx,
	subjectID, purpose string, pendingSince time.Time) (int, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	return scanStoredFileCount(tx.Query(ctx, "count(*)", "stored_file", countForSubjectTail,
		domain.StoredFileSubjectContentItem, subjectID, purpose, pendingSince))
}

// CountForSubject is the completion's pre-check, outside a transaction, before the object leaves temp.
func (s *StoredFileStore) CountForSubject(ctx context.Context, subjectID, purpose string) (int, error) {
	// Scoped.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	return scanStoredFileCount(s.db.For(ctx).Query(ctx, "count(*)", "stored_file", countStoredForSubjectTail,
		domain.StoredFileSubjectContentItem, subjectID, purpose))
}

func scanStoredFileCount(rows *sql.Rows, err error) (int, error) {
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
