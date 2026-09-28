-- platform — the upload policy for purpose 'task-attachment' (core/storage.PurposeTaskAttachment,
-- UPLOAD_PURPOSE_TASK_ATTACHMENT in platform.proto): files attached to a task log line (§5.9).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008 OR 0009: both have been applied and core/migrate compares
-- the checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY NOW, AND WHY THESE VALUES: 0009 widened the purpose CHECK and deliberately seeded NO row,
-- because the limits were the owner's to set (ADR 0052, stop condition #4). The user set them on
-- 2026-09-29 and chose to ship them through the service's deploy, as 0008 did its six rows. The
-- values are the document-scan row of 0008 (50 MiB, PDF/JPEG/PNG, no count limit), which the owner
-- confirmed as the precedent. Until this file is applied every task-attachment upload is refused.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune
--      column. This file writes ONE upload_policy row and ONE platform_audit_log entry, platform-wide;
--      the policy applies to every commune, as the other six do.
--   2. IF IT STOPS HALF-WAY: it cannot. One statement writes the row and its trail entry, in one
--      transaction with the runner's progress row. A failed run leaves neither; a re-run is free —
--      ON CONFLICT (purpose) DO NOTHING returns no row when it already exists, so the trail gets no
--      second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Before commit
--      ListUploadPolicies returns no 'task-attachment' policy (uploads refused, the state since
--      0009); after commit it returns this row. There is no state in between: row and entry commit
--      together, and no reader sees one without the other.
--   5. RETENTION: nothing is removed or rewritten. The trail entry is append-only (0008 triggers);
--      the row is soft delete only.

-- ---------------------------------------------------------------------------
-- SEED — the same shape as 0008's, for the same reasons (0008 explains them): MIME spellings are the
-- core/storage constants, checked against storage.ExtForMIME by internal/store/upload_policy_test.go,
-- and the trail entry is built from what the INSERT returned, not from a second typing of the values.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('task-attachment',      52428800, ARRAY['application/pdf', 'image/jpeg', 'image/png'],              NULL, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0010_upload_policy_seed_task_attachment.sql: người dùng chốt 29/09/2026 (ADR 0052 §10; same values as document-scan in 0008)'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED THE ROW (every environment today). Check first: the row
-- still reads updated_by = 'system', and platform_audit_log holds exactly one entry for
-- subject = 'task-attachment' — the 'upload_policy.seeded' one this file wrote.
--
-- THE ROW CANNOT BE REMOVED, AND IS NOT: a hard DELETE is refused by upload_policy_no_hard_delete
-- (0008), and rightly — the seed trail entry names it, and that entry is append-only. The reversal
-- is a WITHDRAWAL, which ListUploadPolicies reads exactly as "no row" (it filters deleted_at IS
-- NULL), i.e. the fail-closed state 0009 left. Down steps, in one transaction:
--   1. soft delete the 'task-attachment' row: deleted_at = now(), deleted_by = 'system',
--      delete_reason naming the reversal of this file — keyed on purpose = 'task-attachment', never
--      an unfiltered UPDATE;
--   2. append one platform_audit_log entry: actor 'system', action 'upload_policy.withdrawn',
--      subject 'task-attachment', before = the seeded values, after NULL, reason naming this file;
--   3. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0010_upload_policy_seed_task_attachment.sql'.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
--
-- RE-APPLYING THIS FILE AFTER A REVERSAL DOES NOTHING: the withdrawn row still holds the key, so ON
-- CONFLICT inserts no row and writes no entry, and the policy stays withdrawn. Restoring it is an
-- operator's write (clear deleted_*, with its own trail entry), not a migration.
--
-- ONCE AN OPERATOR HAS CHANGED THE ROW it is no longer a reversal: the row holds a limit somebody
-- set and platform_audit_log holds who set it. The user decides.
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
