package domain

import (
	"errors"
	"strings"
)

// LoaiTaiNguyenBanDo is one group the economic map is organised by — the `MapAssetType` entity
// declared on migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql.
//
// THE ENTITY IS `MapAssetType` AND THE URL RESOURCE IS `map-asset-types`, and `asset` is not a
// fresh choice here: the permissions `asset.read` / `asset.update` are already settled
// (docs/ui-ux/10-ban-do-kinh-te-so.md:255), so `resource` or `poi` would give one concept two
// English words on two surfaces. The mapping table is owned by
// kb/00-foundation/ubiquitous-language.md (ADR 0011) — this comment points at it rather than
// restating it (rule 9). STATED GAP: row :156 of that table still reads *(chưa chốt)* in the URL
// column; filling it in belongs to whoever owns that file, not to this one.
//
// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, and the two missing ones are `nguon` and
// `ma_nguon_re_nhanh`. Those two answer "what may a commune DO to this row" — the three tiers
// argued in the migration header — and nothing can be done to a row through a read route. The
// screen that needs them is the catalogue editor, which does not exist and cannot be built yet:
// open question #21 (may a commune edit the CODE LIST, or only labels and order) is unanswered.
// Carrying them now would ship the shape of a decision nobody has taken.
type LoaiTaiNguyenBanDo struct {
	ID   string // ULID, internal — what a map asset record references
	Ma   string // slug: "doanh-nghiep". The value asset records store, never renumbered (rule 7)
	Nhan string // "Doanh nghiệp" — the wording a commune may change without touching `Ma`

	// ThuTu is the order the commune arranged its own groups in. It is carried rather than
	// applied: the store sorts by it, and no layer above re-sorts.
	ThuTu int

	// LaMacDinh marks the ONE group the map's selector opens on. At most one live row per commune
	// carries it, enforced by the generated `moc_mac_dinh` column and UNIQUE (tenant_id,
	// moc_mac_dinh) — not by anything in Go.
	LaMacDinh bool

	// DangDung is false for a group taken out of use. Such a row is STILL RETURNED: the catalogue
	// screen shows it with a "Đã tắt" chip, and an asset already filed under it still has to render
	// its group's name. Only soft-deleted rows drop out, and they drop out in the store (rule 7,
	// invariant 2).
	DangDung bool

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

	// Color is the group's display colour, `#rrggbb` LOWER-CASE, or "" for none chosen (the column
	// is NULL — migration 0020). Presentation only: editable on every tier, system rows included
	// (ADR 0079 Q1 #9), and never a reason the tier guard refuses anything.
	Color string
}

// ErrCatalogueColorShape — a colour that is not `#` and six hex digits. One shape, because the map
// renderer and the chip both parse it and the value reaches a style attribute (0020; rule 13 #3).
var ErrCatalogueColorShape = errors.New("danh_muc: `color` phải có dạng #RRGGBB (sáu chữ số hệ mười sáu)")

// NormalizeCatalogueColor returns the colour to store: "" for none, otherwise `#rrggbb` lower-cased —
// '#1A2B3C' and '#1a2b3c' are one colour, and one stored spelling keeps a before/after audit delta
// from recording a change that changed nothing. Mirrors 0020's CHECK `^#[0-9A-Fa-f]{6}$`.
func NormalizeCatalogueColor(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) != 7 || s[0] != '#' {
		return "", ErrCatalogueColorShape
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return "", ErrCatalogueColorShape
		}
	}
	return strings.ToLower(s), nil
}
