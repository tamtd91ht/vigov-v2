package domain

import (
	"errors"
	"regexp"
)

// PetitionField is one row of `petition_field` (migration 0011): a tier-1 petition field code and
// its platform defaults (ADR 0026, ADR 0060). Configuration shared by every commune — no commune,
// no counts, no personal data.
//
// There is no deleted/withdrawn state: a code is never removed, only retired (Active false), and a
// retired code is still returned so old petitions keep their label (ADR 0060 §4).
type PetitionField struct {
	Code         string
	DefaultLabel string
	SortOrder    int32
	Icon         string // "" = not declared
	Tone         string // "" = not declared
	Active       bool
}

// The operator's input rules for a tier-1 row (ADR 0073 #3). Each mirrors a CHECK of migration 0011
// where one exists, so the console is refused with a field-level answer instead of a 500.
var (
	ErrPetitionFieldCodeInvalid      = errors.New("petition_field: mã không đúng dạng (chữ thường không dấu, số, gạch nối; tối đa 64)")
	ErrPetitionFieldLabelInvalid     = errors.New("petition_field: nhãn mặc định không hợp lệ")
	ErrPetitionFieldSortOrderInvalid = errors.New("petition_field: thứ tự phải từ 1 đến 10000")
	ErrPetitionFieldIconInvalid      = errors.New("petition_field: tên biểu tượng không hợp lệ")
	ErrPetitionFieldToneInvalid      = errors.New("petition_field: tông màu không thuộc bộ cho phép")
)

const (
	// MaxPetitionFieldCodeLength is 0011's CHECK and service-petitions' domain.LinhVucToiDa: a code
	// this table accepts must be one that service can store.
	MaxPetitionFieldCodeLength = 64
	MaxPetitionFieldLabelRunes = 200
	// MaxPetitionFieldSortOrder is a sanity bound, not a rule: the set is a few dozen rows.
	MaxPetitionFieldSortOrder = 10000
)

// petitionFieldCode is 0011's CHECK petition_field_code_shape (ADR 0011's enum-value spelling).
var petitionFieldCode = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// petitionFieldIcon is a lucide icon NAME in its component spelling (`Trash2`, `ShieldAlert`) — the
// form every seeded row uses. Not checked against the library: the icon set is the client's and
// grows with it (0011's comment); the shape only keeps a pasted URL or markup out of the column.
var petitionFieldIcon = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{0,63}$`)

// PetitionFieldTones is 0011's CHECK petition_field_tone_known minus "" — the citizen app's six
// design-token tones. A seventh tone is a client release first, then a migration.
var PetitionFieldTones = []string{"blue", "green", "orange", "purple", "cyan", "red"}

// ValidatePetitionFieldCode checks a NEW code. It does not normalise: the code is stored on archival
// petitions as typed, forever (ADR 0060 §4), so a code that only matches after lower-casing is a
// code the operator did not write.
func ValidatePetitionFieldCode(code string) error {
	if len(code) == 0 || len(code) > MaxPetitionFieldCodeLength || !petitionFieldCode.MatchString(code) {
		return ErrPetitionFieldCodeInvalid
	}
	return nil
}

// ValidatePetitionFieldPresentation checks and trims what an operator may edit on a row: the default
// label, the default order, the icon and the tone. Code and Active are not read — the code never
// changes (0011's trigger), and activation is its own act with its own reason. The caller has applied
// NFC to the label (this package imports only the standard library).
func ValidatePetitionFieldPresentation(f PetitionField) (PetitionField, error) {
	label, err := validText(f.DefaultLabel, MaxPetitionFieldLabelRunes, false, ErrPetitionFieldLabelInvalid)
	if err != nil {
		return PetitionField{}, err
	}
	f.DefaultLabel = label
	if f.SortOrder < 1 || f.SortOrder > MaxPetitionFieldSortOrder {
		return PetitionField{}, ErrPetitionFieldSortOrderInvalid
	}
	if f.Icon != "" && !petitionFieldIcon.MatchString(f.Icon) {
		return PetitionField{}, ErrPetitionFieldIconInvalid
	}
	if f.Tone != "" && !knownTone(f.Tone) {
		return PetitionField{}, ErrPetitionFieldToneInvalid
	}
	return f, nil
}

func knownTone(t string) bool {
	for _, k := range PetitionFieldTones {
		if t == k {
			return true
		}
	}
	return false
}

// SamePetitionFieldPresentation reports whether two rows show the same editable values.
func SamePetitionFieldPresentation(a, b PetitionField) bool {
	return a.DefaultLabel == b.DefaultLabel && a.SortOrder == b.SortOrder && a.Icon == b.Icon && a.Tone == b.Tone
}
