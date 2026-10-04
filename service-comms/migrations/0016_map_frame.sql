-- 0016 — MAP FRAME: the fixed frame ONE commune's economic map may show — a centre and a radius
-- (ADR 0072 §"Sửa đổi 04/10/2026", H3). web-admin sets MapLibre's `maxBounds` from it, so the view
-- can neither be dragged nor zoomed out to national scale, where the world-wide OpenFreeMap tiles
-- (H1) draw Hoàng Sa and Trường Sa under OSM's names. No row = NO MAP is drawn, only the guidance
-- sentence and the form for a holder of `admin.lookup` (H3, "Chưa đặt").
--
-- OWNER: comms, next to the asset register it frames (0015_map_asset.sql) — H3, "Lưu gì". It is
-- per-commune configuration read at runtime (rule 1, invariant 10), never an environment variable.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). `map_` prefix and singular, as its siblings map_asset (0015)
-- and map_field_schema (0007).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- SHAPE: 0008's mail_settings / 0013's portal_sync_settings — one row per commune, tenant_id IS the
-- primary key, hash-partitioned MODULUS 32 (ADR 0010), a guard trigger refusing DELETE. Two columns
-- they do not have, because the brief for this table asks for them: `created_by`, and a guard that
-- also freezes `created_at` / `created_by` (0015's identity rule).
--
-- ---------------------------------------------------------------------------
-- OWNER-PENDING VALUES — NOT CUSTOMER FIGURES. Both are named in ADR 0072 H3 as not yet decided;
-- each lives in exactly one named CHECK below so the owner's answer is one migration:
--
--   * RADIUS 1–30 km (`map_frame_radius_range`). H3: "Đề xuất 1–30 km — chưa chốt". Widening the
--     upper bound is H3's stop condition 2 ("nới cận trên bán kính") — the user's call, not a
--     migration author's. Raising a bound is one migration; lowering it after a commune saved a
--     larger value is not.
--   * MAINLAND BOX lat 8.4–23.4, lng 102.1–109.5 (`map_frame_center_in_mainland_box`). H3: "giá trị
--     chính xác chốt khi dựng". The eastern edge 109.5°E deliberately EXCLUDES Hoàng Sa (~111–113°E)
--     and Trường Sa (~111–117°E): a centre there would let the frame show exactly what H3 exists to
--     keep out of view. Even at the 30 km ceiling a centre on the 109.5°E line reaches ~109.8°E, still
--     more than a degree short of Hoàng Sa. Widening the box is H3's stop condition 2 as well.
--     The box is a RECTANGLE, not a border: it also admits points in Laos, Cambodia, Thailand and
--     southern China. It is a floor against a bogus or sea-ward centre, not a test of "inside the
--     commune" — that check, if any, is the service's, against identity's boundary data (rule 2).
--     Islands INSIDE the box (Phú Quốc, Côn Đảo, Thổ Chu, Bạch Long Vĩ, Cô Tô) are accepted, so an
--     island commune can set its frame. A commune ON Hoàng Sa / Trường Sa cannot, and that is ADR
--     0072's separate open item, not something this file decides.
--
-- OTHER VENDOR CHOICES, stated so a reviewer can overrule them before rows exist:
--
--   * NO SOFT DELETE, for 0008's reason (mail_settings_guard header): one configuration row per
--     commune, overwritten in place; every change is an audit entry with before/after (rule 6),
--     which is the history. Unlike 0008/0013 there is no `is_enabled`: H3 has no "frame off" state —
--     a commune either has a frame or has never set one. Returning a commune to "no frame" is not in
--     H3 and is refused by the guard; if the owner asks for it, it is a column, never a DELETE.
--   * WHO WRITES: the holder of `admin.lookup` (H3, "Ai đặt"); enforced by the service, not here.
--   * numeric(10,6) for the centre — map_asset's own type (0015), ≈ 0.1 m. numeric(4,1) for the
--     radius: 0.1 km steps, and the type alone caps it at 999.9 should the CHECK ever be dropped.
--   * FORWARD-ONLY with a written manual reversal, like every comms migration (ADR 0013).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one, by construction — tenant_id IS the primary key. This file writes
--      NO row: a frame is a commune's own decision, and there is no default centre to seed (ADR 0072
--      §5 struck the hardcoded `15.730507, 108.378110` as one commune's value — rule 1, invariant 10).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every statement
--      is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS; a retry costs nothing. No
--      backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. One new table, function and trigger;
--      nothing existing is touched. Until a commune saves a row, its map page shows no map (H3) —
--      that is the designed state, not a half-applied one.
--   5. RETENTION: the row is never removed — the trigger refuses DELETE. Its history is the audit
--      trail the service writes for every change (rule 6).
--
-- PERSONAL DATA (rule 3): none. A commune's centre is a public place, not a home coordinate.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0003 checks it). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- map_frame_guard — the floor under the application's refusals.
--
--   DELETE                refused: a commune with a frame never silently goes back to none.
--   tenant_id,            identity of the row and who first set it — fixed at creation.
--   created_at/by
--   everything else       editable: moving or resizing the frame is the form's purpose; the audit
--                         entry holds what it was before.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION map_frame_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Change the centre or radius instead. ADR 0072 H3 has no "no frame" state '
                         'for a commune that has set one.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune and who first set the frame are fixed at creation.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: MapFrame
-- @scope:  tenant
--
-- map_frame — the fixed view of ONE commune's economic map: a centre and a radius. One row per
-- commune.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS map_frame (
    tenant_id   TEXT          NOT NULL,

    center_lat  NUMERIC(10,6) NOT NULL,
    center_lng  NUMERIC(10,6) NOT NULL,
    radius_km   NUMERIC(4,1)  NOT NULL,

    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- Business codes (rule 6, invariant 8) — `CB-…`, never an internal id.
    created_by  TEXT          NOT NULL,
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_by  TEXT          NOT NULL,

    -- One row per commune (rule 1, invariant 6: the only key, and it is tenant_id).
    PRIMARY KEY (tenant_id),

    -- OWNER-PENDING (ADR 0072 H3) — see header. Hoàng Sa and Trường Sa lie east of 109.5°E.
    CONSTRAINT map_frame_center_in_mainland_box CHECK (
        center_lat BETWEEN 8.4 AND 23.4 AND center_lng BETWEEN 102.1 AND 109.5),
    -- OWNER-PENDING (ADR 0072 H3) — proposal 1–30 km, see header.
    CONSTRAINT map_frame_radius_range CHECK (radius_km BETWEEN 1 AND 30),
    CONSTRAINT map_frame_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS map_frame_p%s PARTITION OF map_frame '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS map_frame_guard ON map_frame;
CREATE TRIGGER map_frame_guard
    BEFORE UPDATE OR DELETE ON map_frame
    FOR EACH ROW EXECUTE FUNCTION map_frame_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (ADR 0013's line).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM map_frame;
--
-- ZERO → in this order:
--
--   DROP TABLE map_frame;              -- partitions and trigger go with it
--   DROP FUNCTION map_frame_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → communes' saved frames, each one an audited decision by a named official. Dropping the
-- table loses them and turns every such commune's map off (H3: no frame, no map). That is rule 7 stop
-- condition #2 — the user plus a verified backup, never a command — and the way back is a NEW
-- migration.
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
