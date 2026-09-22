-- finance — the UNLOCK ledger of a disbursement voucher, and the commune's own slow-project
-- threshold (docs/ui-ux/06-giai-ngan.md §13 rule 3 and rule 5).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: 0001..0004 have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DECIDES, AND WHO DECIDED IT. READ THIS BEFORE THE SQL.
--
-- Open question #29 asks five things about unlocking a locked disbursement voucher. THREE of them
-- are settled HERE, on 2026-09-22, BY THIS PROJECT AND NOT BY THE CUSTOMER, in the direction that
-- can be loosened later with one line and cannot be tightened later at all:
--
--   1. AN UNLOCK REASON IS MANDATORY, in a column of its own.
--      Why this direction: the trail of unlocks that have ALREADY HAPPENED cannot be rebuilt from
--      any source. Adding the column later is a cheap migration; the span before it exists is a
--      span in which every unlock is unexplainable — on exactly the figure an inspection asks
--      about, because it is a figure somebody signed and somebody then changed.
--
--   2. THE PERSON WHO LOCKED MAY NOT UNLOCK IT THEMSELVES. Somebody else holding `budget.confirm`
--      must do it.
--      Why this direction: the same shape the customer already settled for the staff register in
--      #13 and #14 — nobody acts alone on the act that gives themselves room. Loosening later is
--      one line; tightening later leaves vouchers carrying mixed states that no column can tell
--      apart afterwards.
--
--   3. THERE IS NO CEILING ON THE NUMBER OF UNLOCKS, BUT EVERY ONE IS COUNTED.
--      Why this direction: a ceiling is a number that belongs to the customer. A count cannot be
--      wrong, and it is what lets the customer choose a ceiling later from REAL figures instead of
--      from somebody's guess.
--
-- THE OTHER TWO HALVES OF #29 ARE NOT DECIDED HERE AND NOTHING BELOW PRETENDS OTHERWISE:
--
--   `nam_ngan_sach.khoa`  — the table is deliberately NOT created (see the second half of this
--                           file). What a locked budget year locks is unanswered, and it is tied
--                           to #31, #32 and #33.
--   the `🗑 Gỡ` permission — the specification assigns none (`06-giai-ngan.md:202` covers only
--                           `budget.update` for entry/edit and `budget.confirm` for confirm/lock).
--                           The route chooses `budget.confirm`, and WHY is written at the route,
--                           not here: it is an application decision, not a schema one.
--
-- OPEN QUESTION #30 IS UNTOUCHED AND MUST STAY THAT WAY. `CHECK (so_tien > 0)` on
-- chung_tu_giai_ngan is NOT relaxed, NOT dropped, and NOT worked around. A negative voucher is a
-- REFUND — a different business event with a different name (0004:300-304) — and the tempting
-- detour is not an ALTER at all: it is letting somebody edit an old voucher down to a smaller
-- figure with nothing recording that a refund happened. The unlock ledger below is precisely what
-- makes that detour visible if anybody takes it.
--
-- ---------------------------------------------------------------------------
-- EVERY `nguoi_*_id` ON chung_tu_giai_ngan HOLDS THE STAFF BUSINESS CODE (`CB-2026-7K3M9Q`),
-- NOT THE INTERNAL ULID. The suffix says `_id` because 0004 named the first two that way; the
-- VALUE is the same one `deleted_by` holds on every table in this service and the same one
-- `audit_log.actor_id` holds (rule 6, invariant 8).
--
-- Written out here rather than left to be inferred, because a column holding two kinds of
-- identifier is a column nobody can query — which is what this repository measured on 2026-09-22
-- across six write paths. One convention, stated once, and the two columns added below follow it.
--
-- RENAMING THE FOUR COLUMNS TO `ma_can_bo_*` WAS CONSIDERED AND NOT DONE: it would be a rename in
-- a migration whose subject is the unlock ledger, and a rename that rides along with an unrelated
-- change is a rename nobody reviews. It is a finding, not a silent fix.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero, on both tables. chung_tu_giai_ngan is empty everywhere
--      (0004 seeded nothing and no write route existed until today), and cau_hinh_giai_ngan is new.
--      The statements below are nevertheless written as though every commune held rows — the five
--      columns are added with defaults that make existing rows satisfy every new constraint, so
--      the day this runs against real data it behaves the same way.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or guarded by a DO block
--      that checks for its own object, so a retry after a failure costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Every column added is
--      nullable or defaulted, and nothing existing is altered or dropped. `da_giai_ngan` — the one
--      figure every screen totals — is SUM(so_tien) over live vouchers and is not touched by any
--      statement here.
--   5. WHAT IT COSTS ON THE LARGEST COMMUNE: nothing measurable. ADD COLUMN with a constant
--      DEFAULT does not rewrite a table on PostgreSQL 11 and newer, and the table is empty in any
--      case. The new table is 32 empty partitions.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor, for the same reason 0004 states: BEFORE ... FOR EACH ROW triggers on
-- a PARTITIONED table were only allowed from 13, and the new table below declares one.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the disbursement register needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- PART 1 — the unlock ledger on chung_tu_giai_ngan.
--
-- FIVE COLUMNS, AND NOT ONE OF THEM IS IN THE `chung_tu_da_khoa` TRIGGER'S REFUSAL LIST. That
-- looks, at first reading, exactly like the weakness 0004:137-139 warns about ("A COLUMN ADDED
-- LATER AND NOT ADDED HERE IS AN EDITABLE LOCKED FIELD"). It is the opposite, and the difference
-- is the whole design:
--
--	the trigger refuses changes to the FIGURES AND FACTS of the voucher while it is locked
--	these five columns are the RECORD OF THE UNLOCK ITSELF
--
-- The unlock statement runs while OLD.trang_thai IS STILL 'da-khoa' — that is what unlocking
-- means — so these columns MUST be writable in exactly that moment, or the reason and the count
-- could only be written in a second statement, after the state had already moved. Two statements
-- is two events, and the second one can fail: a voucher unlocked with no reason recorded is the
-- precise state decision (1) above exists to prevent.
--
-- WHAT IS THEREFORE NOT GUARDED BY THE DATABASE, said plainly: a bare UPDATE could rewrite
-- `ly_do_mo_khoa` on a locked row without touching anything else, and the trigger would allow it.
-- What stops it in this service is that no statement in internal/store mentions these columns
-- outside the single unlock UPDATE, and that the append-only `audit_log` holds every unlock reason
-- independently — so a rewritten column disagrees with a ledger that cannot be edited (0002).
-- Closing it in the database means replacing the trigger function, and this file deliberately does
-- NOT: the function is the floor 0004 built, and a floor rewritten as a side effect of adding
-- columns is a floor nobody reviewed.
-- ---------------------------------------------------------------------------

ALTER TABLE chung_tu_giai_ngan
    -- Who locked it. NOT the same person as `nguoi_xac_nhan_id` in general: confirming and
    -- locking are two acts, both under `budget.confirm`, and a commune with two people holding
    -- that key will have rows where they differ. Decision (2) compares against THIS column, so
    -- reusing `nguoi_xac_nhan_id` for it would make "the person who locked it" unanswerable on
    -- exactly the rows where the question has a point.
    ADD COLUMN IF NOT EXISTS nguoi_khoa_id     TEXT,

    -- The LAST unlock: when, by whom, and why.
    --
    -- THE COLUMN HOLDS THE CURRENT STATE, `audit_log` HOLDS THE HISTORY, and that is not the
    -- duplication rule 9 forbids — it is the same split `delete_reason` already has on every
    -- table in this service. The column answers "why is this voucher open again right now", which
    -- is what a screen shows; the ledger answers "how did this voucher get to where it is", which
    -- is what an inspection reads. Neither is derivable from the other: the column is overwritten
    -- by the next unlock, and the ledger is never shown beside the row.
    ADD COLUMN IF NOT EXISTS thoi_diem_mo_khoa TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nguoi_mo_khoa_id  TEXT,
    ADD COLUMN IF NOT EXISTS ly_do_mo_khoa     TEXT,

    -- How many times this voucher has been unlocked. NO CEILING — decision (3).
    --
    -- DEFAULT 0 AND NOT NULL: a NULL here would mean "unknown", and "unknown" is not a state this
    -- counter can be in. A voucher that has never been unlocked has been unlocked zero times, and
    -- that is a fact, not an absence.
    ADD COLUMN IF NOT EXISTS so_lan_mo_khoa    INT NOT NULL DEFAULT 0;

-- The constraints. ADD CONSTRAINT has no IF NOT EXISTS, so each one is guarded by its own
-- existence check — that is what makes a retry after a half-failed run cost nothing (question 2).
DO $$ BEGIN
    -- A locked voucher must name who locked it. Same shape as
    -- `chung_tu_giai_ngan_khoa_co_thoi_diem` (0004:308): `Đã khoá` with nobody attached is a row
    -- nobody can account for when an inspection asks who froze the figure — and it is also a row
    -- decision (2) cannot be enforced on, because there is nobody to compare the unlocker against.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chung_tu_giai_ngan_khoa_co_nguoi') THEN
        ALTER TABLE chung_tu_giai_ngan ADD CONSTRAINT chung_tu_giai_ngan_khoa_co_nguoi
            CHECK (trang_thai <> 'da-khoa' OR nguoi_khoa_id IS NOT NULL);
    END IF;

    -- An unlock that happened carries all three of its facts, or it did not happen. The three are
    -- checked TOGETHER rather than as three constraints because they are one event: a row with a
    -- reason and no timestamp is not "partly recorded", it is a row somebody wrote by hand.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chung_tu_giai_ngan_mo_khoa_du_vet') THEN
        ALTER TABLE chung_tu_giai_ngan ADD CONSTRAINT chung_tu_giai_ngan_mo_khoa_du_vet
            CHECK (so_lan_mo_khoa = 0
                   OR (thoi_diem_mo_khoa IS NOT NULL
                       AND nguoi_mo_khoa_id IS NOT NULL
                       AND btrim(ly_do_mo_khoa) <> ''));
    END IF;

    -- A negative count is a sign error or a bad UPDATE, never a state.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chung_tu_giai_ngan_so_lan_mo_khong_am') THEN
        ALTER TABLE chung_tu_giai_ngan ADD CONSTRAINT chung_tu_giai_ngan_so_lan_mo_khong_am
            CHECK (so_lan_mo_khoa >= 0);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- PART 2 — cau_hinh_giai_ngan: the commune's own slow-project threshold (#31).
--
-- @entity: DisbursementSettings
-- @scope:  tenant
--
-- WHAT THIS FIXES. `nguong_canh_bao_cham` decides whether a project is reported as behind (§3),
-- and therefore whether the commune's KPI card reads "29 dự án chậm" or "0". The specification
-- says THREE TIMES that the number belongs to the commune — §11 puts it on `nam_ngan_sach`, §13
-- rule 5 says "cấu hình theo năm ngân sách, mặc định 10 điểm", and §3's KPI card prints it back to
-- the commune as "Ngưỡng cảnh báo chậm: 10 điểm". Until today it was a constant in the vendor's
-- source (internal/domain/du_an.go), which is rule 1, invariant 10 read backwards: a per-commune
-- business rule living in one codebase that serves 200+ communes.
--
-- WHY THIS TABLE AND NOT `nam_ngan_sach`, which is where §11 puts the column. `nam_ngan_sach`
-- also carries `ke_hoach_von_nam` and `khoa`, and BOTH are unanswered: `khoa` is half of open
-- question #29 (one mention in the whole specification, no sentence saying what it locks), and the
-- year's plan figure is entangled with #32 and #33, the two suspended questions about which column
-- feeds a reported number. Creating `nam_ngan_sach` today means creating two columns nobody can
-- fill correctly next to one that can — so this table carries ONLY the value that is decided, and
-- its name says exactly that much.
--
-- THE COST, STATED: when `nam_ngan_sach` is decided, this column moves there and this table goes
-- away. That is a migration on a table that is EMPTY for every commune, because nothing writes it
-- — see the next paragraph. Had the threshold instead been left in source, the same day would
-- arrive with 200+ communes silently sharing one number and no record of who should have chosen
-- what.
--
-- NO WRITE PATH EXISTS YET, AND THAT IS WHY THE READ PATH REPORTS ITS SOURCE. 0004's header made
-- the right objection to this table — "a table nobody fills is a table that reads as 'already
-- configurable' while every commune silently gets the default". The answer is not to hide the
-- table; it is to stop the read path from being able to lie: fistore.CauHinhGiaiNganStore returns
-- the threshold AND whether it came from this table or from the default, and the API sends both
-- out. A commune on the default can be told so, on the screen, by name.
--
-- WHAT IS NOT BUILT, deliberately: the screen that writes a row here (that is the web's work and
-- needs a permission decision), and any storage of "which threshold was in force when a figure was
-- reported". The second one matters and is NOT an oversight — see the note at the bottom of the
-- REVERSAL block.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cau_hinh_giai_ngan (
    tenant_id             TEXT        NOT NULL,

    -- PER BUDGET YEAR, because §13 rule 5 says so in those words. A single row per commune would
    -- make a threshold chosen in 2026 silently re-judge 2025's projects, and §13 rule 8 is
    -- explicit that each budget year is its own set of projects.
    nam                   INT         NOT NULL,

    -- THE UNIT IS PARTS PER TEN THOUSAND, NOT PERCENTAGE POINTS. `10 điểm` — the figure the
    -- commune sees and the default of §13 rule 5 — is stored as 1000.
    --
    -- WHY NOT "just store 10": the comparison this number takes part in is `diem_cham > nguong`,
    -- and `diem_cham` is computed in parts per ten thousand precisely so that a comparison near
    -- the threshold is exact rather than a float comparison nobody can see (domain.PhanVan). Two
    -- units either side of one comparison is how a project ends up flagged on one screen and not
    -- on another. It also lets a commune say "10,5 điểm", which the specification's own display
    -- format (two decimals: "chậm 31,36 điểm") already implies is meaningful.
    --
    -- THE RANGE IS THE MATHEMATICAL RANGE OF THE SCORE, not a business choice: `diem_cham` is at
    -- most 10000 (a whole year elapsed, nothing disbursed). 0 means "flag anything at all behind",
    -- 10000 means "never flag". A value outside that is a typo — 100 meant as "10 points" would
    -- silently flag projects that are 1 point behind, which reads as the system being broken.
    nguong_canh_bao_cham  INT         NOT NULL,

    -- Who last changed it and when. No soft-delete columns: this is CONFIGURATION, not an
    -- archival record — there is nothing here to destroy, and removing a row simply returns the
    -- commune to the default, which is a state the read path already reports honestly.
    cap_nhat_boi          TEXT,
    tao_luc               TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Composite with tenant_id (rule 1, invariant 6). One row per commune per budget year.
    PRIMARY KEY (tenant_id, nam),
    CONSTRAINT cau_hinh_giai_ngan_nguong_hop_le
        CHECK (nguong_canh_bao_cham BETWEEN 0 AND 10000),
    CONSTRAINT cau_hinh_giai_ngan_nam_hop_le
        CHECK (nam BETWEEN 2000 AND 2100)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS cau_hinh_giai_ngan_p%s PARTITION OF cau_hinh_giai_ngan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3).
--
-- PART 2 reverses completely and loses nothing while no commune has written a threshold: drop
-- cau_hinh_giai_ngan (the 32 partitions go with it). Once a commune HAS set one, dropping the
-- table silently returns that commune to 10 points — which is a change to a reported figure, not a
-- schema tidy-up, so it needs the user.
--
-- PART 1 reverses by dropping the three constraints and then the five columns, in that order. It
-- loses nothing ONLY while `so_lan_mo_khoa = 0` on every row. The moment one voucher has been
-- unlocked, dropping `ly_do_mo_khoa` destroys the only durable copy of why a signed figure was
-- reopened that sits BESIDE the row — `audit_log` still holds it, and that is the point of rule 6,
-- but the two answer different questions (see the column comment). That is rule 7's first stop
-- condition and needs the user, not a command.
--
-- ---------------------------------------------------------------------------
-- WHAT IS STILL MISSING AND IS NOT AN OVERSIGHT: nothing anywhere stores WHICH THRESHOLD WAS IN
-- FORCE when a "số dự án chậm" figure was reported upward. #31's own reversal note says why that
-- matters — the same reason rule 10, invariant 2 stores a deadline instead of recomputing it.
--
-- It is not built here because THERE IS NO REPORTING SURFACE YET: no table, no route and no job in
-- this repository persists a period's disbursement figures. The threshold only ever reaches a
-- client alongside the figure it produced, in the same response (`delay_threshold`), so today
-- there is nothing that could disagree with itself. The day a reported period is stored, the
-- threshold used has to be stored with it, in the same row.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check is repeated at the end of every
-- migration that declares a partitioned table, because it only verifies the state after a file
-- that CARRIES it.
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
