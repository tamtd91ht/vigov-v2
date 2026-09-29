package domain

// CanBoCongKhai is one staff member AS THE PUBLIC MINI APP DIRECTORY PUBLISHES THEM — the row behind
// GET /api/v1/commune-staff (docs/ui-ux/12-danh-ba-can-bo.md:117, owner decision 2026-09-27).
//
// A SIXTH STAFF TYPE, AND THE NARROWEST ONE THAT CARRIES A PHONE NUMBER. The route is PUBLIC: anyone
// who knows a commune's domain can read it. So the type holds exactly the six fields the owner
// allowed out on 2026-09-27, plus the two added 2026-09-29 (order, units headed), and nothing else — no internal id, no staff code, no email, no account or lock flag, no
// consent marks. A field that does not exist cannot be sent by a later edit to a handler, which is a
// stronger guarantee than remembering not to send it.
//
// WHO IS IN IT is decided in the store (store.locDanhBaCongKhai), not here: only rows published to the
// Mini App WITH recorded consent (open question #12), not locked, not soft-deleted.
type CanBoCongKhai struct {
	HoTen  string // personal data (rule 3) — published under #12 consent, NEVER logged
	ChucVu string

	// TenBoPhan is the unit's display name, "" when the person sits in no unit (or the unit was
	// soft-deleted). A NAME, not an id: a citizen has nothing to resolve an id against, and the id is
	// an internal identifier of the commune's org chart.
	TenBoPhan string

	// The two numbers are two kinds of data in law (#16). DienThoaiCoQuan is duty information;
	// DiDongCaNhan is personal data under Decree 13/2023/NĐ-CP and reaches this type ONLY because the
	// person's consent to publication was recorded (#12). Neither is masked on this route: publishing a
	// number so a citizen can ring it is the whole purpose of the consented act.
	DienThoaiCoQuan string
	DiDongCaNhan    string

	// CoZalo describes DiDongCaNhan (migration 0010 §1) and travels with it under the same consent.
	CoZalo bool

	// DisplayOrder is `thu_tu_danh_ba`; nil = no explicit position (0010 §1). Added 2026-09-29 (user
	// decision, SRS M6.1.9). The list already arrives in this order; the value lets a client group.
	DisplayOrder *int

	// ResidentialUnitsHeaded names the live, in-use residential units (thôn / tổ dân phố) this person
	// heads (migration 0018). Names only — never a unit id. Empty for most people.
	ResidentialUnitsHeaded []string
}
