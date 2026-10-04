package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The leadership overview routes — summary.go.
//
// WHAT THIS PROVES: each of the four routes needs BOTH `report.read` and the module key; a session of
// another commune is refused before any read; the period is parsed, refused when malformed, and handed
// down unchanged; the restricted-field fact reaches the petition reads; the queue is bounded and fails
// closed when identity cannot answer; and the drill-down `metric` reaches the two list stores.
//
// WHAT IT DOES NOT PROVE: that the SQL counts the right rows. That lives in internal/store — the
// shared-predicate tests there, and the PostgreSQL suite that skips without VIGOV_TEST_DSN.

// --- fakes ----------------------------------------------------------------------------------------

// taskSummaryFake answers per commune, from the context, the way *store.Scoped does — keyed any other
// way, the isolation cases would pass while proving nothing. `calls` proves a refused request read
// nothing.
type taskSummaryFake struct {
	byTenant map[tenant.ID]domain.TaskSummary
	calls    int
	period   domain.Period

	// The per-department read (/bao-cao). Its own counter, so a test of one route cannot be satisfied
	// by a call to the other.
	unitsByTenant map[tenant.ID]domain.TaskUnitSummary
	unitCalls     int
	unitPeriod    domain.Period
}

func (f *taskSummaryFake) TaskSummary(ctx context.Context, p domain.Period) (domain.TaskSummary, error) {
	f.calls++
	f.period = p
	return f.byTenant[tenant.MustFrom(ctx)], nil
}

func (f *taskSummaryFake) TaskUnitSummary(ctx context.Context, p domain.Period) (domain.TaskUnitSummary, error) {
	f.unitCalls++
	f.unitPeriod = p
	return f.unitsByTenant[tenant.MustFrom(ctx)], nil
}

var unitsAsOf = time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)

// Commune A and commune B carry DIFFERENT, all-distinct figures, so a leak and a field swap both show
// as wrong numbers.
func taskSummarySample() *taskSummaryFake {
	return &taskSummaryFake{byTenant: map[tenant.ID]domain.TaskSummary{
		xaA: {InProgress: 24, Overdue: 14, Suspended: 3, Completed: 9, OnTimeSample: 7, OnTime: 5},
		xaB: {InProgress: 1, Overdue: 2, Suspended: 4, Completed: 8, OnTimeSample: 16, OnTime: 32},
	}, unitsByTenant: map[tenant.ID]domain.TaskUnitSummary{
		xaA: {AsOf: unitsAsOf, Units: []domain.TaskUnitFigures{
			{OrgUnitID: "", Total: 3, Completed: 1, OnTimeSample: 1, OnTime: 0, Overdue: 2},
			{OrgUnitID: "bp-van-phong", Total: 11, Completed: 7, OnTimeSample: 6, OnTime: 5, Overdue: 4},
		}},
		xaB: {AsOf: unitsAsOf, Units: []domain.TaskUnitFigures{
			{OrgUnitID: "bp-xa-b", Total: 99, Completed: 98, OnTimeSample: 97, OnTime: 96, Overdue: 95},
		}},
	}}
}

type citizenReportSummaryFake struct {
	byTenant   map[tenant.ID]domain.CitizenReportSummary
	calls      int
	period     domain.Period
	restricted bool
}

func (f *citizenReportSummaryFake) CitizenReportSummary(ctx context.Context, p domain.Period,
	restricted bool) (domain.CitizenReportSummary, error) {
	f.calls++
	f.period, f.restricted = p, restricted
	return f.byTenant[tenant.MustFrom(ctx)], nil
}

func citizenReportSummarySample() *citizenReportSummaryFake {
	return &citizenReportSummaryFake{byTenant: map[tenant.ID]domain.CitizenReportSummary{
		xaA: {Received: 2, InProgress: 14, OnTimeSample: 6, OnTime: 4, Late: 2,
			RatingSample: 3, RatingSum: 11, LowRating: 1, PublicationPending: 16},
		xaB: {Received: 50, InProgress: 60, OnTimeSample: 70, OnTime: 30, Late: 40,
			RatingSample: 80, RatingSum: 90, LowRating: 25, PublicationPending: 35},
	}}
}

type overdueQueueFake struct {
	tasks      map[tenant.ID][]domain.OverdueItem
	reports    map[tenant.ID][]domain.OverdueItem
	calls      int
	limit      int
	restricted app.QuyenXemHanChe
	err        error
}

func (f *overdueQueueFake) Tasks(ctx context.Context, limit int) ([]domain.OverdueItem, error) {
	f.calls++
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks[tenant.MustFrom(ctx)], nil
}

func (f *overdueQueueFake) CitizenReports(ctx context.Context, limit int, restricted app.QuyenXemHanChe) (
	[]domain.OverdueItem, error) {
	f.calls++
	f.limit, f.restricted = limit, restricted
	if f.err != nil {
		return nil, f.err
	}
	return f.reports[tenant.MustFrom(ctx)], nil
}

var missedAt = time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)

func overdueQueueSample() *overdueQueueFake {
	return &overdueQueueFake{
		tasks: map[tenant.ID][]domain.OverdueItem{
			xaA: {{Kind: domain.DeadlineTask, Code: "NV07", CategoryCode: "theo-van-ban",
				MissedDeadline: missedAt, Critical: true}},
			xaB: {{Kind: domain.DeadlineTask, Code: "NV99", CategoryCode: "kiem-tra", MissedDeadline: missedAt}},
		},
		reports: map[tenant.ID][]domain.OverdueItem{
			xaA: {
				{Kind: domain.DeadlineClassification, Code: "PA-4K7M-92XR-BTVD", CategoryCode: "",
					MissedDeadline: missedAt, Critical: true},
				{Kind: domain.DeadlineResolution, Code: "PA-7F3K-9QXR-MNPT", CategoryCode: "moi-truong",
					MissedDeadline: missedAt.Add(time.Hour)},
			},
			xaB: {{Kind: domain.DeadlineResolution, Code: "PA-BBBB-BBBB-BBBB", MissedDeadline: missedAt}},
		},
	}
}

// --- helpers --------------------------------------------------------------------------------------

// grantBoth gives the commune-A account exactly `keys` in commune A AND in commune B — granting in B
// too is what makes the wrong-commune case refuse on the commune, not on a missing grant.
func grantBoth(m *mayChu, t *testing.T, keys ...string) {
	t.Helper()
	set := map[authz.Perm]bool{}
	for _, k := range keys {
		set[authz.Perm(k)] = true
	}
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: set},
			xaB: {idCanBo: set},
		}}
	})
}

const periodQuery = "?from=2026-09-22T00:00:00%2B07:00&to=2026-09-29T00:00:00%2B07:00"

var (
	periodFrom = time.Date(2026, 9, 22, 0, 0, 0, 0, time.FixedZone("", 7*3600))
	periodTo   = time.Date(2026, 9, 29, 0, 0, 0, 0, time.FixedZone("", 7*3600))
)

// overviewRoute is one of the four routes, with the module key it needs besides `report.read` and a
// way to read how many times its dependency was reached.
type overviewRoute struct {
	name, path, moduleKey string
	calls                 func(m *mayChu) int
}

func overviewRoutes() []overviewRoute {
	return []overviewRoute{
		{"task-summary", "/api/v1/task-summary" + periodQuery, "task.read",
			func(m *mayChu) int { return m.taskSummary.calls }},
		{"task-unit-summary", "/api/v1/task-unit-summary" + periodQuery, "task.read",
			func(m *mayChu) int { return m.taskSummary.unitCalls }},
		{"citizen-report-summary", "/api/v1/citizen-report-summary" + periodQuery, "feedback.read",
			func(m *mayChu) int { return m.reportSummary.calls }},
		{"overdue-tasks", "/api/v1/overdue-tasks", "task.read",
			func(m *mayChu) int { return m.overdue.calls }},
		{"overdue-citizen-reports", "/api/v1/overdue-citizen-reports", "feedback.read",
			func(m *mayChu) int { return m.overdue.calls }},
	}
}

// --- rule 5, invariant 7: the four cases, on every route -------------------------------------------

func TestOverviewRoutesNoSession401(t *testing.T) {
	for _, r := range overviewRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, r.path, nil)
			doiMa(t, w, http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Errorf("đã đọc %d lần khi chưa đăng nhập", r.calls(m))
			}
		})
	}
}

// TestOverviewRoutesNeedBothKeys — ONE KEY IS NOT ENOUGH, EITHER WAY ROUND. The module key alone would
// hand the report to anybody who can read the register; `report.read` alone would hand a register's
// figures to a reader who may not open that register.
func TestOverviewRoutesNeedBothKeys(t *testing.T) {
	for _, r := range overviewRoutes() {
		for _, only := range []string{r.moduleKey, "report.read"} {
			t.Run(r.name+"/chỉ "+only, func(t *testing.T) {
				m := dungMayChu(t)
				grantBoth(m, t, only)
				w := m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaA))
				doiMa(t, w, http.StatusForbidden)
				if r.calls(m) != 0 {
					t.Errorf("đã đọc %d lần cho tài khoản thiếu một khoá", r.calls(m))
				}
			})
		}
	}
}

// TestOverviewRoutesWrongCommune401 — both keys, granted in BOTH communes, and a session issued by
// commune B presented at commune A. authz.xacNhanXa refuses it before any permission is read, which is
// why the answer is 401 and not 403 (the same answer every guarded route of this service gives).
func TestOverviewRoutesWrongCommune401(t *testing.T) {
	for _, r := range overviewRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.moduleKey, "report.read")
			w := m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaB))
			doiMa(t, w, http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Errorf("đã đọc %d lần cho một phiên của xã khác", r.calls(m))
			}
		})
	}
}

func TestOverviewRoutesBothKeysRightCommune200(t *testing.T) {
	for _, r := range overviewRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.moduleKey, "report.read")
			w := m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusOK)
			if r.calls(m) != 1 {
				t.Errorf("đọc %d lần, muốn 1", r.calls(m))
			}
		})
	}
}

// --- the summaries --------------------------------------------------------------------------------

func TestTaskSummaryReturnsOwnCommuneFiguresAndPeriod(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "task.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/task-summary"+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var out map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON số: %v — %s", err, w.Body.String())
	}
	want := map[string]int{"in_progress": 24, "overdue": 14, "suspended": 3,
		"completed": 9, "on_time_sample": 7, "on_time": 5}
	if len(out) != len(want) {
		t.Errorf("trả %d trường, muốn %d: %v", len(out), len(want), out)
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %d, muốn %d (số của xã A)", k, out[k], v)
		}
	}
	// THE FIELD NAMES ARE THE `metric` VALUES of the drill-down — one string for one row set.
	for k := range out {
		if !domain.TaskMetric(k).Valid() {
			t.Errorf("trường %q không phải một metric của GET /api/v1/tasks", k)
		}
	}
	if !m.taskSummary.period.From.Equal(periodFrom) || !m.taskSummary.period.To.Equal(periodTo) {
		t.Errorf("kỳ xuống kho = %+v, muốn [%v, %v)", m.taskSummary.period, periodFrom, periodTo)
	}
	// NO RATIO ON THE WIRE — the client divides and shows a dash for an empty sample.
	for _, cam := range []string{"rate", "ratio", "percent", "pct"} {
		if strings.Contains(w.Body.String(), cam) {
			t.Errorf("phản hồi mang tỷ lệ %q — máy chủ chỉ trả số đếm", cam)
		}
	}
}

// TestTaskUnitSummaryReturnsOwnCommuneRows — commune A's rows only, the no-department row kept as
// `org_unit_id: ""`, the exact key set per row, the period handed down unchanged, no ratio.
func TestTaskUnitSummaryReturnsOwnCommuneRows(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "task.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/task-unit-summary"+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var out struct {
		AsOf  time.Time        `json:"as_of"`
		Units []map[string]any `json:"units"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
	if !out.AsOf.Equal(unitsAsOf) {
		t.Errorf("as_of = %v, muốn %v", out.AsOf, unitsAsOf)
	}
	if len(out.Units) != 2 {
		t.Fatalf("trả %d dòng, muốn 2 của xã A: %s", len(out.Units), w.Body.String())
	}
	if out.Units[0]["org_unit_id"] != "" || out.Units[0]["total"] != float64(3) || out.Units[0]["overdue"] != float64(2) {
		t.Errorf("dòng chưa xác định bộ phận = %v", out.Units[0])
	}
	want := map[string]float64{"total": 11, "completed": 7, "on_time_sample": 6, "on_time": 5, "overdue": 4}
	for k, v := range want {
		if out.Units[1][k] != v {
			t.Errorf("%s = %v, muốn %v", k, out.Units[1][k], v)
		}
	}
	for _, u := range out.Units {
		var keys []string
		for k := range u {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if got := strings.Join(keys, ","); got != "completed,on_time,on_time_sample,org_unit_id,overdue,total" {
			t.Errorf("khoá của một dòng = %s", got)
		}
		if u["org_unit_id"] == "bp-xa-b" {
			t.Error("bộ phận của xã B lọt vào bảng xã A")
		}
	}
	if !m.taskSummary.unitPeriod.From.Equal(periodFrom) || !m.taskSummary.unitPeriod.To.Equal(periodTo) {
		t.Errorf("kỳ xuống kho = %+v", m.taskSummary.unitPeriod)
	}
	for _, cam := range []string{"rate", "ratio", "percent", "rank"} {
		if strings.Contains(w.Body.String(), cam) {
			t.Errorf("phản hồi mang %q — chỉ số đếm, không xếp hạng", cam)
		}
	}
}

// TestTaskUnitSummaryEmptyIsArray — `units` is [] when nothing is in scope, never null.
func TestTaskUnitSummaryEmptyIsArray(t *testing.T) {
	m := dungMayChu(t)
	m.taskSummary.unitsByTenant[xaA] = domain.TaskUnitSummary{AsOf: unitsAsOf}
	grantBoth(m, t, "task.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/task-unit-summary"+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"units":[]`) {
		t.Errorf("thân = %s, muốn units là []", w.Body.String())
	}
}

func TestCitizenReportSummaryFieldsAreMetrics(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-report-summary"+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var out map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON số: %v", err)
	}
	want := map[string]int{"received": 2, "in_progress": 14, "on_time_sample": 6, "on_time": 4, "late": 2,
		"rating_sample": 3, "rating_sum": 11, "low_rating": 1, "publication_pending": 16}
	if len(out) != len(want) {
		t.Errorf("trả %d trường, muốn %d: %v", len(out), len(want), out)
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %d, muốn %d", k, out[k], v)
		}
		// rating_sum IS THE ONE FIELD THAT IS NOT A COUNT OF ROWS — the numerator of the average over
		// the rating_sample rows — so it has no list of its own. Every other field is a drill-down.
		if k != "rating_sum" && !domain.CitizenReportMetric(k).Valid() {
			t.Errorf("trường %q không phải một metric của GET /api/v1/citizen-reports", k)
		}
	}
	if domain.CitizenReportMetric("rating_sum").Valid() {
		t.Error("rating_sum là tổng số sao, không phải số phiếu — không được là một metric có danh sách")
	}
}

// TestCitizenReportSummaryNewFiguresZeroTravelsAsZero — the three stock figures are optional in the
// contract but SET on every response: a commune with nothing rated gets 0 and 0, never an absent key
// (absent means "server predates the field").
func TestCitizenReportSummaryNewFiguresZeroTravelsAsZero(t *testing.T) {
	m := dungMayChu(t)
	m.reportSummary.byTenant[xaA] = domain.CitizenReportSummary{Received: 1}
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-report-summary"+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	for _, k := range []string{`"rating_sample":0`, `"rating_sum":0`, `"low_rating":0`, `"publication_pending":0`} {
		if !strings.Contains(w.Body.String(), k) {
			t.Errorf("thiếu %s: %s", k, w.Body.String())
		}
	}
	// NO RATIO ON THE WIRE — the average is the client's division.
	for _, cam := range []string{"average", "avg", "ratio", "percent"} {
		if strings.Contains(w.Body.String(), cam) {
			t.Errorf("phản hồi mang tỷ lệ %q", cam)
		}
	}
}

func TestCitizenReportListNewStockMetricsIgnorePeriod(t *testing.T) {
	for _, m := range []domain.CitizenReportMetric{domain.CitizenReportRatingSample, domain.CitizenReportLowRating,
		domain.CitizenReportPublicationPending} {
		t.Run(string(m), func(t *testing.T) {
			s := dungMayChu(t)
			doiMa(t, s.goi(t, http.MethodGet, hostA, "/api/v1/citizen-reports?metric="+string(m), canBoCuaXa(xaA)),
				http.StatusOK)
			if s.danhSach.loc.Metric != m || !s.danhSach.loc.Period.From.IsZero() {
				t.Errorf("metric=%s xuống kho = %+v", m, s.danhSach.loc)
			}
		})
	}
}

// TestCitizenReportSummaryRestrictedFact — the SAME fact the register list uses. Without
// `feedback.restricted` the store is told to exclude `can-bo`; with it, not.
func TestCitizenReportSummaryRestrictedFact(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-report-summary"+periodQuery, canBoCuaXa(xaA)),
		http.StatusOK)
	if m.reportSummary.restricted {
		t.Error("không có feedback.restricted mà kho vẫn được bảo đếm cả lĩnh vực can-bo")
	}

	grantBoth(m, t, "feedback.read", "report.read", "feedback.restricted")
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-report-summary"+periodQuery, canBoCuaXa(xaA)),
		http.StatusOK)
	if !m.reportSummary.restricted {
		t.Error("có feedback.restricted mà kho vẫn bị bảo loại lĩnh vực can-bo")
	}
}

// TestSummaryPeriodRefused — every malformed period is 400 and reaches no store. REFUSED, NEVER
// REPAIRED: a swapped pair silently put right is a figure for a question nobody asked.
func TestSummaryPeriodRefused(t *testing.T) {
	for name, q := range map[string]string{
		"thiếu cả hai":        "",
		"thiếu to":            "?from=2026-09-22T00:00:00Z",
		"thiếu from":          "?to=2026-09-29T00:00:00Z",
		"không phải RFC 3339": "?from=2026-09-22&to=2026-09-29",
		"không múi giờ":       "?from=2026-09-22T00:00:00&to=2026-09-29T00:00:00",
		"from bằng to":        "?from=2026-09-22T00:00:00Z&to=2026-09-22T00:00:00Z",
		"from sau to":         "?from=2026-09-29T00:00:00Z&to=2026-09-22T00:00:00Z",
	} {
		for _, r := range []struct{ path, key string }{
			{"/api/v1/task-summary", "task.read"},
			{"/api/v1/task-unit-summary", "task.read"},
			{"/api/v1/citizen-report-summary", "feedback.read"},
		} {
			t.Run(name+" "+r.path, func(t *testing.T) {
				m := dungMayChu(t)
				grantBoth(m, t, r.key, "report.read")
				w := m.goi(t, http.MethodGet, hostA, r.path+q, canBoCuaXa(xaA))
				doiMa(t, w, http.StatusBadRequest)
				if m.taskSummary.calls+m.taskSummary.unitCalls+m.reportSummary.calls != 0 {
					t.Error("kỳ sai mà vẫn chạm kho")
				}
			})
		}
	}
}

// --- the queues -----------------------------------------------------------------------------------

func TestOverdueQueueLimit(t *testing.T) {
	for _, c := range []struct {
		q      string
		status int
		limit  int
	}{
		{"", http.StatusOK, 10},
		{"?limit=3", http.StatusOK, 3},
		{"?limit=10", http.StatusOK, 10},
		// Above the maximum is CLAMPED, not refused (skills/rest-api-design §5).
		{"?limit=500", http.StatusOK, 10},
		{"?limit=0", http.StatusBadRequest, 0},
		{"?limit=-1", http.StatusBadRequest, 0},
		{"?limit=abc", http.StatusBadRequest, 0},
	} {
		for _, r := range []struct{ path, key string }{
			{"/api/v1/overdue-tasks", "task.read"},
			{"/api/v1/overdue-citizen-reports", "feedback.read"},
		} {
			t.Run(r.path+c.q, func(t *testing.T) {
				m := dungMayChu(t)
				grantBoth(m, t, r.key, "report.read")
				w := m.goi(t, http.MethodGet, hostA, r.path+c.q, canBoCuaXa(xaA))
				doiMa(t, w, c.status)
				if c.status == http.StatusOK && m.overdue.limit != c.limit {
					t.Errorf("limit xuống use case = %d, muốn %d", m.overdue.limit, c.limit)
				}
				if c.status != http.StatusOK && m.overdue.calls != 0 {
					t.Error("limit sai mà vẫn chạm use case")
				}
			})
		}
	}
}

// TestOverdueQueueCarriesNoPersonalData — THE EXACT KEY SET of an item, asserted on the raw body, so a
// field added later under ANY name (a title, the content, the reporter) turns this red (rule 3).
func TestOverdueQueueCarriesNoPersonalData(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/overdue-citizen-reports", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("trả %d dòng, muốn 2 của xã A", len(out.Items))
	}
	for _, it := range out.Items {
		var keys []string
		for k := range it {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if got := strings.Join(keys, ","); got != "category_code,code,critical,kind,missed_deadline" {
			t.Errorf("khoá của một dòng = %s — chỉ được mã, loại hạn, mã lĩnh vực, hạn đã lỡ, cờ nghiêm trọng", got)
		}
		if it["code"] == "PA-BBBB-BBBB-BBBB" {
			t.Error("phiếu của xã B lọt vào hàng đợi xã A")
		}
	}
	if out.Items[0]["kind"] != string(domain.DeadlineClassification) || out.Items[0]["critical"] != true {
		t.Errorf("dòng đầu = %v, muốn hạn phân loại, nghiêm trọng", out.Items[0])
	}
	if m.overdue.restricted {
		t.Error("không có feedback.restricted mà use case vẫn được bảo lấy cả lĩnh vực can-bo")
	}
}

// TestOverdueQueueLateWorkingSecondsOptional — present as a number when the use case measured it,
// ABSENT (never 0, never null) when it did not, and the page is 200 either way.
func TestOverdueQueueLateWorkingSecondsOptional(t *testing.T) {
	m := dungMayChu(t)
	measured := uint64(97213)
	zero := uint64(0)
	m.overdue.reports[xaA] = []domain.OverdueItem{
		{Kind: domain.DeadlineResolution, Code: "PA-AAAA-AAAA-AAAA", MissedDeadline: missedAt, LateWorkingSeconds: &measured},
		{Kind: domain.DeadlineResolution, Code: "PA-CCCC-CCCC-CCCC", MissedDeadline: missedAt},
		// A measured 0 is a real answer (deadline missed outside working hours) and travels as 0.
		{Kind: domain.DeadlineResolution, Code: "PA-DDDD-DDDD-DDDD", MissedDeadline: missedAt, LateWorkingSeconds: &zero},
	}
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/overdue-citizen-reports", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Items[0]["late_working_seconds"] != float64(97213) {
		t.Errorf("dòng đo được = %v", out.Items[0]["late_working_seconds"])
	}
	if _, ok := out.Items[1]["late_working_seconds"]; ok {
		t.Errorf("dòng không đo được vẫn mang khoá late_working_seconds: %v", out.Items[1])
	}
	if out.Items[2]["late_working_seconds"] != float64(0) {
		t.Errorf("số 0 đo được bị bỏ: %v", out.Items[2])
	}
}

// TestOverdueQueueEmptyIsArray — `items` is [] on a commune with nothing overdue, never null.
func TestOverdueQueueEmptyIsArray(t *testing.T) {
	m := dungMayChu(t)
	m.overdue.tasks = map[tenant.ID][]domain.OverdueItem{}
	grantBoth(m, t, "task.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/overdue-tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
		t.Errorf("thân = %s, muốn {\"items\":[]}", w.Body.String())
	}
}

// TestOverdueQueueIdentityDown503 — FAIL CLOSED: no list with every row `critical: false`.
func TestOverdueQueueIdentityDown503(t *testing.T) {
	for _, r := range []struct{ path, key string }{
		{"/api/v1/overdue-tasks", "task.read"},
		{"/api/v1/overdue-citizen-reports", "feedback.read"},
	} {
		t.Run(r.path, func(t *testing.T) {
			m := dungMayChu(t)
			m.overdue.err = errors.Join(app.ErrWorkingHoursUnavailable, errors.New("rpc Unavailable"))
			grantBoth(m, t, r.key, "report.read")
			w := m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusServiceUnavailable)
			if e := loiTra(t, w); e.Code != "working_hours_unavailable" {
				t.Errorf("mã lỗi = %q", e.Code)
			}
			if strings.Contains(w.Body.String(), "critical") {
				t.Error("phản hồi lỗi vẫn mang cờ critical")
			}
		})
	}
}

// --- the drill-down filters -----------------------------------------------------------------------

func TestTaskListMetricReachesStore(t *testing.T) {
	m := dungMayChu(t)

	// A STOCK metric: from/to are ignored, the period stays zero.
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?metric=overdue"+strings.Replace(periodQuery, "?", "&", 1),
		canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi.Metric != domain.TaskOverdue || !m.nhiemVu.locCuoi.Period.From.IsZero() {
		t.Errorf("metric=overdue xuống kho = %+v", m.nhiemVu.locCuoi)
	}

	// A PERIOD metric carries its period.
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?metric=on_time"+strings.Replace(periodQuery, "?", "&", 1),
		canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi.Metric != domain.TaskOnTime ||
		!m.nhiemVu.locCuoi.Period.From.Equal(periodFrom) || !m.nhiemVu.locCuoi.Period.To.Equal(periodTo) {
		t.Errorf("metric=on_time xuống kho = %+v", m.nhiemVu.locCuoi)
	}
}

func TestTaskListMetricRefused(t *testing.T) {
	for name, q := range map[string]string{
		"metric lạ":            "?metric=late",
		"kỳ thiếu":             "?metric=completed",
		"kỳ ngược":             "?metric=completed&from=2026-09-29T00:00:00Z&to=2026-09-22T00:00:00Z",
		"kỳ sai định dạng":     "?metric=on_time_sample&from=hom-qua&to=hom-nay",
		"tên metric của phiếu": "?metric=received",
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+q, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if m.nhiemVu.goi != 0 {
				t.Error("bộ lọc sai mà vẫn chạm kho")
			}
		})
	}
}

// TestTaskListWithoutMetricUnchanged — `from`/`to` alone were ignored before the filter existed and
// still are: the existing route's behaviour does not move.
func TestTaskListWithoutMetricUnchanged(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+periodQuery, canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi.Metric != "" || !m.nhiemVu.locCuoi.Period.From.IsZero() {
		t.Errorf("không có metric mà bộ lọc xuống kho vẫn đổi: %+v", m.nhiemVu.locCuoi)
	}
}

func TestCitizenReportListMetricReachesStore(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA,
		"/api/v1/citizen-reports?metric=late"+strings.Replace(periodQuery, "?", "&", 1), canBoCuaXa(xaA)),
		http.StatusOK)
	loc := m.danhSach.loc
	if loc.Metric != domain.CitizenReportLate || !loc.Period.From.Equal(periodFrom) || !loc.Period.To.Equal(periodTo) {
		t.Errorf("metric=late xuống kho = %+v", loc)
	}
	// The restricted exclusion still applies to a drill-down — the SAME fact the figure used.
	if loc.ChoPhepHanChe {
		t.Error("drill-down mở lĩnh vực can-bo cho tài khoản không có feedback.restricted")
	}
}

func TestCitizenReportListMetricRefused(t *testing.T) {
	for name, q := range map[string]string{
		"metric lạ":           "?metric=overdue",
		"kỳ thiếu":            "?metric=received",
		"kỳ bằng nhau":        "?metric=on_time&from=2026-09-22T00:00:00Z&to=2026-09-22T00:00:00Z",
		"tên metric nhiệm vụ": "?metric=suspended",
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-reports"+q, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
		})
	}
}
