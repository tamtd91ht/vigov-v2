package http

// "⬆ Nhập từ Excel" on the two task catalogues of Cấu hình → Danh mục (14-cau-hinh.md §5; user decision
// 2026-09-29, ADR 0059 §3). Routes: routes_catalogue_import.go.
//
//	GET  /api/v1/{task-types|task-priorities}/import-template  the .xlsx to fill in
//	POST /api/v1/{task-types|task-priorities}/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/{task-types|task-priorities}/imports          write it, all or nothing
//
// THE SHAPE IS service-identity's catalogue import (fbbae7b) and service-comms' map-asset-type import
// (4c220a9): the same wire shapes, the same core/xlsx reader. The route decides the catalogue; the file
// cannot (ADR 0059 §3 — no `Nhóm` column).
//
// THE PREVIEW IS ITS OWN ROUTE, NOT A `dry_run` FLAG (unlike POST /api/v1/tasks/imports): the
// idempotency key is scoped to (commune, actor, method, PATH), never the body, so a flag on one path
// would let a client that previewed with key K and then imported with K be REPLAYED the preview —
// told the file was imported while nothing was written.
//
// THE UPLOAD NEVER TOUCHES DISK: MultipartReader part by part, never ParseMultipartForm (which spills a
// large part into a temporary file). The workbook is read by core/xlsx.ReadSheet, whose sentinels map
// to FIXED sentences below — never err.Error(), never a cell value (rule 3, forbidden #3).

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CatalogueImporting is the Excel import of ONE of the two task catalogues — its layout, the preview
// and the all-or-nothing import (app.CatalogueImporter). Two Deps fields, one per catalogue: the route
// decides the table, never the file.
type CatalogueImporting interface {
	Layout() domain.CatalogueImportLayout
	Preview(ctx context.Context, rows []domain.CatalogueImportRow) (app.CatalogueImportResult, error)
	Import(ctx context.Context, rows []domain.CatalogueImportRow, actor audit.Actor) (app.CatalogueImportResult, error)
}

const (
	catalogueFormField = "file"
	catalogueXLSXMIME  = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	// A constant rather than a literal: rbac_guard reads a header lookup by string literal as a route
	// declaration.
	catalogueHeaderContentType = "Content-Type"
)

// catalogueBodyMax is the file cap plus room for the multipart envelope; over it is 413 before a byte
// of the workbook is parsed. Read from core/xlsx so the two caps cannot drift apart.
var catalogueBodyMax = xlsx.DefaultLimits.MaxFileBytes + 64<<10

// catalogueImportSpec is what differs between the two catalogues on the wire.
type catalogueImportSpec struct {
	sheet    string // the data sheet's tab name — a UI string (rule 12, invariant 2)
	filename string
	noun     string // "loại nhiệm vụ" — in sentences only
	guide    []string
	importer func(d Deps) CatalogueImporting
}

// The guide lines both templates share; each spec adds its `Thứ tự` line in the right place.
func catalogueGuide(noun string, orderLine string) []string {
	return []string{
		"Mỗi dòng dưới dòng tiêu đề là MỘT " + noun + " MỚI của xã. Không sửa, không đổi thứ tự dòng tiêu đề.",
		"Tên hiển thị: bắt buộc, tối đa 200 ký tự, không trùng tên một mục đang có của danh mục (kể cả mục đang tắt) và không trùng nhau trong tệp.",
		"Mã: để trống thì tự sinh từ tên (\"Theo văn bản\" → theo-van-ban); nếu nhập thì chỉ gồm chữ thường a-z, số và dấu gạch ngang. Mã đã cấp không bao giờ cấp lại, kể cả mã của mục đã xoá.",
		orderLine,
		"Mục nhập vào là mục của xã, đang dùng và KHÔNG là mặc định — chọn mặc định trên màn hình Danh mục.",
		"Tệp được nhập TOÀN BỘ HOẶC KHÔNG GÌ CẢ: chỉ cần một dòng lỗi thì không mục nào được tạo. Hãy bấm Kiểm tra trước khi Nhập.",
	}
}

var (
	taskTypeImportSpec = catalogueImportSpec{
		sheet: "Loại nhiệm vụ", filename: "mau-nhap-loai-nhiem-vu.xlsx", noun: "loại nhiệm vụ",
		guide:    catalogueGuide("loại nhiệm vụ", "Thứ tự: số nguyên 0–9999, để trống là 0."),
		importer: func(d Deps) CatalogueImporting { return d.TaskTypeImports },
	}
	// NO `Thứ tự` COLUMN ON THE SCALE — domain.CatalogueOrderFromPosition: the order IS the meaning of a
	// priority scale, so the file's own row order ranks the new levels, after every existing one.
	taskPriorityImportSpec = catalogueImportSpec{
		sheet: "Mức ưu tiên", filename: "mau-nhap-muc-uu-tien-nhiem-vu.xlsx", noun: "mức ưu tiên",
		guide: catalogueGuide("mức ưu tiên", "Không có cột Thứ tự: các mức trong tệp được xếp SAU mọi mức đang có của xã, "+
			"theo đúng thứ tự dòng trong tệp (dòng trên xếp trước). Muốn chen một mức vào giữa, hãy sửa Thứ tự trên màn hình Danh mục sau khi nhập."),
		importer: func(d Deps) CatalogueImporting { return d.TaskPriorityImports },
	}
)

type catalogueImportErrorOut struct {
	Row     int    `json:"row"`    // dòng trong bảng tính (tiêu đề là dòng 1); 0 = lỗi của cả tệp
	Column  string `json:"column"` // tên cột như trên tiêu đề; "" = lỗi của cả dòng hoặc cả tệp
	Message string `json:"message"`
}

// catalogueImportEntryOut is one planned (preview) or created (import) row.
type catalogueImportEntryOut struct {
	Row   int    `json:"row"`
	ID    string `json:"id,omitempty"` // only once created
	Code  string `json:"code"`
	Label string `json:"label"`
	Order int    `json:"order"`
}

// catalogueImportPreviewOut — `valid` false means the import would be refused with these errors.
type catalogueImportPreviewOut struct {
	Valid   bool                      `json:"valid"`
	Entries []catalogueImportEntryOut `json:"entries"`
	Errors  []catalogueImportErrorOut `json:"errors"`
}

type catalogueImportCreatedOut struct {
	Created []catalogueImportEntryOut `json:"created"`
}

// catalogueImportRejectedOut is httpx.Error plus the list.
type catalogueImportRejectedOut struct {
	Code    string                    `json:"code"`
	Message string                    `json:"message"`
	TraceID string                    `json:"trace_id"`
	Errors  []catalogueImportErrorOut `json:"errors"`
}

func catalogueErrorsOut(in []domain.CatalogueImportError) []catalogueImportErrorOut {
	out := make([]catalogueImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, catalogueImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func catalogueEntriesOut(in []domain.PlannedCatalogueEntry) []catalogueImportEntryOut {
	out := make([]catalogueImportEntryOut, 0, len(in))
	for _, p := range in {
		out = append(out, catalogueImportEntryOut{Row: p.Row, ID: p.ID, Code: p.Code, Label: p.Label, Order: p.Order})
	}
	return out
}

// catalogueTemplateSpec builds the template. NO EXAMPLE ROW ON THE DATA SHEET: an example left in
// place is imported as a real entry with a permanent code. No dropdown — every column is free text the
// server re-validates.
func (h *Handler) catalogueTemplateSpec(s catalogueImportSpec) xlsx.TemplateSpec {
	return xlsx.TemplateSpec{
		SheetName:    s.sheet,
		Header:       s.importer(h.d).Layout().Columns(),
		Guide:        s.guide,
		DropdownRows: domain.MaxCatalogueImportRows,
	}
}

func (h *Handler) catalogueImportTemplate(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) {
	b, err := xlsx.Template(h.catalogueTemplateSpec(s))
	if err != nil {
		h.d.Log.Error("mẫu nhập "+s.noun+": dựng tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set(catalogueHeaderContentType, catalogueXLSXMIME)
	w.Header().Set("Content-Disposition", `attachment; filename="`+s.filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập "+s.noun+": gửi tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
}

// readCatalogueUpload reads the one `file` part into memory, or answers and returns false.
//
//	415  not multipart/form-data; a part type other than xlsx / octet-stream / none; a name not .xlsx
//	413  the body or the file is over its cap
//	400  no `file` part, or two of them
//
// THE FILE NAME IS READ ONLY FOR ITS SUFFIX and never logged or echoed: it may carry personal data
// (rule 3, forbidden #4).
func readCatalogueUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	mt, _, err := mime.ParseMediaType(r.Header.Get(catalogueHeaderContentType))
	if err != nil || mt != "multipart/form-data" {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Hãy gửi tệp dưới dạng multipart/form-data, trường \"file\".", "")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, catalogueBodyMax)
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
			writeCatalogueUploadError(w, err)
			return nil, false
		}
		if part.FormName() != catalogueFormField {
			// Drained, not kept: bounded by the body cap either way.
			if _, err := io.Copy(io.Discard, part); err != nil {
				writeCatalogueUploadError(w, err)
				return nil, false
			}
			continue
		}
		if data != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ gửi MỘT tệp mỗi lần nhập.", "")
			return nil, false
		}
		if !acceptableCatalogueXLSXPart(part.Header.Get(catalogueHeaderContentType), part.FileName()) {
			httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
				"Chỉ nhận tệp Excel .xlsx (không nhận .xls, .xlsm có macro hay .csv). Hãy tải tệp mẫu và nhập vào đó.", "")
			return nil, false
		}
		b, err := io.ReadAll(io.LimitReader(part, xlsx.DefaultLimits.MaxFileBytes+1))
		if err != nil {
			writeCatalogueUploadError(w, err)
			return nil, false
		}
		if int64(len(b)) > xlsx.DefaultLimits.MaxFileBytes {
			writeCatalogueFileTooLarge(w)
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

func acceptableCatalogueXLSXPart(contentType, fileName string) bool {
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
	return mt == catalogueXLSXMIME || mt == "application/octet-stream"
}

func writeCatalogueFileTooLarge(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
		"Tệp quá lớn. Tối đa 2 MB và 100 dòng mỗi lần nhập.", "")
}

func writeCatalogueUploadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeCatalogueFileTooLarge(w)
		return
	}
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Nội dung gửi lên không đọc được.", "")
}

// readCatalogueRows reads the upload into rows. ok=false means the response was already written;
// sheetErrs non-empty means the SHEET is not the template (answered by the caller).
//
// AN EMPTY SHEET IS A CONTENT ERROR, not a file refusal — "Tệp không có dòng nào.", like identity.
// EVERY SENTINEL HAS ONE FIXED SENTENCE: err.Error() is never written where a client or a log sees it.
func (h *Handler) readCatalogueRows(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) (
	[]domain.CatalogueImportRow, []domain.CatalogueImportError, bool) {

	data, ok := readCatalogueUpload(w, r)
	if !ok {
		return nil, nil, false
	}
	cells, err := xlsx.ReadSheet(bytes.NewReader(data), int64(len(data)), xlsx.DefaultLimits)
	switch {
	case err == nil, errors.Is(err, xlsx.ErrEmptySheet):
		rows, errs := domain.CatalogueRowsFromSheet(cells, s.importer(h.d).Layout())
		return rows, errs, true
	case errors.Is(err, xlsx.ErrTooLarge), errors.Is(err, xlsx.ErrTooManyRows):
		writeCatalogueFileTooLarge(w)
	case errors.Is(err, xlsx.ErrMacroEnabled):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp có macro (.xlsm) không được nhận. Hãy lưu lại dưới dạng Excel Workbook (.xlsx) không macro.", "")
	case errors.Is(err, xlsx.ErrNotXLSX):
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_file_type",
			"Tệp không phải bảng tính Excel .xlsx hợp lệ (hoặc đang đặt mật khẩu). Hãy tải tệp mẫu và nhập vào đó.", "")
	case errors.Is(err, xlsx.ErrMalformed):
		httpx.WriteError(w, http.StatusBadRequest, "malformed_file",
			"Tệp Excel bị hỏng hoặc không đọc trọn được. Hãy mở tệp bằng Excel, lưu lại rồi gửi lại.", "")
	default:
		// No `err` in the log line: it may wrap a read of the request body, and nothing more is known.
		h.d.Log.Error("nhập "+s.noun+": đọc tệp lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return nil, nil, false
}

// catalogueImportPreview — 200 whether or not the file is valid: the list of errors IS the answer.
func (h *Handler) catalogueImportPreview(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) {
	rows, sheetErrs, ok := h.readCatalogueRows(w, r, s)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		vietJSON(w, http.StatusOK, catalogueImportPreviewOut{
			Entries: []catalogueImportEntryOut{}, Errors: catalogueErrorsOut(sheetErrs)})
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := s.importer(h.d).Preview(r.Context(), rows)
	if err != nil {
		h.d.Log.Error("nhập "+s.noun+": xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, catalogueImportPreviewOut{
		Valid:   len(res.Errors) == 0,
		Entries: catalogueEntriesOut(res.Entries),
		Errors:  catalogueErrorsOut(res.Errors),
	})
}

func (h *Handler) catalogueImport(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến nhập "+s.noun+" chạy mà không có mã cán bộ — SAI CẤU HÌNH ROUTE hoặc định danh cũ",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	rows, sheetErrs, ok := h.readCatalogueRows(w, r, s)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		writeCatalogueImportRejected(w, s, sheetErrs)
		return
	}
	res, err := s.importer(h.d).Import(r.Context(), rows, actor)
	if err != nil {
		var rej *app.CatalogueImportRejected
		switch {
		case errors.As(err, &rej):
			writeCatalogueImportRejected(w, s, rej.Errors)
		case errors.Is(err, docstore.ErrMaDaTonTai):
			// A form took a code between the plan and the insert. The whole file was rolled back.
			httpx.WriteError(w, http.StatusConflict, "catalogue_changed",
				"Danh mục "+s.noun+" của xã vừa thay đổi. Chưa mục nào được tạo. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			h.d.Log.Error("nhập "+s.noun+": lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa mục nào được tạo. Vui lòng thử lại.", "")
		}
		return
	}
	// What a retry carrying the same Idempotency-Key is told: THE BATCH, never the body (core/idem
	// stores a code, not a response). Every entry of this file carries it as `lo_nhap`.
	idem.RecordCode(r.Context(), res.Batch)
	vietJSON(w, http.StatusCreated, catalogueImportCreatedOut{Created: catalogueEntriesOut(res.Entries)})
}

// writeCatalogueImportRejected is 400 with every error.
func writeCatalogueImportRejected(w http.ResponseWriter, s catalogueImportSpec, errs []domain.CatalogueImportError) {
	vietJSON(w, http.StatusBadRequest, catalogueImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa " + s.noun + " nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  catalogueErrorsOut(errs),
	})
}

// TaskTypeImportTemplate serves GET /api/v1/task-types/import-template.
func (h *Handler) TaskTypeImportTemplate(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportTemplate(w, r, taskTypeImportSpec)
}

// PreviewTaskTypeImport serves POST /api/v1/task-types/import-previews.
func (h *Handler) PreviewTaskTypeImport(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportPreview(w, r, taskTypeImportSpec)
}

// ImportTaskTypes serves POST /api/v1/task-types/imports.
func (h *Handler) ImportTaskTypes(w http.ResponseWriter, r *http.Request) {
	h.catalogueImport(w, r, taskTypeImportSpec)
}

// TaskPriorityImportTemplate serves GET /api/v1/task-priorities/import-template.
func (h *Handler) TaskPriorityImportTemplate(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportTemplate(w, r, taskPriorityImportSpec)
}

// PreviewTaskPriorityImport serves POST /api/v1/task-priorities/import-previews.
func (h *Handler) PreviewTaskPriorityImport(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportPreview(w, r, taskPriorityImportSpec)
}

// ImportTaskPriorities serves POST /api/v1/task-priorities/imports.
func (h *Handler) ImportTaskPriorities(w http.ResponseWriter, r *http.Request) {
	h.catalogueImport(w, r, taskPriorityImportSpec)
}
