package http

// SỔ ĐƠN THƯ CÔNG DÂN — thirteen routes (ADR 0039, ADR 0078 #2–#4, ADR 0079 lô 5 Q18, ADR 0084). Handlers and masking:
// citizen_letter.go.
//
// TWO KEYS, BOTH SEEDED, NONE INVENTED (rule 5, invariant 3c): `petition.create` ("Tiếp nhận đơn
// thư") and `petition.read` ("Xem đơn thư"), service-identity/migrations/0001_init.sql:302-303. C13/C14
// (ledger `so-don-thu-cong-dan`, user decision 24/09/2026): booking, routing and the deadline are
// `petition.create`;
// a status change is `petition.read` AND (the letter's assignee OR a holder of `petition.create`).
//
// THE THIRD GROUP — status, result, log-entries — IS GATED ON `petition.read` AT THE ROUTE, and the
// "assignee OR petition.create" half is decided by the use case on the row read FOR UPDATE
// (app.mayWorkOn), because it depends on THIS letter. The handler asks the same checker for
// `petition.create` (letterCaller), so the commune check of rule 5 invariant 3 applies to it too.
// ⚠ tools/apidoc records one key per route: the contract lists `petition.read` for those three and
// under-states the runtime rule, as it does for the dashboard routes in routes.go.
//
// ⚠ NO "xem danh tính tố cáo" KEY: the ledger records the user's agreement to a NEW key for viewing a
// whistleblower's identity, but no migration seeds it. Inventing it here would be a route answering
// 403 to everybody forever; so the identity is shown to the assignee alone (domain.DetailDisclosure).
// A finding for open question #27, not a decision.
//
// THE NOUN `citizen-letters` IS THE GLOSSARY'S (kb/00-foundation/ubiquitous-language.md:142). The
// sub-resources `duplicates`, `routings` (as incoming-documents), `status`, `result`, `sender`,
// `deadline`, `log`,
// `log-entries` (as citizen-reports and tasks) and the singular `citizen-letter-report` were chosen
// here (ADR 0011: reported, not settled). `citizen-letter-report` is its OWN first segment so
// tools/ingress routes it to this service like `incoming-document-summary`.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCitizenLetterRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// VÀO SỔ — issues a `don-thu` number of this commune and this year. idem.Required(DongKhiHong),
	// the incoming register's call and for its reason: during a Redis outage a double-submitted form
	// would take THE NEXT NUMBER, and an issued number is never given back (rule 7, invariant 3).
	//
	// @summary  Vào sổ một đơn thư công dân; hệ thống cấp số theo dãy của xã trong năm; hạn xử lý đặt sau bằng PATCH …/deadline
	// @screen   05-van-ban-don-thu §3.4
	// @request  bookLetterIn
	// @reply    201 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.BookCitizenLetter))))

	// CẢNH BÁO TRÙNG (C11) — a POST because the sender's name travels in the BODY, never the URL
	// (rule 3, forbidden #4). `petition.create`: it serves the booking form.
	//
	// @summary  Kiểm đơn có thể trùng trước khi vào sổ: cùng họ tên người gửi (nếu nhập) trong 365 ngày, xếp theo độ giống trích yếu — chỉ cảnh báo
	// @screen   05-van-ban-don-thu §3.4
	// @request  duplicateCheckIn
	// @reply    200 duplicatesOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/duplicates",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.KhongCan("chỉ đọc và so sánh, không ghi gì — gửi lại bao nhiêu lần cũng cho cùng một câu trả lời trên cùng trạng thái sổ")(
				http.HandlerFunc(h.CitizenLetterDuplicates))))

	// THE REGISTER. Denunciations carry neither identity nor summary here, whoever asks. Each row
	// carries `source` (fixed at booking) and `status_group` (ADR 0084's display group, derived on read).
	// `status_group=` filters by that group (closed set, 400 otherwise), ANDed with `status=`.
	//
	// @summary  Sổ đơn thư công dân, phân trang theo con trỏ, mới vào sổ trước; lọc năm · trạng thái · nhóm trạng thái hiển thị (status_group) · loại · bộ phận · cán bộ · khoảng ngày nhận · từ khoá · phạm vi
	// @screen   05-van-ban-don-thu §3.1
	// @reply    200 page.Result[citizenLetterItemOut]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-letters",
		authz.RequirePermission(d.Checker, "petition.read")(
			http.HandlerFunc(h.ListCitizenLetters)))

	// SỐ ĐẾM TRÊN TAB "Đơn thư công dân (N)" (ADR 0084 #7) — the size of the register above under
	// EXACTLY its filters and scope (one parser, letterFilterFromQuery; one predicate builder in the
	// store). A sibling count route and not a `total` on the list: page.Result carries none by design,
	// and petitions' task-counts / task-extension-counts are the precedent. A top-level
	// `citizen-letter-counts`, not `citizen-letters/counts`, which `citizen-letters/{id}` would read as
	// a letter whose id is `counts`; its own first segment, so tools/ingress routes it here.
	//
	// `petition.read`, THE LIST'S OWN KEY: the number reveals nothing the list does not, and a second
	// key would let an account see a badge over a register it may not read. NO KEY WAS INVENTED (rule
	// 5, invariant 3c). NO idem.*: a GET changes no state. NO AUDIT ENTRY: a number, no personal data.
	//
	// @summary  Số đơn thư trong sổ theo đúng bộ lọc và phạm vi của danh sách — số trên tab "Đơn thư công dân (N)"
	// @screen   05-van-ban-don-thu §3.1
	// @reply    200 citizenLetterCountOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-letter-counts",
		authz.RequirePermission(d.Checker, "petition.read")(
			http.HandlerFunc(h.CitizenLetterCount)))

	// ONE LETTER. 404 is one sentence for never-existed, removed and another commune's. Reading a
	// denunciation's identity or content writes an audit entry in the same transaction (rule 6,
	// invariant 7) — a GET that writes, like documents-audit-entries; no idem.* (a second read is a
	// second read, and is recorded as one).
	//
	// @summary  Một đơn thư cho ngăn chi tiết, kèm kết quả giải quyết; SĐT luôn che, địa chỉ không trả về; đơn tố cáo chỉ cán bộ được giao thấy danh tính
	// @screen   05-van-ban-don-thu §3.5
	// @reply    200 citizenLetterOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-letters/{id}",
		authz.RequirePermission(d.Checker, "petition.read")(
			http.HandlerFunc(h.CitizenLetterDetail)))

	// THE PROCESSING LOG, newest first. Staff-internal.
	//
	// @summary  Nhật ký xử lý của một đơn thư, mới nhất trước — chỉ đọc
	// @screen   05-van-ban-don-thu §3.5
	// @reply    200 letterLogOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-letters/{id}/log",
		authz.RequirePermission(d.Checker, "petition.read")(
			http.HandlerFunc(h.CitizenLetterLog)))

	// CHUYỂN XỬ LÝ — assignment is an ATTRIBUTE (C3): no status moves. The unit and the officer are
	// checked live in this commune with identity (503 when it cannot answer).
	//
	// @summary  Chuyển đơn thư cho một bộ phận (và cán bộ, nếu chọn) kèm lý do — không đổi trạng thái, ghi nhật ký
	// @screen   05-van-ban-don-thu §3.5
	// @request  letterRoutingIn
	// @reply    200 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/{id}/routings",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.KhongCan("mỗi lần chuyển là một hành vi có thật và để lại một dòng nhật ký riêng — nhật ký chỉ thêm (luật 7 cấm #5), gửi lại là một lần chuyển nữa chứ không phải bản sao")(
				http.HandlerFunc(h.RouteCitizenLetter))))

	// ĐỔI TRẠNG THÁI along C3's arrows. A second identical request is refused by the table itself
	// (the letter is no longer in the `from` status), so it writes nothing.
	//
	// @summary  Đổi trạng thái đơn thư theo TT 05/2021 (C3); cán bộ được giao hoặc người có quyền tiếp nhận; khiếu nại / tố cáo cần kết quả trước khi sang Đã giải quyết
	// @screen   05-van-ban-don-thu §3.5
	// @request  letterStatusIn
	// @reply    200 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/{id}/status",
		authz.RequirePermission(d.Checker, "petition.read")(
			idem.KhongCan("lần gửi thứ hai bị bảng chuyển trạng thái từ chối (đơn đã rời trạng thái cũ) nên trả 409 và không ghi gì")(
				http.HandlerFunc(h.MoveCitizenLetter))))

	// KẾT QUẢ GIẢI QUYẾT (C10, narrowed by ADR 0084 #2) — a full replacement, so PUT. khieu-nai /
	// to-cao: all five fields. kien-nghi-phan-anh / de-nghi: the reply (`result_summary`) alone.
	//
	// @summary  Ghi kết quả giải quyết khi đơn đang Thụ lý hoặc Đang giải quyết: khiếu nại / tố cáo cần văn bản đã ban hành (số, ngày, người ký, cơ quan) và tóm tắt; kiến nghị-phản ánh / đề nghị chỉ cần nội dung trả lời
	// @screen   05-van-ban-don-thu §3.5
	// @request  letterResultIn
	// @reply    200 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/citizen-letters/{id}/result",
		authz.RequirePermission(d.Checker, "petition.read")(
			idem.KhongCan("ghi đè một trạng thái đã biết; app.RecordResult không ghi gì khi giá trị không đổi, nên lần gửi thứ hai để lại đúng một dòng nhật ký và một vết")(
				http.HandlerFunc(h.RecordCitizenLetterResult))))

	// SỬA NGƯỜI GỬI — correction or Decree 13 anonymisation. Values never enter the log or the trail.
	//
	// @summary  Sửa họ tên / SĐT / địa chỉ người gửi (trường vắng giữ nguyên, null hoặc rỗng là xoá); nhật ký và vết không ghi giá trị
	// @screen   05-van-ban-don-thu §3.5
	// @request  senderCorrectionIn
	// @reply    200 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/citizen-letters/{id}/sender",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.KhongCan("ghi đè một trạng thái đã biết; app.CorrectSender không ghi gì khi không trường nào đổi")(
				http.HandlerFunc(h.CorrectCitizenLetterSender))))

	// HẠN XỬ LÝ — the clerk's deadline (ADR 0079 lô 5 Q18; prototype PetitionDetailDrawer.tsx:519-523,
	// set through PATCH /petitions/{id} under `petition.create`, router.py:430-433). Optional: null
	// clears it ("Không đặt"). Stored as sent, on the CURRENT phase's column; refused (409) on a
	// finished letter. The audit entry carries the deadline before and after.
	//
	// @summary  Đặt hoặc bỏ hạn xử lý của đơn thư do cán bộ nhập (null = Không đặt); lưu nguyên giá trị, áp cho giai đoạn hiện tại của đơn
	// @screen   05-van-ban-don-thu §3.5
	// @request  letterDeadlineIn
	// @reply    200 citizenLetterOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/citizen-letters/{id}/deadline",
		authz.RequirePermission(d.Checker, "petition.create")(
			idem.KhongCan("ghi đè một trạng thái đã biết; app.SetDeadline không ghi gì và không để vết khi hạn không đổi")(
				http.HandlerFunc(h.SetCitizenLetterDeadline))))

	// GHI NHẬT KÝ — idem.Required(MoKhiHong): a double-submitted note is a duplicate line in an
	// append-only log that can never be removed, but refusing an officer's note during a cache outage
	// costs more than that rare duplicate.
	//
	// @summary  Ghi một dòng nhật ký (ghi chú) cho đơn thư — cán bộ được giao hoặc người có quyền tiếp nhận
	// @screen   05-van-ban-don-thu §3.5
	// @request  letterNoteIn
	// @reply    201 letterLogEntryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-letters/{id}/log-entries",
		authz.RequirePermission(d.Checker, "petition.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.AddCitizenLetterNote))))

	// BÁO CÁO — one resource per commune and year, so singular. Denunciations are counted, never named.
	//
	// @summary  Báo cáo sổ đơn thư một năm: nhận, đã giải quyết, đang xử lý (gồm năm trước chuyển sang), quá hạn, tỷ lệ đúng hạn, số ngày trung bình, theo loại · bộ phận · tháng
	// @screen   05-van-ban-don-thu §4
	// @reply    200 letterReportOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-letter-report",
		authz.RequirePermission(d.Checker, "petition.read")(
			http.HandlerFunc(h.CitizenLetterReport)))
}
