package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Period matching — the one place that decides what a close locks (budget_period_close.go header).

func mkClose(code string, year, month int) BudgetPeriodClose {
	return BudgetPeriodClose{ID: code, Code: code, Year: year, Month: month, Revision: 1, ClosedBy: "CB-00012"}
}

func reopen(c BudgetPeriodClose) BudgetPeriodClose {
	c.ReopenedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	c.ReopenedBy, c.ReopenReason = "CB-00003", "sửa sai"
	return c
}

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func TestEntryLockingClose(t *testing.T) {
	sep := mkClose("CK-2026-09-01", 2026, 9)
	y26 := mkClose("CK-2026-CN-01", 2026, 0)
	y27 := mkClose("CK-2027-CN-01", 2027, 0)

	for ten, tc := range map[string]struct {
		closes    []BudgetPeriodClose
		date      time.Time
		sheetYear int
		want      string
	}{
		"month close covers its first day":       {[]BudgetPeriodClose{sep}, day(2026, 9, 1), 2026, "CK-2026-09-01"},
		"month close covers its last day":        {[]BudgetPeriodClose{sep}, day(2026, 9, 30), 2026, "CK-2026-09-01"},
		"month close does not cover next month":  {[]BudgetPeriodClose{sep}, day(2026, 10, 1), 2026, ""},
		"month close does not cover prior month": {[]BudgetPeriodClose{sep}, day(2026, 8, 31), 2026, ""},
		"same month, other year":                 {[]BudgetPeriodClose{sep}, day(2027, 9, 15), 2027, ""},
		"year close by the date's year":          {[]BudgetPeriodClose{y27}, day(2027, 1, 5), 2026, "CK-2027-CN-01"},
		"year close by the sheet's year":         {[]BudgetPeriodClose{y26}, day(2027, 1, 5), 2026, "CK-2026-CN-01"},
		"reopened month close no longer locks":   {[]BudgetPeriodClose{reopen(sep)}, day(2026, 9, 15), 2026, ""},
		"reopened year close no longer locks":    {[]BudgetPeriodClose{reopen(y26)}, day(2026, 9, 15), 2026, ""},
		// Re-close after reopen: the reopened row and the new active row side by side — the new one locks.
		"re-close after reopen locks again": {
			[]BudgetPeriodClose{reopen(sep), {ID: "k2", Code: "CK-2026-09-02", Year: 2026, Month: 9, Revision: 2}},
			day(2026, 9, 15), 2026, "CK-2026-09-02"},
		// Both a month and a year close: the sentence names the narrower one.
		"month close named before year close": {[]BudgetPeriodClose{y26, sep}, day(2026, 9, 15), 2026, "CK-2026-09-01"},
		"nothing closed":                      {nil, day(2026, 9, 15), 2026, ""},
	} {
		t.Run(ten, func(t *testing.T) {
			c, locked := EntryLockingClose(tc.closes, tc.date, tc.sheetYear)
			if tc.want == "" {
				if locked {
					t.Fatalf("locked by %s, want open", c.Code)
				}
				return
			}
			if !locked || c.Code != tc.want {
				t.Fatalf("= %q %v, want %q", c.Code, locked, tc.want)
			}
		})
	}
}

func TestSheetLockingCloseOnlyYearCloses(t *testing.T) {
	if _, locked := SheetLockingClose([]BudgetPeriodClose{mkClose("CK-2026-09-01", 2026, 9)}, 2026); locked {
		t.Fatal("a MONTH close locked the sheet — hand values carry no date")
	}
	if c, locked := SheetLockingClose([]BudgetPeriodClose{mkClose("CK-2026-CN-01", 2026, 0)}, 2026); !locked || c.Code != "CK-2026-CN-01" {
		t.Fatal("a YEAR close did not lock its sheets")
	}
	if _, locked := SheetLockingClose([]BudgetPeriodClose{mkClose("CK-2025-CN-01", 2025, 0)}, 2026); locked {
		t.Fatal("another year's close locked this sheet")
	}
	if _, locked := SheetLockingClose([]BudgetPeriodClose{reopen(mkClose("CK-2026-CN-01", 2026, 0))}, 2026); locked {
		t.Fatal("a reopened year close still locks")
	}
}

func TestBudgetPeriodCloseCode(t *testing.T) {
	for want, got := range map[string]string{
		"CK-2026-09-01": BudgetPeriodCloseCode(2026, 9, 1),
		"CK-2026-01-12": BudgetPeriodCloseCode(2026, 1, 12),
		"CK-2026-CN-01": BudgetPeriodCloseCode(2026, 0, 1),
		"CK-2026-CN-03": BudgetPeriodCloseCode(2026, 0, 3),
	} {
		if got != want {
			t.Errorf("= %q, want %q", got, want)
		}
	}
}

func TestLockYearsAscendingDistinct(t *testing.T) {
	got := LockYears(2027, 2026, 2027)
	if len(got) != 2 || got[0] != 2026 || got[1] != 2027 {
		t.Fatalf("= %v, want [2026 2027]", got)
	}
}

func TestValidateClosePeriod(t *testing.T) {
	for _, ok := range [][2]int{{2026, 0}, {2026, 1}, {2026, 12}, {2000, 5}, {2100, 5}} {
		if err := ValidateClosePeriod(ok[0], ok[1]); err != nil {
			t.Errorf("%v refused: %v", ok, err)
		}
	}
	for _, bad := range [][2]int{{2026, 13}, {2026, -1}, {1999, 1}, {2101, 1}} {
		if err := ValidateClosePeriod(bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestReasonsNormalised(t *testing.T) {
	if r, err := NormaliseReopenReason("  sửa sai  "); err != nil || r != "sửa sai" {
		t.Fatalf("= %q %v", r, err)
	}
	if _, err := NormaliseReopenReason(" "); !errors.Is(err, ErrReopenReasonMissing) {
		t.Fatalf("blank = %v", err)
	}
	if _, err := NormaliseReopenReason(strings.Repeat("ệ", ReopenReasonMax+1)); !errors.Is(err, ErrReopenReasonTooLong) {
		t.Fatalf("501 runes = %v", err)
	}
	if r, err := NormaliseAdjustmentReason(nil); err != nil || r != "" {
		t.Fatalf("absent = %q %v, want ordinary entry", r, err)
	}
	blank := "  "
	if _, err := NormaliseAdjustmentReason(&blank); !errors.Is(err, ErrAdjustmentReasonBlank) {
		t.Fatalf("blank = %v", err)
	}
	full := strings.Repeat("ệ", AdjustmentReasonMax)
	if r, err := NormaliseAdjustmentReason(&full); err != nil || r != full {
		t.Fatalf("500 runes refused: %v", err)
	}
}

func TestConflictSentencesNameThePeriodAndCode(t *testing.T) {
	sep := mkClose("CK-2026-09-01", 2026, 9)
	y := mkClose("CK-2026-CN-01", 2026, 0)
	for _, tc := range []struct {
		err       error
		sentinel  error
		mustHaves []string
	}{
		{EntryPeriodClosedError(sep), ErrPeriodClosed, []string{"tháng 09/2026", "CK-2026-09-01", "điều chỉnh"}},
		{SheetYearClosedError(y), ErrPeriodClosed, []string{"2026", "CK-2026-CN-01"}},
		{AlreadyClosedError(y), ErrPeriodAlreadyClosed, []string{"năm 2026", "CK-2026-CN-01"}},
		{AlreadyReopenedError(sep), ErrCloseAlreadyReopened, []string{"CK-2026-09-01"}},
	} {
		if !errors.Is(tc.err, tc.sentinel) {
			t.Errorf("%q does not match its sentinel", tc.err)
		}
		for _, s := range tc.mustHaves {
			if !strings.Contains(tc.err.Error(), s) {
				t.Errorf("%q lacks %q", tc.err, s)
			}
		}
	}
}
