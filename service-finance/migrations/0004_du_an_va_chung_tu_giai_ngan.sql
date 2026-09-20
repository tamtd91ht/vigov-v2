-- finance — the disbursement register: a commune's investment projects and the disbursement
-- vouchers recorded against them (docs/ui-ux/06-giai-ngan.md §11).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0003: 0001..0003 have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ---------------------------------------------------------------------------
-- MONEY. THIS IS THE ONE THING THIS FILE MUST NOT GET WRONG.
--
-- Every amount here is BIGINT, counted in ĐỒNG — the smallest unit the currency has, so there is
-- no scaling factor to remember and no remainder to round. NOT double precision, NOT real, NOT
-- float, NOT money.
--
-- THE CONSEQUENCE, stated rather than assumed: a disbursement figure is quoted in a decision that
-- has legal effect, it is totalled into the report that goes to the People's Committee, and it is
-- what a citizen or an inspector is shown. Binary floating point cannot represent a decimal amount
-- exactly, and SUM() over it depends on the ORDER the rows arrive in — so the same query can
-- return two different totals on two runs, both looking entirely plausible, differing by đồng.
-- Nobody notices until an inspection compares two printouts of the same figure. One đồng wrong in
-- a disbursement record is a wrong number in a legally effective file, and no amount of later
-- rounding puts it back.
--
-- BIGINT tops out at 9,22 × 10^18 đồng. A commune's whole annual capital plan in the specification
-- is 3,3 × 10^10 đồng (§14), eight orders of magnitude below the ceiling; SUM() of BIGINT in
-- PostgreSQL widens to NUMERIC, so even the total of every voucher of every year cannot overflow.
-- NUMERIC would also be exact and is the other correct choice; BIGINT is used because it maps to
-- Go's int64 with no decoding step, and a conversion nobody can get wrong beats one nobody has to
-- think about.
--
-- ---------------------------------------------------------------------------
-- THE CUMULATIVE FIGURE IS NOT STORED, AND THAT IS THE DESIGN.
--
-- There is no `da_giai_ngan` column on du_an. "Đã giải ngân" is SUM(chung_tu_giai_ngan.so_tien)
-- over the project's live vouchers, computed on every read. Same reasoning rule 10 gives for
-- refusing an `is_overdue` column: two sources for one number drift, and the stale one is the one
-- that reaches the report going upward — here, a project that looks 90% disbursed on the list and
-- 45% on its own detail page, with nothing on either screen saying which is right.
--
-- If this ever has to change for performance, it is not a schema decision to take quietly: it
-- means choosing which of the two numbers is allowed to be wrong, and for how long.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. Both tables are new and this file writes no row into
--      either — §14 of the specification is PROTOTYPE data and is deliberately not seeded. The
--      ceiling in sight is the specification's own figure: ~63 projects in a budget year, and a
--      handful of vouchers per project.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so
--      a retry after a failure costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS. No
--      existing table, column, index, constraint or trigger is touched, so every query that runs
--      today returns exactly what it returned before. hang_muc_ke_hoach_von is READ by the new
--      tables' logical reference but is not altered — see the note on du_an.hang_muc_id.
--   5. WHAT IT COSTS ON THE LARGEST COMMUNE: nothing measurable. Empty tables, 64 empty
--      partitions, four indexes over no rows.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence reads as a decision:
--
--   nam_ngan_sach        §11 has it, carrying `nguong_canh_bao_cham` (the slow-project threshold,
--                        default 10 points). The default is a CONSTANT in internal/domain for now;
--                        the table is what makes it configurable PER COMMUNE AND PER YEAR, and
--                        until a screen writes it a table nobody fills is a table that reads as
--                        "already configurable" while every commune silently gets the default.
--   nguon_von            §6 and §11 — funding sources, and phan_bo_nguon_von with them. Not in this
--                        slice. chung_tu_giai_ngan therefore has NO nguon_von_id column: a foreign
--                        key column pointing at a table that does not exist is a column nobody can
--                        fill correctly, and §13 rule 6 already says a voucher with no funding
--                        source still counts toward the total — which is exactly what this schema
--                        does, by having no such column at all yet.
--   vuong_mac            §11 — obstacles, and the follow-up task they auto-create (§13 rule 4).
--                        That rule crosses into another service's entity (nhiem_vu) and is not a
--                        finance-only decision.
--   loi_he_thong         deliberately NOT created: ADR 0024 fixes the owner of `report.*` before
--                        that table is written for the first time.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13.
-- On 11 and 12 the CREATE TRIGGER statements below fail with "Partitioned tables cannot have
-- BEFORE / FOR EACH ROW triggers", which reads like a syntax mistake and invites somebody to
-- "fix" it by moving the trigger down onto the partitions — where a partition added later
-- arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the disbursement register needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the triggers below are what stop an archival '
            'record from being destroyed or silently rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- ho_so_luu_tru_cam_xoa_cung — one function, attached to every archival table in this service.
--
-- It refuses DELETE and nothing else, so it can be attached to a table whatever its columns are.
-- Rule 7, forbidden #1: business data is soft deleted (deleted_at, deleted_by, delete_reason);
-- a disbursement record is an archival record with a statutory retention period, and destroying
-- one is an administrative procedure, never a statement somebody types.
--
-- ENFORCED IN THE DATABASE, NOT IN THE APPLICATION, for the reason ADR 0013 gives for the audit
-- ledger: a promise the application layer makes is a promise a migration script, a psql session
-- or the next service to connect never heard.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ho_so_luu_tru_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Disbursement records are archival records with a statutory retention '
                     'period (rule 7, invariant 1). Soft delete instead: set deleted_at, '
                     'deleted_by and delete_reason. Destroying one is an administrative '
                     'procedure, not a statement.';
END $$;

-- ---------------------------------------------------------------------------
-- chung_tu_da_khoa — a locked voucher is frozen (§8.2 lifecycle, §13 rule 3).
--
-- `Kế toán nhập` -> `Đã xác nhận` -> `Đã khoá`, and a locked voucher cannot be edited or removed.
-- The specification also says it can be UNLOCKED by somebody holding budget.confirm, so the guard
-- cannot simply refuse every UPDATE on a locked row. What it refuses is precisely the dangerous
-- shape: changing any FIGURE OR FACT of the voucher while the row is locked.
--
-- WHY THE BUSINESS FIELDS ARE LISTED ONE BY ONE rather than "refuse unless only trang_thai
-- changed": one UPDATE can do both at once — unlock AND rewrite the amount — and a guard that
-- only looked at trang_thai would wave it through. Unlocking must be its own statement, because a
-- record of what changed is only readable if the two changes are two events.
--
-- A COLUMN ADDED LATER AND NOT ADDED HERE IS AN EDITABLE LOCKED FIELD. That is the known weakness
-- of the allowlist-by-omission shape; it is written down rather than hidden, and nguon_von_id is
-- the first column that will have to be added to this list when funding sources arrive.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION chung_tu_da_khoa() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.trang_thai <> 'da-khoa' THEN
        RETURN NEW;
    END IF;

    IF NEW.du_an_id   IS DISTINCT FROM OLD.du_an_id
    OR NEW.ngay_chi   IS DISTINCT FROM OLD.ngay_chi
    OR NEW.so_tien    IS DISTINCT FROM OLD.so_tien
    OR NEW.noi_dung   IS DISTINCT FROM OLD.noi_dung
    OR NEW.doi_tac    IS DISTINCT FROM OLD.doi_tac
    OR NEW.so_chung_tu IS DISTINCT FROM OLD.so_chung_tu
    OR NEW.tep_dinh_kem IS DISTINCT FROM OLD.tep_dinh_kem THEN
        RAISE EXCEPTION 'chung_tu_giai_ngan: a locked voucher cannot be edited'
            USING HINT = 'Vòng đời: kế toán nhập -> đã xác nhận -> đã khoá (§8.2). Unlock it '
                         'first, in a statement of its own, with the budget.confirm permission — '
                         'so that the unlocking and the change are two separate audited events.';
    END IF;

    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'chung_tu_giai_ngan: a locked voucher cannot be removed'
            USING HINT = 'Unlock it first (budget.confirm), then soft delete with a reason. A '
                         'locked voucher is a figure somebody has already signed off.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: Project
-- @scope:  tenant
--
-- du_an — one investment project of one commune, in one budget year.
--
-- ONE PROJECT BELONGS TO ONE BUDGET YEAR AND ONE CATEGORY. §13 rule 8: changing budget year does
-- not delete the old year's data — each year is its own set of projects. That is why `nam` is a
-- column on the project and not a filter over a timestamp: a project of 2025 and its successor in
-- 2026 are two rows with two plans and two sets of vouchers, and totalling them together would
-- report one year's disbursement twice.
--
-- hang_muc_id REFERENCES hang_muc_ke_hoach_von(tenant_id, id) LOGICALLY, AND THERE IS NO FOREIGN
-- KEY. A real FK between two HASH-partitioned tables is accepted by PostgreSQL 13, but no
-- PostgreSQL is reachable from this repository's build environment (VIGOV_TEST_DSN is unset, the
-- integration suites skip without executing one statement), and a migration that fails at startup
-- stops the service. The precedent set in 0003 is followed: the form that needs no verification
-- wins. THE COST IS STATED — nothing in the database stops a project from naming a category that
-- does not exist; the read path joins on (tenant_id, hang_muc_id), so such a project simply has no
-- category name and is visible as such, rather than leaking a category from another commune (the
-- join is constrained to $1 on both sides).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS du_an (
    tenant_id            TEXT        NOT NULL,
    id                   TEXT        NOT NULL,
    -- "DA-2026-be-tong-hoa-duong-ngo-xo-2" — issued once, never reissued (rule 7, invariant 3).
    -- The uniqueness below counts soft-deleted rows for exactly that reason.
    ma                   TEXT        NOT NULL,
    nam                  INT         NOT NULL,
    hang_muc_id          TEXT        NOT NULL,
    ten                  TEXT        NOT NULL,
    mo_ta                TEXT,
    -- ĐỒNG. BIGINT, never floating point — see the MONEY block at the top of this file.
    ke_hoach_von_nam     BIGINT      NOT NULL,
    -- NULL means "the same as this year's plan" (§9: "Để trống thì lấy bằng số tiền bố trí năm
    -- nay"). NULL rather than a copied value: a copy silently stops following the plan the day
    -- the plan is revised, and nothing on the screen says which of the two figures is stale.
    tong_muc_duoc_duyet  BIGINT,
    don_vi_thuc_hien_id  TEXT,
    can_bo_phu_trach_id  TEXT,
    ngay_khoi_cong       DATE,
    ngay_hoan_thanh      DATE,
    -- "Mốc phải hoàn tất phần vốn của năm" — a different thing from ngay_hoan_thanh, and §9 says
    -- so explicitly: the works can finish in March and the money still has to be disbursed by 31/12.
    thoi_han_giai_ngan   DATE        NOT NULL,
    deleted_at           TIMESTAMPTZ,
    deleted_by           TEXT,
    delete_reason        TEXT,
    tao_luc              TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id (rule 1, invariant 6), and counting soft-deleted rows: §9 says a
    -- hand-entered code "phải chưa từng được dùng, kể cả bởi dự án đã rút khỏi danh sách".
    -- Restrict this to live rows and a withdrawn project's code can be reissued to an unrelated
    -- project — after which the vouchers of the first one read as belonging to the second.
    UNIQUE (tenant_id, ma),
    -- A plan of zero is a real state (a project entered before its allocation is decided); a
    -- NEGATIVE plan is not a state, it is a parsing accident or a sign error, and it would drag
    -- the commune's total below the truth with no row looking wrong.
    CONSTRAINT du_an_ke_hoach_khong_am CHECK (ke_hoach_von_nam >= 0),
    CONSTRAINT du_an_tong_muc_khong_am CHECK (tong_muc_duoc_duyet IS NULL OR tong_muc_duoc_duyet >= 0),
    -- A budget year outside this window is a typo (1026, 20226) that would make a project
    -- invisible on every screen while sitting in the table looking healthy.
    CONSTRAINT du_an_nam_hop_le CHECK (nam BETWEEN 2000 AND 2100)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS du_an_p%s PARTITION OF du_an '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The list screen is always one commune, one budget year, ordered by code (§7).
CREATE INDEX IF NOT EXISTS du_an_danh_sach
    ON du_an (tenant_id, nam, ma) WHERE deleted_at IS NULL;

-- "Tiến độ theo hạng mục" (§5) groups one year's projects by category.
CREATE INDEX IF NOT EXISTS du_an_theo_hang_muc
    ON du_an (tenant_id, nam, hang_muc_id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS du_an_cam_xoa_cung ON du_an;
CREATE TRIGGER du_an_cam_xoa_cung
    BEFORE DELETE ON du_an
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: DisbursementVoucher
-- @scope:  tenant
--
-- chung_tu_giai_ngan — one recorded payment against one project (§8.2, §11).
--
-- EVERY LIVE VOUCHER COUNTS TOWARD "ĐÃ GIẢI NGÂN", whatever its state. §11 says the total is the
-- sum of vouchers "WHERE trạng thái ≥ kế toán nhập", and `ke-toan-nhap` is the FIRST state, so the
-- condition admits all three. It is written here because a reader meeting `trang_thai` for the
-- first time will assume the total waits for confirmation — it does not, and a read path that
-- added `AND trang_thai <> 'ke-toan-nhap'` would quietly report a commune as further behind than
-- it is. The only rows excluded anywhere are soft-deleted ones (rule 7, invariant 2).
--
-- THE STATE IS A CHECK CONSTRAINT AND NOT A PostgreSQL ENUM, following ADR 0011's rule that enum
-- values stay Vietnamese without diacritics: a CHECK can be widened in a plain migration, while
-- an enum type has to be altered outside a transaction on some versions — which is how a
-- half-applied state change happens.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS chung_tu_giai_ngan (
    tenant_id          TEXT        NOT NULL,
    id                 TEXT        NOT NULL,
    du_an_id           TEXT        NOT NULL,
    ngay_chi           DATE        NOT NULL,
    -- ĐỒNG. BIGINT, never floating point — see the MONEY block at the top of this file.
    so_tien            BIGINT      NOT NULL,
    noi_dung           TEXT        NOT NULL,
    -- "Công ty ABC" — the counterparty. A company, not a person: nothing on this table is personal
    -- data under Decree 13 (rule 3), and nothing that IS should be added here without asking.
    doi_tac            TEXT,
    so_chung_tu        TEXT,
    tep_dinh_kem       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    trang_thai         TEXT        NOT NULL DEFAULT 'ke-toan-nhap',
    nguoi_nhap_id      TEXT        NOT NULL,
    nguoi_xac_nhan_id  TEXT,
    thoi_diem_khoa     TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ,
    deleted_by         TEXT,
    delete_reason      TEXT,
    tao_luc            TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT chung_tu_giai_ngan_trang_thai_hop_le
        CHECK (trang_thai IN ('ke-toan-nhap', 'da-xac-nhan', 'da-khoa')),
    -- A voucher of zero đồng records nothing and would only ever be a mistake or an import
    -- artefact; a negative one is a refund, which is a different business event with a different
    -- name, and letting it in here would silently reduce a disbursement total that a decision
    -- already quoted. If refunds have to be recorded, that is a question for the customer, not a
    -- sign change.
    CONSTRAINT chung_tu_giai_ngan_so_tien_duong CHECK (so_tien > 0),
    -- A locked voucher must carry the moment it was locked: `Đã khoá` with no timestamp is a row
    -- nobody can account for when an inspection asks when the figure was frozen.
    CONSTRAINT chung_tu_giai_ngan_khoa_co_thoi_diem
        CHECK (trang_thai <> 'da-khoa' OR thoi_diem_khoa IS NOT NULL)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS chung_tu_giai_ngan_p%s PARTITION OF chung_tu_giai_ngan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- Two read shapes, both on one commune: the voucher list of one project (§8.2, newest first) and
-- the SUM per project that produces "đã giải ngân" for every list and every total.
CREATE INDEX IF NOT EXISTS chung_tu_giai_ngan_theo_du_an
    ON chung_tu_giai_ngan (tenant_id, du_an_id, ngay_chi DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS chung_tu_giai_ngan_cam_xoa_cung ON chung_tu_giai_ngan;
CREATE TRIGGER chung_tu_giai_ngan_cam_xoa_cung
    BEFORE DELETE ON chung_tu_giai_ngan
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS chung_tu_giai_ngan_da_khoa ON chung_tu_giai_ngan;
CREATE TRIGGER chung_tu_giai_ngan_da_khoa
    BEFORE UPDATE ON chung_tu_giai_ngan
    FOR EACH ROW EXECUTE FUNCTION chung_tu_da_khoa();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE, and DDL
-- by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping a table). Same line ADR 0013 draws
-- for the audit ledger — the job is to make the accidental and the convenient impossible, not to
-- defeat an administrator who has decided to destroy records and is willing to be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while both tables are still empty
-- the reversal is complete and loses nothing: drop chung_tu_giai_ngan, then du_an, then the two
-- trigger functions, and in the same transaction remove this file's row from `schema_migration`,
-- otherwise the runner still believes the schema is in place. The 64 partitions and the triggers
-- go with their parent tables.
--
-- ONCE A COMMUNE HAS ROWS HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of
-- disbursement records, which is rule 7's first stop condition and needs the user, not a command.
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
