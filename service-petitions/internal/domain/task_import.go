package domain

// The spreadsheet import of tasks (docs/ui-ux/02-nhiem-vu.md §8) — the ROW RULES, with no I/O.
//
// THE TEMPLATE FOLLOWS vigov-require 0053854 (apps/api/app/modules/tasks/importer.py TEMPLATE_HEADERS),
// adapted by the user's decisions of 28/09/2026:
//
//	staff              named by BUSINESS CODE (`CB-…`), never by email — emails are personal data in a
//	                   file, and codes are what every record here stores
//	deadline           must carry a DATE AND A TIME; a date alone refuses the row. The software never
//	                   picks the hour a commitment falls due (rule 10)
//	document cells     each non-empty cell becomes ONE document line, stored WHOLE as its summary — no
//	                   number or date is parsed out of a sentence (a guessed reference number is a
//	                   document that does not exist), and no cell is ever dropped
//	approval marks     accepted, as on require's template; they change no status
//
// EVERY REFUSAL IS PER ROW AND PER COLUMN, AND NONE ECHOES THE CELL: a cell may hold a citizen's name
// quoted in a task title, and the error report is shown on screen and may be logged.

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// TaskImportHeadings are the template's column headings, in order. The uploaded sheet's first row
// must equal them exactly (after trimming) — a file built on another layout is refused whole rather
// than read column-by-position into the wrong fields.
var TaskImportHeadings = []string{
	"Tên nhiệm vụ",
	"Mô tả",
	"Đơn vị thực hiện (mã)",
	"Người thực hiện (mã cán bộ)",
	"Mức ưu tiên (mã)",
	"Hạn hoàn thành (ngày giờ)",
	"Loại nhiệm vụ (mã)",
	"Khối nhiệm vụ (mã)",
	"Văn bản cấp trên giao",
	"Văn bản chỉ đạo của Đảng uỷ",
	"Kết quả thực hiện / Sản phẩm đầu ra",
	"Lãnh đạo phê duyệt (x)",
	"Cấp trên đã công nhận (x)",
	"Ghi chú",
}

// Column positions, named once so the rules below read as the template reads.
const (
	importColTitle = iota
	importColDescription
	importColUnit
	importColAssignee
	importColPriority
	importColDue
	importColType
	importColBloc
	importColDocUpper
	importColDocParty
	importColDocOutput
	importColLeaderApproved
	importColSuperiorAcknowledged
	importColNote
)

// TaskImportRowCap bounds one file: 500 data rows, refused above it (never truncated). One commune's
// batch of assignments from a meeting or a directive is tens of rows; the cap keeps one request from
// holding the register's transaction open for thousands of inserts.
const TaskImportRowCap = 500

// importCodeMax bounds a code cell. Longer than any unit slug, staff code or catalogue code, short
// enough that a code cell cannot become a payload sent on to identity.
const importCodeMax = 100

// importZone is the zone a typed deadline is read in: a FIXED +07:00, for the reason muiGioChoDan
// gives (no DST in Vietnam, and no zone database the image may lack).
var importZone = time.FixedZone("ICT", 7*3600)

// importDeadlineLayouts are the accepted spellings of "date and time". Seconds optional; the date is
// day-first (Vietnamese usage) or ISO.
var importDeadlineLayouts = []string{
	"2/1/2006 15:04", "2/1/2006 15:04:05",
	"2006-01-02 15:04", "2006-01-02 15:04:05",
}

// importDateOnlyLayouts recognise a date WITHOUT a time, so the refusal can say exactly that.
var importDateOnlyLayouts = []string{"2/1/2006", "2006-01-02"}

// The per-cell refusals. The HTTP layer never sees these as errors — they travel in the row report.
var (
	ErrImportDeadlineNeedsTime  = errors.New("hạn hoàn thành phải có cả ngày và giờ (ví dụ 30/09/2026 17:00)")
	ErrImportDeadlineUnreadable = errors.New(
		"không đọc được hạn hoàn thành — ghi theo dạng ngày/tháng/năm giờ:phút, ví dụ 30/09/2026 17:00")
	errImportUnitOrAssignee = "phải có đơn vị thực hiện hoặc người thực hiện"
	errImportMarkUnreadable = "chỉ ghi \"x\" (hoặc có) khi đã duyệt, để trống khi chưa"
	errImportCodeTooLong    = fmt.Sprintf("mã quá dài (tối đa %d ký tự)", importCodeMax)
)

// TaskImportError is one refusal in the row report: the spreadsheet row number (1 is the heading
// row), the column heading, and a sentence. NEVER the cell's value.
type TaskImportError struct {
	Row     int
	Column  string
	Message string
}

// TaskImportRow is one row as the rules leave it: trimmed, bounded, deadline parsed. Codes are NOT
// yet checked against identity or the catalogues — that is the use case's, batched over the file.
type TaskImportRow struct {
	Row int

	Title, Description, Note string
	UnitCode                 string
	AssigneeCode             string
	PriorityCode, TypeCode   string
	BlocCode                 string

	// Due is the zero time for "no deadline" (§7.1 does not require one).
	Due time.Time

	// Documents are the three cells as document lines, already validated by the create path's own
	// rule (KiemDanhSachVanBanNhiemVu).
	Documents []VanBanNhiemVuVao

	LeaderApproved, SuperiorAcknowledged bool
}

// TaskImportRetiredHeadings are the two columns the template carried until ADR 0065 NV5 (user decision
// 30/09/2026) made "cơ quan chủ trì" the unit and "chuyên viên theo dõi" the assignee. A file that
// still carries either is a file filled on the OLD template: CheckTaskImportHeadings would refuse it
// anyway, but as "wrong layout" — which tells the clerk nothing. TaskImportCarriesRetiredColumns lets
// the refusal say exactly why. The file is refused WHOLE, never read with those columns skipped: the
// clerk put a person there believing they would monitor the task, and silently dropping that name
// would record an assignment nobody made.
var TaskImportRetiredHeadings = []string{
	"Cơ quan chủ trì tham mưu (mã)",
	"Chuyên viên theo dõi (mã cán bộ)",
}

// TaskImportCarriesRetiredColumns reports whether a heading row names either retired column, at any
// position.
func TaskImportCarriesRetiredColumns(cells []string) bool {
	for _, c := range cells {
		for _, h := range TaskImportRetiredHeadings {
			if strings.TrimSpace(c) == h {
				return true
			}
		}
	}
	return false
}

// CheckTaskImportHeadings refuses a heading row that is not the template's.
func CheckTaskImportHeadings(cells []string) bool {
	if len(cells) < len(TaskImportHeadings) {
		return false
	}
	for i, h := range TaskImportHeadings {
		if strings.TrimSpace(cells[i]) != h {
			return false
		}
	}
	for _, extra := range cells[len(TaskImportHeadings):] {
		if strings.TrimSpace(extra) != "" {
			return false
		}
	}
	return true
}

// TaskImportRowBlank reports whether a data row has nothing in it — skipped, as require skips it.
func TaskImportRowBlank(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// ParseTaskImportRow applies every rule that needs no lookup. The deadline cell arrives as text; the
// reader has already turned a spreadsheet date cell into this same text shape.
func ParseTaskImportRow(row int, raw []string) (TaskImportRow, []TaskImportError) {
	cells := make([]string, len(TaskImportHeadings))
	for i := range cells {
		if i < len(raw) {
			cells[i] = strings.TrimSpace(raw[i])
		}
	}
	out := TaskImportRow{Row: row}
	var errs []TaskImportError
	fail := func(col int, msg string) {
		errs = append(errs, TaskImportError{Row: row, Column: TaskImportHeadings[col], Message: msg})
	}
	failErr := func(col int, err error) { fail(col, stripTaskPrefix(err)) }

	if t, err := KiemTieuDeNhiemVu(cells[importColTitle]); err != nil {
		failErr(importColTitle, err)
	} else {
		out.Title = t
	}
	if d, err := KiemVanBanTuyChon(cells[importColDescription], MoTaNhiemVuToiDa, ErrMoTaNhiemVuQuaDai); err != nil {
		failErr(importColDescription, err)
	} else {
		out.Description = d
	}
	if n, err := KiemVanBanTuyChon(cells[importColNote], GhiChuNhiemVuToiDa, ErrGhiChuNhiemVuQuaDai); err != nil {
		failErr(importColNote, err)
	} else {
		out.Note = n
	}

	code := func(col int, dst *string) {
		v := cells[col]
		if len([]rune(v)) > importCodeMax {
			fail(col, errImportCodeTooLong)
			return
		}
		*dst = v
	}
	code(importColUnit, &out.UnitCode)
	code(importColAssignee, &out.AssigneeCode)
	code(importColPriority, &out.PriorityCode)
	code(importColType, &out.TypeCode)
	code(importColBloc, &out.BlocCode)
	if cells[importColUnit] == "" && cells[importColAssignee] == "" {
		fail(importColUnit, errImportUnitOrAssignee)
	}

	if due, err := ParseImportDeadline(cells[importColDue]); err != nil {
		failErr(importColDue, err)
	} else {
		out.Due = due
	}

	for _, d := range []struct {
		col  int
		nhom NhomVanBanNhiemVu
	}{
		{importColDocUpper, VanBanCapTrenGiao},
		{importColDocParty, VanBanChiDaoDangUy},
		{importColDocOutput, VanBanSanPhamRa},
	} {
		if cells[d.col] == "" {
			continue
		}
		// THE SAME RULE CREATE APPLIES TO A LINE, one cell at a time so a refusal names its column.
		if _, err := KiemDanhSachVanBanNhiemVu([]VanBanNhiemVuVao{{Nhom: d.nhom, TrichYeu: cells[d.col]}}); err != nil {
			failErr(d.col, err)
			continue
		}
		out.Documents = append(out.Documents, VanBanNhiemVuVao{Nhom: d.nhom, TrichYeu: cells[d.col]})
	}

	mark := func(col int, dst *bool) {
		switch strings.ToLower(cells[col]) {
		case "":
		case "x", "có", "co", "đã duyệt", "da duyet", "true", "1":
			*dst = true
		default:
			fail(col, errImportMarkUnreadable)
		}
	}
	mark(importColLeaderApproved, &out.LeaderApproved)
	mark(importColSuperiorAcknowledged, &out.SuperiorAcknowledged)

	return out, errs
}

// ParseImportDeadline reads a typed deadline in Vietnam time. "" is "no deadline". A date with no time
// is refused (ErrImportDeadlineNeedsTime) — the user's decision: the file must say the hour.
func ParseImportDeadline(s string) (time.Time, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range importDeadlineLayouts {
		if t, err := time.ParseInLocation(layout, s, importZone); err == nil {
			return t.UTC(), nil
		}
	}
	for _, layout := range importDateOnlyLayouts {
		if _, err := time.ParseInLocation(layout, s, importZone); err == nil {
			return time.Time{}, ErrImportDeadlineNeedsTime
		}
	}
	return time.Time{}, ErrImportDeadlineUnreadable
}

// stripTaskPrefix turns a domain sentinel ("nhiệm vụ: tiêu đề quá dài…") into the bare sentence the
// row report shows under a column heading.
func stripTaskPrefix(err error) string {
	return strings.TrimPrefix(err.Error(), "nhiệm vụ: ")
}
