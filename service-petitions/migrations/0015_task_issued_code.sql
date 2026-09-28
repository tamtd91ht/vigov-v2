-- 0015 — the ledger of every task code a commune has ever issued (`task_issued_code`), and the
-- relaxation of 0006's `nhiem_vu_bat_bien` that lets `nhiem_vu.ma` change THROUGH that ledger only.
--
-- WHY THIS FILE EXISTS. User decision 28/09/2026 (card P5, kb/50-doi-chieu/2026-09-26-feat-m8-
-- multitenant-foundation-nhiem-vu.md §Mâu thuẫn #2): the task code becomes editable through PATCH, as
-- vigov-require 7764c8a has it, AND a code that was replaced stays reserved and is NEVER issued again
-- (rule 7, invariant 3). The second half is the part require gets wrong: its `code_ever_used` looks at
-- CURRENT rows only, so once NV12 is renamed nothing remembers NV12 and the next auto-generated code
-- can be NV12 again. The minutes that quote NV12 would then name a different task.
--
-- "EVER ISSUED" CANNOT BE READ FROM `nhiem_vu` ONCE `ma` CAN CHANGE. Until today `UNIQUE (tenant_id,
-- ma)` counting soft-deleted rows WAS the never-reissue guarantee, because a row never lost its code.
-- A rename breaks that: the old code leaves every row. The fact therefore needs a table of its own —
-- append-only, one row per code, keyed by commune and code.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file either stops the service (ErrChecksumLech) or leaves two databases
-- claiming one schema version while holding two schemas. `nhiem_vu_bat_bien` is REPLACED below with
-- CREATE OR REPLACE; 0006's text stays as it was applied.
--
-- ---------------------------------------------------------------------------
-- THE DESIGN, STATED ONCE.
--
--   task_issued_code   one row per (commune, code) EVER ISSUED — the current code of every task AND
--                      every code a task was renamed away from. `task_id` says which task it was
--                      issued to, so "who was NV12" stays answerable after NV12 is gone from the
--                      register. APPEND-ONLY: a row that could be removed is a code that could be
--                      reissued.
--
--   ON INSERT INTO nhiem_vu   an AFTER INSERT trigger writes the new task's code into the ledger. The
--                      ledger's primary key then refuses a code issued before — including one that
--                      was renamed away — FOR EVERY WRITER: this service, a psql prompt, a future
--                      import. The write path does not insert the ledger row itself on create, so
--                      there is one registration, not two that could disagree.
--
--   ON A CHANGE OF nhiem_vu.ma   `nhiem_vu_bat_bien` now lets `ma` change ONLY when the NEW code is
--                      already in the ledger, issued to THIS task, and the OLD code is in it too. The
--                      write path (store.ChangeCode) inserts the new code first, in the same
--                      transaction, then moves `ma`. A raw rename of `ma` with no ledger row is still
--                      refused, as it was in 0006. The old code needs no insert at rename time: it has
--                      been in the ledger since it was issued (trigger or the backfill below), and
--                      append-only keeps it there.
--
--   WHY THE RENAME IS NOT ALSO REGISTERED BY A TRIGGER, when the insert is: an insert MINTS a code and
--   the only thing that can go wrong is reuse, which the primary key refuses. A rename is an
--   ADMINISTRATIVE ACT that must leave an audit entry and a timeline row (rule 6); requiring the
--   ledger row to exist first makes it a deliberate two-statement act the write path performs, and
--   keeps a stray rename typed at a psql prompt a refusal rather than a silent renumbering.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: one ledger row per EXISTING task, soft-deleted tasks included — the
--      same count as `SELECT tenant_id, count(*) FROM nhiem_vu GROUP BY tenant_id`. No existing row
--      is modified or removed: `nhiem_vu` is only READ by the backfill. The task register has been
--      writable only since the task write path landed, so this is expected to be tens to low
--      hundreds per commune. THAT IS AN ASSUMPTION, NOT A MEASUREMENT; the operator checks first with
--      the query above.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. core/migrate runs each file in ONE
--      transaction with its progress row; a failure rolls back the table, the backfill, the triggers
--      and the function replacement together, and the next start retries from the beginning. The
--      backfill is `ON CONFLICT DO NOTHING`, every DDL statement is IF NOT EXISTS / CREATE OR REPLACE /
--      DROP TRIGGER IF EXISTS, so a retry costs nothing.
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, for the
--      reason service-identity/migrations/0003_nguoi_dung_co_tai_khoan.sql:100-107 gives — the runner
--      gives the file one transaction, so a loop inside it could not be resumed either, and the
--      genuinely resumable per-commune mechanism does not exist yet (named as missing in core/migrate).
--      The NOTICE lines are the progress record an operator can read: rows written, and the ledger's
--      size per commune. Counts and tenant ids only.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. Lossless UNTIL THE FIRST RENAME; after it, not
--      a reversal but the destruction of a record — said there in full.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can be half-applied (one
--      transaction). Between this file and the Go change that uses it: the old binary mints from
--      `max()` over `nhiem_vu` and checks duplicates on `nhiem_vu` — still exactly right, because
--      without the new binary there is no rename path, so the ledger and the register hold the same
--      codes; its inserts are registered by the new trigger, and it never writes `ma` afterwards.
--      After the Go change, minting and the duplicate check read the ledger, which is a SUPERSET of
--      the register's codes — a code that was valid to mint yesterday and is not today is exactly a
--      code that was renamed away, i.e. the point.
--   5. RETENTION: the ledger is a HISTORICAL RECORD of which number was issued to which task (rule 7,
--      invariant 3; forbidden #4). Append-only, enforced by a trigger on UPDATE/DELETE and per leaf on
--      TRUNCATE. `nhiem_vu` keeps every other refusal of 0006 unchanged: hard removal and any change
--      to `han_ban_dau` are still refused, word for word.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT DO:
--
--   * It does not drop or narrow `UNIQUE (tenant_id, ma)` on `nhiem_vu`. That key still says two
--     CURRENT tasks never share a code; the ledger says a code is never issued twice. Two statements,
--     both true.
--   * It holds no personal data. A task code is a staff-surface register number (0006:189-196).
--   * No soft-delete columns on the ledger: there is no state of it other than "everything issued".
--   * No foreign key to `nhiem_vu`: the triggers already bind every row to a task, and a key between
--     two partitioned tables would make the reversal order-dependent for nothing.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable (same reason as 0006:
-- BEFORE … FOR EACH ROW triggers on a partitioned table).
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'task_issued_code needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what keep an issued task '
            'code from being issued a second time.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: TaskIssuedCode
-- @scope:  tenant
--
-- task_issued_code — every task code a commune has ever issued, and the task it was issued to.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_issued_code (
    tenant_id  TEXT        NOT NULL,

    -- The code as issued — `NV12`, or a typed `KH-2026-07`. Same value space as `nhiem_vu.ma`
    -- (domain.KiemMaNhiemVu, 0006's `nhiem_vu_ma_khong_rong`).
    code       TEXT        NOT NULL,

    -- The internal id of the task the code was issued to. It never changes: a rename issues a NEW
    -- code to the SAME task, so after NV12 → NV15 both rows carry one task_id — which is how "NV12 in
    -- the 2026 minutes" is traced to the task that is NV15 today.
    task_id    TEXT        NOT NULL,

    -- When the code was issued. For backfilled rows, the task's own `tao_luc` — the moment the code
    -- was actually issued — not the day of this migration.
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6): every commune's register starts at NV01. This
    -- key IS the never-reissue guarantee.
    PRIMARY KEY (tenant_id, code),

    CONSTRAINT task_issued_code_not_blank CHECK (btrim(code) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS task_issued_code_p%s PARTITION OF task_issued_code '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- "Which codes did this task carry" — the reverse lookup a complaint handler needs, and what the
-- rename guard below asks. tenant_id first, like every index here (rule 1).
CREATE INDEX IF NOT EXISTS task_issued_code_by_task
    ON task_issued_code (tenant_id, task_id, issued_at);

-- ---------------------------------------------------------------------------
-- Backfill: every code already issued, SOFT-DELETED TASKS INCLUDED.
--
-- NO `deleted_at` FILTER, on purpose: a soft-deleted NV19 still owns the number 19 (0006:325-328), and
-- leaving it out of the ledger would let the next rename or mint take it.
--
-- BEFORE THE TRIGGERS, so the rename guard below never sees a task whose code is not yet registered.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    written int;
    r       record;
BEGIN
    INSERT INTO task_issued_code (tenant_id, code, task_id, issued_at)
    SELECT tenant_id, ma, id, tao_luc FROM nhiem_vu
    ON CONFLICT (tenant_id, code) DO NOTHING;
    GET DIAGNOSTICS written = ROW_COUNT;

    RAISE NOTICE '0015 backfill: % task code(s) registered in task_issued_code', written;

    FOR r IN
        SELECT tenant_id, count(*) AS n FROM task_issued_code GROUP BY tenant_id ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0015 backfill: commune % holds % issued code(s)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- The append-only guard. Same shape as 0006's `nhat_ky_nhiem_vu_chi_them`: a row-level trigger on the
-- parent (cloned onto every partition, present and future), and TRUNCATE per leaf because PostgreSQL
-- refuses a TRUNCATE trigger on a partitioned table. INSERT is absent on purpose.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION task_issued_code_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'task_issued_code is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'An issued task code is never reissued (rule 7, invariant 3). Removing or '
                     'editing its ledger row would free the code for the next task.';
END $$;

DROP TRIGGER IF EXISTS task_issued_code_no_update_delete ON task_issued_code;
CREATE TRIGGER task_issued_code_no_update_delete
    BEFORE UPDATE OR DELETE ON task_issued_code
    FOR EACH ROW EXECUTE FUNCTION task_issued_code_append_only();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'task_issued_code'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS task_issued_code_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER task_issued_code_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION task_issued_code_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- Registration on insert. AFTER, so the task row has passed its own constraints first; a code already
-- in the ledger fails the primary key here and the whole INSERT — and with it the business transaction
-- and its audit entry — rolls back. The write path checks the ledger first so the ordinary case is a
-- sentence (409 `code_taken`), not a constraint error.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION task_register_issued_code() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO task_issued_code (tenant_id, code, task_id, issued_at)
    VALUES (NEW.tenant_id, NEW.ma, NEW.id, NEW.tao_luc);
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS task_register_issued_code ON nhiem_vu;
CREATE TRIGGER task_register_issued_code
    AFTER INSERT ON nhiem_vu
    FOR EACH ROW EXECUTE FUNCTION task_register_issued_code();

-- ---------------------------------------------------------------------------
-- nhiem_vu_bat_bien — REPLACED. The hard-removal and `han_ban_dau` refusals are 0006's, unchanged. The
-- `ma` refusal becomes conditional, per the design at the top of this file.
--
-- BOTH CONDITIONS ARE CHECKED, and the second is not redundant: the NEW code in the ledger bound to
-- THIS task is what makes the change deliberate; the OLD code in the ledger is what guarantees it stays
-- reserved after the change. Every row has its code registered (backfill + insert trigger), so the
-- second check never fires today — it is the floor under a future writer that inserts into `nhiem_vu`
-- with the insert trigger disabled.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'A task carries the commune''s own progress log and an issued number: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1, and docs/ui-ux/02-nhiem-vu.md:356.';
    END IF;

    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        IF NOT EXISTS (SELECT 1 FROM task_issued_code c
                       WHERE c.tenant_id = NEW.tenant_id AND c.code = NEW.ma
                         AND c.task_id = NEW.id)
        OR NOT EXISTS (SELECT 1 FROM task_issued_code c
                       WHERE c.tenant_id = OLD.tenant_id AND c.code = OLD.ma) THEN
            RAISE EXCEPTION 'administrative record %: `ma` changes only through task_issued_code',
                TG_TABLE_NAME
                USING HINT = 'Issue the new code to this task in task_issued_code first, in the same '
                             'transaction (store.ChangeCode). The old code stays in that ledger and is '
                             'never issued again (rule 7, invariant 3).';
        END IF;
    END IF;

    IF NEW.han_ban_dau IS DISTINCT FROM OLD.han_ban_dau THEN
        RAISE EXCEPTION 'administrative record %: `han_ban_dau` is immutable', TG_TABLE_NAME
            USING HINT = 'It is the commitment as FIRST made and the denominator of the on-time '
                         'ratio (docs/ui-ux/02-nhiem-vu.md:188, :354). Granting an extension moves '
                         '`han_xu_ly` and nothing else; moving this one makes every extension read '
                         'as a deadline met, in a figure that goes upward.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (DISABLE TRIGGER, dropping the
-- table). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   FIRST CHECK THAT NO CODE HAS BEEN RENAMED AWAY — every ledger row must still be some task's
--   current code:
--
--     SELECT count(*) FROM task_issued_code c
--     WHERE NOT EXISTS (SELECT 1 FROM nhiem_vu n
--                       WHERE n.tenant_id = c.tenant_id AND n.ma = c.code);
--
--   ZERO → the ledger holds nothing the register does not, and the reversal is lossless: drop the
--   trigger `task_register_issued_code` on nhiem_vu and its function; re-run 0006's
--   `CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien()` (0006:145-171) verbatim; drop the table
--   `task_issued_code` (its partitions and triggers go with it) and the function
--   `task_issued_code_append_only()`; and remove this file's row from `schema_migration` in the same
--   transaction, otherwise the runner still believes the schema is in place.
--
--   NON-ZERO → a code has been renamed away, and its ledger row is now THE ONLY RECORD that the code
--   was ever issued, and to which task. Dropping the table is destroying that record and frees the
--   code for reissue — rule 7's stop condition #2, a user decision plus a verified backup, never a
--   command. The way back is then a NEW migration that restores 0006's trigger body and KEEPS the
--   table.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP) —
-- repeated because this file declares a new PARTITION BY table.
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
