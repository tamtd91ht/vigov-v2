-- platform — the upload policies for a commune's own identity images (ADR 0069, owner decision of
-- 02/10/2026, kb/10-decisions/0069-nhan-dien-xa-logo-banner.md #4 and #5):
--
--   'tenant-logo'   (core/storage.PurposeTenantLogo) — 0008's seed row is rewritten to
--       2 MiB (2097152), PNG/WebP/JPEG, HEIC removed. The server normalises the logo to a square
--       512px PNG that keeps its transparency; a HEIC is a file that step cannot decode.
--   'tenant-banner' (core/storage.PurposeTenantBanner) — NEW purpose: the web-admin banner, the strip
--       under the topbar on every web-admin page. Same 2 MiB, same three types; normalised to 1600px
--       wide. NOT the Mini App `banner` content of comms (ADR 0067 §5, ADR 0069 #6).
--
-- Neither carries a per-subject count limit (NULL, as 0008 seeded tenant-logo): ADR 0069 decides one
-- CURRENT logo and one CURRENT banner per commune — that is the pointer on ho_so_hien_thi_xa (0017),
-- not a count of uploads; a replaced image stays a row (rule 7) and must not use up a quota.
--
-- "2 MB" is read as 2 MiB (2097152), the reading 0008, 0010, 0012, 0013 and 0015 use; max_bytes is BYTES.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008–0015: all have been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY ONE FILE FOR CHECK, CHANGE AND SEED (0013's reasoning): the widened CHECK and the banner row are
-- useless apart, and the logo change is the same owner decision — one file, one trail of it.
--
-- ⚠ THE GO AND PROTO SIDES MUST MOVE IN THE SAME RELEASE. core/storage must hold
-- PurposeTenantBanner = "tenant-banner" and platform.proto UPLOAD_PURPOSE_TENANT_BANNER, or
-- internal/store/upload_policy_test.go turns red — by design: a row the RPC cannot name is a policy no
-- caller ever receives, so every banner upload is refused at step (a).
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      This file rewrites AT MOST ONE upload_policy row (tenant-logo), inserts ONE (tenant-banner) and
--      appends at most TWO platform_audit_log entries, platform-wide. Re-adding the CHECK scans the
--      table once (ten rows).
--   2. IF IT STOPS HALF-WAY: it cannot. One ALTER TABLE (drop and add as two subcommands of one
--      statement), one UPDATE-with-trail statement, one INSERT-with-trail statement and the backstop,
--      in one transaction with the runner's progress row. A failed run leaves 0015's CHECK, 0008's logo
--      values and no banner row. A re-run is free: the CHECK is re-added identically; the logo row no
--      longer holds 0008's values so the UPDATE matches nothing and RETURNING yields no trail; ON
--      CONFLICT (purpose) DO NOTHING returns no banner row, so no second seed entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none inside the database — rows and
--      entries commit together; the CHECK only WIDENS. OUTSIDE it, readers cache
--      (core/platformclient/uploadpolicy): no service uploads 'tenant-logo' yet (the write path is
--      TASK-3 of ADR 0069), so the narrowed logo limit reaches nobody mid-flight; a banner upload
--      before the cache expires is refused and can be retried — the fail-closed direction.
--   5. RETENTION: nothing is removed. No file was ever uploaded under 'tenant-logo' (no write path
--      existed — ho_so_hien_thi_xa.logo_url is a typed URL, 0006), so narrowing it invalidates no
--      stored object. The previous values survive in the trail entry's `before`; entries are
--      append-only (0008 triggers); policy rows are soft delete only.
--
-- PG 16: no RETURNING OLD (that is PG 18). The previous values come from a self-join on the same
-- table in UPDATE ... FROM (0014's device): the FROM side is read from the statement's snapshot.

ALTER TABLE upload_policy
    DROP CONSTRAINT IF EXISTS upload_policy_purpose_known,
    ADD CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-attachment', 'content-audio',
        'tenant-logo', 'tenant-banner', 'petition-photo', 'document-scan', 'task-attachment',
        'petition-verification-photo', 'petition-log-attachment'));

-- ---------------------------------------------------------------------------
-- tenant-logo — ONLY the untouched 0008 seed row is rewritten (0012/0014's reasoning): live, signed
-- 'system', and holding exactly 0008's three values. A row an operator edited or withdrew is a limit
-- somebody chose and signed; it is left alone and gets no entry (see CHECK AFTER APPLYING).
-- ---------------------------------------------------------------------------
WITH changed AS (
    UPDATE upload_policy AS u
       SET max_bytes          = 2097152,
           allowed_mime_types = ARRAY['image/png', 'image/webp', 'image/jpeg'],
           updated_at         = now(),
           updated_by         = 'system'
      FROM upload_policy AS prev
     WHERE u.purpose = 'tenant-logo'
       AND prev.purpose = u.purpose
       AND u.deleted_at IS NULL
       AND u.updated_by = 'system'
       AND u.max_bytes = 10485760
       AND u.allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic']::text[]
       AND u.max_files_per_subject IS NULL
    RETURNING u.purpose,
              prev.max_bytes             AS before_max_bytes,
              prev.allowed_mime_types    AS before_mime_types,
              prev.max_files_per_subject AS before_max_files,
              u.max_bytes                AS after_max_bytes,
              u.allowed_mime_types       AS after_mime_types,
              u.max_files_per_subject    AS after_max_files
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.changed', c.purpose,
       jsonb_build_object(
           'max_bytes', c.before_max_bytes,
           'allowed_mime_types', to_jsonb(c.before_mime_types),
           'max_files_per_subject', c.before_max_files),
       jsonb_build_object(
           'max_bytes', c.after_max_bytes,
           'allowed_mime_types', to_jsonb(c.after_mime_types),
           'max_files_per_subject', c.after_max_files),
       'migration 0016_upload_policy_tenant_logo_banner.sql: chủ dự án chốt 02/10/2026 (ADR 0069 #4) — logo xã PNG/WebP/JPEG ≤ 2 MB, không HEIC'
FROM changed c;

-- ---------------------------------------------------------------------------
-- tenant-banner — SEED, 0008's shape for 0008's reasons: MIME spellings are the core/storage constants,
-- checked against storage.ExtForMIME by internal/store/upload_policy_test.go, and the trail entry is
-- built from what the INSERT returned, not from a second typing of the values.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('tenant-banner', 2097152, ARRAY['image/png', 'image/webp', 'image/jpeg'], NULL, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0016_upload_policy_tenant_logo_banner.sql: chủ dự án chốt 02/10/2026 (ADR 0069 #5) — banner web-admin PNG/WebP/JPEG ≤ 2 MB'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- CHECK AFTER APPLYING. The logo UPDATE is silent when it matches nothing, by design. To see which way
-- it went in an environment:
--
--   SELECT purpose, max_bytes, allowed_mime_types, max_files_per_subject, updated_by
--     FROM upload_policy WHERE purpose IN ('tenant-logo', 'tenant-banner');
--   SELECT occurred_at, action, subject FROM platform_audit_log
--    WHERE subject IN ('tenant-logo', 'tenant-banner') ORDER BY id;
--
-- An 'upload_policy.changed' entry for tenant-logo naming this file → applied. None → the row had been
-- edited or withdrawn before this file ran; ADR 0069 #4 then has to be applied by an operator,
-- deliberately — never by widening this filter.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED EITHER ROW AND NO FILE WAS UPLOADED UNDER EITHER. Check:
--
--   SELECT purpose, updated_by, deleted_at FROM upload_policy
--    WHERE purpose IN ('tenant-logo', 'tenant-banner');               -- both 'system', both live
--   SELECT purpose, count(*) FROM stored_file
--    WHERE purpose IN ('tenant-logo', 'tenant-banner') GROUP BY 1;     -- none (table of 0017)
--
-- Then, in one transaction:
--   1. tenant-logo: UPDATE back to 0008's values (10485760; jpeg/png/webp/heic in that order) with
--      updated_at = now(), updated_by = 'system', keyed on purpose = 'tenant-logo' AND this file's
--      values AND updated_by = 'system' — never an unfiltered UPDATE — and append one
--      'upload_policy.changed' entry with before = this file's values, after = 0008's;
--   2. tenant-banner: the row CANNOT be removed (upload_policy_no_hard_delete), so the CHECK cannot be
--      narrowed back either. WITHDRAW it — deleted_at = now(), deleted_by = 'system', delete_reason
--      naming the reversal of this file, updated_at/updated_by likewise — keyed on purpose =
--      'tenant-banner' AND deleted_at IS NULL AND updated_by = 'system', and append one
--      'upload_policy.withdrawn' entry (before = the values, after NULL) in the same statement
--      (0013's REVERSAL shows the statement). ListUploadPolicies reads a withdrawn row as "no row";
--   3. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0016_upload_policy_tenant_logo_banner.sql'.
-- The entries this file wrote STAY — the trail is append-only, and a reversal is a second change.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
-- Reverting re-opens HEIC and 10 MiB logos — that undoes the owner's ADR 0069 #4, so the owner decides.
--
-- ONCE AN OPERATOR HAS CHANGED A ROW, OR A FILE HAS BEEN UPLOADED UNDER IT, it is no longer a
-- reversal: the row holds a limit somebody set, and the files back what a commune shows its staff and
-- residents. The user decides. The Go and proto sides roll back with it.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill — nothing to
-- iterate and nothing to resume; every statement is idempotent (question 2).
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
