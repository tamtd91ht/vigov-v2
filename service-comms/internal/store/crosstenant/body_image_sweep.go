package crosstenant

// The two unscoped statements of the abandoned body-image sweep (owner, 03/10/2026, ADR 0067 K11;
// internal/app/body_image_sweep.go): which communes hold a READY body image of an article id that was
// never saved and completed before the cut-off — tenant IDENTIFIERS only, no business row — and the
// scheduler's advisory lock, so ONE replica sweeps. The sweep itself runs per commune through core/store.

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// BodyImageSweep is the sweep's unscoped handle. It holds the pool and hands none of it out.
type BodyImageSweep struct{ db *sql.DB }

// NewBodyImageSweep wraps the process pool.
func NewBodyImageSweep(db *sql.DB) *BodyImageSweep { return &BodyImageSweep{db: db} }

// abandonedBodyImageCommunesQuery mirrors the per-commune candidate read of internal/store
// (abandonedBodyImagesStmt) clause for clause, so a commune listed here has work and a commune with work
// is listed. NOT EXISTS reads the article table WITH soft-deleted rows: an id any article ever carried is
// a saved article (rule 7, invariant 3), and its files belong to that record.
//
// $1 purpose · $2 subject type · $3 cut-off (completed strictly before).
const abandonedBodyImageCommunesQuery = `SELECT DISTINCT f.tenant_id FROM stored_file f
	WHERE f.purpose = $1 AND f.subject_type = $2 AND f.status = 'ready'
	  AND f.deleted_at IS NULL AND f.public_object_key IS NULL AND f.updated_at < $3
	  AND NOT EXISTS (SELECT 1 FROM noi_dung_mini_app n WHERE n.tenant_id = f.tenant_id AND n.id = f.subject_id)
	ORDER BY f.tenant_id`

// CommunesWithAbandonedBodyImages lists the communes the sweep has work in. Identifiers that are not
// ULID-shaped are dropped: they cannot name a commune.
func (s *BodyImageSweep) CommunesWithAbandonedBodyImages(ctx context.Context, purpose, subjectType string,
	completedBefore time.Time) ([]tenant.ID, error) {

	// @cross-tenant: ADR 0067 K11 / ADR 0058 §2b — the body-image sweep lists the communes holding an
	// abandoned draft's image (tenant identifiers only, no business row), then sweeps each one IN that
	// commune's context through core/store.
	rows, err := s.db.QueryContext(ctx, abandonedBodyImageCommunesQuery, purpose, subjectType, completedBefore)
	if err != nil {
		return nil, fmt.Errorf("crosstenant: list communes with abandoned body images: %w", err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: list communes with abandoned body images: %w", err)
		}
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: list communes with abandoned body images: %w", err)
	}
	return out, nil
}

// BodyImageSweepLockKey is the sweep scheduler's lock — its own prefix, so it never contends with the
// portal sync's locks.
func BodyImageSweepLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:body-image-sweep:scheduler"))
	return int64(h.Sum64())
}

// TryLockScheduler takes the sweep's lock for one tick, without waiting. ok=false: another replica is
// sweeping. release is never nil and runs on a context that cannot be cancelled.
func (s *BodyImageSweep) TryLockScheduler(ctx context.Context) (release func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, s.db, BodyImageSweepLockKey())
}
