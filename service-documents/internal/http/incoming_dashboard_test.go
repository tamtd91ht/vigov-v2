package http

// The dashboard block of the incoming register: GET /api/v1/incoming-document-summary,
// GET /api/v1/incoming-document-overdue-queue, and the `metric` drill-down on the list route.
//
// THE TWO NEW ROUTES REQUIRE TWO KEYS, so rule 5 invariant 7's four cases grow to six here: holding
// only `report.read` and holding only `document.read` are each a 403 of their own. A single-key test
// would pass with either guard deleted.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

const (
	pathSummary = "/api/v1/incoming-document-summary"
	pathQueue   = "/api/v1/incoming-document-overdue-queue"

	permReport authz.Perm = "report.read"

	// September 2026 in Vietnam, written with its offset: 1/9 00:00 ICT .. 1/10 00:00 ICT.
	septemberQuery = "?from=2026-09-01T00:00:00%2B07:00&to=2026-10-01T00:00:00%2B07:00"
)

var testNow = time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)

// fakeSummaryReader answers PER COMMUNE, reading the commune from the context as *store.Scoped does.
type fakeSummaryReader struct {
	byTenant  map[tenant.ID]domain.IncomingSummary
	err       error
	calls     int
	gotWindow domain.ArrivalWindow
	gotNow    time.Time
}

func (f *fakeSummaryReader) CountIncomingSummary(ctx context.Context, w domain.ArrivalWindow,
	now time.Time) (domain.IncomingSummary, error) {
	f.calls++
	f.gotWindow, f.gotNow = w, now
	if f.err != nil {
		return domain.IncomingSummary{}, f.err
	}
	return f.byTenant[tenant.MustFrom(ctx)], nil
}

func sampleSummaryReader() *fakeSummaryReader {
	return &fakeSummaryReader{byTenant: map[tenant.ID]domain.IncomingSummary{
		xaA: {Arrived: 7, Open: 5, Overdue: 2},
		xaB: {Arrived: 99, Open: 98, Overdue: 97},
	}}
}

type fakeQueueReader struct {
	byTenant map[tenant.ID][]app.OverdueQueueItem
	err      error
	calls    int
	gotLimit int
	gotNow   time.Time
}

func (f *fakeQueueReader) OverdueQueue(ctx context.Context, now time.Time, limit int) ([]app.OverdueQueueItem, error) {
	f.calls++
	f.gotLimit, f.gotNow = limit, now
	if f.err != nil {
		return nil, f.err
	}
	return f.byTenant[tenant.MustFrom(ctx)], nil
}

func sampleQueueReader() *fakeQueueReader {
	return &fakeQueueReader{byTenant: map[tenant.ID][]app.OverdueQueueItem{
		xaA: {
			{Document: domain.VanBanDen{ID: "vbd-a-004", SoVaoSo: 4, Nam: 2026, BoPhanDangGiu: "bp-dia-chinh",
				TrichYeu: "Đơn của ông Nguyễn Văn A về tranh chấp đất", HanXuLyXong: time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC),
				TrangThai: domain.VanBanDangXuLy}, Critical: true},
			{Document: domain.VanBanDen{ID: "vbd-a-009", SoVaoSo: 9, Nam: 2026,
				TrichYeu: "Về việc rà soát hộ nghèo", HanXuLyXong: time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC),
				TrangThai: domain.VanBanMoiVaoSo}},
		},
		xaB: {{Document: domain.VanBanDen{ID: "vbd-b-001", SoVaoSo: 1, Nam: 2026}, Critical: true}},
	}}
}

type dashboardRoute struct {
	name, path string
	calls      func(m *mayChu) int
}

func dashboardRoutes() []dashboardRoute {
	return []dashboardRoute{
		{"summary", pathSummary + septemberQuery, func(m *mayChu) int { return m.summary.calls }},
		{"queue", pathQueue, func(m *mayChu) int { return m.queue.calls }},
	}
}

func TestDashboard_401WithoutSession(t *testing.T) {
	for _, rt := range dashboardRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, permReport, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostA, rt.path, nil), http.StatusUnauthorized)
			if rt.calls(m) != 0 {
				t.Error("register read with no session")
			}
		})
	}
}

func TestDashboard_403WrongOrIncompletePermission(t *testing.T) {
	for _, rt := range dashboardRoutes() {
		for grantName, grant := range map[string][]authz.Perm{
			"unrelated key":       {"admin.lookup"},
			"report.read only":    {permReport},
			"document.read only":  {QuyenDocVanBan},
			"document.create etc": {"document.create", "document.route"},
		} {
			t.Run(rt.name+"/"+grantName, func(t *testing.T) {
				m := dungMayChu(t)
				m.capQuyen(xaA, grant...)
				doiMa(t, m.goi(t, http.MethodGet, hostA, rt.path, canBoCua(xaA)), http.StatusForbidden)
				if rt.calls(m) != 0 {
					t.Error("figures read without both keys")
				}
			})
		}
	}
}

func TestDashboard_403RightPermissionsWrongCommune(t *testing.T) {
	// Both keys granted in commune A; the account is commune B's, signed in at commune B. A checker
	// that ignored the commune would hand B's staff A's grant.
	for _, rt := range dashboardRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, permReport, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostB, rt.path, canBoCua(xaB)), http.StatusForbidden)
			if rt.calls(m) != 0 {
				t.Error("commune B read with commune A's grant")
			}
		})
	}
}

func TestDashboard_401SessionOfAnotherCommune(t *testing.T) {
	for _, rt := range dashboardRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, permReport, QuyenDocVanBan)
			m.capQuyen(xaB, permReport, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostB, rt.path, canBoCua(xaA)), http.StatusUnauthorized)
			if rt.calls(m) != 0 {
				t.Error("a commune A session read commune B")
			}
		})
	}
}

func TestDashboard_200BothKeysRightCommune(t *testing.T) {
	for _, rt := range dashboardRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, permReport, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostA, rt.path, canBoCua(xaA)), http.StatusOK)
			if rt.calls(m) != 1 {
				t.Fatalf("reader called %d times, want 1", rt.calls(m))
			}
			// BOTH KEYS WERE ASKED, as literals — a fake checker grants any string, so this is the
			// assertion that turns red if either guard is deleted or renamed.
			asked := map[authz.Perm]bool{}
			for _, p := range m.checker.hoiGi {
				asked[p] = true
			}
			if !asked["report.read"] || !asked["document.read"] {
				t.Fatalf("keys asked = %v, want both report.read and document.read", m.checker.hoiGi)
			}
		})
	}
}

func TestSummary_ReturnsThisCommunesCountsForTheLocalDatesOfThePeriod(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, permReport, QuyenDocVanBan)
	w := m.goi(t, http.MethodGet, hostA, pathSummary+septemberQuery, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	var got incomingSummaryOut
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := incomingSummaryOut{
		From: "2026-08-31T17:00:00Z", To: "2026-09-30T17:00:00Z", AsOf: "2026-09-28T03:00:00Z",
		Arrived: 7, Open: 5, Overdue: 2,
	}
	if got != want {
		t.Fatalf("body = %+v, want %+v", got, want)
	}
	if f, l := m.summary.gotWindow.FirstDate.Format(time.DateOnly), m.summary.gotWindow.LastDate.Format(time.DateOnly); f != "2026-09-01" || l != "2026-09-30" {
		t.Errorf("window = %s..%s, want 2026-09-01..2026-09-30 (dates in Asia/Ho_Chi_Minh)", f, l)
	}
	if !m.summary.gotNow.Equal(testNow) {
		t.Errorf("now = %s, want the handler clock", m.summary.gotNow)
	}
}

func TestSummary_RefusesAMissingOrBackwardPeriod(t *testing.T) {
	for name, q := range map[string]string{
		"no period":     "",
		"no to":         "?from=2026-09-01T00:00:00Z",
		"from == to":    "?from=2026-09-01T00:00:00Z&to=2026-09-01T00:00:00Z",
		"from > to":     "?from=2026-10-01T00:00:00Z&to=2026-09-01T00:00:00Z",
		"a date only":   "?from=2026-09-01&to=2026-10-01",
		"not a instant": "?from=hom-qua&to=hom-nay",
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, permReport, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostA, pathSummary+q, canBoCua(xaA)), http.StatusBadRequest)
			if m.summary.calls != 0 {
				t.Error("counted with a period that was refused")
			}
		})
	}
}

func TestSummary_StoreFailureIs500(t *testing.T) {
	m := dungMayChu(t)
	m.summary.err = errors.New("db down")
	m.capQuyen(xaA, permReport, QuyenDocVanBan)
	doiMa(t, m.goi(t, http.MethodGet, hostA, pathSummary+septemberQuery, canBoCua(xaA)), http.StatusInternalServerError)
}

func TestQueue_ShowsTheRegisterCodeNotTheSummary(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, permReport, QuyenDocVanBan)
	w := m.goi(t, http.MethodGet, hostA, pathQueue, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	if strings.Contains(w.Body.String(), "Nguyễn Văn A") || strings.Contains(w.Body.String(), "hộ nghèo") {
		t.Fatalf("free-text summary reached the dashboard queue: %s", w.Body.String())
	}
	var got overdueQueueOut
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []overdueQueueItemOut{
		{Kind: "van-ban-den", ID: "vbd-a-004", Code: "VB-DEN-2026-0004", DueAt: "2026-09-20T02:00:00Z",
			Critical: true, HoldingUnit: "bp-dia-chinh"},
		{Kind: "van-ban-den", ID: "vbd-a-009", Code: "VB-DEN-2026-0009", DueAt: "2026-09-28T01:00:00Z"},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("items = %+v", got.Items)
	}
	for i := range want {
		if got.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, got.Items[i], want[i])
		}
	}
	if got.AsOf != "2026-09-28T03:00:00Z" || !m.queue.gotNow.Equal(testNow) {
		t.Errorf("as_of = %s, now = %s", got.AsOf, m.queue.gotNow)
	}
}

func TestQueue_EmptyIsAnEmptyArray(t *testing.T) {
	m := dungMayChu(t)
	m.queue.byTenant = nil
	m.capQuyen(xaA, permReport, QuyenDocVanBan)
	w := m.goi(t, http.MethodGet, hostA, pathQueue, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("body = %s, want items: []", w.Body.String())
	}
}

func TestQueue_Limit(t *testing.T) {
	for q, wantLimit := range map[string]int{"": 10, "?limit=3": 3, "?limit=10": 10, "?limit=500": 10} {
		m := dungMayChu(t)
		m.capQuyen(xaA, permReport, QuyenDocVanBan)
		doiMa(t, m.goi(t, http.MethodGet, hostA, pathQueue+q, canBoCua(xaA)), http.StatusOK)
		if m.queue.gotLimit != wantLimit {
			t.Errorf("%q: limit = %d, want %d", q, m.queue.gotLimit, wantLimit)
		}
	}
	for _, q := range []string{"?limit=0", "?limit=-1", "?limit=muoi"} {
		m := dungMayChu(t)
		m.capQuyen(xaA, permReport, QuyenDocVanBan)
		doiMa(t, m.goi(t, http.MethodGet, hostA, pathQueue+q, canBoCua(xaA)), http.StatusBadRequest)
		if m.queue.calls != 0 {
			t.Errorf("%q: queue read with a refused limit", q)
		}
	}
}

func TestQueue_IdentityUnreachableFailsTheRequest(t *testing.T) {
	m := dungMayChu(t)
	m.queue.err = errors.Join(app.ErrWorkingCalendarUnavailable, errors.New("rpc error: code = Unavailable"))
	m.capQuyen(xaA, permReport, QuyenDocVanBan)
	w := m.goi(t, http.MethodGet, hostA, pathQueue, canBoCua(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if e := loiTra(t, w); e.Code != "working_calendar_unavailable" {
		t.Errorf("code = %q", e.Code)
	}
}

// --- the drill-down on GET /api/v1/incoming-documents ------------------------------------------

func TestDrillDown_EachMetricReachesTheStoreAsItsFigure(t *testing.T) {
	for q, check := range map[string]func(t *testing.T, m *mayChu){
		"?metric=arrived&from=2026-09-01T00:00:00%2B07:00&to=2026-10-01T00:00:00%2B07:00": func(t *testing.T, m *mayChu) {
			f := m.den.locCuo.Metric
			if f.Metric != domain.MetricArrived || f.Window.FirstDate.Format(time.DateOnly) != "2026-09-01" ||
				f.Window.LastDate.Format(time.DateOnly) != "2026-09-30" || !f.Now.IsZero() {
				t.Fatalf("filter = %+v", f)
			}
		},
		"?metric=open": func(t *testing.T, m *mayChu) {
			f := m.den.locCuo.Metric
			if f.Metric != domain.MetricOpen || !f.Window.FirstDate.IsZero() || !f.Now.IsZero() {
				t.Fatalf("filter = %+v", f)
			}
		},
		"?metric=overdue&year=2026": func(t *testing.T, m *mayChu) {
			f := m.den.locCuo.Metric
			if f.Metric != domain.MetricOverdue || !f.Now.Equal(testNow) {
				t.Fatalf("filter = %+v", f)
			}
			if m.den.locCuo.Nam != 2026 {
				t.Fatal("the existing `year` filter was dropped beside the metric")
			}
		},
	} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duongVanBanDen+q, canBoCua(xaA)), http.StatusOK)
			check(t, m)
		})
	}
}

func TestDrillDown_RefusesRatherThanIgnores(t *testing.T) {
	for name, q := range map[string]string{
		"unknown metric":               "?metric=tat-ca",
		"period without metric":        "?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z",
		"arrived without period":       "?metric=arrived",
		"arrived with backward period": "?metric=arrived&from=2026-10-01T00:00:00Z&to=2026-09-01T00:00:00Z",
		"open with a period":           "?metric=open&from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z",
		"overdue with a period":        "?metric=overdue&from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z",
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, QuyenDocVanBan)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duongVanBanDen+q, canBoCua(xaA)), http.StatusBadRequest)
			if m.den.goi != 0 {
				t.Error("list read with a refused drill-down")
			}
		})
	}
}
