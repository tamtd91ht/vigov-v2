package domain

import (
	"strings"
	"testing"
)

func contactSheet(rows ...[]string) [][]string {
	return append([][]string{CitizenContactImportColumns()}, rows...)
}

func TestCitizenContactImportColumns_AreTheCardsSeven(t *testing.T) {
	want := []string{"Họ và tên", "Số điện thoại", "Thôn, tổ dân phố", "Địa chỉ", "Nhóm", "Nhãn", "Ghi chú"}
	got := CitizenContactImportColumns()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("cột = %q, muốn %q", got, want)
	}
	for _, c := range got {
		if strings.Contains(strings.ToLower(c), "zalo") {
			t.Errorf("tệp mẫu không có cột Zalo (chương 16 §7): %q", c)
		}
	}
	if got[CitizenContactImportResidentialUnitColumn] != CitizenContactImportColResidentialUnit {
		t.Errorf("cột thả xuống trỏ sai: %d", CitizenContactImportResidentialUnitColumn)
	}
}

func TestCitizenContactImportTemplatePieces(t *testing.T) {
	ex := CitizenContactImportExampleRow()
	if len(ex) != len(CitizenContactImportColumns()) {
		t.Fatalf("dòng mẫu %d ô, tiêu đề %d cột", len(ex), len(CitizenContactImportColumns()))
	}
	if ex[1] != fakeMobile {
		t.Errorf("dòng mẫu phải dùng số giả %s: %q", fakeMobile, ex[1])
	}
	// The example row must pass the reader it is an example for.
	rows, errs := ReadCitizenContactImportSheet(contactSheet(ex))
	if len(errs) != 0 || len(rows) != 1 {
		t.Fatalf("dòng mẫu không đọc được: %+v", errs)
	}
	r := rows[0]
	if r.ResidentialUnit != "thon-1" || !r.ResidentialUnitIsCode || len(r.GroupNames) != 2 || len(r.Contact.Tags) != 2 {
		t.Errorf("dòng mẫu đọc sai: %+v", r)
	}
	// The guide carries the agreed tag sentence, once, verbatim.
	joined := strings.Join(CitizenContactImportGuide(), "\n")
	if strings.Count(joined, TagSensitiveWarning) != 1 {
		t.Errorf("hướng dẫn phải mang câu cấm nhãn đúng một lần: %q", joined)
	}
	// The dropdown value is code · name with core/xlsx's separator, and the reader cuts it to the code.
	if ResidentialUnitChoiceLabel("to-3", "Tổ 3") != "to-3 · Tổ 3" {
		t.Errorf("nhãn thả xuống: %q", ResidentialUnitChoiceLabel("to-3", "Tổ 3"))
	}
}

func TestReadCitizenContactImportSheet_HeaderMustMatch(t *testing.T) {
	if _, errs := ReadCitizenContactImportSheet(nil); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("tệp rỗng: %+v", errs)
	}
	// The chapter's first draft order (Nhãn before Nhóm) is NOT this template.
	swapped := []string{"Họ và tên", "Số điện thoại", "Thôn, tổ dân phố", "Địa chỉ", "Nhãn", "Nhóm", "Ghi chú"}
	if _, errs := ReadCitizenContactImportSheet([][]string{swapped, {"A", fakeMobile}}); len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề đảo cột phải là lỗi dòng 1: %+v", errs)
	}
	withZalo := append(CitizenContactImportColumns(), "Zalo")
	if _, errs := ReadCitizenContactImportSheet([][]string{withZalo, {"A", fakeMobile}}); len(errs) != 1 || errs[0].Row != 1 {
		t.Errorf("tiêu đề thêm cột phải bị từ chối: %+v", errs)
	}
	// Trailing blank header cells (Excel pads) are fine.
	padded := append(CitizenContactImportColumns(), "", " ")
	if rows, errs := ReadCitizenContactImportSheet([][]string{padded, {"A", fakeMobile}}); len(errs) != 0 || len(rows) != 1 {
		t.Errorf("tiêu đề có ô trống đệm phải được: %+v", errs)
	}
}

func TestReadCitizenContactImportSheet_RowsAndNumbering(t *testing.T) {
	rows, errs := ReadCitizenContactImportSheet(contactSheet(
		[]string{" Nguyễn  Văn A ", "+84 900 000 000", "Thôn 1", "", "Hộ kinh doanh, hộ kinh doanh , Tổ trưởng", " a , b ", " ghi chú "},
		[]string{},
		[]string{" ", "", "  "},
		[]string{"Trần Thị B", "0900 000 001"},
	))
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	if len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 5 {
		t.Fatalf("đánh số dòng phải là số dòng của bảng tính: %+v", rows)
	}
	a := rows[0]
	if a.Contact.FullName != "Nguyễn Văn A" || a.Contact.Phone != fakeMobile || !a.Contact.IsActive ||
		a.Contact.Note != "ghi chú" || a.Contact.Address != "" {
		t.Errorf("dòng 2: %+v", a.Contact)
	}
	if a.ResidentialUnit != "Thôn 1" || a.ResidentialUnitIsCode {
		t.Errorf("ô thôn gõ tay phải giữ nguyên chữ và không được coi là mã: %+v", a)
	}
	if strings.Join(a.GroupNames, "|") != "Hộ kinh doanh|Tổ trưởng" {
		t.Errorf("nhóm: %q", a.GroupNames)
	}
	if strings.Join(a.Contact.Tags, "|") != "a|b" {
		t.Errorf("nhãn: %q", a.Contact.Tags)
	}
	b := rows[1]
	if b.Contact.Phone != "0900000001" || b.ResidentialUnit != "" || b.GroupNames != nil || len(b.Contact.Tags) != 0 || b.Contact.Tags == nil {
		t.Errorf("dòng 5 (ô tuỳ chọn trống): %+v", b)
	}
}

func TestReadCitizenContactImportSheet_DropdownCellBecomesItsCode(t *testing.T) {
	rows, errs := ReadCitizenContactImportSheet(contactSheet(
		[]string{"A", fakeMobile, ResidentialUnitChoiceLabel("thon-binh-an", "Thôn Bình An")},
	))
	if len(errs) != 0 || len(rows) != 1 || rows[0].ResidentialUnit != "thon-binh-an" || !rows[0].ResidentialUnitIsCode {
		t.Errorf("ô thả xuống: %+v %+v", rows, errs)
	}
}

func TestReadCitizenContactImportSheet_EveryErrorNamesRowAndColumn(t *testing.T) {
	longName := strings.Repeat("ă", MaxCitizenContactFullName+1)
	cases := []struct {
		name string
		row  []string
		col  string
	}{
		{"thiếu họ tên", []string{" ", fakeMobile}, CitizenContactImportColFullName},
		{"họ tên quá dài", []string{longName, fakeMobile}, CitizenContactImportColFullName},
		{"thiếu số", []string{"A", ""}, CitizenContactImportColPhone},
		{"số sai ký tự", []string{"A", "09x0000000"}, CitizenContactImportColPhone},
		{"số không bắt đầu 0", []string{"A", "900000000"}, CitizenContactImportColPhone},
		{"ô thôn quá dài", []string{"A", fakeMobile, strings.Repeat("x", MaxResidentialUnitName+1)}, CitizenContactImportColResidentialUnit},
		{"địa chỉ quá dài", []string{"A", fakeMobile, "", strings.Repeat("ê", MaxCitizenContactAddress+1)}, CitizenContactImportColAddress},
		{"tên nhóm quá dài", []string{"A", fakeMobile, "", "", strings.Repeat("ơ", MaxCitizenGroupName+1)}, CitizenContactImportColGroups},
		{"nhãn quá dài", []string{"A", fakeMobile, "", "", "", strings.Repeat("ư", MaxCitizenContactTagLen+1)}, CitizenContactImportColTags},
		{"ghi chú quá dài", []string{"A", fakeMobile, "", "", "", "", strings.Repeat("ô", MaxCitizenContactNote+1)}, CitizenContactImportColNote},
		{"dữ liệu ngoài bảy cột", []string{"A", fakeMobile, "", "", "", "", "", "lạ"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, errs := ReadCitizenContactImportSheet(contactSheet(
				[]string{"Hợp lệ", "0900000002"},
				[]string{},
				c.row,
			))
			if rows != nil {
				t.Fatal("có lỗi mà vẫn trả dòng — tất cả hoặc không")
			}
			if len(errs) != 1 || errs[0].Row != 4 || errs[0].Column != c.col {
				t.Errorf("lỗi = %+v, muốn một lỗi dòng 4 cột %q", errs, c.col)
			}
		})
	}
}

func TestReadCitizenContactImportSheet_TooManyGroupsOnARow(t *testing.T) {
	names := make([]string, 0, MaxCitizenContactImportGroups+1)
	for i := 0; i <= MaxCitizenContactImportGroups; i++ {
		names = append(names, "Nhóm "+strings.Repeat("x", i))
	}
	_, errs := ReadCitizenContactImportSheet(contactSheet([]string{"A", fakeMobile, "", "", strings.Join(names, ", ")}))
	if len(errs) != 1 || errs[0].Column != CitizenContactImportColGroups {
		t.Errorf("quá nhiều nhóm: %+v", errs)
	}
}

func TestReadCitizenContactImportSheet_DuplicatePhoneInFile(t *testing.T) {
	// Two spellings of one number are one number (migration 0033: one live row per number and
	// commune). The later row is the error; the message names the FIRST ROW'S NUMBER, never the phone.
	rows, errs := ReadCitizenContactImportSheet(contactSheet(
		[]string{"A", fakeMobile},
		[]string{"B", "0900000001"},
		[]string{"C", "+84 900 000 000"},
	))
	if rows != nil || len(errs) != 1 {
		t.Fatalf("trùng số: rows %+v errs %+v", rows, errs)
	}
	e := errs[0]
	if e.Row != 4 || e.Column != CitizenContactImportColPhone || !strings.Contains(e.Message, "dòng 2") {
		t.Errorf("lỗi trùng phải ở dòng 4, cột số điện thoại, nhắc dòng 2: %+v", e)
	}
	if strings.Contains(e.Message, fakeMobile) || strings.Contains(e.Message, "900 000 000") {
		t.Errorf("thông báo trùng lặp lại số: %q", e.Message)
	}
}

func TestReadCitizenContactImportSheet_FileLimits(t *testing.T) {
	if _, errs := ReadCitizenContactImportSheet(contactSheet()); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("chỉ có tiêu đề: %+v", errs)
	}
	if _, errs := ReadCitizenContactImportSheet(contactSheet([]string{}, []string{" "})); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("toàn dòng trống: %+v", errs)
	}
	many := make([][]string, 0, MaxCitizenContactImportRows+1)
	for i := 0; i <= MaxCitizenContactImportRows; i++ {
		many = append(many, []string{"A", fakeMobile})
	}
	rows, errs := ReadCitizenContactImportSheet(contactSheet(many...))
	if rows != nil || len(errs) != 1 || errs[0].Row != 0 || !strings.Contains(errs[0].Message, "200") {
		t.Errorf("quá số dòng phải là một lỗi tệp nêu trần: %+v", errs)
	}
}

func TestReadCitizenContactImportSheet_ErrorsSortedByRow(t *testing.T) {
	_, errs := ReadCitizenContactImportSheet(contactSheet(
		[]string{"A", fakeMobile},
		[]string{"", "09x"},
		[]string{"B", fakeMobile},
	))
	if len(errs) != 3 || errs[0].Row != 3 || errs[1].Row != 3 || errs[2].Row != 4 {
		t.Errorf("lỗi phải theo thứ tự dòng: %+v", errs)
	}
}

// NO MESSAGE ECHOES A CELL (rule 3, forbidden #3; chapter 16 §7): a cell is a citizen's name, number
// or home address. Every column is filled with a marker and every message is checked for it.
func TestReadCitizenContactImportSheet_MessagesNeverEchoCells(t *testing.T) {
	const secret = "Bí Mật"
	over := func(n int) string { return strings.Repeat("x", n+1) + secret }
	row := []string{
		over(MaxCitizenContactFullName), fakeMobile + secret, over(MaxResidentialUnitName), over(MaxCitizenContactAddress),
		over(MaxCitizenGroupName), over(MaxCitizenContactTagLen), over(MaxCitizenContactNote), secret,
	}
	rows, errs := ReadCitizenContactImportSheet(contactSheet(row))
	if rows != nil || len(errs) != 8 {
		t.Fatalf("muốn 8 lỗi (bảy cột + ô ngoài bảng), nhận %d: %+v", len(errs), errs)
	}
	for _, e := range errs {
		if strings.Contains(e.Message, secret) || strings.Contains(e.Message, fakeMobile) || strings.Contains(e.Message, "xxx") {
			t.Errorf("thông báo lặp lại nội dung ô: %q", e.Message)
		}
		if e.Row != 2 {
			t.Errorf("mọi lỗi phải ở dòng 2: %+v", e)
		}
	}
	_, headErrs := ReadCitizenContactImportSheet([][]string{{secret, fakeMobile}})
	for _, e := range headErrs {
		if strings.Contains(e.Message, secret) || strings.Contains(e.Message, fakeMobile) {
			t.Errorf("lỗi tiêu đề lặp lại nội dung ô: %q", e.Message)
		}
	}
}
