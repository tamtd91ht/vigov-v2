package domain

import (
	"strings"
	"testing"
)

// The planner of the org-chart import. Every case names the rule it holds; the ones that differ
// from POST /api/v1/org-units (no suffix, sibling names, parent above) are the ones a plausible
// reimplementation would get wrong.

func chartForImport() []ExistingOrgUnit {
	return []ExistingOrgUnit{
		{ID: "u-ld", Code: "lanh-dao", Name: "LÃNH ĐẠO"},
		{ID: "u-vp", Code: "van-phong", Name: "VĂN PHÒNG", ParentID: "u-ld"},
		// Two live units with ONE name under different parents: a name reference to it is ambiguous.
		{ID: "u-tk1", Code: "to-ky-thuat", Name: "TỔ KỸ THUẬT", ParentID: "u-ld"},
		{ID: "u-tk2", Code: "to-ky-thuat-2", Name: "Tổ kỹ thuật", ParentID: "u-vp"},
		// Soft-deleted: its code is taken, it is never a parent and never a sibling.
		{ID: "u-old", Code: "da-giai-the", Name: "ĐÃ GIẢI THỂ", Deleted: true},
	}
}

func row(n int, name, parent, code, order string) OrgUnitImportRow {
	return OrgUnitImportRow{Row: n, Name: name, Parent: parent, Code: code, Order: order}
}

// errorAt reports whether errs holds one for (row, column) whose message contains frag.
func errorAt(errs []OrgUnitImportError, r int, col, frag string) bool {
	for _, e := range errs {
		if e.Row == r && e.Column == col && strings.Contains(e.Message, frag) {
			return true
		}
	}
	return false
}

func TestPlanOrgUnitImport_TwoLevelTreeInOneFile(t *testing.T) {
	rows := []OrgUnitImportRow{
		row(2, "  VĂN PHÒNG ĐẢNG ỦY ", "", "", "3"),
		row(3, "TỔ TỔNG HỢP", "văn phòng đảng ủy", "", ""),        // parent by NAME, a row above
		row(4, "TỔ LƯU TRỮ", "van-phong-dang-uy", "luu-tru", "7"), // parent by CODE, a row above
		row(5, "BỘ PHẬN MỚI", "LÃNH ĐẠO", "", "0"),                // parent: a live unit, by name
	}
	plan, errs := PlanOrgUnitImport(rows, chartForImport(), 500)
	if len(errs) != 0 {
		t.Fatalf("lỗi không mong đợi: %+v", errs)
	}
	if len(plan) != 4 {
		t.Fatalf("kế hoạch có %d dòng, muốn 4", len(plan))
	}
	if u := plan[0].Unit; u.Ten != "VĂN PHÒNG ĐẢNG ỦY" || u.Ma != "van-phong-dang-uy" || u.ThuTu != 3 || u.ChaID != "" || plan[0].ParentRow != 0 {
		t.Errorf("dòng 2: %+v", plan[0])
	}
	if plan[1].ParentRow != 2 || plan[1].Unit.ChaID != "" || plan[1].ParentCode != "van-phong-dang-uy" || plan[1].Unit.Ma != "to-tong-hop" {
		t.Errorf("dòng 3 phải trỏ tới dòng 2 của tệp: %+v", plan[1])
	}
	if plan[2].ParentRow != 2 || plan[2].Unit.Ma != "luu-tru" || plan[2].Unit.ThuTu != 7 {
		t.Errorf("dòng 4: %+v", plan[2])
	}
	if plan[3].Unit.ChaID != "u-ld" || plan[3].ParentRow != 0 {
		t.Errorf("dòng 5 phải trỏ tới bộ phận đang có u-ld: %+v", plan[3])
	}
}

func TestPlanOrgUnitImport_RowRules(t *testing.T) {
	long := strings.Repeat("A", TranTenBoPhan+1)
	cases := []struct {
		name string
		rows []OrgUnitImportRow
		r    int
		col  string
		frag string
	}{
		{"tên trống", []OrgUnitImportRow{row(2, "   ", "", "", "")}, 2, OrgUnitImportColName, "Thiếu tên"},
		{"tên quá dài", []OrgUnitImportRow{row(2, long, "", "x", "")}, 2, OrgUnitImportColName, "quá dài"},
		{"tên có ký tự điều khiển", []OrgUnitImportRow{row(2, "VĂN\nPHÒNG", "", "", "")}, 2, OrgUnitImportColName, "điều khiển"},
		{"mã sai định dạng, không sửa hộ", []OrgUnitImportRow{row(2, "VĂN PHÒNG MỚI", "", "Van-Phong", "")}, 2, OrgUnitImportColCode, "chữ thường"},
		{"tên không sinh được mã", []OrgUnitImportRow{row(2, "!!!", "", "", "")}, 2, OrgUnitImportColCode, "sinh được mã"},
		// vigov-require turns "1.5" into 15. It must be an error here.
		{"thứ tự 1.5", []OrgUnitImportRow{row(2, "A MỚI", "", "", "1.5")}, 2, OrgUnitImportColOrder, "số nguyên"},
		{"thứ tự 1e3", []OrgUnitImportRow{row(2, "A MỚI", "", "", "1e3")}, 2, OrgUnitImportColOrder, "số nguyên"},
		{"thứ tự âm", []OrgUnitImportRow{row(2, "A MỚI", "", "", "-1")}, 2, OrgUnitImportColOrder, "số nguyên"},
		{"thứ tự quá lớn", []OrgUnitImportRow{row(2, "A MỚI", "", "", "10000")}, 2, OrgUnitImportColOrder, "9999"},
		{"thứ tự chữ", []OrgUnitImportRow{row(2, "A MỚI", "", "", "một")}, 2, OrgUnitImportColOrder, "số nguyên"},
		{"mã gõ tay đã cấp cho bộ phận ĐÃ XOÁ", []OrgUnitImportRow{row(2, "MỚI", "", "da-giai-the", "")}, 2, OrgUnitImportColCode, "đã xoá"},
		// Import twice must not create `lanh-dao-2`: the generated code is refused, not suffixed.
		{"mã tự sinh đã cấp — không thêm hậu tố", []OrgUnitImportRow{row(2, "ĐÃ GIẢI THỂ", "", "", "")}, 2, OrgUnitImportColCode, "tự sinh"},
		{"cha không tồn tại", []OrgUnitImportRow{row(2, "MỚI", "không-có", "", "")}, 2, OrgUnitImportColParent, "Không tìm thấy"},
		{"cha là bộ phận đã xoá mềm", []OrgUnitImportRow{row(2, "MỚI", "da-giai-the", "", "")}, 2, OrgUnitImportColParent, "Không tìm thấy"},
		{"cha theo tên bị mơ hồ", []OrgUnitImportRow{row(2, "MỚI", "tổ kỹ thuật", "", "")}, 2, OrgUnitImportColParent, "nhiều bộ phận cùng tên"},
		{"cha là dòng PHÍA DƯỚI", []OrgUnitImportRow{
			row(2, "CON", "CHA", "", ""),
			row(3, "CHA", "", "", ""),
		}, 2, OrgUnitImportColParent, "PHÍA TRÊN"},
		// A row naming itself as its parent is the one-node cycle; "above" makes it impossible.
		{"tự làm cha của mình", []OrgUnitImportRow{row(2, "VÒNG", "vong", "", "")}, 2, OrgUnitImportColParent, "PHÍA TRÊN"},
		{"trùng mã gõ tay trong tệp", []OrgUnitImportRow{
			row(2, "MỘT", "", "trung", ""),
			row(3, "HAI", "", "trung", ""),
		}, 3, OrgUnitImportColCode, "dòng 2"},
		{"mã tự sinh trùng nhau trong tệp", []OrgUnitImportRow{
			row(2, "TỔ MỚI", "LÃNH ĐẠO", "", ""),
			row(3, "Tổ Mới", "VĂN PHÒNG", "", ""),
		}, 3, OrgUnitImportColCode, "dòng 2"},
		{"trùng tên cùng cha trong tệp", []OrgUnitImportRow{
			row(2, "TỔ A", "LÃNH ĐẠO", "to-a", ""),
			row(3, "tổ a", "lanh-dao", "to-a-khac", ""),
		}, 3, OrgUnitImportColName, "dòng 2"},
		{"trùng tên với bộ phận đang có cùng cha", []OrgUnitImportRow{row(2, "văn phòng", "LÃNH ĐẠO", "vp-moi", "")}, 2, OrgUnitImportColName, "đã có"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, errs := PlanOrgUnitImport(c.rows, chartForImport(), 500)
			if plan != nil {
				t.Errorf("có lỗi mà vẫn trả kế hoạch — người gọi có thể ghi nửa tệp")
			}
			if !errorAt(errs, c.r, c.col, c.frag) {
				t.Errorf("không thấy lỗi (dòng %d, cột %q, %q); nhận %+v", c.r, c.col, c.frag, errs)
			}
		})
	}
}

// Same name under DIFFERENT parents is fine when the codes differ.
func TestPlanOrgUnitImport_SameNameDifferentParentsIsAllowed(t *testing.T) {
	_, errs := PlanOrgUnitImport([]OrgUnitImportRow{
		row(2, "TỔ MỚI", "LÃNH ĐẠO", "to-moi-a", ""),
		row(3, "TỔ MỚI", "VĂN PHÒNG", "to-moi-b", ""),
	}, chartForImport(), 500)
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
}

// EVERY error of the file is reported in one pass, ordered by row.
func TestPlanOrgUnitImport_ReportsEveryErrorInOrder(t *testing.T) {
	_, errs := PlanOrgUnitImport([]OrgUnitImportRow{
		row(2, "HỢP LỆ", "", "", ""),
		row(3, "", "", "", "1.5"),
		row(4, "MỚI", "không-có", "Sai", ""),
	}, chartForImport(), 500)
	if len(errs) != 4 {
		t.Fatalf("muốn 4 lỗi (tên+thứ tự dòng 3, mã+cha dòng 4), nhận %d: %+v", len(errs), errs)
	}
	for i := 1; i < len(errs); i++ {
		if errs[i].Row < errs[i-1].Row {
			t.Fatalf("lỗi không theo thứ tự dòng: %+v", errs)
		}
	}
}

func TestPlanOrgUnitImport_FileLevelLimits(t *testing.T) {
	if _, errs := PlanOrgUnitImport(nil, nil, 500); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("tệp rỗng: %+v", errs)
	}
	many := make([]OrgUnitImportRow, MaxOrgUnitImportRows+1)
	for i := range many {
		many[i] = row(i+2, "X", "", "", "")
	}
	if _, errs := PlanOrgUnitImport(many, nil, 10000); len(errs) != 1 || errs[0].Row != 0 || !strings.Contains(errs[0].Message, "tối đa") {
		t.Errorf("quá %d dòng: %+v", MaxOrgUnitImportRows, errs)
	}
	// The ceiling counts LIVE units only: 5 existing, 1 deleted — 4 live + 2 new > 5.
	_, errs := PlanOrgUnitImport([]OrgUnitImportRow{row(2, "MỘT", "", "", ""), row(3, "HAI", "", "", "")}, chartForImport(), 5)
	if !errorAt(errs, 0, "", "vượt trần") {
		t.Errorf("vượt trần sơ đồ: %+v", errs)
	}
	if _, errs := PlanOrgUnitImport([]OrgUnitImportRow{row(2, "MỘT", "", "", "")}, chartForImport(), 5); len(errs) != 0 {
		t.Errorf("4 đang có + 1 = 5, đúng trần — không được từ chối: %+v", errs)
	}
}
