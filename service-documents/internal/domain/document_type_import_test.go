package domain

import (
	"fmt"
	"strings"
	"testing"
)

// What these tests defend: the planner is all-or-nothing, an issued code is taken forever (soft-deleted
// rows included) while a deleted label is free again, the ceiling is the catalogue's own, generated
// codes pass the form's rule, and the header must be the template's.

func TestSlugDocumentTypeCode(t *testing.T) {
	cases := map[string]string{
		"Quyết định":           "quyet-dinh",
		"CÔNG VĂN":             "cong-van",
		"  Tờ trình / Đề án  ": "to-trinh-de-an",
		"!!!":                  "",
	}
	for in, want := range cases {
		if got := SlugDocumentTypeCode(in); got != want {
			t.Errorf("%q → %q, muốn %q", in, got, want)
		}
	}
	long := SlugDocumentTypeCode(strings.Repeat("dài ", 40))
	if len(long) > MaToiDa || strings.HasSuffix(long, "-") {
		t.Errorf("cắt sai: %q", long)
	}
	if _, err := ChuanHoaMa(long); err != nil {
		t.Errorf("mã tự sinh không qua được luật của biểu mẫu: %v", err)
	}
}

func TestReadDocumentTypeSheet(t *testing.T) {
	head := DocumentTypeImportColumns()
	rows, errs := ReadDocumentTypeSheet([][]string{head, {"A", "", "1"}, {}, {" ", ""}, {"B"}})
	if len(errs) != 0 || len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 || rows[1].Order != "" {
		t.Fatalf("rows=%+v errs=%+v", rows, errs)
	}
	if _, errs := ReadDocumentTypeSheet([][]string{{"Nhóm", "Tên hiển thị", "Diễn giải", "Thứ tự"}}); len(errs) != 1 || errs[0].Row != 0 {
		t.Fatalf("tiêu đề của kho yêu cầu (có Nhóm) phải bị từ chối cả tệp: %+v", errs)
	}
	if _, errs := ReadDocumentTypeSheet([][]string{head, {"A", "", "", "thừa"}}); len(errs) != 1 || errs[0].Row != 2 {
		t.Fatalf("dữ liệu ngoài ba cột phải là lỗi dòng: %+v", errs)
	}
	if _, errs := ReadDocumentTypeSheet(nil); len(errs) != 1 {
		t.Fatal("tệp không dòng nào phải bị từ chối")
	}
}

func TestPlanDocumentTypeImport_CeilingAndBounds(t *testing.T) {
	if _, errs := PlanDocumentTypeImport(nil, nil, 200); len(errs) != 1 {
		t.Fatal("tệp rỗng phải bị từ chối")
	}
	existing := make([]ExistingDocumentType, 199)
	for i := range existing {
		existing[i] = ExistingDocumentType{Code: fmt.Sprintf("loai-%d", i), Label: fmt.Sprintf("Loại %d", i)}
	}
	rows := []DocumentTypeImportRow{{Row: 2, Label: "Một"}, {Row: 3, Label: "Hai"}}
	plan, errs := PlanDocumentTypeImport(rows, existing, 200)
	if plan != nil || len(errs) == 0 || errs[0].Row != 0 {
		t.Fatalf("vượt trần mà không bị từ chối cả tệp: %+v", errs)
	}
	many := make([]DocumentTypeImportRow, MaxDocumentTypeImportRows+1)
	for i := range many {
		many[i] = DocumentTypeImportRow{Row: i + 2, Label: fmt.Sprintf("Loại %d", i)}
	}
	if plan, errs := PlanDocumentTypeImport(many, nil, 1000); plan != nil || len(errs) != 1 {
		t.Fatalf("tệp quá %d dòng phải bị từ chối cả tệp: %+v", MaxDocumentTypeImportRows, errs)
	}
}

func TestPlanDocumentTypeImport_DeletedLabelIsFreeButCodeIsNot(t *testing.T) {
	existing := []ExistingDocumentType{{Code: "cong-van", Label: "Công văn", Deleted: true}}
	plan, errs := PlanDocumentTypeImport([]DocumentTypeImportRow{{Row: 2, Label: "Công văn", Code: "cong-van-moi"}}, existing, 200)
	if len(errs) != 0 || len(plan) != 1 || plan[0].Code != "cong-van-moi" {
		t.Fatalf("tên của loại đã xoá phải dùng lại được: %+v %+v", plan, errs)
	}
	_, errs = PlanDocumentTypeImport([]DocumentTypeImportRow{{Row: 2, Label: "Công văn"}}, existing, 200)
	if len(errs) != 1 || errs[0].Column != DocumentTypeImportColCode {
		t.Fatalf("mã tự sinh trùng mã đã cấp phải là lỗi cột Mã: %+v", errs)
	}
}

// One bad row anywhere means NO plan at all — the caller cannot write half a file.
func TestPlanDocumentTypeImport_AllOrNothing(t *testing.T) {
	rows := []DocumentTypeImportRow{
		{Row: 2, Label: "Quyết định", Order: "1"},
		{Row: 3, Label: "Tờ trình", Code: "To-Trinh"}, // upper case: refused, never lower-cased
	}
	plan, errs := PlanDocumentTypeImport(rows, nil, 200)
	if plan != nil || len(errs) != 1 || errs[0].Row != 3 || errs[0].Column != DocumentTypeImportColCode {
		t.Fatalf("plan=%+v errs=%+v", plan, errs)
	}
	plan, errs = PlanDocumentTypeImport(rows[:1], nil, 200)
	if len(errs) != 0 || len(plan) != 1 || plan[0].Code != "quyet-dinh" || plan[0].Order != 1 {
		t.Fatalf("plan=%+v errs=%+v", plan, errs)
	}
}

func TestParseImportOrder(t *testing.T) {
	for in, want := range map[string]int{"": 0, " 7 ": 7, "9999": 9999} {
		if n, msg := parseImportOrder(in); msg != "" || n != want {
			t.Errorf("%q → %d %q", in, n, msg)
		}
	}
	for _, in := range []string{"1.5", "-1", "10000", "1e3", "٣", "abc"} {
		if _, msg := parseImportOrder(in); msg == "" {
			t.Errorf("%q được nhận", in)
		}
	}
}
