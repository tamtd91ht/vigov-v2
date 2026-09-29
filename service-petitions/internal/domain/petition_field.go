package domain

import (
	"errors"
	"sort"
)

// The petition field catalogue as a commune sees it: tier 1 (the closed code set, owned by service
// `platform`, ADR 0026 / 0060) merged with tier 2 (this commune's overrides, NhanLinhVuc).
//
// THIS FILE HOLDS THE MERGE AND THE EDIT RULES AND NOTHING ELSE. It does not know where tier 1 comes
// from — the app layer adapts the platform reader into []FieldDefault — so the rules are testable
// without a network and the domain imports only the standard library.

// FieldDefault is one tier-1 entry as the platform ships it.
type FieldDefault struct {
	Code         string
	DefaultLabel string
	SortOrder    int
	Icon         string // "" = not declared
	Tone         string // "" = not declared or unknown to this build
	// Active false: retired platform-wide. Still a real code — labelled on every read path — but
	// refused for new intake and classification (ADR 0060 §4).
	Active bool
}

// PetitionFieldView is one code with the commune's overrides applied.
type PetitionFieldView struct {
	Code         string
	Label        string // effective: the commune's wording, else DefaultLabel
	DefaultLabel string
	Order        int // effective: the commune's position, else DefaultOrder
	DefaultOrder int
	Icon         string
	Tone         string
	Active       bool // tier 1
	Enabled      bool // tier 2 — offered on the citizen's new-submission form
	// Customised is DERIVED: some override differs from the inherit value. Not "a row exists".
	Customised bool
}

// OfferedToCitizens is the one rule deciding whether the citizen's form lists a code.
//
// `can-bo` (LinhVucHanChe) IS NEVER OFFERED YET, whatever the commune's switch says. ADR 0050 point
// 10 lets citizens pick it, but its leaders-only flow needs a permission key `quyen` does not have
// (open question #27) and a per-commune feature flag that does not exist. Offering it before both
// would file staff-conduct complaints into the ordinary register, readable by every officer holding
// `feedback.read` — the exposure that flow exists to prevent.
func (v PetitionFieldView) OfferedToCitizens() bool {
	return v.Active && v.Enabled && v.Code != LinhVucHanChe
}

// MergePetitionFields applies overrides to the tier-1 set and returns EVERY tier-1 code, retired
// ones included, in effective order: position, then tier-1 order, then code — a total order, so
// two reads never swap two entries.
//
// AN OVERRIDE FOR A CODE TIER 1 DOES NOT KNOW IS DROPPED, not shown: there is no default label to
// fall back on, and a raw code on a government screen is the `ve-sinh-moi-truong` defect (ADR 0026
// §2). The write path refuses such codes, so only a row written before that check can hit this.
func MergePetitionFields(defaults []FieldDefault, overrides []NhanLinhVuc) []PetitionFieldView {
	byCode := make(map[string]NhanLinhVuc, len(overrides))
	for _, o := range overrides {
		byCode[o.Ma] = o
	}
	out := make([]PetitionFieldView, 0, len(defaults))
	for _, d := range defaults {
		var o *NhanLinhVuc
		if row, ok := byCode[d.Code]; ok {
			o = &row
		}
		out = append(out, viewOf(d, o))
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		if a.DefaultOrder != b.DefaultOrder {
			return a.DefaultOrder < b.DefaultOrder
		}
		return a.Code < b.Code
	})
	return out
}

func viewOf(d FieldDefault, o *NhanLinhVuc) PetitionFieldView {
	v := PetitionFieldView{
		Code: d.Code, Label: d.DefaultLabel, DefaultLabel: d.DefaultLabel,
		Order: d.SortOrder, DefaultOrder: d.SortOrder,
		Icon: d.Icon, Tone: d.Tone, Active: d.Active, Enabled: true,
	}
	if o == nil {
		return v
	}
	if o.Nhan != "" {
		v.Label = o.Nhan
	}
	if o.SortOrder > 0 {
		v.Order = o.SortOrder
	}
	v.Enabled = o.Enabled
	v.Customised = v.Label != v.DefaultLabel || v.Order != v.DefaultOrder || !v.Enabled
	return v
}

// CitizenCatalogue keeps what the citizen's form may offer, in the commune's order.
func CitizenCatalogue(all []PetitionFieldView) []PetitionFieldView {
	out := make([]PetitionFieldView, 0, len(all))
	for _, v := range all {
		if v.OfferedToCitizens() {
			out = append(out, v)
		}
	}
	return out
}

// FindFieldDefault returns the tier-1 entry for code, retired or not.
func FindFieldDefault(defaults []FieldDefault, code string) (FieldDefault, bool) {
	for _, d := range defaults {
		if d.Code == code {
			return d, true
		}
	}
	return FieldDefault{}, false
}

// ErrFieldEditEmpty — a PATCH naming none of the three fields. Refused rather than answered 200:
// a client that sent nothing it meant to believes it changed something.
var ErrFieldEditEmpty = errors.New("linh_vuc: cần ít nhất một trong `label`, `order`, `enabled`")

// PetitionFieldEdit is a PARTIAL edit: nil leaves the field alone.
type PetitionFieldEdit struct {
	Label   *string
	Order   *int
	Enabled *bool
}

// ApplyFieldEdit returns the override row after the edit.
//
// SENDING THE DEFAULT IS HOW A FIELD GOES BACK TO INHERITING IT: a label equal to the tier-1 default
// is stored as "" (NULL) and an order equal to the tier-1 order as 0. So "về mặc định" needs no
// separate verb and no DELETE, and a later correction of the default on the platform reaches this
// commune instead of being frozen by a copy.
//
// The label bound is 100 characters, the migration's CHECK (ChuanHoaNhanTrangThai enforces exactly
// that bound); the order is 1..ThuTuToiDa (KiemTraThuTuTrangThai) — 0 is the storage spelling of
// "inherit", never a position a client sends.
func ApplyFieldEdit(d FieldDefault, cur NhanLinhVuc, e PetitionFieldEdit) (NhanLinhVuc, error) {
	if e.Label == nil && e.Order == nil && e.Enabled == nil {
		return NhanLinhVuc{}, ErrFieldEditEmpty
	}
	next := cur
	next.Ma = d.Code
	if e.Label != nil {
		label, err := ChuanHoaNhanTrangThai(*e.Label)
		if err != nil {
			return NhanLinhVuc{}, err
		}
		if label == d.DefaultLabel {
			label = ""
		}
		next.Nhan = label
	}
	if e.Order != nil {
		if err := KiemTraThuTuTrangThai(*e.Order); err != nil {
			return NhanLinhVuc{}, err
		}
		order := *e.Order
		if order == d.SortOrder {
			order = 0
		}
		next.SortOrder = order
	}
	if e.Enabled != nil {
		next.Enabled = *e.Enabled
	}
	return next, nil
}

// ViewOfOverride is the view of one code given its (possibly absent) override.
func ViewOfOverride(d FieldDefault, o *NhanLinhVuc) PetitionFieldView { return viewOf(d, o) }
