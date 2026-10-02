-- 0028 — the commune's own petition lifecycle settings (menu phan-anh-nguoi-dan). Schema only: no row
-- is written.
--
-- WHAT IT HOLDS TODAY: ONE SWITCH, ADR 0008 "Đóng phiếu" decision 3 — `bat_buoc_anh_nghiem_thu`
-- (default TRUE): "thiếu ảnh sau xử lý thì không đóng được". A petition cannot be closed until it holds
-- at least one verification photo (purpose 'petition-verification-photo', 0027). Kept as the decided
-- default by the project owner on 02/10/2026, unlike ../vigov-require where the switch defaults off
-- (ADR 0008 :45-46).
--
-- WHY HERE AND NOT IN service-identity: owner decision of 02/10/2026. The switch is read by exactly one
-- act — closing a petition (app/xu_ly_phan_anh.go Dong) — and this service owns that act, so the read
-- stays a local query inside the closing transaction instead of a gRPC call on the closing path.
--
-- DESIGNED TO GROW. ADR 0008's other per-commune lifecycle switch, `bat_buoc_nguoi_khac_dong` (default
-- FALSE), has no store today; it joins this table as a NEW COLUMN in a later migration, with its own
-- NOT NULL DEFAULT equal to its decided default, so a commune that never saved anything keeps meaning
-- "the decided defaults". The reopen keys of ADR 0008 §Mở lại are NOT candidates: ADR 0050 (28/09/2026)
-- made reopening a fixed rule with no per-commune key.
--
-- NO ROW = EVERY SWITCH AT ITS DECIDED DEFAULT. This is the ordinary state of every commune until the
-- admin screen (a later card) saves one. The reader (store/petition_settings.go) answers "no row" with
-- the decided default, which for this column is TRUE — so the column DEFAULT below and the reader's
-- answer for an absent row are the same value, and must stay so: a row written with the column default
-- is indistinguishable from no row, which is the point.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): `bat_buoc_anh_nghiem_thu` is the ADR's design-time name; the
-- column is `verification_photo_required`, the same English word 0027 and core/storage use for the
-- photo ("verification photo").
--
-- AUDIT: every change to a row is an audit entry with before/after, written by the USE CASE in the same
-- transaction (rule 6, invariants 1 and 3). That use case is a later card; this file writes no row and
-- no code path writes one before it exists. `updated_by` below is the last writer's BUSINESS CODE, not
-- the history — the history is audit_log.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one, by construction — tenant_id IS the primary key. This file writes
--      no row; the table is empty for every commune after it. Nothing existing is altered.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. Every statement is CREATE … IF NOT EXISTS / CREATE OR REPLACE / DROP
--      TRIGGER IF EXISTS, so a retry costs nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied, and NO EXISTING
--      READ PATH CHANGES MEANING AFTER IT — the only reader is store.PetitionSettingsStore, and no use
--      case calls it yet. THE CARD THAT WIRES IT INTO Dong IS THE ONE THAT CHANGES BEHAVIOUR: from that
--      release, in EVERY commune with no row, an open petition holding no verification photo can no
--      longer be closed — including petitions already in progress on the day of the release. That is
--      the decided default (ADR 0008 decision 3, kept 02/10/2026), stated here so the release note says
--      it; it is not a side effect of this file.
--   5. RETENTION: configuration, not an archival record. Rows are never deleted (the guard below
--      refuses DELETE); "back to the default" is an UPDATE to the default value. The history of every
--      change is the audit entry (rule 6), retained with audit_log.
--
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- petition_settings_guard — refuses DELETE and a change of commune or of the creation stamp.
--
-- NO SOFT-DELETE COLUMNS, AND THAT IS A DECISION (comms 0008 mail_settings, same reasoning): one
-- configuration row per commune, overwritten in place. DELETE IS REFUSED because deleting a row is a
-- silent write: a commune that had switched the photo requirement OFF would be switched back ON with no
-- audit entry and no screen saying so. Returning to the default is an UPDATE, which the use case audits.
-- tenant_id is immutable: moving a commune's settings to another commune is cross-tenant writing.
-- created_at / created_by are immutable: they answer "who first configured this commune", and an
-- UPDATE rewriting them would erase the answer.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION petition_settings_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Return a commune to the default with an audited UPDATE, never a DELETE.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION '%: tenant_id is immutable', TG_TABLE_NAME;
    END IF;
    IF NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: created_at and created_by are immutable', TG_TABLE_NAME;
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PetitionSettings
-- @scope:  tenant
--
-- petition_settings — ONE ROW PER COMMUNE: that commune's choices among ADR 0008's petition lifecycle
-- switches. Read by store.PetitionSettingsStore; written only by the settings use case (later card).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS petition_settings (
    tenant_id                    TEXT        NOT NULL,

    -- ADR 0008 "Đóng phiếu" decision 3, `bat_buoc_anh_nghiem_thu` (Bắt buộc ảnh nghiệm thu / ảnh sau
    -- xử lý). TRUE = a petition cannot be closed without at least one verification photo. The DEFAULT
    -- is the decided default and equals the reader's answer for an absent row (header).
    verification_photo_required  BOOLEAN     NOT NULL DEFAULT true,

    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Who: the staff BUSINESS CODE (`CB-00123`), never the internal id — the same value
    -- audit_log.actor_id holds (rule 6, invariant 8).
    created_by                   TEXT        NOT NULL,
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by                   TEXT        NOT NULL,

    -- One row per commune (rule 1, invariant 6: the key is the commune itself).
    PRIMARY KEY (tenant_id),
    -- A blank "who" is a write nobody can be asked about. The shape of the code is the use case's.
    CONSTRAINT petition_settings_created_by_present CHECK (btrim(created_by) <> ''),
    CONSTRAINT petition_settings_updated_by_present CHECK (btrim(updated_by) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS petition_settings_p%s PARTITION OF petition_settings '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- Row-level, so PostgreSQL clones it onto every partition (PG >= 13).
DROP TRIGGER IF EXISTS petition_settings_guard ON petition_settings;
CREATE TRIGGER petition_settings_guard
    BEFORE UPDATE OR DELETE ON petition_settings
    FOR EACH ROW EXECUTE FUNCTION petition_settings_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP OR DECIDE:
--   * TRUNCATE and DDL by the table owner — the line 0003 and ADR 0013 draw (as comms 0008);
--   * which permission may change the switch, and the screen — the later card;
--   * what "has a verification photo" counts (stored / ready, not deleted) — the closing use case.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless WHILE THE TABLE IS EMPTY — check
--   SELECT count(*) FROM petition_settings;   -- must be 0
-- then:
--   DROP TABLE petition_settings;             (partitions and the trigger go with it)
--   DROP FUNCTION petition_settings_guard();
--   and remove this file's row from `schema_migration`.
-- ONCE A COMMUNE HAS A ROW, dropping the table silently returns that commune to the default — a
-- commune that switched the photo requirement OFF would find it ON. That is a change to data a commune
-- entered, so it is the user's call, not a command's; the audit entries keep what each commune chose.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP)
-- — repeated because this file declares a new PARTITION BY table.
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
