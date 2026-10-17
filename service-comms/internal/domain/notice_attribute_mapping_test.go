package domain

import (
	"errors"
	"strings"
	"testing"
)

// The attribute-mapping rules (ADR 0101 #12): the shape of a mapping per audience, the fitting of a value
// to its slot, and the per-recipient rendering for SMS and ZNS.

func TestNormalizeNoticeAttributeMapping(t *testing.T) {
	citizens := NoticeAudience{Kind: NoticeAudienceCitizenAll}
	staff := NoticeAudience{Kind: NoticeAudienceStaffUnit, IDs: []string{"U1"}}
	if m, err := NormalizeNoticeAttributeMapping(nil, citizens); m != nil || err != nil {
		t.Errorf("nil = %v, %v", m, err)
	}
	if m, err := NormalizeNoticeAttributeMapping(map[string]string{"1": "full_name", "su_kien": "hongheo"}, citizens); err != nil || len(m) != 2 {
		t.Errorf("valid = %v, %v", m, err)
	}
	if _, err := NormalizeNoticeAttributeMapping(map[string]string{"1": "full_name"}, staff); err != nil {
		t.Errorf("staff full_name: %v", err)
	}
	over := map[string]string{}
	for i := 0; i <= MaxNoticeAttributeMappings; i++ {
		over[SmsSlotKey(i)] = "full_name"
	}
	for name, c := range map[string]struct {
		m    map[string]string
		a    NoticeAudience
		want error
	}{
		"typed list":      {map[string]string{"1": "full_name"}, NoticeAudience{Kind: NoticeAudienceTypedList}, ErrNoticeAttributeMapping},
		"imported file":   {map[string]string{"1": "full_name"}, NoticeAudience{Kind: NoticeAudienceImportedFile}, ErrNoticeAttributeMapping},
		"21 entries":      {over, citizens, ErrNoticeAttributeMapping},
		"empty key":       {map[string]string{"": "full_name"}, citizens, ErrNoticeAttributeMapping},
		"value not code":  {map[string]string{"1": "Nguyễn Văn A"}, citizens, ErrNoticeAttributeMapping},
		"upper-case code": {map[string]string{"1": "Full_name"}, citizens, ErrNoticeAttributeMapping},
		"staff address":   {map[string]string{"1": "address"}, staff, ErrNoticeAttributeStaff},
		"staff dynamic":   {map[string]string{"1": "hongheo"}, NoticeAudience{Kind: NoticeAudienceStaffAll}, ErrNoticeAttributeStaff},
	} {
		if _, err := NormalizeNoticeAttributeMapping(c.m, c.a); !errors.Is(err, c.want) || !errors.Is(err, ErrNoticeInvalid) {
			t.Errorf("%s: err = %v, want %v (a 400)", name, err, c.want)
		}
	}
}

func TestNoticeAttributeCodesDistinctSorted(t *testing.T) {
	got := NoticeAttributeCodes(map[string]string{"1": "full_name", "2": "address", "3": "full_name"})
	if strings.Join(got, ",") != "address,full_name" {
		t.Errorf("got %v", got)
	}
}

func TestFitAttributeValue(t *testing.T) {
	for _, c := range []struct {
		in   string
		max  int
		want string
		cut  bool
	}{
		{"  Thôn  Đông \n Xã A ", 50, "Thôn Đông Xã A", false},
		{"{P1,20} vỡ mẫu", 50, "P1,20 vỡ mẫu", false},
		{"Nguyễn Văn An", 6, "Nguyễn", true},
		{"abc def", 4, "abc", true}, // the cut never leaves a trailing space
		{strings.Repeat("x", 600), 0, strings.Repeat("x", MaxNoticeParamValueRune), true},
		{"", 10, "", false},
	} {
		got, cut := FitAttributeValue(c.in, c.max)
		if got != c.want || cut != c.cut {
			t.Errorf("Fit(%q, %d) = %q, %v — want %q, %v", c.in, c.max, got, cut, c.want, c.cut)
		}
	}
}

func TestPreviewSmsContentMapped(t *testing.T) {
	content := "Chào {P1,10}, họp lúc {P2,5} tại {P3,20}"
	got, missing, err := PreviewSmsContentMapped(content, map[string]string{"2": "8h"},
		map[string]string{"1": "full_name", "3": "khong_nhan"}, map[string]string{"full_name": "Họ và tên"})
	if err != nil {
		t.Fatal(err)
	}
	// A label longer than the slot is fine at preview; an unlabeled code shows its code.
	if got != "Chào [Họ và tên], họp lúc 8h tại [khong_nhan]" || len(missing) != 0 {
		t.Errorf("got %q missing %v", got, missing)
	}
	got, missing, _ = PreviewSmsContentMapped(content, map[string]string{}, map[string]string{"1": "full_name"}, nil)
	if got != "Chào [full_name], họp lúc {P2,5} tại {P3,20}" || strings.Join(missing, ",") != "2,3" {
		t.Errorf("got %q missing %v", got, missing)
	}
	if _, _, err := PreviewSmsContentMapped(content, map[string]string{"2": "quá dài"}, nil, nil); !errors.Is(err, ErrNoticeParamLong) {
		t.Errorf("a fixed value past its slot: %v", err)
	}
}

func TestRenderSmsRow(t *testing.T) {
	content := "Chào {P1,10}, họp lúc {P2,5}"
	got, cut, err := RenderSmsRow(content, map[string]string{"2": "8h"}, map[string]string{"1": "full_name"},
		map[string]string{"full_name": "Nguyễn Văn An"})
	if err != nil || got != "Chào Nguyễn Văn, họp lúc 8h" || !cut {
		t.Errorf("got %q cut %v err %v — want the name cut to 10", got, cut, err)
	}
	got, cut, err = RenderSmsRow(content, map[string]string{"2": "8h"}, map[string]string{"1": "full_name"}, nil)
	if err != nil || got != "Chào , họp lúc 8h" || cut {
		t.Errorf("empty mapped value: %q %v %v — rendered empty, the provider decides", got, cut, err)
	}
	if _, _, err := RenderSmsRow(content, map[string]string{}, map[string]string{"1": "full_name"}, nil); !errors.Is(err, ErrSmsTemplateValueEmpty) {
		t.Errorf("a fixed slot without a value: %v", err)
	}
	got, _, _ = RenderSmsRow(content, map[string]string{"2": "8h"}, map[string]string{"1": "address"},
		map[string]string{"address": "{P9,9}"})
	if strings.Contains(got, "{") {
		t.Errorf("a value carried a raw slot into the text: %q", got)
	}
}

func TestZnsRowParams(t *testing.T) {
	declared := []TemplateParam{{Name: "ten", MaxLength: 6}, {Name: "su_kien"}, {Name: "dia_chi"}}
	got, cut := ZnsRowParams(declared, map[string]string{"su_kien": "Họp", "khac": "không khai"},
		map[string]string{"ten": "full_name", "dia_chi": "address"}, map[string]string{"full_name": "Nguyễn Văn An"})
	if len(got) != 2 || got["ten"] != "Nguyễn" || got["su_kien"] != "Họp" || !cut {
		t.Errorf("got %v cut %v — want fixed + cut name, empty address left out, undeclared key dropped", got, cut)
	}
}
