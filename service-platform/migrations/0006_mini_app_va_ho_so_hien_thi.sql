-- platform — mini_app (the registry app_id -> mode/commune) and ho_so_hien_thi_xa (a commune's
-- display profile). ADR 0044, ADR 0045 §Bảng app_id → tenant_id and decision 5.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001–0005: those have been applied and core/migrate compares
-- the checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY BOTH LIVE IN service-platform:
--   * mini_app answers "which commune does this app belong to". `tenant_id` is a ViGov concept and
--     the registry that defines communes is here (ADR 0003, ADR 0044 §Xã của app riêng).
--     vihat-miniapp knows apps, never communes.
--   * ho_so_hien_thi_xa is the owner's decision 5 of ADR 0045: "Thuộc service-platform".
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: mini_app has no commune column of its own meaning — one row per
--      Zalo App ID; one main app plus at most a few hundred dedicated apps over the platform's
--      life. ho_so_hien_thi_xa holds AT MOST ONE row per commune.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction, progress row inside it. Every
--      statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry is free.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Two NEW tables, one NEW
--      trigger function, triggers on the new tables only. No existing table, column, constraint or
--      index is touched, and no existing query changes its answer.
--   5. RETENTION: nothing is removed. Both tables are soft delete only; hard DELETE is refused by
--      a trigger.
--
-- NO SEED ROWS, ON PURPOSE. No real App ID is known today, and a placeholder row would be a row
-- that grants a commune to whoever presents that string. "No row → refuse" is the whole rule for
-- every app, the main one included (ADR 0045 §Bảng app). Rows are entered by an operator.

-- ---------------------------------------------------------------------------
-- Hard delete refusal, shared by both tables of this file.
--
-- A NEW FUNCTION, NOT A BORROWED ONE: service-comms has `ho_so_luu_tru_cam_xoa_cung()`, but that
-- is another service's schema and this database cannot see it (rule 2, invariant 2).
-- The message names the table and nothing else — it travels into logs (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION platform_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'platform table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Soft delete instead: set deleted_at, deleted_by and delete_reason '
                     '(rule 7, invariant 1).';
END $$;

-- ---------------------------------------------------------------------------
-- mini_app — one registered Zalo Mini App: which mode it runs in, and for a commune's dedicated
-- app, which commune it is bound to.
--
-- `app_id` IS THE KEY WITHOUT tenant_id, and that is the point: an App ID that resolved to two
-- communes would make the commune of a citizen session undecidable. Same argument as
-- `tenant_domain.host` in 0001. The key also holds soft-deleted rows, so an App ID once bound to
-- commune A is never silently re-bound by inserting a second row (rule 7, invariant 3).
--
-- `tenant_id` IS PRESENT EXACTLY WHEN che_do = 'rieng' — the CHECK below, written as an
-- equivalence. A main app carrying a commune would be a default commune on the isolation path
-- (rule 1, forbidden #1); a dedicated app without one is an app that names nobody.
--
-- NO SECRET HERE, EVER. The app secret lives in vihat-miniapp's secret store (ADR 0032).
--
-- NOT CONSTRAINED, deliberately, and reported rather than decided: how many main apps may exist,
-- and whether one commune may hold more than one dedicated app. Nothing in ADR 0044/0045 answers
-- either; a UNIQUE added later on an empty or small table is cheap.
--
-- Read through the platform registry repository (store.Directory), the sanctioned unscoped
-- reader — this lookup is what ESTABLISHES the commune, so it cannot be scoped by one.
-- ---------------------------------------------------------------------------
-- @entity: MiniApp
-- @scope:  platform
CREATE TABLE IF NOT EXISTS mini_app (
    app_id         TEXT        PRIMARY KEY,           -- Zalo Mini App ID. Not a secret, not personal data
    che_do         TEXT        NOT NULL,              -- 'chinh' (ViHAT main app) | 'rieng' (a commune's app)
    tenant_id      TEXT        REFERENCES tenant (id),-- ONLY for 'rieng'
    -- The app is switched off without removing its binding. An inactive row is answered exactly
    -- like an unknown App ID: the caller refuses.
    dang_hoat_dong BOOLEAN     NOT NULL DEFAULT true,
    ghi_chu        TEXT        NOT NULL DEFAULT '',

    -- WHO entered / last changed the row: a business code, never an internal id (rule 6,
    -- invariant 8). NOT NULL without a default — a row nobody signed is the row nobody can defend.
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    tao_boi        TEXT        NOT NULL,
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_boi   TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    CONSTRAINT mini_app_app_id_khong_rong CHECK (btrim(app_id) <> '' AND app_id = btrim(app_id)),
    CONSTRAINT mini_app_che_do_hop_le CHECK (che_do IN ('chinh', 'rieng')),
    CONSTRAINT mini_app_xa_khi_va_chi_khi_rieng CHECK ((che_do = 'rieng') = (tenant_id IS NOT NULL)),
    CONSTRAINT mini_app_nguoi_ghi_khong_rong CHECK (btrim(tao_boi) <> '' AND btrim(cap_nhat_boi) <> ''),
    CONSTRAINT mini_app_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
);

COMMENT ON TABLE mini_app IS
    'So dang ky Zalo Mini App: app_id -> che do (chinh|rieng) va xa (chi app rieng). '
    'Khong co dong -> tu choi. KHONG luu app secret (ADR 0032).';
COMMENT ON COLUMN mini_app.tenant_id IS
    'Xa gan app rieng. NULL khi va chi khi che_do = chinh. Xa ngung hoat dong thi app bi tu choi, '
    'KHONG tu di theo tenant_succession (ADR 0045); gan lai la viec cua nguoi van hanh.';

DROP TRIGGER IF EXISTS mini_app_cam_xoa_cung ON mini_app;
CREATE TRIGGER mini_app_cam_xoa_cung
    BEFORE DELETE ON mini_app
    FOR EACH ROW EXECUTE FUNCTION platform_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- ho_so_hien_thi_xa — what a citizen reads about ONE commune: office address, logo, hotline,
-- working hours as text, introduction. Owned by the commune, edited by its staff (a later task,
-- which needs a quyen key — open question #27), read at runtime by tenant_id (rule 1,
-- invariant 10; ADR 0044 §Cái gì cấu hình động).
--
-- ONE ROW PER COMMUNE: the primary key IS tenant_id. Soft-deleted rows keep holding it, so a
-- profile is restored by the edit path (audited), never re-created beside the old one.
--
-- NOT PARTITIONED, unlike the business tables of 0001: it holds at most one row per commune, so
-- there is nothing for hash partitioning to spread.
--
-- TEXT COLUMNS ARE NOT NULL DEFAULT '' — ONE spelling of "not declared", as for tenant.tinh_thanh.
-- A consumer renders nothing for ''; it never substitutes another commune's value or a default.
-- `logo_url` is the one nullable column, for the reason on it.
-- ---------------------------------------------------------------------------
-- @entity: TenantDisplayProfile
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS ho_so_hien_thi_xa (
    tenant_id      TEXT        NOT NULL REFERENCES tenant (id),

    dia_chi_tru_so TEXT        NOT NULL DEFAULT '',

    -- A URL, NOT BYTES AND NOT A STORAGE KEY. There is no core/storage in this repository and no
    -- agreed object-key scheme, so this holds a reference to an image some other system already
    -- serves — the same choice as service-comms noi_dung_mini_app.anh_dai_dien_url. NULL means
    -- "no logo"; '' is refused so there is one spelling of it. Revisit when file storage exists.
    logo_url       TEXT,

    -- The commune's OFFICIAL hotline / office landline. Open question #16 (decided 22/09/2026):
    -- an office number is PUBLIC-SERVICE information, a personal mobile is personal data under
    -- Decree 13. This column is for the first kind only and is published to citizens as is; a
    -- member of staff's own mobile does not belong here.
    duong_day_nong TEXT        NOT NULL DEFAULT '',

    -- DISPLAY TEXT ONLY ("Thứ 2 – Thứ 6, 7:30 – 17:00"). The AUTHORITATIVE working-hours calendar
    -- is service-identity `lich_lam_viec` (ADR 0007), which deadlines are counted against. Nothing
    -- may compute anything from this column; a second calendar here would drift from the one the
    -- SLA uses, and the citizen would be promised hours the deadline does not count.
    gio_lam_viec_hien_thi TEXT NOT NULL DEFAULT '',

    gioi_thieu     TEXT        NOT NULL DEFAULT '',

    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    tao_boi        TEXT        NOT NULL,              -- staff business code (rule 6, invariant 8)
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_boi   TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    PRIMARY KEY (tenant_id),

    CONSTRAINT ho_so_hien_thi_xa_logo_khong_rong CHECK (logo_url IS NULL OR btrim(logo_url) <> ''),
    CONSTRAINT ho_so_hien_thi_xa_nguoi_ghi_khong_rong
        CHECK (btrim(tao_boi) <> '' AND btrim(cap_nhat_boi) <> ''),
    CONSTRAINT ho_so_hien_thi_xa_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
);

COMMENT ON TABLE ho_so_hien_thi_xa IS
    'Ho so hien thi cua mot xa cho cong dan (dia chi, logo, duong day nong, gio lam viec, gioi thieu). '
    'Mot dong moi xa.';
COMMENT ON COLUMN ho_so_hien_thi_xa.gio_lam_viec_hien_thi IS
    'CHI DE HIEN THI. Lich lam viec chuan de tinh han la lich_lam_viec cua service-identity (ADR 0007); '
    'khong tinh gi tu cot nay.';
COMMENT ON COLUMN ho_so_hien_thi_xa.duong_day_nong IS
    'So may ban / duong day nong CONG VU cua xa (cau hoi mo #16). Khong ghi so di dong ca nhan.';

DROP TRIGGER IF EXISTS ho_so_hien_thi_xa_cam_xoa_cung ON ho_so_hien_thi_xa;
CREATE TRIGGER ho_so_hien_thi_xa_cam_xoa_cung
    BEFORE DELETE ON ho_so_hien_thi_xa
    FOR EACH ROW EXECUTE FUNCTION platform_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- While both tables are empty — every environment today — reversal is exact: remove the two
-- tables, then the function platform_cam_xoa_cung, then this file's progress row from the
-- schema_migration table, keyed on ten = '0006_mini_app_va_ho_so_hien_thi.sql', in one
-- transaction. Written as prose rather than as a runnable line, because a runnable line is a line
-- that gets run.
--
-- ONCE A ROW EXISTS it is no longer a reversal: a mini_app row is what decided the commune of
-- citizen sessions, and a profile is what a commune told its residents. Discarding either is rule
-- 7 stop condition #1 and needs the user plus a verified backup.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten.
--
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0002 draws.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
