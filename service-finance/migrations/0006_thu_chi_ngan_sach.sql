-- finance — the commune's revenue/expenditure budget board: one sheet per budget year and per kind,
-- the columns that sheet carries, the tree of budget lines, and the value in each cell
-- (docs/ui-ux/07-thu-chi-ngan-sach.md §7).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004/0005: 0001..0005 have been applied and core/migrate
-- compares the checksum of every applied file at startup. Editing an applied file either stops the
-- service (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ---------------------------------------------------------------------------
-- MONEY, AGAIN, AND THE SAME ANSWER AS 0004 — read that file's MONEY block; it is not repeated.
--
-- ONE THING IS DIFFERENT AND IT IS THE TRAP OF THIS SCREEN: the sheet's stated unit is
-- **Triệu đồng** (§1) and its sample figures carry one decimal — `3.463.459,2`. Stored as "triệu
-- đồng" that is not an integer, and an INT column would silently drop the `,2`. EVERY VALUE HERE IS
-- STORED IN ĐỒNG, BIGINT, exactly as `chung_tu_giai_ngan.so_tien` is: `3.463.459,2 triệu đồng` is
-- `3463459200000` đồng, exact, with no scaling factor to remember. The unit on the screen is a
-- DISPLAY concern and lives in `bang_ngan_sach.don_vi_tinh`.
--
-- THE CEILING IS NOT IN SIGHT: the specification's own commune plans 3,99 × 10^12 đồng of revenue
-- for a year, six orders of magnitude below BIGINT's 9,22 × 10^18.
--
-- ---------------------------------------------------------------------------
-- A VALUE MAY BE NEGATIVE, AND MAY BE EMPTY, AND THE TWO ARE DIFFERENT (§9 rule 4).
--
-- So there is NO `CHECK (gia_tri > 0)` here — unlike `chung_tu_giai_ngan`, where a negative amount
-- is a refund masquerading as a payment (ADR 0035 §B). A budget line legitimately carries a
-- negative figure (thu chuyển nguồn điều chỉnh giảm), and "chưa có số" is a state the screen draws
-- as `—`, never as `0`.
--
-- EMPTY IS `gia_tri IS NULL` AND NOT "no row", and that is a decision with a cost worth stating:
-- clearing a cell could have been a DELETE of the row, which is smaller — and is a hard delete on
-- business data, which rule 7 forbidden #1 refuses outright and `ho_so_luu_tru_cam_xoa_cung`
-- below refuses in the database. A cleared cell keeps its row, keeps its NULL, and the figure it
-- used to hold is in `audit_log`, which is append-only.
--
-- ---------------------------------------------------------------------------
-- THE HEADLINE ROW IS CHOSEN BY A PERSON. `is_headline` IS THE WHOLE POINT OF THIS FILE.
--
-- §5 rule 5 reads "số tổng lấy từ dòng được đánh sao, MẶC ĐỊNH DÒNG ĐẦU TIÊN". The second half is
-- the trap, and it was measured rather than guessed — `../vigov-require` built it, hit it, and
-- replaced it with a per-row flag (their migration 0035, commit `0142452`; the measurement is
-- recorded at kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md §M3):
--
--	the THU sheet has TWO top-level rows, NESTED     `A. TỔNG THU NỘI ĐỊA…` and `B. THU NGÂN SÁCH
--	                                                 ĐỊA PHƯƠNG` overlap in what they count.
--	the CHI sheet has `Tổng số` SIBLING to A…E       adding the siblings double-counts it.
--
-- So "the first row" is right on one form and wrong on the next, WITH NOTHING REPORTING IT — the
-- shape of defect this repository exists to refuse. The commune marks the row; the software never
-- infers it from ordering, from `tt`, or from depth in the tree.
--
-- NO VALUE IS SEEDED FOR IT HERE, deliberately. A migration that marked a row would be the same
-- guess, made once, in a place nobody can read afterwards. A sheet with no row marked has NO total
-- — and the read path says that in a sentence instead of answering 0 (ADR 0035 §A: "để trống kèm
-- lý do, không đặt mặc định").
--
-- WHY THERE IS NO `UNIQUE … WHERE is_headline` HERE, though that is exactly the constraint this
-- property wants: `khoan_muc_ngan_sach` is PARTITIONED BY HASH, and a PARTIAL unique index on a
-- partitioned table is a form no PostgreSQL is reachable here to verify (VIGOV_TEST_DSN is unset;
-- Docker is off). 0004 set the precedent for exactly this situation with the foreign key it did not
-- declare: the form that needs no verification wins, and the cost is stated rather than discovered.
-- THE COST: nothing in the database stops a second marked row. What replaces it is two halves in
-- the application, both tested — the mark is a RADIO (setting one clears the others in the same
-- transaction), and the read path REFUSES to produce a total when it finds more than one, rather
-- than summing them or picking one.
--
-- ---------------------------------------------------------------------------
-- A PARENT LINE IS NEVER TYPED INTO. Customer decision, 06/09/2026 (anh Hà), carried over from
-- `../vigov-require` commit `502d6f4`. Enforced in internal/app, not only on the screen.
--
-- THE PRICE THE CUSTOMER ACCEPTED, WRITTEN HERE SO NOBODY DISCOVERS IT LATER: real forms have
-- places where the parent is NOT the sum of its children — khoản mục ngoài cân đối, and the
-- `Trong đó:` lines that restate part of the row above. On those rows THE NUMBER ON THE SCREEN WILL
-- DIFFER FROM THE PAPER THE COMMUNE SIGNED. That is a known, accepted consequence of the decision,
-- not a defect to be fixed by quietly re-allowing direct entry on parents.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. Four new tables and this file writes no row into any of
--      them — §10 of the specification is PROTOTYPE data and is deliberately not seeded. The
--      ceiling in sight is the specification's own: 59 lines on the 2026 chi sheet, 52 on thu, and
--      at most a handful of columns per sheet.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so a
--      retry after a failure costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS. No
--      existing table, column, index, constraint, trigger or function is touched.
--   5. WHAT IT COSTS ON THE LARGEST COMMUNE: nothing measurable. Empty tables, 128 empty
--      partitions, five indexes over no rows.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence reads as a decision:
--
--   dot_thu_chi          §5's `⇄ Các đợt thu, chi` dialog, and with it the `entries` value of
--                        `cach_tinh`. The CHECK below admits `manual` and `children` ONLY. Admitting
--                        a third value with no table behind it would give a commune a calculation
--                        mode that silently reports 0 for every line set to it — a wrong figure that
--                        looks like a working feature. Adding it is one line of CHECK plus the
--                        table, on the day that dialog is built.
--   nguon_von            unchanged from 0004's note. `../vigov-require` has since measured the
--                        relationship to be one-to-MANY (a project draws on several funding
--                        sources, `budget_item_sources`), which makes it a deliberate turn of its
--                        own and not a column to bolt on here.
--   the Excel import     §6. `nguon_tep` and `nap_luc` below are the columns it will fill; the
--                        parser and its route are not in this slice, and the columns are nullable
--                        precisely because a sheet entered by hand has no source file.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor. Same reason as 0004: BEFORE ... FOR EACH ROW triggers on a
-- PARTITIONED table were only allowed from 13, and on 11/12 the CREATE TRIGGER statements below
-- fail with a message that reads like a syntax mistake and invites somebody to move the trigger
-- down onto the partitions — where a partition added later arrives silently unprotected.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the budget board needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the trigger below is what stops an archival '
            'record from being destroyed.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: BudgetSheet
-- @scope:  tenant
--
-- bang_ngan_sach — one tab × one budget year (§7). "Thu ngân sách 2026" and "Chi ngân sách 2026"
-- are two rows, and §13's reasoning for du_an applies unchanged: each budget year is its own set of
-- figures, and totalling two years together reports one year's revenue twice.
--
-- `khoan_muc_tong_id` OF §7 IS DELIBERATELY NOT A COLUMN HERE. The specification puts the pointer
-- to the total row on the SHEET; this schema puts a flag on the ROW instead — see the
-- `is_headline` block at the top of this file for the measurement that decided it. The two shapes
-- hold the same fact; the flag is the one that survives a second row being marked, because the read
-- path can SEE that it happened. A nullable pointer cannot express "two rows claim to be the total",
-- so that state would arrive as a plausible single answer.
--
-- `lan` AND `ma`, AND WHY A SHEET NEEDS A REVISION NUMBER AT ALL. §6 draws `🗑 Gỡ` — wipe this
-- tab/year and load it again from a corrected Excel file. That is an ordinary act, not an
-- exception. A soft delete keeps the old row (rule 7), so `UNIQUE (tenant_id, nam, loai)` would make
-- the SECOND load impossible forever, and the partial-index escape (`WHERE deleted_at IS NULL`) is
-- the one thing rule 7 invariant 3 forbids outright — a code that has been issued is never
-- reissued. `lan` resolves both at once: each load of one year+kind is its own revision, the
-- uniqueness counts soft-deleted rows as it must, and `ma` (`NS-2026-CHI-01`) is a business code
-- that is genuinely never reused. It is also what `audit_log.subject` holds for every write below,
-- since a budget LINE has no business code of its own (rule 6, invariant 8).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bang_ngan_sach (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,
    -- "NS-2026-CHI-01" — issued once, never reissued (rule 7, invariant 3). The uniqueness below
    -- counts soft-deleted rows for exactly that reason.
    ma             TEXT        NOT NULL,
    nam            INT         NOT NULL,
    loai           TEXT        NOT NULL,
    -- 1, 2, 3 … one per `🗑 Gỡ` + reload cycle of this year+kind. See the note above.
    lan            INT         NOT NULL DEFAULT 1,
    -- "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026" — the commune's own wording, printed
    -- on the report card. NOT derived from `nam` and `loai`: the real title carries the commune's
    -- name as the commune writes it, and a name assembled in code is a name that will be wrong for
    -- somebody (rule 1, invariant 10).
    tieu_de        TEXT        NOT NULL,
    -- "Triệu đồng". A DISPLAY unit — every stored value is in đồng, see the MONEY block above.
    don_vi_tinh    TEXT        NOT NULL DEFAULT 'Triệu đồng',
    -- "Luỹ kế đến 25/8/2026" (§6). NULL until the commune states it; a sheet with no stated cutoff
    -- is honest, a sheet defaulted to today is a claim nobody made.
    luy_ke_den     DATE,
    -- Filled by the Excel import when that exists (§6). NULL for a sheet entered by hand, and the
    -- pair is meaningless apart — see the CHECK below.
    nguon_tep      TEXT,
    nap_luc        TIMESTAMPTZ,
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id (rule 1, invariant 6), and COUNTING SOFT-DELETED ROWS on purpose.
    UNIQUE (tenant_id, ma),
    -- The same fact stated in its natural columns, so a second live revision cannot be created with
    -- a hand-written `ma` that happens not to collide.
    UNIQUE (tenant_id, nam, loai, lan),
    CONSTRAINT bang_ngan_sach_loai_hop_le CHECK (loai IN ('thu', 'chi')),
    CONSTRAINT bang_ngan_sach_nam_hop_le CHECK (nam BETWEEN 2000 AND 2100),
    CONSTRAINT bang_ngan_sach_lan_duong CHECK (lan >= 1),
    -- A source file with no load instant, or an instant with no file, is half a fact. Either both
    -- or neither: it is what lets "nạp từ Excel" be told from "gõ tay" years later, which is the
    -- first question asked when a figure is disputed.
    CONSTRAINT bang_ngan_sach_nguon_tep_du
        CHECK ((nguon_tep IS NULL) = (nap_luc IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS bang_ngan_sach_p%s PARTITION OF bang_ngan_sach '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The only read shape there is: one commune, one year, one kind, newest revision first.
CREATE INDEX IF NOT EXISTS bang_ngan_sach_theo_nam_loai
    ON bang_ngan_sach (tenant_id, nam, loai, lan DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS bang_ngan_sach_cam_xoa_cung ON bang_ngan_sach;
CREATE TRIGGER bang_ngan_sach_cam_xoa_cung
    BEFORE DELETE ON bang_ngan_sach
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: BudgetColumn
-- @scope:  tenant
--
-- cot_ngan_sach — the columns of one sheet (§3, "cột là dữ liệu, không phải schema").
--
-- The two tabs carry DIFFERENT column sets, and the set comes from the Phòng Tài chính's file
-- rather than from this repository. So a column is a row, not a Go field — and nothing in code may
-- match a column by its NAME.
--
-- `vai_tro` IS THE COLUMN THAT MAKES ADR 0035 §A ENFORCEABLE, and it is the same shape as
-- `is_headline` for the same reason. ADR 0035 fixes two figures in terms of NAMED columns:
--
--	Cân đối thu - chi   Tổng thu is `Thu xã hưởng`, NOT `Thu ngân sách NSNN`     (#32)
--	Thu đạt dự toán     divided by `Dự toán TP giao`, NOT `Dự toán Xã giao`      (#33)
--
-- With columns as free-text data there are exactly two ways to find them: match the label, or have
-- somebody mark the column. Matching the label is the `is_headline` trap word for word — right on
-- the form it was written against, wrong on the next one, with nothing reporting it, and the
-- difference between the two revenue columns in the specification's own sample is over a million
-- units, enough to flip the sign of the Cân đối cell. So the commune marks the column, the software
-- never guesses, and a sheet with no column marked produces NO indicator and a sentence saying why.
--
-- THE SET OF ROLES IS CLOSED AND MATCHES THE SHEET KIND. A closed set is ADR 0035's own rule for
-- code sets — closed now, opened later with one line; the reverse is manual mapping over records
-- that are already signed.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cot_ngan_sach (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    bang_id       TEXT        NOT NULL,
    -- "Thu ngân sách NSNN" — the heading exactly as the commune's file spells it.
    ten           TEXT        NOT NULL,
    thu_tu        INT         NOT NULL,
    kieu          TEXT        NOT NULL,
    -- For a `phan_tram` column: "col_4 / col_2 * 100" (§7). STORED AND NOT EVALUATED HERE — §9
    -- rule 3 says a percentage column is not stored and is computed at render, so the figure never
    -- becomes a second home for something derivable. The two indicators that DO go into a report
    -- are computed from `vai_tro`, never from this string.
    cong_thuc     TEXT,
    -- NULL for an ordinary column. Non-null only for the six the two indicators need — see the
    -- block above and the CHECK below.
    vai_tro       TEXT,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- One column may hold a given role at most once per sheet. NULL never collides with NULL in
    -- SQL, so ordinary columns are unconstrained — which is the intent. Composite with tenant_id
    -- (rule 1, invariant 6) and counting soft-deleted rows, like every other key in this service.
    UNIQUE (tenant_id, bang_id, vai_tro),
    CONSTRAINT cot_ngan_sach_kieu_hop_le CHECK (kieu IN ('so', 'phan_tram')),
    -- A percentage column with no formula cannot be rendered, and a number column with one is a
    -- formula nothing will ever run. Either way the row is a half-fact.
    CONSTRAINT cot_ngan_sach_cong_thuc_dung_kieu
        CHECK ((kieu = 'phan_tram') = (cong_thuc IS NOT NULL)),
    -- The closed set. `du-toan-tp-giao` · `du-toan-xa-giao` · `thu-nsnn` · `thu-xa-huong` belong to
    -- a `thu` sheet; `du-toan-nam` · `chi-ngan-sach` to a `chi` sheet. WHICH KIND OF SHEET a role
    -- is legal on cannot be checked here — `loai` is on the other table — and that check lives in
    -- internal/domain. Vietnamese without diacritics (ADR 0011), like every other code in this
    -- service.
    CONSTRAINT cot_ngan_sach_vai_tro_hop_le CHECK (
        vai_tro IS NULL OR vai_tro IN (
            'du-toan-tp-giao', 'du-toan-xa-giao', 'thu-nsnn', 'thu-xa-huong',
            'du-toan-nam', 'chi-ngan-sach')),
    -- A role on a percentage column would make an indicator read a figure that is itself a ratio.
    CONSTRAINT cot_ngan_sach_vai_tro_chi_tren_cot_so
        CHECK (vai_tro IS NULL OR kieu = 'so')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS cot_ngan_sach_p%s PARTITION OF cot_ngan_sach '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS cot_ngan_sach_theo_bang
    ON cot_ngan_sach (tenant_id, bang_id, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS cot_ngan_sach_cam_xoa_cung ON cot_ngan_sach;
CREATE TRIGGER cot_ngan_sach_cam_xoa_cung
    BEFORE DELETE ON cot_ngan_sach
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: BudgetLine
-- @scope:  tenant
--
-- khoan_muc_ngan_sach — one row of the tree (§4, §7).
--
-- `tt` IS TEXT AND MUST STAY TEXT. "A", "I", "1.1", "-", and blank are all real values from the
-- commune's own file (§4.1). Parsing it into a number loses "A" entirely and orders "1.10" before
-- "1.2"; ordering is `thu_tu`, which is separate and explicit.
--
-- `cap` IS STORED BUT IS NEVER CLIENT-SUPPLIED. §7 lists it and the screen indents by it. It is
-- derivable from `cha_id`, so internal/app computes it from the parent on every write and refuses
-- a value in the request body — the same discipline `nguon` has on the catalogue tables. Deriving
-- it on read instead would mean walking the whole tree for a value the writer already knew.
--
-- `is_headline` — see the block at the top of this file. NOT NULL DEFAULT false: a new line is not
-- the total until somebody says so.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS khoan_muc_ngan_sach (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    bang_id       TEXT        NOT NULL,
    -- NULL at the top level. A logical reference to another row of this same table; no foreign key,
    -- following the precedent 0003 set and 0004 restated — a self-referencing FK on a HASH
    -- partitioned table is a form no PostgreSQL reachable from this build environment can verify.
    -- THE COST IS STATED: nothing in the database stops a line naming a parent that does not exist,
    -- or two lines naming each other. internal/app refuses both, inside the transaction.
    cha_id        TEXT,
    tt            TEXT        NOT NULL DEFAULT '',
    ten           TEXT        NOT NULL,
    thu_tu        INT         NOT NULL,
    cach_tinh     TEXT        NOT NULL DEFAULT 'manual',
    cap           INT         NOT NULL DEFAULT 0,
    is_headline   BOOLEAN     NOT NULL DEFAULT false,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- `entries` IS ABSENT ON PURPOSE — §5's batch dialog is not built, and a mode with no table
    -- behind it reports 0 for every line set to it while looking like a working feature. See the
    -- "does not create" block at the top.
    CONSTRAINT khoan_muc_ngan_sach_cach_tinh_hop_le
        CHECK (cach_tinh IN ('manual', 'children')),
    CONSTRAINT khoan_muc_ngan_sach_cap_khong_am CHECK (cap >= 0),
    -- A line cannot be its own parent. The only self-reference cycle a single row can express, and
    -- the only one checkable without a recursive query.
    CONSTRAINT khoan_muc_ngan_sach_khong_tu_lam_cha CHECK (cha_id IS DISTINCT FROM id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS khoan_muc_ngan_sach_p%s PARTITION OF khoan_muc_ngan_sach '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The whole tree of one sheet, in display order, is the only read there is (§4 draws all 59 rows).
CREATE INDEX IF NOT EXISTS khoan_muc_ngan_sach_theo_bang
    ON khoan_muc_ngan_sach (tenant_id, bang_id, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS khoan_muc_ngan_sach_cam_xoa_cung ON khoan_muc_ngan_sach;
CREATE TRIGGER khoan_muc_ngan_sach_cam_xoa_cung
    BEFORE DELETE ON khoan_muc_ngan_sach
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: BudgetLineValue
-- @scope:  tenant
--
-- gia_tri_khoan_muc — one cell: one line × one column (§7).
--
-- NO SOFT-DELETE COLUMNS, AND THAT IS NOT AN OVERSIGHT. A cell is not a record somebody files; it
-- is a figure on a record, and the record is the LINE, which does carry the three columns rule 7
-- invariant 1 names. Clearing a cell sets `gia_tri` to NULL — the row stays, the hard delete is
-- refused by the trigger, and what the cell used to hold is in `audit_log`, append-only.
--
-- ONLY `kieu = 'so'` COLUMNS EVER GET A ROW HERE. §9 rule 3: a percentage column is computed at
-- render and never stored. The database cannot check that — `kieu` is on the other table — and
-- internal/app refuses it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gia_tri_khoan_muc (
    tenant_id     TEXT        NOT NULL,
    khoan_muc_id  TEXT        NOT NULL,
    cot_id        TEXT        NOT NULL,
    -- ĐỒNG. BIGINT, never floating point. NULL means the cell is empty, which the screen draws as
    -- `—` and which is NOT the same statement as 0 (§9 rule 4).
    gia_tri       BIGINT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The composite primary key IS the uniqueness this table needs: one cell per (line, column),
    -- inside one commune (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, khoan_muc_id, cot_id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS gia_tri_khoan_muc_p%s PARTITION OF gia_tri_khoan_muc '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS gia_tri_khoan_muc_cam_xoa_cung ON gia_tri_khoan_muc;
CREATE TRIGGER gia_tri_khoan_muc_cam_xoa_cung
    BEFORE DELETE ON gia_tri_khoan_muc
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE, and DDL
-- by the table owner. Same line ADR 0013 draws for the audit ledger — the job is to make the
-- accidental and the convenient impossible, not to defeat an administrator who has decided to
-- destroy records and is willing to be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while all four tables are still
-- empty the reversal is complete and loses nothing: drop gia_tri_khoan_muc, khoan_muc_ngan_sach,
-- cot_ngan_sach, bang_ngan_sach, and in the same transaction remove this file's row from
-- `schema_migration`. The 128 partitions and the four triggers go with their parent tables. The two
-- trigger FUNCTIONS are 0004's and must NOT be dropped — chung_tu_giai_ngan still uses them.
--
-- ONCE A COMMUNE HAS ROWS HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of budget
-- records that feed a document sent to a higher authority, which is rule 7's first stop condition
-- and needs the user, not a command.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions. See 0004 for the
-- full reason; it is repeated at the end of every migration that declares a partitioned table,
-- because it only verifies the state after a file that CARRIES it.
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
