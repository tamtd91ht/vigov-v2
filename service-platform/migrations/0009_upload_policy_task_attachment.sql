-- platform — upload_policy.purpose may hold 'task-attachment' (core/storage.PurposeTaskAttachment,
-- UPLOAD_PURPOSE_TASK_ATTACHMENT in platform.proto).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008: 0008 has been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY THE CHECK MUST MOVE WITH core/storage: the CHECK on upload_policy.purpose is the database's
-- copy of core/storage's closed purpose list (0008 explains why it exists at all). Left at six
-- values, the operator could never store a policy for the seventh purpose, and the purpose would
-- stay refused forever for a reason nobody reading the Go code could find.
-- internal/store/upload_policy_test.go reads the NEWEST migration that defines this constraint and
-- compares it with storage.Purposes(), so the next purpose added to core/storage turns that test red
-- until a file like this one is written.
--
-- NO SEED ROW, ON PURPOSE (ADR 0052, stop condition #4): the limits for task attachments — size,
-- MIME types, count per task — are the owner's to set, and none has been set. With no row,
-- ListUploadPolicies returns no policy for 'task-attachment' and every such upload is REFUSED. That
-- is the fail-closed direction; a guessed limit written here would be a limit nobody decided.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy has no commune column. This file writes no
--      row. Re-adding the CHECK scans the table once to validate it: six rows today.
--   2. IF IT STOPS HALF-WAY: it cannot. One ALTER TABLE statement (drop and add as two subcommands of
--      the same statement) plus the backstop, in one transaction with the runner's progress row. A
--      failed run leaves the six-value CHECK in place, and a retry is free (DROP ... IF EXISTS).
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. The statement holds an ACCESS
--      EXCLUSIVE lock on upload_policy for its duration, so ListUploadPolicies waits (milliseconds on
--      six rows) rather than reading a table without the constraint. The CHECK only WIDENS: every
--      row valid before is valid after, and no read path filters on it.
--   5. RETENTION: nothing is removed or rewritten. No platform_audit_log entry is written because no
--      policy record changes — the trail records changes to policies, and this file changes none.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-attachment',
        'tenant-logo', 'petition-photo', 'document-scan', 'task-attachment'));

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO 'task-attachment' ROW EXISTS — soft-deleted rows included, because a
-- soft-deleted row still carries its purpose and the narrower CHECK would refuse it. Check first:
--
--   SELECT count(*) FROM upload_policy WHERE purpose = 'task-attachment';
--
-- Deliberately NO `deleted_at IS NULL` filter in that query. Zero → the down steps, in one
-- transaction:
--   1. drop the constraint upload_policy_purpose_known and re-add it with 0008's six values
--      (content-video, content-image, content-attachment, tenant-logo, petition-photo,
--      document-scan);
--   2. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0009_upload_policy_task_attachment.sql'.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
--
-- NON-ZERO → it is no longer a reversal. The row is a policy an operator set, platform_audit_log
-- holds who set it, and removing it would be a hard delete of a configuration record (refused by
-- upload_policy_no_hard_delete anyway). The user decides, with a verified backup.
--
-- The Go side must move in the same release: a binary whose core/storage still lists the seventh
-- purpose, over a database narrowed back to six, cannot store the policy it can ask for.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill, no existing row
-- rewritten — nothing to iterate and nothing to resume.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
