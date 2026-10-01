-- platform — upload purpose 'content-audio' (the audio file of a Mini App `truyen-thanh` item,
-- service-comms 0012 `noi_dung_mini_app.audio_file_id`): the purpose CHECK gains it, and its policy is
-- seeded — 30 MB per file, MP3 (audio/mpeg) or M4A (audio/mp4), ONE file per item.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008 / 0009: both have been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY THESE VALUES: project owner's decision of 01/10/2026 (Mini App content, part A). "30 MB" is read
-- as 31457280 bytes (30 MiB) — the reading 0008, 0010 and 0012 already use for every "N MB" policy;
-- max_bytes is BYTES (0008, column comment). `audio/mp4` is the IANA type of an .m4a file (RFC 4337);
-- `audio/x-m4a` is a non-standard client spelling and is not the sniffed type. max_files_per_subject
-- = 1: one broadcast file per item, so a replacement is a new upload and the old row is soft-deleted,
-- never a second live audio on one item.
--
-- WHY ONE FILE FOR BOTH STEPS (0009 and 0010 were two): the CHECK and the seed were decided together,
-- in one sitting, and neither is useful alone — a widened CHECK with no row refuses every upload, a
-- row cannot be written without the widened CHECK. One transaction leaves either both or neither.
--
-- ⚠ THE GO SIDE MUST MOVE IN THE SAME RELEASE, AND THIS FILE ALONE TURNS internal/store TESTS RED
-- UNTIL IT DOES — by design (0009:11-13): core/storage must hold PurposeContentAudio and the two MIME
-- constants with their sniffers (core/storage/mime.go: audio/mpeg → mp3, audio/mp4 → m4a; today the
-- `M4A ` brand is refused by omission at mime.go:59-60), and platform.proto must name
-- UPLOAD_PURPOSE_CONTENT_AUDIO. A policy the storage layer cannot sniff is a policy whose every
-- upload is refused at step (c) — discovered by a commune.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      This file writes ONE upload_policy row and ONE platform_audit_log entry, platform-wide; the
--      policy applies to every commune. Re-adding the CHECK scans the table once (eight rows).
--   2. IF IT STOPS HALF-WAY: it cannot. One ALTER TABLE (drop and add as two subcommands of one
--      statement), one statement writing the row and its trail entry, and the backstop — in one
--      transaction with the runner's progress row. A failed run leaves the seven-value CHECK and no
--      row. A re-run is free: DROP ... IF EXISTS re-adds the same CHECK; ON CONFLICT (purpose) DO
--      NOTHING returns no row, so the trail gets no second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. ACCESS EXCLUSIVE on
--      upload_policy for the ALTER, so ListUploadPolicies waits milliseconds rather than reading a
--      half-changed table; the CHECK only WIDENS (every row valid before is valid after). Before commit
--      no 'content-audio' policy exists (uploads refused — fail closed); after commit the row exists.
--      Cached readers (core/platformclient/uploadpolicy) see it when their cache expires: until then
--      an audio upload is refused and can be retried — the safe direction.
--   5. RETENTION: nothing is removed or rewritten. The trail entry is append-only (0008 triggers); the
--      row is soft delete only.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-attachment', 'content-audio',
        'tenant-logo', 'petition-photo', 'document-scan', 'task-attachment'));

-- ---------------------------------------------------------------------------
-- SEED — 0008's shape, for 0008's reasons: MIME spellings are the core/storage constants, checked
-- against storage.ExtForMIME by internal/store/upload_policy_test.go, and the trail entry is built from
-- what the INSERT returned, not from a second typing of the values.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('content-audio',        31457280, ARRAY['audio/mpeg', 'audio/mp4'],                                   1, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0013_upload_policy_content_audio.sql: chủ dự án chốt 01/10/2026 — tệp âm thanh tin Truyền thanh Mini App (30 MB, MP3/M4A, 1 tệp mỗi tin)'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED THE ROW AND NO AUDIO WAS UPLOADED UNDER IT. Check first:
--
--   SELECT updated_by, deleted_at FROM upload_policy WHERE purpose = 'content-audio';
--   SELECT count(*) FROM platform_audit_log WHERE subject = 'content-audio';   -- must be 1 (the seed)
--   -- and in service-comms: SELECT count(*) FROM stored_file WHERE purpose = 'content-audio';  -- 0
--
-- THE ROW CANNOT BE REMOVED, AND IS NOT (0010's reasoning): a hard DELETE is refused by
-- upload_policy_no_hard_delete, the seed entry names it, and that entry is append-only. So the CHECK
-- CANNOT be narrowed back either — the withdrawn row still carries purpose 'content-audio' and the
-- seven-value CHECK would refuse it. The reversal is therefore a WITHDRAWAL, which ListUploadPolicies
-- reads exactly as "no row" (it filters deleted_at IS NULL) — every audio upload refused again.
-- In one transaction:
--
--   WITH withdrawn AS (
--       UPDATE upload_policy
--          SET deleted_at = now(), deleted_by = 'system',
--              delete_reason = 'reversal of 0013_upload_policy_content_audio.sql',
--              updated_at = now(), updated_by = 'system'
--        WHERE purpose = 'content-audio' AND deleted_at IS NULL AND updated_by = 'system'
--       RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
--   )
--   INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
--   SELECT 'system', 'upload_policy.withdrawn', w.purpose,
--          jsonb_build_object('max_bytes', w.max_bytes,
--                             'allowed_mime_types', to_jsonb(w.allowed_mime_types),
--                             'max_files_per_subject', w.max_files_per_subject),
--          NULL, 'reversal of migration 0013_upload_policy_content_audio.sql'
--   FROM withdrawn w;
--   DELETE FROM schema_migration WHERE ten = '0013_upload_policy_content_audio.sql';  -- runner bookkeeping
--
-- LOSSLESS while the three checks above hold: the values survive in the seed entry's `after` and in
-- the withdrawn row itself. RE-APPLYING THIS FILE AFTER A REVERSAL DOES NOTHING (ON CONFLICT on the
-- withdrawn row); restoring the policy is an operator's write with its own trail entry.
--
-- ONCE AN OPERATOR HAS CHANGED THE ROW, OR AUDIO HAS BEEN UPLOADED UNDER IT, it is no longer a
-- reversal: the row holds a limit somebody set, and withdrawing it strands the uploads' replacement
-- path. The user decides. The Go side (core/storage purpose and MIME list) rolls back with it.
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
