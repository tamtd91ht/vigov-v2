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
	"fmt"
	"net/http"
	"slices"
	"strings"
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

// maxLetterTemplateUnits bounds the repeated `org_unit` query parameter — the unit codes the
// template's `Bộ phận xử lý (mã)` dropdown may offer (owner, 14/10/2026: the screen already holds the
// commune's units, so it sends their codes). One request may not make this route ask identity for an
// unbounded list: 2000 is ten calls of identityclient.MaxLookupKeysPerCall.
const maxLetterTemplateUnits = 2000

// letterImportTemplateSpec is the template: header, one example, a guide. The example uses the agreed
// fake number (rule 3, invariant 5) and an invented sender. The type column carries a WARN-only
// dropdown: the reader also accepts the code and "phản ánh", and re-validates whatever arrives.
//
// units, when non-empty, is a second WARN-only dropdown on the unit column: "code · name" entries
// identity confirmed (app.CitizenLetterImport.UnitChoices). Warn, not refuse: a unit created after the
// download must still be typeable, and the import re-checks every code against identity anyway.
func letterImportTemplateSpec(units []app.LetterUnitChoice) xlsx.TemplateSpec {
	spec := xlsx.TemplateSpec{
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
			"Bộ phận xử lý (mã): không bắt buộc. Chọn trong danh sách (dạng \"mã · tên\") hoặc ghi MÃ bộ phận (không ghi riêng tên); để trống thì đơn ở trạng thái Mới vào sổ.",
			"Số vào sổ do hệ thống cấp theo dãy số của xã trong năm; hạn xử lý tự tính theo cấu hình thời hạn đơn thư của xã — tệp không có hai cột này.",
			"Tệp được nhập TOÀN BỘ hoặc KHÔNG dòng nào: một dòng lỗi thì chưa đơn nào được vào sổ và chưa số nào được cấp. Hãy bấm Kiểm tra trước khi Nhập.",
			"Tối đa 200 dòng mỗi lần nhập.",
		},
	}
	if len(units) > 0 {
		values := make([]string, 0, len(units))
		for _, u := range units {
			values = append(values, xlsx.ChoiceValue(u.Code, u.Name))
		}
		spec.Choices[letterImportUnitColumn()] = xlsx.Choices{Values: values, Strict: false,
			Title: "Bộ phận xử lý", Message: "Chọn một bộ phận trong danh sách, ghi mã bộ phận, hoặc để trống."}
	}
	return spec
}

// letterImportUnitColumn is the 0-based index of `Bộ phận xử lý (mã)` in the template.
func letterImportUnitColumn() int {
	return slices.Index(domain.LetterImportColumns(), domain.LetterImportColUnit)
}

// letterTemplateUnitCodes reads the `org_unit` list: trimmed, blanks and duplicates dropped, first
// occurrence's order kept. A code longer than the reader accepts could never be imported, so it is
// dropped too. Over maxLetterTemplateUnits distinct codes answers 400 here (ok false).
func letterTemplateUnitCodes(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	// The name stays a literal: tools/apidoc reads query parameters from this expression.
	raw := r.URL.Query()["org_unit"]
	seen := make(map[string]bool, len(raw))
	codes := make([]string, 0, len(raw))
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] || len([]rune(c)) > domain.MaxLetterImportUnitCode {
			continue
		}
		seen[c] = true
		codes = append(codes, c)
	}
	if len(codes) > maxLetterTemplateUnits {
		httpx.WriteError(w, http.StatusBadRequest, "too_many_choices",
			fmt.Sprintf("Danh sách bộ phận gửi kèm quá dài (tối đa %d mã).", maxLetterTemplateUnits), "")
		return nil, false
	}
	return codes, true
}

// CitizenLetterImportTemplate serves GET /api/v1/citizen-letters/import-template[?org_unit=<mã>...].
//
// THE FILE IS PER COMMUNE once it carries a unit dropdown — no-store (writeXLSXFile), and the list is
// identity's answer for the commune in the context, never the client's list as sent.
//
// IDENTITY DOWN NEVER FAILS THE DOWNLOAD: the dropdown is a convenience, the import re-checks every
// code, so the template is served without it and the person can still type codes.
func (h *Handler) CitizenLetterImportTemplate(w http.ResponseWriter, r *http.Request) {
	codes, ok := letterTemplateUnitCodes(w, r)
	if !ok {
		return
	}
	var units []app.LetterUnitChoice
	if len(codes) > 0 {
		got, err := h.d.CitizenLetterImport.UnitChoices(r.Context(), codes)
		if err != nil {
			// The commune and a count — codes are not personal data, but the log needs none of them.
			h.d.Log.Warn("mẫu nhập đơn thư: không xác nhận được bộ phận, tệp mẫu không có danh sách chọn bộ phận",
				"xa", string(tenant.MustFrom(r.Context())), "so_ma", len(codes), "err", err)
		} else {
			units = got
		}
	}
	b, err := xlsx.Template(letterImportTemplateSpec(units))
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
	unitCellsToCodes(sheet)
	rows, errs := domain.ReadLetterImportSheet(sheet, h.now())
	return caller, rows, errs, true
}

// unitCellsToCodes reduces every data row's `Bộ phận xử lý (mã)` cell to its code: the dropdown
// writes "code · name", a hand-typed cell holds the bare code, and both mean the code (xlsx.ChoiceCode).
// HERE, NOT IN THE DOMAIN READER: domain/ is standard library only, and the separator belongs to
// core/xlsx. The code is then length-checked and resolved against identity exactly as a typed one —
// the name part is never matched (domain/citizen_letter_import.go header: names repeat).
func unitCellsToCodes(sheet [][]string) {
	col := letterImportUnitColumn()
	for i := 1; i < len(sheet); i++ {
		if col < len(sheet[i]) {
			sheet[i][col] = xlsx.ChoiceCode(sheet[i][col])
		}
	}
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
