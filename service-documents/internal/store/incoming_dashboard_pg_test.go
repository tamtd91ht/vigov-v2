package store

import (
	"context"
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// Against a real PostgreSQL: THE ROW COUNT OF EACH DRILL-DOWN EQUALS ITS FIGURE, in one commune, with
// the other commune's rows and a soft-deleted row present to be wrongly counted. SKIPPED WITHOUT
// VIGOV_TEST_DSN — see loai_van_ban_pg_test.go on what a green run without it does and does not prove.

type incomingFixture struct {
	id, status string
	arrived    string // YYYY-MM-DD
	due        time.Time
	deleted    bool
}

func insertIncoming(t *testing.T, db *sql.DB, tenantID string, no int, f incomingFixture) {
	t.Helper()
	var delAt, delBy, delReason any
	if f.deleted {
		delAt, delBy, delReason = time.Now().UTC(), "CB-00123", "vào sổ nhầm"
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO van_ban_den (tenant_id, id, so_vao_so, nam, ngay_den, co_quan_ban_hanh,
		   loai_van_ban, trich_yeu, han_xu_ly_xong, trang_thai, nguoi_tao_ma,
		   deleted_at, deleted_by, delete_reason)
		 VALUES ($1, $2, $3, 2026, $4::date, 'UBND huyện', 'cong-van', 'Trích yếu thử', $5, $6,
		   'CB-00123', $7, $8, $9)`,
		tenantID, f.id, no, f.arrived, f.due, f.status, delAt, delBy, delReason); err != nil {
		t.Fatalf("insert %s: %v", f.id, err)
	}
}

func TestPgEachDrillDownHasAsManyRowsAsItsFigure(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	s := NewVanBanDenStore(pkgstore.New(db))

	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC) // 10:00 ICT
	past := now.Add(-time.Minute)                       // a deadline one minute ago
	later := now.Add(time.Minute)

	for i, f := range []incomingFixture{
		// in period, open, late
		{id: "vb-1", status: "moi-vao-so", arrived: "2026-09-01", due: past},
		// in period (last day), open, not yet due
		{id: "vb-2", status: "dang-xu-ly", arrived: "2026-09-30", due: later},
		// in period, finished and past its deadline: NOT overdue (QuaHan), NOT open
		{id: "vb-3", status: "da-giai-quyet", arrived: "2026-09-15", due: past},
		// before the period, open, late
		{id: "vb-4", status: "da-phan-cong", arrived: "2026-08-31", due: past.Add(-time.Hour)},
		// after the period, finished
		{id: "vb-5", status: "luu-khong-thu-ly", arrived: "2026-10-01", due: later},
		// deadline EXACTLY now: QuaHan is now.After(due), false at equality
		{id: "vb-6", status: "dang-xu-ly", arrived: "2026-08-01", due: now},
		// soft-deleted, would otherwise count everywhere
		{id: "vb-7", status: "moi-vao-so", arrived: "2026-09-10", due: past, deleted: true},
	} {
		insertIncoming(t, db, a, i+1, f)
	}
	// The other commune: in period, open, late — counted in A only if isolation is broken.
	insertIncoming(t, db, b, 1, incomingFixture{id: "vb-b-1", status: "moi-vao-so", arrived: "2026-09-02", due: past})

	window := domain.ArrivalWindow{
		FirstDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		LastDate:  time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
	}
	ctx := ctxXa(a)

	sum, err := s.CountIncomingSummary(ctx, window, now)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.IncomingSummary{Arrived: 3, Open: 4, Overdue: 2}
	if sum != want {
		t.Fatalf("summary = %+v, want %+v", sum, want)
	}

	req, err := page.Parse(url.Values{"limit": {"100"}}, SapXepVanBanDen)
	if err != nil {
		t.Fatal(err)
	}
	for metric, figure := range map[domain.IncomingMetric]int{
		domain.MetricArrived: sum.Arrived,
		domain.MetricOpen:    sum.Open,
		domain.MetricOverdue: sum.Overdue,
	} {
		res, err := s.DanhSach(ctx, LocVanBanDen{Metric: IncomingMetricFilter{Metric: metric, Window: window, Now: now}}, req)
		if err != nil {
			t.Fatalf("%s: %v", metric, err)
		}
		if len(res.Items) != figure {
			t.Errorf("%s: list has %d rows, figure says %d", metric, len(res.Items), figure)
		}
		for _, v := range res.Items {
			if metric == domain.MetricOverdue && !v.QuaHan(now) {
				t.Errorf("%s listed as overdue but QuaHan(now) is false", v.ID)
			}
		}
	}

	queue, err := s.OverdueIncoming(ctx, now, domain.OverdueQueueMax)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != sum.Overdue || len(queue) != 2 || queue[0].ID != "vb-4" || queue[1].ID != "vb-1" {
		t.Fatalf("queue = %+v, want vb-4 (longest missed) then vb-1", queue)
	}

	one, err := s.OverdueIncoming(ctx, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].ID != "vb-4" {
		t.Fatalf("limit 1 = %+v", one)
	}
}
