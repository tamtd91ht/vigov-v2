package domain

// LoaiDonViDanCu is one entry of the commune's residential-unit-type catalogue — `thon` /
// `to-dan-pho` (@entity ResidentialUnitType, migration 0005).
//
// THE URL RESOURCE IS `residential-unit-types`, and the entity name is the one already fixed in
// kb/00-foundation/ubiquitous-language.md (ADR 0024). That file owns the mapping; this comment
// points at it rather than restating the table (rule 9).
//
// ⚠ WHETHER THIS SHOULD BE A CATALOGUE AT ALL IS AN OPEN QUESTION, AND IT IS NOT SETTLED BY THIS
// TYPE EXISTING. If `thon` and `to-dan-pho` are fixed by law, the right model is an enum column on
// thon_to_dan_pho, because a catalogue is by definition something a commune can switch off —
// switching off `thon` is the failure shape open question #21 describes. The migration's tier-3
// flag (`ma_nguon_re_nhanh`) closes the disable path; it does not answer the modelling question.
// Nothing here resolves it, and nothing here should be "tidied" as if it had been: the read route
// is the same read route either way, and a read route is what was asked for.
//
// THIS TYPE AND KhoiNhiemVu HAVE THE SAME FIELDS AND ARE DELIBERATELY TWO TYPES. They are two
// concepts with two entity names and two tables; one shared "catalogue item" type would let a
// handler hand a task bloc where a residential-unit type belongs and the compiler would agree. The
// duplication is five field names; the thing it buys is that the mistake cannot be written.
type LoaiDonViDanCu struct {
	// ID is the ULID. Returned because a client that will later reference one row must reference
	// the same value the row is identified by.
	ID string

	Ma   string // slug: "thon" — the VALUE thon_to_dan_pho.loai holds, and what a client keys on
	Nhan string // "Thôn" — what a person reads

	// LaMacDinh is the row a form pre-selects, at most one per commune.
	//
	// The schema goes to some length to guarantee "at most one" — the generated `moc_mac_dinh`
	// column plus UNIQUE (tenant_id, moc_mac_dinh), argued at migration 0005:244. That machinery
	// exists so a form can pre-select without guessing; if no read path ever carried the flag out,
	// the guarantee would be unreachable and the form would pick the first row instead, which is to
	// say whichever row sorted first.
	LaMacDinh bool

	// DangDung is false for a row the commune has taken out of use.
	//
	// OUT-OF-USE ROWS ARE RETURNED, NOT FILTERED, and this flag is what makes that safe. The
	// catalogue screen has to show them (they carry a "Đã tắt" chip — migration 0005:283) while a
	// picker must not offer them. One route serves both because the flag travels; a route that
	// filtered server-side would leave the catalogue screen unable to show what it manages, and a
	// route that dropped the flag would put a disabled option in a form with nothing on the screen
	// to show it.
	//
	// Soft-deleted rows are a different case and never come back at all (rule 7, invariant 2).
	DangDung bool
}
