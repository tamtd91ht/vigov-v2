-- 0013 — funding sources become ONE CATALOGUE PER COMMUNE, and the amount granted to a source is
-- recorded PER BUDGET YEAR in a table of its own. User decision 06/10/2026, answering the two STOP
-- items 0007's header left open (0007:34-62) and the shape question behind them.
--
-- WHY A NEW FILE: 0001..0012 have been applied and core/migrate compares the checksum of every applied
-- file at startup (ErrChecksumLech). Editing 0007 in place would either stop the service or leave two
-- databases claiming one schema version while holding two schemas. 0007 stays exactly as it was; this
-- file is the change to it.
--
-- ---------------------------------------------------------------------------
-- WHAT THE USER DECIDED, AND WHERE EACH HALF LANDS BELOW:
--
--   1. A source is declared ONCE per commune, by name, and is shared by every budget year.
--      -> `nguon_von` loses `nam` and `tong_nguon`; it becomes the catalogue.
--   2. The amount granted ("vốn được giao") is recorded PER YEAR, separately, so entering 2027's
--      figure can never move a 2026 figure that has already been reported.
--      -> new table `funding_source_annual_amounts`, one row per (commune, source, year).
--      THE USER EXPLICITLY REJECTED the prototype's single year-agnostic total, and the reason is the
--      reason 0007 gave for putting `nam` on the row: one total shared by every year is the
--      denominator of every year's progress bars, so revising it rewrites reports already read.
--      This file keeps that guarantee while moving it off the catalogue row.
--   3. (a) A source NAME is unique within a commune: (tenant_id, ten).
--      (b) A project holds at most ONE allocation line per source: (tenant_id, du_an_id, nguon_von_id).
--      The year amount is unique per (tenant_id, funding_source_id, year).
--
-- BOTH UNIQUE KEYS COUNT SOFT-DELETED ROWS, and the user accepted that knowing it:
--
--   (a) 0007:43-48 named the cost — a removed source could never be re-added under its name. Sources
--       have no remove and no rename (prototype behaviour, confirmed), so there is no removed row to
--       collide with. If a remove is ever built, the key does not loosen: re-adding means restoring
--       the removed row, which is also what keeps that source's history on one id.
--   (b) Allocation lines are soft deleted ONLY together with their project
--       (internal/store/du_an_ghi.go, xoaMemPhanBoCuaDuAn), and a removed project receives no new
--       line, so a counted dead line can never block a live one. No edit path for allocation lines
--       exists yet (app/du_an.go, YeuCauSuaDuAn says why); whoever builds one must UPDATE a line in
--       place rather than soft-delete-and-reinsert, or this key refuses the reinsert — loudly, which
--       is the right direction to fail.
--
-- Neither is partial, for the reason 0007:65-71 gives: tools/check_khoa_duy_nhat.py refuses a
-- partial unique index on deleted_at repository-wide.
--
-- A NAME IS COMPARED EXACTLY. "Ngân sách xã" and "ngân sách xã" are two names to this key. Folding
-- case would need an expression in the unique index, and PostgreSQL refuses expressions in a unique
-- index on a partitioned table. What IS closed here is the cheaper hole: a leading or trailing space
-- makes a second, invisible copy of a name, so `nguon_von_name_trimmed` refuses an untrimmed name at
-- the floor and the write path (TASK-02) has to trim before it inserts.
--
-- ---------------------------------------------------------------------------
-- THIS FILE DROPS TWO COLUMNS, AND IT IS ONLY ALLOWED TO BECAUSE THEY HOLD NOTHING.
--
-- Rule 7 forbids dropping a populated column without a backup and a user decision. The columns here
-- are empty in every commune, and that is a fact about the code, not a hope:
--
--   * no write route for `nguon_von` exists. internal/store/nguon_von.go was read-only by design
--     ("READ-ONLY, ON PURPOSE"), and nothing else in the service inserts into the table;
--   * 0007 seeded nothing (its question 1);
--   * so `phan_bo_nguon_von` is empty as well: its only writer (DuAnGhiStore.ChenPhanBo) runs after
--     the source-exists check, which finds no source and refuses.
--
-- A FACT ABOUT THE CODE IS NOT PROOF ABOUT A DATABASE — a psql session, an import nobody mentioned.
-- So the first statement below COUNTS `nguon_von`, soft-deleted rows included, and RAISES if there is
-- a single row. The whole file then rolls back and the service does not start: a refusal a person has
-- to read, instead of 2026's granted amounts silently gone. A populated table means the shape change
-- needs a BACKFILL into the new table first, written and decided separately.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: zero, on every object, in every commune — refused otherwise (above).
--      The new table starts empty; nothing is seeded. Ceiling in sight: a handful of sources per
--      commune (§6's sample has four) times one row per budget year.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate,
--      ADR 0013), with the progress row written inside it. Every statement is IF [NOT] EXISTS or
--      guarded by a lookup pinned to the parent table, so a retry after a failure costs nothing.
--      There is no backfill, so the per-commune resumable-progress question has no rows to resume.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. Complete while the new table is empty; one-way
--      once a commune has entered a granted amount.
--   4. WHICH READ PATHS CHANGE MEANING: no reader can see it half-applied. AFTER it, two things change,
--      and the second is the one that matters:
--        a. Every statement that named `nguon_von.nam` / `nguon_von.tong_nguon` FAILS against the new
--           schema — NguonVonStore.DanhSach / TienDoTheoNguon and the project write path's
--           source-in-year check in the PREVIOUS binary. On a rolling deploy an old replica answers
--           500 on those paths until it is replaced. Today that touches one live path only: creating
--           a project WITH allocation lines, which on an old replica was already refused (no source
--           exists) and becomes a 500 instead of a 404. Nothing reachable over HTTP reads the two §6
--           aggregates yet.
--        b. "đã phân bổ" and "đã giải ngân" per source WERE year-scoped by the source row itself
--           (a 2026 source only had 2026 lines). With one row serving every year, the year has to
--           come from the PROJECT (`du_an.nam`) — the store code shipped with this file filters by
--           it. A reader written against 0007's shape and not updated would total every year's
--           money onto one card, with nothing on the screen looking wrong. That is why the store
--           change and this file ship as one task.
--   5. RETENTION: no record is destroyed — the dropped columns are refused unless empty. The new
--      table holds the granted amount a year's figures are measured against, so it is archival:
--      no hard delete (0004's trigger). No business code is issued or renumbered (`nguon_von` has
--      no `ma`; §11).
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor, for the reason 0004..0008 give: the BEFORE ... FOR EACH ROW trigger on
-- the new partitioned table below. The cluster runs 16; nothing here needs more than 13.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'funding source amounts need PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the trigger below is what stops a granted amount '
            'from being destroyed.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- THE REFUSAL. Runs only while the old columns are still there, so a database that already carries
-- this file's shape is not re-examined. count(*) has NO deleted_at predicate on purpose: a
-- soft-deleted source still holds a year and an amount, and dropping them destroys that just the same.
-- The message names how many rows and how many communes, never a value (a name is commune data).
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    n_rows     bigint;
    n_communes bigint;
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema()
                 AND table_name = 'nguon_von'
                 AND column_name IN ('nam', 'tong_nguon')) THEN
        SELECT count(*), count(DISTINCT tenant_id) INTO n_rows, n_communes FROM nguon_von;
        IF n_rows > 0 THEN
            RAISE EXCEPTION
                'nguon_von holds % row(s) in % commune(s); 0013 would drop their year and granted '
                'amount', n_rows, n_communes
                USING HINT = 'Nothing has been changed. These rows need a backfill into '
                             'funding_source_annual_amounts (and a decision on how same-named '
                             'sources of different years merge) before this file may run — rule 7, '
                             'stop condition 2. Take a verified backup and ask the owner.';
        END IF;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- nguon_von — from "one source in one year" to "one source of one commune".
--
-- The index goes first because it names `nam`. The two CHECKs would go with their columns anyway;
-- they are dropped by name so the file says what it removes.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS nguon_von_danh_sach;

ALTER TABLE nguon_von DROP CONSTRAINT IF EXISTS nguon_von_tong_khong_am;
ALTER TABLE nguon_von DROP CONSTRAINT IF EXISTS nguon_von_nam_hop_le;
ALTER TABLE nguon_von DROP COLUMN IF EXISTS tong_nguon;
ALTER TABLE nguon_von DROP COLUMN IF EXISTS nam;

-- §6 lists one commune's sources in the commune's own display order — now without a year.
CREATE INDEX IF NOT EXISTS nguon_von_catalogue_order
    ON nguon_von (tenant_id, thu_tu) WHERE deleted_at IS NULL;

-- Decision 3(a). NOT partial — see the header. Composite with tenant_id, which is also the partition
-- key: the condition PostgreSQL sets for a unique index on a partitioned table (rule 1, invariant 6).
CREATE UNIQUE INDEX IF NOT EXISTS nguon_von_name_unique
    ON nguon_von (tenant_id, ten);

-- The space that would defeat the key above. Pinned to the PARENT (`conrelid`), as 0008/0011/0012
-- do: partitions inherit a CHECK under the same name, so `conname` alone matches 33 rows.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'nguon_von'::regclass
                     AND conname = 'nguon_von_name_trimmed') THEN
        ALTER TABLE nguon_von ADD CONSTRAINT nguon_von_name_trimmed CHECK (ten = btrim(ten));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- phan_bo_nguon_von — decision 3(b): one line per source per project.
--
-- This retires the "count DISTINCT so a duplicate cannot inflate a count" repair 0007 asked the read
-- path to carry. The read keeps COUNT(DISTINCT) anyway — it costs nothing and is still correct.
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS phan_bo_nguon_von_one_line_per_source
    ON phan_bo_nguon_von (tenant_id, du_an_id, nguon_von_id);

-- ---------------------------------------------------------------------------
-- @entity: FundingSourceAnnualAmount
-- @scope:  tenant
--
-- funding_source_annual_amounts — the amount granted to one funding source of one commune for one
-- budget year ("vốn được giao", §6). The denominator of the year's three progress bars.
--
-- ONE ROW PER (source, year), so revising 2027's figure is an UPDATE of the 2027 row and cannot reach
-- the 2026 row a report has already been read from (decision 2).
--
-- A CEILING, NOT A BALANCE — 0007's note on `tong_nguon` carries over unchanged: nothing here is
-- decremented as money is allocated or spent; both are derived from rows on each read.
--
-- A SOURCE WITH NO ROW FOR A YEAR HAS NOTHING GRANTED THAT YEAR. The read path shows it with 0 rather
-- than hiding it: the catalogue is the commune's list of sources, and a source does not vanish from
-- the screen because this year's figure has not been typed yet.
--
-- NO SOFT-DELETE COLUMNS, ON PURPOSE (0012 made the same call for the same kind of reason): there is
-- no "remove a year's amount" act. A wrong figure is corrected by an audited UPDATE carrying before
-- and after (rule 6, invariant 5); nothing granted is the figure 0. A `deleted_at` here would be a
-- second way to say "0", and the unique key below would then refuse the correction of a removed
-- year for ever. Hard DELETE is refused by the trigger like every archival table.
--
-- NOT FROZEN AFTER ENTRY. Whether a past year's granted amount may still be corrected — or only until
-- the year is closed (0012) — is the write path's question (TASK-02), not a schema guess.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS funding_source_annual_amounts (
    tenant_id          TEXT        NOT NULL,
    -- ULID.
    id                 TEXT        NOT NULL,
    -- `nguon_von.id`. LOGICAL reference with no FK, for the reason 0007:102-113 gives; every read
    -- joins on (tenant_id, funding_source_id) with the commune bound on both sides.
    funding_source_id  TEXT        NOT NULL,
    year               INT         NOT NULL,
    -- ĐỒNG. BIGINT, never floating point (0004's header).
    granted_amount     BIGINT      NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Decision 2: one figure per source per year. NOT partial — there is nothing to be partial on.
    UNIQUE (tenant_id, funding_source_id, year),
    -- '' is not "no source", it is a broken reference that joins to nothing (0007's note on
    -- phan_bo_nguon_von, the same reasoning).
    CONSTRAINT funding_source_annual_amounts_source_not_blank CHECK (btrim(funding_source_id) <> ''),
    -- The same window as du_an.nam (0004), bang_ngan_sach.nam (0006) and budget_period_closes.year.
    CONSTRAINT funding_source_annual_amounts_year_valid CHECK (year BETWEEN 2000 AND 2100),
    -- Zero is a real state (the source is known before its figure is decided); negative is a sign
    -- error that would drag the year's headline capital below the truth.
    CONSTRAINT funding_source_annual_amounts_amount_not_negative CHECK (granted_amount >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS funding_source_annual_amounts_p%s PARTITION OF funding_source_annual_amounts '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- §6 reads one commune's amounts for one year. (The unique key above serves the per-source lookup.)
CREATE INDEX IF NOT EXISTS funding_source_annual_amounts_by_year
    ON funding_source_annual_amounts (tenant_id, year);

DROP TRIGGER IF EXISTS funding_source_annual_amounts_no_hard_delete ON funding_source_annual_amounts;
CREATE TRIGGER funding_source_annual_amounts_no_hard_delete
    BEFORE DELETE ON funding_source_annual_amounts
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   1. CHECK FIRST:
--        SELECT count(*) FROM funding_source_annual_amounts;   -- must be 0
--        SELECT count(*) FROM nguon_von;                       -- must be 0
--      If either is not zero, STOP: see below.
--   2. Remove the new table funding_source_annual_amounts (its 32 partitions, index, unique key and
--      trigger go with it). ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's.
--   3. Remove the indexes phan_bo_nguon_von_one_line_per_source, nguon_von_name_unique and
--      nguon_von_catalogue_order, and the constraint nguon_von_name_trimmed.
--   4. Restore 0007's shape — on an EMPTY table NOT NULL needs no default:
--        ALTER TABLE nguon_von ADD COLUMN nam INT NOT NULL, ADD COLUMN tong_nguon BIGINT NOT NULL;
--        ALTER TABLE nguon_von ADD CONSTRAINT nguon_von_tong_khong_am CHECK (tong_nguon >= 0),
--                              ADD CONSTRAINT nguon_von_nam_hop_le CHECK (nam BETWEEN 2000 AND 2100);
--        CREATE INDEX nguon_von_danh_sach ON nguon_von (tenant_id, nam, thu_tu)
--            WHERE deleted_at IS NULL;
--      (Column ORDER differs from 0007's — `nam` and `tong_nguon` come last. Every reader names its
--      columns, so nothing depends on position.)
--   5. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
--   6. Roll the Go code back with the schema: the store shipped with this file reads the new table.
--
-- LOSSLESS ONLY WHILE STEP 1 RETURNS TWO ZEROS. Once a commune has entered a source and a year's
-- granted amount, going back means folding one catalogue row into one row PER YEAR — new ids, which
-- orphans every allocation line and voucher naming the old one. That is not a reversal, it is a data
-- migration of its own and rule 7's stop condition 2: the user and a verified backup, never a command.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (see 0007).
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
