-- identity — the operator session lifetime, capped by the database (ADR 0048 §Chốt bước 1 —
-- 28/09/2026, row "Tham số": "Phiên 8 giờ tuyệt đối, không refresh").
--
-- WHY: until this file the 8-hour ABSOLUTE lifetime lived in one place only, the Go constant
-- domain.SessionLifetime, applied by operatorstore.CreateSession. An operator session reaches every
-- commune's platform metadata; a second write path — a later statement that extends expires_at, a
-- hand-run UPDATE, a bug that passes a different duration — would silently turn a one-day credential
-- into a permanent one, and nothing would report it. The CHECK makes the database refuse any row
-- whose expiry lies beyond created_at + 8 hours, whoever writes it.
--
-- THE INTERVAL MUST EQUAL domain.SessionLifetime. Two literals are two values that drift, so
-- internal/store/operatorstore reads this file and fails when they differ. Raising the lifetime is an
-- owner decision (rule 13, forbidden #4: silently loosening a threshold) and takes a NEW migration
-- that replaces this constraint — never an edit of this file.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0012: 0012 is committed, and core/migrate checksums every
-- applied file at startup (ErrChecksumLech).
--
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL:
--
--   1. HOW MANY ROWS PER COMMUNE: none — the operator realm has no commune. Platform-wide, at most a
--      few operator sessions; the table is empty in every environment where the CLI has not run.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, progress row
--      inside it. The constraint is added only IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. No column, index or row
--      changes; the constraint only refuses future writes that break the owner's rule.
--   5. RETENTION: nothing is removed. If an existing row already exceeds the cap, the migration
--      REFUSES (see the DO block) rather than rewriting or revoking it — which session to revoke is a
--      person's call, not this file's.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill — nothing to
-- iterate and nothing to resume. PERSONAL DATA (rule 3): none in this file.

DO $$
DECLARE over_cap bigint;
BEGIN
    SELECT count(*) INTO over_cap
      FROM operator_session
     WHERE expires_at > created_at + interval '8 hours';

    IF over_cap > 0 THEN
        RAISE EXCEPTION 'operator_session has % row(s) whose expiry exceeds created_at + 8 hours', over_cap
            USING HINT = 'Every session the application issues lasts exactly 8 hours (domain.SessionLifetime). '
                         'A longer one was written by some other path; find it and revoke those sessions '
                         '(revoked_at, revoked_reason) before applying this migration. It refuses rather '
                         'than guessing.';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'operator_session_lifetime_cap'
           AND conrelid = 'operator_session'::regclass
    ) THEN
        ALTER TABLE operator_session
            ADD CONSTRAINT operator_session_lifetime_cap
            CHECK (expires_at <= created_at + interval '8 hours');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose. The reverse is written here as prose, and
-- NONE OF IT IS A RUNNABLE LINE — a runnable line is a line that gets run.
--
-- In one transaction: drop the constraint operator_session_lifetime_cap from operator_session, then
-- remove this file's progress row from schema_migration, keyed on
-- ten = '0013_operator_session_lifetime_cap.sql'. No data is touched either way. Reversing it
-- removes the database's half of an owner-decided security threshold, so it needs the owner's
-- decision first (rule 13, forbidden #4).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0012: a partitioned table with no partitions rejects every
-- INSERT. This file declares no partitioned table and is expected to find nothing; it runs anyway,
-- because the check only describes the state after the newest migration that carries it.
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
