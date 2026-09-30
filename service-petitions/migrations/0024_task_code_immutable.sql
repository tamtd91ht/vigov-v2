-- 0024 — a task's code is IMMUTABLE again: `nhiem_vu_bat_bien` refuses every change to `nhiem_vu.ma`,
-- unconditionally, as 0006 made it. The ledger `task_issued_code` (0015) is KEPT.
--
-- WHY THIS FILE EXISTS. User decision 30/09/2026, ADR 0065 NV3
-- (kb/10-decisions/0065-vong-doi-nhiem-vu-theo-kho-yeu-cau.md): a task code is NEVER edited after it is
-- issued (rule 7, invariant 3; forbidden #4) — a deliberate divergence from vigov-require 7764c8a, and
-- the reversal of the 28/09 choice that 0015 implemented. A task code is quoted on paper minutes,
-- directive documents and the printed Sổ theo dõi; renaming it leaves every one of those pointing at
-- nothing (the URL by the old code answers 404), even though the ledger stops the code being reissued.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006 / 0015 / 0016: core/migrate compares the checksum of every
-- applied file at startup. `nhiem_vu_bat_bien` is REPLACED here with CREATE OR REPLACE; the texts of
-- 0006, 0015 and 0016 stay as they were applied.
--
-- ---------------------------------------------------------------------------
-- WHAT CHANGES AND WHAT DOES NOT.
--
--   `ma`            back to 0006's refusal, word for word: ANY change is refused, for every writer —
--                   this service, a psql prompt, a future import. 0015's "only through the ledger"
--                   condition is gone, so there is no statement pair that can rename a task any more.
--   `han_ban_dau`   0016's rule, UNCHANGED (ADR 0065 open question #1, answered 30/09: keep the 28/09
--                   rule). A correction may carry it along while no extension was ever approved.
--   hard DELETE     refused, unchanged.
--
--   `task_issued_code` STAYS, with its append-only guard and its AFTER INSERT registration trigger.
--   It is still the never-reissue guarantee: a task renamed while the edit path was live (28/09 →
--   this file) left its former code ONLY in that ledger, and dropping it would free that code for the
--   next task. Minting and the duplicate check at creation keep reading it.
--
--   A task that WAS renamed while the path was live KEEPS ITS NEW CODE (ADR 0065 NV3 note; rule 7,
--   forbidden #4). Renaming it back would be a second renumbering, not a repair.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. No row is inserted, updated or deleted; only a trigger
--      function body is replaced. The NOTICE below reports, per commune, how many tasks carry a code
--      other than the first one issued to them (renamed while the path was live) — counts and tenant
--      ids only. The user confirmed 30/09/2026 that no environment holds real data, so zero is the
--      expected answer everywhere; the count is there so that is MEASURED, not assumed.
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, nothing to resume, no commune to
--      iterate.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction with the progress row. CREATE OR
--      REPLACE, so a retry costs nothing.
--   3. HOW IT IS REVERSED: re-run 0016's `CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien()` verbatim and
--      remove this file's row from `schema_migration` in the same transaction. Lossless: this file
--      changes no data. Doing so RE-OPENS the rename path the user closed on 30/09 — a user decision,
--      not an operator's.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can be half-applied. BETWEEN
--      THIS FILE AND THE GO CHANGE THAT RETIRES THE EDIT PATH (PATCH `code`, store.ChangeCode): the
--      migration is embedded in the binary, so it ships with that change; during a rolling deploy an
--      OLD replica still accepts PATCH `code`, and its UPDATE of `ma` is now refused by this trigger —
--      the whole act, audit entry included, rolls back and the client sees a 500. Fail closed: no
--      task is renamed. No READ path reads the function.
--   5. RETENTION: `nhiem_vu` is an archival record and `ma` its issued number (rule 7). This file
--      makes the number immutable again; it removes no record and frees no code.

DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT n.tenant_id, count(*) AS renamed
        FROM nhiem_vu n
        WHERE EXISTS (SELECT 1 FROM task_issued_code c
                      WHERE c.tenant_id = n.tenant_id AND c.task_id = n.id AND c.code <> n.ma)
        GROUP BY n.tenant_id
        ORDER BY n.tenant_id
    LOOP
        RAISE NOTICE '0024: commune % holds % task(s) renamed while the edit path was live; they keep their current code',
            r.tenant_id, r.renamed;
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'A task carries the commune''s own progress log and an issued number: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1, and docs/ui-ux/02-nhiem-vu.md:356.';
    END IF;

    -- 0006's refusal, restored (ADR 0065 NV3). No condition: there is no legitimate rename.
    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        RAISE EXCEPTION 'administrative record %: `ma` is immutable', TG_TABLE_NAME
            USING HINT = 'An issued task number is never reissued and never renumbered (rule 7, '
                         'invariant 3; ADR 0065 NV3). It is quoted in meeting minutes and printed in '
                         'the Sổ theo dõi, and nothing rewrites those.';
    END IF;

    -- 0016, unchanged.
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
