package http

// The revenue / expenditure board, LOAD FROM EXCEL (docs/ui-ux/07-thu-chi-ngan-sach.md §6; ADR 0081 #6).
// Handlers: budget_import.go.
//
// THE KEY IS THE ONE POST /api/v1/budget-sheets ALREADY DECLARES (routes.go), none invented (rule 5,
// invariant 3c): `budget.update`, seeded at service-identity/migrations/0001_init.sql:282-284. Loading
// the Phòng Tài chính's file is creating the year's sheets — the same act as the create route.
//
// TWO ROUTES AND NOT THREE: the prototype has no template for this file and neither does this build —
// the file IS the commune's own report, read as it is (the parser's header says why). Two and not
// `?mode=`: the idempotency key is scoped by path, so a mode flag would let a preview be replayed as an
// import (disbursement_import.go's header).
//
// `year` IS A QUERY PARAMETER: the file states a year only in free-text headings, and reading it from
// there would be a guess about which year a whole report is filed under.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerBudgetImportRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 16 sheets, ≤ 5000 rows a sheet);
	// query `year`. 200 WHETHER OR NOT THE FILE WOULD LOAD: `valid: false` with the errors and the
	// refusal sentences is the answer.
	//
	// @summary  Kiểm tra tệp Excel thu - chi ngân sách trước khi nạp — không ghi gì
	// @screen   07-thu-chi-ngan-sach §6
	// @reply    200 budgetImportOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/budget-sheets/import-previews",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — giao dịch luôn huỷ; gửi lại bao nhiêu lần cũng cho cùng câu trả lời trên cùng trạng thái bảng và chốt kỳ")(
				http.HandlerFunc(h.PreviewBudgetImport))))

	// Every sheet of the file in ONE transaction with its entries; any refusal writes nothing. 400
	// `import_invalid` lists every error; 409 `budget_period_closed` (the year or a month of it is
	// closed) or `budget_sheet_has_entries` (the live sheet holds hand-entered figures — Gỡ first).
	//
	// idem.Required(DongKhiHong), the create route's reason: nothing unique tells a double submit from a
	// deliberate reload, and a reload REPLACES — a cache outage plus a double click would replace the
	// sheet just loaded with itself, burning a revision code.
	//
	// @summary  Nạp bảng thu, chi ngân sách từ tệp Excel của Phòng Tài chính — toàn bộ tệp hoặc không gì cả
	// @screen   07-thu-chi-ngan-sach §6
	// @reply    201 budgetImportOut
	// @reply    400 budgetImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/budget-sheets/imports",
		authz.RequirePermission(d.Checker, "budget.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportBudgetWorkbook))))
}
