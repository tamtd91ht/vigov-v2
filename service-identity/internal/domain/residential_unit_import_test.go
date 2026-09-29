package domain

import (
	"strings"
	"testing"
)

func ruSnapshot() ResidentialUnitImportSnapshot {
	return ResidentialUnitImportSnapshot{
		Units: []ExistingResidentialUnit{
			{ID: "tt-1", Code: "thon-binh-an", Name: "Thôn Bình An"},
			{ID: "tt-2", Code: "thon-cu", Name: "Thôn Cũ"}, // out of use: still live
			{ID: "tt-x", Code: "da-xoa", Name: "Đã Xoá", Deleted: true},
		},
		Types: []ResidentialUnitTypeChoice{{Code: "thon", Label: "Thôn"}, {Code: "to-dan-pho", Label: "Tổ dân phố"}},
		Staff: []HeadStaffChoice{{ID: "nd-1", Code: "CB-2026-AAAAAA", Name: "Cán Bộ Một"}},
	}
}

func ruHeader() []string { return ResidentialUnitImportColumns() }

func TestResidentialUnitRowsFromSheet(t *testing.T) {
	rows, errs := ResidentialUnitRowsFromSheet([][]string{
		ruHeader(),
		{"Thôn A", "Thôn"},
		{},              // blank: skipped, numbering kept
		{" ", "", "  "}, // blank too
		{"Thôn B", "", "", "12", "40", "", "3"},
	})
	if len(errs) != 0 || len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 || rows[1].Households != "12" || rows[1].Order != "3" {
		t.Fatalf("rows %+v errs %+v", rows, errs)
	}

	_, errs = ResidentialUnitRowsFromSheet([][]string{{"Họ và tên", "Số điện thoại"}, {"x"}})
	if len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề sai phải là lỗi dòng 1: %+v", errs)
	}
	withExtra := append(ruHeader(), "Ghi chú")
	if _, errs = ResidentialUnitRowsFromSheet([][]string{withExtra}); len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề thêm cột phải bị từ chối: %+v", errs)
	}
	_, errs = ResidentialUnitRowsFromSheet([][]string{ruHeader(), {"Thôn A", "", "", "", "", "", "", "lạ"}})
	if len(errs) != 1 || errs[0].Row != 2 {
		t.Errorf("dữ liệu ngoài 7 cột phải là lỗi dòng 2: %+v", errs)
	}
}

func TestPlanResidentialUnitImport_Valid(t *testing.T) {
	plan, errs := PlanResidentialUnitImport([]ResidentialUnitImportRow{
		{Row: 2, Name: "Thôn Hoà Bình", Type: "thôn", Head: "CB-2026-AAAAAA · Cán Bộ Một", Households: "0", Population: "1132", Order: "4"},
		{Row: 3, Name: "Tổ dân phố 1", Type: "to-dan-pho", Head: "cb-2026-aaaaaa", Code: "tdp-1"},
		{Row: 4, Name: "Thôn Mới"},
	}, ruSnapshot(), 500)
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	u := plan[0].Unit
	if u.Ma != "thon-hoa-binh" || u.LoaiMa != "thon" || u.HeadStaffID != "nd-1" || u.SoHo == nil || *u.SoHo != 0 ||
		*u.NhanKhau != 1132 || u.SortOrder != 4 || !u.DangDung {
		t.Errorf("dòng 2: %+v", u)
	}
	if plan[1].Unit.Ma != "tdp-1" || plan[1].Unit.HeadStaffCode != "CB-2026-AAAAAA" {
		t.Errorf("dòng 3: %+v", plan[1].Unit)
	}
	if plan[2].Unit.SoHo != nil || plan[2].Unit.NhanKhau != nil || plan[2].Unit.LoaiMa != "" || plan[2].Unit.HeadStaffID != "" {
		t.Errorf("ô trống phải là CHƯA NHẬP, không phải 0: %+v", plan[2].Unit)
	}
}

func TestPlanResidentialUnitImport_EveryErrorAndNoPlan(t *testing.T) {
	cases := []struct {
		name string
		row  ResidentialUnitImportRow
		col  string
	}{
		{"tên trống", ResidentialUnitImportRow{Name: " "}, ResidentialUnitImportColName},
		{"trùng tên đang dùng", ResidentialUnitImportRow{Name: "THÔN BÌNH AN", Code: "khac-1"}, ResidentialUnitImportColName},
		{"trùng tên đã ngưng dùng", ResidentialUnitImportRow{Name: "thôn cũ", Code: "khac-2"}, ResidentialUnitImportColName},
		{"mã gõ tay của dòng đã xoá mềm", ResidentialUnitImportRow{Name: "Thôn X", Code: "da-xoa"}, ResidentialUnitImportColCode},
		{"mã tự sinh đã cấp — không thêm hậu tố", ResidentialUnitImportRow{Name: "Thôn - Cũ"}, ResidentialUnitImportColCode},
		{"mã sai định dạng", ResidentialUnitImportRow{Name: "Thôn X", Code: "Thon X"}, ResidentialUnitImportColCode},
		{"loại không có", ResidentialUnitImportRow{Name: "Thôn X", Type: "Ấp"}, ResidentialUnitImportColType},
		{"cán bộ không có", ResidentialUnitImportRow{Name: "Thôn X", Head: "CB-2026-ZZZZZZ"}, ResidentialUnitImportColHead},
		{"số hộ có dấu chấm", ResidentialUnitImportRow{Name: "Thôn X", Households: "1.132"}, ResidentialUnitImportColHouseholds},
		{"nhân khẩu âm", ResidentialUnitImportRow{Name: "Thôn X", Population: "-5"}, ResidentialUnitImportColPopulation},
		{"nhân khẩu quá lớn", ResidentialUnitImportRow{Name: "Thôn X", Population: "10000001"}, ResidentialUnitImportColPopulation},
		{"thứ tự thập phân", ResidentialUnitImportRow{Name: "Thôn X", Order: "1.5"}, ResidentialUnitImportColOrder},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.row
			r.Row = 7
			plan, errs := PlanResidentialUnitImport([]ResidentialUnitImportRow{{Row: 2, Name: "Thôn Hợp Lệ"}, r}, ruSnapshot(), 500)
			if plan != nil {
				t.Fatal("có lỗi mà vẫn trả kế hoạch — tất cả hoặc không")
			}
			if len(errs) != 1 || errs[0].Row != 7 || errs[0].Column != c.col {
				t.Errorf("lỗi = %+v, muốn một lỗi dòng 7 cột %q", errs, c.col)
			}
		})
	}
}

func TestPlanResidentialUnitImport_DuplicatesWithinFile(t *testing.T) {
	_, errs := PlanResidentialUnitImport([]ResidentialUnitImportRow{
		{Row: 2, Name: "Thôn Mới"},
		{Row: 3, Name: "thôn mới", Code: "khac"},
		{Row: 4, Name: "Thôn Khác", Code: "thon-moi"},
	}, ruSnapshot(), 500)
	if len(errs) != 2 || errs[0].Row != 3 || errs[0].Column != ResidentialUnitImportColName ||
		errs[1].Row != 4 || errs[1].Column != ResidentialUnitImportColCode || !strings.Contains(errs[0].Message, "dòng 2") {
		t.Errorf("trùng trong tệp: %+v", errs)
	}
}

func TestPlanResidentialUnitImport_FileLimits(t *testing.T) {
	if _, errs := PlanResidentialUnitImport(nil, ruSnapshot(), 500); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("tệp rỗng: %+v", errs)
	}
	if _, errs := PlanResidentialUnitImport([]ResidentialUnitImportRow{{Row: 2, Name: "A"}, {Row: 3, Name: "B"}}, ruSnapshot(), 3); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("vượt trần danh sách (2 đang sống + 2 > 3): %+v", errs)
	}
	many := make([]ResidentialUnitImportRow, MaxResidentialUnitImportRows+1)
	if _, errs := PlanResidentialUnitImport(many, ruSnapshot(), 100000); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("quá số dòng: %+v", errs)
	}
}

// NO MESSAGE ECHOES A CELL (rule 3, forbidden #3): the head column carries staff names, and anything
// typed into any column may be personal data.
func TestPlanResidentialUnitImport_MessagesNeverEchoCells(t *testing.T) {
	const secret = "0900000000 Nguyễn Văn Bí Mật"
	_, errs := PlanResidentialUnitImport([]ResidentialUnitImportRow{{
		Row: 2, Name: strings.Repeat("x", MaxResidentialUnitName+1) + secret, Type: secret, Head: secret,
		Households: secret, Population: secret, Code: secret, Order: secret,
	}}, ruSnapshot(), 500)
	if len(errs) != 7 {
		t.Fatalf("muốn 7 lỗi (mỗi cột một), nhận %d: %+v", len(errs), errs)
	}
	for _, e := range errs {
		if strings.Contains(e.Message, "0900000000") || strings.Contains(e.Message, "Bí Mật") {
			t.Errorf("thông báo lặp lại nội dung ô: %q", e.Message)
		}
	}
	_, sheetErrs := ResidentialUnitRowsFromSheet([][]string{{secret}})
	for _, e := range sheetErrs {
		if strings.Contains(e.Message, "Bí Mật") {
			t.Errorf("lỗi tiêu đề lặp lại nội dung ô: %q", e.Message)
		}
	}
}
