-- 0018 — the residential units (`thon_to_dan_pho`, 0005) gain the two fields their WRITE routes need:
-- the head of the unit (Trưởng thôn / Tổ trưởng dân phố) and the rank the commune lists them in.
-- User decision 2026-09-29, ADR 0059 §2: create, edit, take out of use (soft) under `admin.org`, and an
-- Excel import — following ../vigov-require/apps/api/app/modules/org/schemas.py:161-186 (HamletCreate:
-- head_user_id, sort_order).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0005: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file stops the service or leaves two databases with one version number.
--
-- NEW COLUMNS ARE ENGLISH (rule 12, invariant 1); the table's existing Vietnamese columns stay as they
-- are (invariant 3).
--
-- ---------------------------------------------------------------------------
-- 1. `head_staff_id` — the staff row (`nguoi_dung.id`) that heads the unit. NULL = nobody recorded.
--
-- THE ID AND NOT THE STAFF CODE, the convention every row-to-row reference in this schema follows
-- (bo_phan.cha_id, nguoi_dung.bo_phan_id, nguoi_dung.vai_tro_id). The code is what the AUDIT TRAIL
-- quotes (rule 6, invariant 8) and what the write routes accept, because the staff picker every
-- account can read returns codes, not ids; the use case resolves one to the other.
--
-- A REAL COMPOSITE FOREIGN KEY, for the reason 0005 gives `loai`: both tables are in THIS service's
-- schema, so the key crosses no boundary, and `(tenant_id, …)` makes a head from ANOTHER commune a
-- statement the database refuses (rule 1). Staff rows are never hard-deleted (0001 trigger), so the
-- key can never block a legitimate delete.
--
-- ---------------------------------------------------------------------------
-- 2. `sort_order` — the rank on the commune's own list (0 = default; ties sort by name). Same bound as
--    `bo_phan.thu_tu` (domain.TranThuTuBoPhan, 9999), checked here as the floor.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none written. `head_staff_id` is NULL and `sort_order` 0 on every existing
--      row; until this migration there was no write route, so the table is empty in every commune.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. Reads select named columns, never `*`.
--   5. RETENTION: nothing is removed.
-- ---------------------------------------------------------------------------

ALTER TABLE thon_to_dan_pho ADD COLUMN IF NOT EXISTS head_staff_id TEXT;
ALTER TABLE thon_to_dan_pho ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

ALTER TABLE thon_to_dan_pho ADD CONSTRAINT thon_to_dan_pho_head_staff_fkey
    FOREIGN KEY (tenant_id, head_staff_id) REFERENCES nguoi_dung (tenant_id, id);
ALTER TABLE thon_to_dan_pho ADD CONSTRAINT thon_to_dan_pho_sort_order_range
    CHECK (sort_order >= 0 AND sort_order <= 9999);

COMMENT ON COLUMN thon_to_dan_pho.head_staff_id IS
    'nguoi_dung.id cua Truong thon / To truong dan pho. NULL = chua ghi. Tuyen ghi nhan MA can bo va '
    'doi sang id (ADR 0059 muc 2).';
COMMENT ON COLUMN thon_to_dan_pho.sort_order IS
    'Thu tu tren danh sach cua xa; 0 la mac dinh, cung thu tu thi xep theo ten.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, and ONLY while every row still holds
-- `head_staff_id IS NULL AND sort_order = 0` (check first):
--
--   drop constraints thon_to_dan_pho_sort_order_range and thon_to_dan_pho_head_staff_fkey, drop
--   columns sort_order and head_staff_id, and remove this file's row from schema_migration.
--
-- Written as prose, not a runnable line, because a runnable line is a line that gets run. Once a
-- commune has recorded a head or a rank, reversal destroys data the commune entered — rule 7 stop
-- condition #2, a decision for the user with a verified backup.
-- ---------------------------------------------------------------------------
