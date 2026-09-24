-- 0010 — a commune's own LABEL and ORDER for the seven task statuses (open question #21,
-- DECIDED — ADR 0035 §C).
--
-- The decision, verbatim from kb/00-foundation/open-questions.json #21: "xã sửa được NHÃN và THỨ
-- TỰ, KHÔNG sửa được DANH SÁCH MÃ. Nhóm danh mục này KHÔNG có nút `Tắt`". This file is the schema
-- half of it. The default labels and order live in docs/ui-ux/02-nhiem-vu.md §6 and, as the
-- fallback a screen shows when a commune has set nothing, in the web client (user decision
-- 2026-09-24: the task screen READS this table; the hard-coded labels are only the fallback).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied
-- file at startup. Editing 0006 either stops the service (ErrChecksumLech) or leaves two
-- databases claiming one schema version while holding two schemas.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks directly above CREATE
-- TABLE (ADR 0021); tools/kb reads them there.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero now, at most seven ever — one per status code, enforced by
--      the primary key together with the CHECK on `ma`. This file writes no row: NO ROW MEANS
--      "SHOW THE DEFAULT", so there is nothing to seed, and the step that would seed per commune
--      (onboarding) does not exist in this repository (0003's header).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE /
--      DROP TRIGGER IF EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS a
--      table, a trigger function and a trigger. `nhiem_vu.trang_thai` and its CHECK (0006) are
--      untouched, no row changes visibility, and no existing query reads the new table. The risk
--      moves to the next card: a task screen that reads this table must fall back to the default
--      for every code WITHOUT a row — which today is every code of every commune.
--   5. RETENTION: this table holds CONFIGURATION, not an archival record — see "WHY NO
--      SOFT-DELETE COLUMNS" below. No task record references a row here: tasks hold the status
--      CODE, which is fixed by 0006's CHECK and never by this table.
--
-- ---------------------------------------------------------------------------
-- WHY THE CODE LIST IS FIXED, AND WHY IT IS FIXED HERE AS WELL AS IN 0006.
--
-- A status is a node of a state machine with fixed edges (docs/ui-ux/02-nhiem-vu.md §6; `:235`
-- binds `task.approve` to exactly one of them). A code a commune adds is a state with NO WAY IN
-- AND NO WAY OUT: nothing in the source code transitions into it, so no task ever reaches it, and
-- a label written for it is a label nobody sees. The reverse — letting communes mint codes and
-- collecting them back later — means hand-mapping the status of CLOSED tasks, i.e. editing
-- archival records (open question #21, `reversal_cost`).
--
-- So `ma` is CHECK-constrained to the same seven codes as `nhiem_vu_trang_thai_hop_le`
-- (0006_nhiem_vu.sql:340-342), not merely validated by the application. A label for a code the
-- state machine does not have must be impossible to STORE, not just unlikely to be written.
-- migrations/nhan_trang_thai_nhiem_vu_test.go compares the two lists on every `go test`, so the
-- day a migration changes one without the other, a test goes red.
--
-- AND THERE IS NO `dang_dung` / `Tắt` COLUMN (ADR 0035 §C, consequence): taking a status out of
-- use while tasks sit in it makes those tasks drop out of every filter. A column that cannot be
-- set to anything but "in use" is not written at all.
--
-- ---------------------------------------------------------------------------
-- WHY THE LABELS ARE PER COMMUNE: the wording a commune's staff read on their own Kanban is that
-- commune's administrative practice — the specification itself already calls `moi-giao` two
-- things (§6: "Mới giao", Kanban "Chưa thực hiện"). Same model as `nhan_linh_vuc` (0004, ADR 0026
-- tier 2): a fixed code set owned by the software, and a per-commune override of its WORDING.
--
-- ---------------------------------------------------------------------------
-- WHY NO SOFT-DELETE COLUMNS (`deleted_at`, `deleted_by`, `delete_reason`).
--
-- Rule 7 invariant 1 is about BUSINESS RECORDS — documents, petitions, tasks, disbursements —
-- which carry legal weight and are referenced by other records. A row here is neither: it is the
-- current value of a commune's display setting, and nothing points at it. Its history is the
-- audit trail of each write (rule 6, invariants 1 and 5: before/after, same transaction), which
-- the next card's write path must produce.
--
-- A soft-delete column here would also create a state with no meaning: a "deleted" label override
-- is indistinguishable from "no row", while still occupying the (tenant_id, ma) key. So the
-- lifecycle is: absent (default) -> present -> overwritten in place. "Reset to the default" is a
-- write of the default wording, audited like any other — NOT a delete. Hard DELETE is refused by
-- the trigger below rather than by convention, because a delete here would be a configuration
-- change that leaves no row to say who made it last.
--
-- ---------------------------------------------------------------------------
-- WHY NO UNIQUE (tenant_id, thu_tu).
--
-- Three reasons, any one of them sufficient:
--
--   * PARTIAL OVERRIDES. A commune that has set a row for three codes has four codes still at
--     their DEFAULT order, which lives outside the database. A unique key can only compare the
--     rows it sees, so it would refuse nothing that matters (moving `tam-dung` to 1 collides with
--     `moi-giao`'s default 1, which has no row) and would refuse things that are harmless.
--   * SWAPS. Reordering two statuses writes two rows; a non-deferred unique key refuses the first
--     statement of every swap, which invites the classic "park it at -1 first" workaround.
--   * NOTHING BREAKS ON A TIE. Order here is DISPLAY order only — no transition, count or
--     deadline reads it. A tie is resolved by a total tie-break on read (the default order of the
--     code), so it renders deterministically and never silently loses a column.
--
-- The next card's write path therefore writes the reordered set in ONE transaction.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE:
--
--   * No `id` column. The row's identity IS (tenant_id, ma): one commune, one status, one
--     override. A surrogate id would add a second key that admits nothing the first does not.
--   * No index beyond the primary key. A commune has at most seven rows, and every read is "all
--     rows of this commune", which the primary key (tenant_id, ma) already serves.
--   * No seed. See question 1.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table are only allowed from PostgreSQL 13. On
-- 11 and 12 the CREATE TRIGGER below fails with a message that reads like a syntax mistake and
-- invites moving the trigger onto the partitions — where a partition added later arrives
-- silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'nhan_trang_thai_nhiem_vu needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the trigger below is what keeps a row '
            'bound to the status code it was written for.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- nhan_trang_thai_nhiem_vu_bat_bien — refuses hard DELETE, and refuses moving a row to another
-- commune or another status code.
--
-- WHY `ma` IS IMMUTABLE even though both values would pass the CHECK: an UPDATE that turns the
-- `cho-duyet` row into the `hoan-thanh` row silently moves one commune's wording onto a different
-- state, and the audit entry of that write would describe a relabel that never happened. The
-- write path upserts on (tenant_id, ma); it never needs to change either.
--
-- WHY `tenant_id` IS IMMUTABLE: a row moved to another commune is one public authority's
-- configuration written into another's (rule 1).
--
-- A SEPARATE FUNCTION rather than reusing `danh_muc_ba_tang` (0003): that guard requires the
-- catalogue shape (nguon, ma_nguon_re_nhanh, dang_dung, deleted_at) this table deliberately does
-- not have, and its hints speak of soft delete — the wrong instruction to hand someone here.
--
-- Messages name the operation and the relation only; an error message travels into logs and back
-- to clients (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhan_trang_thai_nhiem_vu_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'configuration %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'To return a status to its default wording, write the default label '
                         'and order back. The write is audited like any other; a delete would '
                         'leave no row saying who changed the setting last.';
    END IF;

    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION 'configuration %: `tenant_id` is immutable', TG_TABLE_NAME
            USING HINT = 'A row moved to another commune is one authority''s configuration '
                         'written into another''s (rule 1).';
    END IF;

    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        RAISE EXCEPTION 'configuration %: `ma` is immutable', TG_TABLE_NAME
            USING HINT = 'A row labels exactly one status code. Upsert on (tenant_id, ma) to '
                         'change the wording; never move the row to another status.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: TaskStatusLabel
-- @scope:  tenant
--
-- nhan_trang_thai_nhiem_vu — one commune's wording and display order for ONE task status code.
-- A row is an OVERRIDE: a code with no row shows the default from docs/ui-ux/02-nhiem-vu.md §6.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhan_trang_thai_nhiem_vu (
    tenant_id     TEXT        NOT NULL,

    -- THE SEVEN CODES OF §6, identical to 0006's `nhiem_vu_trang_thai_hop_le` — see the header.
    ma            TEXT        NOT NULL,

    -- The commune's wording: "Chưa thực hiện" for `moi-giao`, say.
    nhan          TEXT        NOT NULL,

    -- Display position among the seven, 1 first. Display only — see "WHY NO UNIQUE (tenant_id,
    -- thu_tu)" in the header.
    thu_tu        INT         NOT NULL,

    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- WHO LAST SET IT, as a STAFF BUSINESS CODE (`CB-00123`), never the internal id — the same
    -- policy as the audit trail (rule 6, invariant 8). Convenience for the configuration screen
    -- ("sửa lần cuối bởi …"); the evidentiary record is the audit entry, not this column.
    cap_nhat_boi  TEXT        NOT NULL,

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6). One override per status per commune: two
    -- rows for `cho-duyet` would make the label a commune sees depend on read order.
    PRIMARY KEY (tenant_id, ma),

    CONSTRAINT nhan_trang_thai_nhiem_vu_ma_hop_le CHECK (ma IN (
        'moi-giao', 'da-tiep-nhan', 'dang-thuc-hien', 'cho-duyet', 'hoan-thanh',
        'tam-dung', 'chuyen-tiep')),

    -- A blank label renders as an empty Kanban column header — a status nobody can name.
    CONSTRAINT nhan_trang_thai_nhiem_vu_nhan_khong_rong CHECK (btrim(nhan) <> ''),

    -- BOUNDED, IN CHARACTERS: char_length counts characters, not bytes, so Vietnamese diacritics
    -- (2-3 bytes each in UTF-8) do not shorten the allowance. 100 is far above any status label
    -- in the specification and well below what stops being a label; the write path must validate
    -- the same bound with utf8.RuneCountInString so a client sees a 400, not a 500.
    CONSTRAINT nhan_trang_thai_nhiem_vu_nhan_toi_da CHECK (char_length(nhan) <= 100),

    -- Position 1 is first. Zero or negative is a position no screen draws.
    CONSTRAINT nhan_trang_thai_nhiem_vu_thu_tu_tu_mot CHECK (thu_tu >= 1),

    -- A setting nobody can be named for is a change nobody answers for (rule 6, invariant 8).
    CONSTRAINT nhan_trang_thai_nhiem_vu_cap_nhat_boi_khong_rong CHECK (btrim(cap_nhat_boi) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhan_trang_thai_nhiem_vu_p%s PARTITION OF nhan_trang_thai_nhiem_vu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS nhan_trang_thai_nhiem_vu_bat_bien ON nhan_trang_thai_nhiem_vu;
CREATE TRIGGER nhan_trang_thai_nhiem_vu_bat_bien
    BEFORE UPDATE OR DELETE ON nhan_trang_thai_nhiem_vu
    FOR EACH ROW EXECUTE FUNCTION nhan_trang_thai_nhiem_vu_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, and DDL by the table owner (ALTER TABLE
-- ... DISABLE TRIGGER, dropping the table). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). Every object here is new. In one transaction: drop the table
-- `nhan_trang_thai_nhiem_vu` (its 32 partitions and its trigger go with it), drop the function
-- `nhan_trang_thai_nhiem_vu_bat_bien()`, and remove this file's row from `schema_migration` —
-- otherwise the runner still believes the schema is in place. No other table references this
-- one, so nothing else needs resetting.
--
-- WHAT A REVERSAL COSTS ONCE COMMUNES HAVE WRITTEN ROWS: every commune's screens go back to the
-- default wording and order, and the settings they chose are gone from the table (their history
-- survives in the audit trail). No archival record is touched — tasks hold the status CODE, not
-- this row. It is still a change to data a commune entered, so it is the user's call, not a
-- command's; from that point the way back is a NEW migration (core/migrate has no automatic
-- rollback — ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that write rolls back entirely. Repeated at the end of every migration that
-- declares a partitioned table, because it only verifies the state after a file that CARRIES it
-- (0002, §BACKSTOP).
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
