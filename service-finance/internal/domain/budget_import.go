package domain

// Loading the revenue / expenditure board straight from the Phòng Tài chính's Excel file
// (docs/ui-ux/07-thu-chi-ngan-sach.md §6; ADR 0081 #6, decided 08/10/2026; prototype
// `../vigov-require/apps/api/app/modules/budget/fiscal_import.py`).
//
// THE FILE IS READ AS THE COMMUNE HAS IT, not converted to a template of ours — the prototype's NT-1:
// the commune already keeps this file, and re-typing a few hundred lines is doing the work twice. So the
// shape is read from the file itself:
//
//	the header row      the first row (of the first 20) whose column A says `TT` or `STT`; a second
//	                    header row under it (merged group headings: "Thu ngân sách" over "NSNN" and
//	                    "Thu xã hưởng") is joined onto it when columns A and B of that row are empty
//	the columns         every column from C on whose heading is not empty; a cell under no heading is
//	                    the preparer's scratch and is dropped (prototype quirk 3)
//	the depth           from the TT code first — Roman BEFORE letter (`I` is a Roman numeral on these
//	                    forms; read as a letter it lifts every section to the top), then Arabic, then
//	                    dotted — and only for a line without a code, from its indent against the line
//	                    above (relative, never absolute: the thu sheet indents 8 spaces for a level-3 line)
//	the parent          the nearest line above with a smaller depth (an ancestor stack) — not "depth
//	                    minus one": the forms skip levels (`B` then straight to `1`)
//	the kind            from the heading lines above the header, then the sheet's tab name
//	the unit            the "Đơn vị tính: …" line above the header — REQUIRED (see parseBudgetAmount)
//
// ADR 0081 #6 OVERRIDES TWO RULES OF thu_chi_ngan_sach.go'S HEADER FOR THIS PATH ONLY: the total row
// is starred automatically (the top-level line with the largest figure in the "actual" column — the
// prototype's pick_headline), and the indicator roles are guessed from the column headings. Both are
// GUESSES and both are reported back as warnings so the person loading the file sees them; the star
// moves with one click afterwards (POST /api/v1/budget-lines/{id}/headline). Columns have no edit route
// (see internal/app's budget file), so a wrongly guessed role is corrected by fixing the heading in the
// file, `Gỡ`, and loading again.
//
// WHAT IS DELIBERATELY NOT TAKEN FROM THE FILE:
//
//	percentages      §9 rule 3 and the user's decision of 30/09/2026: the server computes them from two
//	                 operand columns. The operands are identified from the headings only when the sheet
//	                 has exactly one plan column and one other figure column; otherwise the column is
//	                 kept, and every line of it reads "không tính được" with a sentence. Never guessed.
//	a parent's figure the customer's rule of 06/09/2026: a line with children always sums them, so a
//	                 figure the file states on a parent line is not stored (it could never be shown).
//	                 Where the paper's parent is not the sum of its children (`Trong đó:` lines) the
//	                 screen will differ from the paper — the price the customer accepted.
//
// THIS FILE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4). It never sees the workbook itself:
// internal/http reads it through core/xlsx and hands over text plus a "stored as a number" flag.

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// BudgetImportCell is one spreadsheet cell: its raw text and whether it is STORED AS A NUMBER. The
// flag is what tells a number cell's "1.234" (one point two three four) from the same characters typed
// as text on a Vietnamese machine (one thousand two hundred and thirty-four).
type BudgetImportCell struct {
	Text   string
	Number bool
}

// BudgetImportSheet is one worksheet of the uploaded workbook. Rows[i] is spreadsheet row i+1.
type BudgetImportSheet struct {
	Name   string
	Hidden bool
	Rows   [][]BudgetImportCell
}

// BudgetImportIssue is one error (the file is refused) or one warning (the file loads, and the person
// loading it is told). Row is the spreadsheet row (1-based), 0 for the whole sheet or file; Column is
// the column heading as the file writes it, "" for the whole row.
//
// NO CELL VALUE IS EVER QUOTED IN A MESSAGE. A budget figure is not personal data, but a message has
// no way to know which cell it is looking at, and the reader of these sentences is the same person who
// has the file open.
type BudgetImportIssue struct {
	Sheet   string
	Row     int
	Column  string
	Message string
}

// ImportedColumn is one column of a parsed sheet. Operands are indexes into the SAME sheet's Columns
// (OperandIndexes, the create route's own convention); both nil on a `phan_tram` column means the
// operands could not be identified from the headings.
type ImportedColumn struct {
	Name     string
	Kind     KieuCot
	Role     VaiTroCot
	Operands OperandIndexes
}

// ImportedLine is one line of a parsed sheet, already placed in the tree.
type ImportedLine struct {
	Row    int // spreadsheet row, 1-based
	TT     string
	Name   string
	Depth  int // depth in the TREE (len of the ancestor stack), what `cap` stores
	Parent int // index into ImportedSheet.Lines, -1 at the top level

	// Values is column index -> đồng, only for `so` columns and only for cells the file filled. Empty
	// is ABSENT, never 0 (§9 rule 4).
	Values map[int]Dong
}

// ImportedSheet is one parsed worksheet: one budget sheet ready to be written.
type ImportedSheet struct {
	SheetName string
	Kind      LoaiBang
	Title     string
	Unit      DonViTinh
	Columns   []ImportedColumn
	Lines     []ImportedLine

	// Headline is the index of the line starred automatically, -1 when no top-level line has a figure.
	Headline int

	Warnings []BudgetImportIssue
}

// HasChildren reports whether line i has at least one child.
func (s ImportedSheet) HasChildren(i int) bool {
	for _, l := range s.Lines {
		if l.Parent == i {
			return true
		}
	}
	return false
}

// UnidentifiedPercentColumns lists the `phan_tram` columns whose operands could not be identified.
func (s ImportedSheet) UnidentifiedPercentColumns() []string {
	var out []string
	for _, c := range s.Columns {
		if c.Kind == CotPhanTram && (c.Operands.Numerator == nil || c.Operands.Denominator == nil) {
			out = append(out, c.Name)
		}
	}
	return out
}

// ParsedBudgetWorkbook is a whole file: the sheets that load, and the warnings about what was skipped.
type ParsedBudgetWorkbook struct {
	Sheets   []ImportedSheet
	Warnings []BudgetImportIssue
}

const (
	// budgetImportHeaderSearchRows — the header row is found by its first cell, not by position, within
	// this many rows (the prototype's _HEADER_SEARCH_ROWS).
	budgetImportHeaderSearchRows = 20
	// budgetImportFirstColumn — columns A (TT) and B (content) are the line; figures start at C.
	budgetImportFirstColumn = 2
	// budgetImportMaxLines bounds one sheet, the same ceiling the store refuses to read past
	// (store.TranKhoanMucMotBang). Stated here because domain cannot import the store.
	budgetImportMaxLines = 5000
	// maxNamedEntryLines bounds how many lines a refusal sentence names before "…".
	maxNamedEntryLines = 10
	// SourceFileNameMax bounds `nguon_tep`.
	SourceFileNameMax = 255
)

// ParseBudgetWorkbook reads every sheet that carries a budget table. Errors refuse the whole file
// (nothing is written); warnings travel with a file that loads.
//
// A SHEET WITH NO HEADER ROW IS SKIPPED, with a warning — real files carry a notes tab, and refusing
// a whole report for it would be heavy-handed (the prototype's rule). A HIDDEN sheet is skipped too: a
// hidden tab is typically last year's copy or a scratch calculation, and loading it would be loading a
// table nobody looked at.
func ParseBudgetWorkbook(in []BudgetImportSheet) (ParsedBudgetWorkbook, []BudgetImportIssue) {
	var out ParsedBudgetWorkbook
	var errs []BudgetImportIssue
	for _, s := range in {
		if s.Hidden {
			if !sheetBlank(s) {
				out.Warnings = append(out.Warnings, BudgetImportIssue{Sheet: s.Name,
					Message: "Sheet đang ẩn nên không nạp. Muốn nạp thì bỏ ẩn sheet trong Excel rồi nạp lại."})
			}
			continue
		}
		sheet, ok, sErrs, sWarns := parseBudgetSheet(s)
		errs = append(errs, sErrs...)
		out.Warnings = append(out.Warnings, sWarns...)
		if ok {
			out.Sheets = append(out.Sheets, sheet)
		}
	}
	if len(errs) > 0 {
		return ParsedBudgetWorkbook{}, errs
	}
	if len(out.Sheets) == 0 {
		return ParsedBudgetWorkbook{}, []BudgetImportIssue{{Message: "Không tìm thấy bảng thu, chi nào trong tệp: " +
			"cần một sheet có dòng tiêu đề mà ô cột A ghi \"TT\" hoặc \"STT\", trong 20 dòng đầu."}}
	}
	// ONE SHEET PER KIND. Two `chi` tabs in one file would load one over the other in the same
	// transaction, and which of them the commune meant is not something to choose for it.
	seen := map[LoaiBang]string{}
	for _, s := range out.Sheets {
		if other, dup := seen[s.Kind]; dup {
			errs = append(errs, BudgetImportIssue{Sheet: s.SheetName, Message: fmt.Sprintf(
				"Tệp có hai sheet cùng là bảng %s (\"%s\" và \"%s\") — mỗi tệp chỉ nạp một bảng thu và một bảng chi. "+
					"Xoá hoặc ẩn sheet thừa rồi nạp lại.", kindLabel(s.Kind), other, s.SheetName)})
			continue
		}
		seen[s.Kind] = s.SheetName
	}
	if len(errs) > 0 {
		return ParsedBudgetWorkbook{}, errs
	}
	return out, nil
}

func kindLabel(k LoaiBang) string {
	if k == BangThu {
		return "thu"
	}
	return "chi"
}

func sheetBlank(s BudgetImportSheet) bool {
	for _, r := range s.Rows {
		for _, c := range r {
			if strings.TrimSpace(c.Text) != "" {
				return false
			}
		}
	}
	return true
}

func cellAt(rows [][]BudgetImportCell, r, c int) BudgetImportCell {
	if r < 0 || r >= len(rows) || c < 0 || c >= len(rows[r]) {
		return BudgetImportCell{}
	}
	return rows[r][c]
}

// oneLine turns a cell into one line of text: line breaks inside a cell become spaces and runs of
// whitespace collapse (headings like "Dự toán\nTP giao").
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func parseBudgetSheet(s BudgetImportSheet) (ImportedSheet, bool, []BudgetImportIssue, []BudgetImportIssue) {
	issue := func(row int, col, msg string) BudgetImportIssue {
		return BudgetImportIssue{Sheet: s.Name, Row: row, Column: col, Message: msg}
	}
	var errs, warns []BudgetImportIssue

	header := -1
	for r := 0; r < len(s.Rows) && r < budgetImportHeaderSearchRows; r++ {
		first := strings.ToLower(strings.TrimRight(strings.TrimSpace(cellAt(s.Rows, r, 0).Text), "."))
		if first == "tt" || first == "stt" {
			header = r
			break
		}
	}
	if header < 0 {
		if !sheetBlank(s) {
			warns = append(warns, issue(0, "", "Sheet không có dòng tiêu đề (ô cột A ghi \"TT\" hoặc \"STT\" trong 20 dòng đầu) nên không nạp."))
		}
		return ImportedSheet{}, false, nil, warns
	}

	labels, dataStart := columnLabels(s.Rows, header)
	var cols []ImportedColumn
	var srcCol []int // spreadsheet column (0-based) of each kept column
	for c := budgetImportFirstColumn; c < len(labels); c++ {
		if labels[c] == "" {
			continue
		}
		name, err := ChuanHoaTenCot(labels[c])
		if err != nil {
			errs = append(errs, issue(header+1, labels[c], fmt.Sprintf("Tiêu đề cột quá dài (tối đa %d ký tự).", TenCotToiDa)))
			continue
		}
		kind := CotSo
		low := strings.ToLower(name)
		if strings.Contains(low, "%") || strings.Contains(low, "tỷ lệ") || strings.Contains(low, "tỉ lệ") {
			kind = CotPhanTram
		}
		cols = append(cols, ImportedColumn{Name: name, Kind: kind})
		srcCol = append(srcCol, c)
	}
	if len(cols) == 0 {
		warns = append(warns, issue(header+1, "", "Dòng tiêu đề không có cột số nào từ cột C trở đi nên sheet không nạp."))
		return ImportedSheet{}, false, errs, warns
	}
	if len(cols) > SoCotToiDa {
		errs = append(errs, issue(header+1, "", fmt.Sprintf("Bảng có %d cột số, vượt tối đa %d cột.", len(cols), SoCotToiDa)))
		return ImportedSheet{}, false, errs, warns
	}

	// --- above the header: the heading lines and the unit ---
	var headings []string
	unitText, unitFound := "", false
	for r := 0; r < header; r++ {
		for c := 0; c < len(s.Rows[r]); c++ {
			text := oneLine(s.Rows[r][c].Text)
			if text == "" {
				continue
			}
			low := strings.ToLower(text)
			if strings.Contains(low, "đơn vị tính") || strings.HasPrefix(low, "đvt") {
				if !unitFound {
					unitText, unitFound = unitOf(text), true
				}
				continue
			}
			if c <= 2 {
				headings = append(headings, text)
			}
		}
	}

	kind, kindFrom, ok := guessSheetKind(headings, s.Name)
	if !ok {
		errs = append(errs, issue(0, "", "Không nhận ra sheet này là bảng THU hay bảng CHI: tiêu đề phía trên dòng TT và tên sheet "+
			"đều không có (hoặc có cả hai) chữ \"thu\", \"chi\". Đặt tên sheet là \"Thu\" hoặc \"Chi\" rồi nạp lại."))
	}
	title := kindFrom
	if title == "" && len(headings) > 0 {
		title = headings[0]
	}
	if title == "" {
		title = s.Name
	}
	title, err := ChuanHoaTieuDeBang(title)
	if err != nil {
		errs = append(errs, issue(0, "", fmt.Sprintf("Tiêu đề bảng quá dài (tối đa %d ký tự).", TieuDeBangToiDa)))
	}

	// THE UNIT IS REQUIRED AND NEVER GUESSED: every figure is stored in đồng, so reading a "Triệu đồng"
	// file as đồng shrinks the commune's budget a million times — on a screen that looks normal.
	unit, unitOK := DocDonViTinhDaLuu(unitText)
	if !unitFound || !unitOK {
		errs = append(errs, issue(0, "", "Không đọc được đơn vị tính: cần một dòng \"Đơn vị tính: Đồng\" (hoặc Nghìn đồng, "+
			"Triệu đồng) phía trên dòng tiêu đề. Hệ thống không đoán đơn vị, vì đoán sai là lệch cả nghìn lần."))
	}
	factor := unitFactor(unit)

	// --- the lines ---
	var lines []ImportedLine
	lastDepth, lastIndent := 0, 0
	for r := dataStart; r < len(s.Rows); r++ {
		code := oneLine(cellAt(s.Rows, r, 0).Text)
		rawLabel := strings.TrimRightFunc(strings.NewReplacer("\r", " ", "\n", " ").Replace(cellAt(s.Rows, r, 1).Text), unicode.IsSpace)
		label := oneLine(rawLabel)
		if label == "" {
			if rowHasFigure(s.Rows[r], srcCol, cols) {
				// The prototype's rule: figures with no name are leftovers at the foot of a spreadsheet,
				// not a line. Said out loud rather than dropped silently.
				warns = append(warns, issue(r+1, "", "Dòng có số nhưng không có nội dung (cột B trống) nên bỏ qua."))
			}
			continue
		}
		if utf8.RuneCountInString(label) <= 1 {
			// The column-numbering row many forms put under the header ("A | B | 1 | 2 | 3").
			warns = append(warns, issue(r+1, "", "Dòng đánh số cột (nội dung chỉ một ký tự) nên bỏ qua."))
			continue
		}
		if len(lines) >= budgetImportMaxLines {
			errs = append(errs, issue(r+1, "", fmt.Sprintf("Bảng vượt %d khoản mục.", budgetImportMaxLines)))
			break
		}
		name, err := ChuanHoaTenKhoanMuc(label)
		if err != nil {
			errs = append(errs, issue(r+1, "", fmt.Sprintf("Nội dung khoản mục quá dài (tối đa %d ký tự).", TenKhoanMucToiDa)))
			continue
		}
		tt, err := ChuanHoaTT(code)
		if err != nil {
			errs = append(errs, issue(r+1, "", fmt.Sprintf("Mã TT quá dài (tối đa %d ký tự).", TTKhoanMucToiDa)))
			continue
		}

		values := map[int]Dong{}
		for i, c := range cols {
			if c.Kind != CotSo {
				continue // §9 rule 3: a percentage is computed, never read from the file
			}
			g, present, err := parseBudgetAmount(cellAt(s.Rows, r, srcCol[i]), factor)
			if err != nil {
				errs = append(errs, issue(r+1, c.Name, err.Error()))
				continue
			}
			if present {
				values[i] = g
			}
		}

		indent := len(rawLabel) - len(strings.TrimLeftFunc(rawLabel, unicode.IsSpace))
		depth, byCode := depthFromCode(tt)
		if !byCode {
			switch {
			case len(lines) == 0:
				depth = 0
			case indent > lastIndent:
				depth = lastDepth + 1
			default:
				depth = lastDepth
			}
		}
		lines = append(lines, ImportedLine{Row: r + 1, TT: tt, Name: name, Depth: depth, Values: values})
		lastDepth, lastIndent = depth, indent
	}
	if len(errs) > 0 {
		return ImportedSheet{}, false, errs, warns
	}
	if len(lines) == 0 {
		warns = append(warns, issue(0, "", "Sheet có dòng tiêu đề nhưng không có khoản mục nào nên không nạp."))
		return ImportedSheet{}, false, nil, warns
	}
	placeInTree(lines)

	out := ImportedSheet{SheetName: s.Name, Kind: kind, Title: title, Unit: unit, Columns: cols, Lines: lines}
	guessColumnRoles(&out)
	guessPercentOperands(&out)
	out.Headline = pickImportHeadline(out)

	for _, name := range out.UnidentifiedPercentColumns() {
		warns = append(warns, issue(0, name, "Không xác định chắc chắn được cột tử số và cột mẫu số của cột phần trăm này từ "+
			"tiêu đề, nên tỷ lệ từng dòng sẽ hiện \"không tính được\" (máy chủ không lấy số % trong tệp và không đoán)."))
	}
	for _, role := range vaiTroCuaLoai[out.Kind] {
		found := false
		for _, c := range out.Columns {
			if c.Role == role {
				found = true
			}
		}
		if !found {
			warns = append(warns, issue(0, "", fmt.Sprintf("Không nhận ra cột %q từ tiêu đề — chỉ số đọc từ cột ấy sẽ hiện câu "+
				"lý do thay vì số. Sửa tiêu đề cột trong tệp rồi Gỡ bảng và nạp lại.", roleLabel(role))))
		}
	}
	if out.Headline >= 0 {
		l := out.Lines[out.Headline]
		warns = append(warns, issue(l.Row, "", fmt.Sprintf("Đã tự đánh dấu dòng tổng là %q (dòng cấp cao nhất có số thực hiện "+
			"lớn nhất). Không đúng thì bấm ngôi sao ở dòng tổng thật để đổi.", strings.TrimSpace(l.TT+" "+l.Name))))
	} else {
		warns = append(warns, issue(0, "", "Không tự chọn được dòng tổng (không dòng cấp cao nhất nào có số). "+
			"Bấm ngôi sao ở dòng tổng sau khi nạp."))
	}
	for i := range warns {
		warns[i].Sheet = s.Name
	}
	out.Warnings = warns
	return out, true, nil, nil
}

// columnLabels reads the header row and, when the row under it is a second header row (columns A and
// B empty, some text from C on that is not a number), joins the two: a group heading merged across
// several columns sits in the LEFTMOST cell only, so a sub-heading under an empty group cell inherits
// the nearest group heading to its left. Returns the labels by spreadsheet column and the first data row.
func columnLabels(rows [][]BudgetImportCell, header int) ([]string, int) {
	top := rows[header]
	width := len(top)
	sub := []BudgetImportCell(nil)
	if header+1 < len(rows) {
		next := rows[header+1]
		if strings.TrimSpace(cellAt(rows, header+1, 0).Text) == "" && strings.TrimSpace(cellAt(rows, header+1, 1).Text) == "" {
			for c := budgetImportFirstColumn; c < len(next); c++ {
				if t := strings.TrimSpace(next[c].Text); t != "" && !next[c].Number {
					if _, err := parseDecimal(strings.ReplaceAll(t, ",", ".")); err != nil {
						sub = next
						break
					}
				}
			}
		}
	}
	if len(sub) > width {
		width = len(sub)
	}
	labels := make([]string, width)
	group := ""
	for c := 0; c < width; c++ {
		t := ""
		if c < len(top) {
			t = oneLine(top[c].Text)
		}
		if sub == nil {
			labels[c] = t
			continue
		}
		s := ""
		if c < len(sub) {
			s = oneLine(sub[c].Text)
		}
		if t != "" {
			group = t
		} else if s != "" && c >= budgetImportFirstColumn {
			t = group
		}
		labels[c] = strings.TrimSpace(t + " " + s)
	}
	if sub != nil {
		return labels, header + 2
	}
	return labels, header + 1
}

// unitOf is the unit text of a "Đơn vị tính: Triệu đồng" line: what follows the colon, without
// surrounding brackets or punctuation.
func unitOf(line string) string {
	if i := strings.Index(line, ":"); i >= 0 {
		line = line[i+1:]
	} else {
		low := strings.ToLower(line)
		for _, p := range []string{"đơn vị tính", "đvt"} {
			if j := strings.Index(low, p); j >= 0 {
				line = line[j+len(p):]
				break
			}
		}
	}
	return strings.Trim(strings.TrimSpace(line), "()[].;,")
}

func unitFactor(u DonViTinh) int64 {
	switch u {
	case DonViNghinDong:
		return 1_000
	case DonViTrieuDong:
		return 1_000_000
	default:
		return 1
	}
}

// guessSheetKind reads the heading lines, then the sheet's tab name, as WORDS — "thu" and "chi" as
// whole words, so "thuế" and "chính" decide nothing. The first source that names exactly one kind
// decides; a source naming both decides by whichever word comes first ("BÁO CÁO THU, CHI …" is a thu
// heading by the prototype's rule). Returns the heading line that decided ("" when the tab name did).
// NOTHING DECIDES → not ok: the prototype falls back to `thu`, which is a guess this file refuses.
func guessSheetKind(headings []string, sheetName string) (LoaiBang, string, bool) {
	for i, text := range append(append([]string{}, headings...), sheetName) {
		thu, chi := -1, -1
		for j, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
			if w == "thu" && thu < 0 {
				thu = j
			}
			if w == "chi" && chi < 0 {
				chi = j
			}
		}
		from := ""
		if i < len(headings) {
			from = text
		}
		switch {
		case thu >= 0 && (chi < 0 || thu < chi):
			return BangThu, from, true
		case chi >= 0:
			return BangChi, from, true
		}
	}
	return "", "", false
}

func rowHasFigure(row []BudgetImportCell, srcCol []int, cols []ImportedColumn) bool {
	for i, c := range srcCol {
		if cols[i].Kind != CotSo || c >= len(row) {
			continue
		}
		if _, present, err := parseBudgetAmount(row[c], 1); err == nil && present {
			return true
		}
	}
	return false
}

// depthFromCode is the depth a TT code states, by the forms' own convention: a letter is a part (0), a
// Roman numeral a section (1, or 2 with a sub-number), an Arabic number an item (2, or 3 dotted). ROMAN
// IS TRIED BEFORE THE LETTER — see the file header. false for no code or a code of another shape
// ("-", "+", "a)"), which then takes its depth from its indent.
func depthFromCode(code string) (int, bool) {
	t := strings.TrimRight(strings.TrimSpace(code), ".")
	if t == "" {
		return 0, false
	}
	main, sub, dotted := strings.Cut(t, ".")
	if dotted && !allDigits(sub) {
		return 0, false
	}
	switch {
	case isRoman(main):
		if dotted {
			return 2, true
		}
		return 1, true
	case !dotted && len(main) == 1 && main[0] >= 'A' && main[0] <= 'Z':
		return 0, true
	case allDigits(main):
		if dotted {
			return 3, true
		}
		return 2, true
	}
	return 0, false
}

func isRoman(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != 'I' && r != 'V' && r != 'X' {
			return false
		}
	}
	return true
}

// placeInTree sets Parent and the TREE depth of every line with an ancestor stack: the parent is the
// nearest line above whose stated depth is smaller. The stored depth is the stack's height, not the
// stated one — a form that skips a level would otherwise be drawn with a jump that looks like a fault.
func placeInTree(lines []ImportedLine) {
	type anc struct{ stated, index int }
	var stack []anc
	for i := range lines {
		stated := lines[i].Depth
		for len(stack) > 0 && stack[len(stack)-1].stated >= stated {
			stack = stack[:len(stack)-1]
		}
		lines[i].Parent = -1
		if len(stack) > 0 {
			lines[i].Parent = stack[len(stack)-1].index
		}
		lines[i].Depth = len(stack)
		stack = append(stack, anc{stated: stated, index: i})
	}
}

func isPlanHeading(low string) bool {
	return strings.Contains(low, "dự toán") || strings.Contains(low, "kế hoạch")
}

func hasWord(low, word string) bool {
	for _, w := range strings.FieldsFunc(low, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if w == word {
			return true
		}
	}
	return false
}

// guessColumnRoles marks the indicator columns from their headings — ADR 0081 #6. A role is given
// ONLY when exactly one column matches it: two candidates are two possible answers, and choosing
// between them is the guess thu_chi_ngan_sach.go's header warns about.
func guessColumnRoles(s *ImportedSheet) {
	cands := map[VaiTroCot][]int{}
	for i, c := range s.Columns {
		if c.Kind != CotSo {
			continue
		}
		low := strings.ToLower(c.Name)
		plan := isPlanHeading(low)
		var role VaiTroCot
		switch s.Kind {
		case BangChi:
			switch {
			case plan:
				role = VaiTroDuToanNam
			case strings.Contains(low, "chi ngân sách") || strings.Contains(low, "thực hiện"):
				role = VaiTroChiNganSach
			}
		case BangThu:
			switch {
			case plan && (hasWord(low, "tp") || strings.Contains(low, "thành phố")):
				role = VaiTroDuToanTPGiao
			case plan && strings.Contains(low, "xã"):
				role = VaiTroDuToanXaGiao
			case !plan && strings.Contains(low, "xã hưởng"):
				role = VaiTroThuXaHuong
			case !plan && (hasWord(low, "nsnn") || strings.Contains(low, "ngân sách nhà nước")):
				role = VaiTroThuNSNN
			}
		}
		if role != "" {
			cands[role] = append(cands[role], i)
		}
	}
	for role, idx := range cands {
		if len(idx) == 1 {
			s.Columns[idx[0]].Role = role
		}
	}
}

// guessPercentOperands identifies a percentage column's operands ONLY when the sheet has exactly one
// plan column and exactly one other figure column: then "thực hiện / dự toán" is the only ratio the
// headings can mean (the chi sheet's "So sánh TH/DT (%)"). Anything else — the thu sheet's four figure
// columns — is left unidentified and reads "không tính được".
func guessPercentOperands(s *ImportedSheet) {
	var plans, others []int
	for i, c := range s.Columns {
		if c.Kind != CotSo {
			continue
		}
		if isPlanHeading(strings.ToLower(c.Name)) {
			plans = append(plans, i)
		} else {
			others = append(others, i)
		}
	}
	if len(plans) != 1 || len(others) != 1 {
		return
	}
	for i := range s.Columns {
		if s.Columns[i].Kind == CotPhanTram {
			num, den := others[0], plans[0]
			s.Columns[i].Operands = OperandIndexes{Numerator: &num, Denominator: &den}
		}
	}
}

// actualColumn is the column the headline is chosen on: the kind's "actual" role, else the first
// figure column that is not a plan, else the first figure column. -1 when the sheet has none.
func actualColumn(s ImportedSheet) int {
	want := VaiTroChiNganSach
	if s.Kind == BangThu {
		want = VaiTroThuNSNN
	}
	first, firstOther := -1, -1
	for i, c := range s.Columns {
		if c.Kind != CotSo {
			continue
		}
		if c.Role == want {
			return i
		}
		if first < 0 {
			first = i
		}
		if firstOther < 0 && !isPlanHeading(strings.ToLower(c.Name)) {
			firstOther = i
		}
	}
	if firstOther >= 0 {
		return firstOther
	}
	return first
}

// AsBoard is the sheet as BangDayDu would hold it once written — synthetic ids ("l<i>", "c<i>"), leaf
// figures only — so the read path's own arithmetic (GiaTri: a parent sums its children) answers what
// the screen will show. Used for the headline pick and the preview.
func (s ImportedSheet) AsBoard() BangDayDu {
	b := BangDayDu{Bang: BangNganSach{Loai: s.Kind}, Gia: map[string]map[string]Dong{}}
	for i, c := range s.Columns {
		b.Cot = append(b.Cot, CotNganSach{ID: "c" + strconv.Itoa(i), Ten: c.Name, ThuTu: i + 1, Kieu: c.Kind, VaiTro: c.Role})
	}
	for i, l := range s.Lines {
		k := KhoanMucNganSach{ID: "l" + strconv.Itoa(i), TT: l.TT, Ten: l.Name, ThuTu: i + 1, Cap: l.Depth,
			CachTinh: CachTinhTheoCay(s.HasChildren(i))}
		if l.Parent >= 0 {
			k.ChaID = "l" + strconv.Itoa(l.Parent)
		}
		b.KhoanMuc = append(b.KhoanMuc, k)
		if k.CachTinh == TinhTheoCon {
			continue
		}
		for ci, g := range l.Values {
			if b.Gia[k.ID] == nil {
				b.Gia[k.ID] = map[string]Dong{}
			}
			b.Gia[k.ID]["c"+strconv.Itoa(ci)] = g
		}
	}
	return b
}

// pickImportHeadline is the prototype's pick_headline: among the TOP-LEVEL lines, the one with the
// largest figure in the actual column as the screen will show it. Ties go to the earlier line. -1 when
// none has a figure. NOT a sum of the top-level lines — those are not additive on these forms (the thu
// sheet's A includes part of B; the chi sheet's "Tổng số" sits beside A…E).
func pickImportHeadline(s ImportedSheet) int {
	col := actualColumn(s)
	if col < 0 {
		return -1
	}
	b := s.AsBoard()
	best, bestVal := -1, Dong(0)
	for i, l := range s.Lines {
		if l.Parent >= 0 {
			continue
		}
		v := b.GiaTri("l"+strconv.Itoa(i), "c"+strconv.Itoa(col))
		if !v.Co {
			continue
		}
		if best < 0 || v.Gia > bestVal {
			best, bestVal = i, v.Gia
		}
	}
	return best
}

func roleLabel(v VaiTroCot) string {
	switch v {
	case VaiTroDuToanTPGiao:
		return "Dự toán TP giao"
	case VaiTroDuToanXaGiao:
		return "Dự toán Xã giao"
	case VaiTroThuNSNN:
		return "Thu NSNN"
	case VaiTroThuXaHuong:
		return "Thu xã hưởng"
	case VaiTroDuToanNam:
		return "Dự toán năm"
	case VaiTroChiNganSach:
		return "Chi ngân sách"
	}
	return string(v)
}

// --- amounts -------------------------------------------------------------------------------------------

var (
	errAmountUnreadable = errors.New("Không đọc được số trong ô này. Ghi số (ví dụ 1234,5) hoặc để trống; \"-\" cũng được hiểu là trống.")
	errAmountAmbiguous  = errors.New("Ô ghi số dưới dạng CHỮ mà chỉ có một loại dấu phân cách theo nhóm ba chữ số, nên không rõ là " +
		"phân cách hàng nghìn hay dấu thập phân. Định dạng ô thành Số (Number) rồi nạp lại.")
	errAmountTooLarge = errors.New("Số trong ô này vượt mức một con số ngân sách cấp xã có thể có (sau khi đổi ra đồng).")
)

// parseBudgetAmount reads one figure cell into đồng.
//
// A NUMBER CELL is its stored digits (dot decimal, maybe an exponent). A TEXT CELL is read the way a
// Vietnamese preparer types it: where both "." and "," appear, the LAST one is the decimal point;
// where only one appears in groups of three ("1.234", "1,234"), the cell is REFUSED as ambiguous —
// the two readings differ a thousand times. "-" and an empty cell are EMPTY, never 0 (§9 rule 4).
// The figure is multiplied by the unit's factor and rounded half away from zero to whole đồng.
func parseBudgetAmount(c BudgetImportCell, factor int64) (Dong, bool, error) {
	t := strings.TrimSpace(strings.ReplaceAll(c.Text, " ", " "))
	if t == "" {
		return 0, false, nil
	}
	var r *big.Rat
	var err error
	if c.Number {
		r, err = parseDecimal(t)
	} else {
		switch t {
		case "-", "–", "—":
			return 0, false, nil
		}
		t = strings.ReplaceAll(t, " ", "")
		dot, comma := strings.LastIndex(t, "."), strings.LastIndex(t, ",")
		switch {
		case dot >= 0 && comma >= 0:
			dec, thou := ",", "."
			if dot > comma {
				dec, thou = ".", ","
			}
			t = strings.ReplaceAll(strings.ReplaceAll(t, thou, ""), dec, ".")
		case comma >= 0:
			if groupedThousands(t, ',') {
				return 0, false, errAmountAmbiguous
			}
			t = strings.ReplaceAll(t, ",", ".")
		case dot >= 0:
			if groupedThousands(t, '.') {
				return 0, false, errAmountAmbiguous
			}
		}
		r, err = parseDecimal(t)
	}
	if err != nil {
		return 0, false, errAmountUnreadable
	}
	r.Mul(r, new(big.Rat).SetInt64(factor))
	num, den := r.Num(), r.Denom()
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	// Round half away from zero: |rem| * 2 >= den moves |q| up by one.
	if new(big.Int).Mul(new(big.Int).Abs(rem), big.NewInt(2)).Cmp(den) >= 0 {
		if num.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	if !q.IsInt64() || KiemTraGiaTri(Dong(q.Int64())) != nil {
		return 0, false, errAmountTooLarge
	}
	return Dong(q.Int64()), true, nil
}

// groupedThousands reports whether s is digits grouped in threes by sep, with at least one separator
// ("1.234", "-12.345.678") — the shape that is ambiguous when sep is the only separator in the cell.
func groupedThousands(s string, sep byte) bool {
	s = strings.TrimPrefix(s, "-")
	parts := strings.Split(s, string(sep))
	if len(parts) < 2 || len(parts[0]) < 1 || len(parts[0]) > 3 || !allDigits(parts[0]) {
		return false
	}
	for _, p := range parts[1:] {
		if len(p) != 3 || !allDigits(p) {
			return false
		}
	}
	return true
}

var errNotDecimal = errors.New("ngan_sach: not a decimal")

// parseDecimal reads [-+]digits[.digits][e[-+]digits] EXACTLY. Its own parser, not big.Rat.SetString
// on the raw text: SetString also accepts "a/b" fractions, and an exponent of a billion would make it
// build a number of a billion digits — a cell is attacker-chosen text. The exponent is bounded to ±30.
func parseDecimal(s string) (*big.Rat, error) {
	if s == "" || len(s) > 64 {
		return nil, errNotDecimal
	}
	mant, exp := s, 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.Atoi(s[i+1:])
		if err != nil || e < -30 || e > 30 {
			return nil, errNotDecimal
		}
		mant, exp = s[:i], e
	}
	body := strings.TrimLeft(mant, "+-")
	if len(mant)-len(body) > 1 {
		return nil, errNotDecimal
	}
	intPart, frac, _ := strings.Cut(body, ".")
	if (intPart == "" && frac == "") || (intPart != "" && !allDigits(intPart)) || (frac != "" && !allDigits(frac)) {
		return nil, errNotDecimal
	}
	r, ok := new(big.Rat).SetString(mant)
	if !ok {
		return nil, errNotDecimal
	}
	if exp != 0 {
		p := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(exp))), nil))
		if exp > 0 {
			r.Mul(r, p)
		} else {
			r.Quo(r, p)
		}
	}
	return r, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// --- writing an imported sheet --------------------------------------------------------------------------

// ImportedColumnSet is the column set to insert for an imported sheet, with ids issued and operands
// resolved to ids. CongThuc is the generated caption, or — on a percentage column whose operands could
// not be identified — a sentence saying so (0006's CHECK keeps `cong_thuc` NOT NULL on such a column;
// it is display text and never parsed).
func ImportedColumnSet(s ImportedSheet, sheetID string, ids []string) ([]CotNganSach, error) {
	if len(ids) != len(s.Columns) {
		return nil, fmt.Errorf("ngan_sach: %d id cho %d cột", len(ids), len(s.Columns))
	}
	cols := make([]CotNganSach, len(s.Columns))
	refs := make([]OperandIndexes, len(s.Columns))
	for i, c := range s.Columns {
		cols[i] = CotNganSach{ID: ids[i], BangID: sheetID, Ten: c.Name, ThuTu: i + 1, Kieu: c.Kind, VaiTro: c.Role}
		refs[i] = c.Operands
	}
	if err := AssignOperandsByIndex(cols, refs); err != nil {
		return nil, err
	}
	for i, r := range refs {
		if cols[i].Kieu != CotPhanTram {
			continue
		}
		if r.Numerator != nil && r.Denominator != nil {
			cols[i].CongThuc = PercentFormula(cols[*r.Numerator].Ten, cols[*r.Denominator].Ten)
		} else {
			cols[i].CongThuc = importedPercentCaption
		}
	}
	return cols, CheckImportedColumnSet(s.Kind, cols)
}

const importedPercentCaption = "Nạp từ Excel — chưa xác định được cột tử số, mẫu số"

// CheckImportedColumnSet is KiemTraBoCot for an imported set, with ONE difference: a percentage column
// may arrive without operands (the unidentified state, read as "không tính được"). Everything else —
// each column's own rules, one column per role, operands that are two distinct live `so` columns of
// this set — holds exactly as for a sheet created by hand.
func CheckImportedColumnSet(loai LoaiBang, cols []CotNganSach) error {
	switch {
	case len(cols) == 0:
		return ErrKhongCoCotNao
	case len(cols) > SoCotToiDa:
		return fmt.Errorf("%w (tối đa %d cột)", ErrQuaNhieuCot, SoCotToiDa)
	}
	seen := map[VaiTroCot]struct{}{}
	resolved := make([]CotNganSach, 0, len(cols))
	for _, c := range cols {
		if err := KiemTraCot(loai, c); err != nil {
			return err
		}
		if c.VaiTro != "" {
			if _, dup := seen[c.VaiTro]; dup {
				return fmt.Errorf("%w (`%s`)", ErrVaiTroTrungTrongBang, c.VaiTro)
			}
			seen[c.VaiTro] = struct{}{}
		}
		if c.Kieu == CotPhanTram && c.NumeratorColumnID == "" && c.DenominatorColumnID == "" {
			continue
		}
		resolved = append(resolved, c)
	}
	return checkOperandSet(resolved)
}

// NormaliseSourceFileName is the file name as `nguon_tep` stores it: the base name only (a browser may
// send a full path), control characters removed, at most SourceFileNameMax runes. Never empty: the
// column must be filled for an imported sheet (0006's `bang_ngan_sach_nguon_tep_du`).
func NormaliseSourceFileName(s string) string {
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > SourceFileNameMax {
		s = string(r[:SourceFileNameMax])
	}
	if s == "" {
		return "(tệp không có tên)"
	}
	return s
}

// --- the refusal when the sheet already holds hand-entered figures ---------------------------------------

// ErrBudgetSheetHasHandEntries — the live sheet the file would replace carries figures somebody
// entered by hand: live batches (đợt thu chi), or cells typed or edited after the sheet was created
// (for a sheet created by hand, any filled cell). ADR 0081 #6 and the user's decision of 30/09/2026:
// REFUSED, and the way out is `Gỡ` — a deliberate act with a reason — never a silent overwrite.
var ErrBudgetSheetHasHandEntries = errors.New(
	"ngan_sach: bảng đã có số liệu nhập tay — Gỡ bảng trước rồi mới nạp lại từ Excel")

// HandEntriesConflict names the sheet and what it holds. No figure, and no batch text (a batch's
// `Đơn vị, cá nhân` is personal data): only counts and the budget lines' own names.
type HandEntriesConflict struct {
	Sheet BangNganSach
	msg   string
}

func (e *HandEntriesConflict) Error() string { return e.msg }
func (e *HandEntriesConflict) Unwrap() error { return ErrBudgetSheetHasHandEntries }

// HandEntriesError builds the refusal. linesWithEntries are the "TT Tên" of the lines that carry live
// batches; batches is their count; cells is the count of hand-entered cells.
func HandEntriesError(b BangNganSach, linesWithEntries []string, batches, cells int) error {
	var parts []string
	if batches > 0 {
		named := linesWithEntries
		more := ""
		if len(named) > maxNamedEntryLines {
			named, more = named[:maxNamedEntryLines], "; …"
		}
		parts = append(parts, fmt.Sprintf("%d đợt thu, chi ở các khoản mục: %s%s", batches, strings.Join(named, "; "), more))
	}
	if cells > 0 {
		parts = append(parts, fmt.Sprintf("%d ô số gõ hoặc sửa tay", cells))
	}
	return &HandEntriesConflict{Sheet: b, msg: fmt.Sprintf(
		"ngan_sach: bảng %s năm %d (mã %s) đã có số liệu nhập tay (%s) nên không nạp đè. "+
			"Gỡ bảng trước (nút Gỡ, kèm lý do) rồi nạp lại tệp Excel.",
		kindLabel(b.Loai), b.Nam, b.Ma, strings.Join(parts, "; "))}
}

// SortedKinds orders a workbook's sheets thu before chi, so the writes — and the locks they take —
// happen in one order whatever order the file's tabs are in.
func SortedKinds(sheets []ImportedSheet) []ImportedSheet {
	out := append([]ImportedSheet(nil), sheets...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind == BangThu && out[j].Kind != BangThu })
	return out
}
