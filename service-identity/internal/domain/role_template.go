package domain

// The EIGHT TEMPLATE ROLES of docs/ui-ux/14-cau-hinh.md §4.1 — the set written into a commune's
// `vai_tro` / `vai_tro_quyen` by POST /api/v1/roles/defaults when an administrator asks for it, and
// AUTOMATICALLY into a brand-new commune by the owner's CreateCommuneAdminAccount (the transaction that
// creates its `quan-tri-he-thong` role — app/commune_accounts_owner.go).
//
// USER DECISION 2026-09-28 said "there is no create-role route"; OWNER DECISION 15/10/2026 REVERSED
// IT: a "vai trò" is a staff job title carrying a permission set, and the commune creates, renames and
// soft-deletes its own (POST /api/v1/roles, PUT /api/v1/roles/{id},
// POST /api/v1/roles/{id}/deletion — app/role_write.go). "Bí thư" is deliberately NOT a template: a
// commune that has one creates it. What still holds from 28/09: these rows are written ONCE into the
// commune's OWN tables, after which the commune owns them — the argument that keeps sla_gieo.go clear
// of rule 10 forbidden #3. Nothing on an authorisation path ever reads this list — store.Checker
// reads `vai_tro_quyen`, and only that.
//
// =================================================================================================
// EVERY KEY IS WRITTEN OUT. THERE IS NO WILDCARD, NO "task.*", NO "every key in `quyen`".
//
// Chủ tịch UBND holds the 33 keys of migration 0001 (plus the six of 0028, below) BY NAME, not
// "everything the catalogue holds". The difference is the whole point: the catalogue had 36 keys
// when this was written, and the three that are not
// here — `feedback.classify`, `feedback.unmask` (0007) and `admin.user.delete` (0010) — were added
// later for narrower reasons, one of them the right to read citizens' full names and phone numbers
// (rule 3). An expansion over the table would grant them silently, and would grant whatever the next
// migration adds the day it lands, to a role nobody re-read. A named list grants exactly what a
// person decided on the day they decided it; role_template_test.go pins both the list and the
// three absences.
//
// `quan-tri-he-thong` IS NOT A TEMPLATE. That role is created by the first `admin` sign-in
// (store/quan_tri_mac_dinh.go) and holds every key by construction; seeding must never create,
// alter or re-grant it.
//
// WHERE THE LISTS COME FROM: the user's table of 2026-09-28, cross-checked against
// vigov-require default_config.json:186-335 and against the keys the migrations of this service
// actually seed. Nothing was widened or narrowed here — with ONE later, named exception:
//
// USER DECISION 2026-10-09: Chủ tịch UBND and Phó Chủ tịch UBND ALSO hold the six keys of migration
// 0028 (citizen.read, citizen.update, dossier.import, dossier.read, dossier.update, notice.send), and
// no other template does. 0028 grants the same six to those two roles in every commune that already
// has them, so a commune seeded after 0028 and one seeded before end in the same state. Named here,
// key by key, for the reason above: it is a person's decision, not an expansion over the catalogue.
// Consequence for #14 (app/role_template.go): the caller of a template run must now hold these six
// too — see 0028 question 4.
//
// OWNER DECISION 15/10/2026 (bug sheet web-admin-141026 row 22): the four configuration keys of
// migration 0032 (admin.village, admin.message, admin.mail, admin.zalo)
// are split out of admin.lookup / admin.org and PRE-GRANTED to whoever reaches those tabs today. So
// every template holding admin.lookup — Chủ tịch UBND and Chánh Văn phòng — holds all four; no
// template holds admin.org without admin.lookup. 0032 applies the same rule to existing roles, so a
// commune seeded after 0032 and one seeded before end in the same state. The caller of a template
// run must hold these four too (#14); 0032 grants them to every administrator role that predates it.

// RoleTemplate is one template role.
type RoleTemplate struct {
	// Order is `vai_tro.thu_tu` — the column order on the Phân quyền matrix.
	Order int

	// Code is `vai_tro.ma`, the slug `UNIQUE (tenant_id, ma)` matches on. It is DATA (Vietnamese
	// slug, ADR 0011), not an identifier.
	Code string

	// Name is `vai_tro.ten` — what a person reads.
	Name string

	// IsLeader is `vai_tro.la_lanh_dao`: it chooses the default screen and NOTHING ELSE — see
	// VaiTro.LaLanhDao, which states why it must never become an authority axis.
	IsLeader bool

	// Permissions are the flat keys (rule 5, invariant 3b) the role is granted when it is created.
	Permissions []string
}

// RoleTemplates returns the eight templates in `thu_tu` order.
//
// A FUNCTION RETURNING FRESH SLICES, NOT AN EXPORTED VAR — same reason as BoGieoSLA: a package-level
// slice is mutable by any caller, and one commune's run editing the set the next commune gets is a
// defect that shows up only under load.
func RoleTemplates() []RoleTemplate {
	return []RoleTemplate{
		{
			Order: 1, Code: "chu-tich-ubnd", Name: "Chủ tịch UBND", IsLeader: true,
			// The 33 keys of migration 0001:279-312, in that order, then the six of 0028. NOT the
			// catalogue: see the file comment for the keys deliberately absent.
			Permissions: []string{
				"admin.audit", "admin.lookup", "admin.org", "admin.role", "admin.sla", "admin.user",
				"announcement.create",
				"asset.read", "asset.update",
				"budget.confirm", "budget.read", "budget.update",
				"content.read", "content.update",
				"document.create", "document.read", "document.route",
				"feedback.assign", "feedback.create", "feedback.read", "feedback.resolve", "feedback.restricted",
				"petition.create", "petition.read",
				"report.export", "report.read",
				"task.approve", "task.assign", "task.create", "task.delete", "task.extend", "task.read", "task.update",
				// Migration 0028, user decision 2026-10-09 (file comment).
				"citizen.read", "citizen.update",
				"dossier.import", "dossier.read", "dossier.update",
				"notice.send",
				// Migration 0032, owner decision 15/10/2026 (file comment).
				"admin.village", "admin.message", "admin.mail", "admin.zalo",
			},
		},
		{
			Order: 2, Code: "pho-chu-tich-ubnd", Name: "Phó Chủ tịch UBND", IsLeader: true,
			Permissions: []string{
				"task.approve", "task.assign", "task.create", "task.delete", "task.extend", "task.read", "task.update",
				"document.read", "document.route",
				"petition.read",
				"budget.read", "budget.confirm",
				"asset.read",
				"report.read", "report.export",
				"admin.audit",
				"feedback.read", "feedback.create", "feedback.assign", "feedback.resolve",
				"content.read", "content.update",
				"announcement.create",
				// Migration 0028, user decision 2026-10-09 (file comment).
				"citizen.read", "citizen.update",
				"dossier.import", "dossier.read", "dossier.update",
				"notice.send",
			},
		},
		{
			Order: 3, Code: "chanh-van-phong", Name: "Chánh Văn phòng",
			Permissions: []string{
				"task.read", "task.create", "task.assign", "task.update", "task.delete",
				"document.create", "document.read", "document.route",
				"petition.create", "petition.read",
				"report.read", "report.export",
				"admin.lookup",
				// Migration 0032, owner decision 15/10/2026 (file comment): holds admin.lookup, so holds the four.
				"admin.village", "admin.message", "admin.mail", "admin.zalo",
				"content.read", "content.update",
				"announcement.create",
			},
		},
		{
			Order: 4, Code: "truong-bo-phan", Name: "Trưởng bộ phận",
			Permissions: []string{
				"task.read", "task.create", "task.assign", "task.update", "task.delete", "task.approve",
				"document.read",
				"feedback.read", "feedback.assign",
				"report.read",
				"content.read", "content.update",
			},
		},
		{
			Order: 5, Code: "chuyen-vien", Name: "Chuyên viên chuyên môn",
			Permissions: []string{
				"task.read", "task.update",
				"document.read",
				"petition.read",
				"feedback.read", "feedback.resolve",
				"asset.read", "asset.update",
			},
		},
		{
			Order: 6, Code: "ke-toan", Name: "Kế toán",
			Permissions: []string{
				"task.read", "task.update",
				"budget.read", "budget.update",
				"report.read", "report.export",
			},
		},
		{
			Order: 7, Code: "can-bo-mot-cua", Name: "Cán bộ một cửa",
			Permissions: []string{
				"document.create",
				"petition.create", "petition.read",
				"feedback.create", "feedback.read",
				"task.read",
			},
		},
		{
			Order: 8, Code: "truong-thon", Name: "Trưởng thôn, Tổ trưởng dân phố",
			Permissions: []string{
				"task.read", "task.update",
				"feedback.read",
				"asset.read",
			},
		},
	}
}

// RoleTemplatePermissions returns every key ANY template grants, each once, in first-seen order.
//
// IT IS WHAT THE CALLER MUST HOLD IN FULL before a seeding run writes anything (open question #14,
// decided: nobody grants a key they do not hold). Computed from RoleTemplates rather than listed a
// second time, so the check and the grants cannot drift apart.
func RoleTemplatePermissions() []string {
	seen := make(map[string]bool, 40)
	out := make([]string, 0, 40)
	for _, t := range RoleTemplates() {
		for _, k := range t.Permissions {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}
