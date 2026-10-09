package app

// What these tests defend (ADR 0086 A1, transaction boundary `thong_bao_can_bo_khi_giao_viec`):
//   - each commune's rows are delivered IN THAT COMMUNE'S CONTEXT (the client lifts it into x-tenant-id)
//     and never mixed with another's;
//   - a row's key is the same on every retry — an outage then a recovery delivers it once;
//   - UNAVAILABLE keeps rows owed, counts the attempt and backs off; INVALID_ARGUMENT isolates the bad row
//     and lets the others leave; an unreadable payload is set aside;
//   - no lock, no drain; the backoff grows and is capped; calls stay within comms' bounds.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

type pendingCommunesFake struct{ ids []tenant.ID }

func (f *pendingCommunesFake) CommunesWithPendingStaffNotices(context.Context) ([]tenant.ID, error) {
	return f.ids, nil
}

// queueFake is the outbox per commune. Every method reads the commune from ctx, as the store does.
type queueFake struct {
	mu        sync.Mutex
	rows      map[tenant.ID][]petstore.StaffNoticeRow
	delivered map[string]time.Time
	failed    map[string]string
	attempts  map[string]int
}

func newQueueFake() *queueFake {
	return &queueFake{rows: map[tenant.ID][]petstore.StaffNoticeRow{}, delivered: map[string]time.Time{},
		failed: map[string]string{}, attempts: map[string]int{}}
}

func (q *queueFake) Undelivered(ctx context.Context, limit int) ([]petstore.StaffNoticeRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []petstore.StaffNoticeRow
	for _, r := range q.rows[communeOf(ctx)] {
		if _, d := q.delivered[r.ID]; d || q.failed[r.ID] != "" {
			continue
		}
		if len(out) == limit {
			break
		}
		out = append(out, r)
	}
	return out, nil
}

func (q *queueFake) MarkDelivered(_ context.Context, ids []string, at time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, id := range ids {
		q.delivered[id] = at
	}
	return nil
}

func (q *queueFake) RecordFailedAttempt(_ context.Context, ids []string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, id := range ids {
		q.attempts[id]++
	}
	return nil
}

func (q *queueFake) MarkFailed(_ context.Context, ids []string, class string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, id := range ids {
		q.attempts[id]++
		q.failed[id] = class
	}
	return nil
}

// relayCommsFake answers like comms: a whole call refused when it carries a bad key, UNAVAILABLE when down.
type relayCommsFake struct {
	fakeComms
	down    bool
	badKeys map[string]bool
	sizes   []int
}

func (f *relayCommsFake) DeliverStaffNotifications(ctx context.Context, notices []commsclient.Notice) (
	map[string]commsclient.Delivery, error) {
	f.sizes = append(f.sizes, len(notices))
	if f.down {
		f.calls++
		return nil, fmt.Errorf("commsclient: DeliverStaffNotifications: %w: %w", commsclient.ErrCommsUnavailable,
			status.Error(codes.Unavailable, "comms xuống"))
	}
	for _, n := range notices {
		if f.badKeys[n.IdempotencyKey] {
			f.calls++
			return nil, fmt.Errorf("commsclient: DeliverStaffNotifications: %w", status.Error(codes.InvalidArgument, "loại không biết"))
		}
	}
	return f.fakeComms.DeliverStaffNotifications(ctx, notices)
}

func outboxRowFor(t *testing.T, id, key string, to ...string) petstore.StaffNoticeRow {
	t.Helper()
	payload, err := protojson.Marshal(&commsv1.StaffNotification{IdempotencyKey: key,
		Kind: commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED, RecipientMa: to, Title: "Bạn được giao nhiệm vụ: X"})
	if err != nil {
		t.Fatal(err)
	}
	return petstore.StaffNoticeRow{ID: id, Name: "tasks.assigned.v1", IdempotencyKey: key, Payload: payload}
}

func newRelay(t *testing.T, communes []tenant.ID, q *queueFake, c NoticeDeliverer, locks JobLocker) *StaffNoticeRelay {
	t.Helper()
	r, err := NewStaffNoticeRelay(StaffNoticeRelayDeps{Communes: &pendingCommunesFake{ids: communes}, Locks: locks,
		Queue: q, Comms: c, Log: quietLogger, Now: func() time.Time { return runAt }})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRelayDeliversEachCommuneInItsOwnContext(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{outboxRowFor(t, "a1", "nhiem-vu.giao-moi:nk-a", "CB-A")}
	q.rows[communeB] = []petstore.StaffNoticeRow{outboxRowFor(t, "b1", "nhiem-vu.giao-moi:nk-b", "CB-B")}
	c := &relayCommsFake{}
	r := newRelay(t, []tenant.ID{communeA, communeB}, q, c, &fakeLocks{})

	if !r.Tick(context.Background()) {
		t.Fatal("nhịp khoẻ bị báo hỏng")
	}
	if len(c.delivered) != 2 {
		t.Fatalf("giao %d, muốn 2", len(c.delivered))
	}
	for _, d := range c.delivered {
		mine := "A"
		if d.commune == communeB {
			mine = "B"
		}
		if !strings.HasSuffix(d.n.IdempotencyKey, "-"+strings.ToLower(mine)) || d.n.RecipientMa[0] != "CB-"+mine {
			t.Errorf("xã %s nhận thông báo của xã khác: %+v", d.commune, d.n)
		}
	}
	if !q.delivered["a1"].Equal(runAt) || !q.delivered["b1"].Equal(runAt) {
		t.Errorf("delivered_at = %v", q.delivered)
	}
}

func TestRelayOutageThenRecoveryDeliversOnceWithTheSameKey(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{outboxRowFor(t, "a1", "nhiem-vu.cho-duyet:nk-1", "CB-1")}
	c := &relayCommsFake{down: true}
	r := newRelay(t, []tenant.ID{communeA}, q, c, &fakeLocks{})

	if r.Tick(context.Background()) {
		t.Error("comms xuống mà nhịp báo khoẻ — bộ chuyển sẽ không lùi")
	}
	if _, d := q.delivered["a1"]; d || q.attempts["a1"] != 1 || q.failed["a1"] != "" {
		t.Fatalf("sau sự cố: delivered=%v attempts=%d failed=%q", q.delivered, q.attempts["a1"], q.failed["a1"])
	}
	c.down = false
	if !r.Tick(context.Background()) {
		t.Fatal("comms lên lại mà nhịp vẫn hỏng")
	}
	if len(c.delivered) != 1 || c.delivered[0].n.IdempotencyKey != "nhiem-vu.cho-duyet:nk-1" {
		t.Fatalf("giao = %+v", c.delivered)
	}
	// A third beat finds nothing owed: delivered once.
	r.Tick(context.Background())
	if len(c.delivered) != 1 {
		t.Errorf("giao lại dòng đã giao: %d", len(c.delivered))
	}
}

// A comms retry after a lost reply (MarkDelivered not reached) resends the SAME key; comms dedupes.
func TestRelayResendAfterLostReplyIsDeduplicatedByKey(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{outboxRowFor(t, "a1", "nhiem-vu.nhac-ten:nk-7", "CB-1")}
	c := &relayCommsFake{}
	r := newRelay(t, []tenant.ID{communeA}, q, c, &fakeLocks{})
	r.Tick(context.Background())
	delete(q.delivered, "a1") // the stamp was lost
	r.Tick(context.Background())

	if len(c.delivered) != 2 || c.delivered[0].n.IdempotencyKey != c.delivered[1].n.IdempotencyKey {
		t.Fatalf("khoá đổi giữa hai lần gửi: %+v", c.delivered)
	}
	if len(c.seen) != 1 {
		t.Errorf("comms tạo %d dòng chuông cho một hành vi, muốn 1", len(c.seen))
	}
}

func TestRelayInvalidArgumentIsolatesTheBadRow(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{
		outboxRowFor(t, "a1", "k1", "CB-1"), outboxRowFor(t, "a2", "k2-bad", "CB-2"), outboxRowFor(t, "a3", "k3", "CB-3"),
	}
	c := &relayCommsFake{badKeys: map[string]bool{"k2-bad": true}}
	r := newRelay(t, []tenant.ID{communeA}, q, c, &fakeLocks{})

	if !r.Tick(context.Background()) {
		t.Error("một dòng bị từ chối không phải sự cố — không được lùi")
	}
	if q.failed["a2"] != petstore.FailedClassInvalidArgument {
		t.Errorf("dòng hỏng không bị gác: %v", q.failed)
	}
	if _, ok := q.delivered["a1"]; !ok {
		t.Error("dòng lành a1 bị dòng hỏng chặn")
	}
	if _, ok := q.delivered["a3"]; !ok {
		t.Error("dòng lành a3 bị dòng hỏng chặn")
	}
	// The set-aside row is not read again.
	calls := c.calls
	r.Tick(context.Background())
	if c.calls != calls {
		t.Errorf("dòng đã gác vẫn được gửi lại (%d lần gọi thêm)", c.calls-calls)
	}
}

func TestRelaySetsAsideAnUnreadablePayload(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{
		{ID: "a1", IdempotencyKey: "k1", Payload: []byte("{không phải json")},
		outboxRowFor(t, "a2", "k-other", "CB-2"),
	}
	// a2's payload names its own key; give a third row whose payload names ANOTHER key.
	mismatch := outboxRowFor(t, "a3", "k-payload", "CB-3")
	mismatch.IdempotencyKey = "k-column"
	q.rows[communeA] = append(q.rows[communeA], mismatch)
	c := &relayCommsFake{}
	r := newRelay(t, []tenant.ID{communeA}, q, c, &fakeLocks{})
	r.Tick(context.Background())

	if q.failed["a1"] != FailedClassInvalidPayload || q.failed["a3"] != FailedClassInvalidPayload {
		t.Errorf("gác = %v", q.failed)
	}
	if _, ok := q.delivered["a2"]; !ok || len(c.delivered) != 1 {
		t.Errorf("dòng đọc được không đi: %+v", c.delivered)
	}
}

func TestRelayWithoutTheLockDrainsNothing(t *testing.T) {
	q := newQueueFake()
	q.rows[communeA] = []petstore.StaffNoticeRow{outboxRowFor(t, "a1", "k1", "CB-1")}
	c := &relayCommsFake{}
	locks := &fakeLocks{held: map[string]bool{}}
	r := newRelay(t, []tenant.ID{communeA}, q, c, locks)
	if !r.Tick(context.Background()) {
		t.Error("không giữ khoá là chuyện bình thường, không phải sự cố")
	}
	if c.calls != 0 || len(c.delivered) != 0 {
		t.Error("bản sao không giữ khoá vẫn gửi")
	}
	if locks.releases() != 1 {
		t.Errorf("khoá nhả %d lần", locks.releases())
	}
}

func TestRelayBatchesStayWithinCommsBounds(t *testing.T) {
	q := newQueueFake()
	for i := 0; i < 230; i++ {
		q.rows[communeA] = append(q.rows[communeA], outboxRowFor(t, fmt.Sprintf("r%03d", i), fmt.Sprintf("k%03d", i), "CB-1"))
	}
	c := &relayCommsFake{}
	r := newRelay(t, []tenant.ID{communeA}, q, c, &fakeLocks{})
	r.Tick(context.Background())
	if len(q.delivered) != 230 {
		t.Fatalf("giao %d/230", len(q.delivered))
	}
	for _, s := range c.sizes {
		if s > commsclient.MaxNoticesPerCall {
			t.Errorf("một lần gọi %d thông báo", s)
		}
	}

	// Recipients: 6 notices of 200 recipients = 1 200 > 1 000 → two calls.
	var items []relayItem
	for i := 0; i < 6; i++ {
		to := make([]string, 200)
		for j := range to {
			to[j] = fmt.Sprintf("CB-%d-%d", i, j)
		}
		items = append(items, relayItem{id: fmt.Sprint(i), notice: commsclient.Notice{IdempotencyKey: fmt.Sprint(i), RecipientMa: to}})
	}
	pages := relayPages(items)
	if len(pages) != 2 || len(pages[0]) != 5 || len(pages[1]) != 1 {
		t.Errorf("chia trang = %d trang", len(pages))
	}
}

func TestRelayWaitBacksOffAndIsCapped(t *testing.T) {
	if relayWait(20*time.Second, 0) != 20*time.Second || relayWait(20*time.Second, 2) != 80*time.Second {
		t.Errorf("lùi sai: %v %v", relayWait(20*time.Second, 0), relayWait(20*time.Second, 2))
	}
	if relayWait(20*time.Second, 50) != staffNoticeMaxBackoff {
		t.Errorf("không chặn trần: %v", relayWait(20*time.Second, 50))
	}
}

func TestRelayRunStopsOnCancel(t *testing.T) {
	r := newRelay(t, nil, newQueueFake(), &relayCommsFake{}, &fakeLocks{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run không dừng khi huỷ")
	}
}

func TestNewStaffNoticeRelayRefusesMissingDependencies(t *testing.T) {
	if _, err := NewStaffNoticeRelay(StaffNoticeRelayDeps{}); err == nil {
		t.Error("bộ chuyển thiếu phụ thuộc vẫn dựng được")
	}
}
