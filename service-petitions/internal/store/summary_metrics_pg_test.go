package store

import (
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PostgreSQL proof that each task figure COUNTS THE ROWS ITS DRILL-DOWN LISTS.
//
// ⚠ READ BEFORE BELIEVING A GREEN RUN: this SKIPS unless VIGOV_TEST_DSN is set, and the package still
// prints `ok`. It was written on a machine with no PostgreSQL and Docker off, so IT HAS NEVER RUN. The
// fake-driver suite (summary_metrics_test.go) proves the two statements share one predicate text; only
// this file can prove that text selects the intended rows.
//
// THE PETITION HALF HAS NO SUCH SUITE YET, and that is a stated gap rather than an omission:
// `phieu_phan_anh` carries CHECKs that bind status to several columns (migrations 0004, 0005, 0011), and
// a fixture written blind against them would fail for reasons unrelated to the predicates. It belongs
// with the first session that has a database to write it against.

func TestPgTaskFiguresEqualTheirDrillDownLists(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	danhMucChoXa(t, db, tenantA)
	danhMucChoXa(t, db, tenantB)

	now := time.Now().UTC()
	past, future := now.Add(-72*time.Hour), now.Add(72*time.Hour)
	period := domain.Period{From: now.Add(-7 * 24 * time.Hour), To: now.Add(time.Hour)}
	inPeriod := now.Add(-24 * time.Hour)
	beforePeriod := period.From.Add(-time.Hour)

	type fixture struct {
		id, code, status       string
		due, originalDue, done any
	}
	fixtures := []fixture{
		{"t01", "NV01", "moi-giao", future, future, nil},                                    // in_progress
		{"t02", "NV02", "dang-thuc-hien", past, past, nil},                                  // in_progress, overdue
		{"t03", "NV03", "cho-duyet", nil, nil, nil},                                         // in_progress (no deadline, never overdue)
		{"t04", "NV04", "tam-dung", past, past, nil},                                        // suspended ONLY
		{"t05", "NV05", "hoan-thanh", future, future, inPeriod},                             // completed, sample, on_time
		{"t06", "NV06", "hoan-thanh", future, inPeriod.Add(-time.Hour), inPeriod},           // completed, sample — late vs ORIGINAL though extended
		{"t07", "NV07", "hoan-thanh", nil, nil, inPeriod},                                   // completed only (nothing promised)
		{"t08", "NV08", "hoan-thanh", past, past, beforePeriod},                             // outside the period
		{"t09", "NV09", "chuyen-tiep", past, past, nil},                                     // terminal: in no figure
		{"t10", "NV10", "hoan-thanh", past.Add(-time.Hour), past.Add(-time.Hour), inPeriod}, // finished late: NOT overdue
	}
	for _, f := range fixtures {
		if err := themNhiemVu(db, tenantA, tenantA[:10]+f.id, f.code, "theo-van-ban", f.status,
			f.due, f.originalDue, f.done); err != nil {
			t.Fatalf("thêm %s: %v", f.code, err)
		}
	}
	// A SOFT-DELETED overdue task: in no figure, in no list (rule 7, invariant 2).
	if err := themNhiemVu(db, tenantA, tenantA[:10]+"t11", "NV11", "theo-van-ban", "dang-thuc-hien",
		past, past, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = 'CB-00123',
		delete_reason = 'phép kiểm' WHERE tenant_id = $1 AND ma = 'NV11'`, tenantA); err != nil {
		t.Fatal(err)
	}
	// ANOTHER COMMUNE's overdue task, which must reach neither figure nor list.
	if err := themNhiemVu(db, tenantB, tenantB[:10]+"t01", "NV01", "theo-van-ban", "dang-thuc-hien",
		past, past, nil); err != nil {
		t.Fatal(err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(tenantA))
	got, err := s.TaskSummary(ctx, period)
	if err != nil {
		t.Fatalf("TaskSummary: %v", err)
	}
	// completed: t05 t06 t07 t10 · sample: t05 t06 t10 (t07 promised nothing) · on time: t05 only —
	// t06 was extended but missed its ORIGINAL deadline, t10 finished late.
	want := domain.TaskSummary{InProgress: 3, Overdue: 1, Suspended: 1, Completed: 4, OnTimeSample: 3, OnTime: 1}
	if got != want {
		t.Errorf("= %+v, muốn %+v", got, want)
	}

	figure := map[domain.TaskMetric]int{
		domain.TaskInProgress: got.InProgress, domain.TaskOverdue: got.Overdue,
		domain.TaskSuspended: got.Suspended, domain.TaskCompleted: got.Completed,
		domain.TaskOnTimeSample: got.OnTimeSample, domain.TaskOnTime: got.OnTime,
	}
	req, err := page.Parse(url.Values{"limit": {"100"}}, SapXepNhiemVu)
	if err != nil {
		t.Fatal(err)
	}
	for m, n := range figure {
		res, err := s.DanhSach(ctx, LocNhiemVu{Metric: m, Period: period}, req)
		if err != nil {
			t.Fatalf("DanhSach(%s): %v", m, err)
		}
		if len(res.Items) != n {
			t.Errorf("%s: ô đếm %d nhưng danh sách có %d dòng", m, n, len(res.Items))
		}
	}

	items, err := s.OverdueTasks(ctx, OverdueQueueMax)
	if err != nil {
		t.Fatalf("OverdueTasks: %v", err)
	}
	if len(items) != got.Overdue || (len(items) == 1 && items[0].Code != "NV02") {
		t.Errorf("hàng đợi = %+v, muốn đúng các dòng của ô quá hạn", items)
	}
}
