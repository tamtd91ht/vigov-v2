package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

// ReadSheets reads every tab; ReadSheet must keep reading only the first one, because the imports
// built on it (residential units, staff, the catalogues, disbursement vouchers) assume exactly that.

// twoSheetWorkbook: "Thu" holds a NUMBER 1.234 and a TEXT "1.234"; "Chi" holds a row of its own; a
// third, hidden tab "Ghi chú" holds a note.
func twoSheetWorkbook(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Thu"); err != nil {
		t.Fatal(err)
	}
	set := func(sheet, cell string, v any) {
		t.Helper()
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	set("Thu", "A1", "TT")
	set("Thu", "B1", "Nội dung")
	set("Thu", "A2", "1")
	set("Thu", "C2", 1.234)
	set("Thu", "D2", "1.234")
	if _, err := f.NewSheet("Chi"); err != nil {
		t.Fatal(err)
	}
	set("Chi", "A1", "STT")
	set("Chi", "C3", 42)
	if _, err := f.NewSheet("Ghi chú"); err != nil {
		t.Fatal(err)
	}
	set("Ghi chú", "A1", "nháp")
	if err := f.SetSheetVisible("Ghi chú", false); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadSheets_EverySheetInOrderWithCellTypes(t *testing.T) {
	b := twoSheetWorkbook(t)
	sheets, err := ReadSheets(bytes.NewReader(b), int64(len(b)), Limits{})
	if err != nil {
		t.Fatalf("ReadSheets: %v", err)
	}
	if len(sheets) != 3 || sheets[0].Name != "Thu" || sheets[1].Name != "Chi" || sheets[2].Name != "Ghi chú" {
		t.Fatalf("sheets = %+v", sheets)
	}
	if sheets[0].Hidden || sheets[1].Hidden || !sheets[2].Hidden {
		t.Errorf("hidden flags = %v %v %v", sheets[0].Hidden, sheets[1].Hidden, sheets[2].Hidden)
	}
	thu := sheets[0]
	if got := thu.Rows[1][2]; got != "1.234" {
		t.Errorf("number cell raw = %q", got)
	}
	if got := thu.Rows[1][3]; got != "1.234" {
		t.Errorf("text cell raw = %q", got)
	}
	// THE WHOLE POINT: the same five characters, one a number and one text.
	if !thu.IsNumber(1, 2) || thu.IsNumber(1, 3) {
		t.Errorf("IsNumber(number)=%v IsNumber(text)=%v", thu.IsNumber(1, 2), thu.IsNumber(1, 3))
	}
	if thu.IsNumber(0, 0) || thu.IsNumber(1, 0) {
		t.Error("text cells reported as numbers")
	}
	// Blank row 2 of "Chi" is kept so row numbers stay the spreadsheet's own.
	chi := sheets[1]
	if len(chi.Rows) != 3 || len(chi.Rows[1]) != 0 || !chi.IsNumber(2, 2) {
		t.Errorf("Chi rows = %q", chi.Rows)
	}
}

// OLD READER UNCHANGED: on the same three-tab workbook ReadSheet returns the first tab only, exactly
// as ReadSheets returns it.
func TestReadSheet_StillReadsOnlyTheFirstSheet(t *testing.T) {
	b := twoSheetWorkbook(t)
	rows, err := read(b)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	sheets, err := ReadSheets(bytes.NewReader(b), int64(len(b)), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, sheets[0].Rows) {
		t.Fatalf("ReadSheet = %q, first sheet = %q", rows, sheets[0].Rows)
	}
	for _, r := range rows {
		for _, c := range r {
			if c == "nháp" || c == "STT" {
				t.Fatalf("ReadSheet read past the first sheet: %q", rows)
			}
		}
	}
}

// A blank FIRST sheet: the old reader still says ErrEmptySheet; the new one reads the others.
func TestReadSheets_BlankFirstSheetDoesNotHideTheRest(t *testing.T) {
	f := excelize.NewFile()
	if _, err := f.NewSheet("Chi"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Chi", "A1", "TT"); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if _, err := read(b); !errors.Is(err, ErrEmptySheet) {
		t.Fatalf("ReadSheet on a blank first sheet = %v, want ErrEmptySheet (unchanged)", err)
	}
	sheets, err := ReadSheets(bytes.NewReader(b), int64(len(b)), Limits{})
	if err != nil || len(sheets) != 2 || sheets[0].Rows != nil || sheets[1].Rows[0][0] != "TT" {
		t.Fatalf("ReadSheets = %+v, %v", sheets, err)
	}
}

func TestReadSheets_AllBlankIsEmptySheet(t *testing.T) {
	f := excelize.NewFile()
	buf, err := f.WriteToBuffer()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if _, err := ReadSheets(bytes.NewReader(b), int64(len(b)), Limits{}); !errors.Is(err, ErrEmptySheet) {
		t.Fatalf("err = %v, want ErrEmptySheet", err)
	}
}

func TestReadSheets_TooManySheetsIsTooLarge(t *testing.T) {
	f := excelize.NewFile()
	for i := 0; i < MaxSheets; i++ {
		if _, err := f.NewSheet(fmt.Sprintf("S%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	buf, err := f.WriteToBuffer()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if _, err := ReadSheets(bytes.NewReader(b), int64(len(b)), Limits{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("%d sheets: err = %v, want ErrTooLarge", MaxSheets+1, err)
	}
}

// THE SAME GUARDS: a macro part and a non-zip are refused by ReadSheets exactly as by ReadSheet.
func TestReadSheets_SharesTheGuards(t *testing.T) {
	macro := rewriteZip(t, good(t), nil, map[string][]byte{"xl/vbaProject.bin": []byte("x")})
	if _, err := ReadSheets(bytes.NewReader(macro), int64(len(macro)), Limits{}); !errors.Is(err, ErrMacroEnabled) {
		t.Errorf("macro: %v", err)
	}
	if _, err := ReadSheets(bytes.NewReader([]byte("không phải excel")), 20, Limits{}); !errors.Is(err, ErrNotXLSX) {
		t.Errorf("not xlsx: %v", err)
	}
	if _, err := ReadSheets(bytes.NewReader(nil), lim.MaxFileBytes+1, Limits{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("declared size: %v", err)
	}
}
