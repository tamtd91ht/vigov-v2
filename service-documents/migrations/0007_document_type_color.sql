-- 0007 — colour on the document-type catalogue `loai_van_ban` (ADR 0079 row 5 "màu mục danh mục";
-- Q1 #9: editable on "Hệ thống" rows too — it is presentation only). Owner answers of 08/10/2026.
--
-- NULL = no colour chosen; the screen picks its neutral chip. `#RRGGBB` only — no names, no alpha, no
-- `rgb()`: one spelling, and nothing a renderer could read as CSS beyond a colour (rule 13, invariant
-- 3). The app should store it lower-cased so one colour has one spelling; the CHECK accepts either case.
--
-- danh_muc_ba_tang (0003) needs no change: it guards `ma`, `nguon`, the tier flag, soft delete and
-- disabling. `color` is none of those, so it is editable on every tier, as Q1 #9 decided.
--
-- NOT IN THIS FILE: per-document-type SLA rows (ADR 0079 Q4). The `sla` table belongs to
-- service-identity (ADR 0029) and its `linh_vuc` is a free value, not a foreign key; a document already
-- stores its `loai_van_ban` code. So the per-type lookup needs no column here — it is a change in how
-- app.VanBanDen.hanXuLyXong and the automation jobs choose the `linh_vuc` they ask identity for.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: a handful of catalogue rows. ADD COLUMN with no default is metadata-only:
--      no row is rewritten, every row reads NULL.
--   2. IF IT STOPS HALF-WAY: it cannot — one statement, one transaction with its progress row
--      (core/migrate). IF NOT EXISTS makes a retry free.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. No existing column, key or filter is touched; the catalogue
--      reader ignores a column it does not select.
--   5. RETENTION: nothing removed or rewritten.
-- ---------------------------------------------------------------------------

ALTER TABLE loai_van_ban
    ADD COLUMN IF NOT EXISTS color TEXT
        CONSTRAINT loai_van_ban_color_shape CHECK (color ~ '^#[0-9A-Fa-f]{6}$');

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- `schema_migration` (core/migrate has no automatic rollback, ADR 0013):
--   ALTER TABLE loai_van_ban DROP COLUMN color;
-- Lossless only while
--   SELECT count(*) FROM loai_van_ban WHERE color IS NOT NULL;   -- must be 0
-- otherwise it discards what communes chose — a user decision (rule 7, stop condition 2).
-- ---------------------------------------------------------------------------
