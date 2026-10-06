package domain

// The rules of an IMPORT FROM EXCEL of disbursement vouchers — "Nhập lần giải ngân từ Excel"
// (docs/ui-ux/06-giai-ngan.md §10, §13 rule 7): the whole file is checked before anything is written,
// and one bad row means NO row is taken — "Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi
// nhập lại." Same shape as this service's catalogue import (catalogue_import.go): a sheet reader, a
// planner that returns either the plan or every error, never both.
//
// THE COLUMNS are the prototype's (../vigov-require apps/api/app/modules/budget/importer.py:28-36)
// with two deliberate differences:
//
//	"Ghi chú" is DROPPED   chung_tu_giai_ngan has no note column (migration 0004). A column read and
//	                       then thrown away is somebody's text silently lost — the catalogue import's
//	                       reasoning for dropping `Diễn giải`.
//	"Nguồn vốn" is ADDED   see THE SOURCE RULE below.
//
// THE SOURCE RULE — ⚠ A DECISION THE MAIN SESSION MUST CONFIRM WITH THE USER. The prototype imports
// every voucher with a NULL source (importer.py:227-235) and so never meets the rule the user decided
// on 06/10/2026 (CheckVoucherSource, commit db94b35c): a voucher on a project WITH allocation lines
// must name one of that project's sources. Bypassing it here would let a file create exactly the
// vouchers the form refuses. So the template carries an optional "Nguồn vốn" column holding the SOURCE
// NAME as the commune's catalogue spells it, and:
//
//	project has allocation lines, cell empty      row error "Dự án đã gắn nguồn vốn — ghi tên nguồn
//	                                              ở cột Nguồn vốn"
//	project has allocation lines, cell names one  must be one of THAT project's allocated sources
//	project has no allocation line                the cell must be empty
//
// A name and not an id because a person fills the file; names are unique per commune (migration
// 0013, `nguon_von_name_unique`), so a name resolves to at most one source.
//
// EVERY FIELD GOES THROUGH THE SAME CHECKS AS POST /api/v1/disbursements — KiemTraSoTien,
// KiemTraNgayChi, ChuanHoaNoiDung, ChuanHoaDoiTac, ChuanHoaSoChungTu, CheckVoucherSource — so a row
// that could not be typed into the form cannot be imported. The application layer then writes each row
// through the very function the form's create runs, which re-checks all of it (app/disbursement_import.go).
//
// WHAT IS DELIBERATELY NOT CHECKED, matching the form so the two paths cannot disagree:
//
//	the project's year   the form does not compare `ngay_chi` with `du_an.nam` either; a payment in
//	                     January on last year's project is a real thing. A project code is unique in
//	                     the commune across years (UNIQUE (tenant_id, ma), 0004), so the code alone
//	                     names one project.
//	a closed period      the budget period close (0012) applies to the thu-chi entries, not to vouchers.
//	duplicate rows       two genuine payments to one company on one day for one amount are real (the
//	                     reason POST /api/v1/disbursements has no uniqueness key, routes.go). The
//	                     prototype does not refuse them either.
//
// NO MESSAGE ECHOES A CELL. They name the row and the column. STANDARD LIBRARY ONLY.

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The seven columns of the template, in order. THE HEADER ROW MUST MATCH THEM.
const (
	VoucherImportColProject      = "Mã dự án"
	VoucherImportColDate         = "Ngày chi (dd/mm/yyyy)"
	VoucherImportColAmount       = "Số tiền (đồng)"
	VoucherImportColDescription  = "Nội dung chi"
	VoucherImportColCounterparty = "Đơn vị thụ hưởng"
	VoucherImportColVoucherNo    = "Số chứng từ"
	VoucherImportColSource       = "Nguồn vốn"
)

// VoucherImportColumns is the header row, in order.
func VoucherImportColumns() []string {
	return []string{
		VoucherImportColProject, VoucherImportColDate, VoucherImportColAmount, VoucherImportColDescription,
		VoucherImportColCounterparty, VoucherImportColVoucherNo, VoucherImportColSource,
	}
}

// MaxVoucherImportRows bounds one file. Every row is an INSERT plus an audit entry inside ONE
// transaction that holds a share lock on each project it names, so the bound is the length of that
// transaction. 500 is a commune's busy year of payments; a larger backlog is two files.
const MaxVoucherImportRows = 500

// VoucherImportExampleProjectCode is the project code of the template's example row. IT CAN NEVER BE A
// REAL PROJECT CODE — it holds spaces and diacritics, which ChuanHoaMaDuAn refuses — and the planner
// names it, so an example row left in the file refuses the WHOLE file with a sentence saying why,
// instead of becoming a payment. (The catalogue template has no example row at all, for the same
// reason; here one is asked for, so it is made harmless rather than omitted.)
const VoucherImportExampleProjectCode = "VÍ DỤ - xoá dòng này"

// VoucherImportRow is one data row AS TYPED. Row is the spreadsheet's own number (header = 1).
type VoucherImportRow struct {
	Row          int
	ProjectCode  string
	PaymentDate  string
	Amount       string
	Description  string
	Counterparty string
	VoucherNo    string
	Source       string
}

// VoucherImportError is one refusal. Row 0 and Column "" mean the FILE; Row 1 is the header.
type VoucherImportError struct {
	Row     int
	Column  string
	Message string
}

// VoucherImportProject is one LIVE project of the commune that a row names, with the sources of its
// live allocation lines — read under the project row's share lock, as the form's create reads it.
type VoucherImportProject struct {
	ID        string
	Code      string
	Allocated []string // funding source ids; empty = the project declared no source
}

// VoucherImportSource is one LIVE funding source of the commune's catalogue.
type VoucherImportSource struct {
	ID   string
	Name string
}

// PlannedVoucher is one row the import would write, already normalised.
type PlannedVoucher struct {
	Row          int
	ProjectID    string
	ProjectCode  string
	PaymentDate  time.Time
	Amount       Dong
	Description  string
	Counterparty string
	VoucherNo    string
	SourceID     string // "" = no source (only on a project with no allocation line)
}

// VoucherImportPlan is what a file amounts to. RowCount and Total are filled even when the file has
// errors, so the preview can say "N dòng, tổng X đồng"; Total then counts only the rows whose amount
// was readable. Vouchers is nil whenever there is at least one error, so a caller cannot write half a
// file by accident.
type VoucherImportPlan struct {
	Vouchers []PlannedVoucher
	RowCount int
	Total    Dong
}

// VoucherRowsFromSheet turns the raw cells of the first sheet (core/xlsx.ReadSheet: cells[0] is row 1)
// into data rows, or refuses the SHEET when its header is not the template's. Entirely blank rows are
// skipped and keep the numbering of the rows around them.
func VoucherRowsFromSheet(cells [][]string) ([]VoucherImportRow, []VoucherImportError) {
	cols := VoucherImportColumns()
	if len(cells) == 0 {
		return nil, []VoucherImportError{{Message: "Tệp không có dòng nào."}}
	}
	header := cells[0]
	ok := len(header) >= len(cols)
	for i := 0; ok && i < len(cols); i++ {
		ok = strings.TrimSpace(header[i]) == cols[i]
	}
	for i := len(cols); ok && i < len(header); i++ {
		ok = strings.TrimSpace(header[i]) == ""
	}
	if !ok {
		return nil, []VoucherImportError{{Row: 1,
			Message: "Dòng tiêu đề không khớp tệp mẫu (" + strings.Join(cols, " · ") + "). Hãy tải tệp mẫu và nhập vào đó."}}
	}

	var rows []VoucherImportRow
	var errs []VoucherImportError
	for i := 1; i < len(cells); i++ {
		c := cells[i]
		if blankCatalogueCells(c) {
			continue
		}
		extra := false
		for j := len(cols); j < len(c); j++ {
			if strings.TrimSpace(c[j]) != "" {
				extra = true
				break
			}
		}
		if extra {
			errs = append(errs, VoucherImportError{Row: i + 1,
				Message: fmt.Sprintf("Dòng có dữ liệu ngoài %d cột của tệp mẫu.", len(cols))})
			continue
		}
		at := func(j int) string {
			if j < len(c) {
				return c[j]
			}
			return ""
		}
		rows = append(rows, VoucherImportRow{Row: i + 1,
			ProjectCode: at(0), PaymentDate: at(1), Amount: at(2), Description: at(3),
			Counterparty: at(4), VoucherNo: at(5), Source: at(6)})
	}
	return rows, errs
}

// VoucherImportProjectCodes is the distinct trimmed project codes the rows name — what the store is
// asked for. The example row's code is left out: it can match nothing, and the planner names it.
func VoucherImportProjectCodes(rows []VoucherImportRow) []string {
	seen := make(map[string]bool, len(rows))
	var out []string
	for _, r := range rows {
		c := strings.TrimSpace(r.ProjectCode)
		if c == "" || c == VoucherImportExampleProjectCode || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// PlanVoucherImport validates the WHOLE file and returns the plan, or every error with a nil plan.
// `projects` are the live projects the rows name, keyed by code; `sources` the commune's live catalogue.
func PlanVoucherImport(rows []VoucherImportRow, projects map[string]VoucherImportProject,
	sources []VoucherImportSource) (VoucherImportPlan, []VoucherImportError) {

	var errs []VoucherImportError
	plan := VoucherImportPlan{RowCount: len(rows)}
	fileErr := func(msg string) { errs = append(errs, VoucherImportError{Message: msg}) }

	if len(rows) == 0 {
		fileErr("Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.")
		return plan, errs
	}
	if len(rows) > MaxVoucherImportRows {
		fileErr(fmt.Sprintf("Tệp có %d dòng dữ liệu, tối đa %d dòng cho một lần nhập — hãy chia thành nhiều tệp.",
			len(rows), MaxVoucherImportRows))
		return plan, errs
	}

	rowErr := func(r VoucherImportRow, col, msg string) {
		errs = append(errs, VoucherImportError{Row: r.Row, Column: col, Message: msg})
	}
	planned := make([]PlannedVoucher, 0, len(rows))
	overflow := false
	for _, r := range rows {
		p := PlannedVoucher{Row: r.Row}

		// THE PROJECT. The example row first: it is the one mistake worth its own sentence.
		var project VoucherImportProject
		projectOK := false
		switch code := strings.TrimSpace(r.ProjectCode); {
		case code == VoucherImportExampleProjectCode:
			rowErr(r, VoucherImportColProject, "Đây là dòng ví dụ của tệp mẫu — hãy xoá dòng này rồi nhập lại.")
		case code == "":
			rowErr(r, VoucherImportColProject, "Thiếu mã dự án.")
		default:
			if pr, ok := projects[code]; ok {
				project, projectOK = pr, true
				p.ProjectID, p.ProjectCode = pr.ID, pr.Code
			} else {
				// One sentence for "no such code" and "a code of another commune": the store cannot
				// reach the second, so the two are one answer (rule 1).
				rowErr(r, VoucherImportColProject, "Không có dự án mã này trong xã (hoặc dự án đã bị xoá).")
			}
		}

		if d, msg := ParseVoucherImportDate(r.PaymentDate); msg != "" {
			rowErr(r, VoucherImportColDate, msg)
		} else {
			p.PaymentDate = d
		}

		if a, msg := ParseVoucherImportAmount(r.Amount); msg != "" {
			rowErr(r, VoucherImportColAmount, msg)
		} else {
			p.Amount = a
			if plan.Total > Dong(math.MaxInt64)-a {
				overflow = true
			} else {
				plan.Total += a
			}
		}

		if s, err := ChuanHoaNoiDung(r.Description); err != nil {
			if errors.Is(err, ErrNoiDungQuaDai) {
				rowErr(r, VoucherImportColDescription, fmt.Sprintf("Nội dung chi dài quá %d ký tự.", NoiDungChungTuToiDa))
			} else {
				rowErr(r, VoucherImportColDescription, "Thiếu nội dung chi (hoặc có ký tự điều khiển).")
			}
		} else {
			p.Description = s
		}
		if s, err := ChuanHoaDoiTac(r.Counterparty); err != nil {
			rowErr(r, VoucherImportColCounterparty,
				fmt.Sprintf("Đơn vị thụ hưởng dài quá %d ký tự (hoặc có ký tự điều khiển).", DoiTacToiDa))
		} else {
			p.Counterparty = s
		}
		if s, err := ChuanHoaSoChungTu(r.VoucherNo); err != nil {
			rowErr(r, VoucherImportColVoucherNo,
				fmt.Sprintf("Số chứng từ dài quá %d ký tự (hoặc có ký tự điều khiển).", SoChungTuToiDa))
		} else {
			p.VoucherNo = s
		}

		// THE SOURCE — judged only once the project is known: the rule is the project's.
		if projectOK {
			if id, msg := resolveVoucherImportSource(r.Source, project, sources); msg != "" {
				rowErr(r, VoucherImportColSource, msg)
			} else {
				p.SourceID = id
			}
		}
		planned = append(planned, p)
	}
	if overflow {
		fileErr("Tổng số tiền của tệp vượt mức tính được — hãy chia thành nhiều tệp.")
	}

	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		return plan, errs
	}
	plan.Vouchers = planned
	return plan, nil
}

// resolveVoucherImportSource applies THE SOURCE RULE (file header) to one row. The decision itself is
// CheckVoucherSource — the form's own — once the name has become an id.
//
// THE NAME IS MATCHED EXACTLY after trimming, then case-insensitively when that finds nothing and
// exactly one source matches that way. The catalogue's unique key compares names exactly (0013), so
// "Ngân sách tỉnh" and "ngân sách tỉnh" could both exist; when they do, only the exact spelling works.
func resolveVoucherImportSource(raw string, project VoucherImportProject, sources []VoucherImportSource) (string, string) {
	name := strings.TrimSpace(raw)
	if len(project.Allocated) == 0 {
		if name != "" {
			return "", "Dự án chưa gắn nguồn vốn nào — để trống cột Nguồn vốn."
		}
		return "", ""
	}
	if name == "" {
		return "", "Dự án đã gắn nguồn vốn — ghi tên nguồn ở cột Nguồn vốn."
	}
	id := ""
	for _, s := range sources {
		if s.Name == name {
			id = s.ID
			break
		}
	}
	if id == "" {
		folded, n := strings.ToLower(name), 0
		for _, s := range sources {
			if strings.ToLower(s.Name) == folded {
				id, n = s.ID, n+1
			}
		}
		if n > 1 {
			return "", "Có nhiều nguồn vốn cùng tên này khi không phân biệt chữ hoa, chữ thường — hãy ghi đúng tên như trong danh mục."
		}
	}
	if id == "" {
		return "", "Không có nguồn vốn tên này trong danh mục nguồn vốn của xã."
	}
	switch err := CheckVoucherSource(project.Allocated, id); {
	case errors.Is(err, ErrSourceNotAllocated):
		return "", "Nguồn vốn này không được phân bổ cho dự án — ghi một nguồn đã phân bổ cho dự án."
	case err != nil:
		// ErrSourceRequired cannot happen with a non-empty id; kept so a future error is not a silent pass.
		return "", "Dự án đã gắn nguồn vốn — ghi tên nguồn ở cột Nguồn vốn."
	}
	return id, ""
}

// excelEpoch is day 0 of Excel's 1900 date system. 1899-12-30 rather than -31 absorbs Excel's
// fictitious 29/02/1900 for every serial after it, which is every date KiemTraNgayChi admits.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// ParseVoucherImportDate reads "Ngày chi": a typed dd/mm/yyyy (d/m/yyyy too), or an Excel DATE cell,
// which core/xlsx hands over as its serial number ("46096", or "46096.5" when the cell carries a time,
// whose time is dropped — the column is a DATE). The year then goes through KiemTraNgayChi, the form's
// own bound, so a serial that is really a mistyped number lands outside 2000–2100 and is refused.
//
// ⚠ NOT HANDLED: a workbook saved in Excel's 1904 date system (old Mac Excel). Its serials are 1462
// days off; core/xlsx returns cells only and does not expose the workbook's date system. A typed
// dd/mm/yyyy is unaffected.
func ParseVoucherImportDate(raw string) (time.Time, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, "Thiếu ngày chi."
	}
	format := "Ngày chi phải theo dạng dd/mm/yyyy (ví dụ 15/03/2026) hoặc là ô ngày của Excel."
	var d time.Time
	if whole, frac, isSerial := strings.Cut(s, "."); allDigits(whole) && (!isSerial || allDigits(frac)) {
		if len(whole) > 6 {
			return time.Time{}, format
		}
		n, err := strconv.Atoi(whole)
		if err != nil {
			return time.Time{}, format
		}
		d = excelEpoch.AddDate(0, 0, n)
	} else {
		parts := strings.Split(s, "/")
		if len(parts) != 3 || !allDigits(parts[0]) || !allDigits(parts[1]) || !allDigits(parts[2]) ||
			len(parts[0]) > 2 || len(parts[1]) > 2 || len(parts[2]) != 4 {
			return time.Time{}, format
		}
		day, _ := strconv.Atoi(parts[0])
		month, _ := strconv.Atoi(parts[1])
		year, _ := strconv.Atoi(parts[2])
		d = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		// time.Date normalises 31/02 into March; a date that does not round-trip is not a date.
		if d.Day() != day || int(d.Month()) != month || d.Year() != year {
			return time.Time{}, "Ngày chi không có thật trên lịch (ví dụ 31/02)."
		}
	}
	if err := KiemTraNgayChi(d); err != nil {
		return time.Time{}, fmt.Sprintf("Năm của ngày chi phải trong khoảng %d–%d.", NamChungTuSom, NamChungTuMuon)
	}
	return d, ""
}

// ParseVoucherImportAmount reads "Số tiền (đồng)": a whole number of đồng, either plain digits (what an
// Excel NUMBER cell yields raw) or grouped by thousands with '.', ',' or a space, one separator kind
// per value ("250.000.000", "250,000,000"). Then KiemTraSoTien, the form's own bound.
//
// GROUPS MUST BE EXACTLY THREE DIGITS, so "1.5" and "250000.5" are REFUSED rather than read as 15 or
// 2500005 — the defect the prototype's parser has (importer.py:104 strips every '.' and ','). What this
// cannot tell apart is a NUMBER cell holding a decimal with exactly three decimals ("1.234") from the
// typed text "1.234"; it reads both as 1 234 đồng. A fraction of a đồng is not an amount anyway, and
// Vietnamese Excel stores the typed 1.234 as the number 1234.
func ParseVoucherImportAmount(raw string) (Dong, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "Thiếu số tiền."
	}
	if strings.HasPrefix(s, "-") {
		return 0, "Số tiền phải lớn hơn 0 đồng. Khoản hoàn trả không nhập bằng số âm."
	}
	format := "Số tiền phải là số nguyên đồng, ví dụ 250000000 hoặc 250.000.000."
	digits := s
	if !allDigits(s) {
		sep := ""
		for _, c := range []string{".", ",", " "} {
			if strings.Contains(s, c) {
				if sep != "" {
					return 0, format
				}
				sep = c
			}
		}
		if sep == "" {
			return 0, format
		}
		groups := strings.Split(s, sep)
		if len(groups[0]) == 0 || len(groups[0]) > 3 || !allDigits(groups[0]) {
			return 0, format
		}
		for _, g := range groups[1:] {
			if len(g) != 3 || !allDigits(g) {
				return 0, format
			}
		}
		digits = strings.Join(groups, "")
	}
	if len(digits) > 19 {
		return 0, fmt.Sprintf("Số tiền vượt mức một chứng từ giải ngân cấp xã có thể có (tối đa %d đồng).", int64(SoTienToiDa))
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Sprintf("Số tiền vượt mức một chứng từ giải ngân cấp xã có thể có (tối đa %d đồng).", int64(SoTienToiDa))
	}
	switch err := KiemTraSoTien(Dong(n)); {
	case errors.Is(err, ErrSoTienKhongDuong):
		return 0, "Số tiền phải lớn hơn 0 đồng."
	case err != nil:
		return 0, fmt.Sprintf("Số tiền vượt mức một chứng từ giải ngân cấp xã có thể có (tối đa %d đồng).", int64(SoTienToiDa))
	}
	return Dong(n), ""
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
