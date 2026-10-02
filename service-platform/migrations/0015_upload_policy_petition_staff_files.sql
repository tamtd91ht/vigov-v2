-- platform — two new upload purposes for STAFF files on a petition (menu phan-anh-nguoi-dan): the purpose
-- CHECK gains them, and each gets its policy.
--
--   'petition-verification-photo' (core/storage.PurposePetitionVerificationPhoto) — the "after
--       processing" photo (ảnh sau xử lý, docs/ui-ux/09 §8.4, §14.2) staff upload as evidence that the
--       case was handled. Owner decision C of 02/10/2026, reversing deferral G8 (ADR 0047:255).
--       10 MiB per file, JPEG/PNG/WebP, at most 5 per petition.
--   'petition-log-attachment' (core/storage.PurposePetitionLogAttachment) — a file attached to a
--       petition processing-log entry (docs/ui-ux/09 :197, :312). Owner decision B of 02/10/2026,
--       staff-only. 50 MiB per file, PDF/JPEG/PNG, no count limit.
--
-- THE VALUES WERE SET BY PRECEDENT 02/10/2026, CHANGEABLE BY A LATER MIGRATION. The owner decided THAT
-- both kinds exist, not their limits; the main session set the limits by analogy and they are written
-- here as that, not as an owner's figure:
--   * verification photo = the petition-photo row as it stands after 0014 (0008:199 size and count,
--     0014 MIME list — no HEIC: the same photo kind on the same record, read by the same citizen);
--   * log attachment     = the task-attachment row of 0010:39 (itself the document-scan values) —
--     the same act, a file on a log entry, on the sibling record.
-- Changing either is a new migration with its own trail entry (0012/0014 shape), never an edit here.
-- "N MB" is read as N MiB, the reading 0008, 0010, 0012 and 0013 use; max_bytes is BYTES.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008–0014: all have been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY ONE FILE FOR CHECK AND SEED (0013's reasoning): neither is useful alone — a widened CHECK with no
-- row refuses every upload, a row cannot be written without the widened CHECK.
--
-- ⚠ THE GO AND PROTO SIDES MUST MOVE IN THE SAME RELEASE. core/storage holds both purposes (key.go);
-- platform.proto must name UPLOAD_PURPOSE_PETITION_VERIFICATION_PHOTO and
-- UPLOAD_PURPOSE_PETITION_LOG_ATTACHMENT, or internal/store/upload_policy_test.go and
-- core/platformclient/uploadpolicy turn red — by design: a row the RPC cannot name is a policy no
-- caller ever receives, so every upload of that purpose is refused at step (a).
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      This file writes TWO upload_policy rows and TWO platform_audit_log entries, platform-wide; the
--      policies apply to every commune. Re-adding the CHECK scans the table once (nine rows).
--   2. IF IT STOPS HALF-WAY: it cannot. One ALTER TABLE (drop and add as two subcommands of one
--      statement), one statement writing both rows and their trail entries, and the backstop — in one
--      transaction with the runner's progress row. A failed run leaves 0013's CHECK and no row. A
--      re-run is free: DROP ... IF EXISTS re-adds the same CHECK; ON CONFLICT (purpose) DO NOTHING
--      returns no row, so the trail gets no second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. ACCESS EXCLUSIVE on upload_policy
--      for the ALTER, so ListUploadPolicies waits milliseconds; the CHECK only WIDENS (every row valid
--      before is valid after). Before commit neither policy exists (uploads refused — fail closed);
--      after commit both do. Cached readers (core/platformclient/uploadpolicy) see them when their
--      cache expires; until then such an upload is refused and can be retried — the safe direction.
--      No existing purpose's row is touched, so petition-photo and task-attachment read exactly as before.
--   5. RETENTION: nothing is removed or rewritten. The trail entries are append-only (0008 triggers);
--      the rows are soft delete only. The files these policies admit are `records` class in
--      service-petitions (core/storage key.go says why) — never purged automatically.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-attachment', 'content-audio',
        'tenant-logo', 'petition-photo', 'document-scan', 'task-attachment',
        'petition-verification-photo', 'petition-log-attachment'));

-- ---------------------------------------------------------------------------
-- SEED — 0008's shape, for 0008's reasons: MIME spellings are the core/storage constants, checked
-- against storage.ExtForMIME by internal/store/upload_policy_test.go, and each trail entry is built from
-- what the INSERT returned, not from a second typing of the values.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('petition-verification-photo', 10485760, ARRAY['image/jpeg', 'image/png', 'image/webp'],             5, 'system', 'system'),
        ('petition-log-attachment',     52428800, ARRAY['application/pdf', 'image/jpeg', 'image/png'],     NULL, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0015_upload_policy_petition_staff_files.sql: chủ dự án chốt 02/10/2026 có ảnh sau xử lý (C) và đính kèm nhật ký phiếu (B); giới hạn đặt theo tiền lệ 02/10/2026 (ảnh = petition-photo sau 0014, đính kèm = task-attachment 0010), đổi bằng migration sau'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED EITHER ROW AND NO FILE WAS UPLOADED UNDER EITHER. Check:
--
--   SELECT purpose, updated_by, deleted_at FROM upload_policy
--    WHERE purpose IN ('petition-verification-photo', 'petition-log-attachment');
--   SELECT subject, count(*) FROM platform_audit_log
--    WHERE subject IN ('petition-verification-photo', 'petition-log-attachment') GROUP BY 1;  -- 1 each
--   -- and in service-petitions:
--   SELECT purpose, count(*) FROM stored_file
--    WHERE purpose IN ('petition-verification-photo', 'petition-log-attachment') GROUP BY 1;  -- none
--
-- THE ROWS CANNOT BE REMOVED, AND ARE NOT (0010's and 0013's reasoning): a hard DELETE is refused by
-- upload_policy_no_hard_delete, and each seed entry names its row. So the CHECK cannot be narrowed back
-- either — a withdrawn row still carries its purpose. The reversal is a WITHDRAWAL, which
-- ListUploadPolicies reads exactly as "no row" (it filters deleted_at IS NULL). In one transaction:
--   1. soft delete both rows — deleted_at = now(), deleted_by = 'system', delete_reason naming the
--      reversal of this file, updated_at = now(), updated_by = 'system' — keyed on purpose IN (the two
--      purposes) AND deleted_at IS NULL AND updated_by = 'system', never an unfiltered UPDATE, RETURNING
--      the values;
--   2. in the same statement, append one platform_audit_log entry per withdrawn row: actor 'system',
--      action 'upload_policy.withdrawn', before = the returned values, after NULL, reason naming the
--      reversal of this file (0013's REVERSAL shows the statement);
--   3. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0015_upload_policy_petition_staff_files.sql'.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
--
-- RE-APPLYING THIS FILE AFTER A REVERSAL DOES NOTHING (ON CONFLICT on the withdrawn rows); restoring a
-- policy is an operator's write with its own trail entry.
--
-- ONCE AN OPERATOR HAS CHANGED A ROW, OR A FILE HAS BEEN UPLOADED UNDER IT, it is no longer a
-- reversal: the row holds a limit somebody set, and the files are parts of archival records. The user
-- decides. The Go and proto sides roll back with it.
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
