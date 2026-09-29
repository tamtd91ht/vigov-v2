package domain

import (
	"strings"
	"testing"
)

func catalogueSheet(rows ...[]string) [][]string {
	return append([][]string{CatalogueImportColumns()}, rows...)
}

func TestCatalogueRowsFromSheet_HeaderMustMatch(t *testing.T) {
	if _, errs := CatalogueRowsFromSheet([][]string{{"Nhóm", "Tên hiển thị", "Diễn giải"}}); len(errs) != 1 || errs[0].Row != 1 {
		t.Fatalf("tiêu đề của kho yêu cầu (có Nhóm) phải bị từ chối: %+v", errs)
	}
	rows, errs := CatalogueRowsFromSheet(catalogueSheet(
		[]string{"Khu phố", "", "3"},
		[]string{"", "", ""},
		[]string{"Ấp", "ap", "", "thừa"},
	))
	if len(rows) != 1 || rows[0].Row != 2 || rows[0].Label != "Khu phố" || rows[0].Order != "3" {
		t.Errorf("dòng: %+v", rows)
	}
	if len(errs) != 1 || errs[0].Row != 4 || strings.Contains(errs[0].Message, "thừa") {
		t.Errorf("ô ngoài 3 cột phải là lỗi dòng 4, không nhắc chữ trong ô: %+v", errs)
	}
}

func TestPlanCatalogueImport_Plans(t *testing.T) {
	rows := []CatalogueImportRow{
		{Row: 2, Label: "Khu phố", Order: "3"},
		{Row: 3, Label: "Ấp", Code: "ap-moi"},
	}
	plan, errs := PlanCatalogueImport(rows, []ExistingCatalogueEntry{{Code: "thon", Label: "Thôn"}}, 100)
	if len(errs) != 0 || len(plan) != 2 {
		t.Fatalf("plan %+v errs %+v", plan, errs)
	}
	if plan[0].Code != "khu-pho" || plan[0].Order != 3 || plan[1].Code != "ap-moi" || plan[1].Label != "Ấp" {
		t.Errorf("plan: %+v", plan)
	}
}

func TestPlanCatalogueImport_EveryErrorAndNoPlan(t *testing.T) {
	existing := []ExistingCatalogueEntry{
		{Code: "thon", Label: "Thôn"},
		{Code: "khu-pho", Label: "Khu phố cũ", Deleted: true}, // code still taken, label freed
		{Code: "ap", Label: "Ấp đã xoá", Deleted: true},
	}
	rows := []CatalogueImportRow{
		{Row: 2, Label: "THÔN"},                  // live label, case-folded
		{Row: 3, Label: "Khu phố"},               // derived code of a deleted row
		{Row: 4, Label: "Mới", Code: "ap"},       // typed code of a deleted row
		{Row: 5, Label: "Tổ", Code: "To"},        // not lower-case
		{Row: 6, Label: "Tổ A", Code: "to-a"},    // ok
		{Row: 7, Label: "tổ a", Code: "to-b"},    // label repeats row 6
		{Row: 8, Label: "Tổ C", Code: "to-a"},    // code repeats row 6
		{Row: 9, Label: "Tổ D", Order: "1.5"},    // decimal order
		{Row: 10, Label: "", Code: "x"},          // no label
		{Row: 11, Label: "Tổ E", Order: "10000"}, // out of range
	}
	plan, errs := PlanCatalogueImport(rows, existing, 100)
	if plan != nil {
		t.Fatalf("có lỗi mà vẫn có kế hoạch: %+v", plan)
	}
	got := map[int]string{}
	for _, e := range errs {
		got[e.Row] += e.Column + ";"
	}
	want := map[int]string{
		2: CatalogueImportColLabel, 3: CatalogueImportColCode, 4: CatalogueImportColCode, 5: CatalogueImportColCode,
		7: CatalogueImportColLabel, 8: CatalogueImportColCode, 9: CatalogueImportColOrder,
		10: CatalogueImportColLabel, 11: CatalogueImportColOrder,
	}
	for row, col := range want {
		if !strings.Contains(got[row], col) {
			t.Errorf("dòng %d: lỗi %q, muốn cột %q", row, got[row], col)
		}
	}
	if _, bad := got[6]; bad {
		t.Errorf("dòng 6 hợp lệ mà có lỗi: %q", got[6])
	}
	for i := 1; i < len(errs); i++ {
		if errs[i-1].Row > errs[i].Row {
			t.Fatal("lỗi phải xếp theo dòng")
		}
	}
}

func TestPlanCatalogueImport_Ceiling(t *testing.T) {
	existing := make([]ExistingCatalogueEntry, 0, 99)
	for i := 0; i < 99; i++ {
		existing = append(existing, ExistingCatalogueEntry{Code: "m-" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Label: "L" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	_, errs := PlanCatalogueImport([]CatalogueImportRow{{Row: 2, Label: "Một"}, {Row: 3, Label: "Hai"}}, existing, 100)
	if len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("vượt trần phải là lỗi cả tệp: %+v", errs)
	}
	if _, errs := PlanCatalogueImport(nil, nil, 100); len(errs) != 1 {
		t.Errorf("tệp rỗng: %+v", errs)
	}
}

func TestDeriveCatalogueCode(t *testing.T) {
	for in, want := range map[string]string{"Khu phố": "khu-pho", "Khối Đảng": "khoi-dang", "  Tổ dân phố 3 ": "to-dan-pho-3"} {
		if got, ok := DeriveCatalogueCode(in); !ok || got != want {
			t.Errorf("%q → %q, muốn %q", in, got, want)
		}
	}
	if _, ok := DeriveCatalogueCode("!!!"); ok {
		t.Error("nhãn không có chữ phải không sinh được mã")
	}
	long, ok := DeriveCatalogueCode(strings.Repeat("abcdefghi ", 10))
	if !ok || len(long) > MaDanhMucToiDa || strings.HasSuffix(long, "-") {
		t.Errorf("mã dài: %q", long)
	}
}
