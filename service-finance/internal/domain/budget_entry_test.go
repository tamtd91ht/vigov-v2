package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The read half of the `entries` mode: which source a leaf's figure comes from, and that everything
// above it — parents, the summary row, the indicators — picks the batch sum up.

// entriesSheet is a chi sheet: `Tổng số` (marked) is a PARENT of two leaves — one `manual`, one
// `entries`. The `entries` leaf ALSO carries an old typed figure, which must never be shown.
func entriesSheet() FullSheet {
	return FullSheet{
		Sheet: BudgetSheet{ID: "b", Code: "NS-2026-CHI-01", Year: 2026, Kind: SheetKindExpenditure},
		Columns: []BudgetColumn{
			{ID: "c-dt", Name: "Dự toán năm", SortOrder: 1, Format: ColumnFormatNumber, Indicator: IndicatorAnnualEstimate},
			{ID: "c-chi", Name: "Chi ngân sách", SortOrder: 2, Format: ColumnFormatNumber, Indicator: IndicatorBudgetExpenditure},
		},
		Lines: []BudgetLine{
			{ID: "tong", Name: "Tổng số", SortOrder: 1, Method: MethodChildren, IsHeadline: true},
			{ID: "tay", ParentID: "tong", Name: "Chi thường xuyên", SortOrder: 2, Method: MethodManual, Level: 1},
			{ID: "dot", ParentID: "tong", Name: "Chi sự nghiệp", SortOrder: 3, Method: MethodEntries, Level: 1},
		},
		Values: map[string]map[string]Dong{
			"tay": {"c-dt": 1_000, "c-chi": 400},
			"dot": {"c-dt": 9_999_999, "c-chi": 8_888_888}, // stale typed figures — locked, not shown
		},
		EntryTotals: map[string]map[string]Dong{
			"dot": {"c-chi": 600},                    // c-dt: no live batch stated it -> EMPTY
			"tay": {"c-dt": 77_777, "c-chi": 77_777}, // batches on a MANUAL leaf count nowhere
		},
	}
}

// valueAndPresent reads one cell as (figure, present) and FAILS the test when the cell is unavailable — so a
// test written for a figure cannot pass on an unavailable cell by reading its zero value.
func valueAndPresent(t *testing.T, b FullSheet, lineID, columnID string) (Dong, bool) {
	t.Helper()
	s := b.Value(lineID, columnID)
	if s.Reason != "" {
		t.Fatalf("ô %s/%s không tính được: %s", lineID, columnID, s.Reason)
	}
	return s.Value, s.Present
}

func TestValue_EntriesLeafReadsEntryTotalNotTypedCell(t *testing.T) {
	b := entriesSheet()
	if g, ok := valueAndPresent(t, b, "dot", "c-chi"); !ok || g != 600 {
		t.Fatalf("lá `entries` = %d,%v — muốn tổng đợt 600", g, ok)
	}
	// EMPTY, NOT 0 AND NOT THE STALE 9 999 999 (§9 rule 4, §9.1).
	if g, ok := valueAndPresent(t, b, "dot", "c-dt"); ok {
		t.Fatalf("lá `entries` không có đợt nào ở c-dt mà vẫn ra %d", g)
	}
}

func TestValue_ManualLeafIgnoresEntryTotal(t *testing.T) {
	b := entriesSheet()
	if g, ok := valueAndPresent(t, b, "tay", "c-chi"); !ok || g != 400 {
		t.Fatalf("lá `manual` = %d,%v — muốn số gõ tay 400, không phải tổng đợt", g, ok)
	}
}

func TestValue_ParentSumsEntriesLeafToo(t *testing.T) {
	b := entriesSheet()
	if g, ok := valueAndPresent(t, b, "tong", "c-chi"); !ok || g != 1_000 {
		t.Fatalf("cha = %d,%v — muốn 400 (tay) + 600 (tổng đợt) = 1000", g, ok)
	}
	// c-dt: the manual leaf's 1000 + the entries leaf's EMPTY = 1000. Not 1000 + 9 999 999.
	if g, ok := valueAndPresent(t, b, "tong", "c-dt"); !ok || g != 1_000 {
		t.Fatalf("cha c-dt = %d,%v — muốn 1000; số gõ tay đã khoá của lá `entries` lọt vào tổng", g, ok)
	}
}

func TestIndicator_UsesEntriesLeafFigure(t *testing.T) {
	// Chi đạt dự toán = Chi ngân sách / Dự toán năm on the marked row = 1000 / 1000 = 100,00%.
	// Had the stale typed figures of the entries leaf been read, it would be 8 889 288 / 10 000 999.
	b := entriesSheet()
	_, ratio := EstimateAttainment(b)
	if !ratio.Present || ratio.Value != 10_000 {
		t.Fatalf("Chi đạt dự toán = %+v, muốn 10000 phần vạn", ratio)
	}
	g, ok, err := b.HeadlineValue(IndicatorBudgetExpenditure)
	if err != nil || !ok || g != 1_000 {
		t.Fatalf("số tổng Chi ngân sách = %d,%v,%v — muốn 1000", g, ok, err)
	}
}

func TestValue_LineWithChildrenSumsChildrenWhateverStoredMethod(t *testing.T) {
	// A line whose stored mode says `entries` but which HAS children sums its children: the tree wins.
	// The write path makes this state unreachable; the read path must not trust it anyway.
	b := entriesSheet()
	b.Lines[0].Method = MethodEntries
	b.EntryTotals["tong"] = map[string]Dong{"c-chi": 5}
	if g, _ := valueAndPresent(t, b, "tong", "c-chi"); g != 1_000 {
		t.Fatalf("cha mang nhãn `entries` = %d — muốn tổng con 1000", g)
	}
}

// --- figures that cannot be computed: a reason, never a wrapped or clamped number ---------------------

func TestValueMaxIsBrowserSafeInteger(t *testing.T) {
	// Pinned as a literal: 2^53 − 1 is what JSON.parse reads exactly. A value past it arrives in the
	// browser rounded, so the ceiling moving up again would re-open a one-đồng-off display.
	if ValueMax != 9_007_199_254_740_991 || int64(ValueMax) != 1<<53-1 {
		t.Fatalf("GiaTriToiDa = %d, muốn 2^53-1", int64(ValueMax))
	}
	if err := ValidateValue(ValueMax); err != nil {
		t.Fatalf("đúng trần bị từ chối: %v", err)
	}
	if err := ValidateValue(-ValueMax); err != nil {
		t.Fatalf("đúng trần âm bị từ chối: %v", err)
	}
	// The old ceiling (10^17) is now a refusal on write.
	if err := ValidateValue(100_000_000_000_000_000); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("10^17 = %v, muốn ErrGiaTriQuaLon", err)
	}
}

func TestValue_ParentPastCeilingIsUnavailableNotNegative(t *testing.T) {
	// Two children each AT the ceiling: each is a legal figure, their sum is not one the browser can
	// read. The old `total += g` returned a plausible figure here (and wrapped to a NEGATIVE one once
	// the operands were large enough); now the cell carries a reason.
	b := entriesSheet()
	b.Values["tay"]["c-chi"] = ValueMax
	b.EntryTotals["dot"]["c-chi"] = ValueMax

	s := b.Value("tong", "c-chi")
	if s.Present || s.Value != 0 {
		t.Fatalf("cha vượt trần = %+v, muốn KHÔNG có số", s)
	}
	if !strings.Contains(s.Reason, ErrTotalOverflow.Error()) || !strings.Contains(s.Reason, "Tổng số") {
		t.Fatalf("lý do = %q, muốn câu ErrTongVuotMuc kèm tên dòng", s.Reason)
	}
	// ISOLATED: the children themselves and the other column still read.
	if g, ok := valueAndPresent(t, b, "tay", "c-chi"); !ok || g != ValueMax {
		t.Fatalf("lá tay = %d,%v", g, ok)
	}
	if g, ok := valueAndPresent(t, b, "tong", "c-dt"); !ok || g != 1_000 {
		t.Fatalf("cột khác của cha = %d,%v, muốn 1000", g, ok)
	}
	// Everything built on the unavailable figure is unavailable with the same sentence — never a
	// ratio of a wrong number.
	if _, _, err := b.HeadlineValue(IndicatorBudgetExpenditure); !errors.Is(err, ErrTotalOverflow) {
		t.Fatalf("SoTong = %v, muốn ErrTongVuotMuc", err)
	}
	if _, ratio := EstimateAttainment(b); ratio.Present || !strings.Contains(ratio.Reason, ErrTotalOverflow.Error()) {
		t.Fatalf("chỉ số = %+v, muốn không tính được", ratio)
	}
}

func TestValue_MixedSignSumPastCeilingMidwayStaysCorrect(t *testing.T) {
	// A large positive and a large negative child: the partial sum passes the ceiling, the total does
	// not. Checking per step would refuse a correct figure.
	b := entriesSheet()
	b.Values["tay"]["c-chi"] = ValueMax
	b.EntryTotals["dot"]["c-chi"] = -ValueMax + 5
	b.Lines = append(b.Lines, BudgetLine{ID: "them", ParentID: "tong", Name: "Chi khác", SortOrder: 4, Level: 1})
	b.Values["them"] = map[string]Dong{"c-chi": 10}
	if g, ok := valueAndPresent(t, b, "tong", "c-chi"); !ok || g != 15 {
		t.Fatalf("cha = %d,%v, muốn 15", g, ok)
	}
}

func TestCheckedAddReportsInt64Overflow(t *testing.T) {
	const max = Dong(1<<63 - 1)
	if _, overflow := checkedAdd(max, 1); !overflow {
		t.Fatal("max+1 không báo tràn")
	}
	if _, overflow := checkedAdd(-max-1, -1); !overflow {
		t.Fatal("min-1 không báo tràn")
	}
	if s, overflow := checkedAdd(max, -max); overflow || s != 0 {
		t.Fatalf("max-max = %d,%v", s, overflow)
	}
}

func TestValue_EntryTotalPastInt64BlocksOnlyThatLine(t *testing.T) {
	// The store could not fit the SUM into int64 and flagged it. That line and its ancestors in that
	// column are unavailable with the batch sentence; the other lines and columns are untouched.
	b := entriesSheet()
	delete(b.EntryTotals["dot"], "c-chi")
	b.EntryTotalOverflow = map[string]map[string]bool{"dot": {"c-chi": true}}

	for _, id := range []string{"dot", "tong"} {
		s := b.Value(id, "c-chi")
		if s.Present || !strings.Contains(s.Reason, ErrEntryTotalOverflow.Error()) || !strings.Contains(s.Reason, "Chi sự nghiệp") {
			t.Fatalf("%s = %+v, muốn lý do tổng đợt kèm tên dòng lá", id, s)
		}
	}
	if g, ok := valueAndPresent(t, b, "tay", "c-chi"); !ok || g != 400 {
		t.Fatalf("dòng nhập tay bên cạnh = %d,%v, muốn 400", g, ok)
	}
	// A sum that fits int64 but is past the ceiling is the same sentence.
	b.EntryTotalOverflow = nil
	b.EntryTotals["dot"]["c-chi"] = ValueMax + 1
	if s := b.Value("dot", "c-chi"); s.Present || !strings.Contains(s.Reason, ErrEntryTotalOverflow.Error()) {
		t.Fatalf("tổng đợt vượt trần = %+v", s)
	}
}

func TestValue_StoredValuePastNewCeilingReadsWithReason(t *testing.T) {
	// A cell stored under the old 10^17 ceiling. The sheet READS; that cell is flagged.
	b := entriesSheet()
	b.Values["tay"]["c-chi"] = 50_000_000_000_000_000
	s := b.Value("tay", "c-chi")
	if s.Present || !strings.Contains(s.Reason, ErrStoredValueOverflow.Error()) {
		t.Fatalf("ô cũ vượt trần = %+v, muốn lý do ErrGiaTriDaLuuVuotMuc", s)
	}
	if err := ValidateStoredValue(-50_000_000_000_000_000); !errors.Is(err, ErrStoredValueOverflow) {
		t.Fatalf("KiemTraGiaTriDaLuu âm = %v", err)
	}
	if err := ValidateStoredValue(ValueMax); err != nil {
		t.Fatalf("KiemTraGiaTriDaLuu đúng trần = %v", err)
	}
}

func TestBalancePastCeilingIsUnavailable(t *testing.T) {
	// Each side within the ceiling, the difference up to twice it — not a figure the browser reads.
	column := func(id string, v ColumnIndicator) BudgetColumn {
		return BudgetColumn{ID: id, Name: id, Format: ColumnFormatNumber, Indicator: v}
	}
	one := func(kind SheetKind, c BudgetColumn, g Dong) FullSheet {
		return FullSheet{
			Sheet:   BudgetSheet{Kind: kind},
			Columns: []BudgetColumn{c},
			Lines:   []BudgetLine{{ID: "t", Name: "Tổng", IsHeadline: true}},
			Values:  map[string]map[string]Dong{"t": {c.ID: g}},
		}
	}
	revenue := one(SheetKindRevenue, column("xh", IndicatorCommuneRetainedRevenue), ValueMax)
	expenditure := one(SheetKindExpenditure, column("cn", IndicatorBudgetExpenditure), -ValueMax)
	if s := RevenueExpenditureBalance(revenue, expenditure); s.Present || !strings.Contains(s.Reason, ErrTotalOverflow.Error()) {
		t.Fatalf("cân đối vượt trần = %+v", s)
	}
	expenditure = one(SheetKindExpenditure, column("cn", IndicatorBudgetExpenditure), 1)
	if s := RevenueExpenditureBalance(revenue, expenditure); !s.Present || s.Value != ValueMax-1 {
		t.Fatalf("cân đối bình thường = %+v", s)
	}
}

func TestValidateChosenMethod(t *testing.T) {
	for _, c := range []LineMethod{MethodManual, MethodEntries} {
		if err := ValidateChosenMethod(c); err != nil {
			t.Errorf("%q bị từ chối: %v", c, err)
		}
	}
	for _, c := range []LineMethod{MethodChildren, "", "MANUAL", "entry"} {
		if err := ValidateChosenMethod(c); !errors.Is(err, ErrMethodFromClient) {
			t.Errorf("%q = %v, muốn ErrCachTinhDoTuClient", c, err)
		}
	}
}

func TestEntryTextLimitsCountRunes(t *testing.T) {
	// `ệ` is 3 bytes: a byte count would refuse text a Vietnamese commune types every day.
	if _, err := NormalizeEntryContent(strings.Repeat("ệ", VoucherDescriptionMax)); err != nil {
		t.Fatalf("đúng trần theo ký tự bị từ chối: %v", err)
	}
	if _, err := NormalizeEntryContent(strings.Repeat("ệ", VoucherDescriptionMax+1)); !errors.Is(err, ErrEntryContentTooLong) {
		t.Fatalf("vượt trần = %v", err)
	}
	if s, err := NormalizeEntryCounterparty("   "); err != nil || s != "" {
		t.Fatalf("đơn vị trống = %q,%v — muốn \"\" (lưu NULL)", s, err)
	}
	if _, err := NormalizeEntryCounterparty(strings.Repeat("ễ", CounterpartyMax+1)); !errors.Is(err, ErrEntryCounterpartyTooLong) {
		t.Fatalf("đơn vị vượt trần = %v", err)
	}
	if _, err := NormalizeEntryVoucherNo(strings.Repeat("ố", VoucherNoMax+1)); !errors.Is(err, ErrEntryVoucherNoTooLong) {
		t.Fatalf("số chứng từ vượt trần = %v", err)
	}
}

func TestEntryDateFollowsWindowOf0008(t *testing.T) {
	for _, tc := range []struct {
		date time.Time
		want error
	}{
		{time.Time{}, ErrEntryDateMissing},
		{time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC), ErrEntryDateOutOfRange},
		{time.Date(2101, 1, 1, 0, 0, 0, 0, time.UTC), ErrEntryDateOutOfRange},
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), nil},
		{time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC), nil},
	} {
		if err := ValidateEntryDate(tc.date); !errors.Is(err, tc.want) {
			t.Errorf("%v = %v, muốn %v", tc.date, err, tc.want)
		}
	}
}
