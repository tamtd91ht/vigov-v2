package http

// The spreadsheet import of tasks (docs/ui-ux/02-nhiem-vu.md §8):
//
//	POST /api/v1/tasks/imports          multipart `file`, optional `dry_run=true`   task.create
//	GET  /api/v1/tasks/import-template  the empty template: one example row, dropdowns    task.create
//	                                    (optional repeated `org_unit`, `staff`, `bloc` codes)
//
// THIS FILE DECODES THE UPLOAD, AND THE UPLOAD IS HOSTILE UNTIL PROVEN OTHERWISE. An .xlsx is a zip of
// XML parts; every limit below is one this service controls, applied BEFORE excelize parses a byte of
// XML:
//
//	request body   taskImportMaxBytes (2 MB) — http.MaxBytesReader; above it 413
//	zip entries    taskImportMaxZipEntries — a real workbook has a dozen parts; a zip of thousands is
//	               a resource attack, refused from the central directory without inflating anything
//	unzipped size  taskImportMaxUnzipped (20 MB) — the DECLARED sizes summed first, and excelize's own
//	               UnzipSizeLimit / UnzipXMLSizeLimit set to the same bound, so the declared sizes
//	               cannot lie their way past it and nothing spills to a temp directory
//	rows read      domain.TaskImportRowCap non-blank rows (+1 to detect overflow), and at most
//	               taskImportMaxScannedRows rows scanned in total, blank ones included
//
// ⚠ encoding/xml's missing recursion-depth guard (GO-2026-6088) is fixed only in go1.26.6: the size
// bounds above limit how deep a hostile document can nest, but the real fix is the toolchain. Reported
// with P11; not decided here.

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

const (
	taskImportMaxBytes       = 2 << 20
	taskImportMaxZipEntries  = 64
	taskImportMaxUnzipped    = 20 << 20
	taskImportMaxScannedRows = 5000
	taskImportTemplateName   = "mau-nhap-nhiem-vu.xlsx"
)

// errImportUnreadable — the upload is not a workbook this reader will open (not a zip, too many
// parts, too large unzipped, encrypted, no sheet). One sentence for all: telling a hostile file which
// bound it tripped helps nobody legitimate.
var errImportUnreadable = errors.New("tệp không phải .xlsx đọc được")

// taskImportErrorOut is one refusal of the row report.
type taskImportErrorOut struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// taskImportResultOut is the report. `committed` false means NOTHING was written — a dry run, or at
// least one refused row. `codes` are the issued register numbers, in row order, when committed.
type taskImportResultOut struct {
	TotalRows int                  `json:"total_rows"`
	Created   int                  `json:"created"`
	Committed bool                 `json:"committed"`
	Errors    []taskImportErrorOut `json:"errors"`
	Codes     []string             `json:"codes"`
}

// ImportTasks serves POST /api/v1/tasks/imports.
func (h *Handler) ImportTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dryRun, err := importDryRunFromQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("nhập nhiệm vụ: chủ thể không có mã cán bộ — SAI CẤU HÌNH ROUTE", "xa", string(tenant.MustFrom(ctx)))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	file, status, code, msg := readImportUpload(w, r)
	if status != 0 {
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	sheet, err := readImportSheet(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "import_unreadable",
			"Tệp không phải bảng tính .xlsx đọc được. Hãy tải mẫu, điền vào và gửi lại.", "")
		return
	}

	res, err := h.d.TaskImport.Import(ctx, sheet, dryRun, actor)
	switch {
	case errors.Is(err, app.ErrTaskImportRetiredColumns):
		// ADR 0065 NV5: the file was filled on the template from before 30/09/2026. Refused whole — the
		// retired columns are never skipped silently (app.ErrTaskImportRetiredColumns says why).
		httpx.WriteError(w, http.StatusBadRequest, "import_retired_columns",
			"Tệp theo mẫu cũ: còn cột \"Cơ quan chủ trì tham mưu\" hoặc \"Chuyên viên theo dõi\". Hai vai này đã gộp vào \"Đơn vị thực hiện\" và \"Người thực hiện\" — hãy tải mẫu mới, điền lại rồi gửi.", "")
		return
	case errors.Is(err, app.ErrTaskImportLayout):
		// The use case's sentence names the cell that differs ("ô F2 phải là tiêu đề …"); it quotes the
		// template only, never the file (app.TaskImportLayoutError).
		msg := "Tệp không đúng mẫu nhập nhiệm vụ (hàng tiêu đề khác mẫu, hoặc không có dòng nhiệm vụ nào). Hãy tải mẫu và điền lại."
		var le *app.TaskImportLayoutError
		if errors.As(err, &le) && le.Sentence != "" {
			msg = "Tệp không đúng mẫu nhập nhiệm vụ: " + le.Sentence + " Hãy tải mẫu và điền lại."
		}
		httpx.WriteError(w, http.StatusBadRequest, "import_layout", msg, "")
		return
	case errors.Is(err, app.ErrTaskImportTooManyRows):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "import_too_many_rows",
			"Tệp có quá 500 dòng nhiệm vụ. Hãy chia thành nhiều tệp rồi nhập lần lượt.", "")
		return
	case errors.Is(err, app.ErrTaskImportUnchecked):
		h.d.Log.Warn("CẢNH BÁO: không nhập được tệp nhiệm vụ vì chưa kiểm được với identity",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "import_unchecked",
			"Chưa kiểm tra được bộ phận hoặc cán bộ trong tệp nên CHƯA nhập nhiệm vụ nào. Vui lòng thử lại sau ít phút.", "")
		return
	case err != nil:
		h.d.Log.Error("nhập nhiệm vụ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	out := taskImportResultOut{
		TotalRows: res.TotalRows, Created: res.Created, Committed: res.Committed,
		Errors: make([]taskImportErrorOut, 0, len(res.Errors)), Codes: make([]string, 0, len(res.Codes)),
	}
	for _, e := range res.Errors {
		out.Errors = append(out.Errors, taskImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	out.Codes = append(out.Codes, res.Codes...)

	statusOut := http.StatusOK // a dry run, or a refused file: nothing was written
	if res.Committed {
		statusOut = http.StatusCreated
		// THE FIRST ISSUED NUMBER is what a retry of the same Idempotency-Key is told (idem stores a
		// code, never a body); the whole range is in the batch audit entry.
		idem.RecordCode(ctx, res.Codes[0])
	}
	vietJSON(w, statusOut, out)
}

// importDryRunFromQuery reads `dry_run`. `true` is the only accepted spelling, like `late`.
func importDryRunFromQuery(q map[string][]string) (bool, error) {
	if v := q["dry_run"]; len(v) > 0 {
		if len(v) != 1 || v[0] != "true" {
			return false, errors.New("`dry_run` chỉ nhận giá trị `true`; bỏ hẳn tham số để nhập thật")
		}
		return true, nil
	}
	return false, nil
}

// readImportUpload takes the multipart `file` field, bounded. A non-zero status is the refusal.
func readImportUpload(w http.ResponseWriter, r *http.Request) ([]byte, int, string, string) {
	// The whole body, multipart envelope included, is bounded a little above the file cap.
	r.Body = http.MaxBytesReader(w, r.Body, taskImportMaxBytes+64<<10)
	if err := r.ParseMultipartForm(taskImportMaxBytes + 64<<10); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, http.StatusRequestEntityTooLarge, "import_too_large", "Tệp vượt 2 MB. Hãy chia thành nhiều tệp nhỏ hơn."
		}
		return nil, http.StatusBadRequest, "invalid_request", "Yêu cầu phải gửi tệp trong trường `file` (multipart/form-data)."
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, http.StatusBadRequest, "invalid_request", "Yêu cầu phải gửi tệp trong trường `file` (multipart/form-data)."
	}
	defer f.Close()
	// THE FILE NAME IS NOT READ: it may carry personal data (rule 3, forbidden #4) and decides nothing.
	b, err := io.ReadAll(io.LimitReader(f, taskImportMaxBytes+1))
	if err != nil {
		return nil, http.StatusBadRequest, "invalid_request", "Không đọc được tệp gửi lên."
	}
	if len(b) > taskImportMaxBytes {
		return nil, http.StatusRequestEntityTooLarge, "import_too_large", "Tệp vượt 2 MB. Hãy chia thành nhiều tệp nhỏ hơn."
	}
	return b, 0, "", ""
}

// readImportSheet opens the workbook under the bounds in the header and returns its FIRST sheet as
// text, heading row first. The deadline column's date cells are turned into the domain's text shape.
func readImportSheet(b []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, errImportUnreadable
	}
	if len(zr.File) > taskImportMaxZipEntries {
		return nil, errImportUnreadable
	}
	var declared uint64
	for _, zf := range zr.File {
		declared += zf.UncompressedSize64
		if declared > taskImportMaxUnzipped {
			return nil, errImportUnreadable
		}
	}

	f, err := excelize.OpenReader(bytes.NewReader(b), excelize.Options{
		UnzipSizeLimit:    taskImportMaxUnzipped,
		UnzipXMLSizeLimit: taskImportMaxUnzipped,
	})
	if err != nil {
		return nil, errImportUnreadable
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errImportUnreadable
	}
	rows, err := f.Rows(sheets[0])
	if err != nil {
		return nil, errImportUnreadable
	}
	defer rows.Close()

	// A workbook saved in the 1904 date system (Excel for Mac's old default) counts serials from another
	// day: read as 1900, every deadline would land four years and a day early — silently.
	date1904 := false
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		date1904 = *props.Date1904
	}

	var (
		out      [][]string
		headAt   = -1 // index in out of the heading row, once seen
		dataRows int
		// wholeDue are the data rows whose deadline is a WHOLE serial: a date alone, or a date-time at
		// exactly 00:00. Only the cell's number format tells them apart, and it is read after the
		// iterator is closed (dueCellShowsClock).
		wholeDue []int
	)
	for rows.Next() {
		if len(out) >= taskImportMaxScannedRows {
			return nil, errImportUnreadable
		}
		cells, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, errImportUnreadable
		}
		if len(cells) > len(domain.TaskImportHeadings)+8 {
			cells = cells[:len(domain.TaskImportHeadings)+8] // beyond the template: never read
		}
		if headAt < 0 {
			// Title rows above the heading pass through untouched and are NOT counted against the cap: a
			// title row counted there would let the cap stop the read before the 500th task, and the
			// use case would never see the rows past it. Past the search window there is no heading to
			// find; the use case refuses the file from the rows read so far.
			if domain.CheckTaskImportHeadings(cells) || domain.TaskImportCarriesRetiredColumns(cells) {
				headAt = len(out)
			} else if len(out) >= domain.TaskImportHeadingSearchRows {
				break
			}
			out = append(out, cells)
			continue
		}
		if domain.TaskImportRowBlank(cells) {
			out = append(out, cells)
			continue
		}
		dataRows++
		if dataRows > domain.TaskImportRowCap {
			// One past the cap is enough for the use case to refuse; the rest is never parsed.
			out = append(out, cells)
			break
		}
		if len(cells) > taskImportDueColumn {
			raw := cells[taskImportDueColumn]
			if serial, err := strconv.ParseFloat(raw, 64); err == nil && serial == math.Trunc(serial) {
				wholeDue = append(wholeDue, len(out))
			} else {
				cells[taskImportDueColumn] = importDueCellText(raw, false, date1904)
			}
		}
		out = append(out, cells)
	}
	if err := rows.Error(); err != nil {
		return nil, errImportUnreadable
	}
	_ = rows.Close() // done with the stream before the styles are read from the same file
	for _, i := range wholeDue {
		// out[i] is sheet row i+1: excelize yields every row from the first, gaps included.
		out[i][taskImportDueColumn] = importDueCellText(out[i][taskImportDueColumn],
			dueCellShowsClock(f, sheets[0], i+1), date1904)
	}
	return out, nil
}

// dueCellShowsClock reports whether the deadline cell of sheet row `row` is FORMATTED with a time —
// what tells "30/09/2026 00:00" typed as a date-time from "30/09/2026" typed as a date, both stored as
// the same whole serial. Unreadable style = no clock, which leaves the cell date-only and refused: the
// safe side, never an invented hour.
func dueCellShowsClock(f *excelize.File, sheet string, row int) bool {
	cell, err := excelize.CoordinatesToCellName(taskImportDueColumn+1, row)
	if err != nil {
		return false
	}
	id, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return false
	}
	st, err := f.GetStyle(id)
	if err != nil || st == nil {
		return false
	}
	if st.CustomNumFmt != nil {
		return numFmtShowsClock(*st.CustomNumFmt)
	}
	switch st.NumFmt {
	case 18, 19, 20, 21, 22, 45, 46, 47: // ECMA-376 §18.8.30's built-in formats that show hours
		return true
	}
	return false
}

// numFmtShowsClock reports whether a custom number format displays hours: an `h` outside quoted
// literals, `[...]` locale/colour sections and backslash escapes ("[$-vi-VN]dd/mm/yyyy\ h:mm;@").
func numFmtShowsClock(code string) bool {
	quoted, bracket := false, false
	for i := 0; i < len(code); i++ {
		switch c := code[i]; {
		case quoted:
			quoted = c != '"'
		case bracket:
			bracket = c != ']'
		case c == '"':
			quoted = true
		case c == '[':
			bracket = true
		case c == '\\':
			i++
		case c == 'h' || c == 'H':
			return true
		}
	}
	return false
}

// taskImportDueColumn is the template's "Hạn hoàn thành (ngày giờ)" column (domain.TaskImportHeadings).
const taskImportDueColumn = 5

// importDueCellText turns a spreadsheet DATE cell — which arrives raw as a serial number such as
// 46295.708333 — into the text the domain parses. A whole serial is a date WITHOUT a time unless the
// cell's format shows a clock (`clock`, from dueCellShowsClock): date-only is rendered date-only, so
// the domain refuses it as the user decided. A serial below 1 is a time with no date and passes
// through raw, which the domain refuses as unreadable — never "1899-12-30 17:00". A text cell passes
// through.
func importDueCellText(raw string, clock, date1904 bool) string {
	serial, err := strconv.ParseFloat(raw, 64)
	if err != nil || serial < 1 || serial > 2958465 { // 2958465 = 9999-12-31, Excel's own ceiling
		return raw
	}
	t, err := excelize.ExcelDateToTime(serial, date1904)
	if err != nil {
		return raw
	}
	if serial == math.Trunc(serial) && !clock {
		return t.Format("2006-01-02")
	}
	// The serial is WALL-CLOCK time with no zone; the domain reads it in Vietnam time. Rounded to the
	// minute: a float serial of 17:00 comes back as 16:59:59.999.
	return t.Round(time.Minute).Format("2006-01-02 15:04")
}

// The query parameters of GET /api/v1/tasks/import-template naming the codes the client wants offered
// in three dropdowns, each repeated once per code (`?org_unit=van-phong&org_unit=dia-chinh`). The
// owner's decision of 14/10/2026: the screen that offers the download already holds these lists.
const (
	choiceParamOrgUnit = "org_unit" // org-unit business codes (`bo_phan.ma`)
	choiceParamStaff   = "staff"    // staff business codes (`CB-…`)
	choiceParamBloc    = "bloc"     // task-bloc codes (`khoi_nhiem_vu.ma`)
)

// taskImportChoiceCap bounds each list a client may send. One commune's units, staff and blocs are tens
// to a few hundred; 2000 is far above any of them and still bounds the identity calls one GET can cause
// (2000 staff codes are 40 assignability calls of 50). Above it the request is refused, never truncated:
// a list silently cut short would offer some staff and not others with nothing saying why.
const taskImportChoiceCap = 2000

// taskImportChoiceCodeMax drops a query value longer than any code — the import's own bound on a code
// cell (domain importCodeMax), so a value that could never be imported is never sent on to identity.
const taskImportChoiceCodeMax = 100

// ImportTemplate serves GET /api/v1/tasks/import-template.
//
// THE CLIENT'S LISTS ARE NOT TRUSTED (rule 1, forbidden #2). They only say WHICH codes to look up: the
// file offers a code only when identity confirms it for the commune the Host resolved — a live unit, an
// assignable member of staff, a live bloc — and prints identity's name for it, never text the client
// sent. A code of another commune is absent from identity's answer and so absent from the file, with
// no sign it was asked for. The dropdowns grant nothing either way: the import re-checks every code.
func (h *Handler) ImportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	asked := make(map[string][]string, 3)
	for _, name := range []string{choiceParamOrgUnit, choiceParamStaff, choiceParamBloc} {
		codes, ok := importChoiceCodesFromQuery(q[name])
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "too_many_choices", fmt.Sprintf(
				"Mỗi danh sách chọn của mẫu nhận tối đa %d mã (tham số `%s`).", taskImportChoiceCap, name), "")
			return
		}
		asked[name] = codes
	}

	choices := h.importTemplateChoices(ctx, asked[choiceParamOrgUnit], asked[choiceParamStaff], asked[choiceParamBloc])
	b, err := renderImportTemplate(choices)
	if err != nil {
		h.d.Log.Error("mẫu nhập nhiệm vụ: lỗi dựng tệp", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+taskImportTemplateName+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

// importChoiceCodesFromQuery trims, drops blanks and over-long values, and removes duplicates, keeping
// the client's order (the order its screen shows). ok is false when more than taskImportChoiceCap values
// were sent — counted before anything is dropped, so the bound is on what the request carried.
func importChoiceCodesFromQuery(raw []string) ([]string, bool) {
	if len(raw) > taskImportChoiceCap {
		return nil, false
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" || utf8.RuneCountInString(v) > taskImportChoiceCodeMax {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out, true
}

// importTemplateChoices builds the dropdowns. A list that comes out empty gives its column no dropdown.
//
// NEVER FAILS THE DOWNLOAD: a dropdown is a convenience, and the template without it is the template
// every commune used until 14/10/2026. A catalogue that cannot be read drops its own dropdown; identity
// unreachable (or not wired) drops all three of its dropdowns together — never a list built from half
// the answers. The warnings name the commune and the error only: no code list, no name (rule 3).
func (h *Handler) importTemplateChoices(ctx context.Context, units, staff, blocs []string) map[int]xlsx.Choices {
	xa := string(tenant.MustFrom(ctx))
	out := make(map[int]xlsx.Choices, 7)

	// This service's own two catalogues, IN-USE rows only — what the import accepts.
	if types, err := h.d.LoaiNhiemVu.DanhSach(ctx); err != nil {
		h.d.Log.Warn("mẫu nhập nhiệm vụ: không đọc được danh mục loại nhiệm vụ — mẫu không có danh sách chọn cột này",
			"xa", xa, "err", err)
	} else {
		var values []string
		for _, t := range types {
			if t.DangDung {
				values = append(values, xlsx.ChoiceValue(t.Ma, t.Nhan))
			}
		}
		out[domain.TaskImportColType] = xlsx.Choices{Values: values, Title: "Loại nhiệm vụ",
			Message: "Giá trị này không có trong danh mục loại nhiệm vụ đang dùng của xã. Khi nhập, hệ thống sẽ kiểm lại."}
	}
	if levels, err := h.d.MucUuTien.DanhSach(ctx); err != nil {
		h.d.Log.Warn("mẫu nhập nhiệm vụ: không đọc được danh mục mức ưu tiên — mẫu không có danh sách chọn cột này",
			"xa", xa, "err", err)
	} else {
		var values []string
		for _, l := range levels {
			if l.DangDung {
				values = append(values, xlsx.ChoiceValue(l.Ma, l.Nhan))
			}
		}
		out[domain.TaskImportColPriority] = xlsx.Choices{Values: values, Title: "Mức ưu tiên",
			Message: "Giá trị này không có trong danh mục mức ưu tiên đang dùng của xã. Khi nhập, hệ thống sẽ kiểm lại."}
	}

	mark := xlsx.Choices{Values: []string{"x"}, Strict: true, Title: "Chỉ ghi \"x\"",
		Message: "Ghi \"x\" khi đã duyệt, để trống khi chưa."}
	out[domain.TaskImportColLeaderApproved] = mark
	out[domain.TaskImportColSuperiorAcknowledged] = mark

	if len(units)+len(staff)+len(blocs) == 0 {
		return out
	}
	if h.d.TaskImportChoices == nil {
		h.d.Log.Warn("mẫu nhập nhiệm vụ: chưa nối identity — mẫu không có danh sách chọn bộ phận, cán bộ, khối", "xa", xa)
		return out
	}
	unitValues, staffValues, blocValues, err := h.identityChoices(ctx, units, staff, blocs)
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: mẫu nhập nhiệm vụ: không tra được identity — mẫu không có danh sách chọn bộ phận, cán bộ, khối",
			"xa", xa, "err", err)
		return out
	}
	out[domain.TaskImportColUnit] = xlsx.Choices{Values: unitValues, Title: "Đơn vị thực hiện",
		Message: "Giá trị này không có trong danh sách bộ phận của xã. Khi nhập, hệ thống sẽ kiểm lại."}
	out[domain.TaskImportColAssignee] = xlsx.Choices{Values: staffValues, Title: "Người thực hiện",
		Message: "Giá trị này không có trong danh sách cán bộ nhận việc của xã. Khi nhập, hệ thống sẽ kiểm lại."}
	out[domain.TaskImportColBloc] = xlsx.Choices{Values: blocValues, Title: "Khối nhiệm vụ",
		Message: "Giá trị này không có trong danh mục khối nhiệm vụ đang dùng của xã. Khi nhập, hệ thống sẽ kiểm lại."}
	return out
}

// identityChoices asks identity which of the client's codes are this commune's, with the predicate the
// import itself applies to each column (app.TaskImport), and returns the dropdown values in the client's
// order. Any failure returns an error and no list.
func (h *Handler) identityChoices(ctx context.Context, units, staff, blocs []string) (
	unitValues, staffValues, blocValues []string, err error) {

	id := h.d.TaskImportChoices

	// Units: LIVE units of this commune by code, then their names.
	unitIDs, err := lookupInChunks(ctx, units, identityclient.MaxLookupKeysPerCall, id.LiveOrgUnitIDsByCode)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bộ phận theo mã: %w", err)
	}
	ids := make([]string, 0, len(unitIDs))
	for _, code := range units {
		if v, ok := unitIDs[code]; ok {
			ids = append(ids, v)
		}
	}
	unitNames, err := lookupInChunks(ctx, ids, identityclient.MaxLookupKeysPerCall, id.OrgUnitNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tên bộ phận: %w", err)
	}
	for _, code := range units {
		if v, ok := unitIDs[code]; ok {
			unitValues = append(unitValues, xlsx.ChoiceValue(code, unitNames[v].Name))
		}
	}

	// Staff: those who may be handed work in this commune (the import's own check), then their names.
	assignable, err := lookupInChunks(ctx, staff, identityclient.TranMaGiaoViecMotLo, id.CanBoGiaoViecDuoc)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cán bộ nhận việc: %w", err)
	}
	confirmed := make([]string, 0, len(assignable))
	for _, code := range staff {
		if _, ok := assignable[code]; ok {
			confirmed = append(confirmed, code)
		}
	}
	staffNames, err := lookupInChunks(ctx, confirmed, identityclient.TranMaMotLo, id.TenCanBoTheoMa)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tên cán bộ: %w", err)
	}
	for _, code := range confirmed {
		staffValues = append(staffValues, xlsx.ChoiceValue(code, staffNames[code].HoTen))
	}

	// Blocs: TaskBlocLabels answers removed codes too (flagged), so LIVE is filtered here — the import
	// refuses a removed bloc, and the file must not offer one.
	labels, err := lookupInChunks(ctx, blocs, identityclient.MaxLookupKeysPerCall, id.TaskBlocLabels)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("khối nhiệm vụ: %w", err)
	}
	for _, code := range blocs {
		if l, ok := labels[code]; ok && l.Standing == identityv1.RecordStanding_RECORD_STANDING_LIVE {
			blocValues = append(blocValues, xlsx.ChoiceValue(code, l.Label))
		}
	}
	return unitValues, staffValues, blocValues, nil
}

// lookupInChunks calls an identity lookup over keys (already de-duplicated) in batches of `size` — each
// lookup refuses a larger batch rather than truncating it — and merges the answers.
func lookupInChunks[V any](ctx context.Context, keys []string, size int,
	call func(context.Context, []string) (map[string]V, error)) (map[string]V, error) {

	out := make(map[string]V, len(keys))
	for start := 0; start < len(keys); start += size {
		got, err := call(ctx, keys[start:min(start+size, len(keys))])
		if err != nil {
			return nil, err
		}
		for k, v := range got {
			out[k] = v
		}
	}
	return out, nil
}

// importTemplateExample is the one filled row, so the format is obvious without a manual. EVERY VALUE
// IS INVENTED and names no real unit, person or document of any commune (rule 3, invariant 5): the
// codes are placeholders the clerk replaces with their own commune's.
var importTemplateExample = []string{
	"Rà soát quỹ đất công ích năm 2026",
	"Thống kê và đối chiếu hồ sơ địa chính",
	"ma-bo-phan-thuc-hien",
	"CB-00000",
	"cao",
	"30/09/2026 17:00",
	"theo-van-ban",
	"khoi-uy-ban",
	"Công văn số 0000-CV/XX ngày 01/09/2026 của cơ quan cấp trên",
	"Công văn số 0000-CV/ĐU ngày 05/09/2026 của Ban Thường vụ Đảng uỷ",
	"Báo cáo kết quả rà soát",
	"x",
	"",
	"Ghi chú mẫu — xoá dòng này trước khi nhập",
}

// taskImportSheetName is the data sheet's tab — the FIRST sheet, the one readImportSheet reads.
const taskImportSheetName = "Nhiem vu"

// renderImportTemplate builds the template through core/xlsx: the heading row, the example row as text
// cells, and the dropdowns in `choices` (columns with no entry have none). The heading stays at row 1,
// where the reader looks first.
func renderImportTemplate(choices map[int]xlsx.Choices) ([]byte, error) {
	b, err := xlsx.Template(xlsx.TemplateSpec{
		SheetName: taskImportSheetName,
		Header:    domain.TaskImportHeadings,
		Examples:  [][]string{importTemplateExample},
		Choices:   choices,
	})
	if err != nil {
		return nil, fmt.Errorf("mẫu nhập nhiệm vụ: %w", err)
	}
	return b, nil
}
