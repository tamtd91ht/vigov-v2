package http

// The spreadsheet import of tasks (docs/ui-ux/02-nhiem-vu.md §8):
//
//	POST /api/v1/tasks/imports          multipart `file`, optional `dry_run=true`   task.create
//	GET  /api/v1/tasks/import-template  the empty template with one example row    task.create
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
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
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

// ImportTemplate serves GET /api/v1/tasks/import-template.
func (h *Handler) ImportTemplate(w http.ResponseWriter, r *http.Request) {
	b, err := renderImportTemplate()
	if err != nil {
		h.d.Log.Error("mẫu nhập nhiệm vụ: lỗi dựng tệp", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+taskImportTemplateName+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
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

// renderImportTemplate builds the template: the heading row and one example row, as text cells.
func renderImportTemplate() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Nhiem vu"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	head, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	for i, title := range domain.TaskImportHeadings {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, cell, title); err != nil {
			return nil, err
		}
		ex, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellStr(sheet, ex, importTemplateExample[i]); err != nil {
			return nil, err
		}
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sheet, col, col, 26); err != nil {
			return nil, err
		}
	}
	last, _ := excelize.CoordinatesToCellName(len(domain.TaskImportHeadings), 1)
	if err := f.SetCellStyle(sheet, "A1", last, head); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
