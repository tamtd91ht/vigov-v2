package xlsx

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// The sheet names Template adds. Vietnamese: a person reads them in Excel's tab bar (rule 12,
// invariant 2 — a UI string, not an identifier).
const (
	SheetGuide   = "Hướng dẫn"
	SheetChoices = "Danh sách chọn"
)

// Choices is the dropdown of one data column.
type Choices struct {
	// Values are listed on the hidden SheetChoices sheet, one column per dropdown, and written as
	// STRING cells whatever the text: a value beginning with "=" stays text, never a formula.
	Values []string
	// Strict makes Excel REFUSE a value outside the list; false only WARNS. Warn is the right choice
	// whenever a valid value may be something the list cannot hold today (orgunitxlsx: a parent that
	// is a row above in the same file). Either way the server re-validates: a dropdown is a
	// convenience for the person filling the file, never a control.
	Strict bool
	// Title and Message are what Excel shows on a value outside the list. Empty: Excel's default.
	Title, Message string
}

// TemplateSpec describes one import template.
type TemplateSpec struct {
	// SheetName is the data sheet, always the FIRST sheet — the one ReadSheet reads.
	SheetName string
	// Header is row 1, bold. Required.
	Header []string
	// Examples are the rows after the header, written as STRING cells: no example can become a
	// formula, and a code such as "007" keeps its zeros. None may be longer than Header.
	Examples [][]string
	// Guide, when non-empty, is written one line per row on a second sheet, SheetGuide.
	Guide []string
	// Choices maps a 0-based column of Header to its dropdown. Columns with no entry have none.
	Choices map[int]Choices
	// DropdownRows is how many data rows (after the header) the dropdowns cover. Zero means
	// DefaultLimits.MaxRows - 1, i.e. every row ReadSheet would accept.
	DropdownRows int
}

// ErrTemplateSpec — the spec is unusable (no header, an example wider than the header, a dropdown on
// a column that does not exist, a sheet name that collides). A programming error, not user input.
var ErrTemplateSpec = errors.New("xlsx: invalid template spec")

// Template builds the .xlsx a person downloads, fills and uploads back to ReadSheet: the data sheet
// (header + examples), an optional guide sheet, and — when any column has Choices — a HIDDEN sheet
// feeding the dropdowns.
func Template(spec TemplateSpec) ([]byte, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", spec.SheetName); err != nil {
		return nil, fmt.Errorf("xlsx: name data sheet: %w", err)
	}
	widths := make([]int, len(spec.Header))
	writeRow := func(sheet string, rowNo int, cells []string) error {
		for i, v := range cells {
			cell, err := excelize.CoordinatesToCellName(i+1, rowNo)
			if err != nil {
				return fmt.Errorf("xlsx: cell name: %w", err)
			}
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				return fmt.Errorf("xlsx: write cell: %w", err)
			}
			if sheet == spec.SheetName {
				if n := utf8.RuneCountInString(v); n > widths[i] {
					widths[i] = n
				}
			}
		}
		return nil
	}
	if err := writeRow(spec.SheetName, 1, spec.Header); err != nil {
		return nil, err
	}
	for i, ex := range spec.Examples {
		if err := writeRow(spec.SheetName, i+2, ex); err != nil {
			return nil, err
		}
	}
	for i, w := range widths {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, fmt.Errorf("xlsx: column name: %w", err)
		}
		if err := f.SetColWidth(spec.SheetName, col, col, float64(min(max(w+2, 12), 60))); err != nil {
			return nil, fmt.Errorf("xlsx: column width: %w", err)
		}
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, fmt.Errorf("xlsx: header style: %w", err)
	}
	lastHeader, err := excelize.CoordinatesToCellName(len(spec.Header), 1)
	if err != nil {
		return nil, fmt.Errorf("xlsx: cell name: %w", err)
	}
	if err := f.SetCellStyle(spec.SheetName, "A1", lastHeader, bold); err != nil {
		return nil, fmt.Errorf("xlsx: header style: %w", err)
	}

	if len(spec.Guide) > 0 {
		if _, err := f.NewSheet(SheetGuide); err != nil {
			return nil, fmt.Errorf("xlsx: guide sheet: %w", err)
		}
		for i, line := range spec.Guide {
			if err := f.SetCellStr(SheetGuide, fmt.Sprintf("A%d", i+1), line); err != nil {
				return nil, fmt.Errorf("xlsx: write guide: %w", err)
			}
		}
		if err := f.SetColWidth(SheetGuide, "A", "A", 120); err != nil {
			return nil, fmt.Errorf("xlsx: guide width: %w", err)
		}
	}

	if err := addChoices(f, spec); err != nil {
		return nil, err
	}
	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("xlsx: write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// addChoices writes every non-empty list into its own column of the hidden SheetChoices sheet and
// points a data validation at it. A range reference, not an inline list: Excel caps an inline list
// at 255 characters, which one commune's unit names exceed.
func addChoices(f *excelize.File, spec TemplateSpec) error {
	rows := spec.DropdownRows
	if rows <= 0 {
		rows = DefaultLimits.MaxRows - 1
	}
	listCol := 0
	for col := 0; col < len(spec.Header); col++ { // header order: a deterministic file
		c, ok := spec.Choices[col]
		if !ok || len(c.Values) == 0 {
			continue
		}
		if listCol == 0 {
			if _, err := f.NewSheet(SheetChoices); err != nil {
				return fmt.Errorf("xlsx: choices sheet: %w", err)
			}
			if err := f.SetSheetVisible(SheetChoices, false); err != nil {
				return fmt.Errorf("xlsx: hide choices sheet: %w", err)
			}
		}
		listCol++
		listName, err := excelize.ColumnNumberToName(listCol)
		if err != nil {
			return fmt.Errorf("xlsx: column name: %w", err)
		}
		for i, v := range c.Values {
			if err := f.SetCellStr(SheetChoices, fmt.Sprintf("%s%d", listName, i+1), v); err != nil {
				return fmt.Errorf("xlsx: write choices: %w", err)
			}
		}
		dataName, err := excelize.ColumnNumberToName(col + 1)
		if err != nil {
			return fmt.Errorf("xlsx: column name: %w", err)
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = fmt.Sprintf("%s2:%s%d", dataName, dataName, rows+1)
		dv.SetSqrefDropList(fmt.Sprintf("'%s'!$%s$1:$%s$%d", SheetChoices, listName, listName, len(c.Values)))
		style := excelize.DataValidationErrorStyleWarning
		if c.Strict {
			style = excelize.DataValidationErrorStyleStop
		}
		dv.SetError(style, c.Title, c.Message)
		if err := f.AddDataValidation(spec.SheetName, dv); err != nil {
			return fmt.Errorf("xlsx: add dropdown: %w", err)
		}
	}
	return nil
}

func validateSpec(spec TemplateSpec) error {
	if spec.SheetName == "" || spec.SheetName == SheetGuide || spec.SheetName == SheetChoices || len(spec.Header) == 0 {
		return ErrTemplateSpec
	}
	if len(spec.Header) > DefaultLimits.MaxColumns {
		return ErrTemplateSpec // ReadSheet would drop the columns past it
	}
	if len(spec.Examples)+1 > DefaultLimits.MaxRows {
		return ErrTemplateSpec
	}
	for _, ex := range spec.Examples {
		if len(ex) > len(spec.Header) {
			return ErrTemplateSpec
		}
	}
	for col := range spec.Choices {
		if col < 0 || col >= len(spec.Header) {
			return ErrTemplateSpec
		}
	}
	return nil
}
