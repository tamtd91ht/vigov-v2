package domain

import (
	"strings"
	"testing"
)

func typeRows(rows ...[3]string) []CatalogueImportRow {
	out := make([]CatalogueImportRow, 0, len(rows))
	for i, r := range rows {
		out = append(out, CatalogueImportRow{Row: i + 2, Label: r[0], Code: r[1], Order: r[2]})
	}
	return out
}

func TestPlanCatalogueImport_TypesDeriveCodesAndReadOrder(t *testing.T) {
	plan, errs := PlanCatalogueImport(typeRows(
		[3]string{"Theo văn bản", "", "3"},
		[3]string{"Cơ bản", "co-ban", ""},
		[3]string{"  Đột xuất  ", "", "12"},
	), nil, 100, CatalogueOrderFromColumn)
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	want := []PlannedCatalogueEntry{
		{Row: 2, Code: "theo-van-ban", Label: "Theo văn bản", Order: 3},
		{Row: 3, Code: "co-ban", Label: "Cơ bản", Order: 0},
		{Row: 4, Code: "dot-xuat", Label: "Đột xuất", Order: 12},
	}
	for i := range want {
		if plan[i] != want[i] {
			t.Errorf("dòng %d = %+v, muốn %+v", i, plan[i], want[i])
		}
	}
}

// THE SCALE: new levels go AFTER the last live level (out-of-use rows count, deleted rows do not), in
// file order; an empty scale starts at 0.
func TestPlanCatalogueImport_PrioritiesAppendAfterExistingInFileOrder(t *testing.T) {
	rows := []CatalogueImportRow{{Row: 2, Label: "Rất khẩn"}, {Row: 3, Label: "Theo dõi", Code: "theo-doi"}}
	existing := []ExistingCatalogueEntry{
		{Code: "khan", Label: "Khẩn", Order: 0},
		{Code: "thuong", Label: "Thường", Order: 7}, // live; out of use or not, it holds its place
		{Code: "cu", Label: "Cũ", Order: 900, Deleted: true},
	}
	plan, errs := PlanCatalogueImport(rows, existing, 50, CatalogueOrderFromPosition)
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	if plan[0].Order != 8 || plan[1].Order != 9 || plan[0].Code != "rat-khan" {
		t.Errorf("thứ hạng = %+v, muốn 8 và 9 sau mức cuối cùng đang sống (7)", plan)
	}

	plan, errs = PlanCatalogueImport(rows, nil, 50, CatalogueOrderFromPosition)
	if len(errs) != 0 || plan[0].Order != 0 || plan[1].Order != 1 {
		t.Errorf("thang trống phải bắt đầu từ 0: %+v %+v", plan, errs)
	}

	_, errs = PlanCatalogueImport(rows, []ExistingCatalogueEntry{{Code: "x", Label: "X", Order: ThuTuToiDa - 1}}, 50, CatalogueOrderFromPosition)
	if len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("vượt Thứ tự tối đa phải là lỗi của cả tệp, không kẹp: %+v", errs)
	}
}

func TestPlanCatalogueImport_Duplicates(t *testing.T) {
	existing := []ExistingCatalogueEntry{
		{Code: "theo-van-ban", Label: "Theo văn bản cũ", Deleted: true}, // code still taken
		{Code: "co-ban", Label: "Cơ bản"},                               // live label
	}
	_, errs := PlanCatalogueImport(typeRows(
		[3]string{"Theo văn bản", "", ""},    // 2: derived code issued to a deleted row
		[3]string{"CƠ BẢN", "co-ban-2", ""},  // 3: live label, case-folded
		[3]string{"Mới", "moi", ""},          // 4
		[3]string{"Mới khác", "moi", ""},     // 5: typed code repeated in file
		[3]string{"mới", "moi-3", ""},        // 6: label repeated in file
		[3]string{"Cơ bản cũ", "co-ban", ""}, // 7: typed code taken
	), existing, 100, CatalogueOrderFromColumn)
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
	plan, errs := PlanCatalogueImport(typeRows([3]string{"Tốt", "", ""}, [3]string{"", "", "1.5"}), nil, 100, CatalogueOrderFromColumn)
	if plan != nil || len(errs) != 2 {
		t.Errorf("tệp có lỗi thì không có kế hoạch: %+v %+v", plan, errs)
	}
}

func TestPlanCatalogueImport_Ceiling(t *testing.T) {
	existing := make([]ExistingCatalogueEntry, 49)
	for i := range existing {
		existing[i] = ExistingCatalogueEntry{Code: "m" + strings.Repeat("x", i+1), Label: "Mức " + strings.Repeat("x", i+1)}
	}
	_, errs := PlanCatalogueImport([]CatalogueImportRow{{Row: 2, Label: "A"}, {Row: 3, Label: "B"}}, existing, 50, CatalogueOrderFromPosition)
	if len(errs) != 1 || errs[0].Row != 0 || !strings.Contains(errs[0].Message, "vượt trần 50") {
		t.Errorf("vượt trần: %+v", errs)
	}
}

// NO MESSAGE ECHOES A CELL — not a label, not a code, not an order.
func TestPlanCatalogueImport_MessagesNeverEchoCells(t *testing.T) {
	const secret = "0900000000"
	long := strings.Repeat("Nguyễn Văn A "+secret, 20)
	_, errs := PlanCatalogueImport(typeRows(
		[3]string{long, "", ""},
		[3]string{"Tên " + secret, "Ma " + secret, secret + ".5"},
		[3]string{"Tên " + secret, "", ""},
	), []ExistingCatalogueEntry{{Code: "x", Label: "Tên " + secret}}, 100, CatalogueOrderFromColumn)
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
		{"Tên hiển thị", "Mã (để trống thì tự sinh)"},
		{"Khẩn", ""},
		{},
		{"Cao", "cao", "lạc cột"},
		{"Thường"},
	}, CatalogueOrderFromPosition)
	if len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 || rows[1].Order != "" {
		t.Errorf("dòng = %+v", rows)
	}
	if len(errs) != 1 || errs[0].Row != 4 || strings.Contains(errs[0].Message, "lạc cột") {
		t.Errorf("lỗi = %+v", errs)
	}

	// The type template's header is not the scale's: a `Thứ tự` column is refused there.
	_, errs = CatalogueRowsFromSheet([][]string{CatalogueOrderFromColumn.Columns(), {"Khẩn", "", "1"}}, CatalogueOrderFromPosition)
	if len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề có Thứ tự trên thang ưu tiên phải bị từ chối: %+v", errs)
	}
	_, errs = CatalogueRowsFromSheet(nil, CatalogueOrderFromColumn)
	if len(errs) != 1 {
		t.Errorf("trang trống: %+v", errs)
	}
}

func TestDeriveCatalogueCode(t *testing.T) {
	for in, want := range map[string]string{
		"Theo văn bản":          "theo-van-ban",
		"Đảng ủy – Chính quyền": "dang-uy-chinh-quyen",
		"  Khẩn!! ":             "khan",
	} {
		if got, ok := DeriveCatalogueCode(in); !ok || got != want {
			t.Errorf("%q → %q %v, muốn %q", in, got, ok, want)
		}
	}
	if _, ok := DeriveCatalogueCode("!!!"); ok {
		t.Error("không còn ký tự dùng được thì phải trả false")
	}
	if got, ok := DeriveCatalogueCode(strings.Repeat("dai ", 40)); !ok || len(got) > MaToiDa || strings.HasSuffix(got, "-") {
		t.Errorf("cắt theo ranh giới từ: %q", got)
	}
}
