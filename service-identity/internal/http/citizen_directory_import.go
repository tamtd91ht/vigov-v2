package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// "⬆ Nhập từ Excel" on Danh bạ người dân (chapter 16 §7; ADR 0097 D3, D13) — three routes, all
// `citizen.update`, the key the add form declares:
//
//	GET  /api/v1/citizen-contact-import-template   the .xlsx to fill in, one dropdown (Thôn, tổ dân phố)
//	POST /api/v1/citizen-contact-import-previews   check a filled file — WRITES NOTHING, audits nothing
//	POST /api/v1/citizen-contact-imports           write it, all or nothing; ONE audit entry for the run
//
// TOP-LEVEL NOUNS, not `citizen-contacts/import-*`: `citizen-contacts/{id}` owns the second segment,
// and a person whose id is `imports` is the reading `staff-counts` avoided for the same reason.
//
// THE SHAPE IS THE STAFF IMPORT'S (staff_import.go): the upload is read by readImportUpload (multipart,
// one `file` part, in memory, capped), the sheet by readImportSheet (core/xlsx's hardened reader), the
// rows by the domain and the app layer. A rejected file is 400 `import_invalid` with every row error —
// what every import of this repository answers (staff, residential units, citizen letters).
//
// NO PERSONAL DATA ON THE WIRE BEYOND THE FILE ITSELF. The preview reports the row count and the
// errors — row and column, never the cell (rule 3, forbidden #3) — and does NOT echo the rows back:
// the person who uploaded the file already has the file, and a list of fifty names and numbers in a
// response is a second copy nobody masks. The 201 carries the batch id and the row count.
//
// THE RESIDENTIAL UNITS come from the same store the list screen reads (ThonToDanPho), ACTIVE ONLY: a
// unit taken out of use is not offered in the dropdown and a cell naming it is refused by the app
// layer (resolveImportResidentialUnit sees only what is passed here).

type citizenContactImportErrorOut struct {
	Row     int    `json:"row"`    // dòng trong bảng tính (tiêu đề là dòng 1); 0 = lỗi của cả tệp
	Column  string `json:"column"` // tên cột như trên tiêu đề; "" = lỗi của cả dòng hoặc cả tệp
	Message string `json:"message"`
}

// citizenContactImportPreviewOut — `valid` false means the import would be refused with these errors.
// `row_count` is the number of data rows the sheet holds (0 when the file itself was refused).
type citizenContactImportPreviewOut struct {
	Valid    bool                           `json:"valid"`
	RowCount int                            `json:"row_count"`
	Errors   []citizenContactImportErrorOut `json:"errors"`
}

// citizenContactImportCreatedOut is the 201: the batch every imported row carries (`import_batch_id`,
// also what an idempotent replay returns as `code`) and how many rows were written.
type citizenContactImportCreatedOut struct {
	BatchID  string `json:"batch_id"`
	RowCount int    `json:"row_count"`
}

// citizenContactImportRejectedOut is httpx.Error plus the list, like staffImportRejectedOut.
type citizenContactImportRejectedOut struct {
	Code    string                         `json:"code"`
	Message string                         `json:"message"`
	TraceID string                         `json:"trace_id"`
	Errors  []citizenContactImportErrorOut `json:"errors"`
}

func citizenContactImportErrorsOut(in []domain.CitizenContactImportError) []citizenContactImportErrorOut {
	out := make([]citizenContactImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, citizenContactImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

// citizenContactImportTemplateSpec builds the template from the domain's pieces: the seven columns,
// the one example row (the agreed fake number, rule 3 invariant 5), the guide (which carries
// domain.TagSensitiveWarning) and the residential-unit dropdown. The dropdown WARNS rather than
// refuses (Strict false): a unit may also be typed by code or name, and the server re-validates.
func citizenContactImportTemplateSpec(units []app.ImportResidentialUnit) xlsx.TemplateSpec {
	values := make([]string, 0, len(units))
	for _, u := range units {
		values = append(values, domain.ResidentialUnitChoiceLabel(u.Code, u.Name))
	}
	return xlsx.TemplateSpec{
		SheetName: domain.CitizenContactImportSheetName,
		Header:    domain.CitizenContactImportColumns(),
		Examples:  [][]string{domain.CitizenContactImportExampleRow()},
		Guide:     domain.CitizenContactImportGuide(),
		Choices: map[int]xlsx.Choices{
			domain.CitizenContactImportResidentialUnitColumn: {
				Values: values, Title: "Thôn, tổ dân phố", Message: "Không phải thôn / tổ dân phố đang dùng của xã."},
		},
		DropdownRows: domain.MaxCitizenContactImportRows,
	}
}

// importResidentialUnits lists the commune's ACTIVE residential units for the template and the two
// import routes — the one place the filter lives, so the dropdown and the resolver agree.
func (h *Handler) importResidentialUnits(w http.ResponseWriter, r *http.Request, what string) ([]app.ImportResidentialUnit, bool) {
	ctx := r.Context()
	// The store reads through store.DB.For(ctx): tenant_id comes from the context, never a parameter.
	all, err := h.d.ThonToDanPho.DanhSach(ctx)
	if err != nil {
		h.d.Log.Error(what+": đọc thôn/tổ dân phố lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return nil, false
	}
	units := make([]app.ImportResidentialUnit, 0, len(all))
	for _, u := range all {
		if !u.DangDung {
			continue
		}
		units = append(units, app.ImportResidentialUnit{ID: u.ID, Code: u.Ma, Name: u.Ten})
	}
	return units, true
}

// CitizenContactImportTemplate serves GET /api/v1/citizen-contact-import-template.
func (h *Handler) CitizenContactImportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	units, ok := h.importResidentialUnits(w, r, "mẫu nhập danh bạ người dân")
	if !ok {
		return
	}
	b, err := xlsx.Template(citizenContactImportTemplateSpec(units))
	if err != nil {
		h.d.Log.Error("mẫu nhập danh bạ người dân: dựng tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", mimeXLSX)
	w.Header().Set("Content-Disposition", `attachment; filename="mau-nhap-danh-ba-nguoi-dan.xlsx"`)
	// The dropdown inside is this commune's unit list as of now: never cached in between.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập danh bạ người dân: gửi tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
}

// readCitizenContactSheet reads the upload into cells, or answers the refusal of the FILE and
// returns false. An empty sheet comes back as nil cells: the domain reader reports it as a content
// error ("Tệp không có dòng tiêu đề.").
func (h *Handler) readCitizenContactSheet(w http.ResponseWriter, r *http.Request) ([][]string, bool) {
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return nil, false
	}
	return h.readImportSheet(w, r, data, "nhập danh bạ người dân")
}

// PreviewCitizenContactImport serves POST /api/v1/citizen-contact-import-previews. 200 whether or not
// the file is valid: the list of errors IS the answer. Nothing is written and nothing is audited; the
// use case rolls its transaction back on purpose.
func (h *Handler) PreviewCitizenContactImport(w http.ResponseWriter, r *http.Request) {
	sheet, ok := h.readCitizenContactSheet(w, r)
	if !ok {
		return
	}
	units, ok := h.importResidentialUnits(w, r, "nhập danh bạ người dân: xem trước")
	if !ok {
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.CitizenDirectory.ImportPreview(r.Context(), sheet, units)
	if err != nil {
		h.d.Log.Error("nhập danh bạ người dân: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	// The error messages name rows and columns only, but the response describes a file of personal
	// data: not cached in between, like the staff preview.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusOK, citizenContactImportPreviewOut{
		Valid:    len(res.Errors) == 0,
		RowCount: res.RowCount,
		Errors:   citizenContactImportErrorsOut(res.Errors),
	})
}

// ImportCitizenContacts serves POST /api/v1/citizen-contact-imports. ALL OR NOTHING: a row error
// anywhere is 400 with every error and nothing written.
func (h *Handler) ImportCitizenContacts(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	sheet, ok := h.readCitizenContactSheet(w, r)
	if !ok {
		return
	}
	units, ok := h.importResidentialUnits(w, r, "nhập danh bạ người dân")
	if !ok {
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context; every
	// row and the one audit entry share that transaction.
	res, err := h.d.CitizenDirectory.ImportCommit(r.Context(), actor, sheet, units)
	if err != nil {
		var rej *app.CitizenContactImportRejected
		switch {
		case errors.As(err, &rej):
			writeCitizenContactImportRejected(w, rej.Errors)
		case errors.Is(err, idstore.ErrCitizenPhoneTaken), errors.Is(err, idstore.ErrCitizenGroupNotFound),
			errors.Is(err, idstore.ErrCitizenContactUnitUnknown):
			// The commune changed between the plan and the insert — a number, a group or a unit the
			// plan saw is gone or taken. The whole file was rolled back.
			httpx.WriteError(w, http.StatusConflict, "citizen_contacts_changed",
				"Danh bạ, danh sách nhóm hoặc thôn / tổ dân phố của xã vừa thay đổi. Chưa ai được thêm. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			h.d.Log.Error("nhập danh bạ người dân: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa ai được thêm. Vui lòng thử lại.", "")
		}
		return
	}
	// A retry with the same Idempotency-Key replays THIS code (core/idem stores no body).
	idem.RecordCode(r.Context(), res.Batch)
	vietJSON(w, http.StatusCreated, citizenContactImportCreatedOut{BatchID: res.Batch, RowCount: res.RowCount})
}

// writeCitizenContactImportRejected is 400 with every error — like writeStaffImportRejected.
func writeCitizenContactImportRejected(w http.ResponseWriter, errs []domain.CitizenContactImportError) {
	vietJSON(w, http.StatusBadRequest, citizenContactImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa ai được thêm vào danh bạ. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  citizenContactImportErrorsOut(errs),
	})
}
