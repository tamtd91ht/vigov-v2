package http

// The leadership overview (/tong-quan) routes of the two registers this service owns: one summary of
// figures per register, and one "Cần xử lý ngay" overdue queue per register.
//
// EVERY FIGURE HAS A LIST. The `metric` values accepted by GET /api/v1/tasks and
// GET /api/v1/citizen-reports are the JSON field names of the two summaries below, and each is read
// through the SAME predicate the figure is counted with (store.taskMetricCondition,
// store.citizenReportMetricCondition). So `task-summary.overdue == 12` and
// `GET /api/v1/tasks?metric=overdue` lists twelve rows, with no second definition to drift.
//
// NO PERCENTAGE ON THE WIRE. The client divides, and shows a dash when a sample is zero; a server that
// answered 0% for an empty sample would publish a false figure about a public authority.
//
// THE PERIOD IS THE CLIENT'S. `from` and `to` are RFC 3339 instants, half-open [from, to); the screen
// decides the week (Monday start, Asia/Ho_Chi_Minh) and the "same elapsed portion" comparison and asks
// twice. Both bounds are required on the summaries, because each summary carries period figures;
// `from >= to` is refused.

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The read sides the four routes depend on. NARROW, one per register, for the reason every reader
// in routes.go states: a route depending on a wider surface than it uses is how the next person
// justifies reaching through it.
type (
	// TaskSummaryReader — *petstore.NhiemVuStore satisfies it. TaskUnitSummary is the same register's
	// figures split by department (/bao-cao); ONE reader for both, because both are counts over the
	// one table through the one predicate set — a second field would be a second wiring of one store.
	TaskSummaryReader interface {
		TaskSummary(ctx context.Context, p domain.Period) (domain.TaskSummary, error)
		TaskUnitSummary(ctx context.Context, p domain.Period) (domain.TaskUnitSummary, error)
	}

	// CitizenReportSummaryReader — *petstore.PhieuPhanAnhStore satisfies it. `restricted` is the
	// reader's `feedback.restricted` fact; false excludes the `can-bo` field.
	CitizenReportSummaryReader interface {
		CitizenReportSummary(ctx context.Context, p domain.Period, restricted bool) (
			domain.CitizenReportSummary, error)
	}

	// OverdueQueueReader — *app.OverdueQueue satisfies it. A USE CASE and not a store: `critical`
	// needs identity's working-hours calendar.
	OverdueQueueReader interface {
		Tasks(ctx context.Context, limit int) ([]domain.OverdueItem, error)
		CitizenReports(ctx context.Context, limit int, restricted app.QuyenXemHanChe) ([]domain.OverdueItem, error)
	}
)

// --- replies -------------------------------------------------------------------------------------

// taskSummaryOut is GET /api/v1/task-summary. The field names ARE the `metric` values of
// GET /api/v1/tasks (domain.TaskMetric).
//
//	in_progress · overdue · suspended   stock — the register now; `from`/`to` do not move them
//	completed · on_time_sample · on_time  period — inside [from, to)
//
// The on-time ratio is on_time / on_time_sample, measured against the ORIGINAL deadline
// (`han_ban_dau`), and it has no value when on_time_sample is 0.
type taskSummaryOut struct {
	InProgress   int `json:"in_progress"`
	Overdue      int `json:"overdue"`
	Suspended    int `json:"suspended"`
	Completed    int `json:"completed"`
	OnTimeSample int `json:"on_time_sample"`
	OnTime       int `json:"on_time"`
}

// taskUnitSummaryOut is GET /api/v1/task-unit-summary — the /bao-cao table "Tình hình thực hiện theo
// bộ phận". `units` is [] — never null — when no task is in scope.
type taskUnitSummaryOut struct {
	// AsOf is the database instant the `overdue` figures were measured at.
	AsOf  time.Time        `json:"as_of"`
	Units []taskUnitRowOut `json:"units"`
}

// taskUnitRowOut is one department. ONLY DEPARTMENTS WITH A TASK IN SCOPE are listed; the client adds
// names and zero rows from GET /api/v1/org-units.
//
//	total                                 tasks in hand during [from, to)
//	completed · on_time_sample · on_time  period — the task-summary figures of the same names
//	overdue                               stock — task-summary's `overdue`, as of `as_of`
type taskUnitRowOut struct {
	// OrgUnitID is identity's `bo_phan` id; "" for tasks with no department ("— Chưa xác định —").
	OrgUnitID    string `json:"org_unit_id"`
	Total        int    `json:"total"`
	Completed    int    `json:"completed"`
	OnTimeSample int    `json:"on_time_sample"`
	OnTime       int    `json:"on_time"`
	Overdue      int    `json:"overdue"`
}

// citizenReportSummaryOut is GET /api/v1/citizen-report-summary. The field names ARE the `metric`
// values of GET /api/v1/citizen-reports (domain.CitizenReportMetric).
//
//	in_progress                        stock
//	received · on_time_sample · on_time · late   period
//
// on_time + late == on_time_sample. The sample includes petitions that missed the classification
// ceiling (open question #26, ADR 0035 §C), always as late — see store.citizenReportMetricCondition.
//
//	rating_sample · low_rating · publication_pending   stock (2026-10-02, docs/ui-ux/09 §3 cards 3–4)
//	rating_sum                                         NOT a metric: the stars summed over the
//	                                                   rating_sample rows; average = sum / sample,
//	                                                   divided by the client, dash when sample is 0
//
// THE FOUR ARE POINTERS WITH omitempty, SET ON EVERY RESPONSE — the ReopenCount precedent: omitempty makes
// tools/apidoc declare them optional so yesterday's fixtures stay valid, the pointer makes 0 travel as 0.
// Absent therefore means "this server predates the field", never zero.
type citizenReportSummaryOut struct {
	Received     int `json:"received"`
	InProgress   int `json:"in_progress"`
	OnTimeSample int `json:"on_time_sample"`
	OnTime       int `json:"on_time"`
	Late         int `json:"late"`

	// RatingSample is the number of petitions carrying a CITIZEN's rating (ADR 0062 decision 2 — staff
	// cannot record one). Drill-down: `metric=rating_sample`.
	RatingSample *int `json:"rating_sample,omitempty"`
	// RatingSum is the sum of their stars, 1–5 each.
	RatingSum *int `json:"rating_sum,omitempty"`
	// LowRating is "n phiếu bị đánh giá thấp": rated 1–2 stars — the SAME rows as
	// GET /api/v1/citizen-reports?rating_max=2 and as `metric=low_rating`.
	LowRating *int `json:"low_rating,omitempty"`
	// PublicationPending is "CHỜ KIỂM DUYỆT": `publication_status = cho-duyet`, never counting `can-bo`
	// (it can never be published). Drill-down: `metric=publication_pending`.
	PublicationPending *int `json:"publication_pending,omitempty"`
}

// overdueItemOut is one row of a "Cần xử lý ngay" panel. NO PERSONAL DATA and no free text — code,
// category key and deadline only (domain.OverdueItem).
type overdueItemOut struct {
	// Kind is WHICH deadline was missed: `han-xu-ly` (task), `han-phan-loai` (petition still
	// unclassified past its ceiling) or `han-xu-ly-xong` (petition past its resolve deadline).
	Kind string `json:"kind"`
	// Code is the task number or the petition lookup code — what the record's detail route takes.
	Code string `json:"code"`
	// CategoryCode is the task-type code or the petition field code, for the client to resolve against
	// the commune's catalogue. EMPTY for a petition that is still unclassified.
	CategoryCode string `json:"category_code"`
	// MissedDeadline is the stored deadline that passed. The list is ordered by it, oldest first.
	MissedDeadline time.Time `json:"missed_deadline"`
	// Critical is true once CriticalWorkingHours WORKING hours have passed since MissedDeadline, as
	// identity's calendar counts them.
	Critical bool `json:"critical"`
	// LateWorkingSeconds is the WORKING time, in whole seconds, between MissedDeadline and the instant
	// the server judged the row overdue — identity's MeasureWorkingHours on the commune's calendar.
	// PETITION PANEL ONLY. ABSENT — never 0 — when it could not be measured (identity unavailable,
	// calendar not configured): "Quá hạn" stands without it. Measured on every read, never stored; the
	// screen shows no number yet (ADR 0007 decision 10).
	LateWorkingSeconds *uint64 `json:"late_working_seconds,omitempty"`
}

// overdueQueueOut wraps the rows. `items` is [] — never null — when nothing is overdue.
type overdueQueueOut struct {
	Items []overdueItemOut `json:"items"`
}

// --- parsing -------------------------------------------------------------------------------------

var (
	errPeriodMissing = errors.New(
		"`from` và `to` là bắt buộc: hai mốc thời gian RFC 3339, khoảng nửa mở [from, to)")
	errPeriodFormat = errors.New(
		"`from`/`to` phải là mốc thời gian RFC 3339 có múi giờ, ví dụ 2026-09-28T00:00:00+07:00")
	errPeriodOrder = errors.New("`from` phải sớm hơn `to`")
	errTaskMetric  = errors.New(
		"`metric` không phải một chỉ số tổng quan nhiệm vụ: in_progress, overdue, suspended, " +
			"completed, on_time_sample, on_time")
	errCitizenReportMetric = errors.New(
		"`metric` không phải một chỉ số tổng quan phản ánh: received, in_progress, on_time_sample, " +
			"on_time, late, rating_sample, low_rating, publication_pending")
	errQueueLimit = errors.New("`limit` phải là số nguyên dương")
)

// firstValue is the first value of one query parameter, "" when absent.
func firstValue(q map[string][]string, k string) string {
	if v, ok := q[k]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// parsePeriod reads `from` and `to`. BOTH REQUIRED; REFUSED, NEVER REPAIRED — a missing bound read as
// "all time", or a swapped pair silently put right, answers a question the screen did not ask with a
// number that looks entirely plausible.
func parsePeriod(q map[string][]string) (domain.Period, error) {
	from, to := firstValue(q, "from"), firstValue(q, "to")
	if from == "" || to == "" {
		return domain.Period{}, errPeriodMissing
	}
	f, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return domain.Period{}, errPeriodFormat
	}
	t, err := time.Parse(time.RFC3339, to)
	if err != nil {
		return domain.Period{}, errPeriodFormat
	}
	p, err := domain.NewPeriod(f, t)
	if err != nil {
		return domain.Period{}, errPeriodOrder
	}
	return p, nil
}

// parseQueueLimit reads `limit`: absent means OverdueQueueMax, above it is CLAMPED to it (a client
// asking for more gets the maximum, not an error — skills/rest-api-design §5), and anything that is
// not a positive integer is refused.
func parseQueueLimit(q map[string][]string) (int, error) {
	s := firstValue(q, "limit")
	if s == "" {
		return petstore.OverdueQueueMax, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, errQueueLimit
	}
	if n > petstore.OverdueQueueMax {
		n = petstore.OverdueQueueMax
	}
	return n, nil
}

// parseTaskMetric reads the drill-down pair of GET /api/v1/tasks. `metric` absent: nothing is read and
// nothing changes — `from`/`to` alone were ignored before this filter existed and still are. A stock
// metric ignores `from`/`to`; a period metric requires them.
func parseTaskMetric(q map[string][]string, loc *petstore.LocNhiemVu) error {
	s := firstValue(q, "metric")
	if s == "" {
		return nil
	}
	m := domain.TaskMetric(s)
	if !m.Valid() {
		return errTaskMetric
	}
	loc.Metric = m
	if m.PeriodBound() {
		p, err := parsePeriod(q)
		if err != nil {
			return err
		}
		loc.Period = p
	}
	return nil
}

// parseCitizenReportMetric is parseTaskMetric for GET /api/v1/citizen-reports.
func parseCitizenReportMetric(q map[string][]string, loc *petstore.LocPhieu) error {
	s := firstValue(q, "metric")
	if s == "" {
		return nil
	}
	m := domain.CitizenReportMetric(s)
	if !m.Valid() {
		return errCitizenReportMetric
	}
	loc.Metric = m
	if m.PeriodBound() {
		p, err := parsePeriod(q)
		if err != nil {
			return err
		}
		loc.Period = p
	}
	return nil
}

// --- handlers ------------------------------------------------------------------------------------

// TaskSummary serves GET /api/v1/task-summary.
//
// NO AUDIT ENTRY: counts over one commune's register, no personal data, no cross-commune read
// (rule 6, invariant 7).
func (h *Handler) TaskSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := parsePeriod(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	s, err := h.d.TaskSummary.TaskSummary(ctx, p)
	if err != nil {
		h.d.Log.Error("tổng quan nhiệm vụ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, taskSummaryOut{
		InProgress:   s.InProgress,
		Overdue:      s.Overdue,
		Suspended:    s.Suspended,
		Completed:    s.Completed,
		OnTimeSample: s.OnTimeSample,
		OnTime:       s.OnTime,
	})
}

// TaskUnitSummary serves GET /api/v1/task-unit-summary. Same period parsing and the same refusals as
// TaskSummary. NO AUDIT ENTRY, for the same reason: counts over one commune's register, no personal data.
func (h *Handler) TaskUnitSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := parsePeriod(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	s, err := h.d.TaskSummary.TaskUnitSummary(ctx, p)
	if err != nil {
		h.d.Log.Error("nhiệm vụ theo bộ phận: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := taskUnitSummaryOut{AsOf: s.AsOf, Units: make([]taskUnitRowOut, 0, len(s.Units))}
	for _, u := range s.Units {
		out.Units = append(out.Units, taskUnitRowOut{
			OrgUnitID:    u.OrgUnitID,
			Total:        u.Total,
			Completed:    u.Completed,
			OnTimeSample: u.OnTimeSample,
			OnTime:       u.OnTime,
			Overdue:      u.Overdue,
		})
	}
	vietJSON(w, http.StatusOK, out)
}

// CitizenReportSummary serves GET /api/v1/citizen-report-summary.
//
// `feedback.restricted` IS READ THROUGH THE SAME HELPER THE REGISTER LIST USES (coQuyenHanChe), so a
// reader without it gets figures that exclude the `can-bo` field — the same rows their drill-down list
// can reach. A figure that counted rows the list hides would come up short on click, and would tell a
// colleague that reports about staff exist.
func (h *Handler) CitizenReportSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := parsePeriod(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	s, err := h.d.CitizenReportSummary.CitizenReportSummary(ctx, p, bool(h.coQuyenHanChe(ctx)))
	if err != nil {
		h.d.Log.Error("tổng quan phản ánh: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, citizenReportSummaryOut{
		Received:     s.Received,
		InProgress:   s.InProgress,
		OnTimeSample: s.OnTimeSample,
		OnTime:       s.OnTime,
		Late:         s.Late,

		RatingSample:       &s.RatingSample,
		RatingSum:          &s.RatingSum,
		LowRating:          &s.LowRating,
		PublicationPending: &s.PublicationPending,
	})
}

// OverdueTasks serves GET /api/v1/overdue-tasks.
func (h *Handler) OverdueTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := parseQueueLimit(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	items, err := h.d.OverdueQueue.Tasks(ctx, n)
	h.writeOverdueQueue(w, r, items, err)
}

// OverdueCitizenReports serves GET /api/v1/overdue-citizen-reports. The restricted field is excluded
// unless the reader holds `feedback.restricted`, exactly as on the summary.
func (h *Handler) OverdueCitizenReports(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := parseQueueLimit(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	items, err := h.d.OverdueQueue.CitizenReports(ctx, n, h.coQuyenHanChe(ctx))
	h.writeOverdueQueue(w, r, items, err)
}

// writeOverdueQueue renders a queue, or its failure.
//
// 503 WHEN IDENTITY CANNOT ANSWER, NOT A LIST WITH `critical: false` (app.ErrWorkingHoursUnavailable):
// the panel would look calm at the one moment it cannot know. WARN, like the other identity refusals
// in this service — this process is healthy; the wrapped chain carries the gRPC code identityclient
// already logged.
func (h *Handler) writeOverdueQueue(w http.ResponseWriter, r *http.Request, items []domain.OverdueItem, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, app.ErrWorkingHoursUnavailable):
		h.d.Log.Warn("CẢNH BÁO: không dựng được hàng đợi quá hạn vì chưa tính được giờ làm việc",
			"xa", string(tenant.MustFrom(ctx)), "duong", r.URL.Path, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "working_hours_unavailable",
			"Chưa tính được mức độ nghiêm trọng theo lịch làm việc của xã. Vui lòng thử lại sau ít phút.", "")
		return
	case err != nil:
		h.d.Log.Error("hàng đợi quá hạn: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "duong", r.URL.Path, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := overdueQueueOut{Items: make([]overdueItemOut, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, overdueItemOut{
			Kind:           string(it.Kind),
			Code:           it.Code,
			CategoryCode:   it.CategoryCode,
			MissedDeadline: it.MissedDeadline,
			Critical:       it.Critical,

			LateWorkingSeconds: it.LateWorkingSeconds,
		})
	}
	vietJSON(w, http.StatusOK, out)
}
