-- 0015 — MAP ASSET REGISTER: the places the economic map draws, one row per enterprise, household
-- business, market, cooperative… in one commune (docs/ui-ux/10-ban-do-kinh-te-so.md §8, §10, §12;
-- the specification's `doi_tuong_ban_do`). Staff menu `ban-do-kinh-te-so`.
--
-- OWNER: comms, next to the type catalogue (loai_tai_nguyen_ban_do, 0003) and the custom-field schema
-- (map_field_schema, 0007) that describe these rows (ADR 0024). This is the register 0007's header
-- says "does not exist yet"; every guard 0007 wrote for that day now has its subject.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). `MapAsset` because 0003 fixed `Map…` as the prefix and
-- `asset` as the word (permissions `asset.read` / `asset.update`, spec §12.5). Singular, matching
-- its two siblings `map_field_schema` and `loai_tai_nguyen_ban_do`. Enum VALUES stay Vietnamese
-- without diacritics (ADR 0011) — see `status`.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them before rows exist:
--
--   * `asset_type_code` CARRIES THE SAME COMPOSITE FOREIGN KEY AS map_field_schema (0007): the type
--     must have been issued in THIS commune's catalogue. It can never block the catalogue — its rows
--     are never hard deleted and `ma` never changes (0003, danh_muc_ba_tang). Consequence today: no
--     asset can be written in a commune whose catalogue is empty, which is every commune until the
--     catalogue is sown (0003, "WHERE THE he-thong ROWS COME FROM") — the same state 0007 accepted.
--     Removing the key later is one ALTER and loses nothing; adding it after rows exist needs every
--     row to be valid first. Whether the type is LIVE is checked by the service at write time.
--   * `asset_type_code` IS EDITABLE (staff picked the wrong group). The service owns what happens to
--     `custom_values` keyed by the old type's fields: they stay in the row (spec §12.6: a value is
--     never deleted by a schema change) and simply stop being shown.
--   * TAX CODE UNIQUE AMONG LIVE ROWS, NOT VIA A PARTIAL UNIQUE INDEX. `UNIQUE … WHERE deleted_at IS
--     NULL` is what tools/check_khoa_duy_nhat.py refuses (rule 7, invariant 3). The device finance's
--     0010, petitions' 0020 and reporting's 0003 use instead: a generated marker that holds the tax
--     code on a live row and NULL on a soft-deleted one or one without a code, and NULLs do not
--     collide. WHY LIVE-ONLY IS RIGHT HERE when it is wrong for an issued code: the tax code is NOT
--     issued by this system — the tax authority issues it — so the never-reissue rule has nothing to
--     protect. What the key protects is spec §9 / §12.2, "trùng mã số thuế ⇒ cập nhật": the Excel
--     import must find AT MOST ONE live row to update, or it updates whichever comes back first.
--     A soft-deleted enterprise does not block the same enterprise being entered again.
--   * `tax_code` SHAPE IS LOOSE: digits with at most one `-` group, ≤ 20 characters. Enterprise
--     codes are 10 digits or 10-3 (dependent unit); a household business may be filed under its
--     owner's PERSONAL tax code, which since 2025 is the 12-digit citizen ID number. Narrowing the
--     shape is a business rule this file does not get to guess, and for the same reason the column
--     is treated as POSSIBLY PERSONAL DATA — see PERSONAL DATA below.
--   * `phone` USES external_contacts' DIAL-CHARACTER SHAPE (0014): the screen turns it into a `tel:`
--     link and a letter in it breaks the link silently. NULL = no phone.
--   * `verified` ⇔ (`verified_at`, `verified_by`) both set. Un-verifying clears both; the audit entry
--     the service writes for every change (rule 6) keeps who had verified and when.
--   * NO NAME INDEX. pg_trgm is used nowhere in this repository, and adding an extension is not a
--     migration's call. Name search is `ILIKE '%…%'` inside one commune's partition, narrowed by the
--     type index; a btree on lower(name) would not serve a substring match anyway. The ceiling in
--     sight (below) makes that a sequential read of a few thousand rows at most.
--   * LENGTH BOUNDS are vendor guards against a bogus value, in the style of 0007/0014: name 255,
--     representative 255, address 500, description 4000, custom_values 64 KiB. Raising a bound is one
--     migration; lowering it after a commune saved a longer value is not.
--   * NO POSTGIS: lat/lng numeric(10,6), the spec's own type (§10); six decimals ≈ 0.1 m.
--   * FORWARD-ONLY with a written manual reversal, like every comms migration (ADR 0013). See
--     REVERSAL at the bottom.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: this file writes NO row (no seed: a seed row belongs to a commune and this
--      code path has none — 0003, REASON ONE). Steady state: hundreds to a few thousand per commune
--      (the prototype's sample is 26). Partitioned like every other table (ADR 0010).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every statement
--      is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS; a retry costs nothing. No
--      backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom — exact SQL and its lossless condition.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. One new table, function, trigger and
--      two indexes. The foreign key ADDS a referencing side to loai_tai_nguyen_ban_do and changes
--      nothing that table returns; it can never block a write to it (see VENDOR CHOICES). The KPI
--      reads that will count this table (spec §12.4, `/tong-quan`) do not exist yet — the first of
--      them must exclude soft-deleted rows like every read (rule 7, invariant 2).
--   5. RETENTION: soft delete only (rule 7, invariant 1); hard DELETE refused by the trigger; a
--      soft-deleted row is not edited again. An asset record is what a later inspection of the
--      commune's economic figures is checked against.
--
-- PERSONAL DATA (rule 3). `representative` (a person's name), `phone`, `address`, and `tax_code`
-- (possibly a citizen ID number, see above) are treated as personal data: never logged, never in a
-- cache key or URL, masked in API output without full-view permission, masked in the audit delta.
-- `name` of a household business is often the owner's name too; the service must not log it either.
-- Coordinates of a household business are a home location (rule 3: "home coordinates").
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0003 checks it). Generated
-- columns need 12, a foreign key referencing a partitioned table needs 12. Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- map_asset_guard — the floor under the application's refusals (0007/0014's shape).
--
--   DELETE                refused (rule 7, invariant 1).
--   a soft-deleted row    a historical record (rule 7, forbidden #5); editing it would bring it back
--                         or rewrite who removed it.
--   tenant_id, id,        identity of the row and who first entered it — fixed at creation.
--   created_at/by
--   everything else       editable: correcting a record is the screen's purpose; the audit entry
--                         holds what it was before.
-- Messages name the operation and the relation only (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION map_asset_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1.';
    END IF;

    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted asset is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5). Enter the '
                         'asset again as a new row.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.id         IS DISTINCT FROM OLD.id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune, id and who created the row are fixed at creation.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: MapAsset
-- @scope:  tenant
--
-- map_asset — one place on ONE commune's economic map, as the commune entered or imported it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS map_asset (
    tenant_id           TEXT          NOT NULL,
    id                  TEXT          NOT NULL,        -- ULID, internal

    -- The `ma` of a row in loai_tai_nguyen_ban_do of the SAME commune — composite foreign key below.
    asset_type_code     TEXT          NOT NULL,
    name                TEXT          NOT NULL,        -- "Công ty CP Chế biến Nông sản Bình An"
    address             TEXT,                          -- personal data when a household (rule 3)
    -- identity's residential unit (thôn / tổ dân phố) id, held as a VALUE: another service's row,
    -- so no foreign key (rule 2). NULL = "— Chưa xác định —" (spec §8.1).
    residential_unit_id TEXT,
    lat                 NUMERIC(10,6) NOT NULL,
    lng                 NUMERIC(10,6) NOT NULL,

    representative      TEXT,                          -- a person's name — personal data (rule 3)
    phone               TEXT,                          -- personal data (rule 3)

    status              TEXT          NOT NULL DEFAULT 'dang-hoat-dong',

    verified            BOOLEAN       NOT NULL DEFAULT false,
    verified_at         TIMESTAMPTZ,
    verified_by         TEXT,                          -- business code `CB-…` (rule 6, invariant 8)

    -- Possibly a citizen ID number (household business) — personal data (rule 3). See header.
    tax_code            TEXT,
    industry_code       TEXT,                          -- VSIC level 2, spec §5: "47"
    employee_count      INT,
    established_on      DATE,                          -- KPI `Thành lập mới` (spec §12.4)
    description         TEXT,
    -- {"<map_field_schema.field_code>": value, …}. Values under a retired or disabled field stay
    -- here (spec §12.6); the schema decides what the form shows, never what the row keeps.
    custom_values       JSONB         NOT NULL DEFAULT '{}'::jsonb,

    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- Business codes (rule 6, invariant 8) — `CB-…`, never an internal id.
    created_by          TEXT          NOT NULL,
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_by          TEXT          NOT NULL,

    deleted_at          TIMESTAMPTZ,
    deleted_by          TEXT,
    delete_reason       TEXT,

    -- The tax code of a LIVE row, NULL otherwise — see "TAX CODE UNIQUE AMONG LIVE ROWS" in the
    -- header. The guard refuses edits of a soft-deleted row, so a row's marker can never come back.
    live_tax_code       TEXT GENERATED ALWAYS AS
                            (CASE WHEN deleted_at IS NULL THEN tax_code END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, live_tax_code),

    CONSTRAINT map_asset_asset_type_fk
        FOREIGN KEY (tenant_id, asset_type_code)
        REFERENCES loai_tai_nguyen_ban_do (tenant_id, ma),
    CONSTRAINT map_asset_name_shape CHECK (
        btrim(name) <> '' AND char_length(name) <= 255 AND name !~ '[[:cntrl:]]'),
    -- NULL = absent. An all-blank value is NULL's job, not a second spelling of it.
    CONSTRAINT map_asset_address_shape CHECK (
        address IS NULL
        OR (btrim(address) <> '' AND char_length(address) <= 500 AND address !~ '[[:cntrl:]]')),
    CONSTRAINT map_asset_residential_unit_shape CHECK (
        residential_unit_id IS NULL OR btrim(residential_unit_id) <> ''),
    CONSTRAINT map_asset_lat_range CHECK (lat BETWEEN -90 AND 90),
    CONSTRAINT map_asset_lng_range CHECK (lng BETWEEN -180 AND 180),
    CONSTRAINT map_asset_representative_shape CHECK (
        representative IS NULL
        OR (btrim(representative) <> '' AND char_length(representative) <= 255
            AND representative !~ '[[:cntrl:]]')),
    CONSTRAINT map_asset_phone_shape CHECK (
        phone IS NULL
        OR (btrim(phone) <> '' AND char_length(phone) <= 32
            AND phone ~ '^[0-9+(). -]+$'
            AND char_length(regexp_replace(phone, '[^0-9]', '', 'g')) BETWEEN 3 AND 15)),
    CONSTRAINT map_asset_status_known CHECK (
        status IN ('dang-hoat-dong', 'tam-ngung', 'da-giai-the')),
    -- Verified carries both of its facts, or neither.
    CONSTRAINT map_asset_verified_complete CHECK (
        verified = (verified_at IS NOT NULL)
        AND verified = (verified_by IS NOT NULL)
        AND (verified_by IS NULL OR btrim(verified_by) <> '')),
    CONSTRAINT map_asset_tax_code_shape CHECK (
        tax_code IS NULL
        OR (char_length(tax_code) <= 20 AND tax_code ~ '^[0-9]+(-[0-9]+)?$')),
    CONSTRAINT map_asset_industry_code_shape CHECK (
        industry_code IS NULL OR industry_code ~ '^[0-9]{2}$'),
    CONSTRAINT map_asset_employee_count_non_negative CHECK (
        employee_count IS NULL OR employee_count >= 0),
    CONSTRAINT map_asset_description_length CHECK (
        description IS NULL OR char_length(description) <= 4000),
    CONSTRAINT map_asset_custom_values_shape CHECK (
        jsonb_typeof(custom_values) = 'object' AND octet_length(custom_values::text) <= 65536),
    CONSTRAINT map_asset_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> ''),
    -- Rule 7, invariant 1: all three soft-delete columns, or none.
    CONSTRAINT map_asset_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS map_asset_p%s PARTITION OF map_asset '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The layer counts, the map and the `Sổ địa điểm` table: one commune's live assets per group, id as
-- the tie-break. Leads with tenant_id (rule 1); soft-deleted rows excluded (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS map_asset_by_type
    ON map_asset (tenant_id, asset_type_code, id)
    WHERE deleted_at IS NULL;

-- The density-by-hamlet panel (spec §4.3) and the hamlet filter.
CREATE INDEX IF NOT EXISTS map_asset_by_residential_unit
    ON map_asset (tenant_id, residential_unit_id)
    WHERE deleted_at IS NULL AND residential_unit_id IS NOT NULL;

DROP TRIGGER IF EXISTS map_asset_guard ON map_asset;
CREATE TRIGGER map_asset_guard
    BEFORE UPDATE OR DELETE ON map_asset
    FOR EACH ROW EXECUTE FUNCTION map_asset_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (ADR 0013's line).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM map_asset;
--
-- ZERO → in this order:
--
--   DROP TABLE map_asset;              -- partitions, indexes, trigger, foreign key go with it
--   DROP FUNCTION map_asset_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → a commune's asset register. Dropping it is rule 7 stop condition #2 — the user plus a
-- verified backup, never a command — and the way back is a NEW migration.
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
