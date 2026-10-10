package domain

// The rules of an org-chart IMPORT FROM EXCEL (14-cau-hinh.md §1, "Nhập từ Excel"; user decision
// 2026-09-28): org units only, ALL OR NOTHING, create only.
//
// THIS FILE ONLY PLANS. It turns the rows of one file plus a snapshot of the commune's chart into
// either a list of units to create or a list of every error in the file — never both. Reading the
// workbook is internal/orgunitxlsx; writing the plan, with its audit entries, in one transaction is
// app.OrgUnitImporter. STANDARD LIBRARY ONLY, like the rest of this package.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/org-units — ChuanHoaTenBoPhan,
// ChuanHoaMaBoPhan, SinhMaBoPhan, KiemTraThuTuBoPhan — so a unit that could not be typed into the
// form cannot be imported either. What differs from app.SoDoToChuc.Them is deliberate:
//
//   - NO `-2` SUFFIX ON A GENERATED CODE. Them suffixes because one person adding one unit wants it
//     added; a file imported twice must NOT produce a second tree of `van-phong-2`, `lanh-dao-2`…
//     with permanent codes (rule 7, invariant 3). A generated code already issued — soft-deleted
//     units included, because `UNIQUE (tenant_id, ma)` counts them (0001_init.sql:86) — is a row
//     error, and the person types a code.
//   - A SIBLING WITH THE SAME NAME IS AN ERROR, whether the sibling is in the file or already in the
//     chart. Them allows it; an import that allowed it would be the second-tree failure again with
//     typed codes.
//   - THE PARENT MUST BE A LIVE UNIT OR A ROW ABOVE. "Above" is what makes a cycle impossible by
//     construction: every edge points upward in the file, and existing units cannot point at rows
//     that do not exist yet.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): unit names and codes describe the authority's
// organisation. The messages still never echo a cell's content — they name the row and the column,
// which is all a person needs to find it, and the habit costs nothing.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The four columns of the template, in order. THE HEADER ROW MUST MATCH THEM: a file whose first
// row says something else is a different file (a staff list, last year's export), and guessing which
// column is which would import the wrong column into an archival record.
const (
	OrgUnitImportColName   = "Tên bộ phận"
	OrgUnitImportColParent = "Thuộc bộ phận"
	OrgUnitImportColCode   = "Mã (để trống thì tự sinh)"
	OrgUnitImportColOrder  = "Thứ tự"
)

// OrgUnitImportColumns is the header row, in order.
func OrgUnitImportColumns() []string {
	return []string{OrgUnitImportColName, OrgUnitImportColParent, OrgUnitImportColCode, OrgUnitImportColOrder}
}

// MaxOrgUnitImportRows bounds one file. The whole commune's chart is bounded by
// store.TranDanhMucBoPhan (500) and a real one has about ten units; a file of more rows than the
// ceiling cannot be imported whatever it holds, so it is refused before any row is read.
const MaxOrgUnitImportRows = 500

// OrgUnitImportRow is one data row of the file AS TYPED: four cells, as strings, never evaluated.
// Row is the spreadsheet's own row number (the header is row 1), so an error points where the person
// looks.
type OrgUnitImportRow struct {
	Row    int
	Name   string
	Parent string
	Code   string
	Order  string
}

// OrgUnitImportError is one refusal. Row 0 and Column "" mean the FILE, not a row (too many rows, a
// wrong header, a chart that would exceed its ceiling). Message is Vietnamese, for a person.
type OrgUnitImportError struct {
	Row     int
	Column  string
	Message string
}

// PlannedOrgUnit is one unit the import would create.
//
// Unit.ChaID is set when the parent is an EXISTING unit; ParentRow is set (the spreadsheet row of
// the parent) when the parent is a row of the same file, whose id does not exist until it is
// inserted. Both empty = the root. ParentCode is the parent's code either way, for the preview.
type PlannedOrgUnit struct {
	Row        int
	Unit       BoPhan
	ParentRow  int
	ParentCode string
}

// ExistingOrgUnit is one row of the commune's chart as the planner sees it. SOFT-DELETED ROWS ARE
// INCLUDED, flagged: their codes are still taken, but they are never a parent and never a sibling.
type ExistingOrgUnit struct {
	ID       string
	Code     string
	Name     string
	ParentID string
	Deleted  bool
	// Order is the unit's rank (`thu_tu`). Filled by the sibling read (store.LiveSiblings) for
	// NextSiblingOrder; the import snapshot does not read it and leaves it 0.
	Order int
}

// OrgUnitImportCeiling is the chart's ceiling (store.TranDanhMucBoPhan), passed in because this
// package does not import the store.
//
// PlanOrgUnitImport validates the WHOLE file and returns either the plan or every error — the plan
// is nil whenever there is at least one error, so a caller cannot write half a file by accident.
func PlanOrgUnitImport(rows []OrgUnitImportRow, existing []ExistingOrgUnit, ceiling int) ([]PlannedOrgUnit, []OrgUnitImportError) {
	var errs []OrgUnitImportError
	fileErr := func(msg string) { errs = append(errs, OrgUnitImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return nil, errs
	}
	if len(rows) > MaxOrgUnitImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập.", len(rows), MaxOrgUnitImportRows))
		return nil, errs
	}

	live := 0
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Code] = true
		if !e.Deleted {
			live++
		}
	}
	if live+len(rows) > ceiling {
		// Refused here rather than discovered by GET /api/v1/org-units, which REFUSES a chart over
		// the ceiling — an import that crossed it would break every assignment box in the commune.
		fileErr(fmt.Sprintf("Xã đang có %d bộ phận; thêm %d bộ phận sẽ vượt trần %d của sơ đồ tổ chức.", live, len(rows), ceiling))
	}

	// --- pass 1: the shape of each row, exactly as the create form checks it ----------------------
	type shaped struct {
		name, code string
		order      int
		nameOK     bool
		codeOK     bool
		typedCode  bool
	}
	sh := make([]shaped, len(rows))
	rowErr := func(i int, col, msg string) {
		errs = append(errs, OrgUnitImportError{Row: rows[i].Row, Column: col, Message: msg})
	}
	for i, r := range rows {
		s := &sh[i]
		if name, err := ChuanHoaTenBoPhan(r.Name); err != nil {
			rowErr(i, OrgUnitImportColName, importMessage(err))
		} else {
			s.name, s.nameOK = name, true
		}

		if strings.TrimSpace(r.Code) != "" {
			s.typedCode = true
			if code, err := ChuanHoaMaBoPhan(r.Code); err != nil {
				rowErr(i, OrgUnitImportColCode, importMessage(err))
			} else {
				s.code, s.codeOK = code, true
			}
		} else if s.nameOK {
			if code, err := SinhMaBoPhan(s.name); err != nil {
				rowErr(i, OrgUnitImportColCode, importMessage(err))
			} else {
				s.code, s.codeOK = code, true
			}
		}

		if o, msg := parseImportOrder(r.Order); msg != "" {
			rowErr(i, OrgUnitImportColOrder, msg)
		} else {
			s.order = o
		}
	}

	// --- pass 2: codes — never an issued one, never twice in the file ----------------------------
	codeRow := make(map[string]int, len(rows))
	for i := range rows {
		s := sh[i]
		if !s.codeOK {
			continue
		}
		if taken[s.code] {
			if s.typedCode {
				rowErr(i, OrgUnitImportColCode, "Mã này đã được cấp trong xã (kể cả cho bộ phận đã xoá — mã đã cấp không cấp lại).")
			} else {
				rowErr(i, OrgUnitImportColCode, "Mã tự sinh từ tên đã được cấp trong xã (kể cả cho bộ phận đã xoá). Nhập lại tệp không tạo bộ phận trùng; nếu đây là bộ phận mới, hãy nhập một mã khác.")
			}
			continue
		}
		if first, dup := codeRow[s.code]; dup {
			if s.typedCode {
				rowErr(i, OrgUnitImportColCode, fmt.Sprintf("Mã trùng với dòng %d trong tệp.", first))
			} else {
				rowErr(i, OrgUnitImportColCode, fmt.Sprintf("Mã tự sinh trùng với mã của dòng %d trong tệp — hãy nhập một mã khác.", first))
			}
			continue
		}
		codeRow[s.code] = rows[i].Row
	}

	// --- pass 3: parents — a live unit, or a row ABOVE ------------------------------------------
	type ref struct {
		existingID string // set for a live unit
		row        int    // set for a row of the file
		code       string
	}
	liveUnits := make([]ExistingOrgUnit, 0, live)
	for _, e := range existing {
		if !e.Deleted {
			liveUnits = append(liveUnits, e)
		}
	}
	match := func(v string, code, name string) bool {
		return (code != "" && v == code) || (name != "" && foldName(v) == foldName(name))
	}

	parents := make([]ref, len(rows))
	// unresolved marks a row whose parent cell is filled but matched nothing usable: its sibling
	// check is skipped, or the row would also be reported as a duplicate at the ROOT it is not in.
	unresolved := make([]bool, len(rows))
	for i, r := range rows {
		v := strings.TrimSpace(r.Parent)
		if v == "" {
			continue
		}
		unresolved[i] = true
		var found []ref
		for _, e := range liveUnits {
			if match(v, e.Code, e.Name) {
				found = append(found, ref{existingID: e.ID, code: e.Code})
			}
		}
		for j := 0; j < i; j++ {
			if match(v, sh[j].code, sh[j].name) {
				found = append(found, ref{row: rows[j].Row, code: sh[j].code})
			}
		}
		switch {
		case len(found) == 1:
			parents[i], unresolved[i] = found[0], false
		case len(found) > 1:
			rowErr(i, OrgUnitImportColParent, "Có nhiều bộ phận cùng tên này (trong xã hoặc trong tệp). Hãy ghi MÃ của bộ phận cha thay vì tên.")
		default:
			below := false
			for j := i; j < len(rows); j++ {
				if match(v, sh[j].code, sh[j].name) {
					below = true
					break
				}
			}
			if below {
				rowErr(i, OrgUnitImportColParent, "Bộ phận cha phải nằm ở một dòng PHÍA TRÊN dòng này (hoặc đã có trong xã).")
			} else {
				rowErr(i, OrgUnitImportColParent, "Không tìm thấy bộ phận cha: không khớp mã hay tên của bộ phận nào đang có trong xã, cũng không khớp dòng nào phía trên.")
			}
		}
	}

	// --- pass 4: siblings — one name once under one parent --------------------------------------
	siblingKey := func(parent ref, name string) string {
		switch {
		case parent.existingID != "":
			return "u:" + parent.existingID + "\x00" + foldName(name)
		case parent.row != 0:
			return "r:" + strconv.Itoa(parent.row) + "\x00" + foldName(name)
		default:
			return "root\x00" + foldName(name)
		}
	}
	siblings := make(map[string]int, len(liveUnits)+len(rows))
	for _, e := range liveUnits {
		siblings[siblingKey(ref{existingID: e.ParentID}, e.Name)] = 0
	}
	for i := range rows {
		if !sh[i].nameOK || unresolved[i] {
			continue
		}
		k := siblingKey(parents[i], sh[i].name)
		if first, dup := siblings[k]; dup {
			if first == 0 {
				rowErr(i, OrgUnitImportColName, "Bộ phận cha này đã có một bộ phận cùng tên trong xã. Nhập tệp chỉ THÊM bộ phận mới, không cập nhật bộ phận đang có.")
			} else {
				rowErr(i, OrgUnitImportColName, fmt.Sprintf("Trùng tên với dòng %d dưới cùng một bộ phận cha.", first))
			}
			continue
		}
		siblings[k] = rows[i].Row
	}

	if len(errs) > 0 {
		sortImportErrors(errs)
		return nil, errs
	}

	plan := make([]PlannedOrgUnit, len(rows))
	for i, r := range rows {
		plan[i] = PlannedOrgUnit{
			Row:        r.Row,
			Unit:       BoPhan{Ten: sh[i].name, Ma: sh[i].code, ChaID: parents[i].existingID, ThuTu: sh[i].order},
			ParentRow:  parents[i].row,
			ParentCode: parents[i].code,
		}
	}
	return plan, nil
}

// parseImportOrder reads "Thứ tự". Blank = 0, the column's default, as on the create form.
//
// DIGITS ONLY, and that is the point of writing it out: vigov-require's parser strips everything
// that is not a digit, so "1.5" becomes 15 — a rank nobody typed. Here "1.5", "1e3", "3 " inside a
// word, "٣" are all refused. A leading '-' is read so the refusal can say "negative" rather than
// "not a number".
func parseImportOrder(raw string) (int, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ""
	}
	digits := strings.TrimPrefix(s, "-")
	if digits == "" || len(digits) > 9 {
		return 0, fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d.", TranThuTuBoPhan)
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return 0, fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d (không có phần thập phân).", TranThuTuBoPhan)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d.", TranThuTuBoPhan)
	}
	if err := KiemTraThuTuBoPhan(n); err != nil {
		return 0, fmt.Sprintf("Thứ tự phải là số nguyên từ 0 đến %d.", TranThuTuBoPhan)
	}
	return n, ""
}

// foldName is the comparison key for a unit name: trimmed, case-folded. NOT Unicode-normalised —
// the standard library has no NFC, so a name typed in decomposed form does not match its composed
// twin; that refuses (not found), it never matches the wrong unit.
func foldName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// importMessage is the domain sentence without its "bo_phan: " prefix, capitalised for a person.
func importMessage(err error) string {
	m := strings.TrimPrefix(err.Error(), "bo_phan: ")
	if m == "" {
		return m
	}
	r := []rune(m)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r) + "."
}

// sortImportErrors orders by row, file-level errors first; STABLE, so within a row the columns keep
// the order they were checked in.
func sortImportErrors(errs []OrgUnitImportError) {
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
}
