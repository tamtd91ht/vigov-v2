-- platform — upload purpose 'content-body-image' (core/storage.PurposeContentBodyImage, to be added in
-- the same release): an image placed INSIDE the body of a Mini App news article (menu noi-dung-mini-app,
-- "ảnh trong thân bài tin tức"). The purpose CHECK gains it, and its policy is seeded.
--
-- WHY A SEPARATE PURPOSE FROM 'content-image': owner decision of 03/10/2026 (ADR 0067 amendment).
-- 'content-image' is the ONE cover of an item and carries no count limit; body images are many per
-- article and the owner capped them. upload_policy holds ONE row per purpose, so a count that differs
-- needs a purpose that differs — sharing 'content-image' would either cap the cover at 20 (meaningless)
-- or leave body images uncapped (against the decision).
--
-- WHY THESE VALUES: max_bytes and allowed_mime_types are COPIED from 'content-image' as 0012 left it
-- (0012_upload_policy_content_image_cover.sql:53-54 — 52428800 bytes = 50 MiB, JPEG/PNG/WebP, no HEIC);
-- the owner's decision is "same as the cover, but a count". max_files_per_subject = 20: the owner's
-- cap of 20 body images per article, held here as a PLATFORM configuration value (one row,
-- platform-wide, operator-editable through the upload_policy path — not a Go constant). The subject is
-- the article: the comms side counts live body images per content item against this value.
-- If 0012 did NOT apply in an environment (its CHECK AFTER APPLYING), content-image there still holds
-- 0008's placeholder and the two rows differ; this file does not follow such a row — it seeds the
-- decided values, which is what 0012's header says content-image should hold too.
--
-- WHY ONE FILE FOR CHECK AND SEED (0013's reasoning): a widened CHECK with no row refuses every
-- upload; a row cannot be written without the widened CHECK. One transaction leaves both or neither.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008–0017: all have been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- ⚠ THE GO AND PROTO SIDES MUST MOVE IN THE SAME RELEASE, AND THIS FILE ALONE TURNS internal/store
-- TESTS RED UNTIL THEY DO — by design (0009:11-13, 0013, 0016): core/storage must hold
-- PurposeContentBodyImage = "content-body-image" and platform.proto UPLOAD_PURPOSE_CONTENT_BODY_IMAGE.
-- A row the RPC cannot name is a policy no caller ever receives, so every body-image upload is refused.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      This file writes ONE upload_policy row and ONE platform_audit_log entry, platform-wide; the
--      policy applies to every commune. Re-adding the CHECK scans the table once (eleven rows).
--   2. IF IT STOPS HALF-WAY: it cannot. One ALTER TABLE (drop and add as two subcommands of one
--      statement), one statement writing the row and its trail entry, and the backstop — in one
--      transaction with the runner's progress row. A failed run leaves 0016's CHECK and no row. A
--      re-run is free: DROP ... IF EXISTS re-adds the same CHECK; ON CONFLICT (purpose) DO NOTHING
--      returns no row, so the trail gets no second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. ACCESS EXCLUSIVE on
--      upload_policy for the ALTER, so ListUploadPolicies waits milliseconds rather than reading a
--      half-changed table; the CHECK only WIDENS (every row valid before is valid after — no NOT VALID
--      needed, as in 0013/0016). Before commit no 'content-body-image' policy exists (uploads refused —
--      fail closed); after commit the row exists. Cached readers (core/platformclient/uploadpolicy) see
--      it when their cache expires: until then a body-image upload is refused and can be retried — the
--      safe direction. 'content-image' (the cover) is not touched: its row, limits and stored files keep
--      their meaning.
--   5. RETENTION: nothing is removed or rewritten. The trail entry is append-only (0008 triggers); the
--      row is soft delete only.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-body-image', 'content-attachment', 'content-audio',
        'tenant-logo', 'tenant-banner', 'petition-photo', 'document-scan', 'task-attachment',
        'petition-verification-photo', 'petition-log-attachment'));

-- ---------------------------------------------------------------------------
-- SEED — 0008's shape, for 0008's reasons: MIME spellings are the core/storage constants, checked
-- against storage.ExtForMIME by internal/store/upload_policy_test.go, and the trail entry is built from
-- what the INSERT returned, not from a second typing of the values.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('content-body-image', 52428800, ARRAY['image/jpeg', 'image/png', 'image/webp'], 20, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0018_upload_policy_content_body_image.sql: chủ dự án chốt 03/10/2026 (ADR 0067 bổ sung) — ảnh trong thân bài tin Mini App, mục đích riêng với ảnh bìa (50 MB, JPG/PNG/WebP, tối đa 20 ảnh mỗi bài)'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED THE ROW AND NO IMAGE WAS UPLOADED UNDER IT. Check first:
--
--   SELECT updated_by, deleted_at FROM upload_policy WHERE purpose = 'content-body-image';
--   SELECT count(*) FROM platform_audit_log WHERE subject = 'content-body-image';  -- must be 1 (the seed)
--   -- and in service-comms: SELECT count(*) FROM stored_file WHERE purpose = 'content-body-image';  -- 0
--
-- THE ROW CANNOT BE REMOVED, AND IS NOT (0010/0013's reasoning): a hard delete is refused by
-- upload_policy_no_hard_delete, the seed entry names it, and that entry is append-only. So the CHECK
-- CANNOT be narrowed back either — the withdrawn row still carries purpose 'content-body-image' and
-- 0016's CHECK would refuse it. The reversal is a WITHDRAWAL, which ListUploadPolicies reads exactly as
-- "no row" (it filters deleted_at IS NULL) — every body-image upload refused again. In one transaction:
--   1. withdraw the row — deleted_at = now(), deleted_by = 'system', delete_reason naming the reversal
--      of this file, updated_at/updated_by likewise — keyed on purpose = 'content-body-image' AND
--      deleted_at IS NULL AND updated_by = 'system', never an unfiltered statement; and append one
--      'upload_policy.withdrawn' entry (before = the values, after NULL) in the same statement, from
--      what it returned (0013's REVERSAL shows the statement);
--   2. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0018_upload_policy_content_body_image.sql'.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
--
-- LOSSLESS while the checks above hold: the values survive in the seed entry's `after` and in the
-- withdrawn row itself. RE-APPLYING THIS FILE AFTER A REVERSAL DOES NOTHING (ON CONFLICT on the
-- withdrawn row); restoring the policy is an operator's write with its own trail entry.
--
-- ONCE AN OPERATOR HAS CHANGED THE ROW, OR AN IMAGE HAS BEEN UPLOADED UNDER IT, it is no longer a
-- reversal: the row holds a limit somebody set, and the images are part of published articles. The
-- user decides. The Go and proto sides roll back with it.
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
