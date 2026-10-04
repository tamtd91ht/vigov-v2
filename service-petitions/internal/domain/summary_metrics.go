package domain

// The leadership overview (/tong-quan) figures for the two registers this service owns — tasks and
// citizen reports — and the "Cần xử lý ngay" overdue queue.
//
// THIS FILE NAMES THE FIGURES; IT DOES NOT COMPUTE THEM. Each figure is a COUNT over rows, and the
// one predicate that decides which rows belong to it lives in internal/store as SQL, used by BOTH the
// count and the drill-down list (store.taskMetricCondition, store.citizenReportMetricCondition). That
// is the property the screen depends on: the number a leader clicks and the list that opens must be
// the same set of rows. A Go copy of the predicate here would be a second source for one fact, and
// the copy that drifts is the one on the report (rule 9, invariant 2).
//
// NO PERCENTAGE IS COMPUTED ANYWHERE ON THE SERVER. A ratio with a zero denominator has no value, and
// a server that answered 0% for "no sample" would print a false figure about a public authority. The
// client receives the sample size and the count and shows a dash for an empty sample.

import (
	"errors"
	"time"
)

// Period is a half-open interval [From, To) — the reporting window a period-bound figure counts in.
//
// THE CLIENT COMPUTES IT, NEVER THE SERVER. Which week, which month, where the week starts (Monday),
// which time zone (Asia/Ho_Chi_Minh) and what "the same elapsed portion" of the previous period is are
// all presentation decisions of the overview screen; the server only counts inside the instants it is
// handed. A second implementation of "this week" here would disagree with the screen on the one day a
// leader compares two numbers.
//
// HALF-OPEN SO TWO ADJACENT PERIODS NEVER COUNT ONE ROW TWICE: a task completed at exactly the
// boundary belongs to the later period only.
type Period struct {
	From time.Time
	To   time.Time
}

// ErrPeriodInvalid is returned for a missing bound or a From that is not strictly before To.
var ErrPeriodInvalid = errors.New("khoảng thời gian không hợp lệ")

// NewPeriod validates the two bounds. REFUSED, NEVER REPAIRED: swapping or clamping the bounds would
// answer a question the caller did not ask, with a figure that looks entirely plausible.
func NewPeriod(from, to time.Time) (Period, error) {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return Period{}, ErrPeriodInvalid
	}
	return Period{From: from, To: to}, nil
}

// --- tasks -----------------------------------------------------------------------------------------

// TaskMetric names one figure of the task register. The value is the JSON field name of that figure
// in the summary reply AND the `metric` value of GET /api/v1/tasks that lists its rows — one string
// for one row set, so the client cannot pair a figure with the wrong list.
type TaskMetric string

const (
	// Stock figures — the register as it stands now; no period.
	TaskInProgress TaskMetric = "in_progress"
	TaskOverdue    TaskMetric = "overdue"
	TaskSuspended  TaskMetric = "suspended"

	// Period figures — counted inside a Period.
	TaskCompleted    TaskMetric = "completed"
	TaskOnTimeSample TaskMetric = "on_time_sample"
	TaskOnTime       TaskMetric = "on_time"
)

// TaskMetrics is every task figure, in reply order.
var TaskMetrics = []TaskMetric{
	TaskInProgress, TaskOverdue, TaskSuspended, TaskCompleted, TaskOnTimeSample, TaskOnTime,
}

// Valid reports whether m is one of the six. FAIL CLOSED: an unknown metric is refused, never read as
// "no metric" — a drill-down that silently dropped its filter would open the whole register under a
// heading that names one figure.
func (m TaskMetric) Valid() bool {
	for _, k := range TaskMetrics {
		if k == m {
			return true
		}
	}
	return false
}

// PeriodBound reports whether the figure is counted inside a Period.
func (m TaskMetric) PeriodBound() bool {
	return m == TaskCompleted || m == TaskOnTimeSample || m == TaskOnTime
}

// TaskSummary is the task register's figures. Counts only — see the file comment on percentages.
type TaskSummary struct {
	InProgress   int
	Overdue      int
	Suspended    int
	Completed    int
	OnTimeSample int
	OnTime       int
}

// TaskUnitFigures is one department's (`bo_phan`) row of the /bao-cao table "Tình hình thực hiện theo
// bộ phận". Counts only, like TaskSummary.
//
// OrgUnitID IS EMPTY FOR TASKS WITH NO DEPARTMENT — "— Chưa xác định —" is a real state (migration
// 0006, `bo_phan_id` nullable), and dropping those rows would make the table add up to less than the
// register. Names are NOT here: they are identity's, and the client joins them from GET
// /api/v1/org-units.
type TaskUnitFigures struct {
	OrgUnitID string
	// Total is the tasks IN HAND during the period — see store.taskInHandCondition.
	Total int
	// Completed, OnTimeSample and OnTime are TaskSummary's period figures, split by department.
	Completed    int
	OnTimeSample int
	OnTime       int
	// Overdue is TaskSummary's stock figure (as of AsOf), split by department.
	Overdue int
}

// TaskUnitSummary is the per-department table. Units holds only departments with at least one task in
// one of the figures; the client adds zero rows for the rest.
type TaskUnitSummary struct {
	// AsOf is the DATABASE instant the overdue figure compared deadlines with (`now()` of the
	// statement) — the same clock the deadline was stored against.
	AsOf  time.Time
	Units []TaskUnitFigures
}

// --- citizen reports -------------------------------------------------------------------------------

// CitizenReportMetric names one figure of the petition register. Same contract as TaskMetric: the
// value is both the JSON field name and the `metric` value of GET /api/v1/citizen-reports.
type CitizenReportMetric string

const (
	// Stock.
	CitizenReportInProgress CitizenReportMetric = "in_progress"

	// Period.
	CitizenReportReceived     CitizenReportMetric = "received"
	CitizenReportOnTimeSample CitizenReportMetric = "on_time_sample"
	CitizenReportOnTime       CitizenReportMetric = "on_time"
	CitizenReportLate         CitizenReportMetric = "late"

	// Stock — the three figures of docs/ui-ux/09 §3 cards 3 and 4 ("ĐIỂM HÀI LÒNG TRUNG BÌNH · n phiếu
	// bị đánh giá thấp", "CHỜ KIỂM DUYỆT"). STOCK AND NOT PERIOD, and that is a choice stated as one: the
	// register's own `rating_max` filter — the list "Bị đánh giá thấp" opens — takes no period, and a
	// figure counted in [from, to) would not equal the list it drills into. A period variant is a
	// question for the screen's owner, not a default chosen here.
	//
	// CitizenReportRatingSample counts petitions that carry a citizen rating; the AVERAGE is
	// CitizenReportSummary.RatingSum / RatingSample, divided by the client like every ratio here.
	// ADR 0062 decision 2 ("only the citizen's own ratings") holds BY CONSTRUCTION: the only writer of
	// `diem_hai_long` is the citizen route (POST …/my-citizen-reports/{code}/rating, ADR 0062 decision 1).
	CitizenReportRatingSample CitizenReportMetric = "rating_sample"
	// CitizenReportLowRating counts petitions rated at most LowRatingMaxStars — the SAME predicate as
	// GET /api/v1/citizen-reports?rating_max=2, which is what web-admin's "Bị đánh giá thấp" sends.
	CitizenReportLowRating CitizenReportMetric = "low_rating"
	// CitizenReportPublicationPending counts `publication_status = 'cho-duyet'` (migration 0017, ADR
	// 0050 point 8) OUTSIDE the `can-bo` field — see store.citizenReportMetricCondition for why.
	CitizenReportPublicationPending CitizenReportMetric = "publication_pending"
)

// LowRatingMaxStars is the highest rating that counts as LOW: 1 or 2 stars — docs/ui-ux/09 §4 ("phiếu
// 1–2 sao") and the same number that reopens a petition (RatingReopenThreshold, ADR 0050 point 2).
// Tied to that constant rather than written again, so "low" and "reopens" can never drift apart.
const LowRatingMaxStars = RatingReopenThreshold

// CitizenReportMetrics is every petition figure, in reply order. The three stock figures added on
// 2026-10-02 are APPENDED, so the five before them keep their positions and their SQL.
var CitizenReportMetrics = []CitizenReportMetric{
	CitizenReportReceived, CitizenReportInProgress,
	CitizenReportOnTimeSample, CitizenReportOnTime, CitizenReportLate,
	CitizenReportRatingSample, CitizenReportLowRating, CitizenReportPublicationPending,
}

// Valid reports whether m is one of CitizenReportMetrics. Fail closed, as TaskMetric.Valid.
func (m CitizenReportMetric) Valid() bool {
	for _, k := range CitizenReportMetrics {
		if k == m {
			return true
		}
	}
	return false
}

// PeriodBound reports whether the figure is counted inside a Period. LISTED, not "everything but the
// stock one": with four stock figures a negative rule would make every new one period-bound by default.
func (m CitizenReportMetric) PeriodBound() bool {
	switch m {
	case CitizenReportReceived, CitizenReportOnTimeSample, CitizenReportOnTime, CitizenReportLate:
		return true
	}
	return false
}

// CitizenReportSummary is the petition register's figures.
//
// OnTime + Late == OnTimeSample, by construction of the two predicates (a row of the sample is late
// or it is on time, never both and never neither) — a test asserts it rather than a comment hoping.
type CitizenReportSummary struct {
	Received     int
	InProgress   int
	OnTimeSample int
	OnTime       int
	Late         int

	// RatingSample and RatingSum carry the average citizen rating as a numerator and a denominator:
	// the client divides and shows a dash for a zero sample (the file comment on percentages).
	// RatingSum is the sum of the stars of exactly the RatingSample rows.
	RatingSample       int
	RatingSum          int
	LowRating          int
	PublicationPending int
}

// --- the overdue queue -----------------------------------------------------------------------------

// DeadlineKind says WHICH commitment an overdue item missed. The values are the stored deadline
// columns, Vietnamese without diacritics like every enum value (ADR 0011) — the three are different
// promises with different owners, and an English word for all three would collapse them.
type DeadlineKind string

const (
	// A task's current deadline, `nhiem_vu.han_xu_ly` (after any granted extension).
	DeadlineTask DeadlineKind = "han-xu-ly"
	// A petition's classification ceiling, `phieu_phan_anh.han_phan_loai` (ADR 0035 §C) — missed
	// while the petition is still unclassified.
	DeadlineClassification DeadlineKind = "han-phan-loai"
	// A petition's resolve deadline, `phieu_phan_anh.han_xu_ly_xong` — missed while the work is not
	// done.
	DeadlineResolution DeadlineKind = "han-xu-ly-xong"
)

// OverdueItem is one row of the "Cần xử lý ngay" panel.
//
// NO PERSONAL DATA, AND THE ABSENCES ARE THE DESIGN (rule 3): no petition content, no reporter name
// or phone, no location, no task title (a task title quotes a citizen's complaint often enough that
// the task register's own sort allowlist refuses it). The panel identifies the record by its code and
// its category; everything else is one click away, behind the record's own read route and its own
// masking rules.
type OverdueItem struct {
	Kind DeadlineKind

	// Code is the task's register number (`nhiem_vu.ma`) or the petition's lookup code
	// (`phieu_phan_anh.ma_tra_cuu`) — the identifier the record's own detail route takes.
	Code string

	// CategoryCode is the category CODE the client resolves to the commune's label: the task type
	// (`nhiem_vu.loai`, GET /api/v1/task-types) or the petition field (`phieu_phan_anh.linh_vuc`,
	// the field-label catalogue). EMPTY for a petition that is still unclassified — which is exactly
	// the case DeadlineClassification describes, so the empty value is the true answer.
	CategoryCode string

	// MissedDeadline is the stored deadline that has passed. The queue is ordered by it, oldest
	// first.
	MissedDeadline time.Time

	// Critical is IsCritical's answer for this item.
	Critical bool

	// AsOf is the DATABASE instant the overdue predicate compared the deadline with (`now()` of the
	// statement that read the row). Petition queue only; zero on a task item. It exists so that
	// LateWorkingSeconds measures up to the SAME `now` that decided the row is late — a second clock
	// read later could disagree with it by the length of a network call, or by clock skew.
	AsOf time.Time

	// LateWorkingSeconds is the WORKING time elapsed between MissedDeadline and AsOf, in whole seconds,
	// as identity's MeasureWorkingHours counts it on the commune's calendar (ADR 0007). nil = NOT
	// MEASURED — identity unavailable, calendar not configured, or a task item — and NEVER 0 in that
	// case: 0 reads as "not late" on a row that is. Derived on every read and never stored (rule 10,
	// invariant 3). Never add it to an instant (rule 10, forbidden #2).
	LateWorkingSeconds *uint64
}

// CriticalWorkingHours is how far past its deadline, in WORKING hours, an overdue item becomes
// critical on the overview panel.
//
// A PRESENTATION THRESHOLD OF THE OVERVIEW SCREEN, NOT A COMMITMENT TO A CITIZEN: it moves no
// deadline and is stored nowhere, it only decides which rows of the panel are drawn in the stronger
// colour. It is therefore not a per-commune SLA figure (rule 10, forbidden #3). It IS counted in
// working hours by identity (AdvanceWorkingHours) and never added in wall-clock time here (rule 10,
// forbidden #2): "two working days late" is what the panel means, and 48 wall-clock hours over a
// weekend is zero working hours.
const CriticalWorkingHours = 48

// IsCritical decides an overdue item's colour from the instant identity answered for
// "CriticalWorkingHours working hours after the missed deadline".
//
// AT OR PAST THAT INSTANT IS CRITICAL — the boundary belongs to the more urgent side, so an item
// never flips back and forth across one clock tick between two reads.
func IsCritical(criticalFrom, now time.Time) bool {
	return !criticalFrom.After(now)
}
