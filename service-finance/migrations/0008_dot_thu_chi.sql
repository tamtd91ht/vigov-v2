-- finance — the batches of revenue/expenditure recorded against one budget line (`⇄ Các đợt thu,
-- chi`, docs/ui-ux/07-thu-chi-ngan-sach.md §5), and the third calculation mode `entries` (§4.2)
-- that sums them. User decision 25/09/2026 (ledger service-finance/thu-chi-ngan-sach-82).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: 0001..0007 have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas. 0006's own header says `entries` was POSTPONED until this table exists, and
-- that adding it is "one line of CHECK plus the table" — this file is that line and that table.
--
-- ---------------------------------------------------------------------------
-- WHAT THE USER DECIDED, AND WHAT THIS FILE THEREFORE STORES:
--
--   * A LEAF line chooses `manual` or `entries`. A line WITH children is always `children`
--     (customer decision 06/09/2026, unchanged — internal/domain.CachTinhTheoCay).
--   * Switching `entries` -> `manual` keeps the computed figures as the manual starting values
--     (§9 rule 1). That is a WRITE into `gia_tri_khoan_muc` done by the next card's use case, in
--     the same transaction as the switch. NOTHING here deletes, hides or rewrites a batch when the
--     mode changes: the batches stay, soft-delete-free, and simply stop being summed.
--   * One batch = date, content, "Đơn vị, cá nhân", document number, and ONE amount per NUMBER
--     column of the sheet (§5:140). Money in ĐỒNG, BIGINT, negative allowed, NULL = empty —
--     exactly `gia_tri_khoan_muc` (0006, MONEY block and "A VALUE MAY BE NEGATIVE").
--   * A batch is never edited (the specification draws no edit). A wrong batch is REMOVED — soft
--     delete with a reason (rule 7) — and recorded again.
--
-- ---------------------------------------------------------------------------
-- WHY THE AMOUNTS ARE A CHILD TABLE (`gia_tri_dot`) AND NOT §7's `gia_tri jsonb {cot_id: số}`.
--
--   1. TYPED MONEY. A jsonb number is a JSON number: nothing stops `1.5`, `1e20`, `"12"` or `null`
--      spelled three ways, and every reader has to cast. A BIGINT column refuses a fraction of a
--      đồng at the floor, the way every other amount in this service does (0004 MONEY block).
--   2. ONE SHAPE FOR ONE KIND OF FACT. A cell of the sheet is already `(khoan_muc_id, cot_id,
--      gia_tri BIGINT)` in `gia_tri_khoan_muc`. A batch amount is the same fact one level down.
--      The sum a line in `entries` mode shows is then one GROUP BY over rows of the same type,
--      and the §9.1 hand-over (entries -> manual) copies BIGINT into BIGINT with no parsing step
--      that could round, truncate or silently skip a key it did not recognise.
--   3. A KEY IN JSON IS A REFERENCE NOTHING CAN SEE. `cot_id` as a jsonb key cannot be indexed,
--      joined or checked non-empty; as a column it is all three, and the composite primary key
--      below makes "two amounts for one column in one batch" impossible rather than unlikely.
--
--   THE COST, stated: writing one batch is 1 + N inserts (N = number columns, at most
--   domain.SoCotToiDa = 30) instead of one. It is one transaction either way.
--
-- ---------------------------------------------------------------------------
-- ONLY NUMBER COLUMNS (`cot_ngan_sach.kieu = 'so'`) MAY CARRY AN AMOUNT, and THE DATABASE CANNOT
-- CHECK IT: `kieu` is on another table, and a CHECK constraint cannot read another table. Same
-- limit, same answer as `gia_tri_khoan_muc` (0006): the next card's use case refuses a percentage
-- column, a column of another sheet, and a column that is soft deleted — inside the transaction.
-- A trigger that looked the column up was considered and rejected: it would be the first
-- cross-table trigger in this service, on a hash-partitioned table, with no PostgreSQL reachable
-- here to verify it (VIGOV_TEST_DSN unset), and a migration that fails at startup stops the service.
--
-- ---------------------------------------------------------------------------
-- `don_vi_ca_nhan` IS PERSONAL DATA WHEN IT NAMES A PERSON (rule 3). "Đơn vị, cá nhân" is free text
-- and the commune WILL type a citizen's full name into it (a household paying a fee, a person
-- receiving support). Consequences the next card must honour, written here because the column is
-- born here:
--
--   * never in a log line, never in an error message, never in a URL, file name or cache key;
--   * masked in `audit_log` before/after values (rule 6, forbidden #4) — the audit entry records
--     THAT it was written, not what it said;
--   * the trigger below freezes it with the rest of the row. ⚠ THAT ALSO FREEZES IT AGAINST A
--     DECREE 13/2023 ERASURE REQUEST (anonymise, rule 7 invariant 7). No erasure path exists in
--     this repository yet; the day one is built, how it treats this column is a decision for the
--     user, made there — not by loosening this trigger in passing. Same caveat 0011 of
--     service-petitions writes for its free-text columns.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: this file writes NO row. It widens one CHECK on
--      `khoan_muc_ngan_sach` (validated against every existing row — all of which hold `manual` or
--      `children`, because 0006 admitted nothing else) and creates two empty tables. The ceiling in
--      sight: a few dozen batches per leaf line per year, each with at most 30 amounts.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction (core/migrate),
--      with the progress row written inside it. Every statement is IF NOT EXISTS, CREATE OR
--      REPLACE, DROP TRIGGER IF EXISTS, or guarded by a lookup of its own object — including the
--      constraint swap, which is a no-op on a retry once the widened form is in place.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none today, and the reason is a
--      fact about the code. NO WRITE PATH PRODUCES `entries`: `cach_tinh` is only ever written by
--      internal/domain.CachTinhTheoCay (manual | children) and internal/store.DatCachTinh with one
--      of those two. So after this file every existing line reads exactly as before, and the two
--      new tables have no reader.
--      The constraint swap itself opens NO WINDOW: DROP and ADD run inside one DO block inside the
--      file's transaction, under the ACCESS EXCLUSIVE lock ALTER TABLE takes, so no other session
--      ever observes the table without a CHECK on `cach_tinh`.
--      WHAT MOVES TO THE NEXT CARD, AND IS THE PART THAT CAUSES INCIDENTS:
--        (a) a line in `entries` mode must be summed from LIVE batches only — `dot_thu_chi.deleted_at
--            IS NULL` joined in; `gia_tri_dot` has no soft-delete columns of its own (see below), so
--            a sum that forgets the join counts removed batches;
--        (b) a leaf in `entries` mode that GAINS A CHILD is switched to `children` by the existing
--            use case (internal/app/thu_chi_ngan_sach.go, the `cha.CachTinh != TinhTheoCon` branch).
--            Its batches remain and stop counting. Whether that needs a warning, or a refusal while
--            live batches exist, is a question for the user — not something to decide in a trigger;
--        (c) a line whose last child is removed goes back to `manual` (same file), never to
--            `entries`, even if it had batches before it had children.
--   5. RETENTION: a batch is a record of money received or spent, and feeds a figure reported
--      upward — an ARCHIVAL RECORD (rule 7). Hence: no hard delete, no edit, soft delete once.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence reads as a decision:
--
--   FOREIGN KEYS        dot_thu_chi.khoan_muc_id -> khoan_muc_ngan_sach, gia_tri_dot.dot_id ->
--                       dot_thu_chi, gia_tri_dot.cot_id -> cot_ngan_sach are LOGICAL references,
--                       following the precedent 0003 set and 0004/0006/0007 restated: a real FK
--                       between HASH-partitioned tables is a form no PostgreSQL reachable from this
--                       build environment can verify. THE COST IS STATED: nothing in the database
--                       stops a batch naming a line that does not exist, or an amount naming a
--                       batch or column that does not. The next card's use case checks all three
--                       inside the transaction, with tenant_id bound on both sides of every lookup.
--   A BUSINESS CODE     §5 and §7 give a batch no `ma`, and none is invented here. The audit
--                       subject of a batch write is the SHEET's `ma` (`NS-2026-CHI-01`), the same
--                       choice 0006 made for a budget line (rule 6, invariant 8).
--   ANY UNIQUE KEY      beyond the primary keys. Two batches with the same date, content and amount
--                       are a real case (two instalments of one fee on one day) and refusing the
--                       second would be the software deciding a commune's bookkeeping.
--   `cap_nhat_luc`      a batch is never updated except to be soft deleted, and `deleted_at` is that
--                       instant. A column that could only ever equal `tao_luc` or `deleted_at` is a
--                       second home for a fact that already has one.
--   `bang_id`           derivable through `khoan_muc_id`, and a line never moves between sheets
--                       (internal/app: `BangID` is absent from the line update). Copying it here
--                       would be a second place that can disagree.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor, for the same reason 0004..0007 state: BEFORE ... FOR EACH ROW
-- triggers on a PARTITIONED table were only allowed from 13, and both new tables below declare one.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'budget batches need PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what stop an archival '
            'record from being destroyed or silently rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- khoan_muc_ngan_sach.cach_tinh — WIDENED to admit `entries`.
--
-- ADDING A VALUE TO AN IN-LIST IS LOSSLESS: every row valid under ('manual', 'children') is valid
-- under ('manual', 'entries', 'children'), so the re-validation ADD CONSTRAINT performs cannot fail
-- on data 0006 admitted. No column is dropped or retyped; no row is touched.
--
-- WHY DROP + ADD UNDER THE SAME NAME, and not a second constraint beside the old one: two CHECKs
-- are ANDed, so a new wider one next to 0006's narrower one would still refuse `entries`. The name
-- is kept because internal/domain and this directory's tests refer to it.
--
-- THE GUARD: `conrelid` pins the lookup to the PARENT table. Partitions inherit the CHECK under the
-- same name, so a lookup by `conname` alone matches 33 rows. `pg_get_constraintdef` tells a retry
-- apart from a first run: once the definition already names `entries`, the block does nothing.
-- If the constraint is ABSENT (somebody dropped it by hand), the block adds it — the fail-closed
-- outcome: the floor is restored, and any row outside the three values stops this file.
--
-- NOT VALID IS DELIBERATELY NOT USED: it is not a form the partitioned-table path accepts on every
-- supported PostgreSQL, and validation over a table this size costs nothing.
-- ---------------------------------------------------------------------------
DO $$
DECLARE dinh_nghia text;
BEGIN
    SELECT pg_get_constraintdef(oid) INTO dinh_nghia
    FROM pg_constraint
    WHERE conrelid = 'khoan_muc_ngan_sach'::regclass
      AND conname = 'khoan_muc_ngan_sach_cach_tinh_hop_le';

    IF dinh_nghia IS NULL OR position('entries' IN dinh_nghia) = 0 THEN
        IF dinh_nghia IS NOT NULL THEN
            ALTER TABLE khoan_muc_ngan_sach DROP CONSTRAINT khoan_muc_ngan_sach_cach_tinh_hop_le;
        END IF;
        ALTER TABLE khoan_muc_ngan_sach ADD CONSTRAINT khoan_muc_ngan_sach_cach_tinh_hop_le
            CHECK (cach_tinh IN ('manual', 'entries', 'children'));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- dot_thu_chi_bat_bien — a batch, once written, never changes; it may be soft deleted ONCE.
--
-- DENY BY DEFAULT, NOT AN ALLOWLIST BY OMISSION. 0004's `chung_tu_da_khoa` lists the frozen columns
-- one by one and writes down its own weakness: "A COLUMN ADDED LATER AND NOT ADDED HERE IS AN
-- EDITABLE LOCKED FIELD" (0007 had to come back and add one). This guard compares the WHOLE ROW
-- minus the three soft-delete columns, so a column a later migration adds is frozen the day it
-- exists, with nobody having to remember this function.
--
-- WHAT IT ALLOWS: exactly one UPDATE per row — NULL -> value on all three of deleted_at,
-- deleted_by, delete_reason (the all-or-none CHECK on the table keeps them together).
-- WHAT IT REFUSES: any change to any other column; any change to the trio once set — so no
-- undelete and no rewriting of who removed it or why.
--
-- Messages name the relation and the operation only. This row can carry a person's name
-- (`don_vi_ca_nhan`), and an error message travels into logs and back to clients (rule 3,
-- forbidden #3) — so no column VALUE is ever interpolated.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION dot_thu_chi_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(NEW) - 'deleted_at' - 'deleted_by' - 'delete_reason')
       IS DISTINCT FROM
       (to_jsonb(OLD) - 'deleted_at' - 'deleted_by' - 'delete_reason') THEN
        RAISE EXCEPTION 'archival record %: a recorded batch cannot be edited', TG_TABLE_NAME
            USING HINT = 'A batch of revenue/expenditure is an archival record (rule 7). To '
                         'correct one, soft delete it with a reason and record it again — two '
                         'audited acts, never a silent edit.';
    END IF;

    IF OLD.deleted_at IS NOT NULL
       AND (NEW.deleted_at    IS DISTINCT FROM OLD.deleted_at
         OR NEW.deleted_by    IS DISTINCT FROM OLD.deleted_by
         OR NEW.delete_reason IS DISTINCT FROM OLD.delete_reason) THEN
        RAISE EXCEPTION 'archival record %: a removed batch stays removed', TG_TABLE_NAME
            USING HINT = 'Who removed a batch, when, and why is recorded once. Record the batch '
                         'again instead of restoring or re-stating the removal.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- gia_tri_dot_bat_bien — an amount of a batch is never updated and never deleted.
--
-- NO SOFT DELETE HERE, ON PURPOSE: an amount is a figure ON a record, and the record is the batch,
-- which carries the three columns (same reasoning as gia_tri_khoan_muc in 0006). Removing a batch
-- removes its amounts from every sum by the batch's `deleted_at` — see migration question 4 (a).
-- Unlike gia_tri_khoan_muc, there is no "clear the cell" either: a batch has no edit, so an amount
-- written once is the amount forever.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION gia_tri_dot_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival record %: % refused — amounts of a recorded batch are immutable',
        TG_TABLE_NAME, TG_OP
        USING HINT = 'To correct an amount, soft delete the batch (dot_thu_chi) with a reason and '
                     'record it again. Its amounts then drop out of every sum with it.';
END $$;

-- ---------------------------------------------------------------------------
-- @entity: BudgetEntry
-- @scope:  tenant
--
-- dot_thu_chi — one batch of revenue or expenditure recorded against ONE leaf budget line (§5, §7).
--
-- The line it belongs to must be a LEAF in `entries` mode when the batch is written, and the sheet
-- must be live. None of that is checkable here (other tables); the next card's use case refuses it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dot_thu_chi (
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,
    khoan_muc_id    TEXT        NOT NULL,
    -- The day of the batch as the commune states it (§5 "Ngày", defaulting to today on the SCREEN —
    -- not here: a database default would record the day the row was written, which is a different
    -- fact from the day the money moved).
    ngay            DATE        NOT NULL,
    -- "Thu tiền sử dụng đất đợt 2".
    noi_dung        TEXT        NOT NULL,
    -- "Đơn vị, cá nhân". PERSONAL DATA when it names a person — see the header block. NULL when not
    -- stated; '' is refused below so "not stated" has one spelling.
    don_vi_ca_nhan  TEXT,
    -- "Số chứng từ". NULL when not stated, never ''.
    so_chung_tu     TEXT,
    -- WHO RECORDED IT, as a STAFF BUSINESS CODE (`CB-00123`), never the internal id — the same
    -- policy as the audit trail (rule 6, invariant 8). §7 calls it `nguoi_ghi_id`; the name is
    -- changed so the column cannot be read as licence to store `Principal.ID`.
    nguoi_ghi_ma    TEXT        NOT NULL,
    tao_luc         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      TEXT,
    delete_reason   TEXT,
    -- Composite with tenant_id (rule 1, invariant 6). The only key — see "ANY UNIQUE KEY" above.
    PRIMARY KEY (tenant_id, id),
    -- AN EMPTY STRING IS NOT "NO REFERENCE", IT IS A BROKEN ONE (0007's wording): '' joins to no
    -- line and the batch vanishes from every sum while looking recorded.
    CONSTRAINT dot_thu_chi_khoan_muc_co_that CHECK (btrim(khoan_muc_id) <> ''),
    -- Same window as `bang_ngan_sach.nam` (0006). A typo year (0226, 20226) would put the batch
    -- outside every report while it sits in the table looking healthy.
    CONSTRAINT dot_thu_chi_ngay_hop_le CHECK (ngay BETWEEN DATE '2000-01-01' AND DATE '2100-12-31'),
    -- A batch nobody can describe is a line on §5's list nobody can match to a receipt.
    CONSTRAINT dot_thu_chi_noi_dung_khong_rong CHECK (btrim(noi_dung) <> ''),
    CONSTRAINT dot_thu_chi_don_vi_ca_nhan_khong_rong
        CHECK (don_vi_ca_nhan IS NULL OR btrim(don_vi_ca_nhan) <> ''),
    CONSTRAINT dot_thu_chi_so_chung_tu_khong_rong
        CHECK (so_chung_tu IS NULL OR btrim(so_chung_tu) <> ''),
    -- BOUNDED, IN CHARACTERS (char_length, not bytes — Vietnamese diacritics are 2-3 bytes each).
    -- The numbers are the ones the disbursement voucher already uses for the same three kinds of
    -- text (internal/domain/chung_tu_giai_ngan.go: NoiDungChungTuToiDa 1000, DoiTacToiDa 300,
    -- SoChungTuToiDa 100), so a commune meets one limit for one kind of text across the module.
    -- The next card's route must validate the same numbers with utf8.RuneCountInString so a client
    -- sees a 400, not a 500.
    CONSTRAINT dot_thu_chi_noi_dung_toi_da CHECK (char_length(noi_dung) <= 1000),
    CONSTRAINT dot_thu_chi_don_vi_ca_nhan_toi_da CHECK (char_length(don_vi_ca_nhan) <= 300),
    CONSTRAINT dot_thu_chi_so_chung_tu_toi_da CHECK (char_length(so_chung_tu) <= 100),
    -- A write nobody can be named for is a write nobody answers for (rule 6, invariant 8).
    CONSTRAINT dot_thu_chi_nguoi_ghi_khong_rong CHECK (btrim(nguoi_ghi_ma) <> ''),
    -- SOFT DELETE IS ALL OR NONE. A removal with no reason, or a reason with no removal, is half a
    -- fact — and `deleted_at` alone decides visibility, so a stray `deleted_by` on a live row would
    -- read as a removal to one reader and not to another. 500 = domain.LyDoXoaNganSachToiDa.
    CONSTRAINT dot_thu_chi_xoa_mem_du_ba CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL
            AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
            AND delete_reason IS NOT NULL AND btrim(delete_reason) <> '')),
    CONSTRAINT dot_thu_chi_ly_do_xoa_toi_da CHECK (char_length(delete_reason) <= 500)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS dot_thu_chi_p%s PARTITION OF dot_thu_chi '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The two reads there are, both within one commune and one line: §5's list of batches (newest
-- first) and the per-line sum an `entries` line shows. Partial on deleted_at like every index in
-- this service (rule 7, invariant 2); NOT unique, so check_khoa_duy_nhat has nothing to say.
CREATE INDEX IF NOT EXISTS dot_thu_chi_theo_khoan_muc
    ON dot_thu_chi (tenant_id, khoan_muc_id, ngay DESC, tao_luc DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS dot_thu_chi_cam_xoa_cung ON dot_thu_chi;
CREATE TRIGGER dot_thu_chi_cam_xoa_cung
    BEFORE DELETE ON dot_thu_chi
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS dot_thu_chi_bat_bien ON dot_thu_chi;
CREATE TRIGGER dot_thu_chi_bat_bien
    BEFORE UPDATE ON dot_thu_chi
    FOR EACH ROW EXECUTE FUNCTION dot_thu_chi_bat_bien();

-- ---------------------------------------------------------------------------
-- @entity: BudgetEntryAmount
-- @scope:  tenant
--
-- gia_tri_dot — one amount of one batch, in one NUMBER column of the sheet (§5:140).
--
-- EMPTY IS `gia_tri IS NULL`, and a column with NO row is empty too. Both read the same: SUM skips
-- NULL, and SUM over no rows is NULL — which the screen draws as `—`, not `0` (§9 rule 4). So the
-- line's figure for a column is NULL exactly when no live batch stated an amount there, and 0 only
-- when the stated amounts really add up to 0.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gia_tri_dot (
    tenant_id  TEXT        NOT NULL,
    dot_id     TEXT        NOT NULL,
    cot_id     TEXT        NOT NULL,
    -- ĐỒNG. BIGINT, never floating point. Negative allowed (0006: "A VALUE MAY BE NEGATIVE").
    gia_tri    BIGINT,
    tao_luc    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The composite primary key IS the uniqueness this table needs: one amount per (batch, column),
    -- inside one commune (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, dot_id, cot_id),
    CONSTRAINT gia_tri_dot_dot_co_that CHECK (btrim(dot_id) <> ''),
    CONSTRAINT gia_tri_dot_cot_co_that CHECK (btrim(cot_id) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS gia_tri_dot_p%s PARTITION OF gia_tri_dot '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- No extra index: every read reaches amounts through their batch, and the primary key
-- (tenant_id, dot_id, cot_id) already serves "all amounts of these batches".

-- ONE trigger for both operations: this table refuses UPDATE and DELETE alike, so the shared
-- ho_so_luu_tru_cam_xoa_cung (DELETE only) would cover half of it.
DROP TRIGGER IF EXISTS gia_tri_dot_bat_bien ON gia_tri_dot;
CREATE TRIGGER gia_tri_dot_bat_bien
    BEFORE UPDATE OR DELETE ON gia_tri_dot
    FOR EACH ROW EXECUTE FUNCTION gia_tri_dot_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, and DDL by the table owner (ALTER TABLE
-- ... DISABLE TRIGGER, dropping a constraint). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). In ONE transaction, in this order:
--
--   1. CHECK FIRST that nothing depends on `entries`:
--        SELECT count(*) FROM khoan_muc_ngan_sach WHERE cach_tinh = 'entries';   -- must be 0
--        SELECT count(*) FROM dot_thu_chi;                                        -- must be 0
--      If either is not zero, STOP: see below.
--   2. DROP TABLE gia_tri_dot; DROP TABLE dot_thu_chi;
--      (their 64 partitions, one index and three triggers go with them)
--   3. DROP FUNCTION gia_tri_dot_bat_bien(); DROP FUNCTION dot_thu_chi_bat_bien();
--      ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's and other tables are attached to it.
--   4. ALTER TABLE khoan_muc_ngan_sach DROP CONSTRAINT khoan_muc_ngan_sach_cach_tinh_hop_le;
--      ALTER TABLE khoan_muc_ngan_sach ADD CONSTRAINT khoan_muc_ngan_sach_cach_tinh_hop_le
--          CHECK (cach_tinh IN ('manual', 'children'));
--      (0006's exact definition; it re-validates, and fails loudly if step 1 was skipped)
--   5. DELETE this file's row from `schema_migration` — the runner's own bookkeeping table, not
--      business data — otherwise the runner still believes the schema is in place.
--
-- THAT IS A COMPLETE, LOSSLESS REVERSAL ONLY WHILE STEP 1 RETURNS TWO ZEROS — true in every
-- environment the day this is written, since no route writes a batch or the `entries` mode yet.
-- ONCE A COMMUNE HAS RECORDED ONE BATCH, step 2 DESTROYS BUDGET RECORDS that feed a document sent
-- to a higher authority: rule 7's first stop condition, which needs the user and a verified backup,
-- not a command. From that point the way back is a NEW migration (core/migrate has no automatic
-- rollback — ADR 0013). Setting the lines back to `manual` first is itself a figure change (§9.1)
-- and must go through the use case, audited, never through an UPDATE typed here.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. Repeated at the end of every migration that
-- declares a partitioned table, because it only verifies the state after a file that CARRIES it.
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
