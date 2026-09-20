package domain

// ThonToDanPho is one residential unit of the commune — a thôn in the countryside, a tổ dân phố in
// town (@entity ResidentialUnit, migration 0005).
//
// ONE TYPE COVERS BOTH, because they are the same thing on two kinds of ground. `Hamlet` would be
// half right and `Village` would be wrong of a ward — the reasoning is owned by
// kb/00-foundation/ubiquitous-language.md:159 (ADR 0024) and is not restated here (rule 9).
//
// NOT A REFERENCE CATALOGUE, although it sits beside two of them: it holds data of its own (the
// household and population counts below) and the commune's business records reference it, which is
// why migration 0005 gives it the soft-delete columns and the hard-delete refusal but none of the
// tier machinery.
type ThonToDanPho struct {
	ID  string // ULID — what other records reference
	Ma  string // slug: "thon-binh-an" — immutable once issued (rule 7, invariant 3)
	Ten string // "Thôn Bình An" — what a person reads

	// LoaiMa is the code from loai_don_vi_dan_cu, "" when the classification has not been entered.
	//
	// "" AND NOT A POINTER: the column is nullable because the specification's own column list shows
	// `—` as a legitimate value (migration 0005:376), and "no classification entered" is the same
	// statement whichever way it is spelled. Unlike the two counts below, there is no second reading
	// to protect — an empty code is not a code that means something.
	LoaiMa string

	// LoaiNhan is the label of that type ("Thôn"), read in the SAME query through a LEFT JOIN —
	// never one lookup per row (skills/load-data-once, shape 1).
	//
	// IT CAN BE EMPTY WHILE LoaiMa IS NOT, and that pair is a state the caller must be able to
	// render: the type row was soft-deleted while residential units still carry its code. The unit
	// itself must survive that — dropping it would turn untidy catalogue data into a unit missing
	// from the commune's own list. Same discipline, same reason, as VaiTroStore's left join, where
	// `deleted_at IS NULL` sits in the JOIN rather than in the WHERE.
	LoaiNhan string

	// SoHo and NhanKhau are POINTERS, and that is the whole point of them.
	//
	// 0 is a statement about a unit ("no households"); NULL is the absence of one ("not entered").
	// A commune importing a spreadsheet without that column must not have zeros asserted on its
	// behalf, because a zero travels onward into a report as a number and nothing downstream can
	// tell it apart from a counted zero. The schema keeps the two apart (migration 0005:385); a
	// plain int here would collapse them at the first read, silently.
	//
	// NEITHER IS PERSONAL DATA (rule 3): they are counts of a territory, naming nobody.
	SoHo     *int
	NhanKhau *int

	// DangDung is false for a unit the commune has taken out of use — a merged hamlet, a unit that
	// no longer exists on the ground. The row stays (rule 7); this flag is how a picker stops
	// offering it while the list screen still shows it.
	DangDung bool
}
