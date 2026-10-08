-- 0033 — menu Cấu hình after ADR 0079 (owner answers of 08/10/2026), the petitions half. Three
-- additive changes, one file, one transaction:
--
--   A. `system_message_override.is_active` — "Tắt / Bật lại" on a reworded sentence (ADR 0079 Q2).
--   B. `custom_system_message` — sentences a commune adds itself, groups "Phản ánh" and "Dùng chung"
--      (ADR 0079 Q2, Q5a, Q5b).
--   C. `color` on the two task catalogues `loai_nhiem_vu`, `muc_uu_tien_nhiem_vu` (ADR 0079 row 5, Q1 #9).
--
-- ---------------------------------------------------------------------------
-- THIS FILE REPLACES A DECISION WRITTEN IN 0020, AND SAYS SO HERE BECAUSE 0020 CANNOT BE EDITED.
--
-- 0020:9-17 records the user decision of 28/09/2026: "a commune may reword a shipped sentence; it
-- may not remove one and may not invent one ... there is no `+ Thêm câu mới`", and "Khôi phục câu
-- mặc định (not Tắt)". The project owner REPLACED that on 08/10/2026 (ADR 0079 Q2, "Làm đúng
-- prototype"): a commune MAY add sentences, MAY switch a reworded sentence off and back on (keeping
-- its wording; while off, the software's sentence is used), and MAY soft delete a sentence it added.
-- 0020 is applied and core/migrate refuses an edited file (ErrChecksumLech), so its comment stays as
-- written; where the two disagree, ADR 0079 and this file win.
--
-- WHAT 0020 STILL GOVERNS, UNCHANGED: the closed CHECK on `system_message_override.message_key`. An
-- override still rewords only a key this service RAISES. A commune's own sentence is a different
-- entity with its own table (B), not a widening of that CHECK — otherwise a typo in a commune key
-- would become a silent "override" of a sentence no code shows.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: A touches every row of `system_message_override` (at most six live per
--      commune, plus reverted ones) — as a catalogue-only change: ADD COLUMN with a constant DEFAULT
--      is metadata-only on PostgreSQL 11+, no row is rewritten. B is a new empty table. C adds a
--      nullable column to two catalogues (a handful of rows per commune), again metadata-only.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF
--      EXISTS, so a retry costs nothing. No data is backfilled, so there is no per-commune loop to
--      resume (rule 7 invariant 5 is about backfills; core/migrate/migrate.go:16-26).
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER it lands, none changes meaning: every existing override reads
--      `is_active = true`, which is exactly what every override meant before this file; `color` is
--      NULL everywhere, which is "no colour chosen". The meaning changes only when the go-service-builder
--      card makes the readers honour `is_active` — from then on an inactive override must resolve to
--      the shipped sentence (domain.ResolveMessage), on the configuration list AND on every refusal
--      branch. A reader that keeps filtering only on `deleted_at IS NULL` would keep showing a
--      switched-off wording; that is the one read path to get right in that card.
--   5. RETENTION: nothing is removed. B is soft deleted only, and hard DELETE is refused by a trigger;
--      this file also attaches the same refusal to `system_message_override`, which had none.
-- ---------------------------------------------------------------------------


-- ===========================================================================
-- A. system_message_override.is_active — "Tắt / Bật lại".
--
-- A SWITCH, NOT A SECOND REVERT. "Khôi phục câu mặc định" (soft delete, 0020) ends the wording;
-- "Tắt" keeps it and stops using it, so "Bật lại" brings the SAME words back without retyping. The
-- inactive row still holds the live slot (`live_key` looks only at `deleted_at`), so a commune cannot
-- have one active and one inactive wording of the same key at once — which one would "Bật lại" restore?
--
-- NOT NULL DEFAULT true: every existing row was in force, and stays in force.
-- ===========================================================================
ALTER TABLE system_message_override
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- system_message_override_no_delete — 0020 relied on the store never issuing DELETE
-- (internal/store/system_message_override.go only UPDATEs). That is a promise of the application
-- layer; a psql session or the next writer never heard it (ADR 0013). A hard delete here would erase
-- a commune's wording history and drop the commune back on the default with no trail.
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
-- B. custom_system_message — a sentence the COMMUNE added ("Xã tự thêm").
--
-- @entity: PetitionsCustomSystemMessage
-- @scope:  tenant
--
-- WHY HERE: ADR 0079 Q5a — commune sentences of group "Phản ánh" AND of group "Dùng chung" live in
-- service-petitions; "Giải ngân" lives in service-finance (its own copy, finance 0017); "Báo cáo điều
-- hành" stays a group of its own and gets no commune sentences. The entity name carries the service
-- for the reason 0020 gives (tools/kb: one entity, one owner).
--
-- WHAT IT IS FOR TODAY: STORED AND MANAGED ONLY (ADR 0079 Q5b, "Chỉ lưu và quản lý, chưa hiện ra
-- đâu"). No code path shows these sentences to anybody yet, citizen or staff. The first caller that
-- does (ZNS, Mini App) is a later card and, if a citizen reads it, a rule 4 question at that time.
--
-- WHY A SEPARATE TABLE AND NOT ROWS IN system_message_override: an override's key is closed by a CHECK
-- that must agree with the code (0020); a commune key is open by definition. One table holding both
-- would have to drop that CHECK, and then a mistyped shipped key would be stored as a "commune
-- sentence" nothing raises. Two shapes, two tables. Column names follow 0020 (`message_key`,
-- `message_text`) so one concept keeps one English word (rule 12, forbidden #2).
-- ===========================================================================

-- custom_system_message_guard — what may never happen to a commune sentence, enforced in the database.
--
--   DELETE            soft delete only (rule 7, invariant 1).
--   message_key       an issued code is never renumbered (rule 7, invariant 3).
--   group_code        bound to the key's prefix (CHECK below); moving groups is a new sentence.
--   tenant_id, id     moving a row to another commune is cross-tenant writing (rule 1).
--   created_at/_by    "who first wrote this" must survive every later edit.
--   a deleted row     is closed: no edit, no undelete. A removed sentence stays exactly as it was
--                     removed — the record of what the commune once had.
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

    -- The Lời hệ thống group the sentence is listed under. Values per ADR 0011 (Vietnamese, no
    -- diacritics): `phan-anh` = "Phản ánh", `chung` = "Dùng chung". Finance holds `giai-ngan`.
    group_code     TEXT        NOT NULL,

    -- Chosen by the commune. THREE RULES, and why each:
    --   * lowercase slug segments joined by `.` or `-` — the shape of every key a URL or a template
    --     will carry; no space, no diacritic, no underscore.
    --   * MUST START WITH `<group_code>.` (e.g. `chung.loi-chao`). Shipped keys live under `feedback.`,
    --     `budget.`, `report.` (0020, finance 0010, reporting 0003), and finance's commune keys under
    --     `giai-ngan.` — so a commune key can NEVER equal a shipped key or another service's commune
    --     key, by construction, without either service reading the other's catalogue (rule 2). The
    --     app still refuses a collision with domain.shippedMessages first, for a readable message.
    --   * UNIQUE (tenant_id, message_key) COUNTS SOFT-DELETED ROWS. A deleted key is never reused
    --     (rule 7, invariant 3): once something refers to `chung.loi-chao` — a template, an export, an
    --     audit entry — a later, different `chung.loi-chao` would make that reference silently read
    --     the new wording. The cost: a commune that deletes a key must pick a new one.
    message_key    TEXT        NOT NULL,

    -- Same floor as system_message_override.message_text (0020), for the same reasons: trimmed,
    -- 1..1000 characters, no control character, no `<` or `>` (rule 13, invariant 3).
    message_text   TEXT        NOT NULL,

    -- What the sentence is for, shown under the key on the configuration card. Optional.
    description    TEXT,

    -- "Tắt / Bật lại". An inactive commune sentence is kept, listed, and used by nothing.
    is_active      BOOLEAN     NOT NULL DEFAULT true,

    created_at     TIMESTAMPTZ NOT NULL,
    -- Who: the staff BUSINESS CODE (`CB-00123`), never the internal id — the same value
    -- audit_log.actor_id holds (rule 6, invariant 8).
    created_by     TEXT        NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    updated_by     TEXT        NOT NULL,

    -- Rule 7, invariant 1. Written only by "Xoá" on a commune sentence.
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id, and counting soft-deleted rows — see message_key above.
    UNIQUE (tenant_id, message_key),

    CONSTRAINT custom_system_message_group_known
        CHECK (group_code IN ('phan-anh', 'chung')),
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
    -- A blank "who" is a write nobody can be asked about.
    CONSTRAINT custom_system_message_actors_present
        CHECK (btrim(created_by) <> '' AND char_length(created_by) <= 64
               AND btrim(updated_by) <> '' AND char_length(updated_by) <= 64),
    -- A delete carries all three of its facts, or none of them. BOTH DIRECTIONS: a row with a who and
    -- a why but no `deleted_at` is a half-delete every `deleted_at IS NULL` reader would show as live.
    -- Written with IS NULL equalities, not `btrim(x) <> ''` alone, because a CHECK that evaluates to
    -- NULL PASSES.
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

-- The configuration card lists one commune's live sentences by group; deleted ones drop out (rule 7,
-- invariant 2). NOT unique — the uniqueness that matters is the table-level key above.
CREATE INDEX IF NOT EXISTS custom_system_message_list
    ON custom_system_message (tenant_id, group_code, message_key) WHERE deleted_at IS NULL;

-- Row-level, so PostgreSQL clones it onto every partition (PG >= 13, 0003's version check).
DROP TRIGGER IF EXISTS custom_system_message_guard ON custom_system_message;
CREATE TRIGGER custom_system_message_guard
    BEFORE UPDATE OR DELETE ON custom_system_message
    FOR EACH ROW EXECUTE FUNCTION custom_system_message_guard();


-- ===========================================================================
-- C. color on the task catalogues (ADR 0079 row 5; Q1 #9: editable on "Hệ thống" rows too — it is
-- presentation only).
--
-- NULL = no colour chosen; the screen picks its neutral chip. `#RRGGBB` only — no names, no alpha, no
-- `rgb()`: one spelling, and nothing that a renderer could read as CSS beyond a colour (rule 13 inv 3).
-- The app should store it lower-cased so one colour has one spelling; the CHECK accepts either case.
--
-- danh_muc_ba_tang (0003) needs no change: it guards `ma`, `nguon`, the tier flag, soft delete and
-- disabling — `color` is none of those, so it is editable on every tier, as Q1 #9 decided.
-- ===========================================================================
ALTER TABLE loai_nhiem_vu
    ADD COLUMN IF NOT EXISTS color TEXT
        CONSTRAINT loai_nhiem_vu_color_shape CHECK (color ~ '^#[0-9A-Fa-f]{6}$');

ALTER TABLE muc_uu_tien_nhiem_vu
    ADD COLUMN IF NOT EXISTS color TEXT
        CONSTRAINT muc_uu_tien_nhiem_vu_color_shape CHECK (color ~ '^#[0-9A-Fa-f]{6}$');


-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP OR DECIDE:
--   * TRUNCATE and DDL by the table owner — the line 0003 and ADR 0013 draw.
--   * Whether "Tắt" applies to a shipped sentence the commune never reworded. With no override row
--     there is nothing to switch off and the shipped sentence is already in force; the screen/use case
--     decides whether that button is shown. Nothing here lets a commune SILENCE a shipped refusal.
--   * Who may write any of this (the `admin.*` key) — the handler card, rule 5 invariant 3c.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Then remove this file's row from `schema_migration`.
--   B: lossless while `SELECT count(*) FROM custom_system_message` is 0 —
--        DROP TABLE custom_system_message; DROP FUNCTION custom_system_message_guard();
--      Once a commune has a row, dropping it destroys that commune's sentences: user decision (rule 7,
--      stop condition 2), not a command.
--   A: DROP TRIGGER system_message_override_no_delete ON system_message_override;
--      DROP FUNCTION system_message_override_no_delete();
--      Dropping `is_active` is lossless only while no row has is_active = false
--        (SELECT count(*) FROM system_message_override WHERE NOT is_active  -- must be 0);
--      otherwise it silently switches those wordings back ON — a user decision.
--   C: dropping `color` is lossless only while every row has color IS NULL; otherwise it discards
--      what communes chose — a user decision.
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
