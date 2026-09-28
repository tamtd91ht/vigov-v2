package http

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/orgunitxlsx"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// "Nhập từ Excel" on the org chart (14-cau-hinh.md §1; user decision 2026-09-28) — three routes, all
// `admin.org`, the key the form's own create route declares:
//
//	GET  /api/v1/org-units/import-template  the .xlsx to fill in, with a dropdown of live units
//	POST /api/v1/org-units/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/org-units/imports          write it, all or nothing
//
// THE PREVIEW IS ITS OWN ROUTE, NOT A `dry_run` FLAG ON THE IMPORT, and the reason is idem: the
// duplicate-request key is scoped to (commune, actor, method, PATH) — never the query or the body. A
// flag on one path would let a client that previewed with key K and then imported with the same K
// be REPLAYED the preview's 2xx: told the file was imported while nothing was written. Two paths
// make that impossible by construction.
//
// THE UPLOAD NEVER TOUCHES DISK. The body is read through MultipartReader part by part, not
// ParseMultipartForm — which spills any file part over its memory budget into a temporary file.

// Upload bounds. The body cap is the file cap plus room for the multipart envelope; a body over it
// is 413 before a byte of the workbook is parsed.
const (
	importFormField = "file"
	importBodyMax   = orgunitxlsx.MaxFileBytes + 64<<10
	mimeXLSX        = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	// A constant rather than a literal: rbac_guard reads a header lookup by string literal as a
	// route declaration.
	headerContentType = "Content-Type"
)

type orgUnitImportErrorOut struct {
	Row     int    `json:"row"`    // dòng trong bảng tính (tiêu đề là dòng 1); 0 = lỗi của cả tệp
	Column  string `json:"column"` // tên cột như trên tiêu đề; "" = lỗi của cả dòng hoặc cả tệp
	Message string `json:"message"`
}

// orgUnitImportUnitOut is one planned (preview) or created (import) unit.
type orgUnitImportUnitOut struct {
	Row  int    `json:"row"`
	ID   string `json:"id,omitempty"` // only once created
	Code string `json:"code"`
	Name string `json:"name"`
	// ParentID is the parent's id: an existing unit's on a preview, any parent's once created. ""
	// on a preview whose parent is a row of the same file — ParentRow names it then.
	ParentID   string `json:"parent_id"`
	ParentRow  int    `json:"parent_row,omitempty"`
	ParentCode string `json:"parent_code"`
	Order      int    `json:"order"`
}

// orgUnitImportPreviewOut — `valid` false means the import would be refused with exactly these errors.
type orgUnitImportPreviewOut struct {
	Valid  bool                    `json:"valid"`
	Units  []orgUnitImportUnitOut  `json:"units"`
	Errors []orgUnitImportErrorOut `json:"errors"`
}

type orgUnitImportCreatedOut struct {
	Created []orgUnitImportUnitOut `json:"created"`
}

// orgUnitImportRejectedOut is httpx.Error plus the list: the three fields every failure carries, so
// a client reading `code` and `message` reads this one the same way.
type orgUnitImportRejectedOut struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	TraceID string                  `json:"trace_id"`
	Errors  []orgUnitImportErrorOut `json:"errors"`
}

func importErrorsOut(in []domain.OrgUnitImportError) []orgUnitImportErrorOut {
	out := make([]orgUnitImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, orgUnitImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func importUnitsOut(in []domain.PlannedOrgUnit) []orgUnitImportUnitOut {
	out := make([]orgUnitImportUnitOut, 0, len(in))
	for _, p := range in {
		out = append(out, orgUnitImportUnitOut{
			Row: p.Row, ID: p.Unit.ID, Code: p.Unit.Ma, Name: p.Unit.Ten,
			ParentID: p.Unit.ChaID, ParentRow: p.ParentRow, ParentCode: p.ParentCode, Order: p.Unit.ThuTu,
		})
	}
	return out
}

// OrgUnitImportTemplate serves GET /api/v1/org-units/import-template.
func (h *Handler) OrgUnitImportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	live, err := h.d.BoPhan.DanhSach(ctx)
	if err != nil {
		h.d.Log.Error("mẫu nhập sơ đồ tổ chức: đọc danh mục lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	b, err := orgunitxlsx.Template(live)
	if err != nil {
		h.d.Log.Error("mẫu nhập sơ đồ tổ chức: dựng tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", mimeXLSX)
	w.Header().Set("Content-Disposition", `attachment; filename="mau-nhap-so-do-to-chuc.xlsx"`)
	// The list inside is this commune's chart as of now: never cached by anything in between.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập sơ đồ tổ chức: gửi tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
}

// readImportUpload reads the one `file` part into memory, or answers and returns false.
//
//	415  the request is not multipart/form-data; the part declares a type other than the xlsx one
//	     (a browser on a machine with no mapping sends application/octet-stream, so that and an
//	     absent type are accepted — the CONTENT is what orgunitxlsx.Parse checks); a file name that
//	     is not .xlsx
//	413  the body or the file is over its cap
//	400  no `file` part, or two of them
func (h *Handler) readImportUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	mt, _, err := mime.ParseMediaType(r.Header.Get(headerContentType))
	if err != nil || mt != "multipart/form-data" {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Hãy gửi tệp dưới dạng multipart/form-data, trường \"file\".", "")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, importBodyMax)
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
			h.writeUploadReadError(w, err)
			return nil, false
		}
		if part.FormName() != importFormField {
			// Drained, not kept: bounded by the body cap either way.
			if _, err := io.Copy(io.Discard, part); err != nil {
				h.writeUploadReadError(w, err)
				return nil, false
			}
			continue
		}
		if data != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ gửi MỘT tệp mỗi lần nhập.", "")
			return nil, false
		}
		if !acceptablePart(part.Header.Get(headerContentType), part.FileName()) {
			httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
				"Chỉ nhận tệp Excel .xlsx (không nhận .xls, .xlsm có macro hay .csv). Hãy tải tệp mẫu và nhập vào đó.", "")
			return nil, false
		}
		b, err := io.ReadAll(io.LimitReader(part, orgunitxlsx.MaxFileBytes+1))
		if err != nil {
			h.writeUploadReadError(w, err)
			return nil, false
		}
		if len(b) > orgunitxlsx.MaxFileBytes {
			writeFileTooLarge(w)
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

func acceptablePart(contentType, fileName string) bool {
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
	return mt == mimeXLSX || mt == "application/octet-stream"
}

func writeFileTooLarge(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
		"Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập.", "")
}

func (h *Handler) writeUploadReadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeFileTooLarge(w)
		return
	}
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Nội dung gửi lên không đọc được.", "")
}

// parseImportFile turns the bytes into rows, or answers the refusal of the FILE and returns false.
// Errors of the CONTENT (a wrong header) come back in the second value for the caller to report.
func (h *Handler) parseImportFile(w http.ResponseWriter, r *http.Request, data []byte) ([]domain.OrgUnitImportRow, []domain.OrgUnitImportError, bool) {
	rows, rowErrs, err := orgunitxlsx.Parse(data)
	switch {
	case err == nil:
		return rows, rowErrs, true
	case errors.Is(err, orgunitxlsx.ErrWorkbookTooLarge):
		writeFileTooLarge(w)
	case errors.Is(err, orgunitxlsx.ErrMacroEnabled):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp có macro (.xlsm) không được nhận. Hãy lưu lại dưới dạng Excel Workbook (.xlsx) không macro.", "")
	case errors.Is(err, orgunitxlsx.ErrNotXLSX):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp không phải bảng tính Excel .xlsx hợp lệ (hoặc đang đặt mật khẩu). Hãy tải tệp mẫu và nhập vào đó.", "")
	default:
		h.d.Log.Error("nhập sơ đồ tổ chức: đọc tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return nil, nil, false
}

// PreviewOrgUnitImport serves POST /api/v1/org-units/import-previews. 200 whether or not the file
// is valid: the list of errors IS the answer to "what is wrong with my file".
func (h *Handler) PreviewOrgUnitImport(w http.ResponseWriter, r *http.Request) {
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return
	}
	rows, rowErrs, ok := h.parseImportFile(w, r, data)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		vietJSON(w, http.StatusOK, orgUnitImportPreviewOut{Units: []orgUnitImportUnitOut{}, Errors: importErrorsOut(rowErrs)})
		return
	}
	res, err := h.d.OrgUnitImports.Preview(r.Context(), rows)
	if err != nil {
		h.d.Log.Error("nhập sơ đồ tổ chức: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, orgUnitImportPreviewOut{
		Valid:  len(res.Errors) == 0,
		Units:  importUnitsOut(res.Units),
		Errors: importErrorsOut(res.Errors),
	})
}

// ImportOrgUnits serves POST /api/v1/org-units/imports.
func (h *Handler) ImportOrgUnits(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return
	}
	rows, rowErrs, ok := h.parseImportFile(w, r, data)
	if !ok {
		return
	}
	if len(rowErrs) > 0 {
		writeImportRejected(w, rowErrs)
		return
	}

	res, err := h.d.OrgUnitImports.Import(r.Context(), rows, actor)
	if err != nil {
		var rej *app.OrgUnitImportRejected
		switch {
		case errors.As(err, &rej):
			writeImportRejected(w, rej.Errors)
		case errors.Is(err, idstore.ErrMaBoPhanDaDung), errors.Is(err, idstore.ErrBoPhanChaKhongTonTai):
			// The chart changed between the plan and the insert — a concurrent creation took a code, a
			// parent went away. The whole file was rolled back; a new preview shows the new state.
			httpx.WriteError(w, http.StatusConflict, "org_chart_changed",
				"Sơ đồ tổ chức của xã vừa được người khác thay đổi. Chưa bộ phận nào được tạo. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			h.d.Log.Error("nhập sơ đồ tổ chức: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa bộ phận nào được tạo. Vui lòng thử lại.", "")
		}
		return
	}
	vietJSON(w, http.StatusCreated, orgUnitImportCreatedOut{Created: importUnitsOut(res.Units)})
}

// writeImportRejected is 400 with every error. 400 and not 422, like every other refusal of what a
// client sent in this service.
func writeImportRejected(w http.ResponseWriter, errs []domain.OrgUnitImportError) {
	vietJSON(w, http.StatusBadRequest, orgUnitImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa bộ phận nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  importErrorsOut(errs),
	})
}
