package http

// GET /api/v1/citizen-letter-report/exports?year= — the Báo cáo tab as an .xlsx (owner spec §5 "Xuất
// Excel"; prototype router.py:371-395, report_sheet.py). Reading, unit names and the audit entry are
// app.CitizenLetterReportExport's, in the order that file states; this file is HTTP and the sheet.
//
// THE SHEET IS THE PROTOTYPE'S FORM, four blocks on one sheet, with ITS wording: `TỔNG QUAN`, `THEO
// LOẠI ĐƠN`, `TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ`, `TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG`. Two departures, each
// from a source that outranks the prototype here:
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

// renderLetterReport builds the workbook.
func renderLetterReport(s app.LetterReportSheet) ([]byte, error) {
	rep := s.Report
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Bao cao don thu " + strconv.Itoa(rep.Year)
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	title, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 15}})
	if err != nil {
		return nil, err
	}
	band, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 12}})
	if err != nil {
		return nil, err
	}
	head, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}})
	if err != nil {
		return nil, err
	}
	pctFmt := `0.0"%"`
	pct, err := f.NewStyle(&excelize.Style{CustomNumFmt: &pctFmt})
	if err != nil {
		return nil, err
	}
	dayFmt := "0.0"
	days, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dayFmt})
	if err != nil {
		return nil, err
	}

	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}
	// put writes one value: string → string cell, int → number, *float64 → number or "—".
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
				err = f.SetCellStr(sheet, c, notAvailable)
			} else {
				err = f.SetCellFloat(sheet, c, *x, 1, 64)
			}
		}
		if err == nil && style != 0 {
			err = f.SetCellStyle(sheet, c, c, style)
		}
		return err
	}
	headers := func(row int, labels ...string) error {
		for i, l := range labels {
			if err := put(i+1, row, l, head); err != nil {
				return err
			}
		}
		return nil
	}

	line := 1
	if err := put(1, line, "BÁO CÁO TIẾP NHẬN VÀ XỬ LÝ ĐƠN THƯ NĂM "+strconv.Itoa(rep.Year), title); err != nil {
		return nil, err
	}
	line = 3

	// --- Khối 1: tổng quan — the six figures, prototype wording.
	if err := put(1, line, "TỔNG QUAN", band); err != nil {
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
		{"Tiếp nhận trong năm", rep.Received, 0},
		{"Đã giải quyết", rep.Resolved, 0},
		{"Đang xử lý (gồm cả đơn tồn từ năm trước)", rep.InProgress, 0},
		{"Quá hạn", rep.PastDue, 0},
		{"Tỷ lệ giải quyết đúng hạn", rep.OnTimePercent, pct},
		{"Số ngày xử lý trung bình", rep.AverageDays, days},
	} {
		if err := put(1, line, fig.label, 0); err != nil {
			return nil, err
		}
		if err := put(2, line, fig.value, fig.style); err != nil {
			return nil, err
		}
		line++
	}
	line++

	// --- Khối 2: theo loại đơn — already sorted by total, largest first (domain.BuildLetterReport).
	if err := put(1, line, "THEO LOẠI ĐƠN", band); err != nil {
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
		for i, v := range []any{label, t.Total, t.Resolved, t.InProgress, t.PastDue} {
			if err := put(i+1, line, v, 0); err != nil {
				return nil, err
			}
		}
		line++
	}
	line++

	// --- Khối 3: theo đơn vị — the owner spec's columns (file header), `Chưa phân công` for no unit.
	if err := put(1, line, "TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ", band); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Bộ phận", "Tổng số", "Đang xử lý", "Đã giải quyết", "Quá hạn", "Đúng hạn (%)"); err != nil {
		return nil, err
	}
	line++
	for _, u := range rep.ByUnit {
		for i, v := range []any{letterReportUnitLabel(u.UnitID, s.UnitNames), u.Total, u.InProgress, u.Resolved, u.PastDue} {
			if err := put(i+1, line, v, 0); err != nil {
				return nil, err
			}
		}
		if err := put(6, line, u.OnTimePercent, pct); err != nil {
			return nil, err
		}
		line++
	}
	line++

	// --- Khối 4: theo tháng.
	if err := put(1, line, "TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG", band); err != nil {
		return nil, err
	}
	line++
	if err := headers(line, "Tháng", "Tiếp nhận", "Đã giải quyết"); err != nil {
		return nil, err
	}
	line++
	for _, m := range rep.ByMonth {
		for i, v := range []any{"Tháng " + strconv.Itoa(m.Month), m.Received, m.Resolved} {
			if err := put(i+1, line, v, 0); err != nil {
				return nil, err
			}
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
