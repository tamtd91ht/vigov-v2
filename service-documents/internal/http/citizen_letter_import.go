package http

// `Nhập từ Excel` on the citizen-letter register (ADR 0084 #6). Routes: routes_citizen_letter_import.go.
// The shape of document_type_import.go — the same upload reader, the same sheet reader, the same
// preview-is-its-own-route argument (the idempotency key is scoped to the PATH, so a `dry_run` flag
// could replay a preview's 2xx to an import that wrote nothing).
//
//	GET  /api/v1/citizen-letters/import-template  the .xlsx to fill in
//	POST /api/v1/citizen-letters/import-previews  check a filled file — WRITES NOTHING, takes no number
//	POST /api/v1/citizen-letters/imports          book it, all or nothing (app/citizen_letter_import.go)
//
// NOTHING FROM A CELL IS ECHOED, LOGGED OR RETURNED (rule 3): the responses carry row numbers, types,
// dates, unit ids and deadlines — never a sender's name, phone, address or the letter's content.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const letterImportSheetName = "So don thu"

type letterImportErrorOut struct {
	Row     int    `json:"row"`    // spreadsheet row (header = 1); 0 = the whole file
	Column  string `json:"column"` // the template's column header; "" = the whole row or file
	Message string `json:"message"`
}

// letterImportRowOut is one row as planned (preview: no id, no number) or booked. NO PERSONAL DATA.
type letterImportRowOut struct {
	Row             int     `json:"row"`
	ID              string  `json:"id,omitempty"`     // once booked
	Number          int     `json:"number,omitempty"` // once booked
	Year            int     `json:"year"`
	LetterType      string  `json:"letter_type"`
	ReceivedDate    string  `json:"received_date"` // YYYY-MM-DD
	HoldingUnitID   *string `json:"holding_unit_id"`
	ProcessingDueAt *string `json:"processing_due_at"` // null = "Không đặt hạn"
	SenderUnknown   bool    `json:"sender_unknown"`    // the row said "Không rõ"
}

// letterImportPreviewOut — `valid` false means the import would be refused with exactly these errors.
type letterImportPreviewOut struct {
	Valid   bool                   `json:"valid"`
	Letters []letterImportRowOut   `json:"letters"`
	Errors  []letterImportErrorOut `json:"errors"`
}

type letterImportCreatedOut struct {
	Created []letterImportRowOut `json:"created"`
}

// letterImportRejectedOut is httpx.Error plus the list.
type letterImportRejectedOut struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	TraceID string                 `json:"trace_id"`
	Errors  []letterImportErrorOut `json:"errors"`
}

func letterImportErrorsOut(in []domain.LetterImportError) []letterImportErrorOut {
	out := make([]letterImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, letterImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func letterImportRowsOut(in []app.ImportedLetter) []letterImportRowOut {
	out := make([]letterImportRowOut, 0, len(in))
	for _, l := range in {
		out = append(out, letterImportRowOut{Row: l.Row, ID: l.ID, Number: l.Number, Year: l.Year,
			LetterType: string(l.Type), ReceivedDate: l.ReceivedDate.Format(time.DateOnly),
			HoldingUnitID: strPtr(l.HoldingUnitID), ProcessingDueAt: instantPtr(l.ProcessingDueAt),
			SenderUnknown: l.SenderUnknown})
	}
	return out
}

// letterImportTemplateSpec is the template: header, one example, a guide. The example uses the agreed
// fake number (rule 3, invariant 5) and an invented sender. The type column carries a WARN-only
// dropdown: the reader also accepts the code and "phản ánh", and re-validates whatever arrives.
func letterImportTemplateSpec() xlsx.TemplateSpec {
	return xlsx.TemplateSpec{
		SheetName: letterImportSheetName,
		Header:    domain.LetterImportColumns(),
		Examples: [][]string{{"2026-09-10", "Nguyễn Văn A", "Thôn 1", "0900000000", "kiến nghị",
			"Đề nghị sửa đường bê tông thôn 1 bị hư hỏng", ""}},
		Choices: map[int]xlsx.Choices{
			4: {Values: []string{"khiếu nại", "tố cáo", "kiến nghị", "đề nghị"}, Strict: false,
				Title: "Loại đơn", Message: "Chọn một trong: khiếu nại, tố cáo, kiến nghị (phản ánh), đề nghị."},
		},
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT đơn thư vào sổ. Không sửa dòng tiêu đề, không thêm cột.",
			"Ngày đến: bắt buộc, dạng YYYY-MM-DD (ví dụ 2026-10-07) hoặc ngày/tháng/năm; không ở tương lai.",
			"Họ tên người gửi: bắt buộc. Nếu không biết người gửi, ghi \"" + domain.LetterSenderUnknownWord + "\".",
			"Địa chỉ, Số điện thoại: không bắt buộc. Số điện thoại chỉ gồm chữ số, dấu cách, dấu +, -, . và ngoặc.",
			"Loại đơn: bắt buộc — khiếu nại, tố cáo, kiến nghị (phản ánh) hoặc đề nghị. Đơn tố cáo được che danh tính người gửi trên mọi danh sách.",
			"Nội dung đơn: bắt buộc, tối đa 2000 ký tự.",
			"Bộ phận xử lý (mã): không bắt buộc. Ghi MÃ bộ phận (không ghi tên); để trống thì đơn ở trạng thái Mới vào sổ.",
			"Số vào sổ do hệ thống cấp theo dãy số của xã trong năm; hạn xử lý tự tính theo cấu hình thời hạn đơn thư của xã — tệp không có hai cột này.",
			"Tệp được nhập TOÀN BỘ hoặc KHÔNG dòng nào: một dòng lỗi thì chưa đơn nào được vào sổ và chưa số nào được cấp. Hãy bấm Kiểm tra trước khi Nhập.",
			"Tối đa 200 dòng mỗi lần nhập.",
		},
	}
}

// CitizenLetterImportTemplate serves GET /api/v1/citizen-letters/import-template.
func (h *Handler) CitizenLetterImportTemplate(w http.ResponseWriter, r *http.Request) {
	b, err := xlsx.Template(letterImportTemplateSpec())
	if err != nil {
		h.d.Log.Error("mẫu nhập đơn thư: dựng tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	writeXLSXFile(w, r, h, "mau-so-don-thu.xlsx", b)
}

// writeXLSXFile sends a workbook. no-store: nothing on the way keeps a copy.
func writeXLSXFile(w http.ResponseWriter, r *http.Request, h *Handler, name string, b []byte) {
	w.Header().Set("Content-Type", xlsxMIME)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("gửi tệp Excel: ngắt khi đang gửi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
}

// readLetterUpload is the shared front half of preview and import: the caller, the file, the sheet,
// the shape rules. ok false = already answered.
func (h *Handler) readLetterUpload(w http.ResponseWriter, r *http.Request) (
	app.LetterCaller, []domain.LetterImportRow, []domain.LetterImportError, bool) {
	caller, ok := h.letterCaller(r)
	if !ok {
		h.thieuChuThe(w, r)
		return app.LetterCaller{}, nil, nil, false
	}
	data, ok := readXLSXUpload(w, r)
	if !ok {
		return app.LetterCaller{}, nil, nil, false
	}
	sheet, ok := h.readUploadedSheet(w, r, data)
	if !ok {
		return app.LetterCaller{}, nil, nil, false
	}
	rows, errs := domain.ReadLetterImportSheet(sheet, h.now())
	return caller, rows, errs, true
}

// PreviewCitizenLetterImport serves POST /api/v1/citizen-letters/import-previews. 200 whether or not
// the file is valid.
func (h *Handler) PreviewCitizenLetterImport(w http.ResponseWriter, r *http.Request) {
	caller, rows, rowErrs, ok := h.readLetterUpload(w, r)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		vietJSON(w, http.StatusOK, letterImportPreviewOut{
			Letters: []letterImportRowOut{}, Errors: letterImportErrorsOut(rowErrs)})
		return
	}
	res, err := h.d.CitizenLetterImport.Preview(r.Context(), rows, caller)
	if err != nil {
		h.letterImportError(w, r, err, false)
		return
	}
	vietJSON(w, http.StatusOK, letterImportPreviewOut{
		Valid:   len(res.Errors) == 0,
		Letters: letterImportRowsOut(res.Letters),
		Errors:  letterImportErrorsOut(res.Errors),
	})
}

// ImportCitizenLetters serves POST /api/v1/citizen-letters/imports.
func (h *Handler) ImportCitizenLetters(w http.ResponseWriter, r *http.Request) {
	caller, rows, rowErrs, ok := h.readLetterUpload(w, r)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		writeLetterImportRejected(w, rowErrs)
		return
	}
	res, err := h.d.CitizenLetterImport.Import(r.Context(), rows, caller)
	if err != nil {
		var rej *app.LetterImportRejected
		if errors.As(err, &rej) {
			writeLetterImportRejected(w, rej.Errors)
			return
		}
		h.letterImportError(w, r, err, true)
		return
	}
	vietJSON(w, http.StatusCreated, letterImportCreatedOut{Created: letterImportRowsOut(res.Letters)})
}

// letterImportError maps a failure that is not a row refusal. NOTHING WAS WRITTEN in every case — the
// import is one transaction — and the sentence says so.
func (h *Handler) letterImportError(w http.ResponseWriter, r *http.Request, err error, writing bool) {
	switch {
	case errors.Is(err, app.ErrLetterImportUnchecked):
		httpx.WriteError(w, http.StatusServiceUnavailable, "import_unchecked",
			"Chưa kiểm được bộ phận hoặc hạn xử lý với danh bạ của xã nên chưa đơn nào được vào sổ. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, docstore.ErrDaySoDayTran):
		httpx.WriteError(w, http.StatusConflict, "so_don_thu_day",
			"Dãy số của sổ đơn thư năm nay không đủ cho cả tệp nên chưa đơn nào được vào sổ. Hãy báo quản trị hệ thống.", "")
	default:
		op := "xem trước nhập Excel"
		if writing {
			op = "nhập Excel"
		}
		// The cause, the commune and the operation — nothing from the file (rule 3).
		h.d.Log.Error("đơn thư: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Chưa đơn nào được vào sổ. Vui lòng thử lại.", "")
	}
}

// writeLetterImportRejected is 400 with every error; nothing was written.
func writeLetterImportRejected(w http.ResponseWriter, errs []domain.LetterImportError) {
	vietJSON(w, http.StatusBadRequest, letterImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa đơn nào được vào sổ. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  letterImportErrorsOut(errs),
	})
}
