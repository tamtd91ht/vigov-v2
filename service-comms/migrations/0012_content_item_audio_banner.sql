-- 0012 — Mini App content, part A (project owner's decisions of 01/10/2026): a category can be HIDDEN,
-- a `truyen-thanh` item carries an uploaded AUDIO file and its duration, and a `banner` item carries a
-- tap target, a display order and a mandatory cover image.
--
-- WHY THIS FILE EXISTS. 0011:14-16 added the event and video columns of §7's closing note and said,
-- in so many words, that audio and the banner columns were NOT decided and NOT added. The owner
-- decided both on 01/10/2026 (the ADR recording it is being written alongside this file). 0011's
-- reason for leaving them out therefore no longer holds, and 0011's own test
-- (content_item_media_test.go, the `audio` ban) keeps holding for 0011 — this is a different file.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006 / 0011: core/migrate compares the checksum of every applied
-- file at startup (ErrChecksumLech).
--
-- WHY THIS FILE AND 0013 ARE TWO FILES: they reverse under different conditions. Everything here is a
-- column on an existing table with an inert-until-written CHECK; 0013 creates three tables, one of
-- which holds a sealed credential for a government portal. Rolling back the portal sync must not
-- require rolling back the audio and banner fields, nor the reverse.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NO ROW IS WRITTEN. `danh_muc_mini_app` gains one `BOOLEAN NOT NULL DEFAULT
--      false` column — a constant default, which PostgreSQL 11+ records in the catalogue WITHOUT
--      rewriting a row (§3's figure: ~60 categories per commune). `noi_dung_mini_app` gains four
--      NULLABLE columns with no default — catalogue-only, no rewrite. Each ADD CONSTRAINT scans the
--      register once to validate; every value it reads in the new columns is NULL (or false).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS or a
--      constraint added behind a pg_constraint lookup pinned to the parent, so a retry costs nothing.
--      No backfill, so there is nothing to resume per commune (rule 7, invariant 5).
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom — exact SQL and the condition under which it
--      loses nothing.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER it, before the Go change: internal/store names its columns explicitly (no
--      `SELECT *` in service-comms, 0011 question 4), so no read path sees a new column until it asks.
--        * `hidden` is false on EVERY existing row, i.e. "shown" — today's behaviour exactly. It is
--          NOT NULL, so there is no third state: a reader writing `NOT hidden` and one writing
--          `hidden = false` agree on every row, including rows that predate this file. (Agent rule 6,
--          the `$ne: true` trap, cannot occur on a NOT NULL column with a default; it is named here
--          so nobody makes the column nullable later "to save a rewrite".)
--        * the BANNER COVER RULE is a trigger, not a CHECK, precisely so that a `banner` row that
--          already exists without a cover does NOT become invalid — see the note above the trigger.
--          The public banner strip (not built) must still skip a banner with no cover.
--        * the new index changes no result, only plans.
--   5. RETENTION: a published article is business data (0006 question 5). Nothing is dropped,
--      retyped, emptied or renumbered. The 0006/0011 immutability function
--      (`noi_dung_mini_app_bat_bien`) is NOT replaced — every rule in it stands as 0011 left it.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on a partitioned table), already enforced by 0006.
-- The real cluster is 16. Nothing here needs more.
--
-- THE LOCK: ADD COLUMN / ADD CONSTRAINT take ACCESS EXCLUSIVE on both tables and their 32 partitions
-- until COMMIT; CREATE INDEX on the parent takes SHARE. Acceptable at today's size (0006 question 1).
-- AN ASSUMPTION, NOT A MEASUREMENT — check first:
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('noi_dung_mini_app')) FROM noi_dung_mini_app;
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- danh_muc_mini_app.hidden — the commune takes a category off the Mini App without deleting it.
--
-- A FLAG AND NOT A SOFT DELETE: a deleted category is gone from the staff screen too, and its slug is
-- held for ever (0006:184-188). A hidden one is still the commune's filing — staff keep using it — it
-- is only not shown to residents. Whether hiding a category also hides the ITEMS filed under it is
-- the read path's rule, not this column's; the column states one fact about the category.
-- ---------------------------------------------------------------------------
ALTER TABLE danh_muc_mini_app ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app — FOUR NEW COLUMNS. All NULLABLE, no default, English names (rule 12).
--
--   audio_file_id           `truyen-thanh` only. → stored_file (tenant_id, id), exactly as
--                           cover_image_file_id (0011:353-359). The audio is uploaded into THIS
--                           service's object storage under purpose `content-audio` (platform 0013
--                           seeds its limit: 30 MiB, MP3/M4A, one file per item).
--   audio_duration_seconds  `truyen-thanh` only; set together with the file (all or none). Seconds,
--                           1 .. 21600 (6 h). 6 h is a VENDOR bound against a bogus value, not a
--                           customer figure: a 30 MiB file at the lowest bitrate a broadcast is
--                           realistically encoded at is well under that. Raising it is one migration;
--                           lowering it after a value was written is not.
--   link_to                 `banner` only. NULL = the banner is not tappable. Either an IN-APP PATH
--                           ('/…') or an absolute https:// URL — see the CHECK for what is refused.
--   display_order           `banner` only. Position in the banner strip, ascending, ≥ 0. NULL is
--                           allowed and means "after every ordered banner" (the read sorts NULLS
--                           LAST) — required would invalidate every banner row that exists today.
--
-- WHAT STORED_FILE NEEDS FOR AUDIO: NOTHING, after reading 0011:63-67, 134-142. Its CHECKs are:
--   * subject_type IN ('content-item')  — an audio file belongs to a content item: unchanged.
--   * retention_class IN ('content-source') — MEANS "the source file of Mini App content" (core/storage
--     ClassContentSource), not "an image". The broadcast's original upload is exactly that. A new class
--     would be a new retention rule, which ADR 0052 Còn mở #1 leaves to the customer.
--   * bucket IN ('private') — the original is always private (ADR 0052 §2): unchanged for audio.
--   * purpose is checked by SHAPE only (`^[a-z0-9]+(-[a-z0-9]+)*$`), so 'content-audio' is accepted
--     without a migration, by design (0011:138-140).
-- So stored_file is NOT altered here.
--
-- ⚠ OWED BY GO, NOT BY THIS FILE: a PUBLIC copy of the audio needs a `public_object_key` whose variant
-- is not `original` (0011:156-161), and core/storage's closed variant list (key.go, variantPattern)
-- has no audio variant today. Until one exists the audio can be stored but not published through the
-- public bucket.
-- ---------------------------------------------------------------------------
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS audio_file_id          TEXT;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS audio_duration_seconds INT;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS link_to                TEXT;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS display_order          INT;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS — VALIDATED, not `NOT VALID`, for 0011:332-341's three reasons (NOT VALID on a
-- partitioned table is not portable; a FK NOT VALID on one is refused before PG 18; no lock to save
-- because ADD COLUMN already holds ACCESS EXCLUSIVE until COMMIT). Every value validated is NULL.
--
-- `loai` REMAINS EDITABLE (0006:454), and 0011:346-348's reading is followed exactly: a write that
-- changes an item AWAY from `truyen-thanh` / `banner` must clear that type's columns IN THE SAME
-- UPDATE, or it is refused. A broadcast file left on a news article is audio a resident can play
-- under a headline it was never recorded for.
-- ---------------------------------------------------------------------------
DO $$
DECLARE parent oid := 'noi_dung_mini_app'::regclass;
BEGIN
    -- SAME SERVICE, SAME SCHEMA, composite with tenant_id: the audio cannot be another commune's file.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_audio_file_fk') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_audio_file_fk
            FOREIGN KEY (tenant_id, audio_file_id) REFERENCES stored_file (tenant_id, id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_audio_only_for_truyen_thanh') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_audio_only_for_truyen_thanh
            CHECK (loai = 'truyen-thanh'
                   OR (audio_file_id IS NULL AND audio_duration_seconds IS NULL));
    END IF;

    -- A file with no duration is a player that cannot draw its bar; a duration with no file is a
    -- length of nothing. An equivalence of two IS NULL tests is TRUE/FALSE, never NULL.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_audio_all_or_none') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_audio_all_or_none
            CHECK ((audio_file_id IS NULL) = (audio_duration_seconds IS NULL));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_audio_duration_range') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_audio_duration_range
            CHECK (audio_duration_seconds IS NULL
                   OR audio_duration_seconds BETWEEN 1 AND 21600);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_banner_fields_only_for_banner') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_banner_fields_only_for_banner
            CHECK (loai = 'banner' OR (link_to IS NULL AND display_order IS NULL));
    END IF;

    -- THE TAP TARGET A RESIDENT FOLLOWS. Two shapes and nothing else, at most 500 characters, no
    -- whitespace, control character or backslash anywhere:
    --   in-app path   '/' followed by anything but a second '/'. `//host/x` is a PROTOCOL-RELATIVE
    --                 URL — it leaves the app for whatever host it names — so it is refused here,
    --                 not merely "looks like a path". Backslash is refused because some WebViews
    --                 normalise `/\host` into `//host`.
    --   external      `https://` and a non-empty host. Plain http, `javascript:`, `data:`, `intent:`
    --                 and `zalo:` never reach a citizen's tap.
    -- The scheme matches case-insensitively (`~*`, RFC 3986); the write path should store it
    -- lower-cased. The Go write path re-checks the parse (url.Parse) — a regular expression is a
    -- floor, not a URL parser.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_link_to_valid') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_link_to_valid
            CHECK (link_to IS NULL
                   OR (char_length(link_to) <= 500
                       AND link_to !~ '[[:space:][:cntrl:]\\]'
                       AND (link_to ~ '^/([^/].*)?$'
                            OR link_to ~* '^https://[^/?#@]+([/?#].*)?$')));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_display_order_non_negative') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_display_order_non_negative
            CHECK (display_order IS NULL OR display_order >= 0);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- The public banner strip: one commune's live banners in display order. Leads with tenant_id (every
-- index does, so one commune's read never walks another's rows). NOT UNIQUE — two banners may share a position (the `id` breaks the
-- tie), and a partial predicate on a non-unique index decides only how many rows are read
-- (tools/check_khoa_duy_nhat.py refuses that shape for UNIQUE indexes only).
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS noi_dung_mini_app_banner_strip
    ON noi_dung_mini_app (tenant_id, loai, display_order, id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app_audio_file_check — the floor under choosing a broadcast's audio, for every
-- writer. 0011's cover check (0011:420-448) with the purpose changed: the file must be in this
-- commune (FK), uploaded FOR THIS ITEM, as `content-audio`, not soft-deleted, and past the scan.
-- Fires only when the column is set or changed; clearing it is always allowed (and the all-or-none
-- CHECK then requires the duration to go with it). The file row is locked FOR SHARE so a concurrent
-- soft delete waits for this transaction.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION noi_dung_mini_app_audio_file_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE file_row RECORD;
BEGIN
    IF NEW.audio_file_id IS NULL
       OR (TG_OP = 'UPDATE' AND NEW.audio_file_id IS NOT DISTINCT FROM OLD.audio_file_id) THEN
        RETURN NEW;
    END IF;

    SELECT f.subject_type, f.subject_id, f.purpose, f.status, f.deleted_at
      INTO file_row
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id AND f.id = NEW.audio_file_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'noi_dung_mini_app: audio file not found in this commune';
    END IF;
    IF file_row.deleted_at IS NOT NULL
       OR file_row.subject_type <> 'content-item'
       OR file_row.subject_id <> NEW.id
       OR file_row.purpose <> 'content-audio' THEN
        RAISE EXCEPTION 'noi_dung_mini_app: audio file was not uploaded as this item''s audio';
    END IF;
    IF file_row.status NOT IN ('stored', 'processing', 'ready') THEN
        RAISE EXCEPTION 'noi_dung_mini_app: audio file status % cannot be used', file_row.status;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS noi_dung_mini_app_audio_file_check ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_audio_file_check
    BEFORE INSERT OR UPDATE OF audio_file_id ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION noi_dung_mini_app_audio_file_check();

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app_banner_cover_required — a banner is a picture; one with no cover draws an empty
-- strip on every resident's home screen.
--
-- A TRIGGER AND NOT `CHECK (loai <> 'banner' OR cover_image_file_id IS NOT NULL)`, DELIBERATELY. A
-- CHECK is validated against EVERY EXISTING ROW when added. `banner` has been an accepted `loai`
-- since 0006 and the staff create route exists, while `cover_image_file_id` only exists since 0011 —
-- so a banner created before 0011's cover upload, or created without one, may exist today, and NO
-- DATABASE IS REACHABLE FROM HERE TO COUNT THEM. Under a CHECK, one such row anywhere makes this file
-- fail at startup and the service refuse to start in that environment; NOT VALID is not a portable
-- way out on a partitioned table (above). Forcing a cover onto those rows would be inventing data.
--
-- WHAT IT REFUSES — the three ways a coverless banner can come INTO being:
--   * INSERT of a `banner` with no cover;
--   * an UPDATE that turns another `loai` INTO `banner` without a cover in the same statement
--     (0011:346's reading of an editable `loai`, followed);
--   * an UPDATE that REMOVES the cover of a `banner`.
-- WHAT IT LETS THROUGH: an edit of a banner that ALREADY had no cover before this file (it neither
-- creates nor removes anything), and its soft delete. Those legacy rows are the read path's to skip.
--
-- Fires on INSERT and on any UPDATE naming `loai` or `cover_image_file_id`; the conditions below are
-- on values, so an UPDATE that lists the column with an unchanged value passes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION noi_dung_mini_app_banner_cover_required() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.loai = 'banner' AND NEW.cover_image_file_id IS NULL
       AND (TG_OP = 'INSERT'
            OR OLD.loai IS DISTINCT FROM 'banner'
            OR OLD.cover_image_file_id IS NOT NULL) THEN
        RAISE EXCEPTION 'noi_dung_mini_app: a banner needs a cover image'
            USING HINT = 'Set cover_image_file_id in the same statement that creates the banner or '
                         'turns an item into one; a banner''s cover can be replaced, not removed.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS noi_dung_mini_app_banner_cover_required ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_banner_cover_required
    BEFORE INSERT OR UPDATE OF loai, cover_image_file_id ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION noi_dung_mini_app_banner_cover_required();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, DDL by the table owner (0006, ADR 0013); the
-- PURGE of a file that is still a live broadcast's audio (the purge worker owes that cross-table
-- check, as 0011:527-530 says for covers); and a legacy coverless banner (above).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013), and there is no down file because core/migrate would apply it as the next
-- migration (0011:532-535).
--
-- LOSSLESS ONLY WHILE ALL OF THESE READ ZERO:
--
--   SELECT count(*) FROM danh_muc_mini_app WHERE hidden;
--   SELECT count(*) FROM noi_dung_mini_app
--    WHERE audio_file_id IS NOT NULL OR audio_duration_seconds IS NOT NULL
--       OR link_to IS NOT NULL OR display_order IS NOT NULL;
--
-- ZERO → in this order:
--
--   DROP TRIGGER noi_dung_mini_app_banner_cover_required ON noi_dung_mini_app;
--   DROP FUNCTION noi_dung_mini_app_banner_cover_required();
--   DROP TRIGGER noi_dung_mini_app_audio_file_check ON noi_dung_mini_app;
--   DROP FUNCTION noi_dung_mini_app_audio_file_check();
--   DROP INDEX noi_dung_mini_app_banner_strip;
--   ALTER TABLE noi_dung_mini_app
--       DROP CONSTRAINT noi_dung_mini_app_display_order_non_negative,
--       DROP CONSTRAINT noi_dung_mini_app_link_to_valid,
--       DROP CONSTRAINT noi_dung_mini_app_banner_fields_only_for_banner,
--       DROP CONSTRAINT noi_dung_mini_app_audio_duration_range,
--       DROP CONSTRAINT noi_dung_mini_app_audio_all_or_none,
--       DROP CONSTRAINT noi_dung_mini_app_audio_only_for_truyen_thanh,
--       DROP CONSTRAINT noi_dung_mini_app_audio_file_fk,
--       DROP COLUMN display_order, DROP COLUMN link_to,
--       DROP COLUMN audio_duration_seconds, DROP COLUMN audio_file_id;
--   ALTER TABLE danh_muc_mini_app DROP COLUMN hidden;
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- The `stored_file` rows of uploaded audio (purpose 'content-audio') are NOT touched by the reversal
-- and must not be: they index objects in MinIO. A non-zero count of them with zero items pointing at
-- them is still lossless for THIS file's columns.
--
-- NON-ZERO → a commune has hidden a category, attached a broadcast, or set a banner's link or order.
-- Dropping it destroys part of what a public authority showed its residents. That is rule 7 stop
-- condition #2 — the user plus a verified backup, never a command — and the way back is a NEW
-- migration. The Go code reading these columns must be rolled back with the schema.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP) —
-- repeated because it only verifies the state after a file that carries it.
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
