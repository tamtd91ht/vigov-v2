package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// UploadPolicyStore is the only path to `upload_policy` (migration 0008).
//
// IT HOLDS A RAW *sql.DB, LIKE Directory, and for a reason of the same kind: the table has no
// tenant_id column, so core/store.Scoped — which injects WHERE tenant_id = $1 into every statement —
// cannot read it. It is platform-wide configuration that is the same for every commune today (a
// per-commune override is open, platform.proto ListUploadPolicies), holds no business content and
// no personal data, and this type only READS it. The write path — Vihat's operations zone — is not
// built (ADR 0048); when it is, it writes the row and its platform_audit_log entry in one
// transaction (rule 6, invariant 3).
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
	SELECT purpose, max_bytes, array_to_json(allowed_mime_types)::text, max_files_per_subject, updated_at
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
		var (
			p        domain.UploadPolicy
			mimeJSON string
			maxFiles sql.NullInt32
		)
		if err := rows.Scan(&p.Purpose, &p.MaxBytes, &mimeJSON, &maxFiles, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("upload_policy: scan: %w", err)
		}
		if err := json.Unmarshal([]byte(mimeJSON), &p.AllowedMIMETypes); err != nil {
			return nil, fmt.Errorf("upload_policy: allowed_mime_types of %q: %w", p.Purpose, err)
		}
		p.FileCountLimited = maxFiles.Valid
		p.MaxFilesPerSubject = maxFiles.Int32
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("upload_policy: read: %w", err)
	}
	return out, nil
}
