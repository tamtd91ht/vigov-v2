-- 0004 — "Tắt / Bật lại" on a reworded report sentence (ADR 0079 Q2, owner answers of 08/10/2026), the
-- reporting half. The same change as service-petitions' 0033 §A and service-finance's 0017 §A, in this
-- service's own schema (rule 2, invariant 2).
--
-- THIS FILE REPLACES PART OF A DECISION WRITTEN IN 0003, which cannot be edited (core/migrate,
-- ErrChecksumLech). 0003 carries the user decision of 28/09/2026 (via finance's 0010): reword only,
-- "Khôi phục câu mặc định" and no "Tắt". ADR 0079 Q2 ("Làm đúng prototype") adds "Tắt / Bật lại":
-- switching a reworded sentence off keeps the commune's wording and uses the software's sentence until
-- it is switched back on. Where 0003's comment and ADR 0079 disagree, ADR 0079 and this file win.
--
-- NO COMMUNE-ADDED SENTENCES HERE, deliberately: ADR 0079 Q5a keeps "Báo cáo điều hành" a group of its
-- own and places commune sentences only in "Phản ánh", "Dùng chung" (service-petitions) and "Giải ngân"
-- (service-finance). So `reporting` gets no `custom_system_message` table, and stays a service that owns
-- wording only, never an open-ended commune catalogue (the cost ADR 0024 names in 0003:22-24).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most 38 live overrides per commune, plus reverted ones. ADD COLUMN with a
--      constant DEFAULT is metadata-only on PostgreSQL 11+: no row is rewritten.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with its progress row; every
--      statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS. Nothing is backfilled.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none at landing — every override reads `is_active = true`, which
--      is what it meant before. The meaning changes when the handler card makes the readers honour the
--      flag: an inactive override must resolve to the shipped sentence on the configuration list AND on
--      the exported report and the report notifications that print it. A reader still filtering only on
--      `deleted_at IS NULL` would keep printing a switched-off wording on a report leadership signs.
--   5. RETENTION: nothing removed. This file also refuses hard DELETE on the table, which had no guard.
-- ---------------------------------------------------------------------------

ALTER TABLE system_message_override
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- 0003 relied on the store never issuing DELETE (internal/store/system_message_override.go only
-- UPDATEs) — a promise of the application layer that a psql session never heard (ADR 0013). A hard
-- delete would put a commune back on the software's sentence with no trail.
CREATE OR REPLACE FUNCTION system_message_override_no_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Return to the shipped sentence with "Khôi phục câu mặc định" (soft delete: '
                     'deleted_at, deleted_by, delete_reason) or switch it off (is_active = false). '
                     'Rule 7, invariant 1.';
END $$;

-- Row-level on the partitioned parent, so PostgreSQL clones it onto every partition (PG >= 13).
DROP TRIGGER IF EXISTS system_message_override_no_delete ON system_message_override;
CREATE TRIGGER system_message_override_no_delete
    BEFORE DELETE ON system_message_override
    FOR EACH ROW EXECUTE FUNCTION system_message_override_no_delete();

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- `schema_migration` (core/migrate has no automatic rollback, ADR 0013):
--   DROP TRIGGER system_message_override_no_delete ON system_message_override;
--   DROP FUNCTION system_message_override_no_delete();
--   ALTER TABLE system_message_override DROP COLUMN is_active;
-- The last line is lossless only while
--   SELECT count(*) FROM system_message_override WHERE NOT is_active;   -- must be 0
-- otherwise it silently switches those wordings back ON, on reports — a user decision, not a command.
-- ---------------------------------------------------------------------------
