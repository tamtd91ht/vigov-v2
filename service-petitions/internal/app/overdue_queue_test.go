package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The overdue queue use case — overdue_queue.go. What is proved: critical comes from identity's
// working-hours answer and never from wall-clock arithmetic; identity is asked once per DISTINCT
// deadline; any identity failure fails the whole request; the restricted fact reaches the store.

type overdueStoreFake struct {
	items      []domain.OverdueItem
	err        error
	limit      int
	restricted bool
}

func (f *overdueStoreFake) OverdueTasks(_ context.Context, limit int) ([]domain.OverdueItem, error) {
	f.limit = limit
	return f.items, f.err
}

func (f *overdueStoreFake) OverdueCitizenReports(_ context.Context, limit int, restricted bool) (
	[]domain.OverdueItem, error) {
	f.limit, f.restricted = limit, restricted
	return f.items, f.err
}

// hoursFake answers "N working hours after `from`" with a FIXED instant per origin — deliberately NOT
// 48 wall-clock hours, so a use case that added hours itself would compute a different instant and
// the critical flags would come out wrong.
type hoursFake struct {
	answer map[time.Time]time.Time
	asked  []time.Time
	amount []uint32
	err    error
	drop   bool // answer without the amount asked for

	// The MeasureWorkingHours half.
	seconds      map[time.Time]uint64
	measureErr   error
	measureDrop  bool // leave the last span of each call unanswered
	measureCalls int
	measured     []identityclient.WorkingSpan
}

// vi-name-ok: implements the existing core/identityclient.Client method WorkingHoursCalculator mirrors
func (h *hoursFake) TienGioLamViec(_ context.Context, from time.Time, gio []uint32) (map[uint32]time.Time, error) {
	h.asked = append(h.asked, from)
	h.amount = gio
	if h.err != nil {
		return nil, h.err
	}
	if h.drop {
		return map[uint32]time.Time{}, nil
	}
	return map[uint32]time.Time{gio[0]: h.answer[from]}, nil
}

// MeasureWorkingSeconds answers a FIXED number per span start — deliberately not End-Start, so a use
// case that computed wall-clock seconds itself would produce different numbers.
func (h *hoursFake) MeasureWorkingSeconds(_ context.Context, spans []identityclient.WorkingSpan) (
	map[identityclient.WorkingSpan]uint64, error) {
	h.measureCalls++
	h.measured = append(h.measured, spans...)
	if h.measureErr != nil {
		return nil, h.measureErr
	}
	out := map[identityclient.WorkingSpan]uint64{}
	for i, sp := range spans {
		if h.measureDrop && i == len(spans)-1 {
			continue
		}
		out[sp] = h.seconds[sp.Start]
	}
	return out, nil
}

var (
	queueNow = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	// Friday evening: 48 WALL-CLOCK hours later is Sunday, but 48 WORKING hours later is well into
	// the following week. The fake answers the working-hours instant.
	dueFriday   = time.Date(2026, 9, 18, 17, 0, 0, 0, time.UTC)
	dueThursday = time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
)

func newQueueForTest(s *overdueStoreFake, h *hoursFake) *OverdueQueue {
	q := NewOverdueQueue(s, s, h)
	q.clock = func() time.Time { return queueNow }
	return q
}

func TestOverdueQueueCriticalFromWorkingHours(t *testing.T) {
	s := &overdueStoreFake{items: []domain.OverdueItem{
		{Kind: domain.DeadlineTask, Code: "NV01", MissedDeadline: dueFriday},
		{Kind: domain.DeadlineTask, Code: "NV02", MissedDeadline: dueThursday},
		// Same instant as NV01 in another zone: ONE question to identity, not two.
		{Kind: domain.DeadlineTask, Code: "NV03", MissedDeadline: dueFriday.In(time.FixedZone("ICT", 7*3600))},
	}}
	h := &hoursFake{answer: map[time.Time]time.Time{
		dueFriday:   time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC), // exactly now -> critical (boundary)
		dueThursday: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),  // later -> not yet
	}}
	got, err := newQueueForTest(s, h).Tasks(context.Background(), 10)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if !got[0].Critical || got[1].Critical || !got[2].Critical {
		t.Errorf("critical = %v %v %v, muốn true false true", got[0].Critical, got[1].Critical, got[2].Critical)
	}
	if len(h.asked) != 2 {
		t.Errorf("hỏi identity %d lần, muốn 2 (một lần mỗi hạn khác nhau)", len(h.asked))
	}
	if len(h.amount) != 1 || h.amount[0] != domain.CriticalWorkingHours {
		t.Errorf("hỏi %v giờ làm việc, muốn [%d]", h.amount, domain.CriticalWorkingHours)
	}
	if s.limit != 10 {
		t.Errorf("limit xuống kho = %d", s.limit)
	}
	// The order is the store's; the use case does not re-sort.
	if got[0].Code != "NV01" || got[1].Code != "NV02" || got[2].Code != "NV03" {
		t.Errorf("thứ tự bị đổi: %+v", got)
	}
}

// TestOverdueQueueIdentityFailureFailsRequest — no list with `critical: false` for everything.
func TestOverdueQueueIdentityFailureFailsRequest(t *testing.T) {
	for name, h := range map[string]*hoursFake{
		"identity lỗi":          {err: errors.New("rpc Unavailable")},
		"identity trả thiếu":    {drop: true},
		"identity trả mốc rỗng": {answer: map[time.Time]time.Time{}},
	} {
		t.Run(name, func(t *testing.T) {
			s := &overdueStoreFake{items: []domain.OverdueItem{{Code: "PA-X", MissedDeadline: dueFriday}}}
			got, err := newQueueForTest(s, h).CitizenReports(context.Background(), 10, false)
			if !errors.Is(err, ErrWorkingHoursUnavailable) {
				t.Errorf("err = %v, muốn ErrWorkingHoursUnavailable", err)
			}
			if got != nil {
				t.Errorf("vẫn trả danh sách khi không tính được mức nghiêm trọng: %+v", got)
			}
		})
	}
}

func TestOverdueQueueEmptyAsksNobody(t *testing.T) {
	h := &hoursFake{}
	got, err := newQueueForTest(&overdueStoreFake{}, h).Tasks(context.Background(), 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
	if len(h.asked) != 0 {
		t.Error("không có dòng nào mà vẫn gọi identity")
	}
}

func TestOverdueQueueRestrictedReachesStore(t *testing.T) {
	for _, restricted := range []QuyenXemHanChe{false, true} {
		s := &overdueStoreFake{}
		if _, err := newQueueForTest(s, &hoursFake{}).CitizenReports(context.Background(), 5, restricted); err != nil {
			t.Fatal(err)
		}
		if s.restricted != bool(restricted) {
			t.Errorf("restricted=%v xuống kho thành %v", restricted, s.restricted)
		}
	}
}

func TestOverdueQueueStoreErrorWrapped(t *testing.T) {
	cause := errors.New("mất kết nối")
	_, err := newQueueForTest(&overdueStoreFake{err: cause}, &hoursFake{}).Tasks(context.Background(), 10)
	if !errors.Is(err, cause) || errors.Is(err, ErrWorkingHoursUnavailable) {
		t.Errorf("err = %v — lỗi kho phải được bọc và không bị nhận nhầm là lỗi identity", err)
	}
}

// --- late_working_seconds (identity MeasureWorkingHours) ---------------------------------------------

// asOfDB is the database's `now()` the overdue predicate used — deliberately NOT queueNow, the app clock
// `critical` compares with, so a measurement ending at the wrong clock shows up as a wrong span.
var asOfDB = time.Date(2026, 9, 28, 9, 59, 58, 0, time.UTC)

func overdueReportsWithAsOf() *overdueStoreFake {
	return &overdueStoreFake{items: []domain.OverdueItem{
		{Kind: domain.DeadlineResolution, Code: "PA-A", MissedDeadline: dueFriday, AsOf: asOfDB},
		{Kind: domain.DeadlineClassification, Code: "PA-B", MissedDeadline: dueThursday.In(time.FixedZone("ICT", 7*3600)), AsOf: asOfDB},
	}}
}

func hoursWithMeasure() *hoursFake {
	return &hoursFake{
		answer: map[time.Time]time.Time{
			dueFriday:   time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC),
			dueThursday: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		},
		seconds: map[time.Time]uint64{dueFriday: 97213, dueThursday: 41},
	}
}

func TestOverdueQueueLateWorkingSecondsOneCallSameNow(t *testing.T) {
	h := hoursWithMeasure()
	got, err := newQueueForTest(overdueReportsWithAsOf(), h).CitizenReports(ctxXa(xaThu), 10, false)
	if err != nil {
		t.Fatalf("CitizenReports: %v", err)
	}
	if h.measureCalls != 1 {
		t.Fatalf("gọi MeasureWorkingHours %d lần, muốn 1 cho cả trang", h.measureCalls)
	}
	want := []identityclient.WorkingSpan{{Start: dueFriday, End: asOfDB}, {Start: dueThursday, End: asOfDB}}
	for i, sp := range h.measured {
		if !sp.Start.Equal(want[i].Start) || !sp.End.Equal(want[i].End) || sp.Start.Location() != time.UTC {
			t.Errorf("khoảng %d = %+v, muốn %+v (UTC, kết thúc ở now() của câu lọc)", i, sp, want[i])
		}
	}
	if got[0].LateWorkingSeconds == nil || *got[0].LateWorkingSeconds != 97213 ||
		got[1].LateWorkingSeconds == nil || *got[1].LateWorkingSeconds != 41 {
		t.Errorf("số giây = %v %v, muốn 97213 41 — nguyên văn identity", got[0].LateWorkingSeconds, got[1].LateWorkingSeconds)
	}
	if !got[0].Critical || got[1].Critical {
		t.Error("cờ critical bị đổi khi thêm phép đo")
	}
}

// Identity down, calendar not configured, a short answer: ABSENT on every row, the page still returns.
func TestOverdueQueueLateWorkingSecondsAbsentNeverZero(t *testing.T) {
	for name, mutate := range map[string]func(h *hoursFake){
		"identity không trả lời": func(h *hoursFake) {
			h.measureErr = errors.Join(identityclient.ErrIdentityUnavailable, errors.New("rpc Unavailable"))
		},
		"xã chưa cấu hình lịch": func(h *hoursFake) {
			h.measureErr = errors.Join(identityclient.ErrWorkingCalendarNotConfigured, errors.New("FailedPrecondition"))
		},
		"trả thiếu một khoảng": func(h *hoursFake) { h.measureDrop = true },
	} {
		t.Run(name, func(t *testing.T) {
			h := hoursWithMeasure()
			mutate(h)
			got, err := newQueueForTest(overdueReportsWithAsOf(), h).CitizenReports(ctxXa(xaThu), 10, false)
			if err != nil {
				t.Fatalf("lỗi đo làm hỏng cả hàng đợi: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("trả %d dòng", len(got))
			}
			for _, it := range got {
				if it.LateWorkingSeconds != nil {
					t.Errorf("%s mang %d giây dù không đo được — phải VẮNG, không bao giờ 0", it.Code, *it.LateWorkingSeconds)
				}
			}
			if !got[0].Critical {
				t.Error("critical bị mất khi phép đo hỏng")
			}
		})
	}
}

// A row without AsOf cannot be measured honestly: no call, no number on any row.
func TestOverdueQueueLateWorkingSecondsNeedsAsOf(t *testing.T) {
	s := overdueReportsWithAsOf()
	s.items[1].AsOf = time.Time{}
	h := hoursWithMeasure()
	got, err := newQueueForTest(s, h).CitizenReports(ctxXa(xaThu), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if h.measureCalls != 0 || got[0].LateWorkingSeconds != nil {
		t.Errorf("đo %d lần, dòng đầu mang %v", h.measureCalls, got[0].LateWorkingSeconds)
	}
}

func TestOverdueQueueTasksAreNotMeasured(t *testing.T) {
	h := hoursWithMeasure()
	got, err := newQueueForTest(&overdueStoreFake{items: []domain.OverdueItem{
		{Kind: domain.DeadlineTask, Code: "NV01", MissedDeadline: dueFriday, AsOf: asOfDB},
	}}, h).Tasks(ctxXa(xaThu), 10)
	if err != nil {
		t.Fatal(err)
	}
	if h.measureCalls != 0 || got[0].LateWorkingSeconds != nil {
		t.Error("hàng đợi nhiệm vụ bị đo — chỉ bảng phản ánh mang trường này")
	}
}

// A page larger than the contract's 500 spans is CHUNKED, never refused.
func TestOverdueQueueLateWorkingSecondsChunked(t *testing.T) {
	n := identityclient.MaxMeasuredSpansPerCall + 1
	s := &overdueStoreFake{}
	for i := 0; i < n; i++ {
		s.items = append(s.items, domain.OverdueItem{Kind: domain.DeadlineResolution, Code: "PA",
			MissedDeadline: dueFriday, AsOf: asOfDB})
	}
	h := hoursWithMeasure()
	got, err := newQueueForTest(s, h).CitizenReports(ctxXa(xaThu), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if h.measureCalls != 2 {
		t.Errorf("gọi %d lần cho %d khoảng, muốn 2", h.measureCalls, n)
	}
	if got[n-1].LateWorkingSeconds == nil {
		t.Error("dòng cuối (khúc thứ hai) không được đo")
	}
}

func TestOverdueQueueEmptyMeasuresNothing(t *testing.T) {
	h := hoursWithMeasure()
	if _, err := newQueueForTest(&overdueStoreFake{}, h).CitizenReports(ctxXa(xaThu), 10, false); err != nil {
		t.Fatal(err)
	}
	if h.measureCalls != 0 {
		t.Error("không có dòng nào mà vẫn gọi MeasureWorkingHours")
	}
}
