package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func ip(v int) *int   { return &v }
func bp(v bool) *bool { return &v }

func TestMergePortalSyncSettings(t *testing.T) {
	base := DefaultPortalSyncSettings()
	got, err := MergePortalSyncSettings(base, PortalSyncSettingsInput{APIURL: "  https://x.gov.vn/api  "})
	if err != nil || got.APIURL != "https://x.gov.vn/api" || got.IntervalHours != 6 || got.WindowDays != 90 ||
		got.MaxItemsPerRun != 100 || got.PublishMode != PortalPublishReview || !got.KeepSourceCredit || got.IsEnabled ||
		got.Provider != PortalProviderCityShared {
		t.Fatalf("defaults kept = %+v, %v", got, err)
	}
	for name, tc := range map[string]struct {
		in   PortalSyncSettingsInput
		want error
	}{
		"empty url":      {PortalSyncSettingsInput{}, ErrPortalAPIURLInvalid},
		"space in url":   {PortalSyncSettingsInput{APIURL: "https://x.gov.vn/a b"}, ErrPortalAPIURLInvalid},
		"backslash":      {PortalSyncSettingsInput{APIURL: `https://x.gov.vn\@evil`}, ErrPortalAPIURLInvalid},
		"too long":       {PortalSyncSettingsInput{APIURL: "https://x.gov.vn/" + strings.Repeat("a", PortalAPIURLMaxLen)}, ErrPortalAPIURLInvalid},
		"mode":           {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", PublishMode: "an"}, ErrPortalPublishMode},
		"interval < 0":   {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", IntervalHours: ip(-1)}, ErrPortalInterval},
		"interval > 24":  {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", IntervalHours: ip(25)}, ErrPortalInterval},
		"window 0":       {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", WindowDays: ip(0)}, ErrPortalWindow},
		"window huge":    {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", WindowDays: ip(PortalWindowDaysMax + 1)}, ErrPortalWindow},
		"max 0":          {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", MaxItemsPerRun: ip(0)}, ErrPortalMaxItems},
		"max over bound": {PortalSyncSettingsInput{APIURL: "https://x.gov.vn", MaxItemsPerRun: ip(PortalMaxItemsMax + 1)}, ErrPortalMaxItems},
	} {
		if _, err := MergePortalSyncSettings(base, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	got, err = MergePortalSyncSettings(base, PortalSyncSettingsInput{APIURL: "https://x.gov.vn", IntervalHours: ip(0),
		PublishMode: PortalPublishDirect, KeepSourceCredit: bp(false), IsEnabled: bp(true)})
	if err != nil || got.IntervalHours != 0 || got.PublishMode != PortalPublishDirect || got.KeepSourceCredit || !got.IsEnabled {
		t.Fatalf("explicit values = %+v, %v", got, err)
	}
}

func TestPortalAPIURLChangedIsExact(t *testing.T) {
	a := PortalSyncSettings{APIURL: "https://x.gov.vn/api"}
	if PortalAPIURLChanged(a, PortalSyncSettings{APIURL: " https://x.gov.vn/api "}) {
		t.Error("whitespace counted as a change")
	}
	if !PortalAPIURLChanged(a, PortalSyncSettings{APIURL: "https://x.gov.vn/API"}) {
		t.Error("a different path is a different endpoint")
	}
}

func TestValidatePortalAPIKey(t *testing.T) {
	if err := ValidatePortalAPIKey([]byte("abc-123_XYZ")); err != nil {
		t.Errorf("valid key refused: %v", err)
	}
	for _, k := range []string{"", "a b", "a\nb", strings.Repeat("k", PortalAPIKeyMaxLen+1), "\xff"} {
		if err := ValidatePortalAPIKey([]byte(k)); !errors.Is(err, ErrPortalAPIKeyShape) {
			t.Errorf("%q accepted", k)
		}
	}
}

func TestPortalSyncDue(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	on := PortalSyncSettings{IsEnabled: true, IntervalHours: 6}
	for name, tc := range map[string]struct {
		s    PortalSyncSettings
		want bool
	}{
		"never run":         {on, true},
		"exactly due":       {PortalSyncSettings{IsEnabled: true, IntervalHours: 6, LastRunAt: now.Add(-6 * time.Hour)}, true},
		"not yet":           {PortalSyncSettings{IsEnabled: true, IntervalHours: 6, LastRunAt: now.Add(-5 * time.Hour)}, false},
		"disabled":          {PortalSyncSettings{IntervalHours: 6}, false},
		"manual only (0 h)": {PortalSyncSettings{IsEnabled: true, IntervalHours: 0}, false},
	} {
		if got := PortalSyncDue(tc.s, now); got != tc.want {
			t.Errorf("%s: due = %v, want %v", name, got, tc.want)
		}
	}
}

func TestNormalizePortalSelection(t *testing.T) {
	got, err := NormalizePortalSelection([]PortalCategorySelection{{ExternalID: " 120 ", Name: " Tin ", TargetKind: "su-kien", IsSelected: true}})
	if err != nil || got[0].ExternalID != "120" || got[0].Name != "Tin" {
		t.Fatalf("got %+v, %v", got, err)
	}
	for name, tc := range map[string]struct {
		in   []PortalCategorySelection
		want error
	}{
		"no id":     {[]PortalCategorySelection{{Name: "x", TargetKind: "tin-tuc"}}, ErrPortalSelectionEmpty},
		"id zero":   {[]PortalCategorySelection{{ExternalID: "0", Name: "x", TargetKind: "tin-tuc"}}, ErrPortalSelectionEmpty},
		"no name":   {[]PortalCategorySelection{{ExternalID: "1", TargetKind: "tin-tuc"}}, ErrPortalSelectionName},
		"control":   {[]PortalCategorySelection{{ExternalID: "1", Name: "a\x00", TargetKind: "tin-tuc"}}, ErrPortalSelectionName},
		"banner":    {[]PortalCategorySelection{{ExternalID: "1", Name: "x", TargetKind: "banner"}}, ErrPortalSelectionKind},
		"audio":     {[]PortalCategorySelection{{ExternalID: "1", Name: "x", TargetKind: "truyen-thanh"}}, ErrPortalSelectionKind},
		"duplicate": {[]PortalCategorySelection{{ExternalID: "1", Name: "x", TargetKind: "tin-tuc"}, {ExternalID: "1", Name: "y", TargetKind: "tin-tuc"}}, ErrPortalSelectionDup},
	} {
		if _, err := NormalizePortalSelection(tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := NormalizePortalSelection(make([]PortalCategorySelection, PortalSelectionMax+1)); !errors.Is(err, ErrPortalSelectionTooBig) {
		t.Errorf("too many: %v", err)
	}
}

func TestPortalRunOutcome(t *testing.T) {
	for name, tc := range map[string]struct {
		read, failed int
		c            PortalRunCounts
		interrupted  bool
		want         string
	}{
		"clean":                  {2, 0, PortalRunCounts{Imported: 3}, false, PortalRunSucceeded},
		"nothing new is success": {2, 0, PortalRunCounts{SkippedExisting: 3}, false, PortalRunSucceeded},
		"one category failed":    {1, 1, PortalRunCounts{}, false, PortalRunPartial},
		"an article failed":      {2, 0, PortalRunCounts{Failed: 1}, false, PortalRunPartial},
		"interrupted after work": {1, 0, PortalRunCounts{}, true, PortalRunPartial},
		"every category failed":  {0, 2, PortalRunCounts{}, false, PortalRunFailed},
		"nothing read":           {0, 0, PortalRunCounts{}, true, PortalRunFailed},
	} {
		if got := PortalRunOutcome(tc.read, tc.failed, tc.c, tc.interrupted); got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
}

func TestPortalSourceCreditIsEscaped(t *testing.T) {
	if got := PortalSourceCredit(`<script>"x"</script>`); got != "<p><em>Nguồn: &lt;script&gt;&#34;x&#34;&lt;/script&gt;</em></p>" {
		t.Errorf("credit = %s", got)
	}
	if PortalSourceCredit("  ") != "" {
		t.Error("an empty label made a credit")
	}
	if PortalExternalItemID(PortalProviderCityShared, "9001") != "cttdt-danang:9001" {
		t.Error("external id is not namespaced by provider")
	}
}

// D1 (owner, 02/10/2026): 90 days and 100 items are CEILINGS; 30 selected categories at most.
func TestPortalCeilingsArePinned(t *testing.T) {
	if PortalWindowDaysMax != 90 || PortalMaxItemsMax != 100 || PortalSelectedCategoriesMax != 30 {
		t.Fatalf("ceilings = %d / %d / %d, the owner decided 90 / 100 / 30",
			PortalWindowDaysMax, PortalMaxItemsMax, PortalSelectedCategoriesMax)
	}
	if PortalDefaultWindowDays != PortalWindowDaysMax || PortalDefaultMaxItems != PortalMaxItemsMax {
		t.Fatal("the defaults are the ceilings")
	}
}

func TestPortalSettingsClamped(t *testing.T) {
	s := DefaultPortalSyncSettings()
	if got, lowered := s.Clamped(); lowered || got != s {
		t.Fatalf("a row at the defaults was changed: %+v", got)
	}
	s.WindowDays, s.MaxItemsPerRun = 3650, 1000
	got, lowered := s.Clamped()
	if !lowered || got.WindowDays != 90 || got.MaxItemsPerRun != 100 {
		t.Fatalf("clamped = %+v, %v", got, lowered)
	}
	s.WindowDays, s.MaxItemsPerRun = 7, 5
	if got, lowered := s.Clamped(); lowered || got.WindowDays != 7 || got.MaxItemsPerRun != 5 {
		t.Fatalf("a value under the ceiling was changed: %+v", got)
	}
}

func TestNormalizePortalSelectionRefusesMoreThanThirtySelected(t *testing.T) {
	mk := func(n, unticked int) []PortalCategorySelection {
		out := make([]PortalCategorySelection, 0, n+unticked)
		for i := 0; i < n+unticked; i++ {
			out = append(out, PortalCategorySelection{ExternalID: fmt.Sprintf("%d", i+1), Name: "C", TargetKind: "tin-tuc",
				IsSelected: i < n})
		}
		return out
	}
	if _, err := NormalizePortalSelection(mk(31, 0)); !errors.Is(err, ErrPortalTooManySelected) {
		t.Fatalf("31 selected: %v", err)
	}
	if _, err := NormalizePortalSelection(mk(30, 50)); err != nil {
		t.Fatalf("30 selected and 50 unticked is allowed: %v", err)
	}
}

func TestValidContentStatus(t *testing.T) {
	for s, ok := range map[string]bool{"an": true, "cho-duyet": true, "dang-hien": true, "": false, "Cho-Duyet": false, "xoa": false} {
		if ValidContentStatus(s) != ok {
			t.Errorf("ValidContentStatus(%q) = %v", s, !ok)
		}
	}
}
