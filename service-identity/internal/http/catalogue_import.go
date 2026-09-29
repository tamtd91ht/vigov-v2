package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// "⬆ Nhập từ Excel" on two catalogues of Cấu hình → Danh mục (14-cau-hinh.md §5; user decision
// 2026-09-29, ADR 0059 §3) — three routes each, all `admin.lookup`, the key their create routes declare:
//
//	GET  /api/v1/{residential-unit-types|task-blocs}/import-template  the .xlsx to fill in
//	POST /api/v1/{residential-unit-types|task-blocs}/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/{residential-unit-types|task-blocs}/imports          write it, all or nothing
//
// THE SHAPE IS THE RESIDENTIAL-UNIT IMPORT'S (residential_unit_import.go): the same upload reader, the
// same core/xlsx reader (readImportSheet), the same wire shapes. The route decides the catalogue; the
// file cannot (ADR 0059 §3 — no `Nhóm` column).

// catalogueImportSpec is what differs between the two catalogues on the wire.
type catalogueImportSpec struct {
	sheet    string // the data sheet's tab name — a UI string (rule 12, invariant 2)
	filename string
	noun     string // "loại đơn vị dân cư" — in sentences only
	importer func(d Deps) CatalogueImporting
}

var (
	residentialUnitTypeImportSpec = catalogueImportSpec{
		sheet: "Loại đơn vị dân cư", filename: "mau-nhap-loai-don-vi-dan-cu.xlsx", noun: "loại đơn vị dân cư",
		importer: func(d Deps) CatalogueImporting { return d.ResidentialUnitTypeImports },
	}
	taskBlocImportSpec = catalogueImportSpec{
		sheet: "Khối nhiệm vụ", filename: "mau-nhap-khoi-nhiem-vu.xlsx", noun: "khối nhiệm vụ",
		importer: func(d Deps) CatalogueImporting { return d.TaskBlocImports },
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

// catalogueImportRejectedOut is httpx.Error plus the list, like residentialUnitImportRejectedOut.
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
// place is imported as a real entry with a permanent code. No dropdown either — every column is free
// text the server re-validates.
func catalogueTemplateSpec(s catalogueImportSpec) xlsx.TemplateSpec {
	return xlsx.TemplateSpec{
		SheetName: s.sheet,
		Header:    domain.CatalogueImportColumns(),
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT " + s.noun + " MỚI của xã. Không sửa, không đổi thứ tự dòng tiêu đề.",
			"Tên hiển thị: bắt buộc, không trùng tên một mục đang có của danh mục (kể cả mục đang tắt) và không trùng nhau trong tệp.",
			"Mã: để trống thì tự sinh từ tên (\"Khu phố\" → khu-pho); nếu nhập thì chỉ gồm chữ thường a-z, số và dấu gạch ngang. Mã đã cấp không bao giờ cấp lại, kể cả mã của mục đã xoá.",
			"Thứ tự: số nguyên 0–9999, để trống là 0.",
			"Mục nhập vào là mục của xã, đang dùng và KHÔNG là mặc định — chọn mặc định trên màn hình Danh mục.",
			"Tệp được nhập TOÀN BỘ HOẶC KHÔNG GÌ CẢ: chỉ cần một dòng lỗi thì không mục nào được tạo. Hãy bấm Kiểm tra trước khi Nhập.",
		},
		DropdownRows: domain.MaxCatalogueImportRows,
	}
}

func (h *Handler) catalogueImportTemplate(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) {
	b, err := xlsx.Template(catalogueTemplateSpec(s))
	if err != nil {
		h.d.Log.Error("mẫu nhập "+s.noun+": dựng tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", mimeXLSX)
	w.Header().Set("Content-Disposition", `attachment; filename="`+s.filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập "+s.noun+": gửi tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
}

// readCatalogueRows reads the upload into rows. ok=false means the response was already written;
// sheetErrs non-empty means the SHEET is not the template (answered by the caller).
func (h *Handler) readCatalogueRows(w http.ResponseWriter, r *http.Request, s catalogueImportSpec) ([]domain.CatalogueImportRow, []domain.CatalogueImportError, bool) {
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return nil, nil, false
	}
	cells, ok := h.readImportSheet(w, r, data, "nhập "+s.noun)
	if !ok {
		return nil, nil, false
	}
	rows, errs := domain.CatalogueRowsFromSheet(cells)
	return rows, errs, true
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
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
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
		case errors.Is(err, idstore.ErrMaDaTonTai):
			// A form took a code between the plan and the insert. The whole file was rolled back.
			httpx.WriteError(w, http.StatusConflict, "catalogue_changed",
				"Danh mục "+s.noun+" của xã vừa thay đổi. Chưa mục nào được tạo. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			h.d.Log.Error("nhập "+s.noun+": lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa mục nào được tạo. Vui lòng thử lại.", "")
		}
		return
	}
	vietJSON(w, http.StatusCreated, catalogueImportCreatedOut{Created: catalogueEntriesOut(res.Entries)})
}

// writeCatalogueImportRejected is 400 with every error — like writeResidentialUnitImportRejected.
func writeCatalogueImportRejected(w http.ResponseWriter, s catalogueImportSpec, errs []domain.CatalogueImportError) {
	vietJSON(w, http.StatusBadRequest, catalogueImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa " + s.noun + " nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  catalogueErrorsOut(errs),
	})
}

// ResidentialUnitTypeImportTemplate serves GET /api/v1/residential-unit-types/import-template.
func (h *Handler) ResidentialUnitTypeImportTemplate(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportTemplate(w, r, residentialUnitTypeImportSpec)
}

// PreviewResidentialUnitTypeImport serves POST /api/v1/residential-unit-types/import-previews.
func (h *Handler) PreviewResidentialUnitTypeImport(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportPreview(w, r, residentialUnitTypeImportSpec)
}

// ImportResidentialUnitTypes serves POST /api/v1/residential-unit-types/imports.
func (h *Handler) ImportResidentialUnitTypes(w http.ResponseWriter, r *http.Request) {
	h.catalogueImport(w, r, residentialUnitTypeImportSpec)
}

// TaskBlocImportTemplate serves GET /api/v1/task-blocs/import-template.
func (h *Handler) TaskBlocImportTemplate(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportTemplate(w, r, taskBlocImportSpec)
}

// PreviewTaskBlocImport serves POST /api/v1/task-blocs/import-previews.
func (h *Handler) PreviewTaskBlocImport(w http.ResponseWriter, r *http.Request) {
	h.catalogueImportPreview(w, r, taskBlocImportSpec)
}

// ImportTaskBlocs serves POST /api/v1/task-blocs/imports.
func (h *Handler) ImportTaskBlocs(w http.ResponseWriter, r *http.Request) {
	h.catalogueImport(w, r, taskBlocImportSpec)
}
