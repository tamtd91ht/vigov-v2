package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

type fakeOverdueReader struct {
	docs     []domain.VanBanDen
	err      error
	gotNow   time.Time
	gotLimit int
}

func (f *fakeOverdueReader) OverdueIncoming(_ context.Context, now time.Time, limit int) ([]domain.VanBanDen, error) {
	f.gotNow, f.gotLimit = now, limit
	return f.docs, f.err
}

// fakeAdvancer answers from a table keyed by the ORIGIN instant — the calendar arithmetic is identity's,
// so the fake never adds anything itself.
type fakeAdvancer struct {
	reachedAt map[int64]time.Time
	err       error
	asked     []time.Time
	hours     [][]uint32
}

// vi-name-ok: implements the existing core/identityclient.Client.TienGioLamViec method
func (f *fakeAdvancer) TienGioLamViec(_ context.Context, from time.Time, h []uint32) (map[uint32]time.Time, error) {
	f.asked = append(f.asked, from)
	f.hours = append(f.hours, h)
	if f.err != nil {
		return nil, f.err
	}
	at, ok := f.reachedAt[from.UnixNano()]
	if !ok {
		return map[uint32]time.Time{}, nil
	}
	return map[uint32]time.Time{h[0]: at}, nil
}

func sept(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, time.UTC) }

func TestOverdueQueueClassesCriticalFromIdentitysAnswer(t *testing.T) {
	now := sept(28, 3)
	// Fixture instants only; the verdict comes from the fake's table, never from arithmetic here.
	missedLongAgo, missedAtEdge, missedRecently := sept(20, 3), sept(24, 3), sept(28, 1)

	reader := &fakeOverdueReader{docs: []domain.VanBanDen{
		{ID: "a", HanXuLyXong: missedLongAgo},
		{ID: "b", HanXuLyXong: missedAtEdge},
		{ID: "c", HanXuLyXong: missedRecently},
		{ID: "d", HanXuLyXong: missedRecently}, // same deadline as c: one RPC, not two
	}}
	adv := &fakeAdvancer{reachedAt: map[int64]time.Time{
		missedLongAgo.UnixNano():  sept(27, 3), // 48 working hours already elapsed
		missedAtEdge.UnixNano():   now,         // elapses exactly now: `<= now` is critical
		missedRecently.UnixNano(): sept(30, 3), // not yet
	}}

	items, err := NewIncomingDashboard(reader, adv).OverdueQueue(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if reader.gotLimit != 10 || !reader.gotNow.Equal(now) {
		t.Errorf("reader got limit=%d now=%s", reader.gotLimit, reader.gotNow)
	}
	want := map[string]bool{"a": true, "b": true, "c": false, "d": false}
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	for i, it := range items {
		if it.Document.ID != []string{"a", "b", "c", "d"}[i] {
			t.Errorf("order changed: position %d is %s", i, it.Document.ID)
		}
		if it.Critical != want[it.Document.ID] {
			t.Errorf("%s: critical = %v, want %v", it.Document.ID, it.Critical, want[it.Document.ID])
		}
	}
	if len(adv.asked) != 3 {
		t.Fatalf("identity asked %d times, want 3 (one per distinct deadline)", len(adv.asked))
	}
	for i, origin := range []time.Time{missedLongAgo, missedAtEdge, missedRecently} {
		if !adv.asked[i].Equal(origin) {
			t.Errorf("call %d counted from %s, want the missed deadline %s", i, adv.asked[i], origin)
		}
		if h := adv.hours[i]; len(h) != 1 || h[0] != 48 {
			t.Errorf("call %d asked for %v working hours, want [48]", i, h)
		}
	}
}

func TestOverdueQueueFailsWhenIdentityCannotAnswer(t *testing.T) {
	now := sept(28, 3)
	reader := &fakeOverdueReader{docs: []domain.VanBanDen{{ID: "a", HanXuLyXong: sept(27, 3)}}}

	for name, adv := range map[string]*fakeAdvancer{
		"unreachable":    {err: errors.New("rpc error: code = Unavailable")},
		"partial answer": {reachedAt: map[int64]time.Time{}},
	} {
		t.Run(name, func(t *testing.T) {
			items, err := NewIncomingDashboard(reader, adv).OverdueQueue(context.Background(), now, 10)
			if !errors.Is(err, ErrWorkingCalendarUnavailable) {
				t.Fatalf("err = %v, want ErrWorkingCalendarUnavailable", err)
			}
			if items != nil {
				t.Fatal("a queue was returned with no calendar behind its `critical` flags")
			}
		})
	}
}

func TestOverdueQueueEmptyAsksIdentityNothing(t *testing.T) {
	adv := &fakeAdvancer{err: errors.New("must not be called")}
	items, err := NewIncomingDashboard(&fakeOverdueReader{}, adv).OverdueQueue(context.Background(), sept(28, 3), 10)
	if err != nil || len(items) != 0 || len(adv.asked) != 0 {
		t.Fatalf("items=%v err=%v asked=%d", items, err, len(adv.asked))
	}
}

func TestOverdueQueuePropagatesStoreFailure(t *testing.T) {
	boom := errors.New("db down")
	_, err := NewIncomingDashboard(&fakeOverdueReader{err: boom}, &fakeAdvancer{}).
		OverdueQueue(context.Background(), sept(28, 3), 10)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped store error", err)
	}
}
