package http

// "Nạp từ Excel" on the revenue / expenditure board (07-thu-chi-ngan-sach §6, ADR 0081 #6). Routes:
// routes_budget_import.go.
//
//	POST /api/v1/budget-sheets/import-previews?year=  check the file — WRITES NOTHING
//	POST /api/v1/budget-sheets/imports?year=          load it, all or nothing
//
// THIS LAYER OWNS THE UPLOAD AND THE WIRE: the same upload reader and limits as the other imports of
// this service, core/xlsx.ReadSheets (every tab, with each cell's "stored as a number" flag), then
// domain.ParseBudgetWorkbook. The rules are the domain's; the transaction is the app's.
//
// NOTHING FROM THE FILE REACHES A LOG LINE — not the file name, not a heading, not a figure. Only the
// commune is logged, as everywhere in this service.

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// budgetImportIssueOut is one error or warning. `row` is the spreadsheet row (0 = the whole sheet or
// file); `column` the heading as the file writes it ("" = the whole row); `sheet` the tab name.
type budgetImportIssueOut struct {
	Sheet   string `json:"sheet"`
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// budgetImportColumnOut is one column as it will be (preview) or was (import) created. `role` is the
// GUESS from the heading (ADR 0081 #6); the operand names are null on a percentage column whose
// operands could not be identified — its lines will read "không tính được".
type budgetImportColumnOut struct {
	ID              string  `json:"id,omitempty"`
	Name            string  `json:"name"`
	Type            string  `json:"type"` // `so` | `phan_tram`
	Role            string  `json:"role,omitempty"`
	NumeratorName   *string `json:"numerator_name"`
	DenominatorName *string `json:"denominator_name"`
}

// budgetImportHeadlineOut is the line starred automatically.
type budgetImportHeadlineOut struct {
	No   string `json:"no"`
	Name string `json:"name"`
}

// budgetImportSheetOut is one tab of the file.
type budgetImportSheetOut struct {
	SheetName string `json:"sheet_name"`
	Kind      string `json:"kind"` // `thu` | `chi`
	Title     string `json:"title"`
	Unit      string `json:"unit"` // `dong` | `nghin-dong` | `trieu-dong`
	UnitLabel string `json:"unit_label"`
	LineCount int    `json:"line_count"`

	Columns  []budgetImportColumnOut  `json:"columns"`
	Headline *budgetImportHeadlineOut `json:"headline"`

	// Action is `create` (no live sheet for that year and kind) or `replace` (the live one, named by
	// ReplacesCode, is soft deleted and this becomes the next revision).
	Action       string `json:"action"`
	ReplacesCode string `json:"replaces_code,omitempty"`

	// RefusalReason — preview only: why this sheet would refuse the file (hand-entered figures).
	RefusalReason string `json:"refusal_reason,omitempty"`

	// Sheet is the created sheet — import only.
	Sheet *bangRa `json:"sheet,omitempty"`

	Warnings []budgetImportIssueOut `json:"warnings"`
}

// budgetImportOut is the 200 of the preview and the 201 of the import.
type budgetImportOut struct {
	Valid      bool   `json:"valid"`
	Year       int    `json:"year"`
	SourceFile string `json:"source_file"`

	// RefusalReason — preview only: the period-close sentence when the year or a month of it is closed.
	RefusalReason string `json:"refusal_reason,omitempty"`

	Sheets   []budgetImportSheetOut `json:"sheets"`
	Warnings []budgetImportIssueOut `json:"warnings"` // about the file: hidden tabs, tabs with no table
	Errors   []budgetImportIssueOut `json:"errors"`
}

// budgetImportRejectedOut is the 400 `import_invalid`: every error; nothing was written.
type budgetImportRejectedOut struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	TraceID string                 `json:"trace_id"`
	Errors  []budgetImportIssueOut `json:"errors"`
}

func issuesOut(in []domain.BudgetImportIssue) []budgetImportIssueOut {
	out := make([]budgetImportIssueOut, 0, len(in))
	for _, i := range in {
		out = append(out, budgetImportIssueOut{Sheet: i.Sheet, Row: i.Row, Column: i.Column, Message: i.Message})
	}
	return out
}

func writeBudgetFileTooLarge(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
		"Tệp quá lớn. Tối đa 2 MB, 16 sheet và 5000 dòng mỗi sheet.", "")
}

// readBudgetWorkbook reads `year`, the upload and every tab, and parses them. ok=false means the
// response was already written. parseErrs non-empty means the file does not load (answered by the
// caller: 200 on the preview, 400 on the import).
func (h *Handler) readBudgetWorkbook(w http.ResponseWriter, r *http.Request) (int, string,
	domain.ParsedBudgetWorkbook, []domain.BudgetImportIssue, bool) {

	var raw string
	if v := r.URL.Query()["year"]; len(v) == 1 {
		raw = v[0]
	}
	year, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || domain.KiemTraNamNganSach(year) != nil {
		// No default year: loading a report under the wrong year files a whole year's figures wrongly.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu hoặc sai `year` (năm ngân sách 2000..2100), ví dụ ?year=2026.", "")
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	}
	data, name, ok := readUploadWithName(w, r, writeBudgetFileTooLarge)
	if !ok {
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	}
	sheets, err := xlsx.ReadSheets(bytes.NewReader(data), int64(len(data)), xlsx.DefaultLimits)
	switch {
	case err == nil, errors.Is(err, xlsx.ErrEmptySheet):
	case errors.Is(err, xlsx.ErrTooLarge), errors.Is(err, xlsx.ErrTooManyRows):
		writeBudgetFileTooLarge(w)
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	case errors.Is(err, xlsx.ErrMacroEnabled):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp có macro (.xlsm) không được nhận. Hãy lưu lại dưới dạng Excel Workbook (.xlsx) không macro.", "")
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	case errors.Is(err, xlsx.ErrNotXLSX):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp không phải bảng tính Excel .xlsx hợp lệ (hoặc đang đặt mật khẩu).", "")
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	case errors.Is(err, xlsx.ErrMalformed):
		httpx.WriteError(w, http.StatusBadRequest, "malformed_file",
			"Tệp Excel bị hỏng hoặc không đọc trọn được. Hãy mở tệp bằng Excel, lưu lại rồi gửi lại.", "")
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	default:
		h.d.Log.Error("nạp Excel thu chi: đọc tệp lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return 0, "", domain.ParsedBudgetWorkbook{}, nil, false
	}
	wb, parseErrs := domain.ParseBudgetWorkbook(budgetSheetsIn(sheets))
	return year, name, wb, parseErrs, true
}

// budgetSheetsIn hands the domain each cell's text and its "stored as a number" flag.
func budgetSheetsIn(sheets []xlsx.Sheet) []domain.BudgetImportSheet {
	out := make([]domain.BudgetImportSheet, 0, len(sheets))
	for _, s := range sheets {
		rows := make([][]domain.BudgetImportCell, len(s.Rows))
		for ri, row := range s.Rows {
			rows[ri] = make([]domain.BudgetImportCell, len(row))
			for ci, v := range row {
				rows[ri][ci] = domain.BudgetImportCell{Text: v, Number: s.IsNumber(ri, ci)}
			}
		}
		out = append(out, domain.BudgetImportSheet{Name: s.Name, Hidden: s.Hidden, Rows: rows})
	}
	return out
}

func budgetImportResultOut(res app.BudgetImportResult, wb domain.ParsedBudgetWorkbook) budgetImportOut {
	out := budgetImportOut{
		Valid: res.Valid(), Year: res.Year, SourceFile: res.FileName, RefusalReason: res.Refusal,
		Sheets: []budgetImportSheetOut{}, Warnings: issuesOut(wb.Warnings), Errors: []budgetImportIssueOut{},
	}
	for _, o := range res.Sheets {
		s := o.Sheet
		so := budgetImportSheetOut{
			SheetName: s.SheetName, Kind: string(s.Kind), Title: s.Title, Unit: string(s.Unit),
			UnitLabel: s.Unit.Nhan(), LineCount: len(s.Lines), Columns: []budgetImportColumnOut{},
			Action: "create", RefusalReason: o.Refusal, Warnings: issuesOut(s.Warnings),
		}
		if o.Replaces.ID != "" {
			so.Action, so.ReplacesCode = "replace", o.Replaces.Ma
		}
		for i, c := range s.Columns {
			co := budgetImportColumnOut{Name: c.Name, Type: string(c.Kind), Role: string(c.Role)}
			if i < len(o.Columns) {
				co.ID = o.Columns[i].ID
			}
			if c.Operands.Numerator != nil && c.Operands.Denominator != nil {
				num, den := s.Columns[*c.Operands.Numerator].Name, s.Columns[*c.Operands.Denominator].Name
				co.NumeratorName, co.DenominatorName = &num, &den
			}
			so.Columns = append(so.Columns, co)
		}
		if s.Headline >= 0 {
			l := s.Lines[s.Headline]
			so.Headline = &budgetImportHeadlineOut{No: l.TT, Name: l.Name}
		}
		if o.Created.ID != "" {
			b := bangRaNgoai(o.Created)
			so.Sheet = &b
		}
		out.Sheets = append(out.Sheets, so)
	}
	return out
}

// PreviewBudgetImport serves POST /api/v1/budget-sheets/import-previews — 200 whether or not the file
// would load.
func (h *Handler) PreviewBudgetImport(w http.ResponseWriter, r *http.Request) {
	year, name, wb, parseErrs, ok := h.readBudgetWorkbook(w, r)
	if !ok {
		return
	}
	if len(parseErrs) > 0 {
		vietJSON(w, http.StatusOK, budgetImportOut{Year: year, SourceFile: domain.NormaliseSourceFileName(name),
			Sheets: []budgetImportSheetOut{}, Warnings: issuesOut(wb.Warnings), Errors: issuesOut(parseErrs)})
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.GhiNganSach.PreviewBudgetImport(r.Context(), app.BudgetImportRequest{
		Year: year, FileName: name, Sheets: wb.Sheets,
	})
	if err != nil {
		h.writeBudgetImportError(w, r, err)
		return
	}
	vietJSON(w, http.StatusOK, budgetImportResultOut(res, wb))
}

// ImportBudgetWorkbook serves POST /api/v1/budget-sheets/imports.
func (h *Handler) ImportBudgetWorkbook(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	year, name, wb, parseErrs, ok := h.readBudgetWorkbook(w, r)
	if !ok {
		return
	}
	if len(parseErrs) > 0 {
		vietJSON(w, http.StatusBadRequest, budgetImportRejectedOut{
			Code: "import_invalid",
			Message: "Tệp có lỗi nên chưa nạp bảng nào. Sửa các chỗ được liệt kê rồi nạp lại — " +
				"tệp được nạp toàn bộ hoặc không gì cả.",
			Errors: issuesOut(parseErrs),
		})
		return
	}
	res, err := h.d.GhiNganSach.ImportBudgetWorkbook(r.Context(), app.BudgetImportRequest{
		Year: year, FileName: name, Sheets: wb.Sheets,
	}, actor)
	if err != nil {
		h.writeBudgetImportError(w, r, err)
		return
	}
	// What a retry carrying the same Idempotency-Key is told: THE CODES, never the body.
	var codes []string
	for _, s := range res.Sheets {
		codes = append(codes, s.Created.Ma)
	}
	idem.RecordCode(r.Context(), strings.Join(codes, ","))
	vietJSON(w, http.StatusCreated, budgetImportResultOut(res, wb))
}

// writeBudgetImportError maps the import's own refusals, then hands the rest to the board's shared
// mapping (a period close is 409 `budget_period_closed` there, with the conflict's own sentence).
func (h *Handler) writeBudgetImportError(w http.ResponseWriter, r *http.Request, err error) {
	var entries *domain.HandEntriesConflict
	switch {
	case errors.As(err, &entries):
		// The conflict's own sentence, never the wrapped error (that carries the commune id).
		httpx.WriteError(w, http.StatusConflict, "budget_sheet_has_entries", entries.Error(), "")
	case errors.Is(err, app.ErrBudgetImportEmpty):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", app.ErrBudgetImportEmpty.Error(), "")
	case errors.Is(err, app.ErrBudgetImportDuplicateKind):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", app.ErrBudgetImportDuplicateKind.Error(), "")
	default:
		h.traLoiLoiNganSach(w, r, "nạp Excel", err)
	}
}
