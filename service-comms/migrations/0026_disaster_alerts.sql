-- 0026 — DISASTER ALERTS: a warning a commune posts for its residents (storm, flood, landslide, heavy
-- rain, epidemic, fire), shown on the citizen Mini App with the danger zone, the evacuation points and
-- the numbers to call. Staff post it; nothing is sent by ZNS (ADR 0047 G2: "chưa gửi ZNS").
--
-- OWNER: comms — ADR 0047 G2 (`DisasterAlert`, `/disaster-alerts`, permissions `content.read` /
-- `content.update`), widened by the owner on 11/10/2026 from four kinds to six (epidemic and fire
-- added). Shape follows the prototype (../vigov-require apps/miniapp/src/mocks/data.ts `disasterAlert`)
-- and 0014's `external_contacts` for the table discipline. Routes and Go are a later card.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051), plural like 0013/0014. Enum VALUES are Vietnamese without
-- diacritics, kebab-case (ADR 0011):
--
--   bao        bão        storm
--   lu         lũ         flood
--   sat-lo     sạt lở     landslide
--   mua-lon    mưa lớn    heavy rain
--   dich-benh  dịch bệnh  epidemic
--   chay       cháy       fire
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them before rows exist:
--
--   * `level` IS FREE TEXT AND NULLABLE. The staff member types it ("Cấp độ rủi ro thiên tai: Cấp 2").
--     The statutory risk levels exist for natural disasters, not for every epidemic or fire, so a
--     required level would force staff to invent one. `headline` (the one-line banner) is nullable
--     too: the read path may show `title` in its place. Making either required later is a migration
--     on a populated table; loosening is cheap — the asymmetry is accepted to avoid forcing invented text.
--   * THE THREE JSONB LISTS ARE CHECKED AS ARRAYS WITH A LENGTH CAP, NOT ELEMENT BY ELEMENT. Element
--     shape (danger_zone {x,y} numbers 0..100; evacuation {name,address,phone,capacity}; contact
--     {role,name,phone}) is the Go write path's, which can return a field-level error. The database
--     holds the floor a psql INSERT cannot slip under: never a scalar, never an object, never NULL,
--     never an unbounded list. Caps: 200 polygon points, 50 evacuation points, 50 contacts.
--   * `danger_zone` COORDINATES ARE RELATIVE (0..100 of the picture the Mini App draws), not latitude /
--     longitude. No resident's location is ever stored or asked for (ADR 0047 G2).
--   * ENDING EARLY is `ended_at` + `ended_by`, both or neither. An ended alert is a historical record of
--     what residents were told: from then on only the soft-delete trio and `updated_*` may change
--     (guard below). Correcting text is done while the alert is live.
--   * LENGTH BOUNDS are vendor guards against a bogus value, 0014's style: title / headline 255, level
--     100, detail 10000. Raising a bound is one migration; lowering it after a commune saved a longer
--     value is not.
--   * FORWARD-ONLY with a written manual reversal (ADR 0013). See REVERSAL at the bottom.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: this file writes NO row. Steady state: a handful per commune per season.
--      A plain table, not partitioned (ADR 0096).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every statement
--      is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS; a retry costs nothing. No
--      backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. One new table, one function, one
--      trigger, one index. Nothing existing is altered.
--   5. RETENTION: soft delete only (rule 7, invariant 1); hard DELETE refused by the trigger; a
--      soft-deleted or ended alert is not rewritten. "The app told us the shelter was X" is exactly
--      the complaint this row, and the audit entries the service writes for every change, answer.
--
-- PERSONAL DATA (rule 3). Intended content is institutional (a shelter's switchboard, a duty desk).
-- But `emergency_contacts[].name/phone` and `evacuation_points[].phone` may hold an officer's own
-- mobile, so the service treats both lists as possibly personal: never logged, never in a cache key,
-- masked in the audit delta. Shown unmasked on the Mini App because being shown is the purpose.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0004 checks it). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- disaster_alerts_guard — the floor under the application's refusals (0014's shape, plus the ended
-- rule).
--
--   DELETE                refused (rule 7, invariant 1).
--   a soft-deleted row    a historical record (rule 7, forbidden #5).
--   tenant_id, id,        identity of the row and who first posted it — fixed at creation.
--   created_at/by
--   an ENDED alert        only deleted_at/deleted_by/delete_reason and updated_at/updated_by change.
--                         Compared as the whole row minus those five columns, so a column added
--                         later is frozen on an ended alert without editing this function.
-- Messages name the operation and the relation only (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION disaster_alerts_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1.';
    END IF;

    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted alert is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5). Post a '
                         'new alert instead.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.id         IS DISTINCT FROM OLD.id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune, id and who posted the alert are fixed at creation.';
    END IF;

    IF OLD.ended_at IS NOT NULL
       AND (to_jsonb(NEW) - ARRAY['deleted_at', 'deleted_by', 'delete_reason', 'updated_at', 'updated_by'])
           IS DISTINCT FROM
           (to_jsonb(OLD) - ARRAY['deleted_at', 'deleted_by', 'delete_reason', 'updated_at', 'updated_by']) THEN
        RAISE EXCEPTION '%: an ended alert is not edited', TG_TABLE_NAME
            USING HINT = 'Residents were shown this text until it ended; it is now a historical '
                         'record. Post a new alert instead.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: DisasterAlert
-- @scope:  tenant
--
-- disaster_alerts — one warning ONE commune posted for its residents. Public on the Mini App while
-- not soft-deleted, not ended, and inside [effective_from, effective_to).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS disaster_alerts (
    tenant_id           TEXT        NOT NULL,
    id                  TEXT        NOT NULL,          -- ULID, internal

    kind                TEXT        NOT NULL,          -- one of six, header table
    title               TEXT        NOT NULL,          -- "Mưa lớn diện rộng, ngập cục bộ"
    level               TEXT,                          -- typed by staff; NULL = none stated
    headline            TEXT,                          -- one-line banner; NULL = read path uses title
    detail              TEXT        NOT NULL,

    effective_from      TIMESTAMPTZ NOT NULL,
    effective_to        TIMESTAMPTZ,                   -- NULL = until ended or replaced
    ended_at            TIMESTAMPTZ,                   -- staff ended it early
    ended_by            TEXT,

    -- Relative polygon over the commune picture: [{"x":18,"y":26}, ...], 0..100 each.
    danger_zone         JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- [{"name","address","phone","capacity"}, ...]
    evacuation_points   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- [{"role","name","phone"}, ...]
    emergency_contacts  JSONB       NOT NULL DEFAULT '[]'::jsonb,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Business codes (rule 6, invariant 8) — `CB-…`, never an internal id.
    created_by          TEXT        NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          TEXT        NOT NULL,

    deleted_at          TIMESTAMPTZ,
    deleted_by          TEXT,
    delete_reason       TEXT,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),

    CONSTRAINT disaster_alerts_kind_valid
        CHECK (kind IN ('bao', 'lu', 'sat-lo', 'mua-lon', 'dich-benh', 'chay')),
    CONSTRAINT disaster_alerts_title_shape CHECK (
        btrim(title) <> '' AND char_length(title) <= 255 AND title !~ '[[:cntrl:]]'),
    -- NULL = not stated. An all-blank value is NULL's job, not a second spelling of it.
    CONSTRAINT disaster_alerts_level_shape CHECK (
        level IS NULL
        OR (btrim(level) <> '' AND char_length(level) <= 100 AND level !~ '[[:cntrl:]]')),
    CONSTRAINT disaster_alerts_headline_shape CHECK (
        headline IS NULL
        OR (btrim(headline) <> '' AND char_length(headline) <= 255 AND headline !~ '[[:cntrl:]]')),
    -- detail is multi-line prose: newlines allowed, so no [[:cntrl:]] ban here.
    CONSTRAINT disaster_alerts_detail_shape CHECK (
        btrim(detail) <> '' AND char_length(detail) <= 10000),
    CONSTRAINT disaster_alerts_effective_window CHECK (
        effective_to IS NULL OR effective_to > effective_from),
    -- Ending is an act of a person: when and who, both or neither.
    CONSTRAINT disaster_alerts_ended_complete CHECK (
        (ended_at IS NULL) = (ended_by IS NULL)
        AND (ended_by IS NULL OR btrim(ended_by) <> '')),
    CONSTRAINT disaster_alerts_danger_zone_array CHECK (
        jsonb_typeof(danger_zone) = 'array' AND jsonb_array_length(danger_zone) <= 200),
    CONSTRAINT disaster_alerts_evacuation_points_array CHECK (
        jsonb_typeof(evacuation_points) = 'array' AND jsonb_array_length(evacuation_points) <= 50),
    CONSTRAINT disaster_alerts_emergency_contacts_array CHECK (
        jsonb_typeof(emergency_contacts) = 'array' AND jsonb_array_length(emergency_contacts) <= 50),
    CONSTRAINT disaster_alerts_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> ''),
    -- Rule 7, invariant 1: all three soft-delete columns, or none.
    CONSTRAINT disaster_alerts_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
);

-- The public read (and the staff list): one commune's live alerts by start time. Leads with
-- tenant_id (rule 1); soft-deleted rows excluded (rule 7, invariant 2). The "in force now" filter
-- (ended_at IS NULL, effective_to > now()) is applied on top — few rows per commune.
CREATE INDEX IF NOT EXISTS disaster_alerts_public
    ON disaster_alerts (tenant_id, effective_from)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS disaster_alerts_guard ON disaster_alerts;
CREATE TRIGGER disaster_alerts_guard
    BEFORE UPDATE OR DELETE ON disaster_alerts
    FOR EACH ROW EXECUTE FUNCTION disaster_alerts_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (ADR 0013's line); the element
-- shape inside the three JSONB lists (Go's, see VENDOR CHOICES).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM disaster_alerts;
--
-- ZERO → in this order:
--
--   DROP TABLE disaster_alerts;        -- index, trigger go with it
--   DROP FUNCTION disaster_alerts_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → what a public authority warned its residents. Dropping it is rule 7 stop condition #2 —
-- the user plus a verified backup, never a command — and the way back is a NEW migration.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
