package http

// The capital-plan-category catalogue, IMPORT FROM EXCEL. THREE ROUTES, ALL `admin.lookup`
// (catalogue_import.go).
//
// USER DECISION 2026-09-29 (ADR 0059 §3): one import route PER OWNING SERVICE AND PER CATALOGUE —
// never a shared "lookup-values" import carrying a `Nhóm` column (ADR 0024 stop condition #4). The key
// is the one POST /api/v1/capital-plan-categories declares (routes.go); none invented (rule 5,
// invariant 3c). Create only, ALL OR NOTHING: columns Tên hiển thị · Mã (blank = derived) · Thứ tự. An
// imported row is the commune's own (tier 1), in use, never the default.
//
// The template: NO idem.* (a GET changes nothing). The preview: idem.KhongCan (writes nothing). The
// import: idem.Required(idem.DongKhiHong) — service-identity's catalogue-import call, and NOT the
// MoKhiHong the single-row create form takes: the unique key covers a duplicate CODE, but a file whose
// codes are DERIVED and retried by a flaky network while the first attempt is still in flight is what
// the key stops; the planner can only refuse the second after the first committed. A retry is told
// `{"code":"<batch>","replayed":true}` — the `lo_nhap` every audit entry of that file carries.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCatalogueImportRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// @summary  Tải tệp Excel mẫu để nhập hạng mục kế hoạch vốn
	// @screen   14-cau-hinh §5
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/capital-plan-categories/import-template",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.CapitalPlanCategoryImportTemplate)))

	// multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 200 data rows). 200 WHETHER OR NOT THE FILE
	// IS VALID: `valid: false` with every {row, column, message} is the answer.
	//
	// @summary  Kiểm tra một tệp Excel hạng mục kế hoạch vốn trước khi nhập — không ghi gì
	// @screen   14-cau-hinh §5
	// @reply    200 catalogueImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/capital-plan-categories/import-previews",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái danh mục")(
				http.HandlerFunc(h.PreviewCapitalPlanCategoryImport))))

	// 400 `import_invalid` carries EVERY error as {row, column, message}; nothing was written. 409
	// `catalogue_changed`: a code was taken between the check and the write; the whole file rolled back.
	//
	// @summary  Nhập hạng mục kế hoạch vốn từ tệp Excel — toàn bộ tệp hoặc không gì cả
	// @screen   14-cau-hinh §5
	// @reply    201 catalogueImportCreatedOut
	// @reply    400 catalogueImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/capital-plan-categories/imports",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportCapitalPlanCategories))))
}
