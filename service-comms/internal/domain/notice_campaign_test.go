package domain

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNoticeFailureClassesFit0032sShapeCheck(t *testing.T) {
	shape := regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)
	seen := map[string]bool{}
	for _, c := range NoticeFailureClasses() {
		if !shape.MatchString(c) || len(c) > 40 {
			t.Errorf("%q is outside notice_recipient_failure_class_shape", c)
		}
		if seen[c] {
			t.Errorf("%q listed twice", c)
		}
		seen[c] = true
	}
	if len(seen) != 11 {
		t.Errorf("%d classes", len(seen))
	}
}

func TestValidateNoticeSource(t *testing.T) {
	cases := []struct {
		ch, oa, acc, tpl string
		want             error
	}{
		{NoticeChannelZNS, "OA1", "", "1001", nil},
		{NoticeChannelSMS, "", "ACC1", "TPL1", nil},
		{NoticeChannelZNS, "", "ACC1", "1001", ErrNoticeSource},
		{NoticeChannelZNS, "OA1", "ACC1", "1001", ErrNoticeSource},
		{NoticeChannelSMS, "OA1", "", "TPL1", ErrNoticeSource},
		{"email", "OA1", "", "1001", ErrNoticeChannel},
		{NoticeChannelZNS, "OA1", "", "", ErrNoticeTemplate},
		{NoticeChannelZNS, "OA1", "", " 1001", ErrNoticeTemplate},
		{NoticeChannelZNS, "OA 1", "", "1001", ErrNoticeSource},
	}
	for _, c := range cases {
		if err := ValidateNoticeSource(c.ch, c.oa, c.acc, c.tpl); !errors.Is(err, c.want) && !(c.want == nil && err == nil) {
			t.Errorf("%+v: %v, want %v", c, err, c.want)
		}
	}
}

func TestNormalizeNoticeAudience(t *testing.T) {
	a, err := NormalizeNoticeAudience(NoticeAudienceCitizenAll, nil, nil)
	if err != nil || len(a.IDs) != 0 || a.IDs == nil || a.RecipientKind() != NoticeRecipientCitizen {
		t.Fatalf("all: %+v %v", a, err)
	}
	a, err = NormalizeNoticeAudience(NoticeAudienceCitizenTag, nil, []string{" hộ nghèo ", "hộ nghèo", "cựu chiến binh"})
	if err != nil || len(a.IDs) != 2 || a.IDs[0] != "hộ nghèo" {
		t.Fatalf("tags: %+v %v", a, err)
	}
	a, err = NormalizeNoticeAudience(NoticeAudienceStaffUnit, []string{"U1"}, nil)
	if err != nil || a.RecipientKind() != NoticeRecipientStaff {
		t.Fatalf("unit: %+v %v", a, err)
	}
	many := make([]string, MaxNoticeAudienceIDs+1)
	for i := range many {
		many[i] = "G" + SmsSlotKey(i)
	}
	for _, bad := range []struct {
		kind      string
		ids, tags []string
	}{
		{NoticeAudienceCitizenAll, []string{"G1"}, nil},
		{NoticeAudienceCitizenGroup, nil, nil},
		{NoticeAudienceCitizenGroup, []string{"G1"}, []string{"x"}},
		{NoticeAudienceCitizenTag, []string{"G1"}, nil},
		{NoticeAudienceCitizenGroup, []string{" "}, nil},
		{NoticeAudienceCitizenGroup, many, nil},
		{"everyone", nil, nil},
	} {
		if _, err := NormalizeNoticeAudience(bad.kind, bad.ids, bad.tags); !errors.Is(err, ErrNoticeAudience) {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
}

func TestPreviewSmsContentKeepsMissingSlotsAndChecksLength(t *testing.T) {
	const tpl = "Xa thong bao {P1,10} luc {P2,5}, ma {P2,8}"
	got, missing, err := PreviewSmsContent(tpl, map[string]string{"1": "hop dan"})
	if err != nil || got != "Xa thong bao hop dan luc {P2,5}, ma {P2,8}" || strings.Join(missing, ",") != "2,3" {
		t.Fatalf("%q %v %v", got, missing, err)
	}
	got, missing, err = PreviewSmsContent(tpl, map[string]string{"1": "hop dan", "2": "8h", "3": "AB12"})
	if err != nil || got != "Xa thong bao hop dan luc 8h, ma AB12" || len(missing) != 0 {
		t.Fatalf("%q %v %v", got, missing, err)
	}
	// The SAME name, different lengths: each slot is its own value (domain.SmsTemplateParam).
	if _, _, err := PreviewSmsContent(tpl, map[string]string{"2": "123456"}); !errors.Is(err, ErrNoticeParamLong) {
		t.Fatalf("slot 2 allows 5: %v", err)
	}
	if _, _, err := PreviewSmsContent(tpl, map[string]string{"1": "{P1,10}"}); !errors.Is(err, ErrNoticeParams) {
		t.Fatalf("a value carrying a slot: %v", err)
	}
	vals, _ := SmsSlotValues(tpl, map[string]string{"1": "a", "2": "b", "3": "c"})
	if filled, err := FillSmsTemplate(tpl, vals); err != nil || filled != "Xa thong bao a luc b, ma c" {
		t.Fatalf("fill %q %v", filled, err)
	}
	if d := SmsTemplateData(tpl, map[string]string{"1": "a", "9": "stray"}); len(d) != 1 || d["1"] != "a" {
		t.Fatalf("data %v", d)
	}
}

func TestZnsParams(t *testing.T) {
	declared := []TemplateParam{{Name: "ten_su_kien", MaxLength: 10}, {Name: "thoi_gian"}}
	missing, err := MissingZnsParams(declared, map[string]string{"ten_su_kien": "Hop dan"})
	if err != nil || len(missing) != 1 || missing[0] != "thoi_gian" {
		t.Fatalf("%v %v", missing, err)
	}
	if _, err := MissingZnsParams(declared, map[string]string{"ten_su_kien": "qua dai hon muoi"}); !errors.Is(err, ErrNoticeParamLong) {
		t.Fatalf("length: %v", err)
	}
	if d := ZnsTemplateData(declared, map[string]string{"ten_su_kien": "a", "khac": "b"}); len(d) != 1 {
		t.Fatalf("stray key kept: %v", d)
	}
}

func TestNormalizeNoticeParamsDropsEmptyAndRefusesControl(t *testing.T) {
	p, err := NormalizeNoticeParams(map[string]string{"a": "  x ", "b": "   "})
	if err != nil || len(p) != 1 || p["a"] != "x" {
		t.Fatalf("%v %v", p, err)
	}
	if _, err := NormalizeNoticeParams(map[string]string{"a": "x\ny"}); !errors.Is(err, ErrNoticeParams) {
		t.Fatalf("control: %v", err)
	}
}

func TestMoneyAndEstimates(t *testing.T) {
	if ZnsUnitPrice("TRANSACTION") != "200.00" || ZnsUnitPrice("CUSTOMER_CARE") != "200.00" ||
		ZnsUnitPrice("otp") != "300.00" || ZnsUnitPrice("") != "200.00" {
		t.Fatal("ZNS table")
	}
	for in, want := range map[string]int64{"350": 35000, "350.5": 35050, "0.25": 25, "12.500": 1250} {
		if got, ok := ParseMoney(in); !ok || got != want {
			t.Errorf("%q → %d %v", in, got, ok)
		}
	}
	for _, bad := range []string{"-1", "1.005", "abc", ""} {
		if _, ok := ParseMoney(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
	unit, cost, ok := EstimateCost("350", 1234)
	if !ok || unit != "350.00" || cost != "431900.00" {
		t.Fatalf("%s %s %v", unit, cost, ok)
	}
	if _, _, ok := EstimateCost("999999999999", 1000000); ok {
		t.Fatal("overflow accepted")
	}
}

func TestValidMaskedPhone(t *testing.T) {
	for _, good := range []string{"09****0000", "09*******0"} {
		if !ValidMaskedPhone(good) {
			t.Errorf("%q refused", good)
		}
	}
	for _, bad := range []string{"0900000000", "0900*000000", "", "09**0000 ab"} {
		if ValidMaskedPhone(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCampaignOutcomeAndBackoff(t *testing.T) {
	cases := []struct {
		total, pending, sent, invalid int
		want                          string
	}{
		{10, 1, 9, 0, NoticeCampaignSending},
		{10, 0, 10, 0, NoticeCampaignSent},
		{10, 0, 0, 0, NoticeCampaignFailed},
		{10, 0, 4, 0, NoticeCampaignPartlySent},
		// Every valid row sent, but some typed numbers could not be read: not "all sent" (S5).
		{10, 0, 10, 2, NoticeCampaignPartlySent},
		{10, 0, 0, 2, NoticeCampaignFailed},
	}
	for _, c := range cases {
		if got := NoticeCampaignOutcome(c.total, c.pending, c.sent, c.invalid); got != c.want {
			t.Errorf("%+v → %s", c, got)
		}
	}
	if NoticeRetryBackoff(1) != time.Minute || NoticeRetryBackoff(2) != 5*time.Minute || NoticeRetryBackoff(4) != 15*time.Minute {
		t.Fatal("backoff")
	}
	if DuplicateRequestMessageID("01J") != "request:01J" {
		t.Fatal("dup id")
	}
}

func TestNoticeCursorRoundTrip(t *testing.T) {
	c := NoticeCursor{CreatedAt: time.Date(2026, 10, 17, 3, 4, 5, 123456000, time.UTC), ID: "01JCAMPAIGN0000000000000001"}
	got, err := DecodeNoticeCursor(c.Encode())
	if err != nil || !got.CreatedAt.Equal(c.CreatedAt) || got.ID != c.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if z, err := DecodeNoticeCursor(""); err != nil || !z.CreatedAt.IsZero() {
		t.Fatal("empty cursor")
	}
	for _, bad := range []string{"!!", "bm9kb3Q", "LTEuWA"} {
		if _, err := DecodeNoticeCursor(bad); !errors.Is(err, ErrNoticeCursor) {
			t.Errorf("%q accepted", bad)
		}
	}
}
