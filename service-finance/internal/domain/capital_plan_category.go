package domain

// CapitalPlanCategory is one row of the commune's capital plan category catalogue —
// `@entity: CapitalPlanCategory`, table `hang_muc_ke_hoach_von` (ADR 0024).
//
// The Go type takes the name of the `@entity` mark (ADR 0061; the table keeps its Vietnamese name
// until layer B renames it): the CONTRACT surface and the struct share one name, not a second
// name for the same struct. Two names for one thing in one service is the drift rule 9 describes.
//
// WHAT A "HẠNG MỤC" IS AND IS NOT: it classifies the lines of a capital plan. It is not a line of
// money and carries no amount — see the reason the entity is `…Category` and not `…Item`
// (kb/00-foundation/ubiquitous-language.md:157). Nothing that holds an amount belongs on this
// struct, and adding one here is how the catalogue quietly becomes the plan.
//
// THE COLUMNS THAT ARE ABSENT ARE THE DESIGN, not an oversight — each is left out for a reason:
//
//	tenant_id                   never a field. It rides in context.Context and is bound by the
//	                            scoped repository (rule 1, invariant 4). A struct field would be a
//	                            value somebody can pass, and eventually pass wrong.
//	thu_tu                      the sort key, not data. The store returns the rows already in the
//	                            commune's own order; a field would invite a caller to re-sort, which
//	                            is a client overruling the commune on its own catalogue.
//	nguon, ma_nguon_re_nhanh    they answer "what may be DONE to this row" — the three tiers of
//	                            ADR 0024 §6. Only a configuration surface asks that, and this
//	                            service has no write route: open question #21 (may a commune edit
//	                            the CODE LIST, or only labels and order) is unanswered. Publishing
//	                            the tier now would describe buttons nobody has decided to allow.
//	deleted_at and friends      a soft-deleted row never leaves the store (rule 7, invariant 2), so
//	                            no reader ever needs to ask.
type CapitalPlanCategory struct {
	ID    string // ULID — what a capital plan line will reference
	Code  string // "xay-dung-moi" — Vietnamese without diacritics (ADR 0011); the value stored on a plan line
	Label string // "Xây dựng mới" — the label a person reads

	// IsDefault marks the single row a form pre-selects. At most one live row per commune carries
	// it, enforced by UNIQUE (tenant_id, moc_mac_dinh) in the schema rather than by application
	// code — two defaults would make the pre-selected item depend on read order, which is to say
	// random, with nothing on the screen showing that it is.
	IsDefault bool

	// IsActive is false for a row the commune has taken out of use.
	//
	// SUCH A ROW IS STILL RETURNED, and that is deliberate: the catalogue screen lists it with a
	// "Đã tắt" chip, and a plan line recorded in an earlier budget year still holds its `ma` as a
	// value, so a reader that dropped it would render an existing figure with no category name at
	// all. What a caller must NOT do is offer it in a picker for new work — which is exactly why
	// the flag travels instead of the row being filtered away here.
	IsActive bool
	// SortOrder is the order the commune arranged its own catalogue in.
	//
	// CARRIED BUT NEVER RE-APPLIED. The store's ORDER BY is what puts the rows in order; a caller
	// sorting on this field again is a second answer to the same question, and the two disagree the
	// moment two rows share a rank. It is here because the configuration screen shows a `Thứ tự`
	// column and lets the commune edit it (docs/ui-ux/14-cau-hinh.md:158) — a screen that cannot
	// read the current value cannot offer to change it.
	SortOrder int

	// Source and BranchedInSource together answer "what may be DONE to this row" — the three-tier
	// model of ADR 0024 §6, see three_tier_catalogue.go. They are DERIVED INTO a tier, never stored as
	// one: two sources for one fact drift, and the stale one is what a screen would read.
	//
	// THE COMMUNE NEVER SUPPLIES EITHER FIELD. The write route sets Source to SourceCommune and leaves
	// BranchedInSource false, and the store writes both as SQL LITERALS so there is no parameter a
	// request could reach. Provenance decides the tier; a client that could name it could put its
	// own row in tier 2 and step around every guard.
	Source           string
	BranchedInSource bool
}
