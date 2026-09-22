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
}
