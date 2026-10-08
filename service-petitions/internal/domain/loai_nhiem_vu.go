package domain

// LoaiNhiemVu is one row of the commune's task-type catalogue — `theo-van-ban` / `co-ban`.
//
// The entity is TaskType (migration 0003, `-- @entity: TaskType`), owned by petitions because the
// task lifecycle lives here (ADR 0024). The URL resource is `task-types`.
//
// WHAT IS NOT ON THIS TYPE, AND WHY THE ABSENCE IS THE DESIGN:
//
//	nguon              who added the row — provenance
//	ma_nguon_re_nhanh  whether the SOURCE CODE branches on this row's `ma` (tier 3)
//
// Both answer "what may be DONE to this row", and this service has no write route for these
// catalogues: open question #21 governs the lifecycle side of the task catalogues and is still
// OPEN. Carrying the tier down to a read surface now would put a client one `if` away from
// deciding, on its own, which rows a commune may edit — a second copy of a rule the database
// already enforces with a trigger, and the copy that drifts is the one on the screen.
type LoaiNhiemVu struct {
	// ID is the ULID a task record references.
	ID string

	// Ma is the stable code — "theo-van-ban". A task record holds this code AS A VALUE and
	// nothing rewrites it, which is why the database refuses to let it be edited at all.
	Ma string

	// Nhan is what a person reads — "Theo văn bản". This is the one field a commune may always
	// change, so nothing may key on it.
	Nhan string

	// LaMacDinh marks the row a form pre-selects. AT MOST ONE ROW PER COMMUNE carries it: the
	// database holds that with UNIQUE (tenant_id, moc_mac_dinh) over a generated column, so two
	// defaults cannot exist and a caller never has to decide which of two to believe.
	LaMacDinh bool

	// DangDung is false for a row the commune has taken out of use.
	//
	// SUCH ROWS ARE STILL RETURNED, and that is deliberate: a task recorded last year may hold the
	// code of a type since switched off, and a list that dropped it would leave that task showing a
	// raw code with no label. The configuration screen shows those rows with a "Đã tắt" chip; a
	// picker offering NEW choices filters on this field. Only soft-deleted rows disappear (rule 7,
	// invariant 2), and they disappear in the store, on every path.
	DangDung bool
	// ThuTu is the order the commune arranged its own catalogue in.
	//
	// CARRIED BUT NEVER RE-APPLIED. The store's ORDER BY is what puts the rows in order; a caller
	// sorting on this field again is a second answer to the same question, and the two disagree the
	// moment two rows share a rank. It is here because the configuration screen shows a `Thứ tự`
	// column and lets the commune edit it (docs/ui-ux/14-cau-hinh.md:158) — a screen that cannot
	// read the current value cannot offer to change it.
	ThuTu int

	// Nguon and MaNguonReNhanh together answer "what may be DONE to this row" — the three-tier
	// model of ADR 0024 §6, see danh_muc_ba_tang.go. They are DERIVED INTO a tier, never stored as
	// one: two sources for one fact drift, and the stale one is what a screen would read.
	//
	// THE COMMUNE NEVER SUPPLIES EITHER FIELD. The write route sets Nguon to NguonDonVi and leaves
	// MaNguonReNhanh false, and the store writes both as SQL LITERALS so there is no parameter a
	// request could reach. Provenance decides the tier; a client that could name it could put its
	// own row in tier 2 and step around every guard.
	Nguon          string
	MaNguonReNhanh bool

	// Color is the display colour, `#rrggbb` lower-case, "" when none was chosen (NULL, migration
	// 0033, ADR 0079 row 5). Presentation only — editable on every tier, "Hệ thống" rows included
	// (Q1 #9); nothing branches on it. See catalogue_color.go.
	Color string
}

// TaskTypeByDocument is the `theo-van-ban` code — the one task type the specification attaches a
// directive-document record to (docs/ui-ux/00-tong-quan-he-thong.md:175). Migration 0003:205-208 sows
// it as a TIER-3 code precisely because the source branches on it: a commune may relabel the row, never
// remove or re-code it.
const TaskTypeByDocument = "theo-van-ban"

// RequiresDirective says whether a task of this type is filed against a directing document (the
// prototype's `requires_directive`, vigov-require org/data/default_config.json:517-536: true for
// `theo-van-ban`, false for `co-ban`).
//
// DERIVED FROM THE CODE, NEVER STORED, like Tang. A stored flag would be a second copy of a fact the
// immutable code already carries, and a commune could then flip it on `co-ban` while the source still
// branches on the code — two answers to one question, and the screen would read the wrong one.
func (l LoaiNhiemVu) RequiresDirective() bool { return l.Ma == TaskTypeByDocument }
