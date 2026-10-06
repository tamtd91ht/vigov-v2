package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The aggregate reads of the disbursement screen: GET /api/v1/investment-project-summary,
// GET /api/v1/investment-projects/{id}/disbursement-curve, and the two additions to the existing
// project reads (`delayed_only`, `time_elapsed_ratio`).
//
// The clock is lucDaQua7096 (2026-09-17, 70,96% of the year) — set by dungMayChuVoi.

const summaryPath = "/api/v1/investment-project-summary?year=2026"

// withMonths gives commune A's 2026 year and project da-001 month buckets, and commune B's colliding
// da-001 DIFFERENT ones — keyed by commune, so a curve read across communes shows.
func withMonths(d *duAnGia) {
	var year, a001, b001 domain.DisbursedByMonth
	year.Months[2] = 40_000_000 // March
	year.Months[8] = 50_000_000 // September — the current month, observed
	year.Months[9] = 1          // October: dated ahead; the month has not begun
	year.AfterYear = 7
	a001.Months[2] = 40_000_000
	a001.Months[8] = 50_000_000
	b001.Months[0] = 1_000_000
	d.byMonth = map[tenant.ID]map[string]map[int]domain.DisbursedByMonth{
		xaA: {"": {2026: year}, "da-001": {2026: a001}},
		xaB: {"da-001": {2026: b001}},
	}
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("body is not JSON: %v — %s", err, body)
	}
	return v
}

// --- rule 5, invariant 7: the four cases, on both new routes -----------------------------------

func TestAggregateRoutes_FourPermissionCases(t *testing.T) {
	for _, path := range []string{summaryPath, "/api/v1/investment-projects/da-001/disbursement-curve"} {
		t.Run(path, func(t *testing.T) {
			t.Run("401 no session", func(t *testing.T) {
				m := dungMayChuVoi(t, coQuyen("budget.read"))
				doiMa(t, m.goi(t, http.MethodGet, hostA, path, nil), http.StatusUnauthorized)
				if m.duAn.goi != 0 || m.duAn.monthReads != 0 {
					t.Fatal("store touched with no session")
				}
			})
			t.Run("403 wrong permission", func(t *testing.T) {
				m := dungMayChuVoi(t, coQuyen("budget.update"))
				w := m.goi(t, http.MethodGet, hostA, path, canBoCua(xaA))
				doiMa(t, w, http.StatusForbidden)
				if loiTra(t, w).Code != "forbidden" || m.duAn.goi != 0 || m.duAn.monthReads != 0 {
					t.Fatal("wrong permission must be refused before any read")
				}
			})
			t.Run("refused: right permission, wrong commune", func(t *testing.T) {
				// authz.RequirePermission answers 401 for a token of another commune (du_an_test.go
				// header states why); the case is exercised and refused before any read.
				m := dungMayChuVoi(t, coQuyen("budget.read"))
				doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCua(xaB)), http.StatusUnauthorized)
				if m.duAn.goi != 0 || m.duAn.monthReads != 0 {
					t.Fatal("another commune's token reached the store")
				}
			})
			t.Run("200 right permission, right commune", func(t *testing.T) {
				m := dungMayChuVoi(t, coQuyen("budget.read"))
				withMonths(m.duAn)
				doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCua(xaA)), http.StatusOK)
			})
		})
	}
}

// --- the summary's figures -----------------------------------------------------------------------

func TestProjectSummary_FiguresAreDerivedServerSide(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withMonths(m.duAn)
	w := m.goi(t, http.MethodGet, hostA, summaryPath, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	s := decode[projectSummaryOut](t, w.Body.Bytes())

	// Commune A, 2026: da-001 100tr/90tr and da-002 25 tỷ/0. The 2025 project is not here.
	if s.Year != 2026 || s.ProjectCount != 2 || s.PlannedTotal != 25_100_000_000 || s.DisbursedTotal != 90_000_000 {
		t.Fatalf("totals = %+v", s)
	}
	if s.DisbursedRatio == nil || *s.DisbursedRatio != 35 { // 0,3585…% toward zero
		t.Fatalf("disbursed_ratio = %v, want 35", s.DisbursedRatio)
	}
	if s.TimeElapsedRatio != 7096 {
		t.Fatalf("time_elapsed_ratio = %d, want 7096 (§3's worked example)", s.TimeElapsedRatio)
	}
	if s.RemainingTotal != 25_010_000_000 {
		t.Fatalf("remaining_total = %d", s.RemainingTotal)
	}
	// da-002 has disbursed 0% at 70,96% elapsed: behind by exactly the elapsed share (§3). da-001 is ahead.
	if s.DelayedProjectCount != 1 {
		t.Fatalf("delayed_project_count = %d, want 1", s.DelayedProjectCount)
	}
	if s.DelayThreshold != int64(domain.NguongCanhBaoChamMacDinh) || s.DelayThresholdSource != "mac-dinh" {
		t.Fatalf("threshold = %d/%q — the reply must say which threshold, and who chose it",
			s.DelayThreshold, s.DelayThresholdSource)
	}
	if s.ScopeNotice == "" {
		t.Fatal("scope notice missing")
	}

	// §4: twelve months, linear plan ending EXACTLY at the plan; actual stops after the current month.
	if len(s.Monthly) != 12 || s.Monthly[11].PlannedCumulative != 25_100_000_000 {
		t.Fatalf("monthly = %+v", s.Monthly)
	}
	if v := s.Monthly[2].DisbursedCumulative; v == nil || *v != 40_000_000 {
		t.Fatalf("March cumulative = %v", v)
	}
	if v := s.Monthly[8].DisbursedCumulative; v == nil || *v != 90_000_000 {
		t.Fatalf("September cumulative = %v, want 90000000", v)
	}
	if s.Monthly[9].DisbursedCumulative != nil || s.Monthly[11].DisbursedCumulative != nil {
		t.Fatal("months not yet begun must be null on the actual line, not a flat value")
	}
	if s.DisbursedAfterYear != 7 {
		t.Fatalf("disbursed_after_year = %d, want 7", s.DisbursedAfterYear)
	}
	// The JSON must carry null, not omit the key: a client cannot tell an absent key from a zero.
	if !strings.Contains(w.Body.String(), `"disbursed_cumulative":null`) {
		t.Fatal("future months must serialise as null")
	}

	// §5: catalogue order; hm-003 is out of use with no project, so it is not a row.
	if len(s.ByCategory) != 2 || s.ByCategory[0].CategoryID != "hm-001" || s.ByCategory[1].CategoryID != "hm-002" {
		t.Fatalf("by_category = %+v", s.ByCategory)
	}
	first := s.ByCategory[0]
	if first.Label != "Xây dựng mới" || first.ProjectCount != 1 || first.Planned != 100_000_000 ||
		first.Disbursed != 90_000_000 || first.Undisbursed != 10_000_000 ||
		first.DisbursedRatio == nil || *first.DisbursedRatio != 9000 ||
		first.UndisbursedRatio == nil || *first.UndisbursedRatio != 1000 ||
		first.DisbursementDeadline != "2026-12-31" || !first.InCatalogue {
		t.Fatalf("hm-001 row = %+v", first)
	}
	if s.Total.ProjectCount != 2 || s.Total.Planned != s.PlannedTotal || s.Total.CategoryID != "" {
		t.Fatalf("total row = %+v", s.Total)
	}
	if m.duAn.monthReads != 1 {
		t.Fatalf("month reads = %d — one read for the whole year, never one per project", m.duAn.monthReads)
	}
}

func TestProjectSummary_StaysInItsCommune(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withMonths(m.duAn)
	w := m.goi(t, http.MethodGet, hostB, summaryPath, canBoCua(xaB))
	doiMa(t, w, http.StatusOK)
	s := decode[projectSummaryOut](t, w.Body.Bytes())
	if s.ProjectCount != 1 || s.PlannedTotal != 9_000_000_000 {
		t.Fatalf("commune B summary = %+v — another commune's money is in it", s)
	}
	for _, c := range s.ByCategory {
		if c.CategoryID == "hm-001" || c.CategoryID == "hm-002" {
			t.Fatalf("commune A's category %q in commune B's table", c.CategoryID)
		}
	}
	// Commune B has no year bucket in the fixture: its curve is all zero, never A's.
	if v := s.Monthly[8].DisbursedCumulative; v == nil || *v != 0 {
		t.Fatalf("commune B September = %v, want 0", v)
	}
}

func TestProjectSummary_EmptyYearHasNullRatioNotZero(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-project-summary?year=2027", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	s := decode[projectSummaryOut](t, w.Body.Bytes())
	if s.DisbursedRatio != nil || s.ProjectCount != 0 || s.TimeElapsedRatio != 0 {
		t.Fatalf("empty future year = %+v — ratio must be null, elapsed 0", s)
	}
	if !strings.Contains(w.Body.String(), `"disbursed_ratio":null`) {
		t.Fatal("disbursed_ratio must serialise as null")
	}
	// In-use categories still appear with zeros (a block of the plan with nothing in it is a line).
	if len(s.ByCategory) != 2 || s.ByCategory[0].DisbursedRatio != nil {
		t.Fatalf("by_category = %+v", s.ByCategory)
	}
}

func TestProjectSummary_YearIsRequired(t *testing.T) {
	for _, path := range []string{"/api/v1/investment-project-summary",
		"/api/v1/investment-project-summary?year=abc", "/api/v1/investment-project-summary?year=1999"} {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCua(xaA)), http.StatusBadRequest)
		if m.duAn.goi != 0 || m.duAn.monthReads != 0 {
			t.Fatalf("%s: a rejected request must run no read", path)
		}
	}
}

func TestProjectSummary_FailuresAre500WithoutDetail(t *testing.T) {
	cases := map[string]func(m *mayChu){
		"projects":  func(m *mayChu) { m.duAn.loi = errors.New("pg: secret detail") },
		"ceiling":   func(m *mayChu) { m.duAn.loi = fistore.ErrQuaNhieuDuAn },
		"months":    func(m *mayChu) { m.duAn.monthErr = errors.New("pg: secret detail") },
		"catalogue": func(m *mayChu) { m.hangMuc.loi = errors.New("pg: secret detail") },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuVoi(t, coQuyen("budget.read"))
			breakIt(m)
			w := m.goi(t, http.MethodGet, hostA, summaryPath, canBoCua(xaA))
			doiMa(t, w, http.StatusInternalServerError)
			if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "du_an") {
				t.Fatalf("store detail leaked: %s", w.Body.String())
			}
		})
	}
}

// --- delayed_only ---------------------------------------------------------------------------------

func TestProjectList_DelayedOnlyUsesTheChipRule(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn+"&delayed_only=true", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	list := decode[danhSachDuAnRa](t, w.Body.Bytes())
	if len(list.Items) != 1 || list.Items[0].Code != "DA-2026-nong-thon-moi" || !list.Items[0].IsDelayed {
		t.Fatalf("delayed_only items = %+v", list.Items)
	}

	// The filtered list's length IS the summary card's number — one rule, three places.
	ws := m.goi(t, http.MethodGet, hostA, summaryPath, canBoCua(xaA))
	doiMa(t, ws, http.StatusOK)
	if s := decode[projectSummaryOut](t, ws.Body.Bytes()); s.DelayedProjectCount != len(list.Items) {
		t.Fatalf("summary says %d delayed, filtered list has %d", s.DelayedProjectCount, len(list.Items))
	}

	wf := m.goi(t, http.MethodGet, hostA, duongDanDuAn+"&delayed_only=false", canBoCua(xaA))
	doiMa(t, wf, http.StatusOK)
	if all := decode[danhSachDuAnRa](t, wf.Body.Bytes()); len(all.Items) != 2 {
		t.Fatalf("delayed_only=false must not filter: %d items", len(all.Items))
	}
}

func TestProjectList_DelayedOnlyRejectsOtherValues(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongDanDuAn+"&delayed_only=yes", canBoCua(xaA)), http.StatusBadRequest)
	if m.duAn.goi != 0 {
		t.Fatal("a rejected filter must run no read")
	}
}

// --- time_elapsed_ratio on the detail -------------------------------------------------------------

func TestProjectDetail_TimeElapsedRatioAgreesWithDelayScore(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	p := decode[duAnRa](t, w.Body.Bytes())
	if p.TimeElapsedRatio == nil || *p.TimeElapsedRatio != 7096 {
		t.Fatalf("time_elapsed_ratio = %v, want 7096", p.TimeElapsedRatio)
	}
	if *p.DelayScore != *p.TimeElapsedRatio-*p.DisbursedRatio {
		t.Fatalf("marker %d, ratio %d, score %d — the bar would contradict the chip",
			*p.TimeElapsedRatio, *p.DisbursedRatio, *p.DelayScore)
	}

	// A closed year is 100% elapsed — a carried-over project is not quietly on schedule.
	w = m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-2025", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if old := decode[duAnRa](t, w.Body.Bytes()); old.TimeElapsedRatio == nil || *old.TimeElapsedRatio != 10000 {
		t.Fatalf("2025 project elapsed = %v, want 10000", old.TimeElapsedRatio)
	}

	// The list does not carry it (detail only).
	wl := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	if strings.Contains(wl.Body.String(), "time_elapsed_ratio") {
		t.Fatal("time_elapsed_ratio is a detail-route field")
	}
}

// --- the project curve ----------------------------------------------------------------------------

func TestProjectCurve_UndatedProjectRunsTheWholeYear(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withMonths(m.duAn)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/disbursement-curve", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	c := decode[projectCurveOut](t, w.Body.Bytes())
	if c.ProjectID != "da-001" || c.Year != 2026 || len(c.Points) != 12 || c.ExpectedEndMonth != nil {
		t.Fatalf("curve = %+v", c)
	}
	if c.Points[11].PlannedCumulative != 100_000_000 {
		t.Fatalf("December plan = %d, want exactly the plan", c.Points[11].PlannedCumulative)
	}
	if v := c.Points[8].DisbursedCumulative; v == nil || *v != 90_000_000 {
		t.Fatalf("September cumulative = %v", v)
	}
	if c.Points[9].DisbursedCumulative != nil {
		t.Fatal("October has not begun — null")
	}
}

func TestProjectCurve_FollowsTheProjectsOwnCalendar(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withMonths(m.duAn)
	p := &m.duAn.theo[xaA][0].DuAn
	p.NgayKhoiCong = time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC)
	p.NgayHoanThanh = time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/disbursement-curve", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	c := decode[projectCurveOut](t, w.Body.Bytes())
	if len(c.Points) != 10 || c.Points[0].Month != 3 {
		t.Fatalf("points = %+v, want March..December", c.Points)
	}
	if c.ExpectedEndMonth == nil || *c.ExpectedEndMonth != 6 {
		t.Fatalf("expected_end_month = %v, want 6", c.ExpectedEndMonth)
	}
	// March..June: 25, 50, 75, 100 triệu; flat after.
	if c.Points[0].PlannedCumulative != 25_000_000 || c.Points[3].PlannedCumulative != 100_000_000 ||
		c.Points[9].PlannedCumulative != 100_000_000 {
		t.Fatalf("plan line = %+v", c.Points)
	}
}

func TestProjectCurve_CollidingIDStaysInItsCommune(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withMonths(m.duAn)
	w := m.goi(t, http.MethodGet, hostB, "/api/v1/investment-projects/da-001/disbursement-curve", canBoCua(xaB))
	doiMa(t, w, http.StatusOK)
	c := decode[projectCurveOut](t, w.Body.Bytes())
	if c.Points[11].PlannedCumulative != 9_000_000_000 {
		t.Fatalf("commune B got another project's plan: %+v", c.Points[11])
	}
	if v := c.Points[8].DisbursedCumulative; v == nil || *v != 1_000_000 {
		t.Fatalf("commune B September = %v, want its own 1000000", v)
	}
}

func TestProjectCurve_UnknownProjectIs404AndRunsNoMonthRead(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-none/disbursement-curve",
		canBoCua(xaA)), http.StatusNotFound)
	if m.duAn.monthReads != 0 {
		t.Fatal("month read ran for a project not in this commune")
	}
}

func TestProjectCurve_MonthFailureIs500WithoutDetail(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	m.duAn.monthErr = errors.New("pg: secret detail")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/disbursement-curve", canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("store detail leaked")
	}
}
