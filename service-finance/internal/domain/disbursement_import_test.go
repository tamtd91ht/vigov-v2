package domain

import (
	"strings"
	"testing"
	"time"
)

func voucherHeader() []string { return VoucherImportColumns() }

func TestVoucherRowsFromSheet_HeaderMustBeTheTemplates(t *testing.T) {
	// The prototype's header (no source column, "Ghi chú" instead) is refused: a file built on it would
	// lose its notes silently and carry no source.
	old := []string{"Mã dự án", "Ngày chi (YYYY-MM-DD)", "Số tiền (đồng)", "Nội dung chi", "Đơn vị thụ hưởng", "Số chứng từ", "Ghi chú"}
	_, errs := VoucherRowsFromSheet([][]string{old, {"DA01"}})
	if len(errs) != 1 || errs[0].Row != 1 {
		t.Fatalf("errs = %+v", errs)
	}
	if _, errs := VoucherRowsFromSheet(nil); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("empty sheet: %+v", errs)
	}
}

func TestVoucherRowsFromSheet_SkipsBlankRowsKeepsNumbering(t *testing.T) {
	rows, errs := VoucherRowsFromSheet([][]string{
		voucherHeader(),
		{"DA01", "15/03/2026", "1000", "x"},
		{"", "  "},
		{"DA02", "16/03/2026", "2000", "y", "", "", "Ngân sách tỉnh"},
		{"DA03", "", "", "", "", "", "", "tràn cột"},
	})
	if len(rows) != 2 || rows[0].Row != 2 || rows[1].Row != 4 || rows[1].Source != "Ngân sách tỉnh" {
		t.Errorf("rows = %+v", rows)
	}
	if len(errs) != 1 || errs[0].Row != 5 {
		t.Errorf("extra-column row: %+v", errs)
	}
}

func TestParseVoucherImportDate(t *testing.T) {
	d := func(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"15/03/2026", d(2026, 3, 15), true},
		{"5/3/2026", d(2026, 3, 5), true},
		{" 29/02/2024 ", d(2024, 2, 29), true},
		{"46096", d(2026, 3, 15), true},    // Excel serial of 15/03/2026
		{"46096.75", d(2026, 3, 15), true}, // a date-time cell: the time is dropped
		{"", time.Time{}, false},
		{"31/02/2026", time.Time{}, false},
		{"2026-03-15", time.Time{}, false},
		{"15/03/26", time.Time{}, false},
		{"15/13/2026", time.Time{}, false},
		{"01/01/1999", time.Time{}, false}, // outside the form's year bound
		{"2026", time.Time{}, false},       // a year typed as a number is serial 2026 = 1905
		{"46096.", time.Time{}, false},
	} {
		got, msg := ParseVoucherImportDate(c.in)
		if (msg == "") != c.ok || (c.ok && !got.Equal(c.want)) {
			t.Errorf("%q: got %v %q, want %v ok=%v", c.in, got, msg, c.want, c.ok)
		}
	}
}

func TestParseVoucherImportAmount(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Dong
		ok   bool
	}{
		{"250000000", 250_000_000, true},
		{"250.000.000", 250_000_000, true},
		{"250,000,000", 250_000_000, true},
		{"1 500 000", 1_500_000, true},
		{"1.500", 1_500, true},
		{"", 0, false},
		{"0", 0, false},
		{"-5000", 0, false},
		{"1.5", 0, false},         // never read as 15
		{"250000000.5", 0, false}, // a fraction of a đồng
		{"1.500,000", 0, false},   // two kinds of separator
		{"1.50.000", 0, false},
		{"2.5E+8", 0, false},
		{"abc", 0, false},
		{"100000000000000001", 0, false}, // over SoTienToiDa
		{"99999999999999999999", 0, false},
	} {
		got, msg := ParseVoucherImportAmount(c.in)
		if (msg == "") != c.ok || got != c.want {
			t.Errorf("%q: got %d %q, want %d ok=%v", c.in, got, msg, c.want, c.ok)
		}
	}
}

func planFixture() (map[string]VoucherImportProject, []VoucherImportSource) {
	return map[string]VoucherImportProject{
			"DA01": {ID: "p1", Code: "DA01", Allocated: []string{"s1"}},
			"DA02": {ID: "p2", Code: "DA02"},
		}, []VoucherImportSource{
			{ID: "s1", Name: "Ngân sách tỉnh"},
			{ID: "s2", Name: "Ngân sách xã"},
		}
}

func row(n int, code, date, amount, desc, source string) VoucherImportRow {
	return VoucherImportRow{Row: n, ProjectCode: code, PaymentDate: date, Amount: amount, Description: desc, Source: source}
}

func TestPlanVoucherImport_ValidFile(t *testing.T) {
	projects, sources := planFixture()
	plan, errs := PlanVoucherImport([]VoucherImportRow{
		row(2, "DA01", "15/03/2026", "1.000.000", "Đợt 1", "Ngân sách tỉnh"),
		row(3, " DA02 ", "16/03/2026", "2000000", "Đợt 2", ""),
		// A DUPLICATE ROW IS ACCEPTED: two identical payments are real, as on the form.
		row(4, "DA02", "16/03/2026", "2000000", "Đợt 2", ""),
	}, projects, sources)
	if errs != nil {
		t.Fatalf("errs = %+v", errs)
	}
	if len(plan.Vouchers) != 3 || plan.RowCount != 3 || plan.Total != 5_000_000 {
		t.Fatalf("plan = %+v", plan)
	}
	if v := plan.Vouchers[0]; v.ProjectID != "p1" || v.SourceID != "s1" || v.Amount != 1_000_000 {
		t.Errorf("row 2 = %+v", v)
	}
	if v := plan.Vouchers[1]; v.ProjectID != "p2" || v.SourceID != "" {
		t.Errorf("row 3 = %+v", v)
	}
}

// EVERY error of the file is reported, sorted by row, with NO plan — and no message echoes a cell.
func TestPlanVoucherImport_EveryErrorNoPlanNoEcho(t *testing.T) {
	const secret = "Nguyễn Văn A 0900000000"
	projects, sources := planFixture()
	plan, errs := PlanVoucherImport([]VoucherImportRow{
		row(5, secret, "15/03/2026", "1000", "x", ""),
		row(2, "DA01", "31/02/2026", "1.5", "", ""),
		row(3, VoucherImportExampleProjectCode, "15/03/2026", "250000000", "Thanh toán", ""),
		row(4, "", "", "", "x", secret),
	}, projects, sources)
	if plan.Vouchers != nil {
		t.Fatal("a file with errors still has a plan")
	}
	byRow := map[int][]string{}
	for i, e := range errs {
		if i > 0 && errs[i-1].Row > e.Row {
			t.Errorf("not sorted by row: %+v", errs)
		}
		if strings.Contains(e.Message, "0900000000") || strings.Contains(e.Message, "Nguyễn") {
			t.Errorf("message echoes a cell: %q", e.Message)
		}
		byRow[e.Row] = append(byRow[e.Row], e.Column)
	}
	want := map[int][]string{
		2: {VoucherImportColDate, VoucherImportColAmount, VoucherImportColDescription, VoucherImportColSource},
		3: {VoucherImportColProject},
		4: {VoucherImportColProject, VoucherImportColDate, VoucherImportColAmount},
		5: {VoucherImportColProject},
	}
	for r, cols := range want {
		if strings.Join(byRow[r], "|") != strings.Join(cols, "|") {
			t.Errorf("row %d columns = %v, want %v", r, byRow[r], cols)
		}
	}
	for _, e := range errs {
		if e.Row == 3 && !strings.Contains(e.Message, "dòng ví dụ") {
			t.Errorf("the example row must be named as such: %q", e.Message)
		}
	}
	if plan.RowCount != 4 {
		t.Errorf("row_count %d", plan.RowCount)
	}
}

func TestPlanVoucherImport_SourceMessages(t *testing.T) {
	projects, sources := planFixture()
	for _, c := range []struct {
		code, source, want string
	}{
		{"DA01", "", "Dự án đã gắn nguồn vốn — ghi tên nguồn ở cột Nguồn vốn."},
		{"DA01", "Ngân sách xã", "không được phân bổ cho dự án"},
		{"DA01", "Không có", "Không có nguồn vốn tên này"},
		{"DA02", "Ngân sách tỉnh", "Dự án chưa gắn nguồn vốn"},
	} {
		_, errs := PlanVoucherImport([]VoucherImportRow{row(2, c.code, "15/03/2026", "1000", "x", c.source)}, projects, sources)
		if len(errs) != 1 || errs[0].Column != VoucherImportColSource || !strings.Contains(errs[0].Message, c.want) {
			t.Errorf("%s/%q: %+v", c.code, c.source, errs)
		}
	}
	// Two sources differing only by case: the folded match is ambiguous, the exact spelling still works.
	sources = append(sources, VoucherImportSource{ID: "s3", Name: "ngân sách tỉnh"})
	projects["DA01"] = VoucherImportProject{ID: "p1", Code: "DA01", Allocated: []string{"s1", "s3"}}
	if _, errs := PlanVoucherImport([]VoucherImportRow{row(2, "DA01", "15/03/2026", "1000", "x", "NGÂN SÁCH TỈNH")}, projects, sources); len(errs) != 1 {
		t.Errorf("ambiguous folded name accepted: %+v", errs)
	}
	if plan, errs := PlanVoucherImport([]VoucherImportRow{row(2, "DA01", "15/03/2026", "1000", "x", "ngân sách tỉnh")}, projects, sources); errs != nil || plan.Vouchers[0].SourceID != "s3" {
		t.Errorf("exact spelling refused: %+v", errs)
	}
}

func TestPlanVoucherImport_FileBounds(t *testing.T) {
	if _, errs := PlanVoucherImport(nil, nil, nil); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("no rows: %+v", errs)
	}
	many := make([]VoucherImportRow, MaxVoucherImportRows+1)
	for i := range many {
		many[i] = row(i+2, "DA02", "15/03/2026", "1000", "x", "")
	}
	if _, errs := PlanVoucherImport(many, nil, nil); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("over the row cap: %d errors", len(errs))
	}
}

func TestVoucherImportProjectCodes_DistinctTrimmedWithoutExample(t *testing.T) {
	got := VoucherImportProjectCodes([]VoucherImportRow{
		{ProjectCode: " DA02"}, {ProjectCode: "DA01"}, {ProjectCode: "DA02 "}, {ProjectCode: ""},
		{ProjectCode: VoucherImportExampleProjectCode},
	})
	if strings.Join(got, ",") != "DA01,DA02" {
		t.Errorf("codes = %v", got)
	}
}

// THE EXAMPLE CODE CAN NEVER BE A REAL PROJECT CODE — the property that makes an example row left in
// place harmless.
func TestVoucherImportExampleProjectCode_IsNotAProjectCode(t *testing.T) {
	if _, err := ChuanHoaMaDuAn(VoucherImportExampleProjectCode); err == nil {
		t.Fatal("the example row's code is a valid project code — a commune could own it")
	}
}
