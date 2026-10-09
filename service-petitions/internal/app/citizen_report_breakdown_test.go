package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// CitizenReportBreakdown — the unit section and its working-hours measurement.
//
//	PROVED HERE   petitions are grouped by the unit holding each, the handled ones summed with
//	              IDENTITY's answer (never a subtraction here) · a petition with no recorded hand-over is
//	              counted as finished and left out of the sample · spans go to identity in chunks of at
//	              most MaxMeasuredSpansPerCall · identity down, the calendar missing, or a short answer
//	              REFUSES the whole read — never a partial table, never a clock fallback · the restricted
//	              fact reaches both store reads · no handled petition asks identity nothing.
//	NOT PROVED    the SQL (internal/store).

type breakdownStoreFake struct {
	counts      domain.CitizenReportBreakdown
	handled     []domain.CitizenReportHandling
	restricted  []bool
	countsErr   error
	handlingErr error
}

func (f *breakdownStoreFake) CitizenReportBreakdown(_ context.Context, _ domain.Period, restricted bool) (
	domain.CitizenReportBreakdown, error) {
	f.restricted = append(f.restricted, restricted)
	return f.counts, f.countsErr
}

func (f *breakdownStoreFake) CitizenReportHandling(_ context.Context, _ domain.Period, restricted bool) (
	[]domain.CitizenReportHandling, error) {
	f.restricted = append(f.restricted, restricted)
	return f.handled, f.handlingErr
}

// measurerFake answers each span with the seconds in `answers`, keyed by the span's START (UTC) — fixed
// numbers, so the test never computes a duration from two instants.
type measurerFake struct {
	answers map[time.Time]uint64
	err     error
	calls   []int
	drop    bool // answer one span short
}

func (m *measurerFake) MeasureWorkingSeconds(_ context.Context, spans []identityclient.WorkingSpan) (
	map[identityclient.WorkingSpan]uint64, error) {
	m.calls = append(m.calls, len(spans))
	if m.err != nil {
		return nil, m.err
	}
	out := map[identityclient.WorkingSpan]uint64{}
	for i, s := range spans {
		if m.drop && i == 0 {
			continue
		}
		out[s] = m.answers[s.Start]
	}
	return out, nil
}

var (
	brPeriod = domain.Period{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	brFinish = time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	brA1     = time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	brA2     = time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	brB1     = time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
)

func brHandled() []domain.CitizenReportHandling {
	return []domain.CitizenReportHandling{
		{OrgUnitID: "bp-a", AssignedAt: brA1, FinishedAt: brFinish},
		{OrgUnitID: "bp-b", AssignedAt: brB1, FinishedAt: brFinish},
		{OrgUnitID: "bp-a", AssignedAt: brA2, FinishedAt: brFinish},
		{OrgUnitID: "", FinishedAt: brFinish}, // no recorded hand-over
	}
}

func TestBreakdownSumsIdentityAnswersPerUnit(t *testing.T) {
	st := &breakdownStoreFake{counts: domain.CitizenReportBreakdown{AsOf: brFinish,
		Fields: []domain.CitizenReportFieldFigures{{FieldCode: "rac-thai", Received: 3}}}, handled: brHandled()}
	hours := &measurerFake{answers: map[time.Time]uint64{brA1: 32400, brA2: 3600, brB1: 7200}}
	got, err := NewCitizenReportBreakdown(st, hours).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := []domain.CitizenReportUnitFigures{
		{OrgUnitID: "bp-a", Finished: 2, HandlingSample: 2, HandlingWorkingSeconds: 36000},
		{OrgUnitID: "bp-b", Finished: 1, HandlingSample: 1, HandlingWorkingSeconds: 7200},
		{OrgUnitID: "", Finished: 1, HandlingSample: 0, HandlingWorkingSeconds: 0},
	}
	if len(got.Units) != len(want) {
		t.Fatalf("units = %+v", got.Units)
	}
	for i := range want {
		if got.Units[i] != want[i] {
			t.Errorf("unit %d = %+v, want %+v", i, got.Units[i], want[i])
		}
	}
	if len(got.Fields) != 1 || !got.AsOf.Equal(brFinish) {
		t.Errorf("the store's counts were not passed through: %+v", got)
	}
	if len(hours.calls) != 1 || hours.calls[0] != 3 {
		t.Errorf("identity calls = %v, want ONE call with the three handled spans", hours.calls)
	}
	if len(st.restricted) != 2 || st.restricted[0] || st.restricted[1] {
		t.Errorf("restricted fact = %v, want false to both reads", st.restricted)
	}
}

func TestBreakdownChunksSpansForIdentity(t *testing.T) {
	n := 2*identityclient.MaxMeasuredSpansPerCall + 1
	handled := make([]domain.CitizenReportHandling, 0, n)
	answers := map[time.Time]uint64{}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		start := base.Add(time.Duration(i) * time.Second)
		answers[start] = 1
		handled = append(handled, domain.CitizenReportHandling{OrgUnitID: "bp-a", AssignedAt: start, FinishedAt: brFinish})
	}
	hours := &measurerFake{answers: answers}
	got, err := NewCitizenReportBreakdown(&breakdownStoreFake{handled: handled}, hours).Read(ctxXa(xaThu), brPeriod, coQuyenHanChe)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if fmt.Sprint(hours.calls) != fmt.Sprint([]int{500, 500, 1}) {
		t.Errorf("calls = %v, want chunks of at most %d", hours.calls, identityclient.MaxMeasuredSpansPerCall)
	}
	if len(got.Units) != 1 || got.Units[0].HandlingSample != n || got.Units[0].HandlingWorkingSeconds != uint64(n) {
		t.Errorf("units = %+v", got.Units)
	}
}

func TestBreakdownFailsClosed(t *testing.T) {
	for name, c := range map[string]struct {
		hours WorkingTimeMeasurer
		want  error
	}{
		"identity down":           {&measurerFake{err: errors.Join(identityclient.ErrIdentityUnavailable, errors.New("rpc"))}, ErrWorkingHoursUnavailable},
		"calendar not configured": {&measurerFake{err: fmt.Errorf("x: %w", identityclient.ErrWorkingCalendarNotConfigured)}, ErrWorkingCalendarMissing},
		"short answer":            {&measurerFake{drop: true, answers: map[time.Time]uint64{brA1: 1, brA2: 1, brB1: 1}}, ErrWorkingHoursUnavailable},
		"not wired":               {nil, ErrWorkingHoursUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := NewCitizenReportBreakdown(&breakdownStoreFake{handled: brHandled()}, c.hours).
				Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if got.Units != nil || got.Fields != nil {
				t.Errorf("a refused read returned figures: %+v", got)
			}
		})
	}
}

func TestBreakdownWithNothingHandledAsksIdentityNothing(t *testing.T) {
	st := &breakdownStoreFake{handled: []domain.CitizenReportHandling{{OrgUnitID: "", FinishedAt: brFinish}}}
	got, err := NewCitizenReportBreakdown(st, nil).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got.Units) != 1 || got.Units[0].Finished != 1 || got.Units[0].HandlingSample != 0 {
		t.Errorf("units = %+v", got.Units)
	}
	empty, err := NewCitizenReportBreakdown(&breakdownStoreFake{}, nil).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe)
	if err != nil || empty.Units == nil || len(empty.Units) != 0 {
		t.Errorf("no finished petition: %+v %v — units must be [] not nil", empty.Units, err)
	}
}

func TestBreakdownRefusesAHandOverAfterTheFinish(t *testing.T) {
	st := &breakdownStoreFake{handled: []domain.CitizenReportHandling{{OrgUnitID: "bp-a", AssignedAt: brFinish.Add(time.Second), FinishedAt: brFinish}}}
	if _, err := NewCitizenReportBreakdown(st, &measurerFake{}).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe); err == nil {
		t.Error("a hand-over after the finishing instant was measured")
	}
}

func TestBreakdownStoreErrorsAreWrapped(t *testing.T) {
	cause := errors.New("pg down")
	for _, st := range []*breakdownStoreFake{{countsErr: cause}, {handlingErr: cause}} {
		if _, err := NewCitizenReportBreakdown(st, &measurerFake{}).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe); !errors.Is(err, cause) {
			t.Errorf("err = %v, want wrapped %v", err, cause)
		}
	}
}
