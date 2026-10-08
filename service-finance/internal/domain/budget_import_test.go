package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The parser rules, on hand-built cell grids. The same rules on a REAL workbook built with excelize
// (two sheets, merged two-row header) are in internal/http/budget_import_test.go.

// n is a number cell; a Go string is a text cell; nil is an empty cell.
type n float64

func grid(rows ...[]any) [][]BudgetImportCell {
	out := make([][]BudgetImportCell, len(rows))
	for i, r := range rows {
		for _, v := range r {
			switch x := v.(type) {
			case nil:
				out[i] = append(out[i], BudgetImportCell{})
			case n:
				out[i] = append(out[i], BudgetImportCell{Text: fmt.Sprint(float64(x)), Number: true})
			case string:
				out[i] = append(out[i], BudgetImportCell{Text: x})
			}
		}
	}
	return out
}

func expenditureSheet() BudgetImportSheet {
	return BudgetImportSheet{Name: "chi NS", Rows: grid(
		[]any{"BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ DEMO NĂM 2026"},
		[]any{nil, nil, nil, "Đơn vị tính: Triệu đồng"},
		[]any{},
		[]any{"STT", "Chỉ tiêu", "Dự toán năm", "Chi ngân sách", "So sánh TH/DT (%)"},
		[]any{nil, "Tổng số", n(400), n(300), n(75)},
		[]any{"A", "CHI NGÂN SÁCH NHÀ NƯỚC", n(400), n(290)},
		[]any{"I", "    Chi đầu tư phát triển", n(60), n(80)},
		[]any{"I.1", " Đầu tư cho các dự án", nil, n(80)},
		[]any{"1.1", "Chi quốc phòng", n(0), n(0)},
		[]any{"1.2", "Chi giáo dục", n(0), n(20.5)},
		[]any{"II", "Chi thường xuyên", n(340), n(220)},
		[]any{"1", "Chi quốc phòng", n(3), n(2)},
		[]any{"10", "Chi các hoạt động kinh tế", n(44), n(8)},
		[]any{"10.1", "- Chi giao thông vận tải", "-", n(1)},
		[]any{"B", "CHI CHUYỂN GIAO NGÂN SÁCH", nil, n(1)},
		[]any{"1", "    Chi nộp ngân sách cấp trên", nil, n(1)},
	)}
}

func revenueSheet() BudgetImportSheet {
	return BudgetImportSheet{Name: "Phu luc 1", Rows: grid(
		[]any{"UBND XÃ DEMO"},
		[]any{"THU NGÂN SÁCH XÃ DEMO NĂM 2026"},
		[]any{"Đơn vị tính: Đồng"},
		[]any{"TT", "Nội dung", "Dự toán 2026", nil, "Thu ngân sách", nil, "Tỷ lệ % thu"},
		[]any{nil, nil, "TP giao", "Xã giao", "NSNN", "Thu xã hưởng"},
		[]any{"A", "TỔNG THU NỘI ĐỊA", n(100), n(110), n(90), n(70)},
		[]any{"I", "THUẾ THÀNH PHỐ QUẢN LÝ THU", n(40), n(40), n(36), n(30)},
		[]any{"1", "    Thu từ doanh nghiệp nhà nước", n(10), n(10), n(8), n(6)},
		[]any{nil, "        Thuế GTGT", n(6), n(6), n(5), n(4)},
		[]any{nil, "        Thuế TNDN", n(4), n(4), n(3), n(2)},
		[]any{"2", "    Thuế thu nhập cá nhân", n(30), n(30), n(28), n(24)},
		[]any{"B", "THU NGÂN SÁCH ĐỊA PHƯƠNG", n(300), n(300), n(280), n(280)},
	)}
}

func parseOK(t *testing.T, sheets ...BudgetImportSheet) ParsedBudgetWorkbook {
	t.Helper()
	wb, errs := ParseBudgetWorkbook(sheets)
	if len(errs) > 0 {
		t.Fatalf("errors: %+v", errs)
	}
	return wb
}

func byCode(s ImportedSheet) map[string]ImportedLine {
	m := map[string]ImportedLine{}
	for _, l := range s.Lines {
		if _, dup := m[l.TT]; !dup {
			m[l.TT] = l
		}
	}
	return m
}

func TestImport_TwoSheetsKindsTitlesUnits(t *testing.T) {
	wb := parseOK(t, revenueSheet(), expenditureSheet())
	if len(wb.Sheets) != 2 || wb.Sheets[0].Kind != BangThu || wb.Sheets[1].Kind != BangChi {
		t.Fatalf("sheets = %+v", wb.Sheets)
	}
	// The heading line that NAMES the kind is the title, not "UBND XÃ DEMO" above it.
	if wb.Sheets[0].Title != "THU NGÂN SÁCH XÃ DEMO NĂM 2026" || wb.Sheets[0].Unit != DonViDong {
		t.Errorf("revenue title/unit = %q %q", wb.Sheets[0].Title, wb.Sheets[0].Unit)
	}
	// The unit line sits in column D, not in A..C, and is still found.
	if wb.Sheets[1].Unit != DonViTrieuDong {
		t.Errorf("expenditure unit = %q", wb.Sheets[1].Unit)
	}
}

func TestImport_DepthRomanBeforeLetterArabicDotted(t *testing.T) {
	l := parseOK(t, expenditureSheet()).Sheets[0].Lines
	want := []struct {
		code, parent string
		depth        int
	}{
		{"", "", 0}, {"A", "", 0}, {"I", "A", 1}, {"I.1", "I", 2}, {"1.1", "I.1", 3}, {"1.2", "I.1", 3},
		{"II", "A", 1}, {"1", "II", 2}, {"10", "II", 2}, {"10.1", "10", 3}, {"B", "", 0},
		// B jumps straight to an Arabic item: the parent is still B and the tree depth is 1, not 2.
		{"1", "B", 1},
	}
	if len(l) != len(want) {
		t.Fatalf("%d lines, want %d", len(l), len(want))
	}
	for i, w := range want {
		parent := ""
		if l[i].Parent >= 0 {
			parent = l[l[i].Parent].TT
		}
		if l[i].TT != w.code || parent != w.parent || l[i].Depth != w.depth {
			t.Errorf("line %d %q: parent %q depth %d, want parent %q depth %d", i, l[i].TT, parent, l[i].Depth, w.parent, w.depth)
		}
	}
	if l[0].Name != "Tổng số" || l[2].Name != "Chi đầu tư phát triển" {
		t.Errorf("names not trimmed: %q %q", l[0].Name, l[2].Name)
	}
}

func TestImport_IndentFallbackIsRelativeToTheLineAbove(t *testing.T) {
	rev := parseOK(t, revenueSheet()).Sheets[0]
	vat, cit := rev.Lines[3], rev.Lines[4]
	if vat.Name != "Thuế GTGT" || rev.Lines[vat.Parent].TT != "1" || vat.Depth != 3 {
		t.Errorf("GTGT: parent %d depth %d", vat.Parent, vat.Depth)
	}
	// Same indent as the line above: a sibling, not a child of it.
	if cit.Parent != vat.Parent || cit.Depth != 3 {
		t.Errorf("TNDN: parent %d depth %d", cit.Parent, cit.Depth)
	}
}

func TestImport_TwoRowHeaderJoinedAndRolesGuessed(t *testing.T) {
	rev := parseOK(t, revenueSheet()).Sheets[0]
	names := []string{}
	for _, c := range rev.Columns {
		names = append(names, c.Name)
	}
	want := "Dự toán 2026 TP giao|Dự toán 2026 Xã giao|Thu ngân sách NSNN|Thu ngân sách Thu xã hưởng|Tỷ lệ % thu"
	if strings.Join(names, "|") != want {
		t.Fatalf("columns = %q", names)
	}
	roles := []VaiTroCot{VaiTroDuToanTPGiao, VaiTroDuToanXaGiao, VaiTroThuNSNN, VaiTroThuXaHuong, ""}
	for i, r := range roles {
		if rev.Columns[i].Role != r {
			t.Errorf("column %q role %q, want %q", rev.Columns[i].Name, rev.Columns[i].Role, r)
		}
	}
	if rev.Columns[4].Kind != CotPhanTram {
		t.Error("the % column is not a percentage column")
	}
	// FOUR figure columns: the % operands are not identifiable, and that is said, not guessed.
	if got := rev.UnidentifiedPercentColumns(); len(got) != 1 || got[0] != "Tỷ lệ % thu" {
		t.Errorf("unidentified = %q", got)
	}
	if !hasIssue(rev.Warnings, "tử số") {
		t.Errorf("no warning about the unidentified %% column: %+v", rev.Warnings)
	}
	// The data starts AFTER the sub-header: "A" is the first line, not a line built from "TP giao".
	if rev.Lines[0].TT != "A" {
		t.Errorf("first line = %+v", rev.Lines[0])
	}
}

func TestImport_ExpenditurePercentOperandsAndValuesInVnd(t *testing.T) {
	exp := parseOK(t, expenditureSheet()).Sheets[0]
	if exp.Columns[0].Role != VaiTroDuToanNam || exp.Columns[1].Role != VaiTroChiNganSach {
		t.Fatalf("roles = %q %q", exp.Columns[0].Role, exp.Columns[1].Role)
	}
	op := exp.Columns[2].Operands
	if op.Numerator == nil || *op.Numerator != 1 || op.Denominator == nil || *op.Denominator != 0 {
		t.Fatalf("operands = %+v", op)
	}
	// Triệu đồng → đồng; 20.5 triệu = 20 500 000 đồng exactly. The % cell of the file is NOT read.
	g := byCode(exp)["1.2"].Values
	if g[1] != 20_500_000 || g[0] != 0 {
		t.Errorf("1.2 values = %v", g)
	}
	if _, has := exp.Lines[0].Values[2]; has {
		t.Error("the file's percentage was read into a value")
	}
	// "-" is EMPTY, not 0; a blank cell is absent.
	if _, has := byCode(exp)["10.1"].Values[0]; has {
		t.Error(`"-" read as a figure`)
	}
	if _, has := byCode(exp)["I.1"].Values[0]; has {
		t.Error("blank cell read as a figure")
	}
}

// THE HEADLINE IS THE TOP-LEVEL LINE WITH THE LARGEST "ACTUAL" FIGURE AS THE SCREEN WILL SHOW IT. On
// the expenditure sheet: "Tổng số" 300 (a leaf) against A, which sums its children (80+220 = 300, a
// tie) — ties go to the earlier line, so "Tổng số". A's own 290 in the file does not count: a parent sums.
func TestImport_HeadlineIsTheTotalLine(t *testing.T) {
	exp := parseOK(t, expenditureSheet()).Sheets[0]
	if exp.Headline != 0 || exp.Lines[0].Name != "Tổng số" {
		t.Fatalf("headline = %d", exp.Headline)
	}
	if !hasIssue(exp.Warnings, "Đã tự đánh dấu dòng tổng") {
		t.Errorf("the automatic star is not reported: %+v", exp.Warnings)
	}
	rev := parseOK(t, revenueSheet()).Sheets[0]
	// A sums I (which sums 1 and 2 …) = 5+3+28 = 36 in NSNN; B = 280 → B.
	if rev.Lines[rev.Headline].TT != "B" {
		t.Errorf("revenue headline = %q", rev.Lines[rev.Headline].TT)
	}
}

func TestImport_UnitMissingOrUnknownIsRefused(t *testing.T) {
	for _, unit := range []string{"", "Đơn vị tính: Tỷ đồng"} {
		s := expenditureSheet()
		s.Rows[1] = grid([]any{nil, nil, nil, unit})[0]
		if _, errs := ParseBudgetWorkbook([]BudgetImportSheet{s}); !hasIssue(errs, "đơn vị tính") {
			t.Errorf("unit %q: errs = %+v", unit, errs)
		}
	}
}

func TestImport_KindUndecidableIsRefusedNotDefaulted(t *testing.T) {
	s := expenditureSheet()
	s.Name = "Sheet1"
	s.Rows[0] = grid([]any{"BÁO CÁO NGÂN SÁCH NĂM 2026"})[0]
	if _, errs := ParseBudgetWorkbook([]BudgetImportSheet{s}); !hasIssue(errs, "THU hay bảng CHI") {
		t.Fatalf("errs = %+v", errs)
	}
	// "thuế" and "chính" are not the words "thu" / "chi".
	s.Rows[0] = grid([]any{"BÁO CÁO THUẾ PHÒNG TÀI CHÍNH"})[0]
	if _, errs := ParseBudgetWorkbook([]BudgetImportSheet{s}); !hasIssue(errs, "THU hay bảng CHI") {
		t.Fatalf("thuế/chính decided a kind: %+v", errs)
	}
}

func TestImport_TwoSheetsOfOneKindRefused(t *testing.T) {
	if _, errs := ParseBudgetWorkbook([]BudgetImportSheet{expenditureSheet(), expenditureSheet()}); !hasIssue(errs, "hai sheet cùng là bảng chi") {
		t.Fatalf("errs = %+v", errs)
	}
}

func TestImport_NoTableAnywhereRefused_NotesAndHiddenSkipped(t *testing.T) {
	note := BudgetImportSheet{Name: "Ghi chú", Rows: grid([]any{"Ghi chú của kế toán"})}
	if _, errs := ParseBudgetWorkbook([]BudgetImportSheet{note}); len(errs) != 1 {
		t.Fatalf("errs = %+v", errs)
	}
	hidden := expenditureSheet()
	hidden.Hidden = true
	wb := parseOK(t, revenueSheet(), note, hidden)
	if len(wb.Sheets) != 1 || !hasIssue(wb.Warnings, "đang ẩn") || !hasIssue(wb.Warnings, "không có dòng tiêu đề") {
		t.Fatalf("wb = %+v", wb)
	}
}

func TestImport_BadNumberRefusedWithRowAndColumn(t *testing.T) {
	s := expenditureSheet()
	s.Rows[8][3] = BudgetImportCell{Text: "khoảng 5"}
	_, errs := ParseBudgetWorkbook([]BudgetImportSheet{s})
	if len(errs) != 1 || errs[0].Row != 9 || errs[0].Column != "Chi ngân sách" || strings.Contains(errs[0].Message, "khoảng") {
		t.Fatalf("errs = %+v", errs)
	}
}

func TestParseBudgetAmount(t *testing.T) {
	cases := []struct {
		cell    BudgetImportCell
		factor  int64
		want    Dong
		present bool
		err     error
	}{
		{BudgetImportCell{Text: "1.234", Number: true}, 1_000_000, 1_234_000, true, nil},
		{BudgetImportCell{Text: "1.234"}, 1, 0, false, errAmountAmbiguous},
		{BudgetImportCell{Text: "1,234"}, 1, 0, false, errAmountAmbiguous},
		{BudgetImportCell{Text: "1.234,5"}, 1_000, 1_234_500, true, nil},
		{BudgetImportCell{Text: "1,234.5"}, 1_000, 1_234_500, true, nil},
		{BudgetImportCell{Text: "12,5"}, 1_000_000, 12_500_000, true, nil},
		{BudgetImportCell{Text: "-3"}, 1, -3, true, nil},
		{BudgetImportCell{Text: "0.0000005", Number: true}, 1_000_000, 1, true, nil}, // half away from zero
		{BudgetImportCell{Text: "-0.0000005", Number: true}, 1_000_000, -1, true, nil},
		{BudgetImportCell{Text: "1.5E+3", Number: true}, 1, 1500, true, nil},
		{BudgetImportCell{Text: "0", Number: true}, 1, 0, true, nil}, // a stated zero is a figure
		{BudgetImportCell{Text: " - "}, 1, 0, false, nil},
		{BudgetImportCell{Text: "1/2"}, 1, 0, false, errAmountUnreadable},
		{BudgetImportCell{Text: "1E999999999", Number: true}, 1, 0, false, errAmountUnreadable},
		{BudgetImportCell{Text: "99999999999", Number: true}, 1_000_000, 0, false, errAmountTooLarge},
	}
	for _, c := range cases {
		got, present, err := parseBudgetAmount(c.cell, c.factor)
		if got != c.want || present != c.present || !errors.Is(err, c.err) {
			t.Errorf("%+v ×%d = %d %v %v, want %d %v %v", c.cell, c.factor, got, present, err, c.want, c.present, c.err)
		}
	}
}

func TestImportedColumnSet_UnidentifiedPercentAllowedOnlyOnImport(t *testing.T) {
	rev := parseOK(t, revenueSheet()).Sheets[0]
	cols, err := ImportedColumnSet(rev, "b1", []string{"c1", "c2", "c3", "c4", "c5"})
	if err != nil {
		t.Fatalf("ImportedColumnSet: %v", err)
	}
	if cols[4].CongThuc == "" || cols[4].NumeratorColumnID != "" {
		t.Errorf("percent column = %+v", cols[4])
	}
	// The hand-created path still refuses the same set.
	if err := KiemTraBoCot(BangThu, cols); !errors.Is(err, ErrPercentOperandsMissing) {
		t.Errorf("KiemTraBoCot = %v — the create route must keep refusing a %% column without operands", err)
	}
	// And the read path says WHY, with the import's own sentence.
	b := BangDayDu{Bang: BangNganSach{NguonTep: "bao-cao.xlsx"}, Cot: cols}
	if got := b.PercentCell("l", "c5"); got.LyDo != ErrPercentOperandsNotIdentified.Error() {
		t.Errorf("reason = %q", got.LyDo)
	}
}

func TestImportLockingClose_YearOrAnyMonth(t *testing.T) {
	month := BudgetPeriodClose{Code: "CK-2026-03-01", Year: 2026, Month: 3}
	year := BudgetPeriodClose{Code: "CK-2026-CN-01", Year: 2026}
	other := BudgetPeriodClose{Code: "CK-2025-CN-01", Year: 2025}
	if _, locked := ImportLockingClose([]BudgetPeriodClose{other}, 2026); locked {
		t.Error("another year's close locked the import")
	}
	if c, locked := ImportLockingClose([]BudgetPeriodClose{month}, 2026); !locked || c.Code != month.Code {
		t.Error("a closed MONTH did not lock the import")
	}
	if c, _ := ImportLockingClose([]BudgetPeriodClose{month, year}, 2026); c.Code != year.Code {
		t.Error("the year close is not named first")
	}
	err := ImportPeriodClosedError(month)
	if !errors.Is(err, ErrPeriodClosed) || !strings.Contains(err.Error(), "CK-2026-03-01") {
		t.Errorf("err = %v", err)
	}
}

func TestHandEntriesError_NamesLinesNotFigures(t *testing.T) {
	err := HandEntriesError(BangNganSach{Ma: "NS-2026-CHI-01", Nam: 2026, Loai: BangChi},
		[]string{"1.1 Chi quốc phòng"}, 3, 2)
	if !errors.Is(err, ErrBudgetSheetHasHandEntries) {
		t.Fatal("not the sentinel")
	}
	for _, want := range []string{"NS-2026-CHI-01", "3 đợt", "1.1 Chi quốc phòng", "2 ô số", "Gỡ bảng trước"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %q", want, err.Error())
		}
	}
}

func TestNormaliseSourceFileName(t *testing.T) {
	if got := NormaliseSourceFileName(`C:\Users\ke toan\bao-cao 2026.xlsx`); got != "bao-cao 2026.xlsx" {
		t.Errorf("got %q", got)
	}
	if got := NormaliseSourceFileName("\x00\n"); got == "" {
		t.Error("empty name kept empty")
	}
}

func hasIssue(issues []BudgetImportIssue, part string) bool {
	for _, i := range issues {
		if strings.Contains(strings.ToLower(i.Message), strings.ToLower(part)) {
			return true
		}
	}
	return false
}
