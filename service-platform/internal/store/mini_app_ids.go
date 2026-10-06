package store

import (
	"context"
	"errors"
	"fmt"
)

// ErrOwnMiniAppAmbiguous — the commune holds more than one live (running, not-deleted) 'rieng' row.
// ADR 0070 #1 forbids it and every write path refuses to create it, but no database constraint does
// (migration 0006 left it open). Refused rather than picked: a deploy pushed to the wrong one of two
// App IDs ships a commune's build into an app nobody chose.
var ErrOwnMiniAppAmbiguous = errors.New("directory: xã có hơn một Mini App riêng đang chạy")

// LiveOwnMiniAppID answers "which App ID is the OWN Mini App of the commune that holds this host" —
// the public lookup the Mini App deploy script asks (owner, option A, 06/10/2026).
//
// THE COMMUNE IS RESOLVED EXACTLY AS THE EDGE RESOLVES A Host (ByHostErr): ANY of the commune's
// domains, not only the primary; a reserved platform host or OPERATOR_HOST answers like an unknown
// one; an inactive (merged, dissolved) commune is refused. Every one of those, and a commune with no
// live own app, is ErrKhongCoMiniApp — ONE sentinel, so the caller cannot tell a prober which. A
// database failure is a wrapped error, never that sentinel: an outage is not "no such app".
//
// ON Directory because it is the same registry lookup Directory.MiniApp is, run before any commune is
// known; it returns one App ID or none, never a list (LIMIT 2 exists only to see ambiguity).
func (d *Directory) LiveOwnMiniAppID(ctx context.Context, host string) (string, error) {
	t, err := d.ByHostErr(ctx, host)
	switch {
	case errors.Is(err, ErrKhongCoXa), errors.Is(err, ErrXaNgungHoatDong):
		return "", ErrKhongCoMiniApp
	case err != nil:
		return "", err
	}
	// @cross-tenant: registry lookup keyed by the commune the Host just resolved to — runs before any
	// commune is in context, reads only mini_app (no business data), answers one App ID (ADR 0003, 0070).
	rows, err := d.db.QueryContext(ctx, `
		SELECT app_id FROM mini_app
		 WHERE tenant_id = $1 AND che_do = 'rieng' AND dang_hoat_dong AND deleted_at IS NULL
		 ORDER BY app_id
		 LIMIT 2`, t.ID.String())
	if err != nil {
		return "", fmt.Errorf("directory: own mini app: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("directory: own mini app: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("directory: own mini app: %w", err)
	}
	switch len(ids) {
	case 0:
		return "", ErrKhongCoMiniApp
	case 1:
		return ids[0], nil
	}
	return "", ErrOwnMiniAppAmbiguous
}
