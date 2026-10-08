package http

// `Nhập từ Excel` on the document-type catalogue (ADR 0059 §3). Routes: routes_document_type_import.go.
// The shape of service-comms/internal/http/map_asset_type_import.go.
//
//	GET  /api/v1/document-types/import-template  the .xlsx to fill in
//	POST /api/v1/document-types/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/document-types/imports          write it, all or nothing
//
// THE PREVIEW IS ITS OWN ROUTE, NOT A `dry_run` FLAG: the idempotency key is scoped to (commune, actor,
// method, PATH), never the body. A flag on one path would let a client that previewed with key K and
// then imported with K be REPLAYED the preview's 2xx: told the file was imported while nothing was
// written.
//
// THE UPLOAD NEVER TOUCHES DISK: MultipartReader part by part, never ParseMultipartForm (which spills a
// large part into a temporary file). The workbook is read by core/xlsx.ReadSheet, whose sentinels map
// to FIXED sentences below — never err.Error(), never a cell value.

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

const (
	xlsxFormField = "file"
	xlsxMIME      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	// A constant rather than a literal: rbac_guard reads a header lookup by string literal as a route
	// declaration.
	headerContentTypeName = "Content-Type"

	documentTypeSheetName = "Loai van ban"
)

// xlsxBodyMax is the file cap plus room for the multipart envelope; over it is 413 before a byte of the
// workbook is parsed. Read from core/xlsx so the two caps cannot drift apart.
var xlsxBodyMax = xlsx.DefaultLimits.MaxFileBytes + 64<<10

type documentTypeImportErrorOut struct {
	Row     int    `json:"row"`    // spreadsheet row (header = 1); 0 = the whole file
	Column  string `json:"column"` // the header's column name; "" = the whole row or file
	Message string `json:"message"`
}

type documentTypeImportRowOut struct {
	Row   int    `json:"row"`
	ID    string `json:"id,omitempty"` // only once created
	Code  string `json:"code"`
	Label string `json:"label"`
	Order int    `json:"order"`
}

// documentTypeImportPreviewOut — `valid` false means the import would be refused with exactly these errors.
type documentTypeImportPreviewOut struct {
	Valid  bool                         `json:"valid"`
	Types  []documentTypeImportRowOut   `json:"types"`
	Errors []documentTypeImportErrorOut `json:"errors"`
}

type documentTypeImportCreatedOut struct {
	Created []documentTypeImportRowOut `json:"created"`
}

// documentTypeImportRejectedOut is httpx.Error plus the list.
type documentTypeImportRejectedOut struct {
	Code    string                       `json:"code"`
	Message string                       `json:"message"`
	TraceID string                       `json:"trace_id"`
	Errors  []documentTypeImportErrorOut `json:"errors"`
}

func documentTypeErrorsOut(in []domain.DocumentTypeImportError) []documentTypeImportErrorOut {
	out := make([]documentTypeImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, documentTypeImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func documentTypeRowsOut(in []domain.PlannedDocumentType) []documentTypeImportRowOut {
	out := make([]documentTypeImportRowOut, 0, len(in))
	for _, p := range in {
		out = append(out, documentTypeImportRowOut{Row: p.Row, ID: p.ID, Code: p.Code, Label: p.Label, Order: p.Order})
	}
	return out
}

// documentTypeTemplateSpec is the template: header, one example, a guide. No dropdown — none of the
// three columns draws from a list.
func documentTypeTemplateSpec() xlsx.TemplateSpec {
	return xlsx.TemplateSpec{
		SheetName: documentTypeSheetName,
		Header:    domain.DocumentTypeImportColumns(),
		Examples:  [][]string{{"Tờ trình", "", "1"}},
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT loại văn bản mới của xã. Không sửa dòng tiêu đề.",
			"Tên hiển thị: bắt buộc, tối đa 200 ký tự, không trùng loại đang có trong xã hay dòng khác trong tệp.",
			"Mã: để trống thì tự sinh từ tên (bỏ dấu, nối bằng gạch ngang). Nếu nhập: chữ thường a-z, số, dấu gạch ngang, ví dụ to-trinh. Mã đã cấp — kể cả của loại đã xoá — không cấp lại.",
			"Thứ tự: số nguyên từ 0 đến 9999, để trống là 0.",
			"Tệp được nhập TOÀN BỘ hoặc KHÔNG dòng nào: một dòng lỗi thì chưa loại nào được tạo. Hãy bấm Kiểm tra trước khi Nhập.",
			"Tối đa 200 dòng mỗi lần nhập. Nhập chỉ THÊM loại mới, không sửa loại đang có và không đổi loại mặc định của xã.",
		},
	}
}

// DocumentTypeImportTemplate serves GET /api/v1/document-types/import-template.
func (h *Handler) DocumentTypeImportTemplate(w http.ResponseWriter, r *http.Request) {
	b, err := xlsx.Template(documentTypeTemplateSpec())
	if err != nil {
		h.d.Log.Error("mẫu nhập loại văn bản: dựng tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", xlsxMIME)
	w.Header().Set("Content-Disposition", `attachment; filename="mau-nhap-loai-van-ban.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập loại văn bản: gửi tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
}

// readXLSXUpload reads the one `file` part into memory, or answers and returns false.
//
//	415  not multipart/form-data; a part type other than xlsx / octet-stream / none; a name not .xlsx
//	413  the body or the file is over its cap
//	400  no `file` part, or two of them
func readXLSXUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	mt, _, err := mime.ParseMediaType(r.Header.Get(headerContentTypeName))
	if err != nil || mt != "multipart/form-data" {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Hãy gửi tệp dưới dạng multipart/form-data, trường \"file\".", "")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, xlsxBodyMax)
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Nội dung gửi lên không đọc được.", "")
		return nil, false
	}
	var data []byte
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeUploadError(w, err)
			return nil, false
		}
		if part.FormName() != xlsxFormField {
			if _, err := io.Copy(io.Discard, part); err != nil {
				writeUploadError(w, err)
				return nil, false
			}
			continue
		}
		if data != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ gửi MỘT tệp mỗi lần nhập.", "")
			return nil, false
		}
		if !acceptableXLSXPart(part.Header.Get(headerContentTypeName), part.FileName()) {
			writeNotXLSX(w)
			return nil, false
		}
		b, err := io.ReadAll(io.LimitReader(part, xlsx.DefaultLimits.MaxFileBytes+1))
		if err != nil {
			writeUploadError(w, err)
			return nil, false
		}
		if int64(len(b)) > xlsx.DefaultLimits.MaxFileBytes {
			writeXLSXTooLarge(w)
			return nil, false
		}
		data = b
	}
	if data == nil {
		httpx.WriteError(w, http.StatusBadRequest, "missing_file", "Chưa chọn tệp để nhập (trường \"file\").", "")
		return nil, false
	}
	return data, true
}

func acceptableXLSXPart(contentType, fileName string) bool {
	if fileName != "" && !strings.HasSuffix(strings.ToLower(fileName), ".xlsx") {
		return false
	}
	if contentType == "" {
		return true
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mt == xlsxMIME || mt == "application/octet-stream"
}

func writeXLSXTooLarge(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
		"Tệp quá lớn. Tối đa 2 MB và 200 dòng mỗi lần nhập.", "")
}

func writeNotXLSX(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
		"Chỉ nhận tệp Excel .xlsx (không nhận .xls, .xlsm có macro hay .csv). Hãy tải tệp mẫu và nhập vào đó.", "")
}

func writeUploadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeXLSXTooLarge(w)
		return
	}
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Nội dung gửi lên không đọc được.", "")
}

// parseDocumentTypeUpload reads the workbook into data rows, or answers the refusal of the FILE and
// returns false. Errors of the CONTENT (a wrong header, cells outside the three columns) come back in
// the second value. EVERY SENTENCE IS FIXED: nothing from err.Error() or from a cell.
func (h *Handler) parseDocumentTypeUpload(w http.ResponseWriter, r *http.Request, data []byte) (
	[]domain.DocumentTypeImportRow, []domain.DocumentTypeImportError, bool) {

	sheet, ok := h.readUploadedSheet(w, r, data)
	if !ok {
		return nil, nil, false
	}
	rows, errs := domain.ReadDocumentTypeSheet(sheet)
	return rows, errs, true
}

// readUploadedSheet reads the first sheet of an uploaded workbook, or answers the refusal of the FILE
// and returns false. Shared by every import of this service (document types, citizen letters) so the
// sentences and statuses cannot drift between them. EVERY SENTENCE IS FIXED.
func (h *Handler) readUploadedSheet(w http.ResponseWriter, r *http.Request, data []byte) ([][]string, bool) {
	sheet, err := xlsx.ReadSheet(bytes.NewReader(data), int64(len(data)), xlsx.DefaultLimits)
	switch {
	case err == nil:
		return sheet, true
	case errors.Is(err, xlsx.ErrTooLarge):
		writeXLSXTooLarge(w)
	case errors.Is(err, xlsx.ErrMacroEnabled):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp có macro (.xlsm) không được nhận. Hãy lưu lại dưới dạng Excel Workbook (.xlsx) không macro.", "")
	case errors.Is(err, xlsx.ErrNotXLSX):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp không phải bảng tính Excel .xlsx hợp lệ (hoặc đang đặt mật khẩu). Hãy tải tệp mẫu và nhập vào đó.", "")
	case errors.Is(err, xlsx.ErrMalformed):
		httpx.WriteError(w, http.StatusBadRequest, "file_malformed",
			"Tệp Excel bị hỏng hoặc không đọc trọn được. Hãy mở lại bằng Excel, lưu dưới dạng .xlsx rồi gửi lại.", "")
	case errors.Is(err, xlsx.ErrTooManyRows):
		httpx.WriteError(w, http.StatusBadRequest, "too_many_rows",
			"Trang tính đầu có quá nhiều dòng (kể cả dòng trống). Tối đa 200 dòng dữ liệu mỗi lần nhập.", "")
	case errors.Is(err, xlsx.ErrEmptySheet):
		httpx.WriteError(w, http.StatusBadRequest, "empty_file",
			"Trang tính đầu của tệp trống. Hãy tải tệp mẫu và nhập vào đó.", "")
	default:
		h.d.Log.Error("nhập Excel: đọc tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return nil, false
}

// PreviewDocumentTypeImport serves POST /api/v1/document-types/import-previews. 200 whether or not the
// file is valid.
func (h *Handler) PreviewDocumentTypeImport(w http.ResponseWriter, r *http.Request) {
	data, ok := readXLSXUpload(w, r)
	if !ok {
		return
	}
	rows, rowErrs, ok := h.parseDocumentTypeUpload(w, r, data)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		vietJSON(w, http.StatusOK, documentTypeImportPreviewOut{
			Types: []documentTypeImportRowOut{}, Errors: documentTypeErrorsOut(rowErrs)})
		return
	}
	res, err := h.d.GhiLoaiVanBan.PreviewDocumentTypeImport(r.Context(), rows)
	if err != nil {
		h.d.Log.Error("nhập loại văn bản: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, documentTypeImportPreviewOut{
		Valid:  len(res.Errors) == 0,
		Types:  documentTypeRowsOut(res.Types),
		Errors: documentTypeErrorsOut(res.Errors),
	})
}

// ImportDocumentTypes serves POST /api/v1/document-types/imports.
func (h *Handler) ImportDocumentTypes(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến nhập loại văn bản chạy mà không có mã cán bộ — SAI CẤU HÌNH ROUTE hoặc định danh cũ",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	data, ok := readXLSXUpload(w, r)
	if !ok {
		return
	}
	rows, rowErrs, ok := h.parseDocumentTypeUpload(w, r, data)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		writeDocumentTypeImportRejected(w, rowErrs)
		return
	}
	res, err := h.d.GhiLoaiVanBan.ImportDocumentTypes(r.Context(), rows, actor)
	if err != nil {
		var rej *app.DocumentTypeImportRejected
		if errors.As(err, &rej) {
			writeDocumentTypeImportRejected(w, rej.Errors)
			return
		}
		// Includes a code taken by a concurrent request between the plan and the insert: the unique key
		// refuses it and the WHOLE file rolls back — hence "chưa loại nào được tạo".
		h.d.Log.Error("nhập loại văn bản: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Chưa loại nào được tạo. Vui lòng kiểm tra lại tệp rồi thử lại.", "")
		return
	}
	vietJSON(w, http.StatusCreated, documentTypeImportCreatedOut{Created: documentTypeRowsOut(res.Types)})
}

// writeDocumentTypeImportRejected is 400 with every error — like every other refusal of what a client
// sent in this service.
func writeDocumentTypeImportRejected(w http.ResponseWriter, errs []domain.DocumentTypeImportError) {
	vietJSON(w, http.StatusBadRequest, documentTypeImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa loại nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  documentTypeErrorsOut(errs),
	})
}
