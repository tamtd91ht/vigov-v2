-- 0030 — operator_account.is_owner: the ONE operator account that owns the platform (owner decision
-- 13/10/2026, ADR 0048 amendment).
--
-- AT MOST ONE OWNER, ENFORCED HERE, NOT IN GO. operator_account_single_owner is a UNIQUE index on the
-- constant `true`, partial on is_owner: every owner row has the same key, so a second one is refused
-- by PostgreSQL whichever path writes it — the CLI, a hand-run statement, two concurrent
-- `operatorctl set-owner` runs. A check in Go would be a read-then-write race.
-- "At most", not "exactly": zero owners is the state this file leaves, and stays legal.
--
-- NO BACKFILL, ON PURPOSE. Which account is the owner is a person's designation, made later by
-- `operatorctl set-owner` (run by a Jenkins job, which trails it in operator_audit_log). A migration
-- that picked one — the oldest account, VH-00001 — would be the migration deciding who holds the
-- platform. Every existing row gets false.
--
-- operator_account_owner_not_disabled — THE OWNER IS NEVER A DISABLED ACCOUNT. A disabled owner is a
-- platform whose owner cannot sign in, with the unique index still blocking anyone else from being
-- designated: the role would be held by nobody and transferable by nobody but a hand-run statement.
-- Fail closed, at the database: disabling the owner, or designating a disabled account, is refused;
-- the owner must be moved to another account first. Consequence, stated: a Go path that disables an
-- account without checking is_owner gets a raw 23514 for the owner instead of a readable refusal —
-- operatorctl should test is_owner first and say why.
--
-- NAMES (rule 12, ADR 0051): English, like every column of 0012.
--
-- WHY A NEW FILE: core/migrate checksums every applied file at startup and stops the service when one
-- has changed.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none — operator_account has no commune (0012: @scope platform). A handful of
--      platform-wide rows. ADD COLUMN … NOT NULL DEFAULT false is catalogue-only in PG16 (a constant
--      default is not a rewrite); the CHECK and the index scan the handful once, and every row
--      passes, because every is_owner is false.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with the
--      runner's progress row. A re-run is free: ADD COLUMN IF NOT EXISTS, the CHECK behind a
--      pg_constraint lookup, the index IF NOT EXISTS.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING: none while half-applied (one transaction). After it: store
--      reads name their columns and insertAccount names its columns, so every existing read and
--      write behaves exactly as before; a new account gets is_owner = false by default. The one write
--      that changes is disabling an account that IS the owner — refused (see above), and no owner
--      exists until set-owner runs.
--   5. RETENTION: nothing dropped, retyped, renumbered or rewritten. updated_at is not touched.
--
-- AUDIT (rule 6, invariant 6): this file changes no value (false for every row is "nobody designated
-- yet", which is the truth), so it writes no trail entry. Designating the owner is a write by
-- operatorctl, trailed there.
--
-- PERSONAL DATA (rule 3): none. A boolean.
-- ---------------------------------------------------------------------------

ALTER TABLE operator_account ADD COLUMN IF NOT EXISTS is_owner BOOLEAN NOT NULL DEFAULT false;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'operator_account'::regclass
                     AND conname = 'operator_account_owner_not_disabled') THEN
        ALTER TABLE operator_account ADD CONSTRAINT operator_account_owner_not_disabled
            CHECK (NOT is_owner OR disabled_at IS NULL);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- operator_account_single_owner — at most one row with is_owner = true, platform-wide.
--
-- WITHOUT tenant_id, AND THAT IS CORRECT (rule 1, invariant 6 does not apply): operator_account has
-- no commune by decision (0012), and "one owner" means one for the whole platform. A key composite
-- with tenant_id would mean one owner per commune — a different rule nobody decided.
--
-- @scope:  platform
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS operator_account_single_owner
    ON operator_account ((true)) WHERE is_owner;

COMMENT ON COLUMN operator_account.is_owner IS
    'The platform owner account (owner decision 13/10/2026, ADR 0048). At most one row is true '
    '(operator_account_single_owner); never a disabled account (operator_account_owner_not_disabled). '
    'Designated only by operatorctl set-owner, trailed in operator_audit_log; no migration picks it.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction; core/migrate has no automatic
-- rollback. Prose, not a runnable line — a runnable line is a line that gets run.
--
--   drop index operator_account_single_owner, drop constraint operator_account_owner_not_disabled,
--   drop column is_owner, and remove this file's row from schema_migration
--   (ten = '0030_operator_owner.sql'). Any Go path reading or writing is_owner (operatorctl
--   set-owner) is reverted in the same release.
--
-- WHAT IS LOST: while no owner has been designated — the state this file leaves — nothing; every row
-- is false. ONCE set-owner HAS RUN, dropping the column discards who holds the platform. The
-- designation is recoverable from operator_audit_log (append-only, 0012), which reversal never
-- touches; restoring it is a NEW migration. From then on reversal is a user decision (rule 7, stop
-- condition #2).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0029: a partitioned table with no partitions rejects every
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
