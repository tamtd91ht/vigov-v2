package http

// The disbursement voucher register, IMPORT FROM EXCEL (docs/ui-ux/06-giai-ngan.md §10, §13 rule 7).
// Handlers: disbursement_import.go.
//
// THE KEYS ARE THE ONES THE VOUCHER REGISTER ALREADY DECLARES (routes.go), none invented (rule 5,
// invariant 3c): `budget.update` for the preview and the import — an import IS entering payments, the
// act POST /api/v1/disbursements guards — and `budget.read` for the template, which holds no data of
// the commune (an empty sheet with one fake example row). All three are seeded at
// service-identity/migrations/0001_init.sql:282-284.
//
// THREE ROUTES AND NOT `POST …/imports?mode=dry-run|commit`: the catalogue import's reason
// (disbursement_import.go header) — the idempotency key is scoped by path, so a mode flag would let a
// preview be replayed as an import.
//
// The template: NO idem.* (a GET changes nothing). The preview: idem.KhongCan (writes nothing). The
// import: idem.Required(idem.DongKhiHong) — the key POST /api/v1/disbursements takes, for its reason:
// nothing in the schema makes a voucher unique, so a cache outage plus a double submit would count a
// whole file of payments twice in "đã giải ngân". A retry is told `{"code":"<batch>","replayed":true}`.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerDisbursementImportRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// @summary  Tải tệp Excel mẫu để nhập chứng từ giải ngân
	// @screen   06-giai-ngan §10
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/disbursements/import-template",
		authz.RequirePermission(d.Checker, "budget.read")(
			http.HandlerFunc(h.DisbursementImportTemplate)))

	// multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 500 data rows). 200 WHETHER OR NOT THE FILE
	// IS VALID: `valid: false` with every {row, column, message} is the answer.
	//
	// @summary  Kiểm tra một tệp Excel chứng từ giải ngân trước khi nhập — không ghi gì
	// @screen   06-giai-ngan §10
	// @reply    200 disbursementImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/disbursements/import-previews",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái dự án và nguồn vốn")(
				http.HandlerFunc(h.PreviewDisbursementImport))))

	// 400 `import_invalid` carries EVERY error as {row, column, message}; nothing was written. Every row
	// is written in ONE transaction with its audit entry; any failure rolls the whole file back.
	//
	// @summary  Nhập chứng từ giải ngân từ tệp Excel — toàn bộ tệp hoặc không gì cả
	// @screen   06-giai-ngan §10
	// @reply    201 disbursementImportCreatedOut
	// @reply    400 catalogueImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/disbursements/imports",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportDisbursements))))
}
