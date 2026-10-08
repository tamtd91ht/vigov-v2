package http

// `Nhập từ Excel` and `Xuất Excel` of the citizen-letter register (ADR 0084 #6; owner spec §4.1, §5).
// Handlers: citizen_letter_import.go, citizen_letter_report_export.go.
//
// KEYS — ALL SEEDED, NONE INVENTED (rule 5, invariant 3c; service-identity/migrations/0001_init.sql
// :302-304):
//
//	import-template · import-previews · imports   `petition.create`, the booking key — the prototype's
//	                                              (vigov-require router.py:292-319), and an import IS
//	                                              booking, row by row, through the same use case
//	citizen-letter-report/exports                 `report.export` AND `petition.read`: the prototype
//	                                              gates it on `report.export` alone (router.py:371-374),
//	                                              but the figures are those of a register its holder
//	                                              may not read; the JSON report route already demands
//	                                              `petition.read`, and a file must not be a way round
//	                                              that. Two nested guards, one per key — an AND, which
//	                                              RequireAnyPermission (an OR) cannot express.
//	                                              ⚠ tools/apidoc records one key per route and lists
//	                                              the outer one; the contract under-states the rule.
//
// THE NOUNS are the document-type import's (`import-template`, `import-previews`, `imports`). The
// literal segments cannot collide with GET /api/v1/citizen-letters/{id}: a letter id is a ULID, and
// net/http's mux prefers the literal pattern anyway. The export hangs under `citizen-letter-report`
// (its own first segment, so tools/ingress already routes it here).

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCitizenLetterImportRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// The file exists only to feed the import, so the import's key. NO idem.*: a GET changes no state.
	//
	// @summary  Tải tệp Excel mẫu để nhập sổ đơn thư (Ngày đến, Họ tên người gửi, Địa chỉ, Số điện thoại, Loại đơn, Nội dung đơn, Bộ phận xử lý (mã))
	// @screen   05-van-ban-don-thu §3.1
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-letters/import-template",
		authz.RequirePermission(d.Checker, "petition.create")(
			http.HandlerFunc(h.CitizenLetterImportTemplate)))

	// Checking a filled file. multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 200 data rows).
	// 200 WHETHER OR NOT THE FILE IS VALID: `valid: false` with every {row, column, message}. Each valid
	// row shows its year, unit and the deadline identity would fix — no number (drawn only on import),
	// no personal data. 503 when identity cannot check units or deadlines.
	//
	// @summary  Kiểm tra một tệp Excel sổ đơn thư trước khi nhập — không ghi gì, không cấp số
	// @screen   05-van-ban-don-thu §3.1
	// @reply    200 letterImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/import-previews",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.KhongCan("xem trước không ghi gì, không cấp số và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái sổ")(
				http.HandlerFunc(h.PreviewCitizenLetterImport))))

	// Booking a filled file — ALL OR NOTHING, every row through the booking use case with source
	// `nhap-excel`.
	//
	// idem.Required(idem.DongKhiHong), THE BOOKING ROUTE'S CALL: every row takes a register number that
	// is never given back (rule 7, invariant 3), and a double-submitted file would book the whole batch
	// twice. A 400 releases the key, so the corrected file may go with the same one.
	//
	// 400 `import_invalid` carries EVERY error as {row, column, message}; nothing was written. 503 when
	// identity could not check the file — nothing was written. 413 / 415 / 400 as on the preview.
	//
	// @summary  Nhập sổ đơn thư từ tệp Excel — mỗi dòng vào sổ như nhập tay (cấp số, tự tính hạn, ghi vết), nguồn Nhập từ Excel; toàn bộ tệp hoặc không gì cả
	// @screen   05-van-ban-don-thu §3.1
	// @reply    201 letterImportCreatedOut
	// @reply    400 letterImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/imports",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportCitizenLetters))))

	// The yearly report as a file. AGGREGATES ONLY — no letter, no sender, no summary. The export itself
	// is audited (`xuat_bao_cao_don_thu`) before the bytes leave. A GET that writes an audit entry, like
	// the letter detail; no idem.* (a second export is a second export, and is recorded as one).
	//
	// @summary  Xuất báo cáo sổ đơn thư một năm ra Excel: sáu số liệu, theo loại đơn, theo bộ phận (có dòng Chưa phân công), theo tháng — chỉ số liệu tổng hợp, có ghi vết
	// @screen   05-van-ban-don-thu §4
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-letter-report/exports",
		authz.RequirePermission(d.Checker, "report.export")(
			authz.RequirePermission(d.Checker, "petition.read")(
				http.HandlerFunc(h.ExportCitizenLetterReport))))
}
