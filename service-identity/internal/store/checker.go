package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/store"
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
//
// `nd.co_tai_khoan` SITS NEXT TO `nd.dang_hoat_dong` AND IS NOT THE SAME CHECK. A person who has
// no sign-in account holds no permission at all — the public staff directory shares this table
// with the accounts (migration 0003), and a directory row may well carry a vai_tro_id because
// the org chart needs one. Reading grants off that row would hand the subsystem to somebody who
// was never given an account. Locked-out (`dang_hoat_dong = false`) is the other, separate case:
// an account that exists and is currently shut. Both must hold, for the same fail-closed reason
// stated at the top of this file — an error or an omission here widens access invisibly.
const truyVanQuyenGoc = `
SELECT vq.quyen_ma
FROM nguoi_dung nd
JOIN vai_tro       vt ON vt.tenant_id = nd.tenant_id AND vt.id         = nd.vai_tro_id
JOIN vai_tro_quyen vq ON vq.tenant_id = nd.tenant_id AND vq.vai_tro_id = vt.id
WHERE nd.tenant_id = $1
  AND nd.id = $2
  AND nd.deleted_at IS NULL
  AND nd.co_tai_khoan
  AND nd.dang_hoat_dong
  AND vt.deleted_at IS NULL`

// truyVanQuyen answers "does this person hold any of these keys". truyVanDanhSachQuyen answers
// "which keys does this person hold".
//
// THEY SHARE truyVanQuyenGoc INSTEAD OF EACH SPELLING THE PREDICATES OUT, and that is not tidiness:
// two copies of these six conditions would drift, and drift in either direction is a defect with
// no error attached. Looser here than in the check → the web draws menu entries the server then
// answers 403 on, and staff report the system as broken. Stricter → the web hides work the person
// is entitled to do, and nobody finds out, because a missing button asks no questions.
const (
	truyVanQuyen = truyVanQuyenGoc + `
  AND vq.quyen_ma = ANY($3)`

	// Ordered so one caller's response is stable between requests — an unordered list makes every
	// client-side comparison and every test compare sets by accident.
	truyVanDanhSachQuyen = truyVanQuyenGoc + `
ORDER BY vq.quyen_ma`
)

// Allows reports whether the principal holds perm in the commune carried by ctx.
//
// The commune is NOT a parameter on purpose: it rides in the context, so no caller can pass
// one commune's id while reading another's roles.
func (c *Checker) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	ra, err := c.AllowsNhieu(ctx, p, []authz.Perm{perm})
	if err != nil {
		// Logged with the permission, the staff id and THE COMMUNE — never with personal data
		// (rule 3). The commune is not decoration here: this line says a staff member was
		// REFUSED because a permission could not be read. One process serves 200+ communes into
		// one log stream, and "somebody somewhere was refused" is not something an operator can
		// act on. p.TenantID is the commune the principal was issued for, which is the commune
		// the query ran in (authz.xacNhanXa has already compared it with the one from Host).
		c.log.Error("authz: không kiểm được quyền, từ chối",
			"quyen", string(perm), "can_bo", p.ID, "xa", string(p.TenantID), "err", err)
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

// QuyenCua lists every permission key the principal holds, in the commune carried by ctx.
//
// WHY A LIST EXISTS AT ALL, when authz.Checker deliberately answers only yes/no: the admin web has
// to hide the menu entries and buttons a person cannot use (docs/ui-ux/15-phu-luc-giao-dien-chung.md
// §1 and §2), and it cannot ask 33 separate questions on every page load (skills/load-data-once).
//
// THIS IS A DESCRIPTION, NEVER A DECISION. Nothing in the system decides access from this list:
// every route declares its own permission and goes through Allows, which runs its own query
// against the same grants. Handing a caller their own list changes what is DRAWN, never what is
// PERMITTED (rule 5, forbidden #1).
//
// IT RETURNS AN ERROR INSTEAD OF FAILING CLOSED TO AN EMPTY LIST, unlike Allows which answers
// false on any failure. The asymmetry is deliberate: for Allows, false is the safe answer and is
// never mistaken for a fact. Here, empty is a REAL answer — an account whose roles were withdrawn
// holds nothing — so an empty list on failure would be indistinguishable from that person, and the
// caller could not tell "you may do nothing" from "we could not find out". The caller decides;
// this reports.
func (c *Checker) QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error) {
	// Citizens have no roles: they are isolated by identity (rule 4), never by RBAC. The same
	// guard as AllowsNhieu, and for the same reason — a citizen reaching an RBAC path is a routing
	// mistake, and the empty answer is the safe reading of it.
	if p.Kind != "staff" || p.ID == "" {
		return nil, nil
	}

	rows, err := c.db.For(ctx).QueryJoin(ctx, truyVanDanhSachQuyen, p.ID)
	if err != nil {
		return nil, fmt.Errorf("authz: đọc danh sách quyền: %w", err)
	}
	defer rows.Close()

	var ra []authz.Perm
	for rows.Next() {
		var ma string
		if err := rows.Scan(&ma); err != nil {
			return nil, fmt.Errorf("authz: đọc dòng quyền: %w", err)
		}
		ra = append(ra, authz.Perm(ma))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authz: duyệt danh sách quyền: %w", err)
	}
	return ra, nil
}

var _ authz.Checker = (*Checker)(nil)
