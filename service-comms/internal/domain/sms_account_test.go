package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSmsAccountLabel(t *testing.T) {
	if got, err := NormalizeSmsAccountLabel("  eSMS chính  "); err != nil || got != "eSMS chính" {
		t.Fatalf("= %q %v", got, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", MaxSmsAccountLabelRunes+1), "a\nb", "a\x00b"} {
		if _, err := NormalizeSmsAccountLabel(bad); !errors.Is(err, ErrSmsAccountInvalid) {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := NormalizeSmsAccountLabel(strings.Repeat("ầ", MaxSmsAccountLabelRunes)); err != nil {
		t.Errorf("100 Vietnamese characters refused: %v", err)
	}
}

// The shape mirrors 0032's sms_account_brandname_shape exactly — a value Go accepts and PostgreSQL
// refuses would surface as a 500 after eSMS was already asked.
func TestNormalizeSmsBrandnameMirrorsTheCheck(t *testing.T) {
	for in, want := range map[string]string{"XaMau": "XaMau", " UBND XA A ": "UBND XA A", "Xa.Mau_1-2": "Xa.Mau_1-2",
		strings.Repeat("A", 32): strings.Repeat("A", 32)} {
		if got, err := NormalizeSmsBrandname(in); err != nil || got != want {
			t.Errorf("%q = %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "-Xa", ".Xa", strings.Repeat("A", 33), "Xã", "Xa/Mau", "Xa\tMau"} {
		if _, err := NormalizeSmsBrandname(bad); !errors.Is(err, ErrSmsBrandname) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidSmsKey(t *testing.T) {
	if !ValidSmsKey([]byte("FAKE-key-NOT-REAL_0123")) {
		t.Fatal("a plain key refused")
	}
	for _, bad := range [][]byte{nil, []byte("has space"), []byte("tab\t"), []byte(strings.Repeat("k", 129)), []byte("khóa")} {
		if ValidSmsKey(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNormalizeSmsRetireReason(t *testing.T) {
	if got, err := NormalizeSmsRetireReason(" Hết hợp đồng "); err != nil || got != "Hết hợp đồng" {
		t.Fatalf("= %q %v", got, err)
	}
	for _, bad := range []string{"", " ", strings.Repeat("a", 201), "a\rb"} {
		if _, err := NormalizeSmsRetireReason(bad); !errors.Is(err, ErrSmsRetireReason) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidateSmsTemplateMirrorsTheChecks(t *testing.T) {
	if err := ValidateSmsTemplate("12640", 2, "Cam on quy khach"); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		id      string
		network int
		content string
	}{
		"empty id":     {"", 1, "x"},
		"id with dot":  {"12.6", 1, "x"},
		"long id":      {strings.Repeat("1", 65), 1, "x"},
		"zero network": {"1", 0, "x"},
		"big network":  {"1", 40000, "x"},
		"blank":        {"1", 1, "  "},
		"long content": {"1", 1, strings.Repeat("a", MaxSmsTemplateContent+1)},
	} {
		if err := ValidateSmsTemplate(tc.id, tc.network, tc.content); !errors.Is(err, ErrSmsTemplateShape) {
			t.Errorf("%s accepted", name)
		}
	}
}

// eSMS's own documented samples: names repeat with different lengths, so slots are positional.
func TestParseSmsTemplateParamsKeepsEverySlotInOrder(t *testing.T) {
	ps, err := ParseSmsTemplateParams("Chuc mung sinh nhat {P2,50}. Xin gui den {P2,50} coupon {P2,20}. Ma {P1, 8}")
	if err != nil {
		t.Fatal(err)
	}
	want := []SmsTemplateParam{{"P2", 50}, {"P2", 50}, {"P2", 20}, {"P1", 8}}
	if len(ps) != len(want) {
		t.Fatalf("= %v", ps)
	}
	for i := range want {
		if ps[i] != want[i] {
			t.Fatalf("[%d] = %v, want %v", i, ps[i], want[i])
		}
	}
	if ps, err := ParseSmsTemplateParams("Cam on quy khach"); err != nil || len(ps) != 0 {
		t.Fatalf("no slot = %v %v", ps, err)
	}
	if _, err := ParseSmsTemplateParams("{P1,0}"); !errors.Is(err, ErrSmsTemplateShape) {
		t.Fatalf("zero length accepted: %v", err)
	}
}

func TestFillSmsTemplate(t *testing.T) {
	const tpl = "Da nhan {P2,20} VND luc {P2,5} cho don {P1,10}."
	got, err := FillSmsTemplate(tpl, []string{" 150000 ", "10h30", "DH-01"})
	if err != nil || got != "Da nhan 150000 VND luc 10h30 cho don DH-01." {
		t.Fatalf("= %q %v", got, err)
	}
	for name, tc := range map[string]struct {
		values []string
		want   error
	}{
		"too few":       {[]string{"1", "2"}, ErrSmsTemplateValueCount},
		"too many":      {[]string{"1", "2", "3", "4"}, ErrSmsTemplateValueCount},
		"empty":         {[]string{"1", " ", "3"}, ErrSmsTemplateValueEmpty},
		"too long":      {[]string{"1", "123456", "3"}, ErrSmsTemplateValueLong},
		"line break":    {[]string{"1", "a\nb", "3"}, ErrSmsTemplateValueShape},
		"slot as value": {[]string{"{P1,9}", "2", "3"}, ErrSmsTemplateValueShape},
	} {
		if _, err := FillSmsTemplate(tpl, tc.values); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
	// Length counts characters, not bytes: five Vietnamese letters fit a 5-slot.
	if _, err := FillSmsTemplate("{P1,5}", []string{"Ầầầầầ"}); err != nil {
		t.Errorf("5 characters refused: %v", err)
	}
	if got, err := FillSmsTemplate("Khong co cho trong", nil); err != nil || got != "Khong co cho trong" {
		t.Errorf("no slot = %q %v", got, err)
	}
}

func TestSmsContentIsUnicode(t *testing.T) {
	if SmsContentIsUnicode("Cam on quy khach") || !SmsContentIsUnicode("Cảm ơn") {
		t.Fatal("wrong")
	}
}

func TestSmsCheckClassesAreTheMigrationsVocabulary(t *testing.T) {
	// 0032 sms_account_check_result_known, verbatim. A value not in this list is refused by PostgreSQL.
	got := strings.Join([]string{SmsCheckOK, SmsCheckKeyRejected, SmsCheckBrandnameInactive, SmsCheckUnavailable,
		SmsCheckRejected, SmsCheckMalformed}, ",")
	if got != "thanh-cong,sai-khoa,brandname-chua-kich-hoat,khong-kha-dung,bi-tu-choi,phan-hoi-sai-dang" {
		t.Fatal(got)
	}
	if (SmsAccount{ID: "01J"}).Subject() != "sms-account/01J" {
		t.Fatal("subject")
	}
}
