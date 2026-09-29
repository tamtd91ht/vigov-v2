package domain

// The rules of a STAFF IMPORT FROM EXCEL (14-cau-hinh.md §3, "⬆ Nhập từ Excel" on Người dùng; user
// decision 2026-09-29, ADR 0059 §1): each row CREATES a directory entry (code minted by the system, #15
// — the template has NO code column, ADR 0059 stop condition #4), optionally ASSIGNS a role, and ISSUES
// a sign-in account with a temporary password (#9). ALL OR NOTHING.
//
// THIS FILE ONLY PLANS. It turns the cells of one sheet plus a snapshot of the commune into either the
// people to create or EVERY error in the file — never both. Reading the workbook is core/xlsx (HTTP
// layer); minting codes and passwords, writing, and auditing in ONE transaction is app.StaffImporter.
// STANDARD LIBRARY ONLY.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS POST /api/v1/staff (danh_ba_ghi.go: ChuanHoaHoTen,
// ChuanHoaChucVu, ChuanHoaEmail, ChuanHoaSoDienThoai). Deliberate differences:
//
//	EMAIL IS OPTIONAL      a blank cell is stored NULL (migration 0019), so any number of people
//	                       without an address fit under UNIQUE (tenant_id, email). The address IS the
//	                       login (store.CanBoStore.TheoEmail), so a row WITHOUT one gets a directory
//	                       entry and NO account — there is nothing a person could type to sign in.
//	EMAIL MUST BE NEW      against EVERY row of the commune, soft-deleted included (the unique key is
//	                       not partial), and against the rows above. An import only ADDS people.
//	A SHARED NAME IS FINE  real people share names; the code tells them apart. Refusing would make
//	                       the second Nguyễn Văn An of a commune un-importable.
//	ROLE (#14 applies)     a role cell resolves to a live role of THIS commune, and the role must not
//	                       carry a key the importing administrator does not hold (#14, second
//	                       constraint) — the snapshot carries, per role, the keys that would be handed
//	                       out beyond the actor's own. `admin.role` itself is checked by the use case
//	                       before any plan, because it is about the FILE, not a row.
//
// NO MESSAGE ECHOES A CELL (rule 3, forbidden #3). Names, addresses and both telephone numbers travel
// in this file; the personal mobile is personal data under Decree 13 (#16). Every message names the
// row and the column, which is all a person needs to find the cell.

import (
	"fmt"
	"sort"
	"strings"
)

// The seven columns of the template, in order (../vigov-require catalogues.py:439-465 adjusted: the
// telephone split into two columns by #16, no code column by #15). THE HEADER ROW MUST MATCH THEM.
const (
	StaffImportColFullName    = "Họ và tên"
	StaffImportColEmail       = "Thư điện tử công vụ"
	StaffImportColPosition    = "Chức vụ"
	StaffImportColOrgUnit     = "Bộ phận"
	StaffImportColRole        = "Vai trò"
	StaffImportColOfficePhone = "Điện thoại cơ quan"
	StaffImportColMobile      = "Di động cá nhân"
)

// StaffImportColumns is the header row, in order.
func StaffImportColumns() []string {
	return []string{
		StaffImportColFullName, StaffImportColEmail, StaffImportColPosition, StaffImportColOrgUnit,
		StaffImportColRole, StaffImportColOfficePhone, StaffImportColMobile,
	}
}

// MaxStaffImportRows bounds one file. NOT the directory's ceiling — that is checked below — but the
// cost of the file: every row with an address costs one argon2id hash (19 MiB, tens of milliseconds),
// minted before the transaction opens. 200 rows is seconds, a whole commune's directory several times
// over (the customer's own list is 26).
const MaxStaffImportRows = 200

// ChoiceSeparator joins a code and a name in a template dropdown ("van-phong · VĂN PHÒNG"). The
// planner reads only what precedes it when present, so a NAME is never what identifies a row.
const ChoiceSeparator = HeadStaffSeparator

// StaffImportRow is one data row AS TYPED. Row is the spreadsheet's own number (header = 1).
type StaffImportRow struct {
	Row         int
	FullName    string
	Email       string
	Position    string
	OrgUnit     string
	Role        string
	OfficePhone string
	Mobile      string
}

// StaffImportError is one refusal. Row 0 and Column "" mean the FILE; Row 1 is the header.
type StaffImportError struct {
	Row     int
	Column  string
	Message string
}

// StaffImportOrgUnitChoice is one live org unit of the commune.
type StaffImportOrgUnitChoice struct {
	ID   string
	Code string
	Name string
}

// StaffImportRoleChoice is one live role of the commune. MissingKeys are the permission keys the role
// grants that the importing administrator does NOT hold (#14, second constraint) — non-empty means the
// role may not be handed out by this person. Filled by the use case inside the transaction.
type StaffImportRoleChoice struct {
	ID          string
	Code        string
	Name        string
	MissingKeys []string
}

// StaffImportSnapshot is everything the planner checks a file against, read inside the import's
// transaction. TakenEmails holds EVERY address of the commune, soft-deleted rows included, lower-cased.
type StaffImportSnapshot struct {
	TakenEmails  []string
	OrgUnits     []StaffImportOrgUnitChoice
	Roles        []StaffImportRoleChoice
	LiveStaff    int // rows not soft-deleted
	LiveAccounts int // rows not soft-deleted, with an account, not locked — the staff picker's set
}

// StaffImportCeilings are the store's bounds, passed in because this package does not import the store.
type StaffImportCeilings struct {
	Staff    int // live directory rows a commune may hold
	Accounts int // store.TranDanhBaChonNguoi — the picker REFUSES past it
}

// PlannedStaff is one person the import would create. Staff.ID and Staff.Ma are empty until minted.
type PlannedStaff struct {
	Row          int
	Staff        CanBoTomTat
	OrgUnitCode  string
	OrgUnitName  string
	RoleID       string
	RoleCode     string
	RoleName     string
	IssueAccount bool // true exactly when the row has an address — the address is the login
}

// StaffChoiceLabel is one dropdown value: code, separator, name.
func StaffChoiceLabel(code, name string) string { return code + ChoiceSeparator + name }

// NormalizeOptionalEmail is ChuanHoaEmail for a field that may be blank: "" stays "" (stored NULL).
func NormalizeOptionalEmail(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return ChuanHoaEmail(raw)
}

// StaffImportAssignsRoles reports whether any row carries a role cell — the file-level question the use
// case asks before planning (`admin.role`, ADR 0059 §1).
func StaffImportAssignsRoles(rows []StaffImportRow) bool {
	for _, r := range rows {
		if strings.TrimSpace(r.Role) != "" {
			return true
		}
	}
	return false
}

// StaffRowsFromSheet turns the raw cells of the first sheet into data rows, or refuses the SHEET when
// its header is not the template's. Entirely blank rows are skipped and keep the numbering around them.
func StaffRowsFromSheet(cells [][]string) ([]StaffImportRow, []StaffImportError) {
	cols := StaffImportColumns()
	if len(cells) == 0 {
		return nil, []StaffImportError{{Message: "Tệp không có dòng nào."}}
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
		return nil, []StaffImportError{{Row: 1,
			Message: "Dòng tiêu đề không khớp tệp mẫu (" + strings.Join(cols, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var rows []StaffImportRow
	var errs []StaffImportError
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
			errs = append(errs, StaffImportError{Row: i + 1,
				Message: fmt.Sprintf("Dòng có dữ liệu ngoài %d cột của tệp mẫu.", len(cols))})
			continue
		}
		at := func(j int) string {
			if j < len(c) {
				return c[j]
			}
			return ""
		}
		rows = append(rows, StaffImportRow{
			Row: i + 1, FullName: at(0), Email: at(1), Position: at(2), OrgUnit: at(3), Role: at(4),
			OfficePhone: at(5), Mobile: at(6),
		})
	}
	return rows, errs
}

// PlanStaffImport validates the WHOLE file and returns either the plan or every error — the plan is nil
// whenever there is at least one error, so a caller cannot write half a file by accident.
func PlanStaffImport(rows []StaffImportRow, snap StaffImportSnapshot, ceil StaffImportCeilings) ([]PlannedStaff, []StaffImportError) {
	var errs []StaffImportError
	fileErr := func(msg string) { errs = append(errs, StaffImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return nil, errs
	}
	if len(rows) > MaxStaffImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập.", len(rows), MaxStaffImportRows))
		return nil, errs
	}

	taken := make(map[string]bool, len(snap.TakenEmails))
	for _, e := range snap.TakenEmails {
		taken[strings.ToLower(strings.TrimSpace(e))] = true
	}

	rowErr := func(r StaffImportRow, col, msg string) {
		errs = append(errs, StaffImportError{Row: r.Row, Column: col, Message: msg})
	}
	plan := make([]PlannedStaff, 0, len(rows))
	emailRow := make(map[string]int, len(rows))
	accounts := 0
	for _, r := range rows {
		p := PlannedStaff{Row: r.Row}

		if name, err := ChuanHoaHoTen(r.FullName); err != nil {
			rowErr(r, StaffImportColFullName, staffImportSentence(err))
		} else {
			p.Staff.HoTen = name
		}

		if email, err := NormalizeOptionalEmail(r.Email); err != nil {
			rowErr(r, StaffImportColEmail, staffImportSentence(err))
		} else if email != "" {
			switch first, dup := emailRow[email]; {
			case taken[email]:
				rowErr(r, StaffImportColEmail, "Thư điện tử này đã có trong danh bạ của xã (kể cả dòng đã xoá). Nhập tệp chỉ THÊM người mới, không cập nhật người đang có.")
			case dup:
				rowErr(r, StaffImportColEmail, fmt.Sprintf("Trùng thư điện tử với dòng %d trong tệp.", first))
			default:
				emailRow[email] = r.Row
				p.Staff.Email = email
				p.IssueAccount = true
				accounts++
			}
		}

		if pos, err := ChuanHoaChucVu(r.Position); err != nil {
			rowErr(r, StaffImportColPosition, staffImportSentence(err))
		} else {
			p.Staff.ChucVu = pos
		}

		if v := strings.TrimSpace(r.OrgUnit); v != "" {
			var found []StaffImportOrgUnitChoice
			code, byCode := choiceCode(v)
			for _, u := range snap.OrgUnits {
				if code == u.Code || (!byCode && foldName(v) == foldName(u.Name)) {
					found = append(found, u)
				}
			}
			switch len(found) {
			case 1:
				p.Staff.BoPhanID, p.OrgUnitCode, p.OrgUnitName = found[0].ID, found[0].Code, found[0].Name
			case 0:
				rowErr(r, StaffImportColOrgUnit, "Không có bộ phận nào của xã khớp với ô này. Hãy chọn trong danh sách của tệp mẫu hoặc để trống.")
			default:
				rowErr(r, StaffImportColOrgUnit, "Có nhiều bộ phận cùng tên này. Hãy chọn trong danh sách của tệp mẫu (có kèm mã).")
			}
		}

		if v := strings.TrimSpace(r.Role); v != "" {
			var found []StaffImportRoleChoice
			code, byCode := choiceCode(v)
			for _, ro := range snap.Roles {
				if code == ro.Code || (!byCode && foldName(v) == foldName(ro.Name)) {
					found = append(found, ro)
				}
			}
			switch {
			case len(found) == 0:
				rowErr(r, StaffImportColRole, "Không có vai trò nào của xã khớp với ô này. Hãy chọn trong danh sách của tệp mẫu hoặc để trống.")
			case len(found) > 1:
				rowErr(r, StaffImportColRole, "Có nhiều vai trò cùng tên này. Hãy chọn trong danh sách của tệp mẫu (có kèm mã).")
			case len(found[0].MissingKeys) > 0:
				// The keys are named: they are not personal data (the Phân quyền screen prints them), and
				// "which key" is what the administrator must ask their own superior for.
				rowErr(r, StaffImportColRole, "Vai trò này mang quyền bạn không giữ ("+strings.Join(found[0].MissingKeys, ", ")+") — không được trao vai trò mạnh hơn quyền của chính mình.")
			default:
				p.RoleID, p.RoleCode, p.RoleName = found[0].ID, found[0].Code, found[0].Name
			}
		}

		if tel, err := ChuanHoaSoDienThoai(r.OfficePhone); err != nil {
			rowErr(r, StaffImportColOfficePhone, staffImportSentence(err))
		} else {
			p.Staff.DienThoaiCoQuan = tel
		}
		if tel, err := ChuanHoaSoDienThoai(r.Mobile); err != nil {
			// The domain sentence names the rule and never the character (ChuanHoaSoDienThoai).
			rowErr(r, StaffImportColMobile, staffImportSentence(err))
		} else {
			p.Staff.DiDongCaNhan = tel
		}

		plan = append(plan, p)
	}

	if ceil.Staff > 0 && snap.LiveStaff+len(rows) > ceil.Staff {
		fileErr(fmt.Sprintf("Danh bạ của xã đang có %d người; thêm %d người sẽ vượt trần %d.", snap.LiveStaff, len(rows), ceil.Staff))
	}
	if ceil.Accounts > 0 && snap.LiveAccounts+accounts > ceil.Accounts {
		// The staff picker REFUSES a commune past this number (store.ErrQuaNhieuCanBoChonNguoi): an import
		// that crossed it would empty every assignee box in the commune.
		fileErr(fmt.Sprintf("Xã đang có %d tài khoản; cấp thêm %d tài khoản sẽ vượt trần %d của danh sách chọn người.", snap.LiveAccounts, accounts, ceil.Accounts))
	}

	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		return nil, errs
	}
	return plan, nil
}

// choiceCode reads a dropdown value: the code before the separator when there is one (byCode true), or
// the whole trimmed cell (byCode false — it may be a typed code OR a name).
func choiceCode(v string) (string, bool) {
	if i := strings.Index(v, strings.TrimSpace(ChoiceSeparator)); i >= 0 {
		return strings.TrimSpace(v[:i]), true
	}
	return v, false
}

// staffImportSentence is a domain sentence without its "can_bo: " prefix, capitalised for a person. The
// sentinels of danh_ba_ghi.go name the rule and the bound, never the value.
func staffImportSentence(err error) string {
	m := strings.TrimPrefix(err.Error(), "can_bo: ")
	if m == "" {
		return m
	}
	r := []rune(m)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r) + "."
}
