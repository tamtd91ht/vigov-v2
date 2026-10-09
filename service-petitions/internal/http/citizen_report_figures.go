package http

// The /phan-anh statistics surface and the log-attachment removal (owner decisions of 09/10/2026,
// batch A). Routes: routes_citizen_report_figures.go.
//
//	GET    /api/v1/citizen-report-counts                               feedback.read
//	GET    /api/v1/citizen-report-points                               feedback.read
//	GET    /api/v1/citizen-report-breakdown                            feedback.read + report.read
//	DELETE /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}     feedback.read gate; uploader OR feedback.resolve
//
// THE COUNT AND THE POINTS TAKE THE LIST'S FILTERS THROUGH THE LIST'S PARSER (citizenReportFilter), and
// the store reads both through the list's WHERE (store.registerFilter) — so the total over the register,
// the dots on the map and the rows of the list are one set. `feedback.restricted` absent -> `can-bo` is
// excluded from all three by the same fact.
//
// NO AUDIT ENTRY ON THE THREE READS: counts, coordinates and statuses of one commune — no reporter, no
// content, no lookup code (rule 6, invariant 7 asks for neither case). The removal is audited by the use
// case, in its transaction.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CitizenReportBreakdownReader is app.CitizenReportBreakdown. A use case and not a store: the unit
// section is measured in WORKING hours by identity.
type CitizenReportBreakdownReader interface {
	Read(ctx context.Context, p domain.Period, restricted app.QuyenXemHanChe) (domain.CitizenReportBreakdown, error)
}

// --- replies ---------------------------------------------------------------------------------------

// citizenReportCountsOut is GET /api/v1/citizen-report-counts: how many rows the list holds under the
// same filters. A sibling route and not a field on the list — page.Result carries no total by design
// (task-counts is the precedent).
type citizenReportCountsOut struct {
	Total int `json:"total"`
}

// citizenReportPointOut is one dot on the heat map — the scene coordinates and the status, NOTHING ELSE.
// No lookup code, no content, no reporter (rule 3): a code beside a coordinate turns a heat map into a
// lookup table of who reported what where.
type citizenReportPointOut struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	// Status is one of the nine codes (`dang-xu-ly`, …) — Vietnamese without diacritics (ADR 0011).
	Status string `json:"status"`
}

// citizenReportPointsOut wraps the dots. `items` is [] — never null — when nothing is located.
type citizenReportPointsOut struct {
	Items []citizenReportPointOut `json:"items"`
}

// citizenReportBreakdownTotalsOut is section (a): the /tong-quan tile's on-time figures for the period
// (on_time + late == on_time_sample) and the overdue STOCK as of `as_of`.
type citizenReportBreakdownTotalsOut struct {
	OnTimeSample int `json:"on_time_sample"`
	OnTime       int `json:"on_time"`
	Late         int `json:"late"`
	Overdue      int `json:"overdue"`
}

// citizenReportFieldRowOut is one field (section (b)). `field_code` "" is the row of petitions still
// UNCLASSIFIED — a separate row, labelled by the client, never merged into a field.
//
//	received · finished · on_time_sample · on_time · late   period [from, to)
//	rating_sample · rating_sum · overdue                    stock (rating as on citizen-report-summary)
//
// The average rating is rating_sum / rating_sample, divided by the client (dash for 0).
type citizenReportFieldRowOut struct {
	FieldCode    string `json:"field_code"`
	Received     int    `json:"received"`
	Finished     int    `json:"finished"`
	OnTimeSample int    `json:"on_time_sample"`
	OnTime       int    `json:"on_time"`
	Late         int    `json:"late"`
	RatingSample int    `json:"rating_sample"`
	RatingSum    int    `json:"rating_sum"`
	Overdue      int    `json:"overdue"`
}

// citizenReportUnitRowOut is one unit (section (c)): petitions FINISHED in the period, attributed to the
// unit holding each at `xu_ly_xong_luc`. `org_unit_id` "" is the row of petitions finished with no
// recorded hand-over. The average handling time in WORKING hours is
// handling_working_seconds / handling_sample / 3600, divided by the client — dash when the sample is 0.
// Names and zero rows come from GET /api/v1/org-units.
type citizenReportUnitRowOut struct {
	OrgUnitID              string `json:"org_unit_id"`
	Finished               int    `json:"finished"`
	HandlingSample         int    `json:"handling_sample"`
	HandlingWorkingSeconds uint64 `json:"handling_working_seconds"`
}

// citizenReportResidentialUnitRowOut is one thôn / tổ dân phố (section (d)): received in the period,
// `khong-tiep-nhan` EXCLUDED, and the overdue stock. `residential_unit_id` "" is "Chưa xác định địa bàn".
// Names come from GET /api/v1/residential-units (identity).
type citizenReportResidentialUnitRowOut struct {
	ResidentialUnitID string `json:"residential_unit_id"`
	Received          int    `json:"received"`
	Overdue           int    `json:"overdue"`
}

// citizenReportBreakdownOut is GET /api/v1/citizen-report-breakdown. Every list is [] — never null —
// when empty. NO RATIO AND NO AVERAGE (domain/summary_metrics.go:13-15).
type citizenReportBreakdownOut struct {
	// AsOf is the database instant the stock figures were measured at.
	AsOf             time.Time                            `json:"as_of"`
	Totals           citizenReportBreakdownTotalsOut      `json:"totals"`
	Fields           []citizenReportFieldRowOut           `json:"fields"`
	Units            []citizenReportUnitRowOut            `json:"units"`
	ResidentialUnits []citizenReportResidentialUnitRowOut `json:"residential_units"`
}

// citizenReportLogAttachmentRemoveIn is the body of DELETE …/log-attachments/{id}. A BODY ON A DELETE,
// the task twin's reason: the reason is mandatory (rule 7, invariant 1), and a query string would put
// free text about a government record into every access log.
type citizenReportLogAttachmentRemoveIn struct {
	Reason string `json:"reason"`
}

// --- the shared filter -------------------------------------------------------------------------------

// citizenReportFilter is the register list's filter, read ONCE for the list, the total and the map:
// locPhieuTuQuery's refusals, `scope=mine` filled from the SESSION's business code (never the URL), and
// the restricted-field fact. It writes the refusal itself and answers false.
func (h *Handler) citizenReportFilter(w http.ResponseWriter, r *http.Request) (petstore.LocPhieu, bool) {
	ctx := r.Context()
	loc, onlyMine, err := locPhieuTuQuery(r.URL.Query())
	if err != nil {
		message := err.Error()
		if errors.Is(err, petstore.ErrTimPhieuQuaDai) {
			// The store's sentinel carries its package prefix (`phieu_phan_anh:`); the other refusals
			// of locPhieuTuQuery are this package's own sentences and are already fit to show.
			message = fmt.Sprintf("Chuỗi tìm kiếm quá dài (tối đa %d ký tự).", petstore.TimPhieuToiDa)
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", message, "")
		return petstore.LocPhieu{}, false
	}
	if onlyMine {
		// FAIL CLOSED ON AN EMPTY CODE: dropping the filter would answer "Giao cho tôi" with the whole
		// register (DanhSachPhieu says why).
		p, ok := authz.From(ctx)
		if !ok || p.Ma == "" {
			h.thieuChuTheXuLy(w, r)
			return petstore.LocPhieu{}, false
		}
		loc.CanBoXuLyID = p.Ma
	}
	loc.ChoPhepHanChe = bool(h.coQuyenHanChe(ctx))
	return loc, true
}

// --- handlers ---------------------------------------------------------------------------------------

// CitizenReportCounts serves GET /api/v1/citizen-report-counts. Paging parameters are not read: they
// choose which rows of the set are shown, not which rows are in it.
func (h *Handler) CitizenReportCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loc, ok := h.citizenReportFilter(w, r)
	if !ok {
		return
	}
	n, err := h.d.DanhSachPhieu.CountCitizenReports(ctx, loc)
	if err != nil {
		h.d.Log.Error("đếm phiếu phản ánh theo bộ lọc: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, citizenReportCountsOut{Total: n})
}

// CitizenReportPoints serves GET /api/v1/citizen-report-points.
func (h *Handler) CitizenReportPoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loc, ok := h.citizenReportFilter(w, r)
	if !ok {
		return
	}
	points, err := h.d.DanhSachPhieu.CitizenReportPoints(ctx, loc)
	switch {
	case errors.Is(err, petstore.ErrTooManyCitizenReportPoints):
		// 422 AND NOT A TRUNCATED MAP: a heat map with petitions silently missing reads as "nothing
		// happened there". The officer narrows the filter (service-comms map-asset-points precedent).
		httpx.WriteError(w, http.StatusUnprocessableEntity, "too_many_points",
			fmt.Sprintf("Bộ lọc hiện có hơn %d phản ánh có vị trí — quá nhiều để vẽ trên bản đồ. "+
				"Hãy thu hẹp bộ lọc (theo lĩnh vực, trạng thái hoặc thôn) rồi xem lại.", petstore.CitizenReportPointsCeiling), "")
		return
	case err != nil:
		// The wrapped error carries the store failure, never a coordinate (rule 3).
		h.d.Log.Error("điểm phản ánh trên bản đồ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenReportPointsOut{Items: make([]citizenReportPointOut, 0, len(points))}
	for _, p := range points {
		out.Items = append(out.Items, citizenReportPointOut{Lat: p.Lat, Lng: p.Lng, Status: string(p.Status)})
	}
	vietJSON(w, http.StatusOK, out)
}

// CitizenReportBreakdown serves GET /api/v1/citizen-report-breakdown. The period is the client's, as on
// /tong-quan and /bao-cao (ADR 0053 §3): `from`/`to` RFC 3339, half-open, Tuần/Tháng/Quý/Năm computed in
// Asia/Ho_Chi_Minh by the screen — parsePeriod, the summaries' parser.
func (h *Handler) CitizenReportBreakdown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := parsePeriod(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	b, err := h.d.CitizenReportBreakdown.Read(ctx, p, h.coQuyenHanChe(ctx))
	switch {
	case errors.Is(err, app.ErrWorkingCalendarMissing):
		// 409: the request is fine and so is the service; the commune's calendar is not configured, and
		// a handling time in clock hours instead would be rule 10, forbidden #2.
		h.d.Log.Warn("CẢNH BÁO: thống kê phản ánh từ chối vì xã chưa cấu hình lịch làm việc",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusConflict, "working_calendar_not_configured",
			"Xã chưa cấu hình lịch làm việc nên chưa tính được thời gian xử lý theo giờ làm việc. "+
				"Hãy báo quản trị của xã cấu hình lịch làm việc rồi xem lại.", "")
		return
	case errors.Is(err, app.ErrWorkingHoursUnavailable):
		// 503, NEVER A WALL-CLOCK FALLBACK and never the unit table quietly emptied.
		h.d.Log.Warn("CẢNH BÁO: thống kê phản ánh từ chối vì chưa đo được giờ làm việc",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "working_hours_unavailable",
			"Chưa tính được thời gian xử lý theo lịch làm việc của xã. Vui lòng thử lại sau ít phút.", "")
		return
	case err != nil:
		h.d.Log.Error("thống kê phản ánh theo kỳ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenReportBreakdownOut{
		AsOf: b.AsOf,
		Totals: citizenReportBreakdownTotalsOut{OnTimeSample: b.Totals.OnTimeSample, OnTime: b.Totals.OnTime,
			Late: b.Totals.Late, Overdue: b.Totals.Overdue},
		Fields:           make([]citizenReportFieldRowOut, 0, len(b.Fields)),
		Units:            make([]citizenReportUnitRowOut, 0, len(b.Units)),
		ResidentialUnits: make([]citizenReportResidentialUnitRowOut, 0, len(b.ResidentialUnits)),
	}
	for _, f := range b.Fields {
		out.Fields = append(out.Fields, citizenReportFieldRowOut{FieldCode: f.FieldCode, Received: f.Received,
			Finished: f.Finished, OnTimeSample: f.OnTimeSample, OnTime: f.OnTime, Late: f.Late,
			RatingSample: f.RatingSample, RatingSum: f.RatingSum, Overdue: f.Overdue})
	}
	for _, u := range b.Units {
		out.Units = append(out.Units, citizenReportUnitRowOut{OrgUnitID: u.OrgUnitID, Finished: u.Finished,
			HandlingSample: u.HandlingSample, HandlingWorkingSeconds: u.HandlingWorkingSeconds})
	}
	for _, ru := range b.ResidentialUnits {
		out.ResidentialUnits = append(out.ResidentialUnits, citizenReportResidentialUnitRowOut{
			ResidentialUnitID: ru.ResidentialUnitID, Received: ru.Received, Overdue: ru.Overdue})
	}
	vietJSON(w, http.StatusOK, out)
}

// RemovePetitionLogAttachment serves DELETE /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}.
func (h *Handler) RemovePetitionLogAttachment(w http.ResponseWriter, r *http.Request) {
	var in citizenReportLogAttachmentRemoveIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	// THE ONE FACT THIS HANDLER READS: does the caller hold `feedback.resolve`? Whether that, or being the
	// uploader, opens THIS file is the use case's, on the locked row. Fail closed: no principal, false.
	var resolve app.QuyenXuLyCaXa
	if principal, has := authz.From(ctx); has {
		resolve = app.QuyenXuLyCaXa(h.d.Checker.Allows(ctx, principal, QuyenXuLyCaXa))
	}
	err := h.d.PetitionLogAttachments.Remove(ctx, r.PathValue("maTraCuu"), r.PathValue("id"), in.Reason, actor,
		resolve, h.coQuyenHanChe(ctx))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, domain.ErrAttachmentRemovalNotAllowed):
		// 403 and the petition's own sentence (the domain one names a TASK permission).
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Chỉ người đã tải tệp lên hoặc cán bộ có quyền kết thúc xử lý phản ánh mới gỡ được tệp này.", "")
	case errors.Is(err, domain.ErrAttachmentRemovalReasonMissing):
		// The petition's own sentence: the domain one names a TASK record.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Phải ghi lý do gỡ tệp — tệp đính kèm là một phần hồ sơ xử lý phản ánh.", "")
	case errors.Is(err, domain.ErrAttachmentRemovalReasonTooLong):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("Lý do gỡ tệp quá dài (tối đa %d ký tự).", domain.MaxAttachmentRemovalReasonRunes), "")
	case errors.Is(err, domain.ErrAttachmentUnderLegalHold):
		httpx.WriteError(w, http.StatusConflict, "legal_hold",
			"Tệp đang được giữ để phục vụ khiếu nại hoặc thanh tra nên chưa thể gỡ. "+
				"Khi việc giữ tệp kết thúc, bạn mới gỡ được.", "")
	default:
		// 404 for an invisible file, the petition's 404, the 500 — answered as every log-attachment route.
		h.answerPetitionLogAttachmentError(w, r, "gỡ tệp đính kèm nhật ký", err)
	}
}
