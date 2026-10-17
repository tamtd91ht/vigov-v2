package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestTypedAudiencesNormalise(t *testing.T) {
	for _, kind := range []string{NoticeAudienceTypedList, NoticeAudienceImportedFile} {
		a, err := NormalizeNoticeAudience(kind, nil, nil)
		if err != nil || len(a.IDs) != 0 || !a.Typed() || a.RecipientKind() != NoticeRecipientTyped {
			t.Fatalf("%s: %+v %v", kind, a, err)
		}
		if _, err := NormalizeNoticeAudience(kind, []string{"G1"}, nil); !errors.Is(err, ErrNoticeAudience) {
			t.Fatalf("%s with ids accepted", kind)
		}
	}
	if MaxTypedRecipientsFor(NoticeAudienceTypedList) != 1000 || MaxTypedRecipientsFor(NoticeAudienceImportedFile) != 10000 ||
		MaxTypedRecipientsFor(NoticeAudienceCitizenAll) != 0 {
		t.Fatal("S1 limits")
	}
}

func TestValidateTypedSmsSource(t *testing.T) {
	if err := ValidateTypedSmsSource("", "ACC1", "", SmsContentShared); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ oa, acc, tpl, mode string }{
		{"", "", "", SmsContentShared},
		{"OA1", "ACC1", "", SmsContentShared},
		{"", "ACC1", "TPL1", SmsContentShared}, // no template for typed SMS (S3)
		{"", "ACC1", "", ""},
		{"", "ACC1", "", "other"},
	} {
		if err := ValidateTypedSmsSource(c.oa, c.acc, c.tpl, c.mode); !errors.Is(err, ErrNoticeInvalid) {
			t.Errorf("%+v accepted", c)
		}
	}
}

func TestMaskInvalidNoticePhoneAlwaysSatisfiesTheMaskCheck(t *testing.T) {
	cases := map[string]string{
		"":                       "*",
		"abc":                    "*",
		"12345":                  "*****",
		"0900-000":               "*******",
		"09000000000":            "09*****0000",
		"0240000000":             "02****0000",
		"+84 9000 0000 0000 000": "84***********0000",
	}
	for in, want := range cases {
		if got := MaskInvalidNoticePhone(in); got != want {
			t.Errorf("MaskInvalidNoticePhone(%q) = %q, want %q", in, got, want)
		}
	}
	// Whatever is typed, the result passes 0032's CHECK (Go's mirror of it).
	for _, in := range []string{"", "x", "1", "1234567", "12345678", strings.Repeat("9", 200), "0900000000 / 0900000001",
		"tel:0900000000", strings.Repeat("ab12", 50)} {
		if got := MaskInvalidNoticePhone(in); !ValidMaskedPhone(got) {
			t.Errorf("MaskInvalidNoticePhone(%q) = %q breaks the mask CHECK", in, got)
		}
	}
}

func TestScrubProviderMessage(t *testing.T) {
	cases := map[string]string{
		"Template not match":                  "Template not match",
		"  Số 0900000000 bị chặn\n\tthử lại ": "Số *** bị chặn thử lại",
		"Phone 84900000000, req 123456":       "Phone ***, req 123456", // a 6-digit run is not a phone
		"\x00\x01":                            "",
	}
	for in, want := range cases {
		if got := ScrubProviderMessage(in); got != want {
			t.Errorf("ScrubProviderMessage(%q) = %q, want %q", in, got, want)
		}
	}
	long := ScrubProviderMessage(strings.Repeat("lỗi ", 400))
	if n := len([]rune(long)); n > MaxNoticeErrorMessageRunes {
		t.Fatalf("%d runes kept", n)
	}
	if longDigitRun.MatchString(ScrubProviderMessage(strings.Repeat("1", 50))) {
		t.Fatal("a long digit run survived")
	}
}

func TestNoticeVariables(t *testing.T) {
	const c = "Chào {{variable_2}}, hẹn {{variable_1}} lúc {{variable_2}}. {{khac}} {{ variable_3 }}"
	if got := strings.Join(NoticeContentVariables(c), ","); got != "variable_2,variable_1" {
		t.Fatalf("variables = %s", got)
	}
	got := RenderNoticeContent(c, map[string]string{"variable_2": "Bác An", "variable_1": ""}, nil)
	if got != "Chào Bác An, hẹn  lúc Bác An. {{khac}} {{ variable_3 }}" {
		t.Fatalf("render = %q", got)
	}
	if got := RenderNoticeContent(c, map[string]string{"variable_2": "X"}, map[string]bool{"variable_1": true}); !strings.Contains(got, "{{variable_1}}") {
		t.Fatalf("a kept variable was filled: %q", got)
	}
	// An empty cell is provided; a key no row carries is missing.
	rows := []map[string]string{{"variable_2": "a"}, {"variable_2": "", "variable_9": "x"}}
	if m := MissingNoticeVariables(c, rows); len(m) != 1 || m[0] != "variable_1" {
		t.Fatalf("missing = %v", m)
	}
	var e error = &NoticeMissingVariablesError{Missing: []string{"variable_1"}}
	if !errors.Is(e, ErrNoticeInvalid) || !strings.Contains(e.Error(), "variable_1") {
		t.Fatal("missing variables is a 400")
	}
}

func TestNormalizeNoticeMessageTextAndRowValues(t *testing.T) {
	if s, ok := NormalizeNoticeMessageText("  Dòng 1\nDòng 2 "); !ok || s != "Dòng 1\nDòng 2" {
		t.Fatalf("%q %v", s, ok)
	}
	for _, bad := range []string{"", "   ", "a\x00b", strings.Repeat("x", MaxNoticeSmsContentRunes+1)} {
		if _, ok := NormalizeNoticeMessageText(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
	v, err := NormalizeNoticeRowValues(map[string]string{"variable_1": "  a ", "variable_2": ""})
	if err != nil || v["variable_1"] != "a" || v["variable_2"] != "" || len(v) != 2 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := NormalizeNoticeRowValues(map[string]string{"k": "a\nb"}); !errors.Is(err, ErrNoticeRowValues) {
		t.Fatalf("control: %v", err)
	}
}

func TestNormalizeNoticeMapping(t *testing.T) {
	declared := []TemplateParam{{Name: "ten"}, {Name: "ngay"}}
	const shared = "Chào {{variable_1}} {{variable_2}}"
	ok := func(m *NoticeRecipientMapping, ch, mode, content string) *NoticeRecipientMapping {
		t.Helper()
		out, err := NormalizeNoticeMapping(m, NoticeAudienceImportedFile, ch, mode, content, declared)
		if err != nil {
			t.Fatalf("%+v: %v", m, err)
		}
		return out
	}
	if m := ok(&NoticeRecipientMapping{PhoneColumn: " SĐT ", ContentColumn: "Nội dung"}, NoticeChannelSMS, SmsContentPerRow, ""); m.PhoneColumn != "SĐT" {
		t.Fatalf("trim: %+v", m)
	}
	ok(&NoticeRecipientMapping{PhoneColumn: "SĐT", Variables: map[string]string{"variable_1": "Tên", "variable_2": "Ngày"}},
		NoticeChannelSMS, SmsContentShared, shared)
	ok(&NoticeRecipientMapping{PhoneColumn: "SĐT", Params: map[string]string{"ten": "Tên"}}, NoticeChannelZNS, "", "")

	// Not tep-nhap: must be absent.
	if _, err := NormalizeNoticeMapping(&NoticeRecipientMapping{PhoneColumn: "x"}, NoticeAudienceTypedList, NoticeChannelSMS,
		SmsContentShared, shared, nil); !errors.Is(err, ErrNoticeMapping) {
		t.Fatalf("mapping on danh-sach-so: %v", err)
	}
	if m, err := NormalizeNoticeMapping(nil, NoticeAudienceTypedList, NoticeChannelSMS, SmsContentShared, shared, nil); m != nil || err != nil {
		t.Fatal("no mapping on danh-sach-so is fine")
	}
	for name, c := range map[string]struct {
		m             *NoticeRecipientMapping
		ch, mode, txt string
	}{
		"absent":              {nil, NoticeChannelSMS, SmsContentPerRow, ""},
		"no phone column":     {&NoticeRecipientMapping{ContentColumn: "x"}, NoticeChannelSMS, SmsContentPerRow, ""},
		"no content column":   {&NoticeRecipientMapping{PhoneColumn: "p"}, NoticeChannelSMS, SmsContentPerRow, ""},
		"header too long":     {&NoticeRecipientMapping{PhoneColumn: strings.Repeat("x", 101), ContentColumn: "c"}, NoticeChannelSMS, SmsContentPerRow, ""},
		"unused variable":     {&NoticeRecipientMapping{PhoneColumn: "p", Variables: map[string]string{"variable_1": "a", "variable_2": "b", "variable_3": "c"}}, NoticeChannelSMS, SmsContentShared, shared},
		"params on sms":       {&NoticeRecipientMapping{PhoneColumn: "p", Variables: map[string]string{"variable_1": "a", "variable_2": "b"}, Params: map[string]string{"ten": "x"}}, NoticeChannelSMS, SmsContentShared, shared},
		"unknown zns param":   {&NoticeRecipientMapping{PhoneColumn: "p", Params: map[string]string{"khac": "x"}}, NoticeChannelZNS, "", ""},
		"variables on zns":    {&NoticeRecipientMapping{PhoneColumn: "p", Variables: map[string]string{"variable_1": "x"}}, NoticeChannelZNS, "", ""},
		"content col on zns":  {&NoticeRecipientMapping{PhoneColumn: "p", ContentColumn: "c"}, NoticeChannelZNS, "", ""},
		"control in a header": {&NoticeRecipientMapping{PhoneColumn: "p\x01q", ContentColumn: "c"}, NoticeChannelSMS, SmsContentPerRow, ""},
	} {
		if _, err := NormalizeNoticeMapping(c.m, NoticeAudienceImportedFile, c.ch, c.mode, c.txt, declared); !errors.Is(err, ErrNoticeMapping) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A variable of the text without a column is named.
	_, err := NormalizeNoticeMapping(&NoticeRecipientMapping{PhoneColumn: "p", Variables: map[string]string{"variable_1": "a"}},
		NoticeAudienceImportedFile, NoticeChannelSMS, SmsContentShared, shared, nil)
	var missing *NoticeMissingVariablesError
	if !errors.As(err, &missing) || strings.Join(missing.Missing, ",") != "variable_2" {
		t.Fatalf("unmapped variable: %v", err)
	}
}

func TestNoticeRecipientCursor(t *testing.T) {
	c := EncodeNoticeRecipientCursor("01JROW00000000000000000001")
	if id, err := DecodeNoticeRecipientCursor(c); err != nil || id != "01JROW00000000000000000001" {
		t.Fatalf("%q %v", id, err)
	}
	if id, err := DecodeNoticeRecipientCursor(""); err != nil || id != "" {
		t.Fatal("first page")
	}
	for _, bad := range []string{"!!", EncodeNoticeRecipientCursor("a b")} {
		if _, err := DecodeNoticeRecipientCursor(bad); !errors.Is(err, ErrNoticeCursor) {
			t.Errorf("%q accepted", bad)
		}
	}
}
