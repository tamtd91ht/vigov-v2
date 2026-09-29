package domain

import (
	"strings"
	"testing"
)

func categoryRows(rows ...[3]string) []CatalogueImportRow {
	out := make([]CatalogueImportRow, 0, len(rows))
	for i, r := range rows {
		out = append(out, CatalogueImportRow{Row: i + 2, Label: r[0], Code: r[1], Order: r[2]})
	}
	return out
}

func TestPlanCatalogueImport_DerivesCodesAndReadsOrder(t *testing.T) {
	plan, errs := PlanCatalogueImport(categoryRows(
		[3]string{"Xây dựng mới", "", "3"},
		[3]string{"Sửa chữa", "sua-chua", ""},
		[3]string{"  Đền bù GPMB  ", "", "12"},
	), nil, 200)
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	want := []PlannedCatalogueEntry{
		{Row: 2, Code: "xay-dung-moi", Label: "Xây dựng mới", Order: 3},
		{Row: 3, Code: "sua-chua", Label: "Sửa chữa", Order: 0},
		{Row: 4, Code: "den-bu-gpmb", Label: "Đền bù GPMB", Order: 12},
	}
	for i := range want {
		if plan[i] != want[i] {
			t.Errorf("dòng %d = %+v, muốn %+v", i, plan[i], want[i])
		}
	}
}

func TestPlanCatalogueImport_Duplicates(t *testing.T) {
	existing := []ExistingCatalogueEntry{
		{Code: "xay-dung-moi", Label: "Xây dựng mới (cũ)", Deleted: true}, // code still taken
		{Code: "sua-chua", Label: "Sửa chữa"},                             // live label
	}
	_, errs := PlanCatalogueImport(categoryRows(
		[3]string{"Xây dựng mới", "", ""},         // 2: derived code issued to a deleted row
		[3]string{"SỬA CHỮA", "sua-chua-2", ""},   // 3: live label, case-folded
		[3]string{"Mới", "moi", ""},               // 4
		[3]string{"Mới khác", "moi", ""},          // 5: typed code repeated in file
		[3]string{"mới", "moi-3", ""},             // 6: label repeated in file
		[3]string{"Sửa chữa lớn", "sua-chua", ""}, // 7: typed code taken
	), existing, 200)
	byRow := map[int]string{}
	for _, e := range errs {
		byRow[e.Row] += e.Column + ";"
	}
	for row, col := range map[int]string{2: CatalogueImportColCode, 3: CatalogueImportColLabel, 5: CatalogueImportColCode,
		6: CatalogueImportColLabel, 7: CatalogueImportColCode} {
		if !strings.Contains(byRow[row], col) {
			t.Errorf("dòng %d: muốn lỗi cột %q, nhận %q", row, col, byRow[row])
		}
	}
	if byRow[4] != "" {
		t.Errorf("dòng 4 hợp lệ mà bị báo lỗi: %q", byRow[4])
	}
}

func TestPlanCatalogueImport_AnyErrorMeansNoPlan(t *testing.T) {
	plan, errs := PlanCatalogueImport(categoryRows([3]string{"Tốt", "", ""}, [3]string{"", "", "1.5"}), nil, 200)
	if plan != nil || len(errs) != 2 {
		t.Errorf("tệp có lỗi thì không có kế hoạch: %+v %+v", plan, errs)
	}
}

func TestPlanCatalogueImport_Ceiling(t *testing.T) {
	existing := make([]ExistingCatalogueEntry, 199)
	for i := range existing {
		existing[i] = ExistingCatalogueEntry{Code: "m" + strings.Repeat("x", i+1), Label: "Mục " + strings.Repeat("x", i+1)}
	}
	_, errs := PlanCatalogueImport([]CatalogueImportRow{{Row: 2, Label: "A"}, {Row: 3, Label: "B"}}, existing, 200)
	if len(errs) != 1 || errs[0].Row != 0 || !strings.Contains(errs[0].Message, "vượt trần 200") {
		t.Errorf("vượt trần: %+v", errs)
	}
}

// NO MESSAGE ECHOES A CELL — not a label, not a code, not an order.
func TestPlanCatalogueImport_MessagesNeverEchoCells(t *testing.T) {
	const secret = "0900000000"
	long := strings.Repeat("Nguyễn Văn A "+secret, 20)
	_, errs := PlanCatalogueImport(categoryRows(
		[3]string{long, "", ""},
		[3]string{"Tên " + secret, "Ma " + secret, secret + ".5"},
		[3]string{"Tên " + secret, "", ""},
	), []ExistingCatalogueEntry{{Code: "x", Label: "Tên " + secret}}, 200)
	if len(errs) == 0 {
		t.Fatal("muốn có lỗi")
	}
	for _, e := range errs {
		if strings.Contains(e.Message, secret) || strings.Contains(e.Message, "Nguyễn") {
			t.Errorf("thông báo nhắc lại chữ trong ô: %q", e.Message)
		}
	}
}

func TestCatalogueRowsFromSheet(t *testing.T) {
	rows, errs := CatalogueRowsFromSheet([][]string{
		CatalogueImportColumns(),
		{"Xây dựng mới", "", "1"},
		{},
		{"Sửa chữa", "sua-chua", "", "lạc cột"},
		{"Mua sắm"},
	})
	if len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 {
		t.Errorf("dòng = %+v", rows)
	}
	if len(errs) != 1 || errs[0].Row != 4 || strings.Contains(errs[0].Message, "lạc cột") {
		t.Errorf("lỗi = %+v", errs)
	}
	_, errs = CatalogueRowsFromSheet([][]string{{"Nhóm", "Tên hiển thị", "Thứ tự"}})
	if len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề sai: %+v", errs)
	}
	_, errs = CatalogueRowsFromSheet(nil)
	if len(errs) != 1 {
		t.Errorf("trang trống: %+v", errs)
	}
}

func TestDeriveCatalogueCode(t *testing.T) {
	for in, want := range map[string]string{
		"Xây dựng mới":           "xay-dung-moi",
		"Đền bù – Giải phóng MB": "den-bu-giai-phong-mb",
	} {
		if got, ok := DeriveCatalogueCode(in); !ok || got != want {
			t.Errorf("%q → %q %v, muốn %q", in, got, ok, want)
		}
	}
	if _, ok := DeriveCatalogueCode("!!!"); ok {
		t.Error("không còn ký tự dùng được thì phải trả false")
	}
}
