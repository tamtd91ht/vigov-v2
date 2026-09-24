package domain

// KhoiNhiemVu is one entry of the commune's task-bloc catalogue — `khoi-uy-ban` / `khoi-dang` /
// `khac` (@entity TaskBloc, migration 0005).
//
// THIS LIVES IN identity ALTHOUGH THE ENTITY NAME CARRIES "Task", AND THAT IS NOT A SLIP. The name
// follows the CONCEPT (a bloc is an attribute of a task), ownership follows the RATE OF CHANGE (the
// list moves with the org chart — khối Uỷ ban / khối Đảng — not with tasks). Moving it to
// `petitions` "so the name matches" rebuilds the two-node dependency ADR 0024:130 forbids by name.
// The whole argument is kb/00-foundation/ubiquitous-language.md:161 and migration 0005:300; this
// comment points at them rather than restating them (rule 9).
//
// THE CONSUMER IS THE TASK RECORD IN `petitions`, WHICH HOLDS THE CODE AS A VALUE — no foreign key
// across the service boundary, no JOIN into this schema (rule 2; ADR 0024, §Cái giá của dòng Khối
// nhiệm vụ). That is exactly why `Ma` matters more here than `ID`: the code is what the other
// service's rows carry, and it is why migration 0005's trigger refuses to let a commune edit it.
//
// A FULL CATALOGUE (user decision 2026-09-24), same shape as LoaiDonViDanCu.
//
// NOT THE DIRECTORY'S "khối đơn vị". The user decided on 2026-09-24 that a task bloc is a concept
// separate from any grouping of `bo_phan`; nothing here links to the org chart, and nothing should
// until that decision is revisited (see migration 0005:313 for when the name would have to move).
//
// SAME FIELDS AS LoaiDonViDanCu, DELIBERATELY A SECOND TYPE — see the note there.
type KhoiNhiemVu struct {
	ID string // ULID

	Ma   string // slug: "khoi-uy-ban" — the value a task record in `petitions` holds
	Nhan string // "Khối Uỷ ban" — what a person reads

	// LaMacDinh is the row a form pre-selects, at most one per commune. Reason on
	// LoaiDonViDanCu.LaMacDinh.
	LaMacDinh bool

	// DangDung is false for a row taken out of use. Out-of-use rows are RETURNED, not filtered —
	// reason on LoaiDonViDanCu.DangDung.
	DangDung bool

	// ThuTu, Nguon, MaNguonReNhanh — reasons on LoaiDonViDanCu.
	ThuTu          int
	Nguon          string
	MaNguonReNhanh bool
}
