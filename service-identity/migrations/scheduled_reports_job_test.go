package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0029 (`scheduled_reports`, ADR 0086 B2). Same weaker-half caveat as
// danh_ba_mini_app_test.go: it proves the DDL is WRITTEN, not that PostgreSQL enforces it.

const file0029 = "0029_scheduled_reports_job.sql"

// THE MUTATION THAT MUST TURN THIS RED: one of the three `job` CHECKs left at three jobs — a claim or
// a save of the new job then fails at the database, in production only, for every commune.
func TestMigration0029JobChecksAdmitScheduledReports(t *testing.T) {
	sql := maChay(t, file0029)
	for _, name := range []string{"automation_job_setting_job_known", "automation_run_scope_job_known",
		"automation_run_job_known"} {
		if got, want := codesOf(t, sql, name, "job"), "escalation,scheduled_reports,sla_reminders,weekly_digest"; got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

// The cadence of `scheduled_reports` is weekday + HH:MM with no interval — `weekly_digest`'s shape — and
// a run carries a period exactly when its job is `scheduled_reports`.
func TestMigration0029ShapeAndPeriodConstraints(t *testing.T) {
	sql := maChay(t, file0029)
	for _, c := range []struct{ want, why string }{
		{"or (job in ('weekly_digest', 'scheduled_reports') and interval_minutes is null and run_hour between 0 and 23 " +
			"and run_minute between 0 and 59 and weekday between 1 and 7))", "weekday + HH:MM shape"},
		{"(job = 'sla_reminders' and interval_minutes between 5 and 10080", "sla_reminders shape kept"},
		{"or (job = 'escalation' and interval_minutes is null and run_hour between 0 and 23 and run_minute between 0 and 59 " +
			"and weekday is null)", "escalation shape kept"},
		{"alter table automation_run add column if not exists scheduled_report_period text;", "nullable period column"},
		{"(job = 'scheduled_reports' and scheduled_report_period in ('week', 'month')) " +
			"or (job <> 'scheduled_reports' and scheduled_report_period is null)", "period iff scheduled_reports"},
		{"where c.relkind = 'p' and n.nspname = current_schema()", "partition backstop"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0029 is missing %q — %s", c.want, c.why)
		}
	}
}

// Additive: no row written, removed or rewritten, no column dropped.
func TestMigration0029DestroysNothing(t *testing.T) {
	sql := maChay(t, file0029)
	for _, bad := range []string{"insert into", "update ", "delete from", "truncate", "drop column", "drop table"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0029 contains %q in its executable part — constraints and one nullable column only", bad)
		}
	}
}
