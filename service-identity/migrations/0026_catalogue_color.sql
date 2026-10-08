-- 0026 — IDENTITY CATALOGUES: an optional display colour per row (ADR 0079 #5, "màu mục danh mục";
-- Q1 #9: editable on "Hệ thống" rows too — it is presentation only; spec Cấu hình 05/12
-- `LookupValue.color?`; owner answers of 08/10/2026). One additive, nullable column on each of 0005's
-- two catalogues: `loai_don_vi_dan_cu` (entity ResidentialUnitType) and `khoi_nhiem_vu` (TaskBloc).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0005 is not edited.
--
-- WHY BOTH TABLES IN ONE FILE: same decision, same service, same shape. One transaction means the
-- catalogue screen never sees one identity catalogue with a colour column and the other without.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): `color`, a NEW column, on tables whose existing Vietnamese
-- names stay (rule 12 invariant 3). `color` is the spec's own field name, and the name the sibling
-- migrations use (service-comms 0020, service-documents 0007).
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * NULL = NO COLOUR CHOSEN, and the screen draws its neutral chip. No default: a vendor colour on
--     every existing row would be a choice made for every commune, and would be indistinguishable later
--     from a colour somebody picked.
--   * `#RRGGBB` ONLY — six hex digits, either case. No short `#RGB`, no alpha, no colour names, no
--     `rgb(…)`: one shape is one thing the chip parses, and a CHECK on a closed shape is the floor under
--     a value that is RENDERED (rule 13 #3 — a free-text colour is a string that reaches a style
--     attribute).
--   * CASE IS NOT NORMALISED HERE. Go may lower-case on save; forcing it in the CHECK would refuse a
--     value the owner's own pattern admits.
--   * THE TIER GUARD (0005 `danh_muc_ba_tang`, 0005:157-206) IS UNTOUCHED, verified: it refuses DELETE,
--     a change of `ma` or `nguon`, clearing `ma_nguon_re_nhanh`, soft-deleting a `he-thong` row, and
--     taking a tier-3 row out of use. It names no other column, so `color` is editable on every tier,
--     system and tier-3 rows included — like `nhan` and `thu_tu`, as Q1 #9 decided. No later migration
--     redefines the function.
--
-- NOT IN THIS FILE: per-kind SLA rows (ADR 0079 Q4, "Theo prototype"). The schema already admits them:
-- `sla.linh_vuc` is free TEXT with no foreign key and only `sla_linh_vuc_khong_rong` (0008:228, NULL or
-- non-blank); `sla_loai_viec_hop_le` (0016:61) admits all four kinds; the unique key
-- (tenant_id, loai_viec, linh_vuc_khoa) (0008:218) is per kind. No CHECK ties a non-NULL `linh_vuc` to
-- `phan-anh`. A document-type code under `van-ban-den` or a priority code under `nhiem-vu` inserts as is.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: two catalogues — a handful of rows each (`thon`/`to-dan-pho`, the task blocs
--      plus a commune's own). NO ROW IS WRITTEN. ADD COLUMN without a default is catalogue-only; each
--      ADD CONSTRAINT scans once and every row is NULL.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction with its progress row (core/migrate).
--      IF NOT EXISTS / pg_constraint lookup, so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. internal/store/danh_muc.go:56 (cotDanhMuc) lists its
--      columns explicitly, and the INSERT in internal/store/danh_muc_ghi.go:105 names its columns;
--      nothing reads or writes `color` until Go asks. Every row is NULL = "no colour", today's truth.
--   5. RETENTION: nothing dropped, retyped or emptied. The colour's history is the audit entry of the
--      catalogue edit that sets it (rule 6), written by Go.
--
-- THE LOCK: ACCESS EXCLUSIVE on both tables and their 32 partitions each, until COMMIT. Small tables.
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13 (0005). Cluster: 16.
-- ---------------------------------------------------------------------------

ALTER TABLE loai_don_vi_dan_cu ADD COLUMN IF NOT EXISTS color TEXT;
ALTER TABLE khoi_nhiem_vu      ADD COLUMN IF NOT EXISTS color TEXT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'loai_don_vi_dan_cu'::regclass
                   AND conname = 'loai_don_vi_dan_cu_color_hex') THEN
        ALTER TABLE loai_don_vi_dan_cu ADD CONSTRAINT loai_don_vi_dan_cu_color_hex
            CHECK (color IS NULL OR color ~ '^#[0-9A-Fa-f]{6}$');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'khoi_nhiem_vu'::regclass
                   AND conname = 'khoi_nhiem_vu_color_hex') THEN
        ALTER TABLE khoi_nhiem_vu ADD CONSTRAINT khoi_nhiem_vu_color_hex
            CHECK (color IS NULL OR color ~ '^#[0-9A-Fa-f]{6}$');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no row carries a colour (count of rows with color IS NOT NULL is zero, in BOTH
-- tables): drop the constraints loai_don_vi_dan_cu_color_hex and khoi_nhiem_vu_color_hex, then the two
-- columns, then remove this file's row from `schema_migration` (ten = '0026_catalogue_color.sql').
--
-- ONCE A COMMUNE HAS CHOSEN A COLOUR, dropping the column destroys a recorded choice by a named official —
-- rule 7 stop condition #2: the user, a verified backup, and a NEW migration.
-- ---------------------------------------------------------------------------
