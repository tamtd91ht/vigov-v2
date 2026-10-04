package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validAsset() MapAsset {
	return MapAsset{AssetTypeCode: "doanh-nghiep", Name: "Công ty CP Chế biến Nông sản", Lat: 15.7305071, Lng: 108.3781104,
		Status: MapAssetStatusActive}
}

func TestNormalizeMapAssetRoundsAndKeepsRequired(t *testing.T) {
	a, err := NormalizeMapAsset(validAsset())
	if err != nil {
		t.Fatal(err)
	}
	if a.Lat != 15.730507 || a.Lng != 108.37811 {
		t.Errorf("rounded to %v, %v — want six decimals", a.Lat, a.Lng)
	}
	for name, tc := range map[string]struct {
		edit func(*MapAsset)
		want error
	}{
		"no type":        {func(a *MapAsset) { a.AssetTypeCode = " " }, ErrMapAssetTypeEmpty},
		"type shape":     {func(a *MapAsset) { a.AssetTypeCode = "Doanh Nghiep" }, ErrMapAssetTypeEmpty},
		"no name":        {func(a *MapAsset) { a.Name = "  " }, ErrMapAssetNameEmpty},
		"long name":      {func(a *MapAsset) { a.Name = strings.Repeat("a", 256) }, ErrMapAssetNameTooLong},
		"lat range":      {func(a *MapAsset) { a.Lat = 90.5 }, ErrMapAssetLocationOutOfRange},
		"lng range":      {func(a *MapAsset) { a.Lng = -181 }, ErrMapAssetLocationOutOfRange},
		"status":         {func(a *MapAsset) { a.Status = "dong-cua" }, ErrMapAssetStatusUnknown},
		"phone letter":   {func(a *MapAsset) { a.Phone = "0900 máy lẻ 2" }, ErrMapAssetPhoneShape},
		"tax letters":    {func(a *MapAsset) { a.TaxCode = "01A2" }, ErrMapAssetTaxCodeShape},
		"tax two dashes": {func(a *MapAsset) { a.TaxCode = "0101-001-2" }, ErrMapAssetTaxCodeShape},
		"tax trailing":   {func(a *MapAsset) { a.TaxCode = "0101-" }, ErrMapAssetTaxCodeShape},
		"tax too long":   {func(a *MapAsset) { a.TaxCode = strings.Repeat("1", 21) }, ErrMapAssetTaxCodeShape},
		"vsic 3 digits":  {func(a *MapAsset) { a.IndustryCode = "471" }, ErrMapAssetIndustryCodeShape},
		"vsic letters":   {func(a *MapAsset) { a.IndustryCode = "4a" }, ErrMapAssetIndustryCodeShape},
		"employees < 0":  {func(a *MapAsset) { n := -1; a.EmployeeCount = &n }, ErrMapAssetEmployeeCountInvalid},
		"bad date":       {func(a *MapAsset) { a.EstablishedOn = "2026-02-30" }, ErrMapAssetEstablishedOnInvalid},
		"unit shape":     {func(a *MapAsset) { a.ResidentialUnitID = "thôn 1" }, ErrMapAssetResidentialUnitInvalid},
		"address ctrl":   {func(a *MapAsset) { a.Address = "Thôn\x00 1" }, ErrMapAssetAddressInvalid},
		"long desc":      {func(a *MapAsset) { a.Description = strings.Repeat("x", 4001) }, ErrMapAssetDescriptionTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			a := validAsset()
			tc.edit(&a)
			if _, err := NormalizeMapAsset(a); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeMapAssetAcceptsTheLooseShapes(t *testing.T) {
	a := validAsset()
	a.TaxCode, a.IndustryCode, a.Phone = "0101234567-001", "47", "0900000000"
	a.Description = "Dòng một\nDòng hai"
	a.EstablishedOn = "2020-01-31"
	got, err := NormalizeMapAsset(a)
	if err != nil {
		t.Fatal(err)
	}
	// A household filed under the owner's 12-digit ID (0015 header) is a valid tax code.
	a.TaxCode = "079123456789"
	if _, err := NormalizeMapAsset(a); err != nil {
		t.Fatalf("12-digit tax code refused: %v", err)
	}
	if got.Description != "Dòng một\nDòng hai" {
		t.Errorf("description = %q", got.Description)
	}
}

func TestNormalizeMapAssetLocationMissing(t *testing.T) {
	lat := 15.0
	if _, _, err := NormalizeMapAssetLocation(&lat, nil); !errors.Is(err, ErrMapAssetLocationMissing) {
		t.Fatalf("one coordinate accepted: %v", err)
	}
	zero := -0.0000001
	la, ln, err := NormalizeMapAssetLocation(&zero, &zero)
	if err != nil || la != 0 || ln != 0 {
		t.Fatalf("= %v %v %v", la, ln, err)
	}
}

func TestNormalizeMapAssetFilter(t *testing.T) {
	v := true
	f, err := NormalizeMapAssetFilter(MapAssetFilter{
		AssetTypeCodes: []string{"doanh-nghiep,cho", "cho", " "}, Status: "tam-ngung", Verified: &v,
		IndustryCode: "47", Query: "  Bình An ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.AssetTypeCodes) != 2 || f.AssetTypeCodes[0] != "doanh-nghiep" || f.AssetTypeCodes[1] != "cho" {
		t.Errorf("types = %v", f.AssetTypeCodes)
	}
	if f.Query != "Bình An" || f.Verified == nil || !*f.Verified {
		t.Errorf("filter = %+v", f)
	}
	many := make([]string, 21)
	for i := range many {
		many[i] = "nhom-" + string(rune('a'+i))
	}
	for name, bad := range map[string]MapAssetFilter{
		"type shape": {AssetTypeCodes: []string{"Doanh Nghiep"}},
		"too many":   {AssetTypeCodes: many},
		"status":     {Status: "x"},
		"vsic":       {IndustryCode: "4"},
		"q too long": {Query: strings.Repeat("a", 101)},
	} {
		if _, err := NormalizeMapAssetFilter(bad); !errors.Is(err, ErrMapAssetFilterInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestVerifiedRatio(t *testing.T) {
	if r := (MapAssetSummary{Total: 26, Verified: 11}).VerifiedRatio(); r != 0.4231 {
		t.Errorf("ratio = %v, want 0.4231 (spec §1: 42.3%%)", r)
	}
	if r := (MapAssetSummary{}).VerifiedRatio(); r != 0 {
		t.Errorf("empty ratio = %v", r)
	}
}

// --- custom values -------------------------------------------------------------------------------

func schemaDoanhNghiep() []MapFieldSchema {
	return []MapFieldSchema{
		{FieldCode: "legal_form", ValueType: ValueTypeChoice, IsActive: true, IsRequired: true,
			Options: []FieldOption{{Value: "tnhh", Label: "TNHH"}, {Value: "cp", Label: "Cổ phần"}}},
		{FieldCode: "revenue_estimate", ValueType: ValueTypeDecimal, IsActive: true},
		{FieldCode: "staff_count", ValueType: ValueTypeInteger, IsActive: true},
		{FieldCode: "export", ValueType: ValueTypeBoolean, IsActive: true},
		{FieldCode: "licensed_on", ValueType: ValueTypeDate, IsActive: true},
		{FieldCode: "note", ValueType: ValueTypeText, IsActive: true},
		{FieldCode: "old_flag", ValueType: ValueTypeBoolean, IsActive: false},
	}
}

func raw(m map[string]string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = json.RawMessage(v)
	}
	return out
}

func TestApplyCustomValuesAcceptsEveryType(t *testing.T) {
	got, err := ApplyCustomValues(schemaDoanhNghiep(), nil, raw(map[string]string{
		"legal_form": `"cp"`, "revenue_estimate": `1250000000.50`, "staff_count": `42`, "export": `true`,
		"licensed_on": `"2024-05-01"`, "note": `"  ghi chú  "`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"legal_form": `"cp"`, "revenue_estimate": `1250000000.5`, "staff_count": `42`, "export": `true`,
		"licensed_on": `"2024-05-01"`, "note": `"ghi chú"`,
	} {
		if string(got[k]) != want {
			t.Errorf("%s = %s, want %s", k, got[k], want)
		}
	}
	if err := CheckRequiredCustomValues(schemaDoanhNghiep(), got); err != nil {
		t.Errorf("required present but refused: %v", err)
	}
}

func TestApplyCustomValuesRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		patch string
		want  error
		field string
	}{
		"unknown key":      {`{"bogus_field": 1}`, ErrCustomValueUnknownField, "bogus_field"},
		"unsafe key":       {`{"Bad Key!": 1}`, ErrCustomValueUnknownField, "?"},
		"disabled field":   {`{"old_flag": true}`, ErrCustomValueFieldOff, "old_flag"},
		"int as string":    {`{"staff_count": "42"}`, ErrCustomValueWrongType, "staff_count"},
		"int with decimal": {`{"staff_count": 4.2}`, ErrCustomValueWrongType, "staff_count"},
		"decimal as text":  {`{"revenue_estimate": "nhiều"}`, ErrCustomValueWrongType, "revenue_estimate"},
		"bool as int":      {`{"export": 1}`, ErrCustomValueWrongType, "export"},
		"bad date":         {`{"licensed_on": "01/05/2024"}`, ErrCustomValueWrongType, "licensed_on"},
		"not an option":    {`{"legal_form": "dntn"}`, ErrCustomValueNotAnOption, "legal_form"},
		"text as object":   {`{"note": {"a": 1}}`, ErrCustomValueWrongType, "note"},
	} {
		t.Run(name, func(t *testing.T) {
			var patch map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.patch), &patch); err != nil {
				t.Fatal(err)
			}
			_, err := ApplyCustomValues(schemaDoanhNghiep(), nil, patch)
			var cv *CustomValueError
			if !errors.As(err, &cv) || !errors.Is(err, tc.want) || cv.FieldCode != tc.field {
				t.Fatalf("err = %v, want %v on %q", err, tc.want, tc.field)
			}
		})
	}
}

func TestApplyCustomValuesMergeRules(t *testing.T) {
	stored := raw(map[string]string{"legal_form": `"tnhh"`, "retired_key": `"giữ lại"`, "note": `"cũ"`})
	// Unchanged retired key sent back: accepted. Null removes. Blank text removes. Other keys stay.
	got, err := ApplyCustomValues(schemaDoanhNghiep(), stored, raw(map[string]string{
		"retired_key": `"giữ lại"`, "note": `null`, "staff_count": `7`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if string(got["retired_key"]) != `"giữ lại"` || string(got["legal_form"]) != `"tnhh"` || string(got["staff_count"]) != `7` {
		t.Errorf("merged = %v", got)
	}
	if _, ok := got["note"]; ok {
		t.Error("null did not remove the key")
	}
	if string(stored["note"]) != `"cũ"` {
		t.Error("the stored map was mutated")
	}
	// A CHANGED value under a retired key is refused — the field is gone from the schema.
	if _, err := ApplyCustomValues(schemaDoanhNghiep(), stored, raw(map[string]string{"retired_key": `"mới"`})); !errors.Is(err, ErrCustomValueUnknownField) {
		t.Errorf("changed retired key: %v", err)
	}
	got, err = ApplyCustomValues(schemaDoanhNghiep(), stored, raw(map[string]string{"note": `"   "`}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["note"]; ok {
		t.Error("blank text did not remove the key")
	}
}

func TestCheckRequiredCustomValues(t *testing.T) {
	err := CheckRequiredCustomValues(schemaDoanhNghiep(), raw(map[string]string{"note": `"x"`}))
	var cv *CustomValueError
	if !errors.As(err, &cv) || cv.FieldCode != "legal_form" || !errors.Is(err, ErrCustomValueRequired) {
		t.Fatalf("err = %v", err)
	}
	// A REQUIRED but DISABLED field is not demanded — the form does not show it.
	fields := []MapFieldSchema{{FieldCode: "x", ValueType: ValueTypeText, IsRequired: true, IsActive: false}}
	if err := CheckRequiredCustomValues(fields, nil); err != nil {
		t.Errorf("disabled required field demanded: %v", err)
	}
}

func TestDefaultMapAssetTypesAreADR0072(t *testing.T) {
	d := DefaultMapAssetTypes()
	if len(d) != 11 {
		t.Fatalf("%d groups, ADR 0072 §3 fixes eleven", len(d))
	}
	want := []string{"doanh-nghiep", "ho-kinh-doanh", "hop-tac-xa", "cho", "truong-hoc", "co-so-y-te",
		"di-tich", "du-lich-lang-nghe", "ocop", "ha-tang", "cong-trinh-dau-tu-cong"}
	for i, g := range d {
		if g.Code != want[i] || g.Order != i+1 {
			t.Errorf("row %d = %+v, want code %q order %d", i, g, want[i], i+1)
		}
		// The catalogue's own code rule (ChuanHoaMa) and label rule must accept every row — otherwise
		// the seed writes a row no catalogue route could have written.
		if c, err := ChuanHoaMa(g.Code); err != nil || c != g.Code {
			t.Errorf("%q refused by ChuanHoaMa: %v", g.Code, err)
		}
		if l, err := ChuanHoaNhan(g.Label); err != nil || l != g.Label {
			t.Errorf("%q refused by ChuanHoaNhan: %v", g.Label, err)
		}
	}
	d[0].Code = "changed"
	if DefaultMapAssetTypes()[0].Code != "doanh-nghiep" {
		t.Error("the list is shared state — a caller changed every later run")
	}
}
