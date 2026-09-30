package domain

import (
	"strings"
	"testing"
)

func TestSlugMapAssetTypeCode(t *testing.T) {
	cases := map[string]string{
		"Hộ kinh doanh cá thể": "ho-kinh-doanh-ca-the",
		"ĐIỂM DU LỊCH":         "diem-du-lich",
		"  Chợ / Siêu thị  ":   "cho-sieu-thi",
		"!!!":                  "",
	}
	for in, want := range cases {
		if got := SlugMapAssetTypeCode(in); got != want {
			t.Errorf("%q → %q, muốn %q", in, got, want)
		}
	}
	long := SlugMapAssetTypeCode(strings.Repeat("dài ", 40))
	if len(long) > MaToiDa || strings.HasSuffix(long, "-") {
		t.Errorf("cắt sai: %q", long)
	}
	if _, err := ChuanHoaMa(long); err != nil {
		t.Errorf("mã tự sinh không qua được luật của biểu mẫu: %v", err)
	}
}

func TestReadMapAssetTypeSheet(t *testing.T) {
	head := MapAssetTypeImportColumns()
	rows, errs := ReadMapAssetTypeSheet([][]string{head, {"A", "", "1"}, {}, {" ", ""}, {"B"}})
	if len(errs) != 0 || len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 || rows[1].Order != "" {
		t.Fatalf("rows=%+v errs=%+v", rows, errs)
	}
	if _, errs := ReadMapAssetTypeSheet([][]string{{"Nhóm", "Tên hiển thị", "Diễn giải", "Thứ tự"}}); len(errs) != 1 || errs[0].Row != 0 {
		t.Fatalf("tiêu đề của kho yêu cầu (có Nhóm) phải bị từ chối cả tệp: %+v", errs)
	}
	if _, errs := ReadMapAssetTypeSheet([][]string{head, {"A", "", "", "thừa"}}); len(errs) != 1 || errs[0].Row != 2 {
		t.Fatalf("dữ liệu ngoài ba cột phải là lỗi dòng: %+v", errs)
	}
}

func TestPlanMapAssetTypeImport_CeilingAndEmpty(t *testing.T) {
	if _, errs := PlanMapAssetTypeImport(nil, nil, 500); len(errs) != 1 {
		t.Fatal("tệp rỗng phải bị từ chối")
	}
	existing := make([]ExistingMapAssetType, 499)
	for i := range existing {
		existing[i] = ExistingMapAssetType{Code: "c" + strings.Repeat("x", i%5) + string(rune('a'+i%26)), Label: "L" + string(rune(i))}
	}
	rows := []MapAssetTypeImportRow{{Row: 2, Label: "Một"}, {Row: 3, Label: "Hai"}}
	plan, errs := PlanMapAssetTypeImport(rows, existing, 500)
	if plan != nil || len(errs) == 0 || errs[0].Row != 0 {
		t.Fatalf("vượt trần mà không bị từ chối cả tệp: %+v", errs)
	}
}

func TestPlanMapAssetTypeImport_DeletedLabelIsFreeButCodeIsNot(t *testing.T) {
	existing := []ExistingMapAssetType{{Code: "cho", Label: "Chợ", Deleted: true}}
	plan, errs := PlanMapAssetTypeImport([]MapAssetTypeImportRow{{Row: 2, Label: "Chợ", Code: "cho-moi"}}, existing, 500)
	if len(errs) != 0 || len(plan) != 1 || plan[0].Code != "cho-moi" {
		t.Fatalf("tên của loại đã xoá phải dùng lại được: %+v %+v", plan, errs)
	}
	_, errs = PlanMapAssetTypeImport([]MapAssetTypeImportRow{{Row: 2, Label: "Chợ"}}, existing, 500)
	if len(errs) != 1 || errs[0].Column != MapAssetTypeImportColCode {
		t.Fatalf("mã tự sinh trùng mã đã cấp phải là lỗi cột Mã: %+v", errs)
	}
}

func TestParseImportRank(t *testing.T) {
	for in, want := range map[string]int{"": 0, " 7 ": 7, "9999": 9999} {
		if n, msg := parseImportRank(in); msg != "" || n != want {
			t.Errorf("%q → %d %q", in, n, msg)
		}
	}
	for _, in := range []string{"1.5", "-1", "10000", "1e3", "٣", "abc"} {
		if _, msg := parseImportRank(in); msg == "" {
			t.Errorf("%q được nhận", in)
		}
	}
}
