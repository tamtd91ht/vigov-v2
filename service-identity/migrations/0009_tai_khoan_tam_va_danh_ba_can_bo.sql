-- identity — the schema half of three customer decisions taken on 2026-09-22.
--
-- This file carries out what open questions #9, #15 and #16 settled
-- (kb/00-foundation/open-questions.json, all three `status: DECIDED`, `decided_on: 2026-09-22`).
-- Nothing here is an agent's reading of what the customer probably meant: each section names the
-- question it implements, and a section whose question is still OPEN is not in this file.
--
--   #9  THE FIRST PASSWORD. Answer (b): the system MINTS a temporary password and forces a change
--       at the FIRST sign-in. Answer (c) — an activation link by e-mail — is OFF THE TABLE, and
--       that is what unblocks the CHECK constraint 0003 §4 deliberately left out (§3 below).
--   #15 THE STAFF CODE is SYSTEM-GENERATED; there is no Mã box on either form. The schema half was
--       already right in 0001 (NOT NULL, UNIQUE (tenant_id, ma)); what this file adds is the
--       statement of the rule where a reader will find it (§4).
--   #16 `dien_thoai` AND `di_dong` ARE TWO FIELDS, because they are two kinds of data in law: an
--       office landline is duty information, a personal mobile is personal data under Decree
--       13/2023/NĐ-CP. One column would force one masking rule onto both, and the rule that is
--       right for one is always wrong for the other (§2).
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0001/0003: core/migrate checksums every applied file at
-- startup and stops the service when one has changed (ErrChecksumLech). 0003's own header says so.
-- The one edit this turn does make to 0003 — the §4 comment block, whose argument #9 has now ended
-- — is explained there, and it is an EXCEPTION with evidence, not a precedent.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
-- 1. HOW MANY ROWS PER COMMUNE. ZERO, everywhere, and this is measured rather than assumed —
--    it is what makes §2 free instead of impossible. `nguoi_dung` has NO write path in this
--    repository outside test fixtures: the whole repository contains no `INSERT INTO nguoi_dung`
--    in non-test code, and the only two UPDATEs touch `dang_nhap_gan_nhat` and `mat_khau_hash`
--    (internal/store/can_bo.go:86,103). The staff-creation and directory-write flows are both
--    recorded as not started (kb/90-ephemeral/tien-do/service-identity.json, items
--    `cap-tai-khoan-can-bo` and `tuyen-ghi-danh-ba-can-bo`, `trang_thai: chua_lam`) — they were
--    blocked on exactly the questions this file implements. The only run against a real server
--    was 2026-09-21, into a throwaway empty schema (tools/schema-smoke).
--    The ceiling in sight is still the specification's seed: 26 directory entries and 12 accounts
--    per commune. Tens of rows, not millions.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied. core/migrate runs each file inside ONE
--    transaction together with its progress row, so a failure leaves all three columns, the
--    constraint and the progress table unchanged, and the next start begins again from the top.
--    Every statement here is written to be safe on a second run (ADD COLUMN IF NOT EXISTS,
--    COMMENT ON replaces, and the constraint is added only if pg_constraint does not have it).
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED — the question that causes the
--    incidents, so it gets the long answer:
--
--      * NO ROW CHANGES VISIBILITY. This file adds columns and one constraint. It rewrites no
--        row, narrows no index predicate and touches no WHERE clause. Every query in Go returns
--        exactly the rows it returned before.
--      * `phai_doi_mat_khau` DEFAULT true IS THE ONE THING WITH A FUTURE EFFECT, and it is
--        deliberate. Between this migration and the Go change that reads the column, nothing
--        reads it, so nothing changes. On the day the sign-in path starts enforcing it, EVERY
--        account that existed before this file is sent to "change your password" once. Today
--        that set is empty (question 1). If this file is ever applied to a commune that already
--        holds accounts, that is the effect, and it is the INTENDED one: under #9 every password
--        an account has was typed by an administrator, and #9's whole point is that the
--        administrator must stop knowing it.
--      * The CHECK in §3 is a WRITE-path constraint. No read changes. A writer that today
--        produces `co_tai_khoan = true` with an empty hash starts failing loudly — see §3 for the
--        three fixtures in this repository that did exactly that and why they were wrong.
--
-- 5. RETENTION. Nothing is removed, nothing is retyped, no issued code is touched. No archival
--    record loses a field. Rule 7's stop conditions are not reached by this file.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): this file contains NO BACKFILL — it rewrites
-- no row — so there is nothing to resume and no commune to iterate. What it does do per commune
-- is REPORT: §3 counts, per `tenant_id`, the rows that would refuse the new constraint, before
-- trying to add it, so an operator is told which commune to look at rather than being handed one
-- SQLSTATE 23514. Counts and tenant ids only, never a name, an e-mail or a number (rule 3).
-- The genuinely resumable per-commune backfill mechanism still does not exist (ADR 0013,
-- §Giới hạn); the first migration here that rewrites rows will need it.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. phai_doi_mat_khau — question #9, answer (b).
--
-- "This account is still carrying a password somebody else chose." It is set true when an
-- administrator mints or resets a temporary password, and cleared ONLY by the person themselves
-- changing it. #17 (decided the same day) rules out self-service reset by e-mail, so the
-- administrator path is the only way a password is ever set for somebody else — which is exactly
-- the state this column exists to mark.
--
-- DEFAULT true, AND THE DIRECTION IS THE WHOLE SAFETY PROPERTY — note it is the OPPOSITE
-- direction from `co_tai_khoan` in 0003, for the same reason. There, `true` meant MORE authority,
-- so the default was false. Here, `true` means LESS: you may not carry on with this password.
-- Both defaults are the fail-closed one.
--
--   DEFAULT false would mean an INSERT that forgets this column mints an account whose
--   administrator-chosen password is valid forever. Nothing reports it; the account works
--   perfectly; the only visible symptom is that a second person knows the credentials of a staff
--   member whose name appears in the audit trail (rule 6, invariant 2). That is fail-open on the
--   authentication path.
--
--   DEFAULT true costs, at worst, one extra "change your password" screen for somebody who did
--   not need it. Loud, harmless, and noticed immediately.
--
-- ON A DIRECTORY-ONLY ROW (co_tai_khoan = false) this column says nothing, exactly as
-- `dang_hoat_dong` says nothing there. There is no account to force a change on. It is not
-- constrained to false in that case, because a constraint coupling the two would have to be
-- dropped and re-added on every transition between them for no benefit.
--
-- ON A PARTITIONED TABLE: `nguoi_dung` is PARTITION BY HASH (tenant_id), MODULUS 32 (ADR 0010).
-- ALTER TABLE on the parent recurses into all 32 partitions, and a column may not be added to a
-- partition on its own, so the parent is the only place this can be written. A constant DEFAULT
-- has needed no table rewrite since PostgreSQL 11, so the ACCESS EXCLUSIVE lock is taken and
-- released immediately.
-- ---------------------------------------------------------------------------
ALTER TABLE nguoi_dung
    ADD COLUMN IF NOT EXISTS phai_doi_mat_khau BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN nguoi_dung.phai_doi_mat_khau IS
    'Must change password at next sign-in (open question #9, decided 2026-09-22). '
    'True while the account still carries a password an administrator chose; cleared ONLY when '
    'the person changes it themselves. DEFAULT true is fail-closed — see migration 0009 §1. '
    'Meaningless where co_tai_khoan is false: there is no account to force.';

-- ---------------------------------------------------------------------------
-- 2. di_dong_ca_nhan — question #16. TWO COLUMNS, BECAUSE THEY ARE TWO KINDS OF DATA IN LAW.
--
--   dien_thoai_co_quan  the office landline of a public office. DUTY INFORMATION. A commune
--                       publishes it on its own notice board; masking it between colleagues
--                       protects nobody. (Renamed from `dien_thoai` further down — see the block
--                       above the DO $$ guard for why the name itself is load-bearing.)
--   di_dong_ca_nhan     the staff member's own mobile. PERSONAL DATA under Decree 13/2023/NĐ-CP.
--
-- Merged into one column, every masking, export and publication rule would have to apply ONE
-- level to both, and the level that is right for one is always wrong for the other: safe enough
-- for the mobile means the office's own switchboard number is redacted from the commune's own
-- directory; convenient enough for the landline means a citizen-facing export carries personal
-- mobiles. That is #16's argument and it is a legal one, not a convenience one.
--
-- THE RE-CLASSIFICATION OF THE EXISTING COLUMN COST NOTHING, AND THAT WAS CHECKED RATHER THAN
-- HOPED. #16's own reversal note says the expensive case is a populated `dien_thoai` holding both
-- kinds of number, because no rule can tell them apart from the digits and each of the 26 people
-- per commune would have to be asked one at a time. That case does not exist here: `nguoi_dung`
-- has no write path at all yet, so no environment holds a single staff row (see question 1 at the
-- top of this file). There is NO BACKFILL because there is nothing to back-fill, and no row is
-- re-classified because no row exists. Had one existed, this file would not have been written
-- without asking the customer which kind of number was in that column.
--
-- TYPE: `TEXT NOT NULL DEFAULT ''`, matching the landline column, and deliberately NOT the `text null`
-- of the specification's table (docs/ui-ux/12-danh-ba-can-bo.md §7). Two ways to spell "no
-- number" — NULL and '' — is two ways every future query has to handle, and the one that gets
-- forgotten is NULL, which turns a comparison into NULL and drops the row silently. One
-- representation, matching the column beside it.
--
-- NO FORMAT CHECK. A CHECK on shape would reject a landline written with an area code in
-- brackets, a number with an extension, or an international prefix — all of them things a
-- commune's real directory contains. Normalisation belongs on the write path, where a person can
-- be told what was changed, not in a constraint that can only refuse.
-- ---------------------------------------------------------------------------
-- THE NAMES CARRY THE CLASSIFICATION, because a COMMENT is something you have to go and look up.
--
-- `dien_thoai` and `di_dong` differ in LAW but not in NAME: both read as "phone", and they sit
-- next to each other in every SELECT. The person who exports a directory to Excel, or ticks the
-- box that publishes a column to the Mini App, is choosing between two words that look the same.
-- `dien_thoai_co_quan` and `di_dong_ca_nhan` cannot be confused at a glance.
--
-- SAME MOVE 0003 ALREADY MADE, and for the same reason: it renamed `tai_khoan_hoat_dong` to
-- `co_tai_khoan` because the old name answered a different question from the one it was being
-- read for.
--
-- WHY THE RENAME IS HERE AND NOT IN 0001. Editing an applied migration changes its checksum and
-- `core/migrate` then refuses to start (ErrChecksumLech). §4 of 0003 was edited in this same
-- batch and carries the sentence "THIS IS NOT A PRECEDENT"; doing it a second time the same day
-- would make that sentence untrue. A rename forward costs one statement and leaves the history
-- honest — 0001 still says what was actually deployed.
--
-- FREE TODAY AND NOT TOMORROW: `nguoi_dung` is empty in every environment (§2 above records the
-- four independent checks). Once a commune has rows, a rename is a migration over live archival
-- data plus every query, export and screen that names the column.
--
-- IDEMPOTENT BY GUARD, not by `IF EXISTS` — PostgreSQL has no `RENAME COLUMN IF EXISTS`. The
-- rename recurses into all 32 partitions on its own, exactly like ADD COLUMN above.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_name = 'nguoi_dung' AND column_name = 'dien_thoai')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_name = 'nguoi_dung' AND column_name = 'dien_thoai_co_quan') THEN
        ALTER TABLE nguoi_dung RENAME COLUMN dien_thoai TO dien_thoai_co_quan;
    END IF;
END $$;

ALTER TABLE nguoi_dung
    ADD COLUMN IF NOT EXISTS di_dong_ca_nhan TEXT NOT NULL DEFAULT '';

-- The classification is written into the catalogue, not only into this file. The person about to
-- confuse these two columns is a person at a psql prompt reading `\d nguoi_dung`, and they will
-- not have this migration open. COMMENT ON is idempotent: it replaces.
COMMENT ON COLUMN nguoi_dung.di_dong_ca_nhan IS
    'Personal mobile. PERSONAL DATA under Decree 13/2023/NĐ-CP (open question #16, decided '
    '2026-09-22). NOT masked between staff of the same commune (#11 — they have to ring each '
    'other). ALWAYS masked in Excel exports (rule 3, invariant 4), and never published to the '
    'Mini App without that person''s own recorded consent (#12). NOT the same as '
    'dien_thoai_co_quan.';

COMMENT ON COLUMN nguoi_dung.dien_thoai_co_quan IS
    'Office landline of the public office. DUTY INFORMATION, not personal data (open question '
    '#16, decided 2026-09-22). NOT the same as di_dong_ca_nhan, which is the personal mobile and '
    'is governed by Decree 13/2023/NĐ-CP.';

-- ---------------------------------------------------------------------------
-- 3. THE CHECK CONSTRAINT 0003 §4 LEFT OUT ON PURPOSE. Question #9 has now closed it.
--
-- 0003 §4 wrote out this exact line and did not run it, and its reasoning was right at the time:
-- "create the account, e-mail an activation link, the person sets their own password" needs
-- precisely the state this constraint forbids — `co_tai_khoan = true` holding an empty hash, for
-- as long as the link is outstanding — and writing the constraint while that answer was still
-- available would have decided the customer's question by making one answer unimplementable.
--
-- ON 2026-09-22 THE CUSTOMER CHOSE (b), and the activation-link flow is out. #17, decided the
-- same day, removes the other route to that state by ruling out self-service password reset.
-- There is now no flow in this system in which an account legitimately exists without a hash:
-- the account is minted WITH a temporary password, and `phai_doi_mat_khau` (§1) carries the
-- "this is not yet the person's own password" state that the empty hash used to stand for.
--
-- SO THE INVARIANT MOVES FROM A COMMENT INTO THE DATABASE, which is the only place it holds
-- against every writer — including a psql prompt and a future service that has read none of this.
-- 0003 §4 also noted the asymmetry that made waiting cheap: adding it later is one line in a new
-- migration. This is that line.
--
-- WHAT STILL FAILS CLOSED WITHOUT IT, and is not replaced by it: core/password.KiemTra cannot
-- parse '' as an argon2id encoding and returns an error, so an empty hash could never verify.
-- That is a property of the password package. This constraint is about the row never existing,
-- not about it failing to sign in.
--
-- THE CONSTRAINT IS UNCONDITIONAL — it is NOT written `deleted_at IS NOT NULL OR ...`. A
-- soft-deleted row must satisfy it too, and the consequence is worth stating because it lands on
-- a future write path: withdrawing an account, and anonymising a person under Decree 13 (rule 7,
-- invariant 7), must set `co_tai_khoan = false` in the same statement that clears the hash. That
-- is the honest state anyway — a person with no credential has no account — and making the
-- database insist on it is how the two stay in step.
--
-- THREE TEST FIXTURES IN THIS REPOSITORY WERE BUILDING THE FORBIDDEN STATE and have been
-- corrected in the same turn: internal/store/vai_tro_pg_test.go themNguoi, quyen_pg_test.go
-- themNguoiDaXoa and themNguoiKhoa all inserted `co_tai_khoan = true` with no hash at all. They
-- are not victims of the constraint; they were seeding an account that could never sign in and
-- calling it an account, which is the very confusion 0003 existed to end.
--
-- ON A PARTITIONED TABLE: ADD CONSTRAINT ... CHECK on the parent recurses into all 32 partitions
-- and validates each. With zero rows that is instant.
--
-- Guarded by pg_constraint rather than written with IF NOT EXISTS, which ADD CONSTRAINT does not
-- offer: the integration suites apply this file into a fresh schema every run, but a hand re-run
-- against a schema that already has it must not fail.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    vi_pham int;
    r       record;
BEGIN
    -- PER-COMMUNE REPORT BEFORE THE ATTEMPT. `ALTER TABLE` on a bad row gives one SQLSTATE 23514
    -- naming a partition — `nguoi_dung_p16` — which tells an operator nothing about WHICH commune
    -- to look at. Counting first means the failure names the communes. Counts and tenant ids
    -- only: never a name, an e-mail or a number (rule 3).
    SELECT count(*) INTO vi_pham
      FROM nguoi_dung
     WHERE co_tai_khoan AND mat_khau_hash = '';

    IF vi_pham > 0 THEN
        FOR r IN
            SELECT tenant_id, count(*) AS n
              FROM nguoi_dung
             WHERE co_tai_khoan AND mat_khau_hash = ''
             GROUP BY tenant_id
             ORDER BY tenant_id
        LOOP
            RAISE NOTICE '0009: commune % has % account row(s) with no password hash',
                r.tenant_id, r.n;
        END LOOP;

        RAISE EXCEPTION
            '% row(s) claim co_tai_khoan with an empty mat_khau_hash', vi_pham
            USING HINT = 'Open question #9 was decided on 2026-09-22: an account is minted WITH '
                         'a temporary password. Each row above is either an account whose '
                         'password was never set — give it one and set phai_doi_mat_khau — or a '
                         'directory-only person wrongly marked as an account, in which case '
                         'co_tai_khoan is false. Deciding which is a person''s call, not this '
                         'migration''s, so it refuses rather than guessing.';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'nguoi_dung_co_tai_khoan_co_mat_khau'
           AND conrelid = 'nguoi_dung'::regclass
    ) THEN
        ALTER TABLE nguoi_dung
            ADD CONSTRAINT nguoi_dung_co_tai_khoan_co_mat_khau
            CHECK (NOT co_tai_khoan OR mat_khau_hash <> '');
        RAISE NOTICE '0009: constraint nguoi_dung_co_tai_khoan_co_mat_khau added';
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 4. nguoi_dung.ma — question #15. The schema was already right; the RULE is what was missing.
--
-- 0001 declared `ma TEXT NOT NULL` with `UNIQUE (tenant_id, ma)` (0001_init.sql:167,184) at a
-- time when nobody had said who mints the value. #15 settles it: THE SYSTEM MINTS IT. Neither
-- form in the specification draws a Mã box (docs/ui-ux/12-danh-ba-can-bo.md §5,
-- 14-cau-hinh.md:79), and now that is a decision rather than an omission.
--
-- NO DDL IS NEEDED FOR THAT — and the absence of DDL is the point worth writing down, because the
-- temptation is to add a DEFAULT here. It is refused: a database default would mint codes for rows
-- nobody audited, in a format the Go side does not know, and it would silently paper over a write
-- path that forgot to ask for one. The generator is domain.SinhMaCanBo
-- (internal/domain/ma_can_bo.go) and the write path calls it explicitly.
--
-- HOW "A CODE, ONCE ISSUED, IS NEVER ISSUED AGAIN" IS ACTUALLY GUARANTEED (rule 7, invariant 3).
-- Two mechanisms, and the second is the one that does the work:
--
--   1. THE CODE IS NOT DERIVED FROM THE TABLE. `SinhMaCanBo` reads 30 bits from crypto/rand; it
--      never counts rows, never reads MAX(ma), never increments anything. THIS is the property
--      that matters for soft delete: any counter — `COUNT(*) + 1`, `MAX(ma) + 1`, a sequence
--      reset on restore — hands the departed person's code to the next arrival the moment a row
--      leaves the count, and a soft-deleted row is precisely a row that leaves most counts.
--   2. `UNIQUE (tenant_id, ma)` IS NOT PARTIAL, AND MUST NEVER BECOME PARTIAL. Unlike the two
--      indexes on this table, it carries no `WHERE deleted_at IS NULL`. A soft-deleted person
--      therefore keeps their code occupied forever, and an INSERT proposing it is refused by the
--      server. Adding `WHERE deleted_at IS NULL` to it "to tidy up" would break rule 7 invariant 3
--      in one line, with no symptom until the day a re-used code appears against two different
--      people in the audit trail and neither can be told from the other.
--
-- Pinned by test: internal/store/ma_can_bo_pg_test.go asserts the refusal on a soft-deleted code,
-- and asserts the SAME code IS accepted in a DIFFERENT commune (rule 1, invariant 6 — the key is
-- composite, so one commune's codes never constrain another's).
--
-- The code is also NOT a ULID, deliberately. #15's reasoning: `ma` is the subject of the audit
-- trail, and somebody reading that trail a year later has to be able to look the person up from
-- it. A 26-character ULID satisfies both database constraints and fails that requirement.
-- ---------------------------------------------------------------------------
COMMENT ON COLUMN nguoi_dung.ma IS
    'Staff code. SYSTEM-GENERATED, no form field (open question #15, decided 2026-09-22); the '
    'generator is domain.SinhMaCanBo. NEVER REISSUED, including after soft delete (rule 7, '
    'invariant 3) — guaranteed by UNIQUE (tenant_id, ma), which is deliberately NOT partial, '
    'plus a generator that is random rather than derived from the table. See migration 0009 §4.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on archival records is an
-- administrative act carried out with a person present, not something a process decides at 3am.
-- So the reverse is written out here, to be run by hand.
--
-- THE CONSTRAINT half is fully reversible and loses nothing — it holds no data:
--
--      ALTER TABLE nguoi_dung DROP CONSTRAINT IF EXISTS nguoi_dung_co_tai_khoan_co_mat_khau;
--
--   Reversing it is, however, RE-OPENING A DECIDED QUESTION rather than a technical undo. Anybody
--   running that line is saying an account may exist with no password, which is answer (c) to
--   question #9. That needs the customer, not a deploy.
--
-- THE COMMENTS are reversible by restating the previous text; they hold no data either.
--
-- THE RENAME IS THE ONE STATEMENT HERE THAT IS NOT ADDITIVE, so it is named first rather than
-- left inside a sentence about columns being added. It loses no data — a rename moves no bytes —
-- and it reverses exactly, with the same guard shape:
--
--      ALTER TABLE nguoi_dung RENAME COLUMN dien_thoai_co_quan TO dien_thoai;
--
-- That line IS runnable and IS safe, which is why it appears in full while the two below do not.
-- What it does NOT undo is the Go side: code reading `DienThoaiCoQuan` would then name a column
-- that no longer exists, and the failure is a query error at runtime, not a compile error. Revert
-- both halves or neither.
--
-- THE TWO ADDED COLUMNS ARE ADDITIVE: nothing existing was overwritten, so "reverting" them is
-- achieved by ceasing to read them, at no cost. Actually DROPPING one once it holds data is a
-- DIFFERENT act — and `di_dong_ca_nhan` will hold personal data (Decree 13), so dropping it
-- destroys the only record of a contact a citizen may have been given. That is rule 7, stop
-- condition #2: it needs an explicit decision by the user plus a verified backup. Neither is
-- written here as a runnable line, because a runnable line is a line that gets run:
--
--      -- ALTER TABLE nguoi_dung DROP COLUMN di_dong_ca_nhan;    -- USER DECISION + BACKUP ONLY
--      -- ALTER TABLE nguoi_dung DROP COLUMN phai_doi_mat_khau;  -- USER DECISION + BACKUP ONLY
--
-- AND AFTERWARDS, for any of the above to be applied again, this file's progress row has to be
-- removed from `schema_migration`, keyed on ten = '0009_tai_khoan_tam_va_danh_ba_can_bo.sql'.
-- Written as prose and not as a runnable line, for the same reason as the two lines above.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0008.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check only describes the state after
-- the NEWEST migration that carries it, so every new file ends with it. This file adds no table,
-- so it is expected to find nothing; it runs anyway, because the run after which it is missing is
-- the one that needed it.
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
