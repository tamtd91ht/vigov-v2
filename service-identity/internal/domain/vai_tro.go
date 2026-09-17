package domain

// VaiTro is the role a staff member holds inside ONE commune.
//
// A role belongs to a commune the same way everything else here does: `vai_tro` is partitioned by
// `tenant_id` and every key on it is composite (migration 0001). Two communes may both have a role
// called "Chủ tịch UBND" and they are two different rows with two different grants.
//
// THIS TYPE CARRIES NO PERMISSIONS, AND THAT IS THE DESIGN. Permission keys live in `vai_tro_quyen`
// and are read from the database on every single request by store.Checker — never from a role
// carried on a session or a principal, because a role change must take effect on the next request
// rather than when the session ends (skills/session-and-token). What this type describes is the
// role's IDENTITY: what it is called, and one flag about how the app opens.
type VaiTro struct {
	// ID is the ULID other records reference — `nguoi_dung.vai_tro_id` holds exactly this.
	//
	// IT IS ALWAYS POPULATED, on both read paths, and that is deliberate. The session surface
	// exposes only `Ma` (a slug is what a client keys on), so it would be tempting to leave ID
	// empty there and fill it only in the catalogue. A field that is empty on one path and full on
	// another is a field that can only be wrong in one direction: the day somebody reads it on the
	// wrong path they get "", which compares equal to nothing and matches no row — silently.
	ID string

	Ma  string // slug: "chu-tich-ubnd" — stable, what a client keys a lookup on
	Ten string // "Chủ tịch UBND" — what a person reads

	// LaLanhDao CHOOSES THE DEFAULT SCREEN AND NOTHING ELSE.
	//
	// Its one job: a leader lands on /nhiem-vu/so-tay instead of the ordinary home screen
	// (docs/ui-ux/15-phu-luc-giao-dien-chung.md §1). That is a question about where the app opens.
	//
	// IT IS NOT AN AUTHORITY AXIS, AND IT MUST NEVER BECOME ONE. The day a handler reads
	// `la_lanh_dao` to decide whether an operation is allowed, this project has TWO authorisation
	// systems — and the second one does not go through `(tenant_id, role, permission)`, is not
	// visible on the Phân quyền screen, cannot be granted or withdrawn by an administrator, and
	// leaves no trail when it changes (rule 5, invariant 3 and forbidden #2 and #3). A boolean on a
	// role row that quietly means "may approve" is exactly the "superadmin that skips the check"
	// rule 5 forbids, wearing a different name.
	//
	// What to write instead: a permission key in `quyen`, granted to the role in `vai_tro_quyen`,
	// checked with authz.RequirePermission. `task.approve` is deliberately not `task.extend`, and
	// neither of them is "is a leader".
	LaLanhDao bool
}
