package store

import (
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Against a real PostgreSQL (migration 0036): a row recorded in an act's transaction commits or rolls
// back with it; the relay's reads see ONE commune's owed rows, oldest first; delivered and set-aside rows
// leave the queue; one act cannot record two rows under one key in one commune, but two communes can use
// the same key.
//
// ⚠ SKIPS WITHOUT VIGOV_TEST_DSN, and the package still prints `ok`. Harness: danh_muc_nhiem_vu_pg_test.go.

func TestPgStaffNoticeOutbox(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewStaffNoticeOutboxStore(kho)
	ctxA, ctxB := ctxXa(tenant.ID(tenantA)), ctxXa(tenant.ID(tenantB))
	t1 := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	payload := []byte(`{"idempotency_key":"k","kind":"STAFF_NOTIFICATION_KIND_TASK_ASSIGNED","recipient_ma":["CB-1"],"title":"t"}`)

	record := func(ctxT tenant.ID, id, key string, at time.Time) error {
		return kho.For(ctxXa(ctxT)).Tx(ctxXa(ctxT), func(tx *pkgstore.ScopedTx) error {
			return s.Record(ctxXa(ctxT), tx, StaffNoticeRow{ID: id, Name: "tasks.assigned.v1", IdempotencyKey: key,
				Payload: payload, OccurredAt: at})
		})
	}
	if err := record(tenant.ID(tenantA), "o-2", "k-2", t1.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := record(tenant.ID(tenantA), "o-1", "k-1", t1); err != nil {
		t.Fatal(err)
	}
	// The same key in ANOTHER commune is a different act (composite key, rule 1 invariant 6).
	if err := record(tenant.ID(tenantB), "o-b", "k-1", t1); err != nil {
		t.Fatalf("khoá trùng ở xã khác bị từ chối: %v", err)
	}
	// The same key twice in ONE commune is refused.
	if err := record(tenant.ID(tenantA), "o-dup", "k-1", t1); err == nil {
		t.Error("một hành vi ghi được hai dòng")
	}
	// A row whose act rolls back is not there.
	err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		if err := s.Record(ctxA, tx, StaffNoticeRow{ID: "o-rb", Name: "tasks.assigned.v1", IdempotencyKey: "k-rb",
			Payload: payload, OccurredAt: t1}); err != nil {
			return err
		}
		return errRollbackForTest
	})
	if err != errRollbackForTest {
		t.Fatalf("giao dịch: %v", err)
	}

	rows, err := s.Undelivered(ctxA, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "o-1" || rows[1].ID != "o-2" {
		t.Fatalf("xã A = %+v, muốn o-1 rồi o-2 (cũ trước, không có dòng xã B, không có dòng đã rollback)", rows)
	}
	if err := s.MarkDelivered(ctxA, []string{"o-1"}, t1.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordFailedAttempt(ctxA, []string{"o-2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFailed(ctxA, []string{"o-2"}, FailedClassInvalidArgument); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.Undelivered(ctxA, 100); len(rows) != 0 {
		t.Errorf("còn %d dòng sau khi giao / gác", len(rows))
	}
	var attempts int
	if err := db.QueryRow(`SELECT attempts FROM staff_notice_outbox WHERE tenant_id = $1 AND id = 'o-2'`, tenantA).
		Scan(&attempts); err != nil || attempts != 2 {
		t.Errorf("attempts = %d (%v), muốn 2", attempts, err)
	}
	// Commune B's row is untouched by A's marks.
	if rows, _ := s.Undelivered(ctxB, 100); len(rows) != 1 || rows[0].ID != "o-b" {
		t.Errorf("xã B = %+v", rows)
	}
}

type rollbackForTest struct{}

func (rollbackForTest) Error() string { return "rollback có chủ đích" }

var errRollbackForTest error = rollbackForTest{}
