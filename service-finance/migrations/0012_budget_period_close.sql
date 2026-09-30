-- 0012 — a commune CLOSES a budget period (chốt kỳ), by MONTH or by whole YEAR, with permission
-- `budget.confirm`; a closed period may be REOPENED with `budget.confirm` + a reason + an audit
-- entry. User decision 30/09/2026, ledger service-finance items `thu-chi-ngan-sach-82` (g) and
-- `thu-chi-chot-ky-va-cong-khai`.
--
-- WHY A NEW FILE: 0001..0011 have been applied and core/migrate compares the checksum of every applied
-- file at startup (ErrChecksumLech). Same reason 0008 and 0011 give.
--
-- ---------------------------------------------------------------------------
-- WHAT THE DECISION MEANS, AND WHICH HALF LIVES HERE:
--
--   * After a close, batches (`dot_thu_chi`, 0008) DATED in that period cannot be added or removed.
--     "Dated" = `dot_thu_chi.ngay` (DATE, the day the money moved as the commune states it — 0008),
--     NOT `tao_luc` (the day the row was written). A month close of 09/2026 covers
--     `ngay >= '2026-09-01' AND ngay < '2026-10-01'`; a year close of 2026 covers the whole year.
--   * A mistake inside a closed period is fixed by an ADJUSTMENT batch in a later OPEN period, with
--     a mandatory reason. 0008 has NO reason/note column on `dot_thu_chi` (its columns: ngay,
--     noi_dung, don_vi_ca_nhan, so_chung_tu, nguoi_ghi_ma, and the soft-delete trio — whose
--     `delete_reason` is the reason for a REMOVAL, a different fact). So this file adds ONE nullable
--     column, `adjustment_reason`. NULL = an ordinary batch; non-NULL = an adjustment batch, and the
--     text is why. It is frozen with the rest of the row by 0008's `dot_thu_chi_bat_bien`, which
--     compares the whole row minus the soft-delete trio — so it needs no change to that function.
--   * A YEAR close also locks the hand-entered line values (`gia_tri_khoan_muc`) and the sheet/line
--     edits of that year's sheets (`bang_ngan_sach.nam = year`). Hand-entered values carry no date,
--     so a MONTH close cannot lock them (main-session decision, stated to the user).
--
--   THE DATABASE STORES THE CLOSES. IT DOES NOT REFUSE THE WRITES A CLOSE LOCKS. "Is this batch's
--   date inside an active close" is a cross-table fact; a CHECK cannot read another table, and a
--   trigger on `dot_thu_chi` / `gia_tri_khoan_muc` that looked this table up is exactly the
--   cross-table trigger on a hash-partitioned table 0008 considered and rejected (no PostgreSQL
--   reachable from this build environment to verify it; a migration failing at startup stops the
--   service). THE COST, STATED: nothing in the database stops a writer adding a batch into a closed
--   month. The domain layer (TASK-05) refuses it inside the write transaction — see the RACE note.
--
-- ---------------------------------------------------------------------------
-- THE SHAPE: ONE ROW PER CLOSE ACT. HISTORY IS NEVER EDITED (rule 7, forbidden #5).
--
--   close    INSERT a row: year, month (NULL = whole year), closed_at, closed_by.
--   reopen   fills reopened_at / reopened_by / reopen_reason ONCE, from NULL. Nothing else moves.
--   re-close a NEW row, with the next `revision` and a new `code`.
--
--   So "closed 05/10 by CB-00012, reopened 12/10 by CB-00003 because …, closed again 13/10 by
--   CB-00012" is three facts on two rows, all still readable. A single row per period with a status
--   column would have to OVERWRITE who closed it and when on the second close — a silent edit of a
--   record the trail already cites.
--
--   NO SOFT-DELETE COLUMNS, ON PURPOSE: a close is never removed; reopening IS how a close ends, and
--   it carries its own who/when/why. A `deleted_at` beside `reopened_at` would be a second way to end
--   the same thing, and two readers would disagree about which one counts.
--
-- A BUSINESS CODE, `code`: the audit subject must be a business code (rule 6, invariant 8, and 0006's
-- choice of `bang_ngan_sach.ma` for every budget write). A close has no parent record carrying a
-- `ma` — it is about a PERIOD, not a sheet — and the reopen entry must name WHICH close it ended. So
-- each close row carries its own code, issued once and never reissued (rule 7, invariant 3): the
-- uniqueness below counts reopened rows. The FORMAT is the domain's to issue (TASK-05); a suggestion
-- that keeps month and year closes apart on sight: `CK-2026-09-01` (month 09, revision 1) and
-- `CK-2026-CN-01` (whole year). `revision` is the natural-column form of the same fact — the shape of
-- `bang_ngan_sach.lan` — so a second close of one period cannot be issued with a hand-written code
-- that happens not to collide.
--
-- ---------------------------------------------------------------------------
-- "AT MOST ONE ACTIVE CLOSE PER (tenant, year, month-or-year)" — ENFORCED BY THE DATABASE:
--
--   unique index `budget_period_closes_one_active` over (tenant_id, year, month),
--   NULLS NOT DISTINCT, partial on `reopened_at IS NULL` — the statement is further down.
--   (Not quoted as SQL here: tools/check_khoa_duy_nhat.py scans unique-index statements inside
--   comments too, and would count this prose as a declaration.)
--
--   * composite with tenant_id, which is also the partition key — the condition PostgreSQL sets for
--     a unique index on a partitioned table (rule 1, invariant 6);
--   * NULLS NOT DISTINCT is what makes the YEAR close (month NULL) unique at all. Without it two
--     rows (T, 2026, NULL) are "distinct" and both pass — two active year closes, no error. It needs
--     PostgreSQL 15; the real cluster is 16 (floor check below);
--   * PARTIAL on `reopened_at IS NULL`, and that is NOT the trap check_khoa_duy_nhat hunts: that trap
--     is a partial unique index on a BUSINESS CODE, which lets an issued code be issued again. The
--     code here is protected by a separate, NON-partial key (`UNIQUE (tenant_id, code)`, and
--     `budget_period_closes_revision_once` counts reopened rows too). This index states a STATE rule —
--     one live close at a time — and a reopened close is by definition not live.
--
--   Declared rather than left to the domain because a check-then-insert under READ COMMITTED lets two
--   concurrent closes of one month both see "none active" and both insert. A unique index is the one
--   form that cannot race. The domain maps the violation to a 409 ("kỳ này đã chốt").
--
--   ⚠ NOT VERIFIED AGAINST A SERVER HERE (VIGOV_TEST_DSN unset). It is the first partial /
--   NULLS NOT DISTINCT unique index on a partitioned table in this service; 0006 declined a partial
--   unique index for exactly that reason. It is taken here because the domain-only alternative races
--   (above), and internal/store/budget_period_close_pg_test.go exercises it the first time a DSN is
--   set. If the server refuses it, this file fails at startup inside its transaction and changes
--   nothing — the fail-closed outcome.
--
-- YEAR close vs MONTH close of the same year: both may exist, active at once; nothing in SQL relates
-- them. A date is locked when EITHER an active month close of its month OR an active year close of
-- its year exists.
--
-- ---------------------------------------------------------------------------
-- THE WRITE-PATH CHECK TASK-05 RUNS, and the index that serves it (the unique index above: its
-- columns are exactly the lookup, and its predicate is exactly "active"):
--
--   SELECT EXISTS (SELECT 1 FROM budget_period_closes
--                  WHERE tenant_id = $1 AND year = $2 AND (month = $3 OR month IS NULL)
--                    AND reopened_at IS NULL)
--   -- $2 = extract(year FROM ngay), $3 = extract(month FROM ngay)
--
-- RACE, FOR TASK-05 — NOT SOLVABLE IN THIS FILE: a batch write that checks "no active close" and a
-- close committing at the same instant can both succeed (the batch checks an ABSENCE, which no row
-- lock can hold). Both paths must serialise on one key per (tenant, year) — e.g.
-- `pg_advisory_xact_lock(hashtextextended('budget-period:' || tenant_id || ':' || year, 0))` taken by
-- the close, the reopen, every batch add/remove, and (for a year close) every line/sheet/value
-- write, before the check. Without it a batch can land in a month that reads as closed.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: zero — one new empty table, one nullable column on `dot_thu_chi`
--      (no row rewritten: ADD COLUMN with no default is catalogue-only). Ceiling in sight: 12 month
--      closes + 1 year close per year, plus one row per re-close.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with the progress row. Every statement is IF NOT EXISTS, CREATE OR REPLACE, DROP TRIGGER IF
--      EXISTS, or guarded by a lookup pinned to the parent table, so a retry costs nothing. No
--      backfill, so the per-commune resumable-backfill question (rule 7, invariant 5) has no rows.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied. After
--      it: NONE TODAY. The store reads and inserts `dot_thu_chi` through explicit column lists
--      (internal/store/dot_thu_chi.go, `cotDot` / `chenDot`) that do not name `adjustment_reason`,
--      and nothing reads the new table. ROLLING DEPLOY: an old replica inserts batches with
--      `adjustment_reason` NULL (= ordinary batch — correct), and does NOT check closes: until every
--      replica runs TASK-05's code, a batch can still be added into a period closed by a new replica.
--      Closing a period before the rollout completes is therefore not a lock; the rollout order is
--      the operator's to keep.
--   5. RETENTION: a close is the record that a commune's figures for a period were declared final —
--      the fact the Mini App publication (deferred, ledger item) will cite. Archival (rule 7): no
--      hard delete (trigger), no edit except the single reopen fill (trigger). Nothing existing is
--      dropped, retyped or rewritten; no business code is issued or renumbered by this file.
-- ---------------------------------------------------------------------------

-- PostgreSQL 15 is the floor: NULLS NOT DISTINCT (above) arrived in 15. (13 was the floor for the
-- BEFORE ... FOR EACH ROW triggers on partitioned tables, 0004..0008.) The cluster runs 16.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 150000 THEN
        RAISE EXCEPTION
            'budget period closes need PostgreSQL 15 or newer (server is %). Do not drop NULLS NOT '
            'DISTINCT to fit an older server — without it two active year closes of one year are '
            'both accepted, silently.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- dot_thu_chi.adjustment_reason — see the header. Nullable; when set, not blank and bounded
-- (500 = domain.LyDoXoaNganSachToiDa, the bound every reason on this sheet already meets).
-- Guarded by a lookup pinned to the PARENT table (`conrelid`), as 0008/0011 do: partitions inherit a
-- CHECK under the same name, so `conname` alone matches 33 rows. Every existing row has NULL, which
-- both CHECKs admit.
-- ---------------------------------------------------------------------------
ALTER TABLE dot_thu_chi ADD COLUMN IF NOT EXISTS adjustment_reason TEXT;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'dot_thu_chi'::regclass
                     AND conname = 'dot_thu_chi_adjustment_reason_not_blank') THEN
        ALTER TABLE dot_thu_chi ADD CONSTRAINT dot_thu_chi_adjustment_reason_not_blank
            CHECK (adjustment_reason IS NULL OR btrim(adjustment_reason) <> '');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'dot_thu_chi'::regclass
                     AND conname = 'dot_thu_chi_adjustment_reason_max') THEN
        ALTER TABLE dot_thu_chi ADD CONSTRAINT dot_thu_chi_adjustment_reason_max
            CHECK (char_length(adjustment_reason) <= 500);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- budget_period_close_immutable — a close row never changes, except ONE fill of the reopen trio.
--
-- DENY BY DEFAULT, like 0008's dot_thu_chi_bat_bien: the WHOLE ROW minus the three reopen columns
-- must be unchanged, so a column a later migration adds is frozen the day it exists.
-- ALLOWS: exactly one UPDATE per row — NULL -> value on reopened_at, reopened_by, reopen_reason (the
-- all-or-none CHECK keeps the three together).
-- REFUSES: any change to any other column; any change to the trio once set — no "un-reopen", no
-- rewriting of who reopened it or why. A re-close is a NEW row.
-- Messages name the relation only; no column value is interpolated (the reason is free text).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION budget_period_close_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(NEW) - 'reopened_at' - 'reopened_by' - 'reopen_reason')
       IS DISTINCT FROM
       (to_jsonb(OLD) - 'reopened_at' - 'reopened_by' - 'reopen_reason') THEN
        RAISE EXCEPTION 'archival record %: a recorded period close cannot be edited', TG_TABLE_NAME
            USING HINT = 'Who closed a period, when, and which period is recorded once (rule 7). '
                         'To change it, reopen the close with a reason and close the period again '
                         '— a new row, never an edit.';
    END IF;

    IF OLD.reopened_at IS NOT NULL
       AND (NEW.reopened_at   IS DISTINCT FROM OLD.reopened_at
         OR NEW.reopened_by   IS DISTINCT FROM OLD.reopened_by
         OR NEW.reopen_reason IS DISTINCT FROM OLD.reopen_reason) THEN
        RAISE EXCEPTION 'archival record %: a reopened close stays reopened', TG_TABLE_NAME
            USING HINT = 'A reopen is recorded once. Close the period again instead — a new row '
                         'with the next revision.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: BudgetPeriodClose
-- @scope:  tenant
--
-- budget_period_closes — one close act of one period (a month, or a whole year) of one commune.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS budget_period_closes (
    tenant_id      TEXT        NOT NULL,
    -- ULID.
    id             TEXT        NOT NULL,
    -- Business code, the audit subject of the close and of its reopen. Issued once, never reissued.
    code           TEXT        NOT NULL,
    year           INT         NOT NULL,
    -- 1..12 = that month; NULL = the WHOLE YEAR.
    month          INT,
    -- 1, 2, 3 … one per close of this (year, month-or-year): a re-close after a reopen is the next.
    revision       INT         NOT NULL,
    closed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- STAFF BUSINESS CODE (`CB-00123`), never Principal.ID (rule 6, invariant 8) — the name says
    -- `_by` like `deleted_by`, and the same policy holds.
    closed_by      TEXT        NOT NULL,
    reopened_at    TIMESTAMPTZ,
    -- STAFF BUSINESS CODE, as closed_by.
    reopened_by    TEXT,
    reopen_reason  TEXT,
    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    -- NOT partial: counts reopened rows, so an issued code is never issued again (rule 7, inv. 3).
    UNIQUE (tenant_id, code),
    CONSTRAINT budget_period_closes_code_not_blank CHECK (btrim(code) <> ''),
    -- Same window as `bang_ngan_sach.nam` (0006) and `dot_thu_chi.ngay` (0008).
    CONSTRAINT budget_period_closes_year_valid CHECK (year BETWEEN 2000 AND 2100),
    CONSTRAINT budget_period_closes_month_valid CHECK (month BETWEEN 1 AND 12),
    CONSTRAINT budget_period_closes_revision_positive CHECK (revision >= 1),
    -- A close nobody can be named for is a close nobody answers for (rule 6, invariant 8).
    CONSTRAINT budget_period_closes_closed_by_not_blank CHECK (btrim(closed_by) <> ''),
    -- REOPEN IS ALL OR NONE, reason and actor non-blank. Half a reopen is half a fact — and
    -- `reopened_at` alone decides "active", so a stray reason on an active close would read as a
    -- reopen to one reader and not to another.
    CONSTRAINT budget_period_closes_reopen_all_or_none CHECK (
        (reopened_at IS NULL AND reopened_by IS NULL AND reopen_reason IS NULL)
        OR (reopened_at IS NOT NULL
            AND reopened_by IS NOT NULL AND btrim(reopened_by) <> ''
            AND reopen_reason IS NOT NULL AND btrim(reopen_reason) <> '')),
    -- 500 = domain.LyDoXoaNganSachToiDa, as every reason on this sheet.
    CONSTRAINT budget_period_closes_reopen_reason_max CHECK (char_length(reopen_reason) <= 500),
    -- A reopen before its close is a clock or a forged row, never an act.
    CONSTRAINT budget_period_closes_reopen_after_close CHECK (reopened_at >= closed_at)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS budget_period_closes_p%s PARTITION OF budget_period_closes '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- At most one ACTIVE close per (tenant, year, month-or-year), and the index the write-path check
-- reads. See the header block for every reason.
CREATE UNIQUE INDEX IF NOT EXISTS budget_period_closes_one_active
    ON budget_period_closes (tenant_id, year, month) NULLS NOT DISTINCT WHERE reopened_at IS NULL;

-- The natural-column twin of `code` (the shape of `bang_ngan_sach`'s UNIQUE (tenant_id, nam, loai,
-- lan)). NOT partial: a revision of a reopened close is still issued. NULLS NOT DISTINCT so the year
-- close's revisions are unique too.
CREATE UNIQUE INDEX IF NOT EXISTS budget_period_closes_revision_once
    ON budget_period_closes (tenant_id, year, month, revision) NULLS NOT DISTINCT;

DROP TRIGGER IF EXISTS budget_period_closes_no_hard_delete ON budget_period_closes;
CREATE TRIGGER budget_period_closes_no_hard_delete
    BEFORE DELETE ON budget_period_closes
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS budget_period_closes_immutable ON budget_period_closes;
CREATE TRIGGER budget_period_closes_immutable
    BEFORE UPDATE ON budget_period_closes
    FOR EACH ROW EXECUTE FUNCTION budget_period_close_immutable();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, and DDL by the table owner (DISABLE TRIGGER,
-- dropping a constraint). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no
-- automatic rollback (ADR 0013).
--
--   1. CHECK FIRST that nothing depends on it:
--        SELECT count(*) FROM budget_period_closes;                              -- must be 0
--        SELECT count(*) FROM dot_thu_chi WHERE adjustment_reason IS NOT NULL;   -- must be 0
--      If either is not zero, STOP: see below.
--   2. DROP TABLE budget_period_closes;   (32 partitions, two indexes, two triggers go with it)
--      DROP FUNCTION budget_period_close_immutable();
--      ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's and other tables are attached to it.
--   3. ALTER TABLE dot_thu_chi DROP CONSTRAINT dot_thu_chi_adjustment_reason_not_blank,
--                              DROP CONSTRAINT dot_thu_chi_adjustment_reason_max,
--                              DROP COLUMN adjustment_reason;
--   4. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
--
-- LOSSLESS ONLY WHILE STEP 1 RETURNS TWO ZEROS — true the day this is written (no route writes
-- either yet). ONCE A COMMUNE HAS CLOSED A PERIOD, step 2 destroys the record that its figures were
-- declared final; ONCE ONE ADJUSTMENT BATCH EXISTS, step 3 drops a populated column and the batch
-- loses the only statement of why it exists. Either is rule 7 stop condition 2: the user and a
-- verified backup, not a command. The Go code reading them must be rolled back with the schema.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (see 0008).
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
