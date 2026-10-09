-- 0029 — the fourth background job of Cấu hình → Tự động hoá: `scheduled_reports` ("Gửi báo cáo định
-- kỳ"), un-deferred by the user on 2026-10-09 (ADR 0086 B1/B2; contract d0b02227,
-- AUTOMATION_JOB_SCHEDULED_REPORTS = 4).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0017: core/migrate checksums every applied file at startup and
-- stops the service when one has changed.
--
-- ---------------------------------------------------------------------------
-- 1. THE THREE `job` CHECKS ADMIT `scheduled_reports`.
--
-- Replaced, not widened in place: PostgreSQL has no ALTER for a CHECK expression. Drop and add run in
-- this file's one transaction, so no instant leaves a column unconstrained; dropping the parent's
-- constraint drops the 32 partitions' inherited copies with it (the shape of 0016).
--
-- ---------------------------------------------------------------------------
-- 2. ITS CADENCE HAS THE SHAPE OF `weekly_digest`: weekday + HH:MM, no interval.
--
-- The schedule is the reference system's "monthly_and_weekly" (../vigov-require/apps/api/app/modules/
-- admin/automation.py:91-99): the configured weekday at HH:MM AND the 1st of every month at the same
-- HH:MM, Asia/Ho_Chi_Minh. Both slots are DERIVED from the same three columns
-- (domain.LatestSlot), so the row needs nothing `weekly_digest` does not already store.
--
-- ---------------------------------------------------------------------------
-- 3. `automation_run.scheduled_report_period` — WHICH PERIOD A `scheduled_reports` RUN REPORTED.
--
-- Identity decides it at the claim (contract: AutomationRun.scheduled_report_period; ADR 0086 B2 — two
-- runners reading their own clocks could decide two periods for one day). The claim is NOT audited
-- (0017: a lease), so RecordAutomationRunOutcome's entry is the run's only trail, and it must be able to
-- name the period the notices covered. It cannot be re-derived from `claimed_at`: a slot that opened on
-- the 1st and was claimed on the 2nd (every runner down overnight) is still a MONTH run.
--
-- Values `week` / `month`: software keys like `job` (0017's reasoning for English job keys), the
-- reference system's own PeriodKind values. Present on a `scheduled_reports` run, absent on every
-- other job — pinned by the CHECK so a run can never carry a period its job does not have.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. No row of any of the three tables changes value; the new column
--      is NULL on every existing run, which is exactly what the new CHECK requires of a run whose job
--      is not `scheduled_reports` (and no such run exists before this file).
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. After it, the
--      store selects the new column by name; no read uses `*`.
--   5. RETENTION: nothing is removed. Runs are never deleted by any path in this service (0017).
-- ---------------------------------------------------------------------------

ALTER TABLE automation_job_setting DROP CONSTRAINT IF EXISTS automation_job_setting_job_known;
ALTER TABLE automation_job_setting ADD CONSTRAINT automation_job_setting_job_known
    CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest', 'scheduled_reports'));

ALTER TABLE automation_job_setting DROP CONSTRAINT IF EXISTS automation_job_setting_shape;
ALTER TABLE automation_job_setting ADD CONSTRAINT automation_job_setting_shape CHECK (
    (job = 'sla_reminders'
        AND interval_minutes BETWEEN 5 AND 10080
        AND run_hour IS NULL AND run_minute IS NULL AND weekday IS NULL)
 OR (job = 'escalation'
        AND interval_minutes IS NULL
        AND run_hour BETWEEN 0 AND 23 AND run_minute BETWEEN 0 AND 59
        AND weekday IS NULL)
 OR (job IN ('weekly_digest', 'scheduled_reports')
        AND interval_minutes IS NULL
        AND run_hour BETWEEN 0 AND 23 AND run_minute BETWEEN 0 AND 59
        AND weekday BETWEEN 1 AND 7));

ALTER TABLE automation_run_scope DROP CONSTRAINT IF EXISTS automation_run_scope_job_known;
ALTER TABLE automation_run_scope ADD CONSTRAINT automation_run_scope_job_known
    CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest', 'scheduled_reports'));

ALTER TABLE automation_run DROP CONSTRAINT IF EXISTS automation_run_job_known;
ALTER TABLE automation_run ADD CONSTRAINT automation_run_job_known
    CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest', 'scheduled_reports'));

ALTER TABLE automation_run ADD COLUMN IF NOT EXISTS scheduled_report_period TEXT;
ALTER TABLE automation_run ADD CONSTRAINT automation_run_report_period_shape CHECK (
    (job = 'scheduled_reports' AND scheduled_report_period IN ('week', 'month'))
 OR (job <> 'scheduled_reports' AND scheduled_report_period IS NULL));

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, and ONLY while no row of the three
-- tables holds `scheduled_reports` (check all three first):
--
--   drop constraint automation_run_report_period_shape, drop column scheduled_report_period, replace
--   the four constraints above with 0017's three-job versions, and remove this file's row from
--   schema_migration.
--
-- Prose, not a runnable line, because a runnable line is a line that gets run. Once a commune has
-- switched the job on, its runs are the record of what the job sent to its leadership — rule 7 stop
-- condition #2, a decision for the user with a verified backup.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0028: a partitioned table with no partitions rejects every
-- INSERT. This file adds no table, so it is expected to find nothing; it runs anyway, because the
-- check only describes the state after the newest migration that carries it.
-- ---------------------------------------------------------------------------
DO $$
DECLARE missing_partitions text;
BEGIN
    SELECT string_agg(c.relname, ', ' ORDER BY c.relname) INTO missing_partitions
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind = 'p'
      AND n.nspname = current_schema()
      AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhparent = c.oid);

    IF missing_partitions IS NOT NULL THEN
        RAISE EXCEPTION
            'partitioned table(s) with no partitions in schema %: %',
            current_schema(), missing_partitions
            USING HINT = 'A partitioned table with no partitions rejects every INSERT. Add the '
                         'MODULUS 32 partition loop (ADR 0010) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
