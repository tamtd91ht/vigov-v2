package domain

// The DYNAMIC ATTRIBUTES of the citizen directory (ADR 0101; migration 0036): a commune defines up to 50
// fields of its own ("Hộ nghèo", "Ngày sinh"), and each directory row carries their values. This file
// holds the shape rules of a definition, of an edit to one, and of the values a contact form sends — the
// same rules the CHECKs and the trigger of 0036 hold, refused here first with a sentence.
//
// STANDARD LIBRARY ONLY (doc.go). No commune: tenant_id travels in the context (rule 1, invariant 4).
//
// NO REFUSAL QUOTES A VALUE (rule 3, forbidden #3): an attribute value may be personal data the commune
// chose to record. A sentence names the field by its LABEL (configuration the commune wrote, not data about
// a person) and the rule broken.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// CitizenFieldType is `citizen_field.field_type` — the owner's six words, kept as written (ADR 0101 #2).
type CitizenFieldType string

const (
	FieldSingleText CitizenFieldType = "singleText" // one line of text
	FieldMultiText  CitizenFieldType = "multiText"  // several lines
	FieldCombobox   CitizenFieldType = "combobox"   // choose SEVERAL options — stored as an array
	FieldSingleBox  CitizenFieldType = "singleBox"  // choose ONE option
	FieldNumber     CitizenFieldType = "number"     // integer or decimal (NumberKind)
	FieldDateTime   CitizenFieldType = "dateTime"   // epoch millis, shown with DatePattern
)

// Valid reports whether t is one of the six.
func (t CitizenFieldType) Valid() bool {
	switch t {
	case FieldSingleText, FieldMultiText, FieldCombobox, FieldSingleBox, FieldNumber, FieldDateTime:
		return true
	}
	return false
}

// HasOptions reports whether the type chooses from a list.
func (t CitizenFieldType) HasOptions() bool { return t == FieldCombobox || t == FieldSingleBox }

// IsText reports whether the type is free text (the only types with a placeholder).
func (t CitizenFieldType) IsText() bool { return t == FieldSingleText || t == FieldMultiText }

// The two number kinds (ADR 0101 #1: "số integer/decimal").
const (
	NumberInteger = "integer"
	NumberDecimal = "decimal"
)

// CitizenFieldDatePatterns is the fixed list of display patterns (0036 CHECK citizen_field_date_pattern_valid).
var CitizenFieldDatePatterns = []string{"dd/MM/yyyy", "dd/MM/yyyy HH:mm", "MM/yyyy", "yyyy"}

// Bounds. Every one refuses a payload, never honest input.
const (
	MaxCitizenFieldsLive       = 50 // ADR 0101 #1
	MaxCitizenFieldLabel       = 100
	MaxCitizenFieldPlaceholder = 200
	MaxCitizenFieldOptions     = 100 // the web form's bound (coordinator, 17/10/2026); 0036 CHECK agrees
	MaxCitizenFieldOptionValue = 40  // likewise
	MaxCitizenFieldOptionLabel = 100
	MaxCitizenFieldSortOrder   = 9999
	MaxCitizenFieldRetireLen   = 500
	MaxAttributeSingleText     = 255  // task card E-1
	MaxAttributeMultiText      = 2000 // task card E-1
	// maxIntegerDigits keeps a number exact as an ES double (2^53 ≈ 9.007e15).
	maxIntegerDigits = 15
	// maxEpochMillis is ±8.64e15 — the range of a JavaScript Date, which the browser parses the value into.
	maxEpochMillis = 8_640_000_000_000_000
	// MaxCitizenContactEmail is RFC 5321's path limit (0036 CHECK citizen_contact_email_shape).
	MaxCitizenContactEmail = 254
)

var (
	citizenFieldCodeShape   = regexp.MustCompile(`^[a-z][a-z0-9]{5,7}$`)
	citizenOptionValueShape = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	integerShape            = regexp.MustCompile(`^-?[0-9]{1,15}$`)
	decimalShape            = regexp.MustCompile(`^-?[0-9]{1,15}(\.[0-9]{1,4})?$`)
)

// Definition refusals.
var (
	ErrCitizenFieldCodeInvalid     = errors.New("thuộc tính danh bạ: mã gồm 6–8 ký tự a-z, 0-9 và bắt đầu bằng chữ cái")
	ErrCitizenFieldLabelMissing    = errors.New("thuộc tính danh bạ: thiếu tên hiển thị")
	ErrCitizenFieldLabelTooLong    = errors.New("thuộc tính danh bạ: tên hiển thị quá dài")
	ErrCitizenFieldTypeInvalid     = errors.New("thuộc tính danh bạ: loại không hợp lệ (singleText, multiText, combobox, singleBox, number, dateTime)")
	ErrCitizenFieldImmutable       = errors.New("thuộc tính danh bạ: mã, loại và kiểu số không đổi được sau khi tạo")
	ErrCitizenFieldPlaceholder     = errors.New("thuộc tính danh bạ: chỉ ô chữ (singleText, multiText) có placeholder, tối đa 200 ký tự")
	ErrCitizenFieldNumberKind      = errors.New("thuộc tính danh bạ: loại number phải chọn integer hoặc decimal; loại khác không có kiểu số")
	ErrCitizenFieldDatePattern     = errors.New("thuộc tính danh bạ: định dạng ngày phải là dd/MM/yyyy, dd/MM/yyyy HH:mm, MM/yyyy hoặc yyyy, và chỉ cho loại dateTime")
	ErrCitizenFieldOptionsMissing  = errors.New("thuộc tính danh bạ: combobox và singleBox phải có ít nhất một lựa chọn; loại khác không có lựa chọn")
	ErrCitizenFieldOptionInvalid   = errors.New("thuộc tính danh bạ: mã lựa chọn gồm a-z, 0-9 nối bằng dấu gạch ngang (vd. ho-ngheo), tối đa 40 ký tự; nhãn bắt buộc, tối đa 100 ký tự")
	ErrCitizenFieldOptionDuplicate = errors.New("thuộc tính danh bạ: mã lựa chọn bị trùng")
	ErrCitizenFieldOptionRemoved   = errors.New("thuộc tính danh bạ: lựa chọn đã có không xoá được — hãy đánh dấu ngừng dùng")
	ErrCitizenFieldTooManyOptions  = errors.New("thuộc tính danh bạ: quá nhiều lựa chọn (tối đa 100)")
	ErrCitizenFieldSortOrderRange  = errors.New("thuộc tính danh bạ: thứ tự phải từ 0 đến 9999")
	ErrCitizenFieldReasonMissing   = errors.New("thuộc tính danh bạ: phải nêu lý do ngừng dùng")
	ErrCitizenFieldReasonTooLong   = errors.New("thuộc tính danh bạ: lý do ngừng dùng quá dài")
	ErrCitizenFieldOrderMismatch   = errors.New("thuộc tính danh bạ: danh sách sắp xếp phải gồm đúng mọi mã thuộc tính đang dùng, mỗi mã một lần")
)

// Value refusals. Each is wrapped with the field's LABEL, never the value.
var (
	ErrCitizenAttributeUnknown  = errors.New("thuộc tính danh bạ: mã thuộc tính không có trong cấu hình của xã")
	ErrCitizenAttributeInvalid  = errors.New("thuộc tính danh bạ: giá trị không hợp lệ")
	ErrCitizenAttributeRequired = errors.New("thuộc tính danh bạ: thiếu thuộc tính bắt buộc")
	ErrCitizenContactEmail      = errors.New("danh bạ người dân: email không đúng dạng (ten@mien.vn), tối đa 254 ký tự")
)

// CitizenFieldOption is one choice of a combobox / singleBox. Value is immutable and never removed;
// Retired hides it from the form and the filters (ADR 0101 #1).
type CitizenFieldOption struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Retired bool   `json:"retired"`
}

// CitizenField is one row of `citizen_field`.
type CitizenField struct {
	Code         string
	Label        string
	Type         CitizenFieldType
	Placeholder  string
	Options      []CitizenFieldOption
	NumberKind   string
	DatePattern  string
	Required     bool
	ShowInList   bool
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	RetiredAt    *time.Time
	RetiredBy    string
	RetireReason string
}

// Live reports whether the field is in use.
func (f CitizenField) Live() bool { return f.RetiredAt == nil }

// CitizenFieldInput is a new field as the configuration form sends it.
type CitizenFieldInput struct {
	Code        string
	Label       string
	Type        CitizenFieldType
	Placeholder string
	Options     []CitizenFieldOption
	NumberKind  string
	DatePattern string
	Required    bool
	ShowInList  bool
}

// CheckNewCitizenField applies every rule of a definition and returns it normalised (trimmed label and
// placeholder, trimmed option labels, Retired false on every option of a new field).
func CheckNewCitizenField(in CitizenFieldInput) (CitizenFieldInput, error) {
	out := in
	out.Code = strings.TrimSpace(in.Code)
	if !citizenFieldCodeShape.MatchString(out.Code) {
		return CitizenFieldInput{}, ErrCitizenFieldCodeInvalid
	}
	if !in.Type.Valid() {
		return CitizenFieldInput{}, ErrCitizenFieldTypeInvalid
	}
	var err error
	if out.Label, err = normalizeFieldLabel(in.Label); err != nil {
		return CitizenFieldInput{}, err
	}
	if out.Placeholder, err = normalizePlaceholder(in.Type, in.Placeholder); err != nil {
		return CitizenFieldInput{}, err
	}
	switch {
	case in.Type == FieldNumber && in.NumberKind != NumberInteger && in.NumberKind != NumberDecimal,
		in.Type != FieldNumber && in.NumberKind != "":
		return CitizenFieldInput{}, ErrCitizenFieldNumberKind
	}
	if err := checkDatePattern(in.Type, in.DatePattern); err != nil {
		return CitizenFieldInput{}, err
	}
	if out.Options, err = normalizeOptions(in.Type, in.Options, nil); err != nil {
		return CitizenFieldInput{}, err
	}
	for i := range out.Options {
		out.Options[i].Retired = false
	}
	return out, nil
}

// CitizenFieldPatch is an edit: nil = unchanged. Code, Type and NumberKind are present only so a client
// that sends them can be told they are immutable (ADR 0101 #1).
type CitizenFieldPatch struct {
	Label       *string
	Placeholder *string
	Options     *[]CitizenFieldOption
	Required    *bool
	ShowInList  *bool
	DatePattern *string
	Type        *CitizenFieldType
	NumberKind  *string
}

// ApplyCitizenFieldPatch returns the field after the edit and, by column name, what changed. An option
// already defined may be relabelled or (un)retired but never removed; new options are added.
func ApplyCitizenFieldPatch(cur CitizenField, p CitizenFieldPatch) (CitizenField, []string, error) {
	if (p.Type != nil && *p.Type != cur.Type) || (p.NumberKind != nil && *p.NumberKind != cur.NumberKind) {
		return CitizenField{}, nil, ErrCitizenFieldImmutable
	}
	next := cur
	var changed []string
	if p.Label != nil {
		l, err := normalizeFieldLabel(*p.Label)
		if err != nil {
			return CitizenField{}, nil, err
		}
		if l != cur.Label {
			next.Label = l
			changed = append(changed, "label")
		}
	}
	if p.Placeholder != nil {
		ph, err := normalizePlaceholder(cur.Type, *p.Placeholder)
		if err != nil {
			return CitizenField{}, nil, err
		}
		if ph != cur.Placeholder {
			next.Placeholder = ph
			changed = append(changed, "placeholder")
		}
	}
	if p.DatePattern != nil && *p.DatePattern != cur.DatePattern {
		if err := checkDatePattern(cur.Type, *p.DatePattern); err != nil {
			return CitizenField{}, nil, err
		}
		next.DatePattern = *p.DatePattern
		changed = append(changed, "date_pattern")
	}
	if p.Options != nil {
		opts, err := normalizeOptions(cur.Type, *p.Options, cur.Options)
		if err != nil {
			return CitizenField{}, nil, err
		}
		if !sameOptions(cur.Options, opts) {
			next.Options = opts
			changed = append(changed, "options")
		}
	}
	if p.Required != nil && *p.Required != cur.Required {
		next.Required = *p.Required
		changed = append(changed, "required")
	}
	if p.ShowInList != nil && *p.ShowInList != cur.ShowInList {
		next.ShowInList = *p.ShowInList
		changed = append(changed, "show_in_list")
	}
	return next, changed, nil
}

// NormalizeCitizenFieldRetireReason trims and bounds the MANDATORY reason of "ngừng dùng".
func NormalizeCitizenFieldRetireReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrCitizenFieldReasonMissing
	}
	if utf8.RuneCountInString(reason) > MaxCitizenFieldRetireLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenFieldReasonTooLong, MaxCitizenFieldRetireLen)
	}
	return reason, nil
}

// CheckCitizenFieldOrder checks that `codes` is exactly the set of live codes, each once.
func CheckCitizenFieldOrder(live []CitizenField, codes []string) error {
	if len(codes) != len(live) {
		return ErrCitizenFieldOrderMismatch
	}
	want := make(map[string]bool, len(live))
	for _, f := range live {
		want[f.Code] = true
	}
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if !want[c] || seen[c] {
			return ErrCitizenFieldOrderMismatch
		}
		seen[c] = true
	}
	return nil
}

func normalizeFieldLabel(raw string) (string, error) {
	l := gomKhoangTrang(raw)
	if l == "" {
		return "", ErrCitizenFieldLabelMissing
	}
	if utf8.RuneCountInString(l) > MaxCitizenFieldLabel {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenFieldLabelTooLong, MaxCitizenFieldLabel)
	}
	return l, nil
}

func normalizePlaceholder(t CitizenFieldType, raw string) (string, error) {
	ph := strings.TrimSpace(raw)
	if ph == "" {
		return "", nil
	}
	if !t.IsText() || utf8.RuneCountInString(ph) > MaxCitizenFieldPlaceholder {
		return "", ErrCitizenFieldPlaceholder
	}
	return ph, nil
}

func checkDatePattern(t CitizenFieldType, p string) error {
	if t != FieldDateTime {
		if p != "" {
			return ErrCitizenFieldDatePattern
		}
		return nil
	}
	for _, ok := range CitizenFieldDatePatterns {
		if p == ok {
			return nil
		}
	}
	return ErrCitizenFieldDatePattern
}

// normalizeOptions validates an option list. `prev` is the stored list on an edit: every value of it must
// still be present (never removed — the 0036 trigger refuses it too), in any order.
func normalizeOptions(t CitizenFieldType, in []CitizenFieldOption, prev []CitizenFieldOption) ([]CitizenFieldOption, error) {
	if !t.HasOptions() {
		if len(in) > 0 {
			return nil, ErrCitizenFieldOptionsMissing
		}
		return nil, nil
	}
	if len(in) == 0 {
		return nil, ErrCitizenFieldOptionsMissing
	}
	if len(in) > MaxCitizenFieldOptions {
		return nil, ErrCitizenFieldTooManyOptions
	}
	out := make([]CitizenFieldOption, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, o := range in {
		v := strings.TrimSpace(o.Value)
		l := gomKhoangTrang(o.Label)
		if !citizenOptionValueShape.MatchString(v) || len(v) > MaxCitizenFieldOptionValue ||
			l == "" || utf8.RuneCountInString(l) > MaxCitizenFieldOptionLabel {
			return nil, ErrCitizenFieldOptionInvalid
		}
		if seen[v] {
			return nil, ErrCitizenFieldOptionDuplicate
		}
		seen[v] = true
		out = append(out, CitizenFieldOption{Value: v, Label: l, Retired: o.Retired})
	}
	for _, p := range prev {
		if !seen[p.Value] {
			return nil, ErrCitizenFieldOptionRemoved
		}
	}
	return out, nil
}

func sameOptions(a, b []CitizenFieldOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- values -------------------------------------------------------------------------------------------

// CheckCitizenAttributes validates the attribute values a contact form sends against the commune's
// definitions (live AND retired) and returns the object to store.
//
//   - a code with no definition in this commune is refused (ErrCitizenAttributeUnknown);
//   - a code of a RETIRED definition is ignored: whatever was sent, the stored value is kept;
//   - `null` or an empty value clears the attribute;
//   - values stored under RETIRED codes — and codes with no definition at all — are carried over from
//     `stored` untouched (ADR 0101 #4: ngừng dùng = giữ dữ liệu);
//   - an option the commune has since retired is accepted only if this person already had it;
//   - every live required field must end with a value.
//
// The values come back normalised: text trimmed, combobox sorted in the definition's option order,
// numbers as their canonical decimal text.
func CheckCitizenAttributes(defs []CitizenField, in map[string]json.RawMessage, stored map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	byCode := make(map[string]CitizenField, len(defs))
	for _, d := range defs {
		byCode[d.Code] = d
	}
	out := make(map[string]json.RawMessage, len(in)+len(stored))
	for code, v := range stored {
		if d, ok := byCode[code]; !ok || !d.Live() {
			out[code] = v
		}
	}
	for code, raw := range in {
		d, ok := byCode[code]
		if !ok {
			// The code is configuration, not personal data: naming it tells the form which key to drop.
			return nil, fmt.Errorf("%w: %q", ErrCitizenAttributeUnknown, code)
		}
		if !d.Live() {
			// The web form re-sends the values of retired fields untouched (coordinator, 17/10/2026). What it
			// sends is IGNORED and the stored value kept as it is (copied above): a retired field is never
			// edited through the form, and refusing the whole save for it would block every edit.
			continue
		}
		v, err := checkAttributeValue(d, raw, stored[code])
		if err != nil {
			return nil, err
		}
		if v != nil {
			out[code] = v
		}
	}
	for _, d := range defs {
		if d.Live() && d.Required {
			if _, ok := out[d.Code]; !ok {
				return nil, fmt.Errorf("%w: %s", ErrCitizenAttributeRequired, d.Label)
			}
		}
	}
	return out, nil
}

// checkAttributeValue returns the normalised JSON of one value, nil for "no value".
func checkAttributeValue(d CitizenField, raw, prev json.RawMessage) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	bad := func(rule string) error { return fmt.Errorf("%w: %s — %s", ErrCitizenAttributeInvalid, d.Label, rule) }
	switch d.Type {
	case FieldSingleText, FieldMultiText:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, bad("phải là chữ")
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		limit := MaxAttributeSingleText
		if d.Type == FieldMultiText {
			limit = MaxAttributeMultiText
		} else if strings.ContainsAny(s, "\r\n") {
			return nil, bad("chỉ một dòng")
		}
		if utf8.RuneCountInString(s) > limit {
			return nil, bad(fmt.Sprintf("tối đa %d ký tự", limit))
		}
		return json.Marshal(s)
	case FieldSingleBox:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, bad("phải là một mã lựa chọn")
		}
		if s == "" {
			return nil, nil
		}
		var had string
		_ = json.Unmarshal(prev, &had)
		if !optionAllowed(d, s, map[string]bool{had: had != ""}) {
			return nil, bad("lựa chọn không có hoặc đã ngừng dùng")
		}
		return json.Marshal(s)
	case FieldCombobox:
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			return nil, bad("phải là danh sách mã lựa chọn")
		}
		if len(list) == 0 {
			return nil, nil
		}
		var hadList []string
		_ = json.Unmarshal(prev, &hadList)
		had := make(map[string]bool, len(hadList))
		for _, h := range hadList {
			had[h] = true
		}
		chosen := make(map[string]bool, len(list))
		for _, s := range list {
			if chosen[s] {
				return nil, bad("một lựa chọn bị chọn hai lần")
			}
			if !optionAllowed(d, s, had) {
				return nil, bad("lựa chọn không có hoặc đã ngừng dùng")
			}
			chosen[s] = true
		}
		ordered := make([]string, 0, len(list))
		for _, o := range d.Options {
			if chosen[o.Value] {
				ordered = append(ordered, o.Value)
			}
		}
		return json.Marshal(ordered)
	case FieldNumber:
		n, ok := jsonNumber(raw)
		if !ok {
			return nil, bad("phải là số")
		}
		shape := decimalShape
		rule := "số thập phân, tối đa 4 chữ số sau dấu chấm"
		if d.NumberKind == NumberInteger {
			shape, rule = integerShape, "số nguyên"
		}
		if !shape.MatchString(n) {
			return nil, bad(rule)
		}
		return json.RawMessage(canonicalNumber(n)), nil
	case FieldDateTime:
		n, ok := jsonNumber(raw)
		if !ok || !integerShape.MatchString(n) {
			return nil, bad("phải là mốc thời gian (mili giây)")
		}
		var ms int64
		if _, err := fmt.Sscan(n, &ms); err != nil || ms > maxEpochMillis || ms < -maxEpochMillis {
			return nil, bad("mốc thời gian ngoài khoảng cho phép")
		}
		return json.RawMessage(fmt.Sprint(ms)), nil
	}
	return nil, bad("loại thuộc tính không xác định")
}

func optionAllowed(d CitizenField, v string, had map[string]bool) bool {
	for _, o := range d.Options {
		if o.Value == v {
			return !o.Retired || had[v]
		}
	}
	return false
}

// jsonNumber reads a JSON number literal as text (never through float64, which would round).
func jsonNumber(raw json.RawMessage) (string, bool) {
	if !json.Valid(raw) {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return "", false
	}
	n, ok := v.(json.Number)
	return string(n), ok
}

// canonicalNumber drops a trailing ".0…" and leading zeros, so "007.50" and "7.5" store alike.
func canonicalNumber(n string) string {
	neg := strings.HasPrefix(n, "-")
	n = strings.TrimPrefix(n, "-")
	intPart, frac, _ := strings.Cut(n, ".")
	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	frac = strings.TrimRight(frac, "0")
	out := intPart
	if frac != "" {
		out += "." + frac
	}
	if neg && out != "0" {
		out = "-" + out
	}
	return out
}

// LiveAttributes keeps the values of LIVE fields only — what leaves the API. Values of retired fields stay
// stored (ADR 0101 #4) but are hidden like the field itself.
func LiveAttributes(defs []CitizenField, stored map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(stored))
	for _, d := range defs {
		if v, ok := stored[d.Code]; ok && d.Live() {
			out[d.Code] = v
		}
	}
	return out
}

// NormalizeCitizenContactEmail trims and lower-cases the OPTIONAL e-mail (ADR 0101 #6) and checks its shape:
// one '@', a local part, a dotted domain, no whitespace, ≤ 254. "" means none.
func NormalizeCitizenContactEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" {
		return "", nil
	}
	local, domain, ok := strings.Cut(e, "@")
	if !ok || local == "" || strings.Contains(domain, "@") || len(e) > MaxCitizenContactEmail ||
		strings.ContainsAny(e, " \t\r\n") || !strings.Contains(domain, ".") ||
		strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", ErrCitizenContactEmail
	}
	return e, nil
}
