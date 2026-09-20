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
}
