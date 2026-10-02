package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewPeriodRefusesInsteadOfRepairing(t *testing.T) {
	a := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	b := a.Add(7 * 24 * time.Hour)
	if p, err := NewPeriod(a, b); err != nil || !p.From.Equal(a) || !p.To.Equal(b) {
		t.Fatalf("kỳ hợp lệ bị từ chối: %v %v", p, err)
	}
	for name, c := range map[string][2]time.Time{
		"from bằng to": {a, a},
		"from sau to":  {b, a},
		"thiếu from":   {{}, b},
		"thiếu to":     {a, {}},
		"thiếu cả hai": {{}, {}},
	} {
		if _, err := NewPeriod(c[0], c[1]); !errors.Is(err, ErrPeriodInvalid) {
			t.Errorf("%s: err = %v, muốn ErrPeriodInvalid", name, err)
		}
	}
}

// TestMetricSetsAreClosedAndPeriodBoundIsDeclared — the drill-down refuses anything outside the set,
// and exactly the figures the overview counts inside a period are period-bound.
func TestMetricSetsAreClosedAndPeriodBoundIsDeclared(t *testing.T) {
	stockTask := map[TaskMetric]bool{TaskInProgress: true, TaskOverdue: true, TaskSuspended: true}
	if len(TaskMetrics) != 6 {
		t.Fatalf("%d chỉ số nhiệm vụ, muốn 6", len(TaskMetrics))
	}
	for _, m := range TaskMetrics {
		if !m.Valid() {
			t.Errorf("%s không hợp lệ", m)
		}
		if m.PeriodBound() == stockTask[m] {
			t.Errorf("%s: PeriodBound = %v", m, m.PeriodBound())
		}
	}
	if len(CitizenReportMetrics) != 8 {
		t.Fatalf("%d chỉ số phản ánh, muốn 8", len(CitizenReportMetrics))
	}
	// The four STOCK figures: in progress, and the three of 2026-10-02 (their lists take no period).
	stockReport := map[CitizenReportMetric]bool{CitizenReportInProgress: true, CitizenReportRatingSample: true,
		CitizenReportLowRating: true, CitizenReportPublicationPending: true}
	for _, m := range CitizenReportMetrics {
		if !m.Valid() {
			t.Errorf("%s không hợp lệ", m)
		}
		if m.PeriodBound() == stockReport[m] {
			t.Errorf("%s: PeriodBound = %v", m, m.PeriodBound())
		}
	}
	// "Low" and "reopens" are one number (ADR 0050 point 2, docs/ui-ux/09 §4 "phiếu 1–2 sao").
	if low, reopen := LowRatingMaxStars, RatingReopenThreshold; low != 2 || low != reopen {
		t.Errorf("LowRatingMaxStars = %d, RatingReopenThreshold = %d, muốn cùng là 2", low, reopen)
	}
	for _, bad := range []string{"", "late", "Overdue", "in-progress"} {
		if TaskMetric(bad).Valid() {
			t.Errorf("chỉ số nhiệm vụ %q lọt qua", bad)
		}
	}
	for _, bad := range []string{"", "overdue", "suspended", "completed"} {
		if CitizenReportMetric(bad).Valid() {
			t.Errorf("chỉ số phản ánh %q lọt qua", bad)
		}
	}
}

// TestIsCriticalBoundaryBelongsToCritical — at the instant itself an item is critical, so it never
// flickers across one tick between two reads.
func TestIsCriticalBoundaryBelongsToCritical(t *testing.T) {
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	if !IsCritical(at, at) || !IsCritical(at, at.Add(time.Second)) || IsCritical(at, at.Add(-time.Second)) {
		t.Error("ranh giới nghiêm trọng sai")
	}
}
