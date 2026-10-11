package http

// The staff-recorded rating — "Ghi nhận đánh giá của người dân" (owner rule 11/10/2026, bug sheet row 60;
// vigov-require FeedbackDetailDrawer.tsx:516-540, router.py:391-405). ONE ROUTE, its key SEEDED in `quyen`:
// `feedback.resolve` (service-identity/migrations/0001_init.sql) — the prototype's own key for this act, and
// the key of the petition-processing step it follows (closing). None invented (rule 5, invariant 3c).
// Handler: petition_staff_recorded_rating.go.
//
// A FILE OF ITS OWN, registerCitizenReportMergeRoutes' shape, so routes.go grows by one line.
//
// ⚠ THE PATH SEGMENT `staff-recorded-rating` is coined here: kb/00-foundation/ubiquitous-language.md names
// the citizen's `rating` (POST /api/v1/my-citizen-reports/{maTraCuu}/rating) and nothing for a rating staff
// record. A nominalised sub-resource (skills/rest-api-design §3) that says WHOSE act it is, so it can never
// be read as the citizen's. Reported for the glossary, not added there.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerStaffRecordedRatingRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// At construction, not at the first request — GET /api/v1/citizen-reports/{maTraCuu} reads it too.
	if d.StaffRecordedRatings == nil {
		panic("petitions/http: thiếu use case đánh giá do cán bộ ghi nhận — POST …/staff-recorded-rating và " +
			"GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	}

	// GHI NHẬN ĐÁNH GIÁ CỦA NGƯỜI DÂN — what the citizen said by phone (`dien-thoai`) or at the counter
	// (`tai-quay`), recorded by staff: 1–5 stars, an optional note. Stored with `source = staff_recorded` in
	// its own table (migration 0042) and NEVER counted in a satisfaction figure — those read the citizen's
	// own rating only (ADR 0062 decision 2, kept). Any status (the prototype's block has no status gate).
	// It moves no status, reopens nothing and notifies nobody (rule 10 stop conditions — see migration 0042).
	//
	// 409 `citizen_rated` — the citizen has rated already; their verdict is authoritative. 409
	// `staff_rating_exists` — one staff record per petition. 404: unknown, another commune's, soft-deleted,
	// or `can-bo` without `feedback.restricted` — one body. Audited in the same transaction.
	//
	// idem.KhongCan: UNIQUE (tenant_id, petition_id), checked under the petition's row lock — a second
	// identical request answers 409 and writes nothing.
	//
	// @summary  Cán bộ ghi nhận đánh giá người dân nói qua điện thoại hoặc tại quầy — lưu riêng (nguồn staff_recorded), không tính vào điểm hài lòng; người dân đã tự đánh giá thì từ chối
	// @screen   09-phan-anh-nguoi-dan §8.6
	// @request  staffRecordedRatingIn
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error citizen_rated staff_rating_exists
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/staff-recorded-rating",
		authz.RequirePermission(d.Checker, "feedback.resolve")(
			idem.KhongCan("khoá UNIQUE (tenant_id, petition_id) kiểm dưới khoá dòng phiếu, nên lần gửi thứ hai trả 409 — đúng một bản ghi, một vết")(
				http.HandlerFunc(h.RecordStaffRating))))
}
