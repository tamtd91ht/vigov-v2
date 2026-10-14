package xlsx

import "strings"

// ChoiceSeparator joins a code and its label in a dropdown value ("van-phong · Văn phòng"). The same
// separator service-identity's staff template has used since it shipped (domain.ChoiceSeparator), so
// a person sees one shape in every template of the system.
//
// WHY A CODE AND A LABEL, NOT ONE OF THEM: the person filling the file recognises the label; the
// parser must identify the row by the code, because labels repeat (two "Tổ 1" in two villages) and
// change (a renamed unit). ChoiceCode recovers the code, so a parser accepts either the dropdown
// value or a bare code typed by hand — the shape templates accepted before they had dropdowns.
const ChoiceSeparator = " · "

// ChoiceValue is the dropdown text of one entry. An empty label gives the bare code.
func ChoiceValue(code, label string) string {
	if strings.TrimSpace(label) == "" {
		return code
	}
	return code + ChoiceSeparator + label
}

// ChoiceCode is the code part of a cell: what precedes the first ChoiceSeparator, trimmed, or the
// whole trimmed cell when it holds none. It never consults a list — the caller resolves the code as
// it resolved a typed code before, so a dropdown adds no path that skips validation.
func ChoiceCode(cell string) string {
	code, _, _ := strings.Cut(strings.TrimSpace(cell), ChoiceSeparator)
	return strings.TrimSpace(code)
}
