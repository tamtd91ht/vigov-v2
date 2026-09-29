package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// These run without a database. They pin the SHAPE of the SQL and the order of its arguments — the
// part a placeholder off by one breaks silently. What the SQL COUNTS is proved against PostgreSQL in
// incoming_dashboard_pg_test.go, which skips without VIGOV_TEST_DSN.

func TestMetricPredicateForEachFigure(t *testing.T) {
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	window := domain.ArrivalWindow{
		FirstDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		LastDate:  time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
	}
	finished := []any{"da-giai-quyet", "chuyen-cap-tren", "luu-khong-thu-ly"}

	for name, tc := range map[string]struct {
		f        IncomingMetricFilter
		wantSQL  string
		wantArgs []any
	}{
		"arrived": {
			f:        IncomingMetricFilter{Metric: domain.MetricArrived, Window: window},
			wantSQL:  "ngay_den BETWEEN $2::date AND $3::date",
			wantArgs: []any{"2026-09-01", "2026-09-30"},
		},
		"open": {
			f:        IncomingMetricFilter{Metric: domain.MetricOpen},
			wantSQL:  "trang_thai NOT IN ($2, $3, $4)",
			wantArgs: finished,
		},
		"overdue": {
			f:        IncomingMetricFilter{Metric: domain.MetricOverdue, Now: now},
			wantSQL:  "trang_thai NOT IN ($2, $3, $4) AND han_xu_ly_xong < $5",
			wantArgs: append(append([]any{}, finished...), now),
		},
	} {
		t.Run(name, func(t *testing.T) {
			var args []any
			got, err := metricPredicate(tc.f, newBinder(&args))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantSQL {
				t.Errorf("sql = %q, want %q", got, tc.wantSQL)
			}
			if len(args) != len(tc.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tc.wantArgs)
			}
			for i := range args {
				if args[i] != tc.wantArgs[i] {
					t.Errorf("arg %d = %v, want %v", i, args[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestMetricPredicateRefusesWhatWouldSilentlyCountNothing(t *testing.T) {
	for name, f := range map[string]IncomingMetricFilter{
		"arrived without a window":   {Metric: domain.MetricArrived},
		"overdue without an instant": {Metric: domain.MetricOverdue},
		"unknown metric":             {Metric: "tat-ca"},
	} {
		t.Run(name, func(t *testing.T) {
			var args []any
			if _, err := metricPredicate(f, newBinder(&args)); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	var args []any
	if _, err := metricPredicate(IncomingMetricFilter{Metric: "x"}, newBinder(&args)); !errors.Is(err, ErrUnknownIncomingMetric) {
		t.Errorf("err = %v, want ErrUnknownIncomingMetric", err)
	}
}

func TestListFilterAppendsTheFigurePredicateAfterTheExistingFilters(t *testing.T) {
	// The drill-down combines with the list's own filters; its placeholders must continue the
	// numbering rather than restart it, or `year` would be compared with a status code.
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	sql, args, err := incomingFilterSQL(IncomingDocumentFilter{
		Year:   2026,
		Metric: IncomingMetricFilter{Metric: domain.MetricOverdue, Now: now},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := " AND nam = $2 AND (trang_thai NOT IN ($3, $4, $5) AND han_xu_ly_xong < $6)"
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 5 || args[0] != 2026 || args[4] != now {
		t.Fatalf("args = %v", args)
	}
}

func TestListFilterWithoutMetricIsUnchanged(t *testing.T) {
	sql, args, err := incomingFilterSQL(IncomingDocumentFilter{Status: "dang-xu-ly"})
	if err != nil {
		t.Fatal(err)
	}
	if sql != " AND trang_thai = $2" || len(args) != 1 {
		t.Fatalf("sql = %q args = %v", sql, args)
	}
	if strings.Contains(sql, "NOT IN") {
		t.Fatal("a figure predicate leaked into a list that asked for none")
	}
}

func TestOverdueIncomingRefusesALimitOutsideTheBlock(t *testing.T) {
	s := NewIncomingDocumentStore(nil)
	for _, n := range []int{0, -1, domain.OverdueQueueMax + 1} {
		if _, err := s.OverdueIncoming(tenantCtx("01JTESTLIMITXAXAXAXAXAXAXA"), time.Now(), n); err == nil {
			t.Errorf("limit %d accepted", n)
		}
	}
}
