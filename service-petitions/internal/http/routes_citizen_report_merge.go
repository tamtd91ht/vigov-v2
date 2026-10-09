package http

// Merging duplicate petitions (ADR 0087; ADR 0041 §Sửa đổi 09/10/2026; owner answers of 09/10/2026).
// THREE ROUTES, EVERY KEY SEEDED in `quyen` — `feedback.read` (service-identity/migrations/0001_init.sql)
// and `feedback.classify` (service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql:58, the key
// ADR 0087 §5 names). None invented (rule 5, invariant 3c). Handlers: citizen_report_merge.go.
//
// A FILE OF ITS OWN, registerCitizenReportFigureRoutes' shape, so routes.go grows by one line.
//
// ⚠ THE PATH SEGMENTS `merge` / `unmerge` ARE THE ONES THE MAIN SESSION FIXED FOR THIS CARD (a parallel
// web-admin card builds against them). `unmerge` is a verb where skills/rest-api-design §3 asks for a
// nominalised sub-resource; kb/00-foundation/ubiquitous-language.md has no row for "gộp phiếu" yet
// (migration 0037's header), so the noun is not this file's to coin. Reported, not renamed here.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerCitizenReportMergeRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// At construction, not at the first request — the reason the switch in Register gives.
	switch {
	case d.CitizenReportMerge == nil:
		panic("petitions/http: thiếu use case gộp phiếu — POST /api/v1/citizen-reports/{maTraCuu}/merge và /unmerge sẽ panic khi có người gọi")
	case d.MergeLinks == nil:
		panic("petitions/http: thiếu đường đọc liên kết gộp — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.DuplicateCandidates == nil:
		panic("petitions/http: thiếu use case phiếu nghi trùng — GET /api/v1/citizen-reports/{maTraCuu}/duplicate-candidates sẽ panic khi có người gọi")
	}

	// PHIẾU NGHI TRÙNG — the commune's own radius and window (migration 0038; ADR 0087 §6), closest first,
	// in the list's item shape: same commune, live, not merged, unresolved, never `can-bo`, the same field
	// when both are classified. A SUGGESTION: it merges nothing.
	//
	// `feedback.read`, THE LIST'S KEY: the rows are rows the register list already shows, reporters masked.
	// `can-bo` without `feedback.restricted` is the detail's 404. NO idem.*: a GET. NO AUDIT ENTRY: nothing
	// unmasked is read.
	//
	// @summary  Phiếu phản ánh nghi trùng với một phiếu — cùng xã, trong bán kính và số ngày xã cấu hình, gần nhất trước
	// @screen   09-phan-anh-nguoi-dan §8
	// @reply    200 duplicateCandidatesOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/duplicate-candidates",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DuplicateCandidatesOf)))

	// GỘP PHIẾU — the petition in the PATH becomes linked to `main_code` (ADR 0087 §1: a link, not a status).
	// One transaction: the main's deadline moved to the earlier one first, the link, the history row, two
	// audit entries. 409 `merge_state` (status, chain, can-bo, already merged) · `merge_deadline_before_origin`
	// (migration 0004 would refuse the main's inherited deadline — choose the earlier-reported petition as
	// the main one) · `petition_state` (moved meanwhile). 404: either code unknown, another commune's,
	// soft-deleted, or `can-bo` without `feedback.restricted` — one body.
	//
	// `feedback.classify` (ADR 0087 §5). idem.KhongCan: the link UPDATE carries `merged_into IS NULL`, so a
	// second identical request is refused 409 (already merged) — one link, one history row, one pair of
	// entries.
	//
	// @summary  Gộp phiếu phản ánh trùng vào một phiếu chính cùng xã — liên kết, không đóng phiếu; hạn phiếu chính lấy mốc sớm hơn
	// @screen   09-phan-anh-nguoi-dan §8
	// @request  citizenReportMergeIn
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/merge",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `merged_into IS NULL`, nên lần gửi thứ hai trả 409 đã gộp — đúng một liên kết, một dòng lịch sử, một cặp vết")(
				http.HandlerFunc(h.MergeCitizenReport))))

	// TÁCH PHIẾU — the link cleared, reason MANDATORY (owner, 09/10/2026), the main petition's deadline NOT
	// lengthened back (ADR 0087 §Hệ quả). Only while the petition is still unresolved: unmerging one that
	// already followed its main into `cho-dan-xac-nhan` / `da-dong` is decided by nobody (migration 0037).
	//
	// `feedback.classify`. idem.KhongCan: the UPDATE carries `merged_into = <main>`, so a second request is
	// 409 (not merged) — one unlink, one history row.
	//
	// @summary  Tách một phiếu phản ánh đã gộp khỏi phiếu chính — lý do bắt buộc, có vết, hạn phiếu chính giữ nguyên
	// @screen   09-phan-anh-nguoi-dan §8
	// @request  citizenReportUnmergeIn
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/unmerge",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `merged_into` của phiếu chính, nên lần gửi thứ hai trả 409 chưa gộp — đúng một lần tách, một dòng lịch sử")(
				http.HandlerFunc(h.UnmergeCitizenReport))))
}
