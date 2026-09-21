-- identity — the commune's processing deadlines, in WORKING HOURS (ADR 0029).
--
-- WHY A NEW FILE AND NOT AN EDIT OF AN EARLIER ONE: 0001–0007 have been applied and
-- core/migrate compares the checksum of every applied file at startup. Editing an applied file
-- either stops the service (ErrChecksumLech) or leaves two databases claiming one schema
-- version while holding two different schemas.
--
-- WHY IN `identity` AND NOT IN `petitions` OR `documents` — ADR 0029, decided 2026-09-20. The
-- table is read by at least two services (`van-ban-den` is documents' work, `phan-anh` and
-- `nhiem-vu` are petitions') and owned by neither of them; ownership follows the CADENCE OF
-- CHANGE, and SLA hours change when a commune's administrative policy changes — the same
-- cadence as its office hours and its holidays, which already live here (0006). Writing this
-- table in either reading service is ADR 0029 stop condition #2, "even temporarily".
--
-- ONE TABLE IN THIS FILE, and the calendar's "all three or none" argument does NOT apply: the
-- three calendar tables answer one question together, this one answers a different question
-- (`bao lâu`) from the one they answer (`lúc nào`). It is deliberately NOT appended to 0006 —
-- that file has been applied.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above the CREATE TABLE
-- (ADR 0021); tools/kb reads them into kb/30-indexes/data-ownership.json.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. The table is new and this file writes no row into it —
--      see "NOTHING IS SEEDED" below. The ceiling in sight is three work types times the twelve
--      platform field codes plus a default row each, about 39; the specification's own table
--      (docs/ui-ux/14-cau-hinh.md §8) has 16 rows.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. A failure leaves nothing behind and the next start
--      retries from the beginning; every statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS one
--      table and its indexes. No existing table, column, index or constraint is touched, so
--      every query that runs today returns exactly what it returned before and no row changes
--      visibility. The risk moves to the NEXT change — the first one that writes rows here, and
--      the first deadline computed from them.
--   5. RETENTION: nothing is removed. An SLA row is THE BASIS OF AN ISSUED COMMITMENT: when an
--      inspection asks why a petition received on 30/04 was due 16 working hours later, the
--      answer is the row as it stood that day. So the soft-delete columns are present from the
--      start and a row is never hard-deleted — rule 7, and rule 10 invariant 2, which fixes a
--      deadline at the act that sets it and forbids recomputing it afterwards.
--
-- ---------------------------------------------------------------------------
-- NOTHING IS SEEDED, AND HERE THAT IS THE HEAVIEST LINE IN THE FILE.
--
-- Three separate reasons, any one of them sufficient:
--
--   * A row carries `tenant_id`, so a seeded row would belong to ONE NAMED COMMUNE (ADR 0029,
--     consequence #2: "hạt giống là gieo cho từng xã"). The migration path has no commune in it
--     and deliberately accepts none (ADR 0013); fetching the list would mean reading
--     `platform`'s tenant registry from another service's migration — rule 2, forbidden #2.
--   * A NUMBER IN THIS TABLE IS A PROMISE A PUBLIC AUTHORITY MAKES TO A CITIZEN. Seeding it
--     is the software vendor answering, on behalf of a commune, "how long will you take" —
--     and rule 10, forbidden #3 puts a commune's SLA figures out of reach of the source code
--     for exactly that reason. The sixteen rows of docs/ui-ux/14-cau-hinh.md §8 came from ONE
--     real commune's prototype; they are an EXAMPLE, not a national standard.
--   * Copying those sixteen rows into SQL would create the second copy of a fact that already
--     has an owning file (rule 9). Two copies drift, and the drifted one is the one that ends
--     up in a database.
--
-- WHAT AN EMPTY TABLE MEANS, and it has to be enforced by the code that reads it, not here: a
-- commune with no row for a work type has NO deadline configured, so nothing may be computed.
-- The reader must REFUSE rather than fall back to "24 hours" or to the specification's numbers
-- — a silent default here is a commitment INVENTED BY SOFTWARE and then told to a citizen
-- (rule 10; the fail-closed principle CLAUDE.md opens with). domain.VanDeCuaSLA is what makes
-- that refusal impossible to skip by accident, and it names the empty case rather than
-- returning an error, for the same reason domain.VanDeLichTrong does: the configuration screen
-- that would FIX the problem has to be able to load.
--
-- Today that is EVERY commune. The onboarding step that sows a commune's first rows does not
-- exist in this repository.
--
-- ---------------------------------------------------------------------------
-- THIS TABLE SUPPLIES NUMBERS. IT DOES NOT SUPPLY THE ARITHMETIC.
--
-- Every column here is a count of WORKING HOURS (ADR 0007). Turning "16 working hours" into an
-- instant needs the commune's calendar, and there is exactly one implementation of that —
-- domain.TienGioLamViec, published as `AdvanceWorkingHours` (0006, ADR 0007). A second one
-- would be a second answer to one question, and the two would only disagree on nights,
-- weekends, `ngay_nghi_le` and `ngay_lam_bu` — the moments a citizen notices.
--
-- SO THERE IS NO `so_ngay` COLUMN AND THERE NEVER WILL BE. The security field really is 2
-- WORKING HOURS to acknowledge; no count in days can express that, and rounding it up to one
-- day widens the commitment fourfold (ADR 0007, §Bối cảnh).
--
-- AND THERE IS NO `qua_han` / `is_overdue` COLUMN. Overdue is DERIVED from the stored deadline
-- against now (rule 10, invariant 3). A column set by a nightly job is wrong the moment the job
-- is late, the clock skews, or a holiday is added, and the stale copy is the one that reaches
-- the report going upward.
--
-- ---------------------------------------------------------------------------
-- WHICH COLUMN ANSWERS WHICH ACT — ADR 0028, and the reason there are two deadline columns.
--
--   gio_tiep_nhan   read at the act that CREATES the row: `han_tiep_nhan` (ADR 0028, decision
--                   E). On the citizen channels nobody knows the field yet, so this is read
--                   from the DEFAULT row — `linh_vuc IS NULL`.
--   gio_xu_ly_xong  read at the act that SETTLES THE FIELD: `han_xu_ly_xong`, from the row of
--                   the field that was settled. Deferring it is what removed the 56-hour
--                   ceiling ADR 0028 describes; reading it at insert time rebuilds that ceiling
--                   and is stop condition #1 of that ADR.
--
-- Both deadlines count from the same instant; what differs is WHEN each is fixed.
--
-- ---------------------------------------------------------------------------
-- THE ANCHOR OF THE THREE ALERT COLUMNS IS NOT DECIDED, AND THIS FILE DOES NOT DECIDE IT.
--
-- `gio_sap_den_han` is unambiguous: docs/ui-ux/14-cau-hinh.md §8 states it in the sentence it
-- requires be kept — hours REMAINING before the deadline, and it drives three things at once
-- (when the reminder is sent, what the "Sắp đến hạn" filter selects, the figure in the bell).
--
-- `gio_bao_lanh_dao` and `gio_bao_chu_tich` are NOT. The specification contradicts itself:
--
--   §8 column heading   "Báo lãnh đạo trực tiếp — sau 24 giờ"        after WHAT is not said
--   §9 escalation job   "việc trễ quá số ngày đã đặt thì báo lên     after the deadline, and
--                        trưởng bộ phận, trễ gấp đôi thì báo lên      the president's figure is
--                        chủ tịch"                                    DOUBLED, not a column
--
-- The columns are created because they belong to the one row a commune edits on one screen, and
-- because the moment a table is born is the only moment its shape is cheap. THE MEANING IS NOT
-- CREATED WITH THEM: nothing may compute an escalation from these two numbers until somebody
-- answers "after what", and a reader that guesses will notify a commune's leadership on a basis
-- nobody chose. That question is raised with the user; it is NOT settled here.
--
-- ---------------------------------------------------------------------------
-- `linh_vuc` IS A VALUE, NOT A FOREIGN KEY, AND THAT IS NOT NEGOTIABLE HERE.
--
-- The field codes are a CLOSED platform-level code set living in service `platform`
-- (ADR 0026, closing open question #22). A foreign key from here would be a foreign key across
-- a service boundary — rule 2, forbidden #2. So the code is stored as text, unvalidated by the
-- database.
--
-- WHAT THAT COSTS, WITH EVIDENCE ALREADY IN THE SPECIFICATION: docs/ui-ux/14-cau-hinh.md:308
-- still carries an SLA row pointing at `ve-sinh-moi-truong`, a code no longer in the catalogue,
-- and the screen renders the raw code. The answer is A CHECK AT WRITE TIME, not a constraint at
-- read time (ADR 0024, §Cái giá của dòng Khối nhiệm vụ; ADR 0026 §2).
--
-- THAT WRITE-TIME CHECK IS WHY THIS FILE SHIPS WITHOUT A WRITE PATH. Validating a field code
-- means reading `platform`'s tier-1 code set, and WHETHER THAT READ IS gRPC OR AN
-- EVENT-FED REPLICA HAS NO ADR — ADR 0026 stop condition #2 names writing that read path
-- without one as the thing not to do. Same shape as the three calendar tables, which have had
-- no write path since 0006 for their own reason.
--
-- ---------------------------------------------------------------------------
-- @entity: ProcessingDeadline
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS sla (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    -- THREE VALUES, AND A FOURTH IS A DECISION RATHER THAN A DEPLOYMENT (ADR 0029, stop
    -- condition #1): each value is a business domain admitted into this table.
    --
    -- TEXT + CHECK AND NOT A PostgreSQL ENUM TYPE, following 0004 and 0005. An enum type makes
    -- the value list a database object a migration has to ALTER, and the set of admitted values
    -- is precisely the thing that must not change quietly.
    loai_viec     TEXT        NOT NULL,

    -- NULL IS THE DEFAULT ROW — the row that applies to every field without one of its own
    -- (docs/ui-ux/14-cau-hinh.md:316, "Mặc định cho mọi lĩnh vực"). It is not "unknown" and not
    -- "not entered".
    --
    -- The empty string is REFUSED below, so NULL and '' cannot both mean "default". Two ways to
    -- say one thing is the drift rule 9 exists to prevent, and here the drift would be a
    -- commune's default deadline silently splitting into two rows.
    linh_vuc      TEXT,

    -- EVERY ONE OF THESE FIVE IS A COUNT OF WORKING HOURS. See the block above on why there is
    -- no day-based column and no arithmetic in this service's schema.
    --
    -- FIVE ADJACENT INTEGER COLUMNS ARE THE DEFECT CLASS THIS TABLE CARRIES: a read that scans
    -- them in the wrong order produces no error at all, only a different promise. The store's
    -- column list and Scan are asserted against each other by name in sla_test.go for that
    -- reason.
    gio_tiep_nhan     INTEGER NOT NULL,   -- ADR 0028: fixes han_tiep_nhan when the row is created
    gio_xu_ly_xong    INTEGER NOT NULL,   -- ADR 0028: fixes han_xu_ly_xong when the field is settled
    gio_sap_den_han   INTEGER NOT NULL,   -- hours REMAINING at which "sắp đến hạn" begins
    gio_bao_lanh_dao  INTEGER NOT NULL,   -- anchor UNDECIDED — see the block above
    gio_bao_chu_tich  INTEGER NOT NULL,   -- anchor UNDECIDED — see the block above

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- ONE LIVE ROW PER (commune, work type, field), INCLUDING THE DEFAULT ROW.
    --
    -- The plain unique key below cannot do this on its own: PostgreSQL does not consider two
    -- NULLs equal, so UNIQUE (tenant_id, loai_viec, linh_vuc) admits any number of default rows
    -- for one commune — and which of them a deadline is read from would depend on read order,
    -- which is to say on nothing. ADR 0029, consequence #1 calls for a separate unique key for
    -- the NULL case by name.
    --
    -- THE OBVIOUS FORM IS A PARTIAL UNIQUE INDEX AND IT IS DELIBERATELY NOT USED, for exactly
    -- the reason written out at 0005:247 — unique keys on a PARTITIONED table are restricted,
    -- and whether the partial variant is accepted cannot be verified from this repository: no
    -- PostgreSQL is reachable and VIGOV_TEST_DSN is unset, so the integration suites skip
    -- without running a statement. A migration that fails at startup stops the service.
    --
    -- So the same shape 0005 proved out: a GENERATED column that folds NULL into a value, and a
    -- plain unique key over plain columns.
    --
    -- IT GOES NULL WHEN THE ROW IS SOFT-DELETED, AND THAT IS THE OPPOSITE OF WHAT THE CATALOGUE
    -- TABLES DO (0005:69, "UNIQUE (tenant_id, ma) counts soft-deleted rows too"). The catalogues
    -- count them because AN ISSUED CODE IS NEVER REISSUED (rule 7, invariant 3): archival
    -- records hold that code as a value. An SLA row issues nothing. The deadlines it produced
    -- are STORED ON THE PETITIONS THEMSELVES (rule 10, invariant 2) and are never recomputed
    -- from this table, so a commune that removes the row for `dien` and later adds one back is
    -- not reusing an identifier — it is changing its policy, from that moment forward, which is
    -- what ADR 0007 decision 6 already permits.
    linh_vuc_khoa TEXT GENERATED ALWAYS AS
                      (CASE WHEN deleted_at IS NULL THEN coalesce(linh_vuc, '') END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, loai_viec, linh_vuc_khoa),

    CONSTRAINT sla_id_la_ulid CHECK (length(id) = 26),

    CONSTRAINT sla_loai_viec_hop_le
        CHECK (loai_viec IN ('van-ban-den', 'phan-anh', 'nhiem-vu')),

    -- '' IS NOT A FIELD CODE AND IT IS NOT THE DEFAULT ROW EITHER. Without this, an empty
    -- string arriving from a form would become a second default row that no screen distinguishes
    -- from the real one.
    CONSTRAINT sla_linh_vuc_khong_rong
        CHECK (linh_vuc IS NULL OR btrim(linh_vuc) <> ''),

    -- ZERO WORKING HOURS IS A DEADLINE THAT IS BREACHED AT THE INSTANT IT IS MADE, and a
    -- negative one is a typo. Strict from the start on purpose: relaxing a bound later on an
    -- empty table is one cheap ALTER, whereas a 0 that has been stored acquires a meaning
    -- somebody invented ("no alert", "immediately") that cannot be told apart afterwards from a
    -- commune that typed it by mistake.
    --
    -- THERE IS NO UPPER BOUND AND NO ORDERING CHECK BETWEEN THE COLUMNS, and that is measured
    -- rather than lazy: the specification's own `nhiem-vu` row (14-cau-hinh.md:312) has
    -- gio_sap_den_han = 72 against gio_xu_ly_xong = 40, so the intuitive rule "the warning
    -- window fits inside the deadline" would refuse data the customer already uses. A
    -- constraint that refuses real configuration is worse than none: it is discovered on the
    -- day a commune is being set up.
    CONSTRAINT sla_gio_phai_duong
        CHECK (gio_tiep_nhan > 0 AND gio_xu_ly_xong > 0 AND gio_sap_den_han > 0
               AND gio_bao_lanh_dao > 0 AND gio_bao_chu_tich > 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS sla_p%s PARTITION OF sla '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE sla IS
    'Thoi han xu ly cua xa, dem bang GIO LAM VIEC (ADR 0007, ADR 0029). Bang nay cap CON SO; '
    'phep doi so gio ra moc han thuoc ve AdvanceWorkingHours va chi co mot ban cai dat. '
    'linh_vuc = NULL la dong mac dinh cho moi linh vuc. Khong seed mot dong nao: mot con so o '
    'day la cam ket cua co quan voi nguoi dan, xa tu dat.';
COMMENT ON COLUMN sla.linh_vuc IS
    'Ma linh vuc, giu duoi dang GIA TRI — bo ma tang 1 nam o service platform (ADR 0026) nen '
    'khoa ngoai o day la khoa ngoai xuyen service. Phep kiem thuoc duong GHI, khong phai duong '
    'doc. NULL = dong mac dinh.';
COMMENT ON COLUMN sla.gio_bao_lanh_dao IS
    'So gio lam viec truoc khi bao lanh dao truc tiep. MOC DEM CHUA DUOC CHOT — dac ta §8 va §9 '
    'noi hai dang khac nhau. Khong tinh leo thang tu cot nay cho toi khi co nguoi tra loi.';
COMMENT ON COLUMN sla.gio_bao_chu_tich IS
    'So gio lam viec truoc khi bao Chu tich. MOC DEM CHUA DUOC CHOT — xem gio_bao_lanh_dao.';

-- The one query the configuration screen and the deadline path both run: this commune's whole
-- table, in a TOTAL order. The default row comes first because it is the row everything else
-- falls back to.
--
-- NULLS FIRST IS DECLARED ON THE INDEX TOO. An index built with PostgreSQL's ASC default sorts
-- NULLs LAST and cannot serve this ORDER BY, so the two would silently disagree and the planner
-- would sort every read. With sixteen rows that costs nothing today; the point is that an index
-- which does not match the only statement that reads the table is an index nobody can trust.
CREATE INDEX IF NOT EXISTS sla_danh_sach
    ON sla (tenant_id, loai_viec, linh_vuc NULLS FIRST)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on the basis of issued
-- commitments is an administrative act carried out with a person present, not something a
-- process decides at 3am. So the reverse is written here, to be run by hand.
--
--   BEFORE ANY COMMUNE HAS AN SLA ROW — the table is empty. Reversal is exact and loses
--   nothing: drop sla (the 32 partitions go with the parent), then remove this file's progress
--   row from schema_migration, keyed on ten = '0008_sla.sql', so the runner applies it again.
--   Written as prose and not as a runnable line because a runnable line is a line that gets run.
--
--   AFTER THE FIRST DEADLINE HAS BEEN COMPUTED — there is no reversal. These rows are the basis
--   on which a commitment was made to a citizen and reported upward; dropping them destroys the
--   only record of why a file was due when it was due. That is rule 7 stop condition #1: it
--   needs an explicit decision by the user and a verified backup, and it is not an operation
--   this file offers.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): this file contains no backfill — it creates
-- one empty table and rewrites no existing row — so there is nothing to resume and no commune
-- to iterate. The genuinely per-commune resumable backfill mechanism still does not exist
-- (ADR 0013, §Giới hạn); the first migration that rewrites rows here will need it.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0006.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until
-- the first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check only describes the state after
-- the NEWEST migration that carries it, so every new file ends with it.
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
