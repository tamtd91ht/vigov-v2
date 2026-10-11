package http

// GET /api/v1/citizen-letter-report/exports?year= — the Báo cáo tab as an .xlsx (owner spec §5 "Xuất
// Excel"; prototype router.py:371-395, report_sheet.py). Reading, unit names and the audit entry are
// app.CitizenLetterReportExport's, in the order that file states; this file is HTTP and the sheet.
//
// THE SHEET IS THE PROTOTYPE'S FORM, four blocks on one sheet, with ITS wording: `TỔNG QUAN`, `THEO
// LOẠI ĐƠN`, `TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ`, `TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG`, and ITS styling
// (renderLetterReport). Two departures, each from a source that outranks the prototype here:
//
//   - The unit table has the owner spec's six columns (§5: Bộ phận · Tổng số · Đang xử lý · Đã giải
//     quyết · Quá hạn · Đúng hạn), the columns GET /api/v1/citizen-letter-report returns — not the
//     prototype's eight: `Chuyển cấp trên` / `Lưu, không thụ lý` per unit are figures this report does
//     not compute, and a file must not carry numbers the screen beside it cannot show.
//   - No commune name under the title: the prototype printed it, but no read of the commune's display
//     name exists in this service (platform owns it), and a name typed here would be a second source.
//
// NUMBERS ARE WRITTEN AS NUMBERS (the prototype's rule: a reader sums and sorts them), absent figures as
// "—". Every text cell is a STRING cell — a unit name beginning with "=" stays text, never a formula.
//
// THE FILE NAME carries the year only: no commune name, no personal data (rule 3, forbidden #4).

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// ExportCitizenLetterReport serves the file.
func (h *Handler) ExportCitizenLetterReport(w http.ResponseWriter, r *http.Request) {
	// `year` IS REQUIRED, as on the JSON route and for its reason: a figure filed under a year the reader
	// never chose.
	year, err := strconv.Atoi(r.URL.Query().Get(reportYearParam))
	if err != nil || year < 2000 || year > 2200 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", errNamKhongHopLe.Error(), "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	file, err := h.d.CitizenLetterExport.Export(r.Context(), year, actor, renderLetterReport)
	if err != nil {
		if errors.Is(err, app.ErrLetterReportNamesUnavailable) {
			h.d.Log.Warn("CẢNH BÁO: không xuất được báo cáo đơn thư vì chưa tra được tên bộ phận",
				"xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusServiceUnavailable, "report_names_unavailable",
				"Chưa tra được tên bộ phận nên chưa xuất được báo cáo. Vui lòng thử lại sau ít phút.", "")
			return
		}
		h.letterError(w, r, "xuất báo cáo", err)
		return
	}
	writeXLSXFile(w, r, h, "bao-cao-don-thu-nam-"+strconv.Itoa(year)+".xlsx", file)
}

// reportYearParam is a named constant, not a literal, for the reason headerContentTypeName gives in
// document_type_import.go (the route guard mistakes a literal lookup for a route).
const reportYearParam = "year"

// letterTypeLabels are the prototype's labels (report_sheet.py:31-36), keyed by this register's codes.
var letterTypeLabels = map[domain.LetterType]string{
	domain.LetterTypeComplaint:    "Khiếu nại",
	domain.LetterTypeDenunciation: "Tố cáo",
	domain.LetterTypeFeedback:     "Kiến nghị, phản ánh",
	domain.LetterTypeRequest:      "Đề nghị",
}

// letterReportNoUnit is the row of letters no unit holds (owner spec §5).
const letterReportNoUnit = "Chưa phân công"

const notAvailable = "—"

// letterReportUnitLabel names one unit row. A removed unit keeps its name, flagged; an id identity does
// not know is said to be unknown — never printed as an id, never left blank.
func letterReportUnitLabel(id string, names map[string]app.LetterReportUnitName) string {
	if id == "" {
		return letterReportNoUnit
	}
	n, ok := names[id]
	if !ok || !n.Known {
		return "Bộ phận không xác định"
	}
	if n.Removed {
		return n.Name + " (đã gỡ khỏi sơ đồ tổ chức)"
	}
	return n.Name
}

// The prototype's palette (report_sheet.py:23-29): navy title and bands, a pale surface under column
// headers, thin light-blue-grey lines around every table cell. Row 45 of the customer's sheet asked for
// exactly this — "title rows need a background colour, lines light blue" — so it is copied, not
// re-chosen: two exports of one register that look different are two forms somebody has to explain.
const (
	sheetNavy    = "102B43"
	sheetSurface = "F2F6F9"
	sheetLine    = "D8E0E8"
	sheetWhite   = "FFFFFF"
)

// letterReportWidest is the column count of the widest table (the unit table): the title spans it.
const letterReportWidest = 6

func sheetBorder() []excelize.Border {
	out := make([]excelize.Border, 0, 4)
	for _, side := range []string{"left", "right", "top", "bottom"} {
		out = append(out, excelize.Border{Type: side, Color: sheetLine, Style: 1})
	}
	return out
}

// letterReportStyles are the workbook's cell styles, one per role.
type letterReportStyles struct {
	title, band, head, label, number, percent, days, missing int
}

func newLetterReportStyles(f *excelize.File) (letterReportStyles, error) {
	var st letterReportStyles
	pctFmt, dayFmt, numFmt := `0.0"%"`, "0.0", "#,##0"
	cellFont := &excelize.Font{Size: 10}
	right := &excelize.Alignment{Horizontal: "right", Vertical: "center"}
	for _, s := range []struct {
		dst   *int
		style *excelize.Style
	}{
		{&st.title, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 15, Color: sheetNavy}}},
		{&st.band, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 12, Color: sheetWhite},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{sheetNavy}},
			Alignment: &excelize.Alignment{Vertical: "center", Indent: 1}}},
		{&st.head, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 10, Color: sheetNavy},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{sheetSurface}},
			Border:    sheetBorder(),
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}}},
		{&st.label, &excelize.Style{Font: cellFont, Border: sheetBorder(),
			Alignment: &excelize.Alignment{Vertical: "center", WrapText: true}}},
		{&st.number, &excelize.Style{Font: cellFont, Border: sheetBorder(), Alignment: right, CustomNumFmt: &numFmt}},
		{&st.percent, &excelize.Style{Font: cellFont, Border: sheetBorder(), Alignment: right, CustomNumFmt: &pctFmt}},
		{&st.days, &excelize.Style{Font: cellFont, Border: sheetBorder(), Alignment: right, CustomNumFmt: &dayFmt}},
		{&st.missing, &excelize.Style{Font: cellFont, Border: sheetBorder(), Alignment: right}},
	} {
		id, err := f.NewStyle(s.style)
		if err != nil {
			return st, err
		}
		*s.dst = id
	}
	return st, nil
}

// renderLetterReport builds the workbook.
//
// THE FORM IS THE PROTOTYPE'S (report_sheet.py): title merged across the sheet, each block under a navy
// band merged across its table, column headers on a pale fill, every table cell bordered, numbers
// right-aligned as numbers, no gridlines. One deliberate difference: the average-days figure keeps one
// decimal ("0.0") where the prototype's "#,##0" rounds 3.5 days to 4 — a figure reported upward must not
// be rounded by its cell format.
func renderLetterReport(s app.LetterReportSheet) ([]byte, error) {
	rep := s.Report
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Bao cao don thu " + strconv.Itoa(rep.Year)
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	noGrid := false
	if err := f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &noGrid}); err != nil {
		return nil, err
	}
	st, err := newLetterReportStyles(f)
	if err != nil {
		return nil, err
	}

	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}
	// put writes one value: string → string cell, int → number, *float64 → number or "—" (which takes
	// the bordered right-aligned text style, never a number format).
	put := func(col, row int, v any, style int) error {
		c := cell(col, row)
		var err error
		switch x := v.(type) {
		case string:
			err = f.SetCellStr(sheet, c, x)
		case int:
			err = f.SetCellInt(sheet, c, int64(x))
		case *float64:
			if x == nil {
				err, style = f.SetCellStr(sheet, c, notAvailable), st.missing
			} else {
				err = f.SetCellFloat(sheet, c, *x, 1, 64)
			}
		}
		if err == nil && style != 0 {
			err = f.SetCellStyle(sheet, c, c, style)
		}
		return err
	}
	// band writes a block title merged across `width` columns, white on navy.
	band := func(row, width int, title string) error {
		if err := put(1, row, title, 0); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell(1, row), cell(width, row), st.band); err != nil {
			return err
		}
		if err := f.MergeCell(sheet, cell(1, row), cell(width, row)); err != nil {
			return err
		}
		return f.SetRowHeight(sheet, row, 22)
	}
	headers := func(row int, labels ...string) error {
		for i, l := range labels {
			if err := put(i+1, row, l, st.head); err != nil {
				return err
			}
		}
		return f.SetRowHeight(sheet, row, 30)
	}
	// dataRow writes one table row: a bordered label, then numbers — a *float64 is a percentage.
	dataRow := func(row int, label string, values ...any) error {
		if err := put(1, row, label, st.label); err != nil {
			return err
		}
		for i, v := range values {
			style := st.number
			if _, isPercent := v.(*float64); isPercent {
				style = st.percent
			}
			if err := put(i+2, row, v, style); err != nil {
				return err
			}
		}
		return nil
	}

	line := 1
	if err := put(1, line, "BÁO CÁO TIẾP NHẬN VÀ XỬ LÝ ĐƠN THƯ NĂM "+strconv.Itoa(rep.Year), st.title); err != nil {
		return nil, err
	}
	if err := f.MergeCell(sheet, cell(1, line), cell(letterReportWidest, line)); err != nil {
		return nil, err
	}
	line = 3

	// --- Khối 1: tổng quan — the six figures, prototype wording.
	if err := band(line, 2, "TỔNG QUAN"); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Chỉ số", "Giá trị"); err != nil {
		return nil, err
	}
	line++
	for _, fig := range []struct {
		label string
		value any
		style int
	}{
		{"Tiếp nhận trong năm", rep.Received, st.number},
		{"Đã giải quyết", rep.Resolved, st.number},
		{"Đang xử lý (gồm cả đơn tồn từ năm trước)", rep.InProgress, st.number},
		{"Quá hạn", rep.PastDue, st.number},
		{"Tỷ lệ giải quyết đúng hạn", rep.OnTimePercent, st.percent},
		{"Số ngày xử lý trung bình", rep.AverageDays, st.days},
	} {
		if err := put(1, line, fig.label, st.label); err != nil {
			return nil, err
		}
		if err := put(2, line, fig.value, fig.style); err != nil {
			return nil, err
		}
		line++
	}
	line++

	// --- Khối 2: theo loại đơn — already sorted by total, largest first (domain.BuildLetterReport).
	if err := band(line, 5, "THEO LOẠI ĐƠN"); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Loại đơn", "Tổng số", "Đã giải quyết", "Đang xử lý", "Quá hạn"); err != nil {
		return nil, err
	}
	line++
	for _, t := range rep.ByType {
		label := letterTypeLabels[t.Type]
		if label == "" {
			label = string(t.Type)
		}
		if err := dataRow(line, label, t.Total, t.Resolved, t.InProgress, t.PastDue); err != nil {
			return nil, err
		}
		line++
	}
	line++

	// --- Khối 3: theo đơn vị — the owner spec's columns (file header), `Chưa phân công` for no unit.
	if err := band(line, letterReportWidest, "TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ"); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Bộ phận", "Tổng số", "Đang xử lý", "Đã giải quyết", "Quá hạn", "Đúng hạn (%)"); err != nil {
		return nil, err
	}
	line++
	for _, u := range rep.ByUnit {
		if err := dataRow(line, letterReportUnitLabel(u.UnitID, s.UnitNames),
			u.Total, u.InProgress, u.Resolved, u.PastDue, u.OnTimePercent); err != nil {
			return nil, err
		}
		line++
	}
	line++

	// --- Khối 4: theo tháng.
	if err := band(line, 3, "TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG"); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Tháng", "Tiếp nhận", "Đã giải quyết"); err != nil {
		return nil, err
	}
	line++
	for _, m := range rep.ByMonth {
		if err := dataRow(line, "Tháng "+strconv.Itoa(m.Month), m.Received, m.Resolved); err != nil {
			return nil, err
		}
		line++
	}

	for i, width := range []float64{40, 14, 16, 16, 14, 16} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return nil, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
