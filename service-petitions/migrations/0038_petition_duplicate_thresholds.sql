-- 0038 — the commune's own suspected-duplicate thresholds (ADR 0087 §6: default 50 m, 7 days, PER-COMMUNE
-- CONFIGURATION, never constants — ADR 0087 stop condition #5). Schema only: no row is written.
--
-- WHY IN petition_settings (0028): that table is this service's one-row-per-commune home for the
-- commune's petition choices, and 0028's header says how it grows — "a NEW COLUMN … with its own NOT NULL
-- DEFAULT equal to its decided default, so a commune that never saved anything keeps meaning 'the decided
-- defaults'". This file is exactly that. The search that reads them (ADR 0087 §6) is this service's, so
-- the read stays local.
--
-- A SEPARATE FILE FROM 0037 so each reverses on its own: the link and its history are archival; these two
-- columns are configuration.
--
-- WHAT A THRESHOLD IS FOR: SUGGESTING candidates to an officer. It never merges anything (ADR 0087 §6).
-- A wrong value shows too many or too few suggestions; it cannot link two petitions.
--
-- ---------------------------------------------------------------------------
-- THE RANGES ARE THE WRITER'S SANITY BOUNDS, NOT A CUSTOMER FIGURE — stated so nobody quotes them as one.
--
--   duplicate_radius_meters  1 … 1000   0 or negative is not a radius; beyond 1 km "the same pothole" has
--                                       stopped meaning anything inside one commune.
--   duplicate_window_days    1 … 90     a calendar-day window of REPORTS, not a processing deadline —
--                                       working hours (ADR 0007) do not apply to "reported within D days".
--
-- Turning the suggestion OFF is not representable (no 0). Nobody asked for it; adding it is a later
-- column, never a magic value here.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one exists (PRIMARY KEY tenant_id). ADD COLUMN … NOT NULL DEFAULT <const>
--      is catalogue-only on PostgreSQL 11+: existing rows READ the default, nothing is rewritten. The
--      NOTICE below reports how many communes have a settings row.
--   2. IF IT STOPS HALF-WAY: one file, one transaction (core/migrate). ADD COLUMN IF NOT EXISTS and
--      constraints guarded by a pg_constraint lookup make a retry cost nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. The only reader, store.PetitionSettingsStore, names
--      `verification_photo_required` and nothing else. 0028's guard (DELETE refused, tenant and creation
--      stamp immutable) is untouched and covers the new columns. THE READER CARD must answer "no row"
--      with 50 and 7 — equal to the DEFAULTs below — exactly as it answers TRUE for the photo switch.
--   5. RETENTION: configuration, not an archival record. History of every change is audit_log (rule 6).
-- ---------------------------------------------------------------------------

ALTER TABLE petition_settings
    ADD COLUMN IF NOT EXISTS duplicate_radius_meters INT NOT NULL DEFAULT 50;
ALTER TABLE petition_settings
    ADD COLUMN IF NOT EXISTS duplicate_window_days   INT NOT NULL DEFAULT 7;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'petition_settings'::regclass
                     AND conname = 'petition_settings_duplicate_radius_range') THEN
        ALTER TABLE petition_settings ADD CONSTRAINT petition_settings_duplicate_radius_range
            CHECK (duplicate_radius_meters BETWEEN 1 AND 1000);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'petition_settings'::regclass
                     AND conname = 'petition_settings_duplicate_window_range') THEN
        ALTER TABLE petition_settings ADD CONSTRAINT petition_settings_duplicate_window_range
            CHECK (duplicate_window_days BETWEEN 1 AND 90);
    END IF;
END $$;

COMMENT ON COLUMN petition_settings.duplicate_radius_meters IS
    'ADR 0087 §6: suspected-duplicate radius in metres, per commune. Default 50 = the decided default; '
    'the reader answers 50 for a commune with no row. Suggestion only, never merges.';
COMMENT ON COLUMN petition_settings.duplicate_window_days IS
    'ADR 0087 §6: suspected-duplicate window in CALENDAR days between reports (goc_dem_han), per commune. '
    'Default 7 = the decided default; the reader answers 7 for no row. Not a processing deadline.';

-- MEASUREMENT (question 1): communes that already saved settings and now read the defaults. Counts only.
DO $$
DECLARE n bigint;
BEGIN
    SELECT count(*) INTO n FROM petition_settings;
    RAISE NOTICE '0038: % commune(s) have a petition_settings row; each now reads radius 50 m, window 7 days', n;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless WHILE EVERY ROW HOLDS THE DEFAULTS:
--
--   SELECT count(*) FROM petition_settings
--    WHERE duplicate_radius_meters <> 50 OR duplicate_window_days <> 7;   -- must be 0
--
-- ZERO → ALTER TABLE petition_settings DROP CONSTRAINT petition_settings_duplicate_window_range;
--        ALTER TABLE petition_settings DROP CONSTRAINT petition_settings_duplicate_radius_range;
--        ALTER TABLE petition_settings DROP COLUMN duplicate_window_days;
--        ALTER TABLE petition_settings DROP COLUMN duplicate_radius_meters;
--        and remove this file's row from `schema_migration`.
-- NON-ZERO → a commune chose its own thresholds; dropping the columns silently returns it to 50 m / 7
-- days. That is the user's call (0028's REVERSAL, same reasoning); audit_log keeps what each chose.
-- ---------------------------------------------------------------------------

-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
-- This file declares no table; repeated so every file in this run ends on the same guarantee.
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
