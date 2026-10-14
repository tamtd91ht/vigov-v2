package http

// "Nhập lần giải ngân từ Excel" (docs/ui-ux/06-giai-ngan.md §10, §13 rule 7). Routes:
// routes_disbursement_import.go.
//
//	GET  /api/v1/disbursements/import-template  the .xlsx to fill in
//	POST /api/v1/disbursements/import-previews  check a filled file — WRITES NOTHING
//	POST /api/v1/disbursements/imports          write it, all or nothing
//
// THE SHAPE IS THIS SERVICE'S CATALOGUE IMPORT (catalogue_import.go): the same upload reader, the same
// core/xlsx sheet reader and limits, the same {row, column, message} errors, and the same reason the
// preview is its OWN ROUTE rather than a `?mode=dry-run` flag: the idempotency key is scoped to (commune,
// actor, method, PATH), never the query or body, so a flag on one path would let a client that
// previewed with key K and then imported with K be REPLAYED the preview — told its payments were
// recorded while nothing was written.

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// DisbursementImporting is the Excel import of disbursement vouchers (app.DisbursementImporter).
type DisbursementImporting interface {
	Preview(ctx context.Context, rows []domain.VoucherImportRow) (app.VoucherImportResult, error)
	Import(ctx context.Context, rows []domain.VoucherImportRow, actor audit.Actor) (app.VoucherImportResult, error)
	TemplateChoices(ctx context.Context) (app.VoucherTemplateChoices, error)
}

const (
	// The data sheet's tab name and the download's file name — UI strings (rule 12, invariant 2).
	disbursementImportSheet    = "Giải ngân"
	disbursementImportFilename = "mau-nhap-giai-ngan.xlsx"
)

// disbursementImportPreviewOut — `valid` false means the import would be refused with these errors.
// `total_amount` counts only rows whose amount was readable; with `valid` true it is exactly what the
// import would add to "đã giải ngân".
type disbursementImportPreviewOut struct {
	Valid       bool                      `json:"valid"`
	RowCount    int                       `json:"row_count"`    // số dòng dữ liệu đọc được (bỏ dòng trống)
	TotalAmount int64                     `json:"total_amount"` // đồng
	Errors      []catalogueImportErrorOut `json:"errors"`
}

// disbursementImportCreatedRowOut is one voucher the import wrote.
type disbursementImportCreatedRowOut struct {
	Row         int    `json:"row"`
	ID          string `json:"id"`
	ProjectCode string `json:"project_code"`
}

// disbursementImportCreatedOut is the 201: the preview's figures plus what was written. `batch` is the
// `lo_nhap` every audit entry of this file carries — and what a retry with the same key is told.
type disbursementImportCreatedOut struct {
	Valid       bool                              `json:"valid"`
	RowCount    int                               `json:"row_count"`
	TotalAmount int64                             `json:"total_amount"`
	Errors      []catalogueImportErrorOut         `json:"errors"`
	Batch       string                            `json:"batch"`
	Created     []disbursementImportCreatedRowOut `json:"created"`
}

func voucherErrorsOut(in []domain.VoucherImportError) []catalogueImportErrorOut {
	out := make([]catalogueImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, catalogueImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func writeDisbursementFileTooLarge(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
		"Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập — hãy chia thành nhiều tệp.", "")
}

// disbursementImportTemplateSpec builds the template: the header, ONE example row of fake data whose
// project code can never be a real one (domain.VoucherImportExampleProjectCode — left in place, it
// refuses the file rather than becoming a payment), and a guide sheet.
//
// THE FILE IS PER COMMUNE (owner, 14/10/2026, reversing "one file for every commune, no dropdown"): it
// carries two WARN-only dropdowns read for the commune in the context (app TemplateChoices) —
//
//	Mã dự án    "code · name" of every live project; the reader keeps the code (xlsx.ChoiceCode)
//	Nguồn vốn   plain catalogue NAMES, because the planner resolves a source by name
//
// Warn, not refuse: a project created after the download must still be typeable, and the valid sources
// of a row depend on that row's project, which a dropdown cannot follow. The planner re-checks both.
func disbursementImportTemplateSpec(choices app.VoucherTemplateChoices) xlsx.TemplateSpec {
	spec := xlsx.TemplateSpec{
		SheetName: disbursementImportSheet,
		Header:    domain.VoucherImportColumns(),
		Examples: [][]string{{
			domain.VoucherImportExampleProjectCode, "15/03/2026", "250000000",
			"Thanh toán đợt 1 hợp đồng thi công", "Công ty TNHH Mẫu", "CT-0001", "",
		}},
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT chứng từ giải ngân mới. Không sửa, không đổi thứ tự dòng tiêu đề.",
			"Dòng 2 là DÒNG VÍ DỤ — hãy xoá nó trước khi nhập; để nguyên thì cả tệp bị từ chối.",
			"Mã dự án: bắt buộc — chọn trong danh sách (dạng \"mã · tên dự án\") hoặc ghi đúng mã dự án đang có của xã (ví dụ DA01).",
			"Ngày chi: dd/mm/yyyy (ví dụ 15/03/2026) hoặc ô định dạng ngày của Excel; năm từ 2000 đến 2100.",
			"Số tiền (đồng): số nguyên lớn hơn 0, viết liền (250000000) hoặc tách nghìn (250.000.000). Không nhập số âm.",
			"Nội dung chi: bắt buộc, tối đa 1000 ký tự. Đơn vị thụ hưởng (tối đa 300 ký tự) và Số chứng từ (tối đa 100 ký tự): không bắt buộc.",
			"Nguồn vốn: ghi đúng TÊN nguồn vốn trong danh mục của xã. Dự án đã gắn nguồn vốn thì BẮT BUỘC và phải là một nguồn đã phân bổ cho dự án; dự án chưa gắn nguồn vốn thì để trống.",
			"Chứng từ nhập vào ở trạng thái Kế toán nhập và được tính ngay vào số đã giải ngân.",
			"Tệp được nhập TOÀN BỘ HOẶC KHÔNG GÌ CẢ: còn một dòng sai thì không dòng nào được nhận. Hãy bấm Kiểm tra trước khi Nhập.",
		},
		DropdownRows: domain.MaxVoucherImportRows,
		Choices:      map[int]xlsx.Choices{},
	}
	if len(choices.Projects) > 0 {
		values := make([]string, 0, len(choices.Projects))
		for _, p := range choices.Projects {
			values = append(values, xlsx.ChoiceValue(p.Code, p.Name))
		}
		spec.Choices[voucherImportColumn(domain.VoucherImportColProject)] = xlsx.Choices{Values: values, Strict: false,
			Title: "Mã dự án", Message: "Chọn một dự án trong danh sách hoặc ghi đúng mã dự án của xã."}
	}
	if len(choices.Sources) > 0 {
		spec.Choices[voucherImportColumn(domain.VoucherImportColSource)] = xlsx.Choices{Values: choices.Sources, Strict: false,
			Title: "Nguồn vốn", Message: "Chọn một nguồn vốn đã phân bổ cho dự án của dòng này; dự án chưa gắn nguồn vốn thì để trống."}
	}
	return spec
}

// voucherImportColumn is the 0-based index of a template column.
func voucherImportColumn(name string) int {
	return slices.Index(domain.VoucherImportColumns(), name)
}

// projectCellsToCodes reduces every data row's `Mã dự án` cell to its code: the dropdown writes
// "code · name", a hand-typed cell holds the bare code, and both mean the code (xlsx.ChoiceCode). HERE,
// NOT IN THE DOMAIN READER: domain/ is standard library only and the separator belongs to core/xlsx.
// The code is then matched exactly as a typed one; the name part is never matched.
func projectCellsToCodes(cells [][]string) {
	col := voucherImportColumn(domain.VoucherImportColProject)
	for i := 1; i < len(cells); i++ {
		if col < len(cells[i]) {
			cells[i][col] = xlsx.ChoiceCode(cells[i][col])
		}
	}
}

// DisbursementImportTemplate serves GET /api/v1/disbursements/import-template — per commune (the
// dropdowns are the commune's in the context), no-store. A failed read of the lists never fails the
// download: the template is served without dropdowns and the planner checks every cell anyway.
func (h *Handler) DisbursementImportTemplate(w http.ResponseWriter, r *http.Request) {
	choices, err := h.d.DisbursementImports.TemplateChoices(r.Context())
	if err != nil {
		h.d.Log.Warn("mẫu nhập giải ngân: không đọc được danh sách chọn, tệp mẫu không có danh sách chọn",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		choices = app.VoucherTemplateChoices{}
	}
	b, err := xlsx.Template(disbursementImportTemplateSpec(choices))
	if err != nil {
		h.d.Log.Error("mẫu nhập giải ngân: dựng tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set(catalogueHeaderContentType, catalogueXLSXMIME)
	w.Header().Set("Content-Disposition", `attachment; filename="`+disbursementImportFilename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập giải ngân: gửi tệp lỗi", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
}

// readVoucherRows reads the upload into rows. ok=false means the response was already written;
// sheetErrs non-empty means the SHEET is not the template (answered by the caller).
func (h *Handler) readVoucherRows(w http.ResponseWriter, r *http.Request) ([]domain.VoucherImportRow, []domain.VoucherImportError, bool) {
	cells, ok := h.readUploadedSheet(w, r, writeDisbursementFileTooLarge, "nhập giải ngân")
	if !ok {
		return nil, nil, false
	}
	projectCellsToCodes(cells)
	rows, errs := domain.VoucherRowsFromSheet(cells)
	return rows, errs, true
}

// PreviewDisbursementImport serves POST /api/v1/disbursements/import-previews — 200 whether or not the
// file is valid: the list of errors IS the answer.
func (h *Handler) PreviewDisbursementImport(w http.ResponseWriter, r *http.Request) {
	rows, sheetErrs, ok := h.readVoucherRows(w, r)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		vietJSON(w, http.StatusOK, disbursementImportPreviewOut{Errors: voucherErrorsOut(sheetErrs)})
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.DisbursementImports.Preview(r.Context(), rows)
	if err != nil {
		h.d.Log.Error("nhập giải ngân: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, disbursementImportPreviewOut{
		Valid:       len(res.Errors) == 0,
		RowCount:    res.RowCount,
		TotalAmount: int64(res.Total),
		Errors:      voucherErrorsOut(res.Errors),
	})
}

// ImportDisbursements serves POST /api/v1/disbursements/imports.
func (h *Handler) ImportDisbursements(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	rows, sheetErrs, ok := h.readVoucherRows(w, r)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		writeDisbursementImportRejected(w, sheetErrs)
		return
	}
	res, err := h.d.DisbursementImports.Import(r.Context(), rows, actor)
	if err != nil {
		var rej *app.VoucherImportRejected
		if errors.As(err, &rej) {
			writeDisbursementImportRejected(w, rej.Errors)
			return
		}
		// The wrapped error never reaches the client (rule 3, forbidden #3); no amount, no description
		// in the log line either — only the commune.
		h.d.Log.Error("nhập giải ngân: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa chứng từ nào được ghi. Vui lòng thử lại.", "")
		return
	}
	// What a retry carrying the same Idempotency-Key is told: THE BATCH, never the body.
	idem.RecordCode(r.Context(), res.Batch)
	created := make([]disbursementImportCreatedRowOut, 0, len(res.Created))
	for _, c := range res.Created {
		created = append(created, disbursementImportCreatedRowOut{Row: c.Row, ID: c.ID, ProjectCode: c.ProjectCode})
	}
	vietJSON(w, http.StatusCreated, disbursementImportCreatedOut{
		Valid:       true,
		RowCount:    res.RowCount,
		TotalAmount: int64(res.Total),
		Errors:      []catalogueImportErrorOut{},
		Batch:       res.Batch,
		Created:     created,
	})
}

// writeDisbursementImportRejected is 400 `import_invalid` with every error; nothing was written.
func writeDisbursementImportRejected(w http.ResponseWriter, errs []domain.VoucherImportError) {
	vietJSON(w, http.StatusBadRequest, catalogueImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa chứng từ nào được ghi. Còn một dòng sai thì không dòng nào được nhận — sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  voucherErrorsOut(errs),
	})
}
