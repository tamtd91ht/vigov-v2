package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeFieldCode(t *testing.T) {
	for name, tc := range map[string]struct {
		in   string
		want string
		err  error
	}{
		"plain":              {"legal_form", "legal_form", nil},
		"digits after first": {"revenue_2026", "revenue_2026", nil},
		"single letter":      {"a", "a", nil},
		"trimmed":            {"  legal_form ", "legal_form", nil},
		"exactly 64":         {"a" + strings.Repeat("b", 63), "a" + strings.Repeat("b", 63), nil},
		"empty":              {"", "", ErrFieldCodeEmpty},
		"blank":              {"   ", "", ErrFieldCodeEmpty},
		"65 chars":           {"a" + strings.Repeat("b", 64), "", ErrFieldCodeTooLong},
		// Refused, never lowered: the stored key must be the key the person typed.
		"upper case":         {"Legal_form", "", ErrFieldCodeShape},
		"leading digit":      {"2026_revenue", "", ErrFieldCodeShape},
		"leading underscore": {"_legal", "", ErrFieldCodeShape},
		// The catalogue CODE form (kebab) is not the field KEY form (snake).
		"dash":       {"legal-form", "", ErrFieldCodeShape},
		"space":      {"legal form", "", ErrFieldCodeShape},
		"diacritics": {"loại_hình", "", ErrFieldCodeShape},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := NormalizeFieldCode(tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeFieldLabel(t *testing.T) {
	for name, tc := range map[string]struct {
		in   string
		want string
		err  error
	}{
		"vietnamese":   {"Loại hình doanh nghiệp", "Loại hình doanh nghiệp", nil},
		"trimmed":      {"  Doanh thu ước ", "Doanh thu ước", nil},
		"255 runes":    {strings.Repeat("ệ", 255), strings.Repeat("ệ", 255), nil},
		"empty":        {"", "", ErrFieldLabelEmpty},
		"blank":        {" \t ", "", ErrFieldLabelEmpty},
		"256 runes":    {strings.Repeat("ệ", 256), "", ErrFieldLabelTooLong},
		"control char": {"Doanh\x00thu", "", ErrFieldLabelEmpty},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := NormalizeFieldLabel(tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidValueTypeIsExactlyTheSix(t *testing.T) {
	// The same six the CHECK constraint on map_field_schema admits. English spellings from the
	// prototype (`text`, `integer`) are refused: enum VALUES are Vietnamese (ADR 0011).
	for _, v := range []string{"van-ban", "so-nguyen", "so-thap-phan", "dung-sai", "ngay", "chon"} {
		if !ValidValueType(v) {
			t.Errorf("%q refused, want accepted", v)
		}
	}
	for _, v := range []string{"", "text", "integer", "number", "boolean", "date", "Chon", "chon-nhieu"} {
		if ValidValueType(v) {
			t.Errorf("%q accepted, want refused", v)
		}
	}
}

func TestValidateFieldSortOrder(t *testing.T) {
	for n, want := range map[int]error{
		0: nil, 1: nil, FieldSortOrderMax: nil,
		-1: ErrSortOrderRange, FieldSortOrderMax + 1: ErrSortOrderRange,
	} {
		if err := ValidateFieldSortOrder(n); !errors.Is(err, want) {
			t.Errorf("sort_order %d: err = %v, want %v", n, err, want)
		}
	}
}

func TestNormalizeOptions(t *testing.T) {
	many := make([]FieldOption, FieldOptionsMax+1)
	for i := range many {
		many[i] = FieldOption{Value: "v" + strings.Repeat("x", i%10) + string(rune('a'+i%26)), Label: "L"}
	}
	for name, tc := range map[string]struct {
		valueType string
		in        []FieldOption
		want      []FieldOption
		err       error
	}{
		"choice with options": {ValueTypeChoice,
			[]FieldOption{{" tnhh ", " Công ty TNHH "}, {"cp", "Công ty cổ phần"}},
			[]FieldOption{{"tnhh", "Công ty TNHH"}, {"cp", "Công ty cổ phần"}}, nil},
		// `chon` needs options: a choice field with nothing to choose is a form nobody can fill.
		"choice without options": {ValueTypeChoice, nil, nil, ErrOptionsRequired},
		"choice empty list":      {ValueTypeChoice, []FieldOption{}, nil, ErrOptionsRequired},
		"choice duplicate value": {ValueTypeChoice,
			[]FieldOption{{"cp", "Cổ phần"}, {" cp", "Cổ phần 2"}}, nil, ErrOptionValueDup},
		"choice value empty": {ValueTypeChoice, []FieldOption{{"", "Nhãn"}}, nil, ErrOptionValueEmpty},
		"choice label empty": {ValueTypeChoice, []FieldOption{{"cp", " "}}, nil, ErrOptionLabelEmpty},
		"choice value too long": {ValueTypeChoice,
			[]FieldOption{{strings.Repeat("v", FieldOptionValueMaxLen+1), "Nhãn"}}, nil, ErrOptionValueTooLong},
		// Other types take none, and an absent list comes back as [] (never nil).
		"text without options": {ValueTypeText, nil, []FieldOption{}, nil},
		"text with options": {ValueTypeText,
			[]FieldOption{{"a", "A"}}, nil, ErrOptionsNotAllowed},
		"decimal with options": {ValueTypeDecimal,
			[]FieldOption{{"a", "A"}}, nil, ErrOptionsNotAllowed},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := NormalizeOptions(tc.valueType, tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if tc.err == nil && (got == nil || !SameOptions(got, tc.want)) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
	t.Run("too many", func(t *testing.T) {
		if _, err := NormalizeOptions(ValueTypeChoice, many); !errors.Is(err, ErrOptionsTooMany) {
			t.Errorf("err = %v, want ErrOptionsTooMany", err)
		}
	})
}

func TestCheckOptionsKept(t *testing.T) {
	before := []FieldOption{{"tnhh", "Công ty TNHH"}, {"cp", "Công ty cổ phần"}}
	for name, tc := range map[string]struct {
		after []FieldOption
		err   error
	}{
		"unchanged": {before, nil},
		"relabel":   {[]FieldOption{{"tnhh", "TNHH"}, {"cp", "Cổ phần"}}, nil},
		"append":    {append(append([]FieldOption{}, before...), FieldOption{"hkd", "Hộ kinh doanh"}), nil},
		"reorder":   {[]FieldOption{before[1], before[0]}, nil},
		// Assets store the VALUE; whether any uses `cp` cannot be known, so removal is refused.
		"remove one":     {[]FieldOption{before[0]}, ErrOptionRemoved},
		"rename a value": {[]FieldOption{{"tnhh", "TNHH"}, {"co-phan", "Cổ phần"}}, ErrOptionRemoved},
	} {
		t.Run(name, func(t *testing.T) {
			if err := CheckOptionsKept(before, tc.after); !errors.Is(err, tc.err) {
				t.Errorf("err = %v, want %v", err, tc.err)
			}
		})
	}
}

func TestSubjectIsTheBusinessAddress(t *testing.T) {
	m := MapFieldSchema{ID: "01JINTERNAL", AssetTypeCode: "nhom-mau", FieldCode: "legal_form"}
	if got := m.Subject(); got != "nhom-mau/legal_form" {
		t.Errorf("Subject() = %q — the trail files the (type, key) address, never the internal id", got)
	}
}
