-- platform — upload purpose 'staff-avatar' (core/storage.PurposeStaffAvatar, platform.proto
-- UPLOAD_PURPOSE_STAFF_AVATAR = 13): a staff member's profile photo, Danh bạ cán bộ §5 "Ảnh đại diện"
-- (docs/ui-ux/12-danh-ba-can-bo.md:77). Owner: identity (it owns nguoi_dung). PERSONAL DATA (Decree 13):
-- private original, public derivative only while the person is published with consent — see the
-- comment on PurposeStaffAvatar in core/storage/key.go.
--
-- VALUES SET BY PRECEDENT, 04/10/2026, NOT BY AN OWNER DECISION ON THIS PURPOSE: the spec gives no
-- limit for the photo. The closest decided purpose is the commune logo (ADR 0069 #4, 0016): a small
-- square picture normalised server-side — 2 MiB, PNG/WebP/JPEG, no HEIC (the normaliser cannot decode
-- it), no per-subject count (one CURRENT photo per person is a pointer, not a quota; a replaced photo
-- stays a row under rule 7). The owner may change it; an operator edits the row, never this file.
--
-- WHY A NEW FILE: 0008–0018 are applied and core/migrate checks every applied file's checksum.
-- WHY CHECK AND SEED IN ONE FILE (0013's reasoning): the widened CHECK and the row are useless apart.
--
-- ⚠ THE GO AND PROTO SIDES MOVE IN THE SAME COMMIT (ADR 0052 §10): core/storage PurposeStaffAvatar +
-- platform.proto UPLOAD_PURPOSE_STAFF_AVATAR, or internal/store/upload_policy_test.go turns red.
--
-- THE FIVE MIGRATION QUESTIONS:
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      One row inserted, one trail entry appended, platform-wide.
--   2. IF IT STOPS HALF-WAY: it cannot — one ALTER, one INSERT-with-trail statement and the backstop,
--      in one transaction with the runner's progress row. A re-run is free: the CHECK is re-added
--      identically; ON CONFLICT (purpose) DO NOTHING returns no row, so no second trail entry.
--   3. HOW IT IS REVERSED: withdraw the row (soft delete — upload_policy_no_hard_delete forbids
--      DELETE, so the CHECK cannot be narrowed back), keyed on purpose = 'staff-avatar' AND
--      deleted_at IS NULL AND updated_by = 'system', with one 'upload_policy.withdrawn' entry in the
--      same statement (0013's REVERSAL shows it), then remove this file's progress row keyed on its
--      name. Only while no file was uploaded under it. Written as prose: a runnable line gets run.
--   4. WHICH READ PATHS CHANGE MEANING WHILE HALF-APPLIED: none — the CHECK only widens; before this
--      file no 'staff-avatar' policy exists and uploads are refused (fail closed).
--   5. RETENTION: nothing removed or rewritten; the trail is append-only, the row soft delete only.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-body-image', 'content-attachment', 'content-audio',
        'tenant-logo', 'tenant-banner', 'petition-photo', 'document-scan', 'task-attachment',
        'petition-verification-photo', 'petition-log-attachment', 'staff-avatar'));

WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('staff-avatar', 2097152, ARRAY['image/png', 'image/webp', 'image/jpeg'], NULL, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0019_upload_policy_staff_avatar.sql: ảnh đại diện cán bộ (Danh bạ cán bộ §5) — giá trị theo tiền lệ logo xã (ADR 0069 #4): PNG/WebP/JPEG ≤ 2 MB, không giới hạn số tệp'
FROM seeded s;

-- BACKSTOP, carried forward from 0002: a partitioned table with no partitions rejects every INSERT.
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
