-- 0044 — whether the petition's SCENE COORDINATES were placed/adjusted by the citizen (owner decision
-- 11/10/2026). Schema only: no row is written, no existing value changes.
--
--   location_citizen_adjusted   true  = the citizen dragged the map pin in the commune Mini App, so
--                                       `lat`/`lng` (0004) are where the CITIZEN put them
--                               false = `lat`/`lng` were taken as-is from Zalo's location (or the
--                                       petition has no coordinates / was not placed by a citizen)
--                               NULL  = NOT RECORDED: every petition written before the intake path
--                                       started setting this flag
--
-- WHY IT MATTERS: a pin the citizen moved is a statement by the citizen, not a device reading. Staff
-- deciding where to send a crew, and the merge proximity search (0037), weigh the two differently.
--
-- NULLABLE, NO DEFAULT — DELIBERATELY, although `NOT NULL DEFAULT false` would also be catalogue-only
-- on PG 16 (a constant default is stored as each partition's missing value, no rewrite). The cost is
-- not the lock, it is the MEANING: `false` on an existing archival petition would assert "Zalo's
-- location, untouched" about rows for which nobody recorded that — the staff-booked channel and older
-- Mini App builds included. An invented fact on an archival record is worse than an absent one (0043
-- made the same call for its times).
--
-- NO BACKFILL. Nothing on disk says how an old petition's coordinates were obtained.
--
-- NAMES (rule 12, ADR 0051): English, on a Vietnamese-named table whose existing columns are not renamed
-- (precedent 0017 `rating_comment`, 0032 `zalo_account_id`, 0037 `merged_at`, 0043).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- PG 16 ON A PARTITIONED TABLE (the real cluster, ADR 0010): ADD COLUMN on the partitioned parent
-- propagates to all 32 partitions; a nullable column with no default is catalogue-only, no rewrite.
-- ADD CONSTRAINT … CHECK on the parent is inherited by every partition and validated there. NOT VALID is
-- avoided — not portable on a partitioned table (comms 0011:332-341); every value validated is NULL.
--
-- THE LOCK: ACCESS EXCLUSIVE on phieu_phan_anh and its 32 partitions until COMMIT; the CHECK scans the
-- register once and passes vacuously. The register is small — AN ASSUMPTION, NOT A MEASUREMENT; the
-- operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Every petition gains one NULL column without a rewrite.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. ADD COLUMN IF NOT EXISTS and the constraint behind a pg_constraint lookup bound
--      to the table's oid make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none can see it half-applied. AFTER it, before the Go card: every
--      read of phieu_phan_anh names its columns (store `cotPhieu`, no `SELECT *`), so no reader sees it.
--      Every existing write passes: the INSERT does not name it (NULL), no UPDATE names it. NOTE: rows
--      created in that gap are NULL too, so NULL means "not recorded", NOT "before migration 0044".
--      ONCE THE GO CARD WRITES IT: any reader must treat NULL as "unknown" — never fold it into `false`
--      (that is the `= false` trap of the soft-delete flag, inverted: old rows would silently read as
--      "Zalo's location, untouched"). A figure computed from it must say it covers petitions from the
--      Go card on.
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped or renumbered;
--      `lat`/`lng`, `ma_tra_cuu` and 0004's guard are untouched. The flag is fixed at intake, with the
--      coordinates it describes.
--
-- PERSONAL DATA (rule 3): none — a boolean. It says HOW the coordinates were placed, not where.
-- ---------------------------------------------------------------------------

ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS location_citizen_adjusted BOOLEAN;

DO $$ BEGIN
    -- "The citizen adjusted the location" with no location is a claim about nothing. Both coordinates,
    -- because 0004 stores them as two columns and the domain only ever writes them together
    -- (domain.NormaliseSceneLocation). NULL and false pass.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_location_adjusted_has_coordinates') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_location_adjusted_has_coordinates
            CHECK (location_citizen_adjusted IS NOT TRUE OR (lat IS NOT NULL AND lng IS NOT NULL));
    END IF;
END $$;

COMMENT ON COLUMN phieu_phan_anh.location_citizen_adjusted IS
    'true = nguoi dan tu dat/keo ghim vi tri tren Mini App xa (chu du an chot 11/10/2026); false = lat/lng '
    'lay nguyen tu vi tri Zalo hoac khong do nguoi dan dat; NULL = khong ghi nhan (phieu truoc khi luong '
    'tiep nhan ghi co nay, khong dien lai). Chi true khi co lat va lng.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM phieu_phan_anh WHERE location_citizen_adjusted IS NOT NULL;
--
-- ZERO → drop constraint phieu_phan_anh_location_adjusted_has_coordinates, drop column
-- location_citizen_adjusted, and remove this file's row from `schema_migration`
-- (ten = '0044_petition_location_citizen_adjusted.sql').
--
-- NON-ZERO → the provenance of coordinates on archival petitions. Dropping it is rule 7 stop
-- condition #2 — the user plus a verified backup — and the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
