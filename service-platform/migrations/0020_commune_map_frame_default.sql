-- platform — commune_map_frame_default: the DEFAULT map frame of ONE commune — a centre and a radius
-- Vihat sets for that commune in the operations console (ADR 0072 §"Sửa đổi 04/10/2026 (lần 2)", K1).
-- service-comms reads it over PlatformService.GetMapFrameDefault and applies it only when the commune
-- has set no frame of its own (comms `map_frame`, K3: the commune's own value wins).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0001–0019 are
-- immutable.
--
-- WHY IN service-platform: K3 — a commune's location is COMMUNE METADATA, the same kind as its name,
-- province and hosts, which ADR 0003 places in the registry this service owns (`tenant`, 0001). The
-- commune's OWN frame stays in service-comms; this table never holds it, and comms never writes here
-- (rule 2: the read is gRPC, comms → platform only).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). Columns mirror comms' map_frame (0016 there) so one frame has
-- one spelling across the two services.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: at most one — tenant_id IS the primary key. This file writes NO row:
--      there is no default centre to seed (ADR 0072 §5 struck the hardcoded `15.730507, 108.378110` as
--      one commune's value — rule 1, invariant 10). Every row is entered by an operator, audited.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      progress row inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF
--      EXISTS, so a retry costs nothing. No backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. One NEW table, one NEW function,
--      one trigger on the new table. Until this file is applied GetMapFrameDefault fails (no table),
--      which the caller (core/platformclient.MapFrameDefault) reports as an error → comms answers 503,
--      never "not configured": the safe direction.
--   5. RETENTION: the row is never removed — the trigger refuses DELETE. Its history is the operator
--      trail in audit_log (actor_kind 'operator', tenant_id = the commune) the write path commits in
--      the same transaction as every change (rule 6, invariant 3).
--
-- PERSONAL DATA (rule 3): none. A commune's centre is a public place, not a home coordinate (comms
-- 0016 says the same of its own frame).
--
-- NOT PARTITIONED, like ho_so_hien_thi_xa (0006): at most one row per commune, nothing for hash
-- partitioning to spread.
--
-- NO SOFT DELETE, for comms 0016's reason: one configuration row per commune, overwritten in place;
-- every change is an audit entry with before/after, which is the history. K1 has no "remove the
-- default" act; if the owner asks for one, it is a column, never a DELETE.

-- ---------------------------------------------------------------------------
-- commune_map_frame_default_guard — the floor under the application's refusals.
--
--   DELETE                 refused: a commune's default never silently disappears; comms would then
--                          draw no map for a commune that had one.
--   tenant_id,             identity of the row and who first set it — fixed at creation.
--   created_at/by
--   everything else        editable: moving or resizing the default is the console's purpose; the
--                          audit entry holds what it was before.
--
-- A NEW FUNCTION, NOT 0006's platform_cam_xoa_cung: that one's hint says "soft delete instead", which
-- is the wrong advice for a table with no soft delete (0011 drew the same line).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION commune_map_frame_default_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Change the centre or radius instead (ADR 0072 K1). A default is never removed.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'The commune and who first set the default are fixed at creation.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- commune_map_frame_default — one commune's default map frame. One row per commune.
--
-- tenant_id REFERENCES the registry: a default for a commune that does not exist is refused by the
-- database, not only by the write path (which checks first, to answer 404).
--
-- numeric(10,6) for the centre (≈ 0.1 m, comms map_asset's own type); numeric(4,1) for the radius —
-- 0.1 km steps, so "> 0" is in practice ≥ 0.1 km (K2 "Bước"), and the type alone caps it at 999.9
-- should the CHECK ever be dropped.
--
-- WHO: an operator's business code `VH-…` (rule 6, invariant 8), the same shape service-identity's
-- operator_account.code CHECKs (`^VH-[0-9]{5,}$`, its 0012). Only the operator console writes this
-- table; a staff code or an internal id here would be a write from a path that must not exist.
-- ---------------------------------------------------------------------------
-- @entity: CommuneMapFrameDefault
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS commune_map_frame_default (
    tenant_id   TEXT          NOT NULL REFERENCES tenant (id),

    center_lat  NUMERIC(10,6) NOT NULL,
    center_lng  NUMERIC(10,6) NOT NULL,
    radius_km   NUMERIC(4,1)  NOT NULL,

    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_by  TEXT          NOT NULL,
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_by  TEXT          NOT NULL,

    -- One row per commune (rule 1, invariant 6: the only key, and it is tenant_id).
    PRIMARY KEY (tenant_id),

    -- ADR 0072 K2 — the centre stays in the mainland box of H3 (unchanged): lat 8.4–23.4, lng
    -- 102.1–109.5, the same numbers as comms' map_frame_center_in_mainland_box. The eastern edge
    -- 109.5°E keeps Hoàng Sa (~111.2°E) and Trường Sa (~111.5°E) out of any frame. Widening it is a
    -- stop condition of the second revision.
    CONSTRAINT commune_map_frame_default_center_in_mainland_box CHECK (
        center_lat BETWEEN 8.4 AND 23.4 AND center_lng BETWEEN 102.1 AND 109.5),
    -- ADR 0072 K2 — HARD ceiling, owner-decided: > 0 and ≤ 50 km. At 50 km a centre on 109.5°E reaches
    -- ≈ 109.99°E at most, still ≥ 1.2° short of both archipelagos (K2's table). Raising it is a stop
    -- condition of the second revision. The 3–20 km "usual" band is NOT here: it is an owner-adjustable
    -- proposal enforced as an acknowledgement by the write path, never a ceiling.
    CONSTRAINT commune_map_frame_default_radius_ceiling CHECK (radius_km > 0 AND radius_km <= 50),
    CONSTRAINT commune_map_frame_default_signed_by_operator CHECK (
        created_by ~ '^VH-[0-9]{5,}$' AND updated_by ~ '^VH-[0-9]{5,}$')
);

COMMENT ON TABLE commune_map_frame_default IS
    'Khung ban do MAC DINH cua mot xa (ADR 0072 sua doi lan 2, K1): tam + ban kinh do Vihat dat o khu van hanh. '
    'service-comms chi ap khi xa chua co khung rieng. Mot dong moi xa, khong bao gio xoa.';

DROP TRIGGER IF EXISTS commune_map_frame_default_guard ON commune_map_frame_default;
CREATE TRIGGER commune_map_frame_default_guard
    BEFORE UPDATE OR DELETE ON commune_map_frame_default
    FOR EACH ROW EXECUTE FUNCTION commune_map_frame_default_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the line 0002 draws.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback.
--
-- LOSSLESS ONLY WHILE THE TABLE IS EMPTY. Then, in this order: drop table commune_map_frame_default
-- (its trigger goes with it), then the function commune_map_frame_default_guard, then remove this
-- file's progress row from the schema_migration table, keyed on
-- ten = '0020_commune_map_frame_default.sql'. Written as prose rather than as runnable lines, because a
-- runnable line is a line that gets run.
--
-- NON-EMPTY → each row is an audited decision by a named operator, and dropping the table turns off
-- the map of every commune that relies on its default (K3: no frame at all, no map). That is rule 7
-- stop condition #2 — the user plus a verified backup, never a command — and the way back is a NEW
-- migration. The audit_log entries stay either way: that table is append-only (0002).
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten — nothing to
-- iterate and nothing to resume.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
