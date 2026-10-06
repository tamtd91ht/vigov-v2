-- 0014 — the per-commune counter behind §9's `☑ Tự sinh mã` (docs/ui-ux/06-giai-ngan.md §9): a
-- project created with no `code` is issued the next one in the series DA01, DA02 … DA99, DA100.
-- User decision 06/10/2026 — format and scope (ONE series per commune, not per budget year) follow
-- the prototype (vigov-require budget/service.py `_next_serial_code`).
--
-- WHY A NEW FILE: 0001..0013 have been applied and core/migrate compares the checksum of every
-- applied file at startup (ErrChecksumLech).
--
-- ---------------------------------------------------------------------------
-- WHAT THE COUNTER IS FOR, AND WHAT IT IS NOT.
--
--   IT SERIALISES. Two clerks pressing `Thêm` at the same instant must not both compute "DA07". The
--   create transaction takes this row FOR UPDATE (the upsert in fistore.DuAnGhiStore
--   .LockProjectSerial) before choosing a number, so the second waits for the first to commit and
--   then reads the advanced value. A MAX(ma)+1 over `du_an` cannot do that: it reads an absence
--   nothing can lock.
--
--   IT IS NOT WHAT MAKES "NEVER REISSUED" TRUE. That is `UNIQUE (tenant_id, ma)` on `du_an` (0004),
--   which counts soft-deleted rows, plus the create path stepping over every number whose code
--   already exists in `du_an` (removed rows included). So a counter row that were lost or reset
--   would make the next create step over the taken codes — slower, never a reissue. A ROLLED-BACK
--   create does not advance the counter either, and that is correct: a code nobody committed was
--   never issued.
--
--   CODES TYPED BY HAND are still allowed and still must never have been used (§9). The counter
--   does not jump to them; the create path steps over them when it meets them.
--
-- NOT AN ARCHIVAL RECORD: one integer per commune, derivable (at worst slowly) from `du_an`. So no
-- soft-delete columns and no audit entry of its own — the act it serves is audited as
-- `them_du_an`, whose delta carries the code issued and `auto_code: true`. A hard DELETE is refused
-- anyway (trigger below), so nothing in the application can quietly reset a commune's series.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: zero now; at most one ever (PRIMARY KEY (tenant_id)), created by the
--      first auto-coded create of that commune.
--   2. IF IT STOPS HALF-WAY: one file, one transaction (core/migrate). Every statement is IF NOT
--      EXISTS / DROP TRIGGER IF EXISTS, so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING: none — nothing reads this table except the create path.
--      ROLLING DEPLOY: an old replica refuses a blank `code` (400) as it always did; it never touches
--      this table, and a code it inserts by hand is stepped over by a new replica.
--   5. RETENTION: kept for the life of the commune. Losing it costs nothing but speed (point 1 of the
--      header), which is why the reversal below is lossless.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: ProjectCodeCounter
-- @scope:  tenant
--
-- project_code_counters — the next serial number to try for one commune's auto-issued project codes.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_code_counters (
    tenant_id    TEXT        NOT NULL,
    -- The NEXT number to try. Starts at 1 (DA01). Only ever moves forward: the create path writes
    -- (issued number + 1), and the issued number is >= the value it read under the row lock.
    next_number  BIGINT      NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One series per commune (rule 1, invariant 6: keyed by tenant_id, which is the partition key).
    PRIMARY KEY (tenant_id),
    CONSTRAINT project_code_counters_next_number_positive CHECK (next_number >= 1)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS project_code_counters_p%s PARTITION OF project_code_counters '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- A reset series is not a reissue (header), but it is still nobody's to do from the application.
DROP TRIGGER IF EXISTS project_code_counters_no_hard_delete ON project_code_counters;
CREATE TRIGGER project_code_counters_no_hard_delete
    BEFORE DELETE ON project_code_counters
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   1. Roll back the Go code that reads it first (an auto-coded create would otherwise fail).
--   2. DROP TABLE project_code_counters;   (32 partitions and the trigger go with it)
--      ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's and other tables are attached to it.
--   3. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
--
-- LOSSLESS: every code ever issued is still in `du_an`, and re-applying this file re-creates an empty
-- counter whose first use steps over them.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (see 0008).
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
