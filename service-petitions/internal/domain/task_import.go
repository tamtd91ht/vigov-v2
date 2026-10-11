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

// TaskImportHeadings are the template's column headings, in order. The uploaded sheet's heading row
// (the first, or below title rows — LocateTaskImportHeadings) must carry them in this order, compared
// in NormaliseImportText form — a file built on another layout is refused whole rather than read
// column-by-position into the wrong fields.
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
	errImportMarkUnreadable = "chỉ ghi \"x\" (hoặc có) khi đã duyệt, để trống (hoặc không) khi chưa"
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
			if NormaliseImportText(c) == NormaliseImportText(h) {
				return true
			}
		}
	}
	return false
}

// importToneReplacer maps the "new" tone placement onto the "old" one for the syllables where both are
// in everyday use (hoà/hòa, uỷ/ủy, thuý/thúy). Both spellings are correct Vietnamese and look the same
// to the clerk: the template writes "Đảng uỷ", a keyboard set the other way types "Đảng ủy".
var importToneReplacer = strings.NewReplacer(
	"oà", "òa", "oá", "óa", "oả", "ỏa", "oã", "õa", "oạ", "ọa",
	"oè", "òe", "oé", "óe", "oẻ", "ỏe", "oẽ", "õe", "oẹ", "ọe",
	"uỳ", "ùy", "uý", "úy", "uỷ", "ủy", "uỹ", "ũy", "uỵ", "ụy",
)

// NormaliseImportText is the form two pieces of spreadsheet text are COMPARED in — a heading against
// the template's, a typed catalogue label against the catalogue's. Never stored. Letter case, runs of
// whitespace (a line break typed with Alt+Enter, a non-breaking space pasted from Word) and tone
// placement are all invisible differences, and refusing a file over one of them refuses data that is
// valid (bug sheet row 19).
//
// NOT NORMALISED: Unicode decomposition (NFD). A decomposed "ê" needs golang.org/x/text, which this
// package may not import (domain is standard library only); Excel on Windows writes precomposed text.
func NormaliseImportText(s string) string {
	return importToneReplacer.Replace(strings.ToLower(strings.Join(strings.Fields(s), " ")))
}

// CheckTaskImportHeadings refuses a heading row that is not the template's. Compared after
// NormaliseImportText: the ORDER and the WORDS must be the template's, the spelling details need not.
func CheckTaskImportHeadings(cells []string) bool {
	_, ok := taskImportHeadingMismatch(cells)
	return ok
}

// taskImportHeadingMismatch returns the first column whose heading is not the template's (a column
// beyond the template counts when it holds anything), or ok when the row is the template's.
func taskImportHeadingMismatch(cells []string) (col int, ok bool) {
	for i, h := range TaskImportHeadings {
		if i >= len(cells) || NormaliseImportText(cells[i]) != NormaliseImportText(h) {
			return i, false
		}
	}
	for i, extra := range cells[len(TaskImportHeadings):] {
		if strings.TrimSpace(extra) != "" {
			return len(TaskImportHeadings) + i, false
		}
	}
	return 0, true
}

// TaskImportHeadingSearchRows is how far down the heading row may sit. Clerks put the commune's name and
// a title above a list (often merged across the sheet); ten rows covers the national-standard heading
// block of an administrative document with room to spare. Further down, the rows above could be tasks
// typed above a pasted heading, and reading them as titles would drop them — so that file is refused.
const TaskImportHeadingSearchRows = 10

// LocateTaskImportHeadings returns the index in `sheet` of the template's heading row within the first
// TaskImportHeadingSearchRows rows. Rows above it are titles and are never read as tasks.
func LocateTaskImportHeadings(sheet [][]string) (int, bool) {
	for i := 0; i < len(sheet) && i < TaskImportHeadingSearchRows; i++ {
		if CheckTaskImportHeadings(sheet[i]) {
			return i, true
		}
	}
	return 0, false
}

// TaskImportLayoutProblem is why a sheet's heading row was not found: the best candidate row (the one
// matching the most template headings) and its first differing column. Row is 0 when no row in the
// search window looks like the heading at all.
type TaskImportLayoutProblem struct {
	Row    int // 1-based sheet row
	Column int // 0-based column
}

// DiagnoseTaskImportHeadings explains a sheet LocateTaskImportHeadings refused.
func DiagnoseTaskImportHeadings(sheet [][]string) TaskImportLayoutProblem {
	best, bestHits := TaskImportLayoutProblem{}, 0
	for i := 0; i < len(sheet) && i < TaskImportHeadingSearchRows; i++ {
		hits := 0
		for c, h := range TaskImportHeadings {
			if c < len(sheet[i]) && NormaliseImportText(sheet[i][c]) == NormaliseImportText(h) {
				hits++
			}
		}
		// Half the template's headings in place: that row was MEANT as the heading row. Fewer is a
		// title or a data row, and naming one of its cells would point the clerk at the wrong row.
		if hits > bestHits && hits*2 >= len(TaskImportHeadings) {
			col, _ := taskImportHeadingMismatch(sheet[i])
			best, bestHits = TaskImportLayoutProblem{Row: i + 1, Column: col}, hits
		}
	}
	return best
}

// Sentence is the refusal shown to the clerk. It names the cell as Excel does ("F2") and quotes only
// the TEMPLATE's own heading, never the cell's content (rule 3).
func (p TaskImportLayoutProblem) Sentence() string {
	if p.Row == 0 {
		return fmt.Sprintf("không tìm thấy hàng tiêu đề của mẫu trong %d dòng đầu (hàng có ô “%s” ở cột A).",
			TaskImportHeadingSearchRows, TaskImportHeadings[0])
	}
	cell := fmt.Sprintf("%s%d", importColumnLetter(p.Column), p.Row)
	if p.Column >= len(TaskImportHeadings) {
		return fmt.Sprintf("ô %s nằm ngoài mẫu — hàng tiêu đề chỉ có %d cột, từ A đến %s; hãy xoá cột thừa.",
			cell, len(TaskImportHeadings), importColumnLetter(len(TaskImportHeadings)-1))
	}
	return fmt.Sprintf("ô %s phải là tiêu đề “%s”.", cell, TaskImportHeadings[p.Column])
}

// importColumnLetter is Excel's name for a 0-based column: 0 → A, 25 → Z, 26 → AA.
func importColumnLetter(col int) string {
	name := ""
	for col++; col > 0; col = (col - 1) / 26 {
		name = string(rune('A'+(col-1)%26)) + name
	}
	return name
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

	code := func(col int, dst *string, fold bool) {
		v := cells[col]
		if len([]rune(v)) > importCodeMax {
			fail(col, errImportCodeTooLong)
			return
		}
		if fold {
			v = strings.ToLower(v)
		}
		*dst = v
	}
	// FOLDED: unit, priority, type and bloc codes are lower-case ONLY by their own validators (identity
	// ChuanHoaMaBoPhan and ChuanHoaMaDanhMuc, this service's ChuanHoaMa), so "Dia-Chinh" can name no
	// record but "dia-chinh" — folding a LOOKUP key reaches no other row. The stored value is the
	// catalogue's own code either way. NOT FOLDED: staff codes, whose form identity does not pin to
	// one case.
	code(importColUnit, &out.UnitCode, true)
	code(importColAssignee, &out.AssigneeCode, false)
	code(importColPriority, &out.PriorityCode, true)
	code(importColType, &out.TypeCode, true)
	code(importColBloc, &out.BlocCode, true)
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
		// The explicit "no" spellings mean what an empty cell means. "0" is also how a FALSE boolean cell
		// arrives from Excel (the reader takes raw values).
		case "", "0", "false", "không", "khong", "chưa", "chua":
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
//
// The cell is first brought to one spelling (canonicalImportDeadline), so the layouts below stay the
// only definition of what a valid date and time are — 31/02 and 25:00 are refused by time.Parse, never
// by a hand-written range check.
func ParseImportDeadline(s string) (time.Time, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return time.Time{}, nil
	}
	date, clock, ok := canonicalImportDeadline(s)
	if !ok {
		return time.Time{}, ErrImportDeadlineUnreadable
	}
	if clock == "" {
		for _, layout := range importDateOnlyLayouts {
			if _, err := time.ParseInLocation(layout, date, importZone); err == nil {
				return time.Time{}, ErrImportDeadlineNeedsTime
			}
		}
		return time.Time{}, ErrImportDeadlineUnreadable
	}
	for _, layout := range importDeadlineLayouts {
		if t, err := time.ParseInLocation(layout, date+" "+clock, importZone); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, ErrImportDeadlineUnreadable
}

// canonicalImportDeadline splits a deadline cell into a date ("d/m/yyyy" or "yyyy-mm-dd") and a clock
// ("H:MM[:SS]", "" when absent), accepting the spellings Vietnamese offices write (bug sheet row 19):
//
//	either order           "17:00 30/09/2026" as well as "30/09/2026 17:00"
//	17h00 / 17h            the "h" spelling of a clock
//	30-09-2026, 30.09.2026 other separators of a day-first date
//	2026-09-30T17:00       ISO with its "T"
//	ngày / lúc             the connecting words, ignored
//
// A day-first date needs a FOUR-digit year: "30/09/26" leaves the century to a guess. Anything else —
// two dates, two clocks, a stray word — is not a deadline, and ok is false.
func canonicalImportDeadline(s string) (date, clock string, ok bool) {
	if i := strings.IndexByte(s, 'T'); i > 0 && i < len(s)-1 && isDigit(s[i-1]) && isDigit(s[i+1]) {
		s = s[:i] + " " + s[i+1:]
	}
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		switch tok {
		case "ngày", "ngay", "lúc", "luc":
			continue
		}
		if d, isDate := importDateToken(tok); isDate {
			if date != "" {
				return "", "", false
			}
			date = d
			continue
		}
		if c, isClock := importClockToken(tok); isClock {
			if clock != "" {
				return "", "", false
			}
			clock = c
			continue
		}
		return "", "", false
	}
	return date, clock, date != ""
}

// importDateToken reads "30/09/2026", "30-09-2026", "30.09.2026" (day first) or "2026-09-30".
func importDateToken(tok string) (string, bool) {
	sep := strings.IndexAny(tok, "/-.")
	if sep < 0 {
		return "", false
	}
	parts := strings.Split(tok, tok[sep:sep+1])
	if len(parts) != 3 || !allDigits(parts[0]) || !allDigits(parts[1]) || !allDigits(parts[2]) {
		return "", false
	}
	if len(parts[0]) == 4 {
		return parts[0] + "-" + parts[1] + "-" + parts[2], true
	}
	if len(parts[2]) != 4 {
		return "", false
	}
	return parts[0] + "/" + parts[1] + "/" + parts[2], true
}

// importClockToken reads "17:00", "17:00:00", "17h00", "17h".
func importClockToken(tok string) (string, bool) {
	if h, m, found := strings.Cut(tok, "h"); found && allDigits(h) && (m == "" || allDigits(m)) {
		if m == "" {
			m = "00"
		}
		return h + ":" + m, true
	}
	parts := strings.Split(tok, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return "", false
	}
	for _, p := range parts {
		if !allDigits(p) {
			return "", false
		}
	}
	return tok, true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// stripTaskPrefix turns a domain sentinel ("nhiệm vụ: tiêu đề quá dài…") into the bare sentence the
// row report shows under a column heading.
func stripTaskPrefix(err error) string {
	return strings.TrimPrefix(err.Error(), "nhiệm vụ: ")
}
