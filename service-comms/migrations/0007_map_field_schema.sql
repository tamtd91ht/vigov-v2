-- comms — the per-commune custom field schema of the economic map (docs/ui-ux/14-cau-hinh.md §6,
-- "Trường bản đồ"; docs/ui-ux/10-ban-do-kinh-te-so.md §10, `truong_tuy_bien_ban_do`).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0003: 0001-0006 have been applied and core/migrate compares
-- the checksum of every applied file at startup (see 0003's header).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): the specification's `truong_tuy_bien_ban_do` is the
-- design-time name; new tables and columns are English. The entity is `MapFieldSchema` because
-- `MapAssetType` (0003) already fixed `Map…` as the prefix of this subsystem and `asset` as its
-- word. Enum VALUES stay Vietnamese without diacritics (ADR 0011) — see `value_type`.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. This file writes no row. It cannot even hold one yet in
--      any commune whose type catalogue is empty — which is every commune today (0003, "WHERE THE
--      he-thong ROWS COME FROM"): a field hangs off an asset type code, and the foreign key below
--      refuses a code the commune's catalogue does not hold. NO FIELD IS SEEDED for the same two
--      reasons 0003 seeds no type: there is no commune on this code path, and the group list the
--      fields would hang off is itself undecided (8 vs 11 groups). The ceiling in sight is a few
--      dozen rows per commune.
--   2. IF IT STOPS HALF-WAY: one file, one transaction; every statement is IF NOT EXISTS or
--      CREATE OR REPLACE, so a retry from the beginning costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Only a new table, a new
--      function and a new trigger. The foreign key ADDS a referencing side to
--      loai_tai_nguyen_ban_do and changes nothing it returns; that table already refuses hard
--      DELETE and any change of `ma` (0003, danh_muc_ba_tang), so the key can never block a write
--      the existing code performs.
--   5. RETENTION: nothing is removed. A field is soft deleted and its key stays taken forever —
--      see (1) below.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS TABLE DESCRIBES DOES NOT EXIST YET, said first because it shapes every guard below.
--
-- There is no asset register (`doi_tuong_ban_do`, docs/ui-ux/10-ban-do-kinh-te-so.md §10) in this
-- repository. These rows define the form of data nobody can store yet. Every rule here is written
-- for the day that register exists, because that is the day the rules become impossible to add
-- without a data migration on archival records:
--
-- (1) UNIQUE (tenant_id, asset_type_code, field_code) COUNTS SOFT-DELETED ROWS. The key is the
--     name under which an asset stores its value. Reissue `legal_form` after retiring it and every
--     asset that recorded the old field silently reads as the new one — possibly of another
--     type. Rule 7, invariant 3: an issued code is never reissued.
-- (2) `value_type` IS IMMUTABLE. Stored values were written as that type; changing `so-nguyen` to
--     `ngay` makes every one of them unreadable, with no error until somebody opens the record.
-- (3) AN OPTION VALUE IS NEVER REMOVED. Assets store the option's VALUE. Whether any asset uses a
--     given value cannot be known — the register does not exist — so removal is refused outright
--     rather than refused "when used". Relabelling and appending are allowed.
-- (4) `is_required` false -> true is allowed and will MATTER once assets exist: every asset
--     already filed without that value becomes invalid on its next edit. Nothing here can check
--     that today, because there is nothing to count.
--
-- `Tắt` (is_active = false) and `Xoá` (soft delete) ARE DIFFERENT ACTS, and the difference is the
-- one ../vigov-require settled (apps/api/app/modules/assets/service.py, delete_field_schema): a
-- disabled field stays in the list and can be re-enabled; a deleted field leaves every read path
-- and cannot come back. In BOTH cases values already stored under the key stay where they are —
-- "Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ" (docs/ui-ux/14-cau-hinh.md:202).
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is already the floor (0003 checks it). A foreign key REFERENCING a partitioned
-- table needs 12; a BEFORE ... FOR EACH ROW trigger on a partitioned table needs 13.

-- ---------------------------------------------------------------------------
-- map_field_schema_guard — the floor under the application's refusals. Same argument as 0003's
-- danh_muc_ba_tang: the service refuses first in a sentence staff can act on; this refuses for
-- every writer, including a psql prompt and a future import job.
--
-- WHAT EACH REFUSAL IS FOR, in the order checked:
--
--   DELETE                — soft delete only (rule 7, invariant 1); a hard delete would also
--                           free the key for reuse.
--   update of a retired   — a soft-deleted row is a historical record (rule 7, forbidden #5).
--     row                   Editing it would let the row come back, or rewrite who retired it.
--   tenant_id · asset_type_code · field_code
--                         — the identity of the field; assets hold (type, key) as the address
--                           of a value.
--   value_type            — (2) in the header.
--   option removal        — (3) in the header.
--
-- The messages name the operation and the relation, nothing else (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION map_field_schema_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. A hard delete would also free the field key for reuse.';
    END IF;

    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted field is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5). Add a '
                         'field under a NEW key instead.';
    END IF;

    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.asset_type_code IS DISTINCT FROM OLD.asset_type_code
       OR NEW.field_code IS DISTINCT FROM OLD.field_code THEN
        RAISE EXCEPTION '%: commune, asset type and field key are immutable', TG_TABLE_NAME
            USING HINT = 'Assets address a stored value by (asset type, field key). To rename '
                         'what staff see, edit `label`.';
    END IF;

    IF NEW.value_type IS DISTINCT FROM OLD.value_type THEN
        RAISE EXCEPTION '%: value_type is immutable', TG_TABLE_NAME
            USING HINT = 'Values already stored were written as the old type and would become '
                         'unreadable. Add a new field under a new key instead.';
    END IF;

    IF EXISTS (
        SELECT 1 FROM jsonb_array_elements(OLD.options) AS o
        WHERE NOT EXISTS (
            SELECT 1 FROM jsonb_array_elements(NEW.options) AS n
            WHERE n->>'value' = o->>'value'))
    THEN
        RAISE EXCEPTION '%: an option value cannot be removed', TG_TABLE_NAME
            USING HINT = 'Assets store the option VALUE. Relabel it or append new options; '
                         'removing one would orphan every value recorded under it.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: MapFieldSchema
-- @scope:  tenant
--
-- map_field_schema — one configured field of one asset type's form, in one commune.
--
-- Owned by comms, next to the type catalogue it hangs off (loai_tai_nguyen_ban_do, 0003) and the
-- asset register it will describe (ADR 0024).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS map_field_schema (
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,
    -- The `ma` of a row in loai_tai_nguyen_ban_do of the SAME commune — the foreign key below is
    -- composite with tenant_id, so a field can never hang off another commune's type.
    asset_type_code TEXT        NOT NULL,
    -- "legal_form" — `Mã trường`. ../vigov-require calls it `field_key`; see
    -- internal/http/map_field_schema.go (mapFieldSchemaOut.FieldCode) for why it is `code` here.
    field_code       TEXT        NOT NULL,
    label           TEXT        NOT NULL,          -- "Loại hình doanh nghiệp"
    value_type      TEXT        NOT NULL,          -- "chon"
    -- [{"value": "...", "label": "..."}], non-empty exactly when value_type = 'chon'.
    options         JSONB       NOT NULL DEFAULT '[]'::jsonb,
    is_required     BOOLEAN     NOT NULL DEFAULT false,
    sort_order      INT         NOT NULL DEFAULT 0,
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    deleted_at      TIMESTAMPTZ,
    deleted_by      TEXT,
    delete_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id, and counting soft-deleted rows — see (1) in the header.
    UNIQUE (tenant_id, asset_type_code, field_code),
    -- The type must exist in THIS commune's catalogue. Referencing (tenant_id, ma) — the unique
    -- key 0003 declares, which also counts soft-deleted rows — so a type that is later retired
    -- still anchors the fields filed under it. Whether a type is LIVE is checked by the service
    -- at create time; the key only guarantees it was ever issued in this commune. It can never
    -- block the type catalogue: its rows are never hard deleted and `ma` never changes (0003).
    CONSTRAINT map_field_schema_asset_type_fk
        FOREIGN KEY (tenant_id, asset_type_code)
        REFERENCES loai_tai_nguyen_ban_do (tenant_id, ma),
    CONSTRAINT map_field_schema_field_code_shape
        CHECK (field_code ~ '^[a-z][a-z0-9_]*$' AND char_length(field_code) <= 64),
    CONSTRAINT map_field_schema_label_length
        CHECK (char_length(label) BETWEEN 1 AND 255),
    CONSTRAINT map_field_schema_value_type_known
        CHECK (value_type IN ('van-ban', 'so-nguyen', 'so-thap-phan', 'dung-sai', 'ngay', 'chon')),
    CONSTRAINT map_field_schema_options_match_type
        CHECK (jsonb_typeof(options) = 'array'
               AND (value_type = 'chon') = (jsonb_array_length(options) > 0)),
    CONSTRAINT map_field_schema_sort_order_non_negative
        CHECK (sort_order >= 0),
    -- Rule 7, invariant 1: all three soft-delete columns, or none.
    CONSTRAINT map_field_schema_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS map_field_schema_p%s PARTITION OF map_field_schema '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The configuration tab lists one commune's fields per group in display order, disabled rows
-- included; only soft-deleted rows drop out (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS map_field_schema_list
    ON map_field_schema (tenant_id, asset_type_code, sort_order) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS map_field_schema_guard ON map_field_schema;
CREATE TRIGGER map_field_schema_guard
    BEFORE UPDATE OR DELETE ON map_field_schema
    FOR EACH ROW EXECUTE FUNCTION map_field_schema_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and
-- ADR 0013 draw.
--
-- REVERSAL (question 3). While the table is empty the reversal is complete and loses nothing:
-- drop the table (partitions, trigger and foreign key go with it), then the function, and in the
-- same transaction remove this file's row from `schema_migration`. Once a commune has rows here
-- that is no longer a reversal but the destruction of commune records — rule 7's first stop
-- condition, which needs the user, not a command.
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
