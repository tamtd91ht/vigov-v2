-- 0020 — MAP ASSET TYPE CATALOGUE: an optional display colour per row (ADR 0079 #5, "màu mục danh mục";
-- spec Cấu hình 05/12 `LookupValue.color?`; owner answer D, 08/10/2026). One additive, nullable column on
-- 0003's `loai_tai_nguyen_ban_do` (entity MapAssetType).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0003 is not edited.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): `color`, a NEW column, on a table whose existing Vietnamese
-- names stay (rule 12 invariant 3 — existing names are not renamed). `color` is the spec's own field name
-- (spec 12, `LookupValue.color?`).
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * NULL = NO COLOUR CHOSEN, and the screen draws its neutral chip. No default: a vendor colour on
--     every existing row would be a choice made for every commune, and would be indistinguishable later
--     from a colour somebody picked.
--   * `#RRGGBB` ONLY — six hex digits, either case, as the owner wrote it. No short `#RGB`, no alpha, no
--     colour names, no `rgb(…)`: one shape is one thing the map renderer and the chip both parse, and a
--     CHECK on a closed shape is the floor under a value that is RENDERED (rule 13 #3 — a free-text
--     colour is a string that reaches a style attribute).
--   * CASE IS NOT NORMALISED HERE. '#1A2B3C' and '#1a2b3c' are the same colour; Go may lower-case on
--     save. Forcing it in the CHECK would refuse a value the owner's own pattern admits.
--   * THE TIER GUARD (0003 `danh_muc_ba_tang`) IS UNTOUCHED. It freezes `ma` and `nguon` and protects
--     tier-3 rows from being disabled; a colour is presentation, editable on every tier, system rows
--     included — like `nhan` and `thu_tu`.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: the catalogue — about a dozen per commune. NO ROW IS WRITTEN. ADD COLUMN without
--      a default is catalogue-only; the ADD CONSTRAINT scans once and every row is NULL.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction; IF NOT EXISTS / pg_constraint lookup,
--      so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. internal/store/loai_tai_nguyen_ban_do.go lists its columns
--      explicitly (cotLoaiTaiNguyen); nothing reads `color` until Go asks. Every row is NULL = "no
--      colour", which is today's truth.
--   5. RETENTION: nothing dropped, retyped or emptied. The colour's history is the audit entry of the
--      catalogue edit that sets it (rule 6), written by Go.
--
-- THE LOCK: ACCESS EXCLUSIVE on loai_tai_nguyen_ban_do and its 32 partitions until COMMIT. Small table.
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13 (0003). Cluster: 16.
-- ---------------------------------------------------------------------------

ALTER TABLE loai_tai_nguyen_ban_do ADD COLUMN IF NOT EXISTS color TEXT;

DO $$
DECLARE parent oid := 'loai_tai_nguyen_ban_do'::regclass;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'loai_tai_nguyen_ban_do_color_hex') THEN
        ALTER TABLE loai_tai_nguyen_ban_do ADD CONSTRAINT loai_tai_nguyen_ban_do_color_hex
            CHECK (color IS NULL OR color ~ '^#[0-9A-Fa-f]{6}$');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no row carries a colour (count of rows with color IS NOT NULL is zero): drop the
-- constraint loai_tai_nguyen_ban_do_color_hex, then the column, then remove this file's row from
-- `schema_migration` (ten = '0020_map_asset_type_color.sql').
--
-- ONCE A COMMUNE HAS CHOSEN A COLOUR, dropping the column destroys a recorded choice by a named official —
-- rule 7 stop condition #2: the user, a verified backup, and a NEW migration.
-- ---------------------------------------------------------------------------
