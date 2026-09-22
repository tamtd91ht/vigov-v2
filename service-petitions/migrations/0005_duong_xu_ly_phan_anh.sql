-- petitions — the STAFF processing path: the classification ceiling, the closing result, and the
-- outbox the citizen notification is published from.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: that file has been applied and core/migrate compares the
-- checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two different
-- schemas.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. `phieu_phan_anh` is EMPTY in every environment — no
--      commune has been onboarded, and the intake route has answered 409 `sla_chua_cau_hinh` since
--      it was written. Both ALTERs are therefore on an empty table and both new columns are
--      NULLABLE, so nothing is rewritten and no existing row changes meaning.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS / ADD COLUMN IF NOT EXISTS
--      / CREATE OR REPLACE, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. It only ADDS two nullable
--      columns, one table, two indexes and one CHECK. No existing column, index or constraint is
--      touched, and the store reads the new columns only after this file has run.
--   5. RETENTION: unchanged — a petition is an ARCHIVAL RECORD (rule 7). The outbox below is the
--      one thing here that is NOT archival, and it says so on its own comment.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as unfinished work:
--
--   * `anh_phan_anh` — the before/after photographs (docs/ui-ux/09 §8.4). Business rule 2 of §14
--     says a petition may not be CLOSED without an "after" photograph, and ADR 0008 makes that a
--     per-commune flag `bat_buoc_anh_nghiem_thu` (default TRUE). Neither the table nor the flag
--     store exists, so the closing route below cannot enforce it — stated in the report as a
--     finding, not papered over with a default that would be wrong in one direction or the other.
--   * `nhat_ky_phan_anh` — the processing timeline the right-hand column of §8.7 renders. It is a
--     BUSINESS record (a clerk reads it), distinct from `audit_log` (invisible to the commune), and
--     it is a separate pass. Nothing below references it.
--   * a REOPENING path. ADR 0008 governs it with three per-commune flags — `cho_phep_mo_lai`,
--     `nguong_sao_mo_lai`, `so_lan_mo_lai_toi_da` — and no table in this repository holds them.
--     `so_lan_mo_lai` already exists on the table from 0004 and stays at 0.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. THE CLASSIFICATION CEILING — ADR 0035 §C, which closed open question #26 on 2026-09-22.
--
-- A THIRD DEADLINE COLUMN, and the reason it has to be a COLUMN rather than `han_tiep_nhan` plus a
-- constant is rule 10, invariant 2: it is a commitment fixed by an act and stored, and ADR 0035 §C
-- states the consequence in its own words — "đổi trần về sau chỉ áp cho phiếu nhận từ lúc đổi trở
-- đi". Derived on read, every already-received petition's ceiling would move the day the number
-- changed, retroactively, in every report at once.
--
-- IT IS COUNTED IN WORKING HOURS by identity.AdvanceWorkingHours, from the SAME `goc_dem_han` as
-- the other two clocks, and fixed at the SAME act as `han_tiep_nhan` — the row being created.
--
-- NULL MEANS "KHÔNG ÁP DỤNG", the same meaning `han_tiep_nhan` NULL carries and for the same
-- reason: a staff-booked petition arrives with its field already settled, so the interval this
-- bounds does not exist. It NEVER means "chưa có" — that meaning belongs to `han_xu_ly_xong` alone,
-- and reading the three columns' NULLs as one thing is the mistake migration 0004 warns about at
-- length.
--
-- ⚠ THE NUMBER OF HOURS IS NOT IN THIS SCHEMA, deliberately: it is domain.GioTranPhanLoai, which
-- states in full why 8 is an assumption of the implementer rather than a figure the customer wrote.
-- Putting it in a column here would make it look per-commune, which ADR 0035 §C decided it is not.
-- ---------------------------------------------------------------------------
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS han_phan_loai TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- 2. THE RESULT THE CITIZEN READS — rule 10, invariant 6.
--
-- "Closing a petition records a result the citizen can read. Never close silently." This column is
-- that result, and it is the ONLY free text in this system written by staff that a citizen is meant
-- to read.
--
-- IT IS NOT A STAFF NOTE and must never become one. Staff notes and routing history stay internal
-- (rule 4, forbidden #5); those belong in `nhat_ky_phan_anh`, which this file does not create.
--
-- IT NEVER TRAVELS ON THE NOTIFICATION. proto/vigov/petitions/v1/events.proto states the rule and
-- the reason: text written about a specific case will eventually name the reporter, quote their
-- complaint or give their address, and a queue is persisted, replicated, backed up and read during
-- debugging. What travels is the procedural sentence, identical for every citizen at that
-- transition; the result stays here, reached with a lookup code behind an authenticated read.
--
-- NULLABLE because it is written at ONE act — closing — and every petition spends most of its life
-- before that act. A NOT NULL with a default would put an empty result on every open petition and
-- make "closed with nothing readable" indistinguishable from "not closed yet".
-- ---------------------------------------------------------------------------
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS ket_qua_xu_ly TEXT;

-- A deadline is reached AT OR AFTER the instant it was counted from — the same statement
-- `phieu_phan_anh_han_sau_goc` already makes about the other two clocks (ADR 0007, decision 8).
-- This catches the one mistake that looks right in a diff: writing `goc_dem_han` into the column.
--
-- A SEPARATE CONSTRAINT AND NOT AN EDIT OF phieu_phan_anh_han_sau_goc, because editing an applied
-- file is what note 3 at the top of 0004 forbids, and DROP/ADD of that constraint would leave a
-- window in which neither held.
--
-- DO $$ … $$ RATHER THAN A BARE ALTER: `ADD CONSTRAINT` has no IF NOT EXISTS, and a retry of this
-- file must cost nothing (migration question 2).
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_han_phan_loai_sau_goc'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_han_phan_loai_sau_goc
            CHECK (han_phan_loai IS NULL OR han_phan_loai >= goc_dem_han);
    END IF;
END $$;

-- The staff register screen: one commune's petitions, newest first (docs/ui-ux/09 §2), soft-deleted
-- rows excluded EVERYWHERE, ALWAYS (rule 7, invariant 2).
--
-- `(tenant_id, vao_so_luc DESC)` ALREADY EXISTS as `phieu_phan_anh_so` in 0004 and is what the list
-- route's default sort uses. What is added here is the SECOND sort the cursor offers, `tao_luc`,
-- which page.QueryPage tie-breaks with `id` — so the index carries the tie-break column too. A
-- cursor paging on a column with no matching index is a sequential scan per page of a register that
-- grows with every week the commune operates.
CREATE INDEX IF NOT EXISTS phieu_phan_anh_so_theo_tao_luc
    ON phieu_phan_anh (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL;

-- The classification-ceiling queue: what has come in, nobody has read, and the ceiling is running
-- on. It is the index behind the indicator ADR 0035 §C created — and OVERDUE IS STILL DERIVED, this
-- index holds the deadline and never a verdict about it (rule 10, invariant 3).
CREATE INDEX IF NOT EXISTS phieu_phan_anh_tran_phan_loai
    ON phieu_phan_anh (tenant_id, han_phan_loai)
    WHERE deleted_at IS NULL AND han_phan_loai IS NOT NULL AND phan_loai_luc IS NULL;

-- ---------------------------------------------------------------------------
-- @entity: OutboxEvent
-- @scope:  tenant
--
-- su_kien_di — the OUTBOX. One row per fact this service has to tell another service about.
--
-- # WHY IT EXISTS AT ALL, WHICH IS THE PART WORTH READING
--
-- Rule 10, invariant 5: every status transition notifies the citizen. `comms` does the notifying and
-- listens for `petitions.status_changed.v1` (its consumer is already written —
-- service-comms/internal/event/phieu_doi_trang_thai.go). So the obligation on THIS service is to
-- publish that event, and `skills/events-and-queues` #5 plus the event contract itself both say the
-- same thing: PUBLISHED AFTER THE BUSINESS TRANSACTION COMMITS, NEVER INSIDE IT.
--
-- Those two requirements together have exactly one honest shape, and it is this table:
--
--   in the transaction   the status change, the audit entry, and a ROW HERE — all or nothing
--   after the commit     a relay reads the rows and publishes them
--
-- Publish inside the transaction and the consumer reads a petition the database has not made
-- visible yet — a timing-dependent failure that survives every test. Publish after it with nothing
-- recorded and a process that dies in the window loses the notification permanently, with nothing
-- anywhere saying a citizen was never told. Rule 10 does not permit the second, and the first is
-- what the contract forbids.
--
-- ⚠ THE RELAY DOES NOT EXIST YET, and that is a FINDING rather than a hidden gap. There is no Kafka
-- client in this repository at all — `core/events.Publisher` is a bare interface with no
-- implementation, which service-comms' consumer states in its own header. Until one is written,
-- rows accumulate here with `gui_luc` NULL and no citizen is actually messaged. What this table buys
-- today is that the obligation is RECORDED, transactionally, and is recoverable: every notification
-- owed since the first petition is still here, in order, the day the relay runs. The alternative —
-- writing nothing — loses them silently and forever.
--
-- # NOT AN ARCHIVAL RECORD, AND THE DIFFERENCE MATTERS
--
-- This is INFRASTRUCTURE state, not a government record: it says "a message is owed", while the fact
-- itself lives in `phieu_phan_anh` and the accountability lives in `audit_log`. So, unlike every
-- other table in this service, rows here may eventually be pruned once delivered — and the
-- `ho_so_luu_tru_bat_bien` trigger is deliberately NOT attached to it. A pruning job is not written
-- here because there is nothing to prune until the relay exists.
--
-- # WHAT MAY NOT GO IN `than`
--
-- The payload is the protojson of vigov.petitions.v1.PetitionStatusChanged and NOTHING ELSE. That
-- contract closes the list at four scalars plus a two-field CitizenMessage, and its header states
-- the rule this column inherits: no phone number, no name, no address, no coordinates, no text of
-- the petition, no photograph. A queue is persisted, replicated and backed up; so is this table.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS su_kien_di (
    tenant_id  TEXT        NOT NULL,
    id         TEXT        NOT NULL,

    -- The event name WITH ITS VERSION (rule 2, invariant 4): `petitions.status_changed.v1`. Two
    -- versions run side by side during a migration, so the version is part of the identity of the
    -- message and not metadata beside it.
    ten        TEXT        NOT NULL,

    -- The BUSINESS code of the record this is about — a lookup code. Not the internal ULID: two
    -- identifiers for one record is an invitation to log, join or display the wrong one.
    doi_tuong  TEXT        NOT NULL,

    -- protojson, proto field names preserved. See the note above on what may not be in it.
    than       JSONB       NOT NULL,

    -- When the fact happened, which becomes the envelope's `occurred_at`. NOT the moment the relay
    -- publishes: a consumer ordering by publication time would reorder history after an outage.
    xay_ra_luc TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- NULL until the relay has published it. It is the WHOLE queue state, on purpose: "not yet sent"
    -- is the absence of a timestamp, so there is no status column to get out of step with it.
    gui_luc    TIMESTAMPTZ,

    tao_luc    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6). `id` is the ULID that becomes the envelope's
    -- deduplication id, so a consumer that sees the same row twice sees the same id twice.
    PRIMARY KEY (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS su_kien_di_p%s PARTITION OF su_kien_di '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The relay's own query: what is still owed, oldest first, in this commune.
--
-- PARTIAL ON `gui_luc IS NULL`, so the index shrinks back to nothing as messages are delivered
-- rather than growing with every message the commune has ever sent.
CREATE INDEX IF NOT EXISTS su_kien_di_cho_gui
    ON su_kien_di (tenant_id, xay_ra_luc) WHERE gui_luc IS NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3).
--
-- WHILE `phieu_phan_anh` IS STILL EMPTY — which it is in every environment today — the reversal is
-- complete and loses nothing: drop the two indexes and the CHECK, drop the two columns, drop
-- `su_kien_di` (its 32 partitions go with it), and in the same transaction remove this file's row
-- from `schema_migration`.
--
-- ONCE A COMMUNE HAS CLOSED ONE PETITION, `ket_qua_xu_ly` HOLDS A RESULT THAT WAS SHOWN TO A
-- CITIZEN, and dropping it is destroying part of an archival record — rule 7's first stop condition,
-- which needs the user and not a command. `han_phan_loai` is the same: it is a commitment that was
-- counted. From that point the way back is a NEW migration.
--
-- AND ONCE `su_kien_di` HOLDS UNSENT ROWS, dropping it is throwing away notifications owed to
-- citizens. Drain it first; the table is not archival, but the obligations in it are.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the outbox row shares the business transaction, that first write
-- rolls back entirely. In this service the first write is a clerk acting on a citizen's report.
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
