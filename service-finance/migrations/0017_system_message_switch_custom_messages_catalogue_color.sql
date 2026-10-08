-- 0017 — menu Cấu hình after ADR 0079 (owner answers of 08/10/2026), the finance half. A COPY OF
-- service-petitions' 0033 in this service's own schema (rule 2, invariant 2), differing only in the
-- group code and the catalogue. Three additive changes, one file, one transaction:
--
--   A. `system_message_override.is_active` — "Tắt / Bật lại" on a reworded sentence (ADR 0079 Q2).
--   B. `custom_system_message` — sentences a commune adds itself, group "Giải ngân" (ADR 0079 Q2, Q5a,
--      Q5b).
--   C. `color` on the catalogue `hang_muc_ke_hoach_von` (ADR 0079 row 5, Q1 #9).
--
-- ---------------------------------------------------------------------------
-- THIS FILE REPLACES A DECISION WRITTEN IN 0010, AND SAYS SO HERE BECAUSE 0010 CANNOT BE EDITED.
--
-- 0010:4-24 records the user decision of 28/09/2026: a commune may reword a shipped sentence, may not
-- invent one (no `+ Thêm câu mới`), and the specification's on/off flag (`dang_dung`) is "a switch the
-- user decided not to offer". The project owner REPLACED that on 08/10/2026 (ADR 0079 Q2, "Làm đúng
-- prototype"): a commune MAY add sentences, MAY switch a reworded sentence off and back on (keeping its
-- wording; while off, the software's sentence is used), and MAY soft delete a sentence it added. 0010
-- is applied and core/migrate refuses an edited file (ErrChecksumLech), so its comment stays as
-- written; where the two disagree, ADR 0079 and this file win.
--
-- WHAT 0010 STILL GOVERNS, UNCHANGED: the closed CHECK on `system_message_override.message_key`
-- (`budget.scope_notice`). A commune's own sentence is a different entity with its own table (B).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: A touches `system_message_override` (at most one live row per commune) as a
--      catalogue-only change — ADD COLUMN with a constant DEFAULT rewrites no row. B is a new empty
--      table. C adds a nullable column to a catalogue of a handful of rows per commune.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF
--      EXISTS, so a retry costs nothing. Nothing is backfilled, so there is no per-commune loop.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. After it
--      lands every override reads `is_active = true` — what it meant before — and `color` is NULL.
--      The meaning changes when the handler card makes the readers honour `is_active`: an inactive
--      override must then resolve to the shipped sentence everywhere it is read, not only on the
--      configuration list.
--   5. RETENTION: nothing is removed. B is soft deleted only, hard DELETE refused by a trigger; this
--      file also attaches the same refusal to `system_message_override`, which had none.
-- ---------------------------------------------------------------------------


-- ===========================================================================
-- A. system_message_override.is_active — "Tắt / Bật lại". A switch, not a second revert: the
-- inactive row keeps its wording and its live slot (`live_key` looks only at `deleted_at`).
-- NOT NULL DEFAULT true: every existing row was in force, and stays in force.
-- ===========================================================================
ALTER TABLE system_message_override
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- 0010 relied on the store never issuing DELETE (internal/store/system_message_override.go only
-- UPDATEs) — a promise of the application layer that a psql session never heard (ADR 0013).
CREATE OR REPLACE FUNCTION system_message_override_no_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Return to the shipped sentence with "Khôi phục câu mặc định" (soft delete: '
                     'deleted_at, deleted_by, delete_reason) or switch it off (is_active = false). '
                     'Rule 7, invariant 1.';
END $$;

DROP TRIGGER IF EXISTS system_message_override_no_delete ON system_message_override;
CREATE TRIGGER system_message_override_no_delete
    BEFORE DELETE ON system_message_override
    FOR EACH ROW EXECUTE FUNCTION system_message_override_no_delete();


-- ===========================================================================
-- B. custom_system_message — a sentence the COMMUNE added ("Xã tự thêm"), group "Giải ngân".
--
-- @entity: FinanceCustomSystemMessage
-- @scope:  tenant
--
-- WHY HERE: ADR 0079 Q5a — "Giải ngân ↔ service-finance". Groups "Phản ánh" and "Dùng chung" live in
-- service-petitions (its 0033). The entity name carries the service: petitions already declares
-- PetitionsCustomSystemMessage, and tools/kb reports one entity with two owners (rule 2, invariant 1).
--
-- STORED AND MANAGED ONLY (ADR 0079 Q5b). No code path shows these sentences to anybody yet.
--
-- Shape, constraints and guard are petitions' 0033 §B line for line; the reasons are written there and
-- not copied here (rule 9). The only difference is the group set.
-- ===========================================================================
CREATE OR REPLACE FUNCTION custom_system_message_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete: set deleted_at, deleted_by and delete_reason (rule 7, '
                         'invariant 1). The key stays taken for ever (rule 7, invariant 3).';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.id IS DISTINCT FROM OLD.id THEN
        RAISE EXCEPTION '%: tenant_id and id are immutable', TG_TABLE_NAME;
    END IF;
    IF NEW.message_key IS DISTINCT FROM OLD.message_key
       OR NEW.group_code IS DISTINCT FROM OLD.group_code THEN
        RAISE EXCEPTION '%: message_key and group_code are immutable', TG_TABLE_NAME
            USING HINT = 'An issued code is never renumbered (rule 7, invariant 3). Edit '
                         'message_text, or add a new sentence and delete this one.';
    END IF;
    IF NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: created_at and created_by are immutable', TG_TABLE_NAME;
    END IF;
    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted sentence is closed', TG_TABLE_NAME
            USING HINT = 'No edit and no undelete after a soft delete. Add a new sentence under a new key.';
    END IF;
    RETURN NEW;
END $$;

CREATE TABLE IF NOT EXISTS custom_system_message (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,
    -- ADR 0011 value: `giai-ngan` = "Giải ngân".
    group_code     TEXT        NOT NULL,
    -- Commune-chosen; must start with `giai-ngan.`, so it can never equal `budget.scope_notice` or a
    -- petitions/reporting key. UNIQUE below counts soft-deleted rows: a deleted key is never reused
    -- (rule 7, invariant 3).
    message_key    TEXT        NOT NULL,
    message_text   TEXT        NOT NULL,
    description    TEXT,
    is_active      BOOLEAN     NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL,
    -- Who: the staff BUSINESS CODE (`CB-00123`), never the internal id (rule 6, invariant 8).
    created_by     TEXT        NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    updated_by     TEXT        NOT NULL,
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, message_key),

    CONSTRAINT custom_system_message_group_known
        CHECK (group_code IN ('giai-ngan')),
    CONSTRAINT custom_system_message_key_shape
        CHECK (message_key ~ '^[a-z0-9]+([.-][a-z0-9]+)*$'
               AND char_length(message_key) <= 100
               AND starts_with(message_key, group_code || '.')),
    CONSTRAINT custom_system_message_text_shape
        CHECK (message_text = btrim(message_text)
               AND char_length(message_text) BETWEEN 1 AND 1000
               AND message_text !~ '[[:cntrl:]]'
               AND message_text !~ '[<>]'),
    CONSTRAINT custom_system_message_description_shape
        CHECK (description IS NULL
               OR (description = btrim(description)
                   AND char_length(description) BETWEEN 1 AND 1000
                   AND description !~ '[[:cntrl:]]'
                   AND description !~ '[<>]')),
    CONSTRAINT custom_system_message_actors_present
        CHECK (btrim(created_by) <> '' AND char_length(created_by) <= 64
               AND btrim(updated_by) <> '' AND char_length(updated_by) <= 64),
    -- All three delete facts or none — both directions (petitions 0033 §B).
    CONSTRAINT custom_system_message_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL)
               AND (deleted_by IS NULL OR (btrim(deleted_by) <> '' AND char_length(deleted_by) <= 64))
               AND (delete_reason IS NULL OR (btrim(delete_reason) <> '' AND char_length(delete_reason) <= 200)))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS custom_system_message_p%s PARTITION OF custom_system_message '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS custom_system_message_list
    ON custom_system_message (tenant_id, group_code, message_key) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS custom_system_message_guard ON custom_system_message;
CREATE TRIGGER custom_system_message_guard
    BEFORE UPDATE OR DELETE ON custom_system_message
    FOR EACH ROW EXECUTE FUNCTION custom_system_message_guard();


-- ===========================================================================
-- C. color on `hang_muc_ke_hoach_von` (ADR 0079 row 5; Q1 #9: editable on "Hệ thống" rows too).
-- NULL = no colour chosen. `#RRGGBB` only. danh_muc_ba_tang (0003) does not guard `color`, so it is
-- editable on every tier, as decided.
-- ===========================================================================
ALTER TABLE hang_muc_ke_hoach_von
    ADD COLUMN IF NOT EXISTS color TEXT
        CONSTRAINT hang_muc_ke_hoach_von_color_shape CHECK (color ~ '^#[0-9A-Fa-f]{6}$');


-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (0003, ADR 0013).
--
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- `schema_migration` (core/migrate has no automatic rollback, ADR 0013).
--   B: lossless while `SELECT count(*) FROM custom_system_message` is 0 —
--        DROP TABLE custom_system_message; DROP FUNCTION custom_system_message_guard();
--      with rows, it destroys communes' sentences: user decision (rule 7, stop condition 2).
--   A: DROP TRIGGER system_message_override_no_delete ON system_message_override;
--      DROP FUNCTION system_message_override_no_delete();
--      dropping `is_active` is lossless only while no row has is_active = false; otherwise it silently
--      switches those wordings back ON — a user decision.
--   C: dropping `color` is lossless only while every row has color IS NULL — otherwise a user decision.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions — repeated because
-- this file declares a new PARTITION BY table.
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
