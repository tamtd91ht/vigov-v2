package domain

// The rules of `Nhập từ Excel` on the citizen contact directory (ADR 0097 D3, D13; chapter 16 §7).
// THIS FILE ONLY READS AND CHECKS THE SHEET — standard library only (doc.go). Reading the workbook is
// core/xlsx.ReadSheet in the HTTP layer; rendering the template is core/xlsx.Template there too, from
// the header, example row, guide and dropdown column this file exposes (the staff template's shape,
// internal/http/staff_import.go). Resolving the residential unit and the group names against the
// commune, writing every row with ONE audit entry for the run, and ALL-OR-NOTHING are the app layer's.
//
// EVERY ROW GOES THROUGH THE SAME SHAPE RULES AS THE "Thêm người dân" FORM (citizen_directory.go), so a
// file cannot store what the form refuses. Two things the form does not have:
//
//	Nhóm        comma-separated group NAMES of the commune, matched by the app layer. Names, not codes:
//	            a group has no code (migration 0033), and the person filling the file knows "Hộ kinh
//	            doanh", not an id.
//	Zalo        NO such column (chapter 16 §7): the Zalo column is derived at read time from the
//	            citizen's own session (ADR 0097 §3), never typed.
//
// THE MESSAGES NEVER ECHO A CELL (rule 3, forbidden #3; chapter 16 §7): a cell holds a citizen's name,
// phone number or home address. They name the ROW and the COLUMN, which is all a person needs to find
// it — and a duplicate names the OTHER ROW'S NUMBER, never the number the two rows share.

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// CitizenContactImportSheetName is the data sheet's tab name — a UI string (rule 12, invariant 2).
const CitizenContactImportSheetName = "Danh bạ người dân"

// The seven columns of the template, in order (owner's card 16/10/2026). THE HEADER ROW MUST MATCH
// THEM: a sheet whose first row says something else is a different file, and guessing which column
// is which would put an address under "Ghi chú".
const (
	CitizenContactImportColFullName        = "Họ và tên"
	CitizenContactImportColPhone           = "Số điện thoại"
	CitizenContactImportColResidentialUnit = "Thôn, tổ dân phố"
	CitizenContactImportColAddress         = "Địa chỉ"
	CitizenContactImportColGroups          = "Nhóm"
	CitizenContactImportColTags            = "Nhãn"
	CitizenContactImportColNote            = "Ghi chú"
)

// CitizenContactImportColumns is the header row, in order.
func CitizenContactImportColumns() []string {
	return []string{
		CitizenContactImportColFullName, CitizenContactImportColPhone, CitizenContactImportColResidentialUnit,
		CitizenContactImportColAddress, CitizenContactImportColGroups, CitizenContactImportColTags,
		CitizenContactImportColNote,
	}
}

// CitizenContactImportResidentialUnitColumn is the 0-based column that carries the dropdown of the
// commune's residential units, each value ResidentialUnitChoiceLabel(code, name). The HTTP layer puts
// it in xlsx.TemplateSpec.Choices; the reader cuts a value back to its code (choiceCode).
const CitizenContactImportResidentialUnitColumn = 2

// MaxCitizenContactImportRows bounds one file: the citizen-letter register's 200
// (service-documents MaxLetterImportRows), the precedent the owner's card names. Every row is one
// INSERT plus its group memberships in ONE transaction with one audit entry; 200 keeps that
// transaction short. A commune of thousands imports several files; raising this is one constant and
// the owner's call, not a silent widening here.
const MaxCitizenContactImportRows = 200

// MaxCitizenContactImportGroups bounds the group names on one row. A commune defines a few dozen
// groups in all; a person in more than twenty is a cell that was filled with the wrong column.
const MaxCitizenContactImportGroups = 20

// ResidentialUnitChoiceLabel is one dropdown value of the residential-unit column: code, separator,
// name — the shape every template of this service uses (ChoiceSeparator, which is core/xlsx's).
func ResidentialUnitChoiceLabel(code, name string) string { return code + ChoiceSeparator + name }

// CitizenContactImportExampleRow is the one example row under the header: the agreed fake number
// (rule 3, invariant 5), a unit written the way the dropdown writes it, two groups, two tags.
func CitizenContactImportExampleRow() []string {
	return []string{
		"Nguyễn Văn A", "0900000000", ResidentialUnitChoiceLabel("thon-1", "Thôn 1"),
		"Số 1, đường Lê Lợi", "Tổ trưởng dân phố, Hộ kinh doanh", "tổ trưởng, hộ kinh doanh", "",
	}
}

// CitizenContactImportGuide is the guide sheet, one line per entry. The tag warning is the SAME
// sentence the form shows (TagSensitiveWarning) — one wording in both places.
func CitizenContactImportGuide() []string {
	return []string{
		"Mỗi dòng là một người dân xã muốn liên lạc. Bắt buộc: Họ và tên, Số điện thoại.",
		"Số điện thoại: chỉ chữ số; +84 hoặc 84 ở đầu được quy về 0. Một số chỉ có một dòng trong danh bạ.",
		"Thôn, tổ dân phố: chọn trong danh sách (mã · tên) hoặc gõ mã / tên đúng của xã; để trống nếu chưa rõ.",
		"Nhóm: tên các nhóm xã đã lập, cách nhau dấu phẩy. Nhóm chưa có thì lập ở ngăn Nhóm trước.",
		"Nhãn: chữ tự do, cách nhau dấu phẩy. " + TagSensitiveWarning + ".",
		fmt.Sprintf("Tối đa %d dòng cho một lần nhập. Một dòng sai thì không ghi dòng nào — sửa tệp rồi nhập lại.",
			MaxCitizenContactImportRows),
		"Tệp tải lên không được lưu lại; chỉ các dòng đã nhập và một vết cho cả lượt.",
	}
}

// CitizenContactImportRow is one data row, every shape rule already applied. Row is the spreadsheet's
// own number (header = 1).
//
// Contact is normalised and ready to write EXCEPT ResidentialUnitID, which the app layer fills from
// ResidentialUnit; IsActive is true (the template has no column for it); Source and ImportBatchID
// are the app layer's.
type CitizenContactImportRow struct {
	Row     int
	Contact CitizenContactInput
	// ResidentialUnit is the dropdown value cut to its CODE, or the bare text a person typed ("" =
	// none). ResidentialUnitIsCode says which: a dropdown value is matched by code only; a typed cell
	// may be a code or a name, and the app layer tries the code first (the staff import's rule).
	ResidentialUnit       string
	ResidentialUnitIsCode bool
	// GroupNames are the "Nhóm" cell's names, trimmed, deduplicated, in file order.
	GroupNames []string
}

// CitizenContactImportError is one refusal. Row 0 and Column "" mean the FILE; Row 1 is the header.
type CitizenContactImportError struct {
	Row     int
	Column  string
	Message string
}

// ReadCitizenContactImportSheet turns core/xlsx.ReadSheet's rows into checked data rows, or returns
// EVERY error. rows is nil whenever errs is non-empty, so a caller cannot write half a file by
// accident (ALL OR NOTHING, chapter 16 §7). A phone that appears twice in the file is an error on the
// later row (one number, one live row per commune — migration 0033); whether a number is already in
// the commune's directory is the app layer's check, inside the transaction.
func ReadCitizenContactImportSheet(sheet [][]string) ([]CitizenContactImportRow, []CitizenContactImportError) {
	want := CitizenContactImportColumns()
	if len(sheet) == 0 {
		return nil, []CitizenContactImportError{{Message: "Tệp không có dòng tiêu đề."}}
	}
	head := sheet[0]
	ok := len(head) >= len(want)
	for i := 0; ok && i < len(want); i++ {
		ok = strings.TrimSpace(head[i]) == want[i]
	}
	for i := len(want); ok && i < len(head); i++ {
		ok = strings.TrimSpace(head[i]) == ""
	}
	if !ok {
		return nil, []CitizenContactImportError{{Row: 1, Message: "Dòng tiêu đề không khớp tệp mẫu (cần đúng bảy cột: " +
			strings.Join(want, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var (
		rows []CitizenContactImportRow
		errs []CitizenContactImportError
	)
	phoneRow := make(map[string]int)
	data := 0
	for i, cells := range sheet[1:] {
		if blankCells(cells) {
			continue
		}
		data++
		if data > MaxCitizenContactImportRows {
			return nil, []CitizenContactImportError{{Message: fmt.Sprintf(
				"Tệp có hơn %d dòng người dân; tối đa %d dòng cho một lần nhập. Hãy chia thành nhiều tệp.",
				MaxCitizenContactImportRows, MaxCitizenContactImportRows)}}
		}
		r, rowErrs := readCitizenContactImportRow(i+2, cells, len(want))
		if r.Contact.Phone != "" {
			if first, dup := phoneRow[r.Contact.Phone]; dup {
				rowErrs = append(rowErrs, CitizenContactImportError{Row: r.Row, Column: CitizenContactImportColPhone,
					Message: fmt.Sprintf("Trùng số điện thoại với dòng %d trong tệp.", first)})
			} else {
				phoneRow[r.Contact.Phone] = r.Row
			}
		}
		errs = append(errs, rowErrs...)
		if len(rowErrs) == 0 {
			rows = append(rows, r)
		}
	}
	if data == 0 {
		return nil, []CitizenContactImportError{{Message: "Tệp không có dòng người dân nào dưới dòng tiêu đề."}}
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		return nil, errs
	}
	return rows, nil
}

func readCitizenContactImportRow(rowNo int, raw []string, width int) (CitizenContactImportRow, []CitizenContactImportError) {
	r := CitizenContactImportRow{Row: rowNo, Contact: CitizenContactInput{IsActive: true}}
	var errs []CitizenContactImportError
	fail := func(col, msg string) {
		errs = append(errs, CitizenContactImportError{Row: rowNo, Column: col, Message: msg})
	}

	for j := width; j < len(raw); j++ {
		if strings.TrimSpace(raw[j]) != "" {
			fail("", "Dòng có dữ liệu ngoài bảy cột của tệp mẫu.")
			break
		}
	}
	cell := func(j int) string {
		if j < len(raw) {
			return raw[j]
		}
		return ""
	}

	if name, err := NormalizeCitizenContactFullName(cell(0)); err != nil {
		fail(CitizenContactImportColFullName, citizenDirectorySentence(err))
	} else {
		r.Contact.FullName = name
	}

	if phone, err := NormalizeCitizenPhone(cell(1)); err != nil {
		fail(CitizenContactImportColPhone, citizenDirectorySentence(err))
	} else {
		r.Contact.Phone = phone
	}

	if v := strings.TrimSpace(cell(2)); v != "" {
		code, byCode := choiceCode(v)
		if utf8.RuneCountInString(code) > MaxResidentialUnitName {
			fail(CitizenContactImportColResidentialUnit, "Ô thôn / tổ dân phố quá dài. Hãy chọn trong danh sách của tệp mẫu.")
		} else {
			r.ResidentialUnit, r.ResidentialUnitIsCode = code, byCode
		}
	}

	if s, err := NormalizeCitizenContactAddress(cell(3)); err != nil {
		fail(CitizenContactImportColAddress, citizenDirectorySentence(err))
	} else {
		r.Contact.Address = s
	}

	if names, msg := readCitizenContactImportGroups(cell(4)); msg != "" {
		fail(CitizenContactImportColGroups, msg)
	} else {
		r.GroupNames = names
	}

	if tags, err := NormalizeCitizenContactTags(SplitCitizenContactTags(cell(5))); err != nil {
		fail(CitizenContactImportColTags, citizenDirectorySentence(err))
	} else {
		r.Contact.Tags = tags
	}

	if s, err := NormalizeCitizenContactNote(cell(6)); err != nil {
		fail(CitizenContactImportColNote, citizenDirectorySentence(err))
	} else {
		r.Contact.Note = s
	}
	return r, errs
}

// readCitizenContactImportGroups reads the "Nhóm" cell: names split on commas, whitespace collapsed,
// blanks dropped, repeats dropped case-insensitively, each bounded like a group name. Matching
// against the commune's groups is the app layer's.
func readCitizenContactImportGroups(cellText string) ([]string, string) {
	if strings.TrimSpace(cellText) == "" {
		return nil, ""
	}
	var out []string
	seen := map[string]bool{}
	for part := range strings.SplitSeq(cellText, ",") {
		name := gomKhoangTrang(part)
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > MaxCitizenGroupName {
			return nil, fmt.Sprintf("Tên nhóm quá dài (tối đa %d ký tự).", MaxCitizenGroupName)
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
		if len(out) > MaxCitizenContactImportGroups {
			return nil, fmt.Sprintf("Quá nhiều nhóm trên một dòng (tối đa %d).", MaxCitizenContactImportGroups)
		}
	}
	return out, ""
}

// citizenDirectorySentence is a domain sentence without its "danh bạ người dân: " prefix, capitalised
// for a person. The sentinels of citizen_directory.go name the rule and the bound, never the value.
func citizenDirectorySentence(err error) string {
	m := strings.TrimPrefix(err.Error(), "danh bạ người dân: ")
	if m == "" {
		return m
	}
	r := []rune(m)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r) + "."
}
