package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// FormatCitizenAttributeValue per type (ADR 0101 #12): what a recipient's stored value becomes in a message.
// Every expected text is written out, so a mutation of the label lookup, the join, the date layout or the
// zone turns a case red.

var ict = time.FixedZone("ICT-test", 7*3600)

func TestFormatCitizenAttributeValuePerType(t *testing.T) {
	opts := []CitizenFieldOption{
		{Value: "ho-ngheo", Label: "Hộ nghèo"},
		{Value: "can-ngheo", Label: "Cận nghèo"},
		{Value: "cu", Label: "Diện cũ", Retired: true}, // retired: the label still resolves
	}
	// 2026-10-16T17:30:00Z = 17/10/2026 00:30 in Asia/Ho_Chi_Minh — a date that differs from UTC's.
	ms := time.Date(2026, 10, 16, 17, 30, 0, 0, time.UTC).UnixMilli()
	msText := json.RawMessage(jsonInt(ms))
	for _, c := range []struct {
		name string
		def  CitizenField
		raw  string
		want string
	}{
		{"singleText trimmed", CitizenField{Type: FieldSingleText}, `"  Thôn Đông  "`, "Thôn Đông"},
		{"multiText as is", CitizenField{Type: FieldMultiText}, `"dòng 1\ndòng 2"`, "dòng 1\ndòng 2"},
		{"singleBox label", CitizenField{Type: FieldSingleBox, Options: opts}, `"can-ngheo"`, "Cận nghèo"},
		{"singleBox retired option", CitizenField{Type: FieldSingleBox, Options: opts}, `"cu"`, "Diện cũ"},
		{"singleBox unknown option", CitizenField{Type: FieldSingleBox, Options: opts}, `"khong-co"`, ""},
		{"combobox labels joined in stored order", CitizenField{Type: FieldCombobox, Options: opts}, `["can-ngheo","cu","ho-ngheo"]`, "Cận nghèo, Diện cũ, Hộ nghèo"},
		{"number integer", CitizenField{Type: FieldNumber, NumberKind: NumberInteger}, `42`, "42"},
		{"number decimal kept as stored", CitizenField{Type: FieldNumber, NumberKind: NumberDecimal}, `7.25`, "7.25"},
		{"null", CitizenField{Type: FieldSingleText}, `null`, ""},
		{"wrong JSON type", CitizenField{Type: FieldSingleText}, `12`, ""},
	} {
		if got := FormatCitizenAttributeValue(c.def, json.RawMessage(c.raw), ict); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	for pattern, want := range map[string]string{
		"dd/MM/yyyy":       "17/10/2026",
		"dd/MM/yyyy HH:mm": "17/10/2026 00:30",
		"MM/yyyy":          "10/2026",
		"yyyy":             "2026",
		"":                 "17/10/2026",
	} {
		got := FormatCitizenAttributeValue(CitizenField{Type: FieldDateTime, DatePattern: pattern}, msText, ict)
		if got != want {
			t.Errorf("dateTime %q: got %q, want %q", pattern, got, want)
		}
	}
	if got := FormatCitizenAttributeValue(CitizenField{Type: FieldDateTime}, json.RawMessage(`"x"`), ict); got != "" {
		t.Errorf("unreadable dateTime = %q, want empty", got)
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

// The REAL zone: the deployment formats in Asia/Ho_Chi_Minh, read through MuiGio — not the process's zone.
func TestFormatCitizenAttributeDateUsesTheAdministrativeZone(t *testing.T) {
	loc, err := MuiGio()
	if err != nil {
		t.Skipf("zone database unavailable: %v", err)
	}
	ms := time.Date(2026, 10, 16, 17, 30, 0, 0, time.UTC).UnixMilli()
	got := FormatCitizenAttributeValue(CitizenField{Type: FieldDateTime, DatePattern: "dd/MM/yyyy"}, json.RawMessage(jsonInt(ms)), loc)
	if got != "17/10/2026" {
		t.Errorf("got %q, want 17/10/2026 (UTC would say 16/10)", got)
	}
}

func TestFormatCitizenAttributeValueCutsAt500(t *testing.T) {
	long, _ := json.Marshal(strings.Repeat("á", 600))
	got := FormatCitizenAttributeValue(CitizenField{Type: FieldMultiText}, long, ict)
	if n := len([]rune(got)); n != MaxNoticeAttributeValue {
		t.Errorf("cut to %d characters, want %d", n, MaxNoticeAttributeValue)
	}
}

func TestFormatNoticeAttributesOnePerCode(t *testing.T) {
	defs := map[string]CitizenField{"hongheo": {Code: "hongheo", Type: FieldSingleBox,
		Options: []CitizenFieldOption{{Value: "ho-ngheo", Label: "Hộ nghèo"}}}}
	src := NoticeAttributeSource{
		Static:  map[string]string{"full_name": " Nguyễn Văn A ", "address": "Thôn 1"},
		Dynamic: map[string]json.RawMessage{"hongheo": json.RawMessage(`"ho-ngheo"`), "khac01": json.RawMessage(`"x"`)},
	}
	got := FormatNoticeAttributes([]string{"full_name", "hongheo", "email"}, defs, src, ict)
	if len(got) != 3 || got["full_name"] != "Nguyễn Văn A" || got["hongheo"] != "Hộ nghèo" || got["email"] != "" {
		t.Errorf("got %v — want exactly the three codes, absent email empty", got)
	}
}

func TestCheckNoticeAttributeCodes(t *testing.T) {
	gone := time.Now()
	defs := []CitizenField{{Code: "hongheo"}, {Code: "cucu01", RetiredAt: &gone}}
	got, err := CheckNoticeAttributeCodes(NoticeRecipientCitizen, []string{"full_name", "", "cucu01", "full_name", "hongheo"}, defs)
	if err != nil || strings.Join(got, ",") != "full_name,cucu01,hongheo" {
		t.Errorf("got %v, %v — want collapsed, retired accepted", got, err)
	}
	for name, c := range map[string]struct {
		kind  NoticeRecipientKind
		codes []string
	}{
		"unknown":         {NoticeRecipientCitizen, []string{"khongco"}},
		"staff address":   {NoticeRecipientStaff, []string{"address"}},
		"staff dynamic":   {NoticeRecipientStaff, []string{"hongheo"}},
		"over the bound":  {NoticeRecipientCitizen, make([]string, MaxNoticeAttributeCodes+1)},
		"case sensitive":  {NoticeRecipientCitizen, []string{"Full_Name"}},
		"padded is a new": {NoticeRecipientCitizen, []string{" full_name"}},
	} {
		if _, err := CheckNoticeAttributeCodes(c.kind, c.codes, defs); !errors.Is(err, ErrNoticeAttributeInvalid) {
			t.Errorf("%s: err = %v, want ErrNoticeAttributeInvalid", name, err)
		}
	}
	if got, err := CheckNoticeAttributeCodes(NoticeRecipientStaff, []string{"full_name"}, nil); err != nil || len(got) != 1 {
		t.Errorf("staff full_name: %v, %v", got, err)
	}
}

// A dynamic field whose code equals a static one is not listed: the static attribute owns the code.
func TestNoticeAttributesForStaticOwnsItsCode(t *testing.T) {
	gone := time.Now()
	list := NoticeAttributesFor(NoticeRecipientCitizen, []CitizenField{
		{Code: "address", Label: "Địa chỉ cũ"}, {Code: "hongheo", Label: "Diện hộ"}, {Code: "cucu01", RetiredAt: &gone},
	})
	var codes []string
	for _, a := range list {
		codes = append(codes, a.Code+"="+a.Label)
	}
	if strings.Join(codes, ",") != "full_name=Họ và tên,address=Địa chỉ,email=Email,household_code=Mã hộ,hongheo=Diện hộ" {
		t.Errorf("list = %v", codes)
	}
}
