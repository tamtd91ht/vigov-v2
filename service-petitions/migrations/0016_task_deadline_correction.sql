-- 0016 — correcting a task's deadline through PATCH: `nhiem_vu_bat_bien` replaced again, so that
-- `han_ban_dau` may follow a CORRECTION of `han_xu_ly` while the task has had no approved extension.
--
-- WHY THIS FILE EXISTS. User decision 28/09/2026 (card P6, kb/50-doi-chieu/2026-09-26-feat-m8-
-- multitenant-foundation-nhiem-vu.md §Mâu thuẫn #3), following vigov-require 93cff7f:
--
--   "Sửa hạn xử lý KHÔNG phải gia hạn … Còn đây là sửa cho đúng — gõ nhầm ngày, hoặc văn bản giao việc
--    ghi hạn khác. Nếu chưa có lần gia hạn nào thì hạn ban đầu phải đi theo"
--
-- A correction says the commitment was RECORDED wrong, not that it moved. While no extension has ever
-- been approved, the original commitment and the current one are the same fact typed twice, and a
-- correction that left `han_ban_dau` behind would keep the typo as the denominator of §11.3's on-time
-- ratio for ever. Once an extension HAS been approved, `han_ban_dau` is the commitment as it stood
-- before that approval, and it is frozen exactly as 0006 made it: a correction then moves
-- `han_xu_ly` alone.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006 / 0015: core/migrate compares the checksum of every applied
-- file at startup. `nhiem_vu_bat_bien` is REPLACED here with CREATE OR REPLACE; the texts of 0006 and
-- 0015 stay as they were applied.
--
-- ---------------------------------------------------------------------------
-- THE RULE THE TRIGGER NOW ENFORCES FOR `han_ban_dau`, and why each half is there:
--
--   `han_ban_dau` may change ONLY IF, in the same row version,
--     (a) NEW.han_ban_dau = NEW.han_xu_ly  — it FOLLOWS the current deadline; it is never set to a
--         value of its own. Without (a), "no extension yet" would open the column to any value, and a
--         row whose original commitment differs from its current one with no extension to explain it
--         is a row §11.3 reads as an extension that never happened.
--     (b) no request in `de_nghi_lui_han` for this task was ever APPROVED (`trang_thai = 'da-duyet'`),
--         SOFT-DELETED REQUESTS INCLUDED. Deliberately not filtered on `deleted_at`: an approval that
--         moved the deadline moved it, whatever happened to the request row afterwards, and a soft
--         delete that re-opened `han_ban_dau` would let the deadline an extension replaced be
--         rewritten under the ratio — the exact harm 0006's refusal exists to stop.
--
--   Everything else in the function is 0015's, unchanged: hard DELETE refused; `ma` changes only
--   through `task_issued_code`.
--
-- WHAT THIS DOES NOT REQUIRE, said plainly: that the change came through the service. A psql UPDATE
-- that sets both deadlines to one new value on a task with no approved extension passes, because that
-- IS the rule, not a way around it. What the database cannot see is the audit entry and the timeline
-- row; those are the write path's (app.Sua).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. No row is inserted, updated or deleted; only a trigger
--      function body is replaced.
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no row rewritten, so there is
--      nothing to resume and no commune to iterate.
--   2. IF IT STOPS HALF-WAY: it cannot. One statement, one transaction with the progress row.
--      CREATE OR REPLACE, so a retry costs nothing.
--   3. HOW IT IS REVERSED: re-run 0015's `CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien()` verbatim
--      and remove this file's row from `schema_migration` in the same transaction. Lossless: no data
--      is changed by this file. A deadline corrected while it was in force stays corrected — that is
--      a business act with its audit entry, not something a schema reversal takes back.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can be half-applied. Before the
--      Go change ships nothing writes `han_ban_dau` on UPDATE, so the relaxed branch is unreachable;
--      no query reads the function. After it, §11.3's ratio reads a corrected `han_ban_dau` for tasks
--      corrected BEFORE any extension — which is the decision, stated in its own words above.
--   5. RETENTION: `nhiem_vu` is an archival record (rule 7). The relaxation touches one column under
--      one condition; the prior value survives in the audit entry the write path writes in the same
--      transaction (rule 6, invariant 5).

CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'A task carries the commune''s own progress log and an issued number: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1, and docs/ui-ux/02-nhiem-vu.md:356.';
    END IF;

    -- 0015, unchanged.
    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        IF NOT EXISTS (SELECT 1 FROM task_issued_code c
                       WHERE c.tenant_id = NEW.tenant_id AND c.code = NEW.ma
                         AND c.task_id = NEW.id)
        OR NOT EXISTS (SELECT 1 FROM task_issued_code c
                       WHERE c.tenant_id = OLD.tenant_id AND c.code = OLD.ma) THEN
            RAISE EXCEPTION 'administrative record %: `ma` changes only through task_issued_code',
                TG_TABLE_NAME
                USING HINT = 'Issue the new code to this task in task_issued_code first, in the same '
                             'transaction (store.ChangeCode). The old code stays in that ledger and is '
                             'never issued again (rule 7, invariant 3).';
        END IF;
    END IF;

    -- 0016: a correction may carry `han_ban_dau` along, and only a correction, and only before any
    -- approved extension. See the header for (a) and (b).
    IF NEW.han_ban_dau IS DISTINCT FROM OLD.han_ban_dau THEN
        IF NEW.han_ban_dau IS DISTINCT FROM NEW.han_xu_ly
        OR EXISTS (SELECT 1 FROM de_nghi_lui_han d
                   WHERE d.tenant_id = NEW.tenant_id AND d.nhiem_vu_id = NEW.id
                     AND d.trang_thai = 'da-duyet') THEN
            RAISE EXCEPTION 'administrative record %: `han_ban_dau` moves only with a correction before any approved extension',
                TG_TABLE_NAME
                USING HINT = 'It is the commitment as FIRST made and the denominator of the on-time '
                             'ratio (docs/ui-ux/02-nhiem-vu.md:188, :354). A correction may carry it '
                             'along with `han_xu_ly` while no extension was ever approved; after one, '
                             'only `han_xu_ly` moves (user decision 28/09/2026, require 93cff7f).';
        END IF;
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
-- This file adds no table; it runs anyway, because the run after which it is missing is the one that
-- needed it.
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
