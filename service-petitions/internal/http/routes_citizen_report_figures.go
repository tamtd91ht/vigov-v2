package http

// The /phan-anh statistics and the log-attachment removal (owner decisions of 09/10/2026, batch A).
// FOUR ROUTES, EVERY KEY SEEDED in `quyen` (service-identity/migrations/0001_init.sql: feedback.read,
// feedback.resolve, report.read) — none invented (rule 5, invariant 3c). Handlers: citizen_report_figures.go.
//
// A FILE OF ITS OWN, registerCatalogueImportRoutes' shape, so routes.go grows by one line.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCitizenReportFigureRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	if d.CitizenReportBreakdown == nil {
		// At construction, not at the first request — the reason the switch in Register gives.
		panic("petitions/http: thiếu use case thống kê phản ánh — GET /api/v1/citizen-report-breakdown sẽ panic khi có người gọi")
	}

	// THE TOTAL OF THE REGISTER UNDER THE LIST'S FILTERS — the number over the "Phản ánh của người dân"
	// list, which page.Result never carries (task-counts is the precedent). Same parser, same refusals,
	// same `scope=mine` from the session, same `can-bo` exclusion; paging parameters ignored.
	//
	// `feedback.read`, THE LIST'S KEY: the number reveals nothing the list does not, and a second key
	// would let an account see a total over a list it may not read. 401 is RequirePermission's answer to
	// no session AND to a session of another commune. NO idem.*: a GET. NO AUDIT ENTRY: a count.
	//
	// @summary  Tổng số phiếu phản ánh theo đúng bộ lọc của danh sách (trạng thái · lĩnh vực · thôn · bộ phận · kênh · trễ hạn · đánh giá · phạm vi · metric)
	// @screen   09-phan-anh-nguoi-dan §4
	// @reply    200 citizenReportCountsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-report-counts",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.CitizenReportCounts)))

	// THE HEAT MAP: every LOCATED petition of the list's filter as {lat, lng, status} and nothing else —
	// no code, no content, no reporter (rule 3). NOT PAGINATED, BOUNDED: past
	// petstore.CitizenReportPointsCeiling (5000) it answers 422 `too_many_points` rather than drawing a
	// map with places silently missing (service-comms map-asset-points precedent).
	//
	// `feedback.read`, the list's key, for the count's reason. NO idem.*: a GET. NO AUDIT ENTRY: no
	// reporter and no record is identified by a dot.
	//
	// @summary  Điểm phản ánh trên bản đồ nhiệt theo bộ lọc của danh sách — chỉ vĩ độ, kinh độ, trạng thái
	// @screen   09-phan-anh-nguoi-dan §4
	// @reply    200 citizenReportPointsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error too_many_points
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-report-points",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.CitizenReportPoints)))

	// THE STATISTICS OF ONE PERIOD [from, to) — the client's period (ADR 0053 §3, Tuần/Tháng/Quý/Năm in
	// Asia/Ho_Chi_Minh), counted live here like /tong-quan and /bao-cao (ADR 0053 §1, B1). Four sections:
	// (a) the tile's on-time / late and the overdue stock, (b) by field with an unclassified row, (c) by
	// the unit holding each petition when its work was finished, with the handling time in WORKING
	// seconds measured by identity, (d) by residential unit (khong-tiep-nhan excluded) with a "no place"
	// row. Counts and sums only — the client divides.
	//
	// THE TWO NESTED KEYS OF EVERY FIGURE ROUTE (ADR 0053 §2): `feedback.read` AND `report.read`, the
	// inner one being what tools/apidoc records. `feedback.restricted` absent -> `can-bo` excluded.
	//
	// 409 `working_calendar_not_configured` and 503 `working_hours_unavailable`: the handling time cannot
	// be measured in working hours — refused, never a wall-clock fallback (rule 10, forbidden #2).
	//
	// @summary  Thống kê phản ánh trong kỳ [from, to) — đúng hạn/trễ hạn và tồn quá hạn; theo lĩnh vực (kèm dòng chưa phân loại); theo bộ phận đang giữ phiếu lúc xử lý xong (thời gian xử lý theo giờ làm việc); theo thôn (kèm dòng chưa xác định địa bàn)
	// @screen   09-phan-anh-nguoi-dan §3
	// @reply    200 citizenReportBreakdownOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error working_calendar_not_configured
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error working_hours_unavailable
	mux.Handle("GET /api/v1/citizen-report-breakdown",
		authz.RequirePermission(d.Checker, "feedback.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.CitizenReportBreakdown))))

	// GỠ TỆP ĐÍNH KÈM NHẬT KÝ — the task twin (DELETE /api/v1/tasks/{ma}/attachments/{id}, commit 9d4b2684)
	// on a petition: a SOFT DELETE of the stored_file row with a mandatory reason, a timeline line and the
	// audit entry (`go_tep_phan_anh`) in ONE transaction. The object is NOT removed (ADR 0052 §6, §7) and
	// the append-only link stays; the download, the timeline and the attach path already skip a deleted row.
	//
	// `feedback.read` AT THE GATE, and who may actually remove is decided on the locked row (owner decision
	// 09/10/2026, the task twin's ADR 0076 §4b shape): THE UPLOADER, OR A HOLDER OF `feedback.resolve`. Both
	// keys seeded. A file the caller cannot see — unknown, another commune's or petition's, a photo,
	// somebody else's draft, already removed — is 404, one answer; a visible file without either door is
	// 403; `can-bo` without `feedback.restricted` is the petition's 404; legal hold is 409.
	//
	// A BODY ON A DELETE, the task twin's reason: the reason is free text about a record.
	//
	// idem.KhongCan — the locked read sees only live rows, so a second removal is a 404 and cannot
	// overwrite who removed the file or why; it writes no second timeline line.
	//
	// @summary  Gỡ một tệp đính kèm khỏi nhật ký xử lý phiếu phản ánh (xoá mềm, lý do bắt buộc, không xoá tệp gốc) — người đã tải lên hoặc cán bộ có quyền kết thúc xử lý phản ánh
	// @screen   09-phan-anh-nguoi-dan §8.7
	// @request  citizenReportLogAttachmentRemoveIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error legal_hold
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}",
		authz.RequirePermission(d.Checker, "feedback.read")(
			idem.KhongCan("gỡ lần hai một tệp đã gỡ trả 404: câu đọc khoá dòng chỉ thấy tệp chưa gỡ nên không ghi đè người gỡ, lý do, và không thêm dòng nhật ký thứ hai")(
				http.HandlerFunc(h.RemovePetitionLogAttachment))))
}
