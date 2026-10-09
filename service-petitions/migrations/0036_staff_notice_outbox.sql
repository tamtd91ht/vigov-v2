-- 0036 — `staff_notice_outbox`: staff notices owed by an ACT of a person (ADR 0086 A1; transaction
-- boundary `thong_bao_can_bo_khi_giao_viec`).
--
-- WHAT IT IS: the act (a task handed to someone, an extension request filed, a task sent for review, a
-- colleague mentioned, a petition assigned, a petition reopened by a low rating) writes its business
-- row, its audit entry and ONE row here, in ONE transaction. An in-process relay
-- (internal/app/staff_notice_relay.go — advisory lock, one pod) reads the undelivered rows and calls
-- comms' DeliverStaffNotifications with the row's commune in "x-tenant-id". Comms writes the bell row
-- and the Zalo queue row; nothing about comms changes.
--
-- WHY AN OUTBOX AND NOT A CALL: calling comms inside the act's transaction would make handing out work
-- fail whenever comms does (rule 2, forbidden #5) and hold a register row lock over a network round
-- trip; calling it after commit loses the notice for ever if the process dies in between, with nothing
-- anywhere saying the person was never told (ADR 0086, stop condition #1).
--
-- # NOT AN ARCHIVAL RECORD — the same line `su_kien_di` (0005) draws
--
-- This is INFRASTRUCTURE state: "a notice is owed". The fact itself lives in the task / petition
-- tables and their timelines, and the accountability in `audit_log`. Rows may be pruned once
-- delivered, so the `ho_so_luu_tru_bat_bien` trigger is deliberately NOT attached. No pruning job is
-- written here: none is needed until the table is large, and pruning is its own decision.
--
-- # WHAT MAY GO IN `payload`
--
-- The protojson of vigov.comms.v1.StaffNotification and NOTHING ELSE: the key, the kind, staff
-- business codes, a title and body built from business codes and staff-typed task titles (ADR 0086
-- A3), a relative link. Never a petition's content, a citizen's rating comment, a citizen letter's
-- summary or an extension reason (rule 3). The table is replicated and backed up like every other.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: one per notified act — a few dozen a day for a busy commune. Empty at landing.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with its progress row; every
--      statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. A new table, read only by the relay.
--   5. RETENTION: nothing removed. Purely additive.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS staff_notice_outbox (
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,

    -- The fact WITH ITS VERSION (rule 2, invariant 4): `tasks.assigned.v1`, `petitions.reopened.v1`.
    name            TEXT        NOT NULL,

    -- The comms idempotency key, `<stored kind>:<id of the row the act wrote>` (ADR 0086 A3 #2). Also
    -- here as a column so the UNIQUE below makes a second row for one act impossible.
    idempotency_key TEXT        NOT NULL,

    -- protojson of vigov.comms.v1.StaffNotification. See the note above on what may not be in it.
    payload         JSONB       NOT NULL,

    -- When the ACT happened — not when the relay delivers it.
    occurred_at     TIMESTAMPTZ NOT NULL,

    -- NULL until comms accepted the row. With failed_class it is the whole queue state.
    delivered_at    TIMESTAMPTZ,

    -- Delivery attempts that did not succeed (comms unavailable, or a refusal). A count an operator
    -- reads; nothing decides on it.
    attempts        INTEGER     NOT NULL DEFAULT 0,

    -- Set when comms REFUSED the row (`invalid_argument` — e.g. a comms build that predates the kind,
    -- comms.proto "ROLLOUT"). Such a row is no longer drained, so it cannot block the rows behind it;
    -- clearing the column re-queues it once comms is fixed (a deliberate re-run, transaction boundary
    -- compensation (b)). NULL for every row not refused.
    failed_class    TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6), both keys.
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT staff_notice_outbox_key_once UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT staff_notice_outbox_attempts_non_negative CHECK (attempts >= 0),
    CONSTRAINT staff_notice_outbox_failed_class_shape CHECK (failed_class IS NULL OR failed_class <> '')
) PARTITION BY HASH (tenant_id);

-- 32 partitions, the modulus every partitioned table of this service uses (0005 `su_kien_di`).
DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS staff_notice_outbox_p%s PARTITION OF staff_notice_outbox '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The relay's own query: what is still owed, oldest first, in one commune — and the loose index scan
-- that lists the communes owing anything (store/crosstenant). PARTIAL, so it shrinks back to nothing
-- as rows are delivered instead of growing with every notice ever sent.
CREATE INDEX IF NOT EXISTS staff_notice_outbox_pending
    ON staff_notice_outbox (tenant_id, occurred_at, id)
    WHERE delivered_at IS NULL AND failed_class IS NULL;

-- BACKSTOP, as 0005: a partitioned table with no partitions rejects every INSERT — and the row here
-- shares the act's transaction, so the first task handed out would roll back entirely.
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
            current_schema(), missing_partitions;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3).
--
-- WHILE THE TABLE HOLDS NO UNDELIVERED ROW the reversal loses nothing: drop `staff_notice_outbox` (its
-- 32 partitions and the index go with it) and, in the same transaction, remove this file's row from
-- `schema_migration`. ONCE IT HOLDS UNDELIVERED ROWS, dropping it throws away notices owed to staff —
-- drain it first (the relay, or a deliberate re-run). The table is not archival; the obligations in it
-- are still owed.
-- ---------------------------------------------------------------------------
