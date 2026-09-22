package domain

// MucUuTienNhiemVu is one level of the commune's task-priority scale — `khan` / `cao` / `thuong`.
//
// The entity is TaskPriority (migration 0003, `-- @entity: TaskPriority`), and the name carries no
// `…Type` suffix on purpose: THIS IS A SCALE, NOT A FLAT CLASSIFICATION. "Urgent" means nothing on
// its own; it means something only relative to the levels around it.
//
// WHERE THE ORDER LIVES, AND WHY IT IS NOT A FIELD HERE. The rank is the column `thu_tu`, and this
// type does not carry it: the store sorts by it and hands back a slice, so THE POSITION IN THE
// SLICE IS THE RANK. Carrying the number as well would be two representations of one fact (rule 9)
// — and the day they disagree, they disagree because somebody re-sorted the slice, which is exactly
// the defect the single representation makes impossible to hide. A commune inserting a level
// between two existing ones renumbers `thu_tu`; no `ma` changes, and nothing outside the store ever
// sees the numbers.
//
// The same two fields are absent as on LoaiNhiemVu — `nguon` and `ma_nguon_re_nhanh` — for the same
// reason, which is written out there.
type MucUuTienNhiemVu struct {
	// ID is the ULID a task record references.
	ID string

	// Ma is the stable code — "khan". Held as a value by task records and immutable in the
	// database.
	Ma string

	// Nhan is what a person reads — "Khẩn".
	Nhan string

	// LaMacDinh marks the level a new task starts at. At most one per commune, held by the
	// database — see LoaiNhiemVu.LaMacDinh.
	LaMacDinh bool

	// DangDung is false for a level taken out of use; such rows are still returned, so an older
	// task holding the code still has a label. See LoaiNhiemVu.DangDung.
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
