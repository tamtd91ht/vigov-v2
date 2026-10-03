-- 0014 — EXTERNAL CONTACTS: the commune's list of bodies OUTSIDE its own apparatus that residents
-- call — the health station, the commune police, the power company, schools (`liên hệ ngoài bộ máy
-- xã`). Shown on the citizen Mini App directory; staff menu `danh-ba-can-bo`.
--
-- OWNER: comms — the user's decision of 03/10/2026 (rule 2's stop resolved). A SEPARATE TABLE: these
-- are institutions, not staff. No account, no link to identity's `nguoi_dung`, and nothing here is
-- ever mixed into the staff directory. The commune types the rows in itself; nothing is seeded.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). PLURAL, `external_contacts`, matching the newest
-- English-named comms migration (0013: `portal_categories`, `portal_sync_runs`). The URL nouns are
-- `external-contacts` (staff) and `commune-external-contacts` (public) — routes are a later card.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them before rows exist:
--
--   * NO PUBLISH FLAG. Every row that is not soft-deleted is public on the Mini App. The list exists
--     ONLY to be shown to residents; a "hidden" external contact has no reader. ../vigov-require's
--     StaffContact needed `is_published` because its list held hundreds of officers' own numbers —
--     a different list, which this table is not. Adding a flag later is one nullable column.
--   * `category` IS FREE TEXT, NOT A CATALOGUE. It is the group heading the Mini App draws ("Y tế",
--     "Công an"); rows sharing the exact text are one group. Simplest thing: no table, no foreign
--     key, no screen to manage headings. Cost: a typo makes a second heading, fixed by editing the
--     row. Turning it into a catalogue later is a new table plus a backfill on a few dozen rows.
--   * `display_order` IS NULLABLE. NULL = "no position given"; the public read sorts it LAST
--     (btree ASC puts NULLs last, so the index below serves `ORDER BY display_order, id` as is).
--   * NO BUSINESS UNIQUE KEY beyond the primary key. Two rows may share a name (two "Trường Tiểu
--     học" in different villages, told apart by address). There is no issued code to protect, so
--     the rule-7 "never reissue" argument for non-partial keys does not arise.
--   * `phone` IS DIAL CHARACTERS ONLY: digits, `+`, spaces, `.`, `-`, parentheses; 3 to 15 digits
--     (113/114/115 are three; E.164 caps at fifteen); at most 32 characters. The Mini App turns it
--     into a `tel:` link, and a letter in it ("máy lẻ 12") breaks the link silently. An extension
--     goes in `address` or a second row.
--   * LENGTH BOUNDS are vendor guards against a bogus value, in the style of 0007/0013: name 255
--     (map_field_schema.label), category 100 (a heading), address 500 (noi_dung_mini_app.event_place).
--     Raising a bound is one migration; lowering it after a commune saved a longer value is not.
--   * FORWARD-ONLY with a written manual reversal, like every comms migration (ADR 0013; no down
--     file may live here, 0011:532-535). See REVERSAL at the bottom.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: this file writes NO row. Steady state a few dozen per commune — one per
--      institution a resident might call. Partitioned like every other table (ADR 0010).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every
--      statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS; a retry costs
--      nothing. No backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom — exact SQL and its lossless condition.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. One new table, one new function, one
--      trigger, one index. Nothing existing is altered, so no existing read sees anything new.
--   5. RETENTION: soft delete only (rule 7, invariant 1); hard DELETE refused by the trigger; a
--      soft-deleted row is not edited again. A contact a resident was shown is what a later
--      complaint ("the number on the app was wrong") asks about — the row and the audit entries the
--      service writes for every change (rule 6) answer it.
--
-- PERSONAL DATA (rule 3). The intended content is INSTITUTIONAL — a public body's name, switchboard
-- and address. But nothing stops a commune typing an officer's own mobile into `phone`, so the
-- service treats `phone` and `address` as possibly personal: never logged, never in a cache key,
-- masked in the audit delta like any phone. They are shown UNMASKED on the Mini App because being
-- shown is the purpose of the row — the commune entered them for that.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0003 checks it). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- external_contacts_guard — the floor under the application's refusals (0007/0010's shape).
--
--   DELETE                refused (rule 7, invariant 1).
--   a soft-deleted row    a historical record (rule 7, forbidden #5); editing it would bring it back
--                         or rewrite who removed it.
--   tenant_id, id,        identity of the row and who first entered it — fixed at creation.
--   created_at/by
--   name, category,       editable: correcting a number is the whole point of the screen; the audit
--   phone, address,       entry holds what it was before.
--   display_order, updated_*, the soft-delete trio
-- Messages name the operation and the relation only (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION external_contacts_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1.';
    END IF;

    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted contact is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5). Add the '
                         'contact again as a new row.';
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
-- @entity: ExternalContact
-- @scope:  tenant
--
-- external_contacts — one body outside the commune's apparatus that residents of ONE commune may
-- call, as the commune entered it. Public on the Mini App while not soft-deleted.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS external_contacts (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,          -- ULID, internal

    name           TEXT        NOT NULL,          -- "Trạm Y tế xã"
    category       TEXT        NOT NULL,          -- group heading, free text: "Y tế", "Công an"
    phone          TEXT        NOT NULL,
    address        TEXT,
    display_order  INT,                           -- ascending; NULL sorts last

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Business codes (rule 6, invariant 8) — `CB-…`, never an internal id.
    created_by     TEXT        NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),

    CONSTRAINT external_contacts_name_shape CHECK (
        btrim(name) <> '' AND char_length(name) <= 255 AND name !~ '[[:cntrl:]]'),
    CONSTRAINT external_contacts_category_shape CHECK (
        btrim(category) <> '' AND char_length(category) <= 100 AND category !~ '[[:cntrl:]]'),
    CONSTRAINT external_contacts_phone_shape CHECK (
        btrim(phone) <> '' AND char_length(phone) <= 32
        AND phone ~ '^[0-9+(). -]+$'
        AND char_length(regexp_replace(phone, '[^0-9]', '', 'g')) BETWEEN 3 AND 15),
    -- NULL = no address. An all-blank address is NULL's job, not a second spelling of it.
    CONSTRAINT external_contacts_address_shape CHECK (
        address IS NULL
        OR (btrim(address) <> '' AND char_length(address) <= 500 AND address !~ '[[:cntrl:]]')),
    CONSTRAINT external_contacts_display_order_non_negative CHECK (
        display_order IS NULL OR display_order >= 0),
    CONSTRAINT external_contacts_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> ''),
    -- Rule 7, invariant 1: all three soft-delete columns, or none.
    CONSTRAINT external_contacts_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS external_contacts_p%s PARTITION OF external_contacts '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The public read (and the staff list): one commune's live contacts in display order, id as the
-- tie-break. Leads with tenant_id (rule 1); soft-deleted rows excluded (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS external_contacts_list
    ON external_contacts (tenant_id, display_order, id)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS external_contacts_guard ON external_contacts;
CREATE TRIGGER external_contacts_guard
    BEFORE UPDATE OR DELETE ON external_contacts
    FOR EACH ROW EXECUTE FUNCTION external_contacts_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (ADR 0013's line).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM external_contacts;
--
-- ZERO → in this order:
--
--   DROP TABLE external_contacts;      -- partitions, index, trigger go with it
--   DROP FUNCTION external_contacts_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → a commune's entered contacts, and what its residents were shown. Dropping them is rule 7
-- stop condition #2 — the user plus a verified backup, never a command — and the way back is a NEW
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
