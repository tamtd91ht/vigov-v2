-- 0011 — the two TERMINAL BRANCHES of a petition carry a reason the citizen reads, and the
-- referral carries the body that received it (user decisions 24-25/09/2026).
--
-- `khong-tiep-nhan` and `chuyen-cap-tren` leave from `dang-phan-loai` and from nowhere else, and
-- have no way out (internal/domain/phieu_phan_anh.go:68-78). The decisions this file stores:
--
--   khong-tiep-nhan   REQUIRES a reason the citizen can read
--   chuyen-cap-tren   REQUIRES a reason the citizen can read AND the receiving body — free text,
--                     because since 7/2025 a transfer goes SIDEWAYS as often as up (điện lực, công
--                     an, sở …), so no fixed catalogue of "the level above" exists to point at
--
-- The routes that write them (`POST …/rejection`, `POST …/referral`) are the next card. This file
-- is the schema half, and it makes the rule impossible to break from the database side before any
-- route exists.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004/0005: core/migrate compares the checksum of every applied
-- file at startup. Editing an applied file either stops the service (ErrChecksumLech) or leaves two
-- databases claiming one schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: this file WRITES no row. It adds three NULLABLE columns (no
--      rewrite, no default to fill) and CHECKs that PostgreSQL validates against every existing
--      row. Those rows are, today, zero in every environment (0005's header: no commune onboarded).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is ADD COLUMN IF NOT EXISTS / guarded
--      ADD CONSTRAINT / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none, and the reason is a fact
--      about the code, not a hope. NO WRITE PATH IN THIS SERVICE PUTS A PETITION IN EITHER BRANCH
--      STATUS. The five statements that set `trang_thai` are: intake (`da-tiep-nhan`,
--      internal/app/gui_phan_anh.go:336), classification (`dang-phan-loai`,
--      internal/app/xu_ly_phan_anh.go:455), assignment (domain.SauKhiPhanCong — `da-chuyen-xu-ly`
--      or unchanged), progress (domain.TienTrinhChinh — `dang-xu-ly`, `da-xu-ly`,
--      `cho-dan-xac-nhan`) and closing (`da-dong`). So every existing row sits in a status whose
--      branch columns must be NULL — which a column added seconds ago is.
--      AND IF THAT REASONING IS EVER WRONG, THE FILE FAILS INSTEAD OF LYING: ADD CONSTRAINT
--      validates every row, so a branch-status row with no reason stops this migration, rolls the
--      whole file back, and the service does not start. That is the fail-closed outcome — a
--      petition refused with no readable reason is exactly what the constraint exists to forbid,
--      and it needs a person, not a default string.
--      NO read path reads the new columns yet; the store's column list is unchanged by this card.
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). The reason and the receiving body are
--      what the citizen was told about why the commune did not handle their report — so once
--      written they are frozen by the trigger below (rule 7, forbidden #5).
--
-- ---------------------------------------------------------------------------
-- WHY ONE SHARED REASON COLUMN AND NOT `ly_do_khong_tiep_nhan` + `ly_do_chuyen_cap_tren`.
--
-- It is ONE fact: "why this petition left the commune's own processing, in words the citizen
-- reads". A row can only ever hold ONE of the two branches — both are terminal and both leave from
-- the same status — so two columns would always have one of them NULL, and every reader would have
-- to know which to pick by looking at `trang_thai` anyway. `trang_thai` already says WHICH branch;
-- this column says WHY. The CHECK below binds the two, so the reason cannot exist without a branch
-- status nor a branch status without a reason.
--
-- The receiving body is a DIFFERENT fact, present on one branch only, so it is its own column.
--
-- NAMING: `ket_thuc_nhanh` ("kết thúc nhánh" — the branch ended) and NOT `re_nhanh`: 0003 already
-- uses `re_nhanh` in `ma_nguon_re_nhanh` for "the SOURCE CODE branches on this row", an unrelated
-- meaning. One word, two meanings, one schema is how a reader joins the wrong things.
--
-- ---------------------------------------------------------------------------
-- WHY A NEW INSTANT COLUMN AND NOT `dong_luc` (0004), which already records "when it ended".
--
-- `dong_luc` is the instant of CLOSING (`da-dong`), written with `ket_qua_xu_ly` — the result of
-- work DONE (internal/store/xu_ly_phan_anh.go:371). Writing a refusal or a referral there makes a
-- rejected petition indistinguishable from a resolved one to anything counting `dong_luc`: and a
-- branch row HAS `han_xu_ly_xong` (it is fixed on entering `dang-phan-loai`, the status both
-- branches leave from), so an on-time report written as `dong_luc <= han_xu_ly_xong` would count
-- every quick refusal as a petition resolved on time. Beautiful, wrong, and unreadable from the
-- figure itself — the same class of error 0004's header warns about for the two NULLs.
-- The two instants are two facts; a row can never hold both (the branches never reach `da-dong`),
-- so nothing is duplicated.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT DO:
--
--   * No index. No screen or report queries by these columns yet; the register's existing indexes
--     already filter on `trang_thai` where needed. An index nobody reads is write cost for nothing.
--   * No catalogue of receiving bodies. Free text is the user's decision (see top).
--   * No minimum length beyond non-blank. `ket_qua_xu_ly`'s minimum (domain.KetQuaToiThieu) is a
--     Go-side, session-chosen number; the next card's route validates the reason the same way. The
--     database refuses only what is never acceptable: absent, blank, or oversized.
--   * No change to `ho_so_luu_tru_bat_bien` (0004). Replacing that function here would make the
--     reversal of THIS file depend on restoring 0004's exact body; a separate function and trigger
--     reverse by dropping two objects this file created.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table are only allowed from PostgreSQL 13. On
-- 11 and 12 the CREATE TRIGGER below fails with a message that reads like a syntax mistake and
-- invites moving the trigger onto the partitions — where a partition added later arrives silently
-- unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'phieu_phan_anh branch columns need PostgreSQL 13 or newer (server is %). Do not '
            'weaken this migration to fit an older server — the trigger below is what keeps a '
            'reason already shown to a citizen from being rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- THE THREE COLUMNS. All NULLABLE: NULL on every petition that did not take a branch, which is
-- every petition that exists today (question 4).
-- ---------------------------------------------------------------------------

-- The reason, WRITTEN FOR THE CITIZEN. Same standing as `ket_qua_xu_ly` (0005 §2): it is NOT a
-- staff note and must never become one — internal notes stay internal (rule 4, forbidden #5) — and
-- it NEVER travels on the notification event (proto/vigov/petitions/v1/events.proto: free text
-- about a case will eventually name the reporter; a queue is persisted and replicated).
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS ly_do_ket_thuc_nhanh TEXT;

-- The body that received a referred petition ("Điện lực …", "Công an …", "Sở …"). Free text by the
-- user's decision. Only on `chuyen-cap-tren`.
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS co_quan_nhan TEXT;

-- When the branch act happened — the instant the commune stopped owning the matter. Written from
-- the use case's clock seam, like `phan_loai_luc` and `dong_luc`.
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS ket_thuc_nhanh_luc TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS.
--
-- DO $$ … $$ RATHER THAN A BARE ALTER: `ADD CONSTRAINT` has no IF NOT EXISTS, and a retry of this
-- file must cost nothing (migration question 2). The pg_constraint lookup is by name, the same
-- pattern 0005 uses.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    -- THE BINDING BETWEEN STATUS AND COLUMNS, in both directions:
    --
    --   khong-tiep-nhan   reason present and non-blank · no receiving body · instant present
    --   chuyen-cap-tren   reason present and non-blank · receiving body present and non-blank ·
    --                     instant present
    --   any other status  all three NULL
    --
    -- The ELSE arm is what stops drift: without it a reason could be written onto a petition that
    -- is still being processed, and a citizen screen showing "why we refused" would show it on a
    -- petition nobody refused.
    --
    -- Every arm is written so it can only evaluate to TRUE or FALSE, never NULL — a CHECK treats
    -- NULL as PASSED, and `btrim(NULL) <> ''` alone is NULL. So `IS NOT NULL AND …` is not
    -- redundant; it is what makes an absent reason a refusal.
    --
    -- The status list of 0004 (`phieu_phan_anh_trang_thai_hop_le`) is closed, so the ELSE arm
    -- covers exactly the seven non-branch codes. migrations/ket_thuc_nhanh_phan_anh_test.go
    -- checks that both codes named here are in that list.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_ket_thuc_nhanh_du_truong'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_ket_thuc_nhanh_du_truong CHECK (
            CASE trang_thai
                WHEN 'khong-tiep-nhan' THEN
                    ly_do_ket_thuc_nhanh IS NOT NULL AND btrim(ly_do_ket_thuc_nhanh) <> ''
                    AND co_quan_nhan IS NULL
                    AND ket_thuc_nhanh_luc IS NOT NULL
                WHEN 'chuyen-cap-tren' THEN
                    ly_do_ket_thuc_nhanh IS NOT NULL AND btrim(ly_do_ket_thuc_nhanh) <> ''
                    AND co_quan_nhan IS NOT NULL AND btrim(co_quan_nhan) <> ''
                    AND ket_thuc_nhanh_luc IS NOT NULL
                ELSE
                    ly_do_ket_thuc_nhanh IS NULL
                    AND co_quan_nhan IS NULL
                    AND ket_thuc_nhanh_luc IS NULL
            END);
    END IF;

    -- BOUNDED, IN CHARACTERS: char_length counts characters, not bytes, so Vietnamese diacritics
    -- (2-3 bytes each in UTF-8) do not shorten the allowance. A NULL passes — the binding above
    -- decides when NULL is allowed.
    --
    -- 2000 = domain.KetQuaToiDa, the bound on the OTHER citizen-readable text on this row. Same
    -- kind of text, same reader, same bound: "long enough for a paragraph a citizen reads, short
    -- enough that no decision carries a document". The next card's route must validate the same
    -- number with utf8.RuneCountInString so a client sees a 400, not a 500.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_ly_do_ket_thuc_nhanh_toi_da'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_ly_do_ket_thuc_nhanh_toi_da
            CHECK (char_length(ly_do_ket_thuc_nhanh) <= 2000);
    END IF;

    -- ⚠ 200 IS THIS SESSION'S NUMBER, NOT THE CUSTOMER'S. It is the name of an administrative body
    -- or enterprise branch ("Công ty Điện lực … — Điện lực huyện …"), not a paragraph; 200
    -- characters is well above any such name and well below text that has stopped being a name.
    -- Raising it later is one migration; lowering it after communes have written names is not.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_co_quan_nhan_toi_da'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_co_quan_nhan_toi_da
            CHECK (char_length(co_quan_nhan) <= 200);
    END IF;

    -- The branch act happens AT OR AFTER classification began: both branches leave from
    -- `dang-phan-loai`, whose entry writes `phan_loai_luc`. This catches the one mistake that looks
    -- right in a diff — copying `goc_dem_han` or `vao_so_luc` into the instant — the same statement
    -- `phieu_phan_anh_han_phan_loai_sau_goc` (0005) makes about its own column.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_ket_thuc_nhanh_sau_phan_loai'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_ket_thuc_nhanh_sau_phan_loai
            CHECK (ket_thuc_nhanh_luc IS NULL OR phan_loai_luc IS NULL
                   OR ket_thuc_nhanh_luc >= phan_loai_luc);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- phieu_phan_anh_ket_thuc_nhanh_bat_bien — once written, the three branch columns never change.
--
-- WHY: the reason and the receiving body are what the citizen was told about why the commune did
-- not handle their report, and where it went. Rewriting them afterwards changes what an authority
-- said, on an archival record, with nothing on the row showing it happened (rule 7, forbidden #5).
-- The instant is the fact an inspection asks first ("when did you refuse it?").
--
-- TOGETHER WITH THE CHECK ABOVE THIS ALSO MAKES THE BRANCHES TERMINAL IN THE DATABASE, not only
-- in domain.chuyenDuocSang: leaving a branch status requires the three columns to be NULL (the
-- ELSE arm), and this trigger refuses nulling them.
--
-- WHAT IT ALLOWS: NULL -> value (the branch act itself, which writes status and columns in ONE
-- statement), and any UPDATE that leaves them alone — soft delete (`deleted_at` …) included.
--
-- ⚠ WHAT IT ALSO REFUSES, said plainly: ANONYMISING these columns under a Decree 13/2023 erasure
-- request (rule 7, invariant 7). The text is written by staff FOR the citizen and should carry no
-- personal data, but free text eventually does. No erasure path exists in this repository yet;
-- the day one is built, how it treats these two columns is a decision for the user, and it must
-- be made there — not by loosening this trigger in passing.
--
-- A SEPARATE FUNCTION AND A SECOND TRIGGER rather than a CREATE OR REPLACE of
-- `ho_so_luu_tru_bat_bien` (0004): see "WHAT THIS FILE DELIBERATELY DOES NOT DO" above. BEFORE
-- triggers fire in name order; `phieu_phan_anh_ket_thuc_nhanh_bat_bien` runs after
-- `phieu_phan_anh_luu_tru`, and each refuses independently, so the order changes no outcome.
--
-- Messages name the operation and the relation only; an error message travels into logs and back
-- to clients (rule 3, forbidden #3), and this table holds citizen personal data.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION phieu_phan_anh_ket_thuc_nhanh_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.ly_do_ket_thuc_nhanh IS NOT NULL
       AND NEW.ly_do_ket_thuc_nhanh IS DISTINCT FROM OLD.ly_do_ket_thuc_nhanh THEN
        RAISE EXCEPTION 'archival record %: `ly_do_ket_thuc_nhanh` is immutable once written',
            TG_TABLE_NAME
            USING HINT = 'This is the reason the citizen was given for the refusal or referral. '
                         'Rewriting it changes what the authority said, on an archival record '
                         '(rule 7, forbidden #5).';
    END IF;

    IF OLD.co_quan_nhan IS NOT NULL
       AND NEW.co_quan_nhan IS DISTINCT FROM OLD.co_quan_nhan THEN
        RAISE EXCEPTION 'archival record %: `co_quan_nhan` is immutable once written',
            TG_TABLE_NAME
            USING HINT = 'This is the body the citizen was told their petition went to. A '
                         'correction is a new administrative act, not an edit of this one.';
    END IF;

    IF OLD.ket_thuc_nhanh_luc IS NOT NULL
       AND NEW.ket_thuc_nhanh_luc IS DISTINCT FROM OLD.ket_thuc_nhanh_luc THEN
        RAISE EXCEPTION 'archival record %: `ket_thuc_nhanh_luc` is immutable once written',
            TG_TABLE_NAME
            USING HINT = 'The instant of the refusal or referral is recorded once, at the act.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS phieu_phan_anh_ket_thuc_nhanh_bat_bien ON phieu_phan_anh;
CREATE TRIGGER phieu_phan_anh_ket_thuc_nhanh_bat_bien
    BEFORE UPDATE ON phieu_phan_anh
    FOR EACH ROW EXECUTE FUNCTION phieu_phan_anh_ket_thuc_nhanh_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, and DDL by the table owner (ALTER TABLE
-- ... DISABLE TRIGGER, dropping the constraint). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). WHILE NO PETITION HAS TAKEN A BRANCH — true in every
-- environment today, since no route writes either status — the reversal is complete and loses
-- nothing. In ONE transaction:
--
--   DROP TRIGGER phieu_phan_anh_ket_thuc_nhanh_bat_bien ON phieu_phan_anh;
--   DROP FUNCTION phieu_phan_anh_ket_thuc_nhanh_bat_bien();
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_ket_thuc_nhanh_sau_phan_loai;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_co_quan_nhan_toi_da;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_ly_do_ket_thuc_nhanh_toi_da;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_ket_thuc_nhanh_du_truong;
--   ALTER TABLE phieu_phan_anh DROP COLUMN ket_thuc_nhanh_luc;
--   ALTER TABLE phieu_phan_anh DROP COLUMN co_quan_nhan;
--   ALTER TABLE phieu_phan_anh DROP COLUMN ly_do_ket_thuc_nhanh;
--
-- and remove this file's row from `schema_migration` in the same transaction, otherwise the runner
-- still believes the schema is in place. Nothing of 0004/0005 is altered by this file, so nothing
-- there has to be put back.
--
-- BEFORE DROPPING THE COLUMNS, CHECK THEY ARE EMPTY: a count of rows with `trang_thai IN
-- ('khong-tiep-nhan', 'chuyen-cap-tren')` must be zero. ONCE ONE PETITION HAS BEEN REFUSED OR
-- REFERRED, the reason is what a citizen was told and dropping the column is destroying part of an
-- archival record — rule 7's second stop condition, which needs the user and a verified backup, not
-- a command. From that point the way back is a NEW migration (core/migrate has no automatic
-- rollback — ADR 0013). Dropping only the trigger and the constraints stays lossless at any time.
-- ---------------------------------------------------------------------------
