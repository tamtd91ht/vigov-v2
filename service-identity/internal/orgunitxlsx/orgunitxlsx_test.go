package orgunitxlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/xuri/excelize/v2"
)

// buildWorkbook writes an .xlsx in memory whose first sheet holds exactly these rows.
func buildWorkbook(t *testing.T, rows map[int][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for n, r := range rows {
		row := r
		cell, _ := excelize.CoordinatesToCellName(1, n)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatalf("SetSheetRow: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	return buf.Bytes()
}

func headerRow() []any {
	var h []any
	for _, c := range domain.OrgUnitImportColumns() {
		h = append(h, c)
	}
	return h
}

// rewriteZip copies a workbook, letting edit change or drop an entry, and appends extra entries.
func rewriteZip(t *testing.T, src []byte, edit func(name string, b []byte) []byte, extra map[string][]byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		if edit != nil {
			b = edit(f.Name, b)
		}
		w, _ := zw.Create(f.Name)
		_, _ = w.Write(b)
	}
	for name, b := range extra {
		w, _ := zw.Create(name)
		_, _ = w.Write(b)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return out.Bytes()
}

func TestTemplateRoundTrip(t *testing.T) {
	live := []domain.BoPhan{
		{ID: "u1", Ma: "lanh-dao", Ten: "LÃNH ĐẠO"},
		{ID: "u2", Ma: "to-a", Ten: "TỔ A"},
		{ID: "u3", Ma: "to-a-2", Ten: "Tổ a"}, // same name folded: listed by CODE
	}
	b, err := Template(live)
	if err != nil {
		t.Fatalf("Template: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("mở lại mẫu: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 3 || got[0] != SheetData {
		t.Fatalf("các trang: %v — trang dữ liệu phải đứng đầu", got)
	}
	if vis, _ := f.GetSheetVisible(SheetUnits); vis {
		t.Error("trang danh sách bộ phận phải ẩn")
	}
	list, _ := f.GetRows(SheetUnits)
	if len(list) != 3 || list[0][0] != "LÃNH ĐẠO" || list[1][0] != "to-a" || list[2][0] != "to-a-2" {
		t.Errorf("danh sách chọn cha: %v — tên trùng phải ghi bằng mã", list)
	}
	dvs, err := f.GetDataValidations(SheetData)
	if err != nil || len(dvs) != 1 {
		t.Fatalf("danh sách chọn: %v %v", dvs, err)
	}
	if dvs[0].ErrorStyle == nil || *dvs[0].ErrorStyle != "warning" {
		t.Errorf("danh sách chọn phải CẢNH BÁO, không chặn: %+v", dvs[0].ErrorStyle)
	}
	if !strings.Contains(dvs[0].Sqref, "B2:B") {
		t.Errorf("danh sách chọn phải trên cột Thuộc bộ phận: %q", dvs[0].Sqref)
	}

	rows, rowErrs, err := Parse(b)
	if err != nil || len(rowErrs) != 0 {
		t.Fatalf("đọc lại mẫu: %v %+v", err, rowErrs)
	}
	if len(rows) != len(templateSamples) || rows[0].Row != 2 || rows[0].Name != "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ" || rows[0].Order != "1" {
		t.Errorf("dòng mẫu: %+v", rows)
	}
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Name), "một cửa") {
			t.Errorf("mẫu không được có bộ phận Một cửa: %q", r.Name)
		}
	}
}

func TestTemplateWithoutUnitsHasNoDropdown(t *testing.T) {
	b, err := Template(nil)
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(b))
	defer f.Close()
	if dvs, _ := f.GetDataValidations(SheetData); len(dvs) != 0 {
		t.Errorf("xã chưa có bộ phận thì không có danh sách chọn: %+v", dvs)
	}
}

func TestParse_RawValuesRowNumbersAndBlankRows(t *testing.T) {
	b := buildWorkbook(t, map[int][]any{
		1: headerRow(),
		2: {"VĂN PHÒNG", "", "", 1.5}, // a NUMBER cell: its raw digits, never rounded
		// row 3 blank
		4: {"TỔ A", "VĂN PHÒNG", "to-a", 3},
	})
	rows, rowErrs, err := Parse(b)
	if err != nil || len(rowErrs) != 0 {
		t.Fatalf("Parse: %v %+v", err, rowErrs)
	}
	if len(rows) != 2 {
		t.Fatalf("muốn 2 dòng (bỏ dòng trống), nhận %+v", rows)
	}
	if rows[0].Order != "1.5" {
		t.Errorf("ô số 1.5 phải đọc thô là \"1.5\", nhận %q", rows[0].Order)
	}
	if rows[1].Row != 4 || rows[1].Parent != "VĂN PHÒNG" || rows[1].Code != "to-a" || rows[1].Order != "3" {
		t.Errorf("dòng 4 giữ số dòng của bảng tính: %+v", rows[1])
	}
}

// A FORMULA IS NEVER EVALUATED: with no cached value the cell reads empty, not "2".
func TestParse_FormulaNotEvaluated(t *testing.T) {
	f := excelize.NewFile()
	h := headerRow()
	_ = f.SetSheetRow("Sheet1", "A1", &h)
	_ = f.SetCellStr("Sheet1", "A2", "TỔ B")
	_ = f.SetCellFormula("Sheet1", "D2", "1+1")
	buf, _ := f.WriteToBuffer()
	f.Close()

	rows, _, err := Parse(buf.Bytes())
	if err != nil || len(rows) != 1 {
		t.Fatalf("Parse: %v %+v", err, rows)
	}
	if rows[0].Order == "2" {
		t.Error("công thức đã bị tính — không được tính công thức")
	}
}

func TestParse_WrongHeaderIsRowOneError(t *testing.T) {
	b := buildWorkbook(t, map[int][]any{1: {"Họ tên", "Điện thoại"}, 2: {"x", "y"}})
	rows, rowErrs, err := Parse(b)
	if err != nil || rows != nil || len(rowErrs) != 1 || rowErrs[0].Row != 1 {
		t.Fatalf("tiêu đề sai: %v %+v %+v", err, rows, rowErrs)
	}
}

func TestParse_StopsCollectingPastTheCap(t *testing.T) {
	m := map[int][]any{1: headerRow()}
	for i := 0; i < domain.MaxOrgUnitImportRows+50; i++ {
		m[i+2] = []any{"X"}
	}
	rows, _, err := Parse(buildWorkbook(t, m))
	if err != nil || len(rows) != domain.MaxOrgUnitImportRows+1 {
		t.Fatalf("muốn dừng ở %d dòng, nhận %d (%v)", domain.MaxOrgUnitImportRows+1, len(rows), err)
	}
}

// THE PRE-INSPECTION ITSELF refuses a truncated sheet — not merely excelize failing later. excelize's
// row iterator stops silently on a decoding error, so this guard is the one that matters.
func TestInspect_RefusesTruncatedSheetXML(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}, 3: {"B"}})
	if err := inspect(good); err != nil {
		t.Fatalf("tệp tốt bị từ chối: %v", err)
	}
	cut := rewriteZip(t, good, func(n string, b []byte) []byte {
		if n == "xl/worksheets/sheet1.xml" {
			return b[:len(b)-40]
		}
		return b
	}, nil)
	if err := inspect(cut); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("inspect(XML cắt cụt) = %v, muốn ErrNotXLSX", err)
	}
}

// nested is a WELL-FORMED element nested n deep, so a refusal can only be about depth.
func nested(n int) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` + strings.Repeat("<a>", n) + strings.Repeat("</a>", n))
}

// GO-2026-6088: A DEEPLY NESTED PART IS REFUSED BEFORE EXCELIZE DECODES IT — in ANY XML part, including
// the ones this package never reads itself (styles, relationships, drawings) but excelize unmarshals
// on open.
func TestInspect_RefusesDeeplyNestedXMLInEveryPart(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}})
	for _, part := range []string{"xl/worksheets/sheet1.xml", "xl/styles.xml", "xl/_rels/workbook.xml.rels", "xl/drawings/vmlDrawing1.vml"} {
		t.Run(part, func(t *testing.T) {
			deep := rewriteZip(t, good, func(n string, b []byte) []byte {
				if n == part {
					return nested(maxXMLDepth + 1)
				}
				return b
			}, map[string][]byte{})
			if part == "xl/drawings/vmlDrawing1.vml" {
				deep = rewriteZip(t, good, nil, map[string][]byte{part: nested(maxXMLDepth + 1)})
			}
			if err := inspect(deep); !errors.Is(err, ErrNotXLSX) {
				t.Fatalf("inspect(%s lồng %d tầng) = %v, muốn ErrNotXLSX", part, maxXMLDepth+1, err)
			}
			if _, _, err := Parse(deep); !errors.Is(err, ErrNotXLSX) {
				t.Fatalf("Parse(%s lồng sâu) = %v, muốn ErrNotXLSX trước khi excelize mở tệp", part, err)
			}
		})
	}
}

// THE REFUSAL IS ABOUT DEPTH, and the boundary is exact: maxXMLDepth passes, one more is refused.
func TestWellFormed_DepthBoundary(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}})
	for _, c := range []struct {
		depth int
		deep  bool
	}{{maxXMLDepth, false}, {maxXMLDepth + 1, true}} {
		z := rewriteZip(t, good, nil, map[string][]byte{"xl/custom.xml": nested(c.depth)})
		zr, err := zip.NewReader(bytes.NewReader(z), int64(len(z)))
		if err != nil {
			t.Fatalf("zip: %v", err)
		}
		var part *zip.File
		for _, f := range zr.File {
			if f.Name == "xl/custom.xml" {
				part = f
			}
		}
		err = wellFormed(part)
		if got := errors.Is(err, errXMLTooDeep); got != c.deep {
			t.Errorf("độ sâu %d: wellFormed = %v, muốn quá sâu = %v", c.depth, err, c.deep)
		}
		if !c.deep && err != nil {
			t.Errorf("độ sâu %d phải được nhận: %v", c.depth, err)
		}
	}
}

func TestParse_RefusesWhatIsNotAPlainWorkbook(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}})

	var bomb bytes.Buffer
	zw := zip.NewWriter(&bomb)
	w, _ := zw.Create("[Content_Types].xml")
	_, _ = w.Write([]byte(contentTypeMainXLSX))
	w, _ = zw.Create("xl/worksheets/sheet1.xml")
	_, _ = w.Write(bytes.Repeat([]byte(" "), maxUnzippedBytes+1)) // compresses to a few KB
	_ = zw.Close()

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"không phải zip", []byte("Tên bộ phận,Thuộc bộ phận\nA,"), ErrNotXLSX},
		{"zip hỏng", append([]byte("PK\x03\x04"), bytes.Repeat([]byte{1}, 64)...), ErrNotXLSX},
		{"tệp .xlsm (vbaProject)", rewriteZip(t, good, nil, map[string][]byte{"xl/vbaProject.bin": {0}}), ErrMacroEnabled},
		{"kiểu nội dung macroEnabled", rewriteZip(t, good, func(n string, b []byte) []byte {
			if n == "[Content_Types].xml" {
				return bytes.ReplaceAll(b, []byte(contentTypeMainXLSX), []byte("application/vnd.ms-excel.sheet.macroEnabled.main+xml"))
			}
			return b
		}, nil), ErrMacroEnabled},
		{"tài liệu Word đổi đuôi", rewriteZip(t, good, func(n string, b []byte) []byte {
			if n == "[Content_Types].xml" {
				return bytes.ReplaceAll(b, []byte(contentTypeMainXLSX), []byte("application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"))
			}
			return b
		}, nil), ErrNotXLSX},
		// excelize would stop reading this sheet silently and report the half it read.
		{"XML trang bị cắt cụt", rewriteZip(t, good, func(n string, b []byte) []byte {
			if n == "xl/worksheets/sheet1.xml" {
				return b[:len(b)/2]
			}
			return b
		}, nil), ErrNotXLSX},
		{"bom zip", bomb.Bytes(), ErrWorkbookTooLarge},
		{"quá cỡ tệp", append(append([]byte{}, good...), make([]byte, MaxFileBytes)...), ErrWorkbookTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := Parse(c.data); !errors.Is(err, c.want) {
				t.Errorf("lỗi = %v, muốn %v", err, c.want)
			}
		})
	}
}

// renameWorkbookPart moves the workbook part to a name that is not `.xml`, rewriting the package
// relationship that points at it — the shape excelize follows, whatever the name says.
func renameWorkbookPart(t *testing.T, src []byte, newName string, body func(orig []byte) []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		name := f.Name
		switch name {
		case "xl/workbook.xml":
			name, b = newName, body(b)
		case "xl/_rels/workbook.xml.rels":
			// excelize derives the workbook's own rels path from the workbook part's name.
			name = "xl/_rels/" + strings.TrimPrefix(newName, "xl/") + ".rels"
		case "_rels/.rels":
			b = bytes.ReplaceAll(b, []byte("xl/workbook.xml"), []byte(newName))
		}
		w, _ := zw.Create(name)
		_, _ = w.Write(b)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return out.Bytes()
}

// THE GUARD DOES NOT TRUST THE NAME. excelize finds the workbook through `_rels/.rels`, so a workbook
// part named `xl/wb.bin` is still XML-decoded — and before this fix it skipped the depth guard.
func TestInspect_RefusesDeepPartWhateverItsName(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}})

	// The premise: excelize really does open a workbook whose main part has a non-.xml name.
	renamed := renameWorkbookPart(t, good, "xl/wb.bin", func(b []byte) []byte { return b })
	f, err := excelize.OpenReader(bytes.NewReader(renamed))
	if err != nil {
		t.Fatalf("excelize mở tệp đổi tên phần workbook: %v", err)
	}
	if got := f.GetSheetList(); len(got) != 1 {
		f.Close()
		t.Fatalf("excelize phải đọc được workbook qua quan hệ, nhận các trang %v", got)
	}
	f.Close()
	if err := inspect(renamed); err != nil {
		t.Fatalf("workbook hợp lệ dưới tên khác bị từ chối: %v", err)
	}

	deep := renameWorkbookPart(t, good, "xl/wb.bin", func([]byte) []byte { return nested(maxXMLDepth + 1) })
	if err := inspect(deep); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("inspect(xl/wb.bin lồng %d tầng) = %v, muốn ErrNotXLSX", maxXMLDepth+1, err)
	}
	if _, _, err := Parse(deep); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("Parse(xl/wb.bin lồng sâu) = %v, muốn ErrNotXLSX trước khi excelize mở tệp", err)
	}

	// Depth is refused even when the stream breaks AFTER the deep part — Unmarshal recurses first.
	deepThenBroken := rewriteZip(t, good, nil, map[string][]byte{
		"xl/media/x.bin": append(nested(maxXMLDepth + 1)[:len(nested(maxXMLDepth+1))-maxXMLDepth*2], 0),
	})
	if err := inspect(deepThenBroken); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("inspect(lồng sâu rồi hỏng) = %v, muốn ErrNotXLSX", err)
	}

	// A sheet under a non-.xml name, truncated, would import its first half silently: refused.
	var sheet []byte
	zr, _ := zip.NewReader(bytes.NewReader(good), int64(len(good)))
	for _, zf := range zr.File {
		if zf.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := zf.Open()
			sheet, _ = io.ReadAll(rc)
			rc.Close()
		}
	}
	truncatedSheet := rewriteZip(t, good, nil, map[string][]byte{"xl/worksheets/sheet2.bin": sheet[:len(sheet)-40]})
	if err := inspect(truncatedSheet); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("inspect(trang .bin cắt cụt) = %v, muốn ErrNotXLSX", err)
	}
}

// BINARY ENTRIES STAY IMPORTABLE: a picture or printer settings is scanned too, breaks before its
// first element, and is accepted — a real file saved from Excel carries both.
func TestInspect_AcceptsBinaryParts(t *testing.T) {
	good := buildWorkbook(t, map[int][]any{1: headerRow(), 2: {"A"}})
	withBinary := rewriteZip(t, good, nil, map[string][]byte{
		"xl/media/image1.png":                     []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR<a><b>"),
		"xl/media/image2.jpeg":                    {0xFF, 0xD8, 0xFF, 0xE0, '<', 'a', '>'},
		"xl/printerSettings/printerSettings1.bin": []byte("M\x00i\x00c\x00r\x00o\x00<a>"),
		"xl/embeddings/empty.bin":                 {},
		"docProps/thumbnail.wmf":                  {0xD7, 0xCD, 0xC6, 0x9A, 0, 0},
	})
	if err := inspect(withBinary); err != nil {
		t.Fatalf("tệp có ảnh / thiết lập máy in bị từ chối: %v", err)
	}
	rows, rowErrs, err := Parse(withBinary)
	if err != nil || len(rowErrs) != 0 || len(rows) != 1 {
		t.Fatalf("Parse(tệp có phần nhị phân): %v %+v %+v", err, rowErrs, rows)
	}
	// Named as XML, the same binary bytes are refused: an .xml part must be well-formed outright.
	if err := inspect(rewriteZip(t, good, nil, map[string][]byte{"xl/custom.xml": {0x89, 'P', 'N', 'G'}})); !errors.Is(err, ErrNotXLSX) {
		t.Fatalf("inspect(.xml nhị phân) = %v, muốn ErrNotXLSX", err)
	}
}
