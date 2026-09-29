package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// WHAT THIS FILE IS FOR: core/xlsx is the one reader every import trusts with a hostile upload. The
// hardening tests are ported from service-identity/internal/orgunitxlsx (92ddffc), because the copy
// that lost them is the copy an attacker would aim at. Every refusal is asserted by SENTINEL, and
// TestErrorsCarryNoFileText asserts that no refusal carries text out of the file.

var lim = DefaultLimits

func read(b []byte) ([][]string, error) {
	return ReadSheet(bytes.NewReader(b), int64(len(b)), Limits{})
}

// buildWorkbook writes an .xlsx in memory whose first sheet holds exactly these rows (1-based).
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

func good(t *testing.T) []byte {
	return buildWorkbook(t, map[int][]any{1: {"Tên", "Mã"}, 2: {"A", "a"}})
}

// rewriteZip copies a workbook, letting edit change an entry, and appends extra entries.
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

// nested is a WELL-FORMED element nested n deep, so a refusal can only be about depth.
func nested(n int) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` + strings.Repeat("<a>", n) + strings.Repeat("</a>", n))
}

func TestReadSheet_RawValuesRowNumbersAndBlankRows(t *testing.T) {
	b := buildWorkbook(t, map[int][]any{
		1: {"Tên", "Thứ tự"},
		2: {"VĂN PHÒNG", 1.5}, // a NUMBER cell: its raw digits, never rounded
		// row 3 blank
		4: {"TỔ A", 3},
		// rows 5.. blank: dropped
	})
	rows, err := read(b)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("want 4 rows (row 3 kept blank so numbering holds), got %d: %q", len(rows), rows)
	}
	if rows[1][1] != "1.5" {
		t.Errorf("number cell 1.5 must read raw as \"1.5\", got %q", rows[1][1])
	}
	if len(rows[2]) != 0 {
		t.Errorf("row 3 must be an empty slice, got %q", rows[2])
	}
	if rows[3][0] != "TỔ A" || rows[3][1] != "3" {
		t.Errorf("row 4 keeps the spreadsheet's own index: %q", rows[3])
	}
}

// A FORMULA IS NEVER EVALUATED: with no cached value the cell reads empty, not "2".
func TestReadSheet_FormulaNotEvaluated(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellStr("Sheet1", "A1", "Tên")
	_ = f.SetCellStr("Sheet1", "A2", "TỔ B")
	_ = f.SetCellFormula("Sheet1", "B2", "1+1")
	buf, _ := f.WriteToBuffer()
	f.Close()
	rows, err := read(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if len(rows[1]) > 1 && rows[1][1] == "2" {
		t.Error("formula was evaluated")
	}
}

func TestReadSheet_EmptySheet(t *testing.T) {
	for name, b := range map[string][]byte{
		"no rows":         buildWorkbook(t, map[int][]any{}),
		"only blank text": buildWorkbook(t, map[int][]any{1: {"  ", ""}, 3: {" "}}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := read(b); !errors.Is(err, ErrEmptySheet) {
				t.Fatalf("err = %v, want ErrEmptySheet", err)
			}
		})
	}
}

// MaxRows counts SCANNED rows, blank ones included: a sheet whose last row is far down is refused
// by count, not scanned without end.
func TestReadSheet_TooManyRows(t *testing.T) {
	m := map[int][]any{1: {"Tên"}}
	for i := 2; i <= 11; i++ {
		m[i] = []any{"X"}
	}
	b := buildWorkbook(t, m)
	if rows, err := ReadSheet(bytes.NewReader(b), -1, Limits{MaxRows: 11}); err != nil || len(rows) != 11 {
		t.Fatalf("exactly MaxRows must pass: %d %v", len(rows), err)
	}
	if _, err := ReadSheet(bytes.NewReader(b), -1, Limits{MaxRows: 10}); !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("err = %v, want ErrTooManyRows", err)
	}
	far := buildWorkbook(t, map[int][]any{1: {"Tên"}, lim.MaxRows + 1: {"X"}})
	if _, err := read(far); !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("a row past the default cap after blank rows: err = %v, want ErrTooManyRows", err)
	}
}

func TestReadSheet_ColumnsTruncated(t *testing.T) {
	wide := make([]any, lim.MaxColumns+5)
	for i := range wide {
		wide[i] = "c"
	}
	rows, err := read(buildWorkbook(t, map[int][]any{1: wide}))
	if err != nil || len(rows[0]) != lim.MaxColumns {
		t.Fatalf("want %d cells, got %d (%v)", lim.MaxColumns, len(rows[0]), err)
	}
}

// A LIMIT CAN ONLY TIGHTEN: a caller asking for more than the default gets the default.
func TestLimits_CannotLoosen(t *testing.T) {
	got := Limits{MaxFileBytes: 1 << 40, MaxZipEntries: 10000, MaxUnzippedBytes: 1 << 40, MaxXMLDepth: 10000, MaxRows: 1 << 30, MaxColumns: 1 << 20}.normalize()
	if got != DefaultLimits {
		t.Fatalf("looser limits were honoured: %+v", got)
	}
	if (Limits{}).normalize() != DefaultLimits {
		t.Fatal("zero Limits must be DefaultLimits")
	}
	tight := Limits{MaxFileBytes: 10, MaxZipEntries: 5, MaxUnzippedBytes: 100, MaxXMLDepth: 8, MaxRows: 3, MaxColumns: 2}
	if tight.normalize() != tight {
		t.Fatalf("tighter limits must be kept: %+v", tight.normalize())
	}
	b := good(t)
	if _, err := ReadSheet(bytes.NewReader(b), -1, Limits{MaxFileBytes: int64(len(b)) - 1}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a tightened file cap must refuse: %v", err)
	}
}

// THE PRE-INSPECTION ITSELF refuses a truncated sheet — excelize's row iterator stops silently on a
// decoding error, so this guard is the one that matters.
func TestInspect_RefusesTruncatedSheetXML(t *testing.T) {
	g := good(t)
	if err := inspect(g, lim); err != nil {
		t.Fatalf("good file refused: %v", err)
	}
	cut := rewriteZip(t, g, func(n string, b []byte) []byte {
		if n == "xl/worksheets/sheet1.xml" {
			return b[:len(b)-40]
		}
		return b
	}, nil)
	if _, err := read(cut); !errors.Is(err, ErrMalformed) {
		t.Fatalf("truncated sheet: err = %v, want ErrMalformed", err)
	}
}

// GO-2026-6088: A DEEPLY NESTED PART IS REFUSED BEFORE EXCELIZE DECODES IT — in ANY XML part,
// including the ones never read for rows (styles, relationships, drawings) but unmarshalled on open.
func TestReadSheet_RefusesDeeplyNestedXMLInEveryPart(t *testing.T) {
	g := good(t)
	for _, part := range []string{"xl/worksheets/sheet1.xml", "xl/styles.xml", "xl/_rels/workbook.xml.rels", "xl/drawings/vmlDrawing1.vml"} {
		t.Run(part, func(t *testing.T) {
			deep := rewriteZip(t, g, func(n string, b []byte) []byte {
				if n == part {
					return nested(lim.MaxXMLDepth + 1)
				}
				return b
			}, nil)
			if part == "xl/drawings/vmlDrawing1.vml" {
				deep = rewriteZip(t, g, nil, map[string][]byte{part: nested(lim.MaxXMLDepth + 1)})
			}
			if _, err := read(deep); !errors.Is(err, ErrMalformed) {
				t.Fatalf("%s nested %d deep: err = %v, want ErrMalformed", part, lim.MaxXMLDepth+1, err)
			}
		})
	}
}

// THE REFUSAL IS ABOUT DEPTH, and the boundary is exact: MaxXMLDepth passes, one more is refused.
func TestCheckPart_DepthBoundary(t *testing.T) {
	g := good(t)
	for _, c := range []struct {
		depth int
		deep  bool
	}{{lim.MaxXMLDepth, false}, {lim.MaxXMLDepth + 1, true}} {
		z := rewriteZip(t, g, nil, map[string][]byte{"xl/custom.xml": nested(c.depth)})
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
		err = checkPart(part, lim)
		if got := errors.Is(err, errXMLTooDeep); got != c.deep {
			t.Errorf("depth %d: checkPart = %v, want too deep = %v", c.depth, err, c.deep)
		}
		if !c.deep && err != nil {
			t.Errorf("depth %d must pass: %v", c.depth, err)
		}
	}
	// A tightened depth is honoured.
	if _, err := ReadSheet(bytes.NewReader(g), -1, Limits{MaxXMLDepth: 2}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("MaxXMLDepth 2 must refuse a real workbook: %v", err)
	}
}

func TestReadSheet_RefusesWhatIsNotAPlainWorkbook(t *testing.T) {
	g := good(t)

	var bomb bytes.Buffer
	zw := zip.NewWriter(&bomb)
	w, _ := zw.Create("[Content_Types].xml")
	_, _ = w.Write([]byte(contentTypeMainXLSX))
	w, _ = zw.Create("xl/worksheets/sheet1.xml")
	_, _ = w.Write(bytes.Repeat([]byte(" "), int(lim.MaxUnzippedBytes)+1)) // compresses to a few KB
	_ = zw.Close()

	parts := map[string][]byte{}
	for i := 0; i < lim.MaxZipEntries; i++ {
		parts["xl/media/p"+strings.Repeat("x", i)+".bin"] = []byte{0}
	}

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"CSV", []byte("Tên,Mã\nA,a"), ErrNotXLSX},
		{"empty upload", nil, ErrNotXLSX},
		{"OLE container (encrypted / .xls)", append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 512)...), ErrNotXLSX},
		{"broken zip", append([]byte("PK\x03\x04"), bytes.Repeat([]byte{1}, 64)...), ErrNotXLSX},
		{"no content types", func() []byte {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, _ := z.Create("xl/workbook.xml")
			_, _ = w.Write([]byte("<workbook/>"))
			_ = z.Close()
			return b.Bytes()
		}(), ErrNotXLSX},
		{".xlsm (vbaProject)", rewriteZip(t, g, nil, map[string][]byte{"xl/vbaProject.bin": {0}}), ErrMacroEnabled},
		{"macroEnabled content type", rewriteZip(t, g, func(n string, b []byte) []byte {
			if n == "[Content_Types].xml" {
				return bytes.ReplaceAll(b, []byte(contentTypeMainXLSX), []byte("application/vnd.ms-excel.sheet.macroEnabled.main+xml"))
			}
			return b
		}, nil), ErrMacroEnabled},
		{"Word document renamed", rewriteZip(t, g, func(n string, b []byte) []byte {
			if n == "[Content_Types].xml" {
				return bytes.ReplaceAll(b, []byte(contentTypeMainXLSX), []byte("application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"))
			}
			return b
		}, nil), ErrNotXLSX},
		{"sheet XML cut in half", rewriteZip(t, g, func(n string, b []byte) []byte {
			if n == "xl/worksheets/sheet1.xml" {
				return b[:len(b)/2]
			}
			return b
		}, nil), ErrMalformed},
		{"zip bomb", bomb.Bytes(), ErrTooLarge},
		{"too many parts", rewriteZip(t, g, nil, parts), ErrTooLarge},
		{"over the file cap", append(append([]byte{}, g...), make([]byte, lim.MaxFileBytes)...), ErrTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ReadSheet(bytes.NewReader(c.data), -1, Limits{}); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// THE DECLARED SIZE REFUSES EARLY, and a size that lies small does not lift the cap.
func TestReadSheet_DeclaredSize(t *testing.T) {
	g := good(t)
	if _, err := ReadSheet(bytes.NewReader(g), lim.MaxFileBytes+1, Limits{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("declared over cap: %v", err)
	}
	big := append(append([]byte{}, g...), make([]byte, lim.MaxFileBytes)...)
	if _, err := ReadSheet(bytes.NewReader(big), 10, Limits{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("declared small, actually big: %v", err)
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
// part named `xl/wb.bin` is still XML-decoded.
func TestReadSheet_RefusesDeepPartWhateverItsName(t *testing.T) {
	g := good(t)

	renamed := renameWorkbookPart(t, g, "xl/wb.bin", func(b []byte) []byte { return b })
	if rows, err := read(renamed); err != nil || len(rows) != 2 {
		t.Fatalf("a valid workbook under another name must be readable (the premise): %v %q", err, rows)
	}

	deep := renameWorkbookPart(t, g, "xl/wb.bin", func([]byte) []byte { return nested(lim.MaxXMLDepth + 1) })
	if _, err := read(deep); !errors.Is(err, ErrMalformed) {
		t.Fatalf("xl/wb.bin nested deep: err = %v, want ErrMalformed", err)
	}

	// Depth is refused even when the stream breaks AFTER the deep part — Unmarshal recurses first.
	n := nested(lim.MaxXMLDepth + 1)
	deepThenBroken := rewriteZip(t, g, nil, map[string][]byte{"xl/media/x.bin": append(n[:len(n)-lim.MaxXMLDepth*2], 0)})
	if _, err := read(deepThenBroken); !errors.Is(err, ErrMalformed) {
		t.Fatalf("deep then broken: err = %v, want ErrMalformed", err)
	}

	// A sheet under a non-.xml name, truncated, would import its first half silently: refused.
	var sheet []byte
	zr, _ := zip.NewReader(bytes.NewReader(g), int64(len(g)))
	for _, zf := range zr.File {
		if zf.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := zf.Open()
			sheet, _ = io.ReadAll(rc)
			rc.Close()
		}
	}
	truncated := rewriteZip(t, g, nil, map[string][]byte{"xl/worksheets/sheet2.bin": sheet[:len(sheet)-40]})
	if _, err := read(truncated); !errors.Is(err, ErrMalformed) {
		t.Fatalf("truncated .bin sheet: err = %v, want ErrMalformed", err)
	}
}

// BINARY ENTRIES STAY IMPORTABLE: a picture or printer settings is scanned too, breaks before its
// first element, and is accepted — a real file saved from Excel carries both.
func TestReadSheet_AcceptsBinaryParts(t *testing.T) {
	g := good(t)
	withBinary := rewriteZip(t, g, nil, map[string][]byte{
		"xl/media/image1.png":                     []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR<a><b>"),
		"xl/media/image2.jpeg":                    {0xFF, 0xD8, 0xFF, 0xE0, '<', 'a', '>'},
		"xl/printerSettings/printerSettings1.bin": []byte("M\x00i\x00c\x00r\x00o\x00<a>"),
		"xl/embeddings/empty.bin":                 {},
		"docProps/thumbnail.wmf":                  {0xD7, 0xCD, 0xC6, 0x9A, 0, 0},
	})
	if rows, err := read(withBinary); err != nil || len(rows) != 2 {
		t.Fatalf("file with pictures / printer settings refused: %v %q", err, rows)
	}
	// Named as XML, the same binary bytes are refused: an .xml part must be well-formed outright.
	if _, err := read(rewriteZip(t, g, nil, map[string][]byte{"xl/custom.xml": {0x89, 'P', 'N', 'G'}})); !errors.Is(err, ErrMalformed) {
		t.Fatalf("binary .xml: err = %v, want ErrMalformed", err)
	}
}

// NO REFUSAL CARRIES TEXT OUT OF THE FILE: every error is a bare sentinel, so a cell value or a
// part name (attacker text, possibly personal data) can never reach a log or a response.
func TestErrorsCarryNoFileText(t *testing.T) {
	g := buildWorkbook(t, map[int][]any{1: {"Tên"}, 2: {"0900000000"}})
	marker := "xl/worksheets/MARKER0900000000.xml"
	inputs := [][]byte{
		rewriteZip(t, g, nil, map[string][]byte{marker: nested(lim.MaxXMLDepth + 1)}),
		rewriteZip(t, g, nil, map[string][]byte{marker: []byte("<a><b>0900000000")}),
		rewriteZip(t, g, func(n string, b []byte) []byte {
			if n == "xl/worksheets/sheet1.xml" {
				return b[:len(b)-40]
			}
			return b
		}, nil),
	}
	sentinels := []error{ErrNotXLSX, ErrMacroEnabled, ErrTooLarge, ErrMalformed, ErrTooManyRows, ErrEmptySheet}
	for i, in := range inputs {
		_, err := read(in)
		if err == nil {
			t.Fatalf("input %d accepted", i)
		}
		bare := false
		for _, s := range sentinels {
			if err == s {
				bare = true
			}
		}
		if !bare || strings.Contains(err.Error(), "MARKER") || strings.Contains(err.Error(), "0900000000") {
			t.Errorf("input %d: error %q is not a bare sentinel", i, err)
		}
	}
}

func TestTemplate_RoundTripAndDropdowns(t *testing.T) {
	spec := TemplateSpec{
		SheetName: "Bộ phận",
		Header:    []string{"Tên bộ phận", "Thuộc bộ phận", "Mã", "Thứ tự"},
		Examples: [][]string{
			{"LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ", "", "", "1"},
			{"=1+1", "", "007", "2"}, // text stays text: no formula, zeros kept
		},
		Guide: []string{"HƯỚNG DẪN", "Dòng hai"},
		Choices: map[int]Choices{
			1: {Values: []string{"LÃNH ĐẠO", "=HYPERLINK(\"x\")", "to-a"}, Title: "Bộ phận cha", Message: "Không có trong danh sách."},
			3: {Values: []string{"1", "2"}, Strict: true},
		},
		DropdownRows: 500,
	}
	b, err := Template(spec)
	if err != nil {
		t.Fatalf("Template: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 3 || got[0] != spec.SheetName || got[1] != SheetGuide || got[2] != SheetChoices {
		t.Fatalf("sheets: %v — data sheet must be first", got)
	}
	if vis, _ := f.GetSheetVisible(SheetChoices); vis {
		t.Error("choices sheet must be hidden")
	}
	if formula, _ := f.GetCellFormula(SheetChoices, "A2"); formula != "" {
		t.Errorf("a choice beginning with '=' became a formula: %q", formula)
	}
	if formula, _ := f.GetCellFormula(spec.SheetName, "A3"); formula != "" {
		t.Errorf("an example beginning with '=' became a formula: %q", formula)
	}
	dvs, err := f.GetDataValidations(spec.SheetName)
	if err != nil || len(dvs) != 2 {
		t.Fatalf("data validations: %+v %v", dvs, err)
	}
	for _, dv := range dvs {
		switch {
		case strings.HasPrefix(dv.Sqref, "B2:B501"):
			if dv.ErrorStyle == nil || *dv.ErrorStyle != "warning" {
				t.Errorf("column B must WARN: %+v", dv.ErrorStyle)
			}
		case strings.HasPrefix(dv.Sqref, "D2:D501"):
			if dv.ErrorStyle != nil && *dv.ErrorStyle != "stop" {
				t.Errorf("column D must STOP: %+v", *dv.ErrorStyle)
			}
		default:
			t.Errorf("unexpected sqref %q", dv.Sqref)
		}
	}

	rows, err := ReadSheet(bytes.NewReader(b), int64(len(b)), Limits{})
	if err != nil {
		t.Fatalf("ReadSheet(Template): %v", err)
	}
	want := [][]string{spec.Header, {"LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ", "", "", "1"}, {"=1+1", "", "007", "2"}}
	if len(rows) != len(want) {
		t.Fatalf("round trip rows: %q", rows)
	}
	for i := range want {
		for j := range want[i] {
			got := ""
			if j < len(rows[i]) {
				got = rows[i][j]
			}
			if got != want[i][j] {
				t.Errorf("row %d col %d: got %q want %q", i+1, j+1, got, want[i][j])
			}
		}
	}
}

func TestTemplate_NoChoicesNoHiddenSheet(t *testing.T) {
	b, err := Template(TemplateSpec{SheetName: "Dữ liệu", Header: []string{"A"}})
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(b))
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 1 {
		t.Errorf("sheets: %v — no guide, no choices: one sheet", got)
	}
	if dvs, _ := f.GetDataValidations("Dữ liệu"); len(dvs) != 0 {
		t.Errorf("no dropdown expected: %+v", dvs)
	}
}

func TestTemplate_InvalidSpec(t *testing.T) {
	for name, s := range map[string]TemplateSpec{
		"no sheet name":          {Header: []string{"A"}},
		"no header":              {SheetName: "X"},
		"example wider":          {SheetName: "X", Header: []string{"A"}, Examples: [][]string{{"1", "2"}}},
		"choices out of range":   {SheetName: "X", Header: []string{"A"}, Choices: map[int]Choices{1: {Values: []string{"v"}}}},
		"collides with guide":    {SheetName: SheetGuide, Header: []string{"A"}},
		"collides with choices":  {SheetName: SheetChoices, Header: []string{"A"}},
		"wider than MaxColumns":  {SheetName: "X", Header: make([]string, lim.MaxColumns+1)},
		"negative choice column": {SheetName: "X", Header: []string{"A"}, Choices: map[int]Choices{-1: {Values: []string{"v"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Template(s); !errors.Is(err, ErrTemplateSpec) {
				t.Errorf("err = %v, want ErrTemplateSpec", err)
			}
		})
	}
}
