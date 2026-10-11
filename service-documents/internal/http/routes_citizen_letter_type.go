package http

// "LOẠI ĐƠN THƯ" — four routes on the Cấu hình › Danh mục screen (bug sheet row 33; ADR 0039: the
// commune's label tier is documents'). Handlers: citizen_letter_type.go.
//
// ONE KEY, SEEDED, NOT INVENTED (rule 5, invariant 3c): `admin.lookup` ("Quản lý danh mục",
// service-identity/migrations/0001_init.sql:274) — the key every other Danh mục group's write routes
// declare (document-types, here; map-asset-types, task-types, … in their services).
//
// THE READ IS `admin.lookup` TOO, not AnyAuthenticated like GET /api/v1/document-types. That route is
// open because a document's type label shows on every register screen; nothing reads these labels
// outside the Danh mục tab today (the register still prints its fixed labels), and opening a route to
// every account is rule 5's stop condition #1 — not a call to make for a list nobody else needs yet.
//
// THE NOUN `citizen-letter-types` follows `citizen-letters` (kb/00-foundation/ubiquitous-language.md:142);
// chosen here (ADR 0011: reported, not settled). Its own first segment, so tools/ingress routes it to
// this service.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCitizenLetterTypeRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// @summary  Danh mục Loại đơn thư của xã — nhãn, thứ tự, màu, bật/tắt cho bốn loại đơn theo luật
	// @screen   14-cau-hinh §5
	// @reply    200 citizenLetterTypeListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-letter-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListCitizenLetterTypes)))

	// idem.Required(MoKhiHong): `UNIQUE (tenant_id, code)` already makes a duplicate row impossible, so a
	// cache outage costs at most a confusing 409 — not worth refusing an administrator mid-configuration.
	//
	// @summary  Thêm nhãn của xã cho một trong bốn loại đơn thư (kien-nghi-phan-anh · khieu-nai · to-cao · de-nghi)
	// @screen   14-cau-hinh §5
	// @request  addCitizenLetterTypeIn
	// @reply    201 citizenLetterTypeOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-letter-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.AddCitizenLetterType))))

	// @summary  Sửa nhãn, thứ tự, màu hoặc bật/tắt một dòng Loại đơn thư (mã không đổi được)
	// @screen   14-cau-hinh §5
	// @request  editCitizenLetterTypeIn
	// @reply    200 citizenLetterTypeOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/citizen-letter-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("ghi đè một trạng thái đã biết; app.EditType không ghi gì và không để vết khi không trường nào đổi")(
				http.HandlerFunc(h.EditCitizenLetterType))))

	// @summary  Xoá mềm một dòng Loại đơn thư; lý do không bắt buộc (bỏ trống thì ghi câu cố định); mã không cấp lại
	// @screen   14-cau-hinh §5
	// @request  removeCitizenLetterTypeIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/citizen-letter-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lần xoá thứ hai không thấy dòng còn sống nên trả 404 và không ghi gì")(
				http.HandlerFunc(h.RemoveCitizenLetterType))))
}
