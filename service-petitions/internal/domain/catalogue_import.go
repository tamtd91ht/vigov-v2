package domain

// The rules of an IMPORT FROM EXCEL of this service's two task catalogues — task types
// (loai_nhiem_vu) and task priorities (muc_uu_tien_nhiem_vu) — user decision 2026-09-29, ADR 0059 §3:
// ONE import route per owning service and per catalogue, never a shared "lookup-values" import with a
// `Nhóm` column (ADR 0024 stop condition #4). Create only, ALL OR NOTHING — the shape of
// service-identity's catalogue import (fbbae7b, internal/domain/catalogue_import.go) and of
// service-comms' map-asset-type import (4c220a9), copied because rule 2 forbids importing another
// service's internal/.
//
// ONE PLANNER FOR BOTH CATALOGUES because migration 0003 declares the two tables identically and hangs
// one trigger on both: the rules are a property of the shape. The route decides the table; nothing in
// a file can.
//
// THE COLUMNS are ../vigov-require's `lookup-values` (apps/api/app/modules/admin/catalogues.py:466-481)
// minus `Nhóm` — the route knows its catalogue — and minus `Diễn giải`, because neither table has a
// description column (migration 0003): a column read and then dropped is somebody's text silently
// lost. `Mã` is added because the create route requires a code; blank derives it from the label.
//
// THE PRIORITY SCALE HAS NO `Thứ tự` COLUMN, AND THAT IS THE ONE DELIBERATE DIFFERENCE. On a scale the
// order IS the meaning (store.MucUuTienNhiemVuStore.DanhSach: "THE ORDER IS THE MEANING OF THIS
// LIST"), and a number typed into a spreadsheet cell can silently rank a new level above "Khẩn". So
// the rows of a priority file are ranked BY THEIR POSITION IN THE FILE and placed AFTER every level
// the commune already has (live rows, out-of-use ones included): existing max `thu_tu` + 1, + 2, …
// Moving a level between two existing ones stays one deliberate act on the Danh mục screen.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/task-types and
// POST /api/v1/task-priorities — ChuanHoaNhan, ChuanHoaMa, KiemTraThuTu — so a row that could not be
// typed into the form cannot be imported. What differs, deliberately (identity's two differences):
//
//   - NO `-2` SUFFIX ON A DERIVED CODE. A file imported twice must not produce `theo-van-ban-2`; a
//     derived code already issued — soft-deleted rows included — is a row error.
//   - A LABEL ALREADY LIVE IN THE CATALOGUE, OR REPEATED IN THE FILE, IS AN ERROR. The form does not
//     check labels; an import of forty rows is where a duplicate goes unnoticed.
//
// An imported row is a tier-1 row of the commune (nguon = 'don-vi', written as a literal by the store),
// in use, and NEVER the default: choosing the default is one deliberate act on the form, not a side
// effect of a file.
//
// NO MESSAGE ECHOES A CELL. They name the row and the column. STANDARD LIBRARY ONLY.

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// The template's columns. THE HEADER ROW MUST MATCH THE LAYOUT'S LIST EXACTLY.
const (
	CatalogueImportColLabel = "Tên hiển thị"
	CatalogueImportColCode  = "Mã (để trống thì tự sinh)"
	CatalogueImportColOrder = "Thứ tự"
)

// CatalogueImportLayout says which columns a catalogue's file carries and where a row's order comes
// from. A TYPE AND NOT A BOOL so each call site names the rule it is choosing.
type CatalogueImportLayout int

const (
	// CatalogueOrderFromColumn — Tên hiển thị · Mã · Thứ tự. The task types: `thu_tu` only arranges a
	// flat list, so the person filling the file types it, as on the form.
	CatalogueOrderFromColumn CatalogueImportLayout = iota + 1
	// CatalogueOrderFromPosition — Tên hiển thị · Mã. The priority scale: rank = position in the file,
	// after every existing level (see the file comment).
	CatalogueOrderFromPosition
)

// Columns is the header row of this layout, in order.
func (l CatalogueImportLayout) Columns() []string {
	if l == CatalogueOrderFromPosition {
		return []string{CatalogueImportColLabel, CatalogueImportColCode}
	}
	return []string{CatalogueImportColLabel, CatalogueImportColCode, CatalogueImportColOrder}
}

// MaxCatalogueImportRows bounds one file. The type catalogue's ceiling is 100 live rows
// (store.TranDanhMucLoaiNhiemVu), the scale's 50 (store.TranDanhMucMucUuTien); the ceiling check below
// is the binding one, this only stops a file that could not fit whatever it held.
const MaxCatalogueImportRows = 100

// CatalogueImportRow is one data row AS TYPED. Row is the spreadsheet's own number (header = 1). Order
// is always "" under CatalogueOrderFromPosition.
type CatalogueImportRow struct {
	Row   int
	Label string
	Code  string
	Order string
}

// CatalogueImportError is one refusal. Row 0 and Column "" mean the FILE; Row 1 is the header.
type CatalogueImportError struct {
	Row     int
	Column  string
	Message string
}

// ExistingCatalogueEntry is one catalogue row as the planner sees it. SOFT-DELETED ROWS ARE INCLUDED,
// flagged: their codes stay taken (UNIQUE (tenant_id, ma) is not partial), their labels no longer
// collide and their order no longer ranks. A row OUT OF USE is live here — it still labels the records
// that carry its code, and it still holds its place on the scale.
type ExistingCatalogueEntry struct {
	Code    string
	Label   string
	Order   int
	Deleted bool
}

// PlannedCatalogueEntry is one row the import would create. ID is empty until inserted.
type PlannedCatalogueEntry struct {
	Row   int
	ID    string
	Code  string
	Label string
	Order int
}

// CatalogueRowsFromSheet turns the raw cells of the first sheet (core/xlsx.ReadSheet: cells[0] is row
// 1) into data rows, or refuses the SHEET when its header is not the layout's. Entirely blank rows are
// skipped and keep the numbering of the rows around them.
func CatalogueRowsFromSheet(cells [][]string, layout CatalogueImportLayout) ([]CatalogueImportRow, []CatalogueImportError) {
	cols := layout.Columns()
	if len(cells) == 0 {
		return nil, []CatalogueImportError{{Message: "Tệp không có dòng nào."}}
	}
	header := cells[0]
	ok := len(header) >= len(cols)
	for i := 0; ok && i < len(cols); i++ {
		ok = strings.TrimSpace(header[i]) == cols[i]
	}
	for i := len(cols); ok && i < len(header); i++ {
		ok = strings.TrimSpace(header[i]) == ""
	}
	if !ok {
		return nil, []CatalogueImportError{{Row: 1,
			Message: "Dòng tiêu đề không khớp tệp mẫu (" + strings.Join(cols, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var rows []CatalogueImportRow
	var errs []CatalogueImportError
	for i := 1; i < len(cells); i++ {
		c := cells[i]
		if blankCatalogueCells(c) {
			continue
		}
		extra := false
		for j := len(cols); j < len(c); j++ {
			if strings.TrimSpace(c[j]) != "" {
				extra = true
				break
			}
		}
		if extra {
			errs = append(errs, CatalogueImportError{Row: i + 1,
				Message: fmt.Sprintf("Dòng có dữ liệu ngoài %d cột của tệp mẫu.", len(cols))})
			continue
		}
		at := func(j int) string {
			if j < len(c) && j < len(cols) {
				return c[j]
			}
			return ""
		}
		rows = append(rows, CatalogueImportRow{Row: i + 1, Label: at(0), Code: at(1), Order: at(2)})
	}
	return rows, errs
}

func blankCatalogueCells(c []string) bool {
	for _, s := range c {
		if strings.TrimSpace(s) != "" {
			return false
		}
	}
	return true
}

// PlanCatalogueImport validates the WHOLE file and returns either the plan or every error — the plan
// is nil whenever there is at least one error, so a caller cannot write half a file by accident.
// ceiling is the catalogue's store ceiling, passed in because this package does not import the store.
func PlanCatalogueImport(rows []CatalogueImportRow, existing []ExistingCatalogueEntry, ceiling int,
	layout CatalogueImportLayout) ([]PlannedCatalogueEntry, []CatalogueImportError) {

	var errs []CatalogueImportError
	fileErr := func(msg string) { errs = append(errs, CatalogueImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return nil, errs
	}
	if len(rows) > MaxCatalogueImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập.", len(rows), MaxCatalogueImportRows))
		return nil, errs
	}

	live := 0
	lastOrder := -1 // the rank the first appended level follows; -1 = an empty scale starts at 0
	taken := make(map[string]bool, len(existing))
	liveLabels := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Code] = true
		if !e.Deleted {
			live++
			liveLabels[foldCatalogueLabel(e.Label)] = true
			if e.Order > lastOrder {
				lastOrder = e.Order
			}
		}
	}
	if live+len(rows) > ceiling {
		// Refused here rather than discovered by the list route, which REFUSES a catalogue over its
		// ceiling — an import that crossed it would empty every picker fed by this catalogue.
		fileErr(fmt.Sprintf("Danh mục của xã đang có %d mục; thêm %d mục sẽ vượt trần %d của danh mục.", live, len(rows), ceiling))
	}
	if layout == CatalogueOrderFromPosition && lastOrder+len(rows) > ThuTuToiDa {
		// Never clamped: two levels sharing the last rank would be ordered by their code, not the file.
		fileErr(fmt.Sprintf("Các mức nhập vào xếp sau mức cuối cùng của xã và sẽ vượt Thứ tự %d. Hãy sắp lại Thứ tự trên màn hình Danh mục trước khi nhập.", ThuTuToiDa))
	}

	rowErr := func(r CatalogueImportRow, col, msg string) {
		errs = append(errs, CatalogueImportError{Row: r.Row, Column: col, Message: msg})
	}
	plan := make([]PlannedCatalogueEntry, 0, len(rows))
	codeRow := make(map[string]int, len(rows))
	labelRow := make(map[string]int, len(rows))
	for i, r := range rows {
		p := PlannedCatalogueEntry{Row: r.Row}
		labelOK, codeOK := false, false

		if label, err := ChuanHoaNhan(r.Label); err != nil {
			if errors.Is(err, ErrNhanQuaDai) {
				rowErr(r, CatalogueImportColLabel, fmt.Sprintf("Tên hiển thị dài quá %d ký tự.", NhanToiDa))
			} else {
				rowErr(r, CatalogueImportColLabel, "Thiếu tên hiển thị (hoặc có ký tự điều khiển).")
			}
		} else {
			p.Label, labelOK = label, true
		}

		typed := strings.TrimSpace(r.Code) != ""
		if typed {
			if code, err := ChuanHoaMa(r.Code); err != nil {
				if errors.Is(err, ErrMaQuaDai) {
					rowErr(r, CatalogueImportColCode, fmt.Sprintf("Mã dài quá %d ký tự.", MaToiDa))
				} else {
					rowErr(r, CatalogueImportColCode, "Mã chỉ gồm chữ thường a-z, số và dấu gạch ngang đơn, ví dụ theo-van-ban.")
				}
			} else {
				p.Code, codeOK = code, true
			}
		} else if labelOK {
			if code, ok := DeriveCatalogueCode(p.Label); !ok {
				rowErr(r, CatalogueImportColCode, "Không tự sinh được mã từ tên hiển thị — hãy nhập mã.")
			} else {
				p.Code, codeOK = code, true
			}
		}

		if layout == CatalogueOrderFromPosition {
			p.Order = lastOrder + 1 + i
		} else if order, msg := parseCatalogueOrder(r.Order); msg != "" {
			rowErr(r, CatalogueImportColOrder, msg)
		} else {
			p.Order = order
		}

		// DUPLICATES — against the catalogue and against rows above. An issued code is never reissued,
		// soft-deleted rows included (rule 7, invariant 3); a LABEL only collides with a live row.
		if codeOK {
			switch first, dup := codeRow[p.Code]; {
			case taken[p.Code] && typed:
				rowErr(r, CatalogueImportColCode, "Mã này đã được cấp trong xã (kể cả cho mục đã xoá — mã đã cấp không cấp lại).")
			case taken[p.Code]:
				rowErr(r, CatalogueImportColCode, "Mã tự sinh từ tên đã được cấp trong xã (kể cả cho mục đã xoá). Nhập lại tệp không tạo mục trùng; nếu đây là mục mới, hãy nhập một mã khác.")
			case dup && typed:
				rowErr(r, CatalogueImportColCode, fmt.Sprintf("Mã trùng với dòng %d trong tệp.", first))
			case dup:
				rowErr(r, CatalogueImportColCode, fmt.Sprintf("Mã tự sinh trùng với mã của dòng %d trong tệp — hãy nhập một mã khác.", first))
			default:
				codeRow[p.Code] = r.Row
			}
		}
		if labelOK {
			k := foldCatalogueLabel(p.Label)
			switch first, dup := labelRow[k]; {
			case liveLabels[k]:
				rowErr(r, CatalogueImportColLabel, "Danh mục của xã đã có một mục cùng tên hiển thị (kể cả mục đang tắt). Nhập tệp chỉ THÊM mục mới, không cập nhật mục đang có.")
			case dup:
				rowErr(r, CatalogueImportColLabel, fmt.Sprintf("Trùng tên hiển thị với dòng %d trong tệp.", first))
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

// parseCatalogueOrder reads "Thứ tự". Blank = 0, the form's default. DIGITS ONLY: "1.5" is refused,
// never read as 15 (the defect vigov-require's parser has).
func parseCatalogueOrder(raw string) (int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ""
	}
	msg := fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d (chỉ gồm chữ số).", ThuTuToiDa)
	if len(s) > 4 {
		return 0, msg
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, msg
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || KiemTraThuTu(n) != nil {
		return 0, msg
	}
	return n, ""
}

// foldCatalogueLabel is the comparison key for a label: trimmed, case-folded. Not Unicode-normalised
// (no NFC in the standard library) — a decomposed twin does not match, which errs toward creating,
// never toward refusing the wrong row.
func foldCatalogueLabel(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// vietnameseBaseLetter maps every lower-case Vietnamese letter carrying a diacritic onto its base
// letter. The same table as service-identity's SinhMaBoPhan and service-comms' slug — copied, because
// rule 2 forbids importing another service's internal/ and one table is not worth a core package.
var vietnameseBaseLetter = func() map[rune]rune {
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

// DeriveCatalogueCode derives a code from a label: "Theo văn bản" → "theo-van-ban". Lower-case,
// diacritics stripped (đ → d), every run of anything else one '-', cut at MaToiDa on a word boundary,
// then checked with the form's own validator so a derived code is always a code the form accepts.
// false when nothing usable remains. NOT guaranteed free: that is the planner's check.
func DeriveCatalogueCode(label string) (string, bool) {
	var b strings.Builder
	sep := false
	for _, r := range label {
		r = unicode.ToLower(r)
		if base, ok := vietnameseBaseLetter[r]; ok {
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
	if len(code) > MaToiDa {
		code = code[:MaToiDa]
		if i := strings.LastIndexByte(code, '-'); i > 0 {
			code = code[:i]
		}
		code = strings.TrimRight(code, "-")
	}
	if _, err := ChuanHoaMa(code); err != nil {
		return "", false
	}
	return code, true
}
