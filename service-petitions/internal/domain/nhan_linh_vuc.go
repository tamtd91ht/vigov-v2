package domain

// NhanLinhVuc is one commune's own wording for a petition field code — tier 2 of ADR 0026.
//
// THE ENTITY IS AN OVERRIDE, NOT A CATALOGUE ROW, and the distinction decides how a reader must
// treat an absent row. The catalogue itself — the CLOSED set of twelve codes — belongs to
// service `platform`, and a commune may not add to it or remove from it; the single operation a
// commune has here is RE-WORDING. So:
//
//	a row present   the commune calls this code by its own name
//	a row absent    the code is still perfectly valid; it shows the platform's default label
//
// A reader that treated absence as "unknown code" would blank out eleven of twelve fields for
// every commune that renamed one.
//
// WHY THE CODES ARE CLOSED AT ALL, in one line, because it is the thing people push back on:
// the province adds petition counts BY FIELD across 200+ communes (ADR 0026), so the code is a
// SHARED UNIT OF MEASURE. Two communes spelling rubbish collection differently make the sum
// meaningless, and there is no cheap way to repair that afterwards — the numbers sit on closed
// archival records that rule 7 does not permit rewriting.
type NhanLinhVuc struct {
	ID string

	// Ma is the tier-1 code, held AS A VALUE. There is no foreign key and no JOIN to the table
	// that owns it: it is in another service's database (rule 2, forbidden #2).
	Ma string

	// Nhan is what a person reads. It is the ONE field a commune may change, so nothing may key
	// on it — and it is why the contract answers `label` rather than `name`
	// (kb/00-foundation/ubiquitous-language.md owns that rule).
	//
	// "" = NO LABEL OVERRIDE (the column is NULL, migration 0022): show the tier-1 default.
	Nhan string

	// SortOrder is the commune's display position (`thu_tu`). 0 = inherit the tier-1 sort_order;
	// 1.. is compared in the same numeric space as tier-1 order (migration 0022).
	SortOrder int

	// Enabled false hides the code from the citizen's NEW-SUBMISSION form only. It never filters a
	// read path: a petition already carrying the code keeps its label and stays in every list and
	// report (ADR 0026 §Bổ sung cuối ngày).
	Enabled bool
}
