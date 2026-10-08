package domain

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

var importNow = time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)

func letterSheet(rows ...[]string) [][]string {
	return append([][]string{LetterImportColumns()}, rows...)
}

func TestReadLetterImportSheetAcceptsThePrototypeShapes(t *testing.T) {
	rows, errs := ReadLetterImportSheet(letterSheet(
		[]string{"2026-09-10", " Nguyễn Văn A ", "Thôn 1", "0900000000", "Khiếu nại", " Nội dung ", ""},
		[]string{}, // blank row: skipped, numbering kept
		[]string{"5/9/2026", "không rõ", "", "", "TỐ CÁO", "Nội dung", "dia-chinh"},
		[]string{"46275", "B", "", "", "kien-nghi-phan-anh", "x", ""}, // a date cell, raw serial = 2026-09-10
		[]string{"2026-09-10", "C", "", "", "Kiến nghị, phản ánh", "x", ""},
		[]string{"2026-09-10", "D", "", "", "đề nghị", "x", ""},
		[]string{"2026-09-10", "E", "", "", "phản ánh", "x", ""},
	), importNow)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	if len(rows) != 6 || rows[0].Row != 2 || rows[1].Row != 4 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].SenderName != "Nguyễn Văn A" || rows[0].Summary != "Nội dung" || rows[0].Type != LetterTypeComplaint {
		t.Fatalf("dòng 2 chưa cắt khoảng trắng / sai loại: %+v", rows[0])
	}
	if rows[1].SenderName != "" || rows[1].Type != LetterTypeDenunciation || rows[1].UnitCode != "dia-chinh" ||
		!rows[1].ReceivedDate.Equal(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("dòng 4: %+v", rows[1])
	}
	if !rows[2].ReceivedDate.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("ngày dạng số sê-ri đọc thành %v", rows[2].ReceivedDate)
	}
	for i, want := range []LetterType{LetterTypeFeedback, LetterTypeFeedback, LetterTypeRequest, LetterTypeFeedback} {
		if rows[2+i].Type != want {
			t.Fatalf("dòng %d loại %q, muốn %q", rows[2+i].Row, rows[2+i].Type, want)
		}
	}
}

func TestReadLetterImportSheetRefusesWithoutDefaults(t *testing.T) {
	const secret = "Lê Thị Bí Mật"
	_, errs := ReadLetterImportSheet(letterSheet(
		[]string{"", secret, "", "", "", "Nội dung", ""},                   // no date, no type: no default for either
		[]string{"2027-01-01", "", "", "", "khiếu nại", "", ""},            // future, no sender, no summary
		[]string{"2026-09-10", secret, "", "09x", "khác", "x", "", "thừa"}, // phone, type, extra cell
		[]string{"2026-09-10", strings.Repeat("a", 201), "", "", "tố cáo", "x", strings.Repeat("b", 65)},
	), importNow)
	want := map[string]bool{
		"2|" + LetterImportColReceived: true, "2|" + LetterImportColType: true,
		"3|" + LetterImportColReceived: true, "3|" + LetterImportColSender: true, "3|" + LetterImportColSummary: true,
		"4|" + LetterImportColPhone: true, "4|" + LetterImportColType: true, "4|": true,
		"5|" + LetterImportColSender: true, "5|" + LetterImportColUnit: true,
	}
	if len(errs) != len(want) {
		t.Fatalf("%d lỗi, muốn %d: %+v", len(errs), len(want), errs)
	}
	for _, e := range errs {
		if !want[strconv.Itoa(e.Row)+"|"+e.Column] {
			t.Fatalf("lỗi không mong đợi %+v", e)
		}
		if strings.Contains(e.Message, secret) || strings.Contains(e.Message, "09x") || strings.Contains(e.Message, "khác") {
			t.Fatalf("thông báo lặp lại nội dung ô: %q", e.Message)
		}
	}
}

func TestReadLetterImportSheetRefusesTheFile(t *testing.T) {
	cases := map[string][][]string{
		"tiêu đề lạ":     {{"Số đến", "Ngày đến (YYYY-MM-DD)"}},
		"thiếu dòng":     letterSheet(),
		"chỉ dòng trống": letterSheet([]string{"", " "}),
	}
	many := letterSheet()
	for i := 0; i <= MaxLetterImportRows; i++ {
		many = append(many, []string{"2026-09-10", "A", "", "", "khiếu nại", "x", ""})
	}
	cases["quá 200 dòng"] = many
	for name, sheet := range cases {
		t.Run(name, func(t *testing.T) {
			rows, errs := ReadLetterImportSheet(sheet, importNow)
			if rows != nil || len(errs) != 1 || errs[0].Row != 0 {
				t.Fatalf("rows %d, errs %+v", len(rows), errs)
			}
		})
	}
}
