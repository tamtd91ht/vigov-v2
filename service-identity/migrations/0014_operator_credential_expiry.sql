-- identity — expiry of the operator's temporary password and of a pending TOTP enrolment (owner's
-- decisions 28/09/2026, TASK-04: temporary password valid 24 hours, pending enrolment secret valid
-- 10 minutes).
--
-- WHY: until this file a temporary password printed by `operatorctl create` / `reset-mfa` worked
-- until somebody used it, and a pending TOTP secret — a QR code shown once — could be completed at
-- any later time. Both travel outside the system (a chat message, a screenshot, paper), so both must
-- stop working on their own. The instants are STORED (set when the credential is issued) and the
-- check is a comparison against now — never a flag somebody has to remember to flip.
--
-- THE INTERVALS HERE MUST EQUAL domain.TemporaryPasswordLifetime / domain.PendingTOTPLifetime. The
-- application sets the columns from those constants; this file only backfills with the same values.
-- internal/store/operatorstore reads this file and fails when they differ.
--
-- NO NULL-MEANS-FOREVER. The CHECKs below make "temporary password with no expiry" and "pending
-- secret with no creation time" unrepresentable, and the backfill gives every existing row a
-- finite value first — a default on a security path must be the restrictive one (rule 1, forbidden
-- #1, by analogy; CLAUDE.md "fail closed"). No operator row exists in any environment yet, so the
-- backfill is expected to touch nothing; it is written so that it would be correct if it did.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0012/0013: both are committed, and core/migrate checksums every
-- applied file at startup (ErrChecksumLech).
--
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL:
--
--   1. HOW MANY ROWS PER COMMUNE: none — the operator realm has no commune. Platform-wide, a handful
--      of operator accounts at most.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, progress row
--      inside it. Columns are added IF NOT EXISTS, the backfill only fills NULLs, constraints are
--      added only if absent — a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none — it is all-or-nothing. After
--      it, a temporary password older than 24 hours and a pending secret older than 10 minutes stop
--      working; that is the intended change.
--   5. RETENTION: nothing is removed. Two nullable columns are added; no row is deleted and no value
--      other than a NULL is overwritten.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column; the backfill is two filtered
-- statements over a table of a few rows. PERSONAL DATA (rule 3): none in this file.

-- temporary_password_expires_at — set with must_change_password = true (CreateAccount, SetPassword
-- with a temporary password), cleared when the operator sets their own password (ActivateTOTP,
-- SetPassword). pending_totp_created_at — set by SetPendingTOTP, cleared by ActivateTOTP / ResetMFA.
ALTER TABLE operator_account ADD COLUMN IF NOT EXISTS temporary_password_expires_at TIMESTAMPTZ;
ALTER TABLE operator_account ADD COLUMN IF NOT EXISTS pending_totp_created_at TIMESTAMPTZ;

-- Backfill: a finite expiry for every temporary password and pending secret already stored.
UPDATE operator_account
   SET temporary_password_expires_at = now() + interval '24 hours'
 WHERE must_change_password AND temporary_password_expires_at IS NULL;

UPDATE operator_account
   SET pending_totp_created_at = now()
 WHERE pending_totp_secret_sealed IS NOT NULL AND pending_totp_created_at IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'operator_account_temporary_password_expires'
           AND conrelid = 'operator_account'::regclass
    ) THEN
        ALTER TABLE operator_account
            ADD CONSTRAINT operator_account_temporary_password_expires
            CHECK (NOT must_change_password OR temporary_password_expires_at IS NOT NULL);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'operator_account_pending_totp_dated'
           AND conrelid = 'operator_account'::regclass
    ) THEN
        ALTER TABLE operator_account
            ADD CONSTRAINT operator_account_pending_totp_dated
            CHECK (pending_totp_secret_sealed IS NULL OR pending_totp_created_at IS NOT NULL);
    END IF;
END $$;

COMMENT ON COLUMN operator_account.temporary_password_expires_at IS
    'When the temporary password stops working (domain.TemporaryPasswordLifetime, 24h). Required while must_change_password.';
COMMENT ON COLUMN operator_account.pending_totp_created_at IS
    'When the pending TOTP secret was stored; it can be completed for domain.PendingTOTPLifetime (10 min).';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose. The reverse is written here as prose, and
-- NONE OF IT IS A RUNNABLE LINE — a runnable line is a line that gets run.
--
-- In one transaction: drop the constraints operator_account_temporary_password_expires and
-- operator_account_pending_totp_dated, then the two columns temporary_password_expires_at and
-- pending_totp_created_at (they hold only instants derived from other columns, no record of any
-- act), then remove this file's progress row from schema_migration, keyed on
-- ten = '0014_operator_credential_expiry.sql'. The application code of TASK-04 reads both columns,
-- so it must be rolled back first. Reversing removes two owner-decided security thresholds, so it
-- needs the owner's decision (rule 13, forbidden #4).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0013: a partitioned table with no partitions rejects every
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
