-- 0028 — Mini App content: a `video` item carries its DURATION in seconds, typed by staff (owner
-- decision of 11/10/2026; the prototype's video list shows a length on every card). The mirror of
-- 0012's `audio_duration_seconds` for `truyen-thanh`.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006 / 0011 / 0012: core/migrate compares the checksum of every
-- applied file at startup (ErrChecksumLech).
--
-- NOT ALL-OR-NONE WITH `video_url`, UNLIKE 0012's audio pair: a video's link is external (0011, ADR 0047
-- §6 (3)) and exists on rows written before this file with no duration, so tying the two would make
-- those rows invalid. Staff may also leave the duration unknown; the card then shows no length.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NO ROW IS WRITTEN. One NULLABLE column with no default — catalogue-only, no
--      rewrite. Each ADD CONSTRAINT scans the register once to validate; every value it reads in the
--      new column is NULL.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. ADD COLUMN IF NOT EXISTS and
--      constraints behind a pg_constraint lookup pinned to the parent; a retry costs nothing. No
--      backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. internal/store names its columns (no `SELECT *` in
--      service-comms, 0011 question 4), so no read sees the column until the Go card asks for it.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered. `noi_dung_mini_app_bat_bien` is NOT
--      replaced.
--
-- WHY VALIDATED, NOT `NOT VALID`: 0011:332-341 (not portable on a partitioned table; ADD COLUMN already
-- holds ACCESS EXCLUSIVE until COMMIT). Every value validated is NULL.
--
-- THE LOCK: ACCESS EXCLUSIVE on noi_dung_mini_app and its 32 partitions until COMMIT (0012's figure).
-- PG FLOOR: 13, enforced by 0006. Cluster: 16. PERSONAL DATA (rule 3): none.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app.video_duration_seconds — `video` only. Seconds, 1 .. 21600 (6 h): 0012's VENDOR
-- bound against a bogus value, not a customer figure. Raising it is one migration; lowering it after a
-- value was written is not.
--
-- `loai` REMAINS EDITABLE (0006:454): a write that changes an item AWAY from `video` must clear the
-- duration IN THE SAME UPDATE, or it is refused — 0011:346-348's reading, followed.
-- ---------------------------------------------------------------------------
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS video_duration_seconds INT;

DO $$
DECLARE parent oid := 'noi_dung_mini_app'::regclass;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_video_duration_only_for_video') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_video_duration_only_for_video
            CHECK (loai = 'video' OR video_duration_seconds IS NULL);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_video_duration_range') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_video_duration_range
            CHECK (video_duration_seconds IS NULL
                   OR video_duration_seconds BETWEEN 1 AND 21600);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM noi_dung_mini_app WHERE video_duration_seconds IS NOT NULL;
--
-- ZERO → drop constraints noi_dung_mini_app_video_duration_range and
-- noi_dung_mini_app_video_duration_only_for_video, drop the column video_duration_seconds, and remove
-- this file's row from `schema_migration` (ten = '0028_content_item_video_duration.sql').
--
-- NON-ZERO → a commune has entered a video's length; dropping it destroys part of what was shown to
-- residents. Rule 7 stop condition #2 — the user plus a verified backup — and the way back is a NEW
-- migration. The Go code reading the column must be rolled back with the schema.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
