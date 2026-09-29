package domain

import (
	"strings"
	"testing"
)

// The agreed fake number (rule 3, invariant 5).
const fakeMobile = "0900000000"

func staffSnapshot() StaffImportSnapshot {
	return StaffImportSnapshot{
		TakenEmails: []string{"da.co@xa.gov.vn", "da.xoa@xa.gov.vn"},
		OrgUnits: []StaffImportOrgUnitChoice{
			{ID: "bp-1", Code: "van-phong", Name: "VĂN PHÒNG"},
			{ID: "bp-2", Code: "to-a", Name: "TỔ TRÙNG"},
			{ID: "bp-3", Code: "to-b", Name: "TỔ TRÙNG"},
		},
		Roles: []StaffImportRoleChoice{
			{ID: "vt-1", Code: "chuyen-vien", Name: "Chuyên viên"},
			{ID: "vt-2", Code: "chu-tich", Name: "Chủ tịch", MissingKeys: []string{"budget.confirm"}},
		},
		LiveStaff: 3, LiveAccounts: 2,
	}
}

var staffCeil = StaffImportCeilings{Staff: 2000, Accounts: 500}

func TestStaffRowsFromSheet_HeaderMustMatch(t *testing.T) {
	// The requirements repository's header (one "Di động" column, email required) is NOT this template.
	if _, errs := StaffRowsFromSheet([][]string{{"Họ và tên", "Thư điện tử công vụ", "Chức vụ", "Bộ phận", "Vai trò", "Di động"}}); len(errs) != 1 || errs[0].Row != 1 {
		t.Fatalf("tiêu đề cũ phải bị từ chối: %+v", errs)
	}
	rows, errs := StaffRowsFromSheet([][]string{StaffImportColumns(),
		{"Nguyễn Văn A", "a@xa.gov.vn", "Chuyên viên", "", "", "", fakeMobile},
		{},
		{"B", "", "", "", "", "", "", fakeMobile}})
	if len(rows) != 1 || rows[0].Row != 2 || rows[0].Mobile != fakeMobile {
		t.Errorf("dòng: %+v", rows)
	}
	if len(errs) != 1 || errs[0].Row != 4 || strings.Contains(errs[0].Message, fakeMobile) {
		t.Errorf("ô thừa: %+v", errs)
	}
}

func TestPlanStaffImport_Plans(t *testing.T) {
	rows := []StaffImportRow{
		{Row: 2, FullName: "  Nguyễn  Văn   An ", Email: "An.Nguyen@XA.gov.vn", Position: "Chuyên viên",
			OrgUnit: "van-phong · VĂN PHÒNG", Role: "chuyen-vien · Chuyên viên", OfficePhone: "0200 000 000", Mobile: fakeMobile},
		// Same name, no address: allowed — a directory row with no account.
		{Row: 3, FullName: "Nguyễn Văn An", OrgUnit: "VĂN PHÒNG", Role: "Chuyên viên"},
		{Row: 4, FullName: "Trần Thị B", Email: "b@xa.gov.vn"},
	}
	plan, errs := PlanStaffImport(rows, staffSnapshot(), staffCeil)
	if len(errs) != 0 || len(plan) != 3 {
		t.Fatalf("plan %+v errs %+v", plan, errs)
	}
	a := plan[0]
	if a.Staff.HoTen != "Nguyễn Văn An" || a.Staff.Email != "an.nguyen@xa.gov.vn" || !a.IssueAccount ||
		a.Staff.BoPhanID != "bp-1" || a.RoleID != "vt-1" || a.Staff.DiDongCaNhan != fakeMobile || a.Staff.DienThoaiCoQuan != "0200 000 000" {
		t.Errorf("dòng 2: %+v", a)
	}
	if b := plan[1]; b.IssueAccount || b.Staff.Email != "" || b.Staff.BoPhanID != "bp-1" || b.RoleID != "vt-1" {
		t.Errorf("dòng 3 (không thư điện tử → không tài khoản): %+v", b)
	}
	if plan[0].Staff.Ma != "" || plan[0].Staff.ID != "" {
		t.Error("mã và id do use case sinh, planner không đặt")
	}
	if !StaffImportAssignsRoles(rows) || StaffImportAssignsRoles(rows[2:]) {
		t.Error("StaffImportAssignsRoles")
	}
}

func TestPlanStaffImport_EveryErrorNoPlanNoEcho(t *testing.T) {
	rows := []StaffImportRow{
		{Row: 2, FullName: ""},
		{Row: 3, FullName: "X", Email: "DA.CO@xa.gov.vn"},                         // taken, case-folded
		{Row: 4, FullName: "X", Email: "da.xoa@xa.gov.vn"},                        // soft-deleted row's address
		{Row: 5, FullName: "X", Email: "moi@xa.gov.vn"},                           // ok
		{Row: 6, FullName: "X", Email: "moi@xa.gov.vn"},                           // repeats row 5
		{Row: 7, FullName: "X", Email: "khong-hop-le"},                            // bad shape
		{Row: 8, FullName: "X", OrgUnit: "TỔ TRÙNG"},                              // ambiguous name
		{Row: 9, FullName: "X", OrgUnit: "khong-co"},                              // unknown
		{Row: 10, FullName: "X", Role: "chu-tich · Chủ tịch"},                     // #14 second constraint
		{Row: 11, FullName: "X", Role: "Không có"},                                // unknown
		{Row: 12, FullName: "X", Mobile: fakeMobile + "<script>"},                 // bad character
		{Row: 13, FullName: "X", OfficePhone: strings.Repeat("0", 40)},            // too long
		{Row: 14, FullName: strings.Repeat("A", 151)},                             // too long
		{Row: 15, FullName: "X", OrgUnit: "to-b · TỔ TRÙNG", Role: "chuyen-vien"}, // ok: by code
	}
	plan, errs := PlanStaffImport(rows, staffSnapshot(), staffCeil)
	if plan != nil {
		t.Fatal("có lỗi mà vẫn có kế hoạch")
	}
	got := map[int]string{}
	for _, e := range errs {
		got[e.Row] += e.Column + ";"
		if strings.Contains(e.Message, fakeMobile) || strings.Contains(e.Message, "da.co") || strings.Contains(e.Message, "<script>") {
			t.Errorf("thông báo nhắc lại nội dung ô: %q", e.Message)
		}
	}
	want := map[int]string{
		2: StaffImportColFullName, 3: StaffImportColEmail, 4: StaffImportColEmail, 6: StaffImportColEmail,
		7: StaffImportColEmail, 8: StaffImportColOrgUnit, 9: StaffImportColOrgUnit, 10: StaffImportColRole,
		11: StaffImportColRole, 12: StaffImportColMobile, 13: StaffImportColOfficePhone, 14: StaffImportColFullName,
	}
	for row, col := range want {
		if !strings.Contains(got[row], col) {
			t.Errorf("dòng %d: lỗi %q, muốn cột %q", row, got[row], col)
		}
	}
	for _, ok := range []int{5, 15} {
		if g, bad := got[ok]; bad {
			t.Errorf("dòng %d hợp lệ mà có lỗi %q", ok, g)
		}
	}
	for _, e := range errs {
		if e.Row == 10 && !strings.Contains(e.Message, "budget.confirm") {
			t.Errorf("lỗi #14 phải nêu khoá thiếu: %q", e.Message)
		}
	}
}

func TestPlanStaffImport_Ceilings(t *testing.T) {
	snap := staffSnapshot()
	snap.LiveAccounts = 499
	_, errs := PlanStaffImport([]StaffImportRow{
		{Row: 2, FullName: "A", Email: "a@xa.gov.vn"}, {Row: 3, FullName: "B", Email: "b@xa.gov.vn"},
	}, snap, staffCeil)
	if len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("vượt trần tài khoản phải là lỗi cả tệp: %+v", errs)
	}
	// Rows without an address do not count against the account ceiling.
	if _, errs := PlanStaffImport([]StaffImportRow{{Row: 2, FullName: "A"}, {Row: 3, FullName: "B"}}, snap, staffCeil); len(errs) != 0 {
		t.Errorf("dòng không tài khoản: %+v", errs)
	}
	snap.LiveStaff = 1999
	if _, errs := PlanStaffImport([]StaffImportRow{{Row: 2, FullName: "A"}, {Row: 3, FullName: "B"}}, snap, staffCeil); len(errs) != 1 {
		t.Errorf("vượt trần danh bạ: %+v", errs)
	}
	many := make([]StaffImportRow, MaxStaffImportRows+1)
	for i := range many {
		many[i] = StaffImportRow{Row: i + 2, FullName: "A"}
	}
	if _, errs := PlanStaffImport(many, staffSnapshot(), staffCeil); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("quá số dòng: %+v", errs)
	}
}

func TestNormalizeOptionalEmail(t *testing.T) {
	if v, err := NormalizeOptionalEmail("   "); err != nil || v != "" {
		t.Errorf("trống: %q %v", v, err)
	}
	if v, err := NormalizeOptionalEmail(" A@B.vn "); err != nil || v != "a@b.vn" {
		t.Errorf("chuẩn hoá: %q %v", v, err)
	}
	if _, err := NormalizeOptionalEmail("a@b"); err == nil {
		t.Error("sai định dạng phải bị từ chối")
	}
}
