package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// UploadPolicyStore is the only path to `upload_policy` (migration 0008).
//
// IT HOLDS A RAW *sql.DB, LIKE Directory, and for a reason of the same kind: the table has no
// tenant_id column, so core/store.Scoped — which injects WHERE tenant_id = $1 into every statement —
// cannot read it. It is platform-wide configuration that is the same for every commune today (a
// per-commune override is open, platform.proto ListUploadPolicies), holds no business content and
// no personal data. The one write path — ChangeUploadPolicy, Vihat's operations zone (ADR 0073 #5,
// ADR 0052 §10) — writes the row and its platform_audit_log entry in one transaction (rule 6,
// invariant 3).
type UploadPolicyStore struct {
	db *sql.DB
}

func NewUploadPolicyStore(db *sql.DB) *UploadPolicyStore { return &UploadPolicyStore{db: db} }

// listUploadPolicies reads every live policy.
//
// `deleted_at IS NULL` is the whole of "withdrawn": a soft-deleted policy is absent from the answer,
// so its purpose reads as NOT CONFIGURED and callers refuse (rule 7, invariant 2; platform.proto).
//
// allowed_mime_types travels as JSON text rather than as a native array: database/sql cannot scan a
// PostgreSQL array without a driver-specific scanner, and this keeps pgx out of the store's types.
const listUploadPolicies = `
	SELECT purpose, max_bytes, array_to_json(allowed_mime_types)::text, max_files_per_subject, updated_at, updated_by
	FROM upload_policy
	WHERE deleted_at IS NULL
	ORDER BY purpose`

// ListUploadPolicies returns every live policy, ordered by purpose. An empty slice is an ordinary
// answer ("nothing is configured"), not an error.
func (s *UploadPolicyStore) ListUploadPolicies(ctx context.Context) ([]domain.UploadPolicy, error) {
	// No tenant filter: platform-wide configuration with no commune column (see the type comment).
	rows, err := s.db.QueryContext(ctx, listUploadPolicies)
	if err != nil {
		return nil, fmt.Errorf("upload_policy: query: %w", err)
	}
	defer rows.Close()

	var out []domain.UploadPolicy
	for rows.Next() {
		p, err := scanPolicy(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("upload_policy: read: %w", err)
	}
	return out, nil
}

func scanPolicy(scan func(...any) error) (domain.UploadPolicy, error) {
	var (
		p        domain.UploadPolicy
		mimeJSON string
		maxFiles sql.NullInt32
	)
	if err := scan(&p.Purpose, &p.MaxBytes, &mimeJSON, &maxFiles, &p.UpdatedAt, &p.UpdatedBy); err != nil {
		return domain.UploadPolicy{}, fmt.Errorf("upload_policy: scan: %w", err)
	}
	if err := json.Unmarshal([]byte(mimeJSON), &p.AllowedMIMETypes); err != nil {
		return domain.UploadPolicy{}, fmt.Errorf("upload_policy: allowed_mime_types of %q: %w", p.Purpose, err)
	}
	p.FileCountLimited = maxFiles.Valid
	p.MaxFilesPerSubject = maxFiles.Int32
	return p, nil
}

// ErrUploadPolicyNotFound — no LIVE row for this purpose: never seeded, or withdrawn (soft-deleted).
// A withdrawn purpose is not re-created here: its primary key still holds the soft-deleted row, and
// bringing a withdrawn purpose back is a decision, not an edit.
var ErrUploadPolicyNotFound = errors.New("upload_policy: không có chính sách đang dùng cho mục đích này")

// ActionUploadPolicyChanged is the platform_audit_log verb of an edit — the one migrations 0012/0014
// already write for the same act, so one query finds every change whichever channel made it.
const ActionUploadPolicyChanged = "upload_policy.changed"

// policyTrail is the before/after shape every upload_policy trail entry already has (0008's seed).
func policyTrail(p domain.UploadPolicy) []byte {
	var maxFiles any
	if p.FileCountLimited {
		maxFiles = p.MaxFilesPerSubject
	}
	return delta(map[string]any{
		"max_bytes": p.MaxBytes, "allowed_mime_types": p.AllowedMIMETypes, "max_files_per_subject": maxFiles,
	})
}

func samePolicyValues(a, b domain.UploadPolicy) bool {
	am, bm := slices.Clone(a.AllowedMIMETypes), slices.Clone(b.AllowedMIMETypes)
	slices.Sort(am)
	slices.Sort(bm)
	return a.MaxBytes == b.MaxBytes && slices.Equal(am, bm) && a.FileCountLimited == b.FileCountLimited &&
		(!a.FileCountLimited || a.MaxFilesPerSubject == b.MaxFilesPerSubject)
}

// ChangeUploadPolicy sets the limits of ONE purpose, platform-wide, and appends the
// `upload_policy.changed` entry to platform_audit_log — ONE transaction (rule 6 invariant 3). `next`
// is already validated by the caller (app.ValidateUploadPolicy); the database CHECKs of 0008 are the
// second copy of the bounds.
//
// changed=false when the values are the ones the row already holds (MIME order ignored): nothing is
// written and no entry appended, the convention of every operator write.
//
// WHEN IT TAKES EFFECT: committed here at once; every reader holds its cached answer for up to
// core/platformclient/uploadpolicy.TTL (60 s), the ceiling ListUploadPolicies promises.
func (s *UploadPolicyStore) ChangeUploadPolicy(ctx context.Context, next domain.UploadPolicy, reason string,
	by domain.OperatorActor) (out domain.UploadPolicy, changed bool, err error) {
	if err := by.Validate(); err != nil {
		return domain.UploadPolicy{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.UploadPolicy{}, false, fmt.Errorf("upload_policy: begin: %w", err)
	}
	defer func() {
		if err != nil || !changed {
			_ = tx.Rollback()
		}
	}()

	before, err := scanPolicy(tx.QueryRowContext(ctx, `
		SELECT purpose, max_bytes, array_to_json(allowed_mime_types)::text, max_files_per_subject, updated_at, updated_by
		FROM upload_policy WHERE purpose = $1 AND deleted_at IS NULL FOR UPDATE`, next.Purpose).Scan)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.UploadPolicy{}, false, ErrUploadPolicyNotFound
	case err != nil:
		return domain.UploadPolicy{}, false, err
	}
	if samePolicyValues(before, next) {
		return before, false, nil
	}

	mimes, err := json.Marshal(next.AllowedMIMETypes)
	if err != nil {
		return domain.UploadPolicy{}, false, fmt.Errorf("upload_policy: mime list: %w", err)
	}
	var maxFiles sql.NullInt32
	if next.FileCountLimited {
		maxFiles = sql.NullInt32{Int32: next.MaxFilesPerSubject, Valid: true}
	}
	// The array is built from JSON text in SQL, keeping its order (WITH ORDINALITY) — the read side's
	// reason for JSON applies to binding too: no driver-specific array type in this package.
	out, err = scanPolicy(tx.QueryRowContext(ctx, `
		UPDATE upload_policy
		   SET max_bytes = $2,
		       allowed_mime_types = ARRAY(SELECT v FROM jsonb_array_elements_text($3::jsonb) WITH ORDINALITY AS x(v, n) ORDER BY n),
		       max_files_per_subject = $4,
		       updated_at = now(),
		       updated_by = $5
		 WHERE purpose = $1 AND deleted_at IS NULL
		RETURNING purpose, max_bytes, array_to_json(allowed_mime_types)::text, max_files_per_subject, updated_at, updated_by`,
		next.Purpose, next.MaxBytes, string(mimes), maxFiles, by.Code).Scan)
	if err != nil {
		return domain.UploadPolicy{}, false, fmt.Errorf("upload_policy: update: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO platform_audit_log (actor, actor_ip, action, subject, before, after, reason)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)`,
		by.Code, by.IP, ActionUploadPolicyChanged, next.Purpose,
		string(policyTrail(before)), string(policyTrail(out)), reason); err != nil {
		return domain.UploadPolicy{}, false, fmt.Errorf("upload_policy: trail: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return domain.UploadPolicy{}, false, fmt.Errorf("upload_policy: commit: %w", err)
	}
	return out, true, nil
}
