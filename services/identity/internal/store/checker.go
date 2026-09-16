package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/pkg/authz"
	"github.com/vihat/vigov/pkg/store"
)

// Checker answers whether a staff member holds a permission, within one commune.
//
// It implements authz.Checker, which guards every service's routes.
//
// FAIL CLOSED: any failure — a database error, an unreadable row, a principal with no role —
// answers false. Rule 5, invariant 2: a missing declaration means deny, not allow. An error
// here must never widen access, because that failure is invisible: the request succeeds,
// nothing turns red, and a role reaches a subsystem it has nothing to do with.
type Checker struct {
	db  *store.DB
	log *slog.Logger
}

func NewChecker(db *store.DB, log *slog.Logger) *Checker {
	return &Checker{db: db, log: log}
}

// Every join is constrained to the SAME commune, not just to the matching id. Joining on id
// alone would match another commune's role wherever ids collide — the leak rule 1 exists to
// prevent, and one that no single-commune test would ever show.
const truyVanQuyen = `
SELECT vq.quyen_ma
FROM nguoi_dung nd
JOIN vai_tro       vt ON vt.tenant_id = nd.tenant_id AND vt.id         = nd.vai_tro_id
JOIN vai_tro_quyen vq ON vq.tenant_id = nd.tenant_id AND vq.vai_tro_id = vt.id
WHERE nd.tenant_id = $1
  AND nd.id = $2
  AND nd.deleted_at IS NULL
  AND nd.dang_hoat_dong
  AND vt.deleted_at IS NULL
  AND vq.quyen_ma = ANY($3)`

// Allows reports whether the principal holds perm in the commune carried by ctx.
//
// The commune is NOT a parameter on purpose: it rides in the context, so no caller can pass
// one commune's id while reading another's roles.
func (c *Checker) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	ra, err := c.AllowsNhieu(ctx, p, []authz.Perm{perm})
	if err != nil {
		// Logged with the permission and the staff code — never with personal data (rule 3).
		c.log.Error("authz: không kiểm được quyền, từ chối",
			"quyen", string(perm), "can_bo", p.ID, "err", err)
		return false
	}
	return ra[perm]
}

// AllowsNhieu answers several permissions in ONE round trip.
//
// WHY THIS IS THE PRIMARY METHOD and Allows delegates to it: a screen asks "which of these may
// I do" to decide what to render — the petition drawer alone needs read, assign, resolve and
// restricted. One query per permission is four round trips for one screen, and it grows with
// every button added (skills/load-data-once).
//
// The returned map always contains every requested key, so a caller reading a permission that
// was denied gets false rather than a missing entry that might be mistaken for "unknown".
func (c *Checker) AllowsNhieu(ctx context.Context, p authz.Principal, perms []authz.Perm) (map[authz.Perm]bool, error) {
	ra := make(map[authz.Perm]bool, len(perms))
	for _, perm := range perms {
		ra[perm] = false
	}

	// Citizens have no roles: they are isolated by identity (rule 4), never by RBAC. A citizen
	// reaching an RBAC-guarded route is a routing mistake, and answering false is the safe
	// reading of it.
	if p.Kind != "staff" || p.ID == "" || len(perms) == 0 {
		return ra, nil
	}

	ma := make([]string, 0, len(perms))
	for _, perm := range perms {
		if perm != "" {
			ma = append(ma, string(perm))
		}
	}
	if len(ma) == 0 {
		return ra, nil
	}

	// pgx binds a []string to a Postgres text[] directly, so ANY($3) works without a driver
	// helper — and without pulling in a second driver just to pass an array.
	rows, err := c.db.For(ctx).QueryJoin(ctx, truyVanQuyen, p.ID, ma)
	if err != nil {
		return ra, fmt.Errorf("authz: kiểm quyền: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var got string
		if err := rows.Scan(&got); err != nil {
			return ra, fmt.Errorf("authz: đọc quyền: %w", err)
		}
		ra[authz.Perm(got)] = true
	}
	if err := rows.Err(); err != nil {
		return ra, fmt.Errorf("authz: duyệt kết quả: %w", err)
	}
	return ra, nil
}

var _ authz.Checker = (*Checker)(nil)
