package domain

// The rules of an IMPORT FROM EXCEL of the document-type catalogue (ADR 0059 §3: one import route per
// owning service; docs/ui-ux/14-cau-hinh.md §5 `Nhập từ Excel`). ALL OR NOTHING, create only — the
// shape service-comms' map-asset-type import took (service-comms/internal/domain/map_asset_type_import.go),
// itself the org-unit import's.
//
// THIS FILE ONLY PLANS. It turns the rows of one sheet plus a snapshot of the commune's catalogue into
// either the rows to create or EVERY error in the file — never both. Reading the workbook is
// core/xlsx; writing the plan and its audit entry in one transaction is
// app.DocumentTypeCatalogue.ImportDocumentTypes. Standard library only.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/document-types — NormalizeLabel,
// NormalizeCode, ValidateSortOrder — so a row that could not be typed into the form cannot be imported.
//
// THE COLUMNS are the comms import's three: `Tên hiển thị`, `Mã`, `Thứ tự`. The reference system's
// `lookup-values` sheet (../vigov-require/apps/api/app/modules/admin/catalogues.py:466-481) adds `Nhóm`
// — the route knows its group — and `Diễn giải`, which `loai_van_ban` has no column for (migration
// 0003). THERE IS NO `Mặc định` COLUMN, deliberately: making an imported row the default would CLEAR
// the default of a row the commune already has, and an import is create-only — it edits nothing that
// exists. Every imported row is created `la_mac_dinh = false`; the default is set on the form.
//
// THE MESSAGES NEVER ECHO A CELL. They name the row and the column, which is all a person needs to find
// it; a cell of a spreadsheet somebody filled in is text nobody vetted.

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// The three columns of the template, in order. THE HEADER ROW MUST MATCH THEM: a file whose first row
// says something else is a different file, and guessing which column is which would import the wrong
// column into a catalogue whose codes are never reissued.
const (
	DocumentTypeImportColLabel = "Tên hiển thị"
	DocumentTypeImportColCode  = "Mã (để trống thì tự sinh)"
	DocumentTypeImportColOrder = "Thứ tự"
)

// DocumentTypeImportColumns is the header row, in order.
func DocumentTypeImportColumns() []string {
	return []string{DocumentTypeImportColLabel, DocumentTypeImportColCode, DocumentTypeImportColOrder}
}

// MaxDocumentTypeImportRows bounds one file: the catalogue ceiling itself
// (store.MaxDocumentTypes is 200). A file of more rows cannot be imported whatever it holds.
const MaxDocumentTypeImportRows = 200

// DocumentTypeImportRow is one data row AS TYPED. Row is the spreadsheet's own number (header = 1).
type DocumentTypeImportRow struct {
	Row   int
	Label string
	Code  string
	Order string
}

// DocumentTypeImportError is one refusal. Row 0 and Column "" mean the FILE.
type DocumentTypeImportError struct {
	Row     int
	Column  string
	Message string
}

// ExistingDocumentType is one catalogue row as the planner sees it. SOFT-DELETED ROWS ARE INCLUDED,
// flagged: their codes are still taken, but their labels no longer count as duplicates.
type ExistingDocumentType struct {
	Code    string
	Label   string
	Deleted bool
}

// PlannedDocumentType is one row the import would create (ID filled once created).
type PlannedDocumentType struct {
	Row   int
	ID    string
	Code  string
	Label string
	Order int
}

// ReadDocumentTypeSheet turns ReadSheet's rows into data rows, or refuses the FILE when the header is
// not the template's. Blank rows are skipped and keep the numbering of the rows around them.
func ReadDocumentTypeSheet(sheet [][]string) ([]DocumentTypeImportRow, []DocumentTypeImportError) {
	if len(sheet) == 0 {
		return nil, []DocumentTypeImportError{{Message: "Tệp không có dòng tiêu đề."}}
	}
	want := DocumentTypeImportColumns()
	head := sheet[0]
	ok := len(head) >= len(want)
	for i := 0; ok && i < len(want); i++ {
		ok = strings.TrimSpace(head[i]) == want[i]
	}
	for i := len(want); ok && i < len(head); i++ {
		ok = strings.TrimSpace(head[i]) == ""
	}
	if !ok {
		return nil, []DocumentTypeImportError{{Message: "Dòng tiêu đề không khớp tệp mẫu (cần đúng ba cột: " +
			strings.Join(want, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}
	var (
		rows []DocumentTypeImportRow
		errs []DocumentTypeImportError
	)
	for i, cells := range sheet[1:] {
		rowNo := i + 2
		if blankImportCells(cells) {
			continue
		}
		for j := len(want); j < len(cells); j++ {
			if strings.TrimSpace(cells[j]) != "" {
				errs = append(errs, DocumentTypeImportError{Row: rowNo,
					Message: "Dòng có dữ liệu ngoài ba cột của tệp mẫu."})
				break
			}
		}
		cell := func(j int) string {
			if j < len(cells) {
				return cells[j]
			}
			return ""
		}
		rows = append(rows, DocumentTypeImportRow{Row: rowNo, Label: cell(0), Code: cell(1), Order: cell(2)})
	}
	return rows, errs
}

func blankImportCells(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// PlanDocumentTypeImport validates the WHOLE file and returns either the plan or every error — the plan
// is nil whenever there is at least one error, so a caller cannot write half a file by accident.
// ceiling is store.MaxDocumentTypes, passed in because this package does not import the store.
func PlanDocumentTypeImport(rows []DocumentTypeImportRow, existing []ExistingDocumentType, ceiling int) (
	[]PlannedDocumentType, []DocumentTypeImportError) {

	var errs []DocumentTypeImportError
	fileErr := func(msg string) { errs = append(errs, DocumentTypeImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return nil, errs
	}
	if len(rows) > MaxDocumentTypeImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập.", len(rows), MaxDocumentTypeImportRows))
		return nil, errs
	}

	live := 0
	taken := make(map[string]bool, len(existing))
	liveLabels := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Code] = true
		if !e.Deleted {
			live++
			liveLabels[foldImportLabel(e.Label)] = true
		}
	}
	if live+len(rows) > ceiling {
		// Refused here rather than discovered by GET /api/v1/document-types, which REFUSES a catalogue
		// over the ceiling — an import that crossed it would empty the registration form's type box.
		fileErr(fmt.Sprintf("Xã đang có %d loại văn bản; thêm %d loại sẽ vượt trần %d của danh mục.", live, len(rows), ceiling))
	}

	rowErr := func(r DocumentTypeImportRow, col, msg string) {
		errs = append(errs, DocumentTypeImportError{Row: r.Row, Column: col, Message: msg})
	}
	plan := make([]PlannedDocumentType, 0, len(rows))
	codeRow := make(map[string]int, len(rows))
	labelRow := make(map[string]int, len(rows))
	for _, r := range rows {
		p := PlannedDocumentType{Row: r.Row}
		labelOK, codeOK := false, false

		if label, err := NormalizeLabel(r.Label); err != nil {
			if errors.Is(err, ErrLabelTooLong) {
				rowErr(r, DocumentTypeImportColLabel, fmt.Sprintf("Tên hiển thị dài quá %d ký tự.", MaxLabelLen))
			} else {
				rowErr(r, DocumentTypeImportColLabel, "Thiếu tên hiển thị (hoặc có ký tự điều khiển).")
			}
		} else {
			p.Label, labelOK = label, true
		}

		typed := strings.TrimSpace(r.Code) != ""
		if typed {
			if code, err := NormalizeCode(r.Code); err != nil {
				if errors.Is(err, ErrCodeTooLong) {
					rowErr(r, DocumentTypeImportColCode, fmt.Sprintf("Mã dài quá %d ký tự.", MaxCodeLen))
				} else {
					rowErr(r, DocumentTypeImportColCode, "Mã chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ cong-van.")
				}
			} else {
				p.Code, codeOK = code, true
			}
		} else if labelOK {
			if code := SlugDocumentTypeCode(p.Label); code == "" {
				rowErr(r, DocumentTypeImportColCode, "Không tự sinh được mã từ tên hiển thị — hãy nhập mã.")
			} else {
				p.Code, codeOK = code, true
			}
		}

		if order, msg := parseImportOrder(r.Order); msg != "" {
			rowErr(r, DocumentTypeImportColOrder, msg)
		} else {
			p.Order = order
		}

		// DUPLICATES — against the catalogue and against rows above. An issued code is never reissued,
		// soft-deleted rows included (rule 7, invariant 3); a LABEL only collides with a live row.
		if codeOK {
			switch first, dup := codeRow[p.Code]; {
			case taken[p.Code] && typed:
				rowErr(r, DocumentTypeImportColCode, "Mã này đã được cấp trong xã (kể cả cho loại đã xoá — mã đã cấp không cấp lại).")
			case taken[p.Code]:
				rowErr(r, DocumentTypeImportColCode, "Mã tự sinh từ tên đã được cấp trong xã (kể cả cho loại đã xoá). Nhập lại tệp không tạo loại trùng; nếu đây là loại mới, hãy nhập một mã khác.")
			case dup:
				rowErr(r, DocumentTypeImportColCode, fmt.Sprintf("Mã trùng với dòng %d trong tệp.", first))
			default:
				codeRow[p.Code] = r.Row
			}
		}
		if labelOK {
			k := foldImportLabel(p.Label)
			switch first, dup := labelRow[k]; {
			case liveLabels[k]:
				rowErr(r, DocumentTypeImportColLabel, "Danh mục của xã đã có một loại cùng tên hiển thị. Nhập tệp chỉ THÊM loại mới, không cập nhật loại đang có.")
			case dup:
				rowErr(r, DocumentTypeImportColLabel, fmt.Sprintf("Trùng tên hiển thị với dòng %d trong tệp.", first))
			default:
				labelRow[k] = r.Row
			}
		}
		plan = append(plan, p)
	}

	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		return nil, errs
	}
	return plan, nil
}

// parseImportOrder reads "Thứ tự". Blank = 0, the form's default. DIGITS ONLY: "1.5" is refused, never
// read as 15.
func parseImportOrder(raw string) (int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ""
	}
	msg := fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d.", MaxSortOrder)
	if len(s) > 9 {
		return 0, msg
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, msg
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || ValidateSortOrder(n) != nil {
		return 0, msg
	}
	return n, ""
}

// foldImportLabel is the comparison key for a label: trimmed, case-folded. Not Unicode-normalised — a
// decomposed twin does not match, which errs toward creating, never toward refusing the wrong row.
func foldImportLabel(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// vietnameseBase maps every lower-case Vietnamese letter carrying a diacritic onto its base letter.
// THE THIRD COPY of this table (service-identity's SinhMaBoPhan, service-comms' map-asset-type import):
// rule 2 forbids importing another service's internal/, and lifting it into core/ is its own change.
var vietnameseBase = func() map[rune]rune {
	groups := map[rune]string{
		'a': "àáảãạăằắẳẵặâầấẩẫậ",
		'e': "èéẻẽẹêềếểễệ",
		'i': "ìíỉĩị",
		'o': "òóỏõọôồốổỗộơờớởỡợ",
		'u': "ùúủũụưừứửữự",
		'y': "ỳýỷỹỵ",
		'd': "đ",
	}
	out := make(map[rune]rune, 80)
	for base, marked := range groups {
		for _, r := range marked {
			out[r] = base
		}
	}
	return out
}()

// SlugDocumentTypeCode derives a code from a label: "Quyết định" → "quyet-dinh". Lower-case, Vietnamese
// diacritics stripped (đ → d), every run of anything else one '-', none at the ends, cut at MaxCodeLen on
// a word boundary. "" when nothing usable remains. NOT guaranteed free — the planner checks that.
func SlugDocumentTypeCode(label string) string {
	var b strings.Builder
	sep := false
	for _, r := range label {
		r = unicode.ToLower(r)
		if base, ok := vietnameseBase[r]; ok {
			r = base
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte('-')
			}
			sep = false
			b.WriteRune(r)
			continue
		}
		sep = true
	}
	code := b.String()
	if len(code) > MaxCodeLen {
		code = code[:MaxCodeLen]
		if i := strings.LastIndexByte(code, '-'); i > 0 {
			code = code[:i]
		}
		code = strings.TrimRight(code, "-")
	}
	return code
}
