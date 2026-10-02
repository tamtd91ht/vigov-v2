package store

// The only path to this service's `stored_file` (migration 0017, entity PlatformStoredFile, ADR 0052
// §5). SQL, and nothing else: which purpose, which limit, who may upload, when a derivative is published
// and what is audited are internal/app's (branding.go).
//
// THE SHAPE IS service-comms/internal/store/stored_file.go's — 0017 is comms 0011 column for column —
// minus the batch reads (one commune has at most one current logo and one current banner), plus a
// read of every published file of one purpose (the withdrawal list of a replacement).
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
	"time"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// StoredFileStore holds *corestore.DB, never a *sql.DB: the commune is $1 in every statement, taken
// from the context (rule 1, invariants 4 and 5), and no method takes it as a parameter.
type StoredFileStore struct {
	db *corestore.DB
}

func NewStoredFileStore(db *corestore.DB) *StoredFileStore { return &StoredFileStore{db: db} }

var (
	// ErrStoredFileMoved — the row is not in the status the caller expected (or is soft-deleted, or
	// is not this commune's). Two completions racing on one upload: the second gets this.
	ErrStoredFileMoved = errors.New("stored_file: tệp không còn ở trạng thái mong đợi")
	// ErrStoredFileEdge — a transition that is not an edge, or one with its own method (stored,
	// purged). Refused before any SQL runs.
	ErrStoredFileEdge = errors.New("stored_file: chuyển trạng thái không hợp lệ")
)

// Read by position, in lockstep with scanStoredFile.
const storedFileCols = `id, bucket, object_key, retention_class, purpose, subject_type, subject_id,
	original_name, mime_type, size_bytes, sha256, status, public_object_key, uploaded_by,
	created_at, updated_at`

// InsertPending records an issued upload (ADR 0052 §1a) INSIDE the caller's transaction. `status` IS A
// LITERAL: no layer above can create a row that is already stored. subject_id is the commune ITSELF,
// bound from the transaction — never a value the caller could get wrong (0017's CHECK holds the same).
func (s *StoredFileStore) InsertPending(ctx context.Context, tx *corestore.ScopedTx, f domain.StoredFile) error {
	const stmt = `INSERT INTO stored_file (
		tenant_id, id, bucket, object_key, retention_class, purpose, subject_type, subject_id,
		original_name, status, uploaded_by, legal_hold, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$1,$8,'pending',$9,false,$10,$10)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), f.ID, f.Bucket, f.ObjectKey,
		f.RetentionClass, f.Purpose, domain.StoredFileSubjectDisplayProfile, f.OriginalName,
		f.UploadedBy, f.CreatedAt); err != nil {
		return fmt.Errorf("stored_file: chèn tệp chờ tải: %w", err)
	}
	return nil
}

const storedFileForUpdateStmt = `SELECT ` + storedFileCols + ` FROM stored_file ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

// ForUpdate reads one live row inside the transaction and LOCKS it. nil means no such live row in
// this commune — the caller answers it like "not yours" (rule 4, forbidden #2).
func (s *StoredFileStore) ForUpdate(ctx context.Context, tx *corestore.ScopedTx, id string) (*domain.StoredFile, error) {
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
// object-store I/O) and the rollback compensation. nil means no such live row here.
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
func (s *StoredFileStore) Transition(ctx context.Context, tx *corestore.ScopedTx, id string,
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
func (s *StoredFileStore) MarkStored(ctx context.Context, tx *corestore.ScopedTx, id string,
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
// `status = 'ready'` IS IN THE WHERE: 0017's `stored_file_public_only_when_ready` refuses a key on any
// other row, and a zero-row answer here is a sentence instead of a constraint exception.
//
// The caller records a key only AFTER PublishDerivative succeeded, and clears it only AFTER
// UnpublishDerivative did — so a set key means "a public copy may exist", the marker a retry reads.
func (s *StoredFileStore) SetPublicObjectKey(ctx context.Context, tx *corestore.ScopedTx, id, key string,
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

const publishedOfPurposeStmt = `SELECT ` + storedFileCols + ` FROM stored_file
	WHERE tenant_id = $1 AND subject_type = $2 AND subject_id = $1 AND purpose = $3
	AND public_object_key IS NOT NULL AND deleted_at IS NULL
	ORDER BY id FOR UPDATE`

// PublishedOfPurpose reads, inside the transaction and LOCKED, every file of one purpose of this
// commune that still carries a public key — the current image, and any earlier one whose withdrawal has
// not been confirmed yet. Soft-deleted rows cannot carry one (0017's CHECK).
func (s *StoredFileStore) PublishedOfPurpose(ctx context.Context, tx *corestore.ScopedTx, purpose string) (
	[]domain.StoredFile, error) {

	rows, err := tx.Underlying().QueryContext(ctx, publishedOfPurposeStmt, string(tx.TenantID()),
		domain.StoredFileSubjectDisplayProfile, purpose)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc bản công khai theo mục đích: %w", err)
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
		return nil, fmt.Errorf("stored_file: duyệt bản công khai theo mục đích: %w", err)
	}
	return out, nil
}

// SoftDelete retires one live row (rule 7: `deleted_at` / `deleted_by` / `delete_reason`, never a
// DELETE). The object stays in the bucket — the purge worker, not built, owns that (ADR 0052 §6). `by`
// is a STAFF BUSINESS CODE (rule 6, invariant 8); 0017's CHECK refuses an empty reason. A row with a
// public key is not touched (zero rows → ErrStoredFileMoved): a published copy is withdrawn first.
func (s *StoredFileStore) SoftDelete(ctx context.Context, tx *corestore.ScopedTx, id, by, reason string,
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

// countLiveOfPurposeTail is platform's `max_files_per_subject` count for one purpose of this commune:
// files not deleted, failed, rejected or purged; a `pending` / `scanning` row counts only while its form
// can still receive bytes (created at or after $4) — comms' count, same reasoning. A replaced image is
// soft-deleted after its withdrawal, so it frees its slot.
const countLiveOfPurposeTail = `AND subject_type = $2 AND subject_id = $1 AND purpose = $3
	AND deleted_at IS NULL
	AND (status IN ('stored', 'processing', 'ready')
	     OR (status IN ('pending', 'scanning') AND created_at >= $4))`

// CountLiveOfPurpose counts inside the caller's transaction (the upload request).
func (s *StoredFileStore) CountLiveOfPurpose(ctx context.Context, tx *corestore.ScopedTx, purpose string,
	pendingSince time.Time) (int, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the context.
	rows, err := tx.Query(ctx, "count(*)", "stored_file", countLiveOfPurposeTail,
		domain.StoredFileSubjectDisplayProfile, purpose, pendingSince)
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
