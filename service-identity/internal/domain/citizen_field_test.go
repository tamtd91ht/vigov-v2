package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func optionField() CitizenField {
	return CitizenField{Code: "hongheo1", Label: "Phân loại hộ", Type: FieldCombobox, Options: []CitizenFieldOption{
		{Value: "ho-ngheo", Label: "Hộ nghèo"}, {Value: "can-ngheo", Label: "Cận nghèo"}, {Value: "cu", Label: "Cũ", Retired: true},
	}}
}

func TestCheckNewCitizenFieldRules(t *testing.T) {
	ok := CitizenFieldInput{Code: "ngaysinh", Label: " Ngày  sinh ", Type: FieldDateTime, DatePattern: "dd/MM/yyyy", LayoutWidth: LayoutHalf}
	got, err := CheckNewCitizenField(ok)
	if err != nil || got.Label != "Ngày sinh" {
		t.Fatalf("valid field refused or not normalised: %+v %v", got, err)
	}
	cases := []struct {
		name string
		in   CitizenFieldInput
		want error
	}{
		{"code too short", CitizenFieldInput{Code: "abcdef1", Label: "x", Type: FieldSingleText}, ErrCitizenFieldCodeInvalid},
		{"code digit first", CitizenFieldInput{Code: "1abcdefg", Label: "x", Type: FieldSingleText}, ErrCitizenFieldCodeInvalid},
		{"code upper", CitizenFieldInput{Code: "Abcdefgh", Label: "x", Type: FieldSingleText}, ErrCitizenFieldCodeInvalid},
		{"code thirteen", CitizenFieldInput{Code: "abcdefghijklm", Label: "x", Type: FieldSingleText}, ErrCitizenFieldCodeInvalid},
		{"type", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: "text"}, ErrCitizenFieldTypeInvalid},
		{"no label", CitizenFieldInput{Code: "abcdefgh", Label: " ", Type: FieldSingleText}, ErrCitizenFieldLabelMissing},
		{"number without kind", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldNumber}, ErrCitizenFieldNumberKind},
		{"kind on text", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldSingleText, NumberKind: NumberInteger}, ErrCitizenFieldNumberKind},
		{"date pattern off list", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldDateTime, DatePattern: "yyyy-MM-dd"}, ErrCitizenFieldDatePattern},
		{"date pattern on number", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldNumber, NumberKind: NumberDecimal, DatePattern: "yyyy"}, ErrCitizenFieldDatePattern},
		{"combobox without options", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldCombobox}, ErrCitizenFieldOptionsMissing},
		{"options on text", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldSingleText, Options: []CitizenFieldOption{{Value: "a", Label: "A"}}}, ErrCitizenFieldOptionsMissing},
		{"option shape", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldSingleBox, Options: []CitizenFieldOption{{Value: "Hộ nghèo", Label: "A"}}}, ErrCitizenFieldOptionInvalid},
		{"option dup", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldSingleBox, Options: []CitizenFieldOption{{Value: "a", Label: "A"}, {Value: "a", Label: "B"}}}, ErrCitizenFieldOptionDuplicate},
		{"placeholder on number", CitizenFieldInput{Code: "abcdefgh", Label: "x", Type: FieldNumber, NumberKind: NumberInteger, Placeholder: "nhập"}, ErrCitizenFieldPlaceholder},
	}
	for _, c := range cases {
		if _, err := CheckNewCitizenField(c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

// Display mode (owner 17/10/2026): full|dropdown exactly for the two choice types, "" for the rest, never
// defaulted here. Layout width: half|full for every type, never defaulted here.
func TestCitizenFieldDisplayModeAndLayoutWidth(t *testing.T) {
	opts := []CitizenFieldOption{{Value: "a", Label: "A"}}
	for _, c := range []struct {
		name string
		in   CitizenFieldInput
		want error
	}{
		{"combobox full", CitizenFieldInput{Type: FieldCombobox, Options: opts, DisplayMode: DisplayFull, LayoutWidth: LayoutHalf}, nil},
		{"singleBox dropdown", CitizenFieldInput{Type: FieldSingleBox, Options: opts, DisplayMode: DisplayDropdown, LayoutWidth: LayoutFull}, nil},
		{"text no mode, full width", CitizenFieldInput{Type: FieldMultiText, LayoutWidth: LayoutFull}, nil},
		{"combobox without mode", CitizenFieldInput{Type: FieldCombobox, Options: opts, LayoutWidth: LayoutHalf}, ErrCitizenFieldDisplayMode},
		{"singleBox unknown mode", CitizenFieldInput{Type: FieldSingleBox, Options: opts, DisplayMode: "list", LayoutWidth: LayoutHalf}, ErrCitizenFieldDisplayMode},
		{"mode on text", CitizenFieldInput{Type: FieldSingleText, DisplayMode: DisplayFull, LayoutWidth: LayoutHalf}, ErrCitizenFieldDisplayMode},
		{"mode on number", CitizenFieldInput{Type: FieldNumber, NumberKind: NumberInteger, DisplayMode: DisplayDropdown, LayoutWidth: LayoutHalf}, ErrCitizenFieldDisplayMode},
		{"no width", CitizenFieldInput{Type: FieldSingleText}, ErrCitizenFieldLayoutWidth},
		{"unknown width", CitizenFieldInput{Type: FieldSingleText, LayoutWidth: "third"}, ErrCitizenFieldLayoutWidth},
	} {
		c.in.Code, c.in.Label = "abcdefgh", "x"
		if _, err := CheckNewCitizenField(c.in); !errors.Is(err, c.want) && !(c.want == nil && err == nil) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}

	cur := optionField()
	cur.DisplayMode, cur.LayoutWidth = DisplayFull, LayoutHalf
	dd, full := DisplayDropdown, LayoutFull
	got, changed, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{DisplayMode: &dd, LayoutWidth: &full})
	if err != nil || got.DisplayMode != DisplayDropdown || got.LayoutWidth != LayoutFull || strings.Join(changed, ",") != "display_mode,layout_width" {
		t.Errorf("patch mode+width: %+v %v %v", got, changed, err)
	}
	empty, bad := "", "third"
	if _, _, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{DisplayMode: &empty}); !errors.Is(err, ErrCitizenFieldDisplayMode) {
		t.Errorf("clearing a combobox mode: %v", err)
	}
	if _, _, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{LayoutWidth: &bad}); !errors.Is(err, ErrCitizenFieldLayoutWidth) {
		t.Errorf("unknown width: %v", err)
	}
	text := CitizenField{Code: "ghichu01", Label: "Ghi chú", Type: FieldSingleText, LayoutWidth: LayoutHalf}
	if _, _, err := ApplyCitizenFieldPatch(text, CitizenFieldPatch{DisplayMode: &dd}); !errors.Is(err, ErrCitizenFieldDisplayMode) {
		t.Errorf("mode on text by patch: %v", err)
	}
	if _, changed, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{}); err != nil || len(changed) != 0 {
		t.Errorf("absent = unchanged: %v %v", changed, err)
	}
}

func TestPatchNeverRemovesAnOptionAndTypeIsImmutable(t *testing.T) {
	cur := optionField()
	typ := FieldSingleBox
	if _, _, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{Type: &typ}); !errors.Is(err, ErrCitizenFieldImmutable) {
		t.Errorf("type change: %v", err)
	}
	removed := []CitizenFieldOption{{Value: "ho-ngheo", Label: "Hộ nghèo"}, {Value: "can-ngheo", Label: "Cận nghèo"}}
	if _, _, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{Options: &removed}); !errors.Is(err, ErrCitizenFieldOptionRemoved) {
		t.Errorf("option removed: %v", err)
	}
	next := []CitizenFieldOption{{Value: "ho-ngheo", Label: "Hộ nghèo (2026)", Retired: true}, {Value: "can-ngheo", Label: "Cận nghèo"},
		{Value: "cu", Label: "Cũ", Retired: true}, {Value: "moi", Label: "Mới"}}
	label := "Phân loại hộ gia đình"
	got, changed, err := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{Options: &next, Label: &label})
	if err != nil || len(got.Options) != 4 || !got.Options[0].Retired || strings.Join(changed, ",") != "label,options" {
		t.Errorf("relabel/retire/add: %+v %v %v", got.Options, changed, err)
	}
	if _, changed, _ := ApplyCitizenFieldPatch(cur, CitizenFieldPatch{Label: &cur.Label}); len(changed) != 0 {
		t.Errorf("a no-op patch reports %v", changed)
	}
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestCheckCitizenAttributesByType(t *testing.T) {
	now := time.Now()
	defs := []CitizenField{
		{Code: "ghichu01", Label: "Ghi chú ngắn", Type: FieldSingleText},
		{Code: "motaxxx1", Label: "Mô tả", Type: FieldMultiText},
		optionField(),
		{Code: "tongiao1", Label: "Nhóm", Type: FieldSingleBox, Options: []CitizenFieldOption{{Value: "a", Label: "A"}, {Value: "b", Label: "B", Retired: true}}},
		{Code: "sonhan01", Label: "Số nhân khẩu", Type: FieldNumber, NumberKind: NumberInteger, Required: true},
		{Code: "dientich", Label: "Diện tích", Type: FieldNumber, NumberKind: NumberDecimal},
		{Code: "ngaysinh", Label: "Ngày sinh", Type: FieldDateTime, DatePattern: "dd/MM/yyyy"},
		{Code: "cuxxxxx1", Label: "Cũ", Type: FieldSingleText, RetiredAt: &now},
	}
	stored := map[string]json.RawMessage{"cuxxxxx1": raw(`"giữ nguyên"`), "tongiao1": raw(`"b"`), "khongcon": raw(`1`)}
	in := map[string]json.RawMessage{
		"ghichu01": raw(`"  một dòng "`), "motaxxx1": raw(`"dòng 1\ndòng 2"`),
		"hongheo1": raw(`["can-ngheo","ho-ngheo"]`), "tongiao1": raw(`"b"`),
		"sonhan01": raw(`4`), "dientich": raw(`0120.50`), "ngaysinh": raw(`-315619200000`),
	}
	got, err := CheckCitizenAttributes(defs, in, stored)
	if err == nil {
		// 0120.50 is not valid JSON; the decoder must refuse it.
		t.Fatalf("a leading-zero number must be refused: %v", got)
	}
	in["dientich"] = raw(`120.50`)
	got, err = CheckCitizenAttributes(defs, in, stored)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ghichu01": `"một dòng"`, "hongheo1": `["ho-ngheo","can-ngheo"]`, "tongiao1": `"b"`,
		"sonhan01": `4`, "dientich": `120.5`, "ngaysinh": `-315619200000`,
		"cuxxxxx1": `"giữ nguyên"`, "khongcon": `1`, // retired / undefined codes carried over untouched
	}
	for k, v := range want {
		if string(got[k]) != v {
			t.Errorf("%s = %s, want %s", k, got[k], v)
		}
	}

	refused := []struct {
		name string
		in   map[string]json.RawMessage
		want error
	}{
		{"unknown code", map[string]json.RawMessage{"sonhan01": raw(`1`), "zzzzzz": raw(`"x"`)}, ErrCitizenAttributeUnknown},
		{"required missing", map[string]json.RawMessage{}, ErrCitizenAttributeRequired},
		{"integer with decimals", map[string]json.RawMessage{"sonhan01": raw(`1.5`)}, ErrCitizenAttributeInvalid},
		{"decimal 5 dp", map[string]json.RawMessage{"sonhan01": raw(`1`), "dientich": raw(`1.23456`)}, ErrCitizenAttributeInvalid},
		{"number as string", map[string]json.RawMessage{"sonhan01": raw(`"1"`)}, ErrCitizenAttributeInvalid},
		{"single line", map[string]json.RawMessage{"sonhan01": raw(`1`), "ghichu01": raw(`"a\nb"`)}, ErrCitizenAttributeInvalid},
		{"text too long", map[string]json.RawMessage{"sonhan01": raw(`1`), "ghichu01": raw(`"` + strings.Repeat("a", 256) + `"`)}, ErrCitizenAttributeInvalid},
		{"combobox dup", map[string]json.RawMessage{"sonhan01": raw(`1`), "hongheo1": raw(`["ho-ngheo","ho-ngheo"]`)}, ErrCitizenAttributeInvalid},
		{"combobox retired not held", map[string]json.RawMessage{"sonhan01": raw(`1`), "hongheo1": raw(`["cu"]`)}, ErrCitizenAttributeInvalid},
		{"singlebox unknown", map[string]json.RawMessage{"sonhan01": raw(`1`), "tongiao1": raw(`"z"`)}, ErrCitizenAttributeInvalid},
		{"date as string", map[string]json.RawMessage{"sonhan01": raw(`1`), "ngaysinh": raw(`"01/01/1960"`)}, ErrCitizenAttributeInvalid},
		{"date out of range", map[string]json.RawMessage{"sonhan01": raw(`1`), "ngaysinh": raw(`9000000000000000`)}, ErrCitizenAttributeInvalid},
	}
	for _, c := range refused {
		_, err := CheckCitizenAttributes(defs, c.in, nil)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
		// The sentence names the field, never the value typed.
		if err != nil && strings.Contains(err.Error(), "01/01/1960") {
			t.Errorf("%s: the refusal quotes the value: %v", c.name, err)
		}
	}
	// A retired FIELD re-sent by the form is ignored and its stored value kept; absent, it is kept too.
	for _, in := range []map[string]json.RawMessage{
		{"sonhan01": raw(`1`), "cuxxxxx1": raw(`"đổi"`)}, {"sonhan01": raw(`1`)},
	} {
		got, err := CheckCitizenAttributes(defs, in, stored)
		if err != nil || string(got["cuxxxxx1"]) != `"giữ nguyên"` {
			t.Errorf("retired value not kept: %s, %v", got["cuxxxxx1"], err)
		}
	}
	// A retired option the person already had stays acceptable (tongiao1 "b" was stored).
	if _, err := CheckCitizenAttributes(defs, map[string]json.RawMessage{"sonhan01": raw(`1`), "tongiao1": raw(`"b"`)}, stored); err != nil {
		t.Errorf("a retired option already held must be kept: %v", err)
	}
	if live := LiveAttributes(defs, got); live["cuxxxxx1"] != nil || live["khongcon"] != nil || live["sonhan01"] == nil {
		t.Errorf("LiveAttributes = %v", live)
	}
}

func TestNormalizeCitizenContactEmail(t *testing.T) {
	if e, err := NormalizeCitizenContactEmail("  Ten.Nguoi@Example.VN "); err != nil || e != "ten.nguoi@example.vn" {
		t.Errorf("= %q %v", e, err)
	}
	if e, err := NormalizeCitizenContactEmail(""); err != nil || e != "" {
		t.Errorf("empty = %q %v", e, err)
	}
	for _, bad := range []string{"khongcoacong", "a@b", "@b.vn", "a@b@c.vn", "a b@c.vn", "a@.vn", "a@vn.", strings.Repeat("a", 250) + "@b.vn"} {
		if _, err := NormalizeCitizenContactEmail(bad); !errors.Is(err, ErrCitizenContactEmail) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCheckCitizenFieldOrder(t *testing.T) {
	live := []CitizenField{{Code: "aaaaaa"}, {Code: "bbbbbb"}}
	if err := CheckCitizenFieldOrder(live, []string{"bbbbbb", "aaaaaa"}); err != nil {
		t.Error(err)
	}
	for _, bad := range [][]string{{"aaaaaa"}, {"aaaaaa", "aaaaaa"}, {"aaaaaa", "cccccc"}} {
		if err := CheckCitizenFieldOrder(live, bad); !errors.Is(err, ErrCitizenFieldOrderMismatch) {
			t.Errorf("%v accepted", bad)
		}
	}
}
