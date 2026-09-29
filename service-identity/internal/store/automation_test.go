package store

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT. It runs on the fake driver (kenh_cong_dan_kho_gia_test.go):
// the commune is $1 from the context or the transaction, the claim is a CONDITIONAL write whose loser
// reads as "not won", the positional scans line up with their column lists BY NAME, and the recipient
// reads carry the checker's predicate. NOT PROVED: that PostgreSQL's ON CONFLICT … WHERE really lets
// only one of two concurrent claimers write — automation_pg_test.go asserts that and SKIPS without
// VIGOV_TEST_DSN.

func TestAutomationClaimIsConditionalAndLoserIsNotWon(t *testing.T) {
	run := domain.AutomationRun{
		ID:        "01JRUN0000000000000000000A",
		Scope:     domain.AutomationScope{Job: domain.JobSLAReminders, WorkKind: domain.LoaiViecNhiemVu},
		Trigger:   domain.TriggerSchedule,
		ClaimedAt: time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC),
	}
	for _, tc := range []struct {
		rows int64
		won  bool
	}{{1, true}, {0, false}} {
		k := khoMoi()
		k.soDong = tc.rows
		s := NewAutomationStore(dbGia(k))
		var won bool
		err := dbGia(k).For(ctxXa(xaMau)).Tx(ctxXa(xaMau), func(tx *pkgstore.ScopedTx) error {
			var e error
			won, e = s.ClaimScope(context.Background(), tx, "01JPREV000000000000000000A", run)
			return e
		})
		if err != nil {
			t.Fatalf("ClaimScope: %v", err)
		}
		if won != tc.won {
			t.Errorf("rows affected %d: won = %v, want %v", tc.rows, won, tc.won)
		}
		l := k.cuoi()
		if !strings.Contains(l.sql, "ON CONFLICT (tenant_id, job, work_kind) DO UPDATE") ||
			!strings.Contains(l.sql, "WHERE automation_run_scope.last_run_id = $6") {
			t.Fatalf("claim is not a compare-and-set: %s", l.sql)
		}
		if l.args[0] != xaMau || l.args[5] != "01JPREV000000000000000000A" || l.tx == 0 {
			t.Errorf("args = %v (tx %d): want commune at $1, the read run id at $6, inside a transaction", l.args, l.tx)
		}
	}
}

func TestAutomationSettingsReadScopedAndScannedByName(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{{
		"job": "weekly_digest", "enabled": true, "interval_minutes": nil, "run_hour": int64(7),
		"run_minute": int64(30), "weekday": int64(1),
		"enabled_at":       time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC),
		"run_requested_at": nil, "run_requested_by": nil,
		"updated_at": time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC), "updated_by": "CB-0042",
	}}
	got, err := NewAutomationStore(dbGia(k)).Settings(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if l := k.cuoi(); !strings.Contains(l.sql, "WHERE tenant_id = $1") || l.args[0] != xaMau {
		t.Fatalf("not scoped: %s %v", l.sql, l.args)
	}
	s := got[0]
	if s.Job != domain.JobWeeklyDigest || !s.Enabled || s.Weekday != 1 || s.RunHour != 7 ||
		s.RunMinute != 30 || s.IntervalMinutes != 0 || !s.RunRequestedAt.IsZero() || s.UpdatedBy != "CB-0042" {
		t.Errorf("scanned %+v", s)
	}
}

func TestAutomationLastRunsJoinBindsBothTablesToCommune(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{{
		"id": "01JRUN0000000000000000000A", "job": "escalation", "work_kind": "phan-anh",
		"run_trigger": "request", "claimed_at": time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC),
		"outcome": "succeeded", "records_examined": int64(12), "notices_delivered": int64(3),
		"records_without_recipient": int64(1), "recorded_at": time.Date(2026, 9, 29, 3, 1, 0, 0, time.UTC),
	}}
	runs, err := NewAutomationStore(dbGia(k)).LastRuns(ctxXa(xaMau))
	if err != nil {
		t.Fatalf("LastRuns: %v", err)
	}
	q := k.cuoi().sql
	if !strings.Contains(q, "r.tenant_id = sc.tenant_id") || !strings.Contains(q, "sc.tenant_id = $1") {
		t.Fatalf("join not bound to the commune on both tables: %s", q)
	}
	r := runs[0]
	if r.Report.Outcome != domain.OutcomeSucceeded || r.Report.RecordsExamined != 12 ||
		r.Report.NoticesDelivered != 3 || r.Report.RecordsWithoutRecipient != 1 || r.Trigger != domain.TriggerRequest {
		t.Errorf("scanned %+v", r)
	}
}

func TestHoldersQueryUsesCheckerPredicateAndLiveUnit(t *testing.T) {
	// The STATEMENT is asserted, not a run of it: the fake driver cannot bind the []string that pgx
	// binds to text[] (the same limit LiveIDs and GiaoViecDuoc live with — their rows are proved in
	// *_pg_test.go). What matters here is that the checker's predicate is in it, by constant.
	q := holdersQuery
	for _, part := range []string{"nd.tenant_id = $1", "nd.co_tai_khoan", "nd.dang_hoat_dong",
		"nd.deleted_at IS NULL", "vt.deleted_at IS NULL", "vq.quyen_ma = $3", "nd.bo_phan_id = ANY($2)",
		"bp.deleted_at IS NULL", "bp.tenant_id = nd.tenant_id", "vt.tenant_id = nd.tenant_id",
		"vq.tenant_id = nd.tenant_id"} {
		if !strings.Contains(q, part) {
			t.Errorf("holders query lacks %q", part)
		}
	}

	// Empty input reads nothing — never "every unit".
	k2 := khoMoi()
	if got, _ := (&CanBoStore{db: dbGia(k2)}).OrgUnitPermissionHolders(ctxXa(xaMau), nil, "task.assign"); len(got) != 0 || len(k2.lenh) != 0 {
		t.Error("empty unit list issued a query or answered something")
	}
}

func TestLeadershipQueryReadsLeadershipFlagOfLiveAccounts(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{{"ma": "CB-0001"}}
	if _, err := (&CanBoStore{db: dbGia(k)}).LeadershipCodes(ctxXa(xaMau), 51); err != nil {
		t.Fatalf("leadership: %v", err)
	}
	l := k.cuoi()
	for _, part := range []string{"vt.la_lanh_dao", "nd.co_tai_khoan", "nd.dang_hoat_dong", "nd.tenant_id = $1",
		"vt.tenant_id = nd.tenant_id", "LIMIT $2"} {
		if !strings.Contains(l.sql, part) {
			t.Errorf("leadership query lacks %q", part)
		}
	}
	if l.args[1] != int64(51) {
		t.Errorf("limit = %v, want the caller's ceiling plus one", l.args[1])
	}
}
