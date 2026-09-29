-- 0017 — the per-commune settings and run state of the background jobs of Cấu hình → Tự động hoá
-- (docs/ui-ux/14-cau-hinh.md §9). ADR 0058, user decisions 2026-09-29.
--
-- WHY HERE: ADR 0058 §2 — the settings belong to `identity`, next to `sla`, edited under `admin.sla`,
-- and the runners (`petitions`, `documents`) reach them only through ClaimDueAutomationRuns and
-- RecordAutomationRunOutcome (proto/vigov/identity/v1/identity.proto). No runner reads these tables.
--
-- THREE TABLES, THREE DIFFERENT LIFETIMES:
--
--   automation_job_setting   what the commune chose — one row per (commune, job), edited by a person.
--   automation_run_scope     the LEASE per (commune, job, kind of work) — the one row a claim writes
--                            conditionally so only one caller wins a slot.
--   automation_run           one row per claimed run and its recorded outcome — what "Chạy lần cuối …
--                            · kết quả" shows, and what RecordAutomationRunOutcome's NOT_FOUND and
--                            first-write-wins answers are decided against.
--
-- NO ROW MEANS OFF (§9 "Mặc định tắt hết", ADR 0058 stop condition #3). Nothing here is seeded, and no
-- column has a default schedule: the suggested cadences (15 minutes, 07:00, Monday 07:30) are FORM
-- PREFILL returned by GET /api/v1/automation-jobs for a job with no row, never read by a claim.
--
-- NO SOFT-DELETE COLUMNS, DELIBERATELY. No route deletes a row of any of the three: switching a job
-- off is a STATE of its row (`enabled = false`, cadence kept), a lease is overwritten by the next lease,
-- and a run is append-then-record-once. Columns nothing writes would read as a delete path that exists.
--
-- `job` AND `work_kind` ARE TEXT + CHECK, NOT ENUM TYPES, following 0008. The job keys are English
-- (they are keys of the software's jobs, identical in every commune — the AutomationJob comment in the
-- contract); the kinds of work are `sla.loai_viec`'s own values, `don-thu` included (0016), so a scope
-- for the citizen-letter register needs no further migration once the contract carries it.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Settings: at most 3. Scopes: at most 3 jobs × 4 kinds = 12.
--      Runs: one per claim. With `sla_reminders` every 5 minutes over three kinds that is up to ~864
--      rows a day in a commune that switches it on at the fastest cadence — the same count as the audit
--      entries RecordAutomationRunOutcome must write anyway (rule 6), partitioned the same way.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none — three new tables, nothing existing
--      touched.
--   5. RETENTION: runs are the record of what a background job did to a commune's staff (whom it
--      notified is in `comms`' trail). They are never deleted by any path in this service.
-- ---------------------------------------------------------------------------

-- @entity: AutomationJobSetting
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS automation_job_setting (
    tenant_id        TEXT        NOT NULL,
    job              TEXT        NOT NULL,
    enabled          BOOLEAN     NOT NULL,

    -- THE CADENCE, one shape per job, pinned by `automation_job_setting_shape` below. Kept when the job
    -- is switched off, so switching it back on does not ask the commune to type it again.
    --   sla_reminders  interval_minutes, >= 5 (§9; ../vigov-require automation.py:57-58)
    --   escalation     run_hour:run_minute, daily, Asia/Ho_Chi_Minh
    --   weekly_digest  weekday (ISO 1 = Monday … 7 = Sunday, the convention of `lich_lam_viec.thu`),
    --                  run_hour:run_minute
    interval_minutes INTEGER,
    run_hour         INTEGER,
    run_minute       INTEGER,
    weekday          INTEGER,

    -- WHEN THE JOB WAS LAST SWITCHED ON — the instant before which no slot counts (contract:
    -- "enabling escalation at 15:00 runs it tomorrow at the configured hour"). Set only on an off→on
    -- change; never cleared, so the history of the last switch-on survives a switch-off.
    enabled_at       TIMESTAMPTZ,

    -- "RUN NOW" (ADR 0058 §7). ONE MARK PER JOB, CONSUMED ONCE PER SCOPE: a scope's claim treats the
    -- mark as due while it is later than that scope's own last claim. A job runs in several scopes
    -- (sla_reminders in petitions AND documents), so the mark cannot be cleared by the first runner —
    -- it would take the run away from the other one.
    run_requested_at TIMESTAMPTZ,
    run_requested_by TEXT,

    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A staff business code (`CB-…`), never an internal id (rule 6, invariant 8).
    updated_by       TEXT        NOT NULL,

    PRIMARY KEY (tenant_id, job),

    CONSTRAINT automation_job_setting_job_known
        CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest')),
    CONSTRAINT automation_job_setting_shape CHECK (
        (job = 'sla_reminders'
            AND interval_minutes BETWEEN 5 AND 10080
            AND run_hour IS NULL AND run_minute IS NULL AND weekday IS NULL)
     OR (job = 'escalation'
            AND interval_minutes IS NULL
            AND run_hour BETWEEN 0 AND 23 AND run_minute BETWEEN 0 AND 59
            AND weekday IS NULL)
     OR (job = 'weekly_digest'
            AND interval_minutes IS NULL
            AND run_hour BETWEEN 0 AND 23 AND run_minute BETWEEN 0 AND 59
            AND weekday BETWEEN 1 AND 7)),
    -- An enabled job with no switch-on instant would make every past slot count.
    CONSTRAINT automation_job_setting_enabled_has_instant
        CHECK (NOT enabled OR enabled_at IS NOT NULL),
    CONSTRAINT automation_job_setting_request_has_actor
        CHECK ((run_requested_at IS NULL) = (run_requested_by IS NULL)),
    CONSTRAINT automation_job_setting_updated_by_not_blank CHECK (btrim(updated_by) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS automation_job_setting_p%s PARTITION OF automation_job_setting '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- @entity: AutomationRunScope
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS automation_run_scope (
    tenant_id       TEXT        NOT NULL,
    job             TEXT        NOT NULL,
    work_kind       TEXT        NOT NULL,

    -- THE LEASE. A claim moves both columns in ONE conditional statement: it succeeds only while
    -- `last_run_id` still holds the value the claimer read, so of two replicas that both decided "due",
    -- exactly one writes. The run id is the compare token rather than the timestamp because a text
    -- equality does not depend on how an instant round-trips through the driver.
    last_run_id     TEXT        NOT NULL,
    last_claimed_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, job, work_kind),

    CONSTRAINT automation_run_scope_job_known
        CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest')),
    CONSTRAINT automation_run_scope_kind_known
        CHECK (work_kind IN ('van-ban-den', 'phan-anh', 'nhiem-vu', 'don-thu')),
    CONSTRAINT automation_run_scope_run_id_ulid CHECK (length(last_run_id) = 26)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS automation_run_scope_p%s PARTITION OF automation_run_scope '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- @entity: AutomationRun
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS automation_run (
    tenant_id                 TEXT        NOT NULL,
    id                        TEXT        NOT NULL,   -- the contract's run_id, a ULID
    job                       TEXT        NOT NULL,
    work_kind                 TEXT        NOT NULL,
    -- `schedule` or `request` (run now) — which of the two made the scope due.
    run_trigger               TEXT        NOT NULL,
    claimed_at                TIMESTAMPTZ NOT NULL,   -- identity's clock; the runner's `now`

    -- THE OUTCOME, written ONCE (first write wins — the contract). All five NULL until recorded, all
    -- five set together after: a half-recorded run is refused by the CHECK below.
    outcome                   TEXT,
    records_examined          INTEGER,
    notices_delivered         INTEGER,
    records_without_recipient INTEGER,
    recorded_at               TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT automation_run_id_ulid CHECK (length(id) = 26),
    CONSTRAINT automation_run_job_known
        CHECK (job IN ('sla_reminders', 'escalation', 'weekly_digest')),
    CONSTRAINT automation_run_kind_known
        CHECK (work_kind IN ('van-ban-den', 'phan-anh', 'nhiem-vu', 'don-thu')),
    CONSTRAINT automation_run_trigger_known CHECK (run_trigger IN ('schedule', 'request')),
    CONSTRAINT automation_run_outcome_known CHECK (outcome IS NULL OR outcome IN
        ('succeeded', 'configuration_missing', 'dependency_unavailable', 'failed')),
    CONSTRAINT automation_run_recorded_whole CHECK (
        (outcome IS NULL AND records_examined IS NULL AND notices_delivered IS NULL
            AND records_without_recipient IS NULL AND recorded_at IS NULL)
     OR (outcome IS NOT NULL AND records_examined >= 0 AND notices_delivered >= 0
            AND records_without_recipient >= 0 AND recorded_at IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS automation_run_p%s PARTITION OF automation_run '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand. BEFORE ANY COMMUNE SWITCHED A JOB ON the three tables are
-- empty and dropping them (partitions go with the parents) plus removing this file's schema_migration
-- row loses nothing. AFTER: the runs are the record of what jobs did to a commune's staff — rule 7 stop
-- condition #2, a decision for the user with a verified backup. Prose, not a runnable line.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0008: a table declared PARTITION BY and given no partitions
-- REJECTS EVERY INSERT, silently, until the first real write. Every file that declares one ends with it.
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
