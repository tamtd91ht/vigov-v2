package http

// The two task catalogues, IMPORT FROM EXCEL. SIX ROUTES, ALL `admin.lookup` (catalogue_import.go).
//
// USER DECISION 2026-09-29 (ADR 0059 §3): one import route PER OWNING SERVICE AND PER CATALOGUE —
// never a shared "lookup-values" import carrying a `Nhóm` column (ADR 0024 stop condition #4). The key
// is the one POST /api/v1/task-types and POST /api/v1/task-priorities declare (routes.go); none
// invented (rule 5, invariant 3c). Create only, ALL OR NOTHING. An imported row is the commune's own
// (tier 1), in use, never the default.
//
// COLUMNS: task types Tên hiển thị · Mã (blank = derived) · Thứ tự. Task priorities Tên hiển thị · Mã —
// NO Thứ tự: the new levels are ranked by their position in the file, AFTER every existing level
// (domain.CatalogueOrderFromPosition), because on a scale the order IS the meaning.
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
	// @summary  Tải tệp Excel mẫu để nhập loại nhiệm vụ
	// @screen   14-cau-hinh §5
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-types/import-template",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.TaskTypeImportTemplate)))

	// multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 100 data rows). 200 WHETHER OR NOT THE FILE
	// IS VALID: `valid: false` with every {row, column, message} is the answer.
	//
	// @summary  Kiểm tra một tệp Excel loại nhiệm vụ trước khi nhập — không ghi gì
	// @screen   14-cau-hinh §5
	// @reply    200 catalogueImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-types/import-previews",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái danh mục")(
				http.HandlerFunc(h.PreviewTaskTypeImport))))

	// 400 `import_invalid` carries EVERY error as {row, column, message}; nothing was written. 409
	// `catalogue_changed`: a code was taken between the check and the write; the whole file rolled back.
	//
	// @summary  Nhập loại nhiệm vụ từ tệp Excel — toàn bộ tệp hoặc không gì cả
	// @screen   14-cau-hinh §5
	// @reply    201 catalogueImportCreatedOut
	// @reply    400 catalogueImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-types/imports",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportTaskTypes))))

	// @summary  Tải tệp Excel mẫu để nhập mức ưu tiên nhiệm vụ — không có cột Thứ tự
	// @screen   14-cau-hinh §5
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-priorities/import-template",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.TaskPriorityImportTemplate)))

	// Same contract as the task-type preview; `order` in each entry is the rank the level WOULD get —
	// after every existing level, in file order.
	//
	// @summary  Kiểm tra một tệp Excel mức ưu tiên nhiệm vụ trước khi nhập — không ghi gì
	// @screen   14-cau-hinh §5
	// @reply    200 catalogueImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-priorities/import-previews",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái danh mục")(
				http.HandlerFunc(h.PreviewTaskPriorityImport))))

	// Same contract as the task-type import.
	//
	// @summary  Nhập mức ưu tiên nhiệm vụ từ tệp Excel — xếp sau các mức đang có; toàn bộ tệp hoặc không gì cả
	// @screen   14-cau-hinh §5
	// @reply    201 catalogueImportCreatedOut
	// @reply    400 catalogueImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-priorities/imports",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportTaskPriorities))))
}
