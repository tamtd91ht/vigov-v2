// Package orgunitxlsx reads and writes the Excel file of the org-chart import (14-cau-hinh.md §1,
// "Nhập từ Excel"). It knows the WORKBOOK and nothing else: the rules a row must satisfy are
// domain.PlanOrgUnitImport's, and the transaction is app.OrgUnitImporter's.
//
// THE UPLOAD IS UNTRUSTED INPUT FROM A BROWSER, and an .xlsx is a zip of XML — the two shapes with the
// longest history of parser abuse. What this package does about it, in the order it happens:
//
//  1. IN MEMORY ONLY. Parse takes a []byte the handler read under a byte cap; nothing is written to
//     disk, and excelize is configured so it never spills a part to a temporary file (see
//     openOptions).
//  2. THE ZIP IS INSPECTED BEFORE EXCELIZE SEES IT: magic bytes, entry count, the DECLARED
//     uncompressed size of every entry summed against a cap (archive/zip refuses an entry whose data
//     outruns its declared size, so the declaration is binding) — a zip bomb is refused by
//     arithmetic, before one byte is inflated.
//  3. MACROS ARE REFUSED, not ignored: a vbaProject part, or a macro-enabled content type, and the
//     file is rejected. Nothing here would run a macro, but a file that carries one is not the file
//     the template produced, and it would travel on in backups as something this system accepted.
//  4. EVERY ZIP ENTRY — whatever its name — IS SCANNED AS XML, NO DEEPER THAN maxXMLDepth
//     (GO-2026-6088), and every XML part must be well-formed, before excelize opens the file (checkPart). excelize's row iterator stops SILENTLY at a
//     decoding error, so a truncated sheet would import its first half and report success — exactly
//     the partial write an all-or-nothing import exists to prevent.
//  5. NO FORMULA IS EVALUATED. Cells are read raw (RawCellValue): a formula cell yields the value the
//     spreadsheet cached, never a computation, and a number yields its stored digits ("1.5", not a
//     display format that could round it to "2").
//
// CELL CONTENTS ARE NEVER LOGGED OR ECHOED in an error. Unit names are not personal data (rule 3),
// but the habit is kept everywhere a file is read.
package orgunitxlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html/charset"
)

// MaxFileBytes caps the upload. A template of 500 rows is under 40 KB; 2 MB is fifty times that.
const MaxFileBytes = 2 << 20

// The inflated caps. maxUnzippedBytes bounds the sum of every entry's declared size; a real template
// inflates to well under 1 MB.
const (
	maxUnzippedBytes = 20 << 20
	maxZipEntries    = 200
)

// The refusals of the FILE, before any row is read. The handler maps them to 413 / 415.
var (
	ErrNotXLSX          = errors.New("orgunitxlsx: tệp không phải bảng tính Excel .xlsx")
	ErrMacroEnabled     = errors.New("orgunitxlsx: tệp có macro (.xlsm) — không nhận")
	ErrWorkbookTooLarge = errors.New("orgunitxlsx: bảng tính giải nén quá lớn")
)

// The sheet names of the template. Vietnamese: a person reads them in Excel's tab bar (rule 12,
// invariant 2 — a UI string, not an identifier).
const (
	SheetData  = "Bộ phận"
	SheetGuide = "Hướng dẫn"
	SheetUnits = "Danh sách bộ phận"
)

// contentTypeMainXLSX is the one main-part type an ordinary workbook declares. A macro-enabled
// workbook (.xlsm), a template (.xltx) and a binary workbook (.xlsb) each declare a different one.
const contentTypeMainXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"

// inspect is steps 2–4 of the package comment, over the raw bytes.
func inspect(data []byte) error {
	if len(data) < 4 || !bytes.Equal(data[:4], []byte("PK\x03\x04")) {
		// An encrypted workbook is an OLE container, not a zip, and lands here too: there is no
		// password to open it with, and asking for one is not this route's job.
		return ErrNotXLSX
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ErrNotXLSX
	}
	if len(zr.File) > maxZipEntries {
		return ErrWorkbookTooLarge
	}
	var total uint64
	var contentTypes []byte
	var parts []*zip.File
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > maxUnzippedBytes {
			return ErrWorkbookTooLarge
		}
		name := strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.Contains(name, "vbaproject") || strings.Contains(name, "vbadata") {
			return ErrMacroEnabled
		}
		if name == "[content_types].xml" {
			if contentTypes, err = readPart(f); err != nil {
				return ErrNotXLSX
			}
		}
		// EVERY ENTRY, WHATEVER ITS NAME — see checkPart for why the name cannot be trusted.
		parts = append(parts, f)
	}
	if contentTypes == nil {
		return ErrNotXLSX
	}
	ct := strings.ToLower(string(contentTypes))
	if strings.Contains(ct, "macroenabled") || strings.Contains(ct, "vbaproject") {
		return ErrMacroEnabled
	}
	if !strings.Contains(ct, contentTypeMainXLSX) {
		return ErrNotXLSX
	}
	for _, f := range parts {
		if err := checkPart(f); err != nil {
			return ErrNotXLSX
		}
	}
	return nil
}

// readPart inflates one entry. Bounded by its declared size, which inspect has already summed.
func readPart(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("orgunitxlsx: mở phần %q: %w", f.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, maxUnzippedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("orgunitxlsx: đọc phần %q: %w", f.Name, err)
	}
	return b, nil
}

// maxXMLDepth caps element nesting in every XML part. A real workbook nests about ten deep (a drawing
// is the deepest); 64 refuses nothing genuine.
//
// WHY IT EXISTS: GO-2026-6088 — encoding/xml before go1.26.6 has no recursion-depth guard in Decode /
// Unmarshal, and excelize unmarshals every part on open, so a deeply nested part exhausts the stack of
// the request's goroutine. Token() is iterative and safe on the same input, which is why the depth is
// counted HERE, before excelize sees a byte. Keep the cap after the toolchain is upgraded: it also
// bounds the work a hostile file can ask for.
const maxXMLDepth = 64

// errXMLTooDeep — a part nests deeper than maxXMLDepth.
var errXMLTooDeep = errors.New("orgunitxlsx: XML lồng quá sâu")

// isXMLName reports whether an entry is NAMED as XML: `.xml`, relationship parts (`.rels`) and legacy
// VML drawings (`.vml`). Such an entry must be well-formed outright. The name is NOT what decides
// whether excelize decodes an entry — see checkPart.
func isXMLName(name string) bool {
	return strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") || strings.HasSuffix(name, ".vml")
}

// checkPart is the XML guard of ONE zip entry, and it runs on EVERY entry.
//
// WHY NOT ONLY THE `.xml` ONES: excelize finds its parts through relationship TARGETS, not names — the
// workbook part is whatever `_rels/.rels` points at (excelize@v2.11.0 workbook.go getWorkbookPath),
// the sheets are whatever the workbook's relationships point at — and it XML-decodes each one. A
// workbook part named `xl/wb.bin` skipped a name-based guard and reached encoding/xml's recursive
// Unmarshal: stack exhaustion on Go < 1.26.6 (GO-2026-6088), which is fatal and kills the replica.
// Chasing the targets instead is weaker still: a relationship's declared Type is attacker text too,
// and excelize loads sheets by rId whatever Type says.
//
// WHY THIS RULE PROVABLY COVERS EVERY PART EXCELIZE DECODES: the scan uses the same decoder excelize
// builds (xmlDecoder: encoding/xml, Strict, the same CharsetReader), over the same bytes, so it sees
// the same token stream. Unmarshal's recursion only ever follows the element depth of the tokens it
// has read, so:
//
//   - depth past maxXMLDepth anywhere in the stream is refused, EVEN IF the stream is broken later —
//     Unmarshal would already have recursed that deep before reaching the break;
//   - a stream that breaks after at least one element is refused: that is truncated or corrupted XML,
//     and excelize's row iterator would import the half before the break silently;
//   - a stream that breaks BEFORE ITS FIRST ELEMENT is accepted when the entry is not named as XML:
//     that is binary (a PNG's first byte is not UTF-8, a printerSettings .bin opens with NUL bytes),
//     and excelize's decoder gets no element out of it either, so it can neither recurse nor import a
//     row. This is what keeps a real file with a picture or printer settings importable.
//
// excelize's own re-decodes (extLst fragments wrapped in one extra element) are inner XML of a part
// already scanned, so they add one level at most.
func checkPart(f *zip.File) error {
	elements, err := scanXML(f)
	if err == nil || errors.Is(err, errXMLTooDeep) {
		return err
	}
	if elements > 0 || isXMLName(strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))) {
		return err
	}
	return nil
}

// wellFormed decodes one XML part to its end, refusing any error and nesting past maxXMLDepth.
func wellFormed(f *zip.File) error {
	_, err := scanXML(f)
	return err
}

// xmlDecoder is excelize's decoder, built the way excelize@v2.11.0 builds it (excelize.go
// xmlNewDecoder, with the default Options.CharsetReader = charset.NewReaderLabel). A decoder that
// differed — no CharsetReader, say — would stop at an `encoding="…"` declaration that excelize reads
// straight through, and would count no depth where excelize recurses.
func xmlDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	return dec
}

// scanXML decodes one entry token by token, counting start elements and refusing nesting past
// maxXMLDepth. encoding/xml expands no external entities and has no DTD processing, so this is not
// itself an XXE surface; Token() is iterative, so the scan is safe on the input it refuses.
func scanXML(f *zip.File) (int, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("orgunitxlsx: mở phần %q: %w", f.Name, err)
	}
	defer rc.Close()
	dec := xmlDecoder(io.LimitReader(rc, maxUnzippedBytes+1))
	depth, elements := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return elements, nil
		}
		if err != nil {
			return elements, fmt.Errorf("orgunitxlsx: phần %q không hợp lệ: %w", f.Name, err)
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			elements++
			if depth > maxXMLDepth {
				return elements, fmt.Errorf("%w: phần %q", errXMLTooDeep, f.Name)
			}
		case xml.EndElement:
			depth--
		}
	}
}

// openOptions keeps excelize in memory: UnzipXMLSizeLimit equal to the whole-file cap means no single
// part can exceed it (inspect has already bounded their sum), and a part over that limit is the one
// thing excelize would otherwise extract to the system temporary directory.
func openOptions() excelize.Options {
	return excelize.Options{
		RawCellValue:      true,
		UnzipSizeLimit:    maxUnzippedBytes,
		UnzipXMLSizeLimit: maxUnzippedBytes,
	}
}

// Parse reads the data sheet — the FIRST sheet of the workbook — into rows.
//
// The error is a refusal of the FILE (ErrNotXLSX, ErrMacroEnabled, ErrWorkbookTooLarge) or a system
// failure. A wrong header row is not an error here: it is returned as an import error on row 1, so
// the person sees it the way they see every other problem with their file.
//
// Blank rows are skipped, and row numbers stay the spreadsheet's own. Collection stops one row past
// domain.MaxOrgUnitImportRows, which is enough for the planner to refuse the file by count.
func Parse(data []byte) ([]domain.OrgUnitImportRow, []domain.OrgUnitImportError, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrWorkbookTooLarge
	}
	if err := inspect(data); err != nil {
		return nil, nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data), openOptions())
	if err != nil {
		if errors.Is(err, excelize.ErrOptionsUnzipSizeLimit) || strings.Contains(err.Error(), "unzip size") {
			return nil, nil, ErrWorkbookTooLarge
		}
		return nil, nil, ErrNotXLSX
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, ErrNotXLSX
	}
	it, err := f.Rows(sheets[0])
	if err != nil {
		return nil, nil, ErrNotXLSX
	}
	defer it.Close()

	cols := domain.OrgUnitImportColumns()
	var out []domain.OrgUnitImportRow
	rowNo := 0
	for it.Next() {
		rowNo++
		cells, err := it.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, nil, ErrNotXLSX
		}
		cell := func(i int) string {
			if i < len(cells) {
				return cells[i]
			}
			return ""
		}
		if rowNo == 1 {
			if !headerMatches(cells, cols) {
				return nil, []domain.OrgUnitImportError{{
					Row: 1,
					Message: "Dòng tiêu đề của trang đầu tiên phải là đúng bốn cột: " +
						strings.Join(cols, " · ") + ". Hãy tải tệp mẫu và nhập vào đó.",
				}}, nil
			}
			continue
		}
		r := domain.OrgUnitImportRow{Row: rowNo, Name: cell(0), Parent: cell(1), Code: cell(2), Order: cell(3)}
		if strings.TrimSpace(r.Name+r.Parent+r.Code+r.Order) == "" {
			continue
		}
		out = append(out, r)
		if len(out) > domain.MaxOrgUnitImportRows {
			break
		}
	}
	if err := it.Error(); err != nil {
		return nil, nil, ErrNotXLSX
	}
	if rowNo == 0 {
		return nil, []domain.OrgUnitImportError{{Row: 1, Message: "Trang đầu tiên của tệp đang trống."}}, nil
	}
	return out, nil, nil
}

// headerMatches compares trimmed and case-folded, so "TÊN BỘ PHẬN" is the same header. Extra
// columns after the fourth are allowed and ignored — a person's own notes column harms nothing.
func headerMatches(cells, want []string) bool {
	if len(cells) < len(want) {
		return false
	}
	for i, w := range want {
		if !strings.EqualFold(strings.TrimSpace(cells[i]), w) {
			return false
		}
	}
	return true
}

// templateSamples are the example rows. UPPER CASE, as the commune's convention writes names (§1),
// and taken from the specification's own sample data (§1:40). The one-stop-shop unit is deliberately
// absent: one-stop is out of this system's scope.
var templateSamples = [][]any{
	{"LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ", "", "", 1},
	{"VĂN PHÒNG ĐẢNG ỦY", "", "", 2},
	{"THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN", "", "", 3},
}

// dataRowsWithDropdown is how far down the parent dropdown reaches: the whole file cap, plus the
// header.
const dataRowsWithDropdown = domain.MaxOrgUnitImportRows + 1

// Template builds the import template for one commune: the data sheet with its header and sample
// rows, a guide, and a HIDDEN sheet listing the commune's live units, which feeds a dropdown on the
// "Thuộc bộ phận" column.
//
// THE DROPDOWN WARNS, IT DOES NOT BLOCK: a parent may also be a row above in the same file, which no
// list built today can contain. Each live unit is listed by NAME when its name is unique in the
// commune and by CODE otherwise — the value the importer can resolve without ambiguity.
func Template(live []domain.BoPhan) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", SheetData); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: đặt tên trang: %w", err)
	}
	header := make([]any, 0, 4)
	for _, c := range domain.OrgUnitImportColumns() {
		header = append(header, c)
	}
	if err := f.SetSheetRow(SheetData, "A1", &header); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: ghi tiêu đề: %w", err)
	}
	for i, s := range templateSamples {
		row := s
		if err := f.SetSheetRow(SheetData, fmt.Sprintf("A%d", i+2), &row); err != nil {
			return nil, fmt.Errorf("orgunitxlsx: ghi dòng mẫu: %w", err)
		}
	}
	for col, w := range map[string]float64{"A": 48, "B": 40, "C": 30, "D": 10} {
		if err := f.SetColWidth(SheetData, col, col, w); err != nil {
			return nil, fmt.Errorf("orgunitxlsx: độ rộng cột: %w", err)
		}
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, fmt.Errorf("orgunitxlsx: kiểu chữ: %w", err)
	}
	if err := f.SetCellStyle(SheetData, "A1", "D1", bold); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: kiểu tiêu đề: %w", err)
	}

	if _, err := f.NewSheet(SheetGuide); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: tạo trang hướng dẫn: %w", err)
	}
	for i, line := range guideLines() {
		if err := f.SetCellValue(SheetGuide, fmt.Sprintf("A%d", i+1), line); err != nil {
			return nil, fmt.Errorf("orgunitxlsx: ghi hướng dẫn: %w", err)
		}
	}
	if err := f.SetColWidth(SheetGuide, "A", "A", 120); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: độ rộng hướng dẫn: %w", err)
	}

	values := parentChoices(live)
	if _, err := f.NewSheet(SheetUnits); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: tạo trang danh sách: %w", err)
	}
	for i, v := range values {
		// A STRING CELL, whatever the text: a unit name beginning with "=" stays text, never a formula.
		if err := f.SetCellStr(SheetUnits, fmt.Sprintf("A%d", i+1), v); err != nil {
			return nil, fmt.Errorf("orgunitxlsx: ghi danh sách: %w", err)
		}
	}
	if err := f.SetSheetVisible(SheetUnits, false); err != nil {
		return nil, fmt.Errorf("orgunitxlsx: ẩn trang danh sách: %w", err)
	}
	if len(values) > 0 {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = fmt.Sprintf("B2:B%d", dataRowsWithDropdown)
		dv.SetSqrefDropList(fmt.Sprintf("'%s'!$A$1:$A$%d", SheetUnits, len(values)))
		dv.SetError(excelize.DataValidationErrorStyleWarning, "Bộ phận cha",
			"Giá trị này không có trong danh sách bộ phận hiện có của xã. Vẫn được nếu đó là một dòng PHÍA TRÊN trong tệp này.")
		if err := f.AddDataValidation(SheetData, dv); err != nil {
			return nil, fmt.Errorf("orgunitxlsx: gắn danh sách chọn: %w", err)
		}
	}
	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("orgunitxlsx: ghi tệp: %w", err)
	}
	return buf.Bytes(), nil
}

// parentChoices lists each live unit by name when the name is unique (case-folded), by code
// otherwise.
func parentChoices(live []domain.BoPhan) []string {
	count := make(map[string]int, len(live))
	for _, u := range live {
		count[strings.ToLower(strings.TrimSpace(u.Ten))]++
	}
	out := make([]string, 0, len(live))
	for _, u := range live {
		if count[strings.ToLower(strings.TrimSpace(u.Ten))] == 1 {
			out = append(out, u.Ten)
		} else {
			out = append(out, u.Ma)
		}
	}
	return out
}

func guideLines() []string {
	return []string{
		"HƯỚNG DẪN NHẬP SƠ ĐỒ TỔ CHỨC TỪ EXCEL",
		"",
		"Nhập vào trang \"" + SheetData + "\". Giữ nguyên dòng tiêu đề; mỗi dòng bên dưới là một bộ phận MỚI.",
		"• " + domain.OrgUnitImportColName + ": bắt buộc, tối đa 200 ký tự. Nên viết HOA, ví dụ VĂN PHÒNG ĐẢNG ỦY.",
		"• " + domain.OrgUnitImportColParent + ": để trống nếu là bộ phận cấp cao nhất. Ghi tên hoặc mã của một bộ phận đang có trong xã, hoặc của một dòng PHÍA TRÊN trong tệp này.",
		"• " + domain.OrgUnitImportColCode + ": để trống thì hệ thống tự sinh từ tên (VĂN PHÒNG ĐẢNG ỦY → van-phong-dang-uy). Nếu ghi: chỉ chữ thường không dấu, chữ số và dấu gạch ngang. Mã đã cấp không đổi được.",
		"• " + domain.OrgUnitImportColOrder + ": số nguyên từ 0 đến 9999; để trống là 0.",
		"",
		"Tệp được kiểm tra TOÀN BỘ trước khi ghi: chỉ cần một dòng lỗi là không bộ phận nào được tạo, và mọi lỗi được liệt kê theo dòng và cột.",
		"Việc nhập chỉ THÊM bộ phận mới: một mã đã có trong xã (kể cả của bộ phận đã xoá) hoặc một tên đã có dưới cùng bộ phận cha đều bị báo lỗi, không bị ghi đè.",
		"Tối đa " + fmt.Sprint(domain.MaxOrgUnitImportRows) + " dòng mỗi lần nhập. Chỉ nhận tệp .xlsx, không nhận tệp có macro.",
	}
}
