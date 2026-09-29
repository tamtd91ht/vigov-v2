package http

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// "⬆ Nhập từ Excel" on the residential-unit tab (14-cau-hinh.md §2; user decision 2026-09-29, ADR 0059
// §2) — three routes, all `admin.org`, the key the create route declares:
//
//	GET  /api/v1/residential-units/import-template  the .xlsx to fill in, with two dropdowns
//	POST /api/v1/residential-units/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/residential-units/imports          write it, all or nothing
//
// THE SHAPE IS THE ORG-CHART IMPORT'S (org_unit_import.go), and the upload is read by the same
// readImportUpload — multipart, one `file` part, in memory only, capped. What differs is the reader:
// core/xlsx.ReadSheet, the one hardened reader every import shares. Its errors are sentinels, each
// mapped below to ONE fixed sentence; no cell text, part name or error string from the file ever
// reaches a response or a log (rule 3, forbidden #3).

// residentialUnitSheet is the data sheet's tab name — a UI string (rule 12, invariant 2).
const residentialUnitSheet = "Thôn - Tổ dân phố"

type residentialUnitImportErrorOut struct {
	Row     int    `json:"row"`    // dòng trong bảng tính (tiêu đề là dòng 1); 0 = lỗi của cả tệp
	Column  string `json:"column"` // tên cột như trên tiêu đề; "" = lỗi của cả dòng hoặc cả tệp
	Message string `json:"message"`
}

// residentialUnitImportUnitOut is one planned (preview) or created (import) unit.
type residentialUnitImportUnitOut struct {
	Row             int    `json:"row"`
	ID              string `json:"id,omitempty"` // only once created
	Code            string `json:"code"`
	Name            string `json:"name"`
	TypeCode        string `json:"type_code"`
	TypeLabel       string `json:"type_label"`
	HeadStaffCode   string `json:"head_staff_code"`
	HeadStaffName   string `json:"head_staff_name"`
	HouseholdCount  *int   `json:"household_count"`
	PopulationCount *int   `json:"population_count"`
	Order           int    `json:"order"`
}

// residentialUnitImportPreviewOut — `valid` false means the import would be refused with these errors.
type residentialUnitImportPreviewOut struct {
	Valid  bool                            `json:"valid"`
	Units  []residentialUnitImportUnitOut  `json:"units"`
	Errors []residentialUnitImportErrorOut `json:"errors"`
}

type residentialUnitImportCreatedOut struct {
	Created []residentialUnitImportUnitOut `json:"created"`
}

// residentialUnitImportRejectedOut is httpx.Error plus the list, like orgUnitImportRejectedOut.
type residentialUnitImportRejectedOut struct {
	Code    string                          `json:"code"`
	Message string                          `json:"message"`
	TraceID string                          `json:"trace_id"`
	Errors  []residentialUnitImportErrorOut `json:"errors"`
}

func residentialUnitErrorsOut(in []domain.ResidentialUnitImportError) []residentialUnitImportErrorOut {
	out := make([]residentialUnitImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, residentialUnitImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func residentialUnitUnitsOut(in []domain.PlannedResidentialUnit) []residentialUnitImportUnitOut {
	out := make([]residentialUnitImportUnitOut, 0, len(in))
	for _, p := range in {
		u := p.Unit
		out = append(out, residentialUnitImportUnitOut{
			Row: p.Row, ID: u.ID, Code: u.Ma, Name: u.Ten, TypeCode: u.LoaiMa, TypeLabel: u.LoaiNhan,
			HeadStaffCode: u.HeadStaffCode, HeadStaffName: u.HeadStaffName,
			HouseholdCount: u.SoHo, PopulationCount: u.NhanKhau, Order: u.SortOrder,
		})
	}
	return out
}

// residentialUnitTemplateSpec builds the template. NO EXAMPLE ROW ON THE DATA SHEET: an example left in
// place by a busy clerk is imported as a real hamlet with a permanent code. The examples are sentences
// on the guide sheet instead.
//
// THE HEAD DROPDOWN HOLDS STAFF NAMES. The file is served only to `admin.org`, and carries exactly the
// code and name GET /api/v1/staff-directory already shows every account of the commune — no phone
// number, no email (rule 3). Both dropdowns WARN rather than refuse (Strict false): a type may also be
// typed by its code, and the server re-validates every cell anyway.
func residentialUnitTemplateSpec(types []domain.ResidentialUnitTypeChoice, staff []domain.HeadStaffChoice) xlsx.TemplateSpec {
	typeValues := make([]string, 0, len(types))
	for _, t := range types {
		typeValues = append(typeValues, t.Label)
	}
	headValues := make([]string, 0, len(staff))
	for _, s := range staff {
		headValues = append(headValues, domain.HeadStaffChoiceLabel(s.Code, s.Name))
	}
	return xlsx.TemplateSpec{
		SheetName: residentialUnitSheet,
		Header:    domain.ResidentialUnitImportColumns(),
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT thôn / tổ dân phố MỚI. Không sửa, không đổi thứ tự dòng tiêu đề.",
			"Tên: bắt buộc, không trùng tên đơn vị đang có của xã (kể cả đơn vị đã ngưng dùng) và không trùng nhau trong tệp.",
			"Loại: chọn trong danh sách (hoặc ghi mã loại); để trống nếu chưa phân loại.",
			"Trưởng thôn / Tổ trưởng: chọn trong danh sách — hệ thống chỉ đọc MÃ CÁN BỘ đứng trước dấu \"·\"; để trống nếu chưa có.",
			"Số hộ, Nhân khẩu: số nguyên, chỉ gồm chữ số (ví dụ 1132, không ghi 1.132). Để trống nghĩa là CHƯA NHẬP — không phải 0.",
			"Mã: để trống thì tự sinh từ tên (\"Thôn Bình An\" → thon-binh-an). Mã đã cấp không bao giờ cấp lại.",
			"Thứ tự: số nguyên 0–9999, để trống là 0.",
			"Tệp được nhập TOÀN BỘ HOẶC KHÔNG GÌ CẢ: chỉ cần một dòng lỗi thì không đơn vị nào được tạo. Hãy bấm Kiểm tra trước khi Nhập.",
		},
		Choices: map[int]xlsx.Choices{
			1: {Values: typeValues, Title: "Loại đơn vị dân cư", Message: "Giá trị không có trong danh mục loại đang dùng của xã."},
			2: {Values: headValues, Title: "Trưởng thôn / Tổ trưởng", Message: "Không phải cán bộ đang làm việc của xã."},
		},
		DropdownRows: domain.MaxResidentialUnitImportRows,
	}
}

// ResidentialUnitImportTemplate serves GET /api/v1/residential-units/import-template.
func (h *Handler) ResidentialUnitImportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// The store reads through store.DB.For(ctx): tenant_id comes from the context, never a parameter.
	types, staff, err := h.d.ResidentialUnitImports.TemplateChoices(ctx)
	if err != nil {
		h.d.Log.Error("mẫu nhập thôn/tổ dân phố: đọc danh sách chọn lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	b, err := xlsx.Template(residentialUnitTemplateSpec(types, staff))
	if err != nil {
		h.d.Log.Error("mẫu nhập thôn/tổ dân phố: dựng tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", mimeXLSX)
	w.Header().Set("Content-Disposition", `attachment; filename="mau-nhap-thon-to-dan-pho.xlsx"`)
	// The lists inside are this commune's as of now, and carry staff names: never cached in between.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập thôn/tổ dân phố: gửi tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
}

// parseResidentialUnitFile turns the bytes into rows, or answers the refusal of the FILE and returns
// false. Errors of the CONTENT (a wrong header, an empty sheet) come back in the second value.
//
// EVERY SENTINEL HAS ONE FIXED SENTENCE; err.Error() is never written anywhere a client or a log sees
// it, because nothing core/xlsx returns is allowed to carry file content — and this layer does not
// lean on that promise either.
func (h *Handler) parseResidentialUnitFile(w http.ResponseWriter, r *http.Request, data []byte) ([]domain.ResidentialUnitImportRow, []domain.ResidentialUnitImportError, bool) {
	cells, err := xlsx.ReadSheet(bytes.NewReader(data), int64(len(data)), xlsx.DefaultLimits)
	switch {
	case err == nil:
		rows, errs := domain.ResidentialUnitRowsFromSheet(cells)
		return rows, errs, true
	case errors.Is(err, xlsx.ErrEmptySheet):
		return nil, []domain.ResidentialUnitImportError{{Message: "Tệp không có dòng nào."}}, true
	case errors.Is(err, xlsx.ErrTooLarge), errors.Is(err, xlsx.ErrTooManyRows):
		writeFileTooLarge(w)
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
		h.d.Log.Error("nhập thôn/tổ dân phố: đọc tệp lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return nil, nil, false
}

// PreviewResidentialUnitImport serves POST /api/v1/residential-units/import-previews. 200 whether or
// not the file is valid: the list of errors IS the answer.
func (h *Handler) PreviewResidentialUnitImport(w http.ResponseWriter, r *http.Request) {
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return
	}
	rows, sheetErrs, ok := h.parseResidentialUnitFile(w, r, data)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		vietJSON(w, http.StatusOK, residentialUnitImportPreviewOut{
			Units: []residentialUnitImportUnitOut{}, Errors: residentialUnitErrorsOut(sheetErrs)})
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.ResidentialUnitImports.Preview(r.Context(), rows)
	if err != nil {
		h.d.Log.Error("nhập thôn/tổ dân phố: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, residentialUnitImportPreviewOut{
		Valid:  len(res.Errors) == 0,
		Units:  residentialUnitUnitsOut(res.Units),
		Errors: residentialUnitErrorsOut(res.Errors),
	})
}

// ImportResidentialUnits serves POST /api/v1/residential-units/imports.
func (h *Handler) ImportResidentialUnits(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return
	}
	rows, sheetErrs, ok := h.parseResidentialUnitFile(w, r, data)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		writeResidentialUnitImportRejected(w, sheetErrs)
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.ResidentialUnitImports.Import(r.Context(), rows, actor)
	if err != nil {
		var rej *app.ResidentialUnitImportRejected
		switch {
		case errors.As(err, &rej):
			writeResidentialUnitImportRejected(w, rej.Errors)
		case errors.Is(err, idstore.ErrResidentialUnitCodeTaken), errors.Is(err, idstore.ErrResidentialUnitTypeNotFound),
			errors.Is(err, idstore.ErrHeadStaffNotFound):
			// The commune changed between the plan and the insert. The whole file was rolled back.
			httpx.WriteError(w, http.StatusConflict, "residential_units_changed",
				"Danh sách thôn / tổ dân phố, danh mục loại hoặc danh sách cán bộ của xã vừa thay đổi. Chưa đơn vị nào được tạo. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			h.d.Log.Error("nhập thôn/tổ dân phố: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa đơn vị nào được tạo. Vui lòng thử lại.", "")
		}
		return
	}
	vietJSON(w, http.StatusCreated, residentialUnitImportCreatedOut{Created: residentialUnitUnitsOut(res.Units)})
}

// writeResidentialUnitImportRejected is 400 with every error — like writeImportRejected.
func writeResidentialUnitImportRejected(w http.ResponseWriter, errs []domain.ResidentialUnitImportError) {
	vietJSON(w, http.StatusBadRequest, residentialUnitImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa thôn / tổ dân phố nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  residentialUnitErrorsOut(errs),
	})
}
