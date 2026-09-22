-- identity — nguoi_dung: separate "has a sign-in account" from "not locked out".
--
-- WHY A NEW FILE INSTEAD OF EDITING 0001_init.sql: 0001 has already been applied, and
-- core/migrate compares the checksum of every applied file at startup. Editing it would stop
-- the service (ErrChecksumLech) — and where it did not, it would leave two databases claiming
-- the same schema version with two different schemas.
--
-- WHAT WAS WRONG. The specification (docs/ui-ux/12-danh-ba-can-bo.md, §7) merges two seed sets
-- into ONE `nguoi_dung` table: the 26 people of the public staff directory, and the accounts
-- that can actually sign in. The schema had only `dang_hoat_dong`, which means "NOT LOCKED",
-- and the two concepts were being read off the same column. Three consequences, all silent:
--
--   1. Cấu hình → Người dùng counts a directory entry that has never had a password as
--      "Đang hoạt động" — a figure a commune reports upward, wrong by construction.
--   2. "Khoá tài khoản" on a directory-only person is a button that does nothing meaningful
--      yet reports success.
--   3. The `nguoi_dung_dang_nhap` index covered people who can never sign in.
--
-- COLUMN NAME — `co_tai_khoan`, DELIBERATELY NOT THE SPECIFICATION'S `tai_khoan_hoat_dong`.
-- Nothing is missing from the specification here; the name was changed on purpose, and the
-- reason is the very defect this file repairs. `dang_hoat_dong` and `tai_khoan_hoat_dong` share
-- most of their letters and mean entirely different things. Sitting next to each other in one
-- table, in one SELECT list, they guarantee that somebody eventually reads one for the other —
-- and "somebody read one for the other" is exactly the incident being fixed. `co_tai_khoan`
-- reads as the yes/no question it is ("does this person have an account") and cannot be
-- mistaken for "is this person active". The specification's name is a product of the
-- prototype, not a commitment made to the customer.
--
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL:
--
--   1. HOW MANY ROWS PER COMMUNE: today zero — no commune holds real staff data yet. The
--      specification's own seed is the ceiling in sight: 26 directory entries and 12 accounts
--      per commune. Tens of rows, not millions.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. core/migrate runs each file in ONE
--      transaction together with its progress row, so a failure leaves the column, the
--      backfill, the index and the progress table all unchanged, and the next start retries
--      from the beginning. The backfill statement is idempotent, so retrying costs nothing.
--   3. HOW IT IS REVERSED: see "REVERSAL" at the bottom of this file.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: this is the question that
--      causes incidents, so it is answered where the risk actually is — at the index, below.
--      In short: the Go code has not been changed yet, so between this migration and that
--      change every existing query keeps its exact current meaning, and no row changes
--      visibility anywhere.
--   5. RETENTION: nothing is removed. This file only ADDS a column and re-states the predicate
--      of one index. No archival record loses a field, and no business code is touched.

-- ---------------------------------------------------------------------------
-- 1. The column.
--
-- DEFAULT false, NOT true, and that is the whole safety property of this file. "This person can
-- sign in" is authority, and authority is granted explicitly, by a named administrator, leaving
-- a trail (rule 6). With DEFAULT true, every row an Excel import drops into the directory
-- (§6 of the specification imports by email or by name+unit) would become a sign-in account
-- without anybody deciding so and without anything reporting it — fail-open, on the
-- authentication path. DEFAULT false is the fail-closed reading: a new row can do nothing until
-- somebody says otherwise.
--
-- ADD COLUMN IF NOT EXISTS so the file can be applied to a database that already has it — the
-- integration suites apply migrations more than once, and 0001/0002 are both written this way.
--
-- ON A PARTITIONED TABLE: nguoi_dung is PARTITION BY HASH (tenant_id), MODULUS 32 (ADR 0010).
-- ALTER TABLE on the parent recurses into all 32 partitions; a column may not be added to a
-- partition on its own, so the parent is not merely the convenient place, it is the only one.
-- A constant DEFAULT has needed no table rewrite since PostgreSQL 11 (0002 already sets 13 as
-- the floor), so the ACCESS EXCLUSIVE lock here is taken and released immediately.
-- ---------------------------------------------------------------------------
ALTER TABLE nguoi_dung
    ADD COLUMN IF NOT EXISTS co_tai_khoan BOOLEAN NOT NULL DEFAULT false;

-- The distinction is written into the catalogue as well, not only into this file. A person
-- reading `\d nguoi_dung` at a psql prompt is precisely the person about to confuse the two
-- columns, and they will not have this migration open. COMMENT ON is idempotent: it replaces.
COMMENT ON COLUMN nguoi_dung.co_tai_khoan IS
    'Has a sign-in account. Granted explicitly; default false. NOT the same as dang_hoat_dong.';
COMMENT ON COLUMN nguoi_dung.dang_hoat_dong IS
    'Not locked out. Meaningful only where co_tai_khoan is true. NOT "has an account".';

-- ---------------------------------------------------------------------------
-- 2. Backfill for rows that already exist.
--
-- `mat_khau_hash <> ''` is the only evidence of an account there has ever been in this schema:
-- 0001 defaults the hash to the empty string, and only an account creation or a password reset
-- ever writes a real argon2id hash into it. So "has a hash" is exactly "was treated as an
-- account before this column existed", and that is what is carried forward.
--
-- NO `deleted_at IS NULL` FILTER, on purpose. Soft-deleted rows are kept for the audit trail
-- (rule 7); leaving them at false would rewrite history — a person whose account was withdrawn
-- would read, afterwards, as somebody who never had one. The column describes what the record
-- WAS, and archival records are not edited to suit a later schema.
--
-- `cap_nhat_luc` IS DELIBERATELY NOT TOUCHED. It means "a person last edited this record"; a
-- schema backfill is not a business edit. Stamping it would show every staff record on
-- Cấu hình → Người dùng as just-modified, by nobody, on the day of a deployment.
--
-- `NOT co_tai_khoan` is safe here because the column is NOT NULL with a default: every existing
-- row already has a value. That is NOT the pattern for the soft-delete flag, where rows predate
-- the column and a direct `= false` comparison silently drops all of them — the reason
-- soft-delete filters are written as `deleted_at IS NULL` / `$ne: true`, never `= false`.
--
-- WHY THIS IS ONE STATEMENT AND NOT A RESUMABLE PER-COMMUNE LOOP (rule 7, invariant 5): the
-- runner gives each file ONE transaction, so a per-commune loop inside it could not be resumed
-- either — an interruption rolls the whole file back regardless. What it would add is lock
-- churn on 32 partitions for a table that holds tens of rows per commune. The genuinely
-- resumable per-commune backfill mechanism does not exist yet and is named as missing in the
-- core/migrate package comment; the threshold for needing it is a statement whose lock time is
-- noticeable, which this is not. If `nguoi_dung` ever grows to where that changes, the backfill
-- moves to a tool under tools/ with a progress table — it does not stay here and get bigger.
--
-- The NOTICE lines are the "recording progress" half an operator can actually read: how many
-- rows were rewritten, and how the accounts fall per commune afterwards. Counts and tenant ids
-- only — never a name, an email or a phone number (rule 3).
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    so_dong int;
    r       record;
BEGIN
    UPDATE nguoi_dung
       SET co_tai_khoan = true
     WHERE mat_khau_hash <> ''
       AND NOT co_tai_khoan;
    GET DIAGNOSTICS so_dong = ROW_COUNT;

    RAISE NOTICE '0003 backfill: % row(s) set co_tai_khoan = true', so_dong;

    IF so_dong > 0 THEN
        FOR r IN
            SELECT tenant_id, count(*) AS n
            FROM nguoi_dung
            WHERE co_tai_khoan
            GROUP BY tenant_id
            ORDER BY tenant_id
        LOOP
            RAISE NOTICE '0003 backfill: commune % now has % account(s)', r.tenant_id, r.n;
        END LOOP;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 3. nguoi_dung_dang_nhap — narrowed to people who actually have an account.
--
-- This index exists to serve ONE query: sign-in by email (CanBoStore.TheoEmail). Once the
-- sign-in path filters `co_tai_khoan`, a directory entry that can never sign in has no business
-- occupying an entry in it. On the specification's own seed that is more than half the rows —
-- 26 in the directory against 12 accounts.
--
-- A partial index's predicate cannot be altered in place, so it is dropped and recreated. That
-- is atomic here: core/migrate wraps each file in a single transaction, so there is no window in
-- which the table has no index at all. CREATE INDEX CONCURRENTLY is not an option and is not
-- wanted — it cannot run inside a transaction, and PostgreSQL does not support it on a
-- partitioned table in any case.
--
-- DROP ON THE PARENT ONLY. An index on a partitioned parent has a child index on each of the 32
-- partitions, attached to it; PostgreSQL refuses to drop a child on its own ("cannot drop index
-- ... because index ... requires it"). Dropping the parent takes the children with it, and the
-- CREATE below recreates all 33 relations. Nothing here should ever be written per partition.
--
-- QUESTION 4 — WHAT CHANGES MEANING WHILE THIS IS HALF-APPLIED, i.e. after this migration and
-- before the Go change lands:
--
--   * NO ROW CHANGES VISIBILITY. An index is not a filter; the WHERE clauses in Go decide what
--     is returned, and none of them has changed yet. TheoEmail still returns exactly the rows
--     it returned yesterday.
--   * The only effect is on PLAN CHOICE. A partial index is usable only where the planner can
--     prove the query's predicate implies the index's; the current query does not mention
--     `co_tai_khoan`, so it stops using this index. It does not fall back to a sequential scan:
--     `UNIQUE (tenant_id, email)` from 0001 is itself an index on the same two columns over the
--     whole table, and it serves the lookup with one extra heap check for the flags. That is
--     what makes it safe to narrow this index BEFORE the Go turn rather than after: the
--     in-between state costs a few microseconds and changes no answer.
--
-- HONEST NOTE ON WHAT THIS INDEX IS WORTH. Because of that unique index, this one is close to
-- redundant at today's size, and it is kept for two reasons that are not performance: the
-- predicate is a statement IN THE SCHEMA of what "may sign in" means — the same statement this
-- whole migration exists to make unambiguous — and it becomes a real saving on the day a
-- commune's public directory is much larger than its set of accounts, which is the direction
-- §6 of the specification (Excel import of the directory) points.
--
-- `nguoi_dung_theo_bo_phan` is deliberately NOT narrowed the same way: it serves the directory
-- screen and the org chart, which must show people who have no account at all. Adding
-- `co_tai_khoan` there would hide exactly the 26 people the directory is for.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS nguoi_dung_dang_nhap;

CREATE INDEX IF NOT EXISTS nguoi_dung_dang_nhap
    ON nguoi_dung (tenant_id, email)
    WHERE deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong;

-- ---------------------------------------------------------------------------
-- 4. NO CHECK CONSTRAINT HERE (co_tai_khoan => mat_khau_hash <> '').
--     THE ARGUMENT BELOW EXPIRED ON 2026-09-22. THE CONSTRAINT NOW EXISTS, IN 0009.
--
-- ⚠ WHY THIS BLOCK WAS EDITED AFTER THE FILE HAD BEEN APPLIED — read before doing it again.
-- This file's own header says an applied migration is never edited, and core/migrate enforces
-- that with a checksum (ErrChecksumLech). The edit was made anyway, ONCE, on 2026-09-22, for a
-- reason that was checked rather than assumed: at that moment no durable database had this file
-- applied. tools/schema-smoke creates a throwaway schema per run and removes it; every *_pg_test
-- suite does the same with a schema named after the clock; there is no deployment. So the whole
-- cost of the changed checksum was zero, and the cost of LEAVING IT was not: the paragraph below
-- told every future session that the customer's onboarding flow was still open, which would have
-- stopped somebody from acting on a decision already taken — or worse, prompted them to "restore"
-- consistency by dropping the constraint 0009 adds. A stale argument reads exactly like a live one.
-- THIS IS NOT A PRECEDENT. The next such edit needs the same evidence, freshly checked, and if any
-- database anywhere has applied the file the answer is no — write the correction in a new file.
--
-- WHAT THE BLOCK SAID, AND WHY IT IS NO LONGER TRUE:
--
--   THE CASE FOR IT was always sound: the constraint makes one nonsense state impossible — a
--   record claiming to be an account while holding nothing to authenticate against — and a
--   constraint in the database holds against every writer, including a psql prompt.
--
--   THE CASE AGAINST, which won at the time: "create the account, e-mail an activation link, the
--   person sets their own password" is a normal way to onboard staff, and it needs precisely the
--   state the constraint forbids — co_tai_khoan = true with an empty hash, for as long as the
--   link is outstanding. While the customer had not chosen, writing the constraint would have
--   decided their question by making one answer impossible to implement.
--
--   THAT ANSWER IS GONE. Open question #9 was decided on 2026-09-22 (kb/00-foundation/
--   open-questions.json): the SYSTEM MINTS A TEMPORARY PASSWORD and forces a change at the first
--   sign-in — answer (b). The activation-link flow (c) was rejected, on the customer's own
--   ground: the mail server is per-commune configuration and may not be filled in yet, so a
--   newly onboarded commune could not send the link that creates its first account. #17, decided
--   the same day, closes the other route to a hash-less account by ruling out self-service
--   password reset. No flow left in this system needs co_tai_khoan = true with an empty hash.
--
-- WHERE THE CONSTRAINT LIVES NOW: migration 0009_tai_khoan_tam_va_danh_ba_can_bo.sql §3, under
-- the name this block predicted, `nguoi_dung_co_tai_khoan_co_mat_khau`, and the same file adds
-- the `phai_doi_mat_khau` column that answer (b) needs. The asymmetry noted here held exactly as
-- written: adding it later was one line in a new migration.
--
-- STILL TRUE, AND NOT REPLACED BY THE CONSTRAINT: an empty hash cannot sign in by accident,
-- because core/password.KiemTra cannot parse '' as an argon2id encoding and returns an error, so
-- verification fails closed. That is a property of the password package, not of this schema.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no rollback on purpose: undoing DDL on archival records is an administrative
-- act with a person present, not something a process decides at 3am. So the reverse is written
-- out here, to be run by hand.
--
-- The index half is fully reversible and loses nothing:
--
--      DROP INDEX IF EXISTS nguoi_dung_dang_nhap;
--      CREATE INDEX IF NOT EXISTS nguoi_dung_dang_nhap ON nguoi_dung (tenant_id, email)
--          WHERE deleted_at IS NULL AND dang_hoat_dong;
--      DELETE FROM schema_migration WHERE ten = '0003_nguoi_dung_co_tai_khoan.sql';
--
-- The column half is ADDITIVE: nothing existing was overwritten, so "reverting" is achieved by
-- ceasing to read the column, at no cost. Actually DROPPING it once it holds data is a
-- different act — dropping a populated column is rule 7, stop condition #2, and needs an
-- explicit decision by the user plus a verified backup. It is not written here as a runnable
-- line, because a runnable line is a line that gets run:
--
--      -- ALTER TABLE nguoi_dung DROP COLUMN co_tai_khoan;   -- USER DECISION + BACKUP ONLY
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT, silently, until the first business write fails. 0002 names the gap this repeats
-- to close — the check only describes the state after the NEWEST migration that carries it, so
-- every new file must end with it. This file adds no table, so it is expected to find nothing;
-- it runs anyway, because the run after which it is missing is the one that needed it.
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
