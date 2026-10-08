package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCitizenLetterTaskDeadline(t *testing.T) {
	utc := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, time.UTC) }
	for name, c := range map[string]struct{ in, want time.Time }{
		// 10:00Z = 17:00 ICT on the 20th → the 20th at 17:00 ICT = 10:00Z.
		"mid-day":            {utc(2026, 10, 20, 3, 0), utc(2026, 10, 20, 10, 0)},
		"exactly 17:00 ICT":  {utc(2026, 10, 20, 10, 0), utc(2026, 10, 20, 10, 0)},
		"23:30 ICT same day": {utc(2026, 10, 20, 16, 30), utc(2026, 10, 20, 10, 0)},
		// DATE BOUNDARY: 17:00Z on the 20th is 00:00 ICT on the 21st — the Vietnam date is the 21st.
		"00:00 ICT next day": {utc(2026, 10, 20, 17, 0), utc(2026, 10, 21, 10, 0)},
		// 16:59Z on the 20th is 23:59 ICT on the 20th — still the 20th, although UTC says so too.
		"23:59 ICT":                    {utc(2026, 10, 20, 16, 59), utc(2026, 10, 20, 10, 0)},
		"00:30 ICT is still that date": {utc(2026, 12, 31, 17, 30), utc(2027, 1, 1, 10, 0)},
	} {
		t.Run(name, func(t *testing.T) {
			got := CitizenLetterTaskDeadline(c.in)
			if !got.Equal(c.want) {
				t.Errorf("CitizenLetterTaskDeadline(%v) = %v, want %v", c.in, got, c.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("location = %v, want UTC (TIMESTAMPTZ stores an instant)", got.Location())
			}
		})
	}
	// A non-UTC input is read as its instant, not its wall clock.
	in := time.Date(2026, 10, 21, 0, 15, 0, 0, time.FixedZone("ICT", 7*3600))
	if got := CitizenLetterTaskDeadline(in); !got.Equal(utc(2026, 10, 21, 10, 0)) {
		t.Errorf("ICT input: got %v", got)
	}
}

func TestCitizenLetterTaskDeadlineNoneStaysNone(t *testing.T) {
	if got := CitizenLetterTaskDeadline(time.Time{}); !got.IsZero() {
		t.Errorf("a letter with no deadline gave the task %v — must stay none, never a default", got)
	}
}

func TestCitizenLetterTaskHolder(t *testing.T) {
	for name, c := range map[string]struct {
		bu, ba, lu, la string
		wu, wa         string
		err            error
	}{
		"body unit only wins whole":     {"bp-1", "", "bp-don", "CB-9", "bp-1", "", nil},
		"body assignee only wins whole": {"", "CB-1", "bp-don", "CB-9", "", "CB-1", nil},
		"body both":                     {"bp-1", "CB-1", "bp-don", "CB-9", "bp-1", "CB-1", nil},
		"fallback both":                 {"", "", "bp-don", "CB-9", "bp-don", "CB-9", nil},
		"fallback unit only":            {"", "", "bp-don", "", "bp-don", "", nil},
		"fallback assignee only":        {"", "", "", "CB-9", "", "CB-9", nil},
		"none anywhere":                 {"", "", "", "", "", "", ErrCitizenLetterAssignmentRequired},
	} {
		t.Run(name, func(t *testing.T) {
			u, a, err := CitizenLetterTaskHolder(c.bu, c.ba, c.lu, c.la)
			if !errors.Is(err, c.err) || u != c.wu || a != c.wa {
				t.Errorf("got (%q, %q, %v), want (%q, %q, %v)", u, a, err, c.wu, c.wa, c.err)
			}
		})
	}
	if CitizenLetterAssignmentRequiredSentence != "Chọn bộ phận hoặc người thực hiện." {
		t.Errorf("the owner's sentence (08/10/2026) changed: %q", CitizenLetterAssignmentRequiredSentence)
	}
}

func TestCheckCitizenLetterID(t *testing.T) {
	if err := CheckCitizenLetterID("01JDONTHU0000000000000000A"); err != nil {
		t.Errorf("valid id refused: %v", err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("A", CitizenLetterIDMaxLen+1)} {
		if !errors.Is(CheckCitizenLetterID(bad), ErrCitizenLetterIDInvalid) {
			t.Errorf("%q accepted", bad)
		}
	}
}
