package http

// The petition ↔ citizen-directory link (ADR 0102) — two READ routes. Handlers: contact_petition.go.
//
// EVERY KEY SEEDED in `quyen`, none invented (rule 5, invariant 3c): `feedback.read` (identity
// 0001_init.sql), `feedback.unmask` (0007_quyen_phan_loai_va_xem_day_du.sql), `citizen.read` and
// `citizen.update` (0028_citizen_dossier_notice_permissions.sql). The PAIRS are ADR 0102 #5, owner
// decision 17/10/2026, and the contract of each RPC in proto/vigov/identity/v1/identity.proto says
// petitions checks exactly that pair before it calls:
//
//	"Đồng bộ danh bạ" on a petition    feedback.unmask + citizen.update
//	"Xem phản ánh" on a directory row  citizen.read + feedback.read (+ feedback.restricted for `can-bo`)
//
// TWO NESTED RequirePermission = BOTH keys (the routes_citizen_report_figures.go shape); authz checks
// the commune before either key, so a session of another commune is 401 on both.
//
// A FILE OF ITS OWN, registerCitizenReportMergeRoutes' shape, so routes.go grows by one line.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

func registerContactPetitionRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// At construction, not at the first request — the reason the switch in Register gives.
	switch {
	case d.SenderDirectory == nil:
		panic("petitions/http: thiếu đường hỏi danh bạ người dân (identity) — GET …/directory-match và " +
			"GET /api/v1/citizen-report-lists/by-contact/{contactId} sẽ panic khi có người gọi")
	case d.ContactPetitions == nil:
		panic("petitions/http: thiếu đường đọc phiếu theo khoá đối chiếu số — GET …/directory-match và " +
			"GET /api/v1/citizen-report-lists/by-contact/{contactId} sẽ panic khi có người gọi")
	}

	// ĐỒNG BỘ DANH BẠ — is this petition's sender in the commune's citizen directory: `found` + the row id
	// (open its detail), `not_found` (open "Thêm người dân" pre-filled), `not_applicable` (anonymous, or no
	// usable number — identity is not asked). `phone_unverified` (ADR 0080) drives the dialog's warning.
	// identity unreachable → 503, never not_found. `can-bo` without `feedback.restricted` is the detail's 404.
	//
	// `feedback.unmask` + `citizen.update` (ADR 0102 #5). NO idem.*: a GET. NO AUDIT ENTRY: no personal
	// data leaves — a yes/no and an opaque id.
	//
	// @summary  Đối chiếu người gửi của một phiếu phản ánh với danh bạ người dân của xã — có / chưa có / không áp dụng (ẩn danh hoặc không có số)
	// @screen   09-phan-anh-nguoi-dan §8
	// @reply    200 directoryMatchOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error citizen_directory_unavailable
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/directory-match",
		authz.RequirePermission(d.Checker, "feedback.unmask")(
			authz.RequirePermission(d.Checker, "citizen.update")(
				http.HandlerFunc(h.DirectoryMatch))))

	// XEM PHẢN ÁNH — one directory row's petitions, newest booked first, `page` (1-based) and `page_size`
	// (default 10, served at most 50). The browser sends the ROW ID only; the number is resolved by identity
	// (one audit entry there per call), used for this service's own query and dropped (ADR 0102 #2, #8).
	// Anonymous and deleted petitions never listed; `can-bo` only with `feedback.restricted`. An unknown row
	// is 200 with an empty list. Rows carry no reporter.
	//
	// `citizen.read` + `feedback.read` (ADR 0102 #5). NO idem.*: a GET. `Cache-Control: no-store`.
	//
	// @summary  Danh sách phản ánh của một người trong danh bạ người dân — theo mã dòng danh bạ, phân trang; không kèm người gửi
	// @screen   16-danh-ba-nguoi-dan (Xem phản ánh, ADR 0102)
	// @reply    200 contactPetitionPageOut
	// @reply    400 httpx.Error invalid_page
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error citizen_directory_unavailable
	mux.Handle("GET /api/v1/citizen-report-lists/by-contact/{contactId}",
		authz.RequirePermission(d.Checker, "citizen.read")(
			authz.RequirePermission(d.Checker, "feedback.read")(
				http.HandlerFunc(h.ContactPetitionsOf))))
}
