package http

// Row 45: the export is the prototype's FORM (report_sheet.py), not a bare dump — navy title bands
// merged across their table, pale column headers, thin light-blue-grey lines on every table cell, no
// gridlines. Read back through excelize so the assertion is on what Excel would render.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

func TestLetterReportExport_FollowsThePrototypeForm(t *testing.T) {
	b, err := renderLetterReport(app.LetterReportSheet{
		Report:    domain.BuildLetterReport(2026, exportSampleRows(), testNow),
		UnitNames: map[string]app.LetterReportUnitName{"bp-dia-chinh": {Name: "Địa chính", Known: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sheet := f.GetSheetName(0)

	styleOf := func(c string) *excelize.Style {
		t.Helper()
		id, err := f.GetCellStyle(sheet, c)
		if err != nil {
			t.Fatal(err)
		}
		s, err := f.GetStyle(id)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Title row 1, merged, navy bold.
	if s := styleOf("A1"); s.Font == nil || !s.Font.Bold || !strings.EqualFold(s.Font.Color, sheetNavy) {
		t.Fatalf("tiêu đề: %+v", s.Font)
	}
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		t.Fatal(err)
	}
	merged := map[string]string{}
	for _, m := range merges {
		merged[m.GetStartAxis()] = m.GetEndAxis()
	}
	if merged["A1"] != "F1" {
		t.Fatalf("tiêu đề phải gộp A1:F1, được %q", merged["A1"])
	}

	// Band row 3 ("TỔNG QUAN"): white bold on navy, merged across its table.
	band := styleOf("A3")
	if len(band.Fill.Color) == 0 || !strings.EqualFold(band.Fill.Color[0], sheetNavy) || band.Font == nil ||
		!band.Font.Bold || !strings.EqualFold(band.Font.Color, sheetWhite) {
		t.Fatalf("dải tiêu đề khối: fill %+v font %+v", band.Fill, band.Font)
	}
	if merged["A3"] != "B3" {
		t.Fatalf("dải TỔNG QUAN phải gộp A3:B3, được %q", merged["A3"])
	}

	// Header row 4 and a data row 5: thin light-blue-grey border on all four sides.
	for _, c := range []string{"A4", "B4", "A5", "B5"} {
		s := styleOf(c)
		if len(s.Border) != 4 {
			t.Fatalf("%s: %d cạnh viền, muốn 4", c, len(s.Border))
		}
		for _, bd := range s.Border {
			if !strings.EqualFold(bd.Color, sheetLine) || bd.Style != 1 {
				t.Fatalf("%s: viền %+v, muốn mảnh màu %s", c, bd, sheetLine)
			}
		}
	}
	if h := styleOf("A4"); len(h.Fill.Color) == 0 || !strings.EqualFold(h.Fill.Color[0], sheetSurface) {
		t.Fatalf("hàng tiêu đề cột phải có nền %s: %+v", sheetSurface, h.Fill)
	}

	// Every band is merged: the four block titles.
	for _, title := range []string{"TỔNG QUAN", "THEO LOẠI ĐƠN", "TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ", "TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG"} {
		found, err := f.SearchSheet(sheet, title)
		if err != nil || len(found) != 1 {
			t.Fatalf("không thấy đúng một ô %q: %v %v", title, found, err)
		}
		if _, ok := merged[found[0]]; !ok {
			t.Fatalf("dải %q ở %s không được gộp", title, found[0])
		}
	}

	opts, err := f.GetSheetView(sheet, 0)
	if err != nil {
		t.Fatal(err)
	}
	if opts.ShowGridLines == nil || *opts.ShowGridLines {
		t.Fatal("tệp xuất theo mẫu không hiện lưới")
	}
}
