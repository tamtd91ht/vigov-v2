package domain

// The rules of a residential-unit IMPORT FROM EXCEL (14-cau-hinh.md §2, "⬆ Nhập từ Excel"; user
// decision 2026-09-29, ADR 0059 §2): create only, ALL OR NOTHING.
//
// ALL OR NOTHING IS THE CALLER'S CHOICE FOR ADR 0059's STILL-OPEN QUESTION #5 ("tất cả hoặc không, hay
// từng phần"), taken on the precedent of the org chart and the staff import (both decided all or
// nothing) — a partial import leaves the administrator to work out which rows went in, and every row
// that did holds a code that is never reissued (rule 7, invariant 3).
//
// THIS FILE ONLY PLANS. It turns the cells of one sheet plus a snapshot of the commune into either a
// list of units to create or a list of every error in the file — never both. Reading the workbook is
// core/xlsx.ReadSheet (in the HTTP layer, because it pulls in excelize); writing the plan with its
// audit entries in one transaction is app.ResidentialUnitImporter. STANDARD LIBRARY ONLY.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/residential-units
// (residential_unit_write.go). What differs from the form, deliberately — the same two differences
// the org-chart import makes (org_unit_import.go):
//
//   - NO `-2` SUFFIX ON A DERIVED CODE. A file imported twice must not produce `thon-binh-an-2`; a
//     derived code already issued — to ANY row, soft-deleted or out of use — is a row error.
//   - A NAME ALREADY LIVE IN THE COMMUNE (in use or out of use) OR REPEATED IN THE FILE IS AN ERROR.
//
// NO MESSAGE ECHOES A CELL. Staff codes and names travel in this file (the head column), and a staff
// name is personal data (rule 3, forbidden #3): every message names the row and the column, which is
// all a person needs to find the cell.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The seven columns of the template, in order. THE HEADER ROW MUST MATCH THEM: a sheet whose first row
// says something else is a different file, and guessing which column is which would import the wrong
// figure into a record the commune reports upward.
const (
	ResidentialUnitImportColName       = "Tên thôn / tổ dân phố"
	ResidentialUnitImportColType       = "Loại"
	ResidentialUnitImportColHead       = "Trưởng thôn / Tổ trưởng (mã cán bộ)"
	ResidentialUnitImportColHouseholds = "Số hộ"
	ResidentialUnitImportColPopulation = "Nhân khẩu"
	ResidentialUnitImportColCode       = "Mã (để trống thì tự sinh)"
	ResidentialUnitImportColOrder      = "Thứ tự"
)

// ResidentialUnitImportColumns is the header row, in order.
func ResidentialUnitImportColumns() []string {
	return []string{
		ResidentialUnitImportColName, ResidentialUnitImportColType, ResidentialUnitImportColHead,
		ResidentialUnitImportColHouseholds, ResidentialUnitImportColPopulation,
		ResidentialUnitImportColCode, ResidentialUnitImportColOrder,
	}
}

// MaxResidentialUnitImportRows bounds one file: the commune's whole list is bounded by
// store.TranDanhSachThonToDanPho (500), so a file of more rows cannot be imported whatever it holds.
const MaxResidentialUnitImportRows = 500

// HeadStaffSeparator joins a staff code and a name in the template's dropdown ("CB-2026-7K3M9Q ·
// Nguyễn Văn A"). The planner reads only what precedes it, so the NAME is never what identifies a
// person — two staff members may share one, a code never repeats.
const HeadStaffSeparator = " · "

// HeadStaffChoiceLabel is one dropdown value of the head column.
func HeadStaffChoiceLabel(code, name string) string { return code + HeadStaffSeparator + name }

// ResidentialUnitImportRow is one data row AS TYPED: seven cells, strings, never evaluated. Row is the
// spreadsheet's own row number (the header is row 1).
type ResidentialUnitImportRow struct {
	Row        int
	Name       string
	Type       string
	Head       string
	Households string
	Population string
	Code       string
	Order      string
}

// ResidentialUnitImportError is one refusal. Row 0 and Column "" mean the FILE; Row 1 is the header.
type ResidentialUnitImportError struct {
	Row     int
	Column  string
	Message string
}

// ExistingResidentialUnit is one row of the commune's list as the planner sees it. SOFT-DELETED ROWS ARE
// INCLUDED, flagged: their codes are still taken, their names are not. A unit OUT OF USE is live here —
// its name still labels old records, so a second unit of the same name would make them ambiguous.
type ExistingResidentialUnit struct {
	ID      string
	Code    string
	Name    string
	Deleted bool
}

// ResidentialUnitTypeChoice is one type a NEW unit may be filed under: live and in use.
type ResidentialUnitTypeChoice struct {
	Code  string
	Label string
}

// HeadStaffChoice is one staff member who may be recorded as a head: live and not locked.
type HeadStaffChoice struct {
	ID   string
	Code string
	Name string
}

// ResidentialUnitImportSnapshot is everything the planner checks a file against, read inside the
// import's own transaction.
type ResidentialUnitImportSnapshot struct {
	Units []ExistingResidentialUnit
	Types []ResidentialUnitTypeChoice
	Staff []HeadStaffChoice
}

// PlannedResidentialUnit is one unit the import would create. Unit.ID is empty until inserted.
type PlannedResidentialUnit struct {
	Row  int
	Unit ThonToDanPho
}

// ResidentialUnitRowsFromSheet turns the raw cells of the first sheet (core/xlsx.ReadSheet: rows[0] is
// spreadsheet row 1) into data rows, or reports why the SHEET is not this template.
//
// Entirely blank rows are skipped, keeping the numbering of the rows around them. A cell to the right
// of the seventh column is refused: the file is not the template, or a column was inserted, and
// reading on would put a figure under the wrong heading.
func ResidentialUnitRowsFromSheet(cells [][]string) ([]ResidentialUnitImportRow, []ResidentialUnitImportError) {
	cols := ResidentialUnitImportColumns()
	if len(cells) == 0 {
		return nil, []ResidentialUnitImportError{{Message: "Tệp không có dòng nào."}}
	}
	header := cells[0]
	headerOK := true
	for i, want := range cols {
		if i >= len(header) || strings.TrimSpace(header[i]) != want {
			headerOK = false
			break
		}
	}
	for i := len(cols); headerOK && i < len(header); i++ {
		if strings.TrimSpace(header[i]) != "" {
			headerOK = false
		}
	}
	if !headerOK {
		return nil, []ResidentialUnitImportError{{Row: 1,
			Message: "Dòng tiêu đề không khớp tệp mẫu (" + strings.Join(cols, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var rows []ResidentialUnitImportRow
	var errs []ResidentialUnitImportError
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
			errs = append(errs, ResidentialUnitImportError{Row: i + 1,
				Message: fmt.Sprintf("Dòng có dữ liệu ngoài %d cột của tệp mẫu.", len(cols))})
			continue
		}
		at := func(j int) string {
			if j < len(c) {
				return c[j]
			}
			return ""
		}
		rows = append(rows, ResidentialUnitImportRow{
			Row: i + 1, Name: at(0), Type: at(1), Head: at(2), Households: at(3), Population: at(4), Code: at(5), Order: at(6),
		})
	}
	return rows, errs
}

func blankCells(c []string) bool {
	for _, v := range c {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// PlanResidentialUnitImport validates the WHOLE file and returns either the plan or every error — the
// plan is nil whenever there is at least one error, so a caller cannot write half a file by accident.
// ceiling is store.TranDanhSachThonToDanPho, passed in because this package does not import the store.
func PlanResidentialUnitImport(rows []ResidentialUnitImportRow, snap ResidentialUnitImportSnapshot, ceiling int) ([]PlannedResidentialUnit, []ResidentialUnitImportError) {
	var errs []ResidentialUnitImportError
	fileErr := func(msg string) { errs = append(errs, ResidentialUnitImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return nil, errs
	}
	if len(rows) > MaxResidentialUnitImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập.", len(rows), MaxResidentialUnitImportRows))
		return nil, errs
	}

	live := 0
	taken := make(map[string]bool, len(snap.Units))
	liveNames := make(map[string]bool, len(snap.Units))
	for _, u := range snap.Units {
		taken[u.Code] = true
		if !u.Deleted {
			live++
			liveNames[foldName(u.Name)] = true
		}
	}
	if live+len(rows) > ceiling {
		// Refused here rather than discovered by GET /api/v1/residential-units, which REFUSES a list over
		// its ceiling — an import that crossed it would empty every address picker in the commune.
		fileErr(fmt.Sprintf("Xã đang có %d thôn / tổ dân phố; thêm %d sẽ vượt trần %d của danh sách.", live, len(rows), ceiling))
	}

	rowErr := func(r ResidentialUnitImportRow, col, msg string) {
		errs = append(errs, ResidentialUnitImportError{Row: r.Row, Column: col, Message: msg})
	}

	plan := make([]PlannedResidentialUnit, len(rows))
	nameRow := make(map[string]int, len(rows))
	codeRow := make(map[string]int, len(rows))
	for i, r := range rows {
		u := ThonToDanPho{DangDung: true}

		nameOK := false
		if name, err := NormalizeResidentialUnitName(r.Name); err != nil {
			rowErr(r, ResidentialUnitImportColName, importSentence(err))
		} else {
			u.Ten, nameOK = name, true
			key := foldName(name)
			switch first, dup := nameRow[key]; {
			case liveNames[key]:
				rowErr(r, ResidentialUnitImportColName, "Xã đã có một thôn / tổ dân phố cùng tên (kể cả đơn vị đã ngưng dùng). Nhập tệp chỉ THÊM đơn vị mới, không cập nhật đơn vị đang có.")
			case dup:
				rowErr(r, ResidentialUnitImportColName, fmt.Sprintf("Trùng tên với dòng %d trong tệp.", first))
			default:
				nameRow[key] = r.Row
			}
		}

		typedCode := strings.TrimSpace(r.Code) != ""
		codeOK := false
		if typedCode {
			if code, err := NormalizeResidentialUnitCode(r.Code); err != nil {
				rowErr(r, ResidentialUnitImportColCode, importSentence(err))
			} else {
				u.Ma, codeOK = code, true
			}
		} else if nameOK {
			if code, err := DeriveResidentialUnitCode(u.Ten); err != nil {
				rowErr(r, ResidentialUnitImportColCode, importSentence(err))
			} else {
				u.Ma, codeOK = code, true
			}
		}
		if codeOK {
			switch first, dup := codeRow[u.Ma]; {
			case taken[u.Ma] && typedCode:
				rowErr(r, ResidentialUnitImportColCode, "Mã này đã được cấp trong xã (kể cả cho đơn vị đã ngưng dùng — mã đã cấp không cấp lại).")
			case taken[u.Ma]:
				rowErr(r, ResidentialUnitImportColCode, "Mã tự sinh từ tên đã được cấp trong xã. Nhập lại tệp không tạo đơn vị trùng; nếu đây là đơn vị mới, hãy nhập một mã khác.")
			case dup && typedCode:
				rowErr(r, ResidentialUnitImportColCode, fmt.Sprintf("Mã trùng với dòng %d trong tệp.", first))
			case dup:
				rowErr(r, ResidentialUnitImportColCode, fmt.Sprintf("Mã tự sinh trùng với mã của dòng %d trong tệp — hãy nhập một mã khác.", first))
			default:
				codeRow[u.Ma] = r.Row
			}
		}

		if v := strings.TrimSpace(r.Type); v != "" {
			var found []ResidentialUnitTypeChoice
			for _, t := range snap.Types {
				if v == t.Code || foldName(v) == foldName(t.Label) {
					found = append(found, t)
				}
			}
			switch len(found) {
			case 1:
				u.LoaiMa, u.LoaiNhan = found[0].Code, found[0].Label
			case 0:
				rowErr(r, ResidentialUnitImportColType, "Không có loại đơn vị dân cư nào đang dùng khớp với ô này. Hãy chọn trong danh sách của tệp mẫu hoặc để trống.")
			default:
				rowErr(r, ResidentialUnitImportColType, "Có nhiều loại đơn vị dân cư cùng nhãn này. Hãy ghi MÃ của loại thay vì nhãn.")
			}
		}

		if v := strings.TrimSpace(r.Head); v != "" {
			code := v
			if i := strings.Index(v, strings.TrimSpace(HeadStaffSeparator)); i >= 0 {
				code = strings.TrimSpace(v[:i])
			}
			matched := false
			for _, s := range snap.Staff {
				if strings.EqualFold(code, s.Code) {
					u.HeadStaffID, u.HeadStaffCode, u.HeadStaffName = s.ID, s.Code, s.Name
					matched = true
					break
				}
			}
			if !matched {
				rowErr(r, ResidentialUnitImportColHead, "Không tìm thấy cán bộ đang làm việc của xã có mã ở ô này. Hãy chọn trong danh sách của tệp mẫu hoặc để trống.")
			}
		}

		if n, msg := parseCount(r.Households); msg != "" {
			rowErr(r, ResidentialUnitImportColHouseholds, msg)
		} else {
			u.SoHo = n
		}
		if n, msg := parseCount(r.Population); msg != "" {
			rowErr(r, ResidentialUnitImportColPopulation, msg)
		} else {
			u.NhanKhau = n
		}

		if o, msg := parseResidentialUnitOrder(r.Order); msg != "" {
			rowErr(r, ResidentialUnitImportColOrder, msg)
		} else {
			u.SortOrder = o
		}

		plan[i] = PlannedResidentialUnit{Row: r.Row, Unit: u}
	}

	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		return nil, errs
	}
	return plan, nil
}

// parseCount reads "Số hộ" / "Nhân khẩu". BLANK IS nil — "not entered" — AND NEVER 0: a commune whose
// file leaves the column empty must not publish zeros it never asserted (ThonToDanPho.SoHo).
//
// DIGITS ONLY. "1.132" is refused rather than read: in Vietnamese it is one thousand one hundred and
// thirty-two, in the spreadsheet's own locale it may be one point one three two, and guessing which is
// how a figure is silently wrong by a factor of a thousand. A number typed as a number reaches here as
// its stored digits (core/xlsx reads raw values), so the refusal only meets text.
func parseCount(raw string) (*int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, ""
	}
	const msg = "Phải là số nguyên từ 0 đến 10.000.000, chỉ gồm chữ số (không dấu chấm, dấu phẩy hay khoảng trắng)."
	if len(s) > 9 {
		return nil, msg
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return nil, msg
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || CheckResidentialUnitCount(&n) != nil {
		return nil, msg
	}
	return &n, ""
}

// parseResidentialUnitOrder reads "Thứ tự" — the org-chart import's reader (digits only, blank = 0),
// with this list's bound.
func parseResidentialUnitOrder(raw string) (int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ""
	}
	const msg = "Thứ tự phải là số nguyên từ 0 đến 9999 (chỉ gồm chữ số)."
	if len(s) > 4 {
		return 0, msg
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, msg
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || CheckResidentialUnitOrder(n) != nil {
		return 0, msg
	}
	return n, ""
}

// importSentence is a domain sentence without its "thon_to_dan_pho: " prefix, capitalised for a person.
func importSentence(err error) string {
	m := strings.TrimPrefix(err.Error(), "thon_to_dan_pho: ")
	if m == "" {
		return m
	}
	r := []rune(m)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r) + "."
}
