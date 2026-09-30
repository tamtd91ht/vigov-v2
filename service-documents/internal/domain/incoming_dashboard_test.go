package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFinishedIncomingStatusesAgreesWithTheDomainRule(t *testing.T) {
	// The SQL `open` predicate is built from FinishedIncomingStatuses. If it ever disagreed with
	// DaKetThuc, the dashboard figure and QuaHan would count different documents while each looked
	// correct alone.
	finished := map[TrangThaiVanBanDen]bool{}
	for _, s := range FinishedIncomingStatuses() {
		finished[s] = true
	}
	for _, s := range IncomingStatuses() {
		if s.DaKetThuc() != finished[s] {
			t.Errorf("%s: DaKetThuc=%v but FinishedIncomingStatuses says %v", s, s.DaKetThuc(), finished[s])
		}
	}
	if len(FinishedIncomingStatuses()) != 3 {
		t.Fatalf("want the three closing codes of §3.2, got %v", FinishedIncomingStatuses())
	}
}

func TestParseIncomingMetric(t *testing.T) {
	for _, s := range []string{"arrived", "open", "overdue"} {
		if m, ok := ParseIncomingMetric(s); !ok || string(m) != s {
			t.Errorf("%q refused", s)
		}
	}
	for _, s := range []string{"", "Overdue", "qua-han", "all"} {
		if _, ok := ParseIncomingMetric(s); ok {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestArrivalWindowFor(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	for name, tc := range map[string]struct {
		from, to    time.Time
		first, last time.Time
	}{
		// A month given as local midnights: 1/9 00:00 ICT is 31/8 17:00 UTC. A UTC reading would
		// start the period on 31/8 and put a day of August into September's figure.
		"a month of local midnights": {
			from: time.Date(2026, 9, 1, 0, 0, 0, 0, ict), to: time.Date(2026, 10, 1, 0, 0, 0, 0, ict),
			first: date(2026, 9, 1), last: date(2026, 9, 30),
		},
		"same month, sent in UTC": {
			from: time.Date(2026, 8, 31, 17, 0, 0, 0, time.UTC), to: time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC),
			first: date(2026, 9, 1), last: date(2026, 9, 30),
		},
		"one local day": {
			from: time.Date(2026, 9, 28, 0, 0, 0, 0, ict), to: time.Date(2026, 9, 29, 0, 0, 0, 0, ict),
			first: date(2026, 9, 28), last: date(2026, 9, 28),
		},
		"ending mid-morning includes that day": {
			from: time.Date(2026, 9, 28, 0, 0, 0, 0, ict), to: time.Date(2026, 9, 29, 10, 0, 0, 0, ict),
			first: date(2026, 9, 28), last: date(2026, 9, 29),
		},
		"one second, late evening UTC is the next local day": {
			from: time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC), to: time.Date(2026, 9, 28, 18, 0, 1, 0, time.UTC),
			first: date(2026, 9, 29), last: date(2026, 9, 29),
		},
		"to at local midnight on the first of a month": {
			from: time.Date(2026, 2, 1, 0, 0, 0, 0, ict), to: time.Date(2026, 3, 1, 0, 0, 0, 0, ict),
			first: date(2026, 2, 1), last: date(2026, 2, 28),
		},
	} {
		t.Run(name, func(t *testing.T) {
			w, err := ArrivalWindowFor(tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if !w.FirstDate.Equal(tc.first) || !w.LastDate.Equal(tc.last) {
				t.Fatalf("window = %s..%s, want %s..%s", w.FirstDate.Format(time.DateOnly),
					w.LastDate.Format(time.DateOnly), tc.first.Format(time.DateOnly), tc.last.Format(time.DateOnly))
			}
		})
	}
}

func TestArrivalWindowForRefusesEmptyAndBackwardPeriods(t *testing.T) {
	a := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if _, err := ArrivalWindowFor(a, a); !errors.Is(err, ErrPeriodInverted) {
		t.Errorf("from == to: err = %v, want ErrPeriodInverted", err)
	}
	if _, err := ArrivalWindowFor(b, a); !errors.Is(err, ErrPeriodInverted) {
		t.Errorf("from > to: err = %v, want ErrPeriodInverted", err)
	}
	if _, err := ArrivalWindowFor(time.Time{}, b); !errors.Is(err, ErrPeriodMissing) {
		t.Errorf("zero from: err = %v, want ErrPeriodMissing", err)
	}
}
