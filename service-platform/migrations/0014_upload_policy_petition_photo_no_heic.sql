-- platform — the upload policy for purpose 'petition-photo' (core/storage.PurposePetitionPhoto, the
-- scene photograph a citizen attaches to a petition, menu phan-anh-nguoi-dan): JPEG/PNG/WebP only,
-- HEIC removed. 10 MiB per file and 5 files per petition are kept as 0008 seeded them.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008: 0008 has been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY THESE VALUES: owner decision G3 (kb/10-decisions/0047-hai-luong-dung-citizen-app-theo-ten-mien.md:255,
-- 30/09/2026, re-confirmed 02/10/2026 "theo G3"): the server decodes and re-encodes every scene photo
-- to strip EXIF, and accepts "chỉ JPEG/PNG/WebP … không nhận HEIC". A HEIC the policy admits is a file
-- the re-encode step cannot produce a clean copy of. G3 says nothing about size or count, so
-- max_bytes (10485760) and max_files_per_subject (5) are not touched.
--
-- WHY ONLY THE UNTOUCHED SEED ROW IS REWRITTEN (0012's reasoning): upload_policy has no commune
-- column — there is ONE 'petition-photo' row, platform-wide. No operator write path exists yet (ADR
-- 0048 steps 1–3), so in every environment today that row still holds exactly 0008's values, signed
-- 'system'. The UPDATE nevertheless matches ONLY that state: live (deleted_at IS NULL), updated_by =
-- 'system', and max_bytes / allowed_mime_types / count equal to 0008's seed. A row an operator has
-- edited, or withdrawn, is a limit somebody chose and signed, and a migration must not overwrite it —
-- such a row is left alone, and this file writes no trail entry for it (see CHECK AFTER APPLYING).
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — upload_policy and platform_audit_log have no commune column.
--      This file rewrites AT MOST ONE upload_policy row and appends at most ONE platform_audit_log
--      entry, platform-wide; the policy applies to every commune.
--   2. IF IT STOPS HALF-WAY: it cannot. One statement rewrites the row and writes its trail entry, in
--      one transaction with the runner's progress row. A failed run leaves the 0008 values and no
--      entry. A re-run is free: the row then no longer holds 0008's MIME list, the UPDATE matches
--      nothing, and RETURNING yields no row, so the trail gets no second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none inside the database — the row
--      and its entry commit together. OUTSIDE it, readers cache: a core/platformclient/uploadpolicy
--      caller (none reads 'petition-photo' yet as of this file; the petition upload path will) keeps
--      admitting HEIC until its cache expires. Both directions of
--      that window are safe for records: an upload accepted under the old policy was valid when
--      accepted; one refused under the new one can be retried as JPEG. The ONLY visible effect is that
--      a HEIC scene photo accepted in that window is a file the new policy would refuse — already
--      stored, never re-validated, and not removed by this file.
--   5. RETENTION: nothing is removed. Scene photos are petition evidence (archival); every file
--      already uploaded, HEIC included, is untouched — this narrows FUTURE uploads only. The previous
--      values survive in the trail entry's `before`; the entry is append-only (0008 triggers).
--
-- PG 16: no RETURNING OLD (that is PG 18). The previous values come from a self-join on the same
-- table in UPDATE ... FROM: the FROM side is read from the statement's snapshot, so `prev` holds the
-- values as they were before this statement wrote.

WITH changed AS (
    UPDATE upload_policy AS u
       SET allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp'],
           updated_at         = now(),
           updated_by         = 'system'
      FROM upload_policy AS prev
     WHERE u.purpose = 'petition-photo'
       AND prev.purpose = u.purpose
       AND u.deleted_at IS NULL
       AND u.updated_by = 'system'
       AND u.max_bytes = 10485760
       AND u.allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic']::text[]
       AND u.max_files_per_subject = 5
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
       'migration 0014_upload_policy_petition_photo_no_heic.sql: chủ dự án chốt G3 (ADR 0047, 30/09/2026, nhắc lại 02/10/2026) — ảnh hiện trường chỉ nhận JPEG/PNG/WebP, không nhận HEIC; giữ 10 MB, 5 ảnh'
FROM changed c;

-- ---------------------------------------------------------------------------
-- CHECK AFTER APPLYING. The statement above is silent when it matches nothing, by design (an edited
-- row is not overwritten). To see which way it went in an environment:
--
--   SELECT max_bytes, allowed_mime_types, max_files_per_subject, updated_by
--     FROM upload_policy WHERE purpose = 'petition-photo';
--   SELECT occurred_at, action FROM platform_audit_log WHERE subject = 'petition-photo' ORDER BY id;
--
-- An 'upload_policy.changed' entry naming this file → applied. No such entry → the row had been
-- edited or withdrawn before this file ran; G3 then has to be applied by an operator, deliberately,
-- through the operator path — never by widening this filter.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO OPERATOR HAS EDITED THE ROW SINCE. Check first: the row still reads
-- updated_by = 'system', max_bytes = 10485760, max_files_per_subject = 5, allowed_mime_types =
-- {image/jpeg,image/png,image/webp}, and the LAST platform_audit_log entry for subject =
-- 'petition-photo' is the 'upload_policy.changed' one this file wrote. Then, in one transaction:
--   1. UPDATE the row back to 0008's MIME list — jpeg/png/webp/heic in that order — with
--      updated_at = now(), updated_by = 'system'; keyed on purpose = 'petition-photo' AND the four
--      post-0014 values above, never an unfiltered UPDATE;
--   2. append one platform_audit_log entry: actor 'system', action 'upload_policy.changed', subject
--      'petition-photo', before = this file's values, after = 0008's, reason naming the reversal of
--      this file. The entry this file wrote STAYS — the trail is append-only, and a reversal is a
--      second change, not an erasure of the first;
--   3. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0014_upload_policy_petition_photo_no_heic.sql'.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
-- Reverting re-opens HEIC scene photos — that undoes the owner's G3 decision, so the owner decides.
--
-- ONCE AN OPERATOR HAS CHANGED THE ROW it is no longer a reversal: the row holds a limit somebody
-- set and platform_audit_log holds who set it. The owner decides.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column and one platform-wide row —
-- nothing to iterate and nothing to resume; the statement is idempotent (question 2).
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
