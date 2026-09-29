package domain

// The rules of an IMPORT FROM EXCEL of this service's two reference catalogues — residential-unit
// types (loai_don_vi_dan_cu) and task blocs (khoi_nhiem_vu) — user decision 2026-09-29, ADR 0059 §3:
// ONE import route per owning service and per catalogue, never a shared "lookup-values" import with a
// `Nhóm` column (ADR 0024 stop condition #4). Create only, ALL OR NOTHING — the shape of the org-chart
// import (org_unit_import.go) and of service-comms' map-asset-type import (4c220a9).
//
// ONE PLANNER FOR BOTH CATALOGUES because migration 0005 declares the two tables identically and hangs
// one trigger on both (0005:124): the rules are a property of the shape. The route decides the table;
// nothing in a file can.
//
// THE COLUMNS are ../vigov-require's `lookup-values` (apps/api/app/modules/admin/catalogues.py:466-481)
// minus `Nhóm` — the route knows its catalogue — and minus `Diễn giải`, because neither table has a
// description column (migration 0005): a column read and then dropped is somebody's text silently lost.
// `Mã` is added, as on the org-chart import, because the create route requires a code; blank derives it
// from the label.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/residential-unit-types and
// POST /api/v1/task-blocs — ChuanHoaNhanDanhMuc, ChuanHoaMaDanhMuc, KiemTraThuTuDanhMuc — so a row that
// could not be typed into the form cannot be imported. What differs, deliberately (the org-chart
// import's two differences):
//
//   - NO `-2` SUFFIX ON A DERIVED CODE. A file imported twice must not produce `khu-pho-2`; a derived
//     code already issued — soft-deleted rows included — is a row error.
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
)

// The three columns of the template, in order. THE HEADER ROW MUST MATCH THEM.
const (
	CatalogueImportColLabel = "Tên hiển thị"
	CatalogueImportColCode  = "Mã (để trống thì tự sinh)"
	CatalogueImportColOrder = "Thứ tự"
)

// CatalogueImportColumns is the header row, in order.
func CatalogueImportColumns() []string {
	return []string{CatalogueImportColLabel, CatalogueImportColCode, CatalogueImportColOrder}
}

// MaxCatalogueImportRows bounds one file. Both catalogues' ceilings are 100 live rows
// (store.TranDanhMucLoaiDonViDanCu, store.TranDanhMucKhoiNhiemVu); the ceiling check below is the
// binding one, this only stops a file that could not fit whatever it held.
const MaxCatalogueImportRows = 100

// CatalogueImportRow is one data row AS TYPED. Row is the spreadsheet's own number (header = 1).
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
// collide. A row OUT OF USE is live here — it still labels the records that carry its code.
type ExistingCatalogueEntry struct {
	Code    string
	Label   string
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
// 1) into data rows, or refuses the SHEET when its header is not the template's. Entirely blank rows
// are skipped and keep the numbering of the rows around them.
func CatalogueRowsFromSheet(cells [][]string) ([]CatalogueImportRow, []CatalogueImportError) {
	cols := CatalogueImportColumns()
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
		if blankCells(c) {
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
			if j < len(c) {
				return c[j]
			}
			return ""
		}
		rows = append(rows, CatalogueImportRow{Row: i + 1, Label: at(0), Code: at(1), Order: at(2)})
	}
	return rows, errs
}

// PlanCatalogueImport validates the WHOLE file and returns either the plan or every error — the plan
// is nil whenever there is at least one error, so a caller cannot write half a file by accident.
// ceiling is the catalogue's store ceiling, passed in because this package does not import the store.
func PlanCatalogueImport(rows []CatalogueImportRow, existing []ExistingCatalogueEntry, ceiling int) ([]PlannedCatalogueEntry, []CatalogueImportError) {
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
	taken := make(map[string]bool, len(existing))
	liveLabels := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Code] = true
		if !e.Deleted {
			live++
			liveLabels[foldName(e.Label)] = true
		}
	}
	if live+len(rows) > ceiling {
		// Refused here rather than discovered by the list route, which REFUSES a catalogue over its
		// ceiling — an import that crossed it would empty every picker fed by this catalogue.
		fileErr(fmt.Sprintf("Danh mục của xã đang có %d mục; thêm %d mục sẽ vượt trần %d của danh mục.", live, len(rows), ceiling))
	}

	rowErr := func(r CatalogueImportRow, col, msg string) {
		errs = append(errs, CatalogueImportError{Row: r.Row, Column: col, Message: msg})
	}
	plan := make([]PlannedCatalogueEntry, 0, len(rows))
	codeRow := make(map[string]int, len(rows))
	labelRow := make(map[string]int, len(rows))
	for _, r := range rows {
		p := PlannedCatalogueEntry{Row: r.Row}
		labelOK, codeOK := false, false

		if label, err := ChuanHoaNhanDanhMuc(r.Label); err != nil {
			if errors.Is(err, ErrNhanDanhMucQuaDai) {
				rowErr(r, CatalogueImportColLabel, fmt.Sprintf("Tên hiển thị dài quá %d ký tự.", NhanDanhMucToiDa))
			} else {
				rowErr(r, CatalogueImportColLabel, "Thiếu tên hiển thị (hoặc có ký tự điều khiển).")
			}
		} else {
			p.Label, labelOK = label, true
		}

		typed := strings.TrimSpace(r.Code) != ""
		if typed {
			if code, err := ChuanHoaMaDanhMuc(r.Code); err != nil {
				if errors.Is(err, ErrMaDanhMucQuaDai) {
					rowErr(r, CatalogueImportColCode, fmt.Sprintf("Mã dài quá %d ký tự.", MaDanhMucToiDa))
				} else {
					rowErr(r, CatalogueImportColCode, "Mã chỉ gồm chữ thường a-z, số và dấu gạch ngang đơn, ví dụ khu-pho.")
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

		if order, msg := parseCatalogueOrder(r.Order); msg != "" {
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
			k := foldName(p.Label)
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

// DeriveCatalogueCode derives a code from a label: "Khu phố" → "khu-pho". It is SinhMaBoPhan's slug —
// the one table of Vietnamese diacritics in this service — cut at a word boundary to MaDanhMucToiDa,
// then checked with the form's own validator so a derived code is always a code the form accepts.
// false when nothing usable remains. NOT guaranteed free: that is the planner's check.
func DeriveCatalogueCode(label string) (string, bool) {
	code, err := SinhMaBoPhan(label)
	if err != nil {
		return "", false
	}
	if len(code) > MaDanhMucToiDa {
		code = code[:MaDanhMucToiDa]
		if i := strings.LastIndexByte(code, '-'); i > 0 {
			code = code[:i]
		}
		code = strings.TrimRight(code, "-")
	}
	if _, err := ChuanHoaMaDanhMuc(code); err != nil {
		return "", false
	}
	return code, true
}

// parseCatalogueOrder reads "Thứ tự". Blank = 0, the form's default. DIGITS ONLY: "1.5" is refused,
// never read as 15 (the defect vigov-require's parser has — org_unit_import.go parseImportOrder).
func parseCatalogueOrder(raw string) (int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ""
	}
	msg := fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d (chỉ gồm chữ số).", ThuTuDanhMucToiDa)
	if len(s) > 4 {
		return 0, msg
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, msg
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || KiemTraThuTuDanhMuc(n) != nil {
		return 0, msg
	}
	return n, ""
}
