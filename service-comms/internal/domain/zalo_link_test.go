package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizePairingCode(t *testing.T) {
	ok := map[string]string{
		"ABCDEFGH":     "ABCDEFGH",
		"  abcdefgh  ": "ABCDEFGH",
		"abcd efgh":    "ABCDEFGH",
		"ABCD-2345":    "ABCD2345",
	}
	for in, want := range ok {
		got, valid := NormalizePairingCode(in)
		if !valid || got != want {
			t.Errorf("%q → %q,%v; want %q", in, got, valid, want)
		}
	}
	// Look-alikes (0 O 1 I L), wrong length, stray characters: not code-shaped.
	for _, in := range []string{"ABCDEFG", "ABCDEFGHJ", "ABCDEF0H", "ABCDEFOH", "ABCDEF1H", "ABCDEFIH", "ABCDEFLH",
		"ABC EFGHJ", "ABCD_EFG", "xin chào", "", "/start", "ABCDEFGÀ"} {
		if _, valid := NormalizePairingCode(in); valid {
			t.Errorf("%q accepted as a code", in)
		}
	}
	if len(PairingCodeHash("ABCDEFGH")) != 32 {
		t.Fatal("hash must be 32 bytes (0018 CHECK)")
	}
	a, _ := NormalizePairingCode("abcd-efgh")
	if string(PairingCodeHash(a)) != string(PairingCodeHash("ABCDEFGH")) {
		t.Fatal("two spellings of one code hash differently")
	}
}

func TestPairingPolicyIsTheAdoptedSpec(t *testing.T) {
	// ADR 0074: 8 characters, 10 minutes, 3 wrong tries. A change must turn something red.
	if PairingCodeLength != 8 || PairingCodeTTL != 10*time.Minute || PairingMaxFailedAttempts != 3 {
		t.Fatal("pairing policy drifted from ADR 0074")
	}
	for _, c := range "0O1IL" {
		if strings.ContainsRune(PairingCodeAlphabet, c) {
			t.Errorf("alphabet holds the look-alike %q", c)
		}
	}
}

func TestParseZaloCommand(t *testing.T) {
	cases := map[string]ZaloCommand{
		"/trogiup": ZaloCommandHelp, "/HELP": ZaloCommandHelp, " /batdau ": ZaloCommandHelp,
		"/start abc": ZaloCommandHelp, "/dung": ZaloCommandStop, "/Stop": ZaloCommandStop,
		"dung": ZaloCommandNone, "ABCDEFGH": ZaloCommandNone, "": ZaloCommandNone,
	}
	for in, want := range cases {
		if got := ParseZaloCommand(in); got != want {
			t.Errorf("%q → %v, want %v", in, got, want)
		}
	}
}

func days(n int) *int { return &n }

func TestNormalizeZaloChannelSettingMirrorsTheChecks(t *testing.T) {
	base := ZaloChannelSetting{IsEnabled: true, Kinds: []string{"leo-thang", "sap-den-han", "sap-den-han"},
		QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60}
	got, err := NormalizeZaloChannelSetting(base)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Kinds, ",") != "sap-den-han,leo-thang" {
		t.Errorf("kinds = %v, want de-duplicated in canonical order", got.Kinds)
	}

	refuse := map[string]func(*ZaloChannelSetting){
		"unknown_kind":                func(s *ZaloChannelSetting) { s.Kinds = []string{"thu-nghiem"} },
		"enabled_without_kind":        func(s *ZaloChannelSetting) { s.Kinds = nil },
		"empty_quiet_window":          func(s *ZaloChannelSetting) { s.QuietEndMinute = s.QuietStartMinute },
		"invalid_quiet_time":          func(s *ZaloChannelSetting) { s.QuietEndMinute = 24 * 60 },
		"overdue_cadence_incomplete":  func(s *ZaloChannelSetting) { s.OverdueStartAfterDays = days(1) },
		"overdue_cadence_required":    func(s *ZaloChannelSetting) { s.Kinds = []string{"qua-han"} },
		"overdue_start_out_of_range":  func(s *ZaloChannelSetting) { s.OverdueStartAfterDays, s.OverdueRepeatEveryDays = days(366), days(1) },
		"overdue_repeat_out_of_range": func(s *ZaloChannelSetting) { s.OverdueStartAfterDays, s.OverdueRepeatEveryDays = days(0), days(0) },
	}
	for code, mutate := range refuse {
		s := base
		s.Kinds = append([]string(nil), base.Kinds...)
		mutate(&s)
		_, err := NormalizeZaloChannelSetting(s)
		var se *ZaloSettingError
		if !errors.As(err, &se) || se.Code != code || !errors.Is(err, ErrZaloSettingInvalid) || se.Message == "" {
			t.Errorf("%s: err = %v", code, err)
		}
	}

	// Off with no kind is valid; overdue with both numbers is valid; a disabled channel may keep kinds.
	if _, err := NormalizeZaloChannelSetting(ZaloChannelSetting{QuietStartMinute: 1, QuietEndMinute: 2}); err != nil {
		t.Errorf("off with no kind refused: %v", err)
	}
	ok := base
	ok.Kinds = []string{"qua-han"}
	ok.OverdueStartAfterDays, ok.OverdueRepeatEveryDays = days(0), days(2)
	if _, err := NormalizeZaloChannelSetting(ok); err != nil {
		t.Errorf("overdue with cadence refused: %v", err)
	}
}

func TestClock(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "06:00": 360, "21:30": 1290, "23:59": 1439} {
		if got, ok := ParseClock(in); !ok || got != want || FormatClock(got) != in {
			t.Errorf("%q → %d,%v", in, got, ok)
		}
	}
	for _, in := range []string{"24:00", "6:00", "06:60", "06-00", "", "+6:00", "06:+0", "aa:bb"} {
		if _, ok := ParseClock(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestQuietUntilWrapsMidnightInVietnamTime(t *testing.T) {
	vn := func(h, m int) time.Time { return time.Date(2026, 10, 5, h, m, 0, 0, VietnamTime) }
	start, end := 21*60, 6*60
	cases := []struct {
		at    time.Time
		quiet bool
		until time.Time
	}{
		{vn(20, 59), false, time.Time{}},
		{vn(21, 0), true, time.Date(2026, 10, 6, 6, 0, 0, 0, VietnamTime)},
		{vn(23, 30), true, time.Date(2026, 10, 6, 6, 0, 0, 0, VietnamTime)},
		{vn(2, 0), true, vn(6, 0)},
		{vn(6, 0), false, time.Time{}},
	}
	for _, c := range cases {
		until, quiet := QuietUntil(c.at.UTC(), start, end)
		if quiet != c.quiet || !until.Equal(c.until) {
			t.Errorf("%v: quiet=%v until=%v, want %v %v", c.at, quiet, until, c.quiet, c.until)
		}
	}
	// A daytime window (no wrap).
	if _, quiet := QuietUntil(vn(12, 30), 12*60, 13*60); !quiet {
		t.Error("12:30 is inside 12:00–13:00")
	}
	if _, quiet := QuietUntil(vn(13, 0), 12*60, 13*60); quiet {
		t.Error("13:00 is the end of 12:00–13:00, not inside it")
	}
}

func TestZaloDeliveryBackoff(t *testing.T) {
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 30 * time.Minute, 30 * time.Minute}
	for i, w := range want {
		if got := ZaloDeliveryBackoff(i + 1); got != w {
			t.Errorf("after %d attempts: %v, want %v", i+1, got, w)
		}
	}
}

func TestZaloReminderTextAndLink(t *testing.T) {
	if got := ZaloReminderText(" Việc sắp đến hạn ", "", ""); got != "Việc sắp đến hạn" {
		t.Errorf("got %q", got)
	}
	link := AbsoluteStaffLink("xa-a.vigov.vn", "/nhiem-vu?soon=true")
	if link != "https://xa-a.vigov.vn/nhiem-vu?soon=true" {
		t.Errorf("link = %q", link)
	}
	if got := ZaloReminderText("T", "B", link); got != "T\nB\n"+link {
		t.Errorf("got %q", got)
	}
	for _, bad := range []string{"", "nhiem-vu", "//evil.example/x", "/a b"} {
		if AbsoluteStaffLink("xa-a.vigov.vn", bad) != "" {
			t.Errorf("%q made into a link", bad)
		}
	}
	if AbsoluteStaffLink("", "/x") != "" {
		t.Error("no host, still a link")
	}
}
