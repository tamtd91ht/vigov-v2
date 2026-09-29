package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// WHAT THIS FILE IS FOR: the arithmetic and the refusals of the budget board, checked against
// docs/ui-ux/07-thu-chi-ngan-sach.md's OWN SAMPLE FIGURES rather than against numbers invented here.
//
// A test written from made-up numbers proves the code agrees with itself. The specification prints
// 108,1% and 91,3% beside the inputs that produce them (§3.1, §3.2, §10), so those are the figures
// below — and ADR 0035 §A turns on one of them being reachable only from one of the two candidate
// denominators.
//
// THE UNIT. Every stored figure is in ĐỒNG (migration 0006's MONEY block); the sheet DISPLAYS
// "Triệu đồng". `million` converts one printed figure to what is stored, so a reader can compare the
// constants below with the specification line by line.

// million turns a figure printed in triệu đồng — with at most one decimal, as §3 prints them — into
// đồng. TAKEN AS TENTHS rather than as a float: 3_463_459.2 cannot be written exactly in binary
// floating point, and the whole point of Dong being an integer is that no figure on this screen ever
// passes through one.
func million(tenths int64) Dong { return Dong(tenths * 100_000) }

const (
	// §3.1, the chi tab.
	expenditureAnnualEstimate = 37_947_400 // 3.794.740,0
	budgetExpenditureValue    = 34_634_592 // 3.463.459,2
	// §3.2, the thu tab.
	revenueEstimateByProvince   = 39_930_100 // 3.993.010,0
	revenueEstimateByCommune    = 46_812_900 // 4.681.290,0
	stateBudgetRevenueValue     = 43_167_643 // 4.316.764,3
	communeRetainedRevenueValue = 33_008_005 // 3.300.800,5
)

// --- fixtures -------------------------------------------------------------------------------------

// sampleExpenditureSheet is the chi sheet of §3.1 and §10, cut down to what the figures need: `Tổng số` as the
// MARKED total row, and `A. CHI NGÂN SÁCH NHÀ NƯỚC` beside it as a SIBLING.
//
// THE SIBLING IS THE WHOLE POINT OF THE FIXTURE, not decoration. §10 describes the chi sheet as
// `Tổng số` → `A. CHI NGÂN SÁCH NHÀ NƯỚC`, and `../vigov-require` measured that those two are in
// fact at the SAME level: adding every top-level row counts the same money twice. A fixture with one
// top-level row could not tell a marked-row lookup from "sum the roots" or from "take the first one".
func sampleExpenditureSheet() FullSheet {
	return FullSheet{
		Sheet: BudgetSheet{ID: "b-chi", Code: "NS-2026-CHI-01", Year: 2026, Kind: SheetKindExpenditure, Revision: 1},
		Columns: []BudgetColumn{
			{ID: "c-dt", SheetID: "b-chi", Name: "Dự toán năm", SortOrder: 1, Format: ColumnFormatNumber, Indicator: IndicatorAnnualEstimate},
			{ID: "c-chi", SheetID: "b-chi", Name: "Chi ngân sách", SortOrder: 2, Format: ColumnFormatNumber, Indicator: IndicatorBudgetExpenditure},
			{ID: "c-ty", SheetID: "b-chi", Name: "So sánh TH/DT (%)", SortOrder: 3, Format: ColumnFormatPercent,
				Formula: "col_2 / col_1 * 100"},
		},
		Lines: []BudgetLine{
			{ID: "k-tong", SheetID: "b-chi", OrdinalLabel: "", Name: "Tổng số", SortOrder: 1,
				Method: MethodManual, IsHeadline: true},
			{ID: "k-a", SheetID: "b-chi", OrdinalLabel: "A", Name: "CHI NGÂN SÁCH NHÀ NƯỚC", SortOrder: 2,
				Method: MethodManual},
		},
		Values: map[string]map[string]Dong{
			"k-tong": {"c-dt": million(expenditureAnnualEstimate), "c-chi": million(budgetExpenditureValue)},
			// A. carries FIGURES OF ITS OWN, larger than the total row's (§4.1 prints 5.502.660 /
			// 3.401.673,3 for it). Anything that summed top-level rows would produce a figure larger
			// than the sheet's own total — which is exactly the double count being guarded against.
			"k-a": {"c-dt": million(55_026_600), "c-chi": million(34_016_733)},
		},
	}
}

// sampleRevenueSheet is the thu sheet of §3.2 and §10: `A. TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN` with
// `B. THU NGÂN SÁCH ĐỊA PHƯƠNG` NESTED under it — the second shape `../vigov-require` measured.
//
// THE MARKED ROW IS `A`, and all four columns of §3.2 are present because ADR 0035 turns on the
// DIFFERENCE between two of them: `Thu ngân sách NSNN` (4.316.764,3) and `Thu xã hưởng`
// (3.300.800,5). A fixture carrying only one could not tell #32 from its opposite.
func sampleRevenueSheet() FullSheet {
	return FullSheet{
		Sheet: BudgetSheet{ID: "b-thu", Code: "NS-2026-THU-01", Year: 2026, Kind: SheetKindRevenue, Revision: 1},
		Columns: []BudgetColumn{
			{ID: "c-tp", SheetID: "b-thu", Name: "Dự toán 2026 TP giao", SortOrder: 1, Format: ColumnFormatNumber,
				Indicator: IndicatorEstimateAssignedByProvince},
			{ID: "c-xa", SheetID: "b-thu", Name: "Dự toán 2026 Xã giao", SortOrder: 2, Format: ColumnFormatNumber,
				Indicator: IndicatorEstimateAssignedByCommune},
			{ID: "c-nsnn", SheetID: "b-thu", Name: "Thu ngân sách NSNN", SortOrder: 3, Format: ColumnFormatNumber,
				Indicator: IndicatorStateBudgetRevenue},
			{ID: "c-huong", SheetID: "b-thu", Name: "Thu ngân sách Thu xã hưởng", SortOrder: 4, Format: ColumnFormatNumber,
				Indicator: IndicatorCommuneRetainedRevenue},
		},
		Lines: []BudgetLine{
			{ID: "k-a", SheetID: "b-thu", OrdinalLabel: "A", Name: "TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN",
				SortOrder: 1, Method: MethodManual, IsHeadline: true},
		},
		Values: map[string]map[string]Dong{
			"k-a": {
				"c-tp":    million(revenueEstimateByProvince),
				"c-xa":    million(revenueEstimateByCommune),
				"c-nsnn":  million(stateBudgetRevenueValue),
				"c-huong": million(communeRetainedRevenueValue),
			},
		},
	}
}

// --- (1) the marked row, and nothing else ----------------------------------------------------------

func TestHeadlineIsMarkedRowNotFirstRow(t *testing.T) {
	// THE ASSERTION THAT MATTERS IS THE SECOND ONE. The chi fixture marks `Tổng số`, which happens to
	// be first; the thu fixture below marks a row that is not, and the mutation this file is written
	// against is "take the first row" — see TestHeadlineIsNotInferredFromRowOrder.
	b := sampleExpenditureSheet()
	headline, err := b.Headline()
	if err != nil {
		t.Fatalf("DongTong lỗi: %v", err)
	}
	if headline.ID != "k-tong" {
		t.Fatalf("dòng tổng = %q, muốn %q", headline.ID, "k-tong")
	}
}

func TestHeadlineIsNotInferredFromRowOrder(t *testing.T) {
	// THE MUTATION GUARD. `is_headline` exists because "mặc định dòng đầu tiên" (§5 rule 5) is right
	// on one form and wrong on the next, with nothing reporting it. Here the FIRST row is not the
	// marked one, so any implementation that falls back to ordering — first row, lowest `thu_tu`,
	// shallowest `cap` — answers `k-a` and this turns red.
	b := sampleExpenditureSheet()
	b.Lines[0].IsHeadline = false
	b.Lines[1].IsHeadline = true

	headline, err := b.Headline()
	if err != nil {
		t.Fatalf("DongTong lỗi: %v", err)
	}
	if headline.ID != "k-a" {
		t.Fatalf("dòng tổng = %q, muốn %q — dòng tổng phải là dòng ĐƯỢC ĐÁNH DẤU, "+
			"không phải dòng đầu tiên/nông nhất", headline.ID, "k-a")
	}
}

func TestNoMarkedRowMeansNOHeadlineValueAndOneSENTENCE(t *testing.T) {
	// ADR 0035 §A: "chưa đủ dữ liệu để ra số thì để trống KÈM LÝ DO, không đặt mặc định". Answering 0
	// here would put a zero into a card leadership reads.
	b := sampleExpenditureSheet()
	for i := range b.Lines {
		b.Lines[i].IsHeadline = false
	}

	if _, err := b.Headline(); !errors.Is(err, ErrHeadlineNotMarked) {
		t.Fatalf("DongTong = %v, muốn ErrChuaDanhDauDongTong", err)
	}

	name, ratio := EstimateAttainment(b)
	if ratio.Present {
		t.Fatalf("%s vẫn ra số %d khi chưa đánh dấu dòng tổng", name, ratio.Value)
	}
	if ratio.Value != 0 || ratio.Reason == "" {
		t.Fatalf("chỉ số phải KHÔNG có giá trị và PHẢI có lý do; nhận Gia=%d LyDo=%q", ratio.Value, ratio.Reason)
	}
	if !strings.Contains(ratio.Reason, "ngôi sao") {
		t.Errorf("lý do không nói cho xã phải làm gì: %q", ratio.Reason)
	}
}

func TestTwoMarkedRowsAreREFUSED_noSumAndNoArbitraryPick(t *testing.T) {
	// THE DOUBLE COUNT, STATED AS A TEST. `../vigov-require` measured that the thu sheet has two
	// nested top-level rows and the chi sheet has `Tổng số` beside A…E; summing marked rows, or
	// silently taking the first of them, is exactly the failure `is_headline` was introduced to end.
	//
	// The application makes the mark a RADIO so this state is unreachable through any route here.
	// It is asserted anyway: the database carries no partial unique index for it (migration 0006 says
	// why it cannot be verified in this environment), so the state can arrive from an import or a
	// psql session, and the read path must refuse rather than answer plausibly.
	b := sampleExpenditureSheet()
	b.Lines[1].IsHeadline = true // now BOTH are marked

	_, err := b.Headline()
	if !errors.Is(err, ErrMultipleHeadlines) {
		t.Fatalf("DongTong = %v, muốn ErrNhieuDongTong", err)
	}

	name, ratio := EstimateAttainment(b)
	if ratio.Present {
		t.Fatalf("%s vẫn ra số %d khi có hai dòng cùng đánh dấu — đó đúng là phép ĐẾM ĐÔI", name, ratio.Value)
	}
	if !strings.Contains(ratio.Reason, "đếm đôi") {
		t.Errorf("lý do không nói ra vì sao từ chối: %q", ratio.Reason)
	}
}

// --- (2) the two figures ADR 0035 §A fixed -----------------------------------------------------------

func TestRevenueAttainmentDividesByEstimateByProvince(t *testing.T) {
	// ADR 0035 #33. The specification contradicts itself: the summary cells of §3.2 show 108,1% and
	// the prose at §9 rule 6 says to divide by `Dự toán Xã giao`, which gives 92,2%. The ADR settles
	// it on the city target — the figure the commune is judged against and the only one comparable
	// between communes.
	//
	// BOTH NUMBERS ARE ASSERTED, the wanted one and the rejected one, because asserting only "10811"
	// would stay green the day somebody made the two denominators the same column.
	name, ratio := EstimateAttainment(sampleRevenueSheet())
	if name != "Thu đạt dự toán" {
		t.Fatalf("tên chỉ số = %q", name)
	}
	if !ratio.Present {
		t.Fatalf("không ra số: %s", ratio.Reason)
	}
	const want = BasisPoints(10811) // 108,11% — §3.2 prints 108,1%
	if ratio.Value != want {
		t.Fatalf("Thu đạt dự toán = %d phần vạn, muốn %d", ratio.Value, want)
	}
	const ifDividedByCommuneEstimate = BasisPoints(9221) // 92,21% — the figure §9 rule 6 would produce
	if ratio.Value == ifDividedByCommuneEstimate {
		t.Fatal("đang chia cho `Dự toán Xã giao` — ADR 0035 #33 chốt `Dự toán TP giao`")
	}
}

func TestExpenditureAttainmentDividesByAnnualEstimate(t *testing.T) {
	name, ratio := EstimateAttainment(sampleExpenditureSheet())
	if name != "Chi đạt dự toán" {
		t.Fatalf("tên chỉ số = %q", name)
	}
	if !ratio.Present {
		t.Fatalf("không ra số: %s", ratio.Reason)
	}
	const want = BasisPoints(9127) // 91,27% — §3.1 prints 91,3%
	if ratio.Value != want {
		t.Fatalf("Chi đạt dự toán = %d phần vạn, muốn %d", ratio.Value, want)
	}
}

func TestBalanceUsesCommuneRetainedRevenueNotStateBudgetRevenue(t *testing.T) {
	// ADR 0035 #32, AND THE SIGN IS THE ASSERTION. On the specification's own figures the commune is
	// SHORT: 3.300.800,5 of retained revenue against 3.463.459,2 of expenditure. Built on
	// `Thu ngân sách NSNN` (4.316.764,3) the same cell would read as a surplus of +853.305,1 — a
	// commune told it has money it may not spend.
	res := RevenueExpenditureBalance(sampleRevenueSheet(), sampleExpenditureSheet())
	if !res.Present {
		t.Fatalf("không ra số: %s", res.Reason)
	}
	want := million(communeRetainedRevenueValue) - million(budgetExpenditureValue)
	if res.Value != want {
		t.Fatalf("Cân đối = %d đồng, muốn %d", res.Value, want)
	}
	if res.Value >= 0 {
		t.Fatalf("Cân đối = %d, phải ÂM trên số liệu mẫu — dấu dương nghĩa là đang lấy "+
			"`Thu ngân sách NSNN` chứ không phải `Thu xã hưởng` (ADR 0035 #32)", res.Value)
	}
	ifStateBudgetRevenue := million(stateBudgetRevenueValue) - million(budgetExpenditureValue)
	if res.Value == ifStateBudgetRevenue {
		t.Fatal("Cân đối đang lấy `Thu ngân sách NSNN` — ADR 0035 #32 chốt `Thu xã hưởng`")
	}
}

func TestEmptyEstimateByProvinceColumnDropsIndicatorWithOneSENTENCE(t *testing.T) {
	// ADR 0035 §A's mandatory consequence: `Dự toán TP giao` becomes a column that MUST have data,
	// and a commune that leaves it out loses the indicator — "và điều đó phải hiện ra thành một CÂU,
	// không thành một ô trống". Never 0, never NaN.
	//
	// BOTH HALVES OF "trống" ARE COVERED: the column is not declared at all, and the column exists but
	// the marked row has no figure in it. They need two different acts from the commune, so they get
	// two different sentences.
	t.Run("không có cột nào mang vai trò", func(t *testing.T) {
		b := sampleRevenueSheet()
		b.Columns = b.Columns[1:] // drop `Dự toán 2026 TP giao`

		name, ratio := EstimateAttainment(b)
		if ratio.Present {
			t.Fatalf("%s vẫn ra %d khi không có cột `Dự toán TP giao`", name, ratio.Value)
		}
		if !errors.Is(indicatorError(b), ErrIndicatorNotAssigned) {
			t.Errorf("lý do không phải ErrChuaGanVaiTroCot: %q", ratio.Reason)
		}
	})

	t.Run("có cột nhưng dòng tổng chưa có số", func(t *testing.T) {
		b := sampleRevenueSheet()
		delete(b.Values["k-a"], "c-tp")

		name, ratio := EstimateAttainment(b)
		if ratio.Present {
			t.Fatalf("%s vẫn ra %d khi dòng tổng bỏ trống cột `Dự toán TP giao`", name, ratio.Value)
		}
		if !errors.Is(indicatorError(b), ErrHeadlineValueMissing) {
			t.Errorf("lý do không phải ErrDongTongChuaCoSo: %q", ratio.Reason)
		}
	})
}

// indicatorError re-runs the lookup the indicator performs, so a test can compare against the SENTINEL
// rather than against the rendered sentence. Comparing strings is a test that breaks when somebody
// fixes a typo and stays green when somebody changes the rule.
func indicatorError(b FullSheet) error {
	var denominator ColumnIndicator
	if b.Sheet.Kind == SheetKindRevenue {
		denominator = IndicatorEstimateAssignedByProvince
	} else {
		denominator = IndicatorAnnualEstimate
	}
	_, _, err := b.HeadlineValue(denominator)
	return err
}

func TestZeroDenominatorMeansNoRatio(t *testing.T) {
	// §9 rule 3 for a percentage column: denominator 0 ⇒ shown BLANK. The same holds for an indicator
	// — a division by zero is not 0%, not 100%, and not infinity; it is an absent measurement.
	b := sampleRevenueSheet()
	b.Values["k-a"]["c-tp"] = 0

	_, ratio := EstimateAttainment(b)
	if ratio.Present {
		t.Fatalf("mẫu số 0 mà vẫn ra %d phần vạn", ratio.Value)
	}
	if !strings.Contains(ratio.Reason, "mẫu số") {
		t.Errorf("lý do không nói mẫu số bằng 0: %q", ratio.Reason)
	}
}

func TestMissingOneSheetMeansNoBalanceFigure(t *testing.T) {
	// "Xã chưa nhập bảng chi" and "xã chi 0 đồng" are different statements and only one of them is
	// ever true. A zero here is the second statement made on the evidence of the first.
	res := RevenueExpenditureBalance(sampleRevenueSheet(), FullSheet{})
	if res.Present {
		t.Fatalf("thiếu bảng chi mà Cân đối vẫn ra %d", res.Value)
	}
	if res.Reason == "" {
		t.Fatal("thiếu bảng chi mà không có lý do nào hiện ra")
	}
}

// --- (3) the tree, and the parent that is never typed into ---------------------------------------------

func TestParentLineSumsChildRows_directChildrenOnly(t *testing.T) {
	// §9 rule 2: a `children` line sums its DIRECT children only. Each child may itself be a
	// `children` line that has already summed its own, so reaching down to the grandchildren counts
	// every level below twice.
	b := FullSheet{
		Sheet:   BudgetSheet{ID: "b", Kind: SheetKindExpenditure},
		Columns: []BudgetColumn{{ID: "c", SheetID: "b", Name: "Chi ngân sách", Format: ColumnFormatNumber}},
		Lines: []BudgetLine{
			{ID: "ong", SheetID: "b", Name: "A", SortOrder: 1},
			{ID: "cha", SheetID: "b", ParentID: "ong", Name: "I", SortOrder: 2, Level: 1},
			{ID: "con1", SheetID: "b", ParentID: "cha", Name: "1.1", SortOrder: 3, Level: 2},
			{ID: "con2", SheetID: "b", ParentID: "cha", Name: "1.2", SortOrder: 4, Level: 2},
		},
		Values: map[string]map[string]Dong{
			"con1": {"c": 30},
			"con2": {"c": 12},
		},
	}

	if g, ok := valueAndPresent(t, b, "cha", "c"); !ok || g != 42 {
		t.Fatalf("cha = (%d,%v), muốn (42,true)", g, ok)
	}
	// The grandparent has ONE direct child, whose value is 42. A walk that also added the
	// grandchildren would produce 84.
	if g, ok := valueAndPresent(t, b, "ong", "c"); !ok || g != 42 {
		t.Fatalf("ông = (%d,%v), muốn (42,true) — 84 nghĩa là đang cộng cả cháu", g, ok)
	}
}

func TestParentWithNoFilledChildIsEMPTYNotZero(t *testing.T) {
	// §9 rule 4: empty shows `—`, never `0`. A section the commune has not entered yet must not be
	// reported as a section it spent nothing on.
	b := FullSheet{
		Sheet:   BudgetSheet{ID: "b", Kind: SheetKindExpenditure},
		Columns: []BudgetColumn{{ID: "c", SheetID: "b", Format: ColumnFormatNumber}},
		Lines: []BudgetLine{
			{ID: "cha", SheetID: "b", Name: "I", SortOrder: 1},
			{ID: "con", SheetID: "b", ParentID: "cha", Name: "1.1", SortOrder: 2, Level: 1},
		},
		Values: map[string]map[string]Dong{},
	}
	if g, ok := valueAndPresent(t, b, "cha", "c"); ok {
		t.Fatalf("cha = (%d,true), muốn TRỐNG", g)
	}
}

func TestZEROIsARealValueNotAnEmptyCell(t *testing.T) {
	// The other side of the same rule, and the reason `Value` is a map of present keys rather than a
	// map with a zero default: a commune that typed 0 said something.
	b := FullSheet{
		Sheet:   BudgetSheet{ID: "b", Kind: SheetKindExpenditure},
		Columns: []BudgetColumn{{ID: "c", SheetID: "b", Format: ColumnFormatNumber}},
		Lines:   []BudgetLine{{ID: "k", SheetID: "b", Name: "1.1", SortOrder: 1}},
		Values:  map[string]map[string]Dong{"k": {"c": 0}},
	}
	if g, ok := valueAndPresent(t, b, "k", "c"); !ok || g != 0 {
		t.Fatalf("ô ghi 0 đọc ra (%d,%v), muốn (0,true)", g, ok)
	}
}

func TestTypingIntoParentLineIsRefused(t *testing.T) {
	// The customer's decision of 06/09/2026, enforced in the business layer.
	if err := CanWriteValue(true); !errors.Is(err, ErrParentLineNoDirectValue) {
		t.Fatalf("ChoGhiGiaTri(cóCon) = %v, muốn ErrKhoanMucChaKhongGoThang", err)
	}
	if err := CanWriteValue(false); err != nil {
		t.Fatalf("ChoGhiGiaTri(lá) = %v, muốn nil", err)
	}
}

func TestMethodDerivesFromTreeNotFromClient(t *testing.T) {
	if got := MethodFromTree(true); got != MethodChildren {
		t.Errorf("có con -> %q, muốn %q", got, MethodChildren)
	}
	if got := MethodFromTree(false); got != MethodManual {
		t.Errorf("không con -> %q, muốn %q", got, MethodManual)
	}
}

func TestParentChildCycleDoesNotHangRead(t *testing.T) {
	// Nothing in the database stops a cycle in `cha_id` (migration 0006 states that cost), so a read
	// has to survive one. A wrong figure is something the screen can show; a request that never
	// returns is a worker held until the client gives up.
	b := FullSheet{
		Sheet:   BudgetSheet{ID: "b", Kind: SheetKindExpenditure},
		Columns: []BudgetColumn{{ID: "c", SheetID: "b", Format: ColumnFormatNumber}},
		Lines: []BudgetLine{
			{ID: "x", SheetID: "b", ParentID: "y", Name: "X", SortOrder: 1},
			{ID: "y", SheetID: "b", ParentID: "x", Name: "Y", SortOrder: 2},
		},
		Values: map[string]map[string]Dong{},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Value("x", "c")
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("phép đọc treo trên một vòng cha-con")
	}
}

// --- (4) the column declarations -------------------------------------------------------------------

func TestIndicatorMustMatchSheetKind(t *testing.T) {
	// A `thu` role on a `chi` sheet is an indicator reading a figure that is not what it is named
	// after. Its own sentence, because the correction is "you are on the wrong tab" rather than
	// "you mistyped".
	if err := ValidateIndicator(SheetKindExpenditure, IndicatorCommuneRetainedRevenue); !errors.Is(err, ErrIndicatorWrongSheetKind) {
		t.Fatalf("thu-xa-huong trên bảng chi = %v, muốn ErrVaiTroSaiLoaiBang", err)
	}
	if err := ValidateIndicator(SheetKindRevenue, "khong-co-vai-tro-nay"); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("vai trò lạ = %v, muốn ErrVaiTroSai", err)
	}
	if err := ValidateIndicator(SheetKindRevenue, IndicatorCommuneRetainedRevenue); err != nil {
		t.Fatalf("thu-xa-huong trên bảng thu = %v, muốn nil", err)
	}
}

func TestTwoColumnsWithOneIndicatorInOneSheetAreRefused(t *testing.T) {
	// Two columns both marked `Thu xã hưởng` is a sheet where the `Cân đối` cell has two candidate
	// answers and no way to choose. The UNIQUE key in migration 0006 is the floor; this is the
	// sentence that arrives first.
	err := ValidateColumnSet(SheetKindRevenue, []BudgetColumn{
		{Name: "Thu xã hưởng", Format: ColumnFormatNumber, Indicator: IndicatorCommuneRetainedRevenue},
		{Name: "Thu xã hưởng (điều chỉnh)", Format: ColumnFormatNumber, Indicator: IndicatorCommuneRetainedRevenue},
	})
	if !errors.Is(err, ErrIndicatorDuplicateInSheet) {
		t.Fatalf("= %v, muốn ErrVaiTroTrungTrongBang", err)
	}
}

func TestPercentColumnCarriesNoIndicatorAndNeedsFormula(t *testing.T) {
	if err := ValidateColumn(SheetKindExpenditure, BudgetColumn{Format: ColumnFormatPercent}); !errors.Is(err, ErrFormulaMissing) {
		t.Errorf("cột %% không công thức = %v, muốn ErrThieuCongThuc", err)
	}
	if err := ValidateColumn(SheetKindExpenditure, BudgetColumn{Format: ColumnFormatNumber, Formula: "a/b"}); !errors.Is(err, ErrFormulaUnexpected) {
		t.Errorf("cột số có công thức = %v, muốn ErrThuaCongThuc", err)
	}
	err := ValidateColumn(SheetKindExpenditure, BudgetColumn{Format: ColumnFormatPercent, Formula: "a/b", Indicator: IndicatorBudgetExpenditure})
	if !errors.Is(err, ErrIndicatorOnPercentColumn) {
		t.Errorf("vai trò trên cột %% = %v, muốn ErrVaiTroTrenCotPhanTram", err)
	}
}

// --- (5) the bounds -----------------------------------------------------------------------------------

func TestNegativeValueIsAccepted(t *testing.T) {
	// §9 rule 4 — and this is the one rule most likely to be "fixed" toward `> 0` by somebody who has
	// just read `chung_tu_giai_ngan`. That constraint belongs to a voucher, where a negative amount is
	// a refund pretending to be a payment (ADR 0035 §B). A budget line legitimately carries one.
	if err := ValidateValue(-5_000_000); err != nil {
		t.Fatalf("giá trị âm bị từ chối: %v", err)
	}
	if err := ValidateValue(ValueMax + 1); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("vượt trần = %v, muốn ErrGiaTriQuaLon", err)
	}
	if err := ValidateValue(-ValueMax - 1); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("vượt trần âm = %v, muốn ErrGiaTriQuaLon", err)
	}
}

func TestRatioOfRoundsToNearestAndDoesNotOverflow(t *testing.T) {
	if got := RatioOf(2, 3); got != 6667 { // 66,666…% -> 66,67%
		t.Errorf("2/3 = %d phần vạn, muốn 6667", got)
	}
	if got := RatioOf(0, 5); got != 0 {
		t.Errorf("0/5 = %d, muốn 0", got)
	}
	if got := RatioOf(5, 0); got != 0 {
		t.Errorf("chia 0 = %d, muốn 0 (người gọi phải hỏi Co trước)", got)
	}
	// The overflow path: a figure at the typo-guard ceiling. `tu * 10000` would be 10^21 and wrap.
	if got := RatioOf(ValueMax, ValueMax); got != 10_000 {
		t.Errorf("trần/trần = %d phần vạn, muốn 10000 (100%%) — số âm ở đây là tràn int64", got)
	}
}

// --- the sheet's unit: a closed list, and legacy text read back without guessing ------------------

func TestUnitWriteAcceptsOnlyTheThreeCodes(t *testing.T) {
	for _, code := range []string{"dong", "nghin-dong", "trieu-dong"} {
		d, err := ValidateUnit(code)
		if err != nil || string(d) != code || d.Label() == "" {
			t.Errorf("%q: = %q, %v — muốn nhận và có nhãn", code, d, err)
		}
	}
	// A LABEL IS REFUSED ON A WRITE even though the read path recognises it: one input vocabulary.
	for _, bad := range []string{"Triệu đồng", "trieu_dong", "TRIEU-DONG", "tỷ đồng", "nghin"} {
		if _, err := ValidateUnit(bad); !errors.Is(err, ErrInvalidUnit) {
			t.Errorf("%q: = %v, muốn ErrDonViTinhSai", bad, err)
		}
	}
	if _, err := ValidateUnit("  "); !errors.Is(err, ErrUnitMissing) {
		t.Errorf("rỗng: = %v, muốn ErrThieuDonViTinh", err)
	}
}

func TestMillionDongLabelMatchesMigration0006Default(t *testing.T) {
	// A legacy row holding the column default and a new row written as `trieu-dong` must be the
	// same bytes, or the column would hold two shapes for one unit.
	if UnitMillionDong.Label() != "Triệu đồng" {
		t.Fatalf("nhãn = %q, muốn đúng mặc định 'Triệu đồng' của migration 0006", UnitMillionDong.Label())
	}
}

func TestParseLegacyUnitDoesNotGuess(t *testing.T) {
	for stored, want := range map[string]SheetUnit{
		"Triệu đồng":      UnitMillionDong,
		"  TRIỆU   ĐỒNG ": UnitMillionDong,
		"trđ":             UnitMillionDong,
		"Nghìn đồng":      UnitThousandDong,
		"ngàn đồng":       UnitThousandDong,
		"1.000 đồng":      UnitThousandDong,
		"Đồng":            UnitDong,
		"VNĐ":             UnitDong,
		"trieu-dong":      UnitMillionDong,
	} {
		if d, ok := ParseStoredUnit(stored); !ok || d != want {
			t.Errorf("%q: = %q/%v, muốn %q", stored, d, ok, want)
		}
	}
	// NOT GUESSED: a scale outside the list, a bare word, a typo. Each would make the whole sheet
	// display a thousand times off if mapped wrongly.
	for _, stored := range []string{"Tỷ đồng", "triệu", "Triệu đòng", "", "USD"} {
		if d, ok := ParseStoredUnit(stored); ok {
			t.Errorf("%q: đoán thành %q — phải báo không nhận ra", stored, d)
		}
	}
}
