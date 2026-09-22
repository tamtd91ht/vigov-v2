package domain

// LoaiVanBan is one row of the commune's document-type catalogue — `cong-van`, `quyet-dinh`,
// `thong-bao` and the rest of the list the commune registers documents under.
//
// THE ENTITY IS `DocumentType` and the URL resource is `document-types`. The entity name is the
// one written on the `-- @entity` mark above the table (migrations/0003_danh_muc_loai_van_ban.sql),
// and the mapping table that owns both rows is kb/00-foundation/ubiquitous-language.md (ADR 0011).
// STATED GAP, carried rather than silently closed: that table's URL-resource cell for this concept
// still reads *(chưa chốt)*. The path was settled by the user; filling the cell belongs to whoever
// owns the table (rule 9), not to this file.
//
// ONE TYPE FOR BOTH DIRECTIONS. There is no IncomingDocumentType: đến/đi is a DIRECTION of a
// document, not a kind of one, and splitting the catalogue in two would give a commune two lists to
// keep in step for one concept.
type LoaiVanBan struct {
	ID   string // ULID, internal — what a document record would reference
	Ma   string // "cong-van" — the code a document record stores AS A VALUE, never renumbered
	Nhan string // "Công văn" — the wording the commune may change at any time

	// DangDung is `dang_dung`: false means the commune has taken this type out of use.
	//
	// ROWS OUT OF USE ARE STILL RETURNED, and that is the catalogue's shape rather than an
	// oversight: the configuration screen lists them with a "Đã tắt" chip
	// (migrations/0003_danh_muc_loai_van_ban.sql, the comment on loai_van_ban_danh_sach), and a
	// document registered years ago under a type since retired still has to render its own label.
	// Only soft-deleted rows drop out — everywhere, always (rule 7, invariant 2).
	//
	// A picker filling a form is therefore expected to offer only the rows with DangDung true. The
	// alternative — a second, filtered route — would give two answers to "what types does this
	// commune have", and the stale one is the one that reaches a screen.
	DangDung bool

	// LaMacDinh is the single type a form pre-selects. The schema admits at most one live default
	// per commune through the generated `moc_mac_dinh` column, so a caller may rely on there being
	// no more than one — and on there being NONE at all, which is the state of every commune until
	// its catalogue is sown.
	LaMacDinh bool

	// ThuTu is the order the commune arranged its own catalogue in.
	//
	// IT IS CARRIED BUT NEVER RE-APPLIED. The store's ORDER BY is what puts the rows in order; a
	// caller sorting on this field again is a second answer to the same question, and the two
	// disagree the moment two rows share a rank. It is here because the configuration screen shows
	// a `Thứ tự` column and lets the commune edit it (docs/ui-ux/14-cau-hinh.md:158) — a screen
	// that cannot read the current value cannot offer to change it.
	ThuTu int

	// Nguon and MaNguonReNhanh together answer "what may be DONE to this row" — the three-tier
	// model of ADR 0024 §6, see danh_muc_ba_tang.go.
	//
	// THEY USED TO BE ABSENT ON PURPOSE, and the comment saying so pointed at open question #21.
	// That reading is out of date: #21 asks whether a commune may edit the list of TASK STATUS
	// codes, whose lifecycle a fixed state machine walks; this catalogue's own answer is in the
	// schema and is enforced by a trigger, not by a promise (migrations/0003_danh_muc_loai_van_ban
	// .sql:72-77 and :157-185). A commune may add its own codes AND relabel and reorder every row;
	// what it may not do is delete or disable what the software ships.
	//
	// THE COMMUNE NEVER SUPPLIES EITHER FIELD. A write route sets Nguon to NguonDonVi as a literal
	// and leaves MaNguonReNhanh false; see the comment on the constants.
	Nguon          string
	MaNguonReNhanh bool
}

// WHAT THIS TYPE DELIBERATELY DOES NOT CARRY, so the absence reads as a decision:
//
//	deleted_at         every read path excludes soft-deleted rows (rule 7, invariant 2), so the
//	                   field would always be nil. The soft delete itself is a write that names the
//	                   row by id; it never needs to read the column back.
//	moc_mac_dinh       a GENERATED column. It exists so the database can refuse a second default;
//	                   it carries no fact LaMacDinh does not already carry.
